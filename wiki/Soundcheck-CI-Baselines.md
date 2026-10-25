The usual setup for a team: snapshot the site from `main` after every deploy, then compare every pull request's preview with that snapshot and post the result on the pull request. The GitHub Action does both with two inputs: `baseline` keeps the snapshot between workflow runs in the Actions cache and `comment` posts one comment on the pull request and keeps it up to date. This page sets that up. The Action's other inputs are on [CI](Soundcheck-CI).

## Snapshot main

After the deploy job on `main`, take the snapshot and save it as the baseline `main`:

```yaml
# .github/workflows/soundcheck-baseline.yml
name: soundcheck baseline
on:
  push:
    branches: [main]

jobs:
  snapshot:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # …deploy, or wait for the deploy…
      - uses: RhythmShahriar/soundcheck@v1.2.0
        with:
          url: https://yoursite.com
          mode: snapshot
          baseline: main
```

With `mode: snapshot`, the snapshot is named after the baseline and `.soundcheck/snapshots/main` is saved to the Actions cache under the key `soundcheck-main-<run id>-<attempt>`. It's saved when the snapshot exits 0 or 1, so a site with a few known warnings still gets a baseline.

## Compare every preview and comment

Vercel, Netlify and similar hosts report each preview deploy as a `deployment_status` event with the preview's URL. Compare that preview with the baseline and comment on the pull request:

```yaml
# .github/workflows/soundcheck-preview.yml
name: soundcheck preview
on: deployment_status

permissions:
  contents: read
  pull-requests: write

jobs:
  compare:
    if: github.event.deployment_status.state == 'success'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: RhythmShahriar/soundcheck@v1.2.0
        with:
          url: ${{ github.event.deployment_status.target_url }}
          mode: compare
          baseline: main
          comment: true
          fail-on: broke
```

With `mode: compare`, the newest cache whose key starts with `soundcheck-main-` is restored and compared with `--snapshot main`. Pull requests can restore caches saved on their base branch, so a pull request against `main` finds it. Both workflows must use the same `working-directory`, since the cache path includes it.

For a staging site that isn't per pull request, run the compare on `pull_request` with a fixed `url` instead.

## The pull request comment

`comment: true` posts one comment on the pull request. Later runs update that same comment instead of adding new ones. It holds:

- a hidden marker, `<!-- rhythm-soundcheck -->`, on its first line;
- the Markdown report: the verdict, the counts table and a table per class (or, when Soundcheck couldn't check the site, the last lines of its error);
- a footer: `[Full report (HTML and JUnit)](…) · [Workflow run](…)`, linking the `soundcheck-report` artifact.

A comment over 60,000 bytes is cut at a line and ends `_The list is cut here to fit a comment; the full report has every change._`

On `deployment_status`, the Action finds the **open** pull request of the deployed commit. A run with no pull request, such as a push to `main`, posts nothing and logs:

```text
Soundcheck: no pull request for this run (event push), so no comment was posted.
```

The same holds for an `issue_comment` trigger on a plain issue. When the token can't comment (the job lacks `pull-requests: write`, or the pull request comes from a fork, whose token is read-only), the step still succeeds with a warning:

```text
Soundcheck: couldn't comment on the pull request. The job needs "permissions: pull-requests: write"; pull requests from forks only get a read-only token.
```

Only a comment that has the marker **and** was posted by the token's own user is updated. With the default `GITHUB_TOKEN` that user is `github-actions[bot]`. For a GitHub App token, set the `SOUNDCHECK_COMMENT_AUTHOR` environment variable to the app's bot login (for example `my-app[bot]`) and pass the token as `github-token`.

## Errors

These fail the step with exit 2 and go into the job summary:

| Message | Fix |
|---|---|
| `no baseline "main" was found in the cache. Run the Action with mode: snapshot and baseline: main on your main branch first; pull requests can restore caches saved on their base branch.` | run the baseline workflow once, or check the two workflows use the same `working-directory` |
| `the baseline input works with mode snapshot or compare, not against` | `baseline` only goes with `mode: snapshot` or `mode: compare` |
| `baseline "<b>" isn't a valid snapshot name: use letters, digits, dots, dashes and underscores` | pick another name |

Caches expire: GitHub removes a cache that isn't used for seven days. A busy repository keeps its baseline fresh by snapshotting on every push to `main`.

## Accepting changes from a pull request

When a pull request changes the site on purpose, the comment shows the changes as CHANGED. Merging the pull request deploys `main` and the next baseline snapshot takes the new look, so nothing needs accepting by hand. To quiet a known change on every pull request until then, add an `ignore:` rule with an `until` date to `.soundcheck/config.yaml` in the pull request. See [Review and accept](Soundcheck-Review-And-Accept#ignore).

---
<sub>← [CI](Soundcheck-CI) · [Home](Home) · [Configuration](Soundcheck-Configuration) →</sub>
