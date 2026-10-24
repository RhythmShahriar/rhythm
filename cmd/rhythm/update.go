// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/config"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

func init() {
	register(command{name: "update", args: "[tool...] [flags]", summary: "update rhythm, installed tools and their data", run: cmdUpdate})
}

const updateSynopsis = `Usage: rhythm update [tool...] [flags]

Updates rhythm and the installed tools to their latest releases, then
refreshes each tool's data (such as Tuner's vulnerability database).
Name tools, or rhythm itself, to update only those.

--lock updates rhythm.lock instead of the installed binaries: it looks up
the latest release of everything the lock pins, prints what changed and
rewrites the lock. With --check it only prints the changes.

--min-age (or RHYTHM_MIN_RELEASE_AGE) passes over releases younger than
the age given, such as 3d, 1w or 36h, for rhythm and the tools alike.

Each update keeps the binary it replaces as <binary>.prev. --rollback
puts it back: rhythm update --rollback tuner. --to installs one version
of one tool, older ones too: rhythm update tuner --to 0.3.1.

Extensions listed in extensions.json in rhythm's config folder are
updated alongside the tools. --skip-signature and --cache work as for
rhythm install.

Exit status: 0 when everything worked, 1 when anything failed, 2 for a
usage error and 130 when interrupted. With --check: 0 when everything is
current, 10 when an update is available (or rhythm.lock is behind, with
--lock) and 1 when a check failed, so "if rhythm update --check" reads as
"if everything is current".`

// exitOutdated is rhythm update --check's exit status when at least one
// target has a newer release. It is far from the 0 to 3 that rhythm and
// its tools use, so a script can tell "behind" from "broken".
const exitOutdated = 10

