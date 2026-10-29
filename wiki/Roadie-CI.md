Roadie runs the same in CI as on your machine: stable exit codes, a JSON report, no prompts once you name the target. The GitHub Action `RhythmShahriar/roadie@v1` installs a verified release, runs one mode and puts the result in the job summary. This page covers the Action's inputs and outputs, a nightly refresh, a verify gate, secrets and other CI systems.

## The Action

```yaml
- uses: RhythmShahriar/roadie@v1.0.0
  with:
    target: mysql://ci@127.0.0.1/staging
    production: shop.com
  env:
    ROADIE_TO_PASSWORD: ${{ secrets.STAGING_DB_PASSWORD }}
```

`mode` picks what runs: `verify` (the default), `copy`, `prepare` or `plan`. The Action installs the Roadie release that matches its own tag (`@v1.0.0` installs 1.0.0), checks `checksums.txt` with cosign and the archive's checksum before it runs anything, then runs Roadie with your inputs. Later Roadie steps in the same job reuse the install. Pin it by a release tag, or by a commit SHA with the version in a comment (`RhythmShahriar/roadie@<sha> # v1.0.0`); at `@v1` it installs the latest release and warns.

### Inputs

| Input | Default | What it does |
|---|---|---|
| `mode` | `verify` | `verify`, `copy`, `prepare` or `plan`. |
| `source` | `source:` in the config | Modes copy and plan: production, as `--from` takes it. |
| `target` | `target:` in the config | The staging copy: `--to` (copy, plan) or the staging argument (prepare, verify). |
| `url` | | The copy's URL. copy, prepare and plan rewrite to it; verify fetches its `robots.txt` and home page headers. |
| `production` | | Production hosts or URLs, comma-separated (`shop.com,*.shop.com`). Added to `production:`. |
| `confirm-target` | | Modes copy and prepare: the target's host or database name; for a SQLite target its absolute file path or the file's name (`staging.sqlite`). A site or database target needs it in CI. |
| `config` | `.roadie/config.yaml` | Settings file, relative to `working-directory`. |
| `working-directory` | `.` | Where Roadie runs, reads its config and writes `.roadie/reports`. |
| `fail-on` | `high` | Fail the job on findings at or above `info`, `low`, `medium`, `high` or `critical`; `none` never fails on findings. |
| `baseline` | | Mode verify: a JSON report whose findings are accepted, as `--baseline`. |
| `allow-sample` | `"false"` | `"true"` accepts a verify that checked a sample (`--sample` in `args`). By default a sampled verify counts as failed. |
| `sarif` | `"false"` | Mode verify: also write `roadie.sarif`. |
| `sarif-upload` | `"false"` | Upload `roadie.sarif` to code scanning (category `roadie`). Needs `sarif: "true"` and `security-events: write`. Warns in a public repository: alerts name tables and columns. |
| `args` | | Extra flags, split like a shell splits them (quotes and backslashes work) but never run through one: `$VAR`, `$(...)`, `~` and globs reach Roadie as written. |
| `version` | the Action's tag | A Roadie version to install, or `latest`. |
| `upload` | `"false"` | Upload `report.json`, `report.html` and `roadie.sarif` as the `roadie-report` artifact. Warns in a public repository. |
| `summary-details` | `"false"` | Put the full Markdown report in the job summary. By default the summary has counts only: no hosts, tables, columns or values. |
| `pr-comment` | `"false"` | Post the summary as one comment on the pull request and update it on later runs. Needs `pull-requests: write`. |
| `ssh-key` | | A private key for `ssh://` sources and targets, from a secret. Written to `~/.ssh/roadie_action_key` (0600) and removed when the Action ends, with `~/.ssh/config` and `known_hosts` put back as they were. |
| `known-hosts` | | Required with `ssh-key`: your hosts' keys as `ssh-keyscan` prints them, checked by hand and kept in a secret or the workflow. The Action never turns host key checking off. |

### Outputs

