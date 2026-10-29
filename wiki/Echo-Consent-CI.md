Run the consent check on every pull request, deploy or night and fail the pipeline when a tracker starts firing before the choice or after Reject. This page covers the GitHub Action's `mode: consent`, the masked job summary, checking many sites from a sites file, baselines, expiring ignores, signing and other CI systems.

## The GitHub Action

```yaml
# .github/workflows/consent.yml
name: consent
on:
  pull_request:
  schedule:
    - cron: "17 5 * * 1"          # every Monday, for changes made outside the repository

jobs:
  consent:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write        # for pr-comment
    steps:
      - uses: actions/checkout@v4
      - uses: RhythmShahriar/echo@v1.2.0
        with:
          mode: consent
          site: https://staging.shop.test/ https://staging.shop.test/shop/
          fail-on: error
          pr-comment: true
```

`mode: consent` installs and verifies Echo like the other modes (the release's cosign signature, then the archive's checksum) but doesn't start the catcher. It runs `rhythm-echo consent` on the pages in `site` in the runner's Chrome (the GitHub ubuntu and macOS images ship one, so nothing is downloaded), then writes the job summary and fails the job when the check found issues at or above `fail-on` or couldn't check. It runs on Linux and macOS runners.

Check only sites you own or are authorised to test. The runner loads them like one visitor; tracker hits are held as on your machine.

### Inputs for mode consent

| Input | Default | Description |
|---|---|---|
| `mode` | `run` | `consent`. |
| `site` | `""` | The page or pages to check, separated by spaces or new lines. Empty uses `consent: urls` from the config, or `--sites` in `args`. |
| `fail-on` | `error` | `error`, `warning`, `notice` or `none`; with `--against` in `args`, `broke` (the default), `new`, `changed` or `none`. |
| `args` | `""` | Extra flags, split like a shell would but never run through one, e.g. `--runs +gpc --geo de --against baseline`. `--show-ids` is refused. |
| `working-directory` | `.` | The folder with `.echo/` (config and results). |
| `config` | `""` | A config file relative to `working-directory`. |
| `upload` | `"false"` | Upload the evidence folder (screenshots of the site, `consent.json`, the reports) as the `echo-consent` artifact. The Action warns when it's on in a public repository: anyone can download a public repository's artifacts. |
| `retention-days` | `"14"` | How many days GitHub keeps `echo-consent` (1 to 90). |
| `sign` | `"false"` | Sign `consent.json` with keyless cosign (`consent.json.sigstore.json` in the evidence folder). Needs `permissions: id-token: write`. The repository and workflow name go to the public Rekor log. |
| `summary-details` | `"false"` | Put Echo's own Markdown report in the summary and the comment: query strings, hosts, ID tails and every event. Off by default. |
| `pr-comment` | `"false"` | On a pull request, post one comment with the summary, updated on every later run. Needs `pull-requests: write`. |
| `log` | `"true"` | Upload Echo's log as `echo-log` when the step fails or finds something, masked like the summary. |
| `version` | the Action's own release | Echo version to install. Consent checks need `1.2.0` or later. |

Other inputs (`seal`, `listen`, `smtp`, `name`) are for the other modes and are ignored in mode consent. `url`, `token` and the `smtp` output belong to mode `up`; `action.yml` has an input and an output both named `smtp`; neither is used here.

### Outputs for mode consent

| Output | Description |
|---|---|
| `exit-code` | `0` ok, `1` `fail-on` hit, `2` usage error, `3` couldn't check, `130` interrupted. |
| `json` | Path of `consent.json` in the evidence folder. |
| `before-choice` | How many different trackers fired before the choice (trackers, not hits). |
| `after-reject` | How many different trackers fired after Reject (trackers, not hits). |
| `inconclusive` | How many runs couldn't make or confirm their choice. |
| `report` | Path of `report.html` in the evidence folder. |
| `summary` | Path of a copy of the job summary (Markdown). |
| `comment-id` | ID of the pull request comment, with `pr-comment`. |

On exit `1` the step ends with `Echo found consent issues at or above --fail-on error`; on exit `3` with `Echo couldn't check consent on N runs (see the summary)`.

### The job summary

