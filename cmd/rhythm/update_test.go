// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateToolOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in for the tool")
	}
	srv := fakeRelease(t, "rhythm-tuner", "0.2.0", []byte("#!/bin/sh\necho rhythm-tuner 0.2.0\n"))
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)
	dir := t.TempDir()
	tool := filepath.Join(dir, "rhythm-tuner")
	// An installed 0.1.0 that also answers "update" (its data refresh).
	os.WriteFile(tool, []byte("#!/bin/sh\n[ \"$1\" = update ] && echo data refreshed && exit 0\necho rhythm-tuner 0.1.0\n"), 0o755)
	t.Setenv("PATH", dir)

	var out, errb bytes.Buffer
	run([]string{"update", "--check"}, &out, &errb)
	if !strings.Contains(out.String(), "rhythm-tuner       0.1.0 → 0.2.0 available") {
		t.Errorf("--check output:\n%s%s", out.String(), errb.String())
	}
	if got, _ := os.ReadFile(tool); !strings.Contains(string(got), "0.1.0") {
		t.Error("--check changed the binary")
	}

	out.Reset()
	run([]string{"update"}, &out, &errb)
	if got, _ := os.ReadFile(tool); !strings.Contains(string(got), "0.2.0") {
		t.Errorf("tool not updated:\n%s%s", out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "0.1.0 → 0.2.0 ✔") {
		t.Errorf("output:\n%s", out.String())
	}
}
