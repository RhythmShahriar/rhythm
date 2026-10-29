Laravel has no fixed schema beyond `users` and the framework's own tables, so Roadie copies a Laravel app from a short config that names your tables. It truncates the framework's sessions, tokens, jobs and caches, rewrites `.env` so mail goes to Echo and queues run in the request, rotates `APP_KEY` and swaps or blanks live keys. This page walks through a copy of a Laravel 12 shop on MySQL with orders, notifications, queued jobs and a live `.env`.

The example app is at `/srv/app` (`APP_URL=https://app.staging.shop.test`), its database on MariaDB 10.11; production's dump is `prod.sql` with six tables: `users`, `orders`, `jobs`, `failed_jobs`, `notifications`, `sessions`. The staging's `.env` is a copy of production's: live Stripe and AWS keys, `MAIL_MAILER=ses`, `QUEUE_CONNECTION=database`.

## 1. A config for your tables

```sh
cd ~/app2
rhythm roadie config init --stack laravel
```

```text
Wrote .roadie/config.yaml for a Laravel app.
Set source:, target: and url: in it, then run rhythm-roadie plan.
```

Set the source, target and URL, then the `laravel:` block. The `laravel` profile covers `users.name`, `email`, `password`, `remember_token` and the two-factor columns, scrubs `notifications.data` and truncates the framework's tables. Name your own:

```yaml
source: prod.sql
target: /srv/app
url: https://app.staging.shop.test
production: [app.shop.example.com]
stack: laravel

laravel:                           # only for stack laravel
  env_file: .env
  tables:                          # Laravel has no fixed schema; name yours, e.g.
    users: {email: email, name: full_name, password: password, phone: phone}
    orders: {ip_address: ip, shipping_name: full_name, shipping_address: address}
  env: {}                          # extra .env keys to set, e.g. {QUEUE_CONNECTION: sync}
```

