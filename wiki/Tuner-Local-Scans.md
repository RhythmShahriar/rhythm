A local scan reads a site's files in place, usually on the server itself, instead of sending requests to it. It reads exact versions, reviews `wp-config.php`, finds files that don't belong in a web root, looks for common malware, matches Composer packages and checks the preconditions for CVE-2026-115560. With `--integrity` it also compares core and plugin files with WordPress.org's checksums. This page covers when to use it, what it reads, every local finding and what it never does; [Integrity and malware](Tuner-Integrity-And-Malware) covers checksums, signatures and `--discover`.

```sh
rhythm tuner scan /var/www/yoursite
```

## When to use a local scan

From the outside, Tuner has to guess versions. Many servers hide PHP's. On the machine it can read them. Use a local scan when:

- **You can log in to the server** (SSH, a container shell, a CI job with the code checked out).
- **The PHP version is hidden** and you need to know whether the site is patched.
- **Plugins aren't detected remotely.** A remote scan sees plugins the home page loads or the REST API lists. A local scan reads every plugin folder.
- **You suspect a compromise.** `--integrity` finds changed core and plugin files; the malware signatures find common backdoors.
- **You want configuration review**: keys and salts, debug mode, file permissions, the dashboard file editor.
- **The site sits behind a firewall** or isn't public yet.
- **You made a staging copy with Roadie.** `rhythm tuner scan <staging dir>` covers the file-level leaks Roadie's reports list as not covered (see [Roadie: Security and limits](Roadie-Security-And-Limits#what-roadie-cant-see)).

It doesn't replace a remote scan. A local scan can't tell whether the web server actually serves a stray file. It doesn't check headers, cookies or TLS either. Run both when you can. See [Remote and local scans](Tuner#remote-and-local-scans).

## Running it

Point Tuner at the site's root folder, the one with `wp-config.php` (WordPress) or `artisan` (Laravel):

```sh
rhythm tuner scan /var/www/yoursite
rhythm tuner scan . --php /usr/bin/php8.3 --fail-on high
rhythm tuner scan ~/sites/yoursite -o report.md
```

A target is local when it is `.`, `..` or `~`, or starts with `./`, `../`, `/`, `~/`, `~\` or a drive letter such as `C:\`. A path that doesn't exist, or a file instead of a folder, is a usage error. A folder that is a link (a deploy's `current`, say) is resolved first, so the whole release is scanned. To scan every install under a folder, add `--discover`.

Run Tuner as a user that can read every file in the site, such as the site's own user. Files it can't read are skipped.

Output, `--fail-on`, `--baseline` and the other output flags work exactly as for remote scans. Finding URLs use `file://`; the terminal, Markdown and SARIF show paths relative to the folder you scanned.

### Example

A local scan of a test copy of a WordPress site, run where `php` is 8.3:

```text
$ rhythm tuner scan /var/www/shop --plain
rhythm tuner 0.4.0 - scanning /var/www/shop

Fingerprint: WordPress 6.9.3 · PHP 8.3.36 (from php on PATH)
Components: contact-form-7 5.3.1 · theme astra 4.6.0
[MEDIUM] WordPress keys and salts are not set - /wp-config.php
  evidence: AUTH_KEY is the sample placeholder or empty (line 6)
  fix: Generate fresh keys at https://api.wordpress.org/secret-key/1.1/salt/ and paste them into wp-config.php (or run: wp config shuffle-salts). Everyone is logged out once.
[MEDIUM] WordPress debug mode is on - /wp-config.php
  evidence: WP_DEBUG is true (line 5)
  fix: Set WP_DEBUG to false in production. If you need logs, set WP_DEBUG_DISPLAY to false and point WP_DEBUG_LOG at a file outside the web root.
[LOW] The dashboard file editor is enabled - /wp-config.php
  evidence: DISALLOW_FILE_EDIT is not set to true
  fix: Add define( 'DISALLOW_FILE_EDIT', true ); to wp-config.php.
[INFO] Database tables use the default wp_ prefix - /wp-config.php
  evidence: $table_prefix = 'wp_' (line 4)
  fix: Use a custom prefix on new installs. Changing it on a live site means renaming tables and options, so only do it with a backup.
[INFO] FORCE_SSL_ADMIN is not set - /wp-config.php
  evidence: FORCE_SSL_ADMIN is not true
  fix: Add define( 'FORCE_SSL_ADMIN', true ); to wp-config.php once the site has a certificate.
[LOW] wp-config.php is readable by every user on the server - /wp-config.php
  evidence: mode -rw-rw-r--
  fix: Restrict it to the site's user and the web server, e.g. chmod 640 wp-config.php (or 600 when PHP runs as the site's user).
[HIGH] .env file in the web root: 1 secret (DB_PASSWORD=Su•••••) - /.env
  secrets: DB_PASSWORD=Su•••••
  fix: Move it above the web root, or block dotfiles in the server config (nginx: location ~ /\. { deny all; }). Rotate any secrets in it if it was ever reachable.
[MEDIUM] Log file in the web root - /wp-content/debug.log
  evidence: /wp-content/debug.log (51 B)
  fix: Write logs outside the web root (WP_DEBUG_LOG or error_log in php.ini) and delete this one.
[CRITICAL] eval() of base64-decoded code - /wp-content/uploads/2026/10/cache.php
  evidence: /wp-content/uploads/2026/10/cache.php (43 B)
  fix: Treat the site as compromised. Restore the file from a clean copy (or delete it if it isn't yours) and change all passwords and keys. Then find how it got there.
[HIGH, likely] PHP file in the uploads folder - /wp-content/uploads/2026/10/cache.php
  likely: a few plugins keep PHP files in uploads on purpose
  evidence: /wp-content/uploads/2026/10/cache.php (43 B)
  fix: Check what the file does and when it appeared. If nobody added it on purpose, delete it, then block PHP in uploads (nginx: location ~* /wp-content/uploads/.*\.php$ { deny all; }) and look for how it got there.
[CRITICAL] WordPress 6.9.3: 22 known vulnerabilities - /
  - CRITICAL CVE-2026-63030 WordPress 6.9.x before 6.9.5 and 7.0.x before 7.0.2 is affected by a REST API batch endpo…
  - HIGH WordPress 6.9.x before 6.9.5 and 7.0.x before 7.0.2 is affected by a REST API batch endpo…
  - HIGH CVE-2026-93485 Improper Neutralization of Input During Web Page Generation ('Cross-site Scripting') vuln…
  - …and 19 more
  fix: Update WordPress to the latest release (6.9.4): Dashboard → Updates, or: wp core update
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - /wp-content/plugins/contact-form-7/
  - CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestricted File Upload (fixed in 5.3.2)
  - HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticated (Editor+) Arbitrary File Upload (fixed in 5.8.4)
  - MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cross-Site Scripting (fixed in 5.9.2)
  - …and 3 more
  fix: Update contact-form-7 to 6.0.6 or later (latest is 6.1.3): wp plugin update contact-form-7
[LOW] 1 plugins or themes out of date - /wp-content/
  evidence: astra 4.6.0 → 4.11.12
  fix: Update them (Dashboard → Updates, or: wp plugin update --all && wp theme update --all).

Findings by severity:
[CRITICAL] eval() of base64-decoded code - /wp-content/uploads/2026/10/cache.php
[CRITICAL] WordPress 6.9.3: 22 known vulnerabilities - /
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - /wp-content/plugins/contact-form-7/
[HIGH] .env file in the web root: 1 secret (DB_PASSWORD=Su•••••) - /.env
[HIGH, likely] PHP file in the uploads folder - /wp-content/uploads/2026/10/cache.php
[MEDIUM] Log file in the web root - /wp-content/debug.log
[MEDIUM] WordPress debug mode is on - /wp-config.php
[MEDIUM] WordPress keys and salts are not set - /wp-config.php
[LOW] wp-config.php is readable by every user on the server - /wp-config.php
[LOW] The dashboard file editor is enabled - /wp-config.php
[LOW] 1 plugins or themes out of date - /wp-content/
[INFO] FORCE_SSL_ADMIN is not set - /wp-config.php
[INFO] Database tables use the default wp_ prefix - /wp-config.php

Score 0/100 - 3 critical, 2 high, 3 medium, 3 low, 2 info - 28 checks, 1.2s
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
```

There's no baseline step and no request count: a local scan sends nothing to the site.

## What it reads

