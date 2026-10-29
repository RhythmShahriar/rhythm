`verify` reads a staging copy and looks for what shouldn't be in it: real-looking people, live keys, production URLs, side effects that are still on and personal files. Every `copy` and `prepare` ends with one; you can run it on any staging, dump or database, made by Roadie or not. Each run writes a signed report anyone can check. This page covers the two modes, every check and finding, how to read a result, the report formats and the signature.

## Two modes

| Mode | When | What it can say |
|---|---|---|
| **exact** | `copy` and `prepare` (always), or `verify --from <production>` | Roadie holds a keyed hash of every structured personal value in production and checks that none survives anywhere in the copy: raw, lower case, JSON-escaped, URL-encoded, HTML entity, base64, md5 and sha256 forms, in free text too. The hashes are dropped when the run ends. |
| **heuristic** | a standalone `verify` | Roadie can't know which values were real, so it looks for anything that looks real. Roadie's own fakes are never flagged. |

The mode is on the second line of every verify:

```text
  ▸ Mode          exact: every value Roadie replaced was checked
  ▸ Mode          heuristic: this copy wasn't made by Roadie in this run
```

```sh
rhythm roadie verify /var/www/staging                     # a site: database, wp-config.php, .env and its files
rhythm roadie verify staging.sql --from shop.sql          # exact, against production's dump
rhythm roadie verify mysql://ci@127.0.0.1/staging --production shop.com
rhythm roadie verify /var/www/staging --site https://staging.shop.test --echo
```

## What it reads

Every text column of every table (not only the ones a rule covers) and every string inside serialized, JSON and base64-serialized values; the target's `wp-config.php` and `.env`. On a site target it also lists, by name and size only, `wp-content/debug.log`, `wp-content/uploads/wc-logs/`, `wp-content/uploads/gravity_forms/`, `wp-content/updraft/`, `wp-content/ai1wm-backups/` and `.sql`, `.sql.gz` or `.zip` files in the web root or `wp-content/`; it never opens an archive. With `--site <url>` it fetches the staging's `/robots.txt` and home page headers (never a production host; redirects only within that host; 2 MiB, 10 seconds). With `--echo[=<url>]` it asks Echo whether its seal is on and how many messages it let through since the copy. Nothing else.

## The checks

Every verify runs 30 checks and prints each with its result: `✓` passed (with the number of values it read), `✗` failed, `!` found something below `--fail-on` (a warning) and `·` didn't apply, with the reason. A check that fails or warns is named without its "No":

| Check | ID |
|---|---|
| Asks for a password | `auth.none` |
| The site says it is staging | `config.environment` |
| Nothing points at production | `config.points-at-production` |
| Salts and APP_KEY are the copy's own | `config.salts` |
| Encrypted values listed | `data.encrypted` |
| No pseudonymised value survives | `exact.survivors` |
| No secret-looking settings | `key.generic` |
| No live keys | `key.live` (the `key.<provider>-live` findings) |
| No gateway in live mode | `key.live-mode` |
| No provider key of unknown mode | `key.unknown-env` |
| No real-looking addresses | `pii.address` |
| No card numbers | `pii.card` |
| No real-looking emails | `pii.email` |
| No IBANs | `pii.iban` |
| No public IP addresses | `pii.ip` |
| No real-looking names | `pii.name` |
| No real-looking phones | `pii.phone` |
| Kept out of search engines | `search.indexable` |
| Off: Action Scheduler | `side-effect.action-scheduler` |
| Off: AutomateWoo | `side-effect.automatewoo` |
| Off: WP-Cron | `side-effect.cron` |
| Off: Gateways | `side-effect.gateways` |
| Off: Jetpack | `side-effect.jetpack` |
| Off: Echo connector, sealed | `side-effect.mail` |
| Off: Subscriptions | `side-effect.subscriptions` |
| Off: WooCommerce webhooks | `side-effect.webhooks` |
| Every personal-looking column covered | `uncovered-column` |
| No personal files in uploads | `uploads.personal` |
| No production URLs | `url.production` |
| Site URL is the copy's | `url.production-siteurl` |

A Laravel target adds the queue check (`side-effect.queue`) and a database-only target `Database only: no files to check` (`side-effect.not-applicable`). `--skip` leaves checks out: `emails`, `phones`, `names`, `addresses`, `ips`, `cards`, `keys`, `urls`, `side-effects`, `config`, `uploads`. A skipped check shows `left out with --skip emails`.

