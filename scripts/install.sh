#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Installs rhythm and its tools from GitHub Releases:
#   curl -fsSL https://rythm.sh/install | sh
#
# Environment:
#   RHYTHM_INSTALL_DIR   where to install (default ~/.local/bin, or /usr/local/bin as root)
#   RHYTHM_VERSION       a specific version, e.g. 0.5.0 (default: latest)
#   RHYTHM_TOOLS         tools to install as well, separated by spaces or commas; tool@version
#                        pins one (default: the tools rhythm installs on a fresh setup,
#                        "tuner soundcheck echo" for an older rhythm; "none" for none)
#   RHYTHM_LOCKED        1 installs the tools in ./rhythm.lock instead (rhythm install --locked)
#   RHYTHM_SKIP_SIGNATURE  1 skips the signature checks, for a mirror you trust
#   RHYTHM_GITHUB_API    GitHub API base, for mirrors (default https://api.github.com).
#                        It must be https; plain http is accepted only for 127.0.0.1, ::1 and localhost.
#   GITHUB_TOKEN         sent to api.github.com only, to lift the anonymous rate limit
#   RHYTHM_GITHUB_TOKEN  sent to the RHYTHM_GITHUB_API host, whatever it is; wins over GITHUB_TOKEN
#
# Everything, helpers included, is inside main, called on the last line, so a
# download cut off halfway through is a syntax error instead of a partial install.

