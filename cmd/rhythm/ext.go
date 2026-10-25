// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/RhythmShahriar/rhythm/internal/ext"
	"github.com/RhythmShahriar/rhythm/internal/lock"
	"github.com/RhythmShahriar/rhythm/kit/cli"
	"github.com/RhythmShahriar/rhythm/kit/term"
)

func init() {
	register(command{name: "ext", args: "search|install|list|remove ...", summary: "find, install and remove verified third-party extensions", run: cmdExt})
}

const extSynopsis = `Usage: rhythm ext <command> [args]

Extensions are third-party tools: a GitHub repository owner/rhythm-<name>
with the topic rhythm-extension, whose releases follow the same archive
layout as rhythm's tools and are signed with Sigstore. Once installed,
rhythm <name> runs it and rhythm update keeps it current.

Commands:
  rhythm ext search [words...]                      find extensions on GitHub
  rhythm ext install owner/rhythm-<name>[@version]  install one, signature checked
  rhythm ext list [--json]                          show the installed extensions
  rhythm ext remove <name>                          delete one and forget it

An extension's releases must be signed by a workflow of its own
repository (or by rhythm's shared release workflow called from it). The
first install records which workflow signed it; every later release must
come from the same workflow, or the update is refused.

Exit status: 0 on success, 1 when something failed, 2 for a usage error
and 130 when interrupted.`

// cmdExt is rhythm ext: it hands the rest of args to the subcommand named
// first.
func cmdExt(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: rhythm ext search|install|list|remove ...; see rhythm ext --help")
		return exitUsage
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, extSynopsis)
		return 0
	case "search":
		return extSearch(args[1:], stdout, stderr)
	case "install":
		return extInstall(args[1:], stdout, stderr)
	case "list":
		return extList(args[1:], stdout, stderr)
	case "remove":
		return extRemove(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "rhythm: unknown ext command %q\n", args[0])
	cli.DidYouMean(stderr, "rhythm ext", args[0], []string{"search", "install", "list", "remove"})
	fmt.Fprintln(stderr, "  use one of: search, install, list, remove")
	return exitUsage
}

// extNameTaken says why name can't be an extension's, or "" when it can:
// the registry's tools and aliases, rhythm itself and rhythm's own
// commands all come first, so an extension of that name would never run.
func extNameTaken(name string) string {
	switch {
	case !ext.ValidName(name):
		return fmt.Sprintf("%q isn't a valid tool name", name)
	case name == selfName:
		return "rhythm is the router's own name"
	}
	if t, ok := lookupTool(name); ok {
		if t.Name == name {
			return fmt.Sprintf("%s is one of rhythm's own tools (%s)", name, t.Repo)
		}
		return fmt.Sprintf("%s is short for rhythm's own %s", name, t.Name)
	}
	if _, ok := commands[name]; ok {
		return fmt.Sprintf("%s is a rhythm command (rhythm %s)", name, name)
	}
	return ""
}

const extSearchSynopsis = `Usage: rhythm ext search [words...] [flags]

Searches GitHub for repositories with the topic rhythm-extension whose
names start with rhythm-, most starred first. Names rhythm keeps for its
own tools and commands are marked; those can't be installed.`

// extSearch is rhythm ext search. Results are third-party text, shown
// only after term.SafeText.
func extSearch(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ext search", extSearchSynopsis)
	asJSON := flags.Bool("json", false, "print the results as rhythm-ext-search/v1 JSON")
	words, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	api := os.Getenv("RHYTHM_GITHUB_API")
	// The search carries the token too, so it follows the same token and
	// redirect rules as the release lookups.
	_, tok := apiToken(os.Getenv, api)
	c := &ext.Client{API: apiOrDefault(api), HTTP: apiClient(updater("", "", "").HTTP), Token: tok, UserAgent: fmt.Sprintf("rhythm/%s (+https://rythm.sh)", version)}
	ctx, stop := signalContext()
	defer stop()
	found, err := c.Search(ctx, words)
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: search interrupted")
		return exitInterrupted
	}
	if err != nil {
		var nets netReport
		if nets.add("the search", err) {
			nets.print(stderr)
		} else {
			fmt.Fprintln(stderr, "rhythm:", describe(err))
		}
		return 1
	}
	if *asJSON {
		type result struct {
			ext.Repo
			Taken string `json:"taken"` // why it can't be installed; "" when it can
		}
		out := struct {
			Schema  string   `json:"schema"`
			Results []result `json:"results"`
		}{Schema: "rhythm-ext-search/v1", Results: []result{}}
		for _, r := range found {
			out.Results = append(out.Results, result{Repo: r, Taken: extNameTaken(r.Name)})
		}
		if err := writeJSON(stdout, out); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		return 0
	}
	if len(found) == 0 {
		fmt.Fprintln(stdout, "No extensions found.")
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tREPO\tSTARS\tDESCRIPTION")
	suggest := ""
	for _, r := range found {
		name := r.Name
		if extNameTaken(r.Name) != "" {
			name += " (name taken)"
		} else if suggest == "" {
			suggest = r.FullName
		}
		// Descriptions are anyone's text: no control bytes reach the terminal.
		desc := strings.ReplaceAll(term.SafeText(r.Description), "\t", " ") // a tab would break the columns
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", name, term.SafeText(r.FullName), r.Stars, desc)
	}
	_ = tw.Flush()
	// Only a result that can be installed is suggested.
	if suggest != "" {
		fmt.Fprintf(stdout, "Install one with: rhythm ext install %s\n", term.SafeText(suggest))
	}
	return 0
}

