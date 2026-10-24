// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/config"
)

func init() {
	addRouterCommand(routerCommand{name: "uninstall", args: "<tool>... [--purge] [--all]", summary: "remove tools (with --purge, their data too)", run: cmdUninstall})
}

const uninstallSynopsis = `Usage: rhythm uninstall <tool>... [flags]

Removes each tool's binary from where it is found on PATH, with the
.prev and .old copies updates leave next to it. A tool installed by a
package manager is left to that manager. Separate names with spaces or
commas. rhythm itself is removed only when named together with --all.

Exit status: 0 when every tool was removed or wasn't installed, 1 when
any couldn't be removed and 2 for a usage error.`

func cmdUninstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("uninstall", uninstallSynopsis)
	purge := flags.Bool("purge", false, "also remove each tool's cache and update-check state")
	all := flags.Bool("all", false, "remove every installed tool (and rhythm, when named too)")
	names, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	list, code := uninstallScope(names, *all, stderr)
	if code != 0 {
		return code
	}

	var removed, failed []string
	for _, tool := range list {
		switch err := uninstallOne(tool, *purge, *all, stdout, stderr); {
		case errors.Is(err, errNotInstalled):
		case err != nil:
			failed = append(failed, binaryName(tool))
		default:
			removed = append(removed, binaryName(tool))
		}
	}
	if len(failed) > 0 {
		if len(list) > 1 {
			fmt.Fprintf(stderr, "Removed %d, failed %d: %s\n", len(removed), len(failed), strings.Join(failed, ", "))
		}
		return 1
	}
	return 0
}

// binaryName is the program a tool installs: rhythm-<tool>, or rhythm.
func binaryName(tool string) string {
	if tool == selfName {
		return selfName
	}
	return "rhythm-" + tool
}

// uninstallScope turns the names given to rhythm uninstall into the tools
// to remove, in the order given; --all adds every registry tool. rhythm
// itself goes last, so a failure on a tool doesn't leave rhythm gone and
// the tool still there.
func uninstallScope(names []string, all bool, stderr io.Writer) ([]string, int) {
	var list []string
	seen := map[string]bool{}
	self, bad := false, false
	for _, n := range splitNames(names) {
		tool, ok := resolveName(n)
		switch {
		case !ok:
			fmt.Fprintf(stderr, "rhythm: unknown tool %q%s\n", n, suggest(n))
			bad = true
		case tool == selfName:
			self = true
		case !seen[tool]:
			seen[tool] = true
			list = append(list, tool)
		}
	}
	if bad {
		fmt.Fprintf(stderr, "  known: %s, %s\n", selfName, strings.Join(toolNames(), ", "))
		return nil, exitUsage
	}
	if self && !all {
		// Removing the router by accident would take every tool's
		// commands with it, so it needs both words.
		fmt.Fprintln(stderr, "rhythm: to remove rhythm itself, run: rhythm uninstall --all rhythm")
		return nil, exitUsage
	}
	if all {
		for _, tool := range toolNames() {
			if !seen[tool] {
				seen[tool] = true
				list = append(list, tool)
			}
		}
	}
	if self {
		list = append(list, selfName)
	}
	if len(list) == 0 {
		fmt.Fprintln(stderr, "usage: rhythm uninstall <tool>... [--purge]; name a tool, or use --all")
		return nil, exitUsage
	}
	return list, 0
}

// errNotInstalled is uninstallOne's answer for a tool that isn't there.
var errNotInstalled = errors.New("not installed")

// uninstallOne removes one tool: the copy that runs, its leftovers and,
// with purge, its data. quietMissing leaves out the "isn't installed"
// line, for --all.
func uninstallOne(tool string, purge, quietMissing bool, stdout, stderr io.Writer) error {
	binary := binaryName(tool)
	path, real, err := installedAt(tool)
	if err != nil {
		if !quietMissing {
			fmt.Fprintf(stdout, "%-18s isn't installed\n", binary)
		}
		if purge {
			if err := purgeData(tool, binary, stdout, stderr); err != nil {
				return err
			}
		}
		return errNotInstalled
	}
	if m := managedBy(real, binary); m != nil {
		// The manager's records would still list the tool. Its next
		// upgrade would put it back.
		fmt.Fprintf(stderr, "%s is managed by %s; remove it with: %s\n", binary, m.name, m.remove)
		return &managedError{binary, m}
	}

	files := []string{real, real + ".prev", real + ".prev.json", real + ".old"}
	if path != real {
		files = append([]string{path}, files...) // the link on PATH goes too, or it would dangle
	}
	failed := false
	for i, f := range files {
		err := os.Remove(f)
		if err != nil && runtime.GOOS == "windows" && tool == selfName && f == real {
			// Windows can't delete a running program. It can rename it.
			if err = os.Rename(f, f+".old"); err == nil {
				fmt.Fprintf(stdout, "Removed %s (%s.old is left; delete it once this command ends)\n", f, filepath.Base(f))
				continue
			}
		}
		switch {
		case err == nil:
			fmt.Fprintln(stdout, "Removed", f)
		case errors.Is(err, fs.ErrNotExist) && i >= len(files)-3: // .prev, .prev.json, .old
			// no leftover of this kind
		case errors.Is(err, fs.ErrPermission):
			fmt.Fprintf(stderr, "%-18s can't remove %s; rerun with sudo\n", binary, f)
			failed = true
		default:
			fmt.Fprintf(stderr, "%-18s %v\n", binary, err)
			failed = true
		}
	}
	if failed {
		return errors.New("not removed")
	}
	// Another copy further down PATH now runs instead.
	if rest := pathCopies(binary); len(rest) > 0 {
		fmt.Fprintf(stdout, "Note: another %s is still on your PATH at %s\n", binary, rest[0])
	}
	if purge {
		return purgeData(tool, binary, stdout, stderr)
	}
	return nil
}

// installedAt finds the copy of tool that runs: the path on PATH and the
// file it resolves to.
func installedAt(tool string) (path, real string, err error) {
	if tool == selfName {
		real, err = selfPath()
		if err != nil {
			return "", "", err
		}
		path = real
		// Prefer the PATH entry when it leads to the running rhythm, so a
		// link to it is removed too.
		if p, lerr := exec.LookPath(selfName); lerr == nil {
			if abs, aerr := filepath.Abs(p); aerr == nil && resolve(abs) == real {
				path = abs
			}
		}
		return path, real, nil
	}
	// A copy found through a relative PATH entry (exec.ErrDot) counts as
	// not installed: deleting a file in the current folder is never meant.
	p, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		return "", "", err
	}
	if abs, aerr := filepath.Abs(p); aerr == nil {
		p = abs
	}
	return p, resolve(p), nil
}

// purgeData removes the tool's cache folder and the update-check state
// rhythm and the tool keep for it. The first-run markers stay, so a tool
// removed on purpose isn't announced as new after the next update.
func purgeData(tool, binary string, stdout, stderr io.Writer) error {
	var paths []string
	if d, err := config.CacheDir(tool); err == nil {
		paths = append(paths, d)
	}
	if d, err := config.Dir(); err == nil {
		paths = append(paths, filepath.Join(d, "update-check", binary+".json"))
	}
	var failed error
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			fmt.Fprintf(stderr, "%-18s can't remove %s: %v\n", binary, p, err)
			failed = err
			continue
		}
		fmt.Fprintln(stdout, "Removed", p)
	}
	return failed
}
