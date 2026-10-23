// Package ratelimit keeps Rhythm tools polite: an even request pace per
// host that slows down when a server asks it to, plus the retry helpers
// and target parsing every tool shares.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Slowest pace a limiter backs off to after repeated throttling.
const Slowest = 2 * time.Second

// Longest Retry-After a tool will honor.
const MaxRetryWait = 30 * time.Second

// Limiter spaces requests evenly to stay under a requests-per-second cap.
// A nil *Limiter never waits.
type Limiter struct {
	mu    sync.Mutex
	every time.Duration
	next  time.Time
}

// New returns a limiter for rps requests per second, or nil (no limit)
// when rps <= 0.
func New(rps float64) *Limiter {
	if rps <= 0 {
		return nil
	}
	return &Limiter{every: time.Duration(float64(time.Second) / rps)}
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
// Call it when a server answers 429 or 503.
func (l *Limiter) Slow() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.every = min(l.every*2, Slowest)
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
	d := time.Second << n
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		d = time.Duration(secs) * time.Second
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
