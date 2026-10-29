A copy of production keeps doing what production does: WP-Cron fires renewals, Action Scheduler sends follow-up emails, webhooks post orders to Zapier, a gateway in live mode charges cards, Jetpack talks to WordPress.com. The kill switch turns each of these off on the copy and the exposure guard keeps search engines out. This page lists every switch, what it changes, how verify checks it, how to undo it for a test and what no switch can stop.

`copy` and `prepare` apply the kill switch by default (`--no-kill-switch` copies the data only; verify then fails on side effects). `prepare --only side-effects` puts it back after a plugin update, without touching the data. On a site target the file part goes in **before** the import starts, so nothing can send while the rows go in. Each switch works on its own: removing the Echo connector doesn't turn a gateway or a webhook back on.

## WordPress

| Switch | What Roadie does | Verify checks | To undo it for a test |
|---|---|---|---|
| `config.environment` | `define( 'WP_ENVIRONMENT_TYPE', 'staging' );` in `wp-config.php`, replacing an earlier define in place, else before `/* That's all, stop editing! */` | the constant is `staging` (or `local`, `development`) and defined before `wp-settings.php` | Don't. Echo's connector and Roadie's mu-plugin stay off on a site that says it is production. |
| `side-effect.mail` | the Echo connector in seal mode (`wp-content/mu-plugins/rhythm-echo.php`); WP Mail SMTP, Post SMTP and FluentSMTP switched to PHP's mailer with their credentials blanked, so every `wp_mail()` reaches Echo; without the connector, the mu-plugin blocks `wp_mail()` | the connector is there and unchanged (`side-effect.connector-changed` when it differs from the one Roadie ships); with `--echo`, Echo's seal is on and let nothing through | Run Echo in observe mode, or release one message to a tester from Echo Inbox ([Echo](Echo-Sealing-Staging)). |
| `side-effect.cron` | `define( 'DISABLE_WP_CRON', true );` | the constant is true | Run one event by hand: `wp cron event run <hook>`. |
| `side-effect.action-scheduler` | the mu-plugin turns the async runner off and takes the queue runner off WP-Cron; pending actions are kept (`action_scheduler: cancel` cancels them) | the runner is paused, or nothing is pending | Run one action: `wp action-scheduler run --hooks=<hook>`. |
| `side-effect.webhooks` | each `wc_webhooks` row to `paused`, its secret blanked; the delivery URL kept | no active webhook | Point a webhook at a test endpoint, then set it active. |
| `side-effect.gateways` | Stripe, PayPal Payments, WooPayments, Square, Braintree, Mollie, Razorpay, Klarna and Authorize.net to test or sandbox mode; an enabled gateway Roadie doesn't know gets `enabled = no` | every enabled gateway is in test mode | Add your test keys (`keys.test_keys`) instead of turning live mode on. |
| `side-effect.subscriptions` | `wc_subscriptions_siteurl` left as production's (that is how Subscriptions knows a copy); the mu-plugin returns true for `woocommerce_subscriptions_is_duplicate_site`; `WP_HOME` and `WP_SITEURL` defines rewritten to the staging URL | the filter is there and the option doesn't name this site | Test renewals with Subscriptions' own staging tools; never point the option at the copy. |
| `side-effect.jetpack` | the mu-plugin turns on `jetpack_offline_mode`; Jetpack's tokens blanked | the filter is there | Connect the copy to its own WordPress.com site. |
| `side-effect.automatewoo` | `automatewoo_queue` emptied; its runs go through Action Scheduler, which is paused too | the queue is empty | Run a workflow by hand from AutomateWoo's admin. |
| `config.salts` | the eight salts regenerated, 64 random characters each | in exact mode, they differ from production's | Not needed: everyone logs in again. |
| `config.points-at-production` | nothing: Roadie never guesses the staging's own hosts | `DB_HOST`, `WP_REDIS_HOST`, cache key salts or S3 offload settings equal production's or match `production:` (critical) | Point them at the staging's services by hand. |

Multisite networks also get `DOMAIN_CURRENT_SITE` and `PATH_CURRENT_SITE` set to the staging URL's host and path, in the same step as `WP_HOME` and `WP_SITEURL` ([Multisite](Roadie-Multisite)).

### What it writes

On the shop in [Getting started](Roadie-Getting-Started), the copy into `/srv/staging` changed `wp-config.php` like this (salts shortened here):

