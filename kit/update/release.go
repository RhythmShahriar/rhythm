// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ghRelease is the part of GitHub's release object this package reads.
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"` // null for drafts, which leaves it zero
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (v *ghRelease) release() *Release {
	rel := &Release{
		Version:    strings.TrimPrefix(v.TagName, "v"),
		Tag:        v.TagName,
		Published:  v.Published,
		Prerelease: v.Prerelease,
		Assets:     map[string]string{},
	}
	for _, a := range v.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel
}

// getJSON fetches /repos/{Repo}{path} from the release API into v. A 404
// becomes ErrNoRelease for want ("" for the latest release).
func (u *Updater) getJSON(ctx context.Context, path, want string, v any) error {
	if u.offline() {
		return ErrOffline
	}
	api := u.apiBase()
	if err := checkURL(api); err != nil {
		return fmt.Errorf("release API: %w", err)
	}
	req, err := u.newRequest(ctx, api+"/repos/"+u.Repo+path)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.client().Do(req)
	if err != nil {
		return netError(req, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return &ErrNoRelease{Repo: u.Repo, API: api, Version: want, Err: &StatusError{Code: resp.StatusCode}}
	}
	if resp.StatusCode != http.StatusOK {
		err := u.statusError(req, resp)
		var rl *ErrRateLimited
		if errors.As(err, &rl) {
			return err // already one plain line
		}
		return fmt.Errorf("checking %s releases: %w", u.Repo, err)
	}
	// A listing of 100 releases with all their assets fits easily.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(v); err != nil {
		return fmt.Errorf("checking %s releases: %w", u.Repo, err)
	}
	return nil
}

// ByTag fetches the release tagged v<version>. The version may carry the
// leading v or not. Prereleases are returned too, since asking for one by
// name is deliberate; drafts are not.
func (u *Updater) ByTag(ctx context.Context, version string) (*Release, error) {
	tag := "v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
	var v ghRelease
	if err := u.getJSON(ctx, "/releases/tags/"+tag, tag, &v); err != nil {
		return nil, err
	}
	if v.Draft {
		return nil, &ErrNoRelease{Repo: u.Repo, API: u.apiBase(), Version: tag}
	}
	return v.release(), nil
}

// releases lists the newest 100 published releases, drafts left out.
// Pinning to an older line than the last 100 releases needs an exact
// version, which ByTag finds at any age.
func (u *Updater) releases(ctx context.Context) ([]*Release, error) {
	var vs []ghRelease
	if err := u.getJSON(ctx, "/releases?per_page=100", "", &vs); err != nil {
		return nil, err
	}
	out := make([]*Release, 0, len(vs))
	for i := range vs {
		if !vs[i].Draft {
			out = append(out, vs[i].release())
		}
	}
	return out, nil
}

// Resolve finds the release a version spec names:
//
//   - "" or "latest": the newest release, as Latest.
//   - an exact version such as "0.3.1", "v0.3.1" or "0.4.0-rc1": that
//     release, as ByTag, prereleases included.
//   - a partial version "X.Y" or "X": the newest X.Y.* or X.*.* release
//     among the last 100, prereleases left out.
func (u *Updater) Resolve(ctx context.Context, spec string) (*Release, error) {
	s := strings.TrimPrefix(strings.TrimSpace(spec), "v")
	if s == "" || strings.EqualFold(s, "latest") {
		return u.Latest(ctx)
	}
	prefix, exact, err := parseSpec(s)
	if err != nil {
		return nil, err
	}
	if exact {
		return u.ByTag(ctx, s)
	}
	rels, err := u.releases(ctx)
	if err != nil {
		return nil, err
	}
	var best *Release
	for _, r := range rels {
		if r.Prerelease || !matches(r.Version, prefix) {
			continue
		}
		if best == nil || Newer(best.Version, r.Version) {
			best = r
		}
	}
	if best == nil {
		return nil, &ErrNoRelease{Repo: u.Repo, API: u.apiBase(), Version: s + ".*"}
	}
	return best, nil
}

// parseSpec splits a version spec without its leading v into the numbers
// it pins. exact is true for a full X.Y.Z, with or without a prerelease.
func parseSpec(s string) (prefix []int, exact bool, err error) {
	bad := fmt.Errorf("invalid version %q; use latest, an exact version such as 0.3.1, or 0.3 for the newest 0.3.x", s)
	core, pre, hasPre := strings.Cut(s, "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) > 3 || (hasPre && (len(parts) != 3 || pre == "")) {
		return nil, false, bad
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p != strconv.Itoa(n) {
			return nil, false, bad
		}
		prefix = append(prefix, n)
	}
	return prefix, len(parts) == 3, nil
}

// matches reports whether version starts with the numbers in prefix.
func matches(version string, prefix []int) bool {
	v, ok := parseSemver(version)
	if !ok {
		return false
	}
	for i, n := range prefix {
		if v.n[i] != n {
			return false
		}
	}
	return true
}

// Checksums downloads rel's checksums.txt and returns the sha256 of every
// archive it lists, keyed by archive name, so a lock file can record
// every platform. When u.Verifier is set the signature is checked first,
// exactly as Install does.
func (u *Updater) Checksums(ctx context.Context, rel *Release) (map[string]string, error) {
	sums, _, err := u.checksumsFile(ctx, rel)
	if err != nil {
		return nil, err
	}
	all := parseChecksums(sums)
	if len(all) == 0 {
		return nil, fmt.Errorf("release %s: checksums.txt lists no archives", rel.Version)
	}
	return all, nil
}

// checksumsFile downloads rel's checksums.txt and, when a Verifier is
// set, checks its signature before anyone reads it. signer is the
// identity the Verifier reported.
func (u *Updater) checksumsFile(ctx context.Context, rel *Release) (sums []byte, signer string, err error) {
	if u.offline() {
		return nil, "", ErrOffline
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return nil, "", fmt.Errorf("release %s has no checksums.txt; refusing to install unverified binaries", rel.Version)
	}
	sums, err = u.download(ctx, sumsURL, 1<<20)
	if err != nil {
		return nil, "", fmt.Errorf("downloading checksums: %w", err)
	}
	// A checksums.txt that isn't signed by the release workflow could
	// list a tampered archive's hash.
	signer, err = u.verifySignature(ctx, rel, sums)
	if err != nil {
		return nil, "", err
	}
	return sums, signer, nil
}

// parseChecksums reads a checksums.txt in the sha256sum format GoReleaser
// writes. Lines that aren't "<sha256>  <name>" are skipped; strings.Fields
// also drops the \r of CRLF line ends.
func parseChecksums(sums []byte) map[string]string {
	out := map[string]string{}
	for line := range strings.Lines(string(sums)) {
		f := strings.Fields(line)
		if len(f) != 2 || !isSHA256(f[0]) {
			continue
		}
		out[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
	}
	return out
}

func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
