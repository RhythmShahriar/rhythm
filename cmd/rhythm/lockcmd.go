// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

func init() {
	addRouterCommand(routerCommand{name: "lock", args: "[--no-data]", summary: "pin the installed versions in rhythm.lock", run: cmdLock})
}

// The rhythm.lock file. router-core's internal/lock owns this format;
// until it is merged, the stand-in below reads and writes the same
// rhythm-lock/v1 JSON, so a lock written now reads the same afterwards.
const (
	lockSchema   = "rhythm-lock/v1"
	lockFileName = "rhythm.lock"
)

// lockFile is a rhythm.lock: the exact versions a project runs with.
type lockFile struct {
	Schema string      `json:"schema"`
	Rhythm *lockEntry  `json:"rhythm,omitempty"` // the router itself; nil leaves it unpinned
	Tools  []lockEntry `json:"tools"`
}

// lockEntry pins one program: its version and every platform's archive.
type lockEntry struct {
	Name      string            `json:"name"` // "tuner"; "rhythm" for the router
	Repo      string            `json:"repo"`
	Version   string            `json:"version"`
	Signer    string            `json:"signer,omitempty"`
	Platforms map[string]string `json:"platforms"` // os_arch -> archive sha256
	Data      []dataPack        `json:"data,omitempty"`
	Source    string            `json:"source,omitempty"`
}

// dataPack is one data set a tool reports from `update --json`, such as
// Tuner's checks or its vulnerability database.
type dataPack struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// packValue is what a data pack's name and version may be. They are
// joined into update --pin name=version,..., so a "," or "=" would add
// pins the lock doesn't hold.
var packValue = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// errNoLock is findLock's answer when no rhythm.lock applies.
var errNoLock = fmt.Errorf("no %s found: %w", lockFileName, fs.ErrNotExist)

