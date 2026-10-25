Every Tuner command and flag, its exit codes, the environment variables Tuner and the rhythm installer read and where files are kept. Verified against rhythm-tuner 0.4.0.

Tuner is the `rhythm-tuner` binary. `rhythm tuner …` (or `rhythm t …`) runs it, as does `rhythm-tuner …` directly. This page uses the `rhythm tuner` form; help and messages name the command the way you started it, so with `go install` they say `rhythm-tuner`.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm tuner` | | Interactive picker in a terminal; prints help otherwise. See [below](#the-picker). |
| `rhythm tuner scan <target>... [flags]` | `s` | Scan sites (URLs or hostnames) or site folders. |
| `rhythm tuner checks [flags]` | `ls` | List the checks a scan with the same flags would run; `checks new`, `test`, `lint` and `export` work on check files. |
| `rhythm tuner update` | | Refresh the cached vulnerability data now, or `--prefetch` it for sites, or `--clear` the cache. |
| `rhythm tuner diff <old.json> <new.json>` | | Compare two JSON reports: what is new and what was fixed. |
| `rhythm tuner explain <id>` | | Show a check's full fix offline. |
| `rhythm tuner fix <report.json>... --server <name>` | | Build one nginx or Apache config from reports. |
| `rhythm tuner init ci --target <url>` | | Write a GitHub Actions workflow that runs Tuner. |
| `rhythm tuner report <report.json>... -o <file>` | | Render saved JSON reports as HTML, Markdown or SARIF. |
| `rhythm tuner serve [--listen <addr>]` | | Run the HTTP API: ownership checks, queued scans, webhooks. |
| `rhythm tuner watch -l <file> --every <interval>` | | Scan a site list on a schedule and say what changed. |
| `rhythm tuner version` | `-v`, `--version` | Print the version: `rhythm-tuner 0.4.0`. |
| `rhythm tuner help` | `-h`, `--help` | Print the help. Every command has its own: `rhythm tuner scan --help`, `rhythm tuner watch --help` and so on. |

Flags may come before or after targets. List flags (`--tags`, `--id`…) take comma-separated values and can be repeated. Severities, profiles and `--min-confidence` accept any case.

Typos get a suggestion and exit 2:

```text
$ rhythm tuner scna 127.0.0.1:20400
rhythm tuner: unknown command scna; did you mean scan?
  run 'rhythm tuner --help' for the commands
$ rhythm tuner scan 127.0.0.1:20400 --jsn
rhythm tuner scan: unknown flag --jsn; did you mean --json?
$ rhythm tuner scan 127.0.0.1:20400 --fail-on hihg
rhythm tuner scan: --fail-on hihg: want critical, high, medium, low or info; did you mean high?
$ rhythm tuner scan 127.0.0.1:20400 --id leak-dotnev
rhythm tuner scan: --id leak-dotnev: no check has this ID; did you mean leak-dotenv?
  list them with: rhythm tuner checks
$ rhythm tuner scan 127.0.0.1:20400 --timeout 5
rhythm tuner scan: --timeout 5: use a unit, for example 5s
```

### The picker

Run `rhythm tuner` with no arguments in an interactive terminal to get a short form:

1. **Which site should Tuner scan?** A URL, a hostname or a site folder on this machine. The form reminds you: "Only scan sites you own or are authorised to test."
2. **Profile:** Quick or Default (preselected).
3. **Report:** terminal only, or also a file in the current folder: `tuner-report.md`, `tuner-report.json` or `tuner-report.sarif`.

It then runs `rhythm tuner scan <site> --profile <profile> [-o tuner-report.<ext>]`. Ctrl+C cancels (exit 130). When stdin or stdout isn't a terminal, `rhythm tuner` prints the help and exits 0.

## scan

```sh
rhythm tuner scan <url>... [flags]
rhythm tuner scan <dir> [flags]
```

**Targets.** A target is a **local folder** when it is `.`, `..` or `~`, starts with `./`, `../`, `.\`, `/`, `\`, `~/`, `~\` or a drive letter (`C:\`), or holds a path separator and its first part doesn't look like a host. So `sites/shop` is a folder; `example.com/blog` and `localhost:8080/wp` are sites. A bare name that is also a folder in the current directory is refused rather than guessed:

```text
$ rhythm tuner scan example.com
rhythm tuner scan: example.com is both a folder here and a hostname; use ./example.com or https://example.com
```

Sites without a scheme are tried over `https://` first, then `http://` (see [Requests](Tuner-Requests#targets-without-a-scheme) for when that fallback is refused). A folder that is a link, such as a deploy's `current`, is resolved first.

