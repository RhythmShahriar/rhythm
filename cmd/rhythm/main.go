// rhythm is the umbrella CLI. `rhythm <tool> ...` runs the `rhythm-<tool>`
// binary found on PATH (same extension model as git and gh).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/RhythmShahriar/rhythm/kit"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		usage()
		return
	}
	if os.Args[1] == "--version" || os.Args[1] == "version" {
		fmt.Println("rhythm", version)
		return
	}

	tool := os.Args[1]
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rhythm: unknown tool %q\n  install it with: rhythm install %s\n", tool, tool)
		os.Exit(1)
	}

	cmd := exec.Command(bin, os.Args[2:]...) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "rhythm:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(kit.Banner("rhythm", version))
	fmt.Println(`Usage:
  rhythm <tool> [args]

Tools:
  tuner    WordPress & PHP security scanner

Other:
  rhythm version`)
}
