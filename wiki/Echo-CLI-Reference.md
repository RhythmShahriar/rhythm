Every Echo command and flag, its exit codes, the environment variables it reads, and where it keeps files. Verified against rhythm-echo 1.0.0.

Echo is the `rhythm-echo` binary. `rhythm echo …` runs it, and so does `rhythm-echo …` directly; this page uses the `rhythm echo` form.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm echo up [flags]` | `start` | Start the catcher: the API and live view, the SMTP sink and, with `--proxy`, the HTTP(S) proxy. Runs until Ctrl+C. |
| `rhythm echo install <stack> [path] [flags]` | | Connect a site: `wordpress` (`wp`), `laravel`, `next` (`nextjs`), `node`, `generic` (`php`). |
| `rhythm echo doctor` | | Check every capture layer with canary messages. |
| `rhythm echo watch` | `tail` | Print messages as they arrive. |
| `rhythm echo ls` | `list` | List captured messages. |
| `rhythm echo show <id>` | | Print one message as JSON. |
| `rhythm echo clear` | | Delete captured messages. |
| `rhythm echo snapshot [--name N]` | `snap` | Run the actions in `.echo/config.yaml` and save what they sent. |
| `rhythm echo compare [--snapshot N]` | `cmp` | Run the actions again and show what changed. |
| `rhythm echo run` | | Run the actions once, with no comparison. |
| `rhythm echo report [run \| snapshot]` | | Show a stored result again: the latest run, comparison or snapshot, a run folder, or a snapshot by name. |
| `rhythm echo snapshots [delete <name>]` | | List snapshots. `snapshots delete <name>` (or `rm`) deletes one. |
| `rhythm echo replay <event \| file.json> --to <url>` | | Send a signed test webhook to the site. |
| `rhythm echo check <domain>` | | DNS deliverability checks for the domain you send from. |
| `rhythm echo headers <file.eml \| ->` | | Check a received email's authentication and route. |
| `rhythm echo init [--force]` | | Write `.echo/config.yaml` with every setting. |
| `rhythm echo ca [--pem]` | | Print the proxy CA certificate's path (creating it if needed), or with `--pem` the certificate. |
| `rhythm echo sendmail [-t] [-i] [-f from] [rcpt…]` | | A `sendmail` stand-in for `sendmail_path`; reads the message on stdin and hands it to Echo. |
| `rhythm echo update` | | Nothing to refresh; exits `0`, so `rhythm update` can call it. |
| `rhythm echo version` | `-v`, `--version` | Print the version: `rhythm-echo 1.0.0`. |
| `rhythm echo help` | `-h`, `--help` | Print the help text. |

Running `rhythm echo` with no command on a terminal asks what to do: a menu offers the likeliest next step first (start Echo, or watch when it's already running; compare with the latest snapshot; take a snapshot; run the actions; show the last result; connect a site; doctor; `init` when there's no config), asks the follow-up questions (which stack and where the site is for `install`, whether to open the HTML report), then runs that command. Piped or in CI it prints the help instead. `install`, `init`, `ca` and `snapshots` print their own usage with `-h`, and `sendmail` takes only sendmail's flags; the other commands print the full help. Flags may come before or after arguments: `rhythm echo check shop.com --json` and `rhythm echo check --json shop.com` are the same. An unknown command prints `rhythm echo: unknown command "…"` and the help, and exits `2`.

### up

```sh
rhythm echo up                                   # sealed, API :8025, SMTP :1025
rhythm echo up --observe                         # record and send for real
rhythm echo up --proxy 127.0.0.1:8026 --mitm     # also the HTTPS proxy, decrypting
rhythm echo up --listen 172.17.0.1:8025 --token "$RHYTHM_ECHO_TOKEN"   # reachable from Docker
```

```text
  ♪ rhythm echo 1.0.0                                             this machine
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          seal: nothing leaves; Echo answers for every service
  ▸ API           http://127.0.0.1:8025  (connectors post here; open it for the live view)
  ▸ SMTP          127.0.0.1:1025  (point a mailer or sendmail_path here)

  Waiting for messages. Ctrl+C to stop.
