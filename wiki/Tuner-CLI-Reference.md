Every Tuner command and flag, its exit codes, the environment variables Tuner and the rhythm installer read and where files are kept. Verified against rhythm-tuner 0.3.1.

Tuner is the `rhythm-tuner` binary. `rhythm tuner …` (or `rhythm t …`) runs it, as does `rhythm-tuner …` directly; this page uses the `rhythm tuner` form.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm tuner` | | Interactive picker in a terminal; prints help otherwise. See [below](#the-picker). |
| `rhythm tuner scan <target>... [flags]` | `s` | Scan one or more sites (URLs or hostnames) or site directories. |
| `rhythm tuner checks [flags]` | `ls` | List the checks a selection includes and where they come from. |
| `rhythm tuner update [--clear]` | | Refresh the check pack and cached vulnerability data now. |
| `rhythm tuner version` | `-v`, `--version` | Print the version: `rhythm-tuner 0.3.1`. |
| `rhythm tuner help` | `-h`, `--help` | Print the help text. `rhythm tuner scan --help` prints the same. |

Flags may come before or after targets: `rhythm tuner scan yoursite.com --json` and `rhythm tuner scan --json yoursite.com` are the same. List flags (`--tags`, `--id`…) take comma-separated values and can be repeated.

### The picker

Run `rhythm tuner` with no arguments in an interactive terminal to get a short form:

1. **Which site should Tuner scan?** A URL, a hostname, or (since v0.3.2) a site folder on this machine for a [local scan](Tuner-Local-Scans). The form reminds you: "Only scan sites you own or are authorised to test."
2. **Profile:** Quick, Default (preselected) or Deep.
3. **Report:** terminal only, or also a file in the current directory: `tuner-report.md`, `tuner-report.json` or `tuner-report.sarif`.

It then runs `rhythm tuner scan <site> --profile <profile> [-o tuner-report.<ext>]`. Press Ctrl+C to cancel (exit code 130). When stdin or stdout isn't a terminal, `rhythm tuner` prints the help and exits 0.

## scan

```sh
rhythm tuner scan <url>... [flags]
rhythm tuner scan <dir> [flags]
```

A target is a **local directory** when it starts with `.`, `/` or `~`, or names an existing directory; anything else is a site. Sites without a scheme are tried over `https://` first, then `http://` if that fails. See [Tuner Local Scans](Tuner-Local-Scans).

### Output

| Flag | Default | Description |
|---|---|---|
| `--json` | off | One JSON report on stdout; an array when scanning several targets. |
| `--jsonl` | off | One JSON finding per line, streamed as found. |
| `--sarif` | off | SARIF 2.1.0 for GitHub code scanning. |
| `--md` | off | Markdown for issues and pull requests. |
| `-o`, `--output <file>` | none | Write the report to a file and keep the live view on the terminal. The format comes from the format flag, else the extension: `.json`, `.jsonl`/`.ndjson`, `.sarif`, `.md`/`.markdown`. |
| `-q`, `--quiet` | off | Findings only. |
| `--plain` | off | No color, boxes or glyphs, one line per finding. Used automatically when stdout isn't a terminal. |
| `--no-color` | off | No color. `NO_COLOR` works too. |
| `-v`, `--verbose` | off | Log every request to stderr. |

Pick one format flag. Without `-o`, a format flag replaces the live view on stdout. See [Tuner Output](Tuner-Output).

### Check selection

| Flag | Default | Description |
|---|---|---|
| `--profile <name>` | `default` | `quick`, `default` or `deep`. `quick` also skips plugin and theme version lookups. |
| `--tags <list>` | all | Only checks with any of these tags, e.g. `leak,wordpress`. |
| `--exclude-tags <list>` | none | Skip checks with any of these tags. |
| `--id <list>` | all | Only these checks or findings; globs allowed, e.g. `'leak-*'`. Ignores the profile. |
| `--exclude-id <list>` | none | Skip these checks or findings; globs allowed. |
| `--severity <level>` | `info` | Skip checks and findings below this severity. |
| `--checks <dir>` | built-in or pack | Load checks from a directory instead. |
| `--no-check-updates` | off | Use the built-in checks, not the latest [check pack](Tuner-Check-Packs). |
| `--no-vulns` | off | Skip [known-vulnerability matching](Tuner-Known-Vulnerabilities). |
| `--offline` | off | Use cached vulnerability data and check pack only; never contact the data services. |

`--id` and `--exclude-id` patterns must match a check, or name vulnerability or local findings (`vuln-*`, `local-*`, `wp-core-*`, `wp-components-*`, `wp-plugin-closed`, `wp-theme-closed`, `php-eol`, `php-version-hidden`, `nginx-eol`, `apache-eol`); otherwise the scan stops with a usage error. See [Tuner Choosing Checks](Tuner-Choosing-Checks) and the [Finding Reference](Tuner-Finding-Reference).

