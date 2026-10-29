Every Roadie command and flag, its exit codes, the JSON every command writes, the error codes and the environment variables it reads. Verified against rhythm-roadie 1.0.0.

Roadie is the `rhythm-roadie` binary. `rhythm roadie …` runs it, as do `rhythm ro …` and `rhythm-roadie …` directly: they're the same command. The help texts below are the binary's own, so they say `rhythm-roadie`.

## Commands

```text
♪ rhythm roadie  1.0.0
Is my staging a safe copy of production? Make pseudonymised staging copies with
live keys swapped and side effects off, then prove it with a signed report.

Usage:
  rhythm-roadie                                 pick what to do (on a terminal)
  rhythm-roadie copy --from <source> --to <target> [flags]
                                                copy production to staging: pseudonymise,
                                                swap keys, turn side effects off, verify
  rhythm-roadie prepare <staging> [flags]       make an existing staging copy safe in place
  rhythm-roadie verify [staging] [flags]        look for real people, live keys and
                                                production URLs; signed pass/fail report
  rhythm-roadie plan --from <source> [--to <target>]
  rhythm-roadie plan <staging>                  dry run: every change copy or prepare would make
  rhythm-roadie config init [--stack s]         write .roadie/config.yaml to edit
  rhythm-roadie config check | show             validate the config / print it with defaults filled in
  rhythm-roadie config key show|rotate|forget   the key that keeps fake values the same across refreshes
  rhythm-roadie report <report.json>... -o <file>
                                                render saved reports as HTML, Markdown, SARIF or JUnit
  rhythm-roadie report check <report.json>      check a report's signature
  rhythm-roadie profiles [name] [--json]        list the stack profiles and what each rule changes
  rhythm-roadie explain <id>                    a rule's or finding's full explanation, offline
  rhythm-roadie update [--clear]                refresh the signed profile pack
  rhythm-roadie demo [--keep]                   try the whole loop on a built-in shop
  rhythm-roadie help [command] | version        this help, one command's, or the version
```

