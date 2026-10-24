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

const installSynopsis = `Usage: rhythm install <tool>... [flags]

Downloads the latest release of each tool and installs it next to rhythm.
Separate names with spaces or commas; "none" installs nothing. A tool
that fails doesn't stop the others.

Exit status: 0 when every tool was installed, 1 when any failed, 2 for a
usage error and 130 when interrupted.`

// installOptions are the rhythm install flags that shape each tool's
// install.
type installOptions struct {
	dir   string // absolute install folder
	quiet bool
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("install", installSynopsis)
	dir := flags.String("dir", "", "install into `dir` (default: next to rhythm)")
	quiet := flags.Bool("quiet", false, "print only the Installed lines and errors")
	dryRun := flags.Bool("dry-run", false, "check the names and print the tools, one per line, without installing")
	defaults := flags.Bool("defaults", false, "also install the tools a fresh setup gets")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	// Every name is checked before anything is downloaded, so a typo in
	// the third name doesn't leave the first two installed.
	list, code := installScope(pos, *defaults, stderr)
	if code != 0 {
		return code
	}
	if *dryRun {
		for _, tool := range list {
			fmt.Fprintln(stdout, tool)
		}
		return 0
	}
	if len(list) == 0 {
		if !*quiet {
			fmt.Fprintln(stdout, "Nothing to install.")
		}
		return 0
	}

	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		*dir = filepath.Dir(self)
	}
	// An absolute path, so the PATH note compares like with like:
	// LookPath answers with absolute paths.
	abs, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	// A folder that can't take the binaries should fail now, not after a
	// 30 MB download.
	if err := prepareDir(abs, abs); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	opt := installOptions{dir: abs, quiet: *quiet}

	ctx, stop := signalContext()
	defer stop()
	var installed, failed []string
	offPath := false
	for _, tool := range list {
		note, err := installTool(ctx, tool, opt, stdout)
		if ctx.Err() != nil {
			fmt.Fprintln(stderr, "rhythm: install interrupted")
			return exitInterrupted
		}
		binary := "rhythm-" + tool
		var me *managedError
		switch {
		case errors.As(err, &me):
			fmt.Fprintln(stderr, err)
		case err != nil:
			fmt.Fprintf(stderr, "%-18s %v\n", binary, err)
		}
		if err != nil {
			failed = append(failed, binary)
			continue
		}
		installed = append(installed, binary)
		switch {
		case opt.quiet || note == "":
		case note == noteOffPath:
			offPath = true // said once below, not once per tool
		default:
			fmt.Fprintln(stdout, note)
		}
	}
	if offPath {
		fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", opt.dir)
	}
	if len(failed) > 0 {
		if len(list) > 1 {
			fmt.Fprintf(stderr, "Installed %d, failed %d: %s\n", len(installed), len(failed), strings.Join(failed, ", "))
		}
		return 1
	}
	return 0
}

// installScope turns the names given to rhythm install into the tools to
// install, in the order given and without repeats; --defaults appends the
// tools a fresh setup gets. A non-zero code means a name was wrong.
func installScope(names []string, defaults bool, stderr io.Writer) ([]string, int) {
	list := splitNames(names)
	none := false
	var tools []string
	seen := map[string]bool{}
	add := func(tool string) {
		if !seen[tool] {
			seen[tool] = true
			tools = append(tools, tool)
		}
	}
	bad := false
	for _, n := range list {
		if strings.EqualFold(n, "none") {
			none = true // the installers pass RHYTHM_TOOLS=none through as is
			continue
		}
		tool, ok := resolveName(n)
		switch {
		case ok && tool == selfName:
			fmt.Fprintln(stderr, "rhythm: rhythm installs tools, not itself; update it with: rhythm update --self-only")
			bad = true
		case !ok:
			fmt.Fprintf(stderr, "rhythm: unknown tool %q%s\n", n, suggest(n))
			bad = true
		default:
			add(tool)
		}
	}
	if bad {
		fmt.Fprintf(stderr, "  known: %s\n", strings.Join(toolNames(), ", "))
		return nil, exitUsage
	}
	if defaults {
		for _, tool := range defaultInstallTools() {
			add(tool)
		}
	}
	if len(tools) == 0 && !none && !defaults {
		fmt.Fprintln(stderr, "usage: rhythm install <tool>... [--dir <dir>]; name a tool, or use --defaults")
		return nil, exitUsage
	}
	return tools, 0
}

