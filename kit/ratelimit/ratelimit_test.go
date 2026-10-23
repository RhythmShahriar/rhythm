package ratelimit

import (
	"context"
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
	if d := RetryAfter("", 2); d != 4*time.Second {
		t.Errorf("backoff: %v", d)
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
