The consent banners (CMPs) `rhythm echo consent` knows how to click, how it finds the buttons, how it confirms a choice and what to do when it can't. This page also covers `consent detect`, the selector flags, `--click`, banners shown only in some countries and the reasons a run is inconclusive.

## The banners Echo knows

```text
$ rhythm echo consent cmps
CMP           DETECTED BY                                      ACCEPT       REJECT                        STATE FROM
cookiebot     #CybotCookiebotDialog, window.Cookiebot          button, api  first layer or settings, api  CookieConsent cookie
didomi        #didomi-host, window.Didomi                      button, api  first layer or settings, api  didomi_token cookie
onetrust      #onetrust-banner-sdk, window.OneTrust            button, api  first layer or settings, api  OptanonConsent cookie
usercentrics  #usercentrics-root (shadow DOM), window.UC_UI    button, api  first layer or settings, api  uc_settings storage
borlabs       #BorlabsCookieBox, window.BorlabsCookie          button       first layer or settings       borlabs-cookie cookie
complianz     #cmplz-cookiebanner-container, window.complianz  button       first layer or settings       cmplz_* cookies
cookieyes     .cky-consent-container, window.getCkyConsent     button       first layer or settings       cookieyes-consent cookie
iubenda       #iubenda-cs-banner, window._iub                  button, api  first layer or settings, api  _iub_cs-* cookies
generic       buttons whose text means accept or reject        button       first layer or settings       banner gone and stays gone
              (30 languages)
9 CMPs, rules version 1.2.0 (sha256 92bb...). A banner none of these find: --accept-selector and --reject-selector.
```

