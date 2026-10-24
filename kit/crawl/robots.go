// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// maxRobotsLine is the longest robots.txt line read. It matches the
// most of the file that is read at all, so no line is ever cut short.
const maxRobotsLine = 512 << 10

// robots holds the rules from robots.txt that apply to one user agent.
type robots struct {
	allow, disallow []string
	sitemaps        []string
	// unavailable says why robots.txt couldn't be read ("HTTP 503",
	// a network error), or is empty. RFC 9309 2.3.1.4: when the server
	// can't serve it, a crawler assumes everything is disallowed.
	unavailable string
}

// unavailableRobots disallows everything, for a robots.txt that could
// not be read for reason.
func unavailableRobots(reason string) *robots { return &robots{unavailable: reason} }

// parseRobots reads robots.txt and keeps the group for agent (matched by
// product token, case-insensitive), falling back to "*".
func parseRobots(r io.Reader, agent string) *robots {
	agent = strings.ToLower(agent)
	if i := strings.IndexAny(agent, "/ "); i > 0 {
		agent = agent[:i]
	}
	type group struct{ allow, disallow []string }
	groups := map[string]*group{}
	var current []*group
	inAgents := false
	out := &robots{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxRobotsLine)
	for first := true; sc.Scan(); first = false {
		text := sc.Text()
		if first {
			text = strings.TrimPrefix(text, "\uFEFF")
		}
		line, _, _ := strings.Cut(text, "#")
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			if !inAgents {
				current = nil
			}
			inAgents = true
			name := strings.ToLower(v)
			g := groups[name]
			if g == nil {
				g = &group{}
				groups[name] = g
			}
			current = append(current, g)
		case "allow", "disallow":
			inAgents = false
			v = escapeRule(v)
			for _, g := range current {
				if k == "allow" {
					g.allow = append(g.allow, v)
				} else {
					g.disallow = append(g.disallow, v)
				}
			}
		case "sitemap":
			out.sitemaps = append(out.sitemaps, v)
		}
	}
	if err := sc.Err(); err != nil {
		// A file read only in part could be missing the very rule that
		// matters, so it counts as unavailable rather than truncated.
		return unavailableRobots(fmt.Sprintf("unreadable (%v)", err))
	}
	g := groups[agent]
	if g == nil {
		g = groups["*"]
	}
	if g != nil {
		out.allow, out.disallow = g.allow, g.disallow
	}
	return out
}

// Allowed applies the longest-match rule: the most specific matching
// pattern wins, and Allow wins a tie. It looks at the rules only; the
// frontier checks unavailable first, so the start page still gets in.
func (r *robots) Allowed(pathQuery string) bool {
	if r == nil {
		return true
	}
	best, allowed := -1, true
	for _, p := range r.disallow {
		if p == "" {
			continue // "Disallow:" with no path allows everything
		}
		if n := robotsMatch(p, pathQuery); n > best {
			best, allowed = n, false
		}
	}
	for _, p := range r.allow {
		if n := robotsMatch(p, pathQuery); n >= best && n >= 0 {
			best, allowed = n, true
		}
	}
	return allowed
}

// robotsMatch returns the pattern's length if it matches s, else -1.
// Patterns support * and a trailing $.
func robotsMatch(p, s string) int {
	anchored := strings.HasSuffix(p, "$")
	pat := strings.TrimSuffix(p, "$")
	if !anchored {
		pat += "*"
	}
	if glob(pat, s) {
		return len(p)
	}
	return -1
}

// escapeRule percent-encodes a rule the way paths are compared: bytes
// outside printable ASCII become %XX and existing escapes are upper-cased,
// so "Disallow: /café" matches /caf%C3%A9. * and $ are left alone.
func escapeRule(p string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '%' && i+2 < len(p) && ishex(p[i+1]) && ishex(p[i+2]):
			b.WriteByte('%')
			b.WriteString(strings.ToUpper(p[i+1 : i+3]))
			i += 2
		case c <= ' ' || c >= 0x7f:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func ishex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