```

With `--proxy`, a `Proxy` line follows (`127.0.0.1:8026  (HTTPS_PROXY for SDKs that skip the connector)`), and with an allowlist an `Allowed` line. Each message is printed once, as it arrives (`08:16:33 Twilio SMS to +15551230000: Your code is 4821 [faked]` in plain text). When its status changes later (an observed request gets its answer, a check blocks it), an update line follows: `Update: 08:16:34 HTTPS connection to api.example.com is now blocked (was faked)`. The proxy's own warnings start with `proxy:`, such as `proxy: refused a tunnel host=api.stripe.com reason=payment APIs need --mitm so Echo can tell test from live keys`. On Ctrl+C: `Stopped. Captured 1 email, 3 chat messages this session.` Messages live in memory only (the newest 10,000) and are gone when Echo stops.

An address beyond loopback for `--listen`, `--smtp` or `--proxy` needs a token, and `up` refuses before it listens on anything (exit `2`):

```text
rhythm echo: --listen 0.0.0.0:8025 is reachable from other machines; set --token (or RHYTHM_ECHO_TOKEN) so only your connectors can post
rhythm echo: --smtp 0.0.0.0:1025 is reachable from other machines; set --token (or RHYTHM_ECHO_TOKEN): the sink then asks for it as the SMTP password
rhythm echo: --proxy 0.0.0.0:8026 is reachable from other machines; set --token (or RHYTHM_ECHO_TOKEN) so it isn't an open relay
```

Beyond loopback, the SMTP line reads `0.0.0.0:1025  (log in with any username and the token as the password)`: the sink then asks for SMTP AUTH with the token as the password. On loopback any login, or none, is accepted. A port in use: `listening on 127.0.0.1:8025: … address already in use (is Echo already running? see: rhythm echo doctor)`.

### install

```sh
rhythm echo install wordpress /var/www/staging [--url URL] [--token T] [--proxy URL] [--ca PATH] [--mode seal|observe] [--force] [--config FILE]
rhythm echo install wordpress /var/www/staging --wp-cli
rhythm echo install wordpress --container shop-cli
rhythm echo install wordpress /var/www/staging --uninstall
rhythm echo install laravel | next [dir] | node | generic   [--url URL] [--token T] [--config FILE]
```

`wordpress` writes the mu-plugin; the other stacks print what to add (`next` also writes `instrumentation.ts` when there's none). `--url` defaults to `RHYTHM_ECHO_URL`, then the `listen` address in `.echo/config.yaml` (`http://127.0.0.1:8025` without one); `--token` to `RHYTHM_ECHO_TOKEN`. The SMTP and proxy addresses the texts print come from the config too. `rhythm echo install -h` prints:

```text
usage: rhythm echo install <stack> [path] [flags]
  stacks: wordpress (wp), laravel, next (nextjs), node, generic (php)
  wordpress: [path] --url URL --token T --mode seal|observe --force --proxy URL --ca PATH
             --wp-cli | --container NAME, --uninstall
  all: --url URL --token T --config FILE
```

An unknown stack, a bad `--mode` (`rhythm echo: --mode "bogus": use seal or observe`) and a folder that doesn't look like WordPress are usage errors (exit `2`). Every flag is explained in [Connectors](Echo-Install-Connectors#flags).

### doctor

```text
  ♪ rhythm echo 1.0.0                                                   doctor
  ────────────────────────────────────────────────────────────────────────────

  ✓ Echo          seal mode at http://127.0.0.1:8025, 34 providers recognized
  ✓ SMTP          127.0.0.1:1025 receives mail
  ✓ WordPress     active (staging), layers: wp_mail, pre_http_request
  ✓ Canaries      wp_mail and wp_remote_post were both caught

  4 checks passed, 0 failed.
```

With `--plain` (or piped), each check is a line that starts with a word:

```text
rhythm echo 1.0.0 - doctor

ok    Echo: seal mode at http://127.0.0.1:8025, 34 providers recognized
ok    SMTP: 127.0.0.1:1025 receives mail
note  WordPress: no connector answered at https://staging.shop.test (unexpected answer from the ping route); install it: rhythm echo install wordpress

  2 checks passed, 0 failed.
```

When Echo isn't running: `FAIL  Echo: no Echo is running at http://127.0.0.1:8025 (start it with: rhythm echo up)`. With `-q`, only failed checks are printed.

In order: Echo answers; the SMTP sink receives a canary (unless `smtp` is `off`; beyond loopback, `doctor` logs in with the token); the proxy is listening (when `proxy` is set); the WordPress connector at `site:` is active, can reach Echo, and catches a `wp_mail()` and a `wp_remote_post()` canary; the Laravel connector, when run in a Laravel project (where `artisan` is), catches `php artisan echo:canary`; each `flush` command runs. A `·` (or `note`) line is a note, not a failure: no `site:`, or no connector answered there. Canaries go to `.invalid` addresses and are deleted afterwards. `doctor` exits `1` when any check failed.

