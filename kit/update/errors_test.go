// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// replyTransport answers every request with one canned response, so a
// test can play api.github.com.
type replyTransport struct {
	code   int
	header http.Header
}

func (rt replyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: rt.code,
		Header:     rt.header,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Request:    r,
	}, nil
}

func TestRateLimitedFromGitHub(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("RHYTHM_GITHUB_TOKEN", "")
	reset := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", HTTP: &http.Client{Transport: replyTransport{403, h}}}
	_, err := u.Latest(context.Background())
	var rl *ErrRateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v (%T), want *ErrRateLimited", err, err)
	}
	if !rl.Reset.Equal(reset) {
		t.Errorf("Reset = %v, want %v", rl.Reset, reset)
	}
	want := "GitHub rate limit reached; set GITHUB_TOKEN (resets " + reset.Local().Format("15:04") + ")"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
	var st *StatusError
	if !errors.As(err, &st) || st.Code != 403 {
		t.Errorf("cause = %v, want HTTP 403", st)
	}

	// With a token already sent, don't suggest setting one.
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	_, err = u.Latest(context.Background())
	if want := "GitHub rate limit reached (resets " + reset.Local().Format("15:04") + ")"; err == nil || err.Error() != want {
		t.Errorf("with a token: %v, want %q", err, want)
	}
}

func TestRateLimitedRetryAfterOnAMirror(t *testing.T) {
	t.Setenv("RHYTHM_GITHUB_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	before := time.Now()
	_, err := u.Latest(context.Background())
	var rl *ErrRateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want *ErrRateLimited", err)
	}
	if d := rl.Reset.Sub(before); d < 119*time.Second || d > 125*time.Second {
		t.Errorf("Reset is %v away, want about 2m", d)
	}
	want := "rate limit reached at " + srv.URL + "; set RHYTHM_GITHUB_TOKEN (resets " + rl.Reset.Local().Format("15:04") + ")"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
}

func TestPlain403IsNotARateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	_, err := u.Latest(context.Background())
	var rl *ErrRateLimited
	if errors.As(err, &rl) {
		t.Errorf("plain 403 reported as a rate limit: %v", err)
	}
	if err == nil || err.Error() != "checking RhythmShahriar/tuner releases: HTTP 403" {
		t.Errorf("err = %v", err)
	}
}

func TestNoReleaseNamesTheMirror(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL + "/"}
	_, err := u.Latest(context.Background())
	var nr *ErrNoRelease
	if !errors.As(err, &nr) {
		t.Fatalf("err = %v, want *ErrNoRelease", err)
	}
	if want := "mirror " + srv.URL + " has no release for RhythmShahriar/tuner"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
	var st *StatusError
	if !errors.As(err, &st) || st.Code != 404 {
		t.Errorf("cause = %v, want HTTP 404", st)
	}

	gh := &ErrNoRelease{Repo: "RhythmShahriar/tuner", API: defaultAPI}
	if got := gh.Error(); got != "RhythmShahriar/tuner has no published releases yet" {
		t.Errorf("default API: %q", got)
	}
}

func TestUnreachable(t *testing.T) {
	// Take a free port and close it again, so nothing listens there.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: "http://" + addr}
	_, err = u.Latest(context.Background())
	var un *ErrUnreachable
	if !errors.As(err, &un) {
		t.Fatalf("err = %v (%T), want *ErrUnreachable", err, err)
	}
	if want := "can't reach " + addr + "; check your network, proxy or RHYTHM_GITHUB_API"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
	var op *net.OpError
	if !errors.As(err, &op) {
		t.Errorf("cause %v is not the dial error", un.Err)
	}

	// Downloads say the same, under the Install context.
	rel := &Release{Version: "0.2.0", Assets: map[string]string{
		u.ArchiveName(): "http://" + addr + "/a",
		"checksums.txt": "http://" + addr + "/s",
	}}
	err = u.Install(context.Background(), rel, filepath.Join(t.TempDir(), "bin"))
	if want := "downloading checksums: can't reach " + addr + "; check your network, proxy or RHYTHM_GITHUB_API"; err == nil || err.Error() != want {
		t.Errorf("Install err = %v, want %q", err, want)
	}
}

func TestCanceledIsNotUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	_, err := u.Latest(ctx)
	var un *ErrUnreachable
	if errors.As(err, &un) || !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled and not ErrUnreachable", err)
	}
}

func TestChecksumError(t *testing.T) {
	srv := fakeGitHub(t, []byte("EVIL"), true)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	err := installFrom(t, u)
	var ce *ErrChecksum
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *ErrChecksum", err)
	}
	if ce.Asset != u.ArchiveName() || len(ce.Want) != 64 || len(ce.Got) != 64 || ce.Want == ce.Got {
		t.Errorf("ErrChecksum = %+v", ce)
	}
	if want := u.ArchiveName() + ": checksum mismatch; the download may be corrupt or tampered with, nothing was changed"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
}

func TestResetTime(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.Local)
	if got := resetTime(now.Add(2*time.Hour+5*time.Minute), now); got != "14:05" {
		t.Errorf("same day = %q", got)
	}
	if got := resetTime(now.Add(48*time.Hour), now); got != "Oct 26 12:00" {
		t.Errorf("two days out = %q", got)
	}
}

// TestErrorsHideMirrorCredentials: a user:password, a ?token= or a
// #fragment in the API base never shows in an error message.
func TestErrorsHideMirrorCredentials(t *testing.T) {
	for _, api := range []string{
		"https://user:sekret@mirror.example",
		"https://mirror.example?token=sekret",
		"https://mirror.example#sekret",
	} {
		errs := []error{
			&ErrNoRelease{Repo: "o/r", API: api},
			&ErrNoRelease{Repo: "o/r", API: api, Version: "v1.2.3"},
			&ErrRateLimited{API: api},
			checkURL(strings.Replace(api, "https:", "http:", 1)),
		}
		// Without a scheme there is no host to show either.
		if err := checkURL(strings.Replace(api, "https://", "", 1)); err == nil || strings.Contains(err.Error(), "sekret") {
			t.Errorf("%s without a scheme: %v", api, err)
		}
		for _, err := range errs {
			if err == nil {
				t.Fatalf("%s: no error", api)
			}
			if msg := err.Error(); strings.Contains(msg, "sekret") || !strings.Contains(msg, "mirror.example") {
				t.Errorf("%s: %q", api, msg)
			}
		}
	}
	if got := redactURL("://user:sekret@"); got != "(unparsable URL)" {
		t.Errorf("unparsable: %q", got)
	}
	if err := checkURL("http://[::1"); err == nil || strings.Contains(err.Error(), "::1") {
		t.Errorf("bad URL: %v", err)
	}
}
