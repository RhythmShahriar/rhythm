// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package lock

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func hash(c byte) string { return strings.Repeat(string(c), 64) }

// sample is testdata/rhythm.lock with its lists out of order, as a
// caller might build it.
func sample() *File {
	return &File{
		Rhythm: &Entry{
			Name: "rhythm", Repo: "RhythmShahriar/rhythm", Version: "0.7.0",
			Platforms: map[string]string{"linux_amd64": hash('b'), "darwin_arm64": hash('a')},
		},
		Tools: []Entry{
			{
				Name: "zap", Repo: "someone/rhythm-zap", Version: "1.0.0",
				Platforms: map[string]string{"linux_amd64": hash('d')}, Source: SourceExtension,
			},
			{
				Name: "tuner", Repo: "RhythmShahriar/tuner", Version: "0.3.1-rc.2",
				Signer:    "https://github.com/RhythmShahriar/tuner/.github/workflows/release.yml@refs/tags/v0.3.1-rc.2",
				Platforms: map[string]string{"windows_amd64": hash('a'), "linux_amd64": hash('e'), "darwin_arm64": hash('d')},
				Data: []DataPack{
					{Name: "wpvulnerability", Version: "2026-10-24", SHA256: hash('c')},
					{Name: "checks", Version: "2026.10.20", SHA256: hash('b')},
				},
				Source: SourceRegistry,
			},
			{
				Name: "soundcheck", Repo: "RhythmShahriar/soundcheck", Version: "0.4.2",
				Platforms: map[string]string{"linux_amd64": hash('c')}, Source: SourceRegistry,
			},
		},
	}
}

func TestRoundTrip(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "rhythm.lock"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), FileName)
	f := sample()
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, golden) {
		t.Errorf("Write doesn't match testdata/rhythm.lock:\n%s", got)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, want 0644: the lock is committed with the project", fi.Mode().Perm())
	}

	back, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, f) {
		t.Errorf("Read gave back\n%+v\nwant\n%+v", back, f)
	}
	again := filepath.Join(t.TempDir(), FileName)
	if err := back.Write(again); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(again); !bytes.Equal(got, golden) {
		t.Error("writing what Read returned changed the file")
	}
	if e, ok := back.Tool("tuner"); !ok || e.Version != "0.3.1-rc.2" {
		t.Errorf("Tool(tuner) = %+v, %v", e, ok)
	}
	if _, ok := back.Tool("nope"); ok {
		t.Error("Tool(nope) found something")
	}
}

// TestWriteIsStable checks that the order things were added in never
// shows in the file.
func TestWriteIsStable(t *testing.T) {
	want, err := sample().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		f := sample()
		slices.Reverse(f.Tools)
		if i%2 == 0 {
			slices.Reverse(f.Tools[0].Data)
		}
		for j := range f.Tools {
			slices.Reverse(f.Tools[j].Data)
		}
		got, err := f.Marshal()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("order %d changed the output (%v):\n%s", i, err, got)
		}
	}
	empty, err := (&File{}).Marshal()
	if err != nil || string(empty) != "{\n  \"schema\": \"rhythm-lock/v1\",\n  \"tools\": []\n}\n" {
		t.Errorf("empty lock = %q, %v", empty, err)
	}
}

func TestWriteRefusesInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	f := sample()
	f.Tools[0].Platforms["linux_amd64"] = "nothex"
	if err := f.Write(path); err == nil {
		t.Fatal("Write took a bad hash")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a refused Write left a file behind")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 0 {
		t.Errorf("temporary files left: %v", entries)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(f *File)
		error string
	}{
		{"unknown schema", func(f *File) { f.Schema = "rhythm-lock/v9" }, `schema "rhythm-lock/v9" isn't "rhythm-lock/v1"`},
		{"bad hash", func(f *File) { f.Tools[1].Platforms["linux_amd64"] = "abc" }, `tools[1] (tuner): platforms["linux_amd64"]: sha256 "abc" isn't 64 lowercase hex digits`},
		{"uppercase hash", func(f *File) { f.Tools[1].Platforms["linux_amd64"] = strings.ToUpper(hash('e')) }, "isn't 64 lowercase hex"},
		{"bad data hash", func(f *File) { f.Tools[1].Data[0].SHA256 = "" }, `data "wpvulnerability": sha256 "" isn't 64`},
		{"v prefix", func(f *File) { f.Tools[0].Version = "v1.0.0" }, `version "v1.0.0" isn't an exact version`},
		{"partial version", func(f *File) { f.Tools[0].Version = "1.0" }, `version "1.0" isn't`},
		{"leading zero", func(f *File) { f.Tools[0].Version = "1.02.0" }, `version "1.02.0" isn't`},
		{"bad name", func(f *File) { f.Tools[0].Name = "../zap" }, `"../zap" isn't a tool name`},
		{"bad repo", func(f *File) { f.Tools[0].Repo = "zap" }, `repo "zap" isn't owner/name`},
		{"bad platform", func(f *File) { f.Tools[0].Platforms["linux"] = hash('a') }, `platform "linux" isn't os_arch`},
		{"no platforms", func(f *File) { f.Tools[0].Platforms = nil }, "no platforms"},
		{"bad source", func(f *File) { f.Tools[0].Source = "npm" }, `source "npm" isn't`},
		{"duplicate tool", func(f *File) { f.Tools = append(f.Tools, f.Tools[0]) }, "tools[3] (zap): listed twice"},
		{"duplicate pack", func(f *File) { f.Tools[1].Data = append(f.Tools[1].Data, f.Tools[1].Data[0]) }, `data "wpvulnerability" listed twice`},
		{"rhythm misnamed", func(f *File) { f.Rhythm.Name = "tuner" }, `rhythm: name "tuner" isn't "rhythm"`},
		{"tool named rhythm", func(f *File) { f.Tools[0].Name = "rhythm" }, `"rhythm" isn't a tool name`},
		{"dot repo", func(f *File) { f.Tools[0].Repo = "a/.." }, `repo "a/.." isn't owner/name`},
		// A "," or "=" would add pins to update --pin name=version,...
		{"pin in a pack version", func(f *File) { f.Tools[1].Data[0].Version = "1,vulns=0" },
			`data "wpvulnerability" version "1,vulns=0" may only use letters, digits and . _ + -`},
		{"space in a pack name", func(f *File) { f.Tools[1].Data[0].Name = "a b" }, `data "a b" version`},
	}
	ok := sample()
	ok.Schema = Schema
	if err := ok.Validate(); err != nil {
		t.Fatalf("the sample is invalid: %v", err)
	}
	for _, c := range cases {
		f := sample()
		f.Schema = Schema
		c.edit(f)
		err := f.Validate()
		if err == nil || !strings.Contains(err.Error(), c.error) {
			t.Errorf("%s: Validate() = %v, want %q", c.name, err, c.error)
		}
	}
	// Every problem is reported, not just the first.
	f := sample()
	f.Schema = Schema
	f.Tools[0].Version, f.Tools[2].Repo = "x", "y"
	if err := f.Validate(); err == nil || strings.Count(err.Error(), "\n") != 1 {
		t.Errorf("two problems gave %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	good, _ := os.ReadFile(filepath.Join("testdata", "rhythm.lock"))
	cases := map[string]struct {
		data  string
		error string
	}{
		"unknown schema": {strings.Replace(string(good), "rhythm-lock/v1", "rhythm-lock/v2", 1), `schema "rhythm-lock/v2" isn't "rhythm-lock/v1"; a newer rhythm may read it`},
		"no schema":      {`{"tools": []}`, `no schema; want "rhythm-lock/v1"`},
		"unknown field":  {strings.Replace(string(good), `"source": "extension"`, `"source": "extension", "pinned": true`, 1), `unknown field "pinned"`},
		"not json":       {"tuner 0.3.1\n", "invalid character"},
		"trailing data":  {string(good) + "{}", "after top-level value"},
		"bad hash":       {strings.Replace(string(good), hash('d'), "dd", 1), `sha256 "dd" isn't 64`},
	}
	for name, c := range cases {
		_, err := Parse([]byte(c.data))
		if err == nil || !strings.Contains(err.Error(), c.error) {
			t.Errorf("%s: Parse() = %v, want %q", name, err, c.error)
		}
	}
	path := filepath.Join(t.TempDir(), FileName)
	os.WriteFile(path, []byte(`{"schema":"x"}`), 0o600)
	if _, err := Read(path); err == nil || !strings.HasPrefix(err.Error(), path+": ") {
		t.Errorf("Read error doesn't name the file: %v", err)
	}
	if _, err := Read(filepath.Join(t.TempDir(), FileName)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file gave %v", err)
	}
}

func TestFind(t *testing.T) {
	touch := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	nested := filepath.Join(repo, "a", "b", "c")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(base, FileName)) // above the repository: never used

	if _, err := Find(nested); !errors.Is(err, ErrNotFound) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a lock above the repository was found: %v", err)
	}
	touch(t, filepath.Join(repo, FileName))
	if got, err := Find(nested); err != nil || got != filepath.Join(repo, FileName) {
		t.Errorf("Find(nested) = %q, %v; want the repository's lock", got, err)
	}
	touch(t, filepath.Join(repo, "a", FileName))
	if got, err := Find(nested); err != nil || got != filepath.Join(repo, "a", FileName) {
		t.Errorf("Find(nested) = %q, %v; want the nearest lock", got, err)
	}
	if got, err := Find(repo); err != nil || got != filepath.Join(repo, FileName) {
		t.Errorf("Find(repo) = %q, %v", got, err)
	}
	// A worktree or submodule has a .git file, not a folder.
	sub := filepath.Join(repo, "a", "sub")
	touch(t, filepath.Join(sub, ".git"))
	if _, err := Find(filepath.Join(sub)); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find crossed a submodule's root: %v", err)
	}
	// Outside any repository only the folder itself counts.
	plain := filepath.Join(base, "plain", "deep")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := Find(plain); !errors.Is(err, ErrNotFound) {
		t.Errorf("outside a repository Find walked up to %q", got)
	}
	touch(t, filepath.Join(plain, FileName))
	if got, err := Find(plain); err != nil || got != filepath.Join(plain, FileName) {
		t.Errorf("Find(plain) = %q, %v", got, err)
	}
}

