Run Echo on every pull request or deploy, and fail the pipeline when a message stops being sent. This page covers the GitHub Action with its modes, inputs and outputs, starting Echo for Soundcheck with `mode: up`, a snapshot and a compare in one job, the reports, and other CI systems.

Echo has to run where the site's connector can reach it. In CI that usually means the site runs in the job too, with Docker Compose, `php artisan serve`, `next start` or similar, and its connector points at `http://127.0.0.1:8025`. To check a staging server, run `rhythm echo up` on that server and its actions from there.

## The GitHub Action

```yaml
# .github/workflows/echo.yml
name: echo
on: [pull_request]

jobs:
  messages:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Start the site
        run: docker compose up -d --wait        # your staging stack, with Echo's connector

      - uses: RhythmShahriar/echo@v1
        with:
          mode: run
          fail-on: error
```

The action downloads the Echo release for the runner (Linux or macOS, `amd64` or `arm64`), checks it against the release's `checksums.txt`, and starts `rhythm-echo up` in the background, sealed. Unless the mode is `up`, it then runs the actions in your committed `.echo/config.yaml` with `--plain`, writes the reports, and stops Echo. It writes:

- the Markdown report into the **job summary**;
- `echo-report.html` and `echo-junit.xml` into an artifact named **`echo-report`**;
- an error annotation, and a failed job, when the result meets `fail-on`.

### Modes

| Mode | Does |
|---|---|
| `run` (default) | Starts Echo, runs the actions once, checks the messages, stops Echo. |
| `snapshot` | The same, saving the result as snapshot `name` for a `compare` later in the job. |
| `compare` | Runs the actions again and compares with the latest snapshot. |
| `up` | Starts Echo and leaves it running for later steps, such as Soundcheck with `--echo`. No report. |

### Inputs

| Input | Default | Description |
|---|---|---|
| `mode` | `run` | `up`, `snapshot`, `compare` or `run`. |
| `name` | `ci` | Snapshot name, for mode `snapshot`. |
| `fail-on` | `broke` for `compare`, `error` otherwise | Fail on changes at or above `broke`, `new` or `changed` (`compare`), or on issues at or above `error`, `warning` or `notice` (`run`, `snapshot`). `none` never fails. |
| `seal` | `seal` | `seal` (nothing leaves) or `observe` (send for real). |
| `listen` | `127.0.0.1:8025` | The API address. Anything that isn't `127.*` or `localhost` gets a generated token. |
| `args` | `""` | Extra flags for the command, e.g. `--action Signup -v`. |
| `version` | `latest` | Echo version to install, e.g. `1.0.0`. |
| `upload` | `"true"` | Upload the HTML and JUnit reports as the `echo-report` artifact. |

`args` is split on spaces without honoring quotes, so it can't carry a value with a space, such as an action name like `Contact form`. For that, install Echo and run it in a `run` step; see [Other CI systems](#other-ci-systems).

<!-- TODO(lead): action.yml's own description suggests args "--action 'Contact form'", which the space split breaks; fix the description or the splitting, then update this paragraph. -->

Echo reads its other settings (the SMTP sink, the proxy, the allowlist, the actions) from `.echo/config.yaml` in the working directory, so commit that file. The SMTP sink defaults to `127.0.0.1:1025`.

### Outputs

| Output | Description |
|---|---|
| `url` | Where Echo listens, as `http://127.0.0.1:<port>`, for `RHYTHM_ECHO_URL` in the site under test. |
| `token` | The token connectors must send when Echo listens beyond loopback; masked in logs. Empty on loopback. |
| `exit-code` | The command's exit code: `0` ok, `1` `fail-on` hit, `2` usage error, `3` couldn't run. |
| `report` | Path of the standalone HTML report. |

Later steps also get `RHYTHM_ECHO_URL`, and `RHYTHM_ECHO_TOKEN` when there's a token, in their environment.

