// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRoutes(t *testing.T) {
	rel, err := newRelease("0.7.0", "linux", "amd64", []byte("#!/bin/sh\necho rhythm 0.7.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rel)
	t.Cleanup(srv.Close)
	get := func(path string) (int, []byte) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, body
	}

	for _, p := range []string{"/repos/RhythmShahriar/rhythm/releases/latest", "/repos/RhythmShahriar/rhythm/releases/tags/v0.7.0"} {
		code, body := get(p)
		var r struct {
			Tag    string `json:"tag_name"`
			Assets []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if code != 200 || json.Unmarshal(body, &r) != nil || r.Tag != "v0.7.0" || len(r.Assets) != 3 {
			t.Fatalf("%s: %d %s", p, code, body)
		}
		for _, a := range r.Assets {
			if !strings.HasPrefix(a.URL, srv.URL+"/dl/rhythm/v0.7.0/") {
				t.Errorf("asset %s at %s", a.Name, a.URL)
			}
		}
	}
	_, archive := get("/dl/rhythm/v0.7.0/rhythm_linux_amd64.tar.gz")
	_, sums := get("/dl/rhythm/v0.7.0/checksums.txt")
	sum := sha256.Sum256(archive)
	if string(sums) != hex.EncodeToString(sum[:])+"  rhythm_linux_amd64.tar.gz\n" {
		t.Errorf("checksums.txt = %q", sums)
	}
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	h, err := tar.NewReader(zr).Next()
	if err != nil || h.Name != "rhythm" || h.Mode != 0o755 {
		t.Errorf("archive entry %+v, %v", h, err)
	}
	if code, _ := get("/dl/tuner/v0.1.0/tuner_linux_amd64.tar.gz"); code != 200 {
		t.Errorf("tool archive: %d", code)
	}
	for _, p := range []string{"/repos/RhythmShahriar/rhythm/releases/tags/v0.6.0", "/dl/rhythm/v0.7.0/other", "/repos/RhythmShahriar/tuner/releases/latest"} {
		if code, _ := get(p); code != 404 {
			t.Errorf("%s: %d, want 404", p, code)
		}
	}
}

func TestWindowsReleaseIsAZip(t *testing.T) {
	rel, err := newRelease("0.7.0", "windows", "arm64", []byte("MZ"))
	if err != nil {
		t.Fatal(err)
	}
	data := rel.files["rhythm_windows_arm64.zip"]
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "rhythm.exe" {
		t.Fatalf("zip: %v", err)
	}
}

func TestRunRefuses(t *testing.T) {
	stub := filepath.Join(t.TempDir(), "rhythm")
	if err := os.WriteFile(stub, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{},
		{"-stub", stub, "-addr", "0.0.0.0:23039"},
		{"-stub", stub, "-addr", "example.com:80"},
		{"-stub", filepath.Join(t.TempDir(), "missing"), "-addr", "127.0.0.1:23039"},
	} {
		if err := run(args, io.Discard); err == nil {
			t.Errorf("run(%q) started", args)
		}
	}
}
