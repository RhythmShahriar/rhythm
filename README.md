<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/banner-dark.svg">
    <img alt="rhythm" src="docs/assets/banner-light.svg" width="100%">
  </picture>
</p>

<p align="center">
  <b>One CLI for every Rhythm tool.</b><br>
  Install once, then <code>rhythm &lt;tool&gt;</code>. Every tool ships and updates on its own.
</p>

<p align="center">
  <a href="https://github.com/RhythmShahriar/rhythm/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/RhythmShahriar/rhythm?style=flat-square&color=ff7c52&label=release"></a>
  <a href="https://github.com/RhythmShahriar/rhythm/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/RhythmShahriar/rhythm/ci.yml?branch=master&style=flat-square&label=ci"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-100f0d?style=flat-square"></a>
</p>

<br>

## Install

**macOS · Linux**

```sh
curl -fsSL https://rythm.sh/install | sh
```

**Windows** (PowerShell)

```powershell
irm https://rythm.sh/install.ps1 | iex
```

This installs `rhythm` and Tuner to `~/.local/bin` (`/usr/local/bin` as root; `%LOCALAPPDATA%\Programs\rhythm` on Windows, added to your PATH). Every download is checked against the release's `checksums.txt`.

<details>
<summary>Installer options</summary>

| Variable | Effect |
|---|---|
| `RHYTHM_INSTALL_DIR` | install somewhere else |
| `RHYTHM_VERSION` | install a specific version, e.g. `0.1.0` |
| `RHYTHM_TOOLS` | tools to install alongside (default `tuner`; empty for none) |

```sh
curl -fsSL https://rythm.sh/install | RHYTHM_INSTALL_DIR=/opt/bin RHYTHM_TOOLS="" sh
```

</details>

## Tools

| Tool | What it does | |
|---|---|---|
| **[tuner](https://github.com/RhythmShahriar/tuner)** | Finds what's out of tune in WordPress &amp; PHP sites: leaked secrets, exposed files, known CVEs, weak configuration | `rhythm tuner scan yoursite.com` |

More are on the way. Each lives in its own repository and releases on its own schedule; `rhythm <tool>` runs the `rhythm-<tool>` binary, the same extension model as `git` and `gh`.

## Commands

```sh
rhythm tuner scan yoursite.com   # run a tool (short: rhythm t scan …)
rhythm install tuner             # add a tool
rhythm update                    # update rhythm, every tool, and their data
rhythm update --check            # only report what's out of date
rhythm version
```

`rhythm update` checks each download against the release checksums and swaps binaries atomically, so a failed update never leaves a broken install. Tools mention when a new release is out, at most once a day; set `RHYTHM_NO_UPDATE_CHECK=1` to silence that.

## Verify a release

Release checksums are signed with keyless [cosign](https://docs.sigstore.dev/) from GitHub Actions:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/RhythmShahriar/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

## Uninstall

Delete the binaries, then the optional cache and settings:

```sh
rm ~/.local/bin/rhythm ~/.local/bin/rhythm-*      # or wherever you installed them
rm -r ~/.cache/rhythm ~/.config/rhythm            # vulnerability cache, update-check state
```

On macOS the last two live at `~/Library/Caches/rhythm` and `~/Library/Application Support/rhythm`. On Windows, delete `%LOCALAPPDATA%\Programs\rhythm` and remove it from your user PATH.

## Development

```sh
make hooks   # Conventional Commits check
make check   # lint + tests
```

`kit/` is the shared base every tool imports: theme, config and self-update.

## License

[AGPL-3.0](LICENSE). Copyright 2026 Rhythm Shahriar.

<p align="center"><sub>Useful ideas. Working systems.</sub></p>
