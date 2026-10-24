// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/internal/lock"
	"github.com/RhythmShahriar/rhythm/kit/cli"
)

func init() {
	register(command{
		name:    "completion",
		args:    "bash|zsh|fish|powershell",
		summary: "print a shell completion script",
		run:     cmdCompletion,
	})
	register(command{
		name:   cli.CompleteCommand,
		run:    cmdComplete,
		hidden: true,
	})
}

const completionSynopsis = `Usage: rhythm completion bash|zsh|fish|powershell

Prints a script that completes rhythm's commands, tools and flags as
well as each tool's own commands and flags. Load it from your shell's
start-up file:
  bash        eval "$(rhythm completion bash)"            in ~/.bashrc
  zsh         rhythm completion zsh > "${fpath[1]}/_rhythm"
  fish        rhythm completion fish > ~/.config/fish/completions/rhythm.fish
  powershell  rhythm completion powershell | Out-String | Invoke-Expression   in $PROFILE`

// shells maps each shell `rhythm completion` supports to its script.
// The scripts hold no names of their own: every key press asks
// `rhythm __complete`, so tools installed later, third-party tools and
// new commands complete without regenerating the script.
var shells = map[string]string{
	"bash":       bashCompletion,
	"zsh":        zshCompletion,
	"fish":       fishCompletion,
	"powershell": powershellCompletion,
}

// shellNames lists the shells in the order help names them.
var shellNames = []string{"bash", "zsh", "fish", "powershell"}

func cmdCompletion(args []string, stdout, stderr io.Writer) int {
	fs := cli.NewFlagSet("completion", completionSynopsis, stderr)
	if cli.HasFlag(args, "h") || cli.HasFlag(args, "help") {
		fs.SetOutput(stdout)
	}
	pos, err := cli.ParseArgs(fs, args)
	switch {
	case errors.Is(err, cli.ErrHelp):
		return cli.ExitOK
	case err != nil:
		return cli.ExitUsage
	case len(pos) != 1:
		fmt.Fprintln(stderr, "usage: rhythm completion bash|zsh|fish|powershell")
		return cli.ExitUsage
	}
	script, ok := shells[pos[0]]
	if !ok {
		fmt.Fprintf(stderr, "rhythm: unknown shell %q; want bash, zsh, fish or powershell\n", pos[0])
		if s := cli.Suggest(pos[0], shellNames); s != "" {
			fmt.Fprintf(stderr, "  did you mean: rhythm completion %s\n", s)
		}
		return cli.ExitUsage
	}
	fmt.Fprint(stdout, script)
	return cli.ExitOK
}

// cmdComplete answers `rhythm __complete <words...>`, the words after
// "rhythm" with the one under the cursor last, by printing what may come
// next, one per line. It always exits 0 and prints no errors: a shell
// shows nothing rather than a message in the middle of the prompt.
func cmdComplete(args []string, stdout, _ io.Writer) int {
	if len(args) == 0 {
		args = []string{""}
	}
	for _, w := range completeWords(args) {
		fmt.Fprintln(stdout, w)
	}
	return cli.ExitOK
}

func completeWords(words []string) []string {
	cur := words[len(words)-1]
	if len(words) == 1 {
		return cli.FilterPrefix(suggestions(), cur)
	}
	if c, ok := commands[words[0]]; ok && !c.hidden {
		return completeCommand(c, words[1:])
	}
	return completeTool(words[0], words[1:])
}

// positionals completes the arguments of the commands that take a tool
// or a shell. Only the first argument is completed, except for the
// commands in severalNames, which accept several tools and get them all.
var positionals = map[string]func() []string{
	"help":       func() []string { return suggestions() },
	"install":    toolNames,
	"uninstall":  toolNames,
	"update":     updateNames,
	"verify":     updateNames,
	"ext":        func() []string { return []string{"install", "list", "remove", "search"} },
	"completion": func() []string { return shellNames },
}

// severalNames are the commands that take any number of names.
var severalNames = map[string]bool{"install": true, "uninstall": true, "update": true, "verify": true}

