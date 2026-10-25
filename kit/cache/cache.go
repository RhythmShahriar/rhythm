// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package cache is a small on-disk cache for data a tool fetches from the
// network: fresh entries are reused and stale ones refreshed. When the
// network is down the last good copy is used.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrOffline means the data isn't cached and the store is offline.
var ErrOffline = errors.New("not cached and offline mode is on")

// Store caches JSON-encodable values under
// Dir/<kind>/<readable prefix of key>-<hash of key>.json.
type Store struct {
	Dir     string        // "" disables the disk cache
	TTL     time.Duration // refresh entries older than this
	Offline bool          // never fetch; use cached data however old
	Force   bool          // always fetch, ignoring the TTL

	mu  sync.Mutex
	mem map[memoKey]*call
}

// memoKey includes the value's type, so one kind/key loaded as two types
// gets two lookups instead of a failed type assertion.
type memoKey struct {
	kind, key string
	typ       reflect.Type
}

// call is one lookup: in flight until done is closed, then kept in
// Store.mem only if its fetch succeeded (or wasn't needed).
type call struct {
	done  chan struct{}
	v     any
	stale bool
	err   error
	// abandoned is set when the lookup failed because its caller's
	// context ended. Callers that were waiting on it start their own.
	abandoned bool
	// fellBack is set when a failed fetch was answered with stale data.
	// Callers waiting on it get that data. It isn't remembered, so a
	// later call tries the network again.
	fellBack bool
}

// entry is the file format: {"fetched": RFC 3339 time in UTC, "data": the
// value}. Freshness comes from fetched rather than the file's mtime, so
// copying or restoring a cache folder doesn't make old data look new.
type entry[T any] struct {
	Fetched time.Time `json:"fetched"`
	Data    T         `json:"data"`
}

// Load returns fresh cached data for kind/key, else fetches and stores it,
// else falls back to stale cached data (stale reports that). A forced
// refresh reports a failed fetch instead of falling back. An entry dated
// in the future counts as stale.
//
// Within a process, concurrent calls for one kind, key and type share a
// single lookup and a successful result is remembered for later calls.
// A failed lookup is not remembered, so the next call tries again. Nor
// is a stale copy returned because the fetch failed. When the caller
// running the lookup gives up (its context ends), the others waiting on
// it start their own lookup instead of getting its context error.
//
// kind is a folder under Dir and must be a relative path that stays
// inside it (filepath.IsLocal); key can be any string.
func Load[T any](s *Store, ctx context.Context, kind, key string, fetch func(context.Context) (T, error)) (T, bool, error) {
	var zero T
	if !filepath.IsLocal(filepath.FromSlash(kind)) {
		return zero, false, fmt.Errorf("cache kind %q must be a relative path inside the cache folder", kind)
	}
	id := memoKey{kind, key, reflect.TypeFor[T]()}
	for {
		s.mu.Lock()
		if s.mem == nil {
			s.mem = map[memoKey]*call{}
		}
		c := s.mem[id]
		if c == nil {
			c = &call{done: make(chan struct{})}
			s.mem[id] = c
			s.mu.Unlock()
			s.lead(id, c, func() { load(s, ctx, c, kind, key, fetch) })
			return result[T](c)
		}
		s.mu.Unlock()
		select {
		case <-c.done:
		case <-ctx.Done():
			return zero, false, ctx.Err()
		}
		if !c.abandoned {
			return result[T](c)
		}
	}
}

// lead runs a lookup as the caller that started it, then publishes the
// result: kept for later calls on success, dropped on failure or a
// stale fallback. If the lookup panics, the callers waiting on it start
// their own.
func (s *Store) lead(id memoKey, c *call, run func()) {
	finished := false
	defer func() {
		c.abandoned = c.abandoned || !finished
		s.mu.Lock()
		if c.err != nil || c.abandoned || c.fellBack {
			delete(s.mem, id)
		}
		s.mu.Unlock()
		close(c.done)
	}()
	run()
	finished = true
}

func result[T any](c *call) (T, bool, error) {
	if c.err != nil {
		var zero T
		return zero, false, c.err
	}
	return c.v.(T), c.stale, nil
}

// load runs one lookup into c.
func load[T any](s *Store, ctx context.Context, c *call, kind, key string, fetch func(context.Context) (T, error)) {
	path := s.path(kind, key)
	var cached entry[T]
	have := path != "" && ReadJSON(path, &cached) == nil
	age := time.Since(cached.Fetched)
	fresh := age >= 0 && age < s.TTL
	switch {
	case have && s.Offline:
		c.v, c.stale = cached.Data, !fresh
		return
	case have && !s.Force && fresh:
		c.v = cached.Data
		return
	case s.Offline:
		c.err = ErrOffline
		return
	}
	data, err := fetch(ctx)
	if err != nil {
		c.abandoned = ctx.Err() != nil
		if have && !s.Force {
			c.v, c.stale, c.fellBack = cached.Data, true, true
			return
		}
		c.err = err
		return
	}
	c.v = data
	if path != "" {
		_ = WriteJSON(path, entry[T]{Fetched: time.Now().UTC(), Data: data})
	}
}

// unsafeKey matches characters left out of a file name's readable prefix.
var unsafeKey = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// maxPrefix is the most characters of the key kept readable in a file
// name, well under any file system's name limit.
const maxPrefix = 40

// path names the file for kind/key, or "" when the disk cache is off.
// The name ends in a hash of the whole key, so keys that differ only in
// characters a file name can't hold, or only in case, or that are too
// long for a file name, still get files of their own. The readable
// prefix only helps a person browsing the folder. A key that may carry a
// credential (a query, a fragment, user:password@ or name=value, as in
// ?apikey=...) gets no prefix, so no part of it lands in a file name.
func (s *Store) path(kind, key string) string {
	if s.Dir == "" || !filepath.IsLocal(filepath.FromSlash(kind)) {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:16])
	prefix := ""
	if !strings.ContainsAny(key, "?#@=") {
		prefix = strings.TrimLeft(unsafeKey.ReplaceAllString(key, "_"), "._")
	}
	if len(prefix) > maxPrefix {
		prefix = prefix[:maxPrefix]
	}
	if prefix != "" {
		name = prefix + "-" + name
	}
	return filepath.Join(s.Dir, filepath.FromSlash(kind), name+".json")
}

// ReadJSON decodes the JSON file at path into v.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // callers build paths from their own cache or project dirs
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteJSON writes v as JSON atomically (temp file + rename), creating the
// directory, so a crash never leaves a torn file.
func WriteJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteFile(path, data)
}

// WriteFile writes data atomically, creating the directory. The temp
// file is flushed to disk before it is renamed into place and the
// directory is flushed after, where the OS can. A crash then leaves the
// old file or the new one, never an empty one. The temp file is removed on any
// failure, so none are left behind.
func WriteFile(path string, data []byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := syncFile(tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// The new file is in place; a directory that can't be flushed (some
	// file systems refuse) doesn't undo that.
	syncDir(filepath.Dir(path))
	return nil
}

// syncFile and syncDir flush a file and a directory to disk; tests
// replace them.
var (
	syncFile = (*os.File).Sync
	syncDir  = func(dir string) {
		// Windows can't open a directory for flushing; NTFS orders the
		// rename itself.
		if runtime.GOOS == "windows" {
			return
		}
		if d, err := os.Open(dir); err == nil { //nolint:gosec // the cache's own folder
			_ = d.Sync()
			_ = d.Close()
		}
	}
)
