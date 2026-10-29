A comparison shows every difference, including the ones you meant to make. This page covers how to deal with those: accept a change into the snapshot, ignore a known change for a while, step through the changes in the terminal or the browser. It also explains how the confirm pass keeps a flaky page from failing a build.

| You want to… | Use |
|---|---|
| Make an expected change part of the baseline | `rhythm soundcheck accept <id>` |
| Keep a known problem out of the results, in the committed config | `rhythm soundcheck accept <id> --ignore --reason <text>` |
| Accept a whole kind of change on some pages | `rhythm soundcheck accept --page '/team/*' --check 'visual/*'` |
| Keep a known change out of every comparison until a date | an `ignore:` rule in `.soundcheck/config.yaml` |
| Decide change by change | `rhythm soundcheck review` (terminal) or `rhythm soundcheck report --serve` (browser) |
| Tell a real break from a flaky one | nothing: the confirm pass runs by default |

## Change ids

Every change in a comparison has a short id. The terminal prints how to accept a BROKE, NEW or CHANGED line under it:

```text
  CHANGED  2 ─────────────────────────────────────────────────────────────────

    ■  New block near the bottom of 3 pages (mobile)                  3 pages
       rhythm soundcheck accept 49d86260 if this is expected
    ■  New block near the bottom of 3 pages (desktop)                 3 pages
       rhythm soundcheck accept 994dc87c if this is expected
```

In `--plain` mode the hint is the last line of the change. In `--json` each change and each ignored change has an `id` field. The id comes from the change's check, title and source, so the same change keeps its id from one compare to the next.

