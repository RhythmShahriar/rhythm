// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/RhythmShahriar/rhythm/internal/bundle"
	"github.com/RhythmShahriar/rhythm/kit/update"
)

// init registers bundle and gives install its bundle reader (--from).
func init() {
	openBundle = openBundleSource
	register(command{name: "bundle", args: "--locked -o <file> [--platform os/arch,...]", summary: "pack what rhythm.lock pins into one file for offline installs", run: cmdBundle})
}

const bundleSynopsis = `Usage: rhythm bundle --locked -o <file> [flags]

Packs everything rhythm.lock pins into one file for networks that can't
reach GitHub: each release's archive for the platforms asked for, its
checksums.txt and the Sigstore signature over it. Every signature and
hash is checked here first, against the release and against the lock. On
the machine without the network, install from it with:

  rhythm install --from rhythm-bundle.tar.gz --locked

which checks the signatures again with the Sigstore root built into
rhythm and every archive against rhythm.lock, with no network at all.

--platform takes os/arch pairs separated by commas, such as
linux/amd64,windows/amd64 (default this machine's). rhythm.lock must pin
an archive for each.

Exit status: 0 when the bundle was written, 1 when any release couldn't
be bundled (nothing is written then), 2 for a usage error and 130 when
interrupted.`

// platformRE is one os/arch pair of --platform.
var platformRE = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)

// parsePlatforms turns "linux/amd64,darwin/arm64" into rhythm.lock's
// os_arch keys, sorted and without repeats.
func parsePlatforms(s string) ([]string, error) {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !platformRE.MatchString(p) {
			return nil, fmt.Errorf("invalid platform %q; use os/arch, such as linux/amd64", p)
		}
		out = append(out, strings.Replace(p, "/", "_", 1))
	}
	if len(out) == 0 {
		return nil, errors.New("no platform given; use os/arch, such as linux/amd64")
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// archiveFor is binary's release archive for an os_arch platform.
func archiveFor(binary, platform string) string {
	if strings.HasPrefix(platform, "windows_") {
		return binary + "_" + platform + ".zip"
	}
	return binary + "_" + platform + ".tar.gz"
}

// cmdBundle is rhythm bundle. Every release is checked as it is added
// and the bundle is built under a temporary name next to the output, so
// a failed or interrupted run leaves no bundle and never replaces an
// older one.
func cmdBundle(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("bundle", bundleSynopsis)
	locked := flags.Bool("locked", false, "bundle the versions in rhythm.lock (required)")
	out := flags.String("output", "", "write the bundle to `file`, such as rhythm-bundle.tar.gz")
	flags.StringVar(out, "o", "", "short for --output `file`")
	platformFlag := flags.String("platform", strings.Replace(thisPlatform(), "_", "/", 1), "bundle archives for these `os/arch` pairs, separated by commas")
	pos, code, ok := parseArgs(flags, args, stdout, stderr)
	if !ok {
		return code
	}
	switch {
	case len(pos) > 0:
		fmt.Fprintln(stderr, "rhythm: bundle takes no names; it bundles what rhythm.lock pins")
		return exitUsage
	case !*locked:
		fmt.Fprintln(stderr, "rhythm: bundle needs --locked: it packs the versions rhythm.lock pins (write one with: rhythm lock)")
		return exitUsage
	case *out == "":
		fmt.Fprintln(stderr, "rhythm: bundle needs -o <file> (or --output), such as -o rhythm-bundle.tar.gz")
		return exitUsage
	}
	platforms, err := parsePlatforms(*platformFlag)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm: --platform:", err)
		return exitUsage
	}
	dest, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	lock, lockPath, list, code := lockedScope(nil, stdout, stderr)
	if code != 0 {
		return code
	}
	fmt.Fprintln(stdout, "Using", lockPath)
	lockData, err := lock.Marshal()
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	entries := make([]*lockEntry, 0, len(list)+1)
	if e := lock.Rhythm; e != nil {
		// lockedScope checks the tools' repos; rhythm's own is checked
		// here, so a cloned lock can't slip another router into a bundle.
		if !strings.EqualFold(e.Repo, selfRepo) {
			fmt.Fprintf(stderr, "rhythm: %s says rhythm comes from %s; rhythm is released from %s\n", lockFileName, e.Repo, selfRepo)
			return 1
		}
		entries = append(entries, e)
	}
	for _, it := range list {
		entries = append(entries, lock.Entry(it.tool))
	}

	defer beginRun(&runOptions{})() // signatures checked, built once
	ctx, stop := signalContext()
	defer stop()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".rhythm-bundle-*")
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	w := bundle.NewWriter(tmp)
	m := bundle.Manifest{Rhythm: version, Platforms: platforms, Lock: lockData}
	var nets netReport
	failed := false
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		binary := selfName
		if e.Name != selfName {
			binary = "rhythm-" + e.Name
		}
		if nets.skip(binary, os.Getenv("RHYTHM_GITHUB_API")) {
			failed = true
			continue
		}
		rel, err := bundleRelease(ctx, w, e, binary, platforms)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			if !nets.add(binary, err) {
				fmt.Fprintf(stderr, "%-18s %s\n", binary, describe(err))
			}
			failed = true
			continue
		}
		m.Releases = append(m.Releases, rel)
		fmt.Fprintf(stdout, "%-18s %s  %s\n", binary, e.Version, strings.Join(platforms, ", "))
	}
	if ctx.Err() != nil {
		_ = tmp.Close()
		fmt.Fprintln(stderr, "rhythm: bundle interrupted")
		return exitInterrupted
	}
	nets.print(stderr)
	if failed {
		_ = tmp.Close()
		fmt.Fprintf(stderr, "rhythm: %s not written\n", dest)
		return 1
	}
	err = w.Close(m)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		// CreateTemp makes the file 0600.
		err = os.Chmod(tmp.Name(), 0o644) //nolint:gosec // a bundle is meant to be copied around
	}
	if err == nil {
		err = os.Rename(tmp.Name(), dest)
	}
	if err != nil {
		fmt.Fprintln(stderr, "rhythm:", err)
		return 1
	}
	size := ""
	if fi, err := os.Stat(dest); err == nil {
		size = fmt.Sprintf(" (%.1f MB)", float64(fi.Size())/(1<<20))
	}
	fmt.Fprintf(stdout, "Wrote %s%s\n", dest, size)
	return 0
}

