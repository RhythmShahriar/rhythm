The interact module checks that a page's controls still work: it reads forms and buttons, clicks menus, tabs and modals, and, when you allow it, submits forms and runs scripted flows. This page covers the four levels, dead clicks, the rules that keep clicking safe, and the production guard.

## The four levels

| Level | Name | What it does | Default |
|---|---|---|---|
| 1 | `inspect` | Reads the page: forms, fields, labels, ids and click handlers. Changes nothing. | on |
| 2 | `click` | Clicks controls that act on the page (menus, tabs, accordions, modals, sliders, toggles) and watches what happens. | on |
| 3 | `submit` | Submits forms with test data: empty first, expecting validation, then valid, expecting success. **Sends real data.** | off |
| 4 | `flows` | Runs your YAML flows from `.soundcheck/flows/`. | off |

Choose them with `--level` or `interactions.levels`:

```sh
rhythm soundcheck snapshot staging.yoursite.com --level inspect,click,submit
```

```yaml
interactions:
  levels: [inspect, click, submit, flows]
```

Pass the same levels to `compare`, or keep them in the config file; otherwise the comparison notes `interaction levels differ` and new checks report everything as NEW.

Levels 1 to 3 run on every page while it's open at desktop size, before the mobile reload. Level 4 runs once, after all pages. Nothing is clicked or submitted on `/wp-admin` or `/wp-login.php` pages. Submitting and flows are covered in [Forms and flows](Soundcheck-Forms-And-Flows).

## Level 1: inspect

| Check | Level | Title | Finds |
|---|---|---|---|
| `interact/form-action` | error | `Form sends to a missing page (404): /old-handler/` | a same-site form whose `action` answers 404 or 410 (or 405 for a GET form); tested with `HEAD`, falling back to `GET`, never `POST` |
| `interact/no-nonce` | warning | `Form has no nonce: form #newsletter` | on WordPress, a form that posts to the site without a nonce or CSRF token (comment, search, login and known form plugin forms excepted) |
| `interact/unnamed-field` | warning | `Form field has no name: Email` | a field without a `name`; the browser never sends its value |
| `interact/dead-click` | warning | `Button does nothing: Menu` | a button or a `#` / `javascript:void(0)` link with no click listener on it, its parents, the document or the window |
| `interact/unlabeled` | notice | `Required field has no label: Phone` | a visible required field with no `<label>`, `aria-label` or title |
| `interact/duplicate-id` | notice | `Duplicate id: "main"` | an id used by several elements (icons inside inline SVG excepted) |

Click listeners are found through Chrome's DevTools, so handlers added with `addEventListener`, `onclick` attributes, and handlers delegated to a parent or the document all count. When level 2 is on, the controls it clicks are judged by the click instead.

## Level 2: click

Soundcheck collects up to 200 controls that look interactive: buttons, `role=button|tab|menuitem|switch`, `<summary>`, elements with `aria-expanded`, `aria-controls`, `aria-haspopup`, Bootstrap `data-toggle` attributes, `#` and `javascript:` links, and anything with a click listener. Controls that open menus and panels go first. It then clicks up to `interactions.max_clicks` of them per page (default **15**), each like a visitor: scroll it into view, move the mouse onto it, wait for hover effects, click.

After each click it watches for up to 1.5 s (longer while requests the click started are still running):

| Outcome | Reported as |
|---|---|
| the DOM changed, a request was made, the page navigated, scrolled, or opened a dialog | ok |
| nothing happened at all | `interact/dead-click`, warning: `Button does nothing: Menu` |
| an uncaught JavaScript error | `interact/click-error`, **error**: `Clicking Add to wishlist throws a JS error` |
| a request the click made failed | `interact/click-request`, **error**: `Clicking Load more makes a request that fails` |

```text
[NEW] Button does nothing: Menu - /, /about/, /blog/ +32 more
  Clicking it changed nothing on the page, made no request and didn't navigate. (button.menu-toggle)
  fix: Wire up its click handler, or remove the button if it's no longer needed.
```

```text
[BROKE] Clicking Add to wishlist (li:nth-of-type(3) > button.add-to-wishlist) throws a JS error - /shop/
  Uncaught ReferenceError: WishlistStore is not defined
  fix: Click it with the browser console open to see the stack trace.
```

