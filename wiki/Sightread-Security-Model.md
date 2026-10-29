What Sightread runs, contacts, sends and writes. Also what it does with the hostile text inside the packages it reads. Sightread reads code that may be written to attack whoever opens it, so it treats every downloaded byte as hostile. Verified against rhythm-sightread 1.0.0.

## It never runs the code it downloads

Sightread reads packages as text. It never starts `php`, `node`, `npm`, `composer`, `git` or `svn` on fetched content, embeds no PHP, JS or WASM engine and never runs a lifecycle script or a Composer hook. The only programs it starts are:

| Program | When | What for |
|---|---|---|
| `git` | `changes` | `git show <ref>:<lockfile>` to read a lockfile at the base ref |
| your `--wp` command | `--wp` | read-only listing: `plugin list`, `theme list`, `core version`, `option get auto_update_plugins` |
| `rhythm-soundcheck`, `rhythm-roadie` and your `--wp` or `--apply` command | `--soundcheck` | install the update on the staging copy and compare it; see [With Soundcheck and Roadie](Sightread-With-Soundcheck) |

Each is started with an argument list (never through a shell) and killed with its children on Ctrl+C. `-v` prints each one before it starts:

```text
run wp-cli: .../testbed/bin/wp plugin list --format=json --fields=name,status,version,update_version,auto_update
run wp-cli: .../testbed/bin/wp theme list --format=json --fields=name,status,version,update_version,auto_update
run wp-cli: .../testbed/bin/wp core version
run wp-cli: .../testbed/bin/wp option get auto_update_plugins --format=json
```

`--soundcheck` is the one exception to "never runs": it installs the update on a staging copy and loads it in a browser, so the new code runs there. Every terminal run says which of the two it is under its header: `Read-only. Sightread changes nothing on your site and never runs the code it downloads.` or `Read-only here. --soundcheck installs the update on the staging copy, where its code runs.`