```diff
-define( 'AUTH_KEY',         'kV?0x`Z+3mP4]qL!w8Rn.tY6sB2#jH9e' );
...
-define( 'NONCE_SALT',       'Wc9(gN4kR1tV6yZ0bM3qS8xD5fH2jL7p' );
+define( 'AUTH_KEY',         'Ir8u[Yyn8M #L[@$`[=usU- ,r>j:j4].T,813a&dJZm?37^<y&JM:Y&OpeMdc,#' );
...
+define( 'NONCE_SALT',       'A#%$&qF1u{d>8h6;V&$D=%O%XW4/Tv1 w-ou8ny.&OGGu#aTRZc$Q1yCP[jaB@(+' );

 $table_prefix = 'wp_';

 define( 'WP_DEBUG', false );

+define( 'WP_ENVIRONMENT_TYPE', 'staging' );
+define( 'DISABLE_WP_CRON', true );
 /* That's all, stop editing! Happy publishing. */
```

`wp-config.php` is read and edited as text, never run: only the value bytes of a define change, its quotes and spacing stay, line endings and a byte order mark are kept and the result passes `php -l`. It added three files under `wp-content/mu-plugins/`:

```text
wp-content/mu-plugins/rhythm-echo.php
wp-content/mu-plugins/rhythm-echo/settings.php
wp-content/mu-plugins/roadie-staging.php
```

`rhythm-echo.php` is Echo's WordPress connector, embedded in Roadie at the version Roadie was built with (its SHA-256 is in the report), with its settings:

```php
return array(
	'url' => 'http://127.0.0.1:8025',
	'mode' => 'seal',
);
```

`roadie-staging.php` is Roadie's own guard. Its header says how to turn it off:

```php
/*
 * Plugin Name: Roadie staging guard
 * Description: Keeps this pseudonymised staging copy from running production's queues, renewals, mail and search indexing.
 * Roadie (rhythm-roadie 1.0.0) wrote this file. Delete it to turn the guard off.
 */
```

It returns early on a site whose `wp_get_environment_type()` is `production`, then pauses Action Scheduler, forces Subscriptions' staging mode, turns Jetpack offline, blocks `wp_mail()` when the Echo connector is missing, adds the noindex parts below and shows `Pseudonymised staging copy (Roadie)` in the admin bar.

### Echo on the staging host

The connector sends every message to Echo instead of the world. Echo has to run where the site can reach it: the copy's last screen says so.

```text
  Mail from this copy goes to Echo at http://127.0.0.1:8025. Run rhythm echo
  up on the staging host, or the copy's mail will fail closed.
```

Fail closed means a sealed connector that can't reach Echo refuses to send and logs why; nothing gets out. `--echo-url` (or `side_effects.echo.url`, or `RHYTHM_ECHO_URL`) points it elsewhere; an Echo URL must be `https`, loopback or a private address. It never carries a password in the URL (Echo's token goes in `RHYTHM_ECHO_TOKEN`). PHP's own `mail()` reaches Echo only with `sendmail_path` set to `rhythm-echo sendmail -t -i` in `php.ini`; Roadie never edits `php.ini`, so the plan lists it as a step by hand. `--echo-proxy 127.0.0.1:8026` also routes raw cURL through Echo's proxy. Over ssh and aliases the connector goes in with `wp --path=<path> eval-file - --skip-plugins --skip-themes`, so no plugin copied from production runs before it is in place. See [Echo: Sealing staging](Echo-Sealing-Staging).

`verify --echo` asks Echo whether its seal is on and how many messages it let through since the copy. An Echo that isn't running is a critical finding:

```text
 ▌  CRITICAL   verify --echo: Echo isn't reachable at http://127.0.0.1:39027…
 ▌             http://127.0.0.1:39027 1
 ▌             → Start Echo (rhythm echo up), then run verify again.
```

## Laravel

A Laravel target's `.env` gets, order and comments kept:

| Line | Why |
|---|---|
| `APP_ENV=staging`, `APP_URL=<url>` | the app knows it is staging |
| `APP_KEY=base64:...` (a new key) | unless `--keep-app-key`: encrypted columns and production's sessions become unreadable |
| `MAIL_MAILER=smtp`, `MAIL_HOST=127.0.0.1`, `MAIL_PORT=1025`, `MAIL_USERNAME=`, `MAIL_PASSWORD=` | mail goes to Echo's SMTP sink |
| `QUEUE_CONNECTION=sync` | jobs from production were cleared (`jobs`, `failed_jobs`); new ones run in the request, through Echo |
| `RHYTHM_ECHO_URL`, `RHYTHM_ECHO_MODE=seal` | for Echo's Laravel package |
| live keys blanked or swapped | `STRIPE_*`, `AWS_*`, `MAILGUN_*` and the rest |
| `laravel.env` entries | your own extra lines |

`public/robots.txt` disallows every agent. The plan adds two steps by hand: `composer require rhythmsh/echo-laravel --dev` (Roadie never runs Composer) and `Stop queue workers and the scheduler on this host`. See [Laravel](Roadie-Laravel).

## A database target

A DSN target has no files, so Roadie can only change what is in the data: gateways to test mode, webhooks paused, the AutomateWoo queue emptied, keys swapped. The plan says so and verify lists `side-effect.not-applicable` (info):

```text
  · info: This target is a database only: Roadie can't install the Echo seal
  or edit config files. Turn mail and jobs off where the app runs.
