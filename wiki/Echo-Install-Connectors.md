A connector is the part of Echo that lives inside your site. It hooks the site's mailer and HTTP client, reports every message to Echo before it leaves and obeys Echo's answer. This page covers each connector (WordPress, Laravel, Next.js and Node) plus the generic route for anything else (SMTP, `sendmail` and the HTTPS proxy). For each one it says what it hooks and what it can't see.

```sh
rhythm echo install wordpress /var/www/staging   # writes the mu-plugin
rhythm echo install laravel                      # prints the package and .env lines
rhythm echo install next                         # writes instrumentation.ts, prints the rest
rhythm echo install node
rhythm echo install generic                      # SMTP, sendmail_path, HTTPS_PROXY
```

WordPress is installed for real, as a mu-plugin or as the regular plugin Echo Staging Seal. For the other stacks, `install` prints the package to add and the lines to put in your own files, since those are your project's files to change (`install next` also writes `instrumentation.ts` when there's none). `install next` and `install node` need the project's `package.json`: elsewhere they say `rhythm echo: this folder has no package.json; run this in the Node project, or name its folder` (exit `2`).

## Which connector

| Stack | Connector | Catches | Can't see |
|---|---|---|---|
| **WordPress** | one mu-plugin, `rhythm echo install wordpress`, or the plugin [Echo Staging Seal](#echo-staging-seal) with `--plugin` | every `wp_mail()` (it wins over FluentSMTP and Post SMTP), every `wp_remote_*()` call (SMS plugins, WooCommerce webhooks, Slack, GA4, Meta CAPI, API mailers) | SDKs that call cURL themselves (the Twilio and Stripe PHP SDKs): add [the proxy](#proxy-and-ca-for-raw-curl) |
| **Laravel** | `rhythmsh/echo-laravel` (Composer) | every mailer and transport, the `Http` client, container-built Guzzle clients, Vonage and Slack notifications, queued mail and jobs | `new GuzzleHttp\Client()`, raw cURL, SDKs with their own HTTP stack |
| **Next.js · Node** | `@rhythmsh/echo` (npm) | `fetch` (Node and Edge), `http`/`https` (axios, most SDKs), Nodemailer | a separate `undici` package, `http2`, raw sockets |
| **Anything** | SMTP sink, `rhythm-echo sendmail`, HTTPS proxy | mail and outbound HTTP(S) from any language | traffic that ignores the proxy: use a [hard seal](Echo-Sealing-Staging#hard-seals) |

Every connector switches itself off in production and fails closed: if it can't reach Echo while sealed, it refuses to send rather than let the message out. All three mask credential headers before anything leaves the site, report the files an email attaches (name, type, size and hash, never the content; see [Attachments](Echo-Attachments)) and speak wire version 2, falling back to wire 1 when an older Echo asks for it.

## WordPress

```sh
rhythm echo install wordpress /var/www/staging
```

```text
Installed the Echo connector (wp-content/mu-plugins/rhythm-echo.php), reporting to http://127.0.0.1:8025.

  It runs only where WP_ENVIRONMENT_TYPE is local, development or staging. WordPress
  treats an unset WP_ENVIRONMENT_TYPE as production and there the connector does
  nothing. On staging, add this to wp-config.php, above "That's all, stop editing!":

    define( 'WP_ENVIRONMENT_TYPE', 'staging' );

  (or set it in the server's environment). On a box that isn't production but says
  so, reinstall with --force.

  Next: rhythm echo up, then rhythm echo doctor (with site: in .echo/config.yaml)
  Flush async mail and webhooks after actions: add to .echo/config.yaml
    flush: ["wp cron event run --due-now", "wp action-scheduler run"]
```

When the site's `wp-config.php` already defines `WP_ENVIRONMENT_TYPE` as `local`, `development` or `staging`, the middle paragraph is one line instead: `wp-config.php sets WP_ENVIRONMENT_TYPE to staging, so the connector runs.` With `--force` it's `Forced on: it runs even where WordPress says production.` Through `--wp-cli` or `--container` Echo can't read `wp-config.php`, so it always prints the paragraph.

The connector is one file, `wp-content/mu-plugins/rhythm-echo.php`, embedded in the `rhythm-echo` binary. Its settings go to `wp-content/mu-plugins/rhythm-echo/settings.php`, a PHP file that returns an array, so the web server never serves the token. mu-plugins load before every plugin and can't be turned off from the admin, which is what lets the connector beat SMTP plugins to `wp_mail()`. It needs PHP 7.4 and WordPress 5.7 or later.

### Where it installs

| Form | Does |
|---|---|
| `install wordpress [path]` | Writes the files into the WordPress at `path` (default `.`): the site root, with `wp-config.php` or `wp-content/`, or the `wp-content` folder itself. Any other folder is a usage error. An existing `rhythm-echo.php` that Echo didn't write is never overwritten. |
| `install wordpress [path] --wp-cli` | Pipes an installer to `wp eval-file - --path=<path>`, for a site you reach through WP-CLI (over SSH, or as the site's user). It writes into `WPMU_PLUGIN_DIR`, so custom layouts work. |
| `install wordpress --container <name>` | Runs the same installer with `docker exec -i <name> wp eval-file - --allow-root`. The container needs WP-CLI; see [WordPress in Docker](#wordpress-in-docker). |
| `install wordpress [path] --uninstall` | Removes the plugin and its settings folder and prints `Removed the Echo connector.` Combine with `--wp-cli` or `--container` to remove it the same way. Prints `The connector isn't installed there.` when there's nothing to remove. |
| `install wordpress [path] --plugin` | Writes [Echo Staging Seal](#echo-staging-seal) into `wp-content/plugins/` instead. With `--uninstall` it removes that plugin. |

### Flags

| Flag | Default | Effect |
|---|---|---|
| `--url <url>` | `RHYTHM_ECHO_URL`, else `listen` from `.echo/config.yaml`, else `http://127.0.0.1:8025` | Echo's address **as the site reaches it**: `http://host.docker.internal:8025` from a container. |
| `--token <t>` | `RHYTHM_ECHO_TOKEN` | Sent as `X-Echo-Token` with every capture. Also locks the connector's REST routes to that token. |
| `--proxy <url>` | none | Echo's HTTPS proxy as the site reaches it, e.g. `http://127.0.0.1:8026`. Raw cURL in each request then goes through it. |
| `--ca <path>` | none | Path, on the site's machine, to Echo's CA certificate (`rhythm echo ca`). Added to the WordPress HTTP API's CA bundle, not replacing it. |
| `--mode seal\|observe` | `seal` | Only decides what happens when Echo **can't be reached**: `seal` fails closed, `observe` lets mail and requests out. When Echo answers, Echo's mode decides. Any other value is a usage error. |
| `--force` | off | Run even when WordPress says this is production. Only for a production-like box that isn't production. |
| `--wp-cli`, `--container <name>`, `--uninstall` | | See above. |
| `--plugin` | off | Install Echo Staging Seal instead of the mu-plugin. Can't be combined with `--wp-cli` or `--container`. |
| `--hub <url>` | | Report to an [agency hub](Echo-Hub-And-Reports#connecting-a-site) with the site's write token from `RHYTHM_ECHO_WRITE_TOKEN`. Needs `--site`; can't be combined with `--url` or `--token`. |
| `--site <id>` | | The site's ID, named in every capture: letters, digits, dots, dashes and underscores. |
| `--bin <path>` | | (Echo Staging Seal) The `rhythm-echo` its buttons and `wp echo` run, on the site's machine. |
| `--project <dir>` | | (Echo Staging Seal) The folder with `.echo/config.yaml` they run in. |

The installer reads `.echo/config.yaml` (or `--config <file>`) for the addresses Echo runs on: `listen` for the default `--url`; `smtp` and `proxy` for the lines the other stacks print. Pass `--url` when the site reaches Echo at another address than the config's, such as `host.docker.internal` from a container.

### Settings precedence

The connector reads each setting from the first place that has it:

1. a `define()` constant in `wp-config.php`: `RHYTHM_ECHO_URL`, `RHYTHM_ECHO_TOKEN`, `RHYTHM_ECHO_WRITE_TOKEN`, `RHYTHM_ECHO_SITE_ID`, `RHYTHM_ECHO_MODE`, `RHYTHM_ECHO_FORCE`, `RHYTHM_ECHO_PROXY`, `RHYTHM_ECHO_CA`, `RHYTHM_ECHO_BIN`, `RHYTHM_ECHO_PROJECT`;
2. the same names in the environment;
3. `mu-plugins/rhythm-echo/settings.php`, written by `install` (Echo Staging Seal reads the same file, then its own settings screen);
4. the defaults: Echo at `http://127.0.0.1:8025`, sealed, not forced, no proxy.

So a host's environment or `wp-config.php` can override what the installer wrote, without editing the plugin.

### The production guard

On a production site the connector installs no hooks at all: mail and requests go out exactly as without it. "Production" is `wp_get_environment_type()`, which is `production` when **`WP_ENVIRONMENT_TYPE` isn't set**. Set it on every staging copy:

```php
define( 'WP_ENVIRONMENT_TYPE', 'staging' );   // or development, or local
```

While it's off, administrators see a notice: *Rhythm Echo is installed but not running: WP_ENVIRONMENT_TYPE is not set and WordPress treats that as production. Echo only runs on test sites. Set WP_ENVIRONMENT_TYPE to staging, development or local on a test copy, or remove `wp-content/mu-plugins/rhythm-echo.php` (run `rhythm echo install wordpress --uninstall`).* `rhythm echo doctor` reports it as `the connector is installed but off: production`. `RHYTHM_ECHO_FORCE` (or `--force`) overrides the guard.

To remove the connector, run `rhythm echo install wordpress <path> --uninstall`, or delete `wp-content/mu-plugins/rhythm-echo.php` and the `rhythm-echo/` folder next to it.

### What it hooks

| Hook | Catches |
|---|---|
| its own `wp_mail()` | Every email WordPress, a plugin or a theme sends. Sealed, the message is reported and not sent; the caller gets `true`. Observed, it's sent the way core sends it, with core's filters and actions. The outcome is reported. |
| `pre_wp_mail`, `phpmailer_init` | A fallback for when something defined `wp_mail()` first. Echo still catches mail sent through those hooks. `doctor` shows the warning: *wp_mail() was already defined (…) before Rhythm Echo loaded…* |
| `pre_http_request` | Every `wp_remote_*()` call, before it's sent: API mailers (SendGrid, Mailgun, Postmark and others), SMS plugins, WooCommerce webhooks, Slack, GA4, Meta CAPI, CRMs. Echo answers *allow*, *fake* (a provider-shaped response) or *block* (a `WP_Error`). |
| `http_api_debug` | The response of each allowed request, so observe mode records whether it worked. |
| cron and Action Scheduler | The run ID of the request that scheduled an event or action is stored with it and restored when it runs, so queued mail and webhooks are tied to the action that caused them. |
| downloads | A request with `stream => true` (plugin and theme updates, `download_url()`, backups) that Echo would fake fails instead, with `Rhythm Echo: sealed by Echo, so <host> was not downloaded. Echo answers for services, not with files.` No file is written. It's still recorded. |

The site's own host (cron and Action Scheduler loopbacks) and Echo itself are never reported. Platform update checks to `api.wordpress.org` and friends pass through without being recorded; see [Sealing staging](Echo-Sealing-Staging#what-passes-through).

### Proxy and CA for raw cURL

The Twilio and Stripe PHP SDKs and some mailers call cURL directly, so `pre_http_request` never sees them. With `--proxy`, the connector sets `https_proxy` and `http_proxy` (and `no_proxy` for the site, Echo and loopback) at the start of each request, which libcurl reads for every handle without its own proxy. With `--ca`, Echo's CA is appended to the WordPress HTTP API's bundle so requests that reach the decrypting proxy verify:

```sh
rhythm echo up --proxy 127.0.0.1:8026 --mitm
rhythm echo install wordpress /var/www/staging --proxy http://127.0.0.1:8026 --ca "$(rhythm echo ca)"
```

`doctor` then lists `proxy` among the layers. SDKs that ship their own CA bundle need more; see [FAQ](Echo-FAQ#an-sdk-rejects-echos-certificate).

### Fail closed

When sealed and Echo can't be reached, `wp_mail()` returns `false` and fires `wp_mail_failed`; `wp_remote_*()` returns a `WP_Error`. The PHP error log says why:

```text
Rhythm Echo: email http://host.docker.internal:8025/v1/capture failed (Failed to connect to host.docker.internal port 8025 after 0 ms: Could not connect to server); sealed, so it was not sent
```

The request's error message is `Rhythm Echo could not be reached and this site is sealed, so the request was not sent.`

### The ping and canary routes

The connector adds two REST routes for `rhythm echo doctor`:

| Route | Method | Does |
|---|---|---|
| `/wp-json/rhythm-echo/v1/ping` (or `?rest_route=/rhythm-echo/v1/ping`) | GET | Whether the connector is active and why not, the environment, the email layer, the proxy, warnings and whether the site can reach Echo. |
| `/wp-json/rhythm-echo/v1/canary` | POST | Sends one `wp_mail()` to `canary@echo.invalid` and one `wp_remote_post()` to `https://canary.echo.invalid/`, so `doctor` can check both are caught. `.invalid` addresses can't be delivered even when unsealed. |

The routes exist only while the connector is active: on a production site they answer `404`. With a token configured, they need `X-Echo-Token` or a logged-in administrator. Without one, an administrator or a request straight from the same machine may call them: from loopback, with no `X-Forwarded-For`, `Forwarded` or `X-Real-IP` header, so a reverse proxy or Docker's gateway doesn't count. Anyone else gets `403`:

```json
{"code":"rhythm_echo_forbidden","message":"Rhythm Echo: call from this machine without a proxy, or set a token (rhythm echo install wordpress --token).","data":{"status":403}}
```

With a token set, the message is `Rhythm Echo: send X-Echo-Token with the token in Rhythm Echo's settings.`

### WordPress in Docker

The official `wordpress:*-apache` images don't include WP-CLI, so `--container` on the web container fails with `running wp-cli in shop-wp: exit status 127 (is wp-cli installed in the container? or copy the files with docker cp)`. Two other failures get their own hint: a container that doesn't exist (`no container named shop-cli; see: docker ps`) and a permission error (`wp-cli's user in the container can't write wp-content/mu-plugins; give that folder to it, or copy the files with docker cp`). Use a `wordpress:cli` container on the same volume, as the web server's user, with the same database and `WORDPRESS_CONFIG_EXTRA` environment as the web container (otherwise WP-CLI sees a different environment type):

```sh
docker run -d --name shop-cli --user 33:33 --volumes-from shop-wp --network shop-net \
  -e WORDPRESS_DB_HOST=db -e WORDPRESS_DB_USER=wp -e WORDPRESS_DB_PASSWORD -e WORDPRESS_DB_NAME=wp \
  -e "WORDPRESS_CONFIG_EXTRA=define('WP_ENVIRONMENT_TYPE','staging');" \
  --add-host host.docker.internal:host-gateway --entrypoint sleep wordpress:cli infinity

rhythm echo install wordpress --container shop-cli --url http://host.docker.internal:8025 --token "$RHYTHM_ECHO_TOKEN"
```

```text
Installed /var/www/html/wp-content/mu-plugins/rhythm-echo.php
Installed the Echo connector (wp-content/mu-plugins/rhythm-echo.php), reporting to http://host.docker.internal:8025.
```

The `WP_ENVIRONMENT_TYPE` paragraph and the `Next:` lines follow. If `--url` points at `127.0.0.1` or `localhost`, a last line warns: `In Docker, 127.0.0.1 and localhost are the container itself: reinstall with --url http://host.docker.internal:8025`.

Echo must listen where the container can reach it, which means beyond loopback and therefore with a token; see [FAQ](Echo-FAQ#the-site-runs-in-docker). Without `--user 33:33`, the `wordpress:cli` image can't create `wp-content/mu-plugins` in the Apache image's volume.

### Echo Staging Seal

Echo Staging Seal is the same connector as a regular WordPress plugin, for sites where you'd rather see it in **Plugins**, or where a host doesn't let you add mu-plugins. It adds a banner, a page in **Tools** and WP-CLI commands on top of the seal.

```sh
rhythm echo install wordpress /var/www/staging --plugin --site bean-leaf --bin /usr/local/bin/rhythm-echo --project /home/you/projects/shop
```

```text
Installed Echo Staging Seal (wp-content/plugins/echo-staging-seal), reporting to http://127.0.0.1:8025.
  It names itself bean-leaf in every capture.
  Activate it: wp plugin activate echo-staging-seal (or in Plugins in wp-admin).
```

When activated, it moves itself to the front of the plugin list so it hooks in before other plugins send. It needs WordPress 5.7 and PHP 7.4 and is MIT licensed, which is GPL-compatible.

- **The admin bar** says *This is a sealed copy* (green) while Echo seals, or *Echo is observing: sends go out* (amber), to every logged-in user. Nothing shows on production.
- **Tools → Echo** shows the seal's status, a table of the sends Echo stopped per plugin and channel (faked or blocked) and the settings: Echo's URL, the token, the site ID and what to do when Echo is down. The token is saved exactly as typed. One set in `wp-config.php` or the environment wins; the URL field is then read-only.
- **Snapshot before updating** and **Compare since last update** run `rhythm-echo snapshot --name before-updates` and `rhythm-echo compare --snapshot before-updates` on the server, when `RHYTHM_ECHO_BIN` (the binary) and `RHYTHM_ECHO_PROJECT` (the folder with `.echo/config.yaml`) are set in `wp-config.php`, the environment or the installer's settings file. They're never set from the admin screen, so an admin account alone can't choose a program to run. The commands run without a shell, with fixed arguments.

`wp echo` does the same from WP-CLI:

```sh
wp --path=/var/www/staging echo status
```

```text
Seal:        on: nothing leaves this site
Echo:        http://127.0.0.1:8025 (reachable, seal)
rhythm-echo: /usr/local/bin/rhythm-echo

NAME                     ACTIONS  TAKEN        SENT
before-update            2        5m ago       3 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events
from-main                2        5m ago       3 emails, 1 SMS, 1 chat message, 2 webhooks, 2 analytics events  (latest)
```

```sh
wp echo snapshot before-updates && wp plugin update --all && wp echo compare --snapshot=before-updates
```

If the mu-plugin is installed too, the mu-plugin runs the seal and the plugin only adds its screens and commands. Deleting the plugin removes its settings, cached values and each user's last command output, on every site of a multisite; it leaves the mu-plugin's settings file.

## Laravel

```sh
rhythm echo install laravel
```

```text
Laravel: add the package and point it at Echo.

  composer require rhythmsh/echo-laravel

It's a regular dependency, not --dev: staging deploys usually run composer install
--no-dev, which would leave it out. It does nothing where APP_ENV is production or
testing (RHYTHM_ECHO_FORCE=true overrides that, only for a box that isn't production).

Then in .env on staging or your machine:

  RHYTHM_ECHO_URL=http://127.0.0.1:8025
  QUEUE_CONNECTION=sync          # or flush with: php artisan queue:work --stop-when-empty

Check it: php artisan echo:status, then: rhythm echo doctor
Packages that skip Laravel's mailer and Http client (raw Guzzle, SDKs) are caught
through Echo's proxy: rhythm echo up --proxy 127.0.0.1:8026 --mitm, then set
HTTPS_PROXY=http://127.0.0.1:8026 and trust: /home/you/.config/rhythm/echo/ca/ca.pem
```

The package supports Laravel 11, 12 and 13 on PHP 8.2 or later. Its service provider is auto-discovered. With a token, a `RHYTHM_ECHO_TOKEN=<the token you started Echo with>` line is added under `RHYTHM_ECHO_URL`. Before the CA exists, the last line adds `(rhythm echo ca creates it)`.

### Environment

Everything is set from the environment. To have the file, publish it with `php artisan vendor:publish --tag=rhythm-echo` (`config/rhythm-echo.php`).

| Key | Env | Default | Effect |
|---|---|---|---|
| `enabled` | `RHYTHM_ECHO_ENABLED` | unset | Unset means on in every environment except `production` and `testing`. Set `true` to capture in your test suite. |
| `url` | `RHYTHM_ECHO_URL` | `http://127.0.0.1:8025` | Where Echo listens. |
| `token` | `RHYTHM_ECHO_TOKEN` | none | Sent as `X-Echo-Token`, needed when Echo listens beyond loopback. |
| `force` | `RHYTHM_ECHO_FORCE` | `false` | Run even when `APP_ENV=production`. |
| `mode` | `RHYTHM_ECHO_MODE` | `seal` | What to assume when Echo can't be reached: `seal` fails closed, `observe` lets traffic through. |
| `timeout` | `RHYTHM_ECHO_TIMEOUT` | `2` | Seconds to wait for Echo. |
| `guzzle` | `RHYTHM_ECHO_GUZZLE` | `true` | Also cover Guzzle clients the container builds and the Vonage client. |
| `site_id` | `RHYTHM_ECHO_SITE_ID` | none | The site's ID on an [agency hub](Echo-Hub-And-Reports), put on every capture. |
| `write_token` | `RHYTHM_ECHO_WRITE_TOKEN` | none | The site's write token on a hub, sent instead of `token`. |

The `testing` environment is off by default, so your own test suite keeps its `Mail::fake()` and `Http::fake()`. The site's own host is the host of `APP_URL`, never the request's `Host` header, which a visitor could set to a provider's host.

### What it hooks

| Path | Covered |
|---|---|
| Mailables, `Mail::raw`, mail notifications, queued mail, every mailer and transport, failover and round-robin | yes: each mailer's transport is wrapped, so the capture is the finished message |
| `Http::` facade, `Http::pool`, packages built on it (the Slack Web API channel) | yes: `Http::globalMiddleware` |
| Guzzle clients built by the container (a type-hinted `GuzzleHttp\Client`, the Slack webhook channel) | yes, with `guzzle` on |
| Vonage SMS notifications | yes, with `guzzle` on |
| `new GuzzleHttp\Client()`, raw cURL, SDKs with their own HTTP stack (Twilio and Stripe PHP SDKs, AWS SDK) | **no**: use the proxy |

Sealed, mail is kept and the app sees a normal send; a sealed HTTP call gets Echo's response (`fake`) or a `ConnectionException` (`block`). Observed, the real transport sends. The outcome is reported against the same message. If Echo can't be reached while sealed, mail fails with a `TransportException` and HTTP with a `ConnectionException`, both ending `; sealed, so the mail was not sent. Check the site reaches Echo: php artisan echo:status, or rhythm echo doctor` (`…so the request was not sent…` for HTTP). The `array` and `log` mailers keep working. Credential headers are masked by name before they leave (`Authorization: Bearer •••••`); a Stripe key keeps only its mode prefix (`Bearer sk_test_•••••`), so Echo can fake a test-mode call.

For clients the package can't see, run the worker or server through the proxy and trust Echo's CA in `openssl.cafile` or `curl.cainfo`:

```sh
HTTPS_PROXY=http://127.0.0.1:8026 HTTP_PROXY=http://127.0.0.1:8026 php artisan queue:work
```

Without the package at all, point the mailer at the SMTP sink: `MAIL_MAILER=smtp MAIL_HOST=127.0.0.1 MAIL_PORT=1025`, then `php artisan config:clear`.

### Queues and runs

The run ID comes from the request's `X-Echo-Run` header or cookie, then from the [Context](https://laravel.com/docs/context) a queued job carried over from that request, then from `RHYTHM_ECHO_RUN` (which Echo sets on the commands it runs). A global middleware stores it as a hidden Context value, so queued mail, notifications and jobs keep it with no change to your code. Which run values the connector trusts is in [Run IDs and signatures](#run-ids-and-signatures).

Queued work is only sent when a worker runs it. For staging checks, either set `QUEUE_CONNECTION=sync`, or keep the queue and flush it after each action:

```yaml
# .echo/config.yaml
flush: ["php artisan queue:work --stop-when-empty"]
```

### Commands

- `php artisan echo:status [--json]`: whether the connector is active and why not, Echo's URL and mode and a ping of Echo. Exits non-zero when it's off or Echo can't be reached.
- `php artisan echo:canary [--json]`: sends one mail to `canary@echo.invalid` and one `POST https://canary.echo.invalid/` through the default mailer and the `Http` facade. `rhythm echo doctor`, run from the Laravel project (where `artisan` is), calls it and checks both arrive.

## Next.js and Node

### Next.js

```sh
npm install --save-dev @rhythmsh/echo
rhythm echo install next
```

```text
Next.js: add the package.

  npm install --save-dev @rhythmsh/echo

Wrote instrumentation.ts, which starts Echo's connector (it stays off in production).

Wrap your next.config so mail libraries run outside the bundle and get captured:

  import { withEcho } from '@rhythmsh/echo/next';
  export default withEcho(nextConfig);

Environment (.env.local, or the Preview environment on Vercel):

  RHYTHM_ECHO_URL=http://127.0.0.1:8025

Then: rhythm echo up and rhythm echo watch while you use the app.
```

`install next [dir]` needs the project folder (default `.`, with a `package.json`; otherwise `rhythm echo: this folder has no package.json; run this in the Next.js project, or name its folder`, exit `2`). It writes `instrumentation.ts`, or `src/instrumentation.ts` when the app lives in `src/app`, unless one exists (`.ts` or `.js`, at the root or in `src/`). The file it writes starts the connector:

```ts
// instrumentation.ts
export async function register() {
  const { register } = await import('@rhythmsh/echo/next');
  await register();
}
```

Run again, it sees the file already loads `@rhythmsh/echo` and says `instrumentation.ts already starts Echo's connector; nothing to add there.` A file of your own that doesn't load it gets the lines to add instead:

```text
You already have instrumentation.ts; add this inside its register(), which must be async:

  const { register: registerEcho } = await import('@rhythmsh/echo/next');
  await registerEcho();
```

`register()` runs in both runtimes: on **Node.js** it hooks `fetch`, `http`/`https` and Nodemailer; on **Edge** it hooks `fetch`. Next.js bundles server dependencies. The connector can't reach a bundled Nodemailer, so `withEcho` adds `nodemailer` to `serverExternalPackages` and changes nothing else. On Next.js 14, add `nodemailer` to `experimental.serverComponentsExternalPackages` yourself.

Route handlers, server actions and dynamic pages get the run ID automatically. In middleware and `after()` callbacks, wrap the work in `withEchoRun(request, fn)`.

### Express, Fastify and other Node servers

Load the connector before the app:

```sh
node --import @rhythmsh/echo/register server.js   # ESM or CJS, Node 18.19 or later
node -r @rhythmsh/echo/register server.js         # CJS
```

or make `import '@rhythmsh/echo/register'` the app's first import. To tie captures to the request that caused them, add the middleware, or wrap a handler with `withEchoRun(req, () => handle(req))` in other frameworks:

```js
import express from 'express';
import { echoMiddleware } from '@rhythmsh/echo';

const app = express();
app.use(echoMiddleware());
```

Scripts and cron jobs run with `node --import @rhythmsh/echo/register script.js` and pick up `RHYTHM_ECHO_RUN`; a queue worker passes the run along itself with `withEchoRun(job.data.echoRun, () => process(job))`. In serverless functions and short scripts, `await flush()` before exiting.

### Environment

| Variable | Default | Effect |
|---|---|---|
| `RHYTHM_ECHO_URL` | `http://127.0.0.1:8025` | Echo's capture API. On a Vercel preview, a URL that reaches your Echo, such as a tunnel. |
| `RHYTHM_ECHO_TOKEN` | none | Sent as `X-Echo-Token`. |
| `RHYTHM_ECHO_FORCE` | off | `1` turns the connector on where the production guard says no. |
| `RHYTHM_ECHO_SITE` | none | The site's own hosts or URLs, comma separated. `VERCEL_URL`, `VERCEL_BRANCH_URL` and `NEXT_PUBLIC_SITE_URL` count too. Calls to them are captured and Echo lets them through as sent. |
| `RHYTHM_ECHO_RUN` | none | `<run>/<action>` for work outside a request. Echo sets it on the commands it runs. |
| `RHYTHM_ECHO_SITE_ID`, `RHYTHM_ECHO_WRITE_TOKEN` | none | The site's ID and write token on an [agency hub](Echo-Hub-And-Reports). |

`register(options)` takes the same settings in code (`url`, `token`, `force`, `site`, `siteId`, `writeToken`) plus `maxBody` (default 256 KiB), `timeout` (milliseconds, default 5000), `logger` and `captureLoopback`. The site's host comes only from this configuration, never from the incoming request's `Host` or `X-Forwarded-Host`, which a visitor can set.

```sh
rhythm echo install node
```

```text
Node: add the package and load it before your app.

  npm install --save-dev @rhythmsh/echo   # or a regular dependency if staging installs without dev deps
  node --import @rhythmsh/echo/register server.js      # or: import '@rhythmsh/echo/register'

Environment, on staging or your machine only:

  RHYTHM_ECHO_URL=http://127.0.0.1:8025

The connector is off on VERCEL_ENV=production and on NODE_ENV=production unless
RHYTHM_ECHO_URL is set: setting it there turns the connector on.

Express/Fastify: app.use(echoMiddleware()) ties messages to the request that caused them.
```

### The production guard

The connector stays inert, with no hooks installed, when any of these applies (first match wins; `RHYTHM_ECHO_FORCE=1` overrides them all):

| Environment | Connector |
|---|---|
| `VERCEL_ENV=production` | off |
| `next build` | off, so static generation is never blocked |
| `NODE_ENV=production` outside Vercel | off, even with `RHYTHM_ECHO_URL` set: `next start` on your own staging server needs `RHYTHM_ECHO_FORCE=1` |
| `NODE_ENV` unset outside Vercel | off, unless `RHYTHM_ECHO_URL` is set |
| `NODE_ENV` of `development`, `test`, `staging`, `preview` or `local` | on |
| any other `NODE_ENV`, such as `prod` | off, with one warning: `[rhythm-echo] NODE_ENV=prod is not one of development, test, staging, preview, local or production: treating it as production and staying off` |
| `VERCEL_ENV` of `preview` or `development` | on, including `next start` on a preview |

When it stays off although a `RHYTHM_ECHO_*` variable points it at Echo, it logs why once, so a staging server that forgot `RHYTHM_ECHO_FORCE` doesn't send real mail without a hint: `[rhythm-echo] off: NODE_ENV=production (set RHYTHM_ECHO_FORCE=1 only on a server that is not production)`. A Vercel production deploy and `next build` stay quiet. When active, it logs one line per runtime, such as `[rhythm-echo] active (VERCEL_ENV=preview, nodejs): capturing fetch, http, https, nodemailer via …`. Sealed and unable to reach Echo, requests fail as network errors and Nodemailer rejects.

### Codes and links in Playwright and Cypress

Login, sign-up and checkout tests need the one-time code or magic link the site just sent. Echo caught that message, so the package asks it (`GET /v1/messages/latest`):

```ts
// Playwright, or any Node test runner
import { echo } from '@rhythmsh/echo/playwright';

await page.getByRole('button', { name: 'Send code' }).click();
const { code, link } = await echo.latest({ to: 'ann@example.test', channel: 'email', wait: '10s' });
await page.getByLabel('Code').fill(code!);
```

```ts
// cypress/support/e2e.ts
import '@rhythmsh/echo/cypress';

// in a test
cy.echoLatest({ to: '+15551230000', channel: 'sms' }).its('code').then((code) => cy.get('#otp').type(code));
```

`latest({ to, channel, run, action, since, q, wait })` returns `{ codes, links, message }`, plus `code` (the first code) and `link` (the first link's URL). `wait` is a Go duration (`'10s'`, `'1m30s'`) or milliseconds, at most a minute. When nothing arrives in time it throws with Echo's reason, so the test fails where the message went missing. Echo's address and token come from `RHYTHM_ECHO_URL` and `RHYTHM_ECHO_TOKEN`; in Cypress from `CYPRESS_RHYTHM_ECHO_URL` and `CYPRESS_RHYTHM_ECHO_TOKEN`. For shell scripts, `rhythm echo latest` does the same; see [CLI reference](Echo-CLI-Reference#latest).

### What it can't see

A separately installed `undici` package's `request()` or `fetch()`, `http2`, raw `net` and `tls` sockets, a hand-built `ClientRequest` with `createConnection` and a bundled Nodemailer without `withEcho`. Cover them with the proxy:

```sh
NODE_USE_ENV_PROXY=1 HTTPS_PROXY=http://127.0.0.1:8026 NODE_EXTRA_CA_CERTS="$(rhythm echo ca)" next start
```

`NODE_USE_ENV_PROXY` covers `fetch` from Node 22.21 and 24.0. It covers `http`/`https` from Node 22.21 and 24.5.

## Any other stack

```sh
rhythm echo install generic
```

```text
Any stack: point mail and outbound HTTPS at Echo.

  Mail (SMTP):      host 127.0.0.1, port 1025, any username and password
  Mail (PHP):       sendmail_path = "/usr/local/bin/rhythm-echo sendmail -t -i"   (php.ini or php-fpm pool)
  HTTP(S):          rhythm echo up --proxy 127.0.0.1:8026 --mitm
                    HTTPS_PROXY=http://127.0.0.1:8026  https_proxy=http://127.0.0.1:8026
                    and trust Echo's CA: /home/you/.config/rhythm/echo/ca/ca.pem
                    (curl, PHP cURL, Python requests and Go honor these; Node needs
                    NODE_USE_ENV_PROXY=1 and NODE_EXTRA_CA_CERTS on 22.21+/24.5+)
  Tie to actions:   send X-Echo-Run: <run>/<action> on the request that triggers the work,
                    or set RHYTHM_ECHO_RUN for scripts and workers.

For a seal that can't be bypassed, see docs/seal in the Echo repository: the app on an
internal Docker network or behind a firewall, with Echo as its only way out.
```

`sendmail_path` shows the full path of the `rhythm-echo` binary that printed it, since php-fpm runs without a `PATH`; a path with spaces or shell characters is quoted. The addresses come from `.echo/config.yaml`. When the sink isn't on `127.0.0.1:1025`, the mail lines tell the stand-in where it is:

```text
  Mail (SMTP):      host 127.0.0.1, port 11025, any username and password
  Mail (PHP):       sendmail_path = "RHYTHM_ECHO_SMTP=127.0.0.1:11025 /usr/local/bin/rhythm-echo sendmail -t -i"
                    (php.ini or php-fpm pool; Echo's SMTP sink isn't on 127.0.0.1:1025)
  Mail (scripts):   RHYTHM_ECHO_SMTP=127.0.0.1:11025 in the environment of cron jobs and scripts
```

When the SMTP sink listens beyond loopback, the first line says `any username, Echo's token as the password` and a line adds that php-fpm clears the environment, so the token goes in the pool as `env[RHYTHM_ECHO_TOKEN]`. On the `php -d` command line, quote the whole value: `-d 'sendmail_path="RHYTHM_ECHO_SMTP=… /path/rhythm-echo sendmail -t -i"'`. `install` doesn't create the CA: `rhythm echo ca`, or `up` with `--mitm`, does.

### SMTP

Point any mailer at `127.0.0.1:1025`. On loopback the sink accepts any login, or none (so a site set up for a real SMTP provider works unchanged). Beyond loopback, which `up` only allows with a token, it asks for AUTH PLAIN with any username and the token as the password. It refuses mail until then (`530 Authentication required: log in with any username and Echo's token as the password`). It takes up to 500 recipients and 25 MB per message and never delivers anything. It records every message as `faked`, in observe mode too. Envelope recipients that aren't in `To` or `Cc` are recorded as `Bcc`. An `X-Echo-Run` header in the message ties it to an action; otherwise it belongs to whichever action is running. Try it with curl:

```sh
curl smtp://127.0.0.1:1025 --mail-from shop@shop.test --mail-rcpt jamie@shop.test --upload-file mail.txt
```

```text
08:16:19 email to jamie@shop.test: Your order #1042 [faked]
```

### The sendmail stand-in

PHP's `mail()`, cron and most Unix tools call `sendmail`. `rhythm-echo sendmail` reads a message on stdin like sendmail does and hands it to Echo:

```ini
; php.ini or the php-fpm pool; use the full path, php-fpm has no PATH
sendmail_path = "/usr/local/bin/rhythm-echo sendmail -t -i"
```

| Flag | Meaning |
|---|---|
| `-t` | Recipients from `To`, `Cc` and `Bcc`; `Bcc` is removed from the message and recorded as Bcc. |
| `-i`, `-oi` | A line with a single dot doesn't end the message. |
| `-f <addr>`, `-r <addr>` | The envelope sender. |
| `-F <name>` | The sender's full name, when the message has no `From`. |
| `-o…`, `-B`, `-N`, `-R`, `-V`, `-X`, `-L`, `-v`, `-U`, `-bm`, `-em`, `-ep`, `-eq`, `-G` | Accepted and ignored, so callers that pass them keep working. |
| other `-b…` modes | Refused: only sending is supported. |

It adds `Date` and `Message-ID` when they're missing. It adds `X-Echo-Run` from `RHYTHM_ECHO_RUN` when the message has none (a header already in the message wins), so mail from a `command` action is tied to it. It sends to the first of these that's set:

1. `RHYTHM_ECHO_SMTP`: the SMTP sink, `host:port`;
2. `RHYTHM_ECHO_URL`: the sink Echo's `/v1/health` reports there, else the capture API itself;
3. `smtp:` in the nearest `.echo/config.yaml`, from the working folder up (its `listen:` when `smtp: off`);
4. `127.0.0.1:1025`.

It logs in with `RHYTHM_ECHO_TOKEN` when it's set. When delivery fails and the address came from the config or the default, a second line says where it came from (`rhythm-echo sendmail: Echo's address came from <path>`). Like the sink's own mail, it's recorded as `faked` in every mode. It refuses to run when `WP_ENVIRONMENT_TYPE`, `APP_ENV`, `VERCEL_ENV` or `NODE_ENV` says `production`, unless `RHYTHM_ECHO_FORCE=1`. `rhythm echo sendmail --help` prints all of this.

Its exit codes are sendmail's, which callers understand:

| Code | Meaning | Example |
|---|---|---|
| `0` | Handed to Echo | |
| `64` | Usage | `no recipients (give them as arguments, or use -t)`, `mode -bs is not supported (only sending, -bm)` |
| `65` | Bad message | headers that can't be read, over 25 MB |
| `69` | Echo unreachable | ``can't reach Echo at 127.0.0.1:1025 (is `rhythm echo up` running? set RHYTHM_ECHO_SMTP): … connection refused`` |
| `75` | Temporary failure | the sink or API refused the message |
| `78` | Refused on production | `refusing to run: APP_ENV says production (set RHYTHM_ECHO_FORCE=1 to force)` |

### The HTTPS proxy

For anything that sends HTTP itself, run Echo with its proxy and point the app at it:

```sh
rhythm echo up --proxy 127.0.0.1:8026 --mitm
export HTTPS_PROXY=http://127.0.0.1:8026 https_proxy=http://127.0.0.1:8026
```

With `--mitm`, the proxy decrypts HTTPS with a certificate from Echo's own CA, so it can record and fake each request; the app must trust that CA. `rhythm echo ca` prints its path and `rhythm echo ca --pem` the certificate. Containers can fetch it from `http://<proxy>/ca.pem`. Without `--mitm`, sealed HTTPS to unknown hosts is refused outright, since it can't be answered. Payment APIs are never tunnelled blind, in any mode and even when allowlisted. A live gateway host (live PayPal, Adyen, Braintree, Square, Authorize.net) is refused without `--mitm`, recorded as `blocked` with `a live <Name> API is never reached from staging; use the <Name> sandbox`. Stripe, Mollie and Razorpay use one host for both modes, so without `--mitm` they're refused with `payment APIs need --mitm so Echo can tell test from live keys`; with `--mitm` they're decrypted so Echo can check the key. Sandbox hosts tunnel as usual. How each runtime picks up the proxy and the CA:

| Client | Proxy | Trust Echo's CA |
|---|---|---|
| curl | `HTTPS_PROXY`, `http_proxy` (lowercase for plain HTTP) | `--cacert bundle.pem`, or `CURL_CA_BUNDLE` |
| PHP cURL (Twilio SDK, Guzzle) | `https_proxy` or `HTTPS_PROXY`; for php-fpm, `env[]` in the pool | `curl.cainfo` and `openssl.cafile`, or `CURLOPT_CAINFO` |
| WordPress `wp_remote_*` | `WP_PROXY_HOST` and `WP_PROXY_PORT` constants, or the connector's `--proxy` | the connector's `--ca` |
| Node 22.21+, 24.5+ (`fetch`, `https`) | `NODE_USE_ENV_PROXY=1` with `HTTPS_PROXY` | `NODE_EXTRA_CA_CERTS=ca.pem` (adds to the defaults) |
| Python `requests` | `HTTPS_PROXY` | `REQUESTS_CA_BUNDLE=bundle.pem` (replaces the defaults) |
| Go `net/http` | `HTTPS_PROXY` | `SSL_CERT_FILE=bundle.pem`, or the system store |
| Debian or Alpine system store | | copy to `/usr/local/share/ca-certificates/rhythm-echo.crt`, then `update-ca-certificates` |

A client that **replaces** its trust store needs a bundle of the system CAs plus Echo's, because hosts on Echo's allowlist (other than payment APIs) are tunnelled to the real server with its real certificate:

```sh
cat /etc/ssl/certs/ca-certificates.crt "$(rhythm echo ca)" > /etc/rhythm-echo/bundle.pem
```

A sealed request through the proxy, faked with Twilio's own response shape:

```sh
curl -x http://127.0.0.1:8026 --cacert "$(rhythm echo ca)" -u AC0000:secret \
  -d 'To=%2B15551230000' -d 'From=%2B15005550006' -d 'Body=Your table for 2 is booked' \
  https://api.twilio.com/2010-04-01/Accounts/AC0000/Messages.json
```

```text
{"sid":"SM2db0469431cb00000000000000000000","status":"queued","to":"+15551230000","from":"+15005550006","body":"Your table for 2 is booked","date_created":"Sat, 24 Oct 2026 02:22:51 +0000","num_segments":"1","error_code":null,"error_message":null}
```

Nothing reached Twilio: the proxy terminated TLS itself and answered. Beyond loopback the proxy requires the Echo token as its password (`http://echo:TOKEN@host:8026`, or `Proxy-Authorization: Bearer TOKEN`), so it's never an open relay. The PHP-specific settings (php-fpm's `clear_env`, `env[]` lines, httpoxy) are in [Sealing staging](Echo-Sealing-Staging#php-fpm).

### Tie messages to actions

Echo ties a message to the action that caused it through `X-Echo-Run: <run>/<action>`. Echo's own actions and Soundcheck send it on every request to the site, as a header and as a cookie; a connector copies it into each capture. Scripts and workers read `RHYTHM_ECHO_RUN` from the environment, which Echo sets on the commands it runs. A message without either still lands in the action that's running when it arrives. See [Actions and compare](Echo-Actions-And-Compare#run-ids).

Writing a connector for another stack is a small JSON protocol; see [API](Echo-API#the-capture-protocol). Check it with the [conformance kit](Echo-Conformance).

## Run IDs and signatures

A visitor could send their own `X-Echo-Run` to staging and slip a message into a snapshot, or take one out. So the connectors don't trust every run value:

| Connector setting | A run value counts when |
|---|---|
| A token (`RHYTHM_ECHO_TOKEN`, or the token in WordPress's settings) | it carries Echo's signature: `<run>/<action>.<sig>`, an HMAC of the run and action with that token. Echo signs the values it sends when its token is known. |
| No token | it arrives straight from the same machine: from loopback, with no `X-Forwarded-For`, `Forwarded` or `X-Real-IP` header. |

`RHYTHM_ECHO_RUN` from the environment is always taken. Captures carry the run without its signature. A value over 512 bytes is ignored whole, never cut. Each ignored value is logged, at most once per reason every 10 minutes. A form post that came through a proxy, to the WordPress test shop without a token, left this in the PHP error log:

```text
Rhythm Echo: ignored an X-Echo-Run from 203.0.113.9: without a token only a direct call from this machine may set the run (set the same token here and in Echo)
```

With a token set, the reasons read `ignored an unsigned X-Echo-Run: with a token set, only runs Echo signed with it count` and `ignored an X-Echo-Run whose signature does not verify (not signed with this token)`. Laravel names `RHYTHM_ECHO_TOKEN` in place of "a token"; Node starts its lines with `[rhythm-echo]`. In practice: a site behind a reverse proxy, Docker's port mapping or Next.js (which gives no peer address) needs the same token as Echo for its messages to be tied to actions. Without the token, its messages still arrive and land in whichever action is open, as plain SMTP mail does.

---
<sub>← [Your first run](Echo-First-Run) · [Home](Home) · [Sealing staging](Echo-Sealing-Staging) →</sub>
