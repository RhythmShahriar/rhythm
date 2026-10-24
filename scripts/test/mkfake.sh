#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Builds a static copy of the GitHub API layout for the installer tests:
#   sh mkfake.sh ROOT
#
# ROOT gets repos/RhythmShahriar/<repo>/releases/{latest,tags/v<version>},
# the archives under dl/<repo>/v<version>/ and a checksums.txt (with a
# stand-in checksums.txt.sigstore.json) per release,
# for rhythm, tuner, soundcheck and echo. server.py serves it and fills in
# {{BASE}} in the release JSON.
#
# Every binary in an archive is a shell-script stand-in that prints its
# version and logs its argv to $RHYTHM_TEST_LOG, so a test can assert what
# the installer asked for. rhythm also answers help and "installs" a tool
# by writing a rhythm-<tool> stand-in into --dir. Before that it fetches the
# tool's archive from $RHYTHM_GITHUB_API and fails that tool on anything but
# 200, so a test can make one download 404. Unknown names get rhythm's
# did-you-mean lines. A tool named in $RHYTHM_TEST_ALREADY gets rhythm's
# "is already installed" line instead of an install. The rhythm versions
# differ the way real releases do:
#   0.7.0 (latest)  knows tuner, soundcheck and echo; has install --dry-run
#                   and --quiet; says once that --dir isn't on PATH; has
#                   verify (which fails when RHYTHM_TEST_VERIFY=fail),
#                   install --defaults (answering $RHYTHM_TEST_DEFAULTS when
#                   set), tool@version and install --locked
#   0.6.0           the same without verify, --defaults, pins or --locked
#   0.5.0           knows all three, no --dry-run or --quiet; says for each
#                   tool that --dir isn't on PATH
#   0.4.0           knows tuner and soundcheck, no --dry-run
#   0.3.5           knows all three; its help has no "Tools:" block
# Broken releases test the installer's checks:
#   9.0.1           downloads point at plain http on another host
#   9.0.2           checksums.txt has a wrong sum for every archive
#   9.0.3           has no checksums.txt
#   9.0.4           has no checksums.txt.sigstore.json
#   9.0.5           downloads point at server.py's /hang route
#   9.0.6           downloads point at http://example.invalid?@127.0.0.1
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
# aliases in parentheses), features $4 and help with a "Tools:" block $5
# (yes or no). Features, separated by spaces: "dry-run" (install --dry-run
# and --quiet, which came together), "verify" (the verify command; it fails
# when RHYTHM_TEST_VERIFY is "fail"), "defaults" (install --defaults, which
# adds $RHYTHM_TEST_DEFAULTS or else echo, soundcheck and tuner), "pins"
# (tool@version) and "locked" (install --locked, from ./rhythm.lock). The
# tools it writes are tool stand-ins that report the pinned version.
router() {
	cat >"$1" <<EOF
#!/bin/sh
# A stand-in for rhythm $2, built by mkfake.sh.
version=$2
tools="$3"
features="$4"
tools_block=$5
EOF
	cat >>"$1" <<'EOF'
[ -z "${RHYTHM_TEST_LOG:-}" ] || printf 'rhythm %s\n' "$*" >>"$RHYTHM_TEST_LOG"
has() {
	case " $features " in
	*" $1 "*) return 0 ;;
	esac
	return 1
}
on_path() {
	case ":$PATH:" in
	*":$1:"*) return 0 ;;
	esac
	return 1
}
# bad_flag fails the way Go's flag package does for a flag it doesn't know.
bad_flag() {
	echo "flag provided but not defined: -${1#--}" >&2
	echo 'usage: rhythm install <tool> [--dir <dir>]' >&2
	exit 2
}
# suggest prints the name in $2... within two edits of $1, if any.
suggest() {
	printf '%s\n' "$@" | awk '
		function lev(a, b,   i, j, c, d) {
			for (i = 0; i <= length(a); i++) d[i, 0] = i
			for (j = 0; j <= length(b); j++) d[0, j] = j
			for (i = 1; i <= length(a); i++)
				for (j = 1; j <= length(b); j++) {
					c = substr(a, i, 1) != substr(b, j, 1)
					d[i, j] = d[i - 1, j] + 1
					if (d[i, j - 1] + 1 < d[i, j]) d[i, j] = d[i, j - 1] + 1
					if (d[i - 1, j - 1] + c < d[i, j]) d[i, j] = d[i - 1, j - 1] + c
				}
			return d[length(a), length(b)]
		}
		NR == 1 { input = $0; next }
		lev(input, $0) <= 2 { print; exit }'
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
	! has verify || printf '  rhythm verify [tool...]               check signatures\n'
	exit 0
	;;
install) shift ;;
verify)
	has verify || {
		printf 'rhythm: unknown tool "verify"\n' >&2
		exit 1
	}
	if [ "${RHYTHM_TEST_VERIFY:-}" = fail ]; then
		echo 'rhythm             signature not valid: RhythmShahriar/rhythm '"$version"': failed to verify signature' >&2
		exit 1
	fi
	echo "Signature ok  RhythmShahriar/rhythm @ refs/tags/v$version (release-go.yml)"
	exit 0
	;;
