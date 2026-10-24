Removing Rhythm means deleting the binaries, then, if you like, the cache and settings folders. Nothing else is installed: no services, no background jobs, no system-wide settings. If you used Echo, also remove its connectors from your sites, and stop trusting its certificate authority wherever you added it.

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
rm "$(go env GOPATH)/bin/rhythm" "$(go env GOPATH)/bin/rhythm-tuner" "$(go env GOPATH)/bin/rhythm-soundcheck" "$(go env GOPATH)/bin/rhythm-echo"
```

Then remove the cache and settings folders for your platform as above.

## What's in the cache and settings folders

Both are safe to delete at any time. Tuner downloads what it needs again on the next scan, Soundcheck downloads Chromium again if it needs it, and Echo makes a new CA the next time its proxy decrypts HTTPS (clients then have to trust the new one).

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
| settings `update-check/` | When each tool last looked for a new release, for the [once-a-day notice](Updating#the-new-release-notice) |
| settings `seen/` | Markers for notices shown once, such as the tools' responsible-use notes |

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

Echo keeps snapshots, runs and reports in a `.echo/` folder in each project where you ran it. They contain the messages your sites sent, with their personal data, so delete them when you no longer need them:

```sh
rm -rf .echo/snapshots .echo/runs .echo/.incoming   # keeps config.yaml
rm -r .echo                                        # everything
find ~ -type d -name .echo 2>/dev/null             # to find them
```

Echo added a `# Echo snapshots and reports` block to `.git/info/exclude`, which you can delete too.

## Echo's certificate authority

Delete the CA folder (`~/.cache/rhythm/echo/ca` on Linux, `~/Library/Caches/rhythm/echo/ca` on macOS, `%LOCALAPPDATA%\rhythm\echo\ca` on Windows), then remove it wherever you trusted it:

- a system store: delete `/usr/local/share/ca-certificates/rhythm-echo.crt` and run `sudo update-ca-certificates --fresh` (Debian, Ubuntu, Alpine);
- a bundle you built, such as `/etc/rhythm-echo/bundle.pem`, and the `curl.cainfo`, `openssl.cafile` or `env[]` lines pointing at it in php.ini or a php-fpm pool;
- `NODE_EXTRA_CA_CERTS`, `REQUESTS_CA_BUNDLE`, `SSL_CERT_FILE` or `CURL_CA_BUNDLE` in your environment or `.env` files;
- `HTTPS_PROXY` and `https_proxy` pointing at Echo's proxy, which would otherwise make requests fail once Echo is gone.

## Echo's connectors

Remove each site's connector, or its requests fail while sealed with Echo no longer running:

| Stack | Remove |
|---|---|
| WordPress | `rhythm echo install wordpress /var/www/staging --uninstall` (or with `--wp-cli` or `--container <name>`), before removing `rhythm-echo`. Or delete `wp-content/mu-plugins/rhythm-echo.php` and the `rhythm-echo/` folder next to it. Remove any `RHYTHM_ECHO_*` constants from `wp-config.php`. |
| Laravel | `composer remove rhythmsh/echo-laravel`, then delete the `RHYTHM_ECHO_*` lines from `.env`, `config/rhythm-echo.php` if you published it, and any `MAIL_HOST=127.0.0.1` / `MAIL_PORT=1025` you pointed at Echo. |
| Next.js, Node | `npm uninstall @rhythmsh/echo`, then remove the `register()` call from `instrumentation.ts` (or the file, if Echo wrote it), `withEcho` from `next.config`, `--import @rhythmsh/echo/register` from your scripts, and `RHYTHM_ECHO_*` variables. |
| Generic | Remove `sendmail_path = "… rhythm-echo sendmail …"` from php.ini or the pool, and SMTP settings pointing at `127.0.0.1:1025`. |
| Hard seal | `sudo nft delete table inet echo_seal`, or `sudo sh iptables-echo.sh off`; for Docker, `docker compose down` on the sealed stack. |

> [!TIP]
> Deleting `seen/` brings back the one-time responsible-use note on your next scan.

---
<sub>← [Updating](Updating) · [Home](Home) · [Verifying releases](Verifying-Releases) →</sub>
