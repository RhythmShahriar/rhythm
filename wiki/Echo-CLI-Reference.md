Every Echo command and flag, its exit codes, the JSON every command writes, the environment variables it reads and where it keeps files. Verified against rhythm-echo 1.1.1.

Echo is the `rhythm-echo` binary. `rhythm echo …` runs it, as does `rhythm-echo …` directly: they're the same command. This page uses the `rhythm echo` form.

## Commands

| Command | Alias | What it does |
|---|---|---|
| `rhythm echo up [flags]` | `start` | Start the catcher: the API and Echo Inbox, the SMTP sink and, with `--proxy`, the HTTP(S) proxy. Runs until Ctrl+C. |
| `rhythm echo install <stack> [path] [flags]` | | Connect a site: `wordpress` (`wp`), `laravel`, `next` (`nextjs`), `node`, `generic` (`php`). |
| `rhythm echo doctor` | | Check every capture layer with canary messages. |
| `rhythm echo watch` | `tail` | Print messages as they arrive; `--json` streams one object per line. |
| `rhythm echo ls` | `list` | List captured messages. |
| `rhythm echo show <id>` | | Print one message as JSON. |
| `rhythm echo clear` | | Delete captured messages. |
| `rhythm echo snapshot [--name N]` | `snap` | Run the actions in `.echo/config.yaml` and save what they sent. |
| `rhythm echo compare [--snapshot N]` | `cmp` | Run the actions again and show what changed. |
| `rhythm echo run` | | Run the actions once, with no comparison. |
| `rhythm echo report [run \| snapshot]` | | Show a stored result again: the newest run, comparison or snapshot, a run folder, or a snapshot by name. |
| `rhythm echo report --site <id> --store <dir> [--pdf]` | | A hub site's agency report. See [Agency hub and reports](Echo-Hub-And-Reports#the-agency-report). |
| `rhythm echo snapshots [delete <name>]` | | List snapshots. `snapshots delete <name>` (or `rm`) deletes one. |
| `rhythm echo replay <event \| file.json> --to <url>` | | Send a signed test webhook to the site. |
| `rhythm echo latest --to <address> [--code \| --link]` | | Print the one-time code or link just sent to someone. |
| `rhythm echo release <id> --to <address>` | | Send one caught email to a client through the relay. |
| `rhythm echo check <domain>` | | DNS deliverability checks for the domain you send from. |
| `rhythm echo headers <file.eml \| ->` | | Check a received email's authentication and route. |
| `rhythm echo init [--stack s] [--force]` | | Write `.echo/config.yaml` with an action for this stack. |
| `rhythm echo providers` | | List the services Echo recognizes, by channel. |
| `rhythm echo checks [--test]` | | List the checks and the config's rules; `--test` tries the rules' fixtures. |
| `rhythm echo ca [--pem \| --untrust]` | | Print the proxy CA certificate's path (creating it if needed), the certificate, or how to stop trusting it. |
| `rhythm echo mcp` | | The [MCP server](Echo-MCP) for coding agents, on stdin and stdout. |
| `rhythm echo hub --store D --sites F` | | One Echo for many staging sites. See [Agency hub and reports](Echo-Hub-And-Reports). |
| `rhythm echo schedule "<when>" --store D --after "<command>"` | | Snapshot, update and compare the hub's sites in a window. See [Schedule](Echo-Schedule). |
| `rhythm echo conformance [--listen a]` | | A strict stand-in Echo for a connector's end-to-end test. See [Conformance kit](Echo-Conformance). |
| `rhythm echo sendmail [-t] [-i] [-f from] [rcpt…]` | | A `sendmail` stand-in for `sendmail_path`; reads the message on stdin and hands it to Echo. |
| `rhythm echo update` | | Nothing to refresh; exits `0`, so `rhythm update` can call it. |
| `rhythm echo version` | `-v`, `--version` | Print the version: `rhythm-echo 1.1.1`. |
| `rhythm echo help [command]` | `-h`, `--help` | Print the help text, or one command's. |

Running `rhythm echo` with no command on a terminal asks what to do: a menu offers the likeliest next step first (start Echo, or watch when it's already running; compare with the latest snapshot; take a snapshot; run the actions; show the last result; connect a site; doctor; `init` when there's no config), asks the follow-up questions, then runs that command. Piped or in CI it prints the help instead.

`rhythm echo <command> --help` (or `-h`, or `rhythm echo help <command>`, aliases too) prints only that command: its usage, its flags and its exit codes. Each command takes only the flags it reads; any other is refused before anything runs:

```text
rhythm echo: --fail-on has no effect on ls (rhythm echo ls --help lists its flags)
rhythm echo: unknown command "cmopare"; did you mean compare?
rhythm echo help lists every command.
rhythm echo: unknown flag --snapshto; did you mean --snapshot?
```

All of these exit `2`. Flags may come before or after arguments: `rhythm echo check shop.com --json` and `rhythm echo check --json shop.com` are the same. Flags are spelled with two dashes in errors (`rhythm echo: --fail-on needs a value`). Giving `--token` on the command line prints `rhythm echo: --token shows in ps to every user on this machine; set RHYTHM_ECHO_TOKEN instead` first (`-q` hides it).

### up

```sh
rhythm echo up                                   # sealed, API :8025, SMTP :1025
rhythm echo up --observe                         # record and send for real
rhythm echo up --proxy 127.0.0.1:8026 --mitm     # also the HTTPS proxy, decrypting
rhythm echo up --store ~/echo-inbox --keep-for 14d   # keep the inbox across restarts
rhythm echo up --listen 0.0.0.0:8025 --smtp 0.0.0.0:1025 --private-network   # in a container
```

```text
  ♪ rhythm echo 1.1.1                                             this machine
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          seal: nothing leaves; Echo answers for every service
  ▸ API           http://127.0.0.1:8025  (connectors post here; open it for the live view)
  ▸ SMTP          127.0.0.1:1025  (point a mailer or sendmail_path here)

  Waiting for messages. Ctrl+C to stop.
```

Options add lines, here with `--plain`, other ports and `--proxy 127.0.0.1:18026 --mitm --allow api.stripe.com`:

```text
rhythm echo 1.1.1 - this machine

Mode: seal: nothing leaves; Echo answers for every service
API: http://127.0.0.1:18025  (connectors post here; open it for the live view)
SMTP: 127.0.0.1:11025  (point a mailer or sendmail_path here)
Proxy: 127.0.0.1:18026  (HTTPS_PROXY for SDKs that skip the connector); decrypting HTTPS, trust /home/you/.config/rhythm/echo/ca/ca.pem
Allowed: api.stripe.com

  Waiting for messages. Ctrl+C to stop.

Stopped. Captured no messages this session.
```

`--store` adds `Inbox: <dir>  (kept across restarts; …)`, `--login` a `Login:` line, a relay a `Relay:` line, `--sites` a `Sites:` line, `--spamassassin` a `Spam:` line and `--private-network` without a token a `Warning:` line; each is shown on the page for that feature. Each message is printed once, as it arrives, with a line for its problems; a status change later gets an update line (`Update: 08:16:34 HTTPS connection to api.example.com is now blocked (was faked)`). On Ctrl+C: `Stopped. Captured 16 emails, 5 SMS, 5 chat messages, 9 webhooks, 10 analytics events this session.`, counting this session only.

`up` binds every port before it prints anything. An address beyond loopback for `--listen`, `--smtp` or `--proxy` needs a token of 16 characters or more, or `--private-network` on a private network. `up` refuses before it listens on anything (exit `2`):

```text
rhythm echo: --listen 0.0.0.0:18025 is reachable from other machines; set --token (or RHYTHM_ECHO_TOKEN) so only your connectors can post
rhythm echo: --token (or RHYTHM_ECHO_TOKEN): shorter than 16 characters, which can be guessed while 0.0.0.0:18025 is reachable from other machines; make one with: openssl rand -hex 16
rhythm echo: up takes no arguments; rhythm echo up --help lists its flags
```

A port that's taken is named: `rhythm echo: Echo is already running on 127.0.0.1:8025 (use that one, or stop it first)` or `rhythm echo: Mailpit is already on 127.0.0.1:8025; run Echo next to it: rhythm echo up --listen 127.0.0.1:18025 --smtp 127.0.0.1:11025` (exit `3`). On a terminal Echo offers to move; `--free-ports` moves without asking. See [Docker and DDEV](Echo-Docker-And-DDEV#mailpit-on-the-same-machine).

### install

```sh
rhythm echo install wordpress /var/www/staging [--url URL] [--token T] [--proxy URL] [--ca PATH] [--mode seal|observe] [--force]
rhythm echo install wordpress /var/www/staging --plugin [--bin PATH] [--project DIR] [--site ID]
rhythm echo install wordpress /var/www/staging --hub URL --site ID      # write token in RHYTHM_ECHO_WRITE_TOKEN
rhythm echo install wordpress /var/www/staging --wp-cli
rhythm echo install wordpress --container shop-cli
rhythm echo install wordpress /var/www/staging --uninstall
rhythm echo install laravel | next [dir] | node [dir] | generic   [--url URL] [--token T] [--config FILE]
```

