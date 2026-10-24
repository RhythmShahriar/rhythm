// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// authLog records the Authorization header each path was asked with.
type authLog struct {
	mu   sync.Mutex
	seen map[string]string
}

func (a *authLog) record(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.seen == nil {
		a.seen = map[string]string{}
	}
	a.seen[r.URL.Path] = r.Header.Get("Authorization")
}

func (a *authLog) get(path string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.seen[path]
	return v, ok
}

// mirror is a TLS release server on 127.0.0.1 standing in for a mirror
// or GitHub Enterprise: a host that is not api.github.com.
func mirror(t *testing.T, log *authLog) *httptest.Server {
	t.Helper()
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	archive := tarGz(t, "rhythm-tuner", []byte("NEW BINARY"))
	sum := sha256.Sum256(archive)
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case "/repos/RhythmShahriar/tuner/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":"%s/dl/archive"},
				{"name":"checksums.txt","browser_download_url":"%s/dl/sums"}]}`, name, srv.URL, srv.URL)
		case "/dl/archive":
			w.Write(archive)
		case "/dl/sums":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func installFrom(t *testing.T, u *Updater) error {
	t.Helper()
	rel, err := u.Latest(context.Background())
	if err != nil {
		return err
	}
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	os.WriteFile(dest, []byte("OLD BINARY"), 0o755)
	return u.Install(context.Background(), rel, dest)
}

func TestGitHubTokenIsNotSentToAMirror(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("RHYTHM_GITHUB_TOKEN", "")
	var log authLog
	srv := mirror(t, &log)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL, HTTP: srv.Client()}
	if err := installFrom(t, u); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/repos/RhythmShahriar/tuner/releases/latest", "/dl/archive", "/dl/sums"} {
		if got, ok := log.get(p); !ok || got != "" {
			t.Errorf("%s: asked=%v Authorization=%q, want no token", p, ok, got)
		}
	}
}

func TestMirrorTokenIsSentToTheMirror(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("RHYTHM_GITHUB_TOKEN", "mirror-secret")
	var log authLog
	srv := mirror(t, &log)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL, HTTP: srv.Client()}
	if err := installFrom(t, u); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/repos/RhythmShahriar/tuner/releases/latest", "/dl/archive", "/dl/sums"} {
		if got, _ := log.get(p); got != "Bearer mirror-secret" {
			t.Errorf("%s: Authorization=%q, want the mirror token", p, got)
		}
	}
}

// fakeTransport answers every request itself, so tests can play
// api.github.com without the network.
type fakeTransport struct {
	mu   sync.Mutex
	reqs []*http.Request
	body string
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Request:    r,
	}, nil
}

func (f *fakeTransport) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func TestGitHubTokenIsSentToGitHub(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("RHYTHM_GITHUB_TOKEN", "")
	ft := &fakeTransport{body: `{"tag_name":"v1.0.0","assets":[]}`}
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", HTTP: &http.Client{Transport: ft}}
	if _, err := u.Latest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := ft.reqs[0].Header.Get("Authorization"); got != "Bearer gh-secret" {
		t.Errorf("api.github.com Authorization = %q", got)
	}

	cases := map[string]string{
		"https://api.github.com/repos/x":                  "gh-secret",
		"https://github.com/RhythmShahriar/x/releases/a":  "gh-secret",
		"https://objects.githubusercontent.com/a":         "gh-secret",
		"https://evil.example/a":                          "",
		"https://github.com.evil.example/a":               "",
		"http://github.com/a":                             "",
		"https://release-assets.githubusercontent.com/a":  "",
		"https://uploads.github.com/repos/x/releases/a/b": "",
	}
	for raw, want := range cases {
		p, _ := url.Parse(raw)
		if got := u.token(p); got != want {
			t.Errorf("token(%s) = %q, want %q", raw, got, want)
		}
	}

	// With a mirror as the API, even github.com download hosts get nothing.
	u.API = "https://mirror.example"
	p, _ := url.Parse("https://github.com/a")
	if got := u.token(p); got != "" {
		t.Errorf("token(github.com) with a mirror API = %q, want none", got)
	}
}

func TestPlainHTTPIsRefusedBeforeAnyRequest(t *testing.T) {
	ft := &fakeTransport{body: `{"tag_name":"v1.0.0","assets":[]}`}
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: "http://example.invalid", HTTP: &http.Client{Transport: ft}}
	_, err := u.Latest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing to download over http: http://example.invalid") {
		t.Errorf("err = %v", err)
	}
	if n := ft.count(); n != 0 {
		t.Errorf("%d requests sent over plain http", n)
	}

	for _, ok := range []string{"http://127.0.0.1:20020", "http://localhost:20020", "http://[::1]:20020", "https://example.invalid"} {
		if err := checkURL(ok); err != nil {
			t.Errorf("checkURL(%s) = %v, want allowed", ok, err)
		}
	}
	for _, bad := range []string{"http://example.invalid", "http://10.0.0.1", "ftp://127.0.0.1/x", "file:///etc/passwd", "/relative", "https://"} {
		if err := checkURL(bad); err == nil {
			t.Errorf("checkURL(%s) allowed, want refused", bad)
		}
	}
}

// TestAPIMustBeAPlainBaseURL: user info, a query or a fragment in the
// API base is refused before any request, as the installers refuse it.
func TestAPIMustBeAPlainBaseURL(t *testing.T) {
	for _, api := range []string{
		"https://user:sekret@mirror.example",
		"http://user:sekret@127.0.0.1:20020",
		"https://mirror.example?token=sekret",
		"https://mirror.example?",
		"https://mirror.example#sekret",
	} {
		ft := &fakeTransport{body: `{"tag_name":"v1.0.0","assets":[]}`}
		u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: api, HTTP: &http.Client{Transport: ft}}
		_, err := u.Latest(context.Background())
		if err == nil || !strings.Contains(err.Error(), "release API: must be a plain base URL, without user info, query or fragment") || strings.Contains(err.Error(), "sekret") {
			t.Errorf("%s: err = %v", api, err)
		}
		if n := ft.count(); n != 0 {
			t.Errorf("%s: %d requests sent", api, n)
		}
	}
	for _, ok := range []string{"https://mirror.example/api/v3", "http://127.0.0.1:20020", "https://api.github.com"} {
		if err := CheckAPI(ok); err != nil {
			t.Errorf("CheckAPI(%s) = %v, want allowed", ok, err)
		}
	}
	if err := CheckAPI("http://mirror.example"); err == nil || !strings.Contains(err.Error(), "refusing to download over http") {
		t.Errorf("CheckAPI(http) = %v", err)
	}
}

func TestHTTPDownloadURLIsRefused(t *testing.T) {
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":"http://example.invalid/archive"},
			{"name":"checksums.txt","browser_download_url":"http://example.invalid/sums"}]}`, name)
	}))
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL, HTTP: srv.Client()}
	err := installFrom(t, u)
	if err == nil || !strings.Contains(err.Error(), "refusing to download over http: http://example.invalid/sums") {
		t.Errorf("err = %v", err)
	}
}

func TestRedirectToAnotherHostDropsTheToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("RHYTHM_GITHUB_TOKEN", "mirror-secret")
	var first, target authLog
	// Same 127.0.0.1 host name, different port: Go's own redirect rule
	// would keep the header here, so this exercises the Updater's.
	cdn := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.record(r)
		w.Write([]byte("sums"))
	}))
	defer cdn.Close()
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.record(r)
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/dl", http.StatusFound)
		case "/dl":
			http.Redirect(w, r, cdn.URL+"/dl", http.StatusFound)
		default:
			http.Redirect(w, r, "http://example.invalid/dl", http.StatusFound)
		}
	}))
	defer front.Close()
	// httptest TLS servers share one certificate, so either client trusts both.
	u := &Updater{Binary: "rhythm-tuner", API: front.URL, HTTP: front.Client()}
	if _, err := u.download(context.Background(), urlRelease(front.URL+"/same"), "f", 1<<20); err != nil {
		t.Fatal(err)
	}
	if got, _ := first.get("/dl"); got != "Bearer mirror-secret" {
		t.Errorf("same-host redirect Authorization = %q, want the token", got)
	}
	if got, ok := target.get("/dl"); !ok || got != "" {
		t.Errorf("other host: asked=%v Authorization=%q, want no token", ok, got)
	}

	_, err := u.download(context.Background(), urlRelease(front.URL+"/plain"), "f", 1<<20)
	if err == nil || !strings.Contains(err.Error(), "refusing to download over http: http://example.invalid/dl") {
		t.Errorf("redirect to http: err = %v", err)
	}
}
