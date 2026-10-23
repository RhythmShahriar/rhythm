Soundcheck works with no configuration. To change its defaults for a project, and to share them with your team and CI, use `.soundcheck/config.yaml`. This page lists every key with its default, says what to commit, and describes the `.soundcheck` folder.

## Create the file

```sh
rhythm soundcheck init
```

```text
Wrote .soundcheck/config.yaml. Every setting is optional; edit what you need.
```

`init` writes every setting with its default and a comment, so you can delete what you don't change. It refuses to overwrite an existing file (`.soundcheck/config.yaml already exists (--force replaces it)`); `init --force` replaces it.

Soundcheck reads `.soundcheck/config.yaml` in the current directory, or the file given with `--config`. A missing file is fine. **Unknown keys are errors**, so a typo fails loudly instead of being ignored:

```text
rhythm soundcheck: .soundcheck/config.yaml: yaml: unmarshal errors:
  line 2: field ignored not found in type config.Visual
```

Flags override the file for one run. List flags (`--include`, `--exclude`, `--page`) add to the file's lists; the others replace the value.

## Every setting

```yaml
pages:
  limit: 300            # most pages to check
  per_group: 3          # pages checked per URL pattern (/blog/{slug}/ …); -1 checks all
  include: []           # only pages matching these globs, e.g. ["/shop/*"]
  exclude: []           # skip pages matching, e.g. ["/tag/*", "/author/*"]
  ignore_robots: false  # crawl pages robots.txt disallows (your own site)
  extra: []             # always check these, even if no link points to them, e.g. ["/checkout/"]

viewports: [desktop, mobile]
modules: [health, visual, interact, cache, speed]

visual:
  threshold: 0.5        # % of a page's pixels that must change to report it
  ignore: []            # CSS selectors hidden before screenshots, e.g. [".ad", "#live-chat"]

interactions:
  levels: [inspect, click]  # add submit to send test data through forms, flows to run flows/
  max_clicks: 15            # per page
  allow: []                 # selectors to click even if they look destructive
  forms: []                 # forms that may be submitted on production sites, by CSS selector

# Live sites. Form submits are refused on these unless the form is in
# interactions.forms. URL prefixes:
production: []          # e.g. ["https://example.com"]

login:
  url: ""               # login page; default /wp-login.php on WordPress
  flow: ""              # or a flow file that logs in, for other sites
# Credentials come only from SOUNDCHECK_USER and SOUNDCHECK_PASSWORD.

flows: .soundcheck/flows  # folder of YAML flows (rhythm soundcheck flows init)

speed:
  slower: 25            # % slower before a page is reported SLOWER

rate: 5                 # page loads per second per site
tabs: 8                 # pages checked at once; politeness comes from rate
timeout: 30             # seconds per page
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
| `modules` | all five | `--modules`, `--skip` | `health`, `visual`, `interact`, `cache`, `speed`. |
| `visual.threshold` | `0.5` | | Percent of a page's pixels that must change to report it, 0 to 100. See [Visual](Soundcheck-Visual#how-screenshots-are-compared). |
| `visual.ignore` | `[]` | | CSS selectors hidden before screenshots and left out of comparisons. |
| `interactions.levels` | `[inspect, click]` | `--level` | Any of `inspect`, `click`, `submit`, `flows`. See [Interactions](Soundcheck-Interactions#the-four-levels). |
| `interactions.max_clicks` | `15` | | Controls clicked per page. |
| `interactions.allow` | `[]` | | Selectors to click even when they look destructive. |
| `interactions.forms` | `[]` | | Forms (CSS selectors) that may be submitted, on production URLs too. |
| `speed.slower` | `25` | | Percent a metric must worsen (past its floor) to be SLOWER. See [Speed](Soundcheck-Speed#regressions). |

### Safety and login

| Key | Default | Description |
|---|---|---|
| `production` | `[]` | URL prefixes of live sites, e.g. `["https://yoursite.com", "https://www.yoursite.com"]`. No form submits (except `interactions.forms`) and no cache leak test there. See [the production guard](Soundcheck-Interactions#the-production-guard). |
| `login.url` | `""` | Login page for the test account; empty means `/wp-login.php`. |
| `login.flow` | `""` | A flow file that logs in, for sites that aren't WordPress. Its path is relative to where you run Soundcheck. |
| `flows` | `.soundcheck/flows` | Folder of YAML flows for the `flows` level. |

Credentials are never read from this file: set `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` in the environment. See [Logging in](Soundcheck-Forms-And-Flows#logging-in).

### Pace

| Key | Default | Flag | Description |
|---|---|---|---|
| `rate` | `5` | `--rate` | Page loads per second per site; more than 0, fractions allowed (`0.5` is one every two seconds). |
| `tabs` | `8` | `--tabs` | Pages checked at once, 1 to 16. Each runs in its own background browser window. The pace on the server is set by `rate`, so more tabs only speed up slow pages. |
| `timeout` | `30` | `--timeout` | Seconds per page, at least 5. The flag takes a duration such as `45s`. |

Invalid values stop the run with a usage error that names the setting, such as `--tabs must be between 1 and 16` or `visual.threshold is a percentage, 0 to 100`.

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

production: ["https://yoursite.com", "https://www.yoursite.com"]
```

