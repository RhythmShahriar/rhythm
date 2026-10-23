A local scan reads a site's files in place, usually on the server itself, instead of sending requests to it. It reads exact versions, reviews `wp-config.php`, finds files that don't belong in a web root, matches Composer packages, and checks the preconditions for CVE-2026-115560. This page covers when to use it, what it reads, every local finding, and what it never does.

```sh
rhythm tuner scan /var/www/yoursite
```

## When to use a local scan

From the outside, Tuner has to guess versions, and many servers hide PHP's. On the machine it can read them. Use a local scan when:

- **You can log in to the server** (SSH, a container shell, a CI job with the code checked out).
- **The PHP version is hidden** and you need to know whether the site is patched.
- **Plugins aren't detected remotely.** A remote scan only sees plugins whose assets the home page loads. A local scan reads every plugin folder.
- **You want configuration review**: keys and salts, debug mode, file permissions, the dashboard file editor.
- **The site sits behind a firewall** or isn't public yet.

It doesn't replace a remote scan. A local scan can't tell whether the web server actually serves a stray file, and it doesn't check headers, cookies or TLS. Run both when you can. See [Remote and local scans](Tuner#remote-and-local-scans).

## Running it

Point Tuner at the site's root folder, the one with `wp-config.php` (WordPress) or `artisan` (Laravel):

```sh
rhythm tuner scan /var/www/yoursite
rhythm tuner scan . --php /usr/bin/php8.3 --fail-on high
rhythm tuner scan ~/sites/yoursite -o report.md
```

A target is local when it starts with `.`, `/` or `~`, or names an existing directory. A path that doesn't exist, or a file instead of a folder, is a usage error.

Run Tuner as a user that can read every file in the site, such as the site's own user. Files it can't read are skipped.

Output, `--fail-on`, `--baseline` and the other output flags work exactly as for remote scans. Finding URLs use `file://`, and paths are relative to the folder you scanned.

### Example

A real local scan of a test copy of a WordPress site, trimmed:

```text
  ♪ rhythm tuner 0.3.1                                   /var/www/yoursite
  ────────────────────────────────────────────────────────────────────────

  ▸ Fingerprint   WordPress 6.4.3 · PHP 8.1.2
  ▸ Components    contact-form-7 5.3.1 · theme astra 4.6.0

  CRITICAL  2 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   contact-form-7 5.3.1: 6 known vulnerabilities
 ▌             CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestri…
 ▌             → Update contact-form-7 to 6.0.6 or later (latest is 6.2.…

 ▌  CRITICAL   PHP 8.1.2: 55 known vulnerabilities
 ▌             ~ likely: a distro build may already include the fix
 ▌             CRITICAL CVE-2024-4577 When using Apache and PHP-CGI on W…
 ⋮

  HIGH  3 ────────────────────────────────────────────────────────────────

 ▌  HIGH       .env file in the web root                            /.env
 ▌             DB_PASSWORD=Su•••••
 ▌             → Move it above the web root, or block dotfiles in the se…
 ⋮

  MEDIUM  4 ──────────────────────────────────────────────────────────────

 ▌  MEDIUM     Log file in the web root             /wp-content/debug.log
 ▌             → Write logs outside the web root (WP_DEBUG_LOG or error_…

 ▌  MEDIUM     WordPress debug mode is on                  /wp-config.php
 ▌             → Set WP_DEBUG to false in production. If you need logs, …

 ▌  MEDIUM     WordPress keys and salts are not set        /wp-config.php
 ▌             → Generate fresh keys at https://api.wordpress.org/secret…
 ⋮

  LOW  3 ─────────────────────────────────────────────────────────────────

    ●  CVE-2026-115560: runtime handler switch found, not reachable now
    ●  wp-config.php is readable by every user on the server
    ●  The dashboard file editor is enabled                /wp-config.php

  INFO  3 ────────────────────────────────────────────────────────────────

    ·  FORCE_SSL_ADMIN is not set                          /wp-config.php
    ·  1 must-use plugin installed                 /wp-content/mu-plugins
    ·  Database tables use the default wp_ prefix          /wp-config.php

 ╭────────────────────────────────────────────────────────────────────╮
 │  TUNE SCORE                                               0 / 100  │
 │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
 │  ✖ 2 critical   ▲ 3 high   ■ 4 medium   ● 3 low   · 3 info         │
 ╰────────────────────────────────────────────────────────────────────╯
  19 checks · 700ms
```

