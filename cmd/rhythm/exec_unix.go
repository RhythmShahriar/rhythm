// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build unix

package main

import (
	"os"
	"syscall"
)

// forwardSignals are passed on to a child tool when rhythm cannot exec.
var forwardSignals = []os.Signal{syscall.SIGTERM, syscall.SIGHUP}

// execTool replaces rhythm with the tool, so the tool gets the PID, the
// signals and the exit status rhythm would have had: Ctrl-C, docker stop
// and CI cancels reach it directly, the prompt only returns once it has
// cleaned up, and nothing is left orphaned. It returns only if the exec
// failed, and the caller then runs the tool as a child instead.
func execTool(bin string, args []string) {
	_ = syscall.Exec(bin, append([]string{bin}, args...), os.Environ()) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
}
