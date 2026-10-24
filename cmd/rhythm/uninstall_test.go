// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/config"
)

// files lists the names in dir, sorted.
func files(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallRemovesExactlyTheTool(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	touch(t, filepath.Join(s.bin, "rhythm-tuner.prev"))
	touch(t, filepath.Join(s.bin, "rhythm-tuner.old"))
	touch(t, filepath.Join(s.bin, "rhythm-tuner-notes"))

	code, out, errs := runCmd("uninstall", "t")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	want := []string{"rhythm-echo", "rhythm-soundcheck", "rhythm-tuner-notes"}
	if got := files(t, s.bin); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("left %v, want %v", got, want)
	}
	p := filepath.Join(s.bin, "rhythm-tuner")
	if out != "Removed "+p+"\nRemoved "+p+".prev\nRemoved "+p+".old\n" {
		t.Errorf("stdout:\n%s", out)
	}
}

func TestUninstallFollowsSymlink(t *testing.T) {
	s := newSandbox(t, nil, nil)
	opt := filepath.Join(s.dir, "opt")
	real := script(t, mkdir(t, opt), "rhythm-tuner", "echo rhythm-tuner 0.1.0")
	link := filepath.Join(s.bin, "rhythm-tuner")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// The target is named as it resolves: on macOS the temp folder is
	// under /var, which is a link to /private/var.
	target := resolve(real)
	code, out, errs := runCmd("uninstall", "tuner")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if len(files(t, s.bin)) != 0 || len(files(t, opt)) != 0 {
		t.Errorf("left %v and %v", files(t, s.bin), files(t, opt))
	}
	if out != "Removed "+link+"\nRemoved "+target+"\n" {
		t.Errorf("stdout:\n%s", out)
	}
}

func mkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUninstallRefusesManaged(t *testing.T) {
	s := newSandbox(t, nil, nil)
	cellar := mkdir(t, filepath.Join(s.dir, "Cellar", "rhythm-tuner", "0.3.0", "bin"))
	real := script(t, cellar, "rhythm-tuner", "echo rhythm-tuner 0.3.0")
	if err := os.Symlink(real, filepath.Join(s.bin, "rhythm-tuner")); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runCmd("uninstall", "tuner", "--purge")
	if code != 1 || errs != "rhythm-tuner is managed by Homebrew; remove it with: brew uninstall rhythm-tuner\n" || out != "" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if len(files(t, s.bin)) != 1 || len(files(t, cellar)) != 1 {
		t.Error("a managed install was touched")
	}
}

