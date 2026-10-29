Once someone has read an update, say so once and stop seeing its findings. Sightread has one review record for that, `.sightread/allowed.yaml`, bound to the exact bytes that were reviewed. A config `ignore` entry hides one kind of finding in one place. `--baseline` hides findings an earlier report already had. This page covers all three and when to use which.

The examples on this page ran against Sightread's testbed (a fixture registry on 127.0.0.1) in a WordPress site folder that is also a git repository, with `git config user.name` set to Ann Example.

| You want to… | Use | Bound to |
|---|---|---|
| Record that you reviewed a version and accept its findings | `allow` | that version's archive hash |
| Accept one kind of finding in one package or path, for every version | config `ignore` | signal ID, package and path globs |
| Only see what is new since a report you already read | `--baseline <report.json>` | finding fingerprints |

## Allow a version

```sh
rhythm sightread allow wp-plugin:g05-cron@1.1.0 --reason "daily transient cleanup, read the cron callback"
```

```text
allowed wp-plugin:g05-cron@1.1.0 (sha256 74e9e4bd35dd) in .sightread/allowed.yaml
```

`allow` needs the version's sha256. It fetches and verifies the version itself, or takes the hash from a saved report with `--from`. `--reason` is required, so the next person knows why. `--by` defaults to `git config user.name`, else `$USER`. The file it writes:

```yaml
# Versions someone reviewed. An entry matches only this exact archive (sha256).
# rhythm sightread allow writes this file; protect it with CODEOWNERS.
allowed:
  - package: wp-plugin:g05-cron
    version: 1.1.0
    sha256: 74e9e4bd35dd70fbcb7664bcf8027a5538cfaa64086705fca1dd310d0351e39b
    hash: manifest
    reason: daily transient cleanup, read the cron callback
    by: Ann Example
    date: "2026-10-28"
```

The next review of that update shows the findings as allowed and the verdict becomes ALLOWED:

```text
$ rhythm sightread plugin g05-cron
...
  ▸ Changelog     1.1.0: "Add a daily cleanup task that removes expired
                  transients."

  Not counted: 1 allowed.

 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                    ALLOWED  │
 │  1 allowed                                                           │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  1 update reviewed · 2 archives verified · 1 from cache · 0s

  → update it: wp plugin update g05-cron --version=1.1.0
```

Commit the file and protect it with CODEOWNERS (`/.sightread/allowed.yaml @your-org/reviewers`). In CI the allow file is read from the base branch, so a pull request can't allow its own update; see [CI](Sightread-CI#allow-entries-in-pull-requests).

### The hash

The sha256 is over the archive bytes Sightread verified: the npm tarball or the Composer dist zip. WordPress.org rebuilds zips without changing their files, so for WordPress.org packages it's over the sorted file manifest (`path<TAB>sha256` lines) and the entry says `hash: manifest`.

The same version with any other hash is the critical finding `approved-version-changed`: the review you recorded doesn't cover these bytes. It can't be ignored or allowed away; review the new archive as if it had never been allowed, then replace the entry with `--force`.

### Only some signals, for a while

```sh
rhythm sightread allow npm:leftpadplus@1.0.1 \
  --reason "postinstall reviewed, it only builds a native helper" \
  --signals npm-install-script-new --until 2027-01-31
```

```text
allowed npm:leftpadplus@1.0.1 (sha256 844a96148701) in .sightread/allowed.yaml
```

With `--signals`, only those findings are allowed; anything else in the update still counts. Here the update still has a critical `remote-fetch-exec`, so it stays on HOLD and the box counts the one allowed finding:

```text
 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                       HOLD  │
 │  ■ 1 critical   ■ 1 high   ■ 2 medium   1 allowed                    │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

`--until` counts through that day. A date in the past is refused:

```text
$ rhythm sightread allow npm:patchlib@2.0.1 --reason x --until 2020-01-01
rhythm sightread allow: --until 2020-01-01 is in the past, so the entry would never count
  give today or a later date, or leave --until out
```

`allow list` marks an entry past its day `(expired)`. Allowing a version whose only entry has expired replaces it without `--force`, with a warning.

### Everything a report reviewed

After reading a report, allow every update in it at once. The hashes come from the report, so nothing is fetched:

```text
$ rhythm sightread allow --from before.json --reason "reviewed in PR 412"
allowed wp-plugin:b02-formtools@1.4.1 (sha256 0d2c174cdd71) in .sightread/allowed.yaml
allowed wp-plugin:b04-roles@2.0.1 (sha256 5af2ad9cda64) in .sightread/allowed.yaml
```

An update already allowed with the same hash is skipped with `already allowed <eco>:<name>@<version> (sha256 <12 hex>) in <file>`.

### What can't be allowed

`allow` refuses a version it couldn't verify, a version whose bytes don't match their published hash and a local copy (`--to-file`):

```text
$ rhythm sightread allow wp-plugin:b17-tamper@1.0.1 --reason "looked fine"
rhythm sightread allow: wp-plugin:b17-tamper@1.0.1 doesn't match its published hash, so it can't be allowed
  someone between you and the registry changed it, or the release was edited; don't install this copy
