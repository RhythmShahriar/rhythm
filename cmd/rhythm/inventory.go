// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The router itself, as a target of rhythm update.
const (
	selfName = "rhythm"
	selfRepo = "RhythmShahriar/rhythm"
)

// target is one binary rhythm update looks after: rhythm itself or a
// tool found on PATH.
type target struct {
	tool    string // registry name, or selfName
	binary  string // e.g. rhythm-tuner
	repo    string
	current string // the version the binary reports
	path    string // where it was found
}

// selfPath returns the running rhythm binary with symlinks resolved. It is
// a variable so tests can point it at a stand-in.
var selfPath = func() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// resolveName maps a tool name, alias or binary name ("tuner", "t",
// "rhythm-tuner") to its registry name. "rhythm" names the router.
func resolveName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == selfName {
		return selfName, true
	}
	name = strings.TrimPrefix(name, "rhythm-")
	if full, ok := aliases[name]; ok {
		return full, true
	}
	_, ok := tools[name]
	return name, ok
}

// suggest returns " (did you mean tuner?)" for a name close to a known
// one, or "" when nothing is close enough to be a typo.
func suggest(name string) string {
	name = strings.ToLower(name)
	best, bestDist := "", 3 // only suggest within 2 edits
	consider := func(candidate, full string) {
		if d := levenshtein(name, candidate); d < bestDist {
			best, bestDist = full, d
		}
	}
	consider(selfName, selfName)
	for _, t := range toolNames() {
		consider(t, t)
		consider("rhythm-"+t, t)
	}
	for a, full := range aliases {
		// One- and two-letter aliases are within two edits of every short
		// word, so they would suggest nonsense.
		if len(a) > 2 {
			consider(a, full)
		}
	}
	if best == "" {
		return ""
	}
	return " (did you mean " + best + "?)"
}

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// findTargets returns rhythm itself and every tool installed on PATH, or
// only those in want when it is non-nil. missing lists the wanted tools
// that aren't installed.
func findTargets(want map[string]bool) (targets []target, missing []string) {
	if want == nil || want[selfName] {
		if self, err := selfPath(); err == nil {
			targets = append(targets, target{selfName, selfName, selfRepo, version, self})
		}
	}
	for _, tool := range toolNames() {
		if want != nil && !want[tool] {
			continue
		}
		bin, err := exec.LookPath("rhythm-" + tool)
		if err != nil {
			if want != nil {
				missing = append(missing, tool)
			}
			continue
		}
		targets = append(targets, target{tool, "rhythm-" + tool, tools[tool], toolVersion(bin), bin})
	}
	return targets, missing
}