// defaultInstallTools returns the tools a fresh setup gets. Every tool
// in the registry is one today; router-core's registry marks them with
// DefaultInstall; this then asks it.
func defaultInstallTools() []string { return toolNames() }

// managedError refuses to install over a package manager's binary.
type managedError struct {
	binary string
	m      *manager
}

func (e *managedError) Error() string {
	return fmt.Sprintf("%s is managed by %s; update it with: %s", e.binary, e.m.name, e.m.upgrade)
}

// noteOffPath is installTool's note for a folder that isn't on PATH.
// cmdInstall prints it once for all tools.
const noteOffPath = "off-path"

// installTool downloads tool's latest release into opt.dir and prints the
// Installed line. note is the PATH note for the new binary, if any.
func installTool(ctx context.Context, tool string, opt installOptions, stdout io.Writer) (note string, err error) {
	// An existing symlink is followed, so a link into a tool folder stays
	// a link; a package manager's install is left to the manager.
	binary := "rhythm-" + tool
	dest := filepath.Join(opt.dir, exe(binary))
	writeTo := resolve(dest)
	if m := managedBy(writeTo, binary); m != nil {
		return "", &managedError{binary, m}
	}
	if target := filepath.Dir(writeTo); target != opt.dir {
		if err := prepareDir(opt.dir, target); err != nil {
			return "", err
		}
	}
	u := updater(tools[tool], binary, "")
	rel, err := u.Latest(ctx)
	if err != nil {
		return "", err
	}
	if err := u.Install(ctx, rel, writeTo); err != nil {
		return "", err
	}
	fmt.Fprintf(stdout, "Installed %s %s to %s\n", binary, rel.Version, dest)
	return pathNote(binary, dest), nil
}

// prepareDir makes sure the install folder exists and that the binary
// can be written into target, the folder it really lands in (they differ
// when the destination is a symlink). A missing folder is created.
func prepareDir(dir, target string) error {
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		return fmt.Errorf("%s is a file, not a folder; pick a folder with --dir", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a bin folder others may run from
		return fmt.Errorf("can't create %s: %w", dir, err)
	}
	// Creating a file is the only check that holds everywhere: mode bits
	// say nothing about ACLs, read-only mounts or Windows.
	f, err := os.CreateTemp(target, ".rhythm-write-test-*")
	if err != nil {
		return fmt.Errorf("can't write to %s; rerun with sudo, or install to a folder you own with --dir", target)
	}
	_ = f.Close()
	_ = os.Remove(f.Name()) //nolint:gosec // the probe file CreateTemp just made
	return nil
}

// pathNote explains why running binary may not reach the copy just
// installed at dest, or returns "" when it will. A binary found nowhere
// on PATH gives noteOffPath. Both sides have their
// symlinks resolved, so a link on PATH that points at dest counts as dest.
func pathNote(binary, dest string) string {
	found, err := exec.LookPath(binary)
	if errors.Is(err, exec.ErrDot) {
		err = nil // found in the current folder through "." on PATH; still a real shadow
	}
	if err != nil {
		return noteOffPath
	}
	if abs, aerr := filepath.Abs(found); aerr == nil {
		found = abs
	}
	if found == dest || resolve(found) == resolve(dest) {
		return ""
	}
	return fmt.Sprintf("Note: %s comes first on your PATH; remove it to use this install.", found)
}