**Duplicates** are scanned once: scheme and host are compared in lower case, default ports and a trailing `/` are ignored and a bare host counts as the same site with a scheme. Local folders are compared by absolute path.

### Output

| Flag | Default | Description |
|---|---|---|
| `--json` | off | JSON report on stdout; an array when scanning several targets, with failed targets as `{"target": …, "error": …}` entries. |
| `--json-array` | off | JSON that is always an array, also for one target. Selects JSON on its own. |
| `--jsonl` | off | One JSON finding per line, streamed as found. |
| `--sarif` | off | SARIF 2.1.0 for GitHub code scanning. |
| `--md` | off | Markdown for issues and pull requests. |
| `-o`, `--output <file>` | none | Write the report to a file and keep the live view. The format comes from the format flag, else the extension: `.json`, `.jsonl`/`.ndjson`, `.sarif`, `.md`/`.markdown`, `.html`/`.htm`. An existing file is replaced only once the new report is complete. |
| `-q`, `--quiet` | off | Findings and a one-line summary only. |
| `--plain` | off | No color, boxes or glyphs. Used automatically when stdout isn't a terminal. |
| `--no-color` | off | No color. `NO_COLOR` works too. |
| `-v`, `--verbose` | off | Log every request to stderr. (`rhythm tuner -v` alone prints the version.) |

See [Output and reports](Tuner-Output).

### Check selection

| Flag | Default | Description |
|---|---|---|
| `--profile <name>` | `default` | `quick` or `default`. `quick` also skips plugin and theme version lookups. `deep` is still accepted and runs the same checks as `default`. |
| `--tags <list>` | all | Only checks with any of these tags, e.g. `leak,wordpress`. |
| `--exclude-tags <list>` | none | Skip checks with any of these tags. |
| `--id <list>` | all | Only these checks or findings; globs allowed, e.g. `'leak-*'`. Ignores the profile. |
| `--exclude-id <list>` | none | Skip these checks or findings. |
| `--severity <level>` | `info` | Skip checks and findings below this severity. |
| `--checks <dir>` | built-in | Load checks from a directory instead. |
| `--no-check-updates` | off | Use the built-in checks, never a [check pack](Tuner-Check-Packs). |
| `--no-vulns` | off | Skip [known-vulnerability matching](Tuner-Known-Vulnerabilities). |
| `--offline` | off | Use cached vulnerability data, checksums and check pack only; skip the release check too. |

An `--id` that names no check gets a did-you-mean; vulnerability and local IDs (`vuln-*`, `local-*`, `php-eol`…) are accepted as listed in the [Finding reference](Tuner-Finding-Reference). See [Choosing checks](Tuner-Choosing-Checks).

### Requests

