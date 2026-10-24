#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Builds a static copy of the GitHub API layout for the installer tests:
#   sh mkfake.sh ROOT
#
# ROOT gets repos/RhythmShahriar/<repo>/releases/{latest,tags/v<version>},
# the archives under dl/<repo>/v<version>/ and a checksums.txt per release,
# for rhythm, tuner, soundcheck and echo. server.py serves it and fills in
# {{BASE}} in the release JSON.
#
# Every binary in an archive is a shell-script stand-in that prints its
# version and logs its argv to $RHYTHM_TEST_LOG, so a test can assert what
# the installer asked for. rhythm also answers help and "installs" a tool
# by writing a rhythm-<tool> stand-in into --dir. It never downloads; the
# tool releases are there for tests that run a real rhythm. The rhythm
# versions differ the way real releases do:
#   0.6.0 (latest)  knows tuner, soundcheck and echo; has install --dry-run
#                   and --quiet; says once that --dir isn't on PATH
#   0.5.0           knows all three, no --dry-run or --quiet; says for each
#                   tool that --dir isn't on PATH
#   0.4.0           knows tuner and soundcheck, no --dry-run
#   0.3.5           knows all three; its help has no "Tools:" block
# Broken releases test the installer's checks:
#   9.0.1           downloads point at plain http on another host
#   9.0.2           checksums.txt has a wrong sum for every archive
#   9.0.3           has no checksums.txt
set -eu

