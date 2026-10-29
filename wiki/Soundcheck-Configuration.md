Soundcheck works with no configuration. To change its defaults for a project and share them with your team and CI, use `.soundcheck/config.yaml`. This page lists every key with its default, says what to commit and describes the `.soundcheck` folder.

## Create the file

```sh
rhythm soundcheck init
```

```text
Wrote .soundcheck/config.yaml. Every setting is optional; edit what you need.
```

`init` writes every setting with its default and a comment, so you can delete what you don't change. It refuses to overwrite an existing file (`.soundcheck/config.yaml already exists (--force replaces it)`); `init --force` replaces it. `init --eaa` writes the file for [EAA mode](Soundcheck-EAA): see [EAA mode](#eaa-mode) below.

Soundcheck reads `.soundcheck/config.yaml` in the current directory, or the file given with `--config`. A missing file is fine. **Unknown keys are errors**, so a typo fails loudly instead of being ignored:

```text
rhythm soundcheck: .soundcheck/config.yaml line 2: unknown setting "visual.ignored" (did you mean "visual.ignore"?)
```

Flags override the file for one run. List flags (`--include`, `--exclude`, `--page`) add to the file's lists; the others replace the value.

## Every setting

This is the file `init` writes:

```yaml
# Soundcheck settings. Everything here is optional: delete what you don't
# change. Reference: https://github.com/RhythmShahriar/rhythm/wiki/Soundcheck-Configuration
# Commit this file (and flows/) to share them with your team; snapshots and
# runs are kept out of git automatically.

site: ""                # checked by snapshot and run when no URL is given, e.g. staging.example.com

pages:
  limit: 300            # most pages to check
  per_group: 3          # pages checked per URL pattern (/blog/{slug}/ …); -1 checks all
  include: []           # only pages matching these globs, e.g. ["/shop/*"]
  exclude: []           # skip pages matching, e.g. ["/tag/*", "/author/*"]
  ignore_robots: false  # crawl pages robots.txt disallows (your own site)
  extra: []             # always check these, even if no link points to them, e.g. ["/checkout/"]

viewports: [desktop, mobile]
modules: [health, visual, interact, cache, speed]  # also a11y, keyboard, seo; echo is added when Echo is on

visual:
  threshold: 0.5        # % of a page's pixels that must change to report it
  ignore: []            # CSS selectors hidden before screenshots, e.g. [".ad", "#live-chat"]

interactions:
  levels: [inspect, click]  # submit (which also clicks) sends test data through forms; flows runs flows/ too
  max_clicks: 15            # per page
  allow: []                 # selectors to click even if they look destructive
  forms: []                 # forms that may be submitted on production sites, by CSS selector

# Live sites. Form submits are refused on these unless the form is in
# interactions.forms. A host covers http and https, www and any port;
# *.example.com covers subdomains, example.com/shop only that path.
production: []          # e.g. ["example.com", "*.example.com"]

login:
  url: ""               # login page on the site; default /wp-login.php on WordPress
  flow: ""              # or a flow file that logs in, for other sites
# Credentials come only from SOUNDCHECK_USER and SOUNDCHECK_PASSWORD.

flows: .soundcheck/flows  # folder of YAML flows (the flows init command writes one)

speed:
  slower: 25            # % slower before a page is reported SLOWER

# Echo (rhythm echo up) records the emails, SMS and webhooks each form submit
# and flow sends, so comparisons catch lost or changed messages. Off while url
# is empty; --echo or SOUNDCHECK_ECHO turn it on too.
echo:
  url: ""               # e.g. http://127.0.0.1:8025
  token_env: RHYTHM_ECHO_TOKEN  # variable with Echo's token, if it has one (RHYTHM_ECHO_*)
  quiet: 1500ms         # after a submit, wait until nothing new arrives for this long
  timeout: 10s          # but no longer than this

rate: 5                 # page loads per second per site, 0.01 to 1000
tabs: 8                 # pages checked at once; politeness comes from rate
timeout: 30s            # per page: 30s, 1m or a number of seconds

# Extra request headers, each read from an environment variable so no
# secret is committed: {X-Staging-Auth: SOUNDCHECK_STAGING_AUTH}. Only
# SOUNDCHECK_* variables, unless --allow-env names another.
headers: {}

# Exit 1 at or above this, when --fail-on isn't given: broke, new, changed
# or slower for compare; error, warning or notice for run and snapshot;
# none never fails.
fail_on: ""

# Changes that are known and shouldn't count: listed as ignored, never
# failing a build. Every field given must match (check and title are globs
# with *); until is the last day the rule applies. review adds rules here.
ignore: []              # e.g. [{check: "visual/page", pages: ["/team/*"], until: "2026-12-31", reason: "weekly photos"}]

# Programs that check every page: each gets the page as JSON on stdin and
# answers {"issues": [...]}. They run only with --allow-exec.
exec: []                # e.g. [{name: laravel-log, run: ["./bin/check-log"], timeout: 10s}]
```

