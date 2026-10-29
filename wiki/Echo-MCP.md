`rhythm echo mcp` is an [MCP](https://modelcontextprotocol.io) server: it lets a coding agent read what the site sent, get the one-time code from a sign-up email, run your actions, snapshot and compare, the way you do with the CLI. This page covers adding it to an agent, its eight tools, what it masks and how it finds the project.

```sh
claude mcp add echo -- rhythm-echo mcp
```

Start Echo first (`rhythm echo up`); the server talks to it like the CLI does. Any client that speaks MCP over stdin and stdout works the same way:

```json
{
  "mcpServers": {
    "echo": {"command": "rhythm-echo", "args": ["mcp"]}
  }
}
```

## Tools

| Tool | Title | Takes | Gives |
|---|---|---|---|
| `list_messages` | List captured messages | `channel`, `to`, `run`, `action`, `since` (`15m` or an RFC 3339 time), `limit` (1 to 200, default 20) | `{total, messages: [{id, at, channel, provider, status, from, to, subject, event, method, url, run, action, error}]}`, newest first |
| `get_message` | Get one message | `id`, or `latest` | The whole message, masked |
| `latest_code` | Latest one-time code | `to`, `channel` (`email`, `sms` or `whatsapp`), `since` | `{code, message_id}` |
| `run_action` | Run actions | `actions` | The run, as `rhythm echo run` |
| `snapshot` | Take a snapshot | `name`, `actions` | The snapshot; replaces one of the same name |
| `compare` | Compare with a snapshot | `snapshot`, `actions` | `verdict`, `snapshot`, `counts`, `changes`, `run` |
| `check_domain` | Check a sending domain | `domain`, `provider` | SPF, DKIM, DMARC and MX, as `rhythm echo check` |
| `check_consent` | Check cookie consent on a page | `url`, `runs`, `consent_mode`, `geo` | The `consent` envelope, as `rhythm echo consent --json`: no choice, Accept and Reject in a real browser, tracking hits held, Consent Mode decoded |

`check_consent` needs no running Echo: it loads the page in Chrome and keeps its result under `.echo/consent`, like the CLI. Hits are always held; the MCP server can't send them. Point it only at sites you own or are authorised to test. See [Cookie consent](Echo-Consent).

`compare`'s `verdict` is `Something broke.`, `Something changed.`, `Only fixes: nothing broke or changed.` or `Nothing changed.`. The run tools keep runs and snapshots where the CLI does and run one at a time.

## An exchange

What a client sends and gets, one JSON-RPC message per line, from a run against the test shop. First the handshake:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"docs","version":"1"}}}
```

```json
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":{"listChanged":false}},"instructions":"Echo records every email, SMS, webhook and API call a site sends. list_messages and get_message read what was captured; latest_code gives a one-time code for a login or sign-up test; run_action, snapshot and compare run the actions in .echo/config.yaml; check_domain checks SPF, DKIM, DMARC and MX. Credentials in results are masked.","protocolVersion":"2025-06-18","serverInfo":{"name":"rhythm-echo","title":"Echo","version":"1.1.1"}}}
```

The code from the sign-in email the test just triggered:

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"latest_code","arguments":{"to":"ann@shop.test"}}}
```

```json
{"jsonrpc":"2.0","id":4,"result":{"content":[{"text":"{\"code\":\"482913\",\"message_id\":\"d159729c67c5\"}","type":"text"}],"isError":false,"structuredContent":{"code":"482913","message_id":"d159729c67c5"}}}
```

A tool that fails says why, in the result:

```json
{"jsonrpc":"2.0","id":6,"result":{"content":[{"text":"no message \"nope\" in Echo; list_messages shows what there is","type":"text"}],"isError":true}}
```

The server speaks protocol versions `2025-06-18`, `2025-03-26` and `2024-11-05`, answers `ping` and stops a call when the client cancels it. Results come as JSON text and as `structuredContent`. An unknown tool or argument is error `-32602`, an unknown method `-32601`. Nothing but protocol messages goes to stdout; it exits `0` when stdin closes.

## Masking

Results are masked as Echo masks what it stores: credential headers whole, keys and tokens in text and URLs, the secret part of a hook URL, one-time codes in the stored text. The one thing an agent gets unmasked is the code `latest_code` returns, which Echo reads from the message before it masks it. `get_message` shows the code as `•••••`.

## Which project

The action tools use the nearest `.echo/config.yaml`, in the folder the agent starts in or one above it, with the same bounds as the CLI (it stops at the repository root and below your home folder). Runs and snapshots go next to that config and command, flush and replay actions run in the folder that holds it. `--config <file>` picks another. A config in a folder anyone can write to is skipped, with a note on stderr: `rhythm echo mcp: <reason> (someone else could have put it there); pass --config <path> if you trust it`.

| Flag | Default | Does |
|---|---|---|
| `--url <url>` | the config's `listen`, or `RHYTHM_ECHO_URL` | Where Echo runs. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | The token Echo was started with. |
| `--config <file>` | the nearest `.echo/config.yaml` | The config to read. |

Exit codes: `0` stdin closed, `2` a usage error or a bad config, `3` couldn't read or write. An unknown flag: `rhythm echo mcp: unknown flag --bogus` (exit `2`).

> [!NOTE]
> The tools run what `.echo/config.yaml` says, including `command` actions. Give an agent this server only in projects whose config you'd run yourself.

---
<sub>← [Schedule](Echo-Schedule) · [Home](Home) · [Conformance kit](Echo-Conformance) →</sub>
