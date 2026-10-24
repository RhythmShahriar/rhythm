// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrUnreachable means a release host could not be reached at all: DNS,
// a refused connection, a proxy or a timeout. Err holds the network error.
type ErrUnreachable struct {
	Host string // host[:port] of the request that failed
	Err  error
}

// Error names the host and suggests what to check.
func (e *ErrUnreachable) Error() string {
	return fmt.Sprintf("can't reach %s; check your network, proxy or RHYTHM_GITHUB_API", e.Host)
}

// Unwrap returns the cause, for errors.Is and errors.As.
func (e *ErrUnreachable) Unwrap() error { return e.Err }

// ErrRateLimited means the release API answered 403 or 429 with
// X-RateLimit-Remaining: 0 or a Retry-After header.
type ErrRateLimited struct {
	Reset    time.Time // when the limit lifts; zero when the server didn't say
	API      string    // the API base that limited us
	TokenSet bool      // a token was sent, so suggesting one won't help
	Err      error     // the HTTP status
}

// Error says when the limit lifts and which token would help.
func (e *ErrRateLimited) Error() string {
	var b strings.Builder
	tokenVar := "GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	if e.API == "" || e.API == defaultAPI {
		b.WriteString("GitHub rate limit reached")
	} else {
		fmt.Fprintf(&b, "rate limit reached at %s", redactURL(e.API))
		tokenVar = "RHYTHM_GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	}
	if !e.TokenSet {
		b.WriteString("; set " + tokenVar)
	}
	if !e.Reset.IsZero() {
		fmt.Fprintf(&b, " (resets %s)", resetTime(e.Reset, time.Now()))
	}
	return b.String()
}

// Unwrap returns the cause, for errors.Is and errors.As.
func (e *ErrRateLimited) Unwrap() error { return e.Err }

// resetTime shows t as a clock time, with the date when it isn't within
// the next day.
func resetTime(t, now time.Time) string {
	t = t.Local()
	if t.Sub(now) < 24*time.Hour {
		return t.Format("15:04")
	}
	return t.Format("Jan 2 15:04")
}

// ErrNoRelease means the release API has no release for Repo, or none
// matching Version. When API is a mirror the message names it, because a
// wrong RHYTHM_GITHUB_API is the usual cause.
type ErrNoRelease struct {
	Repo    string
	API     string // the API base asked
	Version string // the tag or spec asked for, e.g. v0.3.9 or 0.3.*; "" for the latest
	Err     error  // the HTTP status; nil when the API answered but nothing matched
}

// Error names the version asked for and the mirror when there is one.
func (e *ErrNoRelease) Error() string {
	mirror := e.API != "" && e.API != defaultAPI
	switch {
	case e.Version != "" && mirror:
		return fmt.Sprintf("mirror %s has no release %s for %s", redactURL(e.API), e.Version, e.Repo)
	case e.Version != "":
		return fmt.Sprintf("%s has no release %s", e.Repo, e.Version)
	case mirror:
		return fmt.Sprintf("mirror %s has no release for %s", redactURL(e.API), e.Repo)
	}
	return e.Repo + " has no published releases yet"
}

// Unwrap returns the cause, for errors.Is and errors.As.
func (e *ErrNoRelease) Unwrap() error { return e.Err }

// ErrTagMismatch means the release API answered a request for one tag
// with a release of another, so nothing was installed from it.
type ErrTagMismatch struct {
	Repo string
	API  string // the API base asked
	Want string // the tag asked for, e.g. v0.3.1
	Got  string // the tag of the release that came back
}

// Error names both tags and the API that answered.
func (e *ErrTagMismatch) Error() string {
	return fmt.Sprintf("%s answered %s %s with release %q; refusing it", redactURL(e.API), e.Repo, e.Want, e.Got)
}

// ErrTooNew means every release that would match is younger than
// Updater.MinAge, so none was picked.
type ErrTooNew struct {
	Repo   string
	Newest string        // the newest matching version
	Age    time.Duration // how old it is
	MinAge time.Duration
}

// Error names the newest release, its age and the cooldown.
func (e *ErrTooNew) Error() string {
	return fmt.Sprintf("no release of %s is old enough: the newest, %s, is %s old (min age %s)",
		e.Repo, e.Newest, FormatAge(e.Age), FormatAge(e.MinAge))
}

// ErrChecksum means a downloaded archive does not match its expected
// sha256. Nothing is replaced when it is returned.
type ErrChecksum struct {
	Asset     string // archive name
	Want, Got string // hex sha256
	Locked    bool   // Want came from InstallOptions.ExpectSHA256, not checksums.txt
}

// Error says nothing was changed.
func (e *ErrChecksum) Error() string {
	if e.Locked {
		return e.Asset + ": sha256 differs from the locked one; the release may have been replaced or tampered with, nothing was changed"
	}
	return e.Asset + ": checksum mismatch; the download may be corrupt or tampered with, nothing was changed"
}

// redactURL drops the user:password part, the query and the fragment of
// raw, any of which can carry a token, so a URL taken from
// RHYTHM_GITHUB_API can be shown in an error.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Without a host, url.Parse may keep "user:pass@..." as opaque
		// text, so none of it is shown.
		return "(unparsable URL)"
	}
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	return strings.TrimSuffix(u.String(), "/")
}

// StatusError is an HTTP status other than 200. The typed errors above
// wrap it when the status has a known meaning.
type StatusError struct {
	Code int
}

// Error is "HTTP <code>".
func (e *StatusError) Error() string { return "HTTP " + strconv.Itoa(e.Code) }

// netError turns a failed request into ErrUnreachable when the network,
// not the server, is to blame. Cancellation and policy refusals (such as
// a redirect to http) come back unchanged.
func netError(req *http.Request, err error) error {
	if req.Context().Err() != nil || errors.Is(err, context.Canceled) {
		return err
	}
	var op *net.OpError
	var dns *net.DNSError
	var ne net.Error
	if errors.As(err, &op) || errors.As(err, &dns) || (errors.As(err, &ne) && ne.Timeout()) {
		return &ErrUnreachable{Host: req.URL.Host, Err: err}
	}
	return err
}

// statusError explains a non-200 response, turning rate limiting into
// ErrRateLimited.
func (u *Updater) statusError(req *http.Request, resp *http.Response) error {
	status := &StatusError{Code: resp.StatusCode}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return status
	}
	h := resp.Header
	retry := h.Get("Retry-After")
	if h.Get("X-RateLimit-Remaining") != "0" && retry == "" {
		return status // a plain 403: a private repo or a blocked token
	}
	e := &ErrRateLimited{API: u.apiBase(), TokenSet: req.Header.Get("Authorization") != "", Err: status}
	if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil && n > 0 {
		e.Reset = time.Unix(n, 0)
	} else if secs, err := strconv.Atoi(retry); err == nil && secs >= 0 {
		e.Reset = time.Now().Add(time.Duration(secs) * time.Second)
	} else if t, err := http.ParseTime(retry); err == nil {
		e.Reset = t
	}
	return e
}
