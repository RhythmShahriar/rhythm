Tuner's remote checks are YAML files: one file per check, no Go code. This page covers the full check format, the fixtures every check needs, how to test locally, the rules every check follows and a worked example of adding a check from start to finish.

## Where checks live

Checks live in [`checks/<category>/`](https://github.com/RhythmShahriar/tuner/tree/master/checks) in the tuner repository, with a fixture file next to each one:

```text
checks/
  headers/  laravel/  leaks/  php/  tls/  wordpress/
  leaks/dotenv.yaml           # the check
  leaks/dotenv_test.yaml      # its fixtures
  pack.json                   # check pack settings (min_tuner)
  LICENSE
```

Built-in checks are compiled into the binary from `checks/*/*.yaml`, so a check sits exactly one folder deep. Once a check lands on `master`, it reaches users through the next [check pack](Tuner-Check-Packs), without a Tuner release.

## The check format

```yaml
id: leak-dotenv                 # unique, kebab-case
name: .env file exposed         # shown as the finding's title
severity: critical              # critical | high | medium | low | info
confidence: high                # optional: high (default) or low
profile: quick                  # optional: quick | default (default) | deep
tags: [leak, secrets]
requires: [wordpress]           # optional: only run when the fingerprint finds this
request:
  method: GET                   # optional, defaults to GET
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
references:
  - https://owasp.org/Top10/A05_2021-Security_Misconfiguration/
```

### Fields

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique across all checks, kebab-case, prefixed by area: `leak-`, `hdr-`, `php-`, `tls-`, `wp-`, `laravel-`. It becomes the finding ID. |
| `name` | yes | Short title for the finding. |
| `severity` | yes | `critical`, `high`, `medium`, `low` or `info`. Drives `--severity`, `--fail-on` and the [Tune score](Tuner-Reading-Results). |
| `confidence` | no | `high` (default) or `low`. Low-confidence findings show as *likely* and count half in the score. Use it when a match strongly suggests a problem but can't confirm it from outside. |
| `profile` | no | The cheapest [profile](#profiles) that runs the check: `quick`, `default` (the default) or `deep`. |
| `tags` | no | Used by `--tags` and `--exclude-tags`. Reuse existing tags where you can: `leak`, `secrets`, `headers`, `cookies`, `tls`, `php`, `wordpress`, `laravel`, `disclosure`… |
| `requires` | no | Technologies the [fingerprint](Tuner) must detect, any of `wordpress`, `laravel`, `php`. The check only runs when all are present. Leave it out for checks that make sense on any site: an exposed `.env` matters whatever the platform. |
| `request.method` | no | Defaults to `GET`. Keep it that way: checks only read. |
| `request.path` | yes | One or more paths, each starting with `/`. Each path is a separate request and can produce its own finding. `{{host}}` expands to the target's hostname, e.g. `/{{host}}.zip` becomes `/yoursite.com.zip`. |
| `match` | yes | `all` and/or `any` lists of [matchers](#matchers). At least one matcher in total. |
| `extract.secrets` | no | `true` to find credentials in the response body and report them masked. |
| `impact` | no | One or two sentences: what an attacker gains. |
| `fix` | no | How to fix it. The first line is shown in the terminal; put copy-pasteable config on the following lines. |
| `references` | no | URLs for further reading. |

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
| `not_baseline` | the response differs from the server's reply to a random path of the same shape | `not_baseline: true` |

`all` and `any` combine like this: every matcher in `all` must hold. If `any` is present, at least one of its matchers must too. Baseline comparisons run last, so a soft-404 probe is only requested when everything else already matched.

> [!IMPORTANT]
> Use `not_baseline: true` on every check for a file path. Many servers answer `200` for any URL (a "soft 404"). The baseline teaches Tuner what that looks like on each site, so a catch-all page never becomes a finding.

The finding's evidence shows what matched: the matched text (shortened to 120 characters), the header, the cookie name, or the TLS state. Evidence is always passed through secret redaction.

### Secrets

`extract.secrets: true` scans the body for credentials and adds them to the finding, masked: a few leading characters, then `•`. It recognizes AWS access keys, Stripe live keys, Google API keys, GitHub and Slack tokens, private keys, credentials in URLs, `wp-config.php` defines (`DB_PASSWORD`, keys and salts) and `.env`-style `*PASS*`, `*SECRET*`, `*_KEY*`, `*TOKEN*` and `*AUTH*` variables. Placeholders such as `null`, `true` or `${VAR}` are ignored.

Raw secret values never leave the matcher package: not in the terminal, not in any report format.

## Profiles

| Profile | Use for |
|---|---|
| `quick` | the highest-value checks that cost few requests. A `--profile quick` scan should stay around 30 requests in total, so add to it sparingly. |
| `default` | most checks: everything polite and non-intrusive. |
| `deep` | aggressive enumeration that sends many requests. |

Profiles are cumulative: `default` runs `quick` checks too; `deep` runs everything. `--id` ignores profiles. Preview a selection with `rhythm tuner checks --profile quick`.

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
| `routes` | Map of path to reply. Paths without a route return `404`, or `default` if set. `routes: {}` serves nothing. |
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

Unknown fields fail the test too. Every masked secret in every case is also checked to really be masked.

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

From the tuner repository (Go is required; see the repository's README for the version):

```sh
go test ./engine                                     # every check's fixtures
go test ./engine -run 'TestCheckFixtures/leak-dotenv' -v   # one check
make test                                            # the whole suite, as CI runs it
```

Then try the check against a real site you own, loading checks from your working tree instead of the built-in set:

```sh
rhythm tuner checks --checks ./checks --id leak-dotenv    # is it loaded and selected?
rhythm tuner scan yoursite.com --checks ./checks --id leak-dotenv -v
```

`--checks <dir>` loads every `.yaml` file under the directory (fixtures excluded) and never uses a check pack, so you always run what you're editing. `-v` logs each request to stderr. A binary built from a working tree with uncommitted changes also keeps its own built-in checks rather than a pack.

## Rules

- **Detect, never exploit.** Requests are plain GETs of public URLs, with no payloads, no form submissions, no login attempts. A check confirms that something is exposed; it doesn't use it.
- **Never print a secret unmasked.** Use `extract.secrets`; evidence is redacted automatically. Match on the structure around a secret, not on the secret.
- **Every check has fixtures**, with positives and negatives that cover the obvious false positives.
- **Be polite.** Keep paths to the few that matter. If a check needs many requests, it belongs in `deep`.

## Worked example: adding a check

Some editors keep deploy settings in the project, including the server's host, user and password. If those files are deployed, anyone can download them. Here's how to add a check for them.

**1. Write the check** as `checks/leaks/sftp-config.yaml`:

```yaml
id: leak-sftp-config
name: Editor SFTP config exposed
severity: high
profile: default
tags: [leak, credentials]
request:
  path: ["/sftp-config.json", "/.vscode/sftp.json"]
match:
  all:
    - status: 200
    - regex: '"(?:type|protocol)"\s*:\s*"(?:sftp|ftp|ftps)"'
    - words: ['"host"']
    - not_baseline: true
impact: Editor deploy settings usually hold the server's host name, user name and password or key path.
fix: |
  Delete the file from the web root and change the password it contains.
  Keep editor settings out of deploys, e.g. add them to .gitignore and your rsync excludes.
references:
  - https://owasp.org/Top10/A05_2021-Security_Misconfiguration/
```

Choices worth noting:

- The regex matches the protocol field, never the password, so the evidence can't contain the secret.
- `words` requires a `"host"` key too, so unrelated JSON doesn't match.
- `not_baseline` keeps catch-all servers quiet.
- `default` profile: it costs two requests, which is too many for `quick` given how rare the file is.

**2. Write the fixtures** as `checks/leaks/sftp-config_test.yaml`:

```yaml
positive:
  - name: sublime sftp config readable
    routes:
      /sftp-config.json:
        body: '{"type": "sftp", "host": "203.0.113.10", "user": "deploy", "password": "hunter22"}'
  - name: vscode sftp config readable
    routes:
      /.vscode/sftp.json:
        headers: {Content-Type: application/json}
        body: '{"name": "prod", "host": "203.0.113.10", "protocol": "sftp", "username": "deploy"}'
negative:
  - name: blocked
    routes:
      /sftp-config.json: {status: 403, body: Forbidden}
      /.vscode/sftp.json: {status: 403, body: Forbidden}
  - name: soft-404 catch-all
    default: {status: 200, body: "<html><p>{{path}} not found</p></html>"}
  - name: unrelated json
    routes:
      /sftp-config.json: {body: '{"name": "my-site", "version": "1.0.0"}'}
```

**3. Run the fixtures:**

```text
$ go test ./engine -run 'TestCheckFixtures/leak-sftp-config' -v
=== RUN   TestCheckFixtures
=== RUN   TestCheckFixtures/leak-sftp-config
--- PASS: TestCheckFixtures (0.01s)
    --- PASS: TestCheckFixtures/leak-sftp-config (0.00s)
PASS
ok  	github.com/RhythmShahriar/tuner/engine	0.010s
```

**4. Check it's selected as expected:**

```text
$ rhythm tuner checks --checks ./checks --id leak-sftp-config
ID                SEVERITY  PROFILE  TAGS              NAME
leak-sftp-config  high      default  leak,credentials  Editor SFTP config exposed

Using checks from ./checks.
```

**5. Try it on a site you own**, ideally a staging copy where you've placed a harmless test file:

```sh
rhythm tuner scan staging.yoursite.com --checks ./checks --id leak-sftp-config
```

**6. Open a pull request** with both files. CI runs the full suite. Once merged to `master`, the [checks-pack workflow](Tuner-Check-Packs#how-a-pack-is-published) runs every fixture again and publishes a new pack; scans pick it up within a day. See [Contributing](Contributing) for commit and review conventions.

> [!TIP]
> If your check needs a matcher that doesn't exist yet, that's a Go change in [`matchers/matchers.go`](https://github.com/RhythmShahriar/tuner/blob/master/matchers/matchers.go). Older binaries will skip packs that use it. See [min_tuner](Tuner-Check-Packs#compatibility-min_tuner-and-schema) for when to bump the pack's minimum version.

---
<sub>← [CI and code scanning](Tuner-CI) · [Home](Home) · [Check packs](Tuner-Check-Packs) →</sub>
