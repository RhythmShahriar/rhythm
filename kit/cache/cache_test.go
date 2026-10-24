// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
	if got := ErrOffline.Error(); got != "not cached and offline mode is on" {
		t.Errorf("ErrOffline = %q", got)
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

func TestLoadRetriesAfterError(t *testing.T) {
	s := &Store{}
	calls := 0
	fetch := func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("down")
		}
		return "ok", nil
	}
	if _, _, err := Load(s, context.Background(), "k", "a", fetch); err == nil {
		t.Fatal("first call should fail")
	}
	if v, _, err := Load(s, context.Background(), "k", "a", fetch); v != "ok" || err != nil || calls != 2 {
		t.Errorf("second call: %q %v calls=%d", v, err, calls)
	}
	// The success is remembered.
	if _, _, _ = Load(s, context.Background(), "k", "a", fetch); calls != 2 {
		t.Errorf("success refetched: calls=%d", calls)
	}
}

// A stale copy returned because the fetch failed is not remembered, so
// the next call in the same process tries the network again.
func TestStaleFallbackRetried(t *testing.T) {
	dir := t.TempDir()
	if err := WriteJSON((&Store{Dir: dir}).path("k", "a"), entry[string]{Fetched: time.Now().Add(-2 * time.Hour), Data: "old"}); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: dir, TTL: time.Hour}
	calls := 0
	fetch := func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("down")
		}
		return "new", nil
	}
	if v, stale, err := Load(s, context.Background(), "k", "a", fetch); v != "old" || !stale || err != nil {
		t.Fatalf("fallback: %q %v %v", v, stale, err)
	}
	if v, stale, err := Load(s, context.Background(), "k", "a", fetch); v != "new" || stale || err != nil || calls != 2 {
		t.Errorf("after the fallback: %q %v %v calls=%d", v, stale, err, calls)
	}
}

func TestCanceledCallerDoesNotPoison(t *testing.T) {
	s := &Store{}
	// Sequential: a canceled first call is not remembered.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wait := func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	if _, _, err := Load(s, ctx, "k", "seq", wait); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled call: %v", err)
	}
	if v, _, err := Load(s, context.Background(), "k", "seq", func(context.Context) (string, error) { return "ok", nil }); v != "ok" || err != nil {
		t.Errorf("after a canceled call: %q %v", v, err)
	}

	// Concurrent: the leader gives up while another caller waits on it.
	ctx, cancel = context.WithCancel(context.Background())
	started := make(chan struct{})
	leader := make(chan error, 1)
	go func() {
		_, _, err := Load(s, ctx, "k", "conc", func(ctx context.Context) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		})
		leader <- err
	}()
	<-started
	follower := make(chan string, 1)
	go func() {
		v, _, err := Load(s, context.Background(), "k", "conc", func(context.Context) (string, error) { return "mine", nil })
		if err != nil {
			v = err.Error()
		}
		follower <- v
	}()
	time.Sleep(50 * time.Millisecond) // let the follower start waiting
	cancel()
	if err := <-leader; !errors.Is(err, context.Canceled) {
		t.Errorf("leader: %v", err)
	}
	if v := <-follower; v != "mine" {
		t.Errorf("follower got %q, want its own fetch", v)
	}

	// A waiter whose own context ends stops waiting.
	release := make(chan struct{})
	started = make(chan struct{})
	go Load(s, context.Background(), "k", "slow", func(context.Context) (string, error) {
		close(started)
		<-release
		return "late", nil
	})
	<-started
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := Load(s, ctx, "k", "slow", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("waiter: %v", err)
	}
	close(release)
}

