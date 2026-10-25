Every Soundcheck command and flag, its exit codes, the environment variables it reads and where it keeps files. Verified against rhythm-soundcheck 1.2.0.

Soundcheck is the `rhythm-soundcheck` binary. `rhythm soundcheck …` (or `rhythm sc …`) runs it. So does `rhythm-soundcheck …` directly; this page uses the `rhythm soundcheck` form.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm soundcheck` | | Interactive picker in a terminal; prints help otherwise. See [below](#the-picker). |
| `rhythm soundcheck snapshot <url> [flags]` | `snap` | Check the site and save the run as a named snapshot. |
| `rhythm soundcheck compare [url] [flags]` | `cmp` | Check the snapshot's pages again and show what changed. The site defaults to the snapshot's. |
| `rhythm soundcheck <url> --against <url> [flags]` | | Check the `--against` site, then `<url>` on the same pages. Compare them now. |
| `rhythm soundcheck run <url> [flags]` | `check` | Check the site once, with no comparison. |
| `rhythm soundcheck report [run] [flags]` | | Show a stored result again: the latest run or comparison, a run folder, or a snapshot by name. `--serve` reviews it in the browser; `--client --brand <agency.yaml>` writes a client report. |
| `rhythm soundcheck accept <id>...` | | Make changes of the latest comparison part of its snapshot. See [Review and accept](Soundcheck-Review-And-Accept). |
| `rhythm soundcheck review [run]` | | Accept or ignore changes one by one, in the terminal. |
| `rhythm soundcheck snapshots [--json]` | `ls` | List snapshots. `snapshots delete <name>` (or `rm`) deletes one. |
| `rhythm soundcheck history` | | Every result so far, with trends. See [History](Soundcheck-History). |
| `rhythm soundcheck fleet <sites.yaml> snapshot\|compare` | | Snapshot or compare many sites. See [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports). |
| `rhythm soundcheck wp-update <url> --wp <command>` | | Update WordPress plugins and find the one that breaks the site. See [WordPress updates](Soundcheck-WP-Update). |
| `rhythm soundcheck rules list\|test [dir]` | | Your own error rules. See [Error rules and exec](Soundcheck-Error-Rules). |
| `rhythm soundcheck init [--force]` | | Write `.soundcheck/config.yaml` with every setting. |
| `rhythm soundcheck import <file>` | | Write `.soundcheck/config.yaml` from a `backstop.json` or, with `--pages-only`, a Lost Pixel config. See [Import](Soundcheck-Import). |
| `rhythm soundcheck flows init` | | Write an example flow to `.soundcheck/flows/example.yaml`. |
| `rhythm soundcheck update [--clear]` | | Install or refresh the browser it uses; `--clear` deletes the downloaded Chrome for Testing. |
| `rhythm soundcheck demo [--keep]` | | Try it on a small built-in site. See [below](#demo). |
| `rhythm soundcheck --list-checks` | `list-checks` | Every check each module makes. See [below](#list-checks). |
| `rhythm soundcheck version` | `-v`, `--version` | Print the version: `rhythm-soundcheck 1.2.0`. |
| `rhythm soundcheck help [command]` | `-h`, `--help` | Print the help text, or more on `snapshot`, `compare`, `report`, `import`, `accept`, `review`, `history`, `fleet`, `wp-update` or `rules`. |

Flags may come before or after the site: `rhythm soundcheck run yoursite.com --json` and `rhythm soundcheck run --json yoursite.com` are the same. List flags (`--include`, `--modules`, `--level`…) take comma-separated values and can be repeated.

A site is a URL or a hostname. Without a scheme, `https://` is tried first, then `http://`. Soundcheck follows redirects and keeps the scheme and host the site answers on, with the path you gave.

### snapshot

```sh
rhythm soundcheck snapshot staging.yoursite.com --name before-update
```

