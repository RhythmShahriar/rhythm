// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/internal/lock"
	"github.com/RhythmShahriar/rhythm/kit/cli"
)

func complete(t *testing.T, words ...string) []string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(append([]string{cli.CompleteCommand}, words...), &out, &errb); code != 0 || errb.Len() > 0 {
		t.Fatalf("rhythm __complete %q: exit %d, stderr %q", words, code, errb.String())
	}
	return strings.Fields(out.String())
}

// completeCommands is fixedCommands plus completion and a command with
// flags in both help formats, so flag completion has exact answers.
func completeCommands(t *testing.T) {
	t.Helper()
	fixedCommands(t)
	commands["completion"] = command{name: "completion", run: cmdCompletion}
	commands[cli.CompleteCommand] = command{name: cli.CompleteCommand, run: cmdComplete, hidden: true}
	commands["install"] = command{name: "install", run: func(args []string, _, stderr io.Writer) int {
		fs := cli.NewFlagSet("install", "Usage: rhythm install <tool>...", stderr)
		fs.String("dir", "", "install into `dir`")
		fs.Bool("locked", false, "install what rhythm.lock says")
		if _, err := cli.ParseArgs(fs, args); err != nil {
			return 2
		}
		return 0
	}}
}

func TestCompleteWords(t *testing.T) {
	completeCommands(t)
	t.Setenv("PATH", t.TempDir())
	cases := []struct {
		words []string
		want  []string
	}{
		{[]string{"tu"}, []string{"tuner"}},
		{[]string{"s"}, []string{"sc", "soundcheck"}},
		{[]string{"__"}, nil}, // hidden
		{[]string{"help", "so"}, []string{"soundcheck"}},
		{[]string{"help", "i"}, []string{"install"}},
		{[]string{"help", "tuner", ""}, nil},
		{[]string{"install", ""}, []string{"echo", "soundcheck", "tuner"}},
		{[]string{"install", "tuner", "s"}, []string{"soundcheck"}}, // install takes several
		{[]string{"install", "--"}, []string{"--dir", "--help", "--locked"}},
		{[]string{"install", "--l"}, []string{"--locked"}},
		{[]string{"install", "--dir", ""}, nil}, // the shell completes the folder
		{[]string{"install", "--dir=x", "t"}, []string{"tuner"}},
		{[]string{"install", "--locked", "e"}, []string{"echo"}},
		{[]string{"completion", ""}, []string{"bash", "fish", "powershell", "zsh"}},
		{[]string{"completion", "z"}, []string{"zsh"}},
		{[]string{"completion", "zsh", ""}, nil},
		{[]string{"version", ""}, nil},
		{[]string{"nope", ""}, nil},
		{[]string{"/../evil", ""}, nil},
	}
	for _, c := range cases {
		if got := complete(t, c.words...); !slices.Equal(got, c.want) {
			t.Errorf("__complete %q = %q, want %q", c.words, got, c.want)
		}
	}
}

// TestCompleteListsEverything checks the real command table and the
// registry: the first word completes to every visible command, tool,
// alias and third-party tool.
func TestCompleteListsEverything(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stub(t, dir, "zap", "exit 0")
	got := complete(t, "")
	if len(complete(t)) != len(got) {
		t.Error("no words should mean one empty word")
	}
	want := []string{"zap"}
	for _, c := range commands {
		if !c.hidden {
			want = append(want, c.name)
		}
	}
	for _, tool := range registry {
		want = append(want, tool.Name)
		want = append(want, tool.Aliases...)
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("__complete '' lacks %q: %q", w, got)
		}
	}
	if slices.Contains(got, cli.CompleteCommand) {
		t.Errorf("__complete '' lists itself")
	}
}

// toolStub writes a rhythm-tuner that answers like a kit/cli.Main tool:
// version --json with a kit_version and __complete by logging how it
// was called to log and printing two candidates.
func toolStub(t *testing.T, dir, log string) {
	t.Helper()
	stub(t, dir, "tuner", fmt.Sprintf(`case "$1" in
version) echo '{"schema":"rhythm-version/v1","name":"rhythm-tuner","version":"1.0.0","commit":"","kit_version":"v0.7.0"}' ;;
__complete) shift; printf '%%s\n' "$RHYTHM_INVOKED_AS" "$@" > %q; echo scan; echo schema; echo "oops" >&2 ;;
*) exit 9 ;;
esac`, log))
}