func TestLoadSharesInFlight(t *testing.T) {
	s := &Store{Dir: t.TempDir(), TTL: time.Hour}
	var calls atomic.Int32
	release := make(chan struct{})
	fetch := func(context.Context) (int, error) {
		calls.Add(1)
		<-release
		return 42, nil
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if v, _, err := Load(s, context.Background(), "k", "shared", fetch); v != 42 || err != nil {
				t.Errorf("got %d %v", v, err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Errorf("10 callers made %d fetches, want 1", n)
	}
}

func TestLoadPanicLetsWaitersRetry(t *testing.T) {
	s := &Store{}
	func() {
		defer func() { _ = recover() }()
		Load(s, context.Background(), "k", "p", func(context.Context) (int, error) { panic("boom") })
	}()
	if v, _, err := Load(s, context.Background(), "k", "p", func(context.Context) (int, error) { return 7, nil }); v != 7 || err != nil {
		t.Errorf("after a panic: %d %v", v, err)
	}
}

func TestLoadTwoTypes(t *testing.T) {
	s := &Store{}
	i, _, err1 := Load(s, context.Background(), "k", "same", func(context.Context) (int, error) { return 1, nil })
	str, _, err2 := Load(s, context.Background(), "k", "same", func(context.Context) (string, error) { return "one", nil })
	if i != 1 || str != "one" || err1 != nil || err2 != nil {
		t.Errorf("got %d %q %v %v", i, str, err1, err2)
	}
}

func TestPathNames(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	a, b := s.path("k", "https://a.example/x?y"), s.path("k", "https_//a.example/x_y")
	if a == b {
		t.Errorf("colliding keys share %s", a)
	}
	if s.path("k", "Readme") == s.path("k", "README") {
		t.Error("case-only differences must not collide")
	}
	name := filepath.Base(s.path("k", "CON"))
	if !strings.HasPrefix(name, "CON-") || len(name) != len("CON-")+32+len(".json") {
		t.Errorf("device name: %s", name)
	}
	if name := filepath.Base(s.path("k", strings.Repeat("x", 300))); len(name) > 100 {
		t.Errorf("long key gives a %d-byte name", len(name))
	}
	if name := filepath.Base(s.path("k", "../..")); strings.HasPrefix(name, ".") || len(name) != 32+len(".json") {
		t.Errorf("dots: %s", name)
	}
	if s.path("../x", "a") != "" || (&Store{}).path("k", "a") != "" {
		t.Error("an unsafe kind or no Dir gives no path")
	}
}

func TestLongKeyRoundTrips(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("k", 300)
	if _, _, err := Load(&Store{Dir: dir, TTL: time.Hour}, context.Background(), "long", key, func(context.Context) (string, error) { return "v", nil }); err != nil {
		t.Fatal(err)
	}
	v, _, err := Load(&Store{Dir: dir, TTL: time.Hour}, context.Background(), "long", key, func(context.Context) (string, error) {
		return "", errors.New("should come from disk")
	})
	if v != "v" || err != nil {
		t.Errorf("from disk: %q %v", v, err)
	}
}

func TestFutureEntryIsStale(t *testing.T) {
	s := &Store{Dir: t.TempDir(), TTL: time.Hour}
	if err := WriteJSON(s.path("k", "a"), entry[string]{Fetched: time.Now().Add(10 * 24 * time.Hour), Data: "old"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	v, _, _ := Load(s, context.Background(), "k", "a", func(context.Context) (string, error) { calls++; return "new", nil })
	if v != "new" || calls != 1 {
		t.Errorf("future entry: %q calls=%d", v, calls)
	}
	// Offline it is still used but reported stale.
	s = &Store{Dir: s.Dir, TTL: time.Hour, Offline: true}
	_ = WriteJSON(s.path("k", "b"), entry[string]{Fetched: time.Now().Add(10 * 24 * time.Hour), Data: "old"})
	if v, stale, err := Load(s, context.Background(), "k", "b", func(context.Context) (string, error) { return "", nil }); v != "old" || !stale || err != nil {
		t.Errorf("offline future entry: %q %v %v", v, stale, err)
	}
}

func TestUnsafeKindRejected(t *testing.T) {
	for _, kind := range []string{"../x", "/abs", "", "a/../../b"} {
		if _, _, err := Load(&Store{Dir: t.TempDir()}, context.Background(), kind, "a", func(context.Context) (int, error) { return 1, nil }); err == nil {
			t.Errorf("kind %q should be rejected", kind)
		}
	}
	if _, _, err := Load(&Store{Dir: t.TempDir()}, context.Background(), "wp/plugins", "a", func(context.Context) (int, error) { return 1, nil }); err != nil {
		t.Errorf("nested kind: %v", err)
	}
}

func TestWriteFileLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	// The target is a non-empty directory, so the rename fails after the
	// temp file was written.
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(target, []byte("x")); err == nil {
		t.Fatal("rename onto a directory should fail")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("left %s behind", e.Name())
		}
	}
}

func TestWriteFileReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if err := WriteFile(filepath.Join(dir, "f.json"), []byte("x")); err == nil {
		t.Error("writing into a read-only folder should fail")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("left %v behind", entries)
	}
	if err := WriteJSON(filepath.Join(dir, "f.json"), func() {}); err == nil {
		t.Error("a value JSON can't encode should fail")
	}
	if err := WriteFile(filepath.Join(dir, "sub", "f.json"), []byte("x")); err == nil {
		t.Error("creating a folder inside a read-only one should fail")
	}
}