*)
	printf 'rhythm: unknown tool "%s"\n' "$1" >&2
	exit 1
	;;
esac
dir=. dry= quiet= want= defaults= locked=
while [ $# -gt 0 ]; do
	case $1 in
	--dir)
		dir=$2
		shift 2
		;;
	-h | --help)
		if ! has dry-run; then
			printf 'Usage of install:\n  -dir string\n    \tinstall directory (default: next to rhythm)\n' >&2
			echo 'usage: rhythm install <tool> [--dir <dir>]' >&2
			exit 2
		fi
		printf 'Usage: rhythm install <tool>... [flags]\n\nFlags:\n'
		! has defaults || printf '  --defaults       also install the tools a fresh setup gets\n'
		printf '  --dir dir        install into dir (default: next to rhythm)\n'
		printf '  --dry-run        check the names and print the tools, one per line, without installing\n'
		! has locked || printf '  --locked         install the versions in rhythm.lock\n'
		printf '  --quiet          print only the Installed lines and errors\n'
		exit 0
		;;
	--quiet | --dry-run)
		has dry-run || bad_flag "$1"
		if [ "$1" = --quiet ]; then quiet=1; else dry=1; fi
		shift
		;;
	--defaults | --locked)
		has "${1#--}" || bad_flag "$1"
		if [ "$1" = --defaults ]; then defaults=1; else locked=1; fi
		shift
		;;
	-*) bad_flag "$1" ;;
	*)
		want="$want $1"
		shift
		;;
	esac
done
# The defaults come after any named tools, as in rhythm.
[ -z "$defaults" ] || want="$want ${RHYTHM_TEST_DEFAULTS-echo soundcheck tuner}"
# --locked takes the tools and versions from ./rhythm.lock, written by
# rhythm lock: "name" and "version" keys in order, rhythm's own left out.
if [ -n "$locked" ]; then
	if [ ! -f rhythm.lock ]; then
		echo 'rhythm: no rhythm.lock found' >&2
		exit 1
	fi
	want=$(grep -oE '"(name|version)": *"[^"]*"' rhythm.lock | sed 's/.*"\([^"]*\)"$/\1/' |
		awk 'NR % 2 { n = $0; next } n != "rhythm" { print n "@" $0 }')
