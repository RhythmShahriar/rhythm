#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Checks a GoReleaser dist/ after a snapshot build: every archive has an
# SPDX SBOM, checksums.txt covers archives, SBOMs and Linux packages and
# matches them, the packages install the binary in /usr/bin and the
# Homebrew, Scoop and winget manifests were written.
#
# Usage: check-dist.sh [dist] [binary]   (defaults: dist rhythm)
#
# With WINGET_SCHEMAS set (a URL or directory holding winget-cli's
# schemas/JSON/manifests tree) and check-jsonschema on PATH, each winget
# manifest is also validated against the schema for its ManifestVersion.
# rpm contents are listed only when rpm is installed.
set -eu

dist=${1:-dist}
bin=${2:-rhythm}
problems=0

fail() {
	echo "check-dist: $*" >&2
	problems=$((problems + 1))
}

# listed NAME: NAME is a file in checksums.txt. It compares the text after
# the two-space separator exactly; grep would read the dots in NAME as any
# character.
listed() {
	awk -v f="$1" 'substr($0, index($0, "  ") + 2) == f { found = 1 } END { exit !found }' "$dist/checksums.txt"
}

# sha256_check FILE: checks the sha256sum-format FILE against the files in
# the current directory and prints only the mismatches. Older macOS ships
# shasum but not sha256sum.
sha256_check() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum --quiet -c "$1"
		return
	fi
	out=$(shasum -a 256 -c "$1" 2>&1) && return 0
	printf '%s\n' "$out" | grep -v ': OK$' >&2
	return 1
}

[ -f "$dist/checksums.txt" ] || { echo "check-dist: $dist/checksums.txt not found" >&2; exit 2; }

if ! (cd "$dist" && sha256_check checksums.txt); then
	fail "checksums.txt doesn't match the files in $dist"
fi

archives=0
for a in "$dist"/*.tar.gz "$dist"/*.zip; do
	[ -f "$a" ] || continue
	archives=$((archives + 1))
	name=${a##*/}
	listed "$name" || fail "$name is not in checksums.txt"
	sbom="$a.sbom.json"
	if [ ! -f "$sbom" ]; then
		fail "$name has no SBOM ($name.sbom.json)"
		continue
	fi
	listed "$name.sbom.json" || fail "$name.sbom.json is not in checksums.txt"
	grep -q '"spdxVersion"' "$sbom" || fail "$name.sbom.json is not SPDX JSON"
done
[ "$archives" -gt 0 ] || fail "no archives in $dist"

for ext in deb rpm apk; do
	found=0
	for p in "$dist"/*."$ext"; do
		[ -f "$p" ] || continue
		found=1
		name=${p##*/}
		listed "$name" || fail "$name is not in checksums.txt"
		case $ext in
		deb) files=$(dpkg-deb -c "$p" | awk '{print $NF}' | sed 's|^\./|/|') ;;
		rpm)
			if ! command -v rpm >/dev/null 2>&1; then
				echo "check-dist: rpm not installed; not listing $name"
				continue
			fi
			files=$(rpm -qlp "$p" 2>/dev/null)
			;;
		apk) files=$(tar -tzf "$p" 2>/dev/null | sed 's|^|/|') ;;
		esac
		for want in "/usr/bin/$bin" "/usr/share/doc/$bin/LICENSE" "/usr/share/doc/$bin/README.md"; do
			printf '%s\n' "$files" | grep -qx "$want" || fail "$name doesn't contain $want"
		done
	done
	[ "$found" -eq 1 ] || fail "no .$ext package in $dist"
done

[ -f "$dist/homebrew/Casks/$bin.rb" ] || fail "no Homebrew cask ($dist/homebrew/Casks/$bin.rb)"
[ -f "$dist/scoop/bucket/$bin.json" ] || fail "no Scoop manifest ($dist/scoop/bucket/$bin.json)"

manifests=$(find "$dist/winget" -name '*.yaml' 2>/dev/null | sort)
[ -n "$manifests" ] || fail "no winget manifests under $dist/winget"
for m in $manifests; do
	type=$(sed -n 's/^ManifestType: *//p' "$m")
	version=$(sed -n 's/^ManifestVersion: *//p' "$m")
	if [ -z "$type" ] || [ -z "$version" ]; then
		fail "$m has no ManifestType or ManifestVersion"
		continue
	fi
	if [ -n "${WINGET_SCHEMAS:-}" ]; then
		schema="$WINGET_SCHEMAS/v$version/manifest.$type.$version.json"
		check-jsonschema --schemafile "$schema" "$m" || fail "$m doesn't match $schema"
	fi
done

if [ "$problems" -gt 0 ]; then
	echo "check-dist: $problems problem(s) in $dist" >&2
	exit 1
fi
echo "check-dist: $dist ok ($archives archives)"
