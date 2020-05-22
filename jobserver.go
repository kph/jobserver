// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var ErrBadMakeflags = errors.New("Invalid format in MAKEFLAGS")
var ErrNotRecursiveMake = errors.New("Make rule not marked as recursive")

type Client struct {
	r    *os.File
	w    *os.File
	m    sync.Mutex
	jobs int
}

type Server struct {
	r           *os.File
	w           *os.File
	m           sync.Mutex
	currentJobs int
	maxJobs     int
}

type Token struct {
	t byte
}

func pipeFdToFile(fd int, name string) *os.File {
	var stats syscall.Stat_t

	err := syscall.Fstat(fd, &stats)
	if err == nil && ((stats.Mode & syscall.S_IFIFO) != 0) {
		return os.NewFile(uintptr(fd), name)
	}
	return nil
}

func ParseMakeflags() (cl *Client, err error) {
	mflags := strings.Fields(os.Getenv("MAKEFLAGS"))
	cl = &Client{}
	for _, mflag := range mflags {
		fmt.Println(mflag)
		if strings.HasPrefix(mflag, "--jobserver-auth=") {
			s := strings.Split(strings.TrimPrefix(
				mflag, "--jobserver-auth="), ",")
			if cl.r != nil {
				return nil, ErrBadMakeflags
			}
			if len(s) != 2 {
				return nil, ErrBadMakeflags
			}
			r, err := strconv.Atoi(s[0])
			if err != nil {
				return nil, ErrBadMakeflags
			}
			w, err := strconv.Atoi(s[1])
			if err != nil {
				return nil, ErrBadMakeflags
			}
			fmt.Printf("R = %d W = %d\n", r, w)
			cl.r = pipeFdToFile(r, "Jobserver-R")
			if cl.r == nil {
				return nil, ErrNotRecursiveMake
			}
			cl.w = pipeFdToFile(w, "Jobserver-W")
			if cl.w == nil {
				return nil, ErrNotRecursiveMake
			}
			continue
		}
		if strings.HasPrefix(mflag, "-j") {
			s := strings.TrimPrefix(mflag, "-j")
			if s != "" {
				cl.jobs, err = strconv.Atoi(s)
				if err != nil {
					return nil, ErrBadMakeflags
				}
			}
		}
	}
	return
}

func (cl *Client) GetToken() (t Token) {
	if cl.r == nil {
		cl.m.Lock()
		return Token{}
	}
	p := make([]byte, 1)
	n, err := cl.r.Read(p)
	if err != nil {
		panic(err)
	}
	if n != 1 {
		panic("Unexpected byte count")
	}
	return Token{t: p[0]}
}

func (cl *Client) PutToken(t Token) {
	if cl.r == nil {
		cl.m.Unlock()
		return
	}
	n, err := cl.w.Write([]byte{t.t})
	if err != nil {
		panic(err)
	}
	if n != 1 {
		panic("Unexpected byte count")
	}
}

func SetupServer(cmd *exec.Cmd, jobs int) (srv *Server, err error) {
	fd := 3 + len(cmd.ExtraFiles)
	r1, w1, err := os.Pipe()
	if err != nil {
		return
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		r1.Close()
		w1.Close()
		return
	}
	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	js := "--jobserver-auth=" + strconv.Itoa(fd) + "," + strconv.Itoa(fd+1)
	found := false
	for i, envvar := range env {
		if strings.HasPrefix(envvar, "MAKEFLAGS=") {
			mflags := strings.Fields(strings.TrimPrefix(envvar,
				"MAKEFLAGS="))
			for j, mflag := range mflags {
				if strings.HasPrefix(mflag, "--jobserver-auth=") {
					mflags[j] = js
					found = true
					break
				}
			}
			if !found {
				mflags = append(mflags, js)
				found = true
			}
			env[i] = "MAKEFLAGS=" + strings.Join(mflags, " ")
			break
		}
	}
	if !found {
		env = append(env, "MAKEFLAGS="+js)
	}
	cmd.Env = env
	cmd.ExtraFiles = append(cmd.ExtraFiles, r1)
	cmd.ExtraFiles = append(cmd.ExtraFiles, w2)

	srv = &Server{r: r2, w: w1, maxJobs: jobs}

	srv.EnableJobs()

	go func() {
		p := make([]byte, 1)
		n, err := r1.Read(p)
		if err != nil {
			panic(err)
		}
		if n != 1 {
			panic("Unexpected byte count")
		}
		srv.m.Lock()
		srv.currentJobs--
		srv.m.Unlock()

	}()

	return
}

func (srv *Server) EnableJobs() {
	for srv.currentJobs < srv.maxJobs {
		n, err := srv.w.Write([]byte{'+'})
		if err != nil {
			panic(err)
		}
		if n != 1 {
			panic("Unexpected byte count")
		}
		srv.m.Lock()
		srv.currentJobs++
		srv.m.Unlock()
	}
}