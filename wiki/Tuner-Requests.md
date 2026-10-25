Tuner is polite by default: five requests a second to a public site, a slower pace when it pushes back and an honest user agent. This page covers every flag that shapes the requests a remote scan sends. It also covers how to scan staging sites, sites behind a login and sites behind Cloudflare or another firewall.

None of these flags apply to [local scans](Tuner-Local-Scans), which send no requests to the site.

## Request flags

| Flag | Default | What it does |
|---|---|---|
| `--rate <n>` | `5`, or `50` for local and private addresses | Maximum requests per second to each host, shared by every target on that host. Decimals work (`--rate 0.5` is one every two seconds). `0` means no limit until the site throttles. |
| `-c, --concurrency <n>` | `10` | Requests in flight at once for a target, for every part of the scan: checks, soft-404 probes, version lookups, the PHP probe and TLS tests. |
| `--timeout <d>` | `10s` | Timeout for each request, as a duration with a unit: `30s`, `1m`. It also applies to the vulnerability data services. |
| `--retries <n>` | `2` | Extra attempts after a network error or a `429`/`503` reply. `0` disables retries. |
| `--max-time <d>` | none | Stop the whole run after this long, e.g. `--max-time 5m`. Partial results are still reported. |
| `-A, --user-agent <ua>` | `rhythm-tuner/<version> (+https://rythm.sh/tuner)` | The `User-Agent` header. |
| `-H, --header <h>` | | An extra header, `"Name: value"`. Repeatable. `Host:` sets the virtual host. |
| `--cookie <c>` | | A `Cookie` header, e.g. `"wordpress_logged_in_abc=…"`. |
| `--proxy <url>` | `HTTPS_PROXY` | An `http://`, `https://`, `socks5://` or `socks5h://` proxy. |
| `-k, --insecure` | off | Don't verify TLS certificates. Certificate problems are still reported as findings. |
| `--follow-host` | off | Keep scanning when the home page redirects to another host. See [Redirects](#redirects). |
| `--plugins <mode>` | `passive` | How hard to look for WordPress plugins the home page doesn't load. See [Plugin discovery](#plugin-discovery). |
| `--request-budget <n>` | `2000` | Most requests plugin discovery may send. |

**Where headers and cookies go.** `-H` headers, `--cookie` and the credentials in a `https://user:pass@host` target go to the host you named (or its `www` twin) and nowhere else: never to another host a redirect leads to, never over plain http once the target is https and never to the vulnerability data services. Tuner checks this on every request and on every redirect hop. Values with line breaks or control characters are refused before any request; the error never shows the value.

## Rate and concurrency

`--rate` spaces requests evenly so a host never sees more than that many per second; `--concurrency` caps how many are open at once. The rate is usually the limit that matters.

```sh
rhythm tuner scan yoursite.com --rate 2          # gentle on a small shared host
rhythm tuner scan yoursite.com --rate 20 -c 20   # your own server, in a hurry
```

**Your own servers are faster by default.** Without `--rate`, a target on `localhost`, `*.localhost`, a loopback, private or link-local address (or a name that resolves only to such addresses) gets 50 requests a second instead of 5. The header says so: `scanning 127.0.0.1:20400 (local address: 50 req/s)`. An explicit `--rate`, even `--rate 5`, always wins. Behind a proxy only literal addresses and `localhost` count, since names aren't resolved locally.

**Per host, not per target.** When you scan several targets with `--parallel`, targets on the same host share one pace, so four paths of one site still see at most `--rate` requests a second together. Different hosts each get their own. See [Batch scans](Tuner-Batch-Scans).

## Retries and throttling

**Network errors** (a refused connection, a reset, a timeout) are retried up to `--retries` times, waiting half a second, then a second, then a second and a half.

**Throttling.** When a site answers `429 Too Many Requests` or `503 Service Unavailable`, Tuner:

1. Waits as long as the `Retry-After` header asks, in seconds or as an HTTP date, capped at 30 seconds. Without the header it waits 1s, then 2s, then 4s.
2. Retries, up to `--retries` times.
3. **Halves its request rate for the rest of the scan.** Each further throttling reply halves it again, down to one request every two seconds.

If the site still answers `429` or `503` after the last retry, Tuner keeps that response and moves on. The stats line at the end says how often it happened (`throttled 3× (slowed down)`). The JSON report has the count in `throttled`.

`--rate 0` means no limit only until the site pushes back: the first `429` or `503` drops the scan to four requests a second, halving on each later one down to one every two seconds. Tuner honors `Retry-After` either way.

> [!TIP]
> If a scan is throttled often, start lower: `--rate 1` or `--rate 2`. You'll finish sooner than with repeated back-offs and won't trip rate-limit bans.

## Timeouts and --max-time

`--timeout` limits each request. When the home page gets no answer on any try, the target fails with a message that says so:

```text
$ rhythm tuner scan http://127.0.0.1:20402 --timeout 1s
rhythm tuner scan: http://127.0.0.1:20402: can't reach 127.0.0.1:20402: no response within 1s (tried 3 times)
  check the address and that the site is up
```

Raise `--timeout` if the site is just slow.

`--max-time` limits the whole run, across every site:

```sh
rhythm tuner scan -l sites.txt --max-time 10m
```

When the time runs out, Tuner stops sending requests and reports what it found so far. Each report says `Incomplete: stopped by --max-time` above the score. stderr says `<target>: stopped by --max-time; results are partial`. JSON reports say `"complete": false, "stopped_by": "max-time"`. The exit code is `3`, or `1` when a finding reaches `--fail-on`; with `--fail-on` and nothing at the level, a partial scan can't pass unless you add `--allow-partial`. See [exit codes](Tuner-CLI-Reference#exit-codes).

## Redirects

- **The home page follows redirects** within the site: http to https (on any port) and the apex to `www` or back. It keeps the **path you gave**, so a scan of `yoursite.com/blog` stays under `/blog`.
- **A redirect to another host stops the scan**, so a site can't send your scan, or your credentials, somewhere you didn't choose:

  ```text
  $ rhythm tuner scan 127.0.0.1:20401
  rhythm tuner scan: 127.0.0.1:20401: 127.0.0.1:20401 redirects to localhost:20400; scan that host directly if you own it, or pass --follow-host
  ```

  With `--follow-host` the scan continues on the new host without your headers and cookies. It says so:

  ```text
  note: 127.0.0.1:20401 redirects to localhost:20400; scanning localhost:20400 because of --follow-host, without your headers and cookies
  ```
- **A public site can't redirect the scan into your network.** Redirects and connections to loopback, link-local (`169.254.0.0/16`, `fe80::/10`, cloud metadata addresses), private, carrier-grade NAT (`100.64.0.0/10`), multicast and reserved addresses are refused unless the target itself is one, as are IPv6 forms that carry such an IPv4 address. A name that changes its answer mid-scan (DNS rebinding) is refused the same way. With `--proxy` or a `*_PROXY` variable, Tuner connects to the proxy, so only the redirect check by name applies.
- **Checks don't follow redirects.** A check for `/.env` sees the `301` or `302` itself, not the page it points to. A redirect to a login page or the home page is never mistaken for an exposed file.

## Targets without a scheme

A bare hostname (`yoursite.com`, `127.0.0.1:8080`) is tried over `https://` first, then `http://`. A server that refuses port 443, hangs up on the TLS hello or answers plain http there counts as "no https": one try, no error counted, then plain http.

With `-H`, `--cookie` or a password in the target, Tuner won't fall back for any other https failure, so your credentials never travel in clear by accident:

```text
rhythm tuner scan: 127.0.0.1:20400: can't reach 127.0.0.1:20400 over https: Get "https://127.0.0.1:20400": tls: first record does not look like a TLS handshake (your headers and cookies are never sent over plain http; give http://127.0.0.1:20400 to scan over http)
```

Give a full URL to choose the scheme yourself. `http://` in the target scans over http on purpose.

## Plugin discovery

A WordPress plugin that loads nothing on the home page is invisible to the page itself. Tuner looks further:

| `--plugins` | What it does | Extra requests |
|---|---|---|
| `passive` (default) | reads the REST API index (`/wp-json/`) and maps the namespaces it lists to plugins (`contact-form-7/v1` to `contact-form-7`, `yoast/v1` to `wordpress-seo`) | 1 |
| `deep` | also asks for `readme.txt` of the 1,500 most installed plugins on wordpress.org, compared against a baseline probe so a site that answers every path can't fake plugins | up to `--request-budget` (default 2000) |

Each plugin records how it was found (`found_by`: `asset`, `rest`, `readme`) and a `confidence` in the JSON report. Namespaces Tuner knows from a plugin's own code give high confidence; ones guessed from a name, or shared by a free and a premium edition, give low. Many hosts block `/wp-json/` for anonymous visitors; then passive discovery finds nothing extra.

`--plugins deep` probes are plain GET requests that share `--rate` and `-c`. A budget you set makes the scan partial when it runs out:

```text
$ rhythm tuner scan 127.0.0.1:20400 --plain --plugins deep --request-budget 40
…
Incomplete: stopped at the request budget
Score 0/100 - 3 critical, 3 high, 1 medium, 7 low, 1 info - 77 checks, 218 requests, 4.3s
note: plugin discovery stopped at its budget of 40 requests: results are partial; raise --request-budget to probe more plugins
```

A plugin outside the 1,500 most installed is found only when the page or the REST API names it, even when it has known advisories.

## User agent

Tuner identifies itself honestly by default:

```text
rhythm-tuner/0.4.0 (+https://rythm.sh/tuner)
```

That makes its requests easy to spot in logs and to allowlist in a firewall. Change it with `-A`:

```sh
rhythm tuner scan yoursite.com -A "acme-agency-audit/1.0 (+https://yoursite.com/security)"
```

A `-H "User-Agent: …"` header also works and takes precedence over `-A`. The same user agent is used for the vulnerability data and check pack downloads.

## Headers and virtual hosts

`-H` adds a header to every request to the host you named. Repeat it for more than one:

```sh
rhythm tuner scan yoursite.com -H "X-Scan-Token: 3f9c…" -H "Accept-Language: en"
```

A header must look like `Name: value` with a valid header name; anything else is a usage error (`bad -H: use "Name: value"`).

**`Host:`** sets the virtual host. Use it to scan a specific server, such as the origin behind a CDN or a new server before DNS moves to it:

```sh
rhythm tuner scan http://203.0.113.10 -H "Host: yoursite.com"
```

Over `https://` the certificate won't match the IP address, so add `-k` and expect a `tls-cert-untrusted` finding that doesn't apply to the real hostname.

## Cookies: scanning as a logged-in user

`--cookie` sends a `Cookie` header with every request to the site, for example a WordPress login cookie copied from your browser's developer tools:

```sh
rhythm tuner scan https://staging.yoursite.com --cookie "wordpress_logged_in_abc123=…"
```

Some sites only reveal certain pages or headers to logged-in users. Tuner's checks still send only GET or HEAD requests. They run with that user's rights, though, so use a low-privilege account where you can. Log the session out afterwards.

## Proxies

`--proxy` sends every request through a proxy: the scan itself, the vulnerability data, the check pack and, in local scans, the OSV.dev queries.

```sh
rhythm tuner scan yoursite.com --proxy http://127.0.0.1:8080
rhythm tuner scan yoursite.com --proxy socks5://127.0.0.1:1080
```

Without `--proxy`, Tuner uses the standard environment variables: `HTTPS_PROXY` for https URLs, `HTTP_PROXY` for http URLs and `NO_PROXY` for exceptions (lower-case names work too).

```sh
export HTTPS_PROXY=http://proxy.corp.example:3128
rhythm tuner scan yoursite.com
```

`socks5h://` resolves hostnames on the proxy instead of locally. When requests go through a proxy, Tuner skips the TLS 1.0/1.1 test, since those direct connections would bypass the proxy.

## Invalid certificates: -k

By default Tuner verifies certificates and stops on a site whose certificate can't be verified:

```text
$ rhythm tuner scan https://127.0.0.1:20403
rhythm tuner scan: https://127.0.0.1:20403: can't reach 127.0.0.1:20403: invalid TLS certificate: x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs (rerun with --insecure to scan anyway)
  check the address and that the site is up
```

`-k` (`--insecure`) scans anyway. The certificate is still checked separately and reported as `tls-cert-untrusted` or `tls-cert-expired`, so you don't lose the finding:

```text
[HIGH] TLS certificate not trusted - https://127.0.0.1:20403
  evidence: x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs
  fix: Use a certificate from a public CA (Let's Encrypt is free) that covers this hostname. Serve the full chain.
```

```sh
rhythm tuner scan https://staging.yoursite.com -k
```

## Scanning staging sites

Staging sites usually sit behind HTTP basic auth, often with a self-signed certificate:

```sh
rhythm tuner scan https://staging.yoursite.com \
  -H "Authorization: Basic $(printf '%s' "$STAGING_AUTH" | base64)" -k
```

Here `STAGING_AUTH` holds `user:password`. Keeping it in an environment variable (or a CI secret) keeps the password out of your shell history. A literal also works: `-H "Authorization: Basic $(printf 'user:pass' | base64)"`. So does `https://user:pass@staging.yoursite.com`, whose password Tuner shows as `xxxxx` everywhere it prints the target. In a [`.tuner.yml`](Tuner-Config), headers read only `${TUNER_…}` variables.

## Cloudflare and other firewalls

A web application firewall (WAF) or CDN can get in the way of any scanner. What you might see:

- **Blocked or challenged requests.** Tuner recognizes challenge and block pages from Cloudflare, Sucuri, Imperva, Wordfence and Akamai. When the home page is one, or more than half of the check requests get one, the report says `Incomplete: blocked by Cloudflare challenge` (or the firewall's name) and the JSON report `"complete": false, "stopped_by": "waf"`. Block pages never become findings. A blocked scan never reads as a clean one. Run with `-v` to log every request and its status to stderr.
- **Throttling.** `429` and `503` replies slow Tuner down, as described [above](#retries-and-throttling).
- **A hidden PHP version.** Page caches often strip `X-Powered-By`. Tuner probes uncacheable endpoints to find it; if the version stays hidden you'll see `php-version-hidden` instead. See [FAQ](Tuner-FAQ).

What helps:

1. **Allowlist the scan** in the firewall for the duration, by your IP address or by Tuner's user agent. This gives the most accurate results.
2. **Lower the rate**, e.g. `--rate 1`, so rate rules don't trigger.
3. **Scan the origin directly** with `-H "Host: yoursite.com"`, if you can reach it. Remember that the firewall may block things the origin exposes; findings on the origin are still worth fixing.
4. **Run a [local scan](Tuner-Local-Scans)** on the server, which doesn't go through the firewall at all.

> [!IMPORTANT]
> Don't try to evade a firewall you don't control. Scan only sites you own or have permission to test. Coordinate with whoever runs the firewall.

---
<sub>← [Choosing checks](Tuner-Choosing-Checks) · [Home](Home) · [Output and reports](Tuner-Output) →</sub>
