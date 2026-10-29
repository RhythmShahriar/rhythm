Every error code Roadie stops with, what it means and how to fix it. Each one is also offline in the binary: `rhythm roadie explain <code>`. For questions about what Roadie does, see the [FAQ](Roadie-FAQ).

## Reading a stop

A run that can't go on prints a box with the error code and what it left the target as, then the message and the `explain` command:

```text
 ╭─────────────────────────────────────────────────────────────╮
 │  STOPPED   target-says-production; the target is untouched  │
 ╰─────────────────────────────────────────────────────────────╯

  target-says-production: /srv/staging has WP_ENVIRONMENT_TYPE unset, which
  WordPress reads as production. If it's staging, set WP_ENVIRONMENT_TYPE to
  staging there, or pass --target-is-staging staging.shop.test.

  → rhythm-roadie explain target-says-production
```

- **the target is untouched**: Roadie stopped before its first write. Fix the cause and run the same command.
- **the target is in a mixed state**: something was written. A copy: run the same command again once the cause is fixed. A `prepare`: restore the backup the stop names first (the command is under the box and in `data.error.restore`), then run it again; a new `prepare` backs up the database as it is now, not as it was.

The message names a line, a table or a kind of statement, never a value. With `--json` the same is in `data.error` (`code`, `message`, `exit`) and `verdict` is `error`. Exit codes: 1 findings, 2 usage, 3 couldn't run, 130 interrupted ([CLI reference](Roadie-CLI-Reference#exit-codes)).

## Every code

