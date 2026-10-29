Every Sightread setting for a project lives in `.sightread/config.yaml`. This page covers how it's found, every key with its default, how the config behaves in pull requests, the environment variables Sightread reads and its cache. Every key is optional and flags on the command line win.

## The config file

```sh
rhythm sightread init            # write .sightread/config.yaml here
rhythm sightread init --force    # replace an existing one
```

In a WordPress site with no lockfiles:

```text
$ rhythm sightread init
Wrote .sightread/config.yaml: reviews wp-plugin and wp-theme, fails on high or worse.

Next steps:
  1. Review the file, then commit it.
  2. Review what is installed here: rhythm sightread all
  3. Review every pull request that moves a lockfile: rhythm sightread init ci
```

`init` lists the ecosystems it finds in the folder (a WordPress site, `composer.lock`, npm lockfiles); in a folder with none it lists all four and says `nothing was found here yet, so it reviews every ecosystem`. It never replaces a file without `--force`:

```text
rhythm sightread init: .sightread/config.yaml exists (use --force to replace it)
```

The file it writes, with every setting explained:

```yaml
# Sightread settings for this project. Commit this file.
# Every key is optional and flags on the command line win.
# All keys: https://github.com/RhythmShahriar/rhythm/wiki/Sightread-Configuration

# Fail the run (exit 1) on findings at or above this severity:
# none, info, low, medium, high or critical.
fail_on: high

# Count only findings at this confidence or surer: confirmed or likely.
min_confidence: likely

# A release younger than this gets the fresh-release signal.
min_age: 72h

# What "all" reviews here. These were found when this file was written.
ecosystems: [wp-plugin, wp-theme]

# Also review packages you don't require directly.
transitive: false

# Look versions up on OSV for known-malicious and known-vulnerable.
osv: true

# Versions someone reviewed. Protect this file with CODEOWNERS.
allow_file: .sightread/allowed.yaml

# Mirrors and private registries, https only. Tokens come from the
# environment (SIGHTREAD_NPM_TOKEN_<SCOPE>, COMPOSER_AUTH), never from here.
# sources:
#   npm:
#     scopes:
#       "@acme": https://npm.acme.example
#   composer:
#     repositories: [https://repo.acme.example]

# Premium plugins have no official source: say where their zips are.
# local_files:
#   gravityforms: ~/vendor-zips/gravityforms-{version}.zip

# Hide a finding someone reviewed. id and reason are required.
# ignore:
#   - id: outbound-new-domain
#     package: wp-plugin:woocommerce
#     path: src/Internal/Admin/RemoteInboxNotifications/*
#     reason: WooCommerce inbox notes, reviewed
#     until: 2027-03-31
```

### How it's found

Every command uses the nearest `.sightread/config.yaml` in the folder you run it from or a folder above it. `--config <file>` names one; `--no-config` reads none, for a folder you don't trust. The config is found from where you run, not from `--path`: to review `sites/shop` with its own config, run from there or pass `--config sites/shop/.sightread/config.yaml`. Paths in the config (`allow_file`, `allow_from`, `local_files`) are relative to the project folder that holds `.sightread/`.

## Keys

