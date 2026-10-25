Everything a site sends becomes a **message** in Echo: an email, an SMS, a Slack alert, a webhook, an analytics event, a payment call. This page covers the channels and the 53 services Echo recognizes. It covers what a message records, how it's masked and how to list and inspect messages. It also lists every check Echo runs on them, with its level and fix; turning checks off and adding your own is in [Checks and rules](Echo-Checks-And-Rules).

## Channels

| Channel | What | Examples |
|---|---|---|
| `email` | Mail handed to the site's mailer, the SMTP sink or `sendmail`, or sent through an email API | `wp_mail()`, Laravel mailables, Nodemailer, SendGrid, the Gmail API |
| `sms` | Text messages through an SMS API | Twilio, Vonage, Plivo, Sinch |
| `whatsapp` | WhatsApp messages | WhatsApp Cloud API, Twilio with a `whatsapp:` number |
| `chat` | Team chat alerts | Slack, Discord, Microsoft Teams, Telegram bots |
| `webhook` | Anything posted to an API: CRMs, automation hooks, WooCommerce webhooks and any unrecognized `POST`, `PUT` or `PATCH` | Zapier, HubSpot, IFTTT, a webhook to your own app |
| `analytics` | Server-side tracking events | GA4 Measurement Protocol, Meta Conversions API, Segment, Mixpanel, PostHog |
| `payment` | Payment API calls | Stripe, PayPal, Adyen, Braintree, Square, Authorize.net, Mollie, Razorpay |
| `push` | Push notifications | FCM, APNs, OneSignal, Expo |
| `http` | Any other outbound request Echo couldn't classify (a `GET`, a `DELETE`, a tunnel through the proxy) | |

Reports list channels in this order.

## Recognized services

A connector reports a raw HTTP request; Echo works out which service it's for and pulls out the recipient, the content and the event. Each recognizer has positive and negative fixture tests in the Echo repository. Anything not listed is still captured, as a generic webhook (with its host and path as the recipient, long or token-like path segments shown as `…`) or a plain request. `rhythm echo providers` lists them by channel:

```sh
rhythm echo providers
```

