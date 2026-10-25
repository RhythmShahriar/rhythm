Run Echo on every pull request or deploy and fail the pipeline when a message stops being sent. This page covers the GitHub Action with its modes, inputs and outputs, the job summary and the pull request comment, starting Echo for Soundcheck with `mode: up`, comparing with a baseline, the reports and other CI systems.

Echo has to run where the site's connector can reach it. In CI that usually means the site runs in the job too, with Docker Compose, `php artisan serve`, `next start` or similar. Its connector points at `http://127.0.0.1:8025`. To check a staging server, run `rhythm echo up` on that server and its actions from there.

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

      - uses: RhythmShahriar/echo@v1.1.1
        with:
          mode: run
```

The Action runs on Linux and macOS runners (`amd64` or `arm64`); on Windows it stops at once with `the Echo Action runs on Linux and macOS runners, not <system>`. It downloads the Echo release, checks `checksums.txt` against its cosign signature from Echo's release workflow for that version, then the archive against `checksums.txt`. It stops if either doesn't match (`Echo's release didn't verify: …`). It starts `rhythm-echo up` in the background, sealed. Unless the mode is `up`, it then runs the actions in your committed `.echo/config.yaml` with `--plain`, writes the reports and stops Echo. It writes:

- a **job summary**, with recipients, numbers and quoted text masked (below);
- with `upload: true`, `echo-report.html` and `echo-junit.xml` into an artifact named **`echo-report`**;
- when the step fails or finds something, Echo's log as the **`echo-log`** artifact, masked the same way;
- with `pr-comment: true`, one comment on the pull request, updated on later runs;
- an error annotation and a failed job when the result meets `fail-on`.

Pin the Action to a release tag (`@v1.1.1`) or to that tag's commit SHA: it then installs that same Echo release. Used by `@v1`, a branch or another commit, it installs the latest release and warns that a new release could change your build; `version` sets it explicitly.

### Modes

| Mode | Does |
|---|---|
| `run` (default) | Starts Echo, runs the actions once, checks the messages, stops Echo. |
| `snapshot` | The same, saving the result as snapshot `name` for a `compare` later in the job. |
| `compare` | Runs the actions again and compares with the latest snapshot. |
| `up` | Starts Echo and leaves it running for later steps, such as Soundcheck with `--echo`. Writes a short summary saying where Echo runs. |
| `stop` | Stops the Echo a `mode: up` step started and reports the latest run in `working-directory`, like the other modes. |

### Inputs

| Input | Default | Description |
|---|---|---|
| `mode` | `run` | `up`, `snapshot`, `compare`, `run` or `stop`. Anything else exits `2` with `mode must be snapshot, compare, run, up or stop, not "<mode>"`. |
| `working-directory` | `.` | The folder with `.echo/`, relative to the workspace, such as `sites/shop` in a monorepo. |
| `config` | `""` | A config file relative to `working-directory`, passed as `--config`. |
| `name` | `ci` | Snapshot name, for mode `snapshot`. |
| `fail-on` | `broke` for `compare`, `error` otherwise | Fail on changes at or above `broke`, `new` or `changed`, or on issues at or above `error`, `warning` or `notice`. `none` never fails. Any case. |
| `seal` | `"true"` | `"true"` seals Echo: nothing leaves, Echo answers for every service. `"false"` observes: everything is recorded and sent for real. Anything else fails the step with `seal must be true or false, not "…"` (exit `2`). |
| `listen` | `127.0.0.1:8025` | The API address. Use `0.0.0.0:8025` when the site runs in a service container; a token is then generated (the `token` output). |
| `smtp` | `127.0.0.1:1025` | The SMTP sink address, or `off`. Use `0.0.0.0:1025` for a site in a service container; the sink then asks for the token as the SMTP password (any username). |
| `args` | `""` | Extra flags for the command, split like a shell would (quotes group words) but never run through one, e.g. `--action 'Contact form' --verbose`. An unclosed quote is a usage error. |
| `version` | the Action's own release | Echo version to install, e.g. `1.1.1`, or `latest`. |
| `upload` | `"false"` | Upload the HTML and JUnit reports as the `echo-report` artifact. Off by default: the HTML report holds every captured recipient, subject and body. Anyone can download a public repository's artifacts. The Action warns when it's on in a public repository. |
| `log` | `"true"` | Upload Echo's log as `echo-log` when the step fails or finds something. |
| `summary-details` | `"false"` | Show recipients, subjects and before and after values in the summary, the comment and the log. |
| `pr-comment` | `"false"` | On a pull request, post one comment with what broke and what is new, updated on every later run. Needs `permissions: pull-requests: write`; a pull request from a fork gets a read-only token, so the Action warns and carries on. |

A token is generated whenever `listen`, or `smtp` when it isn't `off`, is beyond loopback. Loopback is `127.*`, `localhost` and `::1` (also written `[::1]:8025`). Echo reads its other settings (the proxy, the allowlist, the actions) from `.echo/config.yaml`, so commit that file. The `listen` and `smtp` inputs win over the config's.

### Outputs

| Output | Description |
|---|---|
| `url` | Echo's API as the site under test reaches it, for its `RHYTHM_ECHO_URL`: `http://127.0.0.1:8025` by default, the runner's own address when `listen` is `0.0.0.0`, so a service container can reach it. |
| `smtp` | The SMTP sink as the site reaches it (`host:port`), the same way; empty when `smtp` is `off`. |
| `token` | The token connectors must send when Echo listens beyond loopback; masked in logs. Empty on loopback. |
| `exit-code` | The command's exit code: `0` ok, `1` `fail-on` hit, `2` usage error (flags or `.echo/config.yaml`), `3` couldn't run, `130` interrupted. |
| `report` | Path of the standalone HTML report; for `mode: up`, where the later `mode: stop` step writes it. |
| `summary` | Path of a copy of the job summary (Markdown). |
| `comment-id` | ID of the pull request comment posted or updated. |

