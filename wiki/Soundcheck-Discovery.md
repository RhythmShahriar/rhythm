How Soundcheck finds the pages it checks: sitemaps and the links in each rendered page, grouping pages by URL pattern, the page limit, include and exclude patterns, `robots.txt` and the URLs it never visits.

Discovery only happens in `snapshot`, `run` and the `--against` site of a two-site compare. `compare` re-checks exactly the snapshot's pages, so the two runs line up.

## Where pages come from

1. **The start page.** The URL you give, after redirects: `staging.yoursite.com` may become `https://www.staging.yoursite.com/`. The path is kept, so `rhythm soundcheck run yoursite.com/shop/` starts at `/shop/`. The start page is always checked, whatever `--include` says, because links are found from it.
2. **`robots.txt`.** Read once, for its rules and its `Sitemap:` lines.
3. **Sitemaps.** The ones `robots.txt` names, or else `/sitemap.xml`, `/wp-sitemap.xml` (WordPress core) and `/sitemap_index.xml` (Yoast, Rank Math and others). Sitemap indexes are followed three levels deep, gzipped sitemaps are read and at most 50,000 URLs are taken from them.
4. **Links in each rendered page.** As the browser visits a page, the same-site links in it (`<a href>`, including those its scripts added, such as menus and "load more") join the queue. Those pages are visited in turn. Soundcheck doesn't fetch pages a second time just to read their links: what the browser sees is what's followed.
5. **Extra pages** you list with `--page` or `pages.extra`, for pages nothing links to.

The crawl stays on the site: `yoursite.com` and `www.yoursite.com` count as the same site, other hosts don't. URLs are normalized before they're counted: fragments and tracking parameters (`utm_*`, `fbclid`, `gclid` and similar) are dropped, query parameters sorted. `/about` and `/about/` are one page. Files that aren't pages (images, PDFs, archives, CSS, JS, fonts, feeds) are skipped; the browser checks assets as each page loads them.

The discovery line tells you what the start page and the sitemaps gave and why the rest was left out:

```text
  ▸ Pages         36 pages · 13 left out (grouped 13)
```

Links found while pages render add to that, so the final count can be higher (37 in the same run). The reasons:

| Reason | Means |
|---|---|
| `excluded` | matched a [default exclude](#urls-soundcheck-never-visits) or your `--exclude` |
| `not included` | didn't match your `--include` |
| `grouped` | other pages with the same URL pattern already stand for it |
| `robots.txt` | `robots.txt` disallows it |
| `page limit` | the limit was reached |
| `not a page` | a file such as an image or a PDF |
| `other site` | a link to another host |

The counts for the whole run, rendered links included, are in the run's JSON under `skipped`.

## Grouping by URL pattern

A shop with 3,000 products doesn't need 3,000 product pages checked: they share a template, so a few samples catch a template bug. Soundcheck groups pages by URL pattern and keeps at most `pages.per_group` pages of each (default **3**).

- **Numbers, dates and IDs** always generalize: `/2026/03/spring-sale/` belongs to `/{y}/{m}/{slug}`, `/order/1042/` to `/order/{n}`, a hex or UUID segment to `{id}`.
- **Slugs** generalize once their folder has more than **10** different children. `/about/` and `/contact/` stay separate pages, while the 11th and later children of `/product/` share the pattern `/product/{slug}`.
- **Plain permalinks** group the same way by their query parameter: `/?page_id=2`, `/?page_id=9` and `/?p=7` stay separate until a parameter (`page_id`, `p`, `cat`, `tag`, `author`, `post_type`, `product`, `product_cat`…) has more than 10 values; then they share `/?page_id={slug}`.
- **Pagination** (`?page=`, `?paged=`) groups as `?{page}`. Other query strings are left out of the pattern.

So the first ten pages of a folder are checked on their own. Three more stand for the rest: a shop with 3,000 products is checked through about 13 product pages. In the test site above, the first ten blog posts were checked, then three samples stood for the other fourteen. The same went for the products: `grouped 13`.

```yaml
pages:
  per_group: 3    # pages checked per URL pattern; -1 checks every page
```

Set `per_group: -1` to check every page, for example on a small site or before a big release. The group of each page is in the JSON (`group`). The speed module uses it to report slowdowns per template.

## The page limit

`--limit` (or `pages.limit`) caps the pages checked; the default is **300**. Pages are taken in discovery order: the start page and sitemap URLs, then links from rendered pages, so the limit keeps the pages closest to the start.

```sh
rhythm soundcheck snapshot staging.yoursite.com --limit 10
```

```text
Pages: 10 pages · 39 left out (page limit 39)
```

## Include and exclude

Patterns are globs matched against the path **and query** of each URL, such as `/shop/?orderby=price`. `*` matches any run of characters, including `/`. A pattern without `*` must match exactly.

| Pattern | Matches | Doesn't match |
|---|---|---|
| `/shop/*` | `/shop/`, `/shop/mugs/`, `/shop/?orderby=price` | `/shop` |
| `/blog/*` | `/blog/`, `/blog/hello-world/` | `/news/` |
| `*?*replytocom=*` | `/hello-world/?replytocom=12` | `/hello-world/` |
| `/about/` | `/about/` | `/about/team/` |

`--include` keeps only matching pages (and the start page); `--exclude` skips matching pages, on top of the defaults. Both take a comma-separated list and can be repeated. They add to `pages.include` and `pages.exclude` from the config file.

```sh
rhythm soundcheck snapshot staging.yoursite.com --exclude '/tag/*,/author/*'
rhythm soundcheck run staging.yoursite.com --include '/shop/*'
```

Quote patterns so your shell doesn't expand the `*`. On a site without a sitemap, the second command checked the home page, found `/shop/` among its links and went on from there:

```text
Pages: 1 page
[1/2] / 200 1.04s (ok)
[2/2] /shop/ 200 1.03s (ok)
```

Excluded pages are never requested at all, not even by the [broken link check](Soundcheck-Page-Health#broken-links).

## robots.txt

Soundcheck honors the `robots.txt` group for its user agent (`rhythm-soundcheck`), falling back to `*`, with the usual longest-match rule (`Allow` wins a tie). A disallowed page is left out with the reason `robots.txt`.

On your own site, `--ignore-robots` (or `pages.ignore_robots: true`) checks pages `robots.txt` disallows, such as on a staging site that blocks everything:

```sh
rhythm soundcheck snapshot staging.yoursite.com --ignore-robots
```

## Extra pages

Pages nothing links to, such as a landing page or `/checkout/`, can be added with `--page` (repeatable) or `pages.extra`. You asked for them by name, so they get past `--include`, `robots.txt` and grouping; only excludes and the limit still apply.

```sh
rhythm soundcheck run staging.yoursite.com --page /private/
```

```text
Pages: 37 pages · 13 left out (grouped 13)
⋮
[37/38] /private/ 200 1.03s (ok)
```

Here `/private/` is disallowed in `robots.txt` and linked from nowhere. It was checked anyway.

## URLs Soundcheck never visits

Some URLs do something when you load them: log you out, add to the cart, preview a draft. Soundcheck never visits or requests these patterns, whatever you include:

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

Discovery, page visits and every plain request share the run's pace: at most `--rate` page loads a second per host (default 5), with up to `--tabs` pages open at once (default 8). The rate is what keeps a run polite; more tabs only help when pages are slow. Discovery requests carry the same User-Agent, headers, cookie and proxy as the browser. See [CLI reference](Soundcheck-CLI-Reference#requests-and-browser).

---
<sub>← [Reading results](Soundcheck-Reading-Results) · [Home](Home) · [Page health](Soundcheck-Page-Health) →</sub>
