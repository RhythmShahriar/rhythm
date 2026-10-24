// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/config"
	"github.com/RhythmShahriar/rhythm/kit/sigverify"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Signatures are checked by default: every install, update, lock and
// bundle refuses a release whose checksums.txt doesn't carry a valid
// Sigstore signature from its project's release workflow.
func init() {
	newVerifier = defaultVerifier
	signedStatus = recordedStatus
	register(command{name: "verify", args: "[tool...] [--json]", summary: "check installed binaries against their signed releases", run: cmdVerify})
}

// debugEnv, when set, makes rhythm say which Sigstore root it used.
const debugEnv = "RHYTHM_DEBUG"

// defaultVerifier is the signature check of this build: the public-good
// Sigstore instance, refreshed through TUF unless RHYTHM_OFFLINE is set
// or the releases come from a bundle (then the root built into rhythm is
// used, so nothing goes to the network).
func defaultVerifier() (signatureVerifier, error) {
	return buildVerifier(verifierOptions())
}

// verifierOptions are defaultVerifier's settings for this run.
func verifierOptions() sigverify.Options {
	return sigverify.Options{
		Offline: offlineOn(os.Getenv(offlineEnv)) || thisRun.bundle != "",
		OnFallback: func(err error) {
			if os.Getenv(debugEnv) != "" {
				fmt.Fprintf(os.Stderr, "rhythm: using the built-in Sigstore root: %v\n", err)
			}
		},
	}
}

// buildVerifier pins each release to who may sign it. rhythm and the
// registry's tools must be signed by the shared release workflow
// building that repository's tag; an extension may also be signed by a
// workflow of its own repository. base holds the rest of the settings.
func buildVerifier(base sigverify.Options) (signatureVerifier, error) {
	official, err := sigverify.New(base)
	if err != nil {
		return nil, err
	}
	base.OwnWorkflow = true
	ext, err := sigverify.New(base)
	if err != nil {
		return nil, err
	}
	return &pinnedVerifier{official: official, extension: ext}, nil
}

// pinnedVerifier picks the signer policy by repository.
type pinnedVerifier struct {
	official, extension *sigverify.Verifier
}

// VerifyChecksums checks bundle over checksums for repo's version, with
// the policy repo gets: the shared workflow only for rhythm's own repos.
func (p *pinnedVerifier) VerifyChecksums(ctx context.Context, repo, version string, checksums, bundle []byte) (string, error) {
	if officialRepo(repo) {
		return p.official.VerifyChecksums(ctx, repo, version, checksums, bundle)
	}
	return p.extension.VerifyChecksums(ctx, repo, version, checksums, bundle)
}

// officialRepo reports whether repo is rhythm's or a registry tool's.
// Case is ignored, so a differently written name can't pick the looser
// extension policy.
func officialRepo(repo string) bool {
	if strings.EqualFold(repo, selfRepo) {
		return true
	}
	for _, t := range registry {
		if strings.EqualFold(repo, t.Repo) {
			return true
		}
	}
	return false
}

// signatureHint follows a failed signature check: what it means and the
// way out for someone who knows why.
const signatureHint = "; nothing was changed. The release may have been tampered with, or a mirror re-packed it. " +
	"Only if you trust where it comes from, rerun with --skip-signature"

// skipWarning is what --skip-signature prints before anything happens.
const skipWarning = "warning: --skip-signature: release signatures aren't checked, so a tampered release would install; " +
	"use it only with a mirror you trust"

// verifyHint is rhythm verify's version of signatureHint.
const verifyHint = "; the release may have been tampered with, or a mirror re-packed it"

// signerIdentity is what rhythm records as a release's signer: the
// shared workflow with its ref (always refs/heads/master), or another
// workflow without its ref, which changes with every tag it runs on.
func signerIdentity(san string) string {
	if strings.HasPrefix(san, sigverify.Workflow+"@") {
		return san
	}
	if i := strings.LastIndex(san, "@"); i > 0 {
		return san[:i]
	}
	return san
}

