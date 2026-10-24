// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeRelease(t *testing.T, binary, version string, body []byte) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: binary, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	zw.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	name := fmt.Sprintf("%s_%s_%s.tar.gz", binary, runtime.GOOS, runtime.GOARCH)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":"v%s","assets":[{"name":%q,"browser_download_url":"%s/a"},{"name":"checksums.txt","browser_download_url":"%s/s"}]}`,
				version, name, srv.URL, srv.URL)
		case r.URL.Path == "/a":
			w.Write(archive)
		case r.URL.Path == "/s":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive naming differs on windows")
	}
	srv := fakeRelease(t, "rhythm-tuner", "0.2.0", []byte("#!/bin/sh\necho rhythm-tuner 0.2.0\n"))
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"install", "t", "--dir", dir}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Installed rhythm-tuner 0.2.0") {
		t.Errorf("output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "rhythm-tuner")); err != nil {
		t.Error(err)
	}
}

func TestInstallUnknownTool(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"install", "nope", "--dir", t.TempDir()}, &out, &errb); code != 2 {
		t.Errorf("exit %d", code)
	}
}