| Key | Default | Meaning | Flag |
|---|---|---|---|
| `fail_on` | `none` (`init` writes `high`) | exit `1` on a finding at or above this level | `--fail-on` |
| `min_confidence` | `likely` | `confirmed` counts only exact patterns | `--min-confidence` |
| `min_age` | `72h` | `fresh-release` fires for a candidate younger than this | `--min-age` |
| `ecosystems` | all four | what `all` reviews: `wp-plugin`, `wp-theme`, `composer`, `npm` | `--only`, `--skip` |
| `path` | `.` | the site or project folder | `--path` |
| `transitive` | `false` | review transitive Composer and npm dependencies | `--transitive` |
| `pre` | `false` | let `latest` and `wanted` pick pre-releases | `--pre` |
| `svn_check` | `false` | verify themes file by file against themes.svn.wordpress.org | `--svn-check` |
| `osv` | `true` | ask api.osv.dev about known malicious and vulnerable versions | `--no-osv` |
| `sources` | the official hosts | mirrors and private registries; see [Sources](#sources) | |
| `local_files` | none | where premium plugins' zips are; see [Local files](#local-files) | `--to-file` |
| `allow_file` | `.sightread/allowed.yaml` | the review record | `allow --file` |
| `allow_from` | none | other allow files read on every run, e.g. `[../agency-reviews/allowed.yaml]` | |
| `trusted_keys` | none | public keys `verify-report` trusts, as `ed25519:<base64>` | `verify-report --key` |
| `signals.severity` | each signal's default | a severity per signal ID, e.g. `fresh-release: low` | |
| `signals.domains_ok` | none | host globs `outbound-new-domain` treats as known, e.g. `[api.wordpress.org, "*.stripe.com"]` | |
| `ignore` | none | findings to stop counting; see [Ignore](#ignore) | |
| `soundcheck` | none | defaults for `--soundcheck`; see [Soundcheck](#soundcheck) | |

### Sources

```yaml
sources:
  wporg_api: https://api.wordpress.org
  wporg_downloads: https://downloads.wordpress.org
  wporg_svn: https://plugins.svn.wordpress.org
  wporg_theme_svn: https://themes.svn.wordpress.org
  packagist: https://repo.packagist.org
  npm:
    registry: https://registry.npmjs.org
    scopes:
      "@acme": https://npm.acme.example        # token: SIGHTREAD_NPM_TOKEN_ACME
  composer:
    repositories: [https://repo.acme.example]  # auth: COMPOSER_AUTH
```

Every URL must be `https://`; `http://` is accepted only on 127.0.0.1, `::1` or `localhost`, for local mirrors. A scope mapped to a private registry is never asked of the public one; Sightread also reads `@scope:registry=` lines from `.npmrc` (and nothing else from it). Tokens come from the environment only, each sent to its own host. A package your lockfile resolved from the configured npm mirror is reviewed like a registry package.

### Local files

A premium plugin has no official source. `local_files` tells `all` where the vendor's zips are: the plugin's slug (or `wp-plugin:<slug>`) and a path with `{version}`:

```yaml
local_files:
  gravityforms: ~/vendor-zips/gravityforms-{version}.zip
```

`all` then reviews the newest zip there whose version is above the installed one against the installed copy, as an unverified update (see [Local files](Sightread-Sources-And-Integrity#local-files)). For one plugin, `--to-file <zip>` does the same on the command line.

### Ignore

```yaml
ignore:
  - id: outbound-new-domain
    package: wp-plugin:woocommerce
    path: src/Internal/Admin/RemoteInboxNotifications/*
    reason: WooCommerce inbox notes, reviewed
    until: 2027-03-31
```

`id` and `reason` are required; `package` (`<eco>:<name>`) and `path` (inside the package) are globs; `until` is a date after which the entry stops counting. The report counts what it hid (`Not counted: 1 ignored by config.`). `integrity-mismatch`, `version-republished`, `approved-version-changed`, `archive-path-escape` and `known-malicious` can't be ignored. See [Allow and baseline](Sightread-Allow-And-Baseline#ignore-a-kind-of-finding) for when to ignore and when to allow.

### Soundcheck

```yaml
soundcheck:
  staging: https://staging.shop.test
  wp: "wp --path=/srv/staging"
  production: [https://shop.example.com]
  fail_on: broke
  args: [--limit, "100"]
```

| Key | Meaning |
|---|---|
| `staging` | the staging URL `--soundcheck` uses when you give none |
| `wp` | the WP-CLI command on the staging copy; it must run on this machine (see [With Soundcheck and Roadie](Sightread-With-Soundcheck)) |
| `production` | URLs `--soundcheck` refuses, on top of `production:` in `.soundcheck/config.yaml` |
| `fail_on` | the Soundcheck change class that fails: `broke`, `new` or `changed` |
| `args` | extra `rhythm-soundcheck snapshot` flags |

With a `soundcheck` section, `doctor` also checks that `rhythm-soundcheck` and `rhythm-roadie` are installed.

## Errors

The config is checked before anything is fetched. Each error exits `2`:

```text
rhythm sightread: cfgtest/bad1.yaml: unknown key failon (did you mean fail_on?)

rhythm sightread: cfgtest/bad2.yaml: ignore 1 (cron-new-hook): needs a reason, so the next person knows why it is accepted

rhythm sightread: cfgtest/bad3.yaml: soundcheck.fail_on "broken": want broke, new or changed

rhythm sightread: cfgtest/bad4.yaml: ignore 1 (integrity-mismatch): integrity-mismatch can't be ignored; it says the bytes are not the ones published or reviewed
```

The config is read with limits on YAML aliases and depth, so a crafted file can't expand into a huge tree.

## In pull requests

`changes` and the [GitHub Action](Sightread-CI) read the config, the allow file and a committed baseline at the base ref, not from the pull request. A setting the pull request relaxes counts only once it is merged: `ignore`, `signals.severity`, `signals.domains_ok`, `min_confidence`, `fail_on`, `allow_file` and `allow_from`, plus `osv: false`, `sources`, `local_files`, `trusted_keys`, `ecosystems` and `min_age`. The report notes each one with `relaxed in this change, not counted (--trust-pr-allow counts it)`. A baseline the change added is dropped the same way. `--trust-pr-allow` (Action input `trust-pr-allow`) counts them anyway.

## Sites file

`all --sites` and `watch -l` read a fleet file. See [Watch](Sightread-Watch#fleets).

```yaml
sites:
  - name: shop
    path: /var/www/shop
  - name: brochure
    wp: "ssh deploy@brochure.example -- wp --path=/srv/www"
  - name: api
    path: ~/code/acme-api
  - name: client-remote
    tuner_report: ~/reports/client.json
    allow_file: ~/agency/allowed.yaml
```

## Environment

| Variable | Use |
|---|---|
| `GITHUB_TOKEN`, `RHYTHM_GITHUB_TOKEN` | sent only to api.github.com, for Composer archives on GitHub and tag checks |
| `SIGHTREAD_NPM_TOKEN_<SCOPE>` | a private npm scope's token (`@acme` reads `SIGHTREAD_NPM_TOKEN_ACME`), sent only to that scope's registry |
| `COMPOSER_AUTH` | private Composer repositories, as Composer reads it |
| `SIGHTREAD_SMTP_PASSWORD` | `--notify smtp://` |
| `SIGHTREAD_*` base URLs | mirrors and the testbed; see [CLI reference](Sightread-CLI-Reference#environment-variables) |
| `RHYTHM_OFFLINE`, `RHYTHM_CACHE_DIR`, `NO_COLOR`, `CI` | shared with every Rhythm tool |

## The cache

| Folder | What | Kept |
|---|---|---|
| `blobs/<ab>/<sha256>` | every file of every archive, by hash, re-hashed on read | until `cache prune` |
| `archives/<eco>/<name>/<version>.json` | each archive's file tree and how it was verified | until `cache prune --older` |
| `meta/` | registry answers | version lists 10 minutes, per-version metadata 7 days, WordPress.org checksums for good, maintainers 12 hours, OSV 6 hours, npm keys 24 hours |
| `runs/` | the last reports, for `explain` and `watch` | |

The cache is `~/.cache/rhythm/sightread` on Linux (`RHYTHM_CACHE_DIR` moves it), mode 0700. Two runs can share it. `rhythm sightread cache prune [--older 30d]` removes old archive indexes and the blobs nothing names; `rhythm uninstall sightread --purge` removes it all.

---
<sub>← [CI and the GitHub Action](Sightread-CI) · [Home](Home) · [Output and reports](Sightread-Output) →</sub>
