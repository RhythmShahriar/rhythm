Every Echo command and flag, its exit codes, the JSON every command writes, the environment variables it reads and where it keeps files. Verified against rhythm-echo 1.1.1; the consent commands and the changed help lines against rhythm-echo 1.2.0.

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
| `rhythm echo consent <url>… [flags]` | | Check cookie consent in a real browser: no choice, Accept and Reject. See [Cookie consent](Echo-Consent). |
| `rhythm echo consent detect <url>` | | Find the banner and the buttons Echo would click; no verdict. |
| `rhythm echo consent report [<id> \| latest]` | | Show a saved consent result again. |
| `rhythm echo consent verify <folder \| file> [--key <file>]` | | Check an evidence folder's hashes and signature. |
| `rhythm echo consent cmps`, `consent trackers` | | The consent banners Echo knows how to click; the trackers it recognizes. |
| `rhythm echo consent prune [--keep <n>]` | | Delete old consent results, keeping the newest `n` (default 20) and every baseline. |
| `rhythm echo init [--stack s] [--force]` | | Write `.echo/config.yaml` with an action for this stack; `--consent` adds a commented `consent:` block instead. |
| `rhythm echo providers` | | List the services Echo recognizes, by channel. |
| `rhythm echo checks [--test]` | | List the checks and the config's rules; `--test` tries the rules' fixtures. |
| `rhythm echo ca [--pem \| --untrust]` | | Print the proxy CA certificate's path (creating it if needed), the certificate, or how to stop trusting it. |
| `rhythm echo mcp` | | The [MCP server](Echo-MCP) for coding agents, on stdin and stdout. |
| `rhythm echo hub --store D --sites F` | | One Echo for many staging sites. See [Agency hub and reports](Echo-Hub-And-Reports). |
| `rhythm echo schedule "<when>" --store D --after "<command>"` | | Snapshot, update and compare the hub's sites in a window. See [Schedule](Echo-Schedule). |
| `rhythm echo conformance [--listen a]` | | A strict stand-in Echo for a connector's end-to-end test. See [Conformance kit](Echo-Conformance). |
| `rhythm echo sendmail [-t] [-i] [-f from] [rcpt…]` | | A `sendmail` stand-in for `sendmail_path`; reads the message on stdin and hands it to Echo. |
| `rhythm echo update` | | Nothing to refresh; exits `0`, so `rhythm update` can call it. |
| `rhythm echo version` | `-v`, `--version` | Print the version: `rhythm-echo 1.2.0`. `version --json` gives name, version, commit and kit version. |
| `rhythm echo help [command]` | `-h`, `--help` | Print the help text, or one command's. |

Running `rhythm echo` with no command on a terminal asks what to do: a menu offers the likeliest next step first (start Echo, or watch when it's already running; compare with the latest snapshot; take a snapshot; run the actions; show the last result; connect a site; doctor; `init` when there's no config; check cookie consent on a page, with the URL prefilled from `site:`), asks the follow-up questions, then runs that command. Piped or in CI it prints the help instead.

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

### consent

```sh
rhythm echo consent https://staging.shop.test                     # no choice, Accept and Reject
rhythm echo consent --runs +gpc,+withdraw --geo de https://shop.example
rhythm echo consent https://staging.shop.test --save baseline     # keep it as the baseline
rhythm echo consent --against baseline --fail-on broke            # pages from consent: urls
rhythm echo consent --sites sites.yaml --parallel 6               # every site in a Soundcheck-style file
rhythm echo consent detect https://staging.shop.test              # the banner and buttons, no verdict
rhythm echo consent report latest --md                            # a saved result again
rhythm echo consent verify .echo/consent/<id> --key team.pub      # hashes and signature
```

`consent` needs no `.echo/config.yaml` and no running Echo. Every page, flag and output is explained in [Cookie consent](Echo-Consent) and the pages it links. Its help, in full:

```text
rhythm echo consent: check cookie consent with no choice, Accept and Reject

Does Reject really reject? Echo clicks your cookie banner three ways and shows
every tracker that fired before the choice or after Reject, with Consent Mode
decoded from the wire.

Usage:
  rhythm echo consent [<url>...] [flags]

What to check:
      --runs <list>          none, accept, reject, gpc, withdraw (default
                             none,accept,reject); +gpc or +withdraw adds to the
                             default
      --pages <n>            after the choice, visit this many more same-site
                             pages (default 1; 0 only reloads; at most 10)
      --settle <d>           how long the network must be quiet after each step
                             (default 3s, at most 30s)
      --banner-wait <d>      how long to wait for a banner (default 10s, at most
                             60s)
      --timeout <d>          longest time for one run (default 90s, at most 10m)
      --retries <n>          run an inconclusive run again in a fresh browser
                             (default 1; 0 to 3)
      --cmp <name>           auto (default) or the banner to expect: cookiebot,
                             onetrust, cookieyes, complianz, usercentrics,
                             borlabs, didomi, iubenda, generic (rhythm echo
                             consent cmps lists them)
      --accept-selector <sel>
                             the Accept button, overriding the rule; a >> b
                             clicks a then b, host::shadow button enters a
                             shadow root
      --reject-selector <sel>
                             the Reject button, same syntax, e.g. "#settings >>
                             #reject-all"
      --click <how>          button (default), api or button,api (the API when
                             no button is found)
      --consent-mode <policy>
                             advanced (default: cookieless pings are a notice),
                             basic (they count as hits) or strict (tracker
                             scripts count too)
      --exempt <tracker>     a tracker allowed before consent, by catalog id
                             (repeatable)
      --geo <cc>             look like a visitor from this country, e.g. de:
                             language and time zone, never the IP address
      --locale <tag>         browser locale and Accept-Language, e.g. de-DE
                             (default the machine's)
      --timezone <tz>        IANA time zone, e.g. Europe/Berlin (default the
                             machine's)
      --viewport <v>         desktop (default, 1366x900) or mobile (390x844,
                             touch)
      --echo[=<url>]         also ask Echo (RHYTHM_ECHO_URL, else
                             http://127.0.0.1:8025) what the site sent
                             server-side during each run
      --send-hits            send tracker hits for real instead of holding them
                             in the browser; refused on a production: host
      --sites <file>         check every site in a Soundcheck-style sites.yaml
      --parallel <n>         browser runs at once (default 3; 1 to 8)
      --map-host <host=addr> answer host (or *.example.test) from addr instead
                             of the network, e.g. *.shop.test=127.0.0.1:18950
                             (repeatable)
      --map-only             refuse every host --map-host doesn't cover

Compare:
      --save <name>          also keep this result as the baseline <name>
      --against <name|path|last>
                             compare with a baseline, a consent.json, a result
                             folder or the previous result (last)
      --accept <key>         take this change (its key from -v) as intended
                             (repeatable)

Requests and browser:
  -H, --header <h>           extra header for same-site requests, "Name: value";
                             the value may be @file or env:NAME (repeatable)
      --cookie <c>           a cookie for the site, name=value; the value may be
                             @file or env:NAME (repeatable)
  -A, --user-agent <ua>      the User-Agent (default the browser's own, without
                             Headless, plus rhythm-echo-consent/<version>)
      --proxy <url>          send the browser's traffic on through this proxy,
                             e.g. one in the EU
  -k, --insecure             don't verify TLS; only for loopback or private
                             sites
      --insecure-public      with -k, allow it for a public site too
      --follow-host          keep going when the start page redirects to another
                             host
      --browser <path>       the Chrome or Chromium to use
      --no-download          never download Chrome for Testing
      --no-sandbox           run Chrome without its sandbox (needed as root;
                             also RHYTHM_ECHO_NO_SANDBOX=1); only for sites you
                             trust
      --show-browser         show the browser window (one run at a time)

Output:
      --json --md --junit --html
                             the report on stdout, one format at a time
  -o, --output <file>        also write a report to a file, its format from
                             .json .md .xml or .html (repeatable)
      --har                  keep one masked HAR per run in the evidence folder
      --no-screenshots       take no screenshots
      --show-ids             keep tracking and client IDs unmasked (not with
                             Markdown or JUnit)
      --sign-key <file>      sign consent.json with this ed25519 private key
                             (mode 0600)
      --open                 open the HTML report when done
  -q, --quiet                only the verdict and what failed
      --plain                no color or glyphs (screen readers, logs)
      --no-color             no color
  -v, --verbose              add every run's timeline and the change keys
      --live                 print each tracker hit on stderr as it happens
      --fail-on <level>      exit 1 at or above error (the default), warning or
                             notice; with --against broke (the default), new or
                             changed; none: never

Settings:
      --config <file>        settings (default the nearest .echo/config.yaml;
                             optional)
      --store <dir>          where results are kept (default the config's
                             folder, else ./.echo)

Subcommands (rhythm echo consent <subcommand> --help lists each one's flags):
  detect <url>               find the banner and the buttons to click; no
                             verdict
  report [<id>|latest]       show a saved consent result again
  verify <folder|file> [--key <file>]
                             check an evidence folder's hashes and signature
  cmps [--json]              the consent banners Echo knows how to click
  trackers [--json]          the trackers Echo recognizes
  prune [--keep <n>]         delete old consent results, keeping the newest n
                             (default 20) and every baseline

Exit codes:
  0    every run made and confirmed its choice (or the page needed none) and no
       issue is at or above --fail-on
  1    an issue at or above --fail-on (with --against: a change at or above it);
       wins over 3
  2    usage error: wrong flags, a bad URL, an unknown consent: key or check
       ID, a literal header value in the config, a missing baseline,
       --send-hits on a production: host, --show-ids with Markdown or JUnit,
       an invalid selector
  3    couldn't check: no Chrome, the site didn't answer, a TLS error without
       -k, a bot wall, a redirect loop, a run that couldn't make its choice
       and nothing at or above --fail-on, a report that couldn't be written
  130  interrupted: finished runs are kept and the result is complete: false

Examples:
  rhythm echo consent https://staging.shop.test
  rhythm echo consent --runs +gpc --geo de https://shop.example
  rhythm echo consent --against baseline --fail-on broke

What a pass means. In these runs, from this machine, with this browser and these
settings, no tracker on Echo's list sent a request or set a cookie before the
visitor chose or after Reject. It does not mean the site complies with the GDPR,
ePrivacy, CCPA/CPRA or any other law. Evidence, not legal advice.

One visitor, one place. Banners differ by country, language and device.
Geo-targeted banners may not show to a CI runner in the US; Echo can set the
browser's language and time zone (`--geo`) but not its IP address. Run it from
the region that matters, or through a proxy there.

Blind spots. Trackers that are not on the list; server-side tagging on your own
domain when its requests don't look like GA4 or Meta; fingerprinting without
network requests; tags that wait longer than the settle time or need more
interaction than a scroll and one more page; sites that hide tags from automated
browsers.

Consent Mode advanced. Google says its tags in advanced mode "send measurements
without cookies" while consent is denied. Whether that is acceptable before
consent is disputed. Echo reports these pings as `consent/cookieless-ping` under
the default policy `advanced`; `--consent-mode basic` or `strict` counts them as
hits.

gcd is unofficial. Google doesn't document the `gcd` parameter. Echo shows the
raw value next to a reading based on community documentation and treats letters
it doesn't know as unknown.

Pseudonymised, not anonymised. Masked or hashed identifiers in reports can still
be personal data. Treat reports like logs.

What leaves the machine. Requests to the site and to the scripts it loads (which
shows this machine's IP address to those servers). No tracking hits unless
`--send-hits`. Nothing goes to Rhythm. The update check contacts GitHub
(`RHYTHM_OFFLINE=1` turns it off).

Banners Echo can't handle are reported as "couldn't check" (exit 3), never as a
pass. Tested CMPs and the dates their fixtures were captured are in the
benchmark.

A signature proves who made the evidence file and that it hasn't changed. It
doesn't prove the site behaves the same for every visitor, region or day.
Keyless signing in the Action writes the repository and workflow name to the
public Rekor log.

rhythm echo help lists every command.
```