| Column | Means |
|---|---|
| DETECTED BY | the markup and the JavaScript global that identify the banner |
| ACCEPT, REJECT | how Echo can make the choice: `button` (a real click), `api` (the CMP's own function, with `--click api` or `button,api`); where Reject is: on the first layer or behind the settings |
| STATE FROM | where Echo reads the stored choice to confirm the click |

`--json` gives the same list with each rule's sources (the vendor documentation it was written from). The rules are Echo's own, written from vendor documentation and public markup; they're MIT, like Echo.

**generic** is for every other banner: a fixed, sticky or top-most box with cookie words, whose buttons are ranked by their text in 30 languages (the 24 EU languages plus Norwegian, Icelandic, Turkish, Ukrainian, Arabic and Hebrew). It never takes "continue", "close", "OK", "got it" or "x" as a choice, never clicks a link to a privacy policy and ignores newsletter, age, login and cart dialogs. The testbed has a custom banner in Arabic:

```text
$ rhythm echo consent detect https://g10.shop.test/ $map
Banner:  Generic banner, found after 0.0s (by the generic rule)
Accept:  "قبول الكل" (#site-cookie-banner > div:nth-of-type(1) > button:nth-of-type(1)), first layer
Reject:  "رفض الكل" (#site-cookie-banner > div:nth-of-type(1) > button:nth-of-type(2)), first layer
Screenshot: .echo/consent/detect/2026-10-29T02-08-38-e68c56.png
```

The benchmark also checks the real Klaro and orestbida cookieconsent scripts through the generic rule. Its Docker mode runs the real Complianz and CookieYes WordPress plugins. The commercial CMPs are tested against replicas of their public markup captured on 2026-10-26: a vendor that changes its banner can break Echo on real sites while the testbed still passes. Such a run is inconclusive, never a pass.

## How a choice is made

1. **Find the banner.** Echo watches from the start of the first load for up to `--banner-wait` (default `10s`), checking every rule. `--cmp <name>` names the one to expect: when it isn't found, the run is inconclusive (`cmp_not_found`) instead of a guess.
2. **Find the button**, inside that banner only. It must be a `button`, `[role=button]`, an `input` button or a link that doesn't leave the site; visible, with a size and the top-most element at its centre (nothing covering it). Echo waits and retries while it isn't. It looks through open shadow roots and into frames.
3. **Click** with real mouse events at the button's centre. Echo never types into a field and never clicks outside the banner.
4. **Confirm** the choice from the CMP's stored state (its cookie, storage key or API, STATE FROM above) and the banner being gone. For the generic rule, the banner must be gone and stay gone after the reload. A click that changes nothing is tried up to three times.

**Hiding a banner is never a Reject.** A Reject counts only when the stored state says so.

When Reject is behind the settings, Echo opens them, switches every non-essential toggle off, saves and reports [`consent/no-first-layer-reject`](Echo-Consent-Checks#consentno-first-layer-reject).

## consent detect

`detect` loads the page once (hits held) and prints what Echo would click. No runs, no verdict. Use it before a first check, or to write a selector.

```text
$ rhythm echo consent detect https://g03.shop.test/ $map
Banner:  Cookiebot, found after 0.0s (by #CybotCookiebotDialog and window.Cookiebot)
Accept:  "Allow all" (#CybotCookiebotDialogBodyLevelButtonLevelOptinAllowAll), first layer
Reject:  "Deny" (#CybotCookiebotDialogBodyButtonDecline), first layer
Screenshot: .echo/consent/detect/2026-10-29T02-08-11-3122a9.png
```

Reject in the settings:

```text
$ rhythm echo consent detect https://c10.shop.test/ $map
Banner:  OneTrust, found after 0.0s (by #onetrust-banner-sdk and window.OneTrust)
Accept:  "Accept All Cookies" (#onetrust-accept-btn-handler), first layer
Reject:  "Cookies Settings" (#onetrust-pc-btn-handler), second layer
Screenshot: .echo/consent/detect/2026-10-29T02-08-51-e8189c.png
```

A banner in a shadow root, as a German visitor on a phone:

```text
$ rhythm echo consent detect https://g06.shop.test/ --geo de --viewport mobile $map
Banner:  Usercentrics, found after 0.3s (by #usercentrics-root and window.UC_UI)
Accept:  "Accept All" (#usercentrics-root ::shadow [data-testid=uc-accept-all-button]), first layer
Reject:  "Deny" (#usercentrics-root ::shadow [data-testid=uc-deny-all-button]), first layer
Screenshot: .echo/consent/detect/2026-10-29T02-28-26-58f551.png
```

A banner Echo found with a button it didn't (exit `1`), then no banner at all (exit `3`):

```text
$ rhythm echo consent detect https://c11.shop.test/ $map
Banner:  Usercentrics, found after 0.3s (by #usercentrics-root and window.UC_UI)
Accept:  not found on the banner or in its settings
Reject:  not found on the banner or in its settings
Screenshot: .echo/consent/detect/2026-10-29T02-08-21-0d0b70.png

$ rhythm echo consent detect https://g12.shop.test/ $map
Banner:  none found within 10s
Screenshot: .echo/consent/detect/2026-10-29T02-08-24-5f2785.png
```

`detect` takes the browser and request flags, `--cmp`, the selector flags, `--banner-wait`, `--geo`, `--locale`, `--timezone`, `--viewport` and `--json`. `--json` gives each button's selector, text, layer and box:

```json
{
  "url": "https://g06.shop.test/",
  "cmp": {
    "id": "usercentrics",
    "name": "Usercentrics",
    "found": true,
    "present": true,
    "detected_by": "selector:#usercentrics-root and global:UC_UI",
    "banner_at_ms": 289,
    "tcf": false,
    "reject_layer": 0
  },
  "accept": {
    "how": "button",
    "selector": "#usercentrics-root ::shadow [data-testid=uc-accept-all-button]",
    "text": "Accept All",
    "layer": 1,
    ...
  },
  ...
  "screenshot": ".echo/consent/detect/2026-10-29T02-28-50-60d54c.png",
  "browser": "Chrome 153.0.8010.52"
}
```

Screenshots are kept in `.echo/consent/detect/`.

## Your own selectors

When Echo doesn't find a button, name it. `--accept-selector` and `--reject-selector` (config `accept_selector:`, `reject_selector:`) take CSS, plus two steps of their own:

| Form | Clicks |
|---|---|
| `#reject-all` | that element |
| `#settings >> #reject-all` | `#settings`, then `#reject-all` (a second layer) |
| `host-element::shadow button.deny` | a button inside `host-element`'s open shadow root |

```sh
rhythm echo consent https://shop.example --reject-selector "#manage >> #reject-all"
```

The selector still has to point inside the banner, be visible and be on top. The choice is still confirmed from the stored state. A selector that matches nothing is named in the result instead of a guess:

```text
$ rhythm echo consent detect https://g10.shop.test/ --accept-selector '#site-cookie-banner button:nth-of-type(1)' --reject-selector '#nope' $map
Banner:  Generic banner, found after 0.0s (by the generic rule)
Accept:  "قبول الكل" (#site-cookie-banner button:nth-of-type(1)), first layer
Reject:  not found on the banner or in its settings
```

A selector the browser can't parse stops the command before the page loads (exit `2`).

## --click

| `--click` | Does |
|---|---|
| `button` (default) | click the visible button; nothing else |
| `api` | call the CMP's own function (fixed in Echo, never page code) instead of clicking |
| `button,api` | the button, then the API when no button is found or the click doesn't take |

`api` works only where the table above lists it. With `button,api`, a choice made through the API is reported as [`consent/click-fallback`](Echo-Consent-Checks#consentclick-fallback), because the button wasn't tested. A CMP in a closed shadow root (the testbed's `c11`) can only be checked that way:

```text
$ rhythm echo consent https://c11.shop.test/ --click button,api $map
...
  [2/3] Accept      FAIL YouTube embed loaded before the choice; Google Ads, Google Analytics 4 fired; update granted at 3.2s; kept on 1 more page
  [3/3] Reject      FAIL YouTube embed loaded before the choice; update denied at 3.2s; kept on 1 more page
...
  notice  consent/click-fallback  Echo couldn't click the button and used Usercentrics's API, so the button itself wasn't tested (Accept)
  notice  consent/click-fallback  Echo couldn't click the button and used Usercentrics's API, so the button itself wasn't tested (Reject)
```

## Banners shown in some countries

Many banners show only to visitors from some countries. Echo can't change its IP address. It can look like a visitor from elsewhere:

| Flag | Sets |
|---|---|
| `--geo <cc>` | the language, `Accept-Language` and time zone from a built-in table, plus a CMP's documented geolocation override when its rule has one |
| `--locale <tag>` | the browser locale and `Accept-Language`, e.g. `de-DE` (default the machine's, else `en-US`) |
| `--timezone <tz>` | an IANA time zone, e.g. `Europe/Berlin` (default the machine's) |
| `--proxy <url>` | send the browser's traffic on through a proxy, e.g. one in the EU |
| `--viewport mobile` | 390x844 with touch and a mobile User-Agent (default `desktop`, 1366x900) |

`--geo` knows the 27 EU countries, Norway, Iceland, Liechtenstein, Switzerland, the UK (`gb` or `uk`), the US, Canada, Brazil, Australia, Japan and India:

```text
rhythm echo: --geo "zz": Echo has no language and time zone for that country; use --locale and --timezone, or an EU or EEA country, GB, CH, US, CA, BR, AU, JP or IN
```

When no banner shows while trackers fire, the result says so and suggests these: `Many banners show only to visitors in some countries: try --geo de, or an EU --proxy.` A site that hides its banner from automated browsers is a blind spot; Echo's User-Agent drops `Headless`; it can't hide everything.

## When a run can't make its choice

The run is inconclusive, tried once more in a fresh profile (`--retries`, `0` to `3`) and then reported with a reason. Inconclusive runs make the check exit `3` unless an issue at or above `--fail-on` was found.

| Reason | Means | Try |
|---|---|---|
| `no_banner` | a CMP was detected or tracker scripts loaded but no banner showed in `--banner-wait` | a longer `--banner-wait`, `--geo`, `--proxy` |
| `cmp_not_found` | `--cmp` named a banner that wasn't there | `detect`, or `--cmp auto` |
| `no_button` | the banner showed but Echo found no Accept or Reject on it or in its settings | `--accept-selector`, `--reject-selector`, `--click button,api` |
| `click_missed` | the click landed but the banner didn't close | `detect`; something may cover the button |
| `state_unchanged` | the banner closed but the stored state didn't change: hiding is not choosing | a CMP rule, or report it |
| `navigation_failed` | the page didn't load | is the site up? |
| `timeout` | the run took longer than `--timeout` (default `90s`) | a longer `--timeout` |
| `limit` | the run hit 20,000 requests | a lighter page |
| `canceled` | interrupted | |

```text
  INCONCLUSIVE /  Accept: --cmp iubenda was named but its banner wasn't found
  INCONCLUSIVE /  Reject: --cmp iubenda was named but its banner wasn't found
  Many banners show only to visitors in some countries: try --geo de, or an EU --proxy.
  A banner Echo doesn't know: --accept-selector and --reject-selector (consent detect helps).
```

`consent.json` keeps each inconclusive run under `inconclusive[]` with its page, run, reason and message. Each run has a `status`: `ok`, `no-banner`, `no-button`, `click-failed`, `load-failed`, `timeout`, `navigated-away`, `limit-reached` or `canceled`.

## Adding a banner

A banner the generic rule can't handle is worth a rule. Rules live in `consent/cmp/rules/<id>.yaml` in the Echo repository: how to detect it, its Accept, Reject and second layer, how to reopen it, its API and where its stored choice lives, with the sources it was written from and positive and negative fixtures. The format is in [`consent/cmp/rules/README.md`](https://github.com/RhythmShahriar/echo/blob/master/consent/cmp/rules/README.md); see also [Contributing](Contributing). Until then, `--accept-selector` and `--reject-selector` in the config do the job.

---
<sub>← [Consent Mode](Echo-Consent-Mode) · [Home](Home) · [Consent in CI](Echo-Consent-CI) →</sub>
