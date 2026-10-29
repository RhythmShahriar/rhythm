How Rhythm fits together: a small router CLI, a shared kit and tools that live in their own repositories and release on their own. The second half walks through the internals of three tools: Tuner, from the first request of a scan to the report; Soundcheck, from discovery to a comparison; Echo, from a capture to a comparison; Echo's cookie consent check, from a URL to a verdict. Roadie's design (what it reads, the dump allowlist, the spool, every network request) is on its own page: [Security and limits](Roadie-Security-And-Limits). Sightread's design (what it fetches, verifies and analyses) is on its own page: [Security model](Sightread-Security-Model).

## The big picture

```mermaid
flowchart LR
  user(["rhythm tuner scan yoursite.com<br/>rhythm soundcheck compare<br/>rhythm echo up<br/>rhythm roadie copy<br/>rhythm sightread all"]) --> router["rhythm<br/>(router)"]
  router -- "exec rhythm-tuner …" --> tuner["rhythm-tuner"]
  router -- "exec rhythm-soundcheck …" --> sc["rhythm-soundcheck"]
  router -- "exec rhythm-echo …" --> echo["rhythm-echo"]
  router -- "exec rhythm-roadie …" --> roadie["rhythm-roadie"]
  router -- "exec rhythm-sightread …" --> sr["rhythm-sightread"]
  router -- "exec rhythm-&lt;name&gt; …" --> ext["extensions"]
  router -. "install / update / verify / lock" .-> gh[("GitHub Releases")]
  router -. "signature root (TUF)" .-> sigstore[("Sigstore")]
  tuner -. "check packs, update notice" .-> gh
  sc -. "update notice" .-> gh
  echo -. "update notice" .-> gh
  roadie -. "profile pack, update notice" .-> gh
  roadie -- "echo/connectors/wordpress, echo/client" --> echo
  roadie -. "--soundcheck" .-> sc
  sr -. "--soundcheck" .-> sc
  sc --> chrome["headless Chrome"]
  sc -- "echo/client, echo/compare" --> echo
  subgraph kit ["rhythm/kit (Go module)"]
    ui["kit/ui<br/>theme"]
    config["kit/config<br/>user dirs"]
    update["kit/update<br/>verified installs"]
    sigverify["kit/sigverify<br/>release signatures"]
    crawl["kit/crawl<br/>discovery"]
    httpx["kit/httpx<br/>site-scoped HTTP"]
    rl["kit/ratelimit<br/>polite pacing"]
    cache["kit/cache<br/>disk cache"]
    cli["kit/cli<br/>flags, help, tool skeleton"]
    secrets["kit/secrets<br/>masking"]
    term["kit/term<br/>safe text"]
  end
  router --- update
  router --- sigverify
  router --- cli
  tuner --- ui
  tuner --- config
  tuner --- update
  sc --- kit
  echo --- kit
  roadie --- kit
  sr --- kit
```

