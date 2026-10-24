// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package crawl discovers the pages of a website the way a careful
// visitor would: from its sitemaps and by following its own links. It
// stays on the site (the same host, scheme and port, see SameSite) for
// pages, sitemaps and redirects alike, honors robots.txt, skips URLs with
// side effects (logout, add to cart), de-duplicates and groups pages that
// share a URL template so a large catalog is checked through a few
// samples. Its default client times out and won't connect to private
// addresses unless the site itself is on one.
package crawl

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/RhythmShahriar/rhythm/kit/ratelimit"
)

// Limits on what a crawl reads.
const (
	maxBody         = 5 << 20
	maxSitemapURLs  = 50_000
	maxSitemapDepth = 3
	maxSitemaps     = 1000
)

type Options struct {
	Limit        int      // most pages to return; 0 means 500
	PerGroup     int      // pages kept per URL template; 0 means 3, <0 keeps every page
	FanOut       int      // children before a path segment becomes a template; 0 means 10
	Include      []string // if set, only paths matching one of these globs
	Exclude      []string // added to DefaultExcludes
	IgnoreRobots bool     // for your own site, when robots.txt blocks pages you want checked
	NoSitemaps   bool
	// NoFollow stops after robots.txt, sitemaps and the start page: the
	// caller follows links itself, e.g. from pages it renders in a browser,
	// so pages aren't fetched twice.
	NoFollow    bool
	UserAgent   string
	Rate        float64 // requests per second per host; 0 means unlimited
	Concurrency int     // 0 means 4
	MaxSitemaps int     // most sitemap files fetched, from robots.txt and indexes together; 0 means 1000
	// HTTP is the client to send requests with. The crawler uses a copy
	// and adds the same-site redirect rule after the client's own. When
	// nil, it uses a client with a 30 s timeout that won't connect to
	// loopback, link-local or private addresses (see AllowPrivate).
	HTTP *http.Client
	// AllowPrivate lets the default client connect to loopback,
	// link-local (169.254.0.0/16, fe80::/10) and private (RFC 1918, ULA)
	// addresses. Without it they are refused unless the start host
	// resolves to the same kind of address, so local sites still work.
	AllowPrivate bool
}

// Page is one discovered page.
type Page struct {
	URL    string `json:"url"`
	Group  string `json:"group"`  // URL template, e.g. /product/{slug}
	Source string `json:"source"` // start, sitemap, link, or config (pages the caller listed)
	Depth  int    `json:"depth"`  // links from the start page
}

// Frontier is the set of pages to visit. It's safe for concurrent use, so a
// browser that finds more links while rendering can keep adding to it.
type Frontier struct {
	start   *url.URL
	opts    Options
	robots  *robots
	groups  *grouper
	mu      sync.Mutex
	seen    map[string]bool
	perKey  map[string]int
	pages   []Page
	skipped map[string]int // reason → count
}

// NewFrontier returns an empty frontier for the site at start. Pass the
// parsed robots.txt via Crawl, or nil to allow everything.
func NewFrontier(start *url.URL, opts Options) *Frontier {
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.PerGroup == 0 {
		opts.PerGroup = 3
	}
	if opts.FanOut <= 0 {
		opts.FanOut = 10
	}
	return &Frontier{start: start, opts: opts, groups: newGrouper(opts.FanOut),
		seen: map[string]bool{}, perKey: map[string]int{}, skipped: map[string]int{}}
}

// Add offers a URL to the frontier and reports whether it was accepted.
func (f *Frontier) Add(raw, source string, depth int) bool {
	norm, ok := Normalize(f.start, raw)
	if !ok {
		return false
	}
	u, _ := url.Parse(norm)
	f.mu.Lock()
	defer f.mu.Unlock()
	k := key(norm)
	if f.seen[k] {
		return false
	}
	f.seen[k] = true
	// The start page is always admitted, because links are found from it,
	// and pages the user listed ("config") get past include, robots.txt
	// and grouping: they asked for them by name.
	start, listed := source == "start", source == "config"
	reason := ""
	switch {
	case !SameSite(f.start, u):
		reason = "other site"
	case !IsPage(u):
		reason = "not a page"
	case Excluded(u, DefaultExcludes) || Excluded(u, f.opts.Exclude):
		reason = "excluded"
	case len(f.opts.Include) > 0 && !Excluded(u, f.opts.Include) && !start && !listed:
		reason = "not included"
	case !f.opts.IgnoreRobots && !listed && !f.robots.Allowed(pathQuery(u)):
		reason = "robots.txt"
	case len(f.pages) >= f.opts.Limit:
		reason = "page limit"
	}
	if reason != "" {
		f.skipped[reason]++
		return false
	}
	group := f.groups.Group(u)
	if f.opts.PerGroup > 0 && f.perKey[group] >= f.opts.PerGroup && !listed {
		f.skipped["grouped"]++
		return false
	}
	f.perKey[group]++
	f.pages = append(f.pages, Page{URL: norm, Group: group, Source: source, Depth: depth})
	return true
}