In the repository a test fails the build when anything but the one process package imports `os/exec`. The benchmark runs every bad scenario with a canary: each payload would connect to a local port and write a file if it ever ran. The [Benchmark](Sightread-Benchmark#safety) shows zero connections and zero files.

## What it contacts

Only the official sources, over HTTPS, with a fixed host allowlist checked on every redirect hop:

| Host | For |
|---|---|
| `api.wordpress.org` | plugin and theme information, versions, contributors |
| `downloads.wordpress.org` | plugin and theme zips, plugin checksums |
| `plugins.svn.wordpress.org` | who committed a plugin's release tag |
| `themes.svn.wordpress.org` | theme files with `--svn-check` |
| `repo.packagist.org`, `packagist.org` | Composer metadata and maintainers |
| `api.github.com`, `codeload.github.com` | Composer archives on GitHub, git tag checks |
| `gitlab.com`, `bitbucket.org` | Composer archives hosted there |
| `registry.npmjs.org` | npm metadata, tarballs and signing keys |
| `api.osv.dev` | known malicious and vulnerable versions (`--no-osv` skips it) |

Plus the mirrors and private registries you configure. `rhythm sightread update` also reaches GitHub's release hosts (`api.github.com`, `github.com`, `objects.githubusercontent.com`, `release-assets.githubusercontent.com`) for Sightread's own signed data packs. There is no telemetry.

- **HTTPS on every hop.** An `http://` hop is refused, except to a mirror on 127.0.0.1, `::1` or `localhost` you configured, for local mirrors and the testbed. A loopback port you didn't configure is refused too: `127.0.0.1:18902 is not a configured mirror` with the hint `Sightread reaches 127.0.0.1, ::1 or localhost only on the mirror ports you configure (the SIGHTREAD_* variables)`.
- **TLS is verified.** TLS 1.2 or newer, never skipped; `SSL_CERT_FILE` adds a CA for a proxy that inspects TLS.
- **Private addresses are refused.** A redirect to a cloud metadata address (169.254.169.254) or another private address never connects.
- **Lockfile URLs are never fetched.** A lockfile's `resolved` or `dist.url` could point anywhere. Sightread looks the name and version up in the registry's own metadata instead; an entry whose URL left the registry is the finding `lockfile-source-change`.
- **No lookups of what it finds.** Domains found in code are never resolved, looked up in WHOIS or sent to a reputation service.

## What it sends

Each lookup tells that registry a package name and version you use. Sightread never uploads site files.

- **Private names stay local.** A plugin with its own `Update URI`, an npm scope mapped to a private registry (in `.npmrc` or the config) and a Composer package from another repository are never sent to a public host. A plugin whose installed files match none of WordPress.org's files for its slug is treated as not on WordPress.org, so a premium namesake is never diffed against the public plugin. `--check-confusion` sends private names to the public registry on purpose, to look for public namesakes.
- **Credentials go to their own host.** `GITHUB_TOKEN` only to api.github.com and never across the redirect to codeload, a scope token only to its registry, each `COMPOSER_AUTH` entry only to its host. Sightread never reads tokens from `.npmrc` or `auth.json`.
- **Secrets in code are masked.** A credential found in a package is shown masked in every output and log; `-v` masks header values.

## What it writes

| Where | What | Mode |
|---|---|---|
| the cache (`~/.cache/rhythm/sightread` on Linux) | downloaded archives in a content-addressed store, registry answers, the last reports | 0700 |
| `-o` files | reports, written atomically once complete | 0600 |
| `.sightread/` | the config and allow file, only when you run `init` or `allow` | |
| the config folder | the `--sign` key pair | 0600 |

Archives are never unpacked under their own names. Every entry goes into a blob store named by its hash, so an entry called `../../wp-config.php` can't land anywhere. The only file written under a real name is the cached archive itself, plus the verified zip `--soundcheck` hands to WP-CLI. Each blob is hashed again when it is read; a cached blob changed on disk is fetched again.

Reports hold your full inventory. With `CI` set, paths in them are relative to the target (`--redact-paths`) and `data.target.repo_path` gives the target inside its repository.

## Hostile archives

Every entry name is checked before anything is stored: backslashes are read as `/`, then a leading `/`, a drive letter, a UNC path, a `..` segment, control bytes, a name Windows opens as a device or stream (`CON`, `NUL`, a colon) and names over 1,024 bytes are refused. A refused entry is the finding `archive-path-escape` and is never stored. Links, hard links, devices and FIFOs are recorded with their target text and never followed (`archive-link`). Duplicate names and case or Unicode collisions are `archive-collision`; every copy is analysed. This is a fixture archive with each trick:

```text
$ rhythm sightread plugin e05-entries -q
VERDICT     UPDATE       FROM   TO     FINDINGS
HOLD        e05-entries  1.0.0  1.0.1  5 critical, 2 high, 1 medium

HOLD: e05-entries 1.0.0 to 1.0.1
[CRITICAL] Archive entry points outside the package
  evidence: entry "/abs.php" points outside the package (outside the package)
  advice: Don't extract or install this archive.
[CRITICAL] Archive entry points outside the package
  evidence: entry "e05-entries/..\\win.php" points outside the package (outside the package)
  advice: Don't extract or install this archive.
[CRITICAL] Archive entry points outside the package
  evidence: entry "../escape.php" points outside the package (outside the package)
  advice: Don't extract or install this archive.
[CRITICAL] Archive does not match the published hash - File.php
  evidence: Archive does not match wordpress.org checksums: 8 files differ (File.php, dup.php, dup.php#2, e05-entries.php, file.php and 3 more)
  advice: Someone between you and wordpress.org changed it, or the release was edited; don't install this copy.
[CRITICAL] Archive holds a link or special file - e05-entries/link
  evidence: e05-entries/link links outside the package, to /etc/passwd (symlink to outside the package)
  advice: Check where the link points before installing.
[HIGH] Archive holds two entries with the same name - e05-entries/dup.php
  evidence: e05-entries/dup.php and e05-entries/dup.php are the same name (case or Unicode form) (duplicate entry; this copy is kept as e05-entries/dup.php#2)
  advice: Don't trust either copy until the archive is rebuilt.
...
```

Limits are counted on the bytes actually read: 256 MiB downloaded (`--max-size`), 1 GiB unpacked per package, 64 MiB stored and 4 MiB analysed per entry, 100,000 entries, a 200:1 ratio on entries over 1 MiB, nested archives listed but not opened. A limit hit is never clean: the file is `unanalysed` (a high finding) and the update is not verified:

```text
$ rhythm sightread plugin e04-bomb -q
VERDICT     UPDATE    FROM   TO     FINDINGS
UNVERIFIED  e04-bomb  1.0.0  1.0.1  1 high

UNVERIFIED: e04-bomb 1.0.0 to 1.0.1
[HIGH] Code that could not be analysed - e04-bomb/bomb.bin
  evidence: archive limit reached at this entry (expands more than 200:1; the rest of the archive was not read)
  advice: Read these files by hand. Sightread could not.
```

## Hostile text

Every string from a package (code, a changelog, a name, a domain) is cleaned, secret-masked and escaped for its output before any report sees it:

- **Terminal.** Escape sequences and bidirectional controls are shown as text (`\x1b`, `‮`), so a package can't recolor or rewrite your terminal.
- **Markdown.** `@mentions`, links and HTML render as text, so a pull request comment can't ping anyone or embed anything. URLs, `www.` names and the hosts a finding reports are defanged (`hxxps://analytics.wpos-cdn[.]example`) so GitHub doesn't link them.
- **HTML.** One self-contained file built with Go's `html/template`, with a Content-Security-Policy that loads nothing from outside and runs only its own script: `default-src 'none'; style-src 'unsafe-inline'; img-src data:; script-src 'sha256-...'`. Nothing from a package becomes markup, a link or an attribute.
- **SARIF.** Package paths go in the message and properties, never as a location GitHub would open.

## Review records and signatures

- **An allow entry is bound to bytes.** It matches one exact archive by its sha256. Other bytes under the same version are the critical finding `approved-version-changed`, never allowed. See [Allow and baseline](Sightread-Allow-And-Baseline).
- **A pull request can't approve itself.** `changes` and the Action read the allow file, the config and the baseline at the base ref. Anything a pull request relaxes (an allow or ignore entry, `osv: false`, a new mirror, `local_files`, `trusted_keys`) counts only once it is merged. The report notes each one. `--trust-pr-allow` turns that off. Protect `.sightread/allowed.yaml` with CODEOWNERS.
- **Five signals can't be silenced.** `integrity-mismatch`, `version-republished`, `approved-version-changed`, `archive-path-escape` and `known-malicious` can never be ignored, allowed or lowered.
- **Signed reports.** `--sign` writes an Ed25519 signature next to the JSON report; `verify-report` checks it. A signature proves the report wasn't changed after signing and which key signed it. A local key proves only "this machine".
- **Signed releases.** Releases are signed by the shared release workflow. The [GitHub Action](Sightread-CI) checks the release with cosign before anything runs and never builds from source; `update` checks a data pack's Sigstore signature before using it. See [Verifying releases](Verifying-Releases).

## What this doesn't protect against

A static diff can miss what it can't see. See [Honest limits](Sightread-FAQ#honest-limits) for the full list: obfuscation Sightread can't decode, logic that waits for a date or a domain, code fetched at run time from a server that serves clean code to scanners, compiled assets and binaries or a dependency it didn't review. A registry signature or provenance proves who published and from which workflow, not that the content is harmless. A staging copy made by Roadie is pseudonymised, not anonymised. Echo's seal stops mail, SMS and webhooks, not every outbound connection.

Found a security problem in Sightread itself? Report it privately: see the [Security policy](Security-Policy).

---
<sub>← [Output and reports](Sightread-Output) · [Home](Home) · [CLI reference](Sightread-CLI-Reference) →</sub>
