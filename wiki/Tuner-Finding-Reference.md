Every finding ID Tuner 0.4.0 can produce, with its default severity and what it means. Use these IDs with `--id`, `--exclude-id`, `.tuner.yml` ignore entries and baselines. Each ID has its own heading, so `#leak-dotenv` links straight to it. SARIF reports and `rhythm tuner explain` link here the same way.

There are three kinds:

- **[Remote checks](#remote-checks)**: the YAML checks a scan runs against a URL. They have a profile and `rhythm tuner checks` lists them.
- **[Vulnerability and version findings](#vulnerability-and-version-findings)**: from matching detected versions against advisory data. They appear in remote and local scans.
- **[Local findings](#local-findings)**: from scanning a site's folder on the server, including [integrity and malware](Tuner-Integrity-And-Malware) findings.

IDs written with `<…>` are patterns: `vuln-plugin-<slug>` becomes `vuln-plugin-contact-form-7`. Select a whole family with a glob, e.g. `--id 'vuln-plugin-*'`. A *likely* severity means the finding has low confidence: it counts half in the [Tune score](Tuner-Reading-Results#the-tune-score).

> [!TIP]
> `rhythm tuner explain <id>` prints an entry's full text offline: severity, impact, the whole fix with nginx and Apache snippets and references. For a vulnerability or local ID it prints the rule's title, severity and tags and links back here.

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

References
  https://owasp.org/Top10/A05_2021-Security_Misconfiguration/

More
  https://github.com/RhythmShahriar/rhythm/wiki/Tuner-Finding-Reference#leak-dotenv
```

## Remote checks

77 checks, generated from `rhythm tuner checks --json` and `rhythm tuner explain --json` on Tuner 0.4.0, matching the check files in [`checks/`](https://github.com/RhythmShahriar/tuner/tree/master/checks). Every check sends only GET or HEAD requests.

Each entry gives the severity, the cheapest [profile](Tuner-Choosing-Checks#profiles) that runs it (`quick` checks run in every profile, `default` checks in `default`), the platform it needs when it only runs on one and whether `rhythm tuner fix` has a **server snippet** for it. The text is the finding's title and impact.

### Headers and cookies

#### hdr-content-type-options-missing

**low** · `quick` · server snippet · X-Content-Type-Options header missing. Browsers may MIME-sniff uploads and run them as scripts.

#### hdr-cookie-no-httponly

**medium** · `quick` · Session cookie readable by JavaScript. Any XSS bug can steal the session cookie and take over the account.

#### hdr-cookie-no-secure

**low** · `quick` · Cookie set without Secure flag. The cookie is also sent over plain HTTP, where it can be intercepted.

#### hdr-csp-missing

**low** · `quick` · server snippet · Content-Security-Policy header missing. Without a CSP, an XSS bug can load scripts from anywhere.

#### hdr-frame-options-missing

**low** · `quick` · server snippet · Clickjacking protection missing. Pages can be framed by other sites, which allows clickjacking.

#### hdr-hsts-missing

**medium** · `quick` · server snippet · HSTS header missing. Visitors can be downgraded to plain HTTP on hostile networks.

#### hdr-powered-by-version

**low** · `quick` · server snippet · X-Powered-By leaks version. Reveals the exact PHP version, making known vulnerabilities easy to target.

#### hdr-referrer-policy-missing

**info** · `quick` · server snippet · Referrer-Policy header missing. Full URLs, including query strings, may leak to third-party sites.

#### hdr-server-version

**low** · `quick` · server snippet · Server header leaks version. Reveals the exact web server version, making known vulnerabilities easy to target.

### Exposed files

#### leak-archive-backup

**critical** · `default` · server snippet · Site backup archive exposed. A full copy of the site, usually including configuration and secrets, can be downloaded.

#### leak-bzr

**medium** · `default` · server snippet · .bzr (Bazaar) directory exposed. The repository and its history can be downloaded.

#### leak-composer-auth

**critical** · `default` · server snippet · Composer auth.json exposed. Private package registry tokens are readable.

#### leak-composer-manifest

**low** · `default` · server snippet · Composer manifest exposed. Exact package versions are visible, which makes known vulnerabilities easy to target.

#### leak-dotenv

**critical** · `quick` · server snippet · .env file exposed. Anyone can read database passwords, API keys and app secrets.

#### leak-ds-store

**low** · `default` · server snippet · .DS_Store file exposed. macOS folder metadata lists file names in the directory, including hidden backups.

#### leak-ds-store-subdir

**low** · `default` · server snippet · .DS_Store file exposed in a subfolder. macOS folder metadata lists file names in the folder, including hidden backups and plugin names.

#### leak-env-example

**high** · `default` · server snippet · Example .env file with real secrets. The example file was filled with real keys, which are now public. Example files are usually not covered by dotfile rules that list .env only.

#### leak-error-log

**medium** · `default` · server snippet · PHP error log exposed. Error logs reveal server paths, plugin names and sometimes credentials.

#### leak-git

**high** · `quick` · server snippet · .git directory exposed. The full source code and history can be downloaded, often including old credentials.

#### leak-git-index

**high** · `default` · server snippet · .git index exposed. The index lists every file in the repository. With the rest of .git the full source and history can be rebuilt.

#### leak-git-logs

**high** · `default` · server snippet · .git logs or HEAD files exposed. The files confirm a readable .git folder and show commit IDs, authors and remote URLs, which may hold tokens.

#### leak-hg

**high** · `default` · server snippet · .hg (Mercurial) directory exposed. The repository can be downloaded with its full history, often including old credentials.

#### leak-htpasswd

**high** · `default` · server snippet · .htpasswd file exposed. Password hashes for HTTP basic auth can be downloaded and cracked offline.

#### leak-idea

**medium** · `default` · server snippet · JetBrains .idea folder exposed. IDE project files list source paths, recent files, run settings and database connections.

#### leak-node-manifest

**low** · `default` · needs PHP · server snippet · Node package files exposed on a PHP site. The front-end build's packages and versions are public, which points at known flaws. The files also show the deploy copies the whole project.

#### leak-npm-debug

**low** · `default` · server snippet · npm or yarn debug log exposed. Build logs show local paths, package versions, environment details and sometimes registry tokens.

#### leak-sftp-config

**high** · `default` · server snippet · Editor SFTP settings exposed. Editor deploy settings name the server, user and remote path and often hold the FTP or SSH password.

#### leak-sql-dump

**critical** · `default` · server snippet · SQL database dump exposed. The whole database can be downloaded, including user emails and password hashes.

#### leak-staging-listing

**medium** · `default` · server snippet · Old or staging folder listed. Anyone can browse an old copy of the site, which often holds config files, database dumps and outdated plugins.

#### leak-svn

**high** · `default` · server snippet · .svn directory exposed. Subversion metadata exposes file names and can allow source code download.

#### leak-vendor-listing

**medium** · `default` · server snippet · Composer vendor folder listed. Every installed package is listed and its files can be fetched directly, including tests and tools never meant to run on a public server.

#### leak-wp-config

**critical** · `default` · server snippet · wp-config.php source or backup exposed. Database credentials and authentication salts are readable, which allows full site takeover.

#### leak-wp-config-copy

**critical** · `default` · server snippet · Other copy of wp-config.php exposed. Database credentials and authentication salts are readable, which allows full site takeover.

### PHP

#### php-adminer

**medium** · `default` · server snippet · Adminer database tool exposed. A public database login page invites password guessing against the database.

#### php-display-errors

**medium** · `quick` · PHP errors shown to visitors. display_errors is on, so error messages reveal server paths and code details to anyone.

#### php-ini-exposed

**low** · `default` · server snippet · PHP settings file exposed. PHP settings such as disabled functions, open_basedir and log paths are readable, which helps plan an attack. An auto_prepend_file line can point at injected code.

#### php-phpinfo

**medium** · `default` · server snippet · phpinfo() page exposed. Shows the PHP version, loaded modules, server paths and environment variables, which may include secrets.

#### php-phpinfo-extra

**medium** · `default` · server snippet · phpinfo() page exposed at another name. Shows the PHP version, loaded modules, server paths and environment variables, which may include secrets.

#### php-phpmyadmin

**medium** · `default` · server snippet · phpMyAdmin login page public. The database admin login is open to the internet, so anyone can guess passwords against it. Old phpMyAdmin versions have known flaws.

#### php-session-upload-progress

**medium**, *likely* · `default` · phpinfo() shows settings exposed to CVE-2026-115560. CVE-2026-115560 is an unpatched flaw in PHP's session upload progress. It is reachable when the serialize handler is php_binary or php_serialize, the application switches it to php with ini_set() before session_start() and upload progress is enabled. This page shows the two configuration conditions hold; whether the code switches the handler can only be checked on the server: run `rhythm tuner scan /path/to/site` there.

#### php-symfony-app-dev

**high** · `default` · server snippet · Symfony dev front controller public. The site runs in debug mode through this file, with the debug toolbar and profiler showing requests, queries and configuration.

#### php-symfony-profiler

**high** · `default` · server snippet · Symfony profiler public. The profiler shows every recent request with its headers, session, queries and environment, secrets included.

### Laravel

#### laravel-debugbar

**high** · `default` · server snippet · Laravel Debugbar request history public. Debugbar keeps recent requests with their queries, session data and headers. This endpoint hands them to anyone.

#### laravel-horizon

**medium** · `default` · Laravel Horizon dashboard public. Anyone can see queued jobs and their data and retry or delete failed jobs.

#### laravel-ignition

**high** · `quick` · server snippet · Laravel Ignition debug endpoints exposed. Debug mode is on in production. Older Ignition versions allow remote code execution (CVE-2021-3129).

#### laravel-log

**high** · `quick` · server snippet · Laravel log file exposed. Stack traces reveal source paths, queries and often credentials from exception context.

#### laravel-storage-listing

**high** · `default` · server snippet · Laravel storage folder listed. Logs, cached views and session files can be browsed. They hold errors, queries and session data.

#### laravel-telescope

**high** · `default` · Laravel Telescope dashboard public. Telescope shows every request, query, job, mail and exception, often with session data, tokens and personal data.

### Server pages

#### srv-apache-info

**medium** · `default` · server snippet · Apache server-info page public. The page shows the full server configuration, loaded modules and file paths.

#### srv-apache-status

**medium** · `default` · server snippet · Apache server-status page public. The page shows every request being served, with client IP addresses, URLs (query strings and tokens included) and virtual hosts.

#### srv-crossdomain

**low** · `default` · crossdomain.xml allows every domain. Flash or Silverlight clients on any other site may read this site with the visitor's cookies.

#### srv-elmah

**high** · `default` · server snippet · ELMAH error log public. The log shows every recent exception with its request, cookies and server variables, so sessions can be hijacked.

#### srv-nginx-status

**low** · `default` · server snippet · nginx status page public. Connection counts show the site's load, which helps time or measure a denial-of-service attack.

#### srv-web-config

**medium** · `default` · server snippet · IIS web.config exposed. The IIS config of a migrated site is readable, with rewrite rules and sometimes connection strings and keys.

### TLS and HTTPS

#### tls-cert-expired

**high** · `quick` · TLS certificate expired. Browsers show a full-page warning. Visitors who click through are open to interception.

#### tls-cert-expiring

**medium** · `quick` · TLS certificate expires soon. The site will show a browser security warning when the certificate expires.

#### tls-cert-untrusted

**high** · `quick` · TLS certificate not trusted. The certificate is self-signed, issued by an unknown authority, or doesn't match the hostname, so browsers warn visitors.

#### tls-legacy-protocol

**medium** · `default` · server snippet · Obsolete TLS 1.0/1.1 accepted. TLS 1.0 and 1.1 are deprecated (RFC 8996) and have known weaknesses.

#### tls-no-https

**medium** · `quick` · Site served over plain HTTP. Logins and cookies travel unencrypted. The site doesn't redirect to HTTPS.

### WordPress

#### wp-admin-ajax

**info** · `default` · needs WordPress · admin-ajax.php reachable. admin-ajax.php is public on every WordPress site and serves plugin actions, including ones for logged-out visitors (wp_ajax_nopriv_\*). Many plugin vulnerabilities live in these handlers. Tuner doesn't call them; known plugin CVEs are reported by the vulnerability checks.

#### wp-ai1wm-backups

**critical** · `default` · needs WordPress · server snippet · All-in-One WP Migration backups listed. Each .wpress file is a full copy of the site and its database, ready to download.

#### wp-backwpup-listed

**high** · `default` · needs WordPress · server snippet · BackWPup folders listed in uploads. The listing names the folders where BackWPup keeps backups and logs. If those list too, the whole site can be downloaded.

#### wp-debug-log

**high** · `quick` · server snippet · WordPress debug.log exposed. The log reveals server paths, plugin errors and sometimes queries with personal data.

#### wp-debug-log-moved

**high** · `default` · server snippet · WordPress debug log exposed in another folder. The log reveals server paths, plugin errors and sometimes queries with personal data.

#### wp-dir-listing

**medium** · `default` · needs WordPress · server snippet · Directory listing enabled. Anyone can browse every uploaded file and the exact plugins and themes installed.

#### wp-duplicator-archive

**critical** · `default` · needs WordPress · server snippet · Duplicator backup archives listed. The listing links to Duplicator packages, each a full copy of the site and its database.

#### wp-duplicator-installer

**high** · `default` · server snippet · Duplicator installer left on the site. The Duplicator installer can overwrite the site and its database. It also reveals paths and the name of the package archive.

#### wp-readme

**info** · `default` · needs WordPress · server snippet · WordPress readme.html present. Confirms WordPress and can hint at the installed version.

#### wp-readme-version

**low** · `default` · needs WordPress · server snippet · WordPress version shown in readme.html. The exact WordPress version is public, which points attackers at the flaws it has. Only old releases print it, so the core is likely out of date too.

#### wp-staging-copy

**medium** · `default` · server snippet · Old or staging WordPress copy reachable. A second WordPress install is live next to the site. Copies are rarely updated, so their core and plugins are easy targets. They often share the database password.

#### wp-updraft-backups

**critical** · `default` · needs WordPress · server snippet · UpdraftPlus backups listed. The listing links to full site backups, the database with password hashes and personal data included.

#### wp-uploads-php-listed

**high** · `default` · needs WordPress · server snippet · PHP file listed in uploads. Uploads should hold media only. A PHP file there is often a web shell left by an attacker. The open listing shows it to anyone.

#### wp-user-enum-rest

**medium** · `quick` · needs WordPress · User enumeration via REST API. Login names are public, which halves the work of a password-guessing attack.

#### wp-user-enum-rest-route

**medium** · `default` · needs WordPress · User enumeration via ?rest_route=. Login names are public, which halves the work of a password-guessing attack. Blocking /wp-json/ alone does not stop this form of the request.

#### wp-wc-logs

**high** · `default` · needs WordPress · server snippet · WooCommerce logs listed. WooCommerce logs can hold order details, customer emails and payment gateway responses.

#### wp-wpforms-uploads

**medium** · `default` · needs WordPress · server snippet · WPForms uploads listed. Files people sent through forms, such as CVs and ID scans, can be browsed by anyone.

#### wp-xmlrpc

**low** · `quick` · needs WordPress · server snippet · XML-RPC enabled. XML-RPC allows many password guesses in a single request and is often used for pingback abuse.

## Vulnerability and version findings

From matching detected versions against [vulnerability data](Tuner-Known-Vulnerabilities). They run alongside the checks in every profile and are skipped with `--no-vulns`. A `vuln-*` finding's severity is the worst severity among the advisories that apply (an unrated advisory counts as medium); the severity listed here is the default `explain` shows. `vuln-php`, `vuln-nginx`, `vuln-apache` and the `*-eol` findings drop to *likely* when the version string names a distribution build, which may include backported fixes.

#### vuln-core

**worst advisory** · WordPress core with known vulnerabilities. Published vulnerabilities affect the installed WordPress version.

#### wp-core-outdated

**low** · WordPress core out of date. A newer WordPress release exists. Reported only when `vuln-core` isn't.

#### vuln-plugin

**worst advisory** · `vuln-plugin-<slug>` · Plugin with known vulnerabilities. Published vulnerabilities affect the installed version of this plugin.

#### vuln-theme

**worst advisory** · `vuln-theme-<slug>` · Theme with known vulnerabilities. Published vulnerabilities affect the installed version of this theme.

#### wp-plugin-closed

**medium** · Plugin closed on wordpress.org, often for an unfixed security issue.

#### wp-theme-closed

**medium** · Theme closed on wordpress.org.

#### wp-components-outdated

**low** · Plugins or themes out of date. Newer releases exist on wordpress.org; one finding lists them all.

#### wp-components-abandoned

**low** · Plugins or themes that look abandoned: no release in over two years.

#### wp-components-unverified

**info**, *likely* · Plugins or themes with known vulnerabilities and an unknown version. Some versions are affected; the installed version couldn't be read.

#### vuln-php

**worst advisory** · PHP version with known vulnerabilities. On end-of-life branches it includes inferred flaws that will never be fixed there.

#### php-eol

**high** · PHP branch past end of life. It no longer gets security fixes.

#### php-version-hidden

**medium**, *likely* · PHP version hidden while recent critical flaws are out, so they can't be ruled out.

#### vuln-nginx

**worst advisory** · nginx version with known vulnerabilities.

#### vuln-apache

**worst advisory** · Apache version with known vulnerabilities.

#### nginx-eol

**medium** · nginx branch past end of life.

#### apache-eol

**medium** · Apache branch past end of life.

#### vuln-composer

**worst advisory** · `vuln-composer-<vendor>-<package>` · Composer package with known vulnerabilities, matched with OSV.dev. Local scans read `composer.lock`; remote scans match an exposed `composer.lock` or `vendor/composer/installed.json` (up to 500 packages) and say `(from exposed composer.lock)` in the title.

## Local findings

From `rhythm tuner scan <folder>`. See [Local scans](Tuner-Local-Scans) and [Integrity and malware](Tuner-Integrity-And-Malware). Every local finding carries the `local` tag, so `--tags local` or `--id 'local-*'` selects them. Their locations are `file://` URLs; SARIF gives paths relative to the scanned folder. A local ID that only a check pack's local rules produce has no entry of its own and links to this section.

### WordPress and Laravel configuration

#### local-wp-salts

**medium** · Keys and salts in `wp-config.php` are missing, the sample placeholder, empty or duplicated, so a database leak lets an attacker forge login cookies. The evidence names the line.

#### local-wp-debug

**medium** · `WP_DEBUG` is on and errors are shown to visitors, or logged to the public `wp-content/debug.log`. **low** when errors are hidden and logged elsewhere.

#### local-wp-file-edit

**low** · Neither `DISALLOW_FILE_EDIT` nor `DISALLOW_FILE_MODS` is true, so an admin account takeover becomes code execution.

#### local-wp-config-perms

**low** · `wp-config.php` is readable by every user on the server. Not checked on Windows.

#### local-wp-table-prefix

**info** · Tables use the default `wp_` prefix. Obscurity only, no score penalty.

#### local-wp-force-ssl-admin

**info** · `FORCE_SSL_ADMIN` isn't true.

#### local-wp-mu-plugins

**info** · Must-use plugins are installed; they always run and aren't matched against vulnerability data.

#### local-laravel-debug

**medium** · `APP_DEBUG=true` in a Laravel project's `.env`. The evidence names the line.

When `wp-config.php` includes other files, the findings about missing settings are *likely*: the setting may be defined there.

### Files that don't belong in a web root

#### local-dotenv

**high** (medium without secrets) · An environment file (`.env`, `.env.production`, `.ENV`…, not `.env.example`) is in the web root.

#### local-wp-config-backup

**high** · A copy of `wp-config.php` (`.bak`, `.old`, `.save`, `~`, `.swp`…) that the server would send as plain text.

#### local-sql-dump

**high** · A database dump (`*.sql`, `*.sql.gz`…) or SQLite file.

#### local-backup-archive

**high** · A backup-looking archive, a `.wpress` file, or any archive at the top of the web root.

#### local-composer-auth

**high** · Composer `auth.json` with tokens.

#### local-private-key

**high** · A private key (`*.pem`, `*.key`, `id_rsa`…).

#### local-vcs-dir

**medium** · A `.git`, `.svn` or `.hg` folder.

#### local-php-backup

**medium** · A backup copy of a PHP file, e.g. `functions.php~`.

#### local-log-file

**medium** · A log file such as `debug.log` or `error_log`.

#### local-phpinfo

**medium** · A PHP file that calls `phpinfo()`.

#### local-db-admin

**medium** · Adminer or phpMyAdmin.

### PHP

#### local-php-session-upload-progress

**high** to **info** · [CVE-2026-115560](Tuner-PHP-Advisories#session-upload-progress-cve-2026-115560) preconditions. High when all four conditions hold; high and *likely* when the configuration is unsafe but the order of calls couldn't be confirmed from one file; medium and *likely* when the code switches the serialize handler but PHP's configuration couldn't be read; low when the configured handler is `php`; info when upload progress is off everywhere checked.

### Integrity

From `--integrity`. See [Integrity and malware](Tuner-Integrity-And-Malware).

#### local-integrity-core-modified

**critical** · Modified WordPress core file: its MD5 differs from the one WordPress.org publishes for the installed release. **high** when the core file was replaced by a link that points outside the install or doesn't resolve.

#### local-integrity-core-unknown

**high** · Unexpected PHP file in WordPress core: a `.php` file in `wp-admin/` or `wp-includes/` that the release doesn't have.

#### local-integrity-core-missing

**low** · WordPress core files missing. One finding lists up to 20 paths.

#### local-integrity-plugin-modified

**high** · Modified file in a wordpress.org plugin: its SHA-256 differs from the plugin release's checksums.

#### local-integrity-plugin-unknown

**medium** · Unexpected PHP file in a wordpress.org plugin.

#### local-integrity-plugin-unchecked

**info** · Plugins not on wordpress.org weren't compared, because WordPress.org has no checksums for them. Premium and custom plugins land here.

#### local-integrity-plugin-unlisted

**medium**, *likely* · A wordpress.org plugin at a version wordpress.org never released, so its files weren't compared. A development build does this too; so does a `Version:` header edited to hide changed files.

### Malware signatures

Declarative rules that read file contents and never run them. See [Integrity and malware](Tuner-Integrity-And-Malware#malware-signatures) for what each one skips.

#### local-malware-php-in-uploads

**high**, *likely* · PHP file in the uploads folder (`.php`, `.phtml`, `.phar` and the like under `wp-content/uploads`). "Silence is golden" index files are exempt.

#### local-malware-eval-base64

**critical** · `eval()` of base64-decoded code.

#### local-malware-gzinflate-base64

**high** · Compressed and base64-encoded code (`gzinflate`, `gzuncompress` or `gzdecode` of `base64_decode`).

#### local-malware-assert-request

**critical** · `assert()` runs request input.

#### local-malware-preg-replace-e

**high** · `preg_replace()` with the `/e` modifier.

#### local-malware-create-function-input

**critical** · `create_function()` built from request input.

#### local-malware-webshell

**critical** · Known web shell (FilesMan, WSO, b374k, c99, r57 markers).

#### local-malware-prepend-uploads

**critical** · `auto_prepend_file` loads a file from uploads (in `.user.ini`, `.htaccess` or `php.ini`).

#### local-malware-mu-hidden-admin

**critical** · Must-use plugin creates a hidden admin user.

Local scans also report every [vulnerability and version finding](#vulnerability-and-version-findings) above, using versions read from disk.

---
<sub>← [Check packs](Tuner-Check-Packs) · [Home](Home) · [CLI reference](Tuner-CLI-Reference) →</sub>
