The keyboard module uses each page the way someone who can't use a mouse does: Tab and Shift+Tab through every stop, then menus, dialogs and disclosures opened and closed by keyboard. Key flows go further and replay a whole journey, such as a checkout, with keys only. This page covers what the walk presses, the 15 checks it makes, its settings, key flows, the files it writes and its limits.

```sh
rhythm soundcheck snapshot staging.yoursite.com --eaa          # health, a11y and keyboard
rhythm soundcheck run staging.yoursite.com --modules +keyboard # the walk alone, without the statement
```

`--eaa` turns the module on (see [EAA mode](Soundcheck-EAA)); `--modules +keyboard` or `modules:` in the config name it without EAA mode. It needs Chrome: it presses real keys through the Chrome DevTools protocol (trusted key events, never script events). Under Firefox or WebKit every page's keyboard audit is `skipped` with the reason `needs Chrome`.

## How the walk works

On every page, after the other modules:

1. **Tab forward** until focus comes back to the start, up to `keyboard.max_tabs` presses (default 150), recording every stop: its selector, role, accessible name, box and whether it was hidden, covered or changed the page.
2. **Shift+Tab back** the same way.
3. **Trap probe.** A stop that holds focus for `trap_after` presses (default 4) is tried with Escape and Tab once more before it counts as a trap.
4. **Skip link.** Enter on a link to `#id` on the same page, then a check that the next Tab lands in or after the target.
5. **Focus rings.** Up to 40 stops per page are pictured with and without focus; fewer than 20 changed pixels around the element means no visible focus indicator.
6. **Widgets.** On the first page of each URL group (by path) and on every page matching `keyboard.pages`, up to `keyboard.widgets` menus, dialogs, disclosures and custom controls (default 10) are opened with Enter, Space or ArrowDown, closed with Escape or their close control; then Soundcheck checks where focus goes. Custom controls with a click listener that Tab can't reach, or that ignore Enter and Space, are found here.

Under `-v` each page gets a line:

```text
[1/1] /a11y/ 200 1.34s (ok)
  timing: ttfb 2ms, load 25ms, settle 1.32s
  keyboard: /a11y/ 11 stops, 1 widget (dialog), 3.0s
```

> [!TIP]
> Widgets are tested on a sample, not every page. A shop's `/product/{slug}/` group has its widgets tested on the first product by path, the same one every run. Put the pages that hold your important widgets (cart, checkout, account, contact) in `keyboard.pages`. `init --eaa` suggests `["/cart*", "/checkout*", "/my-account*", "/contact*"]`.

### What the walk never does

While it has a page, the walk never:

- **sends anything.** Requests other than GET and HEAD, WebSocket connections and connection upgrades are stopped before they leave the browser, also those of the page's service worker.
- **leaves the page.** Navigations that aren't to a fragment of the same page are stopped; windows the page opens are closed.
- **presses** a link to another page, a form's submit control or anything that looks destructive: delete, remove, log out, sign out, unsubscribe, cancel order, place order, pay, buy, checkout, empty cart (and the same words in German, French, Spanish, Italian and Dutch).
- downloads files, opens a file chooser, accepts an alert, confirm or prompt, grants a permission (location, notifications, camera, microphone, clipboard, MIDI) or answers a passkey request.
- **types** into fields or reads a field's value: names come from Chrome's accessibility tree.
- walks `/wp-admin` or `/wp-login.php` (audit `skipped`, reason `admin pages are never walked`).
- acts inside a frame from another origin: it records only the host and how many stops it holds.

Everything it held or stopped is listed in the page's trace, such as `POST /cart`, `navigation stopped: GET /away`, `download blocked: GET /file.pdf` or `dialog dismissed: alert`. A site that changes data on a GET or on focus can still change: run it on staging.

## The checks

