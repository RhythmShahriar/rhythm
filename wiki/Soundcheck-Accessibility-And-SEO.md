Two modules are off by default and worth turning on for most sites: **a11y** runs axe-core on every page and **seo** compares what search engines read. (A third, **keyboard**, comes with [EAA mode](Soundcheck-EAA).) This page covers turning them on, what each reports and how they show in a comparison.

```sh
rhythm soundcheck snapshot staging.yoursite.com --modules +a11y,seo
# …make your change…
rhythm soundcheck compare
```

`+` adds to the default list (or the config's `modules:`), so the line above runs `health, visual, interact, cache, speed, a11y, seo`. `compare` runs with the snapshot's modules, so you only name them once. To keep them on for the project, set `modules: [health, visual, interact, cache, speed, a11y, seo]` in [`.soundcheck/config.yaml`](Soundcheck-Configuration). Both run after speed on each page; interact still runs last.

## Accessibility

The a11y module runs [axe-core](https://github.com/dequelabs/axe-core) 4.14.0, built into the binary, on every page that loaded with a status below 400. It runs the A and AA rules of the run's WCAG version and reports violations only. New snapshots use **WCAG 2.1** (1.2 used 2.0); `--wcag 2.0`, `--wcag 2.2` or `a11y.wcag` in the config choose another. A compare uses the version its snapshot recorded. Each page gets at most 45 seconds.

| WCAG | Runs axe-core's tags | New rules over the version before |
|---|---|---|
| 2.0 | `wcag2a`, `wcag2aa` | |
| 2.1 | plus `wcag21a`, `wcag21aa` | `autocomplete-valid`, `avoid-inline-spacing`, `label-content-name-mismatch` |
| 2.2 | plus `wcag22aa` | `target-size` |

For accessibility evidence for the European Accessibility Act, with a keyboard walkthrough of every page and a statement draft, use [EAA mode](Soundcheck-EAA) (`--eaa`) instead of `+a11y`.

Each failing element is an issue:

| Part | What it holds |
|---|---|
| check | `a11y/<axe rule id>`, such as `a11y/label`, `a11y/image-alt`, `a11y/color-contrast` |
| title | axe's help text, such as `Form elements must have labels` |
| source | a stable selector for the element, such as `input[name="phone"]` or `main > form > img`; `html` for rules about the document, such as `html-has-lang` |
| wcag | the WCAG success criteria it fails and their EN 301 549 clauses, such as `wcag: 1.4.3 · EN 301 549 9.1.4.3` |
| detail | the element's opening tag (up to 160 characters, never what it holds) and axe's reasons, one per line |
| fix | a link to the rule on dequeuniversity.com |
| level | **warning** when axe rates it critical or serious, **notice** when moderate or minor |

So a new accessibility problem is NEW in a comparison, never BROKE. Problems the snapshot already had aren't shown; ones that went away are FIXED. When a rule fails on more than `a11y.max_elements` elements of a page (default 20), the first ones are reported and each detail says so.

Zoom switched off on one page and a promo bar too wide to reflow on another (testbed scenarios s66 and s69), compared with `--modules health,a11y`:

```text
[NEW] Zooming and scaling must not be disabled - /a11y/zoom.php
  source: meta[name="viewport"]
  wcag: 1.4.4; EN 301 549 9.1.4.4
  <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=no, maximum-scale=1">
  fix: See https://dequeuniversity.com/rules/axe/4.14/meta-viewport for what the rule asks and how to fix it.
  rhythm soundcheck accept e8bf3992 if this is expected
[NEW] Page scrolls sideways at 320 px wide - /a11y/promo.php
  source: #promo
  wcag: 1.4.10; EN 301 549 9.1.4.10
  At 320 CSS px the page is 664 px wide; #promo sticks out.
  fix: Let the content wrap at 320 px: relative widths and max-width: 100% instead of fixed widths; wide tables or code in their own scrolling box.
  rhythm soundcheck accept 7939ec10 if this is expected

New problems - 0 broke, 2 new, 0 changed, 0 slower, 0 fixed - compared with snapshot "before" (just now), 2 pages, 2s
```

The [confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable) visited both pages again and saw both problems again, so they are NEW, not UNSTABLE. 1.2.0 took them for UNSTABLE, which 1.2.1 fixed, so no `--confirm=off` is needed with a11y on.

The selector starts at the nearest element with a stable id or a `name`, so a change higher up the page doesn't move it. Ids that look generated (long numbers, hex, React's `:r1:`) are skipped. Query strings in `href`, `src` and `action`, `data-*` values, form values and anything that looks like a token are masked in the tag; so are email addresses, phone, card and IBAN numbers in the tag and in axe's summary.

### Soundcheck's own checks

Three checks come with the module besides axe-core's rules:

| Check | Level | Title | WCAG |
|---|---|---|---|
| `a11y/placeholder-label` | warning | `Form field is labeled only by its placeholder` | 3.3.2 |
| `a11y/reflow` | warning | `Page scrolls sideways at 320 px wide` (WCAG 2.1 and 2.2 only) | 1.4.10 |
| `a11y/overlay` | notice | `Accessibility overlay widget found` | advisory |

- **placeholder-label**: a visible field whose only name is its placeholder (`The only name is the placeholder "<placeholder>", which disappears when someone types.`). Search fields, checkboxes and radio buttons, the only text field of a form with a visible text button and fields already reported as `a11y/label` are left out.
- **reflow**: the page is checked at 320 by 640 CSS px. Tables, code, media, maps and other content that may scroll both ways are left out.
- **overlay**: the page loads accessiBe, UserWay, EqualWeb, AudioEye, Enable, Access Ally or the WordPress plugin pojo-accessibility. Overlay widgets don't make a site accessible.

### Settings

```yaml
a11y:
  wcag: "2.1"        # 2.0, 2.1 or 2.2 (--wcag)
  max_elements: 20   # elements kept per rule and page, 1 to 200
  iframes: false     # true also audits same-origin iframes
```

With `iframes: true`, issues in a frame have selectors such as `#frame-id >>> input[name="email"]`. Frames from another origin are never audited; their hosts are listed under the page's `audits.a11y.frames` in run.json.

### Audits that don't finish

Each page records `audits.a11y` in run.json: `ok`, `timeout` (`axe-core timed out after 45s`), `error` or `skipped` (`status 500`). A page whose audit didn't finish adds a warning such as `a11y: /shop/: axe-core: timed out after 45s; the page's a11y results are missing; the statement marks it`. In a comparison its a11y issues are neither NEW nor FIXED: `not checked: <path> (a11y: <reason>); its issues are kept from the snapshot, not counted as fixed`.

### Comparing with an older snapshot

A snapshot from 1.2 ran the WCAG 2.0 rules. A compare runs the snapshot's rules, so nothing changes until you take a new snapshot. When the rule sets differ, issues of rules the snapshot didn't run are put aside, not NEW: `note: 3 rules didn't run in the baseline (autocomplete-valid, avoid-inline-spacing, label-content-name-mismatch); their issues aren't counted as new. Take a new snapshot to include them`. Under `--eaa`, a 1.2 snapshot is compared with the 2.1 rules and the comparison warns `note: snapshot "<name>" used the WCAG 2.0 rules; take a new snapshot to add the 2.1 rules`.

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
a11y: off unless asked for (+a11y); on with --eaa
  a11y/label              warning  Form field without a label                     wcag 4.1.2
  a11y/<axe rule id>      warning  Any failing axe-core rule                      wcag from the rule's tags
  a11y/placeholder-label  warning  Form field is labeled only by its placeholder  wcag 3.3.2
  a11y/reflow             warning  Page scrolls sideways at 320 px wide           wcag 1.4.10
  a11y/overlay            notice   Accessibility overlay widget found             advisory

keyboard: off unless asked for (+keyboard); on with --eaa
  keyboard/trap               error    Keyboard focus is trapped                       wcag 2.1.2
  ...

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

The keyboard checks are on [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough#the-checks). The full list (health, visual, interact, cache, speed, echo and exec too) is in the [CLI reference](Soundcheck-CLI-Reference#list-checks).

---
<sub>← [Speed](Soundcheck-Speed) · [Home](Home) · [EAA mode](Soundcheck-EAA) →</sub>
