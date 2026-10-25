// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !windows

package main

import "errors"

// deleteAtRestart is Windows only: elsewhere a running program can be
// deleted, so nothing is left to remove later.
var deleteAtRestart = func(string) error { return errors.ErrUnsupported }
