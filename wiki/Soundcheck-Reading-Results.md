This page explains every part of a Soundcheck result: the five change classes, how a problem on many pages becomes one line, the issue levels of a single run, the HTML report, the other report formats and the exit codes.

## Two kinds of result

| Command | Result | Ranked by |
|---|---|---|
| `compare`, `<url> --against <url>` | a **comparison**: only what's different between two runs | change class: BROKE, NEW, CHANGED, SLOWER, FIXED |
| `snapshot`, `run` | a **run**: the problems the site has right now | issue level: error, warning, notice |

Most of the time you read comparisons. A snapshot also shows its run's issues, so you know the starting point before you change anything.

## The change classes

| Class | Glyph | Means | Comes from |
|---|---|---|---|
| **BROKE** | `✖` | worked in the snapshot, fails now | a new issue at level **error**: a 404 or 500, a PHP fatal, the "critical error" screen, a white screen, a form that no longer submits, a click that throws, a private page now cached, a cache leak |
| **NEW** | `▲` | a problem that wasn't there before | a new issue at level **warning** or **notice**: a PHP warning, a JS error, a dead button, a broken link, a new `noindex` |
| **CHANGED** | `■` | looks or reads differently; check it's intended | screenshots that differ, pages that got taller or shorter, a changed title, a page that lost most of its text or now redirects elsewhere, a page cache that stopped serving pages |
| **SLOWER** | `◆` | measurably slower than the snapshot | lab metrics that got worse past a noise floor, a plugin or theme that got heavier |
| **FIXED** | `✔` | a problem from the snapshot is gone | an issue in the snapshot that's no longer found, a page that got measurably faster or lighter |

Issues are matched between the two runs by their check, title and source, on the same page. A problem that's in both runs isn't shown at all: a comparison is about your change, not about everything imperfect. Run `rhythm soundcheck run` for the full list.

