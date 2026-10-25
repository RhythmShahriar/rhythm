This page explains every part of a Soundcheck result: the change classes, how a problem on many pages becomes one line, the issue levels of a single run, the HTML report, the other report formats and the exit codes.

## Two kinds of result

| Command | Result | Ranked by |
|---|---|---|
| `compare`, `<url> --against <url>` | a **comparison**: only what's different between two runs | change class: BROKE, NEW, CHANGED, SLOWER, FIXED, UNSTABLE |
| `snapshot`, `run` | a **run**: the problems the site has right now | issue level: error, warning, notice |

Most of the time you read comparisons. A snapshot also shows its run's issues, so you know the starting point before you change anything.

## The change classes

| Class | Glyph | Means | Comes from |
|---|---|---|---|
| **BROKE** | `✗` | worked in the snapshot, fails now | a new issue at level **error**: a 404 or 500, a PHP fatal, the "critical error" screen, a white screen, a form that no longer submits, a click that throws, a private page now cached, a cache leak |
| **NEW** | `▲` | a problem that wasn't there before | a new issue at level **warning** or **notice**: a PHP warning, a JS error, a dead button, a broken link, a new `noindex` |
| **CHANGED** | `■` | looks or reads differently; check it's intended | screenshots that differ, pages that got taller or shorter, a changed title, a page that lost most of its text or now redirects elsewhere, a page cache that stopped serving pages |
| **SLOWER** | `◆` | measurably slower than the snapshot | lab metrics that got worse past a noise floor, a plugin or theme that got heavier |
| **FIXED** | `✓` | a problem from the snapshot is gone | an issue in the snapshot that's no longer found, a page that got measurably faster or lighter |
| **UNSTABLE** | | seen once, gone on a second visit | a BROKE or NEW page issue the [confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable) didn't see again, such as a 500 a flaky server gave once |

UNSTABLE changes are shown but never fail `--fail-on`. Changes you [accepted or ignored](Soundcheck-Review-And-Accept) don't show at all; the summary counts the ignored ones.

Issues are matched between the two runs by their check, title and source, on the same page. A problem that's in both runs isn't shown at all: a comparison is about your change, not about everything imperfect. Run `rhythm soundcheck run` for the full list.