There's no baseline step and no request count: a local scan sends nothing to the site.

## What it reads

| What | Where it comes from |
|---|---|
| PHP | `php -n -r 'echo PHP_VERSION;'`, using the `php` on `PATH` or the binary you pass with `--php` |
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

The command-line `php` can differ from the PHP your web server runs. PHP-FPM pools often use another version, and servers may have several installed. Pass the one that serves the site:

```sh
rhythm tuner scan /var/www/yoursite --php /usr/bin/php8.3
```

If no `php` is found on `PATH`, Tuner says so and skips the PHP version check:

```text
! php not found on PATH, so the PHP version wasn't checked; pass --php /path/to/php
```

Versions with a distribution suffix (for example `8.1.2-1ubuntu2.23`) get low confidence, as in remote scans, because distributions backport fixes. See [Reading results](Tuner-Reading-Results#confidence-and-likely-findings).

## wp-config.php review

Tuner reads `wp-config.php` as text and never runs it. Like WordPress, it also looks one folder up when the file isn't in the site root, unless that folder holds another WordPress install. If it finds none, it warns `wp-config.php not found, so its settings weren't checked`.

| ID | Severity | When |
|---|---|---|
| `local-wp-salts` | medium | Keys and salts are missing, left as the sample placeholder, empty, or duplicated. WordPress then falls back to salts stored in the database, so a database leak lets an attacker forge login cookies. |
| `local-wp-debug` | medium | `WP_DEBUG` is on and errors are shown to visitors, or logged to the public `wp-content/debug.log`. |
| | low | `WP_DEBUG` is on, but errors are hidden and logged elsewhere. |
| `local-wp-file-edit` | low | Neither `DISALLOW_FILE_EDIT` nor `DISALLOW_FILE_MODS` is true, so an admin account takeover becomes code execution. Defense in depth, so low. |
| `local-wp-config-perms` | low | `wp-config.php` is readable by every user on the server. Not checked on Windows. |
| `local-wp-table-prefix` | info | The default `wp_` table prefix. Obscurity only, so no score penalty. |
| `local-wp-force-ssl-admin` | info | `FORCE_SSL_ADMIN` isn't true. Most sites redirect to HTTPS at the server anyway. |
| `local-wp-mu-plugins` | info | Must-use plugins are installed (listed in the evidence). |
| `local-laravel-debug` | medium | `APP_DEBUG=true` in a Laravel project's `.env`. |

How settings are read:

- Commented-out `define()` lines don't count, and the first definition of a constant wins, as in PHP.
- Keys and salts set from code, such as `getenv()`, aren't judged.
- The other settings count only when set to a literal value. A constant computed in code, such as `define( 'DISALLOW_FILE_EDIT', getenv( 'WP_NO_EDIT' ) )`, is treated as not set.
- When `wp-config.php` includes other PHP files (besides `wp-settings.php`), a setting may be defined there. Findings about **missing** settings are then marked likely, with the evidence line `wp-config.php includes other files, which may set this`.

## Files that don't belong in a web root

Tuner walks the web root looking for stray files. A local scan can't tell whether the server actually serves a file (a remote scan confirms that), so most are rated one level below their remote counterparts: an `.env` confirmed downloadable is critical; one sitting in the web root is high. PHP tools like phpinfo and Adminer keep their remote severity, since in a web root they almost always run.

| ID | Severity | What |
|---|---|---|
| `local-dotenv` | high (medium without secrets) | `.env`, `.env.production`… Templates (`.env.example`, `.env.sample`, `.env.dist`, `.env.template`, `.env.defaults`) are ignored. |
| `local-wp-config-backup` | high | `wp-config.php.bak`, `.old`, `.save`, `~`, `.swp`… which a server sends as plain text |
| `local-sql-dump` | high | `*.sql`, `*.sql.gz` and other compressed dumps, `.mysql`, `.pgsql`, SQLite files |
| `local-backup-archive` | high | backup-looking archives (`backup`, `site`, `db`, a year…), `.wpress`, `wp-content.zip`, and any archive at the top level |
| `local-composer-auth` | high | Composer `auth.json` with tokens |
| `local-private-key` | high | `*.pem`, `*.key`, `id_rsa`, `id_ecdsa`, `id_ed25519` that contain a private key |
| `local-vcs-dir` | medium | `.git`, `.svn`, `.hg` folders |
| `local-php-backup` | medium | backup copies of PHP files, such as `functions.php~` or `index.php.bak` |
| `local-log-file` | medium | `debug.log`, `error_log`, `error.log`, `php_errors.log` |
| `local-phpinfo` | medium | a small PHP file that calls `phpinfo()`, at the top of the web root or named like `info.php` |
| `local-db-admin` | medium | Adminer (`adminer*.php`) or a phpMyAdmin folder |

Secrets found in these files are shown masked, as everywhere else. Archives and SQL files shipped inside plugin and theme folders aren't reported.

**Laravel.** For a Laravel project (an `artisan` file and a `public/` folder), only `public/` is the web root, so the project's own `.env` and `.git` aren't reported as stray files. The `.env` is still read for `local-laravel-debug`.

**What the walk skips:** `node_modules`, `vendor` and `bower_components` (Composer packages are covered by `composer.lock`), `wp-content/cache`, and the media folders in `wp-content/uploads/<year>`. It doesn't follow symlinks, and stops after 250,000 entries with a warning to scan a smaller folder.

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

Every PHP file in the site, **`vendor/` included**, since libraries manage sessions too. Comments and text inside strings are ignored, so a commented-out call or a string mentioning it doesn't count. For each `ini_set()` (or `ini_alter()`) of `session.serialize_handler`, Tuner checks whether `session_start()` follows it in the same file.

### Results

| Result | Severity |
|---|---|
| all four conditions confirmed | high |
| configuration unsafe, but the order of calls couldn't be confirmed from one file (or the value set is computed, not a literal) | high, *likely* |
| the code switches the handler, but PHP's configuration couldn't be read | medium, *likely* |
| the code switches the handler, but the configured handler is `php` | low |
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

From outside, Tuner can only see the two settings, and only when a `phpinfo()` page is exposed: the remote check `php-session-upload-progress` reports that at low confidence, alongside `php-phpinfo`. See [PHP advisories](Tuner-PHP-Advisories) for the wider picture.

## Selecting local findings

Local findings have IDs starting with `local-` and the tag `local`, and the usual selection flags apply:

```sh
rhythm tuner scan /var/www/yoursite --id 'local-*'           # local checks only, no version matching
rhythm tuner scan /var/www/yoursite --exclude-id local-wp-table-prefix
rhythm tuner scan /var/www/yoursite --no-vulns               # skip vulnerability and Composer matching
```

The HTTP checks don't run in a local scan, and request flags (`--rate`, `-H`, `--cookie`, `-k`…) don't apply. `--proxy` and `--offline` still apply to downloading vulnerability data.

## What it never does

> [!IMPORTANT]
> A local scan only reads. It never changes, moves or deletes the scanned files, and never runs the site's code.

- **It runs at most two commands**, both with the `php` binary you chose and fixed arguments, without a shell: `php -n -r 'echo PHP_VERSION;'` for the version (`-n` loads no php.ini, so no `auto_prepend_file` can run), and, only when the site's code switches the session handler, the session-settings query, which loads php.ini with `auto_prepend_file` and `auto_append_file` forced empty.
- **It sends no requests to the site.** The only network traffic is fetching vulnerability data (WPVulnerability, wordpress.org, php.net, OSV.dev), which `--offline` turns off.
- **It reads at most 1 MB of any file** (8 KB for plugin and theme headers, 16 MB for `composer.lock`), doesn't follow symlinks, and stops walking after 250,000 entries.
- **It masks every secret** it finds, in every output format.

---
<sub>← [Batch scans](Tuner-Batch-Scans) · [Home](Home) · [Known vulnerabilities](Tuner-Known-Vulnerabilities) →</sub>
