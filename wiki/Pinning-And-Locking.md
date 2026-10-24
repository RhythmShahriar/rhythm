How to pin a tool to a version, hold back releases that are too new and give every laptop and pipeline exactly the same versions with `rhythm.lock`. Each tool still releases on its own; pinning only decides which of its releases you get.

| You want | Use |
|---|---|
| one tool at one version, now | `rhythm install tuner@0.3.1` or `rhythm update tuner --to 0.3.1` |
| the newest of a minor line | `rhythm install tuner@0.3` |
| to wait a few days before taking a release | `--min-age 3d` or `RHYTHM_MIN_RELEASE_AGE=3d` |
| the same versions on every machine | `rhythm lock`, commit `rhythm.lock`, then `rhythm install --locked` |
| to move the lock forward on purpose | `rhythm update --lock` |
| a machine without GitHub | a bundle; see [Offline installs](Offline-Installs) |

## Pin a version

`rhythm install` takes `tool@version`:

| Spec | Installs |
|---|---|
| `tuner@0.3.1` (or `v0.3.1`) | exactly 0.3.1; prereleases such as `0.4.0-rc1` too |
| `tuner@0.3` | the newest 0.3.x release, prereleases left out |
| `tuner@0` | the newest 0.x.y release |
| `tuner@latest` | the latest release, the same as `tuner` |

```console
$ rhythm install tuner@0.3 soundcheck
Installed rhythm-tuner 0.3.2 to /home/me/.local/bin/rhythm-tuner
  Signature ok  RhythmShahriar/tuner @ refs/tags/v0.3.2 (release-go.yml)
  Checksum ok   sha256:201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06
Installed rhythm-soundcheck 1.1.0 to /home/me/.local/bin/rhythm-soundcheck
  Signature ok  RhythmShahriar/soundcheck @ refs/tags/v1.1.0 (release-go.yml)
  Checksum ok   sha256:b1528d466f561d5150e52de257d9097ee79a8bf4fa90fd001dc80b0335ee1ed2
```

