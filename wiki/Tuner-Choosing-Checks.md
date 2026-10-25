By default a scan runs every polite check plus known-vulnerability matching. This page shows how to run fewer or more: profiles, tags, check IDs, a minimum severity, your own checks and how to preview a selection before you scan.

## At a glance

| Flag | Effect |
|---|---|
| `--profile quick` | the highest-value checks, about 30 requests; skips plugin and theme version lookups |
| `--profile default` | every polite check (the default) |
| `--tags leak,wordpress` | only checks with any of these tags |
| `--exclude-tags headers` | skip checks with any of these tags |
| `--id 'leak-*'` | only these checks (globs allowed); ignores the profile |
| `--exclude-id wp-readme` | skip these checks |
| `--severity medium` | skip checks below this severity |
| `--checks ./my-checks` | load checks from a directory instead of the built-in ones |
| `--no-check-updates` | use the checks built into your binary, never a check pack |
| `--no-vulns` | skip known-vulnerability matching |

List flags take comma-separated values and can be repeated: `--tags leak --tags wordpress` is the same as `--tags leak,wordpress`.

## Profiles

A profile trades coverage for requests. Each check declares the cheapest profile that runs it. A profile runs its own checks plus every cheaper one.

| Profile | Runs | Use it for |
|---|---|---|
| `quick` | checks marked `quick`: the top leaks (`.env`, `.git`, debug logs), security headers and cookies, TLS, user enumeration, XML-RPC, `display_errors`, Laravel logs and Ignition. 21 checks in 0.4.0, about 30 requests. | a fast first look, or frequent scans of many sites |
| `default` | everything polite and non-intrusive, adding backup and dump searches, `wp-config.php` copies, staging copies, backup plugin folders, phpinfo, database tools, framework debug pages, server status pages, directory listing and more. 77 checks in 0.4.0. | normal scans |

```sh
rhythm tuner scan yoursite.com --profile quick
```

On a test WordPress site the two compare like this:

```text
Score 0/100 - 4 critical, 1 high, 1 medium, 6 low, 1 info - 21 checks, 17 requests, 300ms
Score 0/100 - 4 critical, 2 high, 1 medium, 7 low, 1 info - 77 checks, 180 requests, 3.6s
```

> [!NOTE]
> Earlier releases offered a `deep` profile. No check uses it, so it's gone from the help and the picker. `--profile deep` still works and says so: `rhythm tuner scan: --profile deep: no check uses it yet, so it runs the same checks as default`. For more plugin discovery, see `--plugins deep` in [Requests and politeness](Tuner-Requests#plugin-discovery).

### What quick skips

Besides running fewer checks, `quick` skips the version lookups: it doesn't fetch each plugin's `readme.txt`, each theme's `style.css`, or `/feed/` for the WordPress version. Versions then come only from what the home page states (the generator tag and `?ver=` hints in asset URLs), so some plugins may show no version. When those have known vulnerabilities, Tuner reports `wp-components-unverified` and suggests rerunning with the default profile.

The **PHP version probe still runs in every profile**. When a page cache strips the PHP version from the home page, Tuner asks up to three uncacheable endpoints (such as `/xmlrpc.php`), so quick scans don't wrongly report the version as hidden.

Vulnerability matching runs in every profile too, on whatever versions were found.

## Tags

Every check has tags. `--tags` keeps checks that have **any** of the listed tags; `--exclude-tags` drops checks that have any of them.

```sh
rhythm tuner scan yoursite.com --tags leak                 # only leak checks
rhythm tuner scan yoursite.com --exclude-tags headers      # everything except header checks
```

Common tags:

| Tag | Covers |
|---|---|
| `leak` | exposed files: `.env`, `.git`, `.svn`, `.hg`, dumps, backups, logs, editor and deploy settings, `.DS_Store` |
| `secrets` | checks that look for credentials in what they find. `rhythm tuner checks --tags secrets` also lists the secret rules they apply |
| `backup`, `source` | backup archives and plugin backup folders; source code and version control |
| `server` | server pages such as `/server-status`, `/nginx_status` and `web.config` |
| `wordpress` | WordPress-specific checks and findings |
| `php`, `laravel` | PHP and Laravel checks |
| `headers`, `cookies` | security headers and cookie flags |
| `tls`, `transport` | certificates, TLS versions, plain HTTP |
| `disclosure` | version leaks, error output, readme files, phpinfo |
| `vuln` | known-vulnerability findings |
| `outdated`, `abandoned` | out-of-date or unmaintained components and software |
| `local` | findings from [local scans](Tuner-Local-Scans) |
| `integrity`, `malware` | [integrity and malware](Tuner-Integrity-And-Malware) findings in local scans |

`rhythm tuner checks` shows each check's tags.

Tags also filter vulnerability findings. Matching runs when your `--tags` include one of `vuln`, `wordpress`, `php`, `outdated`, `abandoned`, `nginx`, `apache`, `core`, `plugin`, `theme`, `composer` or `known-exploited`. Its findings are then filtered by the same tags. `--tags headers` alone skips vulnerability matching; `--exclude-tags vuln` turns it off.

## Check IDs

`--id` runs only the checks you name; `--exclude-id` skips them. Both accept shell-style globs (`*`, `?`, `[abc]`); quote them so your shell doesn't expand them.

```sh
rhythm tuner scan yoursite.com --id leak-dotenv,leak-git
rhythm tuner scan yoursite.com --id 'leak-*'
rhythm tuner scan yoursite.com --exclude-id 'hdr-*' --exclude-id wp-readme
```

`--id` **ignores the profile**: a check you name runs whatever its profile.

An ID that names no check is an error, with a suggestion, so typos fail loudly:

