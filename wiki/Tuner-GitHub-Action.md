`RhythmShahriar/tuner@v0` runs Tuner in a GitHub Actions job: it installs a verified `rhythm-tuner` release, scans your sites or site folders, writes a job summary, uploads findings to code scanning when you ask and fails the step on serious findings. `rhythm tuner init ci` writes the workflow for you. This page covers both; [CI and code scanning](Tuner-CI) covers the gates, baselines and other CI systems.

```yaml
- uses: RhythmShahriar/tuner@v0
  with:
    target: https://staging.yoursite.com
    fail-on: high
    sarif: true   # needs permissions: security-events: write
```

## Write the workflow: init ci

Run it anywhere inside your repository. It writes `.github/workflows/tuner.yml` at the repository root:

```text
$ rhythm tuner init ci --target https://staging.example.com --sarif
Wrote .github/workflows/tuner.yml: scans https://staging.example.com, fails on high or worse.

Next steps:
  1. Review the file, then commit and push it:
       git add .github/workflows/tuner.yml
       git commit -m "ci: scan with tuner"
  2. Watch the first run in the repository's Actions tab.
  3. Findings also appear under Security > Code scanning.

Scan only sites you own or are authorised to test.
More: https://github.com/RhythmShahriar/rhythm/wiki/Tuner-CI
```

The file it wrote, verbatim:

```yaml
# Scans the site at https://staging.example.com with Tuner.
# The job fails on findings at or above fail-on. Written by
# rhythm tuner init ci; every input: https://github.com/RhythmShahriar/rhythm/wiki/Tuner-CI
name: tuner
on:
  push:
    branches: [main, master]
  pull_request:
  # Weekly too, so newly published vulnerabilities show up without a push.
  schedule:
    - cron: "17 4 * * 1"
  workflow_dispatch:

permissions:
  contents: read
  security-events: write # upload tuner.sarif to code scanning

jobs:
  tuner:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      # @v0 follows every v0.x release. To pin one exactly, use its commit
      # SHA and keep the version in a comment:
      #   uses: RhythmShahriar/tuner@<commit sha> # <release tag>
      - uses: RhythmShahriar/tuner@v0
        with:
          target: "https://staging.example.com"
          fail-on: high
          sarif: true
          # Pull requests from forks can't write code scanning alerts.
          sarif-upload: ${{ github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository }}
```

| Flag | |
|---|---|
| `--target <url\|dir>` | the site's URL, or its folder in this repository. A bare hostname gets `https://`; an expression such as `${{ vars.STAGING_URL }}` is kept as written. Without it, the current folder is used when it looks like a site (`wp-config.php`, `wp-includes`, `wp-content`, `composer.json` or `index.php`) |
| `--fail-on <level>` | fail the job at or above this severity, or `none` (default `high`) |
| `--sarif` | also upload findings to GitHub code scanning |
| `--force` | replace an existing `tuner.yml` |

It refuses to overwrite a workflow without `--force` and keeps credentials out of the file:

```text
$ rhythm tuner init ci --target https://staging.example.com
rhythm tuner: /home/you/site/.github/workflows/tuner.yml already exists; pass --force to replace it
$ rhythm tuner init ci --target https://user:pw@staging.example.com --force
rhythm tuner: --target has a username or password; keep credentials out of the workflow and pass them from a secret with the Action's headers input
```

## Inputs

| Input | Default | What it does |
|---|---|---|
| `target` | required | a URL such as `https://staging.example.com`, or a site folder such as `./public`; several targets go one per line |
| `version` | `latest` | the Tuner version to install, e.g. `0.4.0` |
| `fail-on` | `high` | fail the step on a finding at or above `critical`, `high`, `medium`, `low` or `info`; `none` never fails on findings |
| `min-confidence` | Tuner's default, `likely` | `confirmed` lets only confirmed findings fail the step; `likely` counts both. The older values `high` and `low` still work |
| `baseline` | none | a JSON report from an earlier scan; findings in it are hidden and don't fail the step |
| `sarif` | `false` | `true` writes `tuner.sarif` in the working directory and uploads it to code scanning (category `tuner`) |
| `sarif-upload` | `true` | with `sarif`, upload the file; `false` only writes it, e.g. for pull requests from forks |
| `headers` | none | extra request headers, one `Name: value` per line, each passed whole as `-H` |
| `cookie` | none | a Cookie header, passed as `--cookie` |
| `args` | none | extra scan flags, e.g. `--profile quick --rate 2`. Split on spaces with no quoting, so a value with a space can't go here |
| `working-directory` | `.` | where the scan runs; relative folder targets, the baseline and `tuner.sarif` resolve here |
| `verify-signature` | `true` | check the release's keyless signature with cosign before trusting `checksums.txt`; `false` is only for mirrors with unsigned builds |
| `download-url` | none | fetch the release files from this URL instead of GitHub, for mirrors |