### Requests

| Flag | Default | Description |
|---|---|---|
| `--rate <n>` | `5` | Max requests per second per site. `0` is unlimited. Halved for the rest of the scan after a `429` or `503`. |
| `-c`, `--concurrency <n>` | `10` | Requests in flight per site. |
| `--timeout <d>` | `10s` | Per-request timeout, as a Go duration (`500ms`, `30s`, `1m`). |
| `--retries <n>` | `2` | Retries after network errors or a `429`/`503` reply. |
| `--max-time <d>` | none | Stop the whole run after this long; results are partial. |
| `-A`, `--user-agent <ua>` | `rhythm-tuner/<version> (+https://rythm.sh/tuner)` | User-Agent header. Also used for vulnerability data requests. |
| `-H`, `--header <h>` | none | Extra header, `"Name: value"`. Repeatable. `Host:` sets the virtual host. |
| `--cookie <c>` | none | Cookie header, e.g. `"wordpress_logged_in_x=…"`. |
| `--proxy <url>` | `HTTPS_PROXY` / `HTTP_PROXY` | `http://`, `https://`, `socks5://` or `socks5h://` proxy. Also used for vulnerability data and check packs. |
| `-k`, `--insecure` | off | Don't verify TLS certificates. Certificate problems are still reported. |

See [Tuner Requests](Tuner-Requests).

### Local scans

| Flag | Default | Description |
|---|---|---|
| `--php <binary>` | `php` on `PATH` | The PHP binary to ask for its version and session settings. |

In a local scan, request flags don't apply and HTTP checks don't run.

### Batch and CI

| Flag | Default | Description |
|---|---|---|
| `-l`, `--list <file>` | none | Read targets from a file, one per line. Blank lines and `# comments` are ignored. |
| `--stdin` | off | Read targets from stdin, in the same format. |
| `--parallel <n>` | `4` | Sites scanned at once. Each site has its own rate limit. |
| `--fail-on <level>` | none | Exit 1 if any finding is at or above this severity: `critical`, `high`, `medium`, `low` or `info`. |
| `--baseline <file>` | none | Hide findings already present in this JSON report. |

Duplicate targets are scanned once. See [Tuner Batch Scans](Tuner-Batch-Scans) and [Tuner CI](Tuner-CI).

## checks

```sh
rhythm tuner checks [flags]
```

Prints a table of checks (ID, severity, profile, tags, name), which check set is in use and the count. `rhythm tuner checks --help` lists its flags.

| Flag | Default | Description |
|---|---|---|
| `--profile <name>` | **`deep`** | Unlike `scan`, defaults to `deep`, so it lists every check. |
| `--tags`, `--exclude-tags`, `--id`, `--exclude-id`, `--severity` | | As for `scan`. |
| `--checks <dir>` | | List checks from a directory. |
| `--no-check-updates` | off | List the built-in checks. |
| `--offline` | off | Use the cached check pack only. |
| `--json` | off | Print a JSON array: `id`, `name`, `severity`, `profile`, `tags`, `requires` (when set) and `paths`. |

```sh
rhythm tuner checks --profile quick          # what a quick scan runs
rhythm tuner checks --tags wordpress --json  # machine-readable
```

## update

```sh
rhythm tuner update [--clear]
```

