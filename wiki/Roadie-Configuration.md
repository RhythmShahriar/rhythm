Roadie reads its settings from `.roadie/config.yaml`, the nearest one in the folder you run it from or above. Every key is optional; flags win over the file and the file wins over the defaults. The file is meant to be committed: it never holds a password or a key, only `${ROADIE_*}` references to the environment. This page lists every key, the rules the file follows, the environment variables Roadie reads and the files it keeps.

```sh
rhythm roadie config init --stack woocommerce    # write the file, every key commented
rhythm roadie config check                       # validate it; list ${ROADIE_*} variables that aren't set
rhythm roadie config show                        # the effective settings, where each comes from, every rule
```

`config init` takes `--stack wordpress` (default), `woocommerce`, `laravel` or `sql`. `--from <source>` reads a source's schema and suggests `columns:` entries ([Generic SQL and Postgres](Roadie-Generic-SQL-And-Postgres#1-let-roadie-suggest-the-columns)). It refuses to replace a file without `--force`. Inside a git repository it adds `.roadie/reports`, `.roadie/key`, `.roadie/backups` and `.roadie/basic-auth` to `.git/info/exclude`.

## The file

This is `config init --stack woocommerce` for Roadie 1.0.0:

```yaml
# Roadie settings. Everything here is optional; delete what you don't change.
# Reference: https://github.com/RhythmShahriar/rhythm/wiki/Roadie-Configuration
# Commit this file to share it. Never put passwords here: ${ROADIE_*} reads the
# environment (only ROADIE_ names). Reports, keys and backups stay out of git.

source: ""                                   # ssh://deploy@shop.com/var/www/shop, prod.sql.gz, @prod, mysql://...
target: ""                                   # /var/www/staging, ssh://..., @staging, staging.sql.gz
url: ""                                      # the copy's URL, e.g. https://staging.shop.test
production: []                               # never a target; verify flags these, e.g. ["shop.com", "*.shop.com"]
url_map: {}                                  # extra pairs, e.g. {"cdn.shop.com": "cdn.staging.shop.test"}
stack: auto                                  # auto, wordpress, laravel or sql
profiles: [auto]                             # auto: by the tables found. Or name them: [woocommerce, gravity-forms]
remote:
  wp: wp                                     # WP-CLI on the remote host, if not on its PATH

pseudonymise:
  email_domain: example.invalid    # fake emails: u<16 hex>@example.invalid (reserved domains only)
  phone_style: echo                # echo: +1 555 01xx xxx (Echo's test range). country: keep the
                                   # country's fictional range where one exists (UK 07700 900xxx)
  locale: en                       # fake names and streets: en, de, fr, es, nl, it
  keep: [country, state]           # address parts kept for tax and shipping rules
  keep_users: []                   # logins or emails left as they are, e.g. ["admin", "*@agency.example"]
  consistency: per-run             # per-run: a new key each copy. stable: .roadie/key or ROADIE_KEY
  on_unparseable: blank            # a serialized value that won't parse and looks personal: blank or fail
  passwords: reset                 # reset: every user not kept gets an unusable hash. keep
  free_text: scrub                 # scrub: replace known values and email-, phone- and card-shaped
                                   # text in notes, comments and form entries. blank. keep

drop: []                           # tables not copied at all, globs allowed
truncate: []                       # structure copied, rows not (profiles add sessions and mail logs)

columns: []                        # your own tables, or overrides of a profile:
#  - table: wp_loyalty_members
#    column: email
#    rule: email                   # email, phone, name, first_name, last_name, full_name, address,
#                                  # street, city, postcode, ip, dob, company, username, url,
#                                  # user_agent, text (scrub), blank, null, keep, fixed:<value>,
#                                  # json:<path>=<rule>, serialized:<key>=<rule>
#  - table: wp_postmeta
#    where: {meta_key: _billing_vat_number}
#    column: meta_value
#    rule: blank

keys:
  mode: swap                       # swap: the test key below when set, else blank. blank: blank all
  test_keys: {}                    # e.g. {stripe: "${ROADIE_STRIPE_TEST_SECRET}",
                                   #       stripe_publishable: "${ROADIE_STRIPE_TEST_PUBLISHABLE}"}
  ignore: []                       # known test values: [{id: key.generic, location: "wp_options my_flag", reason: "...", until: 2027-01-31}]

side_effects:
  echo: {install: true, url: "http://127.0.0.1:8025", mode: seal, proxy: ""}
  cron: pause                      # pause: DISABLE_WP_CRON true; events kept. keep
  action_scheduler: pause          # pause: runners stopped, pending actions kept. cancel. keep
  webhooks: pause                  # WooCommerce webhooks paused and secrets blanked. keep
  gateways: test-mode              # known gateways to test mode, unknown ones disabled. keep
  subscriptions: staging           # WooCommerce Subscriptions staging mode forced. keep
  mail: seal                       # WordPress mailers through Echo; Laravel MAIL_* to Echo's SMTP

noindex: true                      # mu-plugin noindex header, robots and crawler block
basic_auth: false                  # true prints snippets; Roadie never edits server config

verify:
  fail_on: high
  allow_emails: []                 # business addresses that may stay, e.g. ["info@shop.com"]
  allow_phones: []
  skip: []
  sign: true
  ignore: []                       # [{id: pii.phone, location: "wp_options shop_phone", reason: "...", until: 2027-01-31}]

laravel:                           # only for stack laravel
  env_file: .env
  tables: {}                       # Laravel has no fixed schema; name yours, e.g.
                                   # users: {email: email, name: full_name, phone: phone}
  env: {}                          # extra .env keys to set, e.g. {QUEUE_CONNECTION: sync}
```

