// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixedCommands swaps the command table for help and version plus a
// stand-in install for the length of a test, so the golden output below
// doesn't change whenever another file adds or rewords a command.
func fixedCommands(t *testing.T) {
	t.Helper()
	saved := commands
	t.Cleanup(func() { commands = saved })
	commands = map[string]command{
		"help":    saved["help"],
		"version": saved["version"],
		"install": {
			name: "install", args: "<tool> [--dir <dir>]", summary: "download a tool",
			run: func(args []string, stdout, stderr io.Writer) int {
				if len(args) == 1 && args[0] == "--help" {
					fmt.Fprint(stdout, "Usage: rhythm install <tool> [--dir <dir>]\n")
					return 0
				}
				fmt.Fprintln(stderr, "install ran with", args)
				return 1
			},
		},
		"secret": {name: "secret", hidden: true, run: func([]string, io.Writer, io.Writer) int { return 0 }},
	}
}

const goldenUsage = `♪ rhythm  dev

Usage:
  rhythm <tool> [args]
  rhythm <command> [args]

Tools:
  tuner (t)        WordPress & PHP security scanner
  soundcheck (sc)  did my change break anything? whole-site before/after checks, accessibility (EAA)
  echo (e)         is my site still saying what it should? email, SMS, webhooks and cookie consent
  roadie (ro)      is my staging a safe copy of production? pseudonymised copies, keys and side effects off
  sightread (sr)   what will this update really change? diff and rank before you update

Commands:
  rhythm help [tool|command]            show help for rhythm, a tool or a command
  rhythm install <tool> [--dir <dir>]   download a tool
  rhythm version                        print rhythm's version

Run 'rhythm help <tool>' for a tool's own commands and flags.

Exit codes:
  0     success
  1     findings (any other exit code from a tool also passes through)
  2     usage error or unknown tool
  3     tool not installed or can't start
  10    rhythm update --check found an update
  130   interrupted
`

// goldenWithOthers is goldenUsage with third-party tools on PATH.
var goldenWithOthers = strings.Replace(goldenUsage, "\nCommands:\n",
	"\nOther tools:\n  aaa\n  zap\n\nCommands:\n", 1)

func TestHelpGolden(t *testing.T) {
	fixedCommands(t)
	empty := t.TempDir()
	cases := []struct {
		name   string
		args   []string
		path   string
		code   int
		stdout string
		stderr string
	}{
		{"rhythm", nil, empty, 0, goldenUsage, ""},
		{"rhythm help", []string{"help"}, empty, 0, goldenUsage, ""},
		{"rhythm --help", []string{"--help"}, empty, 0, goldenUsage, ""},
		{"rhythm -h", []string{"-h"}, empty, 0, goldenUsage, ""},
		{"rhythm help help", []string{"help", "help"}, empty, 0, goldenUsage, ""},
		{"rhythm help install", []string{"help", "install"}, empty, 0, "Usage: rhythm install <tool> [--dir <dir>]\n", ""},
		{"rhythm help a b", []string{"help", "a", "b"}, empty, 2, "", "usage: rhythm help [tool|command]\n"},
		{"rhythm -v", []string{"-v"}, empty, 0, "rhythm dev\n", ""},
		{"rhythm -V", []string{"-V"}, empty, 0, "rhythm dev\n", ""},
		{"rhythm --version", []string{"--version"}, empty, 0, "rhythm dev\n", ""},
		{"rhythm version", []string{"version"}, empty, 0, "rhythm dev\n", ""},
		{"rhythm help version", []string{"help", "version"}, empty, 0, "Usage: rhythm version\n\nPrints rhythm's version.\n", ""},
		{"rhythm version extra", []string{"version", "extra"}, empty, 2, "", "usage: rhythm version\n"},
		{"rhythm tunr", []string{"tunr"}, empty, 2, "",
			"rhythm: unknown tool \"tunr\"\n  did you mean: rhythm tuner\n  run 'rhythm help' to see the tools\n"},
		{"rhythm instll", []string{"instll"}, empty, 2, "",
			"rhythm: unknown tool \"instll\"\n  did you mean: rhythm install\n  run 'rhythm help' to see the tools\n"},
		{"rhythm help tunr", []string{"help", "tunr"}, empty, 2, "",
			"rhythm: unknown tool \"tunr\"\n  did you mean: rhythm tuner\n  run 'rhythm help' to see the tools\n"},
		{"rhythm qqqqq", []string{"qqqqq"}, empty, 2, "",
			"rhythm: unknown tool \"qqqqq\"\n  run 'rhythm help' to see the tools\n"},
		{"rhythm help tuner, not installed", []string{"help", "tuner"}, empty, 3, "",
			"rhythm: tuner isn't installed\n  install it with: rhythm install tuner\n"},
		{"rhythm help ../x", []string{"help", "../x"}, empty, 2, "", "rhythm: invalid tool name \"../x\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("PATH", c.path)
			var out, errb bytes.Buffer
			code := run(c.args, &out, &errb)
			if code != c.code || out.String() != c.stdout || errb.String() != c.stderr {
				t.Errorf("exit %d (want %d)\nstdout:\n%s\nwant:\n%s\nstderr:\n%s\nwant:\n%s",
					code, c.code, out.String(), c.stdout, errb.String(), c.stderr)
			}
		})
	}
}