Anyone who can read the repository can read a job summary, so it shows tracker names, counts, check titles and page paths, never query strings, cookie values, IDs or screenshots. The summary for the two-site check [below](#many-sites), as the Action writes it:

```text
## ♪ Echo consent

**Findings.** 3 pages, 9 runs, 3 trackers, policy advanced. Trackers before the choice: 1. Trackers after Reject: 1.

| Page | Banner | No choice | Accept | Reject |
|---|---|---|---|---|
| brew / | Complianz | nothing fired | Meta Pixel, Google Ads, Google Analytics 4 | nothing fired |
| shop / | Complianz | ✗ Meta Pixel | ✗ Meta Pixel, Google Ads, Google Analytics 4 | ✗ Meta Pixel |
| shop /page-2/ | Complianz | ✗ Meta Pixel | ✗ Meta Pixel, Google Ads, Google Analytics 4 | ✗ Meta Pixel |

### Errors (8)

| Check | Page | Run | What |
|---|---|---|---|
| consent/before-choice | shop / | No choice | Meta Pixel fired before the visitor chose |
| consent/before-choice | shop / | Accept | Meta Pixel fired before the visitor chose |
| consent/before-choice | shop / | Reject | Meta Pixel fired before the visitor chose |
| consent/after-reject | shop / | Reject | Meta Pixel fired after Reject |
| consent/before-choice | shop /page-2/ | No choice | Meta Pixel fired before the visitor chose |
| consent/before-choice | shop /page-2/ | Accept | Meta Pixel fired before the visitor chose |
| consent/before-choice | shop /page-2/ | Reject | Meta Pixel fired before the visitor chose |
| consent/after-reject | shop /page-2/ | Reject | Meta Pixel fired after Reject |

### Notices (2)

| Check | Page | Run | What |
|---|---|---|---|
| consent/meta-no-consent-api | shop / | No choice | Meta Pixel never received fbq('consent', &#46;&#46;&#46;) |
| consent/meta-no-consent-api | shop /page-2/ | No choice | Meta Pixel never received fbq('consent', &#46;&#46;&#46;) |

_Evidence, not legal advice. Hits held in the browser. Hosts mapped with --map-host; https pages were answered over plain http, so TLS wasn't checked._

<sub>Query strings, cookie values, IDs and screenshots are hidden here. The evidence folder (upload: true) has report.html and consent.json.</sub>
```

**Preview the summary locally.** The summary is written by `integrations/action/summary.sh` (needs bash and jq) from the check's envelope:

```sh
rhythm echo consent https://staging.shop.test/ -o consent.json
bash integrations/action/summary.sh consent consent.json
```

With `pr-comment: true` the comment is the same text. With `upload: true`, open `report.html` from the `echo-consent` artifact for the timelines and screenshots.

### Using the counts

```yaml
      - uses: RhythmShahriar/echo@v1.2.0
        id: consent
        with:
          mode: consent
          site: https://staging.shop.test/
          fail-on: none                 # decide below
      - if: steps.consent.outputs.after-reject != '0'
        run: echo "::warning::${{ steps.consent.outputs.after-reject }} trackers fire after Reject"
```

## Many sites

`--sites` reads the same `sites.yaml` Soundcheck fleet reads. Echo uses each entry's `name` and `url`, plus an optional `consent:` map with the pages to check and extra flags; Soundcheck's own keys are ignored.

```yaml
# sites.yaml
sites:
  - name: brew
    url: https://g01.shop.test/
  - name: shop
    url: https://c02.shop.test/
    consent:
      urls: [/, /page-2/]
      args: ["--cmp", "complianz"]
```

```text
$ rhythm echo consent --sites sites.yaml $map
rhythm echo 1.2.0 - consent check of 2 sites (3 pages)
...
shop
https://c02.shop.test/
  Banner: Complianz, found after 0.1s, Reject on the banner
  Consent Mode: v2 default, all four signals denied at 0.02s
  [1/3] No choice   FAIL Meta Pixel fired before the choice
  [2/3] Accept      FAIL Meta Pixel fired before the choice; Google Ads, Google Analytics 4 fired; update granted at 3.2s; kept on 1 more page
  [3/3] Reject      FAIL Meta Pixel fired before the choice; Meta Pixel fired after Reject on 2 of 3 pages; update denied at 3.2s; kept on 1 more page

https://c02.shop.test/page-2/
...

brew
https://g01.shop.test/
  Banner: Complianz, found after 0.1s, Reject on the banner
  Consent Mode: v2 default, all four signals denied at 0.03s
  [1/3] No choice   ok nothing fired; Meta Pixel held back
  [2/3] Accept      ok Meta Pixel, Google Ads, Google Analytics 4 fired; update granted at 3.2s; kept on 1 more page
  [3/3] Reject      ok nothing fired; Meta Pixel held back; update denied at 3.2s; kept on 1 more page

Findings - 8 errors, 0 warnings, 2 notices - 9 runs, 3 trackers, policy advanced

  error   consent/before-choice   Meta Pixel fired before the visitor chose (No choice, shop /)
...
```

Sites are listed worst first and each issue names its site and page. In the Action, pass the file in `args` and leave `site` empty:

```yaml
        with:
          mode: consent
          args: --sites sites.yaml --parallel 6
```

On the benchmark, 50 sites with `--parallel 6` took 5.6 minutes. A command checks at most 50 pages without `--sites`.

## Baselines

A baseline turns "is anything wrong" into "did anything get worse", so a known issue doesn't fail every build while a new one does.

```sh
rhythm echo consent https://staging.shop.test --save baseline          # once, on a state you accept
rhythm echo consent https://staging.shop.test --against baseline       # every build
```

`--save <name>` keeps the result in `.echo/consent/baselines/<name>/`; saving again replaces it and says `(replaced the one from <date> UTC)`. `--against` takes a baseline name, a `consent.json`, a result folder, or `last` (the newest result before this one). `--fail-on` then reads `broke` (the default), `new`, `changed` or `none`; the classes are in [Consent checks](Echo-Consent-Checks#comparison-classes).

```text
$ rhythm echo consent https://c08.shop.test/ --against baseline $map
rhythm echo 1.2.0 - consent check of https://c08.shop.test/ against baseline from 2026-10-28 20:10 UTC
...
Broke - 4 broke, 0 new, 0 changed, 0 fixed - 3 runs, 4 trackers
  BROKE   TikTok Pixel now fires before the choice            / (No choice, Accept, Reject)
  BROKE   TikTok Pixel now fires after Reject                 / (Reject)
```

The verdict counts each page, run and tracker (one per change key) while the lines group them, so `4 broke` above is two lines; `-v` prints each line's keys, as below.

Use one baseline name per site (`--save shop`, `--against shop`). Echo compares by page path, run and tracker and doesn't refuse a baseline of another site; `--against last` likewise compares with whatever was checked last in that store.

### Accepting a change

`-v` prints each change's key; `--accept <key>` takes one as intended for this comparison, as `compare --accept` does:

```text
$ rhythm echo consent report latest --against baseline -v
...
  BROKE   TikTok Pixel now fires before the choice            / (No choice, Accept, Reject)
          (key: consent/before-choice|/|none|tiktok)
          (key: consent/before-choice|/|accept|tiktok)
          (key: consent/before-choice|/|reject|tiktok)
  BROKE   TikTok Pixel now fires after Reject                 / (Reject)
          (key: consent/after-reject|/|reject|tiktok)

$ rhythm echo consent report latest --against baseline --accept 'consent/after-reject|/|reject|tiktok' -q
Broke - 3 broke, 0 new, 0 changed, 0 fixed - 3 runs, 4 trackers
  BROKE   TikTok Pixel now fires before the choice            / (No choice, Accept, Reject)
  BROKE   TikTok Pixel now fires after Reject (accepted)      / (Reject)
```

`consent report` reruns nothing: it reads a saved result and exits as the check would, so one job can check and a later step can compare again with other flags.

### A baseline from another job

The baseline is a `consent.json`. Keep it in the repository (`.echo/consent/baselines/` is a plain folder) or pass it between jobs as an artifact, then point `--against` at the file:

```yaml
        with:
          mode: consent
          site: https://staging.shop.test/
          args: --against baselines/consent.json
```

A baseline from any 1.2.x Echo loads.

## Known issues that expire

`consent: ignore:` in the config hides one issue until a date at most a year ahead, with a reason that shows in every report. When it expires, the issue counts again. See [Ignoring a known issue](Echo-Consent-Checks#ignoring-a-known-issue).

## Signing the evidence

Locally, with your own ed25519 key:

```sh
openssl genpkey -algorithm ed25519 -out team.key && chmod 600 team.key
openssl pkey -in team.key -pubout -out team.pub
rhythm echo consent https://staging.shop.test --sign-key team.key
rhythm echo consent verify .echo/consent/<id> --key team.pub
```

In the Action, `sign: true` signs with keyless cosign and the job's GitHub identity. Check it with cosign:

```sh
cosign verify-blob --bundle consent.json.sigstore.json \
  --certificate-identity-regexp '^https://github.com/<owner>/<repo>/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com consent.json
```

`consent verify` checks the hashes and an ed25519 signature; it doesn't verify keyless bundles. See [Evidence](Echo-Consent-Evidence#signing).

## Other CI systems

The command works anywhere Chrome runs. The exit code is the gate; JUnit feeds a test report:

```yaml
# .gitlab-ci.yml
consent:
  image: debian:bookworm
  script:
    - apt-get update && apt-get install -y chromium curl
    - curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=echo sh   # as root: into /usr/local/bin
    - rhythm echo consent https://staging.shop.test/ --browser /usr/bin/chromium --no-sandbox -o consent.xml
  artifacts:
    when: always
    reports:
      junit: consent.xml
```

`--no-sandbox` is needed when Chrome runs as root in a container (also `RHYTHM_ECHO_NO_SANDBOX=1`); Echo then warns that you should check only sites you trust. JUnit has one test suite per page and one test case per run (`/ · Reject`), with one `<failure>` per error. In CI (`CI` set), Echo shows no once-only notice, no tip and no color.

---
<sub>← [Banners (CMPs)](Echo-Consent-CMPs) · [Home](Home) · [Evidence](Echo-Consent-Evidence) →</sub>
