// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRel is one release a releaseAPI serves.
type fakeRel struct {
	tag        string
	draft, pre bool
	published  time.Time // zero: an hour ago
	undated    bool      // served without published_at
}

// releaseAPI plays the GitHub release API for RhythmShahriar/tuner. Each
// release ships an archive whose binary reads "rhythm-tuner <version>"
// and a checksums.txt with CRLF line ends. /releases/latest is the newest
// non-draft, non-prerelease release by publish time, as GitHub picks it.
type releaseAPI struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func newReleaseAPI(t *testing.T, rels ...fakeRel) *releaseAPI {
	t.Helper()
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	api := &releaseAPI{hits: map[string]int{}}
	files := map[string][]byte{}
	type asset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	type ghRel struct {
		TagName    string    `json:"tag_name"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
		Published  *string   `json:"published_at"`
		Assets     []asset   `json:"assets"`
		at         time.Time // for picking latest
	}
	var list []ghRel
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.hits[r.URL.Path]++
		api.mu.Unlock()
		p := r.URL.Path
		switch {
		case p == "/repos/RhythmShahriar/tuner/releases":
			json.NewEncoder(w).Encode(list)
			return
		case p == "/repos/RhythmShahriar/tuner/releases/latest":
			var best *ghRel
			for i, rel := range list {
				if !rel.Draft && !rel.Prerelease && (best == nil || rel.at.After(best.at)) {
					best = &list[i]
				}
			}
			if best != nil {
				json.NewEncoder(w).Encode(best)
				return
			}
		case strings.HasPrefix(p, "/repos/RhythmShahriar/tuner/releases/tags/"):
			tag := strings.TrimPrefix(p, "/repos/RhythmShahriar/tuner/releases/tags/")
			for _, rel := range list {
				if rel.TagName == tag {
					json.NewEncoder(w).Encode(rel)
					return
				}
			}
		case files[p] != nil:
			w.Write(files[p])
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(api.Close)
	for _, fr := range rels {
		at := fr.published
		if at.IsZero() {
			at = time.Now().Add(-time.Hour)
		}
		version := strings.TrimPrefix(fr.tag, "v")
		archive := tarGz(t, "rhythm-tuner", []byte("rhythm-tuner "+version))
		sum := sha256.Sum256(archive)
		files["/dl/"+fr.tag+"/archive"] = archive
		files["/dl/"+fr.tag+"/sums"] = fmt.Appendf(nil, "%s  rhythm-tuner_windows_amd64.zip\r\n%s  %s\r\n",
			strings.Repeat("ab", 32), hex.EncodeToString(sum[:]), name)
		published := at.UTC().Format(time.RFC3339)
		rel := ghRel{TagName: fr.tag, Draft: fr.draft, Prerelease: fr.pre, at: at, Assets: []asset{
			{name, api.URL + "/dl/" + fr.tag + "/archive"},
			{"checksums.txt", api.URL + "/dl/" + fr.tag + "/sums"},
		}}
		if !fr.draft && !fr.undated {
			rel.Published = &published
		}
		list = append(list, rel)
	}
	return api
}

// archiveHits counts the archive downloads of every release.
func (a *releaseAPI) archiveHits() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for p, c := range a.hits {
		if strings.HasSuffix(p, "/archive") {
			n += c
		}
	}
	return n
}

func (a *releaseAPI) updater() *Updater {
	return &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: a.URL}
}

// pinned serves 0.3.0, 0.3.1, 0.4.0-rc1, 0.4.0 and a draft 0.5.0.
func pinned(t *testing.T) *releaseAPI {
	return newReleaseAPI(t,
		fakeRel{tag: "v0.3.0", published: time.Now().Add(-40 * 24 * time.Hour)},
		fakeRel{tag: "v0.3.1", published: time.Now().Add(-30 * 24 * time.Hour)},
		fakeRel{tag: "v0.4.0-rc1", pre: true, published: time.Now().Add(-20 * 24 * time.Hour)},
		fakeRel{tag: "v0.4.0", published: time.Now().Add(-10 * 24 * time.Hour)},
		fakeRel{tag: "v0.5.0", draft: true},
	)
}

func TestResolve(t *testing.T) {
	u := pinned(t).updater()
	for _, tc := range []struct{ spec, want string }{
		{"", "0.4.0"},
		{"latest", "0.4.0"},
		{"LATEST", "0.4.0"},
		{"0.3", "0.3.1"},
		{"v0.3", "0.3.1"},
		{"0", "0.4.0"},
		{"0.4", "0.4.0"}, // the rc is skipped for a partial spec
		{"0.3.0", "0.3.0"},
		{"v0.3.0", "0.3.0"},
		{"0.4.0-rc1", "0.4.0-rc1"},
	} {
		rel, err := u.Resolve(context.Background(), tc.spec)
		if err != nil || rel.Version != tc.want {
			t.Errorf("Resolve(%q) = %+v, %v; want %s", tc.spec, rel, err, tc.want)
		}
	}
}

func TestResolveFindsNothing(t *testing.T) {
	u := pinned(t).updater()
	for _, tc := range []struct{ spec, version string }{
		{"0.9", "0.9.*"},
		{"0.9.9", "v0.9.9"},
		{"0.5.0", "v0.5.0"}, // a draft
		{"0.5", "0.5.*"},
	} {
		_, err := u.Resolve(context.Background(), tc.spec)
		var nr *ErrNoRelease
		msg := "mirror " + u.API + " has no release " + tc.version + " for RhythmShahriar/tuner"
		if !errors.As(err, &nr) || nr.Version != tc.version || err.Error() != msg {
			t.Errorf("Resolve(%q) err = %v, want ErrNoRelease %q", tc.spec, err, msg)
		}
	}
}

func TestResolveRejectsBadSpecs(t *testing.T) {
	u := pinned(t).updater()
	for _, spec := range []string{"0.3.x", "1.2.3.4", "0.4-rc1", "0.4.0-", "abc", "-1", "01.2", "0..1"} {
		_, err := u.Resolve(context.Background(), spec)
		if err == nil || !strings.Contains(err.Error(), "invalid version") {
			t.Errorf("Resolve(%q) err = %v, want invalid version", spec, err)
		}
	}
}

func TestByTag(t *testing.T) {
	api := pinned(t)
	u := api.updater()
	for _, v := range []string{"0.4.0-rc1", "v0.4.0-rc1"} {
		rel, err := u.ByTag(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		if rel.Version != "0.4.0-rc1" || rel.Tag != "v0.4.0-rc1" || !rel.Prerelease || rel.Published.IsZero() {
			t.Errorf("ByTag(%q) = %+v", v, rel)
		}
	}
	_, err := u.ByTag(context.Background(), "0.9.9")
	var nr *ErrNoRelease
	if !errors.As(err, &nr) || nr.Version != "v0.9.9" {
		t.Fatalf("missing tag: err = %v, want ErrNoRelease", err)
	}
	var st *StatusError
	if !errors.As(err, &st) || st.Code != 404 {
		t.Errorf("missing tag does not wrap the 404: %v", err)
	}
	if got := (&ErrNoRelease{Repo: u.Repo, API: defaultAPI, Version: "v0.9.9"}).Error(); got != "RhythmShahriar/tuner has no release v0.9.9" {
		t.Errorf("message on GitHub = %q", got)
	}
}

// TestByTagKeepsThePath checks that a version can't reshape the request
// sent to the API host along with the token. ByTag refuses it before any
// request; the GitHub source escapes it for callers that skip ByTag.
func TestByTagKeepsThePath(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		mu.Unlock()
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	bad := []string{"0.3.1/../x", "0.3.1/../../x", "0.3.1?per_page=1", "0.3.1#x", "0.3.1-rc1/x", "0.3.1+b/x", "0.3", "latest", "", "0.3.1-rc 1", "0.3.1-é"}
	for _, v := range bad {
		_, err := u.ByTag(context.Background(), v)
		if err == nil || !strings.Contains(err.Error(), "invalid version") {
			t.Errorf("ByTag(%q) err = %v, want invalid version", v, err)
		}
	}
	mu.Lock()
	if len(seen) != 0 {
		t.Errorf("refused versions still sent %q", seen)
	}
	seen = nil
	mu.Unlock()

	_, _ = gitHub{u}.ByTag(context.Background(), "0.3.1/../x?y#z")
	mu.Lock()
	defer mu.Unlock()
	want := "/repos/RhythmShahriar/tuner/releases/tags/v0.3.1%2F..%2Fx%3Fy%23z?"
	if len(seen) != 1 || seen[0] != want {
		t.Errorf("source sent %q, want [%q]", seen, want)
	}
}

func TestExactVersion(t *testing.T) {
	for v, want := range map[string]bool{
		"0.3.1":          true,
		"0.4.0-rc1":      true,
		"1.2.3-rc.1+b.5": true,
		"10.20.30":       true,
		"0.3":            false,
		"0.3.1-":         false,
		"0.3.1/x":        false,
		"01.2.3":         false,
		"0.3.1-rc_1":     false,
		"0.3.1-rc%2F":    false,
	} {
		if got := exactVersion(v); got != want {
			t.Errorf("exactVersion(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestChecksumsListsEveryPlatform(t *testing.T) {
	u := pinned(t).updater()
	rel, err := u.ByTag(context.Background(), "0.3.1")
	if err != nil {
		t.Fatal(err)
	}
	sums, err := u.Checksums(context.Background(), rel)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(sums))
	for n, s := range sums {
		names = append(names, n)
		if !isSHA256(s) || strings.ContainsAny(s, "\r\n") {
			t.Errorf("%s: sha %q", n, s)
		}
	}
	slices.Sort(names)
	want := []string{u.ArchiveName(), "rhythm-tuner_windows_amd64.zip"}
	slices.Sort(want)
	want = slices.Compact(want)
	if !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if sums["rhythm-tuner_windows_amd64.zip"] != strings.Repeat("ab", 32) {
		t.Errorf("windows sha = %q", sums["rhythm-tuner_windows_amd64.zip"])
	}
}

func TestParseChecksums(t *testing.T) {
	a, b := strings.Repeat("A", 64), strings.Repeat("b", 64)
	got := parseChecksums([]byte(a + "  one.tar.gz\r\n" + b + " *two.zip\r\n\r\nnot a line\n" + strings.Repeat("z", 64) + "  bad.zip\n"))
	if len(got) != 2 || got["one.tar.gz"] != strings.ToLower(a) || got["two.zip"] != b {
		t.Errorf("parseChecksums = %v", got)
	}
}

func TestChecksumsVerifiesTheSignature(t *testing.T) {
	rel, _, _ := signedRelease(t, true)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Verifier: &fakeVerifier{signer: workflow}}
	_, err := u.Checksums(context.Background(), rel)
	var se *ErrSignature
	if !errors.As(err, &se) {
		t.Errorf("unsigned checksums.txt: err = %v, want ErrSignature", err)
	}
}

func TestInstallWithExpectedSHA256(t *testing.T) {
	u := pinned(t).updater()
	rel, err := u.ByTag(context.Background(), "0.3.1")
	if err != nil {
		t.Fatal(err)
	}
	sums, err := u.Checksums(context.Background(), rel)
	if err != nil {
		t.Fatal(err)
	}
	dest := installed(t)

	// A lock that disagrees with the archive wins over checksums.txt.
	wrong := strings.Repeat("0", 64)
	_, err = u.InstallWith(context.Background(), rel, dest, InstallOptions{ExpectSHA256: wrong})
	var ce *ErrChecksum
	if !errors.As(err, &ce) || !ce.Locked || ce.Want != wrong || ce.Got != sums[u.ArchiveName()] {
		t.Fatalf("err = %v (%+v), want a locked ErrChecksum", err, ce)
	}
	if !strings.Contains(err.Error(), "differs from the locked one") {
		t.Errorf("message = %q", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Errorf("dest = %q, want it untouched", got)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("files = %v, want only the old binary", names)
	}

	if _, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{ExpectSHA256: "nope"}); err == nil || !strings.Contains(err.Error(), "not 64 hex digits") {
		t.Errorf("bad hash: err = %v", err)
	}

	// The matching lock installs, in either case.
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{ExpectSHA256: strings.ToUpper(sums[u.ArchiveName()])})
	if err != nil || res.Version != "0.3.1" || res.SHA256 != sums[u.ArchiveName()] {
		t.Fatalf("InstallWith = %+v, %v", res, err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "rhythm-tuner 0.3.1" {
		t.Errorf("dest = %q", got)
	}
}

func TestPinningIsOffline(t *testing.T) {
	isolate(t)
	t.Setenv("RHYTHM_OFFLINE", "1")
	api := pinned(t)
	u := api.updater()
	if _, err := u.Resolve(context.Background(), "0.3"); !errors.Is(err, ErrOffline) {
		t.Errorf("Resolve err = %v", err)
	}
	if _, err := u.ByTag(context.Background(), "0.3.1"); !errors.Is(err, ErrOffline) {
		t.Errorf("ByTag err = %v", err)
	}
	rel := &Release{Version: "0.3.1", Assets: map[string]string{"checksums.txt": api.URL + "/dl/v0.3.1/sums"}}
	if _, err := u.Checksums(context.Background(), rel); !errors.Is(err, ErrOffline) {
		t.Errorf("Checksums err = %v", err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.hits) != 0 {
		t.Errorf("requests while offline: %v", api.hits)
	}
}

func TestByTagRefusesAnotherRelease(t *testing.T) {
	// A mirror that answers /releases/tags/v0.2.0 with the 0.1.0 release
	// must not downgrade a pinned install.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/RhythmShahriar/tuner/releases/tags/v0.2.0", "/repos/RhythmShahriar/tuner/releases/tags/v0.1.0":
			fmt.Fprint(w, `{"tag_name":"v0.1.0","assets":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	_, err := u.ByTag(context.Background(), "0.2.0")
	var tm *ErrTagMismatch
	if !errors.As(err, &tm) || tm.Want != "v0.2.0" || tm.Got != "v0.1.0" {
		t.Fatalf("err = %v, want ErrTagMismatch", err)
	}
	if want := srv.URL + ` answered RhythmShahriar/tuner v0.2.0 with release "v0.1.0"; refusing it`; err.Error() != want {
		t.Errorf("message = %q, want %q", err, want)
	}
	if _, err := u.Resolve(context.Background(), "0.2.0"); !errors.As(err, &tm) {
		t.Errorf("Resolve: err = %v, want ErrTagMismatch", err)
	}
	if rel, err := u.ByTag(context.Background(), "0.1.0"); err != nil || rel.Version != "0.1.0" {
		t.Errorf("the matching tag: %+v, %v", rel, err)
	}
}
