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
var localCount = flag.Uint("local", 0, "Number of local clients")
var recurseFlag = flag.Bool("recurse", false, "call ourselves recursively")
var serveCount = flag.Uint("serve", 1, "number of jobs to serve")
var sleepTime = flag.Uint("sleep", 500, "milliseconds to sleep with tokens")
var tokenCount = flag.Uint("tokens", 1, "number of tokens to allocate")

func main() {
	fmt.Printf("Hello world: %v\n", os.Args)

	flag.Parse()

	cl, err := jobserver.NewClient(int(*localCount))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tokens := 0
	for i := uint(0); i < *tokenCount; i++ {
		cl.GetToken()
		tokens++
		fmt.Printf("%s: Got token %d\n", os.Args[0], i)
	}

	var cmd *exec.Cmd
	var srv *jobserver.Server
	if *recurseFlag {
		cmd = exec.Command("/proc/self/exe", "-tokens",
			strconv.FormatUint(uint64(*clientCount), 10))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		srv, err = jobserver.SetupServer(cmd, cl, 10)
		if err != nil {
			panic(err)
		}
		err = cmd.Start()
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("%s: P1\n", os.Args[0])
	time.Sleep(time.Duration(*sleepTime) * time.Millisecond)

	for tokens > 0 {
		cl.PutToken()
		tokens--
	}

	fmt.Printf("%s: P2\n", os.Args[0])
	time.Sleep(time.Duration(*sleepTime) * time.Millisecond)

	for tokens > 0 {
		cl.PutToken()
		tokens--
	}

	fmt.Printf("%s: P3\n", os.Args[0])
	time.Sleep(time.Duration(*sleepTime) * time.Millisecond)

	if srv != nil {
		srv.DisableJobs()
	}
	fmt.Printf("%s: P4\n", os.Args[0])
	cl.FlushTokens()

	fmt.Printf("%s: P5\n", os.Args[0])
	time.Sleep(time.Duration(*sleepTime) * time.Millisecond)

	if *recurseFlag {
		fmt.Printf("%s: P6\n", os.Args[0])
		err := cmd.Wait()
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: P6\n", os.Args[0])
	}
	fmt.Printf("%s: Exiting with tokens=%d and Tokens()=%d\n",
		os.Args[0], tokens, cl.Tokens())

}
