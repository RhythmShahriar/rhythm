This tutorial takes you through a full before-and-after check: take a snapshot, make a change, compare and read the HTML report. It takes about ten minutes, most of it making your change.

You need a site you own or have permission to test, ideally a staging copy. The examples use `staging.yoursite.com`; replace it with your own. The output on this page comes from a small four-page test site on `127.0.0.1`, where the "change" is a theme update that went wrong.

## 1. Check the install

```sh
rhythm soundcheck --version
```

```text
rhythm-soundcheck 1.2.0
```

If it says `rhythm: soundcheck isn't installed`, run `rhythm install soundcheck`. If `rhythm` isn't found at all, follow [Install](Install).

Soundcheck needs Chrome or Chromium. If neither is installed, it downloads Chrome for Testing once, the first time it needs a browser. It checks the download against the checksum built into Soundcheck. It tells you where it put it. Want to see a result before pointing it at your site? `rhythm soundcheck demo` runs the whole loop on a small built-in site. See [FAQ](Soundcheck-FAQ#chromium-download-and-update).

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
like a visitor at a polite pace. It only submits forms when you turn on the
submit level, never on sites listed as production. This notice is shown once.
```

While it runs, a progress bar counts pages. When it's done:

```text
  ♪ rhythm soundcheck 1.2.0                             http://127.0.0.1:18931
  ────────────────────────────────────────────────────────────────────────────

  ▸ Modules       health, visual, interact, cache, speed
  ▸ Levels        inspect, click
  ▸ Viewports     desktop, mobile
  ▸ Production    none listed: submit and flows run on any site
  ▸ Snapshot      "before-update"
  ▸ Pages         4 pages to start
  · Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)

  WARNINGS  1 ────────────────────────────────────────────────────────────────

 ▌  WARNING    Text files are sent without compression                4 pages
 ▌             /, /about.php, /contact.php +1 more
 ▌             → Turn on gzip or Brotli on the server (Apache mod_deflate/mo…

  NOTICES  2 ─────────────────────────────────────────────────────────────────

    ·  Static files aren't cached by browsers for long              /shop.php
    ·  No page cache detected                                       site-wide

 ╭──────────────────────────────────────────╮
 │  SNAPSHOT BEFORE-UPDATE   saved          │
 │  ✗ 0 errors   ▲ 1 warning   · 2 notices  │
 ╰──────────────────────────────────────────╯
  4 pages · 12s
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…

  → full report: .soundcheck/snapshots/before-update/report.html
  → next: make your change (update a plugin, deploy…), then run: rhythm soundcheck compare
```

- **The steps** at the top say how the run is set up: the modules and interaction levels, the two viewports, the [production guard](Soundcheck-Interactions#the-production-guard) and the browser.
- **Pages** says how many pages discovery found to start with (from the sitemap and the start page) and why others were left out. On a bigger site you'd see `36 pages · 13 left out (grouped 13)`: once a folder has more than ten pages, Soundcheck keeps three samples of the pattern. Links found while the pages render can add more. See [Discovery](Soundcheck-Discovery).
- **The issues** are problems the site has *now*, before your change. A snapshot reports them so you know the starting point; the comparison later shows only what changed.
- **The `!` notes** say what limits the result. The cache leak test needs a test account; it's covered in [Cache](Soundcheck-Cache#the-leak-test), skip it for now. On a page with a slider you'd also see that Soundcheck learned which part of the page changes on its own and will ignore it (see [Visual changes](Soundcheck-Visual#regions-that-change-on-their-own)).
- **The full report** is the snapshot's HTML report, with every page's details. Open it now to see what the baseline looks like.

The snapshot is in `.soundcheck/snapshots/before-update/`. It became the *latest* snapshot, the one `compare` uses by default.

## 4. Make your change

Update the plugin, switch the theme, upgrade PHP, deploy the branch: whatever you want to check. On the test site, the update printed a PHP warning near the bottom of every page, broke a store locator script on the home page and removed the About page.

## 5. Compare

```sh
rhythm soundcheck compare
```

`compare` doesn't discover pages again: it checks exactly the snapshot's pages on the snapshot's site, with the snapshot's settings, so the two runs line up. Then it shows only the differences:

```text
  ♪ rhythm soundcheck 1.2.0                            http://127.0.0.1:18931/
  ────────────────────────────────────────────────────────────────────────────

  ▸ Site          from .soundcheck/snapshots/before-update/run.json (target)
  ▸ Modules       health, visual, interact, cache, speed
  ▸ Levels        inspect, click
  ▸ Viewports     desktop, mobile
  ▸ Production    none listed: submit and flows run on any site
  ▸ Snapshot      "before-update", 4 pages, taken just now
  ▸ Settings      from snapshot "before-update" (modules health,visual,interact,cache,speed; levels inspect,click; viewports desktop,mobile)
  ▸ Confirm       BROKE and NEW pages are visited again before they're reported; --confirm=off skips it
  ▸ Pages         4 pages to start
  · Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
  ▸ Confirming    visiting 4 pages again, to tell a real break from a flaky one
  · Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
  ▸ Confirming    visiting 4 pages once more, where the change came back
  · Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)

  BROKE  1 ───────────────────────────────────────────────────────────────────

 ▌  BROKE      Page not found (404)                                /about.php
 ▌             → Restore the page, or redirect it and update links to it.

  NEW  3 ─────────────────────────────────────────────────────────────────────

 ▌  NEW        Broken link to /about.php (404)                        3 pages
 ▌             /, /contact.php, /shop.php
 ▌             → Update or remove the link, or redirect the old URL.

 ▌  NEW        PHP Warning: Undefined array key "layout"              3 pages
 ▌             /, /contact.php, /shop.php
 ▌             themes/acme/footer.php:42
 ▌             → Fix the code at the source line and turn display_errors off…

 ▌  NEW        JS error: Uncaught ReferenceError: undefinedStoreLocator is n…
 ▌             /
 ▌             inline script
 ▌             → Open the browser console on this page to see the full stack.

  CHANGED  2 ─────────────────────────────────────────────────────────────────

    ■  New block near the bottom of 3 pages (mobile)                  3 pages
       rhythm soundcheck accept 49d86260 if this is expected
    ■  New block near the bottom of 3 pages (desktop)                 3 pages
       rhythm soundcheck accept 994dc87c if this is expected

  FIXED  1 ───────────────────────────────────────────────────────────────────

    ✓  LCP 90% faster                                               /shop.php

 ╭──────────────────────────────────────────────────────────────╮
 │  SOUNDCHECK   Something broke                                │
 │  ✗ 1 broke   ▲ 3 new   ■ 2 changed   ◆ 0 slower   ✓ 1 fixed  │
 ╰──────────────────────────────────────────────────────────────╯
  compared with snapshot "before-update" (just now) · 4 pages · 5s
  ! cache: skipped the cache leak test; set SOUNDCHECK_USER and SOUNDCHECK_PA…

  → full report: .soundcheck/runs/2026-10-25T15-52-40.244Z/report.html

