One grammar names where Roadie reads production and where it writes the copy, on the command line (`--from`, `--to`, the staging argument of `prepare` and `verify`), in `.roadie/config.yaml` (`source:`, `target:`) and in the GitHub Action (`source`, `target`). This page covers each kind, how Roadie reaches it, the dump formats it reads and every command it can run on a remote host.

## The kinds

| You give | Kind | As a source | As a target |
|---|---|---|---|
| `shop.sql`, `.sql.gz`, `.sql.zst`, `.sql.bz2`, `.dump` | dump file | read and streamed; compression by magic bytes, not the name | written atomically (temporary name, renamed when complete), 0600, gzip or zstd by the name |
| `-` | stdin or stdout | stdin, spooled encrypted for the second pass | stdout (not verified; see [Copy](Roadie-Copy-And-Prepare#copy)) |
| `/var/www/staging`, `./site` | local site | database credentials read from `wp-config.php` or `.env` as text, exported through a pure-Go driver | imported through the driver; `wp-config.php` or `.env` edited; the kill switch installed |
| `ssh://deploy@shop.com/var/www/shop`, `ssh://shop.com:2222/srv/app`, `deploy@shop.com:/var/www/shop` | ssh site | your system `ssh`, read commands only (below), streamed | streamed into `wp db import -` (or `mysql`/`psql` with credentials on stdin); remote files edited over ssh |
| `@prod`, `@staging` | WP-CLI alias from `wp-cli.yml` | the alias's `ssh:` or `path:` | the same |
| `mysql://`, `mariadb://`, `postgres://`, `postgresql://`, `sqlite:///abs/path.sqlite` | DSN | a read-only consistent read (MySQL `START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY`, PostgreSQL `REPEATABLE READ READ ONLY`) | import only; no files, so the kill switch is partial |

The shapes are tried in that order: `-`, `@name`, a DSN scheme, `ssh://`, `user@host:/path` (an existing local file or folder of that name wins), a dump extension, an existing file, an existing folder. Windows paths (`C:\x`, `\\server\share`) are always local. Anything else is a usage error:

```text
endpoint: <value> is not a dump file, site folder, ssh://, @alias or DSN
```

A path that doesn't exist stops with `not-found` (exit 2):

```text
not-found: nope.sql doesn't exist. Check the path you gave.
```

## Passwords

Credentials never go on the command line. A DSN's password comes from `ROADIE_FROM_PASSWORD` (the source) and `ROADIE_TO_PASSWORD` (the target), or from `${ROADIE_*}` in the config. A DSN with its own password still works, with a warning that it shows in `ps` to every user on the machine (`the password in --from is visible to other users of this machine; use ROADIE_FROM_PASSWORD`); it is masked everywhere Roadie prints it (`mysql://shop:•••@db.example/shop`).

```sh
ROADIE_TO_PASSWORD=... rhythm roadie copy --from shop.sql --to mysql://staging@127.0.0.1:39001/shop \
  --url https://staging.shop.test --confirm-target shop --yes
```

A local site's credentials are read from its `wp-config.php` (`DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_HOST`) or Laravel `.env` (`DB_*`) as text: PHP never runs. A config that builds them with `getenv()`, `$_ENV` or an expression stops with `site-config` (exit 3): pass the database as a DSN with `ROADIE_TO_PASSWORD` instead, or use a WP-CLI alias. The database user from a site's config is never shown in messages.

A database that doesn't answer stops with `unreachable` (exit 3), before anything is written:

```text
unreachable: the database of mysql://staging@127.0.0.1:39009/shop: dial tcp 127.0.0.1:39009: connect: connection refused. Check that the database is running and reachable from here; then check DB_HOST.
```

## ssh

Roadie runs your system `ssh`, so `~/.ssh/config`, agents, jump hosts, `ProxyJump` and `known_hosts` work as they do for you. Every call is

```text
ssh [-p <port>] [-o BatchMode=yes] -- [user@]host '<command>'
```

with `BatchMode=yes` when there is no terminal. Roadie never passes `StrictHostKeyChecking=no` or a throwaway `known_hosts`, never sends a password (use keys or an agent) and refuses a host that starts with `-` or holds whitespace or quotes. A host you have never connected to stops the run:

```text
ssh-host-key: shop.example.com isn't in known_hosts. Connect once with ssh to check and add it; Roadie never skips host key checks.
```

Connect once with `ssh shop.example.com`, check the fingerprint against what your host shows you and accept it. Then:

```sh
rhythm roadie copy --from ssh://shop.example.com/var/www/shop --to staging-ssh.sql --url https://staging.shop.test
```

```text
  ♪ rhythm roadie 1.0.0                   shop.example.com → staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        ssh://shop.example.com/var/www/shop
  ▸ Target        staging-ssh.sql
  ▸ Stack         WordPress + Action Scheduler + WooCommerce (HPOS)
  ▸ Plan          12 tables pseudonymised, 3 live keys, 2 side effects (plan
                  -v for all)

  ✓ Data           2,515 values pseudonymised in 12 tables, 80 URLs
                   rewritten
  ✓ Keys           0 swapped to a test key, 3 blanked
  ✓ Side effects   gateways, webhooks (data rules only: a dump has no files)
  ✓ Written        staging-ssh.sql (148 KB)
...
 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info   │
 ╰────────────────────────────────────────────────────╯
```

Production is exported once, with `wp db export` in one transaction. Pass 1 spools the stream to Roadie's cache folder encrypted (AES-256-GCM, a key held only in memory) and pass 2 reads the spool; the file is removed when the run ends, also on an error or Ctrl-C. The plain production dump is never written to your disk. A cut connection stops the run with `ssh` (exit 3) and the target untouched.

The production host needs WP-CLI on its `PATH` (or `remote.wp: /path/to/wp` in the config). Without it the run stops with `wp-cli` (exit 3): export the database yourself and copy from the dump. A failed remote command stops with `ssh`; Roadie says which part failed, never what the data held.

### WP-CLI aliases

`@name` reads the alias from `wp-cli.yml` or `wp-cli.local.yml` in the folder Roadie runs from or above it, as WP-CLI does. An alias with `ssh: deploy@shop.com/var/www/shop` is an ssh site; one with `path:` is a local site.

```text
wp-alias: WP-CLI alias not found: @nope is in no wp-cli.yml or wp-cli.local.yml from <dir> up. Add @nope to wp-cli.yml with ssh: user@host/path (or path: for a local site), or name the site as ssh://user@host/path.
```

## Remote commands

On production Roadie runs only read commands, from a fixed list built by one quoting function; a command outside the list can't be built. `plan` lists every command a copy may run there under REMOTE COMMANDS. For the shop over ssh:

```text
  REMOTE COMMANDS  32 ────────────────────────────────────────────────────────

    · test -f '/var/www/shop/wp-config.php'
    · wp --path='/var/www/shop' db export - --skip-plugins --skip-themes
      --single-transaction --skip-triggers
    · find '/var/www/shop/wp-content/plugins/automatewoo/automatewoo.php'
      -maxdepth 0 -printf '%y %s %P\n'
...
    · find '/var/www/shop/wp-content/plugins/wpforms/wpforms.php' -maxdepth
      0 -printf '%y %s %P\n'
    · cat -- '/var/www/shop/wp-config.php'
    · cat -- <arg>
    · find <arg> -maxdepth <n> -printf '%y %s %P\n'
    · mysqldump --defaults-extra-file=/dev/stdin --single-transaction
      --skip-triggers --no-tablespaces --hex-blob <arg> [<table> ...]
    · IFS= read -r PGPASSWORD; export PGPASSWORD; exec pg_dump
      --format=plain --no-owner --no-privileges --host=<arg> --port=<n>
      --username=<arg> --dbname=<arg> [<table> ...]
    · test -f <arg>
    · wp --path=<path> core version --skip-plugins --skip-themes
    · wp --path=<path> db export - --skip-plugins --skip-themes
      --single-transaction --skip-triggers
    · wp --path=<path> db export - --tables=<arg> --skip-plugins
      --skip-themes --single-transaction --skip-triggers
    · wp --path=<path> option get <siteurl|home> --skip-plugins
      --skip-themes
```

The first lines are the exact commands of this run; the rest is the whole list with placeholders. On a **source**, the complete list:

| Command | What for |
|---|---|
| `wp --path=<path> core version --skip-plugins --skip-themes` | WordPress's version |
| `wp --path=<path> option get siteurl` (and `home`) `--skip-plugins --skip-themes` | production's URL |
| `wp --path=<path> db export - --skip-plugins --skip-themes --single-transaction --skip-triggers` | the export, streamed |
| the same with `--tables=<list>` | `prepare --only` and an options-only read |
| `cat -- <path>`, `test -f <path>` | on a source only `wp-config.php` and `.env` |
| `find <path> -maxdepth <n> -printf '%y %s %P\n'` | names and sizes only: which plugins are installed, verify's file checks |
| `mysqldump --defaults-extra-file=/dev/stdin --single-transaction --skip-triggers --no-tablespaces --hex-blob <db> [<table> ...]` | a database without WP-CLI; the credentials go on stdin |
| `IFS= read -r PGPASSWORD; export PGPASSWORD; exec pg_dump --format=plain --no-owner --no-privileges --host=... --port=... --username=... --dbname=... [<table> ...]` | PostgreSQL; the password goes on stdin, never in argv |

On a **target** (staging), also:

| Command | What for |
|---|---|
| `wp --path=<path> db import -` | the import |
| `umask 077 && f=$(mktemp) ... mysql --defaults-extra-file="$f" <db>` | MySQL without WP-CLI: the credentials are read from stdin into a 0600 temporary file that is removed on exit |
| `IFS= read -r PGPASSWORD; export PGPASSWORD; exec psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 ...` | PostgreSQL import |
| `set -C && umask 077 && mkdir -p ~/.roadie-backups && { wp --path=<path> db export - ... } \| gzip > ~/.roadie-backups/<UTC timestamp>.sql.gz` | `prepare`'s backup, on the staging host |
| `umask 077 && cat > <tmp> && mv -f -- <tmp> <path>` (and forms that keep or set the mode and owner) | writing `wp-config.php`, `.env` and the mu-plugins |
| `ls -lnLd -- <path>`, `mkdir -p -- <path>`, `rm -f -- <path>` | the same files |
| `cat -- <path>`, `test -f <path>` | reading back what the kill switch wrote |
| `wp --path=<path> eval-file - --skip-plugins --skip-themes` | installing the Echo connector (the script goes on stdin); plugins copied from production don't run before the connector is in place |

`find -printf` needs GNU find. On a host whose `find` is BusyBox's (Alpine) the plugin checks fail: the copy still works, more slowly, since each failed check is tried again. Install GNU find (`findutils`) on such a host.

Each command is its own ssh connection. A plan or copy over ssh runs one per plugin check, so on a slow link `ControlMaster auto` with a `ControlPersist` in your `~/.ssh/config` saves the handshakes.

## Dumps Roadie reads

| Read | Refused, with the export to use instead |
|---|---|
| `mysqldump` 5.7 and 8.x, `mariadb-dump` (with its sandbox first line), `wp db export`, phpMyAdmin and Adminer SQL exports, `mysqldump --hex-blob`, `--skip-extended-insert`, `pg_dump -Fp` (COPY and INSERT), any of them gzip, zstd or bzip2 compressed | `pg_dump -Fc` and `-Fd`, mydumper folders, `.wpress`, Duplicator archives, zip and tar files, a dump saved as UTF-16 (PowerShell's `>`) |

```text
dump-format: prod.dump is a pg_dump custom-format archive. Roadie reads plain SQL: pg_dump -Fp, mysqldump, mariadb-dump or wp db export.
```

A dump that ends early stops the run, so half a production export never becomes a staging:

```text
dump-truncated: cut.sql ends inside table wp_usermeta at 60.0 KB, with no "Dump completed" line. Export it again; the target is untouched.
```

Dumps are code, so Roadie never pipes one into a database client: it parses it and re-emits only the statements on its allowlist. Triggers, procedures, functions and views stop the run unless `--keep-routines`; `GRANT`, `CREATE USER`, `ALTER ... OWNER TO` and the like are dropped with a plan line; client commands, `LOAD DATA`, `INTO OUTFILE` and a multibyte client character set are refused. The full lists are in [Security and limits](Roadie-Security-And-Limits#dumps-are-untrusted).

MySQL copies to MySQL or MariaDB, PostgreSQL to PostgreSQL, SQLite to SQLite. Anything else (a MySQL dump into a SQLite database too) stops before a write:

```text
dialect-mismatch: prod.sql is a PostgreSQL dump and --to is a MySQL database. Roadie copies MySQL to MySQL or MariaDB, PostgreSQL to PostgreSQL and SQLite to SQLite.
```

A copy from a live PostgreSQL database brings rows only (no `CREATE TABLE`), so the target must have the schema already: run the app's migrations there or load `pg_dump --schema-only` first.

## Where Roadie looks for live keys

In every value of the database (inside serialized, JSON and base64-serialized values too), every `define()` in `wp-config.php` and every `.env` line. A local site target's theme and plugin code is scanned too and reported with `action: manual` (Roadie never edits code). Over ssh Roadie reads only `wp-config.php` and `.env`; the report's coverage says so. For file-level leaks on any site, run `rhythm tuner scan <staging dir>`.

---
<sub>← [Copy and prepare](Roadie-Copy-And-Prepare) · [Home](Home) · [Profiles](Roadie-Profiles) →</sub>
