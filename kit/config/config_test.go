// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirOverride(t *testing.T) {
	d := t.TempDir()
	t.Setenv(EnvConfigDir, d)
	got, err := Dir()
	if err != nil || got != d {
		t.Fatalf("Dir() = %q, %v; want %q", got, err, d)
	}
	t.Setenv(EnvConfigDir, d+string(filepath.Separator))
	if got, _ := Dir(); got != d {
		t.Errorf("trailing separator: Dir() = %q, want %q", got, d)
	}
}

func TestCacheDirOverride(t *testing.T) {
	d := t.TempDir()
	t.Setenv(EnvCacheDir, d)
	got, err := CacheDir("tuner")
	if want := filepath.Join(d, "tuner"); err != nil || got != want {
		t.Fatalf("CacheDir() = %q, %v; want %q", got, err, want)
	}
}

func TestRelativeOverrideIsAnError(t *testing.T) {
	t.Setenv(EnvConfigDir, "rel/config")
	t.Setenv(EnvCacheDir, "./cache")
	if d, err := Dir(); err == nil || !strings.Contains(err.Error(), EnvConfigDir) {
		t.Errorf("Dir() = %q, %v; want an error naming %s", d, err, EnvConfigDir)
	}
	if d, err := CacheDir("tuner"); err == nil || !strings.Contains(err.Error(), EnvCacheDir) {
		t.Errorf("CacheDir() = %q, %v; want an error naming %s", d, err, EnvCacheDir)
	}
	if FirstRun("x") {
		t.Error("FirstRun with an invalid config dir = true, want false")
	}
}

func TestDefaultsEndInRhythm(t *testing.T) {
	t.Setenv(EnvConfigDir, "")
	t.Setenv(EnvCacheDir, "")
	// Point the OS lookups at a temp home so the test never depends on
	// the developer's real folders.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
	t.Setenv("LocalAppData", filepath.Join(home, "localappdata"))

	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(d) != "rhythm" {
		t.Errorf("Dir() = %q, want it to end in rhythm", d)
	}
	c, err := CacheDir("soundcheck")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(c) != "soundcheck" || filepath.Base(filepath.Dir(c)) != "rhythm" {
		t.Errorf("CacheDir() = %q, want it to end in rhythm/soundcheck", c)
	}
}

func TestFirstRun(t *testing.T) {
	d := t.TempDir()
	t.Setenv(EnvConfigDir, d)
	if !FirstRun("welcome") {
		t.Fatal("first FirstRun = false, want true")
	}
	if FirstRun("welcome") {
		t.Fatal("second FirstRun = true, want false")
	}
	if !FirstRun("other") {
		t.Error("FirstRun for a new key = false, want true")
	}
	if _, err := os.Stat(filepath.Join(d, "seen", "welcome")); err != nil {
		t.Errorf("marker not written inside the override: %v", err)
	}
}
