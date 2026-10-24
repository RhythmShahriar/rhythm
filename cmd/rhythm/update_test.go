// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain points the config and cache folders at a temp folder, so no
// test reads or writes the real ~/.config/rhythm (first-run markers,
// update-check state) or ~/.cache/rhythm. It also clears the settings a
// developer may have exported, so the tests see rhythm's defaults; tests
// that need one set it themselves.
// startEnv is the environment the tests started with, before TestMain
// moved HOME and the config folders.
var startEnv = os.Environ()

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "rhythm-test-home-*")
	if err != nil {
		panic(err)
	}
	for _, v := range []string{"RHYTHM_MIN_RELEASE_AGE", "RHYTHM_DOWNLOAD_CACHE", "RHYTHM_OFFLINE", "RHYTHM_GITHUB_API",
		"GITHUB_TOKEN", "RHYTHM_GITHUB_TOKEN", "CI", "RHYTHM_NO_UPDATE_CHECK"} {
		_ = os.Unsetenv(v)
	}
	for _, v := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA", "RHYTHM_CONFIG_DIR", "RHYTHM_CACHE_DIR"} {
		_ = os.Setenv(v, filepath.Join(home, strings.ToLower(v)))
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	if builtBin != "" {
		_ = os.RemoveAll(filepath.Dir(builtBin))
	}
	os.Exit(code)
}

// freshConfig gives the test a config folder of its own.
func freshConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("APPDATA", filepath.Join(dir, "config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "cache"))
	// kit/config prefers these over the OS folders above.
	t.Setenv("RHYTHM_CONFIG_DIR", filepath.Join(dir, "config", "rhythm"))
	t.Setenv("RHYTHM_CACHE_DIR", filepath.Join(dir, "cache", "rhythm"))
	return dir
}

// fakeAPI serves GitHub-style latest releases for several repositories
// and records every request path.
type fakeAPI struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
	noSig atomic.Bool // leave checksums.txt.sigstore.json out of the releases
}

// sigFor is the stand-in Sigstore bundle the fake API serves for repo.
func sigFor(repo string) string { return "bundle for " + repo }

// requests returns the paths requested so far.
func (f *fakeAPI) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

// binaryFor returns the binary a repository releases: rhythm for the
// router, rhythm-<name> for a tool.
func binaryFor(repo string) string {
	if repo == selfRepo {
		return selfName
	}
	return "rhythm-" + repo[strings.LastIndex(repo, "/")+1:]
}