main() {
	set -eu

	REPO=RhythmShahriar/rhythm
	DEFAULT_TOOLS="tuner soundcheck echo"
	# The shared workflow that signs every release. Its identity alone would
	# accept any repository that calls it, so the repository and tag are
	# pinned as well (see "Verify a release" in README).
	WORKFLOW=RhythmShahriar/.github/.github/workflows/release-go.yml
	ISSUER=https://token.actions.githubusercontent.com

	say() { printf '%s\n' "$*" >&2; }
	warn() { say "warning: $*"; }
	die() {
		say "rhythm install: $*"
		exit 1
	}

	# Spelled out: some tr builds, such as BusyBox 1.30, get [:upper:] wrong.
	lower() { printf '%s' "$1" | tr ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz; }

	sha256() {
		if command -v sha256sum >/dev/null 2>&1; then
			sha256sum "$1" | cut -d' ' -f1
		elif command -v shasum >/dev/null 2>&1; then
			shasum -a 256 "$1" | cut -d' ' -f1
		else
			die "sha256sum or shasum is required to verify the download"
		fi
	}

	# url_host prints the lower-case host of a URL, without user info, port or
	# IPv6 brackets.
	url_host() {
		_h=${1#*://}
		_h=${_h%%/*}
		_h=${_h##*@}
		case $_h in
		\[*)
			_h=${_h#\[}
			_h=${_h%%\]*}
			;;
		*) _h=${_h%%:*} ;;
		esac
		lower "$_h"
	}

	# is_loopback reports whether a URL points at this machine. Only these
	# hosts may use plain http, so the installer tests can run a local server.
	is_loopback() {
		case $(url_host "$1") in
		127.0.0.1 | ::1 | localhost) return 0 ;;
		esac
		return 1
	}

	# check_url refuses a URL that isn't https. Plain http passes only when
	# both the API base and the URL are on loopback.
	check_url() {
		case $(lower "${1%%://*}") in
		https) return 0 ;;
		http) is_loopback "$API" && is_loopback "$1" && return 0 ;;
		esac
		die "refusing to download over http: $1"
	}

	# fetch downloads URL $1 to file $2.
	fetch() {
		check_url "$1"
		curl -fsSL --proto "$proto" --tlsv1.2 -o "$2" "$1"
	}

	# api_get downloads API URL $1 to file $2 and prints the HTTP status. The
	# token goes to curl on stdin, so it never shows up in a process listing.
	api_get() {
		if [ -n "$token" ]; then
			printf 'header = "Authorization: Bearer %s"\n' "$token" |
				curl -sSL --proto "$proto" --tlsv1.2 -K - -H 'Accept: application/vnd.github+json' \
					-o "$2" -w '%{http_code}' "$1"
		else
			curl -sSL --proto "$proto" --tlsv1.2 -H 'Accept: application/vnd.github+json' \
				-o "$2" -w '%{http_code}' "$1"
		fi
	}

	# asset prints the download URL of a release asset by file name.
	asset() {
		tr ',' '\n' <"$tmp/release.json" | grep '"browser_download_url"' |
			sed 's/.*"browser_download_url"[^"]*"\([^"]*\)".*/\1/' | grep "/$1\$" | head -n 1
	}

	# check_tools sets $todo to the names in $tools that the installed rhythm
	# knows, checking every one before any tool is downloaded. Newer routers
	# check the whole list in one `rhythm install --dry-run` and name the
	# unknown ones with a suggestion. Older ones reject the flag, so the names
	# come from the "Tools:" block of `rhythm help`, or from DEFAULT_TOOLS
	# when even that is missing.
	check_tools() {
		todo=
		[ -n "$tools" ] || return 0
		# shellcheck disable=SC2086 # one word per name
		if "$dir/rhythm" install --dry-run $tools >"$tmp/probe.out" 2>&1; then
			todo=$tools
			return 0
		fi
		if grep -q 'flag provided but not defined' "$tmp/probe.out"; then
			# Rows look like "  soundcheck (sc)  did my change break anything?".
			known=$("$dir/rhythm" help 2>/dev/null | awk '
				/^Tools:/ { on = 1; next }
				on && /^[[:space:]]*$/ { exit }
				on { print $1; if ($2 ~ /^\(.*\)$/) print substr($2, 2, length($2) - 2) }' | tr '\n' ' ')
			[ -n "$known" ] || known=$DEFAULT_TOOLS
			for tool in $tools; do
				case " $known " in
				*" $tool "*) todo="$todo $tool" ;;
				*) unknown_tool "$tool" ;;
				esac
			done
			return 0
		fi
		if [ -n "$explicit" ]; then
			cat "$tmp/probe.out" >&2
			bad=$(sed -n 's/.*unknown tool "\([^"]*\)".*/\1/p' "$tmp/probe.out" | tr '\n' ' ')
			[ -n "$bad" ] || die "$installed refused RHYTHM_TOOLS (see above); nothing more was installed"
			case $bad in
			*@*) die "$installed doesn't know $(commas "$bad"); it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version" ;;
			esac
			die "$installed doesn't know $(commas "$bad"); run '$dir/rhythm help' to see the tools"
		fi
		# A default this rhythm doesn't know: ask about each one, to skip it.
		for tool in $tools; do
			if "$dir/rhythm" install --dry-run "$tool" >/dev/null 2>&1; then
				todo="$todo $tool"
			else
				unknown_tool "$tool"
			fi
		done
	}

	# unknown_tool stops at a tool the user named that rhythm doesn't know.
	# An unknown default comes from an old pinned rhythm and is skipped.
	unknown_tool() {
		case $1 in
		*@*) die "$installed doesn't know $1; it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version" ;;
		esac
		[ -z "$explicit" ] || die "$installed doesn't know $1; run '$dir/rhythm help' to see the tools"
		warn "$installed doesn't know $1; skipped"
	}

	# commas joins the words in $1 with ", ", or prints "none".
	commas() {
		# shellcheck disable=SC2086 # split into words; globbing is off by then
		set -- $1
		[ $# -gt 0 ] || set -- none
		_c=$1
		shift
		for _w in "$@"; do _c="$_c, $_w"; done
		printf '%s' "$_c"
	}

	# verify_signature checks that checksums.txt was signed by rhythm's
	# release workflow for tag $tag, with cosign or else gh, whichever is
	# installed. Without either it says so and goes on: once installed,
	# rhythm checks itself and every tool it installs.
	verify_signature() {
		why="no cosign or gh"
		if command -v cosign >/dev/null 2>&1; then
			if [ -s "$tmp/checksums.txt.sigstore.json" ]; then
				if cosign verify-blob --bundle "$tmp/checksums.txt.sigstore.json" \
					--certificate-oidc-issuer "$ISSUER" \
					--certificate-identity "https://github.com/$WORKFLOW@refs/heads/master" \
					--certificate-github-workflow-repository "$REPO" \
					--certificate-github-workflow-ref "refs/tags/$tag" \
					"$tmp/checksums.txt" >"$tmp/verify.out" 2>&1; then
					say "Signature ok  $REPO @ refs/tags/$tag (cosign)"
					return
				fi
				cat "$tmp/verify.out" >&2
				die "cosign says checksums.txt wasn't signed for $REPO $tag; nothing was installed"
			fi
			why="the release has no checksums.txt.sigstore.json"
		fi
		# gh needs a login to ask for attestations and --source-ref to pin the tag.
		if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1 &&
			gh attestation verify --help 2>&1 | grep -q -- --source-ref; then
			if gh attestation verify "$tmp/checksums.txt" --repo "$REPO" \
				--signer-workflow "$WORKFLOW" --source-ref "refs/tags/$tag" >"$tmp/verify.out" 2>&1; then
				say "Signature ok  $REPO @ refs/tags/$tag (gh attestation)"
				return
			fi
			cat "$tmp/verify.out" >&2
			die "gh found no attestation that $REPO $tag made this checksums.txt; nothing was installed. Releases from before attestations have none: install cosign to check those"
		fi
		say "signature not checked here ($why); rhythm will verify itself and every tool it installs"
	}

	# first_step prints a first command to try with tool $1, by its name or
	# alias. The targets are local, since the tools are for sites you own.
	first_step() {
		case $1 in
		tuner) echo "rhythm tuner scan ./your-site" ;;
		soundcheck) echo "rhythm soundcheck snapshot http://localhost:8080" ;;
		echo) echo "rhythm echo up" ;;
		*) echo "rhythm $1 --help" ;;
		esac
	}

	# tool_name turns an alias, a rhythm-<tool> name or a pin (tuner@0.3.1)
	# into the tool's name.
	tool_name() {
		set -- "${1%%@*}"
		case ${1#rhythm-} in
		t) echo tuner ;;
		sc) echo soundcheck ;;
		e) echo echo ;;
		*) echo "${1#rhythm-}" ;;
		esac
	}

	# path_hint says how to put $dir on PATH in the user's shell. zsh, the
	# macOS default, never reads ~/.profile; a macOS terminal starts bash as
	# a login shell, which reads ~/.bash_profile and not ~/.bashrc.
	path_hint() {
		case ${SHELL:-} in
		*/fish | fish)
			say "Add $dir to your PATH: fish_add_path '$dir'"
			return
			;;
		*/zsh | zsh) rc=.zshrc ;;
		*/bash | bash)
			rc=.bashrc
			[ "$os" != darwin ] || rc=.bash_profile
			;;
		*) rc=.profile ;;
		esac
		say "Add $dir to your PATH, then open a new terminal: echo 'export PATH=\"$dir:\$PATH\"' >> ~/$rc"
	}

	command -v curl >/dev/null 2>&1 || die "curl is required"
	command -v tar >/dev/null 2>&1 || die "tar is required"

	API=${RHYTHM_GITHUB_API:-https://api.github.com}
	API=${API%/}
	proto='=https'
	case $(lower "${API%%://*}") in
	https) ;;
	http)
		is_loopback "$API" ||
			die "RHYTHM_GITHUB_API must be https (http only for 127.0.0.1, ::1 and localhost): $API"
		proto='=http,https'
		;;
	*) die "RHYTHM_GITHUB_API must be an https URL: $API" ;;
	esac

	# GITHUB_TOKEN belongs to github.com, so it never goes to a mirror.
	token=
	if [ -n "${RHYTHM_GITHUB_TOKEN:-}" ]; then
		token=$RHYTHM_GITHUB_TOKEN
	elif [ "$proto" = '=https' ] && [ "$(url_host "$API")" = api.github.com ]; then
		token=${GITHUB_TOKEN:-}
	fi
	# curl reads the header from a config line, where these would break out.
	case $token in
	*[\"\\]*) die "the GitHub token has a character curl can't send in a header" ;;
	esac
	[ "$(printf '%s' "$token" | wc -l)" -eq 0 ] ||
		die "the GitHub token has a character curl can't send in a header"

	os=$(lower "$(uname -s)")
	case $os in
	linux | darwin) ;;
	*) die "unsupported OS '$os'. On Windows, run in PowerShell: irm https://rythm.sh/install.ps1 | iex" ;;
	esac
	case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "unsupported CPU '$(uname -m)'" ;;
	esac

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	trap 'exit 130' INT
	trap 'exit 143' TERM

	if [ -n "${RHYTHM_VERSION:-}" ]; then
		release_url="$API/repos/$REPO/releases/tags/v${RHYTHM_VERSION#v}"
	else
		release_url="$API/repos/$REPO/releases/latest"
	fi
	status=$(api_get "$release_url" "$tmp/release.json") ||
		die "couldn't reach $(url_host "$API") ($release_url)"
	case $status in
	200) ;;
	403 | 429) die "GitHub rate limit reached; set GITHUB_TOKEN" ;;
	*) die "couldn't fetch the release from GitHub ($release_url): HTTP $status" ;;
	esac

	# The tag the signature must name. A pinned version is what the user
	# asked for, so a mirror can't answer with another one.
	if [ -n "${RHYTHM_VERSION:-}" ]; then
		tag=v${RHYTHM_VERSION#v}
	else
		tag=$(tr ',' '\n' <"$tmp/release.json" | sed -n 's/.*"tag_name"[^"]*"\([^"]*\)".*/\1/p' | head -n 1)
	fi
	case $tag in
	v[0-9]*[!0-9A-Za-z.+-]* | v) die "the release has an odd tag: '$tag'" ;;
	v[0-9]*) ;;
	*) die "the release has an odd tag: '$tag'" ;;
	esac

	name="rhythm_${os}_${arch}.tar.gz"
	archive_url=$(asset "$name")
	sums_url=$(asset checksums.txt)
	bundle_url=$(asset checksums.txt.sigstore.json)
	[ -n "$archive_url" ] || die "the release has no build for $os/$arch"
	[ -n "$sums_url" ] || die "the release has no checksums.txt; refusing to install unverified binaries"

	fetch "$archive_url" "$tmp/$name" || die "download failed: $archive_url"
	fetch "$sums_url" "$tmp/checksums.txt" || die "download failed: $sums_url"
	if [ -n "$bundle_url" ]; then
		fetch "$bundle_url" "$tmp/checksums.txt.sigstore.json" || die "download failed: $bundle_url"
	fi
	# RHYTHM_SKIP_SIGNATURE is for a mirror that re-signs nothing; any value
	# but empty or 0 turns it on.
	skip_sig=
	case ${RHYTHM_SKIP_SIGNATURE:-0} in
	0) verify_signature ;;
	*)
		skip_sig=yes
		warn "RHYTHM_SKIP_SIGNATURE is set: nothing checks that this release was signed by $REPO. Use it only with a mirror you trust."
		;;
	esac

	want=$(grep " \*\{0,1\}$name\$" "$tmp/checksums.txt" | cut -d' ' -f1)
	got=$(sha256 "$tmp/$name")
	if [ -z "$want" ] || [ "$want" != "$got" ]; then
		die "checksum mismatch for $name; nothing was installed"
	fi
	tar -xzf "$tmp/$name" -C "$tmp" rhythm

	if [ -n "${RHYTHM_INSTALL_DIR:-}" ]; then
		dir=$RHYTHM_INSTALL_DIR
	elif [ "$(id -u)" = 0 ]; then
		dir=/usr/local/bin
	else
		dir=$HOME/.local/bin
	fi
	mkdir -p "$dir" || die "can't create $dir"
	# A relative folder would be wrong in a PATH hint and in what rhythm is told.
	dir=$(CDPATH='' cd -- "$dir" && pwd) || die "can't open $dir"
	# The old rhythm is kept until the new one has checked itself.
	old=
	if [ -f "$dir/rhythm" ] && cp -p "$dir/rhythm" "$tmp/rhythm.old" 2>/dev/null; then
		old=$tmp/rhythm.old
	fi
	if ! { cp "$tmp/rhythm" "$dir/.rhythm.new" && chmod 0755 "$dir/.rhythm.new" && mv -f "$dir/.rhythm.new" "$dir/rhythm"; }; then
		die "can't write to $dir; set RHYTHM_INSTALL_DIR or rerun with sudo"
	fi
	installed=$("$dir/rhythm" --version) || die "the installed $dir/rhythm doesn't run"
	# A rhythm with the verify command checks its own release and bytes.
	if [ -z "$skip_sig" ] && "$dir/rhythm" help 2>/dev/null | grep -q '^ *rhythm verify'; then
		if ! "$dir/rhythm" verify rhythm >"$tmp/verify.out" 2>&1; then
			cat "$tmp/verify.out" >&2
			if [ -n "$old" ] && mv -f "$old" "$dir/rhythm"; then
				die "the new rhythm failed its own check (rhythm verify rhythm), so the previous one was put back; nothing was installed"
			fi
			rm -f "$dir/rhythm"
			die "the new rhythm failed its own check (rhythm verify rhythm), so it was removed; nothing was installed"
		fi
		cat "$tmp/verify.out" >&2
	fi
	say "Installed $installed to $dir"

	# An unset RHYTHM_TOOLS means the defaults, which a newer rhythm names
	# itself (`rhythm install --defaults --dry-run`); for an older one they
	# are DEFAULT_TOOLS. Names are separated by spaces or commas as in
	# rhythm's own lists; "none" stands for no tool and tool@version pins
	# one, passed to rhythm as it is. An old pinned rhythm may not know every
	# default tool; those are skipped. A name the user asked for must be
	# known, or no tool is installed. RHYTHM_LOCKED=1 installs what
	# ./rhythm.lock says instead.
	explicit=${RHYTHM_TOOLS+yes}
	locked=
	case ${RHYTHM_LOCKED:-0} in
	0) ;;
	*) locked=yes ;;
	esac
	tools=
	todo=
	set -f
	if [ -n "$locked" ]; then
		[ -f rhythm.lock ] || die "RHYTHM_LOCKED is set but there is no rhythm.lock in $(pwd)"
		"$dir/rhythm" install --help 2>&1 | grep -q -- --locked ||
			die "$installed can't install from rhythm.lock; set RHYTHM_VERSION to a newer rhythm"
		[ -z "$explicit" ] || warn "RHYTHM_TOOLS is ignored: RHYTHM_LOCKED installs the tools in rhythm.lock"
	elif [ -z "$explicit" ] && "$dir/rhythm" install --defaults --dry-run >"$tmp/defaults.out" 2>/dev/null; then
		todo=$(cat "$tmp/defaults.out")
	else
		for tool in $(printf '%s' "${RHYTHM_TOOLS-$DEFAULT_TOOLS}" | tr ',' ' '); do
			[ "$(lower "$tool")" = none ] || tools="$tools $tool"
		done
		check_tools
	fi
	# An alias and the name are one tool, installed once.
	names=
	list=
	for tool in $todo; do
		t=$(tool_name "$tool")
		case " $names " in
		*" $t "*) continue ;;
		esac
		names="$names $t"
		list="$list $tool"
	done
	todo=$list
	# The PATH hint comes once, at the end. A router that has --quiet leaves
	# out its own PATH notes; an older one prints one per tool, filtered here.
	quiet=
	if [ -n "$todo$locked" ] && "$dir/rhythm" install --help 2>&1 | grep -q -- --quiet; then
		quiet=--quiet
	fi
	# A tool that fails doesn't stop the next; the router has said why.
	done_tools=
	failed=
	if [ -n "$locked" ]; then
		# One call; the tools installed are the ones rhythm says it installed.
		# shellcheck disable=SC2086 # $quiet is empty or one flag
		if ! "$dir/rhythm" install --locked --dir "$dir" $quiet >"$tmp/install.out" 2>&1; then
			failed=$(sed -n 's/^Installed [0-9]*, .*failed [0-9]*: //p' "$tmp/install.out" | tr -d ',' | sed 's/rhythm-//g')
			[ -n "$failed" ] || failed="rhythm.lock"
		fi
		done_tools=$(sed -n 's/^Installed rhythm-\([^ ]*\) .*/\1/p' "$tmp/install.out")
		grep -v "^Note: .* isn't on your PATH\.\$" "$tmp/install.out" >&2 || :
	fi
	for tool in $todo; do
		# shellcheck disable=SC2086 # $quiet is empty or one flag
		if "$dir/rhythm" install "$tool" --dir "$dir" $quiet >"$tmp/install.out"; then
			done_tools="$done_tools $(tool_name "$tool")"
		else
			failed="$failed $(tool_name "$tool")"
		fi
		grep -v "^Note: .* isn't on your PATH\.\$" "$tmp/install.out" >&2 || :
	done

	case ":$PATH:" in
	*":$dir:"*)
		# The router never checks rhythm itself. With --quiet it doesn't
		# say when a copy of a tool comes first on PATH either.
		for b in rhythm ${quiet:+$done_tools}; do
			[ "$b" = rhythm ] || b=rhythm-$b
			found=$(command -v "$b" 2>/dev/null) || continue
			[ "$found" = "$dir/$b" ] ||
				say "Note: $found comes first on your PATH; remove it to use this install."
		done
		;;
	*) path_hint ;;
	esac
	if [ -n "$done_tools" ]; then
		say "Get started:"
		for t in $done_tools; do
			say "  $(first_step "$t")"
		done
	elif [ -z "$failed" ]; then
		say "Install a tool: rhythm install tuner"
	fi
	say "Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use"
	if [ -n "$failed" ]; then
		say "Installed: $(commas "$done_tools"). Failed: $(commas "$failed") (see above)."
		exit 1
	fi
	set +f
}

main "$@"
