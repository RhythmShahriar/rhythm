Review every update before it's merged. On a pull request that moves a lockfile, Sightread reviews every version the change adds or moves, writes the result to the job summary and one pull request comment, keeps the reports as an artifact and fails the job at the level you set. This page covers `init ci`, the GitHub Action and its inputs, Dependabot and Renovate, code scanning and running `changes` in any other CI.

## init ci

```sh
rhythm sightread init ci                              # fail on high, keep one PR comment
rhythm sightread init ci --fail-on critical --sarif   # fail on critical, upload to code scanning
rhythm sightread init ci --comment=false              # no PR comment
```

Run it at the repository root:

```text
$ rhythm sightread init ci
Wrote .github/workflows/sightread.yml: reviews lockfile changes on pull requests and everything each Monday, fails on high or worse.

Next steps:
  1. Review the file, then commit and push it:
       git add .github/workflows/sightread.yml
       git commit -m "ci: review updates with sightread"
  2. Protect .sightread/allowed.yaml with CODEOWNERS, so a pull request can't approve its own update:
       /.sightread/allowed.yaml @your-org/reviewers
  3. Dependabot and Renovate pull requests get a read-only token: the gate and the job summary still
     work while the comment and the SARIF upload are skipped with a notice.
More: https://github.com/RhythmShahriar/rhythm/wiki/Sightread-CI
```

The workflow it wrote:

```yaml
name: Sightread
on:
  pull_request:
    paths: ["**/composer.lock", "**/package-lock.json", "**/npm-shrinkwrap.json", "**/pnpm-lock.yaml", "**/yarn.lock", "**/bun.lock"]
  schedule:
    - cron: "17 3 * * 1"
permissions:
  contents: read
  pull-requests: write
jobs:
  sightread:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with: {fetch-depth: 0}
      - uses: RhythmShahriar/sightread@4e49be6985e64cb0e668ea99a7192f612d424626 # v1.0.0
        with:
          version: 1.0.0
          fail-on: high
          comment: true
```

Both actions are pinned by commit. A commit doesn't name a release, so `init ci` also sets `version`; without it the Action installs the latest release and warns. `fetch-depth: 0` gives `changes` the base branch to compare with. On pull requests the job reviews the lockfile changes; each Monday it reviews every available update in the repository. `init ci` never replaces an existing workflow without `--force`: it exits `2` and says to add it.

A build from source (`rhythm-sightread version` says `dev`) has no release commit to pin. Its `init ci` prints a warning first, writes `uses: RhythmShahriar/sightread@v1 # pin this to a commit SHA and set version below` and leaves the input as a comment under `with:`, `# version: 1.2.3`. Pin the commit of the release you want and set `version` to it, or write the workflow with a released binary.

## The GitHub Action

```yaml
- uses: RhythmShahriar/sightread@v1
  with:
    fail-on: high
    comment: true
```

The Action installs the Sightread release that matches its tag, checks it with cosign before anything runs (it never builds from source), restores a per-day cache of data and archives, runs the review and then:

- writes the Markdown review to the job summary;
- uploads the JSON and HTML reports as a workflow artifact;
- with `comment: true`, posts one pull request comment and updates that same comment on later runs;
- with `sarif: true`, uploads findings to code scanning under the category `sightread`;
- fails the job when the review exits non-zero: `Sightread found updates with findings at or above high. See the job summary and the report artifact.`

It runs on Linux and macOS runners, not Windows. On `pull_request` it runs `changes` against the pull request's base branch; on any other event it runs `all`.

### Inputs

