The European Accessibility Act applies to shops, banking, travel, e-books and many other services sold in the EU since June 2025. It asks that they can be used without a mouse, without seeing the screen and without hearing the sound. They must also publish an accessibility statement. **EAA mode** gathers the evidence for that review: it runs axe-core's WCAG rules on every page, walks every page with the keyboard the way a keyboard user would, replays your key journeys (checkout, contact, account) with keys only and drafts an accessibility statement with every criterion and what the tests found. Then, like the rest of Soundcheck, it shows **what your last change broke**.

```sh
rhythm soundcheck snapshot staging.yoursite.com --eaa   # once: what the site does today
# …update a plugin, a theme or the checkout…
rhythm soundcheck compare --fail-on new                 # after a change: new accessibility problems only
rhythm soundcheck statement -o accessibility-statement.md
```

A compare takes `--eaa` from its snapshot, so `compare` needs no `--eaa` of its own. Without the rhythm CLI, type `rhythm-soundcheck` in place of `rhythm soundcheck`: it is the same tool.

> [!IMPORTANT]
> EAA mode gives you **evidence, not a compliance guarantee**. Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act. It never decides a site is compliant and never picks the statement's status for you. See [Honest limits](#honest-limits).

This page covers what EAA mode checks, a first run from start to finish with the real output, what each part of the result means, the exit codes, the JSON and the other formats, its limits and the benchmark. Two pages go deeper: [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough) (the walk, its checks and key flows) and [Accessibility statement](Soundcheck-Accessibility-Statement) (the draft, its config block, EARL and the signed evidence pack).

## What it checks

`--eaa` works on `snapshot`, `compare`, `run`, `<url> --against <url>` and `fleet`. It turns on three modules in Chrome:

