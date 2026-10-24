An **action** is something that makes your site send messages: a form post, a WP-CLI or artisan command, a replayed webhook. This page covers the actions you list in `.echo/config.yaml` and how each one runs, how messages are tied to the action that caused them, snapshots, and how `compare` matches messages, what it ignores, and what each class of change means.

```sh
rhythm echo snapshot --name before-update   # run every action, save what each sent
rhythm echo compare                         # run them again, show what changed
rhythm echo run                             # run them once, just check the messages
```

All three need Echo running (`rhythm echo up`) and at least one action. Forms in a real browser and multi-step journeys are better run by Soundcheck, which uses the same comparison; see [With Soundcheck](Echo-With-Soundcheck).

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
  - name: Stripe payment succeeded
    replay: {event: payment_intent.succeeded, to: /?wc-api=wc_stripe, secret_env: ECHO_STRIPE_WEBHOOK_SECRET}
```

Each action has a `name`, unique in the file, and **exactly one** of `http`, `command` or `replay`. Mistakes stop the command before anything runs:

```text
rhythm echo: action "Contact form": set exactly one of http, command or replay
rhythm echo: .echo/config.yaml: yaml: line 13: mapping values are not allowed in this context
```

The second one is YAML, not Echo: a value containing `: ` (a command with `Subject: …`, a PHP array) must be quoted, or written as a block with `command: |` or `command: >-`.

### http

| Key | Default | Description |
|---|---|---|
| `url` | (required) | Absolute, or relative to `site:`. |
| `method` | `GET`, or `POST` when there's a `form`, `json` or `body` | |
| `form` | | Fields, sent as `application/x-www-form-urlencoded`. |
| `json` | | Any YAML value, sent as `application/json`. |
| `body` | | A raw body. |
| `headers` | | Extra headers. |

The request carries the [run ID](#run-ids) as a header and a cookie, doesn't follow redirects, and times out after `timeout`. A `5xx` answer fails the action (`POST https://staging.shop.test/contact returned 500 Internal Server Error`); `4xx` doesn't, since many forms answer `400` or `422` on purpose. A URL that starts with one of the config's `production:` prefixes is refused: `refusing to run against https://shop.test/…: it's listed under production`.

<!-- TODO(lead): an uncommitted change in runner/runner.go makes 4xx fail http actions too; update this sentence if it lands. -->

### command

```yaml
  - name: Weekly digest
    command: wp cron event run shop_weekly_digest
    env: {WP_CLI_STRICT_ARGS_MODE: "1"}
```

The command runs with `sh -c` in the folder you run Echo from, with `RHYTHM_ECHO_RUN` set to the run ID and the action's `env:` added. Connectors in CLI processes (WP-CLI, artisan, Node scripts) read `RHYTHM_ECHO_RUN`, so what they send is tied to the action. A non-zero exit fails the action, with the first 300 characters of stderr in the error. For a site in Docker, pass the variable through:

```yaml
    command: docker exec -e RHYTHM_ECHO_RUN shop-cli wp eval 'wp_remote_post("https://hooks.slack.com/services/…", array("body" => "{}"));'
```

### replay

| Key | Description |
|---|---|
| `event` | A library event (`payment_intent.succeeded`, `stripe/invoice.paid`) or a path to a JSON file. |
| `to` | Where to send it: absolute, or relative to `site:`. |
| `set` | Field overrides, `data.object.amount: 1999`, applied in key order. |
| `secret_env` | The variable holding the signing secret; default per scheme, such as `ECHO_STRIPE_WEBHOOK_SECRET`. |

Replays follow the same safety rules as `rhythm echo replay`: sealed Echo, staging or local targets, test secrets only. A non-2xx answer fails the action. See [Replay](Echo-Replay).

### expect

```yaml
    expect: {email: 2, sms: 1}
```

The action must send at least this many messages per channel; `email`, `sms`, `webhook` and `chat` can be set. When it doesn't, the action gets the error *Sent fewer emails than expected* (`expected 2, got 1`), which is BROKE in a comparison. With `expect`, Echo waits until enough messages arrived before it starts waiting for quiet, so an action that falls short takes the full `timeout`.

### Test data and environment values

Values in an `http` action's URL, form, JSON, body and headers, and in a `command`, can use:

| Value | Becomes |
|---|---|
| `${fake:email}` | `echo-test+3f2a01@example.com` |
| `${fake:name}` | `echo-test 3f2a01` |
| `${fake:phone}` | a `+1555…` number |
| `${fake:tag}` | `3f2a01` |
| `${env:NAME}` | the environment variable `NAME` |

