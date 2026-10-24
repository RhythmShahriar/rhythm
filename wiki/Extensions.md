Anyone can publish a tool that `rhythm <name>` runs. A verified extension is installed with `rhythm ext`, signature checked, kept current by `rhythm update` and pinned by `rhythm lock` like rhythm's own tools. The second half of this page is the contract for extension authors.

> [!IMPORTANT]
> An extension is someone else's code, running with your permissions. The signature check proves which repository and workflow built a release, not that the code is safe. Read what it does before you install it. Use it only on sites you own or may test ([Responsible use](Responsible-Use)).

## Use an extension

| Command | Does |
|---|---|
| `rhythm ext search [words...] [--json]` | find extensions on GitHub, most starred first |
| `rhythm ext install owner/rhythm-<name>[@version] [--dir dir] [--force] [--quiet]` | install one, signature checked |
| `rhythm ext list [--json]` | show the installed extensions |
| `rhythm ext remove <name>` | delete one and forget it |
| `rhythm <name> [args]` | run it |

`rhythm ext search` asks GitHub for repositories with the topic `rhythm-extension` whose names start with `rhythm-`. It prints `NAME REPO STARS DESCRIPTION` and ends with `Install one with: rhythm ext install <first result you can install>`; names rhythm keeps for itself are shown as `tuner (name taken)`. With no match it says `No extensions found.` and with nothing installed `rhythm ext list` says `No extensions installed. Find some with: rhythm ext search`.

`rhythm ext install` puts the extension next to rhythm (or in `--dir`), records it in `extensions.json` in rhythm's config folder and prints the Installed line, the `Signature ok` and `Checksum ok` lines and `Run it with: rhythm <name>`. There is no `--skip-signature` here: an unsigned or wrongly signed release is refused with nothing installed or recorded.

After that the extension behaves like one of rhythm's tools:

- `rhythm update` updates it with the tools; `rhythm update <name>` and `--rollback` work for it;
- `rhythm list` shows it with its SIGNED status and `rhythm verify` checks it;
- `rhythm lock` pins it (`"source": "extension"` with its signer), so `rhythm install --locked` brings it back on a fresh machine;
- shell completion completes its commands when it is built on the shared kit.

`rhythm ext remove zap` deletes the binary, the `.prev` copy its last update kept and its entry in `extensions.json`.

### What gets refused

rhythm's own names can't be taken, so an extension can never stand in for Tuner, Soundcheck or Echo. Each of these exits 2 before any request:

```text
rhythm: can't install someone/rhythm-tuner as an extension: tuner is one of rhythm's own tools (RhythmShahriar/tuner)
rhythm: can't install someone/rhythm-sc as an extension: sc is short for rhythm's own soundcheck
rhythm: can't install someone/rhythm-list as an extension: list is a rhythm command (rhythm list)
rhythm: someone/zap isn't an extension repository: its name must start with rhythm-, such as someone/rhythm-zap
rhythm: "x" isn't owner/rhythm-<name>, such as someone/rhythm-zap
```

A name that is already installed from another repository exits 1 with `rhythm: zap is already installed from someone/rhythm-zap; remove it first with: rhythm ext remove zap`. A lock entry can't use an extension to replace a tool either: `rhythm: rhythm.lock lists the extension echo from mallory/rhythm-echo, which can't be installed: echo is one of rhythm's own tools (RhythmShahriar/echo)`.

The first install records which workflow signed the release. Every later release must come from that same workflow, at any tag, or the update is refused and the installed binary stays:

```text
refusing to install someone/rhythm-zap 1.1.0: signed by <the new workflow>, not <the recorded one>
```

`rhythm ext remove tuner` points you at `rhythm uninstall tuner`; a name that isn't installed gives `rhythm: no extension "zap" is installed (see rhythm ext list)`.

## Other rhythm-* programs

`rhythm <name>` runs any executable `rhythm-<name>` on your `PATH`, installed with `rhythm ext` or not. Programs rhythm didn't install are listed under **Other tools** in `rhythm help` and as `(3rd)` in `rhythm list`, with `-` under SIGNED. rhythm never updates them, never runs them for completion and asks them only `--version` for `rhythm list`.

## Publish an extension

The contract is the one rhythm's own tools follow, so an extension built like them works with every command above.

| Part | Rule |
|---|---|
| Repository | `owner/rhythm-<name>` on GitHub, public, with the topic `rhythm-extension`. `<name>` is lowercase letters, digits and `-`. It can't be one of rhythm's tools, short names or commands. |
| Binary | `rhythm-<name>`; `rhythm-<name> --version` prints `rhythm-<name> <version>`. |
| Releases | tags `v<version>` (semantic versions); prereleases are never offered as latest |
| Archives | `rhythm-<name>_<os>_<arch>.tar.gz`, `.zip` on Windows, for the platforms you support (`linux`, `darwin`, `windows` × `amd64`, `arm64`) |
| Checksums | `checksums.txt` with the SHA-256 of every archive |
| Signature | `checksums.txt.sigstore.json`: a keyless Sigstore bundle over `checksums.txt`, made by a GitHub Actions workflow of the extension's own repository at the release tag, or by rhythm's shared `release-go.yml` called from it |

The GoReleaser config of [rhythm](https://github.com/RhythmShahriar/rhythm/blob/master/.goreleaser.yaml) produces exactly this layout: set `project_name` to `rhythm-<name>` and sign `checksums.txt` with `cosign sign-blob --bundle` from a workflow with `id-token: write`. rhythm checks the certificate's workflow, that its source repository is yours and that its ref is the release tag. It records the workflow without its ref (for example `https://github.com/someone/rhythm-zap/.github/workflows/release.yml`), so the next tag still matches. Moving the release to another workflow file later means users have to remove and reinstall the extension.

### Working well with rhythm

These are optional. They come for free when the extension is built on the shared kit's `kit/cli`:

- **Its name in help.** rhythm sets `RHYTHM_INVOKED_AS` to what the user typed, such as `rhythm zap`; use it in usage lines and errors.
- **Version JSON.** `rhythm-<name> version --json` printing `{"schema":"rhythm-version/v1","name":"rhythm-zap","version":"1.2.0","kit_version":"v0.7.0"}` lets rhythm read the version reliably.
- **Completion.** With that document and a non-empty `kit_version`, rhythm forwards Tab to `rhythm-<name> __complete <words...>`, which prints one candidate per line. It runs with an empty stdin and a two-second limit and must never do any work beyond printing.
- **Data.** A tool with data answers `update --json` with a `rhythm-data/v1` document (`{"schema":"rhythm-data/v1","data":[{"name","version","sha256"}]}`). `rhythm update` then shows one data line for it, `rhythm lock` records the packs and `rhythm doctor` checks their age. Supporting `update --pin name=version,...` lets `rhythm install --locked` restore the locked data.
- **Exit codes.** 0 success, 1 findings, 2 usage error, 3 the run failed, 130 interrupted; rhythm passes them through.

See [Architecture](Architecture#the-kit) for the kit packages.

---
<sub>← [CI setup](CI-Setup) · [Home](Home) · [Tuner overview](Tuner) →</sub>
