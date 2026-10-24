Run Soundcheck on every deploy or pull request and fail the pipeline when something broke. This page covers the GitHub Action with every input and output, comparing staging with production, a snapshot and a compare in one job, `--fail-on`, JUnit, the job summary, artifacts and other CI systems.

## The GitHub Action

```yaml
# .github/workflows/soundcheck.yml
name: soundcheck
on:
  workflow_dispatch:
  deployment_status:

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: RhythmShahriar/soundcheck@v1
        with:
          url: https://staging.yoursite.com
          against: https://yoursite.com
          fail-on: broke
```

The action downloads the Soundcheck release for the runner (Linux or macOS, `amd64` or `arm64`), checks it against the release's `checksums.txt`, runs it with `--plain`, then writes:

- the Markdown report into the **job summary**;
- `soundcheck-report.html` and `soundcheck-junit.xml` into an artifact named **`soundcheck-report`**;
- an error annotation and a failed job when the result meets `fail-on`.

GitHub's `ubuntu-latest` runners have Google Chrome installed, so no browser download is needed there.

### Inputs

| Input | Default | Description |
|---|---|---|
| `url` | (required) | The site to check, e.g. `https://staging.yoursite.com`. |
| `against` | `""` | Compare with this site now (e.g. production) instead of a snapshot. |
| `mode` | `against` when `against` is set, otherwise `run` | `against`: compare `url` with `against`. `snapshot`: record `url` for a later compare step in the same job. `compare`: compare `url` with the latest snapshot. `run`: check once. |
| `name` | `ci` | Snapshot name, for mode `snapshot`. |
| `fail-on` | `broke` for modes `against` and `compare`, `error` for `run` and `snapshot` | Fail the job on changes at or above `broke`, `new`, `changed` or `slower` (comparisons), or on issues at or above `error`, `warning` or `notice` (`run`, `snapshot`). `none` never fails. |
| `args` | `""` | Extra flags, e.g. `--limit 100 --skip speed -k`. |
| `version` | `latest` | Soundcheck version to install, e.g. `1.0.0`. |
| `upload` | `"true"` | Upload the HTML and JUnit reports as the `soundcheck-report` artifact. |

The default mode is `against` when the `against` input is set. Otherwise it's `run`. Leave `fail-on` empty for the default that fits the mode; a value of the wrong kind for the mode (`broke` for `run`, `error` for `compare`) is a usage error and fails the step with exit code 2.

### Outputs

| Output | Description |
|---|---|
| `exit-code` | Soundcheck's exit code: `0` ok, `1` `fail-on` hit, `2` usage error, `3` couldn't check. |
| `report` | Path of the standalone HTML report. |

