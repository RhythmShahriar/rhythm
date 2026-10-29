How well does Echo catch what goes wrong with what real sites send and with the environment around it? How does it compare with a mail catcher? This page describes the benchmark: a testbed of 20 broken site scenarios on WordPress, Laravel, Next.js and plain PHP and 15 environment scenarios, how each one is run and judged and the results. **Echo caught 20 of 20 site scenarios and 14 of 15 environment scenarios**; the one left (e13) needs a Windows machine and wasn't run. The cookie consent checks have a testbed of their own, [below](#consent-checks): 25 of 25 broken setups caught and no false alarm in 400 checks of correct ones.

## The testbed

The testbed lives in the Echo repository, in [`testbed/`](https://github.com/RhythmShahriar/echo/tree/master/testbed). It runs in Docker Compose, on ports bound to `127.0.0.1`:

| Site | Address | What it is |
|---|---|---|
| WordPress | `http://localhost:18850` | WordPress 6.8.3 on PHP 8.3 with MariaDB 11.4: "Bean & Leaf", a coffee shop, with fixture plugins for a contact form, a WooCommerce stand-in (a receipt through WP-Cron, Twilio SMS, Slack, GA4, Meta CAPI, webhooks through an Action Scheduler stand-in), an SMS add-on whose Twilio client is raw cURL and a Stripe webhook endpoint |
| Laravel | `http://localhost:18851` | A stock Laravel app with `rhythmsh/echo-laravel`: an order queues a Mailable and calls Twilio with the `Http` client |
| Next.js | `http://localhost:18852` | An app with `@rhythmsh/echo`: a signup route (a Resend-style `fetch`, Twilio `fetch`, Nodemailer) and an Edge route (Slack) |
| Plain PHP | `http://localhost:18853` | A legacy order page that sends with `mail()`, through `sendmail_path` = `rhythm-echo sendmail` |
| Mailpit | `http://localhost:18854` | The shop's "real" mail server in observe mode. It's also a comparison tool |
| Buggregator | `http://localhost:18857` | With `TOOLS=buggregator`: a PHP debug hub with an SMTP catcher, the second comparison tool |

The sites sit on a Docker network with `internal: true`, so nothing in the stack can reach the internet, whatever a connector or Echo decides: it's the [hard seal](Echo-Sealing-Staging#docker-internal-network), with Echo on the host. Every connector is installed from the checkout (the mu-plugin with `rhythm echo install wordpress`). Every run starts from the same seeded WordPress.

Each scenario has a **good** state and a **broken** state. `testbed/toggle.sh s05 broken` breaks one by creating `testbed/state/s05`, which the fixtures check on every request, job and command: no rebuild, no restart.

## The 20 scenarios

The manifest, `testbed/scenarios.yaml`, says what each scenario breaks, where, which Echo action shows it and what counts as caught.

| # | Area | Broken state | Caught when the comparison has |
|---|---|---|---|
| s01 | Lost messages | the contact form no longer emails the shop owner | BROKE `message/missing` |
| s02 | Lost messages | the contact form's autoresponder is off | BROKE `message/missing` |
| s03 | Lost messages | the SMS add-on (raw cURL Twilio client, caught by Echo's proxy) listens on a hook that's never fired | BROKE `message/missing` |
| s04 | Lost messages | the new-order Slack alert is gone | BROKE `message/missing` |
| s05 | Lost messages | the GA4 purchase event isn't sent | BROKE `message/missing` |
| s06 | Lost messages | Laravel's queued Mailable names a missing view, so the job throws in the worker | BROKE `message/missing` |
| s07 | Lost messages | the Next.js Resend call throws (a renamed API key variable); the route still answers 200 | BROKE `message/missing` |
| s08 | Lost messages | the crontab line fails, so WP-Cron never sends the deferred receipt | BROKE `message/missing` |
| s09 | Wrong content | the owner's notification goes to the old agency's address | NEW `message/new` or BROKE `message/missing` |
| s10 | Wrong content | `{customer_first_name}` is left in the receipt | BROKE `content/placeholder` |
| s11 | Wrong content | plain PHP prints `Warning: Undefined variable $total` into the order email | BROKE `content/php-error` |
| s12 | Wrong content | the receipt links to `staging.shop.test` | NEW `content/dev-link` |
| s13 | Wrong content | the order SMS gained emoji: UCS-2, 3 parts | NEW `sms/parts` |
| s14 | Wrong content | `order.created` no longer carries `billing.email` | BROKE `message/fields-lost` |
| s15 | Integrations | the lead goes to a test Zap's catch hook | BROKE `message/missing` |
| s16 | Integrations | the CRM answers 500 to the order webhook (observe mode); five failures disable it | BROKE `message/failing` or `send/failed` |
| s17 | Deliverability | the autoresponder is sent from a newsletter domain SendGrid isn't set up for | BROKE `dns/dmarc` |
| s18 | Deliverability | three more includes push SPF to 11 lookups | BROKE `dns/spf` |
| s19 | Staging safety | a staging refresh brings a real customer address, with Echo in observe mode | NEW `seal/real-recipient` |
| s20 | Inbound | a Stripe `payment_intent.succeeded` replay no longer triggers the payment email | BROKE `message/missing` |

## The 15 environment scenarios

The site scenarios break the site. These break what's around Echo: the site going down, two CI jobs sharing one Echo, a slow queue, a restart, version skew. A probe sets up each state, runs Echo's commands and checks what they printed, their exit codes and their verdicts.

| # | Scenario | Echo must |
|---|---|---|
| e01 | The site is down at snapshot and at compare | snapshot: exit 3, says "every action failed"; compare: exit 3, verdict error, never "nothing broke", never "all clear" |
| e02 | Two compares run at once against one Echo, one with untagged SMTP mail | each keeps its own mail; the untagged one says "1 message could not be tied", no BROKE |
| e03 | A token-protected Echo on 0.0.0.0, asked from another container | `up` without a token exits 2; the live view and the API answer 401 without the token and 200 with it |
| e04 | The queue sends the receipt 2.5 s after checkout, with quiet at 1 s | says "after the action ended and is counted with it", no BROKE |
| e05 | The PDF invoice is no longer attached | BROKE `message/attachment-lost` "invoice-1042.pdf" |
| e06 | A 2 MB newsletter with a 5 MB PDF through the Laravel connector | the email is captured, never blocked or failed |
| e07 | Echo restarts in the middle of a compare | says "echo restarted during the run" |
| e08 | The sites can't reach Echo while it's sealed | every connector says it can't reach Echo; BROKE `message/missing` for each stack |
| e09 | Order numbers, dates and links change on every run | s10, s11 and s12 report their finding with no CHANGED |
| e10 | Connector and Echo versions differ (1.0.0 against this build, both ways) | mail is captured, no action error |
| e11 | The snapshot was taken by an older Echo | compares with no BROKE and never "incompatible" |
| e12 | Two action names collide | snapshot exits 2, says "both get the ID action:contact-form" |
| e13 | Snapshot and compare on a Windows runner (`cmd /C`) | BROKE `message/missing` "ann@shop.test" |
| e14 | The DNS resolver refuses queries, then never answers | `check` exits 3, says "could not reach the resolver", never slower than 30 s |
| e15 | Right-to-left, CJK and bidi override text | no false `content/mojibake` or placeholder; `\u202e` escaped in the list; no changes |

## Method

For each scenario, the harness in `testbed/bench/`:

1. drains anything an earlier broken run left queued, sets the good state and starts its own Echo in the scenario's mode (sealed, except s16 and s19, whose actions only reach the testbed's CRM and Mailpit);
2. runs `rhythm-echo snapshot --action "<action>" --name good`;
3. breaks the scenario;
4. runs `rhythm-echo compare --action "<action>" --snapshot good --json -o compare.json` and reads the comparison from the envelope's `data` (it passes `--fail-on none` to `snapshot`, `compare` and `check`, so a comparison that finds changes isn't read as a failed command);
5. for the deliverability scenarios, runs `rhythm-echo check <From domain> --provider sendgrid --return-path …` in both states against a fixture DNS server on `127.0.0.1:18870`, which answers only for the testbed's zone and never forwards. It adds the difference to the comparison;
6. passes when the comparison has a change of the expected class whose check and title match the manifest.

