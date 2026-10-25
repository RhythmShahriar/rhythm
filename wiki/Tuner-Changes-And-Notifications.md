A scan you repeat answers one question: what changed? Tuner compares a scan with an earlier JSON report, lists what is new and what was fixed, fails CI only on new findings, tells Slack, Discord, a webhook or an inbox about them and renders a client-ready HTML report that marks them. This page covers `diff`, `--since`, `--fail-on-new`, `--notify`, `--html` and the `report` command.

```sh
rhythm tuner scan yoursite.com --json -o last.json                     # once: the report to compare with
rhythm tuner scan yoursite.com --since last.json --fail-on-new high    # later: what changed, gate on new
rhythm tuner diff before.json after.json                               # compare two saved reports
```

The examples on this page were run against a test site on `127.0.0.1:20400` twice: before, it exposed a `.env` file; after, the `.env` was gone and a `debug.log` had appeared.

## How findings are matched

Every finding in a JSON report has a `fingerprint`: a hash of the check ID and the location (the URL path for a remote scan, the path inside the folder for a local one). Scheme, host, port and the folder you scanned from are left out, so a scan of staging compares with one of production and a moved checkout with its old copy.

| A finding is… | When |
|---|---|
| **new** | its fingerprint isn't in the earlier report, its severity changed, or it has an advisory the earlier report didn't list |
| **fixed** | the earlier report had it and the new scan looked for it and didn't find it |
| **not seen** | the earlier report had it while the new scan couldn't have seen it: the scan was cut short or blocked, the vulnerability data was missing, or the check didn't run this time (`--id`, `--tags`, `--severity`, `--profile quick`) |
| **unchanged** | in both |

With one report on each side they're compared whatever their targets. With several (a batch), reports pair by host and path, ignoring the scheme and a trailing slash; a site missing from the new scan is listed as `Not in the new scan, so not compared: <site>`, never as fixed.

## Compare two reports: diff

```text
$ rhythm tuner diff rep/before.json rep/after.json
1 new, 1 fixed, 14 unchanged since rep/before.json

New:
[HIGH] WordPress debug.log exposed - http://127.0.0.1:20400/wp-content/debug.log

Fixed:
[CRITICAL] .env file exposed: 1 secret (DB_PASSWORD=Su•••••) - http://127.0.0.1:20400/.env
```

The old report goes first. A finding with new advisories reads like `[CRITICAL] gallery 1.5: 2 known vulnerabilities (1 new: CVE-2026-2)`, one whose severity changed ends ` (was medium)` and a low-confidence one prints `[HIGH, likely]`.

| Flag | |
|---|---|
| `--md` | the comparison as Markdown, for an issue or pull request |
| `--json` | the comparison as JSON (`schema: rhythm-tuner-diff/v0`): `counts`, `new`, `fixed`, `unchanged`, `not_seen` (each with a `why` of `incomplete` or `not_checked`) and `not_scanned` |
| `--fail-on-new <level>` | exit 1 if a new finding is at or above this severity |

```text
$ rhythm tuner diff rep/before.json rep/after.json --md
# Tuner changes since rep/before.json

**1 new** · **1 fixed** · 14 unchanged

## New

| Severity | Finding | Path | Target |
|---|---|---|---|
| HIGH | WordPress debug.log exposed | `/wp-content/debug.log` | http://127.0.0.1:20400 |

## Fixed

| Severity | Finding | Path | Target |
|---|---|---|---|
| CRITICAL | .env file exposed: 1 secret \(DB\_PASSWORD=Su•••••\) | `/.env` | http://127.0.0.1:20400 |
```

Exit codes: `0` ok, `1` new findings at `--fail-on-new`, `2` usage error, `3` a report couldn't be read. A file that isn't a Tuner JSON report (a SARIF file, say) is refused with `not a Tuner JSON report`.

## Compare while you scan: --since

`--since <file>` compares the scan with an earlier JSON report. The report itself is the usual one; after it, stderr gets the comparison:

```text
$ rhythm tuner scan 127.0.0.1:20400 --plain --since rep/before.json --fail-on-new high
…
Score 0/100 - 3 critical, 3 high, 1 medium, 7 low, 1 info - 77 checks, 180 requests, 3.6s
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
run rhythm tuner explain <id> for the full fix
1 new, 1 fixed since rep/before.json
  new: [HIGH] WordPress debug.log exposed - http://127.0.0.1:20400/wp-content/debug.log
  fixed: [CRITICAL] .env file exposed: 1 secret (DB_PASSWORD=Su•••••) - http://127.0.0.1:20400/.env
```

