// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// helpWidth is the width rhythm's help text is filled to.
const helpWidth = 72

// columnRE splits an indented line of a two-column block, such as a
// Commands: row, into its left part and the column the right part
// starts at.
var columnRE = regexp.MustCompile(`^(  \S.*?\S)( {2,})\S`)

// layoutProblems reports the lines of a help text that break early (a
// short line in the middle of a paragraph) and the two-column blocks
// whose right column doesn't start in one place. The Flags: block is
// printed by the flag set and left alone.
func layoutProblems(help string) []string {
	text, _, _ := strings.Cut(help, "\nFlags:\n")
	lines := strings.Split(text, "\n")
	var problems []string
	for i := 0; i+1 < len(lines); i++ {
		cur, next := lines[i], lines[i+1]
		if cur == "" || next == "" || strings.HasPrefix(cur, " ") || strings.HasPrefix(next, " ") ||
			strings.HasSuffix(cur, ":") {
			continue
		}
		word, _, _ := strings.Cut(next, " ")
		if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(word) <= helpWidth {
			problems = append(problems, "breaks early: "+cur)
		}
	}
	// A block is a run of indented lines; every two-column row in it has
	// its right column at the same place.
	col := -1
	for _, line := range lines {
		if !strings.HasPrefix(line, "  ") {
			col = -1
			continue
		}
		m := columnRE.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		at := utf8.RuneCountInString(line[:m[5]])
		switch {
		case col < 0:
			col = at
		case at != col:
			problems = append(problems, "misaligned: "+line)
		}
	}
	return problems
}

func TestLayoutProblems(t *testing.T) {
	good := "Usage: rhythm x\n\nA paragraph filled to the width, so no line in the middle of it ends\nearly.\n\nCommands:\n  rhythm x a      one\n  rhythm x long   two\n"
	if got := layoutProblems(good); len(got) != 0 {
		t.Errorf("good help: %q", got)
	}
	bad := "Usage: rhythm x\n\nA paragraph that breaks\nearly.\n\nCommands:\n  rhythm x a      one\n  rhythm x longer-one  two\n"
	want := []string{"breaks early: A paragraph that breaks", "misaligned:   rhythm x longer-one  two"}
	if got := layoutProblems(bad); !slices.Equal(got, want) {
		t.Errorf("bad help: %q, want %q", got, want)
	}
}

// Every command's help is filled to the width and its blocks are
// aligned, so the text reads the same as the wiki quotes it.
func TestHelpLayout(t *testing.T) {
	var cmds [][]string
	for name, c := range commands {
		if !c.hidden {
			cmds = append(cmds, []string{name, "--help"})
		}
	}
	cmds = append(cmds, []string{"ext", "search", "--help"}, []string{"ext", "install", "--help"}, []string{"ext", "list", "--help"}, []string{"ext", "remove", "--help"})
	for _, args := range cmds {
		_, out, _ := runCmd(args...)
		for _, p := range layoutProblems(out) {
			t.Errorf("rhythm %s: %s", strings.Join(args, " "), p)
		}
	}
}
