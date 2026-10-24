// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// windowsSwap forces the dest.old swap and routes renames through fail,
// which may return an error for a rename it wants to break.
func windowsSwap(t *testing.T, fail func(from, to string) error) {
	t.Helper()
	oldRename, oldSwap := rename, useOldSwap
	t.Cleanup(func() { rename, useOldSwap = oldRename, oldSwap })
	useOldSwap = true
	rename = func(from, to string) error {
		if fail != nil {
			if err := fail(from, to); err != nil {
				return err
			}
		}
		return os.Rename(from, to)
	}
}

func installed(t *testing.T) string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	if err := os.WriteFile(dest, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dest
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWindowsSwap(t *testing.T) {
	windowsSwap(t, nil)
	dest := installed(t)
	if err := replace(dest, []byte("NEW BINARY")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "NEW BINARY" {
		t.Errorf("binary = %q", got)
	}
	// The running binary stays as .old until the next update.
	if got, _ := os.ReadFile(dest + ".old"); string(got) != "OLD BINARY" {
		t.Errorf(".old = %q", got)
	}
	if err := replace(dest, []byte("NEWER BINARY")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest + ".old"); string(got) != "NEW BINARY" {
		t.Errorf(".old after a second update = %q, want the previous binary", got)
	}
}

func TestWindowsSwapFreshInstall(t *testing.T) {
	windowsSwap(t, nil)
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	if err := replace(dest, []byte("NEW BINARY")); err != nil {
		t.Fatal(err)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("files = %v, want only the binary", names)
	}
}

func TestWindowsSwapRestoresOnFailure(t *testing.T) {
	locked := errors.New("file is locked by another process")
	dest := installed(t)
	windowsSwap(t, func(from, to string) error {
		if to == dest && strings.Contains(filepath.Base(from), ".new-") {
			return locked
		}
		return nil
	})
	err := replace(dest, []byte("NEW BINARY"))
	if !errors.Is(err, locked) || !strings.Contains(err.Error(), dest) {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Errorf("binary = %q, want the old one restored", got)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 || names[0] != "rhythm-tuner" {
		t.Errorf("files = %v, want only the restored binary", names)
	}
}

func TestWindowsSwapReportsBothFailures(t *testing.T) {
	locked := errors.New("new binary is locked")
	stuck := errors.New("old binary is stuck")
	dest := installed(t)
	windowsSwap(t, func(from, to string) error {
		switch {
		case to == dest && strings.HasSuffix(from, ".old"):
			return stuck
		case to == dest:
			return locked
		}
		return nil
	})
	err := replace(dest, []byte("NEW BINARY"))
	if !errors.Is(err, locked) || !errors.Is(err, stuck) {
		t.Fatalf("err = %v, want both failures", err)
	}
	for _, want := range []string{dest, "new binary is locked", "old binary is stuck"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not mention %q", err, want)
		}
	}
	// The old binary is still there under .old for the user to recover.
	if got, _ := os.ReadFile(dest + ".old"); string(got) != "OLD BINARY" {
		t.Errorf(".old = %q", got)
	}
}

func TestStaleOldIsRemoved(t *testing.T) {
	for _, swap := range []bool{false, true} {
		oldSwap := useOldSwap
		useOldSwap = swap
		srv := fakeGitHub(t, []byte("NEW BINARY"), false)
		u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
		rel, err := u.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		dest := installed(t)
		if err := os.WriteFile(dest+".old", []byte("STALE"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := u.Install(context.Background(), rel, dest); err != nil {
			t.Fatal(err)
		}
		old, err := os.ReadFile(dest + ".old")
		switch {
		case !swap && !errors.Is(err, os.ErrNotExist):
			t.Errorf("unix: stale .old left behind (%q, %v)", old, err)
		case swap && string(old) != "OLD BINARY":
			t.Errorf("windows: .old = %q, want the binary just replaced", old)
		}
		useOldSwap = oldSwap
	}

	// Without dest, .old may be the only copy left, so it stays.
	lone := filepath.Join(t.TempDir(), "rhythm-tuner")
	if err := os.WriteFile(lone+".old", []byte("ONLY COPY"), 0o755); err != nil {
		t.Fatal(err)
	}
	removeStaleOld(lone)
	if got, _ := os.ReadFile(lone + ".old"); string(got) != "ONLY COPY" {
		t.Errorf(".old without dest = %q, want it kept", got)
	}
}

func TestUnixSwapIsOneRename(t *testing.T) {
	oldRename, oldSwap := rename, useOldSwap
	t.Cleanup(func() { rename, useOldSwap = oldRename, oldSwap })
	useOldSwap = false
	calls := 0
	rename = func(from, to string) error { calls++; return os.Rename(from, to) }
	dest := installed(t)
	if err := replace(dest, []byte("NEW BINARY")); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("%d renames, want 1", calls)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("files = %v", names)
	}
}
