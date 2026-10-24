#!/usr/bin/env bash
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# The steps of the setup-rhythm action (action.yml), one per argument:
#
#   setup.sh prepare   check the true/false inputs, then work out the
#                      install and download-cache folders and the cache
#                      key, as step outputs
#   setup.sh rhythm    install rhythm with scripts/install.sh (macOS, Linux)
#   setup.sh tools     install the tools with rhythm install, add the
#                      install folder to PATH and write the outputs
#
# The action passes its inputs as INPUT_* variables. Every step runs from
# INPUT_WORKING_DIRECTORY, where rhythm looks for rhythm.lock. It runs
# under bash on every runner, Git Bash on Windows included.
set -euo pipefail

die() {
	printf 'setup-rhythm: %s\n' "$*" >&2
	exit 1
}

# output appends name=value to the step outputs, or prints it when run
# outside Actions.
output() {
	if [ -n "${GITHUB_OUTPUT:-}" ]; then
		printf '%s=%s\n' "$1" "$2" >>"$GITHUB_OUTPUT"
	else
		printf '%s=%s\n' "$1" "$2"
	fi
}

# truthy reports whether an input is true; anything but true or false is
# a mistake worth stopping for. It is the one rule for these inputs on
# every runner: the Windows step takes skip-signature from prepare.
truthy() {
	case $(printf '%s' "$2" | tr '[:upper:]' '[:lower:]') in
	true | 1 | yes) return 0 ;;
	false | 0 | no | '') return 1 ;;
	esac
	die "$1 must be true or false, got '$2'"
}

os=${RUNNER_OS:-$(uname -s)}
case $os in
Windows | MINGW* | MSYS* | CYGWIN*) os=Windows ;;
macOS | Darwin) os=macOS ;;
*) os=Linux ;;
esac

# winpath turns a Windows path from the environment into one bash and the
# action both read.
winpath() {
	if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}

# install_dir is where rhythm and the tools go.
install_dir() {
	local d=${INPUT_INSTALL_DIR:-}
	if [ -z "$d" ]; then
		d=${RUNNER_TEMP:-${TMPDIR:-/tmp}}/setup-rhythm/bin
	fi
	[ "$os" != Windows ] || d=$(winpath "$d")
	printf '%s' "$d"
}

# cache_dir is config.CacheDir("rhythm")/downloads, where rhythm install
# --cache keeps verified archives (see kit/config).
cache_dir() {
	local base
	if [ -n "${RHYTHM_CACHE_DIR:-}" ]; then
		base=$RHYTHM_CACHE_DIR
	else
		case $os in
		Windows)
			[ -n "${LOCALAPPDATA:-}" ] || die "LOCALAPPDATA isn't set"
			base=$LOCALAPPDATA/rhythm
			;;
		macOS) base=$HOME/Library/Caches/rhythm ;;
		*) base=${XDG_CACHE_HOME:-$HOME/.cache}/rhythm ;;
		esac
	fi
	[ "$os" != Windows ] || base=$(winpath "$base")
	printf '%s/rhythm/downloads' "$base"
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum | cut -d' ' -f1
	else
		shasum -a 256 | cut -d' ' -f1
	fi
}

prepare() {
	local arch lock key skip=
	# A bad value stops here, before any step installs something.
	truthy locked "${INPUT_LOCKED:-false}" || :
	if truthy skip-signature "${INPUT_SKIP_SIGNATURE:-false}"; then
		skip=1
	fi
	arch=${RUNNER_ARCH:-$(uname -m)}
	lock=none
	if [ -f rhythm.lock ]; then
		lock=$(sha256 <rhythm.lock | cut -c1-16)
	fi
	# The inputs are part of the key, so asking for other tools or another
	# rhythm saves a new entry instead of reusing one that lacks them.
	key=$(printf '%s\n%s\n' "${INPUT_VERSION:-}" "${INPUT_TOOLS:-}" | sha256 | cut -c1-16)
	output install-dir "$(install_dir)"
	output cache-dir "$(cache_dir)"
	output cache-key "setup-rhythm-v1-$os-$arch-$lock-$key"
	output cache-prefix "setup-rhythm-v1-$os-$arch-"
	output skip-signature "$skip"
}

# install_rhythm installs the router alone, with the repository's own
# installer at the same ref as this action.
install_rhythm() {
	local here dir
	here=$(cd "$(dirname "$0")" && pwd)
	dir=$(install_dir)
	mkdir -p "$dir"
	export RHYTHM_INSTALL_DIR=$dir RHYTHM_TOOLS=none
	export RHYTHM_VERSION=${INPUT_VERSION:-}
	if truthy skip-signature "${INPUT_SKIP_SIGNATURE:-false}"; then
		export RHYTHM_SKIP_SIGNATURE=1
	fi
	sh "$here/../scripts/install.sh"
}

tools() {
	local dir rhythm v args specs line out status
	dir=$(install_dir)
	rhythm=$dir/rhythm
	[ "$os" != Windows ] || rhythm=$dir/rhythm.exe
	[ -x "$rhythm" ] || die "rhythm isn't in $dir; the install step failed"
	v=$("$rhythm" --version) || die "$rhythm --version failed"
	v=${v##* }
	output rhythm-version "$v"
	if [ -n "${GITHUB_PATH:-}" ]; then
		printf '%s\n' "$dir" >>"$GITHUB_PATH"
	fi

	read -r -a specs <<<"$(printf '%s' "${INPUT_TOOLS:-}" | tr ',' ' ')"
	args=(install --dir "$dir")
	if truthy locked "${INPUT_LOCKED:-false}"; then
		[ -f rhythm.lock ] || die "locked is true but there is no rhythm.lock in $(pwd); write one with: rhythm lock"
		args+=(--locked)
	elif [ ${#specs[@]} -eq 0 ]; then
		output tool-versions ""
		echo "setup-rhythm: rhythm $v installed, no tools asked for"
		return 0
	fi
	if truthy skip-signature "${INPUT_SKIP_SIGNATURE:-false}"; then
		args+=(--skip-signature)
	fi
	# Verified archives are kept in the folder the cache step restores.
	export RHYTHM_DOWNLOAD_CACHE=1
	out=$(mktemp)
	status=0
	"$rhythm" "${args[@]}" ${specs[@]+"${specs[@]}"} | tee "$out" || status=$?
	# tool@version for each tool now in place, installed or already there.
	local versions=()
	while IFS= read -r line; do
		case $line in
		"Installed rhythm-"*" to "*)
			line=${line#Installed rhythm-}
			versions+=("${line%% *}@$(printf '%s' "$line" | cut -d' ' -f2)")
			;;
		"rhythm-"*" is already installed at "*)
			line=${line#rhythm-}
			versions+=("${line%% *}@$(printf '%s' "$line" | cut -d' ' -f2)")
			;;
		esac
	done <"$out"
	rm -f "$out"
	output tool-versions "${versions[*]+"${versions[*]}"}"
	[ "$status" -eq 0 ] || die "rhythm install failed (exit $status); see above"
}

[ $# -eq 1 ] || die "usage: setup.sh prepare|rhythm|tools"
if [ -n "${INPUT_WORKING_DIRECTORY:-}" ]; then
	cd "$INPUT_WORKING_DIRECTORY" || die "can't enter working-directory $INPUT_WORKING_DIRECTORY"
fi
case $1 in
prepare) prepare ;;
rhythm) install_rhythm ;;
tools) tools ;;
*) die "usage: setup.sh prepare|rhythm|tools" ;;
esac
