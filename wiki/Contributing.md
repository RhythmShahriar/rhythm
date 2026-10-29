Contributions are welcome. New Tuner checks, Soundcheck testbed scenarios, Echo recognizers, Roadie profiles and Sightread signals with their fixtures are the most welcome of all. This page covers setting up the code, the commit message rules the hooks enforce, how pull requests are reviewed and merged and where to start with a check.

For big changes, open an issue first so the approach can be agreed before you write the code. Keep each pull request focused on one thing.

## Repositories

| Repository | What's in it |
|---|---|
| [`RhythmShahriar/rhythm`](https://github.com/RhythmShahriar/rhythm) | The `rhythm` CLI, the install scripts and their tests, the `setup` GitHub Action and `kit/`, the shared base every tool imports |
| [`RhythmShahriar/tuner`](https://github.com/RhythmShahriar/tuner) | Tuner (`rhythm-tuner`) and its checks in `checks/` |
| [`RhythmShahriar/soundcheck`](https://github.com/RhythmShahriar/soundcheck) | Soundcheck (`rhythm-soundcheck`), its modules in `checks/`, the GitHub Action (`action.yml`) and the testbed in `testbed/` |
| [`RhythmShahriar/echo`](https://github.com/RhythmShahriar/echo) | Echo (`rhythm-echo`), its recognizers in `recognize/`, the connectors in `connectors/` (WordPress, Laravel, Node), the hard-seal recipes in `docs/seal/` and the GitHub Action |
| [`RhythmShahriar/echo-laravel`](https://github.com/RhythmShahriar/echo-laravel) | Read-only: Echo's `connectors/laravel`, split out for Packagist. Send changes to the `echo` repository. |
| [`RhythmShahriar/roadie`](https://github.com/RhythmShahriar/roadie) | Roadie (`rhythm-roadie`), its profiles in `profile/`, detectors in `detect/`, the GitHub Action and the Docker testbed in `testbed/` |
| [`RhythmShahriar/sightread`](https://github.com/RhythmShahriar/sightread) | Sightread (`rhythm-sightread`), its detectors in `detect/` with fixture pairs in `detect/pairs/testdata/pairs/<signal-id>/`, the GitHub Action and the testbed in `testbed/` |
| [`RhythmShahriar/.github`](https://github.com/RhythmShahriar/.github) | Shared community files, the reusable CI and release workflows and the scripts they run (`check-style.py`, `add-headers.py`, `check-kit-pin.sh`) |

## Set up

You need **Go 1.27.3 or newer**, as set by the `go` line in each `go.mod`. With a recent Go, the right toolchain downloads automatically if yours is older. `make check` also needs network access the first time, to fetch `golangci-lint`.

### Working on Tuner only

Tuner builds on its own, using the released `rhythm` kit from its `go.mod`:

```sh
git clone https://github.com/RhythmShahriar/tuner && cd tuner
make hooks   # enable the commit message check
make check   # lint + tests, every check against its own fixtures
make build   # bin/rhythm-tuner
```

### Working on Soundcheck only

Soundcheck builds on its own too. Its browser tests need Chrome or Chromium installed:

```sh
git clone https://github.com/RhythmShahriar/soundcheck && cd soundcheck
make hooks   # enable the commit message check
make check   # lint + tests (browser tests need Chrome or Chromium)
make build   # bin/rhythm-soundcheck
```

### Working on Echo only

Echo builds on its own too. Its connectors have their own tests, in their own languages:

```sh
git clone https://github.com/RhythmShahriar/echo && cd echo
make hooks                         # enable the commit message check
make check                         # lint + Go tests
make build                         # bin/rhythm-echo
make -C connectors/laravel check   # php -l, Larastan level 6, PHPUnit + Testbench (needs Docker)
make -C connectors/laravel e2e     # a fresh Laravel app against Echo's real catcher (Go, Docker, jq)
(cd connectors/node && npm ci && npm test)   # builds, then vitest; npm run test:e2e for a Next.js app
go test -tags docker ./connectors/wordpress ./proxy ./sendmail   # the mu-plugin in a real WordPress, the seal recipes, PHP mail() (Docker)
```

The Docker tests use their own containers and ports and remove them afterwards.

### Working on Roadie only

Roadie builds against released kit and Echo versions, with `GOWORK=off`:

```sh
git clone https://github.com/RhythmShahriar/roadie && cd roadie
make hooks     # Conventional Commits check
make check     # lint + tests (includes the dump-level testbed scenarios)
make fuzz      # every fuzz target, briefly
make -C testbed up bench   # the sealed Docker testbed and its scoreboard
```

Every profile rule and detector needs a positive and a negative fixture. Fake keys in tests are assembled at run time. See the repository's `CONTRIBUTING.md`.

### Working on Sightread only

Sightread builds against released kit versions, with `GOWORK=off`:

```sh
git clone https://github.com/RhythmShahriar/sightread && cd sightread
make hooks     # Conventional Commits check
make check     # lint + tests
make testbed   # the fixture registries on 127.0.0.1, for end-to-end tests and the benchmark
make perf      # the speed targets (run it on an idle machine)
```

Every signal needs at least three positive and three negative fixture pairs, one of them a benign look-alike; `TestEverySignalHasFixtures` fails the build otherwise. See the repository's `CONTRIBUTING.md`.

### Working on rhythm, or on the kit and a tool together

Clone the repositories side by side and link them with a Go workspace, so a tool builds against your local copy of `kit/`:

```sh
mkdir rhythmsh && cd rhythmsh
git clone https://github.com/RhythmShahriar/rhythm
git clone https://github.com/RhythmShahriar/tuner
git clone https://github.com/RhythmShahriar/soundcheck
git clone https://github.com/RhythmShahriar/echo
go work init ./rhythm ./tuner ./soundcheck ./echo
```

```text
rhythmsh/
├── go.work      # use ( ./rhythm ./tuner ./soundcheck ./echo )
├── echo/
├── rhythm/
├── soundcheck/
└── tuner/
```

Keep `go.work` outside the repositories; it's local to your machine and isn't committed. Build both from the workspace root:

```sh
go build -o bin/rhythm ./rhythm/cmd/rhythm
go build -o bin/rhythm-tuner ./tuner/cmd/rhythm-tuner
go build -o bin/rhythm-soundcheck ./soundcheck/cmd/rhythm-soundcheck
go build -o bin/rhythm-echo ./echo/cmd/rhythm-echo
```

To build a tool against the kit version in its `go.mod` instead of your local copy, set `GOWORK=off`.

Then run `make hooks` and `make check` in each repository you change.

### Make targets

| Target | Does |
|---|---|
| `make hooks` | Points git at `.githooks/`, enabling the commit message check |
| `make check` | `lint` + `test`, what you should run before every push |
| `make test` | `go test -race ./...` (Soundcheck's browser tests need Chrome or Chromium) |
| `make lint` | `golangci-lint` v2.15.0 |
| `make fmt` | `gofmt -w .` |
| `make build` | `bin/rhythm`, `bin/rhythm-tuner`, `bin/rhythm-soundcheck` or `bin/rhythm-echo` |
| `make pack` | Tuner only: builds the check pack into `dist/checks-pack`, byte for byte as CI publishes it |
| `make testbed`, `make testbed-down` | Soundcheck and Echo: start or delete the testbed in Docker ([Soundcheck's](#soundcheck-and-the-testbed), [Echo's](#echo-and-its-connectors)) |
| `make bench` | Soundcheck and Echo: build, then run the benchmark ([Soundcheck](Soundcheck-Benchmark), [Echo](Echo-Benchmark)) against the testbed |

`rhythm` has a few more:

| Target | Does |
|---|---|
| `make check` | `lint`, `test` and `cross`: the same as CI's Go job |
| `make cross` | `GOOS=windows go vet ./...`, compiles the router's tests for Windows and vets for macOS, so a Unix-only helper in a test fails on your machine and not only on CI |
| `make test-installers` | the installer tests: `sh scripts/test/run.sh`, then `run.ps1` when `pwsh` is on your `PATH` |
| `make actionlint`, `make pins` | lint the workflows; check that every third-party action is pinned to a commit SHA |
| `make vuln` | `govulncheck` over the code rhythm calls |
| `make release-check`, `make dist-check`, `make trusted-root` | the release config, a checked snapshot release and the Sigstore root; see [Release process](Release-Process#before-a-rhythm-release) |

### Installer tests

`scripts/install.sh` and `scripts/install.ps1` have their own tests in `scripts/test/`, against a fake GitHub API on `127.0.0.1` (`server.py`) with fake releases (`mkfake.sh`): checksum and signature failures, mirrors and tokens, timeouts, pins, `rhythm.lock`, the PATH hint and a script cut off halfway.

```sh
sh scripts/test/run.sh                          # every case, install.sh under sh
RHYTHM_TEST_SH=dash dash scripts/test/run.sh    # the same under dash (also bash, "busybox sh")
sh scripts/test/run.sh pins locked              # only some cases
pwsh -NoProfile -File scripts/test/run.ps1      # install.ps1; needs go and python3
```

They need `python3`, `curl` and `tar`. `RHYTHM_TEST_KEEP=1` keeps each case's folder for a look afterwards. A real `cosign` or `gh` on your `PATH` is hidden from the cases.

## CI

Every pull request runs:

| Job | Where | What |
|---|---|---|
| `go` | Ubuntu, Windows and macOS (`rhythm`; the tools on Ubuntu) | `go vet`, `go test -race`, a build and, for `rhythm`, the crawl benchmark once. On Linux also `gofmt`, `golangci-lint`, `shellcheck`, `actionlint`, the pinned-actions check (`rhythm`), the kit pin check and `goreleaser check`. |
| `govulncheck` | Ubuntu | fails on a known vulnerability in code the module calls |
| `commitlint` | Ubuntu | every commit message and the pull request title ([Commit messages](#commit-messages)) |
| `installers` | `rhythm`: Ubuntu, macOS, Windows | the installer tests under `sh`, `dash` and `bash`, PowerShell 7 and Windows PowerShell 5.1 |
| `packages` | `rhythm`: Ubuntu | a snapshot release, then its SBOMs, Linux packages and package-manager manifests are checked |
| `setup-rhythm-test` | `rhythm`: Ubuntu, macOS, Windows | the [setup-rhythm action](CI-Setup) against a fake release API, its cache and its inputs |

Windows runners check out with `core.autocrlf=true`; the repository's `.gitattributes` keeps text files and signed test fixtures byte for byte, so write new fixtures with LF line ends. A tool fails the **kit pin** check when its `go.mod` requires a kit more than one minor release behind the latest `rhythm` tag:

```text
check-kit-pin: tuner/go.mod pins github.com/RhythmShahriar/rhythm v0.1.0 but the latest is v0.6.0; move to v0.5.0 or later
```

Bump the kit (see [When the kit changes](Release-Process#when-the-kit-changes)). A new workflow must pin every third-party action to a full commit SHA, with the version in a comment, or the pinned-actions check lists it: `These uses: lines don't pin a 40-hex commit SHA:`.

## Writing rules and file headers

Docs, help text, messages and comments follow two writing rules: **no em dashes** and **no comma before "and" or "but"**. Write `checks the archive and the signature`, not `checks the archive, and the signature`; use two sentences when one gets long. A line that must keep the pattern (quoted third-party text) opts out with `style:ignore` anywhere on it. Check a repository with the script from `RhythmShahriar/.github`:

```sh
python3 ../.github/scripts/check-style.py .      # from inside the repository you changed
```

It reads every tracked text file (not generated output, vendored code or test fixtures), prints `file:line:column: rule: text` for each problem and exits 1 when it found any. Shared CI runs it for repositories that set `style: true`.

Every source file starts with a two-line copyright and license header in its own comment syntax:

```go
// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only
```

The license comes from the nearest `LICENSE` file, so Tuner's `checks/` (MIT inside an AGPL repository) and the MIT Soundcheck and Echo get theirs. `add-headers.py` adds missing headers and leaves files that have one alone; `--check` only lists the files without one and exits 1:

```sh
python3 ../.github/scripts/add-headers.py --check .
python3 ../.github/scripts/add-headers.py .
```

## Commit messages

Commits and pull request titles follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). The `commit-msg` hook checks each commit locally. CI runs the same script on every commit in a pull request and on the pull request title.

```text
type(scope): subject in lowercase, no period, 72 characters max

Optional body explaining why, after a blank line.
```

### The rules

| Rule | Detail |
|---|---|
| Format | `type(scope): subject`, or `type: subject`. Exactly one space after the colon. The subject can't start with a space. |
| Type | One of `feat` `fix` `perf` `refactor` `docs` `test` `build` `ci` `chore` `style` `revert` |
| Scope | Optional. Lowercase letters and digits, plus `.` `_` `/` `-` after the first character, e.g. `engine`, `checks`, `cli`, `kit/update` |
| Breaking change | `!` right before the colon: `feat(cli)!: rename --list to --targets`. Explain it in a `BREAKING CHANGE:` footer. |
| Subject case | Starts in lowercase. A capital followed by a lowercase letter (`Add …`) fails; acronyms like `SARIF …` are fine. |
| Length | The whole first line is at most 72 characters. |
| Ending | No period at the end of the first line. |
| Body | Optional. If there is one, line 2 must be blank. |

Messages that start with `Merge `, `Revert "`, `fixup! `, `squash! ` or `amend! ` are let through unchanged.

### Which type

| Type | For |
|---|---|
| `feat` | a new feature or check |
| `fix` | a bug fix or false positive |
| `perf` | faster, or fewer requests |
| `refactor` | code change with no behavior change |
| `docs` | documentation only |
| `test` | tests or fixtures only |
| `build` | dependencies, Go version, Makefile |
| `ci` | GitHub Actions |
| `chore` | anything else |
| `style` | formatting only |
| `revert` | reverting an earlier commit |

`feat`, `fix` and `perf` commits get their own sections in the release notes. Unscoped `docs:`, `test:`, `ci:`, `chore:` and `style:` commits are left out of them.

```text
feat(checks): add Laravel Telescope exposure check
fix(engine): retry on 503 with Retry-After
docs: explain baselines in the CI guide
```

When the hook rejects a message, it prints the reason and a reminder of the format:

```text
✖ commit message: start the subject in lowercase

    feat(checks): Add Laravel Telescope check
```

## Pull requests

The **pull request title** follows the same rules as a commit message. So does **every commit** in the pull request: CI's commitlint job checks both, so whichever merge method a repository uses, only well-formed messages reach `master`. Write the title as the one line you'd want in the history.

The pull request template asks for:

- **What and why**: the change and the reason for it.
- **How it was tested**, plus three checkboxes:
  - [ ] `make check` passes (lint + tests)
  - [ ] New or changed checks have positive and negative fixtures
  - [ ] No exploit payloads; secrets stay masked in all output

CI must pass before merging: the [jobs above](#ci), including the commit message check.

## Adding a check

A Tuner check is a YAML file in `tuner/checks/<category>/` plus a `_test.yaml` fixture with at least one positive and one negative case. It takes no Go code. [Writing Checks](Tuner-Writing-Checks) has the format, the matchers and how to test locally.

The ground rules for every check:

- **Detect, never exploit.** Plain GET requests for public URLs, no payloads.
- **Never print a secret unmasked.** Use `extract.secrets`; evidence is redacted automatically.
- **Every check needs positive and negative fixtures.** Good negatives cover the ways it could misfire: the file blocked, a soft-404 catch-all, a similar-looking page.

Checks in `checks/` are MIT licensed so anyone can reuse them; the rest of Tuner is AGPL-3.0. Once a check lands on `master` it reaches users through the next [check pack](Tuner-Check-Packs), without waiting for a release.

## Soundcheck and the testbed

Soundcheck's modules are Go code in `checks/<module>/`, each implementing the hooks in `check/check.go` (see [Architecture](Architecture#inside-soundcheck)). A change to what Soundcheck reports needs a test with a page that shows the problem and one that doesn't, the same rule as Tuner's positive and negative fixtures; most module tests serve small HTML pages from `httptest` and drive a real browser.

End to end, Soundcheck is tested against the **testbed**: a WordPress site and a plain-PHP site in Docker with 89 scenarios, each with a good and a broken state; s49 to s89 test EAA mode (`make -C testbed bench-eaa`). It's also the [benchmark](Soundcheck-Benchmark).

```sh
make testbed                       # WordPress on 127.0.0.1:18800, plain PHP on :18810
testbed/toggle.sh s05 broken && testbed/toggle.sh status
make build && go run -tags testbed ./testbed/bench -only s05   # snapshot good, break, compare, judge
go test -tags testbed ./testbed/bench/   # the testbed's own end-to-end tests
make bench                         # every scenario; writes docs/benchmark.md
make testbed-down                  # throw everything away
```

To add a scenario, add its page (`wp/seed.sh`, or a file in `php/`), implement the broken state behind `testbed_broken('sNN')` next to the code it breaks, describe it in `testbed/scenarios.yaml` with the change Soundcheck should report and run it with `-only`. The [testbed README](https://github.com/RhythmShahriar/soundcheck/blob/master/testbed/README.md) has the details. Keep scenarios to one problem each. Break things the way a real update does.

Like Tuner's checks, Soundcheck must never exploit anything, must mask secrets and must stay safe on real sites: new clicks and submits respect the destructive blocklist and the production guard. Soundcheck is MIT licensed.

## Echo and its connectors

Echo's own code is Go; its connectors are PHP (WordPress, Laravel) and TypeScript (Node). The same rule as Tuner's fixtures applies everywhere: a change to what Echo recognizes or reports needs a test that shows it and one that shows it doesn't misfire.

- **A new service** is a recognizer in `recognize/`: how to match the request, what to record (recipients, content, event) and what to answer when sealed, shaped like the provider's real response. Add fixtures from the provider's documented requests to the file's `_test.go`, including a request that must **not** match. Payment recognizers must refuse live mode.
- **A new check** goes in `check/`, with a message that triggers it and one that doesn't. Titles must be stable across runs (no IDs or times), since `compare` keys on them.
- **The WordPress connector** is one file, `connectors/wordpress/echo.php`, for PHP 7.4 and WordPress 5.7 or later. It's tested against a real WordPress in Docker (`go test -tags docker ./connectors/wordpress`). Keep it to WordPress core APIs. Keep the production guard and fail-closed behavior intact.
- **The Laravel and Node connectors** follow the [capture protocol](Echo-API#the-capture-protocol); change `wire/` only in a backward-compatible way, since connectors in the wild speak it.

Echo must never deliver anything while sealed, must mask secrets and must keep replay's safety rules: tests should prove a sealed path doesn't reach the network, using `127.0.0.1` fixtures only. Echo and its connectors are MIT licensed.

End to end, Echo is tested against its **testbed**: WordPress, Laravel, Next.js and plain PHP in Docker, behind a network with no way out, with 20 scenarios, each with a good and a broken state. It's also the [benchmark](Echo-Benchmark).

```sh
make testbed                                          # the sites on 127.0.0.1:18850-18855
testbed/toggle.sh s05 broken && testbed/toggle.sh status
make build && go run -tags testbed ./testbed/bench -only s05   # snapshot good, break, compare, judge
go test -tags testbed -v ./testbed/bench/             # the harness's own tests
make bench                                            # every scenario; writes docs/benchmark.md
make testbed-down                                     # throw everything away
```

To add a scenario, implement its broken state behind a `testbed/state/<id>` check next to the code it breaks, add the action that shows it to `testbed/.echo/config.yaml` if it needs a new one, describe it in `testbed/scenarios.yaml` with the change Echo should report and run it with `-only`. The [testbed README](https://github.com/RhythmShahriar/echo/blob/master/testbed/README.md) has the details.

## Code of conduct

All Rhythm projects follow the [Contributor Covenant, version 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be kind, be constructive, assume good intent. Read the [Code of Conduct](https://github.com/RhythmShahriar/.github/blob/master/CODE_OF_CONDUCT.md) for how to report unacceptable behavior.

Security issues don't go in pull requests or issues; see the [Security Policy](Security-Policy).

---
<sub>← [Security policy](Security-Policy) · [Home](Home) · [Release process](Release-Process) →</sub>
