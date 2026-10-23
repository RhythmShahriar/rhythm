How well does Soundcheck catch what breaks real WordPress and PHP sites, and how does it compare with other tools? This page describes the benchmark: a testbed of 20 deliberately broken sites, how each scenario is run and judged, and the tools it's compared with. The results are published with each release.

## The testbed

The testbed lives in the Soundcheck repository, in `testbed/`. It runs two sites in Docker:

| Site | Address | Stack |
|---|---|---|
| WordPress | `http://localhost:18800` | WordPress 6.8 on PHP 8.3 and Apache, MariaDB, a test theme and five small test plugins |
| Plain PHP | `http://localhost:18810` | PHP 8.3 on Apache, no framework |

The WordPress site hosts each scenario on its own pages, so scenarios don't affect each other. Every scenario has a **good** state and a **broken** state. `testbed/toggle.sh` switches them by creating or removing a file in `testbed/state/`, which the sites check on every request: no rebuild, no restart. The test page cache keys on the set of broken scenarios, so it never serves a page rendered in the other state. `make down` throws everything away and `make up` starts again from the same seeded site.

## The 20 scenarios

Each scenario breaks one thing the way a real update or deploy does. The manifest, `testbed/scenarios.yaml`, says what each breaks, where it's implemented, which Soundcheck module should catch it, and what counts as caught.

| # | Scenario | Site | Module | Caught when the comparison has |
|---|---|---|---|---|
| s01 | PHP fatal error on one inner page | WordPress | health | BROKE `health/wp-critical` "critical error" |
| s02 | PHP warning with file and line in the footer of every page | WordPress | health | NEW `health/php-error` "PHP Warning: Undefined array key" |
| s03 | Deprecated notice after a PHP upgrade | plain PHP | health | NEW `health/php-error` "PHP Deprecated: strlen()" |
| s04 | White screen (empty 200) on one page | WordPress | health | BROKE `health/white-screen` |
| s05 | JS error on load from a plugin script | WordPress | health | NEW or BROKE `health/js-error` "Uncaught TypeError" |
| s06 | jQuery deregistered while inline code still uses it | WordPress | health | NEW `health/js-error` "jQuery is not defined" |
| s07 | `admin-ajax` endpoint returns 500 | WordPress | health | BROKE `health/failed-request` "admin-ajax.php returned 500" |
| s08 | REST endpoint used by a widget returns 404 | WordPress | health | NEW `health/failed-request` ".../reviews returned 404" |
| s09 | Deleted page that other pages still link to | WordPress | health | BROKE `health/status` "Page not found (404)", or NEW `health/broken-link` |
| s10 | Broken image | WordPress | health | NEW `health/broken-asset` |
| s11 | Contact form stops sending | WordPress | interact (`submit`) | BROKE `interact/…` |
| s12 | Menu toggle button becomes dead | WordPress | interact | BROKE or NEW `interact/…` "dead" |
| s13 | Button throws a JS error on click | WordPress | interact | BROKE or NEW `interact/…` "WishlistStore" |
| s14 | Layout change: hero disappears, header flips | WordPress | visual | CHANGED `visual/…` |
| s15 | Fixed-width element breaks the mobile layout | WordPress | visual | CHANGED `visual/…`, or NEW `health/not-responsive` |
| s16 | `noindex` switched on by "Discourage search engines" | WordPress | health | NEW `health/noindex` |
| s17 | Page title lost | WordPress | health | CHANGED `health/title` |
| s18 | Page cache caches the cart | WordPress | cache | BROKE or NEW `cache/…` |
| s19 | Cache leak: a logged-in page served to visitors | WordPress | cache (login) | BROKE or NEW `cache/…` |
| s20 | Heavy plugin added: 2 MB image and a render-blocking script | WordPress | speed | SLOWER `speed/…` |

## Method

For each scenario, the benchmark (`testbed/bench`):

1. Sets **every** scenario to good.
2. Writes a config that limits the run to the scenario's pages (`pages.include` from the manifest, at most 20 pages), with the interaction levels the scenario needs.
3. Runs `rhythm-soundcheck snapshot <site><path> --name good --json`.
4. Breaks the scenario.
5. Runs `rhythm-soundcheck compare --against-snapshot good --json`.
6. Checks the comparison against the scenario's expectation: a change with that class, whose check ID starts with the expected check and whose title contains the expected text (case-insensitive).
7. Sets the scenario back to good.

Scenario s19 needs a login; it uses the testbed's own test account unless `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD` are set.

Each result is one of:

| Result | Means |
|---|---|
| **caught** | the expected change was reported |
| found, as X | the problem was reported, but in another class than expected |
| missed | nothing matching was reported |
| error | the tool failed to run; the details say why |
| not run | the tool wasn't available |

### The other tools

The same good and broken states are checked with three widely used open-source tools, at pinned versions, where they're installed (or with `-tools all`, fetched through `npx` and Docker):

| Tool | Version | What it checks in the benchmark | Caught when |
|---|---|---|---|
| [BackstopJS](https://github.com/garris/BackstopJS) | 6.3.25 | visual regression: reference screenshots in the good state, a test in the broken one, at desktop (1366) and phone (390) width, 0.1% mismatch threshold | its test fails |
| [lychee](https://github.com/lycheeverse/lychee) | 0.23.0 (Docker image) | every link and image on the scenario's page, in both states | a link fails that didn't before |
| [Unlighthouse](https://unlighthouse.dev/) | 0.19.1 | Lighthouse on the scenario's page in both states, desktop | a category score drops by 0.03 or more (performance: 0.10) |

A tool is only expected to catch what it's built for: a link checker won't see a JS error. The scoreboard shows where each one helps, not a single winner.

## Results

> [!NOTE]
> **Results are published with Soundcheck v1.0.0.** This section is filled in from the benchmark run for that release, with the date, the Soundcheck version and the platform it ran on.

| # | Scenario | Soundcheck | BackstopJS | lychee | Unlighthouse |
|---|---|---|---|---|---|
| s01–s20 | *to be published with v1.0.0* | | | | |

## Run it yourself

You need Docker with Compose v2, Go, and a clone of the Soundcheck repository:

```sh
make build                 # bin/rhythm-soundcheck
make -C testbed up         # WordPress on :18800, plain PHP on :18810
go run -tags testbed ./testbed/bench                      # Soundcheck + the tools you have installed
go run -tags testbed ./testbed/bench -tools all           # also fetch BackstopJS, lychee and Unlighthouse
go run -tags testbed ./testbed/bench -only s05,s09        # some scenarios
make -C testbed down
```

The benchmark writes the scoreboard to `docs/benchmark.md`, and keeps each scenario's runs (snapshot, `compare.json`, the tools' output) in `testbed/bench/work/<id>/`, with every result in `work/results.json`. Switch a scenario by hand to look at it in a browser:

```sh
cd testbed
./toggle.sh s05 broken     # or: make broken ID=s05
./toggle.sh status
./toggle.sh all good
```

The testbed only listens on `127.0.0.1`. It's also the end-to-end test suite for Soundcheck itself; see [Contributing](Contributing#soundcheck-and-the-testbed).

---
<sub>← [FAQ and troubleshooting](Soundcheck-FAQ) · [Home](Home) · [Responsible use](Responsible-Use) →</sub>
