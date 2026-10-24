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

# The PATH every case gets unless it sets $path. A cosign or gh found there
# would check the stand-in bundle for real, so then the cases get a copy of
# those folders without them.
syspath=/usr/local/bin:/usr/bin:/bin
if env -i PATH="$syspath" sh -c 'command -v cosign || command -v gh' >/dev/null 2>&1; then
	mkdir -p "$work/sysbin"
	for d in /usr/local/bin /usr/bin /bin; do
		for f in "$d"/*; do
			b=${f##*/}
			case $b in
			cosign | gh) continue ;;
			esac
			if [ -x "$f" ] && [ ! -e "$work/sysbin/$b" ]; then
				ln -s "$f" "$work/sysbin/$b"
			fi
		done
	done
	syspath=$work/sysbin
fi

# Stand-ins for cosign and gh, each in its own folder so a case can have
# either. They log their argv to $FAKE_SIG_LOG. cosign exits
# $FAKE_COSIGN_EXIT; gh is logged in unless FAKE_GH_AUTH=1, has
# --source-ref unless FAKE_GH_OLD=1 and its attestation check exits
# $FAKE_GH_EXIT.
mkdir -p "$work/cosignbin" "$work/ghbin"
cat >"$work/cosignbin/cosign" <<'EOF'
#!/bin/sh
printf 'cosign %s\n' "$*" >>"$FAKE_SIG_LOG"
[ "${FAKE_COSIGN_EXIT:-0}" = 0 ] || echo 'Error: none of the expected identities matched what was in the certificate' >&2
exit "${FAKE_COSIGN_EXIT:-0}"
EOF
cat >"$work/ghbin/gh" <<'EOF'
#!/bin/sh
case "$*" in
"auth status") exit "${FAKE_GH_AUTH:-0}" ;;
"attestation verify --help")
	[ "${FAKE_GH_OLD:-}" = 1 ] || echo '      --source-ref string   Enforce that the git ref associated with the source repository matches the provided value'
	exit 0
	;;
esac
printf 'gh %s\n' "$*" >>"$FAKE_SIG_LOG"
[ "${FAKE_GH_EXIT:-0}" = 0 ] || echo 'Error: no attestations found' >&2
exit "${FAKE_GH_EXIT:-0}"
EOF
chmod 0755 "$work/cosignbin/cosign" "$work/ghbin/gh"

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

# A curl stand-in that answers every request with HTTP 403, for the rate
# limit at api.github.com. It sends nothing.
mkdir -p "$work/ratebin"
cat >"$work/ratebin/curl" <<'EOF'
#!/bin/sh
[ -t 0 ] || cat >/dev/null
printf 403
EOF
chmod 0755 "$work/ratebin/curl"

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
		HOME="$cdir/home" PATH="${path:-$syspath}" TMPDIR="$cdir/tmp" \
		XDG_CONFIG_HOME="$cdir/home/.config" XDG_CACHE_HOME="$cdir/home/.cache" \
		RHYTHM_GITHUB_API="$base" RHYTHM_INSTALL_DIR="$cdir/bin" \
		RHYTHM_TEST_LOG="$cdir/argv.log" FAKE_CURL_LOG="$cdir/curl.log" FAKE_SIG_LOG="$cdir/sig.log" \
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
	expect_out "Installed rhythm 0.7.0 to $cdir/bin"
	grep -qx 'rhythm install --defaults --dry-run' "$cdir/argv.log" || fail "the router wasn't asked for its defaults"
	grep -qx "rhythm install echo --dir $cdir/bin --quiet" "$cdir/argv.log" || fail "the router wasn't told --quiet"
	expect_started echo soundcheck tuner
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

case_commas() {
	# Commas separate names, as in rhythm's own lists.
	for v in 'tuner,echo' 'tuner, echo' ',tuner,,echo,' 'tuner,none echo'; do
		rm -rf "${cdir:?}/bin"
		install RHYTHM_TOOLS="$v"
		expect_code 0
		expect_tools tuner echo
		expect_started tuner echo
	done
}