| Piece | Repository | Role |
|---|---|---|
| `rhythm` | [`RhythmShahriar/rhythm`](https://github.com/RhythmShahriar/rhythm) | Router CLI and its commands (`install`, `update`, `list`, `verify`, `lock`, `bundle`, `ext`, `doctor`), the install scripts and the `setup` GitHub Action |
| `kit/` | same repository, module `github.com/RhythmShahriar/rhythm` | Shared base every tool imports |
| `rhythm-tuner` | [`RhythmShahriar/tuner`](https://github.com/RhythmShahriar/tuner) | WordPress & PHP security scanner |
| `rhythm-soundcheck` | [`RhythmShahriar/soundcheck`](https://github.com/RhythmShahriar/soundcheck) | Whole-site before/after checks in a real browser |
| `rhythm-echo` | [`RhythmShahriar/echo`](https://github.com/RhythmShahriar/echo) | Captures and compares what a site sends; seals staging. Also publishes the npm connector `@rhythmsh/echo` |
| `rhythm-roadie` | [`RhythmShahriar/roadie`](https://github.com/RhythmShahriar/roadie) | Makes pseudonymised staging copies of production with live keys swapped and side effects off, verifies them and signs the report. Also the `RhythmShahriar/roadie@v1` GitHub Action |
| `rhythm-sightread` | [`RhythmShahriar/sightread`](https://github.com/RhythmShahriar/sightread) | Reads plugin, theme, Composer and npm updates before they're installed: diffs each against its official source and ranks what is new. Also the `RhythmShahriar/sightread@v1` GitHub Action |
| `rhythmsh/echo-laravel` | [`RhythmShahriar/echo-laravel`](https://github.com/RhythmShahriar/echo-laravel) | Echo's Laravel connector, split from `connectors/laravel` in the Echo repository for Packagist |
| Shared workflows | [`RhythmShahriar/.github`](https://github.com/RhythmShahriar/.github) | Reusable CI, commit lint and release workflows, the writing-style, header and kit-pin scripts, community files |

## The router

`rhythm` follows the same extension model as `git` and `gh`: it runs tools it doesn't contain.

- `rhythm <tool> [args]` looks up `rhythm-<tool>` on your `PATH` and runs it with the remaining arguments. On Linux and macOS it replaces itself with the tool (exec), so signals reach the tool directly and its exit code is rhythm's; on Windows it runs the tool as a child and waits. It sets `RHYTHM_INVOKED_AS` (`rhythm sc`) so the tool's help shows what you typed.
- **Aliases** are short names: `t` → `tuner`, `sc` → `soundcheck`, `e` → `echo`, `ro` → `roadie`, `sr` → `sightread`. They work wherever a tool name does.
- A name is checked before any lookup (`^[a-z0-9][a-z0-9-]{0,63}$`), so `rhythm /../evil` never reaches the file system. An unknown name gets a did-you-mean, a known tool that isn't installed exits 3 with the install command. See [Command reference](Command-Reference#running-a-tool).
- Any other executable `rhythm-<name>` on the `PATH` runs too. [Extensions](Extensions) installed with `rhythm ext` are also updated, locked and verified.

### Commands and the registry

The commands live in [`cmd/rhythm`](https://github.com/RhythmShahriar/rhythm/tree/master/cmd/rhythm), one file each. Each registers itself in a command table, which `rhythm help`, did-you-mean and shell completion read. Completion reads each command's flags from its own `--help`. A name that isn't a command is a tool.

The tools rhythm knows are data, in [`cmd/rhythm/tools.json`](https://github.com/RhythmShahriar/rhythm/blob/master/cmd/rhythm/tools.json), embedded at build time:

```json
{
  "name": "echo",
  "repo": "RhythmShahriar/echo",
  "aliases": ["e"],
  "description": "is my site still saying what it should? email, SMS, webhooks and cookie consent",
  "default_install": true,
  "min_rhythm": ""
}
```

Help's Tools block, `rhythm install --defaults` (the tools with `default_install`), the installers' default tools and did-you-mean all come from it. A tool with a `min_rhythm` newer than the running rhythm is listed as `(needs rhythm >= 0.9.0)`. The file is validated when rhythm starts, so the tests catch unknown keys, duplicate names or aliases, bad names and a `repo` that isn't `owner/name`. Adding a tool is one entry here; see [Release process](Release-Process#adding-a-new-tool).

### Installing and verifying

`rhythm install`, `update`, `lock`, `bundle` and `verify` share one path through `kit/update` and `kit/sigverify`:

```mermaid
flowchart TD
  A["Resolve the release<br/>latest, tool@version, --to, --min-age, rhythm.lock"] --> B["Download checksums.txt<br/>and checksums.txt.sigstore.json"]
  B --> C{"Signature<br/>kit/sigverify"}
  C -- "refused" --> X["install nothing"]
  C -- "ok, signer recorded" --> D["Archive: cache or download,<br/>hashed while it streams"]
  D --> E{"sha256 matches checksums.txt<br/>(and rhythm.lock)?"}
  E -- "no" --> X
  E -- "yes" --> F["Unpack the binary next to the old one"]
  F --> G["Keep the old one as .prev,<br/>rename the new one into place"]
  G --> H["Record the sha256 in verified.json"]
```

The signer is pinned per repository: rhythm and its tools must be signed by the shared `release-go.yml` workflow for that repository and tag; an extension by a workflow of its own repository. The Sigstore trusted root is refreshed through TUF on the first check of a command and falls back to the root built into rhythm when that fails or when `RHYTHM_OFFLINE` is set. See [Verifying releases](Verifying-Releases).

Three internal packages hold the router's own formats:

| Package | Holds |
|---|---|
| `internal/lock` | `rhythm.lock` (`rhythm-lock/v1`): strict reading, atomic writing, the search up to the git root, the diff `update --lock` prints; and `extensions.json` (`rhythm-extensions/v1`) |
| `internal/bundle` | offline bundles (`rhythm-bundle/v1`): writing, validating the whole tar before anything is read and serving it to `kit/update` as a release source |
| `internal/ext` | extension names and specs, the signer rule and the GitHub search |

`rhythm update` uses the registry and `extensions.json` to find installed tools on the `PATH`, updates each binary, then runs `rhythm-<tool> update` so each tool refreshes its own data (`update --json` for tools on `kit/cli`, which report their data packs). Copies a package manager owns, including mise, asdf and Scoop shims, are left to that manager. See [Updating](Updating).

## The kit

`kit/` is a Go module every tool imports, so every tool looks the same and installs and updates the same way. `kit.Version()` reports the kit version a program was built with.

| Package | What it provides |
|---|---|
| `kit` | `Banner`, the standard header line: `♪ rhythm tuner  0.3.1`; `Version` |
| `kit/ui` | The terminal theme, built from the Rhythm brand tokens: **Ink** `#100F0D`, **Paper** `#FFFFFF` and one signal color, **Dot** `#FF7C52` (a deeper `#E8603A` on light backgrounds). Severities use conventional colors: critical red, high orange, medium yellow, low blue, ok green, muted gray. Built on Lip Gloss. `Truncate` and `Pad` count terminal cells, so CJK text and emoji line up. |
| `kit/config` | The per-user config and cache folders (`~/.config/rhythm`, `~/.cache/rhythm/<tool>` or the OS equivalents, moved with `RHYTHM_CONFIG_DIR` and `RHYTHM_CACHE_DIR`) and `FirstRun`, which records one-time notices such as Tuner's responsible-use note |
| `kit/update` | Release lookup (`Latest`, `ByTag`, `Resolve` for `0.3.1`, `0.3` or `latest`, `MinAge` for a cooldown), the verified install (signature through a `Verifier`, `checksums.txt`, an expected sha256, size caps, a download cache, progress), the atomic swap with a `.prev` copy and `Rollback`, typed errors (`ErrUnreachable`, `ErrRateLimited`, `ErrNoRelease`, `ErrChecksum`, `ErrSignature`, `ErrTooNew`, `ErrOffline`), pluggable release sources and the once-a-day update check with `Notice`. Tokens follow the [one rule](Command-Reference#release-api-and-tokens) and only `https` is used. |
| `kit/sigverify` | Checks the Sigstore bundle over a release's `checksums.txt`: Fulcio certificate, Rekor entry, timestamp, the OIDC issuer, the `release-go.yml` identity, the source repository and the tag. Reads both the current Sigstore bundle format and the older cosign one. Embeds a trusted root and refreshes it through TUF. `sigverifytest` signs fake releases for tests. |
| `kit/crawl` | Site discovery: `robots.txt` (RFC 9309: an unreachable one allows only the start page), sitemaps on the site only (indexes, gzip, at most 1000 files) and links (or none, with `NoFollow`, for a caller that follows links itself), URL normalization, the same-site rule (host, scheme and port), case-insensitive include and exclude patterns, the `DefaultExcludes` for admin, login, logout, unsubscribe, confirm and nonce URLs and grouping pages by URL pattern (`/product/{slug}`) so a large site is checked through samples. Its `Frontier` takes more pages while a browser renders them. |
| `kit/httpx` | One polite HTTP client for every tool: a per-host pace, retries that honor `Retry-After`, a 30 s timeout and 5 MB body cap by default, redirects only on the site, the user's `-H` headers sent only to the target host and a guard that refuses private and cloud-metadata addresses unless the target is one. A debug log with every header masked. |
| `kit/ratelimit` | An even request pace per host that halves after `429` or `503` (down to one request every two seconds) and never speeds up on its own, `Retry-After` handling up to 30 s, backoff and `Validate` for rate flags. |
| `kit/cache` | A small on-disk JSON cache with a TTL and stale fallback when the network is down, one fetch for concurrent callers, file names that never carry a key's credentials, plus atomic `WriteFile` and `WriteJSON`. |
| `kit/cli` | The shared command-line skeleton: `Main` for a whole tool (commands, help, `version --json`, `update --json` data packs, completion, exit codes, the first-run note and the release notice), flag sets with gh-style help, did-you-mean (`Suggest`), `ParseFailOn` and the exit codes 0, 1, 2, 3 and 130. |
| `kit/secrets` | Finding and masking credentials in text with 96 rules: cloud, payment, messaging, source-control and AI provider keys, JWTs, cookies, authorization and `x-*-token` headers, URLs with passwords, reset and magic-link tokens, form fields, env, YAML, JSON and PHP-serialized values, PEM private keys. `Redact` keeps at most four leading characters; `Rules` lists every rule. |
| `kit/term` | Safe text from untrusted sources: control and bidi characters made visible or replaced for terminals, Markdown escaping, inline code and links for http(s) URLs only. |

`kit/ui` also has the shared report pieces: severity badges, section rules and the summary panel. Soundcheck uses `kit/crawl`, `kit/ratelimit`, `kit/cache` and `kit/cli`; Echo uses `kit/ui`, `kit/cli`, `kit/cache`, `kit/config`, `kit/update` and `kit/secrets`. Roadie uses `kit/cli`, `kit/cache`, `kit/config`, `kit/httpx`, `kit/ratelimit`, `kit/secrets`, `kit/sigverify`, `kit/term`, `kit/ui` and `kit/update`, plus Echo's `connectors/wordpress`, `client`, `model`, `wire` and `codes` at Echo v1.1.2. Sightread uses `kit/cli` (it is the first tool built on `cli.Main`), `kit/httpx`, `kit/cache`, `kit/config`, `kit/ratelimit`, `kit/secrets`, `kit/sigverify`, `kit/term`, `kit/ui` and `kit/update`. The tools adopt `kit/cli.Main`, `kit/httpx` and `kit/term` as they move to the newer kit. The kit's changes reach users only once a kit version is tagged and each tool moves to it.

`kit/update` relies on a naming contract with every repository's GoReleaser config; see [Release Process](Release-Process#the-naming-contract).

## Independent releases

Each tool is its own repository, with its own version numbers, changelog and release cadence. A Tuner release doesn't need a `rhythm` release. The same goes the other way round. They share:

- the release workflow and GoReleaser layout, so every release has the same archive names, `checksums.txt`, Sigstore signature and attestations;
- the kit, pinned by version in each tool's `go.mod`. CI fails a tool whose pin falls more than one minor release behind.

When a kit change is needed by a tool, the kit is tagged (it is a `rhythm` release) and the tool bumps its requirement; see [Release Process](Release-Process#when-the-kit-changes). For local development the repositories sit side by side, linked by a `go.work` file; see [Contributing](Contributing#set-up).

## Inside Tuner

```text
tuner/
├── cmd/rhythm-tuner   CLI: flags, check selection, picker, output wiring, update
├── engine             runs a scan: requests, rate limiting, baselines, checks, score
├── fingerprint        identifies the platform from the home page
├── matchers           evaluates a check's conditions; finds and masks secrets
├── checks             the built-in YAML checks and fixtures, embedded in the binary
├── checkpack          downloads, verifies and loads check packs
├── vulns              vulnerability data client, cache and the advisor
├── local              white-box scans of a directory on the server
└── report             terminal view, JSON, JSONL, SARIF, Markdown, baselines
```

### A remote scan, step by step

```mermaid
flowchart TD
  A["Fetch / (following redirects)"] --> B["TLS probe"]
  B --> C["Fingerprint"]
  C --> D["PHP probe"]
  C --> E["Version resolution<br/>(skipped by --profile quick)"]
  D --> F["Soft-404 baseline"]
  E --> F
  F --> G["Checks<br/>(worker pool)"]
  F --> H["Advisor<br/>(known vulnerabilities)"]
  G --> I["Sort, Tune score, report"]
  H --> I
```

1. **Home page.** The engine fetches `/`, following redirects. That response is shared by every check that requests `/`.
2. **TLS probe.** On `https`, it inspects the handshake: certificate expiry, whether the chain and hostname verify and whether the server still accepts TLS 1.0 or 1.1.
3. **Fingerprint.** From the home page, it identifies the platform (`wordpress`, `laravel`, `php`), the web server and any versions the page reveals.
4. **PHP probe and version resolution**, run concurrently. If the home page didn't state the PHP version, for example because a page cache stripped `X-Powered-By`, it asks up to three uncacheable endpoints. Version resolution reads WordPress core, plugin and theme versions (up to 50 components) from places like the feed generator, plugin `readme.txt` and theme stylesheets. Quick scans skip resolution but keep the PHP probe.
5. **Soft-404 baseline.** It requests a random path to learn whether the server answers `200` for pages that don't exist. During checks, a hit on a path like `/.env` is also compared with a random path of the same shape, so catch-all servers don't produce false findings.
6. **Checks and advisor, concurrently.** A worker pool runs every selected check whose `requires` matches the fingerprint, while the advisor matches the fingerprint against known vulnerabilities. Findings stream to the terminal as they're found.
7. **Finish.** Findings are sorted by severity, confirmed before *likely*. Then the Tune score is computed.

All requests go through one session per target: a rate limiter that spaces requests evenly (5 per second by default) and halves the pace when the server answers `429` or `503`, down to one request every two seconds; retries that honor `Retry-After` (up to 30 seconds); a per-path response cache so no URL is fetched twice; and a client that never follows redirects for checks. Responses are capped at 1 MiB for matching. Several targets scan in parallel (`--parallel`, default 4), each with its own session and limits. See [Requests](Tuner-Requests).

### Checks and matchers

A check is declarative YAML: an ID, severity, profile, tags, optional `requires`, a GET request for one or more paths and a `match` block. The `matchers` package evaluates each condition against the response: status, body regex and words, magic bytes, headers, cookies and their flags, scheme, TLS state and `not_baseline`. Evidence is redacted before it leaves the engine. `extract.secrets` reports credentials only in masked form.

Built-in checks are embedded in the binary from `checks/`. Unknown fields are rejected when checks load, which is also how an older binary refuses a check pack it can't run. See [Writing Checks](Tuner-Writing-Checks).

### Vulnerability data

The `vulns` client fetches advisories from WPVulnerability, latest-release and status data from the wordpress.org API and PHP's supported branches from php.net. Every record is cached as a JSON file under `~/.cache/rhythm/tuner/vulns` (or the OS equivalent):

- fresh records (under 24 hours old) are used as is;
- older ones are re-fetched when a scan needs them; if that fails the stale copy is used with a warning;
- `--offline` uses the cache only, however old;
- requests to the data services are paced (200 ms apart) and deduplicated within a scan.

The **advisor** turns the fingerprint into findings: known vulnerabilities in core, plugins, themes, PHP and the web server; outdated, closed and abandoned plugins; end-of-life PHP. Versions that a Linux distribution may have patched are reported as *likely*. It runs alongside the checks and goes through the same check selection. See [Known Vulnerabilities](Tuner-Known-Vulnerabilities).

### Check packs

The `checkpack` client lets checks update like data. At most once a day it fetches the `checks-latest` release's manifest, `checks-sha256.txt` and `checks.tar.gz`. It verifies the checksums, unpacks the archive in memory with strict limits and loads the checks with the same strict loader as built-in checks. A pack replaces the built-in checks only when it was built from a later commit and its `min_tuner` allows this binary. Any failure falls back to the cached or built-in checks with a warning. See [Check Packs](Tuner-Check-Packs).

### Local mode

When the target is a directory, Tuner runs the `local` package instead of the HTTP engine. It reads exact versions from disk (WordPress core, plugin and theme headers, `composer.lock`) and PHP from the `php` binary, reviews `wp-config.php`, flags files that don't belong in a web root, matches Composer packages against OSV.dev and then runs the same advisor. It only reads files, never runs site code and stops walking after 250,000 entries. See [Local Scans](Tuner-Local-Scans).

### Reports

The `report` package renders one result model several ways:

| Renderer | Used for |
|---|---|
| Pretty | The live terminal view, styled with the kit theme; `--plain` drops color, boxes and glyphs |
| JSON | `--json`, the documented report schema (an array for several targets) |
| JSONL | `--jsonl`, one finding per line as it's found |
| SARIF 2.1.0 | `--sarif`, the subset GitHub code scanning reads |
| Markdown | `--md`, for issues, pull requests and client reports |
| Baseline | `--baseline`, which reads any earlier JSON report and hides findings already in it |

See [Output](Tuner-Output) and [CI](Tuner-CI).

## Inside Soundcheck

```text
soundcheck/
├── cmd/rhythm-soundcheck   CLI: commands, flags, picker, init, update, output wiring
├── config                  .soundcheck/config.yaml: defaults, strict loading, validation
├── runner                  one run: resolve the site, discover, visit pages in tabs, call module hooks
├── browser                 find, download and launch Chrome; private sessions
├── capture                 one page visit: events, metrics, links, text, screenshots
├── check                   the module interface: hooks and the run context
├── checks/                 the modules
│   ├── health              status, PHP and JS errors, failed requests, links, indexing
│   ├── visual              pixel diff, dynamic regions, bands, grouping
│   ├── interact            inspect, click, submit, flows
│   ├── cache               layers, double fetch, private pages, nonces, staleness, leak test
│   ├── speed               lab metrics, regressions, site-wide suggestions, plugin cost
│   ├── a11y                axe-core (embedded), placeholder labels, reflow at 320 px, overlays
│   ├── keyboard            the Tab walk, widgets, focus rings, traps; runs key flows
│   ├── keyflows            key flows: flows played by keyboard only, dry or send
│   ├── seo                 what search engines read, compared between runs
│   ├── echo                what form submits send, through Echo
│   └── exec                your own programs on every page
├── wcag                    every WCAG 2.0, 2.1 and 2.2 A and AA criterion, EN 301 549 clauses, axe rule table
├── statement               the accessibility statement draft: Markdown, HTML, JSON, EARL
├── evidence                the evidence pack of an --eaa run: hashes, chain, ed25519 signatures
├── internal/focus          presses keys like a visitor and reads where focus goes
├── internal/pii            masks emails, phone, card and IBAN numbers and credentials
├── flow                    the YAML flow format, its runner and fake data
├── login                   WordPress or flow login from SOUNDCHECK_USER/PASSWORD
├── compare                 two runs → ranked changes (BROKE, NEW, CHANGED, SLOWER, FIXED)
├── store                   the .soundcheck folder: snapshots, runs, LATEST, pruning, git exclude
├── report                  terminal view, plain, JSON, Markdown, JUnit, standalone HTML
├── lighthouse              hands a site to Unlighthouse for --lighthouse
└── action.yml              the GitHub Action
```

### A run, step by step

```mermaid
flowchart TD
  R["Resolve the site<br/>(follow redirects)"] --> D["Discover<br/>kit/crawl: robots, sitemaps, start page"]
  D --> L["Launch Chrome<br/>private session"]
  L --> T["Page pool (--tabs, one window each)<br/>paced by kit/ratelimit"]
  subgraph page ["Each page, in its own tab"]
    P["Prepare hooks<br/>(speed: JS/CSS coverage)"] --> V["capture.Visit<br/>desktop load, events, metrics, screenshot"]
    V --> LV["Live hooks<br/>(speed, then interact: inspect, click, submit)"]
    LV --> M["capture.Mobile<br/>reload at 390 px, screenshot"]
    M --> C["CheckPage hooks<br/>(health, visual, speed)"]
  end
  T --> P
  C -- "new links" --> T
  C --> S["CheckSite hooks, once<br/>health: links · cache: double fetch, leak test<br/>speed: suggestions · interact: flows"]
  S --> RUN[("run.json")]
  RUN --> CMP["compare.Compare<br/>issue diff + Comparer hooks"]
  CMP --> REP["report<br/>terminal, HTML, MD, JSON, JUnit"]
```

The runner knows nothing about what modules check. A module is a Go type with a `Name()` and any of these hooks, from [`check/check.go`](https://github.com/RhythmShahriar/soundcheck/blob/master/check/check.go):

| Hook | When it runs | Used by |
|---|---|---|
| `Prepare` | on a fresh tab, before it navigates | speed (starts CSS and JS coverage) |
| `Live` | while the page is open at desktop size, after the screenshot, before the mobile reload; interact and then keyboard run last, since they change the page | speed, a11y, seo, interact, keyboard |
| `CheckPage` | after the visit, on the recorded page, without the browser | health, visual (learns dynamic regions in snapshots), speed |
| `CheckSite` | once, after every page | health (links), cache, speed (suggestions), interact (flows) |
| `Compare` | in a comparison, on both runs | health (titles, content, redirects), visual, cache, speed |
| `OwnsIssues` | marks a module that diffs its own issues | speed, so lab noise doesn't flip issues between NEW and FIXED |

Each issue gets a key from its check, title and source, so `compare` can match the same problem on the same page across runs: a new error is BROKE, a new warning or notice NEW, a missing one FIXED. Comparer hooks add CHANGED and SLOWER. The result is sorted by class and size.

### The browser layer

`browser` drives Chrome through the DevTools protocol with [go-rod](https://github.com/go-rod/rod). It uses `--browser`, or an installed Chrome or Chromium, or downloads a pinned Chromium into `<cache>/rhythm/soundcheck/browser` (progress on stderr). Chrome runs headless with GPU, audio and background networking off and background-tab throttling disabled. It drops its sandbox only when running as root. Each page gets its own window with focus emulated, so pages in parallel paint and animate like the front one. The HTTP cache is off so every page loads cold. The crawl runs in one private (incognito) session; the leak test, each flow and each key flow get their own, so a logged-in user never shares cookies with the anonymous crawl. Every tab is kept in the run's network scope: on a public site no tab loads loopback, private or link-local addresses or cloud metadata names unless `--allow-host` names them. The login is tried once at the start of a run.

`capture` records each visit from DevTools events: the document's status, headers and redirect chain; every request with its type, status, size, protocol and cache headers; console errors, uncaught exceptions and browser log errors; Web Vitals from a `PerformanceObserver` injected before any page script; the visible text, title, robots directives and same-site links. Screenshots pause animations first. The visual module learns dynamic regions from a second capture in snapshots.

Discovery itself fetches only `robots.txt`, sitemaps and the start page (`kit/crawl` with `NoFollow`); further pages come from links in the pages the browser renders. Plain HTTP requests outside the browser (discovery, link checks, form action checks, the cache double fetch) share the run's headers, proxy, TLS settings, user agent and per-host pace.

### Storage and reports

`store` writes each run as `run.json` (schema `rhythm-soundcheck/v1`) with its screenshots, into `.soundcheck/snapshots/<name>/` or `.soundcheck/runs/<date-time>/`. A snapshot is written to `.incoming/` and moved into place only when complete. Runs are pruned to the newest 20, except runs with an evidence pack: each `--eaa` run's `eaa/evidence.json` is chained to the one before it, so those are kept. Folders are private to the user (0700) and files 0600. The folders are added to `.git/info/exclude`. `report` renders one result model as the live terminal view, plain text, JSON, Markdown, JUnit and a standalone HTML page with embedded scaled screenshots and the before/after slider. See [Reading results](Soundcheck-Reading-Results).

## Inside Echo

```text
echo/
├── cmd/rhythm-echo     CLI: commands, flags, up, install, doctor, check, headers, replay, output wiring
├── config              .echo/config.yaml: defaults, strict loading, validation, the init template
├── wire                the capture protocol connectors speak (Capture → Decision)
├── catcher             the running side: inbox, REST API, seal policy, SMTP sink
├── recognize           HTTP request → channel, provider, recipients, content; provider-shaped fakes
├── proxy               HTTP(S) proxy for SDKs that skip the connectors; local CA, MITM
├── sendmail            the sendmail stand-in for PHP's mail() and Unix tools
├── runner              runs actions: trigger, flush, wait for quiet, collect, check
├── check               content checks on messages (placeholders, PHP errors, SMS parts…)
├── compare             two runs → BROKE, NEW, CHANGED, FIXED, with normalization and grouping
├── replay              signed test webhooks and their safety rules; the event library
├── dns, headers        deliverability: SPF, DKIM, DMARC, MX, MTA-STS; received-mail analysis
├── store               the .echo folder: snapshots, runs, LATEST, pruning, git exclude
├── report              terminal view, plain, JSON, Markdown, JUnit, standalone HTML
├── client              Go client for the API (used by Soundcheck)
├── consent             cookie consent checks: plan, results, masking, IDs, the check registry
│   ├── cli             the consent command, its subcommands, flags and help
│   ├── browse          one Chrome per command, a fresh context per run, request-stage holding, events
│   ├── cmp             banner rules (rules/*.yaml), detection, clicking, confirming, the generic rule
│   ├── tracker         the tracker catalogue (catalog/*.yaml with fixtures) and matching
│   ├── signal          Consent Mode commands, gcs, gcd, TCF and GPP decoding, CMP state
│   ├── verdict         the checks, the policies, comparisons with a baseline
│   └── evidence        the evidence folder, manifest, hashes, ed25519 signing and verify
├── internal/browser    Soundcheck's browser layer, copied at v1.2.2 with Echo's launch flags
├── model               messages, actions, issues, changes (schema rhythm-echo/v1)
├── connectors/         wordpress (the mu-plugin, embedded), laravel (Composer), node (npm)
└── action.yml          the GitHub Action
```

### From a capture to a comparison

```mermaid
flowchart TD
  subgraph site ["The site"]
    WP["WordPress mu-plugin<br/>wp_mail, pre_http_request"]
    LV["Laravel package<br/>transport, Http middleware"]
    ND["Node package<br/>fetch, http, Nodemailer"]
    MTA["SMTP · sendmail"]
    SDK["SDKs with their own HTTP"]
  end
  WP & LV & ND -- "POST /v1/capture" --> CAP["catcher.Server.Capture"]
  MTA -- "SMTP sink" --> CAP
  SDK -- "HTTPS proxy, MITM" --> CAP
  CAP --> REC["recognize<br/>channel, provider, content"]
  REC --> POL{"seal policy<br/>allow · fake · block"}
  POL -- "decision" --> WP & LV & ND
  POL --> INBOX[("Inbox<br/>in memory")]
  RUN["runner: actions<br/>X-Echo-Run tag, flush, quiet"] --> INBOX
  SC["Soundcheck --echo<br/>echo/client"] --> INBOX
  INBOX --> CHK["check<br/>content issues"]
  CHK --> STORE[(".echo/<br/>snapshots, runs")]
  STORE --> CMP["compare<br/>identity, normalization, classes"]
  CMP --> REP["report<br/>terminal, HTML, MD, JSON, JUnit"]
```

1. **Capture.** A connector inside the site reports each email and outbound HTTP request through the [capture protocol](Echo-API#the-capture-protocol), before it's sent, with the `X-Echo-Run` it found. The SMTP sink and the proxy turn what they receive into the same captures, so everything downstream treats them alike.
2. **Recognize.** `recognize` tries each provider's recognizer in turn: Twilio, Slack, SendGrid, GA4, Stripe and the rest. The first match fills the channel, provider, recipients, content and event; anything else is a generic webhook or request. Secrets are redacted with `kit/secrets` and bodies capped before the message is stored.
3. **Decide.** The seal policy answers: platform update hosts pass unrecorded; observe mode, the site's own host, loopback and the allowlist are allowed; live payment calls are blocked with the provider's own error; everything else gets the recognizer's fake. The connector does what it's told.
4. **Collect.** The inbox keeps messages in memory and wakes anything waiting on it. The runner opens an action (`/v1/actions/begin`), triggers it with its run ID, runs the flush commands, then waits until no new message has matched for the quiet time and closes the action. Soundcheck does the same around each form submit, through `echo/client`.
5. **Check and store.** `check` runs the content checks on each action's messages. `store` writes the run as `run.json` into `.echo/snapshots/<name>/` (through `.incoming/`, moved into place when complete) or `.echo/runs/<date-time>/`, pruned to the newest 20.
6. **Compare.** `compare` matches actions by ID and messages by identity (channel and recipients for email and SMS; channel, provider, method, host, path pattern and event for the rest). It normalizes test data, numbers and dates and diffs recipients, subject, text and payload fields. Issues are diffed by key as in Soundcheck. The same change across actions is one line. `report` renders the result. Soundcheck's `echo` module calls `compare.Messages` directly for its forms and flows.

The WordPress connector is embedded in the binary (`connectors/wordpress/echo.php`) and written by `install`. The Laravel and Node connectors are packages of their own, released with Echo; see [Release process](Release-Process#echos-packages).

### From a URL to a consent verdict

```mermaid
flowchart LR
  U["rhythm echo consent &lt;url&gt;"] --> P["plan<br/>flags + consent: config"]
  P --> B["browse<br/>one Chrome, fresh context per run<br/>none · accept · reject"]
  B -- "hits held at request stage" --> E[("events/*.ndjson")]
  B --> C["cmp<br/>find, click, confirm"]
  B --> S["signal<br/>Consent Mode, gcs, gcd, TCF"]
  E --> V["verdict<br/>checks, policy, baseline"]
  C --> V
  S --> V
  V --> R["report + evidence<br/>consent.json, manifest, report.html"]
```

Each run gets its own incognito context and its own egress guard proxy; Chrome's own traffic goes to a listener that refuses everything. Each page, frame and worker has one Fetch owner at request stage, which answers tracker hits itself (held) and lets everything else through. Requests and clicks share one clock, the run's first navigation, so "before the choice" is exact to the millisecond. The verdict reads only what the network, cookies and storage show; `dataLayer` and CMP globals explain it. Echo doesn't import Soundcheck: `internal/browser` is a copy pinned to Soundcheck v1.2.2, with a test that keeps the Chrome for Testing pin in step.

---
<sub>← [Release process](Release-Process) · [Home](Home)</sub>
