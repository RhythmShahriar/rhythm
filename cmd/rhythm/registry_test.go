// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"slices"
	"strings"
	"testing"
)

func TestLookupTool(t *testing.T) {
	for in, want := range map[string]string{"tuner": "tuner", "t": "tuner", "sc": "soundcheck", "e": "echo", "echo": "echo"} {
		if got, ok := lookupTool(in); !ok || got.Name != want {
			t.Errorf("lookupTool(%q) = %q, %v; want %q", in, got.Name, ok, want)
		}
	}
	if _, ok := lookupTool("nope"); ok {
		t.Error("lookupTool found an unknown tool")
	}
}

func TestToolNames(t *testing.T) {
	want := []string{"echo", "soundcheck", "tuner"}
	if got := toolNames(); !slices.Equal(got, want) {
		t.Errorf("toolNames() = %v", got)
	}
	if got := defaultTools(); !slices.Equal(got, want) {
		t.Errorf("defaultTools() = %v", got)
	}
}

func TestValidToolName(t *testing.T) {
	for _, name := range []string{"tuner", "my-tool2", "t", "0", strings.Repeat("a", 64)} {
		if !validToolName(name) {
			t.Errorf("%q rejected", name)
		}
	}
	for _, name := range []string{"", "/x", "../x", "a/b", `a\b`, "-x", "UPPER", "日本", "a b", "a.b", strings.Repeat("a", 65)} {
		if validToolName(name) {
			t.Errorf("%q accepted", name)
		}
	}
}
