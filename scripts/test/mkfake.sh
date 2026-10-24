#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Builds a static copy of the GitHub API layout for the installer tests:
#   sh mkfake.sh ROOT
#
# ROOT gets repos/RhythmShahriar/rhythm/releases/{latest,tags/v<version>},
# the archives under dl/rhythm/v<version>/ and a checksums.txt per release.
# server.py serves it and fills in {{BASE}} in the release JSON.
#
# Each rhythm in an archive is a shell-script stand-in. It answers
# --version and help, logs its argv to $RHYTHM_TEST_LOG and "installs" a
# tool by writing a rhythm-<tool> stand-in into --dir. The versions differ
# the way real releases do:
#   0.6.0 (latest)  knows tuner, soundcheck and echo; has install --dry-run
#   0.5.0           knows all three, no --dry-run
#   0.4.0           knows tuner and soundcheck, no --dry-run
#   0.3.5           knows all three; its help has no "Tools:" block
# One broken release tests the installer's URL check:
#   9.0.1           downloads point at plain http on another host
set -eu

root=${1:?usage: mkfake.sh ROOT}
repo=RhythmShahriar/rhythm

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# router writes a rhythm stand-in to $1: version $2, tools $3 (with their
# aliases in parentheses), dry-run support $4 (yes or no), help with a
# "Tools:" block $5 (yes or no).
router() {
	cat >"$1" <<EOF
#!/bin/sh
# A stand-in for rhythm $2, built by mkfake.sh.
version=$2
tools="$3"
dry_run=$4
tools_block=$5
EOF
	cat >>"$1" <<'EOF'
[ -z "${RHYTHM_TEST_LOG:-}" ] || printf 'rhythm %s\n' "$*" >>"$RHYTHM_TEST_LOG"
case ${1:-help} in
--version | version)
	echo "rhythm $version"
	exit 0
	;;
help | -h | --help)
	printf 'rhythm %s\n\nUsage:\n  rhythm <tool> [args]\n\n' "$version"
	if [ "$tools_block" = yes ]; then
		echo 'Tools:'
		set -- $tools
		while [ $# -gt 1 ]; do
			printf '  %s %s  a tool\n' "$1" "$2"
			shift 2
		done
		echo
	fi
	printf 'Commands:\n  rhythm install <tool> [--dir <dir>]   download a tool\n'
	exit 0
	;;
install) shift ;;
*)
	printf 'rhythm: unknown tool "%s"\n' "$1" >&2
	exit 1
	;;
esac
dir=. dry= want=
while [ $# -gt 0 ]; do
	case $1 in
	--dir)
		dir=$2
		shift 2
		;;
	--dry-run)
		if [ "$dry_run" != yes ]; then
			echo 'flag provided but not defined: -dry-run' >&2
			echo 'usage: rhythm install <tool> [--dir <dir>]' >&2
			exit 2
		fi
		dry=1
		shift
		;;
	*)
		want="$want $1"
		shift
		;;
	esac
done
known=$(printf '%s' "$tools" | tr -d '()')
for n in $want; do
	case " $known " in
	*" $n "*) ;;
	*)
		printf 'rhythm: unknown tool "%s"; known tools: %s\n' "$n" "$known" >&2
		exit 2
		;;
	esac
done
if [ -n "$dry" ]; then
	for n in $want; do echo "would install $n"; done
	exit 0
fi
for n in $want; do
	printf '#!/bin/sh\necho "rhythm-%s 0.0.0-test"\n' "$n" >"$dir/rhythm-$n"
	chmod 0755 "$dir/rhythm-$n"
	echo "Installed rhythm-$n 0.0.0-test to $dir/rhythm-$n"
done
EOF
	chmod 0755 "$1"
}

# release builds the archives, checksums.txt and release JSON for version
# $1. Download URLs start with $2.
release() {
	v=$1 base=$2 out=$root/dl/rhythm/v$1 json=$root/repos/$repo/releases/tags/v$1
	mkdir -p "$out" "$(dirname "$json")"
	assets=
	: >"$out/checksums.txt"
	for os in linux darwin; do
		for arch in amd64 arm64; do
			name=rhythm_${os}_${arch}.tar.gz
			tar -czf "$out/$name" -C "$root/build/$v" rhythm
			printf '%s  %s\n' "$(sha256 "$out/$name")" "$name" >>"$out/checksums.txt"
			assets="$assets{\"name\":\"$name\",\"browser_download_url\":\"$base/$name\"},"
		done
	done
	assets="$assets{\"name\":\"checksums.txt\",\"browser_download_url\":\"$base/checksums.txt\"}"
	printf '{"tag_name":"v%s","prerelease":false,"assets":[%s]}\n' "$v" "$assets" >"$json"
}

all="tuner (t) soundcheck (sc) echo (e)"
for spec in "0.6.0|$all|yes|yes" "0.5.0|$all|no|yes" "0.4.0|tuner (t) soundcheck (sc)|no|yes" \
	"0.3.5|$all|no|no" "9.0.1|$all|yes|yes"; do
	IFS='|' read -r v tools dry block <<EOF
$spec
EOF
	mkdir -p "$root/build/$v"
	router "$root/build/$v/rhythm" "$v" "$tools" "$dry" "$block"
done

for v in 0.6.0 0.5.0 0.4.0 0.3.5; do
	release "$v" "{{BASE}}/dl/rhythm/v$v"
done
release 9.0.1 http://example.invalid/dl/rhythm/v9.0.1
cp "$root/repos/$repo/releases/tags/v0.6.0" "$root/repos/$repo/releases/latest"
rm -rf "$root/build"