`wordpress` writes the mu-plugin, or with `--plugin` Echo Staging Seal; the other stacks print what to add (`next` also writes `instrumentation.ts` when there's none). `--url` defaults to `RHYTHM_ECHO_URL`, then the `listen` address in `.echo/config.yaml` (`http://127.0.0.1:8025` without one); `--token` to `RHYTHM_ECHO_TOKEN`. The SMTP and proxy addresses the texts print come from the config too. `rhythm echo install -h` prints:

```text
usage: rhythm echo install <stack> [path] [flags]
  stacks: wordpress (wp), laravel, next (nextjs), node, generic (php)
  wordpress: [path] --url URL --token T --mode seal|observe --force --proxy URL --ca PATH
             --plugin (Echo Staging Seal) --bin PATH --project DIR
             --hub URL --site ID (the write token from RHYTHM_ECHO_WRITE_TOKEN)
             --wp-cli | --container NAME, --uninstall
  all: --url URL --token T --config FILE
```

An unknown stack (`rhythm echo: install: unknown stack "rails" (wordpress, laravel, next, node, generic)`), a bad `--mode`, a folder that doesn't look like WordPress and a Node folder without `package.json` are usage errors (exit `2`). Every flag is explained in [Connectors](Echo-Install-Connectors#flags).

### doctor

```text
  ♪ rhythm echo 1.1.1                                                   doctor
  ────────────────────────────────────────────────────────────────────────────

  ✓ Echo          seal mode at http://127.0.0.1:8025, 53 providers recognized
  · Payments      live calls refused for Stripe, PayPal, Adyen, Braintree, Square, Authorize.net, Mollie and Razorpay, even in observe mode
  ✓ SMTP          127.0.0.1:1025 receives mail
  ✓ WordPress     active (staging), layers: wp_mail, pre_http_request
  ✓ Canaries      wp_mail and wp_remote_post were both caught

  4 checks passed, 0 failed.
```

With `--plain` (or piped), each check is a line that starts with a word:

```text
rhythm echo 1.1.1 - doctor

ok    Echo: seal mode at http://127.0.0.1:8025, 53 providers recognized
note  Payments: live calls refused for Stripe, PayPal, Adyen, Braintree, Square, Authorize.net, Mollie and Razorpay, even in observe mode
ok    SMTP: 127.0.0.1:1025 receives mail
ok    WordPress: active (staging), layers: wp_mail, pre_http_request
ok    Canaries: wp_mail and wp_remote_post were both caught

  4 checks passed, 0 failed.
```

`doctor` checks the stack it finds in the project folder (the one holding `.echo/`): the WordPress ping and canaries, `php artisan echo:canary` for Laravel, a Node canary (a sealed fetch to Twilio's API through `@rhythmsh/echo/register`) for Next.js and Node; plain PHP has nothing to check. It reads the SMTP and proxy addresses from Echo's health when Echo reports them. It names a service that isn't Echo: `FAIL  Echo: http://127.0.0.1:8025 is Mailpit, not Echo; start Echo on a free port (rhythm echo up --listen 127.0.0.1:18025) and set listen: in .echo/config.yaml to it`. A `site:` under `production:` fails without canaries being sent through it. With `-q`, only failed checks are printed. `--json` writes the envelope with kind `doctor` and `[{name, status, detail}]` in `data` (a note is `warning`, a failure `error`). `doctor` exits `1` when any check failed and `3` when Echo isn't running or another service answers. Every line is explained in [FAQ](Echo-FAQ#doctor-says-a-layer-failed).

### watch, ls, show, clear

```sh
rhythm echo watch [--channel C] [--to T] [--run R] [--since D] [--json]
rhythm echo ls    [--channel C] [--to T] [--run R] [--since D] [--json]
rhythm echo show <id>
rhythm echo clear [--channel C] [--to T] [--run R]
```

`--channel` is one of `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push`, `http`; `--to` matches part of a recipient, ignoring case; `--run` a run ID; `--since` how long ago (`10m`, `30d`). `watch` prints `Watching http://127.0.0.1:8025 (seal mode). Ctrl+C to stop.` first, on stderr; `ls` prints `No messages captured yet.` when there are none; `clear` prints `Deleted 5 messages`. None takes arguments. Errors (exit `2`):

```text
rhythm echo: --channel "bogus": use one of email, sms, whatsapp, webhook, chat, analytics, payment, push, http
rhythm echo: no message "nope" in Echo; list them with: rhythm echo ls
rhythm echo: clear takes no arguments; narrow it with --run, --to or --channel
```

See [Messages and checks](Echo-Messages-And-Checks#listing-and-inspecting).

### snapshot, compare, run

```sh
rhythm echo snapshot --name before-update
rhythm echo compare                          # the latest snapshot
rhythm echo compare --snapshot before-update --fail-on new
rhythm echo compare --against base.tgz       # a snapshot exported elsewhere
rhythm echo compare --accept 562b5cf7        # a change you meant
rhythm echo run --action "Contact form" --fail-on warning
rhythm echo snapshot --export base.tgz | --import base.tgz [--name N]
```

All three run the actions in the config, need Echo running and write a report into `.echo/`. None takes arguments: `rhythm echo: snapshot takes no arguments; actions come from .echo/config.yaml`. These usage errors exit `2` before anything is printed:

```text
rhythm echo: no .echo/config.yaml here or above; run rhythm echo init
rhythm echo: no action named "x"; the actions are: Contact form, Checkout
rhythm echo: no snapshot yet; take one first: rhythm echo snapshot
rhythm echo: pick --snapshot or --against, not both
rhythm echo: --quiet 45s: over the 30s Echo waits for quiet at most
rhythm echo: --parallel -1: give how many actions may run at once, e.g. 4
```

They exit `1` on findings at `--fail-on` (default `broke` for `compare`, `error` otherwise) or when an action failed. They exit `3` when every action failed (`rhythm echo: every action failed, so nothing was checked; is the site up?`). See [Actions and compare](Echo-Actions-And-Compare).

### report and snapshots

```sh
rhythm echo report                                # the newest run, comparison or snapshot
rhythm echo report before-update                  # a snapshot
rhythm echo report .echo/runs/2026-10-25T14-23-14-f6f2df
rhythm echo report --md -o echo.md
```

`report` renders a stored result without running anything, in any format, with `--open` and `--fail-on`. Without an argument it shows the newest of all runs, comparisons and snapshots, by when each started. A folder argument wins over a snapshot of the same name. It exits `1` on a comparison that broke, as `compare` does. Before anything was stored: `nothing to show yet; run rhythm echo snapshot, compare or run first`; an unknown name: `no snapshot named "nope"; see rhythm echo snapshots` (both exit `2`). It takes no `--url` (`rhythm echo: --url has no effect on report (rhythm echo report --help lists its flags)`).

```text
$ rhythm echo snapshots
NAME                     ACTIONS  TAKEN        SENT
before-update            2        2m ago       3 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events  (latest)
```

With none: `No snapshots yet. Take one: rhythm echo snapshot`. `--json` lists `{name, started, actions, messages, latest}`. Deleting the latest one makes the newest remaining snapshot the latest:

```text
$ rhythm echo snapshots delete after-update
Deleted snapshot after-update
The latest snapshot is now before-update
```

`report` and `snapshots` read `--config` and `--store` too.

### replay

```sh
rhythm echo replay --list
rhythm echo replay payment_intent.succeeded --to /?wc-api=wc_stripe --set data.object.amount=2400
rhythm echo replay lead.json --scheme hmac --type lead.created --to http://127.0.0.1:8080/hooks/lead -H "X-Source: crm"
```

See [Replay](Echo-Replay).

### latest

Prints only the one-time code or link from the newest message to someone, then a newline, for shell scripts and end-to-end tests:

```sh
code=$(rhythm echo latest --to ann@shop.test)
rhythm echo latest --to owner@shop.example.com --link --kind reset
```

```text
482913
http://staging.shop.test/wp-login.php?login=admin&key=CTfZnZrLAmWG5fkCi2zt&action=rp&wp_lang=en_US
```

| Flag | Default | Description |
|---|---|---|
| `--to <address>` | (required) | Whose message: an email address or a phone number. |
| `--code` | | Print the one-time code (the default, else the first link). |
| `--link` | | Print the first link: `magic`, `verify`, `reset`, `login` or `unsubscribe`. |
| `--kind <k>` | | With `--link`, only this kind. |
| `--channel <c,c>` | all | Only these channels, comma-separated. |
| `--action <name>`, `--run <id>` | | Only messages from this action or run. |
| `--since <d>` | `1m`, or any age with `--run` | Only messages from this long ago on, so the previous sign-in's code isn't printed while the new email is on its way. |
| `--wait <d>` | `10s` | How long to wait for a message, at most `1m`; `0` asks once. |
| `--json` | | The envelope with kind `latest`: the codes, the links and the message. |

It exits `1` when nothing arrives in time or the message has no code or link:

```text
rhythm echo: no message to nobody@shop.test arrived within 300ms (nor in the 1m before; --since looks further back)
rhythm echo: the newest message to ann@shop.test has no link (rhythm echo show d3e2ccbf826f)
```

A test that signs the same user in again should pass `--run` or a short `--since`, or run `rhythm echo clear --to …` first: Echo answers the newest match at once, which may be the previous one. Usage errors (exit `2`): `latest needs --to <address or phone>: whose code or link to print`, `pick --code or --link, not both`, `--kind "secret": use one of magic, verify, reset, login, unsubscribe`.

### release

```sh
rhythm echo release 2a5397874054 --to ann@client.example
```

```text
Released 2a5397874054 to ann@client.example (logged as 20a3da3c9442).
```

Sends one caught email through the relay `up` runs with `relay:` in the config, only to an address `relay: allow` lists. Refused: `rhythm echo: x@evil.test isn't on the relay's allowlist; nothing was sent` (exit `2`). See [Releasing an email](Echo-Inbox#releasing-an-email-for-sign-off).

### check and headers

```sh
rhythm echo check shop.com --provider sendgrid --selector s1
rhythm echo check shop.com --from-mail received.eml --mta-sts-fetch --json
rhythm echo headers received.eml --hops --check
cat received.eml | rhythm echo headers -
```

Both exit `1` on an error by default and `3` when the DNS resolver never answers. See [Deliverability](Echo-Deliverability).

### init, providers, checks

```text
rhythm echo init: write .echo/config.yaml with an action for this stack

Usage:
  rhythm echo init [--stack s] [--force]

Flags:
      --stack <s>            wordpress, laravel, next, node or php (default: from this
                             folder's files)
      --force                replace an existing .echo/config.yaml

Exit codes: 0 ok, 2 usage error or the config exists, 3 couldn't write it.
rhythm echo help lists every command.
```

`init` below another project's config notes `rhythm echo: note: <path> is above this folder; here the new one takes its place`. See [Configuration](Echo-Configuration#the-config-file). `providers` lists the 53 services by channel (`--json` for the list as JSON); see [Messages and checks](Echo-Messages-And-Checks#recognized-services). `checks` lists the checks and your rules, `checks --test` tries the rules' fixtures and exits `1` when one fails; see [Checks and rules](Echo-Checks-And-Rules).

### ca and sendmail

```sh
rhythm echo ca             # /home/you/.config/rhythm/echo/ca/ca.pem (created if needed)
rhythm echo ca --pem       # -----BEGIN CERTIFICATE----- …
rhythm echo ca --untrust   # how to stop trusting it
printf 'To: jamie@shop.test\nSubject: Booking confirmed\n\nSee you at 7.\n' | rhythm-echo sendmail -t -i
```

```text
To stop trusting Echo's CA, run the lines for each place you added it:

  rhythm-echo CA, serial 59BF165DF8023C23C690D30E78344B0B, expires 25 Oct 2027
    SHA-256 8D679967140EFF43D7F8D109240840480F4F40AEB269A59126004E005EC3E460
    SHA-1   5A4583B3DE03FFDB3BDECBA1594AFF808BCBE788

  Debian, Ubuntu, Alpine: sudo rm /usr/local/share/ca-certificates/rhythm-echo.crt && sudo update-ca-certificates
  Fedora, RHEL:           sudo rm /etc/pki/ca-trust/source/anchors/rhythm-echo.crt && sudo update-ca-trust
  macOS:                  sudo security delete-certificate -Z <SHA-1> /Library/Keychains/System.keychain
  Windows (admin shell):  certutil -delstore Root <SHA-1>
  Chrome, Firefox (NSS):  certutil -d sql:$HOME/.pki/nssdb -D -n "<the name you gave it>"
  Containers:             drop the line that copies ca.pem in and rebuild; unset NODE_EXTRA_CA_CERTS, REQUESTS_CA_BUNDLE, SSL_CERT_FILE or CURL_CA_BUNDLE where they point at it

To delete the CA itself, so nothing can sign with it again (Echo makes a new one when it next needs one):
  rm -r /home/you/.config/rhythm/echo/ca
```

`--untrust` never creates a CA. With none on the machine it starts `No Echo CA on this machine. If one was trusted before, remove it with:`. `ca --pem --untrust` is a usage error (`rhythm echo: pick --pem or --untrust, not both`). `sendmail` takes sendmail's own flags, not Echo's; `rhythm echo sendmail --help` prints them and where the message goes. See [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in).

### mcp, hub, schedule, conformance

Each has its own page and its own help (`rhythm echo help hub`): [MCP server](Echo-MCP), [Agency hub and reports](Echo-Hub-And-Reports), [Schedule](Echo-Schedule), [Conformance kit](Echo-Conformance). Their exit codes:

| Command | Exit codes |
|---|---|
| `mcp` | `0` stdin closed, `2` usage error or a bad config, `3` couldn't read or write |
| `hub` | `0` stopped, `2` usage error or a bad sites file, `3` couldn't listen or keep the inbox |
| `schedule` (with `--now` and `--once`) | `0` nothing broke, `1` a site broke or its update failed, `2` usage error or a bad sites file, `3` couldn't run |
| `conformance` | `0` every check passed, `1` a check failed, `2` usage error, `3` couldn't listen |

## Flags

### Catcher (`up`)

| Flag | Default | Description |
|---|---|---|
| `--seal` | on | Nothing leaves; Echo answers for every service. |
| `--observe` | | Record everything and send it for real. Can't be combined with `--seal`. |
| `--listen <addr>` | `127.0.0.1:8025` | The API and Echo Inbox. |
| `--smtp <addr>` | `127.0.0.1:1025` | The SMTP sink; `off` disables it. |
| `--proxy <addr>` | off | Also run the HTTP(S) proxy, e.g. `127.0.0.1:8026`. |
| `--mitm` | off | Decrypt HTTPS in the proxy with Echo's CA. |
| `--allow <hosts>` | | Hosts reachable for real even when sealed; comma-separated, repeatable, added to `allow`. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | Require this token from connectors and clients. Needed when the API, SMTP sink or proxy listens beyond loopback, then 16 characters or more (`openssl rand -hex 16`); the SMTP sink then asks for it as the password. |
| `--private-network` | `RHYTHM_ECHO_PRIVATE_NETWORK` | Listen beyond loopback without a token, on private (RFC 1918) and container networks only; prints a warning. |
| `--hostname <name>` | `RHYTHM_ECHO_HOSTNAMES` | Also answer to this name without a token, e.g. `*.ddev.site`; repeatable. |
| `--free-ports` | | When 8025 or 1025 is taken, move to the next free pair from 18025/11025 and save it to the config without asking. |
| `--login <user:pass>` | `RHYTHM_ECHO_LOGIN` | Let viewers sign in to Echo Inbox, read only, for 8 hours (with a relay, they may also release an email). |
| `--sites <file>` | | A write token per site (captures only), from a hub `sites.yaml`; its read token reads all. |
| `--store <dir>` | memory | Keep what Echo catches in this folder, across restarts. |
| `--keep <n>` | `10000` | The most messages kept (never more than 256 MiB of them). |
| `--keep-for <d>` | | With `--store`: drop messages older than this, e.g. `14d`. |
| `--spamassassin <addr>` | | Score each email with the SpamAssassin daemon at `host:port` (port 783 when left out). |

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
| `--plugin` | (`wordpress`) Install Echo Staging Seal, the WordPress.org plugin. |
| `--hub <url>` | (`wordpress`) Report to an agency hub, with `--site` and the site's write token in `RHYTHM_ECHO_WRITE_TOKEN`. |
| `--site <id>` | (`wordpress`) The site's ID, named in every capture. |
| `--bin <path>` | (`wordpress`) The `rhythm-echo` the plugin's buttons run. |
| `--project <dir>` | (`wordpress`) The folder with `.echo/config.yaml` they run in. |
| `--config <file>` | Where to read the addresses from. Default: the nearest `.echo/config.yaml`. |

### Output

| Flag | Description |
|---|---|
| `--json`, `--md`, `--junit`, `--html` | The report format; pick one. Alone, it goes to stdout instead of the terminal view. |
| `-o, --output <file>` | Write the report to a file (readable by you only); the format comes from the extension (`.json`, `.md`, `.xml`, `.html`) unless a format flag is given. The terminal view still prints. |
| `--open` | Open the HTML report when done. |
| `-q, --quiet` | Results only: no header, progress or notes. On `snapshot`, `compare` and `run`, `--quiet <d>` with a duration sets the quiet window instead. |
| `--plain` | No color, boxes or glyphs. On automatically when stdout isn't a terminal. |
| `--no-color` | Keep the layout, drop the color. |
| `-v, --verbose` | Show every message and each change's key. |
| `--fail-on <level>` | Exit `1` at or above this level. `compare` (and `report` of a comparison): changes at `broke` (the default), `new` or `changed`. `run`, `snapshot`, `check` and `headers` (and `report` of a run): issues at `error` (the default), `warning` or `notice`. `run`, `snapshot` and `compare` also exit `1` when an action fails; `none` turns both off. Any case. |

`rhythm echo: pick one output format, not --json and --md`; `rhythm echo: can't tell the format of out.txt; add --json, --md, --junit or --html`.

### Actions and messages

| Flag | Description |
|---|---|
| `--config <file>` | Settings. Default: the nearest `.echo/config.yaml`, here or above. |
| `--store <dir>` | Where snapshots and runs are kept. Default: the config's folder. |
| `--url <url>` | Where Echo runs, for every command but `up`. Default: `RHYTHM_ECHO_URL`, else `http://` + `listen`. |
| `--name <name>` | Snapshot name: up to 64 letters, digits, dots, dashes and underscores. Default: the date and time. |
| `--force` | `snapshot`: save it even when every action failed; with `--import`, replace one of the same name. |
| `--snapshot <name>` | The snapshot `compare` uses. Default: the latest. |
| `--against <path>` | `compare`: with this run instead: a run folder, its `run.json` or a `.tgz` from `snapshot --export`. |
| `--export <file.tgz>`, `--import <file.tgz>` | `snapshot`: write one to a `.tgz` (`-` for stdout), or add one from a `.tgz` and make it the latest. |
| `--accept <key>` | `compare`: take the change with this key (from `-v`) as intended; repeatable. |
| `--action <name>` | Run only this action, ignoring case; repeatable. |
| `--quiet <d>` | How long nothing must arrive before an action is done (overrides `quiet:`, at most `30s`). |
| `--timeout <d>` | The longest wait per action (overrides `timeout:`, at most `1m`). |
| `--parallel <n>` | Run up to `n` actions at once; one with `parallel: false` runs alone. |
| `--channel <c>` | Only `email`, `sms`, `whatsapp`, `webhook`, `chat`, `analytics`, `payment`, `push` or `http`; for `watch`, `ls` and `clear`. |
| `--to <address>` | Only messages to this recipient (part of it, ignoring case). |
| `--run <id>` | Only messages from this run. |
| `--since <d>` | `ls`, `watch`: messages from this long ago on, e.g. `10m`. |

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
| `--resolver <host:port>` | (`check`, `headers --check`) The DNS server to ask; port 53 when left out. Default: the system's. |
| `--mta-sts-fetch` | (`check`, `headers --check`) Also fetch the MTA-STS policy file over HTTPS. |
| `--check` | (`headers`) Also run the DNS checks for the sender. |
| `--hops` | (`headers`) Show the route the message took. |

### Agency report

| Flag | Default | Description |
|---|---|---|
| `--site <id>` | | The hub site to report on. |
| `--since <d>` | `30d` | The period. |
| `--pdf` | | Also print it to a PDF beside the HTML (needs Chrome or Chromium); `-o <file>.pdf` writes the PDF there and the HTML beside it. |
| `--brand-name <name>`, `--brand-logo <file>`, `--brand-color <#hex>` | | The agency's name, logo (PNG, JPEG, GIF, WebP or SVG) and accent color. |

## JSON output

Every `--json` (and `-o <file>.json`) writes one envelope, whatever the command:

```json
{
  "schema": "rhythm-echo/v1",
  "kind": "comparison",
  "tool": "rhythm-echo",
  "version": "1.1.1",
  "complete": true,
  "summary": {
    "broke": 0,
    "new": 0,
    "changed": 0,
    "fixed": 0,
    "errors": 0,
    "warnings": 1,
    "notices": 0,
    "actions": 2,
    "messages": 9,
    "failed_actions": 0
  },
  "verdict": "findings",
  "data": { … }
}
```

| Field | Means |
|---|---|
| `kind` | `run` (`run`, `snapshot`, `report` of a run), `comparison` (`compare`, `report` of one), `check`, `headers`, `messages` (`ls`, `show`), `snapshots`, `doctor`, `replay`, `latest`, `providers`. |
| `summary` | The counts, as the reports show them; for a comparison, `actions` is the number of actions compared. |
| `verdict` | `error` when the result is incomplete, `findings` when anything is BROKE, NEW or CHANGED or there's any issue, `ok` otherwise. It doesn't depend on `--fail-on`. |
| `complete` | `false` when an action failed, a payload was cut to size or the DNS resolver couldn't be reached. |
| `data` | The run, the comparison (with `actions`, the compared run's actions), the DNS report, the message list and so on. Empty lists are `[]`. |

`watch --json` streams one object per line instead. The files Echo stores, `run.json` and `comparison.json`, keep their own shape, which the envelope's `data` carries. JSON Schemas for each kind ship in the Echo repository under `report/schema/`.

## Exit codes

The same codes as Tuner and Soundcheck. The help lists them as:

```text
  0    ok
  1    findings: --fail-on hit, an action failed, a doctor check failed, or the site
       rejected a replay
  2    usage error: a wrong command line, or .echo/config.yaml asks for what isn't there
  3    couldn't run: Echo not reachable, every action failed, the DNS resolver never
       answered, a file or network error
  130  interrupted
  sendmail exits with sendmail's codes instead (sysexits: 64 usage, 69 Echo unreachable…)
```

| Code | Meaning |
|---|---|
| `0` | Done; nothing at or above `--fail-on` |
| `1` | A change or issue at or above `--fail-on` (by default BROKE for a comparison, an error otherwise); an action failed; a failed `doctor` check; a `replay` the site didn't answer with `2xx`; `latest` found nothing; a rule fixture failed in `checks --test` |
| `2` | Usage error, always before any output: a bad flag or value, an unknown command, stack, action, channel, provider, snapshot or message ID, no config, no snapshot yet, a folder that isn't WordPress, a missing signing secret, a refused replay, a config that doesn't load |
| `3` | Couldn't run: Echo isn't running or another service answers (`no Echo is running at http://127.0.0.1:8025 (start it with: rhythm echo up)`), every action failed, the DNS resolver never answered, a port is taken, a file or network error |
| `130` | Interrupted; results are partial |

`rhythm echo sendmail` uses sendmail's codes instead; see [The sendmail stand-in](Echo-Install-Connectors#the-sendmail-stand-in).

## Environment variables

| Variable | Description |
|---|---|
| `RHYTHM_ECHO_URL` | Where Echo runs; `--url` wins. |
| `RHYTHM_ECHO_TOKEN` | The token `up` requires and other commands send; `--token` wins. |
| `RHYTHM_ECHO_WRITE_TOKEN` | A hub site's write token; signs the run IDs actions send. |
| `RHYTHM_ECHO_PRIVATE_NETWORK`, `RHYTHM_ECHO_HOSTNAMES`, `RHYTHM_ECHO_LOGIN`, `RHYTHM_ECHO_RELAY` | For `up`: `--private-network`, `--hostname`, `--login` and the relay's SMTP account. |
| `ECHO_STRIPE_WEBHOOK_SECRET`, `ECHO_WEBHOOK_SECRET`, `ECHO_GITHUB_WEBHOOK_SECRET`, `ECHO_WC_WEBHOOK_SECRET` | Replay signing secrets. |
| `RHYTHM_ECHO_SMTP`, `RHYTHM_ECHO_RUN`, `RHYTHM_ECHO_FORCE` | For `sendmail`. |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day release check. |
| `CI` | When set, the release check is skipped too. |
| `RHYTHM_GITHUB_API` | GitHub API base for the release check. Default `https://api.github.com`. |
| `GITHUB_TOKEN` | Sent with release checks. |

The connectors' variables are in [Configuration](Echo-Configuration#environment-variables).

## Files and locations

| What | Where |
|---|---|
| Settings | `.echo/config.yaml`, found here or in a folder above |
| Snapshots | `.echo/snapshots/<name>/` and `.echo/snapshots/.latest` |
| Runs and comparisons (the 20 newest) | `.echo/runs/<date-time>-<id>/` |
| A kept inbox | the `--store` folder (or `inbox: store:`) |
| The proxy's CA | `~/.config/rhythm/echo/ca` (Linux), `~/Library/Application Support/rhythm/echo/ca` (macOS), `%AppData%\rhythm\echo\ca` (Windows) |
| Release check state | `~/.config/rhythm/update-check/rhythm-echo.json` |
| First-run notice marker | `~/.config/rhythm/seen/echo-authorised-use` |

See [The .echo folder](Echo-Configuration#the-echo-folder). The responsible-use notice is shown once per machine; deleting its marker brings it back.

---
<sub>← [API](Echo-API) · [Home](Home) · [FAQ and troubleshooting](Echo-FAQ) →</sub>
