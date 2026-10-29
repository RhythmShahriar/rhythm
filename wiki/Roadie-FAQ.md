Answers to common questions about Roadie and its honest limits. For an error code, see [Troubleshooting](Roadie-Troubleshooting); for a flag, the [CLI reference](Roadie-CLI-Reference).

## About Roadie

### Why not WP STAGING, WP Migrate or my host's staging button?

Keep using them to make the copy. They copy the site and rewrite its URLs; they leave every customer, order, form entry and live key in place. Most leave cron, webhooks and gateways running. `rhythm roadie prepare <staging>` makes the copy they made safe, in place, after a backup. Or let Roadie make the copy: `rhythm roadie copy --from ssh://... --to <staging>`.

### Why not `wp search-replace`?

It rewrites URLs only and misses some of them: JSON-escaped URLs (`https:\/\/shop.com`, which page builders store) and URLs in serialized array keys stay as they were. It doesn't touch people, keys or side effects. Roadie rewrites every URL form, inside serialized and JSON values with lengths fixed, as one part of a copy.

### Why not a database masking tool?

Generic tools work column by column. They don't know that `wp_usermeta` holds billing addresses under `meta_key`, that Action Scheduler stores emails in JSON in `args`, that a PHP serialized string's length must change with its content, that `testmode = no` means a live gateway or that WP-Cron will email customers from the copy. Roadie ships those rules as profiles, turns the side effects off and proves the result with a verify.

### Is a pseudonymised copy GDPR compliant?

Roadie makes no such claim. A pseudonymised copy is still personal data under the GDPR (Article 4(5), Recital 26): it keeps order totals, dates, products, each person's history and anything Roadie can't recognise in free text. The consistency key (when you keep one) links fakes back to people. What Roadie gives you is evidence: a signed report of what was replaced and what its checks found. Treat the copy, its backups and its reports as you treat production.

### Can I push the staging database back to production?

Never. Its customers are fake, its passwords unusable, its keys blank, its gateways in test mode and its mail sealed. Deploy code and configuration from staging, never its database. `blog_public` is left as production had it on purpose, so a mistaken push at least doesn't hide production from search engines.

### Is it safe to point at production?

