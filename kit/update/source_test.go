// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cachedInstall installs v0.3.1 from api into a new dir with CacheDir set.
func cachedInstall(t *testing.T, api *releaseAPI, cacheDir string, opts InstallOptions) (InstallResult, string, error) {
	t.Helper()
	u := api.updater()
	u.CacheDir = cacheDir
	rel, err := u.ByTag(context.Background(), "0.3.1")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	res, err := u.InstallWith(context.Background(), rel, dest, opts)
	return res, dest, err
}

func TestCacheSkipsTheDownload(t *testing.T) {
	api := newReleaseAPI(t, fakeRel{tag: "v0.3.1"})
	cacheDir := filepath.Join(t.TempDir(), "archives") // created on first use
	first, _, err := cachedInstall(t, api, cacheDir, InstallOptions{})
	if err != nil || first.Cached {
		t.Fatalf("first install = %+v, %v", first, err)
	}
	if names := dirNames(t, cacheDir); len(names) != 1 || names[0] != first.SHA256+hostExt() {
		t.Fatalf("cache = %v, want %s%s", names, first.SHA256, hostExt())
	}
	if api.archiveHits() != 1 {
		t.Fatalf("%d archive downloads after the first install", api.archiveHits())
	}

	second, dest, err := cachedInstall(t, api, cacheDir, InstallOptions{ExpectSHA256: first.SHA256})
	if err != nil || !second.Cached || second.SHA256 != first.SHA256 {
		t.Fatalf("second install = %+v, %v", second, err)
	}
	if n := api.archiveHits(); n != 1 {
		t.Errorf("second install downloaded the archive (%d requests in all)", n)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.1" {
		t.Errorf("dest = %q", got)
	}
	// The cached archive is kept for the next run.
	if names := dirNames(t, cacheDir); len(names) != 1 {
		t.Errorf("cache after a hit = %v", names)
	}

	// A lock that disagrees still wins over a cache hit.
	_, _, err = cachedInstall(t, api, cacheDir, InstallOptions{ExpectSHA256: strings.Repeat("0", 64)})
	var ce *ErrChecksum
	if !errors.As(err, &ce) || !ce.Locked {
		t.Errorf("locked mismatch on a hit: err = %v", err)
	}
}

func TestCorruptCacheIsDownloadedAgain(t *testing.T) {
	api := newReleaseAPI(t, fakeRel{tag: "v0.3.1"})
	cacheDir := t.TempDir()
	first, _, err := cachedInstall(t, api, cacheDir, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cacheDir, first.SHA256+hostExt())
	if err := os.WriteFile(path, []byte("bit rot"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, dest, err := cachedInstall(t, api, cacheDir, InstallOptions{})
	if err != nil || res.Cached {
		t.Fatalf("install over a corrupt cache = %+v, %v", res, err)
	}
	if api.archiveHits() != 2 {
		t.Errorf("%d archive downloads, want 2", api.archiveHits())
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.1" {
		t.Errorf("dest = %q", got)
	}
	if sum, _ := hashFile(path); sum != first.SHA256 {
		t.Errorf("cache not repaired: sha256 %s", sum)
	}
}

func TestCacheStoresOnlyVerifiedArchives(t *testing.T) {
	t.Run("checksum mismatch", func(t *testing.T) {
		srv := fakeGitHub(t, []byte("EVIL"), true)
		u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL, CacheDir: t.TempDir()}
		rel, _ := u.Latest(context.Background())
		var ce *ErrChecksum
		if err := u.Install(context.Background(), rel, installed(t)); !errors.As(err, &ce) {
			t.Fatalf("err = %v", err)
		}
		if names := dirNames(t, u.CacheDir); len(names) != 0 {
			t.Errorf("cache = %v, want empty", names)
		}
	})
	t.Run("bad signature", func(t *testing.T) {
		rel, _, hits := signedRelease(t, false)
		u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", CacheDir: t.TempDir(),
			Verifier: &fakeVerifier{err: errors.New("wrong signer")}}
		var se *ErrSignature
		if err := u.Install(context.Background(), rel, installed(t)); !errors.As(err, &se) {
			t.Fatalf("err = %v", err)
		}
		if names := dirNames(t, u.CacheDir); len(names) != 0 || hits("/archive") != 0 {
			t.Errorf("cache = %v, archive downloads = %d; want neither", names, hits("/archive"))
		}
	})
}

// memSource is an in-memory Source: releases by version, assets by name.
type memSource struct {
	rels  []*Release
	files map[string][]byte // "<version>/<asset>" → content
}