The first word that isn't a flag picks a subcommand (`detect`, `report`, `verify`, `cmps`, `trackers`, `prune`); anything else is a URL. A subcommand's name later on the line is refused, so a host called `report` is written as a URL:

```text
rhythm echo: consent "report": report is a subcommand; put it first (rhythm echo consent report), or write the host as a URL (http://report/)
```

Each subcommand has its own `--help` listing only the flags it reads:

| Subcommand | Flags | Exit codes |
|---|---|---|
| `detect <url>` | `--banner-wait`, `--cmp`, `--accept-selector`, `--reject-selector`, `--geo`, `--locale`, `--timezone`, `--viewport`, `--map-host`, `--map-only`, the request and browser flags, `--json`, `-q`, `--plain`, `--no-color`, `--config`, `--store` | `0` a banner and both buttons found, `1` a banner but a button missing, `2` usage error, `3` no banner or couldn't run, `130` interrupted |
| `report [<id>\|latest]` | `--against`, `--accept`, `--json`, `--md`, `--junit`, `--html`, `-o`, `--open`, `-q`, `--plain`, `--no-color`, `-v`, `--fail-on`, `--config`, `--store` | as `consent` for that result: `0` ok, `1` findings, `2` usage error, `3` couldn't check |
| `verify <folder\|file>` | `--key <file>` (the ed25519 public key), `--json`, `-q`, `--plain`, `--no-color` | `0` every file and the signature match, `1` a hash or the signature doesn't match, `2` usage error, `3` a file is missing or unreadable |
| `cmps` | `--json`, `-q`, `--plain`, `--no-color` | `0` ok, `2` usage error, `3` couldn't run |
| `trackers` | `--json`, `-q`, `--plain`, `--no-color`, `--config` (to list your own trackers too) | `0` ok, `2` usage error, `3` couldn't run |
| `prune` | `--keep <n>` (default `20`), `-q`, `--config`, `--store` | `0` ok, `2` usage error, `3` couldn't run |

