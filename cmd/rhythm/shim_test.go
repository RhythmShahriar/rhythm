// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerOf(t *testing.T) {
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("ASDF_DATA_DIR", "")
	t.Setenv("SCOOP", "")
	for _, c := range []struct {
		path, real string
		checkName  bool
		want       string // manager name and upgrade command, or "" for rhythm's own
	}{
		// mise links every shim to the mise binary.
		{"/home/me/.local/share/mise/shims/rhythm-tuner", "/home/me/.local/bin/mise", true, "mise: mise upgrade"},
		// asdf writes a shell script into its shims folder.
		{"/home/me/.asdf/shims/rhythm-tuner", "/home/me/.asdf/shims/rhythm-tuner", true, "asdf: asdf install <plugin> latest"},
		{"/data/asdf/shims/rhythm-tuner", "/data/asdf/shims/rhythm-tuner", true, "asdf: asdf install <plugin> latest"},
		{`C:\Users\me\scoop\shims\rhythm-tuner.exe`, `C:\Users\me\scoop\shims\rhythm-tuner.exe`, true, "Scoop: scoop update rhythm-tuner"},
		{"/opt/vm/shims/rhythm-tuner", "/opt/vm/shims/rhythm-tuner", true,
			"a version manager: the version manager that made /opt/vm/shims/rhythm-tuner"},
		// A shim is never rhythm's, a third-party program's included.
		{"/home/me/.local/share/mise/shims/rhythm-deploy", "/home/me/.local/bin/mise", false, "mise: mise upgrade"},
		// A link to a binary of another name: a multi-call binary.
		{"/home/me/bin/rhythm-tuner", "/usr/bin/busybox", true, "busybox: the program that installed /home/me/bin/rhythm-tuner"},
		{"/home/me/bin/rhythm-tuner", "/nix/store/abc-rhythm-tuner-0.3.0/bin/.rhythm-tuner-wrapped", true,
			"Nix: nix profile upgrade rhythm-tuner"},
		// rhythm's own: a plain file, a link to a file of the same name,
		// Windows' .exe and a third-party program linked to another name.
		{"/home/me/.local/bin/rhythm-tuner", "/home/me/.local/bin/rhythm-tuner", true, ""},
		{"/home/me/bin/rhythm-tuner", "/home/me/opt/rhythm-tuner", true, ""},
		{`C:\Users\me\bin\rhythm-tuner.exe`, `C:\Users\me\bin\rhythm-tuner.EXE`, true, ""},
		{"/home/me/bin/rhythm-deploy", "/home/me/opt/deploy", false, ""},
		{"/home/me/shimsfoo/rhythm-tuner", "/home/me/shimsfoo/rhythm-tuner", true, ""},
	} {
		got := ""
		if m := ownerOf(c.path, c.real, filepath.Base(strings.TrimSuffix(strings.ReplaceAll(c.path, `\`, "/"), ".exe")), c.checkName); m != nil {
			got = m.name + ": " + m.upgrade
		}
		if got != c.want {
			t.Errorf("ownerOf(%s, %s) = %q, want %q", c.path, c.real, got, c.want)
		}
	}
}

// miseShim lays out mise's way of installing rhythm-tuner: a shim in
// <dir>/.local/share/mise/shims linking to the mise binary in
// <dir>/.local/bin, with the shims first on PATH. It returns the shim and
// the mise binary.
func miseShim(t *testing.T, s *sandbox) (shim, mise string) {
	t.Helper()
	shims := mkdir(t, filepath.Join(s.dir, ".local", "share", "mise", "shims"))
	mise = filepath.Join(mkdir(t, filepath.Join(s.dir, ".local", "bin")), "mise")
	if err := os.WriteFile(mise, stubScript("rhythm-tuner", "0.1.0", s.log("mise"), ""), 0o755); err != nil {
		t.Fatal(err)
	}
	shim = filepath.Join(shims, "rhythm-tuner")
	if err := os.Symlink(mise, shim); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shims+string(os.PathListSeparator)+s.bin)
	return shim, mise
}

func TestUpdateLeavesMiseShim(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	_, mise := miseShim(t, s)
	before, _ := os.ReadFile(mise)

	code, out, errs := runCmd("update", "tuner", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner is managed by mise; update it with: mise upgrade\n") || strings.Contains(out, "✔") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if got, _ := os.ReadFile(mise); string(got) != string(before) {
		t.Error("the mise binary was overwritten")
	}
	for _, leftover := range []string{mise + ".prev", mise + ".prev.json"} {
		if _, err := os.Lstat(leftover); err == nil {
			t.Errorf("%s was written", leftover)
		}
	}
	if code, out, errs := runCmd("update", "--rollback", "tuner"); code == 0 || !strings.Contains(errs, "managed by mise") {
		t.Errorf("rollback: exit %d\n%s%s", code, out, errs)
	}
}

func TestUninstallLeavesShims(t *testing.T) {
	s := newSandbox(t, nil, nil)
	shim, mise := miseShim(t, s)
	code, out, errs := runCmd("uninstall", "tuner")
	want := "rhythm-tuner is managed by mise; remove it with: mise uninstall <tool>@<version> (mise ls shows them)\n"
	if code != 1 || out != "" || errs != want {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	for _, p := range []string{shim, mise} {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("%s was removed", p)
		}
	}

	// asdf's shims are scripts that run `asdf exec`.
	asdf := mkdir(t, filepath.Join(s.dir, ".asdf", "shims"))
	shim = script(t, asdf, "rhythm-echo", `exec asdf exec "rhythm-echo" "$@"`)
	t.Setenv("PATH", asdf+string(os.PathListSeparator)+s.bin)
	code, out, errs = runCmd("uninstall", "echo")
	want = "rhythm-echo is managed by asdf; remove it with: asdf uninstall <plugin> <version> (asdf list shows them)\n"
	if code != 1 || out != "" || errs != want {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Lstat(shim); err != nil {
		t.Error("the asdf shim was removed")
	}
}

func TestInstallLeavesALinkToAnotherProgram(t *testing.T) {
	_, latest := allTools()
	s := newSandbox(t, nil, latest)
	busybox := filepath.Join(mkdir(t, filepath.Join(s.dir, "opt")), "busybox")
	if err := os.WriteFile(busybox, []byte("busybox"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.bin, "rhythm-tuner")
	if err := os.Symlink(busybox, link); err != nil {
		t.Fatal(err)
	}
	code, _, errs := runCmd("install", "tuner", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "rhythm-tuner is managed by busybox; update it with: the program that installed "+link) {
		t.Errorf("exit %d: %s", code, errs)
	}
	if got, _ := os.ReadFile(busybox); string(got) != "busybox" {
		t.Error("busybox was overwritten")
	}
}
