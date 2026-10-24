// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/RhythmShahriar/rhythm/kit/sigverify"
	"github.com/RhythmShahriar/rhythm/kit/sigverify/sigverifytest"
)

// useCA makes f sign every release with a fresh test CA and makes rhythm
// trust that CA, with the same per-repository pinning as a real build.
func useCA(t *testing.T, f *releaseAPI) *sigverifytest.CA {
	t.Helper()
	ca, err := sigverifytest.New()
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.signer = ca
	f.mu.Unlock()
	old := newVerifier
	newVerifier = func() (signatureVerifier, error) { return buildVerifier(ca.Options()) }
	t.Cleanup(func() { newVerifier = old })
	return ca
}

// signedSandbox is pinSandbox with signed releases.
func signedSandbox(t *testing.T) (*sandbox, *releaseAPI) {
	t.Helper()
	s, f := pinSandbox(t)
	useCA(t, f)
	return s, f
}

const sharedSigner = sigverify.Workflow + "@" + sigverify.MasterRef

// pkgDir is this package's folder; sandboxes move the tests elsewhere.
var pkgDir, _ = os.Getwd()

func TestDefaultVerifierChecksTheRealRelease(t *testing.T) {
	t.Setenv("RHYTHM_OFFLINE", "1") // the embedded root; no TUF refresh
	sums, bundle := realFixture(t)
	v, err := defaultVerifier()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := v.VerifyChecksums(t.Context(), selfRepo, "0.1.0", sums, bundle)
	if err != nil || signer != sharedSigner {
		t.Fatalf("signer %q, err %v", signer, err)
	}
	// The same bundle doesn't pass for a tool, whatever the case.
	for _, repo := range []string{repoOf("tuner"), "rhythmshahriar/rhythm"} {
		if _, err := v.VerifyChecksums(t.Context(), repo, "0.1.0", sums, bundle); err == nil {
			t.Errorf("%s accepted rhythm's signature", repo)
		}
	}
}

func realFixture(t *testing.T) (sums, bundle []byte) {
	t.Helper()
	var err error
	if sums, err = os.ReadFile(filepath.Join(pkgDir, "../../kit/sigverify/testdata/rhythm_v0.1.0_checksums.txt")); err != nil {
		t.Fatal(err)
	}
	if bundle, err = os.ReadFile(filepath.Join(pkgDir, "../../kit/sigverify/testdata/rhythm_v0.1.0_checksums.txt.sigstore.json")); err != nil {
		t.Fatal(err)
	}
	return sums, bundle
}

// An extension's own workflow may sign for it. It never may for
// rhythm's own tools.
func TestVerifierPinsByRepository(t *testing.T) {
	ca, err := sigverifytest.New()
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildVerifier(ca.Options())
	if err != nil {
		t.Fatal(err)
	}
	sums := []byte("abc  x.tar.gz\n")
	for _, c := range []struct {
		repo string
		ok   bool
	}{
		{"someone/rhythm-zap", true},
		{repoOf("tuner"), false},
		{"rhythmshahriar/TUNER", false},
		{selfRepo, false},
	} {
		own := sigverifytest.Identity{SAN: "https://github.com/" + c.repo + "/.github/workflows/release.yml@refs/tags/v1.0.0", Repo: c.repo, Ref: "refs/tags/v1.0.0"}
		b, err := ca.Sign(own, sums)
		if err != nil {
			t.Fatal(err)
		}
		_, err = v.VerifyChecksums(t.Context(), c.repo, "1.0.0", sums, b)
		if (err == nil) != c.ok {
			t.Errorf("%s with its own workflow: err %v, want ok=%v", c.repo, err, c.ok)
		}
	}
}

func TestSignerHelpers(t *testing.T) {
	own := "https://github.com/someone/rhythm-zap/.github/workflows/release.yml"
	for san, want := range map[string]string{
		sharedSigner:              sharedSigner,
		own + "@refs/tags/v1.2.0": own,
		"me@example.com":          "me",
	} {
		if got := signerIdentity(san); got != want {
			t.Errorf("signerIdentity(%q) = %q, want %q", san, got, want)
		}
	}
	for _, c := range []struct {
		want, got string
		ok        bool
	}{
		{own, own + "@refs/tags/v1.3.0", true},
		{own + "@refs/heads/main", own + "@refs/heads/main", true},
		{own + "@refs/heads/main", own + "@refs/tags/v1.3.0", false},
		{own, "https://github.com/mallory/rhythm-zap/.github/workflows/release.yml@refs/tags/v1.3.0", false},
		{sharedSigner, sharedSigner, true},
	} {
		if got := signerMatches(c.want, c.got); got != c.ok {
			t.Errorf("signerMatches(%q, %q) = %v", c.want, c.got, got)
		}
	}
	if got := describeSigner(repoOf("tuner"), "0.3.2", sharedSigner); got != "RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)" {
		t.Errorf("describeSigner = %q", got)
	}
}