case_typo() {
	# Every name is checked before any tool is downloaded.
	: >"$work/requests.log"
	install RHYTHM_TOOLS='tuner sondcheck'
	expect_code 1
	expect_out 'rhythm: unknown tool "sondcheck" (did you mean soundcheck?)'
	expect_out '  known: echo, soundcheck, tuner'
	expect_out "rhythm install: rhythm 0.7.0 doesn't know sondcheck; run '$cdir/bin/rhythm help' to see the tools"
	expect_file rhythm
	expect_tools
	! requests | grep -q '/dl/tuner/' || fail "tuner was downloaded before the names were checked"
	grep -qx 'rhythm install --dry-run tuner sondcheck' "$cdir/argv.log" || fail "the names weren't checked in one call"

	install RHYTHM_TOOLS='tunr,sondcheck,nope'
	expect_code 1
	expect_out 'rhythm: unknown tool "tunr" (did you mean tuner?)'
	expect_out 'rhythm: unknown tool "nope"'
	expect_out "rhythm install: rhythm 0.7.0 doesn't know tunr, sondcheck, nope;"
}

case_tool_fails() {
	# A tool whose archive is missing doesn't stop the next one.
	mv "$work/srv/dl/soundcheck" "$work/srv/dl/soundcheck.off"
	install
	mv "$work/srv/dl/soundcheck.off" "$work/srv/dl/soundcheck"
	expect_code 1
	expect_tools tuner echo
	expect_out "rhythm-soundcheck  downloading soundcheck_"
	expect_out ": HTTP 404"
	expect_started echo tuner
	[ "$(tail -n 1 "$cdir/out")" = "Installed: echo, tuner. Failed: soundcheck (see above)." ] ||
		fail "the last line isn't the summary"

	# Nothing installed at all.
	mv "$work/srv/dl/tuner" "$work/srv/dl/tuner.off"
	install RHYTHM_TOOLS=tuner
	mv "$work/srv/dl/tuner.off" "$work/srv/dl/tuner"
	expect_code 1
	expect_no_out "Install a tool"
	expect_out "Installed: none. Failed: tuner (see above)."
}

case_defaults() {
	# Unset RHYTHM_TOOLS takes the router's own list of defaults.
	install RHYTHM_TEST_DEFAULTS='soundcheck echo'
	expect_code 0
	grep -qx 'rhythm install --defaults --dry-run' "$cdir/argv.log" || fail "the router wasn't asked for its defaults"
	expect_tools soundcheck echo
	expect_started soundcheck echo

	# A router without --defaults gets the built-in list, checked at once.
	rm -rf "${cdir:?}/bin" "$cdir/argv.log"
	install RHYTHM_VERSION=0.6.0
	expect_code 0
	grep -qx 'rhythm install --defaults --dry-run' "$cdir/argv.log" || fail "the old router wasn't asked for its defaults"
	grep -qx 'rhythm install --dry-run tuner soundcheck echo' "$cdir/argv.log" ||
		fail "the built-in list wasn't checked in one call"
	expect_tools tuner soundcheck echo
	expect_started tuner soundcheck echo

	# An explicit list never asks for the defaults.
	rm -f "$cdir/argv.log"
	install RHYTHM_TOOLS=tuner
	expect_code 0
	! grep -q -- '--defaults' "$cdir/argv.log" || fail "the defaults were asked for with RHYTHM_TOOLS set"
}

case_pins() {
	# tool@version reaches rhythm as it was written.
	install RHYTHM_TOOLS='tuner@0.3.1,soundcheck'
	expect_code 0
	grep -qx 'rhythm install --dry-run tuner@0.3.1 soundcheck' "$cdir/argv.log" || fail "the pin wasn't checked as written"
	grep -qx "rhythm install tuner@0.3.1 --dir $cdir/bin --quiet" "$cdir/argv.log" || fail "the pin didn't reach rhythm install"
	[ "$("$cdir/bin/rhythm-tuner" --version)" = "rhythm-tuner 0.3.1" ] || fail "tuner isn't the pinned 0.3.1"
	expect_tools tuner soundcheck
	expect_started tuner soundcheck

	# A pin and the plain name are one tool.
	rm -rf "${cdir:?}/bin" "$cdir/argv.log"
	install RHYTHM_TOOLS='t@0.3.1 tuner'
	expect_code 0
	[ "$(grep -c '^rhythm install t\(uner\)\{0,1\}[@ ]' "$cdir/argv.log")" = 1 ] || fail "tuner was installed more than once"

	# A rhythm that can't pin says so before installing any tool.
	rm -rf "${cdir:?}/bin"
	install RHYTHM_VERSION=0.6.0 RHYTHM_TOOLS='tuner@0.3.1 echo'
	expect_code 1
	expect_out "rhythm install: rhythm 0.6.0 doesn't know tuner@0.3.1; it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version"
	expect_tools
	install RHYTHM_VERSION=0.5.0 RHYTHM_TOOLS='echo tuner@0.3.1'
	expect_code 1
	expect_out "rhythm install: rhythm 0.5.0 doesn't know tuner@0.3.1; it can't pin tool versions"
	expect_tools
}

