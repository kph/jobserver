// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// Package jobserver implements the POSIX Jobserver used by GNU Make.
// See https://www.gnu.org/software/make/manual/html_node/POSIX-Jobserver.html
// for a description of the protocol.
//
// Both the client and server are implemented. A go tool can use this to
// manage execution tokens passed by a controller (generally GNU Make)
// or manage tools (such as GNU Make or any other Jobserver-compatible
// application.
package jobserver

import (
	"errors"
)

// ErrBadMakeflags is returned when there was a parsing error reading
// the MAKEFLAGS environment variable
var ErrBadMakeflags = errors.New("Invalid format in MAKEFLAGS")

// ErrNotRecursiveMake occurs when we are started with a jobserver
// argument in MAKEFLAGS, but the referenced file descriptors were not
// open. This will happen if the rule to invoke this tool was not
// started with the "+" character at the start of the Make rule.
var ErrNotRecursiveMake = errors.New("Make rule not marked as recursive")
