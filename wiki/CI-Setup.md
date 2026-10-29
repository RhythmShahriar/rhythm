How to get `rhythm` and its tools onto a CI runner: the `setup-rhythm` GitHub Action for GitHub, the installer for everything else and how to keep both fast, pinned and verified. What to run once the tools are there is on each tool's CI page: [Tuner](Tuner-CI), [Soundcheck](Soundcheck-CI), [Echo](Echo-CI), [Roadie](Roadie-CI) and [Sightread](Sightread-CI).

## GitHub Actions

```yaml
steps:
  - uses: actions/checkout@v5
  - uses: RhythmShahriar/rhythm/setup@v0.7.0
    with:
      tools: tuner@0.3.2 soundcheck   # or: locked: true
  - run: rhythm tuner scan https://staging.yoursite.com --fail-on high
```

The action ships in the `rhythm` repository (in `setup/`) and is versioned with it: use the tag of a `rhythm` release (`@v0.7.0` or later). On Linux, macOS and Windows runners it:

1. works out the install folder and the cache key;
2. restores the download cache;
3. installs `rhythm` with the repository's own installer from the same tag (`install.sh`, or `install.ps1` on Windows), with no tools;
4. runs `rhythm install --dir <folder> <tools>` (or `--locked`) with the download cache on and signatures checked;
5. puts the folder on `PATH` for the steps that follow.

Roadie and Sightread also have their own Actions, which install and verify the release themselves: `RhythmShahriar/roadie@v1` refreshes staging or verifies a copy (see [Roadie CI](Roadie-CI)) and `RhythmShahriar/sightread@v1` reviews lockfile changes (see [Sightread CI](Sightread-CI)).

The install step prints rhythm's own lines, so the log shows what was checked. It leaves out rhythm's `Note: <dir> isn't on your PATH.`, since the step puts the folder on `PATH` itself. From a run of the step's script, with a runner's paths:

```text
Installed rhythm-tuner 0.3.2 to /home/runner/work/_temp/setup-rhythm/bin/rhythm-tuner
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
Installed rhythm-soundcheck 1.1.0 to /home/runner/work/_temp/setup-rhythm/bin/rhythm-soundcheck
  Signature ok  RhythmShahriar/soundcheck @ refs/tags/v1.1.0 (release-go.yml)
  Checksum ok   sha256:b1528d466f561d5150e52de257d9097ee79a8bf4fa90fd001dc80b0335ee1ed2
```

and sets `tool-versions` to `tuner@0.3.2 soundcheck@1.1.0`.

### Inputs

| Input | Default | Effect |
|---|---|---|
| `tools` | none | tools to install, separated by spaces or commas; `tool@version` pins one (`tuner@0.3.2`, `tuner@0.3`). With `locked`, names narrow `rhythm.lock` and a version must be the locked one. |
| `version` | the latest release | the `rhythm` version to install, such as `0.7.0` |
| `locked` | `false` | install the versions `rhythm.lock` pins (`rhythm install --locked`); the lock is part of the cache key |
| `skip-signature` | `false` | skip the signature checks, for a mirror you trust; checksums are still checked |
| `token` | `${{ github.token }}` | passed as `GITHUB_TOKEN`, which rhythm and the installer send to `api.github.com` only, to lift the rate limit |
| `install-dir` | a folder under the runner's temp folder | where rhythm and the tools go |
| `working-directory` | `.` | where to run, which is where `rhythm.lock` is looked for |

`locked` and `skip-signature` take `true`, `false`, `yes`, `no`, `1` or `0` in any case, on every runner. Any other value fails the first step before anything is downloaded: `setup-rhythm: skip-signature must be true or false, got 'maybe'`.

### Outputs

| Output | Example |
|---|---|
| `rhythm-version` | `0.7.0` |
| `tool-versions` | `tuner@0.3.2 soundcheck@1.1.0`, the tools in place, installed or already there |
| `install-dir` | the folder on `PATH` |
| `cache-dir` | the download cache folder the action restores and saves |
| `cache-hit` | `true` when the cache was restored for exactly this key |

