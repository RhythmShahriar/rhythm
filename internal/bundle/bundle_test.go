// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// The bundle reader serves kit/update.
var (
	_ update.Source = (*Source)(nil)
	_ update.Lister = (*Source)(nil)
)

// sample writes a bundle with two releases of one repository and one of
// another. It returns its path.
func sample(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rhythm-bundle.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := NewWriter(f)
	m := Manifest{Rhythm: "0.7.0", Platforms: []string{"linux_amd64"}, Lock: json.RawMessage(`{"schema":"rhythm-lock/v1","tools":[]}`)}
	for _, r := range []struct{ repo, version string }{
		{"RhythmShahriar/tuner", "0.3.1"}, {"RhythmShahriar/tuner", "0.3.2"}, {"RhythmShahriar/echo", "0.1.0"},
	} {
		rel := Release{Name: r.repo[strings.Index(r.repo, "/")+1:], Repo: r.repo, Version: r.version, Assets: map[string]File{}}
		for _, asset := range []string{"checksums.txt", "rhythm-x_linux_amd64.tar.gz"} {
			got, err := w.Add(ReleaseDir(r.repo, r.version)+"/"+asset, []byte(r.repo+" "+r.version+" "+asset))
			if err != nil {
				t.Fatal(err)
			}
			rel.Assets[asset] = got
		}
		m.Releases = append(m.Releases, rel)
	}
	if err := w.Close(m); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRoundTrip(t *testing.T) {
	b, err := Open(sample(t))
	if err != nil {
		t.Fatal(err)
	}
	m := b.Manifest()
	if m.Schema != Schema || m.Rhythm != "0.7.0" || len(m.Releases) != 3 || !b.HasPlatform("linux_amd64") || b.HasPlatform("darwin_arm64") {
		t.Fatalf("manifest %+v", m)
	}
	src, err := b.Source("rhythmshahriar/TUNER")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	latest, err := src.Latest(ctx)
	if err != nil || latest.Version != "0.3.2" || latest.Tag != "v0.3.2" {
		t.Fatalf("latest %+v, %v", latest, err)
	}
	rel, err := src.ByTag(ctx, "v0.3.1")
	if err != nil {
		t.Fatal(err)
	}
	rc, size, err := src.Open(ctx, rel, "checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || string(data) != "RhythmShahriar/tuner 0.3.1 checksums.txt" || size != int64(len(data)) {
		t.Errorf("read %q (%d), %v", data, size, err)
	}
	list, err := src.Releases(ctx)
	if err != nil || len(list) != 2 {
		t.Errorf("releases %v, %v", list, err)
	}
	var nr *update.ErrNoRelease
	if _, err := src.ByTag(ctx, "9.9.9"); !errors.As(err, &nr) {
		t.Errorf("ByTag 9.9.9: %v", err)
	}
	if _, _, err := src.Open(ctx, rel, "nope"); err == nil {
		t.Error("opened an asset the release lacks")
	}
	if _, err := b.Source("RhythmShahriar/soundcheck"); err == nil || !strings.Contains(err.Error(), "has no RhythmShahriar/soundcheck") {
		t.Errorf("Source of a missing repo: %v", err)
	}
}

// entry is one raw tar entry for a crafted bundle.
type entry struct {
	name string
	typ  byte
	body string
}

// craft writes entries as a .tar.gz, with a manifest naming want.
func craft(t *testing.T, entries []entry, files []File) string {
	t.Helper()
	m := Manifest{Schema: Schema, Platforms: []string{"linux_amd64"}, Releases: []Release{{Name: "tuner", Repo: "RhythmShahriar/tuner", Version: "0.3.2", Assets: map[string]File{}}}}
	for i, f := range files {
		m.Releases[0].Assets[string(rune('a'+i))] = f
	}
	man, _ := json.Marshal(m)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, e := range append(entries, entry{name: ManifestName, body: string(man)}) {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: e.typ}
		if e.typ == 0 {
			h.Typeflag = tar.TypeReg
		}
		if h.Typeflag == tar.TypeSymlink {
			h.Linkname, h.Size = "/etc/passwd", 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			io.WriteString(tw, e.body)
		}
	}
	tw.Close()
	zw.Close()
	p := filepath.Join(t.TempDir(), "crafted.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileOf(name, body string) File {
	sum := sha256.Sum256([]byte(body))
	return File{Path: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}
}

func TestOpenRefusesCraftedBundles(t *testing.T) {
	good := fileOf("releases/RhythmShahriar/tuner/v0.3.2/checksums.txt", "sums")
	cases := map[string]struct {
		entries []entry
		files   []File
		want    string
	}{
		"parent dir":       {[]entry{{name: "../evil", body: "x"}}, nil, "unsafe entry name"},
		"inner parent":     {[]entry{{name: "releases/../../evil", body: "x"}}, nil, "unsafe entry name"},
		"absolute":         {[]entry{{name: "/etc/cron.d/evil", body: "x"}}, nil, "unsafe entry name"},
		"backslash":        {[]entry{{name: `releases\..\evil`, body: "x"}}, nil, "unsafe entry name"},
		"dot":              {[]entry{{name: "./manifest.json", body: "x"}}, nil, "unsafe entry name"},
		"double slash":     {[]entry{{name: "releases//x", body: "x"}}, nil, "unsafe entry name"},
		"symlink":          {[]entry{{name: "releases/link", typ: tar.TypeSymlink}}, nil, "isn't a plain file"},
		"directory":        {[]entry{{name: "releases", typ: tar.TypeDir}}, nil, "isn't a plain file"},
		"duplicate":        {[]entry{{name: good.Path, body: "sums"}, {name: good.Path, body: "sums"}}, []File{good}, "appears twice"},
		"not in manifest":  {[]entry{{name: good.Path, body: "sums"}, {name: "extra", body: "x"}}, []File{good}, "which the manifest doesn't name"},
		"missing file":     {nil, []File{good}, "which the bundle doesn't hold"},
		"changed contents": {[]entry{{name: good.Path, body: "other"}}, []File{good}, "doesn't match its sha256 in the manifest"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Open(craft(t, c.entries, c.files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
	// The control: the same shape with nothing wrong opens.
	if _, err := Open(craft(t, []entry{{name: good.Path, body: "sums"}}, []File{good})); err != nil {
		t.Errorf("control: %v", err)
	}
}

func TestOpenRefusesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	notGzip := filepath.Join(dir, "a.tar.gz")
	os.WriteFile(notGzip, []byte("plain text"), 0o600)
	if _, err := Open(notGzip); err == nil || !strings.Contains(err.Error(), "isn't a rhythm bundle") {
		t.Errorf("plain file: %v", err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	tw.Close()
	zw.Close()
	empty := filepath.Join(dir, "empty.tar.gz")
	os.WriteFile(empty, buf.Bytes(), 0o600)
	if _, err := Open(empty); err == nil || !strings.Contains(err.Error(), "has no manifest.json") {
		t.Errorf("no manifest: %v", err)
	}
	if _, err := Open(filepath.Join(dir, "missing.tar.gz")); err == nil {
		t.Error("opened a missing file")
	}
}

func TestWriterRefuses(t *testing.T) {
	w := NewWriter(io.Discard)
	for _, name := range []string{"../x", "/x", ManifestName, "a\\b"} {
		if _, err := w.Add(name, nil); err == nil {
			t.Errorf("Add(%q) worked", name)
		}
	}
	if _, err := w.Add("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("a", []byte("1")); err == nil {
		t.Error("added a twice")
	}
	// A file the manifest doesn't name fails Close.
	if err := w.Close(Manifest{}); err == nil || !strings.Contains(err.Error(), "doesn't name") {
		t.Errorf("Close: %v", err)
	}
}

// A bundle changed after Open fails the read of the changed entry.
func TestOpenNoticesALaterChange(t *testing.T) {
	p := sample(t)
	b, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := b.Source("RhythmShahriar/echo")
	rel, _ := src.Latest(context.Background())

	f, _ := os.Create(p)
	w := NewWriter(f)
	files := map[string]File{}
	for _, asset := range []string{"checksums.txt", "rhythm-x_linux_amd64.tar.gz"} {
		got, _ := w.Add(ReleaseDir("RhythmShahriar/echo", "0.1.0")+"/"+asset, []byte("evil "+asset))
		files[asset] = got
	}
	w.Close(Manifest{Releases: []Release{{Name: "echo", Repo: "RhythmShahriar/echo", Version: "0.1.0", Assets: files}}})
	f.Close()

	rc, _, err := src.Open(context.Background(), rel, "checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if _, err := io.ReadAll(rc); err == nil || !strings.Contains(err.Error(), "changed since the bundle was opened") {
		t.Errorf("read: %v", err)
	}
}
