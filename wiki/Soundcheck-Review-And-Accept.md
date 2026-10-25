A comparison shows every difference, including the ones you meant to make. This page covers how to deal with those: accept a change into the snapshot, ignore a known change for a while, step through the changes in the terminal or the browser. It also explains how the confirm pass keeps a flaky page from failing a build.

| You want to… | Use |
|---|---|
| Make an expected change part of the baseline | `rhythm soundcheck accept <id>` |
| Accept a whole kind of change on some pages | `rhythm soundcheck accept --page '/team/*' --check 'visual/*'` |
| Keep a known change out of every comparison until a date | an `ignore:` rule in `.soundcheck/config.yaml` |
| Decide change by change | `rhythm soundcheck review` (terminal) or `rhythm soundcheck report --serve` (browser) |
| Tell a real break from a flaky one | nothing: the confirm pass runs by default |

## Change ids

Every change in a comparison has a short id. The terminal prints how to accept a CHANGED line under it:

```text
  CHANGED  2 ─────────────────────────────────────────────────────────────────

    ■  New block near the bottom of 3 pages (mobile)                  3 pages
       rhythm soundcheck accept 49d86260 if this is expected
    ■  New block near the bottom of 3 pages (desktop)                 3 pages
       rhythm soundcheck accept 994dc87c if this is expected
```

In `--plain` mode the hint is the last line of the change. In `--json` each change and each ignored change has an `id` field. The id comes from the change's check, title and source, so the same change keeps its id from one compare to the next.

The hint is shown for CHANGED lines of a comparison with a snapshot. BROKE, NEW, SLOWER and FIXED lines don't get it. Neither does a comparison of two sites (`--against`). You can still accept any of them by id.

## Accept

```sh
rhythm soundcheck accept 49d86260 994dc87c
```

```text
Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/)
Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/contact.php)
Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/shop.php)
Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/)
Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/contact.php)
Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/shop.php)
Snapshot "before-update" now holds 6 accepted changes. Run rhythm soundcheck compare to check again.
```

`accept` works on the latest comparison and writes into the snapshot it compared with, one line per change and page (`site-wide` for a site issue). The snapshot's `snapshot.json` records who accepted each change: git's `user.email`, else `$USER`. The next compare no longer reports them:

```text
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
```

Accepting is the lighter choice than a new snapshot: the rest of the baseline stays as it was, so a change you haven't looked at yet still shows up.

To accept by pattern instead of by id, give a page glob and a check glob:

```sh
rhythm soundcheck accept --page '/team/*' --check 'visual/*'
```

That accepts every visual change on the team pages. Either glob alone matches everything for the other. Ids and globs don't mix.

| Error (exit 2) | Means |
|---|---|
| the usage text | no ids and no globs |
| `give change ids or --page and --check, not both` | both were given |
| `nothing to show yet; run rhythm soundcheck compare or run first` | nothing is stored in `.soundcheck` |
| `accept works on a comparison and <dir> has none; run rhythm soundcheck compare first` | the latest result is a run or a snapshot |
| `accept works on a comparison with a snapshot; <dir> compared two sites (--against)` | there is no snapshot to accept into |

## Ignore

Some changes are known and shouldn't count for a while: a photo carousel on the team page, a JS error that goes away with next sprint's release. List them under `ignore:` in [`.soundcheck/config.yaml`](Soundcheck-Configuration):

```yaml
ignore:
  - check: "health/js-error"
    pages: ["/"]
    until: "2026-12-31"
    reason: "store locator is replaced next sprint"
```

| Field | Matches |
|---|---|
| `check` | the change's check, a glob with `*`: `visual/*`, `health/js-error` |
| `title` | the change's title, a glob with `*` |
| `pages` | page globs; the change must be on one of them |
| `until` | the last day the rule applies, as `2026-12-31` |
| `reason` | shown with the ignored change |

Every field given must match. The rules apply after every comparison (after the confirm pass). A matching change moves to `ignored`: it is counted, never fails `--fail-on` and doesn't show in the list. The summary says how many:

```text
Something broke - 1 broke, 2 new, 0 changed, 0 slower, 1 fixed - compared with snapshot "before-update" (1m ago), 4 pages · 1 change ignored · 5s
```

In `--json` the change is under `ignored` with its `reason`; `summary.ignored` counts them. An expired rule, or one that matched nothing, adds a warning to the comparison (`ignore rule 1 (check health/js-error, pages /) expired on 2020-01-01; its changes count again`).