// signerMatches reports whether got, a signer a verifier returned, is
// the recorded signer want: the same identity, or the same workflow at
// another ref when want carries no ref.
func signerMatches(want, got string) bool {
	return want == got || (!strings.Contains(want, "@") && signerIdentity(got) == want)
}

// describeSigner words a verified release for people, such as
// "RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)": the
// repository and tag the signature ties it to and the workflow file
// that signed it.
func describeSigner(repo, version, signer string) string {
	workflow := signer
	if i := strings.LastIndex(workflow, "@"); i > 0 {
		workflow = workflow[:i]
	}
	return fmt.Sprintf("%s @ refs/tags/v%s (%s)", repo, strings.TrimPrefix(version, "v"), path.Base(workflow))
}

// trustLabel aligns the Signature ok, Checksum ok and Binary ok lines.
const trustLabel = "  %-14s%s\n"

// reportTrust says how the release u just installed at dest was checked
// and records it for rhythm list, when its signature was checked. quiet
// leaves the lines out.
func reportTrust(w io.Writer, u *update.Updater, rel *update.Release, dest string, quiet bool) {
	t, ok := u.HTTP.Transport.(*explainTransport)
	if !ok {
		return
	}
	signer, sums := t.verifiedBy, t.verifiedSums
	if t.locked != nil {
		signer, sums = t.locked.signer, t.locked.sums
	}
	if signer == "" {
		return
	}
	if !quiet {
		fmt.Fprintf(w, trustLabel, "Signature ok", describeSigner(u.Repo, rel.Version, signer))
		if sum := parseChecksums(sums)[u.ArchiveName()]; sum != "" {
			fmt.Fprintf(w, trustLabel, "Checksum ok", "sha256:"+sum)
		}
	}
	_ = recordVerified(dest, u.Repo, rel.Version, signer)
}

// The verified record: which binaries passed a signature check, by path
// and sha256, so rhythm list can say "yes" without the network.

// verifiedFile is the record's name in rhythm's config folder.
const verifiedFile = "verified.json"

type verifiedDoc struct {
	Schema   string                   `json:"schema"` // rhythm-verified/v1
	Binaries map[string]verifiedEntry `json:"binaries"`
}

type verifiedEntry struct {
	SHA256     string    `json:"sha256"` // of the binary file
	Repo       string    `json:"repo"`
	Version    string    `json:"version"`
	Signer     string    `json:"signer"`
	VerifiedAt time.Time `json:"verified_at"`
}

func verifiedPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, verifiedFile), nil
}

func readVerified() verifiedDoc {
	doc := verifiedDoc{Schema: "rhythm-verified/v1", Binaries: map[string]verifiedEntry{}}
	p, err := verifiedPath()
	if err != nil {
		return doc
	}
	data, err := os.ReadFile(p) //nolint:gosec // rhythm's own config file
	if err != nil {
		return doc
	}
	var got verifiedDoc
	if json.Unmarshal(data, &got) == nil && got.Schema == doc.Schema && got.Binaries != nil {
		return got
	}
	return doc
}

// recordVerified notes that the binary now at dest is version of repo,
// signed by signer. Entries for binaries that are gone are dropped.
func recordVerified(dest, repo, version, signer string) error {
	sum, err := fileSHA256(dest)
	if err != nil {
		return err
	}
	doc := readVerified()
	for p := range doc.Binaries {
		if _, err := os.Stat(p); err != nil {
			delete(doc.Binaries, p)
		}
	}
	doc.Binaries[resolve(dest)] = verifiedEntry{SHA256: sum, Repo: repo, Version: version, Signer: signer, VerifiedAt: time.Now().UTC()}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	p, err := verifiedPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".verified-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, werr := tmp.Write(append(data, '\n'))
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		return errors.Join(werr, cerr)
	}
	return os.Rename(tmp.Name(), p)
}

// recordedStatus is the SIGNED column: "yes" when the binary at p is
// byte for byte one that passed a signature check, "no" otherwise.
func recordedStatus(p string) string {
	e, ok := readVerified().Binaries[resolve(p)]
	if !ok {
		return "-" // never checked, as with binaries an older rhythm installed
	}
	if sum, err := fileSHA256(p); err != nil || sum != e.SHA256 {
		return "no"
	}
	return "yes"
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p) //nolint:gosec // an installed binary rhythm looks after
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