// newMemSource builds releases of rhythm-tuner for this platform, each
// with a binary "rhythm-tuner <version>" and a checksums.txt.
func newMemSource(t *testing.T, versions ...string) *memSource {
	t.Helper()
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	m := &memSource{files: map[string][]byte{}}
	for i, v := range versions {
		archive := hostArchive(t, "rhythm-tuner", []byte("rhythm-tuner "+v))
		sum := sha256.Sum256(archive)
		m.files[v+"/"+name] = archive
		m.files[v+"/checksums.txt"] = fmt.Appendf(nil, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		m.files[v+"/"+bundleAsset] = []byte(testBundle)
		m.rels = append(m.rels, &Release{
			Version:   v,
			Tag:       "v" + v,
			Published: time.Now().Add(-time.Duration(len(versions)-i) * day),
			// The values are the source's own locators, not URLs.
			Assets: map[string]string{name: "mem:" + name, "checksums.txt": "mem:sums", bundleAsset: "mem:bundle"},
		})
	}
	return m
}

func (m *memSource) Latest(context.Context) (*Release, error) {
	return m.rels[len(m.rels)-1], nil
}

func (m *memSource) ByTag(_ context.Context, version string) (*Release, error) {
	for _, r := range m.rels {
		if r.Version == version {
			return r, nil
		}
	}
	return nil, &ErrNoRelease{Repo: "mem", Version: "v" + version}
}

func (m *memSource) Open(_ context.Context, rel *Release, asset string) (io.ReadCloser, int64, error) {
	b, ok := m.files[rel.Version+"/"+asset]
	if !ok {
		return nil, 0, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

// listing adds Lister to a memSource.
type listing struct{ *memSource }

func (l listing) Releases(context.Context) ([]*Release, error) { return l.rels, nil }

// noNetwork fails the test on any HTTP request.
type noNetwork struct{ t *testing.T }

func (n noNetwork) RoundTrip(r *http.Request) (*http.Response, error) {
	n.t.Errorf("unexpected request to %s", r.URL)
	return nil, errors.New("no network in this test")
}

func TestInstallFromACustomSource(t *testing.T) {
	isolate(t)
	t.Setenv("RHYTHM_OFFLINE", "1") // stops GitHub only, not a bundle
	src := newMemSource(t, "0.3.0", "0.3.1")
	v := &fakeVerifier{signer: workflow}
	u := &Updater{
		Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.3.0",
		HTTP: &http.Client{Transport: noNetwork{t}}, Source: src, Verifier: v,
	}
	rel, err := u.Latest(context.Background())
	if err != nil || rel.Version != "0.3.1" {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}
	dest := installed(t)
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{})
	if err != nil || res.Version != "0.3.1" || res.Signer != workflow {
		t.Fatalf("InstallWith = %+v, %v", res, err)
	}
	if got := content(t, dest); got != "rhythm-tuner 0.3.1" {
		t.Errorf("dest = %q", got)
	}
	if v.calls != 1 || !bytes.Equal(v.checksums, src.files["0.3.1/checksums.txt"]) {
		t.Errorf("verifier calls = %d", v.calls)
	}
	rel, err = u.ByTag(context.Background(), "v0.3.0")
	if err != nil || rel.Version != "0.3.0" {
		t.Errorf("ByTag = %+v, %v", rel, err)
	}
	// Every check applies to a custom source too.
	src.files["0.3.0/"+u.ArchiveName()] = hostArchive(t, "rhythm-tuner", []byte("EVIL"))
	var ce *ErrChecksum
	if err := u.Install(context.Background(), rel, dest); !errors.As(err, &ce) {
		t.Errorf("tampered bundle: err = %v, want ErrChecksum", err)
	}
}

func TestCustomSourceListing(t *testing.T) {
	src := newMemSource(t, "0.3.0", "0.3.1", "0.4.0")
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Source: src}
	for _, spec := range []string{"0.3", "latest"} {
		u.MinAge = 0
		if spec == "latest" {
			u.MinAge = time.Hour
		}
		if _, err := u.Resolve(context.Background(), spec); err == nil || !strings.Contains(err.Error(), "can't list releases") {
			t.Errorf("no Lister, Resolve(%q) err = %v", spec, err)
		}
	}
	u.MinAge = 0
	if rel, err := u.Resolve(context.Background(), "0.3.0"); err != nil || rel.Version != "0.3.0" {
		t.Errorf("exact spec without a Lister = %+v, %v", rel, err)
	}

	u.Source = listing{src}
	if rel, err := u.Resolve(context.Background(), "0.3"); err != nil || rel.Version != "0.3.1" {
		t.Errorf("Resolve(0.3) = %+v, %v", rel, err)
	}
	// 0.4.0 is a day old and 0.3.1 two days.
	u.MinAge = 36 * time.Hour
	rel, err := u.Latest(context.Background())
	if err != nil || rel.Version != "0.3.1" || rel.SkippedNewer == nil || rel.SkippedNewer.Version != "0.4.0" {
		t.Errorf("Latest with MinAge = %+v, %v", rel, err)
	}
}

// TestCacheExtractsWhatWasHashed checks that the archive extracted is
// the one hashed: fromCache hands back a private copy, so a cached file
// swapped after the check changes nothing.
func TestCacheExtractsWhatWasHashed(t *testing.T) {
	good := []byte("the archive that was checked")
	sum := sha256.Sum256(good)
	want := hex.EncodeToString(sum[:])
	u := &Updater{CacheDir: t.TempDir()}
	cached := u.cachePath("rhythm-tuner_linux_amd64.tar.gz", want)
	if err := os.WriteFile(cached, good, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	copyPath, got, ok := u.fromCache("rhythm-tuner_linux_amd64.tar.gz", want, dir, ".x.download-*")
	if !ok || got != want || filepath.Dir(copyPath) != dir {
		t.Fatalf("fromCache = %q, %s, %v", copyPath, got, ok)
	}
	if err := os.WriteFile(cached, []byte("swapped in after the check"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(b, good) {
		t.Errorf("copy = %q, %v", b, err)
	}
	// A cached file that doesn't match is removed and leaves no copy.
	if _, _, ok := u.fromCache("rhythm-tuner_linux_amd64.tar.gz", want, dir, ".x.download-*"); ok {
		t.Error("a swapped cache file was used")
	}
	if _, err := os.Stat(cached); !os.IsNotExist(err) {
		t.Errorf("bad cache file kept: %v", err)
	}
	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("temp files left: %v", names)
	}
}