| Command | What it does | Page |
|---|---|---|
| `copy --from <source> --to <target>` | copy production to staging: pseudonymise, swap keys, turn side effects off, verify | [Copy and prepare](Roadie-Copy-And-Prepare#copy) |
| `prepare <staging>` | make an existing staging copy safe in place, after a backup | [Copy and prepare](Roadie-Copy-And-Prepare#prepare) |
| `verify [staging]` | look for real people, live keys, production URLs and side effects; signed pass or fail | [Verify and reports](Roadie-Verify-And-Reports) |
| `plan` | the same arguments as copy or prepare; every change listed, none made | [Copy and prepare](Roadie-Copy-And-Prepare#plan) |
| `config init`, `check`, `show`, `key` | write, validate and print `.roadie/config.yaml`; manage the consistency key | [Configuration](Roadie-Configuration) |
| `report`, `report check` | render saved reports; check a report's signature | [Verify and reports](Roadie-Verify-And-Reports#reports) |
| `profiles [name]` | the stack profiles and their rules | [Profiles](Roadie-Profiles) |
| `explain <id>` | a finding's, rule's or error code's explanation, offline | below |
| `update` | refresh the signed profile pack | below |
| `demo` | the whole loop on a built-in shop, offline | [Getting started](Roadie-Getting-Started#2-try-the-demo) |
| `help [command]`, `version` | help; the version (`version --json` for `rhythm-version/v1`) | |

`rhythm roadie` with no arguments on a terminal opens a picker:

```text
┃ What should Roadie do?
┃ > Make a staging copy from production (copy)
┃   Make an existing staging site safe (prepare)
┃   Check a staging copy (verify)
┃   See what a copy would change (plan)
┃   Try it on a built-in shop (demo)
┃   Write a config file to edit (config init)

↑ up • ↓ down • / filter • enter submit
```

`copy` then asks where production is, where the copy goes and the copy's URL, shows the plan, asks `Copy now? [y/N]` and at the end offers to save the answers to `.roadie/config.yaml`. Without a terminal, `rhythm roadie` prints the help and exits 0.

`rhythm roadie help <command>` (or `<command> --help`) prints one command's help. Mistakes are caught before anything runs, with a suggestion (exit 2):

```text
rhythm-roadie: unknown command "coppy"
  did you mean: rhythm-roadie copy
  run 'rhythm-roadie help' to see the commands
```

```text
rhythm-roadie copy: unknown flag --form
  did you mean: rhythm-roadie copy --from
  run 'rhythm-roadie help copy' for usage
```

```text
rhythm-roadie verify: --fail-on "loud": want none, info, low, medium, high or critical
  run 'rhythm-roadie help verify' for usage
```

The first command on a machine (other than `help`, `version` and `update`) prints a one-time note on stderr, never with `--json`:

```text
Note: use Roadie on sites you own or are authorised to copy. It reads production
data to pseudonymise it and never writes to production. Pseudonymised data is still
personal data: keep copies as protected as the live site. This notice is shown once.
```

## Flags every command takes

```text
Output:
      --json                 JSON (schema rhythm-roadie/v1)
      --md                   Markdown for pull requests and CI summaries
      --html                 standalone HTML report
      --sarif                SARIF 2.1.0 (verify only)
      --junit                JUnit XML for CI test views (verify only)
  -o, --output <file>        write the report to a file (format from the extension:
                             .json .md .html .sarif .xml), keep the live view;
                             repeat it for several files. With a file of the same
                             format, --json, --md and the others write only the file
      --open                 open the HTML report when done
  -q, --quiet                results only
      --plain                no color, boxes or glyphs (screen readers, logs)
      --no-color             no color (NO_COLOR too)
  -v, --verbose              every table, rule and file edit as it happens
      --debug                timings and counts too (never values)

Settings:
      --config <file>        settings file (default the nearest .roadie/config.yaml, here or above)
      --no-config            don't read .roadie/config.yaml; use it in folders you don't trust
      --offline              built-in profiles only, no update check (RHYTHM_OFFLINE=1 too)
```

Without a terminal (piped, in CI) output is plain, as `--plain` gives. Roadie never asks a question there. `-o` writes a file only once it is complete; one format goes to stdout and the rest with `-o`. A format flag with an `-o` file of the same format (`--md -o r.md`) writes only the file and stdout keeps the live view (`-q` keeps it to the result). Progress words (`Indexing`, `Streaming`, ...) show only on a terminal, never in plain output.

## copy

```text
rhythm roadie copy: copy production to staging, pseudonymised, keys swapped, side effects off

Usage:
  rhythm-roadie copy --from <source> --to <target> [flags]

Where:
      --from <source>        dump file, -, site path, ssh://user@host/path, @alias or DSN
                             (default source: in the config)
      --to <target>          dump file, -, site path, ssh://..., @alias or DSN
                             (default target: in the config)
      --url <url>            the staging site's URL (default url: in the config; asked on a
                             terminal when neither is set; not needed with stack sql).
                             Production's URL is read from the source (siteurl, home, APP_URL)
      --url-map <a=b,...>    extra URL pairs, e.g. for multisite subsites or a CDN:
                             fr.shop.com=fr.staging.shop.test,cdn.shop.com=cdn.staging.shop.test
      --stack <name>         wordpress, laravel or sql (default: detected)
      --site <id>            multisite: only these blog IDs (repeatable; default all)
      --schema <file>        CREATE TABLE statements for a data-only dump
      --dry-run              run plan with the same arguments: every change, none made

Pseudonymise:
      --keep-users <list>    user logins or emails kept as they are, globs allowed,
                             e.g. admin,*@agency.example (default keep_users: in the config)
      --keep-key             keep the consistency key in .roadie/key, so the next refresh
                             gives each real person the same fake (default: a new key per run)
      --email-domain <d>     reserved domain for fake emails (default example.invalid)
      --admin-password-env <NAME>
                             set every kept user's password from this ROADIE_ variable
                             (default: kept users keep their password hash)
      --allow-uncovered      copy even when columns that look personal have no rule
                             (they are listed as findings; default: stop before writing)
      --drop <tables>        leave these tables out of the copy, globs allowed
      --truncate <tables>    copy the table's structure but not its rows
      --one-pass             skip the name index pass (faster; names in free text stay)
      --keep-routines        keep triggers, procedures and views (DEFINER removed)
      --keep-app-key         Laravel: keep APP_KEY (encrypted columns stay readable)

Keys and side effects:
      --keys <mode>          swap (default): a live key becomes the test key you set in
                             keys.test_keys, else it is blanked. blank: blank every live key
      --no-kill-switch       copy the data only (verify will FAIL on side effects)
      --echo-url <url>       where the installed Echo connector reports (default
                             http://127.0.0.1:8025; RHYTHM_ECHO_URL)
      --echo-proxy <addr>    also route raw cURL through Echo's proxy, e.g. 127.0.0.1:8026
      --noindex=off          don't add noindex, robots and the crawler block (default on)
      --basic-auth <user>    also write basic-auth snippets with a generated password,
                             printed once (default off; on GitHub Actions it is
                             masked and kept in .roadie/basic-auth/password). The
                             user is 1 to 64 letters, digits or . _ - @

Target safety:
      --production <list>    production hosts or URLs, e.g. shop.com,*.shop.com (adds to
                             production: in the config); a target that matches is refused
      --confirm-target <host>
                             the target's host or database name (a SQLite file's path or
                             name), typed out; needed when the target is a site or
                             database and there's no terminal
      --target-is-staging <host>
                             the target says it's production (WP_ENVIRONMENT_TYPE or APP_ENV)
                             but isn't: name its host to go on. Never overrides a production:
                             match or a target that is the source
  -y, --yes                  don't ask before replacing the target's database
      --fix-collation        rewrite MySQL 8 collations (utf8mb4_0900_*) for older MariaDB
      --jobs <n>             transform workers (default the number of CPUs, at most 8)
      --max-memory <size>    soft memory limit (default 300MiB, or GOMEMLIMIT)
      --max-value <size>     largest single value accepted (default 256MiB)
      --max-output-ratio <n> stop when the dump grows past n times its compressed size
                             (default 20)

After the copy:
      --no-verify            skip the verify at the end (exit 0 means "copied", not "checked")
      --fail-on <level>      exit 1 when verify finds something at or above none, info, low,
                             medium, high (default) or critical
      --soundcheck           also run rhythm-soundcheck run <url> --json on the copy
      --sign=off             don't sign the JSON report (default: sign it)
      --key <file>           sign with this private Ed25519 key file instead
                             (ROADIE_SIGNING_KEY in CI); report check --key is the
                             public key or key ID to check against
```

Examples:

```sh
rhythm roadie copy --from shop.sql --to staging.sql --url https://staging.shop.test
rhythm roadie copy --from ssh://deploy@shop.com/var/www/shop --to /var/www/staging --url https://staging.shop.test
rhythm roadie copy --from @prod --to @staging --keep-users 'admin,*@agency.example' --keep-key
gunzip -c shop.sql.gz | rhythm roadie copy --from - --to - --url https://staging.shop.test > staging.sql
ROADIE_TO_PASSWORD=... rhythm roadie copy --from prod.sql --to mysql://staging@db.internal/shop --confirm-target shop --yes
```

## prepare

```text
rhythm roadie prepare: make an existing staging copy safe in place

Usage:
  rhythm-roadie prepare <staging> [flags]
  (staging: a site path, ssh://..., @alias or DSN; default target: in the config)

Where:
      --url <url>            the staging site's URL (default url: in the config)
      --url-map <a=b,...>    extra URL pairs, e.g. for multisite subsites or a CDN
      --stack <name>         wordpress, laravel or sql (default: detected)
      --site <id>            multisite: only these blog IDs (repeatable; default all)
      --only <parts>         data, keys, side-effects, urls, noindex (default all). E.g.
                             --only side-effects re-applies the kill switch after a plugin
                             update without touching the data
      --production-url <url> the live site's URL, for rewriting and for verify (default
                             production: in the config; asked on a terminal when the copy
                             still holds it as siteurl)
      --no-backup            don't back up the database first (default: back it up)
      --dry-run              run plan with the same arguments: every change, none made
```

The Pseudonymise, Keys and side effects, Target safety and After the copy flags are copy's, above. On a terminal prepare always asks for the host to be typed; without one it needs `--confirm-target <host>` (`--yes` alone stops with `prepare-confirm`).

```sh
rhythm roadie prepare /var/www/staging
rhythm roadie prepare /var/www/staging --only side-effects
rhythm roadie prepare @staging --confirm-target staging.shop.com --production-url https://shop.com
```

## verify

```text
rhythm roadie verify: look for real people, live keys and production URLs in a staging copy

Usage:
  rhythm-roadie verify [staging] [flags]
  (staging: a site path, ssh://..., @alias, DSN or dump file; default target: in the config)

What to check:
      --production <list>    production hosts or URLs, e.g. shop.com,*.shop.com (adds to
                             production: in the config)
      --from <source>        production's dump or database: read it to check for every real
                             value exactly (exact mode) instead of by shape
      --allow-emails <globs> addresses that may stay real, e.g. *@agency.example
      --allow-phones <globs> numbers that may stay real
      --site <url>           also fetch the staging site's /robots.txt and home page headers
                             (never a production host) to check noindex and basic auth
      --insecure-tls <host>  accept a self-signed certificate on this one staging host
      --echo[=<url>]         also ask Echo whether its seal is on and how many messages it
                             let through since the copy (default http://127.0.0.1:8025;
                             RHYTHM_ECHO_TOKEN for an Echo beyond loopback)
      --sample <n>           rows checked per table (default 0: every row)
      --skip <checks>        leave these checks out: emails, phones, names, addresses, ips,
                             cards, keys, urls, side-effects, config, uploads
      --max-value <size>     largest single value read from a dump (default 256MiB)
      --max-output-ratio <n> stop when a dump grows past n times its compressed size
                             (default 20); both apply to --from too

CI:
      --fail-on <level>      exit 1 at or above none, info, low, medium, high (default) or
                             critical
      --baseline <file>      hide findings already in this JSON report (accepted test data)
      --sign=off             don't sign the JSON report (default: sign it)
      --key <file>           sign with this private Ed25519 key file instead
                             (ROADIE_SIGNING_KEY in CI); report check --key is the
                             public key or key ID to check against
```

```sh
rhythm roadie verify /var/www/staging --production shop.com
rhythm roadie verify staging.sql --from shop.sql
rhythm roadie verify mysql://ci@127.0.0.1/staging --fail-on medium --json -o roadie.sarif
rhythm roadie verify /var/www/staging --site https://staging.shop.test --echo
rhythm roadie verify - < staging.sql
```

## plan

```text
rhythm roadie plan: dry run of copy or prepare; lists every change and makes none

Usage:
  rhythm-roadie plan --from <source> [--to <target>] [flags]
  rhythm-roadie plan <staging> [flags]

plan takes every flag of copy (rhythm-roadie help copy) or, with <staging>, of
prepare (rhythm-roadie help prepare). It opens the source and the target
read-only, lists every table, rule, key, file edit, remote command and side
effect and writes only its report.

Plan:
      --schema-only          read only CREATE TABLE statements or information_schema,
                             not the rows (faster; row counts are estimates)
      --fail-on <level>      exit 1 when the copy would stop or leave a finding at or
                             above none (default), info, low, medium, high or critical
```

## config

```text
rhythm roadie config: write, check and print .roadie/config.yaml and its key

Usage:
  rhythm-roadie config init [--stack wordpress|woocommerce|laravel|sql] [--from <source>] [--force]
  rhythm-roadie config check [--json]
  rhythm-roadie config show [--json]
  rhythm-roadie config key show|rotate|forget [--force]

  init     write .roadie/config.yaml with every key commented
  check    parse and validate it, list the ${ROADIE_*} variables that are unset
  show     the effective config with defaults, where each value comes from
           (production: of .soundcheck/ and .echo/ too) and the rules expanded:
           columns: and each profile it may apply; secrets masked
  key      the consistency key's fingerprint, a new key, or delete .roadie/key
           (ROADIE_KEY wins over the file)

Config:
      --stack <name>         init: wordpress (default), woocommerce, laravel or sql
      --from <source>        init: read this source's schema (read-only) and list its
                             unknown tables with personal-looking columns as suggestions
      --force                init: replace an existing .roadie/config.yaml;
                             key rotate: replace the key without a terminal
```

`config key rotate` and `forget` ask on a terminal; `--force` is for scripts. Output of each: [Pseudonymisation](Roadie-Pseudonymisation#consistency-and-the-key).

## report

```text
rhythm roadie report: render saved reports, or check a report's signature

Usage:
  rhythm-roadie report <report.json>... -o <file> [--brand-name <name>] [--brand-logo <file>]
  rhythm-roadie report check <report.json> [--key <file|id>] [--input <dump>]

  Several reports make one summary page, worst first. The format comes from
  the -o file's extension: .html .md .sarif .xml

Report:
      --brand-name <name>    a name for the top of the HTML report, e.g. your agency's
      --brand-logo <file>    a PNG, SVG or JPEG logo for the HTML report (under 512 KiB)
      --key <file|id>        check: the public key file or key ID (roadie:..., which
                             report check prints) the report must be signed with;
                             copy and verify --key is the private key that signs
      --input <dump>         check: the dump the report describes; its fingerprint must match
```

Exit codes of `report check`: 0 valid, 1 edited, another input or another key, 3 unsigned. Outputs: [Verify and reports](Roadie-Verify-And-Reports#report-check).

## profiles

```text
rhythm roadie profiles: list the stack profiles and what each rule changes

Usage:
  rhythm-roadie profiles [name] [--json]

  Without a name: every profile with its ID, name and what it was checked
  against. With a name: that profile's detector, tables, columns, rules and
  kept fields.
```

## explain

```text
rhythm roadie explain: a finding's, rule's or error code's full explanation, offline

Usage:
  rhythm-roadie explain <id>

  e.g. rhythm-roadie explain key.stripe-live, explain side-effect.action-scheduler,
  explain uncovered-column, explain pseudonymised
```

Every finding ID, rule, profile and error code has a page:

```text
$ rhythm roadie explain uncovered-column
# uncovered-column: a column that looks personal has no rule (medium)

What: A text column no profile or columns: rule covers, whose name looks
personal (email, phone, name, address, ip, dob and similar) or whose sampled
values are 20% or more emails or phones.

Why it matters: Roadie only rewrites what a rule covers. An unknown table with
customer data would reach the copy as it is.

What Roadie does: copy and prepare stop before writing anything (exit 1,
error code uncovered) unless --allow-uncovered; plan lists each column.

By hand: Add the column under columns: in .roadie/config.yaml with its rule,
e.g. table: wp_loyalty_members, column: email, rule: email. rhythm-roadie
config init --from <source> suggests the entries.
```

## update

```text
rhythm roadie update: refresh the signed profile pack

Usage:
  rhythm-roadie update [--clear] [--json] [--pin profiles=<version>]

  Downloads the latest signed profile pack, checks its signature and keeps it
  in the cache; copy, prepare, plan and verify use it when it is newer than
  the built-in one. Offline (--offline or RHYTHM_OFFLINE=1) it downloads
  nothing and reports the pack in use.

Update:
      --clear                drop the downloaded pack and use the built-in one
      --json                 print the rhythm-data/v1 JSON document
      --pin <name=version>   download these versions instead of the latest,
                             as name=version,... (profiles=<version>)
      --offline              download nothing; report the pack in use
```

A downloaded pack is used only when it is newer than the one built in and only after its Sigstore signature and checksum check out (the same trust path as the binary). Reports record the pack version. `rhythm update` calls it for you. Offline:

```text
$ RHYTHM_OFFLINE=1 rhythm roadie update
Offline: nothing was downloaded. This is the profile pack Roadie uses now.
profiles 1.0.0

$ RHYTHM_OFFLINE=1 rhythm roadie update --json
{"schema":"rhythm-data/v1","data":[{"name":"profiles","version":"1.0.0","sha256":"0033e40590dd32d37562c6350e55abaf480e943ee21699061bfa075aa2e4aea6"}]}

$ rhythm roadie update --clear
Removed the cached profile pack; Roadie uses its built-in profiles until rhythm-roadie update runs again.
```

## demo

```text
rhythm roadie demo: try the whole loop on a built-in shop, offline

Usage:
  rhythm-roadie demo [--keep]

  Builds a small WooCommerce dump with fake "real" customers, copies it to a
  temporary dump file, verifies the copy (PASS), then verifies the untouched
  input (FAIL). No network, no database server.

Demo:
      --keep                 keep the demo's dumps and reports (default: delete them)
```

## Exit codes

| Code | Means | Examples |
|---|---|---|
| 0 | ok | done; verify found nothing at or above `--fail-on`; `plan` without `--fail-on`; `report check` valid |
| 1 | findings | verify found something at or above `--fail-on` (the copy was written but isn't safe yet); a copy stopped on uncovered columns (target untouched); `plan --fail-on` would stop or leave a finding; `report check` invalid; a live key in code; a kill switch file that couldn't be written |
| 2 | usage | bad flags or arguments; a bad `.roadie/config.yaml`; an unsupported dump format; no profile matches; output equals input; a target that needs confirming without a terminal; a missing path; a signing key (`--key`, `ROADIE_SIGNING_KEY`) that can't be read, caught before the run starts |
| 3 | couldn't run | unreachable source or target; a database, ssh or WP-CLI error; a dump that ends early, is refused or grows too large; a target refused as production or as the source; nothing scanned; an incomplete verify; another run holds the target; an unsigned report (`report check`) |
| 130 | interrupted | Ctrl-C or SIGTERM |

A run that stops prints a box with the error code and the target's state, the message and the `explain` command:

```text
STOPPED   output-is-input; the target is untouched

output-is-input: --to shop.sql is the file Roadie reads. Write the copy to a new file.

rhythm-roadie explain output-is-input
```

Every code, its message and its fix: [Troubleshooting](Roadie-Troubleshooting).

## JSON output

`--json` prints one envelope on stdout; the same document, signed, is `report.json`:

| Field | Value |
|---|---|
| `schema` | `rhythm-roadie/v1` |
| `kind` | `plan`, `copy`, `prepare`, `verify`, `config`, `config-check`, `profiles`, `report-check`, `demo` |
| `tool`, `version` | `rhythm-roadie`, `1.0.0` |
| `complete` | `false` for a sampled or incomplete verify, or a run that stopped |
| `summary` | `critical`, `high`, `medium`, `low`, `info`, `tables`, `tables_pseudonymised`, `rows`, `values_pseudonymised`, `urls_rewritten`, `keys_found`, `keys_swapped`, `keys_blanked`, `side_effects_off`, `side_effects_total`, `uncovered_columns` |
| `verdict` | `ok`, `findings` or `error` |
| `data` | the result |

`data` of `copy`, `prepare`, `verify` and `plan`:

| Field | Value |
|---|---|
| `result` | `pass`, `fail` or `error` |
| `verify_mode` | `exact`, `heuristic` or `none` |
| `fail_on`, `started` (RFC 3339 UTC), `duration_ms` | |
| `source`, `target` | `kind` (`dump-file`, `stdio`, `local-site`, `ssh-site`, `wp-alias`, `dsn`), `display`, `engine`, `server`, `bytes` (a dump file's size; `0` when not known, as for a database); the target also `state` |
| `target_state` | `untouched`, `replaced` or `mixed` (plan always `untouched`); `""` for verify, which has no target to change; `target.state` too |
| `production`, `url` | production's hosts; the staging URL |
| `stack` | `name`, `profiles`, `multisite`, `hpos`, `sites` |
| `consistency` | `mode`, `key_fingerprint` (`null` for per-run) |
| `input`, `output` | `kind` (`sha256-file` or `db-tables`), `value`, `bytes`: the fingerprints `report check --input` compares |
| `ruleset_sha256`, `config_sha256`, `profile_pack`, `echo_connector_sha256` | what ran |
| `tables[]` | `name`, `rows`, `action`, `profile`, `columns[]` (`name`, `rule`, `changed`, `kept`) |
| `covered[]`, `uncovered[]` | `table`, `column`, `rate` |
| `keys[]` | `id`, `severity`, `provider`, `env`, `location`, `masked`, `action` |
| `side_effects[]` | `id`, `status` (`off`, `on`, `not-present`, ...), `how`, `undo` |
| `findings[]` | `id`, `severity`, `title`, `count`, `locations[]` (`table`, `column`, `row`, `path`, `file`, `line`, `count`), `examples` (masked), `fix` (set on every finding of low or above: what to do, or the `explain` page that says it; an info finding is a note and may have none) |
| `checks[]` | `id`, `status` (`pass`, `warn`, `fail`, `skip`), `checked`, `detail` |
| `coverage` | `tables_scanned`, `tables_total`, `rows_scanned`, `bytes_scanned`, `skipped[]` with reasons, `sample` |
| `remote_commands[]` | the commands a plan may run over ssh |
| `not_covered[]` | always present |
| `soundcheck` | `copy --soundcheck`: `ran`, `exit_code` and counts, else `null` |
| `signature` | `alg` (`ed25519`), `key_id`, `public_key`, `sig` |
| `error` | a run that stopped: `code`, `message`, `exit`; a `prepare` that stopped after its backup also `backup` and `restore` |

A finding, from the verify of `shop.sql`:

```json
{
  "id": "key.stripe-live",
  "severity": "critical",
  "title": "A live Stripe key",
  "count": 1,
  "locations": [
    {
      "table": "wp_options",
      "column": "option_value",
      "row": "option_id=10,option_name=woocommerce_stripe_settings",
      "path": "serialized:secret_key",
      "count": 1
    }
  ],
  "examples": [
    "sk_l•••••"
  ],
  "fix": "Swap it for a test key or blank it: rhythm-roadie prepare <staging> --only keys, or copy again for a dump (rhythm-roadie explain key.stripe-live)."
}
```

A key, from the copy:

```json
{"id": "key.stripe-publishable-live", "severity": "high", "provider": "stripe-publishable", "env": "live", "location": {"table": "wp_options", "column": "option_value", "row": "option_name=woocommerce_stripe_settings", "path": "serialized:publishable_key"}, "masked": "pk_l•••••", "action": "blanked"}
```

Fields are only added within `rhythm-roadie/v1`, never renamed or removed. `config check --json` gives `{"path", "valid", "unset": [{"name", "line"}]}` in `data`; `update --json` gives kit's `rhythm-data/v1`; `version --json` gives `rhythm-version/v1` (a source build from `make build` between release tags says `"version":"dev"` with its commit; `go build` alone leaves `commit` empty):

```json
{"schema":"rhythm-version/v1","name":"rhythm-roadie","version":"1.0.0","commit":"70e086b","kit_version":"v0.8.0"}
```

## Environment

`ROADIE_FROM_PASSWORD`, `ROADIE_TO_PASSWORD`, `ROADIE_KEY`, `ROADIE_SIGNING_KEY`, any `ROADIE_*` the config reads, `RHYTHM_ECHO_URL`, `RHYTHM_ECHO_TOKEN`, `RHYTHM_OFFLINE`, `GOMEMLIMIT` and the variables every Rhythm tool reads. Each is described in [Configuration](Roadie-Configuration#environment-variables).

---
<sub>← [Security and limits](Roadie-Security-And-Limits) · [Home](Home) · [Troubleshooting](Roadie-Troubleshooting) →</sub>