Each environment scenario has a probe in `testbed/bench/probes.go`; the ones marked *local* need no Docker.

The actions come from the testbed's own `.echo/config.yaml`: HTTP form posts, WP-CLI and artisan commands with `RHYTHM_ECHO_RUN` passed into the containers, flushes for WP-Cron and the queues and a Stripe replay.

### The other tools

- **Mailpit** (`axllent/mailpit:v1.29.2`) is the usual staging mail catcher. It has no baseline, so it can't report a change; the question is whether a person reading its inbox would have seen the problem. For each state, the harness delivers the emails the site would have handed to an SMTP server into Mailpit, reads them back through its API and counts the scenario as **caught** when the broken-state inbox visibly shows the problem (the scenario's text appears in a From, To, Subject or body but not in the good state). A message that's no longer sent is a **miss**: the inbox simply has one email fewer. SMS, chat, webhooks, analytics and DNS are **can't see**.
- **Buggregator** (`ghcr.io/buggregator/server:2.4.1`) is judged as Mailpit is, for its SMTP inbox. Its HTTP dump and SMS modules only receive what a site is re-pointed to send them, so the other channels are **can't see**.
- **Staging Shield** (1.0.2, free, from wordpress.org) is a WordPress plugin that detects a staging copy and stops its email. It keeps no record and no baseline, so a lost, changed or new message is **can't see**; for the staging scenarios it's **caught** when the email to the real address didn't leave.
- **MessagePit** isn't run: it has no published Docker image or release binary that could be run without building it.

