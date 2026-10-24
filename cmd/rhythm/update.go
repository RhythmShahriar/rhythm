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
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/config"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

const updateSynopsis = `Usage: rhythm update [tool...] [flags]

Updates rhythm and the installed tools to their latest releases, then
refreshes each tool's data (such as Tuner's vulnerability database).
Name tools, or rhythm itself, to update only those.

--lock updates rhythm.lock instead of the installed binaries: it looks up
the latest release of everything the lock pins, prints what changed and
rewrites the lock. With --check it only prints the changes.

--min-age (or RHYTHM_MIN_RELEASE_AGE) passes over releases younger than
the age given, such as 3d, 1w or 36h, for rhythm and the tools alike.

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
	names, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	age, ok := readMinAge(*ageFlag, stderr)
	if !ok {
		return exitUsage
	}
	switch {
	case *asJSON && !*checkOnly:
		fmt.Fprintln(stderr, "rhythm: --json works with --check only")
		return exitUsage
	case *asJSON && *toLock:
		fmt.Fprintln(stderr, "rhythm: --json doesn't work with --lock; the lock is JSON already")
		return exitUsage
	}
	// Names are checked before anything touches the network, so a typo
	// costs nothing.
	want, code := updateScope(names, *selfOnly, stderr)
	if code != 0 {
		return code
	}
	if *toLock {
		return updateLock(want, *checkOnly, age, stdout, stderr)
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
	targets, missing := findTargets(ctx, want)
	results := make([]updateTarget, len(targets))
	for i, t := range targets {
		results[i] = updateTarget{Name: t.binary, Current: t.current, Path: t.path, Signed: signedJSON(t.real)}
		if t.managed != nil {
			results[i].ManagedBy = t.managed.name
		}
	}
	for _, tool := range missing {
		msg := "isn't installed; install it with: rhythm install " + tool
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
			continue
		}
		if t.current == unknownVersion {
			res.Current = ""
			res.Error = "version unknown (" + t.probe + ")"
			fmt.Fprintf(human, "%-18s %s, not updated\n", t.binary, res.Error)
			continue
		}
		if !update.IsRelease(t.current) {
			if !*quiet {
				fmt.Fprintf(human, "%-18s %s (development build, not updated)\n", t.binary, t.current)
			}
			continue
		}
		if rep.nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")) {
			rep.failed = append(rep.failed, t.binary)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		rel, err := resolveSpec(ctx, u, "", age.d)
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
		rep.updated++
	}

	if !*checkOnly && !*noData {
		for _, t := range targets {
			if t.tool == selfName || t.hung || ctx.Err() != nil {
				continue // rhythm has no data of its own; a hung tool would hang here too
			}
			refreshData(ctx, t, stdout, stderr, &rep)
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

// refreshData runs the tool's own update command, which refreshes its
// data (Tuner's checks and vulnerability database, for example). The
// tool explains a failure itself; rhythm adds which tool it was and how
// it exited, so the failure isn't lost among the up-to-date lines.
func refreshData(ctx context.Context, t target, stdout, stderr io.Writer, rep *updateReport) {
	cmd := exec.CommandContext(ctx, t.path, "update") //nolint:gosec // our own tool binaries
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if err == nil || ctx.Err() != nil {
		return
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		fmt.Fprintf(stderr, "%s  data refresh failed (exit %d)\n", t.binary, ee.ExitCode())
	} else {
		fmt.Fprintf(stderr, "%s  data refresh failed (%v)\n", t.binary, err)
	}
	rep.failed = append(rep.failed, t.binary+" data")
}

// firstRun reports whether a key is new on this machine and remembers
// it. Tests replace it to watch what is recorded.
var firstRun = config.FirstRun

// announceNewTools tells the user about tools a fresh setup would get
// that aren't installed here, such as a tool added in this release of
// rhythm. Each is announced once per machine. Installed tools are
// recorded as seen too, so one removed later isn't announced as new.
func announceNewTools(w io.Writer) {
	for _, tool := range defaultInstallTools() {
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
// targets to update; nil means everything. A non-zero code means the
// names were wrong and the update must not start.
func updateScope(names []string, selfOnly bool, stderr io.Writer) (map[string]bool, int) {
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
