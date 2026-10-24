Every Echo setting for a project lives in `.echo/config.yaml`. This page lists every key with its default, the environment variables Echo and its connectors read and what's kept in the `.echo` folder.

## The config file

```sh
rhythm echo init           # write .echo/config.yaml with every setting
rhythm echo init --force   # replace an existing one
```

```text
Wrote .echo/config.yaml. Every setting is optional; add actions to snapshot and compare.
```

Echo reads `.echo/config.yaml` from the folder you run it in, for every command, `up` included; `--config <file>` reads another file. Without a file, the defaults below apply. Every key is optional. Unknown keys are errors, so a typo fails loudly instead of being ignored:

```text
rhythm echo: .echo/config.yaml: yaml: unmarshal errors:
  line 1: field sit not found in type config.Config
rhythm echo: mode: "sealed" isn't seal or observe
rhythm echo: .echo/config.yaml: line 1: "2" isn't a duration like 2s
rhythm echo: action "A" is listed twice
```

These exit with code `2`. Command-line flags override the file: `--seal`/`--observe` override `mode`, `--listen`, `--smtp` and `--proxy` their keys, `--mitm` turns `mitm` on and `--allow` adds to `allow`.

Commit `config.yaml` to share it with your team. Keep secrets out of it: tokens and signing secrets come from the environment.

## Every key

The file `init` writes:

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
production: []          # URL prefixes of live sites; actions refuse them

flush: []               # run after each action so queued mail and webhooks go out now, e.g.
                        #   ["wp cron event run --due-now", "wp action-scheduler run"]
                        #   ["php artisan queue:work --stop-when-empty"]
quiet: 2s               # an action is done when nothing new arrives for this long
timeout: 30s            # longest wait per action

