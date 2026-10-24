How well does Echo catch what goes wrong with what real sites send, and how does it compare with a mail catcher? This page describes the benchmark: a testbed of 20 broken scenarios on WordPress, Laravel, Next.js and plain PHP, how each one is run and judged, and the results.

## The testbed

The testbed lives in the Echo repository, in [`testbed/`](https://github.com/RhythmShahriar/echo/tree/master/testbed). It runs in Docker Compose, on ports bound to `127.0.0.1`:

| Site | Address | What it is |
|---|---|---|
| WordPress | `http://localhost:18850` | WordPress 6.8.3 on PHP 8.3 with MariaDB 11.4: "Bean & Leaf", a coffee shop, with fixture plugins for a contact form, a WooCommerce stand-in (a receipt through WP-Cron, Twilio SMS, Slack, GA4, Meta CAPI, webhooks through an Action Scheduler stand-in), an SMS add-on whose Twilio client is raw cURL, and a Stripe webhook endpoint |
| Laravel | `http://localhost:18851` | A stock Laravel app with `rhythmsh/echo-laravel`: an order queues a Mailable and calls Twilio with the `Http` client |
| Next.js | `http://localhost:18852` | An app with `@rhythmsh/echo`: a signup route (a Resend-style `fetch`, Twilio `fetch`, Nodemailer) and an Edge route (Slack) |
| Plain PHP | `http://localhost:18853` | A legacy order page that sends with `mail()`, through `sendmail_path` = `rhythm-echo sendmail` |
| Mailpit | `http://localhost:18854` | The shop's "real" mail server in observe mode, and the comparison tool |

The sites sit on a Docker network with `internal: true`, so nothing in the stack can reach the internet, whatever a connector or Echo decides: it's the [hard seal](Echo-Sealing-Staging#docker-internal-network), with Echo on the host. Every connector is installed from the checkout (the mu-plugin with `rhythm echo install wordpress`), and every run starts from the same seeded WordPress.

Each scenario has a **good** state and a **broken** state. `testbed/toggle.sh s05 broken` breaks one by creating `testbed/state/s05`, which the fixtures check on every request, job and command: no rebuild, no restart.

## The 20 scenarios

The manifest, `testbed/scenarios.yaml`, says what each scenario breaks, where, which Echo action shows it, and what counts as caught.

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
| s14 | Wrong content | `order.created` no longer carries `billing.email` | CHANGED `message/fields-lost` |
| s15 | Integrations | the lead goes to a test Zap's catch hook | BROKE `message/missing` |
| s16 | Integrations | the CRM answers 500 to the order webhook (observe mode); five failures disable it | BROKE `message/failing` or `send/failed` |
| s17 | Deliverability | the autoresponder is sent from a newsletter domain SendGrid isn't set up for | BROKE `dns/dmarc` |
| s18 | Deliverability | three more includes push SPF to 11 lookups | BROKE `dns/spf` |
| s19 | Staging safety | a staging refresh brings a real customer address, with Echo in observe mode | NEW `seal/real-recipient` |
| s20 | Inbound | a Stripe `payment_intent.succeeded` replay no longer triggers the payment email | BROKE `message/missing` |

## Method

For each scenario, the harness in `testbed/bench/`:

1. drains anything an earlier broken run left queued, sets the good state, and starts its own Echo in the scenario's mode (sealed, except s16 and s19, whose actions only reach the testbed's CRM and Mailpit);
2. runs `rhythm-echo snapshot --action "<action>" --name good`;
3. breaks the scenario;
4. runs `rhythm-echo compare --action "<action>" --snapshot good`;
5. for the deliverability scenarios, runs `rhythm-echo check <From domain> --provider sendgrid --return-path …` in both states against a fixture DNS server on `127.0.0.1:18870`, which answers only for the testbed's zone and never forwards, and adds the difference to the comparison;
6. passes when the comparison has a change of the expected class whose check and title match the manifest.

The actions come from the testbed's own `.echo/config.yaml`: HTTP form posts, WP-CLI and artisan commands with `RHYTHM_ECHO_RUN` passed into the containers, flushes for WP-Cron and the queues, and a Stripe replay.