Usage errors exit `2` before any browser starts:

```text
rhythm echo: consent needs a page: rhythm echo consent https://staging.shop.test, or list pages under consent: urls in .echo/config.yaml
rhythm echo: consent "file:///etc/passwd": want an http or https URL
rhythm echo: consent "not-a-url": not-a-url isn't a URL; did you mean https://not-a-url.example/? A host without a dot needs its scheme: https://not-a-url/
rhythm echo: --map-host '*.shop.test=...' kept its quotes; use eval, as the testbed section shows: eval "rhythm-echo consent <url> $map"
rhythm echo: --fail-on "high": want none, error, warning or notice (or broke, new, changed when comparing)
rhythm echo: --runs "maybe": want none, accept, reject, gpc or withdraw (+gpc adds one)
rhythm echo: --pages 11: want 0 to 10
rhythm echo: --exempt "nope": no tracker by that id; rhythm echo consent trackers lists them
rhythm echo: --show-ids keeps tracking IDs unmasked; it can't be used with Markdown or JUnit output, which are meant to be shared
rhythm echo: --send-hits on c01.shop.test, which matches "c01.shop.test" under production:. Echo holds tracker hits on production so your analytics stay clean; leave --send-hits off
rhythm echo: --against "nosuch": no saved consent result by that name in .echo/consent/baselines; rhythm echo consent <url> --save nosuch makes one
```

A site that doesn't answer is exit `3`:

```text
rhythm echo: https://down.shop.test/ didn't answer: http://127.0.0.1:39144 (--map-host): dial tcp 127.0.0.1:39144: connect: connection refused. Is the site at the --map-host address running?
```

With `--json` either kind of stop still writes the envelope to stdout, with `verdict: error`, `complete: false`, `data: null` and the reason in `error` (this one from a real run of a dev build):

```json
{
  "schema": "rhythm-echo/v1",
  "kind": "consent",
  "tool": "rhythm-echo",
  "version": "dev",
  "complete": false,
  "summary": { "broke": 0, "new": 0, "changed": 0, "fixed": 0, "errors": 0, "warnings": 0, "notices": 0, "pages": 0, "runs": 0, "failed_runs": 0, "inconclusive_runs": 0, "trackers": 0, "before_choice": 0, "after_reject": 0, "held": 0 },
  "verdict": "error",
  "data": null,
  "error": {
    "exit": 3,
    "message": "https://c01.shop.test/ didn't answer: http://127.0.0.1:39149 (--map-host): dial tcp 127.0.0.1:39149: connect: connection refused. Is the site at the --map-host address running?"
  }
}
```

`-q` prints the verdict line, the issues and the runs that couldn't make their choice, so a log of an exit `3` says why. The header names the browser on every run (`Browser: Chrome 153.0.8010.52`); `-v` adds a stderr line with its path.

`doctor` adds one line about the browser consent checks will use:

```text
ok    Chrome for consent checks: found (Chrome 153 at /usr/bin/google-chrome)
```

### init, providers, checks

```text
rhythm echo init: write .echo/config.yaml with an action for this stack

Usage:
  rhythm echo init [--stack s] [--force]
  rhythm echo init --consent [--force]

Flags:
      --stack <s>            wordpress, laravel, next, node or php (default: from this
                             folder's files)
      --consent              add a commented consent: block to .echo/config.yaml, or
                             write a config with only that block
      --force                replace an existing .echo/config.yaml; with --consent,
                             replace only its consent: block

Exit codes: 0 ok, 2 usage error or the config exists, 3 couldn't write it.
rhythm echo help lists every command.
```

