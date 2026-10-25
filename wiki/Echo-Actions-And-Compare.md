An **action** is something that makes your site send messages: a form post, a WP-CLI or artisan command, a replayed webhook. This page covers the actions you list in `.echo/config.yaml` and how each one runs, how messages are tied to the action that caused them and snapshots. It also covers how `compare` matches messages, what it ignores, what each class of change means and how to accept a change you meant.

```sh
rhythm echo snapshot --name before-update   # run every action, save what each sent
rhythm echo compare                         # run them again, show what changed
rhythm echo run                             # run them once, just check the messages
```

All three need Echo running (`rhythm echo up`) and a config with at least one action, found in the folder you're in or one above it (see [Configuration](Echo-Configuration#the-config-file)). Forms in a real browser and multi-step journeys are better run by Soundcheck, which uses the same comparison; see [With Soundcheck](Echo-With-Soundcheck).

## Actions

```yaml
# .echo/config.yaml
site: https://staging.shop.test
flush: ["wp cron event run --due-now", "wp action-scheduler run"]

actions:
  - name: Contact form
    http: {method: POST, url: /wp-admin/admin-ajax.php, form: {action: contact, email: "${fake:email}", message: Hello}}
    expect: {email: 2}
  - name: Completed order
    command: wp eval 'do_action("woocommerce_order_status_completed", 123);'
    expect: {email: 1, analytics: {purchase: 1}}
    ignore: ["line_items[].meta_data", "x-request-id"]
  - name: Stripe payment succeeded
    replay: {event: payment_intent.succeeded, to: /?wc-api=wc_stripe, secret_env: ECHO_STRIPE_WEBHOOK_SECRET}
    parallel: false
```

Each action has a `name` and **exactly one** of `http`, `command` or `replay`. Mistakes stop the command before anything runs, with exit `2` and the line they're on:

```text
rhythm echo: /tmp/t2/.echo/config.yaml: line 2: action "A": set exactly one of http, command or replay
rhythm echo: /tmp/t2/.echo/config.yaml: line 4: action "contact-form" and action "Contact Form" (line 2) both get the ID action:contact-form; rename one
rhythm echo: /tmp/t2/.echo/config.yaml: line 2: action "!!!": needs a letter or digit from a to z or 0 to 9 in its name, so compare can tell it apart; add one, like "!!! 1"
rhythm echo: /tmp/t3/.echo/config.yaml: yaml: line 4: mapping values are not allowed in this context
```