root=${1:?usage: mkfake.sh ROOT}
owner=RhythmShahriar

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# tool writes a tool stand-in to $1 that reports version $2. It takes its
# name from the file it runs as. router's stand-ins are the same script.
tool() {
	printf '#!/bin/sh\nversion=%s\n' "$2" >"$1"
	cat >>"$1" <<'EOF'
name=${0##*/}
[ -z "${RHYTHM_TEST_LOG:-}" ] || printf '%s %s\n' "$name" "$*" >>"$RHYTHM_TEST_LOG"
echo "$name $version"
EOF
	chmod 0755 "$1"
}

# router writes a rhythm stand-in to $1: version $2, tools $3 (with their
# aliases in parentheses), dry-run support $4 (yes or no), help with a
# "Tools:" block $5 (yes or no). A router with --dry-run also has --quiet,
# as in the real releases. The tools it writes are tool stand-ins.
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
on_path() {
	case ":$PATH:" in
	*":$1:"*) return 0 ;;
	esac
	return 1
}
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
dir=. dry= quiet= want=
while [ $# -gt 0 ]; do
	case $1 in
	--dir)
		dir=$2
		shift 2
		;;
	-h | --help)
		if [ "$dry_run" != yes ]; then
			printf 'Usage of install:\n  -dir string\n    \tinstall directory (default: next to rhythm)\n' >&2
			echo 'usage: rhythm install <tool> [--dir <dir>]' >&2
			exit 2
		fi
		printf 'Usage: rhythm install <tool>... [flags]\n\nFlags:\n'
		printf '  --dir dir        install into dir (default: next to rhythm)\n'
		printf '  --dry-run        check the names and print the tools, one per line, without installing\n'
		printf '  --quiet          print only the Installed lines and errors\n'
		exit 0
		;;
	--quiet)
		if [ "$dry_run" != yes ]; then
			echo 'flag provided but not defined: -quiet' >&2
			exit 2
		fi
		quiet=1
		shift
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
# Aliases install the tool under its name, as rhythm does.
names=
for n in $want; do
	set -- $tools
	while [ $# -gt 1 ] && [ "$n" != "$1" ] && [ "($n)" != "$2" ]; do shift 2; done
	if [ $# -lt 2 ]; then
		printf 'rhythm: unknown tool "%s"; known tools: %s\n' "$n" "$known" >&2
		exit 2
	fi
	names="$names $1"
done
want=$names
if [ -n "$dry" ]; then
	for n in $want; do echo "would install $n"; done
	exit 0
fi
for n in $want; do
	cat >"$dir/rhythm-$n" <<'TOOL'
#!/bin/sh
version=0.0.0-test
name=${0##*/}
[ -z "${RHYTHM_TEST_LOG:-}" ] || printf '%s %s\n' "$name" "$*" >>"$RHYTHM_TEST_LOG"
echo "$name $version"
TOOL
	chmod 0755 "$dir/rhythm-$n"
	echo "Installed rhythm-$n 0.0.0-test to $dir/rhythm-$n"
	[ "$dry_run" = yes ] || on_path "$dir" || echo "Note: $dir isn't on your PATH."
done
[ "$dry_run" != yes ] || [ -n "$quiet" ] || on_path "$dir" || echo "Note: $dir isn't on your PATH."
EOF
	chmod 0755 "$1"
}

# release builds the archives, checksums.txt and release JSON for repo $1,
# version $2, from the files in $root/build/$1/$2. Download URLs start with
# $3. $4 breaks the checksums on purpose: "badsum" or "nosums".
release() {
	r=$1 v=$2 base=$3 broken=${4:-}
	bin=rhythm-$r
	[ "$r" != rhythm ] || bin=rhythm
	src=$root/build/$r/$v out=$root/dl/$r/v$v json=$root/repos/$owner/$r/releases/tags/v$v
	mkdir -p "$out" "$(dirname "$json")"
	assets=
	: >"$out/checksums.txt"
	for os in linux darwin; do
		for arch in amd64 arm64; do
			name=${r}_${os}_${arch}.tar.gz
			tar -czf "$out/$name" -C "$src" "$bin"
			sum=$(sha256 "$out/$name")
			[ "$broken" != badsum ] || sum=$(printf '%s' "$sum" | tr 0-9a-f 1-9a-f0)
			printf '%s  %s\n' "$sum" "$name" >>"$out/checksums.txt"
			assets="$assets{\"name\":\"$name\",\"browser_download_url\":\"$base/$name\"},"
		done
	done
	if [ "$broken" = nosums ]; then
		rm "$out/checksums.txt"
		assets=${assets%,}
	else
		assets="$assets{\"name\":\"checksums.txt\",\"browser_download_url\":\"$base/checksums.txt\"}"
	fi
	printf '{"tag_name":"v%s","prerelease":false,"assets":[%s]}\n' "$v" "$assets" >"$json"
}

all="tuner (t) soundcheck (sc) echo (e)"
for spec in "0.6.0|$all|yes|yes" "0.5.0|$all|no|yes" "0.4.0|tuner (t) soundcheck (sc)|no|yes" \
	"0.3.5|$all|no|no" "9.0.1|$all|yes|yes" "9.0.2|$all|yes|yes" "9.0.3|$all|yes|yes"; do
	IFS='|' read -r v tools dry block <<EOF
$spec
EOF
	mkdir -p "$root/build/rhythm/$v"
	router "$root/build/rhythm/$v/rhythm" "$v" "$tools" "$dry" "$block"
done

for v in 0.6.0 0.5.0 0.4.0 0.3.5; do
	release rhythm "$v" "{{BASE}}/dl/rhythm/v$v"
done
release rhythm 9.0.1 http://example.invalid/dl/rhythm/v9.0.1
release rhythm 9.0.2 "{{BASE}}/dl/rhythm/v9.0.2" badsum
release rhythm 9.0.3 "{{BASE}}/dl/rhythm/v9.0.3" nosums
cp "$root/repos/$owner/rhythm/releases/tags/v0.6.0" "$root/repos/$owner/rhythm/releases/latest"

for t in tuner soundcheck echo; do
	mkdir -p "$root/build/$t/0.1.0"
	tool "$root/build/$t/0.1.0/rhythm-$t" 0.1.0
	release "$t" 0.1.0 "{{BASE}}/dl/$t/v0.1.0"
	cp "$root/repos/$owner/$t/releases/tags/v0.1.0" "$root/repos/$owner/$t/releases/latest"
done
rm -rf "$root/build"