The hint is shown under BROKE, NEW and CHANGED lines of a comparison with a snapshot (BROKE and NEW since 1.3.0). FIXED, SLOWER and UNSTABLE lines don't get it. Neither does a comparison of two sites (`--against`). You can still accept any of them by id.

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
[BROKE] Keyboard focus is trapped - /a11y/consent.php
  source: #consent
  wcag: 2.1.2; EN 301 549 9.2.1.2
  path: #consent (4 times)
  Tab and Shift+Tab kept focus here for 8 presses; 2 focusable elements after it (#main > p > a:nth-of-type(1), #main > p > a:nth-of-type(2)) can't be reached.
  fix: Tab and Shift+Tab can't move focus out of an element or region, so a keyboard user is stuck there.
  rhythm soundcheck accept da3cdeac if this is expected

Accessibility: 2 of 50 WCAG 2.1 A and AA criteria have failures (2.1.2 new), 25
had none found by these tests, 23 need a person, open issues 1 to 2
Something broke - 1 broke, 0 new, 0 changed, 0 slower, 0 fixed - compared with snapshot "before" (6m ago), 21 pages, 17s
```

That is an [EAA mode](Soundcheck-EAA) comparison where `accept 6b8bcc6f` took the NEW `Control doesn't respond to Enter or Space` into the snapshot: it no longer shows, while the trap still does. The accepted problem still counts as open (`open issues 1 to 2`) and against its criterion, since accepting a problem doesn't fix it. Accepting into a snapshot with an evidence pack also records the pack again and keeps the old one ([Kept packs](Soundcheck-Accessibility-Statement#kept-packs)):

```text
Accepted 6b8bcc6f Control doesn't respond to Enter or Space (/a11y/wishlist.php)
Snapshot "before" now holds 1 accepted change. Run rhythm soundcheck compare to check again.
note: accept changed run.json, the pages file and snapshot.json of snapshot "before", so its evidence pack was recorded again. The pack from before is kept in .soundcheck/evidence/a6454f2f96e82d5d4fb2c8a6bc86f8261671203bb88530b809958b85a7e8173b with its files: evidence verify of later --eaa runs, which are chained to it, checks that copy and notes it
```

Accepting is the lighter choice than a new snapshot: the rest of the baseline stays as it was, so a change you haven't looked at yet still shows up.

To accept by pattern instead of by id, give a page glob and a check glob:

```sh
rhythm soundcheck accept --page '/team/*' --check 'visual/*'
```

That accepts every visual change on the team pages. Either glob alone matches everything for the other. Ids and globs don't mix. `--page '*' --check 'a11y/*'` takes a whole accessibility backlog and prints `Accepted <n> changes` once.

### Accept as an ignore rule

A known problem you plan to fix later belongs in the committed config, where it is reviewed with the code, rather than in the snapshot. `--ignore` writes an [ignore rule](#ignore) per id instead of accepting:

```sh
rhythm soundcheck accept 6b8bcc6f --ignore --reason "wishlist button is rebuilt in the next sprint" --until 2026-12-31
```

```text
Ignored 6b8bcc6f keyboard/not-operable (/a11y/wishlist.php): rule added to .soundcheck/config.yaml
Commit .soundcheck/config.yaml to share the rules; the snapshot is unchanged.
```

The rule carries the change's check, pages and source, your reason and the date:

```yaml
ignore: [{check: keyboard/not-operable, pages: [/a11y/wishlist.php], source: '#save', until: "2026-12-31", reason: wishlist button is rebuilt in the next sprint}]
```

`--reason` is required (`--ignore needs --reason: say why this is known`); `--reason` and `--until` go with `--ignore` only (`--reason and --until go with --ignore`). `--ignore` takes ids, not `--page` and `--check`.

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
| `source` | the change's source, a glob where only `*` is special, so a selector copied from a report, such as `input[name="email"]`, matches itself |
| `until` | the last day the rule applies, as `2026-12-31` |
| `reason` | shown with the ignored change |

Every field given must match. The rules apply after every comparison (after the confirm pass). A matching change moves to `ignored`: it is counted, never fails `--fail-on` and doesn't show in the list. The summary says how many:

```text
Something broke - 1 broke, 2 new, 0 changed, 0 slower, 1 fixed - compared with snapshot "before-update" (1m ago), 4 pages, 1 change ignored, 5s
```

In `--json` the change is under `ignored` with its `reason`; `summary.ignored` counts them. An expired rule, or one that matched nothing, adds a warning to the comparison (`ignore rule 1 (check health/js-error, pages /) expired on 2020-01-01; its changes count again`).

Mistakes in a rule stop the command before the run, exit 2:

```text
rhythm soundcheck: .soundcheck/config.yaml line 1: ignore: rule 1: until "soon" isn't a date like 2026-12-31
rhythm soundcheck: .soundcheck/config.yaml line 2: unknown setting "ignore.page" (did you mean "ignore.pages"?)
```

A rule with no `check`, `title`, `pages` or `source` reads `rule 1 has nothing to match: give it a check, a title, pages or a source`.

A rule that matched nothing on pages the run checked gets a note after the run, so stale rules don't pile up: `note: ignore rule 2 (a11y/color-contrast on /legal/*) matched nothing; the problem looks fixed: remove it`. Ignored accessibility problems still count against their WCAG criteria in the [EAA](Soundcheck-EAA) counts and the statement draft.

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

New problems - 0 broke, 1 new, 0 changed, 0 slower, 0 fixed, 1 unstable - compared with snapshot "before" (just now), 4 pages, 3s
```

Here `/shop.php` answered 500 once, then 200 on the second visit. Site-level changes, such as a broken link or a missing favicon, can't be confirmed by visiting a page again and keep their class.

`--confirm=off` skips the pass (also `--confirm=false`, `0` or `no`). The header then reads `Confirm: off (--confirm=off): BROKE and NEW pages aren't visited again`. If the second visit can't run, the comparison gets a warning and its changes stand: `confirm: couldn't visit the pages again (<why>); BROKE and NEW changes aren't confirmed`.

The pass sees every module's issues on its second visit, a11y and seo included. Under `--eaa` the keyboard walk runs again for keyboard issues: an issue the re-walk doesn't find is FIXED only when that re-walk finished clean, otherwise it stays UNSTABLE. A page whose a11y or keyboard audit didn't finish on the second visit moves its NEW or BROKE change to UNSTABLE, never to FIXED.

---
<sub>← [Reading results](Soundcheck-Reading-Results) · [Home](Home) · [Discovery](Soundcheck-Discovery) →</sub>
