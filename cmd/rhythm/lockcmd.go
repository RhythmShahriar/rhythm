// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/internal/lock"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

func init() {
	register(command{name: "lock", args: "[--no-data]", summary: "pin the installed versions in rhythm.lock", run: cmdLock})
}

// rhythm.lock is read, checked, compared and written by internal/lock.
// The short names below keep the commands that use it readable.
type (
	lockFile  = lock.File
	lockEntry = lock.Entry
	dataPack  = lock.DataPack
)

const lockFileName = lock.FileName

// sourceExtension marks an extension's entry in rhythm.lock.
const sourceExtension = lock.SourceExtension

// errNoLock is findLock's answer when no rhythm.lock applies.
var errNoLock = lock.ErrNotFound

// findLock returns the nearest rhythm.lock in dir or a parent, stopping at
// the git repository root. Outside a repository only dir itself counts,
// so a stray lock in a home or temp folder never applies.
func findLock(dir string) (string, error) { return lock.Find(dir) }

// readLock reads and checks the lock at path. Unknown fields are refused,
// so a newer rhythm's key is never dropped when this one rewrites it.
func readLock(path string) (*lockFile, error) { return lock.Read(path) }

// lockDiff lists what changed from old to new, rhythm first and then the
// tools by name, as lines such as "~ tuner 0.3.1 → 0.3.2".
func lockDiff(old, new *lockFile) []string {
	var lines []string
	for _, c := range lock.Diff(old, new) {
		lines = append(lines, c.String())
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
	e := lockEntry{Name: name, Repo: u.Repo, Version: rel.Version, Platforms: platforms}
	if t, ok := u.HTTP.Transport.(*explainTransport); ok && t.verifiedBy != "" {
		e.Signer = signerIdentity(t.verifiedBy)
	}
	return e, nil
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
	if exts, err := readExtensions(); err != nil {
		fmt.Fprintf(stderr, "rhythm: %v; extensions not locked\n", err)
	} else {
		more, _ := extensionTargets(ctx, exts, nil)
		targets = append(targets, more...)
	}
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
		if et, ok := u.HTTP.Transport.(*explainTransport); ok {
			et.signer = t.signer // an extension's recorded signer must sign this release too
		}
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
		if t.extension {
			e.Source = sourceExtension
			if e.Signer == "" {
				e.Signer = t.signer
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
	if err := f.Write(path); err != nil {
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
		if lock.CheckPack(p) != nil {
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
