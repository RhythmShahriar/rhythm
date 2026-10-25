// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Command fakeapi serves a GitHub-style release API on 127.0.0.1 for the
// setup-rhythm action's own workflow (.github/workflows/setup-rhythm-test.yml).
//
//	fakeapi -stub <rhythm stand-in> [-addr 127.0.0.1:23000] [-version 0.7.0]
//
// The stand-in is the rhythm built from scripts/test/stub, which the
// installer tests use too. It is served as RhythmShahriar/rhythm's latest
// release for this machine's platform: a .tar.gz holding rhythm, or a
// .zip holding rhythm.exe on Windows, with checksums.txt and a stand-in
// checksums.txt.sigstore.json. The stand-in "installs" a tool by fetching
// /dl/<tool>/v0.1.0/<archive>, which answers 200 for every tool. Every
// request is logged to stderr.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fakeapi:", err)
		os.Exit(1)
	}
}

// run parses the flags and serves until the listener fails. The address
// must be loopback: the installers accept plain http only there.
func run(args []string, logTo io.Writer) error {
	fs := flag.NewFlagSet("fakeapi", flag.ContinueOnError)
	stub := fs.String("stub", "", "the rhythm stand-in to serve")
	addr := fs.String("addr", "127.0.0.1:23000", "where to listen; loopback only")
	version := fs.String("version", "0.7.0", "the release version to serve")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *stub == "" {
		return errors.New("-stub is required")
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr must be a loopback address, got %s", *addr)
	}
	bin, err := os.ReadFile(*stub)
	if err != nil {
		return err
	}
	rel, err := newRelease(*version, runtime.GOOS, runtime.GOARCH, bin)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: logged(rel, log.New(logTo, "", 0)), ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}

// release is the one rhythm release served, with its files by name.
type release struct {
	version string
	archive string // rhythm_<os>_<arch>.tar.gz or .zip
	files   map[string][]byte
}

// newRelease packs bin as rhythm's release version for goos/goarch.
func newRelease(version, goos, goarch string, bin []byte) (*release, error) {
	r := &release{version: version, files: map[string][]byte{}}
	var archive []byte
	var err error
	if goos == "windows" {
		r.archive = fmt.Sprintf("rhythm_%s_%s.zip", goos, goarch)
		archive, err = zipOf("rhythm.exe", bin)
	} else {
		r.archive = fmt.Sprintf("rhythm_%s_%s.tar.gz", goos, goarch)
		archive, err = tarGzOf("rhythm", bin)
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archive)
	r.files[r.archive] = archive
	r.files["checksums.txt"] = fmt.Appendf(nil, "%s  %s\n", hex.EncodeToString(sum[:]), r.archive)
	// Only a real signature check reads the bundle. The stand-in has none.
	r.files["checksums.txt.sigstore.json"] = []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`)
	return r, nil
}

// ServeHTTP answers the two release API routes, the release's own files
// under /dl/rhythm/ and any tool archive. Asset URLs are built from the
// request's Host, so they work on whatever port the server listens on.
func (r *release) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	base := "http://" + req.Host
	p := req.URL.Path
	switch {
	case p == "/repos/RhythmShahriar/rhythm/releases/latest", p == "/repos/RhythmShahriar/rhythm/releases/tags/v"+r.version:
		var assets []map[string]string
		for _, name := range []string{r.archive, "checksums.txt", "checksums.txt.sigstore.json"} {
			assets = append(assets, map[string]string{"name": name, "browser_download_url": base + "/dl/rhythm/v" + r.version + "/" + name})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v" + r.version, "prerelease": false, "assets": assets})
	case strings.HasPrefix(p, "/dl/rhythm/v"+r.version+"/"):
		data, ok := r.files[strings.TrimPrefix(p, "/dl/rhythm/v"+r.version+"/")]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write(data)
	case strings.HasPrefix(p, "/dl/") && !strings.HasPrefix(p, "/dl/rhythm/"):
		_, _ = io.WriteString(w, "a tool archive\n") // the stand-in only checks for a 200
	default:
		http.NotFound(w, req)
	}
}

// logged writes "GET <path> <status>" for each request.
func logged(h http.Handler, l *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &status{ResponseWriter: w, code: http.StatusOK}
		h.ServeHTTP(rec, r)
		l.Printf("%s %s %d", r.Method, r.URL.Path, rec.code)
	})
}

// status records the code a handler sends, for the request log.
type status struct {
	http.ResponseWriter
	code int
}

// WriteHeader records code and passes it on.
func (s *status) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

// tarGzOf returns a .tar.gz holding one executable file.
func tarGzOf(name string, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(body); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zipOf returns a .zip holding one executable file.
func zipOf(name string, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetMode(0o755)
	f, err := zw.CreateHeader(h)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(body); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
