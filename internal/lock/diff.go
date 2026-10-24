// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package lock

import (
	"fmt"
	"maps"
	"slices"
)

// Kind says what changed about one program between two lock files.
type Kind string

// The kinds of Change, in the order Diff reports them for one program.
const (
	Added    Kind = "added"    // in the new lock only
	Removed  Kind = "removed"  // in the old lock only
	Bumped   Kind = "bumped"   // a different version
	Resigned Kind = "resigned" // the same version from a different signer
	// Rehashed is the same version with a different archive hash for a
	// platform both locks name: the release was rebuilt or replaced after
	// it was locked, which is worth a look before trusting it.
	Rehashed    Kind = "rehashed"
	DataChanged Kind = "data" // a data pack was added, removed or moved to another version
)

// Change is one difference between two lock files.
type Change struct {
	Kind Kind
	Name string // the program: "rhythm" or a tool name
	Pack string // the data pack, for DataChanged
	// From and To are the old and new versions: of the program, or of
	// the pack for DataChanged (empty for a pack that was added or
	// removed). For Resigned they are the signers.
	From, To string
}

// String is the change as `rhythm update --lock` prints it, for example
// "~ tuner 0.3.1 → 0.3.2".
func (c Change) String() string {
	switch c.Kind {
	case Added:
		return fmt.Sprintf("+ %s %s", c.Name, c.To)
	case Removed:
		return fmt.Sprintf("- %s %s", c.Name, c.From)
	case Bumped:
		return fmt.Sprintf("~ %s %s → %s", c.Name, c.From, c.To)
	case Resigned:
		return fmt.Sprintf("! %s: signer %s → %s for the same version", c.Name, orNone(c.From), orNone(c.To))
	case Rehashed:
		return fmt.Sprintf("! %s %s: archives changed for the same version", c.Name, c.To)
	case DataChanged:
		if c.From == c.To {
			return fmt.Sprintf("! %s data %s %s: content changed for the same version", c.Name, c.Pack, c.To)
		}
		return fmt.Sprintf("~ %s data %s %s → %s", c.Name, c.Pack, orNone(c.From), orNone(c.To))
	}
	return fmt.Sprintf("? %s", c.Name)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// Diff lists what changed from old to new: rhythm first, then tools by
// name; for each program its version, signer, archives and data packs by
// name. Either file may be nil, meaning empty.
func Diff(from, to *File) []Change {
	changes := diffEntry("rhythm", from.rhythm(), to.rhythm())
	before, after := from.tools(), to.tools()
	for _, name := range unionKeys(before, after) {
		changes = append(changes, diffEntry(name, before[name], after[name])...)
	}
	return changes
}

// unionKeys returns the keys of a and b, sorted and each once.
func unionKeys[V any](a, b map[string]V) []string {
	keys := slices.Collect(maps.Keys(a))
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func (f *File) rhythm() *Entry {
	if f == nil {
		return nil
	}
	return f.Rhythm
}

func (f *File) tools() map[string]*Entry {
	m := map[string]*Entry{}
	if f == nil {
		return m
	}
	for i := range f.Tools {
		m[f.Tools[i].Name] = &f.Tools[i]
	}
	return m
}

func diffEntry(name string, a, b *Entry) []Change {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return []Change{{Kind: Added, Name: name, To: b.Version}}
	case b == nil:
		return []Change{{Kind: Removed, Name: name, From: a.Version}}
	}
	var changes []Change
	if a.Version != b.Version {
		changes = append(changes, Change{Kind: Bumped, Name: name, From: a.Version, To: b.Version})
	} else {
		// A new version always brings new signatures and archives, so
		// these only mean something for the same version.
		if a.Signer != b.Signer {
			changes = append(changes, Change{Kind: Resigned, Name: name, From: a.Signer, To: b.Signer})
		}
		if rehashed(a.Platforms, b.Platforms) {
			changes = append(changes, Change{Kind: Rehashed, Name: name, From: a.Version, To: b.Version})
		}
	}
	return append(changes, diffData(name, a.Data, b.Data)...)
}

// rehashed reports whether a platform locked in both a and b has a
// different hash. A platform locked in only one of them is not a
// change to the release: locking darwin_arm64 for a second machine
// must not look like a replaced archive, so it is not reported.
func rehashed(a, b map[string]string) bool {
	for platform, sum := range a {
		if other, ok := b[platform]; ok && other != sum {
			return true
		}
	}
	return false
}

func diffData(name string, a, b []DataPack) []Change {
	before, after := map[string]DataPack{}, map[string]DataPack{}
	for _, d := range a {
		before[d.Name] = d
	}
	for _, d := range b {
		after[d.Name] = d
	}
	packs := unionKeys(before, after)
	var changes []Change
	for _, p := range packs {
		x, inA := before[p]
		y, inB := after[p]
		if inA && inB && x.Version == y.Version && x.SHA256 == y.SHA256 {
			continue
		}
		c := Change{Kind: DataChanged, Name: name, Pack: p}
		if inA {
			c.From = x.Version
		}
		if inB {
			c.To = y.Version
		}
		changes = append(changes, c)
	}
	return changes
}
