// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package bundle writes and reads rhythm's offline bundles: one .tar.gz
// that carries everything a locked install needs, for networks where
// GitHub is out of reach.
//
// A bundle holds:
//
//	manifest.json                              Manifest, schema rhythm-bundle/v1
//	releases/<owner>/<name>/v<version>/<asset> each release's archives for the
//	                                           bundled platforms, checksums.txt
//	                                           and checksums.txt.sigstore.json
//	data/<tool>/<pack>/<file>                  a tool's data packs, when the tool
//	                                           can export them
//
// The manifest records the rhythm.lock the bundle was built from and the
// sha256 and size of every file. Open refuses a bundle whose entries
// aren't exactly the files the manifest lists with matching hashes. It
// also refuses any entry that isn't a plain file under a clean relative
// name, so a crafted tar can't name a path outside the bundle or hide a
// file.
// Nothing is ever extracted to disk: assets are streamed from the tar.
//
// The manifest itself is not signed. Its hashes catch a damaged or
// edited bundle; trust still comes from the release signatures and the
// lock, as it does online.
//
// Source serves one repository's releases from a bundle and has the
// method set of kit/update's Source and Lister. The checks a bundle's
// reader still makes are the usual ones: the signature over
// checksums.txt and the archive against checksums.txt (and the lock).
package bundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Schema is the version of the manifest format.
const Schema = "rhythm-bundle/v1"

// ManifestName is the manifest's entry in the tar.
const ManifestName = "manifest.json"

// Limits on what Open reads.
const (
	maxManifest = 4 << 20   // manifest.json
	maxEntry    = 512 << 20 // any other entry
	maxEntries  = 10000
)

// Manifest describes a bundle's contents.
type Manifest struct {
	Schema    string          `json:"schema"`    // always Schema; Close sets it
	Created   time.Time       `json:"created"`   // when the bundle was written, UTC
	Rhythm    string          `json:"rhythm"`    // version of the rhythm that wrote it
	Platforms []string        `json:"platforms"` // os_arch, such as linux_amd64
	Lock      json.RawMessage `json:"lock"`      // the rhythm.lock it was built from
	Releases  []Release       `json:"releases"`  // rhythm and every locked tool
	Data      []DataPack      `json:"data"`      // tool data packs; empty when there are none
}

// Release is one release in a bundle.
type Release struct {
	Name    string          `json:"name"`    // tool name, or "rhythm"
	Repo    string          `json:"repo"`    // GitHub owner/name
	Version string          `json:"version"` // without the leading v
	Assets  map[string]File `json:"assets"`  // asset name → file in the bundle
}

// DataPack is one data pack of a tool in a bundle.
type DataPack struct {
	Tool    string `json:"tool"`    // the tool that uses it, "tuner"
	Name    string `json:"name"`    // the pack's name, as in update --json
	Version string `json:"version"` // the pack's own version or date
	File    File   `json:"file"`    // the pack's entry, under data/<tool>/<pack>/
}

// File is one entry of the tar.
type File struct {
	Path   string `json:"path"`   // the entry name, a clean relative path (see checkName)
	SHA256 string `json:"sha256"` // hex digest of the entry's bytes
	Size   int64  `json:"size"`   // in bytes
}

// ReleaseDir is where a release's assets go in a bundle.
func ReleaseDir(repo, version string) string {
	return "releases/" + repo + "/v" + strings.TrimPrefix(version, "v")
}

// safeName is what an entry's name may look like: plain relative path
// elements of letters, digits and . _ - + @.
var safeName = regexp.MustCompile(`^[A-Za-z0-9._+@-]+(/[A-Za-z0-9._+@-]+)*$`)

// checkName refuses an entry name that could reach outside the bundle or
// mean something else on another OS: absolute paths, "..", ".",
// backslashes, empty elements and anything path.Clean would change.
func checkName(name string) error {
	if !safeName.MatchString(name) || path.Clean(name) != name || path.IsAbs(name) {
		return fmt.Errorf("unsafe entry name %q", name)
	}
	for _, el := range strings.Split(name, "/") {
		if el == "." || el == ".." {
			return fmt.Errorf("unsafe entry name %q", name)
		}
	}
	return nil
}

// Writer builds a bundle. Add every file, then Close with the manifest.
type Writer struct {
	zw      *gzip.Writer
	tw      *tar.Writer
	files   map[string]File
	created time.Time
}

// NewWriter starts a bundle on w.
func NewWriter(w io.Writer) *Writer {
	zw := gzip.NewWriter(w)
	return &Writer{zw: zw, tw: tar.NewWriter(zw), files: map[string]File{}, created: time.Now().UTC()}
}

