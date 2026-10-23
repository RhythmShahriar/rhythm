This tutorial takes you through a full before-and-after check: take a snapshot, make a change, compare, and read the HTML report. It takes about ten minutes, most of it making your change.

You need a site you own or have permission to test, ideally a staging copy. The examples use `staging.yoursite.com`; replace it with your own. The output on this page comes from a small WordPress-like test site, where the "change" is a plugin update that went wrong.

## 1. Check the install

```sh
rhythm soundcheck --version
```

```text
rhythm-soundcheck 1.0.0
```

If it says `rhythm: soundcheck isn't installed`, run `rhythm install soundcheck`. If `rhythm` isn't found at all, follow [Install](Install).

Soundcheck needs Chrome or Chromium. If neither is installed, it downloads Chromium once, the first time it needs a browser, and tells you where it put it. See [FAQ](Soundcheck-FAQ#chromium-download-and-update).

## 2. Pick a folder

Soundcheck keeps snapshots and reports in `./.soundcheck`, next to where you run it. Run it from your project's folder (the site's repository, if it has one) so the snapshots stay with the project:

```sh
cd ~/projects/yoursite
```

Inside a git repository, Soundcheck adds its snapshots and runs to `.git/info/exclude`, so screenshots never get committed by accident.

## 3. Take a snapshot

```sh
rhythm soundcheck snapshot staging.yoursite.com --name before-update
```

You can type a bare hostname; Soundcheck tries `https://` first and falls back to `http://`. `--name` is optional; without it the snapshot is named after the date and time, such as `2026-10-24T00-49-25`.

The first time, Soundcheck prints a one-time reminder:

```text
Note: check only sites you own or are authorised to test. Soundcheck browses
like a visitor at a polite pace, and only submits forms when you turn on the
submit level, never on sites listed as production. This notice is shown once.
```

While it runs, a progress bar counts pages. When it's done:

```text
  ♪ rhythm soundcheck 1.0.0                               staging.yoursite.com
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update"
  ▸ Pages         36 pages · 13 left out (grouped 13)

  WARNINGS  1 ────────────────────────────────────────────────────────────────

 ▌  WARNING    Text files are sent without compression               37 pages
 ▌             /, /about/, /blog/ +34 more
 ▌             → Turn on gzip or Brotli on the server (Apache mod_deflate/mo…

  NOTICES  2 ─────────────────────────────────────────────────────────────────

    ·  CSS and JavaScript block the first render                     37 pages
    ·  Static files aren't cached by browsers for long               37 pages

 ╭──────────────────────────────────────────╮
 │  SNAPSHOT BEFORE-UPDATE   saved          │
 │  ✗ 0 errors   ▲ 1 warning   · 2 notices  │
 ╰──────────────────────────────────────────╯
  37 pages · 54s
  ! /team/: 37% of the page changes on its own (desktop); visual changes ther…
  ! /team/: 48% of the page changes on its own (mobile); visual changes there…
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…
  → next: make your change (update a plugin, deploy…), then run: rhythm soundcheck compare
```

