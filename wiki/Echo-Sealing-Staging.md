A staging copy of a live site still has the live site's mailer, SMS gateway, webhooks and API keys. This page explains how Echo seals it so nothing reaches real people: the two modes, what Echo answers for each service, why live payments are refused, the allowlist, what passes through on its own, what happens when Echo is down and the hard seals that close the gaps a connector can't.

## Seal and observe

| Mode | Start it with | Email | HTTP to a service | Use it for |
|---|---|---|---|---|
| **seal** (default) | `rhythm echo up` or `--seal` | recorded, never delivered | recorded and answered by Echo with a response shaped like the provider's | every staging and local site |
| **observe** | `rhythm echo up --observe` | recorded, then sent the normal way by a connector (mail to Echo's SMTP sink or `sendmail` is still only kept) | recorded, sent for real and the response recorded | a site where real delivery is safe, such as one that only mails your own team |

`mode: observe` in `.echo/config.yaml` does the same as the flag; `--seal` and `--observe` together are a usage error. The catcher's mode decides what happens to every message: a connector's own `mode` setting only matters when it [can't reach Echo](#fail-closed).

> [!WARNING]
> Observe mode really sends: real customers get the email and the SMS. Use it only where you'd be happy for every message to go out. Add your test addresses to `allow_recipients` so anything else is flagged. Replays refuse to run in observe mode. Live payments are refused even here; see [Live payments are refused](#live-payments-are-refused).

In observe mode, Echo's SMTP sink and the `sendmail` stand-in still never deliver mail: only connectors send for real. Mail that came in through them is recorded as `faked` in every mode, so it never counts as really sent.

## What happens to each message

Every capture ends with one of four statuses, shown in `watch`, `ls` and the reports:

| Status | Means |
|---|---|
| `faked` | Sealed: Echo answered instead of the real service. The site saw a normal success. |
| `blocked` | Refused: a live-mode payment call (in every mode), a blind tunnel to a payment API, sealed HTTPS the proxy can't decrypt without `--mitm`, or a release the relay doesn't allow. The site saw a failure. |
| `sent` | Delivered for real by a connector: observe mode, or a host on the allowlist. |
| `failed` | The site tried and the send failed: a transport error, or an HTTP status of 400 or more. |

For an HTTP request, Echo first recognizes what it is (a Twilio SMS, a Slack alert, a GA4 event, a Stripe call; see [Messages and checks](Echo-Messages-And-Checks#recognized-services)), then decides:

```mermaid
flowchart TD
  R["Outbound request"] --> I{"Platform update check?<br/>api.wordpress.org…"}
  I -- yes --> T["Let through, not recorded"]
  I -- no --> P{"Live-mode payment,<br/>or a key Echo can't read?"}
  P -- yes --> B["Answer with the provider's<br/>own refusal: blocked"]
  P -- no --> O{"Observe mode, or<br/>allowed host?"}
  O -- yes --> S["Send for real, record as sent"]
  O -- no --> F["Answer with a fake<br/>success: faked"]
```

## Fakes per provider

A fake has to look enough like the real answer that the site's code carries on. WooCommerce disables a webhook after five failed deliveries and SDKs throw on unexpected bodies, so a bare `200 OK` isn't always enough. The answers Echo gives while sealed:

| Service | Echo answers |
|---|---|
| SendGrid | `202`, with an `X-Message-Id` header |
| Gmail API | `200`, `{"id": …, "threadId": …, "labelIds": ["SENT"]}` |
| Microsoft Graph `sendMail` | `202`, no body |
| Mailjet | `200`, `{"Messages": [{"Status": "success", …}]}` |
| Mailgun | `200`, `{"id": "<…@…>", "message": "Queued. Thank you."}` shape |
| Postmark | `200`, a `MessageID` per message (one object, or an array for batches) |
| Resend | `200`, `{"id": …}`, or `{"data": [...]}` for batches |
| Amazon SES | `200`, a `MessageId` |
| Brevo | `201`, `{"messageId": "<…@smtp-relay.mailin.fr>"}` |
| SparkPost, Mandrill | `200`, their accepted-recipient results |
| Twilio | `201`, a message object (`sid`, `status: queued`, `to`, `from`, `body`, `num_segments`) |
| Vonage | `200` with a `messages` array (SMS API), or `202` with a `message_uuid` (Messages API) |
| MessageBird, Bird, Plivo, Telnyx, ClickSend, Amazon SNS | each provider's own accepted or queued shape |
| Sinch, Infobip, Bandwidth | `201` with a batch, `200` with `PENDING_ACCEPTED` messages, `202` with a message object |
| WhatsApp Cloud | `200`, `{"messaging_product": "whatsapp", "contacts": …, "messages": [{"id": …}]}` |
| Slack | `200`, `ok` |
| Discord | `204` |
| Telegram | `200`, `{"ok": true, "result": {"message_id": …, "chat": …, "text": …}}` |
| Microsoft Teams | `202` for Power Automate workflow webhooks, `200` with `1` for classic incoming webhooks |
| GA4 Measurement Protocol | `204`, or `{"validationMessages": []}` from the debug endpoint |
| Meta Conversions API | `200`, `{"events_received": n, "messages": [], "fbtrace_id": …}` |
| Segment, Mixpanel, PostHog | `200`: `{"success": true}`, `1`, `{"status": 1}` |
| FCM | `200`, a `projects/…/messages/…` name |
| OneSignal | `200`, an `id` and the recipient count |
| APNs | `200`, no body, an `apns-id` header |
| Expo Push | `200`, `{"data": [{"status": "ok", "id": …}]}` |
| HubSpot, Mailchimp, Klaviyo, ActiveCampaign | each API's success shape for the call (a created contact, an empty list…) |
| Zapier | `200`, `{"status": "success", "attempt": …, "id": …, "request_id": …}` |
| Make | `200`, `Accepted` |
| n8n | `200`, `{"message": "Workflow was started"}` |
| IFTTT | `200`, `Congratulations! You've fired the <event> event` |
| Stripe (test key) | a plausible object for the endpoint, with `livemode: false`; see below |
| PayPal (sandbox host) | an order or capture shaped like the sandbox's, with `approve` and `capture` links |
| Adyen, Braintree, Square, Authorize.net (test hosts), Mollie, Razorpay (test keys) | each gateway's success shape for the call |
| anything else | `200`, `{"ok": true}` |

The site gets the fake; Echo records it as the message's `response`, so `rhythm echo show <id>` shows both what the site sent and what it was told.

## Live payments are refused

A fake success is only safe when the real call couldn't have moved money. A real call is only safe in test mode. So for payment APIs Echo looks at the mode first, before the seal and the allowlist. A live call is refused in **every** mode: sealed or observed, allowlisted or not.

| Gateway | Live is told by | Test calls |
|---|---|---|
| Stripe | the key: `sk_live_…`, `rk_live_…`, or a key Echo can't read | `sk_test_…`, `rk_test_…` get a fake success with `livemode: false` |
| PayPal | the host: `api-m.paypal.com`, `api.paypal.com` | `api-m.sandbox.paypal.com`, `api.sandbox.paypal.com` |
| Adyen | the host: `*-live.adyenpayments.com`, `*-live.adyen.com` | `*-test.adyen.com` |
| Braintree | the host: `api.braintreegateway.com`, `payments.braintree-api.com` | the `sandbox` hosts |
| Square | the host: `connect.squareup.com` | `connect.squareupsandbox.com` |
| Authorize.net | the host: `api.authorize.net`, `api2.authorize.net` | `apitest.authorize.net` |
| Mollie | the key: anything but a `test_` key (or an `access_` token with `testmode`) | `test_…` keys |
| Razorpay | the key: anything but an `rzp_test_` key ID | `rzp_test_…` keys |

A refused call gets the gateway's own error shape with `403` (`401` for Razorpay); the message is `blocked`. Its error says why: `refused: a live-mode Stripe call; use test credentials on staging`. The SDK then throws and the site records a failed payment. A staging copy of production holding live keys never charges a card and never sees "paid" for an order nobody paid for:

```sh
curl -s http://127.0.0.1:8025/v1/capture -H 'Content-Type: application/json' -d '{"v":2,"kind":"http","phase":"before","source":{"via":"my-connector"},
  "http":{"method":"POST","url":"https://api.stripe.com/v1/payment_intents","headers":{"Authorization":"Bearer sk_live_…"},"body":"amount=2400&currency=eur"}}'
```

```text
{"id":"23bb60c909ae","mode":"seal","action":"fake","response":{"status":403,"headers":{"content-type":"application/json"},"body":"{\"error\":{\"code\":\"rhythm_echo_sealed\",\"message\":\"Rhythm Echo refused a live-mode call. Use a Stripe test key (sk_test_…) on staging.\",\"type\":\"invalid_request_error\"}}"}}
```

The same for a test key and for Adyen's live endpoint, as `rhythm echo ls --channel payment` lists them:

```text
23bb60c909ae  14:36:09 Stripe call to api.stripe.com: live mode [blocked]
502758beb1e5  14:36:09 Stripe call to api.stripe.com: test mode [faked]
7cf14f21d822  14:36:09 Adyen call to checkout-live.adyenpayments.com: live mode [blocked]
```

Adyen's refusal reads `Rhythm Echo refused a live Adyen call. Use the test endpoints (*-test.adyen.com) on staging.`; each gateway's names its own sandbox. `rhythm echo doctor` lists the gateways the running Echo refuses: `Payments: live calls refused for Stripe, PayPal, Adyen, Braintree, Square, Authorize.net, Mollie and Razorpay, even in observe mode`.

Through the proxy, Echo can only check a key when it decrypts the request. So a `CONNECT` to a payment API is never tunnelled blind: without `--mitm` it's refused in every mode (even for an allowlisted host) and recorded as `blocked`. A live gateway host says `a live <Name> API is never reached from staging; use the <Name> sandbox`; Stripe, Mollie and Razorpay, whose one host serves both modes, say `payment APIs need --mitm so Echo can tell test from live keys`. With `--mitm`, every gateway host is decrypted (even when allowlisted) and the same rules apply. Sandbox hosts can still be tunnelled, since they can't move money.

> [!NOTE]
> Allowlisting `api.stripe.com` lets **test-mode** calls really reach Stripe, for an end-to-end checkout against Stripe's test environment. Live-mode calls are still refused.

## The allowlist

Some calls should really go out even when sealed: a payment provider's test mode for an end-to-end checkout, a search API the pages need. List those hosts:

```sh
rhythm echo up --allow api.stripe.com --allow '*.sandbox.paypal.com'
```

```yaml
# .echo/config.yaml
allow: ["api.stripe.com", "*.sandbox.paypal.com"]
```

`--allow` adds to the config's list, takes comma-separated values and can be repeated. Entries are host globs (`*` matches within a name, so `*.sandbox.paypal.com` matches `api-m.sandbox.paypal.com`). Allowed requests are still recorded, as `sent`, with their responses. Live payment calls are refused whatever the allowlist says. `up` lists them on its `Allowed` line.

Always allowed, without listing:

- the site's own host, so cron and Action Scheduler loopbacks keep working, unless it's a provider's domain, a public suffix (`co.uk`, `github.io`) or a request Echo recognizes as a provider's (a WooCommerce webhook to the site itself), so a connector's reported host can never unseal a service;
- loopback addresses and `localhost`;
- through the proxy, also private address ranges and single-label names (`db`, `redis`) that resolve only to loopback or private addresses, since the proxy can't tell which host is the site.

Never allowed while sealed, whatever the allowlist says: the cloud metadata and credential endpoints (all of `169.254.0.0/16`, `100.100.100.200`, `fd00:ec2::254`, `fd00:ec2::23`, `metadata.google.internal`, `metadata.goog`, `instance-data.ec2.internal`). Plain HTTP to them is faked with `404` and a tunnel refused with `403`; a name that resolves there is refused by the proxy with `refused: the name resolves to a cloud metadata endpoint, which a sealed site never reaches`.

**`allow_recipients`** is the observe-mode counterpart: addresses or numbers you may really message, as globs (`*@yourcompany.com`, `+4915112345678`). In observe mode, a message a connector really sent to anyone else gets the warning *Email really sent to an address outside the allowlist*. Test addresses never count as real: `.test`, `.local`, `.localhost`, `.invalid` and `.example` domains, `example.com/.org/.net`, plus `+1555…` and `+15005550…` numbers.

## What passes through

Platform housekeeping isn't something the site *says* to anyone. Recording it would add noise to every action. These hosts pass through even when sealed and aren't recorded: `api.wordpress.org`, `downloads.wordpress.org`, `wordpress.org`, `s.w.org`, `planet.wordpress.org` and `repo.packagist.org`. WordPress update checks, plugin and theme update screens and the Events widget keep working.

## Fail closed

If a connector can't reach Echo while sealed, it refuses to send rather than letting the message out:

| Connector | Mail | HTTP | Where it says so |
|---|---|---|---|
| WordPress | `wp_mail()` returns `false`, `wp_mail_failed` fires | `WP_Error`: `Rhythm Echo could not be reached and this site is sealed, so the request was not sent.` | the PHP error log |
| Laravel | `TransportException` ending `; sealed, so the mail was not sent. Check the site reaches Echo: php artisan echo:status, or rhythm echo doctor` | `ConnectionException`, the same with `the request` | the Laravel log |
| Next.js, Node | Nodemailer rejects | a network error | one warning on the console |
| `rhythm-echo sendmail` | exit `69`, so `mail()` returns `false` | | stderr |

A connector set to `mode: observe` lets traffic through instead when Echo is down. The Node connector assumes sealed until Echo has said otherwise. A comparison makes a site that can't reach Echo plain: the action fails and every message from the connector is reported missing, with `A connector that can't reach Echo blocks every send while sealed: check the site reaches Echo with rhythm echo doctor`.

## Hard seals

A connector only sees what goes through the hooks it patches. An SDK that opens its own socket, a plugin that calls `fsockopen`, a cron job in another language: all of these go around it. A **hard seal** closes that gap at the network: the site has no route to the internet at all. Its only way out is Echo (the proxy on `:8026`, the SMTP sink on `:1025`, the capture API on `:8025`). Anything that doesn't go through Echo fails instead of leaking.

```text
 site ──► Echo proxy :8026 ──► recorded; faked (seal) or forwarded (observe, --allow)
      ──► Echo SMTP  :1025 ──► recorded, never delivered
      ──► anything else    ──► refused by the network
```

The recipes live in the Echo repository under [`docs/seal`](https://github.com/RhythmShahriar/echo/tree/master/docs/seal), with tested example files:

| Where the site runs | Recipe | Tested |
|---|---|---|
| Docker Compose | an `internal: true` network, with Echo the only service on both networks | yes: `go test -tags docker -run ComposeSeal ./proxy` |
| A Linux VM or server | nftables or iptables egress deny, where only Echo's user may connect out | yes, in a network namespace: `go test -run Firewall ./proxy` |
| php-fpm, WordPress | `env[]` lines, `sendmail_path` and CA trust, so PHP's traffic goes through Echo | partly |

The network recipes enforce the seal; the php-fpm settings route traffic through Echo so it's recorded and faked instead of simply refused. Use both.

### Docker internal network

The app and its database sit on a network with `internal: true`, which Docker gives no gateway. Echo sits on that network and on a normal one, so it's the only way out; a small nginx `front` on both networks is the way in, since services on an internal network can't publish ports:

```sh
export RHYTHM_ECHO_TOKEN=$(openssl rand -hex 16)
docker compose -f docs/seal/examples/docker-compose.yml up -d
```

Echo runs from its image (`ghcr.io/rhythmshahriar/echo:1`; set `RHYTHM_ECHO_IMAGE` to pin a release) as `rhythm-echo up --seal --mitm` on `0.0.0.0` inside its container, so it requires the token, which it reads from `RHYTHM_ECHO_TOKEN`. Its CA and key live in the `echo-state` volume at `/home/nonroot/.config/rhythm/echo`, so trust survives restarts. See [Docker and DDEV](Echo-Docker-And-DDEV) for the image. A one-shot `echo-ca` job copies the CA certificate (never the key) from `http://echo:8026/ca.pem` to a volume the app mounts read-only. The app gets `HTTPS_PROXY` with the token as the proxy password, `RHYTHM_ECHO_URL`, `RHYTHM_ECHO_SMTP` and `RHYTHM_ECHO_TOKEN` for the connectors, plus `NODE_EXTRA_CA_CERTS`. Check it:

```sh
p="docker compose -f docs/seal/examples/docker-compose.yml"
$p exec app sh -c 'env -u https_proxy -u http_proxy curl -m 3 https://api.twilio.com/'   # no route: fails at once
$p exec app curl -sS --cacert /echo-ca/ca.pem -d 'To=%2B15551230000&Body=hi' \
  https://api.twilio.com/2010-04-01/Accounts/AC123/Messages.json                          # through Echo: faked
```

### Linux firewall

On a VM or server, Echo runs as its own user. The firewall lets only that user open connections to the internet. Every other process reaches only loopback, private networks and the DNS resolvers; anything else gets a TCP reset at once, logged with the uid (`journalctl -k | grep echo-seal`):

```sh
sudo useradd --system --create-home --home-dir /var/lib/rhythm-echo rhythm-echo
sudo -u rhythm-echo rhythm-echo up --seal --proxy 127.0.0.1:8026 --mitm
sudo nft -f docs/seal/examples/nftables-echo.nft
# or: sudo ECHO_USER=rhythm-echo RESOLVERS_V4=127.0.0.53 sh docs/seal/examples/iptables-echo.sh
```

Unseal with `sudo nft delete table inet echo_seal` or `sudo sh iptables-echo.sh off`. By default root may not connect out either, so `apt` can't update while sealed; the example files show the line to uncomment. Docker containers on the VM go through `FORWARD`, not `OUTPUT`: use the Docker recipe for them.

### php-fpm

php-fpm starts its workers with an empty environment (`clear_env = yes`), so `HTTPS_PROXY` set on the container or the systemd unit never reaches PHP. Set it per pool, with mail and CA trust:

```ini
[www]
env[https_proxy] = http://echo:ECHO_TOKEN@127.0.0.1:8026
env[HTTPS_PROXY] = http://echo:ECHO_TOKEN@127.0.0.1:8026
env[http_proxy]  = http://echo:ECHO_TOKEN@127.0.0.1:8026
env[no_proxy]    = localhost,127.0.0.1,::1
env[RHYTHM_ECHO_URL]   = http://127.0.0.1:8025
env[RHYTHM_ECHO_SMTP]  = 127.0.0.1:1025
env[RHYTHM_ECHO_TOKEN] = ECHO_TOKEN
php_admin_value[sendmail_path]  = "/usr/local/bin/rhythm-echo sendmail -t -i"
php_admin_value[curl.cainfo]    = /etc/rhythm-echo/bundle.pem
php_admin_value[openssl.cafile] = /etc/rhythm-echo/bundle.pem
```

Drop `echo:ECHO_TOKEN@` when Echo listens on loopback without a token. Uppercase `HTTP_PROXY` is left out on purpose: under CGI a request header can set it (httpoxy), so libcurl never reads it. Apache with mod_php and the PHP CLI inherit the process environment, so there `HTTPS_PROXY` on the container is enough. The full pool file is [`php-fpm-echo.conf`](https://github.com/RhythmShahriar/echo/blob/master/docs/seal/examples/php-fpm-echo.conf).

## Checking the seal

- `rhythm echo doctor` sends canaries through each layer it can reach (SMTP, the WordPress connector, the Laravel connector) and checks they arrive. See [CLI reference](Echo-CLI-Reference#doctor).
- `rhythm echo watch` while you use the site: anything with a `sent` status went out for real.
- After a hard seal, try a request without the proxy from inside the app (as above): it must fail.

---
<sub>← [Connectors](Echo-Install-Connectors) · [Home](Home) · [Echo Inbox](Echo-Inbox) →</sub>