```text
$ rhythm tuner scan 127.0.0.1:20400 --id leak-dotnev
rhythm tuner scan: --id leak-dotnev: no check has this ID; did you mean leak-dotenv?
  list them with: rhythm tuner checks
```

### Vulnerability and local finding IDs

`--id` and `--exclude-id` also select findings that don't come from YAML checks: vulnerability matching and local scans. These patterns are accepted even though `rhythm tuner checks` doesn't list them:

| Pattern | Selects |
|---|---|
| `vuln-*` | all known-vulnerability findings: `vuln-core`, `vuln-plugin-<slug>`, `vuln-theme-<slug>`, `vuln-php`, `vuln-nginx`, `vuln-apache`, `vuln-composer-*` |
| `wp-core-*`, `wp-components-*` | outdated core, outdated, abandoned or unverified plugins and themes |
| `wp-plugin-closed`, `wp-theme-closed` | components removed from wordpress.org |
| `php-eol`, `nginx-eol`, `apache-eol` | end-of-life software |
| `php-version-hidden` | hidden PHP version with recent critical PHP flaws |
| `local-*` | local-scan findings, e.g. `local-wp-salts`; `local-integrity-*` and `local-malware-*` for [integrity and malware](Tuner-Integrity-And-Malware) |

```sh
rhythm tuner scan yoursite.com --id 'vuln-*'          # only vulnerability matching
rhythm tuner scan yoursite.com --exclude-id php-version-hidden
rhythm tuner scan /var/www/yoursite --id 'local-*'    # only local findings
```

`--id 'vuln-*'` on its own runs no YAML checks at all, just the fingerprint and the version match. Every finding ID is listed in the [Finding reference](Tuner-Finding-Reference); `rhythm tuner explain <id>` describes any of them offline.

## Minimum severity

`--severity` skips checks below a level and hides vulnerability findings below it too. Levels from most to least severe: `critical`, `high`, `medium`, `low`, `info`.

```sh
rhythm tuner scan yoursite.com --severity high
```

This changes what runs. To run everything but fail a CI build only on serious findings, use `--fail-on` instead; see [CI and code scanning](Tuner-CI).

## Your own checks

`--checks <dir>` loads checks from a directory **instead of** the built-in ones. Tuner reads every `.yaml` file in it, recursively, skipping `*_test.yaml` fixtures. A check with an unknown key, a missing field or a duplicate ID stops the run with an error naming the file.

```sh
rhythm tuner scan staging.yoursite.com --checks ./my-checks
```

The other selection flags apply on top. Writing a check takes no Go code; see [Writing checks](Tuner-Writing-Checks).

### Check packs

Tuner can take new checks from a signed check pack without a new binary. Tuner 0.4.0 trusts no pack signing key yet, so it always runs the checks built into it; `--no-check-updates` makes that explicit and `--checks` always uses your directory. See [Check packs](Tuner-Check-Packs).

## Preview a selection

`rhythm tuner checks` takes the same selection flags as `scan` and lists what would run, without sending a request to any site:

```sh
rhythm tuner checks --exclude-tags headers,tls --severity medium
```

```text
ID                           SEVERITY  PROFILE  TAGS                                NAME
laravel-debugbar             high      default  laravel,debug                       Laravel Debugbar request history public
laravel-horizon              medium    default  laravel,admin                       Laravel Horizon dashboard public
laravel-ignition             high      quick    laravel,debug                       Laravel Ignition debug endpoints exposed
laravel-log                  high      quick    laravel,leak,secrets                Laravel log file exposed
…
wp-wpforms-uploads           medium    default  wordpress,leak,privacy              WPForms uploads listed

Using built-in checks (rhythm-tuner 0.4.0).
51 checks, plus vulnerability matching for core, plugins, themes, PHP and
nginx/Apache (vuln-*, wp-components-*, *-eol), which scans skip with --no-vulns.
```

The last lines say which checks are in use: built-in, a check pack (with its version), or your `--checks` directory.

| Flag | |
|---|---|
| `--profile`, `--tags`, `--exclude-tags`, `--id`, `--exclude-id`, `--severity`, `--checks` | as for `scan` |
| `--json` | one object per check: `id`, `name`, `severity`, `profile`, `tags`, `requires`, `paths` |
| `--offline` | use only the cached check pack |
| `--no-check-updates` | list the built-in checks |

> [!TIP]
> `rhythm tuner checks` lists every check unless `--profile` picks one. Pass `--profile quick` to see exactly what a quick scan runs.

The listing shows YAML checks only. Vulnerability and local finding IDs aren't in it:

```text
$ rhythm tuner checks --id 'vuln-*'
rhythm tuner: no checks match the selection; list them with: rhythm tuner checks
```

`checks` also has subcommands for check authors (`new`, `test`, `lint`) and `checks export --nuclei <dir>`, which writes the checks as Nuclei templates; see [Writing checks](Tuner-Writing-Checks).

## Skip vulnerability matching

```sh
rhythm tuner scan yoursite.com --no-vulns
```

`--no-vulns` turns off known-vulnerability matching: no `vuln-*`, `*-eol`, `wp-core-*` or `wp-components-*` findings and no calls to the vulnerability data services. It also skips Composer matching with OSV.dev, for an exposed `composer.lock` and in local scans. Use it for a fast config-only pass, or where the data services aren't reachable and you have no cache. To use the cache without contacting the services, use `--offline` instead; see [Known vulnerabilities](Tuner-Known-Vulnerabilities).

---
<sub>← [Reading results](Tuner-Reading-Results) · [Home](Home) · [Requests and politeness](Tuner-Requests) →</sub>