```text
ANALYTICS (5)
  GA4                   ga4        google-analytics.com, analytics.google.com
  Meta Conversions API  meta-capi  graph.facebook.com
  Mixpanel              mixpanel   api.mixpanel.com, api-eu.mixpanel.com, api-in.mixpanel.com (YAML)
  PostHog               posthog    i.posthog.com, app.posthog.com, eu.posthog.com (YAML)
  Segment               segment    api.segment.io, events.eu1.segmentapis.com (YAML)

CHAT (4)
  Discord   discord   discord.com, discordapp.com
  Slack     slack     hooks.slack.com, slack.com
  Teams     teams     webhook.office.com, outlook.office.com, outlook.office365.com, logic.azure.com, api.powerplatform.com
  Telegram  telegram  api.telegram.org

EMAIL (11)
  Amazon SES       amazon-ses       email.*.amazonaws.com, email-fips.*.amazonaws.com
  Brevo            brevo            api.brevo.com, api.sendinblue.com
  Gmail API        gmail            gmail.googleapis.com, www.googleapis.com
  Mailgun          mailgun          api.mailgun.net, api.eu.mailgun.net
  Mailjet          mailjet          api.mailjet.com (YAML)
  Mandrill         mandrill         mandrillapp.com
  Microsoft Graph  microsoft-graph  graph.microsoft.com, graph.microsoft.us, dod-graph.microsoft.us, microsoftgraph.chinacloudapi.cn
  Postmark         postmark         api.postmarkapp.com
  Resend           resend           api.resend.com
  SendGrid         sendgrid         api.sendgrid.com
  SparkPost        sparkpost        api.sparkpost.com, api.eu.sparkpost.com

PAYMENT (8)
  Adyen          adyen          *-live.adyenpayments.com, *-live.adyen.com, *-test.adyen.com (live calls refused)
  Authorize.net  authorize-net  api.authorize.net, api2.authorize.net, apitest.authorize.net (live calls refused)
  Braintree      braintree      api.braintreegateway.com, payments.braintree-api.com, api.sandbox.braintreegateway.com, payments.sandbox.braintree-api.com (live calls refused)
  Mollie         mollie         api.mollie.com (live calls refused)
  PayPal         paypal         api-m.paypal.com, api.paypal.com, api-m.sandbox.paypal.com, api.sandbox.paypal.com (live calls refused)
  Razorpay       razorpay       api.razorpay.com (live calls refused)
  Square         square         connect.squareup.com, connect.squareupsandbox.com (live calls refused)
  Stripe         stripe         api.stripe.com (live calls refused)

PUSH (4)
  APNs       apns       api.push.apple.com, api.sandbox.push.apple.com, api.development.push.apple.com
  Expo Push  expo       exp.host, api.expo.dev (YAML)
  FCM        fcm        fcm.googleapis.com
  OneSignal  onesignal  onesignal.com

SMS (11)
  Amazon SNS   amazon-sns   sns.*.amazonaws.com
  Bandwidth    bandwidth    messaging.bandwidth.com (YAML)
  Bird         bird         api.bird.com
  ClickSend    clicksend    rest.clicksend.com
  Infobip      infobip      api.infobip.com (YAML)
  MessageBird  messagebird  rest.messagebird.com
  Plivo        plivo        api.plivo.com
  Sinch        sinch        sms.api.sinch.com (YAML)
  Telnyx       telnyx       api.telnyx.com
  Twilio       twilio       api.twilio.com
  Vonage       vonage       rest.nexmo.com, api.nexmo.com, messages-sandbox.nexmo.com

WEBHOOK (9)
  ActiveCampaign  activecampaign  api-us1.com, activehosted.com
  HubSpot         hubspot         api.hubapi.com, api.hubspot.com, api.hsforms.com
  IFTTT           ifttt           maker.ifttt.com
  Klaviyo         klaviyo         a.klaviyo.com
  Mailchimp       mailchimp       api.mailchimp.com
  Make            make            hook.*.make.com, hook.*.integromat.com
  n8n             n8n             n8n.cloud, n8n.*, *-n8n.*, *:5678
  WooCommerce     woocommerce     any host, by its headers
  Zapier          zapier          hooks.zapier.com

WHATSAPP (1)
  WhatsApp Cloud API  whatsapp-cloud  graph.facebook.com

53 services recognized. Anything else is captured as a generic webhook or request.
```

