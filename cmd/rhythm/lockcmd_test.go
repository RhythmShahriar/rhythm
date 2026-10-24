// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/sigverify/sigverifytest"
)

// fakeRel is one release a releaseAPI serves.
type fakeRel struct {
	version    string
	age        time.Duration // how long ago it was published
	prerelease bool
	undated    bool // served without published_at
}

// releaseAPI is a GitHub-style release API with several releases per
// repository: latest, the release list, tags, archives and checksums.
// Each checksums.txt lists this platform's archive and two others.
type releaseAPI struct {
	*httptest.Server
	s     *sandbox
	mu    sync.Mutex
	repos map[string][]fakeRel
	// tampered[repo@version] serves another archive. With sums true, a
	// release that was replaced: checksums.txt matches the new archive.
	tampered map[string]bool
	// data[binary] is the shell a release's "update" runs; with
	// kit[binary] the release is a kit/cli tool (see kitStub).
	data  map[string]string
	kit   map[string]bool
	paths []string
	// onRequest, when set, runs with mu held once each request has been
	// answered, so it can change what the next one gets.
	onRequest func(path string)
	// signer, when set, signs each release's checksums.txt (see
	// useCA); signAs then names who signs repo's releases, the shared
	// workflow when nil. Releases in unsigned have no bundle and those in
	// editedSums serve a checksums.txt changed after it was signed, both
	// keyed repo@version.
	// search is the body /search/repositories answers with.
	search     string
	signer     *sigverifytest.CA
	signAs     func(repo, version string) sigverifytest.Identity
	unsigned   map[string]bool
	editedSums map[string]bool
}

// count is how many times path was asked for.
func (f *releaseAPI) count(path string) int {
	n := 0
	for _, p := range f.requests() {
		if p == path {
			n++
		}
	}
	return n
}