### The other tools

- **Mailpit** (`axllent/mailpit:v1.29.2`) is the usual staging mail catcher. It has no baseline, so it can't report a change; the question is whether a person reading its inbox would have seen the problem. For each state, the harness delivers the emails the site would have handed to an SMTP server into Mailpit, reads them back through its API, and counts the scenario as **caught** when the broken-state inbox visibly shows the problem (the scenario's text appears in a From, To, Subject or body, and didn't in the good state). A message that's no longer sent is a **miss**: the inbox simply has one email fewer. SMS, chat, webhooks, analytics and DNS are **can't see**.
- **MessagePit** isn't run: it has no published Docker image or release binary that could be run without building it.

## Results

Run on 24 October 2026 with a development build of rhythm-echo on Linux (amd64); the run took 4 min 53 s.

| # | Scenario | Echo | Mailpit |
|---|---|---|---|
| s01 | The contact form stops emailing the shop owner | **caught** | missed |
| s02 | The contact form's autoresponder is lost | **caught** | missed |
| s03 | The owner's order SMS is gone after a plugin update | **caught** | can't see |
| s04 | The new-order Slack alert is removed | **caught** | can't see |
| s05 | The GA4 purchase event is missing | **caught** | can't see |
| s06 | Laravel's queued order email is stuck (the job throws) | **caught** | missed |
| s07 | The Next.js welcome email (Resend) now throws | missed (known issue) | missed |
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
| | **Total** | **19 / 20** | 5 / 20 |

How to read it:

- **Echo** caught 19 scenarios as the change they are, through its normal commands: lost messages as BROKE `message/missing`, content problems from the message checks, s14 as a lost payload field, s16 as a failed send, s17 and s18 from `check` against the fixture DNS.
- **s07 is a known Echo issue.** `compare` matches emails on their channel and recipients only, so the Resend welcome and the Nodemailer account email to the same new user count as one message. When the welcome is lost, Echo reports *Resend email to the test submitter is sent fewer times* (CHANGED) and diffs the welcome against the account email, instead of BROKE for the welcome. See [How messages are matched](Echo-Actions-And-Compare#how-messages-are-matched).
- **Mailpit** sees what's *in* an email: the wrong address, the merge tag, the PHP warning, the staging link, the real customer address. It can't see an email that's missing (there's nothing to look at), and it never receives SMS, chat, webhooks, analytics or DNS.

The per-scenario details, including everything else Echo reported, are in [`docs/benchmark.md`](https://github.com/RhythmShahriar/echo/blob/master/docs/benchmark.md) in the Echo repository.

> [!NOTE]
> We wrote both Echo and the testbed, so read this as a regression suite with a scoreboard, not an independent review: every scenario is something Echo was built to catch. Each one is still a real failure mode, in a real WordPress, Laravel or Next.js app, and it passes only when Echo reports the expected change through its normal commands. The scenarios, the expectations and the harness are open; rerun it, and add scenarios where you think it falls short.

## Run it yourself

You need Docker with Compose v2 and Go.

```sh
make build                                           # bin/rhythm-echo
make -C testbed up                                   # a few minutes the first time, then about 30 s
go run -tags testbed ./testbed/bench                 # every scenario, Echo and Mailpit; writes docs/benchmark.md
go run -tags testbed ./testbed/bench -only s05,s09   # some scenarios
make -C testbed down                                 # throw everything away
```

The benchmark starts its own Echo on `0.0.0.0:18860`–`18862` with the testbed's token (the containers reach it through the testbed's gate), and keeps each run in `testbed/bench/work/<id>/`. `-tools none` skips Mailpit, `-bin` benchmarks another binary. The [testbed README](https://github.com/RhythmShahriar/echo/blob/master/testbed/README.md) has the details, and how to add a scenario. The testbed is also Echo's end-to-end test suite; see [Contributing](Contributing#echo-and-its-connectors).

---
<sub>← [FAQ and troubleshooting](Echo-FAQ) · [Home](Home) · [Responsible use](Responsible-Use) →</sub>
