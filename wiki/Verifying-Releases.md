Every `rhythm` and tool release ships a `checksums.txt` signed with keyless [Sigstore](https://docs.sigstore.dev/) from GitHub Actions, plus GitHub artifact attestations. `rhythm` checks the signature on every install and update. This page explains what that proves, how to check what you have with `rhythm verify` and how to check a download by hand with cosign or the GitHub CLI.

## What the checks prove

| Check | Catches | Can't tell you |
|---|---|---|
| the archive against `checksums.txt` | a corrupted or cut-off download | who published the release: `checksums.txt` sits on the same release page, so anyone who can upload there can upload a matching pair |
| the Sigstore signature over `checksums.txt` | files swapped on the release page or by a mirror, a release built from another repository or tag | whether the code itself is safe |

The signature's certificate is issued to a GitHub Actions run and records which workflow ran, for which repository and which tag. rhythm accepts a release only when all three match: the shared `release-go.yml` workflow from `RhythmShahriar/.github` (`@refs/heads/master`), the tool's own repository (`RhythmShahriar/tuner` for Tuner) and the tag `v<version>` of the version installed. The workflow alone isn't enough: it is a public reusable workflow that any repository can call, so a signature made for someone else's project would carry the same workflow. The repository and tag pin it to this project and this release. Every signature is also recorded in Sigstore's public transparency log.

> [!NOTE]
> There is no long-lived signing key to publish or rotate. Keyless signing uses a short-lived certificate that GitHub's OIDC token proves came from the release workflow.

## What rhythm checks for you

`rhythm install`, `rhythm update`, `rhythm lock`, `rhythm bundle` and `rhythm install --locked` check the signature before they trust any checksum. They fail closed: a missing `checksums.txt.sigstore.json`, a signature that doesn't verify or one from another identity installs nothing and leaves the old binary in place. After each verified install two lines say what was checked:

```text
Installed rhythm-tuner 0.3.2 to /home/me/.local/bin/rhythm-tuner
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
```

`Signature ok` names the repository and tag the certificate was issued for and the workflow that signed it. `Checksum ok` is the sha256 of the archive that went in, the same one `checksums.txt` and `rhythm.lock` list. A release that fails reads:

```text
rhythm-tuner       refusing to install RhythmShahriar/tuner 0.4.0: signature check failed: <reason>; nothing was changed. The release may have been tampered with, or a mirror re-packed it. Only if you trust where it comes from, rerun with --skip-signature
```

`--skip-signature` installs without the check, after `warning: --skip-signature: release signatures aren't checked, so a tampered release would install; use it only with a mirror you trust`. The checksums are still checked. `rhythm ext install` has no such flag.

The Sigstore trusted root is refreshed online (through TUF, into `~/.cache/rhythm/rhythm/tuf`) on the first check of a command, at most once a day. When that fails, or with `RHYTHM_OFFLINE` set or `install --from`, rhythm uses the root built into it; `RHYTHM_DEBUG=1` says so: `rhythm: using the built-in Sigstore root: <reason>`.

### The SIGNED column

```console
$ rhythm list --offline
TOOL        VERSION  SIGNED  PATH
rhythm      dev      no      /home/me/.local/bin/rhythm
echo        1.0.0    no      /home/me/.local/bin/rhythm-echo
soundcheck  1.1.0    yes     /home/me/.local/bin/rhythm-soundcheck
tuner       0.3.2    yes     /home/me/.local/bin/rhythm-tuner
```

`yes` means the binary passed a signature check (at install, update or `rhythm verify`) and hasn't changed since. rhythm records the sha256 of each verified binary in `verified.json` in its config folder and compares the file with it, so the column needs no network. Above, one byte was added to `rhythm-echo` after it was installed. `rhythm` is a build from source, which has no release to check. A third-party `rhythm-*` program shows `-`.

### rhythm verify

`rhythm verify` checks what you already have, against the release it says it is:

```console
$ rhythm verify echo
rhythm-echo 1.0.0 (/home/me/.local/bin/rhythm-echo)
  Signature ok  RhythmShahriar/echo @ refs/tags/v1.0.0 (release-go.yml)
  Checksum ok   sha256:419130487999167ce27cf0d67c519bd36f35b8b270d307b32a3cee314cc3a5f2
rhythm-echo        the installed binary differs from the one in the signed rhythm-echo_linux_amd64.tar.gz; it may have been changed after install. Reinstall it with: rhythm install echo --force
$ echo $?
1
```

For rhythm and each installed tool and extension (or the ones named), it fetches the installed version's `checksums.txt` and signature and checks the signature, gets the archive (from the download cache when it's there), checks it against `checksums.txt`, unpacks the binary and compares it with the installed file byte for byte. An intact binary ends with `Binary ok     same bytes as in rhythm-echo_linux_amd64.tar.gz`. It exits 0 when every binary checked out and 1 when any didn't or couldn't be checked; `--json` prints [`rhythm-verify/v1`](Command-Reference#json-output). The installers run `rhythm verify rhythm` right after installing rhythm.

| Message | Means |
|---|---|
| `the installed binary differs from the one in the signed <archive>; it may have been changed after install. Reinstall it with: rhythm install tuner --force` | the file on disk isn't the released one; for rhythm itself the advice is the install script |
| `<archive> doesn't match the signed checksums.txt (sha256 <got>, expected <want>); the release may have been tampered with` | the release asset was replaced |
| `can't vouch for RhythmShahriar/tuner 0.3.3: signature check failed: <reason>; the release may have been tampered with, or a mirror re-packed it` | the signature is missing, invalid or from another identity |
| `a development build; only released binaries can be verified` | a build from source or `go install` |
| `version unknown (<why>), can't verify it` | the binary doesn't say its version |
| `isn't installed; install it with: rhythm install tuner` | a tool you named isn't on `PATH` |

## What a release contains

Each release of `rhythm` or a tool has the same assets:

| Asset | What it is |
|---|---|
| `<binary>_<os>_<arch>.tar.gz` | The binary for Linux or macOS, with README and LICENSE, e.g. `rhythm-tuner_darwin_arm64.tar.gz` |
| `<binary>_windows_<arch>.zip` | The same for Windows |
| `<archive>.sbom.json` | The software bill of materials of each archive, SPDX 2.3 JSON (from rhythm v0.7.0 and the tools' next releases) |
| `rhythm_<version>_<arch>.deb`, `.rpm`, `.apk` | `rhythm` only: Linux packages (from v0.7.0; see [Install](Install#linux-packages)) |
| `checksums.txt` | SHA-256 of every archive, SBOM and package |
| `checksums.txt.sigstore.json` | The Sigstore signature bundle for `checksums.txt` |

Platforms: `linux`, `darwin`, `windows` on `amd64` and `arm64`. The build provenance of `checksums.txt` and every archive, SBOM and package is also published as a GitHub artifact attestation.

## Check a download by hand

Download the archive, `checksums.txt` and `checksums.txt.sigstore.json` from the same release, into one folder:

- https://github.com/RhythmShahriar/rhythm/releases/latest
- https://github.com/RhythmShahriar/tuner/releases/latest
- https://github.com/RhythmShahriar/soundcheck/releases/latest
- https://github.com/RhythmShahriar/echo/releases/latest

### With cosign

Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) 2.4 or later, then run this in the same folder, with the repository and the version of what you downloaded:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
  --certificate-github-workflow-repository RhythmShahriar/tuner \
  --certificate-github-workflow-ref refs/tags/v0.3.2 \
  checksums.txt
```

`Verified OK` means `checksums.txt` was signed by the shared release workflow, run for that repository at that tag. Use `RhythmShahriar/rhythm`, `RhythmShahriar/soundcheck` or `RhythmShahriar/echo` for the others. All four flags matter: without the repository and ref flags the command would accept a signature made by any project that calls the same public workflow. Releases signed before the release workflow moved to cosign v3 carry the older cosign bundle format; rhythm reads both.

Then check the archive against `checksums.txt`:

**Linux**

```sh
sha256sum --ignore-missing -c checksums.txt
```

**macOS**

```sh
shasum -a 256 --ignore-missing -c checksums.txt
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

`True` means the archive matches. Together the two steps tie your archive to that workflow, repository and tag.

### With the GitHub CLI

Releases from rhythm v0.7.0 and the tools' next releases on also carry GitHub artifact attestations, which `gh` checks without cosign, for `checksums.txt` or any archive, SBOM or package:

```sh
gh attestation verify checksums.txt --repo RhythmShahriar/tuner \
  --signer-workflow RhythmShahriar/.github/.github/workflows/release-go.yml \
  --source-ref refs/tags/v0.3.2 --deny-self-hosted-runners
```

Always pass `--source-ref`: the release workflow's dry run, started from a branch, is attested by the same workflow, so without it a file built from any branch would verify too. Older releases have no attestations; use cosign for those.

### In the installers

`install.sh` and `install.ps1` check `checksums.txt` of `rhythm` with cosign or `gh attestation verify` themselves when one of them is installed, with the same identity, repository and tag pinned, before anything is written. Without either they say `signature not checked (no cosign or gh); install cosign or gh to check it before anything runs`. The new `rhythm` then checks itself with `rhythm verify rhythm`. See [Install](Install#signature-checks).

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
| `rhythm` release archives, SBOMs and Linux packages | Yes, `checksums.txt` | Yes, `checksums.txt` is signed with Sigstore and attested; rhythm checks it on every install and update |
| Tool release archives (`rhythm-tuner`, `rhythm-soundcheck`, `rhythm-echo`) | Yes, `checksums.txt` | Yes, the same way |
| [Extension](Extensions) release archives | Yes, `checksums.txt` | Yes, by a workflow of the extension's own repository; rhythm refuses one that isn't, or that changes workflow |
| Echo's npm package, `@rhythmsh/echo` | Yes, by npm (the `integrity` hash in your lockfile) | Yes: published with [npm provenance](#echos-npm-package), tying it to the Echo repository's `npm` workflow |
| Echo's Laravel package, `rhythmsh/echo-laravel` | Composer records the commit it installed | No: Packagist serves a tag of the [`echo-laravel`](https://github.com/RhythmShahriar/echo-laravel) repository, split from the Echo repository |
| The WordPress mu-plugin Echo installs | It's embedded in `rhythm-echo`, so it's covered by the binary's checksum | Yes, as part of the binary |
| The Chromium Soundcheck downloads when no Chrome is installed | No: the go-rod library fetches a pinned Chromium snapshot over HTTPS from public mirrors (Google's Chromium snapshot storage and others) | No; it isn't a Rhythm artifact. Install Chrome or Chromium from your OS and pass `--browser` if you'd rather use a browser you've verified. |
| Tuner [check packs](Tuner-Check-Packs) (`checks-latest` release) | Yes, `checks-sha256.txt` | **No** |
| `install.sh` and `install.ps1` | No | No: they are served from `rythm.sh` over HTTPS |

Check packs are verified against `checks-sha256.txt` before use, which protects against corruption and partial uploads. It doesn't protect against someone able to publish to the Tuner repository's releases. A pack can't run code: it's declarative YAML that can only make GET requests to the site being scanned. Tuner rejects any field it doesn't know. Scan with `--no-check-updates` to use only the checks built into your binary, which shipped in a signed release. See [Check Packs](Tuner-Check-Packs) for the full trust model.

The install scripts download from GitHub over HTTPS only and refuse to install anything that doesn't match the release's `checksums.txt`; they check its signature when cosign or gh is there. The scripts themselves aren't signed. If you'd rather not pipe a script into your shell, download it, read it, then run it, or follow the [manual download](Install#manual-download) steps.

---
<sub>← [Uninstall](Uninstall) · [Home](Home) · [Command reference](Command-Reference) →</sub>
