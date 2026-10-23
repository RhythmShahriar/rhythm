A living page for current PHP security advisories: what each one is, exactly how Tuner covers it, and what to do on your servers. It covers conditions and mitigations only, never how to exploit anything.

**Last reviewed: 2026-10-23**

| Advisory | Status | Tuner coverage |
|---|---|---|
| [PHP2Shell](#php2shell-cve-2026-109618-and-cve-2026-109619) (CVE-2026-109618, CVE-2026-109619) | fixed in 8.5.12, 8.4.27, 8.3.36, 8.2.35 | `vuln-php`, `php-version-hidden`, `php-eol` |
| [Session upload progress](#session-upload-progress-cve-2026-115560) (CVE-2026-115560) | **unpatched**, mitigation available | `local-php-session-upload-progress`, `php-session-upload-progress` |

> [!IMPORTANT]
> Scan only sites you own or are authorized to test. Every Tuner check here is read-only: version matching, plain GET requests, and reading files on your own server.

## How to check your servers

Run these on each server, or from a machine that can reach your sites:

1. **Refresh Tuner's data**, so it knows the latest advisories (it does this daily on its own):

   ```sh
   rhythm update
   ```

2. **Scan each site from the outside.** Version matching and the hidden-version warning cover PHP2Shell; the phpinfo check covers exposed CVE-2026-115560 settings:

   ```sh
   rhythm tuner scan yoursite.com --id 'vuln-php,php-*'
   ```

3. **Scan on the server.** A local scan reads the exact PHP version and checks every CVE-2026-115560 condition in your configuration and code:

   ```sh
   rhythm tuner scan /var/www/yoursite --id 'vuln-php,php-*,local-php-*'
   ```

   If several PHP versions are installed, point Tuner at the one your web server runs: `--php /usr/bin/php8.3`.

4. **Confirm the PHP version the web server uses**, not just the command line's:

   ```sh
   php -v
   php-fpm8.3 -v        # or the FPM binary your pool runs
   ```

   In WordPress: **Tools → Site Health → Info → Server**.

5. **Many sites?** Put them in a list and fail CI on anything high or worse:

   ```sh
   rhythm tuner scan -l sites.txt --id 'vuln-php,php-*' --fail-on high
   ```

See [Tuner Local Scans](Tuner-Local-Scans) and [Tuner Batch Scans](Tuner-Batch-Scans) for more.

## PHP2Shell (CVE-2026-109618 and CVE-2026-109619)

**What it is.** Two critical flaws in PHP itself (the engine, not an extension or framework), disclosed on 2026-10-20 and 2026-10-21 and known together as PHP2Shell. Both are rated critical (CVSS 9.8) in the advisory data Tuner uses. Every release before the fixed ones is affected.

| Branch | Fixed in | Status |
|---|---|---|
| 8.5 | 8.5.12 | supported |
| 8.4 | 8.4.27 | supported |
| 8.3 | 8.3.36 | supported (security fixes only) |
| 8.2 | 8.2.35 | supported (security fixes only) |
| 8.1 and older | **no fix** | end of life |

**Exploitation.** Reports of exploitation in the wild were disputed at the time of writing, and neither CVE was in CISA's Known Exploited Vulnerabilities catalog. If that changes, Tuner marks them `[actively exploited]` after its next daily data refresh, with no update needed.

### How Tuner covers it

Tuner matches the PHP version against [WPVulnerability](https://www.wpvulnerability.com/) data, which lists both CVEs for every supported branch. No Tuner release was needed: the data refresh brought them in. See [Tuner Known Vulnerabilities](Tuner-Known-Vulnerabilities) for how matching works.

| Situation | What Tuner reports |
|---|---|
| **Version visible** on the home page (`X-Powered-By`, or `PHP/x.y.z` in `Server`) | `vuln-php`, critical, listing both CVEs with the fixed release for your branch |
| **Version hidden on the home page by a page cache**, but sent by an uncached endpoint (`/xmlrpc.php`, `/wp-admin/admin-ajax.php`, `/wp-login.php`, or `/index.php` on other PHP sites) | the same `vuln-php`; the evidence says where the version came from, e.g. `version from x-powered-by /wp-login.php` |
| **Version hidden everywhere** | `php-version-hidden`, medium with low confidence: "PHP version hidden: can't rule out CVE-2026-109618 (critical) and N more", listing the fixed release on each supported branch |
| **End-of-life branch** (8.1 and older) | `php-eol` at high, and both CVEs added to `vuln-php` as `(no fix for this branch)`, inferred from the 8.2 advisories |
| **Distribution build** (e.g. `PHP/8.3.6-0ubuntu0.24.04.5`) | `vuln-php` with low confidence: the distribution may have backported the fix without changing the version number |
| **Local scan** | `vuln-php` from the exact version `php` reports |

Example for a site on PHP 8.4.26, from the data as of 2026-10-23:

```text
[CRITICAL] PHP 8.4.26: 2 known vulnerabilities - https://yoursite.com/
  - CRITICAL CVE-2026-109618 PHP uses longjmp() for out-of-memory (OOM) errors to exit the current context and abort h… (fixed in 8.4.27)
  - CRITICAL CVE-2026-109619 PHP has various global state that temporarily points into request memory. In some scenari… (fixed in 8.4.27)
  fix: Update PHP to 8.4.27 or later from your OS or vendor packages.
```

Tuner 0.3.0 or later is needed for the uncached-endpoint probe, `php-version-hidden` and end-of-life inference. Check with `rhythm tuner version`.

### What to do

1. **Upgrade PHP** to 8.5.12, 8.4.27, 8.3.36 or 8.2.35 or later, from your OS packages, your hosting panel, or php.net. Restart PHP-FPM or the web server afterwards so the new binary is the one serving requests.
2. **On 8.1 or older, move to a supported branch.** There is no fix for end-of-life branches, unless your vendor provides extended support and states that its build includes the fix.
3. **Check the version the web server runs** with `php -v` on the server (and the FPM binary), your hosting panel, or WordPress Site Health. The command-line PHP and the web server's PHP often differ.
4. **Use a local scan when the version is hidden.** `php-version-hidden` can't tell patched from unpatched; `rhythm tuner scan /var/www/yoursite` can.
5. **On distribution packages,** check your distribution's security tracker for both CVEs. Ubuntu, Debian, RHEL and others often fix them under the same upstream version number.

## Session upload progress (CVE-2026-115560)

**What it is.** A flaw in PHP's session upload progress feature. **There was no fixed PHP release at the time of writing.** It is reachable only when all four of these hold:

1. `session.serialize_handler` is `php_binary` or `php_serialize`;
2. the application switches it to `php` at runtime with `ini_set()`;
3. it does so before calling `session_start()`;
4. `session.upload_progress.enabled` is on. It's on by default.

Sites whose code never switches the serialize handler at runtime aren't affected, according to PHP. Most sites don't switch it, but libraries in `vendor/` sometimes manage sessions, so it's worth checking.

**PHP's interim mitigation.** Turn off upload progress where PHP is configured for the site, then restart PHP-FPM or the web server:

```ini
; php.ini
session.upload_progress.enabled = 0
```

```ini
; PHP-FPM pool, e.g. /etc/php/8.3/fpm/pool.d/www.conf
php_admin_flag[session.upload_progress.enabled] = off
```

Few sites rely on upload progress; it's used for upload progress bars that poll the session. Better still, remove the runtime `ini_set()` of `session.serialize_handler`, or have it set the handler that's already configured.

### How Tuner covers it

**On the server (`local-php-session-upload-progress`).** A [local scan](Tuner-Local-Scans) checks each condition and shows ✓, ✗ or ? with the file, setting or line behind it:

- **Configuration** comes from what `php` itself reports and, because the CLI's settings often differ from the web server's, from the web server's own ini files and PHP-FPM pools (Debian/Ubuntu `/etc/php/<v>/{fpm,apache2,cgi}`, `/etc/php-fpm.d`, Alpine `/etc/php*/php-fpm.d`, the official Docker images' `/usr/local/etc/php-fpm.d`), plus `.user.ini` and `.htaccess` overrides in the site. When the web server's settings are found, the CLI's don't count.
- **Code** is every PHP file in the site, `vendor/` included. Comments and text inside strings are ignored. Tuner reads the code; it never runs it.

| Result | Severity |
|---|---|
| all four conditions confirmed | high |
| configuration unsafe, but the order of calls couldn't be confirmed from one file | high, *likely* |
| the code switches the handler but PHP's configuration couldn't be read | medium, *likely* |
| the code switches the handler, but the configured handler is `php` | low |
| upload progress is off everywhere checked | info: mitigated |

No finding means the code never switches the handler. This check needs Tuner 0.3.1 or later.

**From outside (`php-session-upload-progress`).** A remote scan can only see the two configuration settings, and only when a `phpinfo()` page is exposed. The check reads the effective (local) values on common phpinfo paths and fires when the handler is `php_binary` or `php_serialize` and upload progress is on. It's medium with low confidence, reported alongside `php-phpinfo`, because whether the code switches the handler can only be checked on the server. It shipped in the [check pack](Tuner-Check-Packs), so Tuner 0.3.0 and later pick it up without an update.

**Once PHP ships a fix,** the advisory data will list the fixed releases, and `vuln-php` will match it like any other PHP advisory through the daily data refresh. No Tuner release is needed, and this page will be updated.

### What to do

1. Run a local scan on each server: `rhythm tuner scan /var/www/yoursite --id 'local-php-*'`.
2. If you get a high finding, apply the mitigation above now, restart PHP, and scan again. The finding drops to info: mitigated.
3. If you get a low finding, nothing is reachable today, but the code would become exposed if someone changed the configured handler. Remove the runtime switch when you can.
4. Delete any exposed `phpinfo()` page. It leaks configuration to anyone.
5. Upgrade PHP once a fixed release is out, and keep the mitigation until then.

---
<sub>← [Known vulnerabilities](Tuner-Known-Vulnerabilities) · [Home](Home) · [CI and code scanning](Tuner-CI) →</sub>
