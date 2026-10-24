// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package lock reads and writes rhythm.lock, the file that pins the
// exact rhythm and tool versions a project uses, so a scan gives the same
// result on every laptop and in every pipeline. It also keeps the list
// of installed extensions (see Extensions).
//
// A lock file is JSON with the schema rhythm-lock/v1:
//
//	{
//	  "schema": "rhythm-lock/v1",
//	  "rhythm": {"name": "rhythm", "repo": "RhythmShahriar/rhythm", "version": "0.7.0", ...},
//	  "tools": [
//	    {
//	      "name": "tuner",
//	      "repo": "RhythmShahriar/tuner",
//	      "version": "0.3.1",
//	      "signer": "...",
//	      "platforms": {"darwin_arm64": "<sha256>", "linux_amd64": "<sha256>"},
//	      "data": [{"name": "wpvulnerability", "version": "2026-10-24", "sha256": "<sha256>"}],
//	      "source": "registry"
//	    }
//	  ]
//	}
//
// Write always produces the same bytes for the same content (tools and
// data sorted by name, two-space indent, a trailing newline), so the file
// diffs cleanly in code review.
package lock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/cli"
)

// Schema is the version of the lock file format. A change that older
// readers can't follow needs a new one.
const Schema = "rhythm-lock/v1"

// FileName is the lock file's name in a project.
const FileName = "rhythm.lock"

// Sources of an Entry.
const (
	SourceRegistry  = "registry"  // a tool rhythm knows by name
	SourceExtension = "extension" // a tool added with rhythm ext
)

// File is the content of a rhythm.lock.
type File struct {
	Schema string  `json:"schema"`
	Rhythm *Entry  `json:"rhythm,omitempty"` // the router itself; nil leaves it unpinned
	Tools  []Entry `json:"tools"`
}

// Entry pins one program: rhythm or a tool.
type Entry struct {
	Name    string `json:"name"`    // "tuner"; "rhythm" for the router
	Repo    string `json:"repo"`    // GitHub owner/name its releases come from
	Version string `json:"version"` // exact SemVer, "0.3.1"
	// Signer identifies who signed the release, once signatures are
	// checked; empty when the release wasn't verified.
	Signer string `json:"signer,omitempty"`
	// Platforms maps os_arch ("linux_amd64") to the sha256 of that
	// platform's release archive.
	Platforms map[string]string `json:"platforms"`
	Data      []DataPack        `json:"data,omitempty"` // the tool's data, as `update --json` reported it
	// Source is SourceRegistry or SourceExtension; empty means
	// SourceRegistry.
	Source string `json:"source,omitempty"`
}

// DataPack is one piece of a tool's data, as `<tool> update --json`
// reports it.
type DataPack = cli.DataPack

