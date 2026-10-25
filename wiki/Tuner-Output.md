Tuner shows a live report in your terminal and can write the same results as JSON, JSON Lines, SARIF, Markdown or a self-contained HTML page. This page covers the terminal modes, each machine format, writing to a file with `-o`, the fields in the JSON report and turning a report into server config with `rhythm tuner fix`.

## At a glance

| Flag | Output |
|---|---|
| none | live terminal view |
| `--json` | one JSON report object; an array when several sites are reported |
| `--json-array` | JSON that is always an array, also for one site |
| `--jsonl` | one JSON finding per line, streamed as found |
| `--sarif` | SARIF 2.1.0 for GitHub code scanning |
| `--md` | Markdown for issues, pull requests and clients |
| `-o, --output <file>` | write the report to a file (format from the extension, `.html` included) and keep the live view |
| `--html <file>` | also write a self-contained HTML report |
| `--plain` | no color, boxes or glyphs; automatic when output isn't a terminal |
| `--no-color` | no color (`NO_COLOR` also works) |
| `-q, --quiet` | findings and a one-line summary only |
| `-v, --verbose` | log every request to stderr |

Secrets Tuner recognizes are masked in every format, JSON included. There's no option to unmask them. A value Tuner doesn't recognize as a secret can still appear in evidence, so treat reports as private. Text that came from a site or a file is cleaned in every format: control characters show as `�`, bidirectional overrides are dropped and Markdown and HTML are escaped, so a hostile page or file name can't rewrite your terminal or your report.

## Terminal modes

### Pretty (default)

In an interactive terminal you get the full view: a header, the fingerprint and baseline steps, a live progress line while the scan runs, then findings grouped by severity, the Tune score panel and stats. It follows the terminal's width. [Reading results](Tuner-Reading-Results) explains each part.

The progress line appears only when you scan a single site and don't use `-v`.

### Plain

`--plain` drops color, box drawing and decorative glyphs. Findings stream one per line as they're found, each with its fix, then a **Findings by severity** block lists them again, most severe first, followed by a one-line summary:

```text
Score 0/100 - 4 critical, 2 high, 1 medium, 7 low, 1 info - 77 checks, 180 requests, 3.6s
```

Plain mode turns on **automatically when standard output isn't a terminal**: piped to another command, redirected to a file, or in most CI logs. Use it yourself for screen readers. See [Plain mode](Tuner-Reading-Results#plain-mode) for the full format.

### No color

`--no-color`, or the `NO_COLOR` environment variable set to any value, keeps the pretty layout without color or bold:

```sh
NO_COLOR=1 rhythm tuner scan yoursite.com
```

### Quiet

`-q` prints the findings and the one-line summary, with any `Incomplete:` notes above it. It leaves out the header, steps, score panel, warnings and `Report written` messages. In a terminal the findings are still grouped as cards; in plain mode they're the one-line format. A few lines always print, because you need them even in quiet mode: which `.tuner.yml` a scan used, expired ignore entries and check pack alerts.

### Verbose

`-v` logs every request to stderr as it completes, with its status, its time and `(attempt N)` on retries:

```text
GET 200 https://yoursite.com 37ms
GET 200 https://yoursite.com/wp-content/plugins/contact-form-7/readme.txt 1ms
GET 404 https://yoursite.com/.env 2ms
```

Failed requests show `ERR` and the error instead of a status. Verbose mode turns off the progress line. It's the first thing to try when a site blocks or throttles the scan; see [Requests and politeness](Tuner-Requests).

## Machine formats

Pick one of `--json`, `--json-array`, `--jsonl`, `--sarif` or `--md`. On its own, the format goes to standard output **in place of** the live view, ready to redirect or pipe:

```sh
rhythm tuner scan yoursite.com --json > report.json
rhythm tuner scan yoursite.com --json | jq '.score'
```

Warnings, errors and the one-time notice always go to stderr, so they never corrupt the report. Only one format per run; two is a usage error:

```text
$ rhythm tuner scan 127.0.0.1:20400 --json --md
rhythm tuner scan: pick one output format, not --json and --md
```

