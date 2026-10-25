A *check pack* lets new Tuner checks reach you without a new binary: whenever checks change, CI builds and signs a pack that scans can pick up the newest one within a day. This page explains how packs are built and signed, when a scan uses one, how to control it and what a pack can and can't do.

> [!IMPORTANT]
> **Tuner 0.4.0 doesn't use check packs.** A binary only trusts packs signed with a key compiled into it. 0.4.0 ships with no key yet, so it never downloads a pack and always runs its built-in checks. New checks reach 0.4.0 users with the next release. Everything below describes how packs work once a release trusts the key.

## In short

- A pack is the `checks/` folder of YAML files, plus optional fingerprint data and local scan rules, published on the rolling [`checks-latest`](https://github.com/RhythmShahriar/tuner/releases/tag/checks-latest) release of the tuner repository.
- Its checksum file is signed with Ed25519. A binary verifies the signature against the public keys built into it before it trusts any checksum and again each time it uses the cached pack.
- A scan checks for a new pack at most once a day and uses it only when it's newer than the binary's built-in checks and carries (or retires) every one of them.
- A pack holds only declarative checks and data. Checks can only send GET or HEAD requests to the site you're scanning; anything else doesn't load.
- Any problem with a pack prints a warning and the scan carries on with the last good pack or the built-in checks. A pack never fails a scan.
- `--no-check-updates` pins a run to your binary's checks; `--checks <dir>` to your own.

## How a pack is published

When a change under `checks/`, `fingerprint/data/` or `local/rules/` lands on `master`, the [checks-pack workflow](https://github.com/RhythmShahriar/tuner/blob/master/.github/workflows/checks-pack.yml):

1. Runs every check's positive and negative [fixtures](Tuner-Writing-Checks#fixtures), the lint rules and the Nuclei export. A failure stops the publish.
2. Builds the pack and signs `checks-sha256.txt` with the `TUNER_PACK_SIGNING_KEY` secret. It refuses to publish an unsigned pack.
3. Uploads four assets to the `checks-latest` release, replacing the previous pack.

| Asset | Contents |
|---|---|
| `checks.tar.gz` | every check and the checks' LICENSE, plus `data/<name>.json` fingerprint data and `local/<name>.yaml` local scan rules. Fixtures stay out. |
| `checks-manifest.json` | pack version, schema, `min_tuner`, the commit it was built from (and its time), the number of checks, `retired` IDs and the data files and local rules it holds |
| `checks-sha256.txt` | SHA-256 of the archive and the manifest |
| `checks-sha256.txt.sig` | a base64 Ed25519 signature of `checks-sha256.txt` |

The pack version is the commit time, `YYYY.MM.DD-HHMMSS` in UTC. `checks-latest` is a prerelease and never marked latest, so `rhythm update` and the installers never mistake a pack for a Tuner release.

**Reproducible.** `make pack` builds the same files into `dist/checks-pack` (unsigned unless you give it a key), byte for byte as CI does, given the same commit and Go version. To check a published pack, build it from the commit in `checks-manifest.json` and compare `checks-sha256.txt`.

## When a scan uses a pack

Each remote scan picks one set of checks:

1. **`--checks <dir>`** given: the checks in that directory. No pack is consulted.
2. **`--no-check-updates`** given: the built-in checks.
3. Otherwise, the newest **signed, compatible pack**, if it **supersedes** the built-in checks.
4. Otherwise, the built-in checks.

A binary with no trusted key (Tuner 0.4.0, forks and development builds) stops at step 4 every time, quietly.

A pack supersedes the built-in checks when:

| Your binary | Uses the pack when |
|---|---|
| a release build (knows the commit time it was built from) | the pack was built from a later commit |
| built from a working tree with uncommitted changes | never: you always run the checks you're editing |
| built without commit information (e.g. `go install` from the module proxy) | always, if the pack is compatible |

A cached pack older than the binary is quietly left unused. A pack newer than the binary must carry every built-in check or list it under `retired`, so a pack can't silently drop a check you had.

[Local scans](Tuner-Local-Scans) use a pack's local rules in place of the built-in ones; a run whose targets are all folders doesn't need the pack's HTTP checks.

### Compatibility: `min_tuner` and `schema`

The scan rejects a pack, keeping the last good pack or the built-in checks, when:

- its signature is missing, malformed or not from a trusted key;
- its `schema` isn't the one this binary reads (currently `1`);
- its `min_tuner` is newer than your release (development builds count as new enough);
- any check fails to load. Checks reject unknown fields, so a pack that relies on a matcher your binary doesn't have is refused;
- it lacks a built-in check and doesn't retire it;
- it's older than the pack already cached (no rollback);
- the archive is unsafe or doesn't match the manifest (see [Trust](#trust-model)).

For check authors: `min_tuner` is set in [`checks/pack.json`](https://github.com/RhythmShahriar/tuner/blob/master/checks/pack.json), currently `0.4.0`. **Bump it** when a change would load fine on an older binary but behave differently there. To delete a check, remove its YAML and add its ID to `retired` there.

## Caching and updates

The pack is cached as a single file and checked against the release at most once a day.

| OS | Check pack cache |
|---|---|
| Linux | `$XDG_CACHE_HOME/rhythm/tuner/checks/pack.json`, usually `~/.cache/rhythm/tuner/checks/pack.json` |
| macOS | `~/Library/Caches/rhythm/tuner/checks/pack.json` |
| Windows | `%LocalAppData%\rhythm\tuner\checks\pack.json` |

A scan waits at most 3 seconds for the pack host when a pack is cached, 10 seconds when none is. A failed refresh is remembered for an hour: until then, scans use the cached pack or the built-in checks without asking again. The warning prints once, even under `-q`:

```text
rhythm tuner scan: check pack host unreachable, using built-in checks
```

| When | What happens |
|---|---|
| host unreachable, or a checksum or signature problem | the cached pack (or built-in checks) with a warning; tried again after an hour |
| `--offline` | the cached pack however old, or built-in checks if none is cached |
| `--no-check-updates` | built-in checks; the cache isn't read |
| `--checks <dir>` | checks from the directory |

`rhythm tuner update` refreshes the pack now, ignoring the daily schedule and the back-off. Its first line says which checks scans will use:

```text
$ rhythm tuner update
Checks: built-in (rhythm-tuner 0.4.0)
…
```

With a pack in use it reads `Checks: pack <version>, <n> checks (commit <sha>)`.

`rhythm tuner checks` lists the checks and says where they came from:

```text
$ rhythm tuner checks --profile quick
…
Using built-in checks (rhythm-tuner 0.4.0).
21 checks, plus vulnerability matching for core, plugins, themes, PHP and
nginx/Apache (vuln-*, wp-components-*, *-eol), which scans skip with --no-vulns.
```

With a pack in use the line reads `Using check pack <version> (commit <sha>, <date>).`

### In reports

JSON reports say which checks a scan used:

| Field | Values |
|---|---|
| `check_set` | `built-in`, `pack` or `directory` |
| `check_pack` | the pack version, when `check_set` is `pack` |

Record these with your results if you need to reproduce a scan later. For a fixed check set in CI, use `--no-check-updates` (the binary's checks) or `--checks <dir>` with a pinned checkout of the tuner repository.

## Trust model

A pack is **data, not code**:

- Checks are declarative YAML. They can only send GET or HEAD requests to the site being scanned, with plain query strings. They match the replies. A check that asks for anything else doesn't load. They can't run commands, read local files or contact other hosts.
- Fingerprint data files are JSON (at most 512 KB each, 32 files); local rules are declarative YAML matched against files during a local scan (at most 256 KB each). Neither can run anything.
- Tuner rejects any field it doesn't know, so a pack can't smuggle in behavior your binary doesn't already have.
- `checks-sha256.txt` must carry a valid Ed25519 signature from a key compiled into your binary. The manifest is verified next, then the archive.
- The archive is unpacked in memory, never to disk. Absolute paths, `..`, links, devices, duplicate names, misplaced files and oversized files reject the whole pack.

Whoever holds the signing key can publish checks to every binary that trusts it, so the key lives only in the tuner repository's Actions secrets. If you'd rather review every change yourself, pin your scans with `--no-check-updates` and update the binary through your own review process; release archives can be [verified](Verifying-Releases).

## Private packs and mirrors

`TUNER_CHECKS_URL` points Tuner at a mirror of the `checks-latest` release assets. Tuner appends the asset names to it:

```sh
export TUNER_CHECKS_URL=https://mirror.yoursite.com/tuner/checks-latest
# Tuner fetches:
#   $TUNER_CHECKS_URL/checks-sha256.txt
#   $TUNER_CHECKS_URL/checks-sha256.txt.sig
#   $TUNER_CHECKS_URL/checks-manifest.json
#   $TUNER_CHECKS_URL/checks.tar.gz
```

The default is `https://github.com/RhythmShahriar/tuner/releases/download/checks-latest`. Mirror all four files together; the same signature, checksum and compatibility rules apply.

A fork that publishes its own pack can sign it with its own key and build that key into its binaries, or set `TUNER_CHECKS_INSECURE=1` to skip the signature check (checksums still apply). Every run with it says so:

```text
rhythm tuner scan: TUNER_CHECKS_INSECURE=1: check pack signatures are not verified; use it only for a pack you build yourself
```

---
<sub>← [Writing checks](Tuner-Writing-Checks) · [Home](Home) · [Finding reference](Tuner-Finding-Reference) →</sub>
