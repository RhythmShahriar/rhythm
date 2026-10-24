For a client network, an air-gapped server or a build image that can't reach GitHub: pack the releases `rhythm.lock` pins into one file where you have the network, carry it over and install from it with no network at all. Every signature and hash is checked on both sides.

## Make a bundle

You need a `rhythm.lock` first (see [Pinning and locking](Pinning-And-Locking#lock-a-projects-versions)). In the folder that holds it:

```console
$ rhythm bundle --locked -o rhythm-bundle.tar.gz --platform linux/amd64,windows/amd64
Using /home/me/site/rhythm.lock
rhythm-echo        1.0.0  linux_amd64, windows_amd64
rhythm-soundcheck  1.1.0  linux_amd64, windows_amd64
rhythm-tuner       0.3.2  linux_amd64, windows_amd64
Wrote /home/me/site/rhythm-bundle.tar.gz (34.1 MB)
```

| Flag | Effect |
|---|---|
| `--locked` | required: bundle the versions in `rhythm.lock` |
| `-o file`, `--output file` | where to write the bundle |
| `--platform os/arch,...` | the platforms to pack (default this machine's); `rhythm.lock` must lock an archive for each |

Before anything is written, rhythm checks each release's signature (and that the signer is the locked one), `checksums.txt` and every archive against the lock. The file is written through a temporary file and renamed, so a failed run leaves nothing behind:

```console
$ rhythm bundle --locked -o x.tgz --platform plan9/amd64
Using /home/me/site/rhythm.lock
rhythm-echo        1.0.0: rhythm.lock has no archive for plan9_amd64 (it locks darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64); add it by running rhythm lock where all of them are released
rhythm-soundcheck  1.1.0: rhythm.lock has no archive for plan9_amd64 (it locks darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64); add it by running rhythm lock where all of them are released
rhythm-tuner       0.3.2: rhythm.lock has no archive for plan9_amd64 (it locks darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64); add it by running rhythm lock where all of them are released
rhythm: /home/me/site/x.tgz not written
```

Usage errors exit 2:

```text
rhythm: bundle needs --locked: it packs the versions rhythm.lock pins (write one with: rhythm lock)
rhythm: bundle needs -o <file> (or --output), such as -o rhythm-bundle.tar.gz
rhythm: bundle takes no names; it bundles what rhythm.lock pins
rhythm: --platform: invalid platform "linux"; use os/arch, such as linux/amd64
```

## Install from it

Copy the bundle and the same `rhythm.lock` to the other machine, with a `rhythm` already on it (the installer, a package or a manual download, see [Install](Install)). Then, in the folder with the lock:

```console
$ rhythm install --from rhythm-bundle.tar.gz --locked
Using /home/me/site/rhythm.lock
Installed rhythm-echo 1.0.0 to /home/me/.local/bin/rhythm-echo
  Signature ok  RhythmShahriar/echo @ refs/tags/v1.0.0 (release-go.yml)
  Checksum ok   sha256:419130487999167ce27cf0d67c519bd36f35b8b270d307b32a3cee314cc3a5f2
Installed rhythm-soundcheck 1.1.0 to /home/me/.local/bin/rhythm-soundcheck
  Signature ok  RhythmShahriar/soundcheck @ refs/tags/v1.1.0 (release-go.yml)
  Checksum ok   sha256:b1528d466f561d5150e52de257d9097ee79a8bf4fa90fd001dc80b0335ee1ed2
Installed rhythm-tuner 0.3.2 to /home/me/.local/bin/rhythm-tuner
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
```

That run had `RHYTHM_OFFLINE=1` set and `RHYTHM_GITHUB_API` pointing at a closed port: `install --from` sends no request at all. The signatures are checked again with the Sigstore root built into rhythm and, with `--locked`, every archive against `rhythm.lock`. Name tools to install only some (`rhythm install --from rhythm-bundle.tar.gz --locked tuner`). Without `--locked`, `--from` serves the latest version in the bundle or an exact one; a partial spec such as `tuner@0.3` fails with `this release source can't list releases; ask for latest or an exact version, with no minimum age`.

> [!TIP]
> The SIGNED column of `rhythm list` shows `yes` for tools installed this way. `rhythm verify` works once the machine can reach GitHub again.

A bundle is refused, with nothing installed, when:

| Problem | Message |
|---|---|
| it has nothing for this machine | `<bundle> was made for plan9_amd64 and has nothing for this machine (linux_amd64); make one with: rhythm bundle --locked --platform linux/amd64` |
| it lacks a tool | `<bundle> has no RhythmShahriar/echo` |
| a file changed after it was made | `<bundle>: <file> doesn't match its sha256 in the manifest; the bundle was changed after it was made; refusing the bundle` |
| a name points outside it | `<bundle>: unsafe entry name "../../evil"; refusing the bundle` |
| an entry isn't a plain file | `<bundle>: <file> isn't a plain file; refusing the bundle` |
| a file appears twice, or isn't in the manifest | `<bundle>: <file> appears twice; ...`, `<bundle>: the bundle holds <file>, which the manifest doesn't name; ...` |
| it isn't a bundle | `<bundle> has no manifest.json; it isn't a rhythm bundle` |

Nothing is extracted until the whole file has passed.

## What's in a bundle

A bundle is a `.tar.gz`:

```text
releases/RhythmShahriar/echo/v1.0.0/checksums.txt
releases/RhythmShahriar/echo/v1.0.0/checksums.txt.sigstore.json
releases/RhythmShahriar/echo/v1.0.0/rhythm-echo_linux_amd64.tar.gz
releases/RhythmShahriar/echo/v1.0.0/rhythm-echo_windows_amd64.zip
releases/RhythmShahriar/soundcheck/v1.1.0/...
releases/RhythmShahriar/tuner/v0.3.2/...
manifest.json
```

`manifest.json` (schema `rhythm-bundle/v1`) holds when it was made, the rhythm version that made it, the platforms, a copy of the lock and, for each release, every file's path, sha256 and size. It also has a `data` list for tools' data packs; no tool offers its packs for bundling yet, so today it is empty and the tools fetch their data on first use.

The release archives inside are the release's own, so `rhythm.lock` and the bundle carry the same hashes. When the lock pins rhythm itself, its archives go in too.

## Without a bundle

`RHYTHM_OFFLINE=1` (any value but empty or `0`) stops every rhythm command and tool built on the shared kit from using the network. Release checks and notices are skipped; anything that would download says so once and exits 1. `rhythm update` skips the tools' data refresh as well and says so in one line:

```text
RHYTHM_OFFLINE is set, so nothing was downloaded for rhythm-echo, rhythm-soundcheck, rhythm-tuner; unset it to reach the release API
RHYTHM_OFFLINE is set, so no tool's data was refreshed either; unset it to refresh it
Updated 0, up to date 0, failed 3: rhythm-echo, rhythm-soundcheck, rhythm-tuner
```

For a network that reaches a GitHub mirror rather than nothing, set `RHYTHM_GITHUB_API` (and `RHYTHM_GITHUB_TOKEN` for it) instead; see [Release API and tokens](Command-Reference#release-api-and-tokens).

---
<sub>← [Pinning and locking](Pinning-And-Locking) · [Home](Home) · [CI setup](CI-Setup) →</sub>
