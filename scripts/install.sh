#!/bin/sh
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Installs rhythm and its tools from GitHub Releases:
#   curl -fsSL https://rythm.sh/install | sh
#
# Environment:
#   RHYTHM_INSTALL_DIR   where to install (default ~/.local/bin, or /usr/local/bin as root)
#   RHYTHM_VERSION       a specific version, e.g. 0.5.0 (default: latest)
#   RHYTHM_TOOLS         tools to install as well (default "tuner soundcheck echo"; set empty for none)
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

	say() { printf '%s\n' "$*" >&2; }
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

	name="rhythm_${os}_${arch}.tar.gz"
	archive_url=$(asset "$name")
	sums_url=$(asset checksums.txt)
	[ -n "$archive_url" ] || die "the release has no build for $os/$arch"
	[ -n "$sums_url" ] || die "the release has no checksums.txt; refusing to install unverified binaries"

	fetch "$archive_url" "$tmp/$name" || die "download failed: $archive_url"
	fetch "$sums_url" "$tmp/checksums.txt" || die "download failed: $sums_url"

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
	if ! { cp "$tmp/rhythm" "$dir/.rhythm.new" && chmod 0755 "$dir/.rhythm.new" && mv -f "$dir/.rhythm.new" "$dir/rhythm"; }; then
		die "can't write to $dir; set RHYTHM_INSTALL_DIR or rerun with sudo"
	fi
	installed=$("$dir/rhythm" --version) || die "the installed $dir/rhythm doesn't run"
	say "Installed $installed to $dir"

	for tool in ${RHYTHM_TOOLS-$DEFAULT_TOOLS}; do
		"$dir/rhythm" install "$tool" --dir "$dir" >&2 || die "installing $tool failed"
	done

	case ":$PATH:" in
	*":$dir:"*) ;;
	*) say "Add $dir to your PATH, e.g.: echo 'export PATH=\"$dir:\$PATH\"' >> ~/.profile" ;;
	esac
	say "Get started: rhythm tuner scan example.com · rhythm soundcheck snapshot example.com · rhythm echo up"
}

main "$@"