### watch, ls, show, clear

```sh
rhythm echo watch [--channel C] [--to T] [--run R]
rhythm echo ls    [--channel C] [--to T] [--run R] [--json]
rhythm echo show <id>
rhythm echo clear [--channel C] [--to T] [--run R]
```

`--channel` is one of `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push`, `http`; `--to` matches part of a recipient, ignoring case; `--run` a run ID. `watch` prints `Watching http://127.0.0.1:8025 (seal mode). Ctrl+C to stop.` first; `ls` prints `No messages captured yet.` when there are none; `clear` prints `Deleted 5 messages`. An unknown channel is a usage error: `rhythm echo: --channel "bogus": use one of email, sms, whatsapp, webhook, chat, analytics, payment, push, http`. `show` with an ID Echo doesn't have exits `2` with `no message "x" in Echo; list them with: rhythm echo ls`. See [Messages and checks](Echo-Messages-And-Checks#listing-and-inspecting).

### snapshot, compare, run

```sh
rhythm echo snapshot --name before-update
rhythm echo compare                          # the latest snapshot
rhythm echo compare --snapshot before-update --fail-on broke
rhythm echo run --action "Contact form" --fail-on error
```

All three run the actions in the config, need Echo running, and write a report into `.echo/`. None takes arguments: `rhythm echo: snapshot takes no arguments; actions come from .echo/config.yaml`, `rhythm echo: run takes no arguments; actions come from .echo/config.yaml (pick some with --action)`. These usage errors exit `2` before anything is printed:

```text
rhythm echo: no actions in .echo/config.yaml; add some (rhythm echo init writes examples), or use Soundcheck with --echo
rhythm echo: no action named "nope"; the actions are: Contact form, Order shipped, Weekly digest
rhythm echo: no snapshot yet; take one first: rhythm echo snapshot
```
 See [Actions and compare](Echo-Actions-And-Compare).

### report and snapshots

```sh
rhythm echo report                                # the latest run or comparison
rhythm echo report before-update                  # a snapshot
rhythm echo report .echo/runs/2026-10-24T08-17-42
rhythm echo report --md -o echo.md
```

`report` renders a stored result without running anything, in any format, with `--open` and `--fail-on`. Without an argument it shows the newest run or comparison, or the latest snapshot when nothing ran after it. A folder argument wins over a snapshot of the same name. Before anything was stored: `nothing to show yet; run rhythm echo snapshot, compare or run first`; an unknown name: `no snapshot named "nope"; see rhythm echo snapshots` (both exit `2`).

```text
$ rhythm echo snapshots
NAME                     ACTIONS  TAKEN        SENT
before-update            3        1m ago       3 emails, 1 SMS, 1 chat message, 1 webhook  (latest)
```

With none: `No snapshots yet. Take one: rhythm echo snapshot`. Deleting the latest one makes the newest remaining snapshot the latest:

```text
$ rhythm echo snapshots delete after-update
Deleted snapshot after-update
The latest snapshot is now before-update
```

### replay

```sh
rhythm echo replay --list
rhythm echo replay payment_intent.succeeded --to /?wc-api=wc_stripe --set data.object.amount=2400
rhythm echo replay lead.json --scheme hmac --type lead.created --to http://127.0.0.1:8080/hooks/lead -H "X-Source: crm"
```

See [Replay](Echo-Replay).

### check and headers

```sh
rhythm echo check shop.com --provider sendgrid --selector s1
rhythm echo check shop.com --from-mail received.eml --mta-sts-fetch --json
rhythm echo headers received.eml --hops --check
cat received.eml | rhythm echo headers -
```

See [Deliverability](Echo-Deliverability).

### ca and sendmail

```sh
rhythm echo ca           # /home/you/.cache/rhythm/echo/ca/ca.pem (created if needed)
rhythm echo ca --pem     # -----BEGIN CERTIFICATE----- …
printf 'To: jamie@shop.test\nSubject: Booking confirmed\n\nSee you at 7.\n' | rhythm-echo sendmail -t -i
```

