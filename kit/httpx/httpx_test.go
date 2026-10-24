// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package httpx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/ratelimit"
)

// serve starts h on the first free port in 127.0.0.1:20000-20019, the
// range these tests may use.
func serve(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	for p := 20000; p < 20020; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		srv := httptest.NewUnstartedServer(h)
		_ = srv.Listener.Close()
		srv.Listener = l
		srv.Start()
		t.Cleanup(srv.Close)
		return srv
	}
	t.Skip("no free port in 127.0.0.1:20000-20019")
	return nil
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func get(t *testing.T, c *http.Client, raw string, hdr ...string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	return c.Do(req)
}

// getErr sends a GET that is expected to fail and returns its error,
// closing the body if it unexpectedly succeeds.
func getErr(t *testing.T, c *http.Client, raw string) error {
	t.Helper()
	resp, err := get(t, c, raw)
	if err == nil {
		_ = resp.Body.Close()
	}
	return err
}

// recorder is a server that keeps the headers of every request.
type recorder struct {
	*httptest.Server
	mu   sync.Mutex
	seen []http.Header
}

func newRecorder(t *testing.T, h http.HandlerFunc) *recorder {
	rec := &recorder{}
	rec.Server = serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.seen = append(rec.seen, r.Header.Clone())
		rec.mu.Unlock()
		if h != nil {
			h(w, r)
		}
	}))
	return rec
}

func (r *recorder) headers() []http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]http.Header(nil), r.seen...)
}

func TestHeadersScopedToTarget(t *testing.T) {
	other := newRecorder(t, nil)
	target := newRecorder(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hop" {
			http.Redirect(w, r, other.URL+"/landed", http.StatusFound)
		}
	})
	c := New(Options{
		Target:           mustURL(t, target.URL),
		Headers:          []string{"Authorization: Bearer user-token", "X-Custom: from-flag", "Cookie: session=abc"},
		OffSiteRedirects: true, // so the hop to another port is followed at all
	})
	// The caller also sets the headers itself; Go's client copies them
	// onto the redirect hop, Authorization included, because only the
	// port changes.
	resp, err := get(t, c, target.URL+"/hop", "Authorization", "Bearer caller", "X-Custom", "caller", "Cookie", "c=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	th := target.headers()
	if len(th) != 1 || th[0].Get("Authorization") != "Bearer user-token" || th[0].Get("X-Custom") != "from-flag" || th[0].Get("Cookie") != "session=abc" {
		t.Errorf("target got %v, want the user's headers", th)
	}
	oh := other.headers()
	if len(oh) != 1 {
		t.Fatalf("other host got %d requests, want 1", len(oh))
	}
	for _, k := range []string{"Authorization", "X-Custom", "Cookie"} {
		if v := oh[0].Get(k); v != "" {
			t.Errorf("redirect hop to another port got %s: %q", k, v)
		}
	}

	// A direct request to the other host is stripped the same way.
	resp, err = get(t, c, other.URL+"/direct", "Authorization", "Bearer caller", "X-Other", "kept")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	last := other.headers()[1]
	if last.Get("Authorization") != "" || last.Get("X-Other") != "kept" {
		t.Errorf("direct request to another host: %v", last)
	}
}

func TestNoTargetSendsNoUserHeaders(t *testing.T) {
	srv := newRecorder(t, nil)
	c := New(Options{Headers: []string{"X-Custom: v"}, AllowPrivate: true})
	resp, err := get(t, c, srv.URL, "Authorization", "Bearer caller")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	h := srv.headers()[0]
	if h.Get("X-Custom") != "" || h.Get("Authorization") != "Bearer caller" {
		t.Errorf("without a target: %v", h)
	}
}

func TestHostHeader(t *testing.T) {
	var host string
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { host = r.Host }))
	c := New(Options{Target: mustURL(t, srv.URL), Headers: []string{"Host: example.test"}})
	resp, err := get(t, c, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if host != "example.test" {
		t.Errorf("Host = %q, want example.test", host)
	}
}

func TestOffSiteRedirectRefused(t *testing.T) {
	other := newRecorder(t, nil)
	target := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/", http.StatusFound)
	}))
	c := New(Options{Target: mustURL(t, target.URL)})
	err := getErr(t, c, target.URL+"/")
	if !errors.Is(err, ErrOffSite) {
		t.Errorf("err = %v, want ErrOffSite", err)
	}
	if n := len(other.headers()); n != 0 {
		t.Errorf("other site got %d requests", n)
	}

	// Without a target the first request's site anchors the rule.
	err = getErr(t, New(Options{AllowPrivate: true}), target.URL+"/")
	if !errors.Is(err, ErrOffSite) {
		t.Errorf("no target: err = %v, want ErrOffSite", err)
	}
}

