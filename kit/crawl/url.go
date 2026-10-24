// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Query parameters that only track visits; dropping them dedupes URLs.
var trackingParam = regexp.MustCompile(`^(utm_.*|fbclid|gclid|dclid|msclkid|mc_cid|mc_eid|_ga|_gl|yclid|igshid|ref_src)$`)

// Extensions that aren't pages. Assets are checked by the browser layer.
var nonPage = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|avif|svg|ico|bmp|tiff?|pdf|zip|gz|tgz|rar|7z|tar|mp[34]|m4a|wav|ogg|webm|mov|avi|woff2?|ttf|eot|otf|css|js|json|xml|txt|csv|xlsx?|docx?|pptx?|exe|dmg|apk|iso)$`)

// Normalize resolves href against base and canonicalizes it: lowercase
// scheme and host, no default port, no fragment, no tracking parameters,
// sorted query, cleaned path. ok is false for non-http(s) links.
func Normalize(base *url.URL, href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return "", false
	}
	u, err := base.Parse(href)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		u.Host = u.Hostname()
	}
	u.Fragment, u.RawFragment = "", ""
	u.User = nil
	if u.Path == "" {
		u.Path = "/"
	} else {
		trailing := strings.HasSuffix(u.Path, "/")
		u.Path = path.Clean(u.Path)
		if trailing && u.Path != "/" {
			u.Path += "/"
		}
	}
	u.RawPath = ""
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if trackingParam.MatchString(strings.ToLower(k)) {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode() // Encode sorts by key
	}
	return u.String(), true
}

// key identifies a page for de-duplication: /about and /about/ are one page.
func key(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Path != "/" {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}

// site is a host without "www.", so example.com and www.example.com are
// the same site.
func site(host string) string { return strings.TrimPrefix(strings.ToLower(host), "www.") }

// SameSite reports whether u is on the same site as start: the same host
// (ignoring case and a leading "www."), the same scheme and the same
// port, where a missing port means the scheme's default (80 for http, 443
// for https). Another port on the same host is often another service (an
// admin panel, a database UI), so it counts as another site. One upgrade
// is allowed: an http start on the default port may go to https on the
// default port, because most sites redirect there.
func SameSite(start, u *url.URL) bool {
	if site(start.Hostname()) != site(u.Hostname()) {
		return false
	}
	ss, us := strings.ToLower(start.Scheme), strings.ToLower(u.Scheme)
	sp, up := port(ss, start.Port()), port(us, u.Port())
	if ss == us {
		return sp == up
	}
	return ss == "http" && us == "https" && sp == "80" && up == "443"
}

// port returns p, or the default port for scheme when p is empty.
func port(scheme, p string) string {
	if p != "" {
		return p
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}

// IsPage reports whether u looks like a page rather than a file.
func IsPage(u *url.URL) bool { return !nonPage.MatchString(u.Path) }

// DefaultExcludes skips admin, login, feeds, APIs and URLs with side effects
// (logout, add to cart, comment replies). Patterns match the path and query.
var DefaultExcludes = []string{
	"/wp-admin*", "/wp-login.php*", "/xmlrpc.php*", "/wp-json*", "*/feed", "*/feed/*", "/wp-cron.php*",
	"*?*replytocom=*", "*?*add-to-cart=*", "*?*remove_item=*", "*?*undo_item=*", "*?*removed_item=*",
	"*?*action=logout*", "*/logout*", "*/log-out*", "*/signout*", "*?*preview=*", "*?*attachment_id=*",
	"*?*customize_changeset_uuid=*", "*?s=*", "*&s=*", "/cart/?*", "/my-account/customer-logout*",
}

// Excluded reports whether u matches any pattern. A pattern is matched
// against the path plus query ("/shop/?orderby=price"), with * matching
// any run of characters, including /.
func Excluded(u *url.URL, patterns []string) bool {
	target := u.EscapedPath()
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return slices.ContainsFunc(patterns, func(p string) bool { return glob(p, target) })
}

// glob matches s against p where * is any run of characters.
func glob(p, s string) bool {
	parts := strings.Split(p, "*")
	if len(parts) == 1 {
		return p == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(s, mid)
		if i < 0 {
			return false
		}
		s = s[i+len(mid):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}