`sendmail` takes sendmail's own flags, not Echo's (so `-h` is `rhythm-echo sendmail: unknown option -h`); see [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in).

## Flags

### Catcher (`up`)

| Flag | Default | Description |
|---|---|---|
| `--seal` | on | Nothing leaves; Echo answers for every service. |
| `--observe` | | Record everything and send it for real. Can't be combined with `--seal`. |
| `--listen <addr>` | `127.0.0.1:8025` | The API and live view. |
| `--smtp <addr>` | `127.0.0.1:1025` | The SMTP sink; `off` disables it. |
| `--proxy <addr>` | off | Also run the HTTP(S) proxy, e.g. `127.0.0.1:8026`. |
| `--mitm` | off | Decrypt HTTPS in the proxy with Echo's CA. |
| `--allow <hosts>` | | Hosts reachable for real even when sealed; comma-separated, repeatable, added to `allow`. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | Require this token from connectors and clients. Needed when the API, SMTP sink or proxy listens beyond loopback; the SMTP sink then asks for it as the password. Other commands send it. |

### Install

| Flag | Description |
|---|---|
| `--url <url>` | Echo's API as the site reaches it. Default: `RHYTHM_ECHO_URL`, else `listen` from `.echo/config.yaml`. |
| `--token <t>` | The token Echo was started with. Default: `RHYTHM_ECHO_TOKEN`. |
| `--mode <m>` | (`wordpress`) `seal` or `observe`: what to do when Echo is down. |
| `--force` | (`wordpress`) Run even where WordPress says production. |
| `--proxy <url>` | (`wordpress`) Send raw cURL through Echo's proxy. |
| `--ca <path>` | (`wordpress`) Echo's CA certificate, as the site sees it. |
| `--wp-cli` | (`wordpress`) Install through WP-CLI (`wp eval-file`). |
| `--container <name>` | (`wordpress`) Install with WP-CLI inside this Docker container. |
| `--uninstall` | (`wordpress`) Remove the connector. |
| `--config <file>` | Where to read the addresses from. Default `.echo/config.yaml`. |

### Output

| Flag | Description |
|---|---|
| `--json`, `--md`, `--junit`, `--html` | The report format; pick one. Alone, it goes to stdout instead of the terminal view. |
| `-o, --output <file>` | Write the report to a file; the format comes from the extension (`.json`, `.md`, `.xml`, `.html`) unless a format flag is given. The terminal view still prints. |
| `--open` | Open the HTML report when done. |
| `-q, --quiet` | Results only: no header, progress or notes. |
| `--plain` | No color, boxes or glyphs. On automatically when stdout isn't a terminal. |
| `--no-color` | Keep the layout, drop the color. |
| `-v, --verbose` | Show every message. |
| `--fail-on <level>` | Exit `1` on changes at or above `broke`, `new` or `changed` (`compare`, `report` of a comparison), or on issues at or above `error`, `warning` or `notice` (`run`, `snapshot`, `check`, `headers`, `report` of a run). |

`rhythm echo: pick one output format, not --json and --md`; `rhythm echo: can't tell the format of out.txt; add --json, --md, --junit or --html`.

### Actions and messages

| Flag | Description |
|---|---|
| `--name <name>` | Snapshot name: letters, digits, dots, dashes and underscores, up to 64. Default: the date and time. |
| `--snapshot <name>` | The snapshot `compare` uses. Default: the latest. |
| `--action <name>` | Run only this action, ignoring case; repeatable. |
| `--channel <c>` | Only `email`, `sms`, `whatsapp`, `webhook`, `chat`, `analytics`, `payment`, `push` or `http`; for `watch`, `ls` and `clear`. |
| `--to <address>` | Only messages to this recipient (part of it, ignoring case). |
| `--run <id>` | Only messages from this run. |

### Replay

| Flag | Default | Description |
|---|---|---|
| `--to <url>` | (required) | Where to send: absolute, or relative to `site:`. |
| `--set <path=value>` | | Override a field; repeatable. |
| `--secret-env <NAME>` | per scheme, e.g. `ECHO_STRIPE_WEBHOOK_SECRET` | The variable with the signing secret. |
| `--scheme <s>` | `hmac` | For JSON files: `stripe`, `standard`, `github`, `woocommerce`, `hmac`. |
| `--type <t>` | the payload's `type` | For JSON files: the event type or topic. |
| `-H, --header <"Name: value">` | | Extra header; repeatable; can't replace signature headers. |
| `--list` | | List the built-in events. |
| `--wait <d>` | `5s` | How long to collect what the site sends back. |

