// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/httpx"
)

func TestSameSite(t *testing.T) {
	for _, c := range []struct {
		start, u string
		want     bool
	}{
		{"https://example.com/", "https://example.com/a", true},
		{"https://example.com/", "https://WWW.Example.com/a", true},
		{"https://www.example.com/", "https://example.com/a", true},
		{"https://example.com/", "https://example.com:443/a", true},
		{"https://example.com/", "https://example.com:8443/a", false},
		{"http://127.0.0.1:8080/", "http://127.0.0.1:9090/", false},
		{"http://127.0.0.1:8080/", "http://127.0.0.1:8080/x", true},
		{"https://example.com/", "http://example.com/", false},
		{"http://example.com/", "https://example.com/", true},
		{"http://example.com:80/", "https://example.com:443/", true},
		{"http://example.com:8080/", "https://example.com:8080/", false},
		{"http://example.com/", "https://example.com:8443/", false},
		{"https://example.com/", "https://other.example/", false},
		{"https://example.com/", "ftp://example.com/", false},
	} {
		if got := SameSite(mustURL(t, c.start), mustURL(t, c.u)); got != c.want {
			t.Errorf("SameSite(%s, %s) = %v, want %v", c.start, c.u, got, c.want)
		}
	}
}

// counter is a server that records every request it gets.
type counter struct {
	*httptest.Server
	hits  atomic.Int64
	auths atomic.Int64
}

func newCounter(t *testing.T) *counter {
	t.Helper()
	c := &counter{}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.hits.Add(1)
		if r.Header.Get("Authorization") != "" {
			c.auths.Add(1)
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<urlset><url><loc>/x</loc></url></urlset>`)
	}))
	t.Cleanup(c.Close)
	return c
}

// headerTransport adds a header on every round trip, the way a tool adds
// the user's -H headers, so it would carry credentials to any host the
// client is sent to.
type headerTransport struct{}

func (headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	return http.DefaultTransport.RoundTrip(r)
}

// hostileSite names server b (same host, another port) in robots.txt, in a
// sitemap index and in a redirect.
func hostileSite(t *testing.T, b *counter) *httptest.Server {
	t.Helper()
	var a *httptest.Server
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body>"+body+"</body></html>")
		}
	}
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nSitemap: %s/latest/meta-data/\nSitemap: %s/index.xml\n", b.URL, a.URL)
	})
	mux.HandleFunc("/index.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<sitemapindex><sitemap><loc>%s/victim.xml</loc></sitemap><sitemap><loc>/pages.xml</loc></sitemap></sitemapindex>`, b.URL)
	})
	mux.HandleFunc("/pages.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<urlset><url><loc>%s/from-sitemap/</loc></url></urlset>`, a.URL)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page(fmt.Sprintf(`<a href="/go-b/">b</a> <a href="/go-a/">a</a> <a href="%s/admin/delete-all">port b</a>`, b.URL))(w, r)
	})
	mux.HandleFunc("/go-b/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, b.URL+"/redirected/", http.StatusFound)
	})
	mux.HandleFunc("/go-a/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target/", http.StatusFound)
	})
	mux.HandleFunc("/target/", page(`<a href="/after-redirect/">next</a>`))
	mux.HandleFunc("/after-redirect/", page(`ok`))
	mux.HandleFunc("/from-sitemap/", page(`ok`))
	a = httptest.NewServer(mux)
	t.Cleanup(a.Close)
	return a
}

func TestCrawlStaysOnSite(t *testing.T) {
	for name, opts := range map[string]Options{
		"default client": {},
		"caller client":  {HTTP: &http.Client{Transport: headerTransport{}}},
	} {
		t.Run(name, func(t *testing.T) {
			b := newCounter(t)
			a := hostileSite(t, b)
			f, err := New(opts).Crawl(context.Background(), mustURL(t, a.URL+"/"))
			if err != nil {
				t.Fatal(err)
			}
			if n := b.hits.Load(); n != 0 {
				t.Errorf("port b got %d requests (%d with Authorization)", n, b.auths.Load())
			}
			var paths []string
			for _, p := range f.Pages() {
				paths = append(paths, strings.TrimPrefix(p.URL, a.URL))
			}
			for _, want := range []string{"/from-sitemap/", "/after-redirect/"} {
				if !slices.Contains(paths, want) {
					t.Errorf("missing %s in %v", want, paths)
				}
			}
			sk := f.Skipped()
			if sk["offsite-sitemap"] != 2 || sk["offsite-redirect"] != 1 || sk["other site"] == 0 {
				t.Errorf("skipped = %v", sk)
			}
		})
	}
}