// bundleRelease adds e's locked release to w: checksums.txt and its
// signature (both checked) and the archive of every platform, each
// checked against the lock and checksums.txt.
func bundleRelease(ctx context.Context, w *bundle.Writer, e *lockEntry, binary string, platforms []string) (bundle.Release, error) {
	out := bundle.Release{Name: e.Name, Repo: e.Repo, Version: e.Version, Assets: map[string]bundle.File{}}
	for _, p := range platforms {
		if _, ok := e.Platforms[p]; !ok {
			return out, fmt.Errorf("%s: %s has no archive for %s (it locks %s); add it by running rhythm lock where all of them are released",
				e.Version, lockFileName, p, platformList(e.Platforms))
		}
	}
	u := updater(e.Repo, binary, "")
	et, _ := u.HTTP.Transport.(*explainTransport)
	if et != nil {
		et.signer = e.Signer
	}
	rel, err := resolveSpec(ctx, u, e.Version, 0)
	if err != nil {
		return out, err
	}
	sums, err := verifiedChecksums(ctx, u, rel.Release)
	if err != nil {
		return out, err
	}
	if et == nil || et.verifiedBy == "" {
		return out, fmt.Errorf("the signature of %s %s wasn't checked; a bundle only carries signed releases", e.Repo, e.Version)
	}
	dir := bundle.ReleaseDir(e.Repo, e.Version)
	add := func(asset string, data []byte) error {
		f, err := w.Add(dir+"/"+asset, data)
		if err == nil {
			out.Assets[asset] = f
		}
		return err
	}
	if err := add("checksums.txt", sums); err != nil {
		return out, err
	}
	sigURL, ok := rel.Assets[signatureAsset]
	if !ok {
		return out, fmt.Errorf("release %s has no %s", e.Version, signatureAsset)
	}
	sig, err := releaseFile(ctx, u, sigURL, signatureAsset, 1<<20)
	if err != nil {
		return out, err
	}
	if err := add(signatureAsset, sig); err != nil {
		return out, err
	}
	listed := parseChecksums(sums)
	for _, p := range platforms {
		name := archiveFor(binary, p)
		want := strings.ToLower(e.Platforms[p])
		if listed[name] != want {
			return out, &lockedMismatchError{archive: name}
		}
		archiveURL, ok := rel.Assets[name]
		if !ok {
			return out, fmt.Errorf("release %s has no %s", e.Version, name)
		}
		data, err := releaseFile(ctx, u, archiveURL, name, maxArchive)
		if err != nil {
			return out, err
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want {
			return out, &lockedMismatchError{archive: name}
		}
		if err := add(name, data); err != nil {
			return out, err
		}
	}
	return out, nil
}

// openedBundles keeps each bundle opened in this run, so install --from
// reads and checks the file once, not once per tool.
var openedBundles = struct {
	sync.Mutex
	m map[string]*bundle.Bundle
}{m: map[string]*bundle.Bundle{}}

// openBundleSource is openBundle: repo's releases in the bundle at p. A
// bundle without this machine's platform says which it has and how to
// make one that fits.
func openBundleSource(p, repo string) (releaseSource, error) {
	openedBundles.Lock()
	b, ok := openedBundles.m[p]
	if !ok {
		var err error
		if b, err = bundle.Open(p); err != nil {
			openedBundles.Unlock()
			return nil, err
		}
		openedBundles.m[p] = b
	}
	openedBundles.Unlock()
	if !b.HasPlatform(thisPlatform()) {
		return nil, fmt.Errorf("%s was made for %s and has nothing for this machine (%s); make one with: rhythm bundle --locked --platform %s",
			p, strings.Join(b.Manifest().Platforms, ", "), thisPlatform(), strings.Replace(thisPlatform(), "_", "/", 1))
	}
	src, err := b.Source(repo)
	if err != nil {
		return nil, err
	}
	return src, nil
}

// The bundle reader is a release source for kit/update too.
var _ update.Source = (*bundle.Source)(nil)

// routedSource is the Source rhythm gives kit/update for install --from.
// Its assets come through explainTransport, which reads them from the
// bundle; kit/update would refuse them in offline mode if they looked
// like GitHub downloads, yet a bundle never leaves the machine.
type routedSource struct {
	t      *explainTransport
	client *http.Client
}

// Latest returns the bundle's release of the repository, with asset
// URLs on bundleHost.
func (s routedSource) Latest(ctx context.Context) (*update.Release, error) {
	rel, err := s.t.fromSourceSpec(ctx, "", true)
	if err != nil {
		return nil, err
	}
	return rel.Release, nil
}

// ByTag returns the bundle's release v, with asset URLs on bundleHost.
func (s routedSource) ByTag(ctx context.Context, v string) (*update.Release, error) {
	rel, err := s.t.fromSourceSpec(ctx, v, false)
	if err != nil {
		return nil, err
	}
	return rel.Release, nil
}

// Open fetches asset through the client, so explainTransport serves it
// from the bundle with the same pinning and progress as a download.
func (s routedSource) Open(ctx context.Context, rel *update.Release, asset string) (io.ReadCloser, int64, error) {
	raw, ok := rel.Assets[asset]
	if !ok {
		return nil, 0, fmt.Errorf("release %s has no %s", rel.Version, asset)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil) //nolint:gosec // a bundle locator; the transport serves it from the file
	if err != nil {
		return nil, 0, err
	}
	resp, err := s.client.Do(req) //nolint:gosec // answered by explainTransport from the bundle, never the network
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, 0, &update.StatusError{Code: resp.StatusCode}
	}
	return resp.Body, resp.ContentLength, nil
}
