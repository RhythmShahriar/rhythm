// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func mustURL(t testing.TB, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestNormalize(t *testing.T) {
	base := mustURL(t, "https://Example.com/blog/post/")
	cases := map[string]string{
		"../about#team":                        "https://example.com/blog/about",
		"/shop/?utm_source=x&b=2&a=1&fbclid=y": "https://example.com/shop/?a=1&b=2",
		"HTTPS://EXAMPLE.COM:443/":             "https://example.com/",
		"//example.com/x/./y/../z":             "https://example.com/x/z",
		"?page=2":                              "https://example.com/blog/post/?page=2",
	}
	for in, want := range cases {
		if got, ok := Normalize(base, in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	// URLs whose meaning Go's query and path handling would change.
	for in, want := range map[string]string{
		"/p?id=1;x=2":                  "https://example.com/p?id=1;x=2",
		"/p?b=2;x&utm_source=nl&a=1":   "https://example.com/p?b=2;x&a=1",
		"/p?utm_source=x;id=1":         "https://example.com/p?id=1",
		"/p?id=1;utm_source=x&a=1":     "https://example.com/p?id=1&a=1",
		"/q?x=%zz&utm_medium=email":    "https://example.com/q?x=%zz",
		"/files/a%2Fb":                 "https://example.com/files/a%2Fb",
		"/files/./a%2Fb/../c%2Fd/":     "https://example.com/files/c%2Fd/",
		"/amp?amp":                     "https://example.com/amp?amp",
		"/amp?b&a=&utm_campaign=x":     "https://example.com/amp?a=&b",
		"/s?q=caf%C3%A9+au+lait&a=1":   "https://example.com/s?a=1&q=caf%C3%A9+au+lait",
		"/s?q=a%20b":                   "https://example.com/s?q=a+b",
		"/x?utm_source=a&UTM_Medium=b": "https://example.com/x",
		"/x?":                          "https://example.com/x",
	} {
		if got, ok := Normalize(base, in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if key("https://example.com/files/a%2Fb/") == key("https://example.com/files/a/b/") {
		t.Error("an escaped slash makes another page")
	}
	if key("https://example.com/files/a%2Fb/") != key("https://example.com/files/a%2Fb") {
		t.Error("the trailing slash is ignored for de-duplication")
	}
	for _, bad := range []string{"", "#top", "mailto:a@b.c", "javascript:void(0)", "tel:123"} {
		if _, ok := Normalize(base, bad); ok {
			t.Errorf("Normalize(%q) should be rejected", bad)
		}
	}
}

func TestExcludedAndGlob(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://x.example/wp-admin/":                true,
		"https://x.example/wp-login.php?action=lost": true,
		"https://x.example/blog/feed/":               true,
		"https://x.example/?s=shoes":                 true,
		"https://x.example/blog/?pages=2":            false, // "s=" inside another name
		"https://x.example/shop/?add-to-cart=12":     true,
		"https://x.example/post/?replytocom=5":       true,
		"https://x.example/my-account/":              false,
		"https://x.example/about/":                   false,
	} {
		if got := Excluded(mustURL(t, raw), DefaultExcludes); got != want {
			t.Errorf("Excluded(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestDefaultExcludesSideEffects(t *testing.T) {
	for _, p := range []string{
		"/WP-ADMIN/", "/Wp-Login.php", "/Wp-Login.php?action=logout",
		"/logout", "/logout/", "/account/logout.php", "/LogOut?next=/", "/sign-out/now",
		"/newsletter/unsubscribe/?id=4", "/?mailpoet_page=Unsubscribe", "/opt-out/", "/optout?u=1",
		"/confirm/abc", "/subscribe/confirm", "/post/?action=delete&id=3", "/?action=trash&post=9",
		"/?rest_route=/wp/v2/users", "/verify/?token=abc", "/?a=1&_wpnonce=f00", "/?CSRF_TOKEN=1",
	} {
		if !Excluded(mustURL(t, "https://x.example"+p), DefaultExcludes) {
			t.Errorf("%s should be excluded", p)
		}
	}
	for _, p := range []string{
		"/blog/logout-tips-for-teams/", "/confirmation-of-order-policy", "/tokens-of-appreciation/",
		"/restaurant", "/blog/signout-checklist/", "/rest/route/", "/?page=2",
	} {
		if Excluded(mustURL(t, "https://x.example"+p), DefaultExcludes) {
			t.Errorf("%s should not be excluded", p)
		}
	}
	// The caller's own patterns ignore case too.
	if !Excluded(mustURL(t, "https://x.example/Shop/Hats/"), []string{"/shop/*"}) ||
		!Excluded(mustURL(t, "https://x.example/shop/hats/"), []string{"/SHOP/*"}) {
		t.Error("Include and Exclude patterns are case-insensitive")
	}
}

func TestRobots(t *testing.T) {
	r := parseRobots(strings.NewReader(`
User-agent: *
Disallow: /private/
Allow: /private/ok
Disallow: /*.pdf$
Sitemap: https://x.example/custom-sitemap.xml

User-agent: rhythm-soundcheck
Disallow: /no-bots/
`), "rhythm-soundcheck/1.0 (+https://rythm.sh)")
	if !r.Allowed("/private/") || r.Allowed("/no-bots/page") {
		t.Error("our own group should apply, not *")
	}
	r = parseRobots(strings.NewReader("User-agent: *\nDisallow: /private/\nAllow: /private/ok\nDisallow: /*.pdf$\n"), "other")
	for p, want := range map[string]bool{"/private/x": false, "/private/ok": true, "/a.pdf": false, "/a.pdf?x": true, "/public": true} {
		if got := r.Allowed(p); got != want {
			t.Errorf("Allowed(%s) = %v, want %v", p, got, want)
		}
	}
	if got := parseRobots(strings.NewReader("Sitemap: https://x.example/s.xml"), "a").sitemaps; len(got) != 1 {
		t.Errorf("sitemaps = %v", got)
	}
}

func TestGrouping(t *testing.T) {
	g := newGrouper(3)
	keys := map[string]string{}
	for _, p := range []string{"/about/", "/contact/", "/product/a/", "/product/b/", "/product/c/", "/product/d/", "/product/e/", "/2026/10/hello/", "/blog/page/7/", "/item/12345"} {
		keys[p] = g.Group(mustURL(t, "https://x.example"+p))
	}
	if keys["/about/"] == keys["/contact/"] {
		t.Error("top-level pages stay separate")
	}
	if keys["/product/d/"] != "/product/{slug}" || keys["/product/e/"] != "/product/{slug}" {
		t.Errorf("product fan-out: %v", keys)
	}
	if keys["/product/a/"] != "/product/a" {
		t.Errorf("first children keep their own group: %q", keys["/product/a/"])
	}
	if keys["/2026/10/hello/"] != "/{y}/{m}/hello" || keys["/blog/page/7/"] != "/blog/page/{n}" || keys["/item/12345"] != "/item/{n}" {
		t.Errorf("patterns: %v", keys)
	}
}

// fakeSite serves a small WordPress-like site.
func fakeSite(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<html><body>"+body+"</body></html>")
		}
	}
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nDisallow: /secret/\nSitemap: %s/sitemap_index.xml\n", srv.URL)
	})
	mux.HandleFunc("/sitemap_index.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><sitemapindex><sitemap><loc>%s/posts.xml.gz</loc></sitemap></sitemapindex>`, srv.URL)
	})
	mux.HandleFunc("/posts.xml.gz", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		fmt.Fprintf(zw, `<urlset><url><loc>%s/from-sitemap/</loc></url><url><loc>%s/secret/x/</loc></url></urlset>`, srv.URL, srv.URL)
		zw.Close()
		w.Write(buf.Bytes())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		links := `<a href="/about/">About</a> <a href="/shop/">Shop</a> <a href="/about/#team">dup</a>
			<a href="https://other.example/">off-site</a> <a href="/wp-admin/">admin</a> <a href="/brochure.pdf">pdf</a>
			<a href="/blog/?utm_source=nav">Blog</a> <a href="/?s=x">search</a> <a href="mailto:x@y.z">mail</a>`
		page(links)(w, r)
	})
	mux.HandleFunc("/about/", page(`<a href="/contact/">Contact</a>`))
	mux.HandleFunc("/contact/", page(`ok`))
	mux.HandleFunc("/blog/", page(`<link rel="next" href="/blog/page/2/">`))
	mux.HandleFunc("/blog/page/2/", page(`page 2`))
	mux.HandleFunc("/from-sitemap/", page(`<a href="/deep/">deep</a>`))
	mux.HandleFunc("/deep/", page(`deep`))
	mux.HandleFunc("/shop/", func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		for i := range 40 {
			fmt.Fprintf(&b, `<a href="/product/item-%d/">p</a>`, i)
		}
		page(b.String())(w, r)
	})
	mux.HandleFunc("/product/", page(`product`))
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCrawl(t *testing.T) {
	srv := fakeSite(t)
	start := mustURL(t, srv.URL+"/")
	f, err := New(Options{UserAgent: "test", PerGroup: 2, FanOut: 5}).Crawl(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range f.Pages() {
		paths = append(paths, strings.TrimPrefix(p.URL, srv.URL))
	}
	for _, want := range []string{"/", "/about/", "/contact/", "/shop/", "/blog/", "/blog/page/2/", "/from-sitemap/", "/deep/"} {
		if !slices.Contains(paths, want) {
			t.Errorf("missing %s in %v", want, paths)
		}
	}
	for _, bad := range []string{"/secret/x/", "/wp-admin/", "/brochure.pdf", "/?s=x"} {
		if slices.Contains(paths, bad) {
			t.Errorf("should skip %s", bad)
		}
	}
	products := 0
	for _, p := range paths {
		if strings.HasPrefix(p, "/product/") {
			products++
		}
	}
	// 5 distinct children before fanning out, then 2 samples of the template.
	if products != 7 {
		t.Errorf("products sampled = %d, want 7 (of 40); paths %v", products, paths)
	}
	sk := f.Skipped()
	if sk["grouped"] == 0 || sk["robots.txt"] == 0 || sk["excluded"] == 0 || sk["other site"] == 0 {
		t.Errorf("skipped = %v", sk)
	}
	if f.Pages()[0].Source != "start" {
		t.Error("the start page comes first")
	}

	// IgnoreRobots lets your own blocked pages in; Limit caps the total.
	f, _ = New(Options{IgnoreRobots: true, PerGroup: -1, Limit: 3}).Crawl(context.Background(), start)
	if n := len(f.Pages()); n != 3 {
		t.Errorf("limit: %d pages", n)
	}
	f, _ = New(Options{IgnoreRobots: true}).Crawl(context.Background(), start)
	found := false
	for _, p := range f.Pages() {
		found = found || strings.HasSuffix(p.URL, "/secret/x/")
	}
	if !found {
		t.Error("--ignore-robots should include /secret/x/ from the sitemap")
	}
}

func TestExtractLinksBase(t *testing.T) {
	links, err := ExtractLinks(mustURL(t, "https://x.example/a/"), strings.NewReader(
		`<base href="https://x.example/b/"><a href="c">c</a><a href="/d" rel="nofollow">d</a><a href="e.zip" download>e</a><area href="f">`))
	if err != nil || !slices.Equal(links, []string{"https://x.example/b/c", "https://x.example/b/f"}) {
		t.Errorf("links = %v, %v", links, err)
	}
}

func TestCrawlNoFollow(t *testing.T) {
	srv := fakeSite(t)
	f, err := New(Options{UserAgent: "test", NoFollow: true}).Crawl(context.Background(), mustURL(t, srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range f.Pages() {
		paths = append(paths, strings.TrimPrefix(p.URL, srv.URL))
	}
	if !slices.Contains(paths, "/") || !slices.Contains(paths, "/from-sitemap/") {
		t.Errorf("want the start page and sitemap pages: %v", paths)
	}
	if slices.Contains(paths, "/deep/") {
		t.Errorf("NoFollow must not follow links: %v", paths)
	}
}

func TestGroupingPlainPermalinks(t *testing.T) {
	g := newGrouper(3)
	key := func(p string) string { return g.Group(mustURL(t, "https://x.example"+p)) }
	if a, b := key("/?page_id=2"), key("/?page_id=9"); a == b || a != "/?page_id=2" {
		t.Errorf("pages by ID stay separate: %q %q", a, b)
	}
	if a, b := key("/?p=1"), key("/?cat=1"); a == b || a == "/" {
		t.Errorf("a post and a category aren't the home page: %q %q", a, b)
	}
	for _, id := range []string{"10", "11", "12", "13", "14"} {
		key("/?product=" + id)
	}
	if k := key("/?product=99"); k != "/?product={slug}" {
		t.Errorf("many products group: %q", k)
	}
	if k := key("/?page_id=2&paged=3"); k != "/?page_id=2?{page}" {
		t.Errorf("pagination: %q", k)
	}
	if k := key("/?utm=x"); k != "/" {
		t.Errorf("other params don't make a page: %q", k)
	}
}

func TestStartAndListedPagesAlwaysAdmitted(t *testing.T) {
	start := mustURL(t, "https://x.example/")
	f := NewFrontier(start, Options{Include: []string{"/shop/*"}, PerGroup: 1, Limit: 10})
	f.robots = &robots{disallow: []string{"/private/"}}
	if !f.Add("https://x.example/", "start", 0) {
		t.Error("the start page is where links are found; include mustn't drop it")
	}
	if f.Add("https://x.example/blog/", "link", 1) {
		t.Error("include still applies to found links")
	}
	if !f.Add("https://x.example/private/checkout/", "config", 0) {
		t.Error("a page the user listed gets past include and robots.txt")
	}
}
