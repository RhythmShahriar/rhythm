The cache module checks that a site's page cache works and is safe: which layer serves pages and how often, that private pages are never cached, that logged-in content never reaches visitors, that cached pages don't carry expired WordPress nonces and that cached copies aren't out of date.

Everything except the leak test is done with plain HTTP requests, without cookies, at the run's polite pace, after all pages have been visited.

## Cache layers

Soundcheck recognizes caches from their headers and from the comments cache plugins leave in the HTML:

| Kind | Layers and evidence |
|---|---|
| CDN | Cloudflare (`cf-cache-status`, `cf-ray`), QUIC.cloud (`x-qc-cache`), Bunny CDN (`cdn-cache`), Sucuri (`x-sucuri-cache`), CloudFront and Fastly (`x-cache` with their own headers) |
| Server and host | Varnish (`x-varnish`, `via`), nginx (`x-cache-status`, `x-proxy-cache`, `x-nginx-cache`, `x-fastcgi-cache`), LiteSpeed (`x-litespeed-cache`), Kinsta, WP Engine, WordPress.com, Hostinger, Pantheon, any proxy that sends `x-cache` |
| WordPress plugin | WP Rocket, WP Super Cache, W3 Total Cache, LiteSpeed Cache, WP Fastest Cache, Hummingbird, Cache Enabler (from their HTML comments and time stamps) |

Plugin comments carry the time the HTML was generated; a stamp older than a few seconds means the page came from the plugin's cache. Stamps in the site's local time are converted with the WordPress time zone from the REST API (`gmt_offset`).

## Hit or miss

The browser's visit warms each page. Then Soundcheck fetches up to 40 public pages (one per URL pattern first) twice more. It reads the second answer: `hit`, `miss`, `bypass`, `expired`, `stale`, `dynamic` or `unknown`. It notes the layer that decided it and the evidence. The facts are kept per page in the run's JSON under `cache`:

```json
{"age": "10", "cache_control": "max-age=86400", "first": "hit", "header": "x-cache-status: HIT", "layer": "nginx", "layers": "nginx", "status": "hit", "ttfb_cached_ms": "0", "ttfb_ms": "0", "ttfb_uncached_ms": "0"}
```

The run's summary, `cache` in the run JSON, has the pages fetched, hits, `hit_rate` (percent), layers and the median time to first byte of cached and uncached pages.

| Check | Level | Title | When |
|---|---|---|---|
| `cache/none` | notice | `No page cache detected` | no page carried a cache header or a cache plugin comment |
| `cache/never-cached` | warning (notice for a CDN) | `Pages are never served from the page cache` | a cache layer is there; 0 pages came from it on a second fetch |

A CDN that passes HTML through is common and only a notice. A server or plugin cache that never answers is a warning: it's installed but not working, often because a cookie, a query string or a `Cache-Control: no-cache` header from a plugin bypasses it.