func TestInstallPrintsTheSigner(t *testing.T) {
	s, f := signedSandbox(t)
	code, out, errs := runCmd("install", "tuner", "--dir", s.bin)
	want := "Installed rhythm-tuner 0.4.0 to " + filepath.Join(s.bin, "rhythm-tuner") + "\n" +
		"  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.4.0 (release-go.yml)\n" +
		"  Checksum ok   sha256:" + f.sha(repoOf("tuner"), "0.4.0") + "\n"
	if code != 0 || errs != "" || out != want {
		t.Fatalf("exit %d\n%s%s\nwant\n%s", code, out, errs, want)
	}
	// rhythm list says so without the network. It stops once the file
	// changes.
	_, out, _ = runCmd("list", "--offline")
	if !strings.Contains(out, "tuner   0.4.0    yes     ") {
		t.Errorf("list:\n%s", out)
	}
	appendTo(t, filepath.Join(s.bin, "rhythm-tuner"), "# edited\n")
	_, out, _ = runCmd("list", "--offline")
	if !strings.Contains(out, "tuner   0.4.0    no      ") {
		t.Errorf("list after an edit:\n%s", out)
	}
	// --quiet keeps the Installed line only.
	code, out, _ = runCmd("install", "soundcheck", "--dir", s.bin, "--quiet")
	if code != 0 || strings.Contains(out, "Signature ok") || !strings.Contains(out, "Installed rhythm-soundcheck") {
		t.Errorf("--quiet: exit %d\n%s", code, out)
	}
}

func appendTo(t *testing.T, p, text string) {
	t.Helper()
	fh, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(fh, text)
	fh.Close()
}

func TestUpdatePrintsTheSigner(t *testing.T) {
	s, _ := signedSandbox(t)
	if code, out, errs := runCmd("install", "tuner@0.3.1", "--dir", s.bin); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	code, out, errs := runCmd("update", "tuner", "--no-data")
	if code != 0 || !strings.Contains(out, "rhythm-tuner       0.3.1 → 0.4.0 ✔\n  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.4.0 (release-go.yml)\n") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

// A checksums.txt changed after signing is refused. The binary that
// was there stays.
func TestEditedChecksumsFailClosed(t *testing.T) {
	s, f := signedSandbox(t)
	if code, out, errs := runCmd("install", "tuner@0.3.1", "--dir", s.bin); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	before := s.content("rhythm-tuner")
	f.editedSums[repoOf("tuner")+"@0.4.0"] = true

	code, out, errs := runCmd("update", "tuner", "--no-data")
	if code != 1 || !strings.Contains(errs, "refusing to install RhythmShahriar/tuner 0.4.0: signature check failed: signature not valid") ||
		!strings.Contains(errs, "nothing was changed. The release may have been tampered with") || strings.Contains(out, "✔") {
		t.Errorf("update: exit %d\n%s%s", code, out, errs)
	}
	code, out, errs = runCmd("install", "tuner@0.4.0", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "signature check failed") || strings.Contains(out, "Installed") {
		t.Errorf("install: exit %d\n%s%s", code, out, errs)
	}
	if s.content("rhythm-tuner") != before {
		t.Error("the old binary was replaced")
	}
	if n := f.count("/dl/" + repoOf("tuner") + "/0.4.0"); n != 0 {
		t.Errorf("the archive was downloaded %d times before the signature passed", n)
	}
}

func TestMissingBundleFailsClosed(t *testing.T) {
	s, f := signedSandbox(t)
	f.unsigned[repoOf("tuner")+"@0.4.0"] = true
	code, out, errs := runCmd("install", "tuner", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "signature check failed: the release has no checksums.txt.sigstore.json") ||
		!strings.Contains(errs, "rerun with --skip-signature") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(s.bin, "rhythm-tuner")); err == nil {
		t.Error("installed without a signature")
	}

	// --skip-signature says what it gives up and installs.
	code, out, errs = runCmd("install", "tuner", "--dir", s.bin, "--skip-signature")
	if code != 0 || !strings.Contains(out, "Installed rhythm-tuner 0.4.0") || strings.Contains(out, "Signature ok") ||
		!strings.HasPrefix(errs, "warning: --skip-signature: release signatures aren't checked") {
		t.Errorf("--skip-signature: exit %d\n%s%s", code, out, errs)
	}
	_, out, _ = runCmd("list", "--offline")
	if !strings.Contains(out, "tuner   0.4.0    no      ") {
		t.Errorf("list:\n%s", out)
	}
}

