// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Exit codes of the router's own commands. They match the ExitUsage and
// ExitInterrupted that kit/cli is getting, so rhythm and its tools agree.
const (
	exitUsage       = 2
	exitInterrupted = 130
)

// signalContext is the context install and update run under, ended
// by Ctrl-C or SIGTERM. Tests replace it to simulate an interrupt.
var signalContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// newFlagSet returns a flag set whose usage prints synopsis once and then
// every flag with a double dash, the way rhythm's help writes them.
func newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, synopsis)
		fmt.Fprintln(w, "\nFlags:")
		fs.VisitAll(func(f *flag.Flag) {
			arg, usage := flag.UnquoteUsage(f)
			name := "--" + f.Name
			if arg != "" {
				name += " " + arg
			}
			if f.DefValue != "" && f.DefValue != "false" {
				usage += fmt.Sprintf(" (default %s)", f.DefValue)
			}
			fmt.Fprintf(w, "  %-16s %s\n", name, usage)
		})
	}
	return fs
}

// parseArgs parses flags before or after positional arguments. When ok is
// false the caller returns code: 0 after --help (usage on stdout) or
// exitUsage after a bad flag (error and usage on stderr).
func parseArgs(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (pos []string, code int, ok bool) {
	// The flag package prints the usage itself; catch it so --help can go
	// to stdout and errors to stderr.
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	pos, err := parseInterspersed(fs, args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		_, _ = stdout.Write(buf.Bytes())
		return nil, 0, false
	case err != nil:
		_, _ = stderr.Write(buf.Bytes())
		return nil, exitUsage, false
	}
	return pos, 0, true
}

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

const installSynopsis = `Usage: rhythm install <tool> [--dir <dir>]

Downloads the latest release of a tool and installs it next to rhythm.

Exit status: 0 when the tool was installed, 1 when it failed, 2 for a
usage error and 130 when interrupted.`

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("install", installSynopsis)
	dir := flags.String("dir", "", "install into `dir` (default: next to rhythm)")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
		return exitUsage
	}
	tool, ok := resolveName(pos[0])
	if !ok || tool == selfName {
		fmt.Fprintf(stderr, "rhythm: unknown tool %q%s; known tools: %s\n", pos[0], suggest(pos[0]), strings.Join(toolNames(), ", "))
		return exitUsage
	}
	repo := tools[tool]
	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		*dir = filepath.Dir(self)
	}

	ctx, stop := signalContext()
	defer stop()
	u := updater(repo, "rhythm-"+tool, "")
	rel, err := u.Latest(ctx)
	if err != nil {
		return installFailed(ctx, stderr, err)
	}
	dest := filepath.Join(*dir, exe("rhythm-"+tool))
	if err := u.Install(ctx, rel, dest); err != nil {
		return installFailed(ctx, stderr, err)
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

// installFailed reports err and returns the exit code: exitInterrupted
// when the user pressed Ctrl-C, 1 otherwise.
func installFailed(ctx context.Context, stderr io.Writer, err error) int {
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: install interrupted")
		return exitInterrupted
	}
	fmt.Fprintln(stderr, "rhythm:", err)
	return 1
}