Every keyboard issue is keyed by check and selector, never by an accessible name, so a label that changes (a signed-in customer's email in a link) doesn't make an issue new. At most 20 issues per check per page. The criteria follow the run's WCAG version; the EN 301 549 clause is `9.` plus the criterion.

| Check | Level | WCAG / EN 301 549 | Title |
|---|---|---|---|
| `keyboard/trap` | error | 2.1.2 / 9.2.1.2 | `Keyboard focus is trapped` |
| `keyboard/not-reachable` | warning | 2.1.1 / 9.2.1.1 | `Clickable element can't be reached by keyboard` |
| `keyboard/not-operable` | warning | 2.1.1 / 9.2.1.1 | `Control doesn't respond to Enter or Space` |
| `keyboard/menu` | warning | 2.1.1 / 9.2.1.1 | `Submenu doesn't open by keyboard` |
| `keyboard/focus-lost` | warning | 2.4.3 / 9.2.4.3 | `Focus is lost after closing` |
| `keyboard/dialog-focus` | warning | 2.4.3 / 9.2.4.3 | `Dialog doesn't take or keep focus` |
| `keyboard/no-focus-ring` | warning | 2.4.7 / 9.2.4.7 | `No visible focus indicator` |
| `keyboard/focus-invisible` | warning | 2.4.7 / 9.2.4.7 | `Focused element is hidden` |
| `keyboard/context-change` | warning | 3.2.1 / 9.3.2.1 | `Moving focus changes the page` |
| `keyboard/skip-link-broken` | warning | 2.4.1 / 9.2.4.1 | `Skip link doesn't move focus to the content` |
| `keyboard/skip-link-missing` | notice | advisory | `No skip link before a long run of stops` |
| `keyboard/escape` | notice | advisory | `Escape doesn't close it` |
| `keyboard/focus-obscured` | notice | 2.4.11 / 9.2.4.11 with `--wcag 2.2`, else advisory | `Focused element is covered by fixed content` |
| `keyboard/flow` | error | 2.1.1 / 9.2.1.1 | `Key flow can't be finished by keyboard` |
| `keyboard/error-not-linked` | warning | 3.3.1 / 9.3.3.1 | `Form errors aren't tied to their fields` |

A trap is the only walk check at error level, so a new trap is BROKE and the other new keyboard problems are NEW. `rhythm soundcheck help keyboard` prints the list; `rhythm soundcheck --list-checks` prints it with every other module's.

Four testbed pages in one run (dialog-nofocus, autonav, offcanvas and noescape, each in its broken state), with `keyboard.pages: ["/a11y/*"]`:

```text
[WARNING] Dialog doesn't take or keep focus - /a11y/dialog-nofocus.php
  source: #client-open
  wcag: 2.4.3; EN 301 549 9.2.4.3
  The dialog opened by "Add a client" didn't get focus.
  fix: A dialog opens without moving focus into it, or a modal lets Tab reach the page behind it.
[WARNING] Moving focus changes the page - /a11y/autonav.php
  source: #view
  wcag: 3.2.1; EN 301 549 9.3.2.1
  Moving focus to select "Show invoices for" with Tab opened a new page without Enter.
  fix: Tabbing onto an element alone opens a page, a window or sends a form. Wait for Enter or a click.
[WARNING] Focused element is hidden - /a11y/offcanvas.php
  source: #drawer > a:nth-of-type(1)
  wcag: 2.4.7; EN 301 549 9.2.4.7
  Tab stop 6 (a "Profile") has focus but is hidden (outside the viewport).
  fix: An element gets focus while it is transparent, hidden, clipped or off screen. Show it on focus or take it out of the Tab order.
...
[NOTICE] Escape doesn't close it - /a11y/noescape.php
  source: #filters-open
  The dialog opened by "Filters" is still visible 500 ms after Escape.
```

### What each check looks for

- **trap**: `Tab and Shift+Tab kept focus here for <n> presses; <k> focusable elements after it (...) can't be reached.` Not a trap: a modal dialog that Escape closes, a cycle that passes the page itself, nothing focusable after it, Escape then Tab getting out, a cross-origin frame held for fewer than 50 presses each way, or a walk that was capped or timed out. The issue carries the last six stops as its `path:`.
- **not-reachable**: an element with its own `click`, `mousedown`, `pointerdown`, `mouseup` or `pointerup` listener that Tab never reaches. Listeners on the document, the body or a container with more than 5 children (delegation), elements inside a focusable element or a label, boxes under 4×4 px and inert or hidden elements are left out.
- **not-operable**: Enter and Space did nothing for 500 ms each and one mouse click at its centre did.
- **menu**: `The submenu of "<name>" doesn't open with <Enter, Space or ArrowDown|Tab>.`
- **focus-lost**: `After <Escape|Enter on the close control|toggle> on "<name>" focus went to <the page body|a removed element|a hidden element>.`
- **dialog-focus**: the dialog didn't get focus, or a modal let Tab reach the page behind it.
- **no-focus-ring**: measured by comparing pictures of the area around the element with and without focus, desktop only. A stop whose look keeps changing (an animation) isn't judged. A crop of each failing element is saved in `eaa/focus/` (never of a text field).
- **focus-invisible**: a stop with opacity 0, `visibility: hidden`, clipped to 1 px or outside the viewport.
- **context-change**: Tab alone opened a page, a window or sent a form. A navigation the page starts by itself in the first 2 s after load isn't blamed on Tab.
- **skip-link-broken**: `Skip link "<name>" points at #<id>, which <doesn't exist|isn't where the next Tab goes>.`
- **skip-link-missing**: at least 10 stops before the main landmark (or no main landmark) and none of the first 3 is a skip link. With `skip_link: required` it is a warning failing 2.4.1 from 1 stop; with `skip_link: off` it never fires.

## Settings

The `keyboard:` block in [`.soundcheck/config.yaml`](Soundcheck-Configuration#eaa-mode):

```yaml
keyboard:
  max_tabs: 150        # 10 to 2000: Tab presses per page before the walk stops
  widgets: 10          # 0 to 200: menus, dialogs, disclosures and custom controls tested per page
  trap_after: 4        # 3 to 20: presses on the same element before it counts as a trap
  skip_link: expected  # expected, required or off
  pages: ["/cart*", "/checkout*", "/my-account*", "/contact*"]  # widgets are tested here as well
  ignore: []           # CSS selectors the walk records but never judges or presses
```

| Key | Default | Description |
|---|---|---|
| `max_tabs` | `150` | Tab presses per page before the walk stops. A walk that stops there is capped, never a trap. Raise it for long pages. |
| `widgets` | `10` | Widgets and custom controls tested per page; `0` turns widget tests off. |
| `trap_after` | `4` | Presses on the same element or a 2 or 3 element cycle that count as a trap. |
| `skip_link` | `expected` | `expected`: a missing skip link is a notice. `required`: a warning that fails 2.4.1. `off`: never reported. |
| `pages` | `[]` | Page globs, from the site's root, whose widgets are tested besides the first page of each URL group. |
| `ignore` | `[]` | CSS selectors the walk steps over, such as a cookie banner you test elsewhere. |

Mistakes stop the command with exit 2 and name the file and line, for example `keyboard.max_tabs: must be a whole number from 10 to 2000, not 9`, `keyboard.pages: "shop/*" isn't a path: write it from the site's root, such as /checkout*` or `unknown setting "keyboard.max_tab" (did you mean "keyboard.max_tabs"?)`.

## Key flows

A key flow is a [flow](Soundcheck-Forms-And-Flows#flows) played by keyboard only. It turns "checkout can be finished by keyboard", what EN 301 549 clause 9.2.1.1 asks, into a test. Mark a flow with `keyboard:` and run with `--level flows`:

| `keyboard:` | Means |
|---|---|
| `true` | A key flow, held **dry**: it stops at the first step that would send something to the site. |
| `send` | A key flow that sends for real, only on this machine, a private address or with `--staging`, never on a URL listed under `production:`. |
| `false` or absent | A mouse flow, as before. |

### Write one

Four templates to edit:

```sh
rhythm soundcheck flows init --keyboard
```

```text
Wrote .soundcheck/flows/keyboard-checkout.yaml
Wrote .soundcheck/flows/keyboard-contact.yaml
Wrote .soundcheck/flows/keyboard-dialog.yaml
Wrote .soundcheck/flows/keyboard-menu.yaml
Edit them, then run them with --level flows: they press keys only; keyboard: true holds every request that would send data.
```

| Template | What it does |
|---|---|
| `keyboard-checkout.yaml` | WooCommerce `/checkout/`: fills the billing fields with `${fake:...}`, ticks `#terms` with Space, Tabs to Place order and presses it (dry: the flow stops there), expects `Thank you`. |
| `keyboard-contact.yaml` | `/contact/`: fills name, email and message of the form with a textarea and submits it. |
| `keyboard-menu.yaml` | `/`: Tabs to the menu toggle, presses Enter, expects `[aria-expanded=true]` and focus within `nav`. |
| `keyboard-dialog.yaml` | `/`: opens the first `[aria-haspopup=dialog]`, expects focus inside it, presses Escape, expects focus back on the trigger. |

A file that's already there is kept and named (`Kept .soundcheck/flows/keyboard-checkout.yaml: it's already there`); with all four there it writes nothing and exits 2. `init --eaa` writes `keyboard-checkout.yaml` only.

### How each step plays

| Step | By keyboard |
|---|---|
| `click` | Tab until focus is on the target (or inside it), then Enter; Space on a checkbox, radio, switch or role=button without href. A label tied to a control or a node inside a focusable control reaches that control. |
| `submit` | Tab to the form's submit button (else its first field), then Enter. |
| `fill` | Tab to the field, then type the value without key events carrying it. |
| `select` | Tab to the select, then ArrowDown (or ArrowUp) until the option is selected. |
| `check` | Tab to the box, Space when its state differs. |
| `press` | New: `press: Tab`, `press: {key: Enter, on: ".menu-toggle"}`, `press: {key: ArrowDown, times: 3}`. Keys: Tab, Shift+Tab, Enter, Space, Escape, ArrowUp, ArrowDown, ArrowLeft, ArrowRight, Home, End. `times` is 1 to 50. |
| `expect` | New keys `focus: <selector>` (the focused element matches) and `focus_within: <selector>` (focus is inside a match), both with `absent: true`. |
| `goto`, `wait`, `screenshot` | As in a mouse flow. |

A step's target is reached with up to `keyboard.max_tabs` Tab presses, then as many Shift+Tab. No mouse event is ever sent. `press` also works in mouse flows, where `on:` focuses the element directly.

### A passing key flow

The order form of [the EAA walk-through](Soundcheck-EAA#your-first-eaa-run), `keyboard: true`. The run writes what each step pressed to `keyflows/<flow name>.txt` in its folder; typed values never appear:

```text
step 1 click "#delivery-open": on #delivery-open after 5 key presses
step 1 click "#delivery-open": Enter
step 3 press Escape
step 5 select "#bag": on #bag after 1 key press
step 5 select "#bag": ArrowDown
step 6 fill "#name": on #name after 1 key press
step 6 fill "#name": <typed 15 characters>
step 7 fill "#email": on #email after 1 key press
step 7 fill "#email": <typed 34 characters>
step 8 fill "#street": on #street after 1 key press
step 8 fill "#street": <typed 13 characters>
step 9 fill "#postcode": on #postcode after 1 key press
step 9 fill "#postcode": <typed 5 characters>
step 10 press Tab
```

### A key flow that fails

The wishlist's "Save for later" control lost its key handler (testbed s57). The flow:

```yaml
name: Save to the wishlist by keyboard
start: /a11y/wishlist.php
keyboard: true
steps:
  - click: "#save"
  - expect: {text: Saved}
```

```text
[ERROR] Key flow can't be finished by keyboard - /a11y/wishlist.php
  source: keyboard-wishlist.yaml: click #save
  wcag: 2.1.1; EN 301 549 9.2.1.1
  Save to the wishlist by keyboard: step 1 click "#save": reached; Space did nothing
  fix: Make the control a native button, link or field, or give it tabindex="0" and handle Enter and Space.
[WARNING] Control doesn't respond to Enter or Space - /a11y/wishlist.php
  source: #save
  wcag: 2.1.1; EN 301 549 9.2.1.1
  span "Save for later" reacts to a mouse click but not to Enter or Space.
  fix: A custom control reacts to a click but not to Enter or Space. Use a native button, or handle both keys.
```

`keyboard/flow` is an error, so a key flow that stops working is BROKE. A step whose target Tab never reaches reads `not reached after <max_tabs> Tab stops; focus stayed on <selector>`. Other failures of a key flow (a selector not found, an `expect` that failed) are reported as for any flow ([Forms and flows](Soundcheck-Forms-And-Flows#flows)).

After a submit step, or a click on a button in a form, Soundcheck waits up to 1 s for errors. It reports `keyboard/error-not-linked` when fields show errors, focus didn't move to the first one (or to an error summary) and the fields don't point at their messages with `aria-describedby` or `aria-errormessage`.

### Dry by default

`keyboard: true` holds every request but GET and HEAD for the whole flow. The step whose key press makes the site send passes and ends the flow, with a note. A checkout form on a fixture of this machine:

```yaml
name: Order by keyboard
start: /
keyboard: true
steps:
  - fill: {selector: "#name", value: "${fake:name}"}
  - fill: {selector: "#email", value: "${fake:email}"}
  - click: "#place_order"
  - expect: {text: Thank you}
```

```text
note: keyboard: keyboard-order.yaml stopped at step 3 (would have sent POST /order/); step 4 not run
```

No order was placed. Writes during other steps (autosaves as focus moves, WooCommerce's `update_order_review`, beacons, writes to other hosts) are blocked without ending the flow. With `keyboard: send` the same flow pressed Enter on Place order and the fixture recorded one order: use `send` on staging with a test payment method; pass `--staging` when the staging site has a public address.

`send` on a public address without `--staging` runs dry, with `note: keyboard: send runs only on a loopback or private address, or with --staging; <file> ran dry`. On a URL listed under `production:` key flows are refused like any flow (`Flow "<name>" step <n> refused on production: <step>`), dry ones included: the hold covers only the flow's own tab, not a popup, a `target=_blank` form or a message over an open WebSocket, so it isn't trusted alone against a live shop.

### Notes

| Note | When |
|---|---|
| `keyboard: 1 key flow needs --level flows (it types test data); not run` | Key flows exist and the run's levels stop below `flows`. The header says `1 key flow (keyboard-order.yaml) needs --level flows, not run`. |
| `keyboard: 1 key flow needs the keyboard module (--eaa or +keyboard); not run` | The keyboard module is off. Mouse flows never play key flows. |
| `keyboard: <file> not run: needs a login: set SOUNDCHECK_USER and SOUNDCHECK_PASSWORD` | A flow with `login: true` and no test account. |
| `keyboard: flow file <file> can't be read (<error>); a key flow in it is not run` | A broken flow file when interact doesn't play flows. |

run.json records `key_flows` with `run` (how many ran) and `skipped` (`{file, reason}`).

## Files

| File in the run folder | What it holds |
|---|---|
| `keyboard/<page id>.json` | The trace of a page's walk (schema `rhythm-soundcheck/keyboard-trace/v1`, at most 2 MB): every forward and backward stop, the trap probe, the skip link test, the widget tests, the focus ring measurements, the held requests and the events. Paths lose their query string; token-like segments are masked. |
| `eaa/focus/<page id>-<n>.png` | A crop of each element that failed `keyboard/no-focus-ring` (n is its Tab stop). |
| `keyflows/<flow>.txt` | One line per key press of each key flow. |

Part of a trace, from the walk of `/a11y/`:

```json
"skip_link": {"selector": "a", "name": "Skip to content", "target_id": "main", "target_found": true, "next_stop": "#delivery-open", "next_inside_or_after_target": true, "scrolled": false},
"widgets": [{"kind": "dialog", "trigger": "#delivery-open", "target": "#delivery", "tag": "button", "name": "Delivery times", "keys": ["Enter"], "opened": true, "opened_by": "Enter", "modal": true, "focus_inside": true, "escape_tried": true, "escape_closed": true, "closed": "escape"}],
"rings": [{"stop": 1, "selector": "a", "tag": "a", "name": "Skip to content", "changed": 7355}, {"stop": 2, "selector": "header > nav > a:nth-of-type(1)", "tag": "a", "name": "Home", "changed": 480}, ...]
```

The traces and crops are listed in the run's [evidence pack](Soundcheck-Accessibility-Statement#the-evidence-pack), so a change to one after the run fails `evidence verify`.

Every accessible name the module writes is masked: credentials, email addresses (`***@***`), phone numbers, card numbers that pass the Luhn check and IBANs with a valid checksum become `***`.

## Limits and warnings

Each page gets a keyboard audit in run.json: `ok`, `capped`, `timeout`, `error` or `skipped`.

| Warning | Audit | Means |
|---|---|---|
| `keyboard: <path>: stopped after 150 Tab stops (keyboard.max_tabs); raise it if the page has more` | capped | The walk counts as finished; a keyboard issue it no longer reports isn't FIXED, since it stopped before it could tell. |
| `keyboard: <path>: Tab stop <n> (<tag>) left the page; the rest of the page wasn't walked` | capped | A Tab press navigated away. |
| `keyboard: <path>: stopped after 60 s; the rest of the page wasn't walked` | timeout | The page isn't tested: under `--eaa` the run exits 3 and the statement marks it. |
| `keyboard: <path>: <key> on "<name>" left the page; skipped the rest of that widget` | | A widget's key press navigated away. |

A capped or timed-out walk never reports a trap; its other issues are reported. In a comparison, a keyboard issue that the [confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable) re-walk doesn't find is FIXED only when that re-walk finished clean; otherwise it stays UNSTABLE. The walk runs at desktop width only. It isn't a screen reader test, a voice control test or a test with disabled users.

---
<sub>← [EAA mode](Soundcheck-EAA) · [Home](Home) · [Accessibility statement](Soundcheck-Accessibility-Statement) →</sub>
