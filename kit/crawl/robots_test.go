// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// robotsSite serves a three-page site whose robots.txt is answered by
// robotsTxt.
func robotsSite(t *testing.T, robotsTxt http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", robotsTxt)
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body>"+body+"</body></html>")
		}
	}
	mux.HandleFunc("/{$}", page(`<a href="/public/">p</a> <a href="/private/">x</a>`))
	mux.HandleFunc("/public/", page(`ok`))
	mux.HandleFunc("/private/", page(`ok`))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func crawlPaths(t *testing.T, srv *httptest.Server, opts Options) ([]string, *Frontier) {
	t.Helper()
	opts.NoSitemaps = true
	f, err := New(opts).Crawl(context.Background(), mustURL(t, srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range f.Pages() {
		paths = append(paths, strings.TrimPrefix(p.URL, srv.URL))
	}
	return paths, f
}

func TestRobotsUnavailable(t *testing.T) {
	status := func(code int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
	}
	reset := func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetLinger(0) // close with RST, not FIN
		}
		_ = conn.Close()
	}
	for _, tc := range []struct {
		name    string
		robots  http.HandlerFunc
		ignore  bool
		pages   int
		blocked string // RobotsUnavailable, "" when readable
	}{
		{"503 disallows all but the start page", status(503), false, 1, "HTTP 503"},
		{"429 disallows all but the start page", status(429), false, 1, "HTTP 429"},
		{"connection reset disallows all but the start page", reset, false, 1, "*"},
		{"404 allows everything", status(404), false, 3, ""},
		{"403 allows everything", status(403), false, 3, ""},
		{"IgnoreRobots with a 503 crawls everything", status(503), true, 3, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths, f := crawlPaths(t, robotsSite(t, tc.robots), Options{IgnoreRobots: tc.ignore})
			if len(paths) != tc.pages || paths[0] != "/" {
				t.Errorf("pages = %v, want %d starting with /", paths, tc.pages)
			}
			got := f.RobotsUnavailable()
			if tc.blocked == "*" && got == "" || tc.blocked != "*" && got != tc.blocked {
				t.Errorf("RobotsUnavailable = %q, want %q", got, tc.blocked)
			}
			if n := f.Skipped()["robots-unavailable"]; (tc.pages == 1) != (n == 2) {
				t.Errorf("robots-unavailable skipped = %d", n)
			}
		})
	}
}

func TestRobotsListedPagesWhenUnavailable(t *testing.T) {
	f := NewFrontier(mustURL(t, "https://x.example/"), Options{})
	f.robots = unavailableRobots("HTTP 503")
	if !f.Add("https://x.example/", "start", 0) || !f.Add("https://x.example/listed/", "config", 0) {
		t.Error("the start page and listed pages are still admitted")
	}
	if f.Add("https://x.example/found/", "link", 1) {
		t.Error("found links are refused while robots.txt is unavailable")
	}
}

func TestParseRobotsLongLinesBOMAndUnicode(t *testing.T) {
	long := "# " + strings.Repeat("x", 70<<10) + "\nUser-agent: *\nDisallow: /private\n"
	if r := parseRobots(strings.NewReader(long), "a"); r.unavailable != "" || r.Allowed("/private/x") {
		t.Errorf("a 70 KB comment line must not drop later rules: %+v", r.disallow)
	}
	if r := parseRobots(strings.NewReader("\uFEFFUser-agent: *\nDisallow: /private\n"), "a"); r.Allowed("/private") {
		t.Error("a BOM-prefixed file parses")
	}
	r := parseRobots(strings.NewReader("User-agent: *\nDisallow: /café\nDisallow: /a%2fb\nDisallow: /*.php$\n"), "a")
	for p, want := range map[string]bool{"/caf%C3%A9": false, "/caf%C3%A9/menu": false, "/cafe": true,
		"/a%2Fb": false, "/x.php": false, "/x.php?y": true} {
		if got := r.Allowed(p); got != want {
			t.Errorf("Allowed(%s) = %v, want %v (rules %v)", p, got, want, r.disallow)
		}
	}
	// A line past the buffer can't be read whole, so the file counts as
	// unavailable instead of being silently cut short.
	huge := "User-agent: *\n# " + strings.Repeat("x", maxRobotsLine+10) + "\nDisallow: /private\n"
	if r := parseRobots(strings.NewReader(huge), "a"); r.unavailable == "" {
		t.Error("a line longer than the buffer marks robots.txt unavailable")
	}
}

func TestRobotsLongLineOverHTTP(t *testing.T) {
	srv := robotsSite(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "# "+strings.Repeat("x", 70<<10)+"\nUser-agent: *\nDisallow: /private\n")
	})
	paths, _ := crawlPaths(t, srv, Options{})
	if len(paths) != 2 || paths[1] != "/public/" {
		t.Errorf("pages = %v, want / and /public/", paths)
	}
}

// A robots.txt that redirects to another site (a CDN, another port) is
// not followed. The crawl goes on as if it were missing instead of
// stopping at the start page. A redirect loop is treated the same way.
func TestRobotsRedirectRefusedAllowsAll(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the off-site robots.txt was fetched: %s", r.URL)
	}))
	t.Cleanup(other.Close)
	for _, tc := range []struct {
		name   string
		robots http.HandlerFunc
	}{
		{"off-site redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/robots.txt", http.StatusMovedPermanently)
		}},
		{"redirect loop", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/robots.txt", http.StatusFound)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths, f := crawlPaths(t, robotsSite(t, tc.robots), Options{})
			if len(paths) != 3 {
				t.Errorf("pages = %v, want all 3", paths)
			}
			if got := f.RobotsUnavailable(); got != "" {
				t.Errorf("RobotsUnavailable = %q, want empty", got)
			}
			if n := f.Skipped()["robots-unavailable"]; n != 0 {
				t.Errorf("robots-unavailable skipped = %d", n)
			}
		})
	}
}
