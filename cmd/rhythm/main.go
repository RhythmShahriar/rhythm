// rhythm is the umbrella CLI. `rhythm <tool> ...` runs the `rhythm-<tool>`
// binary found on PATH (same extension model as git and gh).
package main

import (
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
	"sort"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

var version = "dev"

// tools maps each tool to its GitHub repository.
var tools = map[string]string{
	"tuner": "RhythmShahriar/tuner",
}

// aliases are short names for tools.
var aliases = map[string]string{"t": "tuner"}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(stdout)
		return 0
	}
	switch args[0] {
	case "--version", "version":
		fmt.Fprintln(stdout, "rhythm", version)
		return 0
	case "update":
		return cmdUpdate(args[1:], stdout, stderr)
	case "install":
		return cmdInstall(args[1:], stdout, stderr)
	}

	tool := args[0]
	if full, ok := aliases[tool]; ok {
		tool = full
	}
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		if _, known := tools[tool]; known {
			fmt.Fprintf(stderr, "rhythm: %s isn't installed\n  install it with: rhythm install %s\n", tool, tool)
		} else {
			fmt.Fprintf(stderr, "rhythm: unknown tool %q\n  run 'rhythm help' to see the tools\n", tool)
		}
		return 1
	}

	cmd := exec.Command(bin, args[1:]...) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, kit.Banner("rhythm", version))
	fmt.Fprint(w, `Usage:
  rhythm <tool> [args]

Tools:
  tuner (t)  WordPress & PHP security scanner

Commands:
  rhythm install <tool> [--dir <dir>]   download a tool
  rhythm update [--check]               update rhythm, installed tools and their data
  rhythm version
`)
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

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "install directory (default: next to rhythm)")
	pos, err := parseInterspersed(flags, args)
	if err != nil || len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: rhythm install <tool> [--dir <dir>]")
		return 2
	}
	tool := pos[0]
	if full, ok := aliases[tool]; ok {
		tool = full
	}
	repo, ok := tools[tool]
	if !ok {
		fmt.Fprintf(stderr, "rhythm: unknown tool %q; known tools: %s\n", tool, strings.Join(toolNames(), ", "))
		return 2
	}
	if *dir == "" {
		self, err := selfPath()
		if err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
		*dir = filepath.Dir(self)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	u := updater(repo, "rhythm-"+tool, "")
	rel, err := u.Latest(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	dest := filepath.Join(*dir, exe("rhythm-"+tool))
	if err := u.Install(ctx, rel, dest); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
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

// cmdUpdate updates rhythm itself, then each installed tool, then each
// tool's data (e.g. Tuner's vulnerability database).
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "only report available updates")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	status := 0

	type target struct {
		name, repo, binary, current, path string
	}
	var targets []target
	if self, err := selfPath(); err == nil {
		targets = append(targets, target{"rhythm", "RhythmShahriar/rhythm", "rhythm", version, self})
	}
	for _, tool := range toolNames() {
		bin, err := exec.LookPath("rhythm-" + tool)
		if err != nil {
			continue
		}
		targets = append(targets, target{"rhythm-" + tool, tools[tool], "rhythm-" + tool, toolVersion(bin), bin})
	}

	for _, t := range targets {
		if !update.IsRelease(t.current) {
			fmt.Fprintf(stdout, "%-18s %s (development build, not updated)\n", t.name, t.current)
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		rel, err := u.Latest(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "%-18s %v\n", t.name, err)
			status = 1
			continue
		}
		if !update.Newer(t.current, rel.Version) {
			fmt.Fprintf(stdout, "%-18s %s (up to date)\n", t.name, t.current)
			continue
		}
		if *checkOnly {
			fmt.Fprintf(stdout, "%-18s %s → %s available\n", t.name, t.current, rel.Version)
			continue
		}
		if err := u.Install(ctx, rel, t.path); err != nil {
			fmt.Fprintf(stderr, "%-18s %v\n", t.name, err)
			status = 1
			continue
		}
		fmt.Fprintf(stdout, "%-18s %s → %s ✔\n", t.name, t.current, rel.Version)
	}

	if !*checkOnly {
		for _, t := range targets {
			if t.name == "rhythm" {
				continue // rhythm has no data of its own; tools refresh theirs
			}
			cmd := exec.CommandContext(ctx, t.path, "update") //nolint:gosec // our own tool binaries
			cmd.Stdout, cmd.Stderr = stdout, stderr
			if err := cmd.Run(); err != nil {
				status = 1
			}
		}
	}
	return status
}

// toolVersion asks a tool binary for its version ("rhythm-tuner 0.2.0").
func toolVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // our own tool binaries
	if err != nil {
		return "unknown"
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "unknown"
	}
	return f[len(f)-1]
}

// parseInterspersed lets flags come before or after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func toolNames() []string {
	var names []string
	for t := range tools {
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}
