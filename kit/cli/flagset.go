// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// ErrHelp is what ParseArgs returns for -h or --help, after the usage
// has been printed. Callers exit 0 on it: asking for help isn't an error.
var ErrHelp = flag.ErrHelp

// NewFlagSet returns a flag set for one command that reports errors
// instead of exiting and whose usage prints synopsis once followed by
// the flags in the double-dash form users type (--dir, not Go's -dir).
// synopsis is the whole text above the flags, usually a "Usage:" line and
// a short description; a missing final newline is added.
func NewFlagSet(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { PrintUsage(fs.Output(), synopsis, fs) }
	return fs
}

// PrintUsage writes synopsis and then a "Flags:" block listing every flag
// in fs as --name, with its argument type and any non-empty default.
func PrintUsage(w io.Writer, synopsis string, fs *flag.FlagSet) {
	fmt.Fprint(w, synopsis)
	if !strings.HasSuffix(synopsis, "\n") {
		fmt.Fprintln(w)
	}
	type row struct{ left, usage string }
	var rows []row
	width := 0
	fs.VisitAll(func(f *flag.Flag) {
		arg, usage := flag.UnquoteUsage(f)
		left := "--" + f.Name
		if arg != "" {
			left += " " + arg
		}
		if showDefault(f) {
			usage += fmt.Sprintf(" (default %s)", f.DefValue)
		}
		rows = append(rows, row{left, usage})
		width = max(width, len(left))
	})
	if len(rows) == 0 {
		return
	}
	fmt.Fprint(w, "\nFlags:\n")
	for _, r := range rows {
		fmt.Fprintf(w, "  %-*s   %s\n", width, r.left, strings.TrimSpace(r.usage))
	}
}

// showDefault reports whether a flag's default is worth printing. The
// flag package's own test for a zero value is unexported, so this covers
// the values its built-in flag types and ListFlag start with.
func showDefault(f *flag.Flag) bool {
	switch f.DefValue {
	case "", "false", "0", "0s", "[]":
		return false
	}
	return true
}

// ParseArgs parses args with fs, letting flags come before or after
// positional arguments (see ParseInterspersed). It returns ErrHelp for -h
// and --help and wraps any other flag error in a UsageError so callers
// can exit 2. The flag package has already printed the error and the
// usage to fs.Output() by then, so the UsageError is marked Reported and
// callers shouldn't print it again.
func ParseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	pos, err := ParseInterspersed(fs, args)
	switch {
	case err == nil:
		return pos, nil
	case errors.Is(err, flag.ErrHelp):
		return nil, ErrHelp
	}
	return nil, UsageError{Msg: err.Error(), Reported: true}
}
