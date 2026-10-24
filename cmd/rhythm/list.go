// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
)

// routerCommand is one of the housekeeping commands in this lane's files
// (list, uninstall, doctor). It mirrors router-core's command type; once
// that lands, each init() calls register(command{...}) instead and this
// stand-in goes away.
type routerCommand struct {
	name    string // what the user types: `rhythm <name>`
	args    string // argument synopsis for help
	summary string // one line for help
	run     func(args []string, stdout, stderr io.Writer) int
}

// routerCommands holds the commands registered through addRouterCommand.
var routerCommands = map[string]routerCommand{}

// addRouterCommand registers c, as router-core's register() will.
func addRouterCommand(c routerCommand) {
	if _, dup := routerCommands[c.name]; dup {
		panic("rhythm: command registered twice: " + c.name)
	}
	routerCommands[c.name] = c
}

func init() {
	addRouterCommand(routerCommand{name: "list", args: "[--offline] [--json]", summary: "show the installed tools, their versions and paths", run: cmdList})
}

// signedStatus reports whether the binary at path carries a valid
// release signature ("yes" or "no"). It stays nil until signature
// checks land; until then rhythm shows "-".
var signedStatus func(path string) string

// signed returns the SIGNED column for path.
func signed(path string) string {
	if signedStatus == nil {
		return "-"
	}
	return signedStatus(path)
}

const listSynopsis = `Usage: rhythm list [flags]

Shows rhythm, every installed tool and any other rhythm-* program on
your PATH, with the version each reports, the latest release and where
it is. "(3rd)" marks a program that isn't one of rhythm's tools.

Exit status: 0 when the list was shown, 2 for a usage error and 130 when
interrupted. A release that couldn't be looked up shows "?".`

// listEntry is one row of rhythm list.
type listEntry struct {
	name       string // tool name, "rhythm" or a third-party name
	binary     string // e.g. rhythm-tuner
	version    string
	latest     string // "" when not looked up
	signed     string
	path       string // where it was found on PATH
	thirdParty bool
	managedBy  string // package manager name, or ""
	versionErr string // why the version is unknown
	latestErr  string // why the latest release is unknown
}

func cmdList(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("list", listSynopsis)
	offline := flags.Bool("offline", false, "don't look up the latest releases")
	asJSON := flags.Bool("json", false, "print the list as rhythm-list/v1 JSON")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) > 0 {
		fmt.Fprintf(stderr, "rhythm: list takes no names (got %q)\n", pos[0])
		return exitUsage
	}
	ctx, stop := signalContext()
	defer stop()
	*offline = *offline || offlineOn(os.Getenv(offlineEnv))
	entries, nets := listEntries(ctx, *offline)
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: list interrupted")
		return exitInterrupted
	}
	if *asJSON {
		if err := writeJSON(stdout, listJSON(entries, nets)); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
	} else {
		printList(stdout, entries, !*offline)
	}
	for _, e := range entries {
		for _, msg := range []string{e.versionErr, e.latestErr} {
			if msg != "" {
				fmt.Fprintf(stderr, "%-18s %s\n", e.binary, msg)
			}
		}
	}
	nets.print(stderr)
	return 0
}

// listEntries finds rhythm, the installed tools and third-party rhythm-*
// programs and, unless offline, looks up the latest releases. Problems
// reaching the release API are collected in the netReport.
func listEntries(ctx context.Context, offline bool) ([]listEntry, *netReport) {
	// Third-party programs are asked for their versions alongside the
	// registry tools, so hung ones cost the probe timeout once in all.
	var others []target
	for _, tp := range thirdPartyTools() {
		others = append(others, target{tool: tp.name, binary: "rhythm-" + tp.name, path: tp.path, real: resolve(tp.path)})
	}
	probed := make(chan struct{})
	go func() {
		probeTargets(ctx, others)
		close(probed)
	}()
	targets, _ := findTargets(ctx, nil)
	nets := &netReport{}
	var entries []listEntry
	for _, t := range targets {
		e := listEntry{name: t.tool, binary: t.binary, version: t.current, signed: signed(t.real), path: t.path}
		if t.managed != nil {
			e.managedBy = t.managed.name
		}
		if t.current == unknownVersion {
			e.versionErr = "version unknown (" + t.probe + ")"
		}
		if !offline && ctx.Err() == nil && !nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")) {
			rel, err := updater(t.repo, t.binary, t.current).Latest(ctx)
			switch {
			case err == nil:
				e.latest = rel.Version
			case ctx.Err() != nil:
			case !nets.add(t.binary, err):
				e.latestErr = describe(err) // network problems are said once, by nets
			}
		}
		entries = append(entries, e)
	}
	<-probed
	for _, t := range others {
		e := listEntry{name: t.tool, binary: t.binary, version: t.current, signed: signed(t.real), path: t.path, thirdParty: true}
		if t.current == unknownVersion {
			e.versionErr = "version unknown (" + t.probe + ")"
		}
		entries = append(entries, e)
	}
	return entries, nets
}

