// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Command stub is a stand-in for the rhythm router in the install.ps1
// tests. Windows runs only real executables, so run.ps1 builds this for
// each fake release, with the release's behavior set through -ldflags:
//
//	-X main.version=0.4.0          what --version prints
//	-X main.tools=tuner:t,echo:e   the tools it knows, with their aliases
//	-X main.features=dry-run,verify  what it can do, separated by commas:
//	                               "dry-run" (install --dry-run and --quiet,
//	                               which came together), "verify" (the
//	                               verify command, which fails when
//	                               RHYTHM_TEST_VERIFY is "fail"), "defaults"
//	                               (install --defaults: $RHYTHM_TEST_DEFAULTS
//	                               when set, else echo, soundcheck and
//	                               tuner), "pins" (tool@version) and
//	                               "locked" (install --locked, from
//	                               ./rhythm.lock)
//	-X main.toolsBlock=yes         whether help has a "Tools:" block
//
// It logs its argv to $RHYTHM_TEST_LOG, checks every name before installing
// any (with rhythm's did-you-mean lines), fetches each tool's archive from
// $RHYTHM_GITHUB_API so a test can make one 404, "installs" a tool by
// copying itself to rhythm-<tool>(.exe) in --dir and notes when --dir isn't
// on PATH: once with dry-run (unless --quiet), after each tool without, like
// the real releases. `hold` keeps it running for a minute
// so a test can install over a running rhythm.exe. scripts/test/mkfake.sh
// has the same stand-in as a shell script for install.sh.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var (
	version    = "0.0.0"
	tools      = "tuner:t,soundcheck:sc,echo:e"
	features   = "dry-run,verify,defaults,pins,locked"
	toolsBlock = "yes"
)

// has reports whether the stand-in has feature f.
func has(f string) bool {
	for _, g := range strings.Split(features, ",") {
		if g == f {
			return true
		}
	}
	return false
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if log := os.Getenv("RHYTHM_TEST_LOG"); log != "" {
		if f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil { //nolint:gosec // the test names the log
			fmt.Fprintln(f, strings.TrimSpace("rhythm "+strings.Join(args, " ")))
			_ = f.Close()
		}
	}
	cmd := "help"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "--version", "version":
		fmt.Fprintln(stdout, "rhythm", version)
	case "help", "-h", "--help":
		help(stdout)
	case "hold":
		time.Sleep(time.Minute)
	case "install":
		return install(args[1:], stdout, stderr)
	case "verify":
		if !has("verify") {
			fmt.Fprintf(stderr, "rhythm: unknown tool %q\n", cmd)
			return 1
		}
		if os.Getenv("RHYTHM_TEST_VERIFY") == "fail" {
			fmt.Fprintf(stderr, "%-18s signature not valid: RhythmShahriar/rhythm %s: failed to verify signature\n", "rhythm", version)
			return 1
		}
		fmt.Fprintf(stdout, "Signature ok  RhythmShahriar/rhythm @ refs/tags/v%s (release-go.yml)\n", version)
	default:
		fmt.Fprintf(stderr, "rhythm: unknown tool %q\n", cmd)
		return 1
	}
	return 0
}

