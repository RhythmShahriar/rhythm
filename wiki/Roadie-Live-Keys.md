A staging copy that keeps production's payment, mail or SMS keys can charge real cards, detach saved cards at Stripe, send real email and text real people. Roadie finds every live key in the copy, tells live from test and swaps each live key for the test key you give it, or blanks it. Test keys are left byte for byte. This page lists where Roadie looks, what it recognises and how to set test keys.

## Where Roadie looks

- Every string in the database, including inside PHP serialized, JSON and base64-serialized values: a gateway's settings in `wp_options`, an SMTP plugin's connection, a key a plugin stored in post meta.
- Every `define()` in `wp-config.php` and every line of `.env`, on the source and on the target.
- On a local site target, theme and plugin code too. A key in code is reported with `action: manual`: Roadie only edits the database, `wp-config.php` and `.env`, never code. Over ssh Roadie reads only `wp-config.php` and `.env`.

`plan` lists every key it found, masked, with what will happen to it:

```text
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
```

A key is never printed in full: at most its first four characters, then `•••••`. There is no flag that prints one.

## What Roadie recognises

| Provider | Finding | Severity when live | Live | Test (kept as it is) |
|---|---|---|---|---|
| Stripe secret or restricted key | `key.stripe-live` | critical | `sk_live_`, `rk_live_` | `sk_test_`, `rk_test_` |
| Stripe publishable key | `key.stripe-publishable-live` | high | `pk_live_` | `pk_test_` |
| Stripe webhook secret | `key.stripe-webhook-live` | high | `whsec_` next to a live secret key or live-mode setting | next to a test key or test mode |
| PayPal | `key.paypal-live` | critical | client ID or secret in a live field, or with sandbox off | sandbox fields, sandbox on |
| Braintree | `key.braintree-live` | critical | `access_token$production$` | `access_token$sandbox$` |
| Square | `key.square-live` | critical | token or app ID with environment `production` | `sandbox-` prefixes, sandbox flag |
| WooPayments | `key.woopayments-live` | high | an account in live mode | dev or test mode (`WCPAY_DEV_MODE`, a development or staging environment) |
| Mollie | `key.mollie-live` | critical | `live_` | `test_` |
| Razorpay | `key.razorpay-live` | critical | `rzp_live_` | `rzp_test_` |
| Adyen | `key.adyen-live` | critical | API key with a live endpoint | test environment |
| Klarna | `key.klarna-live` | critical | credentials in a live-mode field | playground |
| Authorize.net | `key.authorize-net-live` | critical | login and transaction key with environment production | sandbox |
| Flutterwave | `key.flutterwave-live` | critical | `FLWSECK-` | `FLWSECK_TEST-` |
| WooCommerce REST | `key.woocommerce-rest-live` | high | `ck_` and `cs_` with 40 hex | |
| SendGrid, Mailgun, Postmark, Amazon SES, Brevo, Mailchimp, Mandrill, Resend, SparkPost, MailerSend | `key.<provider>-live` | high | the provider's key format | |
| SMTP password | `key.smtp-live` | high | WP Mail SMTP, Post SMTP and FluentSMTP connections, `MAIL_PASSWORD` with a host that isn't loopback | a loopback or Echo host |
| Twilio | `key.twilio-live` | high | auth token and SID in SMS plugin options or `.env` | Twilio's test credentials |
| Jetpack | `key.jetpack-live` | high | `blog_token`, `user_tokens` in `jetpack_options` | |
| AWS | `key.aws-live` | high | `AKIA` or `ASIA` with a secret | |
| Anything named like a secret | `key.generic` | high | `DB_PASSWORD`, `AUTH_KEY`, `access_token` and the other names in the shared kit's rules, with a value that isn't a placeholder | placeholders (`xxx`, `changeme`, masked `•••`) |
| A provider key Roadie can't classify | `key.unknown-env` | medium | | reported and never changed |

`rhythm roadie explain key.<provider>-live` explains any of them. Gateway settings in live mode are a finding of their own, `key.live-mode` (high), even when the keys are blank: `testmode = no`, PayPal's sandbox off, WooPayments live. A disabled gateway's live keys count as low.

## Swap or blank

