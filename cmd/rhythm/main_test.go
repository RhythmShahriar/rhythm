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

func TestExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, dir string) // fills the fake PATH
		args     []string
		code     int
		inStderr string
	}{
		{"known tool missing", func(*testing.T, string) {}, []string{"tuner"}, 3, "install it with: rhythm install tuner"},
		{"alias of a missing tool", func(*testing.T, string) {}, []string{"t"}, 3, "install it with: rhythm install tuner"},
		{"unknown tool", func(*testing.T, string) {}, []string{"nope"}, 2, `unknown tool "nope"`},
		{"tool file not executable", func(t *testing.T, dir string) {
			os.WriteFile(filepath.Join(dir, "rhythm-tuner"), []byte("#!/bin/sh\nexit 0\n"), 0o644)
		}, []string{"tuner"}, 3, "install it with: rhythm install tuner"},
		{"tool file can't start", func(t *testing.T, dir string) {
			os.WriteFile(filepath.Join(dir, "rhythm-tuner"), []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0}, 0o755)
		}, []string{"tuner"}, 3, "can't start tuner"},
		{"findings pass through", func(t *testing.T, dir string) { stub(t, dir, "tuner", "exit 1") }, []string{"tuner"}, 1, ""},
		{"any code passes through", func(t *testing.T, dir string) { stub(t, dir, "tuner", "exit 42") }, []string{"tuner"}, 42, ""},
		{"success", func(t *testing.T, dir string) { stub(t, dir, "tuner", "exit 0") }, []string{"tuner"}, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("unix file modes and shell stubs")
			}
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			c.setup(t, dir)
			var out, errb bytes.Buffer
			code := run(c.args, &out, &errb)
			if code != c.code || !strings.Contains(errb.String(), c.inStderr) {
				t.Errorf("exit %d (want %d): %s", code, c.code, errb.String())
			}
		})
	}
}

func TestUsageListsExitCodes(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	for _, line := range []string{"Exit codes:", "  2     usage error or unknown tool", "  3     tool not installed or can't start", "  130   interrupted"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("usage lacks %q", line)
		}
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

func TestExitCodeMapping(t *testing.T) {
	cases := []struct{ code, sig, want int }{
		{0, -1, 0},
		{1, -1, 1},
		{42, -1, 42},
		{-1, 2, 130},  // SIGINT
		{-1, 15, 143}, // SIGTERM
		{-1, 9, 137},  // SIGKILL
		{0xC000013A, -1, 130},
	}
	for _, c := range cases {
		if got := exitCode(c.code, c.sig); got != c.want {
			t.Errorf("exitCode(%#x, %d) = %d, want %d", c.code, c.sig, got, c.want)
		}
	}
}
