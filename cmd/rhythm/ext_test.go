// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/internal/lock"
	"github.com/RhythmShahriar/rhythm/kit/sigverify/sigverifytest"
)

const (
	zapRepo   = "someone/rhythm-zap"
	zapSigner = "https://github.com/someone/rhythm-zap/.github/workflows/release.yml"
)

// extSandbox is signedSandbox plus an extension, someone/rhythm-zap,
// whose releases 1.0.0 and 1.1.0 its own release workflow signs. The
// test gets a config folder of its own for extensions.json.
func extSandbox(t *testing.T) (*sandbox, *releaseAPI) {
	t.Helper()
	freshConfig(t)
	s, f := signedSandbox(t)
	f.mu.Lock()
	f.repos[zapRepo] = []fakeRel{{version: "1.1.0", age: time.Hour}, {version: "1.0.0", age: 48 * time.Hour}}
	f.signAs = func(repo, v string) sigverifytest.Identity {
		if repo != zapRepo {
			return sigverifytest.Shared(repo, v)
		}
		return sigverifytest.Identity{SAN: zapSigner + "@refs/tags/v" + v, Repo: repo, Ref: "refs/tags/v" + v}
	}
	f.mu.Unlock()
	return s, f
}

func extensions(t *testing.T) []lock.Extension {
	t.Helper()
	store, err := lock.OpenExtensions()
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestExtInstallRecordsTheSigner(t *testing.T) {
	s, _ := extSandbox(t)
	code, out, errs := runCmd("ext", "install", zapRepo+"@1.0.0", "--dir", s.bin)
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"Installed rhythm-zap 1.0.0 to " + filepath.Join(s.bin, "rhythm-zap") + "\n",
		"  Signature ok  someone/rhythm-zap @ refs/tags/v1.0.0 (release.yml)\n",
		"Run it with: rhythm zap\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	got := extensions(t)
	if len(got) != 1 || got[0] != (lock.Extension{Name: "zap", Repo: zapRepo, Signer: zapSigner}) {
		t.Errorf("extensions.json: %+v", got)
	}
	// rhythm runs it and ext list shows it.
	if code, _, _ := runCmd("zap", "hello"); code != 0 || !strings.Contains(strings.Join(s.calls("rhythm-zap"), "\n"), "hello") {
		t.Errorf("rhythm zap: exit %d", code)
	}
	code, out, _ = runCmd("ext", "list")
	if code != 0 || !strings.Contains(out, "zap   1.0.0    yes     someone/rhythm-zap  "+filepath.Join(s.bin, "rhythm-zap")) {
		t.Errorf("ext list: exit %d\n%s", code, out)
	}
	code, out, _ = runCmd("ext", "list", "--json")
	var rep struct {
		Schema     string `json:"schema"`
		Extensions []struct {
			Name, Repo, Signer, Version, Path, Signed string
		} `json:"extensions"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Schema != "rhythm-ext-list/v1" || len(rep.Extensions) != 1 || rep.Extensions[0].Signer != zapSigner {
		t.Errorf("ext list --json: exit %d\n%s", code, out)
	}

	// update picks up the newer release, signed by the same workflow at
	// its own tag.
	code, out, errs = runCmd("update", "zap", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-zap         1.0.0 → 1.1.0 ✔") {
		t.Errorf("update: exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-zap"), "rhythm-zap 1.1.0") {
		t.Error("the newer extension wasn't installed")
	}
}

// rhythm list shows the signature an installed extension has on record,
// as ext list does, while a rhythm-* program rhythm didn't install stays
// unchecked.
func TestListShowsAnExtensionsSignature(t *testing.T) {
	s, _ := extSandbox(t)
	if code, out, errs := runCmd("ext", "install", zapRepo+"@1.0.0", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	script(t, s.bin, "rhythm-other", "echo rhythm-other 2.0.0")
	code, out, errs := runCmd("list", "--offline")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	row := func(name string) string {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, name+" (3rd)") {
				return strings.Join(strings.Fields(l), " ")
			}
		}
		return ""
	}
	if got, want := row("zap"), "zap (3rd) 1.0.0 yes "+filepath.Join(s.bin, "rhythm-zap"); got != want {
		t.Errorf("zap row %q, want %q in\n%s", got, want, out)
	}
	if got, want := row("other"), "other (3rd) 2.0.0 - "+filepath.Join(s.bin, "rhythm-other"); got != want {
		t.Errorf("other row %q, want %q in\n%s", got, want, out)
	}
	code, out, _ = runCmd("list", "--offline", "--json")
	var rep listReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil {
		t.Fatalf("--json: exit %d\n%s", code, out)
	}
	for _, tl := range rep.Tools {
		if want := map[string]string{"zap": "yes", "other": ""}[tl.Name]; tl.ThirdParty && tl.Signed != want {
			t.Errorf("%s: signed %q, want %q", tl.Name, tl.Signed, want)
		}
	}
	// A changed binary no longer matches what was verified.
	appendTo(t, filepath.Join(s.bin, "rhythm-zap"), "# tampered\n")
	if _, out, _ = runCmd("list", "--offline"); !strings.HasPrefix(row("zap"), "zap (3rd) 1.0.0 no ") {
		t.Errorf("tampered zap shows %q in\n%s", row("zap"), out)
	}
}

// A later release signed by another workflow, even one in the same
// repository, is refused: the first install pinned the signer.
func TestExtUpdateRefusesAnotherSigner(t *testing.T) {
	s, f := extSandbox(t)
	if code, out, errs := runCmd("ext", "install", zapRepo+"@1.0.0", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	f.mu.Lock()
	f.signAs = func(repo, v string) sigverifytest.Identity {
		return sigverifytest.Identity{SAN: "https://github.com/someone/rhythm-zap/.github/workflows/other.yml@refs/tags/v" + v, Repo: repo, Ref: "refs/tags/v" + v}
	}
	f.mu.Unlock()
	code, out, errs := runCmd("update", "zap", "--no-data")
	if code != 1 || !strings.Contains(errs, "refusing to install someone/rhythm-zap 1.1.0: signed by https://github.com/someone/rhythm-zap/.github/workflows/other.yml@refs/tags/v1.1.0, not "+zapSigner) {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(s.content("rhythm-zap"), "rhythm-zap 1.0.0") {
		t.Error("the extension was replaced")
	}
}

func TestExtInstallRefusesUnsignedAndWronglySigned(t *testing.T) {
	s, f := extSandbox(t)
	f.unsigned[zapRepo+"@1.1.0"] = true
	code, out, errs := runCmd("ext", "install", zapRepo, "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "signature check failed: the release has no checksums.txt.sigstore.json") || strings.Contains(out, "Installed") {
		t.Errorf("unsigned: exit %d\n%s%s", code, out, errs)
	}
	f.unsigned[zapRepo+"@1.1.0"] = false
	f.mu.Lock()
	f.signAs = func(repo, v string) sigverifytest.Identity {
		return sigverifytest.Identity{SAN: "https://github.com/mallory/rhythm-zap/.github/workflows/release.yml@refs/tags/v" + v, Repo: repo, Ref: "refs/tags/v" + v}
	}
	f.mu.Unlock()
	code, out, errs = runCmd("ext", "install", zapRepo, "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "signature check failed") {
		t.Errorf("wrongly signed: exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(s.bin, "rhythm-zap")); err == nil {
		t.Error("an unverified extension was installed")
	}
	if got := extensions(t); len(got) != 0 {
		t.Errorf("recorded %+v", got)
	}
}

// An extension may call rhythm's shared release workflow instead.
func TestExtInstallSharedWorkflow(t *testing.T) {
	s, f := extSandbox(t)
	f.mu.Lock()
	f.signAs = nil // every release signed by the shared workflow
	f.mu.Unlock()
	if code, out, errs := runCmd("ext", "install", zapRepo, "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if got := extensions(t); len(got) != 1 || got[0].Signer != sharedSigner {
		t.Errorf("extensions.json: %+v", got)
	}
}

func TestExtInstallRefusesTakenNames(t *testing.T) {
	s, f := extSandbox(t)
	for spec, want := range map[string]string{
		"someone/rhythm-tuner":  "can't install someone/rhythm-tuner as an extension: tuner is one of rhythm's own tools (RhythmShahriar/tuner)",
		"someone/rhythm-sc":     "sc is short for rhythm's own soundcheck",
		"someone/rhythm-list":   "list is a rhythm command (rhythm list)",
		"someone/rhythm-rhythm": "rhythm is the router's own name",
		"someone/zap":           "its name must start with rhythm-",
		"someone/rhythm-zap@x":  `invalid version "x"`,
	} {
		code, out, errs := runCmd("ext", "install", spec, "--dir", s.bin)
		if code != exitUsage || !strings.Contains(errs, want) {
			t.Errorf("%s: exit %d\n%s%s\nwant %q", spec, code, out, errs, want)
		}
	}
	if r := f.requests(); len(r) != 0 {
		t.Errorf("a refused name reached the API: %v", r)
	}
	// Another repository can't take a name an extension already has.
	if code, out, errs := runCmd("ext", "install", zapRepo, "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	code, _, errs := runCmd("ext", "install", "mallory/rhythm-zap", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "zap is already installed from someone/rhythm-zap; remove it first with: rhythm ext remove zap") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestExtRemove(t *testing.T) {
	s, _ := extSandbox(t)
	if code, out, errs := runCmd("ext", "install", zapRepo+"@1.0.0", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if code, out, errs := runCmd("update", "zap", "--no-data"); code != 0 { // leaves rhythm-zap.prev
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	code, out, errs := runCmd("ext", "remove", "zap")
	if code != 0 || !strings.Contains(out, "Removed "+filepath.Join(s.bin, "rhythm-zap")+"\n") || !strings.Contains(out, "Removed zap from ") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	for _, f := range []string{"rhythm-zap", "rhythm-zap.prev"} {
		if _, err := os.Stat(filepath.Join(s.bin, f)); err == nil {
			t.Errorf("%s is still there", f)
		}
	}
	if got := extensions(t); len(got) != 0 {
		t.Errorf("still recorded: %+v", got)
	}
	code, _, errs = runCmd("ext", "remove", "zap")
	if code != 1 || !strings.Contains(errs, `no extension "zap" is installed`) {
		t.Errorf("again: exit %d\n%s", code, errs)
	}
	code, _, errs = runCmd("ext", "remove", "tuner")
	if code != 1 || !strings.Contains(errs, "tuner is one of rhythm's own tools; remove it with: rhythm uninstall tuner") {
		t.Errorf("tuner: exit %d\n%s", code, errs)
	}
}

func TestExtSearch(t *testing.T) {
	_, f := extSandbox(t)
	f.mu.Lock()
	f.search = `{"items":[
		{"full_name":"someone/rhythm-zap","name":"rhythm-zap","description":"zaps \u001b[31mthings\u001b[0m\tin columns","stargazers_count":12},
		{"full_name":"someone/rhythm-tuner","name":"rhythm-tuner","description":"a lookalike","stargazers_count":3},
		{"full_name":"other/thing","name":"thing","description":"not one","stargazers_count":99}]}`
	f.mu.Unlock()
	code, out, errs := runCmd("ext", "search", "zap")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	want := "NAME                REPO                  STARS  DESCRIPTION\n" +
		"zap                 someone/rhythm-zap    12     zaps \uFFFD[31mthings\uFFFD[0m in columns\n" +
		"tuner (name taken)  someone/rhythm-tuner  3      a lookalike\n" +
		"Install one with: rhythm ext install someone/rhythm-zap\n"
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	code, out, _ = runCmd("ext", "search", "--json")
	if code != 0 || !strings.Contains(out, `"schema": "rhythm-ext-search/v1"`) || !strings.Contains(out, `"taken": "tuner is one of rhythm's own tools (RhythmShahriar/tuner)"`) {
		t.Errorf("--json: exit %d\n%s", code, out)
	}
	// The API out of reach is said once.
	t.Setenv("RHYTHM_GITHUB_API", closedPort(t))
	code, _, errs = runCmd("ext", "search", "zap")
	if code != 1 || !strings.Contains(errs, "can't reach") {
		t.Errorf("unreachable: exit %d\n%s", code, errs)
	}
}

// The suggestion names the first result that can be installed, or none
// when every name is taken.
func TestExtSearchSuggestsAnInstallableOne(t *testing.T) {
	_, f := extSandbox(t)
	for _, c := range []struct{ items, want string }{
		{`{"full_name":"someone/rhythm-tuner","name":"rhythm-tuner","stargazers_count":30},
		  {"full_name":"someone/rhythm-list","name":"rhythm-list","stargazers_count":20},
		  {"full_name":"someone/rhythm-zap","name":"rhythm-zap","stargazers_count":10}`,
			"Install one with: rhythm ext install someone/rhythm-zap\n"},
		{`{"full_name":"someone/rhythm-tuner","name":"rhythm-tuner","stargazers_count":30},
		  {"full_name":"someone/rhythm-list","name":"rhythm-list","stargazers_count":20}`,
			""},
	} {
		f.mu.Lock()
		f.search = `{"items":[` + c.items + `]}`
		f.mu.Unlock()
		code, out, errs := runCmd("ext", "search")
		if code != 0 || errs != "" {
			t.Fatalf("exit %d\n%s%s", code, out, errs)
		}
		got := ""
		if i := strings.Index(out, "Install one with:"); i >= 0 {
			got = out[i:]
		}
		if got != c.want {
			t.Errorf("suggestion %q, want %q in\n%s", got, c.want, out)
		}
		if !strings.Contains(out, "tuner (name taken)") || !strings.Contains(out, "list (name taken)") {
			t.Errorf("taken names not marked:\n%s", out)
		}
	}
}

func TestExtUsage(t *testing.T) {
	freshConfig(t)
	for _, c := range []struct{ args, want string }{
		{"", "usage: rhythm ext search|install|list|remove"},
		{"serch", "unknown ext command \"serch\"\n  did you mean: rhythm ext search\n  use one of: search, install, list, remove\n"},
		{"install", "name one extension"},
		{"install a/rhythm-b c/rhythm-d", "name one extension"},
		{"remove", "name one extension"},
		{"list zap", "ext list takes no names"},
	} {
		code, out, errs := runCmd(append([]string{"ext"}, strings.Fields(c.args)...)...)
		if code != exitUsage || !strings.Contains(errs, c.want) {
			t.Errorf("ext %s: exit %d\n%s%s", c.args, code, out, errs)
		}
	}
	if code, out, _ := runCmd("ext", "--help"); code != 0 || !strings.Contains(out, "rhythm ext install owner/rhythm-<name>[@version]") {
		t.Errorf("help: exit %d\n%s", code, out)
	}
	if code, out, _ := runCmd("ext", "list"); code != 0 || out != "No extensions installed. Find some with: rhythm ext search\n" {
		t.Errorf("empty list: exit %d %q", code, out)
	}
}

// rhythm lock pins extensions too (with their signer) and install
// --locked brings them back on a fresh machine.
func TestLockedExtension(t *testing.T) {
	s, _ := extSandbox(t)
	if code, out, errs := runCmd("ext", "install", zapRepo+"@1.0.0", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	writeLock(t)
	l, err := readLock(lockFileName)
	if err != nil {
		t.Fatal(err)
	}
	e := l.Entry("zap")
	if e == nil || e.Source != "extension" || e.Signer != zapSigner || e.Repo != zapRepo || e.Version != "1.0.0" {
		t.Fatalf("lock entry %+v", e)
	}

	// A fresh machine: no extensions.json, nothing installed.
	freshConfig(t)
	dir := t.TempDir()
	code, out, errs := runCmd("install", "--locked", "--dir", dir)
	if code != 0 || !strings.Contains(out, "Installed rhythm-zap 1.0.0") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if got := extensions(t); len(got) != 1 || got[0].Signer != zapSigner {
		t.Errorf("extensions.json: %+v", got)
	}

	// A lock can't use an extension to replace one of rhythm's tools.
	l.Tools = append(l.Tools, lockEntry{Name: "echo", Repo: "mallory/rhythm-echo", Version: "1.0.0", Source: "extension",
		Platforms: map[string]string{thisPlatform(): strings.Repeat("a", 64)}})
	if err := l.Write(lockFileName); err != nil {
		t.Fatal(err)
	}
	code, _, errs = runCmd("install", "--locked", "--dir", dir)
	if code != 1 || !strings.Contains(errs, "lists the extension echo from mallory/rhythm-echo, which can't be installed: echo is one of rhythm's own tools") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}
