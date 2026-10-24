Tuner is a security scanner for WordPress and PHP sites. It finds leaked secrets, exposed files, known vulnerabilities and weak configuration. It gives you the fix for each one. This page explains what it looks for, how a scan works and when to scan from outside or on the server itself.

```sh
rhythm tuner scan yoursite.com
```

Tuner ships as its own binary, `rhythm-tuner`. You normally run it through `rhythm tuner` (or the short alias `rhythm t`). The installer sets up both; if you see `rhythm: tuner isn't installed`, run `rhythm install tuner`. See [Install](Install).

> [!IMPORTANT]
> Scan only sites you own or have permission to test. Tuner sends plain, rate-limited GET requests and never sends exploit payloads. Even so, scanning someone else's site without permission may be illegal. See [Responsible use](Responsible-Use).

## What it finds

| Area | What Tuner looks for |
|---|---|
| 🔑 **Leaks** | `.env` files, `.git` and `.svn` directories, `wp-config.php` backups, SQL dumps, backup archives, error and debug logs, `phpinfo()` pages, `.htpasswd`, Composer auth tokens. Secrets it finds are detected and **always masked**. |
| 🧩 **Known vulnerabilities** | WordPress core, plugins, themes, PHP, nginx and Apache, matched against CVE, Wordfence, WPScan and Patchstack data. Each finding lists the CVEs and the fixed version. It flags anything **actively exploited** (CISA KEV). |
| 🪶 **WordPress & PHP** | User enumeration, XML-RPC, admin-ajax, directory listing, `display_errors`, Laravel logs and Ignition. Outdated, closed and abandoned plugins and end-of-life PHP. |
| 🛡️ **Server & TLS** | Security headers, cookie flags, version leaks, plain HTTP, expired or untrusted certificates and obsolete TLS 1.0/1.1. |

Every finding comes with a severity, the evidence that triggered it and a fix: usually the exact nginx, Apache or WordPress change. The full list of finding IDs is in the [Finding reference](Tuner-Finding-Reference).

## How a remote scan works

A scan of a URL runs these steps. A typical default scan takes 15 to 30 seconds.

1. **Home page.** Tuner fetches the home page and follows redirects. If you typed a bare hostname, it tries `https://` first and falls back to `http://` if that fails. It keeps the scheme and host it was redirected to (http to https, apex to www) but the path you gave, so a scan of `yoursite.com/blog` stays under `/blog`.
2. **TLS.** For https sites it reads the certificate's expiry and trust and tests whether the server still accepts TLS 1.0 or 1.1. (The TLS 1.0/1.1 test is skipped when requests go through a proxy.)
3. **Fingerprint.** From the home page it detects WordPress (with version), Laravel, PHP, the server and the plugins and themes the page loads. Plugin versions then come from each plugin's `readme.txt`, theme versions from `style.css` and the WordPress version from `/feed/` when the page doesn't state it.
4. **PHP version probe.** The PHP version comes from `X-Powered-By`, or a `PHP/x.y.z` token in `Server`. When a page cache strips both from the home page, Tuner asks a few uncacheable PHP endpoints instead (`/xmlrpc.php`, `/wp-admin/admin-ajax.php`, `/wp-login.php` on WordPress, `/index.php` elsewhere) and stops at the first that answers. That's at most three extra requests in every profile. The checks reuse the responses.
5. **Soft-404 baseline.** Tuner requests a random path to learn how the server answers "not found". Before trusting a hit on a file like `/.env`, a check compares it with a random path of the same shape (same folder, same extension, dotfile or not). Sites that return `200 OK` for everything then don't flood you with false findings.
6. **Checks and vulnerability matching, together.** Each check's requests run without following redirects. Vulnerability matching runs alongside them, comparing the versions found against public advisories. Checks that need a platform (for example `requires: wordpress`) only run when the fingerprint found it. Findings stream in as they're found.

At the end you get the findings grouped by severity and a **Tune score** out of 100. [Reading results](Tuner-Reading-Results) walks through the report.

Tuner fetches each URL at most once per scan and shares responses between the steps, so the PHP probe and the checks don't repeat each other's requests.

## Remote and local scans

Tuner has two modes. Point it at a URL and it scans from the outside, like a visitor. Point it at a folder and it scans the site's files in place, usually on the server.

```sh
rhythm tuner scan yoursite.com            # remote: over HTTP
rhythm tuner scan /var/www/yoursite       # local: reads the files
```

| | Remote scan | Local scan |
|---|---|---|
| **Runs from** | anywhere that can reach the site | the server, or any copy of the site's files |
| **Sees** | what the public can reach | everything in the site folder |
| **Versions** | guessed from headers, readme.txt and style.css; PHP may be hidden | read exactly: `php`, `wp-includes/version.php`, plugin headers, `composer.lock` |
| **Confirms exposure** | yes: a file is only reported if the server serves it | no: a stray file is reported because it's in the web root |
| **Also checks** | headers, cookies, TLS, user enumeration, XML-RPC | `wp-config.php` settings, file permissions, Composer packages, the CVE-2026-115560 session preconditions |
| **Sends requests to the site** | yes, rate limited | none |

Use a **remote scan** to see what an attacker sees, for sites you don't have shell access to, for agencies checking many client sites and in CI against staging.

Use a **local scan** when you can log in to the server: it reads exact versions even when the site hides them, finds plugins that the home page doesn't load and reviews configuration you can't see from outside. The two complement each other; many teams run both. See [Local scans](Tuner-Local-Scans).

A target is local when it starts with `.`, `/` or `~`, or names an existing directory. Everything else is treated as a URL or hostname.

## Data and updates

Vulnerability data comes from [WPVulnerability](https://www.wpvulnerability.com/) (CC0), which aggregates CVE, Wordfence, WPScan and Patchstack, plus the wordpress.org API and php.net. Composer packages in local scans are matched with [OSV.dev](https://osv.dev/). No API key is needed. Data is cached and refreshed automatically once a day. Tuner works offline from the cache. See [Known vulnerabilities](Tuner-Known-Vulnerabilities).

New checks arrive as a checksum-verified *check pack*, without a new binary. See [Check packs](Tuner-Check-Packs).

## Where to go next

| You want to… | Read |
|---|---|
| Run your first scan, step by step | [Your first scan](Tuner-First-Scan) |
| Understand severities, "likely" findings and the score | [Reading results](Tuner-Reading-Results) |
| Run fewer or more checks | [Choosing checks](Tuner-Choosing-Checks) |
| Tune speed, headers, cookies, proxies | [Requests and politeness](Tuner-Requests) |
| Save JSON, SARIF or Markdown reports | [Output and reports](Tuner-Output) |
| Scan many sites | [Batch scans](Tuner-Batch-Scans) |
| Scan on the server | [Local scans](Tuner-Local-Scans) |
| Wire it into CI | [CI and code scanning](Tuner-CI) |
| Look up a flag | [CLI reference](Tuner-CLI-Reference) |
| Fix a problem | [FAQ and troubleshooting](Tuner-FAQ) |

---
<sub>← [Verifying releases](Verifying-Releases) · [Home](Home) · [Your first scan](Tuner-First-Scan) →</sub>
