Tuner is polite by default: five requests a second, a slower pace when a site pushes back and an honest user agent. This page covers every flag that shapes the requests a remote scan sends. It also covers how to scan staging sites, sites behind a login and sites behind Cloudflare or another firewall.

None of these flags apply to [local scans](Tuner-Local-Scans), which send no requests to the site.

## Request flags

| Flag | Default | What it does |
|---|---|---|
| `--rate <n>` | `5` | Maximum requests per second, per site. Decimals work (`--rate 0.5` is one every two seconds). `0` means unlimited. |
| `-c, --concurrency <n>` | `10` | Requests in flight at once, per site. |
| `--timeout <d>` | `10s` | Timeout for each request, as a duration: `30s`, `1m`. |
| `--retries <n>` | `2` | Extra attempts after a network error or a `429`/`503` reply. `0` disables retries. |
| `--max-time <d>` | none | Stop the whole run after this long, e.g. `--max-time 5m`. Partial results are still reported. |
| `-A, --user-agent <ua>` | `rhythm-tuner/<version> (+https://rythm.sh/tuner)` | The `User-Agent` header. |
| `-H, --header <h>` | | An extra header, `"Name: value"`. Repeatable. `Host:` sets the virtual host. |
| `--cookie <c>` | | A `Cookie` header, e.g. `"wordpress_logged_in_abc=…"`. |
| `--proxy <url>` | `HTTPS_PROXY` | An `http://`, `https://`, `socks5://` or `socks5h://` proxy. |
| `-k, --insecure` | off | Don't verify TLS certificates. Certificate problems are still reported as findings. |

Headers and cookies go with every request to the scanned site, including the home page, the PHP version probe and every check. They're never sent to the vulnerability data services.

## Rate and concurrency

`--rate` spaces requests evenly so a site never sees more than that many per second; `--concurrency` caps how many are open at once. The rate is usually the limit that matters.

```sh
rhythm tuner scan yoursite.com --rate 2          # gentle on a small shared host
rhythm tuner scan yoursite.com --rate 20 -c 20   # your own server, in a hurry
```

Both limits are **per site**. When you scan several sites with `--parallel`, each one has its own limiter, so the total request rate from your machine is up to `--rate` times `--parallel`. See [Batch scans](Tuner-Batch-Scans).

## Retries and throttling

**Network errors** (a refused connection, a reset, a timeout) are retried up to `--retries` times, waiting half a second, then a second, then a second and a half.

**Throttling.** When a site answers `429 Too Many Requests` or `503 Service Unavailable`, Tuner:

1. Waits as long as the `Retry-After` header asks, in seconds or as an HTTP date, capped at 30 seconds. Without the header it waits 1s, then 2s, then 4s.
2. Retries, up to `--retries` times.
3. **Halves its request rate for the rest of the scan.** Each further throttling reply halves it again, down to one request every two seconds.

If the site still answers `429` or `503` after the last retry, Tuner keeps that response and moves on. The stats line at the end says how often it happened (`throttled 3× (slowed down)`). The JSON report has the count in `throttled`.

With `--rate 0` there's no limiter to slow down. Tuner still honors `Retry-After` and retries.

> [!TIP]
> If a scan is throttled often, start lower: `--rate 1` or `--rate 2`. You'll finish sooner than with repeated back-offs and won't trip rate-limit bans.

## Timeouts and --max-time

`--timeout` limits each request. `--max-time` limits the whole run, across every site:

```sh
rhythm tuner scan -l sites.txt --max-time 10m
```

When the time runs out, Tuner stops sending requests and reports what it found so far, with `stopped by --max-time; results are partial` for each site that didn't finish. Those sites count as not scanned, so the exit code is `3` (unless a finding hits `--fail-on`). See [exit codes](Tuner-CLI-Reference).

## Redirects

- **The home page follows redirects.** Tuner keeps the scheme and host it lands on, so `http://yoursite.com` that redirects to `https://www.yoursite.com` is scanned at `https://www.yoursite.com`. It keeps the **path you gave**, so a scan of `yoursite.com/blog` stays under `/blog`.
- **Checks don't follow redirects.** A check for `/.env` sees the `301` or `302` itself, not the page it points to. A redirect to a login page or the home page is never mistaken for an exposed file.
- **Bare hostnames** are tried over `https://` first, then `http://` if that fails. Give a full URL to choose the scheme yourself.

## User agent

Tuner identifies itself honestly by default:

```text
rhythm-tuner/0.3.1 (+https://rythm.sh/tuner)
```

That makes its requests easy to spot in logs and to allowlist in a firewall. Change it with `-A`:

```sh
rhythm tuner scan yoursite.com -A "acme-agency-audit/1.0 (+https://yoursite.com/security)"
```

A `-H "User-Agent: …"` header also works and takes precedence over `-A`. The same user agent is used for the vulnerability data and check pack downloads.

## Headers and virtual hosts

`-H` adds a header to every request. Repeat it for more than one:

```sh
rhythm tuner scan yoursite.com -H "X-Scan-Token: 3f9c…" -H "Accept-Language: en"
```

A header must look like `Name: value`; anything else is a usage error.

**`Host:`** sets the virtual host. Use it to scan a specific server, such as the origin behind a CDN or a new server before DNS moves to it:

```sh
rhythm tuner scan http://203.0.113.10 -H "Host: yoursite.com"
```

Over `https://` the certificate won't match the IP address, so add `-k` and expect a `tls-cert-untrusted` finding that doesn't apply to the real hostname.

## Cookies: scanning as a logged-in user

`--cookie` sends a `Cookie` header with every request, for example a WordPress login cookie copied from your browser's developer tools:

```sh
rhythm tuner scan https://staging.yoursite.com --cookie "wordpress_logged_in_abc123=…"
```

Some sites only reveal certain pages or headers to logged-in users. Tuner still sends only GET requests. They run with that user's rights, though, so use a low-privilege account where you can. Log the session out afterwards.

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
rhythm tuner: staging.yoursite.com: can't reach staging.yoursite.com: invalid TLS certificate: x509: certificate signed by unknown authority (rerun with --insecure to scan anyway)
  check the address and that the site is up
```

`-k` (`--insecure`) scans anyway. The certificate is still checked separately and reported as `tls-cert-untrusted` or `tls-cert-expired`, so you don't lose the finding.

```sh
rhythm tuner scan https://staging.yoursite.com -k
```

## Scanning staging sites

Staging sites usually sit behind HTTP basic auth, often with a self-signed certificate:

```sh
rhythm tuner scan https://staging.yoursite.com \
  -H "Authorization: Basic $(printf '%s' "$STAGING_AUTH" | base64)" -k
```

Here `STAGING_AUTH` holds `user:password`. Keeping it in an environment variable (or a CI secret) keeps the password out of your shell history. A literal also works: `-H "Authorization: Basic $(printf 'user:pass' | base64)"`.

## Cloudflare and other firewalls

A web application firewall (WAF) or CDN can get in the way of any scanner. What you might see:

- **Blocked or challenged requests.** Checks get a `403` or a challenge page instead of the real response, so real problems can go unreported. Run with `-v` to log every request and its status to stderr.
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
