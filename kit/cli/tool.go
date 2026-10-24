// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/RhythmShahriar/rhythm/kit"
	"github.com/RhythmShahriar/rhythm/kit/config"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Schemas of the JSON that Main prints. The router and CI read these, so a
// change that isn't backwards compatible needs a new version.
const (
	VersionSchema = "rhythm-version/v1" // version --json
	DataSchema    = "rhythm-data/v1"    // update --json
)

// OfflineEnv names the variable that, when set to anything, keeps every
// tool off the network for its own housekeeping, such as the release check.
const OfflineEnv = "RHYTHM_OFFLINE"

// Tool describes a Rhythm tool for Main, which wires up what every tool
// shares: help, version, the data update, exit codes, Ctrl-C, the release
// notice and the first-run note. A tool's main is then:
//
//	func main() { os.Exit(cli.Main(cli.Tool{Name: "rhythm-tuner", ...})) }
type Tool struct {
	Name     string // binary name, "rhythm-tuner"; the first word of `version`
	Version  string // release version, "0.3.0"; empty means "dev"
	Commit   string // short commit hash, shown by `version` when set
	Summary  string // one line under the banner in help
	Commands []Command

	// Update refreshes the tool's data (a vulnerability database, a
	// browser) and reports what it now has. Progress goes to w. When nil,
	// `update` is an ordinary command name the tool may use.
	Update func(ctx context.Context, w io.Writer) ([]DataPack, error)

	// UpdatePinned, when set with Update, brings the data to the versions
	// `update --pin name=version,...` names (rhythm install --locked
	// passes the versions rhythm.lock records) and reports what it now
	// has. pins maps a pack name to its version; packs it doesn't name
	// are refreshed as Update would. Main checks the report and exits 3
	// when a pinned pack is missing or has another version, so a tool
	// that can't get a version may return what it has. When nil,
	// --pin is a usage error (exit 2), which tells the router the tool
	// can't pin.
	UpdatePinned func(ctx context.Context, w io.Writer, pins map[string]string) ([]DataPack, error)

	// Offline reports whether the user asked the tool to stay off the
	// network, so Main skips the release check. It runs before the command
	// parses its flags; HasFlag reads a flag such as --offline from the
	// arguments directly. RHYTHM_OFFLINE does the same for every tool.
	Offline func() bool

	// Repo is the GitHub owner/name the release notice checks. It
	// defaults to RhythmShahriar/ plus Name without "rhythm-".
	Repo string

	// Default is the command to run when the first argument isn't a
	// command name, so `rhythm-soundcheck example.com` can mean
	// `rhythm-soundcheck check example.com`. Empty means such an argument
	// is an unknown command.
	Default string

	// FirstRun, when set, is called once per machine before the first
	// command runs, to print a one-time note such as the authorised-use
	// reminder. FirstRunKey names the marker (see config.FirstRun) and
	// defaults to Name + "-first-run".
	FirstRun    func(stderr io.Writer)
	FirstRunKey string
}

// Command is one subcommand of a Tool.
type Command struct {
	Name    string // what the user types after the tool name
	Args    string // argument synopsis for help, "<site> [flags]"
	Summary string // one line for help
	Hidden  bool   // left out of help, still runs

	// Run does the work. ctx is canceled on Ctrl-C or SIGTERM. Return nil
	// for success, ErrFindings when the run found what --fail-on asked
	// for, a UsageError for bad arguments (exit 2), an *ExitError for any
	// other exit code and any other error when the run failed (exit 3).
	// cli.ErrHelp, as ParseArgs returns it, exits 0.
	//
	// `help <command>` runs Run with the single argument --help, so Run
	// must answer --help by printing its usage and returning ErrHelp
	// without doing any of its work. ParseArgs on a NewFlagSet does that.
	Run func(ctx context.Context, args []string, stdout, stderr io.Writer) error

	// Flags, when set, returns a fresh flag set with the flags Run
	// parses, usually the same NewFlagSet call. Shell completion offers
	// its flags on Tab without running the command; a command without
	// Flags completes only --help, so a key press never runs Run.
	Flags func() *flag.FlagSet

	// Complete, when set, offers values for the word being typed after
	// the command's name in shell completion: args are the words after
	// the name, the last one being the word under the cursor (it may be
	// empty). Flags are completed from the command's --help without it.
	Complete func(args []string) []string
}

// DataPack is one piece of data a tool downloads and keeps, as reported
// by `update --json`.
type DataPack struct {
	Name    string `json:"name"`    // "wpvulnerability"
	Version string `json:"version"` // the data's own version or date
	SHA256  string `json:"sha256"`  // hex digest of what is on disk

	// Pinned is set by Main for a pack update --pin named, so the
	// report shows which versions were asked for. Tools leave it false.
	Pinned bool `json:"pinned,omitempty"`
}

