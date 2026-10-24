#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Tests scripts/install.sh against a fake GitHub API on 127.0.0.1:
#   sh scripts/test/run.sh [case ...]
#
# Runs every case or only the named ones (e.g. "old_router") and exits
# non-zero on the first failure. Each case pipes install.sh into a shell, as
# `curl | sh` does, inside env -i with HOME, XDG and TMPDIR in a temp folder.
#
# Environment:
#   RHYTHM_TEST_SH    the shell that runs install.sh (default sh; try dash, bash, "busybox sh")
#   RHYTHM_TEST_PORT  port for the fake API (default: any free port)
#   RHYTHM_TEST_KEEP  set to 1 to keep the temp folder for a look afterwards
set -eu

here=$(cd "$(dirname "$0")" && pwd)
installer=$here/../install.sh
shell=${RHYTHM_TEST_SH:-sh}
work=$(mktemp -d)
server=

cleanup() {
	[ -z "$server" ] || kill "$server" 2>/dev/null || :
	if [ "${RHYTHM_TEST_KEEP:-}" = 1 ]; then
		echo "kept $work" >&2
	else
		rm -rf "$work"
	fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fail() {
	printf 'FAIL %s: %s\n' "$case" "$*" >&2
	if [ -f "$cdir/out" ]; then
		echo '--- installer output' >&2
		cat "$cdir/out" >&2
	fi
	exit 1
}

sh "$here/mkfake.sh" "$work/srv"
python3 -I "$here/server.py" "$work/srv" "$work/requests.log" "${RHYTHM_TEST_PORT:-0}" 2>"$work/server.err" &
server=$!
i=0
until [ -s "$work/srv/.port" ]; do
	i=$((i + 1))
	if [ $i -gt 100 ] || ! kill -0 "$server" 2>/dev/null; then
		cat "$work/server.err" >&2
		echo "the fake API didn't start" >&2
		exit 1
	fi
	sleep 0.1
done
base=http://127.0.0.1:$(cat "$work/srv/.port")

# A curl stand-in for the cases that must not reach the network. It logs its
# argv (and its stdin when given -K -), then fails like an unknown host.
mkdir -p "$work/fakebin"
cat >"$work/fakebin/curl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"$FAKE_CURL_LOG"
case " $* " in
*" -K - "*) cat >>"$FAKE_CURL_LOG" ;;
esac
exit 6
EOF
chmod 0755 "$work/fakebin/curl"

# install runs the installer for the current case with the defaults below,
# then the VAR=value arguments. It sets $code and leaves the output in
# $cdir/out. $script overrides the installer, for the truncation cases.
install() {
	mkdir -p "$cdir/home" "$cdir/tmp"
	set +e
	# $shell is split on purpose, so RHYTHM_TEST_SH can be "busybox sh".
	# It runs in $cdir, so a relative RHYTHM_INSTALL_DIR lands in the case.
	# shellcheck disable=SC2086
	(cd "$cdir" && cat "${script:-$installer}" | env -i \
		HOME="$cdir/home" PATH="${path:-/usr/local/bin:/usr/bin:/bin}" TMPDIR="$cdir/tmp" \
		XDG_CONFIG_HOME="$cdir/home/.config" XDG_CACHE_HOME="$cdir/home/.cache" \
		RHYTHM_GITHUB_API="$base" RHYTHM_INSTALL_DIR="$cdir/bin" \
		RHYTHM_TEST_LOG="$cdir/argv.log" FAKE_CURL_LOG="$cdir/curl.log" \
		"$@" $shell >"$cdir/out" 2>&1)
	code=$?
	set -e
}

# requests prints the fake API's log lines for the current case.
requests() { [ ! -f "$work/requests.log" ] || cat "$work/requests.log"; }

expect_code() { [ "$code" = "$1" ] || fail "exit code $code, want $1"; }
expect_out() { grep -qF -- "$1" "$cdir/out" || fail "output lacks: $1"; }
expect_no_out() { ! grep -qF -- "$1" "$cdir/out" || fail "output has: $1"; }
expect_file() { [ -x "$cdir/bin/$1" ] || fail "$1 wasn't installed"; }
expect_no_file() { [ ! -e "$cdir/bin/$1" ] || fail "$1 was installed"; }
expect_tools() {
	for t in tuner soundcheck echo; do
		case " $* " in
		*" $t "*) expect_file "rhythm-$t" ;;
		*) expect_no_file "rhythm-$t" ;;
		esac
	done
}

