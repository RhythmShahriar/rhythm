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

// Group returns the template key for u, e.g. "/product/{slug}" or
// "/2026/{m}/{slug}". The query is dropped except for pagination.
func (g *grouper) Group(u *url.URL) string {
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) == 1 && segs[0] == "" {
		return "/"
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
		parent := "/" + strings.Join(out[:last], "/")
		g.mu.Lock()
		kids := g.children[parent]
		if kids == nil {
			kids = map[string]bool{}
			g.children[parent] = kids
		}
		if !kids[out[last]] && len(kids) < g.fanOut {
			kids[out[last]] = true
		}
		fanned := !kids[out[last]]
		g.mu.Unlock()
		if fanned {
			out[last] = "{slug}"
		}
	}
	key := "/" + strings.Join(out, "/")
	if q := u.Query(); q.Has("page") || q.Has("paged") || q.Has("p") {
		key += "?{page}"
	}
	return key
}
