How `rhythm` and its tools are released, for maintainers. A release is a pushed `vX.Y.Z` tag; a shared workflow builds, checksums, signs and publishes it. Tuner's checks also publish on their own, as a check pack, on every push to `master`.

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

Tools import the shared kit (`github.com/RhythmShahriar/rhythm/kit/...`) as a normal Go module, pinned in their `go.mod` (Tuner currently requires `github.com/RhythmShahriar/rhythm v0.1.0`). A tool release builds against that pinned version, not against `rhythm`'s `master`. So when a tool needs a kit change, release in this order:

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

### Adding a new tool

The router only knows the tools listed in `rhythm/cmd/rhythm/main.go` (the `tools` map and its `aliases`). For `rhythm install <tool>` and `rhythm update` to find a new tool, add it there and release `rhythm`, after the tool has at least one published release.

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
   rhythm-tuner   0.3.1 → 0.3.2 available
   ```

5. **Fresh install.** Run the installer in a clean environment, such as a throwaway container or VM, and check `rhythm tuner --version`.

6. **Notes.** Read the generated release notes, and edit them if a change needs more explanation, such as a breaking change.

---
<sub>← [Contributing](Contributing) · [Home](Home) · [Architecture](Architecture) →</sub>
