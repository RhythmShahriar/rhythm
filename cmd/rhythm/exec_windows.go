// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build windows

package main

import "os"

// forwardSignals is empty: Windows has no signals to pass on, and the
// console already delivers Ctrl-C and Ctrl-Break to the child.
var forwardSignals []os.Signal

// execTool does nothing on Windows, which cannot replace a running
// process; the caller runs the tool as a child.
func execTool(string, []string) {}
