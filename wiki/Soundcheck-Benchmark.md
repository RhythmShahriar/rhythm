How well does Soundcheck catch what breaks real WordPress and PHP sites? This page describes the benchmark: a testbed of 48 deliberately broken sites, how each scenario is run and judged, a control pass that checks Soundcheck doesn't cry wolf on a site that didn't change. It also names the tools it can be compared with. The results are published with each release.

**Soundcheck caught 47 of 47 scenarios, with 0 false alarms in 40 compares of an unchanged site.** The EAA mode scenarios added in 1.3.0 (s49 to s89) are scored apart: **29 of 29 caught, 0 false alarms**; see [EAA mode](#eaa-mode). The run for 1.2.0 was made on 2026-10-24 with a development build of the release. One scenario (s33) needs Echo and wasn't run.

## The testbed

The testbed lives in the Soundcheck repository, in `testbed/`. It runs in Docker:

| Site | Address | Stack |
|---|---|---|
| WordPress | `http://localhost:18800` | WordPress 6.8.3 on PHP 8.3 with MariaDB 11.4: a coffee roaster's site with pages, posts, a menu, a contact form, a shop and a cart, built from a test theme and small test plugins. Each scenario has its own page; `/scenarios/` lists them. |
| Plain PHP | `http://localhost:18810` | "Ledger", a few pages of plain PHP 8.3 on Apache, no framework, for s03 and s34 to s45 |
| Plain PHP, https | `https://localhost:18811` | the same site over https with a self-signed certificate, for s25 |

Every scenario has a **good** state and a **broken** state. `testbed/toggle.sh` switches them by creating or removing a file in `testbed/state/`, which the sites check on every request: no rebuild, no restart. The test page cache keys on the set of broken scenarios, so it never serves a page rendered in the other state. `make down` throws everything away and `make up` starts again from the same seeded site. The testbed never contacts WordPress.org: update checks get an empty answer.

Some scenarios put Soundcheck itself in a hard spot rather than breaking the site: Ctrl+C in the middle of a compare (s38), two compares at once in one folder (s39), a busy machine (s44), a 320-page site (s45). Two test the [wp-update](Soundcheck-WP-Update) loop: ten plugins (`testbed-update-01` to `-10`) are offered an update from 1.0 to 1.1 from inside the testbed. `testbed-update-07` 1.1 is a PHP fatal error on `/contact/` only (s47 and s48).

## The 48 scenarios and the score

The manifest, `testbed/scenarios.yaml`, says what each scenario breaks, where it's implemented, which module should catch it and what counts as caught. The results of the 1.2.0 run:

| # | Scenario | Expected | Soundcheck |
|---|---|---|---|
| s01 | PHP fatal error on one inner page | BROKE health/wp-critical “critical error” (health) | **caught** |
| s02 | PHP warning with file and line in the footer of every page | NEW health/php-error “PHP Warning: Undefined array key” (health) | **caught** |
| s03 | Deprecated notice after a PHP upgrade (plain PHP) | NEW health/php-error “PHP Deprecated: strlen()” (health) | **caught** |
| s04 | White screen (empty 200) on one page | BROKE health/white-screen “white screen” (health) | **caught** |
| s05 | JS error on load from a plugin script | NEW health/js-error “Uncaught TypeError”<br>or BROKE health/js-error “Uncaught TypeError” (health) | **caught** |
| s06 | jQuery deregistered while inline code still uses it | NEW health/js-error “jQuery is not defined” (health) | **caught** |
| s07 | admin-ajax endpoint returns 500 | BROKE health/failed-request “admin-ajax.php returned 500” (health) | **caught** |
| s08 | REST endpoint used by a widget returns 404 | NEW health/failed-request “/wp-json/testbed/v1/reviews returned 404” (health) | **caught** |
| s09 | Deleted page that other pages still link to | BROKE health/status “Page not found (404)”<br>or NEW health/broken-link “spring-offer” (health) | **caught** |
| s10 | Broken image | NEW health/broken-asset “team-1024x683.svg” (health) | **caught** |
| s11 | Contact form stops sending | BROKE interact/ (interact) | **caught** |
| s12 | Menu toggle button becomes dead | BROKE interact/dead-click “does nothing”<br>or NEW interact/dead-click “does nothing” (interact) | **caught** |
| s13 | Button throws a JS error on click | BROKE interact/click-error “throws a JS error”<br>or NEW interact/click-error “throws a JS error” (interact) | **caught** |
| s14 | Layout change: hero disappears, header flips | CHANGED visual/ (visual) | **caught** |
| s15 | Fixed-width element breaks the mobile layout | CHANGED visual/<br>or NEW health/not-responsive (visual) | **caught** |
| s16 | noindex switched on by an SEO setting | NEW health/noindex “noindex” (health) | **caught** |
| s17 | Page title lost | NEW health/no-title “no <title>”<br>or CHANGED health/title “title changed” (health) | **caught** |
| s18 | Page cache caches the cart | BROKE cache/<br>or NEW cache/ (cache) | **caught** |
| s19 | Cache leak: a logged-in page served to visitors | BROKE cache/<br>or NEW cache/ (cache) | **caught** |
| s20 | Heavy plugin added: 2 MB image and a render-blocking script | SLOWER speed/ (speed) | **caught** |
| s21 | Maintenance mode left on after a failed update | BROKE health/status “Server error 503” (health) | **caught** |
| s22 | Database connection error | BROKE health/database (health) | **caught** |
| s23 | Fatal error on every page (memory exhausted) | BROKE health/wp-critical “critical error” (health) | **caught** |
| s24 | Stylesheet 404, the site renders unstyled | NEW health/broken-asset “Broken stylesheet”<br>or CHANGED visual/ (health) | **caught** |
| s25 | Mixed content after a move to https | NEW health/mixed-content (health) | **caught** |
| s26 | robots.txt switched to Disallow / | BROKE seo/robots-txt (seo) | **caught** |
| s27 | Canonical, hreflang, Open Graph and JSON-LD lost | any seo/canonical<br>or any seo/hreflang<br>or any seo/og<br>or any seo/jsonld (seo) | **caught** |
| s28 | Sitemap answers 500 | any seo/sitemap (seo) | **caught** |
| s29 | A page now redirects to the home page | CHANGED health/redirect “ends somewhere else”<br>or any seo/redirect (health) | **caught** |
| s30 | Stale cached page with an expired nonce | NEW cache/stale<br>or NEW cache/expired-nonce<br>or BROKE cache/expired-nonce (cache) | **caught** |
| s31 | Checkout breaks when placing an order | BROKE interact/ (interact) | **caught** |
| s32 | A form gains a reCAPTCHA | warning “captcha” (interact)<br>and no BROKE interact/ | **caught** |
| s33 | Form says sent while no email goes out | BROKE echo/<br>or NEW echo/no-message (echo) | not run |
| s34 | Slower server response only (TTFB) | SLOWER speed/ttfb<br>or SLOWER speed/ (speed) | **caught** |
| s35 | A break inside content that loads late | NEW health/broken-asset “team-photo-2.svg” (health) | **caught** |
| s36 | Firewall bot challenge (Cloudflare style) | BROKE health/blocked “Cloudflare” (health) | **caught** |
| s37 | Staging put behind basic auth (401) | BROKE health/auth-required “Password required (401)” (health) | **caught** |
| s38 | Ctrl+C during compare | marked interrupted (health)<br>and no BROKE, NEW | **caught** |
| s39 | Two compares at once in one folder | nothing at or above NEW (health) | **caught** |
| s40 | Snapshot with its screenshots missing | warning “screenshot from the snapshot is missing or unreadable”<br>or warning “screenshots from the snapshot are missing or unreadable”<br>or CHANGED visual/ (visual) | **caught** |
| s41 | A page that answers just under the timeout | SLOWER speed/ (speed)<br>and no BROKE health/white-screen, NEW health/no-title | **caught** |
| s42 | Flaky server, one load in five answers 500 | UNSTABLE (health) | **caught** |
| s43 | Start page redirects to another host | exit 3 (health) | **caught** |
| s44 | Busy machine (every CPU loaded) | nothing at or above NEW (health) | **caught** |
| s45 | A break on page 317 of a 320-page site | NEW health/php-error “PHP Warning: Undefined array key” (health) | **caught** |
| s46 | Elementor widget missing (YAML rule) | BROKE health/elementor-missing-widget “Elementor widget missing”<br>or NEW health/elementor-missing-widget “Elementor widget missing” (health) | **caught** |
| s47 | One of ten plugin updates breaks the contact page (wp-update --bisect) | culprit testbed-update-07 in at most 4 compares (wp-update) | **caught** |
| s48 | One of ten plugin updates breaks the contact page (wp-update --one-by-one) | culprit testbed-update-07 in at most 10 compares (wp-update) | **caught** |

The per-scenario details (what Soundcheck reported and anything else it noticed) are in [`docs/benchmark.md`](https://github.com/RhythmShahriar/soundcheck/blob/master/docs/benchmark.md) in the Soundcheck repository. A few to read there:

- **s32**, a form that gains a reCAPTCHA, is caught as a warning (`form 1 has a CAPTCHA (reCAPTCHA), which a test browser can't solve, so it wasn't submitted`) and, as it should, with no BROKE.
- **s42**, a server that answers 500 to one load in five, comes out as UNSTABLE, not BROKE: the [confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable) at work.
- **s46** is caught by a [YAML error rule](Soundcheck-Error-Rules), `testbed/rules/elementor-missing-widget.yaml`, not by a built-in check.
- **s47** and **s48** name `testbed-update-07` as the culprit, by bisect and one by one.

> [!NOTE]
> We wrote both the tool and the testbed, so read this as evidence that Soundcheck catches these kinds of breakage, not as a neutral contest. The scenarios, the expectations and the harness are open. Rerun it and add scenarios where you think it falls short.

## The control pass

Catching breakage is half the job. The other half is staying quiet when nothing changed. The control pass sets every scenario good, takes a snapshot of each site (at most 40 pages) and compares twice with nothing changed, ten rounds per site. A change at or above NEW in any compare is a false alarm.

| Site | Rounds | Compares | Pages | False alarms | Errors |
|---|---:|---:|---:|---:|---:|
| wp | 10 | 20 | 17 | 0 | 0 |
| php | 10 | 20 | 17 | 0 | 0 |

## Method

For each scenario, the benchmark (`testbed/bench`):

1. Sets **every** scenario to good.
2. Writes a config that limits the run to the scenario's pages (`pages.include` from the manifest, at most 20 pages), with the modules and interaction levels the scenario needs.
3. Runs `rhythm-soundcheck snapshot <site><path> --name good --json`.
4. Breaks the scenario.
5. Runs `rhythm-soundcheck compare --against-snapshot good --json`.
6. Checks the comparison against the scenario's expectation: a change with that class, whose check ID starts with the expected check and whose title contains the expected text (case-insensitive). Some scenarios expect a warning, an exit code, an interrupted result or the absence of a class instead.
7. Sets the scenario back to good.

The wp-update scenarios run `rhythm-soundcheck wp-update <site>/contact/ --wp '<compose> run --rm --no-deps -T --entrypoint wp cli' --bisect|--one-by-one --rollback` and read the culprit from its output; caught means the right plugin within the expected number of compares.

Scenario s19 needs a login; it uses the testbed's own test account unless `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` are set. Scenario s33 needs Echo; it runs only with `-echo`.

Each result is one of:

| Result | Means |
|---|---|
| **caught** | the expected result was reported |
| found, as X | the problem was reported in another class than expected |
| false alarm | a change the scenario rules out was reported |
| missed | nothing matching was reported |
| error | the tool failed to run; the details say why |
| not run | the tool (or what the scenario needs) wasn't available |

### The other tools

The same good and broken states can be checked with three widely used open-source tools, at pinned versions, where they're installed (or with `-tools all`, fetched through `npx` and Docker):

| Tool | Version | What it checks in the benchmark | Caught when |
|---|---|---|---|
| [BackstopJS](https://github.com/garris/BackstopJS) | 6.3.25 | visual regression: reference screenshots in the good state, a test in the broken one, at desktop (1366) and phone (390) width, 0.1% mismatch threshold | its test fails |
| [lychee](https://github.com/lycheeverse/lychee) | 0.23.0 (Docker image) | every link and image on the scenario's page, in both states | a link fails that didn't before |
| [Unlighthouse](https://unlighthouse.dev/) | 0.19.1 | Lighthouse on the scenario's page in both states, desktop | a category score drops by 0.03 or more (performance: 0.10) |

A tool is only expected to catch what it's built for: a link checker won't see a JS error. The 1.2.0 run didn't include them, so their columns in `docs/benchmark.md` read "not run". On the first 20 scenarios, with Soundcheck 1.0.0, BackstopJS caught 12, lychee 4 and Unlighthouse 9.

## EAA mode

The scenarios s49 to s89 test [EAA mode](Soundcheck-EAA). They run with `--eaa` on the same testbed, each with a good and a broken state. Broken ones must be caught; good ones (nothing broken, or a page built to be hard: a native `<dialog>`, focus shown by box-shadow, a visually hidden skip link, 300 old contrast problems) must stay quiet. Each scenario's harness also checks what must never happen: an order placed or a side effect on a GET or on focus, a request to a host outside the testbed, a canary value from the page in any file Soundcheck wrote, or a statement draft that claims the site meets the law.

> Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.
>
> The testbed is written by the same people as the tool. It shows each detector works on a known case, not how much of a real site's accessibility it covers. Deque reports axe-based tests found 57% of issues by count in its own audits; that is a vendor figure counting issues, not success criteria.

**Caught 29 of 29 broken scenarios, with 0 false alarms in 11 good scenarios and 0 in 60 `--eaa` compares of unchanged sites.** One more scenario, s68, waits for the audit of logged-in pages and isn't counted. The run for 1.3.0 was made on 2026-10-28 with a development build of the release.

| # | Scenario | Expected | Soundcheck |
|---|---|---|---|
| s49 | Keyboard trap in the checkout's coupon field (broken) | BROKE keyboard/trap in #tb_coupon<br>and exit 1 (keyboard)<br>and no any keyboard/trap in #news<br>harness: orders | **caught** |
| s50 | Checkout fields lose their labels (broken) | NEW a11y/label (3×) (a11y)<br>and no BROKE<br>harness: statement | **caught** |
| s51 | Focus lost when the newsletter dialog closes (broken) | NEW keyboard/focus-lost (keyboard)<br>and no any keyboard/trap | **caught** |
| s52 | Modal dialog stops containing focus (broken) | NEW keyboard/dialog-focus (keyboard) | **caught** |
| s53 | Dialog opens without taking focus (plain PHP) (broken) | NEW keyboard/dialog-focus (keyboard) | **caught** |
| s54 | Theme removes every focus outline (broken) | NEW keyboard/no-focus-ring (keyboard) | **caught** |
| s55 | Skip link points nowhere (broken) | NEW keyboard/skip-link-broken (keyboard) | **caught** |
| s56 | Menu toggle becomes a div (broken) | NEW keyboard/not-reachable (keyboard) | **caught** |
| s57 | Custom button ignores Enter and Space (plain PHP) (broken) | NEW keyboard/not-operable (keyboard) | **caught** |
| s58 | Submenu opens on hover only (broken) | NEW keyboard/menu (keyboard) | **caught** |
| s59 | Invalid autocomplete tokens in the checkout (broken) | NEW a11y/autocomplete-valid (a11y) | **caught** |
| s60 | Checkout labels replaced by placeholders (broken) | NEW a11y/placeholder-label (a11y)<br>and no any a11y/label | **caught** |
| s61 | Place order text too light to read (broken) | NEW a11y/color-contrast in #place_order (a11y) | **caught** |
| s62 | Pages lose their lang attribute (broken) | NEW a11y/html-has-lang (a11y) | **caught** |
| s63 | Gallery images lose their alt text (broken) | NEW a11y/image-alt (a11y) | **caught** |
| s64 | Contact form errors shown by colour only (key flow) (broken) | NEW keyboard/error-not-linked (keyboard) | **caught** |
| s65 | Cookie banner traps the keyboard (plain PHP) (broken) | BROKE keyboard/trap (keyboard) | **caught** |
| s66 | Zoom switched off (plain PHP) (broken) | NEW a11y/meta-viewport (a11y) | **caught** |
| s67 | Web component loses its label in a shadow root (plain PHP) (broken) | NEW a11y/label in  >>>  (a11y) | **caught** |
| s68 | Account form drops a label; prefilled with canaries (login) (broken) | NEW a11y/label in acc_phone (a11y)<br>harness: canaries, requests | missed |
| s69 | Promo bar too wide to reflow (plain PHP) (broken) | NEW a11y/reflow (a11y) | **caught** |
| s70 | Place order reachable only by mouse (key flow) (broken) | BROKE keyboard/flow (keyboard)<br>harness: orders | **caught** |
| s71 | Select navigates when it gets focus (plain PHP) (broken) | NEW keyboard/context-change (keyboard) | **caught** |
| s72 | Closed off-canvas menu stays tabbable (plain PHP) (broken) | NEW keyboard/focus-invisible (keyboard) | **caught** |
| s73 | Accessibility overlay script added (broken) | NEW a11y/overlay (a11y)<br>harness: requests | **caught** |
| s74 | Page too big and busy for the audits to finish (plain PHP) (broken) | exit 3<br>and warning “not checked” (a11y)<br>and no FIXED | **caught** |
| s75 | Dialog ignores Escape (plain PHP) (broken) | NEW keyboard/escape (keyboard)<br>and no any keyboard/trap | **caught** |
| s76 | Focus handler reloads the page (plain PHP) (broken) | any keyboard/trap<br>or warning “keyboard: /a11y/reload-trap.php:” (keyboard) | **caught** |
| s77 | Native dialog with showModal() (hard good, plain PHP) (good) | nothing at or above NEW<br>and at least 1 widget tested (keyboard)<br>and no any keyboard/trap, any keyboard/focus-lost, any keyboard/dialog-focus | **quiet** |
| s78 | Custom modal done right (hard good, plain PHP) (good) | nothing at or above NEW (keyboard)<br>and no any keyboard/trap, any keyboard/focus-lost, any keyboard/dialog-focus | **quiet** |
| s79 | Focus shown without an outline (hard good, plain PHP) (good) | nothing at or above NEW (keyboard)<br>and no any keyboard/no-focus-ring | **quiet** |
| s80 | Visually hidden skip link that shows on focus (hard good) (good) | nothing at or above NEW (keyboard)<br>and no any keyboard/skip-link, any keyboard/focus-invisible, any keyboard/no-focus-ring | **quiet** |
| s81 | Page that changes on every load (hard good, plain PHP) (good) | nothing at or above NEW (keyboard)<br>and no NEW | **quiet** |
| s82 | 300 old contrast problems, nothing changes (hard good, plain PHP) (good) | nothing at or above NEW (a11y)<br>and no NEW, FIXED | **quiet** |
| s83 | One of 300 old contrast problems fixed (plain PHP) (broken) | FIXED a11y/color-contrast in row-5 (a11y)<br>and no NEW | **caught** |
| s84 | Frames from the same and another origin (hard good, plain PHP) (good) | nothing at or above NEW (a11y)<br>and no NEW<br>harness: statement, canaries, requests | **quiet** |
| s85 | Checkout on a busy machine (hard good) (good) | nothing at or above NEW (keyboard)<br>and no NEW, any keyboard/trap<br>harness: stress, orders | **quiet** |
| s86 | Checkout answers 500 under --eaa (broken) | BROKE health/ (health)<br>harness: statement | **caught** |
| s87 | Snapshot from Soundcheck 1.2 compared with --eaa (plain PHP) (good) | warning “used the WCAG 2.0 rules”<br>and nothing at or above NEW (a11y)<br>and no NEW a11y/autocomplete-valid | **quiet** |
| s88 | Page with side effects on GET and on focus (hard good, plain PHP) (good) | nothing at or above NEW (keyboard)<br>and no NEW<br>harness: orders, requests | **quiet** |
| s89 | Small site with no known accessibility problems (plain PHP) (good) | nothing at or above NEW<br>and at least 1 widget tested (keyboard)<br>and no any a11y/, any keyboard/<br>harness: statement, requests | **quiet** |

**caught**: a broken scenario's problem was reported · **quiet**: a good scenario got no report it rules out · missed: s68 needs the audit of logged-in pages, which 1.3.0 doesn't do.

### False alarms and determinism

Every scenario good, each round an `--eaa` snapshot and two compares with nothing changed. On a scenario's own pages an accessibility warning or error in the snapshot itself counts too.

| Site | Rounds | Compares | Pages | False alarms | Errors |
|---|---:|---:|---:|---:|---:|
| wp | 10 | 20 | 17 | 0 | 0 |
| php | 10 | 20 | 17 | 0 | 0 |
| s89 | 10 | 20 | 1 | 0 | 0 |

The issue keys of 10 snapshots of the same unchanged pages differed in **0** keys (34 keys on WordPress, none on the clean plain-PHP pages).

### Cost

16 cores, linux/amd64, Chrome 153.0.8010.52, axe-core 4.14.0 (WCAG 2.1 tags), seconds per page, median / p95:

| Measure | wp | php | Target |
|---|---:|---:|---|
| Page visit, default run | 2.78 / 2.92 | 2.69 / 4.26 | |
| Page visit, `--eaa` run | 1.43 / 1.61 | 1.32 / 2.86 | |
| a11y audit per page | 0.00 / 0.05 | 0.00 / 0.24 | median at most 2 s |
| Keyboard walk per page | 5.79 / 8.12 **over** | 1.02 / 3.44 | median at most 3 s |
| Whole `--eaa` run ÷ default run | 0.8× (26 s ÷ 32 s) | 0.6× (21 s ÷ 35 s) | at most 3× |
| One key flow (s70, `/checkout/`) | 2.2 s | | at most 15 s |

The keyboard walk misses its target on WordPress pages, which have more stops and widgets. An `--eaa` run is still faster than a default run, since it leaves out the visual, interact, cache and speed modules.

### Coverage

Which WCAG A and AA success criteria the tests can find failures of. A criterion with a test still needs a person to judge it: the tests find some failures, never a pass.

| WCAG | Criteria | Automated | Keyboard | Needs a person only |
|---|---:|---:|---:|---:|
| 2.1 | 50 | 22 | 7 | 23 |
| 2.2 | 55 | 23 | 8 | 26 |

Every criterion with the axe-core rules and Soundcheck checks that test it is in the [benchmark file](https://github.com/RhythmShahriar/soundcheck/blob/master/docs/benchmark.md#eaa-mode) and in each statement draft's appendix.

### Other tools

The benchmark also runs Unlighthouse, the axe-core CLI and pa11y on the same states when they are installed. They weren't installed for the 1.3.0 run, so this run has no figures for them.

### Run the EAA scenarios

```sh
make build
make -C testbed up
make -C testbed bench-eaa      # go run -tags testbed ./testbed/bench -only eaa (10 control rounds)
go run -tags testbed ./testbed/bench -only eaa -tools axe,pa11y,unlighthouse   # also run the other tools
```

An `-only eaa` run rewrites only the EAA section of `docs/benchmark.md`.

## Run it yourself

You need Docker with Compose v2, Go and a clone of the Soundcheck repository:

```sh
make build                 # bin/rhythm-soundcheck
make -C testbed up         # WordPress on :18800, plain PHP on :18810 (about 30 s)
go run -tags testbed ./testbed/bench                      # Soundcheck + installed tools, then the control pass
go run -tags testbed ./testbed/bench -tools all           # also fetch BackstopJS, lychee and Unlighthouse
go run -tags testbed ./testbed/bench -only s05,s09        # some scenarios (no control pass unless -control is given)
go run -tags testbed ./testbed/bench -control 0           # skip the control pass
make -C testbed down
```

`make bench` builds the binary and runs the harness in one step.

| Flag | Default | |
|---|---|---|
| `-bin` | `bin/rhythm-soundcheck` | the binary to benchmark |
| `-only` | all | e.g. `s01,s05` |
| `-tools` | `auto` | `auto` (the ones installed), `none`, `all`, or a list: `backstop,lychee,unlighthouse` |
| `-control` | `10` | control pass rounds; `0` skips it (default `0` with `-only`) |
| `-echo` | none | Echo's URL (e.g. `http://127.0.0.1:8025`) for s33 |
| `-compose` | `docker compose -f testbed/docker-compose.yml` | the Compose command for the testbed's WP-CLI (s47, s48) |
| `-out` | `docs/benchmark.md` | the scoreboard |
| `-work` | `testbed/bench/work` | runs and `results.json` |
| `-wp`, `-php`, `-php-https` | from `scenarios.yaml` | site URLs, if you moved the ports with `TESTBED_WP_PORT`, `TESTBED_PHP_PORT` or `TESTBED_PHP_HTTPS_PORT` |
| `-timeout` | `5m` | per command |

Runs are kept in `testbed/bench/work/<id>/` (snapshot, `compare.json`, the tools' output) and the control pass in `work/control/`; `work/results.json` has every result. Switch a scenario by hand to look at it in a browser:

```sh
testbed/toggle.sh s05 broken     # or: make -C testbed broken ID=s05
testbed/toggle.sh status
testbed/toggle.sh all good
make -C testbed wp-update-reset  # the ten testbed-update plugins back at 1.0 (s47, s48)
make -C testbed wp-updates       # the plugin updates WordPress offers
```

To try [wp-update](Soundcheck-WP-Update) on the testbed yourself, the WP-CLI command is `docker compose -f testbed/docker-compose.yml run --rm --no-deps -T --entrypoint wp cli` (the WordPress image has no WP-CLI of its own).

The plain-PHP site also runs without Docker, under PHP's built-in server: `TESTBED_STATE=/tmp/s php -S 127.0.0.1:18810 -t testbed/php` (add `-d display_errors=1` if your `php.ini` hides errors), with `TESTBED_STATE=/tmp/s testbed/toggle.sh s03 broken` to switch a scenario. The testbed's [README](https://github.com/RhythmShahriar/soundcheck/blob/master/testbed/README.md) has its accounts, every Make target and how to add a scenario.

The testbed only listens on `127.0.0.1`. It's also the end-to-end test suite for Soundcheck itself; see [Contributing](Contributing#soundcheck-and-the-testbed).

---
<sub>← [FAQ and troubleshooting](Soundcheck-FAQ) · [Home](Home) · [Echo overview](Echo) →</sub>
