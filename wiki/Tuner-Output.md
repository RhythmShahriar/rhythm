Tuner shows a live report in your terminal and can write the same results as JSON, JSON Lines, SARIF or Markdown. This page covers the terminal modes, each machine format, writing to a file with `-o`, and the fields in the JSON report.

## At a glance

| Flag | Output |
|---|---|
| none | live terminal view |
| `--json` | one JSON report object; an array when several sites are reported |
| `--jsonl` | one JSON finding per line, streamed as found |
| `--sarif` | SARIF 2.1.0 for GitHub code scanning |
| `--md` | Markdown for issues, pull requests and clients |
| `-o, --output <file>` | write the report to a file (format from the extension) and keep the live view |
| `--plain` | no color, boxes or glyphs; automatic when output isn't a terminal |
| `--no-color` | no color (`NO_COLOR` also works) |
| `-q, --quiet` | findings only |
| `-v, --verbose` | log every request to stderr |

Secrets are masked in every format, including JSON. There's no option to unmask them.

## Terminal modes

### Pretty (default)

In an interactive terminal you get the full view: a header, the fingerprint and baseline steps, a live progress line while the scan runs, then findings grouped by severity, the Tune score panel and stats. [Reading results](Tuner-Reading-Results) explains each part.

The progress line appears only when you scan a single site and don't use `-v`.

### Plain

`--plain` drops color, box drawing and decorative glyphs. Findings stream one per line as they're found (`[HIGH] title - URL`, or `[HIGH, likely] title - URL` with the reason below for a low-confidence finding), followed by a one-line summary:

```text
Score 0/100 - 5 critical, 3 high, 2 medium, 6 low, 3 info - 37 checks, 78 requests, 15.4s
```

Plain mode turns on **automatically when standard output isn't a terminal**: piped to another command, redirected to a file, or in most CI logs. Use it yourself for screen readers. See [Plain mode](Tuner-Reading-Results#plain-mode) for the full format.

### No color

`--no-color`, or the `NO_COLOR` environment variable set to any value, keeps the pretty layout without color or bold:

```sh
NO_COLOR=1 rhythm tuner scan yoursite.com
```

### Quiet

`-q` prints findings only: no header, steps, score panel, stats, warnings or `Report written` message. In a terminal the findings are still grouped as cards; in plain mode they're the one-line format.

### Verbose

`-v` logs every request to stderr as it completes, with its status and time, and `(attempt N)` on retries:

```text
GET 200 https://yoursite.com 37ms
GET 200 https://yoursite.com/wp-content/plugins/contact-form-7/readme.txt 1ms
GET 404 https://yoursite.com/.env 2ms
```

Failed requests show `ERR` and the error instead of a status. Verbose mode turns off the progress line. It's the first thing to try when a site blocks or throttles the scan; see [Requests and politeness](Tuner-Requests).

## Machine formats

Pick one of `--json`, `--jsonl`, `--sarif` or `--md`. On its own, the format goes to standard output **in place of** the live view, ready to redirect or pipe:

```sh
rhythm tuner scan yoursite.com --json > report.json
rhythm tuner scan yoursite.com --json | jq '.score'
```

Warnings, errors and the one-time notice always go to stderr, so they never corrupt the report. Only one format per run; two is a usage error:

```text
rhythm tuner: pick one output format, not --json and --md
```

### JSON

`--json` writes one report object per site. Scanning one site gives an object; scanning several gives an array, even if only some of them could be scanned. A site that couldn't be scanned isn't in the array (its error goes to stderr). Before v0.3.2, a batch where only one site succeeded gave a single object.

### JSON Lines

`--jsonl` writes each finding as one line of JSON **as soon as it's found**, so you can follow a long batch scan or feed a log pipeline:

```sh
rhythm tuner scan -l sites.txt --jsonl | jq -r 'select(.severity == "critical") | .url'
```

```json
{"id":"wp-xmlrpc","title":"XML-RPC enabled","severity":"low","confidence":"high","target":"https://yoursite.com","url":"https://yoursite.com/xmlrpc.php","path":"/xmlrpc.php","evidence":{"request":"GET /xmlrpc.php","status":405,"matched":["XML-RPC server accepts POST requests only"]},"impact":"XML-RPC allows many password guesses in a single request and is often used for pingback abuse.","fix":"Disable it unless Jetpack or a mobile app needs it.\nnginx:  location = /xmlrpc.php { deny all; }\nwp:     add_filter( 'xmlrpc_enabled', '__return_false' );","tags":["wordpress","bruteforce"]}
```

