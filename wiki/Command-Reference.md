Every `rhythm` command and flag, the exit codes, the JSON documents, the environment variables and the files rhythm keeps. The tools' own commands are on their pages: [Tuner](Tuner-CLI-Reference), [Soundcheck](Soundcheck-CLI-Reference) and [Echo](Echo-CLI-Reference).

The samples on this page come from a build of rhythm's `master` (soon v0.7.0) with Tuner 0.3.2, Soundcheck 1.1.0 and Echo 1.0.0 installed in `/home/me/.local/bin`. A build from source calls itself `dev`, so its own line says `development build` where a release says its version.

## Usage

```console
$ rhythm --help
♪ rhythm  dev

Usage:
  rhythm <tool> [args]
  rhythm <command> [args]

Tools:
  tuner (t)        WordPress & PHP security scanner
  soundcheck (sc)  did my change break anything? whole-site before/after checks
  echo (e)         is my site still saying what it should? every email, SMS and webhook

Commands:
  rhythm bundle --locked -o <file> [--platform os/arch,...]   pack what rhythm.lock pins into one file for offline installs
  rhythm completion bash|zsh|fish|powershell                  print a shell completion script
  rhythm config [update-prompt [ask|notify|off]]              show or change rhythm's settings
  rhythm doctor [--offline] [--json]                          check the install: PATH, versions, data, Chrome and the release API
  rhythm ext search|install|list|remove ...                   find, install and remove verified third-party extensions
  rhythm help [tool|command]                                  show help for rhythm, a tool or a command
  rhythm install <tool>[@version]... [flags]                  download tools
  rhythm list [--offline] [--json]                            show the installed tools, their versions and paths
  rhythm lock [--no-data]                                     pin the installed versions in rhythm.lock
  rhythm uninstall <tool>... [--purge] [--all]                remove tools (with --purge, their data too)
  rhythm update [tool...] [flags]                             update rhythm, installed tools and their data
  rhythm verify [tool...] [--json]                            check installed binaries against their signed releases
  rhythm version                                              print rhythm's version

Run 'rhythm help <tool>' for a tool's own commands and flags.

Exit codes:
  0     success
  1     findings (any other exit code from a tool also passes through)
  2     usage error or unknown tool
  3     tool not installed or can't start
  10    rhythm update --check found an update
  130   interrupted
```