| Module | What it does | Page |
|---|---|---|
| 🩺 **health** | Errors, broken pages and links, as in every run. A checkout that answers 500 is BROKE before it is an accessibility question. | [Page health](Soundcheck-Page-Health) |
| ♿ **a11y** | axe-core 4.14.0 with the WCAG 2.1 A and AA rules (`--wcag 2.2` for 2.2) on every page, plus three checks of its own: fields labeled only by a placeholder, pages that scroll sideways at 320 px wide (reflow) and known overlay widgets. | [Accessibility and SEO](Soundcheck-Accessibility-And-SEO#accessibility) |
| ⌨️ **keyboard** | Tab and Shift+Tab through every page, then menus, dialogs and disclosures opened and closed by keyboard. It finds keyboard traps, focus lost after a dialog closes, focus you can't see, skip links that go nowhere and controls only a mouse can use. Key flows (`keyboard: true`) replay a journey with keys only. | [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough) |

Every finding is a normal Soundcheck issue keyed by rule and element, with its WCAG success criterion and its EN 301 549 clause. BROKE, NEW, FIXED, UNSTABLE, `accept`, `ignore:` and `--fail-on` work on it as on any other change. A keyboard trap is an error, so a new one is BROKE and `--fail-on broke` (the GitHub Action's default when comparing) already stops it; `--fail-on new` also stops new warnings such as a missing label.

Every `--eaa` run and snapshot also writes two files into its folder:

- `eaa/statement-draft.md`: an accessibility statement in the layout of the EU model statement, with a table of every WCAG 2.1 A and AA success criterion and a `[TO DO]` wherever only you can fill in. See [Accessibility statement](Soundcheck-Accessibility-Statement).
- `eaa/evidence.json`: the SHA-256 of every file the run recorded, the tool, Chrome and axe-core versions and a hash chain to the previous `--eaa` run of the same site. `evidence sign` signs it. See [The evidence pack](Soundcheck-Accessibility-Statement#the-evidence-pack).

### Modules and other flags

| You chose | Modules with `--eaa` |
|---|---|
| nothing | `health, a11y, keyboard` |
| `--modules x` | `x` plus a11y and keyboard |
| `--modules +x` | `health, a11y, keyboard` plus `x` |
| `modules:` in the config | that list plus a11y and keyboard |
| `--quick` | quick's list plus a11y and keyboard |

Viewports don't change. `eaa: true` in [`.soundcheck/config.yaml`](Soundcheck-Configuration#eaa-mode) does the same as `--eaa` on every command; `--eaa=false` turns it off for one run. A `compare` takes `--eaa` and the WCAG version from its snapshot, so you name them once.

| Flag | Default | What it does |
|---|---|---|
| `--eaa` | off | Accessibility evidence for the European Accessibility Act: the modules above, WCAG 2.1 rules, a statement draft and an evidence pack in the run folder. |
| `--wcag <version>` | `2.1` (a compare uses its snapshot's) | The a11y rules: `2.0`, `2.1` or `2.2`. `--eaa` takes 2.1 or 2.2, since the EAA's standard, EN 301 549 V3.2.1, maps to WCAG 2.1 AA. |
| `--staging` | off | This target is a staging site: key flows marked `keyboard: send` may submit on a public address. |
| `--level flows` | off | Runs the key flows in `.soundcheck/flows/`. |

### What it never does

- It never gives a verdict on a site and never picks the statement's status. The draft says what the tests found per criterion; you choose "fully", "partially" or "not compliant" after your own review.
- The walk never presses a link that leaves the page, a submit control or anything that looks destructive (delete, log out, unsubscribe, place order, pay, buy, checkout, empty cart, in six languages). It never types into fields, never acts inside frames from other sites and never walks `/wp-admin` or `/wp-login.php`.
- While it has a page, every request that would send data (anything but GET and HEAD), WebSocket connections, downloads, file choosers, permission prompts and passkey requests are held or refused. The header says so: `Never: submits a form, places an order or types into fields outside key flows`.
- A key flow only really submits with `keyboard: send` on this machine, a private address or with `--staging`. It never does on a URL listed under `production:`.
- A page whose audit didn't finish is never reported as fixed: its issues are kept from the snapshot, the statement marks it not tested and the run exits 3.
- Nothing leaves your machine except requests to your own site and hosts you allow. No telemetry, no hosted service, no AI.

## Install

EAA mode is part of Soundcheck 1.3.0 and later. With the [installer](Install), `rhythm install soundcheck` adds Soundcheck to an existing `rhythm`; `rhythm update` brings an older Soundcheck up to date. Check the version:

```sh
rhythm install soundcheck
rhythm soundcheck --version
```

```text
rhythm-soundcheck 1.3.0
```

A build from source (`make build` in a clone, see the [README](https://github.com/RhythmShahriar/soundcheck#install)) prints `rhythm-soundcheck dev (1.3.0+<commit>)`: the release it is heading for and its commit. Anything from 1.3.0 on has EAA mode.

EAA mode needs Chrome or Chromium, like the a11y module: Soundcheck uses an installed Chrome, or downloads Chrome for Testing once, checked against a SHA-256 pinned in the binary ([The browser](Soundcheck#the-browser)). `--browser firefox` and `--browser webkit` are refused under `--eaa` (see [Usage errors](#usage-errors)).

No site handy? `rhythm soundcheck demo --eaa` serves a small built-in bakery site on 127.0.0.1, snapshots it, breaks it (a keyboard trap in the newsletter field, focus lost when a dialog closes, plus the usual PHP warning and 404) and shows the comparison, in under a minute and without touching your project folder. Its folder is deleted afterwards; `demo --eaa --keep` keeps it, with the statement draft and evidence pack the demo wrote. `rhythm soundcheck help demo` has the rest.

## Your first EAA run

This walk-through uses "Ledger", the plain-PHP site of Soundcheck's [testbed](Soundcheck-Benchmark#the-testbed), served on this machine. Every output below is what Soundcheck 1.3.0 printed for it. To follow along, serve it from a clone of the Soundcheck repository with PHP's built-in server ([testbed/README.md](https://github.com/RhythmShahriar/soundcheck/blob/master/testbed/README.md#plain-php-without-docker) has more):

```sh
mkdir -p /tmp/s
TESTBED_STATE=/tmp/s php -d display_errors=1 -S 127.0.0.1:39200 -t testbed/php
```

Any free port works. The outputs below used 39200; the testbed's Docker stack serves the same site on 18810. `TESTBED_STATE` is the folder where `testbed/toggle.sh` marks scenarios broken, so give both the same one.

On your own site, use a staging address in place of `http://127.0.0.1:39200/`. List the live site under `production:` in the config so nothing ever submits there.

### 1. Write the config

```sh
rhythm soundcheck init --eaa
```

```text
Wrote .soundcheck/config.yaml. Every setting is optional; edit what you need.
Wrote .soundcheck/flows/keyboard-checkout.yaml, a key flow to edit; it runs with --level flows.
Fill in statement: before you run rhythm soundcheck statement.
```

The file has `eaa: true` and commented `a11y:`, `keyboard:` and `statement:` blocks. Fill in the statement block (who runs the site, its public address and where people report problems), name the pages that hold widgets you want tested and turn on key flows:

```yaml
eaa: true

keyboard:
  pages: ["/a11y/wishlist.php"]   # widgets are tested on the first page of each URL group and here

statement:
  owner: "Ledger Coffee GmbH"
  website: "https://ledger.example"
  contact:
    email: "accessibility@ledger.example"
  response_days: 10
  country: "DE"

pages:
  extra: ["/a11y/", "/a11y/consent.php", "/a11y/wishlist.php"]

interactions:
  levels: [inspect, click, flows]   # flows runs the key flows in .soundcheck/flows
```

Ledger has no WooCommerce checkout, so `keyboard-checkout.yaml` is replaced by a key flow for its office order form, `.soundcheck/flows/keyboard-order.yaml`:

```yaml
name: Order by keyboard
start: /a11y/
keyboard: true
steps:
  - click: "#delivery-open"
  - expect: {focus_within: "dialog[open]"}
  - press: Escape
  - expect: {focus: "#delivery-open"}
  - select: {selector: "#bag", value: "Colombia Huila, 250 g"}
  - fill: {selector: "#name", value: "${fake:name}"}
  - fill: {selector: "#email", value: "${fake:email}"}
  - fill: {selector: "#street", value: "${fake:address}"}
  - fill: {selector: "#postcode", value: "${fake:postcode}"}
  - press: Tab
  - expect: {focus: "button.primary"}
```

Every step reaches its element with Tab, never with the mouse. See [Key flows](Soundcheck-Keyboard-Walkthrough#key-flows).

### 2. Take the snapshot

```sh
rhythm soundcheck snapshot http://127.0.0.1:39200/ --name before
```

```text
rhythm soundcheck 1.3.0 - http://127.0.0.1:39200/

Preset: --eaa: modules health, a11y, keyboard; WCAG 2.1
Modules: health, a11y, keyboard
Accessibility: axe-core 4.14.0, WCAG 2.1 A and AA rules, reflow at 320 px
Keyboard: Tab walk on every page (up to 150 stops), menus and dialogs; 1 key flow (keyboard-order.yaml) runs
Never: submits a form, places an order or types into fields outside key flows
Levels: no clicks or submits (the interact module is off); key flows run
Viewports: desktop, mobile
Production: none listed: only key flows with keyboard: send submit, on a local or private address or with --staging
Snapshot: "before"
Pages: 4 pages to start
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
[1/6] /a11y/wishlist.php 200 1.32s (ok)
[2/7] /a11y/consent.php 200 1.36s (ok)
[3/7] /a11y/ 200 1.35s (ok)
...
[21/21] /archive.php/2020 200 1.36s (ok)
Skipped: 7 (grouped)
Checking site-wide issues (links, sitemap)...
Checking 7 links: 7/7

Accessibility: 0 of 50 WCAG 2.1 A and AA criteria have failures, 27 had none
found by these tests, 23 need a person
Snapshot "before" saved; no failures found by these tests - 0 errors, 0 warnings, 0 notices - 21 pages, 23s
Automated and keyboard tests find some accessibility problems, not all. This is
evidence for your own review, not a statement that the site meets WCAG,
EN 301 549 or the European Accessibility Act.
full report: .soundcheck/snapshots/before/report.html
statement: .soundcheck/snapshots/before/eaa/statement-draft.md (a draft for you to finish:
           rhythm soundcheck statement -o accessibility-statement.md)
next: make your change (update a plugin, deploy…), then run: rhythm soundcheck compare
```

`eaa: true` in the config means no `--eaa` on the command line. The `Levels:` line says the interact module is off under `--eaa`, so nothing is clicked or submitted outside key flows, which run because the config lists the `flows` level. The `Production:` line says what could still submit: only a key flow marked `keyboard: send`, only on this machine, a private address or with `--staging`. `--plain` (and any output that isn't a terminal) writes commas where the live view has `·`. The site's verdict never says "all clear" under `--eaa`: with nothing found it says `no failures found by these tests`, which is all the tests can say. The counts line splits the 50 WCAG 2.1 A and AA criteria into three: those with failures, those the tests looked at and found none in, those only a person can judge.

### 3. Make the change and compare

Two updates go out: a consent plugin's banner now grabs focus and keeps it; the wishlist's "Save for later" control lost its key handler (testbed scenarios s65 and s57). Make them in another terminal, from the clone:

```sh
TESTBED_STATE=/tmp/s testbed/toggle.sh s65 broken
TESTBED_STATE=/tmp/s testbed/toggle.sh s57 broken
```

Compare:

```sh
rhythm soundcheck compare --fail-on broke
```

```text
rhythm soundcheck 1.3.0 - http://127.0.0.1:39200/

Site: from .soundcheck/snapshots/before/run.json (target)
Preset: --eaa: modules health, a11y, keyboard; WCAG 2.1
...
Snapshot: "before", 21 pages, taken just now
Settings: from snapshot "before" (modules health,a11y,keyboard; levels flows; viewports desktop,mobile)
Confirm: BROKE and NEW pages are visited again before they're reported; --confirm=off skips it
Pages: 21 pages to start
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
[1/21] /a11y/consent.php 200 1.33s (error)
...
[7/21] /a11y/wishlist.php 200 1.33s (1 issue)
...
Checking 7 links: 7/7
Confirming: visiting 2 pages again, to tell a real break from a flaky one
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
Confirming: visiting 2 pages once more, where the change came back
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)

[BROKE] Keyboard focus is trapped - /a11y/consent.php
  source: #consent
  wcag: 2.1.2; EN 301 549 9.2.1.2
  path: #consent (4 times)
  Tab and Shift+Tab kept focus here for 8 presses; 2 focusable elements after it (#main > p > a:nth-of-type(1), #main > p > a:nth-of-type(2)) can't be reached.
  fix: Tab and Shift+Tab can't move focus out of an element or region, so a keyboard user is stuck there.
  rhythm soundcheck accept da3cdeac if this is expected
[NEW] Control doesn't respond to Enter or Space - /a11y/wishlist.php
  source: #save
  wcag: 2.1.1; EN 301 549 9.2.1.1
  span "Save for later" reacts to a mouse click but not to Enter or Space.
  fix: A custom control reacts to a click but not to Enter or Space. Use a native button, or handle both keys.
  rhythm soundcheck accept 6b8bcc6f if this is expected

Accessibility: 2 of 50 WCAG 2.1 A and AA criteria have failures (2.1.1, 2.1.2
new), 25 had none found by these tests, 23 need a person, open issues 0 to 2
Something broke - 1 broke, 1 new, 0 changed, 0 slower, 0 fixed - compared with snapshot "before" (1m ago), 21 pages, 17s
Automated and keyboard tests find some accessibility problems, not all. This is
evidence for your own review, not a statement that the site meets WCAG,
EN 301 549 or the European Accessibility Act.
full report: .soundcheck/runs/2026-10-28T20-26-34.073Z/report.html
statement: .soundcheck/runs/2026-10-28T20-26-34.073Z/eaa/statement-draft.md (a draft for you to finish:
           rhythm soundcheck statement -o accessibility-statement.md)
```

It exits 1: the trap is BROKE, which meets `--fail-on broke`. Both pages were visited again (the [confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable)) and the keyboard walk found both problems again, so neither is UNSTABLE.

### 4. Read the result

| Line | Means |
|---|---|
| `[BROKE] Keyboard focus is trapped - /a11y/consent.php` | The class, the check's title and the pages. A trap is an error, so it is BROKE; the not-operable control is a warning, so NEW. |
| `source: #consent` | The element, as a stable CSS selector. The issue is keyed by check and selector, so a label that changes doesn't make it new. |
| `wcag: 2.1.2; EN 301 549 9.2.1.2` | The WCAG success criterion it fails and the EN 301 549 clause, the EAA's standard. The live view writes `·` for the semicolon. |
| `path: #consent (4 times)` | The last six Tab stops before the problem, a stop repeated in a row folded into one. `--plain` writes `then` between stops; the live view uses arrows. |
| the detail | What happened, in the walk's words. Accessible names are masked: emails, phone, card and IBAN numbers never reach a report. |
| `Accessibility: 2 of 50 ...` | Criteria with failures (the new ones in brackets), criteria with none found by these tests, criteria that need a person; then the open accessibility issues before and after. When the tests didn't finish on some page it also says `N pages not fully tested` ([below](#when-the-tests-didnt-finish)). |
| the last sentence | The evidence note. Every `--eaa` result ends with it, `-q` included. |

The HTML report (`full report:`) has an **Accessibility** tab next to **Results**: the counts, a grid of every criterion by principle with its state as a word and an icon (`✕ failures (<n>)`, `○ none found`, `? needs a person`), the issues by criterion and page and each page's keyboard walk as a numbered list of its stops.

### 5. Fix, accept or ignore

Fix the trap, then compare again. A problem you know about and plan to fix later belongs in the config, where it is reviewed with the code:

```sh
rhythm soundcheck accept 6b8bcc6f --ignore --reason "wishlist button is rebuilt in the next sprint" --until 2026-12-31
```

```text
Ignored 6b8bcc6f keyboard/not-operable (/a11y/wishlist.php): rule added to .soundcheck/config.yaml
Commit .soundcheck/config.yaml to share the rules; the snapshot is unchanged.
```

An ignored problem never fails `--fail-on` but still counts against its criterion: ignoring a problem doesn't fix it. After the trap is fixed, the next compare ends:

```text
Accessibility: 1 of 50 WCAG 2.1 A and AA criteria have failures, 26 had none
found by these tests, 23 need a person, open issues 0 to 0
Nothing changed; 1 WCAG criterion has failures - 0 broke, 0 new, 0 changed, 0 slower, 0 fixed - compared with snapshot "before" (1m ago), 21 pages, 1 change ignored, 16s
Automated and keyboard tests find some accessibility problems, not all. This is
evidence for your own review, not a statement that the site meets WCAG,
EN 301 549 or the European Accessibility Act.
```

The trap was never in the snapshot, so its fix is no change. The verdict says `1 WCAG criterion has failures`, not `no failures found`: the ignored control still fails 2.1.1. `accept <id>` without `--ignore` makes a change part of the snapshot instead, as for any change ([Review and accept](Soundcheck-Review-And-Accept)). Under `--eaa` it also records the snapshot's evidence pack again and keeps the old one, so the chain of later runs still verifies ([Kept packs](Soundcheck-Accessibility-Statement#kept-packs)).

### 6. Draft the statement

`statement` drafts from the newest `--eaa` run or snapshot, or from the run folder or snapshot you name. Here it ran right after the comparison of step 3, so it reads that comparison:

```sh
rhythm soundcheck statement -o accessibility-statement.md
```

```text
Statement draft written to accessibility-statement.md
10 [TO DO] markers left
```

The draft starts:

```markdown
> **Draft for you to review. Not ready to publish.**
>
> Rhythm Soundcheck 1.3.0 wrote this on 2026-10-29 from a check of 21 pages of https\://ledger.example.
>
> Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.
>
> Fill in every \[TO DO\], test the criteria marked "needs a person", then delete this box.

# Accessibility statement for Ledger Coffee GmbH
...
## How accessible this website is

[TO DO: choose one after your review: "fully compliant", "partially compliant" or "not compliant" with WCAG 2.1 level AA / EN 301 549 V3.2.1. Soundcheck does not choose this for you. Its tests found failures of 2 of 50 criteria and could not test 23 of them.]
```

The address Soundcheck checked (`127.0.0.1:39200`, a staging host) never appears: the draft uses `statement.website`. The whole draft, its layouts, the EARL export for the W3C WCAG-EM Report Tool and the evidence pack are on [Accessibility statement](Soundcheck-Accessibility-Statement).

### 7. Check the evidence

`evidence verify` checks the newest pack, here the same comparison's:

```sh
rhythm soundcheck evidence verify
```

```text
Verified .soundcheck/runs/2026-10-28T20-26-34.073Z/eaa/evidence.json: every file matches and the chain is whole
not signed
```

To sign packs, make a key once and sign the run you hand over. Publish the public key so others can check it:

```sh
rhythm soundcheck evidence keygen -o soundcheck.key
rhythm soundcheck evidence sign --key soundcheck.key
rhythm soundcheck evidence verify --pub 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=
```

```text
public key: 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=
Private key written to soundcheck.key; keep it out of git
Signed .soundcheck/runs/2026-10-28T20-26-34.073Z/eaa/evidence.json
Verified .soundcheck/runs/2026-10-28T20-26-34.073Z/eaa/evidence.json: every file matches and the chain is whole
signed with key 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=, the --pub key
```

A signed evidence pack proves who produced it and that it wasn't changed afterwards. It doesn't prove the site was in that state for every visitor.

### 8. Run it in CI

```yaml
- uses: RhythmShahriar/soundcheck@v1.3.0
  with:
    url: ${{ github.event.deployment_status.target_url }}
    mode: compare
    baseline: main
    eaa: true
    comment: true
    fail-on: new        # fail on any new accessibility problem, not only traps
```

The job summary and the pull request comment carry counts, criteria, check titles, selectors, page paths and fix hints, never an issue's detail, its focus path or a screenshot. See [CI: EAA mode](Soundcheck-CI#eaa-mode).

## When the tests didn't finish

A page whose a11y audit or keyboard walk timed out, failed or was skipped can't count as tested. Soundcheck lists it before the counts, adds `N pages not fully tested` to the counts line and exits 3, whatever `--fail-on` says. Testbed scenario s74 is a page that grows to 20,000 elements while a runaway script keeps it busy. Break it, then check that page alone (`--config /dev/null` leaves out step 1's config, whose `pages.extra` would add four more pages):

```sh
TESTBED_STATE=/tmp/s testbed/toggle.sh s74 broken
rhythm soundcheck run http://127.0.0.1:39200/a11y/big.php --eaa --include '/a11y/*' --config /dev/null
```

```text
...
not checked: /a11y/big.php (a11y: reflow check at 320 px: timed out after 10s)
not checked: /a11y/big.php (keyboard: the walk took over 60 s)
Accessibility: 1 of 50 WCAG 2.1 A and AA criteria have failures, 0 had none
found by these tests, 26 not tested (their tests didn't finish), 23 need a
person, 1 page not fully tested
a few warnings - 0 errors, 20 warnings, 0 notices - 1 page, 1m36s
Automated and keyboard tests find some accessibility problems, not all. This is
evidence for your own review, not a statement that the site meets WCAG,
EN 301 549 or the European Accessibility Act.
note: keyboard: 1 key flow needs --level flows (it types test data); not run
full report: .soundcheck/runs/2026-10-28T20-27-52.250Z/report.html
statement: .soundcheck/runs/2026-10-28T20-27-52.250Z/eaa/statement-draft.md (a draft for you to finish:
           rhythm soundcheck statement -o accessibility-statement.md)
rhythm soundcheck: the accessibility evidence is incomplete: the a11y or keyboard tests didn't finish on 1 page (see "not checked" above), so the statement marks them not tested
```

A criterion is "not tested (their tests didn't finish)" only when no page finished the tests that look for it, as here with one page. When other pages finished them, the criterion keeps its count from those pages and only `1 page not fully tested` says the evidence has a gap: that page's own problems may be missing.

In a comparison the line reads `not checked: <path> (<module>: <reason>); its issues are kept from the snapshot, not counted as fixed`: a problem on a page the tests couldn't finish is never reported as FIXED. A page that answered with an HTTP error (404, 500) doesn't exit 3: health reports the page itself and the draft lists it as not tested. `-q` still prints the `not checked:` lines.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done, nothing at or above `--fail-on` (or `--fail-on` not set). |
| `1` | A change or issue at or above `--fail-on`. |
| `2` | Usage error, such as `--eaa` with `--browser firefox` or a bad `statement:` block. |
| `3` | The site couldn't be checked, or under `--eaa` a page's a11y or keyboard tests didn't finish (the evidence is incomplete). |
| `130` | Interrupted. An interrupted `--eaa` run writes its evidence pack marked interrupted and no statement draft. |

`2` and `130` win over `3`; `3` wins over `1`. The [GitHub Action](Soundcheck-CI#eaa-mode) sets its `incomplete` output in the exit 3 case and still writes every report before it fails the step.

### Usage errors

Checked before anything runs, exit 2:

```text
rhythm soundcheck: unknown flag --eea (did you mean --eaa?)
rhythm soundcheck: --eaa needs the a11y and keyboard modules; drop --skip keyboard, or leave out --eaa to run without them
rhythm soundcheck: --wcag (a11y.wcag): "2" isn't one of 2.0, 2.1, 2.2
rhythm soundcheck: --eaa maps to WCAG 2.1 AA (EN 301 549 V3.2.1); use --wcag 2.1 or 2.2, or leave out --eaa to run the 2.0 rules
rhythm soundcheck: --eaa needs Chrome: the keyboard walkthrough and axe use the Chrome DevTools protocol. Drop --browser firefox, or use --modules +a11y without --eaa
rhythm soundcheck: --eaa won't turn off Chrome's sandbox from SOUNDCHECK_NO_SANDBOX alone; add --no-sandbox to the command if you mean it
rhythm soundcheck: --eaa doesn't apply to report
```

## JSON and other formats

Every format carries the EAA results: `--json`, `--md`, `--junit`, `--html` and `-o` files of a run, a snapshot or a comparison, also `report` of a stored `--eaa` result.

### JSON

`--json` (schema `rhythm-soundcheck/v1`) gains a top-level `eaa` object. The comparison above, from `rhythm soundcheck report --json`, with the criteria lists shortened:

```json
"eaa": {
  "wcag": "2.1",
  "en301549": "V3.2.1",
  "axe": "4.14.0",
  "criteria": {
    "failed": [
      { "id": "2.1.1", "issues": 1, "pages": 1 },
      { "id": "2.1.2", "issues": 1, "pages": 1 }
    ],
    "no_failures_found": ["1.1.1", "1.2.1", "1.2.2", "1.3.1", "1.3.5", ...],
    "not_tested": [],
    "needs_person": ["1.2.3", "1.2.4", "1.2.5", "1.3.2", "1.3.3", ...]
  },
  "keyboard": {
    "pages_walked": 21,
    "widgets_tested": 1,
    "flows_run": 1
  },
  "open": 2,
  "interrupted": false,
  "evidence_note": "Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act."
}
```

The criteria lists split the 50 as the counts line does: `failed`, `no_failures_found` ("had none found by these tests"), `needs_person` ("need a person") and `not_tested`, the criteria whose tests didn't finish or didn't run ("not tested (their tests didn't finish)"), which is `[]` when every test finished. `keyboard.pages_walked` counts the walks that finished; a walk that timed out is in `not_audited` instead. `not_audited` (pages whose audits didn't finish, with the module and why) and `keyboard.flows_skipped` appear when there are any. The criteria lists are `[]`, never `null`. Each issue and change gains `wcag`, `en301549`, `impact` and, for keyboard issues, `focus_path`:

```json
{
  "id": "da3cdeac",
  "class": "broke",
  "check": "keyboard/trap",
  "title": "Keyboard focus is trapped",
  "detail": "Tab and Shift+Tab kept focus here for 8 presses; 2 focusable elements after it (#main > p > a:nth-of-type(1), #main > p > a:nth-of-type(2)) can't be reached.",
  "source": "#consent",
  "fix": "Tab and Shift+Tab can't move focus out of an element or region, so a keyboard user is stuck there.",
  "pages": ["/a11y/consent.php"],
  "score": 1,
  "wcag": ["2.1.2"],
  "en301549": ["9.2.1.2"],
  "impact": "critical",
  "focus_path": ["#consent", "#consent", "#consent", "#consent"]
}
```

A result made without `--eaa` has no `eaa` object. Runs stored by 1.2 load and render in every format.

### Markdown

`--md` (the job summary and the pull request comment) adds an accessibility section after the changes. Like the change rows above it, it holds counts, criteria, check titles, selectors, page paths and fix hints, never an issue's detail, its focus path or a screenshot, since everyone with access to the repository reads a comment:

```markdown
### Accessibility (WCAG 2.1 A and AA)

2 of 50 criteria have failures · 25 had none found by these tests · 23 need a person · 2 open issues

<details><summary>Criteria with failures</summary>

| Criterion | EN 301 549 | Issues | Pages |
|---|---|---:|---:|
| 2.1.1 Keyboard | 9.2.1.1 | 1 (new) | 1 |
| 2.1.2 No Keyboard Trap | 9.2.1.2 | 1 (new) | 1 |

</details>

> Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.
```

Rows of `a11y/*` and `keyboard/*` changes carry their criteria after the title: `Keyboard focus is trapped (2.1.2)`.

### JUnit

`--junit` writes two kinds of suite. The suites by class (`soundcheck.broke`, `soundcheck.new` and so on, by level for a run) have a case per change, as in every result ([CI: JUnit](Soundcheck-CI#junit-and-the-job-summary)). An `--eaa` result adds one suite per page with a test case per page and module, so a CI test view shows which page failed which test:

```xml
<testsuite name="soundcheck.page /a11y/consent.php" tests="2" failures="1" errors="0" skipped="0" timestamp="2026-10-28T19:28:47Z">
  <testcase name="/a11y/consent.php · a11y" classname="soundcheck.a11y"></testcase>
  <testcase name="/a11y/consent.php · keyboard" classname="soundcheck.keyboard">
    <failure message="Keyboard focus is trapped" type="error">check: keyboard/trap&#xA;wcag: 2.1.2 · EN 301 549 9.2.1.2&#xA;path: #consent (4 times)&#xA;source: #consent&#xA;...</failure>
  </testcase>
</testsuite>
```

A page case fails on the issues that fail their class suite: in a run, the issues of that module on that page at or above `--fail-on` (errors without it); in a comparison, the ones a failing change covers. So with `--fail-on new` a NEW warning fails both `soundcheck.new` and its page's case. An error the snapshot already had is written as the case's output. A module that didn't finish on a page makes the case skipped with `not checked: a11y: <reason>`.

### Client report, history and fleet

- `report --client` adds an **Accessibility** section in plain words: `We tested the site against the WCAG 2.1 A and AA criteria with automated rules and by using it with the keyboard alone.`, four counts, the criteria with failures (`2.1.1 Keyboard (1 problem on 1 page)`), what still needs a person and the evidence note. It never shows selectors or check ids. See [Client reports](Soundcheck-Fleet-And-Client-Reports#client-reports).
- `history --metric a11y` (or `keyboard`) shows the open accessibility (or keyboard) problems of every run and the worst criteria now. See [History](Soundcheck-History#accessibility-and-keyboard).
- `fleet` takes `eaa: true` per site (or `--eaa` for all) and adds open issues, criteria with failures, their trend and a link to each site's statement draft. See [Fleet](Soundcheck-Fleet-And-Client-Reports#eaa-mode).

## Honest limits

These sentences are printed word for word by `rhythm soundcheck help eaa`, in the README and in every statement draft:

1. Soundcheck's EAA mode gives you evidence about your site. It is not an audit, not a certification and not legal advice.
2. Many success criteria need a person: meaningful alt text, captions, reading order, plain language. The draft marks them not tested: needs a person, never passed.
3. Only the pages and flows Soundcheck visited, in the states it saw, in Chrome, with the keyboard walk at desktop width. Content in iframes from other sites, PDFs, videos, apps and emails was not tested.
4. The keyboard walkthrough uses Chrome's keyboard events. It is not a screen reader test, not a voice control test and not a test with disabled users.
5. The site decides what Soundcheck sees. A site that serves headless browsers a different page can pass while visitors fail. Overlay widgets don't make a site accessible.
6. EAA mode presses keys on your pages. It holds requests that would change data, blocks downloads and follows no links except skip links. A site that changes data on a GET or on focus can still change. Run it on staging and list your live site under production.
7. Reports contain what your pages showed. Soundcheck masks form values, credentials and tokens it recognises and emails, phone numbers and card numbers in text it writes. Masking is pattern-based: it is pseudonymised, not anonymised. Screenshots are not masked.
8. A signed evidence pack proves who produced it and that it wasn't changed afterwards. It doesn't prove the site was in that state for every visitor.
9. Nothing leaves your machine except requests to your own site and hosts you allow. No telemetry, no hosted service, no AI.

What that means in practice:

- **Coverage.** Of the 50 WCAG 2.1 A and AA success criteria, the tests look for failures of 27 (29 of the 55 in WCAG 2.2). The other 23 need a person. Even for a criterion the tests look at, only a person can say it is met. A person's review can follow [WCAG-EM](https://www.w3.org/WAI/test-evaluate/conformance/wcag-em/), the W3C's method; `statement --earl` hands Soundcheck's results to the W3C's WCAG-EM Report Tool.
- **Pages visited anonymously.** Pages are checked as a visitor. Pages behind a login (the account area) are reached only by key flows with `login: true`; the a11y audit doesn't run on them.
- **Widgets on a sample.** Menus, dialogs and disclosures are tested on the first page of each URL group (by path) and on the pages in `keyboard.pages`, not on every page. A control on another page of the group is still reached by Tab and never pressed, so `keyboard/not-operable` can miss it there.
- **Desktop only.** The walk runs at desktop width; a menu that exists only at phone width isn't walked.
- **Chrome only.** `--eaa` refuses Firefox and WebKit.
- **What a dry key flow holds.** `keyboard: true` holds the writes of its own tab. A popup, a `target=_blank` form or a message over an open WebSocket isn't held. That is why a dry flow is refused on `production:` URLs like any flow.
- **Network scope.** On a public target, the run's tabs can't load loopback, private or link-local addresses, unless `--allow-host` names them. WebSocket handshakes aren't intercepted.

More answers are in the [FAQ](Soundcheck-FAQ#eaa-mode).

## Benchmark

The EAA scenarios of the testbed (s49 to s89) run with `--eaa`. Broken ones must be caught; good ones, nothing broken or a page built to be hard (a native `<dialog>`, focus shown by box-shadow, a visually hidden skip link, 300 old contrast problems), must stay quiet.

| | Result |
|---|---|
| Broken scenarios caught | **29 of 29** (s68, which needs the audit of logged-in pages, is pending) |
| False alarms in good scenarios | **0 of 11** |
| False alarms in 60 `--eaa` compares of unchanged sites | **0** |
| Differing issue keys over 10 snapshots of the same pages | **0** |
| a11y audit per page, median | 0.00 s (target at most 2 s) |
| Keyboard walk per page, median | 5.79 s on WordPress (over the 3 s target), 1.02 s on plain PHP |
| Whole `--eaa` run against a default run | 0.8× on WordPress, 0.6× on plain PHP (target at most 3×) |
| One key flow (`/checkout/`) | 2.2 s (target at most 15 s) |

The testbed is written by the same people as the tool. It shows each detector works on a known case, not how much of a real site's accessibility it covers. Deque reports axe-based tests found 57% of issues by count in its own audits; that is a vendor figure counting issues, not success criteria. Every scenario and number: [Benchmark: EAA mode](Soundcheck-Benchmark#eaa-mode).

## Where to go next

| You want to… | Read |
|---|---|
| Know what the keyboard walk presses and reports | [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough) |
| Replay a checkout by keyboard | [Key flows](Soundcheck-Keyboard-Walkthrough#key-flows) |
| Finish and publish the statement | [Accessibility statement](Soundcheck-Accessibility-Statement) |
| Sign and verify the evidence | [The evidence pack](Soundcheck-Accessibility-Statement#the-evidence-pack) |
| Run it on every pull request | [CI: EAA mode](Soundcheck-CI#eaa-mode) |
| Look up a setting | [Configuration: EAA mode](Soundcheck-Configuration#eaa-mode) |
| Look up a flag or a command | [CLI reference](Soundcheck-CLI-Reference) |
| Find an answer | [FAQ: EAA mode](Soundcheck-FAQ#eaa-mode) |

---
<sub>← [Accessibility and SEO](Soundcheck-Accessibility-And-SEO) · [Home](Home) · [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough) →</sub>
