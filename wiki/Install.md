Install `rhythm`, Tuner, Soundcheck and Echo with one command on macOS, Linux or Windows. This page also covers the installer's options and messages, mirrors and tokens, Linux packages, GitHub Actions, `go install`, manual downloads and how to add a single tool later.

## Quick install

**macOS · Linux**

```sh
curl -fsSL https://rythm.sh/install | sh
```

**Windows** (PowerShell)

```powershell
irm https://rythm.sh/install.ps1 | iex
```

Both installers put `rhythm`, `rhythm-tuner`, `rhythm-soundcheck` and `rhythm-echo` in the same folder. Every download is checked against the release's `checksums.txt` before anything is written. The signature over `checksums.txt` is checked too when it can be (see [Signature checks](#signature-checks)). The installer ends with your first steps:

```console
$ curl -fsSL https://rythm.sh/install | sh
signature not checked (no cosign or gh); install cosign or gh to check it before anything runs
Installed rhythm 0.6.0 to /home/me/.local/bin
Installed rhythm-echo 1.0.0 to /home/me/.local/bin/rhythm-echo
Installed rhythm-soundcheck 1.1.0 to /home/me/.local/bin/rhythm-soundcheck
Installed rhythm-tuner 0.3.2 to /home/me/.local/bin/rhythm-tuner
Add /home/me/.local/bin to your PATH, then open a new terminal: echo 'export PATH="/home/me/.local/bin:$PATH"' >> ~/.zshrc
Get started:
  rhythm echo up
  rhythm soundcheck snapshot http://localhost:8080
  rhythm tuner scan ./your-site
Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use
```

That run installed rhythm 0.6.0, the latest release while this was written, on a machine without cosign or gh. From rhythm 0.7.0 the new `rhythm` also checks itself with `rhythm verify rhythm` and each tool's lines are followed by its `Signature ok` and `Checksum ok` lines.

