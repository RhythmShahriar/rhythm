For a schema Roadie has no profile for (your own app, a CRM, a Next.js app with Prisma on PostgreSQL), `stack: sql` copies exactly what you tell it to in `columns:` and stops on any column that looks personal and has no rule. This page covers writing those rules, letting `config init --from` suggest them and what differs on PostgreSQL and SQLite.

## 1. Let Roadie suggest the columns

```sh
cd ~/crm2
rhythm roadie config init --stack sql --from crm.sql
```

```text
Wrote .roadie/config.yaml for your own SQL schema.
Listed 7 columns that look personal under columns: as suggestions; check each one.
Set source:, target: and url: in it, then run rhythm-roadie plan.
```

`--from` reads the source's schema and samples its columns, read-only. Every column it would flag is written as a commented suggestion:

```yaml
columns:                           # your own tables; nothing else is changed for stack sql
# Suggested by config init --from: these columns look personal and no profile covers them.
# Check each rule, then uncomment it (and set columns: to the list).
#  - table: contacts
#    column: email
#    rule: email
#  - table: contacts
#    column: emergency_contact
#    rule: blank
#  - table: contacts
#    column: full_name
#    rule: full_name
#  - table: contacts
#    column: notes
#    rule: blank
#  - table: contacts
#    column: phone
#    rule: phone
#  - table: deals
#    column: contact_email
#    rule: email
#  - table: tickets
#    column: requester_email
#    rule: email
```

Check each one before you uncomment it: `notes` is suggested as `blank`; `text` keeps the notes readable and replaces the personal values in them. Below the suggestions the template keeps two example entries for a `customers` table; delete them if you have no such table.

## 2. The rules

```yaml
stack: sql
production: ["crm.example.com"]
columns:
  - table: contacts
    column: email
    rule: email
  - table: contacts
    column: full_name
    rule: full_name
  - table: contacts
    column: phone
    rule: phone
  - table: contacts
    column: notes
    rule: text
  - table: contacts
    column: emergency_contact
    rule: text
  - table: deals
    column: contact_email
    rule: email
  - table: tickets
    column: requester_email
    rule: email
  - table: tickets
    column: body
    rule: text
```

