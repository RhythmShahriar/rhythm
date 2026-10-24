// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Command stub is a stand-in for the rhythm router in the install.ps1
// tests. Windows runs only real executables, so run.ps1 builds this for
// each fake release, with the release's behavior set through -ldflags:
//
//	-X main.version=0.4.0          what --version prints
//	-X main.tools=tuner:t,echo:e   the tools it knows, with their aliases
//	-X main.dryRun=yes             whether install takes --dry-run and --quiet
//	-X main.toolsBlock=yes         whether help has a "Tools:" block
//
// It logs its argv to $RHYTHM_TEST_LOG, "installs" a tool by copying itself
// to rhythm-<tool>(.exe) in --dir and notes when --dir isn't on PATH: once
// with dryRun (unless --quiet), after each tool without, like the real
// releases. `hold` keeps it running for a minute
// so a test can install over a running rhythm.exe. scripts/test/mkfake.sh
// has the same stand-in as a shell script for install.sh.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	version    = "0.0.0"
	tools      = "tuner:t,soundcheck:sc,echo:e"
	dryRun     = "yes"
	toolsBlock = "yes"
)

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
}

func install(args []string, stdout, stderr io.Writer) int {
	dir, dry, quiet := ".", false, false
	var want []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			if dryRun != "yes" {
				fmt.Fprintln(stderr, "Usage of install:\n  -dir string\n    \tinstall directory (default: next to rhythm)")
				fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
				return 2
			}
			fmt.Fprintln(stdout, "Usage: rhythm install <tool>... [flags]\n\nFlags:")
			fmt.Fprintln(stdout, "  --dir dir        install into dir (default: next to rhythm)")
			fmt.Fprintln(stdout, "  --dry-run        check the names and print the tools, one per line, without installing")
			fmt.Fprintln(stdout, "  --quiet          print only the Installed lines and errors")
			return 0
		case "--quiet":
			if dryRun != "yes" {
				fmt.Fprintln(stderr, "flag provided but not defined: -quiet")
				return 2
			}
			quiet = true
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "--dry-run":
			if dryRun != "yes" {
				fmt.Fprintln(stderr, "flag provided but not defined: -dry-run")
				fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
				return 2
			}
			dry = true
		default:
			want = append(want, args[i])
		}
	}
	// known maps each name and alias to the tool's name, as rhythm does.
	known := map[string]string{}
	for _, t := range strings.Split(tools, ",") {
		name, alias, _ := strings.Cut(t, ":")
		known[name], known[alias] = name, name
	}
	for i, name := range want {
		if known[name] == "" {
			fmt.Fprintf(stderr, "rhythm: unknown tool %q; known tools: %s\n", name, tools)
			return 2
		}
		want[i] = known[name]
	}
	for _, name := range want {
		if dry {
			fmt.Fprintln(stdout, "would install", name)
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
		fmt.Fprintf(stdout, "Installed rhythm-%s 0.0.0-test to %s\n", name, dest)
		if dryRun != "yes" && !onPath(dir) {
			fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", dir)
		}
	}
	if dryRun == "yes" && !quiet && !dry && !onPath(dir) {
		fmt.Fprintf(stdout, "Note: %s isn't on your PATH.\n", dir)
	}
	return 0
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
