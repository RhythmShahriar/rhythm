#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Tests for check-dist.sh against a small fake dist/. dpkg-deb, rpm and
# check-jsonschema are stubs on PATH: the fake .deb and .rpm files hold
# their own file listing, so the test needs none of those tools.
# Usage: sh .github/scripts/check-dist_test.sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
script="$here/check-dist.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failures=0

mkdir -p "$tmp/stubs"
cat >"$tmp/stubs/dpkg-deb" <<'EOF'
#!/bin/sh
# dpkg-deb -c FILE: print the listing stored in FILE, in dpkg's format.
while read -r f; do echo "-rwxr-xr-x root/root 1 2026-01-01 00:00 .$f"; done <"$2"
EOF
cat >"$tmp/stubs/rpm" <<'EOF'
#!/bin/sh
# rpm -qlp FILE: the listing stored in FILE.
cat "$2"
EOF
cat >"$tmp/stubs/check-jsonschema" <<'EOF'
#!/bin/sh
echo "$@" >>"$STUB_LOG"
[ -z "${STUB_FAIL:-}" ]
EOF
chmod +x "$tmp/stubs/"*
PATH="$tmp/stubs:$PATH"
STUB_LOG="$tmp/schema.log"
export PATH STUB_LOG

# make_dist DIR: a dist/ that check-dist.sh accepts.
make_dist() {
	d=$1
	mkdir -p "$d/homebrew/Casks" "$d/scoop/bucket" "$d/winget/manifests/r/RhythmShahriar/rhythm/1.0.0" "$d/apkroot/usr/bin" "$d/apkroot/usr/share/doc/rhythm"
	listing='/usr/bin/rhythm
/usr/share/doc/rhythm/LICENSE
/usr/share/doc/rhythm/README.md'
	for a in rhythm_linux_amd64.tar.gz rhythm_windows_amd64.zip; do
		echo "archive $a" >"$d/$a"
		echo '{"spdxVersion": "SPDX-2.3"}' >"$d/$a.sbom.json"
	done
	printf '%s\n' "$listing" >"$d/rhythm_1.0.0_amd64.deb"
	printf '%s\n' "$listing" >"$d/rhythm-1.0.0-1.x86_64.rpm"
	: >"$d/apkroot/usr/bin/rhythm"
	: >"$d/apkroot/usr/share/doc/rhythm/LICENSE"
	: >"$d/apkroot/usr/share/doc/rhythm/README.md"
	(cd "$d/apkroot" && tar -czf ../rhythm_1.0.0_x86_64.apk usr)
	rm -rf "$d/apkroot"
	echo 'cask "rhythm" do end' >"$d/homebrew/Casks/rhythm.rb"
	echo '{}' >"$d/scoop/bucket/rhythm.json"
	for t in installer defaultLocale version; do
		printf 'ManifestType: %s\nManifestVersion: 1.12.0\n' "$t" >"$d/winget/manifests/r/RhythmShahriar/rhythm/1.0.0/RhythmShahriar.rhythm.$t.yaml"
	done
	(cd "$d" && sha256sum -- *.tar.gz *.zip *.sbom.json *.deb *.rpm *.apk >checksums.txt)
}

# expect NAME CODE PATTERN DIR: check-dist.sh DIR exits with CODE and its
# output contains PATTERN.
expect() {
	name=$1 code=$2 pattern=$3 dir=$4
	set +e
	out=$(sh "$script" "$dir" 2>&1)
	got=$?
	set -e
	if [ "$got" -ne "$code" ] || ! printf '%s\n' "$out" | grep -q -- "$pattern"; then
		echo "FAIL $name: exit $got, want $code with '$pattern'; output:"
		printf '%s\n' "$out" | sed 's/^/    /'
		failures=$((failures + 1))
	else
		echo "ok   $name"
	fi
}

n=0
fresh() {
	n=$((n + 1))
	d="$tmp/d$n"
	make_dist "$d"
}

fresh
expect "complete dist passes" 0 " ok (2 archives)" "$d"

fresh
rm "$d/rhythm_windows_amd64.zip.sbom.json"
expect "missing SBOM" 1 "rhythm_windows_amd64.zip has no SBOM" "$d"

fresh
grep -v 'linux_amd64.tar.gz.sbom.json' "$d/checksums.txt" >"$d/c" && mv "$d/c" "$d/checksums.txt"
expect "SBOM not in checksums" 1 "rhythm_linux_amd64.tar.gz.sbom.json is not in checksums.txt" "$d"

fresh
grep -v '\.deb$' "$d/checksums.txt" >"$d/c" && mv "$d/c" "$d/checksums.txt"
expect "package not in checksums" 1 "rhythm_1.0.0_amd64.deb is not in checksums.txt" "$d"

fresh
echo tampered >>"$d/rhythm_linux_amd64.tar.gz"
expect "tampered archive" 1 "checksums.txt doesn't match" "$d"

fresh
echo '{"bomFormat": "CycloneDX"}' >"$d/rhythm_linux_amd64.tar.gz.sbom.json"
(cd "$d" && sha256sum -- *.tar.gz *.zip *.sbom.json *.deb *.rpm *.apk >checksums.txt)
expect "SBOM not SPDX" 1 "is not SPDX JSON" "$d"

fresh
echo /usr/local/bin/rhythm >"$d/rhythm_1.0.0_amd64.deb"
(cd "$d" && sha256sum -- *.tar.gz *.zip *.sbom.json *.deb *.rpm *.apk >checksums.txt)
expect "deb without /usr/bin" 1 "rhythm_1.0.0_amd64.deb doesn't contain /usr/bin/rhythm" "$d"

fresh
grep -v README "$d/rhythm-1.0.0-1.x86_64.rpm" >"$d/r" && mv "$d/r" "$d/rhythm-1.0.0-1.x86_64.rpm"
(cd "$d" && sha256sum -- *.tar.gz *.zip *.sbom.json *.deb *.rpm *.apk >checksums.txt)
expect "rpm without README" 1 "doesn't contain /usr/share/doc/rhythm/README.md" "$d"

fresh
rm "$d"/*.apk
grep -v '\.apk$' "$d/checksums.txt" >"$d/c" && mv "$d/c" "$d/checksums.txt"
expect "no apk" 1 "no .apk package" "$d"

fresh
rm -r "$d/winget" "$d/scoop"
expect "no winget or scoop" 1 "no winget manifests" "$d"

fresh
rm "$d/checksums.txt"
expect "no checksums" 2 "checksums.txt not found" "$d"

fresh
: >"$STUB_LOG"
WINGET_SCHEMAS=https://schemas.example/manifests
export WINGET_SCHEMAS
expect "winget schemas checked" 0 " ok (2 archives)" "$d"
if grep -q -- "--schemafile https://schemas.example/manifests/v1.12.0/manifest.installer.1.12.0.json" "$STUB_LOG" &&
	[ "$(wc -l <"$STUB_LOG")" -eq 3 ]; then
	echo "ok   winget schema per manifest type"
else
	echo "FAIL winget schema per manifest type:"
	sed 's/^/    /' "$STUB_LOG"
	failures=$((failures + 1))
fi
STUB_FAIL=1
export STUB_FAIL
expect "winget schema mismatch" 1 "doesn't match https://schemas.example" "$d"
unset WINGET_SCHEMAS STUB_FAIL

if [ "$failures" -gt 0 ]; then
	echo "$failures test(s) failed"
	exit 1
fi
echo "all check-dist tests passed"
