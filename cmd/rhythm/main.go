// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// rhythm is the umbrella CLI. `rhythm <tool> ...` runs the `rhythm-<tool>`
// binary found on PATH (same extension model as git and gh).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit"
)

var version = "dev"

// command is one of rhythm's own subcommands, as opposed to a tool.
type command struct {
	name    string // what the user types: `rhythm <name>`
	args    string // argument synopsis for help, e.g. "<tool> [--dir <dir>]"
	summary string // one line for help
	run     func(args []string, stdout, stderr io.Writer) int
	hidden  bool // left out of help, still runs
}

// commands holds every registered subcommand by name.
var commands = map[string]command{}

// register adds a subcommand. Each command file calls it from init(), so
// adding a command never means editing main.go, and help lists it.
func register(c command) {
	if _, dup := commands[c.name]; dup {
		panic("rhythm: command registered twice: " + c.name)
	}
	commands[c.name] = c
}

func init() {
	register(command{
		name: "version",
		run: func(_ []string, stdout, _ io.Writer) int {
			fmt.Fprintln(stdout, "rhythm", version)
			return 0
		},
	})
	register(command{
		name:   "help",
		hidden: true,
		run: func(_ []string, stdout, _ io.Writer) int {
			usage(stdout)
			return 0
		},
	})
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	if args[0] == "--version" {
		args[0] = "version"
	}
	if c, ok := commands[args[0]]; ok {
		return c.run(args[1:], stdout, stderr)
	}

	tool := args[0]
	if t, ok := lookupTool(tool); ok {
		tool = t.Name
	}
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		if _, known := lookupTool(tool); known {
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

// visibleCommands returns the commands help lists, sorted by name.
func visibleCommands() []command {
	var cs []command
	for _, c := range commands {
		if !c.hidden {
			cs = append(cs, c)
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].name < cs[j].name })
	return cs
}

func usage(w io.Writer) {
	fmt.Fprintln(w, kit.Banner("rhythm", version))
	fmt.Fprint(w, "Usage:\n  rhythm <tool> [args]\n\nTools:\n")
	labels := make([]string, len(registry))
	width := 0
	for i, t := range registry {
		labels[i] = t.Name
		if len(t.Aliases) > 0 {
			labels[i] += " (" + strings.Join(t.Aliases, ", ") + ")"
		}
		width = max(width, len(labels[i]))
	}
	for i, t := range registry {
		fmt.Fprintf(w, "  %-*s  %s\n", width, labels[i], t.Description)
	}

	fmt.Fprint(w, "\nCommands:\n")
	cs := visibleCommands()
	synopses := make([]string, len(cs))
	width = 0
	for i, c := range cs {
		synopses[i] = strings.TrimSpace("rhythm " + c.name + " " + c.args)
		width = max(width, len(synopses[i]))
	}
	for i, c := range cs {
		if c.summary == "" {
			fmt.Fprintf(w, "  %s\n", synopses[i])
			continue
		}
		fmt.Fprintf(w, "  %-*s   %s\n", width, synopses[i], c.summary)
	}
}