const extInstallSynopsis = `Usage: rhythm ext install owner/rhythm-<name>[@version] [flags]

Installs an extension from its GitHub releases next to rhythm (or into
--dir) and records it in extensions.json, so rhythm <name> runs it and
rhythm update keeps it current. The release must carry a Sigstore
signature over checksums.txt from a workflow of the extension's own
repository; an unsigned or wrongly signed release is refused. The
workflow that signed it is recorded. Later releases must come from the
same one.`

// extInstall is rhythm ext install. It has no --skip-signature: the
// signer is what an extension is trusted on.
func extInstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ext install", extInstallSynopsis)
	dir := flags.String("dir", "", "install into `dir` (default: next to rhythm)")
	force := flags.Bool("force", false, "reinstall when this release is already installed")
	quiet := flags.Bool("quiet", false, "print only the Installed line and errors")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: rhythm ext install owner/rhythm-<name>[@version]; name one extension")
		return exitUsage
	}
	spec, err := ext.ParseSpec(pos[0])
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return exitUsage
	}
	if spec.Version != "" && checkSpec(spec.Version) != nil {
		fmt.Fprintf(stderr, "rhythm: %s: %v\n", spec.Repo, specError(spec.Version))
		return exitUsage
	}
	if why := extNameTaken(spec.Name); why != "" {
		fmt.Fprintf(stderr, "rhythm: can't install %s as an extension: %s\n", spec.Repo, why)
		return exitUsage
	}
	store, err := lock.OpenExtensions()
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	current, err := store.List()
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	it := installItem{tool: spec.Name, spec: spec.Version, extension: true, repo: spec.Repo}
	for _, x := range current {
		if x.Name != spec.Name {
			continue
		}
		if !strings.EqualFold(x.Repo, spec.Repo) {
			fmt.Fprintf(stderr, "rhythm: %s is already installed from %s; remove it first with: rhythm ext remove %s\n", spec.Name, x.Repo, spec.Name)
			return 1
		}
		it.repo, it.signer = x.Repo, x.Signer // a reinstall keeps the signer it had
	}

	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		if m := managedBy(self, selfName); m != nil {
			fmt.Fprintf(stderr, "rhythm: rhythm is managed by %s, so extensions don't go in its folder\n"+
				"  pick a folder on your PATH: rhythm ext install %s --dir <dir>\n", m.name, pos[0])
			return exitUsage
		}
		*dir = filepath.Dir(self)
	}
	abs, err := filepath.Abs(*dir)
	if err == nil {
		err = prepareDir(abs, abs)
	}
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	// Signatures are always checked here: extensions are installed on
	// the strength of who signed them.
	run := &runOptions{}
	defer beginRun(run)()
	if v, err := run.activeVerifier(); err != nil || v == nil {
		fmt.Fprintf(stderr, "rhythm: can't install %s: this rhythm can't check signatures (%v)\n", spec.Repo, err)
		return 1
	}
	opt := installOptions{dir: abs, quiet: *quiet, force: *force}
	if !*quiet && isTerminal(stderr) {
		opt.progress = stderr
	}
	ctx, stop := signalContext()
	defer stop()
	res, err := installTool(ctx, it, opt, stdout)
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: install interrupted")
		return exitInterrupted
	}
	if err != nil {
		var nets netReport
		var me *managedError
		switch {
		case errors.As(err, &me):
			fmt.Fprintln(stderr, err)
		case nets.add("rhythm-"+spec.Name, err):
			nets.print(stderr)
		default:
			fmt.Fprintf(stderr, "%-18s %s\n", "rhythm-"+spec.Name, describe(err))
		}
		return 1
	}
	if res.already {
		return 0
	}
	if !recordExtension(it, res.signer, stderr) {
		return 1
	}
	if !*quiet {
		if res.note == noteOffPath {
			fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", abs)
		} else if res.note != "" {
			fmt.Fprintln(stdout, res.note)
		}
		fmt.Fprintf(stdout, "Run it with: rhythm %s\n", spec.Name)
	}
	return 0
}

