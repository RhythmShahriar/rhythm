// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package update installs and updates Rhythm binaries from GitHub Releases.
//
// Release archives are named <binary>_<os>_<arch>.tar.gz (.zip on Windows)
// with a checksums.txt alongside, as GoReleaser publishes them. Every
// download is verified against checksums.txt before anything is replaced,
// and the binary is swapped atomically so a failed update never leaves a
// broken install.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cache"
	"github.com/RhythmShahriar/rhythm/kit/config"
)

// Largest archive we'll download.
const maxArchive = 200 << 20

// Updater checks for and installs releases of one binary.
//
// It reads these environment variables:
//
//   - GITHUB_TOKEN is sent only when API is api.github.com. It then goes
//     only to api.github.com and the github.com and
//     objects.githubusercontent.com download hosts. It lifts the 60
//     requests an hour anonymous limit in CI.
//   - RHYTHM_GITHUB_TOKEN is sent to the host named by API (a mirror or
//     GitHub Enterprise) and takes precedence over GITHUB_TOKEN.
//   - RHYTHM_OFFLINE, set to anything but "" or "0", works like Offline.
//   - RHYTHM_NO_UPDATE_CHECK and CI turn off the background check (see
//     CheckDisabled) but still allow an explicit update.
//
// The API base and every download URL must be https; plain http is only
// accepted for loopback hosts, for tests. A token is never sent on a
// redirect to another host and never logged.
type Updater struct {
	Repo      string // e.g. RhythmShahriar/tuner
	Binary    string // e.g. rhythm-tuner
	Current   string // running version; "dev" builds never update
	UserAgent string
	HTTP      *http.Client
	API       string // GitHub API base; default https://api.github.com
	OS, Arch  string // default runtime.GOOS, runtime.GOARCH

	// Offline makes the Updater never touch the network: Check and Notice
	// stay quiet and Latest and Install return ErrOffline. Tools set it
	// from their --offline flag.
	Offline bool
}

// ErrOffline is returned by calls that need the network while offline
// mode is on.
var ErrOffline = errors.New("offline mode is on; unset RHYTHM_OFFLINE to download")

// offline reports whether u, or the environment, forbids network access.
func (u *Updater) offline() bool {
	return u.Offline || envOffline()
}

func envOffline() bool {
	v := os.Getenv("RHYTHM_OFFLINE")
	return v != "" && v != "0"
}

// Release is one published release and its downloadable assets.
type Release struct {
	Version string            // without the leading v
	Assets  map[string]string // name → download URL
}

// Latest fetches the newest published release.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	if u.offline() {
		return nil, ErrOffline
	}
	api := u.apiBase()
	if err := checkURL(api); err != nil {
		return nil, fmt.Errorf("release API: %w", err)
	}
	req, err := u.newRequest(ctx, api+"/repos/"+u.Repo+"/releases/latest")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, netError(req, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrNoRelease{Repo: u.Repo, API: api, Err: &StatusError{Code: resp.StatusCode}}
	}
	if resp.StatusCode != http.StatusOK {
		err := u.statusError(req, resp)
		var rl *ErrRateLimited
		if errors.As(err, &rl) {
			return nil, err // already one plain line
		}
		return nil, fmt.Errorf("checking %s releases: %w", u.Repo, err)
	}
	var v struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&v); err != nil {
		return nil, fmt.Errorf("checking %s releases: %w", u.Repo, err)
	}
	rel := &Release{Version: strings.TrimPrefix(v.TagName, "v"), Assets: map[string]string{}}
	for _, a := range v.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// Newer reports whether latest is a newer release than current. Dev builds
