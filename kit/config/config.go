// Package config locates the per-user Rhythm directory (~/.config/rhythm).
package config

import (
	"os"
	"path/filepath"
)

// Dir returns the Rhythm config directory. It is not created.
func Dir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "rhythm"), nil
}

// FirstRun reports whether key has not been seen on this machine before,
// and records it so later calls return false.
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
