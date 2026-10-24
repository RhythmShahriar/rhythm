// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// bigArchive writes a tar.gz holding an n-byte rhythm-tuner to a file,
// without holding either in memory and returns its path and sha256.
// Stored uncompressed, so the archive is as big as the binary.
func bigArchive(t *testing.T, n int64) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "big.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	zw, _ := gzip.NewWriterLevel(io.MultiWriter(f, h), gzip.NoCompression)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: "rhythm-tuner", Mode: 0o755, Size: n, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(tw, rand.NewChaCha8([32]byte{}), n); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path, hex.EncodeToString(h.Sum(nil))
}

// fileRelease serves the archive at path, with its checksum, as a release
// of rhythm-tuner.
func fileRelease(t *testing.T, u *Updater, path, sum string) *Release {
	t.Helper()
	name := u.ArchiveName()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			http.ServeFile(w, r, path)
		case "/sums":
			fmt.Fprintf(w, "%s  %s\n", sum, name)
		}
	}))
	t.Cleanup(srv.Close)
	return &Release{Version: "0.2.0", Assets: map[string]string{name: srv.URL + "/archive", "checksums.txt": srv.URL + "/sums"}}
}

func TestInstallStreamsWithFlatMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 50 MB archive")
	}
	const size = 50 << 20
	path, sum := bigArchive(t, size)
	fi, _ := os.Stat(path)
	var mu sync.Mutex
	var calls [][2]int64
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Progress: func(done, total int64) {
		mu.Lock()
		calls = append(calls, [2]int64{done, total})
		mu.Unlock()
	}}
	rel := fileRelease(t, u, path, sum)
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := u.Install(context.Background(), rel, dest); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	// Everything allocated during Install, server side included, so a
	// single in-memory copy of the archive would fail this.
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 20<<20 {
		t.Errorf("Install allocated %d MB for a %d MB archive, want under 20 MB", grew>>20, size>>20)
	}
	if got, _ := os.Stat(dest); got == nil || got.Size() != size {
		t.Errorf("installed binary = %v, want %d bytes", got, size)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("temp files left behind: %v", names)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatal("Progress never called")
	}
	if last := calls[len(calls)-1]; last[0] != fi.Size() || last[1] != fi.Size() {
		t.Errorf("last Progress = %v, want done == total == %d", last, fi.Size())
	}
	for i := 1; i < len(calls); i++ {
		if calls[i][0] < calls[i-1][0] {
			t.Errorf("Progress went backwards: %v", calls)
		}
	}
}

func TestProgressIsThrottled(t *testing.T) {
	calls := 0
	p := &progress{fn: func(done, total int64) { calls++ }, total: 100, last: time.Now()}
	for range 1000 {
		p.Write(make([]byte, 1))
	}
	if calls != 0 {
		t.Errorf("%d calls within 100 ms, want 0", calls)
	}
	p.last = time.Now().Add(-time.Second)
	p.Write([]byte("x"))
	p.finish(100)
	if calls != 2 {
		t.Errorf("%d calls, want one after 100 ms and one at the end", calls)
	}
}

func TestProgressWithoutContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("part one "))
		w.(http.Flusher).Flush() // forces chunked encoding: no Content-Length
		w.Write([]byte("part two"))
	}))
	defer srv.Close()
	var last [2]int64
	u := &Updater{Binary: "rhythm-tuner", Progress: func(done, total int64) { last = [2]int64{done, total} }}
	file, _, err := u.downloadFile(context.Background(), srv.URL, t.TempDir(), "dl-*", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if last != [2]int64{17, 17} {
		t.Errorf("last Progress = %v, want done == total == 17", last)
	}
	if b, _ := os.ReadFile(file); string(b) != "part one part two" {
		t.Errorf("file = %q", b)
	}
}

func TestCanceledDownloadRemovesTempFiles(t *testing.T) {
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sums":
			fmt.Fprintf(w, "%s  %s\n", strings.Repeat("a", 64), name)
		case "/archive":
			w.Header().Set("Content-Length", strconv.Itoa(100<<20))
			chunk := make([]byte, 64<<10)
			for {
				if _, err := w.Write(chunk); err != nil {
					return
				}
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel from the first progress report, so the temp file exists and
	// holds data when the download stops.
	u := &Updater{Binary: "rhythm-tuner", Progress: func(done, total int64) { cancel() }}
	rel := &Release{Version: "0.2.0", Assets: map[string]string{name: srv.URL + "/archive", "checksums.txt": srv.URL + "/sums"}}
	dest := installed(t)
	err := u.Install(ctx, rel, dest)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 || names[0] != "rhythm-tuner" {
		t.Errorf("dir = %v, want only rhythm-tuner", names)
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Errorf("binary = %q", got)
	}
}

func TestDownloadSizeCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/announced" {
			w.Header().Set("Content-Length", "2048")
			w.Write(make([]byte, 2048))
			return
		}
		w.Write(make([]byte, 1024))
		w.(http.Flusher).Flush() // chunked: the cap must hold while reading
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()
	u := &Updater{Binary: "rhythm-tuner"}
	for _, p := range []string{"/announced", "/chunked"} {
		dir := t.TempDir()
		_, _, err := u.downloadFile(context.Background(), srv.URL+p, dir, "dl-*", 1024)
		if !errors.Is(err, errTooLarge) {
			t.Errorf("%s: err = %v, want errTooLarge", p, err)
		}
		if names := dirNames(t, dir); len(names) != 0 {
			t.Errorf("%s: temp files left behind: %v", p, names)
		}
	}
}

func TestInstallFromZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("rhythm-tuner_windows_amd64/rhythm-tuner.exe")
	w.Write([]byte("NEW EXE"))
	zw.Close()
	path := filepath.Join(t.TempDir(), "a.zip")
	os.WriteFile(path, buf.Bytes(), 0o600)
	sum := sha256.Sum256(buf.Bytes())

	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", OS: "windows", Arch: "amd64"}
	rel := fileRelease(t, u, path, hex.EncodeToString(sum[:]))
	dest := installed(t)
	if err := u.Install(context.Background(), rel, dest); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "NEW EXE" {
		t.Errorf("binary = %q", got)
	}
}

func TestExtractErrorsNameTheArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tar.gz")
	os.WriteFile(path, []byte("not gzip"), 0o600)
	err := extract(path, "rhythm-tuner_linux_amd64.tar.gz", "rhythm-tuner", func(io.Reader) error { return nil })
	if err == nil || !strings.HasPrefix(err.Error(), "rhythm-tuner_linux_amd64.tar.gz: ") {
		t.Errorf("err = %v", err)
	}
	os.WriteFile(path, tarGz(t, "other", []byte("x")), 0o600)
	err = extract(path, "a.tar.gz", "rhythm-tuner", func(io.Reader) error { return nil })
	if err == nil || err.Error() != "a.tar.gz: no rhythm-tuner in the archive" {
		t.Errorf("err = %v", err)
	}
}

func TestCapReader(t *testing.T) {
	if _, err := io.ReadAll(&capReader{r: strings.NewReader("12345"), left: 5}); err != nil {
		t.Errorf("exactly at the cap: %v", err)
	}
	if _, err := io.ReadAll(&capReader{r: strings.NewReader("123456"), left: 5}); err == nil {
		t.Error("over the cap: no error")
	}
}