tip: add --fail-on broke to make this exit 1 in scripts
```

Reading from the top:

- **Settings** and **Confirm**: the compare ran with the snapshot's settings. It visited the pages of BROKE and NEW changes again before reporting them, so a page that failed once by chance would show as UNSTABLE instead.
- **BROKE** is what worked in the snapshot and fails now: the About page answered 200 and now answers 404.
- **NEW** is a problem that wasn't there before. The PHP warning is on 3 pages. Still, it's **one** line, with the file and line it comes from. The links to the deleted page are one line too.
- **CHANGED** is what looks or reads differently. The PHP warning made a block appear near the bottom of 3 pages: one line per viewport, not one per page. Under each is the command that accepts it, if it's what you meant.
- **FIXED** is a problem from the snapshot that's gone, or a page that got measurably faster. Lab timings on a tiny local site move a lot; real slowdowns show as SLOWER.
- **The panel** gives the verdict and the count per class.

[Reading results](Soundcheck-Reading-Results) explains every class, how changes are grouped and the report formats.

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

The report has the same changes as the terminal, with filters for each class and a search box. Visual changes get a **before/after slider** you drag across the screenshots, plus Before, After and Diff views. It's one file with the screenshots inside, so you can attach it to a ticket or send it to a client. See [Reading results](Soundcheck-Reading-Results#the-html-report).

## 7. Decide what's next

- **Something you didn't intend?** Fix it, then run `rhythm soundcheck compare` again. The comparison is always against the same snapshot, so fixed problems disappear from the list.
- **Some changes intended?** Accept them into the snapshot by id. The next compare no longer shows them, while everything else in the baseline stays as it was:

  ```sh
  rhythm soundcheck accept 49d86260 994dc87c
  ```

  ```text
  Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/)
  Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/contact.php)
  Accepted 49d86260 New block near the bottom of 3 pages (mobile) (/shop.php)
  Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/)
  Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/contact.php)
  Accepted 994dc87c New block near the bottom of 3 pages (desktop) (/shop.php)
  Snapshot "before-update" now holds 6 accepted changes. Run rhythm soundcheck compare to check again.
  ```

  Or step through them with `rhythm soundcheck review`. See [Review and accept](Soundcheck-Review-And-Accept).
- **Everything intended?** Take a new snapshot as the new baseline: `rhythm soundcheck snapshot staging.yoursite.com --name after-update`. `compare` uses the latest snapshot.
- **Need a report for the pull request?** `rhythm soundcheck report --md -o soundcheck.md`.
- **Want this on every deploy?** See [CI](Soundcheck-CI).

To see your snapshots:

```sh
rhythm soundcheck snapshots
```

```text
NAME                     PAGES  TAKEN        SITE
before-update            4      just now     http://127.0.0.1:18931/  (latest)
```

`rhythm soundcheck compare --snapshot <name>` compares with an older one. `rhythm soundcheck snapshots delete <name>` removes one.

## Going further

- **Test forms too.** Forms are inspected but not sent by default. On staging, add `--level inspect,click,submit` to submit them with test data. See [Forms and flows](Soundcheck-Forms-And-Flows).
- **Check the cache with a test user.** Set `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` to a test account to run the cache leak test. See [Cache](Soundcheck-Cache).
- **Save settings for the project.** `rhythm soundcheck init` writes `.soundcheck/config.yaml` with every setting. See [Configuration](Soundcheck-Configuration). Coming from BackstopJS? `rhythm soundcheck import backstop.json` writes it from your config. See [Import](Soundcheck-Import).
- **Check accessibility and SEO too.** Add `--modules +a11y,seo` to the snapshot. See [Accessibility and SEO](Soundcheck-Accessibility-And-SEO).
- **Updating plugins?** `rhythm soundcheck wp-update` updates them one by one and names the one that breaks the site. See [WordPress updates](Soundcheck-WP-Update).

---
<sub>← [Soundcheck overview](Soundcheck) · [Home](Home) · [Reading results](Soundcheck-Reading-Results) →</sub>