Soundcheck drives a real browser. It uses Chrome or Chromium if you have one; otherwise it downloads Chromium once, the first time it needs it (see [Chromium on first use](#chromium-on-first-use)).

> [!IMPORTANT]
> Scan and check only sites you own or have permission to test. See [Responsible Use](Responsible-Use).

| Elsewhere | See |
|---|---|
| GitHub Actions | [CI setup](CI-Setup): `uses: RhythmShahriar/rhythm/setup@v0.7.0` |
| a pipeline or laptop that must match a project's versions | [Pinning and locking](Pinning-And-Locking): `RHYTHM_LOCKED=1` |
| a machine that can't reach GitHub | [Offline installs](Offline-Installs) |
| Debian, Ubuntu, Fedora, Alpine packages | [Linux packages](#linux-packages) |

## What the installer does

1. Checks its options: the API URL, the version and the timeout are refused before any request when they aren't valid (see [Messages](#messages)).
2. Detects your OS (`linux` or `darwin`; Windows uses the PowerShell script) and CPU (`amd64` or `arm64`). Other platforms stop with an error.
3. Asks the release API for the latest `rhythm` release, or the version in `RHYTHM_VERSION`.
4. Downloads `rhythm_<os>_<arch>.tar.gz` (`.zip` on Windows), `checksums.txt` and its signature, `checksums.txt.sigstore.json`, over `https` only.
5. Checks the signature with cosign or the GitHub CLI when one of them is installed.
6. Compares the archive's SHA-256 with its line in `checksums.txt`. If the release has no `checksums.txt`, or the hash doesn't match, it stops and installs nothing.
7. Copies `rhythm` into the install folder through a temporary file, so an existing `rhythm` is never left half-written. Then it runs `rhythm verify rhythm`, which checks the new binary against its signed release; if that fails, the previous `rhythm` is put back (or the new one removed) and nothing else is installed.
8. Asks `rhythm` which tools a fresh setup gets (`rhythm install --defaults --dry-run`, today Echo, Soundcheck and Tuner) unless you set `RHYTHM_TOOLS`. Every name is checked in one go before any tool is downloaded.
9. Runs `rhythm install <tool> --dir <install folder>` for each tool. That download is checked the same way, signature included. A tool that fails doesn't stop the others.
10. Prints the first steps for each tool it installed and checks your `PATH`.

### Where it installs

| Platform | Default folder |
|---|---|
| macOS, Linux | `~/.local/bin` |
| macOS, Linux, run as root | `/usr/local/bin` |
| Windows | `%LOCALAPPDATA%\Programs\rhythm` |

A relative `RHYTHM_INSTALL_DIR` is made absolute, so the messages and the `PATH` hint always name the real folder.

### PATH

- **macOS and Linux:** the installer doesn't edit your shell profile. When the folder isn't on your `PATH`, the last line says how to add it, for the shell in `$SHELL`:

  | Shell | Line |
  |---|---|
  | zsh | `Add /home/me/.local/bin to your PATH, then open a new terminal: echo 'export PATH="/home/me/.local/bin:$PATH"' >> ~/.zshrc` |
  | bash | the same with `~/.bashrc` (`~/.bash_profile` on macOS) |
  | fish | `Add /home/me/.local/bin to your PATH: fish_add_path '/home/me/.local/bin'` |
  | anything else | the same as zsh with `~/.profile` |

  When the folder is on your `PATH` but another copy comes first, it says `Note: <path> comes first on your PATH; remove it to use this install.`
- **Windows:** the installer adds the folder to your **user** `PATH` if it isn't there yet (`Added <dir> to your user PATH. Open a new terminal to use rhythm.`), or says `<dir> is on your user PATH. Open a new terminal to use rhythm.` when only the current session lacks it.

### Requirements

- macOS and Linux: `curl`, `tar` and either `sha256sum` or `shasum`. Any POSIX `sh` runs it: `sh`, dash, bash and BusyBox `sh` are tested.
- Windows: Windows PowerShell 5.1 or PowerShell 7.

## Signature checks

The release workflow signs `checksums.txt` with keyless [Sigstore](https://docs.sigstore.dev/). The installer checks that signature before it trusts any checksum, with whichever of these it finds:

| You have | The installer runs | It prints |
|---|---|---|
| `cosign` | `cosign verify-blob` with the identity, the repository and the tag pinned ([the full command](Verifying-Releases#with-cosign)) | `Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (cosign)` |
| `gh`, logged in | `gh attestation verify` with the repository, the signing workflow and the tag pinned | `Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (gh attestation)` |
| neither | nothing | `signature not checked (no cosign or gh); install cosign or gh to check it before anything runs` |

A failed check stops the install: `rhythm install: cosign says checksums.txt wasn't signed for RhythmShahriar/rhythm v0.7.0; nothing was installed`, or the `gh found no attestation …` line for gh. When gh is there but can't be used, the line says why instead (`gh is not logged in`, `gh is too old to pin the tag`). A release without a signature bundle gets a warning, not a failure, so releases from before signing stay installable with `RHYTHM_VERSION`:

```text
warning: signature not checked (the release has no checksums.txt.sigstore.json); install cosign or gh to check it before anything runs
```

Either way, once `rhythm` is in place it checks itself with `rhythm verify rhythm` and every tool it installs, signature included; that needs neither cosign nor gh. With `RHYTHM_SKIP_SIGNATURE=1` the installer skips both of its checks and says `warning: RHYTHM_SKIP_SIGNATURE is set: nothing checks that this release was signed by RhythmShahriar/rhythm. Use it only with a mirror you trust.` The tools are still installed with their signatures checked; see [Verifying releases](Verifying-Releases).

## Installer options

Set these environment variables to change what the installer does. Both scripts read the same names.

| Variable | Default | Effect |
|---|---|---|
| `RHYTHM_INSTALL_DIR` | see [Where it installs](#where-it-installs) | Install somewhere else. The folder is created if needed. |
| `RHYTHM_VERSION` | latest | Install a specific `rhythm` version, e.g. `0.5.0` (one leading `v` is fine). Anything that isn't a version is refused before a request. |
| `RHYTHM_TOOLS` | the tools rhythm installs on a fresh setup (`tuner soundcheck echo` for a rhythm older than 0.7.0) | Tools to install alongside, separated by spaces or commas. `none` installs no tool. `tool@version` pins one: `tuner@0.3.1`, or `tuner@0.3` for the newest 0.3.x. |
| `RHYTHM_LOCKED` | off | `1` installs the tools and versions in `rhythm.lock` in the current folder (`rhythm install --locked`) instead of `RHYTHM_TOOLS`. |
| `RHYTHM_SKIP_SIGNATURE` | off | `1` skips the installer's signature checks of `rhythm`, with a warning. For a mirror you trust that can't serve the signatures. |
| `GITHUB_TOKEN` | none | Sent to `api.github.com` only, to lift GitHub's limit of 60 anonymous requests an hour (shared CI runners hit it). |
| `RHYTHM_GITHUB_API` | `https://api.github.com` | A GitHub API mirror or GitHub Enterprise; see [Mirrors and tokens](#mirrors-and-tokens). |
| `RHYTHM_GITHUB_TOKEN` | none | A token for that mirror; sent to the `RHYTHM_GITHUB_API` host and used instead of `GITHUB_TOKEN`. |
| `RHYTHM_INSTALL_TIMEOUT` | `30` | Seconds to wait for a server that doesn't answer, at most `3600`. `install.sh` also gives up on a download that has stalled for that long. |
| `RHYTHM_ARCH` | none | Windows only: the CPU to use when `PROCESSOR_ARCHITECTURE` is neither `AMD64` nor `ARM64`. |

```sh
# rhythm and Soundcheck only
curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=soundcheck sh

# rhythm alone, into /opt/bin
curl -fsSL https://rythm.sh/install | RHYTHM_INSTALL_DIR=/opt/bin RHYTHM_TOOLS=none sh

# Tuner pinned to 0.3.1 plus Soundcheck
curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS='tuner@0.3.1,soundcheck' sh

# rhythm pinned to a version
curl -fsSL https://rythm.sh/install | RHYTHM_VERSION=0.5.0 sh

# the versions a project's rhythm.lock pins, from the folder that holds it
curl -fsSL https://rythm.sh/install | RHYTHM_LOCKED=1 sh
```

```powershell
$env:RHYTHM_INSTALL_DIR = 'C:\Tools\rhythm'
irm https://rythm.sh/install.ps1 | iex

$env:RHYTHM_TOOLS = 'none'; irm https://rythm.sh/install.ps1 | iex
```

> [!NOTE]
> On Windows, `RHYTHM_TOOLS = 'none'` is the way to install no tools: Windows PowerShell 5.1 and PowerShell before 7.5 delete a variable you set to an empty value.

`RHYTHM_VERSION` pins `rhythm` only; tools come at their latest release unless you pin them in `RHYTHM_TOOLS` or a lock. An older pinned `rhythm` installs the default tools it knows and skips the rest with `warning: rhythm 0.4.0 doesn't know echo; skipped`. With tools you named, an unknown one stops before any tool is installed: `rhythm install: rhythm 0.7.0 doesn't know sondcheck; run '<dir>/rhythm help' to see the tools`, after rhythm's own `did you mean` line. A rhythm too old for a pin says `rhythm install: rhythm 0.6.0 doesn't know tuner@0.3.1; it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version`.

When a tool fails, the others are still installed and the very last line says which, with exit 1:

```text
Installed: tuner, soundcheck. Failed: echo (see above).
```

### Mirrors and tokens

`RHYTHM_GITHUB_API` points the installer (and `rhythm` itself) at a GitHub API mirror or GitHub Enterprise. The same rules hold everywhere:

- The URL must be `https`. Plain `http` is accepted only for `127.0.0.1`, `::1` and `localhost`, for tests. It must be a plain base URL, with no user info, `?` or `#`; a trailing `/` is ignored.
- Every download URL the release lists must be `https` too. A redirect is followed only to `https`.
- `GITHUB_TOKEN` goes only to `api.github.com`, never to a mirror. For a mirror, set `RHYTHM_GITHUB_TOKEN`; it goes to the `RHYTHM_GITHUB_API` host (whatever it is) and wins over `GITHUB_TOKEN`.
- The token is never printed. `install.sh` passes it to curl on stdin, so it doesn't show in `ps`.

```sh
curl -fsSL https://rythm.sh/install | \
  RHYTHM_GITHUB_API=https://ghe.example.com/api/v3 RHYTHM_GITHUB_TOKEN="$GHE_TOKEN" sh
```

### Messages

All of these exit 1 and install nothing.

| Message | Means |
|---|---|
| `rhythm install: RHYTHM_GITHUB_API must be https (http only for 127.0.0.1, ::1 and localhost): <url>` | plain `http` to another host |
| `rhythm install: RHYTHM_GITHUB_API must be an https URL: <url>` | another scheme, or none |
| `rhythm install: RHYTHM_GITHUB_API must be a plain base URL, without user info, query or fragment: <url>` | an `@`, `?`, `#` or backslash in it |
| `rhythm install: RHYTHM_VERSION isn't a version: '../x'` | checked before it goes into a URL |
| `rhythm install: RHYTHM_INSTALL_TIMEOUT must be a whole number of seconds: 'abc'` | also for `0` |
| `rhythm install: RHYTHM_INSTALL_TIMEOUT must be at most 3600 seconds: '4000'` | |
| `rhythm install: GitHub rate limit reached; set GITHUB_TOKEN` | GitHub refused with 403 or 429; on a mirror `rate limit reached at <host>; set RHYTHM_GITHUB_TOKEN` (the advice is left out when a token was sent) |
| `rhythm install: couldn't reach <host> (<release url>)` | a network problem |
| `rhythm install: timed out after 30s waiting for <host> (<release url>); set RHYTHM_INSTALL_TIMEOUT to wait longer` | also `timed out after 30s downloading <url>` |
| `rhythm install: couldn't fetch the release from GitHub (<release url>): HTTP 404` | another HTTP status |
| `rhythm install: refusing to download over http: <url>` | the release lists a plain `http` download |
| `rhythm install: <host> redirected to a URL that isn't https (<release url>)` | also `refusing to follow a redirect that isn't https from <url>` |
| `rhythm install: the release has no checksums.txt; refusing to install unverified binaries` | |
| `rhythm install: checksum mismatch for rhythm_linux_amd64.tar.gz; nothing was installed` | |
| `rhythm install: can't write to <dir>; set RHYTHM_INSTALL_DIR or rerun with sudo` | on Windows `can't write to <dir>; set RHYTHM_INSTALL_DIR to a folder you own` |
| `rhythm install: RHYTHM_LOCKED is set but there is no rhythm.lock in <folder>` | |

Ctrl-C exits 130 and `SIGTERM` 143; the temporary folder is removed either way. The whole script runs from one function (one script block in PowerShell) called on its last line, so a download cut off halfway is a syntax error that runs nothing.

### Windows

`install.ps1` runs in Windows PowerShell 5.1 and PowerShell 7. Both are tested on every change. It replaces a running `rhythm.exe` safely: the new file is copied next to it and moved into place; when Windows refuses because rhythm is running, the old one is renamed to `rhythm.exe.old` first and put back if the move still fails. Its messages match `install.sh`, with `can't replace <dir>\rhythm.exe; close any running rhythm and try again (<reason>)` for a file Windows keeps locked.

## Linux packages

From v0.7.0, every `rhythm` release also has packages of `rhythm` for `amd64` and `arm64`, listed in the signed `checksums.txt`:

| Package | For | Example |
|---|---|---|
| `.deb` | Debian, Ubuntu | `rhythm_0.7.0_amd64.deb` |
| `.rpm` | Fedora, RHEL, openSUSE | `rhythm-0.7.0-1.x86_64.rpm` |
| `.apk` | Alpine | `rhythm_0.7.0_x86_64.apk` |

```sh
sudo dpkg -i rhythm_0.7.0_amd64.deb          # or: sudo rpm -i …, sudo apk add --allow-untrusted …
rhythm install --defaults --dir ~/.local/bin
```

A package holds `rhythm` alone, in `/usr/bin`, with its license and README in `/usr/share/doc/rhythm`. Install the tools into a folder you own with `--dir`, since `/usr/bin` belongs to the package manager. Update `rhythm` itself by installing the next release's package. Verify a package like any asset (see [Verifying releases](Verifying-Releases)). A Homebrew cask, a Scoop manifest and a winget package are prepared by every release and will be announced once they are published.

## Install a single tool

With `rhythm` installed, add a tool by name:

```console
$ rhythm install echo
Installed rhythm-echo 1.0.0 to /home/me/.local/bin/rhythm-echo
  Signature ok  RhythmShahriar/echo @ refs/tags/v1.0.0 (release-go.yml)
  Checksum ok   sha256:419130487999167ce27cf0d67c519bd36f35b8b270d307b32a3cee314cc3a5f2
```

Short names work too: `rhythm install t` installs Tuner, `sc` Soundcheck and `e` Echo. Name several (`rhythm install tuner,echo soundcheck`), pin one (`rhythm install tuner@0.3.1`) or check names first with `--dry-run`. The download is checked against the release's signature and `checksums.txt` and swapped into place atomically, exactly like an update. Every flag and message is in the [Command reference](Command-Reference#install).

After installing, `rhythm` warns you if the tool won't be found:

- `Note: <dir> isn't on your PATH.` Add the folder to your `PATH`.
- `Note: <path> comes first on your PATH; remove it to use this install.` An older copy elsewhere shadows the new one.

## Install with Go

If you have Go installed:

```sh
go install github.com/RhythmShahriar/rhythm/cmd/rhythm@latest
go install github.com/RhythmShahriar/tuner/cmd/rhythm-tuner@latest
go install github.com/RhythmShahriar/soundcheck/cmd/rhythm-soundcheck@latest
go install github.com/RhythmShahriar/echo/cmd/rhythm-echo@latest
```

Binaries go to `$(go env GOPATH)/bin`.

> [!NOTE]
> `go install` builds report their version as `dev`. `rhythm update` skips development builds, `rhythm verify` and `rhythm lock` can't check them and they never show the "new release" notice. Update them by running `go install …@latest` again, or replace one with a release: `rhythm install tuner --force`.

## Manual download

Every release has archives for Linux, macOS and Windows on `amd64` and `arm64`, each with an SBOM:

- `rhythm`: https://github.com/RhythmShahriar/rhythm/releases/latest
- Tuner: https://github.com/RhythmShahriar/tuner/releases/latest
- Soundcheck: https://github.com/RhythmShahriar/soundcheck/releases/latest
- Echo: https://github.com/RhythmShahriar/echo/releases/latest

Download the archive for your platform (for example `rhythm-tuner_linux_arm64.tar.gz` or `rhythm-tuner_windows_amd64.zip`) together with `checksums.txt` and `checksums.txt.sigstore.json`, check them as described in [Verifying Releases](Verifying-Releases), then extract the binary into a folder on your `PATH`. Keep `rhythm` and its tools in the same folder so `rhythm update` and `rhythm install` find them.

## Chromium on first use

Soundcheck needs Chrome or Chromium; Tuner and Echo don't. Soundcheck uses an installed Chrome or Chromium when it finds one. Otherwise, the first time it needs a browser, it downloads the Chromium build it's pinned to into your cache folder (`~/.cache/rhythm/soundcheck/browser` on Linux) and says so with `Downloading Chromium once, into <folder> …`.

To do it ahead of time, for example while building a Docker image, run `rhythm soundcheck update`. To use a specific browser, pass `--browser /path/to/chrome`; to forbid the download, `--no-download`. See [Soundcheck FAQ](Soundcheck-FAQ#chromium-download-and-update).

## Echo's connectors

Echo's binary is all you need on the machine that runs it. The sites it watches get a connector, installed separately: the WordPress mu-plugin comes with the binary (`rhythm echo install wordpress`), the Laravel package from Packagist (`rhythmsh/echo-laravel`) and the Next.js and Node package from npm (`@rhythmsh/echo`). See [Connectors](Echo-Install-Connectors).

## Check it worked

```console
$ rhythm list
TOOL        VERSION  LATEST  SIGNED  PATH
rhythm      dev      0.6.0   no      /home/me/.local/bin/rhythm
echo        1.0.0    1.0.0   yes     /home/me/.local/bin/rhythm-echo
soundcheck  1.1.0    1.1.0   yes     /home/me/.local/bin/rhythm-soundcheck
tuner       0.3.2    0.3.2   yes     /home/me/.local/bin/rhythm-tuner
```

(This `rhythm` was built from source, so it says `dev` and has no signed release to match; a released rhythm shows its version and `yes`.)

`rhythm list` shows every tool with its version, the latest release and `yes` under SIGNED for each binary that passed its signature check. `rhythm doctor` checks the rest: which copies run first on your `PATH`, whether the folder is writable, Chrome for Soundcheck and whether the release API answers. If you see `rhythm: tuner isn't installed`, run `rhythm install tuner`. If your shell can't find `rhythm` at all, the install folder isn't on your `PATH` yet; see [PATH](#path).

---
<sub>[Home](Home) · [Updating](Updating) →</sub>
