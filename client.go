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
	r    *os.File
	w    *os.File
	m    sync.Mutex
	jobs int
	tks  chan Token
}

type Token struct {
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
		cl.tks = make(chan Token, 100)
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
				cl.tks <- Token{t: p[0]}
			}
		}()
	}
	return
}

func (cl *Client) GetToken() (t Token) {
	//	if cl.r == nil {
	//	cl.m.Lock()
	//return Token{}
	//}
	t = <-cl.tks
	return
}

func (cl *Client) PutToken(t Token) {
	if cl.r == nil {
		return
	}
	cl.tks <- t
}

func (cl *Client) FlushTokens() {
	if cl.r == nil {
		return
	}
	cl.r.Close()

	for {
		select {
		case tk := <-cl.tks:
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

func (cl *Client) TksLen() int {
	return len(cl.tks)
}
