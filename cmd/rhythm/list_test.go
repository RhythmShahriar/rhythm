// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runManage runs one of this file set's commands the way `rhythm <name>`
// will once they are wired into main.go's command table.
func runManage(args ...string) (code int, stdout, stderr string) {
	c, ok := routerCommands[args[0]]
	if !ok {
		return runCmd(args...)
	}
	var out, errb bytes.Buffer
	code = c.run(args[1:], &out, &errb)
	return code, out.String(), errb.String()
}

// listSandbox has rhythm 0.1.0, tuner 0.1.0 (0.2.0 released), a
// third-party rhythm-foo 1.2.0 and things that must not be listed.
func listSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.1.0", "rhythm-foo": "1.2.0"},
		map[string]string{tools["tuner"]: "0.2.0", selfRepo: "0.1.0"})
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")
	// Not programs: a plain file and a folder.
	if err := os.WriteFile(filepath.Join(s.bin, "rhythm-notes"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.bin, "rhythm-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A second rhythm-foo further down PATH is hidden by the first.
	later := t.TempDir()
	script(t, later, "rhythm-foo", "echo rhythm-foo 9.9.9")
	t.Setenv("PATH", s.bin+string(os.PathListSeparator)+later)
	return s
}

func TestList(t *testing.T) {
	s := listSandbox(t)
	code, out, errs := runManage("list")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	want := "TOOL       VERSION  LATEST  SIGNED  PATH\n" +
		"rhythm     0.1.0    0.1.0   -       " + filepath.Join(s.bin, "rhythm") + "\n" +
		"tuner      0.1.0    0.2.0   -       " + filepath.Join(s.bin, "rhythm-tuner") + "\n" +
		"foo (3rd)  1.2.0    -       -       " + filepath.Join(s.bin, "rhythm-foo") + "\n"
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	for _, p := range s.api.requests() {
		if strings.Contains(p, "/dl/") {
			t.Errorf("list downloaded something: %s", p)
		}
	}
}

// TestListSkipsLeftovers lists after an update kept rhythm-tuner.prev,
// with other leftovers and a rhythm-* program in a relative PATH entry.
// None of them is a third-party tool or is run.
func TestListSkipsLeftovers(t *testing.T) {
	s := twoVersions(t)
	ran := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"rhythm-tuner.prev-new", "rhythm-tuner.rollback", "rhythm-tuner.old"} {
		script(t, s.bin, name, "echo "+name+" >> "+ran)
	}
	script(t, ".", "rhythm-here", "echo rhythm-here >> "+ran)
	t.Setenv("PATH", s.bin+string(os.PathListSeparator)+".")

	code, out, errs := runManage("list", "--offline")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if strings.Contains(out, "(3rd)") || strings.Contains(out, ".prev") {
		t.Errorf("leftovers listed:\n%s", out)
	}
	if data, err := os.ReadFile(ran); err == nil {
		t.Errorf("ran: %s", data)
	}
	if got := pathCopies("rhythm-here"); len(got) != 0 {
		t.Errorf("pathCopies found %v in the current folder", got)
	}
}

func TestListOffline(t *testing.T) {
	s := listSandbox(t)
	for _, args := range [][]string{{"list", "--offline"}, {"list"}} {
		if args[len(args)-1] == "list" {
			t.Setenv("RHYTHM_OFFLINE", "1")
		}
		code, out, errs := runManage(args...)
		if code != 0 || errs != "" {
			t.Fatalf("exit %d\n%s%s", code, out, errs)
		}
		if !strings.HasPrefix(out, "TOOL       VERSION  SIGNED  PATH\n") || !strings.Contains(out, "tuner      0.1.0    -       ") {
			t.Errorf("%v:\n%s", args, out)
		}
	}
	if r := s.api.requests(); len(r) != 0 {
		t.Errorf("offline list asked the API: %v", r)
	}
}

func TestListLookupFailure(t *testing.T) {
	listSandbox(t)
	api := closedPort(t)
	t.Setenv("RHYTHM_GITHUB_API", api)
	code, out, errs := runManage("list")
	if code != 0 {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(out, "tuner      0.1.0    ?       -") {
		t.Errorf("stdout:\n%s", out)
	}
	want := "can't reach " + strings.TrimPrefix(api, "http://") + " for rhythm, rhythm-tuner; check network, proxy or RHYTHM_GITHUB_API\n"
	if errs != want {
		t.Errorf("stderr:\n%s\nwant\n%s", errs, want)
	}
}

func TestListManagedSignedAndUnknown(t *testing.T) {
	s := newSandbox(t, map[string]string{"rhythm-echo": "0.1.0"}, map[string]string{tools["tuner"]: "0.2.0", tools["echo"]: "0.1.0"})
	// tuner lives in a Homebrew Cellar and is linked onto PATH.
	cellar := filepath.Join(s.dir, "Cellar", "rhythm-tuner", "0.1.0", "bin")
	if err := os.MkdirAll(cellar, 0o755); err != nil {
		t.Fatal(err)
	}
	script(t, cellar, "rhythm-tuner", "echo rhythm-tuner 0.1.0")
	if err := os.Symlink(filepath.Join(cellar, "rhythm-tuner"), filepath.Join(s.bin, "rhythm-tuner")); err != nil {
		t.Fatal(err)
	}
	s.stub(t, "rhythm-soundcheck", "", "")
	script(t, s.bin, "rhythm-soundcheck", "echo no version here")
	old := signedStatus
	signedStatus = func(path string) string {
		if strings.HasSuffix(path, "rhythm-echo") {
			return "yes"
		}
		return "no"
	}
	t.Cleanup(func() { signedStatus = old })

	code, out, errs := runManage("list")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"echo        0.1.0    0.1.0   yes     " + filepath.Join(s.bin, "rhythm-echo") + "\n",
		"tuner       0.1.0    0.2.0   no      " + filepath.Join(s.bin, "rhythm-tuner") + " (Homebrew)\n",
		"soundcheck  unknown  ?       no      ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if !strings.Contains(errs, "rhythm-soundcheck  version unknown (rhythm-soundcheck --version printed no version number)") {
		t.Errorf("stderr:\n%s", errs)
	}
}

func TestListUsage(t *testing.T) {
	if code, out, _ := runManage("list", "--help"); code != 0 || !strings.Contains(out, "Usage: rhythm list [flags]") || !strings.Contains(out, "--offline") {
		t.Errorf("help: exit %d\n%s", code, out)
	}
	if code, _, errs := runManage("list", "tuner"); code != 2 || !strings.Contains(errs, "takes no names") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestRouterCommandsRegistered(t *testing.T) {
	for _, name := range []string{"list", "uninstall", "doctor"} {
		c, ok := routerCommands[name]
		if !ok || c.run == nil || c.summary == "" {
			t.Errorf("%s isn't registered: %+v", name, c)
		}
	}
}
