// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stub writes a shell script named rhythm-<name> into dir that runs body.
func stub(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in for the tool")
	}
	p := filepath.Join(dir, "rhythm-"+name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnknownTool(t *testing.T) {
	var out, errb bytes.Buffer
	t.Setenv("PATH", t.TempDir())
	if code := run([]string{"tuner"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "rhythm install tuner") {
		t.Errorf("exit %d: %s", code, errb.String())
	}
}

func TestInvalidToolName(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("PATH", cwd)
	marker := filepath.Join(t.TempDir(), "ran")
	// A planted file at each path an unchecked lookup would reach.
	for _, p := range []string{"/x", "../x", "a/b"} {
		stub(t, cwd, p, ": >"+marker) // a builtin, as PATH has no touch
	}
	for _, name := range []string{"/x", "../x", "a/b", "-x", "", "UPPER", strings.Repeat("a", 5000), "日本"} {
		var out, errb bytes.Buffer
		code := run([]string{name}, &out, &errb)
		if code != 2 || errb.String() != fmt.Sprintf("rhythm: invalid tool name %q\n", name) {
			t.Errorf("%.20q: exit %d: %.80s", name, code, errb.String())
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a planted file ran")
	}
}

func TestValidToolNamesRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stub(t, dir, "tuner", `echo tuner "$@"`)
	stub(t, dir, "my-tool2", `echo my-tool2 "$@"`)
	for _, args := range [][]string{{"tuner", "scan"}, {"t", "scan"}, {"my-tool2", "scan"}} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 || !strings.HasSuffix(out.String(), " scan\n") {
			t.Errorf("%v: exit %d: %s%s", args, code, out.String(), errb.String())
		}
	}
}

func TestSoundcheckRegistered(t *testing.T) {
	sc, ok := lookupTool("sc")
	if !ok || sc.Name != "soundcheck" || sc.Repo != "RhythmShahriar/soundcheck" {
		t.Errorf("soundcheck isn't registered: %+v", sc)
	}
	var out bytes.Buffer
	usage(&out)
	if !strings.Contains(out.String(), "soundcheck (sc)") {
		t.Error("usage doesn't list soundcheck")
	}
}