// ExitError ends a command with a chosen exit code. Main prints Err, when
// it isn't nil and exits with Code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return "exit status " + strconv.Itoa(e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// ErrFindings is what a command returns when the run finished and found
// something at or above --fail-on. Main exits 1 without printing anything.
var ErrFindings error = &ExitError{Code: ExitFindings}

// noticeWait is how long Main waits at the end of a command for the
// release check to finish. It is short so a slow network never holds up
// the exit; the check caches its answer for a day, so a missed notice
// shows on a later run.
var noticeWait = 200 * time.Millisecond

// Main runs the tool with os.Args and returns the exit code for os.Exit.
//
// Besides the tool's commands it answers help, -h and --help, version and
// --version (`<name> <version>`; with --json the rhythm-version/v1
// document) and update when Update is set (with --json the
// rhythm-data/v1 document; exit 3 when it fails; --pin when UpdatePinned
// is set, exit 3 when a pinned version can't be had). Commands run under a
// context canceled by Ctrl-C or SIGTERM, which exits 130. While a command
// runs, Main checks for a newer release in the background, unless the
// tool is offline and prints the notice at the end. The hidden
// __complete command (CompleteCommand) answers shell completion.
func Main(t Tool) int {
	return t.main(os.Args[1:], os.Stdout, os.Stderr)
}

func (t Tool) main(args []string, stdout, stderr io.Writer) int {
	t.check()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	prog := Name()
	if prog == "" {
		prog = t.Name
	}

	if len(args) == 0 {
		t.usage(stdout, prog)
		return ExitOK
	}
	switch name := args[0]; {
	case name == "-h" || name == "--help":
		t.usage(stdout, prog)
		return ExitOK
	case name == "help":
		return t.help(ctx, prog, args[1:], stdout, stderr)
	case name == "version" || name == "--version":
		return t.version(prog, args[1:], stdout, stderr)
	case name == "update" && t.Update != nil:
		return t.update(ctx, prog, args[1:], stdout, stderr)
	case name == CompleteCommand:
		return t.complete(args[1:], stdout)
	}

	c, ok := t.command(args[0])
	switch {
	case ok:
		args = args[1:]
	case t.Default != "":
		c, _ = t.command(t.Default)
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n", prog, args[0])
		if s := Suggest(args[0], t.names()); s != "" {
			fmt.Fprintf(stderr, "  did you mean: %s %s\n", prog, s)
		}
		fmt.Fprintf(stderr, "  run '%s help' to see the commands\n", prog)
		return ExitUsage
	}

	if t.FirstRun != nil {
		key := t.FirstRunKey
		if key == "" {
			key = t.Name + "-first-run"
		}
		if config.FirstRun(key) {
			t.FirstRun(stderr)
		}
	}
	var notice <-chan string
	if !t.offline() {
		notice = update.Notice(ctx, t.updater())
	}

	err := c.Run(ctx, args, stdout, stderr)
	if ctx.Err() != nil {
		// Only the signal cancels ctx before stop runs. A command that
		// returned nil after Ctrl-C still stopped early, so it exits 130
		// either way and never passes as a clean run.
		fmt.Fprintf(stderr, "%s: interrupted\n", prog)
		return ExitInterrupted
	}
	code := exitFor(stderr, prog, c.Name, err)
	if notice != nil {
		select {
		case msg, ok := <-notice:
			if ok {
				fmt.Fprintln(stderr, msg)
			}
		case <-time.After(noticeWait):
		}
	}
	return code
}

// exitFor prints err the way it should be shown and returns its exit code.
func exitFor(stderr io.Writer, prog, cmd string, err error) int {
	var ee *ExitError
	var ue UsageError
	switch {
	case err == nil, errors.Is(err, ErrHelp):
		return ExitOK
	case errors.As(err, &ee):
		if ee.Err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prog, ee.Err)
		}
		return ee.Code
	case errors.As(err, &ue):
		if !ue.Reported {
			fmt.Fprintf(stderr, "%s: %s\n  run '%s help %s' for usage\n", prog, ue.Msg, prog, cmd)
		}
		return ExitUsage
	}
	// A failed run gets 3, never 1, so CI can't mistake it for findings.
	fmt.Fprintf(stderr, "%s: %v\n", prog, err)
	return ExitUnavailable
}

// check panics on a Tool that can't work, a bug every test of the tool's
// main would hit.
func (t Tool) check() {
	reserved := map[string]bool{"help": true, "version": true, CompleteCommand: true}
	if t.Update != nil {
		reserved["update"] = true
	}
	seen := map[string]bool{}
	for _, c := range t.Commands {
		if reserved[c.Name] || seen[c.Name] || c.Name == "" || c.Run == nil {
			panic(fmt.Sprintf("cli: %s: command %q is reserved, repeated or incomplete", t.Name, c.Name))
		}
		seen[c.Name] = true
	}
	if t.Default != "" && !seen[t.Default] {
		panic(fmt.Sprintf("cli: %s: default command %q isn't defined", t.Name, t.Default))
	}
	if t.UpdatePinned != nil && t.Update == nil {
		panic(fmt.Sprintf("cli: %s: UpdatePinned needs Update", t.Name))
	}
}

