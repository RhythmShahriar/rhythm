Everything a site sends becomes a **message** in Echo: an email, an SMS, a Slack alert, a webhook, an analytics event, a payment call. This page covers the channels, the 34 services Echo recognizes, what a message records and how it's masked, how to list and inspect messages, and every check Echo runs on them, with its level and fix.

## Channels

| Channel | What | Examples |
|---|---|---|
| `email` | Mail handed to the site's mailer, the SMTP sink or `sendmail`, or sent through an email API | `wp_mail()`, Laravel mailables, Nodemailer, SendGrid |
| `sms` | Text messages through an SMS API | Twilio, Vonage, Plivo |
| `whatsapp` | WhatsApp messages | WhatsApp Cloud API, Twilio with a `whatsapp:` number |
| `chat` | Team chat alerts | Slack, Discord, Microsoft Teams |
| `webhook` | Anything posted to an API: CRMs, automation hooks, WooCommerce webhooks, and any unrecognized `POST`, `PUT` or `PATCH` | Zapier, HubSpot, a webhook to your own app |
| `analytics` | Server-side tracking events | GA4 Measurement Protocol, Meta Conversions API |
| `payment` | Payment API calls | Stripe, PayPal |
| `push` | Push notifications | FCM, OneSignal |
| `http` | Any other outbound request Echo couldn't classify (a `GET`, a `DELETE`, a tunnel through the proxy) | |

Reports list channels in this order.

## Recognized services

A connector reports a raw HTTP request; Echo works out which service it's for and pulls out the recipient, the content and the event. Each recognizer has fixture tests in the Echo repository. Anything not listed is still captured, as a generic webhook (with its host and path as the recipient) or a plain request.

| Service | Channel | Recognized by | Records |
|---|---|---|---|
| SendGrid | email | `POST api.sendgrid.com/v3/mail/send` | from, to, cc, bcc, subject, text, HTML |
| Mailgun | email | `api.mailgun.net` and `api.eu.mailgun.net` `/v3/<domain>/messages` | the same |
| Postmark | email | `api.postmarkapp.com/email…`, batches too | the same |
| Resend | email | `POST api.resend.com/emails` and `/emails/batch` | the same |
| Amazon SES | email | `email.<region>.amazonaws.com` (v1 and v2 APIs) | the same |
| Brevo (Sendinblue) | email | `api.brevo.com` and `api.sendinblue.com` `/v3/smtp/email` | the same |
| SparkPost | email | `POST api.sparkpost.com/api/v1/transmissions` (and EU) | the same |
| Mandrill | email | `mandrillapp.com/api/1.0/messages/send…` | the same |
| Twilio | sms, whatsapp | `POST api.twilio.com/…/Messages.json` | to, from, body; `whatsapp:` numbers become WhatsApp; content templates as the event |
| Vonage | sms | `rest.nexmo.com/sms/json` and the Messages API at `api.nexmo.com/v1/messages` | to, from, text |
| MessageBird | sms | `POST rest.messagebird.com/messages` | recipients, originator, body |
| Bird | sms | `POST api.bird.com/workspaces/…/channels/…/messages` | recipient, text |
| Plivo | sms | `POST api.plivo.com/…/Account/…/Message` | dst, src, text |
| Amazon SNS | sms | `sns.<region>.amazonaws.com` `Publish` to a phone number | phone number, message |
| Telnyx | sms | `POST api.telnyx.com/v2/messages` | to, from, text |
| ClickSend | sms | `rest.clicksend.com/v3/sms/send` | each message's to and body |
| WhatsApp Cloud API | whatsapp | `POST graph.facebook.com/…/messages` | to, text or template name |
| Slack | chat | `hooks.slack.com` incoming webhooks, `slack.com/api/chat.postMessage` | channel, text |
| Discord | chat | `discord.com/api/webhooks/…` | content |
| Microsoft Teams | chat | `webhook.office.com`, `outlook.office.com`, Power Automate workflows (`logic.azure.com`, `api.powerplatform.com`) | card text |
| GA4 | analytics | Measurement Protocol on `google-analytics.com` and `analytics.google.com` | event names, as the event |
| Meta Conversions API | analytics | `POST graph.facebook.com/…/events` | event names |
| FCM | push | `POST fcm.googleapis.com/v1/projects/…/messages:send` | token or topic, title, body |
| OneSignal | push (email or sms when the call targets those) | `POST onesignal.com` notifications API | recipients or segments, heading, content |
| Stripe | payment | `api.stripe.com` | the call (`payment_intents create`), amount and currency, test or live mode |
| PayPal | payment | `api-m.paypal.com`, `api.paypal.com` and the sandbox hosts | the call, sandbox or live mode |
| WooCommerce webhooks | webhook | any request with an `X-WC-Webhook-Topic` header | the topic (`order.created`) as the event, the delivery URL |
| HubSpot | webhook | `api.hubapi.com`, `api.hubspot.com`, `api.hsforms.com` | the call (`contacts create`) or `form submit`, and the contact's email |
| Mailchimp (Marketing API) | webhook | `api.mailchimp.com/3.0/…` | the list member's email |
| Klaviyo | webhook | `a.klaviyo.com` | the metric or call, and the profile's email |
| ActiveCampaign | webhook | `…api-us1.com` and `…activehosted.com` `/api/3/…` | the contact |
| Zapier | webhook | `hooks.zapier.com` | the hook, with its secret part masked |
| Make | webhook | `hook.*.make.com`, `hook.*.integromat.com` | the hook, with its secret part masked |
| n8n | webhook | `/webhook/` and `/webhook-test/` paths on `*.n8n.cloud`, on a host with an `n8n` label (`n8n.shop.com`, `shop-n8n.example.net`), or on port 5678 | the hook, with its secret part masked |