// Add writes data as the entry name and returns its File.
func (w *Writer) Add(name string, data []byte) (File, error) {
	if err := checkName(name); err != nil {
		return File{}, err
	}
	if name == ManifestName {
		return File{}, errors.New("manifest.json is written by Close")
	}
	if _, dup := w.files[name]; dup {
		return File{}, fmt.Errorf("%s added twice", name)
	}
	if err := w.tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg, ModTime: w.created}); err != nil {
		return File{}, err
	}
	if _, err := w.tw.Write(data); err != nil {
		return File{}, err
	}
	sum := sha256.Sum256(data)
	f := File{Path: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}
	w.files[name] = f
	return f, nil
}

// Close writes m as manifest.json and finishes the bundle. Every file m
// names must have been added. Every file added must be named.
func (w *Writer) Close(m Manifest) error {
	m.Schema = Schema
	if m.Created.IsZero() {
		m.Created = w.created
	}
	if m.Releases == nil {
		m.Releases = []Release{}
	}
	if m.Data == nil {
		m.Data = []DataPack{}
	}
	if err := matchFiles(&m, w.files); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := w.tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg, ModTime: w.created}); err != nil {
		return err
	}
	if _, err := w.tw.Write(data); err != nil {
		return err
	}
	if err := w.tw.Close(); err != nil {
		return err
	}
	return w.zw.Close()
}

// matchFiles checks that the manifest names exactly the entries in files,
// with the same hashes and sizes.
func matchFiles(m *Manifest, files map[string]File) error {
	named := map[string]bool{}
	check := func(f File) error {
		got, ok := files[f.Path]
		switch {
		case !ok:
			return fmt.Errorf("the manifest names %s, which the bundle doesn't hold", f.Path)
		case got.SHA256 != f.SHA256 || got.Size != f.Size:
			return fmt.Errorf("%s doesn't match its sha256 in the manifest; the bundle was changed after it was made", f.Path)
		case named[f.Path]:
			return fmt.Errorf("the manifest names %s twice", f.Path)
		}
		named[f.Path] = true
		return nil
	}
	for _, r := range m.Releases {
		for _, f := range r.Assets {
			if err := check(f); err != nil {
				return err
			}
		}
	}
	for _, d := range m.Data {
		if err := check(d.File); err != nil {
			return err
		}
	}
	for p := range files {
		if !named[p] {
			return fmt.Errorf("the bundle holds %s, which the manifest doesn't name", p)
		}
	}
	return nil
}

// Bundle is an opened bundle file.
type Bundle struct {
	path     string
	manifest Manifest
	files    map[string]File
}

// Open reads the bundle at p and checks it whole: every entry's name and
// type, its hash against the manifest and that nothing is missing or
// extra. It keeps no file open.
func Open(p string) (*Bundle, error) {
	f, err := os.Open(p) //nolint:gosec // the bundle the user named
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s isn't a rhythm bundle (a .tar.gz): %w", p, err)
	}
	tr := tar.NewReader(zr)
	b := &Bundle{path: p, files: map[string]File{}}
	var manifest []byte
	for n := 0; ; n++ {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if n >= maxEntries {
			return nil, fmt.Errorf("%s holds more than %d entries", p, maxEntries)
		}
		if err := checkName(h.Name); err != nil {
			return nil, fmt.Errorf("%s: %w; refusing the bundle", p, err)
		}
		if h.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("%s: %s isn't a plain file; refusing the bundle", p, h.Name)
		}
		if _, dup := b.files[h.Name]; dup || (h.Name == ManifestName && manifest != nil) {
			return nil, fmt.Errorf("%s: %s appears twice; refusing the bundle", p, h.Name)
		}
		if h.Name == ManifestName {
			if manifest, err = readAtMost(tr, maxManifest); err != nil {
				return nil, fmt.Errorf("%s: manifest.json: %w", p, err)
			}
			continue
		}
		hash := sha256.New()
		size, err := io.Copy(hash, io.LimitReader(tr, maxEntry+1))
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", p, h.Name, err)
		}
		if size > maxEntry {
			return nil, fmt.Errorf("%s: %s is larger than %d MB", p, h.Name, maxEntry>>20)
		}
		b.files[h.Name] = File{Path: h.Name, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: size}
	}
	if manifest == nil {
		return nil, fmt.Errorf("%s has no manifest.json; it isn't a rhythm bundle", p)
	}
	if err := json.Unmarshal(manifest, &b.manifest); err != nil {
		return nil, fmt.Errorf("%s: manifest.json: %w", p, err)
	}
	if b.manifest.Schema != Schema {
		return nil, fmt.Errorf("%s: manifest schema %q isn't %q; it needs another rhythm", p, b.manifest.Schema, Schema)
	}
	if err := matchFiles(&b.manifest, b.files); err != nil {
		return nil, fmt.Errorf("%s: %w; refusing the bundle", p, err)
	}
	return b, nil
}