func TestRedirectLimit(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
	}))
	err := getErr(t, New(Options{Target: mustURL(t, srv.URL)}), srv.URL+"/")
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Errorf("err = %v", err)
	}
}

// fakeClock records the waits a transport asks for instead of sleeping.
type fakeClock struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (f *fakeClock) sleep(_ context.Context, d time.Duration) error {
	f.mu.Lock()
	f.waits = append(f.waits, d)
	f.mu.Unlock()
	return nil
}

func withClock(c *http.Client) *fakeClock {
	f := &fakeClock{}
	c.Transport.(*transport).sleep = f.sleep
	return f
}

func TestRetryAfterHonored(t *testing.T) {
	var hits int
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch hits {
		case 1:
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.Header().Set("Retry-After", "3600") // capped at MaxRetryWait
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			fmt.Fprint(w, "ok")
		}
	}))
	c := New(Options{Target: mustURL(t, srv.URL), Retries: 2})
	clock := withClock(c)
	resp, err := get(t, c, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "ok" || hits != 3 {
		t.Errorf("status %d, body %q after %d requests", resp.StatusCode, body, hits)
	}
	if want := []time.Duration{7 * time.Second, 30 * time.Second}; fmt.Sprint(clock.waits) != fmt.Sprint(want) {
		t.Errorf("waits = %v, want %v", clock.waits, want)
	}
}

func TestNoRetryForPostOrWhenOff(t *testing.T) {
	var hits int
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	c := New(Options{Target: mustURL(t, srv.URL), Retries: 3})
	clock := withClock(c)
	resp, err := c.Post(srv.URL, "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if hits != 1 || len(clock.waits) != 0 || resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("POST: %d requests, waits %v, status %d", hits, clock.waits, resp.StatusCode)
	}
}

func TestNetworkErrorRetried(t *testing.T) {
	var calls int
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection reset by peer")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}, Request: r}, nil
	})
	c := New(Options{Retries: 1, Transport: rt})
	clock := withClock(c)
	resp, err := get(t, c, "http://site.test/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if calls != 2 || len(clock.waits) != 1 {
		t.Errorf("calls %d, waits %v", calls, clock.waits)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestThrottleSlowsHost(t *testing.T) {
	var mu sync.Mutex
	var times []time.Time
	throttle := true
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		times = append(times, time.Now())
		if throttle {
			throttle = false
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	// Unlimited and no retries: the 429 is returned but the host's
	// limiter now spaces requests (250 ms after a first slow-down).
	c := New(Options{Target: mustURL(t, srv.URL)})
	for range 3 {
		resp, err := get(t, c, srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if gap := times[2].Sub(times[1]); gap < 200*time.Millisecond {
		t.Errorf("requests after a 429 were %s apart, want the host slowed down", gap)
	}
}

func TestRatePerHost(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := New(Options{Target: mustURL(t, srv.URL), Rate: 10})
	began := time.Now()
	for range 3 {
		resp, err := get(t, c, srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if d := time.Since(began); d < 190*time.Millisecond {
		t.Errorf("three requests at 10/s took %s", d)
	}
}

func TestMaxBody(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("a", 100))
	}))
	for _, c := range []struct {
		max  int64
		want int
		err  bool
	}{{10, 10, true}, {100, 100, false}, {101, 100, false}, {-1, 100, false}} {
		resp, err := get(t, New(Options{Target: mustURL(t, srv.URL), MaxBody: c.max}), srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if len(body) != c.want || errors.Is(err, ErrBodyTooLarge) != c.err {
			t.Errorf("MaxBody %d: read %d bytes, err %v", c.max, len(body), err)
		}
	}
}

func TestDebugLogMasksSecrets(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "sid=server-secret-value; Path=/")
		w.WriteHeader(http.StatusTeapot)
	}))
	var log bytes.Buffer
	c := New(Options{Target: mustURL(t, srv.URL), Debug: &log, UserAgent: "rhythm-test/1",
		Headers: []string{"Authorization: Bearer fake-token-1234567890", "X-Api-Key: fake-key-abcdef"}})
	u := strings.Replace(srv.URL, "http://", "http://admin:fake-pass-xyz@", 1) + "/p?token=fake-query-token"
	resp, err := get(t, c, u, "Cookie", "wordpress_logged_in=fake-cookie-secret")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	out := log.String()
	for _, secret := range []string{"fake-token-1234567890", "fake-key-abcdef", "fake-pass-xyz", "fake-query-token", "fake-cookie-secret", "server-secret-value"} {
		if strings.Contains(out, secret) {
			t.Errorf("debug log contains %q:\n%s", secret, out)
		}
	}
	for _, want := range []string{"> GET http://", ">   Authorization: •••••", ">   User-Agent: rhythm-test/1", "< 418 I'm a teapot GET", "<   Set-Cookie: sid=•••••"} {
		if !strings.Contains(out, want) {
			t.Errorf("debug log lacks %q:\n%s", want, out)
		}
	}
}

