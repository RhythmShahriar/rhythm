// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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

func (e *ErrUnreachable) Error() string {
	return fmt.Sprintf("can't reach %s; check your network, proxy or RHYTHM_GITHUB_API", e.Host)
}

func (e *ErrUnreachable) Unwrap() error { return e.Err }

// ErrRateLimited means the release API answered 403 or 429 with
// X-RateLimit-Remaining: 0 or a Retry-After header.
type ErrRateLimited struct {
	Reset    time.Time // when the limit lifts; zero when the server didn't say
	API      string    // the API base that limited us
	TokenSet bool      // a token was sent, so suggesting one won't help
	Err      error     // the HTTP status
}

func (e *ErrRateLimited) Error() string {
	var b strings.Builder
	tokenVar := "GITHUB_TOKEN" //nolint:gosec // a variable name, not a credential
	if e.API == "" || e.API == defaultAPI {
		b.WriteString("GitHub rate limit reached")
	} else {
		fmt.Fprintf(&b, "rate limit reached at %s", e.API)
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

// ErrNoRelease means the release API has no release for Repo. When API is
// a mirror the message names it, because a wrong RHYTHM_GITHUB_API is the
// usual cause.
type ErrNoRelease struct {
	Repo string
	API  string // the API base asked
	Err  error  // the HTTP status
}

func (e *ErrNoRelease) Error() string {
	if e.API == "" || e.API == defaultAPI {
		return e.Repo + " has no published releases yet"
	}
	return fmt.Sprintf("mirror %s has no release for %s", e.API, e.Repo)
}

func (e *ErrNoRelease) Unwrap() error { return e.Err }

// ErrChecksum means a downloaded archive does not match its expected
// sha256. Nothing is replaced when it is returned.
type ErrChecksum struct {
	Asset     string // archive name
	Want, Got string // hex sha256
}

func (e *ErrChecksum) Error() string {
	return e.Asset + ": checksum mismatch; the download may be corrupt or tampered with, nothing was changed"
}

// StatusError is an HTTP status other than 200. The typed errors above
// wrap it when the status has a known meaning.
type StatusError struct {
	Code int
}

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