`init --consent` writes the block shown in [Configuration](Echo-Configuration#consent) and refuses a config that already has one: `rhythm echo: .echo/config.yaml already has a consent: block; pass --force to replace only that block`. `init` below another project's config notes `rhythm echo: note: <path> is above this folder; here the new one takes its place`. See [Configuration](Echo-Configuration#the-config-file). `providers` lists the 54 services by channel (`--json` for the list as JSON); see [Messages and checks](Echo-Messages-And-Checks#recognized-services). `checks` lists the checks and your rules, the 21 consent checks among them (channel `consent`); `checks --test` tries the rules' fixtures and the fixtures of your own consent trackers and exits `1` when one fails; see [Checks and rules](Echo-Checks-And-Rules).

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

### Consent

`consent` and its subcommands have flags of their own, listed with their defaults in [its help](#consent); [Cookie consent](Echo-Consent) explains them. Flags shared by name keep their meaning: `--accept <key>` takes a change as intended, as on `compare`; the Accept button is `--accept-selector`. `--fail-on` takes `error`, `warning`, `notice` or `none`; with `--against` it takes `broke`, `new`, `changed` or `none`. `-H` and `--cookie` take `@file` or `env:NAME` for the value; a plain value warns that it shows in `ps`.

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
| `kind` | `run` (`run`, `snapshot`, `report` of a run), `comparison` (`compare`, `report` of one), `check`, `headers`, `messages` (`ls`, `show`), `snapshots`, `doctor`, `replay`, `latest`, `providers`, `consent` (`consent`, `consent report`), `consent-detect`, `consent-verify`, `consent-catalog` (`consent cmps`, `consent trackers`). |
| `summary` | The counts, as the reports show them; for a comparison, `actions` is the number of actions compared. A `consent` result adds `pages`, `runs`, `failed_runs`, `inconclusive_runs`, `trackers`, `before_choice`, `after_reject` and `held`, always present; see [Evidence](Echo-Consent-Evidence#the-json-envelope). |
| `verdict` | `error` when the result is incomplete, `findings` when anything is BROKE, NEW or CHANGED or there's any issue, `ok` otherwise. It doesn't depend on `--fail-on`. |
| `complete` | `false` when an action failed, a payload was cut to size or the DNS resolver couldn't be reached. |
| `error` | Only in `consent`, when the check stopped before it had a result (exit `2` or `3`): `{exit, message}`, with `data` null. |
| `data` | The run, the comparison (with `actions`, the compared run's actions), the DNS report, the message list and so on. Empty lists are `[]`. |

`watch --json` streams one object per line instead. The files Echo stores, `run.json` and `comparison.json`, keep their own shape, which the envelope's `data` carries. JSON Schemas for each kind ship in the Echo repository under `report/schema/`.

## Exit codes

The same codes as Tuner and Soundcheck. The help lists them as:

```text
  0    ok
  1    findings: --fail-on hit, an action failed, a doctor check failed, or the site
       rejected a replay
  2    usage error: a wrong command line, or .echo/config.yaml asks for what isn't there
  3    couldn't run or couldn't check (a consent run that couldn't make its choice):
       Echo not reachable, every action failed, the DNS resolver never answered, a
       file or network error
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

`consent` uses the same codes; its `1` wins over `3` and a run that couldn't make or confirm its choice is `3`, never a pass. See [Cookie consent](Echo-Consent#exit-codes).

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
| `RHYTHM_ECHO_NO_SANDBOX`, `SOUNDCHECK_NO_SANDBOX` | `1` is `consent --no-sandbox`: Chrome without its sandbox, needed as root in a container. |
| `RHYTHM_ECHO_<NAME>` | Values for `consent: headers:` in the config (`${env:RHYTHM_ECHO_<NAME>}`). The `env:NAME` form of `-H` and `--cookie` reads any variable. |
| `RHYTHM_OFFLINE` | Any value skips the release check (kit). |
| `RHYTHM_NO_UPDATE_CHECK` | Any value stops the once-a-day release check. |
| `CI` | When set, the release check is skipped too; `consent` shows no once-only notice, no tip and no color. |
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
| Consent results | `.echo/consent/<date-time>-<id>/` (the time in UTC, with a `Z`), next to the config or in `./.echo`; never pruned on their own (`consent prune`) |
| Consent baselines | `.echo/consent/baselines/<name>/consent.json` |
| `consent detect` screenshots | `.echo/consent/detect/<date-time>-<id>.png` |
| Chrome for Testing, when downloaded | `~/.cache/rhythm/soundcheck/browser`, shared with Soundcheck |
| The proxy's CA | `~/.config/rhythm/echo/ca` (Linux), `~/Library/Application Support/rhythm/echo/ca` (macOS), `%AppData%\rhythm\echo\ca` (Windows) |
| Release check state | `~/.config/rhythm/update-check/rhythm-echo.json` |
| First-run notice markers | `~/.config/rhythm/seen/echo-authorised-use`; for `consent`, `echo-consent-notice` and `echo-consent-tip-save` |

See [The .echo folder](Echo-Configuration#the-echo-folder). The responsible-use notice is shown once per machine; deleting its marker brings it back.

---
<sub>← [API](Echo-API) · [Home](Home) · [FAQ and troubleshooting](Echo-FAQ) →</sub>