Actions are matched by an ID made from the name (see [Run IDs](#run-ids)), so two names that give the same ID are refused. A name needs at least one letter or digit from `a` to `z` or `0` to `9`: `注文` is refused, `注文 checkout` works. The last error is YAML, not Echo: a value containing `: ` (a command with `Subject: …`, a PHP array) must be quoted, or written as a block with `command: |` or `command: >-`.

| Key | Description |
|---|---|
| `name` | Required, unique. |
| `http`, `command`, `replay` | What the action does; one of them. |
| `env` | Extra environment variables for a `command`. |
| `expect` | The least it must send; see [expect](#expect). |
| `ignore` | What `compare` leaves out for this action; see [ignore](#ignore). |
| `quiet` | This action's own quiet window; see [Quiet and timeout](#quiet-and-timeout). |
| `parallel` | `false` runs it alone, never beside another action; see [Parallel runs](#parallel-runs). |

### http

| Key | Default | Description |
|---|---|---|
| `url` | (required) | Absolute, or relative to `site:`. |
| `method` | `GET`, or `POST` when there's a `form`, `json` or `body` | |
| `form` | | Fields, sent as `application/x-www-form-urlencoded`. |
| `json` | | Any YAML value, sent as `application/json`. Placeholders in its strings are filled before it's encoded, so quotes and backslashes in a value stay valid JSON. |
| `body` | | A raw body. |
| `headers` | | Extra headers. |

The request carries the [run ID](#run-ids) as a header and a cookie, doesn't follow redirects and times out after `timeout`. An answer of `400` or more fails the action (`POST https://staging.shop.test/contact returned 500 Internal Server Error`), so a form that rejects the test data doesn't pass as "sent nothing". Redirects (`3xx`) are fine. A URL under one of the config's `production:` entries is refused before anything is sent; see [Production](#production). Errors show the URL with its secrets masked. Every value an `${env:NAME}` put into the request shows as `•••••`.

### command

```yaml
  - name: Weekly digest
    command: wp cron event run shop_weekly_digest
    env: {WP_CLI_STRICT_ARGS_MODE: "1"}
```

The command runs with `sh -c` (`cmd /C` on Windows, so write `%VAR%` there, not `$VAR`) in the project's folder, the one holding `.echo/`, wherever you run Echo from. `RHYTHM_ECHO_RUN` is set to the run ID and the action's `env:` added. Connectors in CLI processes (WP-CLI, artisan, Node scripts) read `RHYTHM_ECHO_RUN`, so what they send is tied to the action; a script can forward it to the site as `X-Echo-Run`. A non-zero exit fails the action, with the first 300 characters of stderr in the error. Ctrl+C stops the command's whole process tree at once. A command that leaves a background process holding its output fails with `exec: WaitDelay expired before I/O complete`: send the background job's output to `/dev/null`. For a site in Docker, pass the variable through:

```yaml
    command: docker exec -e RHYTHM_ECHO_RUN shop-cli wp eval 'wp_remote_post("https://hooks.slack.com/services/…", array("body" => "{}"));'
```

### replay

| Key | Description |
|---|---|
| `event` | A library event (`payment_intent.succeeded`, `stripe/invoice.paid`) or a path to a JSON file, relative to the project folder. |
| `to` | Where to send it: absolute, or relative to `site:`. |
| `set` | Field overrides, `data.object.amount: 1999`, applied in key order. |
| `secret_env` | The variable holding the signing secret; default per scheme, such as `ECHO_STRIPE_WEBHOOK_SECRET`. |

Replays follow the same safety rules as `rhythm echo replay`: sealed Echo, staging or local targets that aren't under `production:`, test secrets only. With Echo observing, the action fails before anything is sent: `Echo is in observe mode; replays only run sealed, so the site's reactions can't reach anyone (rhythm echo up --seal)`. A non-2xx answer fails the action. See [Replay](Echo-Replay).

### expect

```yaml
    expect: {email: 2, sms: 1, analytics: {purchase: 1}}
```

The least the action must send, per channel (`email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push`, `http`), as a count or as counts per event. When it falls short, the action gets the error `expect/unmet`, one per channel or event short of its count, which is BROKE in a comparison:

```text
[BROKE] Sent fewer emails than expected - Contact form
  email: expected 2, got 1
  fix: Check the settings of whatever sends them, or update expect in .echo/config.yaml if the change is intended.
```

An event counts only on its own channel: `analytics: {purchase: 1}` fails when the purchase event is missing, even if a `page_view` arrived (`Sent fewer "purchase" analytics events than expected`, `analytics purchase: expected 1, got 0`). With `expect`, an action is done as soon as everything it expects has arrived and nothing new comes for a moment, so it's faster than waiting for quiet. One that falls short waits up to `timeout` for the missing message. Its progress line shows `✗` and ends with `fewer than expected`. Errors (exit `2`): `line 4: expect: "mail" isn't a channel; use email, sms, whatsapp, chat, webhook, analytics, payment, push, http`.

### ignore

```yaml
    ignore: ["line_items[].meta_data", "x-request-id", "*@agency.example"]
```

What `compare` leaves out for this action before it diffs: payload fields (`order.total`, `line_items[].sku`; a field takes its children with it), header names and recipients that match. Patterns are globs (`*`, `?`, `[ ]`), case-insensitive, with `[]` meaning "every item of a list". Use it for a value that's different on purpose every run; to turn a whole check off, use `checks:` instead ([Checks and rules](Echo-Checks-And-Rules#tuning-a-check)). A pattern that doesn't parse is refused: `line 4: action "A": ignore: "items[.sku" isn't a valid pattern (unclosed [ or a trailing \)`.

### Test data and environment values

Values in a `command` and in an `http` action's URL, form, JSON, body and headers can use:

| Value | Becomes |
|---|---|
| `${fake:email}` | `echo-test+3f2a01@example.com` |
| `${fake:name}` | `echo-test 3f2a01` |
| `${fake:phone}` | a `+1555…` number |
| `${fake:tag}` | `3f2a01` |
| `${env:NAME}` | the environment variable `NAME` |

The tag is random and the same for every action in one run. `compare` recognizes these values (and Soundcheck's `soundcheck-test` data) as test data, so a new address each run isn't a change.

> [!WARNING]
> **`.echo/config.yaml` is code.** A `command` action or a `flush` entry runs in your shell on every `snapshot`, `compare` and `run`. `${env:NAME}` sends an environment variable to the URL an `http` action names. Read a config from a repository you didn't write before you run Echo in it, as you would a Makefile.

### Flush queued work

WooCommerce webhooks and many notifications are sent later, by WP-Cron, Action Scheduler or a queue worker. `flush:` lists commands Echo runs after each action, in the project folder and with the action's `RHYTHM_ECHO_RUN`, so that work is sent while the action is still open:

```yaml
flush: ["wp cron event run --due-now", "wp action-scheduler run"]      # WordPress
flush: ["php artisan queue:work --stop-when-empty"]                     # Laravel
```

A flush that fails is noted (`flush "…": exit status 1`) and doesn't fail the action. In a parallel run, one action's flush runs at a time. `rhythm echo doctor` runs each flush command once to check it works. On WordPress, `DISABLE_WP_CRON` on staging makes runs repeatable: cron then only runs when Echo flushes it.

Mail that still arrives after its action ended, tagged with that action or dated before the next one began, is counted with the action that caused it. The run says so:

```text
note: Checkout: email message e16e4deefac4 arrived 1.5 s after the action ended and is counted with it
note: Some mail arrived after its action ended; add a flush: command to .echo/config.yaml (like wp cron event run --due-now or php artisan queue:work --stop-when-empty) so queued senders send during their action
```

With `flush:` set, the second note reads `…although the flush: commands ran; check they cover every queue the site uses, or raise quiet`. Mail that arrives after the last action of a run isn't caught.

### Quiet and timeout

After triggering an action and flushing, Echo waits until nothing new has arrived for `quiet` (default `2s`), up to at most `timeout` (default `30s`), then records what came in. Raise `quiet` when messages trickle in slowly; lower it to speed up a long list of actions. An action's own `quiet:` replaces the global one for it. `--quiet 500ms` and `--timeout 10s` on `snapshot`, `compare` and `run` replace both for one run (`-q` alone still means "results only").

Echo's limits: `quiet` at most `30s` and `timeout` at most `1m`, with quiet no longer than the timeout. A value outside them is refused, in the config or on the command line:

```text
rhythm echo: /tmp/t2/.echo/config.yaml: line 1: quiet: 45s is over the 30s Echo waits for quiet at most
rhythm echo: /tmp/t2/.echo/config.yaml: line 2: quiet: 20s is longer than the timeout of 10s (line 1), so no action could ever fall quiet
rhythm echo: --quiet 45s: over the 30s Echo waits for quiet at most
```

### How an action runs

1. Echo opens the action with a lease of its own: messages that arrive without a run ID are attributed to it.
2. It triggers the action, tagged with the run ID.
3. It runs each `flush` command.
4. It waits, as above.
5. It closes its lease and checks each message it caught (see [Messages and checks](Echo-Messages-And-Checks#checks)).

A lease that isn't closed (a runner killed mid-action) runs out after 10 minutes; a long action renews its lease every 3 minutes. Two runs sharing one Echo, such as two CI jobs, never close each other's action; plain SMTP mail with no run ID that arrives while both are open is marked ambiguous and the run says so (`1 message could not be tied to an action because another run was open`).

Actions run in the order of the file. `--action <name>` (repeatable, ignoring case) runs only some of them; a name that isn't in the config is a usage error, `no action named "x"; the actions are: Contact form, Checkout`:

```sh
rhythm echo compare --action "Order shipped" --action "Weekly digest"
```

```text
note: 1 action from the snapshot wasn't run this time
```

### Parallel runs

`--parallel <n>` runs up to `n` actions at once. Echo widens only when it's safe: actions run one by one until a message from a connector shows up tagged with this run (WordPress, Laravel and Node tag every message), then up to `n` at once. If the first action sent nothing tagged, the rest run one by one with an exclusive lease, so untagged mail (plain SMTP, `sendmail`) can't be credited to the wrong action. An action with `parallel: false` waits for every other one to end and runs alone. Results stay in the file's order. Against an Echo without leases, actions run one by one, with a note: `actions ran one by one: running them in parallel needs a newer Echo that keeps a lease per action`.

### Production

`production:` lists your live sites, as hosts or URLs: `["shop.com"]` covers `https://shop.com`, `http://www.shop.com` and any other subdomain, with any scheme on the default ports. An entry with a port or a path matches only that port or path. `snapshot`, `compare` and `run` refuse a `site:` under it before they contact anything (exit `3`). Each `http` and `replay` action refuses a URL under it:

```text
rhythm echo: refusing to run against http://staging.shop.test: it matches "shop.test" under production; actions only run against staging and local sites
```

`doctor` fails on the same site; [Replay](Echo-Replay#safety-rules) refuses it too.

## Run IDs

Every action gets an ID of the form `<run>/<action>`, such as `2026-10-24T08-29-07-cd7ff3/action:contact-form`: the run's start time with a random suffix, then `action:` and the action's name, lowercased, with runs of other characters turned into `-`. Renaming an action makes it a different action to `compare`.

The ID reaches the site in several ways. Connectors look in this order:

1. **The `X-Echo-Run` header** on the request that triggers the work. Echo's `http` actions and Soundcheck send it.
2. **The `X-Echo-Run` cookie**, the same value URL-encoded, for handlers that read cookies but not headers.
3. **Queued work** carries it on: the WordPress connector stores it with each WP-Cron event and Action Scheduler action scheduled during the request, Laravel keeps it in the job's Context and the Node connector in `withEchoRun()` scopes.
4. **`RHYTHM_ECHO_RUN`** in the environment, for commands and workers.

When Echo knows a token, the header and cookie carry a signature (`<run>/<action>.<sig>`). A connector with the same token ignores a value without it, so a visitor can't add messages to a snapshot or take them out. See [Run IDs and signatures](Echo-Install-Connectors#run-ids-and-signatures).

A message with none of these still lands in the action that's open when it arrives, which covers plain SMTP mail and a cron job Echo flushed. Messages that arrive outside any action (`rhythm echo watch` while you click around) belong to no run.

A tool of your own can do the same: send `X-Echo-Run: <run>/<action>` on your requests, or open and close the action through the API (`POST /v1/actions/begin`, `POST /v1/actions/end`); see [API](Echo-API).

## Snapshots

`snapshot` runs every action and saves the run as `.echo/snapshots/<name>/run.json`, with a `report.html`. Without `--name`, the name is the date and time, such as `2026-10-24T08-19-22`. Names use up to 64 letters, digits, dots, dashes and underscores, starting with a letter or digit. `latest` (in any case) is taken, as are Windows device names (`CON`, `NUL`, `COM1`…) and names ending in a dot: `rhythm echo: --name "latest": use letters, digits, dots, dashes and underscores (LATEST is taken)`. Names that differ only in case are one snapshot. A snapshot with the same name is replaced once the new one is complete; the newest snapshot becomes the *latest*, the one `compare` uses.

```sh
rhythm echo snapshots
```

```text
NAME                     ACTIONS  TAKEN        SENT
before-update            2        2m ago       3 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events  (latest)
```

`rhythm echo compare --snapshot <name>` compares with another one. `rhythm echo snapshots delete <name>` (or `rm`) removes one. Deleting the latest snapshot makes the newest remaining one, by the time it was taken, the latest:

```text
$ rhythm echo snapshots delete after-update
Deleted snapshot after-update
The latest snapshot is now before-update
```

A snapshot is written to `.echo/snapshots/.incoming-<name>` while it runs and moved into place when it's complete, so a snapshot that fails leaves nothing behind. When every action failed, nothing is saved and the old snapshot of that name stays the latest (exit `3`):

```text
[1/1] Down: nothing sent (failed: Get "http://127.0.0.1:9/contact": dial tcp 127.0.0.1:9: connect: connection refused)
rhythm echo: not saved: every action failed; fix the site or pass --force
```

`--force` saves it anyway (`rhythm echo: every action failed; saved anyway because of --force`, still exit `3`).

A snapshot reports the problems the site has now (an unfilled placeholder already in a template), so you know the starting point; `compare` then reports only what changed.

### Baselines that travel

A snapshot taken in one place can be compared in another, such as the base branch's CI job and the pull request's:

```sh
rhythm echo snapshot --export base.tgz          # the latest snapshot, or --name one
```

```text
Exported snapshot before-update to base.tgz. Elsewhere: rhythm echo compare --against base.tgz
```

```sh
rhythm echo compare --against base.tgz          # compare with it directly
rhythm echo snapshot --import base.tgz --name from-main   # or add it as a snapshot
```

```text
Imported snapshot from-main from base.tgz; compare uses it now.
```

`--against` takes a `.tgz` from `--export`, a run or snapshot folder or a `run.json`. `--import` makes the snapshot the latest; one of the same name is refused (`rhythm echo: snapshot before-update already exists (--force replaces it, --name imports it under another name)`). An archive is checked before anything is unpacked: only `run.json` and `comparison.json` are kept. Paths that point outside, links, more than one snapshot, more than 100 entries or 128 MB are refused with `not a snapshot archive: <why>`. `--export -` writes to stdout. Export and import run no actions, so they take only `--name`, `--force`, `--config` and `--store`.

## Reading a comparison

`compare` runs the actions again from the current config, matches each action by its ID and diffs the messages each one sent. An action the snapshot doesn't have is run too. Everything it sends is listed as NEW, without checking those messages for problems (`run` shows them); actions missing this time are noted.

### The classes

| Class | Means | Changes |
|---|---|---|
| **BROKE** | Worked in the snapshot, fails or is missing now | a message is no longer sent; a message now fails; the action itself fails; a webhook or payment payload lost fields; an attachment is gone or empty; a new error in a message (an unfilled placeholder, a PHP warning, a failed `expect`) |
| **NEW** | Wasn't there before | a message is now sent; a new attachment; a new warning or notice |
| **CHANGED** | The same message with differences: check it's intended | recipients' copies, subject, sender, Reply-To, text, event, payload fields or values, an attachment's type or size, fewer or more sends |
| **FIXED** | A problem from the snapshot is gone | an error, warning or notice that's no longer reported |

| Check | Class | Title | Before → after |
|---|---|---|---|
| `message/missing` | BROKE | Email to owner@shop.example.com is no longer sent | `sent` → `not sent` |
| `message/failing` | BROKE | Twilio SMS to the test number now fails | `faked` → `failed` |
| `run/action-failed` | BROKE | Action fails, Action still fails | the error |
| `message/new` | NEW | Email to office@shop.test is now sent | the detail is its subject or text |
| `message/fewer` | CHANGED | Email to the test submitter is sent fewer times | `2 times` → `1 time` |
| `message/more` | CHANGED | Email to team@shop.test ("Weekly digest") is sent more times | `1 time` → `2 times`; fix: check for a hook or plugin that fires twice |
| `message/subject` | CHANGED | Email to the test submitter: subject changed | the subjects as sent |
| `message/from` | CHANGED | …: sender changed | the senders; fix: check it with `rhythm echo check <domain>` |
| `message/copies` | CHANGED | …: copies (Cc/Bcc) changed | the copies |
| `message/reply-to` | CHANGED | …: lost its Reply-To | the old Reply-To |
| `message/text` | CHANGED | Twilio SMS to the test number: text changed | a line diff |
| `message/event` | CHANGED | …: event changed | the events |
| `message/fields-lost` | BROKE for webhooks and payments, CHANGED otherwise | WooCommerce webhook to gate ("order.created"): payload lost fields | the fields, e.g. `billing.email` |
| `message/fields-added` | CHANGED | …: payload has new fields | the fields |
| `message/payload` | CHANGED | …: payload changed | a diff of the JSON, when the fields are the same and a value isn't |
| `message/attachment-lost`, `-changed`, `-new` | see [Attachments](Echo-Attachments#how-theyre-compared) | Email to jamie@shop.test no longer attaches invoice-1042.pdf | the file → `not attached` |

Issues found in messages (see [Messages and checks](Echo-Messages-And-Checks#checks)) keep their own check names: a new `content/placeholder` error is BROKE, a new `email/no-subject` warning is NEW. When a message gets a new placeholder or PHP error, its text change isn't listed again.

A changed text is shown as a line diff of what a reader sees (for an HTML-only email, its visible text), with the first place that changed under the title and `-v` showing every place:

```text
[CHANGED] Twilio SMS to the test number: text changed - Checkout
  line 1 changed: Bean & Leaf: order #1002 is confirmed. Track it at https://shop.example.com/t/1… → 🎉 Hi Jamie! Thanks so much for your order #1004 with Bean & Leaf ☕️ We’re roast…
    - Bean & Leaf: order #1002 is confirmed. Track it at https://shop.example.com/t/1002
    + 🎉 Hi Jamie! Thanks so much for your order #1004 with Bean & Leaf ☕️ We’re roasting your beans now and they’ll ship tomorrow. Track it any time at https://shop.example.com/t/1004 🚚
  fix: Read the new text; if the change is intended, take a new snapshot.
```

### When an action fails

An action that fails in this run is BROKE `run/action-failed` and what it would have sent isn't compared, so the verdict says the comparison is incomplete instead of claiming nothing changed:

```text
[BROKE] Action fails - Order
  "test ! -f /tmp/fail && printf \"To: ann@shop.test\\nSubject: Order received\\n\\nThanks.\\n\" | rhythm-echo sendmail -t -i\n": exit status 1
  fix: The request that triggers the messages failed, so nothing it sends was compared: fix that first.

Could not compare: 1 of 2 actions failed in this run - 1 broke, 0 new, 0 changed, 0 fixed - 2 actions run, 1 email
```

It's `Action still fails` when it failed in the snapshot too; one that failed in the snapshot and works now is FIXED, with the note `"<action>" failed in the snapshot, so what it sends now wasn't compared: take a new snapshot`. When every action fails, `compare` exits `3`: `rhythm echo: every action failed, so nothing was checked; is the site up?`.

A site whose connector can't reach Echo fails its sends while sealed, so the action fails and nothing arrives. Then each message the connector sent in the snapshot is listed as BROKE `message/missing` too, with a fix that says where to look: `A connector that can't reach Echo blocks every send while sealed: check the site reaches Echo with rhythm echo doctor`.

### How messages are matched

Two messages are *the same message* when they have the same **identity**:

- email, SMS and WhatsApp: the channel, the service and the recipients;
- everything else: the channel, the provider, the method, the host, the path with IDs replaced by `{id}` (`/v1/orders/123` and `/v1/orders/456` match) and the event.

Several messages can share an identity, such as a welcome email and an account email to the same address. Within an identity, messages are paired by subject (and event) first, then whatever is left in order, so a changed subject reads as a change rather than one message lost and another new. Each pair is compared for content. A message left over in the snapshot is **no longer sent** (`sent` → `not sent`). One left over in this run is **now sent**, unless it's the same message as one already paired, which reads as **sent fewer times** or **sent more times**. When several messages share an identity, titles add the subject: `Email to team@shop.test ("Weekly digest") is sent more times`. A Zapier, Make or n8n hook is named by its masked URL, since each URL is its own Zap or scenario.

Before anything is compared, what varies on every run is normalized away:

| What | Becomes |
|---|---|
| `echo-test…@…` and `soundcheck-test…@…` addresses | "the test submitter" |
| `+1555` followed by seven digits and `+15005550…` numbers | "the test number" |
| The subject | numbers replaced by `#` (`#1042` becomes `#`) and test names by `{name}`, with case ignored, so `Order #1041` and `Order #1042` match |
| The text | dates and times in the usual forms, order, invoice, receipt, booking and ticket numbers after those words, coupon codes, UUIDs, long tokens, test data and whitespace; digits on their own don't count, so a moved price isn't a change either |
| A JSON payload | its field paths (`order.total`, `line_items[].sku`); a form body, its keys. Values are diffed as indented JSON for `message/payload`. |

So a new order number in the receipt, a fresh test address or a different timestamp isn't a change; a different admin address, a new subject line or a lost payload field is. A link to the staging host in a line that's otherwise the same is reported once, as the `content/dev-link` finding, not also as a text change. A payload cut to the size cap isn't compared field by field: `<message>: payload too large to compare fields (cut at 256 KiB)`.

### Accepting a change

Each change has a key, eight hex characters that stay the same from run to run until the change itself differs. `-v` prints it under each change (`key: 562b5cf7`). `--accept <key>` (repeatable) takes a change as intended: it leaves the comparison and a note says so.

```sh
rhythm echo compare --plain --accept 562b5cf7 --accept df2d8e35 --accept 0000beef
```

```text
rhythm echo: --accept 0000beef matched no change

…
note: accepted as intended: SMS needs more than one part (df2d8e35)
note: accepted as intended: Twilio SMS to the test number: text changed (562b5cf7)
```

A key that matched nothing is reported on stderr (as above) and doesn't stop the run.

### Grouping and order

The same change in several actions is one line, listing every action it affects. Changes are sorted by class, then by how many actions they affect, then by title. At the end, the panel gives the verdict (*Something broke*, *New messages or problems*, *Everything is still sent the same way*, *Could not compare*) and the count per class.

Notes below the panel say what limits the result: `the snapshot ran in seal mode and this run in observe mode`, `1 action from the snapshot wasn't run this time` (`2 actions from the snapshot weren't run this time`), `1 action isn't in the snapshot; what it sent is listed as new` (`2 actions aren't in the snapshot; what they sent is listed as new`). A run during which Echo restarted says so in a warning that starts `Echo restarted during the run;` and ends `run again with Echo up throughout`, since messages it held in memory may be gone.

### Failing a build

Without `--fail-on`, `compare` exits `1` on a BROKE change and `run` and `snapshot` on an error. `--fail-on broke|new|changed` sets the class for a comparison (`new` fails on BROKE or NEW, `changed` on anything but FIXED); `--fail-on error|warning|notice` sets the level for a run or snapshot. Any case works. An action that fails counts at every level. `--fail-on none` never fails on findings or failed actions; an action that can't run at all still exits `3`. A value of the wrong kind is a usage error:

```text
rhythm echo: --fail-on "error": want none, broke, new or changed
rhythm echo: --fail-on "broke": want none, error, warning or notice (or broke, new, changed when comparing)
```

See [CI](Echo-CI).

## Reports

Every snapshot, run and comparison is stored in `.echo/` with a standalone `report.html`. `report` renders a stored result again without running anything:

```sh
rhythm echo report                  # the newest run, comparison or snapshot
rhythm echo report before-update    # a snapshot
rhythm echo report .echo/runs/2026-10-25T14-23-14-f6f2df
rhythm echo report --md -o echo.md
rhythm echo report --junit -o echo-junit.xml --fail-on new
rhythm echo report --open           # open the HTML report
```

| Format | Flag | |
|---|---|---|
| Terminal | (default) | The live view, styled; `--plain` (or any output that isn't a terminal) gives plain text with every detail. |
| HTML | `--html`, `-o report.html` | One file: actions with their messages (emails in a sandboxed frame with remote images blocked until you click **Load remote images**, SMS and chat as bubbles, payloads as sent and each answer), changes on top with their diffs. |
| Markdown | `--md` | The verdict, a count table, then a table per class with the change, actions, before → after and fix, plus a collapsed diff per text or payload change and a table of attachments. Fits a pull request comment or a job summary. |
| JSON | `--json` | The JSON envelope with the comparison or the run in `data`; see [CLI reference](Echo-CLI-Reference#json-output). |
| JUnit | `--junit` | A test suite per class (`echo.broke`, `echo.new`…) and a test case per change; cases at or above `--fail-on` (BROKE only without it) are failures. |

`-o <file>` picks the format from the extension (`.html`, `.md`, `.json`, `.xml`) and still prints the terminal view; a format flag without `-o` prints only that format on stdout. Reports and the files under `.echo/runs/` and `.echo/snapshots/` are written readable by you only (`0600`, folders `0700`). `report` of a comparison exits `1` when something broke, as `compare` does. Runs and comparisons go in `.echo/runs/<date-time>-<id>/`; the 20 newest are kept. Snapshots are never pruned.

---
<sub>← [Attachments](Echo-Attachments) · [Home](Home) · [With Soundcheck](Echo-With-Soundcheck) →</sub>