| Output | Value |
|---|---|
| `exit-code` | Roadie's exit code: 0, 1, 2, 3 or 130. |
| `verdict` | `pass`, `fail` or `error`: the report's `data.result` ([which is which](#pass-and-fail-in-each-output)). |
| `findings` | How many findings are at or above `fail-on`. |
| `report` | The path of the signed `report.json`. |
| `html` | The path of `report.html`. |
| `sarif-file` | The path of `roadie.sarif`, when `sarif` is `"true"`. |
| `signature` | The key ID that signed the report, e.g. `roadie:xSmAjAEGqcTF5wui`. Empty when unsigned. |
| `version` | The Roadie version that ran. |

### The job summary

The Action's run script, run on the production dump from [Getting started](Roadie-Getting-Started) with `mode: verify`, wrote this summary (counts only, the default):

```markdown
## Roadie: FAIL (verify)

| | |
|---|---|
| Findings | 2 critical · 16 high · 14 medium · 0 low · 0 info |
| Tables checked | 13 of 13 |
| Live keys found | 3 |
| Signed by | `roadie:xSmAjAEGqcTF5wui` |

Pseudonymised, not anonymised. The copy still holds order totals, dates, products and anything Roadie can't recognise in free text. Treat it as personal data.

_Counts only. Set summary-details: "true" for the full report here, or upload: "true" for the HTML report as an artifact._
```

and these outputs:

```text
exit-code=1
verdict=fail
findings=18
...
signature=roadie:xSmAjAEGqcTF5wui
version=1.0.0
```

The step fails when Roadie exits 1: `Roadie found something at or above --fail-on high. See the job summary.` A run that couldn't finish says `## Roadie: ERROR`; a verify that couldn't read all its input says `## Roadie: INCOMPLETE (verify)`.

### Pass and fail in each output

The Action's `verdict`, the report's `data.result` and the exit code say the same thing. The JSON's top-level `verdict` answers another question: whether there are findings at all, whatever `--fail-on` says.

| Run | Exit | Action `verdict` | `data.result` | JSON `verdict` | `complete` |
|---|---|---|---|---|---|
| Passed, no findings | 0 | `pass` | `pass` | `ok` | `true` |
| Passed, findings below `--fail-on` | 0 | `pass` | `pass` | `findings` | `true` |
| Findings at or above `--fail-on` | 1 | `fail` | `fail` | `findings` | `true` |
| A verify that checked a sample | 0 | `fail` (`pass` with `allow-sample: "true"`) | `pass` | `ok` or `findings` | `false` |
| INCOMPLETE: a verify that couldn't read all its input | 3 | `error` | `error` | `error` | `false` |
| ERROR: stopped, unreachable, usage error | 2 or 3 | `error` | `error` | `error` | `false` |

A script that reads `report.json` itself should treat a run as passed only when `complete` is `true` and `data.result` is `pass`; add JSON `verdict` `ok` to that when findings below `--fail-on` must fail it too.

## Refresh staging every night

```yaml
name: staging refresh
on:
  schedule: [{cron: "0 3 * * *"}]
  workflow_dispatch:
permissions: {contents: read}
jobs:
  staging:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: RhythmShahriar/roadie@v1.0.0
        with:
          mode: copy
          source: ssh://deploy@shop.com/var/www/shop
          target: ssh://deploy@staging.shop.com/var/www/staging
          url: https://staging.shop.com
          confirm-target: staging.shop.com
          ssh-key: ${{ secrets.DEPLOY_KEY }}
          known-hosts: ${{ secrets.KNOWN_HOSTS }}
          args: --yes
        env:
          ROADIE_SIGNING_KEY: ${{ secrets.ROADIE_SIGNING_KEY }}
          ROADIE_KEY: ${{ secrets.ROADIE_KEY }}                          # stable fakes across refreshes
          ROADIE_STRIPE_TEST_SECRET: ${{ secrets.STRIPE_TEST_SECRET }}  # read by keys.test_keys
      - uses: RhythmShahriar/soundcheck@v1
        with: {url: https://staging.shop.com, mode: run}
```

