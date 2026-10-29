This tutorial takes you from the demo to a checked staging copy: try Roadie on its built-in shop, write a config, look at the plan, copy a production dump, verify the copy, check the signed report, then copy into a real staging site and log in. It takes about fifteen minutes.

You need a dump or a copy of a site you own or are authorised to copy. The examples use a WooCommerce shop whose production site is `https://shop.example.com`. It is exported to `shop.sql`, the demo's own shop: `rhythm roadie demo --keep` leaves it in `~/.cache/rhythm/roadie/demo-<n>/` (the demo prints the folder on its last lines), so copy it into your project folder. The staging site is at `/srv/staging` for `https://staging.shop.test`, on the same machine, with its database on MariaDB 10.11. Every output on this page comes from those runs with Roadie 1.0.0. Fakes are random per run, so yours differ.

## 1. Check the install

```sh
rhythm roadie version
```

```text
rhythm-roadie 1.0.0 (commit 70e086b)
```

If it says `rhythm: roadie isn't installed`, run `rhythm install roadie` (rhythm v0.9.0 or later). If `rhythm` isn't found at all, follow [Install](Install). `go install github.com/RhythmShahriar/roadie/cmd/rhythm-roadie@latest` works too.

From a checkout, build it yourself (Go 1.27.3 or later, no network once the modules are in your cache):

```sh
git clone https://github.com/RhythmShahriar/roadie && cd roadie
make build                     # bin/rhythm-roadie, stamped with the version and commit
bin/rhythm-roadie version
```

A source build runs as `rhythm-roadie` (here `bin/rhythm-roadie`), not `rhythm roadie`, so use that name wherever this page says `rhythm roadie`. `make build` stamps the nearest release tag and the commit; between tags, or with `go build` alone, the version reads `dev` (`rhythm-roadie dev (commit b2fb226)`) and the profile pack `0.0.0-<date>-<commit>`. The build is still right: only the label differs.

## 2. Try the demo

```sh
rhythm roadie demo
```

The first Roadie command on a machine (other than `help`, `version` and `update`) prints a one-time note:

```text
Note: use Roadie on sites you own or are authorised to copy. It reads production
data to pseudonymise it and never writes to production. Pseudonymised data is still
personal data: keep copies as protected as the live site. This notice is shown once.
```

Then the demo builds a small WooCommerce shop in memory, with 40 fake "real" customers, a live-looking Stripe key, an active webhook and production URLs in serialized and JSON-escaped forms. It plans and copies it, verifies the copy, verifies the untouched shop and deletes everything again. No network, no database server, about a second:

```text
  ♪ rhythm roadie 1.0.0                                 demo · a built-in shop
  ────────────────────────────────────────────────────────────────────────────

  ▸ Shop          40 fake customers, a live-looking Stripe key, an active
                  webhook, production URLs in serialized and JSON-escaped
                  forms

  plan ───────────────────────────────────────────────────────────────────────


 ╭────────────────────────────────────────────────────╮
 │  PLAN   ready: nothing changes until you run copy  │
 ╰────────────────────────────────────────────────────╯

  copy ───────────────────────────────────────────────────────────────────────


 ╭─────────────────────────────────────────────────────────╮
 │  PASS   staging.demo-shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info        │
 ╰─────────────────────────────────────────────────────────╯
Created a signing key for Roadie reports: roadie:21ZYFug… (keep /home/ann/.config/rhythm/roadie/signing.key private; set ROADIE_SIGNING_KEY in CI to sign with one identity).

  verify the copy ────────────────────────────────────────────────────────────


 ╭───────────────────────────────────────────────────╮
 │  PASS   staging.sql is a pseudonymised copy       │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info  │
 ╰───────────────────────────────────────────────────╯

  verify the shop ────────────────────────────────────────────────────────────


 ╭─────────────────────────────────────────────────────╮
 │  FAIL   shop.sql is not a safe copy yet             │
 │  2 critical · 16 high · 14 medium · 0 low · 0 info  │
 ╰─────────────────────────────────────────────────────╯

  Pseudonymised, not anonymised. The copy still holds order totals, dates,
  products and anything Roadie can't recognise in free text. Treat it as
  personal data.

  → report of the copy: /home/ann/.cache/rhythm/roadie/demo-2368045250/.roadie/reports/2026-10-28T18-56-01Z-verify-staging.sql/report.html
  → report of the shop: /home/ann/.cache/rhythm/roadie/demo-2368045250/.roadie/reports/2026-10-28T18-56-01Z-verify-shop.sql/report.html
  The demo's dumps and reports are deleted now; --keep keeps them.
  → next: rhythm-roadie plan --from <your dump or ssh://...> --to <staging>
```

