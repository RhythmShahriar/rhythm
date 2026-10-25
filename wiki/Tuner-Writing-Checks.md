Tuner's remote checks are YAML files: one file per check, no Go code. This page covers the full check format, the fixtures every check needs, how to test and lint them with `rhythm tuner checks` (no Go needed), the rules every check follows and a worked example of adding a check from start to finish.

## Where checks live

Checks live in [`checks/<category>/`](https://github.com/RhythmShahriar/tuner/tree/master/checks) in the tuner repository, with a fixture file next to each one:

```text
checks/
  headers/  laravel/  leaks/  php/  server/  tls/  wordpress/
  leaks/dotenv.yaml           # the check
  leaks/dotenv_test.yaml      # its fixtures
  pack.json                   # check pack settings (min_tuner, retired)
  LICENSE
```

Built-in checks are compiled into the binary from `checks/*/*.yaml`, so a check sits exactly one folder deep. Once a check lands on `master`, it reaches users with the next Tuner release. [Check packs](Tuner-Check-Packs) can deliver checks between releases once a release trusts the pack signing key; Tuner 0.4.0 doesn't yet.

## The check format

```yaml
id: leak-dotenv                 # unique, lower-case words joined by hyphens
name: .env file exposed         # shown as the finding's title
severity: critical              # critical | high | medium | low | info
confidence: high                # optional: high (default) or low
profile: quick                  # optional: quick | default (default)
tags: [leak, secrets]
requires: [wordpress]           # optional: only run when the fingerprint finds this
request:
  method: GET                   # optional: GET (default) or HEAD, nothing else
  path: ["/.env", "/{{host}}.zip"]   # {{host}} expands to the target hostname
match:
  all:                          # every matcher must hold
    - status: 200
    - regex: '(?m)^[A-Z_]+=.+'
    - not_baseline: true        # must differ from the server's soft-404 reply
  any:                          # optional: at least one must hold
    - magic: "504b0304"
extract:
  secrets: true                 # report masked secrets found in the body
impact: Anyone can read database passwords.
fix: |
  First line is shown in the terminal; the rest is in JSON and Markdown output.
fix_server:                     # optional: snippets rhythm tuner fix bundles
  nginx: |
    location ~ /\.(?!well-known) { deny all; }
  apache: |
    <FilesMatch "^\.">
      Require all denied
    </FilesMatch>
references:
  - https://owasp.org/Top10/A05_2021-Security_Misconfiguration/
```

### Fields

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique across all checks, lower-case words joined by hyphens, prefixed by area: `leak-`, `hdr-`, `php-`, `tls-`, `wp-`, `laravel-` or `srv-` (server pages). It becomes the finding ID. Prefixes that vulnerability matching and local scans use (`vuln-`, `local-`, `wp-core-outdated`…) are refused. |
| `name` | yes | Short title for the finding. |
| `severity` | yes | `critical`, `high`, `medium`, `low` or `info`. Drives `--severity`, `--fail-on` and the [Tune score](Tuner-Reading-Results). |
| `confidence` | no | `high` (default) or `low`. Low-confidence findings show as *likely* and count half in the score. Use it when a match strongly suggests a problem but can't confirm it from outside. |
| `profile` | no | The cheapest [profile](#profiles) that runs the check: `quick` or `default` (the default). |
| `tags` | no | Used by `--tags` and `--exclude-tags`. Reuse existing tags where you can: `leak`, `secrets`, `headers`, `cookies`, `tls`, `php`, `wordpress`, `laravel`, `disclosure`… |
| `requires` | no | Technologies the [fingerprint](Tuner) must detect, any of `wordpress`, `laravel`, `php`. The check only runs when all are present. Leave it out for checks that make sense on any site: an exposed `.env` matters whatever the platform. |
| `request.method` | no | `GET` (the default) or `HEAD`, in any case. Any other method is refused when the check loads, built-in, from `--checks` or from a pack. |
| `request.path` | yes | One or more paths, each starting with `/`. Each path is a separate request and can produce its own finding. `{{host}}` expands to the target's hostname, e.g. `/{{host}}.zip` becomes `/yoursite.com.zip`. A query string may hold only plain `name=value` pairs (letters, digits, `_ . ~ -` and `/` in values), so `/?rest_route=/wp/v2/users` works and payloads don't load. |
| `match` | yes | `all` and/or `any` lists of [matchers](#matchers). At least one matcher in total. |
| `extract.secrets` | no | `true` to find credentials in the response body and report them masked. |
| `impact` | no | One or two sentences: what an attacker gains. |
| `fix` | no | How to fix it. The first line is shown in the terminal; put copy-pasteable config on the following lines. |
| `fix_server` | no | `nginx` and/or `apache` snippets as literal block scalars. Each must be valid inside an nginx `server { }` block or an Apache `<VirtualHost>`; `rhythm tuner explain` shows them and `rhythm tuner fix` bundles them. `checks lint` checks that braces and sections balance. |
| `references` | yes | At least one `https://` URL for further reading. |

Unknown fields are errors. A typo such as `regx:` fails to load with a clear message instead of silently matching nothing. That's also how older Tuner binaries refuse a [check pack](Tuner-Check-Packs) that uses a matcher they don't have.

### How a check runs

- Check requests **never follow redirects**: a `301` to a login page is a `301`, not the login page.
- The home page (`/`) is fetched once (following redirects) and shared by every check that requests `/`. Header, cookie and TLS checks use it.
- Responses are cached per scan, so two checks asking for the same path cost one request.
- Matchers see at most the first 1 MB of a body.
- A check that `requires` something the fingerprint didn't find is skipped and doesn't count toward the scan's check total.

## Matchers

Each item in `all` or `any` is one matcher. **Every field set on it must hold**. Put conditions in separate items when you want them read separately; combine fields in one item when they belong together.

| Field | Holds when | Example |
|---|---|---|
| `status` | the status code is one of the given values (an int or a list) | `status: 200`, `status: [200, 206]` |
| `regex` | the body matches ([RE2 syntax](https://github.com/google/re2/wiki/Syntax)) | `regex: '(?m)^DB_PASSWORD='` |
| `not_regex` | the body doesn't match | `not_regex: '(?i)<html'` |
| `words` | the body contains any of the strings | `words: ["ref: refs/", "[core]"]` |
| `magic` | the body starts with these bytes, in hex | `magic: "504b0304"` (zip) |
| `header` | `{Name: regex}`: a value of that header matches | `header: {Content-Type: 'text/plain'}` |
| `not_header` | `{Name: regex}`: no value of that header matches | `not_header: {Content-Type: 'html'}` |
| `missing_header` | none of the listed headers is present | `missing_header: [Strict-Transport-Security]` |
| `scheme` | the final URL is `http` or `https` | `scheme: https` |
| `cookie` | `{name: regex, missing_flag: secure\|httponly\|samesite}`: a cookie whose name matches lacks the flag. An empty `name` means any cookie. Only the cookie's name is reported, never its value. | `cookie: {name: 'PHPSESSID', missing_flag: httponly}` |
| `tls` | `{expired, expires_within_days: N, untrusted, legacy}`: certificate and protocol state, for `/` on https targets. Set at least one; every one set must hold. | `tls: {expires_within_days: 14}` |
| `not_baseline` | the response isn't the server's soft-404 page (see below) | `not_baseline: true` |

`all` and `any` combine like this: every matcher in `all` must hold. If `any` is present, at least one of its matchers must too. Baseline comparisons run last, so a soft-404 probe is only requested when everything else already matched.

> [!IMPORTANT]
> Use `not_baseline: true` on every check for a file path. Many servers answer `200` for any URL (a "soft 404"). The baseline teaches Tuner what that looks like on each site, so a catch-all page never becomes a finding. `checks lint` requires it, or a `regex`, `words` or `magic` matcher, on every path but `/`.

How `not_baseline` decides: with a `regex`, `words` or `magic` condition, the hit is dropped only if the soft-404 page also matches them or is the same page apart from the path it echoes; otherwise the page must look like the soft-404 page (same status, size within 5%). So a real 78-byte `.env` next to an 80-byte "Not found" page is still reported.

The finding's evidence shows what matched: the matched text (shortened to 120 characters), the header, the cookie name, or the TLS state. Evidence is always passed through secret redaction.

### Secrets

`extract.secrets: true` scans the body for credentials and adds them to the finding, masked: at most four leading characters, then `•••••`. The finding's title names them too (`.env file exposed: 1 secret (DB_PASSWORD=Su•••••)`). Recognition comes from a catalog of about 100 rules: cloud, payment, mail and messaging, CI and AI keys, private keys, JWTs, passwords in URLs, `wp-config.php` defines and `.env`-style names. `rhythm tuner checks --tags secrets` lists them. Placeholders such as `null`, `true` or `${VAR}` are ignored.

Masking covers what the catalog recognizes. A value it doesn't recognize can still reach the evidence. So match on the structure around a secret (the key and the `=` or `:`), never on the value. Keep evidence to names where you can.

## Profiles

| Profile | Use for |
|---|---|
| `quick` | the highest-value checks that cost few requests. `checks lint` fails when the quick profile asks for more than 30 distinct paths in total, so add to it sparingly. |
| `default` | most checks: everything polite and non-intrusive. |

Profiles are cumulative: `default` runs `quick` checks too. `--id` ignores profiles. Preview a selection with `rhythm tuner checks --profile quick`. (`deep` is still accepted in a check file; no built-in check uses it.)

## Fixtures

Every check needs `<name>_test.yaml` next to it, with **at least one positive and one negative case**. The test suite starts a mock web server for each case, runs the check against it and fails if a positive case produces no finding or a negative case produces one.

```yaml
positive:
  - name: env readable
    expect_secrets: true        # assert masked secrets are extracted
    routes:
      /.env: {body: "DB_PASSWORD=hunter22\n"}
negative:
  - name: blocked
    routes:
      /.env: {status: 403}
  - name: soft-404 catch-all
    default: {status: 200, body: "<html>{{path}} not found</html>"}
```

### Case fields

| Field | Description |
|---|---|
| `name` | Describes the case; shown when it fails. |
| `routes` | Map of path to reply. Paths without a route return `404`, or `default` if set. `routes: {}` serves nothing. A route may carry a query string (`/?rest_route=/wp/v2/users:`); it answers that exact path and query. |
| `default` | Reply for every path without a route. `{{path}}` in its body is replaced with the requested path, which is how you build realistic soft-404 pages. |
| `expect_secrets` | Positive cases only: also assert that masked secrets were extracted. |
| `tls` | `true` to serve over HTTPS with a trusted certificate. |
| `tls_min` | `"1.0"` to serve HTTPS that still accepts TLS 1.0 and 1.1 (for `tls: {legacy: true}`). `"1.0"` is the only value. |
| `cert` | A custom certificate, implying HTTPS: `{expires_in_days: N, untrusted: true}`. Negative days give an expired certificate; `untrusted` leaves it out of the trust store. |

### Reply fields

| Field | Default | Description |
|---|---|---|
| `status` | `200` | HTTP status code |
| `headers` | none | Map of header name to value, e.g. `{Content-Type: text/plain}` |
| `body` | empty | Response body |
| `body_hex` | none | Body bytes in hex, sent before `body`; for binary files such as archives |

Unknown fields fail the test too. For every positive case the runner also renders the finding as plain text, JSON, SARIF and Markdown. It fails the check when any of them shows a raw secret from the fixture (any value Tuner's masker hides; in a secrets fixture, also any `KEY=value` or JSON string value of 6 characters or more) or a control byte such as ESC. Fixture sites only listen on 127.0.0.1 and can't redirect the scan anywhere else.

**Good negatives** cover the ways a check could misfire: the file blocked (`403`), a soft-404 catch-all (`default` with `{{path}}`), a page that looks similar (an HTML download page named `backup.zip`) and a configuration that's already safe.

```yaml
# TLS fixtures from checks/tls/cert-expiring_test.yaml
positive:
  - name: expires in 5 days
    cert: {expires_in_days: 5}
    routes:
      /: {body: home}
negative:
  - name: valid for 60 days
    cert: {expires_in_days: 60}
    routes:
      /: {body: home}
```

## Testing locally

`rhythm tuner checks` has everything a check author needs, with no Go toolchain:

| Command | |
|---|---|
| `rhythm tuner checks new <id> [dir]` | write `<id>.yaml` and `<id>_test.yaml` to fill in; both pass test and lint as written |
| `rhythm tuner checks test [dir]` | run every check in `dir` (default: the built-in ones) against its fixtures on 127.0.0.1 |
| `rhythm tuner checks lint [dir]` | check them against the rules below |
| `rhythm tuner checks --checks <dir> --id <id>` | confirm a check loads and is selected |
| `rhythm tuner scan <site> --checks <dir> --id <id> -v` | try it on a site you own |

```text
$ rhythm tuner checks test
ok  hdr-content-type-options-missing  1 positive, 1 negative
ok  hdr-cookie-no-httponly            1 positive, 2 negative
ok  hdr-cookie-no-secure              1 positive, 2 negative
…
All 77 checks in built-in checks passed (287 cases).
$ rhythm tuner checks lint
No issues in built-in checks.
```

`--checks <dir>` loads every `.yaml` file under the directory (fixtures excluded) and never uses a check pack, so you always run what you're editing. `-v` logs each request to stderr.

In the tuner repository, `go test ./engine/...` runs the same fixtures and lint rules; `make check` runs the whole suite as CI does.

`checks lint` reports one issue per line (`<file>: <rule>: <message>`) and exits 1. A check that doesn't load at all is reported as a load error; a scan with `--checks` refuses it the same way:

```text
checks: load: leak-wp-cli-config.yaml: request.method "POST": checks may only use GET or HEAD

1 issue in bad.
```

## Rules

- **Detect, never exploit.** Requests are plain GETs (or HEADs) of public URLs, with no payloads, no form submissions, no login attempts. Tuner enforces the method and the query rules when it loads a check. A check confirms that something is exposed; it doesn't use it.
- **Never print a secret unmasked.** Use `extract.secrets`; evidence is redacted automatically. Match on the structure around a secret, not on the secret.
- **Every check has fixtures**, with positives and negatives that cover the obvious false positives.
- **Be polite.** Keep paths to the few that matter and keep `quick` small.
- **Removing a check** means deleting its YAML and adding its ID to `retired` in `checks/pack.json`, so packs and older binaries agree on what's gone.

## Worked example: adding a check

WP-CLI reads `wp-cli.yml` from the folder it runs in. Some sites keep it in the web root. It names the site's path on disk and often SSH aliases with the deploy user and host. Here's how to add a check for it.

**1. Scaffold it:**

```text
$ rhythm tuner checks new leak-wp-cli-config my-checks
Wrote my-checks/leak-wp-cli-config.yaml and my-checks/leak-wp-cli-config_test.yaml.
Replace the TODOs, then run:
  rhythm tuner checks test my-checks
  rhythm tuner checks lint my-checks
```

**2. Write the check** in `my-checks/leak-wp-cli-config.yaml`:

```yaml
id: leak-wp-cli-config
name: WP-CLI config exposed
severity: medium
profile: default
tags: [leak, wordpress]
request:
  path: ["/wp-cli.yml", "/wp-cli.local.yml"]
match:
  all:
    - status: 200
    # Keys only: never match on the values after the colon.
    - regex: '(?m)^(?:path|url|ssh|@[A-Za-z0-9_-]+)\s*:'
    - not_regex: '(?i)<(?:!doctype|html|head|body)[\s>]'
    - not_baseline: true
impact: WP-CLI config names the site's path on disk and often SSH aliases with the deploy user and host.
fix: |
  Delete wp-cli.yml from the web root, or keep it one folder above it.
fix_server:
  nginx: |
    location ~ /wp-cli(\.local)?\.yml$ { deny all; }
  apache: |
    RedirectMatch 404 /wp-cli(\.local)?\.yml$
references:
  - https://make.wordpress.org/cli/handbook/references/config/
```

Choices worth noting:

- The regex matches the keys, never the values, so the evidence can't contain a host or user.
- `not_regex` keeps an HTML page that happens to mention the keys quiet; `not_baseline` does the same for catch-all servers.
- `default` profile: two requests is too many for `quick` given how rare the file is.

**3. Write the fixtures** in `my-checks/leak-wp-cli-config_test.yaml`:

```yaml
positive:
  - name: config with an ssh alias
    routes:
      /wp-cli.yml: {body: "path: /var/www/shop\n@staging:\n  ssh: deploy@203.0.113.10/var/www/shop\n"}
  - name: local config
    routes:
      /wp-cli.local.yml: {body: "url: https://shop.test\n"}
negative:
  - name: blocked
    routes:
      /wp-cli.yml: {status: 403, body: Forbidden}
  - name: soft-404 catch-all
    default: {status: 200, body: "<html><title>Not found</title><p>{{path}} does not exist</p></html>"}
  - name: html page that mentions the keys
    routes:
      /wp-cli.yml: {body: "<html><body>path: is the first key of wp-cli.yml</body></html>"}
```

**4. Test and lint it:**

```text
$ rhythm tuner checks test my-checks
ok  leak-wp-cli-config  2 positive, 3 negative

The check in my-checks passed (5 cases).
$ rhythm tuner checks lint my-checks
No issues in my-checks.
```

**5. Check it's selected as expected:**

```text
$ rhythm tuner checks --checks my-checks --id leak-wp-cli-config
ID                  SEVERITY  PROFILE  TAGS            NAME
leak-wp-cli-config  medium    default  leak,wordpress  WP-CLI config exposed

Using checks from my-checks.
1 check, plus vulnerability matching for core, plugins, themes, PHP and
nginx/Apache (vuln-*, wp-components-*, *-eol), which scans skip with --no-vulns.
```

**6. Try it on a site you own**, ideally a staging copy where you've placed a harmless test file. Here a local test server holds one:

```text
$ rhythm tuner scan http://127.0.0.1:20404 --checks my-checks --id leak-wp-cli-config --plain
…
[MEDIUM] WP-CLI config exposed - http://127.0.0.1:20404/wp-cli.yml
  evidence: path:
  fix: Delete wp-cli.yml from the web root, or keep it one folder above it.
…
Score 94/100 - 1 medium - 1 check, 5 requests, 100ms
```

**7. Open a pull request** with both files in `checks/leaks/`, with the copyright header the repository's other checks carry. CI runs every fixture and the lint rules on Linux, macOS and Windows. See [Contributing](Contributing) for commit and review conventions.

> [!TIP]
> If your check needs a matcher that doesn't exist yet, that's a Go change in [`matchers/matchers.go`](https://github.com/RhythmShahriar/tuner/blob/master/matchers/matchers.go). Older binaries refuse checks with fields they don't know. See [min_tuner](Tuner-Check-Packs#compatibility-min_tuner-and-schema) for packs.

## Sharing checks with Nuclei users

`rhythm tuner checks export --nuclei <dir>` writes the checks as Nuclei templates, `tuner-<id>.yaml`, with a README that explains the differences and the MIT license they ship under:

```text
$ rhythm tuner checks export --nuclei nuclei
Wrote 69 Nuclei templates and README.md to nuclei.
Skipped 8 checks a template can't express:
  hdr-cookie-no-httponly  cookie flag conditions have no Nuclei matcher
  hdr-cookie-no-secure    the scheme condition (https only) has no Nuclei matcher
  hdr-hsts-missing        the scheme condition (https only) has no Nuclei matcher
  tls-cert-expired        TLS conditions need a TLS probe, which a Nuclei http template has not
  tls-cert-expiring       TLS conditions need a TLS probe, which a Nuclei http template has not
  tls-cert-untrusted      TLS conditions need a TLS probe, which a Nuclei http template has not
  tls-legacy-protocol     TLS conditions need a TLS probe, which a Nuclei http template has not
  tls-no-https            the scheme condition (http only) has no Nuclei matcher
Run them with: nuclei -t nuclei -u <url>
```

The templates approximate Tuner's soft-404 comparison with a "not found" title check, don't enforce `requires` and leave secret masking to you, since Nuclei can store full responses. `--id`, `--tags` and `--checks` choose what to export.

---
<sub>← [Watch and serve](Tuner-Watch-And-Serve) · [Home](Home) · [Check packs](Tuner-Check-Packs) →</sub>
