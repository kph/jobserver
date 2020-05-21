// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"os"
	"testing"
)

func TestJobserver(t *testing.T) {
	if _, x := os.LookupEnv("MAKEFLAGS"); !x {
		t.Error("not run under make - type make to test")
	}
	js, err := parseMakeflags()
	if err != nil {
		t.Error("parseMakeflags:", err)
	}
	tk := js.GetToken()
	js.PutToken(tk)
}
