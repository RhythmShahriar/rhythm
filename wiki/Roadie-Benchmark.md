How well does Roadie remove real people, live keys and side effects from a copy? How well does `verify` catch an unsafe staging? How fast is Roadie? This page describes the testbed, what was run for the 1.0.0 documentation and the results, with what they don't show.

> [!NOTE]
> We wrote both Roadie and the testbed, so read this as a regression suite with a scoreboard, not an independent review. Every scenario is something we built Roadie to handle. The canaries are checked by a separate oracle that does not use Roadie's code. Pseudonymised data is still personal data under the GDPR: these numbers show what Roadie removed from these fixtures, not that a copy of your site is free of personal data.

## Summary

| Measure | Target | Result at 1.0.0 |
|---|---|---|
| Dump-level copy scenarios (seeded canaries, live keys, serialized integrity, rows and joins, fresh verify) | all pass | **all pass** (`TestDumpScenarios`, run with the real binary, 6.5 s) |
| Demo: copy PASS, verify of the copy PASS, verify of the untouched shop FAIL | every run | **30 of 30** runs |
| verify on unsafe stagings, dump level | the expected finding IDs, exit 1 | **5 of 5** (v01, v02, v03, v05, v07); v04 and v06 need a site's files and weren't run here |
| verify false alarms on clean data | 0 findings at or above low over 5 rounds | **0 in 15 checks** (g01, g02, g03, five rounds each) |
| Copy throughput, 200 MB dump (`BenchmarkCopy200MBDump`) | index 300 MB/s, transform 100 MB/s | **not met**: 36.6 MB/s overall, index 59.9 MB/s, transform 83.7 MB/s |
| Peak RSS on that copy | 300 MB | **299.6 MB** |
| Verify throughput | 100 MB/s | **met**: 168.7 MB/s (database), 168.5 MB/s (dump) |
| Live key scan per value | 200 MB/s | **met**: 327.1 MB/s |
| MySQL dump tokenizer and scanner | 300 MB/s | tokenize **426.8 MB/s**, scan **219.3 MB/s** |

The full sealed testbed (the Docker sites, the side-effect drill and the comparison with other tools) wasn't run for this page; its generated report, `docs/benchmark.md` in the Roadie repository, comes with the release run. The numbers above were measured on one 16-thread Linux machine at a load average of about 4 to 5, so they are noisy; quieter machines run faster.

## The testbed

