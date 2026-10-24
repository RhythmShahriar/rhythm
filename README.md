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

## Documentation

Everything about rhythm and its tools lives in the **[documentation wiki](https://github.com/RhythmShahriar/rhythm/wiki)**: installing and updating, every Tuner, Soundcheck and Echo feature, CI, writing checks, and the project's policies.

## Install

**macOS · Linux**

```sh
curl -fsSL https://rythm.sh/install | sh
```

**Windows** (PowerShell)

```powershell
irm https://rythm.sh/install.ps1 | iex
```

This installs `rhythm`, Tuner, Soundcheck and Echo to `~/.local/bin` (`/usr/local/bin` as root; `%LOCALAPPDATA%\Programs\rhythm` on Windows, added to your PATH). Every download is checked against the release's `checksums.txt`.

<details>
<summary>Installer options</summary>

| Variable | Effect |
|---|---|
| `RHYTHM_INSTALL_DIR` | install somewhere else |
| `RHYTHM_VERSION` | install a specific version, e.g. `0.5.0` |
| `RHYTHM_TOOLS` | tools to install alongside, separated by spaces or commas (default `tuner soundcheck echo`; `none` for none). Every name is checked before any tool is downloaded; a tool that fails doesn't stop the others (the installer then exits 1) |
| `GITHUB_TOKEN` | sent to `api.github.com` only, to lift GitHub's limit of 60 anonymous requests an hour (shared CI runners hit it) |
| `RHYTHM_GITHUB_API` | a GitHub API mirror; it must be `https` (plain `http` only for `127.0.0.1`, `::1` and `localhost`) |
| `RHYTHM_GITHUB_TOKEN` | a token for that mirror; sent to the `RHYTHM_GITHUB_API` host and used instead of `GITHUB_TOKEN` |

```sh
curl -fsSL https://rythm.sh/install | RHYTHM_INSTALL_DIR=/opt/bin RHYTHM_TOOLS=none sh
curl -fsSL https://rythm.sh/install | RHYTHM_VERSION=0.5.0 sh
```

```powershell
$env:RHYTHM_TOOLS = 'none'; irm https://rythm.sh/install.ps1 | iex
```

Windows PowerShell 5.1 can't set a variable to an empty value, so use `none` there. A pinned older `rhythm` installs the tools it knows and skips the rest with a warning.

</details>

## Tools

Three questions, one for each tool: is my site safe, did my change break anything, and is it still saying what it should?

| Tool | What it does | |
|---|---|---|
| **[tuner](https://github.com/RhythmShahriar/tuner)** | Finds what's out of tune in WordPress &amp; PHP sites: leaked secrets, exposed files, known CVEs, weak configuration | `rhythm tuner scan yoursite.com` |
| **[soundcheck](https://github.com/RhythmShahriar/soundcheck)** | Did your change break anything? Snapshots a whole site in a real browser, then shows what broke after an update or deploy: PHP and JS errors, dead buttons, forms, cache leaks, layout and speed | `rhythm soundcheck snapshot yoursite.com` |
| **[echo](https://github.com/RhythmShahriar/echo)** | Is your site still saying what it should? Captures every email, SMS, webhook and API call it sends, seals staging so nothing reaches real people, and shows what changed after an update. WordPress, Laravel, Next.js | `rhythm echo up` |

More are on the way. Each lives in its own repository and releases on its own schedule; `rhythm <tool>` runs the `rhythm-<tool>` binary, the same extension model as `git` and `gh`.

## Commands

```sh
rhythm tuner scan yoursite.com   # run a tool (short: rhythm t scan …)
rhythm sc compare                # Soundcheck, by its short name
rhythm e watch                   # Echo: every message as it's sent
rhythm install tuner             # add a tool
rhythm update                    # update rhythm, every tool, and their data
rhythm update --check            # only report what's out of date
rhythm version
```

`rhythm update` checks each download against the release checksums and swaps binaries atomically, so a failed update never leaves a broken install. Tools mention when a new release is out, at most once a day; set `RHYTHM_NO_UPDATE_CHECK=1` to silence that.

## Verify a release

`rhythm update`, `rhythm install` and both installers check every download against the release's `checksums.txt`. That catches a corrupted or cut-off download. It can't tell you who published the release: `checksums.txt` sits on the same release page as the archives, so anyone who can upload there can upload a matching pair.

The signature closes that gap. The release workflow signs `checksums.txt` with keyless [cosign](https://docs.sigstore.dev/). The signing certificate records which repository and which tag the run was for. Download `checksums.txt` and `checksums.txt.sigstore.json` from the release next to your archive, then run (cosign 2.4 or later):

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
  --certificate-github-workflow-repository RhythmShahriar/rhythm \
  --certificate-github-workflow-ref refs/tags/v<version> \
  checksums.txt
sha256sum --ignore-missing -c checksums.txt   # macOS: shasum -a 256 --ignore-missing -c checksums.txt
```

Replace `<version>` with the release you downloaded, e.g. `refs/tags/v0.5.0`. For a tool, use its repository: `RhythmShahriar/tuner`, `RhythmShahriar/soundcheck` or `RhythmShahriar/echo`. All four flags matter. `--certificate-identity` names the shared release workflow, which any public repository can call, so on its own it would accept a signature made for someone else's project. The repository and ref flags pin the run to this project and this tag.

Releases published with GitHub artifact attestations can also be checked with the GitHub CLI, for `checksums.txt` or any archive:

```sh
gh attestation verify checksums.txt --repo RhythmShahriar/rhythm \
  --signer-workflow RhythmShahriar/.github/.github/workflows/release-go.yml
```

Older releases have no attestations; use cosign for those.

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
sh scripts/test/run.sh          # install.sh against a fake GitHub API on 127.0.0.1
pwsh scripts/test/run.ps1       # install.ps1 the same way (needs go and python3)
```

`kit/` is the shared base every tool imports: theme, config and self-update.

## Responsible use

Rhythm tools are for securing **your own** systems, or systems you have explicit permission to test. You are responsible for using them lawfully. They are provided "as is", without warranty, as set out in the [license](LICENSE), and the authors are not liable for misuse or for any damage arising from their use.

## License

[AGPL-3.0](LICENSE). Copyright 2026 Rhythm Shahriar.

<p align="center"><sub>Useful ideas. Working systems.</sub></p>
