// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// A Client tracks the jobserver controlling us, i.e. our parent. It
// is also used in the case where we are the parent.
type Client struct {
	r          *os.File   // Pipe from parent giving us tokens
	w          *os.File   // Pipe to parent returning tokens
	m          sync.Mutex // Serialize access to fields below
	jobs       int        // Count of jobs from MAKEFLAGS -j option
	freeTokens chan token // Tokens we've been given but aren't using
	usedTokens []token    // Tokens that are currently in use
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
// return an error.
func NewClient() (cl *Client, err error) {
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
	if cl.r != nil {
		cl.freeTokens = make(chan token, 100)
		cl.usedTokens = make([]token, 0)
		go func() {
			p := make([]byte, 1)
			for {
				n, err := cl.r.Read(p)
				if err != nil {
					//panic(err)
					return
				}
				if n != 1 {
					panic("Unexpected byte count")
				}
				cl.freeTokens <- token{t: p[0]}
			}
		}()
	}
	return
}

func (cl *Client) GetToken() {
	//	if cl.r == nil {
	//	cl.m.Lock()
	//return token{}
	//}
	t := <-cl.freeTokens
	cl.saveUsedToken(t)
	fmt.Printf("%s: GetToken() free %d saved %d\n", os.Args[0],
		len(cl.freeTokens), len(cl.usedTokens))
	return
}

func (cl *Client) saveUsedToken(t token) {
	cl.m.Lock()
	defer cl.m.Unlock()
	cl.usedTokens = append(cl.usedTokens, t)
}

func (cl *Client) PutToken() {
	//if cl.r == nil {
	//	return
	//}
	fmt.Printf("%s: PutToken() free %d saved %d\n", os.Args[0],
		len(cl.freeTokens), len(cl.usedTokens))
	cl.m.Lock()
	t := cl.usedTokens[len(cl.usedTokens)-1]
	cl.usedTokens = cl.usedTokens[:len(cl.usedTokens)-1]
	cl.m.Unlock()
	cl.freeTokens <- t
}

func (cl *Client) FlushTokens() {
	if cl.r == nil {
		return
	}
	cl.r.Close()
	fmt.Printf("%s: FlushTokens free %d saved %d\n", os.Args[0],
		len(cl.freeTokens), len(cl.usedTokens))
	for {
		select {
		case tk := <-cl.freeTokens:
			fmt.Printf("%s: FlushTokens select loop free %d saved %d\n",
				os.Args[0], len(cl.freeTokens),
				len(cl.usedTokens))
			n, err := cl.w.Write([]byte{tk.t})
			if err != nil {
				panic(err)
			}
			if n != 1 {
				panic("Unexpected byte count")
			}
		default:
			return
		}
	}
}

// Tokens() returns the count of available tokens.
func (cl *Client) Tokens() int {
	return len(cl.freeTokens)
}
