Removing Rhythm means deleting the binaries, then, if you like, the cache and settings folders. Nothing else is installed: no services, no background jobs, no system-wide settings. If you used Echo, also remove its connectors from your sites and stop trusting its certificate authority wherever you added it.

## With rhythm uninstall

```sh
rhythm uninstall --all rhythm --purge   # every tool, their caches and rhythm itself
```

`rhythm uninstall` removes the tools you name, or every tool with `--all`:

```console
$ rhythm uninstall tuner --purge
Removed /home/me/.local/bin/rhythm-tuner
Removed /home/me/.local/bin/rhythm-tuner.prev
Removed /home/me/.local/bin/rhythm-tuner.prev.json
Removed /home/me/.cache/rhythm/tuner
$ rhythm uninstall tuner
rhythm-tuner       isn't installed
```

| Flag | Effect |
|---|---|
| `--all` | remove every installed tool; add `rhythm` to remove rhythm itself too (last) |
| `--purge` | also remove each tool's cache folder (Tuner's vulnerability data and check pack, Soundcheck's Chromium (and the Chrome for Testing Echo's consent checks share with it), Echo's CA, Roadie's profile pack, signing key and spool, Sightread's archive cache) and its saved update check, even for a tool that is already gone |

For each tool it removes the copy that runs (the first on your `PATH`, plus the file a link points to), plus the `.prev`, `.prev.json` and `.old` files updates leave next to it. If another copy is still on your `PATH` afterwards it says `Note: another rhythm-tuner is still on your PATH at /usr/local/bin/rhythm-tuner`. A tool that isn't installed is not an error.

| Message | Exit |
|---|---|
| `rhythm: to remove rhythm itself, run: rhythm uninstall --all rhythm` | 2 |
| `usage: rhythm uninstall <tool>... [--purge] [--all]; name a tool, or use --all` | 2 |
| `rhythm: unknown tool "tunr"`<br>`  did you mean: rhythm uninstall tuner`<br>`  known: rhythm, echo, soundcheck, tuner` | 2, nothing removed |
| `rhythm-tuner is managed by Homebrew; remove it with: brew uninstall rhythm-tuner` | 1, nothing removed, not even with `--purge` |
| `rhythm-echo        can't remove /usr/local/bin/rhythm-echo; rerun with sudo` | 1 |

A tool a package manager installed is left to it, with the right command (`brew uninstall`, `nix profile remove`, `mise uninstall`, `asdf uninstall`, `scoop uninstall`, `winget uninstall`). On Windows the running `rhythm.exe` can't delete itself; it is renamed and the line says `(rhythm.exe.old is left; delete it once this command ends)`. Run from an administrator terminal, rhythm asks Windows to delete that file at the next restart and the line says `(rhythm.exe.old is left; Windows deletes it at the next restart)`. Either way the uninstall succeeds. Reinstalling with `install.ps1` removes a leftover `rhythm.exe.old` too.

`--purge` keeps the `seen/` markers, so a tool you remove isn't announced as new by the next `rhythm update`. Extensions go with `rhythm ext remove <name>`; see [Extensions](Extensions). Then remove the install folder from your `PATH` if you added it. Delete the settings folder if you like (see [below](#whats-in-the-cache-and-settings-folders)).

## By hand

With rhythm 0.6.0 or older, or if `rhythm` is already gone, delete the files yourself.

### Linux

```sh
rm ~/.local/bin/rhythm ~/.local/bin/rhythm-*     # or /usr/local/bin, or your RHYTHM_INSTALL_DIR
rm -r ~/.cache/rhythm ~/.config/rhythm           # optional: cache and settings
```

If you set `XDG_CACHE_HOME` or `XDG_CONFIG_HOME`, the folders are `$XDG_CACHE_HOME/rhythm` and `$XDG_CONFIG_HOME/rhythm` instead.

### macOS

```sh
rm ~/.local/bin/rhythm ~/.local/bin/rhythm-*     # or /usr/local/bin, or your RHYTHM_INSTALL_DIR
rm -r ~/Library/Caches/rhythm "$HOME/Library/Application Support/rhythm"   # optional
```

> [!NOTE]
> On macOS the cache and settings live under `~/Library`, not `~/.cache` and `~/.config`.

### Windows