Speed is the exception. Lab timings wobble between loads, so a page near a threshold would flip between NEW and FIXED. Speed issues are never diffed directly; the speed module reports SLOWER and FIXED itself, only when a value moved by more than `speed.slower` percent (default 25) **and** a fixed floor. See [Speed](Soundcheck-Speed#regressions).

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
- **Visual changes** shared by several pages become one change, and more than five pages that each look different in one viewport collapse into one line such as `35 pages look different (mobile)`. See [Visual changes](Soundcheck-Visual#grouping).
- **Speed** changes group by URL pattern, or become one site-wide change when most pages moved.

Within a class, changes are sorted by how big they are (how many pages, how much changed or got slower), then by title.

## Reading the terminal

```text
  ♪ rhythm soundcheck 1.0.0                      https://staging.yoursite.com/   ← 1 header
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update", 37 pages, taken 6m ago                          ← 2 steps
  ▸ Pages         37 pages

  BROKE  2 ───────────────────────────────────────────────────────────────────    ← 3 section

 ▌  BROKE      Private page is served from cache                       /cart/     ← 4 card
 ▌             nginx                                                              ← 5 source
 ▌             x-cache-status: HIT; cache-control: max-age=86400; age: 128        ← 6 detail
 ▌             → Exclude cart, checkout, account and login pages from every …     ← 7 fix

  CHANGED  12 ────────────────────────────────────────────────────────────────

    ■  Page got 168 px taller (mobile)                                      /     ← 8 line
    ■  Page title changed                                             /about/

  SLOWER  3 ──────────────────────────────────────────────────────────────────

    ◆  Plugin heavy-slider now loads assets on pages          0 B → 293 KB  /     ← 9 before → after

 ╭───────────────────────────────────────────────────────────────╮
 │  SOUNDCHECK   Something broke                                 │                ← 10 verdict
 │  ✖ 2 broke   ▲ 6 new   ■ 12 changed   ◆ 3 slower   ✔ 0 fixed  │
 ╰───────────────────────────────────────────────────────────────╯
  compared with snapshot "before-update" (9m ago) · 37 pages · 2m42s              ← 11 stats
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…     ← 12 notes
  → full report: .soundcheck/runs/2026-10-24T00-45-13/report.html                 ← 13 report
```

1. **Header.** The version and the site, as it was resolved (scheme and host after redirects).
2. **Steps.** Which snapshot is compared, how many pages, and the login (`Login: as sa*** (from SOUNDCHECK_USER)`) when one is used.
3. **Sections**, one per class, with a count.
4. **Cards.** BROKE and NEW changes get a card: the class, the title and where it happened, a page or a count of pages, followed by up to three pages.
5. **Source.** Where it comes from: a theme or plugin file and line, a script URL, a cache layer, a flow file.
6. **Detail.** The evidence, such as the headers that show a page came from the cache.
7. **Fix.** The first line of the suggested fix. The full text is in the HTML, Markdown and JSON reports.
8. **Lines.** CHANGED, SLOWER and FIXED changes are one line each.
9. **Before → after**, for SLOWER changes.
10. **Verdict.** `Something broke` (any BROKE), `New problems` (any NEW), `Nothing broke; some pages changed`, or `Nothing changed`.
11. **Stats.** What was compared, the number of pages and how long it took.
12. **Notes**, lines starting with `!`: anything that limits the result, such as a skipped test, pages that failed to load, or two runs made with different settings.
13. **The HTML report** of this result.

Lines are cut to fit 80 columns, marked with `…`. The plain view, the HTML report and the other formats have everything.

### Notes about different settings

A comparison is only fair when both runs were made the same way. When they weren't, a note says so:

```text
note: modules differ (snapshot [interact], now [health visual interact cache])
note: interaction levels differ (snapshot [inspect click submit], now [inspect click])
```

`compare` doesn't reuse the snapshot's flags: pass the same `--level`, `--modules`, `--skip` and `--viewport` again, or better, keep them in [`.soundcheck/config.yaml`](Soundcheck-Configuration) so every run uses them. A check that only ran this time reports everything it finds as NEW.

## The issue levels of a run

`snapshot` and `run` list the site's current issues by level:

| Level | Glyph | Means | Examples |
|---|---|---|---|
| **error** | `✖` | something doesn't work | a 5xx or 404 page, a PHP fatal, a white screen, a form that fails, a click that throws, a cached cart, a cache leak, a failed flow |
| **warning** | `▲` | a problem visitors may hit | a PHP warning, a JS error, a dead click, a broken link or image, mixed content, a form without a nonce |
| **notice** | `·` | worth knowing | a PHP notice or deprecation, a missing title, `noindex`, a redirect chain, speed suggestions |

The verdict is `the site already has errors`, `a few warnings` or `all clear`, prefixed with `Snapshot "<name>" saved;` for a snapshot. In a later comparison, a new error becomes BROKE and a new warning or notice becomes NEW.

## Plain mode

When the output isn't a terminal (a pipe, a file, a CI log), or with `--plain`, Soundcheck prints one line per page while it works and a plain list at the end, with no color, boxes or glyphs:

```text
rhythm soundcheck 1.0.0 - https://staging.yoursite.com/

Snapshot: "before-update", 37 pages, taken 6m ago
Pages: 37 pages
⋮
[BROKE] Private page is served from cache - /cart/
  source: nginx
  x-cache-status: HIT; cache-control: max-age=86400; age: 128
  fix: Exclude cart, checkout, account and login pages from every cache layer (nginx), and bypass the cache for requests with woocommerce_items_in_cart, wp_woocommerce_session_* or wordpress_logged_in_* cookies. WooCommerce marks these pages no-cache; a cache rule is overriding that. Purge the cache afterwards.
[NEW] PHP Warning: Undefined array key "layout" - /, /about/, /blog/ +33 more
  source: themes/acme/footer.php:42
  fix: Fix the code at the source line, and turn display_errors off in production so visitors never see these.
⋮
[SLOWER] Total Blocking Time up from 0 - /
  0 ms → 399 ms
  Lab measurement, one load per page; a page counts when it's higher by more than 25% and at least 100 ms
  fix: New or updated JavaScript runs longer tasks; check which plugin or theme scripts changed (see the per-plugin cost suggestions).

Something broke - 2 broke, 6 new, 12 changed, 3 slower, 0 fixed - compared with snapshot "before-update" (9m ago), 37 pages · 2m42s
note: cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PASSWORD to a test account to run it
full report: .soundcheck/runs/2026-10-24T00-45-13/report.html
```

Each page line, such as `[11/12] / 200 1.48s (2 issues)`, shows the progress, the path, the status, how long the visit took, and `ok`, the number of issues, or `error`. For a run (not a comparison), plain mode lists each issue as `[LEVEL] title - pages` with its source and, for errors and warnings, the fix. `-v` prints the page lines in the normal view too, and `-q` prints only the results.

## The HTML report

Every `snapshot`, `run` and `compare` writes `report.html` into its folder under `.soundcheck/`. It's one standalone file: styles, scripts and the screenshots (scaled to 960 px wide, as JPEG) are inside, so it opens in any browser without a server and can be attached to a ticket.

- **Verdict and chips.** The verdict, the stats, and a chip per class (or level) with its count. Click a chip to hide or show that class.
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
| `--md` | Markdown: the verdict, a count table, a table per class (change, where, before → after, fix), notes in a collapsed section | pull request comments, CI job summaries, client reports |
| `--json` | the comparison, or the run, in the `rhythm-soundcheck/v1` schema | scripts and dashboards |
| `--junit` | JUnit XML: a test suite per class (or level), a test case per change, plus one passing `N pages checked` case | CI test reports |
| `--html` | the standalone HTML report | sharing |

Without `-o`, a format flag prints the report on stdout instead of the live view. With `-o <file>`, the format comes from the flag or the extension (`.json`, `.md`, `.xml`, `.html`), the live view still shows, and Soundcheck prints `Report written to <file>`. Pick one format per run; `report` re-renders a stored result in any other format without checking the site again:

```sh
rhythm soundcheck report --md -o soundcheck.md
rhythm soundcheck report --junit -o soundcheck-junit.xml --fail-on new
```

In JUnit, test cases at or above `--fail-on` are failures. Without `--fail-on`, BROKE and NEW changes fail (errors, for a run).

### The JSON schema

A comparison has `schema` (`rhythm-soundcheck/v1`), `tool`, `version`, `before` and `after` (each with `target`, `name`, `dir`, `started`), `pages`, `created`, optional `warnings`, and `changes`. Each change has:

| Field | Meaning |
|---|---|
| `class` | `broke`, `new`, `changed`, `slower` or `fixed` |
| `check` | module and rule, such as `health/php-error` or `visual/size` |
| `title` | the one-line description |
| `detail`, `source`, `fix` | evidence, origin, suggested fix (when there are any) |
| `pages` | the paths affected |
| `before`, `after` | the values compared, such as `1 KB` and `2.3 MB` |
| `score` | the ranking within the class: pages affected, % of pixels changed, % slower |
| `files` | `before`, `after` and `diff` screenshots, relative to the run folder |

```json
{
  "class": "changed",
  "check": "visual/size",
  "title": "Page got 168 px taller (mobile)",
  "detail": "A 168 px tall block was inserted 840 px down; the content below it moved down.",
  "fix": "Expected if content was added or removed; otherwise look for a new notice, banner or element pushing the layout.",
  "pages": ["/"],
  "before": "390×955 px",
  "after": "390×1123 px",
  "score": 17.59,
  "files": {
    "after": "visual/de4beeca4d-mobile-after.png",
    "before": "visual/de4beeca4d-mobile-before.png",
    "diff": "visual/de4beeca4d-mobile-diff.png"
  }
}
```

A run (from `run`, `snapshot`, or `report` of either) has `target`, `name`, `started`, `duration_ms`, `options` (viewports, levels, modules, limit, logged_in), `pages` (each with status, redirects, title, metrics, screenshots, `interactions` and `issues`), site-level `issues`, `skipped` (discovery counts by reason), `warnings` and the cache summary. Each issue has `check`, `level`, `title`, `detail`, `source`, `fix`, `key` and `pages`. The full layout is in [`model/model.go`](https://github.com/RhythmShahriar/soundcheck/blob/master/model/model.go).

## Exit codes and --fail-on

Without `--fail-on`, Soundcheck exits `0` whatever it finds. With it, it exits `1` when the result meets the threshold:

| Result | `--fail-on` values | Fails on |
|---|---|---|
| comparison | `broke`, `new`, `changed`, `slower` | that class or any above it: `new` fails on BROKE or NEW. FIXED never fails. |
| run, snapshot | `error`, `warning`, `notice` | that level or any above it: `warning` fails on errors or warnings |

```sh
rhythm soundcheck compare --fail-on broke
```

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or `--fail-on` not set) |
| `1` | A change or issue at or above `--fail-on` |
| `2` | Usage error: unknown command or flag, a bad value, no snapshot to compare with |
| `3` | The site couldn't be checked (unreachable, no pages found, no browser), or a report couldn't be written |
| `130` | Interrupted (Ctrl+C) |

A `--fail-on` value of the wrong kind is a usage error: `run` and `snapshot` don't accept `broke`, and comparisons don't accept `error`. See [CI](Soundcheck-CI).

---
<sub>← [Your first check](Soundcheck-First-Check) · [Home](Home) · [Discovery](Soundcheck-Discovery) →</sub>
