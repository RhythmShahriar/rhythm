Roadie changes a database in two ways. `copy` reads production and writes a pseudonymised copy to a new place. `prepare` takes a staging copy that already exists, made by WP STAGING, WP Migrate, a host panel or a restore. It makes that copy safe in place, after a backup. `plan` runs either one without changing anything. This page explains what each one does, in order, what can stop it and what it leaves behind.

The examples use the shop from [Getting started](Roadie-Getting-Started): production `https://shop.example.com`, its dump `shop.sql` and a WordPress staging site at `/srv/staging` for `https://staging.shop.test`, on MariaDB 10.11. Every output is from Roadie 1.0.0.

## The order of a copy

1. **Read the settings.** Flags win over `.roadie/config.yaml`, which wins over the defaults. Roadie takes a lock on the target, so two runs can't write to it at once.
2. **Guards, read-only.** The target is refused if it matches `production:`, is the source, or says it is production. Nothing is written when a guard refuses. See [Target safety](#target-safety).
3. **Pass 1: index.** Roadie opens the source read-only and reads the schema and every personal value: it detects the stack, the profiles, multisite, HPOS and production's URLs, counts rows, looks for [uncovered columns](#uncovered-columns) and builds the set of real values exact mode checks for. A source that can't be read twice (ssh, an alias, stdin, a live database) is spooled encrypted during this pass ([the spool](Roadie-Security-And-Limits#no-real-data-on-your-disk)).
4. **Plan and confirm.** The plan header is printed; a site or database target asks `Replace the database of <target>? [y/N]` unless `--yes`.
5. **Kill switch, files first.** On a site target, `wp-config.php`, `.env` and the mu-plugins are written before any row, so nothing can send while the import runs.
6. **Pass 2: transform.** Rows stream through the rules to the target. Gateways, webhooks, mailers and Subscriptions in the database are rules in the same stream.
7. **Commit.** A dump file is written under a temporary name and renamed when complete; a database import finishes its last table.
8. **Verify.** The written copy is read back and checked in exact mode (or not, with `--no-verify`).
9. **Report.** The signed report goes to `.roadie/reports/<UTC time>Z-<command>-<target>/`, the result box and next steps to the screen. Then the lock is released.

`prepare` is the same with the staging as both source and target, plus a backup after the guards and, on WordPress, a `.maintenance` file during the import. `plan` is steps 1 to 4 with nothing written but its report.

## Plan

```sh
rhythm roadie plan --from shop.sql --to staging.sql --url https://staging.shop.test
rhythm roadie plan /srv/staging            # what prepare would do
rhythm roadie copy ... --dry-run           # the same as plan with the same arguments
```

`plan` takes every flag `copy` or `prepare` takes. It opens the source and the target read-only, reads every row so the counts are exact (`--schema-only` reads only `CREATE TABLE` or `information_schema`, faster, with estimated counts) and lists every table, rule, key, file edit, remote command and side effect. [Getting started](Roadie-Getting-Started#4-look-first) shows a full plan for a dump. On a site the plan adds the kill switch's file edits; this is the plan for `prepare /srv/staging`, a staging made by a clone tool that kept production's customers and keys:

```text
  SIDE EFFECTS  8 ────────────────────────────────────────────────────────────

  ✓ WP-Cron                  define( 'DISABLE_WP_CRON', true );
  · Subscriptions            WooCommerce Subscriptions isn't on this copy
  ✓ Echo connector, sealed   Echo connector in seal mode
                             (http://127.0.0.1:8025); Echo connector
                             settings; sendmail_path =
                             "/usr/local/bin/rhythm-echo sendmail -t -i"
                             (the path of rhythm-echo on this host), so
                             PHP's mail() reaches Echo too
  ✓ Action Scheduler         Roadie mu-plugin: Action Scheduler paused,
                             Subscriptions staging mode, Jetpack offline,
                             wp_mail blocked without Echo, noindex
  · Jetpack                  Jetpack isn't on this copy
  ✓ WooCommerce webhooks     status to paused, secret blanked
  ✓ Gateways                 Stripe to test mode, live keys swapped or
                             blanked
  · AutomateWoo              AutomateWoo isn't on this copy
...
  FILE EDITS  12 ─────────────────────────────────────────────────────────────

    · wp-config.php: define( 'WP_ENVIRONMENT_TYPE', 'staging' );
    · wp-config.php: define( 'DISABLE_WP_CRON', true );
    · wp-content/mu-plugins/roadie-staging.php:
      woocommerce_subscriptions_is_duplicate_site returns true;
      wc_subscriptions_siteurl left as it is
    · wp-config.php: the eight salts (AUTH_KEY to NONCE_SALT) regenerated,
      64 random characters each
    · wp-content/mu-plugins/rhythm-echo.php: Echo connector in seal mode
      (http://127.0.0.1:8025)
    · wp-content/mu-plugins/rhythm-echo/settings.php: Echo connector
      settings
    · by hand, php.ini: sendmail_path = "/usr/local/bin/rhythm-echo sendmail
      -t -i" (the path of rhythm-echo on this host), so PHP's mail() reaches
      Echo too
    · wp-content/mu-plugins/roadie-staging.php: Roadie mu-plugin: Action
      Scheduler paused, Subscriptions staging mode, Jetpack offline, wp_mail
      blocked without Echo, noindex
    · wp-content/mu-plugins/roadie-staging.php: jetpack_offline_mode returns
      true
    · wc_webhooks: status to paused, secret blanked
    · options woocommerce_stripe_settings: Stripe to test mode, live keys
      swapped or blanked
    · wp-content/mu-plugins/roadie-staging.php: noindex and nofollow
      (wp_robots and X-Robots-Tag), robots.txt from the robots_txt filter,
      sitemaps off; blog_public in the database left as it is
```

A line that starts `by hand` is a step Roadie can't take itself (it never edits `php.ini`). A plan whose source or target is reached over ssh lists every command it may run there under REMOTE COMMANDS; see [Sources and targets](Roadie-Sources-And-Targets#remote-commands).

`plan` exits 0 whatever it finds. With `--fail-on <level>` it exits 1 when the copy would stop or would leave a finding at or above that level, which makes it a CI gate for config changes.

## Copy

```sh
rhythm roadie copy --from shop.sql --to staging.sql --url https://staging.shop.test
rhythm roadie copy                                  # source:, target: and url: from the config
gunzip -c shop.sql.gz | rhythm roadie copy --from - --to - --url https://staging.shop.test > staging.sql
```

`--url` is the copy's URL; production's URL is read from the source (`siteurl`, `home`, `APP_URL`). On a terminal Roadie asks for it when neither the flag nor `url:` sets it; a `stack: sql` copy needs none. [Getting started](Roadie-Getting-Started#5-copy) shows a copy to a dump and to a site.

A copy to stdout can't be read back, so it isn't verified. The screen goes to stderr and says so:

```text
rhythm roadie 1.0.0 - stdin → staging.shop.test

Source: -
Target: -
Stack: WordPress + Action Scheduler + WooCommerce (HPOS)
Plan: 12 tables pseudonymised, 3 live keys, 2 side effects (plan -v for all)

[ok] Data: 2,515 values pseudonymised in 12 tables, 80 URLs rewritten
[ok] Keys: 0 swapped to a test key, 3 blanked
[ok] Side effects: gateways, webhooks (data rules only: a dump has no files)
[ok] Written: to stdout
...
DONE   written to stdout; not checked
0 critical · 0 high · 0 medium · 0 low · 0 info
166 KB · 13 tables · 0.0s
```

Verify the result afterwards with `rhythm roadie verify staging.sql` (or `verify -` to read it from stdin). Output that isn't a terminal is plain: no colour, boxes or glyphs, as `--plain` gives.

### Dump targets

A dump target is written compressed when its name ends `.gz` or `.zst` (`.sql.bz2` is read, not written). The file is written to a temporary name in the same folder, mode 0600. It is renamed only when complete, so a stopped copy never leaves a finished-looking file. A dump target is never `mixed`. `--to` can't be the input file (`output-is-input`, exit 2). Before the first byte Roadie checks the disk has room for the copy (`disk-full`, exit 3).

### Database and site targets

A site folder (WordPress or Laravel), `ssh://`, an `@alias` or a DSN target gets an import: Roadie drops and recreates each table the dump creates, through a pure-Go driver with `LOCAL INFILE` off (over ssh through `wp db import -` or `mysql`/`psql` with the credentials on stdin). A site target also gets the kill switch in its files. A DSN target has no files, so only the data rules apply and the result lists `side-effect.not-applicable`: turn mail and jobs off where the app runs.

The JSON report's `data.target_state` says what the target holds after the run:

| State | Means |
|---|---|
| `untouched` | Roadie stopped before the first write. The target is as it was. |
| `replaced` | Every table was written. |
| `mixed` | The import stopped after the first table: some tables are pseudonymised, the rest hold the earlier copy. The stop box says so and how to recover: run the same command again (copy), or restore the backup first (prepare). |

Ctrl-C before the import is `untouched` (exit 130). During the import Roadie stops after the current table and says `mixed`.

## Target safety

Every guard runs before anything is written and every refusal leaves the target untouched (exit 3):

| Code | When | Way past it |
|---|---|---|
| `target-is-production` | The target's URL host, `siteurl` or `home`, ssh host or database host matches `production:` (or `--production`). A DSN target's `siteurl` is read from every `*options` table. | None. |
| `target-is-source` | The same file, the same folder, the same ssh host and path or the same database host, port and name as the source. | None. |
| `target-says-production` | The target's `WP_ENVIRONMENT_TYPE` is unset or `production`, or `APP_ENV=production`. | `--target-is-staging <host>` naming the target's host, or set the variable to `staging` there. |
| `target-holds-production` | `prepare` only: the staging's `siteurl` is production's URL and nothing says which is live. | Answer the question, or pass `--production-url`. |

```text
target-is-production: /srv/staging is staging.shop.test, which production: lists. Roadie never writes to production. Point --to at the staging copy.
```

```text
target-is-source: --from and --to are the same folder (/srv/staging). A copy can't replace its own source.
```

Hosts are compared after normalising: case, a trailing dot, `www.`, punycode and Unicode forms of the same name, IPv4-mapped IPv6. `production:` takes hosts, `*.host` wildcards or URLs; Roadie also reads `production:` from `.echo/config.yaml` and `.soundcheck/config.yaml` in the project.

**Confirmation.** A site or database target needs a confirmation. On a terminal `copy` asks `Replace the database of /srv/staging? [y/N]`. Without a terminal, pass `--confirm-target` with the target's host (a site), database name (a DSN) or, for SQLite, the file's absolute path or name, plus `--yes`. Without it the copy stops with exit 2, a usage error:

```text
rhythm roadie 1.0.0 - shop.sql → staging.shop.test

Source: shop.sql
Target: /srv/staging

STOPPED   confirm-target; the target is untouched

confirm-target: there's no terminal to confirm on. Pass --confirm-target staging.shop.test (the site's host) to go on.

rhythm-roadie explain confirm-target
```

## Uncovered columns

Roadie only rewrites what a rule covers. In pass 1 it samples up to 1,000 values of every text column no rule covers. A column whose name looks personal (`email`, `phone`, `first_name`, `address`, `ip`, `dob`, `iban` and the like), or whose sampled values are 20% or more emails or phones, is an **uncovered column**: `copy` and `prepare` stop before writing anything. Here a CRM's config forgot `contacts.emergency_contact`:

```text
  ♪ rhythm roadie 1.0.0                              crm.sql → crm-staging.sql
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        crm.sql
  ▸ Target        crm-staging.sql

  UNCOVERED  1 ───────────────────────────────────────────────────────────────

 ▌  MEDIUM     contacts.emergency_contact looks personal (100% of 100 sample…
 ▌             → Add the column under columns: in .roadie/config.yaml
 ▌               (rhythm-roadie explain uncovered-column).

 ╭────────────────────────────────────────────────╮
 │  STOPPED   uncovered; the target is untouched  │
 ╰────────────────────────────────────────────────╯

  uncovered: 1 column looks personal and no rule covers it:
  contacts.emergency_contact. Add it under columns: (rhythm-roadie explain
  uncovered-column), or --allow-uncovered to copy and list it as a finding.
  The target is untouched.

  → rhythm-roadie explain uncovered-column
```

Exit 1. Add the column under `columns:` (here `rule: text`, which keeps the words and replaces the phone numbers and names in them) and run it again. `--allow-uncovered` copies anyway and lists each column as an `uncovered-column` finding (medium): with `--fail-on medium` that copy exits 1. `rhythm roadie config init --from <source>` writes a commented `columns:` entry for every column it would flag; see [Generic SQL and Postgres](Roadie-Generic-SQL-And-Postgres).

Exact mode is the second net. A Laravel app whose config covered `orders.shipping_address` but not `orders.shipping_name` passed the uncovered scan (the name doesn't look personal and the values aren't emails or phones). The copy's own verify still found every name in it, because they were names Roadie had seen and replaced in `users`:

```text
 ▌  CRITICAL   120 A production value Roadie replaced survived in the copy
 ▌             orders.shipping_name 120
 ▌             e.g. L•••, J•••, H•••
 ▌             found as raw
 ▌             → Add a rule for the column (rhythm-roadie explain
 ▌               exact.survivors), then copy again.
```

## URLs

Production's URL (from `siteurl` and `home`, the multisite `blogs` and `site` tables, `APP_URL` and the URLs in `production:`) is rewritten to `--url` in every text value, longest match first:

- `https://host`, `http://host` (to the staging scheme), `//host` and the `www.` form;
- JSON-escaped `https:\/\/host` and URL-encoded `https%3A%2F%2Fhost` (any hex case);
- inside PHP serialized values (every length fixed), JSON inside serialized and serialized inside JSON;
- bare hosts only in host columns (`blogs.domain`, `site.domain`).

The character after the host must not continue it, so `shop.com.au` and `myshop.com` are left alone. A host after `@` is an email domain and is never rewritten. `wc_subscriptions_siteurl` is never rewritten (it is how Subscriptions knows it is a copy). `--url-map a=b,...` (or `url_map:`) adds pairs for subsites, mapped domains and CDNs; see [Multisite](Roadie-Multisite).

## Prepare

`prepare` is for a staging that exists already. A clone made by WP STAGING still holds every customer and live key; `verify` shows it:

```text
  ♪ rhythm roadie 1.0.0                                  verify · /srv/staging
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          heuristic: this copy wasn't made by Roadie in this run
  ▸ Database      /srv/staging, 13 tables of 13, every row
  ▸ Production    shop.example.com
...
  CRITICAL  4 ────────────────────────────────────────────────────────────────

 ▌  CRITICAL   WP_ENVIRONMENT_TYPE isn't set, which WordPress reads as produ…
 ▌             wp-config.php 1
 ▌             → rhythm-roadie prepare /srv/staging --only side-effects
 ▌  CRITICAL   A live Stripe key                              key.stripe-live
 ▌             wp_options.option_value → serialized:secret_key 1
 ▌             e.g. sk_l•••••
 ▌  CRITICAL   Stripe is enabled and not in test mode    side-effect.gateways
 ▌             wp_options woocommerce_stripe_settings → testmode 1
 ▌             → rhythm-roadie prepare /srv/staging --only side-effects
 ▌  CRITICAL   No Echo connector and nothing blocks wp_mail  side-effect.mail
 ▌             wp-content/mu-plugins/rhythm-echo.php 1
 ▌             → rhythm-roadie prepare /srv/staging --only side-effects
...
 ╭─────────────────────────────────────────────────────╮
 │  FAIL   staging.shop.test is not a safe copy yet    │
 │  4 critical · 18 high · 15 medium · 0 low · 0 info  │
 ╰─────────────────────────────────────────────────────╯
...
  → rhythm-roadie prepare /srv/staging fixes what it can of these
```

`prepare` asks you to type the site's host, backs the database up, then runs the same pipeline as a copy with the staging as source and target:

```sh
rhythm roadie prepare /srv/staging --target-is-staging staging.shop.test
```

```text
  ♪ rhythm roadie 1.0.0                            prepare · staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Staging       /srv/staging
prepare rewrites this site's database while it runs. Visitors see a maintenance page for about a minute. Type the site's host to go on (staging.shop.test): staging.shop.test
  ▸ Stack         WordPress + Action Scheduler + WooCommerce (HPOS)
  ▸ Plan          12 tables pseudonymised, 3 live keys, 5 side effects (plan
                  -v for all)

  ✓ Data           2,515 values pseudonymised in 12 tables, 76 URLs
                   rewritten
  ✓ Keys           0 swapped to a test key, 3 blanked
  ✓ Side effects   cron, Echo seal, Action Scheduler, webhooks, gateways
  ✓ Search         noindex, robots.txt
  ✓ Backup         /home/ann/shop/.roadie/backups/2026-10-28T19-08-14.sql.gz;
                   to undo: gunzip -c
                   /home/ann/shop/.roadie/backups/2026-10-28T19-08-14.sql.gz
                   | wp --path=/srv/staging db import -
  ✓ Import         13 tables into /srv/staging
...
 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info   │
 ╰────────────────────────────────────────────────────╯
  117 KB · 13 tables · 0.6s
...
  Mail from this copy goes to Echo at http://127.0.0.1:8025. Run rhythm echo
  up on the staging host, or the copy's mail will fail closed.

  No user was kept, so nobody can log in with a real password. Set one: wp
  user update user_5588b3b064a7a206 --user_pass=<new password>
...
  → signed report: /home/ann/shop/.roadie/reports/2026-10-28T19-08-14Z-prepare-staging/report.html
  → next: rhythm echo up, then rhythm soundcheck run https://staging.shop.test
```

What `prepare` adds to a copy:

- **The question.** On a terminal it always asks for the host. Without one, `--confirm-target <host>` must name it (a DSN: the database name); `--yes` alone stops with `prepare-confirm` (exit 2), as does a wrong name:

  ```text
  prepare-confirm: --confirm-target shop.example.com doesn't name this target, which is staging.shop.test. Pass --confirm-target staging.shop.test (the site's host) to go on.
  ```

- **The backup.** Before any write, the database is exported to `.roadie/backups/<UTC time>.sql.gz` (0600) next to the config in use; an ssh target's backup goes to `~/.roadie-backups/` on that host; a SQLite database is copied to `.roadie/backups/<UTC time>.sqlite`. The screen and `data.error.restore` in the JSON give the command that loads it back. `--no-backup` skips it. A prepare that stops after its backup prints the backup and the restore command under the stop box.
- **The production URL.** A staging that still calls itself production's URL can't tell Roadie which URL is live: `prepare` asks on a terminal and otherwise stops with `target-holds-production` until `--production-url` or `production:` says.
- **Maintenance.** On WordPress, `.maintenance` is written for the import and removed after it: visitors see WordPress's maintenance page for about a minute.

### Only some parts

`--only` re-applies some parts without touching the rest. Parts: `data`, `keys`, `side-effects`, `urls`, `noindex` (default all). After a plugin update on staging, put the kill switch back:

```sh
rhythm roadie prepare /srv/staging --only side-effects
```

```text
  ▸ Stack         WordPress
  ▸ Plan          0 tables pseudonymised, 0 live keys, 5 side effects (plan
                  -v for all)

  ✓ Data           0 values pseudonymised in 0 tables, 0 URLs rewritten
  ✓ Keys           no live keys found
  ✓ Side effects   cron, Echo seal, Action Scheduler, webhooks, gateways
  ✓ Backup         /home/ann/shop/.roadie/backups/2026-10-28T19-08-21.sql.gz;
                   to undo: gunzip -c
                   /home/ann/shop/.roadie/backups/2026-10-28T19-08-21.sql.gz
                   | wp --path=/srv/staging db import -
  ✓ Import         2 tables into /srv/staging
...
 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info   │
 ╰────────────────────────────────────────────────────╯
  1.0 KB · 2 tables · 0.8s
```

Only the tables the kill switch needs (here `wp_options` and `wp_wc_webhooks`) are read and written. `--only noindex` reads no data, so it can't verify: its box reads `DONE   prepared staging.shop.test: noindex; not checked (--only noindex reads no data)`.

## Tuning a run

| Flag | Default | What it does |
|---|---|---|
| `--drop <tables>` | none | Leave these tables out (globs). Also `drop:` in the config. |
| `--truncate <tables>` | the profiles' list | Copy the structure, not the rows. Profiles truncate sessions, payment tokens, mail logs and caches already. |
| `--keep-routines` | off | Keep triggers, procedures, functions and views, with `DEFINER=CURRENT_USER`; each is listed under FILE EDITS. Without it a dump that holds one stops with `dump-refused`. |
| `--one-pass` | off | Skip the name index. Faster; names in free text that Roadie only learns from structured columns stay. |
| `--fix-collation` | off | Rewrite MySQL 8's `utf8mb4_0900_*` collations for an older MariaDB target. |
| `--jobs <n>` | CPUs, at most 8 | Transform workers. |
| `--max-memory <size>` | 300MiB, or `GOMEMLIMIT` | Soft memory limit. |
| `--max-value <size>` | 256MiB | Largest single value or statement; a bigger one stops the run (`value-too-large`). |
| `--max-output-ratio <n>` | 20 | Stop when a dump grows past n times its compressed size (`dump-bomb`). |
| `--schema <file>` | none | `CREATE TABLE` statements for a data-only dump. |
| `--no-verify` | off | Skip the verify. Exit 0 then means copied, not checked. |
| `--soundcheck` | off | Also run `rhythm-soundcheck run <url> --json` on the copy; Soundcheck's exit 1 becomes a `soundcheck.failed` finding. |

Every flag is in the [CLI reference](Roadie-CLI-Reference).

---
<sub>← [Getting started](Roadie-Getting-Started) · [Home](Home) · [Sources and targets](Roadie-Sources-And-Targets) →</sub>
