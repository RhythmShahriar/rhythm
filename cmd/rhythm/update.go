// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// cmdUpdate updates rhythm itself, then each installed tool, then each
// tool's data (e.g. Tuner's vulnerability database).
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "only report available updates")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	status := 0

	type target struct {
		name, repo, binary, current, path string
	}
	var targets []target
	if self, err := selfPath(); err == nil {
		targets = append(targets, target{"rhythm", "RhythmShahriar/rhythm", "rhythm", version, self})
	}
	for _, tool := range toolNames() {
		bin, err := exec.LookPath("rhythm-" + tool)
		if err != nil {
			continue
		}
		targets = append(targets, target{"rhythm-" + tool, tools[tool], "rhythm-" + tool, toolVersion(bin), bin})
	}

	for _, t := range targets {
		if !update.IsRelease(t.current) {
			fmt.Fprintf(stdout, "%-18s %s (development build, not updated)\n", t.name, t.current)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		rel, err := u.Latest(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "%-18s %v\n", t.name, err)
			status = 1
			continue
		}
		if !update.Newer(t.current, rel.Version) {
			fmt.Fprintf(stdout, "%-18s %s (up to date)\n", t.name, t.current)
			continue
		}
		if *checkOnly {
			fmt.Fprintf(stdout, "%-18s %s → %s available\n", t.name, t.current, rel.Version)
			continue
		}
		if err := u.Install(ctx, rel, t.path); err != nil {
			fmt.Fprintf(stderr, "%-18s %v\n", t.name, err)
			status = 1
			continue
		}
		fmt.Fprintf(stdout, "%-18s %s → %s ✔\n", t.name, t.current, rel.Version)
	}

	if !*checkOnly {
		for _, t := range targets {
			if t.name == "rhythm" {
				continue // rhythm has no data of its own; tools refresh theirs
			}
			cmd := exec.CommandContext(ctx, t.path, "update") //nolint:gosec // our own tool binaries
			cmd.Stdout, cmd.Stderr = stdout, stderr
			if err := cmd.Run(); err != nil {
				status = 1
			}
		}
	}
	return status
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
