An agency looks after many client sites, each with its own staging copy. `rhythm echo hub` is one Echo for all of them: each site sends with a write token of its own that can only add captures, one read token sees every site. The inbox survives restarts. `rhythm echo schedule` runs each site's update window against it. `rhythm echo report --site` turns those windows into a report for the client, as HTML or PDF. This page covers the hub, connecting sites to it and the agency report; the update windows are in [Schedule](Echo-Schedule).

```sh
rhythm echo hub --store /srv/echo --sites sites.yaml          # the hub
rhythm echo schedule "Tue 02:00" --store /srv/echo --after "wp plugin update --all"
rhythm echo report --site client-a --store /srv/echo --pdf    # the client's report
```

## sites.yaml

```yaml
read_token_env: ECHO_HUB_READ_TOKEN
sites:
  - id: client-a
    token_env: ECHO_TOKEN_CLIENT_A
    config: /home/you/projects/shop/.echo/config.yaml
    schedule: "Tue 02:00"
```

| Key | Means |
|---|---|
| `read_token_env` | The variable holding the hub's read token. Default `RHYTHM_ECHO_TOKEN`. |
| `sites[].id` | The site's ID: letters, digits, dots, dashes and underscores. It names the site in every capture, in search (`site:client-a`) and in its report. |
| `sites[].token_env` | The variable holding that site's write token. |
| `sites[].config` | The site's `.echo/config.yaml`, for `schedule`. A relative path is relative to `sites.yaml`. |
| `sites[].schedule` | When its update window runs, as `schedule` reads it. |

Tokens never go in the file, only the names of the variables that hold them. Every site needs a token of its own, different from the read token. Unknown keys are refused, as are mistakes: `sites.yaml: site "client-a": set ECHO_TOKEN_CLIENT_A to its write token`, `… share a token; give each site a token of its own`, `… its token is the read token; give each site a token of its own` (exit `2`).

## The hub

```sh
export ECHO_HUB_READ_TOKEN=$(openssl rand -hex 16) ECHO_TOKEN_CLIENT_A=$(openssl rand -hex 16)
rhythm echo hub --store /home/you/hub/store --sites /home/you/hub/sites.yaml
```

```text
Echo hub (seal mode): 1 sites, 0 messages kept in /home/you/hub/store
  API   http://127.0.0.1:8025  (read token: ECHO_HUB_READ_TOKEN)
  Inbox http://127.0.0.1:8025/?token=<read token>  (open once in a browser)
  SMTP  127.0.0.1:1025  (log in with any username and the site's write token)
  Site  client-a  (write token: ECHO_TOKEN_CLIENT_A)
```

| Flag | Default | Does |
|---|---|---|
| `--store <dir>` | (required) | Where the inbox, TLS files and results are kept. |
| `--sites <file>` | (required) | The `sites.yaml` above. |
| `--listen <addr>` | `127.0.0.1:8025` | The API and Echo Inbox. |
| `--smtp <addr>` | `127.0.0.1:1025` | The SMTP sink; `off` turns it off. Log in with any username and the site's write token. |
| `--seal`, `--observe` | seal | As for `up`. |
| `--allow <host>` | | A host reachable while sealed (repeat, or a comma list). |
| `--keep <n>` | `10000` | Messages kept. |
| `--tls-cert <file> --tls-key <file>` | | Serve the API over HTTPS and offer STARTTLS on SMTP. |
| `--tls-self-signed` | | The same with a certificate the hub makes and keeps in the store (valid 825 days, replaced 30 days before it runs out or when a name is missing). |
| `--tls-host <names>` | | More names or IPs for that certificate (comma list). |

Beyond loopback, every token needs 16 characters or more and the hub warns while tokens would cross the network in clear:

```text
Echo hub (seal mode): 1 sites, 0 messages kept in /home/you/hub3
  API   http://[::]:28025  (read token: ECHO_HUB_READ_TOKEN)
  Inbox http://[::]:28025/?token=<read token>  (open once in a browser)
  Warning: tokens cross the network in clear; add --tls-self-signed, or --tls-cert and --tls-key.
  Site  client-a  (write token: ECHO_TOKEN_CLIENT_A)
```

With `--tls-self-signed --tls-host hub.agency.example` it serves HTTPS and prints the certificate's fingerprint, to check from the sites:

```text
Echo hub (seal mode): 1 sites, 0 messages kept in /home/you/hub2
  API   https://[::]:28025  (read token: ECHO_HUB_READ_TOKEN)
  Inbox https://[::]:28025/?token=<read token>  (open once in a browser)
  TLS   certificate SHA-256 D9:77:15:FA:BB:AB:4D:11:27:0E:EB:5A:00:AD:CB:AC:4D:68:B8:D3:72:F8:10:F9:18:86:97:63:5D:FA:CE:F3
  Site  client-a  (write token: ECHO_TOKEN_CLIENT_A)
```

The store keeps the inbox in `inbox/` (owner-only, kept across restarts), the certificate in `tls/` and each site's snapshots and runs in `sites/<id>/`. A hub on a store an older hub wrote (`inbox.jsonl`) moves those messages into the new inbox once.

### Tokens

| Token | May |
|---|---|
| A site's write token | Send captures, begin, renew and end its actions, read `/v1/health`. Its captures are marked with its site. Nothing else: `a site's write token can only send captures; reading needs the hub's read token` (`403`). |
| The read token | Read everything: every site's messages, `GET /v1/sites`, Echo Inbox. It can't send captures: `the read token can't send captures; each site uses its own write token`. |

```sh
curl -s -H "X-Echo-Token: $ECHO_HUB_READ_TOKEN" http://127.0.0.1:8025/v1/sites
```

```json
[{"id":"client-a","messages":2,"last":"2026-10-25T14:29:12.056328207Z","schedule":"Tue 02:00"}]
```

`GET /v1/messages?site=client-a` (or `q=site:client-a`) lists one site's messages; an unknown site is `404` `no site "client-z"`. The rest of Echo's [API](Echo-API) works with the read token, as do the CLI's reading commands:

```sh
RHYTHM_ECHO_TOKEN=$ECHO_HUB_READ_TOKEN rhythm echo ls
```

```text
f5fc368e5069  14:29:12 email to jamie@shop.test: Thanks for getting in touch [faked]
ae1306214786  14:29:12 Zapier webhook to hooks.zapier.com/hooks/catch/1234567/ab•••••: POST [faked]
```

Echo Inbox is at `/`. A browser opens `/?token=<read token>` once; the hub keeps the token in an HttpOnly cookie that opens only Echo Inbox, not the API.

## Connecting a site

Each site's connector reports to the hub with its write token and names itself with its ID. For WordPress:

```sh
RHYTHM_ECHO_WRITE_TOKEN=$ECHO_TOKEN_CLIENT_A rhythm echo install wordpress /var/www/staging --hub https://hub.agency.example:8025 --site client-a
```

`--hub` needs `--site` (`rhythm echo: --hub needs --site, the site's ID in the hub's sites.yaml`) and the write token in `RHYTHM_ECHO_WRITE_TOKEN` (`rhythm echo: --hub needs the site's write token: set RHYTHM_ECHO_WRITE_TOKEN`); it never takes `--token`. For Laravel and Node, set the variables in the site's environment:

| Variable | Does |
|---|---|
| `RHYTHM_ECHO_URL` | The hub's URL, `https://hub.agency.example:8025`. |
| `RHYTHM_ECHO_WRITE_TOKEN` | The site's write token, sent as `X-Echo-Token`. |
| `RHYTHM_ECHO_SITE_ID` | The site's ID, put on every capture. |
| `RHYTHM_ECHO_TOKEN` | Optional: checks the signature of `X-Echo-Run`. With only a write token set, the write token does that. |

