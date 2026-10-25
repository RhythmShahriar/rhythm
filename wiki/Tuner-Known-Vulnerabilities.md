Besides its YAML checks, every Tuner scan matches the software versions it finds against public vulnerability data: WordPress core, plugins, themes, PHP, nginx and Apache, plus Composer packages from a local `composer.lock` or one a site exposes. This page explains where the data comes from, how matching works, what each finding means and how the data stays current.

## At a glance

- One finding per vulnerable component, listing every advisory that applies, with CVE, severity, CVSS, the version that fixes it and whether attackers are using it in the wild.
- Extra findings for outdated, closed and abandoned plugins and themes, for end-of-life PHP and web servers and for sites that hide their PHP version.
- No API key. Data is cached on your machine and refreshed automatically once it's a day old. A free Wordfence Intelligence key adds a second source.
- Matching runs alongside the checks in every scan. Turn it off with `--no-vulns`, or run only matching with `--id 'vuln-*'`.

```sh
rhythm tuner scan yoursite.com                  # checks + vulnerability matching
rhythm tuner scan yoursite.com --id 'vuln-*'    # vulnerability matching only
rhythm tuner scan yoursite.com --no-vulns       # checks only
```

## Data sources

| Source | Used for | License / notes |
|---|---|---|
| [WPVulnerability](https://www.wpvulnerability.com/) | advisories for WordPress core, plugins, themes, PHP, nginx and Apache, plus end-of-life dates for PHP and server branches | public domain (CC0). Aggregates CVE, Wordfence, WPScan and Patchstack |
| [api.wordpress.org](https://api.wordpress.org/) | latest WordPress release; each plugin's and theme's latest version and last update date | used for "outdated" and "abandoned" findings |
| [php.net release states](https://www.php.net/releases/states.php) | which PHP branches are supported and until when | drives hidden-version and end-of-life inference, so a new PHP branch or end of life needs no Tuner update |
| [OSV.dev](https://osv.dev/) | Composer packages from `composer.lock` in [local scans](Tuner-Local-Scans), or an exposed `composer.lock` or `vendor/composer/installed.json` in remote scans | GitHub Advisory Database and FriendsOfPHP |
| [Wordfence Intelligence](#wordfence-intelligence) | a second advisory feed for core, plugins and themes | optional; needs a free API key in `TUNER_WORDFENCE_KEY` |

Findings from vulnerability data carry their source for attribution. The terminal shows it once at the end of a scan:

```text
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
```

In JSON, each such finding has `"source": "WPVulnerability (wpvulnerability.com, CC0)"`; Composer findings name `OSV.dev (GitHub Advisory Database, FriendsOfPHP)`.

## What gets matched

| Component | Remote scan: where the version comes from | Local scan |
|---|---|---|
| WordPress core | the `generator` meta tag, the block library stylesheet's `?ver=`, or `/feed/` | `$wp_version` in `wp-includes/version.php` |
| Plugins | `readme.txt` (`Stable tag:`), else an asset's `?ver=` hint | the `Version:` header of the plugin's main file |
| Themes | `style.css` (`Version:`), else an asset's `?ver=` hint | the `Version:` header of `style.css` |
| PHP | `X-Powered-By`, or a `PHP/x.y.z` token in `Server` (see [Versions behind caches](#versions-behind-page-caches)) | `php -n -r 'echo PHP_VERSION;'`, or `--php <binary>` |
| nginx, Apache | the `Server` header, e.g. `nginx/1.24.0`, `Apache/2.4.57 (Debian)` | not read |
| Composer packages | an exposed `composer.lock` or `vendor/composer/installed.json` (up to 500 packages) | `composer.lock` |

Plugins and themes are found from the asset URLs on the home page (`/wp-content/plugins/<slug>/…`, `/wp-content/themes/<slug>/…`) and the REST API index, plus the `readme.txt` probes of `--plugins deep` (see [Plugin discovery](Tuner-Requests#plugin-discovery)). Up to 50 of them get a version lookup per scan. The `quick` profile skips those lookups, so it relies on `?ver=` hints. An asset's `?ver=` is ignored when it equals the WordPress version (many plugins pass that instead of their own).

## One finding per component

Tuner groups every advisory that affects an installed version into a single finding. The terminal shows the first three, worst first:

```text
[CRITICAL] contact-form-7 5.3.1: 6 known vulnerabilities - http://127.0.0.1:20400/wp-content/plugins/contact-form-7/
  - CRITICAL CVE-2020-35489 Contact Form 7 < 5.3.2 - Unrestricted File Upload (fixed in 5.3.2)
  - HIGH CVE-2023-6449 Contact Form 7 <= 5.8.3 - Authenticated (Editor+) Arbitrary File Upload (fixed in 5.8.4)
  - MEDIUM CVE-2024-2242 Contact Form 7 <= 5.9 - Reflected Cross-Site Scripting (fixed in 5.9.2)
  - …and 3 more
  fix: Update contact-form-7 to 6.0.6 or later (latest is 6.1.3): wp plugin update contact-form-7
```

An exposed `composer.lock` gets the same treatment, one finding per vulnerable package:

```text
[HIGH] guzzlehttp/guzzle 7.4.1: 3 known vulnerabilities (from exposed composer.lock) - http://127.0.0.1:20400/composer.lock
  - HIGH CVE-2022-29248 Cross-domain cookie leakage (fixed in 7.4.3)
  - HIGH CVE-2022-31042 Failure to strip the Cookie header on change in host or HTTP downgrade (fixed in 7.4.4)
  - HIGH CVE-2022-31091 Change in port should be considered a change in origin (fixed in 7.4.5)
  fix: Update guzzlehttp/guzzle to 7.4.5 or later: composer update guzzlehttp/guzzle --with-dependencies
```

| Finding ID | Component |
|---|---|
| `vuln-core` | WordPress core |
| `vuln-plugin-<slug>` | a plugin, e.g. `vuln-plugin-contact-form-7` |
| `vuln-theme-<slug>` | a theme, e.g. `vuln-theme-astra` |
| `vuln-php` | PHP |
| `vuln-nginx`, `vuln-apache` | the web server |
| `vuln-composer-<vendor>-<package>` | a Composer package, e.g. `vuln-composer-guzzlehttp-guzzle` |

How a finding is built:

- **Severity** is the worst advisory's severity. An unrated advisory counts as medium.
- **Advisories** are ordered: actively exploited first, then by severity, then by CVSS score. The terminal shows the top three, the finding's evidence the top five. JSON and Markdown list every advisory; SARIF carries every CVE.
- **Fixed in** is the first safe version, when the advisory states one. The fix line names the highest fixed version across all advisories, so one update clears them all. When no fixed version is known, Tuner says so and suggests removing or replacing the component.
- **Actively exploited** marks advisories in CISA's [Known Exploited Vulnerabilities](https://www.cisa.gov/known-exploited-vulnerabilities-catalog) catalog. They're tagged `[actively exploited]`, the finding gets the `known-exploited` tag and the impact says how many there are.

In JSON, each advisory is an object in the finding's `advisories` array:

```json
{
  "id": "d7be3d4ac93e5686af000b7e2014a21c26a005b249f789b0d2a5ec8aa45c20eb",
  "title": "When using Apache and PHP-CGI on Windows, if the system is set up to use certain code pag…",
  "cves": ["CVE-2024-4577"],
  "severity": "critical",
  "cvss": 9.8,
  "kev": true,
  "fixed_in": "8.1.29",
  "references": ["https://www.cve.org/CVERecord?id=CVE-2024-4577"]
}
```

In JSON, a finding with any actively exploited advisory has `known-exploited` in its `tags`, which makes it easy to pick out in CI scripts. Since v0.3.2 you can also scan for them alone: `rhythm tuner scan yoursite.com --tags known-exploited`.

## Other findings from version data

| ID | Severity | When |
|---|---|---|
| `wp-core-outdated` | low | a newer WordPress release exists (only when `vuln-core` didn't already fire) |
| `wp-components-outdated` | low | plugins or themes have newer releases; one finding lists them all (those already in a `vuln-*` finding are left out) |
| `wp-plugin-closed`, `wp-theme-closed` | medium | closed on wordpress.org, often for an unfixed security issue; shows the date and reason when known |
| `wp-components-abandoned` | low | no release on wordpress.org in over two years |
| `wp-components-unverified` | info, *likely* | a plugin or theme has known vulnerabilities in some versions, though its installed version couldn't be read |
| `php-eol` | high | the PHP branch is past end of life (see [below](#end-of-life-php-and-inferred-advisories)) |
| `nginx-eol`, `apache-eol` | medium | the server branch is past end of life |
| `php-version-hidden` | medium, *likely* | the site runs PHP, its version couldn't be read and recent critical PHP flaws exist (see [below](#hidden-php-versions)) |

Plugins and themes that aren't on wordpress.org (premium or custom) get no outdated or abandoned findings; they're still matched against advisories. See the [Finding Reference](Tuner-Finding-Reference) for every ID.

## Hidden PHP versions

Hiding the PHP version (`expose_php = Off`, or a proxy that strips `X-Powered-By`) is good practice. It also means Tuner can't tell from outside whether the site is patched. Rather than stay silent, Tuner reports `php-version-hidden` with the flaws it can't rule out.

It's reported when all of these hold:

1. The site runs PHP: it's WordPress or Laravel, or it sets a `PHPSESSID` cookie.
2. No PHP version was found on the home page or the [uncached endpoints](#versions-behind-page-caches).
3. At least one advisory qualifies.

**Which advisories qualify.** Tuner reads php.net's release states and keeps the branches that still get security fixes (state `stable` or `security` with the security-support end date not yet passed). For each supported branch it looks up the advisories that were **published in the last 180 days** and are **critical or in CISA's KEV catalog**. The same CVE across branches is merged into one entry that lists the fixed release on each branch, or "no fix yet" for a branch without one.

The title names the most pressing flaw: actively exploited first, then the most recently published, then the highest CVSS score. A WordPress test site that hides its PHP version, scanned on 2026-10-25:

```text
$ rhythm tuner scan http://127.0.0.1:20405 --plain --id php-version-hidden
…
[MEDIUM, likely] PHP version hidden: can't rule out CVE-2026-109618 (critical) and 7 more - http://127.0.0.1:20405/
  the PHP version is hidden, so this can't be confirmed
  - CVE-2026-109618 PHP uses longjmp() for out-of-memory (OOM) errors to exit the current context and abort h… (fixed in 8.5.12, 8.4.27, 8.3.36, 8.2.35)
  - CVE-2026-109619 PHP has various global state that temporarily points into request memory. In some scenari… (fixed in 8.5.12, 8.4.27, 8.3.36, 8.2.35)
  - CVE-2026-17543 Improper escaping of backslashes in attacker-provided parameters would allow for trivial… (fixed in 8.5.9, 8.4.24, 8.3.33, 8.2.33)
  - …and 5 more
  fix: Check the PHP version serving the site: php -v on the server (the command line can run a different PHP than the web server, so confirm in your hosting panel, or in WordPress under Tools → Site Health → Info → Server). If it's older than 8.5.12, 8.4.27, 8.3.36, 8.2.35 on its branch, upgrade. Branches older than 8.2 are end of life and need moving to a supported one.
```

The fix tells you how to check the version yourself and which releases are safe on each branch. Branches older than the oldest supported one are end of life and need moving to a supported branch.

The finding is medium with low confidence, so it counts half in the [Tune score](Tuner-Reading-Results). It disappears once the advisories age past 180 days and no new critical one arrives, or once Tuner can read the version.

> [!TIP]
> The quickest way to settle it is a [local scan](Tuner-Local-Scans) on the server, which reads the version directly: `rhythm tuner scan /var/www/yoursite`. Or check `php -v`, your hosting panel, or WordPress under **Tools → Site Health → Info → Server**. The command-line `php` can differ from the one your web server runs.

In a local scan of a WordPress or Laravel site, the same finding appears if Tuner can't run `php` (it notes `php not found on PATH`); pass `--php /path/to/php` to fix that.

## Versions behind page caches

Page caches (Cloudflare, LiteSpeed, Varnish and others) often serve the home page without `X-Powered-By`. When the home page shows PHP but no version, Tuner asks a few endpoints that caches don't store, stopping at the first that answers with a version:

| Site | Endpoints, in order |
|---|---|
| WordPress | `/xmlrpc.php`, `/wp-admin/admin-ajax.php`, `/wp-login.php` |
| Other PHP sites | `/index.php` |

That's at most three extra plain GET requests in every profile. The checks reuse the responses. The JSON report's `fingerprint.php_source` names the header and path the version came from. PHP findings add a `version from …` line to their evidence:

```json
"fingerprint": {
  "tech": ["wordpress", "php"],
  "php_version": "8.1.27",
  "php_source": "x-powered-by /wp-login.php",
  "server": "nginx"
}
```

## End-of-life PHP and inferred advisories

Branches past end of life (8.1, 8.0, 7.4 and older as of this writing) never get fixes. Vulnerability databases usually stop listing new flaws for them, so a plain version match would make an old PHP look safer than a supported one. Tuner closes that gap:

1. The PHP record's end-of-life date has passed, so `php-eol` is raised, at **high**.
2. Tuner takes the oldest supported branch newer than yours (from php.net) and looks at its advisories.
3. Advisories with **no lower version bound**, published **after your branch's end of life** and not already listed for your version are added to `vuln-php`, marked `(no fix for this branch)`.
4. The fix says to move to a supported branch and how many of the issues will never be fixed on yours.

```text
[CRITICAL] PHP 8.1.27: 41 known vulnerabilities - http://yoursite.com/
  - CRITICAL CVE-2024-4577 When using Apache and PHP-CGI on Windows, … (fixed in 8.1.29) [actively exploited]
  - CRITICAL CVE-2025-14179 The PDO Firebird driver improperly handles NUL bytes … (no fix for this branch)
  - CRITICAL CVE-2026-6722 The SOAP extension's object deduplication mechanism … (no fix for this branch)
  - …and 38 more
  fix: Upgrade to a supported PHP branch (8.2 or later). PHP 8.1 is end of life, so 13 of these issues will never be fixed on it.
[HIGH] PHP 8.1 is past end of life - http://yoursite.com/
  evidence: PHP 8.1.27, support ended 2025-12-31
```

**Limits of inference.** It's an informed estimate, not a confirmed match:

- An advisory with no lower bound is assumed to reach older code. A flaw in a feature added after your branch was released doesn't. The data often can't tell which is which.
- Only advisories of the oldest supported branch that have a publish date are considered.
- When *every* advisory in `vuln-php` is inferred, the finding gets low confidence with the caveat "inferred from a supported branch's advisories".
- nginx and Apache get `*-eol` findings but no inferred advisories.

Whatever the count, the fix is the same: move to a supported PHP branch.

## Distribution builds and backports

Linux distributions patch security flaws without changing the upstream version number. `Apache/2.4.57 (Debian)` may already include fixes from 2.4.60. Tuner can't see that from outside, so when the PHP or server version string names a distribution build, the finding gets low confidence, the caveat "a distro build may already include the fix" and a note to check the package's security status before acting.

This applies to `vuln-php`, `vuln-nginx`, `vuln-apache` and the `*-eol` findings when the version string contains a marker such as `ubuntu`, `debian`, `deb12`, `el9`, `centos`, `red hat`, `rhel`, `fedora`, `amazon`, `rocky`, `alma` or `suse` (e.g. `PHP/7.4.3-4ubuntu2.19`). Such findings count half in the score and are marked *likely* in the live terminal view, with `confidence: low` in JSON and Markdown reports.

> [!NOTE]
> Low confidence isn't "ignore". Check your distribution's security tracker or package changelog for the CVEs listed. An end-of-life branch stays end of life whatever the distribution backports, unless your vendor offers extended support for it.

## Caching and automatic refresh

Each lookup (one component, one release record, the PHP branch list) is cached as a small JSON file and reused for 24 hours. A scan that needs a record older than that fetches it again. Requests to the data services go through your `--proxy`, use the scan's User-Agent and are spaced 200 ms apart to be polite to free services. They connect within 3 seconds and wait `--timeout` (default 10 seconds) for an answer.

| OS | Vulnerability data cache |
|---|---|
| Linux | `$XDG_CACHE_HOME/rhythm/tuner/vulns`, usually `~/.cache/rhythm/tuner/vulns` |
| macOS | `~/Library/Caches/rhythm/tuner/vulns` |
| Windows | `%LocalAppData%\rhythm\tuner\vulns` |

Inside it: `wpvulnerability/<kind>/<key>.json`, `wordpress.org/<kind>/<slug>.json`, `php.net/releases/states.json`, `osv/…` for Composer data, `wordfence/` for the optional feed and `checksums/` for [`--integrity`](Tuner-Integrity-And-Malware#caching-offline-use-and-partial-scans). A damaged cache file is treated as missing and fetched again; it never stops a scan.

**When a service can't be reached**, Tuner gives up on that host for the rest of the run after the first timeout or refused connection, uses the cached copy however old and prints one line, for example:

```text
Score may miss known vulnerabilities (vulnerability data partial)
Score 0/100 - 3 critical, 3 high, 1 medium, 7 low, 1 info - 77 checks, 180 requests, 3.6s
note: vulnerability data unreachable, using cache from 3 days ago (use --offline to skip)
note: Composer vulnerability data unreachable, using cache from 3 days ago (use --offline to skip)
```

With nothing cached, the same scan reads:

```text
Score excludes known vulnerabilities (vulnerability data missing)
Score 67/100 - 1 high, 1 medium, 6 low, 1 info - 77 checks, 180 requests, 3.6s
note: vulnerability data unreachable, no cached data; known vulnerabilities not checked
note: Composer vulnerability data unreachable, no cached data; known vulnerabilities not checked
```

JSON reports say the same in `vuln_data`: `partial` or `missing`. Vulnerability data never fails a scan on its own. `--fail-on` and `--fail-on-new` won't pass a site whose data is missing unless you add `--allow-missing-vulns`.

## Controls

| Command or flag | Effect |
|---|---|
| `rhythm tuner update` | refresh every cached record now, Composer data and the Wordfence feed included, ignoring the 24-hour age |
| `rhythm tuner update --prefetch <dir\|url>...` | fingerprint sites or folders and cache the data their scans need, for a later `--offline` run |
| `rhythm tuner update --clear` | delete the cached data, checksums and check pack |
| `rhythm update` | update rhythm and its tools, then run `rhythm tuner update` |
| `--offline` | use cached data only, however old; never contact the data services |
| `--no-vulns` | skip vulnerability matching entirely |
| `--id 'vuln-*'` | run only vulnerability matching (no YAML checks) |

```text
$ rhythm tuner update
Checks: built-in (rhythm-tuner 0.4.0)
Refreshed 7 vulnerability records
Refreshed 6 Composer advisory records
Data: WPVulnerability (wpvulnerability.com, CC0); OSV.dev (GitHub Advisory Database, FriendsOfPHP)
Cache: /home/you/.cache/rhythm/tuner/vulns
```

`rhythm tuner update` refreshes only what's already cached; data for a new site is fetched during its first scan, or ahead of it with `--prefetch`. It exits with code 3 if any record failed to refresh (the cached copies are kept).

**Offline and air-gapped use.** Prefetch what the scans will need, then scan with `--offline`:

```text
$ rhythm tuner update --prefetch /var/www/app 127.0.0.1:20400
Checks: built-in (rhythm-tuner 0.4.0)
Prefetched /var/www/app: PHP 8.3.36, 2 Composer packages
Prefetched 127.0.0.1:20400: WordPress 6.9.3, 2 plugins, 1 theme, PHP 8.1.2
Data: WPVulnerability (wpvulnerability.com, CC0); OSV.dev (GitHub Advisory Database, FriendsOfPHP)
Cache: /home/you/.cache/rhythm/tuner/vulns
```

`--prefetch` sends the same requests a scan's fingerprint step does (the home page, plugin and theme versions, the PHP probe) and runs no checks. In offline mode, components with nothing cached are skipped with the warning `vulnerability data not cached for N components; run once online to cache it`. In CI, cache `~/.cache/rhythm` between runs; see [CI and code scanning](Tuner-CI#speed-and-reproducibility).

## Wordfence Intelligence

Set `TUNER_WORDFENCE_KEY` to a free [Wordfence Intelligence](https://www.wordfence.com/threat-intel/) API key and Tuner also matches plugins, themes and core against the Wordfence Intelligence feed. Without the variable nothing changes and no request goes to Wordfence.

- The feed is downloaded whole, indexed and cached under `vulns/wordfence/`. A scan reuses a copy up to a week old and downloads it (which can take a few minutes) only when none is cached; `rhythm tuner update` refreshes it every time. With the key set, run `rhythm tuner update` daily (from cron, say) so scans never wait for it.
- Advisories both sources list appear once: same CVE, or the same title and an overlapping range. The merged advisory keeps the wider range, gains the other's CVEs and references and takes the higher CVSS score. Wordfence-only advisories are added.
- Findings with a Wordfence advisory credit both sources in their `Source` line, with Defiant's copyright notice and license link. Each Wordfence advisory links its record on wordfence.com.
- A feed that can't be fetched gives one warning (`couldn't fetch the Wordfence Intelligence feed (www.wordfence.com: HTTP 401; check TUNER_WORDFENCE_KEY); matched with WPVulnerability alone`); the key never appears in messages.
- `TUNER_WORDFENCE_API` points Tuner at a mirror.

Check Wordfence's terms for how you may use and share the data in your reports.

## Mirrors

Point Tuner at your own mirror or caching proxy with these environment variables. Each takes a base URL; Tuner appends the same paths it uses on the public service.

| Variable | Default | Paths Tuner requests |
|---|---|---|
| `TUNER_VULN_API` | `https://www.wpvulnerability.net` | `/<kind>/<key>/` for `core`, `plugin`, `theme`, `php`, `nginx`, `apache` |
| `TUNER_WPORG_API` | `https://api.wordpress.org` | `/core/version-check/1.7/`, `/plugins/info/1.2/?…`, `/themes/info/1.2/?…`, `/core/checksums/1.0/?…` |
| `TUNER_WPDL_API` | `https://downloads.wordpress.org` | `/plugin-checksums/<slug>/<version>.json` (`--integrity`) |
| `TUNER_PHPNET_API` | `https://www.php.net` | `/releases/states.php` |
| `TUNER_OSV_API` | `https://api.osv.dev` | `POST /v1/querybatch`, `GET /v1/vulns/<id>` |
| `TUNER_WORDFENCE_API` | `https://www.wordfence.com/api/intelligence/v3` | `/vulnerabilities/production`, with the key as a bearer token |

```sh
export TUNER_VULN_API=https://vulns.internal.yoursite.com
rhythm tuner scan yoursite.com
```

The cache doesn't record which mirror a record came from, so switching mirrors takes effect as records reach 24 hours, or immediately after `rhythm tuner update`.

---
<sub>← [Changes and notifications](Tuner-Changes-And-Notifications) · [Home](Home) · [PHP advisories](Tuner-PHP-Advisories) →</sub>
