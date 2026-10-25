Tuner is a security scanner for WordPress and PHP sites. It finds leaked secrets, exposed files, known vulnerabilities, weak configuration and, on the server, changed core files and malware. It gives you the fix for each one. This page explains what it looks for, how a scan works and when to scan from outside or on the server itself.

```sh
rhythm tuner scan yoursite.com
```

Tuner ships as its own binary, `rhythm-tuner`. You normally run it through `rhythm tuner` (or the short alias `rhythm t`). The installer sets up both; if you see `rhythm: tuner isn't installed`, run `rhythm install tuner`. See [Install](Install).

> [!IMPORTANT]
> Scan only sites you own or have permission to test. Tuner's checks send plain, rate-limited GET or HEAD requests and never send exploit payloads; a check that asks for any other method doesn't load. Even so, scanning someone else's site without permission may be illegal. See [Responsible use](Responsible-Use).

## What it finds

| Area | What Tuner looks for |
|---|---|
| 🔑 **Leaks** | `.env` files, `.git` and `.svn` directories, `wp-config.php` backups, SQL dumps, backup archives, error and debug logs, `phpinfo()` pages, `.htpasswd`, Composer auth tokens. Secrets Tuner recognizes are **masked** in every report. A value it doesn't recognize as a secret can still show in evidence, so treat reports as private. |
| 🧩 **Known vulnerabilities** | WordPress core, plugins, themes, PHP, nginx and Apache, matched against CVE, Wordfence, WPScan and Patchstack data, plus Composer packages through OSV.dev. Each finding lists the CVEs and the fixed version. It flags anything **actively exploited** (CISA KEV). |
| 🪶 **WordPress & PHP** | User enumeration, XML-RPC, admin-ajax, directory listing, `display_errors`, Laravel logs and Ignition. Outdated, closed and abandoned plugins and end-of-life PHP. |
| 🛡️ **Server & TLS** | Security headers, cookie flags, version leaks, server status pages, plain HTTP, expired or untrusted certificates and obsolete TLS 1.0/1.1. |
| 🧪 **On the server** | `wp-config.php` settings, files that don't belong in a web root, core and plugin files that differ from WordPress.org's checksums and common malware patterns. See [Local scans](Tuner-Local-Scans) and [Integrity and malware](Tuner-Integrity-And-Malware). |

Every finding comes with a severity, the evidence that triggered it and a fix: usually the exact nginx, Apache or WordPress change. `rhythm tuner explain <id>` prints a finding's full fix offline and `rhythm tuner fix` turns a report into one server config. The full list of finding IDs is in the [Finding reference](Tuner-Finding-Reference).

## How a remote scan works

A scan of a URL runs these steps. A typical default scan of a public site takes 15 to 30 seconds at the default 5 requests a second; your own server on a local or private address gets 50 a second and finishes in a few seconds.

1. **Home page.** Tuner fetches the home page and follows redirects. If you typed a bare hostname, it tries `https://` first and falls back to `http://` if that fails. It follows http to https and apex to www, keeping the path you gave, so a scan of `yoursite.com/blog` stays under `/blog`. A redirect to another host stops the scan unless you pass `--follow-host`. A firewall challenge or a failing home page marks the scan partial instead of reporting a clean site.
2. **TLS.** For https sites it reads the certificate's expiry and trust and tests whether the server still accepts TLS 1.0 or 1.1. (The TLS 1.0/1.1 test is skipped when requests go through a proxy.)
3. **Fingerprint.** From the home page it detects WordPress (with version), Laravel, PHP, the server and the plugins and themes the page loads. On WordPress it also reads the REST API index (`/wp-json/`, one request) to find plugins the page doesn't load; `--plugins deep` goes further. Plugin versions then come from each plugin's `readme.txt`, theme versions from `style.css` and the WordPress version from `/feed/` when the page doesn't state it.
4. **PHP version probe.** The PHP version comes from `X-Powered-By`, or a `PHP/x.y.z` token in `Server`. When a page cache strips both from the home page, Tuner asks a few uncacheable PHP endpoints instead (`/xmlrpc.php`, `/wp-admin/admin-ajax.php`, `/wp-login.php` on WordPress, `/index.php` elsewhere) and stops at the first that answers. That's at most three extra requests in every profile. The checks reuse the responses.
5. **Soft-404 baseline.** Tuner requests a random path to learn how the server answers "not found". Before trusting a hit on a file like `/.env`, a check compares it with a random path of the same shape (same folder, same extension, dotfile or not). Sites that return `200 OK` for everything then don't flood you with false findings.
6. **Checks and vulnerability matching, together.** Each check's requests run without following redirects. Vulnerability matching runs alongside them, comparing the versions found against public advisories; an exposed `composer.lock` is matched too. Checks that need a platform (for example `requires: wordpress`) only run when the fingerprint found it. Findings stream in as they're found.

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
| **Also checks** | headers, cookies, TLS, user enumeration, XML-RPC | `wp-config.php` settings, file permissions, Composer packages, core and plugin checksums (`--integrity`), malware signatures, the CVE-2026-115560 session preconditions |
| **Sends requests to the site** | yes, rate limited | none |