Refreshes the [check pack](Tuner-Check-Packs) and every cached [vulnerability data](Tuner-Known-Vulnerabilities#caching-and-automatic-refresh) record now, ignoring the daily schedule. It also says which checks scans will use. `rhythm update` runs it for you after updating the binaries.

| Flag | Default | Description |
|---|---|---|
| `--clear` | off | Delete the cached check pack and vulnerability data instead. |

```text
$ rhythm tuner update
Checks: built-in (rhythm-tuner 0.3.1); they're as new as pack 2026.10.23-154747
Refreshed 29 vulnerability records
Data: WPVulnerability (wpvulnerability.com, CC0)
Cache: /home/you/.cache/rhythm/tuner/vulns
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done, with no finding at or above `--fail-on` (or `--fail-on` not set). |
| `1` | A finding at or above `--fail-on`. |
| `2` | Usage error: unknown command or flag, bad value, no target, no matching check. |
| `3` | A target couldn't be scanned (other targets' reports are still written), a report couldn't be written, or `update` failed to refresh something. |
| `130` | Interrupted (Ctrl+C); partial results are written. |

When several apply, `130` wins over `1` and `1` wins over `3`. A run stopped by `--max-time` exits `3` (or `1` if `--fail-on` is met), with partial results.

## Environment variables

### Tuner

| Variable | Description |
|---|---|
| `TUNER_VULN_API` | Base URL of a WPVulnerability mirror. Default `https://www.wpvulnerability.net`. |
| `TUNER_WPORG_API` | Base URL of an api.wordpress.org mirror. Default `https://api.wordpress.org`. |
| `TUNER_PHPNET_API` | Base URL of a php.net mirror (release states). Default `https://www.php.net`. |
| `TUNER_OSV_API` | Base URL of an OSV.dev mirror, for Composer packages in local scans. Default `https://api.osv.dev`. |
| `TUNER_CHECKS_URL` | Base URL of a mirror of the `checks-latest` release assets. Default `https://github.com/RhythmShahriar/tuner/releases/download/checks-latest`. |
| `RHYTHM_NO_UPDATE_CHECK` | Set to any value to stop the once-a-day "new release available" check after scans. |
| `CI` | When set (as CI systems do), the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check, for mirrors or GitHub Enterprise. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks and `rhythm update` to avoid GitHub's anonymous rate limit. |
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | Standard proxy settings, used when `--proxy` isn't given. |
| `NO_COLOR` | Disable color, like `--no-color`. |

The mirror variables are covered in [Known Vulnerabilities](Tuner-Known-Vulnerabilities#mirrors) and [Check Packs](Tuner-Check-Packs#mirrors).

### rhythm and the installers

| Variable | Used by | Description |
|---|---|---|
| `RHYTHM_INSTALL_DIR` | install scripts | Where to install. Default `~/.local/bin` (or `/usr/local/bin` as root) on Linux and macOS; `%LOCALAPPDATA%\Programs\rhythm` on Windows. |
| `RHYTHM_VERSION` | install scripts | Install a specific rhythm version, e.g. `0.5.0`. Default: latest. |
| `RHYTHM_TOOLS` | install scripts | Tools to install along with rhythm, separated by spaces or commas; `tuner@0.3.1` pins one, `none` installs none. Default: the tools a fresh setup gets (`tuner soundcheck echo`). |
| `RHYTHM_LOCKED` | install scripts | `1` installs the versions in `./rhythm.lock` instead. |
| `RHYTHM_GITHUB_API` | install scripts, `rhythm` | GitHub API base, for mirrors. Default `https://api.github.com`. Must be `https` (plain `http` only for loopback). |
| `RHYTHM_GITHUB_TOKEN` | install scripts, `rhythm` | Token sent to the `RHYTHM_GITHUB_API` host; wins over `GITHUB_TOKEN`. |
| `RHYTHM_ARCH` | `install.ps1` | CPU architecture to use when Windows reports one the script doesn't recognize (`amd64` or `arm64`). |
| `GITHUB_TOKEN` | install scripts, `rhythm` | Sent to `api.github.com` only, never to a mirror. |
| `RHYTHM_MIN_RELEASE_AGE` | `rhythm` | Skip releases younger than this, such as `3d`; the default for `--min-age`. |

```sh
curl -fsSL https://rythm.sh/install | RHYTHM_INSTALL_DIR=/opt/bin sh
```

See [Install](Install), [Updating](Updating) and the [Command reference](Command-Reference#environment).

## Files and locations

Tuner uses your operating system's standard per-user config and cache directories (Go's `os.UserConfigDir` and `os.UserCacheDir`):

| What | Linux | macOS | Windows |
|---|---|---|---|
| Config directory | `$XDG_CONFIG_HOME/rhythm`, usually `~/.config/rhythm` | `~/Library/Application Support/rhythm` | `%AppData%\rhythm` |
| Release check state | `<config>/update-check/rhythm-tuner.json` | same | same |
| First-run notices | `<config>/seen/` | same | same |
| Vulnerability data cache | `$XDG_CACHE_HOME/rhythm/tuner/vulns`, usually `~/.cache/rhythm/tuner/vulns` | `~/Library/Caches/rhythm/tuner/vulns` | `%LocalAppData%\rhythm\tuner\vulns` |
| Composer (OSV.dev) data | `<vulns cache>/osv` | same | same |
| Check pack cache | `$XDG_CACHE_HOME/rhythm/tuner/checks/pack.json`, usually `~/.cache/rhythm/tuner/checks/pack.json` | `~/Library/Caches/rhythm/tuner/checks/pack.json` | `%LocalAppData%\rhythm\tuner\checks\pack.json` |

- **Release check state** remembers when GitHub was last asked about new Tuner releases (at most once a day) and the answer.
- **First-run notices**: the responsible-use note ("scan only sites you own or are authorised to test") is shown once per machine; the marker file `tuner-authorised-use` records that.
- **Caches** are safe to delete at any time; `rhythm tuner update --clear` does it for you. Tuner refetches what it needs on the next scan.

Tuner doesn't read a config file; every setting is a flag or environment variable.

---
<sub>← [Finding reference](Tuner-Finding-Reference) · [Home](Home) · [FAQ and troubleshooting](Tuner-FAQ) →</sub>
