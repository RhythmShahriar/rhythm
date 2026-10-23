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

	"github.com/RhythmShahriar/rhythm/kit/config"
)

// Largest archive we'll download.
const maxArchive = 200 << 20

type Updater struct {
	Repo      string // e.g. RhythmShahriar/tuner
	Binary    string // e.g. rhythm-tuner
	Current   string // running version; "dev" builds never update
	UserAgent string
	HTTP      *http.Client
	API       string // GitHub API base; default https://api.github.com
	OS, Arch  string // default runtime.GOOS, runtime.GOARCH
}

type Release struct {
	Version string            // without the leading v
	Assets  map[string]string // name → download URL
}

func (u *Updater) client() *http.Client {
	if u.HTTP != nil {
		return u.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

// Latest fetches the newest published release.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	api := u.API
	if api == "" {
		api = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/repos/"+u.Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", u.UserAgent)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok) // avoids the 60/hour anonymous limit in CI
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s has no published releases yet", u.Repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checking %s releases: HTTP %d", u.Repo, resp.StatusCode)
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
	return l.pre > c.pre
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
	name := u.ArchiveName()
	archiveURL, ok := rel.Assets[name]
	if !ok {
		return fmt.Errorf("release %s has no build for this platform (%s)", rel.Version, name)
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return fmt.Errorf("release %s has no checksums.txt; refusing to install unverified binaries", rel.Version)
	}

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
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("%s: checksum mismatch; the download may be corrupt or tampered with, nothing was changed", name)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.UserAgent)
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
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

// replace writes data next to dest and renames it into place. On Windows a
// running executable can't be overwritten, but it can be renamed away.
func replace(dest string, data []byte) error {
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
	if runtime.GOOS == "windows" {
		old := dest + ".old"
		_ = os.Remove(old)
		if err := os.Rename(dest, old); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return os.Rename(tmp.Name(), dest)
}

// IsRelease reports whether v is a release version (not "dev").
func IsRelease(v string) bool {
	_, ok := parseSemver(v)
	return ok
}

// CheckDisabled reports whether update checks are off: dev builds, CI,
// or RHYTHM_NO_UPDATE_CHECK set.
func CheckDisabled(current string) bool {
	if !IsRelease(current) {
		return true
	}
	return os.Getenv("RHYTHM_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != ""
}

type checkState struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

// Check returns the latest version if it is newer than Current. It asks
// GitHub at most once a day and remembers the answer in the config dir.
func (u *Updater) Check(ctx context.Context) (latest string, newer bool) {
	if CheckDisabled(u.Current) {
		return "", false
	}
	dir, err := config.Dir()
	if err != nil {
		return "", false
	}
	statePath := filepath.Join(dir, "update-check", u.Binary+".json")
	var st checkState
	if data, err := os.ReadFile(statePath); err == nil { //nolint:gosec // path is built from the config dir
		_ = json.Unmarshal(data, &st)
	}
	if time.Since(st.Checked) > 24*time.Hour {
		rel, err := u.Latest(ctx)
		if err != nil {
			return "", false
		}
		st = checkState{Checked: time.Now().UTC(), Latest: rel.Version}
		if data, err := json.Marshal(st); err == nil && os.MkdirAll(filepath.Dir(statePath), 0o700) == nil {
			_ = os.WriteFile(statePath, data, 0o600)
		}
	}
	return st.Latest, Newer(u.Current, st.Latest)
}
