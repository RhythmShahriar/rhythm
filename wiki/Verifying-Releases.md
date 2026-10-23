Every `rhythm` and tool release ships a `checksums.txt`, and that file is signed with keyless [cosign](https://docs.sigstore.dev/) from GitHub Actions. This page shows how to check both, and what is and isn't signed.

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

`Verified OK` means `checksums.txt` was signed by Rhythm's release workflow (the shared `release-go.yml` in the `RhythmShahriar/.github` repository, run from its `master` branch) through GitHub Actions, and the signature is recorded in Sigstore's public transparency log. Together with step 1, that ties your archive to that workflow. Any other identity, even another workflow under `RhythmShahriar`, is rejected.

> [!NOTE]
> There is no long-lived signing key to publish or rotate. Keyless signing uses a short-lived certificate that GitHub's OIDC token proves came from the release workflow.

## What is signed and what isn't

| Item | Checksummed | Signed |
|---|---|---|
| `rhythm` release archives | Yes, `checksums.txt` | Yes, `checksums.txt` is signed with cosign |
| Tool release archives (e.g. `rhythm-tuner`) | Yes, `checksums.txt` | Yes, the same way |
| Tuner [check packs](Tuner-Check-Packs) (`checks-latest` release) | Yes, `checks-sha256.txt` | **No** |
| `install.sh` and `install.ps1` | No | No |

Check packs are verified against `checks-sha256.txt` before use, which protects against corruption and partial uploads, but not against someone able to publish to the Tuner repository's releases. A pack can't run code: it's declarative YAML that can only make GET requests to the site being scanned, and Tuner rejects any field it doesn't know. Scan with `--no-check-updates` to use only the checks built into your binary, which shipped in a signed release. See [Check Packs](Tuner-Check-Packs) for the full trust model.

The install scripts download from GitHub over HTTPS and refuse to install anything that doesn't match the release's `checksums.txt`. They don't check the cosign signature themselves. If you'd rather not pipe a script into your shell, download it, read it, then run it, or follow the [manual download](Install#manual-download) steps.

---
<sub>← [Uninstall](Uninstall) · [Home](Home) · [Tuner overview](Tuner) →</sub>
