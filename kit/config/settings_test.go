// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettings(t *testing.T) {
	d := t.TempDir()
	t.Setenv(EnvConfigDir, d)
	if v, err := Setting("update-prompt"); v != "" || err != nil {
		t.Fatalf("unset: %q, %v", v, err)
	}
	if err := SetSetting("update-prompt", "off"); err != nil {
		t.Fatal(err)
	}
	if err := SetSetting("other", "x"); err != nil {
		t.Fatal(err)
	}
	if v, err := Setting("update-prompt"); v != "off" || err != nil {
		t.Errorf("after set: %q, %v", v, err)
	}
	if err := SetSetting("update-prompt", ""); err != nil {
		t.Fatal(err)
	}
	if v, _ := Setting("update-prompt"); v != "" {
		t.Errorf("after removing: %q", v)
	}
	if v, _ := Setting("other"); v != "x" {
		t.Errorf("other setting lost: %q", v)
	}
	path := filepath.Join(d, SettingsFile)
	for _, bad := range []string{"{", `["a"]`} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Setting("update-prompt"); err == nil {
			t.Errorf("%s: no error", bad)
		}
		if err := SetSetting("update-prompt", "ask"); err == nil {
			t.Errorf("%s: SetSetting overwrote a file it couldn't read", bad)
		}
	}
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetSetting("update-prompt", "ask"); err != nil {
		t.Errorf("null file: %v", err)
	}
}

func TestSettingsRelativeDir(t *testing.T) {
	t.Setenv(EnvConfigDir, "rel")
	if _, err := Setting("x"); err == nil {
		t.Error("relative config dir: no error")
	}
	if err := SetSetting("x", "y"); err == nil {
		t.Error("relative config dir: SetSetting worked")
	}
}
