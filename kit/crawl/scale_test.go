// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// memSite is an in-memory site of n pages, /p/0/ to /p/{n-1}/, each
// linking to 20 others: its children in a 20-ary tree (so every page is
// reachable) and the rest back towards the root. No network is used.
type memSite struct{ n int }

func (m memSite) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Request: r,
		Body: io.NopCloser(strings.NewReader(""))}
	id, err := strconv.Atoi(strings.Trim(strings.TrimPrefix(r.URL.Path, "/p/"), "/"))
	if r.URL.Path == "/" {
		id, err = 0, nil
	}
	if err != nil || id < 0 || id >= m.n {
		return resp, nil
	}
	var b strings.Builder
	b.WriteString("<html><body>")
	for k := 1; k <= 20; k++ {
		child := id*20 + k
		if child >= m.n {
			child = (id + k) % m.n
		}
		fmt.Fprintf(&b, `<a href="/p/%d/">%d</a>`, child, child)
	}
	b.WriteString("</body></html>")
	resp.StatusCode = http.StatusOK
	resp.Header.Set("Content-Type", "text/html")
	resp.Body = io.NopCloser(strings.NewReader(b.String()))
	return resp, nil
}

// crawlMem crawls an in-memory site of n pages and returns the frontier.
func crawlMem(tb testing.TB, n int) *Frontier {
	tb.Helper()
	c := New(Options{Limit: n + 1, PerGroup: -1, NoSitemaps: true,
		HTTP: &http.Client{Transport: memSite{n}}})
	f, err := c.Crawl(context.Background(), mustURL(tb, "http://mem.example/"))
	if err != nil {
		tb.Fatal(err)
	}
	return f
}

func TestCrawlNoQuadraticCopy(t *testing.T) {
	f := crawlMem(t, 500)
	if n := f.pagesCalls.Load(); n != 0 {
		t.Errorf("Crawl called Pages %d times; the loop must not copy the page list", n)
	}
	if got := len(f.Pages()); got != 501 { // the start page plus /p/0/ to /p/499/
		t.Errorf("crawled %d pages, want 501", got)
	}
}

func TestCrawlScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	// Best of three runs each, to keep scheduler noise out of the ratio.
	best := func(n int) time.Duration {
		d := time.Duration(1 << 62)
		for range 3 {
			t0 := time.Now()
			crawlMem(t, n)
			d = min(d, time.Since(t0))
		}
		return d
	}
	small, large := best(2000), best(8000)
	// Four times the pages: linear is about 4x, the old quadratic copy 16x.
	if large > 6*small {
		t.Errorf("8,000 pages took %v, 2,000 took %v: more than 6x", large, small)
	}
}

func BenchmarkCrawl10k(b *testing.B) {
	for b.Loop() {
		crawlMem(b, 10_000)
	}
}

func TestAddPage(t *testing.T) {
	f := NewFrontier(mustURL(t, "https://x.example/"), Options{})
	p, ok := f.AddPage("/about/#team", "link", 2)
	if !ok || p.URL != "https://x.example/about/" || p.Source != "link" || p.Depth != 2 || p.Group == "" {
		t.Errorf("AddPage = %+v, %v", p, ok)
	}
	if _, ok := f.AddPage("/about", "link", 2); ok {
		t.Error("a page already seen is not accepted twice")
	}
}
