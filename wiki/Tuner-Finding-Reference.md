Every finding ID Tuner 0.3.1 can produce, with its default severity and a one-line meaning. Use these IDs with `--id`, `--exclude-id` and baselines. They also help you look up a finding you got in a report.

There are three kinds:

- **[Remote checks](#remote-checks)**: the YAML checks a scan runs against a URL. They have a profile and can be listed with `rhythm tuner checks`.
- **[Vulnerability and version findings](#vulnerability-and-version-findings)**: from matching detected versions against advisory data. They appear in remote and local scans.
- **[Local findings](#local-findings)**: from scanning a site's directory on the server.

IDs written with `<…>` are patterns: `vuln-plugin-<slug>` becomes `vuln-plugin-contact-form-7`. Select a whole family with a glob, e.g. `--id 'vuln-plugin-*'`. A *likely* severity means the finding has low confidence: it counts half in the [Tune score](Tuner-Reading-Results).

## Remote checks

37 checks, generated from `rhythm tuner checks --json --profile deep` and matching the 37 check files in [`checks/`](https://github.com/RhythmShahriar/tuner/tree/master/checks). A newer [check pack](Tuner-Check-Packs) may add more; run `rhythm tuner checks` for the set your scans use.

**Profile** is the cheapest profile that runs the check: `quick` checks run in every profile, `default` checks in `default` and `deep`. No current check is `deep`-only. Checks marked *requires WordPress* only run when the site is detected as WordPress.

### Headers and cookies

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `hdr-content-type-options-missing` | low | quick | No `X-Content-Type-Options: nosniff`, so browsers may MIME-sniff uploads and run them as scripts. |
| `hdr-cookie-no-httponly` | medium | quick | A session cookie lacks `HttpOnly`, so any XSS bug can steal it. |
| `hdr-cookie-no-secure` | low | quick | A cookie lacks `Secure`, so it's also sent over plain HTTP. |
| `hdr-csp-missing` | low | quick | No `Content-Security-Policy`, so an XSS bug can load scripts from anywhere. |
| `hdr-frame-options-missing` | low | quick | No clickjacking protection (`X-Frame-Options` or CSP `frame-ancestors`). |
| `hdr-hsts-missing` | medium | quick | No `Strict-Transport-Security`, so visitors can be downgraded to HTTP. |
| `hdr-powered-by-version` | low | quick | `X-Powered-By` reveals the exact PHP version. |
| `hdr-referrer-policy-missing` | info | quick | No `Referrer-Policy`, so full URLs may leak to third-party sites. |
| `hdr-server-version` | low | quick | The `Server` header reveals the exact web server version. |

### Exposed files

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `leak-archive-backup` | critical | default | A site backup archive (`backup.zip`, `<host>.tar.gz`…) can be downloaded. |
| `leak-composer-auth` | critical | default | Composer `auth.json` with private registry tokens is readable. |
| `leak-composer-manifest` | low | default | `composer.json`, `composer.lock` or `installed.json` reveals exact package versions. |
| `leak-dotenv` | critical | quick | A `.env` file with passwords, API keys and app secrets is readable. |
| `leak-ds-store` | low | default | A macOS `.DS_Store` file lists the folder's file names. |
| `leak-error-log` | medium | default | A PHP error log (`error_log`, `php_errors.log`…) is readable. |
| `leak-git` | high | quick | The `.git` directory is exposed, so source and history can be downloaded. |
| `leak-htpasswd` | high | default | An `.htpasswd` file with basic-auth password hashes is readable. |
| `leak-sql-dump` | critical | default | A SQL database dump can be downloaded. |
| `leak-svn` | high | default | Subversion metadata (`.svn`) is exposed. |
| `leak-wp-config` | critical | default | `wp-config.php` source or a backup copy is readable: database credentials and salts. |

### PHP

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `php-adminer` | medium | default | The Adminer database tool is publicly reachable. |
| `php-display-errors` | medium | quick | PHP error messages are shown to visitors (`display_errors` on). |
| `php-phpinfo` | medium | default | A `phpinfo()` page is exposed: version, modules, paths, environment. |
| `php-session-upload-progress` | medium, *likely* | default | An exposed `phpinfo()` page shows the two settings [CVE-2026-115560](Tuner-PHP-Advisories#session-upload-progress-cve-2026-115560) needs; confirm with a local scan. |

### Laravel

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `laravel-ignition` | high | quick | Laravel Ignition debug endpoints are exposed: debug mode is on in production. |
| `laravel-log` | high | quick | `storage/logs/laravel.log` is readable: stack traces and often credentials. |

### TLS and HTTPS

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `tls-cert-expired` | high | quick | The TLS certificate has expired. |
| `tls-cert-expiring` | medium | quick | The TLS certificate expires soon. |
| `tls-cert-untrusted` | high | quick | The certificate is self-signed, from an unknown authority, or doesn't match the hostname. |
| `tls-legacy-protocol` | medium | default | The server still accepts obsolete TLS 1.0 or 1.1. |
| `tls-no-https` | medium | quick | The site is served over plain HTTP without redirecting to HTTPS. |

### WordPress

| ID | Severity | Profile | Meaning |
|---|---|---|---|
| `wp-admin-ajax` | info | default | `admin-ajax.php` is reachable, as on every WordPress site; plugin handlers there are common attack surface. *Requires WordPress.* |
| `wp-debug-log` | high | quick | `wp-content/debug.log` is readable. |
| `wp-dir-listing` | medium | default | Directory listing is on for uploads, plugins, themes or `wp-includes`. *Requires WordPress.* |
| `wp-readme` | info | default | `readme.html` confirms WordPress and can hint at the version. *Requires WordPress.* |
| `wp-user-enum-rest` | medium | quick | The REST API lists user login names. *Requires WordPress.* |
| `wp-xmlrpc` | low | quick | XML-RPC is enabled, allowing many password guesses per request. *Requires WordPress.* |

## Vulnerability and version findings

From matching detected versions against [vulnerability data](Tuner-Known-Vulnerabilities). They run alongside the checks in every profile and are skipped with `--no-vulns`. The `vuln-*` severity is the worst severity among the advisories that apply (an unrated advisory counts as medium).

| ID | Severity | Meaning |
|---|---|---|
| `vuln-core` | worst advisory | Published vulnerabilities affect the installed WordPress version. |
| `vuln-plugin-<slug>` | worst advisory | Published vulnerabilities affect the installed version of this plugin. |
| `vuln-theme-<slug>` | worst advisory | Published vulnerabilities affect the installed version of this theme. |
| `vuln-php` | worst advisory | Published vulnerabilities affect the PHP version; on end-of-life branches it includes inferred, never-to-be-fixed flaws. |
| `vuln-nginx` | worst advisory | Published vulnerabilities affect the nginx version. |
| `vuln-apache` | worst advisory | Published vulnerabilities affect the Apache version. |
| `vuln-composer-<vendor>-<package>` | worst advisory | A Composer package in `composer.lock` has known vulnerabilities (OSV.dev). Local scans only. |
| `php-eol` | high | The PHP branch no longer gets security fixes. |
| `nginx-eol` | medium | The nginx branch no longer gets security fixes. |
| `apache-eol` | medium | The Apache branch no longer gets security fixes. |
| `php-version-hidden` | medium, *likely* | The site runs PHP but hides its version, so recent critical PHP flaws can't be ruled out. |
| `wp-core-outdated` | low | A newer WordPress release exists (reported only when `vuln-core` isn't). |
| `wp-components-outdated` | low | Plugins or themes have newer releases on wordpress.org (one finding lists them all). |
| `wp-plugin-closed` | medium | The plugin was closed on wordpress.org, often for an unfixed security issue. |
| `wp-theme-closed` | medium | The theme was closed on wordpress.org. |
| `wp-components-abandoned` | low | Plugins or themes with no release in over two years. |
| `wp-components-unverified` | info, *likely* | Plugins or themes have known vulnerabilities in some versions, though their installed versions couldn't be read. |

`vuln-php`, `vuln-nginx`, `vuln-apache` and the `*-eol` findings drop to *likely* when the version string names a distribution build, which may include backported fixes.

## Local findings

From `rhythm tuner scan <directory>`. See [Tuner Local Scans](Tuner-Local-Scans). Every local finding carries the `local` tag, so `--tags local` or `--id 'local-*'` selects them; their locations are `file://` URLs.

### WordPress and Laravel configuration

| ID | Severity | Meaning |
|---|---|---|
| `local-wp-salts` | medium | Keys and salts in `wp-config.php` are missing, the sample placeholder, empty or duplicated, so a database leak lets an attacker forge login cookies. |
| `local-wp-debug` | medium | `WP_DEBUG` is on and errors are shown to visitors, or logged to the public `wp-content/debug.log`. |
| | low | `WP_DEBUG` is on; errors are hidden and logged elsewhere. |
| `local-wp-file-edit` | low | `DISALLOW_FILE_EDIT` isn't true, so an admin account takeover becomes code execution. |
| `local-wp-config-perms` | low | `wp-config.php` is readable by every user on the server. |
| `local-wp-table-prefix` | info | Tables use the default `wp_` prefix. Obscurity only, no score penalty. |
| `local-wp-force-ssl-admin` | info | `FORCE_SSL_ADMIN` isn't true. |
| `local-wp-mu-plugins` | info | Must-use plugins are installed; they always run and aren't matched against vulnerability data. |
| `local-laravel-debug` | medium | `APP_DEBUG=true` in a Laravel project's `.env`. |

Settings Tuner can't see in `wp-config.php` itself (for example, when it includes other files) make the "missing setting" findings *likely*.

### Files that don't belong in a web root

| ID | Severity | Meaning |
|---|---|---|
| `local-dotenv` | high (medium without secrets) | An environment file (`.env`, `.env.production`…, not `.env.example`) is in the web root. |
| `local-wp-config-backup` | high | A copy of `wp-config.php` (`.bak`, `.old`, `.save`, `~`, `.swp`…) that the server would send as plain text. |
| `local-sql-dump` | high | A database dump (`*.sql`, `*.sql.gz`…) or SQLite file. |
| `local-backup-archive` | high | A backup-looking archive, a `.wpress` file, or any archive at the top of the web root. |
| `local-composer-auth` | high | Composer `auth.json` with tokens. |
| `local-private-key` | high | A private key (`*.pem`, `*.key`, `id_rsa`…). |
| `local-vcs-dir` | medium | A `.git`, `.svn` or `.hg` folder. |
| `local-php-backup` | medium | A backup copy of a PHP file, e.g. `functions.php~`. |
| `local-log-file` | medium | A log file such as `debug.log` or `error_log`. |
| `local-phpinfo` | medium | A PHP file that calls `phpinfo()`. |
| `local-db-admin` | medium | Adminer or phpMyAdmin. |

### PHP

| ID | Severity | Meaning |
|---|---|---|
| `local-php-session-upload-progress` | high | All four [CVE-2026-115560](Tuner-PHP-Advisories#session-upload-progress-cve-2026-115560) conditions hold. |
| | high, *likely* | The configuration is unsafe; the order of calls couldn't be confirmed from one file. |
| | medium, *likely* | The code switches the serialize handler; PHP's configuration couldn't be read. |
| | low | The code switches the handler. The configured handler is `php`, so it isn't reachable now. |
| | info | Upload progress is off everywhere checked: mitigated. |

Local scans also report every [vulnerability and version finding](#vulnerability-and-version-findings) above, using versions read from disk.

---
<sub>← [Check packs](Tuner-Check-Packs) · [Home](Home) · [CLI reference](Tuner-CLI-Reference) →</sub>
