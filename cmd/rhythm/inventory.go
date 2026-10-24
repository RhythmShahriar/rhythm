// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
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
	current string // the version the binary reports, or unknownVersion
	probe   string // why the version is unknown
	hung    bool   // the version probe timed out
	path    string // where it was found
	real    string // path with symlinks resolved; updates write here so links stay links
	managed *manager
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

// splitNames splits each argument on commas, so "tuner,echo" works as in
// every tool's list flags. Empty names are dropped.
func splitNames(args []string) []string {
	var list []string
	for _, a := range args {
		for _, part := range strings.Split(a, ",") {
			if part = strings.TrimSpace(part); part != "" {
				list = append(list, part)
			}
		}
	}
	return list
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
// that aren't installed. Tools are asked for their versions in parallel,
// so a slow one costs the probe timeout once, not once per tool.
func findTargets(ctx context.Context, want map[string]bool) (targets []target, missing []string) {
	if want == nil || want[selfName] {
		if self, err := selfPath(); err == nil {
			targets = append(targets, target{tool: selfName, binary: selfName, repo: selfRepo, current: version, path: self, real: self})
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
		targets = append(targets, target{tool: tool, binary: "rhythm-" + tool, repo: tools[tool], path: bin, real: resolve(bin)})
	}
	for i := range targets {
		targets[i].managed = managedBy(targets[i].real, targets[i].binary)
	}
	var wg sync.WaitGroup
	for i := range targets {
		if targets[i].tool == selfName {
			continue // the running binary knows its own version
		}
		wg.Go(func() {
			t := &targets[i]
			t.current, t.probe, t.hung = probeVersion(ctx, t.path)
		})
	}
	wg.Wait()
	return targets, missing
}

// resolve follows symlinks from p, or returns p when it can't.
func resolve(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// manager is a package manager that owns an install.
type manager struct {
	name    string // e.g. Homebrew
	upgrade string // the command that updates the install
	remove  string // the command that uninstalls it
}

// managedBy reports which package manager installed the binary at real (a
// path with symlinks resolved), or nil when rhythm may replace it. Writing
// over a managed install would leave the manager's records wrong. Its
// next upgrade or cleanup would then undo rhythm's work.
func managedBy(real, binary string) *manager {
	// Windows paths are compared with forward slashes; case doesn't
	// matter there or on macOS.
	p := strings.ReplaceAll(real, `\`, "/")
	lower := strings.ToLower(p)
	if len(lower) != len(p) {
		p = lower // non-ASCII case folding changed the length; offsets must match
	}
	// segment returns the path element starting at at: the formula, app
	// or plugin name in the managers' layouts. next is the element after
	// it, the version folder in mise's and asdf's.
	segment := func(at int) (name, next string) {
		parts := strings.SplitN(p[at:], "/", 3)
		name = parts[0]
		if name == "" {
			name = binary
		}
		if len(parts) > 1 {
			next = parts[1]
		}
		return name, next
	}
	contains := func(marker string) (int, bool) {
		i := strings.Index(lower, marker)
		return i + len(marker), i >= 0
	}
	under := func(env, sub string) (int, bool) {
		dir := strings.ToLower(strings.ReplaceAll(os.Getenv(env), `\`, "/"))
		if dir == "" {
			return 0, false
		}
		prefix := strings.TrimSuffix(dir, "/") + sub
		return len(prefix), strings.HasPrefix(lower, prefix)
	}
	mise := func(at int) *manager {
		tool, v := segment(at)
		return &manager{"mise", "mise upgrade " + tool, "mise uninstall " + tool + "@" + v}
	}
	asdf := func(at int) *manager {
		plugin, v := segment(at)
		return &manager{"asdf", "asdf install " + plugin + " latest", "asdf uninstall " + plugin + " " + v}
	}
	scoop := func(at int) *manager {
		app, _ := segment(at)
		return &manager{"Scoop", "scoop update " + app, "scoop uninstall " + app}
	}

	if at, ok := contains("/caskroom/"); ok {
		cask, _ := segment(at)
		return &manager{"Homebrew", "brew upgrade --cask " + cask, "brew uninstall --cask " + cask}
	}
	if at, ok := contains("/cellar/"); ok {
		formula, _ := segment(at)
		return &manager{"Homebrew", "brew upgrade " + formula, "brew uninstall " + formula}
	}
	if strings.HasPrefix(lower, "/opt/homebrew/") || strings.HasPrefix(lower, "/home/linuxbrew/") {
		return &manager{"Homebrew", "brew upgrade " + binary, "brew uninstall " + binary}
	}
	if strings.HasPrefix(lower, "/nix/store/") {
		return &manager{"Nix", "nix profile upgrade " + binary, "nix profile remove " + binary}
	}
	if at, ok := under("MISE_DATA_DIR", "/installs/"); ok {
		return mise(at)
	}
	if at, ok := contains("/mise/installs/"); ok {
		return mise(at)
	}
	if at, ok := under("ASDF_DATA_DIR", "/installs/"); ok {
		return asdf(at)
	}
	if at, ok := contains("/.asdf/installs/"); ok {
		return asdf(at)
	}
	if at, ok := under("SCOOP", "/apps/"); ok {
		return scoop(at)
	}
	if at, ok := contains("/scoop/apps/"); ok {
		return scoop(at)
	}
	if at, ok := contains("/winget/packages/"); ok {
		// Package folders are named <id>_<source>, e.g.
		// RhythmShahriar.rhythm_Microsoft.Winget.Source_8wekyb3d8bbwe.
		folder, _ := segment(at)
		id, _, _ := strings.Cut(folder, "_")
		return &manager{"WinGet", "winget upgrade --id " + id, "winget uninstall --id " + id}
	}
	return nil
}

// unknownVersion is reported for a tool whose version couldn't be read.
const unknownVersion = "unknown"

// probeTimeout bounds the version probes of one tool, so a tool that
// hangs can't hang rhythm update with it.
var probeTimeout = 5 * time.Second

// probeVersion asks a tool binary for its version: first `version --json`
// (the rhythm-version/v1 answer of tools built on kit/cli), then the
// plain `--version` every tool has. hung reports a timeout.
func probeVersion(ctx context.Context, bin string) (v, reason string, hung bool) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	timedOut := func() bool { return errors.Is(ctx.Err(), context.DeadlineExceeded) }

	if out, err := probeOutput(ctx, bin, "version", "--json"); err == nil {
		if v, ok := versionFromJSON(out); ok {
			return v, "", false
		}
	}
	if timedOut() {
		return unknownVersion, fmt.Sprintf("no answer within %s", probeTimeout), true
	}
	out, err := probeOutput(ctx, bin, "--version")
	switch {
	case timedOut():
		return unknownVersion, fmt.Sprintf("no answer within %s", probeTimeout), true
	case err != nil:
		return unknownVersion, fmt.Sprintf("%s --version failed: %v", filepath.Base(bin), err), false
	}
	if v, ok := versionFromText(out); ok {
		return v, "", false
	}
	return unknownVersion, fmt.Sprintf("%s --version printed no version number", filepath.Base(bin)), false
}

// probeOutput runs bin with args and returns what it printed on stdout,
// at most 64 KiB of it.
func probeOutput(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // our own tool binaries
	out := &capped{max: 64 << 10}
	cmd.Stdout = out
	// A killed shell script can leave a child holding stdout open;
	// WaitDelay stops waiting for it shortly after the kill.
	cmd.WaitDelay = 500 * time.Millisecond
	err := cmd.Run()
	return out.buf.Bytes(), err
}

// capped keeps the first max bytes written to it and drops the rest, so a
// tool that floods its output can't exhaust memory.
type capped struct {
	buf bytes.Buffer
	max int
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// versionFromJSON reads a rhythm-version/v1 answer:
// {"schema":"rhythm-version/v1","name":...,"version":"0.3.0",...}.
func versionFromJSON(out []byte) (string, bool) {
	var v struct {
		Schema  string `json:"schema"`
		Version string `json:"version"`
	}
	if json.Unmarshal(out, &v) != nil || v.Schema != "rhythm-version/v1" || v.Version == "" {
		return "", false
	}
	return strings.TrimPrefix(v.Version, "v"), true
}

// semverToken matches a version such as 0.3.0, v1.2.3-rc.1 or 1.0.0+build.
var semverToken = regexp.MustCompile(`^v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)(?:\+[0-9A-Za-z.-]+)?$`)

// versionFromText finds the version in --version output such as
// "rhythm-tuner 0.3.0 (commit abc1234)": the first word that is a semver.
// A development build says "dev" instead.
func versionFromText(out []byte) (string, bool) {
	words := strings.Fields(string(out))
	for _, w := range words {
		if m := semverToken.FindStringSubmatch(strings.Trim(w, "(),;:")); m != nil {
			return m[1], true
		}
	}
	for _, w := range words {
		if w == "dev" {
			return w, true
		}
	}
	return "", false
}
