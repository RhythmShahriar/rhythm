// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubAs sets the -ldflags variables for one test.
func stubAs(t *testing.T, v, ts, feats, block string) {
	t.Helper()
	old := []string{version, tools, features, toolsBlock}
	version, tools, features, toolsBlock = v, ts, feats, block
	t.Cleanup(func() { version, tools, features, toolsBlock = old[0], old[1], old[2], old[3] })
}

func call(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionAndHelp(t *testing.T) {
	stubAs(t, "0.4.0", "tuner:t,soundcheck:sc", "", "yes")
	if code, out, _ := call("--version"); code != 0 || out != "rhythm 0.4.0\n" {
		t.Fatalf("--version = %d %q", code, out)
	}
	_, out, _ := call("help")
	if !strings.Contains(out, "Tools:\n  tuner (t)  a tool\n  soundcheck (sc)  a tool\n\n") {
		t.Fatalf("help has no Tools block:\n%s", out)
	}

	stubAs(t, "0.3.5", "tuner:t", "", "no")
	if _, out, _ := call(); strings.Contains(out, "Tools:") {
		t.Fatalf("help has a Tools block:\n%s", out)
	}
}

func TestDryRun(t *testing.T) {
	stubAs(t, "0.6.0", "tuner:t,echo:e", "dry-run", "yes")
	if code, out, _ := call("install", "--dry-run", "tuner", "e"); code != 0 || out != "tuner\necho\n" {
		t.Fatalf("dry run = %d %q", code, out)
	}
	// Every unknown name is listed, with a suggestion when one is close.
	want := "rhythm: unknown tool \"tunr\" (did you mean tuner?)\nrhythm: unknown tool \"bogus\"\n  known: echo, tuner\n"
	if code, _, errOut := call("install", "--dry-run", "tunr", "echo", "bogus"); code != 2 || errOut != want {
		t.Fatalf("unknown = %d %q", code, errOut)
	}
	if code, _, errOut := call("install", "--defaults"); code != 2 || !strings.Contains(errOut, "flag provided but not defined: -defaults") {
		t.Fatalf("unknown flag = %d %q", code, errOut)
	}

	stubAs(t, "0.5.0", "tuner:t", "", "yes")
	if code, _, errOut := call("install", "--dry-run", "tuner"); code != 2 || !strings.Contains(errOut, "flag provided but not defined: -dry-run") {
		t.Fatalf("old router = %d %q", code, errOut)
	}
}

func TestVerify(t *testing.T) {
	stubAs(t, "0.7.0", "tuner:t", "dry-run,verify", "yes")
	if _, out, _ := call("help"); !strings.Contains(out, "\n  rhythm verify ") {
		t.Fatalf("help doesn't list verify:\n%s", out)
	}
	if code, out, _ := call("verify", "rhythm"); code != 0 || out != "Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (release-go.yml)\n" {
		t.Fatalf("verify = %d %q", code, out)
	}
	t.Setenv("RHYTHM_TEST_VERIFY", "fail")
	if code, _, errOut := call("verify", "rhythm"); code != 1 || !strings.Contains(errOut, "signature not valid") {
		t.Fatalf("failing verify = %d %q", code, errOut)
	}

	stubAs(t, "0.6.0", "tuner:t", "dry-run", "yes")
	if _, out, _ := call("help"); strings.Contains(out, "verify") {
		t.Fatalf("help lists verify:\n%s", out)
	}
	if code, _, _ := call("verify", "rhythm"); code != 1 {
		t.Fatalf("verify without the feature = %d", code)
	}
}

func TestDefaultsPinsAndLock(t *testing.T) {
	stubAs(t, "0.7.0", "tuner:t,soundcheck:sc,echo:e", "dry-run,defaults,pins,locked", "yes")
	if code, out, _ := call("install", "--defaults", "--dry-run"); code != 0 || out != "echo\nsoundcheck\ntuner\n" {
		t.Fatalf("--defaults = %d %q", code, out)
	}
	t.Setenv("RHYTHM_TEST_DEFAULTS", "sc")
	if code, out, _ := call("install", "t@0.3.1", "--defaults", "--dry-run"); code != 0 || out != "tuner@0.3.1\nsoundcheck\n" {
		t.Fatalf("pin and --defaults = %d %q", code, out)
	}
	if _, out, _ := call("install", "--help"); !strings.Contains(out, "--defaults") || !strings.Contains(out, "--locked") {
		t.Fatalf("install --help:\n%s", out)
	}

	dir := t.TempDir()
	t.Chdir(dir)
	if code, _, errOut := call("install", "--locked"); code != 1 || errOut != "rhythm: no rhythm.lock found\n" {
		t.Fatalf("no lock = %d %q", code, errOut)
	}
	lock := `{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","version":"0.3.1"},{"name":"echo","version":"0.1.0"}]}`
	if err := os.WriteFile(filepath.Join(dir, "rhythm.lock"), []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := call("install", "--locked", "--dir", dir, "--quiet")
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.3.1 ") || !strings.Contains(out, "Installed rhythm-echo 0.1.0 ") {
		t.Fatalf("--locked = %d %q %q", code, out, errOut)
	}

	stubAs(t, "0.6.0", "tuner:t", "dry-run", "yes")
	if code, _, errOut := call("install", "--defaults", "--dry-run"); code != 2 || !strings.Contains(errOut, "flag provided but not defined: -defaults") {
		t.Fatalf("old --defaults = %d %q", code, errOut)
	}
	if code, _, errOut := call("install", "--dry-run", "tuner@0.3.1"); code != 2 || !strings.Contains(errOut, `unknown tool "tuner@0.3.1"`) {
		t.Fatalf("old pin = %d %q", code, errOut)
	}
}

func TestQuietAndPathNote(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	stubAs(t, "0.6.0", "tuner:t,echo:e", "dry-run", "yes")
	if code, out, _ := call("install", "--help"); code != 0 || !strings.Contains(out, "--quiet") {
		t.Fatalf("install --help = %d %q", code, out)
	}
	note := "Note: " + dir + " isn't on your PATH.\n"
	if _, out, _ := call("install", "tuner", "echo", "--dir", dir); strings.Count(out, note) != 1 {
		t.Fatalf("want one PATH note:\n%s", out)
	}
	if _, out, _ := call("install", "tuner", "--dir", dir, "--quiet"); strings.Contains(out, "Note:") {
		t.Fatalf("--quiet printed a note:\n%s", out)
	}
	t.Setenv("PATH", dir)
	if _, out, _ := call("install", "tuner", "--dir", dir); strings.Contains(out, "Note:") {
		t.Fatalf("a note for a folder on PATH:\n%s", out)
	}

	t.Setenv("PATH", "")
	stubAs(t, "0.5.0", "tuner:t,echo:e", "", "yes")
	if code, out, _ := call("install", "--help"); code != 2 || strings.Contains(out, "--quiet") {
		t.Fatalf("old install --help = %d %q", code, out)
	}
	if code, _, _ := call("install", "tuner", "--quiet"); code != 2 {
		t.Fatalf("old install --quiet = %d", code)
	}
	if _, out, _ := call("install", "tuner", "echo", "--dir", dir); strings.Count(out, note) != 2 {
		t.Fatalf("want a PATH note for each tool:\n%s", out)
	}
}

func TestInstallFetchesTheArchive(t *testing.T) {
	stubAs(t, "0.6.0", "tuner:t,echo:e", "dry-run", "yes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dl/echo/") {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)
	t.Setenv("PATH", "")
	dir := t.TempDir()
	code, out, errOut := call("install", "echo", "tuner", "--dir", dir, "--quiet")
	if code != 1 || !strings.Contains(out, "Installed rhythm-tuner ") || strings.Contains(out, "rhythm-echo ") {
		t.Fatalf("install = %d %q %q", code, out, errOut)
	}
	if !strings.Contains(errOut, "rhythm-echo        downloading echo_") || !strings.HasSuffix(errOut, ": HTTP 404\nInstalled 1, failed 1: rhythm-echo\n") {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestInstallCopiesItselfAndLogs(t *testing.T) {
	stubAs(t, "0.6.0", "tuner:t", "dry-run", "yes")
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
	// An alias installs the tool under its name.
	if code, out, errOut := call("install", "t", "--dir", dir); code != 0 || !strings.Contains(out, "Installed rhythm-tuner ") {
		t.Fatalf("install t = %d %q %q", code, out, errOut)
	}
	if code, _, _ := call("bogus"); code != 1 {
		t.Fatalf("unknown command = %d", code)
	}
}

func TestAlreadyInstalled(t *testing.T) {
	stubAs(t, "0.7.0", "tuner:t,echo:e", "dry-run,pins", "yes")
	dir := t.TempDir()
	t.Setenv("RHYTHM_TEST_ALREADY", "tuner")
	t.Setenv("PATH", dir)
	code, out, errOut := call("install", "t@0.3.1", "echo", "--dir", dir)
	name := "rhythm-tuner"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	want := "rhythm-tuner 0.3.1 is already installed at " + filepath.Join(dir, name) + " (use --force to reinstall)\n"
	if code != 0 || !strings.HasPrefix(out, want) || !strings.Contains(out, "Installed rhythm-echo ") {
		t.Fatalf("install = %d %q %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
		t.Fatalf("an already installed tool was copied: %v", err)
	}
}

// TestMain clears the variables users are told to set, so a mirror, a
// token or offline mode in the environment doesn't change the results.
func TestMain(m *testing.M) {
	for _, v := range []string{"RHYTHM_OFFLINE", "RHYTHM_GITHUB_API", "RHYTHM_GITHUB_TOKEN", "GITHUB_TOKEN",
		"RHYTHM_MIN_RELEASE_AGE", "RHYTHM_DOWNLOAD_CACHE", "RHYTHM_TEST_LOG", "RHYTHM_TEST_VERIFY", "RHYTHM_TEST_ALREADY"} {
		_ = os.Unsetenv(v)
	}
	os.Exit(m.Run())
}
