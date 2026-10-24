// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package cli holds the small command-line helpers every Rhythm tool
// shares, so flags behave the same in all of them.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// ParseInterspersed lets flags come before or after positional arguments:
// `tool scan site.com --json` and `tool scan --json site.com` are the same.
// Everything after a "--" is positional, even if it looks like a flag.
func ParseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		// fs.Parse stops at "--" and drops it, which leaves no trace in
		// fs.Args(); look at the argument just before the rest instead.
		if used := len(args) - fs.NArg(); used > 0 && args[used-1] == "--" {
			return append(pos, fs.Args()...), nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// ListFlag collects comma-separated values across repeated flags.
type ListFlag []string

func (l *ListFlag) String() string { return strings.Join(*l, ",") }

func (l *ListFlag) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*l = append(*l, p)
		}
	}
	return nil
}

// RepeatFlag collects each use of a flag verbatim (e.g. -H "Name: value").
type RepeatFlag []string

func (r *RepeatFlag) String() string     { return strings.Join(*r, "; ") }
func (r *RepeatFlag) Set(v string) error { *r = append(*r, v); return nil }

// UsageError marks an error caused by how the tool was called (exit 2),
// as opposed to one that happened while it worked.
type UsageError struct {
	Msg string
	// Reported means the message has already been printed (ParseArgs
	// sets it, since the flag package prints its own errors), so Main
	// doesn't print it twice.
	Reported bool
}

func (e UsageError) Error() string { return e.Msg }

// Usagef returns a UsageError.
func Usagef(format string, a ...any) error { return UsageError{Msg: fmt.Sprintf(format, a...)} }

// IsTerminal reports whether w is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

// Exit codes shared by rhythm and every tool, so scripts and CI can tell
// "the tool found something" apart from "the tool could not run".
const (
	ExitOK          = 0   // finished, nothing to report
	ExitFindings    = 1   // finished and found something the user asked to fail on
	ExitUsage       = 2   // bad flags, arguments or an unknown name
	ExitUnavailable = 3   // a tool, site or service needed for the run isn't there
	ExitInterrupted = 130 // stopped by Ctrl-C or a signal (128 + SIGINT)
)