// findLock returns the nearest rhythm.lock in dir or a parent, stopping at
// the git repository root. Outside a repository only dir itself counts,
// so a stray lock in a home or temp folder never applies.
func findLock(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root := dir
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			root = d
			break
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	for d := dir; ; d = filepath.Dir(d) {
		p := filepath.Join(d, lockFileName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
		if d == root || filepath.Dir(d) == d {
			return "", errNoLock
		}
	}
}

// readLock reads and checks the lock at path. Unknown fields are refused,
// so a newer rhythm's key is never dropped when this one rewrites it.
func readLock(path string) (*lockFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the user's own lock file
	if err != nil {
		return nil, err
	}
	var head struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if head.Schema != lockSchema {
		if head.Schema == "" {
			return nil, fmt.Errorf("%s: no schema; want %q", path, lockSchema)
		}
		return nil, fmt.Errorf("%s: schema %q isn't %q; a newer rhythm may read it", path, head.Schema, lockSchema)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f lockFile
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &f, nil
}

// validate reports every entry that couldn't be installed from, one per
// line.
func (f *lockFile) validate() error {
	var errs []error
	check := func(where string, e *lockEntry) {
		if !exactSpec.MatchString(e.Version) || strings.HasPrefix(e.Version, "v") {
			errs = append(errs, fmt.Errorf("%s: version %q isn't an exact version such as 1.2.3", where, e.Version))
		}
		if !strings.Contains(e.Repo, "/") {
			errs = append(errs, fmt.Errorf("%s: repo %q isn't owner/name", where, e.Repo))
		}
		if len(e.Platforms) == 0 {
			errs = append(errs, fmt.Errorf("%s: no platforms", where))
		}
		for _, p := range slices.Sorted(maps.Keys(e.Platforms)) {
			if !sha256Hex.MatchString(e.Platforms[p]) {
				errs = append(errs, fmt.Errorf("%s: platforms[%q]: sha256 %q isn't 64 lowercase hex digits", where, p, e.Platforms[p]))
			}
		}
		packs := map[string]bool{}
		for _, d := range e.Data {
			switch {
			case d.Name == "":
				errs = append(errs, fmt.Errorf("%s: a data pack has no name", where))
			case packs[d.Name]:
				errs = append(errs, fmt.Errorf("%s: data %q listed twice", where, d.Name))
			case d.Version == "":
				errs = append(errs, fmt.Errorf("%s: data %q has no version", where, d.Name))
			case !packValue.MatchString(d.Name) || !packValue.MatchString(d.Version):
				errs = append(errs, fmt.Errorf("%s: data %q version %q may only use letters, digits and . _ + -", where, d.Name, d.Version))
			}
			packs[d.Name] = true
		}
	}
	if f.Rhythm != nil {
		check("rhythm", f.Rhythm)
	}
	seen := map[string]bool{}
	for i := range f.Tools {
		e := &f.Tools[i]
		where := fmt.Sprintf("tools[%d] (%s)", i, e.Name)
		if seen[e.Name] {
			errs = append(errs, fmt.Errorf("%s: listed twice", where))
		}
		seen[e.Name] = true
		check(where, e)
	}
	return errors.Join(errs...)
}

// write saves f at path atomically: tools and data sorted by name, two
// spaces of indent and a trailing newline, so the file diffs well in
// review.
func (f *lockFile) write(path string) error {
	f.Schema = lockSchema
	slices.SortFunc(f.Tools, func(a, b lockEntry) int { return strings.Compare(a.Name, b.Name) })
	for _, e := range f.entries() {
		slices.SortFunc(e.Data, func(a, b dataPack) int { return strings.Compare(a.Name, b.Name) })
	}
	if f.Tools == nil {
		f.Tools = []lockEntry{}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+lockFileName+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone after the rename
	if _, err := tmp.Write(b.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil { //nolint:gosec // a lock is shared through the repository
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// clone returns a deep copy of f.
func (f *lockFile) clone() (*lockFile, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	var c lockFile
	return &c, json.Unmarshal(data, &c)
}

// entries returns rhythm's entry, if any, then the tools'.
func (f *lockFile) entries() []*lockEntry {
	var list []*lockEntry
	if f.Rhythm != nil {
		list = append(list, f.Rhythm)
	}
	for i := range f.Tools {
		list = append(list, &f.Tools[i])
	}
	return list
}

// entry returns the entry for name ("rhythm" or a tool), or nil.
func (f *lockFile) entry(name string) *lockEntry {
	for _, e := range f.entries() {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// lockDiff lists what changed from old to new, rhythm first and then the
// tools by name, as lines such as "~ tuner 0.3.1 → 0.3.2".
func lockDiff(old, new *lockFile) []string {
	names := []string{selfName}
	for _, f := range []*lockFile{old, new} {
		for _, e := range f.Tools {
			if !slices.Contains(names[1:], e.Name) {
				names = append(names, e.Name)
			}
		}
	}
	slices.Sort(names[1:])
	none := func(s string) string {
		if s == "" {
			return "(none)"
		}
		return s
	}
	var lines []string
	for _, name := range names {
		a, b := old.entry(name), new.entry(name)
		switch {
		case a == nil && b == nil:
			continue
		case a == nil:
			lines = append(lines, fmt.Sprintf("+ %s %s", name, b.Version))
			continue
		case b == nil:
			lines = append(lines, fmt.Sprintf("- %s %s", name, a.Version))
			continue
		case a.Version != b.Version:
			lines = append(lines, fmt.Sprintf("~ %s %s → %s", name, a.Version, b.Version))
		case !maps.Equal(a.Platforms, b.Platforms):
			// The same version with other archives: a release replaced
			// after it was locked. Worth a reviewer's eye.
			lines = append(lines, fmt.Sprintf("! %s %s: archives changed for the same version", name, b.Version))
		}
		packs := map[string][2]*dataPack{}
		for i := range a.Data {
			packs[a.Data[i].Name] = [2]*dataPack{&a.Data[i], nil}
		}
		for i := range b.Data {
			p := packs[b.Data[i].Name]
			p[1] = &b.Data[i]
			packs[b.Data[i].Name] = p
		}
		for _, pack := range slices.Sorted(maps.Keys(packs)) {
			p := packs[pack]
			var from, to, fromSum, toSum string
			if p[0] != nil {
				from, fromSum = p[0].Version, p[0].SHA256
			}
			if p[1] != nil {
				to, toSum = p[1].Version, p[1].SHA256
			}
			switch {
			case from != to:
				lines = append(lines, fmt.Sprintf("~ %s data %s %s → %s", name, pack, none(from), none(to)))
			case fromSum != toSum:
				lines = append(lines, fmt.Sprintf("! %s data %s %s: content changed for the same version", name, pack, to))
			}
		}
	}
	return lines
}

// lockEntryFor pins rel of the program binary from repo: its version and
// the archive of every platform the release has.
func lockEntryFor(ctx context.Context, u *update.Updater, name string, rel *update.Release) (lockEntry, error) {
	sums, err := releaseChecksums(ctx, u, rel)
	if err != nil {
		return lockEntry{}, err
	}
	platforms := platformsOf(u.Binary, sums)
	if len(platforms) == 0 {
		return lockEntry{}, fmt.Errorf("release %s: checksums.txt lists no %s archives", rel.Version, u.Binary)
	}
	return lockEntry{Name: name, Repo: u.Repo, Version: rel.Version, Platforms: platforms}, nil
}

const lockSynopsis = `Usage: rhythm lock [flags]

Writes rhythm.lock in the current folder from what is installed: rhythm
and each tool's version, the sha256 of every platform's release archive
and, for tools that report it, the version of their data. Commit the file
and run "rhythm install --locked" on every laptop and pipeline to get the
same versions and data. Finding each tool's data refreshes it first.

Exit status: 0 when the lock was written, 1 when any program couldn't be
locked (nothing is written then), 2 for a usage error and 130 when
interrupted.`

func cmdLock(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("lock", lockSynopsis)
	noData := flags.Bool("no-data", false, "don't record the tools' data")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	if len(pos) > 0 {
		fmt.Fprintln(stderr, "rhythm: lock takes no names; it locks everything installed")
		return exitUsage
	}
	path, err := filepath.Abs(lockFileName)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}

	defer beginRun(&runOptions{})() // a signature check of its own, built once
	ctx, stop := signalContext()
	defer stop()
	targets, _ := findTargets(ctx, nil)
	f := &lockFile{}
	var nets netReport
	failed := false
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		switch {
		case t.current == unknownVersion:
			fmt.Fprintf(stderr, "%-18s version unknown (%s), can't lock it\n", t.binary, t.probe)
			failed = true
			continue
		case !update.IsRelease(t.current) && t.tool == selfName:
			fmt.Fprintf(stderr, "%-18s %s is a development build; rhythm itself isn't locked\n", t.binary, t.current)
			continue
		case !update.IsRelease(t.current):
			fmt.Fprintf(stderr, "%-18s %s is a development build, can't lock it\n", t.binary, t.current)
			failed = true
			continue
		case nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")):
			failed = true
			continue
		}
		u := updater(t.repo, t.binary, t.current)
		e, err := lockTarget(ctx, u, t)
		if err != nil {
			if ctx.Err() == nil && !nets.add(t.binary, err) {
				fmt.Fprintf(stderr, "%-18s %s\n", t.binary, describe(err))
			}
			failed = true
			continue
		}
		if !*noData && t.tool != selfName && !t.hung {
			if e.Data, err = lockData(ctx, t, stdout, stderr); err != nil {
				failed = true
				continue
			}
		}
		if t.tool == selfName {
			f.Rhythm = &e
		} else {
			f.Tools = append(f.Tools, e)
		}
		fmt.Fprintf(stdout, "%-18s %s  %s\n", t.binary, e.Version, platformList(e.Platforms))
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: lock interrupted")
		return exitInterrupted
	}
	nets.print(stderr)
	if failed {
		fmt.Fprintf(stderr, "rhythm: %s not written\n", lockFileName)
		return 1
	}
	if err := f.write(path); err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Wrote", path)
	return 0
}

// lockTarget pins the release t runs.
func lockTarget(ctx context.Context, u *update.Updater, t target) (lockEntry, error) {
	rel, err := resolveSpec(ctx, u, t.current, 0)
	if err != nil {
		return lockEntry{}, err
	}
	return lockEntryFor(ctx, u, t.tool, rel.Release)
}

// lockData refreshes t's data and returns the packs it reports, for the
// lock. A tool that isn't on kit/cli.Main can't report its data, so it is
// locked without it.
func lockData(ctx context.Context, t target, stdout, stderr io.Writer) ([]dataPack, error) {
	if !t.kitCLI {
		return nil, nil
	}
	packs, err := refreshData(ctx, t, nil, stdout, stderr)
	if err != nil {
		if ctx.Err() == nil {
			fmt.Fprintln(stderr, "  lock without the tools' data with: rhythm lock --no-data")
		}
		return nil, err
	}
	for _, p := range packs {
		if !packValue.MatchString(p.Name) || !packValue.MatchString(p.Version) || !sha256Hex.MatchString(p.SHA256) {
			fmt.Fprintf(stderr, "%s reported data %q version %q sha256 %q, which can't be locked\n", t.binary, p.Name, p.Version, p.SHA256)
			return nil, errDataFailed
		}
	}
	return packs, nil
}

// platformList is "darwin_arm64, linux_amd64, windows_amd64".
func platformList(platforms map[string]string) string {
	return strings.Join(slices.Sorted(maps.Keys(platforms)), ", ")
}

// lockedBinary returns the binary bin inside a locked archive, read the
// way kit/update reads it: the first regular file of that base name.
func lockedBinary(archive []byte, bin string) ([]byte, error) {
	if zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive))); err == nil {
		for _, f := range zr.File {
			if path.Base(f.Name) == bin && !f.FileInfo().IsDir() {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer func() { _ = rc.Close() }()
				return io.ReadAll(io.LimitReader(rc, maxArchive))
			}
		}
		return nil, fmt.Errorf("no %s in the archive", bin)
	}
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == bin {
			return io.ReadAll(io.LimitReader(tr, maxArchive))
		}
	}
}

// matchesLocked reports whether the file at dest is the binary in the
// locked archive, byte for byte. A binary that only reports the locked
// version, such as one rebuilt or edited in place, doesn't match.
func matchesLocked(dl *lockedDownload, dest, bin string) bool {
	want, err := lockedBinary(dl.archive, bin)
	if err != nil {
		return false
	}
	got, err := os.ReadFile(dest) //nolint:gosec // the binary rhythm is about to replace
	return err == nil && bytes.Equal(got, want)
}