1. Delete the install folder, `%LOCALAPPDATA%\Programs\rhythm` (or your `RHYTHM_INSTALL_DIR`). It holds `rhythm.exe`, the tools and any `.exe.old` files left by updates.
2. Remove that folder from your **user** `PATH`: **Settings → System → About → Advanced system settings → Environment Variables**, select `Path` under *User variables*, choose **Edit** and delete the entry.
3. Optional: delete the cache and settings folders, `%LOCALAPPDATA%\rhythm` and `%APPDATA%\rhythm`.

The same in PowerShell:

```powershell
$dir = Join-Path $env:LOCALAPPDATA 'Programs\rhythm'
Remove-Item -Recurse -Force $dir
$path = ([Environment]::GetEnvironmentVariable('Path', 'User') -split ';' | Where-Object { $_ -and $_ -ne $dir }) -join ';'
[Environment]::SetEnvironmentVariable('Path', $path, 'User')
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\rhythm", "$env:APPDATA\rhythm" -ErrorAction SilentlyContinue
```

### Installed with Go

```sh
rm "$(go env GOPATH)/bin/rhythm" "$(go env GOPATH)/bin/rhythm-tuner" "$(go env GOPATH)/bin/rhythm-soundcheck" "$(go env GOPATH)/bin/rhythm-echo" "$(go env GOPATH)/bin/rhythm-roadie" "$(go env GOPATH)/bin/rhythm-sightread"
```

Then remove the cache and settings folders for your platform as above.

## What's in the cache and settings folders

Both are safe to delete at any time. Tuner downloads what it needs again on the next scan. Soundcheck downloads Chromium again if it needs it. Echo makes a new CA the next time its proxy decrypts HTTPS (clients then have to trust the new one).

| Folder | Linux | macOS | Windows |
|---|---|---|---|
| Cache | `~/.cache/rhythm` | `~/Library/Caches/rhythm` | `%LOCALAPPDATA%\rhythm` |
| Settings | `~/.config/rhythm` | `~/Library/Application Support/rhythm` | `%APPDATA%\rhythm` |