case_locked() {
	cat >"$cdir/rhythm.lock" <<'EOF'
{
  "schema": "rhythm-lock/v1",
  "rhythm": {"name": "rhythm", "repo": "RhythmShahriar/rhythm", "version": "0.7.0", "platforms": {}},
  "tools": [
    {"name": "tuner", "repo": "RhythmShahriar/tuner", "version": "0.3.1", "platforms": {}},
    {"name": "echo", "repo": "RhythmShahriar/echo", "version": "0.1.0", "platforms": {}}
  ]
}
EOF
	install RHYTHM_LOCKED=1
	expect_code 0
	grep -qx "rhythm install --locked --dir $cdir/bin --quiet" "$cdir/argv.log" || fail "rhythm wasn't asked to install --locked"
	! grep -q -- '--defaults' "$cdir/argv.log" || fail "the defaults were asked for with RHYTHM_LOCKED=1"
	[ "$("$cdir/bin/rhythm-tuner" --version)" = "rhythm-tuner 0.3.1" ] || fail "tuner isn't the locked 0.3.1"
	expect_tools tuner echo
	expect_started tuner echo

	# RHYTHM_TOOLS gives way to the lock.
	install RHYTHM_LOCKED=1 RHYTHM_TOOLS=soundcheck
	expect_code 0
	expect_out "warning: RHYTHM_TOOLS is ignored: RHYTHM_LOCKED installs the tools in rhythm.lock"
	expect_tools tuner echo

	# A tool the lock names that fails is in the summary.
	mv "$work/srv/dl/echo" "$work/srv/dl/echo.off"
	install RHYTHM_LOCKED=1
	mv "$work/srv/dl/echo.off" "$work/srv/dl/echo"
	expect_code 1
	expect_out "Installed: tuner. Failed: echo (see above)."

	# A tool rhythm says is already installed gets its first step too.
	install RHYTHM_LOCKED=1 RHYTHM_TEST_ALREADY=tuner
	expect_code 0
	expect_out "rhythm-tuner 0.3.1 is already installed at $cdir/bin/rhythm-tuner (use --force to reinstall)"
	expect_started tuner echo

	# A rhythm without --locked can't.
	rm -rf "${cdir:?}/bin"
	install RHYTHM_LOCKED=1 RHYTHM_VERSION=0.6.0
	expect_code 1
	expect_out "rhythm install: rhythm 0.6.0 can't install from rhythm.lock; set RHYTHM_VERSION to a newer rhythm"
	expect_tools

	# RHYTHM_LOCKED=0 is off; with no lock RHYTHM_LOCKED=1 stops.
	rm -f "$cdir/argv.log"
	install RHYTHM_LOCKED=0 RHYTHM_TOOLS=tuner
	expect_code 0
	! grep -q -- '--locked' "$cdir/argv.log" || fail "RHYTHM_LOCKED=0 used the lock"
	rm "$cdir/rhythm.lock"
	install RHYTHM_LOCKED=1
	expect_code 1
	expect_out "rhythm install: RHYTHM_LOCKED is set but there is no rhythm.lock in $cdir"
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
	expect_out "Installed rhythm 0.7.0 to $cdir/bin"
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

# expect_path_hint checks that the output has exactly one line about PATH
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

case_path_hint_quote() {
	# A ' in the folder must not break the command the hint prints.
	qbin="$cdir/it's/bin"
	install SHELL=/bin/bash RHYTHM_INSTALL_DIR="$qbin"
	expect_code 0
	expect_path_hint "echo 'export PATH=\"$cdir/it'\\''s/bin:\$PATH\"' >> ~/.bashrc"
	cmd=$(sed -n 's/^Add .* to your PATH, then open a new terminal: //p' "$cdir/out")
	mkdir -p "$cdir/qhome"
	HOME="$cdir/qhome" sh -c "$cmd" || fail "the hint isn't a working command: $cmd"
	# shellcheck disable=SC2016 # the line must hold a literal $PATH
	[ "$(cat "$cdir/qhome/.bashrc")" = "export PATH=\"$qbin:\$PATH\"" ] || fail ".bashrc got: $(cat "$cdir/qhome/.bashrc")"

	install SHELL=/usr/bin/fish RHYTHM_INSTALL_DIR="$qbin"
	expect_code 0
	expect_path_hint "Add $qbin to your PATH: fish_add_path '$cdir/it'\\''s/bin'"
}

case_path_hint_macos() {
	# A macOS terminal starts bash as a login shell, which reads ~/.bash_profile.
	mkdir -p "$work/darwin"
	# shellcheck disable=SC2016 # $1 belongs to the stand-in
	printf '#!/bin/sh\ncase $1 in -s) echo Darwin ;; *) echo arm64 ;; esac\n' >"$work/darwin/uname"
	chmod 0755 "$work/darwin/uname"
	path=$work/darwin:$syspath
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
	path=$cdir/bin:$syspath
	install SHELL=/bin/zsh
	path=
	expect_code 0
	expect_no_out PATH

	# A rhythm found first on PATH would shadow the new one.
	mkdir -p "$cdir/old"
	printf '#!/bin/sh\necho rhythm 0.1.0\n' >"$cdir/old/rhythm"
	chmod 0755 "$cdir/old/rhythm"
	path=$cdir/old:$cdir/bin:$syspath
	install SHELL=/bin/zsh
	path=
	expect_code 0
	expect_path_hint "Note: $cdir/old/rhythm comes first on your PATH; remove it to use this install."
}

# expect_no_install checks that nothing landed in the install folder.
expect_no_install() {
	[ ! -e "$cdir/bin" ] || [ -z "$(ls -A "$cdir/bin")" ] || fail "$1 installed $(ls "$cdir/bin")"
}

case_signature_cosign() {
	path=$work/cosignbin:$syspath
	install
	expect_code 0
	expect_out "Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (cosign)"
	expect_no_out "signature not checked"
	expect_tools tuner soundcheck echo
	# The same check README tells people to run, flag for flag.
	sed -n '/^cosign verify-blob/,/^  checksums.txt$/p' "$here/../../README.md" | grep -o -- '--[a-z-]* [^ ]*' |
		sed 's|v<version>|v0.7.0|' >"$cdir/readme.flags"
	[ "$(wc -l <"$cdir/readme.flags")" = 5 ] || fail "README's cosign command has $(wc -l <"$cdir/readme.flags") flags, want 5"
	while read -r flag; do
		case $flag in
		--bundle*) flag="--bundle $cdir/tmp/" ;;
		esac
		grep -qF -- "$flag" "$cdir/sig.log" || fail "cosign wasn't given $flag"
	done <"$cdir/readme.flags"
	grep -q ' --certificate-github-workflow-repository RhythmShahriar/rhythm ' "$cdir/sig.log" || fail "the repository isn't pinned"
	grep -q ' --certificate-github-workflow-ref refs/tags/v0.7.0 ' "$cdir/sig.log" || fail "the tag isn't pinned"
	grep -q '/checksums.txt$' "$cdir/sig.log" || fail "cosign didn't check checksums.txt"

	# A pinned version is the tag the signature must name.
	rm -f "$cdir/sig.log"
	install RHYTHM_VERSION=0.6.0
	path=
	expect_code 0
	grep -q ' --certificate-github-workflow-ref refs/tags/v0.6.0 ' "$cdir/sig.log" || fail "the pinned tag isn't checked"
}

