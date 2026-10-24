// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// rhythm is the umbrella CLI. `rhythm <tool> ...` runs the `rhythm-<tool>`
// binary found on PATH (same extension model as git and gh).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"

	"github.com/RhythmShahriar/rhythm/kit"
)

var version = "dev"

// tools maps each tool to its GitHub repository.
var tools = map[string]string{
	"tuner":      "RhythmShahriar/tuner",
	"soundcheck": "RhythmShahriar/soundcheck",
	"echo":       "RhythmShahriar/echo",
}

// aliases are short names for tools.
var aliases = map[string]string{"t": "tuner", "sc": "soundcheck", "e": "echo"}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(stdout)
		return 0
	}
	switch args[0] {
	case "--version", "version":
		fmt.Fprintln(stdout, "rhythm", version)
		return 0
	case "update":
		return cmdUpdate(args[1:], stdout, stderr)
	case "install":
		return cmdInstall(args[1:], stdout, stderr)
	}

	tool := args[0]
	if full, ok := aliases[tool]; ok {
		tool = full
	}
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		if _, known := tools[tool]; known {
			fmt.Fprintf(stderr, "rhythm: %s isn't installed\n  install it with: rhythm install %s\n", tool, tool)
		} else {
			fmt.Fprintf(stderr, "rhythm: unknown tool %q\n  run 'rhythm help' to see the tools\n", tool)
		}
		return 1
	}

	cmd := exec.Command(bin, args[1:]...) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, kit.Banner("rhythm", version))
	fmt.Fprint(w, `Usage:
  rhythm <tool> [args]

Tools:
  tuner (t)        WordPress & PHP security scanner
  soundcheck (sc)  did my change break anything? whole-site before/after checks
  echo (e)         is my site still saying what it should? every email, SMS and webhook

Commands:
  rhythm install <tool> [--dir <dir>]   download a tool
  rhythm update [--check]               update rhythm, installed tools and their data
  rhythm version
`)
}

// parseInterspersed lets flags come before or after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func toolNames() []string {
	var names []string
	for t := range tools {
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}