// cmdUpdate updates rhythm itself, then each installed tool, then each
// tool's data (e.g. Tuner's vulnerability database).
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("update", updateSynopsis)
	checkOnly := flags.Bool("check", false, "only report available updates")
	asJSON := flags.Bool("json", false, "with --check, print the report as rhythm-update/v1 JSON")
	selfOnly := flags.Bool("self-only", false, "update only rhythm itself")
	noData := flags.Bool("no-data", false, "skip refreshing the tools' data")
	quiet := flags.Bool("quiet", false, "print only updates, problems and the tools' own output")
	toLock := flags.Bool("lock", false, "update rhythm.lock to the latest releases instead of the binaries")
	ageFlag := flags.String("min-age", "", "skip releases younger than `age`, such as 3d or 1w (default $"+minAgeEnv+")")
	back := flags.Bool("rollback", false, "put the named tools back to the version their last update replaced")
	to := flags.String("to", "", "install `version` of the one tool named, even an older one")
	trust := addTrustFlags(flags)
	names, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	age, ok := readMinAge(*ageFlag, stderr)
	if !ok {
		return exitUsage
	}
	if code := checkUpdateModes(names, *selfOnly, *back, *checkOnly, *toLock, *to, stderr); code != 0 {
		return code
	}
	switch {
	case *asJSON && !*checkOnly:
		fmt.Fprintln(stderr, "rhythm: --json works with --check only")
		return exitUsage
	case *asJSON && *toLock:
		fmt.Fprintln(stderr, "rhythm: --json doesn't work with --lock; the lock is JSON already")
		return exitUsage
	}
	exts, extErr := readExtensions()
	if extErr != nil && !*selfOnly {
		fmt.Fprintf(stderr, "rhythm: %v; extensions not updated\n", extErr)
	}
	// Names are checked before anything touches the network, so a typo
	// costs nothing.
	want, code := updateScope(names, *selfOnly, exts, stderr)
	if code != 0 {
		return code
	}
	defer beginRun(trust.options(stderr))()
	if *toLock {
		return updateLock(want, *checkOnly, age, stdout, stderr)
	}
	if *back {
		return rollbackTargets(want, exts, stdout, stderr)
	}
	// With --json, stdout carries the JSON alone; the lines people read
	// move to stderr.
	human := stdout
	if *asJSON {
		human = stderr
	}

	ctx, stop := signalContext()
	defer stop()
	var rep updateReport
	if extErr != nil && !*selfOnly {
		rep.failed = append(rep.failed, "extensions")
	}
	targets, missing := updateTargets(ctx, want, exts)
	results := make([]updateTarget, len(targets))
	for i, t := range targets {
		results[i] = updateTarget{Name: t.binary, Current: t.current, Path: t.path, Signed: signedJSON(t.real)}
		if t.managed != nil {
			results[i].ManagedBy = t.managed.name
		}
	}
	for _, tool := range missing {
		msg := "isn't installed; install it with: rhythm install " + tool
		if _, registry := toolRepo(tool); !registry {
			msg = "isn't on PATH, though " + extensionsFile + " lists it"
		}
		fmt.Fprintf(stderr, "%-18s %s\n", "rhythm-"+tool, msg)
		rep.failed = append(rep.failed, "rhythm-"+tool)
		results = append(results, updateTarget{Name: "rhythm-" + tool, Error: msg})
	}

	var progress io.Writer // the download progress line, on a terminal only
	if !*quiet && isTerminal(stderr) {
		progress = stderr
	}
	for i, t := range targets {
		if ctx.Err() != nil {
			break
		}
		res := &results[i]
		if t.managed != nil {
			fmt.Fprintf(human, "%s is managed by %s; update it with: %s\n", t.binary, t.managed.name, t.managed.upgrade)
			if *to != "" {
				rep.failed = append(rep.failed, t.binary) // --to asked for a version and didn't get it
			}
			continue
		}
		if t.current == unknownVersion {
			res.Current = ""
			res.Error = "version unknown (" + t.probe + ")"
			fmt.Fprintf(human, "%-18s %s, not updated\n", t.binary, res.Error)
			if *to != "" {
				rep.failed = append(rep.failed, t.binary)
			}
			continue
		}
		if !update.IsRelease(t.current) {
			switch {
			case *to != "":
				// The version asked for wasn't installed, so it is a failure.
				hint := ""
				if t.tool != selfName {
					hint = fmt.Sprintf("; replace it with: rhythm install %s@%s --force", t.tool, *to)
				}
				fmt.Fprintf(human, "%-18s %s (development build, not updated)%s\n", t.binary, t.current, hint)
				rep.failed = append(rep.failed, t.binary)
			case !*quiet:
				fmt.Fprintf(human, "%-18s %s (development build, not updated)\n", t.binary, t.current)
			}
			continue
		}
		if rep.nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")) {
			rep.failed = append(rep.failed, t.binary)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		if et, ok := u.HTTP.Transport.(*explainTransport); ok {
			et.signer = t.signer
		}
		rel, err := resolveSpec(ctx, u, *to, age.d)
		var tooNew *tooNewError
		switch {
		case errors.As(err, &tooNew):
			// Nothing is old enough yet: the cooldown doing its job.
			res.SkippedNewer = newSkippedJSON(&tooNew.skippedRelease)
			fmt.Fprintln(human, age.skipNote(t.binary, &tooNew.skippedRelease, "staying on "+t.current))
			rep.upToDate++
			continue
		case err != nil:
			rep.fail(ctx, stderr, t.binary, err)
			res.Error = describe(err)
			continue
		}
		res.Latest = rel.Version
		newer := update.Newer(t.current, rel.Version)
		if *to != "" && rel.Version != t.current {
			// --to goes where it is told, down a version too.
			if !newer {
				fmt.Fprintf(human, "Note: %s is older than %s %s; installing it as asked.\n", rel.Version, t.binary, t.current)
			}
			newer = true
		}
		if s := rel.skippedNewer; s != nil {
			res.SkippedNewer = newSkippedJSON(s)
			then := "staying on " + t.current
			switch {
			case newer && *checkOnly:
				then = rel.Version + " is the newest old enough"
			case newer:
				then = "updating to " + rel.Version
			}
			fmt.Fprintln(human, age.skipNote(t.binary, s, then))
		}
		if !newer {
			if !*quiet {
				fmt.Fprintf(human, "%-18s %s (up to date)\n", t.binary, t.current)
			}
			rep.upToDate++
			continue
		}
		if *checkOnly {
			res.Outdated = true
			fmt.Fprintf(human, "%-18s %s → %s available\n", t.binary, t.current, rel.Version)
			rep.updated++
			continue
		}
		if err := installWithProgress(ctx, u, rel.Release, t.real, progress); err != nil {
			rep.fail(ctx, stderr, t.binary, err)
			continue
		}
		fmt.Fprintf(human, "%-18s %s → %s ✔\n", t.binary, t.current, rel.Version)
		// The data step talks to the new binary, which may have moved to
		// kit/cli.Main; the line names the version now installed.
		p, _ := probeVersionInfo(ctx, t.path)
		targets[i].current, targets[i].kitCLI = rel.Version, p.kitCLI
		rep.updated++
	}

	if !*checkOnly && !*noData {
		for _, t := range targets {
			if t.tool == selfName || t.hung || ctx.Err() != nil {
				continue // rhythm has no data of its own; a hung tool would hang here too
			}
			if t.extension && !t.kitCLI {
				continue // a plain `update` may mean anything to a third-party tool
			}
			if _, err := refreshData(ctx, t, nil, stdout, stderr); err != nil {
				rep.failed = append(rep.failed, t.binary+" data")
			}
		}
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: update interrupted")
		return exitInterrupted
	}
	rep.nets.print(stderr)
	if *asJSON {
		for i := range results {
			if msg := rep.nets.explain(results[i].Name); msg != "" {
				results[i].Error = msg
			}
		}
		if err := writeJSON(stdout, updateReportJSON{Schema: "rhythm-update/v1", Targets: results}); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
	}
	if len(rep.failed) > 0 {
		// A failed check wins over "outdated": the answer is incomplete.
		fmt.Fprintln(stderr, rep.summary(*checkOnly))
		return 1
	}
	if !*quiet {
		announceNewTools(human)
	}
	if *checkOnly && rep.updated > 0 {
		return exitOutdated
	}
	return 0
}

