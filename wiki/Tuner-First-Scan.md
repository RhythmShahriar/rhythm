This tutorial takes you from a fresh install to a saved report in about five minutes. You'll scan a site, read what Tuner found, look up a full fix, save a report, try the quick profile and scan a staging site behind a password.

You need a site you own or have permission to test. The commands use `yoursite.com`; replace it with your own. The output shown comes from a deliberately broken test site served on `127.0.0.1:20400`.

## 1. Check the install

```text
$ rhythm --version
rhythm 0.7.2
$ rhythm tuner --version
rhythm-tuner 0.4.0
```

If the second command says `rhythm: tuner isn't installed`, run `rhythm install tuner`. If `rhythm` isn't found at all, follow [Install](Install).

## 2. Run your first scan

```sh
rhythm tuner scan yoursite.com
```

You can type a bare hostname. Tuner tries `https://` first and falls back to `http://`. To be explicit, pass a full URL such as `https://yoursite.com` or `https://yoursite.com/blog`.

The first time you scan, Tuner prints a one-time reminder:

```text
Note: scan only sites you own or are authorised to test.
Tuner is non-intrusive and never sends exploit payloads. This notice is shown once.
```

While the scan runs, a progress line shows how many requests are done and a running tally of critical, high and medium findings. A public site takes 15 to 30 seconds at the default 5 requests a second. Your own server on `localhost` or a private address gets 50 a second; the header then says `(local address: 50 req/s)`.

## 3. What you'll see

The terminal report of the test site, after the progress line has cleared (the low and info sections are shortened):

```text
  ♪ rhythm tuner 0.4.0           127.0.0.1:20400 (local address: 50 req/s)
  ────────────────────────────────────────────────────────────────────────

  ▸ Fingerprint   WordPress 6.9.3 · PHP 8.1.2 (from x-powered-by /) · nginx
  ▸ Components    contact-form-7 5.3.1 · theme astra 4.6.0 · wordpress-seo
  ▸ Baseline      server returns real 404s

  CRITICAL  4 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   .env file exposed: 1 secret (DB_PASSWORD=Su•••••)    /.env
 ▌             DB_PASSWORD=Su•••••
 ▌             → Deny access to dotfiles, then rotate every secret in the
 ▌               file. (see report)

 ▌  CRITICAL   WordPress 6.9.3: 22 known vulnerabilities
 ▌             CRITICAL CVE-2026-63030 WordPress 6.9.x before 6.9.5 and …
 ▌             HIGH WordPress 6.9.x before 6.9.5 and 7.0.x before 7.0.2 …
 ▌             HIGH CVE-2026-93485 Improper Neutralization of Input Duri…
 ▌             …and 19 more
 ▌             → Update WordPress to the latest release (6.9.4):
 ▌               Dashboard → Updates, or: wp core update

 ▌  CRITICAL   PHP 8.1.2: 55 known vulnerabilities
 ▌             CRITICAL CVE-2024-4577 When using Apache and PHP-CGI on W…
 ▌             CRITICAL CVE-2025-14179 The PDO Firebird driver improperl…
 ▌             CRITICAL CVE-2026-6722 The SOAP extension's object dedupl…
 ▌             …and 52 more
 ▌             → Upgrade to a supported PHP branch (8.2 or later). PHP
 ▌               8.1 is end of life, so 13 of these issues will never be
 ▌               fixed on it.

 ▌  CRITICAL   contact-form-7 5.3.1: 6 known vulnerabilities
 ▌             CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestri…
 ▌             HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticate…
 ▌             MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cr…
 ▌             …and 3 more
 ▌             → Update contact-form-7 to 6.0.6 or later (latest is
 ▌               6.1.3): wp plugin update contact-form-7

  HIGH  2 ────────────────────────────────────────────────────────────────

 ▌  HIGH       PHP 8.1 is past end of life
 ▌             → Upgrade to a supported PHP branch (8.2 or later).

 ▌  HIGH       guzzlehttp/guzzle 7.4.1: 3 known vulnerabilities (from ex…
 ▌             HIGH CVE-2022-29248 Cross-domain cookie leakage (fixed in…
 ▌             HIGH CVE-2022-31042 Failure to strip the Cookie header on…
 ▌             HIGH CVE-2022-31091 Change in port should be considered a…
 ▌             → Update guzzlehttp/guzzle to 7.4.5 or later: composer
 ▌               update guzzlehttp/guzzle --with-dependencies

  MEDIUM  1 ──────────────────────────────────────────────────────────────

 ▌  MEDIUM     Site served over plain HTTP
 ▌             → Get a free certificate (e.g. certbot) and redirect all
 ▌               HTTP traffic. (see report)

  LOW  7 ─────────────────────────────────────────────────────────────────

    ●  X-Content-Type-Options header missing
    ●  Content-Security-Policy header missing
    ●  Composer manifest exposed                           /composer.lock
    ●  XML-RPC enabled                                        /xmlrpc.php
 ⋮

 ╭────────────────────────────────────────────────────────────────────╮
 │  TUNE SCORE                                               0 / 100  │
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
 │  ✖ 4 critical   ▲ 2 high   ■ 1 medium   ● 7 low   · 1 info         │
 ╰────────────────────────────────────────────────────────────────────╯
  77 checks · 180 requests · 3.6s
  ! vulnerability data: WPVulnerability (wpvulnerability.com, CC0)

  → run rhythm tuner explain <id> for the full fix
  → full report: rhythm tuner scan 127.0.0.1:20400 -o report.md
```