Buggregator and Staging Shield run only when `TOOLS` names them.

## Results

Run on 24 October 2026 with a development build of rhythm-echo, as published with Echo 1.1.1 in `docs/benchmark.md`, on Linux (amd64); the run took 10 min 37 s. Buggregator and Staging Shield weren't part of this run.

| # | Scenario | Echo | Mailpit |
|---|---|---|---|
| s01 | The contact form stops emailing the shop owner | **caught** | missed |
| s02 | The contact form's autoresponder is lost | **caught** | missed |
| s03 | The owner's order SMS is gone after a plugin update | **caught** | can't see |
| s04 | The new-order Slack alert is removed | **caught** | can't see |
| s05 | The GA4 purchase event is missing | **caught** | can't see |
| s06 | Laravel's queued order email is stuck (the job throws) | **caught** | missed |
| s07 | The Next.js welcome email (Resend) now throws | **caught** | missed |
| s08 | The receipt is never sent because cron is off | **caught** | missed |
| s09 | The owner's notification goes to the wrong address | **caught** | **caught** |
| s10 | A merge tag is left unreplaced in the receipt | **caught** | **caught** |
| s11 | A PHP warning is printed into the order email (plain PHP) | **caught** | **caught** |
| s12 | The receipt links to the staging site | **caught** | **caught** |
| s13 | The order SMS is now 3 segments | **caught** | can't see |
| s14 | A field is removed from the order webhook payload | **caught** | can't see |
| s15 | The Zapier hook URL changed | **caught** | can't see |
| s16 | The order webhook to the CRM fails (observe mode) | **caught** | can't see |
| s17 | The From domain changes and breaks DMARC alignment | **caught** | missed |
| s18 | SPF needs more than 10 DNS lookups | **caught** | can't see |
| s19 | A staging refresh brings a real customer address (observe mode) | **caught** | **caught** |
| s20 | A Stripe payment no longer triggers the payment email | **caught** | missed |
| | **Total** | **20 / 20** | 5 / 20 |

