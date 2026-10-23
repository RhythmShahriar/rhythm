The health module checks that every page loads and renders without errors. This page lists everything it reports, the level of each issue, and how it becomes a change in a comparison.

Every issue has a check ID, such as `health/php-error`, shown in the HTML report and the JSON. In a comparison, a new **error** is BROKE and a new **warning** or **notice** is NEW; see [Reading results](Soundcheck-Reading-Results#the-change-classes).

## Status and loading

| Check | Level | Title | When |
|---|---|---|---|
| `health/unreachable` | error | `Page didn't load: <reason>` | the browser couldn't load the page at all, e.g. `net::ERR_CONNECTION_REFUSED` or `timed out` |
| `health/status` | error | `Server error 500` | the page answered 5xx |
| `health/status` | error | `Page not found (404)` | the page answered 404 or 410 |
| `health/status` | error | `Page blocked (403)` | any other 4xx |
| `health/redirect-chain` | notice | `Redirects 3 times before loading` | more than two redirects; the detail lists each hop |

A page that answered 200 in the snapshot and 404 now is the classic BROKE:

```text
[BROKE] Page not found (404) - /spring-offer/
  fix: Restore the page, or redirect it and update links to it.
```

Pages that time out get `Page didn't load: timed out`. The limit is `--timeout` per page (default 30 s).

## PHP errors

When PHP prints an error into the page (`display_errors` on, as on many staging sites), Soundcheck reads it from the rendered text, with the file and line:

| Check | Level | PHP message |
|---|---|---|
| `health/php-error` | error | `Fatal error`, `Parse error`, `Recoverable fatal error` |
| `health/php-error` | warning | `Warning` |
| `health/php-error` | notice | `Notice`, `Deprecated`, `Strict Standards` |

```text
[NEW] PHP Warning: Undefined array key "layout" - /, /about/, /blog/ +33 more
  source: themes/acme/footer.php:42
  fix: Fix the code at the source line, and turn display_errors off in production so visitors never see these.
```

The source is shortened to the part you recognize: `/var/www/html/wp-content/themes/acme/footer.php` becomes `themes/acme/footer.php`, and paths under `wp-includes/`, `wp-admin/`, `vendor/`, `app/`, `src/` or `public/` keep that folder. Long messages are cut at 140 characters so the same error groups across pages and runs. Up to 20 PHP errors are read per page.

> [!TIP]
> On staging, turn `display_errors` on (or `WP_DEBUG` and `WP_DEBUG_DISPLAY`) while you snapshot and compare. Errors that PHP only logs are invisible from the browser; the module still catches their effects (a white screen, the critical error screen, a 500).

## Error pages

Only the first of these that matches is reported:

| Check | Level | Title | Detects |
|---|---|---|---|
| `health/wp-critical` | error | `WordPress "critical error" screen` | "There has been a critical error on this website" |
| `health/database` | error | `Database error on the page` | "Error establishing a database connection", `SQLSTATE[…]` |
| `health/error-page` | error | `Application error page` | Laravel and Symfony error pages, Ignition, a stack trace, an uncaught exception |
| `health/white-screen` | error | `Blank page (white screen)` | status 200 with no visible text and no resources loaded |

The fix for the critical error screen points at the email WordPress sends to the admin and at `wp-content/debug.log` with `WP_DEBUG_LOG` on.

## JavaScript errors

Uncaught exceptions, `console.error()` calls and errors the browser logs (a blocked script, a CSP violation) are recorded during the desktop load and the mobile reload.

| Check | Level | Title |
|---|---|---|
| `health/js-error` | warning | `JS error: <message>` |

```text
[NEW] JS error: Uncaught TypeError: Cannot read properties of undefined (reading 'init') - /shop/
  source: https://staging.yoursite.com/wp-content/plugins/store-locator/js/locator.js:1
  fix: Open the browser console on this page to see the full stack.
```

The source is the script and line, without its query string (`?ver=2.0`), so a plugin's error groups across pages. Numbers in the message are replaced with `N` so timestamps and IDs don't split one error into many. An error in a `<script>` inside the page has the source `inline script`, since its line moves with the content. Errors caused by clicking are reported by the [interact module](Soundcheck-Interactions) instead.

## Failed requests

Every request the page makes is watched.

| Check | Level | Title | When |
|---|---|---|---|
| `health/failed-request` | error or warning | `POST /wp-admin/admin-ajax.php returned 500` | an XHR or fetch call, or any request to `admin-ajax.php` or `/wp-json/`, answered 4xx or 5xx; 5xx and 403 are errors, the rest warnings |
| `health/failed-request` | warning | `Request failed (ERR_NAME_NOT_RESOLVED): <url>` | a request got no answer at all |
| `health/broken-asset` | warning | `Broken image: /wp-content/uploads/team.jpg (404)` | an image, script, stylesheet or font answered 4xx or 5xx |
| `health/favicon` | notice | `No favicon` | `/favicon.ico` is missing |

Same-site URLs are shown as paths; other hosts in full.

## Broken links

At the end of a run, Soundcheck tests links on the visited pages that point at same-site pages it didn't visit (grouped or excluded pages, pages past the limit), with a `HEAD` request, falling back to `GET` when the server doesn't support `HEAD`. It tests up to 300 such links, four at a time and at the run's pace. Pages it did visit use the status it saw.

| Check | Level | Title |
|---|---|---|
| `health/broken-link` | warning | `Broken link to /spring-offer (404)` |

A link answering 404, 410 or 5xx is reported once, with every page that links to it. Links to `/wp-admin`, logout links and files (images, PDFs, CSS, JS) aren't tested.

## Mixed content and indexing

| Check | Level | Title | When |
|---|---|---|---|
| `health/mixed-content` | warning | `Insecure (http://) resource on an https page` | an https page loads something over `http://`; the fix suggests `wp search-replace` |
| `health/no-title` | notice | `Page has no <title>` | a 200 page without a title |
| `health/noindex` | notice | `Page is set to noindex` | `noindex` in a robots or googlebot meta tag, or the `X-Robots-Tag` header |
| `health/not-responsive` | notice | `Page renders 980 px wide on a phone` | the phone screenshot is wider than 500 px: no viewport meta tag, or a fixed-width element |

A `noindex` that appears after a change is worth a look: WordPress adds one to every page when "Discourage search engines" is ticked, a common accident when a staging database is copied to production.

```text
[NEW] Page is set to noindex - /services/
  noindex, nofollow
  fix: If this page should be in search results, remove the noindex (WordPress: Settings → Reading, or your SEO plugin).
```

## Content changes

In a comparison, the health module also reports, as CHANGED:

| Check | Title | When |
|---|---|---|
| `health/title` | `Page title changed` | the `<title>` differs; before → after shows both |
| `health/content-lost` | `Page lost most of its text` | a page that had more than 400 characters of visible text now has less than half |
| `health/redirect` | `Page now ends somewhere else` | the page redirects to a different place than before |

```text
[CHANGED] Page title changed - /about/
  About – Acme Studio → About us – Acme Studio
```

Pages that fail (4xx, 5xx, didn't load) are left out of these, since their status is already reported.

---
<sub>← [Discovery](Soundcheck-Discovery) · [Home](Home) · [Visual changes](Soundcheck-Visual) →</sub>
