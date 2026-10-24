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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testBundle = `{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`

// signedRelease serves a release of rhythm-tuner 0.2.0 with a bundle
// (unless noBundle) and counts the requests per path.
func signedRelease(t *testing.T, noBundle bool) (rel *Release, sums []byte, hits func(string) int) {
	t.Helper()
	name := (&Updater{Binary: "rhythm-tuner"}).ArchiveName()
	archive := tarGz(t, "rhythm-tuner", []byte("NEW BINARY"))
	sum := sha256.Sum256(archive)
	sums = fmt.Appendf(nil, "%s  other.tar.gz\n%s  %s\n", hex.EncodeToString(make([]byte, 32)), hex.EncodeToString(sum[:]), name)
	var mu sync.Mutex
	count := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/archive":
			w.Write(archive)
		case "/sums":
			w.Write(sums)
		case "/bundle":
			w.Write([]byte(testBundle))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	rel = &Release{Version: "0.2.0", Assets: map[string]string{name: srv.URL + "/archive", "checksums.txt": srv.URL + "/sums"}}
	if !noBundle {
		rel.Assets[bundleAsset] = srv.URL + "/bundle"
	}
	return rel, sums, func(p string) int {
		mu.Lock()
		defer mu.Unlock()
		return count[p]
	}
}

// fakeVerifier records what it was asked and answers with signer or err.
type fakeVerifier struct {
	signer string
	err    error

	calls             int
	repo, version     string
	checksums, bundle []byte
}

func (f *fakeVerifier) VerifyChecksums(_ context.Context, repo, version string, checksums, bundle []byte) (string, error) {
	f.calls++
	f.repo, f.version, f.checksums, f.bundle = repo, version, checksums, bundle
	return f.signer, f.err
}

const workflow = "https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master"

func TestVerifierGetsTheExactChecksums(t *testing.T) {
	rel, sums, _ := signedRelease(t, false)
	v := &fakeVerifier{signer: workflow}
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Verifier: v}
	dest := installed(t)
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v.calls != 1 || v.repo != "RhythmShahriar/tuner" || v.version != "0.2.0" {
		t.Errorf("Verifier got calls=%d repo=%q version=%q", v.calls, v.repo, v.version)
	}
	if !bytes.Equal(v.checksums, sums) || string(v.bundle) != testBundle {
		t.Errorf("Verifier got checksums %q bundle %q", v.checksums, v.bundle)
	}
	archive := tarGz(t, "rhythm-tuner", []byte("NEW BINARY"))
	sum := sha256.Sum256(archive)
	if want := (InstallResult{Version: "0.2.0", SHA256: hex.EncodeToString(sum[:]), Signer: workflow}); res != want {
		t.Errorf("result = %+v, want %+v", res, want)
	}
	if got, _ := os.ReadFile(dest); string(got) != "NEW BINARY" {
		t.Errorf("binary = %q", got)
	}
}

func TestFailingVerifierLeavesDestUntouched(t *testing.T) {
	rel, _, hits := signedRelease(t, false)
	bad := errors.New("certificate identity does not match")
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Verifier: &fakeVerifier{err: bad}}
	dest := installed(t)
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{})
	var se *ErrSignature
	if !errors.As(err, &se) || se.Repo != "RhythmShahriar/tuner" || se.Version != "0.2.0" || !errors.Is(err, bad) {
		t.Fatalf("err = %v, want ErrSignature wrapping the cause", err)
	}
	if want := "refusing to install RhythmShahriar/tuner 0.2.0: signature check failed: certificate identity does not match"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
	if res != (InstallResult{}) {
		t.Errorf("result = %+v, want zero", res)
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Errorf("binary = %q", got)
	}
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 1 {
		t.Errorf("files left behind: %v", names)
	}
	if n := hits("/archive"); n != 0 {
		t.Errorf("archive downloaded %d times before the signature passed", n)
	}
}

func TestMissingBundleFailsClosed(t *testing.T) {
	rel, _, _ := signedRelease(t, true)
	v := &fakeVerifier{signer: workflow}
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Verifier: v}
	dest := installed(t)
	err := u.Install(context.Background(), rel, dest)
	var se *ErrSignature
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want ErrSignature", err)
	}
	if want := "refusing to install RhythmShahriar/tuner 0.2.0: signature check failed: the release has no checksums.txt.sigstore.json"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
	if v.calls != 0 {
		t.Error("Verifier called without a bundle")
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Errorf("binary = %q", got)
	}

	// A bundle that is listed but can't be downloaded fails the same way.
	rel.Assets[bundleAsset] = strings.TrimSuffix(rel.Assets["checksums.txt"], "/sums") + "/missing"
	err = u.Install(context.Background(), rel, dest)
	if !errors.As(err, &se) {
		t.Errorf("unreachable bundle: err = %v, want ErrSignature", err)
	}
}

func TestSkipSignatureBypassesTheVerifier(t *testing.T) {
	rel, _, hits := signedRelease(t, true)
	v := &fakeVerifier{err: errors.New("must not be called")}
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Verifier: v, SkipSignature: true}
	dest := installed(t)
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v.calls != 0 || res.Signer != "" || hits("/bundle") != 0 {
		t.Errorf("calls=%d signer=%q bundle hits=%d, want none", v.calls, res.Signer, hits("/bundle"))
	}
	if got, _ := os.ReadFile(dest); string(got) != "NEW BINARY" {
		t.Errorf("binary = %q", got)
	}
}

func TestNilVerifierKeepsChecksumOnly(t *testing.T) {
	rel, _, hits := signedRelease(t, true)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner"}
	dest := installed(t)
	res, err := u.InstallWith(context.Background(), rel, dest, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "0.2.0" || len(res.SHA256) != 64 || res.Signer != "" || hits("/bundle") != 0 {
		t.Errorf("result = %+v, bundle hits %d", res, hits("/bundle"))
	}
}