| # | Environment scenario | Echo |
|---|---|---|
| e01 | The site is down at snapshot and at compare | **caught** |
| e02 | Two compares run at once against one Echo | **caught** |
| e03 | A token-protected Echo on 0.0.0.0 | **caught** |
| e04 | The queue sends the receipt later than quiet | **caught** |
| e05 | The PDF invoice is no longer attached | **caught** |
| e06 | A large newsletter through the Laravel connector | **caught** |
| e07 | Echo restarts in the middle of a compare | **caught** |
| e08 | The sites can't reach Echo while it is sealed | **caught** |
| e09 | Order numbers, dates and links change on every run | **caught** |
| e10 | Connector and Echo versions differ | **caught** |
| e11 | The snapshot was taken by an older Echo | **caught** |
| e12 | Two action names collide | **caught** |
| e13 | Snapshot and compare on a Windows runner | not run |
| e14 | The DNS resolver is down for the deliverability checks | **caught** |
| e15 | Right-to-left, CJK and bidi override text | **caught** |
| | **Total** | **14 / 15** (1 not run) |

How to read it:

- **Echo** caught all 20 site scenarios as the change they are, through its normal commands: lost messages as BROKE `message/missing`, content problems from the message checks, s14 as a lost payload field (BROKE for a webhook), s16 as a failed send, s17 and s18 from `check` against the fixture DNS. s01 and s02 also report the action's unmet `expect` (`Sent fewer emails than expected`).
- **s07** was the one miss in the first run of the benchmark (19 of 20): `compare` matched emails on their channel and recipients only, so the Resend welcome and the Nodemailer account email to the same new user counted as one message. Emails are now matched by service and recipients, then paired by subject. See [How messages are matched](Echo-Actions-And-Compare#how-messages-are-matched).
- **s15** is reported as the old hook lost and the new one sent: *Zapier webhook to hooks.zapier.com/hooks/catch/1234567/ab••••• is no longer sent* and *…/catch/7654321/zx••••• is now sent*. A catch hook is named by its masked URL, since each URL is its own Zap.
- **e08 and e09** were the last gaps closed before 1.1. In e08 the connectors fail closed and every send fails, which used to read as "the action failed" only; the comparison now also lists each lost message as BROKE and says to check the site reaches Echo. In e09 the order number and date that change on every run no longer add a CHANGED text line next to s10, s11 and s12's findings.
- **e13** needs a Windows machine, which this run didn't have. CI's Windows job runs snapshot and compare in the CLI's tests, not this probe.
- **Mailpit** sees what's *in* an email: the wrong address, the merge tag, the PHP warning, the staging link, the real customer address. It can't see an email that's missing (there's nothing to look at). It never receives SMS, chat, webhooks, analytics or DNS.

