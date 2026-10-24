// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// InvokedAsEnv names the variable the rhythm router sets when it runs a
// tool, holding what the user typed ("rhythm soundcheck"), so the tool's
// usage lines and hints show a command that works.
const InvokedAsEnv = "RHYTHM_INVOKED_AS"

// Name returns the command name for usage lines and hints: the value of
// RHYTHM_INVOKED_AS when set, else the program's file name without a
// directory or ".exe" ("rhythm-tuner").
func Name() string {
	if v := strings.TrimSpace(os.Getenv(InvokedAsEnv)); v != "" {
		return v
	}
	if len(os.Args) == 0 {
		return ""
	}
	base := filepath.Base(os.Args[0])
	if strings.EqualFold(filepath.Ext(base), ".exe") {
		base = base[:len(base)-len(".exe")]
	}
	return base
}
