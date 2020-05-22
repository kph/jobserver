// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	r           *os.File
	w           *os.File
	m           sync.Mutex
	currentJobs int
	maxJobs     int
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

	go func() {
		go srv.EnableJobs()
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
	srv.m.Lock()
	defer srv.m.Unlock()
	for srv.currentJobs < srv.maxJobs {
		n, err := srv.w.Write([]byte{'+'})
		if err != nil {
			panic(err)
		}
		if n != 1 {
			panic("Unexpected byte count")
		}
		srv.currentJobs++
	}
}