Speed is the exception. Lab timings wobble between loads, so a page near a threshold would flip between NEW and FIXED. Speed issues never show as NEW or FIXED; only the speed module reports speed changes, as SLOWER and FIXED, only when a value moved by more than `speed.slower` percent (default 25) **and** a fixed floor. See [Speed](Soundcheck-Speed#regressions).

> [!NOTE]
> A page that's only in one of the two runs doesn't make its problems new or fixed. If pages are missing this time, the result says so in a note.

## One line per problem, not per page

The same problem on many pages is one change, with every page listed:

```text
 ▌  NEW        PHP Warning: Undefined array key "layout"             36 pages
 ▌             /, /about/, /blog/ +33 more
 ▌             themes/acme/footer.php:42
```

How grouping works:

- **Issues** group by their identity: the check, the title and the source. PHP messages and JS errors are normalized (long messages cut, numbers in JS errors replaced) so the same error groups across pages and runs. An error in a script inside the page itself is reported with the source `inline script`, since its line number moves with the content.
- **Site-wide issues**, such as a broken link or a speed suggestion, are reported once with every page they affect.
- **Identical controls** on a page with the same outcome are one change. Six wishlist buttons that all throw are `Clicking Add to wishlist throws a JS error`, with the selectors in the detail (`6 controls: …`).
- **Visual changes** shared by several pages become one change. More than five pages that each look different in one viewport collapse into one line such as `35 pages look different (desktop)`. See [Visual changes](Soundcheck-Visual#grouping).
- **Speed** changes group by URL pattern, or become one site-wide change when most pages moved.

Within a class, changes are sorted by how big they are (how many pages, how much changed or got slower), then by title.

## Reading the terminal

```text
  ♪ rhythm soundcheck 1.2.0                            http://127.0.0.1:18931/   ← 1 header
  ────────────────────────────────────────────────────────────────────────────

  ▸ Site          from .soundcheck/snapshots/before-update/run.json (target)     ← 2 steps
  ▸ Modules       health, visual, interact, cache, speed
  ▸ Levels        inspect, click
  ▸ Viewports     desktop, mobile
  ▸ Production    none listed: submit and flows run on any site
  ▸ Snapshot      "before-update", 4 pages, taken just now
  ▸ Settings      from snapshot "before-update" (modules health,visual,interact,cache,speed; levels inspect,click; viewports desktop,mobile)
  ▸ Confirm       BROKE and NEW pages are visited again before they're reported; --confirm=off skips it
  ▸ Pages         4 pages to start
  · Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
  ▸ Confirming    visiting 4 pages again, to tell a real break from a flaky one

  BROKE  1 ───────────────────────────────────────────────────────────────────    ← 3 section

 ▌  BROKE      Page not found (404)                                /about.php     ← 4 card
 ▌             → Restore the page, or redirect it and update links to it.         ← 7 fix

  NEW  3 ─────────────────────────────────────────────────────────────────────

 ▌  NEW        PHP Warning: Undefined array key "layout"              3 pages
 ▌             /, /contact.php, /shop.php
 ▌             themes/acme/footer.php:42                                          ← 5 source
 ▌             → Fix the code at the source line and turn display_errors off…

  CHANGED  2 ─────────────────────────────────────────────────────────────────

    ■  New block near the bottom of 3 pages (mobile)                  3 pages     ← 8 line
       rhythm soundcheck accept 49d86260 if this is expected                      ← 9 accept hint

  FIXED  1 ───────────────────────────────────────────────────────────────────

    ✓  LCP 90% faster                                               /shop.php

 ╭──────────────────────────────────────────────────────────────╮
 │  SOUNDCHECK   Something broke                                │                 ← 10 verdict
 │  ✗ 1 broke   ▲ 3 new   ■ 2 changed   ◆ 0 slower   ✓ 1 fixed  │
 ╰──────────────────────────────────────────────────────────────╯
  compared with snapshot "before-update" (just now) · 4 pages · 5s                ← 11 stats
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…     ← 12 notes

  → full report: .soundcheck/runs/2026-10-25T15-52-40.244Z/report.html            ← 13 report
```

(Shortened: one NEW card and one CHANGED line shown of each.)

1. **Header.** The command, the version and the site, as it was resolved (scheme and host after redirects).
2. **Steps.** Where the site came from, the modules, levels and viewports, the production guard, which snapshot is compared and with which settings, the confirm pass, how many pages, the browser and the test account (`Login: logging in as so••••• (from SOUNDCHECK_USER) for the leak test and login flows`) when one is set for the leak test and login flows.
3. **Sections**, one per class, with a count.
4. **Cards.** BROKE and NEW changes get a card: the class, the title and where it happened, a page or a count of pages, followed by up to three pages.
5. **Source.** Where it comes from: a theme or plugin file and line, a script URL, a cache layer, a flow file.
6. **Detail.** The evidence, such as the headers that show a page came from the cache (not in this example).
7. **Fix.** The first line of the suggested fix. The full text is in the HTML, Markdown and JSON reports.
8. **Lines.** CHANGED, SLOWER and FIXED changes are one line each.
9. **Accept hint.** Under a CHANGED line of a comparison with a snapshot: the command that makes it part of the snapshot. See [Review and accept](Soundcheck-Review-And-Accept). SLOWER lines show **before → after** in this place.
10. **Verdict.** `Something broke` (any BROKE), `New problems` (any NEW), `Nothing broke; some pages changed`, or `Nothing changed`. An interrupted run shows INTERRUPTED instead.
11. **Stats.** What was compared, the number of pages, how many changes were ignored and how long it took.
12. **Notes**, lines starting with `!`: anything that limits the result, such as a skipped test, pages that failed to load, or two runs made with different settings.
13. **The HTML report** of this result. With `-o report.html` it names that file instead. A snapshot prints the same line, then the `next:` hint.

Below a comparison that found something, a tip shows the `--fail-on` value that would make it fail in a script: `tip: add --fail-on broke to make this exit 1 in scripts`.

Lines are cut to fit 80 columns, marked with `…`. The plain view, the HTML report and the other formats have everything.

### Settings the snapshot used

`compare` runs with the snapshot's modules, levels, viewports and browser, unless a flag or the config file sets them. The `Settings` step says where they came from. When you set something else, a step says what you changed:

```text
You changed: modules health,visual,cache -> health (--modules); browser firefox -> chrome (--browser)
```

Only modules that ran in both runs are compared, so adding a module doesn't flood the result with NEW. With `--strict`, `compare` stops (exit 2) instead of running with other settings:

```text
rhythm soundcheck: --strict: this comparison wouldn't use snapshot "ff"'s settings: browser firefox -> chrome (--browser)
```

## The issue levels of a run

`snapshot` and `run` list the site's current issues by level:

| Level | Glyph | Means | Examples |
|---|---|---|---|
| **error** | `✗` | something doesn't work | a 5xx or 404 page, a PHP fatal, a white screen, a form that fails, a click that throws, a cached cart, a cache leak, a failed flow |
| **warning** | `▲` | a problem visitors may hit | a PHP warning, a JS error, a dead click, a broken link or image, mixed content, a form without a nonce |
| **notice** | `·` | worth knowing | a PHP notice or deprecation, a missing title, `noindex`, a redirect chain, speed suggestions |

The verdict is `the site already has errors`, `a few warnings` or `all clear`, prefixed with `Snapshot "<name>" saved;` for a snapshot. In a later comparison, a new error becomes BROKE and a new warning or notice becomes NEW.

## Plain mode

When the output isn't a terminal (a pipe, a file, a CI log), or with `--plain`, Soundcheck prints one line per page while it works and a plain list at the end, with no color, boxes or glyphs:

```text
rhythm soundcheck 1.2.0 - http://127.0.0.1:18931/

Site: from .soundcheck/snapshots/before-update/run.json (target)
Modules: health, visual, interact, cache, speed
Levels: inspect, click
Viewports: desktop, mobile
Production: none listed: submit and flows run on any site
Snapshot: "before-update", 4 pages, taken just now
Settings: from snapshot "before-update" (modules health,visual,interact,cache,speed; levels inspect,click; viewports desktop,mobile)
Confirm: BROKE and NEW pages are visited again before they're reported; --confirm=off skips it
Pages: 4 pages to start
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
[1/4] /shop.php 200 1.4s (1 issue)
[2/4] /contact.php 200 1.43s (1 issue)
[3/4] /about.php 404 1.37s (error)
[4/4] / 200 1.39s (2 issues)
Checking site-wide issues (links, sitemap)...
Confirming: visiting 4 pages again, to tell a real break from a flaky one
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
Confirming: visiting 4 pages once more, where the change came back
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)

[BROKE] Page not found (404) - /about.php
  fix: Restore the page, or redirect it and update links to it.
[NEW] Broken link to /about.php (404) - /, /contact.php, /shop.php
  fix: Update or remove the link, or redirect the old URL.
[NEW] PHP Warning: Undefined array key "layout" - /, /contact.php, /shop.php
  source: themes/acme/footer.php:42
  fix: Fix the code at the source line and turn display_errors off in production so visitors never see these.
[NEW] JS error: Uncaught ReferenceError: undefinedStoreLocator is not defined - /
  source: inline script
  fix: Open the browser console on this page to see the full stack.
[FIXED] LCP 93% faster - /shop.php
  396 ms → 28 ms
  Lab measurement, one load per page; a page counts when it's faster by more than 25% and at least 300 ms

Something broke - 1 broke, 3 new, 0 changed, 0 slower, 1 fixed - compared with snapshot "before-update" (1m ago), 4 pages · 5s
note: cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PASSWORD to a test account to run it
full report: .soundcheck/runs/2026-10-25T15-53-15.778Z/report.html
tip: add --fail-on broke to make this exit 1 in scripts
```

Each page line, such as `[4/4] / 200 1.39s (2 issues)`, shows the progress, the path, the status, how long the visit took and `ok`, the number of issues or `error`. For a run (not a comparison), plain mode lists each issue as `[LEVEL] title - pages` with its source and the detail for issues on one page. Errors and warnings also get the fix. CHANGED lines end with the `accept` hint. `-v` prints the page lines in the normal view too; `-q` prints only the results.

## The HTML report

Every `snapshot`, `run` and `compare` writes `report.html` into its folder under `.soundcheck/`. It's one standalone file: styles, scripts and the screenshots (scaled to 960 px wide, as JPEG) are inside, so it opens in any browser without a server and can be attached to a ticket.

- **Verdict and chips.** The verdict, the date of the run (and of the snapshot it was compared with), the stats and a chip per class (or level) with its count. Click a chip to hide or show that class.
- **Filter.** Type to filter by title, page, plugin or file.
- **Cards.** Each change with its badge, where it happened, before → after, source, detail, the full fix, the check ID (such as `health/php-error`) and the list of pages.
- **Screenshots.** Visual changes have a viewer: **Slider** (drag the handle across the before and after screenshots), **Before**, **After** and **Diff** (changed pixels highlighted on a faded copy of the page). Mobile captures show at phone width. Past 60 visual changes, the images are linked instead of embedded.
- **Notes.** The same notes as the terminal.
- **All pages.** A table of every page checked: status, issues, LCP, CLS, weight and load time.
- **Theme.** Light and dark, following your system, with a toggle.

Open the latest one with `rhythm soundcheck report --open`, or pass `--open` to `snapshot`, `run` or `compare`. To write it somewhere else, use `--html -o soundcheck.html`.

## Other formats

| Flag | Format | For |
|---|---|---|
| `--md` | Markdown: the verdict with the snapshot's date (`compared with snapshot "before-update" (23 Oct 2026 20:22 UTC)`), a count table, a table per class (change, where, before → after, fix), notes in a collapsed section | pull request comments, CI job summaries, client reports |
| `--json` | the comparison, or the run, in the `rhythm-soundcheck/v1` schema | scripts and dashboards |
| `--junit` | JUnit XML: a test suite per class (or level), a test case per change, plus one passing `N pages checked` case | CI test reports |
| `--html` | the standalone HTML report | sharing |

Without `-o`, a format flag prints the report on stdout instead of the live view. With `-o <file>`, the format comes from the flag or the extension (`.json`, `.md`, `.xml`, `.html`). The live view still shows and Soundcheck prints `Report written to <file>`. Repeat `-o` for several files in one run (`-o report.md -o report.json`). `report` re-renders a stored result in any other format without checking the site again:

```sh
rhythm soundcheck report --md -o soundcheck.md
rhythm soundcheck report --junit -o soundcheck-junit.xml --fail-on new
```

In JUnit, test cases at or above `--fail-on` are failures. Without `--fail-on`, BROKE and NEW changes fail (errors, for a run).

### The JSON schema

A comparison has `schema` (`rhythm-soundcheck/v1`), `kind`, `tool`, `version`, `before` and `after` (each with `target`, `name`, `dir`, `started`), `pages`, `created`, `warnings` (every note, including the run's own, such as a skipped leak test or an unreadable flow file; left out when there are none), `changes`, `ignored` (the changes an `ignore:` rule matched, each with its `reason`) and `summary`. `summary` has a count per class, `ignored`, the `verdict` (`broke`, `changed`, `clean` or `interrupted`) and, when `--fail-on` or `fail_on` applies, `failed`. Each change has:

| Field | Meaning |
|---|---|
| `id` | the change's id, which `accept` takes |
| `class` | `broke`, `new`, `changed`, `slower`, `fixed` or `unstable` |
| `check` | module and rule, such as `health/php-error` or `visual/block` |
| `title` | the one-line description |
| `detail`, `source`, `fix` | evidence, origin, suggested fix (when there are any) |
| `pages` | the paths affected |
| `before`, `after` | the values compared, such as `1 KB` and `2.3 MB` |
| `score` | the ranking within the class: pages affected, % of pixels changed, % slower |
| `files` | `before`, `after` and `diff` screenshots, relative to the run folder |

```json
{
  "id": "a145b4ce",
  "class": "changed",
  "check": "visual/page",
  "title": "Page looks different (mobile)",
  "detail": "A 21 px tall block was inserted 230 px down; the content below it moved down.",
  "fix": "Open the report to compare the screenshots. If the change is expected, accept it into the snapshot with accept <id> or review.",
  "pages": [
    "/contact.php"
  ],
  "score": 2.49,
  "files": {
    "after": "visual/6f3e13a78e-mobile-after.png",
    "before": "visual/6f3e13a78e-mobile-before.png",
    "diff": "visual/6f3e13a78e-mobile-diff.png"
  },
  "viewport": "mobile"
}
```

A run (from `run`, `snapshot`, or `report` of either) has `target`, `name`, `started`, `duration_ms`, `options` (viewports, levels, modules, limit, logged_in), `pages` (each with status, redirects, title, metrics, screenshots, `interactions` and `issues`), site-level `issues`, `skipped` (discovery counts by reason), `warnings` and the cache summary. Each issue has `check`, `level`, `title`, `detail`, `source`, `fix`, `key` and `pages`. The full layout is in [`model/model.go`](https://github.com/RhythmShahriar/soundcheck/blob/master/model/model.go).

## Exit codes and --fail-on

Without `--fail-on`, Soundcheck exits `0` whatever it finds. With it, it exits `1` when the result meets the threshold:

| Result | `--fail-on` values | Fails on |
|---|---|---|
| comparison | `broke`, `new`, `changed`, `slower` | that class or any above it: `new` fails on BROKE or NEW. FIXED, UNSTABLE and ignored changes never fail. |
| run, snapshot | `error`, `warning`, `notice` | that level or any above it: `warning` fails on errors or warnings |

```sh
rhythm soundcheck compare --fail-on broke
```

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or `--fail-on` not set) |
| `1` | A change or issue at or above `--fail-on` |
| `2` | Usage error: unknown command or flag, a bad value, no snapshot to compare with, nothing stored for `report` |
| `3` | The site couldn't be checked (unreachable, no pages found, no browser), or a report couldn't be written |
| `130` | Interrupted (Ctrl+C). `report` of an interrupted result exits 130 too, with `this result is partial (interrupted); run it again`. |

A `--fail-on` value of the wrong kind is a usage error: `run` and `snapshot` don't accept `broke`; comparisons don't accept `error`. See [CI](Soundcheck-CI).

---
<sub>← [Your first check](Soundcheck-First-Check) · [Home](Home) · [Review and accept](Soundcheck-Review-And-Accept) →</sub>
