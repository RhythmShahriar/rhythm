// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package ratelimit keeps Rhythm tools polite: an even request pace per
// host (Limiter, Hosts) that slows down when a server asks it to (Slow)
// and the retry waits every tool shares (Backoff, RetryAfter, Sleep).
//
// ParseTarget is still here so tools keep compiling but it moved to
// kit/httpx.
package ratelimit

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Slowest pace a limiter backs off to after repeated throttling.
const Slowest = 2 * time.Second

// MaxRetryWait is the longest Retry-After a tool will honor.
const MaxRetryWait = 30 * time.Second

// MinRate is the slowest pace New accepts, one request per hour. Slower
// rates are clamped to it, because a tiny rate would overflow the
// interval between requests.
const MinRate = 1.0 / 3600

// slowStart is the pace an unlimited limiter falls back to on its first
// Slow, before doubling like any other.
const slowStart = 250 * time.Millisecond

// Limiter spaces requests evenly to stay under a requests-per-second cap.
// A nil *Limiter never waits and never slows down.
type Limiter struct {
	mu    sync.Mutex
	every time.Duration
	next  time.Time
}

// New returns a limiter for rps requests per second. When rps <= 0 (or
// NaN or +Inf) it doesn't wait at all until a server throttles it: Slow
// still applies, so an unlimited run backs off after a 429 or 503. Rates
// below MinRate are clamped to one request per hour.
func New(rps float64) *Limiter {
	if rps <= 0 || math.IsNaN(rps) || math.IsInf(rps, 1) {
		return &Limiter{}
	}
	return &Limiter{every: time.Duration(float64(time.Second) / max(rps, MinRate))}
}

// Validate reports whether rps is a usable rate for a flag or config
// value: 0 means unlimited and anything else must be a finite number of
// at least MinRate.
func Validate(rps float64) error {
	switch {
	case math.IsNaN(rps) || math.IsInf(rps, 0):
		return fmt.Errorf("rate %v is not a number of requests per second", rps)
	case rps < 0:
		return fmt.Errorf("rate %v is negative; use 0 for no limit", rps)
	case rps > 0 && rps < MinRate:
		return fmt.Errorf("rate %v is below one request per hour", rps)
	}
	return nil
}

// Wait blocks until the next request may be sent.
func (l *Limiter) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.every)
	l.mu.Unlock()
	return Sleep(ctx, time.Until(at))
}

// Slow halves the request rate, down to one request every two seconds.
// Call it when a server answers 429 or 503. It never speeds a limiter up:
// one that is already slower than Slowest keeps its pace. An unlimited
// limiter starts at four requests per second.
func (l *Limiter) Slow() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.every <= 0 {
		l.every = slowStart
	} else {
		l.every = max(l.every, min(l.every*2, Slowest))
	}
	l.mu.Unlock()
}

// Hosts hands out one limiter per host, so a crawl that touches several
// hosts (a CDN, a subdomain) is polite to each of them.
type Hosts struct {
	rps float64
	mu  sync.Mutex
	m   map[string]*Limiter
}

// NewHosts returns per-host limiters at rps requests per second each.
func NewHosts(rps float64) *Hosts { return &Hosts{rps: rps, m: map[string]*Limiter{}} }

// For returns the limiter for host (case-insensitive).
func (h *Hosts) For(host string) *Limiter {
	host = strings.ToLower(host)
	h.mu.Lock()
	defer h.mu.Unlock()
	l, ok := h.m[host]
	if !ok {
		l = New(h.rps)
		h.m[host] = l
	}
	return l
}

// Backoff is the wait before retry n+1 after a network error.
func Backoff(n int) time.Duration { return time.Duration(n+1) * 500 * time.Millisecond }

// RetryAfter parses a Retry-After header (seconds or an HTTP date),
// falling back to exponential backoff, capped at MaxRetryWait.
func RetryAfter(h string, n int) time.Duration {
	// Clamp n before shifting: from 34 on, 1s << n overflows. 2^6 s is
	// already past MaxRetryWait.
	d := time.Second << min(max(n, 0), 6)
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		// Clamp before converting: a huge value would wrap negative and
		// turn into the shortest wait instead of the longest.
		d = time.Duration(min(secs, int(MaxRetryWait/time.Second))) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		d = time.Until(t)
	}
	return min(max(d, 100*time.Millisecond), MaxRetryWait)
}

// Sleep waits for d or until ctx is done.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ParseTarget accepts a URL or a bare hostname. explicitScheme is false
// when the scheme was guessed (https), so callers can fall back to http.
//
// Deprecated: use httpx.ParseTarget, which behaves the same. It is a copy
// rather than a wrapper because httpx imports this package.
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
