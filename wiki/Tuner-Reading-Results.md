This page explains every part of Tuner's terminal report: what each severity means and what to fix first, why some findings are marked "likely" and exactly how the Tune score is calculated.

## Anatomy of the report

```text
  ♪ rhythm tuner 0.3.1                                        yoursite.com      ← 1 header
  ────────────────────────────────────────────────────────────────────────

  ▸ Fingerprint   WordPress 6.4.3 · PHP 8.1.27 · Apache/2.4.57 (Debian)          ← 2 fingerprint
  ▸ Components    theme astra 4.6.0 · contact-form-7 5.3.1                       ← 3 components
  ▸ Baseline      server returns real 404s                                      ← 4 baseline

  CRITICAL  5 ────────────────────────────────────────────────────────────      ← 5 section

 ▌  CRITICAL   .env file exposed                                    /.env       ← 6 card
 ▌             DB_PASSWORD=Su•••••   sk_live_•••••                               ← 7 evidence
 ▌             → Deny access to dotfiles, then rotate every secret in th…        ← 8 fix

 ▌  CRITICAL   Apache 2.4.57: 73 known vulnerabilities                          ← outline badge
 ▌             ~ likely: a distro build may already include the fix              ← 9 caveat
 ▌             CRITICAL CVE-2024-38475 Improper escaping of output in mo…
 ▌             …and 70 more
 ▌             → Update Apache to 2.4.69 or later from your OS or vendor…

  LOW  6 ─────────────────────────────────────────────────────────────────

    ●  X-Content-Type-Options header missing                                    ← 10 compact list
    ●  XML-RPC enabled                                        /xmlrpc.php

 ╭────────────────────────────────────────────────────────────────────╮
 │  TUNE SCORE                                               0 / 100  │       ← 11 score panel
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
 │  ✖ 5 critical   ▲ 3 high   ■ 2 medium   ● 6 low   · 3 info         │
 ╰────────────────────────────────────────────────────────────────────╯
  37 checks · 78 requests · 15.4s                                               ← 12 stats
  ! vulnerability data: WPVulnerability (wpvulnerability.com, CC0)               ← 13 notes

  → full report: rhythm tuner scan yoursite.com -o report.md
```