// Pages returns the accepted pages in discovery order.
func (f *Frontier) Pages() []Page {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Page(nil), f.pages...)
}

// skip counts one URL left out for reason.
func (f *Frontier) skip(reason string) {
	f.mu.Lock()
	f.skipped[reason]++
	f.mu.Unlock()
}

// Skipped counts URLs left out, by reason ("grouped", "robots.txt",
// "offsite-sitemap", "offsite-redirect"…).
func (f *Frontier) Skipped() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int{}
	for k, v := range f.skipped {
		out[k] = v
	}
	return out
}

func pathQuery(u *url.URL) string {
	if u.RawQuery != "" {
		return u.EscapedPath() + "?" + u.RawQuery
	}
	return u.EscapedPath()
}

// Crawler discovers pages over plain HTTP.
type Crawler struct {
	opts  Options
	http  *http.Client
	hosts *ratelimit.Hosts
}

// New returns a crawler for opts. It never changes opts.HTTP.
func New(opts Options) *Crawler {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.MaxSitemaps <= 0 {
		opts.MaxSitemaps = maxSitemaps
	}
	c := &Crawler{opts: opts, hosts: ratelimit.NewHosts(opts.Rate)}
	c.http = c.newClient(opts)
	return c
}

// Crawl discovers pages starting at start: robots.txt, then sitemaps, then
// links breadth-first from the start page. It returns the frontier so
// callers can keep adding pages they find some other way.
func (c *Crawler) Crawl(ctx context.Context, start *url.URL) (*Frontier, error) {
	f := NewFrontier(start, c.opts)
	st := &crawlState{start: start, front: f}
	if c.opts.HTTP == nil && !c.opts.AllowPrivate {
		st.startIPs = resolve(ctx, start.Hostname())
	}
	ctx = context.WithValue(ctx, stateKey{}, st)
	if !c.opts.IgnoreRobots || !c.opts.NoSitemaps {
		f.robots = c.robots(ctx, start)
	}
	if c.opts.IgnoreRobots {
		f.robots = &robots{sitemaps: f.robots.sitemapsOrNil()}
	}
	f.Add(start.String(), "start", 0)
	if !c.opts.NoSitemaps {
		c.sitemaps(ctx, f, start)
	}
	if c.opts.NoFollow {
		return f, ctx.Err()
	}

	// Breadth-first over links. Pages from sitemaps are crawled too, at
	// depth 1, so their links count.
	type job struct {
		url   string
		depth int
	}
	var (
		mu      sync.Mutex
		queue   []job
		visited = map[string]bool{}
		wg      sync.WaitGroup
		sem     = make(chan struct{}, c.opts.Concurrency)
	)
	push := func(p Page) {
		mu.Lock()
		if !visited[p.URL] {
			visited[p.URL] = true
			queue = append(queue, job{p.URL, p.Depth})
		}
		mu.Unlock()
	}
	for _, p := range f.Pages() {
		push(p)
	}
	for {
		mu.Lock()
		if len(queue) == 0 {
			mu.Unlock()
			wg.Wait()
			mu.Lock()
			if len(queue) == 0 {
				mu.Unlock()
				break
			}
		}
		j := queue[0]
		queue = queue[1:]
		mu.Unlock()
		if ctx.Err() != nil {
			wg.Wait()
			return f, ctx.Err()
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			links, err := c.links(ctx, j.url)
			if err != nil {
				return
			}
			before := len(f.Pages())
			for _, l := range links {
				f.Add(l, "link", j.depth+1)
			}
			for _, p := range f.Pages()[before:] {
				push(p)
			}
		})
	}
	return f, ctx.Err()
}

func (r *robots) sitemapsOrNil() []string {
	if r == nil {
		return nil
	}
	return r.sitemaps
}

func (c *Crawler) get(ctx context.Context, raw string) (*http.Response, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := c.hosts.For(u.Host).Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	if c.opts.UserAgent != "" {
		req.Header.Set("User-Agent", c.opts.UserAgent)
	}
	return c.http.Do(req)
}