const verifySynopsis = `Usage: rhythm verify [tool...] [flags]

Checks that each installed binary is the one its release published. The
release's checksums.txt must carry a valid Sigstore signature from the
project's release workflow for that version, the archive must match
checksums.txt and the binary in it must match the installed file byte
for byte. Name tools, or rhythm itself, to check only those; with none,
rhythm and every installed tool and extension are checked.

The archive comes from the download cache when it is there (see rhythm
install --cache), so only checksums.txt and its signature are fetched.

Exit status: 0 when every binary checked out, 1 when any didn't or
couldn't be checked, 2 for a usage error and 130 when interrupted.`

// verifyReport is rhythm verify --json's answer.
type verifyReport struct {
	Schema  string         `json:"schema"` // rhythm-verify/v1
	Targets []verifyTarget `json:"targets"`
}

// verifyTarget is one binary in the rhythm-verify/v1 report. The hashes
// and signer are empty when the check stopped before them; Error then
// says why.
type verifyTarget struct {
	Name          string `json:"name"` // rhythm or rhythm-<tool>
	Version       string `json:"version"`
	Path          string `json:"path"`
	Repo          string `json:"repo"`
	Signer        string `json:"signer"`     // the workflow identity that signed checksums.txt
	SourceRef     string `json:"source_ref"` // the tag the signature ties it to
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	BinarySHA256  string `json:"binary_sha256"`
	Cached        bool   `json:"cached"` // the archive came from the download cache
	OK            bool   `json:"ok"`
	Error         string `json:"error"`
}

func cmdVerify(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("verify", verifySynopsis)
	asJSON := flags.Bool("json", false, "print the report as rhythm-verify/v1 JSON")
	names, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	exts, extErr := readExtensions()
	if extErr != nil {
		fmt.Fprintf(stderr, "rhythm: %v; extensions not checked\n", extErr)
	}
	want, code := updateScope("rhythm verify", names, false, exts, stderr)
	if code != 0 {
		return code
	}
	run := &runOptions{}
	if dir, err := config.CacheDir(selfName); err == nil {
		run.cacheDir = filepath.Join(dir, "downloads")
	}
	defer beginRun(run)()
	human := stdout
	if *asJSON {
		human = io.Discard // stdout carries the JSON alone; problems still go to stderr
	}

	ctx, stop := signalContext()
	defer stop()
	targets, missing := updateTargets(ctx, want, exts)
	rep := verifyReport{Schema: "rhythm-verify/v1", Targets: []verifyTarget{}}
	var nets netReport
	failed := extErr != nil
	for _, tool := range missing {
		msg := "isn't installed; install it with: rhythm install " + tool
		if _, registry := toolRepo(tool); !registry {
			msg = "isn't on PATH"
		}
		fmt.Fprintf(stderr, "%-18s %s\n", "rhythm-"+tool, msg)
		rep.Targets = append(rep.Targets, verifyTarget{Name: "rhythm-" + tool, Error: msg})
		failed = true
	}
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		res := verifyTarget{Name: t.binary, Version: t.current, Path: t.path, Repo: t.repo}
		err := verifyOne(ctx, t, &res, &nets, human)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			failed = true
			res.Error = describe(err)
			if !errors.Is(err, errSkipped) && !nets.add(t.binary, err) {
				fmt.Fprintf(stderr, "%-18s %s\n", t.binary, res.Error)
			}
		} else {
			res.OK = true
		}
		rep.Targets = append(rep.Targets, res)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "rhythm: verify interrupted")
		return exitInterrupted
	}
	nets.print(stderr)
	if *asJSON {
		for i := range rep.Targets {
			if msg := nets.explain(rep.Targets[i].Name); msg != "" {
				rep.Targets[i].Error = msg
			}
		}
		if err := writeJSON(stdout, rep); err != nil {
			fmt.Fprintln(stderr, "rhythm:", err)
			return 1
		}
	}
	if failed {
		return 1
	}
	return 0
}

// errNotRelease is verifyOne's answer for a binary that isn't a release.
var errNotRelease = errors.New("a development build; only released binaries can be verified")

