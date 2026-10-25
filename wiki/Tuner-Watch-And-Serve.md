Two commands keep Tuner running for you. `rhythm tuner watch` scans a list of sites on a schedule, keeps every run's report and says what changed since the last one. `rhythm tuner serve` runs Tuner as an HTTP API: your app registers sites, each proves its owner asked for the scan and the API queues scans and posts the results to a webhook. Both run each scan as its own `rhythm tuner scan` process.

## watch: scheduled scans with a digest

```sh
rhythm tuner watch -l clients.txt --at 02:00 --fail-on-new high --html   # every night at 02:00
rhythm tuner watch example.com --every 6h -- --profile quick             # every six hours
rhythm tuner watch -l clients.txt --once                                 # one run, for cron and CI
```

Each run prints one line. Here are two `--once` runs of a list with one test site, before and after its `.env` was fixed and a `debug.log` appeared:

```text
$ rhythm tuner watch -l clients.txt --once --html
2026-10-25 14:29 UTC: 1 site, 15 findings (4 critical, 2 high, 1 medium, 7 low, 1 info); report /home/you/.cache/rhythm/tuner/watch/clients/20261025T142932Z.json and /home/you/.cache/rhythm/tuner/watch/clients/20261025T142932Z.html
$ rhythm tuner watch -l clients.txt --once --html --fail-on-new high
2026-10-25 14:29 UTC: 1 site, 15 findings (3 critical, 3 high, 1 medium, 7 low, 1 info); 1 new, 1 fixed since the last run; report /home/you/.cache/rhythm/tuner/watch/clients/20261025T142937Z.json and /home/you/.cache/rhythm/tuner/watch/clients/20261025T142937Z.html (scan exit 1)
```

The second run exited 1: the new `debug.log` finding is high.

### Schedule

| Flag | |
|---|---|
| `--every <interval>` | run now, then every interval from the previous start, e.g. `24h` or `6h` (at least `1m`). A start missed while a run overran is skipped, not caught up |
| `--at <HH:MM>` | run every day at this local time; several separated by commas, e.g. `02:00,14:00` |
| `--now` | with `--at`, also run once right away |
| `--once` | run once now and exit with the scan's exit code (for cron and CI) |

The watch checks its sleep against the wall clock every minute, so a suspended laptop or a clock change doesn't delay runs.

### Targets, state and what each run does

| Flag | |
|---|---|
| `-l, --list <file>` | sites to scan, one per line, read again on every run. Sites can also follow `watch` on the command line |
| `--name <name>` | the watch's name (default: the list file's name, or the first site's host) |
| `--dir <dir>` | where reports and logs go (default: your user cache folder, `rhythm/tuner/watch/<name>`) |
| `--keep <n>` | runs to keep, oldest deleted first (default 30; `0` keeps all) |
| `--exec <binary>` | the Tuner binary to run (default: this one) |
| `--html` | also render each run's report as HTML next to it |

Each run runs `rhythm-tuner scan -l <list> --json -o <dir>/<UTC time>.json`, plus `--since <previous report>` from the second run on. It writes the scan's output to `<dir>/<UTC time>.log`, which starts with the command line:

```text
$ rhythm-tuner scan -l /home/you/clients.txt --json -o /home/you/.cache/rhythm/tuner/watch/clients/20261025T142937Z.json --since /home/you/.cache/rhythm/tuner/watch/clients/20261025T142932Z.json --fail-on-new high
```

The folder then holds the run files, `last.json` (the last good report and its exit code) and `watch.lock`:

```text
20261025T142932Z.html
20261025T142932Z.json
20261025T142932Z.log
20261025T142937Z.html
20261025T142937Z.json
20261025T142937Z.log
last.json
watch.lock
```

Only a scan that exited 0 or 1 with a readable report becomes the next run's comparison; after a failed or interrupted run, the next one still compares with the last complete report. `--keep` deletes only the watch's own run files, never anything else in the folder.

### Passed to every scan

