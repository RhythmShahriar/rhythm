// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package update installs and updates Rhythm binaries from GitHub Releases.
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
//     read it and set Updater.API; it must be https.
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
//     checksums.txt; otherwise Install returns ErrChecksum.
//  6. The binary is unpacked from that file into a second temp file next
//     to the destination and swapped into place.
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
// # Errors
//
// Network failures come back as ErrUnreachable, ErrRateLimited or
// ErrNoRelease, a bad archive as ErrChecksum and a bad signature as
// ErrSignature. Each prints as one line a user can act on and wraps its
// cause for errors.Is and errors.As.
package update
