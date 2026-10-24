// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	fetch := func(context.Context) (string, error) { calls++; return "v1", nil }
	ctx := context.Background()

	v, stale, err := Load(&Store{Dir: dir, TTL: time.Hour}, ctx, "k", "a", fetch)
	if v != "v1" || stale || err != nil || calls != 1 {
		t.Fatalf("first: %q %v %v calls=%d", v, stale, err, calls)
	}
	// Fresh on disk: no fetch.
	if v, _, _ := Load(&Store{Dir: dir, TTL: time.Hour}, ctx, "k", "a", fetch); v != "v1" || calls != 1 {
		t.Errorf("fresh cache refetched (calls=%d)", calls)
	}
	// Expired and the fetch fails: stale copy.
	fail := func(context.Context) (string, error) { return "", errors.New("down") }
	if v, stale, err := Load(&Store{Dir: dir, TTL: -1}, ctx, "k", "a", fail); v != "v1" || !stale || err != nil {
		t.Errorf("stale fallback: %q %v %v", v, stale, err)
	}
	// A forced refresh reports the failure.
	if _, _, err := Load(&Store{Dir: dir, Force: true}, ctx, "k", "a", fail); err == nil {
		t.Error("forced refresh must report a failed fetch")
	}
	// Offline: cached data however old, and no fetch.
	if v, _, err := Load(&Store{Dir: dir, TTL: -1, Offline: true}, ctx, "k", "a", fail); v != "v1" || err != nil {
		t.Errorf("offline: %q %v", v, err)
	}
	if _, _, err := Load(&Store{Dir: t.TempDir(), Offline: true}, ctx, "k", "a", fetch); !errors.Is(err, ErrOffline) {
		t.Errorf("offline without cache: %v", err)
	}
	// One lookup per key per store.
	s := &Store{}
	n := 0
	for range 3 {
		Load(s, ctx, "k", "b", func(context.Context) (int, error) { n++; return n, nil })
	}
	if n != 1 {
		t.Errorf("memo: fetched %d times", n)
	}
}
