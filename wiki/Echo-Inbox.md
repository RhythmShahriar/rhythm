**Echo Inbox** is the web page `rhythm echo up` serves at its API address: one list of every email, SMS, webhook, chat message, analytics event and payment call the site sent, with each message's HTML, text, headers, checks and attachments. This page covers opening it, searching it, keeping it across restarts, letting a client sign in to read it and releasing one email to that client for sign-off.

```sh
rhythm echo up               # then open http://127.0.0.1:8025
```

The inbox is part of Echo. There is nothing to install and it loads nothing from the internet: its scripts and styles are built into the binary.

## What it shows

The list holds every channel, newest first, 50 messages a page with an **Older messages** button. Each row has the kind (`Email`, `Twilio SMS`, `Zapier webhook`) and the action that sent it, the time, a title (the subject, the event or the first words), badges (`1 error`, `2 warnings`, `1 file`, the status), who it's from and to and a snippet.

Opening a message shows its tabs. A tab without content is hidden:

| Tab | Shows |
|---|---|
| **HTML** | The email as a reader sees it, in a sandboxed frame: no scripts, links open in a new tab, remote images blocked until you tick **Load remote images** for that message. |
| **Text** | The plain-text part, or an SMS, chat or push text. |
| **Headers** | The headers as stored, with credentials masked. |
| **Raw** | The MIME source, or the HTTP request. |
| **JSON** | The message as `rhythm echo show <id>` prints it. |
| **Checks** | The problems Echo found in it, with a count: an unfilled placeholder, a PHP warning, a link to staging. See [Messages and checks](Echo-Messages-And-Checks#checks). |
| **Attachments** | Each file's name, type, size and SHA-256, with a count. Echo never keeps a file's content. See [Attachments](Echo-Attachments). |

**Delete** (or the Delete key) removes the open message. The inbox follows Echo's event stream: a green dot says it's connected, new and changed messages appear without a reload and new rows flash once.

| Key | Does |
|---|---|
| `j`, `k`, or the arrow keys | Next or previous message |
| `/` | Focus the search box; Escape leaves it |
| Delete | Delete the open message |

The page before Echo Inbox, the full HTML report of the session so far, is still there at `/report`.

## Search

The search box takes words (all of which must match) and fields:

| Search | Finds |
|---|---|
| `to:ann`, `from:shop.test` | A part of a recipient (To, Cc or Bcc) or of the sender |
| `subject:"order shipped"` | A phrase in the subject (quotes group words) |
| `channel:sms`, `channel:email,sms` | One channel, or any of several |
| `action:checkout` | Messages from an action, or from a run |
| `provider:twilio` | Messages to one service; `rhythm echo providers` lists the slugs |
| `check:content/dev-link`, `check:content` | Messages with a check's finding, or any check in a group |
| `site:client-a` | A site's ID or host, whole or by whole dot-separated labels: `client-a` and `client-a.test`, never `client-ab` |
| `status:blocked` | Messages with this status |
| `has:attachment`, `has:issue`, `has:html` | Messages with files, with findings or with HTML |
| `-channel:analytics` | A minus sign excludes |
| free words | The subject, the text, the HTML, the addresses, the URL, the event and the body |

The filters next to the box offer every channel, action, provider and check the inbox holds. The search, the filters and the open message are kept in the address, so a link to the inbox opens the same view. The same search works on the API, as `GET /v1/messages?q=…`; see [API](Echo-API#search-and-pages).

## Keeping the inbox across restarts

Echo keeps messages in memory by default (the newest 10,000) and forgets them when it stops. `--store` keeps them on disk instead:

```sh
rhythm echo up --store ~/echo-inbox --keep-for 14d
```

```text
rhythm echo 1.1.1 - this machine

Mode: seal: nothing leaves; Echo answers for every service
API: http://127.0.0.1:18025  (connectors post here; open it for the live view)
Inbox: /home/you/echo-inbox  (kept across restarts; 0 messages in it now, each kept 14d)

  Waiting for messages. Ctrl+C to stop.

Stopped. Captured no messages this session.
```

(This run used `--listen 127.0.0.1:18025 --smtp off`.) The same in `.echo/config.yaml`, relative to the project folder:

```yaml
inbox: {store: .echo/inbox, keep: 50000, keep_for: 14d}
```

| Flag | Config | Default | Does |
|---|---|---|---|
| `--store <dir>` | `inbox: store:` | memory | The folder to keep the inbox in. |
| `--keep <n>` | `inbox: keep:` | `10000` | The most messages kept, in memory or on disk. Never more than 256 MiB of them; the oldest go first. |
| `--keep-for <d>` | `inbox: keep_for:` | forever | With a store: drop messages older than this, such as `14d` or `12h`. |

The flags win over the config. `--keep-for` without a folder is refused: `rhythm echo: --keep-for and keep_for: need a folder to keep messages in: --store <dir> or inbox: store: in .echo/config.yaml` (exit `2`). The folder is made owner-only (`0700`), with files `0600`. A folder that holds files that aren't Echo's is refused, so Echo never deletes anything of yours there. One-time codes and links that Echo reads from messages (for [`latest`](Echo-CLI-Reference#latest)) are never written to disk.

Two Echos can't share a folder. The second one exits `3`:

```text
rhythm echo hub: open inbox: another Echo (PID 1482) keeps its inbox in /home/you/hub/store/inbox; stop it first, or give this one a folder of its own
```

`Stopped. Captured … this session.` counts only what this session caught, not what the folder held before.

## Who can open it

Without a token, Echo Inbox answers only to `localhost`, `127.0.0.1` and other loopback names (and, with `--private-network`, to IP addresses and service names in a container network; see [Docker and DDEV](Echo-Docker-And-DDEV)). A page on another site can't read it or frame it: the inbox sends `Content-Security-Policy: default-src 'self'; img-src 'self' data:; style-src 'self'; frame-src 'self'; script-src 'self'; frame-ancestors 'none'` and `X-Frame-Options: DENY`. Requests carrying another site's `Origin` are refused.

### With a token

With `--token` (or `RHYTHM_ECHO_TOKEN`), every route needs the token, the inbox too. A browser opens `/?token=<your token>` once: Echo sets an `echo_token` cookie (HttpOnly, SameSite=Strict) and sends the browser on to `/` without the token in the address. The banner says so:

```text
API: http://127.0.0.1:18025  (connectors post here; open /?token=<your token> once for the live view)
```

```sh
curl -s -o /dev/null -D - "http://127.0.0.1:18025/?token=$RHYTHM_ECHO_TOKEN"
```

```text
HTTP/1.1 303 See Other
Location: /
Set-Cookie: echo_token=…; Path=/; HttpOnly; SameSite=Strict
```

Without it, a request gets `401` with `missing or wrong X-Echo-Token (in a browser, open /?token=<your token> once)`.

### A viewer login for clients

`--login user:password` (better: `RHYTHM_ECHO_LOGIN=user:password`, since other users can read a flag in `ps`) puts Echo Inbox behind a sign-in page at `/login`, for someone who should read what the site sent but not change it, such as a client signing off a launch. The password needs at least 8 characters.

```sh
RHYTHM_ECHO_LOGIN=client:correct-horse rhythm echo up
```

```text
Login: viewers sign in as client at http://127.0.0.1:18025/login (read only, for 8 hours); the API stays open to programs on this machine
```

A session lasts 8 hours. A signed-in viewer can read the inbox but not change it: deleting answers `403` with `signed in to view only: changing the inbox needs Echo's token`. The session never opens the API (`/v1/…`, `/api/v1/…`), which stays with the token. After 20 wrong passwords in a minute (from all clients together), Echo Inbox refuses every sign-in for up to a minute, so a guesser can lock real viewers out for that minute.

A viewer login beyond loopback needs a token too, so connectors use the token and viewers the login:

```text
rhythm echo: RHYTHM_ECHO_LOGIN needs a token when --listen 0.0.0.0:18025 is reachable from other machines: without one the API answers anyone there; set --token (or RHYTHM_ECHO_TOKEN) for connectors and keep the login for viewers
```

With a login set, Echo Inbox no longer insists on a loopback `Host`, so it works behind a reverse proxy that adds TLS. Put the proxy in front for anyone outside your network: the token and the session cross the network in clear otherwise.

## Releasing an email for sign-off

Sealed, no email leaves. Sometimes one should: the client wants to see the real order confirmation in their own inbox before launch. A **relay** lets Echo send one caught email through your own SMTP account, only to the addresses you allow. Each release is logged in the inbox.

```yaml
# .echo/config.yaml
relay:
  allow: ["*@client.example"]
```

```sh
export RHYTHM_ECHO_RELAY=smtp://user:pass@smtp.agency.example:587   # keep the password out of the config
export RHYTHM_ECHO_TOKEN=$(openssl rand -hex 16)
RHYTHM_ECHO_LOGIN=client:correct-horse rhythm echo up
```

```text
rhythm echo 1.1.1 - this machine

Mode: seal: nothing leaves; Echo answers for every service
API: http://127.0.0.1:18025  (connectors post here; open /?token=<your token> once for the live view)
SMTP: 127.0.0.1:11025  (point a mailer or sendmail_path here)
Login: viewers sign in as client at http://127.0.0.1:18025/login (read only, for 8 hours)
Relay: releases go out through smtp.agency.example:2587, only to *@client.example (Release in Echo Inbox, or rhythm echo release <id> --to <address>)

  Waiting for messages. Ctrl+C to stop.
```

(This run used a local test SMTP server on port 2587.) An email's detail then has a **Release to** field and a **Release** button, which a signed-in viewer may use too. From the command line:

```sh
rhythm echo release 2a5397874054 --to ann@client.example
```

```text
Released 2a5397874054 to ann@client.example (logged as 20a3da3c9442).
```

An address the relay doesn't allow is refused and nothing is sent (exit `2`):

```text
rhythm echo: x@evil.test isn't on the relay's allowlist; nothing was sent
```

Every attempt, sent or refused, is logged in the inbox as its own entry, outside any action, so it never counts in a comparison:

```text
2a5397874054  14:33:12 email to ann@client.example: Your Bean & Leaf order #1042 [faked]
20a3da3c9442  14:33:12 request to ann@client.example: Released: Your Bean & Leaf order #1042 [sent]
c43871858db9  14:33:12 request to x@evil.test: Released: Your Bean & Leaf order #1042 [blocked]
```

| Rule | Why |
|---|---|
| The account comes from `RHYTHM_ECHO_RELAY` (or `relay: url:`), as `smtp://user:pass@host:587` (STARTTLS) or `smtps://user:pass@host:465` (TLS). | It holds a password: the environment is better than a committed file. |
| Echo never sends a login or a message in clear: a server without STARTTLS is refused. | The account's password and the client's email stay private. |
| Only addresses matching `allow` get a release; an empty list allows none. | A release can't be turned into a way to mail anyone. |
| Releasing needs Echo's token or a viewer signed in with `--login`. | `up` refuses to start a relay without one of them. |
| Bcc lines are dropped; the message gets `X-Echo-Release: <id>`. | The client gets only their copy. |

Errors at start (exit `2`) say what's missing: `relay: needs the SMTP account to send through: set RHYTHM_ECHO_RELAY to smtp://user:pass@host:587 (or url: under relay: in .echo/config.yaml)`, `relay: releasing needs Echo's token or a viewer login; set --token (or RHYTHM_ECHO_TOKEN) or --login` and `relay: allow: lists no address, so nothing could be released; add the client's, e.g. "*@client.example"`. When the account refuses the message: `rhythm echo: the relay at <host:port> didn't take it: <reason>` (exit `3`). The relay runs on `rhythm echo up`, not on the [agency hub](Echo-Hub-And-Reports).

## Mailpit clients

Echo answers Mailpit's API too (`/api/v1/messages`, `/api/v1/message/{ID}`, `/api/v1/search` and more), so tools and tests written against Mailpit read Echo Inbox's emails unchanged. See [API](Echo-API#mailpit-style-routes).

---
<sub>← [Sealing staging](Echo-Sealing-Staging) · [Home](Home) · [Messages and checks](Echo-Messages-And-Checks) →</sub>
