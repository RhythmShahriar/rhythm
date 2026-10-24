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

func TestInstallRelativeDirOnPathHasNoNote(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	t.Chdir(s.bin)
	code, out, errs := runCmd("install", "tuner", "--dir", ".")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if want := "Installed rhythm-tuner 0.2.0 to " + filepath.Join(s.bin, "rhythm-tuner") + "\n"; out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestInstallNotesARealShadow(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	other := filepath.Join(s.dir, "other")
	os.Mkdir(other, 0o755)
	shadow := filepath.Join(other, "rhythm-tuner")
	os.WriteFile(shadow, []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", other+string(os.PathListSeparator)+s.bin)

	code, out, errs := runCmd("install", "tuner", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "Note: "+shadow+" comes first on your PATH; remove it to use this install.\n") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	// A link on PATH that points at the new install is not a shadow.
	os.Remove(shadow)
	os.Symlink(filepath.Join(s.bin, "rhythm-tuner"), shadow)
	if code, out, _ := runCmd("install", "tuner", "--dir", s.bin); code != 0 || strings.Contains(out, "Note:") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestInstallCreatesMissingDir(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	dir := filepath.Join(s.dir, "a", "b", "c")
	code, out, errs := runCmd("install", "tuner", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "rhythm-tuner")); err != nil {
		t.Error(err)
	}
	if !strings.Contains(out, "Note: "+dir+" isn't on your PATH.\n") {
		t.Errorf("no PATH note:\n%s", out)
	}
}

func TestInstallBadDirFailsBeforeDownload(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	file := filepath.Join(s.dir, "file")
	os.WriteFile(file, nil, 0o644)
	code, _, errs := runCmd("install", "tuner", "--dir", file)
	if code != 1 || errs != "rhythm: "+file+" is a file, not a folder; pick a folder with --dir\n" {
		t.Errorf("exit %d: %q", code, errs)
	}

	if os.Geteuid() != 0 { // root can write anywhere
		ro := filepath.Join(s.dir, "ro")
		os.Mkdir(ro, 0o555)
		code, _, errs = runCmd("install", "tuner", "--dir", ro)
		if code != 1 || !strings.Contains(errs, "can't write to "+ro+"; rerun with sudo, or install to a folder you own with --dir") {
			t.Errorf("exit %d: %q", code, errs)
		}
	}
	if r := s.api.requests(); len(r) != 0 {
		t.Errorf("requests before the folder was checked: %v", r)
	}
}

func TestInstallQuiet(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	dir := filepath.Join(s.dir, "off-path")
	code, out, errs := runCmd("install", "tuner", "--dir", dir, "--quiet")
	if want := "Installed rhythm-tuner 0.2.0 to " + filepath.Join(dir, "rhythm-tuner") + "\n"; code != 0 || out != want || errs != "" {
		t.Errorf("exit %d: %q %q", code, out, errs)
	}
}