| Rule | Result |
|---|---|
| `email`, `phone`, `name`, `first_name`, `last_name`, `full_name`, `address`, `street`, `city`, `postcode`, `ip`, `dob`, `company`, `username`, `url`, `user_agent` | a consistent fake of that kind ([formats](Roadie-Pseudonymisation#what-the-fakes-look-like)) |
| `text` | the text stays; emails, phones, cards, IPs and known names and streets in it are replaced |
| `blank`, `null` | emptied, or NULL |
| `keep` | left as it is (overrides a profile's rule) |
| `fixed:<value>` | set to that value |
| `json:<path>=<rule>` | a rule for one value inside a JSON column, e.g. `json:$.email=email` |
| `serialized:<key>=<rule>` | a rule for one key inside a PHP serialized value |

`where: {column: value}` limits a rule to matching rows (globs allowed), for key-value tables: `{table: wp_postmeta, where: {meta_key: _billing_vat_number}, column: meta_value, rule: blank}`. One person gets one fake across every column, so a join on an email (`contacts.email` to `deals.contact_email`) still joins after the copy.

## 3. Copy

A column the config forgot stops the copy before anything is written; [Copy and prepare](Roadie-Copy-And-Prepare#uncovered-columns) shows it for `contacts.emergency_contact`. With every column covered:

```text
  ♪ rhythm roadie 1.0.0                              crm.sql → crm-staging.sql
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        crm.sql
  ▸ Target        crm-staging.sql
  ▸ Stack         SQL
  ▸ Plan          3 tables pseudonymised, 0 live keys, 0 side effects (plan
                  -v for all)

  ✓ Data           1,000 values pseudonymised in 3 tables, 0 URLs rewritten
  ✓ Keys           no live keys found
  ✓ Written        crm-staging.sql (45.6 KB)
...
 │  PASS   crm-staging.sql is a pseudonymised copy   │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info  │
 ╰───────────────────────────────────────────────────╯
  45.5 KB · 3 tables · 0.1s
...
  → signed report: /home/ann/crm/.roadie/reports/2026-10-28T19-04-33Z-copy-crm-staging.sql/report.html
  → next: load crm-staging.sql into the staging database, or copy straight to it: rhythm-roadie copy --from crm.sql --to <staging site or DSN>
```

`stack: sql` needs no `--url`. Live keys are still looked for in every value and swapped or blanked. There is no kill switch: a database has no config files, so turn mail and jobs off where the app runs.

## PostgreSQL

Roadie reads `pg_dump -Fp` (plain SQL, with `COPY ... FROM stdin` or `INSERT`) and writes PostgreSQL back as PostgreSQL. A Prisma schema keeps its quoted, mixed-case names; name tables in `columns:` as the schema has them (`User`, not `user`):

```yaml
stack: sql
production: ["app.shop.example.com"]
columns:
  - table: User
    column: email
    rule: email
  - table: User
    column: name
    rule: full_name
  - table: User
    column: phone
    rule: phone
  - table: Address
    column: line1
    rule: street
  - table: Address
    column: city
    rule: city
  - table: Session
    column: ipAddress
    rule: ip
  - table: Session
    column: data
    rule: json:$.email=email
  - table: Order
    column: note
    rule: text
```

To a compressed dump:

```text
  ♪ rhythm roadie 1.0.0                              prod.sql → staging.sql.gz
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        prod.sql
  ▸ Target        staging.sql.gz
  ▸ Stack         SQL
  ▸ Plan          4 tables pseudonymised, 0 live keys, 0 side effects (plan
                  -v for all)

  ✓ Data           540 values pseudonymised in 4 tables, 0 URLs rewritten
  ✓ Keys           no live keys found
  ✓ Written        staging.sql.gz (5.7 KB)
...
 ╭───────────────────────────────────────────────────╮
 │  PASS   staging.sql.gz is a pseudonymised copy    │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info  │
 ╰───────────────────────────────────────────────────╯
```

The rows keep pg_dump's layout, so `psql -f` or `gunzip -c staging.sql.gz | psql` loads them:

```text
COPY public."User" (id, email, name, phone, "createdAt") FROM stdin;
usr_000001	u841cc5d3d9ede804@example.invalid	Gareth Newman	+1 555 019 6351	2026-05-01 10:00:00
usr_000002	ucad30c4206ff47ac@example.invalid	Hilary Flores	+1 555 011 0290	2026-05-01 10:00:00
```

Or straight into a PostgreSQL database (password in `ROADIE_TO_PASSWORD`):

```sh
rhythm roadie copy --from prod.sql --to 'postgres://app@127.0.0.1:39021/app?sslmode=disable' --confirm-target app --yes
```

```text
  ▸ Source        prod.sql
  ▸ Target        postgres://app@127.0.0.1:39021/app?sslmode=disable
  ▸ Stack         SQL
  ▸ Plan          4 tables pseudonymised, 0 live keys, 1 side effect (plan
                  -v for all)

  ✓ Data           540 values pseudonymised in 4 tables, 0 URLs rewritten
  ✓ Keys           no live keys found
  ✗ Side effects   still on: database only
  ✓ Import         4 tables into
                   postgres://app@127.0.0.1:39021/app?sslmode=disable
...
 ╭─────────────────────────────────────────────────────────────────────────────────────╮
 │  PASS   postgres://app@127.0.0.1:39021/app?sslmode=disable is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 1 info                                    │
 ╰─────────────────────────────────────────────────────────────────────────────────────╯
  24.5 KB · 4 tables · 0.1s
  · info: This target is a database only: Roadie can't install the Echo seal
  or edit config files. Turn mail and jobs off where the app runs.
```

What differs from MySQL:

- **Dialects don't mix.** A PostgreSQL dump into a MySQL database (or a WordPress site) stops before a write with `dialect-mismatch` (exit 2). So does the other way round. So does a MySQL dump into a SQLite database or a SQLite database into MySQL or PostgreSQL: SQLite copies to SQLite only.
- **Formats.** `pg_dump -Fc` and `-Fd` are refused with the command to use (`dump-format`). Export with `pg_dump -Fp` (and `-E UTF8`).
- **A live PostgreSQL source** (`--from postgres://...`) sends rows only, no `CREATE TABLE`: the target must have the schema already. Run the app's migrations there or load `pg_dump --schema-only` first; otherwise the import stops with the steps to take and the target untouched.
- **Allowlist.** pg_dump's settings, `CREATE TABLE`, `INDEX`, `SEQUENCE`, `TYPE`, `DOMAIN` and `SCHEMA`, the common extensions (`pgcrypto`, `uuid-ossp`, `citext`, `pg_trgm`, `hstore`, `postgis`, `vector` and others), `COMMENT ON` and `setval()` are kept; `ALTER ... OWNER TO` is dropped with a plan line; `COPY ... PROGRAM`, `DO` blocks, `\connect` and the server's file functions are refused. Functions and triggers need `--keep-routines`; see [Security and limits](Roadie-Security-And-Limits#dumps-are-untrusted).

## SQLite

`sqlite:///absolute/path/app.sqlite` is a DSN source or target (three slashes, an absolute path). Roadie opens it with a pure-Go driver: no `sqlite3` binary needed. A SQLite source copies only to a SQLite target.

The file must exist: Roadie never creates a SQLite database. For a new target, create the empty file first (`touch /srv/app/staging.sqlite`); a missing one stops with `not-found` (exit 2) and the command to run. Confirm the target with its absolute path or the file's name: `--confirm-target staging.sqlite`. `prepare sqlite:///...` backs the database up by copying its file (and its `-wal`) to `.roadie/backups/<UTC time>.sqlite`; to undo, copy it back.

## When verify flags a column

A standalone verify of a generic database checks every text column for real-looking values; it doesn't know your config's intent. A column that holds personal data no rule covers is `uncovered-column` in a copy and a `pii.*` finding in a verify. Add a rule and copy again. Test data you keep on purpose goes in `verify.ignore` or a `--baseline` ([Verify and reports](Roadie-Verify-And-Reports#the-verdict)).

---
<sub>← [Laravel](Roadie-Laravel) · [Home](Home) · [Multisite](Roadie-Multisite) →</sub>