func TestCompleteForwardsToTool(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	log := filepath.Join(dir, "log")
	toolStub(t, dir, log)

	if got := complete(t, "tuner", "sc"); !slices.Equal(got, []string{"scan", "schema"}) {
		t.Errorf("__complete tuner sc = %q", got)
	}
	if got, _ := os.ReadFile(log); string(got) != "rhythm tuner\nsc\n" {
		t.Errorf("the tool was called with %q", got)
	}
	// An alias, a later word and an empty current word all reach the tool.
	complete(t, "t", "scan", "")
	if got, _ := os.ReadFile(log); string(got) != "rhythm t\nscan\n\n" {
		t.Errorf("via the alias the tool was called with %q", got)
	}
}

// TestCompleteSkipsOtherTools checks that a tool not built on cli.Main is
// never sent __complete, which it could take for a site to check.
func TestCompleteSkipsOtherTools(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	called := filepath.Join(dir, "called")
	for name, version := range map[string]string{
		"tuner":      `echo "rhythm-tuner 0.3.0"`,                                        // plain version
		"soundcheck": `echo '{"schema":"rhythm-version/v1","name":"rhythm-soundcheck"}'`, // no kit_version
		"echo":       `exit 2`,
	} {
		stub(t, dir, name, fmt.Sprintf(`if [ "$1" = version ]; then %s; else : > %q; echo crawled; fi`, version, called))
	}
	for _, tool := range []string{"tuner", "soundcheck", "echo", "missing"} {
		if got := complete(t, tool, ""); len(got) > 0 {
			t.Errorf("__complete %s '' = %q, want nothing", tool, got)
		}
	}
	if _, err := os.Stat(called); err == nil {
		t.Error("a tool without __complete was run with it")
	}
}

// TestCompleteProbesOnlyKnownTools checks that a rhythm-* program that is
// neither in the registry nor an installed extension is never run, not
// even with version --json, while an installed extension is asked.
func TestCompleteProbesOnlyKnownTools(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("RHYTHM_CONFIG_DIR", filepath.Join(dir, "config"))
	called := filepath.Join(dir, "called")
	stub(t, dir, "zap", fmt.Sprintf(`: > %q
case "$1" in
version) echo '{"schema":"rhythm-version/v1","name":"rhythm-zap","version":"1.0.0","commit":"","kit_version":"v0.7.0"}' ;;
__complete) echo spider ;;
esac`, called))

	if got := complete(t, "zap", ""); len(got) > 0 {
		t.Errorf("__complete zap '' = %q, want nothing", got)
	}
	if _, err := os.Stat(called); err == nil {
		t.Error("a program that is no known tool was run")
	}

	store, err := lock.OpenExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add(lock.Extension{Name: "zap", Repo: "someone/zap"}); err != nil {
		t.Fatal(err)
	}
	if got := complete(t, "zap", ""); !slices.Equal(got, []string{"spider"}) {
		t.Errorf("__complete zap '' with zap installed = %q, want [spider]", got)
	}
}

func TestCompleteTimesOut(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep command")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stub(t, dir, "tuner", sleep+" 5")
	saved := completeTimeout
	completeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { completeTimeout = saved })
	start := time.Now()
	if got := complete(t, "tuner", ""); len(got) > 0 {
		t.Errorf("a hung tool completed %q", got)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("completion waited %v for a hung tool", d)
	}
}

func TestCompletionCommand(t *testing.T) {
	cases := []struct {
		args               []string
		code               int
		inStdout, inStderr string
	}{
		{[]string{"bash"}, 0, "complete -o default -F _rhythm_complete rhythm\n", ""},
		{[]string{"zsh"}, 0, "#compdef rhythm\n", ""},
		{[]string{"fish"}, 0, "complete -c rhythm -f -a '(__rhythm_complete)'\n", ""},
		{[]string{"powershell"}, 0, "Register-ArgumentCompleter -Native -CommandName 'rhythm', 'rhythm.exe'", ""},
		{[]string{"--help"}, 0, "Usage: rhythm completion bash|zsh|fish|powershell\n", ""},
		{nil, 2, "", "usage: rhythm completion bash|zsh|fish|powershell\n"},
		{[]string{"bash", "zsh"}, 2, "", "usage: rhythm completion"},
		{[]string{"zhs"}, 2, "", "rhythm: unknown shell \"zhs\"; want bash, zsh, fish or powershell\n  did you mean: rhythm completion zsh\n"},
		{[]string{"--nope"}, 2, "", "flag provided but not defined"},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := run(append([]string{"completion"}, c.args...), &out, &errb)
		if code != c.code || !strings.Contains(out.String(), c.inStdout) || !strings.Contains(errb.String(), c.inStderr) {
			t.Errorf("rhythm completion %q: exit %d\nstdout %q\nstderr %q", c.args, code, out.String(), errb.String())
		}
	}
}