// newFakeAPI offers latest[repo] for each repository. Each archive holds
// body(binary, version), built when the server starts.
func newFakeAPI(t *testing.T, latest map[string]string, body func(binary, version string) []byte) *fakeAPI {
	t.Helper()
	type asset struct{ archive, sums []byte }
	assets := map[string]asset{}
	for repo, v := range latest {
		bin := binaryFor(repo)
		archive := tarGz(t, bin, body(bin, v))
		sum := sha256.Sum256(archive)
		name := fmt.Sprintf("%s_%s_%s.tar.gz", bin, runtime.GOOS, runtime.GOARCH)
		assets[repo] = asset{archive, fmt.Appendf(nil, "%s  %s\n", hex.EncodeToString(sum[:]), name)}
	}
	f := &fakeAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/repos/") && strings.HasSuffix(p, "/releases/latest"):
			repo := strings.TrimSuffix(strings.TrimPrefix(p, "/repos/"), "/releases/latest")
			v, ok := latest[repo]
			if !ok {
				http.NotFound(w, r)
				return
			}
			bin := binaryFor(repo)
			name := fmt.Sprintf("%s_%s_%s.tar.gz", bin, runtime.GOOS, runtime.GOARCH)
			sig := ""
			if !f.noSig.Load() {
				sig = fmt.Sprintf(`,{"name":"checksums.txt.sigstore.json","browser_download_url":"%s/sig/%s"}`, f.URL, repo)
			}
			fmt.Fprintf(w, `{"tag_name":"v%s","assets":[{"name":%q,"browser_download_url":"%s/dl/%s"},{"name":"checksums.txt","browser_download_url":"%s/sums/%s"}%s]}`,
				v, name, f.URL, repo, f.URL, repo, sig)
		case strings.HasPrefix(p, "/dl/"):
			w.Write(assets[strings.TrimPrefix(p, "/dl/")].archive)
		case strings.HasPrefix(p, "/sums/"):
			w.Write(assets[strings.TrimPrefix(p, "/sums/")].sums)
		case strings.HasPrefix(p, "/sig/"):
			io.WriteString(w, sigFor(strings.TrimPrefix(p, "/sig/")))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write(body)
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

// stubScript stands in for a tool binary. It appends its arguments to
// log, prints "<binary> <version>" when asked for its version and runs
// data (shell) for "update", its data refresh.
func stubScript(binary, version, log, data string) []byte {
	if data == "" {
		data = "exit 0"
	}
	return fmt.Appendf(nil, "#!/bin/sh\necho \"$*\" >> '%s'\ncase \"$1\" in\nupdate) %s ;;\n*) echo '%s %s' ;;\nesac\n",
		log, data, binary, version)
}

// sandbox is a temp PATH holding stub tools, plus a fake release API.
type sandbox struct {
	dir string // holds bin/ and one <binary>.log per stub
	bin string // the only directory on PATH
	api *fakeAPI
}

// newSandbox installs a stub for each installed[binary] = version and
// serves latest[repo] = version. rhythm itself stays the test binary, a
// dev build, unless the test points selfPath elsewhere.
func newSandbox(t *testing.T, installed, latest map[string]string) *sandbox {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-ins for the tools")
	}
	s := &sandbox{dir: t.TempDir()}
	s.bin = filepath.Join(s.dir, "bin")
	if err := os.Mkdir(s.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	s.api = newFakeAPI(t, latest, func(binary, v string) []byte { return stubScript(binary, v, s.log(binary), "") })
	for binary, v := range installed {
		s.stub(t, binary, v, "")
	}
	t.Setenv("PATH", s.bin)
	t.Setenv("RHYTHM_GITHUB_API", s.api.URL)
	t.Setenv("GITHUB_TOKEN", "")
	return s
}

// stub writes a stand-in for binary into bin/, dated an hour ago so a
// rewrite shows in its mtime.
func (s *sandbox) stub(t *testing.T, binary, version, data string) string {
	t.Helper()
	p := filepath.Join(s.bin, binary)
	if err := os.WriteFile(p, stubScript(binary, version, s.log(binary), data), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	return p
}

func (s *sandbox) log(binary string) string { return filepath.Join(s.dir, binary+".log") }

// calls returns the argument lines binary was run with.
func (s *sandbox) calls(binary string) []string {
	data, _ := os.ReadFile(s.log(binary))
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// ranUpdate reports whether binary's data refresh ran.
func (s *sandbox) ranUpdate(binary string) bool {
	for _, c := range s.calls(binary) {
		if c == "update" || strings.HasPrefix(c, "update ") {
			return true
		}
	}
	return false
}

func (s *sandbox) mtime(t *testing.T, binary string) time.Time {
	t.Helper()
	fi, err := os.Stat(filepath.Join(s.bin, binary))
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime()
}

func (s *sandbox) content(binary string) string {
	data, _ := os.ReadFile(filepath.Join(s.bin, binary))
	return string(data)
}

// allTools has every registry tool at 0.1.0, with 0.2.0 released.
func allTools() (installed, latest map[string]string) {
	installed, latest = map[string]string{}, map[string]string{}
	for _, tool := range toolNames() {
		installed["rhythm-"+tool] = "0.1.0"
		latest[repoOf(tool)] = "0.2.0"
	}
	return installed, latest
}

func runCmd(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestUpdateNamedToolOnly(t *testing.T) {
	for _, name := range []string{"tuner", "t", "rhythm-tuner", "TUNER"} {
		t.Run(name, func(t *testing.T) {
			installed, latest := allTools()
			s := newSandbox(t, installed, latest)
			before := map[string]time.Time{}
			for binary := range installed {
				before[binary] = s.mtime(t, binary)
			}

			code, out, errs := runCmd("update", name)
			if code != 0 {
				t.Fatalf("exit %d\n%s%s", code, out, errs)
			}
			if !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.2.0") {
				t.Errorf("tuner not updated:\n%s%s", out, errs)
			}
			if !s.ranUpdate("rhythm-tuner") {
				t.Error("tuner's data wasn't refreshed")
			}
			for binary := range installed {
				if binary == "rhythm-tuner" {
					continue
				}
				if !s.mtime(t, binary).Equal(before[binary]) {
					t.Errorf("%s was rewritten", binary)
				}
				if s.ranUpdate(binary) {
					t.Errorf("%s's data refresh ran", binary)
				}
			}
			for _, p := range s.api.requests() {
				if !strings.Contains(p, "/tuner/") && !strings.HasSuffix(p, "/tuner") {
					t.Errorf("request for another target: %s", p)
				}
			}
			if strings.Contains(out, "rhythm ") || strings.Contains(out, "development build") {
				t.Errorf("rhythm itself was considered:\n%s", out)
			}
		})
	}
}

func TestUpdateUnknownNameMakesNoRequest(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	code, _, errs := runCmd("update", "tuner", "nope")
	if code != 2 || !strings.Contains(errs, `unknown tool "nope"`) {
		t.Errorf("exit %d: %s", code, errs)
	}
	code, _, errs = runCmd("update", "tunr")
	if code != 2 || !strings.Contains(errs, "did you mean tuner?") {
		t.Errorf("exit %d: %s", code, errs)
	}
	if r := s.api.requests(); len(r) != 0 {
		t.Errorf("requests before the names were checked: %v", r)
	}
	if s.ranUpdate("rhythm-tuner") {
		t.Error("tuner ran despite the usage error")
	}
}

func TestUpdateCommaListAndFlagsAfterNames(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	code, out, errs := runCmd("update", "tuner,echo", "--no-data")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for binary, want := range map[string]string{"rhythm-tuner": "0.2.0", "rhythm-echo": "0.2.0", "rhythm-soundcheck": "0.1.0"} {
		if !strings.Contains(s.content(binary), want) {
			t.Errorf("%s is not %s", binary, want)
		}
		if s.ranUpdate(binary) {
			t.Errorf("--no-data: %s's data refresh ran", binary)
		}
	}
}

func TestUpdateNoData(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	code, out, errs := runCmd("update", "--no-data")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for binary := range installed {
		if !strings.Contains(s.content(binary), "0.2.0") {
			t.Errorf("%s not updated", binary)
		}
		if s.ranUpdate(binary) {
			t.Errorf("%s's data refresh ran", binary)
		}
	}
}

// fakeSelf makes rhythm a release build at path, so it can be updated.
func fakeSelf(t *testing.T, path, v string) {
	t.Helper()
	oldPath, oldVersion := selfPath, version
	selfPath = func() (string, error) { return path, nil }
	version = v
	t.Cleanup(func() { selfPath, version = oldPath, oldVersion })
}

func TestUpdateSelfOnly(t *testing.T) {
	installed, latest := allTools()
	latest[selfRepo] = "0.2.0"
	s := newSandbox(t, installed, latest)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")
	before := map[string]time.Time{}
	for binary := range installed {
		before[binary] = s.mtime(t, binary)
	}

	code, out, errs := runCmd("update", "--self-only")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm"), "rhythm 0.2.0") || !strings.Contains(out, "rhythm             0.1.0 → 0.2.0 ✔") {
		t.Errorf("rhythm not updated:\n%s%s", out, errs)
	}
	for binary := range installed {
		if !s.mtime(t, binary).Equal(before[binary]) || s.ranUpdate(binary) {
			t.Errorf("%s was touched", binary)
		}
	}

	// "rhythm" as a name means the same.
	s.stub(t, "rhythm", "0.1.0", "")
	if code, out, errs := runCmd("update", "rhythm"); code != 0 || !strings.Contains(out, "0.1.0 → 0.2.0 ✔") {
		t.Errorf("update rhythm: exit %d\n%s%s", code, out, errs)
	}

	if code, _, errs := runCmd("update", "--self-only", "tuner"); code != 2 || !strings.Contains(errs, "--self-only") {
		t.Errorf("--self-only with a name: exit %d: %s", code, errs)
	}
}

func TestUpdateNamedButNotInstalled(t *testing.T) {
	installed, latest := allTools()
	delete(installed, "rhythm-echo")
	newSandbox(t, installed, latest)
	code, _, errs := runCmd("update", "echo")
	if code != 1 || !strings.Contains(errs, "rhythm-echo        isn't installed; install it with: rhythm install echo") || !strings.Contains(errs, "failed 1: rhythm-echo") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestUpdateHelp(t *testing.T) {
	for _, h := range []string{"--help", "-h"} {
		code, out, errs := runCmd("update", h)
		if code != 0 || errs != "" {
			t.Errorf("%s: exit %d: %s", h, code, errs)
		}
		for _, want := range []string{"--check", "--self-only", "--no-data", "rhythm update [tool...]"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: help lacks %q:\n%s", h, want, out)
			}
		}
		if strings.Count(out, "Usage:") != 1 || strings.Contains(out, "  -check") {
			t.Errorf("%s: help:\n%s", h, out)
		}
	}
	if code, _, errs := runCmd("update", "--bogus"); code != 2 || !strings.Contains(errs, "--no-data") {
		t.Errorf("bad flag: exit %d: %s", code, errs)
	}
}

// preInterrupted makes the next command run as if Ctrl-C was already pressed.
func preInterrupted(t *testing.T) {
	t.Helper()
	old := signalContext
	signalContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}
	t.Cleanup(func() { signalContext = old })
}

func TestInterruptedExits130(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	preInterrupted(t)
	if code, out, errs := runCmd("update"); code != 130 || !strings.Contains(errs, "interrupted") {
		t.Errorf("update: exit %d\n%s%s", code, out, errs)
	}
	if code, out, errs := runCmd("install", "tuner", "--dir", t.TempDir()); code != 130 {
		t.Errorf("install: exit %d\n%s%s", code, out, errs)
	}
	for binary := range installed {
		if strings.Contains(s.content(binary), "0.2.0") {
			t.Errorf("%s changed after the interrupt", binary)
		}
	}
}

func TestUpdateReportsDataFailure(t *testing.T) {
	installed, latest := allTools()
	latest[repoOf("tuner")] = "0.1.0" // tuner is up to date; only its data fails
	s := newSandbox(t, installed, latest)
	s.stub(t, "rhythm-tuner", "0.1.0", "echo 'checks: fetch failed' >&2; exit 3")

	code, out, errs := runCmd("update")
	if code != 1 {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(errs, "rhythm-tuner  data refresh failed (exit 3)\n") {
		t.Errorf("no failure line:\n%s", errs)
	}
	if !strings.HasSuffix(errs, "Updated 2, up to date 1, failed 1: rhythm-tuner data\n") {
		t.Errorf("no closing summary:\n%s", errs)
	}
	if !strings.Contains(errs, "checks: fetch failed") || !strings.Contains(out, "rhythm-tuner       0.1.0 (up to date)") {
		t.Errorf("the tool's own output is lost:\n%s%s", out, errs)
	}
}

func TestUpdateSummaryNamesFailedBinaries(t *testing.T) {
	installed, latest := allTools()
	delete(latest, repoOf("echo")) // echo has no release: the binary update fails
	newSandbox(t, installed, latest)
	code, _, errs := runCmd("update", "--check")
	if code != 1 || !strings.HasSuffix(errs, "Available 2, up to date 0, failed 1: rhythm-echo\n") {
		t.Errorf("exit %d:\n%s", code, errs)
	}
	code, _, errs = runCmd("update", "echo", "soundcheck")
	if code != 1 || !strings.HasSuffix(errs, "Updated 1, up to date 0, failed 1: rhythm-echo\n") {
		t.Errorf("exit %d:\n%s", code, errs)
	}
}

func TestUpdateSuccessHasNoSummary(t *testing.T) {
	installed, latest := allTools()
	newSandbox(t, installed, latest)
	code, out, errs := runCmd("update")
	if code != 0 || strings.Contains(errs, "failed") || strings.Contains(out+errs, "Updated ") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateWritesThroughSymlinks(t *testing.T) {
	installed, latest := allTools()
	delete(installed, "rhythm-tuner")
	s := newSandbox(t, installed, latest)
	// bin/rhythm-tuner -> ../opt/rhythm-tuner, as stow or a hand-made link
	// would leave it.
	opt := filepath.Join(s.dir, "opt")
	if err := os.Mkdir(opt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opt, "rhythm-tuner"), stubScript("rhythm-tuner", "0.1.0", s.log("rhythm-tuner"), ""), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.bin, "rhythm-tuner")
	if err := os.Symlink("../opt/rhythm-tuner", link); err != nil {
		t.Fatal(err)
	}

	if code, out, errs := runCmd("update", "tuner"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced by a file: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(opt, "rhythm-tuner")); !strings.Contains(string(got), "rhythm-tuner 0.2.0") {
		t.Errorf("the link target wasn't updated:\n%s", got)
	}
}

func TestUpdateLeavesManagedInstalls(t *testing.T) {
	installed, latest := allTools()
	delete(installed, "rhythm-tuner")
	latest[selfRepo] = "0.2.0"
	s := newSandbox(t, installed, latest)
	cellar := func(name string) string {
		dir := filepath.Join(s.dir, "Cellar", name, "0.1.0", "bin")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, stubScript(name, "0.1.0", s.log(name), ""), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := os.Symlink(cellar("rhythm-tuner"), filepath.Join(s.bin, "rhythm-tuner")); err != nil {
		t.Fatal(err)
	}
	self := cellar("rhythm")
	fakeSelf(t, self, "0.1.0")

	code, out, errs := runCmd("update")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"rhythm-tuner is managed by Homebrew; update it with: brew upgrade rhythm-tuner\n",
		"rhythm is managed by Homebrew; update it with: brew upgrade rhythm\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	for _, p := range s.api.requests() {
		if strings.Contains(p, "/tuner") || strings.Contains(p, "/rhythm/") || strings.HasSuffix(p, "/rhythm") {
			t.Errorf("a managed install was looked up: %s", p)
		}
	}
	for _, p := range []string{self, filepath.Join(s.dir, "Cellar", "rhythm-tuner", "0.1.0", "bin", "rhythm-tuner")} {
		if got, _ := os.ReadFile(p); !strings.Contains(string(got), "0.1.0") {
			t.Errorf("%s was rewritten", p)
		}
	}
	// The data is the tool's own; Homebrew doesn't refresh it.
	if !s.ranUpdate("rhythm-tuner") {
		t.Error("the managed tool's data refresh didn't run")
	}
}

// closedPort returns a loopback URL nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return "http://" + addr
}

func TestUpdateUnreachableSaidOnce(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")
	api := closedPort(t)
	t.Setenv("RHYTHM_GITHUB_API", api)

	code, _, errs := runCmd("update", "--no-data")
	want := "can't reach " + strings.TrimPrefix(api, "http://") +
		" for rhythm, rhythm-echo, rhythm-soundcheck, rhythm-tuner; check network, proxy or RHYTHM_GITHUB_API\n" +
		"Updated 0, up to date 0, failed 4: rhythm, rhythm-echo, rhythm-soundcheck, rhythm-tuner\n"
	if code != 1 || errs != want {
		t.Errorf("exit %d\n%s\nwant\n%s", code, errs, want)
	}
}

func TestOfflineSaidOnce(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")
	t.Setenv("RHYTHM_OFFLINE", "1")

	code, _, errs := runCmd("update", "--no-data")
	want := "RHYTHM_OFFLINE is set, so nothing was downloaded for rhythm, rhythm-echo, rhythm-soundcheck, rhythm-tuner; unset it to reach the release API\n" +
		"Updated 0, up to date 0, failed 4: rhythm, rhythm-echo, rhythm-soundcheck, rhythm-tuner\n"
	if code != 1 || errs != want {
		t.Errorf("update: exit %d\n%s\nwant\n%s", code, errs, want)
	}
	code, _, errs = runCmd("install", "tuner", "echo", "--dir", t.TempDir())
	want = "RHYTHM_OFFLINE is set, so nothing was downloaded for rhythm-tuner, rhythm-echo; unset it to reach the release API\n" +
		"Installed 0, failed 2: rhythm-tuner, rhythm-echo\n"
	if code != 1 || errs != want {
		t.Errorf("install: exit %d\n%s\nwant\n%s", code, errs, want)
	}
	if n := len(s.api.requests()); n != 0 {
		t.Errorf("%d requests reached the API", n)
	}
}

// rateLimited serves 403 with X-RateLimit-Remaining: 0 and counts requests.
func rateLimited(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func TestUpdateRateLimitSaidOnce(t *testing.T) {
	installed, latest := allTools()
	newSandbox(t, installed, latest)
	srv, n := rateLimited(t)
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)

	code, _, errs := runCmd("update", "--check")
	want := "rate limit reached at " + srv.URL + " for rhythm-echo, rhythm-soundcheck, rhythm-tuner; set RHYTHM_GITHUB_TOKEN\n"
	if code != 1 || !strings.HasPrefix(errs, want) || strings.Count(errs, "\n") != 2 {
		t.Errorf("exit %d\n%s", code, errs)
	}
	// Once limited, rhythm stops asking.
	if got := n.Load(); got != 1 {
		t.Errorf("%d requests after the limit", got)
	}
}

func TestRateLimitedWording(t *testing.T) {
	soon, later := time.Now().Add(time.Hour), time.Now().Add(72*time.Hour)
	for _, tc := range []struct {
		e    rateLimitedError
		want string
	}{
		{rateLimitedError{api: defaultAPI}, "GitHub rate limit reached for rhythm, rhythm-tuner; set GITHUB_TOKEN"},
		{rateLimitedError{api: defaultAPI, tokenSent: true}, "GitHub rate limit reached for rhythm, rhythm-tuner"},
		{rateLimitedError{api: "https://mirror.example"}, "rate limit reached at https://mirror.example for rhythm, rhythm-tuner; set RHYTHM_GITHUB_TOKEN"},
		{rateLimitedError{api: defaultAPI, tokenSent: true, reset: soon}, "GitHub rate limit reached for rhythm, rhythm-tuner (resets " + soon.Format("15:04") + ")"},
		{rateLimitedError{api: defaultAPI, tokenSent: true, reset: later}, "GitHub rate limit reached for rhythm, rhythm-tuner (resets " + later.Format("Jan 2 15:04") + ")"},
	} {
		if got := tc.e.explain("rhythm, rhythm-tuner"); got != tc.want {
			t.Errorf("got  %q\nwant %q", got, tc.want)
		}
	}
}

func TestRateLimitNeedsTheHeader(t *testing.T) {
	// A plain 403 (a private repository, a blocked token) is not a rate
	// limit. It reads as a plain status.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)
	_, err := updater(repoOf("tuner"), "rhythm-tuner", "").Latest(context.Background())
	if _, _, _, ok := netProblem(err); ok || err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("err = %v", err)
	}
}