func TestSitemapBudget(t *testing.T) {
	var fetches atomic.Int64
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/robots.txt":
			fmt.Fprintf(w, "Sitemap: %s/sm/index.xml\n", srv.URL)
		case r.URL.Path == "/sm/index.xml":
			fetches.Add(1)
			var b strings.Builder
			b.WriteString("<sitemapindex>")
			for i := range 2000 {
				fmt.Fprintf(&b, "<sitemap><loc>%s/sm/%d.xml</loc></sitemap>", srv.URL, i)
			}
			b.WriteString("</sitemapindex>")
			fmt.Fprint(w, b.String())
		case strings.HasPrefix(r.URL.Path, "/sm/"):
			fetches.Add(1)
			fmt.Fprint(w, "<urlset></urlset>")
		default:
			fmt.Fprint(w, "<html></html>")
		}
	}))
	t.Cleanup(srv.Close)
	f, err := New(Options{NoFollow: true}).Crawl(context.Background(), mustURL(t, srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if n := fetches.Load(); n != maxSitemaps {
		t.Errorf("sitemap fetches = %d, want %d", n, maxSitemaps)
	}
	if sk := f.Skipped()["sitemap limit"]; sk != 2001-maxSitemaps {
		t.Errorf("sitemap limit skips = %d", sk)
	}

	fetches.Store(0)
	if _, err := New(Options{NoFollow: true, MaxSitemaps: 5}).Crawl(context.Background(), mustURL(t, srv.URL+"/")); err != nil {
		t.Fatal(err)
	}
	if n := fetches.Load(); n != 5 {
		t.Errorf("MaxSitemaps 5: %d fetches", n)
	}
}

func TestRedirectRules(t *testing.T) {
	var hops atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops.Add(1)
		http.Redirect(w, r, fmt.Sprintf("/loop/%d", hops.Load()), http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	ctx := context.WithValue(context.Background(), stateKey{}, &crawlState{start: mustURL(t, srv.URL), guard: httpx.NewGuard("127.0.0.1")})

	c := New(Options{})
	resp, err := c.get(ctx, srv.URL+"/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("an endless redirect loop should fail")
	}
	if n := hops.Load(); n != maxRedirects {
		t.Errorf("hops = %d, want %d", n, maxRedirects)
	}

	// The caller's client is copied, its policy runs first, and it is
	// never changed.
	called := 0
	stop := errors.New("caller stop")
	mine := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { called++; return stop }}
	before := reflect.ValueOf(mine.CheckRedirect).Pointer()
	c = New(Options{HTTP: mine})
	if c.http == mine || reflect.ValueOf(mine.CheckRedirect).Pointer() != before {
		t.Error("the caller's client must not be changed")
	}
	if resp, err := c.get(ctx, srv.URL+"/"); !errors.Is(err, stop) || called != 1 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Errorf("caller policy: err %v, called %d", err, called)
	}
	if New(Options{}).http.Timeout != defaultTimeout || New(Options{HTTP: &http.Client{}}).http.Timeout != 0 {
		t.Error("only the default client gets the default timeout")
	}
}

func TestDialRefusesPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	// A public start host: the loopback server is refused before any
	// connection is made.
	st := &crawlState{guard: httpx.NewGuard("93.184.216.34")}
	ctx := context.WithValue(context.Background(), stateKey{}, st)
	if _, err := dial(ctx, "tcp", addr); !errors.Is(err, httpx.ErrPrivate) {
		t.Errorf("public start: err = %v, want httpx.ErrPrivate", err)
	}
	if _, err := dial(context.Background(), "tcp", addr); !errors.Is(err, httpx.ErrPrivate) {
		t.Errorf("no crawl state: err = %v, want httpx.ErrPrivate", err)
	}

	// A loopback start host may reach loopback addresses.
	st = &crawlState{guard: httpx.NewGuard("127.0.0.1")}
	conn, err := dial(context.WithValue(context.Background(), stateKey{}, st), "tcp", addr)
	if err != nil {
		t.Fatalf("loopback start: %v", err)
	}
	_ = conn.Close()

	// Through the default client: refused for a public start, allowed
	// with AllowPrivate.
	ctx = context.WithValue(context.Background(), stateKey{}, &crawlState{start: mustURL(t, srv.URL), guard: httpx.NewGuard("93.184.216.34")})
	for _, allow := range []bool{false, true} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		resp, err := New(Options{AllowPrivate: allow}).http.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		if refused := errors.Is(err, httpx.ErrPrivate); refused == allow {
			t.Errorf("AllowPrivate %v: err = %v", allow, err)
		}
	}
}

func TestRecordProxy(t *testing.T) {
	// Requests to loopback are never proxied, so nothing is recorded, with
	// or without crawl state.
	for _, ctx := range []context.Context{context.Background(), context.WithValue(context.Background(), stateKey{}, &crawlState{guard: httpx.NewGuard("")})} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1/", nil)
		if p, err := recordProxy(req); p != nil || err != nil {
			t.Errorf("recordProxy = %v, %v", p, err)
		}
	}
}

func TestDefaultTimeout(t *testing.T) {
	defer func(d time.Duration) { defaultTimeout = d }(defaultTimeout)
	defaultTimeout = 200 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)
	began := time.Now()
	if _, err := New(Options{NoSitemaps: true}).Crawl(context.Background(), mustURL(t, srv.URL+"/")); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(began); d > 3*time.Second {
		t.Errorf("a stalled page took %s", d)
	}
}

func TestRedirectHopsWaitOnLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/c", http.StatusFound)
		}
	}))
	t.Cleanup(srv.Close)
	ctx := context.WithValue(context.Background(), stateKey{}, &crawlState{start: mustURL(t, srv.URL), guard: httpx.NewGuard("127.0.0.1")})
	began := time.Now()
	resp, err := New(Options{Rate: 10}).get(ctx, srv.URL+"/a")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	// Three requests at 10 per second: the two hops wait 100 ms each.
	if d := time.Since(began); d < 190*time.Millisecond {
		t.Errorf("three requests took %s; redirect hops skipped the limiter", d)
	}
}