When several controls on a page have the same label, the title adds a CSS selector to tell them apart. The detail of a dead click ends with the selector, so you can find the element with `document.querySelector()`.

Requests to analytics and ad hosts (Google Analytics and Tag Manager, Facebook, Hotjar, Clarity, Segment and others) don't count as a reaction: they fire on almost any click. A link to `#` that only jumps to the top of the page doesn't count either.

Between clicks, Escape closes whatever opened. If a click navigated, or a menu covers the next control, the page is reloaded. Every click is recorded in the run's JSON under `interactions` with its outcome (`ok`, `dead`, `error`, `skipped`) and why it was skipped, such as `navigates away`, `looks destructive` or `can't scroll to it`.

## Keeping clicks safe

Level 2 explores a page without changing anything on the server:

- **Links that leave the page aren't clicked.** Navigating links are recorded as `navigates away`; discovery already visits those pages.
- **Destructive-looking controls aren't clicked.** A control is skipped (`looks destructive`) when its text, `aria-label`, id, class, href, title, value or name matches any of:

  `delete`, `remove`, `unsubscribe`, `log out` / `logout` / `log-out` / `logoff`, `sign out` / `signout` / `sign-out`, `cancel`, `deactivate`, `trash`, `destroy`, `erase`, `purge`, `revoke`, `wipe`, `uninstall`, `disconnect`, `unlink`, `empty cart`, `clear cart`, `reset password`, `lost password`, `close account`, `add to cart`, `place order`, `buy now`, `pay now`, `checkout`, `purchase`, `_wpnonce`, `/wp-admin`, `?action=` / `&action=`

- **Dialogs are defused.** Before clicking, `window.open`, `alert`, `confirm`, `prompt` and `print` are replaced in the page: nothing opens or blocks, and `confirm()` answers **Cancel**, so a "Really delete?" prompt never goes ahead.
- **Form submit buttons belong to level 3.** Level 2 never clicks a submit button inside a form.
- **Admin pages are off limits.** Nothing under `/wp-admin` or `/wp-login.php` is clicked.
- **Every check runs as a visitor.** Pages are visited in a private browser session that's never logged in.

To click a control the blocklist skips, because you know it's harmless on your site (a "Remove filter" chip, say), allow it by CSS selector:

```yaml
interactions:
  allow: [".filter-chip .remove", "#cancel-search"]
```

## The production guard

List your live site under `production` in `.soundcheck/config.yaml`:

```yaml
production: ["https://yoursite.com", "https://www.yoursite.com"]
```

Entries are URL prefixes: a page is production when its URL starts with one, so list every host the site answers on. On production URLs Soundcheck:

- **Doesn't submit forms.** Level 3 skips every form with `production URL; allowlist the form in interactions.forms to submit it`, unless the form matches a selector in `interactions.forms`.
- **Doesn't run the cache leak test**, which would serve the test account's pages to real visitors if the cache leaks. See [Cache](Soundcheck-Cache#the-leak-test).

Inspecting and clicking still run. A note says what was skipped:

```text
note: Form submits skipped on production URLs: allowlist forms in interactions.forms to test them there
note: cache: skipped the cache leak test on a production URL: a leaking cache would show the test account to real visitors. Run it on staging
```

To test one form on production anyway, for example a newsletter signup that writes to a test list, allowlist it:

```yaml
interactions:
  forms: ["#newsletter-form"]
```

> [!WARNING]
> Flows (level 4) aren't covered by the production guard: they do exactly what their steps say, on any site. Keep flows that submit or buy pointed at staging, or out of the flows folder when you check production.

## Speed and reliability

Clicking takes time: each click waits for a reaction, and a page with many controls takes several seconds. If a check is slow, lower `interactions.max_clicks`, or use `--level inspect` for a quick pass. Clicks that couldn't happen (the element was hidden, covered or off-screen) are recorded as skipped in the JSON rather than reported, so a missing click never shows up as a false problem.

---
<sub>← [Visual changes](Soundcheck-Visual) · [Home](Home) · [Forms and flows](Soundcheck-Forms-And-Flows) →</sub>