func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("larger than %d MB", limit>>20)
	}
	return data, nil
}

// Path is the bundle's file.
func (b *Bundle) Path() string { return b.path }

// Manifest returns the bundle's manifest.
func (b *Bundle) Manifest() Manifest { return b.manifest }

// HasPlatform reports whether the bundle carries archives for os_arch.
func (b *Bundle) HasPlatform(platform string) bool {
	return slices.Contains(b.manifest.Platforms, platform)
}

// Source returns the releases of repo in the bundle, compared without
// regard to case as GitHub does.
func (b *Bundle) Source(repo string) (*Source, error) {
	var rels []Release
	for _, r := range b.manifest.Releases {
		if strings.EqualFold(r.Repo, repo) {
			rels = append(rels, r)
		}
	}
	if len(rels) == 0 {
		return nil, fmt.Errorf("%s has no %s", b.path, repo)
	}
	return &Source{b: b, repo: repo, rels: rels}, nil
}

// Source serves one repository's releases from a bundle. It satisfies
// kit/update's Source and Lister.
type Source struct {
	b    *Bundle
	repo string
	rels []Release
}

func (s *Source) release(r Release) *update.Release {
	rel := &update.Release{Version: r.Version, Tag: "v" + r.Version, Assets: map[string]string{}}
	for name, f := range r.Assets {
		rel.Assets[name] = f.Path
	}
	return rel
}

// Latest returns the newest release of the repository in the bundle.
func (s *Source) Latest(context.Context) (*update.Release, error) {
	best := s.rels[0]
	for _, r := range s.rels[1:] {
		if update.Newer(best.Version, r.Version) {
			best = r
		}
	}
	return s.release(best), nil
}

// ByTag returns the release with this version, given with or without
// the leading v.
func (s *Source) ByTag(_ context.Context, version string) (*update.Release, error) {
	version = strings.TrimPrefix(version, "v")
	for _, r := range s.rels {
		if r.Version == version {
			return s.release(r), nil
		}
	}
	return nil, &update.ErrNoRelease{Repo: s.repo, API: s.b.path, Version: "v" + version}
}

// Releases lists the repository's releases in the bundle.
func (s *Source) Releases(context.Context) ([]*update.Release, error) {
	var out []*update.Release
	for _, r := range s.rels {
		out = append(out, s.release(r))
	}
	return out, nil
}

// Open streams one of rel's assets from the bundle. The entry is checked
// against the manifest's sha256 as it is read: a bundle changed since
// Open fails at the end of the read.
func (s *Source) Open(ctx context.Context, rel *update.Release, asset string) (io.ReadCloser, int64, error) {
	p, ok := rel.Assets[asset]
	if !ok {
		return nil, 0, fmt.Errorf("%s has no %s for %s %s", s.b.path, asset, s.repo, rel.Version)
	}
	want, ok := s.b.files[p]
	if !ok {
		return nil, 0, fmt.Errorf("%s has no %s", s.b.path, p)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	f, err := os.Open(s.b.path)
	if err != nil {
		return nil, 0, err
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	// Only offsets into the gzip stream would allow a seek, so the tar
	// is read from the start each time. Bundles hold a few dozen entries.
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			_ = f.Close()
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("%s no longer holds %s", s.b.path, p)
			}
			return nil, 0, err
		}
		if h.Name == p && h.Typeflag == tar.TypeReg {
			return &entryReader{r: io.LimitReader(tr, want.Size+1), c: f, h: sha256.New(), want: want, bundle: s.b.path}, want.Size, nil
		}
	}
}

// entryReader reads one entry and fails at its end unless it hashes to
// what the manifest says.
type entryReader struct {
	r      io.Reader
	c      io.Closer
	h      hash.Hash
	n      int64
	want   File
	bundle string
}

func (e *entryReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	_, _ = e.h.Write(p[:n])
	e.n += int64(n)
	if errors.Is(err, io.EOF) && (e.n != e.want.Size || hex.EncodeToString(e.h.Sum(nil)) != e.want.SHA256) {
		return n, fmt.Errorf("%s: %s changed since the bundle was opened", e.bundle, e.want.Path)
	}
	return n, err
}

func (e *entryReader) Close() error { return e.c.Close() }