The copy runs the whole pipeline over ssh: production is exported once and spooled encrypted on the runner, the staging is replaced and verified. The signed report is in `.roadie/reports/`. With `ROADIE_KEY` set and `consistency: stable` in the config, each customer keeps the same fake every night. Run Echo on the staging host so the copy's mail is caught, not lost.

Refreshing a staging that a host panel copies for you? Use `mode: prepare` with `target` and `confirm-target` after the panel's copy step. After a plugin update on staging, `mode: prepare` with `args: --only side-effects` puts the kill switch back.

## Gate a pull request

Check a staging copy (or a fixture dump committed for tests) on every push, with code scanning:

```yaml
permissions: {contents: read, security-events: write}
steps:
  - uses: actions/checkout@v5
  - uses: RhythmShahriar/roadie@v1.0.0
    with:
      target: tests/fixtures/staging.sql
      production: shop.com
      sarif: "true"
      sarif-upload: "true"
```

Check that a change to `.roadie/config.yaml` still covers every column, without copying anything:

```yaml
  - uses: RhythmShahriar/roadie@v1.0.0
    with:
      mode: plan
      source: tests/fixtures/prod-schema.sql
      fail-on: medium
      args: --schema-only
```

`plan --fail-on medium` exits 1 when the copy would stop (an uncovered column) or would leave a finding at medium or above.

**Forks.** `copy` and `prepare` are refused in a pull request from a fork, where the workflow would need production's credentials:

```text
::error::Roadie could not run: Roadie won't copy production in a pull request from a fork: the workflow would need production credentials. Run it on push or schedule; don't use pull_request_target for this.
```

`verify` and `plan` on files in the repository are fine there.

## Secrets

| Variable | Use |
|---|---|
| `ROADIE_SIGNING_KEY` | The report signing key: base64 of a 32-byte Ed25519 seed. Without it every runner makes a new key and reports can't be pinned to one signer. Make one with `head -c 32 /dev/urandom \| base64`. |
| `ROADIE_KEY` | The consistency key (base64 of 32 bytes) for `consistency: stable`. It can link fakes back to people: keep it as secret as production's database password. |
| `ROADIE_FROM_PASSWORD`, `ROADIE_TO_PASSWORD` | DSN passwords. |
| `ROADIE_*` | Anything the config reads with `${ROADIE_NAME}`: test keys, an admin password. |
| `RHYTHM_ECHO_TOKEN` | Echo's token, for `verify --echo` against an Echo beyond loopback. |

The config never holds a secret and only `ROADIE_` variables are read, so a config change can't pull another secret out of the runner. Roadie never prints a password or key; `--basic-auth` on GitHub Actions masks the password and keeps it in `.roadie/basic-auth/password` instead of printing it.

## Other CI systems

Install `rhythm` and Roadie as on any machine ([CI setup](CI-Setup)), then run the command. Without a terminal Roadie never asks: a site or database target needs `--confirm-target <host> --yes`, output is plain text and the update check is off when `CI` is set.

```sh
rhythm install roadie
rhythm roadie verify "$STAGING_DSN" --production shop.com --fail-on high --json -o roadie.sarif -o roadie.xml > roadie.json
```

| Exit | Means |
|---|---|
| 0 | done; verify found nothing at or above `--fail-on` |
| 1 | findings at or above `--fail-on`; a copy stopped on uncovered columns; `plan --fail-on` would stop; `report check` found an edited report, another input or another key |
| 2 | usage error: a wrong command line, a bad `.roadie/config.yaml`, a signing key that can't be read, a site or database target without `--confirm-target` and no terminal |
| 3 | couldn't run: unreachable, a dump that ends early, a target refused as production or as the source, nothing to scan, an incomplete verify, an unsigned report (`report check`) |
| 130 | interrupted |

`report.json`'s `complete` is `false` for a sampled or incomplete verify: treat only `complete: true` with `data.result: "pass"` as passed ([the table above](#pass-and-fail-in-each-output) maps each output). `rhythm roadie report check report.json --key roadie:<id> --input <dump>` checks a report later, in another job or by an auditor.

---
<sub>← [Multisite](Roadie-Multisite) · [Home](Home) · [Configuration](Roadie-Configuration) →</sub>