The per-scenario details, including everything else Echo reported, are in [`docs/benchmark.md`](https://github.com/RhythmShahriar/echo/blob/master/docs/benchmark.md) in the Echo repository.

> [!NOTE]
> We wrote both Echo and the testbed, so read this as a regression suite with a scoreboard, not an independent review: every scenario is something Echo was built to catch. Each one is still a real failure mode, in a real WordPress, Laravel or Next.js app. It passes only when Echo reports the expected change through its normal commands. The scenarios, the expectations and the harness are open. Rerun it and add scenarios where you think it falls short.

## Consent checks

Echo 1.2's cookie consent checks have a testbed of their own: 40 small shops, each a few pages with a cookie banner and the tags a real shop loads. 25 are broken in one way each (GA4 before consent, a Meta Pixel that ignores Consent Mode, Reject reported as granted, a choice not kept, a server-side event after Reject), 15 are correct setups that must pass with no error and no warning, lookalikes included. Every tracker and CMP host is served by the testbed itself and a sink counts any hit that gets through.

Go mode, Chrome 153, 16 CPUs, 2026-10-28:

| Measure | Result | Target |
|---|---|---|
| Broken scenarios caught | 25 of 25 | 25 of 25 |
| False alarms on correct sites | 0 in 400 checks | 0 |
| Flakes (10 rounds of every correct state) | 0 | 0 |
| Accept and Reject clicks verified | 880 of 880 (100%) | 100% |
| Tracker hits that reached the sink (held mode) | 0 | 0 |
| Time per page | 15.0 s p50, 25.6 s p95 | under 25 s and 45 s |
| `consent detect`, slowest | 4.5 s | under 15 s |
| 50 sites, `--parallel 6` | 5.6 min | under 15 min |
| Echo's peak memory (Chrome excluded) | 116.5 MiB | under 200 MB |

Environment probes: 17 of 18 caught (a site that's down, no Chrome, a bot wall, Ctrl-C mid-run, every CPU busy, a request storm, hostile text in the banner, secrets in the URL and others); e16, private addresses on the page, needs the sealed Docker testbed and wasn't run.

How to read it: Cookiebot, OneTrust, Usercentrics, Didomi, iubenda and Borlabs (and Complianz and CookieYes in Go mode) are replicas of public markup captured on 2026-10-26, not the vendors' code; Klaro, orestbida cookieconsent and, in Docker mode, the Complianz and CookieYes WordPress plugins are the real thing. Google's and Meta's tags are stand-ins written to community-documented behaviour. We wrote Echo and the testbed, so read it as a regression suite with a scoreboard. The sealed Docker run (the benchmark of record, with the binary size check and `make reproduce`) and the reference columns for DuckDuckGo autoconsent and the EDPS Website Evidence Collector weren't part of this run. The full tables are in [`docs/benchmark-consent.md`](https://github.com/RhythmShahriar/echo/blob/master/docs/benchmark-consent.md); the scenarios are in the [testbed README](https://github.com/RhythmShahriar/echo/blob/master/testbed/consent/README.md).

```sh
make build
go run -tags testbed ./testbed/consent/bench -bin bin/rhythm-echo     # Go mode, no Docker
make -C testbed/consent up && make bench-consent                      # sealed Docker mode
make -C testbed/consent reproduce                                    # two full runs, compared
```

## Run it yourself

You need Docker with Compose v2 and Go.

```sh
git clone https://github.com/RhythmShahriar/echo && cd echo
make -C testbed up TOOLS="echo mailpit buggregator staging-shield"          # the sites on :18850-18858
make -C testbed benchmark TOOLS="echo mailpit buggregator staging-shield"   # every scenario; writes docs/benchmark.md
make -C testbed reproduce TOOLS="echo mailpit buggregator staging-shield"   # twice; fails if any outcome differs
go run -tags testbed ./testbed/bench -only s05,e05                           # some scenarios
go run -tags testbed ./testbed/bench -only env -tools none                   # the environment scenarios; the local ones need no Docker
make -C testbed down                                                         # throw everything away
```

The first `up` takes a few minutes, later ones about 30 s. The benchmark starts its own Echo on `0.0.0.0:18860`–`18862` with the token `make up` wrote (the containers reach it through the testbed's gate) and a fixture DNS server on `127.0.0.1:18870` for the deliverability scenarios. The local environment probes run their own Echo and fixtures on `127.0.0.1:23200`–`23239`. The version skew scenario builds Echo v1.0.0 from its git tag, so its first run needs the Go module cache or the network. It keeps each run in `testbed/bench/work/<id>/`; `work/results.json` has every result. `-tools none` skips the other tools, `-bin` benchmarks another binary. The [testbed README](https://github.com/RhythmShahriar/echo/blob/master/testbed/README.md) has the details, including how to add a scenario. The testbed is also Echo's end-to-end test suite; see [Contributing](Contributing#echo-and-its-connectors).

---
<sub>← [FAQ and troubleshooting](Echo-FAQ) · [Home](Home) · [Roadie overview](Roadie) →</sub>