// TestCompletionScripts checks each script asks rhythm __complete and
// passes the shell's syntax check where that shell is installed: pwsh and,
// on Windows, Windows PowerShell 5.1 too.
func TestCompletionScripts(t *testing.T) {
	syntax := map[string][][]string{
		"bash":       {{"bash", "-n"}},
		"zsh":        {{"zsh", "-n"}},
		"fish":       {{"fish", "--no-execute"}},
		"powershell": {{"pwsh"}, {"powershell"}},
	}
	for _, shell := range shellNames {
		t.Run(shell, func(t *testing.T) {
			script := shells[shell]
			if !strings.Contains(script, "rhythm __complete") {
				t.Error("script doesn't call rhythm __complete")
			}
			if !strings.HasSuffix(script, "\n") || strings.Contains(script, "\r") {
				t.Error("script should end in a newline and use LF")
			}
			file := filepath.Join(t.TempDir(), "rhythm."+shell)
			if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			checked := false
			for _, argv := range syntax[shell] {
				if _, err := exec.LookPath(argv[0]); err != nil {
					continue
				}
				checked = true
				if shell == "powershell" {
					argv = append(argv, "-NoProfile", "-NonInteractive", "-Command", psParse(file))
				} else {
					argv = append(argv, file)
				}
				if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil { //nolint:gosec // the test's own shells
					t.Errorf("%s rejects the script: %v\n%s", argv[0], err, out)
				}
			}
			if !checked {
				t.Skipf("%s isn't installed", syntax[shell][0][0])
			}
		})
	}
}

// psParse is a PowerShell command that parses file and prints each syntax
// error. The path is in the command itself: -Command joins any arguments
// after it into the command text rather than passing them as $args.
func psParse(file string) string {
	quoted := "'" + strings.ReplaceAll(file, "'", "''") + "'"
	return "$e = $null; $null = [System.Management.Automation.Language.Parser]::ParseFile(" + quoted +
		", [ref]$null, [ref]$e); if ($e) { $e | ForEach-Object { $_.ToString() }; exit 1 }"
}

// TestBashScriptCompletes runs the bash script against a stand-in
// rhythm that logs its arguments, so the words reach __complete as typed
// (the empty word under the cursor included) and the script filters
// them by the word under the cursor.
func TestBashScriptCompletes(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash isn't installed")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	script := filepath.Join(dir, "rhythm.bash")
	if err := os.WriteFile(script, []byte(bashCompletion), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "rhythm")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\nprintf 'tuner\\ntools\\nsoundcheck\\n'\n", log)
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil { //nolint:gosec // the stand-in must be executable
		t.Fatal(err)
	}
	cases := []struct {
		words, want, args string
	}{
		{`(rhythm install t)`, "tuner\ntools\n", "__complete\ninstall\nt\n"},
		{`(rhythm install "")`, "tuner\ntools\nsoundcheck\n", "__complete\ninstall\n\n"},
		{`(rhythm x)`, "", "__complete\nx\n"},
	}
	for _, c := range cases {
		cmd := exec.Command("bash", "--norc", "--noprofile", "-c", fmt.Sprintf(
			`source %q; COMP_WORDS=%s; COMP_CWORD=$((${#COMP_WORDS[@]}-1)); _rhythm_complete; for w in "${COMPREPLY[@]}"; do printf '%%s\n' "$w"; done`,
			script, c.words))
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil || string(out) != c.want {
			t.Errorf("%s: %v, completed %q, want %q", c.words, err, out, c.want)
		}
		if got, _ := os.ReadFile(log); string(got) != c.args {
			t.Errorf("%s: rhythm got %q, want %q", c.words, got, c.args)
		}
	}
}

