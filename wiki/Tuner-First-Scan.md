This tutorial takes you from a fresh install to a saved report in about five minutes. You'll scan a site, read what Tuner found, save a Markdown report, try the quick profile, and scan a staging site behind a password.

You need a site you own or have permission to test. The examples use `yoursite.com`; replace it with your own.

## 1. Check the install

```sh
rhythm --version
rhythm tuner --version
```

```text
rhythm 0.1.0
rhythm-tuner 0.3.1
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

While the scan runs, a progress line shows how many requests are done and a running tally of critical, high and medium findings. A typical scan takes 15 to 30 seconds.

## 3. What you'll see

Here is a real scan of a deliberately out-of-date test site, trimmed (`⋮` marks cut lines):

```text
  ♪ rhythm tuner 0.3.1                                        yoursite.com
  ────────────────────────────────────────────────────────────────────────

  ▸ Fingerprint   WordPress 6.4.3 · PHP 8.1.27 · Apache/2.4.57 (Debian)
  ▸ Components    theme astra 4.6.0 · contact-form-7 5.3.1
  ▸ Baseline      server returns real 404s

  CRITICAL  5 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   .env file exposed                                    /.env
 ▌             DB_PASSWORD=Su•••••   sk_live_•••••
 ▌             → Deny access to dotfiles, then rotate every secret in th…

 ▌  CRITICAL   contact-form-7 5.3.1: 6 known vulnerabilities
 ▌             CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestri…
 ▌             HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticate…
 ▌             MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cr…
 ▌             …and 3 more
 ▌             → Update contact-form-7 to 6.0.6 or later (latest is 6.2.…

 ▌  CRITICAL   Apache 2.4.57: 73 known vulnerabilities
 ▌             ~ likely: a distro build may already include the fix
 ▌             CRITICAL CVE-2024-38475 Improper escaping of output in mo…
 ⋮

  HIGH  3 ────────────────────────────────────────────────────────────────

 ▌  HIGH       PHP 8.1 is past end of life
 ▌             → Upgrade to a supported PHP branch (8.2 or later).

 ▌  HIGH       WordPress debug.log exposed          /wp-content/debug.log
 ▌             PHP Warning:
 ▌             → Turn off WP_DEBUG_LOG on production or point it outside…
 ⋮

  LOW  6 ─────────────────────────────────────────────────────────────────

    ●  X-Content-Type-Options header missing
    ●  Content-Security-Policy header missing
    ●  XML-RPC enabled                                        /xmlrpc.php
 ⋮

 ╭────────────────────────────────────────────────────────────────────╮
 │  TUNE SCORE                                               0 / 100  │
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
 │  ✖ 5 critical   ▲ 3 high   ■ 2 medium   ● 6 low   · 3 info         │
 ╰────────────────────────────────────────────────────────────────────╯
  37 checks · 78 requests · 15.4s
  ! vulnerability data: WPVulnerability (wpvulnerability.com, CC0)

  → full report: rhythm tuner scan yoursite.com -o report.md
```

Reading from the top:

- **Fingerprint** and **Components**: what Tuner detected, and the plugin and theme versions it could read.
- **Baseline**: whether the server returns real 404s. If it says `soft-404 detected, filtering enabled`, the server answers `200` for missing pages and Tuner compares every hit against a random path to avoid false findings.
- **Severity sections**: critical, high and medium findings appear as cards with evidence and a `→` fix line; low and info findings as a compact list.
- **`~ likely:`** lines mark findings Tuner can't confirm from outside. Here, Debian may have backported fixes into its Apache 2.4.57 package.
- **Secrets** like `DB_PASSWORD=Su•••••` are always masked.
- **Tune score**: 100 minus a penalty for each failing check. This test site scores 0.

[Reading results](Tuner-Reading-Results) explains every part of the report in detail.

> [!TIP]
> Start with the critical section. An exposed `.env` or `wp-config.php` backup means your secrets are already public: fix access first, then rotate the passwords and keys.

## 4. Save a report

The terminal view is for you. To share findings with a client, a teammate or an issue tracker, save a Markdown report:

```sh
rhythm tuner scan yoursite.com -o report.md
```

Tuner picks the format from the file extension and still shows the live view in your terminal. When it's done it prints `Report written to report.md`. The report has a summary table, the plugins and themes found, and a section per finding with the evidence, advisories, impact and the full fix.

Other formats work the same way: `-o report.json`, `-o results.jsonl`, `-o tuner.sarif`. See [Output and reports](Tuner-Output).

## 5. Try the quick profile

For a fast first look, the quick profile runs only the highest-value checks:

```sh
rhythm tuner scan yoursite.com --profile quick
```

On the same test site it ran 21 checks in 16 requests and 3 seconds, against 37 checks and 78 requests for the default profile. It still found the exposed `.env`, the debug log and the vulnerable versions, but skipped checks such as the `wp-config.php` backup search.

Quick scans don't read plugin `readme.txt` or theme `style.css` files, so plugin and theme versions come only from hints in the page's asset URLs. Use the default profile for an accurate inventory. See [Choosing checks](Tuner-Choosing-Checks).

## 6. Scan a staging site behind basic auth

Staging sites are often protected with HTTP basic auth. Send the credentials as a header:

```sh
rhythm tuner scan https://staging.yoursite.com \
  -H "Authorization: Basic $(printf 'user:pass' | base64)"
```

`-H` can be repeated and is sent with every request. If staging uses a self-signed certificate, add `-k` (`--insecure`); Tuner still reports the untrusted certificate as a finding. To scan as a logged-in WordPress user instead, pass the login cookie with `--cookie`. See [Requests and politeness](Tuner-Requests).

## 7. What to do next

1. **Fix from the top down.** Critical and high first; [Reading results](Tuner-Reading-Results) has a prioritizing guide.
2. **Rescan** after each fix to confirm it's gone and watch the score climb.
3. **Scan on the server** if you can log in to it. A local scan reads exact versions and finds plugins the home page doesn't load: `rhythm tuner scan /var/www/yoursite`. See [Local scans](Tuner-Local-Scans).
4. **Scan all your sites** from a list: [Batch scans](Tuner-Batch-Scans).
5. **Automate it** in CI or on a schedule: [CI and code scanning](Tuner-CI).

---
<sub>← [Tuner overview](Tuner) · [Home](Home) · [Reading results](Tuner-Reading-Results) →</sub>
