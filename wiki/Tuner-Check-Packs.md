New Tuner checks reach you without a new binary. Whenever checks change, CI publishes them as a *check pack*. Scans pick up the newest pack within a day. This page explains how packs are published, when a scan uses a pack instead of the checks built into your binary, how to control it and what a pack can and can't do.

## In short

- A pack is the `checks/` folder of YAML files, published on the rolling [`checks-latest`](https://github.com/RhythmShahriar/tuner/releases/tag/checks-latest) release of the tuner repository.
- Scans check for a new pack at most once a day, verify it against its SHA-256 checksums and use it only when it's newer than your binary's built-in checks.
- A pack contains only declarative checks: GET requests to the site you're scanning and matching on the replies. It can't run code.
- Any problem with a pack prints a warning and the scan carries on with the last good pack or the built-in checks. A pack never fails a scan.
- `--no-check-updates` pins a run to your binary's checks; `--checks <dir>` to your own.

## How a pack is published

When a change under `checks/` lands on `master`, the [checks-pack workflow](https://github.com/RhythmShahriar/tuner/blob/master/.github/workflows/checks-pack.yml):

1. Runs every check's positive and negative [fixtures](Tuner-Writing-Checks#fixtures) (`go test ./engine ./checkpack`). A failing check stops the publish.
2. Builds the pack with `make pack`.
3. Uploads three assets to the `checks-latest` release, replacing the previous pack.

| Asset | Contents |
|---|---|
| `checks.tar.gz` | every check and the checks' LICENSE. Fixtures stay out. |
| `checks-manifest.json` | pack version, schema, `min_tuner`, the commit it was built from (and its time) and the number of checks |
| `checks-sha256.txt` | SHA-256 of the archive and the manifest |

```json
{
  "schema": 1,
  "version": "2026.10.23-154747",
  "commit": "6e1ecee2a436fcfde43e431740cd68efbec96b70",
  "created": "2026-10-23T15:47:47Z",
  "min_tuner": "0.2.0",
  "checks": 37
}
```

The pack version is the commit time, `YYYY.MM.DD-HHMMSS` in UTC.

`checks-latest` is a prerelease and never marked latest. `rhythm update` and the installers read GitHub's latest release, which skips prereleases, so a pack is never mistaken for a Tuner release.

**Reproducible.** `make pack` builds the same three files into `dist/checks-pack`, byte for byte as CI does, given the same commit and Go version. To check a published pack yourself:

```sh
git clone https://github.com/RhythmShahriar/tuner && cd tuner
git checkout <commit from checks-manifest.json>
make pack
cat dist/checks-pack/checks-sha256.txt    # compare with the published checks-sha256.txt
```

## When a scan uses a pack

Each remote scan picks one set of checks:

1. **`--checks <dir>`** given: the checks in that directory. No pack is consulted.
2. **`--no-check-updates`** given: the built-in checks.
3. Otherwise, the newest **verified, compatible pack**, if it **supersedes** the built-in checks.
4. Otherwise, the built-in checks.

A pack supersedes the built-in checks when:

| Your binary | Uses the pack when |
|---|---|
| a release build (knows the commit time it was built from) | the pack was built from a later commit |
| built from a working tree with uncommitted changes | never: you always run the checks you're editing |
| built without commit information (e.g. `go install` from the module proxy) | always, if the pack is compatible |

So right after a Tuner release, the pack and the binary are usually from the same commit, so the built-in checks are used. As soon as a new check lands, the pack wins.

[Local scans](Tuner-Local-Scans) don't run HTTP checks, so a run whose targets are all directories never downloads or uses a pack.

### Compatibility: `min_tuner` and `schema`

The scan rejects a pack, keeping the last good pack or the built-in checks, when:

- its `schema` isn't the one this binary reads (currently `1`);
- its `min_tuner` is newer than your release (development builds count as new enough);
- any check fails to load. Checks reject unknown fields, so a pack that relies on a matcher your binary doesn't have is refused;
- the archive is unsafe or doesn't match the manifest (see [Trust](#trust-model)).

The rejection is remembered for a day, so every scan doesn't download the same pack again. The warning says what to do:

```text
rhythm tuner: check pack rejected: pack 2026.11.02-090000 needs rhythm-tuner 0.4.0 or newer; run: rhythm update; using built-in checks (rhythm-tuner 0.3.1)
```

For check authors: `min_tuner` is set in [`checks/pack.json`](https://github.com/RhythmShahriar/tuner/blob/master/checks/pack.json). A check that uses a **new matcher field** doesn't need a bump: older binaries reject the unknown field and keep their own checks. **Bump it** when a change would load fine on an older binary but behave differently there, for example a new allowed value for an existing field. `schema` changes only when the pack layout itself changes.

## Caching and updates

The pack is cached as a single file and checked against the release at most once a day (24 hours since the last check).

| OS | Check pack cache |
|---|---|
| Linux | `$XDG_CACHE_HOME/rhythm/tuner/checks/pack.json`, usually `~/.cache/rhythm/tuner/checks/pack.json` |
| macOS | `~/Library/Caches/rhythm/tuner/checks/pack.json` |
| Windows | `%LocalAppData%\rhythm\tuner\checks\pack.json` |

The daily check costs two tiny downloads (the checksums and the manifest). The archive is only downloaded when its checksum differs from the cached one. The cached pack is verified again every time it's used, because your binary may have changed since it was cached.

The lookup is capped at 10 seconds per scan so an unreachable GitHub never holds up a scan. It goes through `--proxy` like the rest of the scan.

| When | What happens |
|---|---|
| network down, or a checksum mismatch (e.g. a release caught mid-upload) | the cached pack is used with a warning (`couldn't refresh the check pack: …`); the next run tries again |
| no cached pack and no network | built-in checks, with a warning |
| `--offline` | the cached pack however old, or built-in checks if none is cached (no warning) |
| `--no-check-updates` | built-in checks; the cache isn't read |
| `--checks <dir>` | checks from the directory |

### Commands and output

`rhythm tuner checks` lists the checks and says where they came from. It takes the same selection and check-source flags as `scan`, plus `--json`. Unlike `scan`, its profile defaults to `deep`, so with no flags it lists every check:

```text
$ rhythm tuner checks
ID                                SEVERITY  PROFILE  TAGS                                NAME
hdr-content-type-options-missing  low       quick    headers                             X-Content-Type-Options header missing
…
wp-xmlrpc                         low       quick    wordpress,bruteforce                XML-RPC enabled

Using built-in checks (rhythm-tuner 0.3.1).
37 checks, plus vulnerability matching for core, plugins, themes, PHP and
nginx/Apache (vuln-*, wp-components-*, *-eol); skip it with --no-vulns.
```

With a newer pack in use, the line reads `Using check pack 2026.11.02-090000 (commit 1a2b3c4, 2026-11-02).`

`rhythm tuner update` refreshes the pack now, ignoring the daily schedule, along with the [vulnerability data](Tuner-Known-Vulnerabilities#caching-and-automatic-refresh):

```text
$ rhythm tuner update
Checks: built-in (rhythm-tuner 0.3.1); they're as new as pack 2026.10.23-154747
Refreshed 29 vulnerability records
```

When the pack is newer, the first line reads `Checks: pack <version>, <n> checks (commit <sha>)`. `rhythm update` runs this for you after updating the binaries. `rhythm tuner update --clear` deletes the cached pack and vulnerability data. `update` exits with code 3 if the pack couldn't be fetched or was rejected.

### In reports

JSON reports say which checks a scan used:

| Field | Values |
|---|---|
| `check_set` | `built-in`, `pack` or `directory` |
| `check_pack` | the pack version, e.g. `2026.11.02-090000`, when `check_set` is `pack` |

Record these with your results if you need to reproduce a scan later. For a fixed check set in CI, use `--no-check-updates` (the binary's checks) or `--checks <dir>` with a pinned checkout of the tuner repository.

## Trust model

A pack is **data, not code**:

- Checks are declarative YAML. They can only make GET requests to the site being scanned and match the replies. They can't run commands, read local files, or contact other hosts.
- Tuner rejects any field it doesn't know, so a pack can't smuggle in behavior your binary doesn't already have.
- The pack is downloaded over HTTPS from the tuner repository's releases and verified against `checks-sha256.txt`. The manifest is verified first, then the archive.
- The archive is unpacked in memory, never to disk. Absolute paths, `..`, links, devices, duplicate names and oversized files reject the whole pack (limits: 16 MB archive, 1 MB per file, 32 MB unpacked, 5,000 files).
- The number of checks must match the manifest.

> [!WARNING]
> The assets are **checksummed, not signed**. The checksum protects against corruption and partial uploads; it doesn't protect against someone who can publish to the tuner repository's releases. If that matters for your environment, pin your scans with `--no-check-updates` and update the binary through your own review process, whose release archives you can [verify](Verifying-Releases).

## Mirrors

`TUNER_CHECKS_URL` points Tuner at a mirror of the `checks-latest` release assets. Tuner appends the asset names to it:

```sh
export TUNER_CHECKS_URL=https://mirror.yoursite.com/tuner/checks-latest
# Tuner fetches:
#   $TUNER_CHECKS_URL/checks-sha256.txt
#   $TUNER_CHECKS_URL/checks-manifest.json
#   $TUNER_CHECKS_URL/checks.tar.gz
```

The default is `https://github.com/RhythmShahriar/tuner/releases/download/checks-latest`. Mirror all three files together; the same checksum and compatibility rules apply.

---
<sub>← [Writing checks](Tuner-Writing-Checks) · [Home](Home) · [Finding reference](Tuner-Finding-Reference) →</sub>
