// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !unix

package main

// cleanupBuilt has nothing to remove here: the exec tests that build the
// router run on unix only.
func cleanupBuilt() {}
