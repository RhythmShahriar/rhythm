How Soundcheck tests that forms still send (level 3, `submit`), the form plugins it knows, the test data it types, the YAML flow format for scripted journeys (level 4, `flows`), and logging in with a test account.

> [!WARNING]
> Submitting is real. With the `submit` level or a flow that sends a form, messages arrive, sign-ups are created, orders may be placed. Use it on staging, route mail to a catcher such as Mailpit, and list your live site under `production` so Soundcheck refuses to submit there. See [Interactions](Soundcheck-Interactions#the-production-guard).

## Submitting forms

Turn it on for a run, and pass the same levels to `compare`:

```sh
rhythm soundcheck snapshot staging.yoursite.com --level inspect,click,submit
rhythm soundcheck compare --level inspect,click,submit
```

On each page, up to five forms are submitted, each in two steps, reloading the page in between:

1. **Empty, expecting validation.** Text fields are cleared and the form is submitted. If the browser's own validation blocks it (`required` attributes), that's a pass. Otherwise Soundcheck waits up to 10 s for the plugin's error messages. This step runs for forms with required fields and for known form plugins.
2. **Valid, expecting success.** Fields are filled with [test data](#test-data), required and consent checkboxes ticked, a radio picked in each group and the first real option of each select chosen. The form is submitted through its own submit button, so the plugin's JavaScript runs, and Soundcheck waits up to 10 s for a confirmation.

| Check | Level | Title | When |
|---|---|---|---|
| `interact/form-submit` | error | `Form no longer submits: Contact Form 7 #12` | the valid submit showed no confirmation, the plugin reported a failure, or the response was 4xx |
| `interact/form-error` | error | `Form submission fails (HTTP 500): WPForms #8` | the submission's request answered 5xx |
| `interact/form-validation` | warning | `Form shows no validation: form #quote` | the empty submit showed no error at all |

A contact form whose mail started failing after an update:

```text
[BROKE] Form no longer submits: Contact Form 7 #12 - /contact/
  There was an error trying to send your message. Please try again later.
  fix: Submit the form by hand and watch the browser's network tab; check the plugin's settings, mail setup and spam protection.
```

The detail is the message the plugin showed. A successful submit isn't reported as an issue; it's recorded in the run's JSON under `interactions`:

```json
{"kind": "submit", "selector": "form.wpcf7-form.init", "label": "Contact Form 7 #12", "outcome": "validation", "detail": "One or more fields have an error. Please check and try again."}
{"kind": "submit", "selector": "form.wpcf7-form.init", "label": "Contact Form 7 #12", "outcome": "success", "detail": "Thank you for your message. It has been sent."}
```

### Forms that are never submitted

| Form | Recorded as |
|---|---|
| any form, on a URL under `production` (unless allowlisted in `interactions.forms`) | `production URL; allowlist the form in interactions.forms to submit it` |
| hidden forms | `hidden` |
| login and other forms with a password field | `has a password field` |
| search forms and `GET` forms (except known plugins) | `search or GET form` |
| the WordPress comment form | `comment form; allowlist it in interactions.forms to test it` |
| the WooCommerce cart form | `cart form` |
| forms that post to `/wp-admin` or `wp-login.php` | `posts to WordPress admin` |
| forms with no fields | `no fields` |

A form listed in `interactions.forms` (by CSS selector) is always eligible, on production too.

## Form profiles

Soundcheck recognizes the big form plugins and reads their own success, failure and validation markers, so it knows the difference between "sent", "mail failed" and "a field is missing":

| Profile | Recognized by | Named in results as | Success means |
|---|---|---|---|
| Contact Form 7 | `.wpcf7` wrapper or `_wpcf7` field | `Contact Form 7 #12` | `form.sent` / `.wpcf7-mail-sent-ok` |
| WPForms | `form.wpforms-form` | `WPForms #8` | the confirmation container |
| Gravity Forms | `form#gform_3` or `.gform_wrapper` | `Gravity Forms #3` | `.gform_confirmation_message` |
| Elementor | `form.elementor-form` | `Elementor form contact` | `.elementor-message-success` |
| WooCommerce add to cart | `form.cart` | `WooCommerce add to cart` | `.woocommerce-message` or the "View cart" link |
| WooCommerce checkout | `form.checkout` | `WooCommerce checkout` | the order received page |
| any other form | | `form #quote`, `form "Get a quote"`, `form 2` | a redirect, a 2xx answer with no error shown, or a success message |

WooCommerce checkout is only submitted with a payment method that takes no money: Cash on delivery, Direct bank transfer or Check (`cod`, `bacs`, `cheque`), or a gateway that says it's in test or sandbox mode. Otherwise it's skipped with `no offline (cod, bacs, cheque) or test-mode payment method`.

For other forms, Soundcheck looks for common error markers (`[aria-invalid=true]`, `.invalid-feedback`, `.has-error`, `.alert-danger` and similar) and treats a redirect or a clean AJAX answer as success.

## Test data

Every value Soundcheck types contains **`soundcheck-test`**, so test entries are easy to find and delete in your inbox, CRM or database. Values are chosen from the field's type, `autocomplete`, name, id and label:

| Field | Value |
|---|---|
| email | `soundcheck-test+3fa1c2@example.com` (the tag changes per run) |
| name | `Soundcheck Test` (first name `Soundcheck`, last name `Test`) |
| phone | `+1 555 0100` (a number reserved for fiction) |
| URL | `https://example.com/soundcheck-test` |
| company | `Soundcheck Test Co` |
| subject | `soundcheck-test 3fa1c2` |
| message, any textarea | `soundcheck-test: an automated form check by Rhythm Soundcheck (3fa1c2). Please ignore.` |
| address, city, postcode | `1 Test Street`, `Testville`, `12345` |
| number | `1`, or the field's `min` |
| date | `2030-01-15`, kept within `min` and `max` |
| anything else | `soundcheck-test` |

Values are padded or cut to the field's `minlength` and `maxlength`. Password and file fields are never filled. The tag is derived from the run's start time, so one run types the same values everywhere and two runs can be told apart.

## Flows

A flow is a scripted journey in YAML: open a page, fill fields, click, wait, and check what the visitor should see. Use flows for what level 3 can't guess: a multi-step checkout, a booking widget, a search that should find something, a members area.

Write an example to start from:

```sh
rhythm soundcheck flows init
```

```text
Wrote .soundcheck/flows/example.yaml. Edit it, then run flows with: --level inspect,click,flows
```

It never overwrites an existing file. Every `.yaml` or `.yml` file in the flows folder (`.soundcheck/flows`, or the `flows` setting) is a flow, run in name order, each in its own private browser session. Run them with the `flows` level:

```sh
rhythm soundcheck run staging.yoursite.com --level inspect,click,flows
```

A flow that checks the contact form:

```yaml
name: Contact form sends
start: /contact/
steps:
  - fill: {selector: "input[name=your-name]", value: "${fake:name}"}
  - fill: {selector: "input[type=email]", value: "${fake:email}"}
  - fill: {selector: textarea, value: "${fake:message}"}
  - click: {text: Send}
  - wait: .wpcf7 form.sent
  - expect: {text: Thank you for your message}
  - screenshot: contact-sent
```

When a step fails, the flow becomes an issue at level **error** (BROKE in a comparison) that names the flow and the step:

```text
[ERROR] Flow "Contact form sends" step 5 failed: wait for .wpcf7 form.sent - /contact/
  source: .soundcheck/flows/contact.yaml
  fix: Run the flow by hand from /contact/; if the page changed on purpose, update .soundcheck/flows/contact.yaml.
```

The detail says what went wrong (`.wpcf7 form.sent not found`). A failed action stops the flow, since later steps depend on it; a failed `expect` is reported and the flow carries on. A flow that can't open its start page is reported as `Flow "…" couldn't open /contact/`.

### The flow file

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | Shown in results and used for screenshot names. |
| `start` | no | Path or URL opened first. Default: the site's home page. Relative paths resolve against the site being checked. |
| `login` | no | `true` runs the flow as the test user (see [Logging in](#logging-in)). Without credentials, the flow is skipped with a note. |
| `steps` | yes | The actions, in order. Each step is a one-key mapping. |

### Steps

| Step | Form | Does |
|---|---|---|
| `goto` | `goto: /pricing/` | Opens a path or URL and waits for it to load. |
| `click` | `click: "#send"` or `click: {selector: …}` or `click: {text: Send}` | Clicks an element by CSS selector, or by its visible text, value or `aria-label` (exact match first, then contains; with `selector`, only among matching elements). Then waits for the page to settle. |
| `fill` | `fill: {selector: "#email", value: "${fake:email}"}` | Clears a field and types the value. |
| `select` | `select: {selector: "#size", value: Large}` | Picks a `<select>` option by value, or else by its text. |
| `check` | `check: "#terms"` or `check: {selector: "#news", checked: false}` | Ticks (default) or unticks a checkbox or radio. |
| `submit` | `submit:` or `submit: "#quote-form"` or `submit: {text: Send}` | Submits a form, or the form of the element given, through its submit button. With no target, the first form on the page. |
| `wait` | `wait: 1500` or `wait: .done` or `wait: {selector: .done}` / `{ms: 500}` | Waits for milliseconds (at most 60,000), or until an element is visible. |
| `expect` | `expect: Thank you` or `expect: {…}` | Checks the page; see below. |
| `screenshot` | `screenshot: contact-sent` | Saves a screenshot as `flows/<flow>-<step>-<name>.png` in the run folder. |

`expect` takes any of these; every one given must hold. It retries until the step timeout before failing.

| Key | Holds when |
|---|---|
| `text` | the page's visible text contains it (case-insensitive) |
| `selector` | an element matching it is visible |
| `url` | the current URL contains it |
| `status` | the last page load answered this HTTP status |
| `absent: true` | turns `text` and `selector` around: they must **not** be there |

```yaml
  - expect: {url: /order-received/, text: Thank you}
  - expect: {selector: .woocommerce-error, absent: true}
```

Each step waits up to the run's `timeout` (default 30 s) for its element.

### Values

| Reference | Becomes |
|---|---|
| `${fake:email}`, `${fake:name}`, `${fake:first_name}`, `${fake:last_name}`, `${fake:phone}`, `${fake:url}`, `${fake:company}`, `${fake:subject}`, `${fake:text}`, `${fake:message}`, `${fake:number}`, `${fake:address}`, `${fake:city}`, `${fake:zip}`, `${fake:date}` | the [test data](#test-data) above (aliases such as `${fake:tel}`, `${fake:postcode}` and `${fake:full_name}` work too) |
| `${env:NAME}` | the environment variable `NAME` |

Values from `${env:…}` are never stored or printed: every error message is masked (`****`). Use them for coupon codes, test card numbers and anything else you don't want in the repository. A missing variable fails the step.

### Mistakes in a flow file

Flow files are checked strictly: unknown keys, unknown steps and unknown `${fake:…}` kinds are errors, reported as a note with the line:

```text
note: flows: .soundcheck/flows/bad.yaml: line 3: unknown step "clik" (actions: goto, click, fill, select, check, submit, wait, expect, screenshot)
```

> [!IMPORTANT]
> A flow file with a mistake stops **all** flows for that run, and the run still ends `all clear`. Check the notes after editing flows, and in CI look for a `flows:` note in the report.

## Logging in

Set a test account in the environment:

```sh
export SOUNDCHECK_USER=soundcheck-test
export SOUNDCHECK_PASSWORD='…'
rhythm soundcheck snapshot staging.yoursite.com
```

```text
Login: as so*** (from SOUNDCHECK_USER)
```

Credentials are read only from these two variables, never from flags or the config file, so they don't end up in shell history, config files or reports. They're masked in every error message. Cookies stay in the browser's memory.

The login is used by:

- **the cache leak test**, which visits up to ten pages as the test user and checks that anonymous visitors don't get those copies. See [Cache](Soundcheck-Cache#the-leak-test).
- **flows with `login: true`**, which start already logged in.

The pages themselves are always checked as an anonymous visitor. `--no-login` ignores the variables for one run.

On WordPress, Soundcheck logs in through `/wp-login.php` (`#user_login`, `#user_pass`, `#wp-submit`), with "Remember me" off. It succeeds when WordPress sets its `wordpress_logged_in_*` cookie and the front end shows the admin bar or the `logged-in` body class. For a login page elsewhere, set `login.url: /account/login/`. Errors are specific:

| Message | Means |
|---|---|
| `WordPress refused the login: <message>` | wrong user or password; WordPress's own message |
| `… has no WordPress login form (#user_login); set login.url, or login.flow for other sites` | the login page isn't WordPress's |
| `logged in, but the site doesn't show the admin bar or the logged-in body class (a cache may be serving anonymous pages to logged-in users)` | the cookie was set, but the site doesn't look logged in |

For sites that aren't WordPress, write a flow that logs in and point `login.flow` at it:

```yaml
# .soundcheck/login.yaml
name: Log in
start: /account/login/
steps:
  - fill: {selector: "#email", value: "${env:SOUNDCHECK_USER}"}
  - fill: {selector: "#password", value: "${env:SOUNDCHECK_PASSWORD}"}
  - click: {text: Sign in}
  - expect: {selector: .account-menu}
```

```yaml
# .soundcheck/config.yaml
login:
  flow: .soundcheck/login.yaml
```

Keep the login flow outside the flows folder, or it will also run as a level 4 flow.

> [!TIP]
> Create a dedicated test account with the lowest role that sees what you need (Subscriber or Customer). Never use an administrator account for automated checks.

---
<sub>← [Interactions](Soundcheck-Interactions) · [Home](Home) · [Cache](Soundcheck-Cache) →</sub>