Mail over SMTP logs in with any username and the site's write token as the password, which marks it with the site too. The token is a plain HTTP header: beyond a private network, serve the hub over TLS and give the sites an `https://` URL.

### One machine, several sites

`rhythm echo up --sites sites.yaml` reads the same file and gives each site its own write token on a normal `up`, without a store, a schedule or reports:

```text
Sites: client-a, client-b: each sends with its own write token (captures only); the read token (ECHO_HUB_READ_TOKEN) reads them all
```

Use the hub for an agency's many clients; use `up --sites` to keep two local projects apart.

## The agency report

After [`schedule`](Echo-Schedule) has run a site's update windows, `report --site` writes one page for the client: did the site keep sending what it should after each update?

```sh
rhythm echo report --site client-a --store /home/you/hub/store
```

```text
Agency report for client-a: client-a-report.html (1 update window since 25 Sep 2026)
```

The page shows the agency's logo and name, the site and the period (`Email and webhook report · 25 Sep to 25 Oct 2026`), a headline (*Something broke after updates*, *Nothing broke; some messages changed*, *Nothing broke after updates*, *Some checks could not run*, *No comparisons in this period*), the totals per class, a table of runs (when, compared with, the result, what was sent) and what changed in each window, with each change's title and actions. It never shows message bodies, payloads or diffs: the client sees verdicts and change titles only. It's styled for printing on A4 and loads nothing from outside.

| Flag | Default | Does |
|---|---|---|
| `--site <id>` | (required) | The hub site to report on. |
| `--store <dir>` | (required) | The hub's store. |
| `--since <d>` | `30d` | The period, such as `90d`. |
| `--pdf` | | Also print it to a PDF beside the HTML, with Chrome or Chromium. |
| `-o <file>` | `<site>-report.html` | `.html` names the HTML; `.pdf` writes the PDF there and the HTML beside it. |
| `--brand-name <name>` | | The agency's name. |
| `--brand-logo <file>` | | The agency's logo: PNG, JPEG, GIF, WebP or SVG, embedded in the page. |
| `--brand-color <#hex>` | `#E8603A` | The accent color, `#RGB` or `#RRGGBB`. |
| `--open` | | Open the PDF, else the HTML. |

```sh
rhythm echo report --site client-a --store store -o client-a.pdf --brand-name 'North Star Studio' --brand-color '#1F6FEB'
```

```text
Agency report for client-a: client-a.html (1 update window since 25 Sep 2026)
PDF: client-a.pdf
```

`--pdf` looks for Chrome, Chromium or Edge on `PATH`, then in `/Applications` on macOS and under `%ProgramFiles%`, `%ProgramFiles(x86)%` and `%LocalAppData%` on Windows. It prints with a throwaway profile and no network. Both files are written `0600`. Without a browser the HTML is still written (exit `3`):

```text
rhythm echo: no Chrome or Chromium found on PATH: install one, or open the HTML report in a browser and print it to PDF; the HTML report is client-a-report.html
```

Other errors:

```text
rhythm echo: no update windows for site "client-b" in /home/you/hub/store since 25 Sep 2026; rhythm echo schedule runs them
rhythm echo: --site reads the hub's results: name the hub's store with --store
rhythm echo: --site "../x": a hub site's id is letters, digits, dots, dashes and underscores, as in sites.yaml
rhythm echo: --brand-color "blue": want #RGB or #RRGGBB, e.g. #1F6FEB
rhythm echo: the agency report is HTML (-o <file>.html), or a PDF with -o <file>.pdf
rhythm echo: --since, --pdf and the --brand flags are for the agency report: add --site <id>
```

The first exits `3`, the others `2`.

---
<sub>← [Docker and DDEV](Echo-Docker-And-DDEV) · [Home](Home) · [Schedule](Echo-Schedule) →</sub>
