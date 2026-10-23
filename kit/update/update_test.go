package update

import (
	"archive/tar"
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
	"strings"
	"testing"
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
	}
	for _, c := range cases {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.cur, c.latest, got, c.want)
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

// fakeGitHub serves a release whose archive contains newBinary. If tamper
// is set, checksums.txt doesn't match the archive.
func fakeGitHub(t *testing.T, newBinary []byte, tamper bool) *httptest.Server {
	t.Helper()
	u := &Updater{Binary: "rhythm-tuner"}
	name := u.ArchiveName()
	archive := tarGz(t, "rhythm-tuner", newBinary)
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
	if fi, _ := os.Stat(dest); fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("not executable: %v", fi.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(dest)); len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CI", "")
	t.Setenv("RHYTHM_NO_UPDATE_CHECK", "")
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
