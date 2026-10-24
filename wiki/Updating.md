One command keeps `rhythm`, every installed tool and their data current. This page explains what `rhythm update` does, how to update only some tools, hold back new releases, go to a chosen version or back to the last one, how downloads are verified, the once-a-day release notice, how Tuner refreshes its vulnerability data and checks on its own, how Soundcheck keeps its browser current and what Echo updates.

## Update everything

```sh
rhythm update
```

It works in this order:

1. **`rhythm` itself.**
2. **Every installed tool**, found as `rhythm-<tool>` on your `PATH` (today: `rhythm-tuner`, `rhythm-soundcheck` and `rhythm-echo`), then the [extensions](Extensions) you installed with `rhythm ext`.
3. **Each tool's data**, by running `rhythm-<tool> update`. For Tuner that refreshes the [check pack](Tuner-Check-Packs) and the cached vulnerability data; for Soundcheck, the browser it uses (see [Soundcheck's browser](#soundchecks-browser)). Echo has nothing to refresh (see [Echo](#echo)).

```console
$ rhythm update
rhythm             dev (development build, not updated)
rhythm-echo        1.0.0 (up to date)
rhythm-soundcheck  1.1.0 (up to date)
rhythm-tuner       0.3.1 → 0.3.2 ✔
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
Echo has nothing to refresh; provider recognizers update with the binary (rhythm update).
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
Checks: pack 2026.10.24-084900, 37 checks (commit 78c563e)
Nothing cached yet. Vulnerability data is fetched during scans and refreshed daily.
```

A released rhythm says `rhythm             0.7.0 (up to date)` on its first line, or updates itself first; the one above was built from source.

The lines after the binaries are the tools' own `update` output: Echo's, Soundcheck's browser check and Tuner's check pack. A tool built on the shared kit reports its data in one line instead, such as `rhythm-tuner 0.3.2  data: checks 2026.10.20, vulns 2026-10-24  ok`.

Each binary is updated in place, wherever it is installed. A problem with one target is reported and the rest still run. When anything failed, the run ends with a summary such as `Updated 2, up to date 1, failed 1: rhythm-tuner data` and exits `1`; otherwise it exits `0`.

### Only some tools

Name tools, short names, binaries or `rhythm` itself to update only those (and refresh only their data):

```sh
rhythm update tuner                 # Tuner and its data
rhythm update tuner,echo --no-data  # two tools, no data refresh
rhythm update --self-only           # rhythm alone
```

| Flag | Effect |
|---|---|
| `--self-only` | update only rhythm |
| `--no-data` | skip the tools' data refresh |
| `--quiet` | print only updates, problems and the tools' own output: no `(up to date)` lines, no progress |
| `--min-age age` | skip releases younger than `age`; see [below](#hold-back-new-releases) |
| `--to version` | install that version of one tool; see [A chosen version](#a-chosen-version) |
| `--rollback` | go back to the binary the last update replaced; see [Roll back](#roll-back) |
| `--lock` | move `rhythm.lock` forward instead of the binaries; see [Pinning and locking](Pinning-And-Locking#move-the-lock-forward) |
| `--skip-signature`, `--cache` | as for [`rhythm install`](Command-Reference#install) |

A name that isn't installed counts as a failure: `rhythm-echo        isn't installed; install it with: rhythm install echo`. A misspelt one stops before any request: `rhythm: unknown tool "tunr" (did you mean tuner?)`.

After a run without failures, rhythm mentions a default tool you haven't installed, once per machine: `New tool available: echo (rhythm install echo)`.

### Only check

```console
$ rhythm update --check
rhythm             dev (development build, not updated)
rhythm-echo        1.0.0 (up to date)
rhythm-soundcheck  1.1.0 (up to date)
rhythm-tuner       0.3.1 → 0.3.2 available
$ echo $?
10
```

`--check` reports what's out of date and changes nothing. It doesn't refresh tool data either. It exits `0` when everything is current, `10` when an update is available and `1` when a check failed, so in a script `if rhythm update --check` reads as "if everything is current". `--json` prints the report as [`rhythm-update/v1`](Command-Reference#json-output), with the same exit codes and the human lines on stderr.

### Development builds and other installs

Binaries built from source or with `go install` report their version as `dev`. `rhythm update` lists them as `(development build, not updated)` and leaves them alone. Update those with `go install …@latest` or by rebuilding, or replace one with a release: `rhythm install tuner --force`.

A tool whose version can't be read is left alone too: `rhythm-echo        version unknown (rhythm-echo --version printed no version number), not updated`. A copy installed by a package manager is left to it, with the command to use:

```text
rhythm-tuner is managed by Homebrew; update it with: brew upgrade rhythm-tuner
rhythm-tuner is managed by mise; update it with: mise upgrade
rhythm-tuner is managed by Scoop; update it with: scoop update rhythm-tuner
```

Homebrew, Nix, mise, asdf, Scoop, WinGet, any `shims` folder and links to a program of another name are recognized. Its data is still refreshed. A plain link to a binary of the same name (`bin/rhythm-tuner -> ../opt/rhythm-tuner`) is followed and its target updated.

## Hold back new releases

`--min-age` (or `RHYTHM_MIN_RELEASE_AGE` for every run) passes over releases younger than the age given, for rhythm and the tools alike. A release pulled in its first days then never reaches you.

```console
$ rhythm update --check --min-age 3w tuner
rhythm-tuner 0.3.2 is 28 hours old; staying on 0.3.1 (min age 3w)
```

When an older release in between is old enough, `rhythm update` takes that one: `rhythm-tuner 0.3.3 is 1 day old; updating to 0.3.2 (min age 3d)`. Ages are `3d`, `1w`, `36h`, `1w2d` and the like; [Pinning and locking](Pinning-And-Locking#wait-before-taking-a-release) has every form and message.

## A chosen version

`--to` installs one version of one tool, an older one too:

```console
$ rhythm update tuner --to 0.3.1
Note: 0.3.1 is older than rhythm-tuner 0.3.2; installing it as asked.
rhythm-tuner       0.3.2 → 0.3.1 ✔
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.1 (release-go.yml)
  Checksum ok   sha256:246fbc4086518bfc6779b18a1adbbc1fb891daba86a50f65e96b876c10e9f431
Checks: pack 2026.10.24-084900, 37 checks (commit 78c563e)
Nothing cached yet. Vulnerability data is fetched during scans and refreshed daily.
```

The version can be exact or partial (`--to 0.3` for the newest 0.3.x); an exact one ignores `--min-age`. The tool's data is refreshed as usual. A development build or a tool whose version can't be read fails with `--to`, with the way out: `rhythm-tuner       dev (development build, not updated); replace it with: rhythm install tuner@0.3.1 --force`. To pin versions for a whole project, use [`rhythm.lock`](Pinning-And-Locking).

## Roll back

Every update keeps the binary it replaced next to the new one, as `<binary>.prev` with a `<binary>.prev.json` that records both versions and their sha256. `--rollback` puts it back:

```console
$ rhythm update tuner
rhythm-tuner       0.3.1 → 0.3.2 ✔
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
Checks: pack 2026.10.24-084900, 37 checks (commit 78c563e)
Nothing cached yet. Vulnerability data is fetched during scans and refreshed daily.
$ rhythm update --rollback tuner
Rolled back rhythm-tuner 0.3.2 → 0.3.1
$ rhythm update --rollback tuner
Rolled back rhythm-tuner 0.3.1 → 0.3.2
```

The two files swap, so a second rollback undoes the first. No data refresh runs and nothing is downloaded. It works for rhythm itself too (`rhythm update --rollback rhythm`), once a `rhythm update` has replaced it; the installers keep no `.prev`.

| Message | Exit |
|---|---|
| `rhythm-echo has no previous version to roll back to; rhythm keeps one after each update` | 1 |
| `rhythm-tuner       /home/me/.local/bin/rhythm-tuner.prev has changed since it was set aside (sha256 differs); not rolling back` | 1 |
| `rhythm-tuner is managed by Homebrew; rhythm leaves its versions to Homebrew` | 1 |
| `rhythm-echo        isn't installed; there is nothing to roll back` | 1 |
| `Rolled back rhythm-tuner to the binary its last update replaced` | 0, when the `.prev.json` is missing |

`rhythm uninstall` removes the `.prev` files with the binary.

## How updates are verified

Every binary update, every `rhythm install` and `rhythm lock` go through the same steps:

1. Read the release from the API: `/releases/latest` (which never returns a prerelease), the exact tag for a pinned version, or the release list for a partial version or `--min-age`. An answer whose tag isn't the one asked for is refused.
2. Download `checksums.txt` and its Sigstore signature, `checksums.txt.sigstore.json`.
3. **Check the signature.** It must come from the shared `release-go.yml` workflow, for the tool's own repository and the tag `v<version>`. A missing or invalid signature installs nothing and leaves the old binary in place.
4. Download the archive for your platform, named `<binary>_<os>_<arch>.tar.gz` (`.zip` on Windows), into a temporary file next to the old binary, hashing it on the way. With `--cache` a verified copy of the same archive is reused instead.
5. Compare the archive's SHA-256 with its entry in `checksums.txt` (and with `rhythm.lock` for `install --locked`). If the release has no `checksums.txt`, has no entry for the archive, or the hash differs, the update stops and nothing is changed.
6. Keep the old binary as `<binary>.prev`, then rename the new one over it. A rename is atomic, so a failed or interrupted update never leaves a broken install. Ctrl-C exits 130 with `rhythm: update interrupted` and removes the temporary files.

After a verified update, the `Signature ok` and `Checksum ok` lines say who signed the release and which archive went in. `rhythm list` then shows `yes` under SIGNED. A refused release reads:

```text
rhythm-tuner       refusing to install RhythmShahriar/tuner 0.4.0: signature check failed: <reason>; nothing was changed. The release may have been tampered with, or a mirror re-packed it. Only if you trust where it comes from, rerun with --skip-signature
```

`--skip-signature` installs without the signature check and says so first: `warning: --skip-signature: release signatures aren't checked, so a tampered release would install; use it only with a mirror you trust`. The checksums are still checked. See [Verifying releases](Verifying-Releases) for what the signature proves and how to check it yourself.

On Windows a running `.exe` can't be overwritten, so the old binary is first renamed to `<name>.exe.old` and the new one moved into its place; if that fails, the `.old` is put back, so a working binary is always there. After a good update the `.old` becomes the `.prev`.

If the folder isn't writable, for example `/usr/local/bin` after a root install, you'll see `can't write to <dir>; rerun with sudo, or install to a folder you own with --dir`. Updates need to write there from the start of the download.

### Network problems

A host that can't be reached, a rate limit or a mirror without the release is said once for every binary it hit:

```text
can't reach api.github.com for rhythm-echo, rhythm-soundcheck, rhythm-tuner; check network, proxy or RHYTHM_GITHUB_API
Updated 0, up to date 0, failed 3: rhythm-echo, rhythm-soundcheck, rhythm-tuner
```

On a shared CI runner, set `GITHUB_TOKEN` to lift GitHub's limit of 60 release lookups an hour: `GitHub rate limit reached for <binaries>; set GITHUB_TOKEN (resets 14:05)`. [Release API and tokens](Command-Reference#release-api-and-tokens) has every message.

### Environment

| Variable | Effect |
|---|---|
| `RHYTHM_GITHUB_API` | GitHub API base for release lookups, for a mirror or GitHub Enterprise. Default `https://api.github.com`. It must be `https` (plain `http` only for `127.0.0.1`, `::1` and `localhost`) and a plain base URL. |
| `GITHUB_TOKEN` | Sent to GitHub only, to avoid its anonymous rate limit of 60 requests an hour (useful in CI). Never sent to a mirror. |
| `RHYTHM_GITHUB_TOKEN` | Sent to the `RHYTHM_GITHUB_API` host. When set, `GITHUB_TOKEN` isn't sent anywhere. |
| `RHYTHM_OFFLINE` | Any value but empty or `0`: no network requests; updates fail with `RHYTHM_OFFLINE is set, so nothing was downloaded for <binaries>; unset it to reach the release API`. |
| `RHYTHM_MIN_RELEASE_AGE` | The default for `--min-age`. |
| `RHYTHM_DOWNLOAD_CACHE` | The default for `--cache`. |

The full list is in the [Command reference](Command-Reference#environment).

## The "new release" notice

When a newer Tuner, Soundcheck or Echo release is out, a scan, a check, or an Echo snapshot, compare or run ends with a line on stderr:

```text
A new release of rhythm-tuner is available: 0.3.0 → 0.3.1
  run: rhythm update
```

- Each tool asks GitHub at most **once a day**, in the background while it runs. It never delays the run for it. The answer is remembered in `update-check/rhythm-tuner.json` (or `rhythm-soundcheck.json`, `rhythm-echo.json`) under your config directory (`~/.config/rhythm` on Linux).
- The notice only appears with the terminal view. It doesn't appear with `--quiet`.
- It never installs anything. Rhythm doesn't update binaries silently; you run `rhythm update` when you choose to.

To turn the check off, set one of these:

| Variable | Effect |
|---|---|
| `RHYTHM_NO_UPDATE_CHECK=1` | No release check, no notice. |
| `CI` (any value) | Same. Most CI systems set it for you. |
| `RHYTHM_OFFLINE` (any value but `0`) | Same, for tools on a kit that knows it; it also stops their other downloads. |

Development builds never check.

## Vulnerability data

Tuner matches versions against data from WPVulnerability, the wordpress.org API and php.net. It matches Composer packages against OSV.dev. Every lookup is cached on disk and **refreshed automatically once it's more than 24 hours old**, the next time a scan needs it. If the network is down, Tuner uses the cached copy and says so. See [Known Vulnerabilities](Tuner-Known-Vulnerabilities) for the details.

You don't need to do anything to keep it current. To refresh it right away:

```sh
rhythm tuner update
```

This refreshes the check pack, then re-fetches every cached WPVulnerability, wordpress.org and php.net record, ignoring their age. If a fetch fails, the cached copy is kept and the command exits `3`. When nothing is cached yet, it says so: data is fetched during scans.

To delete the cached check pack and vulnerability data:

```sh
rhythm tuner update --clear
```

```text
Cleared /home/you/.cache/rhythm/tuner/checks
Cleared /home/you/.cache/rhythm/tuner/vulns
```

The next scan downloads what it needs again.

## Check packs

New Tuner checks reach you without a new binary. Tuner looks for a newer check pack at most once a day during scans, verifies it against its checksums and uses it when it's newer than the checks built into your binary. `rhythm tuner update` checks for one immediately. Any problem with a pack prints a warning and the scan carries on with cached or built-in checks.

Use `--no-check-updates` to scan with the built-in checks only, or `--offline` to use only what's cached. How packs are built, verified and trusted is on [Check Packs](Tuner-Check-Packs).

## Soundcheck's browser

Soundcheck uses an installed Chrome or Chromium when there is one; those update with your system. Otherwise it downloads the Chromium build its version is pinned to, once, into your cache folder. `rhythm soundcheck update` (which `rhythm update` runs for you) makes sure that browser is there:

- With Chrome installed, it reports what it found:

  ```text
  Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
  ```

- It also deletes Chromium it downloaded earlier and no longer needs: every downloaded revision when an installed Chrome is used, or the revisions other than the pinned one. Each prints `Removed unused Chromium <folder>`.
- Without Chrome, it downloads the pinned Chromium if it's missing (progress on stderr) and ends with `Browser: Chromium at <path>`.

A new Soundcheck release may pin a newer Chromium. `rhythm update` then downloads it and removes the old one in the same step; if a check downloads it first, the old revision stays until the next `update`.

To delete the downloaded Chromium:

```sh
rhythm soundcheck update --clear
```

```text
Cleared /home/you/.cache/rhythm/soundcheck/browser
```

The next run downloads it again if no Chrome is installed. Soundcheck has no other cached data: snapshots and reports live in your project's `.soundcheck/` folder and are never touched by `update`.

## Echo

Echo keeps no downloaded data: the services it recognizes, its fakes and its checks are built into the binary, so `rhythm update` updating `rhythm-echo` is all it needs. `rhythm echo update` exists so `rhythm update` can call every tool the same way; it changes nothing and exits `0`:

```text
$ rhythm echo update
Echo has nothing to refresh; provider recognizers update with the binary (rhythm update).
```

Echo's connectors update separately, with the site: the WordPress mu-plugin by running `rhythm echo install wordpress` again after updating Echo (it replaces its own file), the Laravel package with `composer update rhythmsh/echo-laravel` and the Node package with `npm update @rhythmsh/echo`. The proxy's CA renews itself when it has less than 30 days left; clients then have to trust the new one.

Snapshots and runs in `.echo/` are never touched by an update.

---
<sub>← [Install](Install) · [Home](Home) · [Uninstall](Uninstall) →</sub>
