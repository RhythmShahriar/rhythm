// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package crawl

import (
	"bufio"
	"io"
	"strings"
)

// robots holds the rules from robots.txt that apply to one user agent.
type robots struct {
	allow, disallow []string
	sitemaps        []string
}

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
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
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
// pattern wins, and Allow wins a tie.
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
