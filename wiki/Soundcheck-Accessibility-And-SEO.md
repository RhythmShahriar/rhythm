Two modules are off by default and worth turning on for most sites: **a11y** runs axe-core on every page and **seo** compares what search engines read. This page covers turning them on, what each reports and how they show in a comparison.

```sh
rhythm soundcheck snapshot staging.yoursite.com --modules +a11y,seo
# …make your change…
rhythm soundcheck compare
```

`+` adds to the default list (or the config's `modules:`), so the line above runs `health, visual, interact, cache, speed, a11y, seo`. `compare` runs with the snapshot's modules, so you only name them once. To keep them on for the project, set `modules: [health, visual, interact, cache, speed, a11y, seo]` in [`.soundcheck/config.yaml`](Soundcheck-Configuration). Both run after speed on each page; interact still runs last.

## Accessibility

The a11y module runs [axe-core](https://github.com/dequelabs/axe-core) 4.14.0, built into the binary, on every page that loaded with a status below 400. It runs the WCAG 2.0 A and AA rules and reports violations only. Iframes are skipped and each page gets at most 45 seconds.

Each failing element is an issue:

| Part | What it holds |
|---|---|
| check | `a11y/<axe rule id>`, such as `a11y/label`, `a11y/image-alt`, `a11y/color-contrast` |
| title | axe's help text, such as `Form elements must have labels` |
| source | a stable selector for the element, such as `input[name="phone"]` or `main > form > img` |
| detail | the element's HTML (up to 160 characters) and axe's reasons, one per line |
| fix | a link to the rule on dequeuniversity.com |
| level | **warning** when axe rates it critical or serious, **notice** when moderate or minor |

So a new accessibility problem is NEW in a comparison, never BROKE. Problems the snapshot already had aren't shown; ones that went away are FIXED. When a rule fails on more than 20 elements of a page, the first 20 are reported and each detail says so.

A contact form that gained an unlabeled phone field and an image without alt text:

```text
[NEW] Images must have alternative text - /contact.php
  source: main > form > img
  <img src="/beans.svg" width="20" height="10">
  fix: See https://dequeuniversity.com/rules/axe/4.14/image-alt for what the rule asks and how to fix it.
[NEW] Form elements must have labels - /contact.php
  source: input[name="phone"]
  <input name="phone">
  fix: See https://dequeuniversity.com/rules/axe/4.14/label for what the rule asks and how to fix it.
```

The selector starts at the nearest element with a stable id or a `name`, so a change higher up the page doesn't move it. Ids that look generated (long numbers, hex, React's `:r1:`) are skipped. Query strings in `href`, `src` and `action` and anything that looks like a token are masked in the HTML snippet.

> [!WARNING]
> In 1.2.0 the confirm pass doesn't see accessibility issues on its second visit, so the issues above come back as `[UNSTABLE]` instead of `[NEW]` and never fail `--fail-on`. Until that's fixed, compare with `--confirm=off` when a11y is on: `rhythm soundcheck compare --confirm=off`. The output above is from that command.

A page where axe itself fails adds a run warning, `a11y: <path>: <reason>`.

## SEO

The seo module reports no issues of its own. It records what search engines read on each page and on the site, then reports what changed between two runs that both had it on.

Per page it keeps the meta description, the canonical link, the robots meta (and `X-Robots-Tag`), the number of `<h1>` headings, hreflang links, Open Graph tags and the JSON-LD types. Per site it reads `robots.txt` and the sitemap (from robots.txt's `Sitemap:` lines, else `/sitemap.xml`, `/sitemap_index.xml` or `/wp-sitemap.xml`), following sitemap indexes on the same host up to 50 files. The facts go into `seo/` in the run folder.

An SEO plugin update that swapped every canonical for a `noindex`:

```text
[BROKE] Page is now set to noindex - /, /about.php, /contact.php +1 more
  (none) → noindex
  fix: Search engines will drop this page. Remove the noindex (WordPress: Settings → Reading, or the SEO plugin's settings for the page).
[NEW] Canonical link removed - /
  / → –
  fix: Every page should print <link rel="canonical"> with its own URL; check the SEO plugin's settings after the update.
[NEW] Canonical link removed - /about.php
  /about.php → –
  fix: Every page should print <link rel="canonical"> with its own URL; check the SEO plugin's settings after the update.
```

What it reports:

| Check | Class | Title |
|---|---|---|
| `seo/robots-meta` | BROKE | `Page is now set to noindex` (also `none`, or `googlebot: noindex` in X-Robots-Tag) |
| `seo/robots-meta` | CHANGED | `Robots meta changed` (`index`, `follow` and `all` count as saying nothing) |
| `seo/robots-txt` | BROKE | `robots.txt now blocks the whole site`, `robots.txt now blocks every page but the home page`, `robots.txt returns <code>` (5xx) |
| `seo/robots-txt` | CHANGED | `robots.txt is gone (404)`, `robots.txt rules changed` (detail: `Now disallows: /shop/. No longer disallows: /wp-admin/.`) |
| `seo/sitemap` | BROKE | `Sitemap returns <code>` (5xx), `Sitemap is gone (404)` |
| `seo/sitemap` | CHANGED | `Sitemap lists fewer pages`, when it lost more than 20% |
| `seo/canonical` | NEW, CHANGED | `Canonical link removed`, `Canonical URL changed` (adding one isn't reported) |
| `seo/description` | NEW, CHANGED | `Meta description removed`, `Meta description changed` |
| `seo/h1` | NEW, CHANGED | `Page has no <h1> heading anymore`, `Page now has several <h1> headings` |
| `seo/hreflang` | NEW, CHANGED | `Language links (hreflang) removed`, `Language links (hreflang) changed` |
| `seo/og` | NEW, CHANGED | `Open Graph tags removed`, `Open Graph tags changed` |
| `seo/jsonld` | NEW | `Structured data removed: <Type>`, `Structured data is no longer valid JSON` |
| `seo/redirect` | CHANGED | `Page now redirects to <path>`, `Page now redirects to the home page` |

Every change carries its own fix text. A few rules keep the noise down:

- URLs on either run's host compare by path, so comparing staging with production (`--against`) doesn't flag canonicals that name production.
- `robots.txt` counts as blocking the site for `Disallow: /` or `/*` under `User-agent: *`, unless `Allow: /` or `Allow: /*` opens it again. `Allow: /$` alone leaves only the home page open, which is its own BROKE.
- A page that now redirects isn't compared field by field, since that would show the target's facts.
- Pages that errored or answered 400 or more in either run are left to health.
- With seo on, health's `Page now ends somewhere else` and `Page is set to noindex` are dropped on pages where seo reports the same thing, so one cause gives one line.

Site-level changes (`robots-txt`, `sitemap`) have no pages; in JSON their `pages` is `null`.

## Listing every check

`--list-checks` prints every module with whether it's on, then the checks it makes:

```sh
rhythm soundcheck --list-checks
```

```text
a11y: off unless asked for (+a11y)
  a11y/label          warning  Form field without a label
  a11y/<axe rule id>  warning  Every axe-core rule that fails, e.g. a11y/color-contrast (a notice when axe rates it minor or moderate)

seo: off unless asked for (+seo)
  seo/canonical    notice   Canonical link changed
  seo/description  notice   Meta description changed
  seo/robots-meta  warning  Robots meta changed
  seo/h1           notice   Headings changed
  seo/hreflang     notice   Language links changed
  seo/og           notice   Open Graph tags changed
  seo/jsonld       notice   Structured data changed
  seo/redirect     notice   Redirect changed
  seo/robots-txt   warning  robots.txt changed
  seo/sitemap      warning  Sitemap changed
```

The full list (health, visual, interact, cache, speed, echo and exec too) is in the [CLI reference](Soundcheck-CLI-Reference#list-checks).

---
<sub>← [Speed](Soundcheck-Speed) · [Home](Home) · [CI](Soundcheck-CI) →</sub>