`rhythm echo doctor` reports how many services are recognized (`34 providers recognized`).

Titles and message lines use each service's display name: `Amazon SES email`, `MessageBird SMS`, `Teams message`, `Meta Conversions API event`, `Stripe call`, `Zapier webhook`, `HubSpot webhook`. A WooCommerce webhook and an unrecognized webhook name the host they go to (`WooCommerce webhook to shop.example.com`, `webhook to crm.shop.test`). Anything else is `request to <host>`, and a tunnel through the proxy `HTTPS connection to <host>`.

## What a message records

```sh
rhythm echo show f0e357147b13
```

```json
{
  "id": "f0e357147b13",
  "channel": "payment",
  "provider": "stripe",
  "subject": "live mode",
  "text": "2400 eur",
  "event": "payment_intents create",
  "method": "POST",
  "url": "https://api.stripe.com/v1/payment_intents",
  "headers": {
    "authorization": "•••••",
    "content-type": "application/x-www-form-urlencoded"
  },
  "body": "amount=2400&currency=eur",
  "response": {
    "status": 403,
    "body": "{\"error\":{\"code\":\"rhythm_echo_sealed\",\"message\":\"Rhythm Echo refused a live-mode call from a sealed site. Use a Stripe test key (sk_test_…) on staging.\",\"type\":\"invalid_request_error\"}}"
  },
  "status": "blocked",
  "error": "refused: a live-mode Stripe call; use test credentials on staging",
  "source": {
    "via": "my-connector"
  },
  "at": "2026-10-24T02:16:33.034003618Z"
}
```