// updateNames is what rhythm update takes: rhythm itself, the tools and
// the installed extensions.
func updateNames() []string {
	names := append([]string{selfName}, toolNames()...)
	return append(names, extensionNames()...)
}

// completeCommand completes the words after one of rhythm's commands:
// flags read from the command's --help, then tool or shell names. A word
// after a flag that takes a value gets nothing, so the shell falls back
// to file names (for install --dir).
func completeCommand(c command, args []string) []string {
	cur := args[len(args)-1]
	var flags []cli.HelpFlag
	if c.name != "help" { // help --help would print rhythm's whole usage
		var help bytes.Buffer
		c.run([]string{"--help"}, &help, &help)
		flags = cli.ParseHelpFlags(help.String())
	}
	if strings.HasPrefix(cur, "-") {
		return cli.FilterPrefix(cli.FlagWords(flags), cur)
	}
	if len(args) > 1 && cli.TakesValue(flags, args[len(args)-2]) {
		return nil
	}
	values, ok := positionals[c.name]
	if !ok {
		return nil
	}
	n := 0 // positional arguments before the cursor
	for i, a := range args[:len(args)-1] {
		if !strings.HasPrefix(a, "-") && (i == 0 || !cli.TakesValue(flags, args[i-1])) {
			n++
		}
	}
	if n > 0 && !severalNames[c.name] {
		return nil
	}
	return cli.FilterPrefix(values(), cur)
}

// completeTimeout bounds everything one completion asks of a tool, so a
// hung or slow tool never freezes the shell.
var completeTimeout = 2 * time.Second

// completeTool forwards `rhythm <tool> <args...>` to
// `rhythm-<tool> __complete <args...>` and returns its lines. Only tools
// built on kit/cli.Main answer __complete; any other tool would take it
// for an argument (Soundcheck would check a site called "__complete"),
// so the tool is first asked for `version --json`, which only Main
// prints with a kit_version. Even that probe runs a program, so it is
// sent only to registry tools and installed extensions: some other
// rhythm-* program on PATH may take "version" as something to act on.
func completeTool(name string, args []string) []string {
	if !validToolName(name) {
		return nil
	}
	tool := name
	if info, ok := lookupTool(name); ok {
		tool = info.Name
	} else if !isExtension(name) {
		return nil
	}
	bin, err := exec.LookPath("rhythm-" + tool)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), completeTimeout)
	defer cancel()
	env := toolEnv(os.Environ(), "rhythm "+name)
	if !answersComplete(ctx, bin, env) {
		return nil
	}
	out, err := toolOutput(ctx, bin, env, append([]string{cli.CompleteCommand}, args...)...)
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimRight(l, "\r"); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// isExtension reports whether name is in the installed extensions
// list. A list that can't be read counts as empty, as completion has
// nowhere to report the error.
func isExtension(name string) bool {
	store, err := lock.OpenExtensions()
	if err != nil {
		return false
	}
	list, err := store.List()
	if err != nil {
		return false
	}
	return slices.ContainsFunc(list, func(x lock.Extension) bool { return x.Name == name })
}

// extensionNames returns the names in the installed extensions list,
// none when it can't be read.
func extensionNames() []string {
	store, err := lock.OpenExtensions()
	if err != nil {
		return nil
	}
	list, err := store.List()
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, x := range list {
		names[i] = x.Name
	}
	return names
}

// answersComplete reports whether the tool at bin runs on kit/cli.Main
// and so answers __complete.
func answersComplete(ctx context.Context, bin string, env []string) bool {
	out, err := toolOutput(ctx, bin, env, "version", "--json")
	if err != nil {
		return false
	}
	var v struct {
		Schema     string `json:"schema"`
		KitVersion string `json:"kit_version"`
	}
	return json.Unmarshal(out, &v) == nil && v.Schema == cli.VersionSchema && v.KitVersion != ""
}