func TestDebugLogsErrors(t *testing.T) {
	var log bytes.Buffer
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("boom\x1b[2J") })
	_ = getErr(t, New(Options{Debug: &log, Transport: rt}), "http://site.test/")
	if !strings.Contains(log.String(), `< error GET http://site.test/: boom\x1b[2J`) {
		t.Errorf("log = %q", log.String())
	}
}

func TestPrivateRefused(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	// A public target may not reach a loopback server, not even by a
	// direct request.
	public := mustURL(t, "http://93.184.216.34/")
	err := getErr(t, New(Options{Target: public, OffSiteRedirects: true}), srv.URL)
	if !errors.Is(err, ErrPrivate) {
		t.Errorf("public target: err = %v, want ErrPrivate", err)
	}
	err = getErr(t, New(Options{}), srv.URL)
	if !errors.Is(err, ErrPrivate) {
		t.Errorf("no target: err = %v, want ErrPrivate", err)
	}
	for _, opts := range []Options{{Target: public, AllowPrivate: true}, {Target: mustURL(t, srv.URL)}} {
		resp, err := get(t, New(opts), srv.URL)
		if err != nil {
			t.Errorf("AllowPrivate %v, target %s: %v", opts.AllowPrivate, opts.Target, err)
			continue
		}
		_ = resp.Body.Close()
	}
}

func TestAllowAddr(t *testing.T) {
	public := []netip.Addr{netip.MustParseAddr("93.184.216.34")}
	loopback := []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}
	office := []netip.Addr{netip.MustParseAddr("192.168.1.20")}
	for _, c := range []struct {
		ip                       string
		public, loopback, office bool
	}{
		{"127.0.0.1", false, true, false},
		{"::1", false, true, false},
		{"0.0.0.0", false, true, false},
		{"169.254.169.254", false, false, false},
		{"fe80::1%eth0", false, false, false},
		{"10.0.0.1", false, false, true},
		{"fd00::1", false, false, true},
		{"::ffff:10.0.0.1", false, false, true},
		{"100.100.100.200", false, false, true},
		{"100.64.0.1", false, false, true},
		{"100.128.0.1", true, true, true},
		{"64:ff9b::a9fe:a9fe", false, false, false},
		{"64:ff9b::7f00:1", false, true, false},
		{"64:ff9b::808:808", true, true, true},
		{"8.8.8.8", true, true, true},
		{"2001:4860:4860::8888", true, true, true},
	} {
		ip := netip.MustParseAddr(c.ip)
		if got := AllowAddr(public, ip); got != c.public {
			t.Errorf("public target: AllowAddr(%s) = %v", c.ip, got)
		}
		if got := AllowAddr(loopback, ip); got != c.loopback {
			t.Errorf("loopback target: AllowAddr(%s) = %v", c.ip, got)
		}
		if got := AllowAddr(office, ip); got != c.office {
			t.Errorf("private target: AllowAddr(%s) = %v", c.ip, got)
		}
		if got := AllowAddr(nil, ip); got != c.public {
			t.Errorf("unresolved target: AllowAddr(%s) = %v", c.ip, got)
		}
	}
}

