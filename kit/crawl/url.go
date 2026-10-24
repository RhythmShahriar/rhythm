// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/httpx"
)

// Query parameters that only track visits; dropping them dedupes URLs.
var trackingParam = regexp.MustCompile(`^(utm_.*|fbclid|gclid|dclid|msclkid|mc_cid|mc_eid|_ga|_gl|yclid|igshid|ref_src)$`)

// Extensions that aren't pages. Assets are checked by the browser layer.
var nonPage = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|avif|svg|ico|bmp|tiff?|pdf|zip|gz|tgz|rar|7z|tar|mp[34]|m4a|wav|ogg|webm|mov|avi|woff2?|ttf|eot|otf|css|js|json|xml|txt|csv|xlsx?|docx?|pptx?|exe|dmg|apk|iso)$`)

// Normalize resolves href against base and canonicalizes it: lowercase
// scheme and host, no default port, no fragment, no tracking parameters,
// sorted query, cleaned path. It keeps what changes a URL's meaning: an
// escaped slash in the path (/files/a%2Fb is not /files/a/b), a key with
// no value (?amp stays ?amp, not ?amp=) and, for a query Go can't parse
// (one with ';' or a bad escape), every pair as written and in order.
// ok is false for non-http(s) links.
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
	u.Path = cleanPath(u.Path)
	if u.RawPath != "" {
		// RawPath is set only when the path has escapes that matter, such
		// as %2F. Clean it the same way and derive Path from it, so the
		// two stay consistent and String keeps the escapes.
		raw := cleanPath(u.RawPath)
		if p, err := url.PathUnescape(raw); err == nil {
			u.Path, u.RawPath = p, raw
		} else {
			u.RawPath = ""
		}
	}
	u.ForceQuery = false
	u.RawQuery = normQuery(u.RawQuery)
	return u.String(), true
}

// cleanPath cleans p and keeps a trailing slash; empty becomes "/".
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	trailing := strings.HasSuffix(p, "/")
	p = path.Clean(p)
	if trailing && p != "/" {
		p += "/"
	}
	return p
}

// normQuery drops tracking parameters from a raw query. A query Go
// parses cleanly is sorted by key (values of one key keep their order)
// and re-escaped the standard way, so equivalent URLs compare equal. A
// query it can't parse is kept pair by pair as written, because
// url.Query would silently drop the pairs it doesn't understand.
func normQuery(raw string) string {
	if raw == "" {
		return ""
	}
	type pair struct{ key, text string }
	_, err := url.ParseQuery(raw)
	verbatim := err != nil || strings.Contains(raw, ";")
	var pairs []pair
	for _, p := range strings.Split(raw, "&") {
		if p == "" {
			continue
		}
		k, v, hasValue := strings.Cut(p, "=")
		key, kerr := url.QueryUnescape(k)
		if kerr != nil {
			key = k
		}
		if trackingParam.MatchString(strings.ToLower(key)) {
			continue
		}
		if verbatim {
			pairs = append(pairs, pair{key, p})
			continue
		}
		val, _ := url.QueryUnescape(v) // ParseQuery succeeded, so this can't fail
		text := url.QueryEscape(key)
		if hasValue {
			text += "=" + url.QueryEscape(val)
		}
		pairs = append(pairs, pair{key, text})
	}
	if !verbatim {
		slices.SortStableFunc(pairs, func(a, b pair) int { return strings.Compare(a.key, b.key) })
	}
	texts := make([]string, len(pairs))
	for i, p := range pairs {
		texts[i] = p.text
	}
	return strings.Join(texts, "&")
}

// key identifies a page for de-duplication: /about and /about/ are one
// page. Normalize keeps the trailing slash, because the URL that is
// fetched and reported should be the one the site links to (many sites
// redirect one form to the other); only the de-duplication ignores it.
func key(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Path != "/" {
		u.Path = strings.TrimSuffix(u.Path, "/")
		u.RawPath = strings.TrimSuffix(u.RawPath, "/")
	}
	return u.String()
}

// SameSite reports whether u is on the same site as start: the same host
// (ignoring case and a leading "www."), the same scheme and the same
// port, where a missing port means the scheme's default (80 for http, 443
// for https). Another port on the same host is often another service (an
// admin panel, a database UI), so it counts as another site. One upgrade
// is allowed: an http start on the default port may go to https on the
// default port, because most sites redirect there. It is httpx.SameSite,
// kept here so the crawler's rule is documented where it is used.
func SameSite(start, u *url.URL) bool { return httpx.SameSite(start, u) }

// IsPage reports whether u looks like a page rather than a file.
func IsPage(u *url.URL) bool { return !nonPage.MatchString(u.Path) }

// DefaultExcludes skips admin, login, feeds, APIs and URLs with side
// effects: logout, add to cart, comment replies, unsubscribe and opt-out
// links, confirmations, delete and trash actions and links carrying a
// token or nonce, which can act on a real person's account just by being
// visited. Patterns match the lower-cased path and query.
var DefaultExcludes = append([]string{
	"/wp-admin*", "/wp-login.php*", "/xmlrpc.php*", "/wp-json*", "*/feed", "*/feed/*", "/wp-cron.php*",
	"*?*replytocom=*", "*?*add-to-cart=*", "*?*remove_item=*", "*?*undo_item=*", "*?*removed_item=*",
	"*?*action=logout*", "*?*preview=*", "*?*attachment_id=*",
	"*?*customize_changeset_uuid=*", "*?s=*", "*&s=*", "/cart/?*", "/my-account/customer-logout*",
	"*unsubscribe*", "*/opt-out*", "*/optout*",
	"*?*action=delete*", "*?*action=trash*", "*?*rest_route=*", "*?*token=*", "*?*_wpnonce=*",
}, segment("logout", "log-out", "signout", "sign-out", "logoff", "confirm")...)

// segment returns patterns that match each name only as a whole path
// segment (/logout, /logout/, /logout/now, /logout.php, /logout?next=/),
// so /blog/logout-tips-for-teams/ and /confirmation-policy are crawled.
func segment(names ...string) []string {
	var out []string
	for _, n := range names {
		out = append(out, "*/"+n, "*/"+n+"/*", "*/"+n+".*", "*/"+n+"?*")
	}
	return out
}

// Excluded reports whether u matches any pattern. A pattern is matched
// against the path plus query ("/shop/?orderby=price"), with * matching
// any run of characters, including /. Matching ignores case, for
// DefaultExcludes and the caller's Include and Exclude patterns alike,
// so /WP-ADMIN/ is as excluded as /wp-admin/.
func Excluded(u *url.URL, patterns []string) bool {
	target := u.EscapedPath()
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	target = strings.ToLower(target)
	return slices.ContainsFunc(patterns, func(p string) bool { return glob(strings.ToLower(p), target) })
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
