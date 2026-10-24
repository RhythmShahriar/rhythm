// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/ratelimit"
	"github.com/RhythmShahriar/rhythm/kit/secrets"
	"github.com/RhythmShahriar/rhythm/kit/term"
)

// ErrBodyTooLarge is returned (wrapped) by a response body read past
// Options.MaxBody. The bytes up to the limit have been returned by then.
var ErrBodyTooLarge = errors.New("response body too large")

// transport is the RoundTripper behind every client from New. Redirect
// hops come through it one by one, so each hop is paced, scoped and
// logged like a first request.
type transport struct {
	base    http.RoundTripper
	opts    Options
	headers http.Header
	hosts   *ratelimit.Hosts
	maxBody int64
	// sleep waits between retries; tests replace it with a fake clock.
	sleep func(context.Context, time.Duration) error
	log   *logger
}

// RoundTrip sends req, retrying after a 429, a 503 or a network error up
// to Options.Retries times.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := t.prepare(req)
	ctx := r.Context()
	retryable := idempotent(r.Method) && (r.Body == nil || r.Body == http.NoBody || r.GetBody != nil)
	lim := t.hosts.For(r.URL.Host)
	for n := 0; ; n++ {
		if err := lim.Wait(ctx); err != nil {
			return nil, err
		}
		att := r
		if n > 0 {
			att = r.Clone(ctx)
			if r.GetBody != nil {
				body, err := r.GetBody()
				if err != nil {
					return nil, err
				}
				att.Body = body
			}
		}
		t.log.request(att)
		began := time.Now()
		resp, err := t.base.RoundTrip(att)
		t.log.response(att, resp, err, time.Since(began))
		if err != nil {
			if !retryable || n >= t.opts.Retries || ctx.Err() != nil || errors.Is(err, ErrPrivate) {
				return nil, err
			}
			if err := t.sleep(ctx, ratelimit.Backoff(n)); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			// The server asked for less traffic: every later request
			// to this host goes slower, retried or not.
			lim.Slow()
			if retryable && n < t.opts.Retries {
				wait := ratelimit.RetryAfter(resp.Header.Get("Retry-After"), n)
				// Drain a little so the connection can be reused.
				_, _ = io.CopyN(io.Discard, resp.Body, 4<<10)
				_ = resp.Body.Close()
				if err := t.sleep(ctx, wait); err != nil {
					return nil, err
				}
				continue
			}
		}
		if t.maxBody > 0 && resp.Body != nil {
			resp.Body = &limitedBody{rc: resp.Body, left: t.maxBody, limit: t.maxBody}
		}
		return resp, nil
	}
}

// prepare copies req (a RoundTripper must not change it) and applies the
// user agent and the header scope.
func (t *transport) prepare(req *http.Request) *http.Request {
	r := req.Clone(req.Context())
	if t.opts.UserAgent != "" && r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", t.opts.UserAgent)
	}
	t.scopeHeaders(r)
	return r
}

// scopeHeaders adds the user's headers when SameHost(Target, r.URL)
// holds and removes them everywhere else. Removal matters because Go's
// client copies the first request's headers onto each redirect hop and
// keeps Authorization and Cookie when only the port changes, and a
// caller may have set them on the request itself. The credential
// headers in offTarget are always removed off the target for the same
// reason.
func (t *transport) scopeHeaders(r *http.Request) {
	if t.opts.Target == nil {
		return
	}
	if SameHost(t.opts.Target, r.URL) {
		for k, vs := range t.headers {
			if k == "Host" {
				r.Host = vs[len(vs)-1]
				continue
			}
			r.Header[k] = append([]string(nil), vs...)
		}
		return
	}
	for k := range t.headers {
		if k == "Host" {
			r.Host = ""
			continue
		}
		r.Header.Del(k)
	}
	for _, k := range offTarget {
		r.Header.Del(k)
	}
}

// offTarget are the headers that carry credentials and never leave the
// target, whoever set them.
var offTarget = []string{"Authorization", "Cookie", "Cookie2", "Proxy-Authorization"}

// idempotent reports whether a request with method may be sent again.
func idempotent(method string) bool {
	switch method {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// limitedBody returns ErrBodyTooLarge once more than limit bytes have
// been read, after handing out the first limit bytes.
type limitedBody struct {
	rc          io.ReadCloser
	left, limit int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.left < 0 {
		return 0, b.err()
	}
	// Ask for one byte past the limit, to tell a body that ends exactly
	// at the limit from one that goes on.
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.rc.Read(p)
	if int64(n) > b.left {
		n = int(b.left)
		b.left = -1
		return n, b.err()
	}
	b.left -= int64(n)
	return n, err
}

func (b *limitedBody) err() error {
	return fmt.Errorf("%w: more than %d bytes", ErrBodyTooLarge, b.limit)
}

func (b *limitedBody) Close() error { return b.rc.Close() }

// logger writes the debug log. A nil *logger writes nothing.
type logger struct {
	mu sync.Mutex
	w  io.Writer
}

// request logs the method, URL and headers about to be sent.
func (l *logger) request(r *http.Request) {
	if l == nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "> %s %s\n", r.Method, safeURL(r))
	if r.Host != "" && r.Host != r.URL.Host {
		writeHeader(&b, ">", "Host", r.Host)
	}
	writeHeaders(&b, ">", r.Header)
	l.write(b.String())
}

// response logs the status (or error) and response headers.
func (l *logger) response(r *http.Request, resp *http.Response, err error, d time.Duration) {
	if l == nil {
		return
	}
	var b strings.Builder
	if err != nil {
		fmt.Fprintf(&b, "< error %s %s: %s\n", r.Method, safeURL(r), term.Sanitize(secrets.Redact(err.Error())))
	} else {
		fmt.Fprintf(&b, "< %s %s %s (%s)\n", term.Sanitize(resp.Status), r.Method, safeURL(r), d.Round(time.Millisecond))
		writeHeaders(&b, "<", resp.Header)
	}
	l.write(b.String())
}

func (l *logger) write(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.w, s)
}

// safeURL is r's URL with any password and secret-looking query values
// masked.
func safeURL(r *http.Request) string {
	return term.Sanitize(secrets.Redact(r.URL.Redacted()))
}

// writeHeaders logs every header value in name order, masked.
func writeHeaders(b *strings.Builder, dir string, h http.Header) {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		for _, v := range h[k] {
			writeHeader(b, dir, k, v)
		}
	}
}

func writeHeader(b *strings.Builder, dir, name, value string) {
	fmt.Fprintf(b, "%s   %s: %s\n", dir, term.Sanitize(name), term.Sanitize(secrets.RedactHeader(name, value)))
}