## Findings

Each finding has an ID, a severity and, on screen, a fix. `rhythm roadie explain <id>` gives the whole explanation offline. SARIF results link to the entries below.

| ID | Severity | What it flags | Never flagged |
|---|---|---|---|
| <a id="exact.survivors"></a>`exact.survivors` | critical | (exact mode) a production value Roadie replaced, found anywhere in the copy; the detail names the encoding (`found as raw`) | kept users' values; values under 4 bytes; values that are also Roadie fakes |
| <a id="pii.email"></a>`pii.email` | high | an email with a domain that isn't reserved, also in HTML-entity, URL-encoded and JSON forms or as the key of a serialized or JSON value | reserved domains (`example.com`, `.invalid`, `.test`, `.localhost`, `.local`, `*.example`); `allow_emails`; kept users; `wordpress@<staging host>`; file names such as `@2x.png`; `git@host:` |
| <a id="pii.email-role"></a>`pii.email-role` | info | a role address: `noreply`, `no-reply`, `donotreply`, `mailer-daemon`, `postmaster`, `wordpress` | |
| <a id="pii.phone"></a>`pii.phone` | high | in phone columns, any number of 7 to 15 digits; in free text only strong forms (`+` and country code, `tel:`, national formats with separators) | fictional ranges (`+1 555 01xx`, UK `07700 900xxx`, Ofcom's drama ranges, the German ones); SKU, EAN, tracking, price, ID and postcode columns; numbers next to a currency sign; timestamps; versions |
| <a id="pii.name"></a>`pii.name` | medium | in name columns, a name that isn't from Roadie's lists | rows whose email is reserved or a Roadie fake; one-letter values; kept users |
| <a id="pii.address"></a>`pii.address` | medium | in street columns, a street that isn't from Roadie's lists | as for names |
| <a id="pii.ip"></a>`pii.ip` | medium | a public IPv4 or IPv6 in an IP column or an `ip` key inside serialized or JSON values (session tokens) | loopback, private, link-local, CGNAT and documentation ranges; dotted numbers in prose |
| <a id="pii.card"></a>`pii.card` | high | 13 to 19 digits, Luhn-valid, that start like a card and are grouped like one, sit in a card column or follow a card word | published test cards (`4242 4242 4242 4242` and the rest); order numbers |
| <a id="pii.iban"></a>`pii.iban` | medium | a valid IBAN | the documented example IBANs |
| <a id="data.unparseable"></a>`data.unparseable` | medium | a serialized-looking value that doesn't parse, or an SQL expression, that held personal-looking text (blanked, or the copy failed) | text that only looks serialized in prose |
| <a id="data.encrypted"></a>`data.encrypted` | info | Laravel encrypted casts and encrypted job payloads nobody can read | |
| <a id="uncovered-column"></a>`uncovered-column` | medium | a column that looks personal with no rule (see [uncovered columns](Roadie-Copy-And-Prepare#uncovered-columns)) | dropped and truncated tables |
| <a id="key.live"></a>`key.<provider>-live` | critical or high | a live key; [the list](Roadie-Live-Keys#what-roadie-recognises) | test keys; placeholders; masked values; a disabled gateway's keys (low) |
| <a id="key.live-mode"></a>`key.live-mode` | high | a gateway in live mode, even with blank keys | disabled gateways |
| <a id="key.generic"></a>`key.generic` | high | a secret-looking setting with a real value | `keys.ignore`; known non-secrets |
| <a id="key.unknown-env"></a>`key.unknown-env` | medium | a provider key Roadie can't call live or test | |
| <a id="url.production"></a>`url.production` | medium | production's host in any form | lookalike hosts; email domains; `wc_subscriptions_siteurl`; links to other sites |
| <a id="url.production-siteurl"></a>`url.production-siteurl` | critical | `siteurl`, `home`, `WP_HOME`, `WP_SITEURL` or `APP_URL` still production's | |
| <a id="config.environment"></a>`config.environment` | critical | `WP_ENVIRONMENT_TYPE` not `staging`, `local` or `development` (or defined after `wp-settings.php`); `APP_ENV=production` | |
| <a id="config.points-at-production"></a>`config.points-at-production` | critical | `DB_HOST`, `WP_REDIS_HOST`, cache salts or S3 offload settings equal to production's or matching `production:` | |
| <a id="config.salts"></a>`config.salts` | medium | salts or `APP_KEY` equal to production's (exact mode), or `--keep-app-key` | |
| <a id="side-effect.mail"></a>`side-effect.mail` | critical | no Echo connector and nothing blocking `wp_mail()`; a mailer with live credentials; with `--echo`, the seal off, mail let through, or Echo unreachable | Laravel `MAIL_HOST` on loopback or Echo |
| <a id="side-effect.connector-changed"></a>`side-effect.connector-changed` | high | the installed Echo connector differs from the one Roadie ships | |
| <a id="side-effect.cron"></a>`side-effect.cron` | high | `DISABLE_WP_CRON` isn't true | |
| <a id="side-effect.action-scheduler"></a>`side-effect.action-scheduler` | high | the runner isn't paused and actions are pending | nothing pending |
| <a id="side-effect.webhooks"></a>`side-effect.webhooks` | high | an active WooCommerce webhook | |
| <a id="side-effect.gateways"></a>`side-effect.gateways` | critical | an enabled gateway not in test mode | WooPayments in dev mode |
| <a id="side-effect.subscriptions"></a>`side-effect.subscriptions` | critical | Subscriptions without forced staging mode, or `wc_subscriptions_siteurl` naming this site | Subscriptions absent |
| <a id="side-effect.queue"></a>`side-effect.queue` | high | Laravel's `QUEUE_CONNECTION` not `sync`, `null` or `log` with rows in `jobs` | |
| <a id="side-effect.jetpack"></a>`side-effect.jetpack` | medium | Jetpack connected and not offline | Jetpack absent |
| <a id="side-effect.automatewoo"></a>`side-effect.automatewoo` | high | rows in `automatewoo_queue` | |
| <a id="side-effect.not-applicable"></a>`side-effect.not-applicable` | info | a database-only target: the file switches can't be checked | |
| <a id="search.indexable"></a>`search.indexable` | medium | no noindex mu-plugin with `blog_public = 1`, a physical `robots.txt` that allows crawling, or (with `--site`) no noindex header | |
| <a id="auth.none"></a>`auth.none` | low | (with `--site`) the staging answers without a password | |
| <a id="uploads.personal"></a>`uploads.personal` | critical (backups), high (logs), medium (form uploads) | the files listed under [What it reads](#what-it-reads) | empty files |
| <a id="soundcheck.failed"></a>`soundcheck.failed` | medium | `copy --soundcheck` ran Soundcheck and it exited 1 | Soundcheck not installed (a note, not a finding) |
| <a id="info.kept-users"></a>`info.kept-users` | info | users kept as they are, counted | |
| <a id="info.ignore-expired"></a>`info.ignore-expired` | info | an ignore entry past its `until` date | |

The live key IDs are `key.stripe-live`, `key.stripe-publishable-live`, `key.stripe-webhook-live`, `key.paypal-live`, `key.braintree-live`, `key.square-live`, `key.woopayments-live`, `key.mollie-live`, `key.razorpay-live`, `key.adyen-live`, `key.klarna-live`, `key.authorize-net-live`, `key.flutterwave-live`, `key.woocommerce-rest-live`, `key.sendgrid-live`, `key.mailgun-live`, `key.postmark-live`, `key.ses-live`, `key.brevo-live`, `key.mailchimp-live`, `key.mandrill-live`, `key.resend-live`, `key.sparkpost-live`, `key.mailersend-live`, `key.smtp-live`, `key.twilio-live`, `key.jetpack-live` and `key.aws-live`.

**Masking.** No finding shows a value: an email shows two characters of its local part and domain (`jo•••@gm•••.com`), a phone its country code (`+44 •••`), a name its initial (`J•••`), an IP its first octet, a key its first four characters (`sk_l•••••`). `-v` adds row keys (primary keys, `option_name`, `meta_key`), never values. Findings of one ID in one column are one finding with a count and at most three examples.

## Reading a failing verify

`verify shop.sql` on the production dump itself, as the demo does:

```text
  ♪ rhythm roadie 1.0.0                                      verify · shop.sql
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          heuristic: this copy wasn't made by Roadie in this run
  ▸ Database      shop.sql, 13 tables of 13, every row
  ▸ Production    shop.example.com

  CHECKS  30 ─────────────────────────────────────────────────────────────────

    · Asks for a password                     --site not given
    · The site says it is staging             a database or dump target has
                                              no files to read
    · Nothing points at production            a database or dump target has
                                              no files to read
    · Salts and APP_KEY are the copy's own    a database or dump target has
                                              no files to read
    ✓ Encrypted values listed                 1,536 checked
    · No pseudonymised value survives         heuristic mode: no record of
                                              what Roadie replaced
    ✓ No secret-looking settings              1,536 checked
    ✗ Live keys                               3 findings (key.live)
    ✗ Gateways in live mode                   1 finding (key.live-mode)
    ✓ No provider key of unknown mode         1,536 checked
    ! Real-looking addresses                  154 findings below --fail-on
                                              high (pii.address)
    ✓ No card numbers                         1,536 checked
    ✗ Real-looking emails                     280 findings (pii.email)
    ✓ No IBANs                                1,536 checked
    ! Public IP addresses                     100 findings below --fail-on
                                              high (pii.ip)
    ! Real-looking names                      335 findings below --fail-on
                                              high (pii.name)
    ✗ Real-looking phones                     181 findings (pii.phone)
...
    ✗ WooCommerce webhooks                    1 finding
                                              (side-effect.webhooks)
    ✓ Every personal-looking column covered   13 checked
    · No personal files in uploads            a database or dump target has
                                              no files to read
    ! Production URLs                         77 findings below --fail-on
                                              high (url.production)
    ✗ Site URL                                2 findings
                                              (url.production-siteurl)

  CRITICAL  2 ────────────────────────────────────────────────────────────────

 ▌  CRITICAL   A live Stripe key                              key.stripe-live
 ▌             wp_options.option_value → serialized:secret_key 1
 ▌             e.g. sk_l•••••
 ▌  CRITICAL   2 The site URL is still production's    url.production-siteurl
 ▌             wp_options.option_value 2
 ▌             → Copy again with --url, or set siteurl and home to the
 ▌               staging URL.

  HIGH  16 ───────────────────────────────────────────────────────────────────

 ▌  HIGH       Stripe is in live mode                           key.live-mode
 ▌             wp_options.option_value → serialized:testmode 1
 ▌             → Switch the gateway to test or sandbox mode (rhythm-roadie
 ▌               prepare --only side-effects).
...
 ▌  HIGH       40 Real-looking email addresses                      pii.email
 ▌             wp_users.user_email 40
 ▌             e.g. il•••@ho•••.uk, qu•••@gm•••.com, qu•••@ic•••.com
 ▌             → Cover the column with a rule, or list the address in
 ▌               allow_emails if it may stay real.
...
  MEDIUM  14 ─────────────────────────────────────────────────────────────────

    · 60 Real-looking street addresses wp_usermeta billing_address_1 10 1 ·
      wp_usermeta shipping_address_1 17 1 · wp_usermeta billing_address_1 32
      1 (pii.address)
...
    · 71 Production URLs wp_posts.guid 71 (url.production)

 ╭─────────────────────────────────────────────────────╮
 │  FAIL   shop.sql is not a safe copy yet             │
 │  2 critical · 16 high · 14 medium · 0 low · 0 info  │
 ╰─────────────────────────────────────────────────────╯
  166 KB · 13 tables · 1,536 rows · 0.1s
...
  → rhythm-roadie explain key.stripe-live
  → signed report: /home/ann/shop/.roadie/reports/2026-10-28T18-56-50Z-verify-shop.sql/report.html
```

Findings come by severity, critical first; each `▌` block names the finding, its location (table, column, the path inside a serialized or JSON value), a count, masked examples and the fix. Medium and below are one line each. The exit code is 1: findings at or above `--fail-on high`.

## The verdict

| Box | Means | Exit |
|---|---|---|
| `PASS   <name> is a pseudonymised copy` | no finding at or above `--fail-on`, at least one table and row read, every input read whole | 0 |
| `FAIL   <name> is not a safe copy yet` | findings at or above `--fail-on` | 1 |
| `PASS (sample)   <name> is a pseudonymised copy` | `--sample <n>` read n rows per table and found nothing; the report has `complete: false`, so CI (and the Action, unless `allow-sample: "true"`) counts it as not passed | 0 |
| `INCOMPLETE   <name> couldn't be read whole; coverage lists what wasn't` | a read error partway through; never passes | 3 |
| `PASS   <name>: 1,134 findings hidden by --baseline or ignore entries` | everything found was accepted earlier | 0 |
| `STOPPED   nothing-scanned` | no table or no row read: an empty dump, a wrong prefix, no SELECT rights | 3 |

`--fail-on` takes `none`, `info`, `low`, `medium`, `high` (default) or `critical`; `verify.fail_on` sets it in the config. Findings below it still show, as `!` warnings.

**Accepting test data.** `--baseline <report.json>` hides every finding whose ID and location are already in that report and says how many it hid. For one value, an entry under `verify.ignore` with `id`, `location`, `reason` and an optional `until` date. `--allow-emails '*@agency.example'` and `--allow-phones` keep business contacts out of the findings. Kept users are never findings.

## Reports

Every `copy`, `prepare` and `verify` writes `.roadie/reports/<UTC time>Z-<command>-<target>/report.json` (signed) and `report.html`, readable by you only. They hold no personal value (findings are masked) but they do name production's host, tables, columns and counts: treat them as internal. The same result renders as:

| Flag | Format | For |
|---|---|---|
| `--json` | the signed envelope, schema `rhythm-roadie/v1` | scripts, CI |
| `--md` | Markdown | pull request comments, job summaries |
| `--html` | one file, no scripts, no external assets | people, clients |
| `--sarif` | SARIF 2.1.0 (verify) | GitHub code scanning |
| `--junit` | JUnit XML, suite `roadie.verify`, one case per check (verify) | CI test views |

One of them goes to stdout; `-o <file>` writes any of them by extension (`.json`, `.md`, `.html`, `.sarif`, `.xml`). It is repeatable and keeps the live view: `rhythm roadie verify shop.sql -o roadie.sarif -o roadie.xml -o roadie.md`. A format flag with an `-o` file of the same format writes only the file: `verify shop.sql --md -o r.md -q` leaves stdout to the result box. Two on stdout is a usage error:

```text
rhythm-roadie verify: pick one of --json, --md, --html, --sarif and --junit for stdout; write the others with -o
```

The Markdown starts like this:

```markdown
## Roadie: FAIL, shop.sql is not a safe copy yet

| | |
|---|---|
| Target | `shop.sql` |
| Production | shop.example.com |
| Stack | wordpress \(action-scheduler, woocommerce, woocommerce-hpos, wordpress\) |
| Verify | heuristic, every row of 13 tables |
| Fail on | high |
| Findings | 2 critical · 16 high · 14 medium · 0 low · 0 info |
| Roadie | `1.0.0`, profile pack `1.0.0` |

| Check | Result |
|---|---|
| Asks for a password | ➖ \--site not given |
...
| Live keys | ❌ 3 hits |
| Gateways in live mode | ❌ 1 hit in 1 column |
...
| Real-looking addresses | ⚠️ 154 hits in 2 columns |
```

A SARIF result for a database location names the dump (or `db://<target>/<table>`) and the column as a logical location; each rule links to this page:

```json
{
 "ruleId": "key.stripe-live",
 "level": "error",
 "message": {
  "text": "A live Stripe key: wp_options 10 woocommerce_stripe_settings → serialized:secret_key (e.g. sk_l•••••)"
 },
 "locations": [
  {
   "physicalLocation": {
    "artifactLocation": {
     "uri": "shop.sql"
    }
   },
   "logicalLocations": [
    {
     "name": "option_value",
     "fullyQualifiedName": "wp_options.option_value",
     "kind": "member"
    }
   ]
  }
 ],
...
```

JUnit has one test case per check, named with its label and ID (`No live keys (key.live)`); a failed case's message is its finding. A check that only warns is a passing case with the findings in `system-out`, so CI history follows the check.

Every report ends with what it doesn't cover:

```text
  Not covered by this report: files and uploads (PDF invoices, form uploads,
  images with location data), other databases, object caches (Redis,
  Memcached), search indexes (Algolia, ElasticPress), logs on disk outside
  wp-content, copies held by third parties (Mailchimp lists, Stripe
  customers), names and street addresses written in free text that Roadie
  didn't see in a structured column, encrypted columns, tables you told Roadie
  to keep.
```

### Rendering saved reports

`report` renders saved JSON reports again. Several make one summary page, worst first; `--brand-name` and `--brand-logo` (PNG, SVG or JPEG under 512 KiB) brand the HTML for a client:

```text
$ rhythm roadie report .roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.json .roadie/reports/2026-10-28T18-56-50Z-verify-shop.sql/report.json -o summary.md
Wrote summary.md (2 reports).

$ rhythm roadie report .roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.json -o client.html --brand-name Northwind
Wrote client.html (1 report).
```

### JSON

Every command's `--json` is one envelope:

```json
{
  "schema": "rhythm-roadie/v1",
  "kind": "verify",
  "tool": "rhythm-roadie",
  "version": "1.0.0",
  "complete": true,
  "summary": {
    "critical": 2,
    "high": 16,
    "medium": 14,
    "low": 0,
    "info": 0,
    "tables": 13,
    "tables_pseudonymised": 0,
    "rows": 1536,
    "values_pseudonymised": 0,
    "urls_rewritten": 0,
    "keys_found": 3,
    "keys_swapped": 0,
    "keys_blanked": 0,
    "side_effects_off": 0,
    "side_effects_total": 2,
    "uncovered_columns": 0
  },
  "verdict": "findings",
  "data": { ... }
}
```

The fields and every `data` member are in the [CLI reference](Roadie-CLI-Reference#json-output).

## Signed reports

Each `report.json` is signed with Ed25519 over its canonical JSON (keys sorted, no insignificant whitespace, integers only). The signature covers the tool version, the ruleset and config hashes, the profile pack version, a fingerprint of the input (`sha256-file` for a dump or stdin, `db-tables` for a database) and every result. The key is, in order: `--key <file>`, `ROADIE_SIGNING_KEY` (base64 of a 32-byte seed; use it in CI so every run signs with one identity), or `~/.config/rhythm/roadie/signing.key`, created on the first signing run:

```text
Created a signing key for Roadie reports: roadie:xSmAjAE… (keep /home/ann/.config/rhythm/roadie/signing.key private; set ROADIE_SIGNING_KEY in CI to sign with one identity).
```

A key that can't be read stops the run before it scans anything (exit 2), never after a PASS box. `--key` means the private key that signs on `copy`, `prepare` and `verify`; on `report check` it is the public key or key ID to check against (`--key roadie:<id>`, as `report check` prints it).

`--sign=off` (or `verify.sign: false`) writes an unsigned report. The report holds the public key and its ID, never the signing key or the consistency key.

### report check

```sh
rhythm roadie report check .roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.json
```

```text
signed by roadie:xSmAjAE… and unchanged since. It shows this Roadie run found these results; it doesn't prove the checks covered everything.
Key ID roadie:xSmAjAEGqcTF5wui: pin it with --key roadie:xSmAjAEGqcTF5wui to accept only this signer.
```

`--input <dump>` also checks the report describes that dump (a copy's report describes its source; a verify's report what it read):

```text
$ rhythm roadie report check .roadie/reports/2026-10-28T18-57-00Z-verify-staging.sql/report.json --input staging.sql
signed by roadie:xSmAjAE… and unchanged since. It shows this Roadie run found these results; it doesn't prove the checks covered everything.
Key ID roadie:xSmAjAEGqcTF5wui: pin it with --key roadie:xSmAjAEGqcTF5wui to accept only this signer.
staging.sql is the input it describes (the sha256 matches).

$ rhythm roadie report check .roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.json --input staging.sql
rhythm-roadie report check: report.json describes another input (sha256 54870b528019…).
```

The failures:

```text
rhythm-roadie report check: edited.json was changed after it was signed (summary.high 16 → 0).
rhythm-roadie report check: report.json is signed by roadie:xSmAjAEGqcTF5wui, not by the key you pinned (roadie:AAAAAAAAAAAAAAAA).
rhythm-roadie report check: unsigned.json has no signature (it was written with --sign=off).
```

Exit 0 valid, 1 edited, another input or another key, 3 unsigned. A signature shows who ran Roadie, which version and rules, on which input and that the report is unchanged since. It doesn't prove the data holds no personal data. The HTML and Markdown footers say so:

```text
Pseudonymised data is still personal data under the GDPR (Article 4(5), Recital 26).
This report shows what Roadie's checks found in this copy; it is not a statement that
the copy holds no personal data. Signed by roadie:7Q3kXc2… (rhythm-roadie report check report.json):
the signature shows who ran Roadie, which version and rules, on which input and that
the report is unchanged since. It doesn't prove the data holds no personal data.
```

---
<sub>← [Kill switch](Roadie-Kill-Switch) · [Home](Home) · [Laravel](Roadie-Laravel) →</sub>