That run exited 1: the debug log is new and high. Under `-q` only the `1 new, 1 fixed` line (and any incomplete-scan notes) is printed. The `--since` file is read before the scan starts, so a wrong path fails at once:

```text
$ rhythm tuner scan 127.0.0.1:20400 --since nope.json
rhythm tuner scan: --since nope.json: no such file
```

### Failing CI on new findings only: --fail-on-new

`--fail-on-new <level>` needs `--since` and exits 1 when a new finding is at or above the level. It follows the same rules as `--fail-on`:

- With `--min-confidence confirmed`, only confirmed new findings count.
- A scan with no new finding at the level still exits 3 when its vulnerability data was missing or the scan was blocked, cut short or missing `--integrity` checksums, unless `--allow-missing-vulns` or `--allow-partial` accepts that.
- It works alongside `--fail-on`; the higher exit code wins.

```text
$ rhythm tuner scan 127.0.0.1:20400 --fail-on-new high
rhythm tuner scan: --fail-on-new needs --since <file>, the report to compare with
```

This is the gentler CI gate: accepted problems stay visible in the report while the build only breaks on something that wasn't there last time. Keep the last good JSON report as a CI artifact or cache entry and pass it as `--since` on the next run. See [CI and code scanning](Tuner-CI).

## Tell someone: --notify

`--notify <url>` sends the new findings to a chat channel, a webhook or an inbox once the scan is done. It needs `--since`, so it sends only what is new:

```text
$ rhythm tuner scan 127.0.0.1:20400 --plain --since rep/before.json --notify webhook+http://127.0.0.1:20410/tuner
…
1 new, 1 fixed since rep/before.json
  new: [HIGH] WordPress debug.log exposed - http://127.0.0.1:20400/wp-content/debug.log
  fixed: [CRITICAL] .env file exposed: 1 secret (DB_PASSWORD=Su•••••) - http://127.0.0.1:20400/.env
notified webhook+http://127.0.0.1:20410: 1 finding
```

When nothing qualifies, nothing is sent:

```text
0 new, 0 fixed since rep/after.json
nothing new at high or above, so no notification
```

### Destinations

| URL | Sends |
|---|---|
| `slack://hooks.slack.com/services/T…/B…/…` | a Slack incoming webhook message over https; `<!channel>`-style mentions are escaped and link previews are off |
| `discord://discord.com/api/webhooks/<id>/<token>` | a Discord webhook message over https; nobody is pinged, previews are off, cut at 2,000 characters |
| `webhook+https://host/path` | the message as JSON (below). `webhook+http://` works only for a receiver on this machine |
| `smtp://user@host:587?to=a@example.com&from=tuner@example.com` | a plain-text email. STARTTLS is required unless the host is loopback; `smtps://` (port 465) uses TLS from the start. The password comes from `TUNER_SMTP_PASSWORD`, never the URL. `to=` may repeat or hold a comma-separated list |

`--notify` can be repeated to send to several places. Webhook requests never follow redirects and time out after 30 seconds. Notification URLs hold secrets, so Tuner only ever prints their scheme and host.

| Flag | |
|---|---|
| `--notify-min <level>` | lowest severity to send (default `high`) |
| `--notify-all-first` | when the `--since` file doesn't exist yet, send every finding at or above `--notify-min` instead of refusing. `--fail-on-new` isn't applied on that first run, since nothing can be new yet |
| `--notify-strict` | exit 3 when a notification can't be sent (otherwise it's a warning and the exit code is the scan's) |

```text
rhythm tuner scan: warning: could not notify webhook+http://127.0.0.1:20499: dial tcp 127.0.0.1:20499: connect: connection refused
```

### What is sent

The findings that are new at or above `--notify-min`, one line each, at most 25 in a chat message or email (webhooks get them all), plus any incomplete-scan notes. No evidence, request or response body, user info in URLs is removed, every string goes through the secret masker and only the base name of the `--since` file is sent. The webhook body from the run above, verbatim:

```json
{
  "schema": "rhythm-tuner-notify/v0",
  "tool": "rhythm-tuner",
  "version": "0.4.0",
  "since": "before.json",
  "min_severity": "high",
  "summary": "Tuner: 1 new finding (1 high) on 127.0.0.1:20400",
  "counts": {
    "critical": 0,
    "high": 1,
    "info": 0,
    "low": 0,
    "medium": 0
  },
  "targets": [
    "http://127.0.0.1:20400"
  ],
  "findings": [
    {
      "target": "http://127.0.0.1:20400",
      "fingerprint": "v1:f9bf949661562a01e6f49b55159f6eea",
      "id": "wp-debug-log",
      "title": "WordPress debug.log exposed",
      "severity": "high",
      "confidence": "high",
      "location": "http://127.0.0.1:20400/wp-content/debug.log",
      "fix": "Turn off WP_DEBUG_LOG on production or point it outside the web root:\ndefine( 'WP_DEBUG_LOG', '/var/log/wordpress/debug.log' );\nThen delete wp-content/debug.log."
    }
  ]
}
```