# expect_started checks the closing lines: one first step for each tool
# named, in order, or the "Install a tool" line for none. Never a real site.
expect_started() {
	expect_no_out example.com
	expect_out "Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use"
	if [ $# -eq 0 ]; then
		expect_out "Install a tool: rhythm install tuner"
		expect_no_out "Get started"
		return
	fi
	expect_no_out "Install a tool"
	want=
	for t in "$@"; do
		case $t in
		tuner) want="$want  rhythm tuner scan ./your-site|" ;;
		soundcheck) want="$want  rhythm soundcheck snapshot http://localhost:8080|" ;;
		echo) want="$want  rhythm echo up|" ;;
		esac
	done
	got=$(sed -n '/^Get started:$/,$p' "$cdir/out" | grep '^  rhythm ' | tr '\n' '|')
	[ "$got" = "$want" ] || fail "first steps are '$got', want '$want'"
}

case_default() {
	install
	expect_code 0
	expect_file rhythm
	expect_tools tuner soundcheck echo
	expect_out "Installed rhythm 0.6.0 to $cdir/bin"
	grep -qx 'rhythm install --dry-run tuner' "$cdir/argv.log" || fail "the router wasn't asked about tuner"
	grep -qx "rhythm install echo --dir $cdir/bin --quiet" "$cdir/argv.log" || fail "the router wasn't told --quiet"
	expect_started tuner soundcheck echo
}

case_none() {
	for v in none NONE ' None ' ''; do
		rm -rf "${cdir:?}/bin"
		install RHYTHM_TOOLS="$v"
		expect_code 0
		expect_file rhythm
		expect_tools
		expect_started
	done
}

case_some() {
	install RHYTHM_TOOLS='soundcheck  echo'
	expect_code 0
	expect_tools soundcheck echo
	expect_started soundcheck echo

	rm -rf "${cdir:?}/bin"
	install RHYTHM_TOOLS=tuner
	expect_code 0
	expect_tools tuner
	expect_started tuner

	# Aliases name the same tool, which gets one line.
	rm -rf "${cdir:?}/bin" "${cdir:?}/argv.log"
	install RHYTHM_TOOLS='e t echo'
	expect_code 0
	expect_tools tuner echo
	expect_started echo tuner
	[ "$(grep -c '^rhythm install \(e\|echo\) ' "$cdir/argv.log")" = 1 ] || fail "echo was installed more than once"
}

case_bad_checksum() {
	: >"$work/requests.log"
	install RHYTHM_VERSION=9.0.2
	expect_code 1
	expect_out "checksum mismatch for rhythm_"
	expect_out "; nothing was installed"
	[ ! -e "$cdir/bin" ] || fail "a wrong checksum created $cdir/bin"
	! requests | grep -q 'releases/latest' || fail "the installer asked for another release"
}

case_no_checksums() {
	: >"$work/requests.log"
	install RHYTHM_VERSION=9.0.3
	expect_code 1
	expect_out "rhythm install: the release has no checksums.txt; refusing to install unverified binaries"
	[ ! -e "$cdir/bin" ] || fail "a release without checksums created $cdir/bin"
	! requests | grep -q '/dl/' || fail "the installer downloaded from a release without checksums"
}

case_missing_dir() {
	install RHYTHM_INSTALL_DIR="$cdir/bin/a/b" RHYTHM_TOOLS=tuner
	expect_code 0
	[ -x "$cdir/bin/a/b/rhythm" ] || fail "rhythm wasn't installed into a new nested folder"
	[ -x "$cdir/bin/a/b/rhythm-tuner" ] || fail "tuner wasn't installed into a new nested folder"
}

case_relative_dir() {
	install RHYTHM_INSTALL_DIR=bin RHYTHM_TOOLS=tuner
	expect_code 0
	expect_file rhythm
	expect_tools tuner
	expect_out "Installed rhythm 0.6.0 to $cdir/bin"
	grep -qx "rhythm install tuner --dir $cdir/bin --quiet" "$cdir/argv.log" || fail "rhythm wasn't given the absolute folder"
}

case_unwritable_dir() {
	# root ignores the mode.
	if [ "$(id -u)" = 0 ]; then
		echo "skip $case"
		return
	fi
	mkdir -p "$cdir/ro"
	chmod 0555 "$cdir/ro"
	install RHYTHM_INSTALL_DIR="$cdir/ro/bin"
	chmod 0755 "$cdir/ro"
	expect_code 1
	expect_out "rhythm install: can't create $cdir/ro/bin"
}

# expect_path_hint checks that the output has exactly one line about PATH,
# and that it is $1.
expect_path_hint() {
	n=$(grep -c PATH "$cdir/out" || :)
	[ "$n" = 1 ] || fail "$n lines mention PATH, want 1"
	expect_out "$1"
}

