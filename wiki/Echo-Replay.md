Many of a site's messages are triggered by a webhook coming **in**: Stripe says a payment succeeded, and the shop sends the order email, an SMS and a CRM update. `rhythm echo replay` sends a signed test webhook to your staging site and shows what the site sends in response. This page covers the event library, your own payloads, overriding fields, the signing schemes and their secrets, the safety rules, and why PayPal can't be replayed.

```sh
export ECHO_STRIPE_WEBHOOK_SECRET=whsec_…   # the staging endpoint's test signing secret
rhythm echo replay payment_intent.succeeded --to /webhooks/stripe --set data.object.receipt_email=jamie@shop.test
```

```text
rhythm echo 1.0.0 - https://staging.shop.test/webhooks/stripe

Event: stripe/payment_intent.succeeded (stripe)
Response: 200 in 15ms
08:23:49 email to jamie@shop.test: Your order is confirmed [faked]
08:23:49 email to office@shop.test: New order paid [faked]

  The site answered 200 and sent 2 emails in response.
```

The header shows the full URL the replay went to, with any secret in it masked. The output is from a local test shop whose Stripe endpoint is `/webhooks/stripe`; for WooCommerce's Stripe gateway it's `/?wc-api=wc_stripe`. Replay needs Echo running and **sealed**, so whatever the site sends back is caught; `--to` is absolute, or relative to `site:` in `.echo/config.yaml`.

## The event library

```sh
rhythm echo replay --list
```

```text
clerk/user.created
github/ping
paypal/PAYMENT.CAPTURE.COMPLETED
resend/email.bounced
resend/email.delivered
stripe/charge.refunded
stripe/checkout.session.completed
stripe/customer.subscription.created
stripe/customer.subscription.deleted
stripe/customer.subscription.updated
stripe/invoice.paid
stripe/invoice.payment_failed
stripe/payment_intent.payment_failed
stripe/payment_intent.succeeded
woocommerce/order.created
```

Name an event in full (`stripe/invoice.paid`) or by its type alone when that's unique (`invoice.paid`). Stripe events get a fresh `evt_test_…` ID and `created` time on every replay, so a receiver that de-duplicates on the event ID processes each one. An unknown name lists the library: `unknown event "foo" (try one of: clerk/user.created, …)`.

## Your own payloads

Any JSON file works too. Say how to sign it with `--scheme`, and give its event type with `--type` when the payload has no top-level `"type"`:

```sh
ECHO_WEBHOOK_SECRET=… rhythm echo replay lead.json --scheme hmac --to /hooks/lead -H "X-Source: crm"
```

The scheme defaults to `hmac`. `-H "Name: value"` (repeatable) adds headers, but can't replace the signature headers.

## Overriding fields

`--set path=value` changes one field before signing, and can be repeated; overrides apply in order:

```sh
rhythm echo replay payment_intent.succeeded --to /?wc-api=wc_stripe \
  --set data.object.amount=2400 --set data.object.receipt_email=jamie@shop.test \
  --set data.object.metadata.order_id='"1042"'
```

- Paths are dot-separated keys, with numbers indexing arrays: `line_items.0.quantity`. Missing objects are created, and an index one past the end appends.
- The value is JSON when it parses as JSON (`2400`, `true`, `null`, `{"a":1}`), and a string otherwise, so `jamie@shop.test` needs no quotes and `'"1042"'` keeps a number-like value a string.

## Signing schemes