// updateReportJSON is rhythm update --check --json's answer.
type updateReportJSON struct {
	Schema  string         `json:"schema"` // rhythm-update/v1
	Targets []updateTarget `json:"targets"`
}

// updateTarget is one binary in the rhythm-update/v1 report. Latest is
// empty when it wasn't looked up (dev builds, managed installs, unknown
// versions) or the lookup failed; Error then says why.
type updateTarget struct {
	Name      string `json:"name"` // rhythm or rhythm-<tool>
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Path      string `json:"path"`
	Outdated  bool   `json:"outdated"`
	Signed    string `json:"signed"`     // "yes", "no" or "" when not checked
	ManagedBy string `json:"managed_by"` // e.g. Homebrew, or ""
	Error     string `json:"error"`
	// SkippedNewer is a newer release passed over for being younger than
	// --min-age; null when none was.
	SkippedNewer *skippedJSON `json:"skipped_newer"`
}

// skippedJSON is the skipped_newer field of the rhythm-update/v1 report.
type skippedJSON struct {
	Version   string `json:"version"`
	Published string `json:"published"` // RFC 3339; "" when the API didn't say
}

func newSkippedJSON(s *skippedRelease) *skippedJSON {
	j := &skippedJSON{Version: s.version}
	if !s.published.IsZero() {
		j.Published = s.published.UTC().Format(time.RFC3339)
	}
	return j
}

// signedJSON is the signed field of the JSON reports: "" until signature
// checks land.
func signedJSON(path string) string {
	if signedStatus == nil {
		return ""
	}
	return signedStatus(path)
}

// writeJSON writes v as indented JSON with a trailing newline.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// errDataFailed is refreshData's answer after it has reported a failed
// refresh.
var errDataFailed = errors.New("data refresh failed")

// refreshData runs the tool's own update command, which refreshes its
// data (Tuner's checks and vulnerability database, for example). A tool
// on kit/cli.Main gets `update --json` and its rhythm-data/v1 report is
// printed as one line and returned; any other tool gets plain `update`
// and speaks for itself. pins, when given, ask for those pack versions
// with --pin; a tool that can't pin is warned about and refreshed to its
// latest data instead. The tool explains a failure itself; rhythm adds
// which tool it was and how it exited, so the failure isn't lost among
// the up-to-date lines.
func refreshData(ctx context.Context, t target, pins []dataPack, stdout, stderr io.Writer) ([]dataPack, error) {
	if !t.kitCLI {
		if len(pins) > 0 {
			fmt.Fprintf(stderr, "%s can't pin its data; refreshing it to the latest instead\n", t.binary)
		}
		cmd := exec.CommandContext(ctx, t.path, "update") //nolint:gosec // our own tool binaries
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return nil, dataFailure(ctx, stderr, t.binary, cmd.Run())
	}
	args := []string{"update", "--json"}
	if len(pins) > 0 {
		args = append(args, "--pin", pinArg(pins))
	}
	out, err := runDataJSON(ctx, t.path, args, stderr)
	var ee *exec.ExitError
	if len(pins) > 0 && errors.As(err, &ee) && ee.ExitCode() == exitUsage {
		// A usage error: this version of the tool has no --pin yet.
		fmt.Fprintf(stderr, "%s can't pin its data; refreshing it to the latest instead\n", t.binary)
		pins = nil
		out, err = runDataJSON(ctx, t.path, args[:2], stderr)
	}
	if err := dataFailure(ctx, stderr, t.binary, err); err != nil {
		return nil, err
	}
	packs, ok := parseDataReport(out)
	if !ok {
		fmt.Fprintf(stderr, "%s  data refresh failed (no rhythm-data/v1 report)\n", t.binary)
		return nil, errDataFailed
	}
	fmt.Fprintf(stdout, "%s %s  data: %s  ok\n", t.binary, t.current, packList(packs))
	for _, pin := range pins {
		got := "(none)"
		for _, p := range packs {
			if p.Name == pin.Name {
				got = p.Version
			}
		}
		if got != pin.Version {
			fmt.Fprintf(stderr, "%s data %s is %s, not the locked %s\n", t.binary, pin.Name, got, pin.Version)
		}
	}
	return packs, nil
}

