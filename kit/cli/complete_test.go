// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"context"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
)

// completeTool is fakeTool with a scan command that takes a value flag
// and completes its own positional argument.
func completeTool() Tool {
	tool := fakeTool()
	tool.Commands = append(tool.Commands, Command{
		Name: "check", Summary: "check a site",
		Run: func(_ context.Context, args []string, _, stderr io.Writer) error {
			_, err := ParseArgs(checkFlags(stderr), args)
			return err
		},
		Flags:    func() *flag.FlagSet { return checkFlags(io.Discard) },
		Complete: func([]string) []string { return []string{"wordpress", "woocommerce", "laravel"} },
	})
	return tool
}

func checkFlags(stderr io.Writer) *flag.FlagSet {
	fs := NewFlagSet("check", "Usage: rhythm-fake check <site>", stderr)
	fs.String("preset", "", "use a `name`d preset")
	fs.Bool("json", false, "print JSON")
	return fs
}

func completeLines(t *testing.T, tool Tool, words ...string) []string {
	t.Helper()
	code, out, errOut := runTool(t, tool, append([]string{CompleteCommand}, words...)...)
	if code != ExitOK || errOut != "" {
		t.Fatalf("__complete %q: exit %d, stderr %q", words, code, errOut)
	}
	return strings.Fields(out)
}

func TestMainComplete(t *testing.T) {
	cases := []struct {
		words []string
		want  []string
	}{
		{nil, []string{"check", "help", "scan", "update", "version"}},
		{[]string{""}, []string{"check", "help", "scan", "update", "version"}},
		{[]string{"s"}, []string{"scan"}},
		{[]string{"hid"}, nil}, // hidden commands stay hidden
		{[]string{"__c"}, nil},
		{[]string{"help", "v"}, []string{"version"}},
		{[]string{"help", "scan", ""}, nil},
		{[]string{"version", "--"}, []string{"--help", "--json"}},
		{[]string{"update", "--j"}, []string{"--json"}},
		{[]string{"scan", "--"}, []string{"--help", "--offline"}},
		{[]string{"check", "--p"}, []string{"--preset"}},
		{[]string{"check", "w"}, []string{"woocommerce", "wordpress"}},
		{[]string{"check", "--preset", ""}, nil}, // the value is the shell's to complete
		{[]string{"check", "--json", "l"}, []string{"laravel"}},
		{[]string{"scan", ""}, nil}, // no Complete hook
		{[]string{"nope", ""}, nil},
	}
	for _, c := range cases {
		got := completeLines(t, completeTool(), c.words...)
		if !slices.Equal(got, c.want) {
			t.Errorf("__complete %q = %q, want %q", c.words, got, c.want)
		}
	}
}

func TestMainCompleteDefault(t *testing.T) {
	tool := completeTool()
	tool.Default = "check"
	if got := completeLines(t, tool, "--j"); !slices.Equal(got, []string{"--json"}) {
		t.Errorf("flags of the default command: %q", got)
	}
	if got := completeLines(t, tool, "example.com", "--p"); !slices.Equal(got, []string{"--preset"}) {
		t.Errorf("flags after a site: %q", got)
	}
}

// TestMainCompleteRunsNothing checks that completion never runs a
// command for real, prints the release notice or the first-run note.
func TestMainCompleteRunsNothing(t *testing.T) {
	tool := completeTool()
	tool.Default = "scan"
	ran := false
	tool.FirstRun = func(io.Writer) { ran = true }
	t.Setenv("RHYTHM_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := completeLines(t, tool, "ok"); len(got) != 0 {
		t.Errorf("__complete ok = %q, want nothing (scan ok would print scanned)", got)
	}
	if got := completeLines(t, tool, "ok", ""); len(got) != 0 {
		t.Errorf("__complete ok '' = %q, want nothing", got)
	}
	if ran {
		t.Error("__complete ran the first-run note")
	}
}

// TestCompleteNeverRunsACommand checks that a command whose Run ignores
// its arguments isn't run on Tab: only Flags is asked for flags.
func TestCompleteNeverRunsACommand(t *testing.T) {
	ran := 0
	tool := fakeTool()
	tool.Commands = append(tool.Commands, Command{Name: "wipe", Run: func(context.Context, []string, io.Writer, io.Writer) error {
		ran++
		return nil
	}})
	for _, words := range [][]string{{"wipe", "--"}, {"wipe", "x", ""}, {"wipe", "--force", "x"}} {
		if got := completeLines(t, tool, words...); !slices.Equal(got, nil) && !slices.Equal(got, []string{"--help"}) {
			t.Errorf("__complete %q = %q", words, got)
		}
	}
	if ran != 0 {
		t.Errorf("Run ran %d times on Tab", ran)
	}
	if got := completeLines(t, tool, "update", "--"); !slices.Equal(got, []string{"--help", "--json"}) {
		t.Errorf("update flags = %q", got)
	}
	tool.UpdatePinned = func(context.Context, io.Writer, map[string]string) ([]DataPack, error) { return nil, nil }
	if got := completeLines(t, tool, "update", "--"); !slices.Equal(got, []string{"--help", "--json", "--pin"}) {
		t.Errorf("update flags with UpdatePinned = %q", got)
	}
}

func TestSetFlags(t *testing.T) {
	fs := checkFlags(io.Discard)
	fs.Bool("help", false, "")
	want := []HelpFlag{{"json", false}, {"preset", true}}
	if got := SetFlags(fs); !slices.Equal(got, want) {
		t.Errorf("SetFlags = %+v, want %+v", got, want)
	}
	if SetFlags(nil) != nil {
		t.Error("nil flag set has flags")
	}
}

func TestToolCheckReservesComplete(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a command named __complete didn't panic")
		}
	}()
	tool := fakeTool()
	tool.Commands = append(tool.Commands, Command{Name: CompleteCommand, Run: tool.Commands[0].Run})
	tool.check()
}

func TestParseHelpFlags(t *testing.T) {
	// The first half is NewFlagSet's format, the second the flag package's.
	help := `Usage: rhythm install <tool> [--dir <dir>]

Flags:
  --check              only report
  --dir dir            install into dir
  --retries int        attempts per download (default 3)
  -h, --help
Usage of update:
  -check
    	only report available updates
  -to string
    	install this version
  -x	short one
  -dir string
    	repeated, listed once
`
	want := []HelpFlag{{"check", false}, {"dir", true}, {"retries", true}, {"to", true}, {"x", false}}
	if got := ParseHelpFlags(help); !slices.Equal(got, want) {
		t.Errorf("ParseHelpFlags = %+v, want %+v", got, want)
	}
	if got := FlagWords(want[:2]); !slices.Equal(got, []string{"--check", "--dir", "--help"}) {
		t.Errorf("FlagWords = %q", got)
	}
	for word, takes := range map[string]bool{"--dir": true, "-dir": true, "--dir=x": false, "--check": false, "dir": false, "--nope": false} {
		if got := TakesValue(want, word); got != takes {
			t.Errorf("TakesValue(%q) = %v, want %v", word, got, takes)
		}
	}
}

func TestFilterPrefix(t *testing.T) {
	got := FilterPrefix([]string{"tuner", "t", "soundcheck", "tuner", "echo"}, "t")
	if !slices.Equal(got, []string{"t", "tuner"}) {
		t.Errorf("FilterPrefix = %q", got)
	}
	if got := FilterPrefix([]string{"a"}, "b"); got != nil {
		t.Errorf("no match = %q, want nil", got)
	}
}