// and unparseable versions are never considered outdated.
func Newer(current, latest string) bool {
	c, ok1 := parseSemver(current)
	l, ok2 := parseSemver(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range 3 {
		if l.n[i] != c.n[i] {
			return l.n[i] > c.n[i]
		}
	}
	// 1.2.0 > 1.2.0-rc1; between prereleases, compare the labels.
	switch {
	case c.pre == l.pre:
		return false
	case l.pre == "":
		return true
	case c.pre == "":
		return false
	}
	return comparePre(l.pre, c.pre) > 0
}

// comparePre orders two prerelease labels as SemVer 2.0.0 section 11
// does: dot-separated identifiers compared left to right, numeric ones
// numerically and below alphanumeric ones; a label with fewer
// identifiers lower when the rest are equal. One extension: digit runs
// inside an alphanumeric identifier also compare numerically, so rc9
// sorts before rc10 the way people tag releases. Identifiers without
// digits keep plain ASCII order, as in the spec.
func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(as), len(bs)) {
		x, y := as[i], bs[i]
		xn, yn := isNumeric(x), isNumeric(y)
		var c int
		switch {
		case xn && yn:
			c = compareDigits(x, y)
		case xn:
			c = -1
		case yn:
			c = 1
		default:
			c = compareNatural(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// compareDigits compares two digit strings by value without parsing, so
// no identifier is too long to compare.
func compareDigits(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// compareNatural compares a and b run by run: digit runs by value, other
// runs as text.
func compareNatural(a, b string) int {
	x, y := a, b
	for x != "" && y != "" {
		var rx, ry string
		rx, x = nextRun(x)
		ry, y = nextRun(y)
		var c int
		if isNumeric(rx) && isNumeric(ry) {
			c = compareDigits(rx, ry)
		} else {
			c = strings.Compare(rx, ry)
		}
		if c != 0 {
			return c
		}
	}
	if c := cmp.Compare(len(x), len(y)); c != 0 {
		return c
	}
	return strings.Compare(a, b) // equal by value, as with rc01 and rc1
}

// nextRun splits s after its leading run of digits or of non-digits.
func nextRun(s string) (run, rest string) {
	digit := s[0] >= '0' && s[0] <= '9'
	i := 1
	for i < len(s) && (s[i] >= '0' && s[i] <= '9') == digit {
		i++
	}
	return s[:i], s[i:]
}

type semver struct {
	n   [3]int
	pre string
}

func parseSemver(v string) (semver, bool) {
	var s semver
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "+")
	v, s.pre, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return s, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return s, false
		}
		s.n[i] = n
	}
	return s, true
}

func (u *Updater) platform() (goos, arch string) {
	goos, arch = u.OS, u.Arch
	if goos == "" {
		goos = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	return goos, arch
}

// ArchiveName is the release asset for this platform.
func (u *Updater) ArchiveName() string {
	goos, arch := u.platform()
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s_%s_%s%s", u.Binary, goos, arch, ext)
}

// Install downloads rel for this platform, verifies its checksum and
// atomically replaces (or creates) the binary at dest.
func (u *Updater) Install(ctx context.Context, rel *Release, dest string) error {
	if u.offline() {
		return ErrOffline
	}
	name := u.ArchiveName()
	archiveURL, ok := rel.Assets[name]
	if !ok {
		return fmt.Errorf("release %s has no build for this platform (%s)", rel.Version, name)
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return fmt.Errorf("release %s has no checksums.txt; refusing to install unverified binaries", rel.Version)
	}

	removeStaleOld(dest)
	sums, err := u.download(ctx, sumsURL, 1<<20)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return err
	}
	archive, err := u.download(ctx, archiveURL, maxArchive)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return &ErrChecksum{Asset: name, Want: want, Got: got}
	}

	goos, _ := u.platform()
	bin := u.Binary
	if goos == "windows" {
		bin += ".exe"
	}
	var data []byte
	if strings.HasSuffix(name, ".zip") {
		data, err = fromZip(archive, bin)
	} else {
		data, err = fromTarGz(archive, bin)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return replace(dest, data)
}

func (u *Updater) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	if err := checkURL(url); err != nil {
		return nil, err
	}
	req, err := u.newRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, netError(req, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, u.statusError(req, resp)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file is larger than expected")
	}
	return data, nil
}

func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func fromTarGz(archive []byte, bin string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("no %s in the archive", bin)
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == bin {
			return io.ReadAll(io.LimitReader(tr, maxArchive))
		}
	}
}

func fromZip(archive []byte, bin string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if path.Base(f.Name) == bin && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(io.LimitReader(rc, maxArchive))
		}
	}
	return nil, fmt.Errorf("no %s in the archive", bin)
}