func (c *Crawler) robots(ctx context.Context, start *url.URL) *robots {
	resp, err := c.get(ctx, start.Scheme+"://"+start.Host+"/robots.txt")
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	return parseRobots(io.LimitReader(resp.Body, 512<<10), c.opts.UserAgent)
}

// sitemaps reads sitemaps named in robots.txt, or the usual locations.
func (c *Crawler) sitemaps(ctx context.Context, f *Frontier, start *url.URL) {
	root := start.Scheme + "://" + start.Host
	locs := f.robots.sitemapsOrNil()
	if len(locs) == 0 {
		locs = []string{root + "/sitemap.xml", root + "/wp-sitemap.xml", root + "/sitemap_index.xml"}
	}
	seen := map[string]bool{}
	n, fetched := 0, 0
	var read func(loc string, depth int)
	read = func(loc string, depth int) {
		if seen[loc] || depth > maxSitemapDepth || n >= maxSitemapURLs || ctx.Err() != nil {
			return
		}
		seen[loc] = true
		// robots.txt and sitemap indexes are written by the site, so a
		// hostile one could name any host; only the site's own are read.
		u, err := start.Parse(loc)
		if err != nil || !SameSite(start, u) {
			f.skip("offsite-sitemap")
			return
		}
		if fetched >= c.opts.MaxSitemaps {
			f.skip("sitemap limit")
			return
		}
		fetched++
		var doc struct {
			XMLName  xml.Name
			URLs     []string `xml:"url>loc"`
			Sitemaps []string `xml:"sitemap>loc"`
		}
		if err := c.xml(ctx, u.String(), &doc); err != nil {
			return
		}
		for _, u := range doc.URLs {
			if n++; n > maxSitemapURLs {
				return
			}
			f.Add(strings.TrimSpace(u), "sitemap", 1)
		}
		for _, s := range doc.Sitemaps {
			read(strings.TrimSpace(s), depth+1)
		}
	}
	for _, loc := range locs {
		read(loc, 0)
	}
}

func (c *Crawler) xml(ctx context.Context, loc string, v any) error {
	resp, err := c.get(ctx, loc)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	br := bufio.NewReader(io.LimitReader(resp.Body, 50<<20))
	if b, _ := br.Peek(2); len(b) == 2 && b[0] == 0x1f && b[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return err
		}
		defer func() { _ = zr.Close() }()
		return xml.NewDecoder(io.LimitReader(zr, 50<<20)).Decode(v)
	}
	return xml.NewDecoder(br).Decode(v)
}

// links fetches a page and returns the hrefs it links to. Non-HTML
// responses and pages that redirect off the site give no links.
func (c *Crawler) links(ctx context.Context, raw string) ([]string, error) {
	resp, err := c.get(ctx, raw)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	start, _ := url.Parse(raw)
	if !SameSite(start, resp.Request.URL) {
		return nil, nil
	}
	if mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); mt != "" && mt != "text/html" && mt != "application/xhtml+xml" {
		return nil, nil
	}
	return ExtractLinks(resp.Request.URL, io.LimitReader(resp.Body, maxBody))
}

// ExtractLinks returns the absolute URLs of <a href>, <area href> and
// <link rel="next|prev"> in an HTML document, honoring <base href>.
func ExtractLinks(base *url.URL, r io.Reader) ([]string, error) {
	z := html.NewTokenizer(r)
	var out []string
	for {
		switch z.Next() {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return out, nil
			}
			return out, z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			if !hasAttr {
				continue
			}
			tag := string(name)
			var href, rel, target string
			for {
				k, v, more := z.TagAttr()
				switch string(k) {
				case "href":
					href = string(v)
				case "rel":
					rel = strings.ToLower(string(v))
				case "download":
					target = "download"
				}
				if !more {
					break
				}
			}
			switch {
			case tag == "base" && href != "":
				if b, err := base.Parse(href); err == nil {
					base = b
				}
			case (tag == "a" || tag == "area") && href != "" && target != "download" && !strings.Contains(rel, "nofollow"):
				if u, err := base.Parse(strings.TrimSpace(href)); err == nil {
					out = append(out, u.String())
				}
			case tag == "link" && (rel == "next" || rel == "prev") && href != "":
				if u, err := base.Parse(strings.TrimSpace(href)); err == nil {
					out = append(out, u.String())
				}
			}
		}
	}
}