Discovers and checks the site, then saves the run in `.soundcheck/snapshots/<name>/` and makes it the latest snapshot. It ends with the path of its `report.html` (`full report: .soundcheck/snapshots/<name>/report.html`) and the `next:` hint. Without `--name`, the name is the date and time, such as `2026-10-24T00-49-25`. Names use letters, digits, dots, dashes and underscores, up to 64 characters. An existing snapshot with the same name is replaced once the new one is complete. Each screenshot is taken twice to learn [dynamic regions](Soundcheck-Visual#regions-that-change-on-their-own). With `--lighthouse`, Unlighthouse runs after the snapshot is saved and writes into its folder.

A snapshot stopped with Ctrl+C keeps the pages it finished for 24 hours and says how to carry on:

```text
rhythm soundcheck: snapshot canceled; 1 finished page kept: to carry on with the same modules, levels, viewports and browser, run rhythm soundcheck snapshot http://127.0.0.1:18931 --name before --resume
```

With `--resume` it visits only the pages it didn't finish:

```text
Snapshot: "before"
Resume: keeping 1 page the stopped snapshot finished
Pages: 4 pages to start
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
[2/4] /about.php 200 2.93s (ok)
[3/4] /contact.php 200 2.91s (ok)
[4/4] /shop.php 200 3.08s (ok)
```

`--resume` needs `--name`: `--resume carries on with a snapshot by its name: add --name, as in the snapshot that stopped`. With nothing to carry on with, it says `Resume: no stopped snapshot "<name>" to carry on with; starting afresh`.

### compare

```sh
rhythm soundcheck compare                              # the latest snapshot, its site
rhythm soundcheck compare --snapshot before-update     # a named snapshot
rhythm soundcheck compare staging2.yoursite.com        # the snapshot's pages on another site
```

Doesn't discover pages: it visits the snapshot's pages, in the same order, with the snapshot's modules, levels, viewports and browser unless a flag or the config file sets them. It says what you changed; `--strict` stops (exit 2) instead. BROKE and NEW pages are visited once more before they're reported ([confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable)). Without a URL the header says where the site came from: `Site: from .soundcheck/snapshots/<name>/run.json (target)`. `compare --against <url>` is the same as `<url> --against <url>`.

### --against

```sh
rhythm soundcheck staging.yoursite.com --against yoursite.com
```

The `--against` site is discovered and checked first and plays the snapshot's part (saved in the run's `against/` folder); then the first site is checked on the same pages. Nothing is stored as a snapshot.

### report

```sh
rhythm soundcheck report                                    # the latest run or comparison
rhythm soundcheck report before-update                      # a snapshot
rhythm soundcheck report .soundcheck/runs/2026-10-24T02-23-17
rhythm soundcheck report --md -o soundcheck.md
```

