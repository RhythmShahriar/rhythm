// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// CompleteCommand is the hidden command that answers shell completion.
// `<tool> __complete <words...>` prints, one per line, what may come
// next: words are the arguments after the tool's name, the last one
// being the word under the cursor (empty when the cursor follows a
// space). `rhythm completion` scripts call the router, which forwards
// `rhythm <tool> ...` to the tool this way.
//
// The router forwards only to a tool in its registry or one the user
// added as an extension. It first runs `<tool> version --json` to see that
// the tool prints a rhythm-version/v1 document with a kit_version, as
// Main does. A tool not built on Main that wants completion must answer
// both the same way; one that doesn't is never sent __complete.
const CompleteCommand = "__complete"

// complete answers CompleteCommand from the tool's commands. It never
// runs a command: flags come from Command.Flags. Nothing it does touches
// the network or the disk, so it is safe on every key press.
func (t Tool) complete(words []string, stdout io.Writer) int {
	if len(words) == 0 {
		words = []string{""}
	}
	for _, w := range t.completions(words) {
		fmt.Fprintln(stdout, w)
	}
	return ExitOK
}

func (t Tool) completions(words []string) []string {
	cur := words[len(words)-1]
	if len(words) == 1 {
		if t.Default != "" && strings.HasPrefix(cur, "-") {
			// `rhythm-soundcheck --j` is `rhythm-soundcheck check --j`.
			return t.commandCompletions(t.Default, words)
		}
		return FilterPrefix(t.names(), cur)
	}
	switch name := words[0]; {
	case name == "help":
		if len(words) == 2 {
			return FilterPrefix(t.names(), cur)
		}
		return nil
	case name == "version":
		return FilterPrefix([]string{"--json", "--help"}, cur)
	case name == "update" && t.Update != nil:
		words := []string{"--json", "--help"}
		if t.UpdatePinned != nil {
			words = append(words, "--pin")
		}
		return FilterPrefix(words, cur)
	}
	if _, ok := t.command(words[0]); ok {
		return t.commandCompletions(words[0], words[1:])
	}
	if t.Default != "" {
		return t.commandCompletions(t.Default, words)
	}
	return nil
}

// commandCompletions completes the words after one command's name:
// flags from its Flags and anything else from its Complete hook. A word
// after a flag that takes a value gets nothing, so the shell falls back
// to file names.
func (t Tool) commandCompletions(name string, args []string) []string {
	c, _ := t.command(name)
	cur := args[len(args)-1]
	var flags []HelpFlag
	if c.Flags != nil && (strings.HasPrefix(cur, "-") || len(args) > 1) {
		flags = SetFlags(c.Flags())
	}
	if strings.HasPrefix(cur, "-") {
		return FilterPrefix(FlagWords(flags), cur)
	}
	if len(args) > 1 && TakesValue(flags, args[len(args)-2]) {
		return nil
	}
	if c.Complete == nil {
		return nil
	}
	return FilterPrefix(c.Complete(args), cur)
}

// HelpFlag is a flag found in a command's help text or flag set.
type HelpFlag struct {
	Name       string // without dashes: "dir"
	TakesValue bool   // written with an argument, "--dir dir" or "-dir string"
}

// helpFlagRE matches a flag at the start of a help line, as NewFlagSet
// prints it ("  --dir dir   usage", "  --json   usage") or as the flag
// package does ("  -dir string", "  -json"). A value is a word one space
// after the name; NewFlagSet pads usage by three spaces or more and the
// flag package puts it on the next line or after a tab, so a bare flag's
// usage is never taken for an argument.
var helpFlagRE = regexp.MustCompile(`(?m)^[ \t]+--?([A-Za-z0-9][A-Za-z0-9_.-]*)( [^\s-]\S*)?`)

// ParseHelpFlags lists the flags a command's help text documents, in
// order and each once, leaving out -h and --help. It serves a program
// that completes from its commands' printed help, as the rhythm router
// does for its own commands; a Tool's completion uses Command.Flags
// (see SetFlags) so it never runs a command.
func ParseHelpFlags(help string) []HelpFlag {
	var flags []HelpFlag
	seen := map[string]bool{"h": true, "help": true}
	for _, m := range helpFlagRE.FindAllStringSubmatch(help, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		flags = append(flags, HelpFlag{Name: m[1], TakesValue: m[2] != ""})
	}
	return flags
}

// SetFlags lists the flags of fs in the form ParseHelpFlags returns,
// leaving out -h and --help. A nil fs has none.
func SetFlags(fs *flag.FlagSet) []HelpFlag {
	var flags []HelpFlag
	if fs == nil {
		return nil
	}
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == "h" || f.Name == "help" {
			return
		}
		b, isBool := f.Value.(interface{ IsBoolFlag() bool })
		flags = append(flags, HelpFlag{Name: f.Name, TakesValue: !isBool || !b.IsBoolFlag()})
	})
	return flags
}

// FlagWords returns flags as the user types them, "--dir", plus --help.
func FlagWords(flags []HelpFlag) []string {
	words := make([]string, 0, len(flags)+1)
	for _, f := range flags {
		words = append(words, "--"+f.Name)
	}
	return append(words, "--help")
}

// TakesValue reports whether word is a flag in flags that needs a value
// in the next word: "--dir" or "-dir" but not "--dir=x".
func TakesValue(flags []HelpFlag, word string) bool {
	name, ok := strings.CutPrefix(word, "-")
	if !ok || strings.Contains(name, "=") {
		return false
	}
	name = strings.TrimPrefix(name, "-")
	return slices.ContainsFunc(flags, func(f HelpFlag) bool { return f.Name == name && f.TakesValue })
}

// FilterPrefix returns the words that start with prefix, sorted and each
// once.
func FilterPrefix(words []string, prefix string) []string {
	var out []string
	for _, w := range words {
		if strings.HasPrefix(w, prefix) {
			out = append(out, w)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