| Command | What it does | More |
|---|---|---|
| `rhythm <tool> [args]` | run a tool | [Running a tool](#running-a-tool) |
| `rhythm help [tool\|command]` | help for rhythm, a tool or a command | [help](#help-and-version) |
| `rhythm install` | download tools, latest or pinned | [install](#install) |
| `rhythm update` | update rhythm, the tools and their data | [update](#update), [Updating](Updating) |
| `rhythm list` | what is installed, its version and where | [list](#list) |
| `rhythm verify` | check installed binaries against their signed releases | [verify](#verify), [Verifying releases](Verifying-Releases) |
| `rhythm uninstall` | remove tools, their data and rhythm itself | [uninstall](#uninstall), [Uninstall](Uninstall) |
| `rhythm doctor` | check the install | [doctor](#doctor) |
| `rhythm lock` | write `rhythm.lock` | [Pinning and locking](Pinning-And-Locking) |
| `rhythm bundle` | pack locked releases for a network without GitHub | [Offline installs](Offline-Installs) |
| `rhythm ext` | find and install third-party extensions | [Extensions](Extensions) |
| `rhythm completion` | print a shell completion script | [completion](#completion) |
| `rhythm config` | show or change settings, such as the update prompt | [config](#config) |
| `rhythm version` | print rhythm's version (also `-v`, `-V`, `--version`) | [version](#help-and-version) |

Flags go before or after names (`rhythm update tuner --no-data`). Everything after `--` is passed on as is. Names can be separated by spaces or commas (`rhythm install tuner,echo soundcheck`), in any case, as a tool name (`tuner`), its short name (`t`) or its binary (`rhythm-tuner`).

## Running a tool

`rhythm <tool> [args]` runs `rhythm-<tool>` from your `PATH` with the rest of the arguments. Short names work: `t` is `tuner`, `sc` is `soundcheck` and `e` is `echo`. Any other `rhythm-<name>` program on your `PATH` runs as `rhythm <name>` too. `rhythm help` lists it under **Other tools**. With a small `rhythm-zap` script on the `PATH` that prints what it was given:

```console
$ rhythm zap hello
zap ran as: rhythm zap, args: hello
$ rhythm --help
...
Tools:
  tuner (t)        WordPress & PHP security scanner
  soundcheck (sc)  did my change break anything? whole-site before/after checks
  echo (e)         is my site still saying what it should? every email, SMS and webhook

Other tools:
  zap
...
```

Only absolute folders on `PATH` are searched and only executable files count. The tool gets `RHYTHM_INVOKED_AS` set to what you typed (`rhythm zap` above, `rhythm sc` for `rhythm sc compare`), so its help and errors can show the command you ran.

When the name doesn't lead to a program, rhythm says why:

| You ran | rhythm says | Exit |
|---|---|---|
| `rhythm tunr` | `rhythm: unknown tool "tunr"`<br>`  did you mean: rhythm tuner`<br>`  run 'rhythm help' to see the tools` | 2 |
| `rhythm nope` | `rhythm: unknown tool "nope"`<br>`  run 'rhythm help' to see the tools` | 2 |
| `rhythm /../evil` | `rhythm: invalid tool name "/../evil"` (names are lowercase letters, digits and `-`) | 2 |
| `rhythm tuner`, not installed | `rhythm: tuner isn't installed`<br>`  install it with: rhythm install tuner` | 3 |
| a tool that can't start | `rhythm: can't start tuner: <error>`<br>`  reinstall it with: rhythm install tuner` | 3 |

The did-you-mean line suggests a tool, short name, command or other tool within two edits of what you typed. A one-letter name is never guessed from.

### Exit codes and signals

The tool's exit code becomes rhythm's, so `rhythm tuner scan … --fail-on high` behaves in CI exactly like `rhythm-tuner scan … --fail-on high`. A tool that exits 42 makes `rhythm` exit 42.

On Linux, macOS and the BSDs rhythm replaces itself with the tool, which keeps rhythm's process ID. Ctrl-C, `docker stop`, systemd and a cancelled CI job reach the tool directly, the prompt comes back only once the tool has cleaned up and nothing is left running. A tool ended by a signal looks to your shell like any process ended by it: 130 for Ctrl-C, 143 for `SIGTERM`. On Windows rhythm runs the tool as a child and waits for it; Ctrl-C and Ctrl-Break give 130.

## help and version

| Command | Prints |
|---|---|
| `rhythm`, `rhythm help`, `rhythm --help` | the usage above |
| `rhythm help <tool>` (or a short name: `rhythm help t`) | the tool's own `--help` |
| `rhythm help <command>` | the command's help, the same as `rhythm <command> --help` |
| `rhythm version`, `rhythm --version`, `-v`, `-V` | `rhythm <version>`, such as `rhythm 0.7.0` |

`rhythm help a b` exits 2 with `usage: rhythm help [tool|command]` and `rhythm version extra` with `usage: rhythm version`. Every command's `--help` goes to stdout and exits 0.

## install

```text
Usage: rhythm install <tool>[@version]... [flags]
```

Downloads the latest release of each tool and installs it next to rhythm. Every name is checked before the first download; a tool that fails doesn't stop the others.

| Flag | Effect |
|---|---|
| `tool@version` | `tuner@0.3.1` installs exactly 0.3.1, `tuner@0.3` the newest 0.3.x, `tuner@latest` the latest. See [Pinning and locking](Pinning-And-Locking#pin-a-version). |
| `--defaults` | also install the tools a fresh setup gets (today Echo, Soundcheck and Tuner) |
| `--dir dir` | install into `dir`, created if missing (default: the folder rhythm is in) |
| `--dry-run` | check the names and print the tools one per line, without a network request |
| `--force` | reinstall a tool that is already on the release asked for |
| `--quiet` | print only the Installed lines and errors |
| `--locked` | install the versions in `rhythm.lock`; see [Pinning and locking](Pinning-And-Locking#install-what-the-lock-pins) |
| `--min-age age` | skip releases younger than `age`, such as `3d` or `1w` (default `$RHYTHM_MIN_RELEASE_AGE`) |
| `--from bundle` | install from an offline bundle instead of GitHub; see [Offline installs](Offline-Installs) |
| `--skip-signature` | install without checking the release signature, with a warning |
| `--cache` | keep verified archives and reuse them (default `$RHYTHM_DOWNLOAD_CACHE`) |

```console
$ rhythm install tuner@0.3 soundcheck
Installed rhythm-tuner 0.3.2 to /home/me/.local/bin/rhythm-tuner
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
Installed rhythm-soundcheck 1.1.0 to /home/me/.local/bin/rhythm-soundcheck
  Signature ok  RhythmShahriar/soundcheck @ refs/tags/v1.1.0 (release-go.yml)
  Checksum ok   sha256:b1528d466f561d5150e52de257d9097ee79a8bf4fa90fd001dc80b0335ee1ed2

$ rhythm install echo tuner
Installed rhythm-echo 1.0.0 to /home/me/.local/bin/rhythm-echo
  Signature ok  RhythmShahriar/echo @ refs/tags/v1.0.0 (release-go.yml)
  Checksum ok   sha256:419130487999167ce27cf0d67c519bd36f35b8b270d307b32a3cee314cc3a5f2
rhythm-tuner 0.3.2 is already installed at /home/me/.local/bin/rhythm-tuner (use --force to reinstall)

$ rhythm install --defaults --dry-run
echo
soundcheck
tuner
```

The two indented lines say whose workflow signed the release and which archive was installed; [Verifying releases](Verifying-Releases) explains them. On a terminal a `Downloading rhythm-tuner 0.3.2  12.4/27.1 MB` line shows the progress and is erased when the download ends.

Messages you may see, all before anything is downloaded:

| Message | Exit |
|---|---|
| `rhythm: unknown tool "sondcheck"`<br>`  did you mean: rhythm install soundcheck`<br>`rhythm: unknown tool "nope"`<br>`  known: echo, soundcheck, tuner` | 2 |
| `rhythm: rhythm installs tools, not itself; update it with: rhythm update --self-only` | 2 |
| `usage: rhythm install <tool>[@version]... [flags]; name a tool, or use --defaults` | 2 |
| `rhythm: tuner: invalid version "0.3.x"; use latest, an exact version such as 0.3.1, or 0.3 for the newest 0.3.x` | 2 |
| `rhythm: tuner is named twice with different versions: tuner@0.3 and tuner@0.4` | 2 |
| `rhythm: <dir> is a file, not a folder; pick a folder with --dir` | 1 |
| `rhythm: can't write to <dir>; rerun with sudo, or install to a folder you own with --dir` | 1 |
| `rhythm: rhythm is managed by Homebrew, so tools don't go in its folder`<br>`  pick a folder on your PATH: rhythm install tuner --dir <dir>` | 2 |

`rhythm install none` prints `Nothing to install.` and exits 0. When more than one tool was asked for and one failed, the run ends with a line such as `Installed 1, already installed 1, failed 1: rhythm-echo` and exits 1. After the install rhythm checks your `PATH` once: `Note: <dir> isn't on your PATH.` or `Note: <path> comes first on your PATH; remove it to use this install.`

A tool binary that a package manager installed is left to it: `rhythm-tuner is managed by Homebrew; update it with: brew upgrade rhythm-tuner`. Homebrew, Nix, mise, asdf, Scoop and WinGet are recognized, as is a `shims` folder or a link to a program of another name (`rhythm-tuner is managed by busybox; update it with: the program that installed <path>`).

## update

```text
Usage: rhythm update [tool...] [flags]
```

Updates rhythm and every installed tool, then refreshes each tool's data. Name tools, or `rhythm`, to update only those. [Updating](Updating) walks through it with samples.

| Flag | Effect |
|---|---|
| `--check` | only report; exit 10 when an update is available |
| `--json` | with `--check`, print the report as [`rhythm-update/v1`](#json-output) |
| `--self-only` | update only rhythm |
| `--no-data` | skip the tools' data refresh |
| `--quiet` | print only updates, problems and the tools' own output |
| `--min-age age` | skip releases younger than `age` (default `$RHYTHM_MIN_RELEASE_AGE`) |
| `--to version` | install that version of the one tool named, older ones too |
| `--rollback` | put the named tools back to the binary their last update replaced |
| `--lock` | update `rhythm.lock` instead of the binaries; see [Pinning and locking](Pinning-And-Locking#move-the-lock-forward) |
| `--skip-signature` | update without checking release signatures, with a warning |
| `--cache` | keep verified archives and reuse them (default `$RHYTHM_DOWNLOAD_CACHE`) |

Usage errors exit 2 before any request:

| Message |
|---|
| `rhythm: unknown tool "tunr"`<br>`  did you mean: rhythm update tuner`<br>`  known: rhythm, echo, soundcheck, tuner` |
| `rhythm: --self-only updates rhythm alone; drop the tool names or the flag` |
| `rhythm: --json works with --check only` |
| `rhythm: --json doesn't work with --lock; the lock is JSON already` |
| `rhythm: --min-age: invalid age "3"; use a duration such as 3d, 1w or 36h` |
| `rhythm: RHYTHM_MIN_RELEASE_AGE: invalid age "-1d"; use a duration such as 3d, 1w or 36h` |
| `rhythm: --rollback needs a tool name, such as: rhythm update --rollback tuner` |
| `rhythm: --rollback and --to don't mix; pick one` |
| `rhythm: --rollback works alone; drop --check and --lock` |
| `rhythm: --to needs exactly one tool name, such as: rhythm update tuner --to 0.3.1` |
| `rhythm: --to works alone; drop --check and --lock` |
| `rhythm: --to: invalid version "0.3.x"; use latest, an exact version such as 0.3.1, or 0.3 for the newest 0.3.x` |

## list

```console
$ rhythm list
TOOL        VERSION  LATEST  SIGNED  PATH
rhythm      0.7.1    0.7.1   yes     /home/me/.local/bin/rhythm
echo        1.0.0    1.0.0   yes     /home/me/.local/bin/rhythm-echo
soundcheck  1.1.0    1.1.0   -       /home/me/.local/bin/rhythm-soundcheck
tuner       0.3.2    0.3.2   yes     /home/me/.local/bin/rhythm-tuner
zap (3rd)   1.2.0    -       -       /home/me/.local/bin/rhythm-zap

SIGNED - means rhythm hasn't checked that binary yet; rhythm verify checks it.
```

One row for rhythm, one for each installed tool and one for every other `rhythm-*` program on your `PATH`, marked `(3rd)`. When rhythm, a tool or an extension shows `-` under SIGNED, the table ends with the line above; a third-party program's `-` doesn't add it.

| Column | Shows |
|---|---|
| VERSION | what the binary reports; `dev` for a development build, `unknown` when it doesn't say |
| LATEST | the latest release; `-` for a third-party program, `?` when the lookup failed (the reason follows on stderr) |
| SIGNED | `yes` for a binary that passed a signature check (at install, update or `rhythm verify`) and hasn't changed since; `no` for one whose bytes changed since; `-` for one rhythm hasn't checked yet (installed by a rhythm older than 0.7.0 or with `--skip-signature`; `rhythm verify` checks it) and for a third-party program. Extensions installed with `rhythm ext install` show `yes`, `no` or `-` too. No network is needed: rhythm compares the file's sha256 with what it recorded in `verified.json`. See [The SIGNED column](Verifying-Releases#the-signed-column). |
| PATH | where it runs from, with the package manager after it when one installed it: `/opt/homebrew/bin/rhythm-tuner (Homebrew)` |

`--offline` (or `RHYTHM_OFFLINE`) leaves out the LATEST column and makes no request. `--json` prints [`rhythm-list/v1`](#json-output). `rhythm list tuner` exits 2 with `rhythm: list takes no names (got "tuner")`. A failed lookup doesn't change the exit code: the list was shown, so it exits 0. A config folder rhythm can't read, such as a relative `RHYTHM_CONFIG_DIR`, exits 1 before anything is listed, as `rhythm ext list` and `rhythm update` do: `rhythm: RHYTHM_CONFIG_DIR must be an absolute path, got "rel"`.

## verify

```console
$ rhythm verify tuner
rhythm-tuner 0.3.2 (/home/me/.local/bin/rhythm-tuner)
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
  Binary ok     same bytes as in rhythm-tuner_linux_amd64.tar.gz
```

For rhythm and each installed tool and extension (or the ones named) it fetches the installed version's `checksums.txt` and signature, checks the signature, checks the archive against `checksums.txt` and compares the binary in the archive with the installed file, byte for byte. The archive comes from the download cache when it's there. It exits 0 when every binary checked out and 1 when any didn't or couldn't be checked. [Verifying releases](Verifying-Releases#rhythm-verify) lists every failure message. `--json` prints [`rhythm-verify/v1`](#json-output).

## uninstall

```console
$ rhythm uninstall tuner --purge
Removed /home/me/.local/bin/rhythm-tuner
Removed /home/me/.local/bin/rhythm-tuner.prev
Removed /home/me/.local/bin/rhythm-tuner.prev.json
Removed /home/me/.cache/rhythm/tuner
$ rhythm uninstall tuner
rhythm-tuner       isn't installed
```

Removes the copy of each tool that runs (the first on `PATH`), the `.prev` and `.old` copies updates leave next to it and, with `--purge`, the tool's cache and update-check state. `--all` removes every installed tool. rhythm itself goes only with `rhythm uninstall --all rhythm`. [Uninstall](Uninstall) has the rest, including tools a package manager installed.

## doctor

```console
$ rhythm doctor
ok    rhythm runs from /home/me/.local/bin/rhythm
ok    rhythm-echo runs from /home/me/.local/bin/rhythm-echo
ok    rhythm-soundcheck runs from /home/me/.local/bin/rhythm-soundcheck
ok    rhythm-tuner runs from /home/me/.local/bin/rhythm-tuner
ok    /home/me/.local/bin is writable
warn  rhythm is a development build (dev); rhythm update leaves it alone
ok    no update checks saved yet
ok    Soundcheck can use /usr/bin/google-chrome
ok    no GitHub token set; GitHub allows 60 release lookups an hour without one
ok    api.github.com answers; 3 of 60 requests left this hour
0 failures, 1 warning.
```

One line per check, each `ok`, `warn` or `fail`. It exits 1 when any check failed; warnings don't fail it.

| Check | What it looks at |
|---|---|
| `path.<binary>` | which copy of rhythm and each tool runs first and the copies it hides: `rhythm-tuner: <first> comes first on your PATH and hides <others>; remove the copy you don't use` |
| `install-dir` | whether rhythm's folder is writable, or which package manager owns it |
| `version` | development builds and tools whose version can't be read, which `rhythm update` leaves alone |
| `update-check` | the saved once-a-day release checks: unreadable, dated in the future, left from a tool that's gone, or naming a newer release |
| `data.<binary>` | the data of tools built on the shared kit, refreshed and dated (packs older than 14 days warn). Today's Tuner, Soundcheck and Echo get no data line yet. |
| `chrome` | the Chrome or Chromium Soundcheck would use, when Soundcheck is installed |
| `token` | the token rhythm would send (never its value) and the release API in use |
| `api` | whether the release API answers and how many requests are left (GitHub doesn't count this one) |

The token line names the token really sent: `token in use: GITHUB_TOKEN (value not shown)`, `token in use: RHYTHM_GITHUB_TOKEN (value not shown)`, or for a mirror with only `GITHUB_TOKEN` set:

```text
ok    no token for the mirror; GITHUB_TOKEN goes to GitHub only (set RHYTHM_GITHUB_TOKEN for a mirror); release API https://mirror.example (from RHYTHM_GITHUB_API)
```

A `RHYTHM_GITHUB_API` that breaks the [https rule](#release-api-and-tokens) fails the `api` check before anything is sent:

```text
fail  RHYTHM_GITHUB_API must be https (plain http only for 127.0.0.1, ::1 or localhost); got http://mirror.example
```

`--offline` (or `RHYTHM_OFFLINE`) skips the data refresh and the `api` check. `--json` prints [`rhythm-doctor/v1`](#json-output).

## config

```text
Usage: rhythm config [update-prompt [ask|notify|off]]
```

Shows rhythm's settings, one of them, or changes one. They are kept in `settings.json` in rhythm's config folder and apply to rhythm and every tool. Today there is one:

| Setting | Values | Effect |
|---|---|---|
| `update-prompt` | `ask` (default), `notify`, `off` | at the end of a run with a newer release out: ask "Update now? [y/N]", print the notice, or say nothing. See [The update prompt](Updating#the-update-prompt). |

```console
$ rhythm config
update-prompt  ask  (default)
$ rhythm config update-prompt off
update-prompt set to off
$ rhythm config update-prompt
off
```

`RHYTHM_UPDATE_PROMPT` overrides the saved value: `rhythm config update-prompt` then says `ask  (from RHYTHM_UPDATE_PROMPT)` and saving a value adds `Note: RHYTHM_UPDATE_PROMPT=ask is set and wins over this setting.` on stderr.

| Message | Exit |
|---|---|
| `rhythm: "loud" isn't an update-prompt mode; use ask, notify or off` | 2 |
| `rhythm: unknown setting "update-promt"`<br>`  did you mean: rhythm config update-prompt`<br>`  known: update-prompt` | 2 |
| `usage: rhythm config [update-prompt [ask\|notify\|off]]` | 2 |
| `rhythm: RHYTHM_UPDATE_PROMPT: "sometimes" isn't an update-prompt mode; use ask, notify or off` | 1 |
| `rhythm: reading <config>/settings.json: …` (a file that isn't a JSON object) | 1 |

### The update prompt after rhythm's commands

After its own commands rhythm may ask about its own release, on a terminal only and never in CI: `A new release of rhythm is out: 0.8.1 (you have 0.8.0). Update now? [y/N] `. `y` runs `rhythm update --self-only`. It doesn't ask after `update`, `uninstall`, `version`, `completion` or `config`, with `--json`, `--quiet` or `--offline`, or when stdout is piped. The full rules are on [Updating](Updating#the-update-prompt).

## completion

```sh
eval "$(rhythm completion bash)"                                   # in ~/.bashrc
rhythm completion zsh > "${fpath[1]}/_rhythm"                      # zsh
rhythm completion fish > ~/.config/fish/completions/rhythm.fish    # fish
```

```powershell
rhythm completion powershell | Out-String | Invoke-Expression      # in $PROFILE
```

Tab completes rhythm's commands, the tools, their short names, other `rhythm-*` programs, every command's flags (`rhythm update --<Tab>` offers `--cache --check --help --json --lock --min-age --no-data --quiet --rollback --self-only --skip-signature --to`), tool names after `install`, `update`, `uninstall`, `verify` and `help`, extension names after `update` and `verify` and the four `ext` subcommands. After a flag that takes a value, such as `--dir`, the shell completes file names.

The script holds no names: each Tab runs `rhythm __complete`, so tools you install later complete without a new script. For `rhythm <tool> …` rhythm asks the tool itself. It does so only for a registry tool or an installed extension built on the shared kit (`kit/cli`); other programs are never run by completion. Today's Tuner, Soundcheck and Echo complete their own words once they move to that kit.

`rhythm completion zhs` exits 2 with `rhythm: unknown shell "zhs"; want bash, zsh, fish or powershell` and `  did you mean: rhythm completion zsh`.

> [!NOTE]
> If you saved a bash script from an older rhythm in a file, generate it again: older scripts passed candidates through `compgen -W`, which expanded them a second time.

## JSON output

Every JSON document has a `schema` field naming its version, is indented two spaces and goes to stdout only; the human lines go to stderr. The exit codes are the same as without `--json`.

| Command | Schema | Fields |
|---|---|---|
| `rhythm update --check --json` | `rhythm-update/v1` | `targets[]`: `name` (binary), `current`, `latest`, `path`, `outdated`, `signed`, `managed_by`, `error`, `skipped_newer` (`null`, or `version` and `published` of the newest release `--min-age` passed over) |
| `rhythm list --json` | `rhythm-list/v1` | `tools[]`: `name`, `binary`, `version`, `latest`, `signed`, `path`, `third_party`, `managed_by`, `error` |
| `rhythm doctor --json` | `rhythm-doctor/v1` | `checks[]`: `id`, `status` (`ok`, `warn`, `fail`), `message` |
| `rhythm verify --json` | `rhythm-verify/v1` | `targets[]`: `name`, `version`, `path`, `repo`, `signer`, `source_ref`, `archive`, `archive_sha256`, `binary_sha256`, `cached`, `ok`, `error` |
| `rhythm ext search --json` | `rhythm-ext-search/v1` | `results[]`: `full_name`, `name`, `description`, `stars`, `url`, `taken` |
| `rhythm ext list --json` | `rhythm-ext-list/v1` | `extensions[]`: `name`, `repo`, `signer`, `version`, `path`, `signed` |

```console
$ rhythm update --check --json tuner
rhythm-tuner       0.3.1 → 0.3.2 available
{
  "schema": "rhythm-update/v1",
  "targets": [
    {
      "name": "rhythm-tuner",
      "current": "0.3.1",
      "latest": "0.3.2",
      "path": "/home/me/.local/bin/rhythm-tuner",
      "outdated": true,
      "signed": "yes",
      "managed_by": "",
      "error": "",
      "skipped_newer": null
    }
  ]
}
```

`signed` is `yes`, `no` or `""` (not checked yet, or third-party); the text table's `-` is `""` here and the hint line is left out. `current` and `version` are `""` when unknown; `latest` is `""` when it wasn't looked up. `rhythm.lock` (`rhythm-lock/v1`) and bundle manifests (`rhythm-bundle/v1`) are described in [Pinning and locking](Pinning-And-Locking#the-lock-file) and [Offline installs](Offline-Installs#whats-in-a-bundle).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | the tool's findings, passed through; for rhythm's own commands, something failed |
| 2 | usage error or unknown tool |
| 3 | the tool isn't installed or can't start |
| 10 | `rhythm update --check` found an update (or `update --lock --check` found the lock behind) |
| 130 | interrupted (Ctrl-C); a tool stopped with `SIGTERM` gives 143 |

Any other exit code from a tool passes through unchanged.

## Environment

| Variable | Effect |
|---|---|
| `RHYTHM_GITHUB_API` | a GitHub API mirror or GitHub Enterprise (default `https://api.github.com`); see [Release API and tokens](#release-api-and-tokens) |
| `GITHUB_TOKEN` | sent to GitHub only, to lift its limit of 60 release lookups an hour |
| `RHYTHM_GITHUB_TOKEN` | sent to the `RHYTHM_GITHUB_API` host; when set, `GITHUB_TOKEN` isn't sent anywhere |
| `RHYTHM_OFFLINE` | any value but empty or `0`: no network requests. Release checks are skipped; downloads fail with `RHYTHM_OFFLINE is set, so nothing was downloaded for <binaries>; unset it to reach the release API`. `rhythm update` skips the tools' data refresh too. `install --from` still works. |
| `RHYTHM_MIN_RELEASE_AGE` | the default for `--min-age`, such as `3d` |
| `RHYTHM_DOWNLOAD_CACHE` | any value but empty, `0` or `false`: the default for `--cache` |
| `RHYTHM_CONFIG_DIR` | the whole rhythm config folder; must be an absolute path |
| `RHYTHM_CACHE_DIR` | the cache root, as `<dir>/<tool>`; must be an absolute path |
| `RHYTHM_NO_UPDATE_CHECK` | no once-a-day release check, update prompt or notice in rhythm and the tools (also off when `CI` is set) |
| `RHYTHM_UPDATE_PROMPT` | `ask`, `notify` or `off`: overrides [`rhythm config update-prompt`](#config) |
| `RHYTHM_DEBUG` | any value: say `rhythm: using the built-in Sigstore root: <reason>` when the online refresh of the signature root failed |
| `RHYTHM_INVOKED_AS` | set by rhythm for the tool it runs, such as `rhythm sc` |
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | the usual proxy settings |

A relative `RHYTHM_CONFIG_DIR` stops with `RHYTHM_CONFIG_DIR must be an absolute path, got "<value>"` (`RHYTHM_CACHE_DIR` the same). The installers read a few more variables; see [Install](Install#installer-options).

### Release API and tokens

Every command that reaches the release API (`install`, `update`, `list`, `lock`, `verify`, `bundle`, `ext` and `doctor`) follows one rule:

- `RHYTHM_GITHUB_API` must be `https`. Plain `http` is accepted only for `127.0.0.1`, `::1` and `localhost`, for tests. It must be a plain base URL, with no user info, query or fragment.
- `GITHUB_TOKEN` goes only to `api.github.com` and GitHub's download hosts, never to a mirror.
- `RHYTHM_GITHUB_TOKEN` goes to the `RHYTHM_GITHUB_API` host. When it is set, `GITHUB_TOKEN` isn't sent anywhere.
- A redirect to another host or port drops the token. A redirect off `https` isn't followed.

A URL that breaks the rule is refused before anything is sent, per tool:

```text
rhythm-tuner       release API: refusing plain http: http://mirror.example; use https
rhythm-tuner       release API: must be a plain base URL, without user info, query or fragment: https://mirror.example
```

Network problems are said once for all the binaries they hit, then the summary:

```text
can't reach api.github.com for rhythm-echo, rhythm-soundcheck, rhythm-tuner; check network, proxy or RHYTHM_GITHUB_API
Updated 0, up to date 0, failed 3: rhythm-echo, rhythm-soundcheck, rhythm-tuner
```

| Problem | Message |
|---|---|
| GitHub's rate limit | `GitHub rate limit reached for <binaries>; set GITHUB_TOKEN (resets 14:05)` |
| a mirror's rate limit | `rate limit reached at <mirror> for <binaries>; set RHYTHM_GITHUB_TOKEN` |
| a repository the mirror doesn't have | `mirror <mirror> has no release for RhythmShahriar/tuner; check RHYTHM_GITHUB_API` |
| a version the mirror doesn't have | `mirror <mirror> has no release v0.9.9 for RhythmShahriar/tuner; check RHYTHM_GITHUB_API` |
| an answer with another version than asked | `<api> answered RhythmShahriar/tuner v0.2.0 with release "v0.1.0"; refusing it` |

The `; set …` part is left out when a token was sent. A mirror URL is always shown without its user info, query or fragment.

## Files

| Where | What |
|---|---|
| config `extensions.json` | the extensions `rhythm ext install` added, with the workflow that must sign them |
| config `verified.json` | the sha256 of every binary that passed a signature check, for the SIGNED column |
| config `settings.json` | the settings [`rhythm config`](#config) changes |
| config `update-check/` | the once-a-day release checks of rhythm and the tools, with the version you last said no to at the [update prompt](Updating#the-update-prompt) |
| config `seen/` | one-time notices: the tools' responsible-use notes and `New tool available` lines |
| cache `rhythm/downloads/` | verified archives kept by `--cache` |
| cache `rhythm/tuf/` | the Sigstore trusted root, refreshed online at most once a day |
| next to each binary | `<binary>.prev` and `<binary>.prev.json`, the binary its last update replaced (see [Rolling back](Updating#roll-back)) |

The config folder is `~/.config/rhythm` on Linux, `~/Library/Application Support/rhythm` on macOS and `%APPDATA%\rhythm` on Windows. The cache folder is `~/.cache/rhythm`, `~/Library/Caches/rhythm` or `%LOCALAPPDATA%\rhythm`. `RHYTHM_CONFIG_DIR` and `RHYTHM_CACHE_DIR` move them.

---
<sub>← [Verifying releases](Verifying-Releases) · [Home](Home) · [Pinning and locking](Pinning-And-Locking) →</sub>