Mistakes in a rule stop the command before the run, exit 2:

```text
rhythm soundcheck: .soundcheck/config.yaml line 1: ignore: rule 1: until "soon" isn't a date like 2026-12-31
rhythm soundcheck: .soundcheck/config.yaml line 2: unknown setting "ignore.page" (did you mean "ignore.pages"?)
```

A rule with no `check`, `title` or `pages` reads `ignore: rule 1 has nothing to match: give it a check, a title or pages`.

## Review in the terminal

```sh
rhythm soundcheck review
```

`review` steps through the latest comparison (or the run folder given) one change at a time, most serious first. FIXED changes are left out.

```text
  Change 1 of 3                                                       23ecc1aa
   BROKE    Page not found (404)
            /about.php
            → Restore the page, or redirect it and update links to it.
            health/status
  a accept   i ignore always   s skip   q quit
```

| Key | Does |
|---|---|
| `a` | accept the change into the snapshot, as `accept <id>` does |
| `i` | ignore it from now on: writes an `ignore:` rule with the change's check, title and pages and `reason: ignored in review by <who> on <date>` into `.soundcheck/config.yaml`. The file's comments are kept. A rule that wouldn't load isn't written. |
| `s` | skip it |
| `q` | quit (also Esc and Ctrl+C) |

At the end it prints `Reviewed: 2 accepted, 1 ignored, 4 skipped (of 7)`. With nothing to review: `Nothing to review: no changes since the snapshot.`

`review` needs a terminal. In a script it stops with exit 2:

```text
rhythm soundcheck: review needs an interactive terminal; in scripts use the HTML report or accept changes by id
```

## Review in the browser

```sh
rhythm soundcheck report --serve
```

```text
Review the changes in your browser:
  http://127.0.0.1:45211/?token=<48 hex characters>
Press Ctrl-C here when you're done.
```

It serves the latest comparison's HTML report on `127.0.0.1` with **Accept**, **Ignore always** and **Skip** buttons under each change. Accept and Ignore always do what `a` and `i` do in the terminal. The page only answers on that address with the token printed, so other sites open in your browser can't use it. Press Ctrl+C when you're done. `report <run folder> --serve` reviews an older comparison. A result that isn't a comparison stops with `nothing to review: this result isn't a comparison`.

When the report on disk doesn't match the changes on record, the buttons are off and the page says `Review buttons are off: this report doesn't match the changes on record.`

## The confirm pass and UNSTABLE

A server that fails one load in five, or a page that times out while the machine is busy, would otherwise fail a build for nothing. So `compare` and `--against` visit the pages of BROKE and NEW changes again before reporting. A page issue that doesn't come back becomes **UNSTABLE**: shown, never failing `--fail-on`. One that does come back gets one more visit, so a flaky page is rarely taken for a broken one.

```text
Confirm: BROKE and NEW pages are visited again before they're reported; --confirm=off skips it
...
Confirming: visiting 4 pages again, to tell a real break from a flaky one

[NEW] Broken link to /shop.php (500) - /, /about.php, /contact.php
  fix: Update or remove the link, or redirect the old URL.
[UNSTABLE] Server error 500 - /shop.php
  fix: Look at the PHP and web server error logs for this URL.

New problems - 0 broke, 1 new, 0 changed, 0 slower, 0 fixed, 1 unstable - compared with snapshot "before" (just now), 4 pages · 3s
```

Here `/shop.php` answered 500 once, then 200 on the second visit. Site-level changes, such as a broken link or a missing favicon, can't be confirmed by visiting a page again and keep their class.

`--confirm=off` skips the pass (also `--confirm=false`, `0` or `no`). The header then reads `Confirm: off (--confirm=off): BROKE and NEW pages aren't visited again`. If the second visit can't run, the comparison gets a warning and its changes stand: `confirm: couldn't visit the pages again (<why>); BROKE and NEW changes aren't confirmed`.

> [!NOTE]
> In 1.2.0 the second visit doesn't see accessibility issues, so a NEW a11y issue comes back as UNSTABLE. When you compare with `+a11y`, add `--confirm=off`. See [Accessibility and SEO](Soundcheck-Accessibility-And-SEO#accessibility).

---
<sub>← [Reading results](Soundcheck-Reading-Results) · [Home](Home) · [Discovery](Soundcheck-Discovery) →</sub>
