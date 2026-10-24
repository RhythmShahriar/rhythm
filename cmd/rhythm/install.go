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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

func updater(repo, binary, current string) *update.Updater {
	return &update.Updater{
		Repo:      repo,
		Binary:    binary,
		Current:   current,
		UserAgent: fmt.Sprintf("rhythm/%s (+https://rythm.sh)", version),
		API:       os.Getenv("RHYTHM_GITHUB_API"), // a mirror or GitHub Enterprise; default api.github.com
	}
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "install directory (default: next to rhythm)")
	pos, err := parseInterspersed(flags, args)
	if err != nil || len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
		return 2
	}
	tool := pos[0]
	if full, ok := aliases[tool]; ok {
		tool = full
	}
	repo, ok := tools[tool]
	if !ok {
		fmt.Fprintf(stderr, "rhythm: unknown tool %q; known tools: %s\n", tool, strings.Join(toolNames(), ", "))
		return 2
	}
	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		*dir = filepath.Dir(self)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	u := updater(repo, "rhythm-"+tool, "")
	rel, err := u.Latest(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	dest := filepath.Join(*dir, exe("rhythm-"+tool))
	if err := u.Install(ctx, rel, dest); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Installed rhythm-%s %s to %s\n", tool, rel.Version, dest)
	switch found, err := exec.LookPath("rhythm-" + tool); {
	case err != nil:
		fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", *dir)
	case found != dest:
		fmt.Fprintf(stdout, "Note: %s comes first on your PATH; remove it to use this install.\n", found)
	}
	return 0
}