// Seams for tests: rename lets them inject failures and useOldSwap
// forces the Windows path on every OS.
var (
	rename     = os.Rename
	useOldSwap = runtime.GOOS == "windows"
)

// replace writes data next to dest and renames it into place. On Unix that
// one rename is atomic. On Windows a running executable can't be
// overwritten but can be renamed away, so dest moves to dest.old first and
// moves back if the new binary can't take its place.
func replace(dest string, data []byte) error {
	removeStaleOld(dest)
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".new-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("can't write to %s; rerun with sudo, or reinstall to a directory you own", dir)
		}
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil { //nolint:gosec // an executable must be executable
		return err
	}
	if !useOldSwap {
		return rename(tmp.Name(), dest)
	}
	old := dest + ".old"
	_ = os.Remove(old)
	moved := true
	if err := rename(dest, old); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("replacing %s: %w", dest, err)
		}
		moved = false // a fresh install: nothing to move away
	}
	if err := rename(tmp.Name(), dest); err != nil {
		if moved {
			if rerr := rename(old, dest); rerr != nil {
				return fmt.Errorf("replacing %s: %w", dest, errors.Join(err, fmt.Errorf("restoring %s: %w", old, rerr)))
			}
		}
		return fmt.Errorf("replacing %s: %w", dest, err)
	}
	return nil
}

// removeStaleOld deletes the dest.old a previous Windows update left
// behind. It only does so while dest exists, so it never deletes the last
// copy of a binary.
func removeStaleOld(dest string) {
	if _, err := os.Stat(dest); err == nil {
		_ = os.Remove(dest + ".old")
	}
}

// IsRelease reports whether v is a release version (not "dev").
func IsRelease(v string) bool {
	_, ok := parseSemver(v)
	return ok
}

// CheckDisabled reports whether update checks are off: dev builds, CI,
// RHYTHM_NO_UPDATE_CHECK set, or RHYTHM_OFFLINE set to anything but ""
// or "0".
func CheckDisabled(current string) bool {
	if !IsRelease(current) {
		return true
	}
	return os.Getenv("RHYTHM_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != "" || envOffline()
}

func (u *Updater) checkDisabled() bool {
	return u.Offline || CheckDisabled(u.Current)
}

type checkState struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

// Check returns the latest version if it is newer than Current. It asks
// GitHub at most once a day and remembers the answer in the config dir;
// a state file that is missing, corrupt or dated in the future counts as
// stale.
func (u *Updater) Check(ctx context.Context) (latest string, newer bool) {
	if u.checkDisabled() {
		return "", false
	}
	dir, err := config.Dir()
	if err != nil {
		return "", false
	}
	statePath := filepath.Join(dir, "update-check", u.Binary+".json")
	var st checkState
	fresh := false
	if err := cache.ReadJSON(statePath, &st); err == nil {
		// A state from the future (a clock that was wrong, or a copied
		// config dir) would otherwise silence checks until that date.
		age := time.Since(st.Checked)
		fresh = age >= 0 && age < 24*time.Hour
	}
	if !fresh {
		// A missing or corrupt state ends up here and is rewritten.
		rel, err := u.Latest(ctx)
		if err != nil {
			return "", false
		}
		st = checkState{Checked: time.Now().UTC(), Latest: rel.Version}
		// Atomic, so tools starting together never leave a torn file.
		_ = cache.WriteJSON(statePath, st)
	}
	return st.Latest, Newer(u.Current, st.Latest)
}

// Notice is the one way a tool should check for updates. It checks for a
// newer release in the background and delivers a one-line notice on the
// returned channel, or closes it when there's nothing to say. Run it
// alongside the tool's real work and read it at the end without waiting
// long, so it never delays the user. When u.Offline is set or
// CheckDisabled is true the channel is closed at once and nothing is
// sent over the network.
func Notice(ctx context.Context, u *Updater) <-chan string {
	ch := make(chan string, 1)
	if u.checkDisabled() {
		close(ch)
		return ch
	}
	go func() {
		defer close(ch)
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if latest, newer := u.Check(cctx); newer {
			ch <- fmt.Sprintf("A new release of %s is available: %s → %s\n  run: rhythm update", u.Binary, u.Current, latest)
		}
	}()
	return ch
}
