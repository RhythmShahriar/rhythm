// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// releaseServer plays the GitHub API with one release of rhythm-tuner
// for linux/amd64, laid out as the release contract says.
func releaseServer() *httptest.Server {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	bin := []byte("#!/bin/sh\necho rhythm-tuner 0.3.0\n")
	_ = tw.WriteHeader(&tar.Header{Name: "rhythm-tuner", Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(bin)
	_ = tw.Close()
	_ = zw.Close()
	archive := buf.Bytes()
	sums := fmt.Sprintf("%x  rhythm-tuner_linux_amd64.tar.gz\n", sha256.Sum256(archive))

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/RhythmShahriar/tuner/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.3.0","assets":[
				{"name":"rhythm-tuner_linux_amd64.tar.gz","browser_download_url":"%[1]s/dl/archive"},
				{"name":"checksums.txt","browser_download_url":"%[1]s/dl/checksums.txt"}]}`, srv.URL)
		case "/dl/archive":
			_, _ = w.Write(archive)
		case "/dl/checksums.txt":
			_, _ = fmt.Fprint(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

// Look up the latest release and install it over an older binary.
func ExampleUpdater_Install() {
	srv := releaseServer()
	defer srv.Close()
	dir, _ := os.MkdirTemp("", "example")
	defer func() { _ = os.RemoveAll(dir) }()
	dest := filepath.Join(dir, "rhythm-tuner")

	u := &update.Updater{
		Repo:    "RhythmShahriar/tuner",
		Binary:  "rhythm-tuner",
		Current: "0.2.0",
		API:     srv.URL, // normally empty, or RHYTHM_GITHUB_API
		OS:      "linux", // normally empty: the running platform
		Arch:    "amd64",
	}
	ctx := context.Background()
	rel, err := u.Latest(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !update.Newer(u.Current, rel.Version) {
		fmt.Println("up to date")
		return
	}
	if err := u.Install(ctx, rel, dest); err != nil {
		fmt.Println(err)
		return
	}
	got, _ := os.ReadFile(dest)
	fmt.Printf("installed %s from %s\n%s", rel.Version, u.ArchiveName(), got)
	// Output:
	// installed 0.3.0 from rhythm-tuner_linux_amd64.tar.gz
	// #!/bin/sh
	// echo rhythm-tuner 0.3.0
}
