Every Soundcheck command and flag, its exit codes, the environment variables it reads and where it keeps files. Verified against rhythm-soundcheck 1.0.0.

Soundcheck is the `rhythm-soundcheck` binary. `rhythm soundcheck …` (or `rhythm sc …`) runs it. So does `rhythm-soundcheck …` directly; this page uses the `rhythm soundcheck` form.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm soundcheck` | | Interactive picker in a terminal; prints help otherwise. See [below](#the-picker). |
| `rhythm soundcheck snapshot <url> [flags]` | `snap` | Check the site and save the run as a named snapshot. |
| `rhythm soundcheck compare [url] [flags]` | `cmp` | Check the snapshot's pages again and show what changed. The site defaults to the snapshot's. |
| `rhythm soundcheck <url> --against <url> [flags]` | | Check the `--against` site, then `<url>` on the same pages. Compare them now. |
| `rhythm soundcheck run <url> [flags]` | `check` | Check the site once, with no comparison. |
| `rhythm soundcheck report [run] [flags]` | | Show a stored result again: the latest run or comparison, a run folder, or a snapshot by name. |
| `rhythm soundcheck snapshots` | `ls` | List snapshots. `snapshots delete <name>` (or `rm`) deletes one. |
| `rhythm soundcheck init [--force]` | | Write `.soundcheck/config.yaml` with every setting. |
| `rhythm soundcheck flows init` | | Write an example flow to `.soundcheck/flows/example.yaml`. |
| `rhythm soundcheck update [--clear]` | | Install or refresh the browser it uses; `--clear` deletes the downloaded Chromium. |
| `rhythm soundcheck version` | `-v`, `--version` | Print the version: `rhythm-soundcheck 1.0.0`. |
| `rhythm soundcheck help` | `-h`, `--help` | Print the help text. |

Flags may come before or after the site: `rhythm soundcheck run yoursite.com --json` and `rhythm soundcheck run --json yoursite.com` are the same. List flags (`--include`, `--modules`, `--level`…) take comma-separated values and can be repeated.

A site is a URL or a hostname. Without a scheme, `https://` is tried first, then `http://`. Soundcheck follows redirects and keeps the scheme and host the site answers on, with the path you gave.

### snapshot

```sh
rhythm soundcheck snapshot staging.yoursite.com --name before-update
```