The first run that signs a report creates your signing key (more in [step 7](#7-check-the-signature)). `--keep` keeps the demo's `shop.sql`, `staging.sql` and both reports so you can open them, in the folder its last lines name (`~/.cache/rhythm/roadie/demo-<n>/`, or under `$XDG_CACHE_HOME`). Copy `shop.sql` from there into your project folder for the next steps:

```sh
mkdir -p ~/shop && cp ~/.cache/rhythm/roadie/demo-*/shop.sql ~/shop/
```

## 3. Write a config

Run Roadie from your project's folder, the site's repository if it has one, so its reports stay with the project:

```sh
cd ~/shop
rhythm roadie config init --stack woocommerce
```

```text
Wrote .roadie/config.yaml for a WooCommerce site.
Set source:, target: and url: in it, then run rhythm-roadie plan.
Added .roadie/reports, .roadie/key, .roadie/backups and .roadie/basic-auth to .git/info/exclude.
```

The file lists every setting with a comment; everything is optional. Set the four at the top:

```yaml
source: shop.sql                             # ssh://deploy@shop.com/var/www/shop, prod.sql.gz, @prod, mysql://...
target: staging.sql                          # /var/www/staging, ssh://..., @staging, staging.sql.gz
url: https://staging.shop.test               # the copy's URL, e.g. https://staging.shop.test
production: [shop.example.com]               # never a target; verify flags these, e.g. ["shop.com", "*.shop.com"]
```

`production:` is the list Roadie refuses to write to and the hosts `verify` reports when they are left in a copy. Commit the file: it never holds a password or a key (`${ROADIE_*}` reads them from the environment). `rhythm roadie config check` validates it. Every key is in [Configuration](Roadie-Configuration).

## 4. Look first

```sh
rhythm roadie plan
```

`plan` reads the source and the target read-only and lists every table, rule, key, file edit, remote command and side effect a copy would touch. It changes nothing:

```text
  ♪ rhythm roadie 1.0.0                                 plan · nothing changes
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        shop.sql (166 KB, 13 tables)
  ▸ Target        staging.sql
  ▸ Stack         WordPress + Action Scheduler + WooCommerce (HPOS)
  ▸ URLs          shop.example.com → https://staging.shop.test (serialized
                  and JSON safe)
  ▸ Consistency   per-run key: each real person gets one fake, across every
                  table

  DATA  12 tables ────────────────────────────────────────────────────────────

    wp_actionscheduler_actions       20 rows   args, extended_args → scrub
    wp_comments                      80 rows   comment_agent, comment_author,
                                               comment_author_email → fake;
                                               comment_content → scrub
    wp_options                       14 rows   option_value (1 kept) → URLs
                                               rewritten
    wp_postmeta                      24 rows   meta_value → URLs rewritten
    wp_posts                         71 rows   guid, post_content → URLs
                                               rewritten
    wp_usermeta                     883 rows   meta_value (40 kept) → fake
    wp_users                         41 rows   display_name, user_email,
                                               user_login, user_nicename,
                                               user_url (40 kept) → fake;
                                               user_pass → reset
    wp_wc_customer_lookup            40 rows   city, email, first_name,
                                               last_name, postcode, username →
                                               fake
    wp_wc_order_addresses           120 rows   address_1, city, email (60
                                               kept), first_name, last_name,
                                               phone, postcode → fake
    wp_wc_orders                     60 rows   billing_email, ip_address,
                                               user_agent → fake;
                                               transaction_id → blank
    · ... 2 more (rhythm-roadie plan -v lists them)

  KEYS  3 live keys ──────────────────────────────────────────────────────────

 ▌  HIGH       stripe-publishable live key
 ▌             publishable_key = pk_l•••••
 ▌             → blank
 ▌  CRITICAL   stripe live key         wp_options woocommerce_stripe_settings
 ▌             secret_key = sk_l•••••
 ▌             → blank
 ▌  HIGH       stripe-webhook live key
 ▌             webhook_secret = whse•••••
 ▌             → blank

  SIDE EFFECTS  2 ────────────────────────────────────────────────────────────

  ✓ Gateways                 payment gateways to test mode, live keys
                             swapped or blanked in the data
  ✓ WooCommerce webhooks     WooCommerce webhooks paused, their secrets
                             blanked in the data

  ▸ Not stopped   a server crontab calling wp cron or artisan schedule:run,
                  wp action-scheduler run by hand, queue workers already
                  running, code that opens SMTP sockets or uses fsockopen
                  with credentials Roadie doesn't know, raw cURL without
                  --echo-proxy. Credentials are blanked so those paths fail
                  at the provider. Blocking outbound traffic at the host is
                  the only certainty.

  ALSO ───────────────────────────────────────────────────────────────────────

    · 80 production URLs rewritten in 3 tables
    · Files are not copied. For uploads, leave out the folders that hold
      personal data: rsync -a --exclude wp-content/debug.log --exclude
      wp-content/uploads/wc-logs --exclude wp-content/uploads/gravity_forms
      --exclude wp-content/updraft --exclude wp-content/ai1wm-backups
      --exclude '*.sql' --exclude '*.sql.gz' --exclude '*.zip'
      '<production>/' '<staging>/'
    · Object caches and search indexes (Redis, Algolia, ElasticPress) are
      not copied or cleared

 ╭────────────────────────────────────────────────────╮
 │  PLAN   ready: nothing changes until you run copy  │
 ╰────────────────────────────────────────────────────╯
  read schema and counts in 0.0s; no rows changed

  → run the same command line with copy (or prepare) instead of plan to make the change
```

How to read it:

- **DATA** names each table, its row count and what happens to each column: `→ fake` gets a consistent fake, `→ scrub` keeps the text and replaces the personal values in it, `→ blank`, `→ reset` (passwords), `truncated` (structure kept, rows not). `(40 kept)` counts values a rule left as they were: a country or state Roadie keeps for tax rules, an empty field. `plan -v` lists every column with the rule that covers it.
- **KEYS** lists every live key with its location, its masked value and what happens to it: no test key is set yet, so each is blanked.
- **SIDE EFFECTS** lists what is switched off. A dump has no files, so only the data rules apply here; a site target gets the whole kill switch (step 8).
- **ALSO** says what Roadie doesn't do: copy files, clear caches.

A plan that would stop says so in its box, for example `PLAN   copy would stop: 1 uncovered column` when a column looks personal and no rule covers it. See [Copy and prepare](Roadie-Copy-And-Prepare#uncovered-columns).

`plan` exits 0 whatever its box says, a plan that would stop included: it only reports. In CI, add `--fail-on` (any level, e.g. `--fail-on high`) and it exits 1 when the copy would stop or would leave a finding at or above that level.

## 5. Copy

```sh
rhythm roadie copy
```

With `source:`, `target:` and `url:` in the config, `copy` needs no arguments (flags win over the config: `rhythm roadie copy --from shop.sql --to staging.sql --url https://staging.shop.test` is the same run). It streams the dump through the rules into `staging.sql`, then verifies what it wrote:

```text
  ♪ rhythm roadie 1.0.0                           shop.sql → staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        shop.sql
  ▸ Target        staging.sql
  ▸ Stack         WordPress + Action Scheduler + WooCommerce (HPOS)
  ▸ Plan          12 tables pseudonymised, 3 live keys, 2 side effects (plan
                  -v for all)

  ✓ Data           2,515 values pseudonymised in 12 tables, 80 URLs
                   rewritten
  ✓ Keys           0 swapped to a test key, 3 blanked
  ✓ Side effects   gateways, webhooks (data rules only: a dump has no files)
  ✓ Written        staging.sql (147 KB)

  ▸ Not stopped   a server crontab calling wp cron or artisan schedule:run,
                  wp action-scheduler run by hand, queue workers already
                  running, code that opens SMTP sockets or uses fsockopen
                  with credentials Roadie doesn't know, raw cURL without
                  --echo-proxy. Credentials are blanked so those paths fail
                  at the provider. Blocking outbound traffic at the host is
                  the only certainty.

  VERIFY (exact) ─────────────────────────────────────────────────────────────

    · Asks for a password                     --site not given
    · The site says it is staging             a database or dump target has
                                              no files to read
    · Nothing points at production            a database or dump target has
                                              no files to read
    · Salts and APP_KEY are the copy's own    a database or dump target has
                                              no files to read
    ✓ Encrypted values listed                 1,496 checked
    ✓ No pseudonymised value survives         1,496 checked
    ✓ No secret-looking settings              1,496 checked
    ✓ No live keys                            1,496 checked
    ✓ No gateway in live mode                 1,496 checked
    ✓ No provider key of unknown mode         1,496 checked
    ✓ No real-looking addresses               1,496 checked
    ✓ No card numbers                         1,496 checked
    ✓ No real-looking emails                  1,496 checked
    ✓ No IBANs                                1,496 checked
    ✓ No public IP addresses                  1,496 checked
    ✓ No real-looking names                   1,496 checked
    ✓ No real-looking phones                  1,496 checked
    · Kept out of search engines              --site not given
    · Off: Action Scheduler                   20 pending actions; a database
                                              or dump target has no files to
                                              read, so pause Action
                                              Scheduler where the app runs
    ✓ Off: AutomateWoo                        1 checked
    · Off: WP-Cron                            a database or dump target has
                                              no files to read
    · Off: Gateways                           on a database or dump target
                                              the data scan's key.live-mode
                                              reads the gateways
    · Off: Jetpack                            a database or dump target has
                                              no files to read
    · Off: Echo connector, sealed             a database or dump target has
                                              no files to read
    ✓ Off: Subscriptions                      1 checked
    ✓ Off: WooCommerce webhooks               1 checked
    ✓ Every personal-looking column covered   13 checked
    · No personal files in uploads            a database or dump target has
                                              no files to read
    ✓ No production URLs                      1,496 checked
    ✓ Site URL is the copy's                  1,496 checked

 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info   │
 ╰────────────────────────────────────────────────────╯
  166 KB · 13 tables · 0.1s

  Pseudonymised, not anonymised. The copy still holds order totals, dates,
  products and anything Roadie can't recognise in free text. Treat it as
  personal data.

  No user was kept, so nobody can log in with a real password. Set one: wp
  user update user_54679f497d903d88 --user_pass=<new password>

  ALSO ───────────────────────────────────────────────────────────────────────

    · 80 production URLs rewritten in 3 tables
    · Files are not copied. For uploads, leave out the folders that hold
      personal data: rsync -a --exclude wp-content/debug.log --exclude
      wp-content/uploads/wc-logs --exclude wp-content/uploads/gravity_forms
      --exclude wp-content/updraft --exclude wp-content/ai1wm-backups
      --exclude '*.sql' --exclude '*.sql.gz' --exclude '*.zip'
      '<production>/' '<staging>/'
    · Object caches and search indexes (Redis, Algolia, ElasticPress) are
      not copied or cleared

  → signed report: /home/ann/shop/.roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.html
  → next: load staging.sql into the staging database, or copy straight to it: rhythm-roadie copy --from shop.sql --to <staging site or DSN>
```

The verify ran in **exact** mode: Roadie kept a keyed hash of every real value it replaced and checked that none of them is left anywhere in the copy, in eight encodings. A `·` check didn't apply or couldn't tell (a dump has no `wp-config.php` to read; only the running app can pause Action Scheduler's 20 pending actions); a `✓` passed with the number of values it read. The result box is the verdict: PASS means no finding at or above `--fail-on` (default `high`). Exit code 0.

`shop.sql` is untouched: Roadie never rewrites a dump in place. `staging.sql` is written to a temporary name first and renamed when it is complete, readable by you only (mode 0600).

## 6. Check it yourself

`verify` checks any copy, made by Roadie or not. On its own it runs in **heuristic** mode: it can't know which values were real, so it looks for anything that looks real.

```sh
rhythm roadie verify staging.sql
```

```text
  ♪ rhythm roadie 1.0.0                                   verify · staging.sql
  ────────────────────────────────────────────────────────────────────────────

  ▸ Mode          heuristic: this copy wasn't made by Roadie in this run
  ▸ Database      staging.sql, 13 tables of 13, every row
  ▸ Production    shop.example.com

  CHECKS  30 ─────────────────────────────────────────────────────────────────
...
 ╭───────────────────────────────────────────────────╮
 │  PASS   staging.sql is a pseudonymised copy       │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info  │
 ╰───────────────────────────────────────────────────╯
  147 KB · 13 tables · 1,496 rows · 0.0s
...
  Not covered by this report: files and uploads (PDF invoices, form uploads,
  images with location data), other databases, object caches (Redis,
  Memcached), search indexes (Algolia, ElasticPress), logs on disk outside
  wp-content, copies held by third parties (Mailchimp lists, Stripe
  customers), names and street addresses written in free text that Roadie
  didn't see in a structured column, encrypted columns, tables you told Roadie
  to keep.

  → signed report: /home/ann/shop/.roadie/reports/2026-10-28T18-56-41Z-verify-staging.sql/report.html
```

Give it production's dump and it checks every real value exactly, as the copy's own verify did:

```sh
rhythm roadie verify staging.sql --from shop.sql
```

```text
  ▸ Mode          exact: every value Roadie replaced was checked
...
    ✓ No pseudonymised value survives         1,496 checked
...
 │  PASS   staging.sql is a pseudonymised copy       │
```

To see what a failing verify looks like, verify the production dump itself (`rhythm roadie verify shop.sql`); the [Verify and reports](Roadie-Verify-And-Reports#reading-a-failing-verify) page walks through its 32 findings.

## 7. Check the signature

Every copy, prepare and verify writes `report.json` (signed) and `report.html` into `.roadie/reports/<UTC time>Z-<command>-<target>/`, for example `2026-10-28T18-56-30Z-copy-staging.sql`. The time is UTC (the `Z`), so it may differ from your clock; two runs in the same second get `-2`, `-3`. Anyone with the JSON can check it:

```sh
rhythm roadie report check .roadie/reports/2026-10-28T18-56-30Z-copy-staging.sql/report.json --input shop.sql
```

```text
signed by roadie:xSmAjAE… and unchanged since. It shows this Roadie run found these results; it doesn't prove the checks covered everything.
Key ID roadie:xSmAjAEGqcTF5wui: pin it with --key roadie:xSmAjAEGqcTF5wui to accept only this signer.
shop.sql is the input it describes (the sha256 matches).
```

The key was created on the first signing run, in your config folder. A copy's report fingerprints the input it read (`shop.sql`); a verify's report fingerprints what it verified. A changed byte, another input or another key fails with exit 1; an unsigned report exits 3. In CI, set `ROADIE_SIGNING_KEY` so every run signs with one identity. See [Verify and reports](Roadie-Verify-And-Reports#signed-reports).

## 8. Copy into a staging site

A dump is the safe first step. Most of the time the copy goes straight into a staging site, which also gets the file-level kill switch. Here `/srv/staging` is a WordPress site whose `wp-config.php` points at an empty staging database:

```sh
rhythm roadie copy --from shop.sql --to /srv/staging --url https://staging.shop.test
```

```text
  ♪ rhythm roadie 1.0.0                           shop.sql → staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        shop.sql
  ▸ Target        /srv/staging

 ╭─────────────────────────────────────────────────────────────╮
 │  STOPPED   target-says-production; the target is untouched  │
 ╰─────────────────────────────────────────────────────────────╯

  target-says-production: /srv/staging has WP_ENVIRONMENT_TYPE unset, which
  WordPress reads as production. If it's staging, set WP_ENVIRONMENT_TYPE to
  staging there, or pass --target-is-staging staging.shop.test.

  → rhythm-roadie explain target-says-production
```

That is a guard, not an error: WordPress treats an unset `WP_ENVIRONMENT_TYPE` as production. Roadie never writes to a target that says it is production. This one is staging, so name its host (or set `WP_ENVIRONMENT_TYPE` to `staging` in its `wp-config.php` first). Roadie then asks before it replaces the database:

```sh
rhythm roadie copy --from shop.sql --to /srv/staging --url https://staging.shop.test --target-is-staging staging.shop.test
```

```text
  ♪ rhythm roadie 1.0.0                           shop.sql → staging.shop.test
  ────────────────────────────────────────────────────────────────────────────

  ▸ Source        shop.sql
  ▸ Target        /srv/staging
Replace the database of /srv/staging? [y/N] y
  ▸ Stack         WordPress + Action Scheduler + WooCommerce (HPOS)
  ▸ Plan          12 tables pseudonymised, 3 live keys, 5 side effects (plan
                  -v for all)

  ✓ Data           2,515 values pseudonymised in 12 tables, 80 URLs
                   rewritten
  ✓ Keys           0 swapped to a test key, 3 blanked
  ✓ Side effects   cron, Echo seal, Action Scheduler, webhooks, gateways
  ✓ Search         noindex, robots.txt
  ✓ Import         13 tables into /srv/staging
...
  VERIFY (exact) ─────────────────────────────────────────────────────────────

    · Asks for a password                     --site not given
    ✓ The site says it is staging             1 checked
    ✓ Nothing points at production            1 checked
    ✓ Salts and APP_KEY are the copy's own    1 checked
...
    ✓ Kept out of search engines              1 checked
    ✓ Off: Action Scheduler                   1 checked
    ✓ Off: AutomateWoo                        1 checked
    ✓ Off: WP-Cron                            1 checked
    ✓ Off: Gateways                           1 checked
    ✓ Off: Jetpack                            1 checked
    ✓ Off: Echo connector, sealed             1 checked
    ✓ Off: Subscriptions                      1 checked
    ✓ Off: WooCommerce webhooks               1 checked
    ✓ Every personal-looking column covered   13 checked
    ✓ No personal files in uploads            7 checked
    ✓ No production URLs                      1,496 checked
    ✓ Site URL is the copy's                  1,497 checked

 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 0 info   │
 ╰────────────────────────────────────────────────────╯
  166 KB · 13 tables · 0.5s

  Pseudonymised, not anonymised. The copy still holds order totals, dates,
  products and anything Roadie can't recognise in free text. Treat it as
  personal data.

  Mail from this copy goes to Echo at http://127.0.0.1:8025. Run rhythm echo
  up on the staging host, or the copy's mail will fail closed.

  No user was kept, so nobody can log in with a real password. Set one: wp
  user update user_ec55e2810c05a5ab --user_pass=<new password>

  ALSO ───────────────────────────────────────────────────────────────────────

    · noindex: X-Robots-Tag, robots.txt Disallow: / and the crawler block
      (blog_public in the database left as it is)
    · 80 production URLs rewritten in 3 tables
    · Files are not copied. For uploads, leave out the folders that hold
      personal data: rsync -a --exclude wp-content/debug.log --exclude
      wp-content/uploads/wc-logs --exclude wp-content/uploads/gravity_forms
      --exclude wp-content/updraft --exclude wp-content/ai1wm-backups
      --exclude '*.sql' --exclude '*.sql.gz' --exclude '*.zip'
      '<production>/' /srv/staging/
    · Basic auth is off: --basic-auth <user> writes snippets for Apache,
      nginx and Caddy (Roadie never edits server config)
    · Object caches and search indexes (Redis, Algolia, ElasticPress) are
      not copied or cleared

  → signed report: /home/ann/shop/.roadie/reports/2026-10-28T19-07-34Z-copy-staging/report.html
  → next: rhythm echo up, then rhythm soundcheck run https://staging.shop.test
```

Before the import started, Roadie wrote the kill switch into the site's files: `WP_ENVIRONMENT_TYPE` and `DISABLE_WP_CRON` in `wp-config.php` with eight new salts, the Echo connector in seal mode and its own `roadie-staging.php` mu-plugin. Nothing could send while the rows went in. [Kill switch](Roadie-Kill-Switch) shows each change and how to undo it for a test.

In CI, or anywhere without a terminal to answer on, pass `--confirm-target staging.shop.test --yes`; without it the copy stops with `confirm-target` (exit 2, a usage error) before anything is written.

## 9. Start Echo and log in

**Echo.** The copy's mail goes to Echo's seal at `http://127.0.0.1:8025`, as the screen says. Run `rhythm echo up` on the staging host (see [Echo: your first run](Echo-First-Run)). Until Echo runs, mail from the copy fails closed: nothing is sent and nothing is caught. Echo running elsewhere: `--echo-url` (or `side_effects.echo.url` in the config) tells the connector where.

**Logging in.** Every user's password is replaced with a hash nothing matches, so nobody can log in with a production password. Either set one for the fake login the screen names (`wp user update user_ec55e2810c05a5ab --user_pass=...`), or keep your own accounts as they are with `--keep-users` (globs over logins and emails):

```sh
rhythm roadie copy --to keep.sql --keep-users shopowner
```

```text
 ╭────────────────────────────────────────────────────╮
 │  PASS   staging.shop.test is a pseudonymised copy  │
 │  0 critical · 0 high · 0 medium · 0 low · 1 info   │
 ╰────────────────────────────────────────────────────╯
  166 KB · 13 tables · 0.1s
  · info: 1 kept users left as they are (keep_users)
```

A kept user stays real in every table, password hash included; `--admin-password-env ROADIE_ADMIN_PASSWORD` sets the kept users' password from that variable instead. Put the list in `keep_users:` in the config to keep it for every run.

## Next

- Make a staging that already exists safe: [Copy and prepare](Roadie-Copy-And-Prepare#prepare).
- Copy straight from production over ssh: [Sources and targets](Roadie-Sources-And-Targets).
- Give gateways a test key instead of blanking them: [Live keys](Roadie-Live-Keys).
- Refresh staging every night: [CI](Roadie-CI).

---
<sub>← [Roadie overview](Roadie) · [Home](Home) · [Copy and prepare](Roadie-Copy-And-Prepare) →</sub>