Later steps on the runner also get `RHYTHM_ECHO_URL` (always on loopback) and `RHYTHM_ECHO_TOKEN` when there's a token. A job summary is written whatever happens: on exit `2` it starts **Echo didn't run: the command line or the config has a mistake (exit 2).**, on `3` **Echo couldn't run (exit 3).**, followed by the error lines and the last 30 lines of Echo's log, masked. On `1`, the reports are written first, then the job fails with `Echo found changes or issues at or above --fail-on <level>. See the job summary.`

### The job summary

Anyone who can read the repository can read a job summary, so by default it has counts, change titles and action names, with email addresses reduced to their domain and numbers and quoted text masked. For the comparison of [Your first run](Echo-First-Run):

```text
## ♪ Echo

**Something broke.** 2 actions compared with snapshot "before-update".

| Broke | New | Changed | Fixed |
|---:|---:|---:|---:|
| 4 | 1 | 1 | 0 |

### ✗ BROKE (4)

| Change | Actions |
|---|---|
| Email to ***@shop.example.com is no longer sent | Contact form |
| Sent fewer emails than expected | Contact form |
| Unfilled placeholder {customer_first_name} in email | Checkout |
| WooCommerce webhook to gate ("***"): payload lost fields | Checkout |

### ▲ NEW (1)

| Change | Actions |
|---|---|
| SMS needs more than one part | Checkout |

### ■ CHANGED (1)

| Change | Actions |
|---|---|
| Twilio SMS to the test number: text changed | Checkout |

<sub>Recipients, subjects and message contents are hidden here. Set `summary-details: true` to show them, or open the HTML report.</sub>
```

