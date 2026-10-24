// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// rhythm is the umbrella CLI. `rhythm <tool> ...` runs the `rhythm-<tool>`
// binary found on PATH (same extension model as git and gh).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/RhythmShahriar/rhythm/kit/cli"
)

var version = "dev"

// command is one of rhythm's own subcommands, as opposed to a tool.
type command struct {
	name    string // what the user types: `rhythm <name>`
	args    string // argument synopsis for help, e.g. "<tool> [--dir <dir>]"
	summary string // one line for help
	run     func(args []string, stdout, stderr io.Writer) int
	hidden  bool // left out of help, still runs
}

// commands holds every registered subcommand by name.
var commands = map[string]command{}

// register adds a subcommand. Each command file calls it from init(), so
// adding a command never means editing main.go, and help lists it.
func register(c command) {
	if _, dup := commands[c.name]; dup {
		panic("rhythm: command registered twice: " + c.name)
	}
	commands[c.name] = c
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	if slices.Contains(versionFlags, args[0]) {
		args = append([]string{"version"}, args[1:]...)
	}
	if c, ok := commands[args[0]]; ok {
		return c.run(args[1:], stdout, stderr)
	}

	tool := args[0]
	if !validToolName(tool) {
		fmt.Fprintf(stderr, "rhythm: invalid tool name %q\n", tool)
		return cli.ExitUsage
	}
	info, known := lookupTool(tool)
	if known {
		tool = info.Name
	}
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		if known {
			fmt.Fprintf(stderr, "rhythm: %s isn't installed\n  install it with: rhythm install %s\n", tool, tool)
			return cli.ExitUnavailable
		}
		return unknownTool(stderr, tool)
	}

	env := toolEnv(os.Environ(), "rhythm "+args[0])
	// rhythm can only become the tool when the tool would write to
	// rhythm's own streams; tests pass buffers and take the child path.
	if stdout == os.Stdout && stderr == os.Stderr {
		execTool(bin, args[1:], env) // returns only if the exec failed
	}
	code, err := runChild(bin, args[1:], env, stdout, stderr)
	if err != nil {
		return startFailed(stderr, tool, known, err)
	}
	return code
}

// toolEnv returns environ with RHYTHM_INVOKED_AS set to invokedAs, so
// the tool's usage lines and hints name the command the user typed
// ("rhythm sc") rather than its binary. An inherited value is dropped,
// not shadowed, because C programs read the first match in the
// environment and Go reads the last.
func toolEnv(environ []string, invokedAs string) []string {
	env := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		if !strings.HasPrefix(kv, cli.InvokedAsEnv+"=") {
			env = append(env, kv)
		}
	}
	return append(env, cli.InvokedAsEnv+"="+invokedAs)
}

// runChild runs a tool as a child process, for Windows (which has no
// exec) and for when exec fails. It returns the tool's exit code mapped
// the way a shell reports it, and an error only when the tool could not
// start.
func runChild(bin string, args, env []string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.Command(bin, args...) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr

	// Catch signals while the child runs, so rhythm waits for the tool to
	// clean up instead of exiting under it. Ctrl-C already reaches the
	// child (the terminal or console signals the whole group), so it is
	// only swallowed here; forwarding it would look like a second Ctrl-C.
	// Signals sent to rhythm alone, like SIGTERM from docker stop, are
	// passed on. signal.Ignore would be simpler but an ignored signal
	// stays ignored in every process rhythm starts.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, append([]os.Signal{os.Interrupt}, forwardSignals...)...)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-sigs:
				if s != os.Interrupt {
					_ = cmd.Process.Signal(s)
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		return 0, err
	}
	sig := -1
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		sig = int(ws.Signal())
	}
	return exitCode(cmd.ProcessState.ExitCode(), sig), nil
}

// statusControlCExit is the exit status Windows gives a console process
// that Ctrl-C or Ctrl-Break ended (STATUS_CONTROL_C_EXIT).
const statusControlCExit = 0xC000013A

// exitCode maps how a tool ended to rhythm's exit code. code is the
// tool's exit status and sig the signal that killed it, or -1. A tool
// killed by signal n gives 128+n, as in a shell, rather than Go's -1
// (which os.Exit would turn into 255); a Windows Ctrl-C exit gives 130 so
// scripts see the same code on every platform.
func exitCode(code, sig int) int {
	switch {
	case sig > 0:
		return 128 + sig
	case uint32(code) == statusControlCExit: //nolint:gosec // exit statuses are 32-bit on Windows
		return cli.ExitInterrupted
	}
	return code
}

// startFailed reports a tool binary that was found but could not be
// started (a broken download, the wrong architecture) and returns
// ExitUnavailable, so CI never mistakes it for the tool's findings.
func startFailed(stderr io.Writer, tool string, known bool, err error) int {
	fmt.Fprintf(stderr, "rhythm: can't start %s: %v\n", tool, err)
	if known {
		fmt.Fprintf(stderr, "  reinstall it with: rhythm install %s\n", tool)
	}
	return cli.ExitUnavailable
}

// visibleCommands returns the commands help lists, sorted by name.
func visibleCommands() []command {
	var cs []command
	for _, c := range commands {
		if !c.hidden {
			cs = append(cs, c)
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].name < cs[j].name })
	return cs
}
