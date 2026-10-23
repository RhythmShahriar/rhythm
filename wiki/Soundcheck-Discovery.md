How Soundcheck finds the pages it checks: sitemaps and links, grouping pages by URL pattern, the page limit, include and exclude patterns, `robots.txt`, and the URLs it never visits.

Discovery only happens in `snapshot`, `run` and the `--against` site of a two-site compare. `compare` re-checks exactly the snapshot's pages, so the two runs line up.

## Where pages come from

1. **The start page.** The URL you give, after redirects: `staging.yoursite.com` may become `https://www.staging.yoursite.com/`. The path is kept, so `rhythm soundcheck run yoursite.com/shop/` starts at `/shop/`.
2. **`robots.txt`.** Read once, for its rules and its `Sitemap:` lines.
3. **Sitemaps.** The ones `robots.txt` names, or else `/sitemap.xml`, `/wp-sitemap.xml` (WordPress core) and `/sitemap_index.xml` (Yoast, Rank Math and others). Sitemap indexes are followed three levels deep, gzipped sitemaps are read, and at most 50,000 URLs are taken from them.
4. **Links.** `<a href>`, `<area href>` and `<link rel="next|prev">` on the pages found so far, breadth-first. Links marked `rel="nofollow"` or `download` aren't followed.
5. **Links in the rendered page.** While a page is open in the browser, links its scripts added (menus, "load more") join the queue too.
6. **Extra pages** you list with `--page` or `pages.extra`, for pages nothing links to.

The crawl stays on the site: `yoursite.com` and `www.yoursite.com` count as the same site, other hosts don't. URLs are normalized before they're counted: fragments and tracking parameters (`utm_*`, `fbclid`, `gclid` and similar) are dropped, query parameters sorted, and `/about` and `/about/` are one page. Files that aren't pages (images, PDFs, archives, CSS, JS, fonts, feeds) are skipped; the browser checks assets as each page loads them.

The discovery line tells you what was found and why the rest was left out:

```text
  ▸ Pages         37 pages · 19 left out (excluded 6, grouped 13)
```

