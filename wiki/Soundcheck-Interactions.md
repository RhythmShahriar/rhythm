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

> [!TIP]
> A form can say "sent" while no email goes out: the confirmation is on the page, the mailer behind it is broken. Soundcheck alone can't see that. Add `--echo` with [Echo](Echo) running, and each submit records the emails, SMS and webhooks it caused; a form that sent nothing gets *Form says sent, but no message went out*, and a message the snapshot had is BROKE when it's gone. See [With Soundcheck](Echo-With-Soundcheck).

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

Soundcheck collects up to 200 controls that look interactive: buttons, `role=button|tab|menuitem|switch`, `<summary>`, elements with `aria-expanded`, `aria-controls`, `aria-haspopup`, Bootstrap `data-toggle` attributes, `#` and `javascript:` links, and anything with a click listener. Controls that open menus and panels go first. It then clicks up to `interactions.max_clicks` of them per page (default **15**), each like a visitor: scroll it into view, send hover events, wait a moment for hover effects, then press and release and click. These pointer and mouse events are dispatched inside the page, so clicking works the same in every window while several pages are checked at once. Because they're script events, CSS `:hover` styles don't apply and handlers that insist on `event.isTrusted` ignore them.

Only controls a visitor could click are candidates. A control that's hidden, has no size, sits off screen, or is clipped by a parent that scrolls or hides its overflow is left out. One that's covered by something else (an open menu, a modal, a cookie banner) gets one retry after a reload. Clicking stops after **45 seconds** on a page, with a note such as `interact /shop/: stopped clicking after 45s with 12 controls left (the page is slow)`.

After each click it watches for up to 1.5 s (longer while requests the click started are still running):

| Outcome | Reported as |
|---|---|
| the DOM changed, a request was made, the page navigated, scrolled, or opened a dialog | ok |
| nothing happened at all | `interact/dead-click`, warning: `Button does nothing: Menu` |
| an uncaught JavaScript error | `interact/click-error`, **error**: `Clicking Add to wishlist throws a JS error` |
| a request the click made failed | `interact/click-request`, **error**: `Clicking Load more makes a request that fails` |

```text
[NEW] Button does nothing: Menu - /, /about/, /blog/ +31 more
  Clicking it changed nothing on the page, made no request and didn't navigate. (button.menu-toggle)
  fix: Wire up its click handler, or remove the button if it's no longer needed.
```

```text
[BROKE] Clicking Add to wishlist throws a JS error - /shop/
  Uncaught ReferenceError: WishlistStore is not defined (6 controls: li:nth-of-type(1) > button.add-to-wishlist, li:nth-of-type(2) > button.add-to-wishlist, li:nth-of-type(3) > button.add-to-wishlist, li:nth-of-type(4) > button.add-to-wishlist, li:nth-of-type(5) > button.add-to-wishlist, li:nth-of-type(6) > button.add-to-wishlist)
  fix: Click it with the browser console open to see the stack trace.
```

Titles use the control's visible label. Identical controls on a page with the same outcome (six "Add to wishlist" buttons that all throw) are one issue, with the selectors in the detail (`6 controls: …`, up to ten listed). The detail of a single control ends with its selector, so you can find it with `document.querySelector()`. Selectors leave out the state classes form plugins add and change (Contact Form 7's `init`, `sent`, `invalid` and the like, and their WPForms, Gravity Forms and Elementor equivalents), so the same control has the same selector in both runs.

Requests to analytics and ad hosts (Google Analytics and Tag Manager, Facebook, Hotjar, Clarity, Segment and others) don't count as a reaction: they fire on almost any click. A link to `#` that only jumps to the top of the page doesn't count either.

Between clicks, Escape closes whatever opened. If a click navigated, the page is reloaded. Every click is recorded in the run's JSON under `interactions` with its outcome (`ok`, `dead`, `error`, `skipped`) and, for skipped ones, why, such as `navigates away` or `looks destructive`.

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

- **Refuses click and submit steps in flows**, unless the flow file sets `production: true`. Read-only steps (`goto`, `fill`, `select`, `check`, `wait`, `expect`, `screenshot`) still run; the flow stops at the first refused step with a warning, `interact/flow-production`. See [Flows on production](Soundcheck-Forms-And-Flows#flows-on-production).

Inspecting and clicking pages still run. Notes say what was skipped:

```text
note: Form submits skipped on production URLs: allowlist forms in interactions.forms to test them there
note: cache: skipped the cache leak test on a production URL: a leaking cache would show the test account to real visitors. Run it on staging
note: Flow clicks and submits skipped on production URLs: set production: true in a flow to allow them
```

To test one form on production anyway, for example a newsletter signup that writes to a test list, allowlist it:

```yaml
interactions:
  forms: ["#newsletter-form"]
```

## Speed and reliability

Clicking takes time: each click waits up to 1.5 s for a reaction, and a page with many controls takes several seconds, at most 45. If a check is slow, lower `interactions.max_clicks`, or use `--level inspect` for a quick pass. Controls a visitor couldn't click (hidden, off screen, clipped, still covered after the retry) are never reported, so a control that couldn't be clicked doesn't show up as a false problem.

---
<sub>← [Visual changes](Soundcheck-Visual) · [Home](Home) · [Forms and flows](Soundcheck-Forms-And-Flows) →</sub>