| Input | Default | Meaning |
|---|---|---|
| `mode` | `changes` on `pull_request`, else `all` | `changes`, `all` or `package` |
| `package` | | for mode `package`, one `eco:name@version` per line, e.g. `npm:left-pad@1.4.0` |
| `path` | `.` | the site or project folder, relative to `working-directory` |
| `working-directory` | `.` | folder every step runs in, e.g. `apps/web` in a monorepo |
| `base` | the pull request's base branch | for mode `changes`, the branch to compare with; allow entries count only from it |
| `fail-on` | `high` | fail the job at `none`, `info`, `low`, `medium`, `high` or `critical` |
| `min-confidence` | the config's, else `likely` | `confirmed` or `likely` |
| `allow-partial` | `false` | let `fail-on` pass a run with packages not checked, fetch failures or timeouts |
| `allow-unverified` | `false` | review packages whose integrity couldn't be checked and let `fail-on` pass them |
| `baseline` | | a saved JSON report whose findings are hidden |
| `trust-pr-allow` | `false` | count the allow entries, config and baseline the pull request itself edits |
| `transitive` | `false` | review transitive Composer and npm dependencies too; on pull requests `changes` always does |
| `sarif` | `false` | write `sightread.sarif` and upload it to code scanning; needs `security-events: write` |
| `comment` | `false` | one pull request comment, updated on later runs; needs `pull-requests: write` |
| `summary` | `true` | the Markdown review in the job summary |
| `upload` | `true` | the JSON and HTML reports as a workflow artifact |
| `attest` | `false` | sign the JSON report with `actions/attest` under the workflow's identity; needs `id-token: write` and `attestations: write` |
| `args` | | extra flags, e.g. `--only npm:* --jobs 2`, split on spaces with no glob expansion |
| `version` | the release that matches the action's tag | e.g. `1.0.0` or `latest`; at a commit or `@v1` it installs the latest release and warns |
| `verify-signature` | `true` | check with cosign that `checksums.txt` was signed by Sightread's release workflow; `false` checks only the checksum and warns |
| `github-token` | `${{ github.token }}` | for the comment, the SARIF upload and the GitHub API rate of Composer packages and tag checks |
| `download-url` | | a folder holding the release files instead of the GitHub release, for mirrors; needs `version` |

### Outputs

| Output | Meaning |
|---|---|
| `exit-code` | Sightread's exit code |
| `verdict` | the worst verdict of the run: `hold`, `review`, `unverified`, `allowed` or `routine`, or `none` |
| `updates` | how many updates were reviewed |
| `findings` | how many findings count, at every severity |
| `report` | path of the JSON report |
| `html` | path of the HTML report |
| `sarif-file` | path of the SARIF file, when `sarif` is true |
| `summary` | path of the Markdown review |
| `comment-id` | ID of the pull request comment, when one was posted or updated |
| `version` | the Sightread version that ran |

```yaml
- uses: RhythmShahriar/sightread@v1
  id: sightread
  with: {fail-on: none}
- if: steps.sightread.outputs.verdict == 'hold'
  run: echo "A dependency update is on HOLD; read ${{ steps.sightread.outputs.html }}"
```

## What a pull request shows

`changes` compares each lockfile at the base with the working tree. In a repository where a Renovate branch moved `fetchkit` from 1.2.0 to 1.2.1 and its lockfile entry to a tarball off the registry:

```text
$ rhythm sightread changes --base main --fail-on high
reviewing 1 of 1: fetchkit 1.2.0 → 1.2.1

  ♪ rhythm sightread 1.0.0  npm fetchkit  1.2.0 → 1.2.1
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  ▸ Installed     1.2.0 in node_modules/fetchkit
  ▸ Candidate     1.2.1 · released 2026-10-28
  ▸ Integrity     both versions: SHA-512 matches dist.integrity and the
                  registry signature verifies with a pinned npm key (this
                  proves the registry published it, not that the content
                  is harmless) ✓
  ▸ Publisher     unchanged (goodmaintainer)
  ▸ Diff          2 files changed · +2 −1 lines · 56 B → 209 B
  ▸ Changelog     none for this version

  HIGH  1 ────────────────────────────────────────────────────────────────

 ▌  HIGH       Lockfile entry moved off the registry   package-lock.json:8
 ▌             the lock entry moved from the registry to a tarball URL:
 ▌             https://evil-mirror.example/fetchkit-1.2.1.tgz
 ▌             → Find out who moved this entry off the registry before
 ▌               merging.
...
 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                     REVIEW  │
 │  ■ 1 high   ■ 2 medium                                               │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

It exits `1`. The job summary and the comment get the Markdown form (`--md`), shown here as GitHub renders it from this run:

> ## Sightread: 1 to review
>
> | Verdict | Update | From | To | Findings |
> |---|---|---|---|---|
> | ⚠️ REVIEW | fetchkit (npm) | 1.2.0 | 1.2.1 | 1 high, 2 medium |
>
> ### ⚠️ fetchkit 1.2.0 → 1.2.1
>
> Integrity (both versions): SHA-512 matches dist.integrity and the registry signature verifies with a pinned npm key \(this proves the registry published it, not that the content is harmless\). Publisher: goodmaintainer.
> Changelog: none for this version.
>
> | Severity | Signal | Where | Evidence |
> |---|---|---|---|
> | high | Lockfile entry moved off the registry | `package-lock.json:8` | `the lock entry moved from the registry to a tarball URL: hxxps://evil-mirror[.]example/fetchkit-1.2.1.tgz` |
> | medium | Released very recently | n/a | `published 3 hours ago (2026-10-28 11:11 UTC); the minimum age is 72h` |
> | medium | Changelog does not mention what the code adds | `index.js:2` | `no changelog entry for 1.2.1; the update adds network sends (index.js:2), file writes outside the package (index.js:2)` |
>
> Record your review: `rhythm sightread allow npm:fetchkit@1.2.1 --reason "..."`
>
> Static diff of official releases; Sightread never ran this code. Not proof an update is safe.