Each entry is `column: rule`, with the rules of `columns:` (`email`, `full_name`, `phone`, `ip`, `address`, `text`, `blank`, `null`, `json:<path>=<rule>` and the rest; see [Configuration](Roadie-Configuration#columns)). Laravel is detected by its `migrations` table; a dump without one needs `stack: laravel` (or `--stack laravel`).

## 2. Plan

Without the `orders` and `users.phone` entries, the plan stops on three columns that look personal:

```text
  UNCOVERED  3 ───────────────────────────────────────────────────────────────

 ▌  MEDIUM     orders.ip_address looks personal (by its name) and no rule co…
 ▌             → Add the column under columns: in .roadie/config.yaml
 ▌               (rhythm-roadie explain uncovered-column).
 ▌  MEDIUM     orders.shipping_address looks personal (by its name) and no r…
 ▌             → Add the column under columns: in .roadie/config.yaml
 ▌               (rhythm-roadie explain uncovered-column).
 ▌  MEDIUM     users.phone looks personal (100% of 60 sampled values are pho…
 ▌             → Add the column under columns: in .roadie/config.yaml
 ▌               (rhythm-roadie explain uncovered-column).

 ╭───────────────────────────────────────────────╮
 │  PLAN   copy would stop: 3 uncovered columns  │
 ╰───────────────────────────────────────────────╯
```

The rest of the plan shows what happens to the data and to `.env`:

```text
  ▸ Source        prod.sql (146 KB, 6 tables)
  ▸ Target        /srv/app (mysql)
  ▸ Stack         Laravel
...
  DATA  5 tables ─────────────────────────────────────────────────────────────

    failed_jobs                      60 rows   truncated
    jobs                             60 rows   truncated
    notifications                   120 rows   data → scrub
    sessions                         60 rows   truncated
    users                            60 rows   email, name, phone → fake;
                                               password → reset

  SIDE EFFECTS  2 ────────────────────────────────────────────────────────────

  ✓ Echo connector, sealed   MAIL_MAILER=smtp; MAIL_HOST=127.0.0.1; 6 more
  ✓ Queue                    QUEUE_CONNECTION=sync; Stop queue workers and
                             the scheduler on this host: jobs from
                             production were cleared, new ones run in the
                             request (QUEUE_CONNECTION=sync) through Echo.
...
  FILE EDITS  19 ─────────────────────────────────────────────────────────────

    · .env: APP_ENV=staging
    · .env: APP_URL=https://app.staging.shop.test
    · .env: APP_KEY=base64: and 32 random bytes: encrypted columns and old
      sessions from production become unreadable (pass --keep-app-key to
      keep the key)
    · .env: MAIL_MAILER=smtp
    · .env: MAIL_HOST=127.0.0.1
    · .env: MAIL_PORT=1025
    · .env: MAIL_USERNAME=
    · .env: MAIL_PASSWORD=
    · .env: RHYTHM_ECHO_URL=http://127.0.0.1:8025
    · .env: RHYTHM_ECHO_MODE=seal
    · by hand, composer.json: composer require rhythmsh/echo-laravel --dev
      (Roadie never runs Composer), so Echo seals mail, SMS and webhooks in
      the app too
    · .env: QUEUE_CONNECTION=sync
    · by hand, queue workers: Stop queue workers and the scheduler on this
      host: jobs from production were cleared, new ones run in the request
      (QUEUE_CONNECTION=sync) through Echo.
    · .env: key.aws-live at line 17 (AKIA•••••): swapped or blanked
    · .env: key.aws-live at line 18 (0XdW•••••): swapped or blanked
    · .env: key.stripe-publishable-live at line 20 (pk_l•••••): swapped or
      blanked
    · .env: key.stripe-live at line 21 (sk_l•••••): swapped or blanked
    · .env: key.stripe-webhook-live at line 22 (whse•••••): swapped or
      blanked
    · public/robots.txt: written: everything disallowed for every agent and
      the named AI crawlers
```

## 3. Copy

With `users.phone`, `orders.ip_address` and `orders.shipping_address` covered, the first copy got through the uncovered scan. Its exact verify still caught a column nobody named:

```text
 ▌  CRITICAL   120 A production value Roadie replaced survived in the copy
 ▌             orders.shipping_name 120
 ▌             e.g. L•••, J•••, H•••
 ▌             found as raw
 ▌             → Add a rule for the column (rhythm-roadie explain
 ▌               exact.survivors), then copy again.

 ╭───────────────────────────────────────────────────────╮
 │  FAIL   app.staging.shop.test is not a safe copy yet  │
 │  1 critical · 0 high · 0 medium · 0 low · 1 info      │
 ╰───────────────────────────────────────────────────────╯
```

`shipping_name` doesn't look personal by its name and holds no emails or phones. Its values were names Roadie had replaced in `users`, so exact mode found every one. With `shipping_name: full_name` added, the copy passes:

```text
  ♪ rhythm roadie 1.0.0                       prod.sql → app.staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        prod.sql
  ▸ Target        /srv/app
Replace the database of /srv/app? [y/N] y
  ▸ Stack         Laravel
  ▸ Plan          3 tables pseudonymised, 5 live keys, 2 side effects (plan
                  -v for all)

  ✓ Data           720 values pseudonymised in 3 tables, 0 URLs rewritten
  ✓ Keys           0 swapped to a test key, 5 blanked
  ✓ Side effects   Echo seal, queue
  ✓ Import         6 tables into /srv/app
...
  VERIFY (exact) ─────────────────────────────────────────────────────────────

    · Asks for a password                     --site not given
    ✓ The site says it is staging             1 checked
    ✓ Nothing points at production            1 checked
    ✓ Salts and APP_KEY are the copy's own    1 checked
    ✓ Encrypted values listed                 300 checked
    ✓ No pseudonymised value survives         300 checked
...
    ✓ Kept out of search engines              1 checked
    ✓ Off: Echo connector, sealed             1 checked
    ✓ Off: Queue                              1 checked
    ✓ Every personal-looking column covered   3 checked
    ✓ No personal files in uploads            7 checked
    ✓ No production URLs                      300 checked
    ✓ Site URL is the copy's                  301 checked

 ╭────────────────────────────────────────────────────────╮
 │  PASS   app.staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 1 info       │
 ╰────────────────────────────────────────────────────────╯
  146 KB · 6 tables · 0.3s
  · info: 60 Encrypted values no scanner can read (Laravel Crypt)

  Pseudonymised, not anonymised. The copy still holds order totals, dates,
  products and anything Roadie can't recognise in free text. Treat it as
  personal data.

  Mail from this copy goes to Echo at http://127.0.0.1:8025. Run rhythm echo
  up on the staging host, or the copy's mail will fail closed.
...
  → signed report: /home/ann/app2/.roadie/reports/2026-10-28T19-10-23Z-copy-app/report.html
  → next: rhythm echo up, then rhythm soundcheck run https://app.staging.shop.test
```

The info finding is `data.encrypted`: 60 values written with Laravel's encrypted cast, which no scanner can read. With the rotated `APP_KEY` the copy can't read them either. `--keep-app-key` keeps production's key so it can (and verify reports `config.salts`, since anyone with the copy can then forge production's cookies).

## What changed in .env

Order and comments are kept; keys are matched by name, last occurrence wins, as Laravel reads them:

```diff
-QUEUE_CONNECTION=database
-MAIL_MAILER=ses
+QUEUE_CONNECTION=sync
+MAIL_MAILER=smtp
...
-AWS_ACCESS_KEY_ID=AKIAAIAGNIBJKHF6QLKC
-AWS_SECRET_ACCESS_KEY=…
+AWS_ACCESS_KEY_ID=
+AWS_SECRET_ACCESS_KEY=
...
-STRIPE_KEY=…
-STRIPE_SECRET=…
-STRIPE_WEBHOOK_SECRET=…
+STRIPE_KEY=
+STRIPE_SECRET=
+STRIPE_WEBHOOK_SECRET=
+MAIL_HOST=127.0.0.1
+MAIL_PORT=1025
+MAIL_USERNAME=
+MAIL_PASSWORD=
+RHYTHM_ECHO_URL=http://127.0.0.1:8025
+RHYTHM_ECHO_MODE=seal
```

`APP_KEY` changed too (a new `base64:` key) and `public/robots.txt` disallows every agent. `laravel.env` adds lines of your own, for example `{BROADCAST_CONNECTION: log, SCOUT_DRIVER: "null"}`. A `.env` that sets the database credentials from another variable (`DB_PASSWORD=${SECRET}`) can't be read as text: pass the database as a DSN with `ROADIE_TO_PASSWORD` instead.

## On the staging host

Two steps are yours; the plan lists both as steps by hand:

1. **Echo's Laravel package.** `composer require rhythmsh/echo-laravel --dev` lets Echo seal mail, SMS and webhooks sent through the HTTP client, not only SMTP. Roadie never runs Composer. Then run `rhythm echo up` on the host. See [Echo: Connectors](Echo-Install-Connectors).
2. **Workers and the scheduler.** Stop queue workers and the scheduler (`schedule:run` in a crontab, Horizon, Supervisor) that ran before the copy: they were started with production's settings. New jobs run in the request (`QUEUE_CONNECTION=sync`) and reach Echo.

## Cashier

The `laravel-cashier` profile applies when the database has Cashier's `subscription_items` table. Each user's `stripe_id`, `pm_type` and `pm_last_four` are set to NULL, so the copy can't charge or look up a real Stripe customer. `trial_ends_at` and the subscriptions' `stripe_id` and `stripe_price` are kept for testing; `subscription_items` is kept as it is. `rhythm roadie profiles laravel-cashier` lists the rules.

## Other databases

Roadie reads `DB_CONNECTION` values `mysql`, `mariadb`, `pgsql` and `sqlite` from `.env`; any other stops with `site-config` (pass the database as a DSN instead). See [Generic SQL and Postgres](Roadie-Generic-SQL-And-Postgres) for what differs on PostgreSQL (PostgreSQL to PostgreSQL only; a live PostgreSQL source brings rows only).

---
<sub>← [Verify and reports](Roadie-Verify-And-Reports) · [Home](Home) · [Generic SQL and Postgres](Roadie-Generic-SQL-And-Postgres) →</sub>
