Every Echo setting for a project lives in `.echo/config.yaml`. This page covers how commands find it, every key with its default, the environment variables Echo and its connectors read and what's kept in the `.echo` folder.

## The config file

```sh
rhythm echo init                   # write .echo/config.yaml, with an action for this folder's stack
rhythm echo init --stack laravel   # wordpress, laravel, next, node or php
rhythm echo init --force           # replace an existing one
```

```text
Wrote .echo/config.yaml for a WordPress site, with one action to start from (Lost password).
Set site: in it to your staging site's URL; the action's URL is relative to it.
Then: rhythm echo up, rhythm echo doctor and rhythm echo snapshot.
```

`init` looks at the folder's files to pick the stack (`wp-config.php` or `wp-load.php` for WordPress, `artisan` for Laravel, `next.config.*` for Next.js, `package.json` for Node, any `.php` file for plain PHP) and writes one working action for it, plus two commented examples: a lost-password request on WordPress, `php artisan echo:canary` on Laravel, a sign-up POST on Next.js and Node, the home page otherwise. It fills `site:` from `WP_HOME`/`WP_SITEURL` in `wp-config.php` or `APP_URL` in `.env` when it finds them. A folder with no stack gets `Wrote .echo/config.yaml with one action to start from (Home page).` A wrong `--stack` is a usage error: `rhythm echo: --stack "rails": use wordpress, laravel, next, node or php`.

### How it's found

Every command looks for `.echo/config.yaml` in the folder you're in, then in each folder above it, as git finds `.git`, so commands work from any subfolder of the project:

- the search stops at the repository root (the folder holding `.git`, which is searched) and, outside a repository, below your home folder (home itself is searched only when you're in it);
- a folder above the one you're in is skipped when it isn't yours (or root's), or anyone can write to it (`/tmp`), since a config there could run someone else's commands;
- `--config <file>` picks a file explicitly.

```text
rhythm echo: not using /tmp/ww/.echo/config.yaml: its folder is writable by anyone (someone else could have put it there); pass --config /tmp/ww/.echo/config.yaml if you trust it
```

From a subfolder, `snapshot`, `compare`, `run` and `doctor` say which config they use, since its `command` and `flush` actions run in the project folder, not where you are:

```text
rhythm echo: using /tmp/g/.echo/config.yaml; its flush and command actions run in /tmp/g
```

`snapshot`, `compare`, `run` and `doctor` need a config: without one, `rhythm echo: no .echo/config.yaml here or above; run rhythm echo init` (exit `2`). `up`, `watch`, `ls`, `show`, `clear`, `replay`, `check`, `headers`, `report` and `snapshots` work without one, on the defaults below. Snapshots and runs are kept next to the config that was found (`proj/.echo/` for `proj/.echo/config.yaml`); `--store <dir>` keeps them elsewhere.

### Errors

Every key is optional. Unknown keys are errors, so a typo fails loudly instead of being ignored. Every error names the file and the line:

```text
rhythm echo: /tmp/t2/.echo/config.yaml: line 1: quiet: 45s is over the 30s Echo waits for quiet at most
rhythm echo: /tmp/t2/.echo/config.yaml: line 4: expect: "mail" isn't a channel; use email, sms, whatsapp, chat, webhook, analytics, payment, push, http
rhythm echo: /tmp/t2/.echo/config.yaml: line 1: production: "https://" isn't a URL or host name
rhythm echo: /tmp/t1/.echo/config.yaml: line 2: checks: content/devlink isn't a check; rhythm echo checks lists them
```

These exit with code `2`. Command-line flags override the file: `--seal`/`--observe` override `mode`, `--listen`, `--smtp` and `--proxy` their keys, `--mitm` turns `mitm` on, `--allow` adds to `allow`, `--store`, `--keep` and `--keep-for` override `inbox:` and `--quiet` and `--timeout` override `quiet` and `timeout`.

Commit `config.yaml` to share it with your team. Keep secrets out of it: tokens, signing secrets and the relay's SMTP account come from the environment.

## Every key

The file `init` writes, up to its actions:

```yaml
# Echo settings. Everything here is optional; delete what you don't change.
# Reference: https://github.com/RhythmShahriar/rhythm/wiki/Echo-Configuration
# Commit this file to share it; snapshots and runs are kept out of git.

site: ""                # base URL for relative action URLs, e.g. https://staging.shop.test
mode: seal              # seal: nothing leaves, Echo answers instead. observe: record and send for real
listen: 127.0.0.1:8025  # API and web view (connectors post here)
smtp: 127.0.0.1:1025    # SMTP sink; "off" to disable
proxy: ""               # HTTP(S) proxy for SDKs that skip the connector, e.g. 127.0.0.1:8026
mitm: false             # decrypt HTTPS through the proxy (trust the CA from: rhythm echo ca)

allow: []               # hosts reachable even when sealed, e.g. ["api.stripe.com"] (test-mode keys only)
allow_recipients: []    # in observe mode, recipients you may really message, e.g. ["*@yourcompany.com"]
staging: []             # URL prefixes that are staging; replay only sends to these and local addresses
production: []          # live sites, e.g. ["shop.com"] (subdomains too); actions and replays refuse them

flush: []               # run after each action so queued mail and webhooks go out now, e.g.
                        #   ["wp cron event run --due-now", "wp action-scheduler run"]
                        #   ["php artisan queue:work --stop-when-empty"]
quiet: 2s               # an action is done when nothing new arrives for this long (at most 30s)
timeout: 30s            # longest wait per action (at least quiet, at most 1m)

# Client sign-off: up lets Echo Inbox's Release button (or rhythm echo release
# <id> --to <address>) send one caught email through your SMTP account, only to
# allow. It needs --token or --login. Keep the account out of this file:
# RHYTHM_ECHO_RELAY=smtp://user:pass@smtp.agency.example:587
# relay:
#   allow: ["*@client.example"]
```

| Key | Default | Description |
|---|---|---|
| `site` | `""` | The site's base URL. Relative action and replay URLs resolve against it; `doctor` checks the WordPress connector there; the content checks use its host to tell your links and your sending domain from others. |
| `mode` | `seal` | `seal`: nothing leaves, Echo answers. `observe`: record and send for real. See [Sealing staging](Echo-Sealing-Staging#seal-and-observe). |
| `listen` | `127.0.0.1:8025` | The capture API and Echo Inbox. Other commands find Echo here too, unless `--url` or `RHYTHM_ECHO_URL` says otherwise. An address beyond loopback needs a token, or `--private-network`. |
| `smtp` | `127.0.0.1:1025` | The SMTP sink. `off` turns it off. Beyond loopback it needs a token and asks for it as the SMTP password. |
| `proxy` | `""` (off) | The HTTP(S) proxy, e.g. `127.0.0.1:8026`. Beyond loopback it needs a token. |
| `mitm` | `false` | Decrypt HTTPS in the proxy with Echo's CA, so sealed HTTPS can be recorded and faked. Clients must trust the CA (`rhythm echo ca`). |
| `allow` | `[]` | Host globs reachable for real even when sealed. Live payment calls are refused anyway. See [The allowlist](Echo-Sealing-Staging#the-allowlist). |
| `allow_recipients` | `[]` | Address or number globs that may really be messaged in observe mode; any other real recipient is flagged. |
| `staging` | `[]` | URL prefixes that are staging. Replays send only to these, or to targets that resolve to loopback or private addresses. See [Replay](Echo-Replay#safety-rules). |
| `production` | `[]` | Your live sites, as hosts or URLs, subdomains included. Actions, replays and `doctor` refuse them. See [Production](Echo-Actions-And-Compare#production). |
| `flush` | `[]` | Commands run after each action (with `sh -c` or `cmd /C` on Windows, with `RHYTHM_ECHO_RUN` set), so queued mail and webhooks go out during it. See [Flush queued work](Echo-Actions-And-Compare#flush-queued-work). |
| `quiet` | `2s` | An action is done when nothing new has arrived for this long. At most `30s`. |
| `timeout` | `30s` | The longest wait per action and the timeout of its `http` request. At least `quiet`, at most `1m`. |
| `checks` | `{}` | A check's level, or `off`, by its ID. See [Checks and rules](Echo-Checks-And-Rules#tuning-a-check). |
| `rules` | `[]` | Checks of your own. See [Checks and rules](Echo-Checks-And-Rules#rules-of-your-own). |
| `inbox` | memory | `{store, keep, keep_for}`: keep what `up` catches on disk. See [Echo Inbox](Echo-Inbox#keeping-the-inbox-across-restarts). |
| `relay` | none | `{allow, url}`: the client sign-off relay. `url` is better left to `RHYTHM_ECHO_RELAY`. See [Releasing an email](Echo-Inbox#releasing-an-email-for-sign-off). |
| `hub` | | Reserved; accepted and ignored. |
| `actions` | `[]` | What `snapshot`, `compare` and `run` run. |

Durations are Go durations (`1500ms`, `2s`, `1m`) with days too (`14d`, `1d12h`).

### actions

| Key | Description |
|---|---|
| `name` | Required, unique, with a letter or digit from `a` to `z` or `0` to `9`. The action's ID for `compare` is `action:` and the name lowercased. |
| `http` | `{method, url, headers, form, json, body}`: a request to the site. |
| `command` | A shell command, run in the project folder. |
| `replay` | `{event, to, set, secret_env}`: a signed webhook. |
| `env` | Extra environment variables for a `command`. |
| `expect` | The least it must send, per channel (`email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push`, `http`), as a count or counts per event: `{email: 1, analytics: {purchase: 1}}`. |
| `ignore` | Globs of payload fields, headers and recipients `compare` leaves out for this action. |
| `quiet` | This action's own quiet window. |
| `parallel` | `false`: never runs beside another action. |

Exactly one of `http`, `command` and `replay`. Every field is explained in [Actions and compare](Echo-Actions-And-Compare#actions).

## Environment variables

Read by `rhythm-echo`:

| Variable | Description |
|---|---|
| `RHYTHM_ECHO_URL` | Where Echo runs, for every command but `up` (which uses `listen`); default `http://` + `listen`. `--url` wins. |
| `RHYTHM_ECHO_TOKEN` | The token `up` requires and other commands send. `--token` wins with a warning, since other users can read a flag in `ps`. Never written to reports. |
| `RHYTHM_ECHO_WRITE_TOKEN` | A site's write token on a hub: signs the run IDs actions send when set. |
| `RHYTHM_ECHO_PRIVATE_NETWORK` | `1` is `up --private-network`. |
| `RHYTHM_ECHO_HOSTNAMES` | Comma-separated names `up` answers to without a token, as `--hostname`. |
| `RHYTHM_ECHO_LOGIN` | `user:password` for Echo Inbox's viewer sign-in, as `--login`. |
| `RHYTHM_ECHO_RELAY` | The relay's SMTP account, `smtp://user:pass@host:587` or `smtps://…:465`. |
| `ECHO_STRIPE_WEBHOOK_SECRET`, `ECHO_WEBHOOK_SECRET`, `ECHO_GITHUB_WEBHOOK_SECRET`, `ECHO_WC_WEBHOOK_SECRET` | Default signing secrets for [replay](Echo-Replay#secrets), per scheme. |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day "new release available" check. |
| `CI` | When set, as CI systems do, the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks, to avoid GitHub's anonymous rate limit. |

`hub` and `schedule` read the token variables `sites.yaml` names; see [Agency hub and reports](Echo-Hub-And-Reports).

Read by the connectors and `rhythm-echo sendmail`, in the site's environment:

| Variable | Read by | Description |
|---|---|---|
| `RHYTHM_ECHO_URL` | all connectors, `sendmail` | Echo's capture API as the site reaches it. |
| `RHYTHM_ECHO_TOKEN` | all connectors, `sendmail` | Sent as `X-Echo-Token`; also checks the signature of `X-Echo-Run`. |
| `RHYTHM_ECHO_WRITE_TOKEN` | all connectors | A hub site's write token, sent instead of `RHYTHM_ECHO_TOKEN`. |
| `RHYTHM_ECHO_SITE_ID` | all connectors | The site's ID on a hub, on every capture. |
| `RHYTHM_ECHO_RUN` | all connectors, `sendmail` | `<run>/<action>` for work outside a request. Echo sets it on the commands it runs. |
| `RHYTHM_ECHO_FORCE` | all connectors, `sendmail` | Run even where the environment says production. |
| `RHYTHM_ECHO_MODE` | WordPress, Laravel | What to do when Echo can't be reached: `seal` (default) or `observe`. |
| `RHYTHM_ECHO_PROXY`, `RHYTHM_ECHO_CA` | WordPress | Echo's proxy and CA for raw cURL. |
| `RHYTHM_ECHO_BIN`, `RHYTHM_ECHO_PROJECT` | Echo Staging Seal | The `rhythm-echo` binary and the project folder its buttons and `wp echo` use. |
| `RHYTHM_ECHO_ENABLED`, `RHYTHM_ECHO_TIMEOUT`, `RHYTHM_ECHO_GUZZLE` | Laravel | See [Laravel](Echo-Install-Connectors#laravel). |
| `RHYTHM_ECHO_SITE` | Node | The site's own hosts or URLs. |
| `RHYTHM_ECHO_SMTP` | `sendmail` | The SMTP sink; see [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in). |
| `WP_ENVIRONMENT_TYPE`, `APP_ENV`, `VERCEL_ENV`, `NODE_ENV` | connectors, `sendmail` | The production guard. |

The installer variables (`RHYTHM_INSTALL_DIR`, `RHYTHM_VERSION`, `RHYTHM_TOOLS`) are in [Install](Install#installer-options).

## The .echo folder

```text
.echo/
├── config.yaml                 # your settings: commit it
├── snapshots/
│   ├── before-update/          # run.json, report.html
│   ├── .incoming-<name>/       # a snapshot being written; moved into place when complete
│   └── .latest                 # the name compare uses by default
└── runs/
    └── 2026-10-25T14-23-14-f6f2df/   # run.json, comparison.json, report.html
```

| What | Where |
|---|---|
| Settings | `.echo/config.yaml` |
| Snapshots | `.echo/snapshots/<name>/`, never pruned |
| Runs and comparisons | `.echo/runs/<date-time>-<id>/`, the 20 newest; two runs in the same second get folders of their own |
| The latest snapshot's name | `.echo/snapshots/.latest`; when it's missing or names a deleted snapshot, the newest snapshot is used. An older `LATEST` file is moved to `.latest` once. |

The folders are `0700` and the files in them `0600`, readable by you only; `config.yaml` stays `0644`, since it's meant to be shared. Runs are stored as JSON with the schema `rhythm-echo/v1`; a run or snapshot taken by Echo 1.0 still compares. Inside a git repository, the first command that writes there adds this block to `.git/info/exclude`, so captured messages are never committed by accident:

```text
# Echo snapshots and reports
/.echo/snapshots/
/.echo/runs/
/.echo/snapshots/.incoming-*
```

An exclude file an older version wrote gets only the line it lacks. The folder is the one holding the config that was found, or `--store <dir>`.

### Per user

| What | Linux | macOS | Windows |
|---|---|---|---|
| The proxy's CA (`ca.pem`, `ca-key.pem`) | `~/.config/rhythm/echo/ca` (or `$XDG_CONFIG_HOME/rhythm/echo/ca`) | `~/Library/Application Support/rhythm/echo/ca` | `%AppData%\rhythm\echo\ca` |
| Release check state | `~/.config/rhythm/update-check/rhythm-echo.json` | `~/Library/Application Support/rhythm/update-check/…` | `%AppData%\rhythm\update-check\…` |
| First-run notice marker | `~/.config/rhythm/seen/echo-authorised-use` | `~/Library/Application Support/rhythm/seen/…` | `%AppData%\rhythm\seen\…` |

The CA is made the first time something needs it (`up --proxy … --mitm` or `ca`; `install` only prints where it will be) and lasts one year; its key is readable only by you. 30 days before it runs out Echo makes a new one. It keeps signing with the old one for those 30 days, so clients that trust only the old one keep working while you trust the new one: `ca.pem` holds both meanwhile. A CA from Echo 1.0 (in the cache folder `~/.cache/rhythm/echo/ca`, valid for ten years) is moved to the config folder (where cleaners don't wipe it) and replaced by a one-year CA:

```sh
rhythm echo ca
```

```text
rhythm echo: moved Echo's CA from /home/you/.cache/rhythm/echo/ca to /home/you/.config/rhythm/echo/ca, out of the cache folder that cleaners wipe; point NODE_EXTRA_CA_CERTS, REQUESTS_CA_BUNDLE, SSL_CERT_FILE or --cacert at /home/you/.config/rhythm/echo/ca/ca.pem
/home/you/.config/rhythm/echo/ca/ca.pem
```

`up --mitm` shows the same as a `CA:` line in its banner. Repoint every client that trusts the old path.

`rhythm echo ca --untrust` prints how to remove the CA from every trust store you may have added it to; see [CLI reference](Echo-CLI-Reference#ca-and-sendmail). When this machine's own system store trusts Echo's CA, `ca` and `up --mitm` warn: every program here would then accept certificates Echo signs, so trust it only inside containers.

---
<sub>← [CI](Echo-CI) · [Home](Home) · [API](Echo-API) →</sub>