# What snapshot and compare run. Each action makes the site send something.
# Form submits and multi-step journeys are better run by Soundcheck with --echo.
actions: []
#  - name: Contact form
#    http: {method: POST, url: /wp-admin/admin-ajax.php, form: {action: contact, email: "${fake:email}", message: Hello}}
#    expect: {email: 2}
#  - name: Completed order
#    command: wp eval 'do_action("woocommerce_order_status_completed", 123);'
#  - name: Stripe payment succeeded
#    replay: {event: payment_intent.succeeded, to: /?wc-api=wc_stripe, secret_env: ECHO_STRIPE_WEBHOOK_SECRET}
```

| Key | Default | Description |
|---|---|---|
| `site` | `""` | The site's base URL. Relative action and replay URLs resolve against it; `doctor` checks the WordPress connector there; the content checks use its host to tell your links and your sending domain from others. |
| `mode` | `seal` | `seal`: nothing leaves, Echo answers. `observe`: record and send for real. See [Sealing staging](Echo-Sealing-Staging#seal-and-observe). |
| `listen` | `127.0.0.1:8025` | The capture API and live view. Other commands find Echo here too, unless `--url` or `RHYTHM_ECHO_URL` says otherwise. An address beyond loopback needs a token. |
| `smtp` | `127.0.0.1:1025` | The SMTP sink. `off` turns it off. Beyond loopback it needs a token and asks for it as the SMTP password. |
| `proxy` | `""` (off) | The HTTP(S) proxy, e.g. `127.0.0.1:8026`. Beyond loopback it needs a token. |
| `mitm` | `false` | Decrypt HTTPS in the proxy with Echo's CA, so sealed HTTPS can be recorded and faked. Clients must trust the CA (`rhythm echo ca`). |
| `allow` | `[]` | Host globs reachable for real even when sealed. Live payment calls are refused anyway. See [The allowlist](Echo-Sealing-Staging#the-allowlist). |
| `allow_recipients` | `[]` | Address or number globs that may really be messaged in observe mode; any other real recipient is flagged. |
| `staging` | `[]` | URL prefixes that are staging. Replays send only to these, or to targets that resolve to loopback or private addresses. See [Replay](Echo-Replay#safety-rules). |
| `production` | `[]` | URL prefixes of live sites. `http` actions refuse them. `doctor` fails when `site` is one of them. |
| `flush` | `[]` | Commands run after each action (with `sh -c` and `RHYTHM_ECHO_RUN`), so queued mail and webhooks go out during it. See [Flush queued work](Echo-Actions-And-Compare#flush-queued-work). |
| `quiet` | `2s` | An action is done when nothing new has arrived for this long. A Go duration: `1500ms`, `2s`, `1m`. |
| `timeout` | `30s` | The longest wait per action and the timeout of its `http` request. |
| `actions` | `[]` | What `snapshot`, `compare` and `run` run. |

### actions

| Key | Description |
|---|---|
| `name` | Required, unique. Also the action's ID for `compare`, lowercased. |
| `http` | `{method, url, headers, form, json, body}`: a request to the site. |
| `command` | A shell command, run with `sh -c`. |
| `replay` | `{event, to, set, secret_env}`: a signed webhook. |
| `env` | Extra environment variables for a `command`. |
| `expect` | `{email, sms, webhook, chat}`: the least the action must send per channel. |

Exactly one of `http`, `command` and `replay`. Every field is explained in [Actions and compare](Echo-Actions-And-Compare#actions).

## Environment variables

Read by `rhythm-echo`:

| Variable | Description |
|---|---|
| `RHYTHM_ECHO_URL` | Where Echo runs, for every command but `up` (which uses `listen`); default `http://` + `listen`. `--url` wins. |
| `RHYTHM_ECHO_TOKEN` | The token `up` requires and other commands send. `--token` wins. Never written to reports. |
| `ECHO_STRIPE_WEBHOOK_SECRET`, `ECHO_WEBHOOK_SECRET`, `ECHO_GITHUB_WEBHOOK_SECRET`, `ECHO_WC_WEBHOOK_SECRET` | Default signing secrets for [replay](Echo-Replay#secrets), per scheme. |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day "new release available" check. |
| `CI` | When set, as CI systems do, the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks, to avoid GitHub's anonymous rate limit. |

Read by the connectors and `rhythm-echo sendmail`, in the site's environment:

| Variable | Read by | Description |
|---|---|---|
| `RHYTHM_ECHO_URL` | all connectors, `sendmail` | Echo's capture API as the site reaches it. |
| `RHYTHM_ECHO_TOKEN` | all connectors, `sendmail` | Sent as `X-Echo-Token`. |
| `RHYTHM_ECHO_RUN` | all connectors | `<run>/<action>` for work outside a request. Echo sets it on the commands it runs. |
| `RHYTHM_ECHO_FORCE` | all connectors, `sendmail` | Run even where the environment says production. |
| `RHYTHM_ECHO_MODE` | WordPress, Laravel | What to do when Echo can't be reached: `seal` (default) or `observe`. |
| `RHYTHM_ECHO_PROXY`, `RHYTHM_ECHO_CA` | WordPress | Echo's proxy and CA for raw cURL. |
| `RHYTHM_ECHO_ENABLED`, `RHYTHM_ECHO_TIMEOUT`, `RHYTHM_ECHO_GUZZLE` | Laravel | See [Laravel](Echo-Install-Connectors#laravel). |
| `RHYTHM_ECHO_SITE` | Node | Extra hosts that count as the site. |
| `RHYTHM_ECHO_SMTP` | `sendmail` | The SMTP sink, default `127.0.0.1:1025`. |
| `WP_ENVIRONMENT_TYPE`, `APP_ENV`, `VERCEL_ENV`, `NODE_ENV` | connectors, `sendmail` | The production guard. |

The installer variables (`RHYTHM_INSTALL_DIR`, `RHYTHM_VERSION`, `RHYTHM_TOOLS`) are in [Install](Install#installer-options).

## The .echo folder

```text
.echo/
├── config.yaml                 # your settings: commit it
├── snapshots/
│   ├── before-update/          # run.json, report.html
│   ├── .incoming-<name>/       # a snapshot being written; moved into place when complete
│   └── LATEST                  # the name compare uses by default
└── runs/
    └── 2026-10-24T08-17-42/    # run.json, comparison.json, report.html
```

| What | Where |
|---|---|
| Settings | `.echo/config.yaml` |
| Snapshots | `.echo/snapshots/<name>/`, never pruned |
| Runs and comparisons | `.echo/runs/<date-time>/`, the 20 newest |
| The latest snapshot's name | `.echo/snapshots/LATEST`; when it's missing or names a deleted snapshot, the newest snapshot is used and `LATEST` is rewritten |

Runs are stored as JSON with the schema `rhythm-echo/v1`. Inside a git repository, the first command that writes there adds this block to `.git/info/exclude`, so captured messages are never committed by accident:

```text
# Echo snapshots and reports
/.echo/snapshots/
/.echo/runs/
/.echo/.incoming/
```

(`/.echo/.incoming/` is where earlier builds wrote snapshots in progress; an empty one left behind is removed.)

The folder is always `./.echo`, even with `--config` pointing elsewhere.

### Per user

| What | Linux | macOS | Windows |
|---|---|---|---|
| The proxy's CA (`ca.pem`, `ca-key.pem`) | `~/.cache/rhythm/echo/ca` (or `$XDG_CACHE_HOME/rhythm/echo/ca`) | `~/Library/Caches/rhythm/echo/ca` | `%LocalAppData%\rhythm\echo\ca` |
| Release check state | `~/.config/rhythm/update-check/rhythm-echo.json` | `~/Library/Application Support/rhythm/update-check/…` | `%AppData%\rhythm\update-check\…` |
| First-run notice marker | `~/.config/rhythm/seen/echo-authorised-use` | `~/Library/Application Support/rhythm/seen/…` | `%AppData%\rhythm\seen\…` |

The CA is made the first time something needs it (`up --proxy … --mitm` or `ca`; `install` only prints where it will be) and is valid for ten years; Echo makes a new one when it has less than 30 days left. Its key is readable only by you. Deleting the folder makes a new CA, which clients then have to trust again.

---
<sub>← [CI](Echo-CI) · [Home](Home) · [API](Echo-API) →</sub>