It's sent with `Content-Type: application/json` and Tuner's User-Agent. The schema is published as [`schema/notify-v0.json`](https://github.com/RhythmShahriar/tuner/blob/master/schema/notify-v0.json); new optional fields can appear in any release, so let your receiver ignore fields it doesn't know.

Usage errors are caught before the scan starts, for example:

```text
rhythm tuner scan: --notify needs --since <file>, so it sends only what is new (add --notify-all-first to send every finding when there is no earlier report)
rhythm tuner scan: --notify: unsupported notification scheme "ftp"; want slack, discord, webhook+https or smtp
rhythm tuner scan: --notify: slack://hooks.slack.com: the URL has no path; paste the whole webhook URL
rhythm tuner scan: --notify: webhook+http://hooks.example.com: plain http is only for a receiver on this machine; use webhook+https://
```

For a scan on a schedule that keeps its own history and notifies on every run, use [`rhythm tuner watch`](Tuner-Watch-And-Serve).

## A report for a client: --html and report

`--html <file>` writes a self-contained HTML report next to whatever else the scan writes: one file, inline styles for light, dark and print, no scripts, no remote assets and a strict Content-Security-Policy, so it's safe to email or attach. With `--since` it marks new findings, shows how each score moved and lists what was fixed.

```text
$ rhythm tuner scan 127.0.0.1:20400 --plain --since rep/before.json --html rep/client.html --brand-name "Acme Web"
…
HTML report written to rep/client.html
```

| Flag | |
|---|---|
| `--brand-name <name>` | your agency's name: the title reads `Acme Web security report: <site>` and the byline `Prepared by Acme Web with rhythm-tuner 0.4.0` |
| `--brand-logo <file>` | your logo: PNG, JPEG or SVG, up to 512 KB, embedded in the file. SVGs are rebuilt from a safe subset; scripts, event handlers, external references and `style` elements are refused with a message naming what's wrong |

`-o client.html` also writes the HTML report (and takes the brand flags). The brand flags without an HTML report are a usage error: `--brand-name and --brand-logo only go on the HTML report (--html)`.

The report has an overview table when it covers several sites, then per site: the fingerprint, the score and its trend, counts per severity and new findings, the plugins and themes, every finding with its evidence, masked secrets, advisories (with `NEW` on advisories the earlier report lacked), impact and the full fix. A "Fixed since" list ends it. Its footer says what a clean report means: `A clean report means none of Tuner's checks found a problem. It is not proof that a site is secure.`

### Render saved reports: report

`rhythm tuner report` turns saved JSON reports into HTML, Markdown, SARIF or JSON without scanning again. Several files make one report with a section per site:

```text
$ rhythm tuner report rep/after.json --since rep/before.json --brand-name "Acme Web" -o rep/client2.html
Report written to rep/client2.html
```

| Flag | |
|---|---|
| `--html`, `--md`, `--sarif`, `--json` | the format (or let `-o` pick it from the extension) |
| `-o <file>` | write to a file instead of stdout |
| `--since <file>` | mark what is new in the HTML report (for the changes alone, use `diff`) |
| `--brand-name`, `--brand-logo` | as for `scan` |

The HTML is byte for byte what `scan --html` writes for the same scan. Targets that failed are kept from the saved JSON. Reports saved by older Tuner releases render too.

## Baselines or --since?

Both start from a saved JSON report. They do different jobs:

| | `--baseline` | `--since` |
|---|---|---|
| **Hides** accepted findings from the report, the score and `--fail-on` | yes | no |
| **Lists** what is new and fixed | no | yes |
| **Gate** | `--fail-on` on what's left | `--fail-on-new` on what's new |
| **Use it for** | a reviewed list of accepted risk, committed to the repository | "what changed since last week" |

Baselines match by fingerprint too, one advisory at a time: when a baselined plugin gets a new vulnerability, the finding comes back with only the new advisories. See [Accepting known findings](Tuner-CI#accepting-known-findings).

---
<sub>← [Integrity and malware](Tuner-Integrity-And-Malware) · [Home](Home) · [Known vulnerabilities](Tuner-Known-Vulnerabilities) →</sub>
