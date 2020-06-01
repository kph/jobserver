// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	r           *os.File
	w           *os.File
	m           sync.Mutex
	cl          *Client
	tokens      int
	currentJobs int
	maxJobs     int
}

// SetupServer is used to set up a controlling jobserver for a new process.
// We are passed an exec.Cmd which may already have been set up with
// ExtraFiles. We assign two new files (pipes for the jobserver protocol)
// to the two next free file descriptors, and modify the process
// environment MAKEFLAGS variable to reference the allocated file descriptors.
func SetupServer(cmd *exec.Cmd, cl *Client, jobs int) (srv *Server, err error) {
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

	srv = &Server{r: r2, w: w1, maxJobs: jobs, cl: cl}

	go func() {
		for {
			go srv.EnableJobs()
			p := make([]byte, 1)
			n, err := srv.r.Read(p)
			if err != nil {
				panic(err)
			}
			if n != 1 {
				panic("Unexpected byte count")
			}
			fmt.Printf("%s: Client returned a token\n",
				os.Args[0])
			srv.m.Lock()
			srv.maxJobs = 0
			srv.cl.PutToken()
			srv.tokens--
			srv.m.Unlock()
		}
	}()

	return
}

func (srv *Server) EnableJobs() {
	srv.m.Lock()
	defer srv.m.Unlock()
	for srv.tokens < srv.maxJobs {
		srv.cl.GetToken()
		srv.tokens++
		n, err := srv.w.Write([]byte{'+'})
		if err != nil {
			panic(err)
		}
		if n != 1 {
			panic("Unexpected byte count")
		}
		fmt.Printf("Sent client a token, savedTokens=%d\n",
			len(srv.cl.usedTokens))
	}
}

func (srv *Server) DisableJobs() {
	srv.m.Lock()
	srv.maxJobs = 0
	srv.m.Unlock()
	for {
		srv.m.Lock()
		cnt := srv.tokens
		srv.m.Unlock()
		if cnt == 0 {
			break
		}
		//		if srv.cl != nil {
		//	srv.cl.PutToken()
		//}
		//srv.tokens--
		time.Sleep(10 * time.Millisecond)
	}
}