fi
# Every name is checked before anything is installed. Aliases install the
# tool under its name, as rhythm does. A pin (tuner@0.3.1) keeps its version.
names= bad=
for spec in $want; do
	n=${spec%%@*} pin=
	case $spec in
	*@*) has pins && pin=${spec#*@} || n=$spec ;;
	esac
	set -- $tools
	while [ $# -gt 1 ] && [ "$n" != "$1" ] && [ "($n)" != "$2" ]; do shift 2; done
	if [ $# -lt 2 ]; then
		bad="$bad $n"
	else
		names="$names $1@$pin"
	fi
done
if [ -n "$bad" ]; then
	known=$(set -- $tools; while [ $# -gt 1 ]; do echo "$1"; shift 2; done | sort | tr '\n' ' ')
	for n in $bad; do
		near=$(suggest "$n" $known)
		printf 'rhythm: unknown tool "%s"\n' "$n" >&2
		[ -z "$near" ] || printf '  did you mean: rhythm install %s\n' "$near" >&2
	done
	printf '  known: %s\n' "$(echo $known | sed 's/ /, /g')" >&2
	exit 2
fi
if [ -n "$dry" ]; then
	for n in $names; do echo "${n%@}"; done
	exit 0
fi
os=$(uname -s | tr ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz)
case $(uname -m) in aarch64 | arm64) arch=arm64 ;; *) arch=amd64 ;; esac
ok=0 failed= nfailed=0 count=0
for spec in $names; do
	n=${spec%@*} v=${spec#*@}
	count=$((count + 1))
	# A tool in $RHYTHM_TEST_ALREADY is "already installed", as rhythm says
	# when the binary in --dir is the version asked for.
	case " ${RHYTHM_TEST_ALREADY:-} " in
	*" $n "*)
		echo "rhythm-$n ${v:-0.0.0-test} is already installed at $dir/rhythm-$n (use --force to reinstall)"
		ok=$((ok + 1))
		continue
		;;
	esac
	# The archive is fetched from the fake API, so a test can make it 404.
	if [ -n "${RHYTHM_GITHUB_API:-}" ]; then
		archive=${n}_${os}_${arch}.tar.gz
		code=$(curl -s -o /dev/null -w '%{http_code}' "$RHYTHM_GITHUB_API/dl/$n/v0.1.0/$archive" || :)
		if [ "$code" != 200 ]; then
			printf '%-18s downloading %s: HTTP %s\n' "rhythm-$n" "$archive" "$code" >&2
			failed="$failed rhythm-$n"
			nfailed=$((nfailed + 1))
			continue
		fi
	fi
	printf '#!/bin/sh\nversion=%s\n' "${v:-0.0.0-test}" >"$dir/rhythm-$n"
	cat >>"$dir/rhythm-$n" <<'TOOL'
name=${0##*/}
[ -z "${RHYTHM_TEST_LOG:-}" ] || printf '%s %s\n' "$name" "$*" >>"$RHYTHM_TEST_LOG"
echo "$name $version"
TOOL
	chmod 0755 "$dir/rhythm-$n"
	ok=$((ok + 1))
	echo "Installed rhythm-$n ${v:-0.0.0-test} to $dir/rhythm-$n"
	has dry-run || on_path "$dir" || echo "Note: $dir isn't on your PATH."
done
! has dry-run || [ -n "$quiet" ] || on_path "$dir" || echo "Note: $dir isn't on your PATH."
if [ -n "$failed" ]; then
	[ "$count" -lt 2 ] || printf 'Installed %s, failed %s:%s\n' "$ok" "$nfailed" "$(echo "$failed" | sed 's/ /, /g; s/^,//')" >&2
	exit 1
fi
EOF
	chmod 0755 "$1"
}

# release builds the archives, checksums.txt and release JSON for repo $1,
# version $2, from the files in $root/build/$1/$2. Download URLs start with
# $3. $4 breaks the checksums on purpose: "badsum", "nosums" or "nobundle"
# (no checksums.txt.sigstore.json).
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
	if [ "$broken" != nosums ] && [ "$broken" != nobundle ]; then
		# Only the fake cosign reads the bundle, so any bytes do.
		echo '{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}' >"$out/checksums.txt.sigstore.json"
		assets="$assets,{\"name\":\"checksums.txt.sigstore.json\",\"browser_download_url\":\"$base/checksums.txt.sigstore.json\"}"
	fi
	printf '{"tag_name":"v%s","prerelease":false,"assets":[%s]}\n' "$v" "$assets" >"$json"
}

all="tuner (t) soundcheck (sc) echo (e)"
new="dry-run verify defaults pins locked"
for spec in "0.7.0|$all|$new|yes" "0.6.0|$all|dry-run|yes" "0.5.0|$all||yes" \
	"0.4.0|tuner (t) soundcheck (sc)||yes" "0.3.5|$all||no" \
	"9.0.1|$all|$new|yes" "9.0.2|$all|$new|yes" "9.0.3|$all|$new|yes" "9.0.4|$all|$new|yes" "9.0.5|$all|$new|yes" \
	"9.0.6|$all|$new|yes"; do
	IFS='|' read -r v tools features block <<EOF
$spec
EOF
	mkdir -p "$root/build/rhythm/$v"
	router "$root/build/rhythm/$v/rhythm" "$v" "$tools" "$features" "$block"
done

for v in 0.7.0 0.6.0 0.5.0 0.4.0 0.3.5; do
	release rhythm "$v" "{{BASE}}/dl/rhythm/v$v"
done
release rhythm 9.0.1 http://example.invalid/dl/rhythm/v9.0.1
release rhythm 9.0.2 "{{BASE}}/dl/rhythm/v9.0.2" badsum
release rhythm 9.0.3 "{{BASE}}/dl/rhythm/v9.0.3" nosums
release rhythm 9.0.4 "{{BASE}}/dl/rhythm/v9.0.4" nobundle
release rhythm 9.0.5 "{{BASE}}/hang/dl/rhythm/v9.0.5"
release rhythm 9.0.6 "http://example.invalid?@127.0.0.1/dl/rhythm/v9.0.6"
cp "$root/repos/$owner/rhythm/releases/tags/v0.7.0" "$root/repos/$owner/rhythm/releases/latest"

for t in tuner soundcheck echo; do
	mkdir -p "$root/build/$t/0.1.0"
	tool "$root/build/$t/0.1.0/rhythm-$t" 0.1.0
	release "$t" 0.1.0 "{{BASE}}/dl/$t/v0.1.0"
	cp "$root/repos/$owner/$t/releases/tags/v0.1.0" "$root/repos/$owner/$t/releases/latest"
done
rm -rf "$root/build"