Discovers and checks the site, then saves the run in `.soundcheck/snapshots/<name>/` and makes it the latest snapshot. Without `--name`, the name is the date and time, such as `2026-10-24T00-49-25`. Names use letters, digits, dots, dashes and underscores, up to 64 characters. An existing snapshot with the same name is replaced once the new one is complete. Each screenshot is taken twice to learn [dynamic regions](Soundcheck-Visual#regions-that-change-on-their-own). With `--lighthouse`, Unlighthouse runs after the snapshot is saved and writes into its folder.

### compare

```sh
rhythm soundcheck compare                              # the latest snapshot, its site
rhythm soundcheck compare --snapshot before-update     # a named snapshot
rhythm soundcheck compare staging2.yoursite.com        # the snapshot's pages on another site
```

Doesn't discover pages: it visits the snapshot's pages, in the same order. Pass the same `--level`, `--modules`, `--skip` and `--viewport` as the snapshot, or keep them in the config file; differences are noted. `compare --against <url>` is the same as `<url> --against <url>`.

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

Re-renders a stored result without visiting the site, in any format, with `--open` and `--fail-on`. A folder argument wins over a snapshot of the same name.

### snapshots

```text
$ rhythm soundcheck snapshots
NAME                     PAGES  TAKEN        SITE
before-update            37     6m ago       https://staging.yoursite.com/  (latest)
```

With none, it prints `No snapshots yet. Take one: rhythm soundcheck snapshot <url>`.

### init and flows init

`init` writes `.soundcheck/config.yaml` (see [Configuration](Soundcheck-Configuration)); `--force` replaces an existing one. `flows init` writes `example.yaml` into the flows folder and never overwrites it. See [Flows](Soundcheck-Forms-And-Flows#flows).

### update

```text
$ rhythm soundcheck update
Removed unused Chromium /home/you/.cache/rhythm/soundcheck/browser/chromium-1321438
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
```

With an installed Chrome, Soundcheck uses it and removes any Chromium it downloaded earlier. Otherwise it downloads the Chromium this version is pinned to, if it isn't there yet. It removes other downloaded revisions and prints `Browser: Chromium at <path>`. `rhythm update` runs it after updating the binary.

| Flag | Default | Description |
|---|---|---|
| `--clear` | off | Delete the downloaded Chromium (`Cleared <dir>`). The next run downloads it again if no Chrome is installed. |

### The picker

Run `rhythm soundcheck` with no arguments in an interactive terminal to get a short form:

1. **What should Soundcheck do?** Compare with the latest snapshot (offered first when there is one, with its name, site and age), take a snapshot, or check once without comparing.
2. **Which site?** Skipped for a compare. The form reminds you: "Only check sites you own or are authorised to test."
3. **Report:** terminal, then open the HTML report (the default); terminal only; or a Markdown file, `soundcheck.md`.

It then runs the matching command. Press Ctrl+C to cancel (exit code 130). When stdin or stdout isn't a terminal, `rhythm soundcheck` prints the help and exits 0.

## Flags

These apply to `snapshot`, `compare`, `run` and `--against`. `report` takes the output flags and `--fail-on`.

### Snapshots

| Flag | Default | Description |
|---|---|---|
| `--name <name>` | date and time | Snapshot name. |
| `--snapshot <name>` | the latest | Compare with this snapshot. Alias `--against-snapshot`. |
| `--against <url>` | none | Compare with another site, checked now, instead of a snapshot. |

### Output

| Flag | Default | Description |
|---|---|---|
| `--json` | off | JSON in the `rhythm-soundcheck/v1` schema: the comparison, or the run. |
| `--md` | off | Markdown for pull requests and CI summaries. |
| `--junit` | off | JUnit XML for CI test reports. |
| `--html` | off | The standalone HTML report. |
| `-o`, `--output <file>` | none | Write the report to a file and keep the live view. The format comes from the format flag, else the extension: `.json`, `.md`/`.markdown`, `.xml`, `.html`/`.htm`. |
| `--open` | off | Open the HTML report when done (with `--html -o`, that file). |
| `-q`, `--quiet` | off | Results only: no header, progress, steps or summary. |
| `--plain` | off | No color, boxes or glyphs, a line per page. Used automatically when stdout isn't a terminal. |
| `--no-color` | off | No color. |
| `-v`, `--verbose` | off | A line per page on stderr, plus the browser's log. |

Pick one format flag. Without `-o`, a format flag replaces the live view on stdout. Every run also writes `report.html` into its own folder. See [Reading results](Soundcheck-Reading-Results#other-formats).

### What to check

| Flag | Default | Description |
|---|---|---|
| `--config <file>` | `.soundcheck/config.yaml` | Settings file. A missing default file is fine; a missing `--config` file is an error. Snapshots and runs still go in `./.soundcheck`. |
| `--limit <n>` | `300` | At most this many pages. |
| `--include <globs>` | all | Only pages whose path and query match, e.g. `'/shop/*'`. The start page is always checked. |
| `--exclude <globs>` | none | Skip pages matching, on top of the built-in excludes. |
| `--page <path>` | none | Check this page too, even if nothing links to it; it gets past `--include`, `robots.txt` and grouping. Repeatable. |
| `--ignore-robots` | off | Visit pages `robots.txt` disallows (your own site). |
| `--modules <list>` | all | `health`, `visual`, `interact`, `cache`, `speed`. |
| `--skip <list>` | none | Leave these modules out. |
| `--level <list>` | `inspect,click` | Interaction levels: `inspect`, `click`, `submit`, `flows`. `submit` sends test data. |
| `--viewport <list>` | `desktop,mobile` | Viewports to screenshot. |
| `--no-login` | off | Skip the login (leak test, login flows), even if `SOUNDCHECK_USER` is set. |
| `--lighthouse` | off | Also run Unlighthouse (needs Node.js and `npx`). See [Speed](Soundcheck-Speed#--lighthouse). |

See [Discovery](Soundcheck-Discovery) and [Interactions](Soundcheck-Interactions).

### Requests and browser

| Flag | Default | Description |
|---|---|---|
| `--rate <n>` | `5` | Page loads per second per site; fractions allowed. |
| `--tabs <n>` | `8` | Pages checked at once, 1 to 16, each in its own background window. |
| `--timeout <d>` | `30s` | Per page, as a duration (`45s`, `1m`); at least 5 s. |
| `-A`, `--user-agent <ua>` | `rhythm-soundcheck/<version> (+https://rythm.sh/soundcheck)` | User-Agent header, for the browser and plain requests. |
| `-H`, `--header <h>` | none | Extra header, `"Name: value"`. Repeatable. Sent with every request, by the browser too. |
| `--cookie <c>` | none | Cookie header, e.g. `"staging_auth=abc"`. Don't pass a login cookie: use the test account, so the leak test stays meaningful. |
| `--proxy <url>` | none | `http://`, `https://`, `socks5://` or `socks5h://` proxy, for the browser and plain requests. |
| `-k`, `--insecure` | off | Don't verify TLS certificates (local and staging sites). |
| `--browser <path>` | found or downloaded | Chrome or Chromium to use. |
| `--no-download` | off | Never download Chromium; fail if none is installed. |
| `--show-browser` | off | Show the browser window, for debugging flows. |

### CI

| Flag | Default | Description |
|---|---|---|
| `--fail-on <level>` | none | Exit 1 when a comparison has changes at or above `broke`, `new`, `changed` or `slower`; for `run` and `snapshot`, issues at or above `error`, `warning` or `notice`. |

See [CI](Soundcheck-CI).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done, with nothing at or above `--fail-on` (or `--fail-on` not set). |
| `1` | A change or issue at or above `--fail-on`. |
| `2` | Usage error: unknown command or flag, a bad value or config file, no site, no snapshot to compare with, nothing stored for `report`, an existing file for `init` or `flows init`. |
| `3` | The site couldn't be checked (unreachable, no pages found, no browser), or a file couldn't be written. |
| `130` | Interrupted (Ctrl+C). An interrupted snapshot saves nothing; an interrupted compare or run keeps partial results. |

Usage errors name the flag and what it accepts:

```text
rhythm soundcheck: --level (interactions.levels): "submitt" isn't one of inspect, click, submit, flows
rhythm soundcheck: pick one output format, not --json and --md
rhythm soundcheck: --fail-on "broke": want error, warning or notice (or a class like broke when comparing)
```

## Environment variables

| Variable | Description |
|---|---|
| `SOUNDCHECK_USER`, `SOUNDCHECK_PASSWORD` | The test account for the cache leak test and flows with `login: true`. Both must be set. Pages are still checked as a visitor. Never written to reports. |
| `RHYTHM_NO_UPDATE_CHECK` | Set to any value to stop the once-a-day "new release available" check. |
| `CI` | When set (as CI systems do), the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check, for mirrors or GitHub Enterprise. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks to avoid GitHub's anonymous rate limit. |
| Any name your flows use | Read by `${env:NAME}` in flows. |

The installer variables (`RHYTHM_INSTALL_DIR`, `RHYTHM_VERSION`, `RHYTHM_TOOLS`) are in [Install](Install#installer-options).

## Files and locations

In the project, relative to where you run Soundcheck:

| What | Where |
|---|---|
| Settings | `.soundcheck/config.yaml` |
| Flows | `.soundcheck/flows/*.yaml` |
| Snapshots | `.soundcheck/snapshots/<name>/` and `.soundcheck/snapshots/LATEST` |
| Runs and comparisons (the 20 newest) | `.soundcheck/runs/<date-time>/` |

See [The .soundcheck folder](Soundcheck-Configuration#the-soundcheck-folder).

Per user, in the operating system's standard folders (Go's `os.UserConfigDir` and `os.UserCacheDir`):

| What | Linux | macOS | Windows |
|---|---|---|---|
| Downloaded Chromium | `~/.cache/rhythm/soundcheck/browser` (or `$XDG_CACHE_HOME/rhythm/soundcheck/browser`) | `~/Library/Caches/rhythm/soundcheck/browser` | `%LocalAppData%\rhythm\soundcheck\browser` |
| Release check state | `~/.config/rhythm/update-check/rhythm-soundcheck.json` | `~/Library/Application Support/rhythm/update-check/…` | `%AppData%\rhythm\update-check\…` |
| First-run notice marker | `~/.config/rhythm/seen/soundcheck-authorised-use` | `~/Library/Application Support/rhythm/seen/…` | `%AppData%\rhythm\seen\…` |

The responsible-use notice is shown once per machine; deleting its marker brings it back.

---
<sub>← [Configuration](Soundcheck-Configuration) · [Home](Home) · [FAQ and troubleshooting](Soundcheck-FAQ) →</sub>
