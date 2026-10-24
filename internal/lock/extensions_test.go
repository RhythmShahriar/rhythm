// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package lock

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/config"
)

func TestExtensions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	s, err := OpenExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ExtensionsFile); s.Path() != want {
		t.Fatalf("Path() = %q, want %q", s.Path(), want)
	}
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("a new store lists %v, %v", list, err)
	}
	if _, err := os.Stat(s.Path()); err == nil {
		t.Error("List created the file")
	}

	for _, x := range []Extension{
		{Name: "zap", Repo: "someone/rhythm-zap"},
		{Name: "aaa", Repo: "other/rhythm-aaa", Signer: "release-bot"},
		{Name: "zap", Repo: "fork/rhythm-zap"}, // a reinstall replaces
	} {
		if err := s.Add(x); err != nil {
			t.Fatalf("Add(%+v): %v", x, err)
		}
	}
	want := []Extension{{"aaa", "other/rhythm-aaa", "release-bot"}, {"zap", "fork/rhythm-zap", ""}}
	if list, err := s.List(); err != nil || !slices.Equal(list, want) {
		t.Errorf("List() = %+v, %v; want %+v", list, err, want)
	}
	data, _ := os.ReadFile(s.Path())
	wantFile := `{
  "schema": "rhythm-extensions/v1",
  "extensions": [
    {
      "name": "aaa",
      "repo": "other/rhythm-aaa",
      "signer": "release-bot"
    },
    {
      "name": "zap",
      "repo": "fork/rhythm-zap"
    }
  ]
}
`
	if string(data) != wantFile {
		t.Errorf("extensions.json =\n%s", data)
	}
	if fi, _ := os.Stat(s.Path()); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}

	if removed, err := s.Remove("zap"); !removed || err != nil {
		t.Errorf("Remove(zap) = %v, %v", removed, err)
	}
	if removed, err := s.Remove("zap"); removed || err != nil {
		t.Errorf("removing zap twice = %v, %v", removed, err)
	}
	if list, _ := s.List(); !slices.Equal(list, want[:1]) {
		t.Errorf("after Remove: %+v", list)
	}
	if removed, _ := s.Remove("aaa"); !removed {
		t.Error("Remove(aaa) found nothing")
	}
	if data, _ := os.ReadFile(s.Path()); !strings.Contains(string(data), `"extensions": []`) {
		t.Errorf("an emptied store wrote %s", data)
	}
}

func TestExtensionsRejects(t *testing.T) {
	s := ExtensionsAt(filepath.Join(t.TempDir(), "nested", ExtensionsFile))
	for _, x := range []Extension{
		{Name: "../evil", Repo: "a/b"},
		{Name: "", Repo: "a/b"},
		{Name: "zap", Repo: "zap"},
		{Name: "zap", Repo: "a/.."}, // the repo becomes part of the release API path
		{Name: "zap", Repo: "a/."},
	} {
		if err := s.Add(x); err == nil {
			t.Errorf("Add(%+v) took it", x)
		}
	}
	if _, err := os.Stat(s.Path()); err == nil {
		t.Error("a refused Add wrote the file")
	}

	for name, content := range map[string]string{
		"corrupt":       "{",
		"wrong schema":  `{"schema":"rhythm-extensions/v9","extensions":[]}`,
		"unknown field": `{"schema":"rhythm-extensions/v1","extensions":[],"x":1}`,
		"bad entry":     `{"schema":"rhythm-extensions/v1","extensions":[{"name":"A","repo":"a/b"}]}`,
	} {
		path := filepath.Join(t.TempDir(), ExtensionsFile)
		os.WriteFile(path, []byte(content), 0o600)
		s := ExtensionsAt(path)
		if _, err := s.List(); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%s: List() = %v", name, err)
		}
		// A broken file is never overwritten by a change.
		if err := s.Add(Extension{Name: "zap", Repo: "a/b"}); err == nil {
			t.Errorf("%s: Add wrote over the file", name)
		}
	}

	t.Setenv(config.EnvConfigDir, "relative/dir")
	if _, err := OpenExtensions(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Errorf("a relative %s gave %v", config.EnvConfigDir, err)
	}
}

func TestOpenExtensionsDefault(t *testing.T) {
	t.Setenv(config.EnvConfigDir, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, err := OpenExtensions()
	if err != nil {
		t.Skipf("no config folder here: %v", err)
	}
	if want := filepath.Join("rhythm", ExtensionsFile); !strings.HasSuffix(s.Path(), want) {
		t.Errorf("Path() = %q, want it to end in %q", s.Path(), want)
	}
}