// listReport is rhythm list --json's answer.
type listReport struct {
	Schema string     `json:"schema"` // rhythm-list/v1
	Tools  []listTool `json:"tools"`
}

// listTool is one row of the rhythm-list/v1 report. Latest is empty when
// it wasn't looked up (--offline, third-party programs) or the lookup
// failed; Error then says why.
type listTool struct {
	Name       string `json:"name"`   // tool name, rhythm or a third-party name
	Binary     string `json:"binary"` // rhythm or rhythm-<name>
	Version    string `json:"version"`
	Latest     string `json:"latest"`
	Signed     string `json:"signed"` // "yes", "no" or "" when not checked
	Path       string `json:"path"`
	ThirdParty bool   `json:"third_party"`
	ManagedBy  string `json:"managed_by"`
	Error      string `json:"error"`
}

func listJSON(entries []listEntry, nets *netReport) listReport {
	rep := listReport{Schema: "rhythm-list/v1", Tools: []listTool{}}
	for _, e := range entries {
		var errs []string
		for _, msg := range []string{e.versionErr, e.latestErr, nets.explain(e.binary)} {
			if msg != "" {
				errs = append(errs, msg)
			}
		}
		v := e.version
		if v == unknownVersion {
			v = ""
		}
		signed := e.signed
		if signed == "-" {
			signed = ""
		}
		rep.Tools = append(rep.Tools, listTool{Name: e.name, Binary: e.binary, Version: v, Latest: e.latest, Signed: signed,
			Path: e.path, ThirdParty: e.thirdParty, ManagedBy: e.managedBy, Error: strings.Join(errs, "; ")})
	}
	return rep
}

// printList writes the table, e.g.
//
//	TOOL       VERSION  LATEST  SIGNED  PATH
//	rhythm     0.6.0    0.6.0   -       /home/me/.local/bin/rhythm
//	foo (3rd)  1.2.0    -       -       /usr/local/bin/rhythm-foo
func printList(w io.Writer, entries []listEntry, withLatest bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if withLatest {
		fmt.Fprintln(tw, "TOOL\tVERSION\tLATEST\tSIGNED\tPATH")
	} else {
		fmt.Fprintln(tw, "TOOL\tVERSION\tSIGNED\tPATH")
	}
	for _, e := range entries {
		name, path := e.name, e.path
		if e.thirdParty {
			name += " (3rd)"
		}
		if e.managedBy != "" {
			path += " (" + e.managedBy + ")"
		}
		if !withLatest {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, e.version, e.signed, path)
			continue
		}
		latest := e.latest
		switch {
		case e.thirdParty:
			latest = "-" // rhythm doesn't know where its releases are
		case latest == "":
			latest = "?"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", name, e.version, latest, e.signed, path)
	}
	_ = tw.Flush()
}

// thirdParty is a rhythm-* program on PATH that isn't a registry tool.
type thirdParty struct {
	name string // without the rhythm- prefix
	path string
}

// thirdPartyTools returns the rhythm-* programs on PATH that aren't
// rhythm's own tools, sorted by name. Only the first of each name counts:
// it is the one `rhythm <name>` runs.
func thirdPartyTools() []thirdParty {
	seen := map[string]bool{}
	var found []thirdParty
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		// A relative entry such as "." is the current folder, which may be
		// a download or a checkout: nothing there is run (see exec.ErrDot).
		if !filepath.IsAbs(dir) {
			continue
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range ents {
			name, ok := strings.CutPrefix(ent.Name(), "rhythm-")
			if !ok {
				continue
			}
			if runtime.GOOS == "windows" {
				if name, ok = strings.CutSuffix(strings.ToLower(name), ".exe"); !ok {
					continue
				}
			}
			// Only extension names count, so a kept rhythm-tuner.prev or
			// an update's leftover .prev-new or .rollback isn't a tool.
			if _, registry := tools[name]; registry || !extensionName.MatchString(name) || seen[name] {
				continue
			}
			p := filepath.Join(dir, ent.Name())
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
			if !isProgram(p) {
				continue
			}
			seen[name] = true
			found = append(found, thirdParty{name: name, path: p})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].name < found[j].name })
	return found
}

// isProgram reports whether p is a file that can be run.
func isProgram(p string) bool {
	fi, err := os.Stat(p) //nolint:gosec // a file found on the user's own PATH
	if err != nil || fi.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || fi.Mode()&0o111 != 0
}

// pathCopies returns every copy of binary on PATH, in PATH order, as
// absolute paths. The first is the one that runs.
func pathCopies(binary string) []string {
	var copies []string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			continue // the current folder isn't an install; see thirdPartyTools
		}
		p, err := exec.LookPath(filepath.Join(dir, exe(binary)))
		if err != nil {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if !seen[p] {
			seen[p] = true
			copies = append(copies, p)
		}
	}
	return copies
}