Re-renders a stored result without visiting the site, in any format, with `--open` and `--fail-on`. A folder argument wins over a snapshot of the same name. `report --serve` reviews a comparison in the browser ([Review and accept](Soundcheck-Review-And-Accept#review-in-the-browser)); `report --client --brand agency.yaml -o client.html` writes a [client report](Soundcheck-Fleet-And-Client-Reports#client-reports). A stored interrupted result exits 130.

### snapshots

```text
$ rhythm soundcheck snapshots
NAME                     PAGES  TAKEN        SITE
before-update            37     6m ago       https://staging.yoursite.com/  (latest)
```

With none, it prints `No snapshots yet. Take one: rhythm soundcheck snapshot <url>`. `--json` prints them for scripts:

```json
[
  {
    "name": "before-update",
    "created": "2026-10-25T15:52:16Z",
    "site": "http://127.0.0.1:18931/",
    "pages": 4,
    "version": "1.2.0",
    "latest": true
  }
]
```

### init and flows init

`init` writes `.soundcheck/config.yaml` (see [Configuration](Soundcheck-Configuration)); `--force` replaces an existing one. `flows init` writes `example.yaml` into the flows folder and never overwrites it. See [Flows](Soundcheck-Forms-And-Flows#flows).

### update

```text
$ rhythm soundcheck update
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
```

With an installed Chrome, Soundcheck uses it and removes any Chrome for Testing it downloaded earlier (`Removed unused Chrome for Testing <dir>`). Otherwise it downloads the Chrome for Testing this version is pinned to (156.0.8078.47 in 1.2.0), checked against the SHA-256 in the binary. It prints `Browser: Chrome for Testing at <path>`. Ctrl+C stops the download. `rhythm update` runs it after updating the binary.

| Flag | Default | Description |
|---|---|---|
| `--clear` | off | Delete the downloaded Chrome for Testing (`Cleared <dir>`). The next run downloads it again if no Chrome is installed. |

### demo

```sh
rhythm soundcheck demo
```

Serves a small bakery site from the binary on `127.0.0.1`, takes a snapshot, breaks it the way an update can and compares, so you can see a result without a site of your own:

```text
Soundcheck demo: a small bakery site at http://127.0.0.1:41033, served by this binary.
1. Taking a snapshot of it while it works (a few seconds)...
2. Breaking it the way an update can: a PHP warning in the footer of every page, a JS error on /, a dead button on /about/, no title on /blog/, /pricing/ answers 404 and the contact form answers 500.
3. Comparing with the snapshot:
```

It ends with `Something broke - 2 broke, 5 new, 2 changed, 0 slower, 0 fixed` and how to try it on your own site. It works in a temporary folder and removes it, unless you pass `--keep`.

### list-checks

`--list-checks` (or `list-checks`) prints every module in run order with whether it's on, then a line per check with its level:

```text
health: on by default
  health/unreachable     error    Page can't be reached
  health/blocked         error    Blocked by a firewall or bot check
  health/auth-required   error    Password required (401)
  health/status          error    Page returns an error status
⋮
a11y: off unless asked for (+a11y)
⋮
echo: on with --echo
  echo/message/missing  error    Message no longer sent
  echo/no-message       warning  Form sends nothing

exec: on when the config lists exec: commands, which run with --allow-exec
```

### The picker

Run `rhythm soundcheck` with no arguments in an interactive terminal to get a short form:

1. **What should Soundcheck do?** Compare with the latest snapshot (offered first when there is one, with its name, site and age), take a snapshot, or check once without comparing.
2. **Which site?** Skipped for a compare. The form reminds you: "Only check sites you own or are authorised to test."
3. **Report:** terminal, then open the HTML report (the default); terminal only; or a Markdown file, `soundcheck.md`.

It then runs the matching command. Press Ctrl+C to cancel (exit code 130). When stdin or stdout isn't a terminal, `rhythm soundcheck` prints the help and exits 0.

## Flags

These apply to `snapshot`, `compare`, `run` and `--against`. `report` takes the output flags and `--fail-on`. `history`, `accept`, `review`, `fleet`, `wp-update` and `rules` take `--plain` and `--no-color`; their own flags are on their pages.

### Snapshots

| Flag | Default | Description |
|---|---|---|
| `--name <name>` | date and time | Snapshot name. |
| `--snapshot <name>` | the latest | Compare with this snapshot. Alias `--against-snapshot`. |
| `--against <url>` | none | Compare with another site, checked now, instead of a snapshot. |
| `--strict` | off | Stop (exit 2) when `compare` wouldn't run with the snapshot's modules, levels, viewports and browser. |
| `--confirm[=off]` | on | Visit BROKE and NEW pages again and show the ones that don't come back as UNSTABLE. |
| `--resume` | off | `snapshot`: carry on with a stopped snapshot of the same `--name`, keeping the pages it finished. |

### Output

| Flag | Default | Description |
|---|---|---|
| `--json` | off | JSON in the `rhythm-soundcheck/v1` schema: the comparison, or the run. |
| `--md` | off | Markdown for pull requests and CI summaries. |
| `--junit` | off | JUnit XML for CI test reports. |
| `--html` | off | The standalone HTML report. |
| `-o`, `--output <file>` | none | Write the report to a file and keep the live view. The format comes from the format flag, else the extension: `.json`, `.md`/`.markdown`, `.xml`, `.html`/`.htm`. Repeat it for several files and formats. |
| `--open` | off | Open the HTML report when done (with `--html -o`, that file). |
| `--serve` | off | `report`: review the comparison in the browser, accepting or ignoring each change. |
| `--client` | off | `report`: a client-ready HTML report, to `-o` or stdout; needs `--brand`. |
| `--brand <file>` | none | `report --client`: the agency's name, logo, color and contact (`agency.yaml`). |
| `-q`, `--quiet` | off | Results only: no header, progress, steps or summary. |
| `--plain` | off | No color, boxes or glyphs, a line per page. Used automatically when stdout isn't a terminal. |
| `--no-color` | off | No color. |
| `-v`, `--verbose` | off | Also show the browser used, the slowest pages and every log line; a line per page in place of the progress bar. |

Pick one format flag (`pick one output format, not --json and --md`). Without `-o`, a format flag replaces the live view on stdout. Every run also writes `report.html` into its own folder. See [Reading results](Soundcheck-Reading-Results#other-formats).

### What to check

| Flag | Default | Description |
|---|---|---|
| `--config <file>` | `.soundcheck/config.yaml` | Settings file. A missing default file is fine; a missing `--config` file is an error. Snapshots and runs still go in `./.soundcheck`. |
| `--limit <n>` | `300` | At most this many pages. |
| `--include <globs>` | all | Only pages whose path and query match, e.g. `'/shop/*'`. The start page is always checked. |
| `--exclude <globs>` | none | Skip pages matching, on top of the built-in excludes. |
| `--page <path>` | none | Check this page too, even if nothing links to it; it gets past `--include`, `robots.txt` and grouping. Repeatable. |
| `--ignore-robots` | off | Visit pages `robots.txt` disallows (your own site). |
| `--modules <list>` | `health,visual,interact,cache,speed` | Also `a11y`, `seo` (off unless asked for), `echo` (on with `--echo`) and `exec` (on when the config lists `exec:` commands). `+<list>` adds to the default or config list, e.g. `+a11y,seo`. |
| `--skip <list>` | none | Leave these modules out. |
| `--level <list>` | `inspect,click` | Interaction levels: `inspect`, `click`, `submit`, `flows`. Each includes the ones before it, so `submit` also clicks; `+submit` works too. `submit` sends test data. |
| `--viewport <list>` | `desktop,mobile` | Viewports to screenshot. |
| `--quick` | off | A fast pass: `--viewport desktop` and `--modules health,interact`, unless you set those flags. |
| `--no-login` | off | Skip the login (leak test, login flows), even if `SOUNDCHECK_USER` is set. |
| `--allow-env <name>` | none | Let flows read this variable too (repeatable); otherwise `${env:...}` reads only `SOUNDCHECK_*` ones. |
| `--allow-host <host>` | none | Let flows open this host too, e.g. a payment page (repeatable). |
| `--insecure-login` | off | Type the test password over plain http to a site that isn't this machine. |
| `--allow-exec` | off | Let the exec module run the commands the config lists under `exec:` (also `SOUNDCHECK_ALLOW_EXEC=1`). See [Error rules and exec](Soundcheck-Error-Rules#exec-your-own-program-on-every-page). |
| `--lighthouse` | off | Also run Unlighthouse (needs Node.js and npm); never on a production site. Also `SOUNDCHECK_LIGHTHOUSE=1`. See [Speed](Soundcheck-Speed#--lighthouse). |
| `--echo[=<url>]` | off | Record the emails, SMS and webhooks each form submit and flow sends, through Echo (default `http://127.0.0.1:8025`). Also `SOUNDCHECK_ECHO=<url>`. See [With Soundcheck](Echo-With-Soundcheck). |

See [Discovery](Soundcheck-Discovery) and [Interactions](Soundcheck-Interactions).

### Requests and browser

| Flag | Default | Description |
|---|---|---|
| `--rate <n>` | `5` | Page loads per second per site, 0.01 to 1000. |
| `--tabs <n>` | `8` | Pages checked at once, 1 to 16, each in its own background window. |
| `--timeout <d>` | `30s` | Per page, as a duration (`45s`, `1m`); at least 5 s. |
| `-A`, `--user-agent <ua>` | `rhythm-soundcheck/<version> (+https://rythm.sh/soundcheck)` | User-Agent header, for the browser and plain requests. |
| `-H`, `--header <h>` | none | Extra header, `"Name: value"`. Repeatable. Sent to the checked site only (and its www. or bare host), by the browser too. A `Host` header for another host is a usage error. |
| `--cookie <c>` | none | Cookie for the checked site and its www. or bare host, e.g. `"staging_auth=abc"` (browsers send it to every port of the host). Don't pass a login cookie: use the test account, so the leak test stays meaningful. |
| `--proxy <url>` | none | `http://`, `https://`, `socks5://` or `socks5h://` proxy, for the browser and plain requests. |
| `-k`, `--insecure` | off | Don't verify TLS certificates (local and staging sites). |
| `--browser <name\|path>` | `chrome` | `chrome`, `firefox` or `webkit`, or the path to a Chrome binary; `firefox=<path>` names another engine's binary. See [Firefox and WebKit](Soundcheck-Browsers). |
| `--no-download` | off | Never download Chrome for Testing; fail if no Chrome is installed. |
| `--no-sandbox` | off | Run Chrome without its sandbox, as root in a container (also `SOUNDCHECK_NO_SANDBOX=1`); only for sites you trust. |
| `--follow-host` | off | Check the site a cross-site redirect leads to, instead of stopping. |
| `--show-browser` | off | Show the browser window, for debugging flows. |

### CI

| Flag | Default | Description |
|---|---|---|
| `--fail-on <level>` | none | Exit 1 when a comparison has changes at or above `broke`, `new`, `changed` or `slower`; for `run` and `snapshot`, issues at or above `error`, `warning` or `notice`. `none` never fails (overrides the config's `fail_on`). Any case. |

### Import

| Flag | Default | Description |
|---|---|---|
| `--site <url>` | the first scenario's | `import`: the site to check, written as `site:`. |
| `--force` | off | `import`: replace `.soundcheck/config.yaml` when there is one. |
| `--pages-only` | off | `import`: read only the page paths of a Lost Pixel config, which is code and never runs. |

See [CI](Soundcheck-CI).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done, with nothing at or above `--fail-on` (or `--fail-on` not set). |
| `1` | A change or issue at or above `--fail-on`. |
| `2` | Usage error: unknown command or flag, a bad value or config file, a rule that doesn't load, no site, no snapshot to compare with, nothing stored for `report`, an existing file for `init`, `import` or `flows init`. |
| `3` | The site couldn't be checked (unreachable, no pages found, no browser), or a file couldn't be written. |
| `130` | Interrupted (Ctrl+C, SIGHUP or SIGQUIT). An interrupted snapshot keeps the pages it finished for `--resume`; an interrupted compare or run keeps partial results, marked interrupted. |

Usage errors name the flag and what it accepts:

```text
rhythm soundcheck: --level (interactions.levels): "submitt" isn't one of inspect, click, submit, flows
rhythm soundcheck: pick one output format, not --json and --md
rhythm soundcheck: --fail-on "broke": want error, warning or notice (or a class like broke when comparing)
rhythm soundcheck: --confirm: "maybe" isn't on or off
rhythm soundcheck: --browser: "firefx" isn't chrome, firefox or webkit (did you mean firefox?)
```

## Environment variables

| Variable | Description |
|---|---|
| `SOUNDCHECK_USER`, `SOUNDCHECK_PASSWORD` | The test account for the cache leak test and flows with `login: true`. Both must be set. Pages are still checked as a visitor. Never written to reports. |
| `SOUNDCHECK_NO_SANDBOX` | `1` runs Chrome without its sandbox, like `--no-sandbox`. |
| `SOUNDCHECK_ALLOW_EXEC` | `1` lets exec commands run, like `--allow-exec`. |
| `SOUNDCHECK_LIGHTHOUSE` | `1` turns on `--lighthouse`. |
| `SOUNDCHECK_ECHO` | Echo's URL, like `--echo=<url>`. |
| `RHYTHM_ECHO_TOKEN` | Echo's token, if it has one (or the variable `echo.token_env` names). |
| `SOUNDCHECK_*` | Config `headers` and flows read their values from these; other names only with `--allow-env`. |
| `<PREFIX>_SOUNDCHECK_*`, `<PREFIX>_RHYTHM_ECHO_*` | In a [fleet](Soundcheck-Fleet-And-Client-Reports#variables-and-logins-per-site), one site's variables. |
| `SOUNDCHECK_COMMENT_AUTHOR` | The GitHub Action: the bot login whose comment to update, for a GitHub App token. |
| `RHYTHM_NO_UPDATE_CHECK` | Set to any value to stop the once-a-day "new release available" check. |
| `CI` | When set (as CI systems do), the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check, for mirrors or GitHub Enterprise. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks to avoid GitHub's anonymous rate limit. |
| Any name passed with `--allow-env` | Read by `${env:NAME}` in flows. |

The installer variables (`RHYTHM_INSTALL_DIR`, `RHYTHM_VERSION`, `RHYTHM_TOOLS`) are in [Install](Install#installer-options).

## Files and locations

In the project, relative to where you run Soundcheck:

| What | Where |
|---|---|
| Settings | `.soundcheck/config.yaml` |
| Flows | `.soundcheck/flows/*.yaml` |
| Snapshots | `.soundcheck/snapshots/<name>/` and `.soundcheck/snapshots/LATEST` |
| Runs and comparisons (the 20 newest) | `.soundcheck/runs/<date-time>/` |
| Your error rules | `.soundcheck/rules/*.yaml` |
| History | `.soundcheck/history.jsonl` |
| wp-update logs | `.soundcheck/wp-update/<date-time>.txt` and `.md` |
| A stopped snapshot, for `--resume` | `.soundcheck/.incoming/<name>/` (24 hours) |
| Fleet results | `.soundcheck-fleet/` next to `sites.yaml` |

See [The .soundcheck folder](Soundcheck-Configuration#the-soundcheck-folder).

Per user, in the operating system's standard folders (Go's `os.UserConfigDir` and `os.UserCacheDir`):

| What | Linux | macOS | Windows |
|---|---|---|---|
| Downloaded Chrome for Testing | `~/.cache/rhythm/soundcheck/browser` (or `$XDG_CACHE_HOME/rhythm/soundcheck/browser`) | `~/Library/Caches/rhythm/soundcheck/browser` | `%LocalAppData%\rhythm\soundcheck\browser` |
| Release check state | `~/.config/rhythm/update-check/rhythm-soundcheck.json` | `~/Library/Application Support/rhythm/update-check/…` | `%AppData%\rhythm\update-check\…` |
| First-run notice marker | `~/.config/rhythm/seen/soundcheck-authorised-use` | `~/Library/Application Support/rhythm/seen/…` | `%AppData%\rhythm\seen\…` |

The responsible-use notice is shown once per machine; deleting its marker brings it back.

---
<sub>← [Firefox and WebKit](Soundcheck-Browsers) · [Home](Home) · [FAQ and troubleshooting](Soundcheck-FAQ) →</sub>