```

When the app's files are reachable, copy to the site folder, `ssh://` or `@alias` instead, or run `prepare <site> --only side-effects` on the site after the import.

## Keeping search engines out

Search engines must not index a staging copy. Noindex asks them not to; basic auth keeps them out. Roadie does both parts it can:

- **noindex** (on by default; `--noindex=off` or `noindex: false` turns it off): the mu-plugin adds `noindex, nofollow` to every page and an `X-Robots-Tag` header, turns sitemaps off and serves a `robots.txt` that disallows everything for every agent and names `GPTBot`, `ClaudeBot`, `CCBot`, `Google-Extended`, `Bytespider`, `PerplexityBot`, `Amazonbot` and `Applebot-Extended`. A physical `robots.txt`, which wins over the filter, is backed up to `robots.txt.roadie-backup` and replaced. `blog_public` in the database is left as it is, so pushing this database to production by mistake can't carry noindex there.
- **Basic auth** (off by default): `--basic-auth <user>` writes snippets for Apache, nginx and Caddy into `.roadie/basic-auth/` with a bcrypt hash and prints the generated password once. Roadie never edits server config.

```sh
rhythm roadie prepare /srv/staging --only noindex --basic-auth ann
```

```text
  ALSO ───────────────────────────────────────────────────────────────────────

    · noindex: X-Robots-Tag, robots.txt Disallow: / and the crawler block
      (blog_public in the database left as it is)
    · Basic auth snippets for Apache, nginx and Caddy go in
      .roadie/basic-auth/; add the one for your server (Roadie never edits
      server config)
...
  Basic auth for ann: password ... (shown once; .roadie/basic-auth/ holds only its hash)
```

```text
.roadie/basic-auth/apache.htaccess
.roadie/basic-auth/Caddyfile
.roadie/basic-auth/.htpasswd
.roadie/basic-auth/nginx.conf
```

The nginx snippet:

```nginx
# Roadie basic auth for a staging copy (nginx).
# Copy .htpasswd to /etc/roadie/staging.htpasswd
# (outside the web root), then put these lines in the site's server block
# and reload nginx.
# Loopback requests pass without a password (satisfy any), so WP-Cron
# spawns and Site Health keep working.
# Behind a reverse proxy on this host every request is loopback: remove
# the loopback allow lines there.
satisfy any;
allow 127.0.0.1;
allow ::1;
deny all;
auth_basic "Staging copy";
auth_basic_user_file /etc/roadie/staging.htpasswd;
```

On GitHub Actions the password is masked in the log and kept in `.roadie/basic-auth/password` instead of printed. `verify --site <url>` fetches the staging's `/robots.txt` and home page headers and reports `search.indexable` (medium) and `auth.none` (low) when they let everyone in.

## What no switch can stop

Every plan, copy and report says it:

```text
  ▸ Not stopped   a server crontab calling wp cron or artisan schedule:run,
                  wp action-scheduler run by hand, queue workers already
                  running, code that opens SMTP sockets or uses fsockopen
                  with credentials Roadie doesn't know, raw cURL without
                  --echo-proxy. Credentials are blanked so those paths fail
                  at the provider. Blocking outbound traffic at the host is
                  the only certainty.
```

Roadie works inside the site. A crontab on the server, a queue worker that was already running or code that opens its own sockets is outside it. Blanked credentials make those paths fail at the provider, which covers most of them; for certainty, block outbound traffic from the staging host except to Echo (Echo's [hard seal](Echo-Sealing-Staging#hard-seals)). Uploads, object caches and search indexes aren't touched either: see [Security and limits](Roadie-Security-And-Limits#what-roadie-cant-see).

---
<sub>← [Live keys](Roadie-Live-Keys) · [Home](Home) · [Verify and reports](Roadie-Verify-And-Reports) →</sub>
