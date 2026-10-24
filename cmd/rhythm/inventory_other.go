// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !unix

package main

import "os/exec"

// probeInGroup leaves cmd as it is: outside Unix, exec.CommandContext's
// kill of the direct child is all rhythm does at the timeout.
func probeInGroup(*exec.Cmd) {}
