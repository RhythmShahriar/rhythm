// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/cache"
)

// installTwice installs 0.3.1 into an empty dir, then 0.3.2 over it, as
// the router does (Current is the installed version).
func installTwice(t *testing.T) string {
	t.Helper()
	api := newReleaseAPI(t, fakeRel{tag: "v0.3.1"}, fakeRel{tag: "v0.3.2"})
	u := api.updater()
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	for _, v := range []string{"0.3.1", "0.3.2"} {
		rel, err := u.ByTag(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		if err := u.Install(context.Background(), rel, dest); err != nil {
			t.Fatal(err)
		}
		u.Current = v
	}
	return dest
}

func content(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func testRollback(t *testing.T) {
	dest := installTwice(t)
	var rec prevRecord
	if err := cache.ReadJSON(dest+".prev.json", &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Version != "0.3.1" || rec.SHA256 != sum("rhythm-tuner 0.3.1") || rec.ReplacedAt.IsZero() ||
		rec.ReplacedBy == nil || rec.ReplacedBy.Version != "0.3.2" || rec.ReplacedBy.SHA256 != sum("rhythm-tuner 0.3.2") {
		t.Fatalf("sidecar = %+v", rec)
	}

	from, to, err := Rollback(dest)
	if err != nil || from != "0.3.2" || to != "0.3.1" {
		t.Fatalf("Rollback = %q, %q, %v", from, to, err)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.1" {
		t.Errorf("dest = %q", got)
	}
	if got := content(t, dest+".prev"); got != "rhythm-tuner 0.3.2" {
		t.Errorf(".prev = %q", got)
	}
	if fi, _ := os.Stat(dest); fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("not executable: %v", fi.Mode())
	}

	from, to, err = Rollback(dest)
	if err != nil || from != "0.3.1" || to != "0.3.2" {
		t.Fatalf("second Rollback = %q, %q, %v", from, to, err)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.2" {
		t.Errorf("dest after swapping back = %q", got)
	}
	if names := dirNames(t, filepath.Dir(dest)); !slices.Equal(names, []string{"rhythm-tuner", "rhythm-tuner.prev", "rhythm-tuner.prev.json"}) {
		t.Errorf("files = %v", names)
	}
}

func TestRollback(t *testing.T) {
	t.Run("unix", func(t *testing.T) {
		old := useOldSwap
		t.Cleanup(func() { useOldSwap = old })
		useOldSwap = false
		testRollback(t)
	})
	t.Run("windows", func(t *testing.T) {
		windowsSwap(t, nil)
		testRollback(t)
	})
}

func TestRollbackWithoutPrevious(t *testing.T) {
	dest := installed(t)
	if _, _, err := Rollback(dest); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("err = %v, want ErrNoPrevious", err)
	}
	// A fresh install has nothing to go back to either.
	api := newReleaseAPI(t, fakeRel{tag: "v0.3.1"})
	u := api.updater()
	rel, _ := u.ByTag(context.Background(), "0.3.1")
	fresh := filepath.Join(t.TempDir(), "rhythm-tuner")
	if err := u.Install(context.Background(), rel, fresh); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rollback(fresh); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("fresh install: err = %v, want ErrNoPrevious", err)
	}
}