The testbed lives in the Roadie repository, in [`testbed/`](https://github.com/RhythmShahriar/roadie/tree/master/testbed). It runs in Docker Compose on a network with no route out: a production WordPress on MySQL 8.4, a staging WordPress on MariaDB 11.4, a Laravel app, PostgreSQL, an ssh host with WP-CLI, a sinkhole that answers every DNS name and records every connection, stubs for Stripe, PayPal, Twilio, Zapier and Mailchimp that flag live keys, Mailpit and Echo. Its one gate publishes on `127.0.0.1` only.

Before each run, `testbed/seed` fills every fixture with canaries: unique fake personal values per scenario and field (made-up names on real mail domains, numbers in real phone ranges) and fake keys assembled at run time, about 9,000 in all. An independent **oracle**, which imports no Roadie code, then looks for every canary in the output in every encoding (raw, case variants, JSON, URL-encoded, HTML entities, base64, serialized, md5, sha256), checks that every serialized value still unserializes in real PHP and that row counts and joins are unchanged.

Each copy and prepare scenario must pass four gates:

1. **No canary** left anywhere in the output database or file tree.
2. **Integrity**: every serialized value that parsed before still does; rows and joins are unchanged.
3. **No side effect**: zero sinkhole connections and zero stub calls with live keys during the run and a drill afterwards that tries `wp_mail`, `wp_remote_post`, raw cURL, `mail()`, an SMTP socket and `fsockopen`.
4. **Roadie's own verify** of the output in a fresh process: nothing at or above medium.

## The scenarios

The manifest, `testbed/scenarios.yaml`, says what each scenario holds and what counts as a pass. Findings are matched by ID, never by wording.

| # | Kind | Stack | Scenario |
|---|---|---|---|
| r01 | copy | woocommerce | WooCommerce HPOS shop with PII in orders, addresses, meta, lookup and notes |
| r02 | copy | woocommerce | HPOS compatibility mode on (orders in posts and HPOS tables) |
| r03 | copy | woocommerce | legacy shop with woocommerce_sessions and payment tokens |
| r04 | copy | woocommerce | Action Scheduler args, extended_args and the 191-byte boundary |
| r05 | copy | woocommerce | Subscriptions with live Stripe IDs, production WP_SITEURL and WP_HOME, a renewal due |
| r06 | copy | wordpress | Gravity Forms fields known from form JSON, notes, notifications and an upload |
| r07 | copy | wordpress | WP Mail Logging and FluentSMTP logs with bodies and reset links |
| r08 | copy | wordpress | serialized torture with a broken value |
| r09 | copy | wordpress | production URLs in JSON, blocks, URL-encoded and protocol-relative forms |
| r10 | copy | woocommerce | multisite in subdirectories with blogs 1, 2 and 11, Woo on blog 2 |
| r11 | copy | wordpress | multisite on subdomains with a mapped domain on blog 3 |
| r12 | copy | woocommerce | live keys everywhere, test keys in a second gateway |
| r15 | copy | laravel | Laravel on MySQL with jobs, notifications, sessions, .env keys and an encrypted cast |
| r16 | copy | sql | generic SQL CRM with email as a join key and a column the config forgot |
| r17 | copy | woocommerce | large generated dump with a 50 MB row, hex blobs, a DELIMITER trigger and canaries past 4 GiB |
| r18 | copy | woocommerce | every dump format Roadie reads, imported into MySQL 8.4 and MariaDB 11.4 |
| r19 | copy | woocommerce | copy from the sshd container; no raw production data on the local disk |
| r28 | copy | sql | PostgreSQL pg_dump -Fp of a Prisma-style schema with columns config |
| r13 | prepare | woocommerce | side effects armed on a running staging, traffic during prepare, host cron every 30 s |
| r14 | prepare | woocommerce | wp-config traps (environment, cron, getenv, CRLF and BOM, production DB and Redis hosts) |
| r21 | prepare | woocommerce | prepare twice, then with another stable key |
| r20 | env | woocommerce | Ctrl-C at 40%, SSH cut at 60%, disk full |
| r22 | env | woocommerce | Echo down while sealed |
| r23 | env | woocommerce | egress drill on a prepared staging (wp_mail, wp_remote_post, cURL, mail(), SMTP socket, fsockopen) |
| r24 | env | wordpress | hostile dumps |
| r25 | env | woocommerce | production refusals before any write |
| r26 | env | woocommerce | SIGKILL at random points, 50 runs; verify never passes on a partial state |
| r27 | env | woocommerce | plan with a SELECT-only user and a file system snapshot |
| v01 | verify | woocommerce | a staging made by wp search-replace only |
| v02 | verify | woocommerce | a generic tool skipped serialized and JSON values |
| v03 | verify | woocommerce | PII only in free text |
| v04 | verify | woocommerce | clean data, live key only in wp-config.php |
| v05 | verify | woocommerce | indexable and wired to the world |
| v06 | verify | woocommerce | debug.log, wc-logs and an UpdraftPlus backup |
| v07 | verify | woocommerce | sha256 and md5 of real emails, verify --from prod |
| g01 | verify | woocommerce | a clean small business with its own info@ on the allow list |
| g02 | verify | woocommerce | Woo sample data and WP-CLI users at example.com and .test |
| g03 | verify | woocommerce | fake names, CJK and RTL gift messages, Arabic-Indic digits, fiction phones, IBAN-shaped references |
| g04 | verify | laravel | a clean Laravel app with test keys, MAIL_MAILER=log and APP_ENV=staging |
| g05 | verify | mixed | every r-scenario output verified again in a fresh process, report check valid |

## What was run for this page

**Dump-level copies.** `TestDumpScenarios` runs every scenario that reads a seeded dump with the real `rhythm-roadie` and no Docker: a copy to the staging site becomes a copy to a dump, then the oracle (canaries, live and test keys, serialized integrity with PHP, row counts, joins), the fresh verify and the expected exit and error codes. It is part of `go test ./...`, so CI runs it on every push.

```text
=== RUN   TestDumpScenarios
--- PASS: TestDumpScenarios (6.50s)
=== RUN   TestDumpScenariosTranslate
--- PASS: TestDumpScenariosTranslate (0.00s)
PASS
ok  	github.com/RhythmShahriar/roadie/testbed/bench	6.507s
```

**Verify on unsafe stagings**, each seeded dump verified on its own with `--fail-on low`:

| # | Exit | Findings (critical · high · medium) | IDs |
|---|---|---|---|
| v01 | 1 | 1 · 6 · 3 | `key.live-mode`, `key.stripe-live`, `key.stripe-publishable-live`, `key.stripe-webhook-live`, `pii.address`, `pii.email`, `pii.name`, `pii.phone` |
| v02 | 1 | 0 · 3 · 2 | `pii.email`, `pii.ip`, `pii.phone`, `uncovered-column` |
| v03 | 1 | 0 · 5 · 1 | `pii.email`, `pii.phone`, `uncovered-column` |
| v05 | 1 | 1 · 4 · 0 | `key.live-mode`, `key.stripe-live`, `key.stripe-publishable-live`, `key.stripe-webhook-live`, `side-effect.webhooks` |
| v07 | 1 | 4 · 0 · 0 with `--from prod.sql` (exact mode, 400 hits) | `exact.survivors` |

v04 (a live key only in `wp-config.php`) and v06 (`debug.log`, `wc-logs`, an UpdraftPlus backup) are about a site's files, which a dump doesn't have; v05's `search.indexable` likewise needs the site. They run in the sealed testbed.

**False alarms.** g01 (with its config's allow list), g02 and g03 verified five rounds each with `--fail-on low`: exit 0 every time, 0 findings, 15 of 15. g04 is a Laravel site with its `.env`; at dump level it can't be judged.

**The demo**, 30 runs in a row: each copy passed its own verify, each verify of the untouched shop failed. (At integration it ran 100 times in a row, clean.)

**Speed**, from the repository's Go benchmarks (`go test -bench`), on a 16-thread machine at load 4.7:

```text
BenchmarkCopy200MBDump-16    	       1	4539789326 ns/op	  36.58 MB/s	        59.93 index-MB/s	       299.6 peak-RSS-MB	        83.67 transform-MB/s
BenchmarkVerify-16        	       1	1243708005 ns/op	 168.65 MB/s	351222872 B/op	12367712 allocs/op
BenchmarkVerifyDump-16    	       1	1297906064 ns/op	 168.54 MB/s	1981259504 B/op	12582607 allocs/op
BenchmarkScanValue-16    	  271909	      4203 ns/op	 327.13 MB/s
BenchmarkTokenize-16    	      15	  74991748 ns/op	 426.75 MB/s	       425.6 MB/cpu-s	 3567801 B/op	     745 allocs/op
BenchmarkScan-16        	       7	 145914903 ns/op	 219.33 MB/s	       158.8 MB/cpu-s
```

A 200 MB dump copies, with every value pseudonymised, in about 4.5 seconds. The index and transform passes are below their 1.0 targets (300 and 100 MB/s); the copy's peak memory sits at the 300 MB target. A real copy of a 400 MB generated WooCommerce dump during integration peaked at 325 MB RSS under heavy load.

## What this does not show

- **No independent review.** We built the tool, the fixtures and the oracle.
- **Fixtures, not your site.** The canaries are in places we knew about. A plugin we've never seen can store personal data where no rule looks; the uncovered-column scan and exact mode are there for that. Neither is perfect.
- **The sealed testbed wasn't run here.** The site-level gates (the side-effect drill, booting the copy, Soundcheck on staging), the ssh scenario under its watcher, the hostile-dump and kill scenarios and the comparison with `wp search-replace` and other tools are in `docs/benchmark.md` once the release run generates it.
- **One machine, under load.** Speed depends on the disk, the CPUs, compression and how much of the dump is personal data.

## Run it

```sh
git clone https://github.com/RhythmShahriar/roadie && cd roadie
go test ./testbed/bench/ -run TestDumpScenarios     # the dump-level scenarios, no Docker
go test -run '^$' -bench . ./engine/ ./verify/ ./keys/ ./dump/mysql/
make -C testbed up && make -C testbed bench         # the sealed testbed (Docker; downloads images and plugin zips once)
make -C testbed reproduce                           # twice, compared: results must be identical
```

---
<sub>← [FAQ](Roadie-FAQ) · [Home](Home) · [Sightread overview](Sightread) →</sub>