func TestHelpRunsToolHelp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stub(t, dir, "tuner", `echo "tuner got: $* as $RHYTHM_INVOKED_AS"`)
	for _, name := range []string{"tuner", "t"} {
		var out, errb bytes.Buffer
		code := run([]string{"help", name}, &out, &errb)
		want := "tuner got: --help as rhythm " + name + "\n"
		if code != 0 || out.String() != want || errb.Len() != 0 {
			t.Errorf("help %s: exit %d: %q %q", name, code, out.String(), errb.String())
		}
	}
}

func TestHelpListsOtherTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-ins")
	}
	fixedCommands(t)
	a, b := t.TempDir(), t.TempDir()
	t.Setenv("PATH", a+string(os.PathListSeparator)+"relative"+string(os.PathListSeparator)+b)
	stub(t, a, "zap", "exit 0")
	stub(t, b, "zap", "exit 0")   // listed once
	stub(t, b, "aaa", "exit 0")   // sorted
	stub(t, a, "tuner", "exit 0") // a known tool isn't "other"
	for _, name := range []string{"rhythm-notexec", "rhythm-Bad.Name"} {
		if err := os.WriteFile(filepath.Join(a, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(b, "rhythm-adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	usage(&out)
	if out.String() != goldenWithOthers {
		t.Errorf("usage:\n%s\nwant:\n%s", out.String(), goldenWithOthers)
	}

	// A third-party tool is suggested too.
	var errb bytes.Buffer
	if code := run([]string{"zapp"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "did you mean: rhythm zap\n") {
		t.Errorf("exit %d: %s", code, errb.String())
	}
}

// TestHelpListsEveryCommand checks the real command table, whatever
// files registered it: each visible command is listed, hidden ones
// aren't and `rhythm help <command>` is the command's own --help.
func TestHelpListsEveryCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	usage(&out)
	for _, c := range commands {
		listed := strings.Contains(out.String(), "\n  rhythm "+c.name)
		if listed == c.hidden {
			t.Errorf("command %q: listed=%v hidden=%v", c.name, listed, c.hidden)
		}
	}
	for name := range maps.Keys(commands) {
		if name == "help" {
			continue
		}
		var viaHelp, viaFlag, e1, e2 bytes.Buffer
		c1 := run([]string{"help", name}, &viaHelp, &e1)
		c2 := run([]string{name, "--help"}, &viaFlag, &e2)
		if c1 != c2 || viaHelp.String() != viaFlag.String() || e1.String() != e2.String() {
			t.Errorf("rhythm help %s differs from rhythm %s --help", name, name)
		}
	}
}

func TestSuggestionsOrder(t *testing.T) {
	fixedCommands(t)
	t.Setenv("PATH", t.TempDir())
	got := strings.Join(suggestions(), " ")
	want := "echo roadie sightread soundcheck tuner help install version t sc e ro sr"
	if got != want {
		t.Errorf("suggestions() = %q, want %q", got, want)
	}
}
