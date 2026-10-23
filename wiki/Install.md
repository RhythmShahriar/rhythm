Install `rhythm` and Tuner with one command on macOS, Linux or Windows. This page also covers `go install`, manual downloads, the installer's options, and how to add a single tool later.

## Quick install

**macOS · Linux**

```sh
curl -fsSL https://rythm.sh/install | sh
```

**Windows** (PowerShell)

```powershell
irm https://rythm.sh/install.ps1 | iex
```

Both installers put `rhythm` and `rhythm-tuner` in the same folder and check every download against the release's `checksums.txt` before anything is written. Then run your first scan:

```sh
rhythm tuner scan yoursite.com
```

> [!IMPORTANT]
> Scan only sites you own or have permission to test. See [Responsible Use](Responsible-Use).

## What the installer does

1. Detects your OS (`linux` or `darwin`; Windows uses the PowerShell script) and CPU (`amd64` or `arm64`). Other platforms stop with an error.
2. Asks the GitHub API for the latest `rhythm` release, or the version in `RHYTHM_VERSION`.
3. Downloads `rhythm_<os>_<arch>.tar.gz` (`.zip` on Windows) and `checksums.txt`.
4. Compares the archive's SHA-256 with its line in `checksums.txt`. If the release has no `checksums.txt`, or the hash doesn't match, it stops and installs nothing.
5. Copies `rhythm` into the install directory. On macOS and Linux it writes a temporary file and renames it into place, so an existing `rhythm` is never left half-written.
6. Runs `rhythm install <tool> --dir <install dir>` for each tool in `RHYTHM_TOOLS` (default: `tuner`). That download is checked against the tool's own `checksums.txt` the same way.
7. Checks your `PATH`.

### Where it installs

| Platform | Default directory |
|---|---|
| macOS, Linux | `~/.local/bin` |
| macOS, Linux, run as root | `/usr/local/bin` |
| Windows | `%LOCALAPPDATA%\Programs\rhythm` |

### PATH

- **macOS and Linux:** the installer doesn't edit your shell profile. If the directory isn't on your `PATH`, it prints the line to add, for example:

  ```sh
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.profile
  ```

  Open a new terminal afterwards (or `source ~/.profile`).
- **Windows:** the installer adds the directory to your **user** `PATH` if it isn't there yet. Open a new terminal to pick it up.

### Requirements

- macOS and Linux: `curl`, `tar`, and `sha256sum` or `shasum`.
- Windows: PowerShell (Windows PowerShell 5.1 or PowerShell 7).

## Installer options

Set these environment variables to change what the installer does. Both scripts read the same names.

| Variable | Default | Effect |
|---|---|---|
| `RHYTHM_INSTALL_DIR` | see [Where it installs](#where-it-installs) | Install somewhere else. The directory is created if needed. |
| `RHYTHM_VERSION` | latest | Install a specific `rhythm` version, e.g. `0.1.0` (a leading `v` is fine). |
| `RHYTHM_TOOLS` | `tuner` | Space-separated tools to install alongside `rhythm`. Set it empty to install `rhythm` alone. |
| `RHYTHM_GITHUB_API` | `https://api.github.com` | GitHub API base, for a mirror or GitHub Enterprise. |
| `RHYTHM_ARCH` | — | Windows only: CPU to use when `PROCESSOR_ARCHITECTURE` is neither `AMD64` nor `ARM64`. |

```sh
# Install into /opt/bin, rhythm only
curl -fsSL https://rythm.sh/install | RHYTHM_INSTALL_DIR=/opt/bin RHYTHM_TOOLS="" sh

# Pin rhythm to a version
curl -fsSL https://rythm.sh/install | RHYTHM_VERSION=0.1.0 sh
```

```powershell
$env:RHYTHM_INSTALL_DIR = 'C:\Tools\rhythm'
irm https://rythm.sh/install.ps1 | iex
```

> [!NOTE]
> `RHYTHM_VERSION` pins `rhythm` only. Tools are always installed at their latest release, because each tool versions and releases on its own.

If the install directory isn't writable, the macOS/Linux installer stops with `can't write to <dir>; set RHYTHM_INSTALL_DIR or rerun with sudo`.

## Install a single tool

With `rhythm` installed, add a tool by name:

```sh
rhythm install tuner
```

```text
Installed rhythm-tuner 0.3.1 to /home/you/.local/bin/rhythm-tuner
```

| Flag | Default | Effect |
|---|---|---|
| `--dir <dir>` | the folder `rhythm` lives in | Where to put the tool binary. |

Short names work too: `rhythm install t` installs Tuner. The download is verified against the tool release's `checksums.txt` and swapped into place atomically, exactly like an update.

After installing, `rhythm` warns you if the tool won't be found:

- `Note: <dir> isn't on your PATH.` Add the directory to your `PATH`.
- `Note: <path> comes first on your PATH; remove it to use this install.` An older copy elsewhere shadows the new one.

## Install with Go

If you have Go installed:

```sh
go install github.com/RhythmShahriar/rhythm/cmd/rhythm@latest
go install github.com/RhythmShahriar/tuner/cmd/rhythm-tuner@latest
```

Binaries go to `$(go env GOPATH)/bin`.

> [!NOTE]
> `go install` builds report their version as `dev`. `rhythm update` skips development builds and they never show the "new release" notice, so update them by running `go install …@latest` again.

## Manual download

Every release has archives for Linux, macOS and Windows on `amd64` and `arm64`:

- `rhythm`: https://github.com/RhythmShahriar/rhythm/releases/latest
- Tuner: https://github.com/RhythmShahriar/tuner/releases/latest

Download the archive for your platform (for example `rhythm-tuner_linux_arm64.tar.gz` or `rhythm-tuner_windows_amd64.zip`) together with `checksums.txt`, check it as described in [Verifying Releases](Verifying-Releases), then extract the binary into a directory on your `PATH`. Keep `rhythm` and its tools in the same directory so `rhythm update` and `rhythm install` find them.

## Check it worked

```sh
rhythm version
rhythm tuner --version
```

```text
rhythm 0.1.0
rhythm-tuner 0.3.1
```

If you see `rhythm: tuner isn't installed`, run `rhythm install tuner`. If your shell can't find `rhythm` at all, the install directory isn't on your `PATH` yet; see [PATH](#path).

---
<sub>[Home](Home) · [Updating](Updating) →</sub>
