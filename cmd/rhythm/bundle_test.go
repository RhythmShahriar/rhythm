// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/internal/bundle"
)

// lockedSandbox has Tuner 0.3.1 and Soundcheck 0.4.1 installed and
// locked, signed releases and rhythm 0.7.0.
func lockedSandbox(t *testing.T) (*sandbox, *releaseAPI) {
	t.Helper()
	s, f := signedSandbox(t)
	s.stub(t, "rhythm-tuner", "0.3.1", "")
	s.stub(t, "rhythm-soundcheck", "0.4.1", "")
	writeLock(t)
	return s, f
}

// airGap cuts rhythm off: offline mode on and the API on a closed port.
func airGap(t *testing.T) {
	t.Helper()
	t.Setenv("RHYTHM_OFFLINE", "1")
	t.Setenv("RHYTHM_GITHUB_API", closedPort(t))
}

func TestBundleInstallsOffline(t *testing.T) {
	s, f := lockedSandbox(t)
	lock, err := readLock(lockFileName)
	if err != nil {
		t.Fatal(err)
	}
	if e := lock.Entry("tuner"); e == nil || e.Signer != sharedSigner {
		t.Fatalf("the lock doesn't record the signer: %+v", e)
	}
	code, out, errs := runCmd("bundle", "--locked", "-o", "rhythm-bundle.tar.gz")
	if code != 0 || errs != "" {
		t.Fatalf("bundle: exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{"rhythm             0.7.0  " + thisPlatform(), "rhythm-tuner       0.3.1  " + thisPlatform(),
		"rhythm-soundcheck  0.4.1  " + thisPlatform(), "Wrote " + filepath.Join(mustGetwd(t), "rhythm-bundle.tar.gz")} {
		if !strings.Contains(out, want) {
			t.Errorf("bundle output lacks %q:\n%s", want, out)
		}
	}
	b, err := bundle.Open("rhythm-bundle.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	m := b.Manifest()
	if m.Schema != "rhythm-bundle/v1" || len(m.Releases) != 3 || !b.HasPlatform(thisPlatform()) || !strings.Contains(string(m.Lock), `"rhythm-lock/v1"`) {
		t.Errorf("manifest %+v", m)
	}
	for _, r := range m.Releases {
		if _, ok := r.Assets[signatureAsset]; !ok {
			t.Errorf("%s has no signature in the bundle", r.Repo)
		}
	}

	// The air-gapped machine.
	airGap(t)
	before := len(f.requests())
	dir := t.TempDir()
	code, out, errs = runCmd("install", "--from", "rhythm-bundle.tar.gz", "--locked", "--dir", dir)
	if code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{"Installed rhythm-tuner 0.3.1", "Installed rhythm-soundcheck 0.4.1", "  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.1 (release-go.yml)"} {
		if !strings.Contains(out, want) {
			t.Errorf("install output lacks %q:\n%s", want, out)
		}
	}
	for tool, v := range map[string]string{"tuner": "0.3.1", "soundcheck": "0.4.1"} {
		binary := "rhythm-" + tool
		got, err := os.ReadFile(filepath.Join(dir, binary))
		want := stubScript(binary, v, s.log(binary), "")
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s isn't the locked binary: %v", binary, err)
		}
		if got, want := recordedStatus(filepath.Join(dir, binary)), "yes"; got != want {
			t.Errorf("%s signed = %s", binary, got)
		}
	}
	if n := len(f.requests()); n != before {
		t.Errorf("the offline install made %d requests", n-before)
	}
	// The verifier of a bundle install uses the root built into rhythm.
	end := beginRun(&runOptions{bundle: "x"})
	defer end()
	if !verifierOptions().Offline {
		t.Error("a bundle install would refresh the Sigstore root online")
	}
}

// rewrite copies the bundle at src to dst, passing each file through
// edit and the manifest through editManifest.
func rewrite(t *testing.T, src, dst string, edit func(name string, data []byte) []byte, editManifest func(*bundle.Manifest)) {
	t.Helper()
	b, err := bundle.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	m := b.Manifest()
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	w := bundle.NewWriter(f)
	for i, r := range m.Releases {
		s, err := b.Source(r.Repo)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := s.ByTag(context.Background(), r.Version)
		if err != nil {
			t.Fatal(err)
		}
		for name, file := range r.Assets {
			rc, _, err := s.Open(context.Background(), rel, name)
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			if edit != nil {
				data = edit(name, data)
			}
			if m.Releases[i].Assets[name], err = w.Add(file.Path, data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if editManifest != nil {
		editManifest(&m)
	}
	if err := w.Close(m); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestBundleTamperedArchiveFails(t *testing.T) {
	s, _ := lockedSandbox(t)
	if code, out, errs := runCmd("bundle", "--locked", "-o", "good.tar.gz"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	tuner := "rhythm-tuner_" + thisPlatform() + ".tar.gz"
	rewrite(t, "good.tar.gz", "bad.tar.gz", func(name string, data []byte) []byte {
		if name == tuner {
			return tarGz(t, "rhythm-tuner", []byte("#!/bin/sh\necho evil\n"))
		}
		return data
	}, nil)
	airGap(t)
	dir := t.TempDir()
	code, out, errs := runCmd("install", "--from", "bad.tar.gz", "--locked", "--dir", dir)
	if code != 1 || !strings.Contains(errs, tuner+": sha256 differs from the locked one") || !strings.Contains(errs, "Nothing was installed") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("installed %v", entries)
	}
	// Without --locked the signed checksums.txt still catches it.
	code, _, errs = runCmd("install", "tuner", "--from", "bad.tar.gz", "--dir", dir)
	if code != 1 || !strings.Contains(errs, "checksum mismatch") {
		t.Errorf("unlocked: exit %d\n%s", code, errs)
	}
	_ = s
}

func TestBundleWithoutThisPlatform(t *testing.T) {
	lockedSandbox(t)
	if code, out, errs := runCmd("bundle", "--locked", "-o", "good.tar.gz"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	rewrite(t, "good.tar.gz", "other.tar.gz", nil, func(m *bundle.Manifest) { m.Platforms = []string{"plan9_amd64"} })
	airGap(t)
	code, _, errs := runCmd("install", "--from", "other.tar.gz", "--locked", "--dir", t.TempDir())
	want := "other.tar.gz was made for plan9_amd64 and has nothing for this machine (" + thisPlatform() +
		"); make one with: rhythm bundle --locked --platform " + strings.Replace(thisPlatform(), "_", "/", 1)
	if code != 1 || !strings.Contains(errs, want) {
		t.Errorf("exit %d\n%s\nwant %s", code, errs, want)
	}
}

func TestBundleRefusesUnsafeNames(t *testing.T) {
	signedSandbox(t)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: "../../evil", Mode: 0o755, Size: 4, Typeflag: tar.TypeReg})
	tw.Write([]byte("evil"))
	tw.Close()
	zw.Close()
	os.WriteFile("evil.tar.gz", buf.Bytes(), 0o600)
	code, _, errs := runCmd("install", "tuner", "--from", "evil.tar.gz", "--dir", t.TempDir())
	if code != 1 || !strings.Contains(errs, `unsafe entry name "../../evil"; refusing the bundle`) {
		t.Errorf("exit %d\n%s", code, errs)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "evil")); err == nil {
		t.Error("a file was written outside the bundle")
	}
}

func TestBundlePlatforms(t *testing.T) {
	lockedSandbox(t)
	// The lock pins this platform and three others. Only this one's
	// archive is published, so another platform fails closed.
	code, out, errs := runCmd("bundle", "--locked", "-o", "b.tar.gz", "--platform", "plan9/amd64")
	if code != 1 || !strings.Contains(errs, "rhythm.lock has no archive for plan9_amd64") || !strings.Contains(errs, "not written") {
		t.Errorf("unlocked platform: exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat("b.tar.gz"); err == nil {
		t.Error("a bundle was written")
	}
	got, err := parsePlatforms("linux/amd64, darwin/arm64,linux/amd64")
	if err != nil || strings.Join(got, " ") != "darwin_arm64 linux_amd64" {
		t.Errorf("parsePlatforms = %v, %v", got, err)
	}
	for _, bad := range []string{"", "linux", "linux_amd64", "Linux/AMD64", "a/b/c"} {
		if _, err := parsePlatforms(bad); err == nil {
			t.Errorf("parsePlatforms(%q) passed", bad)
		}
	}
	if archiveFor("rhythm-tuner", "windows_arm64") != "rhythm-tuner_windows_arm64.zip" || archiveFor("rhythm", "darwin_arm64") != "rhythm_darwin_arm64.tar.gz" {
		t.Error("archiveFor")
	}
}

func TestBundleUsage(t *testing.T) {
	pinSandbox(t)
	for _, c := range []struct{ args, want string }{
		{"-o x.tar.gz", "rhythm: bundle needs --locked"},
		{"--locked", "rhythm: bundle needs -o <file>"},
		{"--locked -o x tuner", "rhythm: bundle takes no names"},
		{"--locked -o x --platform linux", `rhythm: --platform: invalid platform "linux"`},
	} {
		code, out, errs := runCmd(append([]string{"bundle"}, strings.Fields(c.args)...)...)
		if code != exitUsage || !strings.Contains(errs, c.want) {
			t.Errorf("%s: exit %d\n%s%s", c.args, code, out, errs)
		}
	}
	// No lock: say how to make one.
	code, _, errs := runCmd("bundle", "--locked", "-o", "x.tar.gz")
	if code != 1 || !strings.Contains(errs, "write one with: rhythm lock") {
		t.Errorf("no lock: exit %d\n%s", code, errs)
	}
	if code, out, _ := runCmd("bundle", "--help"); code != 0 || !strings.Contains(out, "rhythm install --from rhythm-bundle.tar.gz --locked") {
		t.Errorf("help: exit %d\n%s", code, out)
	}
	// The short flag is shown the way it is typed.
	if _, out, _ := runCmd("bundle", "--help"); !strings.Contains(out, "\n  -o file              short for --output file\n") || strings.Contains(out, "--o ") {
		t.Errorf("help doesn't show -o file:\n%s", out)
	}
}

// A bundle only carries releases whose signature was checked.
func TestBundleNeedsSignatures(t *testing.T) {
	_, f := lockedSandbox(t)
	f.unsigned[repoOf("tuner")+"@0.3.1"] = true
	code, _, errs := runCmd("bundle", "--locked", "-o", "b.tar.gz")
	if code != 1 || !strings.Contains(errs, "signature check failed: the release has no checksums.txt.sigstore.json") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}