| Scheme | Used for | Headers sent | Secret, from | Secret must be |
|---|---|---|---|---|
| `stripe` | Stripe | `Stripe-Signature: t=<time>,v1=<HMAC-SHA256 of "<time>.<body>">` | `ECHO_STRIPE_WEBHOOK_SECRET` | a `whsec_…` signing secret |
| `standard` | [Standard Webhooks](https://www.standardwebhooks.com) and Svix senders: Resend, Clerk | `webhook-id`, `webhook-timestamp`, `webhook-signature: v1,<base64>`, and the same as `svix-*` | `ECHO_WEBHOOK_SECRET` | a `whsec_<base64>` key |
| `github` | GitHub | `X-Hub-Signature-256: sha256=…`, `X-GitHub-Event`, `X-GitHub-Delivery` | `ECHO_GITHUB_WEBHOOK_SECRET` | any |
| `woocommerce` | WooCommerce webhooks, for receivers of them | `X-WC-Webhook-Signature` (base64 HMAC-SHA256 of the body), `X-WC-Webhook-Topic`, `-Resource`, `-Event`, `-ID`, `-Delivery-ID`, `-Source` | `ECHO_WC_WEBHOOK_SECRET` | any |
| `hmac` | anything else | `X-Signature: <hex HMAC-SHA256 of the body>` | `ECHO_WEBHOOK_SECRET` | any |
| `paypal` | | refused; see [PayPal](#paypal) | | |

Every replay also sends `Content-Type: application/json`, a user agent like the provider's, and `X-Echo-Run`, which ties what the site sends to this replay. Library events are signed with their directory's scheme; `--scheme` is for your own files.

The `hmac` scheme always sends `X-Signature` with a hex HMAC-SHA256 of the body; its header name, algorithm and encoding can't be changed from the command line or the config. For a receiver that verifies another way, write a small test of your own, or use the provider's own test tooling.

## Secrets

The signing secret comes from the environment, never from a flag, so it isn't in your shell history or the config. `--secret-env NAME` (or `secret_env:` in a replay action) reads another variable. Use the **test** endpoint's secret: for Stripe, the signing secret of the staging endpoint in test mode, or the one `stripe listen` prints.

```text
rhythm echo: no signing secret: set ECHO_STRIPE_WEBHOOK_SECRET to the endpoint's test signing secret
```

That's a usage error (exit `2`), printed after the replay's header and `Event:` line.

Results show the secret only as `ECHO_STRIPE_WEBHOOK_SECRET=•••••`, and signatures by their first characters.

## Safety rules

Replay sends real requests, so it checks before it sends, and refuses with exit code `2`:

| Rule | Refusal |
|---|---|
| Echo must be running and sealed, so the site's reactions can't reach anyone | `Echo is in observe mode; replays only run sealed, so the site's reactions can't reach anyone (rhythm echo up --seal)` |
| The target must be listed under `staging:` in the config, or resolve **only** to loopback or private addresses (RFC 1918, `fc00::/7`; not link-local, where cloud metadata lives). The address is checked again when connecting, so DNS can't change the answer in between. | `replay refused: 203.0.113.10 resolves to 203.0.113.10, which is neither loopback nor private; add the staging URL under staging: in the Echo config to allow it` |
| No live Stripe secret or restricted key (`sk_live_…`, `rk_live_…`) anywhere in reach: the environment, `.env`, `wp-config.php`, `../wp-config.php` and `.echo/config.yaml` | ``replay refused: a live Stripe key is in reach (STRIPE_SECRET_KEY=sk_live_•••••); run replays where only test keys are configured`` |
| The payload must not say `"livemode": true` | `replay refused: the payload has "livemode": true; replays are for test-mode events only` |
| The secret must not be a live key or an API key | `replay refused: ECHO_STRIPE_WEBHOOK_SECRET holds a Stripe API key, not a webhook signing secret (whsec_…)` |

Redirects are never followed (a redirect could point anywhere, including production), and no proxy is used. A `staging:` entry matches on scheme, host and port, with the path at a segment boundary:

```yaml
staging: ["https://staging.shop.test"]
```

## PayPal

```text
rhythm echo: PayPal webhooks can't be replayed: receivers verify them by asking PayPal's API (POST /v1/notifications/verify-webhook-signature) or with PayPal's signing certificate, so no local secret can sign one. Use the PayPal sandbox instead: Developer Dashboard → Webhooks simulator, pointed at the staging URL, with sandbox credentials
```

The library still has `paypal/PAYMENT.CAPTURE.COMPLETED` so you can see the payload's shape. To test a PayPal flow, use the sandbox's simulator with Echo running: the site's calls to PayPal's sandbox API are recorded, and faked unless you [allow](Echo-Sealing-Staging#the-allowlist) `*.sandbox.paypal.com`.

## Output and exit codes

After the response, replay waits for what the site sends, until nothing new arrives for 1.5 s and at most `--wait` (default `5s`), and lists it. Async senders need more: raise `--wait`, or use a replay action with `flush:` commands.

| Code | Meaning |
|---|---|
| `0` | The site answered `2xx` |
| `1` | The site answered something else (a `400` for a bad signature, a `500`) |
| `2` | Refused by a safety rule, or a usage error: an unknown event, a missing signing secret |
| `3` | Echo isn't running, or the request failed |

```text
Response: 400 in 1ms

  The site answered 400 and sent no messages in response.
```

A `400` usually means the site rejected the signature: check the secret matches the one the site's endpoint uses. `--json` prints the replay (event, scheme, URL, run, status, the site's answer, masked headers and secret) and the messages together.

## As an action

To compare what a webhook triggers before and after a change, make it an action:

```yaml
actions:
  - name: Stripe payment succeeded
    replay:
      event: payment_intent.succeeded
      to: /?wc-api=wc_stripe
      set: {data.object.amount: "2400"}
      secret_env: ECHO_STRIPE_WEBHOOK_SECRET
```

The same rules apply, and a non-2xx answer fails the action. See [Actions and compare](Echo-Actions-And-Compare#replay).

---
<sub>← [With Soundcheck](Echo-With-Soundcheck) · [Home](Home) · [Deliverability](Echo-Deliverability) →</sub>
