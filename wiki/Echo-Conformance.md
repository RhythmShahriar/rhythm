A connector is a small program inside someone else's site, so a bug in one can lose a message or let it out. Echo's **conformance kit** checks a connector against the protocol it speaks: `rhythm echo conformance` runs a strict stand-in Echo that checks every capture against the wire schema. The repository holds shared test vectors that the WordPress, Laravel and Node connectors all run. This page covers the stand-in, the scenarios it can expect, its report and the vectors; the protocol itself is in [API](Echo-API#the-capture-protocol).

```sh
rhythm echo conformance --expect email,run,http-before   # then point the connector's end-to-end test at it
```

## The stand-in

The stand-in answers like Echo (the same decisions, fakes and errors). It checks each `POST /v1/capture` against the wire schema first. A bad capture gets `400` with every problem and never reaches its inbox, so a connector's own end-to-end test fails on a protocol bug instead of passing silently.

```sh
rhythm echo conformance --listen 127.0.0.1:28025 --expect email,run,http-before
```

```text
rhythm echo conformance: stand-in Echo on http://127.0.0.1:28025 (seal, wire 2)
```

A good capture is answered as Echo would:

```sh
curl -s -H "Content-Type: application/json" -d @good.json http://127.0.0.1:28025/v1/capture
```

```json
{"id":"52b3ee51758a","mode":"seal","action":"fake"}
```

A capture with a field the protocol doesn't have is refused:

```sh
curl -s -H "Content-Type: application/json" -d @bad.json http://127.0.0.1:28025/v1/capture
```

```text
conformance: capture.http.header: not part of the wire protocol
```

Stop it with Ctrl+C or SIGTERM and it prints its verdict (exit `1` when any check failed, `0` when all passed):

```text
CHECK        RESULT  DETAIL
schema       FAIL    1 of 2 captures broke the wire schema: capture.http.header: not part of the wire protocol
email        PASS    1: an email reported before it is sent
http-before  FAIL    expected an HTTP request reported before it is sent, saw none
run          PASS    1: a capture tied to a run
wire-2       INFO    1: a capture in wire 2

refused capture 1:
  capture.http.header: not part of the wire protocol

FAIL
```

A run where no capture arrived at all fails with `captures  FAIL    no captures arrived`; one where every capture was valid and nothing was expected ends `PASS` (exit `0`).

`GET /v1/conformance` returns the report so far as JSON, for a test that checks it before stopping the stand-in:

```json
{"ok":false,"captures":2,"rows":[{"check":"schema","result":"fail","detail":"1 of 2 captures broke the wire schema: capture.http.header: not part of the wire protocol"},{"check":"email","result":"pass","detail":"1: an email reported before it is sent"},{"check":"http-before","result":"fail","detail":"expected an HTTP request reported before it is sent, saw none"},{"check":"run","result":"pass","detail":"1: a capture tied to a run"},{"check":"wire-2","result":"info","detail":"1: a capture in wire 2"}],"problems":[…]}
```

## Flags

| Flag | Default | Does |
|---|---|---|
| `--listen <addr>` | `127.0.0.1:8025` | The API address. |
| `--smtp <addr>` | off | Also run Echo's SMTP sink there. |
| `--mode <m>` | `seal` | `seal` or `observe`: what the stand-in decides. |
| `--token <t>` | | Require this `X-Echo-Token`. |
| `--allow <hosts>` | | Hosts reachable when sealed, comma separated. |
| `--wire <n>` | `2` | Speak this wire version, to check a connector falls back to an older Echo. |
| `--expect <list>` | | Scenarios the run must include, comma separated (below). |
| `--json` | | Print the report as JSON instead of a table. |

Exit codes: `0` every check passed, `1` a check failed, `2` usage error, `3` couldn't listen (`rhythm echo conformance: listen tcp 127.0.0.1:1: bind: permission denied`).

### Scenarios

| `--expect` | The run must include |
|---|---|
| `email` | an email reported before it's sent |
| `email-after` | an email's outcome reported after a real send (observe mode) |
| `http-before` | an HTTP request reported before it's sent |
| `http-after` | an HTTP request's outcome reported after it was sent |
| `run` | a capture tied to a run (`X-Echo-Run` or `RHYTHM_ECHO_RUN`) |
| `site` | a capture naming its hub site (`source.site`) |
| `attachments` | an email listing its attachments |
| `wire-1`, `wire-2` | a capture in that wire version |

The `schema` row is always there. With `--wire 1`, the stand-in's health says `"wire": 1` and a wire 2 capture gets `400` with `this connector is newer than Echo; run: rhythm update`, the words a connector falls back on; the `fallback` row then says whether it did (`refused 1 newer capture, then got wire 1`).

## The wire schema

The protocol is wire version 2 (Echo still accepts 1). Wire 2 adds `source.site` (the site's ID on an [agency hub](Echo-Hub-And-Reports)) and `email.attachments`, each file's name, type, size and SHA-256 for connectors that send no raw MIME. The schema is JSON Schema (draft 2020-12), strict: an unknown field is refused, so a typo shows. It's in the Echo repository at [`wire/schema.json`](https://github.com/RhythmShahriar/echo/blob/master/wire/schema.json). The stand-in's problems read like `missing (required)`, `not part of the wire protocol`, `"sms" is not one of email, http`, `3 is above 2, the newest this Echo speaks` and `new in wire 2 while this capture says v 1: leave it out for an older Echo`.

An Echo that receives a newer wire than it speaks answers `400` with `this connector is newer than Echo; run: rhythm update (it speaks wire 3, this Echo wire 2 at most)`. The connectors then ask `/v1/health`, send the older version and log it once (`[rhythm-echo] Echo at <url> speaks wire 1: sending that (rhythm update gets a newer Echo)`). They try the newer one again after an hour.

## The test vectors

[`conformance/testdata`](https://github.com/RhythmShahriar/echo/tree/master/conformance/testdata) holds the cases every connector's suite reads:

| Files | Cover |
|---|---|
| `capture-*.json` | valid captures and the decision Echo gives each: a sealed email, an observed one, a faked HTTP call, the site's own host, wire 1, wire 2 with a site and attachments |
| `broken-*.json` | captures the schema must refuse, with the problems expected |
| `fail-closed-*.json`, `fail-open-observe.json` | what a connector must do when Echo is unreachable, answers `5xx` or times out: refuse to send while sealed, send in observe mode |
| `run-signature.json` | signed `X-Echo-Run` values: which ones verify, which a connector must ignore, the 512-byte limit |

A connector for another stack can run the same cases. The repository's `conformance/README.md` describes the format.

---
<sub>← [MCP server](Echo-MCP) · [Home](Home) · [CI](Echo-CI) →</sub>
