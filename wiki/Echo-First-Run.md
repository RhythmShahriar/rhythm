This tutorial seals a staging site and checks what a change does to the messages it sends: start Echo, connect the site, check the connection, watch messages arrive, then snapshot what each action sends, make a change and compare. It takes about ten minutes.

You need a staging or local copy of a site you own or may test. The examples use a WordPress site at `https://staging.shop.test` on the same machine as Echo; replace it with your own. The output on this page comes from runs against a WordPress staging container and a small test shop, where the "change" is a plugin update that went wrong.

## 1. Check the install

```sh
rhythm echo --version
```

```text
rhythm-echo 1.0.0
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
  ♪ rhythm echo 1.0.0                                             this machine
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          seal: nothing leaves; Echo answers for every service
  ▸ API           http://127.0.0.1:8025  (connectors post here; open it for the live view)
  ▸ SMTP          127.0.0.1:1025  (point a mailer or sendmail_path here)

  Waiting for messages. Ctrl+C to stop.
```

Leave it running in its own terminal. Echo is now **sealed**: whatever the site sends is recorded and answered by Echo, and nothing reaches a real inbox, phone or API. Open http://127.0.0.1:8025 in a browser for a live view that refreshes every five seconds.

> [!NOTE]
> Echo listens on `127.0.0.1` only. If the site runs in Docker, a VM or another machine, it can't reach that address; see [FAQ](Echo-FAQ#the-site-runs-in-docker).

## 3. Connect the site

Install the WordPress connector, a single mu-plugin, into the site:

```sh
rhythm echo install wordpress /var/www/staging
```

```text
Installed the Echo connector (wp-content/mu-plugins/rhythm-echo.php), reporting to http://127.0.0.1:8025.

  It stays off on production (WP_ENVIRONMENT_TYPE=production) unless forced.
  Next: rhythm echo up, then rhythm echo doctor (with site: in .echo/config.yaml)
  Flush async mail and webhooks after actions: add to .echo/config.yaml
    flush: ["wp cron event run --due-now", "wp action-scheduler run"]
```

The path is the site's root (with `wp-config.php`) or its `wp-content` folder. If you can't write there, use `--wp-cli` or `--container`; Laravel, Next.js, Node and other stacks are in [Connectors](Echo-Install-Connectors).

> [!IMPORTANT]
> The connector does nothing on a production site, and WordPress treats a site **without** `WP_ENVIRONMENT_TYPE` as production. Make sure staging's `wp-config.php` has `define( 'WP_ENVIRONMENT_TYPE', 'staging' );` (or `development` or `local`). Otherwise the connector stays off and shows an admin notice saying why.

## 4. Check every layer

Tell Echo where the site is, then let `doctor` send a test message through each layer:

```sh
rhythm echo init
```

```text
Wrote .echo/config.yaml. Every setting is optional; add actions to snapshot and compare.
```

Open `.echo/config.yaml` and set `site:` to your staging URL:

```yaml
site: https://staging.shop.test
```

```sh
rhythm echo doctor
```

```text
  ♪ rhythm echo 1.0.0                                                   doctor
  ────────────────────────────────────────────────────────────────────────────

  ▸ ✓ Echo        seal mode at http://127.0.0.1:8025, 35 providers recognized
  ▸ ✓ SMTP        127.0.0.1:1025 receives mail
  ▸ ✓ WordPress   active (staging), layers: wp_mail, pre_http_request
  ▸ ✓ Canaries    wp_mail and wp_remote_post were both caught

  4 checks passed, 0 failed.
```

`doctor` asks the connector whether it's active and can reach Echo, then makes the site send one email and one HTTP request to addresses that can't exist (`canary@echo.invalid`, `https://canary.echo.invalid/`) and checks both arrive. It deletes the canaries afterwards. A `✗` line says what's wrong; [FAQ](Echo-FAQ#doctor-says-a-layer-failed) goes through each one.

