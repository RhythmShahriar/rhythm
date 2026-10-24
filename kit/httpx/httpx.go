// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package httpx builds the one HTTP client every Rhythm tool should use
// to talk to a site: polite, scoped to the site and safe to log.
//
// A client from New:
//
//   - paces requests per host with a ratelimit.Limiter and slows a host
//     down when it answers 429 or 503, retrying after its Retry-After
//   - follows redirects only on the target's site (see SameSite)
//   - adds the user's -H headers only to requests for the target's host
//     and port (see SameHost) and strips them, Authorization, cookies and
//     Proxy-Authorization from any other host, even when the caller set
//     them on the request itself
//   - limits every response body to Options.MaxBody
//   - refuses to connect to loopback, link-local and private addresses
//     unless the target is on the same kind of address (see Guard)
//   - logs each request and response to Options.Debug with every header
//     value masked by secrets.RedactHeader
//
// Tools used to assemble these from parts and each copy got one of them
// wrong (credentials sent to another port, off-site redirects followed).
// This package is the single place they are right.
package httpx

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/ratelimit"
)

// Defaults for Options fields left at zero.
const (
	DefaultTimeout = 30 * time.Second
	DefaultMaxBody = 5 << 20
	// MaxRedirects is the most redirect hops followed for one request.
	MaxRedirects = 10
)

// Options configures New. The zero value is a client with no rate limit,
// a 30 s timeout, a 5 MB body limit, same-site redirects and no access to
// private addresses.
type Options struct {
	// UserAgent is sent on every request that doesn't set its own.
	UserAgent string
	// Rate is requests per second per host; 0 means unlimited until a
	// server answers 429 or 503 (see ratelimit.New).
	Rate float64
	// Timeout bounds each request including redirects and reading the
	// body. 0 means DefaultTimeout and a negative value means none.
	Timeout time.Duration
	// MaxBody is the most bytes read from a response body; reading past
	// it returns ErrBodyTooLarge. 0 means DefaultMaxBody and a negative
	// value means no limit.
	MaxBody int64
	// Headers are the user's -H values ("Name: value"). They are sent
	// only where SameHost(Target, url) holds, so they need a Target. Malformed
	// values are skipped; check flags with ParseHeaders first.
	Headers []string
	// Target is the site being checked. It scopes Headers, anchors the
	// same-site redirect rule and decides which private addresses may be
	// reached.
	Target *url.URL
	// OffSiteRedirects follows redirects to other sites. By default a
	// redirect is followed only when SameSite(Target, next) holds (or,
	// without a Target, when it stays on the first request's site).
	OffSiteRedirects bool
	// AllowPrivate lets the client connect to loopback, link-local and
	// private addresses. Without it they are refused unless Target
	// resolves to the same kind of address, so local sites still work.
	AllowPrivate bool
	// Retries is how many times a GET, HEAD or OPTIONS request is sent
	// again after a 429, a 503 or a network error.
	Retries int
	// Debug, when set, receives one line per request and response.
	// Header values are masked.
	Debug io.Writer
	// Transport is the base transport, for tests or a caller's proxy
	// setup. When nil, New builds one from http.DefaultTransport that
	// applies AllowPrivate. A caller's Transport dials as it likes, so
	// AllowPrivate has no effect on it.
	Transport http.RoundTripper
}

// New returns a client for opts. Each call has its own per-host limiters,
// so share one client across a run to keep the pace for a host.
func New(opts Options) *http.Client {
	timeout := opts.Timeout
	switch {
	case timeout == 0:
		timeout = DefaultTimeout
	case timeout < 0:
		timeout = 0
	}
	maxBody := opts.MaxBody
	if maxBody == 0 {
		maxBody = DefaultMaxBody
	}
	base := opts.Transport
	if base == nil {
		base = guardedTransport(opts)
	}
	headers, _ := ParseHeaders(opts.Headers)
	t := &transport{
		base:    base,
		opts:    opts,
		headers: headers,
		hosts:   ratelimit.NewHosts(opts.Rate),
		maxBody: maxBody,
		sleep:   ratelimit.Sleep,
	}
	if opts.Debug != nil {
		t.log = &logger{w: opts.Debug}
	}
	return &http.Client{Transport: t, Timeout: timeout, CheckRedirect: redirectPolicy(opts)}
}

// guardedTransport clones http.DefaultTransport and, unless AllowPrivate
// is set, routes its dials through a Guard for the target host.
func guardedTransport(opts Options) http.RoundTripper {
	tr := &http.Transport{}
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		tr = dt.Clone()
	}
	if !opts.AllowPrivate {
		host := ""
		if opts.Target != nil {
			host = opts.Target.Hostname()
		}
		g := NewGuard(host)
		tr.Proxy = g.Proxy
		tr.DialContext = g.DialContext
	}
	return tr
}