func help(w io.Writer) {
	fmt.Fprintf(w, "rhythm %s\n\nUsage:\n  rhythm <tool> [args]\n\n", version)
	if toolsBlock == "yes" {
		fmt.Fprintln(w, "Tools:")
		for _, t := range strings.Split(tools, ",") {
			name, alias, _ := strings.Cut(t, ":")
			fmt.Fprintf(w, "  %s (%s)  a tool\n", name, alias)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "Commands:\n  rhythm install <tool> [--dir <dir>]   download a tool")
	if has("verify") {
		fmt.Fprintln(w, "  rhythm verify [tool...]               check signatures")
	}
}

func install(args []string, stdout, stderr io.Writer) int {
	dir, dry, quiet, defaults, locked := ".", false, false, false, false
	var want []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			if !has("dry-run") {
				fmt.Fprintln(stderr, "Usage of install:\n  -dir string\n    \tinstall directory (default: next to rhythm)")
				fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
				return 2
			}
			fmt.Fprintln(stdout, "Usage: rhythm install <tool>... [flags]\n\nFlags:")
			if has("defaults") {
				fmt.Fprintln(stdout, "  --defaults       also install the tools a fresh setup gets")
			}
			fmt.Fprintln(stdout, "  --dir dir        install into dir (default: next to rhythm)")
			fmt.Fprintln(stdout, "  --dry-run        check the names and print the tools, one per line, without installing")
			if has("locked") {
				fmt.Fprintln(stdout, "  --locked         install the versions in rhythm.lock")
			}
			fmt.Fprintln(stdout, "  --quiet          print only the Installed lines and errors")
			return 0
		case "--defaults", "--locked":
			if !has(strings.TrimPrefix(args[i], "--")) {
				return badFlag(args[i], stderr)
			}
			defaults = defaults || args[i] == "--defaults"
			locked = locked || args[i] == "--locked"
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "--quiet", "--dry-run":
			if !has("dry-run") {
				return badFlag(args[i], stderr)
			}
			quiet = quiet || args[i] == "--quiet"
			dry = dry || args[i] == "--dry-run"
		default:
			if strings.HasPrefix(args[i], "-") {
				return badFlag(args[i], stderr)
			}
			want = append(want, args[i])
		}
	}
	// The defaults come after any named tools, as in rhythm.
	if defaults {
		list, ok := os.LookupEnv("RHYTHM_TEST_DEFAULTS")
		if !ok {
			list = "echo soundcheck tuner"
		}
		want = append(want, strings.Fields(list)...)
	}
	if locked {
		pins, err := readLock("rhythm.lock")
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		want = pins
	}
	// known maps each name and alias to the tool's name, as rhythm does.
	known := map[string]string{}
	var names []string
	for _, t := range strings.Split(tools, ",") {
		name, alias, _ := strings.Cut(t, ":")
		known[name], known[alias] = name, name
		names = append(names, name)
	}
	sort.Strings(names)
	// Every name is checked before anything is installed. A pin
	// (tuner@0.3.1) keeps its version.
	var bad []string
	pinned := make([]string, len(want))
	for i, spec := range want {
		name := spec
		if n, v, ok := strings.Cut(spec, "@"); ok && has("pins") {
			name, pinned[i] = n, v
		}
		if known[name] == "" {
			bad = append(bad, name)
		}
		want[i] = known[name]
	}
	if len(bad) > 0 {
		for _, name := range bad {
			if near := suggest(name, names); near != "" {
				fmt.Fprintf(stderr, "rhythm: unknown tool %q (did you mean %s?)\n", name, near)
			} else {
				fmt.Fprintf(stderr, "rhythm: unknown tool %q\n", name)
			}
		}
		fmt.Fprintln(stderr, "  known:", strings.Join(names, ", "))
		return 2
	}
	var failed []string
	for i, name := range want {
		if dry {
			if pinned[i] != "" {
				name += "@" + pinned[i]
			}
			fmt.Fprintln(stdout, name)
			continue
		}
		if err := fetchArchive(name); err != nil {
			fmt.Fprintf(stderr, "%-18s %v\n", "rhythm-"+name, err)
			failed = append(failed, "rhythm-"+name)
			continue
		}
		dest := filepath.Join(dir, "rhythm-"+name)
		if runtime.GOOS == "windows" {
			dest += ".exe"
		}
		if err := copySelf(dest); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		v := pinned[i]
		if v == "" {
			v = "0.0.0-test"
		}
		fmt.Fprintf(stdout, "Installed rhythm-%s %s to %s\n", name, v, dest)
		if !has("dry-run") && !onPath(dir) {
			fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", dir)
		}
	}
	if has("dry-run") && !quiet && !dry && !onPath(dir) {
		fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", dir)
	}
	if len(failed) > 0 {
		if len(want) > 1 {
			fmt.Fprintf(stderr, "Installed %d, failed %d: %s\n", len(want)-len(failed), len(failed), strings.Join(failed, ", "))
		}
		return 1
	}
	return 0
}

// readLock returns the tools in a rhythm.lock as name@version, in order.
func readLock(path string) ([]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the test writes the lock
	if os.IsNotExist(err) {
		return nil, errors.New("no rhythm.lock found")
	}
	if err != nil {
		return nil, err
	}
	var lock struct {
		Tools []struct{ Name, Version string }
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	var pins []string
	for _, t := range lock.Tools {
		pins = append(pins, t.Name+"@"+t.Version)
	}
	return pins, nil
}

// badFlag fails the way Go's flag package does for a flag it doesn't know.
func badFlag(arg string, stderr io.Writer) int {
	fmt.Fprintln(stderr, "flag provided but not defined: -"+strings.TrimLeft(arg, "-"))
	fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
	return 2
}

// fetchArchive asks the fake API for the tool's archive, so a test can make
// one download fail with a 404. Without RHYTHM_GITHUB_API it does nothing.
func fetchArchive(name string) error {
	api := os.Getenv("RHYTHM_GITHUB_API")
	if api == "" {
		return nil
	}
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	archive := fmt.Sprintf("%s_%s_%s%s", name, runtime.GOOS, runtime.GOARCH, ext)
	resp, err := http.Get(api + "/dl/" + name + "/v0.1.0/" + archive) //nolint:gosec,noctx // the test's own fake API
	if err != nil {
		return fmt.Errorf("downloading %s: %w", archive, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", archive, resp.StatusCode)
	}
	return nil
}

// suggest returns the option within two edits of name, if any.
func suggest(name string, options []string) string {
	for _, o := range options {
		if levenshtein(name, o) <= 2 {
			return o
		}
	}
	return ""
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// onPath reports whether dir is one of the folders on PATH.
func onPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return true
		}
	}
	return false
}

func copySelf(dest string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(self) //nolint:gosec // our own executable
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o755) //nolint:gosec // an executable must be executable
}
