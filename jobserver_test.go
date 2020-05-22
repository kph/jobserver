// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestJobserver(t *testing.T) {
	if _, x := os.LookupEnv("MAKEFLAGS"); !x {
		t.Error("not run under make - type make to test")
		return
	}
	cl, err := ParseMakeflags()
	if err != nil {
		t.Error("parseMakeflags:", err)
		return
	}

	tks := []Token{}
	var m sync.Mutex
	quitting := false
	for i := 0; i < 2*(cl.jobs+1); i++ {
		go func() {
			tk := cl.GetToken()
			m.Lock()
			if !quitting {
				tks = append(tks, tk)
			} else {
				cl.PutToken(tk)
			}
			m.Unlock()
		}()
	}
	time.Sleep(1 * time.Second)
	fmt.Printf("Jobs %d Tokens %d\n", cl.jobs, len(tks))
	expected := cl.jobs - 1
	if cl.jobs <= 2 {
		expected = 1
	}
	if len(tks) != expected {
		t.Error("Jobs", cl.jobs, "Expected", expected, "Tokens",
			len(tks))
		return
	}
	m.Lock() // Block goroutines from our free
	for _, tk := range tks {
		cl.PutToken(tk)
	}
	quitting = true
	m.Unlock()
	time.Sleep(1 * time.Second)
}