### JSON

`--json` writes one report object for one site and an array for several. Sites that couldn't be scanned are entries of their own at the end of the array, `{"target": …, "error": …}`, with the reason alone (the hint stays on stderr). `--json-array` gives an array even for one site, so a script never has to check which shape it got:

```text
$ rhythm tuner scan 127.0.0.1:20400 127.0.0.1:20499 --json-array -q
rhythm tuner scan: 127.0.0.1:20499: can't reach 127.0.0.1:20499: Get "http://127.0.0.1:20499": dial tcp 127.0.0.1:20499: connect: connection refused
  check the address and that the site is up
[
  {
    "schema": "rhythm-tuner/v0",
    …
  },
  {
    "target": "127.0.0.1:20499",
    "error": "can't reach 127.0.0.1:20499: Get \"http://127.0.0.1:20499\": dial tcp 127.0.0.1:20499: connect: connection refused"
  }
]
```

The layout is published as a JSON Schema, [`schema/report-v0.json`](https://github.com/RhythmShahriar/tuner/blob/master/schema/report-v0.json). New optional fields can appear in any release, so let validators allow unknown properties.

### JSON Lines

`--jsonl` writes each finding as one line of JSON **as soon as it's found**, so you can follow a long batch scan or feed a log pipeline:

```sh
rhythm tuner scan -l sites.txt --jsonl | jq -r 'select(.severity == "critical") | .url'
```

```json
{"id":"tls-no-https","title":"Site served over plain HTTP","severity":"medium","confidence":"high","target":"http://127.0.0.1:20400","url":"http://127.0.0.1:20400","path":"/","evidence":{"request":"GET /","status":200},"impact":"Logins and cookies travel unencrypted. The site doesn't redirect to HTTPS.","fix":"Get a free certificate (e.g. certbot) and redirect all HTTP traffic.\nnginx:  return 301 https://$host$request_uri;","references":["https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Security_Cheat_Sheet.html","https://certbot.eff.org/"],"tags":["tls","transport"],"fingerprint":"v1:4d3d6734891797087e82dcc900e179a1"}
```

Each line is a [finding](#finding-fields) with its `target`, so lines from several sites can be told apart. JSON Lines has findings only: no fingerprint, score or summary. Findings hidden by a baseline or ignore entries are left out. Use `.jsonl` or `.ndjson` with `-o`.

### SARIF

`--sarif` writes SARIF 2.1.0, the format GitHub code scanning reads. Each finding becomes a result whose rule is the check ID, with a severity GitHub understands; several sites become one SARIF log with a run per site.

```sh
rhythm tuner scan https://staging.yoursite.com --sarif -o tuner.sarif
```

- **Stable alerts.** Every result has `partialFingerprints` (`tunerFinding/v1`, the same value as the JSON `fingerprint`), so code scanning keeps one alert per finding across runs, hosts and checkout folders.
- **Local scans** give paths relative to the scanned folder, with `uriBaseId: SRCROOT` and the folder in `originalUriBaseIds`, plus the line for `wp-config.php` and `.env` settings:

  ```json
  "artifactLocation": { "uri": "wp-config.php", "uriBaseId": "SRCROOT" },
  "region": { "startLine": 5 }
  ```
- **Rule help** links to the finding's entry in the [Finding reference](Tuner-Finding-Reference), e.g. `…/Tuner-Finding-Reference#leak-dotenv`.
- **Partial and failed scans.** A run says `executionSuccessful: false` with a notification per note when the scan was partial. A site that couldn't be scanned gets a run of its own with no results and an error notification.

See [CI and code scanning](Tuner-CI) for the upload workflow.

### Markdown

`--md` writes a report for people: a title and summary line with the score, a collapsible table of plugins and themes with their versions and where each version came from, a table of findings and a details section per finding with the check ID, severity, confidence, request and status, evidence, masked secrets, impact, an advisory table (CVE, fixed version, actively exploited), the full fix and references. A partial scan starts with its notes; sites that couldn't be scanned end up in a `# Not scanned` table.

```text
$ rhythm tuner report rep/after.json --md
# Tuner report: http://127.0.0.1:20400

**Tune score 0/100** · WordPress 6.9.3 · PHP 8.1.2 \(from x-powered-by /\) · nginx · 77 checks · 180 requests · rhythm-tuner 0.4.0

<details><summary>Plugins and themes</summary>
…
```

It pastes cleanly into GitHub issues and pull requests: everything that came from the site is escaped, links render only for http and https URLs and an `@name` from a site can't mention anyone.

### HTML

`-o client.html` or `--html client.html` writes one self-contained page for a client: no scripts or remote assets, light and dark styles and a print layout. `--brand-name` and `--brand-logo` put your agency on it. With `--since` it marks what's new and what was fixed. `rhythm tuner report` renders saved JSON reports the same way. See [A report for a client](Tuner-Changes-And-Notifications#a-report-for-a-client---html-and-report).

## Writing to a file: -o

`-o <file>` writes the report to a file and **keeps the live view** on your terminal, so you can watch the scan and save it at once:

```sh
rhythm tuner scan yoursite.com -o report.md
```

The format comes from the extension:

| Extension | Format |
|---|---|
| `.json` | JSON |
| `.jsonl`, `.ndjson` | JSON Lines |
| `.sarif` | SARIF |
| `.md`, `.markdown` | Markdown |
| `.html`, `.htm` | HTML |

For any other extension, add the format flag: `--json -o results.txt`. Without one, Tuner stops before scanning:

```text
$ rhythm tuner scan 127.0.0.1:20400 -o r.txt
rhythm tuner scan: can't tell the format of r.txt; add --json, --jsonl, --sarif or --md, or end the name in .html
```

**The previous report is safe.** Tuner writes to a temporary file in the same folder and renames it into place only once the report is complete, so the folder must be writable. If every target failed or you press Ctrl+C, the old file stays: `no report written to out/r.json: every target failed; it keeps the previous report`. A new file gets the usual permissions for your umask; a replaced one keeps its own. When the file is written, Tuner prints `Report written to <file>` on stderr (not with `-q`).

> [!TIP]
> Save a JSON report alongside the Markdown one. JSON is what `--baseline`, `--since`, `diff`, `report` and `fix` read. See [Changes and notifications](Tuner-Changes-And-Notifications).

## Server config from a report

`rhythm tuner fix` reads JSON reports and prints one nginx or Apache config bundle with the snippets that fix their findings, each snippet once, commented with the checks and URLs it fixes. It changes nothing: review it, add it to your server config and test it.

```text
$ rhythm tuner fix rep/before.json --server nginx
# Tuner fix bundle for nginx, from rhythm-tuner 0.4.0.
# 8 snippets for 8 findings. Review every block before you use it.
# Put it inside the site's server { } block, above any location ~ \.php$
# block. A location with its own add_header lines drops these headers, so
# repeat them there. Then run: nginx -t

# hdr-content-type-options-missing
#   http://127.0.0.1:20400
add_header X-Content-Type-Options nosniff always;

# hdr-csp-missing
#   http://127.0.0.1:20400
add_header Content-Security-Policy-Report-Only "default-src 'self'" always;
…
# leak-dotenv
#   http://127.0.0.1:20400/.env
location ~ /\.(?!well-known) { deny all; }

# wp-xmlrpc
#   http://127.0.0.1:20400/xmlrpc.php
location = /xmlrpc.php { deny all; }

# No nginx snippet for: php-eol, tls-no-https, vuln-composer-guzzlehttp-guzzle, vuln-core, vuln-php, vuln-plugin-contact-form-7, wp-components-outdated
# See: rhythm tuner explain <id>
```

`--dry-run` lists what the bundle would hold:

```text
$ rhythm tuner fix rep/before.json --server apache --dry-run
apache bundle: 8 snippets for 8 findings
  hdr-content-type-options-missing (1 URL)
  hdr-csp-missing (1 URL)
  hdr-frame-options-missing (1 URL)
  hdr-powered-by-version (1 URL)
  hdr-referrer-policy-missing (1 URL)
  leak-composer-manifest (1 URL)
  leak-dotenv (1 URL)
  wp-xmlrpc (1 URL)
no apache snippet: php-eol, tls-no-https, vuln-composer-guzzlehttp-guzzle, vuln-core, vuln-php, vuln-plugin-contact-form-7, wp-components-outdated
```

Several reports (a batch, or one per site) make one bundle. Snippets for Tuner's own checks come from the running version, not the report, so reports from older releases work too; a snippet taken from a report for a check Tuner doesn't have is marked as such. Every built-in snippet is tested to pass `nginx -t` inside a `server { }` block and `apachectl -t` inside a `<VirtualHost>`. Findings fixed by updating software or by application settings (vulnerable plugins, cookie flags, TLS certificates) have no snippet; the bundle lists them at the end. `-` reads a report from stdin.

## The JSON report

The JSON report is the most complete output. Its layout is identified by `schema`, currently `rhythm-tuner/v0`. The report for the test site, with its findings left out and two of its three components:

```json
{
  "schema": "rhythm-tuner/v0",
  "tool": "rhythm-tuner",
  "version": "0.4.0",
  "duration_ms": 3580,
  "summary": {
    "critical": 4,
    "high": 2,
    "info": 1,
    "low": 7,
    "medium": 1
  },
  "check_set": "built-in",
  "baselined": 0,
  "complete": true,
  "vuln_data": "ok",
  "target": "http://127.0.0.1:20400",
  "fingerprint": {
    "tech": [
      "wordpress",
      "php"
    ],
    "wordpress_version": "6.9.3",
    "php_version": "8.1.2",
    "php_source": "x-powered-by /",
    "server": "nginx",
    "components": [
      {
        "type": "plugin",
        "slug": "contact-form-7",
        "version": "5.3.1",
        "version_source": "readme",
        "found_by": [
          "asset",
          "rest"
        ],
        "confidence": "high"
      },
      {
        "type": "theme",
        "slug": "astra",
        "version": "4.6.0",
        "version_source": "style.css",
        "found_by": [
          "asset"
        ],
        "confidence": "high"
      }
    ]
  },
  "soft_404": false,
  "score": 0,
  "checks": 77,
  "requests": 180,
  "errors": 0,
  "throttled": 0
}
```

### Report fields

| Field | Meaning |
|---|---|
| `schema` | the report layout, `rhythm-tuner/v0` |
| `tool`, `version` | `rhythm-tuner` and its version |
| `duration_ms` | how long the scan took |
| `summary` | number of findings per severity, always all five |
| `check_set` | where the checks came from: `built-in`, `pack` or `directory` (`--checks`) |
| `check_pack` | the check pack's version, when `check_set` is `pack` |
| `baselined` | how many findings `--baseline` hid (ignore entries aren't counted here) |
| `complete` | `false` when the scan didn't finish |
| `stopped_by` | why it stopped early: `interrupted`, `max-time`, `waf`, `home-status`, `budget` or `integrity` |
| `blocked_by` | the firewall behind a `waf` stop, e.g. `cloudflare` |
| `vuln_data` | `ok`, `partial` (some lookups failed or used old cache), `missing` (none worked) or `off` (`--no-vulns`) |
| `selection` | for a narrowed scan (`--id`, `--tags`, `--severity`, `--profile quick`…), what it left out, so a comparison counts a finding as fixed only when its check ran |
| `target` | the scanned URL after redirects, or the absolute folder for a local scan |
| `fingerprint` | what Tuner detected (below) |
| `soft_404` | `true` when the server answers `200` for missing pages |
| `soft_404_unknown` | `true` when the probe hit a block page or the home page failed, so `soft_404` says nothing |
| `tls` | for https sites: `version` negotiated, certificate `not_after`, `verify_error` if it failed, `legacy` protocol versions still accepted |
| `score` | the [Tune score](Tuner-Reading-Results#the-tune-score), 0 to 100 |
| `checks`, `requests` | checks that applied, requests sent |
| `errors` | requests that failed after retries |
| `throttled` | `429`/`503` replies that made Tuner slow down |
| `findings` | every finding, most severe first |
| `warnings` | notes such as stale cached data or a missing `php` binary (left out when empty) |

### Fingerprint fields

| Field | Meaning |
|---|---|
| `tech` | detected platforms: `wordpress`, `laravel`, `php` |
| `wordpress_version` | WordPress version, when found |
| `php_version` | PHP version, when found |
| `php_build` | the full build string when it has a distribution suffix, e.g. `7.4.3-4ubuntu2.19` |
| `php_source` | where the PHP version came from: a header and path such as `x-powered-by /wp-login.php`, `php on PATH` or `--php <path>` |
| `server` | the `Server` header |
| `components` | plugins and themes: `type`, `slug`, `version`, `version_source` (`readme`, `style.css` or `asset` remotely; `plugin header` or `style.css` locally) and, for remote scans, `found_by` (`asset`, `rest`, `readme`) and `confidence` |

### Finding fields

| Field | Meaning |
|---|---|
| `id` | the check or finding ID, e.g. `leak-dotenv`, `vuln-plugin-contact-form-7` |
| `title` | one-line description; it names the masked secrets a check found, e.g. `.env file exposed: 1 secret (DB_PASSWORD=Su•••••)` |
| `severity` | `critical`, `high`, `medium`, `low` or `info` |
| `confidence` | `high`, or `low` for [likely findings](Tuner-Reading-Results#confidence-and-likely-findings) |
| `caveat` | why confidence is low, e.g. `a distro build may already include the fix` |
| `target`, `url`, `path` | the site, the full URL (or `file://` path) and the path that triggered it |
| `line` | for local `wp-config.php` and `.env` settings, the line it's on |
| `evidence` | `request` (e.g. `GET /.env`, or `version match`), `status`, `matched` (lines that matched) and `secrets` (each with `kind`, `name` and a `masked` value) |
| `impact` | why it matters |
| `fix` | how to fix it, often with nginx, Apache or WordPress variants on separate lines |
| `fixes` | the check's server config snippets, keyed `nginx` and `apache`, as `rhythm tuner fix` uses them |
| `references` | links for more detail |
| `tags` | the check's tags; `known-exploited` marks vulnerabilities in CISA KEV |
| `advisories` | for vulnerability findings: each advisory's `id`, `title`, `cves`, `severity`, `cvss`, `kev`, `fixed_in` and `references` |
| `source` | the vulnerability data provider, for attribution |
| `fingerprint` | `v1:` and a hash of the check ID and location, stable across hosts and folders; baselines, `--since`, `diff` and SARIF match on it |

Fields with no value are left out of the JSON.

```json
{
  "id": "leak-dotenv",
  "title": ".env file exposed: 1 secret (DB_PASSWORD=Su•••••)",
  "severity": "critical",
  "confidence": "high",
  "target": "http://127.0.0.1:20400",
  "url": "http://127.0.0.1:20400/.env",
  "path": "/.env",
  "evidence": {
    "request": "GET /.env",
    "status": 200,
    "matched": [
      "APP_ENV="
    ],
    "secrets": [
      {
        "kind": "env",
        "name": "DB_PASSWORD",
        "masked": "Su•••••"
      }
    ]
  },
  "impact": "Anyone can read database passwords, API keys and app secrets.",
  "fix": "Deny access to dotfiles, then rotate every secret in the file.\nnginx:  location ~ /\\.(?!well-known) { deny all; }\napache: <FilesMatch \"^\\.\"> Require all denied </FilesMatch>",
  "references": [
    "https://owasp.org/Top10/A05_2021-Security_Misconfiguration/"
  ],
  "tags": [
    "leak",
    "secrets",
    "php",
    "laravel",
    "wordpress"
  ],
  "fixes": { … },
  "fingerprint": "v1:efd3d5b510718fdc28d6b705068c79c4"
}
```

The `.env` evidence holds variable names only (`APP_ENV=`), never their values; values Tuner recognizes as secrets are listed masked under `secrets`.

---
<sub>← [Requests and politeness](Tuner-Requests) · [Home](Home) · [Batch scans](Tuner-Batch-Scans) →</sub>
