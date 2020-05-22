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
	fmt.Println("In GetToken", len(cl.tks))
	t = <-cl.tks
	fmt.Println("Done GetToken", len(cl.tks))
	return t
}

func (cl *Client) PutToken(t Token) {
	if cl.r == nil {
		return
	}
	fmt.Println("PutToken")
	cl.tks <- t
	fmt.Println("Done pUtToken len", len(cl.tks))
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
