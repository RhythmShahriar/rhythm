If you scan WordPress sites with Wordfence CLI or WPScan today, this page maps what you run now to Tuner, says what carries over and is honest about what doesn't. Tuner is one binary, needs no API key and uses free vulnerability data with no daily quota.

> [!NOTE]
> Wordfence CLI's [README](https://github.com/wordfence/wordfence-cli) says it's no longer sold to new users and that its free version reached end of life on October 14, 2026. WPScan's free API is [capped at 25 calls a day](https://wpscan.com/pricing/) for non-commercial use, with a paid license for commercial use. Check both pages for their current terms.

## Coming from Wordfence CLI

Wordfence CLI scans a server's files. Tuner's equivalent is a [local scan](Tuner-Local-Scans): point it at a folder.

| You ran | With Tuner |
|---|---|
| `wordfence malware-scan /var/www/site` | `rhythm tuner scan /var/www/site --integrity`: malware signatures run in every local scan; `--integrity` adds core and plugin checksum comparison |
| `wordfence vuln-scan /var/www/site` | `rhythm tuner scan /var/www/site`: exact versions from disk matched against known vulnerabilities, plus `wp-config.php` review and stray files |
| several sites, one per command | `rhythm tuner scan /var/www --discover`: every WordPress and Laravel install under the folder, one report each |
| `--output-format csv`, `--output-path` | `-o report.json`, `-o report.md`, `-o report.sarif` or `--html client.html` |
| a license key | nothing: no key, no account. An optional free [Wordfence Intelligence](Tuner-Known-Vulnerabilities#wordfence-intelligence) key adds its feed to the matching |
| cron with email on findings | `--fail-on critical` for the exit code, or `--since last.json --notify smtp://…` for an email listing only what's new; see [Changes and notifications](Tuner-Changes-And-Notifications) |
| `wordfence remediate` | not offered: Tuner never changes files. The integrity findings' fixes give the `wp core download --force` and `wp plugin install --force` commands to restore originals |

What carries over and what doesn't:

| | Tuner 0.4.0 |
|---|---|
| **Changed core files** | yes: every core file's MD5 against WordPress.org's checksums for the installed release and locale |
| **Changed plugin files** | yes, for plugins hosted on wordpress.org (SHA-256 per file). Premium and custom plugins have no public checksums; they're listed in one info finding |
| **Unknown PHP in core and plugins** | yes |
| **Malware signatures** | nine rules for common patterns: `eval(base64_decode())`, compressed payloads, `assert()` and `create_function()` on request input, `preg_replace /e`, known web shell markers, `auto_prepend_file` into uploads, hidden admin users in must-use plugins and PHP in uploads. Wordfence's commercial signature set is far larger; Tuner's rules catch the common cases, not everything |
| **Themes** | versions matched against advisories; no checksum comparison (WordPress.org publishes none) |
| **Speed and load** | one pass over the files, read in parallel; at most 1 MB of most files and the whole file for checksums |
| **Quarantine, repair** | no. Tuner only reads |

The [Integrity and malware](Tuner-Integrity-And-Malware) page covers the details, including what the signatures skip.

## Coming from WPScan

WPScan scans from outside, over HTTP. So does a Tuner [remote scan](Tuner#how-a-remote-scan-works).

| You ran | With Tuner |
|---|---|
| `wpscan --url https://example.com` | `rhythm tuner scan https://example.com` |
| `--api-token …` for vulnerability data | nothing: vulnerability data is free and has no quota |
| `--enumerate vp` (vulnerable plugins) | every scan: plugins from the page's assets and the REST API index, matched against advisories |
| `--plugins-detection aggressive` | `--plugins deep`: also checks the `readme.txt` of the 1,500 most installed plugins, within `--request-budget` |
| `--enumerate u` (users) | the `wp-user-enum-rest` and `wp-user-enum-rest-route` checks report that user names are public; Tuner doesn't list them |
| `--format json --output out.json` | `--json -o out.json`, or `--sarif` for GitHub code scanning |
| `--http-auth user:pass` | `-H "Authorization: Basic …"`, or `https://user:pass@host` (the password is masked everywhere it's printed) |
| `--cookie-string` | `--cookie` |
| `--disable-tls-checks` | `-k` (the certificate problem is still reported) |
| `--proxy` | `--proxy`, or `HTTPS_PROXY` |
| `--throttle`, `--max-threads` | `--rate`, `-c` |
| `--random-user-agent` | not offered: Tuner always says who it is |

Each plugin Tuner finds says how: `found_by` in JSON is `asset`, `rest` or `readme`, with a `confidence`. A plugin guessed from a REST namespace (one a free and a premium edition share, say) has `"confidence": "low"`: confirm it's installed before you act on its findings.

What Tuner adds in the same scan: exposed files and secrets (masked), security headers, cookies, TLS, end-of-life PHP and servers, outdated, closed and abandoned plugins, a fix for every finding and [server config snippets](Tuner-Output#server-config-from-a-report) that fix them. What it doesn't do: it never sends exploit payloads, logs in or guesses passwords; checks only send GET or HEAD requests.

> [!IMPORTANT]
> `--plugins deep` sends up to 2,000 extra requests by default. It's still paced by `--rate` and stops at its budget. Run it only on sites you own or have permission to test. Expect a firewall to notice.

## In both cases

- **Install:** `curl -fsSL https://rythm.sh/install | sh` (see [Install](Install)), or the [GitHub Action](Tuner-GitHub-Action) in CI.
- **Read the report:** [Reading results](Tuner-Reading-Results) explains severities, *likely* findings and the score.
- **Accepted risk:** a [`.tuner.yml`](Tuner-Config) with ignore entries that carry a reason and an expiry date.
- **What changed:** [`--since` and `diff`](Tuner-Changes-And-Notifications).

---
<sub>← [CLI reference](Tuner-CLI-Reference) · [Home](Home) · [FAQ and troubleshooting](Tuner-FAQ) →</sub>