// toolOutput runs bin with args and returns its stdout. Its stdin is
// empty and its stderr dropped, as nothing it says may reach the prompt.
func toolOutput(ctx context.Context, bin string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // running rhythm-<tool> from PATH is the extension model
	cmd.Env = env
	// A child the tool started could keep stdout open after the tool
	// is killed; WaitDelay stops Output waiting for it.
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Output()
}

// The scripts pass every word up to the cursor to `rhythm __complete`,
// the word under the cursor last even when it is empty and fall back to
// file names when nothing comes back. The bash script reads the lines
// as they are: compgen -W would expand them again, so a candidate such
// as $(cmd) taken from a file name would run as a command.

const bashCompletion = `# bash completion for rhythm. Load it with:
#   eval "$(rhythm completion bash)"
_rhythm_complete() {
    local cur words cword
    if declare -F _get_comp_words_by_ref >/dev/null 2>&1; then
        _get_comp_words_by_ref -n =: cur words cword
    else
        cur=${COMP_WORDS[COMP_CWORD]}
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi
    local line
    COMPREPLY=()
    while IFS= read -r line; do
        [[ $line == "$cur"* ]] && COMPREPLY+=("$line")
    done < <(rhythm __complete "${words[@]:1:cword}" 2>/dev/null)
    if declare -F __ltrim_colon_completions >/dev/null 2>&1; then
        __ltrim_colon_completions "$cur"
    fi
}
complete -o default -F _rhythm_complete rhythm
`

const zshCompletion = `#compdef rhythm
# zsh completion for rhythm. Save it as _rhythm in a folder on $fpath:
#   rhythm completion zsh > "${fpath[1]}/_rhythm"
# or load it after compinit with: source <(rhythm completion zsh)
_rhythm() {
    local -a candidates
    candidates=(${(f)"$(rhythm __complete "${(@)words[2,CURRENT]}" 2>/dev/null)"})
    if (( ${#candidates} )); then
        compadd -- "${candidates[@]}"
    else
        _files
    fi
}
if [ "$funcstack[1]" = "_rhythm" ]; then
    _rhythm "$@"
else
    compdef _rhythm rhythm
fi
`

const fishCompletion = `# fish completion for rhythm. Save it with:
#   rhythm completion fish > ~/.config/fish/completions/rhythm.fish
function __rhythm_complete
    set -l args (commandline -opc)
    set -e args[1]
    set -l cur (commandline -ct)
    set -l out (rhythm __complete $args "$cur" 2>/dev/null)
    if test (count $out) -gt 0
        printf '%s\n' $out
    else
        __fish_complete_path "$cur"
    end
end
complete -c rhythm -f -a '(__rhythm_complete)'
`

const powershellCompletion = `# PowerShell completion for rhythm. Load it from $PROFILE with:
#   rhythm completion powershell | Out-String | Invoke-Expression
Register-ArgumentCompleter -Native -CommandName 'rhythm', 'rhythm.exe' -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $words = @()
    foreach ($e in ($commandAst.CommandElements | Select-Object -Skip 1)) {
        if ($e.Extent.StartOffset -ge $cursorPosition) { break }
        if ($e -is [System.Management.Automation.Language.StringConstantExpressionAst]) {
            $words += $e.Value
        } else {
            $words += $e.Extent.Text
        }
    }
    if ($wordToComplete -eq '') {
        # Before 7.3 PowerShell drops an empty argument to a program;
        # a bare pair of quotes reaches it as one.
        $legacy = $PSVersionTable.PSVersion -lt [version]'7.3' -or $PSNativeCommandArgumentPassing -eq 'Legacy'
        if ($legacy) { $words += '""' } else { $words += '' }
    }
    $candidates = @(& rhythm __complete @words 2>$null)
    if ($candidates.Count -eq 0) { return }
    foreach ($c in $candidates) {
        if ($c -and $c.StartsWith($wordToComplete)) {
            [System.Management.Automation.CompletionResult]::new($c, $c, 'ParameterValue', $c)
        }
    }
}
`
