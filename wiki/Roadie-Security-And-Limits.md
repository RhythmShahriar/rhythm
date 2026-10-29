Roadie handles production's customer data and writes to servers, so this page says exactly what it promises, how each promise is kept, every network request it can make and what it can't do. Read it before you point Roadie at production or tell anyone a copy is safe.

## The promises

### Production is read-only

- A source is opened read-only in code: a dump file is only read; a database is read in a read-only transaction (`START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY` on MySQL and MariaDB, `REPEATABLE READ READ ONLY` on PostgreSQL); a local site's `wp-config.php` and `.env` are read as text, never run.
- Over ssh, only the read commands on [the list](Roadie-Sources-And-Targets#remote-commands) can run on a source, each built by one quoting function. A command outside the list can't be built. `plan` prints every command a run may use.
- `plan` writes nothing but its report; it is tested with a SELECT-only database user and a read-only file system.

### Production is never a target

Before anything is written, a target is refused when it matches `production:` (its URL host, `siteurl` and `home` read from every options table, its ssh host or its database host), when it is the source (the same file, folder, ssh path or database), or when it says it is production (`WP_ENVIRONMENT_TYPE` unset or `production`, `APP_ENV=production`). Only the last has a way past it, `--target-is-staging <host>`; that never overrides the first two. A site or database target needs its host or database name typed out (or `--confirm-target` in CI). See [Target safety](Roadie-Copy-And-Prepare#target-safety).

### No real data on your disk

- `copy` streams: the production rows pass through memory on their way to the target.
- A dump you pass is never rewritten in place. A target can't be the input file.
- A source that must be read twice (ssh, an alias, stdin, a live database) is spooled during the first pass to `~/.cache/rhythm/roadie/spool/` in 1 MiB AES-256-GCM chunks, with a key from `crypto/rand` held only in memory. The file is removed when the run ends, on success, error, Ctrl-C and SIGTERM. A SIGKILL can leave the file; without the key it is noise.
- Temporary files live in a 0700 folder; outputs, reports and backups are 0600.
- `prepare`'s backup is the exception, by design: it is the staging's own data (which was already on that machine), kept next to the config, or in `~/.roadie-backups/` on an ssh staging host.

### Every copy is verified

`copy` and `prepare` end with a verify that reads the written copy back, never what the engine believed it changed, in exact mode. A verify that read nothing can't pass (`nothing-scanned`, exit 3) and one that couldn't read all of its input never passes (`INCOMPLETE`, exit 3).

### Fail closed

- A column that looks personal with no rule stops the copy before it writes.
- A serialized value that won't parse and holds personal-looking text is blanked, or stops the copy (`on_unparseable: fail`); it is never passed through.
- An enabled payment gateway Roadie doesn't know is disabled.
- A dump that ends early stops the copy. A target stopped half way says it is `mixed`.
- Mail from a copy whose Echo isn't running is refused, not sent.

## Pseudonymised data is still personal data

Under the GDPR (Article 4(5), Recital 26) a pseudonymised copy is personal data. It keeps order totals, dates, products, each person's history and anything Roadie can't recognise in free text. Every copy and verify ends with:

```text
  Pseudonymised, not anonymised. The copy still holds order totals, dates,
  products and anything Roadie can't recognise in free text. Treat it as
  personal data.
```

Keep the copy, its backups and its reports as protected as production. Never push a staging database back to production: its customers are fake.

## Evidence, not a guarantee

A PASS means Roadie's checks found no real-looking person, live key, production URL or side effect still on, in what they read. It isn't a statement that the copy holds no personal data and it isn't a compliance certificate. A signed report proves who ran Roadie, which version and rules, on which input and that the report is unchanged since; it doesn't prove the checks covered everything. Each report says what it doesn't cover.

## What Roadie can't see

Not covered by any report:

- **files and uploads**: PDF invoices, form uploads, images with location data. Roadie copies no files; `plan` prints an `rsync` exclude list for the folders that usually hold personal data; `verify` lists backups, logs and form uploads it finds by name. For file-level leaks, run `rhythm tuner scan <staging dir>`.
- other databases, object caches (Redis, Memcached) and search indexes (Algolia, ElasticPress);
- logs on disk outside `wp-content`;
- copies held by third parties: Mailchimp lists, Stripe customers;
- names and street addresses written in free text that Roadie didn't see in a structured column;
- encrypted columns (Laravel's encrypted casts are listed, not read);
- tables you told Roadie to keep and the users you kept.

Theme and plugin code is scanned for live keys on local site targets only; over ssh Roadie reads only `wp-config.php` and `.env`.

## What the kill switch can't stop

A server crontab calling `wp cron` or `artisan schedule:run`, `wp action-scheduler run` by hand, queue workers already running, code that opens SMTP sockets or uses `fsockopen` with credentials Roadie doesn't know, raw cURL without `--echo-proxy`. Credentials are blanked so those paths fail at the provider. Blocking outbound traffic at the host is the only certainty. See [Kill switch](Roadie-Kill-Switch#what-no-switch-can-stop).

**The seal needs `WP_ENVIRONMENT_TYPE`.** Echo's connector and Roadie's mu-plugin do nothing on a site that says it is production. Roadie sets the variable to `staging`; if someone changes it back, the seal is off. `verify` reports it (`config.environment`, critical).

**Noindex is a request.** Search engines and crawlers may ignore it. Basic auth (`--basic-auth`) is protection; Roadie writes the snippets, you add them to the server.

## Keys and hashes

- **The consistency key is a re-identification key.** With `consistency: stable`, anyone with the key and a production email can compute that person's fake. Keep `ROADIE_KEY` or `.roadie/key` as secret as production's database password. The default, `per-run`, forgets the key when the run ends.
- **Password hashes are replaced** with ones nothing matches, for everyone not kept. Kept users keep their real hash unless `--admin-password-env` sets a new one.
- **Fakes can't reach anyone:** reserved email domains, fictional phone ranges, documentation IP ranges.
- **No value is printed.** Every finding, log line and report is masked through one function; a test seeds unique values into every field type, runs every command with every output format and `--debug` and fails if one appears anywhere. There is no flag that prints a raw value.
- **The signing key** (`~/.config/rhythm/roadie/signing.key` or `ROADIE_SIGNING_KEY`) never goes into a report; reports carry its public key and ID.

## Dumps are untrusted

A dump is code: piped into `mysql` it can run shell commands (`\!`), read and write files on the database server or create users. Roadie never pipes a dump into a database client. It parses each statement and re-emits only the ones on its allowlist; live imports go through pure-Go drivers with `LOCAL INFILE` off.

| | MySQL and MariaDB | PostgreSQL |
|---|---|---|
| **Kept** | `CREATE TABLE` with a known engine, `CREATE INDEX`, `CREATE SEQUENCE`, `DROP TABLE` or `DROP INDEX` of an object the dump creates, `LOCK` and `UNLOCK TABLES`, `ALTER TABLE` adding keys or changing columns and table options, `INSERT`, `REPLACE`, transactions, `DELIMITER`, `SET NAMES` and the session settings dump tools write | pg_dump's `SET` and `set_config()` settings, `CREATE TABLE`, `INDEX`, `SEQUENCE`, `TYPE`, `DOMAIN` and `SCHEMA`, common extensions, `ALTER TABLE` without `SET TABLESPACE`, `COMMENT ON`, `COPY ... FROM stdin`, transactions, `setval()` with literals |
| **Dropped, with a plan line** | `GRANT`, `REVOKE`, users and roles, `SET PASSWORD`, events, `ALTER DATABASE`, `USE` of the dump's own database, GTID and `SQL_LOG_BIN` settings | `ALTER ... OWNER TO`, `SET SESSION AUTHORIZATION`, publications |
| **Refused** (exit 3, `dump-refused`) | client commands (`\!`, `source`), `USE` of another database, `CREATE` or `DROP DATABASE`, `LOAD DATA`, `INTO OUTFILE`, `LOAD_FILE()`, `CALL`, `UPDATE`, `DELETE`, `INSERT ... SELECT`, `SET GLOBAL`, a multibyte client character set (SJIS, BIG5, GBK and the like, by name, number or `DEFAULT`), `FEDERATED` or `CONNECT` tables, names that carry their database | `\connect` and other psql commands, `COPY ... PROGRAM` or to a file, `DO` blocks, server file functions (`pg_read_file()`, `lo_import()`, `dblink`), Unicode-escaped names and strings, foreign servers, subscriptions |
| **With `--keep-routines`** | triggers, procedures, functions and views kept, `DEFINER` replaced by `CURRENT_USER`, each listed | the same; untrusted languages refused even then |

```text
dump-refused: trigger.sql line 487 holds a CREATE TRIGGER statement, which could run code on the staging database. Export with --skip-triggers, or pass --keep-routines to keep it without its DEFINER.
```

The message names the line and the kind of statement, never its data. String literals in `CREATE TABLE` and `ALTER TABLE` (a column's default, a comment) are scrubbed like row values. Size limits stop runaway input: a value or statement over `--max-value` (256 MiB) stops with `value-too-large`; a compressed dump that grows past `--max-output-ratio` (20 times) stops with `dump-bomb`. Archives (`.wpress`, zip, tar) are refused by their bytes, never unpacked. Serialized PHP objects are kept as bytes and never instantiated. No PHP or `wp` process runs on a local file; Roadie never starts PHP or a web server.

## Every network request Roadie can make

| To | When |
|---|---|
| The source and target you name | Database connections; `ssh` to the hosts you give (your own `ssh`, your `known_hosts`, never a password). |
| The staging URL in `verify --site` | `GET /robots.txt` and the home page's headers, never a production host, redirects only within that host, 2 MiB and 10 seconds at most; `--insecure-tls <host>` accepts a self-signed certificate on that one host only. |
| Echo, at the URL you set | `verify --echo` reads Echo's health and its sent count. An Echo URL must be `https`, loopback or a private address. The installed connector talks to the same Echo from the site, not from Roadie. |
| `api.github.com` and GitHub's download hosts | The update check after a run on a terminal (off with `--offline`, `RHYTHM_OFFLINE=1`, `CI`, `--json` or `-q`) and `rhythm roadie update`, which downloads the signed profile pack. |
| Sigstore's trust root (`tuf-repo-cdn.sigstore.dev`) | `update`, to check the profile pack's signature, as for the binary. |
| Soundcheck | `copy --soundcheck` starts `rhythm-soundcheck run <url>`, which fetches the staging URL. |

Nothing else: no telemetry, no DNS or MX lookups of the data, no third-party API. A test runs `copy` and `verify` with a network dialer and resolver that fail on any other address.

## Reports are internal

Reports hold no personal value. They do name production's host, tables, columns, counts, masked examples and the people Roadie kept. Treat them as internal: the GitHub Action keeps them out of the job summary doesn't upload them unless you ask and warns in a public repository.

## Licence

Roadie's own code is MIT. The `rhythm-roadie` binary also includes the [rhythm kit](https://github.com/RhythmShahriar/rhythm/tree/master/kit), which is AGPL-3.0-only. Echo's WordPress connector inside it is MIT.

## Responsible use

Use Roadie only on sites you own or are authorised to copy: it reads production data to pseudonymise it. A vulnerability in Roadie itself? Report it privately: https://github.com/RhythmShahriar/roadie/security/advisories/new. See [Security policy](Security-Policy) and [Responsible use](Responsible-Use).

---
<sub>← [Configuration](Roadie-Configuration) · [Home](Home) · [CLI reference](Roadie-CLI-Reference) →</sub>
