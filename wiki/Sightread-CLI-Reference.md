Every Sightread command and flag, its exit codes, its error messages, the JSON it writes, the environment variables it reads and where it keeps files. Verified against rhythm-sightread 1.0.0.

Sightread is the `rhythm-sightread` binary. `rhythm sightread …` runs it, as do `rhythm sr …` and `rhythm-sightread …` directly: they're the same command. This page uses the `rhythm sightread` form; messages name the program the way you ran it.

## Commands

| Command | What it does |
|---|---|
| `rhythm sightread` | On a terminal outside CI: shows what it found here and offers what to review. Otherwise prints the usage and exits `0`. |
| `rhythm sightread plugin <slug>... [flags]` | Review a WordPress.org plugin update. |
| `rhythm sightread theme <slug>... [flags]` | Review a WordPress.org theme update. |
| `rhythm sightread composer [<vendor/pkg>...] [flags]` | Review the Composer updates of `composer.lock`. |
| `rhythm sightread npm [<pkg>...] [flags]` | Review the npm updates of the first lockfile found. |
| `rhythm sightread all [<path>] [flags]` | Review every update a site or project has, or every site of a fleet file with `--sites`. |
| `rhythm sightread changes [--base <ref>] [flags]` | Review every package a lockfile change adds or moves. |
| `rhythm sightread explain <signal-id>` | What a signal means and how to review it. No network. |
| `rhythm sightread explain <report.json> <n\|fingerprint>` | One finding of a saved report, with its evidence and diff. |
| `rhythm sightread allow <eco:name@version>... --reason <text>` | Record a reviewed version. |
| `rhythm sightread allow --from <report.json> --reason <text>` | Record every update a saved report reviewed. |
| `rhythm sightread allow list \| remove <ref> \| import <file>` | List, remove or merge allow entries. |
| `rhythm sightread watch -l <sites.yaml> --every <d> \| --at <HH:MM>` | Review sites on a schedule and notify what is new. |
| `rhythm sightread report <report.json>... -o <file>` | Render saved JSON as Markdown, HTML, SARIF or merged JSON. No network. |
| `rhythm sightread verify-report <report.json>` | Check a signed report. |
| `rhythm sightread init [ci] [--force]` | Write `.sightread/config.yaml`, or with `ci` a GitHub workflow. |
| `rhythm sightread doctor [--offline] [--json]` | Check sources, npm keys, cache, token, git and helpers. |
| `rhythm sightread demo [--keep]` | Review three built-in updates from a server on 127.0.0.1. No network. |
| `rhythm sightread signals [--json]` | List every signal with its default severity. |
| `rhythm sightread cache prune [--older <d>]` | Remove cached archives nothing uses. |
| `rhythm sightread update [--json] [--pin ...]` | Refresh the signal and name data. Never updates a package. |
| `rhythm sightread version [--json]` | Print the version. |
| `rhythm sightread help [command]` | Print the help, or one command's. |

`rhythm sightread <command> --help` (or `help <command>`) prints only that command: its usage, its flags in groups, examples and, for review commands, the hosts it contacts. A command refuses a flag it doesn't read, before anything runs:

```text
$ rhythm sightread plgin b01-widget
rhythm sightread: unknown command "plgin"
  did you mean: rhythm sightread plugin
  run 'rhythm sightread help' to see the commands
```

## Review commands

`plugin`, `theme`, `composer`, `npm`, `all` and `changes` share most flags. The groups below follow the help pages. A flag a command doesn't take is left off its help page and refused with exit `2`; for example `changes` takes no `--to`, `--from`, `--wp`, `--from-tuner`, `--allow-downgrade` or `--pre`, since it reads the old and new versions from the lockfiles.

### plugin and theme

```sh
rhythm sightread plugin akismet --from 5.3.3
rhythm sightread plugin woocommerce --path /var/www/shop -o review.html --open
rhythm sightread plugin akismet --to list
rhythm sightread plugin gravityforms --to-file ~/zips/gravityforms-2.9.1.zip
rhythm sightread theme astra --path /var/www/shop
rhythm sightread theme astra --from 4.8.0 --svn-check
```

One or more slugs. Without `--from` the installed copy is read under `--path` (the WordPress root is found up to 6 folders down). Before diffing, the installed copy is compared with the official copy of its version; a file you edited gives the info finding `local-drift`. When the installed version is already the candidate, Sightread says so and exits `0` (`<slug> <version> is already the latest: nothing to compare`), unless something recorded its bytes (a lock entry, an allow entry), which it then checks again.

