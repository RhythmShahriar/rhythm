The visual module screenshots every page at desktop and phone size and, in a comparison, shows what looks different. This page covers how screenshots are taken, the threshold, hiding elements, regions that change on their own, inserted and removed blocks, and how shared changes are grouped.

Visual differences are always **CHANGED**: Soundcheck can't know whether a new look was intended, so it shows you, ranked by how much changed. Check each one, and take a new snapshot once they're all expected.

## Screenshots

| Viewport | Size |
|---|---|
| `desktop` | 1366 × 900, full page |
| `mobile` | 390 × 844, full page, as a phone |

Each page is captured once it has loaded and its network has been quiet for a moment. Before the capture, animations and transitions are paused and text carets hidden, so the same page gives the same pixels. Pages taller than 12,000 px are captured down to that height.

Choose viewports with `--viewport desktop` or `viewports: [desktop]` in the config. Screenshots are kept in the run's `shots/` folder; for each visual change, the before, after and diff images are copied into the comparison's `visual/` folder, so the report's slider works on its own.

## How screenshots are compared

The diff works like [pixelmatch](https://github.com/mapbox/pixelmatch): two pixels differ when their perceptual (YIQ) color distance is over a small threshold, which ignores compression and gamma noise, and anti-aliased edges that render a subpixel differently don't count. Changed pixels close to each other are joined into regions.

A page is reported when the changed pixels are more than **`visual.threshold` percent** of the page (default `0.5`):

```yaml
visual:
  threshold: 0.5   # % of a page's pixels that must change to report it, 0 to 100
```

Raise it if small, harmless differences (a date, a counter) keep showing up; lower it to catch a one-word change in a long page. A page's `Page looks different` line gives the share that changed, how many areas, and the largest:

```text
[CHANGED] Page looks different (desktop) - /
  13.50% of the page changed, in 5 areas; the largest at 233,157 (600×300 px); a 168 px tall block was inserted 457 px down; the content below it moved down.
```

Pages that failed in either run (an error, a 4xx or 5xx) aren't compared visually; their status is the news.

## Hiding elements

Some elements are never the same twice: a live chat bubble, an ad slot, a cookie banner, a "visitors online" counter. List their CSS selectors and they're hidden (`visibility: hidden`, so the layout doesn't move) before every screenshot, and their area is left out of the comparison:

```yaml
visual:
  ignore: [".ad", "#live-chat", ".cookie-notice"]
```

Selectors that match nothing are fine.

## Regions that change on their own

A snapshot takes each screenshot **twice**, a little over a second apart. Whatever differs between the two (a slider moving to the next slide, a clock, a rotating testimonial) is recorded as a *dynamic region*, and later comparisons ignore changes inside it. Nothing to configure.

When a large part of a page changes on its own, a note says so, because changes there can't be seen:

```text
note: /team/: 37% of the page changes on its own (desktop); visual changes there are ignored
note: /team/: 48% of the page changes on its own (mobile); visual changes there are ignored
```

The note appears when more than 30% of the page moves. If the moving element isn't what you want to check, list it under `visual.ignore` instead: with `ignore: [".slider"]`, the same page snapshotted with no note.

> [!NOTE]
> Dynamic regions are learned in the snapshot only. `run` and the live side of a comparison take one screenshot per viewport, so a run is faster than a snapshot.

## Pages that got taller or shorter

When a new block (a notice, a banner, a new section) appears in the middle of a page, everything below it moves down. A naive diff would call the whole lower page changed. Soundcheck looks for the band of rows that was inserted or removed, lines up the content above and below it, and keeps that alignment when it explains the change better. Then it reports the size change with the band:

```text
[CHANGED] Page got 48 px taller (desktop) - /blog/
  1366×901 px → 1366×949 px
  A 48 px tall block was inserted 0 px down; the content below it moved down.
  fix: Expected if content was added or removed; otherwise look for a new notice, banner or element pushing the layout.
```

Size changes under 8 px aren't reported. Wider and narrower pages are reported the same way, which on mobile often means something no longer fits the screen (see also `health/not-responsive` in [Page health](Soundcheck-Page-Health#mixed-content-and-indexing)).

## Grouping

A theme or plugin change usually touches every page the same way. Soundcheck groups those into one change:

- **A block inserted or removed at the same place** on several pages (the same height, starting within 16 px) is one change: `New block at the top of 12 pages (desktop)`, `New block at the bottom of 13 pages (mobile)`, `Block removed from the top of …`, or `New block 570 px down on 13 pages`.
- **An area that changed identically** on several pages (the same position, the same pixels before and after) is one change, named by where it is: `Header changed on 30 pages (desktop)`, `Footer changed on …`, or `Area 370 px down changed on …`.
- **Pages that changed size by the same amount** are one change: `Page got 25 px taller (mobile)` with `13 pages changed size by the same amount.`
- **Whatever else changed** on a page is `Page looks different (desktop)`, with "besides the changes shared with other pages" when part of it was grouped. When more than five pages look different in one viewport, they become one line with the median and largest change, showing the worst page's screenshots:

```text
[CHANGED] 13 pages look different (mobile) - /blog/backup-strategy/, /blog/caching-101/, /blog/case-study-bakery/ +10 more
  Median 17.13% of each page changed, at most 17.30% (/blog/plugin-roundup/, shown).
  fix: Open the report to compare the screenshots. If the change is expected, take a new snapshot.
```

```text
[CHANGED] New block at the bottom of 13 pages (mobile) - /blog/backup-strategy/, /blog/caching-101/, /blog/case-study-bakery/ +10 more
  On every page listed: a 25 px tall block was inserted 753 px down; the content below it moved down.
  fix: Expected if you added or removed it; otherwise look for a plugin or theme notice, banner or element that appeared.
```

Desktop and mobile are compared and grouped separately, so one change often shows once per viewport.

> [!TIP]
> Open the HTML report for visual changes. The text tells you where and how much; the **Slider**, **Before**, **After** and **Diff** views show what. See [Reading results](Soundcheck-Reading-Results#the-html-report).

## When visual results are noisy

| Symptom | Try |
|---|---|
| The same widget changes every time | `visual.ignore` with its selector |
| Tiny differences on many pages (fonts, dates) | raise `visual.threshold`, e.g. to `2` |
| Ads or embeds load at different times | `visual.ignore` the slots |
| Everything moved after a deliberate redesign | take a new snapshot |
| You only care about errors and speed | `--skip visual` (both runs) |

More in the [FAQ](Soundcheck-FAQ#noisy-visual-diffs).

---
<sub>← [Page health](Soundcheck-Page-Health) · [Home](Home) · [Interactions](Soundcheck-Interactions) →</sub>
