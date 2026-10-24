// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVersionFromText(t *testing.T) {
	for in, want := range map[string]string{
		"rhythm-tuner 0.3.0 (commit abc1234)\n": "0.3.0",
		"rhythm-tuner v1.2.3-rc.1":              "1.2.3-rc.1",
		"rhythm 1.0.0+build.5":                  "1.0.0",
		"rhythm-echo (0.4.2)":                   "0.4.2",
		"rhythm-tuner dev":                      "dev",
		"rhythm-tuner dev (commit 0.1.0abc)":    "dev",
		"garbage\x00 output":                    "",
		"":                                      "",
		"rhythm-tuner 1.2":                      "",
	} {
		got, ok := versionFromText([]byte(in))
		if got != want || ok != (want != "") {
			t.Errorf("versionFromText(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestVersionFromJSON(t *testing.T) {
	for in, want := range map[string]string{
		`{"schema":"rhythm-version/v1","name":"rhythm-tuner","version":"0.3.2","commit":"abc1234"}`: "0.3.2",
		`{"schema":"rhythm-version/v1","version":"v1.0.0"}`:                                         "1.0.0",
		`{"schema":"rhythm-version/v1","version":"dev"}`:                                            "dev",
		`{"schema":"rhythm-version/v2","version":"0.3.2"}`:                                          "",
		`{"schema":"rhythm-version/v1"}`:                                                            "",
		`rhythm-tuner 0.3.2`:                                                                        "",
	} {
		got, ok := versionFromJSON([]byte(in))
		if got != want || ok != (want != "") {
			t.Errorf("versionFromJSON(%s) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

// script writes an executable shell script into dir.
func script(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbeVersion(t *testing.T) {
	newSandbox(t, nil, nil) // skips on Windows
	dir := t.TempDir()
	for name, c := range map[string]struct{ body, want string }{
		"commit": {`echo 'rhythm-tuner 0.3.0 (commit abc1234)'`, "0.3.0"},
		"json": {`if [ "$1 $2" = "version --json" ]; then echo '{"schema":"rhythm-version/v1","name":"rhythm-tuner","version":"0.3.2"}'; exit 0; fi
echo 'rhythm-tuner 9.9.9'`, "0.3.2"},
		// A tool without the JSON answer that treats "version" as a
		// usage error still answers --version.
		"legacy":  {`[ "$1" = version ] && exit 2; echo 'rhythm-tuner 0.2.0'`, "0.2.0"},
		"garbage": {`echo 'Segmentation fault'`, unknownVersion},
		"fails":   {`exit 1`, unknownVersion},
	} {
		v, reason, hung := probeVersion(t.Context(), script(t, dir, name, c.body))
		if v != c.want || hung || (v == unknownVersion) != (reason != "") {
			t.Errorf("%s: got %q (%q, hung %v), want %q", name, v, reason, hung, c.want)
		}
	}
}

func TestUpdateReadsVersionWithCommit(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	// The old parser took the last word, "abc1234)", for a dev build and
	// never updated this tool again.
	script(t, s.bin, "rhythm-tuner", `echo "$*" >> '`+s.log("rhythm-tuner")+`'; echo 'rhythm-tuner 0.1.0 (commit abc1234)'`)
	code, out, errs := runCmd("update", "tuner", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner       0.1.0 → 0.2.0 ✔") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateSkipsUnknownVersion(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	script(t, s.bin, "rhythm-echo", `echo "$*" >> '`+s.log("rhythm-echo")+`'; echo 'no version here'`)
	code, out, errs := runCmd("update", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-echo        version unknown (rhythm-echo --version printed no version number), not updated") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	for _, p := range s.api.requests() {
		if strings.Contains(p, "/echo") {
			t.Errorf("rhythm-echo was looked up: %s", p)
		}
	}
	if !strings.Contains(s.content("rhythm-tuner"), "0.2.0") {
		t.Error("the other tools weren't updated")
	}
}

func TestUpdateHungToolTimesOut(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary")
	}
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	// No exec: sleep is the shell's child and holds its stdout open,
	// which must not keep rhythm waiting. The timeout kills sleep too, as
	// it would the real tool behind an asdf or mise shim.
	pids := filepath.Join(s.dir, "sleep.pids")
	script(t, s.bin, "rhythm-soundcheck", `echo "$*" >> '`+s.log("rhythm-soundcheck")+`'; `+sleep+` 1000 & echo $! >> '`+pids+`'; wait`)
	alive := func() (left []int) {
		data, _ := os.ReadFile(pids)
		for _, f := range strings.Fields(string(data)) {
			pid, _ := strconv.Atoi(f)
			if p, err := os.FindProcess(pid); pid > 0 && err == nil && p.Signal(syscall.Signal(0)) == nil {
				left = append(left, pid)
			}
		}
		return left
	}
	t.Cleanup(func() {
		for _, pid := range alive() {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	})

	start := time.Now()
	code, out, errs := runCmd("update")
	if took := time.Since(start); took > 7*time.Second {
		t.Errorf("took %s", took)
	}
	if code != 0 || !strings.Contains(out, "rhythm-soundcheck  version unknown (no answer within 5s), not updated") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if s.ranUpdate("rhythm-soundcheck") {
		t.Error("the hung tool's data refresh ran")
	}
	// The kill is asynchronous; give it a moment to land.
	for deadline := time.Now().Add(2 * time.Second); len(alive()) > 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	if left := alive(); len(left) > 0 {
		t.Errorf("the hung tool's child still runs: pids %v", left)
	}
	if !strings.Contains(s.content("rhythm-tuner"), "0.2.0") || !s.ranUpdate("rhythm-tuner") {
		t.Error("the other tools weren't updated")
	}
}

func TestManagedBy(t *testing.T) {
	t.Setenv("MISE_DATA_DIR", "/data/mise")
	t.Setenv("ASDF_DATA_DIR", "")
	t.Setenv("SCOOP", `D:\tools\scoop-root`)
	for path, want := range map[string]string{
		"/opt/homebrew/Cellar/rhythm/0.3.0/bin/rhythm":                 "Homebrew: brew upgrade rhythm",
		"/usr/local/Cellar/rhythm-tuner/0.3.0/bin/rhythm-tuner":        "Homebrew: brew upgrade rhythm-tuner",
		"/home/linuxbrew/.linuxbrew/Cellar/rhythm/0.3.0/bin/rhythm":    "Homebrew: brew upgrade rhythm",
		"/opt/homebrew/Caskroom/rhythm/0.3.0/rhythm":                   "Homebrew: brew upgrade --cask rhythm",
		"/opt/homebrew/bin/rhythm":                                     "Homebrew: brew upgrade rhythm",
		"/nix/store/abc123-rhythm-0.3.0/bin/rhythm":                    "Nix: nix profile upgrade rhythm",
		"/home/me/.local/share/mise/installs/rhythm/0.3.0/rhythm":      "mise: mise upgrade rhythm",
		"/data/mise/installs/ubi-rhythmshahriar-rhythm/0.3.0/rhythm":   "mise: mise upgrade ubi-rhythmshahriar-rhythm",
		"/home/me/.asdf/installs/rhythm/0.3.0/bin/rhythm":              "asdf: asdf install rhythm latest",
		`C:\Users\me\scoop\apps\rhythm\current\rhythm.exe`:             "Scoop: scoop update rhythm",
		`D:\tools\scoop-root\apps\rhythm-tuner\0.3.0\rhythm-tuner.exe`: "Scoop: scoop update rhythm-tuner",
		`C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\RhythmShahriar.rhythm_Microsoft.Winget.Source_8wekyb3d8bbwe\rhythm.exe`: "WinGet: winget upgrade --id RhythmShahriar.rhythm",
		"/usr/local/bin/rhythm":                "",
		"/home/me/.local/bin/rhythm":           "",
		"/home/me/opt/rhythm-tuner":            "",
		"/home/me/src/homebrew-notes/rhythm":   "",
		`C:\Users\me\AppData\Local\rhythm\bin`: "",
	} {
		got := ""
		if m := managedBy(path, filepath.Base(strings.ReplaceAll(path, `\`, "/"))); m != nil {
			got = m.name + ": " + m.upgrade
		}
		if got != want {
			t.Errorf("managedBy(%s) = %q, want %q", path, got, want)
		}
	}
}