`keys.mode: swap` (the default) gives a live key the test key you set for that provider under `keys.test_keys`, when that value is itself a test key. Without one, the live key is blanked. `keys.mode: blank` (or `--keys blank`) blanks every live key. A blanked value is an empty string with its serialized length fixed, `define( 'NAME', '' );` in `wp-config.php` or `NAME=` in `.env`.

Either way, known gateways go to test or sandbox mode in the same run, so a blanked key never leaves a gateway live with no key. An enabled gateway Roadie doesn't know is disabled (`enabled = no`). See [Kill switch](Roadie-Kill-Switch#wordpress).

Set test keys through the environment, never in the file:

```yaml
keys:
  mode: swap
  test_keys: {stripe: "${ROADIE_STRIPE_TEST_SECRET}"}
```

`config check` lists a variable that isn't set:

```text
/home/ann/shop/.roadie/config.yaml is valid.
Unset: ROADIE_STRIPE_TEST_SECRET (line 46); set it before a run that reads it.
```

With it set to a Stripe test key, the copy swaps the secret key and blanks the two keys it has no test value for:

```text
  ✓ Data           2,515 values pseudonymised in 12 tables, 80 URLs
                   rewritten
  ✓ Keys           1 swapped to a test key, 2 blanked
  ✓ Side effects   gateways, webhooks (data rules only: a dump has no files)
  ✓ Written        staging2.sql (146 KB)
```

A "test key" that is really live is refused before anything runs (exit 2):

```text
rhythm-roadie: .roadie/config.yaml line 46: test-key-is-live: keys.test_keys.stripe reads a live key (sk_l•••••). Give a test key (sk_test_...) or remove the entry to blank it.
```

Only `${ROADIE_*}` variables are read, so a config can't pull an unrelated secret out of a CI environment:

```text
rhythm-roadie: bad2.yaml line 2: keys.test_keys.stripe reads ${STRIPE_TEST}; only ROADIE_ variables are read. Rename it ROADIE_STRIPE_TEST.
```

In the GitHub Action, pass the variable from a secret in the step's `env:` (see [CI](Roadie-CI#secrets)).

## Laravel's .env

A Laravel target's `.env` is edited in place, order and comments kept. In the Laravel example from [Laravel](Roadie-Laravel), the plan lists each live key by line, masked:

```text
    · .env: key.aws-live at line 17 (AKIA•••••): swapped or blanked
    · .env: key.aws-live at line 18 (0XdW•••••): swapped or blanked
    · .env: key.stripe-publishable-live at line 20 (pk_l•••••): swapped or
      blanked
    · .env: key.stripe-live at line 21 (sk_l•••••): swapped or blanked
    · .env: key.stripe-webhook-live at line 22 (whse•••••): swapped or
      blanked
```

After the copy those lines read `AWS_ACCESS_KEY_ID=`, `AWS_SECRET_ACCESS_KEY=`, `STRIPE_KEY=`, `STRIPE_SECRET=` and `STRIPE_WEBHOOK_SECRET=`.

## Salts and APP_KEY

They aren't provider keys. Still, a copy that shares them with production can forge cookies production accepts:

- WordPress: the eight salts (`AUTH_KEY` to `NONCE_SALT`) in `wp-config.php` are regenerated, 64 random characters each.
- Laravel: `APP_KEY` is rotated to a new `base64:` key. Encrypted columns and old sessions from production become unreadable. `--keep-app-key` keeps it (verify then reports `config.salts`, medium).

## Keys Roadie leaves alone

- A key in theme or plugin code: reported (`key-manual`, exit 1 when it stops a copy) with the file and line. Move it into `wp-config.php` or blank it by hand, then run verify.
- A key it can't classify (`key.unknown-env`): reported, never changed. Check it at the provider.
- A value you know is harmless: list it under `keys.ignore` with a reason and, optionally, an end date:

  ```yaml
  keys:
    ignore: [{id: key.generic, location: "wp_options my_flag", reason: "a feature flag, not a secret", until: 2027-01-31}]
  ```

  An entry past its `until` date stops hiding and is listed as `info.ignore-expired`.

Error trackers (Sentry, Bugsnag) are left as production set them: errors from staging reach production's project unless you change the DSN.

---
<sub>← [Pseudonymisation](Roadie-Pseudonymisation) · [Home](Home) · [Kill switch](Roadie-Kill-Switch) →</sub>