`--to list` prints the published versions, newest first:

```text
$ rhythm sightread plugin b01-widget --to list
2.6.8
2.6.7
```

A theme is verified by TLS from downloads.wordpress.org, since WordPress.org publishes no theme checksums; `--svn-check` compares every file with themes.svn.wordpress.org instead. See [Sources and integrity](Sightread-Sources-And-Integrity).

### composer and npm

```sh
rhythm sightread composer --path api
rhythm sightread composer monolog/monolog --to 3.12.1
rhythm sightread composer --transitive --md -o composer.md
rhythm sightread npm --path web --sarif -o sightread.sarif
rhythm sightread npm axios --to latest
rhythm sightread npm left-pad --to list
```

They read the lockfile in `--path`: `composer.lock`, or for npm the first of `npm-shrinkwrap.json`, `package-lock.json` (v1 to v3), `pnpm-lock.yaml` (v6 and v9), `yarn.lock` (classic and Berry) and `bun.lock`. Direct dependencies by default; `--transitive` adds the rest. Names limit the review to those packages. The default candidate is `wanted`: the newest version the `composer.json` or `package.json` range allows.

### all

```sh
rhythm sightread all /var/www/shop -o review.html --open
rhythm sightread all --sites sites.yaml --md -o fleet.md
rhythm sightread all --only wp-plugin:* --fail-on high
```

