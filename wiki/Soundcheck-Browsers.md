Soundcheck runs in Chrome by default. With `--browser firefox` or `--browser webkit` it checks the site in Firefox or in WebKit (Safari's engine) instead, to catch what breaks in only one browser. This page covers picking an engine, what runs in each and how comparisons handle them. For Chrome itself (the download, the sandbox), see the [FAQ](Soundcheck-FAQ#the-browser).

```sh
rhythm soundcheck snapshot staging.yoursite.com --name ff --browser firefox
# …make your change…
rhythm soundcheck compare
```

## --browser

| Value | Uses |
|---|---|
| `chrome` (default), `chromium` | an installed Chrome or Chromium, else the downloaded Chrome for Testing |
| `firefox` | an installed Firefox: `firefox`, `firefox-esr`, `firefox-developer-edition` or `firefox-nightly` on PATH, or `/Applications/Firefox.app` on macOS |
| `webkit`, `safari` | a WebKit driver: `WebKitWebDriver` or `WPEWebDriver` (Linux) or `safaridriver` (macOS) |
| a path, e.g. `/usr/bin/google-chrome` | that Chrome binary |
| `firefox=<path>`, `webkit=<path>` | that engine's binary |

Engine names ignore case. Soundcheck downloads only Chrome; Firefox and WebKit must be installed. A typo stops before the run, exit 2:

```text
rhythm soundcheck: --browser: "firefx" isn't chrome, firefox or webkit (did you mean firefox?)
```

Without the engine installed, the run stops with exit 3:

```text
rhythm soundcheck: no WebKit driver found; install WebKitWebDriver (Linux) or enable safaridriver (macOS), or pass the path to it
```

For Firefox the message is `no Firefox found; install it or pass the path to its binary`. On macOS, turn safaridriver on once with `safaridriver --enable`.

## What runs in Firefox and WebKit

Firefox runs headless in a throwaway profile and is driven through WebDriver BiDi. Health, visual, cache and exec run as in Chrome. The modules that work inside the page through Chrome's DevTools protocol are skipped, each with a note:

```text
Snapshot "ff" saved; all clear - 0 errors, 0 warnings, 1 notice - 4 pages · 9s
note: speed was skipped in firefox: it needs Chrome
note: interact was skipped in firefox: it needs Chrome
```

| Module | Chrome | Firefox | WebKit |
|---|---|---|---|
| health | yes | yes | yes; with a classic-only driver, no console, network or status facts |
| visual | yes | yes | yes; with a classic-only driver, viewport-only screenshots |
| cache | yes | yes | yes |
| exec | yes | yes | yes |
| interact (and so echo, which records what form submits send) | yes | skipped | skipped |
| speed | yes | skipped | skipped |
| a11y, seo | yes | skipped | skipped |
| logging in | yes | skipped (`the login was skipped in firefox: logging in needs Chrome`) | skipped |

Other differences you may see:

- Network errors keep the engine's words: `NS_ERROR_CONNECTION_REFUSED` where Chrome says `net::ERR_CONNECTION_REFUSED`.
- The phone pass outside Chrome only sets the 390 x 844 window size, with no phone emulation, so `Not responsive` can't fire there.
- Chrome's own console messages (CSP and mixed-content warnings from the browser) have no Firefox counterpart.
- WebKit drivers that offer only classic WebDriver can't send `-H`, `--cookie` or `--user-agent` to the site alone, so each page fails with `not supported by this browser` rather than sending them everywhere.

Screenshots of another engine are stored as `shots/<page>-<viewport>-<engine>.png`.

## Comparing

`compare` runs in the snapshot's engine unless you pass `--browser`, as it does for modules, levels and viewports. The settings line names it:

```text
Settings: from snapshot "ff" (modules health,visual,cache; levels inspect,click; viewports desktop,mobile; browser firefox)
```

A comparison in Firefox reads like one in Chrome:

```text
[BROKE] Page not found (404) - /about.php
  fix: Restore the page, or redirect it and update links to it.
[NEW] Broken link to /about.php (404) - /, /contact.php, /shop.php
  fix: Update or remove the link, or redirect the old URL.
[NEW] PHP Warning: Undefined array key "layout" - /, /contact.php, /shop.php
  source: themes/acme/footer.php:42
  fix: Fix the code at the source line and turn display_errors off in production so visitors never see these.
[NEW] JS error: Uncaught ReferenceError: undefinedStoreLocator is not defined - /
  source: inline script
  fix: Open the browser console on this page to see the full stack.
```

Passing another engine than the snapshot's is a change you made, listed with the others (`You changed: modules health,visual,cache -> health (--modules); browser firefox -> chrome (--browser)`). Screenshots and speed only compare within one engine, so the comparison warns `browser engines differ (snapshot firefox, now chrome); screenshots and speed only compare in the same engine`. With `--strict` it stops instead:

```text
rhythm soundcheck: --strict: this comparison wouldn't use snapshot "ff"'s settings: browser firefox -> chrome (--browser)
```

To cover more than one browser, keep a snapshot per engine and compare each:

```sh
rhythm soundcheck snapshot staging.yoursite.com --name chrome
rhythm soundcheck snapshot staging.yoursite.com --name ff --browser firefox
# …make your change…
rhythm soundcheck compare --snapshot chrome
rhythm soundcheck compare --snapshot ff
```

In a [fleet](Soundcheck-Fleet-And-Client-Reports), a site's `args` may name an engine (`--browser firefox`) but never a path.

---
<sub>← [Import from BackstopJS and Lost Pixel](Soundcheck-Import) · [Home](Home) · [CLI reference](Soundcheck-CLI-Reference) →</sub>
