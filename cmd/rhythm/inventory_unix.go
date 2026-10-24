// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// probeInGroup starts cmd in a process group of its own and makes the
// timeout kill the whole group. A wrapper script (an asdf or mise shim,
// say) runs the real tool as its child and killing only the script
// would leave that child running after rhythm exits.
func probeInGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
