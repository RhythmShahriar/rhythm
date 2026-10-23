Answers to common questions and fixes for common problems. If yours isn't here, check the [CLI reference](Tuner-CLI-Reference) or open an issue (see [the end of this page](#reporting-a-false-positive-or-a-bug)).

## Scanning problems

### "can't reach" a site

```text
rhythm tuner: yoursite.com: can't reach yoursite.com: Get "http://yoursite.com": dial tcp: lookup yoursite.com: no such host
  check the address and that the site is up
```

Tuner couldn't load the home page, so it couldn't scan anything. Work through these:

1. **Check the address.** A typo, a missing subdomain, or a site that only answers on `www.`.
2. **Check the site is up from where you are.** `curl -I https://yoursite.com` from the same machine. A site that loads in your browser may be unreachable from a server, a CI runner or a VPN.
3. **Firewalls and allowlists.** A firewall that blocks unknown IPs drops the connection before any page loads. Allowlist the scanning machine; see [Cloudflare and other firewalls](Tuner-Requests#cloudflare-and-other-firewalls).
4. **Slow sites.** Raise the per-request timeout: `--timeout 30s`.
5. **Proxies.** On a network that requires a proxy, set `HTTPS_PROXY` or pass `--proxy`. See [Proxies](#proxies-and-corporate-networks).

For a bare hostname, Tuner tries `https://` first and then `http://`; the error shows the last attempt. Run with `-v` to see every request. In a [batch scan](Tuner-Batch-Scans) the other sites carry on, and the run exits `3`.

### Invalid TLS certificate

```text
rhythm tuner: staging.yoursite.com: can't reach staging.yoursite.com: invalid TLS certificate: x509: certificate signed by unknown authority (rerun with --insecure to scan anyway)
```

The site's certificate is self-signed, expired, issued for another name, or missing an intermediate certificate. To scan it anyway, add `-k`:

```sh
rhythm tuner scan https://staging.yoursite.com -k
```

Tuner still checks the certificate separately and reports `tls-cert-untrusted` or `tls-cert-expired`. On a production site, fix the certificate; on staging, `-k` is usually fine.

If **every** https site fails this way, something on your network is intercepting TLS; see [Proxies and corporate networks](#proxies-and-corporate-networks).

### The site answers "200 OK" for everything (soft 404)

Some sites return a normal page with status `200` for URLs that don't exist. Tuner detects this before running checks; the report says `Baseline  soft-404 detected, filtering enabled`. Every hit on a sensitive path is then compared with a random path of the same shape (same folder, same extension, dotfile or not), and ignored if the two responses match. You don't need to do anything.

If you still see a finding you believe is a soft-404 page, look at its evidence (`-o report.md` shows the request, status and matched text) and [report it](#reporting-a-false-positive-or-a-bug).

### The scan is slow, throttled or blocked

The default is five requests a second per site. If the stats line says `throttled N× (slowed down)`, the site answered `429` or `503` and Tuner backed off. Lower the rate from the start (`--rate 1` or `--rate 2`), or allowlist the scan in the firewall. If many requests come back `403` or as a challenge page (check with `-v`), a firewall is blocking the scan and results will be incomplete. See [Requests and politeness](Tuner-Requests#retries-and-throttling).

To put an upper bound on a run, use `--max-time 10m`. Partial results are still reported.

## Understanding results

### Why isn't my plugin detected?

A remote scan only sees plugins and themes the **home page** loads: it looks for `/wp-content/plugins/<slug>/` and `/wp-content/themes/<slug>/` in asset URLs. A plugin that loads nothing on the home page (an admin-only tool, a form on one page, a plugin that combines its assets) is invisible from outside.

Other reasons a version is missing or wrong:

- **Quick profile.** `--profile quick` doesn't read `readme.txt` or `style.css`, so versions come only from hints in asset URLs. Use the default profile.
- **Blocked files.** Some hosts block `readme.txt`; then the version can't be read.
- **Many plugins.** At most 50 plugins and themes get a version lookup per scan.

For a complete, exact inventory, run a [local scan](Tuner-Local-Scans) on the server: it reads every plugin folder and the installed version from the plugin's own header.

### Why is the PHP version hidden?

Many servers and page caches remove `X-Powered-By`, which is good practice. Tuner tries up to three uncacheable endpoints (such as `/xmlrpc.php`) to find it. If it's still hidden, the fingerprint shows `PHP` with no number, and you may get `php-version-hidden`: a medium, low-confidence finding listing recent critical PHP flaws and the releases that fix them.

To resolve it, check the version yourself: `php -v` on the server (the command line can run a different PHP than the web server), your hosting panel, or WordPress → Tools → Site Health → Info → Server. Or run a [local scan](Tuner-Local-Scans) with `--php` pointing at the PHP that serves the site. Then fix or dismiss the finding. See [Known vulnerabilities](Tuner-Known-Vulnerabilities).

### What does "likely" mean?

A likely finding (outline badge, `~ likely:` line, `"confidence": "low"` in JSON) is one Tuner can't confirm from where it stands. The most common reason is a distribution build: `Apache/2.4.57 (Debian)` or `PHP/7.4.3-4ubuntu2.19` may already include backported fixes even though the version number is old. Likely findings count half in the score. [Reading results](Tuner-Reading-Results#confidence-and-likely-findings) lists every reason and what to do.

### Why is my score 0?

The [Tune score](Tuner-Reading-Results#the-tune-score) starts at 100 and loses 30 points per critical check, 15 per high, 6 per medium and 2 per low, never going below 0. Four critical findings are enough to reach 0. An old site with a few vulnerable plugins, end-of-life PHP and an exposed file gets there quickly.

Don't chase the number; work through the findings from the top. Fixing one critical check is worth 30 points, so the score climbs fast once the worst items are fixed.

### Is this a false positive?

Look at the evidence first. Save a report (`-o report.md`): each finding shows the request, the status code and the text that matched. Then check by hand, for example by opening the URL in a private browser window. Common cases:

- **A version-based finding on a distro build.** Marked likely; check your distribution's security tracker.
- **A vulnerability in a feature you don't use.** Still worth updating; the vulnerable code is on the server.
- **A file blocked for most visitors but not the scanner** (an IP allowlist, for example). It's still exposed to anyone on that list.

If it's a genuine false positive, [report it](#reporting-a-false-positive-or-a-bug).

### How do I hide findings I've accepted?

Two ways:

- **Skip the check** for good: `--exclude-id hdr-csp-missing`.
- **Baseline** what you've reviewed: save a JSON report once, then pass it to later scans. Findings already in it (matched by check ID and URL) are hidden and don't count in the score or for `--fail-on`; new ones still show.

```sh
rhythm tuner scan yoursite.com -o accepted.json
rhythm tuner scan yoursite.com --baseline accepted.json
```

See [CI and code scanning](Tuner-CI).

## Environment

### Can I use Tuner offline?

Yes, once it has cached data. Vulnerability data and the check pack are cached during normal scans and refreshed once they're a day old. `--offline` uses only the cache and never contacts the data services:

```sh
rhythm tuner update                       # while online: refresh everything now
rhythm tuner scan yoursite.com --offline  # later, without internet
```

Components with nothing cached are reported in a warning (`vulnerability data not cached for N components; run once online to cache it`). If the data services are unreachable during a normal scan, Tuner uses the cached copy and says so. For networks with no internet access, point Tuner at internal mirrors with `TUNER_VULN_API`, `TUNER_WPORG_API`, `TUNER_PHPNET_API`, `TUNER_OSV_API` and `TUNER_CHECKS_URL`. See [Known vulnerabilities](Tuner-Known-Vulnerabilities).

The scan itself still needs to reach the site, or use a [local scan](Tuner-Local-Scans).

### Proxies and corporate networks

Tuner uses the standard proxy variables (`HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`), or `--proxy` to override them. HTTP, HTTPS and SOCKS5 proxies work, and the proxy is used for the scan, the vulnerability data and the check pack.

```sh
rhythm tuner scan yoursite.com --proxy http://proxy.corp.example:3128
```

If your network inspects TLS with its own certificate authority, every https site fails as "invalid TLS certificate". Don't reach for `-k`: add your company's CA certificate to the system trust store, which Tuner uses. On Linux you can also point `SSL_CERT_FILE` at a bundle that includes it.

Behind a proxy, Tuner skips the TLS 1.0/1.1 test, because it needs a direct connection. See [Requests and politeness](Tuner-Requests#proxies).

### Windows notes

- **Install** with PowerShell: `irm https://rythm.sh/install.ps1 | iex`. See [Install](Install).
- **Colors** work in Windows Terminal and recent PowerShell consoles. Use `--plain` or `$env:NO_COLOR = "1"` if a console shows stray characters.
- **Basic auth headers.** PowerShell has no `base64` command:

  ```powershell
  $auth = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("user:pass"))
  rhythm tuner scan https://staging.yoursite.com -H "Authorization: Basic $auth"
  ```

- **Local scans** take Windows paths: an existing folder such as `C:\inetpub\wwwroot\yoursite`, or a relative one such as `.\yoursite`. The `wp-config.php` permissions check is skipped on Windows, and the session-settings check reads Linux PHP configuration paths, so it's most useful on the server itself.
- **Quote globs** in PowerShell too: `--id 'leak-*'`.

### Where is data cached?

Under your user cache directory:

| OS | Cache directory |
|---|---|
| Linux | `~/.cache/rhythm/tuner` (or `$XDG_CACHE_HOME/rhythm/tuner`) |
| macOS | `~/Library/Caches/rhythm/tuner` |
| Windows | `%LocalAppData%\rhythm\tuner` |

Inside it, `vulns/` holds the vulnerability data (WPVulnerability, wordpress.org, php.net, and `osv/` for Composer packages), and `checks/` holds the check pack. `rhythm tuner update` refreshes both and prints the cache path; `rhythm tuner update --clear` deletes them.

The one-time responsible-use notice is remembered in your user config directory (`~/.config/rhythm` on Linux).

## Reporting a false positive or a bug

Open an issue at [github.com/RhythmShahriar/tuner/issues](https://github.com/RhythmShahriar/tuner/issues). Include:

- the output of `rhythm tuner --version`, and `check_set` / `check_pack` from a JSON report;
- the finding's **ID**, severity and evidence (request, status, matched text);
- why you believe it's wrong, and what the URL actually returns;
- the fingerprint line, if it's a version-based finding.

> [!WARNING]
> Never paste unmasked secrets, credentials, private URLs or client data into a public issue. Tuner's output is already masked; keep it that way, and replace your real domain with `yoursite.com` if you need to.

Want a new check, or have one ready? See [Writing checks](Tuner-Writing-Checks) and [Contributing](Contributing).

### Found a security issue in Tuner itself?

Don't open a public issue. Report it privately as described in the [Security policy](Security-Policy).

---
<sub>← [CLI reference](Tuner-CLI-Reference) · [Home](Home) · [Responsible use](Responsible-Use) →</sub>