func (t Tool) command(name string) (Command, bool) {
	for _, c := range t.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// names lists every command a user can type, for did-you-mean.
func (t Tool) names() []string {
	var names []string
	for _, c := range t.Commands {
		if !c.Hidden {
			names = append(names, c.Name)
		}
	}
	if t.Update != nil {
		names = append(names, "update")
	}
	return append(names, "help", "version")
}

func (t Tool) version(prog string, args []string, stdout, stderr io.Writer) int {
	fs := NewFlagSet("version", "Usage: "+prog+" version [--json]\n\nPrints the version.", stderr)
	asJSON := fs.Bool("json", false, "print the "+VersionSchema+" JSON document")
	if code, done := parseOnly(fs, args, stdout); done {
		return code
	}
	v := t.Version
	if v == "" {
		v = "dev"
	}
	if *asJSON {
		return writeJSON(stdout, stderr, prog, struct {
			Schema     string `json:"schema"`
			Name       string `json:"name"`
			Version    string `json:"version"`
			Commit     string `json:"commit"`
			KitVersion string `json:"kit_version"`
		}{VersionSchema, t.Name, v, t.Commit, kit.Version()})
	}
	// The version is always the second word, so scripts and the router
	// can read it without knowing the tool.
	if t.Commit != "" {
		fmt.Fprintf(stdout, "%s %s (commit %s)\n", t.Name, v, t.Commit)
	} else {
		fmt.Fprintf(stdout, "%s %s\n", t.Name, v)
	}
	return ExitOK
}

func (t Tool) update(ctx context.Context, prog string, args []string, stdout, stderr io.Writer) int {
	synopsis := "Usage: " + prog + " update [--json]\n\nDownloads the latest data the tool works from."
	if t.UpdatePinned != nil {
		synopsis = "Usage: " + prog + " update [--json] [--pin name=version,...]\n\n" +
			"Downloads the latest data the tool works from, or with --pin the versions named."
	}
	fs := NewFlagSet("update", synopsis, stderr)
	asJSON := fs.Bool("json", false, "print the "+DataSchema+" JSON document")
	var pinSpec string
	if t.UpdatePinned != nil {
		fs.StringVar(&pinSpec, "pin", "", "download these data versions, as name=version,...")
	}
	if code, done := parseOnly(fs, args, stdout); done {
		return code
	}
	pins, err := ParsePins(pinSpec)
	if err != nil {
		fmt.Fprintf(stderr, "%s: --pin: %v\n", prog, err)
		return ExitUsage
	}
	progress := stdout
	if *asJSON {
		progress = stderr // stdout carries only the JSON
	}
	var packs []DataPack
	if len(pins) > 0 {
		packs, err = t.UpdatePinned(ctx, progress, pins)
	} else {
		packs, err = t.Update(ctx, progress)
	}
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "%s: update interrupted\n", prog)
		return ExitInterrupted
	}
	if err == nil {
		err = checkPins(packs, pins)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: update failed: %v\n", prog, err)
		return ExitUnavailable
	}
	if packs == nil {
		packs = []DataPack{} // "data": [] rather than null
	}
	if *asJSON {
		return writeJSON(stdout, stderr, prog, struct {
			Schema string     `json:"schema"`
			Data   []DataPack `json:"data"`
		}{DataSchema, packs})
	}
	for _, p := range packs {
		fmt.Fprintf(stdout, "%s %s\n", p.Name, p.Version)
	}
	return ExitOK
}

// ParsePins reads the value of update --pin, "checks=2026.10.20,vulns=2026-10-24",
// into a map from pack name to version. An empty value gives no pins.
func ParsePins(spec string) (map[string]string, error) {
	if spec == "" {
		return nil, nil
	}
	pins := map[string]string{}
	for _, part := range strings.Split(spec, ",") {
		name, ver, ok := strings.Cut(part, "=")
		name, ver = strings.TrimSpace(name), strings.TrimSpace(ver)
		switch {
		case !ok || name == "" || ver == "":
			return nil, fmt.Errorf("%q isn't name=version", part)
		case pins[name] != "":
			return nil, fmt.Errorf("%s is pinned twice", name)
		}
		pins[name] = ver
	}
	return pins, nil
}

