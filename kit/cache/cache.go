// Package cache is a small on-disk cache for data a tool fetches from the
// network: fresh entries are reused, stale ones refreshed, and when the
// network is down the last good copy is used.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// ErrOffline means the data isn't cached and the store is offline.
var ErrOffline = errors.New("not cached, and offline mode is on")

// Store caches JSON-encodable values under Dir/<kind>/<key>.json.
type Store struct {
	Dir     string        // "" disables the disk cache
	TTL     time.Duration // refresh entries older than this
	Offline bool          // never fetch; use cached data however old
	Force   bool          // always fetch, ignoring the TTL

	mu  sync.Mutex
	mem map[string]*memo
}

type memo struct {
	once  sync.Once
	v     any
	stale bool
	err   error
}

type entry[T any] struct {
	Fetched time.Time `json:"fetched"`
	Data    T         `json:"data"`
}

// Load returns fresh cached data for kind/key, else fetches and stores it,
// else falls back to stale cached data (stale reports that). Repeated and
// concurrent calls for one key in a process share a single lookup. A
// forced refresh reports a failed fetch instead of falling back.
func Load[T any](s *Store, ctx context.Context, kind, key string, fetch func(context.Context) (T, error)) (T, bool, error) {
	id := kind + "/" + key
	s.mu.Lock()
	if s.mem == nil {
		s.mem = map[string]*memo{}
	}
	m := s.mem[id]
	if m == nil {
		m = &memo{}
		s.mem[id] = m
	}
	s.mu.Unlock()

	m.once.Do(func() {
		path := s.path(kind, key)
		var cached entry[T]
		have := path != "" && ReadJSON(path, &cached) == nil
		age := time.Since(cached.Fetched)
		switch {
		case have && s.Offline:
			m.v, m.stale = cached.Data, age > s.TTL
			return
		case have && !s.Force && age < s.TTL:
			m.v = cached.Data
			return
		case s.Offline:
			m.err = ErrOffline
			return
		}
		data, err := fetch(ctx)
		if err != nil {
			if have && !s.Force {
				m.v, m.stale = cached.Data, true
				return
			}
			m.err = err
			return
		}
		m.v = data
		if path != "" {
			_ = WriteJSON(path, entry[T]{Fetched: time.Now().UTC(), Data: data})
		}
	})
	if m.err != nil {
		var zero T
		return zero, false, m.err
	}
	return m.v.(T), m.stale, nil
}

var unsafeKey = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (s *Store) path(kind, key string) string {
	if s.Dir == "" || key == "" || key == "." || key == ".." {
		return ""
	}
	return filepath.Join(s.Dir, filepath.FromSlash(kind), unsafeKey.ReplaceAllString(key, "_")+".json")
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

// WriteFile writes data atomically, creating the directory.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
