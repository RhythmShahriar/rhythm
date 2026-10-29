This tutorial seals a staging site and checks what a change does to the messages it sends: start Echo, connect the site, check the connection, watch messages arrive, then snapshot what each action sends, make a change and compare. It takes about ten minutes.

You need a staging or local copy of a site you own or may test. The examples use a WordPress shop at `http://staging.shop.test` on the same machine as Echo; replace it with your own. The output on this page comes from runs against that shop, a WordPress 6.8 staging copy with a contact form and a checkout that emails a receipt, texts the customer, alerts Slack and posts to GA4, Meta and a CRM. The "change" is a plugin update that went wrong.

## 1. Check the install

```sh
rhythm echo --version
```

```text
rhythm-echo 1.1.1
```

If it says `rhythm: echo isn't installed`, run `rhythm install echo`. If `rhythm` isn't found at all, follow [Install](Install).

## 2. Start Echo

Run Echo from your project's folder, the site's repository if it has one, so its snapshots stay with the project:

```sh
cd ~/projects/shop
rhythm echo up
```

The first time, Echo prints a one-time reminder:

```text
Note: use Echo on sites you own or are authorised to test. Captured messages can
hold personal data: they stay on this machine, with credentials masked. This notice is shown once.
```

Then:

```text
  ♪ rhythm echo 1.1.1                                             this machine
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          seal: nothing leaves; Echo answers for every service
  ▸ API           http://127.0.0.1:8025  (connectors post here; open it for the live view)
  ▸ SMTP          127.0.0.1:1025  (point a mailer or sendmail_path here)

  Waiting for messages. Ctrl+C to stop.
```

Leave it running in its own terminal. Echo is now **sealed**: whatever the site sends is recorded and answered by Echo. Nothing reaches a real inbox, phone or API. Open http://127.0.0.1:8025 in a browser for [Echo Inbox](Echo-Inbox), which shows each message as it arrives.

> [!NOTE]
> Echo listens on `127.0.0.1` only. If the site runs in Docker, a VM or another machine, it can't reach that address; see [Docker and DDEV](Echo-Docker-And-DDEV) and [FAQ](Echo-FAQ#the-site-runs-in-docker). If Mailpit already has port 8025, Echo says so and offers the next free pair; see [Mailpit on the same machine](Echo-Docker-And-DDEV#mailpit-on-the-same-machine).

## 3. Connect the site

Install the WordPress connector, a single mu-plugin, into the site:

```sh
rhythm echo install wordpress /var/www/staging
```

```text
Installed the Echo connector (wp-content/mu-plugins/rhythm-echo.php), reporting to http://127.0.0.1:8025.

  It runs only where WP_ENVIRONMENT_TYPE is local, development or staging. WordPress
  treats an unset WP_ENVIRONMENT_TYPE as production and there the connector does
  nothing. On staging, add this to wp-config.php, above "That's all, stop editing!":

    define( 'WP_ENVIRONMENT_TYPE', 'staging' );

  (or set it in the server's environment). On a box that isn't production but says
  so, reinstall with --force.

  Next: rhythm echo up, then rhythm echo doctor (with site: in .echo/config.yaml)
  Flush async mail and webhooks after actions: add to .echo/config.yaml
    flush: ["wp cron event run --due-now", "wp action-scheduler run"]
```

