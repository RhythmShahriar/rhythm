This page explains every part of Tuner's terminal report: what each severity means and what to fix first, why some findings are marked "likely" and exactly how the Tune score is calculated.

## Anatomy of the report

```text
  ♪ rhythm tuner 0.4.0           127.0.0.1:20400 (local address: 50 req/s)     ← 1 header
  ────────────────────────────────────────────────────────────────────────

  ▸ Fingerprint   WordPress 6.9.3 · PHP 8.1.2 (from x-powered-by /) · nginx     ← 2 fingerprint
  ▸ Components    contact-form-7 5.3.1 · theme astra 4.6.0 · wordpress-seo      ← 3 components
  ▸ Baseline      server returns real 404s                                      ← 4 baseline

  CRITICAL  4 ────────────────────────────────────────────────────────────      ← 5 section

 ▌  CRITICAL   .env file exposed: 1 secret (DB_PASSWORD=Su•••••)    /.env       ← 6 card
 ▌             DB_PASSWORD=Su•••••                                              ← 7 evidence
 ▌             → Deny access to dotfiles, then rotate every secret in the       ← 8 fix
 ▌               file. (see report)

 ▌  CRITICAL   contact-form-7 5.3.1: 6 known vulnerabilities
 ▌             CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestri…
 ▌             HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticate…
 ▌             MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cr…
 ▌             …and 3 more
 ▌             → Update contact-form-7 to 6.0.6 or later (latest is
 ▌               6.1.3): wp plugin update contact-form-7

  LOW  7 ─────────────────────────────────────────────────────────────────

    ●  X-Content-Type-Options header missing                                    ← 9 compact list
    ●  XML-RPC enabled                                        /xmlrpc.php

 ╭────────────────────────────────────────────────────────────────────╮
 │  TUNE SCORE                                               0 / 100  │       ← 10 score panel
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
 │  ✖ 4 critical   ▲ 2 high   ■ 1 medium   ● 7 low   · 1 info         │
 ╰────────────────────────────────────────────────────────────────────╯
  77 checks · 180 requests · 3.6s                                               ← 11 stats
  ! vulnerability data: WPVulnerability (wpvulnerability.com, CC0)               ← 12 notes

  → run rhythm tuner explain <id> for the full fix                              ← 13 hints
  → full report: rhythm tuner scan 127.0.0.1:20400 -o report.md
```