In a comparison, a cache that served pages in the snapshot and serves none now is one CHANGED line (the `cache/never-cached` issue that comes with it isn't also reported as NEW):

| Check | Title |
|---|---|
| `cache/stopped` | `Page cache stopped working`, before → after such as `30 of 30 pages cached → 0 of 30 pages cached` |

## Private pages

Pages that hold one visitor's cart, order or account must never come from a shared cache: the next visitor would see them, or checkout would break. A page is private when its path is `/cart`, `/basket`, `/checkout`, `/my-account` or `/wp-login.php`, or its body has WooCommerce's `woocommerce-cart`, `woocommerce-checkout` or `woocommerce-account` class. Soundcheck also probes the cart, checkout and account URLs from WooCommerce's script settings and `wp-login.php` on WordPress, even when discovery didn't visit them.

| Check | Level | Title |
|---|---|---|
| `cache/private-cached` | error | `Private page is served from cache` |

```text
[BROKE] Private page is served from cache - /cart/
  source: nginx
  x-cache-status: HIT; cache-control: max-age=86400; age: 24
  fix: Exclude cart, checkout, account and login pages from every cache layer (nginx), and bypass the cache for requests with woocommerce_items_in_cart, wp_woocommerce_session_* or wordpress_logged_in_* cookies. WooCommerce marks these pages no-cache; a cache rule is overriding that. Purge the cache afterwards.
```

A private page counts as cached when the cache says hit, or when it carries a shared-cache `Cache-Control` and a positive `Age`.

## The leak test

The worst cache bug is serving a page rendered for a logged-in user to everyone: their name, email, admin bar, nonces. The leak test checks for it with a test account.

```sh
export SOUNDCHECK_USER=soundcheck-test
export SOUNDCHECK_PASSWORD='…'
rhythm soundcheck run staging.yoursite.com
```

How it works:

1. Log the test account in, in a separate private browser session (see [Logging in](Soundcheck-Forms-And-Flows#logging-in)).
2. Read who it is: display name, email and username, from the REST API (`/wp/v2/users/me`) or the profile page. This only reads.
3. Visit up to ten pages as the test user, so a misconfigured cache stores them.
4. Fetch each page again anonymously, then once more with a cache-busting query for a fresh anonymous render.
5. Report the page when the anonymous copy contains something the fresh render doesn't: the user's email, username or display name, the user's nonces, the admin bar (`#wpadminbar`), the `logged-in` body class or a "Howdy," greeting.

| Check | Level | Title |
|---|---|---|
| `cache/leak` | error | `Cache leaks logged-in content to visitors` |

```text
[ERROR] Cache leaks logged-in content to visitors - /, /about/, /services/ +6 more
  source: nginx
  fix: Purge the page cache now. Then make every cache layer (nginx) bypass requests carrying wordpress_logged_in_* or other session cookies, and never store responses rendered for a logged-in user.
```

The detail lists what leaked, **masked** to its first two characters:

```text
Anonymous visitors get a page rendered for the logged-in test user, served by x-cache-status: HIT: display name (Sa***), wpApiSettings nonce of the logged-in user (9b***), admin bar (#wpadminbar), body class "logged-in", "Howdy," greeting
```

The test is skipped, with a note, when:

| Note | Why |
|---|---|
| `cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PASSWORD to a test account to run it` | no credentials, or `--no-login` |
| `cache: skipped the cache leak test on a production URL: a leaking cache would show the test account to real visitors. Run it on staging` | the site is listed under `production` |
| `login failed, so the checks that need it were skipped: …` | the test account couldn't log in; the message says why. The note about missing credentials follows it. |
| `cache: leak test skipped pages whose anonymous fetch looked logged in; don't pass login cookies with --cookie or -H` | your own headers made the "anonymous" requests logged in |

> [!WARNING]
> The production refusal matters: if the cache does leak, the test itself puts the test account's pages in front of real visitors. Run the leak test on staging, or on a copy with the same cache setup.

## Expired nonces

WordPress nonces (the security tokens in forms, AJAX and REST calls) are valid for 12 to 24 hours. A page cached for longer serves tokens WordPress no longer accepts: contact forms, add to cart and AJAX fail on the cached copy. They still work for you when you're logged in or the cache is cold.

When a cached page carries a nonce (`_wpnonce`, `wpApiSettings`, Contact Form 7, WooCommerce and other `*nonce` fields) and the copy is at least 12 hours old, Soundcheck reports it, as a warning, or an error past 24 hours:

| Check | Level | Title |
|---|---|---|
| `cache/expired-nonce` | warning or error | `Cached page carries an expired security token` |

```text
[ERROR] Cached page carries an expired security token - /team/
  source: nginx
  The cached copy is at least 25 h old (Age header) and carries wpApiSettings nonce; WordPress rejects nonces after 12–24 hours, so forms, add to cart and AJAX on this copy fail.
  fix: Lower the page cache lifetime below 10 hours (nginx), or load nonces with AJAX after the page loads (e.g. your cache plugin's nonce/fragment option), so no visitor gets a page with a token WordPress no longer accepts.
```

The age comes from the `Age` header or the cache plugin's time stamp. A stamp in an unknown time zone is discounted by 14 hours, so the age is never overstated.

## Stale cache

When a page came from the cache, Soundcheck also fetches a fresh render with a cache-busting `?soundcheck=…` query and compares the main content (the `<main>`, `<article>` or content area, without scripts and forms). If they differ, it fetches a second fresh render: only when the two fresh renders agree is the cached copy reported as out of date, so random widgets and rotating content don't count.

| Check | Level | Title |
|---|---|---|
| `cache/stale` | warning | `Cached page is out of date` |

```text
[NEW] Cached page is out of date - /pricing/
  source: nginx
  The cached copy (x-cache-status: HIT, 1 min old) differs from a fresh render; only in the cached copy: "$49"; only in the fresh page: "$59"
  fix: Purge nginx when content changes: enable your cache plugin's purge-on-update for related pages (home, archives, product and category pages), or connect it to your CDN so purges reach the edge.
```

When the cache ignores query strings and serves the same copy for the busted URL, there's nothing fresh to compare with; the page's facts say `busted: ignored`.

## Requests the module makes

All `GET`, without cookies, at the run's pace: two fetches per page (up to 40 public pages plus every private page), a cache-busting fetch or two for each hit, the WordPress REST index once for the time zone. The leak test adds the test user's own page views, a REST call for its details and two anonymous fetches per page. Skip the module with `--skip cache`.

---
<sub>← [Forms and flows](Soundcheck-Forms-And-Flows) · [Home](Home) · [Speed](Soundcheck-Speed) →</sub>