`--json` gives the same list as JSON (`provider`, `name`, `channel`, `hosts`, `yaml`, `payment`). `(YAML)` marks a service described in a YAML file with its fixtures rather than in Go; payment gateways are always in Go, since they must refuse live calls. What each one records:

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
| Mailjet | email | `POST api.mailjet.com/v3.1/send` | the same |
| Gmail API | email | `users.messages.send` on `gmail.googleapis.com` and `www.googleapis.com` | the raw message, decoded: from, to, cc, bcc, Reply-To, subject, text, HTML |
| Microsoft Graph | email | `POST …/users/{id}/sendMail` and `…/me/sendMail` on `graph.microsoft.com` and the government and China clouds | the same, from the JSON message or a base64 MIME body |
| Twilio | sms, whatsapp | `POST api.twilio.com/…/Messages.json` | to, from, body; `whatsapp:` numbers become WhatsApp; content templates as the event |
| Vonage | sms | `rest.nexmo.com/sms/json` and the Messages API at `api.nexmo.com/v1/messages` | to, from, text |
| MessageBird | sms | `POST rest.messagebird.com/messages` | recipients, originator, body |
| Bird | sms | `POST api.bird.com/workspaces/…/channels/…/messages` | recipient, text |
| Plivo | sms | `POST api.plivo.com/…/Account/…/Message` | dst, src, text |
| Amazon SNS | sms | `sns.<region>.amazonaws.com` `Publish` to a phone number | phone number, message |
| Telnyx | sms | `POST api.telnyx.com/v2/messages` | to, from, text |
| ClickSend | sms | `rest.clicksend.com/v3/sms/send` | each message's to and body |
| Sinch | sms | `/xms/v1/{plan}/batches` on `*.sms.api.sinch.com` | to, from, body |
| Infobip | sms | `/sms/2/text/advanced`, `/sms/1/text/single`, `/sms/3/messages` on `*.api.infobip.com` | destinations, text |
| Bandwidth | sms | `/api/v2/users/{account}/messages` on `messaging.bandwidth.com` | to, from, text |
| WhatsApp Cloud API | whatsapp | `POST graph.facebook.com/…/messages` | to, text or template name |
| Slack | chat | `hooks.slack.com` incoming webhooks, `slack.com/api/chat.postMessage` | channel, text |
| Discord | chat | `discord.com/api/webhooks/…` | content |
| Microsoft Teams | chat | `webhook.office.com`, `outlook.office.com`, Power Automate workflows (`logic.azure.com`, `api.powerplatform.com`) | card text |
| Telegram | chat | Bot API `send*` calls on `api.telegram.org` | `chat_id`, text or caption; the method as the event for anything but `sendMessage` |
| GA4 | analytics | Measurement Protocol on `google-analytics.com` and `analytics.google.com` | event names, as the event |
| Meta Conversions API | analytics | `POST graph.facebook.com/…/events` | event names |
| Segment | analytics | `/v1/track`, `identify`, `page`, `screen`, `group`, `alias`, `batch`, `import` on `api.segment.io` and `events.eu1.segmentapis.com` | the event |
| Mixpanel | analytics | `/track` and `/engage` on `api`, `api-eu` and `api-in.mixpanel.com` | the call (the base64 `data=` form isn't read) |
| PostHog | analytics | `/capture`, `/batch`, `/e`, `/i/v0/e` on PostHog Cloud | the event |
| FCM | push | `POST fcm.googleapis.com/v1/projects/…/messages:send` | token or topic, title, body |
| APNs | push | `POST /3/device/{token}` on `api.push.apple.com` and the sandbox hosts | the device token, the alert's title and body, the push type when it isn't `alert` |
| Expo Push | push | `/--/api/v2/push/send` on `exp.host`, `/v2/push/send` on `api.expo.dev` | the push tokens, title, body |
| OneSignal | push (email or sms when the call targets those) | `POST onesignal.com` notifications API | recipients or segments, heading, content |
| Stripe | payment | `api.stripe.com` | the call (`payment_intents create`), amount and currency, test or live mode |
| PayPal | payment | `api-m.paypal.com`, `api.paypal.com` and the sandbox hosts | the call, sandbox or live mode |
| Adyen, Braintree, Square, Authorize.net | payment | their live and sandbox hosts | the call (`payments create`; Authorize.net: the request type), live or sandbox mode |
| Mollie, Razorpay | payment | `api.mollie.com`, `api.razorpay.com` | the call, live or test mode by the key |
| WooCommerce webhooks | webhook | any request with an `X-WC-Webhook-Topic` header | the topic (`order.created`) as the event, the delivery URL |
| HubSpot | webhook | `api.hubapi.com`, `api.hubspot.com`, `api.hsforms.com` | the call (`contacts create`) or `form submit`, plus the contact's email |
| Mailchimp (Marketing API) | webhook | `api.mailchimp.com/3.0/…` | the list member's email |
| Klaviyo | webhook | `a.klaviyo.com` | the metric or call, plus the profile's email |
| ActiveCampaign | webhook | `…api-us1.com` and `…activehosted.com` `/api/3/…` | the contact |
| Zapier | webhook | `hooks.zapier.com` | the hook, with its secret part masked |
| IFTTT | webhook | `maker.ifttt.com/trigger/{event}/…` | the event, as `ifttt <event>`, with the key masked |
| Make | webhook | `hook.*.make.com`, `hook.*.integromat.com` | the hook, with its secret part masked |
| n8n | webhook | `/webhook/` and `/webhook-test/` paths on `*.n8n.cloud`, on a host with an `n8n` label (`n8n.shop.com`, `shop-n8n.example.net`), or on port 5678 | the hook, with its secret part masked |

`rhythm echo doctor` reports how many services are recognized (`53 providers recognized`). A WooCommerce webhook sent to a Zapier, Make or n8n hook is recorded as that hook's service, with the topic (`order.created`) as the event.

Titles and message lines use each service's display name: `Amazon SES email`, `MessageBird SMS`, `Teams message`, `Meta Conversions API event`, `Stripe call`, `Zapier webhook`, `HubSpot webhook`. A WooCommerce webhook and an unrecognized webhook name the host they go to (`WooCommerce webhook to shop.example.com`, `webhook to crm.shop.test`). Anything else is `request to <host>`. A tunnel through the proxy is `HTTPS connection to <host>`.

## What a message records

```sh
rhythm echo show 23bb60c909ae
```

```json
{
  "schema": "rhythm-echo/v1",
  "kind": "messages",
  …
  "data": [
    {
      "id": "23bb60c909ae",
      "channel": "payment",
      "provider": "stripe",
      "subject": "live mode",
      "text": "2400 eur",
      "event": "payment_intents create",
      "method": "POST",
      "url": "https://api.stripe.com/v1/payment_intents",
      "headers": {
        "authorization": "•••••"
      },
      "body": "amount=2400&currency=eur",
      "response": {
        "status": 403,
        "body": "{\"error\":{\"code\":\"rhythm_echo_sealed\",\"message\":\"Rhythm Echo refused a live-mode call. Use a Stripe test key (sk_test_…) on staging.\",\"type\":\"invalid_request_error\"}}"
      },
      "status": "blocked",
      "error": "refused: a live-mode Stripe call; use test credentials on staging",
      "source": {
        "via": "my-connector"
      },
      "at": "2026-10-25T14:36:09.833523391Z"
    }
  ]
}
```

`show` prints the same JSON envelope as every `--json` (see [CLI reference](Echo-CLI-Reference#json-output)), with the message in `data`.

| Field | Meaning |
|---|---|
| `id` | A short ID for `show`. |
| `run`, `action` | The run and action that caused it, from `X-Echo-Run`; see [Run IDs](Echo-Actions-And-Compare#run-ids). |
| `attribution`, `late_for` | `late` with the action it belongs to, for mail that arrived after its action ended; `ambiguous` when several actions were open and the message named none. |
| `channel`, `provider` | What it is and which service. |
| `from`, `to`, `cc`, `bcc`, `reply_to`, `subject` | For email; `to` is the number for SMS, the channel for chat, the hook for webhooks. |
| `text`, `html` | The content a reader sees. |
| `event` | An analytics event, a webhook topic, a template, a payment call. |
| `method`, `url`, `headers`, `body` | The raw request (or the raw MIME message for email), masked and capped. |
| `attachments` | An email's files: name, type, size and SHA-256, never the content. See [Attachments](Echo-Attachments). |
| `response` | What the service, or Echo's fake, answered. |
| `status` | `faked`, `blocked`, `sent` or `failed`; see [Sealing staging](Echo-Sealing-Staging#what-happens-to-each-message). |
| `error` | Why a send failed or was refused. |
| `issues` | What [the checks](#checks) found in this message. |
| `truncated` | `true` when a field was cut to the size cap. |
| `source` | How Echo saw it: `via` (`wordpress`, `laravel`, `node`, `smtp`, `sendmail`, `proxy`), the stack, the plugin or package that sent it when known, the hook (`wp_mail`, `pre_http_request`, `fetch`, `nodemailer`…), the site's host and, on an [agency hub](Echo-Hub-And-Reports), the site's ID (`site`). |
| `at` | When it was captured, in UTC. |

### Masking

Credentials are masked before a message is stored, shown or written to a report:

- **Headers** whose name contains `token`, `secret`, `key`, `auth`, `signature`, `session`, `password`, `cookie`, `credential`, `jwt` or `assertion` (any case) are masked whole: `•••••`. That covers `Authorization`, `X-Auth-Token`, `X-Goog-Api-Key`, `X-Shopify-Access-Token`, `Set-Cookie`, `X-WC-Webhook-Signature`, `DKIM-Signature` and the like. `X-Request-Id` and `Content-Type` are kept.
- **URLs** (the request's, every link in a text or HTML, every recipient): the values of the query keys `key`, `k`, `code`, `otp`, `reset`, `token`, `sig`, `signature`, `hash`, `auth`, `ticket`, `magic`, `apikey` and any key ending in `signature`, `credential`, `secret`, `_token`, `-token`, `_key` or `-key` (`X-Amz-Signature`, `login_token`, `api_key`). A Telegram `bot<id>:<token>` and the segment after `key`, `token`, `secret` or `access_token` in a path keep at most a few characters. On a sign-in, magic, verify or reset link, the token in the query or the path is masked too.
- **Texts and subjects**: known secret shapes keep a short prefix (`sk_live_•••••`), a JSON Web Token keeps its first 4 characters and one-time codes named as such are masked whole (`Use 482913 to sign in.` is stored as `Use ••••• to sign in.`); zip and postal codes are kept.
- **JSON bodies**: values under keys containing `pass`, `pwd`, `secret`, `token` or `auth` (not `author`), ending in `key` (`apiKey`, `license_key`) or ending in `otp`, `pin` or `code` with a code-like value (`login_code`, `verificationCode`, not `zip_code` or `status_code`).
- **A Zapier, Make or n8n hook URL** is its own credential, so its secret last part is masked in `url`, `to`, Echo Inbox and the reports: `https://hooks.zapier.com/hooks/catch/1234567/ab•••••/`. The masked form is the same every run, so `compare` still pairs the same hook.
- **The connectors** mask credential headers the same way before anything leaves the site. A Stripe key keeps only its mode prefix (`Bearer sk_test_•••••`), which is all the [live-mode check](Echo-Sealing-Staging#live-payments-are-refused) needs.

A mask keeps nothing of a value that changes on every send (a reset key, a login code), so `compare` doesn't report it as changed. Echo reads one-time codes and links before it masks them and keeps them in memory beside the message, for `rhythm echo latest` and `GET /v1/messages/latest`; they're never written to a run, a snapshot, a kept inbox or a report.

Content isn't masked: names, addresses and order details are stored as sent. Each field is capped at 256 KiB (a cut one ends in `…(truncated)` and the message gets `"truncated": true`), a capture at 8 MiB and the whole inbox at 256 MiB, oldest first. Treat captures, `.echo/` and reports like a database dump; see [Responsible use](Responsible-Use#what-echo-does). Terminal, Markdown and JUnit output escape control characters and bidi overrides in captured text (`\x1b`, `\u202e`), so a subject can't rewrite your terminal or reorder a line.

## Listing and inspecting

The running catcher keeps the newest 10,000 messages in memory and forgets them when it stops, unless you give it a [store](Echo-Inbox#keeping-the-inbox-across-restarts). Snapshots and runs keep their own copies in `.echo/`.

```sh
rhythm echo watch                       # new messages as they arrive (Ctrl+C to stop)
rhythm echo watch --json                # one JSON object per line, for a script
rhythm echo ls                          # everything captured so far
rhythm echo ls --since 10m              # the last ten minutes (also 30d)
rhythm echo ls --channel sms            # one channel
rhythm echo ls --to 1555                # recipients containing "1555", ignoring case
rhythm echo ls --run 2026-10-24T08-29-07-cd7ff3   # one run
rhythm echo show 23bb60c909ae           # one message as JSON
rhythm echo ls --json                   # all of them as JSON
rhythm echo clear                       # delete them (takes the same filters)
```

```text
d3e2ccbf826f  14:26:22 email to ann@shop.test: Your sign-in code [faked]
23bb60c909ae  14:36:09 Stripe call to api.stripe.com: live mode [blocked]
502758beb1e5  14:36:09 Stripe call to api.stripe.com: test mode [faked]
7cf14f21d822  14:36:09 Adyen call to checkout-live.adyenpayments.com: live mode [blocked]
181afd6a549a  14:38:18 email to jamie@shop.test: Payment received for order #10234 [faked]
```

That's the plain form, piped or with `--plain`. On a terminal each line has an icon per channel and columns for the kind, the recipient, the content and the status, under 100 columns. With `-v`, an email's files follow it (`  attaches invoice-1042.pdf (application/pdf, 84 KB)`).

`watch` prints each message's problems under it and a line when a message's status changes later (`Update: 15:04:05 email to ann@shop.test is now blocked (was faked)`). `watch --since 10m` first shows the last ten minutes. `watch --json` writes one object per line: `{"type":"message","message":{…},"issues":[…]}` for a new message and `{"type":"update","message":{…},"was":"faked"}` for a status change; the `Watching … Ctrl+C to stop.` line goes to stderr.

```text
$ rhythm echo clear
Deleted 5 messages
```

`watch`, `ls` and `clear` take `--channel`, `--to` and `--run` (and `--since`, for `ls` and `watch`); an unknown channel is a usage error, as is an argument: `rhythm echo: ls takes no arguments; narrow it with --run, --to, --channel or --since (rhythm echo show <id> for one)`. `show` with an unknown ID exits `2`: `no message "x" in Echo; list them with: rhythm echo ls`. [Echo Inbox](Echo-Inbox), at http://127.0.0.1:8025, shows the same messages with search, every message's HTML and its checks.

## Checks

Echo checks every message it catches: `snapshot`, `compare` and `run` list the problems in what each action sent. The live log of `up`, `watch`, Echo Inbox and the API show them on every capture. Soundcheck's `--echo` reports them as page issues. Each problem has a stable title, so a comparison can tell a new problem from one the snapshot already had: a new error is **BROKE**, a new warning or notice **NEW** and one that went away **FIXED**.

Three actions on a WordPress site whose order mailer, SMS add-on and webhook went wrong:

```text
[ERROR] Unfilled placeholder *|ORDER_ID|* in email - Order email
  The template still contains the placeholder, so readers see it instead of their data.
  fix: Check the tag's spelling against the plugin's list of tags. Make sure the data it needs exists when the message is sent.
[ERROR] PHP error printed in email - Order email
  PHP Warning: Undefined array key "total" (plugins/shop-mailer/templates/order.php line 42)
  fix: Fix the code the error points at. Turn display_errors off so errors never reach messages.
[WARNING] Garbled characters in email - Order email
  found "Ã©": text was encoded twice
  fix: Make sure the message is built and sent as UTF-8 (the mailer's charset and the template file's encoding).
[WARNING] Email links to a development address (localhost) - Order email
  fix: Build links from the site's configured URL, not a hard-coded or local one.
[WARNING] SMS needs more than 3 parts - Shipping SMS
  4 parts (GSM-7); each part is charged
  fix: Shorten the message, or avoid characters that force UCS-2 (emoji, curly quotes).
[WARNING] WooCommerce webhook is sent without a signature - Order webhooks
  fix: Set a secret on the webhook (WooCommerce → Settings → Advanced → Webhooks) so receivers can verify it.
[NOTICE] Email is sent from another domain (othermailer.com) - Order email
  the site is shop.example.com

Messages have errors - 2 errors, 4 warnings, 1 notice - 3 actions, 1 email, 1 SMS, 1 webhook
```

Without `--fail-on`, `run` and `snapshot` exit `1` on an error (as here) and `compare` on a BROKE change; `--fail-on warning` is stricter, `--fail-on none` never fails. See [CLI reference](Echo-CLI-Reference#exit-codes). `rhythm echo checks` lists every check; `checks:` in `.echo/config.yaml` changes a check's level or turns it off; `rules:` adds your own. See [Checks and rules](Echo-Checks-And-Rules).

### Content

These apply to email, SMS, WhatsApp and chat messages. The text checked is the plain-text part, or the HTML with its tags removed, plus the subject.

| Check | Level | Title | Fix |
|---|---|---|---|
| `content/placeholder` | error | Unfilled placeholder `{first_name}` in email | Check the tag's spelling against the plugin's list of tags. Make sure the data it needs exists when the message is sent. |
| `content/php-error` | error | PHP error printed in email | Fix the code the error points at. Turn `display_errors` off so errors never reach messages. |
| `content/encoding` | warning | Garbled characters in email | Make sure the message is built and sent as UTF-8 (the mailer's charset and the template file's encoding). |
| `content/dev-link` | warning | Email links to a development address (localhost) | Build links from the site's configured URL, not a hard-coded or local one. |
| `content/secret` | warning | Webhook to … carries a credential in its body | Don't put API keys or passwords in payloads; send them in an Authorization header, or not at all. |

- **Placeholders** found: `{{ name }}`, `*|FNAME|*` (Mailchimp), `%%name%%`, `{first_name}` (lowercase names in single braces) and `[your-name]` or `[_field]` (Contact Form 7).
- **PHP errors**: `Fatal error`, `Parse error`, `Warning`, `Notice` and `Deprecated` lines with a file and line number, in plain text or PHP's HTML format. The detail shortens the path to what follows `wp-content/`.
- **Development addresses**: `localhost`, `127.*`, `10.*`, `192.168.*`, `0.0.0.0` and hosts ending in `.local`, `.test`, `.localhost`, `.ddev.site`, `.lndo.site`, `.docker` or `.invalid`. This check is skipped when the configured `site:` is itself a development address, since then every link would match.
- **Credentials in a body** (`content/secret`) is reported for any message but email whose stored body had a credential masked: a key or password field (`"api_key":"ab•••••"`, `DB_PASSWORD=•••••`), an Authorization-style token, a `/key/` path segment, a Telegram bot token, a private key or a key Echo knows by its shape (`sk_live_•••••`). A one-time code, a reset or magic-link token and a signed URL are masked too but don't count, so `Your code is •••••` in an SMS isn't a finding. It applies to webhooks and other API calls, not only the channels above, for example `Meta Conversions API event carries a credential in its body`.

### Email

| Check | Level | Title | Fix |
|---|---|---|---|
| `email/empty` | error | Email is empty | The template rendered nothing: check the notification's message setting and the variables it uses. |
| `email/no-subject` | warning | Email has no subject | Give the notification a subject; many filters treat mail without one as spam. |
| `email/clipped` | warning | Email is too big for Gmail and gets clipped | Trim inline CSS and markup, or move content behind links. (Gmail clips HTML over 102 KB.) |
| `email/no-text` | notice | Email has no plain-text part | Send a text alternative too; some clients and spam filters prefer it. |
| `email/from-domain` | notice | Email is sent from another domain (othermailer.com) | Send from an address on your own domain, or make sure its SPF, DKIM and DMARC cover this sender. |

The From-domain check compares the sender's registrable domain with the site's, from the public suffix list, so `mail.yoursite.com` counts as `yoursite.com` and `orders@other.co.uk` on `shop.co.uk` is another domain. It's skipped when `site:` is unset, an IP address or a development address.

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

### HTML and spam

| Check | Level | Title |
|---|---|---|
| `compat/html` | notice | Email uses display: grid, which Outlook for Windows, Gmail and Outlook.com drop |
| `compat/spam` | notice | SpamAssassin scores the email 2.1 of 5.0 (with `up --spamassassin`) |

See [Checks and rules](Echo-Checks-And-Rules#html-in-email-clients).

### Sending and recipients

| Check | Level | Title | Fix |
|---|---|---|---|
| `send/failed` | error | Email fails, Twilio SMS fails (HTTP 401) | Check the sender's settings (API key, SMTP login, endpoint URL) and the provider's error above. |
| `seal/real-recipient` | warning | Email really sent to an address outside the allowlist | Run sealed on staging (`rhythm echo up`), or add test recipients to `allow_recipients`. |

`seal/real-recipient` only applies to messages a connector really sent (observe mode or an allowed host; never mail caught by the SMTP sink or `sendmail`). It ignores test addresses: see [The allowlist](Echo-Sealing-Staging#the-allowlist).

### Actions

| Check | Level | Title | Fix |
|---|---|---|---|
| `expect/unmet` | error | Sent fewer emails than expected (detail: `email: expected 2, got 1`); Sent fewer "purchase" analytics events than expected (`analytics purchase: expected 1, got 0`) | Check the settings of whatever sends them, or update `expect` in `.echo/config.yaml` if the change is intended. |

These come from an action's `expect:`, one per channel or event short of its count; see [Actions and compare](Echo-Actions-And-Compare#expect). Comparisons add their own changes (a message no longer sent, a changed subject, a payload that lost fields); those are listed in [Reading a comparison](Echo-Actions-And-Compare#reading-a-comparison). Deliverability checks on DNS and headers are in [Deliverability](Echo-Deliverability).

---
<sub>← [Echo Inbox](Echo-Inbox) · [Home](Home) · [Checks and rules](Echo-Checks-And-Rules) →</sub>
