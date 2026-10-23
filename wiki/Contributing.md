Contributions are welcome, and new Tuner checks are the most welcome of all. This page covers setting up the code, the commit message rules the hooks enforce, how pull requests are reviewed and merged, and where to start with a check.

For big changes, open an issue first so the approach can be agreed before you write the code. Keep each pull request focused on one thing.

## Repositories

| Repository | What's in it |
|---|---|
| [`RhythmShahriar/rhythm`](https://github.com/RhythmShahriar/rhythm) | The `rhythm` CLI, the install scripts, and `kit/`, the shared base every tool imports |
| [`RhythmShahriar/tuner`](https://github.com/RhythmShahriar/tuner) | Tuner (`rhythm-tuner`) and its checks in `checks/` |
| [`RhythmShahriar/.github`](https://github.com/RhythmShahriar/.github) | Shared community files and the reusable CI and release workflows |

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

### Working on rhythm, or on the kit and a tool together

Clone the repositories side by side and link them with a Go workspace, so a tool builds against your local copy of `kit/`:

```sh
mkdir rhythmsh && cd rhythmsh
git clone https://github.com/RhythmShahriar/rhythm
git clone https://github.com/RhythmShahriar/tuner
go work init ./rhythm ./tuner
```

```text
rhythmsh/
├── go.work      # use ( ./rhythm ./tuner )
├── rhythm/
└── tuner/
```

Keep `go.work` outside the repositories; it's local to your machine and isn't committed. Build both from the workspace root:

```sh
go build -o bin/rhythm ./rhythm/cmd/rhythm
go build -o bin/rhythm-tuner ./tuner/cmd/rhythm-tuner
```

Then run `make hooks` and `make check` in each repository you change.

### Make targets

| Target | Does |
|---|---|
| `make hooks` | Points git at `.githooks/`, enabling the commit message check |
| `make check` | `lint` + `test`, what you should run before every push |
| `make test` | `go test -race ./...` |
| `make lint` | `golangci-lint` v2.15.0 |
| `make fmt` | `gofmt -w .` |
| `make build` | `bin/rhythm` or `bin/rhythm-tuner` |
| `make pack` | Tuner only: builds the check pack into `dist/checks-pack`, byte for byte as CI publishes it |

CI runs the same checks on every pull request, plus `gofmt`, `go vet`, a build, `shellcheck` on scripts and hooks, and `goreleaser check`.

## Commit messages

Commits and pull request titles follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). The `commit-msg` hook checks each commit locally, and CI runs the same script on every commit in a pull request and on the pull request title.

```text
type(scope): subject in lowercase, no period, 72 characters max

Optional body explaining why, after a blank line.
```

### The rules

| Rule | Detail |
|---|---|
| Format | `type(scope): subject`, or `type: subject`. Exactly one space after the colon, and the subject can't start with a space. |
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

The **pull request title** follows the same rules as a commit message, and so does **every commit** in the pull request: CI's commitlint job checks both, so whichever merge method a repository uses, only well-formed messages reach `master`. Write the title as the one line you'd want in the history.

The pull request template asks for:

- **What and why**: the change and the reason for it.
- **How it was tested**, and three checkboxes:
  - [ ] `make check` passes (lint + tests)
  - [ ] New or changed checks have positive and negative fixtures
  - [ ] No exploit payloads; secrets stay masked in all output

CI must pass before merging: the Go checks and the commit message check.

## Adding a check

A Tuner check is a YAML file in `tuner/checks/<category>/` plus a `_test.yaml` fixture with at least one positive and one negative case. It takes no Go code. [Writing Checks](Tuner-Writing-Checks) has the format, the matchers and how to test locally.

The ground rules for every check:

- **Detect, never exploit.** Plain GET requests for public URLs, no payloads.
- **Never print a secret unmasked.** Use `extract.secrets`; evidence is redacted automatically.
- **Every check needs positive and negative fixtures**, and good negatives cover the ways it could misfire: the file blocked, a soft-404 catch-all, a similar-looking page.

Checks in `checks/` are MIT licensed so anyone can reuse them; the rest of Tuner is AGPL-3.0. Once a check lands on `master` it reaches users through the next [check pack](Tuner-Check-Packs), without waiting for a release.

## Code of conduct

All Rhythm projects follow the [Contributor Covenant, version 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be kind, be constructive, assume good intent. Read the [Code of Conduct](https://github.com/RhythmShahriar/.github/blob/master/CODE_OF_CONDUCT.md) for how to report unacceptable behavior.

Security issues don't go in pull requests or issues; see the [Security Policy](Security-Policy).

---
<sub>← [Security policy](Security-Policy) · [Home](Home) · [Release process](Release-Process) →</sub>