### Deliverability

| Flag | Description |
|---|---|
| `--from-mail <file.eml>` | (`check`) Learn the real sender from a received email. |
| `--provider <name>` | (`check`) Name the sending service: `sendgrid`, `mailgun`, `postmark`, `ses`, `resend`, `brevo`, `mailchimp`, `mandrill`, `sparkpost`, `google`, `microsoft`, `php-mail`. Any other name is a usage error that lists these. |
| `--selector <s>` | (`check`) The DKIM selector. |
| `--return-path <domain>` | (`check`) The envelope sender's domain. |
| `--smtp-host <host>` | (`check`) The relay that hands mail out. |
| `--resolver <host:port>` | (`check`, `headers --check`) The DNS server to ask. Default: the system's. |
| `--mta-sts-fetch` | (`check`, `headers --check`) Also fetch the MTA-STS policy file over HTTPS. |
| `--check` | (`headers`) Also run the DNS checks for the sender. |
| `--hops` | (`headers`) Show the route the message took. |

### Other

| Flag | Default | Description |
|---|---|---|
| `--config <file>` | `.echo/config.yaml` | Settings file. A missing file named with `--config` is an error. |
| `--url <url>` | `RHYTHM_ECHO_URL`, else `http://` + `listen` | Where Echo runs, for every command but `up`. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | The token to send. |

## Exit codes

The same codes as Tuner and Soundcheck. The help lists them as:

```text
  0    ok
  1    findings: --fail-on hit, a doctor check failed, or the site rejected a replay
  2    usage error: a wrong command line, or .echo/config.yaml asks for what isn't there
  3    couldn't run: Echo not reachable, a file or network error
  130  interrupted
  sendmail exits with sendmail's codes instead (sysexits: 64 usage, 69 Echo unreachable…)
```

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or no `--fail-on`) |
| `1` | A change or issue at or above `--fail-on`; a failed `doctor` check; a `replay` the site didn't answer with `2xx` |
| `2` | Usage error, always before any output: a bad flag or value, an unknown command, stack, action (`--action`), channel, provider, snapshot or message ID, no actions in the config, no snapshot yet, a folder that isn't WordPress, a missing signing secret, a refused replay |
| `3` | Couldn't run: Echo isn't running (`no Echo is running at http://127.0.0.1:8025 (start it with: rhythm echo up)`), Echo answered an error (`Echo answered 404 Not Found to GET /v1/…`), an action couldn't start, a file or network error |
| `130` | Interrupted; results are partial |

`rhythm echo sendmail` uses sendmail's codes instead; see [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in).

## Environment variables

| Variable | Description |
|---|---|
| `RHYTHM_ECHO_URL` | Where Echo runs; `--url` wins. |
| `RHYTHM_ECHO_TOKEN` | The token `up` requires and other commands send; `--token` wins. |
| `ECHO_STRIPE_WEBHOOK_SECRET`, `ECHO_WEBHOOK_SECRET`, `ECHO_GITHUB_WEBHOOK_SECRET`, `ECHO_WC_WEBHOOK_SECRET` | Replay signing secrets. |
| `RHYTHM_ECHO_SMTP`, `RHYTHM_ECHO_FORCE` | For `sendmail`. |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day release check. |
| `CI` | When set, the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks. |

The connectors' variables are in [Configuration](Echo-Configuration#environment-variables).

## Files and locations

| What | Where |
|---|---|
| Settings | `.echo/config.yaml` |
| Snapshots | `.echo/snapshots/<name>/` and `.echo/snapshots/LATEST` |
| Runs and comparisons (the 20 newest) | `.echo/runs/<date-time>/` |
| The proxy's CA | `~/.cache/rhythm/echo/ca` (Linux), `~/Library/Caches/rhythm/echo/ca` (macOS), `%LocalAppData%\rhythm\echo\ca` (Windows) |
| Release check state | `~/.config/rhythm/update-check/rhythm-echo.json` |
| First-run notice marker | `~/.config/rhythm/seen/echo-authorised-use` |

See [The .echo folder](Echo-Configuration#the-echo-folder). The responsible-use notice is shown once per machine; deleting its marker brings it back.

---
<sub>← [API](Echo-API) · [Home](Home) · [FAQ and troubleshooting](Echo-FAQ) →</sub>
