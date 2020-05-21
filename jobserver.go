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
	"syscall"
)

var ErrBadMakeflags = errors.New("Invalid format in MAKEFLAGS")
var ErrNotRecursiveMake = errors.New("Make rule not marked as recursive")

func pipeFdToFile(fd int, name string) *os.File {
	var stats syscall.Stat_t

	err := syscall.Fstat(fd, &stats)
	if err == nil && ((stats.Mode & syscall.S_IFIFO) != 0) {
		return os.NewFile(uintptr(fd), name)
	}
	return nil
}

func parseMakeflags() (err error) {
	mflags := strings.Fields(os.Getenv("MAKEFLAGS"))

	for _, mflag := range mflags {
		fmt.Println(mflag)
		if strings.HasPrefix(mflag, "--jobserver-auth=") {
			s := strings.Split(strings.TrimPrefix(
				mflag, "--jobserver-auth="), ",")
			if len(s) != 2 {
				return ErrBadMakeflags
			}
			r, err := strconv.Atoi(s[0])
			if err != nil {
				return ErrBadMakeflags
			}
			w, err := strconv.Atoi(s[1])
			if err != nil {
				return ErrBadMakeflags
			}
			fmt.Printf("R = %d W = %d\n", r, w)
			rFile := pipeFdToFile(r, "Jobserver-R")
			if rFile == nil {
				return ErrNotRecursiveMake
			}
			wFile := pipeFdToFile(r, "Jobserver-W")
			if wFile == nil {
				return ErrNotRecursiveMake
			}
			return nil
		}
	}
	return nil
}
