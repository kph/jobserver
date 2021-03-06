// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// A Client tracks the jobserver controlling us, i.e. our parent. It
// is also used in the case where we are the parent.
type Client struct {
	r             *os.File   // Pipe from parent giving us tokens
	w             *os.File   // Pipe to parent returning tokens
	c             *sync.Cond // Condition variable for tokens
	jobs          int        // Count of jobs from MAKEFLAGS -j option
	freeTokens    []token    // Tokens we've been given but aren't using
	usedTokens    []token    // Tokens that are currently in use
	maxLocalJobs  int        // Maximum jobs to allocate from our private pool
	usedLocalJobs int        // Current number of local jobs allocated
	flushing      bool       // We are flusing tokens
}

type token struct {
	t byte
}

// pipeFDToFile is a wrapper for os.NewFile() which makes sure that
// the file descriptor is actually a pipe, and then returns an os.File.
func pipeFdToFile(fd int, name string) *os.File {
	var stats syscall.Stat_t

	err := syscall.Fstat(fd, &stats)
	if err == nil && ((stats.Mode & syscall.S_IFIFO) != 0) {
		return os.NewFile(uintptr(fd), name)
	}
	return nil
}

// NewClient determines whether we have a parent jobserver or not, and
// returns a client structure. If MAKEFLAGS can not be parsed, we will
// return an error. If there is no MAKEFLAGS, then we are a top-level
// build controller, and we will default to allowing as many concurrent
// operations as there are CPUs, or a limit set by localClients.
func NewClient(localClients int) (cl *Client, err error) {
	mflags := strings.Fields(os.Getenv("MAKEFLAGS"))
	cl = &Client{}
	cl.c = sync.NewCond(&sync.Mutex{})
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
	if cl.r != nil {
		cl.freeTokens = make([]token, 0)
		cl.usedTokens = make([]token, 0)
		go func() {
			p := make([]byte, 1)
			for {
				n, err := cl.r.Read(p)
				if err != nil {
					if errors.Is(err, os.ErrClosed) {
						return
					}
					panic(err)
				}
				if n != 1 {
					panic("Unexpected byte count")
				}
				cl.c.L.Lock()
				cl.freeTokens = append(cl.freeTokens,
					token{t: p[0]})
				cl.c.Signal()
				cl.c.L.Unlock()
			}
		}()
	} else {
		if localClients != 0 {
			cl.maxLocalJobs = localClients
		} else {
			cl.maxLocalJobs = runtime.NumCPU()
		}
	}
	return
}

// GetToken() is used to get an execution token. Before starting a CPU-bound
// build operation, call GetToken() and the caller will block until a token
// is available.
func (cl *Client) GetToken() {
	cl.c.L.Lock()
	for {
		if cl.flushing {
			panic("GetToken() while flusing tokens")
		}
		if cl.usedLocalJobs < cl.maxLocalJobs {
			cl.usedLocalJobs++
			fmt.Printf("%s: GetToken() usedLocalJobs %d maxLocalJobs %d\n",
				os.Args[0], cl.usedLocalJobs,
				cl.maxLocalJobs)
			cl.c.L.Unlock()
			return
		}

		if len(cl.freeTokens) > 0 {
			t := cl.freeTokens[0]
			cl.freeTokens = cl.freeTokens[1:]
			cl.usedTokens = append(cl.usedTokens, t)
			fmt.Printf("%s: GetToken() free %d saved %d\n",
				os.Args[0],
				len(cl.freeTokens), len(cl.usedTokens))
			cl.c.L.Unlock()
			return
		}
		fmt.Printf("%s: GetToken() waiting\n", os.Args[0])
		cl.c.Wait()
	}
}

// PutToken() is used to return an execution token. When a CPU-bound build
// operation is done, call PutToken() to make execution available to another
// build operation.
func (cl *Client) PutToken() {
	cl.c.L.Lock()
	defer cl.c.L.Unlock()
	fmt.Printf("%s: PutToken() usedLocalJobs %d maxLocalJobs %d free %d saved %d\n",
		os.Args[0], cl.usedLocalJobs, cl.maxLocalJobs,
		len(cl.freeTokens), len(cl.usedTokens))
	if cl.flushing {
		panic("GetToken() while flusing tokens")
	}
	if cl.usedLocalJobs > 0 {
		cl.usedLocalJobs--
	} else {
		if len(cl.usedTokens) > 0 {
			t := cl.usedTokens[0]
			cl.usedTokens = cl.usedTokens[1:]
			cl.freeTokens = append(cl.freeTokens, t)
		} else {
			panic("PutToken() without a token to free")
		}
	}
	cl.c.Signal()
}

func (cl *Client) FlushTokens() {
	cl.c.L.Lock()
	defer cl.c.L.Unlock()
	cl.flushing = true
	if cl.r != nil {
		cl.r.Close()
	}
	fmt.Printf("%s: FlushTokens free %d saved %d\n", os.Args[0],
		len(cl.freeTokens), len(cl.usedTokens))
	for len(cl.freeTokens) > 0 {
		tk := cl.freeTokens[0]
		cl.freeTokens = cl.freeTokens[1:]
		fmt.Printf("%s: FlushTokens() free %d saved %d\n",
			os.Args[0], len(cl.freeTokens),
			len(cl.usedTokens))
		n, err := cl.w.Write([]byte{tk.t})
		if err != nil {
			panic(err)
		}
		if n != 1 {
			panic("Unexpected byte count")
		}
	}
}

// Tokens() returns the count of available tokens.
func (cl *Client) Tokens() int {
	cnt := len(cl.freeTokens)
	if cl.usedLocalJobs < cl.maxLocalJobs {
		cnt += (cl.maxLocalJobs - cl.usedLocalJobs)
	}
	return cnt
}

// ExpectedJobs() returns the max jobs we expect will be allowed at once.
// This number is a guess. When make calls us with the -j option, and
// we are the only job running, we will get one less token than the option
// specified on -j.
func (cl *Client) ExpectedJobs() int {
	x := cl.jobs
	if x >= 2 {
		x--
	}
	return x + cl.maxLocalJobs
}