```yaml
  - id: rhythm
    uses: RhythmShahriar/rhythm/setup@v0.7.0
    with:
      tools: tuner
  - run: echo "Scanning with ${{ steps.rhythm.outputs.tool-versions }}"
```

### Pinned and locked

`tools: tuner@0.3.2` pins one tool in the workflow. To keep a project's versions in one place, commit a [`rhythm.lock`](Pinning-And-Locking) and use `locked: true`: every run installs exactly those versions, checks each archive against its locked sha256 and fails the step on any mismatch. Move the lock forward with `rhythm update --lock` in a pull request, so a new release reaches CI only when you merge it.

```yaml
  - uses: RhythmShahriar/rhythm/setup@v0.7.0
    with:
      locked: true
      working-directory: site     # where rhythm.lock is
```

With `locked: true` and no lock, the step fails: `setup-rhythm: locked is true but there is no rhythm.lock in <dir>; write one with: rhythm lock`. A version in `tools` that the lock doesn't pin fails it too. A scheduled job can tell you when the lock is behind, since `rhythm update --lock --check` exits 10:

```yaml
  - run: rhythm update --lock --check
```

### Cache

The action caches the verified archives in `cache-dir` (the `rhythm/downloads` folder of rhythm's cache, which honors `RHYTHM_CACHE_DIR`). The key is `setup-rhythm-v1-<os>-<arch>-<rhythm.lock hash or none>-<hash of version and tools>`, with the OS and architecture prefix as the restore key, so a run downloads each release once and later runs only fetch `checksums.txt` and its signature. A cached archive is hashed again before it is used; one that doesn't match is downloaded again.

### What can fail the step

| Message | Means |
|---|---|
| `setup-rhythm: rhythm install failed (exit N); see above` | a tool didn't install; rhythm's own lines above say why |
| `setup-rhythm: rhythm isn't in <dir>; the install step failed` | the installer step failed |
| `setup-rhythm: <input> must be true or false, got '<value>'` | a bad `locked` or `skip-signature` |
| `setup-rhythm: locked is true but there is no rhythm.lock in <dir>; write one with: rhythm lock` | |

A refused signature or a checksum mismatch stops the install, so a tampered release never reaches the steps after it.

## Other CI systems

Use the installer, with the same options as on a laptop (see [Install](Install#installer-options)):

```yaml
# .gitlab-ci.yml
scan:
  image: ubuntu:24.04
  variables:
    RHYTHM_TOOLS: tuner@0.3.2
  script:
    - apt-get update && apt-get install -y curl
    - curl -fsSL https://rythm.sh/install | sh
    - export PATH="$HOME/.local/bin:$PATH"
    - rhythm tuner scan https://staging.yoursite.com --fail-on high
```

| Want | Set |
|---|---|
| the project's locked versions | `RHYTHM_LOCKED=1`, run from the folder with `rhythm.lock` |
| no rate limit on shared runners | `GITHUB_TOKEN` (sent to `api.github.com` only) |
| a GitHub mirror | `RHYTHM_GITHUB_API` and `RHYTHM_GITHUB_TOKEN` |
| each archive downloaded once | `RHYTHM_DOWNLOAD_CACHE=1`; cache `~/.cache/rhythm/rhythm/downloads` between runs |
| no network at all | a bundle and `rhythm install --from`; see [Offline installs](Offline-Installs) |

The installer checks rhythm's signature when cosign or gh is on the runner and `rhythm verify rhythm` checks it either way; the tools are always checked. On a runner with an older `rhythm` already installed, `rhythm update --check` exits 10 when something is out of date, which a pipeline can use as a gate.

---
<sub>← [Offline installs](Offline-Installs) · [Home](Home) · [Extensions](Extensions) →</sub>