### pages

| Key | Default | Flag | Description |
|---|---|---|---|
| `pages.limit` | `300` | `--limit` | Most pages to check, at least 1. See [Discovery](Soundcheck-Discovery#the-page-limit). |
| `pages.per_group` | `3` | | Pages checked per URL pattern; `-1` checks every page. See [Grouping](Soundcheck-Discovery#grouping-by-url-pattern). |
| `pages.include` | `[]` | `--include` | Only pages whose path and query match one of these globs (the start page is always checked). |
| `pages.exclude` | `[]` | `--exclude` | Skip pages matching, on top of the [built-in excludes](Soundcheck-Discovery#urls-soundcheck-never-visits). Matching links aren't requested by the broken link check either. |
| `pages.ignore_robots` | `false` | `--ignore-robots` | Check pages `robots.txt` disallows. |
| `pages.extra` | `[]` | `--page` | Paths to check even if nothing links to them; they get past `include`, `robots.txt` and grouping. |

### What to check

| Key | Default | Flag | Description |
|---|---|---|---|
| `viewports` | `[desktop, mobile]` | `--viewport` | `desktop` (1366 × 900) and/or `mobile` (390 × 844). |
| `modules` | `[health, visual, interact, cache, speed]` | `--modules`, `--skip` | Also `a11y` and `seo` ([Accessibility and SEO](Soundcheck-Accessibility-And-SEO)) and `keyboard` ([Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough)); `echo` is added when Echo is on and `exec` when `exec:` lists commands. `[+a11y, seo]` adds to the default list. With `eaa: true` the list gains a11y and keyboard. |
| `visual.threshold` | `0.5` | | Percent of a page's pixels that must change to report it, 0 to 100. See [Visual](Soundcheck-Visual#how-screenshots-are-compared). |
| `visual.ignore` | `[]` | | CSS selectors hidden before screenshots and left out of comparisons. |
| `interactions.levels` | `[inspect, click]` | `--level` | Any of `inspect`, `click`, `submit`, `flows`; each includes the ones before it. See [Interactions](Soundcheck-Interactions#the-four-levels). |
| `interactions.max_clicks` | `15` | | Controls clicked per page. |
| `interactions.allow` | `[]` | | Selectors to click even when they look destructive. |
| `interactions.forms` | `[]` | | Forms (CSS selectors) that may be submitted, on production URLs too. |
| `speed.slower` | `25` | | Percent a metric must worsen (past its floor) to be SLOWER, 1 to 1000. See [Speed](Soundcheck-Speed#regressions). |
| `ignore` | `[]` | | Changes that are known and shouldn't count, until a date, matched by `check`, `title`, `pages` and `source`. `accept <id> --ignore` and `review` add rules. See [Review and accept](Soundcheck-Review-And-Accept#ignore). |
| `exec` | `[]` | `--allow-exec` | Programs that check every page; they run only with `--allow-exec`. See [Error rules and exec](Soundcheck-Error-Rules#exec-your-own-program-on-every-page). |
| `echo.url`, `echo.token_env`, `echo.quiet`, `echo.timeout` | off | `--echo` | Record what form submits and flows send through Echo. See [With Soundcheck](Echo-With-Soundcheck). |
| `fail_on` | `""` | `--fail-on` | Exit 1 at or above this, when `--fail-on` isn't given: `broke`, `new`, `changed` or `slower` for compare; `error`, `warning` or `notice` for run and snapshot; `none` never fails. |

The `speed:` block belongs to the speed module: a bad value stops every command, exit 2, even when speed is off (`.soundcheck/config.yaml line 2: speed.slower: must be a percentage between 1 and 1000`).

### Safety and login

| Key | Default | Description |
|---|---|---|
| `site` | `""` | The site `snapshot` and `run` check when no URL is given. A site that comes from this file gets no login, `-H` or `--cookie` unless it's this machine: name it on the command line for those. |
| `production` | `[]` | Live sites. A host covers http and https, www and any port; `*.example.com` covers subdomains, `example.com/shop` only that path. No form submits (except `interactions.forms`), no cache leak test and no `--lighthouse` there. See [the production guard](Soundcheck-Interactions#the-production-guard). |
| `login.url` | `""` | Login page for the test account; empty means `/wp-login.php`. |
| `login.flow` | `""` | A flow file that logs in, for sites that aren't WordPress. Its path is relative to where you run Soundcheck. |
| `flows` | `.soundcheck/flows` | Folder of YAML flows for the `flows` level. |
| `headers` | `{}` | Extra request headers, each read from an environment variable so no secret is committed: `{X-Staging-Auth: SOUNDCHECK_STAGING_AUTH}`. Only `SOUNDCHECK_*` variables, unless `--allow-env` names another. |

Your own error rules don't go in this file: they're YAML files in `.soundcheck/rules/`. See [Error rules and exec](Soundcheck-Error-Rules).

Credentials are never read from this file: set `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` in the environment. See [Logging in](Soundcheck-Forms-And-Flows#logging-in).

### Pace

| Key | Default | Flag | Description |
|---|---|---|---|
| `rate` | `5` | `--rate` | Page loads per second per site, 0.01 to 1000 (`0.5` is one every two seconds). |
| `tabs` | `8` | `--tabs` | Pages checked at once, 1 to 16. Each runs in its own background browser window. The pace on the server is set by `rate`, so more tabs only speed up slow pages. |
| `timeout` | `30s` | `--timeout` | Per page: `30s`, `1m` or a number of seconds; at least 5 s. |

Invalid values stop the run with a usage error that names the setting, such as `--tabs must be between 1 and 16` or `visual.threshold is a percentage, 0 to 100`.

## EAA mode

[EAA mode](Soundcheck-EAA) has four keys of its own. `init --eaa` writes the file with them:

```sh
rhythm soundcheck init --eaa
```

```text
Wrote .soundcheck/config.yaml. Every setting is optional; edit what you need.
Wrote .soundcheck/flows/keyboard-checkout.yaml, a key flow to edit; it runs with --level flows.
Fill in statement: before you run rhythm soundcheck statement.
```

It differs from the plain file in these lines:

```yaml
eaa: true               # same as --eaa on every snapshot, run and compare (rhythm soundcheck help eaa)

# The a11y module (axe-core).
# a11y:
#   wcag: "2.1"           # 2.0, 2.1 or 2.2 (--wcag)
#   max_elements: 20      # per rule per page, 1-200
#   iframes: false        # true audits same-origin iframes too

# The keyboard walk (rhythm soundcheck help keyboard).
# keyboard:
#   max_tabs: 150         # Tab presses per page before the walk stops, 10-2000
#   widgets: 10           # menus, dialogs, disclosures tested per page, 0-200
#   trap_after: 4         # presses on the same element or region that count as a trap, 3-20
#   skip_link: expected   # expected (missing is a notice), required (warning) or off
#   pages: ["/cart*", "/checkout*", "/my-account*", "/contact*"]  # widget tests run here and on the first page of each URL group
#   ignore: ["#cookie-banner"]  # selectors the walk steps over

# The accessibility statement draft (rhythm soundcheck statement). owner,
# website and contact.email are required. Everything here is published in
# the statement: give a role address such as accessibility@ rather than a
# person's own email or phone.
# statement:
#   owner: "[your organization]"
#   website: "https://[your public address]"
#   contact:
#     email: "accessibility@[your domain]"
#     phone: ""
#     address: ""
#   response_days: 10
#   country: ""           # ISO 3166 alpha-2, e.g. DE
#   enforcement: ""       # the authority visitors can turn to
#   scope: []             # e.g. [/, /shop/*, /checkout/*]
#   service: ""           # what the service is, for --layout service
#   known_issues: []      # e.g. [{criterion: 1.2.2, text: "Product videos have no captions yet.", until: 2027-03-31}]
#   manual: []            # what a person checked: [{criterion: 1.4.11, result: no failures found, by: Jane Doe, date: 2026-10-20}]
#   list_private_paths: false  # list paths behind login in the draft (default: count only)
...
# modules: [health, visual, interact, cache, speed]  # with eaa: true the modules are health, a11y, keyboard; a list here gains a11y and keyboard
```

| Key | Default | Flag | Description |
|---|---|---|---|
| `eaa` | `false` | `--eaa` | Accessibility evidence for the European Accessibility Act on every snapshot, run and compare. `--eaa=false` turns it off for one run. |
| `a11y.wcag` | `"2.1"` | `--wcag` | The a11y rules: `2.0`, `2.1` or `2.2`. A compare uses its snapshot's. |
| `a11y.max_elements` | `20` | | Elements kept per rule and page, 1 to 200. |
| `a11y.iframes` | `false` | | `true` also audits same-origin iframes. Frames from other origins never are. |
| `keyboard.max_tabs` | `150` | | Tab presses per page before the walk stops, 10 to 2000. |
| `keyboard.widgets` | `10` | | Menus, dialogs, disclosures and custom controls tested per page, 0 to 200. |
| `keyboard.trap_after` | `4` | | Presses on the same element before it counts as a trap, 3 to 20. |
| `keyboard.skip_link` | `expected` | | `expected`, `required` or `off`. |
| `keyboard.pages` | `[]` | | Page globs whose widgets are tested besides the first page of each URL group. |
| `keyboard.ignore` | `[]` | | CSS selectors the walk records but never judges or presses. |
| `statement.*` | | | Owner, public address, contact and your own text for the statement draft. Every key is on [Accessibility statement](Soundcheck-Accessibility-Statement#the-statement-block). |

The `a11y:` and `keyboard:` blocks are checked by their modules, so a mistake stops every run with exit 2, also when the module is off:

```text
rhythm soundcheck: .soundcheck/config.yaml line 2: unknown setting "keyboard.max_tab" (did you mean "keyboard.max_tabs"?)
```

The `statement:` block is read by `statement` and, under `--eaa`, before the run, so a mistake in it stops an `--eaa` run before any page is visited. Details on the keys: [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough#settings), [Accessibility and SEO](Soundcheck-Accessibility-And-SEO#settings) and [Accessibility statement](Soundcheck-Accessibility-Statement#the-statement-block).

The config file is read up to 1 MB, with at most 100 YAML aliases and 20 levels of nesting (`<path> is larger than 1 MB`, `config.yaml uses too many YAML aliases`, `config.yaml nests settings more than 20 levels deep`).

## A typical project file

```yaml
# .soundcheck/config.yaml for yoursite.com
pages:
  exclude: ["/tag/*", "/author/*"]
  extra: ["/checkout/"]

visual:
  ignore: ["#live-chat", ".cookie-notice", ".instagram-feed"]

interactions:
  levels: [inspect, click, submit]

production: ["yoursite.com"]

ignore:
  - check: "visual/*"
    pages: ["/team/*"]
    until: "2026-12-31"
    reason: "weekly team photos"
```

With this file, `rhythm soundcheck snapshot staging.yoursite.com` and the later `compare` use the same settings without any flags. Nobody can submit forms on the live site by accident.

## What to commit

| Path | Commit it? |
|---|---|
| `.soundcheck/config.yaml` | **Yes.** Shared settings for the team and CI. |
| `.soundcheck/flows/` | **Yes.** Your flows. Keep secrets out: use `${env:NAME}`. |
| `.soundcheck/rules/` | **Yes.** Your error rules and their samples. |
| `.soundcheck/snapshots/` | No. Screenshots and runs, large and machine-specific. |
| `.soundcheck/runs/` | No. Recent runs and comparisons. |
| `.soundcheck/evidence/` | No. Kept evidence packs; keep them with the runs you hand over. |
| `.soundcheck/history.jsonl`, `.soundcheck/wp-update/` | Your call. Your history and maintenance logs. Soundcheck doesn't exclude them, so add them to `.gitignore` if you'd rather not commit them. |

Inside a git repository, Soundcheck adds the snapshots and runs to `.git/info/exclude` the first time it writes them. They never show up in `git status` and your `.gitignore` stays untouched:

```text
# Soundcheck snapshots and reports
/.soundcheck/snapshots/
/.soundcheck/runs/
/.soundcheck/.incoming/
/.soundcheck/LATEST
/.soundcheck/.lock
```

## The .soundcheck folder

```text
.soundcheck/
├── config.yaml            settings (optional)
├── flows/                 YAML flows (optional)
├── rules/                 your error rules and their samples (optional)
├── LATEST                 the name of the latest snapshot, which compare uses by default
├── history.jsonl          a line per snapshot, run and comparison
├── wp-update/             wp-update's maintenance logs
├── snapshots/
│   └── before-update/
│       ├── run.json       the recorded run (schema rhythm-soundcheck/v1)
│       ├── pages.jsonl    its pages, one per line
│       ├── snapshot.json  the changes accepted into it, with who accepted them
│       ├── seo/           what search engines read, with the seo module
│       ├── keyboard/      keyboard traces, with the keyboard module
│       ├── eaa/           with --eaa: evidence.json, statement-draft.md, focus ring crops
│       ├── report.html    the standalone report
│       └── shots/         screenshots: <page id>-desktop.png, -mobile.png and .2.png second captures
├── runs/
│   └── 2026-10-25T15-52-40.244Z/
│       ├── run.json
│       ├── pages.jsonl
│       ├── comparison.json   for compare and --against
│       ├── report.html
│       ├── shots/
│       ├── visual/           before, after and diff images of visual changes
│       ├── against/          the --against site's run
│       ├── flows/            flow screenshots
│       └── lighthouse/       --lighthouse results (for a snapshot, in its own folder)
├── evidence/              evidence packs kept when accept, a new snapshot or a delete changes an --eaa snapshot
└── .incoming/             while a snapshot is being taken; moved into snapshots/ when complete.
                           A stopped snapshot's finished pages stay here 24 hours, for --resume
```

- **Snapshots** are kept until you delete them: `rhythm soundcheck snapshots delete <name>`. Taking a snapshot with an existing name replaces it only once the new one is complete. A failed or interrupted snapshot never destroys a good one. `accept` writes into a snapshot without retaking it.
- **Runs** (`run` and `compare`) are pruned to the **20** most recent. Runs with an evidence pack (`--eaa`) are kept, since each pack is chained to the one before it. The history keeps a line for each, so [`history`](Soundcheck-History) still shows them.
- Two commands can run in one folder at once: each run gets its own folder and the store is locked while it's written. Containers may share one `.soundcheck` folder (a mounted volume) as long as each has its own hostname, which is Docker's default.
- A snapshot of a 37-page site with both viewports took about 7 MB. The HTML reports embed scaled-down JPEG screenshots, so they're smaller than the folders they come from.

All paths are relative to where you run Soundcheck; run it from the same folder each time. Soundcheck's own files outside the project (the downloaded Chrome for Testing, the release check) are listed in the [CLI reference](Soundcheck-CLI-Reference#files-and-locations).

---
<sub>← [CI baselines and PR comments](Soundcheck-CI-Baselines) · [Home](Home) · [Error rules and exec](Soundcheck-Error-Rules) →</sub>