func TestMissingReleaseNamesTheMirror(t *testing.T) {
	_, latest := allTools()
	delete(latest, repoOf("echo"))
	s := newSandbox(t, nil, latest)
	code, _, errs := runCmd("install", "echo", "--dir", s.bin)
	want := "rhythm-echo        mirror " + s.api.URL + " has no release for RhythmShahriar/echo; check RHYTHM_GITHUB_API\n"
	if code != 1 || errs != want {
		t.Errorf("exit %d\n%q\nwant\n%q", code, errs, want)
	}
	if got := (&noReleaseError{api: defaultAPI, repo: "RhythmShahriar/echo"}).Error(); got != "RhythmShahriar/echo has no published releases yet" {
		t.Errorf("default API: %q", got)
	}
}

func TestInstallUnreachableSaidOnce(t *testing.T) {
	s := newSandbox(t, nil, nil)
	api := closedPort(t)
	t.Setenv("RHYTHM_GITHUB_API", api)
	code, out, errs := runCmd("install", "tuner", "echo", "--dir", s.bin)
	want := "can't reach " + strings.TrimPrefix(api, "http://") + " for rhythm-tuner, rhythm-echo; check network, proxy or RHYTHM_GITHUB_API\n" +
		"Installed 0, failed 2: rhythm-tuner, rhythm-echo\n"
	if code != 1 || out != "" || errs != want {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateProgressOnATerminal(t *testing.T) {
	installed, latest := allTools()
	newSandbox(t, installed, latest)
	if _, _, errs := runCmd("update", "tuner", "--no-data"); errs != "" {
		t.Errorf("progress without a terminal: %q", errs)
	}
	newSandbox(t, installed, latest)
	fakeTerminal(t)
	code, _, errs := runCmd("update", "tuner", "--no-data")
	if code != 0 || !strings.Contains(errs, "\rDownloading rhythm-tuner 0.2.0  ") || !strings.HasSuffix(errs, "\r\x1b[K") {
		t.Errorf("exit %d: %q", code, errs)
	}
}

func TestUpdateCheckExitStatus(t *testing.T) {
	t.Run("outdated", func(t *testing.T) {
		installed, latest := allTools()
		s := newSandbox(t, installed, latest)
		code, out, errs := runCmd("update", "--check")
		if code != exitOutdated || errs != "" {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
		if strings.Contains(s.content("rhythm-tuner"), "0.2.0") {
			t.Error("--check changed a binary")
		}
	})
	t.Run("one outdated", func(t *testing.T) {
		installed, latest := allTools()
		latest[repoOf("echo")], latest[repoOf("soundcheck")] = "0.1.0", "0.1.0"
		newSandbox(t, installed, latest)
		if code, out, errs := runCmd("update", "--check"); code != exitOutdated {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
	})
	t.Run("current", func(t *testing.T) {
		installed, latest := allTools()
		for repo := range latest {
			latest[repo] = "0.1.0"
		}
		newSandbox(t, installed, latest)
		if code, out, errs := runCmd("update", "--check"); code != 0 {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
	})
	t.Run("network error wins", func(t *testing.T) {
		installed, latest := allTools()
		s := newSandbox(t, installed, latest)
		// Echo's release lookup has its connection dropped; the others
		// are outdated.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/echo/") {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					conn.Close()
				}
				return
			}
			r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(s.api.URL, "http://")
			resp, err := http.Get(r.URL.String())
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
		}))
		t.Cleanup(srv.Close)
		t.Setenv("RHYTHM_GITHUB_API", srv.URL)
		code, out, errs := runCmd("update", "--check")
		if code != 1 || !strings.Contains(errs, "failed 1: rhythm-echo") {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
	})
	t.Run("plain update is 0", func(t *testing.T) {
		installed, latest := allTools()
		newSandbox(t, installed, latest)
		if code, out, errs := runCmd("update", "--no-data"); code != 0 {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
	})
	t.Run("help", func(t *testing.T) {
		_, out, _ := runCmd("update", "--help")
		if !strings.Contains(out, "10 when an update is available") {
			t.Errorf("help doesn't explain exit 10:\n%s", out)
		}
	})
}

func TestUpdateAnnouncesNewTools(t *testing.T) {
	freshConfig(t)
	installed, latest := allTools()
	delete(installed, "rhythm-echo")
	delete(installed, "rhythm-soundcheck")
	newSandbox(t, installed, latest)

	code, out, errs := runCmd("update", "--no-data")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	want := "New tool available: echo (rhythm install echo)\nNew tool available: soundcheck (rhythm install soundcheck)\n"
	if !strings.HasSuffix(out, want) || strings.Contains(out, "available: tuner") {
		t.Errorf("stdout:\n%s\nwant it to end with\n%s", out, want)
	}
	// Once per machine.
	if _, out, _ := runCmd("update", "--check"); strings.Contains(out, "New tool") {
		t.Errorf("announced twice:\n%s", out)
	}
	// tuner was installed, so it was recorded and isn't announced as new
	// after it is removed.
	t.Setenv("PATH", t.TempDir())
	if _, out, _ := runCmd("update", "--check"); strings.Contains(out, "New tool") {
		t.Errorf("an installed tool announced as new:\n%s", out)
	}
}

func TestUpdateAnnouncesAfterCheck(t *testing.T) {
	freshConfig(t)
	installed, latest := allTools()
	delete(installed, "rhythm-echo")
	newSandbox(t, installed, latest)
	code, out, _ := runCmd("update", "--check")
	if code != exitOutdated || !strings.Contains(out, "New tool available: echo (rhythm install echo)\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestUpdateQuiet(t *testing.T) {
	freshConfig(t)
	installed, latest := allTools()
	delete(installed, "rhythm-echo")
	latest[repoOf("tuner")] = "0.1.0"
	newSandbox(t, installed, latest)

	code, out, errs := runCmd("update", "--quiet", "--no-data")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if strings.Contains(out, "New tool") || strings.Contains(out, "up to date") {
		t.Errorf("--quiet printed:\n%s", out)
	}
	if !strings.Contains(out, "rhythm-soundcheck  0.1.0 → 0.2.0 ✔") {
		t.Errorf("--quiet hid the update:\n%s", out)
	}
	// A quiet run doesn't use up the announcement.
	if _, out, _ := runCmd("update", "--check"); !strings.Contains(out, "New tool available: echo") {
		t.Errorf("not announced after a quiet run:\n%s", out)
	}
}

func TestUpdateFailureHasNoAnnouncement(t *testing.T) {
	freshConfig(t)
	installed, latest := allTools()
	delete(installed, "rhythm-echo")
	delete(latest, repoOf("tuner"))
	newSandbox(t, installed, latest)
	if code, out, _ := runCmd("update", "--check"); code != 1 || strings.Contains(out, "New tool") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// cooldownSandbox serves Tuner 0.3.3 (a day old) and 0.3.2 (ten days)
// with installed tools at the given versions.
func cooldownSandbox(t *testing.T, installed map[string]string) *sandbox {
	t.Helper()
	s := newSandbox(t, installed, nil)
	newReleaseAPI(t, s, map[string][]fakeRel{
		repoOf("tuner"): {{version: "0.3.3", age: 24*time.Hour + time.Minute}, {version: "0.3.2", age: 10 * 24 * time.Hour}},
		selfRepo:        {{version: "0.8.0", age: 2 * 24 * time.Hour}, {version: "0.7.0", age: 30 * 24 * time.Hour}},
	})
	t.Setenv(minAgeEnv, "")
	return s
}

func TestUpdateMinAgeStays(t *testing.T) {
	s := cooldownSandbox(t, map[string]string{"rhythm-tuner": "0.3.2"})
	before := s.content("rhythm-tuner")
	code, out, errs := runCmd("update", "tuner", "--min-age", "3d", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner 0.3.3 is 1 day old; staying on 0.3.2 (min age 3d)\n") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if s.content("rhythm-tuner") != before {
		t.Error("the binary was replaced")
	}
}

func TestUpdateMinAgeTakesTheOlderRelease(t *testing.T) {
	s := cooldownSandbox(t, map[string]string{"rhythm-tuner": "0.3.1"})
	t.Setenv(minAgeEnv, "3d") // the default when --min-age isn't given
	code, out, errs := runCmd("update", "tuner", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner 0.3.3 is 1 day old; updating to 0.3.2 (min age 3d)\nrhythm-tuner       0.3.1 → 0.3.2 ✔") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.3.2") {
		t.Error("0.3.2 wasn't installed")
	}
	// The flag wins over the environment; 0 turns the cooldown off.
	code, out, errs = runCmd("update", "tuner", "--no-data", "--min-age", "0")
	if code != 0 || !strings.Contains(out, "0.3.2 → 0.3.3 ✔") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateMinAgeAppliesToRhythm(t *testing.T) {
	s := cooldownSandbox(t, nil)
	fakeSelf(t, s.stub(t, "rhythm", "0.7.0", ""), "0.7.0")
	code, out, errs := runCmd("update", "--self-only", "--min-age", "1w")
	if code != 0 || !strings.Contains(out, "rhythm 0.8.0 is 2 days old; staying on 0.7.0 (min age 1w)\n") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateMinAgeJSON(t *testing.T) {
	cooldownSandbox(t, map[string]string{"rhythm-tuner": "0.3.1"})
	code, out, errs := runCmd("update", "tuner", "--check", "--json", "--min-age", "3d")
	if code != exitOutdated {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	var rep updateReportJSON
	decodeStrict(t, out, &rep, "targets")
	got := rep.Targets[0]
	if got.Latest != "0.3.2" || got.SkippedNewer == nil || got.SkippedNewer.Version != "0.3.3" || got.SkippedNewer.Published == "" {
		t.Errorf("got %+v %+v", got, got.SkippedNewer)
	}
	if !strings.Contains(errs, "rhythm-tuner 0.3.3 is 1 day old; 0.3.2 is the newest old enough (min age 3d)") {
		t.Errorf("stderr:\n%s", errs)
	}
}

func TestInstallMinAge(t *testing.T) {
	s := cooldownSandbox(t, nil)
	code, out, errs := runCmd("install", "tuner", "--dir", s.bin, "--min-age", "3d")
	if code != 0 || !strings.Contains(out, "rhythm-tuner 0.3.3 is 1 day old; installing 0.3.2 (min age 3d)\nInstalled rhythm-tuner 0.3.2") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	// --quiet leaves only the Installed line.
	code, out, errs = runCmd("install", "tuner", "--dir", t.TempDir(), "--min-age", "3d", "--quiet")
	if code != 0 || !strings.HasPrefix(out, "Installed rhythm-tuner 0.3.2 to ") || strings.Count(out, "\n") != 1 || errs != "" {
		t.Errorf("--quiet: exit %d\n%s%s", code, out, errs)
	}
	// An exact version is the user's call; the cooldown doesn't apply.
	code, out, errs = runCmd("install", "tuner@0.3.3", "--dir", s.bin, "--min-age", "3d")
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.3.3") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	// Nothing old enough.
	code, _, errs = runCmd("install", "tuner", "--dir", t.TempDir(), "--min-age", "2w")
	if code != 1 || !strings.Contains(errs, "no release of RhythmShahriar/tuner is old enough: the newest, 0.3.3, is 1 day old (min age 2 weeks)") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestMinAgeInvalid(t *testing.T) {
	cooldownSandbox(t, nil)
	for _, args := range [][]string{{"update", "--min-age", "3"}, {"install", "tuner", "--min-age", "soon"}} {
		code, _, errs := runCmd(args...)
		if code != exitUsage || !strings.Contains(errs, "rhythm: --min-age: invalid age") || !strings.Contains(errs, "; use a duration such as 3d, 1w or 36h") {
			t.Errorf("%v: exit %d\n%s", args, code, errs)
		}
	}
	t.Setenv(minAgeEnv, "-1d")
	code, _, errs := runCmd("update")
	if code != exitUsage || !strings.Contains(errs, `rhythm: RHYTHM_MIN_RELEASE_AGE: invalid age "-1d"; use a duration such as 3d, 1w or 36h`) {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestParseAndFormatAge(t *testing.T) {
	const day = 24 * time.Hour
	for in, want := range map[string]time.Duration{
		"3d": 3 * day, "1w": 7 * day, "36h": 36 * time.Hour, "1w2d": 9 * day, "1.5d": 36 * time.Hour, "90m": 90 * time.Minute, "0": 0, "1h30m": 90 * time.Minute,
	} {
		if got, err := parseAge(in); err != nil || got != want {
			t.Errorf("parseAge(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "3", "-1d", "3y", "d", "1d-", "99999999999w"} {
		if _, err := parseAge(in); err == nil {
			t.Errorf("parseAge(%q) worked", in)
		}
	}
	for d, want := range map[time.Duration]string{
		time.Minute: "less than an hour", time.Hour: "1 hour", 5 * time.Hour: "5 hours", day + time.Minute: "1 day",
		36 * time.Hour: "36 hours", 3 * day: "3 days", 7 * day: "1 week", 14 * day: "2 weeks", 10 * day: "10 days",
	} {
		if got := formatAge(d); got != want {
			t.Errorf("formatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

// twoVersions installs Tuner 0.3.0 and then updates it to 0.4.0 through
// the fake API, leaving 0.3.0 as rhythm-tuner.prev.
func twoVersions(t *testing.T) *sandbox {
	t.Helper()
	s, _ := pinSandbox(t)
	if code, out, errs := runCmd("install", "tuner@0.3.0", "--dir", s.bin); code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(s.bin, "rhythm-tuner.prev")); err == nil {
		t.Error("a fresh install left a .prev")
	}
	if code, out, errs := runCmd("update", "tuner", "--no-data"); code != 0 || !strings.Contains(out, "0.3.0 → 0.4.0 ✔") {
		t.Fatalf("update: exit %d\n%s%s", code, out, errs)
	}
	return s
}

func TestUpdateKeepsPrevious(t *testing.T) {
	s := twoVersions(t)
	dest := filepath.Join(s.bin, "rhythm-tuner")
	if !strings.Contains(s.content("rhythm-tuner.prev"), "rhythm-tuner 0.3.0") {
		t.Fatalf(".prev isn't 0.3.0: %q", s.content("rhythm-tuner.prev"))
	}
	var side struct {
		Version    string `json:"version"`
		SHA256     string `json:"sha256"`
		ReplacedAt string `json:"replaced_at"`
		ReplacedBy struct {
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"replaced_by"`
	}
	data, err := os.ReadFile(dest + ".prev.json")
	if err != nil || json.Unmarshal(data, &side) != nil {
		t.Fatalf("sidecar: %v %s", err, data)
	}
	prevSum, _ := fileSHA256(dest + ".prev")
	curSum, _ := fileSHA256(dest)
	if side.Version != "0.3.0" || side.SHA256 != prevSum || side.ReplacedBy.Version != "0.4.0" || side.ReplacedBy.SHA256 != curSum || side.ReplacedAt == "" {
		t.Errorf("sidecar %+v", side)
	}
	if fi, err := os.Stat(dest + ".prev"); err != nil || fi.Mode()&0o111 == 0 {
		t.Errorf(".prev isn't executable: %v", err)
	}
}

func TestRollback(t *testing.T) {
	s := twoVersions(t)
	code, out, errs := runCmd("update", "--rollback", "tuner")
	if code != 0 || out != "Rolled back rhythm-tuner 0.4.0 → 0.3.0\n" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.3.0") || !strings.Contains(s.content("rhythm-tuner.prev"), "rhythm-tuner 0.4.0") {
		t.Error("the binaries weren't swapped")
	}
	// A second rollback swaps back.
	code, out, _ = runCmd("update", "--rollback", "t")
	if code != 0 || out != "Rolled back rhythm-tuner 0.3.0 → 0.4.0\n" {
		t.Errorf("exit %d\n%s", code, out)
	}
	// uninstall takes the .prev files too.
	if code, out, errs := runCmd("uninstall", "tuner"); code != 0 {
		t.Fatalf("uninstall: exit %d\n%s%s", code, out, errs)
	}
	if left := files(t, s.bin); slices.ContainsFunc(left, func(f string) bool { return strings.HasPrefix(f, "rhythm-tuner") }) {
		t.Errorf("left behind: %v", left)
	}
}

func TestRollbackWithoutPrevious(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.0", "")
	code, out, errs := runCmd("update", "--rollback", "tuner")
	if code != 1 || out != "" || errs != "rhythm-tuner has no previous version to roll back to; rhythm keeps one after each update\n" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	code, _, errs = runCmd("update", "--rollback", "echo")
	if code != 1 || !strings.Contains(errs, "rhythm-echo        isn't installed; there is nothing to roll back") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestRollbackRefusesAChangedPrevious(t *testing.T) {
	s := twoVersions(t)
	before := s.content("rhythm-tuner")
	if err := os.WriteFile(filepath.Join(s.bin, "rhythm-tuner.prev"), []byte("#!/bin/sh\necho other\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errs := runCmd("update", "--rollback", "tuner")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner.prev has changed since it was set aside (sha256 differs); not rolling back") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	if s.content("rhythm-tuner") != before {
		t.Error("the binary changed")
	}
}

func TestRollbackAndToRefuseManaged(t *testing.T) {
	s, _ := pinSandbox(t)
	dir := filepath.Join(s.dir, "Cellar", "rhythm-tuner", "0.3.0", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rhythm-tuner")
	if err := os.WriteFile(p, stubScript("rhythm-tuner", "0.3.0", s.log("rhythm-tuner"), ""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p, filepath.Join(s.bin, "rhythm-tuner")); err != nil {
		t.Fatal(err)
	}
	code, _, errs := runCmd("update", "--rollback", "tuner")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner is managed by Homebrew; rhythm leaves its versions to Homebrew") {
		t.Errorf("rollback: exit %d\n%s", code, errs)
	}
	code, out, errs := runCmd("update", "tuner", "--to", "0.2.9", "--no-data")
	if code != 1 || !strings.Contains(out, "rhythm-tuner is managed by Homebrew; update it with: brew upgrade rhythm-tuner") {
		t.Errorf("--to: exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateTo(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.4.0", "")
	code, out, errs := runCmd("update", "tuner", "--to", "0.3.0", "--no-data")
	if code != 0 || !strings.Contains(out, "Note: 0.3.0 is older than rhythm-tuner 0.4.0; installing it as asked.\nrhythm-tuner       0.4.0 → 0.3.0 ✔\n") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.3.0") {
		t.Error("0.3.0 wasn't installed")
	}
	// A partial version picks the newest match; up is fine too.
	code, out, errs = runCmd("update", "tuner", "--to", "0.3", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner       0.3.0 → 0.3.1 ✔\n") || strings.Contains(out, "Note:") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	code, out, _ = runCmd("update", "tuner", "--to", "0.3.1", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner       0.3.1 (up to date)") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// TestUpdateToNeedsAVersion: --to fails when the tool's version can't
// be told or is a development build, since the version asked for wasn't
// installed.
func TestUpdateToNeedsAVersion(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "dev", "")
	code, out, errs := runCmd("update", "tuner", "--to", "0.3.1", "--no-data")
	if code != 1 || !strings.Contains(out, "rhythm-tuner       dev (development build, not updated); replace it with: rhythm install tuner@0.3.1 --force\n") {
		t.Errorf("dev: exit %d\n%s%s", code, out, errs)
	}
	script(t, s.bin, "rhythm-tuner", "echo garbage")
	code, out, errs = runCmd("update", "tuner", "--to", "0.3.1", "--no-data")
	if code != 1 || !strings.Contains(out, "rhythm-tuner       version unknown") {
		t.Errorf("unknown: exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateModeUsage(t *testing.T) {
	pinSandbox(t)
	for _, c := range []struct{ args, want string }{
		{"--rollback", "rhythm: --rollback needs a tool name, such as: rhythm update --rollback tuner"},
		{"--rollback tuner --to 0.3.0", "rhythm: --rollback and --to don't mix; pick one"},
		{"--rollback tuner --check", "rhythm: --rollback works alone; drop --check and --lock"},
		{"--to 0.3.0", "rhythm: --to needs exactly one tool name, such as: rhythm update tuner --to 0.3.1"},
		{"tuner echo --to 0.3.0", "rhythm: --to needs exactly one tool name"},
		{"tuner --to 0.3.0 --check", "rhythm: --to works alone; drop --check and --lock"},
		{"tuner --to 0.3.x", `rhythm: --to: invalid version "0.3.x"`},
	} {
		code, _, errs := runCmd(append([]string{"update"}, strings.Fields(c.args)...)...)
		if code != exitUsage || !strings.Contains(errs, c.want) {
			t.Errorf("%s: exit %d, want 2 and %q\n%s", c.args, code, c.want, errs)
		}
	}
}

// writeExtensions lists exts in extensions.json in a config folder of
// the test's own.
func writeExtensions(t *testing.T, doc string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RHYTHM_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateExtensions(t *testing.T) {
	installed, latest := allTools()
	installed["rhythm-zap"] = "1.0.0"
	latest["someone/zap"] = "1.1.0"
	s := newSandbox(t, installed, latest)
	writeExtensions(t, `{"schema":"rhythm-extensions/v1","extensions":[{"name":"zap","repo":"someone/zap"},{"name":"gone","repo":"someone/gone"}]}`)

	code, out, errs := runCmd("update", "zap")
	if code != 0 || !strings.Contains(out, "rhythm-zap         1.0.0 → 1.1.0 ✔") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-zap"), "rhythm-zap 1.1.0") {
		t.Error("the extension wasn't updated")
	}
	if s.ranUpdate("rhythm-zap") {
		t.Error("a third-party tool without kit/cli got a plain update")
	}
	if s.ranUpdate("rhythm-tuner") {
		t.Error("tuner was touched by update zap")
	}

	// Everything includes the extensions on PATH, after the tools.
	s.stub(t, "rhythm-zap", "1.0.0", "")
	code, out, errs = runCmd("update", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-zap         1.0.0 → 1.1.0 ✔") || strings.Index(out, "rhythm-zap") < strings.Index(out, "rhythm-tuner") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}

	// Named but not on PATH.
	code, _, errs = runCmd("update", "gone")
	if code != 1 || !strings.Contains(errs, "rhythm-gone        isn't on PATH, though extensions.json lists it") {
		t.Errorf("exit %d: %s", code, errs)
	}
	// Not listed at all is still a usage error.
	if code, _, errs := runCmd("update", "nope"); code != 2 || !strings.Contains(errs, `unknown tool "nope"`) {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestUpdateExtensionNeedsItsSigner(t *testing.T) {
	installed := map[string]string{"rhythm-zap": "1.0.0"}
	s := newSandbox(t, installed, map[string]string{"someone/zap": "1.1.0"})
	writeExtensions(t, `{"schema":"rhythm-extensions/v1","extensions":[{"name":"zap","repo":"someone/zap","signer":"https://github.com/someone/zap/.github/workflows/release.yml@refs/heads/main"}]}`)
	useVerifier(t, &fakeVerifier{signer: "https://github.com/mallory/zap/.github/workflows/release.yml@refs/heads/main"})

	code, _, errs := runCmd("update", "zap")
	if code != 1 || !strings.Contains(errs, "refusing to install someone/zap 1.1.0: signed by https://github.com/mallory/") ||
		!strings.Contains(s.content("rhythm-zap"), "1.0.0") {
		t.Errorf("exit %d: %s", code, errs)
	}
	useVerifier(t, &fakeVerifier{signer: "https://github.com/someone/zap/.github/workflows/release.yml@refs/heads/main"})
	if code, out, errs := runCmd("update", "zap"); code != 0 || !strings.Contains(s.content("rhythm-zap"), "1.1.0") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

// TestUpdateExtensionSignerWithoutVerifier: a rhythm that can't check
// signatures refuses an extension that requires a signer, unless
// --skip-signature says to go ahead without it.
func TestUpdateExtensionSignerWithoutVerifier(t *testing.T) {
	installed := map[string]string{"rhythm-zap": "1.0.0"}
	s := newSandbox(t, installed, map[string]string{"someone/zap": "1.1.0"})
	signer := "https://github.com/someone/zap/.github/workflows/release.yml@refs/heads/main"
	writeExtensions(t, `{"schema":"rhythm-extensions/v1","extensions":[{"name":"zap","repo":"someone/zap","signer":"`+signer+`"}]}`)

	code, _, errs := runCmd("update", "zap")
	want := "refusing to install someone/zap 1.1.0: its releases must be signed by " + signer + " and this rhythm can't check signatures; update rhythm, or install without the check with --skip-signature"
	if code != 1 || !strings.Contains(errs, want) || !strings.Contains(s.content("rhythm-zap"), "1.0.0") {
		t.Errorf("exit %d: %s", code, errs)
	}
	code, out, errs := runCmd("update", "zap", "--skip-signature")
	if code != 0 || !strings.Contains(errs, "warning: signature check skipped") || !strings.Contains(s.content("rhythm-zap"), "1.1.0") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestUpdateBadExtensionsFile(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	for doc, want := range map[string]string{
		`{"schema":"rhythm-extensions/v9","extensions":[]}`:                           `schema "rhythm-extensions/v9" isn't "rhythm-extensions/v1"`,
		`{"schema":"rhythm-extensions/v1","extensions":[{"name":"Z!","repo":"a/b"}]}`: `extension name "Z!" isn't a tool name`,
		`{"schema":"rhythm-extensions/v1","extensions":[{"name":"zap","repo":"ab"}]}`: `extension "zap": repo "ab" isn't owner/name`,
		`not json`: "invalid character",
	} {
		writeExtensions(t, doc)
		s.stub(t, "rhythm-tuner", "0.1.0", "")
		code, _, errs := runCmd("update", "--no-data")
		if code != 1 || !strings.Contains(errs, want) || !strings.Contains(errs, "; extensions not updated") || !strings.Contains(errs, "failed 1: extensions") {
			t.Errorf("%s: exit %d: %s", doc, code, errs)
		}
		if !strings.Contains(s.content("rhythm-tuner"), "0.2.0") {
			t.Errorf("%s: the tools weren't updated", doc)
		}
	}
	// A registry name in the file doesn't take the tool from its repo.
	writeExtensions(t, `{"schema":"rhythm-extensions/v1","extensions":[{"name":"tuner","repo":"mallory/tuner"}]}`)
	exts, err := readExtensions()
	if err != nil || len(exts) != 0 {
		t.Errorf("got %v, %v", exts, err)
	}
	t.Setenv("RHYTHM_CONFIG_DIR", "relative")
	if _, err := readExtensions(); err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Errorf("relative RHYTHM_CONFIG_DIR: %v", err)
	}
}

// TestOfflineZeroIsOnline: RHYTHM_OFFLINE=0 switches offline mode off,
// as it does for kit/update.
func TestOfflineZeroIsOnline(t *testing.T) {
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.1.0"}, map[string]string{repoOf("tuner"): "0.2.0"})
	t.Setenv("RHYTHM_OFFLINE", "0")
	code, out, errs := runCmd("update", "tuner", "--check")
	if code != exitOutdated || strings.Contains(errs, "RHYTHM_OFFLINE") || len(s.api.requests()) == 0 {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if !offlineOn("1") || !offlineOn("true") || offlineOn("") || offlineOn("0") {
		t.Error("offlineOn: only \"\" and \"0\" are off")
	}
}

var regenerate = flag.Bool("golden", false, "rewrite the testdata/*.golden.json files")

// golden compares got, with the sandbox paths and the fake API's address
// replaced by placeholders, with testdata/name.
func golden(t *testing.T, name, got string, replace ...string) {
	t.Helper()
	got = strings.NewReplacer(replace...).Replace(got)
	p := filepath.Join("testdata", name)
	if *regenerate {
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs; got\n%s\nwant\n%s", name, got, want)
	}
}

// decodeStrict decodes data into v, refusing unknown fields. It returns
// the keys of every object in arrays under field, to check that each
// required key is present.
func decodeStrict(t *testing.T, data string, v any, field string) []map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	var raw map[string][]map[string]any
	var loose map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &loose); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"x":`+string(loose[field])+`}`), &raw); err != nil {
		t.Fatalf("%s isn't an array of objects: %v", field, err)
	}
	return raw["x"]
}

func requireKeys(t *testing.T, objs []map[string]any, keys ...string) {
	t.Helper()
	for i, o := range objs {
		for _, k := range keys {
			if _, ok := o[k]; !ok {
				t.Errorf("item %d has no %q: %v", i, k, o)
			}
		}
	}
}

func TestUpdateCheckJSON(t *testing.T) {
	installed, latest := allTools()
	latest[repoOf("echo")] = "0.1.0"
	delete(latest, repoOf("soundcheck")) // its lookup fails
	latest[selfRepo] = "0.1.0"
	s := newSandbox(t, installed, latest)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")

	code, out, errs := runCmd("update", "--check", "--json")
	if code != 1 {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "update-check-failed.golden.json", out, s.dir, "$SANDBOX", s.api.URL, "$API")
	if !strings.Contains(errs, "rhythm-tuner       0.1.0 → 0.2.0 available") || strings.Contains(out, "available\n") {
		t.Errorf("the human lines aren't on stderr alone:\n%s", errs)
	}

	var rep updateReportJSON
	objs := decodeStrict(t, out, &rep, "targets")
	if rep.Schema != "rhythm-update/v1" || len(rep.Targets) != 4 {
		t.Errorf("%+v", rep)
	}
	requireKeys(t, objs, "name", "current", "latest", "path", "outdated", "signed", "managed_by", "error")
}

func TestUpdateCheckJSONOutdatedExits10(t *testing.T) {
	installed, latest := allTools()
	latest[repoOf("echo")], latest[repoOf("soundcheck")] = "0.1.0", "0.1.0"
	s := newSandbox(t, installed, latest)

	code, out, errs := runCmd("update", "--check", "--json", "tuner", "echo")
	if code != exitOutdated {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "update-check-outdated.golden.json", out, s.dir, "$SANDBOX")
	var rep updateReportJSON
	requireKeys(t, decodeStrict(t, out, &rep, "targets"), "name", "current", "latest", "path", "outdated", "signed", "managed_by", "error")
	if len(rep.Targets) != 2 || !rep.Targets[1].Outdated || rep.Targets[0].Outdated {
		t.Errorf("%+v", rep.Targets)
	}
}

func TestUpdateJSONNeedsCheck(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	if code, out, errs := runCmd("update", "--json"); code != 2 || out != "" || errs != "rhythm: --json works with --check only\n" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if len(s.api.requests()) != 0 {
		t.Error("a usage error asked the API")
	}
}

// fileSHA256 returns the lowercase hex sha256 of the file at p.
func fileSHA256(p string) (string, error) {
	data, err := os.ReadFile(p) //nolint:gosec // a test file
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