With `summary-details: true` the summary is the full `rhythm-echo report --md` ([below](#reports)). With `pr-comment: true`, the comment says what broke and what's new and links to the run:

```text
## ♪ Echo

**Something broke.** Compared with snapshot "before-update": 4 broke, 1 new, 1 changed.

**✗ BROKE (4)**

- Email to ***@shop.example.com is no longer sent (Contact form)
- Sent fewer emails than expected (Contact form)
- Unfilled placeholder {customer_first_name} in email (Checkout)
- WooCommerce webhook to gate ("***"): payload lost fields (Checkout)

**▲ NEW (1)**

- SMS needs more than one part (Checkout)

<sub>Everything else is in the job summary of [this run](https://github.com/you/shop/actions/runs/123).</sub>
```

The Action edits only a comment its own account wrote; a comment anyone else posts is left alone. Both were rendered by the Action's own script from that comparison's `report --json`.

### A site in a container

A site in a container can't reach the runner's loopback, so let Echo listen on every address, start Echo before the site and give the site the `url`, `smtp` and `token` outputs:

```yaml
      - uses: RhythmShahriar/echo@v1.1.1
        id: echo
        with:
          mode: up
          listen: 0.0.0.0:8025
          smtp: 0.0.0.0:1025

      - name: Start the site
        run: docker compose up -d --wait    # passes these to the app's environment
        env:
          RHYTHM_ECHO_URL: ${{ steps.echo.outputs.url }}
          RHYTHM_ECHO_SMTP: ${{ steps.echo.outputs.smtp }}
          RHYTHM_ECHO_TOKEN: ${{ steps.echo.outputs.token }}

      # …run the actions or Soundcheck…

      - uses: RhythmShahriar/echo@v1.1.1
        if: always()
        with:
          mode: stop
```

The step's `env:` wins over the loopback `RHYTHM_ECHO_URL` the Echo step exported. The SMTP sink asks for a login: any username, with the token as the password. GitHub's `services:` containers start before any step, so they can't take these outputs; start the site in a step instead, or run Echo itself as a service from its image (see [Docker and DDEV](Echo-Docker-And-DDEV)).

## With Soundcheck

Start Echo in `up` mode, then let Soundcheck submit the forms with `--echo`:

```yaml
    steps:
      - uses: actions/checkout@v4
      - name: Start the site
        run: docker compose up -d --wait

      - uses: RhythmShahriar/echo@v1.1.1
        with:
          mode: up

      - uses: RhythmShahriar/soundcheck@v1
        with:
          url: http://127.0.0.1:8080
          mode: run
          args: --level inspect,click,submit --echo
```

Echo keeps running until the job ends, or until a `mode: stop` step. Soundcheck reads Echo's token from `RHYTHM_ECHO_TOKEN`, which the Echo step exported. With another `listen` address, pass it on: `--echo=${{ steps.echo.outputs.url }}` (give the Echo step `id: echo`). Its messages appear in Soundcheck's own reports; see [With Soundcheck](Echo-With-Soundcheck). Echo's log is in `$RUNNER_TEMP/echo.log`.

To compare what forms send before and after a deploy in one job, take a Soundcheck snapshot with `--echo`, deploy, then compare with `--echo`, as in [Soundcheck's CI guide](Soundcheck-CI#snapshot-deploy-compare-in-one-job).

## Snapshot, change, compare in one job

```yaml
    steps:
      - uses: actions/checkout@v4
        with: {ref: "${{ github.event.pull_request.base.sha }}"}
      - run: docker compose up -d --wait

      - name: Snapshot the base
        uses: RhythmShahriar/echo@v1.1.1
        with:
          mode: snapshot
          name: base
          fail-on: none

      - uses: actions/checkout@v4
        with: {clean: false}
      - run: docker compose up -d --wait --build

      - name: Compare the pull request
        uses: RhythmShahriar/echo@v1.1.1
        with:
          mode: compare
```

Each Echo step installs the release again, which takes a few seconds. Snapshots live in `.echo/` next to the config, so both steps must run in the same job and directory. The second checkout must keep it (`clean: false`, since `.echo/snapshots/` isn't committed).

### A baseline from another job

To compare with a snapshot taken elsewhere, such as a nightly job on the main branch, export it there and compare against the file here:

```sh
rhythm-echo snapshot --export base.tgz                 # in the job that has the snapshot
rhythm-echo compare --against base.tgz --fail-on broke # in the job that compares
```

Pass the file between jobs as an artifact (keep it private: it holds what the site sent). `--against` reads it without adding it to `.echo/snapshots/`; `snapshot --import` adds it. See [Baselines that travel](Echo-Actions-And-Compare#baselines-that-travel).

## Reports

The Action writes the summary and, with `upload`, the HTML and JUnit files. To make them yourself, render the stored result again with `report`, which doesn't run anything:

```sh
rhythm-echo report --md >> "$GITHUB_STEP_SUMMARY"
rhythm-echo report --junit -o echo-junit.xml --fail-on new -q
rhythm-echo report --html -o echo-report.html -q
rhythm-echo report --json -o echo.json -q
```

The Markdown starts with the verdict and a count table, then a table per class with the change, the actions, before → after and the fix:

```text
## ♪ Echo

**Something broke.** 2 actions compared with snapshot "before-update" (25 Oct 2026 14:22 UTC); this run sent 2 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events.

| Broke | New | Changed | Fixed |
|---:|---:|---:|---:|
| 4 | 1 | 1 | 0 |

### ✗ BROKE (4)

_Sent in the snapshot, missing or failing now._

| Change | Actions | Before → after | Fix |
|---|---|---|---|
| Email to owner@shop.example.com is no longer sent | Contact form | sent → not sent | Something in the change stopped this message: check the plugin or code that sends it and its settings. |
| Sent fewer emails than expected<br><sub>email: expected 2, got 1</sub> | Contact form |  | Check the settings of whatever sends them, or update expect in .echo/config.yaml if the change is intended. |
```

Text and payload changes get a collapsed `Diff:` block with the hunks. Captured text in Markdown and JUnit has backticks, brackets and control characters escaped, so a subject can't open a link or a code span in a comment.

In JUnit, each class is a test suite (`echo.broke`, `echo.new`, `echo.changed`, `echo.fixed`) and each change a test case; cases at or above `--fail-on`, or BROKE alone without it, are failures. A run or snapshot has a suite per action (`echo.action:contact-form`), with a passing case `Contact form sends its messages` and a case per issue, failing at or above `--fail-on`. To see it in GitHub's test views, upload it and add a test reporter action after the Echo step.

To keep the whole `.echo/runs/` folder (each run's JSON and HTML), upload it yourself:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: echo-runs
          path: .echo/runs/
          retention-days: 7
```

> [!WARNING]
> Runs and reports hold the messages your site sent, with their content. On a site with real customer data, keep artifacts private and short-lived, or use test data on staging. Reports are written readable by the runner's user only, which an artifact upload is. See [Responsible use](Responsible-Use#what-echo-does).

## Failing a build

| Result | `--fail-on` values | Default | Example |
|---|---|---|---|
| `compare` | `broke`, `new`, `changed`, `none` | `broke` | `new` fails on BROKE or NEW |
| `run`, `snapshot`, `check`, `headers` | `error`, `warning`, `notice`, `none` | `error` | `warning` fails on errors or warnings |

`run`, `snapshot` and `compare` also fail when an action fails, unless `--fail-on none`. The exit codes:

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` |
| `1` | A change or issue at or above `--fail-on`; an action failed; also `doctor` with a failed check and `replay` when the site didn't answer 2xx |
| `2` | Usage error: a bad flag or value, an unknown action, no config or no snapshot to compare with, a refused replay |
| `3` | Couldn't run: Echo isn't running, every action failed, the DNS resolver never answered, a report couldn't be written |
| `130` | Interrupted |

Start with the defaults. CHANGED is often intended (a new subject line), so `changed` suits a team that reviews every message change before release; `--accept <key>` takes one you meant.

> [!TIP]
> On GitHub Actions, `uses: RhythmShahriar/rhythm/setup@v0.7.0` with `tools: echo` installs rhythm and Echo with signatures checked, caches the downloads and puts them on `PATH`; it can also install the versions a `rhythm.lock` pins. See [CI setup](CI-Setup).

## Other CI systems

Nothing above needs GitHub except the Action itself. In GitLab CI, Jenkins or a cron job, install Echo, start it in the background, run your command:

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
    - rhythm-echo run --md -o echo.md
  artifacts:
    when: always
    paths: [echo.md, .echo/runs/]
```

Or run Echo as a service from its image, `ghcr.io/rhythmshahriar/echo:1`. `rhythm-echo` is the tool itself; `rhythm echo …` only forwards to it. In CI, Echo never prints the "new release" notice: it's off whenever the `CI` environment variable is set. `--json` on any command writes the same envelope, for a script that reads the result; see [CLI reference](Echo-CLI-Reference#json-output).

---
<sub>← [Conformance kit](Echo-Conformance) · [Home](Home) · [Configuration](Echo-Configuration) →</sub>
