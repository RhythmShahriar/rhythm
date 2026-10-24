// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package update installs and updates Rhythm binaries from GitHub Releases,
// or from any other Source such as an offline bundle.
//
// # Release contract
//
// Every release of a binary publishes, as GoReleaser builds them
// (.goreleaser.yaml) and as scripts/install.sh and scripts/install.ps1
// expect them:
//
//   - <binary>_<os>_<arch>.tar.gz, or <binary>_<os>_<arch>.zip on
//     Windows, holding the binary (<binary>.exe on Windows) at any depth.
//     os and arch are Go's names: linux, darwin, windows; amd64, arm64.
//   - checksums.txt: one "<sha256>  <archive>" line per archive, in the
//     sha256sum format.
//   - checksums.txt.sigstore.json: a keyless Sigstore bundle over
//     checksums.txt, signed by the shared release workflow.
//
// The tag is v<version>; Release.Version drops the v. Renaming any of
// these breaks updates for everyone on an older version, so the names
// only ever change together with the installers and with a transition
// period.
//
// # Environment
//
//   - RHYTHM_GITHUB_API: a mirror or GitHub Enterprise API base. Callers
//     read it and set Updater.API. It must be https (plain http only
//     to a loopback host) and a plain base URL, without user info, query
//     or fragment (see CheckAPI).
//   - GITHUB_TOKEN: sent only when the API is api.github.com and then
//     only to api.github.com, github.com and objects.githubusercontent.com.
//   - RHYTHM_GITHUB_TOKEN: sent to the host named by the API base, for a
//     mirror; it takes precedence over GITHUB_TOKEN.
//   - RHYTHM_OFFLINE: anything but "" or "0" turns on offline mode, as
//     Updater.Offline does: no request is sent at all.
//   - RHYTHM_NO_UPDATE_CHECK or CI: no background update check (Check
//     and Notice), while an explicit Install still works.
//
// The default client takes its proxy from HTTPS_PROXY, HTTP_PROXY and
// NO_PROXY.
//
// # Verification order
//
// Install changes nothing until every check has passed:
//
//  1. The API base and every download URL, redirects included, must be
//     https (plain http only to loopback, for tests).
//  2. checksums.txt is downloaded (at most 1 MB).
//  3. When Updater.Verifier is set and SkipSignature is not,
//     checksums.txt.sigstore.json is downloaded (at most 1 MB) and the
//     Verifier must accept it over the exact checksums.txt bytes. A
//     missing bundle or a failed check is ErrSignature: verification fails
//     closed. With no Verifier this step is skipped.
//  4. checksums.txt must list the platform archive.
//  5. The archive is streamed to a temp file next to the destination (at
//     most 200 MB) and hashed on the way; its sha256 must match
//     checksums.txt and, when InstallOptions.ExpectSHA256 is set, that
//     hash too; otherwise Install returns ErrChecksum. With
//     Updater.CacheDir set, an archive already cached under that sha256
//     is hashed again and used instead of a download; a newly downloaded
//     one is cached only after it has passed.
//  6. The binary is unpacked from that file into a second temp file next
//     to the destination and swapped into place.
//
// # Sources
//
// Releases come from GitHub unless Updater.Source names another Source:
// Latest, ByTag and Open, plus Releases when it is also a Lister. The
// checks above apply to every Source. Offline mode and RHYTHM_OFFLINE
// stop only the GitHub source, so a local bundle still installs.
//
// # Pinning
//
// Resolve turns a version spec into a release: "latest", an exact
// version (found by tag, prereleases included) or a partial "X.Y" for the
// newest X.Y.* release. Checksums returns every platform's archive hash
// from a release's checksums.txt, signature checked, for a lock file;
// InstallOptions.ExpectSHA256 then refuses an archive that no longer has
// the locked hash.
//
// Updater.MinAge adds a cooldown: Latest and partial specs skip releases
// younger than it and name the newest one skipped in
// Release.SkippedNewer; ErrTooNew means none is old enough. ParseAge
// reads ages such as "3d", "1w" or "36h" and FormatAge prints them.
//
// # Swap guarantees
//
// On macOS and Linux the new binary replaces the old one with a single
// rename, so the destination is always either the whole old binary or
// the whole new one. On Windows a running executable can't be replaced,
// so the old one is renamed to <binary>.old first; if the new one then
// can't take its place, .old is renamed back. A stale .old is deleted on
// the next update, only while the binary itself exists. Temp files
// are removed on every error, including a canceled context.
//
// # Rollback
//
// Every install over an existing binary keeps the one it replaced as
// <binary>.prev: on Unix a hard link (or a copy) made before the rename,
// on Windows the .old file once the swap has worked. <binary>.prev.json
// records its version, sha256 and replaced_at time, plus the version and
// sha256 of the binary that replaced it. Rollback swaps the two back and
// forth with the same undo on failure, refuses a .prev whose sha256 has
// changed and returns ErrNoPrevious when there is none. Failing to keep
// .prev never fails an install; it removes any older .prev instead.
//
// # Errors
//
// Network failures come back as ErrUnreachable, ErrRateLimited or
// ErrNoRelease, a bad archive as ErrChecksum and a bad signature as
// ErrSignature. Each prints as one line a user can act on and wraps its
// cause for errors.Is and errors.As.
package update
