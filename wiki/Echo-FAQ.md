Answers to common questions and fixes for common problems with Echo. If yours isn't here, check the [CLI reference](Echo-CLI-Reference) or open an issue (see [the end of this page](#reporting-a-bug)).

## About Echo

### Why not Mailpit?

Mailpit, MailHog, Mailtrap and smtp4dev are good mail catchers, and if all you need is an inbox for one app's SMTP mail, use one. Echo does three things they don't:

- **It compares.** A catcher shows you an inbox. Echo records what each action sent, then tells you after an update that the office's copy of the contact form is gone, the SMS now has `{first_name}` in it, and the Zapier payload lost a field.
- **Every channel, not only SMTP.** Most of what a modern site sends isn't SMTP at all: API mailers (SendGrid, Postmark, Resend), SMS, WhatsApp, Slack, webhooks, GA4 and Meta events, payment calls. Echo catches them through the connector or its proxy, recognizes 34 services, and answers each one the way the service would, so the site carries on.
- **It seals.** Pointing the mailer at a catcher stops email. It doesn't stop the SMS plugin, the WooCommerce webhook to the live CRM, or the purchase event that feeds your ads. Echo's seal covers them, and refuses live payment calls.

Echo's API isn't Mailpit's, so tools built for Mailpit's API don't work with it; see [API](Echo-API).

### Is it safe to run on production?

`check` and `headers` are: they only read DNS and files. Everything else is for staging and local sites. The connectors do nothing in production (`WP_ENVIRONMENT_TYPE`, `APP_ENV`, `NODE_ENV`, `VERCEL_ENV`), Echo's actions refuse URLs you list under `production:`, and `doctor` fails when `site:` is one of them.

### Where did my messages go?

The running catcher keeps messages in memory, the newest 10,000, and forgets them when it stops. What actions sent is saved in `.echo/snapshots/` and `.echo/runs/`, and `rhythm echo report` shows it again. To keep what you captured while clicking around, run an action, or save `rhythm echo ls --json` to a file.

## Connecting a site

### The connector didn't catch a message

Work down this list:

1. **Is the connector on?** Run `rhythm echo doctor`. `the connector is installed but off: production` means the site thinks it's production: on WordPress, set `WP_ENVIRONMENT_TYPE` to `staging`, `development` or `local` (WordPress treats *unset* as production). For Laravel it's `APP_ENV`; for Node, `NODE_ENV=production` outside Vercel without `RHYTHM_ECHO_URL`.
2. **Can it reach Echo?** `active, but it can't reach Echo at …` means the URL it uses is wrong from where the site runs; see [The site runs in Docker](#the-site-runs-in-docker). While Echo is unreachable, a sealed connector refuses to send and logs why.
3. **Does it go through a hook the connector patches?** On WordPress, that's `wp_mail()` and `wp_remote_*()`. SDKs that call cURL themselves (the Twilio and Stripe PHP SDKs, WP Mail SMTP's Gmail mailer) don't: add [the proxy](Echo-Install-Connectors#proxy-and-ca-for-raw-curl). In Laravel, `new GuzzleHttp\Client()` and SDKs with their own HTTP stack; in Node, a separate `undici`, `http2` or raw sockets. The proxy covers those too.
4. **Did something replace `wp_mail()` before Echo loaded?** `doctor` shows the warning *wp_mail() was already defined (…)*. Echo then catches mail through `pre_wp_mail` and `phpmailer_init`, which a replacement may skip. Find the plugin in the warning's path.
5. **Is it sent later?** WooCommerce webhooks, many notifications and anything queued go out with WP-Cron, Action Scheduler or a queue worker, after the action is over. Add a `flush:` command; see [Flush queued work](Echo-Actions-And-Compare#flush-queued-work).
6. **Did it arrive, but in no action?** `rhythm echo ls` shows everything. A message outside any action came without a run ID, after the action closed. Raise `quiet`, or make sure the request that triggers it carries `X-Echo-Run`.

Platform update checks (`api.wordpress.org`, `repo.packagist.org`) are let through without being recorded, on purpose; see [What passes through](Echo-Sealing-Staging#what-passes-through).

### The site runs in Docker

Inside a container, `127.0.0.1` is the container itself, so a connector pointed at `http://127.0.0.1:8025` can't reach Echo on the host. Two parts to fix:

1. **Make Echo listen where the container can reach it.** On Linux, that's the Docker bridge's address on the host (`172.17.0.1` for the default bridge; `ip -4 addr show docker0`). It's beyond loopback, so Echo needs a token:

   ```sh
   export RHYTHM_ECHO_TOKEN=$(openssl rand -hex 16)
   rhythm echo up --listen 172.17.0.1:8025 --smtp 172.17.0.1:1025
   ```

   With Docker Desktop (macOS, Windows), `host.docker.internal` reaches the host's loopback, so Echo can stay on `127.0.0.1`.
2. **Point the connector at the host.** Start the container with `--add-host host.docker.internal:host-gateway` (or `extra_hosts` in Compose), then install with that URL and the token:

   ```sh
   rhythm echo install wordpress --container shop-cli --url http://host.docker.internal:8025 --token "$RHYTHM_ECHO_TOKEN"
   ```

   Put the same address in `.echo/config.yaml` (`listen: 172.17.0.1:8025`, `smtp: 172.17.0.1:1025`), so `doctor` and your actions find Echo. When the install uses `--container` and a `127.0.0.1` URL, Echo reminds you: `In Docker, 127.0.0.1 is the container itself: reinstall with --url http://host.docker.internal:8025`.

For a stack that's all in Compose, run Echo as a service instead; the [hard seal recipe](Echo-Sealing-Staging#docker-internal-network) does that and seals the network too. Installing into the official `wordpress` image, which has no WP-CLI, is covered in [WordPress in Docker](Echo-Install-Connectors#wordpress-in-docker).

### doctor says a layer failed

| Line | Means | Fix |
|---|---|---|
| `✗ Echo: echo isn't running at http://127.0.0.1:8025 (start it with: rhythm echo up)` | Nothing answered at the URL | Start Echo, or pass `--url` / set `listen:` to where it runs. |
| `✗ SMTP: 127.0.0.1:1025: … (is Echo running with this config?)` | The sink isn't where the config says | Run `doctor` from the same folder as `up`, with the same config. |
| `· WordPress: no connector answered at … (…)` | The site has no connector, the URL is wrong, or the REST route is blocked (a security plugin, basic auth) | Install it; check `site:`; allow `?rest_route=/rhythm-echo/v1/ping` from Echo's machine. |
| `✗ WordPress: the connector is installed but off: production` | The production guard | Set `WP_ENVIRONMENT_TYPE`. |
| `✗ WordPress: active, but it can't reach Echo at … (in Docker use host.docker.internal)` | The connector's URL is wrong from the site | Reinstall with the right `--url`. |
| `✗ Canaries: only 1 of 2 canaries arrived` | One layer isn't catching | The email or HTTP hook is bypassed; see the warnings on the `·` lines. |
| `✗ Laravel: …` | `php artisan echo:canary` failed | Run it yourself to see the error; check `RHYTHM_ECHO_URL` in `.env` and `php artisan config:clear`. |
| `✗ Flush: "…": exit status 1 …` | A flush command fails | Run it by hand from the same folder. |
| `✗ Site: … is listed under production` | `site:` starts with a `production:` prefix | Point `site:` at staging. |

### Changes to the mu-plugin or settings don't show up at once

PHP's opcache may keep serving the old file for a moment: the official `wordpress` images set `opcache.revalidate_freq=2`, so a changed file is picked up within two seconds. Some hosts set it much higher, or turn `validate_timestamps` off; then reload PHP-FPM after installing or uninstalling.

### An SDK rejects Echo's certificate

With `--mitm`, the proxy presents certificates from Echo's own CA, so each client must trust it. Most read the system store or an environment variable (see [the table](Echo-Install-Connectors#the-https-proxy)). Some ship their own CA bundle and ignore all of these:

- **Stripe's PHP SDK** uses its own `data/ca-certificates.crt`. Call `\Stripe\Stripe::setCABundlePath('/etc/rhythm-echo/bundle.pem')`, or allowlist `api.stripe.com` on a site with test keys, so it's tunneled to Stripe with its real certificate.
- **WordPress** verifies against `wp-includes/certificates/ca-bundle.crt`: install the connector with `--ca`, which adds Echo's CA to that bundle.
- **Python's `certifi`** (used by `requests`): set `REQUESTS_CA_BUNDLE` to a bundle of the system CAs plus Echo's.
- **Java** keeps its own keystore: import `ca.pem` with `keytool -importcert`.

A client that **replaces** its store with Echo's certificate alone breaks for allowed hosts, which present their real certificates; build a bundle:

```sh
cat /etc/ssl/certs/ca-certificates.crt "$(rhythm echo ca)" > /etc/rhythm-echo/bundle.pem
```

When the TLS handshake fails, the client usually says `self-signed certificate in certificate chain` (curl's error 60), and nothing is recorded.

## Actions and comparisons

### An action takes 30 seconds

An action with `expect:` waits until enough messages arrive. When they don't, it waits the full `timeout` (30 s by default) before giving up and reporting *Sent fewer emails than expected*. That's the expect failing, not Echo hanging. Lower `timeout` if your messages arrive quickly.

### Everything shows as CHANGED

Usually a value that changes on every run reaches a message: a real address typed into an action, a random reference in the subject. Use `${fake:email}`, `${fake:name}` and `${fake:phone}` in actions, which Echo recognizes as test data, and check what differs with `rhythm echo report --plain`. Numbers, dates and token-like strings are already ignored; see [How messages are matched](Echo-Actions-And-Compare#how-messages-are-matched).

### "no snapshot named …"

The latest snapshot was deleted. `compare` doesn't fall back to another one: pass `--snapshot <name>`, or take a new snapshot.

### "actions from the snapshot weren't run this time"

An action was renamed, removed, or left out with `--action`. Actions are matched by name, so a renamed action isn't compared with its old self: take a new snapshot after renaming.

### A replay is refused

Each refusal says which rule it hit; see [Safety rules](Echo-Replay#safety-rules). The most common is a target that resolves to a public address: add the staging URL under `staging:`.

## Reporting a bug

Open an issue on the [Echo repository](https://github.com/RhythmShahriar/echo/issues) with:

- `rhythm echo --version`, your OS, and the stack and connector (WordPress, Laravel or Node version);
- the command you ran and its output, with `--plain`;
- `rhythm echo doctor` output;
- for a missed or misrecognized message, the request as `rhythm echo show <id>` prints it, with real data removed.

Use your own test site, never a third-party one.

### Found a security issue in Echo itself?

Don't open a public issue. Report it privately at https://github.com/RhythmShahriar/echo/security/advisories/new. See the [Security policy](Security-Policy).

---
<sub>← [CLI reference](Echo-CLI-Reference) · [Home](Home) · [Benchmark](Echo-Benchmark) →</sub>
