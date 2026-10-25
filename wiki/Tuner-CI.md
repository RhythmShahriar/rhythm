Run Tuner on a schedule or on every deploy, show its findings in GitHub's **Security** tab and fail a pipeline when something serious appears, or only when something new does. This page covers the gates, baselines and caching, with GitHub Actions examples; the same commands work in any CI.

> [!TIP]
> On GitHub, the quickest start is `rhythm tuner init ci --target https://staging.yoursite.com --sarif`, which writes a workflow using the [Tuner GitHub Action](Tuner-GitHub-Action). The Action installs a verified release, scans, writes a job summary and uploads SARIF. The rest of this page applies to it too.

## Weekly scan with GitHub code scanning

This workflow installs Tuner with the installer and uploads SARIF itself, so each step is visible. Tuner writes [SARIF](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning), which GitHub turns into code scanning alerts: each finding appears in the repository's **Security → Code scanning** list, with its severity, the fix and history across runs.

```yaml
# .github/workflows/tuner.yml
name: tuner
on:
  schedule: [{cron: "0 6 * * 1"}]   # Mondays 06:00 UTC
  workflow_dispatch:

permissions:
  contents: read
  security-events: write   # upload SARIF

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - name: Install Tuner
        run: |
          curl -fsSL https://rythm.sh/install | sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"

      - name: Scan
        run: rhythm tuner scan https://staging.yoursite.com --sarif -o tuner.sarif

      - uses: github/codeql-action/upload-sarif@v3
        if: always()   # upload what was found even if the scan step failed
        with:
          sarif_file: tuner.sarif
```

> [!TIP]
> On GitHub Actions, `uses: RhythmShahriar/rhythm/setup@v0.7.0` with `tools: tuner` installs rhythm and Tuner with signatures checked, caches the downloads and puts them on `PATH`; it can also install the versions a `rhythm.lock` pins. See [CI setup](CI-Setup).