Use a **remote scan** to see what an attacker sees, for sites you don't have shell access to, for agencies checking many client sites and in CI against staging.

Use a **local scan** when you can log in to the server: it reads exact versions even when the site hides them, finds plugins that the home page doesn't load and reviews configuration you can't see from outside. The two complement each other; many teams run both. See [Local scans](Tuner-Local-Scans).

A target is local when it is `.`, `..` or `~`, or starts with `./`, `../`, `/`, `~/` or a drive letter such as `C:\`. A path such as `sites/shop` counts too, as long as its first part doesn't look like a host. Everything else is a URL or hostname. A bare name that is also a folder here is refused as ambiguous. See [scan](Tuner-CLI-Reference#scan).

## Data and updates

Vulnerability data comes from [WPVulnerability](https://www.wpvulnerability.com/) (CC0), which aggregates CVE, Wordfence, WPScan and Patchstack, plus the wordpress.org API and php.net. Composer packages are matched with [OSV.dev](https://osv.dev/). No API key is needed; a free Wordfence Intelligence key adds its feed. Data is cached and refreshed automatically once a day. Tuner works offline from the cache. See [Known vulnerabilities](Tuner-Known-Vulnerabilities).

Checks are built into the binary. Tuner can also take new checks from a signed *check pack* without a new binary. Tuner 0.4.0 doesn't trust a pack signing key yet, so it always uses its built-in checks and new checks arrive with new releases. See [Check packs](Tuner-Check-Packs).

## Where to go next

| You want to… | Read |
|---|---|
| Run your first scan, step by step | [Your first scan](Tuner-First-Scan) |
| Understand severities, "likely" findings and the score | [Reading results](Tuner-Reading-Results) |
| Run fewer or more checks | [Choosing checks](Tuner-Choosing-Checks) |
| Tune speed, headers, cookies, proxies | [Requests and politeness](Tuner-Requests) |
| Save JSON, SARIF, Markdown or HTML reports | [Output and reports](Tuner-Output) |
| Scan many sites | [Batch scans](Tuner-Batch-Scans) |
| Scan on the server | [Local scans](Tuner-Local-Scans) |
| Check core files and look for malware | [Integrity and malware](Tuner-Integrity-And-Malware) |
| See what changed and get notified | [Changes and notifications](Tuner-Changes-And-Notifications) |
| Wire it into CI | [CI and code scanning](Tuner-CI) and the [GitHub Action](Tuner-GitHub-Action) |
| Keep settings and accepted risk in the repository | [Config file](Tuner-Config) |
| Scan on a schedule or from your own app | [Watch and serve](Tuner-Watch-And-Serve) |
| Move over from Wordfence CLI or WPScan | [Coming from Wordfence CLI or WPScan](Tuner-Coming-From-Wordfence-Or-WPScan) |
| Look up a flag | [CLI reference](Tuner-CLI-Reference) |
| Fix a problem | [FAQ and troubleshooting](Tuner-FAQ) |

---
<sub>← [Extensions](Extensions) · [Home](Home) · [Your first scan](Tuner-First-Scan) →</sub>
