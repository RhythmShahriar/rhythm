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

// stubAs sets the -ldflags variables for one test.
func stubAs(t *testing.T, v, ts, dry, block string) {
	t.Helper()
	old := []string{version, tools, dryRun, toolsBlock}
	version, tools, dryRun, toolsBlock = v, ts, dry, block
	t.Cleanup(func() { version, tools, dryRun, toolsBlock = old[0], old[1], old[2], old[3] })
}

func call(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionAndHelp(t *testing.T) {
	stubAs(t, "0.4.0", "tuner:t,soundcheck:sc", "no", "yes")
	if code, out, _ := call("--version"); code != 0 || out != "rhythm 0.4.0\n" {
		t.Fatalf("--version = %d %q", code, out)
	}
	_, out, _ := call("help")
	if !strings.Contains(out, "Tools:\n  tuner (t)  a tool\n  soundcheck (sc)  a tool\n\n") {
		t.Fatalf("help has no Tools block:\n%s", out)
	}

	stubAs(t, "0.3.5", "tuner:t", "no", "no")
	if _, out, _ := call(); strings.Contains(out, "Tools:") {
		t.Fatalf("help has a Tools block:\n%s", out)
	}
}

func TestDryRun(t *testing.T) {
	stubAs(t, "0.6.0", "tuner:t,echo:e", "yes", "yes")
	if code, out, _ := call("install", "--dry-run", "tuner", "e"); code != 0 || out != "would install tuner\nwould install e\n" {
		t.Fatalf("dry run = %d %q", code, out)
	}
	if code, _, errOut := call("install", "--dry-run", "bogus"); code != 2 || !strings.Contains(errOut, `unknown tool "bogus"`) {
		t.Fatalf("unknown = %d %q", code, errOut)
	}

	stubAs(t, "0.5.0", "tuner:t", "no", "yes")
	if code, _, errOut := call("install", "--dry-run", "tuner"); code != 2 || !strings.Contains(errOut, "flag provided but not defined: -dry-run") {
		t.Fatalf("old router = %d %q", code, errOut)
	}
}

func TestInstallCopiesItselfAndLogs(t *testing.T) {
	stubAs(t, "0.6.0", "tuner:t", "yes", "yes")
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	t.Setenv("RHYTHM_TEST_LOG", log)
	if code, _, errOut := call("install", "tuner", "--dir", dir); code != 0 {
		t.Fatalf("install = %d %q", code, errOut)
	}
	name := "rhythm-tuner"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(log)
	if err != nil || string(got) != "rhythm install tuner --dir "+dir+"\n" {
		t.Fatalf("log = %q, %v", got, err)
	}
	if code, _, _ := call("bogus"); code != 1 {
		t.Fatalf("unknown command = %d", code)
	}
}
