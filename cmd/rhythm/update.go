// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

const updateSynopsis = `Usage: rhythm update [tool...] [flags]

Updates rhythm and the installed tools to their latest releases, then
refreshes each tool's data (such as Tuner's vulnerability database).
Name tools, or rhythm itself, to update only those.

Exit status: 0 when everything worked, 1 when anything failed, 2 for a
usage error and 130 when interrupted.`

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
	status := 0
	targets, missing := findTargets(want)
	for _, tool := range missing {
		fmt.Fprintf(stderr, "%-18s isn't installed; install it with: rhythm install %s\n", "rhythm-"+tool, tool)
		status = 1
	}

	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		if !update.IsRelease(t.current) {
			fmt.Fprintf(stdout, "%-18s %s (development build, not updated)\n", t.binary, t.current)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		rel, err := u.Latest(ctx)
		if err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(stderr, "%-18s %v\n", t.binary, err)
				status = 1
			}
			continue
		}
		if !update.Newer(t.current, rel.Version) {
			fmt.Fprintf(stdout, "%-18s %s (up to date)\n", t.binary, t.current)
			continue
		}
		if *checkOnly {
			fmt.Fprintf(stdout, "%-18s %s → %s available\n", t.binary, t.current, rel.Version)
			continue
		}
		if err := u.Install(ctx, rel, t.path); err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(stderr, "%-18s %v\n", t.binary, err)
				status = 1
			}
			continue
		}
		fmt.Fprintf(stdout, "%-18s %s → %s ✔\n", t.binary, t.current, rel.Version)
	}

	if !*checkOnly && !*noData {
		for _, t := range targets {
			if t.tool == selfName || ctx.Err() != nil {
				continue // rhythm has no data of its own; tools refresh theirs
			}
			cmd := exec.CommandContext(ctx, t.path, "update") //nolint:gosec // our own tool binaries
			cmd.Stdout, cmd.Stderr = stdout, stderr
			if err := cmd.Run(); err != nil {
				status = 1
			}
		}
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: update interrupted")
		return exitInterrupted
	}
	return status
}

// updateScope turns the names given to rhythm update into the set of
// targets to update; nil means everything. A non-zero code means the
// names were wrong and the update must not start.
func updateScope(names []string, selfOnly bool, stderr io.Writer) (map[string]bool, int) {
	var list []string
	for _, n := range names {
		for _, part := range strings.Split(n, ",") { // tuner,echo works as in every tool's list flags
			if part = strings.TrimSpace(part); part != "" {
				list = append(list, part)
			}
		}
	}
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

// toolVersion asks a tool binary for its version ("rhythm-tuner 0.2.0").
func toolVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // our own tool binaries
	if err != nil {
		return "unknown"
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "unknown"
	}
	return f[len(f)-1]
}