| What | Where it comes from |
|---|---|
| PHP | `php -n -r 'echo PHP_VERSION;'`, using the `php` on `PATH` or the binary you pass with `--php`. Only asked when the folder looks like PHP code (a `.php` file near the top, `composer.json`, `artisan` or WordPress), unless you pass `--php`. The fingerprint names the source: `PHP 8.3.36 (from php on PATH)` |
| WordPress | `$wp_version` in `wp-includes/version.php` |
| Plugins | the `Version:` header of each plugin's main file, read the way WordPress reads it: the installed version, not readme.txt's Stable tag. Single-file plugins such as `hello.php` count too. |
| Themes | the `Version:` header of each theme's `style.css` |
| Must-use plugins | listed from `wp-content/mu-plugins`, with their name and version |
| Laravel | detected from an `artisan` file and a `public/` folder, or `laravel/framework` in `composer.lock` |
| Composer packages | `composer.lock` in the scanned folder, including `packages-dev` (branches such as `dev-main` are skipped) |

These versions go through the same known-vulnerability matching as remote scans (`vuln-core`, `vuln-plugin-<slug>`, `vuln-php`, `php-eol`, `wp-components-outdated` and the rest). See [Known vulnerabilities](Tuner-Known-Vulnerabilities).

**Composer packages** are matched against [OSV.dev](https://osv.dev/), which covers the GitHub Advisory Database and FriendsOfPHP. Each vulnerable package is one finding, `vuln-composer-<vendor>-<package>` (for example `vuln-composer-guzzlehttp-guzzle`), rated by its worst advisory, with the version that fixes all of them and a `composer update` command. Results are cached with the rest of the vulnerability data; `TUNER_OSV_API` points at a mirror.

**Must-use plugins** are listed as `local-wp-mu-plugins` (info) but not matched against vulnerability data: they're usually custom or host-provided code whose file names don't match wordpress.org slugs. Make sure each one is expected.

### Choosing the right PHP: --php

The command-line `php` can differ from the PHP your web server runs. PHP-FPM pools often use another version. Servers may also have several installed. Pass the one that serves the site:

```sh
rhythm tuner scan /var/www/yoursite --php /usr/bin/php8.3
```

If no `php` is found on `PATH`, Tuner says so and skips the PHP version check:

```text
Fingerprint: WordPress 6.9.3 · PHP (version unknown)
…
note: php not found on PATH, so the PHP version wasn't checked; pass --php /path/to/php
```

A folder that isn't PHP at all (a Next.js app, say) gets no PHP lookup and no warning.

Versions with a distribution suffix (for example `8.1.2-1ubuntu2.23`) get low confidence, as in remote scans, because distributions backport fixes. See [Reading results](Tuner-Reading-Results#confidence-and-likely-findings).

## wp-config.php review

Tuner reads `wp-config.php` as text and never runs it. Like WordPress, it also looks one folder up when the file isn't in the site root, unless that folder holds another WordPress install. If it finds none, it warns `wp-config.php not found, so its settings weren't checked`.

| ID | Severity | When |
|---|---|---|
| `local-wp-salts` | medium | Keys and salts are missing, left as the sample placeholder, empty, or duplicated. WordPress then falls back to salts stored in the database, so a database leak lets an attacker forge login cookies. |
| `local-wp-debug` | medium | `WP_DEBUG` is on and errors are shown to visitors, or logged to the public `wp-content/debug.log`. |
| | low | `WP_DEBUG` is on; errors are hidden and logged elsewhere. |
| `local-wp-file-edit` | low | Neither `DISALLOW_FILE_EDIT` nor `DISALLOW_FILE_MODS` is true, so an admin account takeover becomes code execution. Defense in depth, so low. |
| `local-wp-config-perms` | low | `wp-config.php` is readable by every user on the server. Not checked on Windows. |
| `local-wp-table-prefix` | info | The default `wp_` table prefix. Obscurity only, so no score penalty. |
| `local-wp-force-ssl-admin` | info | `FORCE_SSL_ADMIN` isn't true. Most sites redirect to HTTPS at the server anyway. |
| `local-wp-mu-plugins` | info | Must-use plugins are installed (listed in the evidence). |
| `local-laravel-debug` | medium | `APP_DEBUG=true` in a Laravel project's `.env`. |

Findings about a setting name its line in the evidence (`WP_DEBUG is true (line 5)`, `APP_DEBUG=true (line 2)`). SARIF and JSON carry the line too.

How settings are read:

- Commented-out `define()` lines don't count. The first definition of a constant wins, as in PHP.
- Keys and salts set from code, such as `getenv()`, aren't judged.
- The other settings count only when set to a literal value. A constant computed in code, such as `define( 'DISALLOW_FILE_EDIT', getenv( 'WP_NO_EDIT' ) )`, is treated as not set.
- When `wp-config.php` includes other PHP files (besides `wp-settings.php`), a setting may be defined there. Findings about **missing** settings are then marked likely, with the evidence line `wp-config.php includes other files, which may set this`.

## Files that don't belong in a web root

Tuner walks the web root looking for stray files. A local scan can't tell whether the server actually serves a file (a remote scan confirms that), so most are rated one level below their remote counterparts: an `.env` confirmed downloadable is critical; one sitting in the web root is high. PHP tools like phpinfo and Adminer keep their remote severity, since in a web root they almost always run.

| ID | Severity | What |
|---|---|---|
| `local-dotenv` | high (medium without secrets) | `.env`, `.env.production`, `.ENV`… in any case. Templates (`.env.example`, `.env.sample`, `.env.dist`, `.env.template`, `.env.defaults`) are ignored. |
| `local-wp-config-backup` | high | `wp-config.php.bak`, `.old`, `.save`, `~`, `.swp`… which a server sends as plain text |
| `local-sql-dump` | high | `*.sql`, `*.sql.gz` and other compressed dumps, `.mysql`, `.pgsql`, SQLite files |
| `local-backup-archive` | high | backup-looking archives (`backup`, `site`, `db`, a year…), `.wpress`, `wp-content.zip` and any archive at the top level |
| `local-composer-auth` | high | Composer `auth.json` with tokens |
| `local-private-key` | high | `*.pem`, `*.key`, `id_rsa`, `id_ecdsa`, `id_ed25519` that contain a private key |
| `local-vcs-dir` | medium | `.git`, `.svn`, `.hg` folders |
| `local-php-backup` | medium | backup copies of PHP files, such as `functions.php~` or `index.php.bak` |
| `local-log-file` | medium | `debug.log`, `error_log`, `error.log`, `php_errors.log` |
| `local-phpinfo` | medium | a small PHP file that calls `phpinfo()`, at the top of the web root or named like `info.php` |
| `local-db-admin` | medium | Adminer (`adminer*.php`) or a phpMyAdmin folder |

Secrets found in these files are shown masked, as everywhere else. The title names them (`.env file in the web root: 1 secret (DB_PASSWORD=Su•••••)`). Archives and SQL files shipped inside plugin and theme folders aren't reported.

**Laravel.** For a Laravel project (an `artisan` file and a `public/` folder), only `public/` is the web root, so the project's own `.env` and `.git` aren't reported as stray files. The `.env` is still read for `local-laravel-debug`.

**How the walk works.** One pass over the project, reading files in parallel; each file is opened at most once and only when a check looks inside it. The stray-file checks skip `node_modules`, `vendor` and `bower_components` (Composer packages are covered by `composer.lock`), `wp-content/cache` and the media folders in `wp-content/uploads/<year>`; the [malware signatures](Tuner-Integrity-And-Malware#malware-signatures) read the media folders too. It doesn't follow links. Named pipes, sockets and devices are never opened: each gets a warning such as `skipped /wp-config.php: it is a named pipe, not a regular file`. After 250,000 entries it stops with `stopped after 250,000 entries, so the file checks are incomplete; scan a smaller folder to cover the rest`. Files it can't read are counted in a warning.

## Session upload progress (CVE-2026-115560)

CVE-2026-115560 is an unpatched flaw in PHP's session upload progress. PHP says it's only reachable when **all four** of these hold:

1. `session.serialize_handler` is `php_binary` or `php_serialize`;
2. the application switches it to `php` at runtime with `ini_set()`;
3. it does so before `session_start()`;
4. `session.upload_progress.enabled` is on (the default).

A local scan checks each condition and shows `✓` (met), `✗` (not met) or `?` (unknown) with the file or setting behind it. The finding is `local-php-session-upload-progress`:

```text
✗ session.serialize_handler is php in every configuration checked (FPM pool www)
✓ switched to php at runtime with ini_set() (/inc/session.php:2)
✓ before session_start() (same file, line 3)
✓ session.upload_progress.enabled is on (default)
```

### Where the settings come from

- **What `php` itself reports**, by running it with the php.ini loaded (to see real values) but with `auto_prepend_file` and `auto_append_file` forced empty, so no site code runs.
- **The web server's own ini files and PHP-FPM pools**, because the CLI's settings often differ from the web server's:
  - Debian and Ubuntu: `/etc/php/<version>/fpm`, `apache2` and `cgi`, with their `conf.d` files and FPM `pool.d` pools
  - RHEL and Fedora: `/etc/php-fpm.d`
  - Alpine: `/etc/php*/php-fpm.d`
  - the official Docker images: `/usr/local/etc/php-fpm.d`
- **Overrides in the site**: `.user.ini` files (FPM and CGI) and `.htaccess` `php_value`/`php_flag` lines (mod_php).

Upload progress only exists for web requests, so when the web server's settings are found, the CLI's don't count. Without them (for example in the official Docker images, where both share one configuration) the CLI's settings stand in.

### Where the code comes from

Every PHP file in the site except `vendor/` and `node_modules/`. Only code between PHP tags counts: inline HTML, comments, heredocs and strings that mention the call are ignored, so a theme template with an apostrophe in its HTML can't hide the code after it. For each `ini_set()` (or `ini_alter()`) of `session.serialize_handler`, Tuner checks whether `session_start()` follows it in the same file.

### Results

| Result | Severity |
|---|---|
| all four conditions confirmed | high |
| configuration unsafe; the order of calls couldn't be confirmed from one file (or the value set is computed, not a literal) | high, *likely* |
| the code switches the handler; PHP's configuration couldn't be read | medium, *likely* |
| the code switches the handler; the configured handler is `php` | low |
| upload progress is off everywhere checked | info: mitigated |

Sites whose code never switches the handler get no finding: PHP says they aren't affected.

### Mitigation

Until PHP releases a fix, turn off upload progress in php.ini or the PHP-FPM pool, then restart PHP-FPM or the web server:

```ini
; php.ini
session.upload_progress.enabled = 0

; PHP-FPM pool, e.g. /etc/php/8.3/fpm/pool.d/www.conf
php_admin_flag[session.upload_progress.enabled] = off
```

Better still, remove the runtime `ini_set()` of `session.serialize_handler`, or call it with the configured handler. Rescan to confirm the result drops to info.

From outside, Tuner can see the two settings only when a `phpinfo()` page is exposed: the remote check `php-session-upload-progress` reports that at low confidence, alongside `php-phpinfo`. See [PHP advisories](Tuner-PHP-Advisories) for the wider picture.

## Selecting local findings

Local findings have IDs starting with `local-` and the tag `local`. The usual selection flags apply:

```sh
rhythm tuner scan /var/www/yoursite --id 'local-*'           # local checks only, no version matching
rhythm tuner scan /var/www/yoursite --exclude-id local-wp-table-prefix
rhythm tuner scan /var/www/yoursite --no-vulns               # skip vulnerability and Composer matching
```

The HTTP checks don't run in a local scan. Request flags (`--rate`, `-H`, `--cookie`, `-k`…) don't apply either. `--proxy` and `--offline` still apply to downloading vulnerability data.

## What it never does

> [!IMPORTANT]
> A local scan only reads. It never changes, moves or deletes the scanned files and never runs the site's code. `--integrity` and the malware signatures only read too.

- **It runs at most two commands**, both with the `php` binary you chose and fixed arguments, without a shell: `php -n -r 'echo PHP_VERSION;'` for the version (`-n` loads no php.ini, so no `auto_prepend_file` can run). The other is the session-settings query, run only when the site's code switches the session handler. It loads php.ini with `auto_prepend_file` and `auto_append_file` forced empty.
- **It sends no requests to the site.** The only network traffic is fetching vulnerability data (WPVulnerability, wordpress.org, php.net, OSV.dev) and, with `--integrity`, checksums from wordpress.org. `--offline` turns all of it off.
- **It reads at most 1 MB of any file** (8 KB for plugin and theme headers, 16 MB for `composer.lock`; `--integrity` hashes whole files), doesn't follow links, never opens pipes, sockets or devices and stops walking after 250,000 entries.
- **It masks the secrets it recognizes** in every output format and cleans file names of control and bidi characters before printing them.

---
<sub>← [Batch scans](Tuner-Batch-Scans) · [Home](Home) · [Integrity and malware](Tuner-Integrity-And-Malware) →</sub>
