// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"os"
	"testing"
)

func TestName(t *testing.T) {
	args := os.Args
	t.Cleanup(func() { os.Args = args })

	t.Setenv(InvokedAsEnv, "rhythm soundcheck")
	if got := Name(); got != "rhythm soundcheck" {
		t.Errorf("with %s: %q", InvokedAsEnv, got)
	}

	t.Setenv(InvokedAsEnv, "")
	for arg0, want := range map[string]string{
		"/usr/local/bin/rhythm-tuner": "rhythm-tuner",
		"rhythm-echo":                 "rhythm-echo",
		"rhythm-soundcheck.exe":       "rhythm-soundcheck",
		"RHYTHM-TUNER.EXE":            "RHYTHM-TUNER",
	} {
		os.Args = []string{arg0}
		if got := Name(); got != want {
			t.Errorf("Name() with argv[0] %q = %q, want %q", arg0, got, want)
		}
	}
	os.Args = nil
	if got := Name(); got != "" {
		t.Errorf("no argv: %q", got)
	}
}
