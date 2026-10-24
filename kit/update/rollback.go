// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cache"
)

// ErrNoPrevious is returned by Rollback when there is no <binary>.prev to
// go back to.
var ErrNoPrevious = errors.New("no previous version to roll back to")

// prevRecord is the <binary>.prev.json sidecar: what <binary>.prev is and
// what replaced it.
type prevRecord struct {
	Version    string     `json:"version"` // of <binary>.prev; "" when unknown
	SHA256     string     `json:"sha256"`  // hex sha256 of <binary>.prev
	ReplacedAt time.Time  `json:"replaced_at"`
	ReplacedBy *binRecord `json:"replaced_by,omitempty"` // the binary that took its place
}

type binRecord struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// swapInfo names the versions on both sides of a replace, for the
// sidecar. Either may be "" when unknown.
type swapInfo struct {
	Old, New string
}

// Rollback swaps the binary at dest with dest.prev, the one the last
// install or rollback replaced. It returns the versions it went from and
// to (either "" when the sidecar doesn't know). A second Rollback swaps
// back. It returns ErrNoPrevious when there is no dest.prev. It refuses a
// dest.prev whose sha256 no longer matches the one recorded when it was
// set aside.
//
// dest always holds a whole, working binary: if a rename fails half way
// the first one is undone, as Install's Windows swap does.
func Rollback(dest string) (from, to string, err error) {
	prev := dest + ".prev"
	if _, err := os.Stat(prev); errors.Is(err, os.ErrNotExist) {
		return "", "", ErrNoPrevious
	} else if err != nil {
		return "", "", err
	}
	prevSum, err := hashFile(prev)
	if err != nil {
		return "", "", err
	}
	var rec prevRecord
	if cache.ReadJSON(prev+".json", &rec) != nil {
		rec = prevRecord{} // no sidecar, or a corrupt one: versions unknown
	}
	if rec.SHA256 != "" && rec.SHA256 != prevSum {
		return "", "", fmt.Errorf("%s has changed since it was set aside (sha256 differs); not rolling back", prev)
	}
	curSum, err := hashFile(dest)
	if err != nil {
		return "", "", err
	}
	// The recorded version of dest only counts if dest is still that
	// binary, not one installed by hand since.
	if rec.ReplacedBy != nil && rec.ReplacedBy.SHA256 == curSum {
		from = rec.ReplacedBy.Version
	}
	to = rec.Version
	if err := swapFiles(dest, prev); err != nil {
		return "", "", fmt.Errorf("rolling back %s: %w", dest, err)
	}
	writePrevRecord(dest, prevRecord{
		Version:    from,
		SHA256:     curSum,
		ReplacedAt: time.Now().UTC(),
		ReplacedBy: &binRecord{Version: to, SHA256: prevSum},
	})
	return from, to, nil
}

// swapFiles exchanges dest and prev. On Unix, dest is replaced by one
// rename, so it is never missing; on Windows it must be renamed away
// first, as in replace.
func swapFiles(dest, prev string) error {
	if useOldSwap {
		return swapViaOld(dest, prev)
	}
	// Links (or copies) of both, so each step is one rename and undoing
	// one is one rename too.
	newDest, err := snapshot(prev)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(newDest) }()
	oldDest, err := snapshot(dest)
	if err != nil {
		return err
	}
	if err := rename(newDest, dest); err != nil {
		_ = os.Remove(oldDest)
		return err
	}
	if err := rename(oldDest, prev); err != nil {
		if uerr := rename(oldDest, dest); uerr != nil {
			// dest works (it is the previous version now); keep the other
			// binary where the user can find it.
			return errors.Join(err, fmt.Errorf("restoring %s: %w; the replaced binary is at %s", dest, uerr, oldDest))
		}
		return err
	}
	return nil
}

// swapViaOld is swapFiles for Windows: dest → dest.old, prev → dest,
// dest.old → prev, undoing the earlier steps when a later one fails.
func swapViaOld(dest, prev string) error {
	old := dest + ".old"
	_ = os.Remove(old)
	if err := rename(dest, old); err != nil {
		return err
	}
	if err := rename(prev, dest); err != nil {
		if rerr := rename(old, dest); rerr != nil {
			return errors.Join(err, fmt.Errorf("restoring %s: %w", old, rerr))
		}
		return err
	}
	if err := rename(old, prev); err != nil {
		// Put everything back. If that fails too, dest stays the previous
		// version (which works) while the other binary stays at .old.
		if uerr := rename(dest, prev); uerr != nil {
			return errors.Join(err, fmt.Errorf("restoring %s: %w; the replaced binary is at %s", dest, uerr, old))
		}
		if uerr := rename(old, dest); uerr != nil {
			if rerr := rename(prev, dest); rerr != nil {
				return errors.Join(err, uerr, rerr)
			}
			return errors.Join(err, fmt.Errorf("restoring %s: %w; the replaced binary is at %s", dest, uerr, old))
		}
		return err
	}
	return nil
}

// keepPrev moves src, the binary dest replaced, to dest.prev and records
// it in the sidecar. Keeping it is a convenience, so a failure here
// doesn't fail the install: it removes dest.prev instead, so a stale one
// can't be rolled back to under the wrong name.
func keepPrev(dest, src string, rec prevRecord) {
	if err := rename(src, dest+".prev"); err != nil {
		dropPrev(dest)
		return
	}
	writePrevRecord(dest, rec)
}

// writePrevRecord writes the sidecar or, failing that, removes it, so
// Rollback never trusts a record of another binary.
func writePrevRecord(dest string, rec prevRecord) {
	if err := cache.WriteJSON(dest+".prev.json", rec); err != nil {
		_ = os.Remove(dest + ".prev.json")
	}
}

func dropPrev(dest string) {
	_ = os.Remove(dest + ".prev")
	_ = os.Remove(dest + ".prev.json")
}

// snapshot makes a hard link to path under a temp name in the same
// directory (a copy where links aren't supported) and returns its name. Either way a later rename of it is atomic.
func snapshot(path string) (string, error) {
	f, err := createTemp(filepath.Dir(path), "."+filepath.Base(path)+".prev-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	if err := os.Link(path, name); err == nil {
		return name, nil
	}
	if err := copyFile(path, name, 0o755); err != nil { // an executable
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func copyFile(from, to string, perm os.FileMode) error {
	src, err := os.Open(from) //nolint:gosec // a binary we manage
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // callers pick the mode
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // a binary we manage
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