func TestRollbackRefusesAChangedPrev(t *testing.T) {
	dest := installTwice(t)
	if err := os.WriteFile(dest+".prev", []byte("TAMPERED"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rollback(dest); err == nil || !strings.Contains(err.Error(), "has changed since it was set aside") {
		t.Fatalf("err = %v", err)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.2" {
		t.Errorf("dest = %q, want it untouched", got)
	}
}

// Without the sidecar the swap still works; only the versions are unknown.
// A dest installed by hand since the sidecar was written isn't named.
func TestRollbackWithoutSidecar(t *testing.T) {
	dest := installTwice(t)
	if err := os.WriteFile(dest, []byte("BY HAND"), 0o755); err != nil {
		t.Fatal(err)
	}
	from, to, err := Rollback(dest)
	if err != nil || from != "" || to != "0.3.1" {
		t.Errorf("hand-installed dest: Rollback = %q, %q, %v", from, to, err)
	}
	if err := os.Remove(dest + ".prev.json"); err != nil {
		t.Fatal(err)
	}
	from, to, err = Rollback(dest)
	if err != nil || from != "" || to != "" || content(t, dest) != "BY HAND" {
		t.Errorf("no sidecar: Rollback = %q, %q, %v", from, to, err)
	}
}

// Every rename Rollback makes is broken in turn; dest must still hold a
// whole binary each time and a failure must say so.
func TestRollbackSurvivesRenameFailures(t *testing.T) {
	for _, windows := range []bool{false, true} {
		for step := 1; step <= 3; step++ {
			dest := installTwice(t)
			broken := errors.New("injected rename failure")
			n := 0
			oldRename, oldSwap := rename, useOldSwap
			useOldSwap = windows
			rename = func(from, to string) error {
				n++
				if n == step {
					return broken
				}
				return os.Rename(from, to)
			}
			_, _, err := Rollback(dest)
			rename, useOldSwap = oldRename, oldSwap
			if !windows && step == 3 {
				// The Unix swap makes two renames; there is no third to break.
				if err != nil {
					t.Errorf("unix: err = %v", err)
				}
				continue
			}
			if !errors.Is(err, broken) {
				t.Errorf("windows=%v step %d: err = %v, want the injected failure", windows, step, err)
			}
			got := content(t, dest)
			if got != "rhythm-tuner 0.3.2" {
				t.Errorf("windows=%v step %d: dest = %q, want the binary restored", windows, step, got)
			}
			if got := content(t, dest+".prev"); got != "rhythm-tuner 0.3.1" {
				t.Errorf("windows=%v step %d: .prev = %q", windows, step, got)
			}
			// Nothing is left over: the swap was fully undone.
			if names := dirNames(t, filepath.Dir(dest)); len(names) != 3 {
				t.Errorf("windows=%v step %d: files = %v", windows, step, names)
			}
		}
	}
}

// When the undo fails too, dest still works and the error says where the
// other binary went.
func TestRollbackReportsAFailedUndo(t *testing.T) {
	for _, windows := range []bool{false, true} {
		dest := installTwice(t)
		broken := errors.New("injected rename failure")
		n := 0
		oldRename, oldSwap := rename, useOldSwap
		useOldSwap = windows
		last := 2 // the Unix swap's second rename and its undo
		if windows {
			last = 3 // the third rename and the first undo step
		}
		rename = func(from, to string) error {
			n++
			if n == last || n == last+1 {
				return broken
			}
			return os.Rename(from, to)
		}
		_, _, err := Rollback(dest)
		rename, useOldSwap = oldRename, oldSwap
		if !errors.Is(err, broken) || !strings.Contains(err.Error(), "the replaced binary is at") {
			t.Errorf("windows=%v: err = %v", windows, err)
		}
		if got := content(t, dest); got != "rhythm-tuner 0.3.1" && got != "rhythm-tuner 0.3.2" {
			t.Errorf("windows=%v: dest = %q, want a whole binary", windows, got)
		}
	}
}

// A failure while setting .prev aside never fails the install and never
// leaves a stale .prev to roll back to.
func TestInstallSurvivesAFailedPrev(t *testing.T) {
	dest := installTwice(t)
	oldRename := rename
	t.Cleanup(func() { rename = oldRename })
	rename = func(from, to string) error {
		if strings.HasSuffix(to, ".prev") {
			return errors.New("injected rename failure")
		}
		return os.Rename(from, to)
	}
	if err := replace(dest, strings.NewReader("rhythm-tuner 0.3.3"), swapInfo{Old: "0.3.2", New: "0.3.3"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.3" {
		t.Errorf("dest = %q", got)
	}
	if _, _, err := Rollback(dest); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("Rollback err = %v, want ErrNoPrevious", err)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("files = %v", names)
	}
}