case_signature_cosign_fails() {
	path=$work/cosignbin:$syspath
	install FAKE_COSIGN_EXIT=1
	path=
	expect_code 1
	expect_out "Error: none of the expected identities matched"
	expect_out "rhythm install: cosign says checksums.txt wasn't signed for RhythmShahriar/rhythm v0.7.0; nothing was installed"
	expect_no_install "a failed signature"
}

case_signature_gh() {
	path=$work/ghbin:$syspath
	install
	expect_code 0
	expect_out "Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (gh attestation)"
	grep -q "^gh attestation verify $cdir/tmp/.*/checksums.txt --repo RhythmShahriar/rhythm --signer-workflow RhythmShahriar/.github/.github/workflows/release-go.yml --source-ref refs/tags/v0.7.0\$" "$cdir/sig.log" ||
		fail "gh wasn't asked to verify checksums.txt for the repository and tag"

	rm -rf "${cdir:?}/bin"
	install FAKE_GH_EXIT=1
	expect_code 1
	expect_out "rhythm install: gh found no attestation that RhythmShahriar/rhythm v0.7.0 made this checksums.txt; nothing was installed."
	expect_no_install "a failed attestation"

	# gh without a login can't ask for attestations; one without
	# --source-ref can't pin the tag. The line says which.
	rm -f "$cdir/sig.log"
	install FAKE_GH_AUTH=1
	expect_code 0
	expect_out "signature not checked (gh is not logged in); install cosign or gh to check it before anything runs"
	expect_no_out "no cosign or gh"
	expect_no_out "warning: signature"
	[ ! -e "$cdir/sig.log" ] || fail "gh ran without a login"
	install FAKE_GH_OLD=1
	path=
	expect_code 0
	expect_out "signature not checked (gh is too old to pin the tag); install cosign or gh to check it before anything runs"
	[ ! -e "$cdir/sig.log" ] || fail "gh ran without --source-ref"
}