## 5. Watch what the site sends

In another terminal:

```sh
rhythm echo watch
```

Now use the site as a visitor would: send the contact form, place a test order. Each message appears as it's sent:

```text
Watching http://127.0.0.1:8025 (seal mode). Ctrl+C to stop.
08:19:45 email to office@shop.test: New message from Jamie [faked]
08:19:45 email to jamie@shop.test: Thanks for getting in touch [faked]
08:19:45 Slack message to slack webhook: New contact form message from Jamie [faked]
08:19:45 Twilio SMS to +15551230000: Hi Jamie, your order #1042 has shipped. [faked]
```

`[faked]` means Echo answered in the service's place: the site got a normal "sent" reply, and nothing left. `rhythm echo ls` lists everything captured so far, and `rhythm echo show <id>` prints one message as JSON. See [Messages and checks](Echo-Messages-And-Checks).

## 6. List the actions

An **action** is something that makes the site send messages. Add yours to `.echo/config.yaml`; this is the test shop's:

```yaml
site: https://staging.shop.test
quiet: 1s          # the test shop sends at once; the default is 2s

actions:
  - name: Contact form
    http: {method: POST, url: /contact, form: {name: "${fake:name}", email: "${fake:email}", message: Hello}}
    expect: {email: 2}
  - name: Order shipped
    http: {method: POST, url: /order, form: {name: Jamie, email: jamie@shop.test}}
  - name: Weekly digest
    command: |
      printf 'To: team@shop.test\nSubject: Weekly digest\n\n3 new orders this week.\n' | rhythm-echo sendmail -t
```

- `http` sends a request to the site, here two form posts. URLs are relative to `site:`. `${fake:email}` and `${fake:name}` become test data that changes each run and that `compare` knows to ignore.
- `command` runs a shell command, such as `wp eval '…'` or `php artisan …`.
- `expect: {email: 2}` makes the action fail when fewer than two emails go out.

Every request and command is tagged with the action it belongs to, so messages are never mixed up between actions. Forms in a real browser and multi-step journeys are better driven by Soundcheck; see [With Soundcheck](Echo-With-Soundcheck). Every action type is in [Actions and compare](Echo-Actions-And-Compare).

## 7. Take a snapshot

```sh
rhythm echo snapshot --name before-update
```

```text
  ♪ rhythm echo 1.0.0                                https://staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update"
  ▸ Echo          seal mode at http://127.0.0.1:8025
  ✓ Contact form                         2 emails, 1 chat message
  ✓ Order shipped                        1 SMS, 1 webhook
  ✓ Weekly digest                        1 email
 ╭──────────────────────────────────╮
 │  SNAPSHOT BEFORE-UPDATE   saved  │
 │  ✓ no problems in what was sent  │
 ╰──────────────────────────────────╯
  3 actions · 3 emails, 1 SMS, 1 chat message, 1 webhook · 3s · seal mode

  → next: make your change, then run: rhythm echo compare
```

Each action ran in turn, and Echo waited until nothing new had arrived for `quiet` (one second here) before moving on. The snapshot is in `.echo/snapshots/before-update/` and is now the *latest* one, which `compare` uses. If the snapshot found problems in what was sent (an unfilled placeholder, a PHP warning in an email), they're listed above the panel: that's the starting point.

## 8. Make your change

Update the plugin, switch the SMTP plugin, deploy the branch. In the test shop, the "update" stopped the admin notification, changed the autoresponder's subject, broke the SMS template and dropped a field from the Zapier webhook.

## 9. Compare

```sh
rhythm echo compare
```

