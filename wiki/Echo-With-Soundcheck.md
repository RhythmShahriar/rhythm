Soundcheck submits your site's real forms in a real browser and runs your scripted flows. With Echo running, `--echo` ties each submit and flow to the emails, SMS and webhooks it made the site send, so a form that says "sent" but emails nobody is caught, and a lost or changed message shows up in the comparison. This page covers turning it on, what happens on each submit, checking messages in flows with `expect: {message: …}`, and how the messages are compared.

<!-- TODO(lead): written against the soundcheck feat/echo branch (to ship as Soundcheck 1.1.0). Re-check flag names, config keys and output against the released binary. -->

```sh
rhythm echo up                                                          # Echo, sealed
rhythm soundcheck snapshot staging.shop.test --level inspect,click,submit --echo
# …update plugins, deploy…
rhythm soundcheck compare --level inspect,click,submit --echo
```

You need Echo running with the site's [connector](Echo-Install-Connectors) installed, and Soundcheck 1.1 or later. Forms are only submitted at the `submit` level and flows at the `flows` level, so add those to `--level` (or `interactions.levels` in Soundcheck's config); without them, `--echo` is on but has nothing to record, and says so.

> [!IMPORTANT]
> The `submit` level really sends forms. With Echo sealed, the messages they trigger stop at Echo; without it, they go out. Run this on staging, and list production under `production` in Soundcheck's config, which it never submits forms on. See [Interactions](Soundcheck-Interactions#the-production-guard).

## Turning it on

| Where | Value | |
|---|---|---|
| `--echo` | | Echo at `http://127.0.0.1:8025`. |
| `--echo=<url>` | `--echo=http://127.0.0.1:18725` | Echo somewhere else. The value needs `=`: a separate argument would be read as the site to check, so `--echo http://127.0.0.1:8025` is refused with `--echo takes its address with "=": --echo=http://127.0.0.1:8025`. |
| `--echo=false` | | Off, even when the environment or config turns it on. |
| `SOUNDCHECK_ECHO` | a URL, or `1`, `true`, `on`, `yes` for the default address | Same as `--echo`. `0`, `false`, `off`, `no` or empty leave it off. |
| `echo.url` in `.soundcheck/config.yaml` | `http://127.0.0.1:8025` | On for every run of the project. |

`--echo` wins over `SOUNDCHECK_ECHO`, which wins over `echo.url`. Whichever turns it on adds the `echo` module to the run; `--skip echo` turns it off again. Naming the module without an address fails: `modules: echo needs Echo's address: --echo, SOUNDCHECK_ECHO or echo.url`.

Soundcheck checks Echo answers before it starts. If it doesn't, the run stops at once (exit `2`) rather than silently recording nothing:

```text
rhythm soundcheck: --echo: Echo isn't running at http://127.0.0.1:8025 (start it with: rhythm echo up)
```

### The echo section

```yaml
# .soundcheck/config.yaml
echo:
  url: ""                       # e.g. http://127.0.0.1:8025; empty: off
  token_env: RHYTHM_ECHO_TOKEN  # environment variable with Echo's token, if it has one
  quiet: 1500ms                 # after a submit, wait until nothing new arrives for this long
  timeout: 10s                  # but no longer than this
```

| Key | Default | Rules |
|---|---|---|
| `url` | `""` | An `http://` or `https://` URL. |
| `token_env` | `RHYTHM_ECHO_TOKEN` | The name of the variable to read Echo's token from; the token itself is never in the file. |
| `quiet` | `1500ms` | Between `100ms` and `30s`. |
| `timeout` | `10s` | At least `quiet`, at most `1m`. |

`rhythm soundcheck init` writes this section with the rest of the config.

## What happens on each submit

At the `submit` level Soundcheck fills each form with test data and submits it (see [Forms and flows](Soundcheck-Forms-And-Flows)). With Echo on:

1. **Tag.** Before the submit, Soundcheck sets `X-Echo-Run: <run>/<action>` on the tab's requests to the site's own origin, as a header and as a cookie. Requests to other hosts (reCAPTCHA, payment scripts, CRMs) are left alone, so they don't fail cross-origin checks. The action ID is the page and the form's stable selector, such as `form:/contact/#form.wpcf7-form`, the same in every run.
2. **Submit.** The form is submitted as usual. Tagged submits take turns across tabs, because the cookie is shared by the browser session.
3. **Collect.** When the submit succeeds, Soundcheck asks Echo for that action's messages, waiting until nothing new arrives for `quiet` (at most `timeout`).
4. **Record.** The messages are saved with the page, the interaction's detail gets what was sent (`… · sent 2 emails, 1 chat message`), and Echo's checks run on them: an unfilled placeholder or a PHP warning in the email becomes a page issue named `echo/content/placeholder`, `echo/content/php-error` and so on (see [Messages and checks](Echo-Messages-And-Checks#checks)).

When a submit shows success but Echo saw nothing go out, the page gets a notice:

```text
[2/2] /contact/ 200 1.03s (1 issue)

[NOTICE] Form says sent, but no message went out - /contact/
  source: form #contact
  form #contact showed success, and Echo saw no email, SMS or webhook from it within 1.5s

all clear - 0 errors, 0 warnings, 1 notice - 2 pages · Echo: no messages from 1 form or flow · 6s
```

Its fix: *If the form should notify someone, check its notification settings and the site's mailer; check the site has Echo's connector (rhythm echo doctor). Forms meant to send nothing can ignore this.* The interaction's detail in the JSON report says the same: `redirected to /contact/sent · no message went out`.

It's only a notice, because many forms rightly send nothing (a search, a filter, a poll). Losing a message the snapshot had is what the comparison reports as BROKE.

The run's summary line counts what was sent:

```text
Snapshot "before-update" saved; all clear - 0 errors, 0 warnings, 0 notices - 2 pages · Echo: 2 emails, 1 chat message from 1 form or flow · 7s
```

The HTML report gets a **Messages sent** table, one row per message (form or flow, page, channel, recipient, subject), and a row for each submit that sent nothing. Message bodies stay in Echo; its live view shows them in full.

## Comparing

`compare` with `--echo` submits the same forms again and compares each form's messages with the snapshot's, using Echo's own comparison (see [Reading a comparison](Echo-Actions-And-Compare#reading-a-comparison)):

```text
rhythm soundcheck 1.1.0 - https://staging.shop.test/

Snapshot: "before-update", 2 pages, taken just now
Echo: http://127.0.0.1:8025, seal mode: recording what each form submit and flow sends
Pages: 2 pages to start
[1/2] / 200 1.03s (ok)
[2/2] /contact/ 200 1.03s (ok)

[BROKE] Email to office@shop.test is no longer sent - /contact/
  1 → 0
  source: form #contact
  fix: Something in the change stopped this message: check the plugin or code that sends it, and its settings.
[CHANGED] Email to the test submitter: subject changed - /contact/
  thanks for getting in touch → we got your message
  source: form #contact

Something broke - 1 broke, 0 new, 1 changed, 0 slower, 0 fixed - compared with snapshot "before-update" (just now), 2 pages · Echo: 1 email, 1 chat message from 1 form or flow · 5s
```

- Changes are named `echo/message/missing`, `echo/message/subject` and so on, and grouped across pages: a newsletter form in every footer that stops sending is one line, with every page.
- With several forms on one page, each title starts with the form's name, so their changes don't merge.
- A form that didn't submit this time isn't compared; the interact module reports why it didn't.
- Where a form's messages are reported as BROKE, the *Form says sent, but no message went out* notice for the same form is dropped: it's the same news.
- Soundcheck's fake data (`soundcheck-test+…@example.com`) is "the test submitter" to Echo, so a new address each run isn't a change.

Take the snapshot and the comparison both with `--echo`. Comparing without it notes `modules differ (snapshot [interact echo], now [interact]); only modules in both are compared`, and the messages aren't checked.

## Flows: expect a message

In a [flow](Soundcheck-Forms-And-Flows#flows), `expect: {message: …}` checks what the flow has made the site send so far:

```yaml
# .soundcheck/flows/contact.yaml
name: Contact form emails the office
start: /contact/
steps:
  - fill: {selector: "input[name=name]", value: "${fake:name}"}
  - fill: {selector: "input[name=email]", value: "${fake:email}"}
  - fill: {selector: "textarea[name=message]", value: Hello}
  - submit: "#contact"
  - expect: {text: Thank you}
  - expect: {message: {channel: email, to: office@shop.test, subject: New message}}
  - expect: {message: {channel: email, to: "${fake:email}", subject: Thanks}}
  - expect: {message: {channel: sms}, absent: true}
```

| Field | Matches |
|---|---|
| `channel` | `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push` or `http` |
| `to` | a substring of any recipient (to, cc, bcc) or of a webhook's URL |
| `subject` | a substring of the subject |
| `text` | a substring of the text, the HTML or the raw body |
| `count` | at least this many such messages; default 1 |

Text fields ignore case and can use `${fake:…}` and `${env:…}`. `absent: true` wants no such message, and doesn't mix with `count`. A flow keeps its tag on for all its steps (it has its own browser session), so everything it causes counts. Like other expects, it retries until the step timeout, so a missing message fails after that wait:

```text
[ERROR] Flow "Contact form emails the office" step 6 failed: expect message email to "office@shop.test" subject "New message" - /contact/
  source: .soundcheck/flows/contact.yaml
  no such message was sent (Echo saw: email to soundcheck-test+640c49@example.com "We got your message"; chat to slack webhook)
```

Without `--echo`, a message expect fails with `expect message needs Echo: start it (rhythm echo up) and run with --echo`.

Flows' messages are compared like forms', action by action (`flow:<name>`), and problems Echo finds in them are reported on the flow's start page.

## In CI

Start Echo with its GitHub Action in `up` mode, then run Soundcheck with `--echo` in the same job; see [CI](Echo-CI#with-soundcheck).

---
<sub>← [Actions and compare](Echo-Actions-And-Compare) · [Home](Home) · [Replay](Echo-Replay) →</sub>