case_signature_none() {
	install
	expect_code 0
	expect_out "signature not checked (no cosign or gh); install cosign or gh to check it before anything runs"
	expect_tools tuner soundcheck echo

	# cosign can't check a release without a bundle; gh isn't there.
	path=$work/cosignbin:$syspath
	install RHYTHM_VERSION=9.0.4
	path=
	expect_code 0
	expect_out "warning: signature not checked (the release has no checksums.txt.sigstore.json); install cosign or gh to check it before anything runs"
	[ ! -e "$cdir/sig.log" ] || fail "cosign ran without a bundle"

	# The same with a gh that can't be used names both reasons.
	path=$work/cosignbin:$work/ghbin:$syspath
	install RHYTHM_VERSION=9.0.4 FAKE_GH_AUTH=1
	path=
	expect_code 0
	expect_out "warning: signature not checked (the release has no checksums.txt.sigstore.json and gh is not logged in);"

	# gh still checks a release without a bundle.
	path=$work/cosignbin:$work/ghbin:$syspath
	install RHYTHM_VERSION=9.0.4
	path=
	expect_code 0
	expect_out "Signature ok  RhythmShahriar/rhythm @ refs/tags/v9.0.4 (gh attestation)"
	expect_no_out "signature not checked"
}

case_skip_signature() {
	path=$work/cosignbin:$syspath
	install RHYTHM_SKIP_SIGNATURE=1 FAKE_COSIGN_EXIT=1
	expect_code 0
	expect_out "warning: RHYTHM_SKIP_SIGNATURE is set: nothing checks that this release was signed by RhythmShahriar/rhythm. Use it only with a mirror you trust."
	[ ! -e "$cdir/sig.log" ] || fail "cosign ran with RHYTHM_SKIP_SIGNATURE=1"
	! grep -q '^rhythm verify' "$cdir/argv.log" || fail "rhythm verify ran with RHYTHM_SKIP_SIGNATURE=1"
	expect_tools tuner soundcheck echo

	install RHYTHM_SKIP_SIGNATURE=0
	path=
	expect_code 0
	expect_no_out RHYTHM_SKIP_SIGNATURE
	[ -s "$cdir/sig.log" ] || fail "RHYTHM_SKIP_SIGNATURE=0 skipped cosign"
}