A partial spec searches the newest 100 releases; an exact version is found at any age. Installing an older version over a newer one is an ordinary install. To change the version of a tool you already have, `rhythm update tuner --to 0.3.1` does the same and says when it goes down (see [Updating](Updating#a-chosen-version)).

The specs are checked before any request:

```text
rhythm: tuner: invalid version "0.3.x"; use latest, an exact version such as 0.3.1, or 0.3 for the newest 0.3.x
rhythm: tuner is named twice with different versions: tuner@0.3 and tuner@0.4
```

A release that doesn't exist reads `RhythmShahriar/tuner has no release v0.9.9`, or on a mirror `mirror <api> has no release v0.9.9 for RhythmShahriar/tuner; check RHYTHM_GITHUB_API`. rhythm also refuses an answer whose tag isn't the one it asked for, so a mirror can't hand back another release: `<api> answered RhythmShahriar/tuner v0.2.0 with release "v0.1.0"; refusing it`.

The installers take the same specs in `RHYTHM_TOOLS`, such as `RHYTHM_TOOLS='tuner@0.3.1,soundcheck'`; see [Install](Install#installer-options).

## Wait before taking a release

A release cooldown passes over releases younger than an age you choose, so a release that is pulled in its first days never reaches you. Set it per command with `--min-age` or for every command with `RHYTHM_MIN_RELEASE_AGE`; the flag wins and `0` turns it off. It applies to `rhythm install`, `rhythm update` (rhythm itself included) and `rhythm update --lock`.

Ages are Go durations plus `d` for days and `w` for weeks, combined or fractional: `3d`, `1w`, `36h`, `1w2d`, `1.5d`, `90m`.

```console
$ rhythm update --check --min-age 3w tuner
rhythm-tuner 0.3.2 is 28 hours old; staying on 0.3.1 (min age 3w)
$ echo $?
0
```

| Command | When the newest release is too young |
|---|---|
| `rhythm update` | `rhythm-tuner 0.3.3 is 1 day old; staying on 0.3.2 (min age 3d)`, or `...; updating to 0.3.2 (min age 3d)` when an older release in between is old enough |
| `rhythm update --check` | `rhythm-tuner 0.3.3 is 1 day old; 0.3.2 is the newest old enough (min age 3d)` |
| `rhythm install` | `rhythm-tuner 0.3.3 is 1 day old; installing 0.3.2 (min age 3d)` |
| `rhythm install`, nothing old enough | `rhythm-tuner       no release of RhythmShahriar/tuner is old enough: the newest, 0.3.3, is 1 day old (min age 2 weeks)` (exit 1) |
| `rhythm update --lock` | `rhythm-tuner 0.4.0 is 1 day old; keeping 0.3.1 (min age 3d)` or `...; locking 0.3.2 (min age 3d)` |

A version named exactly (`tuner@0.3.3`, `--to 0.3.3`) ignores the cooldown: naming a version is your decision. A release with no publish date never counts as old enough: `no release of RhythmShahriar/tuner is old enough: the newest, 0.3.3, has no publish date (min age 20 days)`. In `rhythm update --check --json`, `skipped_newer` names the release that was passed over. `rhythm list` always shows the real latest release.

A bad value exits 2: `rhythm: --min-age: invalid age "3"; use a duration such as 3d, 1w or 36h`.

## Lock a project's versions

`rhythm lock` writes `rhythm.lock` in the current folder from what is installed:

```console
$ rhythm lock
rhythm             dev is a development build; rhythm itself isn't locked
rhythm-echo        1.0.0  darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64
rhythm-soundcheck  1.1.0  darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64
rhythm-tuner       0.3.2  darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64
Wrote /home/me/site/rhythm.lock
```

For rhythm and each tool it records the version, the repository, the workflow that signed the release and the sha256 of every platform's archive, read from the release's signed `checksums.txt`. A release build of rhythm locks itself too; the `dev` line above is a build from source. Extensions installed with `rhythm ext install` are locked as well. For tools built on the shared kit it also refreshes their data and records each data pack's version; `--no-data` leaves that out.

Commit `rhythm.lock`. Any problem (a development build of a tool, a version that can't be read, a release without a signature) writes nothing and ends with `rhythm: rhythm.lock not written` (exit 1). `rhythm lock tuner` exits 2: `rhythm: lock takes no names; it locks everything installed`.

### Where rhythm looks for it

`rhythm install --locked`, `rhythm update --lock` and `rhythm bundle` use the nearest `rhythm.lock` in the current folder or a parent, up to and including the root of the git repository (a `.git` folder or file, so worktrees and submodules stop there too). Outside a repository only the current folder counts, so a stray lock in your home folder never applies. Without one, the message names what was checked:

```text
rhythm: no rhythm.lock in /home/me/site/web or its parents up to the repository root /home/me/site; write one with: rhythm lock
rhythm: no rhythm.lock in /home/me/site, the root of its git repository; write one with: rhythm lock
rhythm: no rhythm.lock in /tmp/scratch, which isn't in a git repository, so no other folder was checked; write one with: rhythm lock
```

### The lock file

```json
{
  "schema": "rhythm-lock/v1",
  "tools": [
    {
      "name": "tuner",
      "repo": "RhythmShahriar/tuner",
      "version": "0.3.2",
      "signer": "https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml@refs/heads/master",
      "platforms": {
        "darwin_amd64": "38eac9c4699895380f2b25122c2b2dfca79a93b174ee0a933d6ea0844c4fb3ee",
        "darwin_arm64": "5f30347231d19400e8c0ef33baf495dc89fd64f8b4de345b15f01aaf2957e3cc",
        "linux_amd64": "201c8643969bfb434cdf294aa19c5a1646f88fa34b62187619587ef5662b2f06",
        "linux_arm64": "0a68d53709d1a35543973d4cb7ee47db9d53e3fd4cfc85b1b3798cc9e333121b",
        "windows_amd64": "5f5383e8cfef62890005f09b569fc673bb156ccbe44fc612487024f6f8018e0e",
        "windows_arm64": "28a96c932b1d48fccaaccc9f44ca7d03d832f24e771dd562ef80bf5e07af3eeb"
      }
    }
  ]
}
```

(One tool shown; the file lists every tool, sorted by name.)

| Field | Holds |
|---|---|
| `schema` | `rhythm-lock/v1` |
| `rhythm` | rhythm itself, when a release build wrote the lock; the same fields as a tool |
| `tools[].name`, `repo` | the tool and the repository it comes from |
| `version` | an exact version, without `v` |
| `signer` | the workflow identity that signed the release; `install --locked` requires it again |
| `platforms` | `os_arch` to the sha256 of that platform's archive |
| `data` | for tools built on the shared kit: `name`, `version` and `sha256` of each data pack |
| `source` | `extension` for an extension; left out for rhythm's own tools |

rhythm reads the file strictly. A field it doesn't know, a schema it doesn't know (`<path>: schema "rhythm-lock/v2" isn't "rhythm-lock/v1"; a newer rhythm may read it`), a version that isn't exact (`tools[0] (zap): version "v1.0.0" isn't an exact version such as 1.2.3`), a hash that isn't 64 hex digits or a repository that isn't `owner/name` is refused, with every problem listed.

## Install what the lock pins

```console
$ rhythm install --locked
Using /home/me/site/rhythm.lock
rhythm-echo 1.0.0 is already installed at /home/me/.local/bin/rhythm-echo (use --force to reinstall)
rhythm-soundcheck 1.1.0 is already installed at /home/me/.local/bin/rhythm-soundcheck (use --force to reinstall)
rhythm-tuner 0.3.2 is already installed at /home/me/.local/bin/rhythm-tuner (use --force to reinstall)
```

Before installing anything, rhythm checks every tool: the release by its exact tag, the signer against the lock, `checksums.txt` against the lock and this platform's archive against its locked sha256. The install then uses exactly the bytes it checked. Any mismatch installs nothing:

```text
Using /home/me/site/rhythm.lock
rhythm-tuner       rhythm-tuner_linux_amd64.tar.gz: sha256 differs from the locked one; the release may have been replaced or tampered with, nothing was changed
Nothing was installed: the releases don't match rhythm.lock.
```

A tool counts as already installed only when it is byte for byte the binary in the locked archive; anything else is replaced. Name tools to install only those (`rhythm install --locked tuner`). A version named with `--locked` must be the locked one:

| You ran | rhythm says | Exit |
|---|---|---|
| `rhythm install --locked tuner@0.3.1` | `rhythm: rhythm.lock pins tuner 0.3.2, not 0.3.1; drop the version, or move the lock with: rhythm update --lock tuner` | 1 |
| `rhythm install --locked tuner@0.3` | `rhythm: --locked installs the versions in rhythm.lock; name an exact version or none in tuner@0.3` | 2 |
| `rhythm install --locked --defaults` | `rhythm: --locked installs the tools in rhythm.lock; drop --defaults` | 2 |
| `rhythm install --locked echo` (not in the lock) | `rhythm: rhythm.lock doesn't lock echo; add it with: rhythm install echo && rhythm lock` | 1 |

Other refusals: a lock entry for a tool this rhythm doesn't know (`rhythm: rhythm.lock locks zap, which this rhythm doesn't know; update rhythm first`), a tool moved to another repository (`rhythm: rhythm.lock says tuner comes from someone/tuner; rhythm installs it from RhythmShahriar/tuner`) and a platform the lock lacks (`rhythm-tuner       0.3.1: rhythm.lock has no archive for linux_arm64 (it locks darwin_arm64, linux_amd64)`). `rhythm install` never installs rhythm itself; when the lock pins another rhythm it says `Note: rhythm.lock pins rhythm 0.8.0 and this is rhythm 0.7.0.`

When a lock entry has data, rhythm then runs the tool's `update --json --pin` with those versions. A tool that can't pin its data says so and refreshes to the latest instead: `rhythm-tuner can't pin its data; refreshing it to the latest instead`.

On a fresh machine or CI runner, the installers do the same with `RHYTHM_LOCKED=1` from the folder that holds the lock. The GitHub Action does it with `locked: true`; see [Install](Install#installer-options) and [CI setup](CI-Setup).

## Move the lock forward

`rhythm update --lock` changes `rhythm.lock` only, never a binary. It looks up the latest release of everything the lock pins (or the tools named), prints what changed and rewrites the lock:

```console
$ rhythm update --lock --check
~ tuner 0.3.1 → 0.3.2
$ echo $?
10
$ rhythm update --lock
~ tuner 0.3.1 → 0.3.2
Updated /home/me/site/rhythm.lock
$ rhythm update --lock --check
/home/me/site/rhythm.lock is current
```

| Line | Means |
|---|---|
| `~ tuner 0.3.1 → 0.3.2` | a new version |
| `+ echo 0.1.0` | added |
| `- zap 1.0.0` | removed |
| `~ tuner data checks 1 → 2` | a data pack moved |
| `! soundcheck 0.4.2: archives changed for the same version` | a release was replaced after you locked it |
| `! soundcheck: signer <a> → <b> for the same version` | the same version is now signed by someone else |

The `!` lines deserve a look before you commit: a release should never change once it is out. With `--check` nothing is written; it exits 10 when the lock is behind and 0 when it is current, so a scheduled job can open a pull request when it exits 10. `--min-age` holds back young releases here too. The signer recorded for each entry is required for the next release. A lock that moves rhythm or a tool to another repository is refused before any lookup.

---
<sub>← [Command reference](Command-Reference) · [Home](Home) · [Offline installs](Offline-Installs) →</sub>
