// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit"
	"github.com/RhythmShahriar/rhythm/kit/cli"
)

func init() {
	register(command{
		name:    "help",
		args:    "[tool|command]",
		summary: "show help for rhythm, a tool or a command",
		run:     cmdHelp,
	})
	register(command{
		name:    "version",
		summary: "print rhythm's version",
		run:     cmdVersion,
	})
}

// versionFlags are the spellings that mean `rhythm version`. -v is
// included because that is what people try first; rhythm itself has no
// verbose mode for it to clash with.
var versionFlags = []string{"-v", "-V", "--version"}

// cmdHelp prints rhythm's usage, or the help of one tool or command.
// A tool's help comes from the tool itself (`rhythm-<tool> --help`), run
// through the same validated lookup as `rhythm <tool>`, so a typo gets
// the same message and suggestion.
func cmdHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return cli.ExitOK
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: rhythm help [tool|command]")
		return cli.ExitUsage
	}
	name := args[0]
	if name == "help" {
		usage(stdout)
		return cli.ExitOK
	}
	if c, ok := commands[name]; ok {
		return c.run([]string{"--help"}, stdout, stderr)
	}
	return run([]string{name, "--help"}, stdout, stderr)
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	fs := cli.NewFlagSet("version", "Usage: rhythm version\n\nPrints rhythm's version.", stdout)
	pos, err := cli.ParseArgs(fs, args)
	switch {
	case errors.Is(err, cli.ErrHelp):
		return cli.ExitOK
	case err != nil:
		return cli.ExitUsage
	case len(pos) > 0:
		fmt.Fprintln(stderr, "usage: rhythm version")
		return cli.ExitUsage
	}
	fmt.Fprintln(stdout, "rhythm", version)
	return cli.ExitOK
}

// unknownTool reports a name that is neither a command, a known tool nor
// a rhythm-<name> on PATH, with a suggestion when one is close.
func unknownTool(stderr io.Writer, name string) int {
	fmt.Fprintf(stderr, "rhythm: unknown tool %q\n", name)
	if s := cli.Suggest(name, suggestions()); s != "" {
		fmt.Fprintf(stderr, "  did you mean: rhythm %s\n", s)
	}
	fmt.Fprintln(stderr, "  run 'rhythm help' to see the tools")
	return cli.ExitUsage
}

// suggestions lists every name `rhythm <name>` accepts, best first: Suggest
// breaks ties by order, so a full tool name wins over a command and both
// win over a short alias.
func suggestions() []string {
	names := slices.Clone(toolNames())
	for _, c := range visibleCommands() {
		names = append(names, c.name)
	}
	for _, t := range registry {
		names = append(names, t.Aliases...)
	}
	return append(names, otherTools()...)
}

// otherTools returns the names of rhythm-<name> programs on PATH that
// rhythm doesn't know, sorted: third-party tools `rhythm <name>` will run.
// Like exec.LookPath, it skips relative PATH entries.
func otherTools() []string {
	var names []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := strings.CutPrefix(e.Name(), "rhythm-")
			if !ok {
				continue
			}
			if name, ok = trimExecExt(name); !ok || !validToolName(name) {
				continue
			}
			if _, known := lookupTool(name); known || slices.Contains(names, name) {
				continue
			}
			if isExecutable(filepath.Join(dir, e.Name())) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names
}

// trimExecExt strips the extension Windows needs to run a file (from
// PATHEXT) and reports whether the name is runnable as `rhythm-<name>`.
// Elsewhere names are used as they are.
func trimExecExt(name string) (string, bool) {
	if runtime.GOOS != "windows" {
		return name, true
	}
	exts := os.Getenv("PATHEXT")
	if exts == "" {
		exts = ".com;.exe;.bat;.cmd"
	}
	ext := filepath.Ext(name)
	for _, e := range strings.Split(exts, ";") {
		if ext != "" && strings.EqualFold(ext, e) {
			return name[:len(name)-len(ext)], true
		}
	}
	return "", false
}

// isExecutable reports whether path is a file that can be run, following
// symlinks. Windows has no execute bit; the extension decided that.
func isExecutable(path string) bool {
	fi, err := os.Stat(path) //nolint:gosec // a file in a PATH folder, only stat-ed to list it
	if err != nil || fi.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || fi.Mode()&0o111 != 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, kit.Banner("rhythm", version))
	fmt.Fprint(w, "\nUsage:\n  rhythm <tool> [args]\n  rhythm <command> [args]\n\nTools:\n")
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
		desc := t.Description
		if needsNewerRhythm(t) {
			desc += fmt.Sprintf(" (needs rhythm >= %s)", t.MinRhythm)
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, labels[i], desc)
	}

	if others := otherTools(); len(others) > 0 {
		fmt.Fprint(w, "\nOther tools:\n")
		for _, name := range others {
			fmt.Fprintf(w, "  %s\n", name)
		}
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

	fmt.Fprint(w, `
Run 'rhythm help <tool>' for a tool's own commands and flags.

Exit codes:
  0     success
  1     findings (any other exit code from a tool also passes through)
  2     usage error or unknown tool
  3     tool not installed or can't start
  130   interrupted
`)
}