| Flag | Default | Description |
|---|---|---|
| `--rate <n>` | `5`, or `50` for localhost and private addresses | Max requests per second to each host, shared by every target on that host. `0` is no limit until the site throttles. Halved after a `429` or `503`. |
| `-c`, `--concurrency <n>` | `10` | Requests in flight per target, for every part of the scan. |
| `--timeout <d>` | `10s` | Per-request timeout, for the site and the vulnerability data services. Needs a unit: `500ms`, `30s`. |
| `--retries <n>` | `2` | Retries after network errors or a `429`/`503` reply. |
| `--max-time <d>` | none | Stop the whole run after this long; results are partial. |
| `-A`, `--user-agent <ua>` | `rhythm-tuner/<version> (+https://rythm.sh/tuner)` | User-Agent header. |
| `-H`, `--header <h>` | none | Extra header, `"Name: value"`. Repeatable. Sent only to the host you named. |
| `--cookie <c>` | none | Cookie header. Sent only to the host you named. |
| `--proxy <url>` | `HTTPS_PROXY` / `HTTP_PROXY` | `http://`, `https://`, `socks5://` or `socks5h://` proxy. Also used for data downloads. |
| `-k`, `--insecure` | off | Don't verify TLS certificates. Certificate problems are still reported. |
| `--follow-host` | off | Keep scanning when the home page redirects to another host. Your headers and cookies still go only to the host you named. |
| `--plugins <mode>` | `passive` | `passive` reads the REST API index (one request); `deep` also checks `readme.txt` of the 1,500 most installed plugins. |
| `--request-budget <n>` | `2000` | Most requests plugin discovery may send. A budget you set makes the scan partial when it runs out. |

See [Requests and politeness](Tuner-Requests).

### Local scans

| Flag | Default | Description |
|---|---|---|
| `--php <binary>` | `php` on `PATH` | The PHP to ask for its version and session settings. Asked only for folders that look like PHP projects, unless you pass `--php`. |
| `--integrity` | off | Compare WordPress core and wordpress.org plugin files with WordPress.org's checksums. Needs a folder target; missing checksums make the scan partial. |
| `--discover` | off | Scan every WordPress and Laravel install under each folder target, each with its own report (6 folders deep). |

Request flags don't apply to local scans. See [Local scans](Tuner-Local-Scans) and [Integrity and malware](Tuner-Integrity-And-Malware).

### Batch and CI

| Flag | Default | Description |
|---|---|---|
| `-l`, `--list <file>` | none | Read targets from a file, one per line. Blank lines and `# comments` are ignored. |
| `--stdin` | off | Read targets from stdin, in the same format. |
| `--parallel <n>` | `4` | Targets scanned at once. |
| `--fail-on <level>` | none | Exit 1 if any finding is at or above this severity. |
| `--min-confidence <c>` | `likely` | `confirmed` makes `--fail-on` and `--fail-on-new` skip *likely* findings. Every finding is still reported. |
| `--baseline <file>` | none | Hide findings already in this JSON report. |
| `--allow-missing-vulns` | off | Let `--fail-on` and `--fail-on-new` pass a site whose vulnerability data couldn't be fetched (otherwise exit 3). |
| `--allow-partial` | off | Let them pass a scan that was blocked, cut short or missing `--integrity` checksums (otherwise exit 3). |
| `--config <file>` | `.tuner.yml` | Read settings from this file instead. |
| `--no-config` | off | Don't read `.tuner.yml`; use it in folders you don't trust. |

See [Batch scans](Tuner-Batch-Scans), [CI and code scanning](Tuner-CI) and [Config file](Tuner-Config).

### Changes and client reports

| Flag | Default | Description |
|---|---|---|
| `--since <file>` | none | Compare with an earlier JSON report and list what is new and what was fixed. |
| `--fail-on-new <level>` | none | Exit 1 if a new finding is at or above this severity. Needs `--since`. |
| `--notify <url>` | none | Send new findings to `slack://…`, `discord://…`, `webhook+https://…` or `smtp://…`. Repeatable; needs `--since`. |
| `--notify-min <level>` | `high` | Lowest severity to send. |
| `--notify-all-first` | off | With no earlier report yet, send every finding. |
| `--notify-strict` | off | Exit 3 when a notification can't be sent. |
| `--html <file>` | none | Also write a self-contained HTML report. |
| `--brand-name <name>` | none | Your agency's name on the HTML report. |
| `--brand-logo <file>` | none | Your logo on the HTML report: PNG, JPEG or SVG, up to 512 KB. |