func TestUninstallPurge(t *testing.T) {
	freshConfig(t)
	installed, latest := allTools()
	newSandbox(t, installed, latest)
	cache, _ := config.CacheDir("tuner")
	echoCache, _ := config.CacheDir("echo")
	cfg, _ := config.Dir()
	touch(t, filepath.Join(cache, "vulns", "db.json"))
	touch(t, filepath.Join(echoCache, "ca.pem"))
	state := filepath.Join(cfg, "update-check", "rhythm-tuner.json")
	touch(t, state)
	touch(t, filepath.Join(cfg, "update-check", "rhythm-echo.json"))
	touch(t, filepath.Join(cfg, "seen", "tool-tuner"))

	code, out, errs := runCmd("uninstall", "--purge", "tuner")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, p := range []string{cache, state} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s is still there", p)
		}
		if !strings.Contains(out, "Removed "+p+"\n") {
			t.Errorf("no Removed line for %s:\n%s", p, out)
		}
	}
	for _, p := range []string{filepath.Join(echoCache, "ca.pem"), filepath.Join(cfg, "update-check", "rhythm-echo.json"), filepath.Join(cfg, "seen", "tool-tuner")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed", p)
		}
	}

	// A tool that is gone already still has its leftovers purged.
	touch(t, filepath.Join(cache, "checks", "pack"))
	code, out, _ = runCmd("uninstall", "--purge", "tuner")
	if code != 0 || !strings.Contains(out, "rhythm-tuner       isn't installed\nRemoved "+cache+"\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	newSandbox(t, map[string]string{"rhythm-tuner": "0.1.0"}, nil)
	code, out, errs := runCmd("uninstall", "echo")
	if code != 0 || out != "rhythm-echo        isn't installed\n" || errs != "" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

// TestUninstallIgnoresCurrentFolder has rhythm-tuner only in the current
// folder, reached through "." on PATH: it isn't installed and stays.
func TestUninstallIgnoresCurrentFolder(t *testing.T) {
	s := newSandbox(t, nil, nil)
	t.Chdir(t.TempDir())
	script(t, ".", "rhythm-tuner", "echo rhythm-tuner 0.1.0")
	t.Setenv("PATH", "."+string(os.PathListSeparator)+s.bin)
	code, out, errs := runCmd("uninstall", "tuner")
	if code != 0 || out != "rhythm-tuner       isn't installed\n" || errs != "" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat("rhythm-tuner"); err != nil {
		t.Errorf("the file in the current folder was removed: %v", err)
	}
}

func TestUninstallAll(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	self := s.stub(t, "rhythm", "0.1.0", "")
	fakeSelf(t, self, "0.1.0")

	if code, _, errs := runCmd("uninstall", "rhythm"); code != 2 || !strings.Contains(errs, "rhythm uninstall --all rhythm") {
		t.Errorf("rhythm without --all: exit %d: %s", code, errs)
	}
	delete(installed, "rhythm-echo")
	if err := os.Remove(filepath.Join(s.bin, "rhythm-echo")); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runCmd("uninstall", "--all")
	if code != 0 || errs != "" || strings.Contains(out, "isn't installed") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if got := files(t, s.bin); len(got) != 1 || got[0] != "rhythm" {
		t.Errorf("--all left %v; want only rhythm", got)
	}

	s.stub(t, "rhythm-tuner", "0.1.0", "")
	code, out, _ = runCmd("uninstall", "rhythm", "--all")
	if code != 0 || len(files(t, s.bin)) != 0 {
		t.Errorf("exit %d, left %v", code, files(t, s.bin))
	}
	if !strings.HasSuffix(out, "Removed "+self+"\n") {
		t.Errorf("rhythm isn't removed last:\n%s", out)
	}
}

func TestUninstallUsage(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	if code, _, errs := runCmd("uninstall"); code != 2 || errs != "usage: rhythm uninstall <tool>... [--purge] [--all]; name a tool, or use --all\n" {
		t.Errorf("no names: exit %d: %s", code, errs)
	}
	if code, _, errs := runCmd("uninstall", "tuner", "tunr", "nope"); code != 2 ||
		!strings.Contains(errs, "unknown tool \"tunr\"\n  did you mean: rhythm uninstall tuner\n") || !strings.Contains(errs, `unknown tool "nope"`) {
		t.Errorf("bad names: exit %d: %s", code, errs)
	}
	if len(files(t, s.bin)) != 3 {
		t.Error("a usage error removed something")
	}
	if code, out, _ := runCmd("uninstall", "--help"); code != 0 || !strings.Contains(out, "--purge") || !strings.Contains(out, "--all") {
		t.Errorf("help: exit %d\n%s", code, out)
	}
}

func TestUninstallNotesAnotherCopy(t *testing.T) {
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.1.0"}, nil)
	later := t.TempDir()
	other := script(t, later, "rhythm-tuner", "echo rhythm-tuner 0.0.9")
	t.Setenv("PATH", s.bin+string(os.PathListSeparator)+later)
	code, out, _ := runCmd("uninstall", "tuner")
	if code != 0 || !strings.HasSuffix(out, "Note: another rhythm-tuner is still on your PATH at "+other+"\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestUninstallCantRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can remove anything")
	}
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.1.0", "rhythm-echo": "0.1.0"}, nil)
	ro := mkdir(t, filepath.Join(s.dir, "ro"))
	script(t, ro, "rhythm-echo", "echo rhythm-echo 0.1.0")
	if err := os.Remove(filepath.Join(s.bin, "rhythm-echo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	t.Setenv("PATH", s.bin+string(os.PathListSeparator)+ro)

	code, _, errs := runCmd("uninstall", "echo", "tuner")
	p := filepath.Join(ro, "rhythm-echo")
	if code != 1 || !strings.Contains(errs, "rhythm-echo        can't remove "+p+"; rerun with sudo\n") ||
		!strings.HasSuffix(errs, "Removed 1, failed 1: rhythm-echo\n") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestManagedByRemove(t *testing.T) {
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("ASDF_DATA_DIR", "")
	t.Setenv("SCOOP", "")
	for path, want := range map[string]string{
		"/usr/local/Cellar/rhythm-tuner/0.3.0/bin/rhythm-tuner":   "brew uninstall rhythm-tuner",
		"/opt/homebrew/Caskroom/rhythm/0.3.0/rhythm":              "brew uninstall --cask rhythm",
		"/opt/homebrew/bin/rhythm":                                "brew uninstall rhythm",
		"/nix/store/abc123-rhythm-0.3.0/bin/rhythm":               "nix profile remove rhythm",
		"/home/me/.local/share/mise/installs/rhythm/0.3.0/rhythm": "mise uninstall rhythm@0.3.0",
		"/home/me/.asdf/installs/rhythm/0.3.0/bin/rhythm":         "asdf uninstall rhythm 0.3.0",
		`C:\Users\me\scoop\apps\rhythm\current\rhythm.exe`:        "scoop uninstall rhythm",
		`C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\RhythmShahriar.rhythm_Microsoft.Winget.Source_8wekyb3d8bbwe\rhythm.exe`: "winget uninstall --id RhythmShahriar.rhythm",
	} {
		m := managedBy(path, filepath.Base(strings.ReplaceAll(path, `\`, "/")))
		if m == nil || m.remove != want {
			t.Errorf("managedBy(%s) = %+v, want remove %q", path, m, want)
		}
	}
}
