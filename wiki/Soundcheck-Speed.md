The speed module measures how fast each page loads, reports pages that got measurably slower after a change, and finds what slows the site down, grouped by cause, including what each WordPress plugin and theme costs. This page covers the lab metrics and their thresholds, regressions and noise floors, the site-wide suggestions and `--lighthouse`.

> [!NOTE]
> Every number is a **lab** measurement: one unthrottled desktop load per page in headless Chrome, on the machine running Soundcheck. Lab numbers are good for spotting changes and their causes. Real visitors on slower phones and networks see higher numbers; for those, use field data such as the Chrome UX Report or your analytics.

## Metrics

| Metric | What it is |
|---|---|
| **TTFB** | time to first byte of the page: how long the server took to start answering |
| **FCP** | First Contentful Paint: when anything first showed |
| **LCP** | Largest Contentful Paint: when the largest element (often the hero image or heading) showed |
| **CLS** | Cumulative Layout Shift: how much content moved after it rendered |
| **TBT** | Total Blocking Time: long JavaScript tasks (over 50 ms) that would block clicks and typing; stands in for INP in a lab |
| **Weight** | bytes transferred for the page's resources |
| **Requests** | how many resources it loaded |

They're kept per page in the run's JSON under `metrics` and shown in the HTML report's page table.

## Page issues

A page outside the good band gets an issue. The bands follow Google's Core Web Vitals ("needs improvement" is a warning, "poor" an error) and web.dev's TTFB guidance:

| Check | Warning | Error |
|---|---|---|
| `speed/lcp` | `Largest Contentful Paint over 2.5 s (lab)` | `Largest Contentful Paint over 4 s (lab)` |
| `speed/cls` | `Layout shift (CLS) over 0.1 (lab)` | `Layout shift (CLS) over 0.25 (lab)` |
| `speed/tbt` | `Total Blocking Time over 200 ms (lab)` | `Total Blocking Time over 600 ms (lab)` |
| `speed/ttfb` | `Server response (TTFB) over 0.8 s (lab)` (notice) | `Server response (TTFB) over 1.8 s (lab)` (warning) |
| `speed/weight` | `Page weighs over 3 MB` (warning) | |
| `speed/requests` | `Page makes over 100 requests` (notice) | |

```text
[WARNING] Total Blocking Time over 200 ms (lab) - /
  fix: Long JavaScript tasks block clicks and typing: defer scripts the page doesn't need at first, and remove plugins that add heavy JS to every page.
```

The title names the band, not the value, so the same problem keeps the same identity between runs; the measured value is in the detail (`TBT 399 ms (lab: …)`).

## Regressions

Lab timings jitter between loads, so a page near 2.5 s LCP would flip between NEW and FIXED on every run. Speed issues are therefore never diffed like other issues. Instead, the speed module compares the metrics themselves and reports a page as **SLOWER** only when a metric got worse by **more than `speed.slower` percent** (default 25) **and** by at least a fixed floor:

| Metric | Floor | Title |
|---|---|---|
| LCP | 300 ms | `LCP 40% slower` |
| Total Blocking Time | 100 ms | `Total Blocking Time 2.5× higher` |
| Layout shift (CLS) | 0.05 | `Layout shift (CLS) 60% higher` |
| Page weight | 100 KB | `Page weight 2112× higher` |
| TTFB | 200 ms | `Server response (TTFB) 50% slower` |

A rise from 20 ms to 30 ms is 50% but under the floor: noise, not news. A value that was 0 before reads `… up from 0`. Pages that got faster or lighter by the same rule are **FIXED**.

```text
[SLOWER] Total Blocking Time up from 0 - /
  0 ms → 399 ms
  Lab measurement, one load per page; a page counts when it's higher by more than 25% and at least 100 ms
  fix: New or updated JavaScript runs longer tasks; check which plugin or theme scripts changed (see the per-plugin cost suggestions).
```

Hits are grouped: when most pages moved (at least three and more than half), it's one site-wide change, such as `LCP 40% slower on 30 pages`, with the median before and after. Otherwise pages that share a URL pattern are one change (`… on 3 /product/{slug} pages`), and the rest one change each.

```yaml
speed:
  slower: 25   # % slower before a page is reported SLOWER
```

### Plugin and theme cost

