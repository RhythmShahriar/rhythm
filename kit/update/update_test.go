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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cache"
	"github.com/RhythmShahriar/rhythm/kit/config"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"0.1.0", "0.2.0", true},
		{"v0.1.0", "0.1.1", true},
		{"0.10.0", "0.9.0", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.0-rc1", "1.0.0", true},
		{"1.0.0", "1.0.0-rc1", false},
		{"dev", "9.9.9", false},
		{"0.1.0", "garbage", false},
		{"1.0.0-rc9", "1.0.0-rc10", true},
		{"1.0.0-rc10", "1.0.0-rc9", false},
		{"1.0.0-beta.2", "1.0.0-beta.10", true},
		{"1.0.0-rc.1", "1.0.0-rc.1", false},
	}
	for _, c := range cases {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.cur, c.latest, got, c.want)
		}
	}
}

// The precedence example from SemVer 2.0.0 section 11, plus rc9 < rc10.
func TestNewerFollowsSemVerPrecedence(t *testing.T) {
	chain := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0-rc9", "1.0.0-rc10", "1.0.0",
	}
	for i, lo := range chain {
		for _, hi := range chain[i+1:] {
			if !Newer(lo, hi) {
				t.Errorf("Newer(%q, %q) = false, want true", lo, hi)
			}
			if Newer(hi, lo) {
				t.Errorf("Newer(%q, %q) = true, want false", hi, lo)
			}
		}
	}
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"1", "a", -1},
		{"999999999999999999999999", "1000000000000000000000000", -1},
		{"rc01", "rc1", -1}, // equal by value; ASCII breaks the tie
		{"x.1", "x.1.0", -1},
		{"beta", "alpha", 1},
	} {
		if got := comparePre(c.a, c.b); got != c.want {
			t.Errorf("comparePre(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, f := range []struct {
		name string
		body []byte
	}{{"README.md", []byte("hi")}, {name, body}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		tw.Write(f.body)
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

// zipOf is a zip of README.md and name, as the Windows releases are.
func zipOf(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct {
		name string
		body []byte
	}{{"README.md", []byte("hi")}, {name, body}} {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(f.body)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// hostArchive is the archive ArchiveName names on this platform: a
// tar.gz holding binary, or on Windows a zip holding binary.exe.
func hostArchive(t *testing.T, binary string, body []byte) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		return zipOf(t, binary+".exe", body)
	}
	return tarGz(t, binary, body)
}

// hostExt is the extension of this platform's archives.
func hostExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// fakeGitHub serves a release whose archive contains newBinary. If tamper
// is set, checksums.txt doesn't match the archive.
func fakeGitHub(t *testing.T, newBinary []byte, tamper bool) *httptest.Server {
	t.Helper()
	u := &Updater{Binary: "rhythm-tuner"}
	name := u.ArchiveName()
	archive := hostArchive(t, "rhythm-tuner", newBinary)
	sum := sha256.Sum256(archive)
	if tamper {
		sum[0] ^= 0xff
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/RhythmShahriar/tuner/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","assets":[{"name":%q,"browser_download_url":"%s/dl/archive"},
				{"name":"checksums.txt","browser_download_url":"%s/dl/sums"}]}`, name, srv.URL, srv.URL)
		case "/dl/archive":
			w.Write(archive)
		case "/dl/sums":
			fmt.Fprintf(w, "%s  other_file.tar.gz\n%s  %s\n", strings.Repeat("0", 64), hex.EncodeToString(sum[:]), name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstall(t *testing.T) {
	srv := fakeGitHub(t, []byte("NEW BINARY"), false)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.1.0", API: srv.URL}
	rel, err := u.Latest(context.Background())
	if err != nil || rel.Version != "0.2.0" {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	os.WriteFile(dest, []byte("OLD BINARY"), 0o755)
	if err := u.Install(context.Background(), rel, dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "NEW BINARY" {
		t.Errorf("binary = %q", got)
	}
	// Windows has no executable bit; Go reports 0666 there.
	if fi, _ := os.Stat(dest); runtime.GOOS != "windows" && fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("not executable: %v", fi.Mode())
	}
	// Only the binary and the one it replaced, kept for Rollback.
	if names := dirNames(t, filepath.Dir(dest)); len(names) != 3 {
		t.Errorf("temp files left behind: %v", names)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	srv := fakeGitHub(t, []byte("EVIL"), true)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL}
	rel, _ := u.Latest(context.Background())
	dest := filepath.Join(t.TempDir(), "rhythm-tuner")
	os.WriteFile(dest, []byte("OLD BINARY"), 0o755)
	err := u.Install(context.Background(), rel, dest)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD BINARY" {
		t.Error("binary replaced despite a checksum mismatch")
	}
}

func TestCheckCachesForADay(t *testing.T) {
	isolate(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"tag_name":"v0.3.0","assets":[]}`))
	}))
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL}
	for range 3 {
		if latest, newer := u.Check(context.Background()); latest != "0.3.0" || !newer {
			t.Fatalf("Check = %q, %v", latest, newer)
		}
	}
	if hits != 1 {
		t.Errorf("GitHub asked %d times, want 1", hits)
	}
	u.Current = "dev"
	if _, newer := u.Check(context.Background()); newer {
		t.Error("dev builds must not get update notices")
	}
}

// statePath is where Check keeps its state for rhythm-tuner.
func statePath(t *testing.T) string {
	t.Helper()
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "update-check", "rhythm-tuner.json")
}

func TestCheckRefetchesBadState(t *testing.T) {
	for name, state := range map[string]string{
		"future":  fmt.Sprintf(`{"checked":%q,"latest":"0.2.0"}`, time.Now().Add(48*time.Hour).UTC().Format(time.RFC3339)),
		"corrupt": `{"checked":"yesterday`,
	} {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			path := statePath(t)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
				t.Fatal(err)
			}
			srv, hits := counting(t)
			u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL}
			if latest, newer := u.Check(context.Background()); latest != "0.3.0" || !newer {
				t.Errorf("Check = %q, %v", latest, newer)
			}
			if hits.Load() != 1 {
				t.Errorf("%d requests, want 1", hits.Load())
			}
			var st checkState
			if err := cache.ReadJSON(path, &st); err != nil || st.Latest != "0.3.0" || time.Since(st.Checked) < 0 {
				t.Errorf("state not rewritten: %+v, %v", st, err)
			}
		})
	}
}

func TestConcurrentChecksLeaveAParseableState(t *testing.T) {
	isolate(t)
	srv, _ := counting(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL}
			u.Check(context.Background())
		})
	}
	wg.Wait()
	var st checkState
	if err := cache.ReadJSON(statePath(t), &st); err != nil || st.Latest != "0.3.0" {
		t.Errorf("state = %+v, %v", st, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(statePath(t))); len(entries) != 1 {
		t.Errorf("files left in the state dir: %v", entries)
	}
}
