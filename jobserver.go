// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var ErrBadMakeflags = errors.New("Invalid format in MAKEFLAGS")
var ErrNotRecursiveMake = errors.New("Make rule not marked as recursive")

type Client struct {
	r *os.File
	w *os.File
	m sync.Mutex
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

func parseMakeflags() (js *Client, err error) {
	mflags := strings.Fields(os.Getenv("MAKEFLAGS"))

	for _, mflag := range mflags {
		fmt.Println(mflag)
		if strings.HasPrefix(mflag, "--jobserver-auth=") {
			s := strings.Split(strings.TrimPrefix(
				mflag, "--jobserver-auth="), ",")
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
			rFile := pipeFdToFile(r, "Jobserver-R")
			if rFile == nil {
				return nil, ErrNotRecursiveMake
			}
			wFile := pipeFdToFile(w, "Jobserver-W")
			if wFile == nil {
				return nil, ErrNotRecursiveMake
			}
			return &Client{r: rFile, w: wFile}, nil
		}
	}
	return &Client{}, nil
}

func (j *Client) GetToken() (t Token) {
	if j.r == nil {
		j.m.Lock()
		return Token{}
	}
	p := make([]byte, 1)
	n, err := j.r.Read(p)
	if err != nil {
		panic(err)
	}
	if n != 1 {
		panic("Unexpected byte count")
	}
	return Token{t: p[0]}
}

func (j *Client) PutToken(t Token) {
	if j.r == nil {
		j.m.Unlock()
		return
	}
	n, err := j.w.Write([]byte{t.t})
	if err != nil {
		panic(err)
	}
	if n != 1 {
		panic("Unexpected byte count")
	}
	j.w.Sync()
}
