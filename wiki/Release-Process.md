How `rhythm` and its tools are released, and how a new tool is added, for maintainers. A release is a pushed `vX.Y.Z` tag; a shared workflow builds, checksums, signs and publishes it. Tuner's checks also publish on their own, as a check pack, on every push to `master`, and an Echo release also publishes its npm and Composer connectors.

## Release a version

`rhythm` and each tool version independently with [semantic versioning](https://semver.org/). To release one:

1. Make sure `master` is green in CI and has everything you want to ship.
2. Tag the commit and push the tag:

   ```sh
   git switch master && git pull
   git tag v0.3.2
   git push origin v0.3.2
   ```

3. The repository's `release` workflow runs, builds every platform and publishes the GitHub release with notes generated from the commits.
4. [Verify the release](#verify-a-release).

Any tag starting with `v` triggers a release, so only push tags you mean to ship. A tag with a suffix, like `v0.4.0-rc1`, is published as a **prerelease** (`release.prerelease: auto` in `.goreleaser.yaml`): GitHub never reports a prerelease as "latest", so `rhythm update` and the installers don't offer it, and you can test it by downloading it directly.

## The release workflow

Each repository's `.github/workflows/release.yml` runs on `push` of tags matching `v*` and calls the shared, reusable workflow [`release-go.yml`](https://github.com/RhythmShahriar/.github/blob/master/.github/workflows/release-go.yml) in `RhythmShahriar/.github`, with `contents: write` (to publish the release) and `id-token: write` (for keyless signing).

`release-go.yml`:

1. Checks out the full history, so GoReleaser can find the previous tag.
2. Sets up Go from the repository's `go.mod`.
3. Optionally links a checkout of `rhythm` with `go.work` (the `rhythm-kit` input; see [When the kit changes](#when-the-kit-changes)).
4. Installs cosign.
5. Runs `goreleaser release --clean`.

GoReleaser reads the repository's [`.goreleaser.yaml`](https://github.com/RhythmShahriar/rhythm/blob/master/.goreleaser.yaml):

| Step | Setting |
|---|---|
| Build | `linux`, `darwin`, `windows` × `amd64`, `arm64`; `CGO_ENABLED=0`, `-trimpath`; version stamped with `-X main.version` |
| Archive | `<binary>_<os>_<arch>.tar.gz`, `.zip` on Windows, with README and LICENSE |
| Checksums | `checksums.txt`, SHA-256 of every archive |
| Sign | `cosign sign-blob` over `checksums.txt`, keyless, written to `checksums.txt.sigstore.json` |
| Notes | From GitHub, grouped into Features (`feat`), Fixes (`fix`), Performance (`perf`) and Other |

### The naming contract

> [!IMPORTANT]
> The archive names and `checksums.txt` are a contract. `kit/update` (used by `rhythm update` and `rhythm install`), `install.sh` and `install.ps1` look for exactly `<binary>_<os>_<arch>.tar.gz` (`.zip` on Windows) and `checksums.txt`, and refuse to install anything without a matching checksum line. Renaming the binary, the project or the archive template breaks installs and updates for every user.

Keep these in sync when adding a tool: the GoReleaser `project_name` must equal the binary name (`rhythm-<tool>`), so archives come out as `rhythm-<tool>_<os>_<arch>`.

### CI checks before you tag

The shared `ci-go.yml` workflow runs on every push to `master` and every pull request: `gofmt`, `go vet`, `golangci-lint`, `go test -race`, a build, `shellcheck`, and `goreleaser check` to validate the release config. A clean CI run on the tagged commit is the best sign the release will build.

## When the kit changes

Tools import the shared kit (`github.com/RhythmShahriar/rhythm/kit/...`) as a normal Go module, pinned in their `go.mod` (Tuner currently requires `github.com/RhythmShahriar/rhythm v0.1.0`, Soundcheck `v0.2.1`, Echo `v0.4.0`). A tool release builds against that pinned version, not against `rhythm`'s `master`. So when a tool needs a kit change, release in this order:

1. **Release `rhythm` first.** Merge the kit change and tag a new `rhythm` version, e.g. `v0.2.0`.
2. **Bump the tool.** In the tool repository:

   ```sh
   go get github.com/RhythmShahriar/rhythm@v0.2.0
   go mod tidy
   git commit -am "build(deps): require rhythm v0.2.0"
   ```

3. **Release the tool**, once that commit is green.

Releasing `rhythm` first also means users who run `rhythm update` get the new `kit/update` logic in the router before any tool depends on it.

<details>
<summary>The <code>rhythm-kit</code> workflow input</summary>

`ci-go.yml` and `release-go.yml` both accept `rhythm-kit: true`. It checks out `rhythm`'s default branch and links it with a `go.work` file outside the repository, so a tool can build against an untagged kit. It exists for a tool that doesn't yet require a tagged kit. Tuner doesn't use it; prefer tagging `rhythm` and pinning the version, so every release builds from a fixed, reproducible kit.

</details>

## Adding a new tool

Soundcheck is the worked example: a new repository, `RhythmShahriar/soundcheck`, with the binary `rhythm-soundcheck`, reached as `rhythm soundcheck` and `rhythm sc`.

### 1. Set up the repository

- **Module and binary.** The module is `github.com/RhythmShahriar/<tool>` and the command lives in `cmd/rhythm-<tool>`. The binary must be `rhythm-<tool>`: the router runs `rhythm-<tool>` from the `PATH`.
- **The kit.** Require a tagged `rhythm` for the kit (Soundcheck requires `v0.2.1`) and use it for the banner, theme, user folders, update notice and shared flags, so the tool looks and behaves like the others.
- **The standard commands.** `version` (printing `rhythm-<tool> <version>`), `help`, the exit codes `0`, `1` (`--fail-on`), `2` (usage), `3` (couldn't run), `130` (interrupted), and `update`, which `rhythm update` calls after updating the binary to refresh the tool's data (for Soundcheck, its browser). An `update` with nothing to do should still exit `0`.
- **Responsible use.** A one-time notice through `kit/config.FirstRun` (Soundcheck's marker is `soundcheck-authorised-use`), and the background release check through `kit/update.Notice` with the tool's repository and binary.
- **Release config.** Copy `.goreleaser.yaml`, `.github/workflows/release.yml` and `ci.yml` from an existing tool. Set `project_name` to the binary name so archives are named `rhythm-<tool>_<os>_<arch>`; see [the naming contract](#the-naming-contract).
- **Community files.** `make hooks` and the commit message hook, a README that links into this wiki, the license, and private vulnerability reporting turned on in the repository's security settings.

### 2. Release the tool

Tag and push `v1.0.0` (or whatever the first version is) as in [Release a version](#release-a-version), and [verify the release](#verify-a-release). The release must exist before the router points at it: `rhythm install` and `rhythm update` read `/releases/latest`.

### 3. Register it in the router

The router only knows the tools listed in [`cmd/rhythm/main.go`](https://github.com/RhythmShahriar/rhythm/blob/master/cmd/rhythm/main.go): the `tools` map (name → repository), its `aliases`, and the tools list in the help text. For Soundcheck and Echo:

```go
var tools = map[string]string{
	"tuner":      "RhythmShahriar/tuner",
	"soundcheck": "RhythmShahriar/soundcheck",
	"echo":       "RhythmShahriar/echo",
}

var aliases = map[string]string{"t": "tuner", "sc": "soundcheck", "e": "echo"}
```

To install it by default, add it to the installers' default `RHYTHM_TOOLS` (`tuner soundcheck echo` in `scripts/install.sh` and `scripts/install.ps1`) and to their "Get started" line. Then add the tool to the wiki: its pages, the sidebar, Home, Install, Updating, Uninstall and this page.

### 4. Release rhythm

Release `rhythm` with the registration. Users get the tool with `rhythm install <tool>`, and new installs get it by default.

### The order

```text
rhythm (kit the tool needs)  →  the tool's first release  →  rhythm (registers the tool)
```

1. **`rhythm` with the kit**, if the tool needs kit packages that aren't released yet. For Soundcheck that was `v0.2.0`, which added `kit/crawl`, `kit/ratelimit`, `kit/cache` and `kit/cli`, and then `v0.2.1` for the crawl fixes it now requires. For Echo it's `v0.4.0`, which added `kit/secrets`.
2. **The tool**, requiring that `rhythm` version. For Echo, the one `v1.0.0` tag releases the binary and the npm package, and `split-laravel.sh v1.0.0` then publishes the Laravel package (see [Echo's packages](#echos-packages)). A tool that others build on releases before them: Soundcheck `v1.1.0`, which imports Echo's `client` and `compare`, comes after Echo `v1.0.0`.
3. **`rhythm` again**, registering the tool and installing it by default. For Echo, `v0.5.0` registers `echo` with the alias `e` and makes the installers default to `tuner soundcheck echo`.

Registering a tool before it has a release breaks the installer: `rhythm install` finds no release, and the installer stops (`installing <tool> failed`) for everyone using the default `RHYTHM_TOOLS`.

## Echo's packages

An Echo release ships three things from one tag of [`RhythmShahriar/echo`](https://github.com/RhythmShahriar/echo): the `rhythm-echo` binary (through the shared release workflow, like every tool), the Next.js and Node connector on npm, and the Laravel connector on Packagist. The WordPress connector is embedded in the binary and needs nothing extra.

### npm: `@rhythmsh/echo`

The Echo repository's [`npm` workflow](https://github.com/RhythmShahriar/echo/blob/master/.github/workflows/npm.yml) runs on the same `v*` tag push. In `connectors/node`, it installs with `npm ci`, sets the package version from the tag (`npm version "${GITHUB_REF_NAME#v}"`), runs `npm test`, and publishes with provenance:

```sh
npm publish --provenance --access public --tag latest   # or --tag next for a v1.1.0-rc1 style tag
```

It needs the **`NPM_TOKEN`** repository secret: an npm granular access token with write access to `@rhythmsh/echo`, used until npm Trusted Publishing is set up for the package. The workflow has `id-token: write` for the provenance statement. A tag with a suffix is published under `next`, so `npm install @rhythmsh/echo` never picks up a release candidate. Check the result on the package's npm page (the *Provenance* section) or with `npm audit signatures`; see [Verifying releases](Verifying-Releases#echos-npm-package).

### Packagist: `rhythmsh/echo-laravel`

Packagist needs a repository whose root is the package, so `connectors/laravel` is split into its own repository, [`RhythmShahriar/echo-laravel`](https://github.com/RhythmShahriar/echo-laravel), with `scripts/split-laravel.sh` in the Echo repository:

```sh
scripts/split-laravel.sh v1.0.0
```

It runs `git subtree split --prefix connectors/laravel`, pushes the result to `echo-laravel`'s `master`, and creates the same tag there, then prints `Pushed connectors/laravel at v1.0.0 to <remote>`. `ECHO_LARAVEL_REMOTE` overrides the remote. Packagist reads `rhythmsh/echo-laravel` from that repository and picks up the new tag through its GitHub hook (or **Update** on the package's Packagist page). Run it from an up-to-date `master` after tagging Echo, since the subtree history is pushed as it is.

<!-- TODO(lead): split-laravel.sh is run by hand; if a workflow takes it over, describe that here. -->

### Before tagging Echo

- `make check` in the Echo repository, plus the connectors' own tests: `make -C connectors/laravel check` (Docker) and `cd connectors/node && npm test`.
- The `version` in `connectors/node/package.json` doesn't matter: the workflow sets it from the tag.
- The mu-plugin's `Version:` header and `VERSION` constant in `connectors/wordpress/echo.php` should match the release.

## Check packs

Tuner's checks also ship without a release. The [`checks-pack`](https://github.com/RhythmShahriar/tuner/blob/master/.github/workflows/checks-pack.yml) workflow runs on every push to `master` that touches `checks/**` (or the workflow itself), and can be run by hand:

1. Runs every check's fixtures: `go test ./engine ./checkpack`.
2. Builds the pack with `make pack` (`checks.tar.gz`, `checks-manifest.json`, `checks-sha256.txt`), byte for byte as you'd get locally from the same commit and Go version.
3. Uploads the three assets to the rolling `checks-latest` release, replacing the previous pack.

Runs never overlap: a newer run waits for the current upload to finish. The first run creates the `checks-latest` tag on a parentless commit that holds only a README, so it's never reachable from `master` and GoReleaser can't mistake it for a version tag.

> [!WARNING]
> `checks-latest` is a **prerelease** and must never be marked as the latest release. `rhythm update`, `rhythm install` and both installers read `/releases/latest`, which only returns the most recent non-prerelease, non-draft release. If `checks-latest` became "latest", installs and updates of Tuner would find no binaries. The workflow re-applies `--prerelease --latest=false` on every run; don't change it by hand in the GitHub UI.

If a checks change only behaves correctly on a newer `rhythm-tuner`, bump `min_tuner` in `checks/pack.json` in the same change. See [Check Packs](Tuner-Check-Packs) for how clients choose and verify packs.

## Verify a release

After the workflow finishes:

1. **Assets.** The release has six archives, `checksums.txt` and `checksums.txt.sigstore.json`:

   ```sh
   gh release view v0.3.2 -R RhythmShahriar/tuner --json assets --jq '.assets[].name'
   ```

2. **Latest.** The new version is marked *Latest*, and `checks-latest` is still a *Pre-release*:

   ```sh
   gh release list -R RhythmShahriar/tuner --limit 5
   ```

   ```text
   v0.3.2       Latest       v0.3.2          …
   Check pack   Pre-release  checks-latest   …
   ```

3. **Signature.** Download `checksums.txt` and its bundle, then verify them as described in [Verifying Releases](Verifying-Releases):

   ```sh
   gh release download v0.3.2 -R RhythmShahriar/tuner -p 'checksums.txt*'
   cosign verify-blob --bundle checksums.txt.sigstore.json \
     --certificate-identity https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     checksums.txt
   ```

4. **Update path.** On a machine with the previous version, `rhythm update --check` should offer the new one, and `rhythm update` should install it:

   ```text
   rhythm-tuner       0.3.1 → 0.3.2 available
   ```

5. **Fresh install.** Run the installer in a clean environment, such as a throwaway container or VM, and check `rhythm tuner --version`.

6. **Notes.** Read the generated release notes, and edit them if a change needs more explanation, such as a breaking change.

---
<sub>← [Contributing](Contributing) · [Home](Home) · [Architecture](Architecture) →</sub>