case_path_hint() {
	export_line="Add $cdir/bin to your PATH, then open a new terminal: echo 'export PATH=\"$cdir/bin:\$PATH\"' >>"
	for spec in /bin/zsh:.zshrc /usr/local/bin/zsh:.zshrc /bin/bash:.bashrc /bin/dash:.profile :.profile; do
		install SHELL="${spec%%:*}"
		expect_code 0
		expect_path_hint "$export_line ~/${spec#*:}"
	done

	install SHELL=/usr/bin/fish
	expect_code 0
	expect_path_hint "Add $cdir/bin to your PATH: fish_add_path '$cdir/bin'"

	# No SHELL at all, as in env -i. bash fills it in from the user's
	# login shell, so only the line count is certain there.
	install
	expect_code 0
	# shellcheck disable=SC2016,SC2086 # expanded by $shell, split on purpose
	if [ -z "$(env -i $shell -c 'echo "${SHELL:-}"')" ]; then
		expect_path_hint "$export_line ~/.profile"
	else
		expect_path_hint "$export_line ~/."
	fi

	# An older router prints its own note for each tool; it is filtered.
	rm -f "$cdir/argv.log"
	install RHYTHM_VERSION=0.5.0 SHELL=/bin/zsh
	expect_code 0
	expect_tools tuner soundcheck echo
	expect_path_hint "$export_line ~/.zshrc"
	! grep -q 'install .*--quiet' "$cdir/argv.log" || fail "--quiet went to a router without it"
}

case_path_hint_macos() {
	# A macOS terminal starts bash as a login shell, which reads ~/.bash_profile.
	mkdir -p "$work/darwin"
	# shellcheck disable=SC2016 # $1 belongs to the stand-in
	printf '#!/bin/sh\ncase $1 in -s) echo Darwin ;; *) echo arm64 ;; esac\n' >"$work/darwin/uname"
	chmod 0755 "$work/darwin/uname"
	path=$work/darwin:/usr/local/bin:/usr/bin:/bin
	# busybox sh runs its own uname applet whatever PATH says.
	# shellcheck disable=SC2086 # split on purpose
	if [ "$(env -i PATH="$path" $shell -c 'uname -s')" != Darwin ]; then
		path=
		echo "skip $case: $shell ignores a uname on PATH"
		return
	fi
	install SHELL=/bin/bash
	path=
	expect_code 0
	expect_path_hint "echo 'export PATH=\"$cdir/bin:\$PATH\"' >> ~/.bash_profile"
}

case_path_ok() {
	path=$cdir/bin:/usr/local/bin:/usr/bin:/bin
	install SHELL=/bin/zsh
	path=
	expect_code 0
	expect_no_out PATH

	# A rhythm found first on PATH would shadow the new one.
	mkdir -p "$cdir/old"
	printf '#!/bin/sh\necho rhythm 0.1.0\n' >"$cdir/old/rhythm"
	chmod 0755 "$cdir/old/rhythm"
	path=$cdir/old:$cdir/bin:/usr/local/bin:/usr/bin:/bin
	install SHELL=/bin/zsh
	path=
	expect_code 0
	expect_path_hint "Note: $cdir/old/rhythm comes first on your PATH; remove it to use this install."
}

# truncate writes the first $1 percent of install.sh to $2.
truncate() {
	size=$(wc -c <"$installer")
	head -c $((size * $1 / 100)) "$installer" >"$2"
}

case_truncated() {
	for pct in 50 90; do
		script=$cdir/install-$pct.sh
		truncate "$pct" "$script"
		: >"$work/requests.log"
		install
		script=
		[ "$code" != 0 ] || fail "a script cut at $pct% exited 0"
		[ ! -e "$cdir/bin" ] || fail "a script cut at $pct% created $cdir/bin"
		[ ! -s "$work/requests.log" ] || fail "a script cut at $pct% made a request"
	done
}

case_proto() {
	path=$work/fakebin:/usr/bin:/bin
	install RHYTHM_GITHUB_API=
	path=
	expect_code 1
	grep -qF -- "--proto =https --tlsv1.2" "$cdir/curl.log" || fail "curl ran without --proto =https --tlsv1.2"
	grep -qF -- "https://api.github.com/repos/RhythmShahriar/rhythm/releases/latest" "$cdir/curl.log" ||
		fail "the default API isn't api.github.com"
	expect_out "couldn't reach api.github.com"
}

case_http_refused() {
	for api in http://example.invalid HTTP://10.0.0.1:8080 ftp://127.0.0.1 example.invalid; do
		path=$work/fakebin:/usr/bin:/bin
		rm -f "$cdir/curl.log"
		install RHYTHM_GITHUB_API="$api"
		path=
		expect_code 1
		expect_out "rhythm install: RHYTHM_GITHUB_API must be"
		[ ! -e "$cdir/curl.log" ] || fail "curl ran for $api"
	done
}

case_http_download_refused() {
	install RHYTHM_VERSION=9.0.1
	expect_code 1
	expect_out "refusing to download over http: http://example.invalid/dl/rhythm/v9.0.1/"
	expect_no_file rhythm
}