// runDataJSON runs a tool's update --json with its progress on stderr
// and returns the report it printed, at most 1 MiB of it.
func runDataJSON(ctx context.Context, bin string, args []string, stderr io.Writer) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // our own tool binaries
	out := &capped{max: 1 << 20}
	cmd.Stdout, cmd.Stderr = out, stderr
	err := cmd.Run()
	return out.buf.Bytes(), err
}

// dataFailure reports a failed refresh and returns errDataFailed, or nil
// when err is nil or Ctrl-C caused it.
func dataFailure(ctx context.Context, stderr io.Writer, binary string, err error) error {
	if err == nil || ctx.Err() != nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		fmt.Fprintf(stderr, "%s  data refresh failed (exit %d)\n", binary, ee.ExitCode())
	} else {
		fmt.Fprintf(stderr, "%s  data refresh failed (%v)\n", binary, err)
	}
	return errDataFailed
}

// parseDataReport reads update --json's answer:
// {"schema":"rhythm-data/v1","data":[{"name","version","sha256"}]}.
func parseDataReport(out []byte) ([]dataPack, bool) {
	var r struct {
		Schema string     `json:"schema"`
		Data   []dataPack `json:"data"`
	}
	if json.Unmarshal(out, &r) != nil || r.Schema != "rhythm-data/v1" {
		return nil, false
	}
	slices.SortFunc(r.Data, func(a, b dataPack) int { return strings.Compare(a.Name, b.Name) })
	return r.Data, true
}

// pinArg is the value of update --pin: "checks=2026.10.20,vulns=2026-10-24".
func pinArg(pins []dataPack) string {
	parts := make([]string, len(pins))
	for i, p := range pins {
		parts[i] = p.Name + "=" + p.Version
	}
	return strings.Join(parts, ",")
}

// packList is "checks 2026.10.20, vulns 2026-10-24", or "none".
func packList(packs []dataPack) string {
	if len(packs) == 0 {
		return "none"
	}
	parts := make([]string, len(packs))
	for i, p := range packs {
		parts[i] = p.Name + " " + p.Version
	}
	return strings.Join(parts, ", ")
}

// firstRun reports whether a key is new on this machine and remembers
// it. Tests replace it to watch what is recorded.
var firstRun = config.FirstRun

// announceNewTools tells the user about tools a fresh setup would get
// that aren't installed here, such as a tool added in this release of
// rhythm. Each is announced once per machine. Installed tools are
// recorded as seen too, so one removed later isn't announced as new.
func announceNewTools(w io.Writer) {
	for _, tool := range defaultTools() {
		seen := !firstRun("tool-" + tool)
		if _, err := exec.LookPath("rhythm-" + tool); err == nil || seen {
			continue
		}
		fmt.Fprintf(w, "New tool available: %s (rhythm install %s)\n", tool, tool)
	}
}

// updateReport counts what rhythm update did, for the closing summary.
type updateReport struct {
	updated  int      // binaries replaced, or found outdated with --check
	upToDate int      // binaries already on the latest release
	failed   []string // "rhythm-tuner" for a binary, "rhythm-tuner data" for its data
	nets     netReport
}