See [Changes and notifications](Tuner-Changes-And-Notifications).

## checks

```text
rhythm tuner checks [flags]
rhythm tuner checks new <id> [dir]
rhythm tuner checks test [dir]
rhythm tuner checks lint [dir]
rhythm tuner checks export --nuclei <dir>
```

`checks` prints a table of checks (ID, severity, profile, tags, name), which check set is in use and the count. Unlike `scan`, it lists every check unless `--profile` picks one. It takes `--profile`, `--tags`, `--exclude-tags`, `--id`, `--exclude-id`, `--severity`, `--checks`, `--offline`, `--no-check-updates` and `--json`. `--tags secrets` also lists the secret rules every check applies.

| Subcommand | |
|---|---|
| `checks new <id> [dir]` | Write `<id>.yaml` and `<id>_test.yaml` to fill in. Never replaces a file. |
| `checks test [dir]` | Run checks (default: the built-in ones) against their fixtures on 127.0.0.1. Exits 1 when a check fails. |
| `checks lint [dir]` | Check checks against the rules built-in checks follow. Exits 1 when a rule is broken. |
| `checks export --nuclei <dir>` | Write the checks as Nuclei templates, with `--id`, `--tags`, `--checks`, `--offline` and `--no-check-updates` to choose them. |

None of them need Go. See [Writing checks](Tuner-Writing-Checks) and [Choosing checks](Tuner-Choosing-Checks#preview-a-selection).

## update

```text
rhythm tuner update                       refresh everything cached
rhythm tuner update --prefetch <target>...
                                          cache what scans of these sites need
rhythm tuner update --clear               delete the cached check pack and data
```

| Flag | |
|---|---|
| `--prefetch <dir\|url>` | Fingerprint each target (no checks run) and cache the vulnerability data a scan of it needs, so a later `--offline` scan has it. More targets may follow. |
| `--php <binary>` | For local prefetch targets. |
| `--clear` | Delete the cached check pack, vulnerability data and checksums. |

```text
$ rhythm tuner update
Checks: built-in (rhythm-tuner 0.4.0)
Refreshed 7 vulnerability records
Refreshed 6 Composer advisory records
Data: WPVulnerability (wpvulnerability.com, CC0); OSV.dev (GitHub Advisory Database, FriendsOfPHP)
Cache: /home/you/.cache/rhythm/tuner/vulns
$ rhythm tuner update --prefetch /var/www/app 127.0.0.1:20400
Checks: built-in (rhythm-tuner 0.4.0)
Prefetched /var/www/app: PHP 8.3.36, 2 Composer packages
Prefetched 127.0.0.1:20400: WordPress 6.9.3, 2 plugins, 1 theme, PHP 8.1.2
Data: WPVulnerability (wpvulnerability.com, CC0); OSV.dev (GitHub Advisory Database, FriendsOfPHP)
Cache: /home/you/.cache/rhythm/tuner/vulns
```

`update` exits 3 when a record or prefetch target fails (cached copies are kept). `rhythm update` runs `rhythm tuner update` for you after updating the binaries. See [Known vulnerabilities](Tuner-Known-Vulnerabilities#caching-and-automatic-refresh).

## diff, report, explain and fix

| Command | Flags | Exit codes |
|---|---|---|
| `diff <old.json> <new.json>` | `--json`, `--md`, `--fail-on-new <level>` | 0 ok, 1 new findings at `--fail-on-new`, 2 usage error, 3 a report couldn't be read |
| `report <report.json>...` | `--html`, `--md`, `--sarif`, `--json`, `-o <file>`, `--since <file>`, `--brand-name`, `--brand-logo` | 0 ok, 2 usage error, 3 a report couldn't be read or written |
| `explain <id>` | `--json`, `--checks <dir>`, `--no-check-updates` | 0 ok, 2 no such check or rule |
| `fix <report.json>...` | `--server nginx\|apache` (required), `--dry-run`; `-` reads a report from stdin | 0 ok, 2 usage error, 3 a report couldn't be read |

`diff` and `report` are covered in [Changes and notifications](Tuner-Changes-And-Notifications), `explain` in the [Finding reference](Tuner-Finding-Reference) and `fix` in [Output and reports](Tuner-Output#server-config-from-a-report). `explain` never goes online.

## init ci, watch and serve

| Command | See |
|---|---|
| `init ci [--target <url\|dir>] [--fail-on <level>] [--sarif] [--force]` | [GitHub Action](Tuner-GitHub-Action#write-the-workflow-init-ci) |
| `watch -l <file> --every <interval>` or `--at <HH:MM>` or `--once` | [Watch and serve](Tuner-Watch-And-Serve#watch-scheduled-scans-with-a-digest) |
| `serve [--listen <addr>] [--data <dir>]` | [Watch and serve](Tuner-Watch-And-Serve#serve-an-http-api-with-ownership-checks) |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done, with nothing at `--fail-on` or new at `--fail-on-new` (or neither set). |
| `1` | A finding at or above `--fail-on`, or a new finding at or above `--fail-on-new`. |
| `2` | Usage error: unknown command or flag, bad value, no target, an ID that names no check, a bad config file. |
| `3` | A target couldn't be scanned (other targets' reports are still written), a report couldn't be written, `update` failed to refresh something, a notification failed under `--notify-strict`, or `--fail-on`/`--fail-on-new` found nothing but couldn't trust the scan: missing vulnerability data or a partial scan (see below). |
| `130` | Interrupted (Ctrl+C); partial results are written. |

When several apply, `130` wins over `1` and `1` wins over `3`. A run stopped by `--max-time` exits `3`, or `1` if a gate is met, with partial results.

**Gates and partial scans.** With `--fail-on` or `--fail-on-new` and no finding at the level, the scan still exits 3 when it can't be trusted, one line per report on stderr:

```text
rhythm tuner scan: /var/www/shop: the scan is incomplete (some checksums for --integrity could not be loaded), so --fail-on critical can't pass it
  add --allow-partial to accept a partial scan
```

The reason in brackets is one of `stopped by --max-time`, `interrupted`, `blocked by Cloudflare challenge` (or another firewall), `the home page did not load`, `stopped at the request budget` or `some checksums for --integrity could not be loaded`. Missing vulnerability data reads `no vulnerability data, so --fail-on high can't pass it` with the hint `rerun when the data source is back, or add --allow-missing-vulns`.

**Ctrl+C.** The first one prints `rhythm tuner scan: stopping... press Ctrl+C again to quit now` and stops the scan with partial results. A second one ends the process at once.

## Environment variables

### Tuner

| Variable | Description |
|---|---|
| `TUNER_VULN_API` | Base URL of a WPVulnerability mirror. Default `https://www.wpvulnerability.net`. |
| `TUNER_WPORG_API` | Base URL of an api.wordpress.org mirror (latest releases, core checksums). Default `https://api.wordpress.org`. |
| `TUNER_WPDL_API` | Base URL of a downloads.wordpress.org mirror (plugin checksums for `--integrity`). Default `https://downloads.wordpress.org`. |
| `TUNER_PHPNET_API` | Base URL of a php.net mirror (release states). Default `https://www.php.net`. |
| `TUNER_OSV_API` | Base URL of an OSV.dev mirror, for Composer packages. Default `https://api.osv.dev`. |
| `TUNER_WORDFENCE_KEY` | A free Wordfence Intelligence API key: plugins, themes and core are also matched against its feed. Unset, nothing goes to Wordfence. |
| `TUNER_WORDFENCE_API` | Base URL of a Wordfence Intelligence mirror. Default `https://www.wordfence.com/api/intelligence/v3`. |
| `TUNER_CHECKS_URL` | Base URL of a mirror of the `checks-latest` release assets. |
| `TUNER_CHECKS_INSECURE` | `1` skips the check pack signature check (checksums still apply), for a pack you build yourself. Every run with it prints a warning. |
| `TUNER_SMTP_PASSWORD` | The password for a `--notify smtp://…` destination. |
| `TUNER_SERVE_TOKEN` | `serve`'s bearer token, at least 16 characters. |
| `TUNER_WEBHOOK_SECRET` | `serve`'s webhook signing secret. |
| `TUNER_*` | Any other `TUNER_` variable can be read by `${TUNER_…}` in [`.tuner.yml`](Tuner-Config#headers-and-environment-variables) headers. |
| `RHYTHM_OFFLINE` | Any value but empty or `0` skips the once-a-day release check, like `--offline` does. It doesn't make scans use cached data only. |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day "new release available" check after scans. |
| `CI` | When set (as CI systems do), the release check is skipped too. |
| `RHYTHM_INVOKED_AS` | Set by the `rhythm` router to the command you typed, so help and hints say `rhythm tuner` or `rhythm t`. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks to avoid GitHub's anonymous rate limit. |
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | Standard proxy settings, used when `--proxy` isn't given. |
| `NO_COLOR` | Disable color, like `--no-color`. |

The mirror variables are covered in [Known vulnerabilities](Tuner-Known-Vulnerabilities#mirrors) and [Check packs](Tuner-Check-Packs#private-packs-and-mirrors).

### rhythm and the installers

| Variable | Used by | Description |
|---|---|---|
| `RHYTHM_INSTALL_DIR` | install scripts | Where to install. Default `~/.local/bin` (or `/usr/local/bin` as root) on Linux and macOS; `%LOCALAPPDATA%\Programs\rhythm` on Windows. |
| `RHYTHM_VERSION` | install scripts | Install a specific rhythm version, e.g. `0.5.0`. Default: latest. |
| `RHYTHM_TOOLS` | install scripts | Tools to install along with rhythm, separated by spaces or commas; `tuner@0.4.0` pins one, `none` installs none. Default: the tools a fresh setup gets (`tuner soundcheck echo`). |
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

Tuner uses your operating system's standard per-user config and cache folders (Go's `os.UserConfigDir` and `os.UserCacheDir`):

| What | Linux | macOS | Windows |
|---|---|---|---|
| Config folder | `$XDG_CONFIG_HOME/rhythm`, usually `~/.config/rhythm` | `~/Library/Application Support/rhythm` | `%AppData%\rhythm` |
| Release check state | `<config>/update-check/rhythm-tuner.json` | same | same |
| First-run notices | `<config>/seen/` | same | same |
| Cache folder | `$XDG_CACHE_HOME/rhythm/tuner`, usually `~/.cache/rhythm/tuner` | `~/Library/Caches/rhythm/tuner` | `%LocalAppData%\rhythm\tuner` |
| Vulnerability data | `<cache>/vulns` (with `osv/` for Composer, `wordfence/` for the feed and `checksums/` for `--integrity`) | same | same |
| Check pack | `<cache>/checks/pack.json` | same | same |
| Watch reports | `<cache>/watch/<name>` | same | same |
| serve data | `<cache>/serve` | same | same |

- **Release check state** remembers when GitHub was last asked about new Tuner releases (at most once a day) and the answer.
- **First-run notices**: the responsible-use note is shown once per machine; the marker file `tuner-authorised-use` records that.
- **Caches** are safe to delete; `rhythm tuner update --clear` deletes the check pack and data. Tuner fetches what it needs again on the next scan.
- **Settings** come from flags, environment variables and an optional [`.tuner.yml`](Tuner-Config) in the current folder.

---
<sub>← [Finding reference](Tuner-Finding-Reference) · [Home](Home) · [Coming from Wordfence CLI or WPScan](Tuner-Coming-From-Wordfence-Or-WPScan) →</sub>