## The keys

### Where

| Key | Default | Flag | What it does |
|---|---|---|---|
| `source` | | `--from` | Production: a dump, a site folder, `ssh://`, `@alias` or DSN ([Sources and targets](Roadie-Sources-And-Targets)). |
| `target` | | `--to`, the staging argument | The staging copy, in the same forms. |
| `url` | | `--url` | The copy's URL. Production's URLs are rewritten to it. |
| `production` | `[]` | `--production` (adds) | Hosts, `*.host` wildcards or URLs. Never a target; verify reports any left in a copy. Roadie also merges `production:` from `.echo/config.yaml` and `.soundcheck/config.yaml` in the project. |
| `url_map` | `{}` | `--url-map a=b,...` | Extra host pairs: multisite subsites, mapped domains, a CDN. |
| `stack` | `auto` | `--stack` | `auto`, `wordpress`, `laravel` or `sql`. |
| `profiles` | `[auto]` | | `auto` turns profiles on by what is found; or name them ([Profiles](Roadie-Profiles)). |
| `remote.wp` | `wp` | | WP-CLI's path on ssh hosts. |

### pseudonymise

| Key | Default | Flag | What it does |
|---|---|---|---|
| `email_domain` | `example.invalid` | `--email-domain` | The fake emails' domain. Must be reserved: `example.com`, `example.net`, `example.org` or a name under `.example`, `.invalid`, `.test`, `.localhost`. |
| `phone_style` | `echo` | | `echo`: `+1 555 01x xxxx`. `country`: the country's fictional range where one exists. |
| `locale` | `en` | | Fake names and streets: `en`, `de`, `fr`, `es`, `nl`, `it`. |
| `keep` | `[country, state]` | | Address parts kept as they are. |
| `keep_users` | `[]` | `--keep-users` | Logins or emails kept as they are, globs allowed. |
| `consistency` | `per-run` | `--keep-key` (stable) | `per-run` or `stable` ([the key](Roadie-Pseudonymisation#consistency-and-the-key)). |
| `on_unparseable` | `blank` | | A broken serialized value that looks personal: `blank` it, or `fail` the copy. |
| `passwords` | `reset` | | `reset` or `keep`. |
| `free_text` | `scrub` | | `scrub`, `blank` or `keep` for columns with a `text` rule. |

### columns

A list of rules for your own tables, or overrides of a profile's rule for one column. Each entry has `table`, `column`, `rule` and optionally `where` (`{column: value}`, globs allowed):

```yaml
columns:
  - table: wp_loyalty_members
    column: email
    rule: email
  - table: wp_postmeta
    where: {meta_key: _billing_vat_number}
    column: meta_value
    rule: blank
  - table: Session
    column: data
    rule: json:$.email=email
```

Rules: `email`, `phone`, `name`, `first_name`, `last_name`, `full_name`, `address`, `street`, `city`, `postcode`, `ip`, `dob`, `company`, `username`, `url`, `user_agent`, `text` (scrub), `blank`, `null`, `keep`, `fixed:<value>`, `json:<path>=<rule>`, `serialized:<key>=<rule>`. A `columns:` rule wins over a profile's rule for the same column.

### drop, truncate

`drop` (`--drop`) leaves tables out of the copy entirely; `truncate` (`--truncate`) copies their structure without rows. Globs allowed: `drop: ["wp_*_log", "wp_wfLogins"]`. Profiles already truncate sessions, payment tokens, mail logs, transients and caches.

### keys

| Key | Default | Flag | What it does |
|---|---|---|---|
| `mode` | `swap` | `--keys` | `swap`: a live key gets its test key when one is set, else it is blanked. `blank`: every live key is blanked. |
| `test_keys` | `{}` | | Test keys by provider, from the environment: `{stripe: "${ROADIE_STRIPE_TEST_SECRET}", stripe_publishable: "${ROADIE_STRIPE_TEST_PUBLISHABLE}"}`. A live value here is refused (`test-key-is-live`). |
| `ignore` | `[]` | | Known harmless values: `id`, `location`, `reason`, optional `until`. |

### side_effects

| Key | Default | Other values | What it does |
|---|---|---|---|
| `echo.install` | `true` | `false` | Install the Echo connector on a WordPress target. |
| `echo.url` | `http://127.0.0.1:8025` | | Where the connector reports (`--echo-url`, `RHYTHM_ECHO_URL`). `https`, loopback or a private address only. |
| `echo.mode` | `seal` | | The connector's mode. |
| `echo.proxy` | `""` | | Echo's HTTPS proxy for raw cURL (`--echo-proxy`). |
| `cron` | `pause` | `keep` | `DISABLE_WP_CRON`. |
| `action_scheduler` | `pause` | `cancel`, `keep` | Pause the runners and keep pending actions, or cancel them. |
| `webhooks` | `pause` | `keep` | WooCommerce webhooks. |
| `gateways` | `test-mode` | `keep` | Known gateways to test mode, unknown enabled ones disabled. |
| `subscriptions` | `staging` | `keep` | Force Subscriptions' staging mode. With `keep`, `WP_HOME`, `WP_SITEURL` and the network constants stay too. |
| `mail` | `seal` | | WordPress mailers through Echo; Laravel `MAIL_*` to Echo's SMTP sink. |

`--no-kill-switch` turns every switch off for one run. See [Kill switch](Roadie-Kill-Switch).

### noindex, basic_auth

`noindex: true` (`--noindex=off` for one run) installs the noindex parts of the mu-plugin. `basic_auth: true` (or `--basic-auth <user>`) writes the snippets for Apache, nginx and Caddy.

### verify

| Key | Default | Flag | What it does |
|---|---|---|---|
| `fail_on` | `high` | `--fail-on` | `none`, `info`, `low`, `medium`, `high`, `critical`. |
| `allow_emails` | `[]` | `--allow-emails` | Addresses that may stay real, globs allowed. |
| `allow_phones` | `[]` | `--allow-phones` | Numbers that may stay real. |
| `skip` | `[]` | `--skip` | Checks to leave out. |
| `sign` | `true` | `--sign=off` | Sign the JSON report. |
| `ignore` | `[]` | | Accepted findings: `id`, `location`, `reason`, optional `until`. |

### laravel

`env_file` (default `.env`), `tables` (each table's `column: rule` pairs) and `env` (extra `.env` lines to set). See [Laravel](Roadie-Laravel).

## Rules for the file

- **Unknown keys are errors**, with a suggestion:

  ```text
  rhythm-roadie: bad1.yaml line 2: unknown key pseudonymise.emial_domain; did you mean pseudonymise.email_domain?
  ```

- **Only `${ROADIE_NAME}` is read**, only in string values:

  ```text
  rhythm-roadie: bad2.yaml line 2: keys.test_keys.stripe reads ${STRIPE_TEST}; only ROADIE_ variables are read. Rename it ROADIE_STRIPE_TEST.
  ```

  An unset variable is an error when a run reads it; `config check` lists them first.

- **Fake emails need a reserved domain:**

  ```text
  rhythm-roadie: bad3.yaml line 2: pseudonymise.email_domain is "mycompany.com", which can receive real mail. Use a reserved domain: example.invalid, example.com or a name under .test, .example, .invalid or .localhost.
  ```

- **Globs must be valid:**

  ```text
  rhythm-roadie: bad4.yaml line 1: drop[0] is "wp_[x", which isn't a valid glob (use *, ? and [abc] as in path.Match)
  ```

Each of these exits 2. `--config <file>` reads another file; `--no-config` reads none (use it in folders you don't trust: a config can name targets).

## config show

`config show` prints the effective settings with the source of each value (`file` or `default`), then every rule that may apply to the project. For a config that sets `source`, `url`, `production` and `keep_users`:

```text
# Effective settings from .roadie/config.yaml and the defaults; each value names its source.
source: prod.sql.gz  # file
target: ""  # default
url: https://staging.shop.test  # file
production:  # file
  - shop.com  # file
  - '*.shop.com'  # file
url_map: {}  # default
stack: auto  # default
profiles: [auto]  # default
remote:
  wp: wp  # default
pseudonymise:
  email_domain: example.invalid  # default
  phone_style: echo  # default
  locale: en  # default
  keep: [country, state]  # default
  keep_users: [admin, '*@agency.example']  # file
  consistency: per-run  # default
  on_unparseable: blank  # default
  passwords: reset  # default
  free_text: scrub  # default
...
# Rules, expanded: each profile's rules apply when its tables, options or files are found; columns: always apply
# action-scheduler (Action Scheduler)
#   {prefix}{site}actionscheduler_actions.args: scrub text
#   {prefix}{site}actionscheduler_actions.extended_args: scrub text
#   {prefix}{site}actionscheduler_logs.message: scrub text
# automatewoo (AutomateWoo)
#   {prefix}{site}automatewoo_guests.email: fake email
...
```

Secrets are masked. `config show --json` gives the same as data (`data.config`, `data.sources`, `data.production_from`, `data.unset`, `data.rules`); `config check --json` gives `{"path", "valid", "unset": [{"name", "line"}]}`.

## Environment variables

| Name | Use |
|---|---|
| `ROADIE_FROM_PASSWORD`, `ROADIE_TO_PASSWORD` | DSN passwords for the source and the target. |
| `ROADIE_KEY` | The consistency key, base64 of 32 bytes. Wins over `.roadie/key`. |
| `ROADIE_SIGNING_KEY` | The report signing key, base64 of a 32-byte Ed25519 seed. |
| `ROADIE_*` | Anything the config reads with `${ROADIE_NAME}`; `--admin-password-env NAME` reads `NAME`, which must start with `ROADIE_`. |
| `RHYTHM_ECHO_URL` | The default Echo URL when the config names none. |
| `RHYTHM_ECHO_TOKEN` | Echo's token for `verify --echo` beyond loopback. Never shown in a finding or report. |
| `RHYTHM_OFFLINE` | Built-in profiles only, no update check (as `--offline`). |
| `RHYTHM_NO_UPDATE_CHECK`, `RHYTHM_UPDATE_PROMPT`, `RHYTHM_CONFIG_DIR`, `RHYTHM_CACHE_DIR`, `NO_COLOR`, `CI` | As for every Rhythm tool ([Command reference](Command-Reference)). |
| `GOMEMLIMIT` | The soft memory limit when `--max-memory` isn't given (default 300 MiB). |

## Files Roadie keeps

| Path | What | Mode |
|---|---|---|
| `.roadie/config.yaml` | settings, committed | yours |
| `.roadie/reports/<UTC time>Z-<command>-<target>/report.json`, `report.html` | every copy, prepare and verify; the JSON is signed | 0600, folders 0700 |
| `.roadie/backups/<UTC time>.sql.gz` (`.sqlite` for SQLite) | `prepare`'s backup of a local or DSN target | 0600, folder 0700 |
| `~/.roadie-backups/<UTC time>.sql.gz` on the ssh host | `prepare`'s backup of an ssh target | 0600 |
| `.roadie/key` | the consistency key, with `consistency: stable` or `--keep-key` | 0600 |
| `.roadie/basic-auth/` | basic-auth snippets and `.htpasswd` | 0600 |
| `~/.config/rhythm/roadie/signing.key` | the report signing key | 0600 |
| `~/.cache/rhythm/roadie/profiles/` | a downloaded profile pack | 0600 |
| `~/.cache/rhythm/roadie/spool/` | the encrypted spool of a streamed source, removed when the run ends | 0600, folder 0700 |
| `~/.cache/rhythm/roadie/locks/` | one lock per target, so two runs can't write to it at once | 0600 |

The `.roadie/` paths are next to the config in use (the folder above `.roadie/`, or the `--config` file's folder); without a config, in the folder you run from. A folder that isn't writable sends reports to the cache folder instead. Paths are Linux's; macOS and Windows use their own config and cache folders, as every Rhythm tool does ([Uninstall](Uninstall)). On Windows, file modes aren't enforced; your user profile's permissions protect these files.

---
<sub>← [CI](Roadie-CI) · [Home](Home) · [Security and limits](Roadie-Security-And-Limits) →</sub>
