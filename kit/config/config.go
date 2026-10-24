// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package config locates the per-user Rhythm directories.
//
// By default the config directory is "rhythm" inside os.UserConfigDir
// (~/.config/rhythm on Linux, ~/Library/Application Support/rhythm on
// macOS) and each tool's cache is "rhythm/<tool>" inside os.UserCacheDir.
//
// Two environment variables move them, for CI runners and tests (macOS
// ignores XDG_CONFIG_HOME, so it cannot be used to keep tests out of the
// real home folder):
//
//   - RHYTHM_CONFIG_DIR replaces the whole config directory.
//   - RHYTHM_CACHE_DIR replaces the cache root; a tool's cache is
//     RHYTHM_CACHE_DIR/<tool>.
//
// Both must be absolute paths. A relative value is an error rather than
// being resolved against whatever directory the tool happens to run in.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Environment variables that override the default directories.
const (
	EnvConfigDir = "RHYTHM_CONFIG_DIR"
	EnvCacheDir  = "RHYTHM_CACHE_DIR"
)

// Dir returns the Rhythm config directory: RHYTHM_CONFIG_DIR when set,
// otherwise "rhythm" inside the OS config directory. It is not created.
func Dir() (string, error) {
	if d, ok, err := override(EnvConfigDir); ok {
		return d, err
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "rhythm"), nil
}

// CacheDir returns the Rhythm cache directory for one tool:
// RHYTHM_CACHE_DIR/<tool> when that is set, otherwise
// ~/.cache/rhythm/<tool> (or the OS equivalent). It is not created.
func CacheDir(tool string) (string, error) {
	if d, ok, err := override(EnvCacheDir); ok {
		if err != nil {
			return "", err
		}
		return filepath.Join(d, tool), nil
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "rhythm", tool), nil
}

// override reads an environment variable that replaces a directory. ok is
// false when it is unset or empty, so an empty value falls back to the
// default the same way an unset one does.
func override(name string) (dir string, ok bool, err error) {
	v := os.Getenv(name)
	if v == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(v) {
		return "", true, fmt.Errorf("%s must be an absolute path, got %q", name, v)
	}
	return filepath.Clean(v), true, nil
}

// FirstRun reports whether key has not been seen on this machine before
// and records it so later calls return false. It reports false when the
// config directory cannot be found, so a broken setup never repeats a
// first-run message on every start.
func FirstRun(key string) bool {
	dir, err := Dir()
	if err != nil {
		return false
	}
	marker := filepath.Join(dir, "seen", key)
	if _, err := os.Stat(marker); err == nil {
		return false
	}
	if os.MkdirAll(filepath.Dir(marker), 0o700) == nil {
		_ = os.WriteFile(marker, nil, 0o600)
	}
	return true
}
