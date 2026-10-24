// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"testing"
)

func TestParseInterspersed(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	var tags ListFlag
	fs.Var(&tags, "tags", "")
	pos, err := ParseInterspersed(fs, []string{"a.example", "--json", "b.example", "--tags", "x, y", "--tags", "z"})
	if err != nil || !*asJSON || !slices.Equal(pos, []string{"a.example", "b.example"}) || !slices.Equal(tags, ListFlag{"x", "y", "z"}) {
		t.Errorf("pos=%v json=%v tags=%v err=%v", pos, *asJSON, tags, err)
	}
}

func TestUsageError(t *testing.T) {
	var ue UsageError
	if !errors.As(Usagef("bad %s", "flag"), &ue) || ue.Msg != "bad flag" {
		t.Errorf("got %v", ue)
	}
	if IsTerminal(&bytes.Buffer{}) {
		t.Error("a buffer isn't a terminal")
	}
}

func TestExitCodes(t *testing.T) {
	// These values are a public contract with scripts and CI; changing one
	// breaks every pipeline that checks it.
	got := []int{ExitOK, ExitFindings, ExitUsage, ExitUnavailable, ExitInterrupted}
	if !slices.Equal(got, []int{0, 1, 2, 3, 130}) {
		t.Errorf("exit codes changed: %v", got)
	}
}