// recordExtension writes it to extensions.json once it is installed,
// with the workflow that signed it. It reports whether that worked.
func recordExtension(it installItem, signer string, stderr io.Writer) bool {
	x := lock.Extension{Name: it.tool, Repo: it.repo, Signer: it.signer}
	if signer != "" {
		x.Signer = signerIdentity(signer)
	}
	// The verifier already holds an extension to its own repository's
	// workflows (see buildVerifier). The rule is checked again here because
	// every later update trusts the signer on record.
	if x.Signer != "" && !ext.SignerAllowed(it.repo, x.Signer) {
		fmt.Fprintf(stderr, "rhythm: %s was signed by %s, which isn't a workflow of %s; not recorded\n", it.repo, x.Signer, it.repo)
		return false
	}
	store, err := lock.OpenExtensions()
	if err == nil {
		err = store.Add(x)
	}
	if err != nil {
		fmt.Fprintf(stderr, "rhythm: installed rhythm-%s but couldn't record it: %v\n", it.tool, err)
		return false
	}
	return true
}

// lockedExtension is the install item for an extension rhythm.lock
// lists. A name rhythm uses itself is refused: a cloned lock must not
// replace one of rhythm's own tools.
func lockedExtension(e *lockEntry, stderr io.Writer) (installItem, bool) {
	if why := extNameTaken(e.Name); why != "" {
		fmt.Fprintf(stderr, "rhythm: %s lists the extension %s from %s, which can't be installed: %s\n", lockFileName, e.Name, e.Repo, why)
		return installItem{}, false
	}
	return installItem{tool: e.Name, spec: e.Version, extension: true, repo: e.Repo, signer: e.Signer}, true
}

// extList is rhythm ext list.
func extList(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ext list", "Usage: rhythm ext list [flags]\n\nShows the installed extensions: where each comes from, the workflow that\nmust sign it, its version and where it is.")
	asJSON := flags.Bool("json", false, "print the list as rhythm-ext-list/v1 JSON")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) > 0 {
		fmt.Fprintln(stderr, "rhythm: ext list takes no names")
		return exitUsage
	}
	store, err := lock.OpenExtensions()
	var all []lock.Extension
	if err == nil {
		all, err = store.List()
	}
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	ctx, stop := signalContext()
	defer stop()
	type row struct {
		Name    string `json:"name"`
		Repo    string `json:"repo"`
		Signer  string `json:"signer"`
		Version string `json:"version"` // "" when it isn't on PATH or didn't say
		Path    string `json:"path"`    // "" when it isn't on PATH
		Signed  string `json:"signed"`  // "yes", "no" or "" when not checked
	}
	rows := []row{}
	for _, x := range all {
		r := row{Name: x.Name, Repo: x.Repo, Signer: x.Signer, Signed: "no"}
		if p, err := exec.LookPath("rhythm-" + x.Name); err == nil {
			if abs, aerr := filepath.Abs(p); aerr == nil {
				p = abs
			}
			r.Path, r.Signed = p, signed(resolve(p))
			if v, _, _ := probeVersion(ctx, p); v != unknownVersion {
				r.Version = v
			}
		}
		rows = append(rows, r)
	}
	if *asJSON {
		for i := range rows {
			if rows[i].Signed == "-" {
				rows[i].Signed = ""
			}
		}
		if err := writeJSON(stdout, struct {
			Schema     string `json:"schema"`
			Extensions []row  `json:"extensions"`
		}{"rhythm-ext-list/v1", rows}); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		return 0
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No extensions installed. Find some with: rhythm ext search")
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tSIGNED\tREPO\tPATH")
	for _, r := range rows {
		v, p := r.Version, r.Path
		if v == "" {
			v = "-"
		}
		if p == "" {
			p = "(not on PATH)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, v, r.Signed, r.Repo, p)
	}
	_ = tw.Flush()
	return 0
}

// extRemove is rhythm ext remove. Only a name extensions.json lists is
// removed, so it can't delete one of rhythm's own tools.
func extRemove(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ext remove", "Usage: rhythm ext remove <name> [flags]\n\nDeletes the extension's binary (and the copy its last update kept) and\nremoves it from extensions.json.")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: rhythm ext remove <name>; name one extension")
		return exitUsage
	}
	name := strings.TrimPrefix(pos[0], "rhythm-")
	if spec, err := ext.ParseSpec(pos[0]); err == nil {
		name = spec.Name // owner/rhythm-name works too
	}
	store, err := lock.OpenExtensions()
	var all []lock.Extension
	if err == nil {
		all, err = store.List()
	}
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	known := ""
	for _, x := range all {
		if x.Name == name {
			known = x.Name
		}
	}
	if known == "" {
		if _, registry := lookupTool(name); registry {
			fmt.Fprintf(stderr, "rhythm: %s is one of rhythm's own tools; remove it with: rhythm uninstall %s\n", name, name)
		} else {
			fmt.Fprintf(stderr, "rhythm: no extension %q is installed (see rhythm ext list)\n", name)
		}
		return 1
	}
	// The name as extensions.json has it, which passed the tool-name rule.
	if err := uninstallOne(known, false, false, stdout, stderr); err != nil && !errors.Is(err, errNotInstalled) {
		return 1
	}
	if _, err := store.Remove(known); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Removed %s from %s\n", name, store.Path())
	return 0
}
