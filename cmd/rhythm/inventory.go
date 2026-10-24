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

	"github.com/RhythmShahriar/rhythm/kit/config"
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
	kitCLI  bool   // built on kit/cli.Main, so it has update --json
	path    string // where it was found
	real    string // path with symlinks resolved; updates write here so links stay links
	managed *manager

	extension bool   // from extensions.json, not the registry
	signer    string // the signer an extension's releases must carry; "" for any
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
	probeTargets(ctx, targets)
	return targets, missing
}

// probeTargets finds out how each target was installed and which
// version it is.
func probeTargets(ctx context.Context, targets []target) {
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
			var p versionProbe
			p, t.probe = probeVersionInfo(ctx, t.path)
			t.current, t.hung, t.kitCLI = p.version, p.hung, p.kitCLI
		})
	}
	wg.Wait()
}

// extension is a third-party tool listed in extensions.json in the
// Rhythm config folder: `rhythm <name>` runs rhythm-<name>, released
// from repo. This reads internal/lock's rhythm-extensions/v1 format
// until that package is part of this rhythm.
type extension struct {
	Name   string `json:"name"`
	Repo   string `json:"repo"`
	Signer string `json:"signer,omitempty"`
}

const (
	extensionsSchema = "rhythm-extensions/v1"
	extensionsFile   = "extensions.json"
)

var (
	extensionName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	extensionRepo = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)
)

// validRepo reports whether repo is a GitHub owner/name. Release API
// paths are built from it, so "." and ".." (which the pattern allows,
// as GitHub allows dots in names) are refused too.
func validRepo(repo string) bool {
	_, name, _ := strings.Cut(repo, "/")
	return extensionRepo.MatchString(repo) && name != "." && name != ".."
}

// registryRepo returns the repository rhythm itself installs name from:
// its own for rhythm, the registry's for a tool. ok is false for any
// other name, such as an extension.
func registryRepo(name string) (repo string, ok bool) {
	if name == selfName {
		return selfRepo, true
	}
	repo, ok = tools[name]
	return repo, ok
}

// readExtensions returns the extensions in the Rhythm config folder
// (RHYTHM_CONFIG_DIR when set), none when there is no file. A name the
// registry already has is left out: the registry's repo wins.
func readExtensions() ([]extension, error) {
	dir := os.Getenv("RHYTHM_CONFIG_DIR")
	if dir != "" && !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("RHYTHM_CONFIG_DIR must be an absolute path, not %q", dir)
	}
	if dir == "" {
		var err error
		if dir, err = config.Dir(); err != nil {
			return nil, err
		}
	}
	path := filepath.Join(dir, extensionsFile)
	data, err := os.ReadFile(path) //nolint:gosec // rhythm's own config file
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Schema     string      `json:"schema"`
		Extensions []extension `json:"extensions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Schema != extensionsSchema {
		return nil, fmt.Errorf("%s: schema %q isn't %q", path, doc.Schema, extensionsSchema)
	}
	var list []extension
	for _, x := range doc.Extensions {
		switch {
		case !extensionName.MatchString(x.Name):
			return nil, fmt.Errorf("%s: extension name %q isn't a tool name", path, x.Name)
		case !validRepo(x.Repo):
			return nil, fmt.Errorf("%s: extension %q: repo %q isn't owner/name", path, x.Name, x.Repo)
		}
		if _, registry := tools[x.Name]; !registry && x.Name != selfName {
			list = append(list, x)
		}
	}
	return list, nil
}

// extensionTargets returns the extensions found on PATH, or only those
// in want when it is non-nil, with their versions probed. missing lists
// the wanted ones that aren't on PATH.
func extensionTargets(ctx context.Context, exts []extension, want map[string]bool) (targets []target, missing []string) {
	for _, x := range exts {
		if want != nil && !want[x.Name] {
			continue
		}
		bin, err := exec.LookPath("rhythm-" + x.Name)
		if err != nil {
			if want != nil {
				missing = append(missing, x.Name)
			}
			continue
		}
		targets = append(targets, target{tool: x.Name, binary: "rhythm-" + x.Name, repo: x.Repo, path: bin, real: resolve(bin),
			extension: true, signer: x.Signer})
	}
	probeTargets(ctx, targets)
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
	p, reason := probeVersionInfo(ctx, bin)
	return p.version, reason, p.hung
}

// versionProbe is what a tool says about itself.
type versionProbe struct {
	version string // or unknownVersion
	hung    bool   // no answer within probeTimeout
	kitCLI  bool   // version --json named a kit version: the tool runs on kit/cli.Main
}

// probeVersionInfo is probeVersion with what else the answer tells;
// reason says why the version is unknown.
func probeVersionInfo(ctx context.Context, bin string) (p versionProbe, reason string) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	timedOut := func() bool { return errors.Is(ctx.Err(), context.DeadlineExceeded) }
	noAnswer := func() (versionProbe, string) {
		return versionProbe{version: unknownVersion, hung: true}, fmt.Sprintf("no answer within %s", probeTimeout)
	}

	if out, err := probeOutput(ctx, bin, "version", "--json"); err == nil {
		if v, ok := versionFromJSON(out); ok {
			return versionProbe{version: v, kitCLI: kitVersionIn(out)}, ""
		}
	}
	if timedOut() {
		return noAnswer()
	}
	out, err := probeOutput(ctx, bin, "--version")
	unknown := versionProbe{version: unknownVersion}
	switch {
	case timedOut():
		return noAnswer()
	case err != nil:
		return unknown, fmt.Sprintf("%s --version failed: %v", filepath.Base(bin), err)
	}
	if v, ok := versionFromText(out); ok {
		return versionProbe{version: v}, ""
	}
	return unknown, fmt.Sprintf("%s --version printed no version number", filepath.Base(bin))
}

// kitVersionIn reports whether a rhythm-version/v1 answer names the kit
// version the tool was built with. Only kit/cli.Main prints it. Tools
// that print it also have the update --json data report.
func kitVersionIn(out []byte) bool {
	var v struct {
		KitVersion string `json:"kit_version"`
	}
	return json.Unmarshal(out, &v) == nil && v.KitVersion != ""
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
	probeInGroup(cmd)
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