| Field | Meaning |
|---|---|
| `id` | A short ID for `show`. |
| `run`, `action` | The run and action that caused it, from `X-Echo-Run`; see [Run IDs](Echo-Actions-And-Compare#run-ids). |
| `channel`, `provider` | What it is and which service. |
| `from`, `to`, `cc`, `bcc`, `reply_to`, `subject` | For email; `to` is the number for SMS, the channel for chat, the hook for webhooks. |
| `text`, `html` | The content a reader sees. |
| `event` | An analytics event, a webhook topic, a template, a payment call. |
| `method`, `url`, `headers`, `body` | The raw request (or the raw MIME message for email), masked and capped. |
| `response` | What the service, or Echo's fake, answered. |
| `status` | `faked`, `blocked`, `sent` or `failed`; see [Sealing staging](Echo-Sealing-Staging#what-happens-to-each-message). |
| `error` | Why a send failed or was refused. |
| `source` | How Echo saw it: `via` (`wordpress`, `laravel`, `node`, `smtp`, `sendmail`, `proxy`), the stack, the plugin or package that sent it when known, the hook (`wp_mail`, `pre_http_request`, `fetch`, `nodemailer`…) and the site's host. |
| `at` | When it was captured, in UTC. |

### Masking

Credentials are masked before a message is stored:

- `Authorization`, `Proxy-Authorization`, `Cookie`, `X-Api-Key`, `Api-Key`, `X-Postmark-Server-Token` and `X-Echo-Token` headers are masked whole: `•••••`.
- Known secrets in other headers, URLs and bodies keep a short prefix: `sk_live_•••••`, `https://hooks.slack.com/services/T0000/B0000/fi•••••`.
- A Zapier, Make or n8n hook URL is its own credential, so its secret last part is masked in `url`, `to`, the live view and the reports: `https://hooks.zapier.com/hooks/catch/123456/ab•••••/`. So are `sig`, `signature` and `code` query values, which covers Logic Apps and Power Automate trigger URLs. The masked form is the same every run, so `compare` still pairs the same hook. Two hooks on one account whose secrets start with the same two characters look the same to `compare`.
- The connectors mask credential headers before anything leaves the site, and keep only the first few characters of payment keys, which is all the [live-mode check](Echo-Sealing-Staging#live-payments-are-refused) needs.

Content isn't masked: names, addresses, order details and links in emails, including password reset links, are stored as sent. Bodies are capped at 256 KiB per message (larger ones end in `…(truncated)`). Treat captures, `.echo/` and reports like a database dump; see [Responsible use](Responsible-Use#what-echo-does).

## Listing and inspecting

The running catcher keeps the newest 10,000 messages in memory; they're gone when it stops. Snapshots and runs keep their own copies in `.echo/`.

```sh
rhythm echo watch                       # new messages as they arrive (Ctrl+C to stop)
rhythm echo ls                          # everything captured so far
rhythm echo ls --channel sms            # one channel
rhythm echo ls --to 1555                # recipients containing "1555", ignoring case
rhythm echo ls --run 2026-10-24T08-29-07-cd7ff3   # one run
rhythm echo show f0e357147b13           # one message as JSON
rhythm echo ls --json                   # all of them as JSON
rhythm echo clear                       # delete them (takes the same filters)
```

```text
bfed9f3b36ed  08:16:19 email to jamie@shop.test: Your order #1042 [faked]
5ea4bd5bdfb0  08:16:33 Twilio SMS to +15551230000: Your code is 4821 [faked]
f0e357147b13  08:16:33 Stripe call to api.stripe.com: live mode [blocked]
d598160b7ade  08:16:33 Stripe call to api.stripe.com: test mode [faked]
0b084eca24a7  08:16:40 Zapier webhook to hooks.zapier.com/hooks/catch/123456/ab•••••: POST [faked]
335cd5c64334  08:16:41 webhook to crm.shop.test/api/leads: POST [faked]
```

That's the plain form, piped or with `--plain`. On a terminal each line has an icon per channel and columns for the kind, the recipient, the content and the status.

```text
$ rhythm echo clear
Deleted 5 messages
```

`watch`, `ls` and `clear` take `--channel`, `--to` and `--run`; an unknown channel is a usage error. `show` with an unknown ID exits `2`: `no message "x" in Echo; list them with: rhythm echo ls`. The live view at http://127.0.0.1:8025 shows the same messages, grouped by action, as an HTML report that refreshes every five seconds.

## Checks

Echo checks each message an action sends: `snapshot`, `compare` and `run` list the problems, and Soundcheck's `--echo` reports them as page issues. Each problem has a stable title, so a comparison can tell a new problem from one the snapshot already had: a new error is **BROKE**, a new warning or notice **NEW**, and one that went away **FIXED**. `watch` and `ls` show messages without checking them.

```text
[ERROR] Unfilled placeholder *|ORDER_ID|* in email - Order email
  The template still contains the placeholder, so readers see it instead of their data.
  fix: Check the tag's spelling against the plugin's list of tags, and that the data it needs exists when the message is sent.
[ERROR] PHP error printed in email - Order email
  PHP Warning: Undefined array key "total" (plugins/shop-mailer/templates/order.php line 42)
  fix: Fix the code the error points at, and turn display_errors off so errors never reach messages.
[WARNING] Email links to a development address (localhost) - Order email
  fix: Build links from the site's configured URL, not a hard-coded or local one.
[WARNING] Garbled characters in email - Order email
  found "Ã©": text was encoded twice
  fix: Make sure the message is built and sent as UTF-8 (the mailer's charset and the template file's encoding).
[WARNING] SMS needs more than 3 parts - Shipping SMS
  4 parts (GSM-7); each part is charged
  fix: Shorten the message, or avoid characters that force UCS-2 (emoji, curly quotes).
[WARNING] WooCommerce webhook is sent without a signature - Order webhooks
  fix: Set a secret on the webhook (WooCommerce → Settings → Advanced → Webhooks) so receivers can verify it.
[NOTICE] Email is sent from another domain (othermailer.com) - Order email
  the site is staging.yoursite.com

Messages have errors - 3 errors, 7 warnings, 2 notices - 3 actions, 2 emails, 1 SMS, 2 webhooks
```

`run`, `snapshot` and `compare` exit `0` whatever they find, unless you set `--fail-on`: `--fail-on error` for a run or snapshot, `--fail-on broke` for a comparison. See [CLI reference](Echo-CLI-Reference#exit-codes).

### Content

These apply to email, SMS, WhatsApp and chat messages. The text checked is the plain-text part, or the HTML with its tags removed, plus the subject.

| Check | Level | Title | Fix |
|---|---|---|---|
| `content/placeholder` | error | Unfilled placeholder `{first_name}` in email | Check the tag's spelling against the plugin's list of tags, and that the data it needs exists when the message is sent. |
| `content/php-error` | error | PHP error printed in email | Fix the code the error points at, and turn `display_errors` off so errors never reach messages. |
| `content/encoding` | warning | Garbled characters in email | Make sure the message is built and sent as UTF-8 (the mailer's charset and the template file's encoding). |
| `content/dev-link` | warning | Email links to a development address (localhost) | Build links from the site's configured URL, not a hard-coded or local one. |
| `content/secret` | warning | Webhook to … carries a credential in its body | Don't put API keys or passwords in payloads; send them in an Authorization header, or not at all. |

- **Placeholders** found: `{{ name }}`, `*|FNAME|*` (Mailchimp), `%%name%%`, `{first_name}` (lowercase names in single braces), and `[your-name]` or `[_field]` (Contact Form 7).
- **PHP errors**: `Fatal error`, `Parse error`, `Warning`, `Notice` and `Deprecated` lines with a file and line number, in plain text or PHP's HTML format. The detail shortens the path to what follows `wp-content/`.
- **Development addresses**: `localhost`, `127.*`, `10.*`, `192.168.*`, `0.0.0.0`, and hosts ending in `.local`, `.test`, `.localhost`, `.ddev.site`, `.lndo.site`, `.docker` or `.invalid`. This check is skipped when the configured `site:` is itself a development address, since then every link would match.
- **Credentials in a body** (`content/secret`) is reported for any message but email whose stored body had a secret masked, whatever happened to it: a sealed site's payload says what the live site would send. It applies to webhooks and other API calls, not only the channels above, for example `Webhook to crm.shop.test carries a credential in its body`.

### Email

| Check | Level | Title | Fix |
|---|---|---|---|
| `email/empty` | error | Email is empty | The template rendered nothing: check the notification's message setting and the variables it uses. |
| `email/no-subject` | warning | Email has no subject | Give the notification a subject; many filters treat mail without one as spam. |
| `email/clipped` | warning | Email is too big for Gmail and gets clipped | Trim inline CSS and markup, or move content behind links. (Gmail clips HTML over 102 KB.) |
| `email/no-text` | notice | Email has no plain-text part | Send a text alternative too; some clients and spam filters prefer it. |
| `email/from-domain` | notice | Email is sent from another domain (othermailer.com) | Send from an address on your own domain, or make sure its SPF, DKIM and DMARC cover this sender. |

The From-domain check compares the sender's domain with `site:`, without `www.`, on their last two labels (so `mail.yoursite.com` counts as `yoursite.com`). It's skipped when `site:` is unset, an IP address or a development address.

### SMS and WhatsApp

| Check | Level | Title | Fix |
|---|---|---|---|
| `sms/number` | warning | Message sent to a number that isn't in international format | Store and send numbers in E.164 form, e.g. `+4915112345678`. |
| `sms/parts` | warning | SMS needs more than 3 parts | Shorten the message, or avoid characters that force UCS-2 (emoji, curly quotes). |
| `sms/parts` | notice | SMS needs more than one part | Keep SMS under 160 GSM-7 characters (70 with emoji or other Unicode). |

Parts are counted the way carriers charge them: one GSM-7 part holds 160 characters (153 each when split, with `^{}\[~]|€` counting twice), one UCS-2 part 70 (67 each when split). Any character outside GSM-7 makes the whole message UCS-2.

### Webhooks

| Check | Level | Title | Fix |
|---|---|---|---|
| `webhook/json` | warning | Webhook to crm.shop.test sends invalid JSON | The payload says it's JSON but doesn't parse; check how it's built (`json_encode` errors, stray output). |
| `webhook/signature` | warning | WooCommerce webhook is sent without a signature | Set a secret on the webhook (WooCommerce → Settings → Advanced → Webhooks) so receivers can verify it. |

### Sending and recipients

| Check | Level | Title | Fix |
|---|---|---|---|
| `send/failed` | error | Email fails, Twilio SMS fails (HTTP 401) | Check the sender's settings (API key, SMTP login, endpoint URL) and the provider's error above. |
| `seal/real-recipient` | warning | Email really sent to an address outside the allowlist | Run sealed on staging (`rhythm echo up`), or add test recipients to `allow_recipients`. |

`seal/real-recipient` only applies to messages a connector really sent (observe mode or an allowed host; never mail caught by the SMTP sink or `sendmail`), and ignores test addresses: see [The allowlist](Echo-Sealing-Staging#the-allowlist).

### Actions

| Check | Level | Title | Fix |
|---|---|---|---|
| `expect/email`, `expect/sms`, `expect/webhook`, `expect/chat` | error | Sent fewer emails than expected (detail: `expected 2, got 1`) | Check the settings of whatever sends them, or update `expect` in `.echo/config.yaml` if the change is intended. |

These come from an action's `expect:`; see [Actions and compare](Echo-Actions-And-Compare#expect). Comparisons add their own changes (a message no longer sent, a changed subject, a payload that lost fields); those are listed in [Reading a comparison](Echo-Actions-And-Compare#reading-a-comparison). Deliverability checks on DNS and headers are in [Deliverability](Echo-Deliverability).

---
<sub>← [Sealing staging](Echo-Sealing-Staging) · [Home](Home) · [Actions and compare](Echo-Actions-And-Compare) →</sub>