func TestDiff(t *testing.T) {
	edit := func(change func(f *File)) *File {
		f := sample()
		change(f)
		return f
	}
	cases := []struct {
		name     string
		old, new *File
		want     []string
	}{
		{"same", sample(), sample(), nil},
		{"both empty", nil, nil, nil},
		{"from nothing", nil, edit(func(f *File) { f.Tools = f.Tools[2:] }), []string{"+ rhythm 0.7.0", "+ soundcheck 0.4.2"}},
		{"added", edit(func(f *File) { f.Tools = f.Tools[:2] }), sample(), []string{"+ soundcheck 0.4.2"}},
		{"removed", sample(), edit(func(f *File) { f.Tools = f.Tools[1:] }), []string{"- zap 1.0.0"}},
		{"bumped", sample(), edit(func(f *File) {
			f.Tools[1].Version = "0.3.2"
			f.Tools[1].Signer = "someone else" // a new version is signed anew; not reported
			f.Rhythm.Version = "0.8.0"
		}), []string{"~ rhythm 0.7.0 → 0.8.0", "~ tuner 0.3.1-rc.2 → 0.3.2"}},
		{"data changed", sample(), edit(func(f *File) {
			f.Tools[1].Data = []DataPack{
				{Name: "wpvulnerability", Version: "2026-10-25", SHA256: hash('f')},
				{Name: "abuse", Version: "1", SHA256: hash('a')},
			}
			f.Tools[0].Data = []DataPack{{Name: "rules", Version: "3", SHA256: hash('a')}}
		}), []string{
			"~ tuner data abuse (none) → 1",
			"~ tuner data checks 2026.10.20 → (none)",
			"~ tuner data wpvulnerability 2026-10-24 → 2026-10-25",
			"~ zap data rules (none) → 3",
		}},
		{"data rehashed", sample(), edit(func(f *File) { f.Tools[1].Data[0].SHA256 = hash('9') }),
			[]string{"! tuner data wpvulnerability 2026-10-24: content changed for the same version"}},
		{"bumped and data", sample(), edit(func(f *File) {
			f.Tools[1].Version = "0.3.2"
			f.Tools[1].Data = f.Tools[1].Data[:1]
		}), []string{"~ tuner 0.3.1-rc.2 → 0.3.2", "~ tuner data checks 2026.10.20 → (none)"}},
		{"rehashed", sample(), edit(func(f *File) { f.Tools[2].Platforms["linux_amd64"] = hash('9') }),
			[]string{"! soundcheck 0.4.2: archives changed for the same version"}},
		{"rehashed beside a new platform", sample(), edit(func(f *File) {
			f.Tools[2].Platforms["linux_amd64"] = hash('9')
			f.Tools[2].Platforms["linux_arm64"] = hash('8')
		}), []string{"! soundcheck 0.4.2: archives changed for the same version"}},
		// Locking another machine's platform, or dropping one, leaves the
		// archives already locked as they were.
		{"platform added", sample(), edit(func(f *File) { f.Tools[2].Platforms["linux_arm64"] = hash('9') }), nil},
		{"platform removed", sample(), edit(func(f *File) { delete(f.Tools[2].Platforms, "darwin_arm64") }), nil},
		{"resigned", sample(), edit(func(f *File) { f.Tools[2].Signer = "release-bot" }),
			[]string{"! soundcheck: signer (none) → release-bot for the same version"}},
		{"rhythm dropped", sample(), edit(func(f *File) { f.Rhythm = nil }), []string{"- rhythm 0.7.0"}},
	}
	for _, c := range cases {
		var got []string
		for _, ch := range Diff(c.old, c.new) {
			got = append(got, ch.String())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: Diff =\n%s\nwant\n%s", c.name, strings.Join(got, "\n"), strings.Join(c.want, "\n"))
		}
	}
	// The fields, not just the text, carry the change.
	ch := Diff(sample(), edit(func(f *File) { f.Tools[1].Version = "0.3.2" }))
	if len(ch) != 1 || ch[0] != (Change{Kind: Bumped, Name: "tuner", From: "0.3.1-rc.2", To: "0.3.2"}) {
		t.Errorf("Diff = %+v", ch)
	}
}

func TestEntryEntriesClone(t *testing.T) {
	f := sample()
	if e := f.Entry("rhythm"); e == nil || e != f.Rhythm {
		t.Errorf("Entry(rhythm) = %v", e)
	}
	if e := f.Entry("tuner"); e == nil || e.Name != "tuner" {
		t.Errorf("Entry(tuner) = %v", e)
	}
	if e := f.Entry("nope"); e != nil {
		t.Errorf("Entry(nope) = %v", e)
	}
	names := []string{}
	for _, e := range f.Entries() {
		names = append(names, e.Name)
	}
	if len(names) != len(f.Tools)+1 || names[0] != "rhythm" {
		t.Errorf("Entries() = %v", names)
	}
	c := f.Clone()
	c.Rhythm.Version = "9.9.9"
	c.Entry("tuner").Platforms["linux_amd64"] = "changed"
	c.Entry("tuner").Data[0].Version = "changed"
	if f.Rhythm.Version == "9.9.9" || f.Entry("tuner").Platforms["linux_amd64"] == "changed" || f.Entry("tuner").Data[0].Version == "changed" {
		t.Error("changing the clone changed the original")
	}
	if (&File{}).Clone().Tools != nil {
		t.Error("a clone of no tools has tools")
	}
}
