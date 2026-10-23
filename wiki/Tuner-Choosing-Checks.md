By default a scan runs every polite check plus known-vulnerability matching. This page shows how to run fewer or more: profiles, tags, check IDs, a minimum severity, your own checks, and how to preview a selection before you scan.

## At a glance

| Flag | Effect |
|---|---|
| `--profile quick` | the highest-value checks, about 30 requests; skips plugin and theme version lookups |
| `--profile default` | every polite check (the default) |
| `--profile deep` | adds aggressive enumeration |
| `--tags leak,wordpress` | only checks with any of these tags |
| `--exclude-tags headers` | skip checks with any of these tags |
| `--id 'leak-*'` | only these checks (globs allowed); ignores the profile |
| `--exclude-id wp-readme` | skip these checks |
| `--severity medium` | skip checks below this severity |
| `--checks ./my-checks` | load checks from a directory instead of the built-in ones |
| `--no-check-updates` | use the checks built into your binary, not the latest check pack |
| `--no-vulns` | skip known-vulnerability matching |

List flags take comma-separated values and can be repeated: `--tags leak --tags wordpress` is the same as `--tags leak,wordpress`.

## Profiles

A profile trades coverage for requests. Each check declares the cheapest profile that runs it, and a profile runs its own checks plus every cheaper one.

| Profile | Runs | Use it for |
|---|---|---|
| `quick` | checks marked `quick`: the top leaks (`.env`, `.git`, debug logs), security headers and cookies, TLS, user enumeration, XML-RPC, `display_errors`, Laravel logs and Ignition. 21 checks in v0.3.1. | a fast first look, or frequent scans of many sites |
| `default` | everything polite and non-intrusive, adding backup and dump searches, `wp-config.php` copies, phpinfo, Adminer, directory listing and more. 37 checks in v0.3.1. | normal scans |
| `deep` | `default` plus checks marked `deep`: more aggressive enumeration | thorough audits of your own sites |

```sh
rhythm tuner scan yoursite.com --profile quick
rhythm tuner scan yoursite.com --profile deep
```

> [!NOTE]
> No built-in check is marked `deep` yet, so `deep` runs the same checks as `default`. New checks can arrive in a [check pack](Tuner-Check-Packs) at any time; `rhythm tuner checks --profile deep` shows the current set.

### What quick skips

Besides running fewer checks, `quick` skips the version lookups: it doesn't fetch each plugin's `readme.txt`, each theme's `style.css`, or `/feed/` for the WordPress version. Versions then come only from what the home page states (the generator tag, and `?ver=` hints in asset URLs), so some plugins may show no version. When those have known vulnerabilities, Tuner reports `wp-components-unverified` and suggests rerunning with the default profile.

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
| `leak` | exposed files: `.env`, `.git`, `.svn`, dumps, backups, logs, `.DS_Store` |
| `secrets` | checks that look for credentials in what they find |
| `wordpress` | WordPress-specific checks and findings |
| `php`, `laravel` | PHP and Laravel checks |
| `headers`, `cookies` | security headers and cookie flags |
| `tls`, `transport` | certificates, TLS versions, plain HTTP |
| `disclosure` | version leaks, error output, readme files, phpinfo |
| `vuln` | known-vulnerability findings |
| `outdated`, `abandoned` | out-of-date or unmaintained components and software |
| `local` | findings from [local scans](Tuner-Local-Scans) |

`rhythm tuner checks` shows each check's tags.

Tags also filter vulnerability findings. Matching runs when your `--tags` include one of `vuln`, `wordpress`, `php`, `outdated`, `abandoned`, `nginx`, `apache`, `core`, `plugin`, `theme`, `composer` or `known-exploited`, and its findings are then filtered by the same tags. `--tags headers` alone skips vulnerability matching; `--exclude-tags vuln` turns it off.

## Check IDs

`--id` runs only the checks you name, and `--exclude-id` skips them. Both accept shell-style globs (`*`, `?`, `[abc]`); quote them so your shell doesn't expand them.

```sh
rhythm tuner scan yoursite.com --id leak-dotenv,leak-git
rhythm tuner scan yoursite.com --id 'leak-*'
rhythm tuner scan yoursite.com --exclude-id 'hdr-*' --exclude-id wp-readme
```

`--id` **ignores the profile**: a check you name runs even if it's marked `deep`.

An ID or pattern that matches nothing is an error, so typos fail loudly:

```text
rhythm tuner: no check matches "leak-dotnev"; list them with: rhythm tuner checks
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
| `local-*` | local-scan findings, e.g. `local-wp-salts` |

```sh
rhythm tuner scan yoursite.com --id 'vuln-*'          # only vulnerability matching
rhythm tuner scan yoursite.com --exclude-id php-version-hidden
rhythm tuner scan /var/www/yoursite --id 'local-*'    # only local findings
```

`--id 'vuln-*'` on its own runs no YAML checks at all, just the fingerprint and the version match. Every finding ID is listed in the [Finding reference](Tuner-Finding-Reference).

## Minimum severity

`--severity` skips checks below a level, and hides vulnerability findings below it too. Levels from most to least severe: `critical`, `high`, `medium`, `low`, `info`.

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

New checks are published as a check pack, and scans pick up the newest one within a day without a new binary. `--no-check-updates` pins a run to the checks built into your binary; `--checks` always uses your directory. See [Check packs](Tuner-Check-Packs).

## Preview a selection

`rhythm tuner checks` takes the same selection flags as `scan` and lists what would run, without sending a request to any site:

```sh
rhythm tuner checks --exclude-tags headers,tls --severity medium
```

```text
ID                           SEVERITY  PROFILE  TAGS                                NAME
laravel-ignition             high      quick    laravel,debug                       Laravel Ignition debug endpoints exposed
laravel-log                  high      quick    laravel,leak,secrets                Laravel log file exposed
leak-archive-backup          critical  default  leak,backup                         Site backup archive exposed
…

Using built-in checks (rhythm-tuner 0.3.1).
18 checks, plus vulnerability matching for core, plugins, themes, PHP and
nginx/Apache (vuln-*, wp-components-*, *-eol); skip it with --no-vulns.
```

The last lines say which checks are in use: built-in, a check pack (with its version), or your `--checks` directory.

| Flag | |
|---|---|
| `--profile`, `--tags`, `--exclude-tags`, `--id`, `--exclude-id`, `--severity`, `--checks` | as for `scan` |
| `--json` | one object per check: `id`, `name`, `severity`, `profile`, `tags`, `requires`, `paths` |
| `--offline` | use only the cached check pack |
| `--no-check-updates` | list the built-in checks |

> [!TIP]
> `rhythm tuner checks` defaults to `--profile deep`, so it lists everything. Pass `--profile default` or `--profile quick` to see exactly what those scans run.

The listing shows YAML checks only. Vulnerability and local finding IDs aren't in it, and `rhythm tuner checks --id 'vuln-*'` reports that no checks match.

## Skip vulnerability matching

```sh
rhythm tuner scan yoursite.com --no-vulns
```

`--no-vulns` turns off known-vulnerability matching: no `vuln-*`, `*-eol`, `wp-core-*` or `wp-components-*` findings, and no calls to the vulnerability data services. In a local scan it also skips Composer matching with OSV.dev. Use it for a fast config-only pass, or where the data services aren't reachable and you have no cache. To use the cache without contacting the services, use `--offline` instead; see [Known vulnerabilities](Tuner-Known-Vulnerabilities).

---
<sub>← [Reading results](Tuner-Reading-Results) · [Home](Home) · [Requests and politeness](Tuner-Requests) →</sub>