When Soundcheck exits `2` or higher, the step fails straight away and no reports are written. On `1`, the reports are written and uploaded first, then the job fails with `Soundcheck found changes or issues at or above --fail-on broke. See the job summary and the soundcheck-report artifact.` (with `its default` in place of the level when `fail-on` isn't set).

## Staging against production

The simplest setup: after a deploy to staging, compare it with production. Nothing to store between runs; both sites are checked now.

```yaml
      - uses: RhythmShahriar/soundcheck@v1
        with:
          url: https://staging.yoursite.com
          against: https://yoursite.com
          fail-on: new
          args: --limit 100
```

The production site is discovered and checked first, then staging is checked on the same pages. The same command on your machine:

```sh
rhythm soundcheck staging.yoursite.com --against yoursite.com --fail-on new
```

Differences that are only about the environment (another domain in a link, a staging banner) show up as changes; hide stable ones with `visual.ignore`.

> [!TIP]
> Add production to the config's `production` list. A comparison then never submits forms or runs the leak test against it, even if someone adds `--level submit` later. See [Interactions](Soundcheck-Interactions#the-production-guard).

## Snapshot, deploy, compare in one job

To check what a deploy did to the same site, snapshot before and compare after, in one job so the snapshot is still on disk:

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Snapshot before
        uses: RhythmShahriar/soundcheck@v1
        with:
          url: https://staging.yoursite.com
          mode: snapshot
          name: before-deploy
          fail-on: none
          upload: "false"

      - name: Deploy
        run: ./deploy.sh staging

      - name: Compare after
        uses: RhythmShahriar/soundcheck@v1
        with:
          url: https://staging.yoursite.com
          mode: compare
          fail-on: broke
```

Each step installs the release again, which takes a few seconds. Snapshots live in `.soundcheck/` in the working directory, so both steps must run in the same directory. The checkout also brings your committed `.soundcheck/config.yaml` and flows, so both runs use the same settings.

## Failing a build

`--fail-on` makes Soundcheck exit `1` when the result meets the threshold:

| Result | Values | Example |
|---|---|---|
| a comparison | `broke`, `new`, `changed`, `slower` | `new` fails on BROKE or NEW; `slower` on anything but FIXED |
| a run or snapshot | `error`, `warning`, `notice` | `warning` fails on errors or warnings |

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or `--fail-on` not set) |
| `1` | A change or issue at or above `--fail-on` |
| `2` | Usage error: a bad flag or value, no snapshot to compare with |
| `3` | The site couldn't be checked, or a report couldn't be written |
| `130` | Interrupted |

Start with `broke`. Visual and speed changes are often intended, so `changed` and `slower` suit a team that reviews every visual change before release.

## JUnit and the job summary

The action writes JUnit XML and Markdown for you. To make them yourself, render the stored result again with `report`, which doesn't re-check the site:

```sh
rhythm soundcheck report --md >> "$GITHUB_STEP_SUMMARY"
rhythm soundcheck report --junit -o soundcheck-junit.xml --fail-on new -q
rhythm soundcheck report --html -o soundcheck-report.html -q
```

In JUnit, each class (or level) is a test suite and each change a test case; cases at or above `--fail-on` are failures (BROKE and NEW without it). One extra passing case, `37 pages checked`, records that the check ran even when nothing changed.

The action only attaches the JUnit file to the `soundcheck-report` artifact; nothing publishes it as test results. To see it in GitHub's test views, add a test reporter action of your choice after the Soundcheck step, pointed at `${{ runner.temp }}/soundcheck-report/soundcheck-junit.xml`.

The Markdown report starts with the verdict and a count table, then a table per class with the change, where, before → after and the fix. It fits pull request comments too.

## Artifacts

With `upload: "true"`, the `soundcheck-report` artifact holds `soundcheck-report.html` (the standalone report with the before/after sliders) and `soundcheck-junit.xml`. Download it from the run's summary page and open the HTML in any browser.

To keep the whole `.soundcheck/runs/` folder (screenshots and JSON) instead, upload it yourself:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: soundcheck-runs
          path: .soundcheck/runs/
```

## Logging in and secrets

Pass the test account from repository secrets, so the leak test and login flows run:

```yaml
      - uses: RhythmShahriar/soundcheck@v1
        env:
          SOUNDCHECK_USER: ${{ secrets.SOUNDCHECK_USER }}
          SOUNDCHECK_PASSWORD: ${{ secrets.SOUNDCHECK_PASSWORD }}
        with:
          url: https://staging.yoursite.com
          against: https://yoursite.com
```

`args` is split on spaces, without honoring quotes, so it can't carry a value that contains a space, such as a basic auth header. For staging behind basic auth, install Soundcheck and run it in a `run` step instead:

```yaml
      - name: Install Soundcheck
        run: |
          curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=soundcheck sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"

      - name: Compare
        env:
          AUTH: ${{ secrets.STAGING_BASIC_AUTH }}   # base64 of user:password
        run: |
          rhythm soundcheck staging.yoursite.com --against yoursite.com \
            -H "Authorization: Basic $AUTH" --fail-on broke --md -o soundcheck.md

      - name: Summary
        if: always()
        run: cat soundcheck.md >> "$GITHUB_STEP_SUMMARY"
```

Headers go to both sites in an `--against` comparison. Values your flows read with `${env:NAME}` go in `env:` the same way. Soundcheck never writes them to reports.

## Other CI systems

Nothing above needs GitHub except the action itself. In GitLab CI, Jenkins, Bitbucket or a cron job:

1. Use an image with Chrome or Chromium, or let Soundcheck download Chromium on first use (it needs the usual Chrome system libraries).
2. Install Soundcheck with the installer, or download the release archive and check it against `checksums.txt`.
3. Run it with `--fail-on` and keep the reports.

```yaml
# .gitlab-ci.yml
soundcheck:
  image: node:22-bookworm   # any image with the Chrome libraries, or one with Chrome installed
  script:
    - curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=soundcheck sh
    - export PATH="$HOME/.local/bin:$PATH"
    - rhythm soundcheck staging.yoursite.com --against yoursite.com --fail-on broke --md -o soundcheck.md
  artifacts:
    when: always
    paths: [soundcheck.md, .soundcheck/runs/]
```

`rhythm-soundcheck` is the tool itself; `rhythm soundcheck …` only forwards to it, so you can run `rhythm-soundcheck …` without `rhythm`. In containers running as root, Soundcheck starts Chrome without its sandbox automatically. To fail fast instead of downloading Chromium, add `--no-download` and point at a browser with `--browser /usr/bin/chromium`. See [FAQ](Soundcheck-FAQ#docker-root-and-the-chrome-sandbox).

In CI, Soundcheck never prints the "new release" notice: it's off whenever the `CI` environment variable is set, as most CI systems do.

> [!IMPORTANT]
> Point CI at sites you own or are authorized to test, ideally staging. On production, keep the default levels (no `submit`) and list it under `production` in the config.

---
<sub>← [Speed](Soundcheck-Speed) · [Home](Home) · [Configuration](Soundcheck-Configuration) →</sub>
