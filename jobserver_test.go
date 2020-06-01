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

	tokens := 0
	var m sync.Mutex
	done := false
	for i := 0; i < 2*(cl.jobs+1); i++ {
		go func() {
			for {
				cl.GetToken()
				m.Lock()
				if done {
					fmt.Printf("Tokens %d while done\n",
						tokens)
					cl.PutToken()
					m.Unlock()
					return
				}
				tokens++
				m.Unlock()
			}
		}()
	}
	time.Sleep(100 * time.Millisecond)
	m.Lock()
	done = true
	m.Unlock()
	fmt.Printf("Jobs %d Tokens %d\n", cl.jobs, tokens)
	expected := cl.jobs - 1
	if cl.jobs <= 2 {
		expected = 1
	}
	if tokens != expected {
		t.Error("Jobs", cl.jobs, "Expected", expected, "Tokens",
			tokens)
		return
	}
	m.Lock() // Block goroutines from our free
	for tokens > 0 {
		cl.PutToken()
		tokens--
	}
	m.Unlock()
	time.Sleep(100 * time.Millisecond)
	m.Lock() //
	for tokens > 0 {
		cl.PutToken()
		tokens--
	}
	m.Unlock()
	cl.FlushTokens()
	time.Sleep(100 * time.Millisecond)
}
