// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestLookupTool(t *testing.T) {
	for in, want := range map[string]string{"tuner": "tuner", "t": "tuner", "sc": "soundcheck", "e": "echo", "echo": "echo", "ro": "roadie", "sr": "sightread", "sightread": "sightread"} {
		if got, ok := lookupTool(in); !ok || got.Name != want {
			t.Errorf("lookupTool(%q) = %q, %v; want %q", in, got.Name, ok, want)
		}
	}
	if _, ok := lookupTool("nope"); ok {
		t.Error("lookupTool found an unknown tool")
	}
}

func TestToolNames(t *testing.T) {
	want := []string{"echo", "roadie", "sightread", "soundcheck", "tuner"}
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

func TestEmbeddedRegistry(t *testing.T) {
	tools, err := loadRegistry(toolsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(tools, registry, func(a, b toolInfo) bool { return a.Name == b.Name }) {
		t.Errorf("registry isn't tools.json: %v", registry)
	}
	for _, tool := range tools {
		if tool.Repo != "RhythmShahriar/"+tool.Name {
			t.Errorf("%s: repo %q", tool.Name, tool.Repo)
		}
	}
}

// TestRegistryDoesNotShadowCommands keeps tool names and aliases apart
// from rhythm's own commands, which run first and would hide the tool.
func TestRegistryDoesNotShadowCommands(t *testing.T) {
	for _, tool := range registry {
		for _, name := range append([]string{tool.Name}, tool.Aliases...) {
			if _, ok := commands[name]; ok {
				t.Errorf("tool %s: %q is also a command", tool.Name, name)
			}
		}
	}
}

func TestLoadRegistryRejects(t *testing.T) {
	const ok = `{"name": "tuner", "repo": "RhythmShahriar/tuner", "aliases": ["t"], "description": "d", "default_install": true, "min_rhythm": ""}`
	cases := []struct{ name, doc, err string }{
		{"duplicate alias", `[` + ok + `, {"name": "tool2", "repo": "o/tool2", "aliases": ["t"], "description": "d"}]`, `tool "tool2": "t" is already used by "tuner"`},
		{"alias is another tool's name", `[` + ok + `, {"name": "tool2", "repo": "o/tool2", "aliases": ["tuner"], "description": "d"}]`, `tool "tool2": "tuner" is already used by "tuner"`},
		{"duplicate name", `[` + ok + `, ` + ok + `]`, `tool "tuner": "tuner" is already used by "tuner"`},
		{"alias repeated in one tool", `[{"name": "tool2", "repo": "o/tool2", "aliases": ["x2", "x2"], "description": "d"}]`, `tool "tool2": "x2" is already used by "tool2"`},
		{"bad name", `[{"name": "../evil", "repo": "o/evil", "description": "d"}]`, `tool "../evil": invalid name or alias "../evil"`},
		{"upper-case name", `[{"name": "Tuner", "repo": "o/tuner", "description": "d"}]`, `tool "Tuner": invalid name or alias "Tuner"`},
		{"bad alias", `[{"name": "tool2", "repo": "o/tool2", "aliases": ["a b"], "description": "d"}]`, `tool "tool2": invalid name or alias "a b"`},
		{"missing name", `[{"repo": "o/x", "description": "d"}]`, `entry 1: missing name`},
		{"missing repo", `[{"name": "tool2", "description": "d"}]`, `tool "tool2": missing repo`},
		{"repo without owner", `[{"name": "tool2", "repo": "tool2", "description": "d"}]`, `tool "tool2": repo "tool2" isn't owner/name`},
		{"repo as a URL", `[{"name": "tool2", "repo": "https://github.com/o/tool2", "description": "d"}]`, `tool "tool2": repo "https://github.com/o/tool2" isn't owner/name`},
		{"repo with a path", `[{"name": "tool2", "repo": "o/tool2/x", "description": "d"}]`, `tool "tool2": repo "o/tool2/x" isn't owner/name`},
		{"missing description", `[{"name": "tool2", "repo": "o/tool2", "description": " "}]`, `tool "tool2": missing description`},
		{"bad min_rhythm", `[{"name": "tool2", "repo": "o/tool2", "description": "d", "min_rhythm": "soon"}]`, `tool "tool2": min_rhythm "soon" isn't a version`},
		{"misspelled key", `[{"name": "tool2", "repo": "o/tool2", "description": "d", "default_instll": true}]`, `json: unknown field "default_instll"`},
		{"empty list", `[]`, `no tools`},
		{"not a list", `{}`, `json: cannot unmarshal object into Go value of type []main.toolInfo`},
		{"trailing data", `[` + ok + `] []`, `data after the tool list`},
	}
	for _, c := range cases {
		_, err := loadRegistry([]byte(c.doc))
		if err == nil || err.Error() != c.err {
			t.Errorf("%s: got %v, want %s", c.name, err, c.err)
		}
	}
	if _, err := loadRegistry([]byte(`[` + ok + `, {"name": "tool2", "repo": "Some-Org/tool_2.go", "description": "d", "min_rhythm": "v0.7.0"}]`)); err != nil {
		t.Errorf("a valid registry was rejected: %v", err)
	}
}

func TestMustLoadRegistryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.HasPrefix(fmt.Sprint(r), "rhythm: tools.json: ") {
			t.Errorf("recovered %v", r)
		}
	}()
	mustLoadRegistry([]byte(`[]`))
}

// TestHelpListsRegistry checks that help's Tools block is exactly the
// registry, in order and marks a tool that needs a newer rhythm.
func TestHelpListsRegistry(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	savedReg, savedVer := registry, version
	t.Cleanup(func() { registry, version = savedReg, savedVer })
	registry = mustLoadRegistry([]byte(`[
		{"name": "aa", "repo": "o/aa", "aliases": ["a"], "description": "first"},
		{"name": "bb", "repo": "o/bb", "description": "second", "min_rhythm": "0.9.0"},
		{"name": "cc", "repo": "o/cc", "description": "third", "min_rhythm": "0.5.0"}]`))

	tools := func() string {
		var out bytes.Buffer
		usage(&out)
		_, s, _ := strings.Cut(out.String(), "\nTools:\n")
		s, _, _ = strings.Cut(s, "\n\n")
		return s
	}
	version = "0.6.0"
	if got, want := tools(), "  aa (a)  first\n  bb      second (needs rhythm >= 0.9.0)\n  cc      third"; got != want {
		t.Errorf("tools:\n%s\nwant:\n%s", got, want)
	}
	version = "dev" // a development build is assumed new enough
	if got, want := tools(), "  aa (a)  first\n  bb      second\n  cc      third"; got != want {
		t.Errorf("dev tools:\n%s\nwant:\n%s", got, want)
	}

	var errb bytes.Buffer
	if code := run([]string{"bbb"}, &errb, &errb); code != 2 || !strings.Contains(errb.String(), "did you mean: rhythm bb\n") {
		t.Errorf("did-you-mean doesn't come from the registry: %s", errb.String())
	}

	registry = savedReg
	got := tools()
	for _, tool := range registry {
		if !strings.Contains(got, "  "+tool.Name+" (") {
			t.Errorf("help lacks %s", tool.Name)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != len(registry) {
		t.Errorf("help lists %d tools, the registry has %d:\n%s", n, len(registry), got)
	}
}
