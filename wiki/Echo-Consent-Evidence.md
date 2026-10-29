Every `rhythm echo consent` result is an evidence folder: the result as JSON, every event of every run, screenshots, the reports and the SHA-256 of each file. This page covers the folder's layout, the JSON, the other report formats, masking, `consent verify`, signing, `consent report` and `consent prune`. Evidence is what a browser did on these pages in these runs, not a legal compliance guarantee.

## The folder

```text
.echo/consent/<date-time>-<id>/           0700, files 0600
├── consent.json                          the result: the evidence of record
├── consent.json.sig                      with --sign-key
├── manifest.json                         path, bytes and SHA-256 of every file
├── report.html                           always
├── report.md  report.xml                 when -o or a format flag asks for them
├── events/page-1-none.ndjson             every event of every run, one JSON object per line
├── events/page-1-accept.ndjson, page-1-reject.ndjson
├── evidence/page-1/none-banner.png       viewport screenshots: the banner before the choice
├── evidence/page-1/none-after.png        and the page after it, per run
├── evidence/page-1/accept-banner.png, accept-after.png, reject-banner.png, reject-after.png
└── har/page-1-reject.har …               with --har: masked, no bodies
.echo/consent/baselines/<name>/consent.json
.echo/consent/detect/<id>.png
```

`<date-time>` is when the check started, in UTC with a `Z` (`2026-10-28T20-43-14Z-b879a6`), so it reads the same as every time in the reports and the `--against` header. `<id>` is six random hex digits.