1. **Header.** Tuner's version and the target as you typed it.
2. **Fingerprint.** The platform, PHP version and server Tuner detected, e.g. `WordPress 6.4.3 · PHP 8.1.27 · nginx`. A missing number means the version couldn't be read: `PHP` alone means the site runs PHP but hides its version. `unknown` means Tuner recognized none of them.
3. **Components.** Plugins and themes the home page loads, with versions where Tuner could read them. Up to six are shown, then `+N more`. Themes are prefixed with `theme`.
4. **Baseline.** `server returns real 404s`, or `soft-404 detected, filtering enabled` when the server answers `200` for pages that don't exist. In the second case Tuner compares every hit with a random path of the same shape, so the catch-all page doesn't produce false findings. Local scans have no baseline line.
5. **Severity sections.** Findings grouped from critical to info, with a count. Within a section, confirmed findings come before likely ones.
6. **Cards.** Critical, high and medium findings get a card: a severity badge, the title and, when it fits, the path that triggered it. The colored rail on the left matches the severity.
7. **Evidence.** One line showing why the finding fired: the masked secrets found, the text that matched, or for vulnerability findings the top three advisories (`SEVERITY CVE title (fixed in x.y.z)`, `[actively exploited]` for CISA KEV entries, `(no fix for this branch)` for end-of-life PHP) and `…and N more`.
8. **Fix.** The `→` line is the first line of the fix. The full fix, often with nginx and Apache variants, is in the Markdown and JSON reports.
9. **Caveat.** Low-confidence findings get an `~ likely:` line saying why they can't be confirmed. See [Confidence](#confidence-and-likely-findings).
10. **Compact list.** Low and info findings are one line each: a glyph, the title and the path.
11. **Score panel.** The [Tune score](#the-tune-score), a bar and the count per severity. The panel is green at 80 and above, amber from 50 to 79 and red below 50. With no findings it reads `✔ nothing out of tune`.
12. **Stats.** How many checks ran, requests sent and how long it took. Extra items appear when they apply: `N baselined` (findings hidden by `--baseline`), `throttled N× (slowed down)` (the site answered `429` or `503`; see [Requests](Tuner-Requests#retries-and-throttling)) and `N errors` (requests that failed after retries).
13. **Notes.** Lines starting with `!`: warnings (for example that cached vulnerability data was used because the data service couldn't be reached) and data attribution.

Card text is truncated to fit 72 columns, marked with `…`. Save a report with `-o report.md` to read everything.

### Filled and outline badges

The severity badge on a card is **filled** (solid background) for a confirmed finding and **outline** (colored text, no background) for a likely one, so the two read apart at a glance. Likely findings also carry the `~ likely:` caveat line. In JSON they have `"confidence": "low"` and a `caveat` field.

### Glyphs

| Glyph | Severity |
|---|---|
| `✖` | critical |
| `▲` | high |
| `■` | medium |
| `●` | low |
| `·` | info |

## Severities and what to fix first

| Severity | Typical findings | What to do |
|---|---|---|
| **critical** | an exposed `.env`, `wp-config.php` backup, SQL dump or backup archive; a component with a critical known vulnerability | Today. Block or remove the file, update the component. If secrets were exposed, rotate them: assume they've been copied. |
| **high** | an exposed `.git` folder or debug log, Laravel Ignition, an expired or untrusted certificate, end-of-life PHP, high-rated vulnerabilities | This week. These are directly useful to an attacker. |
| **medium** | missing HSTS, user enumeration, PHP errors shown to visitors, directory listing, plain HTTP, medium-rated vulnerabilities | Plan it in. Each one makes an attack easier or leaks information. |
| **low** | missing security headers, version leaks, XML-RPC enabled, outdated or abandoned plugins | Hardening. Fix when convenient; batch them. |
| **info** | `readme.html` present, admin-ajax reachable, a default table prefix | Good to know. No score penalty. |

A practical order:

1. **Exposed secrets first**, regardless of anything else. Block access, then rotate every password, key and token in the file.
2. **Actively exploited vulnerabilities** (`[actively exploited]`, or `known-exploited` in the JSON tags). Attackers already use these.
3. **The rest of critical and high**, confirmed findings before likely ones.
4. **Medium**, then **low** as a hardening pass.

Use `--fail-on high` in CI to stop new critical or high findings from shipping; see [CI and code scanning](Tuner-CI).

## Confidence and "likely" findings

Every finding has a confidence of `high` or `low`. Low-confidence findings are shown as **likely**: the evidence points to a problem that Tuner can't confirm from where it stands. They're sorted after confirmed findings of the same severity and count half in the score.

| Why it's likely | Caveat shown | What to do |
|---|---|---|
| **Distribution build.** The server reports a distro package version such as `PHP/7.4.3-4ubuntu2.19` or `Apache/2.4.57 (Debian)`. Distributions backport security fixes without changing the upstream version number. | `a distro build may already include the fix` | Check the package's security status with your distribution (for example its security tracker or `apt changelog`), or ask your host. |
| **Hidden PHP version.** The site runs PHP but doesn't reveal which version (`php-version-hidden`). Tuner lists recent critical PHP flaws and the releases that fix them. | `the PHP version is hidden, so this can't be confirmed` | Check the version yourself: `php -v` on the server, your hosting panel, or WordPress → Tools → Site Health → Info → Server. Or run a [local scan](Tuner-Local-Scans). |
| **End-of-life inference.** Branches past end of life stop getting advisories. Tuner infers that advisories for the oldest supported branch also reach older ones and adds them with "no fix for this branch". | `inferred from a supported branch's advisories` | Upgrade to a supported PHP branch; that removes the question. |
| **Order of calls (local CVE-2026-115560 check).** The configuration is unsafe and the code switches the session handler. Tuner couldn't confirm from one file that the switch runs before `session_start()`. | `likely: the code's order of calls couldn't be confirmed from one file` | Apply the mitigation (turn off upload progress). See [Local scans](Tuner-Local-Scans#session-upload-progress-cve-2026-115560). |
| **Unknown PHP configuration (local CVE-2026-115560 check).** The code switches the handler; PHP's configuration couldn't be read. | `likely: PHP's configuration couldn't be read; check session.serialize_handler` | Check `session.serialize_handler` in the PHP that serves the site. |
| **phpinfo() settings (remote CVE-2026-115560 check).** An exposed `phpinfo()` page shows the two risky settings; whether the code switches the handler can only be checked on the server. | `not confirmed from outside` | Delete the phpinfo page, then run a local scan. |
| **Missing settings in an included file (local wp-config review).** `wp-config.php` includes other files that may define what's missing. | Evidence line: `wp-config.php includes other files, which may set this` | Check the included files. |

Two related findings are also low confidence: `wp-components-unverified` (plugins or themes with known vulnerabilities whose installed version couldn't be read) and the remote `php-session-upload-progress` check.

> [!NOTE]
> "Likely" doesn't mean "probably fine". A likely critical finding still deserves a look; it means the last step of confirmation is yours.

## The Tune score

Each scan gets a score from 0 to 100. It's calculated in [`engine.Score`](https://github.com/RhythmShahriar/tuner/blob/master/engine/select.go) like this:

1. Group findings by **check ID**. A check that fires on several paths counts **once**, at its worst severity.
2. Each distinct failing check costs a penalty by that severity:

   | Severity | Penalty |
   |---|---|
   | critical | 30 |
   | high | 15 |
   | medium | 6 |
   | low | 2 |
   | info | 0 |

3. If **every** finding for that check is low confidence, the penalty is halved, rounding up: critical 15, high 8, medium 3, low 1. One confirmed finding for the check means the full penalty.
4. Score = 100 minus the sum of penalties, never below 0.

Vulnerability findings have one ID per component (`vuln-plugin-elementor`, `vuln-theme-astra`, `vuln-php`…), so each vulnerable plugin counts separately.

**Worked example.** A site with an exposed `.env` (critical), missing HSTS (medium), three missing security headers (low each) and a likely critical Apache finding from a distro build:

| Check | Penalty |
|---|---|
| `leak-dotenv` (critical) | 30 |
| `hdr-hsts-missing` (medium) | 6 |
| `hdr-csp-missing`, `hdr-frame-options-missing`, `hdr-content-type-options-missing` (low) | 2 + 2 + 2 |
| `vuln-apache` (critical, likely) | 15 |
| **Score** | 100 − 57 = **43** |

The score reflects what's in the report. Findings hidden with `--baseline` are left out and the score is recomputed; checks you didn't run (a narrower `--profile`, `--tags`, `--id` or `--severity`) can't cost points. Compare scores between runs that used the same options.

## Plain mode

When output isn't a terminal (a pipe, a file, a CI log), or with `--plain`, Tuner prints a plain report with no color, box drawing or decorative glyphs. It suits logs and screen readers:

```text
rhythm tuner 0.3.1 - scanning yoursite.com

Fingerprint: WordPress 6.4.3 · PHP 8.1.27 · Apache/2.4.57 (Debian)
Components: theme astra 4.6.0 · contact-form-7 5.3.1
Baseline: server returns real 404s

[LOW] X-Content-Type-Options header missing - https://yoursite.com
  evidence: no X-Content-Type-Options header
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - https://yoursite.com/wp-content/plugins/contact-form-7/
  - CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestricted File Upload (fixed in 5.3.2)
  - HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticated (Editor+) Arbitrary File Upload (fixed in 5.8.4)
  - MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cross-Site Scripting (fixed in 5.9.2)
  - …and 3 more
  fix: Update contact-form-7 to 6.0.6 or later (latest is 6.2.3): wp plugin update contact-form-7
[CRITICAL] .env file exposed - https://yoursite.com/.env
  secrets: DB_PASSWORD=Su•••••  sk_live_•••••
  fix: Deny access to dotfiles, then rotate every secret in the file.
[CRITICAL, likely] Apache 2.4.57: 73 known vulnerabilities - https://yoursite.com
  likely: a distro build may already include the fix
  - CRITICAL CVE-2024-38475 Apache HTTPD 2.4 <= 2.4.59 [actively exploited]
  …

Score 0/100 - 5 critical, 3 high, 2 medium, 6 low, 3 info - 37 checks, 78 requests, 15.4s
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
```

Differences from the pretty view:

- Findings **stream in the order they're found**, one `[SEVERITY] title - URL` line each, instead of being grouped by severity at the end.
- Each finding has at most one detail: up to three advisories, the masked secrets, or the first evidence line.
- The `fix:` line is shown only for critical and high findings.
- A low-confidence finding reads `[SEVERITY, likely]`, with the reason on the next line, so logs never present it as confirmed. (Before v0.3.2 plain output left the marker out.) The Markdown report adds a "Why only likely" line. `--json` has both `confidence` and `caveat`.
- The summary is one line, `Score N/100 - counts - stats`, followed by `note:` lines.

See [Output and reports](Tuner-Output) for every output mode.

---
<sub>← [Your first scan](Tuner-First-Scan) · [Home](Home) · [Choosing checks](Tuner-Choosing-Checks) →</sub>