case_router_verify() {
	install
	expect_code 0
	grep -qx 'rhythm verify rhythm' "$cdir/argv.log" || fail "rhythm didn't verify itself"
	expect_out "Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (release-go.yml)"

	# A rhythm without the command isn't asked.
	rm -rf "${cdir:?}/bin" "$cdir/argv.log"
	install RHYTHM_VERSION=0.6.0
	expect_code 0
	! grep -q '^rhythm verify' "$cdir/argv.log" || fail "a rhythm without verify was asked to verify"

	# A failed self-check puts the previous rhythm back and installs no tool.
	rm -rf "${cdir:?}/bin"/rhythm-*
	install RHYTHM_TEST_VERIFY=fail
	expect_code 1
	expect_out "signature not valid: RhythmShahriar/rhythm 0.7.0"
	expect_out "rhythm install: the new rhythm failed its own check (rhythm verify rhythm), so the previous one was put back; nothing was installed"
	[ "$("$cdir/bin/rhythm" --version)" = "rhythm 0.6.0" ] || fail "the previous rhythm wasn't put back"
	expect_tools

	# Without a previous one, the new one is removed.
	rm -rf "${cdir:?}/bin"
	install RHYTHM_TEST_VERIFY=fail
	expect_code 1
	expect_out "so it was removed; nothing was installed"
	expect_no_install "a failed self-check"
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
	grep -qF -- "--proto =https --proto-redir =https --tlsv1.2 --connect-timeout 30 --speed-limit 1 --speed-time 30 " "$cdir/curl.log" ||
		fail "curl ran without --proto =https --proto-redir =https --tlsv1.2 and the timeouts"
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

	# The host ends at '?' or '#', as curl reads it, so loopback text after
	# them doesn't make another host loopback.
	install RHYTHM_VERSION=9.0.6
	expect_code 1
	expect_out "refusing to download over http: http://example.invalid?@127.0.0.1/dl/rhythm/v9.0.6/"
	expect_no_file rhythm
}

