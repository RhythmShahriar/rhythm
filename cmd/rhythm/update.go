// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

const updateSynopsis = `Usage: rhythm update [tool...] [flags]

Updates rhythm and the installed tools to their latest releases, then
refreshes each tool's data (such as Tuner's vulnerability database).
Name tools, or rhythm itself, to update only those.

Exit status: 0 when everything worked, 1 when anything failed, 2 for a
usage error and 130 when interrupted. With --check: 0 when everything is
current, 10 when an update is available and 1 when a check failed, so
"if rhythm update --check" reads as "if everything is current".`

// exitOutdated is rhythm update --check's exit status when at least one
// target has a newer release. It is far from the 0 to 3 that rhythm and
// its tools use, so a script can tell "behind" from "broken".
const exitOutdated = 10

// cmdUpdate updates rhythm itself, then each installed tool, then each
// tool's data (e.g. Tuner's vulnerability database).
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("update", updateSynopsis)
	checkOnly := flags.Bool("check", false, "only report available updates")
	selfOnly := flags.Bool("self-only", false, "update only rhythm itself")
	noData := flags.Bool("no-data", false, "skip refreshing the tools' data")
	names, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	// Names are checked before anything touches the network, so a typo
	// costs nothing.
	want, code := updateScope(names, *selfOnly, stderr)
	if code != 0 {
		return code
	}

	ctx, stop := signalContext()
	defer stop()
	var rep updateReport
	targets, missing := findTargets(ctx, want)
	for _, tool := range missing {
		fmt.Fprintf(stderr, "%-18s isn't installed; install it with: rhythm install %s\n", "rhythm-"+tool, tool)
		rep.failed = append(rep.failed, "rhythm-"+tool)
	}

	var progress io.Writer // the download progress line, on a terminal only
	if isTerminal(stderr) {
		progress = stderr
	}
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		if t.managed != nil {
			fmt.Fprintf(stdout, "%s is managed by %s; update it with: %s\n", t.binary, t.managed.name, t.managed.upgrade)
			continue
		}
		if t.current == unknownVersion {
			fmt.Fprintf(stdout, "%-18s version unknown (%s), not updated\n", t.binary, t.probe)
			continue
		}
		if !update.IsRelease(t.current) {
			fmt.Fprintf(stdout, "%-18s %s (development build, not updated)\n", t.binary, t.current)
			continue
		}
		if rep.nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")) {
			rep.failed = append(rep.failed, t.binary)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		rel, err := u.Latest(ctx)
		if err != nil {
			rep.fail(ctx, stderr, t.binary, err)
			continue
		}
		if !update.Newer(t.current, rel.Version) {
			fmt.Fprintf(stdout, "%-18s %s (up to date)\n", t.binary, t.current)
			rep.upToDate++
			continue
		}
		if *checkOnly {
			fmt.Fprintf(stdout, "%-18s %s → %s available\n", t.binary, t.current, rel.Version)
			rep.updated++
			continue
		}
		if err := installWithProgress(ctx, u, rel, t.real, progress); err != nil {
			rep.fail(ctx, stderr, t.binary, err)
			continue
		}
		fmt.Fprintf(stdout, "%-18s %s → %s ✔\n", t.binary, t.current, rel.Version)
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
	if len(rep.failed) > 0 {
		// A failed check wins over "outdated": the answer is incomplete.
		fmt.Fprintln(stderr, rep.summary(*checkOnly))
		return 1
	}
	if *checkOnly && rep.updated > 0 {
		return exitOutdated
	}
	return 0
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