// binaryMismatchError is an installed binary that isn't the one in its
// signed release.
type binaryMismatchError struct {
	tool, archive string
}

func (e *binaryMismatchError) Error() string {
	fix := "rhythm install " + e.tool + " --force"
	if e.tool == selfName {
		fix = "the install script (see https://rythm.sh)"
	}
	return fmt.Sprintf("the installed binary differs from the one in the signed %s; it may have been changed after install. Reinstall it with: %s", e.archive, fix)
}

// archiveMismatchError is a release archive that doesn't match its own
// signed checksums.txt.
type archiveMismatchError struct{ archive, want, got string }

func (e *archiveMismatchError) Error() string {
	return fmt.Sprintf("%s doesn't match the signed checksums.txt (sha256 %s, expected %s); the release may have been tampered with", e.archive, e.got, e.want)
}

// verifyOne checks t against its signed release and fills res. It
// writes the ok lines to w as each check passes.
func verifyOne(ctx context.Context, t target, res *verifyTarget, nets *netReport, w io.Writer) error {
	switch {
	case t.current == unknownVersion:
		res.Version = ""
		return fmt.Errorf("version unknown (%s), can't verify it", t.probe)
	case !update.IsRelease(t.current):
		return errNotRelease
	case nets.skip(t.binary, os.Getenv("RHYTHM_GITHUB_API")):
		return errSkipped
	}
	u := updater(t.repo, t.binary, t.current)
	et, _ := u.HTTP.Transport.(*explainTransport)
	if et != nil {
		et.signer, et.verifying = t.signer, true
	}
	v, err := thisRun.activeVerifier()
	switch {
	case err != nil:
		return fmt.Errorf("signature check unavailable: %w", err)
	case v == nil:
		return errors.New("this rhythm can't check signatures")
	}
	rel, err := resolveSpec(ctx, u, t.current, 0)
	if err != nil {
		return err
	}
	sums, err := verifiedChecksums(ctx, u, rel.Release)
	if err != nil {
		return err
	}
	if et != nil {
		res.Signer = et.verifiedBy
	}
	res.SourceRef = "refs/tags/v" + rel.Version
	fmt.Fprintf(w, "%s %s (%s)\n", t.binary, rel.Version, t.path)
	fmt.Fprintf(w, trustLabel, "Signature ok", describeSigner(u.Repo, rel.Version, res.Signer))

	name := u.ArchiveName()
	res.Archive = name
	want := parseChecksums(sums)[name]
	if want == "" {
		return fmt.Errorf("the signed checksums.txt of %s %s lists no %s", u.Repo, rel.Version, name)
	}
	archive := thisRun.cacheLookup(want, name)
	res.Cached = archive != nil
	if archive == nil {
		archiveURL, ok := rel.Assets[name]
		if !ok {
			return fmt.Errorf("release %s has no %s", rel.Version, name)
		}
		if archive, err = releaseFile(ctx, u, archiveURL, name, maxArchive); err != nil {
			return err
		}
		sum := sha256.Sum256(archive)
		if got := hex.EncodeToString(sum[:]); got != want {
			return &archiveMismatchError{archive: name, want: want, got: got}
		}
		if cacheFromEnv() {
			thisRun.cacheStore(want, name, archive)
		}
	}
	res.ArchiveSHA256 = want
	fmt.Fprintf(w, trustLabel, "Checksum ok", "sha256:"+want)

	inside, err := lockedBinary(archive, exe(t.binary))
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	installed, err := os.ReadFile(t.real)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(installed)
	res.BinarySHA256 = hex.EncodeToString(sum[:])
	if !bytes.Equal(installed, inside) {
		return &binaryMismatchError{tool: t.tool, archive: name}
	}
	fmt.Fprintf(w, trustLabel, "Binary ok", "same bytes as in "+name)
	_ = recordVerified(t.real, u.Repo, rel.Version, res.Signer)
	return nil
}

// errSkipped stands for a binary not checked because an earlier one
// found the release API out of reach; netReport says why.
var errSkipped = errors.New("skipped: the release API can't be reached")