> [!NOTE]
> Code scanning is free for public repositories. For private ones it needs GitHub Advanced Security; without it, keep the report as an artifact instead (see [Keeping reports](#keeping-reports)).

## Failing a build

`--fail-on <severity>` makes the scan exit with code **1** when any finding is at or above that severity:

```sh
rhythm tuner scan https://staging.yoursite.com --fail-on high
```

| Exit code | Meaning |
|---|---|
| 0 | done; no finding at or above `--fail-on` (or `--fail-on` not set) |
| 1 | a finding at or above `--fail-on` |
| 2 | usage error (a bad flag, target or config file) |
| 3 | a site couldn't be scanned, or the scan can't be trusted to pass (below) |
| 130 | interrupted |

When several sites are scanned, findings are checked first: if any report meets `--fail-on`, the exit code is 1 even if another site was unreachable.

**A scan that can't be trusted doesn't pass.** With `--fail-on` and nothing at the level, the scan still exits 3 when the vulnerability data couldn't be fetched, or the scan was blocked by a firewall, stopped by `--max-time`, ran out of request budget or couldn't load its `--integrity` checksums. A green build then means a real pass, not a scan that saw nothing:

```text
rhythm tuner scan: http://127.0.0.1:20400: no vulnerability data, so --fail-on critical can't pass it
  rerun when the data source is back, or add --allow-missing-vulns
rhythm tuner scan: http://127.0.0.1:20400: the scan is incomplete (stopped by --max-time), so --fail-on critical can't pass it
  add --allow-partial to accept a partial scan
```

`--allow-missing-vulns` and `--allow-partial` accept those cases when you'd rather not block on them.

**Likely findings.** `--fail-on` counts every finding, including ones marked *likely* (low confidence, e.g. a PHP version a distribution may have patched). `--min-confidence confirmed` keeps them in the report but stops them from failing the build:

```sh
rhythm tuner scan https://staging.yoursite.com --fail-on high --min-confidence confirmed
```

Accept the ones you've verified with an [ignore entry or a baseline](#accepting-known-findings), or leave a whole check out with `--exclude-id`.

## Failing only on new findings

`--since` compares the scan with an earlier JSON report and `--fail-on-new` fails the build only when something new reaches the level. Accepted problems stay visible in the report while the gate watches for regressions:

```sh
rhythm tuner scan https://staging.yoursite.com --since last.json --fail-on-new high -o this.json
```

Keep the last report between runs, for example in the Actions cache or as an artifact you download in the next run. Pass it as `--since`. `--fail-on-new` follows the same rules as `--fail-on` (`--min-confidence`, missing data, partial scans). See [Changes and notifications](Tuner-Changes-And-Notifications), which also covers `--notify` for sending new findings to Slack, Discord, a webhook or email.

A typical deploy gate scans staging after the deploy step and blocks promotion on high or critical findings:

```yaml
      - name: Security gate
        run: rhythm tuner scan https://staging.yoursite.com --fail-on high --md -o tuner.md
```

## Accepting known findings

Some findings you'll accept, at least for now: a header you can't change on shared hosting, a *likely* finding you've confirmed is patched. Tuner has two ways to say so; both hide the findings from the report, the score and the gates.

**Ignore entries in `.tuner.yml`** name a check or path, a reason and an expiry date. They're short and reviewable. They turn back into findings on their own:

```yaml
# .tuner.yml
ignore:
  - id: hdr-csp-missing
    reason: the theme needs inline scripts; tracked in issue 42
    until: 2026-12-31
```

See [Config file](Tuner-Config) for the keys and the trust rules for files a pull request can change.

**A baseline** accepts everything in a saved JSON report at once:

```sh
rhythm tuner scan https://staging.yoursite.com --json -o tuner-baseline.json
rhythm tuner scan https://staging.yoursite.com --baseline tuner-baseline.json --fail-on medium
```

The summary says what was hidden:

```text
Score 100/100 after baseline (15 hidden) - no findings - 77 checks, 180 requests, 3.6s
```

How baselines match:

- **By fingerprint**: the check ID and the path, without scheme or host. A baseline saved on staging works on production; one saved from `./wp` still works after the folder moves.
- **One advisory at a time.** A baselined plugin that gets a *new* vulnerability comes back as a finding listing only the new advisories, at their severity, so the gate fails again.
- **Per site in a batch.** A baseline saved from several sites (an array) matches each finding only on the site it was accepted on, even if some sites failed when it was saved. Only a single-site report matches on any host.
- **Only Tuner JSON.** A SARIF file or any other JSON is refused (`--baseline out.sarif: not a Tuner JSON report: no rhythm-tuner schema`).

Commit the baseline or the `.tuner.yml` to the repository so accepting a finding is a reviewed change.

## Keeping reports

Every format can be written to a file and kept as a build artifact:

```yaml
      - run: rhythm tuner scan https://staging.yoursite.com -o tuner.md --fail-on high
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: tuner-report
          path: tuner.md
```

`-o` picks the format from the extension (`.json`, `.jsonl`, `.sarif`, `.md`, `.html`) and still prints the plain view to the log. `--html report.html` adds a self-contained HTML report alongside any of them. See [Output and reports](Tuner-Output).

## Speed and reproducibility

**Cache the vulnerability data** between runs. Tuner refreshes it once it's a day old either way; the cache just saves the lookups. On a cold runner that must not reach the data services during the scan, fill it first with `rhythm tuner update --prefetch <site>`, then scan with `--offline`:

```yaml
      - uses: actions/cache@v4
        with:
          path: ~/.cache/rhythm
          key: rhythm-cache-${{ github.run_id }}
          restore-keys: rhythm-cache-
```

**Pin the version** when a pipeline must not change behavior on its own. The installer always fetches the newest Tuner (`RHYTHM_VERSION` pins `rhythm` only), so for a pinned Tuner download the release directly and check it against the release checksums:

```yaml
      - name: Install Tuner 0.4.0 (pinned)
        run: |
          v=0.4.0
          base=https://github.com/RhythmShahriar/tuner/releases/download/v$v
          curl -fsSLO "$base/rhythm-tuner_linux_amd64.tar.gz"
          curl -fsSLO "$base/checksums.txt"
          sha256sum --ignore-missing -c checksums.txt
          tar -xzf rhythm-tuner_linux_amd64.tar.gz rhythm-tuner
          sudo install rhythm-tuner /usr/local/bin/
```

`rhythm-tuner` is the tool itself; `rhythm tuner …` only forwards to it, so `rhythm-tuner scan …` works without installing `rhythm`. The [GitHub Action](Tuner-GitHub-Action) takes `version: 0.4.0` and checks the release signature as well. Tuner 0.4.0 uses only its built-in checks; once a release can use [check packs](Tuner-Check-Packs), add `--no-check-updates` to freeze those too.

In CI, Tuner never prints the "new release" notice: it's off whenever the `CI` environment variable is set, which GitHub Actions and most CI systems do.

## Other CI systems

Nothing above is GitHub-specific except the SARIF upload and the Action. In GitLab CI, Jenkins or a cron job, install with the same one-liner, run the scan with `--fail-on` or `--fail-on-new` and keep `-o report.json` (and `--html report.html`) as an artifact. A container image, `ghcr.io/rhythmshahriar/tuner`, runs `rhythm-tuner` too; see [Watch and serve](Tuner-Watch-And-Serve#in-a-container). For many sites at once, see [Batch scans](Tuner-Batch-Scans).

> [!IMPORTANT]
> Point CI at sites you own or are authorized to test, ideally staging. Scheduled scans of production are fine at Tuner's default rate (5 requests a second); see [Requests and politeness](Tuner-Requests). A scan reads `.tuner.yml` from the current folder, so in a repository that takes pull requests from others, read [the trust rules](Tuner-Config#trust-rules) or pass `--no-config`.

---
<sub>← [PHP advisories](Tuner-PHP-Advisories) · [Home](Home) · [GitHub Action](Tuner-GitHub-Action) →</sub>