```text
  ♪ rhythm echo 1.0.0                                https://staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Snapshot      "before-update", 3 actions, taken just now
  ▸ Echo          seal mode at http://127.0.0.1:8025
  ✓ Contact form                         1 email, 1 chat message
  ✓ Order shipped                        1 SMS, 1 webhook
  ✓ Weekly digest                        1 email

  BROKE  3 ───────────────────────────────────────────────────────────────────

 ▌  BROKE      Email to office@shop.test is no longer sent       Contact form
 ▌             1 → 0
 ▌             → Something in the change stopped this message: check the plu…

 ▌  BROKE      Sent fewer emails than expected                   Contact form
 ▌             expected 2, got 1
 ▌             → Check the settings of whatever sends them, or update expect…

 ▌  BROKE      Unfilled placeholder {first_name} in Twilio SMS  Order shipped
 ▌             The template still contains the placeholder, so readers see i…
 ▌             → Check the tag's spelling against the plugin's list of tags,…

  CHANGED  3 ─────────────────────────────────────────────────────────────────

    ■  Email to the test submitter: subject changed              Contact form
    ■  Twilio SMS to the test number: text changed              Order shipped
    ■  Webhook to hooks.zapier.com: payload lost fields         Order shipped

 ╭─────────────────────────────────────────────────╮
 │  ECHO   Something broke                         │
 │  ✗ 3 broke   ▲ 0 new   ■ 3 changed   ✓ 0 fixed  │
 ╰─────────────────────────────────────────────────╯
  3 actions compared with snapshot "before-update" (just now) · 2 emails, 1 SMS, 1 chat message, 1 webhook

  → full report: .echo/runs/2026-10-24T08-37-26/report.html
```

Reading from the top:

- **BROKE** is what the snapshot sent and this run doesn't. The office's copy of the contact form is gone, so the `expect` fails too. The SMS now has an unfilled `{first_name}`, which is a new error.
- **CHANGED** is the same message, but different. "The test submitter" is the `${fake:email}` address, which changes every run; Echo compares it as a role, not an address. The same goes for `+1555…` numbers, which are test numbers. The Zapier payload lost a field that whatever receives it may need.
- **The panel** gives the verdict and the count per class.

To see the details of every change, including the before and after values, print the stored result in plain text:

```sh
rhythm echo report --plain
```

```text
[CHANGED] Email to the test submitter: subject changed - Contact form
  thanks for getting in touch → we got your message
[CHANGED] Twilio SMS to the test number: text changed - Order shipped
  Hi Jamie, your order #1042 has shipped. → Hi {first_name}, your order #1042 has shipped.
  fix: Read the new text; if the change is intended, take a new snapshot.
[CHANGED] Webhook to hooks.zapier.com: payload lost fields - Order shipped
  order.email
  fix: Whatever receives this (a CRM, Zapier, an app) may depend on these fields.
```

[Actions and compare](Echo-Actions-And-Compare#reading-a-comparison) explains every class and how messages are matched.

> [!TIP]
> Fix from the top. A message that's no longer sent usually has one cause: the plugin you just updated, its settings, or the mailer it now uses.

## 10. Open the report

Every snapshot, run and comparison writes a standalone HTML report. Open it with `--open`, or show the last result again at any time:

```sh
rhythm echo compare --open
rhythm echo report --open
```

The report shows each action with its messages: emails rendered in a sandboxed frame, SMS and chat messages as bubbles, webhook bodies as they were sent, each with the answer it got, and the changes on top. It's one file, so you can attach it to a ticket.

## Next steps

- **Something you didn't intend?** Fix it and run `rhythm echo compare` again; it always compares with the same snapshot.
- **Everything intended?** Take a new snapshot as the new baseline: `rhythm echo snapshot --name after-update`.
- **Test real forms.** Run Soundcheck with `--echo`: [With Soundcheck](Echo-With-Soundcheck).
- **Async mail.** If messages are sent by WP-Cron, Action Scheduler or a queue, add a `flush:` command so they go out during the action: [Actions and compare](Echo-Actions-And-Compare#flush-queued-work).
- **Want this in CI?** See [CI](Echo-CI).

---
<sub>← [Echo overview](Echo) · [Home](Home) · [Connectors](Echo-Install-Connectors) →</sub>
