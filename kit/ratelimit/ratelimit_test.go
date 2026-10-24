// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package ratelimit

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestLimiterPaces(t *testing.T) {
	l := New(20) // one request every 50ms
	start := time.Now()
	for range 5 {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 190*time.Millisecond {
		t.Errorf("5 requests at 20/s took %v", d)
	}
	var nilLimiter *Limiter
	if err := nilLimiter.Wait(context.Background()); err != nil {
		t.Error("nil limiter must not wait")
	}
	nilLimiter.Slow()
}

func TestSlowCapsAtSlowest(t *testing.T) {
	l := New(1000)
	for range 20 {
		l.Slow()
	}
	if l.every != Slowest {
		t.Errorf("every = %v, want %v", l.every, Slowest)
	}
}

func TestSlowNeverSpeedsUp(t *testing.T) {
	l := New(0.25) // every 4 s, already slower than Slowest
	l.Slow()
	if l.every != 4*time.Second {
		t.Errorf("New(0.25) after Slow: every = %v, want 4s", l.every)
	}
	l = New(10)
	for _, want := range []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond, Slowest, Slowest} {
		l.Slow()
		if l.every != want {
			t.Errorf("New(10) after Slow: every = %v, want %v", l.every, want)
		}
	}
}

func TestUnlimitedStillSlows(t *testing.T) {
	for _, rps := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		l := New(rps)
		if l.every != 0 {
			t.Errorf("New(%v) should not wait: every = %v", rps, l.every)
		}
		start := time.Now()
		for range 3 {
			_ = l.Wait(context.Background())
		}
		if d := time.Since(start); d > 100*time.Millisecond {
			t.Errorf("New(%v) waited %v", rps, d)
		}
		l.Slow()
		if l.every != slowStart {
			t.Errorf("New(%v) after Slow: every = %v, want %v", rps, l.every, slowStart)
		}
		l.Slow()
		if l.every != 2*slowStart {
			t.Errorf("New(%v) after two Slows: every = %v", rps, l.every)
		}
	}
}

func TestTinyRatesClamp(t *testing.T) {
	for _, rps := range []float64{1e-12, 1e-300, MinRate / 2} {
		if l := New(rps); l.every != time.Hour {
			t.Errorf("New(%v): every = %v, want 1h", rps, l.every)
		}
	}
	if l := New(1); l.every != time.Second {
		t.Errorf("New(1): every = %v", l.every)
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []float64{0, MinRate, 0.5, 10, 1e6} {
		if err := Validate(ok); err != nil {
			t.Errorf("Validate(%v) = %v", ok, err)
		}
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 1e-12} {
		if err := Validate(bad); err == nil {
			t.Errorf("Validate(%v) should fail", bad)
		}
	}
}

func TestHostsAreIndependent(t *testing.T) {
	h := NewHosts(5)
	if h.For("A.example") != h.For("a.example") {
		t.Error("hosts are case-insensitive")
	}
	if h.For("a.example") == h.For("b.example") {
		t.Error("each host gets its own limiter")
	}
}

func TestRetryAfter(t *testing.T) {
	if d := RetryAfter("3", 0); d != 3*time.Second {
		t.Errorf("seconds: %v", d)
	}
	if d := RetryAfter("3600", 0); d != MaxRetryWait {
		t.Errorf("cap: %v", d)
	}
	for _, h := range []string{"9223372037", "9223372036854775807"} {
		if d := RetryAfter(h, 0); d != MaxRetryWait {
			t.Errorf("RetryAfter(%q) = %v, want %v", h, d, MaxRetryWait)
		}
	}
	if d := RetryAfter("", 2); d != 4*time.Second {
		t.Errorf("backoff: %v", d)
	}
	for _, n := range []int{5, 34, 63, 64, 1000} {
		if d := RetryAfter("", n); d != MaxRetryWait {
			t.Errorf("RetryAfter(\"\", %d) = %v, want %v", n, d, MaxRetryWait)
		}
	}
	if d := RetryAfter("", -3); d != time.Second {
		t.Errorf("negative n: %v", d)
	}
	if d := RetryAfter(time.Now().Add(10*time.Second).UTC().Format(http.TimeFormat), 0); d < 8*time.Second || d > 10*time.Second {
		t.Errorf("HTTP date: %v", d)
	}
}

func TestParseTarget(t *testing.T) {
	u, explicit, err := ParseTarget("yoursite.com/blog")
	if err != nil || explicit || u.String() != "https://yoursite.com/blog" {
		t.Errorf("bare host: %v %v %v", u, explicit, err)
	}
	if _, explicit, _ := ParseTarget("http://yoursite.com"); !explicit {
		t.Error("explicit scheme")
	}
	for _, bad := range []string{"ftp://x.example", "://", "http://"} {
		if _, _, err := ParseTarget(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}
