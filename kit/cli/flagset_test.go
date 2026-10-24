// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"strings"
	"testing"
	"time"
)

const installSynopsis = "Usage: rhythm install <tool> [--dir <dir>]\n\nDownloads a tool.\n"

// newInstallFlags returns a flag set like rhythm install's.
func newInstallFlags(w *bytes.Buffer) (fs *flag.FlagSet, force *bool, dir *string) {
	fs = NewFlagSet("install", installSynopsis, w)
	return fs, fs.Bool("force", false, "replace an existing install"), fs.String("dir", "", "install into `dir`")
}

func TestNewFlagSetUsage(t *testing.T) {
	var out bytes.Buffer
	fs := NewFlagSet("install", installSynopsis, &out)
	fs.String("dir", "", "install into `dir`")
	fs.Bool("force", false, "replace an existing install")
	fs.Int("retries", 3, "attempts per download")
	fs.Duration("timeout", time.Minute, "give up after this long")
	var tags ListFlag
	fs.Var(&tags, "tag", "only these `tags`")
	fs.String("o", "", "short for --output `file`")

	_, err := ParseArgs(fs, []string{"--help"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("--help: %v", err)
	}
	want := installSynopsis + `
Flags:
  --dir dir            install into dir
  --force              replace an existing install
  -o file              short for --output file
  --retries int        attempts per download (default 3)
  --tag tags           only these tags
  --timeout duration   give up after this long (default 1m0s)
`
	if out.String() != want {
		t.Errorf("usage:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestParseArgsHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "-help"} {
		var out bytes.Buffer
		fs, _, _ := newInstallFlags(&out)
		_, err := ParseArgs(fs, []string{"tuner", arg})
		if !errors.Is(err, ErrHelp) {
			t.Errorf("%s: %v", arg, err)
		}
		var ue UsageError
		if errors.As(err, &ue) {
			t.Errorf("%s is a usage error", arg)
		}
		if n := strings.Count(out.String(), "Usage:"); n != 1 {
			t.Errorf("%s printed the usage %d times:\n%s", arg, n, out.String())
		}
	}
}

func TestParseArgsBadFlag(t *testing.T) {
	var out bytes.Buffer
	fs := NewFlagSet("install", installSynopsis, &out)
	fs.String("dir", "", "install into `dir`")
	_, err := ParseArgs(fs, []string{"--nope"})
	var ue UsageError
	if !errors.As(err, &ue) || ue.Msg != "flag provided but not defined: -nope" || !ue.Reported {
		t.Fatalf("got %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "flag provided but not defined: -nope\n"+installSynopsis) || strings.Count(got, "Usage:") != 1 {
		t.Errorf("output:\n%s", got)
	}
	if strings.Contains(got, "\n  -dir") || !strings.Contains(got, "\n  --dir dir") {
		t.Errorf("flags aren't double-dash:\n%s", got)
	}
}

func TestParseArgs(t *testing.T) {
	var out bytes.Buffer
	fs, force, dir := newInstallFlags(&out)
	pos, err := ParseArgs(fs, []string{"tuner", "--dir", "/opt", "echo", "--force"})
	if err != nil || !slices.Equal(pos, []string{"tuner", "echo"}) || *dir != "/opt" || !*force {
		t.Errorf("pos=%v dir=%q force=%v err=%v", pos, *dir, *force, err)
	}
	if out.Len() != 0 {
		t.Errorf("printed %q", out.String())
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	for _, c := range []struct {
		args, pos []string
		force     bool
	}{
		{[]string{"--", "--force"}, []string{"--force"}, false},
		{[]string{"a", "--", "b", "--force"}, []string{"a", "b", "--force"}, false},
		{[]string{"--force", "--", "-x"}, []string{"-x"}, true},
		{[]string{"a", "--force", "b"}, []string{"a", "b"}, true},
	} {
		var out bytes.Buffer
		fs := NewFlagSet("x", "Usage: x", &out)
		force := fs.Bool("force", false, "")
		pos, err := ParseArgs(fs, c.args)
		if err != nil || !slices.Equal(pos, c.pos) || *force != c.force {
			t.Errorf("%q: pos=%q force=%v err=%v", c.args, pos, *force, err)
		}
	}
}

func TestPrintUsageNoFlags(t *testing.T) {
	var out bytes.Buffer
	fs := NewFlagSet("version", "Usage: rhythm version", &out)
	fs.Usage()
	if out.String() != "Usage: rhythm version\n" {
		t.Errorf("got %q", out.String())
	}
}