The tag is random and the same for every action in one run. `compare` recognizes these values (and Soundcheck's `soundcheck-test` data) as test data, so a new address each run isn't a change.

### Flush queued work

WooCommerce webhooks and many notifications are sent later, by WP-Cron, Action Scheduler or a queue worker. `flush:` lists commands Echo runs after each action, with the action's `RHYTHM_ECHO_RUN`, so that work is sent while the action is still open:

```yaml
flush: ["wp cron event run --due-now", "wp action-scheduler run"]      # WordPress
flush: ["php artisan queue:work --stop-when-empty"]                     # Laravel
```

A flush that fails is noted (`flush "…": exit status 1`) and doesn't fail the action. `rhythm echo doctor` runs each flush command once to check it works. On WordPress, `DISABLE_WP_CRON` on staging makes runs repeatable: cron then only runs when Echo flushes it.

### Quiet and timeout

After triggering an action and flushing, Echo waits until nothing new has arrived for `quiet` (default `2s`), and at most `timeout` (default `30s`), then records what came in. Raise `quiet` when messages trickle in slowly; lower it to speed up a long list of actions.

### How an action runs

1. Echo opens the action: messages that arrive without a run ID are attributed to it.
2. It triggers the action, tagged with the run ID.
3. It runs each `flush` command.
4. It waits for quiet, as above.
5. It closes the action, and checks each message it caught (see [Messages and checks](Echo-Messages-And-Checks#checks)).

Actions run one at a time, in the order of the file. `--action <name>` (repeatable, ignoring case) runs only some of them:

```sh
rhythm echo compare --action "Order shipped" --action "Weekly digest"
```

```text
note: 1 actions from the snapshot weren't run this time
```

## Run IDs

Every action gets an ID of the form `<run>/<action>`, such as `2026-10-24T08-29-07-cd7ff3/action:contact-form`: the run's start time with a random suffix, then the action's name, lowercased, with runs of other characters turned into `-`. Renaming an action makes it a different action to `compare`.

The ID reaches the site in several ways, and connectors look in this order:

1. **The `X-Echo-Run` header** on the request that triggers the work. Echo's `http` actions and Soundcheck send it.
2. **The `X-Echo-Run` cookie**, the same value URL-encoded, for handlers that read cookies but not headers.
3. **Queued work** carries it on: the WordPress connector stores it with each WP-Cron event and Action Scheduler action scheduled during the request, Laravel keeps it in the job's Context, and the Node connector in `withEchoRun()` scopes.
4. **`RHYTHM_ECHO_RUN`** in the environment, for commands and workers.

A message with none of these still lands in whatever action is open when it arrives, which covers plain SMTP mail and a cron job Echo flushed. Messages that arrive outside any action (`rhythm echo watch` while you click around) belong to no run.

A tool of your own can do the same: send `X-Echo-Run: <run>/<action>` on your requests, or open and close the action through the API (`POST /v1/actions/begin`, `POST /v1/actions/end`); see [API](Echo-API).

## Snapshots

`snapshot` runs every action and saves the run as `.echo/snapshots/<name>/run.json`, with a `report.html`. Without `--name`, the name is the date and time, such as `2026-10-24T08-19-22`. Names use letters, digits, dots, dashes and underscores, up to 64 characters, and start with a letter or digit. A snapshot with the same name is replaced once the new one is complete; the newest snapshot becomes the *latest*, the one `compare` uses.

```sh
rhythm echo snapshots
```

```text
NAME                     ACTIONS  TAKEN        SENT
before-update            3        1m ago       3 emails, 1 SMS, 1 chat message, 1 webhook  (latest)
```

`rhythm echo compare --snapshot <name>` compares with another one, and `rhythm echo snapshots delete <name>` (or `rm`) removes one.

> [!NOTE]
> Deleting the latest snapshot doesn't make another one latest: `compare` then stops with `no snapshot named "…"`. Pass `--snapshot <name>`, or take a new snapshot.
<!-- TODO(lead): drop this note if deleting the latest snapshot starts moving LATEST. -->

A snapshot reports the problems the site has now (an unfilled placeholder already in a template), so you know the starting point; `compare` then reports only what changed.

## Reading a comparison

`compare` runs the snapshot's actions again from the current config, matches each action by its ID, and diffs the messages each one sent. Actions the snapshot doesn't have aren't compared (take a new snapshot to include them); actions missing this time are noted.

### The classes

| Class | Means | Changes |
|---|---|---|
| **BROKE** | Worked in the snapshot, fails or is missing now | a message is no longer sent; a message now fails; the action itself fails; a new error in a message (an unfilled placeholder, a PHP warning, a failed `expect`) |
| **NEW** | Wasn't there before | a message is now sent; a new warning or notice |
| **CHANGED** | The same message, but different: check it's intended | recipients' copies, subject, sender, Reply-To, text, event, payload fields, fewer sends |
| **FIXED** | A problem from the snapshot is gone | an error, warning or notice that's no longer reported |

| Check | Class | Title | Before → after |
|---|---|---|---|
| `message/missing` | BROKE | Email to office@shop.test is no longer sent | how many were sent → `0` |
| `message/failing` | BROKE | Twilio SMS to the test number now fails | `faked` → `failed` |
| `action/failed` | BROKE | Action fails | the error |
| `message/new` | NEW | Email to office@shop.test is now sent | `–` → how many; the detail is its subject |
| `message/fewer` | CHANGED | Email to the test submitter is sent fewer times | `2` → `1` |
| `message/subject` | CHANGED | Email to the test submitter: subject changed | the subject templates |
| `message/from` | CHANGED | …: sender changed | the senders; fix: check it with `rhythm echo check <domain>` |
| `message/copies` | CHANGED | …: copies (Cc/Bcc) changed | the copies |
| `message/reply-to` | CHANGED | …: lost its Reply-To | the old Reply-To |
| `message/text` | CHANGED | Twilio SMS to the test number: text changed | excerpts of both |
| `message/event` | CHANGED | …: event changed | the events |
| `message/fields-lost` | CHANGED | Webhook to hooks.zapier.com: payload lost fields | the fields, e.g. `order.email` |
| `message/fields-added` | CHANGED | …: payload has new fields | the fields |

Issues found in messages (see [Messages and checks](Echo-Messages-And-Checks#checks)) keep their own check names: a new `content/placeholder` error is BROKE, a new `email/no-subject` warning is NEW.

### How messages are matched

Two messages are *the same message* when they have the same **identity**:

- email, SMS and WhatsApp: the channel and the recipients;
- everything else: the channel, the provider, the method, the host, the path with IDs replaced by `{id}` (`/v1/orders/123` and `/v1/orders/456` match), and the event.

Messages of the same identity are counted, and the first of each pair is compared for content. Before anything is compared, what varies on every run is normalized away:

<!-- TODO(lead): an uncommitted change in compare/compare.go pairs messages of one identity by subject and reports each lost one (before/after "sent" → "not sent"); update this section, the check table and the s07 note on Echo-Benchmark if it lands. -->

| What | Becomes |
|---|---|
| `echo-test…@…` and `soundcheck-test…@…` addresses | "the test submitter" |
| `+1555` followed by seven digits, and `+15005550…` numbers | "the test number" |
| The subject | lowercased, with digits replaced by `#` and test names by `{name}`, so `Order #1041` and `Order #1042` match |
| The text | fingerprinted without numbers, dates, times, long hex and token-like strings, test data and whitespace |
| A JSON payload | its field paths (`order.total`, `line_items[].sku`); a form body, its keys |

So a new order number in the subject, a fresh test address or a different timestamp isn't a change; a different admin address, a new subject line or a lost payload field is. The subject's before and after are shown in that normalized form, lowercased.

### Grouping and order

The same change in several actions is one line, listing every action it affects. Changes are sorted by class, then by how many actions they affect, then by title. At the end, the panel gives the verdict (*Something broke*, *New messages or problems*, *Everything is still sent the same way*) and the count per class.

Notes below the panel say what limits the result: `the snapshot ran in seal mode and this run in observe mode`, `1 actions from the snapshot weren't run this time`.

### Failing a build

`--fail-on broke|new|changed` makes `compare` exit `1` when there's a change at or above that class: `new` fails on BROKE or NEW, `changed` on anything but FIXED. For `run` and `snapshot`, `--fail-on error|warning|notice` works on the problems found, and an action that fails (its request errors, its command exits non-zero) counts at every level. There's no `none`: leave the flag out. A value of the wrong kind is a usage error:

```text
rhythm echo: --fail-on "error": want broke, new or changed
```

See [CI](Echo-CI).

## Reports

Every snapshot, run and comparison is stored in `.echo/` with a standalone `report.html`. `report` renders a stored result again without running anything:

```sh
rhythm echo report                  # the latest run or comparison
rhythm echo report before-update    # a snapshot
rhythm echo report .echo/runs/2026-10-24T08-17-42
rhythm echo report --md -o echo.md
rhythm echo report --junit -o echo-junit.xml --fail-on new
rhythm echo report --open           # open the HTML report
```

| Format | Flag | |
|---|---|---|
| Terminal | (default) | The live view, styled; `--plain` (or any output that isn't a terminal) gives plain text with every detail. |
| HTML | `--html`, `-o report.html` | One file: actions with their messages (emails in a sandboxed frame, SMS and chat as bubbles, payloads as sent, and each answer), changes on top. |
| Markdown | `--md` | The verdict, a count table, then a table per class with the change, actions, before → after and fix. Fits a pull request comment or a job summary. |
| JSON | `--json` | The comparison, or the run (schema `rhythm-echo/v1`). |
| JUnit | `--junit` | A test suite per class (`echo.broke`, `echo.new`…) and a test case per change; cases at or above `--fail-on` (BROKE and NEW without it) are failures. |

`-o <file>` picks the format from the extension (`.html`, `.md`, `.json`, `.xml`) and still prints the terminal view; a format flag without `-o` prints only that format on stdout. The 20 newest runs and comparisons are kept; snapshots are never pruned.

---
<sub>← [Messages and checks](Echo-Messages-And-Checks) · [Home](Home) · [With Soundcheck](Echo-With-Soundcheck) →</sub>