// newReleaseAPI serves repos and points rhythm at it.
func newReleaseAPI(t *testing.T, s *sandbox, repos map[string][]fakeRel) *releaseAPI {
	t.Helper()
	f := &releaseAPI{s: s, repos: repos, tampered: map[string]bool{}, data: map[string]string{}, kit: map[string]bool{},
		unsigned: map[string]bool{}, editedSums: map[string]bool{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	t.Setenv("RHYTHM_GITHUB_API", f.URL)
	return f
}

// otherPlatforms are listed in every checksums.txt besides this one.
var otherPlatforms = []string{"darwin_arm64", "linux_amd64", "windows_amd64"}

// archive returns the archive repo's version ships for this platform.
func (f *releaseAPI) archive(repo, version string) []byte {
	f.mu.Lock()
	tampered := f.tampered[repo+"@"+version]
	f.mu.Unlock()
	return f.build(repo, version, tampered)
}

// build makes the archive, tampered with or not.
func (f *releaseAPI) build(repo, version string, tampered bool) []byte {
	bin := binaryFor(repo)
	f.mu.Lock()
	data, kit := f.data[bin], f.kit[bin]
	f.mu.Unlock()
	body := stubScript(bin, version, f.s.log(bin), data)
	if kit {
		body = kitStub(bin, version, f.s.log(bin), data)
	}
	if tampered {
		body = append(body, "# tampered\n"...)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	// Writes to a bytes.Buffer can't fail.
	_ = tw.WriteHeader(&tar.Header{Name: bin, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = zw.Close()
	return buf.Bytes()
}

// sha is the sha256 of the archive repo's version ships for this platform.
func (f *releaseAPI) sha(repo, version string) string {
	sum := sha256.Sum256(f.archive(repo, version))
	return hex.EncodeToString(sum[:])
}

// otherSHA is the made-up sha256 of another platform's archive.
func otherSHA(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

func (f *releaseAPI) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.paths)
}

func (f *releaseAPI) find(repo, version string) (fakeRel, bool) {
	for _, r := range f.repos[repo] {
		if r.version == version {
			return r, true
		}
	}
	return fakeRel{}, false
}

func (f *releaseAPI) releaseJSON(repo string, r fakeRel) map[string]any {
	bin := binaryFor(repo)
	name := fmt.Sprintf("%s_%s.tar.gz", bin, thisPlatform())
	rel := map[string]any{
		"tag_name":     "v" + r.version,
		"prerelease":   r.prerelease,
		"published_at": time.Now().Add(-r.age).UTC().Format(time.RFC3339),
		"assets": []map[string]string{
			{"name": name, "browser_download_url": fmt.Sprintf("%s/dl/%s/%s", f.URL, repo, r.version)},
			{"name": "checksums.txt", "browser_download_url": fmt.Sprintf("%s/sums/%s/%s", f.URL, repo, r.version)},
		},
	}
	f.mu.Lock()
	signed := f.signer != nil && !f.unsigned[repo+"@"+r.version]
	f.mu.Unlock()
	if signed {
		rel["assets"] = append(rel["assets"].([]map[string]string), map[string]string{
			"name": signatureAsset, "browser_download_url": fmt.Sprintf("%s/sig/%s/%s", f.URL, repo, r.version)})
	}
	if r.undated {
		delete(rel, "published_at")
	}
	return rel
}

func (f *releaseAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.onRequest != nil {
			f.onRequest(r.URL.Path)
		}
	}()
	p := r.URL.Path
	repoVersion := func(prefix string) (string, string) {
		rest := strings.TrimPrefix(p, prefix)
		i := strings.LastIndex(rest, "/")
		return rest[:i], rest[i+1:]
	}
	switch {
	case strings.HasPrefix(p, "/dl/"):
		repo, v := repoVersion("/dl/")
		_, _ = w.Write(f.archive(repo, v))
	case strings.HasPrefix(p, "/sums/"):
		repo, v := repoVersion("/sums/")
		sums := f.sums(repo, v)
		f.mu.Lock()
		edited := f.editedSums[repo+"@"+v]
		f.mu.Unlock()
		if edited {
			sums = append(sums, "0000000000000000000000000000000000000000000000000000000000000000  extra.tar.gz\n"...)
		}
		_, _ = w.Write(sums)
	case p == "/search/repositories":
		f.mu.Lock()
		body := f.search
		f.mu.Unlock()
		_, _ = w.Write([]byte(body))
	case strings.HasPrefix(p, "/sig/"):
		repo, v := repoVersion("/sig/")
		f.mu.Lock()
		ca, as := f.signer, f.signAs
		f.mu.Unlock()
		id := sigverifytest.Shared(repo, v)
		if as != nil {
			id = as(repo, v)
		}
		b, err := ca.Sign(id, f.sums(repo, v))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(b)
	case strings.HasPrefix(p, "/repos/"):
		rest := strings.TrimPrefix(p, "/repos/")
		parts := strings.SplitN(rest, "/", 4) // owner, name, "releases", [latest | tags/vX]
		if len(parts) < 3 || parts[2] != "releases" {
			http.NotFound(w, r)
			return
		}
		repo := parts[0] + "/" + parts[1]
		rels, ok := f.repos[repo]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var out any
		switch {
		case len(parts) == 3:
			var list []map[string]any
			for _, rel := range rels {
				list = append(list, f.releaseJSON(repo, rel))
			}
			out = list
		case parts[3] == "latest":
			var best *fakeRel
			for i, rel := range rels {
				if !rel.prerelease && (best == nil || rel.age < best.age) {
					best = &rels[i]
				}
			}
			if best == nil {
				http.NotFound(w, r)
				return
			}
			out = f.releaseJSON(repo, *best)
		case strings.HasPrefix(parts[3], "tags/v"):
			rel, ok := f.find(repo, strings.TrimPrefix(parts[3], "tags/v"))
			if !ok {
				http.NotFound(w, r)
				return
			}
			out = f.releaseJSON(repo, rel)
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(out)
	default:
		http.NotFound(w, r)
	}
}

// sums is the checksums.txt of repo's version: this platform's archive
// and three others.
func (f *releaseAPI) sums(repo, v string) []byte {
	bin := binaryFor(repo)
	f.mu.Lock()
	keepSums := f.tampered[repo+"@"+v+"/keep-sums"]
	f.mu.Unlock()
	sha := f.sha(repo, v)
	if keepSums {
		sum := sha256.Sum256(f.build(repo, v, false)) // the sum of the untampered archive
		sha = hex.EncodeToString(sum[:])
	}
	lines := map[string]string{fmt.Sprintf("%s_%s.tar.gz", bin, thisPlatform()): sha}
	for _, pl := range otherPlatforms {
		ext := ".tar.gz"
		if strings.HasPrefix(pl, "windows") {
			ext = ".zip"
		}
		name := bin + "_" + pl + ext
		if _, ok := lines[name]; !ok {
			lines[name] = otherSHA(name + v)
		}
	}
	var out []byte
	for _, name := range slices.Sorted(maps.Keys(lines)) {
		out = fmt.Appendf(out, "%s  %s\n", lines[name], name)
	}
	return out
}

// tunerReleases are Tuner's releases in most tests: 0.3.1 is the newest
// 0.3.x, 0.4.0 the latest and 0.4.1-rc.1 a prerelease.
func tunerReleases() []fakeRel {
	return []fakeRel{
		{version: "0.4.1-rc.1", age: time.Hour, prerelease: true},
		{version: "0.4.0", age: 24 * time.Hour},
		{version: "0.3.1", age: 10 * 24 * time.Hour},
		{version: "0.3.0", age: 20 * 24 * time.Hour},
		{version: "0.2.9", age: 30 * 24 * time.Hour},
	}
}

// pinSandbox has no tools installed and serves several releases of
// Tuner and Soundcheck. rhythm itself is 0.7.0 at bin/rhythm.
func pinSandbox(t *testing.T) (*sandbox, *releaseAPI) {
	t.Helper()
	s := newSandbox(t, nil, nil)
	f := newReleaseAPI(t, s, map[string][]fakeRel{
		repoOf("tuner"):      tunerReleases(),
		repoOf("soundcheck"): {{version: "0.4.2", age: 5 * 24 * time.Hour}, {version: "0.4.1", age: 9 * 24 * time.Hour}},
		selfRepo:             {{version: "0.7.0", age: 30 * 24 * time.Hour}, {version: "0.8.0", age: 2 * 24 * time.Hour}},
	})
	fakeSelf(t, s.stub(t, "rhythm", "0.7.0", ""), "0.7.0")
	t.Chdir(t.TempDir())
	return s, f
}

func TestInstallVersionSpecs(t *testing.T) {
	cases := []struct{ spec, want string }{
		{"0.3", "0.3.1"},             // the newest 0.3.x
		{"0.3.0", "0.3.0"},           // exact
		{"v0.2.9", "0.2.9"},          // a leading v is fine
		{"0", "0.4.0"},               // the newest 0.x, not the prerelease
		{"latest", "0.4.0"},          // as with no spec
		{"0.4.1-rc.1", "0.4.1-rc.1"}, // a prerelease, named exactly
	}
	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			s, _ := pinSandbox(t)
			code, out, errs := runCmd("install", "tuner@"+c.spec, "--dir", s.bin)
			if code != 0 {
				t.Fatalf("exit %d\n%s%s", code, out, errs)
			}
			if !strings.Contains(out, "Installed rhythm-tuner "+c.want+" to ") || !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner "+c.want) {
				t.Errorf("want %s installed:\n%s%s", c.want, out, errs)
			}
		})
	}
}