```

Reports offer the allow command only for a version `allow` accepts. And `integrity-mismatch`, `version-republished`, `approved-version-changed`, `archive-path-escape` and `known-malicious` are never allowed, whatever the entry says.

### List, remove, import

```text
$ rhythm sightread allow list
PACKAGE                   SHA256        SIGNALS                 BY           DATE        UNTIL       REASON
npm:leftpadplus@1.0.1     844a96148701  npm-install-script-new  Ann Example  2026-10-28  2027-01-31  postinstall reviewed, it only builds a native helper
wp-plugin:g05-cron@1.1.0  74e9e4bd35dd  all                     Ann Example  2026-10-28  -           daily transient cleanup, read the cron callback

$ rhythm sightread allow remove npm:leftpadplus@1.0.1
removed npm:leftpadplus@1.0.1 from .sightread/allowed.yaml
```

`allow list --json` prints the `allowed` document:

```json
{
  "schema": "rhythm-sightread/v1",
  "kind": "allowed",
  ...
  "verdict": "ok",
  "data": {
    "file": ".sightread/allowed.yaml",
    "entries": [
      {
        "package": "npm:leftpadplus",
        "version": "1.0.1",
        "sha256": "844a96148701b98b53f2ad506a4fb62716aaf03e2e029392d71a03d62b0079f2",
        "signals": [
          "npm-install-script-new"
        ],
        "reason": "postinstall reviewed, it only builds a native helper",
        "by": "Ann Example",
        "date": "2026-10-28",
        "until": "2027-01-31"
      },
      ...
    ]
  }
}
```

`allow list` escapes control bytes, tabs and newlines in every field, so a shared file can't send escape sequences to your terminal.

### Sharing reviews across projects

An agency reviews the same plugin version once for every client. Two ways to share:

- `allow import ~/agency/allowed.yaml` merges another file's entries into this one and records `imported_from` on each.
- Config `allow_from: [../agency-reviews/allowed.yaml]` reads other files on every run, without copying them.

In a fleet file, each site can name its own `allow_file`; see [Watch](Sightread-Watch#fleets). `--no-allow` ignores every allow file for one run.

## Ignore a kind of finding

An `ignore` entry in `.sightread/config.yaml` accepts one signal in one package or path, for every version, until a date:

```yaml
ignore:
  - id: capability-change
    package: wp-plugin:b04-roles
    path: roles.php
    reason: grants our own shop_manager role, checked with the vendor
    until: 2027-03-31
```

`id` and `reason` are required; `package` and `path` are globs. The report counts what it hid:

```text
$ rhythm sightread plugin b04-roles --config cfgtest/good.yaml
...
[HIGH, likely] Changelog does not mention what the code adds - roles.php:3
  evidence: no changelog entry for 2.0.1; the update adds capability changes (roles.php:3), network sends (roles.php:5), file writes outside the package (roles.php:5)
  advice: Hold the update. Nothing in the changelog explains this.
...
Not counted: 1 ignored by config.

Verdict: REVIEW
1 high, 1 medium
```

Use an allow entry when you read a version; use ignore when a package's known behaviour keeps firing release after release. The five unignorable signals are refused:

```text
rhythm sightread: cfgtest/bad4.yaml: ignore 1 (integrity-mismatch): integrity-mismatch can't be ignored; it says the bytes are not the ones published or reviewed
```

## Baseline

`--baseline <report.json>` hides every finding already in an earlier report, matched by fingerprint (the first 16 hex of a SHA-256 over the ecosystem, name, candidate version, signal, file and evidence, with paths and whitespace normalised). It works like Tuner's `--baseline`. Here the baseline report reviewed `b02-formtools` and `b04-roles`; a new run adds `b06-ajax`:

```text
$ rhythm sightread plugin b02-formtools b04-roles b06-ajax --baseline before.json
...
  VERDICT     UPDATE         FROM   TO     FINDINGS
  REVIEW      b06-ajax       4.2.0  4.2.1  2 high, 1 medium
  ROUTINE     b02-formtools  1.4.0  1.4.1
  ROUTINE     b04-roles      2.0.0  2.0.1
...
 ╭──────────────────────────────────────────────────────────────────────╮
 │  3 UPDATES                                     1 review · 2 routine  │
 │  ■ 2 high   ■ 1 medium                                               │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
```

Baselined findings are counted in the JSON (`summary.baselined`, `baselined: true` on each). In CI a committed baseline, like the allow file and the config, counts from the base branch only.

## Hiding is not accepting

`--id`, `--exclude-id` and `--severity` only hide findings from what you read. Hidden findings still set the verdict and still count for `--fail-on`; see [CLI reference](Sightread-CLI-Reference#signals). To accept a finding, use one of the three above.

---
<sub>← [Signal reference](Sightread-Signal-Reference) · [Home](Home) · [Sources and integrity](Sightread-Sources-And-Integrity) →</sub>
