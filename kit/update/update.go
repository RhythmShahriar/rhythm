// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/RhythmShahriar/rhythm/kit/cache"
	"github.com/RhythmShahriar/rhythm/kit/config"
)

// Largest archive we'll download and largest binary we'll unpack from
// one (a compressed archive may hold a bigger binary, though not an
// unbounded one).
const (
	maxArchive = 200 << 20
	maxBinary  = 2 * maxArchive
)

// Updater checks for and installs releases of one binary. The zero
// value of every field but Repo and Binary is a working default. An
// Updater is safe for concurrent use as long as its fields are not
// changed meanwhile.
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
	Repo      string       // GitHub owner/name, e.g. RhythmShahriar/tuner
	Binary    string       // binary and archive prefix, e.g. rhythm-tuner
	Current   string       // running version; "dev" builds never update
	UserAgent string       // default rhythm-update/<Binary>
	HTTP      *http.Client // default: 5 minute timeout, proxy from the environment
	API       string       // GitHub API base; default https://api.github.com (RHYTHM_GITHUB_API in the tools)
	OS, Arch  string       // platform to install for; default runtime.GOOS, runtime.GOARCH

	// Offline makes the Updater never touch the network: Check and Notice
	// stay quiet and Latest and Install return ErrOffline. Tools set it
	// from their --offline flag.
	Offline bool

	// Progress, when set, is called while Install downloads the archive
	// with the bytes received so far and the size the server announced
	// (-1 when it didn't). It is called at most every 100 ms and once at
	// the end with done == total.
	Progress func(done, total int64)

	// Verifier, when set, checks the Sigstore bundle over checksums.txt
	// before Install trusts any checksum in it; Install fails closed
	// with ErrSignature when the bundle is missing or doesn't verify.
	// SkipSignature turns the check off for one run; this package prints
	// nothing about it, so the caller should warn.
	Verifier      Verifier
	SkipSignature bool

	// MinAge, when above zero, is a cooldown: Latest and Resolve pick the
	// newest release published at least MinAge ago and report a newer
	// one they passed over in Release.SkippedNewer. A release that turns
	// out to be malicious is usually pulled within days, so waiting keeps
	// it off machines. ParseAge reads the "3d" form; the tools take it
	// from RHYTHM_MIN_RELEASE_AGE, which this package doesn't read.
	MinAge time.Duration
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

// Release is one published release and its downloadable assets, as
// Latest, ByTag and Resolve return it. Install needs the platform archive
// and checksums.txt among the Assets.
type Release struct {
	Version    string            // without the leading v
	Tag        string            // the git tag, e.g. v0.3.1
	Published  time.Time         // when it was published; zero when unknown
	Prerelease bool              // marked as a prerelease
	Assets     map[string]string // asset name → https download URL

	// SkippedNewer is the newest release that Updater.MinAge passed over
	// because it is too young; nil when none was.
	SkippedNewer *Skipped
}

// Skipped is a release left out for being younger than Updater.MinAge.
type Skipped struct {
	Version string        // without the leading v
	Age     time.Duration // how long ago it was published
}

// Latest fetches the newest published release, never a draft or a
// prerelease. GitHub picks it, unless MinAge is set: then it is the
// newest of the last 100 releases that is at least MinAge old.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	if u.MinAge > 0 {
		return u.newest(ctx, nil, "")
	}
	var v ghRelease
	if err := u.getJSON(ctx, "/releases/latest", "", &v); err != nil {
		return nil, err
	}
	return v.release(), nil
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

// Install downloads rel for this platform, verifies it and atomically
// replaces (or creates) the binary at dest. It is InstallWith with no
// options, for callers that don't need the result.
func (u *Updater) Install(ctx context.Context, rel *Release, dest string) error {
	_, err := u.InstallWith(ctx, rel, dest, InstallOptions{})
	return err
}

// InstallOptions tunes one InstallWith call. The zero value installs the
// way Install does.
type InstallOptions struct {
	// ExpectSHA256, when set, is the hex sha256 the archive must have,
	// as a lock file recorded it. The install fails with ErrChecksum when
	// the archive differs, even if checksums.txt agrees with it: a
	// release whose assets were replaced after locking is refused.
	ExpectSHA256 string
}

// InstallResult describes a finished install.
type InstallResult struct {
	Version string // the release installed, without the leading v
	SHA256  string // hex sha256 of the archive
	Signer  string // identity the Verifier reported; "" when none ran
}