When the command exits `2` or higher, the step fails straight away and no reports are written. On `1`, the reports are written and uploaded first, then the job fails with `Echo found changes or issues at or above --fail-on broke. See the job summary and the echo-report artifact.` (with `its default` in place of the level when `fail-on` isn't set).

<!-- TODO(lead): with listen 0.0.0.0:8025 for a site in a service container, the url output is still http://127.0.0.1:<port>, which a service container can't reach. Confirm the address a service container should use and document it. -->

## With Soundcheck

Start Echo in `up` mode, then let Soundcheck submit the forms with `--echo`:

```yaml
    steps:
      - uses: actions/checkout@v4
      - name: Start the site
        run: docker compose up -d --wait

      - uses: RhythmShahriar/echo@v1
        with:
          mode: up

      - uses: RhythmShahriar/soundcheck@v1
        with:
          url: http://127.0.0.1:8080
          mode: run
          args: --level inspect,click,submit --echo
```

Echo keeps running until the job ends. Soundcheck reads Echo's token from `RHYTHM_ECHO_TOKEN`, which the Echo step exported. With another `listen` address, pass it on: `--echo=${{ steps.echo.outputs.url }}` (give the Echo step `id: echo`). Its messages appear in Soundcheck's own reports; see [With Soundcheck](Echo-With-Soundcheck). Echo's log is in `$RUNNER_TEMP/echo.log`.

To compare what forms send before and after a deploy in one job, take a Soundcheck snapshot with `--echo`, deploy, then compare with `--echo`, as in [Soundcheck's CI guide](Soundcheck-CI#snapshot-deploy-compare-in-one-job).

## Snapshot, change, compare in one job

```yaml
    steps:
      - uses: actions/checkout@v4
        with: {ref: "${{ github.event.pull_request.base.sha }}"}
      - run: docker compose up -d --wait

      - name: Snapshot the base
        uses: RhythmShahriar/echo@v1
        with:
          mode: snapshot
          name: base
          fail-on: none
          upload: "false"

      - uses: actions/checkout@v4
        with: {clean: false}
      - run: docker compose up -d --wait --build

      - name: Compare the pull request
        uses: RhythmShahriar/echo@v1
        with:
          mode: compare
          fail-on: broke
```

Each Echo step installs the release again, which takes a few seconds. Snapshots live in `.echo/` in the working directory, so both steps must run in the same job and directory, and the second checkout must keep it (`clean: false`, since `.echo/snapshots/` isn't committed).

## Reports

The action writes Markdown and JUnit for you. To make them yourself, render the stored result again with `report`, which doesn't run anything:

```sh
rhythm-echo report --md >> "$GITHUB_STEP_SUMMARY"
rhythm-echo report --junit -o echo-junit.xml --fail-on new -q
rhythm-echo report --html -o echo-report.html -q
```

The Markdown starts with the verdict and a count table, then a table per class with the change, the actions, before → after and the fix:

```text
## ♪ Echo

**Something broke.** 3 actions compared with snapshot "before-update" (24 Oct 2026 02:17 UTC); this run sent 2 emails, 1 SMS, 1 chat message, 1 webhook.

| Broke | New | Changed | Fixed |
|---:|---:|---:|---:|
| 3 | 0 | 3 | 0 |

### ✗ BROKE (3)

_Sent in the snapshot, missing or failing now._

| Change | Actions | Before → after | Fix |
|---|---|---|---|
| Email to office@shop.test is no longer sent | Contact form | 1 → 0 | Something in the change stopped this message: … |
```

In JUnit, each class is a test suite (`echo.broke`, `echo.new`, `echo.changed`, `echo.fixed`) and each change a test case; cases at or above `--fail-on`, or BROKE and NEW without it, are failures. A run or snapshot has a suite per action (`echo.action:contact-form`), with a passing case `Contact form sends its messages` and a case per issue, failing at or above `--fail-on`. The action only attaches the JUnit file to the artifact; add a test reporter action after the Echo step, pointed at `${{ runner.temp }}/echo-report/echo-junit.xml`, to see it in GitHub's test views.

To keep the whole `.echo/runs/` folder (each run's JSON and HTML), upload it yourself:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: echo-runs
          path: .echo/runs/
```

> [!WARNING]
> Runs and reports hold the messages your site sent, with their content. On a site with real customer data, keep artifacts private and short-lived, or use test data on staging. See [Responsible use](Responsible-Use#what-echo-does).

## Failing a build

| Result | `--fail-on` values | Example |
|---|---|---|
| `compare` | `broke`, `new`, `changed` | `new` fails on BROKE or NEW |
| `run`, `snapshot`, `check`, `headers` | `error`, `warning`, `notice` | `warning` fails on errors or warnings |

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or no `--fail-on`) |
| `1` | A change or issue at or above `--fail-on`; also `doctor` with a failed check, and `replay` when the site didn't answer 2xx |
| `2` | Usage error: a bad flag or value, no snapshot to compare with, a refused replay |
| `3` | Couldn't run: Echo isn't running, a command failed to start, a report couldn't be written |
| `130` | Interrupted |

Start with `fail-on: broke`. CHANGED is often intended (a new subject line), so `changed` suits a team that reviews every message change before release.

## Other CI systems

Nothing above needs GitHub except the action itself. In GitLab CI, Jenkins or a cron job, install Echo, start it in the background, run your command:

```yaml
# .gitlab-ci.yml
echo:
  image: ubuntu:24.04
  script:
    - apt-get update && apt-get install -y curl
    - curl -fsSL https://rythm.sh/install | RHYTHM_TOOLS=echo sh
    - export PATH="$HOME/.local/bin:$PATH"
    - rhythm-echo up --plain > echo.log 2>&1 &
    - ./start-site.sh                     # with the connector pointed at 127.0.0.1:8025
    - rhythm-echo run --fail-on error --md -o echo.md
  artifacts:
    when: always
    paths: [echo.md, .echo/runs/]
```

`rhythm-echo` is the tool itself; `rhythm echo …` only forwards to it. In CI, Echo never prints the "new release" notice: it's off whenever the `CI` environment variable is set.

---
<sub>← [Deliverability](Echo-Deliverability) · [Home](Home) · [Configuration](Echo-Configuration) →</sub>
