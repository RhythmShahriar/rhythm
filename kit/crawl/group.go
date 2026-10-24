// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var (
	numeric = regexp.MustCompile(`^[0-9]+$`)
	year    = regexp.MustCompile(`^(19|20)[0-9]{2}$`)
	month   = regexp.MustCompile(`^(0?[1-9]|1[0-2])$`)
	hexID   = regexp.MustCompile(`^[0-9a-f]{8,}$|^[0-9a-f]{8}-[0-9a-f]{4}-`)
)

// grouper assigns pages to URL templates so a site with 2,000 product
// pages is checked through a few samples. Numeric, date and ID segments
// always generalize; a slug segment generalizes once its parent has more
// than fanOut distinct children (so /about and /contact stay separate
// while /product/<anything> becomes one group).
type grouper struct {
	fanOut   int
	mu       sync.Mutex
	children map[string]map[string]bool // parent prefix → child segments seen
}

func newGrouper(fanOut int) *grouper {
	return &grouper{fanOut: fanOut, children: map[string]map[string]bool{}}
}

// contentParams name the page in a query string, as WordPress's plain
// permalinks do (/?page_id=2, /?p=7, /?cat=3).
var contentParams = []string{"page_id", "p", "cat", "tag", "author", "post_type", "attachment_id", "product", "product_cat", "m"}

// Group returns the template key for u, e.g. "/product/{slug}" or
// "/2026/{m}/{slug}". The query is dropped except for pagination and
// parameters that name the content (plain permalinks), which group like a
// path segment: /?page_id=2 and /?page_id=9 stay apart until there are
// more than fanOut of them.
func (g *grouper) Group(u *url.URL) string {
	q := u.Query()
	for _, name := range contentParams {
		if v := q.Get(name); v != "" {
			base := strings.TrimSuffix(u.Path, "index.php") + "?" + name + "="
			return base + g.child(base, v) + paged(q)
		}
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) == 1 && segs[0] == "" {
		return "/" + paged(q)
	}
	out := make([]string, len(segs))
	for i, s := range segs {
		switch {
		case year.MatchString(s) && i < len(segs)-1:
			out[i] = "{y}"
		case month.MatchString(s) && i > 0 && out[i-1] == "{y}":
			out[i] = "{m}"
		case numeric.MatchString(s):
			out[i] = "{n}"
		case hexID.MatchString(strings.ToLower(s)):
			out[i] = "{id}"
		default:
			out[i] = s
		}
	}
	// The last segment: generalize when its parent fans out.
	last := len(out) - 1
	if !strings.HasPrefix(out[last], "{") {
		out[last] = g.child("/"+strings.Join(out[:last], "/"), out[last])
	}
	return "/" + strings.Join(out, "/") + paged(q)
}

// child returns name, or "{slug}" once parent has more than fanOut
// distinct children.
func (g *grouper) child(parent, name string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	kids := g.children[parent]
	if kids == nil {
		kids = map[string]bool{}
		g.children[parent] = kids
	}
	if !kids[name] && len(kids) < g.fanOut {
		kids[name] = true
	}
	if !kids[name] {
		return "{slug}"
	}
	return name
}

// paged marks paginated pages, so page 2, 3… of a list group together.
func paged(q url.Values) string {
	if q.Has("page") || q.Has("paged") {
		return "?{page}"
	}
	return ""
}
