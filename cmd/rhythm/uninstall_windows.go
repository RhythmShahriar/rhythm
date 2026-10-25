// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import "golang.org/x/sys/windows"

// deleteAtRestart asks Windows to delete path at the next restart. Only
// an administrator may; anyone else gets an error and deletes it by hand.
var deleteAtRestart = func(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}