## Outputs

| Output | |
|---|---|
| `score` | the Tune score, the lowest across targets |
| `findings` | how many findings were reported, after the baseline, across targets |
| `report` | the path of the JSON report |
| `sarif-file` | the path of `tuner.sarif` when `sarif` is `true` |
| `exit-code` | `rhythm-tuner`'s exit code: 0 ok, 1 findings at `fail-on`, 2 usage error, 3 a target couldn't be scanned |

```yaml
- uses: RhythmShahriar/tuner@v0
  id: tuner
  with:
    target: https://staging.yoursite.com
    fail-on: none
- run: echo "Tune score ${{ steps.tuner.outputs.score }}"
```

## What the step does

1. **Installs Tuner.** It picks `rhythm-tuner_<os>_<arch>` for the runner (Linux, macOS or Windows; amd64 or arm64), verifies `checksums.txt` with cosign against the release workflow's keyless identity for `RhythmShahriar/tuner` and that version's tag, then checks the archive's SHA-256 against it. A mismatch stops the job before anything runs: `::error::checksum mismatch for <archive>: …`.
2. **Scans** with `rhythm-tuner scan <targets> --plain`, your gate flags and `args`, writing a JSON report. The full plain view is in the step log.
3. **Writes a job summary**: a `### Tuner` table with each target's score and counts per severity.
4. **Writes the SARIF** with `rhythm-tuner report <json> --sarif` (so the site is scanned once) and uploads it with `github/codeql-action/upload-sarif` when `sarif-upload` is on.
5. **Fails the step** when a finding reaches `fail-on`: `::error::Tuner found issues at or above high. See the scan log and the job summary.` The SARIF upload still happens first.

It also caches Tuner's data folder (vulnerability data and checksums) per day, so later jobs that day skip most lookups.

## Secrets: headers and cookie

Staging sites behind a login need a header or a cookie. Pass them from a secret through `headers` or `cookie`, never through `args`, which splits on spaces and shows up in logs as typed:

```yaml
- uses: RhythmShahriar/tuner@v0
  with:
    target: https://staging.yoursite.com
    headers: |
      Authorization: Bearer ${{ secrets.STAGING_TOKEN }}
    cookie: session=${{ secrets.STAGING_SESSION }}
```

Tuner sends `-H` headers and `--cookie` only to the host you named, never to another host a redirect leads to and never over plain http after https. See [Requests and politeness](Tuner-Requests#headers-and-virtual-hosts).

## Folder targets

A folder in the repository is scanned as a [local scan](Tuner-Local-Scans): `target: ./public` runs no HTTP requests at all, so it works in pull requests from forks and on code that isn't deployed yet. Folder targets resolve against `working-directory`.

## Pinning

`@v0` moves to each new v0.x release, so you get fixes and new checks without editing the workflow. To pin exactly, use a release's commit SHA with the version in a comment, as the generated workflow shows. Set `version` to the same release. Dependabot's `github-actions` updates keep a SHA pin current.

## Pull requests from forks

GitHub gives fork pull requests a read-only token, so they can't upload SARIF. The generated workflow sets `sarif-upload` to an expression that skips the upload for them; the scan, the summary and the gate still run. A fork's pull request runs the workflow and any `.tuner.yml` as the fork changed them, without your secrets. Tuner's [config trust rules](Tuner-Config#trust-rules) keep a planted `.tuner.yml` from sending your headers to a host it picks; a folder target sends no requests at all.

---
<sub>← [CI and code scanning](Tuner-CI) · [Home](Home) · [Config file](Tuner-Config) →</sub>
