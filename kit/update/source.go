// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Source is where an Updater finds releases and their assets. GitHub
// Releases is the default; Updater.Source swaps in another, such as an
// offline bundle read from a tar file.
type Source interface {
	// Latest returns the newest release that is neither a draft nor a
	// prerelease.
	Latest(ctx context.Context) (*Release, error)
	// ByTag returns the release with this version, given without the
	// leading v; a missing one is an ErrNoRelease.
	ByTag(ctx context.Context, version string) (*Release, error)
	// Open streams one of rel's assets (named as in rel.Assets) and
	// returns its size, or -1 when unknown. The caller closes it.
	Open(ctx context.Context, rel *Release, asset string) (io.ReadCloser, int64, error)
}

// Lister is a Source that can also list its releases, in any order and
// with drafts left out. Resolve needs it for a partial version such as
// "0.3"; Latest and Resolve need it when MinAge is set.
type Lister interface {
	Releases(ctx context.Context) ([]*Release, error)
}

func (u *Updater) source() Source {
	if u.Source != nil {
		return u.Source
	}
	return gitHub{u}
}

// gitHub is the default Source: the GitHub Releases API at u.API.
type gitHub struct{ u *Updater }

func (g gitHub) Latest(ctx context.Context) (*Release, error) {
	var v ghRelease
	if err := g.u.getJSON(ctx, "/releases/latest", "", &v); err != nil {
		return nil, err
	}
	return v.release(), nil
}

func (g gitHub) ByTag(ctx context.Context, version string) (*Release, error) {
	tag := "v" + version
	var v ghRelease
	// Escaped as well, for callers that use this Source without ByTag's
	// check: a / ? or # in the version must not reshape the request.
	if err := g.u.getJSON(ctx, "/releases/tags/"+url.PathEscape(tag), tag, &v); err != nil {
		return nil, err
	}
	if v.Draft {
		return nil, &ErrNoRelease{Repo: g.u.Repo, API: g.u.apiBase(), Version: tag}
	}
	// A broken or hostile mirror could answer with another release, an
	// older one still signed, to downgrade a pinned install.
	if got := strings.TrimPrefix(v.TagName, "v"); got != version {
		return nil, &ErrTagMismatch{Repo: g.u.Repo, API: g.u.apiBase(), Want: tag, Got: v.TagName}
	}
	return v.release(), nil
}

// Releases lists the newest 100 releases, drafts left out. Pinning to a
// line older than that needs an exact version, which ByTag finds at any
// age.
func (g gitHub) Releases(ctx context.Context) ([]*Release, error) {
	var vs []ghRelease
	if err := g.u.getJSON(ctx, "/releases?per_page=100", "", &vs); err != nil {
		return nil, err
	}
	out := make([]*Release, 0, len(vs))
	for i := range vs {
		if !vs[i].Draft {
			out = append(out, vs[i].release())
		}
	}
	return out, nil
}

func (g gitHub) Open(ctx context.Context, rel *Release, asset string) (io.ReadCloser, int64, error) {
	if g.u.offline() {
		return nil, 0, ErrOffline
	}
	raw, ok := rel.Assets[asset]
	if !ok {
		return nil, 0, fmt.Errorf("release %s has no %s", rel.Version, asset)
	}
	return g.u.open(ctx, raw)
}

// cachePath is where CacheDir keeps the archive name with this sha256.
func (u *Updater) cachePath(name, sum string) string {
	ext := ".tar.gz"
	if strings.HasSuffix(name, ".zip") {
		ext = ".zip"
	}
	return filepath.Join(u.CacheDir, sum+ext)
}

// fromCache copies the cached archive with sha256 want into a private
// temp file in dir and returns that copy and its hash, computed on the
// copy as it is written. The cache dir may be shared and writable by
// others, so its contents are never trusted by name. The file that was
// hashed is the file that gets extracted: swapping the cached file
// afterwards changes nothing. A cached file that doesn't match is
// removed. The caller removes the copy.
func (u *Updater) fromCache(name, want, dir, pattern string) (path, sum string, ok bool) {
	if u.CacheDir == "" {
		return "", "", false
	}
	cached := u.cachePath(name, want)
	src, err := os.Open(cached) //nolint:gosec // checked by hash below
	if err != nil {
		return "", "", false
	}
	defer func() { _ = src.Close() }()
	dst, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", "", false
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(dst, h), &capReader{r: src, left: maxArchive})
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst.Name())
		return "", "", false
	}
	if sum = hex.EncodeToString(h.Sum(nil)); sum != want {
		_ = os.Remove(dst.Name())
		_ = os.Remove(cached)
		return "", "", false
	}
	return dst.Name(), sum, true
}

// toCache stores a verified archive under its sha256, by a hard link when
// the cache is on the same file system and a copy otherwise. The cache is
// only a shortcut, so failures are ignored.
func (u *Updater) toCache(archive, name, sum string) {
	if u.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(u.CacheDir, 0o700); err != nil {
		return
	}
	f, err := os.CreateTemp(u.CacheDir, ".store-*")
	if err != nil {
		return
	}
	tmp := f.Name()
	_ = f.Close()
	_ = os.Remove(tmp)
	if os.Link(archive, tmp) != nil && copyFile(archive, tmp, 0o600) != nil {
		_ = os.Remove(tmp)
		return
	}
	if os.Rename(tmp, u.cachePath(name, sum)) != nil {
		_ = os.Remove(tmp)
	}
}