// InstallWith downloads rel for this platform, checks the signature over
// checksums.txt when u.Verifier is set, verifies the archive against
// checksums.txt (and opts.ExpectSHA256) and atomically replaces (or
// creates) the binary at dest. dest is untouched unless every check
// passes.
func (u *Updater) InstallWith(ctx context.Context, rel *Release, dest string, opts InstallOptions) (InstallResult, error) {
	var res InstallResult
	if u.offline() {
		return res, ErrOffline
	}
	expect := strings.ToLower(strings.TrimSpace(opts.ExpectSHA256))
	if expect != "" && !isSHA256(expect) {
		return res, fmt.Errorf("expected sha256 %q is not 64 hex digits", opts.ExpectSHA256)
	}
	name := u.ArchiveName()
	archiveURL, ok := rel.Assets[name]
	if !ok {
		return res, fmt.Errorf("release %s has no build for this platform (%s)", rel.Version, name)
	}

	removeStaleOld(dest)
	sums, signer, err := u.checksumsFile(ctx, rel)
	if err != nil {
		return res, err
	}
	want, ok := parseChecksums(sums)[name]
	if !ok {
		return res, fmt.Errorf("checksums.txt has no entry for %s", name)
	}
	// The archive goes to a temp file next to dest and is hashed on the
	// way, so memory stays flat however big the release is.
	archive, got, err := u.downloadFile(ctx, archiveURL, filepath.Dir(dest), "."+filepath.Base(dest)+".download-*", maxArchive)
	if err != nil {
		return res, fmt.Errorf("downloading %s: %w", name, err)
	}
	defer func() { _ = os.Remove(archive) }()
	if got != want {
		return res, &ErrChecksum{Asset: name, Want: want, Got: got}
	}
	if expect != "" && got != expect {
		return res, &ErrChecksum{Asset: name, Want: expect, Got: got, Locked: true}
	}

	goos, _ := u.platform()
	bin := u.Binary
	if goos == "windows" {
		bin += ".exe"
	}
	info := swapInfo{New: rel.Version}
	if IsRelease(u.Current) {
		info.Old = strings.TrimPrefix(u.Current, "v")
	}
	if err := extract(archive, name, bin, func(r io.Reader) error { return replace(dest, r, info) }); err != nil {
		return res, err
	}
	return InstallResult{Version: rel.Version, SHA256: got, Signer: signer}, nil
}

// Seams for tests: rename lets them inject failures and useOldSwap
// forces the Windows path on every OS.
var (
	rename     = os.Rename
	useOldSwap = runtime.GOOS == "windows"
)

// replace writes data next to dest and renames it into place, keeping
// the binary it replaces as dest.prev for Rollback. On Unix that one
// rename is atomic and dest.prev is a hard link (or copy) made before it.
// On Windows a running executable can't be overwritten but can be renamed
// away, so dest moves to dest.old first, moves back if the new binary
// can't take its place, and becomes dest.prev once it has.
func replace(dest string, r io.Reader, info swapInfo) error {
	removeStaleOld(dest)
	tmp, err := createTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".new-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), &capReader{r: r, left: maxBinary}); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil { //nolint:gosec // an executable must be executable
		return err
	}
	rec := prevRecord{Version: info.Old, ReplacedBy: &binRecord{Version: info.New, SHA256: hex.EncodeToString(h.Sum(nil))}}
	oldSum, err := hashFile(dest)
	fresh := errors.Is(err, os.ErrNotExist) // nothing to keep or move away
	if err != nil && !fresh {
		return fmt.Errorf("replacing %s: %w", dest, err)
	}
	rec.SHA256 = oldSum

	if !useOldSwap {
		var keep string
		if !fresh {
			if keep, err = snapshot(dest); err != nil {
				dropPrev(dest) // no rollback this time, but never a stale one
				keep = ""
			} else {
				defer func() { _ = os.Remove(keep) }()
			}
		}
		if err := rename(tmp.Name(), dest); err != nil {
			return err
		}
		if keep != "" {
			rec.ReplacedAt = time.Now().UTC()
			keepPrev(dest, keep, rec)
		}
		return nil
	}
	old := dest + ".old"
	_ = os.Remove(old)
	moved := !fresh
	if moved {
		if err := rename(dest, old); err != nil {
			return fmt.Errorf("replacing %s: %w", dest, err)
		}
	}
	if err := rename(tmp.Name(), dest); err != nil {
		if moved {
			if rerr := rename(old, dest); rerr != nil {
				return fmt.Errorf("replacing %s: %w", dest, errors.Join(err, fmt.Errorf("restoring %s: %w", old, rerr)))
			}
		}
		return fmt.Errorf("replacing %s: %w", dest, err)
	}
	if moved {
		// .old is the running binary when a tool updates itself; Windows
		// lets it be renamed, and if it can't, removeStaleOld clears it
		// next time.
		rec.ReplacedAt = time.Now().UTC()
		keepPrev(dest, old, rec)
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
