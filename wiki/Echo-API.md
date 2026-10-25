`rhythm echo up` serves a small HTTP API on its `listen` address (`127.0.0.1:8025` by default), next to [Echo Inbox](Echo-Inbox). Connectors post captures to it. Echo's own commands, Soundcheck and your tests read from it. This page covers who may call it, every route, search and paging, the event stream, action leases, the capture protocol for writing a connector for another stack, the Mailpit-style routes and the `client` Go package.

## Who may call it

When Echo has a token (`--token` or `RHYTHM_ECHO_TOKEN`, required when it listens beyond loopback without `--private-network`), every request must send it:

```sh
curl -H "X-Echo-Token: $RHYTHM_ECHO_TOKEN" http://172.17.0.1:8025/v1/health
```

Without it, or with the wrong one, Echo answers `401` with `missing or wrong X-Echo-Token (in a browser, open /?token=<your token> once)`. A browser opens `/?token=…` once and gets a cookie instead; see [Echo Inbox](Echo-Inbox#with-a-token).

Without a token, a request is checked so a web page can't reach the API through your browser:

| Check | Refused with |
|---|---|
| The `Host` is a loopback name, the listen host, a name `--hostname` gives or, with `--private-network`, an IP address or a one-label service name | `403` `Echo answers only to localhost: Host rebind.attacker.example is not allowed` |
| No `Origin` from another page | `403` `Echo refuses requests from web pages: Origin https://evil.example is not allowed` |
| A body is `application/json` (a form-typed body from a non-browser client, such as `curl -d`, is fine) | `415` `send JSON: Content-Type must be application/json` |

On loopback without a token, anything on the machine can read the captured messages, as with any local development server.

On a [hub](Echo-Hub-And-Reports) or with `up --sites`, each site has a **write token** that may only send captures, begin, renew and end actions and read `/v1/health`; anything else answers `403` (`this is a write token: it may only send captures and begin, renew or end actions; reading messages needs the read token`). The read token reads everything.

## Routes

| Route | Does |
|---|---|
| `GET /` | [Echo Inbox](Echo-Inbox). `/assets/…` and `/ui/…` serve it; `/report` is the HTML report of the session so far; `/login` is the viewer sign-in with `--login`. |
| `GET /v1/health` | Echo's version, mode, wire version, the real SMTP and proxy addresses, a boot ID, the store, its capabilities and the recognized providers. |
| `POST /v1/capture` | A connector reports an email or an HTTP request; Echo answers what to do. At most 8 MiB (`413` `capture too large: Echo accepts up to 8 MiB`). See [the capture protocol](#the-capture-protocol). |
| `GET /v1/messages` | Captured messages, oldest first, filtered; a page, newest first, with `limit` or `before`. |
| `GET /v1/messages/{id}` | One message; `404` when there's none. |
| `DELETE /v1/messages/{id}` | Delete one message: `204`, or `404`. |
| `DELETE /v1/messages` | Delete the messages matching the filters (all, without filters). Answers `{"deleted": n}`. |
| `GET /v1/messages/latest` | The newest matching message with the one-time codes and links Echo read from it, unmasked. See [Codes and links](#codes-and-links). |
| `GET /v1/wait` | Wait for messages, then return them. |
| `GET /v1/events` | A stream of changes, as server-sent events. |
| `POST /v1/actions/begin` | Open an action and get its lease. See [Actions and leases](#actions-and-leases). |
| `POST /v1/actions/renew` | Keep an action open for another lease period. |
| `POST /v1/actions/end` | Close it. Answers `204`. |
| `POST /v1/messages/{id}/release` | Send one caught email through the relay, `{"to": "ann@client.example"}`. See [Releasing an email](Echo-Inbox#releasing-an-email-for-sign-off). |
| `GET /v1/sites` | On a hub: each site with its message count, its last message and its schedule. |

```sh
curl -s http://127.0.0.1:8025/v1/health
```

```json
{"tool":"rhythm-echo","version":"1.1.1","mode":"seal","wire":2,"smtp":"127.0.0.1:1025","proxy":"off","boot_id":"Q7HOUFZMQM4GKPNWWDRUP2YDX3","store":"memory","capabilities":["leases","issues","late","search","events","attachments","codes","exclusive","site-tokens","sites","signed-runs","lease-ttl"],"providers":["activecampaign","adyen","amazon-ses","amazon-sns","apns","authorize-net","bandwidth","bird","braintree","brevo","clicksend","discord","expo","fcm","ga4","gmail","hubspot","ifttt","infobip","klaviyo","mailchimp","mailgun","mailjet","make","mandrill","messagebird","meta-capi","microsoft-graph","mixpanel","mollie","n8n","onesignal","paypal","plivo","posthog","postmark","razorpay","resend","segment","sendgrid","sinch","slack","sparkpost","square","stripe","teams","telegram","telnyx","twilio","vonage","whatsapp-cloud","zapier","woocommerce"]}
```

`capabilities` lists what this Echo does beyond wire 1, so a client can tell an older Echo apart. `boot_id` changes when Echo restarts, which is how a run notices a restart. `store` is `memory` or `jsonl` (a kept inbox).

### Filters

`/v1/messages`, `DELETE /v1/messages`, `/v1/wait` and `/v1/messages/latest` take the same query parameters; every one given must match:

| Parameter | Matches |
|---|---|
| `run` | the run ID, exactly |
| `action` | the action ID, exactly, including late mail credited to it |
| `channel` | `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push` or `http` |
| `to` | a substring of any recipient, ignoring case |
| `since` | messages captured at or after an RFC 3339 time, or this long ago (`10m`, `2h`) |
| `site` | a hub site's ID |
| `q` | a search; see below |

```sh
curl -s "http://127.0.0.1:8025/v1/messages?channel=sms&to=1555"
```

The answer is a JSON array of messages, `[]` when nothing matches. The fields are in [Messages and checks](Echo-Messages-And-Checks#what-a-message-records); a message the checks found problems in carries them in `issues`.

### Search and pages

`q` takes the search Echo Inbox's box does: words that must all match, fields such as `to:`, `from:`, `subject:"order shipped"`, `channel:email,sms`, `action:`, `provider:`, `check:content/dev-link`, `site:client-a`, `has:attachment` and `has:issue`, plus free text over the subject, text, HTML, addresses, URL, event and body. Case never matters.

`limit=N` (default 50, at most 1000) or `before=<id>` switches the answer to a page, newest first: `{"messages": […], "next": "<id>"}`. Pass `next` as `before` for the next page; it's absent on the last one. Pages stay consistent while captures arrive.

```sh
curl -s "http://127.0.0.1:8025/v1/messages?q=channel:email%20subject:%22sign-in%20code%22&limit=1"
```

```text
{"messages":[{"id":"d159729c67c5","channel":"email","from":"you@staging","to":["ann@shop.test"],"subject":"Your sign-in code","text":"Use ••••• to sig…
```

Mistakes answer `400` with the reason, such as `q: channel:fax: not a channel (email, sms, whatsapp, chat, webhook, analytics, payment, push, http)`, `q: a quote in the search isn't closed` or `before: no message with that ID; it was deleted or dropped, so start again without a cursor`.

### Waiting

`/v1/wait` returns once at least `count` messages match **and** none has arrived for `quiet`, or when `timeout` runs out, with whatever matched by then. It's how a test waits for the email a request triggers without guessing a sleep:

| Parameter | Default | Limit |
|---|---|---|
| `count` | `0` | |
| `quiet` | `1.5s` | `30s` |
| `timeout` | `10s` | `1m` |

```sh
curl -s "http://127.0.0.1:8025/v1/wait?run=checkout-test&channel=email&count=1&quiet=1s&timeout=20s"
```

### Codes and links

Echo reads every capture for one-time codes and for sign-in, magic, verify, reset and unsubscribe links before it masks them. It keeps them in memory beside the message. `GET /v1/messages/latest` returns the newest matching message with them, unmasked:

```sh
curl -s "http://127.0.0.1:8025/v1/messages/latest?to=ann@shop.test"
```

```text
{"codes":["482913"],"links":[],"message":{"id":"d3e2ccbf826f","channel":"email","from":"hello@shop.example.com","to":["ann@shop.test"],"subject":"Your sign-in code","text":"Use ••••• to sign in. It expires in 10 minutes.\r\n",…
```

A link comes as `{"kind": "reset", "url": "http://staging.shop.test/wp-login.php?login=admin&key=CTfZnZrLAmWG5fkCi2zt&action=rp&wp_lang=en_US"}`. `wait=10s` (at most `1m`) waits for a match; with none it answers `404` `no message matched within 1s`. `has=code` or `has=link` skips messages without one, so the sign-in email is found even when a welcome email came after it (`has: want code or link, not pdf` otherwise). Codes and links are never stored on disk, so after a restart the message is still there but its codes aren't. `rhythm echo latest` and the Playwright and Cypress helpers use this route.

### Events

`GET /v1/events` is a server-sent event stream. It opens with `retry: 2000`, then one event per change, `id: <n>` and `data: {"type":"add","id":"<message id>"}`, with `type` `add`, `update`, `delete` (also for messages dropped to make room) or `reset` (the client fell more than 4096 changes behind, or reconnected with a `Last-Event-ID` from before a restart: read the messages again). A client reconnecting with `Last-Event-ID` gets what it missed. While idle, a `: ping` comment every 20 seconds keeps it open.

### Actions and leases

An open action collects the messages that arrive without a run ID, such as plain SMTP mail.

```sh
curl -s -X POST -H "Content-Type: application/json" -d '{"run":"r1","action":"a1"}' http://127.0.0.1:8025/v1/actions/begin
```

```json
{"lease":"SHJWSAUZMEIUCY7RY46PB74YZ6"}
```

`end` takes `{"lease": "…"}` and closes exactly that action (`204`). Several actions may be open at once: a message that names no run goes to the only open action. It gets `"attribution": "ambiguous"` when more than one is open. An action not ended or renewed within its lease (10 minutes by default, `"ttl"` up to an hour) closes by itself, so a killed runner never leaves one open. `renew` with the lease keeps it open; a lease that already ended answers `404` `renew: no open action has that lease; it ended or its ttl ran out, so begin it again`. `"exclusive": true` waits (up to `"timeout"`, default `30s`, at most `1m`) until no other action is open and holds new ones back until it ends; past the timeout it answers `409`. An `end` without a lease (a client from Echo 1.0) closes the oldest action begun from the same machine.

## The capture protocol

A connector is code inside the site that reports each email and outbound HTTP request to Echo before it's sent and does what Echo answers. The protocol is one JSON request, versioned: wire `2` (Echo still accepts `1`). The WordPress, Laravel and Node connectors, the SMTP sink, the `sendmail` stand-in and the proxy all speak it, so a connector for another stack gets the same recognition, fakes, checks and comparisons. The schema and the shared test cases are in the [conformance kit](Echo-Conformance), with a strict stand-in Echo to test against.

### An HTTP request

Before sending, post the request with `"phase": "before"`:

```sh
curl -s http://127.0.0.1:8025/v1/capture -H 'Content-Type: application/json' -d '{
  "v": 2, "kind": "http", "phase": "before",
  "run": "2026-10-24T08-29-07-cd7ff3/action:order-shipped",
  "source": {"via": "my-connector", "stack": "php", "hook": "http_client", "host": "staging.shop.test"},
  "http": {"method": "POST", "url": "https://api.twilio.com/2010-04-01/Accounts/AC0000/Messages.json",
           "headers": {"Content-Type": "application/x-www-form-urlencoded", "Authorization": "Basic •••••"},
           "body": "To=%2B15551230000&From=%2B15005550006&Body=Your+table+for+2+is+booked"}}'
```

```json
{"id":"8aa9958f960b","mode":"seal","action":"fake","response":{"status":201,"headers":{"content-type":"application/json"},"body":"{\"sid\":\"SM8dc9af9b6e5200000000000000000000\",\"status\":\"queued\",\"to\":\"+15551230000\",\"from\":\"+15005550006\",\"body\":\"Your table for 2 is booked\",\"date_created\":\"Sun, 25 Oct 2026 15:02:37 +0000\",\"num_segments\":\"1\",\"error_code\":null,\"error_message\":null}"}}
```

Then do what `action` says:

| `action` | Do |
|---|---|
| `fake` | Don't send. Give the caller `response` (status, headers, body) as if the service had answered. |
| `block` | Don't send. Fail the request the way the HTTP client fails when the network is down. |
| `allow` | Send it. Afterwards, post `"phase": "after"` with the same `id`, the `status` and the `response` body, or an `error`, so Echo records the outcome. |

```json
{"v": 2, "kind": "http", "phase": "after", "id": "8aa9958f960b", "source": {"via": "my-connector"},
 "http": {"method": "POST", "url": "https://api.twilio.com/…", "status": 201, "response": "{\"sid\": …}"}}
```

An `after` capture without a `before` (a connector that can only see requests once they're sent) is recorded as sent, or as failed for a status of 400 or more.

### An email

```json
{"v": 2, "kind": "email", "run": "…/action:contact-form", "source": {"via": "my-connector", "hook": "mailer"},
 "email": {"from": "shop@shop.test", "to": ["office@shop.test"], "cc": [], "bcc": [], "reply_to": "",
           "subject": "New message from Jamie", "text": "…", "html": "…",
           "headers": {"X-Mailer": "…"}, "raw": "<the full MIME message, when you have it>",
           "attachments": [{"name": "invoice-42.pdf", "type": "application/pdf", "size": 84211, "sha256": "…"}]}}
```

Echo answers `"action": "fake"` when sealed (don't send; tell the caller it was sent) and `"allow"` in observe mode (send it normally). After a real send, post `{"kind": "email", "phase": "after", "id": "<the id>", "error": "<the transport error, if it failed>"}` to record the outcome on the same message. Fields missing from `email` are filled from `raw` when it's given. A capture with `"error"` set records a send that failed before it reached Echo.

### Fields

| Field | Type | |
|---|---|---|
| `v` | number | The wire version, `2` (or `1`). Echo refuses a newer one with `this connector is newer than Echo; run: rhythm update (it speaks wire 3, this Echo wire 2 at most)`: ask `/v1/health` and send the version it names. |
| `kind` | string | `email` or `http`. |
| `phase` | string | HTTP: `before` or `after`. Email: `after` with an `id` reports a real send's outcome. |
| `id` | string | For `after`: the `id` Echo returned for the `before`. |
| `run` | string | `<run>/<action>`, copied from the triggering request's `X-Echo-Run` header or cookie, or `RHYTHM_ECHO_RUN`, without its signature. |
| `source.via` | string | Your connector's name. |
| `source.stack`, `source.plugin`, `source.hook` | string | The stack, the plugin or package that sent it (best guess) and the hook you caught it in. |
| `source.host` | string | The site's own host, from its configuration. Requests to it are allowed, unless it's a provider's domain or a public suffix. |
| `source.site` | string | (wire 2) The site's ID on a hub. |
| `email` | object | `from`, `to[]`, `cc[]`, `bcc[]`, `reply_to`, `subject`, `text`, `html`, `headers{}`, `raw` plus (wire 2) `attachments[]` of `{name, type, size, sha256}` when you send no raw MIME. Never file content. |
| `http` | object | `method`, `url`, `headers{}` and `body`, plus for `after`: `status`, `response`. |
| `error` | string | The send failed: a transport error, `wp_mail_failed`. |

A bad capture gets `400` with the reason (`an email capture needs email`, `http capture: bad url`, `kind must be "email" or "http"`). Echo keeps 256 KiB of each field.

### What a connector must do

1. **Stay off in production.** Read the platform's environment (`WP_ENVIRONMENT_TYPE`, `APP_ENV`, `NODE_ENV`, `VERCEL_ENV`) and install no hooks there, unless `RHYTHM_ECHO_FORCE` is set.
2. **Fail closed.** If Echo can't be reached and the connector's mode is `seal` (the default), don't send: fail the mail or the request.
3. **Skip what isn't a message.** Don't report requests to Echo itself or to loopback. Take the site's own host from configuration, never from the request's `Host` header.
4. **Mask credentials before they leave.** Mask every header whose name contains `token`, `secret`, `key`, `auth`, `signature`, `session`, `password`, `cookie` or `credential`; keep only a Stripe key's mode prefix (`sk_test_•••••`) so Echo can tell live from test.
5. **Copy the run ID, carefully.** Take `X-Echo-Run` from the incoming request (header, then cookie) and carry it into queued work, falling back to `RHYTHM_ECHO_RUN`. With a token set, accept only values Echo signed; without one, only values from this machine. See [Run IDs and signatures](Echo-Install-Connectors#run-ids-and-signatures).
6. **Send the token** as `X-Echo-Token`: `RHYTHM_ECHO_WRITE_TOKEN` when set, else `RHYTHM_ECHO_TOKEN`. Read `RHYTHM_ECHO_URL` for Echo's address (default `http://127.0.0.1:8025`) and `RHYTHM_ECHO_SITE_ID` for `source.site`.
7. **Fall back.** On `400` with `newer than Echo`, ask `/v1/health` and resend in the version it names; try the newer one again after an hour.
8. **Keep it short.** Echo runs next to the site: a timeout of a few seconds is plenty.

The Laravel package's `src/` and the Node package's `src/` in the [Echo repository](https://github.com/RhythmShahriar/echo/tree/master/connectors) are compact references, with tests against the conformance stand-in.

## Mailpit-style routes

`rhythm echo up` also answers [Mailpit](https://mailpit.axllent.org)'s API for email, for tools and tests written against Mailpit: `GET /api/v1/messages` (newest first, with `start` and `limit`, default 50), `GET /api/v1/message/{ID}` (`latest` works; marks it read), `GET /api/v1/message/{ID}/headers`, `GET /api/v1/message/{ID}/raw`, `GET /api/v1/search?query=…` (Mailpit's search syntax, plus Echo's `site:`, `action:`, `provider:`, `check:` and `has:issue`), `PUT /api/v1/messages` (read flags), `DELETE /api/v1/messages`, `GET /api/v1/info` and `GET /api/v1/webui`. They cover email only and take the same token as the rest of the API.

```sh
curl -s "http://127.0.0.1:8025/api/v1/search?query=subject:invoice&limit=1"
```

```json
{"total":16,"unread":16,"count":1,"messages_count":3,"messages_unread":3,"start":0,"tags":[],"messages":[{"ID":"4d889db33f0d","MessageID":"","Read":false,"From":{"Name":"WordPress","Address":"wordpress@staging.shop.test"},"To":[{"Name":"","Address":"jamie@shop.test"}],"Cc":[],"Bcc":[],"ReplyTo":[],"Subject":"Your invoice for order 1042","Created":"2026-10-25T14:26:07.282733508Z","Tags":[],"Username":"","Size":86,"Attachments":0,"Snippet":"Hi Jamie, Your invoice is attached. Questions? Call us on 0800 123 456. Bean & Leaf"}]}
```

Attachments are listed with their name, type, size and SHA-256; Echo keeps no file content, so `GET /api/v1/message/{ID}/part/{PartID}` answers `404`. Read flags live in memory. Tags, chaos, link checks, the send API and thumbnails aren't served.

## The client package

Go programs and tests can use `github.com/RhythmShahriar/echo/client`, which Echo's commands and Soundcheck use themselves:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/RhythmShahriar/echo/client"
	"github.com/RhythmShahriar/echo/model"
	"github.com/RhythmShahriar/echo/wire"
)

func main() {
	ctx := context.Background()
	echo := client.New("http://127.0.0.1:8025", "") // the token, when Echo has one

	h, err := echo.Health(ctx)
	if err != nil {
		log.Fatal(err) // no Echo is running at … (start it with: rhythm echo up)
	}
	fmt.Println("Echo", h.Version, "in", h.Mode, "mode; leases:", h.Supports("leases"))

	// Tie what the site sends to this test, then trigger it.
	run, action := "checkout-test", "place-order"
	lease, err := echo.BeginLease(ctx, run, action)
	if err != nil {
		log.Fatal(err)
	}
	defer echo.EndLease(context.WithoutCancel(ctx), lease)
	// …send the request that places the order, with the header
	// X-Echo-Run: client.RunValue(run, action). Here a stand-in capture:
	_, err = echo.Capture(ctx, wire.Capture{Version: wire.Version, Kind: "email", Run: client.RunValue(run, action),
		Source: wire.Source{Via: "test"},
		Email:  &wire.Email{From: "shop@shop.test", To: []string{"jamie@shop.test"}, Subject: "Order #1042 confirmed", Text: "Your code is 482913."}})
	if err != nil {
		log.Fatal(err)
	}

	// At least one email, then nothing new for 1s (at most 10s).
	msgs, err := echo.Wait(ctx, client.Filter{Run: run, Action: action, Channel: model.Email}, 1, time.Second, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range msgs {
		fmt.Printf("%s to %v: %q (%s)\n", m.Channel, m.To, m.Subject, m.Status)
	}

	// The one-time code it sent, read before Echo masked it.
	l, err := echo.Latest(ctx, client.LatestQuery{To: "jamie@shop.test", Run: run, Wait: 5 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("code:", l.Code(), "stored text:", l.Message.Text)

	n, _ := echo.Clear(ctx, client.Filter{Run: run})
	fmt.Println("cleared", n)
}
```

Run against a local Echo:

```text
Echo 1.1.1 in seal mode; leases: true
email to [jamie@shop.test]: "Order #1042 confirmed" (faked)
code: 482913 stored text: Your code is •••••.
cleared 1
```

| Function | Does |
|---|---|
| `client.New(url, token)` | A client; `url` defaults to `client.DefaultURL` (`http://127.0.0.1:8025`). |
| `Health(ctx)` | `/v1/health`; fails with `client.ErrNotRunning` when nothing answers. `h.Supports(name)` checks a capability. |
| `Messages(ctx, filter)`, `Message(ctx, id)` | List or get messages. Any answer other than success is a `*client.APIError`; `client.IsNotFound(err)` tells a missing message apart. |
| `List(ctx, filter)` | One page: `client.Page{Messages, Next}`, with `Filter.Limit`, `Filter.Before` and `Filter.Q`. |
| `Wait(ctx, filter, count, quiet, timeout)` | `/v1/wait`. |
| `Latest(ctx, client.LatestQuery{…})` | `/v1/messages/latest`; the result has `Codes`, `Links`, `Message`, `Code()` and `Link(kind)`. |
| `Events(ctx)` | The event stream, as a channel of `client.Event{Type, ID}`; closed when the stream drops. |
| `Clear(ctx, filter)`, `DeleteOne(ctx, id)` | Delete messages. |
| `BeginLease(ctx, run, action)`, `BeginExclusive(ctx, run, action, timeout)`, `RenewLease(ctx, lease)`, `EndLease(ctx, lease)` | Open, renew and close an action. `client.IsBusy(err)` is an exclusive begin that timed out. `Begin` and `End` still work, without leases. |
| `Capture(ctx, wire.Capture)` | Report a capture, as a connector would. |
| `client.RunValue(run, action)` | The `X-Echo-Run` value, `run + "/" + action`. |

`client.Filter` has `Run`, `Action`, `Channel`, `To`, `Since`, `Q`, `Limit` and `Before`. The message type is `model.Message` from `github.com/RhythmShahriar/echo/model`, the capture types are in `github.com/RhythmShahriar/echo/wire`. The comparison Soundcheck uses is `github.com/RhythmShahriar/echo/compare`.

---
<sub>← [Configuration](Echo-Configuration) · [Home](Home) · [CLI reference](Echo-CLI-Reference) →</sub>