case_redirect_to_http() {
	# A loopback mirror may use plain http itself; a redirect from it
	# to plain http elsewhere (here localhost on the same port, which is
	# another host to curl) is refused, so nothing reaches that host.
	port=${base##*:}
	before=$(requests | wc -l)
	install RHYTHM_GITHUB_API="$base/redirect/localhost:$port" RHYTHM_GITHUB_TOKEN=mirror-test-secret
	expect_code 1
	expect_out "rhythm install: 127.0.0.1 redirected to a URL that isn't https ($base/redirect/localhost:$port/repos/RhythmShahriar/rhythm/releases/latest)"
	expect_no_out mirror-test-secret
	expect_no_file rhythm
	got=$(requests | tail -n +$((before + 1)))
	[ "$got" = "GET /redirect/localhost:$port/repos/RhythmShahriar/rhythm/releases/latest auth=Bearer mirror-test-secret" ] ||
		fail "requests were: $got"
}

case_api_not_plain() {
	# Anything after '?' or '#' isn't the host curl connects to, so these
	# would send GITHUB_TOKEN, or plain http, to x.invalid. They are refused
	# before any request, with user info and backslashes.
	for api in 'https://x.invalid?@api.github.com' 'https://x.invalid#@api.github.com' \
		'http://x.invalid?@127.0.0.1' 'http://x.invalid#@localhost' 'https://user@api.github.com' \
		'https://api.github.com\@x.invalid' "$base?x=1"; do
		path=$work/fakebin:/usr/bin:/bin
		rm -f "$cdir/curl.log"
		install RHYTHM_GITHUB_API="$api" GITHUB_TOKEN=gh-test-secret
		path=
		expect_code 1
		expect_out "rhythm install: RHYTHM_GITHUB_API must be a plain base URL, without user info, query or fragment: $api"
		[ ! -e "$cdir/curl.log" ] || fail "curl ran for $api"
	done
}

case_version_checked() {
	# RHYTHM_VERSION goes into the release URL only once it looks like one.
	for v in '../../x' '0.5.0?x=1' '0.5.0#x' '0.5.0/../../latest' 'x1' 'V0.5.0' '0.5 .0'; do
		: >"$work/requests.log"
		install RHYTHM_VERSION="$v"
		expect_code 1
		expect_out "rhythm install: RHYTHM_VERSION isn't a version: '$v'"
		[ ! -s "$work/requests.log" ] || fail "RHYTHM_VERSION=$v made a request"
	done
	install RHYTHM_VERSION=v0.6.0 RHYTHM_TOOLS=none
	expect_code 0
	expect_out "Installed rhythm 0.6.0 to $cdir/bin"
}

case_timeout() {
	# A server that never answers is given up on, with a clear error.
	install RHYTHM_GITHUB_API="$base/hang" RHYTHM_INSTALL_TIMEOUT=2
	expect_code 1
	expect_out "rhythm install: timed out after 2s waiting for 127.0.0.1 ($base/hang/repos/RhythmShahriar/rhythm/releases/latest); set RHYTHM_INSTALL_TIMEOUT to wait longer"
	install RHYTHM_VERSION=9.0.5 RHYTHM_INSTALL_TIMEOUT=2
	expect_code 1
	expect_out "rhythm install: timed out after 2s downloading $base/hang/dl/rhythm/v9.0.5/rhythm_"
	expect_no_file rhythm

	for t in abc 0 00 -1 1.5 ' ' 3601 99999999999 0099999; do
		path=$work/fakebin:/usr/bin:/bin
		rm -f "$cdir/curl.log"
		install RHYTHM_INSTALL_TIMEOUT="$t"
		path=
		expect_code 1
		case $t in
		3601 | 99999999999 | 0099999) expect_out "rhythm install: RHYTHM_INSTALL_TIMEOUT must be at most 3600 seconds: '$t'" ;;
		*) expect_out "rhythm install: RHYTHM_INSTALL_TIMEOUT must be a whole number of seconds: '$t'" ;;
		esac
		[ ! -e "$cdir/curl.log" ] || fail "curl ran with RHYTHM_INSTALL_TIMEOUT=$t"
	done
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
	# A mirror: GITHUB_TOKEN never goes there, so the advice names
	# RHYTHM_GITHUB_TOKEN. None is given once that token was sent.
	for status in 403 429; do
		install RHYTHM_GITHUB_API="$base/status/$status" GITHUB_TOKEN=gh-test-secret
		expect_code 1
		expect_out "rhythm install: rate limit reached at 127.0.0.1; set RHYTHM_GITHUB_TOKEN"
		expect_no_out "GitHub rate limit"
		expect_no_out "curl:"
		install RHYTHM_GITHUB_API="$base/status/$status" RHYTHM_GITHUB_TOKEN=mirror-test-secret
		expect_code 1
		expect_out "rhythm install: rate limit reached at 127.0.0.1"
		expect_no_out "; set"
		expect_no_out mirror-test-secret
	done

	# api.github.com, through a curl stand-in that answers 403: the advice
	# is GITHUB_TOKEN until one was sent.
	path=$work/ratebin:/usr/bin:/bin
	install RHYTHM_GITHUB_API=
	expect_code 1
	expect_out "rhythm install: GitHub rate limit reached; set GITHUB_TOKEN"
	install RHYTHM_GITHUB_API= GITHUB_TOKEN=gh-test-secret
	path=
	expect_code 1
	expect_out "rhythm install: GitHub rate limit reached"
	expect_no_out "; set"

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
	expect_out "rhythm install: rhythm 0.7.0 doesn't know bogus"
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

# readme_commands prints the rhythm commands README's Commands table names,
# one per line: the word after "rhythm" in each row's first cell.
readme_commands() {
	awk '/^## /{ on = ($0 == "## Commands") } on && /^\| `rhythm [a-z]/ { sub(/[^a-z-].*/, "", $3); print $3 }' "$1" | sort -u
}

# help_commands prints the commands in the Commands block of `rhythm help`.
help_commands() {
	awk '/^Commands:/ { on = 1; next } on && /^[^ ]/ { on = 0 } on && $1 == "rhythm" && $2 ~ /^[a-z]/ { print $2 }' "$1" | sort -u
}