// TestBashScriptDoesNotExpandCandidates checks that a candidate is
// offered as the literal text the tool printed: a $(...) in it must not
// run, as it would if the script passed it through compgen -W.
func TestBashScriptDoesNotExpandCandidates(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash isn't installed")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	script := filepath.Join(dir, "rhythm.bash")
	if err := os.WriteFile(script, []byte(bashCompletion), 0o600); err != nil {
		t.Fatal(err)
	}
	candidate := "$(touch " + marker + ")"
	fake := filepath.Join(dir, "rhythm")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %s snap1 '$HOME' '`id`'\n", shellQuote(candidate))
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil { //nolint:gosec // the stand-in must be executable
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "--norc", "--noprofile", "-c", fmt.Sprintf(
		`source %q; COMP_WORDS=(rhythm soundcheck ""); COMP_CWORD=2; _rhythm_complete; for w in "${COMPREPLY[@]}"; do printf '%%s\n' "$w"; done`,
		script))
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	want := candidate + "\nsnap1\n$HOME\n`id`\n"
	if err != nil || string(out) != want {
		t.Errorf("completed %q (%v), want %q", out, err, want)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a candidate ran as a command")
	}
}

// shellQuote quotes s for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// TestZshScriptCompletes runs the zsh function outside the completion
// system, with compadd and _files stood in for, against the same kind of
// logging rhythm as the bash test.
func TestZshScriptCompletes(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh isn't installed")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	script := filepath.Join(dir, "_rhythm")
	if err := os.WriteFile(script, []byte(zshCompletion), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "rhythm")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n[ \"$2\" = none ] || printf 'tuner\\ntools\\n'\n", log)
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil { //nolint:gosec // the stand-in must be executable
		t.Fatal(err)
	}
	cases := []struct {
		words, want, args string
	}{
		{`(rhythm install t)`, "tuner\ntools\n", "__complete\ninstall\nt\n"},
		{`(rhythm install "")`, "tuner\ntools\n", "__complete\ninstall\n\n"},
		{`(rhythm none "")`, "FILES\n", "__complete\nnone\n\n"},
	}
	for _, c := range cases {
		cmd := exec.Command("zsh", "-f", "-c", fmt.Sprintf(
			`compdef() { :; }; compadd() { shift; print -l -- "$@"; }; _files() { print FILES; }; source %q; words=%s; CURRENT=${#words}; _rhythm`,
			script, c.words))
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil || string(out) != c.want {
			t.Errorf("%s: %v, completed %q, want %q", c.words, err, out, c.want)
		}
		if got, _ := os.ReadFile(log); string(got) != c.args {
			t.Errorf("%s: rhythm got %q, want %q", c.words, got, c.args)
		}
	}
}

// TestCompleteRealCommands completes the flags and names of rhythm's own
// install, update, list, uninstall, doctor and lock, read from their
// real --help.
func TestCompleteRealCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("RHYTHM_CONFIG_DIR", filepath.Join(dir, "config"))
	if err := lock.ExtensionsAt(filepath.Join(dir, "config", lock.ExtensionsFile)).Add(lock.Extension{Name: "zap", Repo: "o/zap"}); err != nil {
		t.Fatal(err)
	}
	has := func(words []string, want ...string) {
		t.Helper()
		got := complete(t, words...)
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("__complete %q = %q, missing %q", words, got, w)
			}
		}
	}
	has([]string{"update", "--"}, "--check", "--json", "--lock", "--min-age", "--no-data", "--quiet", "--rollback", "--self-only", "--skip-signature", "--cache", "--to")
	has([]string{"install", "--"}, "--dir", "--defaults", "--dry-run", "--force", "--locked", "--min-age", "--quiet", "--skip-signature", "--cache")
	has([]string{"list", "--"}, "--json", "--offline")
	has([]string{"uninstall", "--"}, "--all", "--purge")
	has([]string{"doctor", "--"}, "--json", "--offline")
	has([]string{"lock", "--"}, "--no-data")
	has([]string{"update", ""}, "rhythm", "tuner", "soundcheck", "echo", "zap")
	has([]string{"update", "tuner", "s"}, "soundcheck") // update takes several
	has([]string{"uninstall", "tuner", "e"}, "echo")
	has([]string{"install", "--"}, "--from") // this rhythm reads bundles
	has([]string{"verify", "--"}, "--json")
	has([]string{"verify", "tuner", "e"}, "echo") // verify takes several
	has([]string{"bundle", "--"}, "--locked", "--output", "--platform")
	has([]string{"ext", ""}, "install", "list", "remove", "search")
	if got := complete(t, "list", ""); len(got) != 0 {
		t.Errorf("list takes no names, got %q", got)
	}
}

// TestCompletionHelpLines keeps the help paragraph wrapped like the
// others: no prose line runs past 72 columns.
func TestCompletionHelpLines(t *testing.T) {
	for _, line := range strings.Split(completionSynopsis, "\n") {
		if !strings.HasPrefix(line, "  ") && len(line) > 72 {
			t.Errorf("line of %d columns: %q", len(line), line)
		}
	}
}