// fail records a failed binary update. A network problem is held back
// and said once for every binary it hit. An error caused by Ctrl-C isn't
// a failure to report: the command ends with "interrupted" instead.
func (r *updateReport) fail(ctx context.Context, stderr io.Writer, binary string, err error) {
	if ctx.Err() != nil {
		return
	}
	if !r.nets.add(binary, err) {
		fmt.Fprintf(stderr, "%-18s %s\n", binary, describe(err))
	}
	r.failed = append(r.failed, binary)
}

// summary is the closing line printed when anything failed, e.g.
// "Updated 2, up to date 1, failed 1: rhythm-tuner data".
func (r *updateReport) summary(checkOnly bool) string {
	verb := "Updated"
	if checkOnly {
		verb = "Available"
	}
	return fmt.Sprintf("%s %d, up to date %d, failed %d: %s", verb, r.updated, r.upToDate, len(r.failed), strings.Join(r.failed, ", "))
}

// updateScope turns the names given to rhythm update into the set of
// targets to update; nil means everything. Extensions are named as
// listed. A non-zero code means the names were wrong and the update must
// not start.
func updateScope(names []string, selfOnly bool, exts []extension, stderr io.Writer) (map[string]bool, int) {
	list := splitNames(names)
	if selfOnly {
		if len(list) > 0 {
			fmt.Fprintln(stderr, "rhythm: --self-only updates rhythm alone; drop the tool names or the flag")
			return nil, exitUsage
		}
		return map[string]bool{selfName: true}, 0
	}
	if len(list) == 0 {
		return nil, 0
	}
	want := map[string]bool{}
	bad := false
	for _, n := range list {
		tool, ok := resolveName(n)
		if !ok {
			ok = slices.ContainsFunc(exts, func(x extension) bool { return x.Name == tool })
		}
		if !ok {
			fmt.Fprintf(stderr, "rhythm: unknown tool %q%s\n", n, suggest(n))
			bad = true
			continue
		}
		want[tool] = true
	}
	if bad {
		fmt.Fprintf(stderr, "  known: %s, %s\n", selfName, strings.Join(toolNames(), ", "))
		return nil, exitUsage
	}
	return want, 0
}

// updateLock is rhythm update --lock: it moves each program rhythm.lock
// pins (or those in want) to its latest release, prints the changes and,
// unless checkOnly, rewrites the lock. Data pins are kept; rhythm lock
// records new ones after an update.
func updateLock(want map[string]bool, checkOnly bool, age minAge, stdout, stderr io.Writer) int {
	path, err := findLock(".")
	if errors.Is(err, errNoLock) {
		fmt.Fprintf(stderr, "rhythm: no %s in this folder or its parents up to the repository root; write one with: rhythm lock\n", lockFileName)
		return 1
	}
	var old *lockFile
	if err == nil {
		old, err = readLock(path)
	}
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	for name := range want {
		if old.entry(name) == nil {
			fmt.Fprintf(stderr, "rhythm: %s doesn't lock %s\n", lockFileName, name)
			return 1
		}
	}
	// A lock comes with the project, so anyone can write it. Its repo
	// for rhythm or a registry tool must be the one rhythm installs from,
	// or a cloned lock could point the release lookups (and the token
	// sent with them) at a repository of its choosing.
	moved := false
	for _, e := range old.entries() {
		if repo, ok := registryRepo(e.Name); ok && !strings.EqualFold(repo, e.Repo) {
			fmt.Fprintf(stderr, "rhythm: %s says %s comes from %s; rhythm installs it from %s\n", lockFileName, e.Name, e.Repo, repo)
			moved = true
		}
	}
	if moved {
		return 1
	}
	next, err := old.clone()
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}

	ctx, stop := signalContext()
	defer stop()
	var rep updateReport
	for _, e := range next.entries() {
		if ctx.Err() != nil {
			break
		}
		if want != nil && !want[e.Name] {
			continue
		}
		binary := binaryName(e.Name)
		if rep.nets.skip(binary, os.Getenv("RHYTHM_GITHUB_API")) {
			rep.failed = append(rep.failed, binary)
			continue
		}
		u := updater(e.Repo, binary, e.Version)
		rel, err := resolveSpec(ctx, u, "", age.d)
		var tooNew *tooNewError
		switch {
		case errors.As(err, &tooNew):
			fmt.Fprintln(stderr, age.skipNote(binary, &tooNew.skippedRelease, "keeping "+e.Version))
			rep.upToDate++
			continue
		case err != nil:
			rep.fail(ctx, stderr, binary, err)
			continue
		}
		newer := update.Newer(e.Version, rel.Version)
		if s := rel.skippedNewer; s != nil {
			then := "keeping " + e.Version
			if newer {
				then = "locking " + rel.Version
			}
			fmt.Fprintln(stderr, age.skipNote(binary, s, then))
		}
		if !newer {
			rep.upToDate++
			continue
		}
		bumped, err := lockEntryFor(ctx, u, e.Name, rel.Release)
		if err != nil {
			rep.fail(ctx, stderr, binary, err)
			continue
		}
		bumped.Data, bumped.Source = e.Data, e.Source
		*e = bumped
		rep.updated++
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: update interrupted")
		return exitInterrupted
	}
	rep.nets.print(stderr)
	changes := lockDiff(old, next)
	for _, line := range changes {
		fmt.Fprintln(stdout, line)
	}
	if len(rep.failed) > 0 {
		fmt.Fprintln(stderr, rep.summary(checkOnly))
		if !checkOnly {
			fmt.Fprintf(stderr, "rhythm: %s not written\n", path)
		}
		return 1
	}
	switch {
	case len(changes) == 0:
		fmt.Fprintln(stdout, path, "is current")
		return 0
	case checkOnly:
		return exitOutdated
	}
	if err := next.write(path); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Updated", path)
	return 0
}

