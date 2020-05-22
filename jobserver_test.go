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
	}
	js, err := parseMakeflags()
	if err != nil {
		t.Error("parseMakeflags:", err)
	}

	tks := []Token{}
	var m sync.Mutex
	quitting := false
	for i := 0; i < 20; i++ {
		go func() {
			tk := js.GetToken()
			m.Lock()
			if !quitting {
				tks = append(tks, tk)
				fmt.Printf("Got token %x\n", tk.t)
			} else {
				fmt.Printf("Returning token %x\n", tk.t)
				js.PutToken(tk)
			}
			m.Unlock()
		}()
	}
	time.Sleep(1 * time.Second)
	fmt.Printf("Tokens %d\n", len(tks))
	m.Lock() // Block goroutines from our free
	for i, tk := range tks {
		fmt.Printf("Putting token %x\n", tk.t)
		js.PutToken(tk)
		fmt.Printf("Put token %d\n", i)
	}
	quitting = true
	m.Unlock()
	time.Sleep(1 * time.Second)
}