| Path inside | What it is |
|---|---|
| cache `tuner/vulns/` | Cached vulnerability data: WPVulnerability, wordpress.org, php.net and OSV.dev records |
| cache `tuner/checks/` | The latest verified [check pack](Tuner-Check-Packs) |
| cache `soundcheck/browser/` | The Chromium Soundcheck downloaded, when no Chrome is installed; often the largest item |
| cache `echo/ca/` | Echo's certificate authority for its HTTPS proxy: `ca.pem` and its private key `ca-key.pem` |
| cache `roadie/profiles/`, `roadie/spool/`, `roadie/locks/` | Roadie's downloaded profile pack; the encrypted spool of a streamed source (removed when a run ends); one lock per target |
| settings `roadie/signing.key` | the key Roadie signs reports with; delete it and the next run makes a new one (reports signed before can still be checked against their own public key) |
| cache `sightread/` | Sightread's downloaded archives (by hash), registry answers and its last reports, plus `watch/<name>/` for each watch |
| settings `sightread/report-key`, `report-key.pub` | the key pair `rhythm sightread ... --sign` signs reports with |
| settings `update-check/` | When each tool last looked for a new release, for the [once-a-day notice](Updating#the-new-release-notice) |
| settings `seen/` | Markers for notices shown once, such as the tools' responsible-use notes and `New tool available`, plus Echo's `echo-consent-notice` and `echo-consent-tip-save` (the consent checks' once-only note and the `--save` tip) |
| settings `extensions.json`, `verified.json` | The extensions `rhythm ext` installed and the signature checks behind the SIGNED column |
| cache `rhythm/tuf/`, `rhythm/downloads/` | The Sigstore trusted root and the archives `--cache` keeps |

To clear only Tuner's cache, run `rhythm tuner update --clear`. To delete only Soundcheck's Chromium, run `rhythm soundcheck update --clear`.

## Soundcheck folders in your projects

Soundcheck keeps snapshots, runs and reports in a `.soundcheck/` folder in each project where you ran it. Uninstalling doesn't touch them. Delete them when you no longer need them:

```sh
rm -r .soundcheck/snapshots .soundcheck/runs .soundcheck/.incoming   # keeps config.yaml and flows/
rm -r .soundcheck                                                    # everything
```

`config.yaml` and `flows/` are project files you may have committed; the rest is excluded from git through `.git/info/exclude`, where Soundcheck added a `# Soundcheck snapshots and reports` block you can delete too. To find the folders:

```sh
find ~ -type d -name .soundcheck 2>/dev/null
```

## Echo folders in your projects

Echo keeps snapshots, runs and reports in a `.echo/` folder in each project where you ran it. They contain the messages your sites sent with their personal data; consent results in `.echo/consent/` hold screenshots of the sites you checked. Delete them when you no longer need them:

```sh
rm -rf .echo/snapshots .echo/runs .echo/.incoming   # keeps config.yaml
rm -rf .echo/consent                                 # consent results, baselines and detect screenshots
rm -r .echo                                        # everything
find ~ -type d -name .echo 2>/dev/null             # to find them
```

Echo added a `# Echo snapshots and reports` block to `.git/info/exclude`, which you can delete too.

## Roadie files in your projects

`rhythm roadie config init` writes `.roadie/config.yaml`, meant to be committed: it holds settings, never a secret. Everything else in `.roadie/` is git-excluded and holds data about production: `reports/` (signed reports naming tables, columns and masked examples), `backups/` (`prepare`'s backups of the staging database, real data if the staging held any), `key` (the consistency key, which can link fakes back to people) and `basic-auth/`. Delete them when you're done:

```sh
rm -r .roadie/reports .roadie/backups .roadie/key .roadie/basic-auth
```

On ssh staging hosts, `prepare` keeps its backups in `~/.roadie-backups/` on that host.

## Sightread files in your projects

`rhythm sightread init` writes `.sightread/config.yaml` and `allow` writes `.sightread/allowed.yaml`. They're meant to be committed: they hold your settings and your review record, no personal data. Reports you saved with `-o` hold your inventory; delete them when you're done.

```sh
rm -r .sightread    # the config and the review record
```

## Echo's certificate authority

Delete the CA folder (`~/.cache/rhythm/echo/ca` on Linux, `~/Library/Caches/rhythm/echo/ca` on macOS, `%LOCALAPPDATA%\rhythm\echo\ca` on Windows), then remove it wherever you trusted it:

- a system store: delete `/usr/local/share/ca-certificates/rhythm-echo.crt` and run `sudo update-ca-certificates --fresh` (Debian, Ubuntu, Alpine);
- a bundle you built, such as `/etc/rhythm-echo/bundle.pem`, plus the `curl.cainfo`, `openssl.cafile` or `env[]` lines pointing at it in php.ini or a php-fpm pool;
- `NODE_EXTRA_CA_CERTS`, `REQUESTS_CA_BUNDLE`, `SSL_CERT_FILE` or `CURL_CA_BUNDLE` in your environment or `.env` files;
- `HTTPS_PROXY` and `https_proxy` pointing at Echo's proxy, which would otherwise make requests fail once Echo is gone.

## Echo's connectors

Remove each site's connector, or its requests fail while sealed with Echo no longer running:

| Stack | Remove |
|---|---|
| WordPress | `rhythm echo install wordpress /var/www/staging --uninstall` (or with `--wp-cli` or `--container <name>`), before removing `rhythm-echo`. Or delete `wp-content/mu-plugins/rhythm-echo.php` and the `rhythm-echo/` folder next to it. Remove any `RHYTHM_ECHO_*` constants from `wp-config.php`. |
| Laravel | `composer remove rhythmsh/echo-laravel`, then delete the `RHYTHM_ECHO_*` lines from `.env`, `config/rhythm-echo.php` if you published it and any `MAIL_HOST=127.0.0.1` / `MAIL_PORT=1025` you pointed at Echo. |
| Next.js, Node | `npm uninstall @rhythmsh/echo`, then remove the `register()` call from `instrumentation.ts` (or the file, if Echo wrote it), `withEcho` from `next.config`, `--import @rhythmsh/echo/register` from your scripts and `RHYTHM_ECHO_*` variables. |
| Generic | Remove `sendmail_path = "… rhythm-echo sendmail …"` from php.ini or the pool. Also remove SMTP settings pointing at `127.0.0.1:1025`. |
| Hard seal | `sudo nft delete table inet echo_seal`, or `sudo sh iptables-echo.sh off`; for Docker, `docker compose down` on the sealed stack. |

> [!TIP]
> Deleting `seen/` brings back the one-time responsible-use note on your next scan.

---
<sub>← [Updating](Updating) · [Home](Home) · [Verifying releases](Verifying-Releases) →</sub>