func TestGuardProxy(t *testing.T) {
	srv := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := strings.TrimPrefix(srv.URL, "http://")
	g := NewGuard("93.184.216.34")
	if _, err := g.DialContext(context.Background(), "tcp", addr); !errors.Is(err, ErrPrivate) {
		t.Fatalf("err = %v, want ErrPrivate", err)
	}
	// A proxy the user configured is dialed as is.
	g.proxies.Store(addr, true)
	conn, err := g.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}
	_ = conn.Close()
	var nilGuard *Guard
	if _, err := nilGuard.DialContext(context.Background(), "tcp", addr); !errors.Is(err, ErrPrivate) {
		t.Errorf("nil guard: err = %v, want ErrPrivate", err)
	}
	// Requests to loopback are never proxied, so nothing is recorded.
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	if p, err := g.Proxy(req); p != nil || err != nil {
		t.Errorf("Proxy = %v, %v", p, err)
	}
	for in, want := range map[string]string{"http://proxy.local": "proxy.local:80", "socks5://[::1]": "[::1]:1080", "https://p:3128": "p:3128"} {
		if got := canonicalAddr(mustURL(t, in)); got != want {
			t.Errorf("canonicalAddr(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestParseHeaders(t *testing.T) {
	h, err := ParseHeaders([]string{"x-one: 1", "X-One:2", "Empty:", " Spaced : v "})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(h["X-One"]) != "[1 2]" || h.Get("Spaced") != "v" || len(h["Empty"]) != 1 {
		t.Errorf("headers = %v", h)
	}
	for _, bad := range []string{"no colon", ": no name", "bad name: v", "x\x7f: v"} {
		if _, err := ParseHeaders([]string{bad}); err == nil {
			t.Errorf("ParseHeaders(%q) = nil error", bad)
		}
	}
}

func TestSameHost(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"https://Example.com", "https://example.com:443/x", true},
		{"https://example.com", "https://example.com:8443", false},
		{"https://example.com", "http://example.com", false},
		{"https://example.com", "https://www.example.com", false},
		{"http://127.0.0.1:20001", "http://127.0.0.1:20002", false},
	} {
		if got := SameHost(mustURL(t, c.a), mustURL(t, c.b)); got != c.want {
			t.Errorf("SameHost(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

func TestSameSite(t *testing.T) {
	for _, c := range []struct {
		start, u string
		want     bool
	}{
		{"https://example.com/", "https://www.example.com/a", true},
		{"https://example.com/", "https://EXAMPLE.com:443/a", true},
		{"https://example.com/", "https://example.com:8443/", false},
		{"http://example.com/", "https://example.com/", true},
		{"https://example.com/", "http://example.com/", false},
		{"https://example.com/", "https://evil.com/", false},
		{"https://example.com/", "https://sub.example.com/", false},
	} {
		if got := SameSite(mustURL(t, c.start), mustURL(t, c.u)); got != c.want {
			t.Errorf("SameSite(%s, %s) = %v", c.start, c.u, got)
		}
	}
}

func TestParseTarget(t *testing.T) {
	u, explicit, err := ParseTarget("example.com")
	if err != nil || u.String() != "https://example.com" || explicit {
		t.Errorf("bare host: %v %v %v", u, explicit, err)
	}
	u, explicit, err = ParseTarget("http://example.com/x")
	if err != nil || u.Scheme != "http" || !explicit {
		t.Errorf("http URL: %v %v %v", u, explicit, err)
	}
	for _, bad := range []string{"ftp://example.com", "http://", "://x"} {
		if _, _, err := ParseTarget(bad); err == nil {
			t.Errorf("ParseTarget(%q) = nil error", bad)
		}
	}
	// The deprecated copy in ratelimit must keep behaving the same.
	for _, in := range []string{"example.com/blog", "http://example.com", "ftp://x", "http://", "Example.COM:8080"} {
		u1, e1, err1 := ParseTarget(in)
		u2, e2, err2 := ratelimit.ParseTarget(in) //nolint:staticcheck // comparing against the deprecated copy on purpose
		if fmt.Sprint(u1, e1, err1) != fmt.Sprint(u2, e2, err2) {
			t.Errorf("ParseTarget(%q): httpx %v %v %v, ratelimit %v %v %v", in, u1, e1, err1, u2, e2, err2)
		}
	}
}

func TestDefaults(t *testing.T) {
	c := New(Options{})
	if c.Timeout != DefaultTimeout || c.Transport.(*transport).maxBody != DefaultMaxBody {
		t.Errorf("timeout %s, max body %d", c.Timeout, c.Transport.(*transport).maxBody)
	}
	if New(Options{Timeout: -1}).Timeout != 0 || New(Options{Timeout: time.Second}).Timeout != time.Second {
		t.Error("Timeout not applied")
	}
}

func TestUserAgentKeepsCallers(t *testing.T) {
	srv := newRecorder(t, nil)
	c := New(Options{Target: mustURL(t, srv.URL), UserAgent: "rhythm/1"})
	for _, ua := range []string{"", "caller/2"} {
		resp, err := get(t, c, srv.URL, "User-Agent", ua)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	h := srv.headers()
	if h[0].Get("User-Agent") != "rhythm/1" || h[1].Get("User-Agent") != "caller/2" {
		t.Errorf("user agents %q, %q", h[0].Get("User-Agent"), h[1].Get("User-Agent"))
	}
}