Reading from the top:

- **Fingerprint** and **Components**: what Tuner detected and the plugin and theme versions it could read. `wordpress-seo` has no version: Tuner found it in the REST API index, not on the page.
- **Baseline**: whether the server returns real 404s. If it says `soft-404 detected, filtering enabled`, the server answers `200` for missing pages and Tuner compares every hit against a random path to avoid false findings.
- **Severity sections**: critical, high and medium findings appear as cards with evidence and a `→` fix; low and info findings as a compact list.
- **The guzzle finding** came from an exposed `composer.lock`: Tuner matched the packages it lists against known vulnerabilities.
- **Secrets** like `DB_PASSWORD=Su•••••` are masked.
- **Tune score**: 100 minus a penalty for each failing check. This test site scores 0.

[Reading results](Tuner-Reading-Results) explains every part of the report in detail.

> [!TIP]
> Start with the critical section. An exposed `.env` or `wp-config.php` backup means your secrets are already public: fix access first, then rotate the passwords and keys.

## 4. Look up the full fix

A fix that doesn't fit on the card ends in `(see report)`. `rhythm tuner explain` prints the whole thing, offline:

```text
$ rhythm tuner explain leak-dotenv
leak-dotenv: .env file exposed
  Severity:   critical
  Confidence: high
  Profile:    quick
  Tags:       leak, secrets, php, laravel, wordpress
  Requests:   /.env /.env.local /.env.production /.env.backup

Impact
  Anyone can read database passwords, API keys and app secrets.

Fix
  Deny access to dotfiles, then rotate every secret in the file.

nginx
  location ~ /\.(?!well-known) { deny all; }

Apache
  <FilesMatch "^\.">
    Require all denied
  </FilesMatch>
…
```

## 5. Save a report

The terminal view is for you. To share findings with a client, a teammate or an issue tracker, save a report:

```sh
rhythm tuner scan yoursite.com -o report.md
rhythm tuner scan yoursite.com -o client.html --brand-name "Your Agency"
```

Tuner picks the format from the file extension and still shows the live view in your terminal. When it's done it prints `Report written to report.md`. The Markdown report has a summary table, the plugins and themes found and a section per finding with the evidence, advisories, impact and the full fix. The HTML report is one self-contained file for a client.

Other formats work the same way: `-o report.json`, `-o results.jsonl`, `-o tuner.sarif`. Keep the JSON one: the next scan can compare with it (`--since report.json`) and tell you what changed. See [Output and reports](Tuner-Output) and [Changes and notifications](Tuner-Changes-And-Notifications).

## 6. Try the quick profile

For a fast first look, the quick profile runs only the highest-value checks:

```sh
rhythm tuner scan yoursite.com --profile quick
```

On the test site it ran 21 checks in 17 requests, against 77 checks and 180 requests for the default profile:

```text
Score 0/100 - 4 critical, 1 high, 1 medium, 6 low, 1 info - 21 checks, 17 requests, 300ms
```

It still found the exposed `.env` and the vulnerable versions. It skipped checks such as the `composer.lock` and backup searches, so it missed the guzzle finding. Quick scans don't read plugin `readme.txt` or theme `style.css` files either, so versions come only from hints in asset URLs. Use the default profile for an accurate inventory. See [Choosing checks](Tuner-Choosing-Checks).

## 7. Scan a staging site behind basic auth

Staging sites are often protected with HTTP basic auth. Send the credentials as a header:

```sh
rhythm tuner scan https://staging.yoursite.com \
  -H "Authorization: Basic $(printf 'user:pass' | base64)"
```

`-H` can be repeated. Headers and cookies go only to the host you named: never to another host a redirect leads to, never over plain http after https. If staging uses a self-signed certificate, add `-k` (`--insecure`); Tuner still reports the untrusted certificate as a finding. To scan as a logged-in WordPress user instead, pass the login cookie with `--cookie`. See [Requests and politeness](Tuner-Requests).

## 8. What to do next

1. **Fix from the top down.** Critical and high first; [Reading results](Tuner-Reading-Results) has a prioritizing guide. `rhythm tuner fix report.json --server nginx` turns the server-side fixes into one config to review.
2. **Rescan** after each fix to confirm it's gone and watch the score climb.
3. **Scan on the server** if you can log in to it. A local scan reads exact versions, finds plugins the site doesn't show and can check core files for changes: `rhythm tuner scan /var/www/yoursite --integrity`. See [Local scans](Tuner-Local-Scans).
4. **Scan all your sites** from a list: [Batch scans](Tuner-Batch-Scans).
5. **Automate it** in CI or on a schedule: [CI and code scanning](Tuner-CI) and [Watch and serve](Tuner-Watch-And-Serve).

---
<sub>← [Tuner overview](Tuner) · [Home](Home) · [Reading results](Tuner-Reading-Results) →</sub>
