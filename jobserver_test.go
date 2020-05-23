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
	cl, err := NewClient()
	if err != nil {
		t.Error("parseMakeflags:", err)
		return
	}

	tks := []Token{}
	var m sync.Mutex
	done := make(chan bool)
	for i := 0; i < 2*(cl.jobs+1); i++ {
		go func() {
			select {
			case tk := <-cl.tks:
				m.Lock()
				tks = append(tks, tk)
				m.Unlock()
			case <-done:
				return
			}
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(done)
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
	for len(tks) != 0 {
		tk := tks[0]
		tks = tks[1:]
		cl.PutToken(tk)
	}
	m.Unlock()
	time.Sleep(100 * time.Millisecond)
	m.Lock() //
	for len(tks) != 0 {
		tk := tks[0]
		tks = tks[1:]
		cl.PutToken(tk)
	}
	m.Unlock()
	cl.FlushTokens()
	time.Sleep(100 * time.Millisecond)
}