| Flag | |
|---|---|
| `--notify <url>` | tell this destination about new findings (repeatable); the first run sends every finding. See [notifications](Tuner-Changes-And-Notifications#tell-someone---notify) |
| `--notify-min <level>` | lowest severity to send |
| `--fail-on-new <level>` | exit 1 if a new finding is at or above this severity, from the second run on |
| `-- <scan flags>` | everything after `--` goes to `scan` as written, e.g. `-- --profile quick --rate 2` |

The watch sets the output flags itself, so they can't go after `--`:

```text
$ rhythm tuner watch -l clients.txt --once -- --json
rhythm tuner: the watch sets --json itself; drop it after --
run 'rhythm tuner watch --help' for its flags
```

### Locks and signals

Two runs of one watch never overlap: a lock in its folder turns the second away, so a `--once` cron job and a long-running watch of the same name can't collide. The first Ctrl-C or SIGTERM lets the current scan finish, then the watch stops; a second one stops the scan too.

`watch` scans sites by URL. For site folders on the server, run `rhythm tuner scan /var/www --discover` from cron; see [Integrity and malware](Tuner-Integrity-And-Malware#running-it-regularly).

## serve: an HTTP API with ownership checks

`rhythm tuner serve` is for agencies and hosts that want to start scans from their own app. A site is scanned only after it proves its owner asked for it, by publishing a token Tuner hands out.

```text
$ export TUNER_SERVE_TOKEN=$(openssl rand -hex 32)
$ rhythm tuner serve --listen 127.0.0.1:20420 --data serve --webhook http://127.0.0.1:20410/serve
2026-10-25T14:33:12Z listening on 127.0.0.1:20420
```

Every request needs `Authorization: Bearer $TUNER_SERVE_TOKEN`. A POST body needs `Content-Type: application/json`. Without the token variable, the API has no authentication, so it listens on loopback only and answers only requests addressed to `localhost`, `127.0.0.1` or `[::1]` (a web page can't reach it through DNS rebinding). It refuses anything else:

```text
$ rhythm tuner serve --listen 0.0.0.0:20421
rhythm tuner: refusing to listen on 0.0.0.0:20421 without TUNER_SERVE_TOKEN: anyone who can reach it could queue scans; set the token, or listen on 127.0.0.1
run 'rhythm tuner serve --help' for its flags
```

The token must be at least 16 characters. Tuner removes it and the webhook secret from each scan's environment.

### Endpoints

| Request | |
|---|---|
| `GET /v0/health` | status, version, workers, queued and running scans |
| `POST /v0/targets` | `{"url": "https://example.com"}`: register a site and get its token |
| `GET /v0/targets`, `GET /v0/targets/{id}`, `DELETE /v0/targets/{id}` | list, read or remove targets |
| `POST /v0/targets/{id}/verify` | `{"method": "dns"}` or `{"method": "file"}`; both when the body is empty |
| `POST /v0/scans` | queue a scan: `target_id` plus optional `profile`, `tags`, `exclude_tags`, `ids`, `exclude_ids`, `severity`, `no_vulns` and `webhook_url` |
| `GET /v0/scans/{id}` | status, summary, findings and the last 200 log lines |
| `GET /v0/scans/{id}/events` | live Server-Sent Events: `queued`, `started`, `finding`, `log`, `done` |

Errors are JSON, `{"error": "…"}`, with a fitting status: 401 for a missing token, 403 for an unverified target, 415 for a body without the JSON content type, 429 when the queue is full.

### 1. Register a site

```text
$ curl -s -H "Authorization: Bearer $TUNER_SERVE_TOKEN" -H "Content-Type: application/json" \
    -d '{"url":"http://127.0.0.1:20400"}' 127.0.0.1:20420/v0/targets
{
  "id": "t_5d73afd84268b1f9",
  "url": "http://127.0.0.1:20400",
  "host": "127.0.0.1:20400",
  "token": "tuner-verify=02c0ee37f40aec76b007d0ff8c813c1d",
  "created": "2026-10-25T14:33:13.759261576Z",
  "is_verified": false,
  "verify": {
    "file": {
      "url": "http://127.0.0.1:20400/.well-known/tuner-verify.txt",
      "body": "tuner-verify=02c0ee37f40aec76b007d0ff8c813c1d"
    }
  }
}
```

For a site with a host name, `verify` also has a `dns` block: a TXT record at `_tuner-verify.<host>` holding the token. An IP address can only use the file. URLs with a user name, query or fragment are refused; the error never repeats the credentials.

### 2. Prove ownership

Publish the token, as a TXT record or as the file `/.well-known/tuner-verify.txt`, then ask Tuner to check. The file is fetched from the target's own scheme, host and port; redirects must stay on that host and an https check never drops to http.

```text
$ curl -s -X POST -H "Authorization: Bearer $TUNER_SERVE_TOKEN" 127.0.0.1:20420/v0/targets/t_5d73afd84268b1f9/verify
{
  "error": "ownership not proven for 127.0.0.1:20400",
  "checks": [
    {
      "method": "file",
      "where": "http://127.0.0.1:20400/.well-known/tuner-verify.txt",
      "ok": false,
      "error": "HTTP 404"
    }
  ]
}
```

Once the file is there, the same call answers 200 with the target, now `"is_verified": true`, `"verified_by": "file"` and a `verification_expires` 30 days later. After that, verify again. Asking for DNS on an IP address gets a clear answer:

```text
{
  "error": "method dns: 127.0.0.1 is an IP address, which has no DNS name for a TXT record; use file"
}
```

### 3. Queue a scan

```text
$ curl -s -i -H "Authorization: Bearer $TUNER_SERVE_TOKEN" -H "Content-Type: application/json" \
    -d '{"target_id":"t_5d73afd84268b1f9","profile":"quick"}' 127.0.0.1:20420/v0/scans
HTTP/1.1 202 Accepted
Location: /v0/scans/s_44a0bf56130800b5
…
{
  "id": "s_44a0bf56130800b5",
  "target_id": "t_5d73afd84268b1f9",
  "url": "http://127.0.0.1:20400",
  "status": "queued",
  "options": {
    "profile": "quick"
  },
  "created": "2026-10-25T14:33:13.828752819Z",
  "summary": {},
  "findings": []
}
```

The 202 shows the scan as it was queued; follow `Location` for its current state. A target that isn't verified gets 403: `target t_… isn't verified; publish its token and POST /v0/targets/t_…/verify`. Scans run one at a time per site and `--workers` (default 2) at a time overall, each with `--max-scan-time` (default 30 minutes). Status goes `queued`, `running`, then `done` (exit 0 or 1), `failed`, `canceled` or `interrupted` (the server stopped while it ran).

### 4. Follow it

`GET /v0/scans/{id}/events` streams the scan as Server-Sent Events. Each `finding` event carries one finding as JSON, the same as `scan --jsonl`:

```text
id: 1
event: queued
data: {"id":"s_44a0bf56130800b5"}

id: 2
event: started
data: {"id":"s_44a0bf56130800b5","url":"http://127.0.0.1:20400"}

id: 3
event: finding
data: {"id":"hdr-content-type-options-missing","title":"X-Content-Type-Options header missing","severity":"low",…}
```

The stream replays from `Last-Event-ID`, pings every 15 seconds and closes after `done`. `GET /v0/scans/{id}` returns the finished record:

```json
{
  "id": "s_44a0bf56130800b5",
  "target_id": "t_5d73afd84268b1f9",
  "url": "http://127.0.0.1:20400",
  "status": "done",
  "options": {
    "profile": "quick"
  },
  "created": "2026-10-25T14:33:13.828752819Z",
  "started": "2026-10-25T14:33:13.828963845Z",
  "finished": "2026-10-25T14:33:14.156886659Z",
  "exit_code": 0,
  "summary": {
    "critical": 3,
    "high": 2,
    "info": 1,
    "low": 6,
    "medium": 1
  }
}
```

plus its `findings` (up to 5,000) and `log`. The server log shows each step:

```text
2026-10-25T14:33:13Z target t_5d73afd84268b1f9 added: http://127.0.0.1:20400
2026-10-25T14:33:13Z target t_5d73afd84268b1f9 verified by file
2026-10-25T14:33:13Z scan s_44a0bf56130800b5 queued for http://127.0.0.1:20400
2026-10-25T14:33:13Z scan s_44a0bf56130800b5 started
2026-10-25T14:33:14Z scan s_44a0bf56130800b5 done
2026-10-25T14:33:14Z scan s_44a0bf56130800b5: webhook delivered to 127.0.0.1:20410
```

### Webhooks

`--webhook <url>` posts every finished scan to your endpoint; a scan's own `webhook_url` gets that scan. Each delivery is signed with `TUNER_WEBHOOK_SECRET` (or `--webhook-secret`, which `ps` can show, so prefer the variable) and carries these headers:

| Header | |
|---|---|
| `X-Tuner-Event` | `scan.completed` |
| `X-Tuner-Delivery` | the scan ID |
| `X-Tuner-Signature` | `sha256=<hex HMAC-SHA256 of the raw body>` |

The body lists each finding's ID, title, severity and URL, never evidence. This delivery is from a second quick scan of the same target (13 findings, the rest cut here):

```json
{
  "event": "scan.completed",
  "sent": "2026-10-25T14:45:26.324301461Z",
  "scan": {
    "id": "s_35c5784d5bd97d4a",
    "target_id": "t_5d73afd84268b1f9",
    "url": "http://127.0.0.1:20400",
    "status": "done",
    "exit_code": 0,
    "started": "2026-10-25T14:45:26.004488027Z",
    "finished": "2026-10-25T14:45:26.323760909Z",
    "summary": {
      "critical": 3,
      "high": 2,
      "info": 1,
      "low": 6,
      "medium": 1
    },
    "findings": [
      {
        "id": "hdr-content-type-options-missing",
        "title": "X-Content-Type-Options header missing",
        "severity": "low",
        "url": "http://127.0.0.1:20400"
      },
      …
    ]
  }
}
```

Check the signature against the raw body before you trust it. With the body saved as `body.json`:

```text
$ openssl dgst -sha256 -hmac "$TUNER_WEBHOOK_SECRET" body.json
HMAC-SHA2-256(body.json)= 34bb88a3a62bdd64377f1f8a8f797e8af68b3103c48c740e551b82321d8e4dd5
```

That matched the delivery's `X-Tuner-Signature: sha256=34bb88a3a62bdd64377f1f8a8f797e8af68b3103c48c740e551b82321d8e4dd5`. Deliveries never follow redirects, time out after 10 seconds and are retried after 5 seconds, 30 seconds and 2 minutes when the receiver fails or answers 5xx or 429.

A scan's own `webhook_url` must be a public address, so a caller can't aim deliveries at your internal network. Names are checked on the address they resolve to when the delivery connects. Per-scan deliveries ignore the proxy variables:

```text
{
  "error": "webhook_url: 10.0.0.5 is a private or local address; start serve with --webhook-allow-private to allow it"
}
```

`--webhook` is yours and may be any address.

### Flags and data

| Flag | Default | |
|---|---|---|
| `--listen <addr>` | `127.0.0.1:8080` | anything but loopback needs `TUNER_SERVE_TOKEN` |
| `--data <dir>` | your user cache folder, `rhythm/tuner/serve` | `targets.json` and `scans/<id>.json` |
| `--workers <n>` | `2` | scans at once across all sites |
| `--max-scan-time <d>` | `30m` | stop a scan after this long |
| `--max-queue <n>` | `100` | queued scans before new ones get 429 |
| `--webhook <url>` | none | post every finished scan here, signed |
| `--webhook-secret <s>` | `TUNER_WEBHOOK_SECRET` | the HMAC key |
| `--webhook-allow-private` | off | let a scan's `webhook_url` reach loopback, private and link-local addresses |
| `--exec <binary>` | this one | the Tuner binary to run scans with |

On SIGINT or SIGTERM the server cancels running scans, marks queued ones canceled, gives webhooks up to 10 seconds and exits 0. Scans found queued or running at the next start are marked `interrupted`.

### In a container

Each release publishes a signed multi-arch image, `ghcr.io/rhythmshahriar/tuner:<version>` (plus `:v0` and `:latest` for the newest stable release). It runs `rhythm-tuner serve --listen 0.0.0.0:8080` as a non-root user on a distroless base, so it needs the token:

```sh
docker run -d -p 127.0.0.1:8080:8080 -e TUNER_SERVE_TOKEN="$TUNER_SERVE_TOKEN" \
  -v tuner-data:/home/nonroot/.cache ghcr.io/rhythmshahriar/tuner:0.4.0
```

Verify the image before you run it:

```sh
cosign verify ghcr.io/rhythmshahriar/tuner:0.4.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity https://github.com/RhythmShahriar/tuner/.github/workflows/release.yml@refs/tags/v0.4.0
```

The image's entrypoint is `rhythm-tuner`, so `docker run ghcr.io/rhythmshahriar/tuner:0.4.0 scan …` runs a one-off scan instead.

> [!IMPORTANT]
> serve checks ownership. Scanning internal sites is a valid use for hosts, though, so its scans and ownership checks can reach private addresses. Run it where only your app can reach it, keep the token secret and scan only sites whose owners asked.

---
<sub>← [Config file](Tuner-Config) · [Home](Home) · [Writing checks](Tuner-Writing-Checks) →</sub>