| Reason | Means |
|---|---|
| `excluded` | matched a [default exclude](#urls-soundcheck-never-visits) or your `--exclude` |
| `not included` | didn't match your `--include` |
| `grouped` | another page with the same URL pattern already stands for it |
| `robots.txt` | `robots.txt` disallows it |
| `page limit` | the limit was reached |
| `not a page` | a file such as an image or a PDF |
| `other site` | a link to another host |

The counts are also in the run's JSON, under `skipped`.

## Grouping by URL pattern

A shop with 3,000 products doesn't need 3,000 product pages checked: they share a template, so a few samples catch a template bug. Soundcheck groups pages by URL pattern and keeps at most `pages.per_group` pages of each (default **3**).

- **Numbers, dates and IDs** always generalize: `/2026/03/spring-sale/` belongs to `/{y}/{m}/{slug}`, `/order/1042/` to `/order/{n}`, a hex or UUID segment to `{id}`.
- **Slugs** generalize once their folder has more than **10** different children. `/about/` and `/contact/` stay separate pages, while the 11th and later children of `/product/` share the pattern `/product/{slug}`.
- **Pagination** (`?page=`, `?paged=`, `?p=`) groups as `?{page}`. Other query strings are left out of the pattern.

So in the test site above, the first ten blog posts were checked on their own, then three more stood for the other fourteen: `grouped 13`.

```yaml
pages:
  per_group: 3    # pages checked per URL pattern; -1 checks every page
```

Set `per_group: -1` to check every page, for example on a small site or before a big release.

> [!WARNING]
> Grouping drops the query string. On a WordPress site that still uses plain permalinks, every page and archive (`/?page_id=2`, `/?cat=1`) shares the pattern `/` with the home page, and every post (`/?p=123`) shares `/?{page}`, so only three of each are checked. A test WordPress site with plain permalinks reported `3 pages · 3 left out (excluded 1, grouped 2)`. Switch to pretty permalinks (Settings → Permalinks), or set `per_group: -1`.

The group of each page is in the JSON (`group`), and the speed module uses it to report slowdowns per template.

## The page limit

`--limit` (or `pages.limit`) caps the pages checked; the default is **300**. Pages are taken in discovery order: the start page, sitemap URLs, then links breadth-first, so the limit keeps the pages closest to the start.

```sh
rhythm soundcheck snapshot staging.yoursite.com --limit 50
```

```text
Pages: 10 pages · 46 left out (excluded 6, page limit 40)
```

## Include and exclude

Patterns are globs matched against the path **and query** of each URL, such as `/shop/?orderby=price`. `*` matches any run of characters, including `/`. A pattern without `*` must match exactly.

| Pattern | Matches | Doesn't match |
|---|---|---|
| `/shop/*` | `/shop/`, `/shop/mugs/`, `/shop/?orderby=price` | `/shop` |
| `/blog/*` | `/blog/`, `/blog/hello-world/` | `/news/` |
| `*?*replytocom=*` | `/hello-world/?replytocom=12` | `/hello-world/` |
| `/about/` | `/about/` | `/about/team/` |

`--include` keeps only matching pages; `--exclude` skips matching pages, on top of the defaults. Both take a comma-separated list and can be repeated, and they add to `pages.include` and `pages.exclude` from the config file.

```sh
rhythm soundcheck snapshot staging.yoursite.com --exclude '/tag/*,/author/*'
rhythm soundcheck snapshot staging.yoursite.com/shop/ --include '/shop/*,/product/*'
```

Quote patterns so your shell doesn't expand the `*`.

> [!IMPORTANT]
> With `--include`, start at a page that matches it, as in the second example. The start page is subject to `--include` like any other: if it doesn't match, discovery has no page to follow links from, and only finds pages listed in a sitemap. Without a sitemap, the run stops with `found no pages to check`.

## robots.txt

Soundcheck honors the `robots.txt` group for its user agent (`rhythm-soundcheck`), falling back to `*`, with the usual longest-match rule (`Allow` wins a tie). A disallowed page is left out with the reason `robots.txt`, even when you list it with `--page`.

On your own site, `--ignore-robots` (or `pages.ignore_robots: true`) checks pages `robots.txt` disallows, such as a staging site that blocks everything:

```sh
rhythm soundcheck snapshot staging.yoursite.com --ignore-robots --page /private/
```

```text
Pages: 38 pages · 19 left out (excluded 6, grouped 13)
```

Without `--ignore-robots`, the same run left `/private/` out: `20 left out (excluded 6, grouped 13, robots.txt 1)`.

## Extra pages

Pages nothing links to, such as a landing page or `/checkout/`, can be added with `--page` (repeatable) or `pages.extra`. They follow the same rules as discovered pages: excludes, includes, `robots.txt`, grouping and the limit.

```sh
rhythm soundcheck snapshot staging.yoursite.com --page /landing/spring/ --page /checkout/
```

## URLs Soundcheck never visits

Some URLs do something when you load them: log you out, add to the cart, preview a draft. Discovery never visits these patterns, whatever you include:

| Pattern | Why |
|---|---|
| `/wp-admin*`, `/wp-login.php*`, `/xmlrpc.php*`, `/wp-cron.php*` | admin, login and background endpoints |
| `/wp-json*`, `*/feed`, `*/feed/*` | APIs and feeds, not pages |
| `*?*add-to-cart=*`, `*?*remove_item=*`, `*?*undo_item=*`, `*?*removed_item=*`, `/cart/?*` | WooCommerce cart actions |
| `*?*action=logout*`, `*/logout*`, `*/log-out*`, `*/signout*`, `/my-account/customer-logout*` | logging out |
| `*?*replytocom=*`, `*?*preview=*`, `*?*attachment_id=*`, `*?*customize_changeset_uuid=*` | comment replies, previews, attachment and customizer URLs |
| `*?s=*`, `*&s=*` | search results |

The list is `DefaultExcludes` in [`kit/crawl`](https://github.com/RhythmShahriar/rhythm/blob/master/kit/crawl/url.go), shared by every Rhythm tool that crawls.

## Politeness

Discovery and page visits share the run's pace: at most `--rate` page loads a second per host (default 5), with `--tabs` pages open at once (default 4). Discovery requests carry the same User-Agent, headers, cookie and proxy as the browser. See [CLI reference](Soundcheck-CLI-Reference#requests-and-browser).

---
<sub>← [Reading results](Soundcheck-Reading-Results) · [Home](Home) · [Page health](Soundcheck-Page-Health) →</sub>