Roadie only reads production: a read-only transaction on a database, a fixed list of read commands over ssh ([the list](Roadie-Sources-And-Targets#remote-commands)), files read as text. A target that is production, is the source or says it is production is refused before anything is written. Run `plan` first: it shows every command and change and makes none. The export runs in one transaction, like `wp db export`; on a large database run it when production is quiet.

## Using it

### What do I need installed?

On your machine, nothing but `rhythm-roadie`: it talks to MySQL, MariaDB, PostgreSQL and SQLite through built-in drivers and never needs `mysql`, `php` or `wp` locally. For an ssh site, your own `ssh` plus, on the host, WP-CLI (or `mysqldump`/`pg_dump`) and GNU `find`. For the mail seal, Echo on the staging host.

### Does it copy files and uploads?

No. Roadie copies the database and edits `wp-config.php`, `.env` and its mu-plugins. `plan` prints an `rsync` command that leaves out the folders that usually hold personal data (logs, form uploads, backups). Uploads can still hold PDF invoices or photos with location data; `verify` lists backups, logs and form uploads it finds by name and size. `rhythm tuner scan <staging dir>` checks the files for leaks.

### How do I log in to the copy?

Every password is replaced. Keep your own accounts with `--keep-users 'admin,*@agency.example'` (or `keep_users:`); they stay real, password included. Or set a password for the fake administrator the copy names at the end: `wp user update user_ec55e2810c05a5ab --user_pass=...`.

### Can each person keep the same fake on every refresh?

Yes: `consistency: stable` (or `--keep-key`) keeps the key in `.roadie/key`, or set `ROADIE_KEY` in CI. Bookmarked test accounts and saved searches keep working. The key can link fakes back to real people, so guard it. See [Pseudonymisation](Roadie-Pseudonymisation#consistency-and-the-key).

### How big a database can it copy?

Roadie streams: rows go from source to target in batches, never a whole table in memory, with a soft memory limit of 300 MiB (`--max-memory`). A single value larger than 256 MiB stops the run unless you raise `--max-value`. Speed: see [Benchmark](Roadie-Benchmark) (a 200 MB dump takes seconds; the 1.0 throughput is below the targets on a busy machine).

### My plugin isn't in the profiles

The copy stops if its tables look personal (uncovered columns); exact mode catches values the rules missed. Add the columns under `columns:`; `config init --from <source>` suggests them. A profile for a popular plugin is a welcome contribution: each rule needs a positive and a negative fixture (see the repository's CONTRIBUTING).

### Does it work with MySQL 8 dumps on MariaDB?

Yes, with `--fix-collation`, which rewrites MySQL 8's `utf8mb4_0900_*` collations. Without it MariaDB refuses the import and Roadie says so:

```text
import: MariaDB 10.11 doesn't know collation utf8mb4_0900_ai_ci (MySQL 8). Run again with --fix-collation. The database is untouched but the kill switch already changed the site's files, so the target is in a mixed state.
```

### Does Roadie send anything anywhere?

Only to what you name: the source and target, the staging URL with `verify --site`, Echo at the URL you set plus GitHub and Sigstore for the update check and `rhythm roadie update`. No telemetry. The full list is in [Security and limits](Roadie-Security-And-Limits#every-network-request-roadie-can-make).

### Windows and macOS?

Roadie ships for Linux, macOS and Windows. ssh is your system's `ssh`; file modes (0600) aren't enforced on Windows, where your user profile's permissions protect Roadie's files. A dump written by PowerShell's `>` is UTF-16, which Roadie refuses: write it with `mysqldump --result-file` or `pg_dump -f`.

## Trusting the result

### What does a PASS prove?

That Roadie's checks found no real-looking person, live key, production URL or side effect still on, in what they read, at or above `--fail-on`. After a copy (exact mode) it also means no production value Roadie replaced survives anywhere, in eight encodings. It doesn't prove the copy holds no personal data. The signature proves who ran Roadie, which version and rules, on which input and that the report is unchanged since.

### What can't verify see?

Names and street addresses written in prose that Roadie never saw in a structured column; files (it lists some by name); other databases, Redis and search indexes; copies at third parties (Mailchimp lists, Stripe customers); encrypted columns; what you told it to keep. Every report ends with that list.

### Why did verify flag a fake-looking value?

Heuristic mode (a standalone verify) can't tell real from fake by origin, so it judges the shape: an email outside the reserved domains, a phone outside the fictional ranges, a name in a name column that isn't from Roadie's lists. Test data you made yourself (`test@gmail.com`) looks real. Allow it (`allow_emails`), accept it with `verify.ignore` or a `--baseline`, or verify with `--from <production>` for exact mode.

## Honest limits

- **Pseudonymised, not removed.** See above: the copy is still personal data.
- **Free text** keeps names and addresses Roadie never saw in a structured column.
- **The kill switch works inside the site.** A server crontab, a queue worker started before the copy, raw sockets and raw cURL without `--echo-proxy` are outside it. Credentials are blanked so most of those fail at the provider; blocking outbound traffic at the host is the only certainty.
- **Noindex is a request.** Basic auth is protection; Roadie only writes the snippets.
- **No files.** Uploads, logs outside `wp-content`, caches and search indexes aren't copied, cleaned or checked (apart from the listed folders).
- **Code isn't edited.** A live key in a theme or plugin is reported, never changed; over ssh, code isn't scanned.
- **Error trackers** (Sentry, Bugsnag) keep production's DSN, so the copy's errors reach production's project.
- **A copy by DSN** can't install the Echo seal or edit config files: turn mail and jobs off where the app runs.
- **Speed.** At 1.0 the index and transform passes run below their targets on a loaded machine ([Benchmark](Roadie-Benchmark)).
- **BusyBox hosts.** The plugin checks over ssh need GNU `find`.
- **Formats.** No `pg_dump -Fc`/`-Fd`, mydumper folders, `.wpress`, Duplicator archives or binary backups; no in-place `UPDATE` mode; no Drupal, Magento or Shopify. See the [roadmap](https://github.com/RhythmShahriar/roadie/blob/master/ROADMAP.md).

## Reporting a bug

Run the command again with `--debug` (timings and counts, never values) and open an issue at https://github.com/RhythmShahriar/roadie/issues with the output, `rhythm roadie version` and your OS. Never paste a dump or a report from a real site. A security problem in Roadie goes to https://github.com/RhythmShahriar/roadie/security/advisories/new instead.

---
<sub>← [Troubleshooting](Roadie-Troubleshooting) · [Home](Home) · [Benchmark](Roadie-Benchmark) →</sub>
