For agencies that look after many sites: `fleet` snapshots or compares every site in a list, a few at a time. It ranks them worst first on one page. `report --client` turns a result into a report with your agency's name, logo and color that you can send to a client.

## Fleet

List the sites in a `sites.yaml`:

```yaml
sites:
  - name: brew
    url: http://127.0.0.1:18931
    args: ["--viewport", "desktop"]
  - name: ledger
    url: http://127.0.0.1:18932
    args: ["--viewport", "desktop"]
```

A site can also name its own settings file and variable prefix:

```yaml
  - name: acme-shop
    url: https://staging.acme.test
    config: acme.yaml            # relative to sites.yaml, passed as --config
    env_prefix: ACME             # default from the name: ACME_SHOP
    args: ["--limit", "50"]
```

| Field | Means |
|---|---|
| `name` | the site's folder name: letters, digits, dots, dashes and underscores, up to 64. Names are compared without case. |
| `url` | the site; a bare host works |
| `config` | a settings file for this site, passed as `--config` |
| `env_prefix` | where this site's variables come from (see below). Default from the name: `acme-shop` reads `ACME_SHOP_…`. |
| `args` | extra flags for this site |
| `eaa` | `true` adds `--eaa` to this site's command, unless its args or the fleet's flags already have it. See [EAA mode](#eaa-mode). |

Before the first deploy, snapshot them all; after it, compare them all:

```sh
rhythm soundcheck fleet sites.yaml snapshot
rhythm soundcheck fleet sites.yaml compare
```

```text
Fleet: taking a snapshot of 2 sites, 2 at a time
  [1/2] ledger: Warnings · 1 warning, 2 notices · 5s
  [2/2] brew: Warnings · 1 warning, 2 notices · 5s
Worst: brew (Warnings)
Fleet report: /home/you/agency/.soundcheck-fleet/fleet.html
```

```text
Fleet: comparing 2 sites, 2 at a time
  [1/2] brew: Clean · 4s
  [2/2] ledger: Something broke · 1 broke, 3 new, 1 changed · 9s
Worst: ledger (Something broke)
Fleet report: /home/you/agency/.soundcheck-fleet/fleet.html
```

Each site runs in its own folder, `.soundcheck-fleet/<name>/` next to `sites.yaml`, with its own `.soundcheck/` store, its JSON result (`result.json`) and its log (`soundcheck.log`). `fleet.html` ranks the sites worst first (Couldn't check, Something broke, Interrupted, Has errors, Changed, Warnings, Clean) with each site's counts, pages, time and an LCP trend from its [history](Soundcheck-History). Each site links to its report. `fleet.json` holds the same for scripts.

| Flag | Default | Description |
|---|---|---|
| `--tabs <n>` | `3` | Sites checked at once, 1 to 16. |
| `--open` | off | Open `fleet.html` when done. |
| `--plain`, `--no-color` | off | As for every command. |

The exit code is the worst site's. A sites file that doesn't load exits 2 before any site runs.

### EAA mode

`eaa: true` on a site, or `fleet sites.yaml compare --eaa` for every site, runs [EAA mode](Soundcheck-EAA). `--eaa` and `--wcag <version>` on the fleet go to every site's command. Two sites of the testbed's Ledger, the first with a keyboard trap after the snapshot:

```yaml
sites:
  - name: ledger
    url: http://127.0.0.1:39200/a11y/
    eaa: true
    args: ["--include", "/a11y/*", "--page", "/a11y/consent.php"]
  - name: ledger-pricing
    url: http://127.0.0.1:39200/a11y/wishlist.php
    eaa: true
    args: ["--include", "/a11y/wishlist.php"]
```

```text
Fleet: taking a snapshot of 2 sites, 2 at a time
  [1/2] ledger-pricing: No failures found by these tests · 6s
  [2/2] ledger: No failures found by these tests · 10s
Worst: ledger (No failures found by these tests)
Fleet report: .../.soundcheck-fleet/fleet.html
```

```text
Fleet: comparing 2 sites, 2 at a time
  [1/2] ledger-pricing: Nothing changed; no failures found by these tests · 5s
  [2/2] ledger: Something broke · 1 broke · 13s
Worst: ledger (Something broke)
Fleet report: .../.soundcheck-fleet/fleet.html
```

A clean `--eaa` site never reads `Clean`: a snapshot says `No failures found by these tests` or `<n> WCAG criteria have failures`; a comparison says `Nothing changed; no failures found by these tests` (just `Nothing changed` while accessibility problems are still open). `fleet.html` adds the columns `Open accessibility issues` (with the trend against the site's previous fleet run, such as `↑ up from 9 at the last fleet run`) and `Criteria with failures`; each site links its `Statement draft`. A site without `--eaa` has empty cells there, not 0. `fleet.json` gains, per site:

```json
"eaa_open": 1,
"criteria_failed": 1,
"eaa_trend": "up",
"eaa_before": 0,
"statement": "ledger/.soundcheck/runs/2026-10-28T19-34-24.517Z/eaa/statement-draft.md",
"label": "Something broke"
```

`label` is the wording above; `verdict` stays `clean` for a clean site. Each site's run writes its own statement draft in its folder.

### Variables and logins per site

A site never sees another site's secrets. Each site gets a short allowlist of the fleet's environment (`PATH`, `HOME`, locale, proxy and CI variables), plus its own prefixed variables with the prefix taken off: `ACME_SOUNDCHECK_USER` becomes `SOUNDCHECK_USER` for the site `acme` only. The fleet's own unprefixed `SOUNDCHECK_*` variables go to no site.

```sh
export BREW_SOUNDCHECK_USER=sc-test BREW_SOUNDCHECK_PASSWORD=…
export LEDGER_RHYTHM_ECHO_TOKEN=…
rhythm soundcheck fleet sites.yaml compare
```

### What args can't do

`sites.yaml` may come from someone else, so a site's `args` can't make Soundcheck run programs or write elsewhere:

- `--json`, `--md`, `--junit`, `--html`, `-o`, `--config`, `--open` and `--name` are set by the fleet: `args can't set --json; the fleet sets it for every site`.
- `--allow-exec` and `--lighthouse` need a variable instead: `sites.yaml: site "acme": args can't set --allow-exec; to let this site's exec commands run, set ACME_SOUNDCHECK_ALLOW_EXEC=1` (and `ACME_SOUNDCHECK_LIGHTHOUSE=1`).
- `--browser` takes an engine name only (`chrome`, `firefox`, `webkit`), never a path: `args can't set --browser ./evil.sh; a site's args name an engine (chrome, firefox or webkit), since the file can't let a site run programs`.

Ctrl+C stops every site's run; each closes its browser and drops a half-made snapshot.

## Client reports

`report --client` writes a standalone HTML page for a client: what was checked, what changed, before and after pictures, in plain words, with your brand. Describe the brand in an `agency.yaml`:

```yaml
name: Acme Web & Co
logo: logo.svg          # PNG, JPEG or SVG, in this file's folder, up to 1 MB
accent: "#0F766E"       # hex color; default #E8603A
contact: hello@acme.test
```

Then render any stored result:

```sh
rhythm soundcheck report --client --brand agency.yaml -o client-report.html
```

```text
Client report written to client-report.html
```

Without `-o` the HTML goes to stdout. For a fleet site, run it in that site's folder (`.soundcheck-fleet/<name>/`). The page for the comparison above reads, top to bottom:

- the title `127.0.0.1:18932 · website check · 25 October 2026`, your logo and name, then `Website check for <site>`;
- a headline, here `Something stopped working` (or `We found new problems`, `Everything works; some pages look or load differently`, `Everything works as before`), with `We checked 4 pages on 25 October 2026 and compared them with how they worked on 25 October 2026.`;
- counts for `Stopped working`, `New problem`, `Looks different`, `Slower` and `Fixed`;
- **What we checked**, in plain words: `4 pages of the site, on desktop screens`, `Every page loads, without errors or missing parts`, `How every page looks, compared picture by picture`, `Buttons and menus respond when clicked` and so on, one line per module and level that ran;
- **What we found**: up to eight changes such as `Stopped working · Page not found (404) · On /about.php`, with Before and After pictures for visual changes;
- **Fixed since last time**, then the footer `Prepared by Acme Web & Co · hello@acme.test`.

It never shows file paths, stack traces, selectors, check ids or fix hints: those are for you, in the normal report.

For a result made with [`--eaa`](Soundcheck-EAA), **What we checked** says `Accessibility evidence: WCAG 2.1 AA automated rules and a keyboard walkthrough of 21 pages` instead of `Accessibility basics`. An **Accessibility** section follows, in plain words. From the EAA walk-through, after the trap was fixed and the wishlist control ignored:

```text
Accessibility
We tested the site against the WCAG 2.1 A and AA criteria with automated rules and by using it with the keyboard alone.
0  Open accessibility problems
1  Criteria with failures
26 Criteria with none found by these tests
23 Criteria a person needs to check
Criteria with failures
2.1.1 Keyboard (1 problem on 1 page)
What still needs a person (23 criteria)
These tests can't judge these criteria; someone needs to check them by hand.
...
Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.
```

(the page's text, without its layout). Problems set aside by ignore rules aren't open, though their criteria still count. Pages the tests couldn't finish are named (`We couldn't finish the accessibility tests on /catalog/, so that page needs checking again.`) and the report is titled `This check is partial`; `Everything works` is never shown then. UNSTABLE changes are left out. A run (not a comparison) gets `We found errors on the site`, `The site works, with a few warnings` or `Everything works`.

Errors, exit 2:

```text
rhythm soundcheck: --client needs --brand <agency.yaml>: the agency's name, logo and color
rhythm soundcheck: --client writes HTML; name the file .html, not client.md
```

The brand file is checked too: a logo outside its folder, over 1 MB or not an image is refused. An SVG logo with scripts, event handlers, external links or entities is refused, never rewritten (`SVG logos can't have <script> elements`).

---
<sub>← [WordPress updates](Soundcheck-WP-Update) · [Home](Home) · [History](Soundcheck-History) →</sub>