// A release signed by anyone but the shared workflow is refused for a
// registry tool, even when the signature itself is sound.
func TestOtherSignerRefused(t *testing.T) {
	s, f := signedSandbox(t)
	f.signAs = func(repo, v string) sigverifytest.Identity {
		return sigverifytest.Identity{SAN: "https://github.com/" + repo + "/.github/workflows/release.yml@refs/tags/v" + v, Repo: repo, Ref: "refs/tags/v" + v}
	}
	code, out, errs := runCmd("install", "tuner", "--dir", s.bin)
	if code != 1 || !strings.Contains(errs, "signature check failed") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestVerify(t *testing.T) {
	s, f := signedSandbox(t)
	if code, out, errs := runCmd("install", "tuner", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	code, out, errs := runCmd("verify", "tuner")
	sha := f.sha(repoOf("tuner"), "0.4.0")
	want := "rhythm-tuner 0.4.0 (" + filepath.Join(s.bin, "rhythm-tuner") + ")\n" +
		"  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.4.0 (release-go.yml)\n" +
		"  Checksum ok   sha256:" + sha + "\n" +
		"  Binary ok     same bytes as in rhythm-tuner_" + thisPlatform() + ".tar.gz\n"
	if code != 0 || errs != "" || out != want {
		t.Fatalf("exit %d\n%s%s\nwant\n%s", code, out, errs, want)
	}

	// A binary changed after install fails, with the way back.
	appendTo(t, filepath.Join(s.bin, "rhythm-tuner"), "# tampered\n")
	code, _, errs = runCmd("verify", "tuner")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner       the installed binary differs from the one in the signed rhythm-tuner_") ||
		!strings.Contains(errs, "Reinstall it with: rhythm install tuner --force") {
		t.Errorf("tampered: exit %d\n%s", code, errs)
	}

	// The archive comes from the download cache when it is there: the
	// cache holds the right bytes, the release server now serves others.
	if code, out, errs := runCmd("install", "tuner", "--dir", s.bin, "--force", "--cache", "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	f.tampered[repoOf("tuner")+"@0.4.0/keep-sums"] = true
	f.tampered[repoOf("tuner")+"@0.4.0"] = true
	dl := f.count("/dl/" + repoOf("tuner") + "/0.4.0")
	code, out, errs = runCmd("verify", "tuner", "--json")
	if code != 0 || !strings.Contains(out, `"cached": true`) {
		t.Errorf("cached: exit %d\n%s%s", code, out, errs)
	}
	if f.count("/dl/"+repoOf("tuner")+"/0.4.0") != dl {
		t.Error("verify downloaded an archive the cache had")
	}
}

// A release archive that doesn't match its signed checksums.txt fails
// verify too.
func TestVerifyTamperedArchive(t *testing.T) {
	s, f := signedSandbox(t)
	if code, out, errs := runCmd("install", "tuner", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	f.tampered[repoOf("tuner")+"@0.4.0/keep-sums"] = true
	f.tampered[repoOf("tuner")+"@0.4.0"] = true
	code, _, errs := runCmd("verify", "tuner")
	if code != 1 || !strings.Contains(errs, "doesn't match the signed checksums.txt") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}

func TestVerifyRhythmItselfAndJSON(t *testing.T) {
	s, _ := signedSandbox(t)
	if code, out, errs := runCmd("install", "tuner", "--dir", s.bin, "--quiet"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	// What the installers run after installing rhythm.
	code, out, errs := runCmd("verify", "rhythm")
	if code != 0 || !strings.Contains(out, "  Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (release-go.yml)\n") {
		t.Errorf("verify rhythm: exit %d\n%s%s", code, out, errs)
	}

	code, out, errs = runCmd("verify", "--json")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	hexes := regexp.MustCompile(`[0-9a-f]{64}`)
	t.Chdir(pkgDir)
	golden(t, "verify.golden.json", hexes.ReplaceAllString(out, "<sha256>"), s.dir, "$SANDBOX", thisPlatform(), "$PLATFORM")
	var rep verifyReport
	objs := decodeStrict(t, out, &rep, "targets")
	if rep.Schema != "rhythm-verify/v1" || len(rep.Targets) != 2 || !rep.Targets[0].OK || !rep.Targets[1].OK {
		t.Errorf("%+v", rep)
	}
	requireKeys(t, objs, "name", "version", "path", "repo", "signer", "source_ref", "archive", "archive_sha256", "binary_sha256", "cached", "ok", "error")
}

func TestVerifyFailures(t *testing.T) {
	s, f := signedSandbox(t)
	// Not installed, then a development build.
	s.stub(t, "rhythm-echo", "dev", "")
	code, out, errs := runCmd("verify", "tuner", "echo")
	if code != 1 || !strings.Contains(errs, "rhythm-tuner       isn't installed; install it with: rhythm install tuner") ||
		!strings.Contains(errs, "rhythm-echo        a development build; only released binaries can be verified") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	// An unknown name is a usage error.
	if code, _, errs := runCmd("verify", "tunr"); code != 2 || !strings.Contains(errs, "unknown tool \"tunr\"\n  did you mean: rhythm verify tuner\n") {
		t.Errorf("exit %d: %s", code, errs)
	}
	// A missing bundle fails verify and says so.
	f.unsigned[selfRepo+"@0.7.0"] = true
	code, _, errs = runCmd("verify", "rhythm")
	if code != 1 || !strings.Contains(errs, "can't vouch for RhythmShahriar/rhythm 0.7.0: signature check failed: the release has no checksums.txt.sigstore.json") ||
		strings.Contains(errs, "--skip-signature") {
		t.Errorf("exit %d: %s", code, errs)
	}
	// An unreachable API is said once.
	t.Setenv("RHYTHM_GITHUB_API", closedPort(t))
	code, _, errs = runCmd("verify", "rhythm")
	if code != 1 || strings.Count(errs, "can't reach") != 1 {
		t.Errorf("exit %d: %s", code, errs)
	}
}

// The real v0.1.0 bundle verifies through rhythm verify with the root
// built into rhythm. The archive served here isn't the real one, so the
// checksum step then fails, closed.
func TestVerifyRealBundle(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the embedded Sigstore root")
	}
	sums, bundle := realFixture(t)
	archive := "rhythm_" + thisPlatform() + ".tar.gz"
	if thisPlatform() == "windows_amd64" || thisPlatform() == "windows_arm64" {
		archive = "rhythm_" + thisPlatform() + ".zip"
	}
	if !strings.Contains(string(sums), archive) {
		t.Skip("v0.1.0 has no", archive)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/repos/" + selfRepo + "/releases/tags/v0.1.0":
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.0", "assets": []map[string]string{
				{"name": archive, "browser_download_url": base + "/dl/archive"},
				{"name": "checksums.txt", "browser_download_url": base + "/dl/sums"},
				{"name": signatureAsset, "browser_download_url": base + "/dl/bundle"},
			}})
		case "/dl/archive":
			_, _ = w.Write([]byte("not the real archive"))
		case "/dl/sums":
			_, _ = w.Write(sums)
		case "/dl/bundle":
			_, _ = w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s := newSandbox(t, nil, nil)
	t.Setenv("RHYTHM_GITHUB_API", srv.URL)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")
	old := newVerifier
	newVerifier = func() (signatureVerifier, error) { return buildVerifier(sigverify.Options{Offline: true}) }
	t.Cleanup(func() { newVerifier = old })

	code, out, errs := runCmd("verify", "rhythm")
	if code != 1 || !strings.Contains(out, "  Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.1.0 (release-go.yml)\n") ||
		!strings.Contains(errs, archive+" doesn't match the signed checksums.txt") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}

func TestVerifyHelp(t *testing.T) {
	code, out, errs := runCmd("verify", "--help")
	if code != 0 || errs != "" || !strings.Contains(out, "Usage: rhythm verify [tool...] [flags]") || !strings.Contains(out, "--json") {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
}