| Code | Exit | What happened |
|---|---|---|
| [`config`](#config) | 2 | a problem in .roadie/config.yaml or on the command line |
| [`not-found`](#not-found) | 2 | a path the command names doesn't exist |
| [`output-is-input`](#output-is-input) | 2 | the output is the input |
| [`stack`](#stack) | 2 | no profile matches |
| [`url-map`](#url-map) | 2 | a host has no staging URL |
| [`test-key-is-live`](#test-key-is-live) | 2 | a test key is live |
| [`wp-alias`](#wp-alias) | 2 | a WP-CLI alias Roadie can't use |
| [`confirm-target`](#confirm-target) | 2 | copy needs the target confirmed |
| [`prepare-confirm`](#prepare-confirm) | 2 | prepare needs the target confirmed |
| [`dump-format`](#dump-format) | 2 | a dump format Roadie doesn't read |
| [`dump-truncated`](#dump-truncated) | 3 | the dump ends early |
| [`dump-refused`](#dump-refused) | 3 | a statement in the dump could run code |
| [`dump-bomb`](#dump-bomb) | 3 | the dump grew too much while reading |
| [`value-too-large`](#value-too-large) | 3 | a value is too large |
| [`dialect-mismatch`](#dialect-mismatch) | 2 | source and target speak different SQL |
| [`disk-full`](#disk-full) | 3 | not enough free disk space |
| [`unreachable`](#unreachable) | 3 | a database or host doesn't answer |
| [`site-config`](#site-config) | 3 | the site's config can't be read as text |
| [`ssh`](#ssh) | 3 | ssh failed |
| [`ssh-host-key`](#ssh-host-key) | 3 | the host key isn't known |
| [`wp-cli`](#wp-cli) | 3 | WP-CLI isn't there |
| [`lock`](#lock) | 3 | another run holds the target |
| [`target-is-production`](#target-is-production) | 3 | the target is production |
| [`target-is-source`](#target-is-source) | 3 | the target is the source |
| [`target-says-production`](#target-says-production) | 3 | the target says it is production |
| [`target-holds-production`](#target-holds-production) | 3 | the staging may still call itself production |
| [`uncovered`](#uncovered) | 1 | the copy stopped on uncovered columns |
| [`import`](#import) | 3 | the import failed |
| [`import-mixed`](#import-mixed) | 3 | the import stopped half way |
| [`kill-switch`](#kill-switch) | 1 | a kill switch couldn't be installed |
| [`key-manual`](#key-manual) | 1 | a live key sits in code |
| [`fake-collision`](#fake-collision) | 3 | two emails got the same fake |
| [`interrupted`](#interrupted) | 130 | the run was stopped by Ctrl-C or a signal |
| [`nothing-scanned`](#nothing-scanned) | 3 | verify read nothing |
| [`echo-unreachable`](#echo-unreachable) | 1 | Echo isn't reachable |
| [`report-edited`](#report-edited) | 1 | the report was changed after signing |
| [`report-unsigned`](#report-unsigned) | 3 | the report has no signature |
| [`not-built`](#not-built) | 3 | not built yet |

## The command line and the config

### config

A problem in .roadie/config.yaml or on the command line. Exit 2.

```text
rhythm-roadie: bad1.yaml line 2: unknown key pseudonymise.emial_domain; did you mean pseudonymise.email_domain?
rhythm-roadie: bad2.yaml line 2: keys.test_keys.stripe reads ${STRIPE_TEST}; only ROADIE_ variables are read. Rename it ROADIE_STRIPE_TEST.
rhythm-roadie: bad3.yaml line 2: pseudonymise.email_domain is "mycompany.com", which can receive real mail. Use a reserved domain: example.invalid, example.com or a name under .test, .example, .invalid or .localhost.
rhythm-roadie: bad4.yaml line 1: drop[0] is "wp_[x", which isn't a valid glob (use *, ? and [abc] as in path.Match)
```

An unknown key, a wrong type, a bad glob, a non-ROADIE_ variable or a non-reserved email domain in the config; or a flag or a source or target the command can't take, such as a missing --from or a dump file given to prepare. Roadie reads its config and flags strictly so a typo never turns a rule off. Nothing was read or written; the target is untouched.

**Fix:** Fix the line or the flag the message names; rhythm-roadie config check validates the file.

### not-found

A path the command names doesn't exist. Exit 2.

```text
not-found: nope.sql doesn't exist. Check the path you gave.
```

The dump file, site folder or SQLite database given as the source, the target or the copy to verify isn't there. Roadie stops before it reads or writes anything; the target is untouched.

**Fix:** Check the path for a typo and that it is relative to the folder you run Roadie from, or give it in full. Roadie never creates a SQLite database: for a new SQLite target create the empty file first (`touch <path>`):

```text
not-found: the SQLite file /srv/app/staging.sqlite doesn't exist. Roadie never creates one: create it (touch /srv/app/staging.sqlite) or check the path.
```

### output-is-input

The output is the input. Exit 2.

```text
output-is-input: --to shop.sql is the file Roadie reads. Write the copy to a new file.
```

A dump-file target is the input file or a hard link to it. Roadie never rewrites a dump in place.

**Fix:** Write the copy to a new file.

### stack

No profile matches. Exit 2.

```text
stack: no profile matches this dump (no wp_options or migrations table). For your own schema, list the tables under columns: and set stack: sql.
```

The dump has no wp_options or migrations table and no columns: config. Without a profile Roadie doesn't know what to change.

**Fix:** List your tables under columns: and set stack: sql.

### url-map

A host has no staging URL. Exit 2.

```text
url-map: gifts.example.net has no staging URL; add --url-map gifts.example.net=gifts.staging.shop.test.
```

A multisite subsite or mapped domain has no url_map entry. Roadie never guesses a staging host.

**Fix:** Add --url-map <host>=<staging host>.

### test-key-is-live

A test key is live. Exit 2.

```text
rhythm-roadie: .roadie/config.yaml line 46: test-key-is-live: keys.test_keys.stripe reads a live key (sk_l•••••). Give a test key (sk_test_...) or remove the entry to blank it.
```

A keys.test_keys value classifies as live. Swapping a live key for another live key would keep the copy live.

**Fix:** Give a test key (sk_test_...) or remove the entry to blank it.

### wp-alias

A WP-CLI alias Roadie can't use. Exit 2.

```text
wp-alias: WP-CLI alias not found: no wp-cli.yml or wp-cli.local.yml in /home/ann/shop or above. Add @nope to wp-cli.yml with ssh: user@host/path (or path: for a local site), or name the site as ssh://user@host/path.
```

The site was named as a WP-CLI alias (@prod) and Roadie can't turn it into a site: no wp-cli.yml or wp-cli.local.yml from the working folder up holds it, or its ssh value is not user@host[:port]/path (a docker, vagrant or other non-ssh alias), or it names no absolute site folder. Roadie opens an alias only as an ssh site or a local folder. Nothing was read or written; the target is untouched.

**Fix:** Add the alias to wp-cli.yml in the project folder, such as

### confirm-target

Copy needs the target confirmed. Exit 2.

```text
confirm-target: there's no terminal to confirm on. Pass --confirm-target staging.shop.test (the site's host) to go on.
```

copy ran without a terminal and without --confirm-target, the name given to --confirm-target doesn't name the target, or the replace question was answered no. copy replaces the target's database, so a run in CI or a script must say which target it means.

**Fix:** Pass --confirm-target <name> (the site's host or the database name the message gives; for a SQLite target its absolute path or the file's name). On a terminal, answer y to the replace question or pass --yes.

### prepare-confirm

Prepare needs the target confirmed. Exit 2.

```text
prepare-confirm: there's no terminal to confirm on. Pass --confirm-target staging.shop.test (the site's host) to go on.
prepare-confirm: --confirm-target shop.example.com doesn't name this target, which is staging.shop.test. Pass --confirm-target staging.shop.test (the site's host) to go on.
```

prepare ran without a terminal and without --confirm-target, or the host typed didn't match. prepare rewrites the database a site is running on.

**Fix:** Pass --confirm-target <host> (the site's host or the database name).

## Dumps

### dump-format

A dump format Roadie doesn't read. Exit 2.

```text
dump-format: prod.dump is a pg_dump custom-format archive. Roadie reads plain SQL: pg_dump -Fp, mysqldump, mariadb-dump or wp db export.
```

The input is a pg_dump custom or directory archive, a .wpress or Duplicator file, a zip or tar archive, a mydumper folder or a dump saved as UTF-16 (PowerShell's > redirect writes that). A UTF-8 dump with a byte order mark is fine. As a target, it is a --to name Roadie can read but not write: a .sql.bz2 copy. Roadie reads plain SQL in UTF-8 only and never opens archives.

**Fix:** Export with pg_dump -Fp, mysqldump, mariadb-dump or wp db export. On Windows write the dump with mysqldump --result-file or pg_dump -f, not with >. Name a copy .sql, .sql.gz or .sql.zst.

### dump-truncated

The dump ends early. Exit 3.

```text
dump-truncated: cut.sql ends inside table wp_usermeta at 60.0 KB, with no "Dump completed" line. Export it again; the target is untouched.
```

The dump stops inside a table or has no completion line. A truncated dump would make a copy with missing rows.

**Fix:** Export the database again; the target is untouched.

### dump-refused

A statement in the dump could run code. Exit 3.

```text
dump-refused: trigger.sql line 487 holds a CREATE TRIGGER statement, which could run code on the staging database. Export with --skip-triggers, or pass --keep-routines to keep it without its DEFINER.
```

The dump holds a statement Roadie never copies: a client command, USE of another database, CREATE DATABASE, LOAD DATA, INTO OUTFILE, a trigger, procedure, function or view (without --keep-routines) or a statement that is on no list below. The message names the line and the kind, never its data. Dumps are code: these statements could run on the staging database, read or write files on its server or reach another server.

**Fix:** Export with --skip-triggers and without --routines, one database at a time (no --databases or --all-databases), with --default-character-set=utf8mb4 or pg_dump -E UTF8, or pass --keep-routines to keep routines without their DEFINER. Leave anything else the message names out of the export.

### dump-bomb

The dump grew too much while reading. Exit 3.

The decompressed input grew past 20 times its compressed size (or --max-output-ratio). That is what a compression bomb does.

**Fix:** If the dump is real, pass --max-output-ratio 50 to copy, prepare, plan or verify. A copy that stops here leaves the target untouched.

### value-too-large

A value is too large. Exit 3.

```text
value-too-large: a statement at line 118 is over 1 KiB. Pass --max-value 4KiB if it's real; the target is untouched.
```

A single value is over --max-value (256 MiB by default). Roadie holds one value in memory at a time; the cap keeps memory flat.

**Fix:** Pass --max-value 1GiB to copy, prepare, plan or verify if the value is real.

### dialect-mismatch

Source and target speak different SQL. Exit 2.

```text
dialect-mismatch: prod.sql is a PostgreSQL dump and --to is a MySQL database. Roadie copies MySQL to MySQL or MariaDB, PostgreSQL to PostgreSQL and SQLite to SQLite.
```

A MySQL dump into PostgreSQL or SQLite, a PostgreSQL dump into MySQL, or a SQLite database into MySQL or PostgreSQL: any copy between two kinds. Roadie copies MySQL to MySQL or MariaDB, PostgreSQL to PostgreSQL and SQLite to SQLite. It stops before anything is written.

**Fix:** Pick a target of the same kind.

### disk-full

Not enough free disk space. Exit 3.

The disk that holds the copy or the spool has less free space than the copy needs. Roadie checks free space before and while it writes, so a full disk never leaves a finished-looking copy.

**Fix:** Free space or write the copy to another disk; the target is untouched.

## Reaching the source and target

### unreachable

A database or host doesn't answer. Exit 3.

```text
unreachable: the database of mysql://staging@127.0.0.1:39009/shop: dial tcp 127.0.0.1:39009: connect: connection refused. Check that the database is running and reachable from here; then check the host and port in the DSN.
```

For a site target the last words are `then check DB_HOST.`: the host comes from its `wp-config.php` or `.env`.

Roadie couldn't connect to the database of a DSN or a site's config: the connection was refused, timed out or the host name didn't resolve. Without a connection nothing can be read or written; the target is untouched unless the message says otherwise.

**Fix:** Check that the database is running and reachable from this machine (a firewall, a VPN, an SSH tunnel), then the host and port in the DSN or the site's config (`DB_HOST` in `wp-config.php` or `.env`).

### site-config

The site's config can't be read as text. Exit 3.

```text
site-config: /srv/envsite/wp-config.php sets DB_PASSWORD from getenv( 'DB_PASS' ), which Roadie can't run. Pass the source as a DSN with ROADIE_FROM_PASSWORD, or use @prod through WP-CLI.
```

wp-config.php or .env sets the database credentials from getenv(), $_ENV or an expression. Roadie reads config files as text and never runs PHP.

**Fix:** Pass the database as a DSN with ROADIE_TO_PASSWORD, or use a WP-CLI alias.

### ssh

Ssh failed. Exit 3.

The system ssh refused the key, the host or the command. Roadie uses your ssh setup and never skips host key checks.

**Fix:** Check ssh <host> works on its own.

### ssh-host-key

The host key isn't known. Exit 3.

```text
ssh-host-key: shop.example.com isn't in known_hosts. Connect once with ssh to check and add it; Roadie never skips host key checks.
```

The host isn't in known_hosts. Roadie never turns host key checking off.

**Fix:** Connect once with ssh to check the key and add it.

### wp-cli

WP-CLI isn't there. Exit 3.

wp isn't on the remote host's PATH. Remote exports and imports go through WP-CLI.

**Fix:** Install WP-CLI there, set remote.wp to its path, or export the database yourself and use --from dump.sql.gz.

### lock

Another run holds the target. Exit 3.

Another Roadie run is copying to the same target. Two runs on one target would mix their results.

**Fix:** Wait for it, or remove the lock file named in the message if that run crashed.

## Target guards

### target-is-production

The target is production. Exit 3.

```text
target-is-production: /srv/staging is staging.shop.test, which production: lists. Roadie never writes to production. Point --to at the staging copy.
```

The target's URL host, siteurl, home, SSH host or database host matches production:. Roadie never writes to production; there is no override.

**Fix:** Point --to at the staging copy.

### target-is-source

The target is the source. Exit 3.

```text
target-is-source: --from and --to are the same folder (/srv/staging). A copy can't replace its own source.
```

--from and --to reach the same file, the same SSH host and path or the same database. A copy can't replace its own source.

**Fix:** Point --to somewhere else.

### target-says-production

The target says it is production. Exit 3.

```text
target-says-production: /srv/staging has WP_ENVIRONMENT_TYPE unset, which
WordPress reads as production. If it's staging, set WP_ENVIRONMENT_TYPE to
staging there, or pass --target-is-staging staging.shop.test.
```

The target's WP_ENVIRONMENT_TYPE is unset or production, or APP_ENV=production. WordPress reads an unset environment as production, so Roadie can't tell this target apart from production.

**Fix:** Set WP_ENVIRONMENT_TYPE to staging on the target, or pass --target-is-staging <host> naming its host.

### target-holds-production

The staging may still call itself production. Exit 3.

```text
target-holds-production: /srv/clone calls itself https://shop.example.com; Roadie can't tell whether that is the live site. Pass --production-url <production URL> (the live site's URL) so Roadie knows which URL is live, or --url https://shop.example.com if it is this staging's own URL.
```

prepare found a siteurl on the staging and nothing says whether it is the live site's URL: no --production-url, no production: and no --url that matches it. Rewriting URLs needs to know which one is live.

**Fix:** Pass --production-url <live URL> (never the staging's own URL), or --url <siteurl> when the siteurl is the staging's own URL already. On a terminal Roadie asks instead.

## Copy and import

### uncovered

The copy stopped on uncovered columns. Exit 1.

```text
uncovered: 1 column looks personal and no rule covers it:
contacts.emergency_contact. Add it under columns: (rhythm-roadie explain
uncovered-column), or --allow-uncovered to copy and list it as a finding.
The target is untouched.
```

Columns that look personal have no rule. Roadie fails closed: it doesn't copy data it can't pseudonymise.

**Fix:** Add them under columns: (rhythm-roadie explain uncovered-column), or pass --allow-uncovered to copy and list them as findings.

### import

The import failed. Exit 3.

```text
import: MariaDB 10.11 doesn't know collation utf8mb4_0900_ai_ci (MySQL 8). Run again with --fix-collation. The database is untouched but the kill switch already changed the site's files, so the target is in a mixed state.
```

The target database refused the import, for example a MySQL 8 collation on an older MariaDB, or a table the input fills that the target doesn't have. Nothing is replaced when the import fails before the first table.

**Fix:** Run again with --fix-collation, or fix what the message names. A copy from a live PostgreSQL database brings rows only: create the schema on the target first (run the app's migrations there or load pg_dump --schema-only).

### import-mixed

The import stopped half way. Exit 3.

The import stopped after the first table was replaced: the connection was lost or the server refused a later table. Some tables are pseudonymised, the rest still hold the earlier copy.

**Fix:** For copy, fix the cause and run the same command again. For prepare, restore the backup the stop names first (the command is in the message and in data.error.restore), then run prepare again: a new prepare backs up the mixed database, not the one you had.

### kill-switch

A kill switch couldn't be installed. Exit 1.

Roadie couldn't write one of its files, for example the Echo connector. That side effect stays on.

**Fix:** Fix the folder's owner, then run rhythm-roadie prepare <staging> --only side-effects.

### key-manual

A live key sits in code. Exit 1.

```text
 ▌  CRITICAL   A live Stripe key                              key.stripe-live
 ▌             wp-content/themes/shop/functions.php line 3 1
 ▌             e.g. sk_l•••••
 ▌             → key.stripe-live: wp-content/themes/shop/functions.php line 3
 ▌               holds a live Stripe key. Roadie only edits the database,
 ▌               wp-config.php and .env. Move it into wp-config.php or blank
 ▌               it by hand, then run verify.
```

A live key is in a theme or plugin file. Roadie only edits the database, wp-config.php and .env; it never edits code.

**Fix:** Move the key into wp-config.php or blank it by hand, then run verify.

### fake-collision

Two emails got the same fake. Exit 3.

Two different emails mapped to the same fake in one run. Fakes must stay unique so joins and logins keep working.

**Fix:** Run again; a new key fixes it.

### interrupted

The run was stopped by Ctrl-C or a signal. Exit 130.

The run got Ctrl-C or a signal (SIGINT or SIGTERM) and stopped before it finished. The stop box says the state the target was left in. An untouched target is as it was before the run; a target in a mixed state holds part of the copy and shouldn't be used as it is.

**Fix:** Run the command again. If the box says the target is in a mixed state, restore the backup the run named or run the copy again before you use the target.

## Verify and reports

### nothing-scanned

Verify read nothing. Exit 3.

```text
nothing-scanned: verify read 0 rows from schema-only.sql (13 tables, all empty). A verify that sees nothing can't pass; the dump holds no tables or no rows. Check that the export wrote the database you meant.
```

verify read no table or no row. A verify that sees nothing can't pass.

**Fix:** Check the prefix, the dump and the user's SELECT rights.

### echo-unreachable

Echo isn't reachable. Exit 1.

```text
 ▌  CRITICAL   verify --echo: Echo isn't reachable at http://127.0.0.1:39027…
 ▌             http://127.0.0.1:39027 1
 ▌             → Start Echo (rhythm echo up), then run verify again.
```

verify --echo couldn't reach Echo. Without Echo the copy's mail fails closed or goes out, depending on the seal.

**Fix:** Start Echo with rhythm echo up on the staging host.

### report-edited

The report was changed after signing. Exit 1.

```text
rhythm-roadie report check: edited.json was changed after it was signed (summary.high 16 → 0).
```

report check found a signature that doesn't match the report. The report no longer shows what Roadie found.

**Fix:** Use the report as Roadie wrote it, or run verify again.

### report-unsigned

The report has no signature. Exit 3.

```text
rhythm-roadie report check: unsigned.json has no signature (it was written with --sign=off).
```

The report was written with --sign=off. An unsigned report can't be checked.

**Fix:** Run verify again without --sign=off.

### not-built

Not built yet. Exit 3.

This part of Roadie isn't in this build. It is a development build.

**Fix:** Use a released version of Roadie.

## Other problems

### The copy is slow over ssh

Each remote command is its own ssh connection; a plan or copy runs one per plugin check. Add `ControlMaster auto`, a `ControlPath` and `ControlPersist 60` for the host in `~/.ssh/config` so they share one. On a host whose `find` is BusyBox's (Alpine), the plugin checks fail and are tried again; install GNU find there.

### Mail from the copy doesn't arrive anywhere

That is the seal working: the Echo connector refuses to send while it can't reach Echo. Run `rhythm echo up` on the staging host, or point the connector at your Echo with `--echo-url` and run `prepare <site> --only side-effects`. `verify --echo` checks Echo's seal and counts what it let through.

### Nobody can log in to the copy

Every password was replaced. Set one for the fake administrator the copy's last screen names (`wp user update <login> --user_pass=...`), or copy again with `--keep-users <your login>`.

### verify flags our own business address or phone

List it: `verify.allow_emails: ["info@shop.com"]` (or `--allow-emails`), `allow_phones` for numbers. To accept findings you keep on purpose, `verify.ignore` entries or a `--baseline` report.

### A plugin's table isn't pseudonymised

If Roadie has no profile for the plugin, its columns are either found by the uncovered scan (and the copy stops) or caught by exact mode as survivors. Add the columns under `columns:`; `rhythm roadie config init --from <source>` lists candidates.

### The copy fails its own verify

The finding names the table and column. An `exact.survivors` finding means a production value is still in a column no rule covers: add a rule. A side-effect finding on a site target usually means a file couldn't be written (`kill-switch`); fix the folder's owner and run `prepare <site> --only side-effects`.

---
<sub>← [CLI reference](Roadie-CLI-Reference) · [Home](Home) · [FAQ](Roadie-FAQ) →</sub>