func TestInstallSpecAlreadyInstalled(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	code, out, _ := runCmd("install", "tuner@0.3", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "rhythm-tuner 0.3.1 is already installed") {
		t.Errorf("exit %d\n%s", code, out)
	}
	// Going down a version is an install like any other.
	code, out, _ = runCmd("install", "tuner@0.3.0", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.3.0") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestInstallBadSpecs(t *testing.T) {
	s, f := pinSandbox(t)
	for _, c := range []struct{ args, want string }{
		{"tuner@0.3.x", `rhythm: tuner: invalid version "0.3.x"; use latest, an exact version such as 0.3.1, or 0.3 for the newest 0.3.x`},
		{"tuner@", `rhythm: tuner: invalid version ""`},
		{"tuner@01.2", `invalid version "01.2"`},
		{"tuner@0.3 t@0.4", "rhythm: tuner is named twice with different versions: tuner@0.3 and tuner@0.4"},
		{"tunr@0.3", "rhythm: unknown tool \"tunr\"\n  did you mean: rhythm install tuner\n"},
		{"tuner@0.3 --locked", "rhythm: --locked installs the versions in rhythm.lock; name an exact version or none in tuner@0.3"},
		{"--locked --defaults", "rhythm: --locked installs the tools in rhythm.lock; drop --defaults"},
	} {
		code, out, errs := runCmd(append([]string{"install", "--dir", s.bin}, strings.Fields(c.args)...)...)
		if code != exitUsage || !strings.Contains(errs, c.want) {
			t.Errorf("%s: exit %d, want 2 and %q\n%s%s", c.args, code, c.want, out, errs)
		}
	}
	if r := f.requests(); len(r) > 0 {
		t.Errorf("bad specs reached the API: %v", r)
	}
	// The same version twice is one install.
	code, out, _ := runCmd("install", "tuner@0.3", "t@0.3", "--dry-run")
	if code != 0 || out != "tuner@0.3\n" {
		t.Errorf("dry run: exit %d %q", code, out)
	}
}

func TestInstallSpecWithoutRelease(t *testing.T) {
	s, f := pinSandbox(t)
	code, _, errs := runCmd("install", "tuner@0.9", "--dir", s.bin)
	want := fmt.Sprintf("mirror %s has no release 0.9.x for RhythmShahriar/tuner; check RHYTHM_GITHUB_API", f.URL)
	if code != 1 || !strings.Contains(errs, want) {
		t.Errorf("exit %d\n%s", code, errs)
	}
	code, _, errs = runCmd("install", "tuner@0.9.9", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "has no release v0.9.9 for RhythmShahriar/tuner") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	if got := (&noReleaseError{api: defaultAPI, repo: "RhythmShahriar/tuner", version: "v0.9.9"}).Error(); got != "RhythmShahriar/tuner has no release v0.9.9" {
		t.Errorf("default API: %q", got)
	}
}

// lockGolden compares the lock in the current folder with the golden
// file, with each archive's sha256 replaced by $SHA(<binary> <version>).
// The files list linux_amd64, darwin_arm64 and windows_amd64, so other
// platforms skip the comparison.
func lockGolden(t *testing.T, f *releaseAPI, name string, pinned map[string]string) {
	t.Helper()
	if !slices.Contains(otherPlatforms, thisPlatform()) {
		t.Skipf("the golden lock lists %v", otherPlatforms)
	}
	data, err := os.ReadFile(lockFileName)
	if err != nil {
		t.Fatal(err)
	}
	// Every platform's hash gets a placeholder, so the file reads the
	// same on each of them.
	var repl []string
	for repo, v := range pinned {
		bin := binaryFor(repo)
		place := fmt.Sprintf("$SHA(%s %s)", bin, v)
		repl = append(repl, f.sha(repo, v), place)
		for _, pl := range otherPlatforms {
			ext := ".tar.gz"
			if strings.HasPrefix(pl, "windows") {
				ext = ".zip"
			}
			repl = append(repl, otherSHA(bin+"_"+pl+ext+v), place)
		}
	}
	// golden reads testdata/ relative to the package; the test runs in
	// a folder of its own.
	wd := mustGetwd(t)
	t.Chdir(packageDir)
	golden(t, name, string(data), repl...)
	t.Chdir(wd)
}

// packageDir is the folder go test started in.
var packageDir, _ = os.Getwd()

func TestLockWritesInstalledVersions(t *testing.T) {
	s, f := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.2", "")

	code, out, errs := runCmd("lock")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"rhythm             0.7.0  ",
		"rhythm-soundcheck  0.4.2  ",
		"rhythm-tuner       0.3.1  ",
		"Wrote " + filepath.Join(mustGetwd(t), "rhythm.lock"),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	lockGolden(t, f, "rhythm.lock.golden", map[string]string{
		selfRepo: "0.7.0", repoOf("tuner"): "0.3.1", repoOf("soundcheck"): "0.4.2",
	})
	// The file reads back.
	if _, err := readLock(lockFileName); err != nil {
		t.Error(err)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func TestLockRefusesDevBuildsAndWritesNothing(t *testing.T) {
	s, _ := pinSandbox(t)
	fakeSelf(t, s.stub(t, "rhythm", "dev", ""), "dev")
	s.stub(t, "rhythm-tuner", "dev", "")
	code, out, errs := runCmd("lock")
	if code != 1 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"rhythm             dev is a development build; rhythm itself isn't locked",
		"rhythm-tuner       dev is a development build, can't lock it",
		"rhythm: rhythm.lock not written",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
	if _, err := os.Stat(lockFileName); err == nil {
		t.Error("a lock was written")
	}
}

// writeLock locks the sandbox's current installs.
func writeLock(t *testing.T) {
	t.Helper()
	if code, out, errs := runCmd("lock", "--no-data"); code != 0 {
		t.Fatalf("lock: exit %d\n%s%s", code, out, errs)
	}
}

func TestInstallLocked(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.1", "")
	writeLock(t)
	// Another laptop: newer versions installed, the same lock.
	s.stub(t, "rhythm-tuner", "0.4.0", "")
	s.stub(t, "rhythm-soundcheck", "0.4.2", "")

	code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{"Using " + filepath.Join(mustGetwd(t), "rhythm.lock"), "Installed rhythm-tuner 0.3.1", "Installed rhythm-soundcheck 0.4.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.3.1") {
		t.Error("tuner isn't the locked version")
	}
	// Named tools must be in the lock.
	code, _, errs = runCmd("install", "--locked", "echo", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "rhythm: rhythm.lock doesn't lock echo; add it with: rhythm install echo && rhythm lock") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	code, out, _ = runCmd("install", "--locked", "tuner", "--dry-run")
	if code != 0 || out != "tuner@0.3.1\n" {
		t.Errorf("dry run: exit %d %q", code, out)
	}
	// A version named with --locked must be the locked one.
	code, out, _ = runCmd("install", "--locked", "tuner@0.3.1", "--dry-run")
	if code != 0 || out != "tuner@0.3.1\n" {
		t.Errorf("matching version: exit %d %q", code, out)
	}
	code, _, errs = runCmd("install", "--locked", "tuner@0.4.0", "soundcheck", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "rhythm: rhythm.lock pins tuner 0.3.1, not 0.4.0; drop the version, or move the lock with: rhythm update --lock tuner") {
		t.Errorf("other version: exit %d\n%s", code, errs)
	}
}

func TestInstallLockedTamperedInstallsNothing(t *testing.T) {
	for _, keepSums := range []bool{false, true} {
		t.Run(fmt.Sprintf("checksums-updated=%v", !keepSums), func(t *testing.T) {
			s, f := pinSandbox(t)
			s.stub(t, "rhythm-tuner", "0.3.1", "")
			s.stub(t, "rhythm-soundcheck", "0.4.1", "")
			writeLock(t)
			s.stub(t, "rhythm-tuner", "0.4.0", "")
			s.stub(t, "rhythm-soundcheck", "0.4.2", "")
			before := map[string]string{"rhythm-tuner": s.content("rhythm-tuner"), "rhythm-soundcheck": s.content("rhythm-soundcheck")}

			f.mu.Lock()
			f.tampered[repoOf("soundcheck")+"@0.4.1"] = true
			if keepSums {
				// Only the archive changed: the lock still matches
				// checksums.txt and the archive's own hash catches it.
				f.tampered[repoOf("soundcheck")+"@0.4.1/keep-sums"] = true
			}
			f.mu.Unlock()

			code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
			if code != 1 {
				t.Fatalf("exit %d\n%s%s", code, out, errs)
			}
			want := "rhythm-soundcheck  rhythm-soundcheck_" + thisPlatform() + ".tar.gz: sha256 differs from the locked one; the release may have been replaced or tampered with, nothing was changed"
			if !strings.Contains(errs, want) || !strings.Contains(errs, "Nothing was installed: the releases don't match rhythm.lock.") {
				t.Errorf("stderr:\n%s", errs)
			}
			// Not even the tools that matched were installed.
			for b, c := range before {
				if s.content(b) != c {
					t.Errorf("%s was replaced", b)
				}
			}
		})
	}
}

// TestInstallLockedServesWhatWasChecked swaps the release after the
// first look at it. The lock check reads checksums.txt and the archive
// once; the install must use those bytes, not fetch the files again.
func TestInstallLockedServesWhatWasChecked(t *testing.T) {
	for _, after := range []string{"/sums/", "/dl/"} {
		t.Run(strings.Trim(after, "/"), func(t *testing.T) {
			s, f := pinSandbox(t)
			s.stub(t, "rhythm-tuner", "0.3.1", "")
			writeLock(t)
			s.stub(t, "rhythm-tuner", "0.4.0", "")
			before := s.content("rhythm-tuner")
			f.onRequest = func(p string) {
				if strings.HasPrefix(p, after+repoOf("tuner")+"/0.3.1") {
					f.tampered[repoOf("tuner")+"@0.3.1"] = true
				}
			}

			code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
			if strings.Contains(s.content("rhythm-tuner"), "# tampered") {
				t.Fatalf("the swapped archive was installed: exit %d\n%s%s", code, out, errs)
			}
			if after == "/sums/" {
				// The archive downloaded next is the swapped one.
				if code != 1 || !strings.Contains(errs, "sha256 differs from the locked one") || s.content("rhythm-tuner") != before {
					t.Errorf("exit %d\n%s%s", code, out, errs)
				}
				return
			}
			if code != 0 || !strings.Contains(s.content("rhythm-tuner"), "rhythm-tuner 0.3.1") {
				t.Errorf("exit %d\n%s%s", code, out, errs)
			}
			for _, p := range f.requests() {
				if strings.HasPrefix(p, "/dl/") && f.count(p) > 1 {
					t.Errorf("%s was downloaded more than once", p)
				}
			}
		})
	}
}

// TestInstallLockedChecksTheInstalledBinary edits the installed binary
// so it still reports the locked version: --locked replaces it.
func TestInstallLockedChecksTheInstalledBinary(t *testing.T) {
	s, _ := pinSandbox(t)
	if code, out, errs := runCmd("install", "tuner@0.3.1", "--dir", s.bin); code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	writeLock(t)
	code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "rhythm-tuner 0.3.1 is already installed") {
		t.Fatalf("the locked binary wasn't kept: exit %d\n%s%s", code, out, errs)
	}
	p := filepath.Join(s.bin, "rhythm-tuner")
	if err := os.WriteFile(p, []byte(s.content("rhythm-tuner")+"# edited\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runCmd("install", "--locked", "--dir", s.bin)
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.3.1") || strings.Contains(s.content("rhythm-tuner"), "# edited") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestLockedBinary(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("dist/rhythm-tuner.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("MZ"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := lockedBinary(buf.Bytes(), "rhythm-tuner.exe"); err != nil || string(got) != "MZ" {
		t.Errorf("zip: %q %v", got, err)
	}
	if _, err := lockedBinary(buf.Bytes(), "rhythm-echo.exe"); err == nil {
		t.Error("zip: found a binary that isn't there")
	}
	f := &releaseAPI{s: &sandbox{}, data: map[string]string{}, kit: map[string]bool{}}
	if got, err := lockedBinary(f.build(repoOf("tuner"), "1.0.0", false), "rhythm-tuner"); err != nil || !strings.Contains(string(got), "rhythm-tuner 1.0.0") {
		t.Errorf("tar.gz: %q %v", got, err)
	}
	if _, err := lockedBinary([]byte("not an archive"), "rhythm-tuner"); err == nil {
		t.Error("read a binary from junk")
	}
}

func TestInstallLockedMissingPlatform(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	writeLock(t)
	lock, err := readLock(lockFileName)
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Tools[0].Platforms, thisPlatform())
	lock.Tools[0].Platforms["plan9_mips"] = otherSHA("x")
	if err := lock.Write(lockFileName); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
	want := "rhythm-tuner       0.3.1: rhythm.lock has no archive for " + thisPlatform() + " (it locks "
	if code != 1 || !strings.Contains(errs, want) || !strings.Contains(errs, "Nothing was installed") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestInstallLockedNeedsALock(t *testing.T) {
	s, _ := pinSandbox(t)
	code, _, errs := runCmd("install", "--locked", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "rhythm: no rhythm.lock in this folder or its parents up to the repository root; write one with: rhythm lock") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	if err := os.WriteFile(lockFileName, []byte(`{"schema":"rhythm-lock/v2","tools":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errs = runCmd("install", "--locked", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, `schema "rhythm-lock/v2" isn't "rhythm-lock/v1"; a newer rhythm may read it`) {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestFindLockStopsAtRepoRoot(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(root, lockFileName))
	if _, err := findLock(sub); err == nil {
		t.Error("outside a repository only the folder itself counts")
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := findLock(sub); err == nil {
		t.Error("found a lock above the repository root")
	}
	touch(t, filepath.Join(repo, lockFileName))
	if p, err := findLock(sub); err != nil || p != filepath.Join(repo, lockFileName) {
		t.Errorf("got %q, %v", p, err)
	}
}

func TestUpdateLock(t *testing.T) {
	s, f := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.2", "")
	writeLock(t)
	before, _ := os.ReadFile(lockFileName)
	installedBefore := s.content("rhythm-tuner")

	// --check prints the diff, exits 10 and leaves the file alone.
	code, out, errs := runCmd("update", "--lock", "--check")
	wantDiff := "~ rhythm 0.7.0 → 0.8.0\n~ tuner 0.3.1 → 0.4.0\n"
	if code != exitOutdated || !strings.HasPrefix(out, wantDiff) {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if after, _ := os.ReadFile(lockFileName); string(after) != string(before) {
		t.Error("--check rewrote the lock")
	}

	code, out, errs = runCmd("update", "--lock")
	if code != 0 || out != wantDiff+"Updated "+filepath.Join(mustGetwd(t), "rhythm.lock")+"\n" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	lockGolden(t, f, "rhythm.lock.updated.golden", map[string]string{
		selfRepo: "0.8.0", repoOf("tuner"): "0.4.0", repoOf("soundcheck"): "0.4.2",
	})
	if s.content("rhythm-tuner") != installedBefore {
		t.Error("update --lock replaced a binary")
	}
	// Now current.
	code, out, _ = runCmd("update", "--lock", "--check")
	if code != 0 || !strings.HasSuffix(out, "rhythm.lock is current\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestUpdateLockNamedTool(t *testing.T) {
	s, _ := pinSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	writeLock(t)
	code, out, errs := runCmd("update", "tuner", "--lock")
	if code != 0 || !strings.HasPrefix(out, "~ tuner 0.3.1 → 0.4.0\nUpdated ") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	lock, _ := readLock(lockFileName)
	if lock.Rhythm.Version != "0.7.0" {
		t.Errorf("rhythm moved: %s", lock.Rhythm.Version)
	}
	code, _, errs = runCmd("update", "echo", "--lock")
	if code != 1 || !strings.Contains(errs, "rhythm: rhythm.lock doesn't lock echo") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	code, _, errs = runCmd("update", "--lock", "--check", "--json")
	if code != exitUsage || !strings.Contains(errs, "--json doesn't work with --lock") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestLockDiff(t *testing.T) {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	old := &lockFile{Tools: []lockEntry{
		{Name: "tuner", Version: "0.3.1", Platforms: map[string]string{"linux_amd64": sha("a")},
			Data: []dataPack{{Name: "checks", Version: "1", SHA256: sha("b")}, {Name: "vulns", Version: "1", SHA256: sha("c")}}},
		{Name: "zap", Version: "1.0.0"},
		{Name: "soundcheck", Version: "0.4.2", Platforms: map[string]string{"linux_amd64": sha("a")}},
	}}
	next := &lockFile{Rhythm: &lockEntry{Name: "rhythm", Version: "0.7.0"}, Tools: []lockEntry{
		{Name: "tuner", Version: "0.3.2", Platforms: map[string]string{"linux_amd64": sha("a")},
			Data: []dataPack{{Name: "checks", Version: "2", SHA256: sha("b")}, {Name: "vulns", Version: "1", SHA256: sha("d")}, {Name: "abuse", Version: "1", SHA256: sha("e")}}},
		{Name: "soundcheck", Version: "0.4.2", Platforms: map[string]string{"linux_amd64": sha("f")}},
		{Name: "echo", Version: "0.1.0"},
	}}
	got := strings.Join(lockDiff(old, next), "\n")
	want := strings.Join([]string{
		"+ rhythm 0.7.0",
		"+ echo 0.1.0",
		"! soundcheck 0.4.2: archives changed for the same version",
		"~ tuner 0.3.1 → 0.3.2",
		"~ tuner data abuse (none) → 1",
		"~ tuner data checks 1 → 2",
		"! tuner data vulns 1: content changed for the same version",
		"- zap 1.0.0",
	}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReadLockRejects(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ body, want string }{
		{`{"tools":[]}`, `no schema; want "rhythm-lock/v1"`},
		{`{"schema":"rhythm-lock/v1","tools":[],"extra":1}`, `unknown field "extra"`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b","version":"v1.0.0","platforms":{"linux_amd64":"abc"}}]}`,
			`tools[0] (tuner): version "v1.0.0" isn't an exact version such as 1.2.3`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"abc"}}]}`,
			`tools[0] (tuner): platforms["linux_amd64"]: sha256 "abc" isn't 64 lowercase hex digits`},
		{`{"schema":"rhythm-lock/v1","rhythm":{"name":"rhythm2","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"H"}},"tools":[]}`,
			`rhythm: name "rhythm2" isn't "rhythm"`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"rhythm","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"H"}}]}`,
			`tools[0] (rhythm): "rhythm" isn't a tool name`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"Tuner","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"H"}}]}`,
			`tools[0] (Tuner): "Tuner" isn't a tool name`},
		// The repo becomes part of the release API path.
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b/../../../user","version":"1.0.0","platforms":{"linux_amd64":"H"}}]}`,
			`tools[0] (tuner): repo "a/b/../../../user" isn't owner/name`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/..","version":"1.0.0","platforms":{"linux_amd64":"H"}}]}`,
			`tools[0] (tuner): repo "a/.." isn't owner/name`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b?x=1","version":"1.0.0","platforms":{"linux_amd64":"H"}}]}`,
			`tools[0] (tuner): repo "a/b?x=1" isn't owner/name`},
		// A "," or "=" would add pins to update --pin name=version,...
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"H"},"data":[{"name":"checks","version":"1,vulns=0","sha256":"H"}]}]}`,
			`tools[0] (tuner): data "checks" version "1,vulns=0" may only use letters, digits and . _ + -`},
		{`{"schema":"rhythm-lock/v1","tools":[{"name":"tuner","repo":"a/b","version":"1.0.0","platforms":{"linux_amd64":"H"},"data":[{"name":"a b","version":"1","sha256":"H"}]}]}`,
			`tools[0] (tuner): data "a b" version "1" may only use letters, digits and . _ + -`},
	} {
		c.body = strings.ReplaceAll(c.body, `"H"`, `"`+otherSHA("x")+`"`)
		p := filepath.Join(dir, lockFileName)
		if err := os.WriteFile(p, []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readLock(p); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.body, err, c.want)
		}
	}
}

func TestPlatformsOf(t *testing.T) {
	got := platformsOf("rhythm-tuner", map[string]string{
		"rhythm-tuner_linux_amd64.tar.gz":       "a",
		"rhythm-tuner_windows_arm64.zip":        "b",
		"rhythm-tuner-extra_linux_amd64.tar.gz": "c",
		"rhythm-tuner_linux.tar.gz":             "d",
		"rhythm-tuner_linux_amd64.deb":          "e",
		"rhythm_linux_amd64.tar.gz":             "f",
	})
	if len(got) != 2 || got["linux_amd64"] != "a" || got["windows_arm64"] != "b" {
		t.Errorf("got %v", got)
	}
}

func TestUpdateLockMinAge(t *testing.T) {
	s, _ := pinSandbox(t)
	t.Setenv(minAgeEnv, "")
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	writeLock(t)
	code, out, errs := runCmd("update", "--lock", "--check", "--min-age", "3d")
	if code != 0 || !strings.HasSuffix(out, "rhythm.lock is current\n") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"rhythm 0.8.0 is 2 days old; keeping 0.7.0 (min age 3d)",
		"rhythm-tuner 0.4.0 is 1 day old; keeping 0.3.1 (min age 3d)",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}

// tunerData is the rhythm-data/v1 report of the kit stubs.
const tunerData = `{"schema":"rhythm-data/v1","data":[` +
	`{"name":"vulns","version":"2026-10-24","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},` +
	`{"name":"checks","version":"2026.10.20","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`

// kitStub stands in for a tool on kit/cli.Main: version --json names a
// kit version and update runs data, by default printing tunerData. It
// logs its arguments like stubScript.
func kitStub(binary, version, log, data string) []byte {
	if data == "" {
		data = "echo '" + tunerData + "'"
	}
	return fmt.Appendf(nil, `#!/bin/sh
echo "$*" >> '%s'
case "$1" in
version) if [ "$2" = --json ]; then echo '{"schema":"rhythm-version/v1","name":"%s","version":"%s","kit_version":"v0.7.0"}'; else echo '%s %s'; fi ;;
update) %s ;;
*) echo '%s %s' ;;
esac
`, log, binary, version, binary, version, data, binary, version)
}

// kitTool installs a kit stub for binary at version in the sandbox.
func (s *sandbox) kitTool(t *testing.T, binary, version, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.bin, binary), kitStub(binary, version, s.log(binary), data), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDataLine(t *testing.T) {
	s, f := pinSandbox(t)
	f.kit["rhythm-tuner"] = true
	s.kitTool(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.2", "echo legacy data refreshed")
	code, out, errs := runCmd("update", "tuner", "soundcheck")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	// The binary went to 0.4.0 first, so the line names that version.
	if !strings.Contains(out, "rhythm-tuner 0.4.0  data: checks 2026.10.20, vulns 2026-10-24  ok\n") {
		t.Errorf("no data line:\n%s", out)
	}
	if !slices.Contains(s.calls("rhythm-tuner"), "update --json") {
		t.Errorf("tuner calls: %v", s.calls("rhythm-tuner"))
	}
	// A tool without kit/cli still refreshes, the old way.
	if !strings.Contains(out, "legacy data refreshed") || !slices.Contains(s.calls("rhythm-soundcheck"), "update") {
		t.Errorf("legacy refresh:\n%s\n%v", out, s.calls("rhythm-soundcheck"))
	}
}

func TestUpdateDataReportUnreadable(t *testing.T) {
	s, _ := pinSandbox(t)
	s.kitTool(t, "rhythm-tuner", "0.4.0", "echo done")
	code, _, errs := runCmd("update", "tuner")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner  data refresh failed (no rhythm-data/v1 report)") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestLockRecordsData(t *testing.T) {
	s, _ := pinSandbox(t)
	s.kitTool(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.2", "")
	code, out, errs := runCmd("lock")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(out, "rhythm-tuner 0.3.1  data: checks 2026.10.20, vulns 2026-10-24  ok") {
		t.Errorf("no data line:\n%s", out)
	}
	lock, err := readLock(lockFileName)
	if err != nil {
		t.Fatal(err)
	}
	tuner, sc := lock.Entry("tuner"), lock.Entry("soundcheck")
	want := []dataPack{
		{Name: "checks", Version: "2026.10.20", SHA256: strings.Repeat("b", 64)},
		{Name: "vulns", Version: "2026-10-24", SHA256: strings.Repeat("c", 64)},
	}
	if !slices.Equal(tuner.Data, want) || len(sc.Data) != 0 {
		t.Errorf("tuner data %v, soundcheck data %v", tuner.Data, sc.Data)
	}
	// A failed refresh stops the lock.
	s.kitTool(t, "rhythm-tuner", "0.3.1", "exit 3")
	if err := os.Remove(lockFileName); err != nil {
		t.Fatal(err)
	}
	code, _, errs = runCmd("lock")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner  data refresh failed (exit 3)\n  lock without the tools' data with: rhythm lock --no-data") {
		t.Errorf("exit %d\n%s", code, errs)
	}
	if _, err := os.Stat(lockFileName); err == nil {
		t.Error("a lock was written")
	}
}

// lockedWithData locks Tuner 0.3.1 with its data and leaves 0.4.0
// installed, served as a kit tool when kit is true.
func lockedWithData(t *testing.T, kit bool, data string) *sandbox {
	t.Helper()
	s, f := pinSandbox(t)
	f.mu.Lock()
	f.kit["rhythm-tuner"], f.data["rhythm-tuner"] = kit, data
	f.mu.Unlock()
	s.kitTool(t, "rhythm-tuner", "0.3.1", "")
	if code, out, errs := runCmd("lock"); code != 0 {
		t.Fatalf("lock: exit %d\n%s%s", code, out, errs)
	}
	s.stub(t, "rhythm-tuner", "0.4.0", "")
	if err := os.Remove(s.log("rhythm-tuner")); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInstallLockedPinsData(t *testing.T) {
	s := lockedWithData(t, true, "")
	code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if !slices.Contains(s.calls("rhythm-tuner"), "update --json --pin checks=2026.10.20,vulns=2026-10-24") {
		t.Errorf("calls: %v", s.calls("rhythm-tuner"))
	}
	if !strings.Contains(out, "rhythm-tuner 0.3.1  data: checks 2026.10.20, vulns 2026-10-24  ok") || errs != "" {
		t.Errorf("output:\n%s%s", out, errs)
	}
}

func TestInstallLockedDataDiffersFromPin(t *testing.T) {
	s := lockedWithData(t, true, `echo '{"schema":"rhythm-data/v1","data":[{"name":"checks","version":"2026.10.21","sha256":"`+strings.Repeat("b", 64)+`"}]}'`)
	code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"rhythm-tuner data checks is 2026.10.21, not the locked 2026.10.20",
		"rhythm-tuner data vulns is (none), not the locked 2026-10-24",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}

func TestInstallLockedToolsWithoutPins(t *testing.T) {
	const warn = "rhythm-tuner can't pin its data; refreshing it to the latest instead\n"
	t.Run("legacy", func(t *testing.T) {
		s := lockedWithData(t, false, "echo legacy refresh")
		code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
		if code != 0 || errs != warn || !strings.Contains(out, "legacy refresh") {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
		if !slices.Contains(s.calls("rhythm-tuner"), "update") {
			t.Errorf("calls: %v", s.calls("rhythm-tuner"))
		}
	})
	t.Run("kit without --pin", func(t *testing.T) {
		s := lockedWithData(t, true, `case "$*" in *--pin*) echo 'unknown flag: --pin' >&2; exit 2 ;; esac; echo '`+tunerData+`'`)
		code, out, errs := runCmd("install", "--locked", "--dir", s.bin)
		if code != 0 || errs != warn || !strings.Contains(out, "data: checks 2026.10.20, vulns 2026-10-24  ok") {
			t.Errorf("exit %d\n%s%s", code, out, errs)
		}
		calls := s.calls("rhythm-tuner")
		if !slices.Contains(calls, "update --json --pin checks=2026.10.20,vulns=2026-10-24") || !slices.Contains(calls, "update --json") {
			t.Errorf("calls: %v", calls)
		}
	})
}

func TestInstallLockedChecksTheRegistry(t *testing.T) {
	s, f := pinSandbox(t)
	sha := strings.Repeat("a", 64)
	lock := &lockFile{Tools: []lockEntry{
		{Name: "tuner", Repo: "someone/tuner", Version: "0.3.1", Platforms: map[string]string{thisPlatform(): sha}},
		{Name: "zap", Repo: "someone/rhythm-zap", Version: "1.0.0", Platforms: map[string]string{thisPlatform(): sha}},
	}}
	if err := lock.Write(lockFileName); err != nil {
		t.Fatal(err)
	}
	code, _, errs := runCmd("install", "--locked", "--dir", s.bin)
	for _, want := range []string{
		"rhythm: rhythm.lock says tuner comes from someone/tuner; rhythm installs it from RhythmShahriar/tuner\n",
		"rhythm: rhythm.lock locks zap, which this rhythm doesn't know; update rhythm first\n",
	} {
		if code != 1 || !strings.Contains(errs, want) {
			t.Errorf("exit %d, stderr lacks %q:\n%s", code, want, errs)
		}
	}
	if r := f.requests(); len(r) > 0 {
		t.Errorf("asked the API: %v", r)
	}
}

// TestUpdateLockChecksTheRegistry: update --lock looks nothing up for a
// lock that moves rhythm or a registry tool to another repository.
func TestUpdateLockChecksTheRegistry(t *testing.T) {
	_, f := pinSandbox(t)
	sha := strings.Repeat("a", 64)
	lock := &lockFile{
		Rhythm: &lockEntry{Name: selfName, Repo: "someone/rhythm", Version: "0.7.0", Platforms: map[string]string{thisPlatform(): sha}},
		Tools: []lockEntry{
			{Name: "tuner", Repo: "rhythmshahriar/TUNER", Version: "0.3.1", Platforms: map[string]string{thisPlatform(): sha}},
		},
	}
	if err := lock.Write(lockFileName); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runCmd("update", "--lock", "--check")
	want := "rhythm: rhythm.lock says rhythm comes from someone/rhythm; rhythm installs it from RhythmShahriar/rhythm\n"
	if code != 1 || errs != want || out != "" {
		t.Errorf("exit %d\n%s%s\nwant\n%s", code, out, errs, want)
	}
	if r := f.requests(); len(r) > 0 {
		t.Errorf("asked the API: %v", r)
	}
}
