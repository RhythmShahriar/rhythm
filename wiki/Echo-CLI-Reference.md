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
| `rhythm echo report [run]` | | Show a stored result again: the latest run or comparison, a run folder, or a snapshot by name. |
| `rhythm echo snapshots` | | List snapshots. `snapshots delete <name>` (or `rm`) deletes one. |
| `rhythm echo replay <event \| file.json> --to <url>` | | Send a signed test webhook to the site. |
| `rhythm echo check <domain>` | | DNS deliverability checks for the domain you send from. |
| `rhythm echo headers <file.eml \| ->` | | Check a received email's authentication and route. |
| `rhythm echo init [--force]` | | Write `.echo/config.yaml` with every setting. |
| `rhythm echo ca [--pem]` | | Print the proxy CA certificate's path (creating it if needed), or with `--pem` the certificate. |
| `rhythm echo sendmail [sendmail flags]` | | A `sendmail` stand-in that hands mail to Echo. |
| `rhythm echo update` | | Nothing to refresh; exits `0`, so `rhythm update` can call it. |
| `rhythm echo version` | `-v`, `--version` | Print the version: `rhythm-echo 1.0.0`. |
| `rhythm echo help` | `-h`, `--help` | Print the help text. |

Running `rhythm echo` with no command prints the help. Flags may come before or after arguments: `rhythm echo check shop.com --json` and `rhythm echo check --json shop.com` are the same. An unknown command prints `rhythm echo: unknown command "…"` and the help, and exits `2`.

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

Each message is printed as it arrives (`08:16:33 Twilio SMS to +15551230000: Your code is 4821 [faked]`), and again when an observed request's outcome comes in. On Ctrl+C: `Stopped. Captured 1 email, 3 chat messages this session.` Messages live in memory only (the newest 10,000) and are gone when Echo stops.

An address beyond loopback for `--listen` or `--proxy` needs a token: `rhythm echo: --listen 0.0.0.0:8025 is reachable from other machines; set --token (or RHYTHM_ECHO_TOKEN) so only your connectors can post`. A port in use: `listening on 127.0.0.1:8025: … address already in use (is Echo already running? see: rhythm echo doctor)`.

### install

```sh
rhythm echo install wordpress /var/www/staging [--url URL] [--token T] [--proxy URL] [--ca PATH] [--mode seal|observe] [--force]
rhythm echo install wordpress /var/www/staging --wp-cli
rhythm echo install wordpress --container shop-cli
rhythm echo install wordpress /var/www/staging --uninstall
rhythm echo install laravel | next [dir] | node | generic   [--url URL] [--token T]
```