With this file, `rhythm soundcheck snapshot staging.yoursite.com` and the later `compare` use the same settings without any flags, and nobody can submit forms on the live site by accident.

## What to commit

| Path | Commit it? |
|---|---|
| `.soundcheck/config.yaml` | **Yes.** Shared settings for the team and CI. |
| `.soundcheck/flows/` | **Yes.** Your flows. Keep secrets out: use `${env:NAME}`. |
| `.soundcheck/snapshots/` | No. Screenshots and runs, large and machine-specific. |
| `.soundcheck/runs/` | No. Recent runs and comparisons. |

Inside a git repository, Soundcheck adds the snapshots and runs to `.git/info/exclude` the first time it writes them, so they never show up in `git status`, and your `.gitignore` stays untouched:

```text
# Soundcheck snapshots and reports
/.soundcheck/snapshots/
/.soundcheck/runs/
/.soundcheck/.incoming/
```

## The .soundcheck folder

```text
.soundcheck/
├── config.yaml            settings (optional)
├── flows/                 YAML flows (optional)
├── snapshots/
│   ├── LATEST             the name of the latest snapshot, which compare uses by default
│   └── before-update/
│       ├── run.json       the recorded run (schema rhythm-soundcheck/v1)
│       ├── report.html    the standalone report
│       └── shots/         screenshots: <page id>-desktop.png, -mobile.png, and .2.png second captures
├── runs/
│   └── 2026-10-24T02-23-17/
│       ├── run.json
│       ├── comparison.json   for compare and --against
│       ├── report.html
│       ├── shots/
│       ├── visual/           before, after and diff images of visual changes
│       ├── against/          the --against site's run
│       ├── flows/            flow screenshots
│       └── lighthouse/       --lighthouse results (for a snapshot, in its own folder)
└── .incoming/             only while a snapshot is being taken; moved into snapshots/ when complete
```

- **Snapshots** are kept until you delete them: `rhythm soundcheck snapshots delete <name>`. Taking a snapshot with an existing name replaces it, but only once the new one is complete, so a failed or interrupted snapshot never destroys a good one.
- **Runs** (`run` and `compare`) are pruned to the **20** most recent.
- A snapshot of a 37-page site with both viewports took about 7 MB. The HTML reports embed scaled-down JPEG screenshots, so they're smaller than the folders they come from.

All paths are relative to where you run Soundcheck; run it from the same folder each time. Soundcheck's own files outside the project (the downloaded Chromium, the release check) are listed in the [CLI reference](Soundcheck-CLI-Reference#files-and-locations).

---
<sub>← [CI](Soundcheck-CI) · [Home](Home) · [CLI reference](Soundcheck-CLI-Reference) →</sub>
