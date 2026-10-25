// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

var errTooLarge = errors.New("file is larger than expected")

// open starts an HTTP download of raw and returns its body and announced
// size (-1 when unknown) once the status is 200.
func (u *Updater) open(ctx context.Context, raw string) (io.ReadCloser, int64, error) {
	if err := checkURL(raw); err != nil {
		return nil, 0, err
	}
	req, err := u.newRequest(ctx, raw)
	if err != nil {
		return nil, 0, err
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, 0, netError(req, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, 0, u.statusError(req, resp)
	}
	return resp.Body, resp.ContentLength, nil
}

// download reads a small asset of rel, such as checksums.txt, into
// memory.
func (u *Updater) download(ctx context.Context, rel *Release, asset string, limit int64) ([]byte, error) {
	body, _, err := u.source().Open(ctx, rel, asset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

// downloadFile streams an asset of rel into a new temp file in dir,
// hashing it on the way; it returns the file's path and hex sha256. The
// file is removed on every error, including a canceled ctx.
func (u *Updater) downloadFile(ctx context.Context, rel *Release, asset, dir, pattern string, limit int64) (file, sum string, err error) {
	body, total, err := u.source().Open(ctx, rel, asset)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = body.Close() }()
	if total > limit {
		return "", "", errTooLarge
	}
	f, err := createTemp(dir, pattern)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	h := sha256.New()
	p := &progress{fn: u.Progress, total: total, last: time.Now()}
	n, err := io.Copy(io.MultiWriter(f, h, p), io.LimitReader(body, limit+1))
	if err != nil {
		return "", "", err
	}
	if n > limit {
		return "", "", errTooLarge
	}
	if err = f.Close(); err != nil {
		return "", "", err
	}
	p.finish(n)
	return f.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// createTemp is os.CreateTemp with a hint for the common permission
// error: a binary installed by root.
func createTemp(dir, pattern string) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if errors.Is(err, os.ErrPermission) {
		return nil, fmt.Errorf("can't write to %s; rerun with sudo, or reinstall to a directory you own", dir)
	}
	return f, err
}

// progress is an io.Writer that counts bytes and reports them to fn at
// most every 100 ms.
type progress struct {
	fn    func(done, total int64)
	total int64
	done  int64
	last  time.Time
}

func (p *progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.fn != nil && time.Since(p.last) >= 100*time.Millisecond {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
	return len(b), nil
}

// finish makes the last call, with done == total even when the server
// announced no size.
func (p *progress) finish(n int64) {
	if p.fn != nil {
		p.fn(n, n)
	}
}

// capReader fails, instead of silently truncating, once more than left
// bytes are read.
type capReader struct {
	r    io.Reader
	left int64
}

// Read asks for one byte past the cap, so a binary of exactly left bytes
// passes and a bigger one fails instead of being cut short.
func (c *capReader) Read(b []byte) (int, error) {
	if c.left < 0 {
		return 0, errors.New("binary is larger than expected")
	}
	if int64(len(b)) > c.left+1 {
		b = b[:c.left+1]
	}
	n, err := c.r.Read(b)
	c.left -= int64(n)
	if c.left < 0 {
		return n, errors.New("binary is larger than expected")
	}
	return n, err
}

// tagged prefixes read errors with the archive name, so a corrupt archive
// found while the binary is copied reads the same as one found earlier.
type tagged struct {
	r    io.Reader
	name string
}

func (t tagged) Read(b []byte) (int, error) {
	n, err := t.r.Read(b)
	if err != nil && !errors.Is(err, io.EOF) {
		err = fmt.Errorf("%s: %w", t.name, err)
	}
	return n, err
}

// extract finds bin in the archive file and hands a stream of it to use.
// Nothing is unpacked into memory: tar.gz is read as a stream and zip
// through the file's io.ReaderAt. Errors about the archive itself are
// prefixed with its name; errors from use come back as they are.
func extract(file, name, bin string, use func(io.Reader) error) error {
	f, err := os.Open(file) //nolint:gosec // our own temp file
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if path.Ext(name) == ".zip" {
		return fromZip(f, name, bin, use)
	}
	return fromTarGz(f, name, bin, use)
}

func fromTarGz(f *os.File, name, bin string, use func(io.Reader) error) error {
	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s: no %s in the archive", name, bin)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		// Regular files only: a symlink or hard link entry with the
		// binary's name holds no data, just the name of another file.
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == bin {
			return use(tagged{tr, name})
		}
	}
}

func fromZip(f *os.File, name, bin string, use func(io.Reader) error) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	for _, zf := range zr.File {
		// Regular files only, as in a tar.gz: a symlink entry's data is
		// the name of another file, not the binary.
		if path.Base(zf.Name) == bin && zf.Mode().IsRegular() {
			rc, err := zf.Open()
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			defer func() { _ = rc.Close() }()
			return use(tagged{rc, name})
		}
	}
	return fmt.Errorf("%s: no %s in the archive", name, bin)
}