`wordpress` writes the mu-plugin; the other stacks print what to add (`next` also writes `instrumentation.ts` when there's none). `--url` defaults to `RHYTHM_ECHO_URL`, then `http://127.0.0.1:8025`; `--token` to `RHYTHM_ECHO_TOKEN`. Every flag is explained in [Connectors](Echo-Install-Connectors#flags).

### doctor

```text
  ♪ rhythm echo 1.0.0                                                   doctor
  ────────────────────────────────────────────────────────────────────────────

  ▸ ✓ Echo        seal mode at http://127.0.0.1:8025, 35 providers recognized
  ▸ ✓ SMTP        127.0.0.1:1025 receives mail
  ▸ ✓ WordPress   active (staging), layers: wp_mail, pre_http_request
  ▸ ✓ Canaries    wp_mail and wp_remote_post were both caught

  4 checks passed, 0 failed.
```

In order: Echo answers; the SMTP sink receives a canary (unless `smtp` is `off`); the proxy is listening (when `proxy` is set); the WordPress connector at `site:` is active, can reach Echo, and catches a `wp_mail()` and a `wp_remote_post()` canary; the Laravel connector, when run in a Laravel project (where `artisan` is), catches `php artisan echo:canary`; each `flush` command runs. A `·` line is a note, not a failure: no `site:`, or no connector answered there. Canaries go to `.invalid` addresses and are deleted afterwards. `doctor` exits `1` when any check failed.

### watch, ls, show, clear

```sh
rhythm echo watch [--channel C] [--to T] [--run R]
rhythm echo ls    [--channel C] [--to T] [--run R] [--json]
rhythm echo show <id>
rhythm echo clear [--channel C] [--to T] [--run R]
```

`--channel` is one of `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push`, `http`; `--to` matches part of a recipient, ignoring case; `--run` a run ID. `watch` prints `Watching http://127.0.0.1:8025 (seal mode). Ctrl+C to stop.` first; `ls` prints `No messages captured yet.` when there are none; `clear` prints `Deleted 5 messages`. See [Messages and checks](Echo-Messages-And-Checks#listing-and-inspecting).

### snapshot, compare, run

```sh
rhythm echo snapshot --name before-update
rhythm echo compare                          # the latest snapshot
rhythm echo compare --snapshot before-update --fail-on broke
rhythm echo run --action "Contact form" --fail-on error
```

All three run the actions in the config, need Echo running, and write a report into `.echo/`. `snapshot` and `compare` take no arguments: `rhythm echo: snapshot takes no arguments; actions come from .echo/config.yaml`. Without actions: `no actions in .echo/config.yaml; add some (rhythm echo init writes examples), or use Soundcheck with --echo`. Without a snapshot: `no snapshot yet; take one first: rhythm echo snapshot` (exit `2`). See [Actions and compare](Echo-Actions-And-Compare).

### report and snapshots

```sh
rhythm echo report                                # the latest run or comparison
rhythm echo report before-update                  # a snapshot
rhythm echo report .echo/runs/2026-10-24T08-17-42
rhythm echo report --md -o echo.md
```

`report` renders a stored result without running anything, in any format, with `--open` and `--fail-on`. A folder argument wins over a snapshot of the same name. Before any run or comparison: `nothing to show yet; run rhythm echo compare or run first` (a snapshot alone isn't a run: name it).

```text
$ rhythm echo snapshots
NAME                     ACTIONS  TAKEN        SENT
before-update            3        1m ago       3 emails, 1 SMS, 1 chat message, 1 webhook  (latest)
```

With none: `No snapshots yet. Take one: rhythm echo snapshot`.

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
rhythm echo ca           # /home/you/.cache/rhythm/echo/ca/ca.pem
rhythm echo ca --pem     # -----BEGIN CERTIFICATE----- …
printf 'To: jamie@shop.test\nSubject: Booking confirmed\n\nSee you at 7.\n' | rhythm-echo sendmail -t -i
```

`sendmail` takes sendmail's own flags, not Echo's; see [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in).

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
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | Require this token from connectors and clients. Needed beyond loopback. Other commands send it. |

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
| `--channel <c>`, `--to <t>`, `--run <id>` | Filters for `watch`, `ls` and `clear`. |

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
| `--provider <name>` | (`check`) Name the sending service: `sendgrid`, `mailgun`, `postmark`, `ses`, `resend`, `brevo`, `mailchimp`, `mandrill`, `sparkpost`, `google`, `microsoft`, `php-mail`. |
| `--selector <s>` | (`check`) The DKIM selector. |
| `--return-path <domain>` | (`check`) The envelope sender's domain. |
| `--smtp-host <host>` | (`check`) The relay that hands mail out. |
| `--resolver <host:port>` | (`check`) The DNS server to ask. |
| `--mta-sts-fetch` | (`check`) Also fetch the MTA-STS policy file over HTTPS. |
| `--check` | (`headers`) Also run the DNS checks for the sender. |
| `--hops` | (`headers`) Show the route the message took. |

### Other

| Flag | Default | Description |
|---|---|---|
| `--config <file>` | `.echo/config.yaml` | Settings file. A missing file named with `--config` is an error. |
| `--url <url>` | `RHYTHM_ECHO_URL`, else `http://` + `listen` | Where Echo runs, for every command but `up`. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | The token to send. |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` (or no `--fail-on`) |
| `1` | A change or issue at or above `--fail-on`; a failed `doctor` check; a `replay` the site didn't answer with `2xx` |
| `2` | Usage error: a bad flag, value or config, no snapshot, a refused replay |
| `3` | Couldn't run: Echo isn't running, an action couldn't start, a file couldn't be read or written |
| `130` | Interrupted; results are partial |

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
