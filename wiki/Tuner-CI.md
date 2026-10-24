Run Tuner on a schedule or on every deploy, show its findings in GitHub's **Security** tab and fail a pipeline when something serious appears. This page covers GitHub Actions in detail; the same commands work in any CI.

## Weekly scan with GitHub code scanning

Tuner writes [SARIF](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning), which GitHub turns into code scanning alerts: each finding appears in the repository's **Security → Code scanning** list, with its severity, the fix and history across runs.

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
| 2 | usage error (a bad flag or target) |
| 3 | a site couldn't be scanned, e.g. it was unreachable |
| 130 | interrupted |

When several sites are scanned, findings are checked first: if any report meets `--fail-on`, the exit code is 1 even if another site was unreachable.

> [!TIP]
> `--fail-on` counts every finding, including ones marked *likely* (low confidence, e.g. a server version a distribution may have patched). Accept the ones you've verified with a [baseline](#accepting-known-findings), or leave a whole check out with `--exclude-id`.

A typical deploy gate scans staging after the deploy step and blocks promotion on high or critical findings:

```yaml
      - name: Security gate
        run: rhythm tuner scan https://staging.yoursite.com --fail-on high --md -o tuner.md
```

## Accepting known findings

Some findings you'll accept, at least for now: a header you can't change on shared hosting, a *likely* finding you've confirmed is patched. Save a report once and pass it as a **baseline**; only findings that aren't in it are reported and counted for `--fail-on`.

```sh
rhythm tuner scan https://staging.yoursite.com -o tuner-baseline.json
rhythm tuner scan https://staging.yoursite.com --baseline tuner-baseline.json --fail-on medium
```

Findings are matched by **check ID and URL**. Commit the baseline to the repository so accepting a finding is a reviewed change. The summary line shows how many were hidden ("3 baselined"). A finding that changes (for example a plugin gets a *new* vulnerability) keeps the same ID and URL, so review the baseline when you update it.

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

`-o` picks the format from the extension (`.json`, `.jsonl`, `.sarif`, `.md`) and still prints the live view to the log. See [Output and reports](Tuner-Output).

## Speed and reproducibility

**Cache the vulnerability data** between runs. Tuner refreshes it once it's a day old either way; the cache just saves the lookups:

```yaml
      - uses: actions/cache@v4
        with:
          path: ~/.cache/rhythm
          key: rhythm-cache-${{ github.run_id }}
          restore-keys: rhythm-cache-
```

**Pin the version** when a pipeline must not change behavior on its own. The installer always fetches the newest Tuner (`RHYTHM_VERSION` pins `rhythm` only), so for a pinned Tuner download the release directly and check it against the release checksums:

```yaml
      - name: Install Tuner 0.3.2 (pinned)
        run: |
          v=0.3.2
          base=https://github.com/RhythmShahriar/tuner/releases/download/v$v
          curl -fsSLO "$base/rhythm-tuner_linux_amd64.tar.gz"
          curl -fsSLO "$base/checksums.txt"
          sha256sum --ignore-missing -c checksums.txt
          tar -xzf rhythm-tuner_linux_amd64.tar.gz rhythm-tuner
          sudo install rhythm-tuner /usr/local/bin/
```

`rhythm-tuner` is the tool itself; `rhythm tuner …` only forwards to it, so `rhythm-tuner scan …` works without installing `rhythm`. New checks still arrive through [check packs](Tuner-Check-Packs); add `--no-check-updates` to freeze those too.

In CI, Tuner never prints the "new release" notice: it's off whenever the `CI` environment variable is set, which GitHub Actions and most CI systems do.

## Other CI systems

Nothing above is GitHub-specific except the SARIF upload. In GitLab CI, Jenkins or a cron job, install with the same one-liner, run the scan with `--fail-on` and keep `-o report.md` or `-o report.json` as an artifact. For many sites at once, see [Batch scans](Tuner-Batch-Scans).

> [!IMPORTANT]
> Point CI at sites you own or are authorized to test, ideally staging. Scheduled scans of production are fine at Tuner's default rate (5 requests a second); see [Requests and politeness](Tuner-Requests).

---
<sub>← [PHP advisories](Tuner-PHP-Advisories) · [Home](Home) · [Writing checks](Tuner-Writing-Checks) →</sub>