- **Pages** says how many pages discovery found in the sitemap and why others were left out. Here 13 blog posts and products were *grouped*: once a folder has more than ten pages, Soundcheck keeps three samples of the pattern. Links found while the pages render can add more; this run ended with 37. See [Discovery](Soundcheck-Discovery).
- **The issues** are problems the site has *now*, before your change. A snapshot reports them so you know the starting point; the comparison later shows only what changed.
- **The `!` notes** say what limits the result. `/team/` has a slider, so Soundcheck learned that part of the page changes on its own and will ignore it (see [Visual changes](Soundcheck-Visual#regions-that-change-on-their-own)). The cache leak test needs a test account; it's covered in [Cache](Soundcheck-Cache#the-leak-test), skip it for now.

The snapshot is in `.soundcheck/snapshots/before-update/`, and it became the *latest* snapshot, the one `compare` uses by default.

## 4. Make your change

Update the plugin, switch the theme, upgrade PHP, deploy the branch: whatever you want to check. On the test site, the update added a PHP warning to the footer, broke a store locator script, the menu button and the wishlist buttons, deleted a page, made the home page heavier and started caching the cart.

## 5. Compare

```sh
rhythm soundcheck compare
```

`compare` doesn't discover pages again: it checks exactly the snapshot's pages on the snapshot's site, so the two runs line up. Then it shows only the differences:

```text
  ♪ rhythm soundcheck 1.0.0                      https://staging.yoursite.com/
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update", 37 pages, taken 1m ago
  ▸ Pages         37 pages

  BROKE  3 ───────────────────────────────────────────────────────────────────

 ▌  BROKE      Clicking Add to wishlist throws a JS error              /shop/
 ▌             Uncaught ReferenceError: WishlistStore is not defined (6 cont…
 ▌             → Click it with the browser console open to see the stack tra…

 ▌  BROKE      Page not found (404)                            /spring-offer/
 ▌             → Restore the page, or redirect it and update links to it.

 ▌  BROKE      Private page is served from cache                       /cart/
 ▌             nginx
 ▌             x-cache-status: HIT; cache-control: max-age=86400; age: 24
 ▌             → Exclude cart, checkout, account and login pages from every …

  NEW  6 ─────────────────────────────────────────────────────────────────────

 ▌  NEW        Broken link to /spring-offer/ (404)                   37 pages
 ▌             /, /about/, /blog/ +34 more
 ▌             → Update or remove the link, or redirect the old URL.

 ▌  NEW        PHP Warning: Undefined array key "layout"             36 pages
 ▌             /, /about/, /blog/ +33 more
 ▌             themes/acme/footer.php:42
 ▌             → Fix the code at the source line, and turn display_errors of…

 ▌  NEW        Button does nothing: Menu                             34 pages
 ▌             /, /about/, /blog/ +31 more
 ▌             Clicking it changed nothing on the page, made no request and …
 ▌             → Wire up its click handler, or remove the button if it's no …
 ⋮

  CHANGED  5 ─────────────────────────────────────────────────────────────────

    ■  Page looks different (mobile)                                        /
    ■  Page looks different (desktop)                                       /
    ■  New block near the bottom of 35 pages (mobile)                35 pages
    ■  New block near the bottom of 35 pages (desktop)               35 pages
    ■  Page title changed                                             /about/

  SLOWER  3 ──────────────────────────────────────────────────────────────────

    ◆  Page weight 517× higher                               5 KB → 2.3 MB  /
    ◆  LCP 18× slower                                       28 ms → 500 ms  /
    ◆  Plugin heavy-slider now loads assets on pages          0 B → 293 KB  /

 ╭──────────────────────────────────────────────────────────────╮
 │  SOUNDCHECK   Something broke                                │
 │  ✗ 3 broke   ▲ 6 new   ■ 5 changed   ◆ 3 slower   ✓ 0 fixed  │
 ╰──────────────────────────────────────────────────────────────╯
  compared with snapshot "before-update" (1m ago) · 37 pages · 47s
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…
  → full report: .soundcheck/runs/2026-10-24T02-23-17/report.html
```

Reading from the top:

- **BROKE** is what worked in the snapshot and fails now: six identical wishlist buttons that throw when clicked (one line; the selectors are in the detail), a page that answered 200 and now answers 404, and a cart the cache now serves to everyone.
- **NEW** is a problem that wasn't there before. The PHP warning is on 36 pages, but it's **one** line, with the file and line it comes from.
- **CHANGED** is what looks or reads differently. The PHP warning made a block appear near the bottom of 35 pages: one line per viewport, not 35. Check each change was intended.
- **SLOWER** is a measurable slowdown, past a noise floor. The plugin cost line names the plugin that made the home page heavier.
- **The panel** gives the verdict and the count per class.

[Reading results](Soundcheck-Reading-Results) explains every class, how changes are grouped, and the report formats.

> [!TIP]
> Fix from the top: BROKE, then NEW. When a NEW problem is on every page, it usually has one cause, often the plugin or theme you just updated. The `source` (here `themes/acme/footer.php:42`) tells you where to look.

## 6. Open the HTML report

Every compare writes a standalone HTML report into the run's folder. Open it straight away with `--open`:

```sh
rhythm soundcheck compare --open
```

Or open the file the last line points at, or show the last result again at any time:

```sh
rhythm soundcheck report --open
```

The report has the same changes as the terminal, with filters for each class, a search box, and for visual changes a **before/after slider** you drag across the screenshots, plus Before, After and Diff views. It's one file with the screenshots inside, so you can attach it to a ticket or send it to a client. See [Reading results](Soundcheck-Reading-Results#the-html-report).

## 7. Decide what's next

- **Something you didn't intend?** Fix it, then run `rhythm soundcheck compare` again. The comparison is always against the same snapshot, so fixed problems disappear from the list.
- **Everything intended?** Take a new snapshot as the new baseline: `rhythm soundcheck snapshot staging.yoursite.com --name after-update`. `compare` uses the latest snapshot.
- **Need a report for the pull request?** `rhythm soundcheck report --md -o soundcheck.md`.
- **Want this on every deploy?** See [CI](Soundcheck-CI).

To see your snapshots:

```sh
rhythm soundcheck snapshots
```

```text
NAME                     PAGES  TAKEN        SITE
before-update            37     6m ago       https://staging.yoursite.com/  (latest)
```

`rhythm soundcheck compare --snapshot <name>` compares with an older one, and `rhythm soundcheck snapshots delete <name>` removes one.

## Going further

- **Test forms too.** Forms are inspected but not sent by default. On staging, add `--level inspect,click,submit` to submit them with test data. See [Forms and flows](Soundcheck-Forms-And-Flows).
- **Check the cache with a test user.** Set `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` to a test account to run the cache leak test. See [Cache](Soundcheck-Cache).
- **Save settings for the project.** `rhythm soundcheck init` writes `.soundcheck/config.yaml` with every setting. See [Configuration](Soundcheck-Configuration).

---
<sub>← [Soundcheck overview](Soundcheck) · [Home](Home) · [Reading results](Soundcheck-Reading-Results) →</sub>
