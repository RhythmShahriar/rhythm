Every `rhythm` and tool release ships a `checksums.txt`. That file is signed with keyless [cosign](https://docs.sigstore.dev/) from GitHub Actions. This page shows how to check both and what is and isn't signed.

The installers and `rhythm update` already check every download against `checksums.txt`. Verifying the signature yourself adds one more guarantee: that the checksums were produced by Rhythm's release workflow on GitHub, not by someone who swapped files on the release page.

## What a release contains

Each release of `rhythm` or a tool has the same assets:

| Asset | What it is |
|---|---|
| `<binary>_<os>_<arch>.tar.gz` | The binary for Linux or macOS, with README and LICENSE, e.g. `rhythm-tuner_darwin_arm64.tar.gz` |
| `<binary>_windows_<arch>.zip` | The same for Windows |
| `checksums.txt` | SHA-256 of every archive |
| `checksums.txt.sigstore.json` | The cosign signature bundle for `checksums.txt` |

Platforms: `linux`, `darwin`, `windows` on `amd64` and `arm64`.

## 1. Check the archive against checksums.txt

Download the archive, `checksums.txt` and `checksums.txt.sigstore.json` from the same release, into one folder:

- https://github.com/RhythmShahriar/rhythm/releases/latest
- https://github.com/RhythmShahriar/tuner/releases/latest
- https://github.com/RhythmShahriar/soundcheck/releases/latest
- https://github.com/RhythmShahriar/echo/releases/latest

**Linux**

```sh
grep ' rhythm-tuner_linux_amd64.tar.gz$' checksums.txt | sha256sum -c
```

**macOS**

```sh
grep ' rhythm-tuner_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c
```

```text
rhythm-tuner_linux_amd64.tar.gz: OK
```

The same works for every tool: for Soundcheck, use `rhythm-soundcheck_linux_amd64.tar.gz` and the `checksums.txt` from the Soundcheck release; for Echo, `rhythm-echo_linux_amd64.tar.gz` and Echo's.

**Windows** (PowerShell)

```powershell
$name = 'rhythm-tuner_windows_amd64.zip'
$want = ((Select-String -Path checksums.txt -Pattern " $name$").Line -split '\s+')[0]
(Get-FileHash -Algorithm SHA256 $name).Hash.ToLower() -eq $want
```

`True` means the archive matches.

## 2. Verify the signature on checksums.txt

Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/), then run this in the same folder:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

Every repository (`rhythm`, `tuner`, `soundcheck`, `echo`) releases through that same shared workflow, so the identity is the same for all of them. `Verified OK` means `checksums.txt` was signed by Rhythm's release workflow (the shared `release-go.yml` in the `RhythmShahriar/.github` repository, run from its `master` branch) through GitHub Actions. The signature is recorded in Sigstore's public transparency log. Together with step 1, that ties your archive to that workflow. Any other identity, even another workflow under `RhythmShahriar`, is rejected.

> [!NOTE]
> There is no long-lived signing key to publish or rotate. Keyless signing uses a short-lived certificate that GitHub's OIDC token proves came from the release workflow.

## Echo's npm package

`@rhythmsh/echo`, the Next.js and Node connector, is published from the Echo repository's `npm` workflow on each version tag, with [npm provenance](https://docs.npmjs.com/generating-provenance-statements): a signed statement, recorded in Sigstore's transparency log, of the repository, commit and workflow that built it. The npm website shows it as *Provenance* on the package page. To check what you installed:

```sh
npm audit signatures
```

It checks the registry signature of every installed package and the provenance attestation of those that have one; a verified attestation for `@rhythmsh/echo` means the package was built from `github.com/RhythmShahriar/echo` by GitHub Actions. Release candidates (`v1.1.0-rc1`) are published under the `next` tag, never `latest`.

On a project that has `@rhythmsh/echo` installed, the output counts verified registry signatures and verified attestations; any package listed as invalid or missing a signature is worth a look before you deploy.

## What is signed and what isn't

| Item | Checksummed | Signed |
|---|---|---|
| `rhythm` release archives | Yes, `checksums.txt` | Yes, `checksums.txt` is signed with cosign |
| Tool release archives (`rhythm-tuner`, `rhythm-soundcheck`, `rhythm-echo`) | Yes, `checksums.txt` | Yes, the same way |
| Echo's npm package, `@rhythmsh/echo` | Yes, by npm (the `integrity` hash in your lockfile) | Yes: published with [npm provenance](#echos-npm-package), tying it to the Echo repository's `npm` workflow |
| Echo's Laravel package, `rhythmsh/echo-laravel` | Composer records the commit it installed | No: Packagist serves a tag of the [`echo-laravel`](https://github.com/RhythmShahriar/echo-laravel) repository, split from the Echo repository |
| The WordPress mu-plugin Echo installs | It's embedded in `rhythm-echo`, so it's covered by the binary's checksum | Yes, as part of the binary |
| The Chromium Soundcheck downloads when no Chrome is installed | No: the go-rod library fetches a pinned Chromium snapshot over HTTPS from public mirrors (Google's Chromium snapshot storage and others) | No; it isn't a Rhythm artifact. Install Chrome or Chromium from your OS and pass `--browser` if you'd rather use a browser you've verified. |
| Tuner [check packs](Tuner-Check-Packs) (`checks-latest` release) | Yes, `checks-sha256.txt` | **No** |
| `install.sh` and `install.ps1` | No | No |

Check packs are verified against `checks-sha256.txt` before use, which protects against corruption and partial uploads. It doesn't protect against someone able to publish to the Tuner repository's releases. A pack can't run code: it's declarative YAML that can only make GET requests to the site being scanned. Tuner rejects any field it doesn't know. Scan with `--no-check-updates` to use only the checks built into your binary, which shipped in a signed release. See [Check Packs](Tuner-Check-Packs) for the full trust model.

The install scripts download from GitHub over HTTPS and refuse to install anything that doesn't match the release's `checksums.txt`. They don't check the cosign signature themselves. If you'd rather not pipe a script into your shell, download it, read it, then run it, or follow the [manual download](Install#manual-download) steps.

---
<sub>← [Uninstall](Uninstall) · [Home](Home) · [Tuner overview](Tuner) →</sub>
