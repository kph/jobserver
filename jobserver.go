// Copyright © 2020 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package jobserver

import (
	"errors"
)

var ErrBadMakeflags = errors.New("Invalid format in MAKEFLAGS")
var ErrNotRecursiveMake = errors.New("Make rule not marked as recursive")
