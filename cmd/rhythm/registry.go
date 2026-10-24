// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"regexp"
	"sort"
)

// toolInfo describes one tool the router knows by name.
type toolInfo struct {
	Name        string   // short name: `rhythm <Name>` runs rhythm-<Name>
	Repo        string   // GitHub owner/name its releases come from
	Description string   // one line for help
	Aliases     []string // shorter names that also work
	// DefaultInstall marks the tools the installers set up when the user
	// names none.
	DefaultInstall bool
	// MinRhythm is the oldest router version the tool works with; empty
	// means any.
	MinRhythm string
}

// toolNameRE is the shape of every tool name and alias. The router joins
// the name onto "rhythm-" and looks it up on PATH, and exec.LookPath
// treats anything with a slash as a path, so `rhythm /../evil` would run
// a file relative to the current folder. Allowing only lowercase letters,
// digits and dashes keeps the lookup on PATH.
var toolNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// validToolName reports whether s may name a tool or alias.
func validToolName(s string) bool { return toolNameRE.MatchString(s) }

// registry lists the known tools in the order help shows them.
var registry = []toolInfo{
	{
		Name:           "tuner",
		Repo:           "RhythmShahriar/tuner",
		Description:    "WordPress & PHP security scanner",
		Aliases:        []string{"t"},
		DefaultInstall: true,
	},
	{
		Name:           "soundcheck",
		Repo:           "RhythmShahriar/soundcheck",
		Description:    "did my change break anything? whole-site before/after checks",
		Aliases:        []string{"sc"},
		DefaultInstall: true,
	},
	{
		Name:           "echo",
		Repo:           "RhythmShahriar/echo",
		Description:    "is my site still saying what it should? every email, SMS and webhook",
		Aliases:        []string{"e"},
		DefaultInstall: true,
	},
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
