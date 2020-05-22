// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/platinasystems/jobserver"
)

var clientCount = flag.Uint("client", 1, "tokens to allocate in recursive client")
var recurseFlag = flag.Bool("recurse", false, "call ourselves recursively")
var serveCount = flag.Uint("serve", 1, "number of jobs to serve")
var sleepTime = flag.Uint("sleep", 1, "seconds to sleep with tokens")
var tokenCount = flag.Uint("tokens", 1, "number of tokens to allocate")

func main() {
	fmt.Printf("Hello world: %v\n", os.Args)

	flag.Parse()

	cl, err := jobserver.ParseMakeflags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tks := []jobserver.Token{}
	for i := uint(0); i < *tokenCount; i++ {
		tks = append(tks, cl.GetToken())
		fmt.Printf("Got token %d\n", i)
	}

	var cmd *exec.Cmd
	if *recurseFlag {
		cmd = exec.Command("/proc/self/exe", "-tokens",
			strconv.FormatUint(uint64(*clientCount), 10))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_, err := jobserver.SetupServer(cmd, 10)
		if err != nil {
			panic(err)
		}
		err = cmd.Start()
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Duration(*sleepTime) * time.Second)

	for _, tk := range tks {
		cl.PutToken(tk)
	}

	time.Sleep(time.Duration(*sleepTime) * time.Second)

	if *recurseFlag {
		err := cmd.Wait()
		if err != nil {
			panic(err)
		}
	}
}