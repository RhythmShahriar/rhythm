Answers to common questions, fixes for common problems and the honest limits of Sightread. If yours isn't here, check the [CLI reference](Sightread-CLI-Reference) or open an issue (see [the end of this page](#reporting-a-bug)).

## Honest limits

These are the limits Sightread states in its README, its help and every report. Read them before you rely on a clean result.

1. **Sightread never runs the code it downloads.** `--soundcheck` is the one exception: it installs the update on a staging copy and loads it in a browser, so the new code runs there.
2. **No risk signals found means Sightread found no known risk signal in what it could analyse.** It is evidence for your review, not a guarantee that the update is safe. It can miss obfuscation it cannot decode, logic that waits for a date or a domain, code fetched at run time from a server that serves clean code to scanners, compiled assets and binaries and changes in a dependency of the package that Sightread did not review.
3. **Unanalysed means a file was too large or too strange to read.** It fails the gate by default because hiding code that way is a known trick.
4. **"Verified" means something different for each source,** and never "verified as safe". npm: matches the hash and signature the npm registry publishes. WordPress.org plugins: matches WordPress.org's own checksums (both come from WordPress.org, so this proves the copy was not changed on the way, not what the author intended). Themes: downloaded over TLS from WordPress.org; WordPress.org publishes no theme checksums. Composer: pinned to a commit; Packagist publishes no archive hash for most packages. See [Sources and integrity](Sightread-Sources-And-Integrity).
5. **A registry signature or provenance proves who published and from which workflow, not that the content is harmless.** A worm that publishes from a hijacked CI can carry valid provenance.
6. **Owner change detection depends on what the registry shows.** WordPress.org shows contributors and committers, not sales; a transfer that keeps the same account is invisible.
7. **Typosquat checks compare names with a list of popular packages;** a niche package can be squatted without a match.
8. **Each lookup tells that registry a package name and version you use,** and reports hold your full inventory. Private names stay local. See [Security model](Sightread-Security-Model#what-it-sends).
9. **A staging copy made by Roadie is pseudonymised, not anonymised.** Echo's seal stops mail, SMS and webhooks, not every outbound connection.
10. **A signed report proves it was not changed after signing and which key signed it.** A local key proves only "this machine".
11. **WordPress can install updates on its own** (auto-updates, forced security releases). A gate only covers updates you run by hand or through your tooling. With `--wp`, Sightread warns when plugin auto-updates are on.
12. **Sightread reports what changed. It does not judge intent.** Every report ends with that line.

## About Sightread

### How is this different from npm audit, composer audit or Dependabot?

They know published advisories: a version someone already reported. The updates that hurt most arrive as a normal, signed release with no advisory yet: a plugin bought and backdoored, a hijacked npm account, a typosquatted dependency. Sightread reads the update itself and says what it adds. It also asks OSV for known records (`known-malicious`, `known-vulnerable`), so it's never weaker than an audit for npm and Composer. Dependabot and Renovate open the pull request; Sightread reviews it (see [CI](Sightread-CI)).

### How is it different from Tuner?

[Tuner](Tuner) checks what a site exposes now: leaks, known vulnerabilities, weak config. Sightread reads what an update will change before you install it. They meet at "update available": Tuner says a plugin is vulnerable and an update is out, Sightread reads that update. `--from-tuner tuner.json` reads the installed versions from a Tuner report.

### Why does an old `eval` in a plugin not fire?

Sightread compares what the two versions can do. A signal fires only when a capability is in the new version and absent from the installed one. In one real Laravel vendor tree `unserialize(` appears in 28 packages and `eval(` in 11; flagging presence would flag almost every update. To review what you already run, use `--to installed --from <older version>`, or `all --rereview` across a site.

### Is it safe to run on production?

Yes for reviews: Sightread reads the site's files (or a `--wp` listing) and never changes them. With `--wp`, it runs only `plugin list`, `theme list`, `core version` and `option get auto_update_plugins`. `--soundcheck` installs updates, so it refuses a production site; see [With Soundcheck and Roadie](Sightread-With-Soundcheck#refusals).

### Does it work on premium plugins?

There is no official source to check a premium plugin against, so a plugin with its own `Update URI`, or whose files match nothing on WordPress.org, is listed under Not checked as `not-wporg` and never sent to WordPress.org. Give the vendor's zip with `--to-file` (and the installed one with `--from-file` if you have it); the update is reviewed and marked UNVERIFIED. See [Sources and integrity](Sightread-Sources-And-Integrity#local-files).

### What about WordPress core?

Out of scope in 1.0. Core is listed under Not checked (`WordPress core is out of scope in v1`, hint `WordPress core updates come signed from WordPress.org`) but it's informational: it never makes a run partial or fails `--fail-on`. Must-use plugins and drop-ins are listed the same way.

## Reading results

### Why is an update UNVERIFIED?

Sightread couldn't establish where its bytes came from: a local file (`--to-file`, `--from-file`, an installed premium plugin), WordPress.org checksums that were unavailable, or an archive that hit a limit. It isn't diffed, so you see no findings. It also makes the run incomplete. `--allow-unverified` reviews it anyway and keeps the UNVERIFIED verdict. Under `--fail-on`, an unverified update exits `3` unless `--allow-unverified`.

### What does "Incomplete review" mean?

Something wasn't checked: a package Not checked (premium, private, a git dependency), a fetch that failed or an update that couldn't be verified. The verdict box says what and the JSON has `complete: false`. Without `--fail-on` the run still exits `0`; with it, exit `3` unless `--allow-partial` (gaps) or `--allow-unverified` (unverified updates).

### What is "likely: a heuristic, not an exact pattern"?

The finding's confidence is `likely`: the detector matched a shape that is usually what it says, not an exact pattern. `--min-confidence confirmed` keeps likely findings out of the verdict and the gate (they stay in the JSON with `below_confidence: true`).

### What does "raised by owner-change" mean?

A context rule raised the finding one level. A new owner makes new code more suspicious, so code signals in the same update go up one level; two code signals in one function raise the lower one. See [Context rules](Sightread-Signal-Reference#context-rules).

### Why did `--severity critical` not make my gate pass?

`--severity`, `--id` and `--exclude-id` hide findings from the report only. Hidden findings still set the verdict and still count for `--fail-on`. The report says so: `note: 4 findings are hidden by --severity critical (they still count for the verdict and --fail-on)`. To stop a finding counting, add an `ignore` entry to the [config](Sightread-Configuration#ignore), an [allow entry](Sightread-Allow-And-Baseline) or a baseline.

### Why does it say "already the latest: nothing to compare"?

The installed version is the candidate. Use `--to <version>` to review another one, `--to list` to see them, or `--to installed --from <older>` to review what you run against the release before it.

### Why does `changes` say every package was added?

The lockfile isn't at the base ref, for example a new lockfile or a wrong `--base`: `<lockfile> is not at <base>, so every package in it counts as added`. Check `--base` and that the checkout has history (`fetch-depth: 0` in Actions).

## Errors

### `couldn't verify ... checksums answered 503`

The source didn't give Sightread what it needs to verify the bytes, so the update wasn't diffed. Run again later, or `--allow-unverified` to review it anyway (the report marks it unverified).

### `api.github.com allows 60 requests an hour without a token`

Composer packages hosted on GitHub and npm tag checks call api.github.com, which allows 60 requests an hour without a token. Sightread plans its requests before it starts, so this comes before the allowance is spent. Set `GITHUB_TOKEN` (sent only to api.github.com). In GitHub Actions the Action passes the job's token.

### `asked us to slow down (429)`

The registry rate-limited Sightread and still did after the retries. Run again in a few minutes, or lower `--jobs`.

### `can't reach ... (offline and not cached)`

With `--offline` (or `RHYTHM_OFFLINE` set) only cached data and archives are used and some weren't cached. Run once without it to fill the cache. Every offline run says so in its report: `Offline: only cached data and archives were used.`

### `no lockfile in ...`

`composer` and `npm` compare versions a lockfile pins. Run your package manager once to write one, or point `--path` at the folder that has it. `bun.lockb` is Bun's binary lockfile, which Sightread can't read: `bun install --save-text-lockfile` writes `bun.lock`.

### `--soundcheck needs rhythm-soundcheck and rhythm-roadie`

Install them with `rhythm install soundcheck roadie`, or pass `--skip-roadie-verify` (the report says the staging copy wasn't checked). See [With Soundcheck and Roadie](Sightread-With-Soundcheck).

### `report: ... is not valid JSON`

`report`, `explain` and `allow --from` read the JSON a review wrote. A file that was edited, cut short or written in another format is refused:

```text
$ rhythm sightread report before.json b04.json -o merged.md -o merged.sarif
rhythm sightread: b04.json is not valid JSON (invalid character 't' after top-level value)
  give the JSON a review wrote, such as -o sightread.json
```

### A config error

The config is checked before anything is fetched. Unknown keys name the nearest valid one; see [Configuration](Sightread-Configuration#errors).

## Doctor

`rhythm sightread doctor` checks what a review needs: each source answers, the npm registry's signing keys match the set pinned in the build, the cache is writable, `GITHUB_TOKEN`, `git` and, when the config has a `soundcheck` section, `rhythm-soundcheck` and `rhythm-roadie`. Here it ran against the testbed's mirrors on 127.0.0.1, so the sources are named by their mirror addresses:

```text
$ rhythm sightread doctor
  ✓ Config           .sightread/config.yaml
  ✓ Cache            .../.cache/rhythm/sightread is writable
  ✓ 127.0.0.1:18900  answers (127.0.0.1 answered 404)
  ...
  ✓ 127.0.0.1:18903  answers (127.0.0.1 answered 404)
  ✓ 127.0.0.1:18904  answers (127.0.0.1 answered 404)
  ✓ npm keys         1 key of 127.0.0.1 matches the set pinned in this build
  ✓ GitHub token     not set; the mirror at 127.0.0.1 reports 4999 of 5000 requests left this hour
  ✓ git              /usr/bin/git
  ✓ Signal data      sightread-names 2026.10.26, sightread-signals 2026.10.26

  14 checks passed, 0 failed.
```

The GitHub API here is the testbed's mirror, which sets its own limits. Against api.github.com without a token the line is a warning instead: `! GitHub token  not set, so 57 of 60 requests left this hour; set GITHUB_TOKEN for Composer packages and tag checks`.

`--offline` skips the network checks:

```text
$ rhythm sightread doctor --offline
ok    Config: .sightread/config.yaml
ok    Cache: .../.cache/rhythm/sightread is writable
skip  Sources: offline
warn  GitHub token: not set: api.github.com allows 60 requests an hour without one
ok    git: /usr/bin/git
ok    Signal data: sightread-names 2026.10.26, sightread-signals 2026.10.26

  4 checks passed, 0 failed.
```

A failed npm keys check (`<host> serves <n> signing keys this build does not pin`) means the registry rotated its keys: update Sightread. Exit `1` when a check fails.

## Reporting a bug

Open an issue at https://github.com/RhythmShahriar/sightread/issues with the command you ran, the output with `-v` and `rhythm sightread version`. Check the output for private package names and paths before you paste it. A false alarm or a miss on a real package is worth an issue: name the package and the two versions.

Found a security problem in Sightread itself? Don't open a public issue. Report it privately at https://github.com/RhythmShahriar/sightread/security/advisories/new. See the [Security policy](Security-Policy).

---
<sub>← [CLI reference](Sightread-CLI-Reference) · [Home](Home) · [Benchmark](Sightread-Benchmark) →</sub>
