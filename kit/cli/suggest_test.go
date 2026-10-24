// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

import (
	"bytes"
	"testing"
)

func TestDidYouMean(t *testing.T) {
	var b bytes.Buffer
	if !DidYouMean(&b, "rhythm install", "tunr", []string{"echo", "tuner"}) || b.String() != "  did you mean: rhythm install tuner\n" {
		t.Errorf("close: %q", b.String())
	}
	b.Reset()
	if DidYouMean(&b, "rhythm install", "zzzzz", []string{"echo", "tuner"}) || b.Len() != 0 {
		t.Errorf("nothing close: %q", b.String())
	}
}

func TestSuggest(t *testing.T) {
	opts := []string{"tuner", "t", "soundcheck", "sc", "echo", "e", "install", "update", "version"}
	cases := map[string]string{
		"tunr":       "tuner", // one deletion
		"tunner":     "tuner", // one insertion
		"tuenr":      "tuner", // two substitutions
		"soundchek":  "soundcheck",
		"isntall":    "install", // a swap is two edits
		"updte":      "update",
		"tuner":      "tuner", // exact
		"x":          "",      // one letter is too short to guess from
		"zz":         "",      // two edits away from "sc" but no closer than its length
		"tnrx":       "",      // three edits
		"completely": "",
		"":           "",
		"ecoh":       "echo",
		"übersicht":  "",
	}
	for in, want := range cases {
		if got := Suggest(in, opts); got != want {
			t.Errorf("Suggest(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Suggest("tunr", nil); got != "" {
		t.Errorf("no options: %q", got)
	}
}

func TestLevenshtein(t *testing.T) {
	for _, c := range []struct {
		a, b string
		d    int
	}{{"", "", 0}, {"a", "", 1}, {"", "abc", 3}, {"kitten", "sitting", 3}, {"日本", "日本語", 1}} {
		if got := levenshtein([]rune(c.a), []rune(c.b)); got != c.d {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.d)
		}
	}
}