The comparison also adds up each plugin's and theme's files per page (by their `/wp-content/plugins/<slug>/` or `/wp-content/themes/<slug>/` path) and reports the ones that got much heavier, with the same percentage bar and a 100 KB floor:

```text
[SLOWER] Plugin heavy-slider now loads assets on pages - /
  0 B → 293 KB
  source: plugins/heavy-slider
  Lab measurement: average transferred size of its files per page that loads them
  fix: Check what the plugin update changed, or limit its assets to the pages that use them.
```

`Plugin X got 3× heavier per page` points straight at the plugin an update made heavier; `… no longer loads assets on pages` and `… got 60% lighter per page` are FIXED.

## Site-wide suggestions

At the end of every run, the speed module adds suggestions grouped by cause, each counted across pages, so one cause is one line however many pages it affects:

| Check | Level | Title | When |
|---|---|---|---|
| `speed/oversized-images` | notice, warning from 500 KB wasted per page | `Images are much larger than they're displayed` | an image's natural width is more than twice its displayed width (allowing for retina screens) and wastes at least 20 KB |
| `speed/lazy-load` | notice | `Images below the fold aren't lazy-loaded` | images below the first screen without `loading="lazy"` |
| `speed/render-blocking` | notice, warning from 500 ms | `CSS and JavaScript block the first render` | stylesheets and scripts that must load before anything shows |
| `speed/unused-css`, `speed/unused-js` | notice | `CSS files are mostly unused`, `JavaScript files are mostly unused` | more than half of a file over 2 KB never used on the pages that load it (from Chrome's coverage) |
| `speed/compression` | warning | `Text files are sent without compression` | HTML, CSS, JS or other text over 1 KB without gzip or Brotli |
| `speed/http-version` | notice | `Site isn't served over HTTP/2 or HTTP/3` | an https site serving its own files over HTTP/1.1 |
| `speed/cache-headers` | notice | `Static files aren't cached by browsers for long` | the site's images, scripts, styles or fonts with no or under a week of `max-age` |
| `speed/third-party` | notice or warning | `Heavy third party: <host>` | another host adding over 100 KB, 10 requests or 100 ms of blocking per page |
| `speed/plugin-cost`, `speed/theme-cost` | notice or warning | `Plugin heavy-slider adds weight to pages` | a plugin or theme adding over 100 KB, 10 requests or 100 ms of blocking per page (a warning from 500 KB or 300 ms) |

The detail lists the worst files with their sizes:

```text
notice | Plugin heavy-slider adds weight to pages | Plugin heavy-slider adds 293 KB and 1 request to 1 of 8 pages, with 5 ms of render-blocking loads (lab, per page)
warning | Text files are sent without compression | 10 responses (HTML, CSS, JS or other text) on every page (8) have no gzip or Brotli: /wp-content/plugins/heavy-slider/slider.js (293 KB); /blog/ (HTML) (3 KB); …
```

Each suggestion has a fix with the WordPress, Apache or nginx way to do it. Suggestions are issues like any other: in a comparison, one that appears is NEW and one that's gone is FIXED, but only if the cause appeared or went away, since their titles don't carry numbers.

## --lighthouse

For full Lighthouse audits, `--lighthouse` hands the site to [Unlighthouse](https://unlighthouse.dev/) after the check:

```sh
rhythm soundcheck run staging.yoursite.com --lighthouse
```

It needs Node.js: Soundcheck runs `npx --yes unlighthouse-ci@0.19.1 --site <url> --reporter jsonExpanded` and writes the results to `lighthouse/ci-result.json` in the run's folder. Unlighthouse does its own crawl and takes a while, so it's off by default. Without `npx`, Soundcheck says `--lighthouse needs npx; install Node.js from https://nodejs.org` and carries on. The Lighthouse results are separate files; they don't change Soundcheck's own report.

## Getting stable numbers

- Run Soundcheck on the same machine, with the same load, for the snapshot and the compare. A laptop on battery or a busy CI runner adds noise.
- Compare staging with staging, or production with production. Different servers have different TTFB.
- Check a slowdown twice before acting on a small one: `rhythm soundcheck compare` again.
- Skip the module with `--skip speed` when you don't care about speed for a run.

---
<sub>← [Cache](Soundcheck-Cache) · [Home](Home) · [CI](Soundcheck-CI) →</sub>