// checkPins marks the packs pins named and reports the first pin the
// tool couldn't meet, by name order.
func checkPins(packs []DataPack, pins map[string]string) error {
	names := make([]string, 0, len(pins))
	for name := range pins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got := ""
		for i := range packs {
			if packs[i].Name == name {
				got = packs[i].Version
				packs[i].Pinned = true
			}
		}
		switch {
		case got == "":
			return fmt.Errorf("no %s data to pin at %s", name, pins[name])
		case got != pins[name]:
			return fmt.Errorf("%s %s isn't available (got %s)", name, pins[name], got)
		}
	}
	return nil
}

// parseOnly parses the flags of a built-in command that takes no
// arguments. done is true when the command should exit with code now:
// after --help (printed to stdout, as help asked for is not an error) or
// a usage error.
func parseOnly(fs *flag.FlagSet, args []string, stdout io.Writer) (code int, done bool) {
	if HasFlag(args, "h") || HasFlag(args, "help") {
		fs.SetOutput(stdout)
		fs.Usage()
		return ExitOK, true
	}
	if err := fs.Parse(args); err != nil {
		return ExitUsage, true
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(fs.Output(), "unexpected argument %q\n", args[len(args)-fs.NArg()])
		fs.Usage()
		return ExitUsage, true
	}
	return 0, false
}

func writeJSON(stdout, stderr io.Writer, prog string, v any) int {
	if err := json.NewEncoder(stdout).Encode(v); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", prog, err)
		return ExitUnavailable
	}
	return ExitOK
}

// help answers `help` and `help <command>`. A command's help is its own
// --help, so the flags it lists are the ones it parses.
func (t Tool) help(ctx context.Context, prog string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" {
		t.usage(stdout, prog)
		return ExitOK
	}
	switch args[0] {
	case "version":
		return t.version(prog, []string{"--help"}, stdout, stderr)
	case "update":
		if t.Update != nil {
			return t.update(ctx, prog, []string{"--help"}, stdout, stderr)
		}
	}
	c, ok := t.command(args[0])
	if !ok {
		fmt.Fprintf(stderr, "%s: unknown command %q\n", prog, args[0])
		if s := Suggest(args[0], t.names()); s != "" {
			fmt.Fprintf(stderr, "  did you mean: %s help %s\n", prog, s)
		}
		return ExitUsage
	}
	return exitFor(stderr, prog, c.Name, c.Run(ctx, []string{"--help"}, stdout, stderr))
}

func (t Tool) usage(w io.Writer, prog string) {
	v := t.Version
	if v == "" {
		v = "dev"
	}
	fmt.Fprintln(w, kit.Banner(t.Name, v))
	if t.Summary != "" {
		fmt.Fprintln(w, t.Summary)
	}
	fmt.Fprintf(w, "\nUsage:\n  %s <command> [args]\n\nCommands:\n", prog)
	type row struct{ left, summary string }
	var rows []row
	for _, c := range t.Commands {
		if !c.Hidden {
			rows = append(rows, row{strings.TrimSpace(c.Name + " " + c.Args), c.Summary})
		}
	}
	if t.Update != nil {
		rows = append(rows, row{"update [--json]", "download the latest data"})
	}
	rows = append(rows, row{"version [--json]", "print the version"})
	width := 0
	for _, r := range rows {
		width = max(width, len(r.left))
	}
	for _, r := range rows {
		fmt.Fprintf(w, "  %-*s   %s\n", width, r.left, r.summary)
	}
	fmt.Fprintf(w, `
Run '%s help <command>' for a command's flags.

Exit codes:
  0     success
  1     findings at or above --fail-on
  2     usage error
  3     the run failed, or a site, tool or service couldn't be reached
  130   interrupted
`, prog)
}

// offline reports whether the release check must not run.
func (t Tool) offline() bool {
	return os.Getenv(OfflineEnv) != "" || (t.Offline != nil && t.Offline())
}

func (t Tool) updater() *update.Updater {
	repo := t.Repo
	if repo == "" {
		repo = "RhythmShahriar/" + strings.TrimPrefix(t.Name, "rhythm-")
	}
	return &update.Updater{
		Repo:      repo,
		Binary:    t.Name,
		Current:   t.Version,
		UserAgent: fmt.Sprintf("%s/%s (+https://rythm.sh)", t.Name, t.Version),
		API:       os.Getenv("RHYTHM_GITHUB_API"), // a mirror or GitHub Enterprise
	}
}

// HasFlag reports whether args turn on the boolean flag name, written as
// --name, -name or --name=<bool>, before any "--". The last use wins, as
// in the flag package. It is for decisions made before a command parses
// its own flags, such as Tool.Offline.
func HasFlag(args []string, name string) bool {
	on := false
	for _, a := range args {
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		k, v, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-"), "=")
		if k != name {
			continue
		}
		if !hasValue {
			on = true
			continue
		}
		b, err := strconv.ParseBool(v)
		on = err == nil && b
	}
	return on
}