// Rollback. Every install over an existing binary keeps the replaced one
// as <binary>.prev, with a <binary>.prev.json sidecar saying which
// versions they are (kit/update's Install does it), so rhythm update
// --rollback can swap them back with update.Rollback.

// updateTargets is findTargets plus the extensions, after the registry
// tools.
func updateTargets(ctx context.Context, want map[string]bool, exts []extension) ([]target, []string) {
	targets, missing := findTargets(ctx, want)
	more, gone := extensionTargets(ctx, exts, want)
	return append(targets, more...), append(missing, gone...)
}

// rollbackTargets is rhythm update --rollback: it puts each named binary
// back to the version its last update replaced.
func rollbackTargets(want map[string]bool, exts []extension, stdout, stderr io.Writer) int {
	ctx, stop := signalContext()
	defer stop()
	targets, missing := updateTargets(ctx, want, exts)
	failed := len(missing) > 0
	for _, tool := range missing {
		fmt.Fprintf(stderr, "%-18s isn't installed; there is nothing to roll back\n", "rhythm-"+tool)
	}
	for _, t := range targets {
		if t.managed != nil {
			fmt.Fprintf(stderr, "%s is managed by %s; rhythm leaves its versions to %s\n", t.binary, t.managed.name, t.managed.name)
			failed = true
			continue
		}
		from, to, err := update.Rollback(t.real)
		switch {
		case errors.Is(err, update.ErrNoPrevious):
			fmt.Fprintf(stderr, "%s has no previous version to roll back to; rhythm keeps one after each update\n", t.binary)
			failed = true
		case err != nil:
			fmt.Fprintf(stderr, "%-18s %v\n", t.binary, err)
			failed = true
		case from != "" && to != "":
			fmt.Fprintf(stdout, "Rolled back %s %s → %s\n", t.binary, from, to)
		case to != "":
			fmt.Fprintf(stdout, "Rolled back %s to %s\n", t.binary, to)
		default:
			fmt.Fprintf(stdout, "Rolled back %s to the binary its last update replaced\n", t.binary)
		}
	}
	if failed {
		return 1
	}
	return 0
}

// checkUpdateModes rejects flag mixes rhythm update can't follow, before
// anything runs.
func checkUpdateModes(names []string, selfOnly, back, checkOnly, toLock bool, to string, stderr io.Writer) int {
	n := len(splitNames(names))
	if selfOnly {
		n++
	}
	bad := func(msg string) int {
		fmt.Fprintln(stderr, "rhythm:", msg)
		return exitUsage
	}
	switch {
	case back && to != "":
		return bad("--rollback and --to don't mix; pick one")
	case back && (checkOnly || toLock):
		return bad("--rollback works alone; drop --check and --lock")
	case back && n == 0:
		return bad("--rollback needs a tool name, such as: rhythm update --rollback tuner")
	case to == "":
		return 0
	case checkOnly || toLock:
		return bad("--to works alone; drop --check and --lock")
	case n != 1:
		return bad("--to needs exactly one tool name, such as: rhythm update tuner --to 0.3.1")
	}
	if err := checkSpec(to); err != nil {
		return bad("--to: " + err.Error())
	}
	return 0
}