1. **Header.** Tuner's version and the target as you typed it (a password in it shows as `xxxxx`). Notes in brackets say when a [`.tuner.yml`](Tuner-Config) is in use or a local address gets the faster rate.
2. **Fingerprint.** The platform, PHP version and server Tuner detected, e.g. `WordPress 6.9.3 · PHP 8.1.2 (from x-powered-by /) · nginx`, or `PHP 8.3.36 (from php on PATH)` in a local scan. The PHP version names its source when Tuner knows it. `PHP (version unknown)` means the site runs PHP but hides its version. `unknown` means Tuner recognized none of them.
3. **Components.** Plugins and themes found on the page, in the REST API index or, with `--plugins deep`, by their `readme.txt`, with versions where Tuner could read them. Up to six are shown, then `+N more`. Themes are prefixed with `theme`.
4. **Baseline.** `server returns real 404s`, or `soft-404 detected, filtering enabled` when the server answers `200` for pages that don't exist. In the second case Tuner compares every hit with a random path of the same shape, so the catch-all page doesn't produce false findings. The line is left out when the home page was blocked or failed. Local scans have none.
5. **Severity sections.** Findings grouped from critical to info, with a count. Within a section, confirmed findings come before likely ones.
6. **Cards.** Critical, high and medium findings get a card: a severity badge, the title and, when it fits, the path that triggered it. The colored rail on the left matches the severity.
7. **Evidence.** Why the finding fired: the masked secrets found, the text that matched, or for vulnerability findings the top three advisories (`SEVERITY CVE title (fixed in x.y.z)`, `[actively exploited]` for CISA KEV entries, `(no fix for this branch)` for end-of-life PHP) and `…and N more`.
8. **Fix.** The `→` lines are the first line of the fix, wrapped to the terminal's width; a URL is never cut. `(see report)` means the fix has more lines, often nginx and Apache variants.
9. **Compact list.** Low and info findings are one line each: a glyph, the title and the path.
10. **Score panel.** The [Tune score](#the-tune-score), a bar and the count per severity. The panel is green at 80 and above, amber from 50 to 79 and red below 50. With no findings it reads `✔ nothing out of tune`. When a baseline or ignore entries hid findings, it says so: `100 / 100 after baseline (15 hidden)`.
11. **Stats.** How many checks ran, requests sent and how long it took. Extra items appear when they apply: `throttled N× (slowed down)` (the site answered `429` or `503`; see [Requests](Tuner-Requests#retries-and-throttling)) and `N errors` (requests that failed after retries).
12. **Notes.** Lines starting with `!`: data attribution, warnings (cached data used, a skipped file) and, for a scan that didn't finish, why: `Incomplete: stopped by --max-time`, `Incomplete: blocked by Cloudflare challenge`, `Incomplete: the home page did not load`, `Incomplete: stopped at the request budget`, `Incomplete: some checksums for --integrity could not be loaded`, `Score excludes known vulnerabilities (vulnerability data missing)`. A partial scan's score is not a clean bill of health.
13. **Hints.** `explain` prints a finding's full fix offline; `-o report.md` saves everything.

The report follows the terminal's width (between 56 and 100 columns). Long advisory titles are cut with `…`; save a report with `-o report.md` to read everything.

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

Use `--fail-on high` in CI to stop critical or high findings from shipping, or `--since last.json --fail-on-new high` to stop only new ones; see [CI and code scanning](Tuner-CI).

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

Other findings that are always low confidence: `wp-components-unverified` (plugins or themes with known vulnerabilities whose installed version couldn't be read), the remote `php-session-upload-progress` check, `local-malware-php-in-uploads` (a few plugins keep PHP files in uploads on purpose) and `local-integrity-plugin-unlisted` (a development build has no checksums either).

In CI, `--min-confidence confirmed` keeps likely findings in the report but stops them from failing `--fail-on` and `--fail-on-new`. That's the usual fix for a pipeline that fails on every Ubuntu host's PHP build.

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

The score reflects what's in the report. Findings hidden with `--baseline` or `.tuner.yml` ignore entries are left out and the score is recomputed (the summary says `after baseline (N hidden)`); checks you didn't run (a narrower `--profile`, `--tags`, `--id` or `--severity`) can't cost points. Compare scores between runs that used the same options.

## Plain mode

When output isn't a terminal (a pipe, a file, a CI log), or with `--plain`, Tuner prints a plain report with no color, box drawing or decorative glyphs. It suits logs and screen readers. The same test site, shortened:

```text
rhythm tuner 0.4.0 - scanning 127.0.0.1:20400 (local address: 50 req/s)

Fingerprint: WordPress 6.9.3 · PHP 8.1.2 (from x-powered-by /) · nginx
Components: contact-form-7 5.3.1 · theme astra 4.6.0 · wordpress-seo
Baseline: server returns real 404s

[LOW] X-Content-Type-Options header missing - http://127.0.0.1:20400
  evidence: no X-Content-Type-Options header
  fix: nginx:  add_header X-Content-Type-Options nosniff always; (see report)
…
[CRITICAL] .env file exposed: 1 secret (DB_PASSWORD=Su•••••) - http://127.0.0.1:20400/.env
  secrets: DB_PASSWORD=Su•••••
  fix: Deny access to dotfiles, then rotate every secret in the file. (see report)
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - http://127.0.0.1:20400/wp-content/plugins/contact-form-7/
  - CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestricted File Upload (fixed in 5.3.2)
  - HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticated (Editor+) Arbitrary File Upload (fixed in 5.8.4)
  - MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cross-Site Scripting (fixed in 5.9.2)
  - …and 3 more
  fix: Update contact-form-7 to 6.0.6 or later (latest is 6.1.3): wp plugin update contact-form-7
…

Findings by severity:
[CRITICAL] .env file exposed: 1 secret (DB_PASSWORD=Su•••••) - http://127.0.0.1:20400/.env
[CRITICAL] WordPress 6.9.3: 22 known vulnerabilities - http://127.0.0.1:20400/
[CRITICAL] PHP 8.1.2: 55 known vulnerabilities - http://127.0.0.1:20400/
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - http://127.0.0.1:20400/wp-content/plugins/contact-form-7/
[HIGH] PHP 8.1 is past end of life - http://127.0.0.1:20400/
[HIGH] guzzlehttp/guzzle 7.4.1: 3 known vulnerabilities (from exposed composer.lock) - http://127.0.0.1:20400/composer.lock
[MEDIUM] Site served over plain HTTP - http://127.0.0.1:20400
…
[INFO] Referrer-Policy header missing - http://127.0.0.1:20400

Score 0/100 - 4 critical, 2 high, 1 medium, 7 low, 1 info - 77 checks, 180 requests, 3.6s
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
run rhythm tuner explain <id> for the full fix
```

Differences from the terminal view:

- Findings **stream in the order they're found**, one `[SEVERITY] title - URL` line each, then a **Findings by severity** block lists them again, most severe first, so a CI log reads well from the bottom.
- Each finding has its detail (up to three advisories, the masked secrets, or the first evidence line) and a `fix:` line, whatever its severity. A longer fix ends in `(see report)`.
- A low-confidence finding reads `[SEVERITY, likely]`, with the reason on the next line, so logs never present it as confirmed. The Markdown report adds a "Why only likely" line; JSON has `confidence` and `caveat`.
- Local findings show the path inside the scanned folder (`/wp-config.php`), not the full `file://` URL.
- The summary is one line, `Score N/100 - counts - stats`, preceded by any `Incomplete:` notes and followed by `note:` lines. `-q` keeps the findings and this line.

See [Output and reports](Tuner-Output) for every output mode.

---
<sub>← [Your first scan](Tuner-First-Scan) · [Home](Home) · [Choosing checks](Tuner-Choosing-Checks) →</sub>