Each line is a [finding](#finding-fields) with its `target`, so lines from several sites can be told apart. JSON Lines has findings only: no fingerprint, score or summary. Findings in a `--baseline` file are left out. Use `.jsonl` or `.ndjson` with `-o`.

### SARIF

`--sarif` writes SARIF 2.1.0, the format GitHub code scanning reads. Each finding becomes a result whose rule is the check ID, with a severity GitHub understands; several sites become one SARIF log with a run per site. See [CI and code scanning](Tuner-CI) for the upload workflow.

```sh
rhythm tuner scan https://staging.yoursite.com --sarif -o tuner.sarif
```

### Markdown

`--md` writes a report for people: a title and summary line with the score, a collapsible table of plugins and themes with their versions and where each version came from, a table of findings, and a details section per finding with the check ID, severity, confidence, request and status, evidence, masked secrets, impact, an advisory table (CVE, fixed version, actively exploited), the full fix and references. Several sites are separated by horizontal rules.

It pastes cleanly into GitHub issues and pull requests, and reads well as a client report.

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

For any other extension, add the format flag: `--json -o results.txt`. Without one, Tuner stops with `can't tell the format of results.txt; add --json, --jsonl, --sarif or --md`. When the file is written, Tuner prints `Report written to <file>` on stderr (not with `-q`).

> [!TIP]
> Save a JSON report alongside the Markdown one. JSON is what `--baseline` reads, so it lets you hide accepted findings on the next run. See [CI and code scanning](Tuner-CI).

## The JSON report

The JSON report is the most complete output. Its layout is identified by `schema`, currently `rhythm-tuner/v0`; it may change before v1.0.

```json
{
  "schema": "rhythm-tuner/v0",
  "tool": "rhythm-tuner",
  "version": "0.3.1",
  "duration_ms": 15412,
  "summary": { "critical": 5, "high": 3, "medium": 2, "low": 6, "info": 3 },
  "check_set": "built-in",
  "target": "https://yoursite.com",
  "fingerprint": {
    "tech": ["wordpress", "php"],
    "wordpress_version": "6.4.3",
    "php_version": "8.1.27",
    "php_source": "x-powered-by /",
    "server": "Apache/2.4.57 (Debian)",
    "components": [
      { "type": "theme", "slug": "astra", "version": "4.6.0", "version_source": "style.css" },
      { "type": "plugin", "slug": "contact-form-7", "version": "5.3.1", "version_source": "readme" }
    ]
  },
  "soft_404": false,
  "score": 0,
  "checks": 37,
  "requests": 78,
  "errors": 0,
  "throttled": 0,
  "findings": [ … ]
}
```

### Report fields

| Field | Meaning |
|---|---|
| `schema` | the report layout, `rhythm-tuner/v0` |
| `tool`, `version` | `rhythm-tuner` and its version |
| `duration_ms` | how long the scan took |
| `summary` | number of findings per severity (severities with none are left out) |
| `check_set` | where the checks came from: `built-in`, `pack` or `directory` (`--checks`) |
| `check_pack` | the check pack's version, when `check_set` is `pack` |
| `target` | the scanned URL after redirects, or the absolute folder for a local scan |
| `fingerprint` | what Tuner detected (below) |
| `soft_404` | `true` when the server answers `200` for missing pages |
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
| `php_source` | the header and path the PHP version came from, e.g. `x-powered-by /wp-login.php` |
| `server` | the `Server` header |
| `components` | plugins and themes: `type`, `slug`, `version`, and `version_source` (`readme`, `style.css` or `asset` remotely; `plugin header` or `style.css` locally) |

### Finding fields

| Field | Meaning |
|---|---|
| `id` | the check or finding ID, e.g. `leak-dotenv`, `vuln-plugin-contact-form-7` |
| `title` | one-line description |
| `severity` | `critical`, `high`, `medium`, `low` or `info` |
| `confidence` | `high`, or `low` for [likely findings](Tuner-Reading-Results#confidence-and-likely-findings) |
| `caveat` | why confidence is low, e.g. `a distro build may already include the fix` |
| `target`, `url`, `path` | the site, the full URL (or `file://` path) and the path that triggered it |
| `evidence` | `request` (e.g. `GET /.env`, or `version match`), `status`, `matched` (lines that matched), and `secrets` (each with `kind`, `name` and a `masked` value) |
| `impact` | why it matters |
| `fix` | how to fix it, often with nginx, Apache or WordPress variants on separate lines |
| `references` | links for more detail |
| `tags` | the check's tags; `known-exploited` marks vulnerabilities in CISA KEV |
| `advisories` | for vulnerability findings: each advisory's `id`, `title`, `cves`, `severity`, `cvss`, `kev`, `fixed_in` and `references` |
| `source` | the vulnerability data provider, for attribution |

Fields with no value are left out of the JSON.

```json
{
  "id": "leak-dotenv",
  "title": ".env file exposed",
  "severity": "critical",
  "confidence": "high",
  "target": "https://yoursite.com",
  "url": "https://yoursite.com/.env",
  "path": "/.env",
  "evidence": {
    "request": "GET /.env",
    "status": 200,
    "matched": ["APP_ENV=production"],
    "secrets": [
      { "kind": "env", "name": "DB_PASSWORD", "masked": "Su•••••" },
      { "kind": "stripe-key", "masked": "sk_live_•••••" }
    ]
  },
  "impact": "Anyone can read database passwords, API keys and app secrets.",
  "fix": "Deny access to dotfiles, then rotate every secret in the file.\nnginx:  location ~ /\\.(?!well-known) { deny all; }\napache: <FilesMatch \"^\\.\"> Require all denied </FilesMatch>",
  "references": ["https://owasp.org/Top10/A05_2021-Security_Misconfiguration/"],
  "tags": ["leak", "secrets", "php", "laravel", "wordpress"]
}
```

---
<sub>← [Requests and politeness](Tuner-Requests) · [Home](Home) · [Batch scans](Tuner-Batch-Scans) →</sub>