case_github_token() {
	# api.github.com gets GITHUB_TOKEN, on curl's stdin and not its argv.
	path=$work/fakebin:/usr/bin:/bin
	install RHYTHM_GITHUB_API= GITHUB_TOKEN=gh-test-secret
	path=
	expect_code 1
	grep -qx 'header = "Authorization: Bearer gh-test-secret"' "$cdir/curl.log" ||
		fail "GITHUB_TOKEN wasn't sent to api.github.com"
	! grep -F -- "-K -" "$cdir/curl.log" | grep -qF gh-test-secret || fail "the token is in curl's argv"
	expect_no_out gh-test-secret

	# A mirror never sees GITHUB_TOKEN.
	: >"$work/requests.log"
	install GITHUB_TOKEN=gh-test-secret RHYTHM_TOOLS=none
	expect_code 0
	requests | grep -q 'releases/latest auth=-$' || fail "the mirror wasn't asked for the release"
	! requests | grep -qF gh-test-secret || fail "GITHUB_TOKEN reached the mirror"
}

case_rhythm_token() {
	: >"$work/requests.log"
	install GITHUB_TOKEN=gh-test-secret RHYTHM_GITHUB_TOKEN=mirror-test-secret RHYTHM_TOOLS=none
	expect_code 0
	requests | grep -q 'releases/latest auth=Bearer mirror-test-secret$' ||
		fail "RHYTHM_GITHUB_TOKEN wasn't sent to the mirror"
	! requests | grep -qF gh-test-secret || fail "GITHUB_TOKEN reached the mirror"
	! requests | grep '/dl/' | grep -qF mirror-test-secret || fail "the token went to a download"
	expect_no_out mirror-test-secret

	install RHYTHM_GITHUB_TOKEN='bad"token' RHYTHM_TOOLS=none
	expect_code 1
	expect_out "the GitHub token has a character curl can't send in a header"
}

case_rate_limit() {
	for status in 403 429; do
		install RHYTHM_GITHUB_API="$base/status/$status"
		expect_code 1
		expect_out "rhythm install: GitHub rate limit reached; set GITHUB_TOKEN"
		expect_no_out "curl:"
	done
	install RHYTHM_GITHUB_API="$base/status/500"
	expect_code 1
	expect_out "couldn't fetch the release from GitHub ($base/status/500/repos/RhythmShahriar/rhythm/releases/latest): HTTP 500"
}

case_old_router() {
	install RHYTHM_VERSION=0.4.0
	expect_code 0
	expect_out "Installed rhythm 0.4.0 to $cdir/bin"
	expect_out "warning: rhythm 0.4.0 doesn't know echo; skipped"
	expect_tools tuner soundcheck
	! grep -q 'install echo' "$cdir/argv.log" || fail "the router was asked to install echo"
	expect_started tuner soundcheck
}

case_old_router_named() {
	install RHYTHM_VERSION=v0.4.0 RHYTHM_TOOLS='tuner echo'
	expect_code 1
	expect_out "rhythm install: rhythm 0.4.0 doesn't know echo; run '$cdir/bin/rhythm help' to see the tools"
	expect_file rhythm
	expect_tools
}

case_router_without_dry_run() {
	install RHYTHM_VERSION=0.5.0
	expect_code 0
	expect_tools tuner soundcheck echo
	expect_no_out warning
}

case_router_without_tools_block() {
	install RHYTHM_VERSION=0.3.5
	expect_code 0
	expect_tools tuner soundcheck echo
}

case_unknown_tool() {
	install RHYTHM_TOOLS='tuner bogus'
	expect_code 1
	expect_out 'rhythm: unknown tool "bogus"'
	expect_out "rhythm install: rhythm 0.6.0 doesn't know bogus"
	expect_file rhythm
	expect_tools
}

# The README's cosign command must pin the calling repository and tag: the
# workflow identity alone is shared by every repository that calls it.
case_readme() {
	readme=$here/../../README.md
	for flag in '--certificate-oidc-issuer https://token.actions.githubusercontent.com' \
		'--certificate-github-workflow-repository RhythmShahriar/rhythm' \
		'--certificate-github-workflow-ref refs/tags/v<version>' \
		'gh attestation verify checksums.txt --repo RhythmShahriar/rhythm'; do
		grep -qF -- "$flag" "$readme" || fail "README lacks: $flag"
	done
	! grep -qi 'other identity' "$readme" || fail "README claims other identities are rejected"
}

cases=${*:-default none some path_hint path_hint_macos path_ok bad_checksum no_checksums missing_dir relative_dir unwritable_dir truncated proto http_refused http_download_refused github_token
rhythm_token rate_limit old_router old_router_named router_without_dry_run
router_without_tools_block unknown_tool readme}
script='' path=''
for case in $cases; do
	cdir=$work/cases/$case
	mkdir -p "$cdir"
	"case_$case"
	echo "ok   $case"
done
echo "all installer cases passed with $shell"
