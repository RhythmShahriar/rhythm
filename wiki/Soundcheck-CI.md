Run Soundcheck on every deploy or pull request and fail the pipeline when something broke. This page covers the GitHub Action with every input and output, comparing staging with production, a snapshot and a compare in one job, accessibility evidence with `eaa: true`, `--fail-on`, JUnit, the job summary, artifacts and other CI systems. To keep a snapshot of `main` between workflow runs and comment on pull requests, see [CI baselines and PR comments](Soundcheck-CI-Baselines).

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
      - uses: RhythmShahriar/soundcheck@v1.3.0
        with:
          url: https://staging.yoursite.com
          against: https://yoursite.com
          fail-on: broke
```

The action downloads the Soundcheck release for the runner (Linux or macOS, `amd64` or `arm64`), verifies the cosign signature on the release's `checksums.txt` and the archive's checksum, runs it with `--plain`, then writes:

- the Markdown report into the **job summary**;
- `soundcheck-report.html` and `soundcheck-junit.xml` into an artifact named **`soundcheck-report`**;
- an error annotation and a failed job when the result meets `fail-on`.

Use the action at a release tag such as `@v1.3.0`: it then installs that same Soundcheck version, so results only change when you move the tag. At a branch it installs the latest release and warns that the version isn't pinned. A later Soundcheck step in the same job reuses the install.

GitHub's `ubuntu-latest` runners have Google Chrome installed, so no browser download is needed there.

### Inputs

| Input | Default | Description |
|---|---|---|
| `url` | (required) | The site to check, e.g. `https://staging.yoursite.com`. |
| `against` | `""` | Compare with this site now (e.g. production) instead of a snapshot. |
| `mode` | `against` when `against` is set, otherwise `run` | `against`: compare `url` with `against`. `snapshot`: record `url` for a later compare step in the same job, or with `baseline` for later runs. `compare`: compare `url` with the latest snapshot, or the baseline. `run`: check once. |
| `name` | `ci` | Snapshot name, for mode `snapshot`. |
| `baseline` | `""` | Keep a snapshot between workflow runs, e.g. `main`, in the Actions cache. See [CI baselines](Soundcheck-CI-Baselines). |
| `comment` | `"false"` | `"true"` posts the result as one comment on the pull request and updates it on later runs. Needs `permissions: pull-requests: write`. See [The pull request comment](Soundcheck-CI-Baselines#the-pull-request-comment). |
| `github-token` | the job's `GITHUB_TOKEN` | Token for the pull request comment. |
| `fail-on` | `broke` for modes `against` and `compare`, `error` for `run` and `snapshot` | Fail the job on changes at or above `broke`, `new`, `changed` or `slower` (comparisons), or on issues at or above `error`, `warning` or `notice` (`run`, `snapshot`). `none` never fails. |
| `args` | `""` | Extra flags, e.g. `--limit 100 --skip speed -k`. Split on spaces with no glob expansion, so a value with spaces or quotes can't go here: use `headers` and `config` for those. |
| `headers` | `""` | Extra request headers, one `Name: value` per line, each passed whole as its own `-H`. Values of four or more characters are masked in the log. |
| `config` | `.soundcheck/config.yaml` when it exists | Settings file, passed as `--config`, relative to `working-directory`. |
| `working-directory` | `.` | Folder every step runs in, e.g. `apps/web` in a monorepo. `.soundcheck/` is looked up there. |
| `echo` | `""` | `"true"` for Echo at `http://127.0.0.1:8025`, or Echo's URL. Start Echo in an earlier step. See [Echo CI](Echo-CI). |
| `version` | the action's tag | Soundcheck version to install, e.g. `1.3.0`, or `latest`. |
| `eaa` | `"false"` | `"true"` adds `--eaa`: accessibility evidence for the European Accessibility Act. See [EAA mode](#eaa-mode). |
| `statement` | `"false"` | `"true"` also writes the statement draft `accessibility-statement-draft.md` in `working-directory` after the run and adds it to the artifact. Needs a `statement:` block in the config; without one the step only warns. Replaces a file of that name in the workspace. |
| `wcag` | `""` | Passed as `--wcag`: `2.0`, `2.1` or `2.2`. Default 2.1 for a new snapshot; a compare uses its snapshot's. |
| `upload` | `"true"` | Upload the HTML and JUnit reports as the `soundcheck-report` artifact, kept for 14 days. Reports can show logged-in pages, so keep the repository's artifacts private if they do. |

The default mode is `against` when the `against` input is set. Otherwise it's `run`. Leave `fail-on` empty for the default that fits the mode; a value of the wrong kind for the mode (`broke` for `run`, `error` for `compare`) is a usage error and fails the step with exit code 2.

### Outputs

| Output | Description |
|---|---|
| `exit-code` | Soundcheck's exit code: `0` ok, `1` `fail-on` hit, `2` usage error, `3` couldn't check, or with `eaa` the a11y or keyboard tests didn't finish on some pages (see `incomplete`). |
| `version` | The Soundcheck version that ran, e.g. `1.3.0`. |
| `report` | Path of the standalone HTML report. |
| `statement` | Path of the statement draft with `statement: true`, otherwise empty. |
| `a11y-open` | Open accessibility and keyboard problems of an `--eaa` run, e.g. `12`; empty without it. |
| `criteria-failed` | WCAG success criteria with failures in an `--eaa` run, e.g. `3`; empty without it. |
| `incomplete` | `true` when an `--eaa` run exited 3 because the a11y or keyboard tests didn't finish on some pages; the reports, outputs and baseline are still written. Empty otherwise. |

When Soundcheck exits `2` or higher, the step fails straight away and no reports are written (except an incomplete `--eaa` run, see [EAA mode](#eaa-mode)). The job summary then shows why, with the last lines Soundcheck printed:

```text
## Soundcheck

Soundcheck could not check:

~~~text
<the last five lines Soundcheck printed on stderr>
~~~
``` On `1`, the reports are written and uploaded first, then the job fails with `Soundcheck found changes or issues at or above --fail-on broke. See the job summary and the soundcheck-report artifact.` (with `its default` in place of the level when `fail-on` isn't set).

## Staging against production

The simplest setup: after a deploy to staging, compare it with production. Nothing to store between runs; both sites are checked now. To refresh staging from production first, with every customer pseudonymised, see Roadie's [nightly refresh](Roadie-CI#refresh-staging-every-night), which ends with a Soundcheck step.

```yaml
      - uses: RhythmShahriar/soundcheck@v1.3.0
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
        uses: RhythmShahriar/soundcheck@v1.3.0
        with:
          url: https://staging.yoursite.com
          mode: snapshot
          name: before-deploy
          fail-on: none
          upload: "false"

      - name: Deploy
        run: ./deploy.sh staging

      - name: Compare after
        uses: RhythmShahriar/soundcheck@v1.3.0
        with:
          url: https://staging.yoursite.com
          mode: compare
          fail-on: broke
```

Each step installs the release again, which takes a few seconds. Snapshots live in `.soundcheck/` in the working directory, so both steps must run in the same directory. The checkout also brings your committed `.soundcheck/config.yaml` and flows, so both runs use the same settings.

## EAA mode

`eaa: true` runs Soundcheck with [`--eaa`](Soundcheck-EAA): the a11y and keyboard modules, WCAG 2.1 rules and a statement draft. On every pull request preview, compared with the baseline of `main`:

```yaml
on: deployment_status
permissions: {contents: read, pull-requests: write}

jobs:
  accessibility:
    if: github.event.deployment_status.state == 'success'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4      # brings .soundcheck/config.yaml with its statement: block and key flows
      - uses: RhythmShahriar/soundcheck@v1.3.0
        id: soundcheck                 # so later steps can read its outputs
        with:
          url: ${{ github.event.deployment_status.target_url }}
          mode: compare
          baseline: main
          eaa: true
          statement: true
          comment: true
          fail-on: new        # fail on any new accessibility problem, not only traps
```

The baseline comes from a snapshot job on `main` with `mode: snapshot`, `baseline: main` and `eaa: true` ([CI baselines](Soundcheck-CI-Baselines)). A compare takes `--eaa` from its snapshot.

With `fail-on` left at its default (`broke` when comparing) a new keyboard trap or a key flow that can't be finished fails the job, since both are errors; other new accessibility problems are NEW and fail only with `fail-on: new`.

The job summary and the pull request comment are the Markdown report: counts, criteria, check titles, selectors, page paths and fix hints, never an issue's detail, its focus path or a screenshot. A keyboard check's title can name the control it is about (`Keyboard focus is trapped: Card number`), as the page shows it to every visitor. The Action's run step (`scripts/action-run.sh`), run against the testbed's Ledger site with a keyboard trap, wrote this summary:

```markdown
## ♪ Soundcheck

**Something broke.** 21 pages of http\://127.0.0.1:39200/ compared with snapshot "before" \(28 Oct 2026 19:38 UTC\).

| Broke | New | Changed | Slower | Fixed |
|---:|---:|---:|---:|---:|
| 1 | 0 | 0 | 0 | 0 |

### ✗ BROKE (1)

_Worked in the snapshot, fails now._

| Change | Where | Before → after | Fix |
|---|---|---|---|
| Keyboard focus is trapped (2.1.2)<br><sub>\#consent</sub> | `/a11y/consent.php` |  | Tab and Shift+Tab can't move focus out of an element or region, so a keyboard user is stuck there. |

### Accessibility (WCAG 2.1 A and AA)

1 of 50 criteria have failures · 26 had none found by these tests · 23 need a person · 1 open issue

<details><summary>Criteria with failures</summary>

| Criterion | EN 301 549 | Issues | Pages |
|---|---|---:|---:|
| 2.1.2 No Keyboard Trap | 9.2.1.2 | 1 (new) | 1 |

</details>

> Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.

<sub>rhythm-soundcheck 1.3.0 · 21 pages · 16s · 2026-10-28 19:47 UTC</sub>
```

and these outputs:

```text
exit-code=1
statement=<working-directory>/accessibility-statement-draft.md
a11y-open=1
criteria-failed=1
incomplete=
```

Use the outputs in later steps (the workflow above gives the Soundcheck step `id: soundcheck`), for example to post the counts elsewhere or to fail on a threshold of your own:

```yaml
      - if: steps.soundcheck.outputs.criteria-failed != '' && steps.soundcheck.outputs.criteria-failed != '0'
        run: echo "${{ steps.soundcheck.outputs.criteria-failed }} WCAG criteria have failures"
```

### When the tests didn't finish

An `--eaa` run exits 3 when a page's a11y audit or keyboard walk didn't finish. The Action then doesn't treat it as "could not check": it writes the job summary, the JUnit and HTML reports, the statement draft and the outputs as for a finished run, saves the baseline in mode `snapshot`, sets `incomplete` to `true` and then fails the step. The summary and the comment end with:

```markdown
**The accessibility evidence is incomplete:** the a11y or keyboard tests didn't finish on some pages (see "not checked" in the report), so the statement marks them not tested and the job fails.
```

and the log gets:

```text
::error::rhythm-soundcheck: the accessibility evidence is incomplete: the a11y or keyboard tests didn't finish on 1 page (see "not checked" above), so the statement marks them not tested
```

A draft that can't be written (no `statement:` block, no `--eaa` results) gives `::warning::Soundcheck couldn't write the statement draft: <why>` and the job goes on. `eaa` or `statement` other than `true` or `false` fails the step with exit 2 (`the eaa input is "yes": use true or false`).

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
| `3` | The site couldn't be checked, or a report couldn't be written; with `--eaa` also: a page's a11y or keyboard tests didn't finish |
| `130` | Interrupted |

Start with `broke`. Visual and speed changes are often intended, so `changed` and `slower` suit a team that reviews every visual change before release.

## JUnit and the job summary

The action writes JUnit XML and Markdown for you. To make them yourself, render the stored result again with `report`, which doesn't re-check the site:

```sh
rhythm soundcheck report --md >> "$GITHUB_STEP_SUMMARY" || true
rhythm soundcheck report --junit -o soundcheck-junit.xml --fail-on new -q || true
rhythm soundcheck report --html -o soundcheck-report.html -q || true
```

`report` exits as the stored result did: 1 when `--fail-on` is met, 3 for an `--eaa` result whose tests didn't finish on some pages, 130 for an interrupted one. The `|| true` keeps a shell with `set -e` (GitHub's default for `run:`) writing the other files; fail the job on the check's own step.

`--json` can't go with several `-o`: each file's format comes from its extension, so `report -o r.json -o r.md -o r.xml` writes all three and `--json -o r.md -o r.xml` stops with exit 2. Read the JSON from `r.json`.

The JUnit file has two kinds of suite:

- **By class** (by level for a run): `soundcheck.broke`, `soundcheck.new` and so on, with a test case per change (per issue for a run). Cases at or above `--fail-on` are failures (BROKE and NEW without it).
- **By page**, for an `--eaa` result only: `soundcheck.page /checkout/`, with a case per page and accessibility module (`/checkout/ · keyboard`). A page case fails on the same issues that fail their class suite, so with `--fail-on new` a NEW missing label fails both `soundcheck.new` and its page's `a11y` case. A module whose tests didn't finish on the page makes its case skipped.

One extra passing case, such as `4 pages checked`, records that the check ran even when nothing changed.

The action only attaches the JUnit file to the `soundcheck-report` artifact; nothing publishes it as test results. To see it in GitHub's test views, add a test reporter action of your choice after the Soundcheck step, pointed at `${{ runner.temp }}/soundcheck-report/soundcheck-junit.xml`.

The Markdown report starts with the verdict and a count table, then a table per class with the change, where, before → after and the fix. It fits pull request comments too.

## Try the Action locally

The Action's work happens in `scripts/action-run.sh`, which reads its inputs from the environment (`URL`, `AGAINST`, `MODE`, `NAME`, `BASELINE`, `FAIL_ON`, `ARGS`, `HEADERS`, `CONFIG`, `ECHO`, `EAA`, `STATEMENT`, `WCAG`) and writes where GitHub's runner would tell it to. To run it before you push, build Soundcheck in a clone of its repository and run the script in your project folder, the one with `.soundcheck/config.yaml`. Here it is the folder of the [EAA walk-through](Soundcheck-EAA#your-first-eaa-run), with the testbed's Ledger site served on port 39200:

```sh
SC=~/src/soundcheck                              # your clone of the Soundcheck repository
(cd "$SC" && make build) && export PATH="$SC/bin:$PATH"   # the script runs rhythm-soundcheck from PATH
mkdir -p /tmp/sc-action
export RUNNER_TEMP=/tmp/sc-action GITHUB_OUTPUT=/tmp/sc-action/output GITHUB_STEP_SUMMARY=/tmp/sc-action/summary.md
URL=http://127.0.0.1:39200/ MODE=snapshot BASELINE=main EAA=true "$SC/scripts/action-run.sh" run
TESTBED_STATE=/tmp/s "$SC/testbed/toggle.sh" s65 broken
URL=http://127.0.0.1:39200/ MODE=compare BASELINE=main EAA=true STATEMENT=true FAIL_ON=new "$SC/scripts/action-run.sh" run
cat /tmp/sc-action/summary.md /tmp/sc-action/output
```

The compare writes the summary shown under [EAA mode](#eaa-mode) and outputs such as:

```text
exit-code=1
report=/tmp/sc-action/soundcheck-report/soundcheck-report.html
markdown=/tmp/sc-action/soundcheck-summary.md
statement=<your project folder>/accessibility-statement-draft.md
a11y-open=1
criteria-failed=1
incomplete=
```

The script itself exits 0 after a check that ran; the Action fails the step from `exit-code` afterwards. The JUnit and HTML reports land in `/tmp/sc-action/soundcheck-report/` and the statement draft in the project folder. The cache, the artifact upload and the pull request comment are GitHub's own steps in `action.yml` and don't run here; the snapshot simply stays in `.soundcheck/`. To run the whole workflow file, [act](https://github.com/nektos/act) runs GitHub workflows in Docker.

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
      - uses: RhythmShahriar/soundcheck@v1.3.0
        env:
          SOUNDCHECK_USER: ${{ secrets.SOUNDCHECK_USER }}
          SOUNDCHECK_PASSWORD: ${{ secrets.SOUNDCHECK_PASSWORD }}
        with:
          url: https://staging.yoursite.com
          against: https://yoursite.com
```

For staging behind basic auth, pass the header with the `headers` input. Each line is one `-H`, kept whole. Its value is masked in the log:

```yaml
      - uses: RhythmShahriar/soundcheck@v1.3.0
        with:
          url: https://staging.yoursite.com
          mode: compare
          headers: |
            Authorization: Basic ${{ secrets.STAGING_BASIC_AUTH }}
```

Headers go to the checked site only (and its www. or bare host), never to other hosts a page loads from. In an `--against` comparison they go to both sites. Values your flows read with `${env:NAME}` go in `env:` the same way. Soundcheck never writes them to reports.

## Other CI systems

Nothing above needs GitHub except the action itself. In GitLab CI, Jenkins, Bitbucket or a cron job:

1. Use an image with Chrome or Chromium, or let Soundcheck download Chrome for Testing on first use (it needs the usual Chrome system libraries).
2. Install Soundcheck with the installer, or download the release archive and check it against `checksums.txt`.
3. Run it with `--fail-on` and keep the reports.

```yaml
# .gitlab-ci.yml
soundcheck:
  image: node:22-bookworm   # any image with the Chrome libraries, or one with Chrome installed
  script:
    - curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=soundcheck sh
    - export PATH="$HOME/.local/bin:$PATH"
    - rhythm soundcheck staging.yoursite.com --against yoursite.com --fail-on broke -o soundcheck.md -o soundcheck.xml
  artifacts:
    when: always
    paths: [soundcheck.md, .soundcheck/runs/]
    reports:
      junit: soundcheck.xml
```

`rhythm-soundcheck` is the tool itself; `rhythm soundcheck …` only forwards to it, so you can run `rhythm-soundcheck …` without `rhythm`. Chrome can't use its sandbox as root. Soundcheck no longer turns it off by itself: in a container running as root, run as a normal user, or pass `--no-sandbox` (or set `SOUNDCHECK_NO_SANDBOX=1`) when the site is one you trust. To fail fast instead of downloading Chrome for Testing, add `--no-download` and point at a browser with `--browser /usr/bin/chromium`. See [FAQ](Soundcheck-FAQ#docker-root-and-the-chrome-sandbox).

In CI, Soundcheck never prints the "new release" notice: it's off whenever the `CI` environment variable is set, as most CI systems do.

> [!IMPORTANT]
> Point CI at sites you own or are authorized to test, ideally staging. On production, keep the default levels (no `submit`) and list it under `production` in the config.

---
<sub>← [Accessibility statement](Soundcheck-Accessibility-Statement) · [Home](Home) · [CI baselines and PR comments](Soundcheck-CI-Baselines) →</sub>
