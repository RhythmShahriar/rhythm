// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHelp(t *testing.T) {
	code, out, errs := runCmd("install", "--help")
	if code != 0 || errs != "" || !strings.Contains(out, "--dir dir") || strings.Count(out, "Usage:") != 1 {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestInstallSuggestsName(t *testing.T) {
	code, _, errs := runCmd("install", "soundchek", "--dir", t.TempDir())
	if code != 2 || !strings.Contains(errs, `unknown tool "soundchek" (did you mean soundcheck?)`) {
		t.Errorf("exit %d: %s", code, errs)
	}
	// rhythm installs tools; it doesn't install itself.
	if code, _, _ := runCmd("install", "rhythm", "--dir", t.TempDir()); code != 2 {
		t.Errorf("install rhythm: exit %d", code)
	}
}

func TestSuggest(t *testing.T) {
	for in, want := range map[string]string{
		"tunr":         " (did you mean tuner?)",
		"rhythm-tunre": " (did you mean tuner?)",
		"rhytm":        " (did you mean rhythm?)",
		"ecoh":         " (did you mean echo?)",
		"zzzzzzzz":     "",
		"x":            "",
	} {
		if got := suggest(in); got != want {
			t.Errorf("suggest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInstallFollowsSymlinkAndRefusesManaged(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	opt := filepath.Join(s.dir, "opt")
	os.Mkdir(opt, 0o755)
	os.WriteFile(filepath.Join(opt, "rhythm-tuner"), []byte("old"), 0o755)
	link := filepath.Join(s.bin, "rhythm-tuner")
	os.Symlink("../opt/rhythm-tuner", link)

	code, out, errs := runCmd("install", "tuner", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.2.0 to "+link) || strings.Contains(out, "Note:") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced by a file: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(opt, "rhythm-tuner")); !strings.Contains(string(got), "0.2.0") {
		t.Errorf("the link target wasn't written: %s", got)
	}

	cellar := filepath.Join(s.dir, "Cellar", "rhythm-echo", "0.1.0", "bin")
	os.MkdirAll(cellar, 0o755)
	os.WriteFile(filepath.Join(cellar, "rhythm-echo"), []byte("brew's"), 0o755)
	os.Symlink(filepath.Join(cellar, "rhythm-echo"), filepath.Join(s.bin, "rhythm-echo"))
	before := len(s.api.requests())
	code, _, errs = runCmd("install", "echo", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "rhythm-echo is managed by Homebrew; update it with: brew upgrade rhythm-echo") {
		t.Errorf("exit %d: %s", code, errs)
	}
	if len(s.api.requests()) != before {
		t.Error("a managed install was downloaded")
	}
	if got, _ := os.ReadFile(filepath.Join(cellar, "rhythm-echo")); string(got) != "brew's" {
		t.Error("the managed binary was overwritten")
	}
}