// Read parses and validates the lock file at path.
func Read(path string) (*File, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the user's own lock file
	if err != nil {
		return nil, err
	}
	f, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Parse decodes and validates a lock file's content. Unknown fields are
// errors: a lock file is a promise about exact versions, so a key this
// rhythm doesn't understand must not be dropped on the next write.
func Parse(data []byte) (*File, error) {
	var head struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	if head.Schema != Schema {
		return nil, schemaError(head.Schema)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	// The schema check above has already refused anything after the
	// first JSON value.
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

func schemaError(s string) error {
	if s == "" {
		return fmt.Errorf("no schema; want %q", Schema)
	}
	return fmt.Errorf("schema %q isn't %q; a newer rhythm may read it", s, Schema)
}

// Write validates f and writes it to path atomically: readers see the
// old file or the new one, never half of it. An empty Schema is set to
// Schema. Tools and data packs are sorted by name in place.
func (f *File) Write(path string) error {
	data, err := f.Marshal()
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o644)
}

// Marshal validates f and returns the bytes Write would write.
func (f *File) Marshal() ([]byte, error) {
	if f.Schema == "" {
		f.Schema = Schema
	}
	f.sort()
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if f.Tools == nil {
		f.Tools = []Entry{} // "tools": [] rather than null
	}
	return marshal(f)
}

// marshal encodes v with a two-space indent and a trailing newline.
// encoding/json writes map keys sorted, so platforms are stable too, and
// HTML escaping is off so a signer URL reads as written.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (f *File) sort() {
	slices.SortFunc(f.Tools, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	for i := range f.Tools {
		f.Tools[i].sortData()
	}
	if f.Rhythm != nil {
		f.Rhythm.sortData()
	}
}

func (e *Entry) sortData() {
	slices.SortFunc(e.Data, func(a, b DataPack) int { return strings.Compare(a.Name, b.Name) })
}

// Tool returns the entry for the named tool.
func (f *File) Tool(name string) (Entry, bool) {
	for _, e := range f.Tools {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// ErrNotFound is what Find returns when no lock file is in reach.
var ErrNotFound = fmt.Errorf("no %s found: %w", FileName, fs.ErrNotExist)

// Find returns the path of the rhythm.lock that applies in dir: the
// nearest one in dir or a parent, up to and including the root of the
// git repository dir is in. Outside a repository only dir itself is
// looked in, so a stray lock in a home or temp folder never applies to
// unrelated projects.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root := repoRoot(dir)
	if root == "" {
		root = dir
	}
	for d := dir; ; d = filepath.Dir(d) {
		p := filepath.Join(d, FileName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
		if d == root || filepath.Dir(d) == d {
			return "", ErrNotFound
		}
	}
}

// repoRoot returns the nearest folder at or above dir holding .git (a
// folder, or a file in a worktree or submodule), or "" outside a
// repository.
func repoRoot(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// writeAtomic writes data to a temporary file next to path, flushes it
// to disk and renames it over path.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // gone after the rename; cleans up after a failure
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

var (
	// nameRE matches the router's tool names (cmd/rhythm validToolName).
	nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	// repoRE matches a GitHub owner/name.
	repoRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)
	// semverRE is SemVer 2.0.0's own pattern: exact versions only, no
	// "v" prefix and no partial "0.3".
	semverRE   = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
	sha256RE   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	platformRE = regexp.MustCompile(`^[a-z0-9]+_[a-z0-9]+$`)
)

// Validate reports every problem with f: an unknown schema, duplicate
// tools, names, repos, versions or sources that aren't valid, and
// hashes that aren't 64 lowercase hex digits.
func (f *File) Validate() error {
	var errs []error
	if f.Schema != Schema {
		errs = append(errs, schemaError(f.Schema))
	}
	if f.Rhythm != nil {
		if f.Rhythm.Name != "rhythm" {
			errs = append(errs, fmt.Errorf("rhythm: name %q isn't \"rhythm\"", f.Rhythm.Name))
		}
		errs = append(errs, f.Rhythm.validate("rhythm")...)
	}
	seen := map[string]bool{}
	for i, e := range f.Tools {
		where := fmt.Sprintf("tools[%d]", i)
		if e.Name != "" {
			where += " (" + e.Name + ")"
		}
		if seen[e.Name] {
			errs = append(errs, fmt.Errorf("%s: listed twice", where))
		}
		seen[e.Name] = true
		errs = append(errs, e.validate(where)...)
	}
	return errors.Join(errs...)
}

func (e *Entry) validate(where string) []error {
	var errs []error
	bad := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf(where+": "+format, a...))
	}
	if !nameRE.MatchString(e.Name) {
		bad("name %q isn't a tool name", e.Name)
	}
	if !repoRE.MatchString(e.Repo) {
		bad("repo %q isn't owner/name", e.Repo)
	}
	if !semverRE.MatchString(e.Version) {
		bad("version %q isn't an exact version such as 1.2.3", e.Version)
	}
	if len(e.Platforms) == 0 {
		bad("no platforms")
	}
	for _, p := range sortedKeys(e.Platforms) {
		if !platformRE.MatchString(p) {
			bad("platform %q isn't os_arch", p)
		}
		if h := e.Platforms[p]; !sha256RE.MatchString(h) {
			bad("platforms[%q]: sha256 %q isn't 64 lowercase hex digits", p, h)
		}
	}
	packs := map[string]bool{}
	for _, d := range e.Data {
		switch {
		case d.Name == "":
			bad("a data pack has no name")
		case packs[d.Name]:
			bad("data %q listed twice", d.Name)
		}
		packs[d.Name] = true
		if d.Version == "" {
			bad("data %q has no version", d.Name)
		}
		if !sha256RE.MatchString(d.SHA256) {
			bad("data %q: sha256 %q isn't 64 lowercase hex digits", d.Name, d.SHA256)
		}
	}
	switch e.Source {
	case "", SourceRegistry, SourceExtension:
	default:
		bad("source %q isn't %q or %q", e.Source, SourceRegistry, SourceExtension)
	}
	return errs
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