The comment also holds the flagged hunks in a collapsed diff. Fetched URLs and hosts are defanged and package text is escaped, so a package can't ping anyone, link anywhere or inject HTML into the comment. Rows of routine updates fold into one `<details>` above 10. The comment stays under GitHub's size limit by trimming hunks first.

## Allow entries in pull requests

A pull request can't approve its own update. `changes` reads `.sightread/allowed.yaml`, the config and a committed baseline at the base branch. An entry the pull request adds shows as `allowed in this change, not counted (--trust-pr-allow counts it)`. Anything else it relaxes (an ignore entry, `osv: false`, a new mirror, a lower `fail_on`) gets the note `relaxed in this change, not counted (--trust-pr-allow counts it)`. To accept an update:

1. Read the comment or the HTML report from the artifact.
2. In a separate pull request (or on the base branch), record the review: `rhythm sightread allow npm:fetchkit@1.2.1 --reason "..."`. It's bound to that archive's sha256.
3. Merge it; the next run of the dependency pull request shows the update as ALLOWED.

Protect the allow file with CODEOWNERS so only reviewers can merge it. `trust-pr-allow: true` turns the rule off, for repositories where every pull request is already reviewed by the same people.

## Dependabot and Renovate

Their pull requests get a read-only token. The gate and the job summary still work; the comment and the SARIF upload are skipped with a notice that says which permission was missing. To run the job only on their pull requests:

```yaml
jobs:
  sightread:
    if: github.actor == 'dependabot[bot]' || github.actor == 'renovate[bot]'
```

Don't use `pull_request_target` with a checkout of the pull request's head: that hands a write token to code from the pull request.

## Code scanning

```yaml
permissions:
  contents: read
  security-events: write
steps:
  - uses: actions/checkout@<sha> # v7.0.1
    with: {fetch-depth: 0}
  - uses: RhythmShahriar/sightread@v1
    with:
      sarif: true
```

Each finding becomes a result located where the version is pinned: the lockfile line, or the plugin's main file when the WordPress site is in the repository, since code scanning ignores files that aren't committed. The path inside the package is in the message and in `properties.package_path`. Fingerprints are stable across runs, so an alert closes when the update is allowed or gone. See [Output and reports](Sightread-Output#sarif).

## Other CI

`changes` is a plain command; any CI that has the repository's history can run it. Install with `rhythm install sightread` (or the installer, see [CI setup](CI-Setup)) and run:

```sh
rhythm sightread changes --base origin/main --fail-on high --md -o sightread.md -o sightread.json
```

`CI` set turns on `--redact-paths`, so reports hold paths relative to the repository. Set `GITHUB_TOKEN` when the project has Composer packages on GitHub; api.github.com allows 60 requests an hour without one. Exit codes are in the [CLI reference](Sightread-CLI-Reference#exit-codes): `1` for findings at `--fail-on`, `3` for packages that couldn't be fetched or verified unless `--allow-partial` or `--allow-unverified`.

For a whole site on a schedule, outside a repository, see [Watch](Sightread-Watch).

---
<sub>← [Watch](Sightread-Watch) · [Home](Home) · [Configuration](Sightread-Configuration) →</sub>
