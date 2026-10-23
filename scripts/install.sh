#!/bin/sh
# Installs rhythm and its tools from GitHub Releases:
#   curl -fsSL rythm.sh/install | sh
#
# Environment:
#   RHYTHM_INSTALL_DIR  where to install (default ~/.local/bin, or /usr/local/bin as root)
#   RHYTHM_VERSION      a specific version, e.g. 0.2.0 (default: latest)
#   RHYTHM_TOOLS        tools to install as well (default "tuner"; set empty for none)
#   RHYTHM_GITHUB_API   GitHub API base, for mirrors (default https://api.github.com)
set -eu

REPO=RhythmShahriar/rhythm
API=${RHYTHM_GITHUB_API:-https://api.github.com}

say() { printf '%s\n' "$*" >&2; }
die() {
	say "rhythm install: $*"
	exit 1
}
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		die "sha256sum or shasum is required to verify the download"
	fi
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in
linux | darwin) ;;
*) die "unsupported OS '$os'. On Windows, run in PowerShell: irm https://raw.githubusercontent.com/$REPO/master/scripts/install.ps1 | iex" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported CPU '$(uname -m)'" ;;
esac

if [ -n "${RHYTHM_VERSION:-}" ]; then
	release_url="$API/repos/$REPO/releases/tags/v${RHYTHM_VERSION#v}"
else
	release_url="$API/repos/$REPO/releases/latest"
fi
json=$(curl -fsSL -H 'Accept: application/vnd.github+json' "$release_url") ||
	die "couldn't fetch the release from GitHub ($release_url)"

# asset prints the download URL of a release asset by file name.
asset() {
	printf '%s' "$json" | tr ',' '\n' | grep '"browser_download_url"' |
		sed 's/.*"browser_download_url"[^"]*"\([^"]*\)".*/\1/' | grep "/$1\$" | head -n 1
}

name="rhythm_${os}_${arch}.tar.gz"
archive_url=$(asset "$name")
sums_url=$(asset checksums.txt)
[ -n "$archive_url" ] || die "the release has no build for $os/$arch"
[ -n "$sums_url" ] || die "the release has no checksums.txt; refusing to install unverified binaries"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
curl -fsSL -o "$tmp/$name" "$archive_url" || die "download failed: $archive_url"
curl -fsSL -o "$tmp/checksums.txt" "$sums_url" || die "download failed: $sums_url"

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
say "Installed $("$dir/rhythm" --version) to $dir"

for tool in ${RHYTHM_TOOLS-tuner}; do
	"$dir/rhythm" install "$tool" --dir "$dir" >&2 || die "installing $tool failed"
done

case ":$PATH:" in
*":$dir:"*) ;;
*) say "Add $dir to your PATH, e.g.: echo 'export PATH=\"$dir:\$PATH\"' >> ~/.profile" ;;
esac
say "Get started: rhythm tuner scan example.com"