The folder is next to `.echo/config.yaml` when there is one, else in `./.echo`; `--store <dir>` puts it elsewhere. A check that stops before any run (a bad selector, a site that doesn't answer) leaves no empty folder behind. `-o <file>` writes the report where you say and keeps a copy in the folder. `--no-screenshots` takes none.

> [!NOTE]
> Echo adds `.echo/snapshots/` and `.echo/runs/` to `.git/info/exclude`. In 1.2.0 it doesn't add `.echo/consent/`. The folder holds screenshots of the site and masked URLs: add `/.echo/consent/` to `.gitignore` unless you mean to commit the evidence (a baseline you keep in the repository, say).

## Hashes

`consent.json` lists the SHA-256 of every screenshot, events file and HAR, written before it. Each report states the SHA-256 of `consent.json`. `manifest.json` (`rhythm-echo-consent-manifest/v1`) covers every file, `consent.json` included:

```json
{
  "schema": "rhythm-echo-consent-manifest/v1",
  "files": [
    {
      "path": "consent.json",
      "bytes": 49648,
      "sha256": "112404a184658cd9f9d3aaaa31a15be1a0169056f4588a65b27687adfad37643"
    },
    {
      "path": "events/page-1-accept.ndjson",
      "bytes": 21963,
      "sha256": "314bda0da923108079317b96e323bd8c4662ee248a5918ce21552f461bc47f41"
    },
    ...
```

## consent verify

`rhythm echo consent verify <folder | consent.json>` recomputes every hash and checks the signature when you give the public key:

```text
$ rhythm echo consent verify .echo/consent/<id> --key team.pub
ok  consent.json                       sha256 5f9e76de...7c26
ok  consent.json.sig                   sha256 6bb8069e...650f
ok  events/page-1-accept.ndjson        sha256 7dabfe83...b999
ok  events/page-1-none.ndjson          sha256 5ee2b7d2...2d53
ok  events/page-1-reject.ndjson        sha256 f0b37664...8133
ok  evidence/page-1/accept-after.png   sha256 15a0a3d5...96be
ok  evidence/page-1/accept-banner.png  sha256 78ccd3b1...d25e
ok  evidence/page-1/none-after.png     sha256 78ccd3b1...d25e
ok  evidence/page-1/none-banner.png    sha256 78ccd3b1...d25e
ok  evidence/page-1/reject-after.png   sha256 15a0a3d5...96be
ok  evidence/page-1/reject-banner.png  sha256 78ccd3b1...d25e
ok  report.html                        sha256 604c3458...9d1c
Signature: good (ed25519, key team.pub)
Verified: 12 files match, signed by team.pub.
The signature covers consent.json and the files it lists; report.html is checked against manifest.json, which is not signed.
```

The same folder after one line was added to an events file and `report.html` was deleted (`-q` leaves out the `ok` lines):

```text
$ rhythm echo consent verify tampered -q
changed  events/page-1-reject.ndjson  want f0b37664... got afc20fa7...
missing  report.html
Signature: present, no --key to check it
Not verified: 1 changed, 1 missing; 10 files match.
```

| Exit | Means |
|---|---|
| `0` | every file and the signature match |
| `1` | a hash or the signature doesn't match; a `--key` was given and the folder has no signature (`Signature: missing (--key team.pub given)`) |
| `2` | usage error |
| `3` | a file is missing or unreadable |

A file in the folder that neither `manifest.json` nor `consent.json` lists is named (`not in the manifest  <path>`) without changing the exit code. `--json` gives the result as the `consent-verify` kind.

## Signing

`--sign-key <file>` signs `consent.json` with an ed25519 private key in PEM form, writing `consent.json.sig`. The key file must be readable by you only (mode `0600`).

```sh
openssl genpkey -algorithm ed25519 -out team.key && chmod 600 team.key
openssl pkey -in team.key -pubout -out team.pub
rhythm echo consent https://staging.shop.test --sign-key team.key
rhythm echo consent verify .echo/consent/<id> --key team.pub
```

Share `team.pub`; keep `team.key` like any other secret. In the GitHub Action, `sign: true` signs with keyless cosign instead (`consent.json.sigstore.json`); `verify` notices the bundle and prints the `cosign verify-blob` line to check it, since Echo doesn't verify keyless bundles itself. See [Consent in CI](Echo-Consent-CI#signing-the-evidence).

A signature proves who made the evidence file and that it hasn't changed. It doesn't prove the site behaves the same for every visitor, region or day. Keyless signing writes the repository and workflow name to the public Rekor log.

## consent.json

`consent.json` is the `data` of the JSON envelope Echo prints with `--json`. Top-level fields of a real result:

```json
{
  "id": "2026-10-29T02-09-24-6076b5",
  "started": "2026-10-28T20:09:24.512863619Z",
  "duration_ms": 14885,
  "echo": "1.2.0",
  "browser": "Chrome 153.0.8010.52",
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36 rhythm-echo-consent/1.2.0",
  "os": "linux/amd64",
  "sandbox": true,
  "tls_verified": false,
  "held": true,
  "mapped": true,
  "policy": "advanced",
  "geo": "",
  "locale": "en-US",
  "timezone": "",
  "viewport": "desktop",
  "catalog": { "version": "1.2.0", "sha256": "6b08cf0a…", "count": 47 },
  "cmp_rules": { "version": "1.2.0", "sha256": "92bb49a8…", "count": 9 },
  "network": { "requests": 45, "held": 15, "refused": 50, "browser_internal": 10 },
  "pages": [ … ],
  "issues": [ … ],
  "inconclusive": [],
  "comparison": null,
  "evidence": { "folder": ".echo/consent/2026-10-28T20-09-24Z-6076b5", "files": [ … ] },
  "notes": [
    "Evidence, not legal advice.",
    "Hits held in the browser.",
    "TLS not verified.",
    "Hosts mapped with --map-host.",
    "gcd decoded with an unofficial, community-documented mapping.",
    "--map-host answered https pages over plain http, so TLS wasn't checked."
  ],
  "complete": true
}
```

| Field | Holds |
|---|---|
| `held`, `tls_verified`, `mapped`, `sandbox` | how the check ran; reports repeat each one that isn't the safe default |
| `catalog`, `cmp_rules` | the tracker catalogue and CMP rules used, with their SHA-256, so a result says what it was judged by |
| `network` | requests seen, hits held, requests the egress guard refused and Chrome's own requests (never judged) |
| `pages[]` | per page: `url`, `final_url`, `redirects`, `cmp`, `consent_mode`, `needs_no_consent` and `runs[]` |
| `pages[].runs[]` | per run: `status`, `reason`, `attempts`, `choice` (how, selector, text, layer, time, `verified_by`, the CMP's state), `visits`, `hits`, `cookies`, `storage`, `scripts`, `signals`, `final_state`, `screenshots`, `events_file` |
| `issues[]` | `check`, `level`, `key`, `page`, `run`, `tracker`, `policy`, `title`, `detail`, `fix` and `evidence` (event numbers) |
| `inconclusive[]` | runs that couldn't make or confirm their choice, with `reason` |
| `comparison` | with `--against`: the baseline and every BROKE, NEW, CHANGED and FIXED change |
| `complete` | `false` when the check was interrupted |

One hit, as it's recorded:

```json
{
  "seq": 16,
  "at_ms": 3861,
  "phase": "after-reject",
  "visit": 1,
  "url": "https://www.youtube.com/api/stats/qoe?event=streamingstats&docid=tbvideo1&_tbrun=c11",
  "held": true,
  "tracker": "youtube",
  "name": "YouTube embed",
  "category": "embed",
  "need": "needed",
  "kind": "beacon"
}
```

Times are milliseconds since the run's first navigation, on one clock for requests and clicks: a hit at the click's millisecond or later is after the choice.

## The JSON envelope

`--json` (or `-o <file>.json`) writes Echo's usual envelope, kind `consent`, with the result as `data`:

```json
{
  "schema": "rhythm-echo/v1",
  "kind": "consent",
  "tool": "rhythm-echo",
  "version": "1.2.0",
  "complete": true,
  "summary": {
    "broke": 0,
    "new": 0,
    "changed": 0,
    "fixed": 0,
    "errors": 0,
    "warnings": 3,
    "notices": 2,
    "pages": 1,
    "runs": 3,
    "failed_runs": 0,
    "inconclusive_runs": 0,
    "trackers": 3,
    "before_choice": 1,
    "after_reject": 1,
    "held": 15
  },
  "verdict": "findings",
  "data": { … }
}
```

`verdict` is `ok`, `findings` or `error` (incomplete) and doesn't depend on `--fail-on`. `before_choice` and `after_reject` count trackers, not hits. With a format flag the report goes alone to stdout and the terminal view to stderr. `consent detect --json`, `consent verify --json`, `consent cmps --json` and `consent trackers --json` use the kinds `consent-detect`, `consent-verify` and `consent-catalog`. JSON Schemas for every kind are in [`report/schema/`](https://github.com/RhythmShahriar/echo/tree/master/report/schema) in the Echo repository.

## Other formats

| Flag | Writes |
|---|---|
| `--html`, or `report.html` | a standalone page with its own Content Security Policy: a verdict bar, a matrix of pages by runs, the issues, the screenshots with the clicked button outlined, a timeline per run with the choice as a line, a Consent Mode panel, "Seen, no consent needed" and the run details. It shows nothing the JSON doesn't |
| `--md` | the summary for a pull request or a ticket |
| `--junit` | one test suite per page, one test case per run, one `<failure>` per error |
| `--har` | a HAR 1.2 file per run in `har/`: method, masked URL, masked headers, timings, status, size; no bodies, no `Cookie` or `Authorization` |

Markdown, for the testbed's `c01` with no Consent Mode default:

```text
## ♪ Echo consent

**Findings.** 1 page, 3 runs, 2 trackers, policy advanced: Google Analytics 4 fired before the choice; Google Ads fired before the choice; Google tags ran with no Consent Mode default.

| Page | Banner | No choice | Accept | Reject |
|---|---|---|---|---|
| / | Complianz | ✗ Google Analytics 4, Google Ads, no-default, v2-signals | ✗ Google Analytics 4, Google Ads | ✗ Google Analytics 4, Google Ads |

### Errors (7)

| Check | Page | Run | What | Fix |
|---|---|---|---|---|
| consent/no-default | / | No choice | Google tags ran with no Consent Mode default | Set a Consent Mode default with every signal denied before any Google tag loads, e.g. gtag\('consent', 'default', {...}\) at the top of the head. |
| consent/before-choice | / | No choice | Google Analytics 4 fired before the visitor chose | Set a Consent Mode default \(all four signals denied\) before the Google tag, or let the banner block it until consent |
...

_Evidence, not legal advice. Echo 1.2.0, Chrome 153.0.8010.52, 2026-10-28 20:07 UTC. Hits held in the browser. Hosts mapped with --map-host; https pages were answered over plain http, so TLS wasn't checked. consent.json sha256 6f81fb81dd17..._
```

JUnit, the same result:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="rhythm-echo consent" tests="3" failures="3" skipped="0" time="32.224">
  <testsuite name="/" tests="3" failures="3" skipped="0" time="32.224" timestamp="2026-10-28T20:07:39Z">
    <properties>
      <property name="consent.json.sha256" value="6f81fb81dd17bca5245ec6aa67f940a18fe438a4242c94fcbda9f47ea6dd8a61"></property>
    </properties>
    <testcase name="/ · No choice" classname="rhythm-echo.consent" time="10.060">
      <failure message="consent/no-default: Google tags ran with no Consent Mode default" type="error">…</failure>
      …
```

## What's masked

Results stay on your machine and IDs are masked before anything is written:

- tracking IDs to their last four characters, in URLs, cookie names and paths: `tid=••••D001`, `_ga_••••D001`, `viewthroughconversion/••••0001/`;
- query values that look like secrets or personal data: `?token=•••••&email=••••@shop.test&phone=••••(phone)`;
- header and cookie values in HAR files.

`--show-ids` keeps IDs whole for your own debugging. It's refused with `--md` and `--junit` (and by the Action), since those are meant to be shared. A result made with it can't be shown again as Markdown or JUnit:

```text
rhythm echo: --show-ids keeps tracking IDs unmasked; it can't be used with Markdown or JUnit output, which are meant to be shared
```

Masked or hashed identifiers can still be personal data. Treat the folder like a log.

## consent report

`rhythm echo consent report [<id> | latest]` shows a saved result again, in any format, without running anything. It takes `--against`, `--accept` and `--fail-on` and exits as the check would for that result:

```text
$ rhythm echo consent report latest -q
Findings - 0 errors, 3 warnings, 2 notices - 3 runs, 3 trackers, policy advanced
  warning consent/embed           YouTube embed loaded before the visitor chose (No choice)
...
```

`<id>` is a folder name in `.echo/consent`, or a path to a folder or a `consent.json`. `--against last` compares with the newest result older than the one shown. The evidence folder isn't written to.

## consent prune

```text
$ rhythm echo consent prune --keep 5
Removed 11 consent results, kept 5.
```

`prune` keeps the newest `--keep` results (default `20`) and every baseline; it leaves `detect/` alone. Consent results are never pruned on their own, so run it now and then.

---
<sub>← [Consent in CI](Echo-Consent-CI) · [Home](Home) · [Docker and DDEV](Echo-Docker-And-DDEV) →</sub>
