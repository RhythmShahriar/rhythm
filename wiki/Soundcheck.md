Soundcheck answers one question: **did my change break anything?** You snapshot a whole site, update a plugin, switch PHP or deploy, then compare. Soundcheck shows only what's different, most serious first. This page explains what it checks, when to use it, and how a run works.

```sh
rhythm soundcheck snapshot staging.yoursite.com   # before
# …update plugins, switch PHP, deploy…
rhythm soundcheck compare                         # after
```

Soundcheck ships as its own binary, `rhythm-soundcheck`, and you normally run it through `rhythm soundcheck` (or the short alias `rhythm sc`). The installer sets it up along with Tuner; if you see `rhythm: soundcheck isn't installed`, run `rhythm install soundcheck`. See [Install](Install).

> [!IMPORTANT]
> Check only sites you own or have permission to test. Soundcheck browses like a visitor at a polite pace. It only submits forms when you turn on the `submit` level, and refuses to on sites you list as production. See [Responsible use](Responsible-Use).

## What it checks

Soundcheck opens every page in headless Chrome, at desktop and phone size, and runs five modules on it.

| Module | What it looks for | Page |
|---|---|---|
| 🩺 **health** | Server errors and 404s, PHP fatals, warnings, notices and deprecations with file and line, the WordPress "critical error" screen, white screens, framework error pages, JavaScript errors, failed XHR, REST and `admin-ajax` calls, broken links and assets, mixed content, lost titles, a new `noindex`. | [Page health](Soundcheck-Page-Health) |
| 🖼️ **visual** | Full-page screenshots, compared pixel by pixel. Regions that change on their own (sliders, clocks, ads) are learned and ignored. A change shared by many pages is reported once. | [Visual changes](Soundcheck-Visual) |
| 🖱️ **interact** | Forms and controls read from the page, then menus, tabs, accordions and modals clicked to find **dead clicks** and clicks that throw. Opt in to submit forms with test data and to run your own YAML flows. | [Interactions](Soundcheck-Interactions), [Forms and flows](Soundcheck-Forms-And-Flows) |
| 🗄️ **cache** | Which cache layer serves pages and how often, private pages (cart, checkout, account) served from cache, logged-in content leaking to visitors, cached pages carrying expired WordPress nonces, stale copies. | [Cache](Soundcheck-Cache) |
| ⚡ **speed** | Lab LCP, CLS, TBT, TTFB, weight and requests per page; slowdowns past a noise floor; site-wide suggestions grouped by cause, including what each plugin costs. | [Speed](Soundcheck-Speed) |

Soundcheck is built for PHP and WordPress (it knows plugin paths, nonces, form plugins and cache plugins) and works on any site.

## When to use it

- **Before and after a change.** Plugin and theme updates, a WordPress or PHP upgrade, a new cache rule, a deploy. Take a snapshot, make the change, compare.
- **Staging against production.** `rhythm soundcheck staging.yoursite.com --against yoursite.com` checks both now and compares them, with no snapshot.
- **In CI.** Fail a pull request or a deploy when something broke. See [CI](Soundcheck-CI).
- **A one-off health check.** `rhythm soundcheck run yoursite.com` lists the problems a site has right now.

Soundcheck isn't a security scanner; that's [Tuner](Tuner). The two complement each other: Tuner looks for what an attacker could use, Soundcheck for what your last change did to visitors.

## How a run works

```mermaid
flowchart LR
  A["Discover<br/>sitemaps, links"] --> B["Visit<br/>headless Chrome,<br/>desktop + mobile"]
  B --> C["Check<br/>each module"]
  C --> D["Snapshot<br/>.soundcheck/"]
  D -.-> E["Compare<br/>same pages again"]
  E --> F["Report<br/>terminal, HTML,<br/>Markdown, JSON, JUnit"]
```

1. **Discover.** Soundcheck fetches the start page (following redirects, so http becomes https and apex becomes www when the site says so), reads `robots.txt` and the sitemaps, and follows the site's own links. Pages that share a URL pattern, such as `/product/{slug}/`, are grouped, and a few samples stand for the rest. Admin, login, feeds and URLs with side effects (log out, add to cart) are never visited. See [Discovery](Soundcheck-Discovery).
2. **Visit.** Each page opens in its own tab of a private browser session (like a private window, never logged in), four at a time and at most 5 page loads a second. Soundcheck records the response, redirects, console errors, failed requests, every resource, lab timings and the visible text, then takes a full-page screenshot. The page then reloads at phone size (390 px wide) for a second screenshot.
3. **Check.** While the page is open, the interact module reads its forms and clicks its controls. Then every module checks the recorded page. After the last page, site-wide checks run: links to pages that weren't visited, the cache double fetch and leak test, speed suggestions and your flows.
4. **Snapshot.** `snapshot` saves the run (`run.json` and the screenshots) in `.soundcheck/snapshots/<name>/`. A snapshot takes each screenshot twice, a moment apart, to learn the regions that change on their own.
5. **Compare.** `compare` checks the snapshot's pages again, with no new discovery, and diffs the two runs. New errors become **BROKE**, other new problems **NEW**, visual and content differences **CHANGED**, slowdowns **SLOWER**, and problems that went away **FIXED**.
6. **Report.** The terminal shows the result live. Every run also writes a standalone `report.html` with before/after sliders, and you can save Markdown, JSON or JUnit. See [Reading results](Soundcheck-Reading-Results).

A run on a 37-page test site took about three minutes for the snapshot and under three for the compare, with every module on. Large sites stay manageable because grouping caps how many pages of each pattern are checked; see [FAQ](Soundcheck-FAQ#slow-or-huge-sites).

## The browser

Soundcheck drives Chrome or Chromium through the DevTools protocol. It uses an installed Chrome when it finds one, and otherwise downloads a pinned Chromium once, on first use, into your cache folder. `rhythm soundcheck update` installs or refreshes it, and `--browser` points at a specific binary. See [FAQ](Soundcheck-FAQ#chromium-download-and-update).

## Files

Everything Soundcheck records stays in your project, in `./.soundcheck/`: snapshots, recent runs and comparisons, screenshots and reports. Inside a git repository, the snapshots and runs are added to `.git/info/exclude` so screenshots are never committed by accident, while `config.yaml` and `flows/` stay visible for your team. See [Configuration](Soundcheck-Configuration#the-soundcheck-folder).

## Where to go next

| You want to… | Read |
|---|---|
| Take a snapshot and compare, step by step | [Your first check](Soundcheck-First-Check) |
| Understand BROKE, NEW, CHANGED, SLOWER and FIXED | [Reading results](Soundcheck-Reading-Results) |
| Control which pages are checked | [Discovery](Soundcheck-Discovery) |
| Test forms and scripted journeys | [Forms and flows](Soundcheck-Forms-And-Flows) |
| Check caching and logged-in leaks | [Cache](Soundcheck-Cache) |
| Wire it into CI | [CI](Soundcheck-CI) |
| Change defaults for a project | [Configuration](Soundcheck-Configuration) |
| Look up a flag | [CLI reference](Soundcheck-CLI-Reference) |
| Fix a problem | [FAQ and troubleshooting](Soundcheck-FAQ) |

---
<sub>← [FAQ and troubleshooting](Tuner-FAQ) · [Home](Home) · [Your first check](Soundcheck-First-Check) →</sub>