Everything a folder holds, in one report: WordPress plugins and themes, `composer.lock` and npm lockfiles. `--sites <sites.yaml>` reviews every site of a fleet file once per update and groups the result by update (`b01-widget 2.6.7 → 2.6.8  HOLD on 2 sites`). `--rereview` reviews each installed version against the release before it, so new signals reach what you already run. `all` takes no `--from`, no folder with `--sites` and no `--path`, `--wp` or `--from-tuner` with `--sites` (the fleet file sets them per site). See [Watch](Sightread-Watch#fleets) for the file.

### changes

```sh
rhythm sightread changes --base origin/main --fail-on high --md
rhythm sightread changes --before old.json --after package-lock.json
```

Each lockfile at `--base` (default `GITHUB_BASE_REF` in Actions, else the merge base of HEAD and `origin/HEAD`) against the working tree, or `--before` against `--after` with no git. Every added or moved package is reviewed, transitive ones included. Allow entries, the config and the baseline are read at the base ref, so a pull request can't approve its own update; `--trust-pr-allow` counts the ones the change edits. This is what the [GitHub Action](Sightread-CI) runs on pull requests.

| Flag | Meaning |
|---|---|
| `--base <ref>` | compare the lockfiles at this git ref |
| `--before <lockfile>` | the old lockfile (with `--after`; no git needed) |
| `--after <lockfile>` | the new lockfile |
| `--lockfile <lockfile>` | a lockfile to compare (repeatable; default: every one found) |
| `--path <folder>` | the git repository folder whose lockfiles `--base` compares (default `.`) |
| `--trust-pr-allow` | count allow entries, config and baseline the change itself edits |

### Versions

| Flag | Default | Meaning |
|---|---|---|
| `--to <version>` | `latest` (plugins, themes), `wanted` (composer, npm) | the candidate: `latest`, `wanted`, `installed` (against an older `--from`), `x.y.z`, an npm dist-tag, or `list` to print the published versions |
| `--from <version>` | read from `--path`, `--wp`, `--from-tuner` or the lockfile | the installed version |
| `--to-file <path>` | | the candidate from a local zip, tgz or folder (premium plugins, vendor downloads); marked unverified |
| `--from-file <path>` | | the installed version from a local zip, tgz or folder |
| `--path <folder>` | `.` | the site or project folder: a WordPress root or a folder with lockfiles |
| `--wp <command>` | | read installed WordPress versions with this WP-CLI command, e.g. `"ssh deploy@host -- wp --path=/srv/www"` |
| `--from-tuner <report>` | | read installed plugin and theme versions from a `rhythm-tuner` JSON report |
| `--allow-downgrade` | off | review a `--to` older than the installed version |
| `--pre` | off | let `latest` and `wanted` pick pre-releases |
| `--transitive` | off | composer, npm and all: review transitive dependencies too (changes always does and refuses the flag) |
| `--only <packages>` | | only these packages: ecosystem IDs or `<eco>:<name>` globs, e.g. `wp-plugin:*,npm:@acme/*` |
| `--skip <packages>` | | leave these packages out (same form as `--only`) |

Ecosystem IDs are `wp-plugin`, `wp-theme`, `composer` and `npm`. A package is written `<eco>:<name>@<version>`: `wp-plugin:akismet@5.3.5`, `composer:monolog/monolog@3.12.1`, `npm:@scope/name@1.2.3`.

### Sources and integrity

| Flag | Default | Meaning |
|---|---|---|
| `--offline` | off | use cached data and archives only (`RHYTHM_OFFLINE=1` does the same) |
| `--allow-unverified` | off | review packages whose integrity couldn't be checked and let `--fail-on` pass them |
| `--allow-partial` | off | let `--fail-on` pass a run with packages not checked, fetch failures or timeouts |
| `--svn-check` | off | verify themes file by file against themes.svn.wordpress.org |
| `--no-osv` | off | don't ask api.osv.dev for known malicious and vulnerable versions |
| `--check-confusion` | off | ask the public registry about private names (this sends them there) |
| `--max-size <size>` | `256MB` | largest archive to download; unpacked it may be 4 times that, at most 1GiB |
| `--timeout <duration>` | `30s` | how long one request may take |
| `--deadline <duration>` | `30m` | how long the whole run may take |
| `--retries <n>` | `2` | retries after network errors, 429 and 503; `0` means none |
| `--jobs <n>` | `4` | packages reviewed in parallel (at most 4 requests per host) |
| `--proxy <url>` | `HTTPS_PROXY` | HTTP proxy |
| `-A, --user-agent <header>` | `rhythm-sightread/<version> (+https://github.com/RhythmShahriar/sightread)` | the User-Agent header |

### Signals

| Flag | Default | Meaning |
|---|---|---|
| `--signals <folder>` | | load extra signal YAML from this folder (repeatable); a folder that isn't there is an error |
| `--no-signal-updates` | off | use the built-in signals and names, not a newer downloaded pack |
| `--id <ids>` | | show only findings with these signal IDs (globs); the others still count for the verdict and `--fail-on` |
| `--exclude-id <ids>` | | hide findings with these signal IDs (globs), never integrity or allow signals; they still count |
| `--severity <level>` | `info` | hide findings below `info`, `low`, `medium`, `high` or `critical`; they still count |
| `--min-confidence <confidence>` | `likely` | confidence a finding needs to count: `confirmed` or `likely` |
| `--min-age <duration>` | `72h` | `fresh-release` fires for a candidate younger than this |

`--id`, `--exclude-id` and `--severity` only hide. To stop a finding counting, use an `ignore` entry in the [config](Sightread-Configuration#ignore), an [allow entry](Sightread-Allow-And-Baseline) or a baseline. A run that hides findings says so:

```text
$ rhythm sightread plugin b02-formtools --severity critical --fail-on high
rhythm sightread 1.0.0 - wp-plugin b02-formtools - 1.4.0 to 1.4.1
Read-only. Sightread changes nothing on your site and never runs the code it downloads.
note: 4 findings are hidden by --severity critical (they still count for the verdict and --fail-on)
...
Verdict: REVIEW
4 hidden
...
```

It exits `1`: the hidden high findings still trip `--fail-on high`.

### Gate

| Flag | Default | Meaning |
|---|---|---|
| `--fail-on <level>` | `none` (config `fail_on`) | exit `1` on a finding at or above `none`, `info`, `low`, `medium`, `high` or `critical` |
| `--baseline <report>` | | hide findings already in this saved JSON report |
| `--no-allow` | off | ignore `.sightread/allowed.yaml` for this run |
| `--trust-pr-allow` | off | `changes` only: count allow entries, config and baseline the change edits |
| `--config <file>` | the nearest `.sightread/config.yaml` here or above | config file |
| `--no-config` | off | read no config, for folders you don't trust |

### Output

| Flag | Default | Meaning |
|---|---|---|
| `--json` | | print the `rhythm-sightread/v1` JSON report |
| `--jsonl` | | print one JSON line per update as it finishes, then the summary |
| `--md` | | print Markdown for pull requests and job summaries |
| `--html` | | print a standalone HTML report with the full diff |
| `--sarif` | | print SARIF 2.1.0 for GitHub code scanning |
| `-o, --output <file>` | | also write the report to this file, format from `.json`, `.jsonl`, `.md`, `.html` or `.sarif` (repeatable; written once complete) |
| `--open` | | open the HTML report when done |
| `--diff` | | print every changed hunk, not just the ones findings point at |
| `--max-findings <n>` | `10` | findings shown per update on the terminal |
| `--sign` | | sign the JSON report (`<file>.json.sig`, Ed25519) |
| `--redact-paths` | on when `CI` is set | write paths relative to the target or `~` |
| `-q, --quiet` | | verdicts and findings only |
| `--plain` | when stdout isn't a terminal | no color, boxes or glyphs |
| `--no-color` | | no color (`NO_COLOR` does the same) |
| `-v, --verbose` | | every request, hash and program started to stderr, header values masked |

See [Output and reports](Sightread-Output) for each format.

### Soundcheck

| Flag | Default | Meaning |
|---|---|---|
| `--soundcheck <url>` | | install the update on this staging URL and compare it before and after with `rhythm-soundcheck` |
| `--apply <command>` | | composer and npm, or a remote `--wp`: the command that installs the update on staging |
| `--rollback` | off | reinstall the reviewed old version when Soundcheck finds changes at `--soundcheck-fail-on` |
| `--soundcheck-fail-on <class>` | `broke` | Soundcheck change class that fails: `broke`, `new` or `changed` |
| `--soundcheck-anyway` | off | install on staging even when the update is on HOLD |
| `--skip-roadie-verify` | off | don't check the staging copy with `rhythm-roadie verify` (the report says so) |
| `-- <flags>` | | arguments after `--` go to `rhythm-soundcheck snapshot` |

`all` and `changes` take no Soundcheck flags. See [With Soundcheck and Roadie](Sightread-With-Soundcheck).

## Other commands

### explain

```sh
rhythm sightread explain remote-fetch-exec
rhythm sightread explain review.json 3
rhythm sightread explain review.json 9f2c41aa07d3b6e1
```

With a signal ID: what it means, why it matters, how to review it, its usual false alarms and the incidents it is built on, with no network. With a saved JSON report and a finding's number (the `#n` the terminal prints next to each finding, counted from 1 over the report worst first) or fingerprint: that finding, its evidence and the diff hunk with 6 lines of context from the cached archives. A review of a local file, or one whose archives are no longer cached, shows the hunk the report saved. `--json` prints the `explain` document. Output is in [Your first review](Sightread-First-Review#4-read-a-finding).

### allow

| Flag | Default | Meaning |
|---|---|---|
| `--reason <text>` | required when adding | why it is accepted |
| `--by <name>` | `git config user.name`, else `$USER` | who reviewed it |
| `--until <date>` | | `YYYY-MM-DD`; the entry counts through that day |
| `--signals <IDs>` | every finding of that version | only these signal IDs |
| `--file <file>` | `.sightread/allowed.yaml` next to the config | the allow file |
| `--from <report>` | | take every update of this saved JSON report |
| `--force` | | replace an entry that allows the same version with another sha256 |
| `--json` | | `allow list`: print the `allowed` JSON document |
| `--config <file>`, `--no-config`, `--offline` | | as for reviews |

See [Allow and baseline](Sightread-Allow-And-Baseline).

### watch

| Flag | Default | Meaning |
|---|---|---|
| `-l, --list <file>` | required | the `sites.yaml` file to review (read again on every run) |
| `--every <duration>` | | run now, then every duration, e.g. `24h` (at least `1m`) |
| `--at <time>` | | run every day at this local time; several separated by commas, e.g. `02:00,14:00` |
| `--now` | | with `--at`, also run once right away |
| `--once` | | run once now and exit (for cron and CI) |
| `--name <name>` | the sites file's name | the watch's name, in notifications and its folder |
| `--dir <folder>` | the user cache, `rhythm/sightread/watch/<name>` | folder for reports, logs and state |
| `--keep <n>` | `30` | runs to keep, oldest deleted first (`0` keeps all) |
| `--notify <url>` | | `slack://`, `discord://`, `webhook+https://` or `smtp://` (repeatable) |
| `--notify-min <level>` | `high` | lowest finding level notified |
| `--notify-strict` | | exit `3` when a notification can't be sent |
| `--fail-on-new <level>` | | exit `1` when a run finds a new finding at or above this level |
| `--html` | | also write each run's report as HTML next to its JSON |
| `-v, --verbose` | | every program started and notification sent to stderr, secrets masked |
| `-- <flags>` | | review flags for every run, e.g. `-- --jobs 2` |

See [Watch](Sightread-Watch).

### report and verify-report

```sh
rhythm sightread report review.json -o review.html
rhythm sightread report web.json api.json -o all.md -o all.sarif
rhythm sightread report review.json --md >> "$GITHUB_STEP_SUMMARY"
rhythm sightread verify-report review.json
rhythm sightread verify-report review.json --key ci-report-key.pub
```

`report` renders saved JSON reports by the extension of each `-o` file (`.md`, `.html`, `.sarif` or `.json`), or prints one format to stdout with `--md`, `--html`, `--sarif` or `--json`; several merge into one, worst first. It takes no `.jsonl`: JSON lines come only from a review. `--diff` keeps every changed hunk the reports hold and `--redact-paths` shows paths relative to the project. `verify-report` checks `<report.json>.sig` against this machine's key, the config's `trusted_keys` and each `--key`: exit `0` good, `1` bad or untrusted, `2` a missing file. `--json` prints the `verify` document. See [Output and reports](Sightread-Output#signed-reports).

### init

| Flag | Default | Meaning |
|---|---|---|
| `ci` | | write `.github/workflows/sightread.yml` instead of the config |
| `--force` | | replace an existing file; without it an existing file is a usage error (exit `2`) |
| `--fail-on <level>` | `high` | `ci`: fail the job at this level, or `none` |
| `--sarif` | off | `ci`: also upload findings to code scanning |
| `--comment` | on | `ci`: keep one pull request comment; `--comment=false` turns it off |

Output is in [Configuration](Sightread-Configuration#the-config-file) and [CI](Sightread-CI#init-ci). A build from source (`version` says `dev`) can't pin the Action to a release commit: `init ci` says so, writes `@v1` and leaves the `version` input as a comment to fill in.

### doctor, demo, signals, cache, update, version

| Command | Flags | Notes |
|---|---|---|
| `doctor` | `--offline`, `--json`, `--plain` | exit `1` when a check fails; output in [FAQ](Sightread-FAQ#doctor) |
| `demo` | `--keep`, `-o <file>` (repeatable), `--open`, `--json`, `-q`, `--fail-on <level>`, `--plain`, `--no-color` | `--keep` keeps the archives, cache and example site in a folder under `TMPDIR` and prints a command that reviews two of them; `--fail-on high` exits `1` (two updates are on HOLD), as CI would. The other review flags don't apply to the built-in packages |
| `signals` | `--json` | `ID  SEVERITY  CONFIDENCE  ECOSYSTEMS  NAME` |
| `cache prune` | `--older <duration>` (default `30d`) | removes archive indexes older than that, then every blob nothing names |
| `update` | `--json`, `--pin <name=version,...>` | downloads the signal and popular-name data, checked against the signed release; never a package |
| `version` | `--json` | `rhythm-sightread 1.0.0` |

```text
$ rhythm sightread cache prune --older 7d
removed 0 archive indexes older than 1 week and 0 cached files nothing uses (0 B)

$ rhythm sightread version --json
{"schema":"rhythm-version/v1","name":"rhythm-sightread","version":"1.0.0","commit":"","kit_version":"v0.8.0"}
```

## Exit codes

| Code | Means |
|---|---|
| `0` | ok |
| `1` | findings at `--fail-on`, or new findings at `--fail-on-new` (watch); a refused HOLD under `--soundcheck`; a bad or untrusted signature (`verify-report`); a failed `doctor` check |
| `2` | usage error: a bad flag, value, config or allow entry, a version that doesn't exist, nothing installed found, a file `init` would replace without `--force` |
| `3` | couldn't fetch, verify or check a package (with `--fail-on`, unless `--allow-partial` or `--allow-unverified`); a run that reviewed nothing because every package failed to fetch, with any flags; a failed Soundcheck run or missing helper; a notification that failed under `--notify-strict` |
| `130` | interrupted |

Without `--fail-on`, a run with gaps still exits `0` and says so in the verdict box (`Incomplete review: ...`) and in the JSON (`complete: false`). The exception is a run that could review nothing at all: when every package it should review failed to fetch (a registry down, `--offline` with an empty cache), it exits `3` with `verdict: "error"` and the `E7` entries in `data.errors`, with or without `--fail-on`, `--allow-partial` or `--no-osv`, so a dead registry never reads as a pass. `--allow-partial` covers the gaps of a run that reviewed something. When both findings and gaps would fail the gate, the exit is `1`. The first Ctrl+C stops after the current package and writes what it has (`stopped_by: "interrupted"`); the second stops at once.

## Error messages

Every message is `<program> <command>: <what happened>`, then indented lines with the fix. A flag a command doesn't take is named as you typed it, with one hint and where the flags are listed, rather than the whole help page:

```text
$ rhythm sightread demo --md
rhythm sightread demo: unknown flag --md
  the demo writes other formats as files: -o demo.md
  rhythm sightread help demo lists every flag
```

When a mirror you set can't be reached, the hint names the variable that set it (`check SIGHTREAD_NPM_REGISTRY, which points at https://...`) instead of the connection. Network errors are summarised; URLs and raw errors show only with `-v`. These were printed by 1.0.0:

```text
$ rhythm sightread plugin b01-widgit
rhythm sightread plugin: no installed b01-widgit under .../shop
  name the site with --path /var/www/site, read it with --wp "wp --path=..." or give the version: --from <version>

$ rhythm sightread plugin b01-widget --to 9.9.9
rhythm sightread plugin: b01-widget has no version 9.9.9 on wordpress.org
  newest: 2.6.8 · see them all: rhythm sightread plugin b01-widget --to list

$ rhythm sightread npm unpublib --to 1.0.1
rhythm sightread npm: unpublib 1.0.1 was unpublished from registry.npmjs.org
  newest: 1.0.0 · see them all: rhythm sightread npm unpublib --to list

$ rhythm sightread npm
rhythm sightread npm: no lockfile in .../home (looked for npm-shrinkwrap.json, package-lock.json, pnpm-lock.yaml, yarn.lock, bun.lock)
  run your package manager once to write one; Sightread compares versions a lockfile pins

$ rhythm sightread npm leftpadplus --offline
rhythm sightread npm: can't reach 127.0.0.1 (offline and not cached)
  with --offline only cached data and archives are used; run once without it to fill the cache

$ rhythm sightread all --fail-on severe
rhythm sightread: --fail-on "severe": want none, info, low, medium, high or critical

$ rhythm sightread plugin myplug --to-file /nonexistent.zip
rhythm sightread: --to-file /nonexistent.zip: no such file or folder
  check the path; it names the zip, tgz or folder of the package

$ rhythm sightread plugin b01-widget --to installed
rhythm sightread plugin: --to installed reviews the installed version against an older one, so it needs --from
  give the older version: rhythm sightread plugin b01-widget --to installed --from <version>

$ rhythm sightread explain remote-fetch-eval
rhythm sightread explain: no signal "remote-fetch-eval"
  list them: rhythm sightread signals
```

The offline example ran against the testbed mirror on 127.0.0.1, so it names that host; against the real registry it names registry.npmjs.org. The rest of the catalogue:

| Case | Exit |
|---|---|
| Nothing installed found, or a `--wp` command that fails | 2 |
| A slug WordPress.org doesn't know (with a "did you mean" when one is close) | 2 |
| A version that isn't published, or was unpublished from npm | 2 |
| No lockfile, or one Sightread can't read (`bun.lockb`: run `bun install --save-text-lockfile` once) | 2 |
| Integrity couldn't be checked (checksums unavailable, a limit hit): not diffed | 3 with `--fail-on` |
| Integrity mismatch: the critical finding `integrity-mismatch`, never an error | 1 with `--fail-on` |
| Network down, or `--offline` with an empty cache | 3 |
| Rate limited and still after the retries | 3 |
| A private package that needs credentials (`COMPOSER_AUTH`, a scope token) | 3 |
| An archive over `--max-size` or a zip bomb | 3 |
| A bad config, allow entry or `--signals` folder | 2 |
| A bad flag value | 2 |
| `--soundcheck` pointed at production | 2 |
| Soundcheck or Roadie missing, or `rhythm roadie verify` failed | 3 |
| api.github.com's anonymous limit too low for the run (set `GITHUB_TOKEN`) | 3 |
| `--soundcheck` on an update that is on HOLD | 1 |
| `allow` on an unverified or mismatched version, or an `--until` in the past | 2 |
| A `--to` older than the installed version without `--allow-downgrade` | 2 |
| `--soundcheck` with a remote `--wp` for a plugin or theme and no `--apply` | 2 |
| An unknown command | 2 |

More messages, with what to do about each, are in the [FAQ](Sightread-FAQ).

## JSON documents

Every `--json` document has the same envelope: `schema` (`rhythm-sightread/v1`), `kind`, `tool`, `version`, `complete`, `summary`, `verdict` (`ok`, `findings` or `error`) and `data`. Fields are only added within v1. The schema ships in the repository as [`docs/schema/rhythm-sightread-v1.json`](https://github.com/RhythmShahriar/sightread/blob/master/docs/schema/rhythm-sightread-v1.json).

| Kind | Written by |
|---|---|
| `review` | `plugin`, `theme`, `composer`, `npm`, `all`, `changes`, `report -o x.json` |
| `allowed` | `allow list --json` |
| `watch` | each `watch` run's report |
| `signals` | `signals --json` |
| `explain` | `explain ... --json` |
| `doctor` | `doctor --json` |
| `verify` | `verify-report --json` |

`version --json` writes `rhythm-version/v1` and `update --json` `rhythm-data/v1`, shared with the other tools. The review document, field by field, is in [Output and reports](Sightread-Output#json).

## Environment variables

| Variable | Use |
|---|---|
| `SIGHTREAD_WPORG_API`, `SIGHTREAD_WPDL_API`, `SIGHTREAD_WPSVN`, `SIGHTREAD_WPTHEMESVN`, `SIGHTREAD_PACKAGIST`, `SIGHTREAD_PACKAGIST_META`, `SIGHTREAD_GITHUB_API`, `SIGHTREAD_GITHUB_CODELOAD`, `SIGHTREAD_NPM_REGISTRY`, `SIGHTREAD_OSV_API` | base URLs for mirrors and the testbed; `https://`, or `http://` on a loopback host only |
| `SIGHTREAD_NPM_EXTRA_KEYS` | a JSON file of extra npm signing keys to trust (a private registry, the testbed) |
| `GITHUB_TOKEN`, `RHYTHM_GITHUB_TOKEN` | sent only to api.github.com: Composer archives on GitHub and tag checks |
| `SIGHTREAD_NPM_TOKEN_<SCOPE>` | a private npm scope's token, sent only to that scope's registry |
| `COMPOSER_AUTH` | private Composer repositories, as Composer reads it; each entry goes only to its host |
| `SIGHTREAD_SMTP_PASSWORD` | the password for `--notify smtp://` |
| `HTTPS_PROXY` | the proxy, unless `--proxy` |
| `SSL_CERT_FILE`, `SSL_CERT_DIR` | extra CA certificates (a TLS-inspecting proxy, a private mirror) |
| `GITHUB_BASE_REF` | the default `--base` of `changes` in GitHub Actions |
| `CI` | turns on `--redact-paths` and keeps the picker away |
| `RHYTHM_OFFLINE` | the same as `--offline` |
| `RHYTHM_CACHE_DIR` | moves the cache (the Action points it at the runner's temp folder) |
| `NO_COLOR` | no color |
| `RHYTHM_UPDATE_PROMPT`, `RHYTHM_NO_UPDATE_CHECK` | shared with every tool; see [Updating](Updating) |

Sightread never reads tokens from `.npmrc` or `auth.json`. It reads `.npmrc` only for `@scope:registry=` lines, so private scopes are never sent to the public registry.

## Files

| Path | What |
|---|---|
| `.sightread/config.yaml` | project settings, found here or in a folder above; see [Configuration](Sightread-Configuration) |
| `.sightread/allowed.yaml` | reviewed versions, bound to their sha256 |
| `~/.cache/rhythm/sightread/` (Linux) | `blobs/` content-addressed and re-hashed on read, `archives/` tree indexes, `meta/` registry answers, `runs/` the last reports; mode 0700 |
| `~/.cache/rhythm/sightread/watch/<name>/` | a watch's reports, logs and state, unless `--dir` |
| `~/.config/rhythm/sightread/report-key` and `.pub` | the `--sign` key pair, created on first use, mode 0600 |

On macOS the cache is under `~/Library/Caches/rhythm` and the config under `~/Library/Application Support/rhythm`; on Windows under `%LocalAppData%` and `%AppData%`. `rhythm uninstall sightread --purge` removes the cache. The first-run notice is shown once per machine.

---
<sub>← [Security model](Sightread-Security-Model) · [Home](Home) · [FAQ and honest limits](Sightread-FAQ) →</sub>