The path is the site's root (with `wp-config.php`) or its `wp-content` folder. If you can't write there, use `--wp-cli` or `--container`; Laravel, Next.js, Node and other stacks are in [Connectors](Echo-Install-Connectors). Prefer a regular plugin you can see in wp-admin? `--plugin` installs [Echo Staging Seal](Echo-Install-Connectors#echo-staging-seal) instead.

When `wp-config.php` already sets `WP_ENVIRONMENT_TYPE` to `staging`, `development` or `local`, that paragraph is replaced by `wp-config.php sets WP_ENVIRONMENT_TYPE to staging, so the connector runs.`

> [!IMPORTANT]
> The connector does nothing on a production site. WordPress treats a site **without** `WP_ENVIRONMENT_TYPE` as production. Make sure staging's `wp-config.php` has the `define()` line above (or `development` or `local`). Otherwise the connector stays off and shows an admin notice saying why.

## 4. Check every layer

Write a config with a first action for your stack, then tell Echo where the site is:

```sh
rhythm echo init --stack wordpress
```

```text
Wrote .echo/config.yaml for a WordPress site, with one action to start from (Lost password).
Set site: in it to your staging site's URL; the action's URL is relative to it.
Then: rhythm echo up, rhythm echo doctor and rhythm echo snapshot.
```

Run in the WordPress folder itself, `init` finds the stack without `--stack`. Open `.echo/config.yaml` and set `site:` to your staging URL:

```yaml
site: http://staging.shop.test
```

```sh
rhythm echo doctor
```

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

`doctor` asks the connector whether it's active and can reach Echo, then makes the site send one email and one HTTP request to addresses that can't exist (`canary@echo.invalid`, `https://canary.echo.invalid/`) and checks both arrive. It deletes the canaries afterwards. A `✗` line says what's wrong (with `--plain`, lines start with `ok`, `FAIL` or `note` instead); [FAQ](Echo-FAQ#doctor-says-a-layer-failed) goes through each one.

## 5. Watch what the site sends

In another terminal:

```sh
rhythm echo watch
```

Now use the site as a visitor would: send the contact form, place a test order. Each message appears as it's sent, with any problem Echo finds in it underneath:

```text
Watching http://127.0.0.1:8025 (seal mode). Ctrl+C to stop.
  14:22:23  ✉ email                     → owner@shop.example.com  New message from the contact form  faked
  14:22:23  ✉ email                     → jamie@shop.test  Thanks for getting in touch  faked
  14:22:23  ↗ Zapier webhook            → hooks.zapier.com/hooks/catc…  POST  faked
  14:22:23  ◆ Twilio SMS                → 15551230000  Bean & Leaf: order #1001 is confirmed. …  faked
            ! warning  Message sent to a number that isn't in international format
  14:22:23  # Slack message             → slack webhook  :shopping_trolley: New order #1001: $49…  faked
  14:22:23  ∿ GA4 event                 → G-TESTBED01  purchase  faked
  14:22:23  ∿ Meta Conversions API …    → 1234567890123456  Purchase  faked
            ! warning  Meta Conversions API event carries a credential in its body
  14:22:23  ✉ email                     → jamie@shop.test  Your Bean & Leaf order #1001  faked
  14:22:24  ↗ WooCommerce webhook       → gate:18856/woo/orders  order.created  faked
```

That order was placed with a phone number typed without its `+`, which Echo flags; the Meta event carries its access token in the body, which Echo masked and flags too. Piped or with `--plain`, each message is one line of text instead: `14:22:49 Zapier webhook to hooks.zapier.com/hooks/catch/1234567/ab•••••: POST [faked]`. The last part of a Zapier, Make or n8n hook URL is its secret, so Echo masks it everywhere it shows the URL.

`faked` means Echo answered in the service's place: the site got a normal "sent" reply and nothing left. `rhythm echo ls` lists everything captured so far; `rhythm echo show <id>` prints one message as JSON. See [Messages and checks](Echo-Messages-And-Checks).

## 6. List the actions

An **action** is something that makes the site send messages. Add yours to `.echo/config.yaml`; this is the shop's:

```yaml
site: http://staging.shop.test
flush:
  - wp --path=/var/www/staging cron event run --due-now --quiet
  - wp --path=/var/www/staging testbed-queue run --quiet
quiet: 1s               # the shop sends at once; the default is 2s

actions:
  - name: Contact form
    http: {method: POST, url: "/?rest_route=/testbed-forms/v1/contact", form: {name: "${fake:name}", email: "${fake:email}", message: Hello}}
    expect: {email: 2}
  - name: Checkout
    http: {method: POST, url: "/?rest_route=/testbed-shop/v1/checkout", form: {name: Jamie, email: jamie@shop.test, phone: "+15551230000"}}
    expect: {email: 1, sms: 1, analytics: {purchase: 1}}
```

- `http` sends a request to the site, here two form posts. URLs are relative to `site:`. `${fake:email}` and `${fake:name}` become test data that changes each run and that `compare` knows to ignore.
- `flush` runs after each action, so mail and webhooks the site queues (WP-Cron, a job queue) go out while the action is still open.
- `expect` makes the action fail when fewer messages go out: two emails from the form, an email, an SMS and a GA4 `purchase` event from the checkout.

Every request and command is tagged with the action it belongs to, so messages are never mixed up between actions. Forms in a real browser and multi-step journeys are better driven by Soundcheck; see [With Soundcheck](Echo-With-Soundcheck). Every action type is in [Actions and compare](Echo-Actions-And-Compare).

## 7. Take a snapshot

```sh
rhythm echo snapshot --name before-update
```

```text
  ♪ rhythm echo 1.1.1                                 http://staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update"
  ▸ Echo          seal mode at http://127.0.0.1:8025
  ✓ Contact form                         2 emails, 1 webhook
  ✓ Checkout                             1 email, 1 SMS, 1 chat message, 1 webhook, 2 analytics events

  WARNINGS  1 ────────────────────────────────────────────────────────────────

 ▌  WARNING    Meta Conversions API event carries a credential in its body   
 ▌             → Don't put API keys or passwords in payloads; send them in a…

 ╭──────────────────────────────────────────╮
 │  SNAPSHOT BEFORE-UPDATE   saved          │
 │  ✗ 0 errors   ▲ 1 warning   · 0 notices  │
 ╰──────────────────────────────────────────╯
  2 actions · 3 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events · 2s · seal mode

  → next: make your change, then run: rhythm echo compare
```

Each action ran in turn. Echo waited until everything `expect` asked for had arrived, then a moment with nothing new, before moving on. The snapshot is in `.echo/snapshots/before-update/` and is now the *latest* one, which `compare` uses. The problems the snapshot found in what was sent (here the token in Meta's payload) are listed above the panel: that's the starting point. `compare` reports only what changes from it.

## 8. Make your change

Update the plugin, switch the SMTP plugin, deploy the branch. In the shop, the "update" switched off the owner's notification, left a merge tag in the receipt, rewrote the order SMS with emoji and dropped the customer's email from the order webhook.

## 9. Compare

```sh
rhythm echo compare
```

```text
  ♪ rhythm echo 1.1.1                                 http://staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update", 2 actions, taken just now
  ▸ Echo          seal mode at http://127.0.0.1:8025
  ✗ Contact form                         1 email, 1 webhook, fewer than expected
  ✓ Checkout                             1 email, 1 SMS, 1 chat message, 1 webhook, 2 analytics events

  BROKE  4 ───────────────────────────────────────────────────────────────────

 ▌  BROKE      Email to owner@shop.example.com is no longer sent             
 ▌             Contact form                                                  
 ▌             sent → not sent                                               
 ▌             → Something in the change stopped this message: check the plu…

 ▌  BROKE      Sent fewer emails than expected                   Contact form
 ▌             email: expected 2, got 1                                      
 ▌             → Check the settings of whatever sends them, or update expect…

 ▌  BROKE      Unfilled placeholder {customer_first_name} in email   Checkout
 ▌             The template still contains the placeholder, so readers see i…
 ▌             → Check the tag's spelling against the plugin's list of tags.…

 ▌  BROKE      WooCommerce webhook to gate ("order.created"): payload lost f…
 ▌             Checkout                                                      
 ▌             billing.email                                                 
 ▌             → Whatever receives this (a CRM, Zapier, an app) may depend o…

  NEW  1 ─────────────────────────────────────────────────────────────────────

 ▌  NEW        SMS needs more than one part                          Checkout
 ▌             3 parts (UCS-2); each part is charged                         
 ▌             → Keep SMS under 160 GSM-7 characters (70 with emoji or other…

  CHANGED  1 ─────────────────────────────────────────────────────────────────

    ■  Twilio SMS to the test number: text changed                   Checkout
       line 1 changed: Bean & Leaf: order #1002 is confirmed. Track it at h…
         - Bean & Leaf: order #1002 is confirmed. Track it at https://shop.…
         + 🎉 Hi Jamie! Thanks so much for your order #1004 with Bean & Lea…

 ╭─────────────────────────────────────────────────╮
 │  ECHO   Something broke                         │
 │  ✗ 4 broke   ▲ 1 new   ■ 1 changed   ✓ 0 fixed  │
 ╰─────────────────────────────────────────────────╯
  2 actions compared with snapshot "before-update" (just now) · 2 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events

  → full report: .echo/runs/2026-10-25T14-23-14-f6f2df/report.html
```

It exits `1`, since something broke. Reading from the top:

- **BROKE** is what the snapshot sent and this run doesn't, or a new error. The owner's copy of the contact form is gone, so the `expect` fails too. The receipt now shows `{customer_first_name}` to the customer. The order webhook lost `billing.email`, which the CRM needs.
- **NEW** is a problem that wasn't there before: the new SMS text needs three parts, each one charged.
- **CHANGED** is the same message with different content, shown as a line diff. "The test number" is the `+1555…` test number Echo compares as a role, not a number. The order numbers differ on every run and aren't a change on their own.
- **The panel** gives the verdict and the count per class.

To see the details of every change, print the stored result in plain text:

```sh
rhythm echo report --plain
```

```text
[BROKE] Email to owner@shop.example.com is no longer sent - Contact form
  sent → not sent
  fix: Something in the change stopped this message: check the plugin or code that sends it and its settings.
[BROKE] Sent fewer emails than expected - Contact form
  email: expected 2, got 1
  fix: Check the settings of whatever sends them, or update expect in .echo/config.yaml if the change is intended.
[BROKE] Unfilled placeholder {customer_first_name} in email - Checkout
  The template still contains the placeholder, so readers see it instead of their data.
  fix: Check the tag's spelling against the plugin's list of tags. Make sure the data it needs exists when the message is sent.
[BROKE] WooCommerce webhook to gate ("order.created"): payload lost fields - Checkout
  billing.email
  fix: Whatever receives this (a CRM, Zapier, an app) may depend on these fields.
[NEW] SMS needs more than one part - Checkout
  3 parts (UCS-2); each part is charged
  fix: Keep SMS under 160 GSM-7 characters (70 with emoji or other Unicode).
[CHANGED] Twilio SMS to the test number: text changed - Checkout
  line 1 changed: Bean & Leaf: order #1002 is confirmed. Track it at https://shop.example.com/t/1… → 🎉 Hi Jamie! Thanks so much for your order #1004 with Bean & Leaf ☕️ We’re roast…
    - Bean & Leaf: order #1002 is confirmed. Track it at https://shop.example.com/t/1002
    + 🎉 Hi Jamie! Thanks so much for your order #1004 with Bean & Leaf ☕️ We’re roasting your beans now and they’ll ship tomorrow. Track it any time at https://shop.example.com/t/1004 🚚
  fix: Read the new text; if the change is intended, take a new snapshot.

Something broke - 4 broke, 1 new, 1 changed, 0 fixed - 2 actions compared, 2 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events
full report: .echo/runs/2026-10-25T14-23-14-f6f2df/report.html
```

[Actions and compare](Echo-Actions-And-Compare#reading-a-comparison) explains every class and how messages are matched.

> [!TIP]
> Fix from the top. A message that's no longer sent usually has one cause: the plugin you just updated, its settings, or the mailer it now uses.

## 10. Accept what you meant

The new SMS text was intended; its length was a known cost. Each change has a key, which `-v` prints (`key: 562b5cf7`). Pass the keys of the changes you meant to `--accept` and they leave the comparison, with a note:

```sh
rhythm echo compare --plain --accept 562b5cf7 --accept df2d8e35
```

```text
Something broke - 4 broke, 0 new, 0 changed, 0 fixed - 2 actions compared, 2 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events
note: accepted as intended: SMS needs more than one part (df2d8e35)
note: accepted as intended: Twilio SMS to the test number: text changed (562b5cf7)
```

A key stays the same from run to run until the change itself differs. When everything left is intended, take a new snapshot as the new baseline.

## 11. Open the report

Every snapshot, run and comparison writes a standalone HTML report. Open it with `--open`, or show the last result again at any time:

```sh
rhythm echo compare --open
rhythm echo report --open
```

The report shows each action with its messages: emails rendered in a sandboxed frame with remote images blocked until you load them, SMS and chat messages as bubbles and webhook bodies as they were sent, each with the answer it got. The changes are on top, text changes as diffs. It's one file, readable by you only, so you can attach it to a ticket.

## Next steps

- **Something you didn't intend?** Fix it and run `rhythm echo compare` again; it always compares with the same snapshot.
- **Everything intended?** Take a new snapshot as the new baseline: `rhythm echo snapshot --name after-update`.
- **Test real forms.** Run Soundcheck with `--echo`: [With Soundcheck](Echo-With-Soundcheck).
- **Async mail.** If messages are sent by WP-Cron, Action Scheduler or a queue, keep a `flush:` command so they go out during the action: [Actions and compare](Echo-Actions-And-Compare#flush-queued-work).
- **Want this in CI?** See [CI](Echo-CI).
- **The cookie banner too.** `rhythm echo consent https://staging.shop.test` checks that Reject really rejects; it needs no running Echo. See [Cookie consent](Echo-Consent).

---
<sub>← [Echo overview](Echo) · [Home](Home) · [Connectors](Echo-Install-Connectors) →</sub>
