How `rhythm` and its tools are released and how a new tool is added, for maintainers. A release is a pushed `vX.Y.Z` tag; a shared workflow builds, checksums, signs and publishes it. Tuner's checks also publish on their own, as a check pack, on every push to `master`. An Echo release also publishes its npm and Composer connectors.

## Release a version

`rhythm` and each tool version independently with [semantic versioning](https://semver.org/). To release one:

1. Make sure `master` is green in CI on Linux, macOS and Windows and has everything you want to ship.
2. For `rhythm`, refresh the Sigstore trusted root the kit embeds and check the release config (see [Before a rhythm release](#before-a-rhythm-release)).
3. Tag the commit and push the tag:

   ```sh
   git switch master && git pull
   git tag v0.3.2
   git push origin v0.3.2
   ```

4. The repository's `release` workflow runs, builds every platform and publishes the GitHub release with notes generated from the commits.
5. [Verify the release](#verify-a-release).

Any tag starting with `v` triggers a release, so only push tags you mean to ship. A tag with a suffix, like `v0.4.0-rc1`, is published as a **prerelease** (`release.prerelease: auto` in `.goreleaser.yaml`): GitHub never reports a prerelease as "latest", so `rhythm update` and the installers don't offer it. You can test it with `rhythm install tuner@0.4.0-rc1` or by downloading it directly.

## The release workflow

Each repository's `.github/workflows/release.yml` runs on `push` of tags matching `v*` and calls the shared, reusable workflow [`release-go.yml`](https://github.com/RhythmShahriar/.github/blob/master/.github/workflows/release-go.yml) in `RhythmShahriar/.github`. The caller must grant all three permissions the workflow asks for, or GitHub refuses to start the release:

```yaml
jobs:
  release:
    uses: RhythmShahriar/.github/.github/workflows/release-go.yml@master
    permissions:
      contents: write       # publish the release
      id-token: write       # keyless signing and attestations
      attestations: write   # GitHub artifact attestations
    secrets:
      TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}   # optional, for package managers
```

`release-go.yml`:

1. Checks out the full history without keeping the token (`persist-credentials: false`), so GoReleaser can find the previous tag.
2. Sets up Go from the repository's `go.mod`.
3. Installs cosign v3.0.6 and syft v1.54.1.
4. Runs GoReleaser v2.18.3 (`goreleaser release --clean`) with `GOWORK=off`, so the build uses exactly the kit version `go.mod` pins.
5. Attests the build provenance of `checksums.txt`, every archive, SBOM and Linux package (`actions/attest-build-provenance`).

Every action in it is pinned to a full commit SHA with its version in a comment. The tool versions are pinned too, so a moved tag upstream can't change what builds or signs a release. Dependabot keeps them current.

GoReleaser reads the repository's [`.goreleaser.yaml`](https://github.com/RhythmShahriar/rhythm/blob/master/.goreleaser.yaml):

| Step | Setting |
|---|---|
| Build | `linux`, `darwin`, `windows` × `amd64`, `arm64`; `CGO_ENABLED=0`, `-trimpath`; version stamped with `-X main.version` |
| Archive | `<binary>_<os>_<arch>.tar.gz`, `.zip` on Windows, with README and LICENSE |
| SBOM | `<archive>.sbom.json`, SPDX 2.3 JSON written by syft for every archive; Soundcheck also ships `axe-core.cdx.json`, the committed CycloneDX SBOM of the axe-core it embeds |
| Packages | `rhythm` only: `.deb`, `.rpm` and `.apk` for `amd64` and `arm64`, with the binary in `/usr/bin` and the license and README in `/usr/share/doc/rhythm` |
| Checksums | `checksums.txt`, SHA-256 of every archive, SBOM and package |
| Sign | `cosign sign-blob` over `checksums.txt`, keyless, written to `checksums.txt.sigstore.json` (the Sigstore bundle format cosign v3 writes) |
| Package managers | `rhythm` only: a Homebrew cask, a Scoop manifest and a winget pull request, pushed with `TAP_GITHUB_TOKEN` (see [Package managers](#package-managers)) |
| Notes | From GitHub, grouped into Features (`feat`), Fixes (`fix`), Performance (`perf`) and Other |

Because SBOMs and packages are listed in `checksums.txt`, the one signature covers them too.

### The naming contract

> [!IMPORTANT]
> The archive names, `checksums.txt` and `checksums.txt.sigstore.json` are a contract. `kit/update` (used by every rhythm command that installs), `install.sh` and `install.ps1` look for exactly `<binary>_<os>_<arch>.tar.gz` (`.zip` on Windows), `checksums.txt` and its signature at the tag `v<version>`. They refuse to install anything without a matching checksum line or a valid signature from `release-go.yml` for that repository and tag. Renaming the binary, the project, the archive template or the workflow breaks installs and updates for every user.

Keep these in sync when adding a tool: the GoReleaser `project_name` must equal the binary name (`rhythm-<tool>`), so archives come out as `rhythm-<tool>_<os>_<arch>`.

> [!WARNING]
> The signing identity names `release-go.yml@refs/heads/master`, which is why every caller still uses the shared workflows `@master`. Moving them to a tag or SHA changes the identity users verify. It has to land in one change with the README, `.goreleaser.yaml`, both installers, [Verifying releases](Verifying-Releases) and the refs `kit/sigverify` accepts.

### CI checks before you tag

The shared `ci-go.yml` workflow runs on every push to `master` and every pull request. On each runner in its `os` matrix: `go vet`, `go test -race`, a build and the optional benchmark. On Linux only: `gofmt`, `golangci-lint` v2.15.0, `shellcheck`, `actionlint`, the optional pinned-actions and writing-style checks, the kit pin check and `goreleaser check`. A separate job runs `govulncheck` and fails on a vulnerability in code the module calls.

| Input | Default | Effect |
|---|---|---|
| `os` | `'["ubuntu-latest"]'` | JSON list of runners; `rhythm` uses Ubuntu, Windows and macOS. Check names become `go (<os>)`, so branch protection must name those. |
| `pin-check` | `false` | fail when a third-party action isn't pinned to a 40-hex commit SHA (local actions and the shared workflows are exempt) |
| `style` | `false` | run `check-style.py` (no em dashes, no comma before "and" or "but") |
| `kit-pin-check` | `true` | fail when `go.mod` pins the kit more than one minor release behind the latest `rhythm` tag; skipped when there is no kit requirement |
| `bench`, `bench-packages` | empty, `./...` | run one benchmark regexp once; a regexp that matches nothing gives a warning |
| `tools-ref` | `master` | the ref of `RhythmShahriar/.github` the shared scripts load from |

The old `rhythm-kit` input is gone: the kit is a normal `go.mod` requirement. A caller that still passes it gets an "invalid input" error from GitHub. `rhythm`'s own CI also runs the installer tests on all three systems (see [Contributing](Contributing#ci)) and a `packages` job that builds a snapshot release and checks it. A clean CI run on the tagged commit is the best sign the release will build.

### Before a rhythm release

| Target | Does |
|---|---|
| `make trusted-root` | refreshes `kit/sigverify/trusted_root.json` from Sigstore through TUF; commit the result |
| `make release-check` | `goreleaser check` at v2.18.3 |
| `make dist-check` | a snapshot release, then checks that every archive has an SBOM, every file is in `checksums.txt`, each package holds `/usr/bin/rhythm` and the winget manifests validate (needs syft on `PATH`) |
| `make pins`, `make actionlint` | every third-party action pinned to a SHA; the workflows lint clean |
| `make vuln` | `govulncheck` |

To try the whole release workflow without publishing, run the **release-dry-run** workflow from the Actions tab on a branch. It calls `release-go.yml` with `snapshot: true`: it builds, signs and attests a snapshot and publishes nothing. It refuses to run on a tag, so a snapshot is never signed with a release tag as its source. Its attestations name the branch, which `gh attestation verify --source-ref refs/tags/v<version>` rejects.

### Package managers

The `rhythm` config also generates a Homebrew cask (`RhythmShahriar/homebrew-tap`), a Scoop manifest (`RhythmShahriar/scoop-bucket`) and a winget manifest (`RhythmShahriar.rhythm`, a pull request to `microsoft/winget-pkgs` from a fork). They are pushed only when the repository has the `TAP_GITHUB_TOKEN` secret, a fine-grained token with contents and pull-request write access on those repositories; without it they are generated into `dist/` and not uploaded. Prereleases are never pushed. Until those repositories and the secret exist, the `.deb`, `.rpm` and `.apk` assets and the installers are the way to install. The cask clears the quarantine flag on macOS, since the binaries aren't notarized.

### Changing the shared workflows

A change to `release-go.yml` or `ci-go.yml` can break every caller at once, since they all use `@master`. Merge in this order and say so in the release notes:

1. **The callers first.** When the workflow will ask for a new permission (as `attestations: write` did), add it to `release.yml` in tuner, soundcheck, echo, roadie, sightread and rhythm. GitHub refuses to start a release that asks for more than its caller grants.
2. **Tools behind on the kit.** `kit-pin-check` is on by default, so bump each tool's kit requirement (or set `kit-pin-check: false` as a stopgap) before the check lands, or its CI goes red.
3. **Then `RhythmShahriar/.github`.**
4. **Then the callers that use the new inputs.** `rhythm`'s `ci.yml` passes `os`, `pin-check`, `bench` and `bench-packages`, which a `ci-go.yml` without them rejects.

## When the kit changes

Tools import the shared kit (`github.com/RhythmShahriar/rhythm/kit/...`) as a normal Go module, pinned in their `go.mod`. A tool release builds against that pinned version (`GOWORK=off`), never against `rhythm`'s `master`. So when a tool needs a kit change, release in this order:

1. **Tag the kit.** Merge the kit change and tag a new `rhythm` version, e.g. `v0.7.0`. The kit is part of the `rhythm` module, so this is a `rhythm` release.
2. **Bump the tool.** In the tool repository:

   ```sh
   go get github.com/RhythmShahriar/rhythm@v0.7.0
   go mod tidy
   git commit -am "build(deps): require rhythm v0.7.0"
   ```

3. **Release the tool**, once that commit is green.

Tagging the kit first also means users who run `rhythm update` get the new `kit/update` logic in the router before any tool depends on it. Keep every tool within one minor release of the latest kit: the kit pin check fails a tool that falls further behind, so kit security fixes can't silently stop reaching its users.

```console
$ RHYTHM_KIT_LATEST=v0.6.0 sh .github/scripts/check-kit-pin.sh tuner/go.mod
check-kit-pin: tuner/go.mod pins github.com/RhythmShahriar/rhythm v0.1.0 but the latest is v0.6.0; move to v0.5.0 or later
```

The check reads `go.mod` only, never `go.work`, takes the latest tag from `git ls-remote` (retried once after `RHYTHM_KIT_RETRY_DELAY` seconds, default 5) unless `RHYTHM_KIT_LATEST` is set and exits 0 when current, 1 when stale and 2 when it can't tell.

## Adding a new tool

Soundcheck is the worked example: a new repository, `RhythmShahriar/soundcheck`, with the binary `rhythm-soundcheck`, reached as `rhythm soundcheck` and `rhythm sc`.

### 1. Set up the repository

- **Module and binary.** The module is `github.com/RhythmShahriar/<tool>` and the command lives in `cmd/rhythm-<tool>`. The binary must be `rhythm-<tool>`: the router runs `rhythm-<tool>` from the `PATH`.
- **The kit.** Require a tagged `rhythm` for the kit (Soundcheck requires `v0.2.1`) and use it for the banner, theme, user folders, update notice and shared flags, so the tool looks and behaves like the others.
- **The standard commands.** `version` (printing `rhythm-<tool> <version>`), `help`, the exit codes `0`, `1` (`--fail-on`), `2` (usage), `3` (couldn't run), `130` (interrupted) and `update`, which `rhythm update` calls after updating the binary to refresh the tool's data (for Soundcheck, its browser). An `update` with nothing to do should still exit `0`. `kit/cli.Main` gives a new tool all of these, plus `version --json`, `update --json` data packs and shell completion through `rhythm`; see [Extensions](Extensions#working-well-with-rhythm) for what each one buys.
- **Responsible use.** A one-time notice through `kit/config.FirstRun` (Soundcheck's marker is `soundcheck-authorised-use`) and the background release check through `kit/update.Notice` with the tool's repository and binary.
- **Release config.** Copy `.goreleaser.yaml`, `.github/workflows/release.yml` (with all three permissions) and `ci.yml` from an existing tool. Set `project_name` to the binary name so archives are named `rhythm-<tool>_<os>_<arch>`; see [the naming contract](#the-naming-contract). Copy rhythm's `sboms:` block too, so the tool's archives get SBOMs.
- **Community files.** `make hooks` and the commit message hook, a README that links into this wiki, the license and private vulnerability reporting turned on in the repository's security settings.

### 2. Release the tool

Tag and push `v1.0.0` (or whatever the first version is) as in [Release a version](#release-a-version). Then [verify the release](#verify-a-release). The release must exist before the router points at it: `rhythm install` and `rhythm update` read `/releases/latest`.

### 3. Register it in the router

The router knows the tools listed in [`cmd/rhythm/tools.json`](https://github.com/RhythmShahriar/rhythm/blob/master/cmd/rhythm/tools.json). Add one entry:

```json
{
  "name": "soundcheck",
  "repo": "RhythmShahriar/soundcheck",
  "aliases": ["sc"],
  "description": "did my change break anything? whole-site before/after checks",
  "default_install": true,
  "min_rhythm": ""
}
```

`rhythm help`, did-you-mean, completion and `rhythm install` read it; `default_install: true` puts the tool in `rhythm install --defaults`, which the installers ask for their default list. Set `min_rhythm` when the tool needs a newer router than some users have. `go test ./cmd/rhythm` checks the file. The installers still carry a built-in list (`tuner soundcheck echo` in `scripts/install.sh` and `scripts/install.ps1`) for older routers and a "Get started" line per tool: add the tool to both. Then add the tool to the wiki: its pages, the sidebar, Home, Install, Updating, Uninstall and this page.

### 4. Release rhythm

Release `rhythm` with the registration. Users get the tool with `rhythm install <tool>`. New installs get it by default.

### The order

```text
rhythm (kit the tool needs)  →  the tool's first release  →  rhythm (registers the tool)
```

1. **`rhythm` with the kit**, if the tool needs kit packages that aren't released yet. For Soundcheck that was `v0.2.0`, which added `kit/crawl`, `kit/ratelimit`, `kit/cache` and `kit/cli`. Then came `v0.2.1` for the crawl fixes it now requires. For Echo it's `v0.4.0`, which added `kit/secrets`.
2. **The tool**, requiring that `rhythm` version. For Echo, the one `v1.0.0` tag releases the binary and the npm package. `split-laravel.sh v1.0.0` then publishes the Laravel package (see [Echo's packages](#echos-packages)). A tool that others build on releases before them: Soundcheck `v1.1.0`, which imports Echo's `client` and `compare`, comes after Echo `v1.0.0`.
3. **`rhythm` again**, registering the tool and installing it by default. For Echo, `v0.5.0` registers `echo` with the alias `e` and makes the installers default to `tuner soundcheck echo`.

For Roadie it's kit `v0.8.0` and Echo `v1.1.2` (both already released), then Roadie `v1.0.0`, then rhythm `v0.9.0` registering `roadie` with the alias `ro` and `min_rhythm: "0.9.0"`, installed by default. For Sightread it's kit `v0.8.0` (already released), then Sightread `v1.0.0`, then rhythm `v0.9.0` registering `sightread` with the alias `sr` and `min_rhythm: "0.9.0"`, installed by default.

Registering a tool as a default before it has a release breaks every fresh install: the installer installs the other tools, then ends with `Installed: tuner, soundcheck. Failed: echo (see above).` and exits 1 for everyone using the default `RHYTHM_TOOLS`.

## Echo's packages

An Echo release ships three things from one tag of [`RhythmShahriar/echo`](https://github.com/RhythmShahriar/echo): the `rhythm-echo` binary (through the shared release workflow, like every tool), the Next.js and Node connector on npm and the Laravel connector on Packagist. The WordPress connector is embedded in the binary and needs nothing extra.

### npm: `@rhythmsh/echo`

The Echo repository's [`npm` workflow](https://github.com/RhythmShahriar/echo/blob/master/.github/workflows/npm.yml) runs on the same `v*` tag push. In `connectors/node`, it installs with `npm ci`, sets the package version from the tag (`npm version "${GITHUB_REF_NAME#v}"`), runs `npm test` and publishes with provenance:

```sh
npm publish --provenance --access public --tag latest   # or --tag next for a v1.1.0-rc1 style tag
```

It needs the **`NPM_TOKEN`** repository secret: an npm granular access token with write access to `@rhythmsh/echo`, used until npm Trusted Publishing is set up for the package. The workflow has `id-token: write` for the provenance statement. A tag with a suffix is published under `next`, so `npm install @rhythmsh/echo` never picks up a release candidate. Check the result on the package's npm page (the *Provenance* section) or with `npm audit signatures`; see [Verifying releases](Verifying-Releases#echos-npm-package).

### Packagist: `rhythmsh/echo-laravel`

Packagist needs a repository whose root is the package, so `connectors/laravel` is split into its own repository, [`RhythmShahriar/echo-laravel`](https://github.com/RhythmShahriar/echo-laravel), with `scripts/split-laravel.sh` in the Echo repository:

```sh
scripts/split-laravel.sh v1.0.0
```

It runs `git subtree split --prefix connectors/laravel`, pushes the result to `echo-laravel`'s `master` and creates the same tag there. Then it prints `Pushed connectors/laravel at v1.0.0 to <remote>`. `ECHO_LARAVEL_REMOTE` overrides the remote. Packagist reads `rhythmsh/echo-laravel` from that repository and picks up the new tag through its GitHub hook (or **Update** on the package's Packagist page). Run it from an up-to-date `master` after tagging Echo, since the subtree history is pushed as it is.

No workflow runs the split: the Echo repository's workflows are `ci.yml`, `release.yml` (the binaries) and `npm.yml` (the Node package), so after each Echo tag, run `scripts/split-laravel.sh` yourself.

### Before tagging Echo

- `make check` in the Echo repository, plus the connectors' own tests: `make -C connectors/laravel check` (Docker) and `cd connectors/node && npm test`.
- The `version` in `connectors/node/package.json` doesn't matter: the workflow sets it from the tag.
- For the consent checks: `make test-browser` (the consent tests that start Chrome) and `make test-integrated` (every consent stage required, as the nightly CI runs it), `make fuzz-consent` and the consent benchmark: `make -C testbed/consent up && make bench-consent && make -C testbed/consent reproduce` (sealed Docker mode, the benchmark of record), which rewrites `docs/benchmark-consent.md`. Its targets must all be met.
- The JSON Schemas in `report/schema/` (four of them for consent: `consent`, `consent-detect`, `consent-verify`, `consent-catalog`) are regenerated and the goldens updated in the same commit as any change to an output.
- `ci.yml`'s `consent-browser` job (the consent tests that start Chrome, on the hosted runners) is green.
- The mu-plugin's `Version:` header and `VERSION` constant in `connectors/wordpress/echo.php` should match the release.

## Soundcheck's axe-core

Soundcheck's archives carry axe-core's license and source under `THIRD_PARTY/axe-core/`; each release also ships `axe-core.cdx.json`, the CycloneDX SBOM of the axe-core it embeds. Updating axe-core is a reviewed change, not a Dependabot bump: `scripts/update-axe.sh <version>` in the Soundcheck repository checks the npm integrity and registry signatures, refreshes the SBOM and the rule table (`node scripts/gen-axe-rules.mjs`); it refuses to run for Dependabot.

## Roadie's profile pack and Action

A Roadie release carries, next to the binaries, `rhythm-roadie_profiles.tar.gz`: the stack profiles, so the pack can be read and checked without the binary. It is listed in the release's `checksums.txt`, so the release signature covers it. `rhythm-roadie update` (and `rhythm update`) downloads the newest pack, checks its Sigstore signature and checksum with `kit/sigverify` (the same trust path as the binary) and uses it only when it is newer than the profiles built in; `--pin profiles=<version>` picks one. Reports record the pack version.

The GitHub Action is `action.yml` in the Roadie repository. After `v1.0.0` is tagged, move the `v1` tag to it, so `uses: RhythmShahriar/roadie@v1` gets the release. The Action installs the release that matches its tag and checks it with cosign (`scripts/verify-release.sh`) before it runs.

Before tagging, run the speed benchmarks on an idle machine (`go test -run '^$' -bench . ./engine/ ./verify/ ./keys/ ./dump/mysql/`) and the sealed testbed (`make -C testbed up bench reproduce`), then commit the generated `docs/benchmark.md`. CI's `goreleaser-snapshot` job checks the archive names on every push.

## Sightread's data packs and Action

A Sightread release carries, next to the binaries, its data packs: the signal rules and the popular-name list it checks typosquats against. They're listed in the release's `checksums.txt`, so the release signature covers them. `rhythm-sightread update` (and `rhythm update`) downloads the newest pack, checks its Sigstore signature (the shared release workflow at `refs/heads/master`, repository `RhythmShahriar/sightread`, tag `v<version>`) and its checksum before using it; `--pin` picks a version. It never updates a package.

The GitHub Action is `action.yml` in the Sightread repository. After `v1.0.0` is tagged, move the `v1` tag to it, so `uses: RhythmShahriar/sightread@v1` gets the release. The Action installs the release that matches its tag and checks it with cosign before it runs; `rhythm sightread init ci` writes a workflow pinned by commit with the matching `version` input.

Before tagging, run `make perf` on an idle machine (the analyzer and lexer speed targets) and `make -C testbed bench` with the Docker WordPress up. Then commit the regenerated `docs/benchmark.md`.

## Check packs

Tuner's checks also ship without a release. The [`checks-pack`](https://github.com/RhythmShahriar/tuner/blob/master/.github/workflows/checks-pack.yml) workflow runs on every push to `master` that touches `checks/**` (or the workflow itself). It can also be run by hand:

1. Runs every check's fixtures: `go test ./engine ./checkpack`.
2. Builds the pack with `make pack` (`checks.tar.gz`, `checks-manifest.json`, `checks-sha256.txt`), byte for byte as you'd get locally from the same commit and Go version.
3. Uploads the three assets to the rolling `checks-latest` release, replacing the previous pack.

Runs never overlap: a newer run waits for the current upload to finish. The first run creates the `checks-latest` tag on a parentless commit that holds only a README, so it's never reachable from `master` and GoReleaser can't mistake it for a version tag.

> [!WARNING]
> `checks-latest` is a **prerelease** and must never be marked as the latest release. `rhythm update`, `rhythm install` and both installers read `/releases/latest`, which only returns the most recent non-prerelease, non-draft release. If `checks-latest` became "latest", installs and updates of Tuner would find no binaries. The workflow re-applies `--prerelease --latest=false` on every run; don't change it by hand in the GitHub UI.

If a checks change only behaves correctly on a newer `rhythm-tuner`, bump `min_tuner` in `checks/pack.json` in the same change. See [Check Packs](Tuner-Check-Packs) for how clients choose and verify packs.

## Verify a release

After the workflow finishes:

1. **Assets.** The release has six archives, an SBOM for each, `checksums.txt` and `checksums.txt.sigstore.json`; a `rhythm` release also has two each of `.deb`, `.rpm` and `.apk`:

   ```sh
   gh release view v0.3.2 -R RhythmShahriar/tuner --json assets --jq '.assets[].name'
   ```

   A Soundcheck release also has `axe-core.cdx.json`, listed in `checksums.txt`. A Roadie release also has `rhythm-roadie_profiles.tar.gz` and a Sightread release its data packs, each listed in `checksums.txt`.

2. **Latest.** The new version is marked *Latest* while `checks-latest` is still a *Pre-release*:

   ```sh
   gh release list -R RhythmShahriar/tuner --limit 5
   ```

   ```text
   v0.3.2       Latest       v0.3.2          …
   Check pack   Pre-release  checks-latest   …
   ```

3. **Signature.** On a machine with the router, `rhythm install tuner@0.3.2 --dir "$(mktemp -d)"` should print `Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)`. By hand, download `checksums.txt` and its bundle and check them with cosign and gh as described in [Verifying releases](Verifying-Releases):

   ```sh
   gh release download v0.3.2 -R RhythmShahriar/tuner -p 'checksums.txt*'
   cosign verify-blob --bundle checksums.txt.sigstore.json \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
     --certificate-github-workflow-repository RhythmShahriar/tuner \
     --certificate-github-workflow-ref refs/tags/v0.3.2 \
     checksums.txt
   gh attestation verify checksums.txt --repo RhythmShahriar/tuner \
     --signer-workflow RhythmShahriar/.github/.github/workflows/release-go.yml \
     --source-ref refs/tags/v0.3.2 --deny-self-hosted-runners
   ```

4. **Update path.** On a machine with the previous version, `rhythm update --check` should exit 10 and offer the new one. `rhythm update` should install it:

   ```text
   rhythm-tuner       0.3.1 → 0.3.2 available
   ```

5. **Fresh install.** Run the installer in a clean environment, such as a throwaway container or VM. Then check `rhythm list` (every tool `yes` under SIGNED) and `rhythm verify`.

6. **Notes.** Read the generated release notes and edit them if a change needs more explanation, such as a breaking change or a [shared workflow change](#changing-the-shared-workflows) callers must follow.

---
<sub>← [Contributing](Contributing) · [Home](Home) · [Architecture](Architecture) →</sub>
