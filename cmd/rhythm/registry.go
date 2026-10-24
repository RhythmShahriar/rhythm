// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// toolInfo describes one tool the router knows by name. The list lives
// in tools.json, so adding a tool means adding an entry there.
type toolInfo struct {
	Name        string   `json:"name"`        // short name: `rhythm <Name>` runs rhythm-<Name>
	Repo        string   `json:"repo"`        // GitHub owner/name its releases come from
	Description string   `json:"description"` // one line for help
	Aliases     []string `json:"aliases"`     // shorter names that also work
	// DefaultInstall marks the tools the installers set up when the user
	// names none.
	DefaultInstall bool `json:"default_install"`
	// MinRhythm is the oldest router version the tool works with; empty
	// means any.
	MinRhythm string `json:"min_rhythm"`
}

// toolNameRE is the shape of every tool name and alias. The router joins
// the name onto "rhythm-" and looks it up on PATH, and exec.LookPath
// treats anything with a slash as a path, so `rhythm /../evil` would run
// a file relative to the current folder. Allowing only lowercase letters,
// digits and dashes keeps the lookup on PATH.
var toolNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// validToolName reports whether s may name a tool or alias.
func validToolName(s string) bool { return toolNameRE.MatchString(s) }

// toolsJSON is the tool registry, in the order help shows the tools.
//
//go:embed tools.json
var toolsJSON []byte

// registry lists the known tools in the order help shows them. A broken
// tools.json is a bug in the build, so loading it panics at start-up and
// the registry tests catch it before a release.
var registry = mustLoadRegistry(toolsJSON)

func mustLoadRegistry(data []byte) []toolInfo {
	tools, err := loadRegistry(data)
	if err != nil {
		panic("rhythm: tools.json: " + err.Error())
	}
	return tools
}

// repoRE is the shape of a GitHub owner/name.
var repoRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)

// loadRegistry parses and validates a tools.json document. Unknown
// fields are rejected so a misspelled key ("default_instll") fails loudly
// instead of silently meaning false.
func loadRegistry(data []byte) ([]toolInfo, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var tools []toolInfo
	if err := dec.Decode(&tools); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("data after the tool list")
	}
	if len(tools) == 0 {
		return nil, errors.New("no tools")
	}
	seen := map[string]string{} // name or alias -> the tool that has it
	claim := func(name, tool string) error {
		if !validToolName(name) {
			return fmt.Errorf("tool %q: invalid name or alias %q", tool, name)
		}
		if other, dup := seen[name]; dup {
			return fmt.Errorf("tool %q: %q is already used by %q", tool, name, other)
		}
		seen[name] = tool
		return nil
	}
	for i, t := range tools {
		if t.Name == "" {
			return nil, fmt.Errorf("entry %d: missing name", i+1)
		}
		if err := claim(t.Name, t.Name); err != nil {
			return nil, err
		}
		for _, a := range t.Aliases {
			if err := claim(a, t.Name); err != nil {
				return nil, err
			}
		}
		switch {
		case t.Repo == "":
			return nil, fmt.Errorf("tool %q: missing repo", t.Name)
		case !repoRE.MatchString(t.Repo):
			return nil, fmt.Errorf("tool %q: repo %q isn't owner/name", t.Name, t.Repo)
		case strings.TrimSpace(t.Description) == "":
			return nil, fmt.Errorf("tool %q: missing description", t.Name)
		case t.MinRhythm != "" && !update.IsRelease(t.MinRhythm):
			return nil, fmt.Errorf("tool %q: min_rhythm %q isn't a version", t.Name, t.MinRhythm)
		}
	}
	return tools, nil
}

// needsNewerRhythm reports whether t needs a newer router than this one.
// Development builds are assumed new enough.
func needsNewerRhythm(t toolInfo) bool {
	return t.MinRhythm != "" && update.IsRelease(version) && update.Newer(version, t.MinRhythm)
}

// lookupTool finds a tool by its name or one of its aliases.
func lookupTool(nameOrAlias string) (toolInfo, bool) {
	for _, t := range registry {
		if t.Name == nameOrAlias {
			return t, true
		}
		for _, a := range t.Aliases {
			if a == nameOrAlias {
				return t, true
			}
		}
	}
	return toolInfo{}, false
}

// toolNames returns every known tool name, sorted.
func toolNames() []string {
	names := make([]string, 0, len(registry))
	for _, t := range registry {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// defaultTools returns the tools installed when none are named, sorted.
func defaultTools() []string {
	var names []string
	for _, t := range registry {
		if t.DefaultInstall {
			names = append(names, t.Name)
		}
	}
	sort.Strings(names)
	return names
}
