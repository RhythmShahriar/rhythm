One command keeps `rhythm`, every installed tool and their data current. This page explains what `rhythm update` does, how downloads are verified, the once-a-day release notice, how Tuner refreshes its vulnerability data and checks on its own, how Soundcheck keeps its browser current and what Echo updates.

## Update everything

```sh
rhythm update
```

It works in this order:

1. **`rhythm` itself.**
2. **Every installed tool**, found as `rhythm-<tool>` on your `PATH` (today: `rhythm-tuner`, `rhythm-soundcheck` and `rhythm-echo`).
3. **Each tool's data**, by running `rhythm-<tool> update`. For Tuner that refreshes the [check pack](Tuner-Check-Packs) and the cached vulnerability data; for Soundcheck, the browser it uses (see [Soundcheck's browser](#soundchecks-browser)). Echo has nothing to refresh (see [Echo](#echo)).

```text
rhythm             0.1.0 (up to date)
rhythm-tuner       0.3.0 → 0.3.1 ✔
rhythm-soundcheck  1.0.0 (up to date)
rhythm-echo        1.0.0 (up to date)
Checks: built-in (rhythm-tuner 0.3.1); they're as new as pack 2026.10.23-154747
Refreshed 29 vulnerability records
Data: WPVulnerability (wpvulnerability.com, CC0)
Cache: /home/you/.cache/rhythm/tuner/vulns
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
Echo has nothing to refresh; provider recognizers update with the binary (rhythm update).
```

The `Browser:` line is Soundcheck's own `update`, which checked the browser it uses; the last line is Echo's.

Each binary is updated in place, wherever it is installed. A problem with one target is reported and the rest still run; the command exits `1` if anything failed and `0` otherwise.

### Only check

```sh
rhythm update --check
```

```text
rhythm             0.1.0 (up to date)
rhythm-tuner       0.3.0 → 0.3.1 available
rhythm-soundcheck  1.0.0 (up to date)
rhythm-echo        1.0.0 (up to date)
```

`--check` reports what's out of date and changes nothing. It doesn't refresh tool data either.

### Development builds

Binaries built from source or with `go install` report their version as `dev`. `rhythm update` lists them as `(development build, not updated)` and leaves them alone. Update those with `go install …@latest` or by rebuilding.

## How updates are verified

Every binary update and every `rhythm install` goes through the same steps:

1. Read the tool's latest release from the GitHub API (`/releases/latest`, which never returns a prerelease).
2. Download `checksums.txt` and the archive for your platform, named `<binary>_<os>_<arch>.tar.gz` (`.zip` on Windows).
3. Compare the archive's SHA-256 with its entry in `checksums.txt`. If the release has no `checksums.txt`, has no entry for the archive, or the hash differs, the update stops and nothing is changed.
4. Extract the binary in memory and write it to a temporary file next to the old one.
5. Rename the temporary file over the old binary. A rename is atomic, so a failed or interrupted update never leaves a broken install.

On Windows a running `.exe` can't be overwritten, so the old binary is first renamed to `<name>.exe.old` and the new one moved into its place. The `.old` file is removed on the next update.

> [!TIP]
> The release checksums are signed with keyless cosign. `rhythm update` relies on the checksum file; to check the signature yourself, see [Verifying Releases](Verifying-Releases).

If the directory isn't writable, for example `/usr/local/bin` after a root install, you'll see `can't write to <dir>; rerun with sudo, or reinstall to a directory you own`.

### Environment

| Variable | Effect |
|---|---|
| `RHYTHM_GITHUB_API` | GitHub API base for release lookups, for a mirror or GitHub Enterprise. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release lookups when set, to avoid GitHub's anonymous rate limit of 60 requests an hour (useful in CI). |

## The "new release" notice

When a newer Tuner, Soundcheck or Echo release is out, a scan, a check, or an Echo snapshot, compare or run ends with a line on stderr:

```text
A new release of rhythm-tuner is available: 0.3.0 → 0.3.1
  run: rhythm update
```

- Each tool asks GitHub at most **once a day**, in the background while it runs. It never delays the run for it. The answer is remembered in `update-check/rhythm-tuner.json` (or `rhythm-soundcheck.json`, `rhythm-echo.json`) under your config directory (`~/.config/rhythm` on Linux).
- The notice only appears with the terminal view. It doesn't appear with `--quiet`.
- It never installs anything. Rhythm doesn't update binaries silently; you run `rhythm update` when you choose to.

To turn the check off, set either variable:

| Variable | Effect |
|---|---|
| `RHYTHM_NO_UPDATE_CHECK=1` | No release check, no notice. |
| `CI` (any value) | Same. Most CI systems set it for you. |

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