// ErrOffSite is returned (wrapped in a *url.Error) when a redirect leaves
// the site.
var ErrOffSite = errors.New("redirect off the site")

// redirectPolicy stops after MaxRedirects and, unless OffSiteRedirects is
// set, refuses hops that leave the site. Go's own policy copies the
// first request's headers onto every hop, Authorization included when
// only the port changes; the transport strips them again (see
// scopeHeaders).
func redirectPolicy(opts Options) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxRedirects {
			return fmt.Errorf("stopped after %d redirects", MaxRedirects)
		}
		if opts.OffSiteRedirects {
			return nil
		}
		start := via[0].URL
		if opts.Target != nil {
			start = opts.Target
		}
		if !SameSite(start, req.URL) {
			return fmt.Errorf("not following a redirect to %s: %w", req.URL.Redacted(), ErrOffSite)
		}
		return nil
	}
}

// ParseHeaders parses -H values ("Name: value") into a header. A value
// without a colon or with an invalid name is an error naming it, so a
// tool can reject the flag before it sends anything. The value may be
// empty.
func ParseHeaders(values []string) (http.Header, error) {
	h := http.Header{}
	var bad []string
	for _, v := range values {
		name, val, ok := strings.Cut(v, ":")
		name = strings.TrimSpace(name)
		if !ok || !validHeaderName(name) {
			bad = append(bad, fmt.Sprintf("%q", v))
			continue
		}
		h.Add(textproto.CanonicalMIMEHeaderKey(name), strings.TrimSpace(val))
	}
	if len(bad) > 0 {
		return h, fmt.Errorf("invalid header %s, use \"Name: value\"", strings.Join(bad, ", "))
	}
	return h, nil
}

// validHeaderName reports whether name is an RFC 9110 token.
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c > 0x7e || c <= ' ' || strings.ContainsRune(`"(),/:;<=>?@[\]{}`, c) {
			return false
		}
	}
	return true
}

// ParseTarget accepts a URL or a bare hostname. explicitScheme is false
// when the scheme was guessed (https), so callers can fall back to http.
func ParseTarget(target string) (u *url.URL, explicitScheme bool, err error) {
	explicitScheme = strings.Contains(target, "://")
	if !explicitScheme {
		target = "https://" + target
	}
	u, err = url.Parse(target)
	if err != nil || u.Host == "" {
		return nil, false, fmt.Errorf("%q is not a valid URL or hostname", target)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false, fmt.Errorf("unsupported scheme %q, use http or https", u.Scheme)
	}
	return u, explicitScheme, nil
}

// site is a host without "www.", so example.com and www.example.com are
// the same site.
func site(host string) string { return strings.TrimPrefix(strings.ToLower(host), "www.") }

// SameSite reports whether u is on the same site as start: the same host
// (ignoring case and a leading "www."), the same scheme and the same
// port, where a missing port means the scheme's default (80 for http, 443
// for https). Another port on the same host is often another service (an
// admin panel, a database UI), so it counts as another site. One upgrade
// is allowed: an http start on the default port may go to https on the
// default port, because most sites redirect there.
func SameSite(start, u *url.URL) bool {
	if site(start.Hostname()) != site(u.Hostname()) {
		return false
	}
	ss, us := strings.ToLower(start.Scheme), strings.ToLower(u.Scheme)
	sp, up := port(ss, start.Port()), port(us, u.Port())
	if ss == us {
		return sp == up
	}
	return ss == "http" && us == "https" && sp == "80" && up == "443"
}

// SameHost reports whether u may receive credentials meant for target:
// the same host (ignoring case), scheme and port, with the scheme's
// default port filled in. Unlike SameSite it treats www.example.com and
// example.com as different hosts, because a credential meant for one
// should not reach the other, so a target that redirects from the apex
// to www (or back) loses its -H headers on the hop; give the host the
// site redirects to. One upgrade is allowed, the one SameSite allows: an
// http target on the default port may send them to https on the default
// port of the same host, because most sites redirect there and the
// credential is safer over TLS. https never sends them to http.
func SameHost(target, u *url.URL) bool {
	if !strings.EqualFold(target.Hostname(), u.Hostname()) {
		return false
	}
	ts, us := strings.ToLower(target.Scheme), strings.ToLower(u.Scheme)
	tp, up := port(ts, target.Port()), port(us, u.Port())
	if ts == us {
		return tp == up
	}
	return ts == "http" && us == "https" && tp == "80" && up == "443"
}

// port returns p, or the default port for scheme when p is empty.
func port(scheme, p string) string {
	if p != "" {
		return p
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}
