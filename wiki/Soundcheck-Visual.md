The visual module screenshots every page at desktop and phone size and, in a comparison, shows what looks different. This page covers how screenshots are taken, the threshold, hiding elements, regions that change on their own, inserted and removed blocks, and how shared changes are grouped.

Visual differences are always **CHANGED**: Soundcheck can't know whether a new look was intended, so it shows you, ranked by how much changed. Check each one, and take a new snapshot once they're all expected.

## Screenshots

| Viewport | Window | Capture |
|---|---|---|
| `desktop` | 1366 × 900 | the full page |
| `mobile` | 390 × 844, as a phone | the full page |

Each page is captured once it has loaded and its network has been quiet for a moment. Before the capture, animations and transitions are paused and text carets hidden, so the same page gives the same pixels. The full page is captured by clipping it at its height, without resizing the window, so layouts sized to the window (`100vh` heroes, sticky footers) look the way a visitor sees them. Pages shorter than the window are captured at the window's height; pages taller than 12,000 px are captured down to that height.

Every page runs in its own background browser window that's treated as focused, so it paints and animates like a page in the front window.

Choose viewports with `--viewport desktop` or `viewports: [desktop]` in the config. Screenshots are kept in the run's `shots/` folder; for each visual change, the before, after and diff images are copied into the comparison's `visual/` folder, so the report's slider works on its own.

## How screenshots are compared

The diff works like [pixelmatch](https://github.com/mapbox/pixelmatch): two pixels differ when their perceptual (YIQ) color distance is over a small threshold, which ignores compression and gamma noise, and anti-aliased edges that render a subpixel differently don't count. Changed pixels close to each other are joined into areas.

Before comparing, Soundcheck lines up the rows of the two screenshots, so a block inserted or removed somewhere on the page doesn't make everything below it look changed (see [Inserted and removed blocks](#inserted-and-removed-blocks)).

A page is reported when what's left changed is more than **`visual.threshold` percent** of the page (default `0.5`):

```yaml
visual:
  threshold: 0.5   # % of a page's pixels that must change to report it, 0 to 100
```

Raise it if small, harmless differences (a date, a counter) keep showing up; lower it to catch a one-word change in a long page. A `Page looks different` change gives the share that changed, how many areas, the largest, and a height change nothing else explains:

```text
[CHANGED] Page looks different (desktop) - /
  12.55% of the page changed, in 1 area; the largest at 233,157 (600×300 px). The page is 120 px taller.
  fix: Open the report to compare the screenshots. If the change is expected, take a new snapshot.
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

A snapshot takes each screenshot **twice**, a little over a second apart, at both viewports. Whatever differs between the two (a slider moving to the next slide, a clock, a rotating testimonial) is recorded as a *dynamic region* for that viewport, and later comparisons ignore changes inside it. Nothing to configure.

When a large part of a page changes on its own, a note says so, because changes there can't be seen:

```text
note: /team/: 37% of the page changes on its own (desktop); visual changes there are ignored
note: /team/: 48% of the page changes on its own (mobile); visual changes there are ignored
```

The note appears when more than 30% of the page moves. If the moving element isn't what you want to check, list it under `visual.ignore` instead: with `ignore: [".slider"]`, the same page snapshotted with no note.

> [!NOTE]
> Dynamic regions are learned in the snapshot only. `run` and the live side of a comparison take one screenshot per viewport, so they're faster than a snapshot.

## Inserted and removed blocks

When a new block (a notice, a banner, a PHP warning, a new section) appears on a page, everything below it moves down. Soundcheck finds the rows that were inserted or removed by lining up the rows of the two screenshots, and checks that the content around them is unchanged. A block found in the same place on several pages is one change:

```text
[CHANGED] New block at the top of 15 pages (desktop) - /, /blog/, /blog/backup-strategy/ +12 more
  A 48 px tall block appeared at the top of each page listed, moving the content below it; that content is unchanged.
  fix: Expected if you added or removed it; otherwise look for a plugin or theme notice, banner or element that appeared.
```

```text
[CHANGED] New block near the bottom of 35 pages (desktop) - /, /about/, /blog/ +32 more
  A 48 px tall block appeared near the bottom of each page listed, moving the content below it; that content is unchanged.
```

The place is `at the top of` (within 150 px of the top), `near the bottom of`, or `N px down on` for a block mid-page, which must start at about the same height on every page. Removed blocks read `Block removed from the top of …`, `Block removed N px down on …`. Blocks of a slightly different height (text that wraps differently) still match.

The height change a block explains is part of its change. A separate `Page got 48 px taller (desktop)` line only appears when nothing else explains a page's new size; size changes under 8 px aren't reported. Wider and narrower pages are reported the same way, which on mobile often means something no longer fits the screen (see also `health/not-responsive` in [Page health](Soundcheck-Page-Health#mixed-content-and-indexing)).

## Grouping

A theme or plugin change usually touches every page the same way. After blocks, Soundcheck groups what's left:

- **An area that changed identically** on several pages (the same position, the same pixels before and after) is one change, named by where it is: `Header changed on 30 pages (desktop)`, `Footer changed on …`, or `Area 370 px down changed on …`. Different shared changes with the same name in one viewport merge into one line.
- **Whatever else changed** on a page is `Page looks different (desktop)`. When more than five pages look different in one viewport, they become one line such as `13 pages look different (mobile)`, whose detail gives the median and largest change (`Median <n>% of each page changed, at most <n>% (<page>, shown).`), with the worst page's screenshots.

Desktop and mobile are compared and grouped separately, so one change usually shows once per viewport.

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
