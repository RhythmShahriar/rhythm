Coming from BackstopJS or Lost Pixel? `rhythm soundcheck import` writes `.soundcheck/config.yaml` from your existing config: the site, the pages, the viewports and the selectors you hide. It lists what it couldn't carry over and what to do instead.

```sh
rhythm soundcheck import backstop.json
rhythm soundcheck import lostpixel.config.ts --pages-only
```

## From BackstopJS

```json
{
  "id": "brew",
  "viewports": [
    {"label": "desktop", "width": 1440, "height": 900},
    {"label": "phone", "width": 375, "height": 667}
  ],
  "scenarios": [
    {"label": "Home", "url": "http://127.0.0.1:18931/", "hideSelectors": [".cookie-banner"], "delay": 500},
    {"label": "Shop", "url": "http://127.0.0.1:18931/shop.php?sort=price", "removeSelectors": ["#live-chat"], "delay": 500},
    {"label": "Docs", "url": "http://docs.brew.test/help/", "onReadyScript": "puppet/onReady.js"}
  ]
}
```

```sh
rhythm soundcheck import backstop.json
```

```text
Wrote .soundcheck/config.yaml from backstop.json:
  site: http://127.0.0.1:18931, from scenario "Home"
  pages.extra: /, /shop.php?sort=price (2 pages; crawling still finds the rest of the site)
  viewports: desktop, mobile (desktop 1440 px as desktop; phone 375 px as mobile; Soundcheck checks at its own desktop and phone sizes)
  visual.ignore: .cookie-banner, #live-chat, from hideSelectors and removeSelectors (hidden before screenshots, not removed from the page); they now apply to every page
Not imported:
  - scenario "Docs": http://docs.brew.test/help/ isn't on http://127.0.0.1:18931, so it isn't checked
  - delay on scenarios "Home" and "Shop": Soundcheck waits for each page to settle by itself
  - onReadyScript on scenario "Docs": scripts aren't run; a flow can click and type before checking (flows init writes one)
Next: rhythm soundcheck snapshot, then rhythm soundcheck compare after your change
```

The file it wrote:

```yaml
# Imported from backstop.json. Settings left out keep their defaults; the
# init command writes a file that lists them all.
site: http://127.0.0.1:18931
pages:
  extra:
    - /
    - /shop.php?sort=price
viewports:
  - desktop
  - mobile
visual:
  ignore:
    - .cookie-banner
    - '#live-chat'
```

How it maps:

| BackstopJS | Soundcheck |
|---|---|
| `viewports` | `desktop` for widths over 600 px, `mobile` for 600 px or less |
| scenario `url`s on the site | `pages.extra`; crawling stays on, so the rest of the site is checked too |
| the first scenario's site, or `--site` | `site:`, so `snapshot` and `compare` need no URL |
| `hideSelectors`, `removeSelectors` | `visual.ignore` (hidden, not removed, on every page) |
| `delay`, `readyEvent`, `readySelector` | not needed: Soundcheck waits for each page to settle |
| `onBeforeScript`, `onReadyScript`, hover and click selectors | not run; write a [flow](Soundcheck-Forms-And-Flows#flows) |
| `misMatchThreshold` and the rest | listed under `Not imported:` with what to use instead |

A `backstop.js` file is code. Save it as JSON first: `node -p "JSON.stringify(require('./backstop.js'))" > backstop.json`.

## From Lost Pixel

A Lost Pixel config is TypeScript or JavaScript. Soundcheck never runs it, so it reads only the page paths, as text, with `--pages-only`:

```sh
rhythm soundcheck import lostpixel.config.ts --pages-only --site http://127.0.0.1:18931 --force
```

```text
Wrote .soundcheck/config.yaml from lostpixel.config.ts:
  pages.extra: /, /shop.php (2 pages; crawling still finds the rest of the site)
Not imported:
  - path `/blog/${slug}` is computed, not written out; add it to pages.extra by hand
  - everything but the page paths (--pages-only): breakpoints, masks, thresholds and the rest of the file
Next: rhythm soundcheck snapshot, then rhythm soundcheck compare after your change
```

It reads `pageShots.pages[].path`. A path built at run time (a template with `${...}` or a variable) is listed, not guessed. Without `--pages-only` it stops:

```text
rhythm soundcheck: lostpixel.config.ts is code, which Soundcheck never runs; add --pages-only to read just its Lost Pixel page paths
```

## Flags

| Flag | Description |
|---|---|
| `--site <url>` | The site to check, written as `site:`. Default: the first BackstopJS scenario's scheme and host. |
| `--force` | Replace a `.soundcheck/config.yaml` that's already there. Without it: `.soundcheck/config.yaml already exists (--force replaces it)`. |
| `--pages-only` | Read only the page paths of a Lost Pixel config. |

The new file is checked like any config before it replaces anything. `import` starts no other program. Errors exit 2; only a file that can't be written exits 3.

## After importing

```sh
rhythm soundcheck snapshot        # the imported site:
# …make your change…
rhythm soundcheck compare
```

Soundcheck checks more than pixels, so the first comparison may show problems your old tool didn't look for: PHP warnings, JS errors, dead buttons, a cached cart. See [Reading results](Soundcheck-Reading-Results). `rhythm soundcheck init --force` writes the full config file with every setting commented, if you'd rather start from that.

---
<sub>← [History](Soundcheck-History) · [Home](Home) · [Firefox and WebKit](Soundcheck-Browsers) →</sub>
