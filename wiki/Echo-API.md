`rhythm echo up` serves a small HTTP API on its `listen` address (`127.0.0.1:8025` by default). Connectors post captures to it. Echo's own commands, Soundcheck and your tests read from it. This page covers every REST route, the capture protocol for writing a connector for another stack and the `client` Go package.

## Authentication

When Echo has a token (`--token` or `RHYTHM_ECHO_TOKEN`, required when it listens beyond loopback), every request must send it:

```sh
curl -H "X-Echo-Token: $RHYTHM_ECHO_TOKEN" http://172.17.0.1:8025/v1/health
```

Without it, or with the wrong one, Echo answers `401` with `missing or wrong X-Echo-Token`. On loopback without a token, there's no authentication: anything on the machine can read the captured messages, as with any local development server.

## REST routes

| Route | Does |
|---|---|
| `GET /` | The live view: an HTML report of everything captured, grouped by action, refreshed every 5 seconds. |
| `GET /v1/health` | Echo's version, mode, wire protocol version and recognized providers. |
| `POST /v1/capture` | A connector reports an email or an HTTP request; Echo answers what to do. See [the capture protocol](#the-capture-protocol). |
| `GET /v1/messages` | Captured messages, oldest first, filtered. |
| `GET /v1/messages/{id}` | One message; `404` when there's none. |
| `DELETE /v1/messages` | Delete the messages matching the filters (all, without filters). Answers `{"deleted": n}`. |
| `GET /v1/wait` | Wait for messages, then return them. |
| `POST /v1/actions/begin` | Open an action: `{"run": "…", "action": "…"}`. Messages that arrive without a run ID are attributed to it until it's closed. Answers `204`. |
| `POST /v1/actions/end` | Close it. Answers `204`. |

```sh
curl -s http://127.0.0.1:8025/v1/health
```

```json
{"mode":"seal","providers":["ga4","meta-capi","teams","twilio","slack","discord","woocommerce","hubspot","mailchimp","klaviyo","activecampaign","zapier","make","n8n","sendgrid","mailgun","postmark","resend","amazon-ses","brevo","sparkpost","mandrill","stripe","paypal","fcm","onesignal","vonage","messagebird","bird","plivo","amazon-sns","telnyx","clicksend","whatsapp-cloud"],"tool":"rhythm-echo","version":"1.0.0","wire":1}
```

### Filters

`/v1/messages`, `DELETE /v1/messages` and `/v1/wait` take the same query parameters; every one given must match:

| Parameter | Matches |
|---|---|
| `run` | the run ID, exactly |
| `action` | the action ID, exactly |
| `channel` | `email`, `sms`, `whatsapp`, `chat`, `webhook`, `analytics`, `payment`, `push` or `http` |
| `to` | a substring of any recipient, ignoring case |
| `since` | messages captured at or after this RFC 3339 time |

```sh
curl -s "http://127.0.0.1:8025/v1/messages?channel=sms&to=1555"
```

The answer is a JSON array of messages, `[]` when nothing matches. The fields are in [Messages and checks](Echo-Messages-And-Checks#what-a-message-records).

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

## The capture protocol

A connector is code inside the site that reports each email and outbound HTTP request to Echo before it's sent and does what Echo answers. The protocol is one JSON request, versioned as wire `1`. The WordPress, Laravel and Node connectors, the SMTP sink, the `sendmail` stand-in and the proxy all speak it, so a connector for another stack gets the same recognition, fakes, checks and comparisons.

### An HTTP request

Before sending, post the request with `"phase": "before"`:

```sh
curl -s http://127.0.0.1:8025/v1/capture -H 'Content-Type: application/json' -d '{
  "v": 1, "kind": "http", "phase": "before",
  "run": "2026-10-24T08-29-07-cd7ff3/action:order-shipped",
  "source": {"via": "my-connector", "stack": "php", "hook": "http_client", "host": "staging.shop.test"},
  "http": {"method": "POST", "url": "https://api.twilio.com/2010-04-01/Accounts/AC0000/Messages.json",
           "headers": {"Content-Type": "application/x-www-form-urlencoded", "Authorization": "Basic •••••"},
           "body": "To=%2B15551230000&From=%2B15005550006&Body=Your+code+is+4821"}}'
```

```json
{"id":"5ea4bd5bdfb0","mode":"seal","action":"fake","response":{"status":201,"headers":{"content-type":"application/json"},"body":"{\"sid\":\"SM6e162471d7c000000000000000000000\",\"status\":\"queued\",\"to\":\"+15551230000\",\"from\":\"+15005550006\",\"body\":\"Your code is 4821\",\"date_created\":\"Sat, 24 Oct 2026 02:16:33 +0000\",\"num_segments\":\"1\",\"error_code\":null,\"error_message\":null}"}}
```

Then do what `action` says:

| `action` | Do |
|---|---|
| `fake` | Don't send. Give the caller `response` (status, headers, body) as if the service had answered. |
| `block` | Don't send. Fail the request the way the HTTP client fails when the network is down. |
| `allow` | Send it. Afterwards, post `"phase": "after"` with the same `id`, the `status` and the `response` body, or an `error`, so Echo records the outcome. |

```json
{"v": 1, "kind": "http", "phase": "after", "id": "5ea4bd5bdfb0", "source": {"via": "my-connector"},
 "http": {"method": "POST", "url": "https://api.twilio.com/…", "status": 201, "response": "{\"sid\": …}"}}
```

An `after` capture without a `before` (a connector that can only see requests once they're sent) is recorded as sent, or as failed for a status of 400 or more.

### An email

```json
{"v": 1, "kind": "email", "run": "…/action:contact-form", "source": {"via": "my-connector", "hook": "mailer"},
 "email": {"from": "shop@shop.test", "to": ["office@shop.test"], "cc": [], "bcc": [], "reply_to": "",
           "subject": "New message from Jamie", "text": "…", "html": "…",
           "headers": {"X-Mailer": "…"}, "raw": "<the full MIME message, when you have it>"}}
```

Echo answers `"action": "fake"` when sealed (don't send; tell the caller it was sent) and `"allow"` in observe mode (send it normally). After a real send, post `{"kind": "email", "phase": "after", "id": "<the id>", "error": "<the transport error, if it failed>"}` to record the outcome on the same message. Fields missing from `email` are filled from `raw` when it's given. A capture with `"error"` set records a send that failed before it reached Echo.

### Fields

| Field | Type | |
|---|---|---|
| `v` | number | The wire version, `1`. Echo refuses a newer one with `this connector is newer than Echo; run: rhythm update`. |
| `kind` | string | `email` or `http`. |
| `phase` | string | HTTP: `before` or `after`. Email: `after` with an `id` reports a real send's outcome. |
| `id` | string | For `after`: the `id` Echo returned for the `before`. |
| `run` | string | `<run>/<action>`, copied from the triggering request's `X-Echo-Run` header or cookie, or `RHYTHM_ECHO_RUN`. |
| `source.via` | string | Your connector's name. |
| `source.stack`, `source.plugin`, `source.hook` | string | The stack, the plugin or package that sent it (best guess) and the hook you caught it in. |
| `source.host` | string | The site's own host. Requests to it are always allowed. |
| `email` | object | `from`, `to[]`, `cc[]`, `bcc[]`, `reply_to`, `subject`, `text`, `html`, `headers{}`, `raw`. |
| `http` | object | `method`, `url`, `headers{}` and `body`, plus for `after`: `status`, `response`. |
| `error` | string | The send failed: a transport error, `wp_mail_failed`. |

A bad capture gets `400` with the reason (`an email capture needs email`, `http capture: bad url`, `kind must be "email" or "http"`). A capture is limited to 32 MiB; Echo keeps 256 KiB of each body.

### What a connector must do

1. **Stay off in production.** Read the platform's environment (`WP_ENVIRONMENT_TYPE`, `APP_ENV`, `NODE_ENV`, `VERCEL_ENV`) and install no hooks there, unless `RHYTHM_ECHO_FORCE` is set.
2. **Fail closed.** If Echo can't be reached and the connector's mode is `seal` (the default), don't send: fail the mail or the request.
3. **Skip what isn't a message.** Don't report requests to the site's own host or to Echo itself.
4. **Mask credentials before they leave.** Mask `Authorization`, cookies and API-key headers; keep the first characters of payment keys (`sk_live_•••••`) so Echo can tell live from test. Echo redacts secrets in bodies.
5. **Copy the run ID.** Take `X-Echo-Run` from the incoming request (header, then cookie), carry it into queued work and fall back to `RHYTHM_ECHO_RUN`.
6. **Send the token** as `X-Echo-Token` when `RHYTHM_ECHO_TOKEN` is set. Read `RHYTHM_ECHO_URL` for Echo's address (default `http://127.0.0.1:8025`).
7. **Keep it short.** Echo runs next to the site: a timeout of a few seconds is plenty.

The Laravel package's `src/` and the Node package's `src/` in the [Echo repository](https://github.com/RhythmShahriar/echo/tree/master/connectors) are compact references, with tests against a fake Echo.

## Mailpit-style routes

`rhythm echo up` also serves a small read API for email shaped like [Mailpit](https://mailpit.axllent.org)'s, for tests written against Mailpit: `GET /api/v1/messages` (newest first, with `start` and `limit`, default 50), `GET /api/v1/message/{ID}` (`latest` works; adds a `Headers` field Mailpit doesn't have), `GET /api/v1/message/{ID}/headers` and `DELETE /api/v1/messages` (`{"IDs": […]}`, or every email with an empty body). They cover email only and take the same token as the rest of the API.


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
)

func main() {
	ctx := context.Background()
	echo := client.New("http://127.0.0.1:8025", "") // the token, when Echo has one

	h, err := echo.Health(ctx)
	if err != nil {
		log.Fatal(err) // no Echo is running at … (start it with: rhythm echo up)
	}
	fmt.Println("Echo", h.Version, "in", h.Mode, "mode")

	// Tie what the site sends to this test, then trigger it.
	run, action := "checkout-test", "place-order"
	if err := echo.Begin(ctx, run, action); err != nil {
		log.Fatal(err)
	}
	defer echo.End(context.WithoutCancel(ctx))
	// …send the request that places the order, with the header
	// X-Echo-Run: client.RunValue(run, action)…

	// At least one email, then nothing new for 1s (at most 10s).
	msgs, err := echo.Wait(ctx, client.Filter{Run: run, Action: action, Channel: model.Email}, 1, time.Second, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range msgs {
		fmt.Printf("%s to %v: %q (%s)\n", m.Channel, m.To, m.Subject, m.Status)
	}
	n, _ := echo.Clear(ctx, client.Filter{Run: run})
	fmt.Println("cleared", n)
}
```

Run against a local Echo, with a stand-in capture where the site's request would be:

```text
Echo 1.0.0 in seal mode
email to [jamie@shop.test]: "Order #1042 confirmed" (faked)
cleared 1
```

| Function | Does |
|---|---|
| `client.New(url, token)` | A client; `url` defaults to `client.DefaultURL` (`http://127.0.0.1:8025`). |
| `Health(ctx)` | `/v1/health`; fails with `client.ErrNotRunning` when nothing answers (`no Echo is running at http://127.0.0.1:8025 (start it with: rhythm echo up)`). |
| `Messages(ctx, filter)`, `Message(ctx, id)` | List or get messages. Any answer other than success is a `*client.APIError` (`Echo answered 404 Not Found to GET /v1/messages/x`); `client.IsNotFound(err)` tells a missing message apart. |
| `Wait(ctx, filter, count, quiet, timeout)` | `/v1/wait`. |
| `Clear(ctx, filter)` | Delete messages; returns how many. |
| `Begin(ctx, run, action)`, `End(ctx)` | Open and close an action. |
| `Capture(ctx, wire.Capture)` | Report a capture, as a connector would. |
| `client.RunValue(run, action)` | The `X-Echo-Run` value, `run + "/" + action`. |

`client.Filter` has `Run`, `Action`, `Channel`, `To` and `Since`. The message type is `model.Message` from `github.com/RhythmShahriar/echo/model`, the capture types are in `github.com/RhythmShahriar/echo/wire`. The comparison Soundcheck uses is `github.com/RhythmShahriar/echo/compare`.

---
<sub>← [Configuration](Echo-Configuration) · [Home](Home) · [CLI reference](Echo-CLI-Reference) →</sub>
