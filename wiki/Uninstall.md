Removing Rhythm means deleting the binaries, then, if you like, the cache and settings folders. Nothing else is installed: no services, no background jobs, no system-wide settings.

## Linux

```sh
rm ~/.local/bin/rhythm ~/.local/bin/rhythm-*     # or /usr/local/bin, or your RHYTHM_INSTALL_DIR
rm -r ~/.cache/rhythm ~/.config/rhythm           # optional: cache and settings
```

If you set `XDG_CACHE_HOME` or `XDG_CONFIG_HOME`, the folders are `$XDG_CACHE_HOME/rhythm` and `$XDG_CONFIG_HOME/rhythm` instead.

## macOS

```sh
rm ~/.local/bin/rhythm ~/.local/bin/rhythm-*     # or /usr/local/bin, or your RHYTHM_INSTALL_DIR
rm -r ~/Library/Caches/rhythm "$HOME/Library/Application Support/rhythm"   # optional
```

> [!NOTE]
> On macOS the cache and settings live under `~/Library`, not `~/.cache` and `~/.config`.

## Windows

1. Delete the install folder, `%LOCALAPPDATA%\Programs\rhythm` (or your `RHYTHM_INSTALL_DIR`). It holds `rhythm.exe`, the tools, and any `.exe.old` files left by updates.
2. Remove that folder from your **user** `PATH`: **Settings → System → About → Advanced system settings → Environment Variables**, select `Path` under *User variables*, **Edit**, and delete the entry.
3. Optional: delete the cache and settings folders, `%LOCALAPPDATA%\rhythm` and `%APPDATA%\rhythm`.

The same in PowerShell:

```powershell
$dir = Join-Path $env:LOCALAPPDATA 'Programs\rhythm'
Remove-Item -Recurse -Force $dir
$path = ([Environment]::GetEnvironmentVariable('Path', 'User') -split ';' | Where-Object { $_ -and $_ -ne $dir }) -join ';'
[Environment]::SetEnvironmentVariable('Path', $path, 'User')
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\rhythm", "$env:APPDATA\rhythm" -ErrorAction SilentlyContinue
```

## Installed with Go

```sh
rm "$(go env GOPATH)/bin/rhythm" "$(go env GOPATH)/bin/rhythm-tuner"
```

Then remove the cache and settings folders for your platform as above.

## What's in the cache and settings folders

Both are safe to delete at any time. Tuner downloads what it needs again on the next scan.

| Folder | Linux | macOS | Windows |
|---|---|---|---|
| Cache | `~/.cache/rhythm` | `~/Library/Caches/rhythm` | `%LOCALAPPDATA%\rhythm` |
| Settings | `~/.config/rhythm` | `~/Library/Application Support/rhythm` | `%APPDATA%\rhythm` |

| Path inside | What it is |
|---|---|
| cache `tuner/vulns/` | Cached vulnerability data: WPVulnerability, wordpress.org, php.net and OSV.dev records |
| cache `tuner/checks/` | The latest verified [check pack](Tuner-Check-Packs) |
| settings `update-check/` | When each tool last looked for a new release, for the [once-a-day notice](Updating#the-new-release-notice) |
| settings `seen/` | Markers for notices shown once, such as Tuner's responsible-use note |

To clear only Tuner's cache, run `rhythm tuner update --clear`.

> [!TIP]
> Deleting `seen/` brings back the one-time responsible-use note on your next scan.

---
<sub>← [Updating](Updating) · [Home](Home) · [Verifying releases](Verifying-Releases) →</sub>