# check_commands fails for a command that help ($2) lists but README ($1)
# doesn't. It also says which README commands this build doesn't have yet.
check_commands() {
	help_commands "$2" >"$cdir/help.cmds"
	readme_commands "$1" >"$cdir/readme.cmds"
	[ -s "$cdir/help.cmds" ] || fail "no commands in the router's help"
	[ -s "$cdir/readme.cmds" ] || fail "no commands in README's Commands table"
	while read -r c; do
		grep -qx "$c" "$cdir/help.cmds" || echo "skip $case: this build's help doesn't list rhythm $c"
	done <"$cdir/readme.cmds"
	while read -r c; do
		grep -qx "$c" "$cdir/readme.cmds" || return 1
	done <"$cdir/help.cmds"
}

case_readme_commands() {
	if ! command -v go >/dev/null 2>&1; then
		echo "skip $case: go isn't on PATH"
		return
	fi
	(cd "$here/../.." && GOWORK=off go build -o "$cdir/rhythm" ./cmd/rhythm) || fail "go build ./cmd/rhythm failed"
	"$cdir/rhythm" help >"$cdir/help.txt" 2>&1 || fail "rhythm help failed"
	check_commands "$here/../../README.md" "$cdir/help.txt" || fail "README's Commands table lacks a command rhythm help lists"

	# The check itself: a command missing from README fails it.
	printf 'Commands:\n  rhythm install <tool>   download a tool\n  rhythm zap   not in README\n' >"$cdir/help-extra.txt"
	! check_commands "$here/../../README.md" "$cdir/help-extra.txt" >/dev/null || fail "a command missing from README passed"
}

# check_links runs the wiki checker's relative-link rule over a README:
# every relative link, href, src and srcset names a file that exists.
# Every #anchor names a heading in the README.
check_links() {
	python3 -I - "$here/../check-wiki.py" "$1" <<'EOF'
import importlib.util
import os
import re
import sys

sys.dont_write_bytecode = True  # no __pycache__ in scripts/
spec = importlib.util.spec_from_file_location("check_wiki", sys.argv[1])
wiki = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wiki)
readme = sys.argv[2]
root = os.path.dirname(readme)
anchors = wiki.headings(readme)
targets = list(wiki.links(readme))
with open(readme, encoding="utf-8") as f:
    for no, line in enumerate(f, 1):
        targets += [(no, t) for t in re.findall(r'\b(?:src|srcset)="([^"]+)"', line)]
bad = 0
for no, target in targets:
    if re.match(r"^[a-z][a-z0-9+.-]*:", target) or target.startswith(("/", "mailto:")):
        continue  # external
    name, _, anchor = target.partition("#")
    if name and not os.path.exists(os.path.join(root, name)):
        print(f"README.md:{no}: link to missing file {name!r}")
        bad += 1
    elif not name and anchor not in anchors:
        print(f"README.md:{no}: no heading #{anchor}")
        bad += 1
sys.exit(1 if bad else 0)
EOF
}

case_readme_links() {
	check_links "$here/../../README.md" || fail "README has a broken relative link"

	# The check itself: a missing file and a missing heading both fail it.
	mkdir -p "$cdir/docs"
	printf '# Top\n\n[ok](#top) [gone](docs/missing.svg)\n' >"$cdir/README.md"
	! check_links "$cdir/README.md" >/dev/null || fail "a link to a missing file passed"
	printf '# Top\n\n<img src="docs">[ok](#top) [gone](#nowhere)\n' >"$cdir/README.md"
	! check_links "$cdir/README.md" >/dev/null || fail "a link to a missing heading passed"
	printf '# Top\n\n<img src="docs">[ok](#top) [web](https://example.invalid/x)\n' >"$cdir/README.md"
	check_links "$cdir/README.md" || fail "a good README failed"
}

cases=${*:-default none some commas typo tool_fails defaults pins locked path_hint path_hint_quote path_hint_macos path_ok
signature_cosign signature_cosign_fails signature_gh signature_none skip_signature router_verify bad_checksum no_checksums missing_dir relative_dir unwritable_dir truncated proto http_refused http_download_refused redirect_to_http api_not_plain version_checked timeout github_token
rhythm_token rate_limit old_router old_router_named router_without_dry_run
router_without_tools_block unknown_tool readme readme_commands readme_links}
script='' path=''
for case in $cases; do
	cdir=$work/cases/$case
	mkdir -p "$cdir"
	"case_$case"
	echo "ok   $case"
done
echo "all installer cases passed with $shell"
