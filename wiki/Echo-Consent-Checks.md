Every check `rhythm echo consent` runs: what it finds, a real example and the fix. This page also covers tuning a check's level, ignoring a known issue until a date and what each comparison class means.

`rhythm echo checks` lists them with the rest of Echo's checks, under the channel `consent`. IDs stay the same between releases, so a config or a CI rule that names one keeps working.

## The checks

| Check | Level | Finds |
|---|---|---|
| [`consent/before-choice`](#consentbefore-choice) | error | A tracker that needs consent sent a hit, set a cookie or wrote storage before the visitor chose. |
| [`consent/after-reject`](#consentafter-reject) | error | The same after Reject, on the page, a later page or the reload. |
| [`consent/after-withdraw`](#consentafter-withdraw) | error | The same after the visitor withdrew consent (run `withdraw`). |
| [`consent/reject-granted`](#consentreject-granted) | error | After Reject, a Consent Mode update granted `ad_storage` or `analytics_storage`, or a Google hit carried `gcs` `G101`, `G110` or `G111`. |
| [`consent/reject-not-kept`](#consentreject-not-kept) | error | After Reject, the banner showed again on a later page or the reload, or the CMP's stored choice was gone. |
| [`consent/reject-broken`](#consentreject-broken) | error | The Reject button was clicked and didn't take while the CMP's API reject worked, so visitors can't reject (`--click button,api`). |
| [`consent/no-default`](#consentno-default) | error | Google tags sent hits with no `gcs` and no Consent Mode default was ever set. |
| [`consent/server-side`](#consentserver-side) | error | The site's server sent a Meta Conversions API or GA4 Measurement Protocol event for a visitor who rejected or withdrew (`--echo`). |
| [`consent/script-before`](#consentscript-before) | error | A tracker script, Google Tag Manager included, loaded before the choice or after Reject (`--consent-mode strict` only). |
| [`consent/default-late`](#consentdefault-late) | warning | The Consent Mode default was set only after the first Google hit. |
| [`consent/v2-signals`](#consentv2-signals) | warning | The default lacks `ad_user_data` or `ad_personalization`, or a hit's `gcd` shows them unset. |
| [`consent/accept-not-granted`](#consentaccept-not-granted) | warning | After Accept, no Consent Mode update granted anything and Google hits still said denied. |
| [`consent/no-first-layer-reject`](#consentno-first-layer-reject) | warning | The banner has no Reject on its first layer, so Echo had to open its settings. |
| [`consent/no-banner`](#consentno-banner) | warning | Trackers that need consent fired or set cookies while no banner showed. |
| [`consent/gpc-ignored`](#consentgpc-ignored) | warning | Advertising trackers fired or set cookies for a visitor who sent Global Privacy Control (run `gpc`). |
| [`consent/tcf-mismatch`](#consenttcf-mismatch) | warning | The CMP's TCF consent string granted a purpose after Reject, or granted none after Accept. |
| [`consent/embed`](#consentembed) | warning | An embed such as YouTube, Vimeo without `dnt=1` or Google Maps loaded or set cookies before the choice or after Reject. |
| [`consent/cookieless-ping`](#consentcookieless-ping) | notice | Google tags sent cookieless pings before the choice or after Reject (`--consent-mode advanced`). |
| [`consent/meta-no-consent-api`](#consentmeta-no-consent-api) | notice | Meta Pixel fired before the choice or after Reject and never received `fbq('consent', ...)`. |
| [`consent/click-fallback`](#consentclick-fallback) | notice | Echo couldn't find the button and made the choice through the CMP's API, so the button itself wasn't tested. |
| [`consent/ignore-expired`](#consentignore-expired) | notice | An `ignore:` entry passed its `until` date and no longer hides its issue. |

Every issue names its policy, a one-line fix and the events that prove it (their sequence numbers in `consent.json`, linked in `report.html`).

What never counts, so a correct site passes with no warning:

- trackers that need no consent: exempt ones (Plausible, Fathom, or any you name with `--exempt`) and necessary services (Stripe.js, PayPal checkout, reCAPTCHA, hCaptcha, Cloudflare Turnstile, Google Fonts), which reports list under "Seen, no consent needed";
- a tracker's script loading, unless the policy is `strict` (a tag script loading is not a hit);
- Google's cookieless pings under the default policy `advanced` (they're `consent/cookieless-ping`);
- requests Chrome makes on its own, outside any page;
- cookies you add with `--cookie`.

The examples below are real output from the consent testbed (see [Try it on the testbed](Echo-Consent#try-it-on-the-testbed)).

## Errors

### consent/before-choice

A tracker that needs consent sent a hit, set a cookie or wrote storage before the visitor chose. In the No choice run, everything counts; in Accept and Reject, everything before the click. A hit at the click's millisecond or later is after the choice.

```text
  error   consent/before-choice   Google Analytics 4 fired before the visitor chose (No choice)
          3 hits (page_view on /, /page-2/, /); cookie _ga written; cookie _ga_••••D001 written
          fix: Set a Consent Mode default (all four signals denied) before the Google tag, or let
          the banner block it until consent
```

Session replay is caught by its storage too:

```text
  error   consent/before-choice   Hotjar fired before the visitor chose (Accept, /)
          in.hotjar.com/api/v2/client/sites/1000001/visit-data?sv=6&_tbrun=c09 at 3.08s; cookie
          _hjSessionUser_1000001 written; cookie _hjSession_1000001 written; local storage key
          _hjLocalStorageTest written; session storage key _hjSessionStorageTest written
          fix: Load Hotjar only after the visitor accepts analytics or session recording
```

The fix comes from the tracker's catalogue entry. For Google tags it's a Consent Mode default (see [Consent Mode](Echo-Consent-Mode)); for others, letting the banner block the script until Accept.

### consent/after-reject

The same, after the Reject click: on the page, the next page or the reload.

```text
  error   consent/after-reject    Meta Pixel fired after Reject (Reject)
          2 hits (PageView on /page-2/, /)
          fix: Meta Pixel doesn't read Google Consent Mode. Call fbq('consent', 'revoke') before
          fbq('init'), then fbq('consent', 'grant') when the banner reports marketing consent, or
          let the banner block the pixel script.
```

When a hit after Reject carries a granted Consent Mode state, it's reported as `consent/reject-granted` instead, not both.

### consent/after-withdraw

Run `withdraw` (`--runs +withdraw`) accepts, reopens the banner, rejects and keeps browsing. Anything that needs consent after the second choice is this error:

```text
  [5/5] Withdraw    FAIL Meta Pixel fired after consent was withdrawn on 2 of 3 pages; update denied at 7.2s; kept on 1 more page
...
  error   consent/after-withdraw  Meta Pixel fired after consent was withdrawn (Withdraw)
          2 hits (PageView on /page-2/, /); cookie _fbp written
          fix: Meta Pixel doesn't read Google Consent Mode. Call fbq('consent', 'revoke') before
          fbq('init'), then fbq('consent', 'grant') when the banner reports marketing consent, or
          let the banner block the pixel script.
```

When the banner can't be reopened after Accept, the run is inconclusive: `Withdraw: Accept worked but the banner couldn't be reopened to withdraw`, with what to try.

### consent/reject-granted

Reject was sent to Google as "granted", usually a CMP's category map or a tag manager's consent tag pointing the wrong way.

```text
  error   consent/reject-granted  After Reject, Google was told analytics_storage is granted (Reject)
          dataLayer.push(['consent', 'update']) at 3.19s granted analytics_storage;
          dataLayer.push(['consent', 'update']) at 6.84s granted analytics_storage;
          dataLayer.push(['consent', 'update']) at 10.38s granted analytics_storage; 3 Google hits
          with a granted gcs, the first Google Analytics 4 with gcs G101 at 3.21s
          fix: Map the banner's Reject to a Consent Mode update with every signal denied; check the
          CMP's Consent Mode settings or the GTM consent tag.
```

Only signals sent after the click count. A `gcs` or `gcd` Echo can't read is never taken as granted.

### consent/reject-not-kept

The choice didn't survive the next page or the reload: the banner came back, or the CMP's stored choice was gone.

```text
  error   consent/reject-not-kept  After Reject, the banner came back on /page-2/ (Reject)
          the banner showed again on the next page (/page-2/) at 4.75s; the banner showed again on
          the reload (/) at 7.78s
          fix: Keep the visitor's choice: check the CMP cookie's domain, path and expiry and that
          nothing clears it on the next page.
```

A banner that never closed after the click isn't this check: that run is inconclusive (`click_missed`).

### consent/reject-broken

With `--click button,api`, Echo found the Reject button and clicked it, the choice wasn't confirmed after retries and the CMP's own API reject then worked. Visitors can't reject with the button. It needs both the failed button and the working API, so it's rare and no testbed scenario forces it yet. The testbed's "click does nothing" site (c20: an overlay over the page and a Reject handler bound 6 s late) usually passes, because Echo waits for the button to be the top-most element and retries the click. When Reject truly does nothing, fix the button: a transparent overlay on top of it or a handler that never binds are the usual causes.

### consent/no-default

Google tags sent hits with no `gcs` and Echo saw no Consent Mode default in any run. Raised only when Google tags sent hits.

```text
  error   consent/no-default      Google tags ran with no Consent Mode default (No choice)
          Google Ads sent
          googleads.g.doubleclick.net/pagead/viewthroughconversion/••••0001/?cv=11&fst=1793218059689
          at 0.07s with no gcs; no gtag('consent', 'default', ...) was seen in any run
          fix: Set a Consent Mode default with every signal denied before any Google tag loads, e.g.
          gtag('consent', 'default', {...}) at the top of the head.
```

### consent/server-side

With `--echo`, Echo asks the Echo catcher what the site's server sent while each run was going on. A Meta Conversions API or GA4 Measurement Protocol event during the Reject or Withdraw run is this error. Same-site requests carry an `X-Echo-Run` header, which Echo's connectors copy into what the server sends, so only this run's events count.

```text
  [3/3] Reject      FAIL Meta Conversions API received PageView from the server after Reject; nothing fired; update denied at 3.2s; kept on 1 more page
...
  error   consent/server-side     Meta Conversions API received PageView from the server after Reject (Reject)
          2 messages caught by Echo, the first at 4.84s (914471bfb5b4, 58d17c8a393e)
          fix: Send server-side events only for visitors who accepted: pass the consent state to the
          server and check it before calling the API.
```

It needs the site connected to Echo (`rhythm echo up` and a [connector](Echo-Install-Connectors)). When Echo doesn't answer, the check says so and goes on: `Echo at http://127.0.0.1:39142 didn't answer; server-side tracking wasn't checked.`

### consent/script-before

Only under `--consent-mode strict`: a tracker script, Google Tag Manager included, loaded before the choice or after Reject.

```text
  error   consent/script-before   Google Tag Manager script loaded before the visitor chose (strict policy) (No choice)
          3 loads (googletagmanager.com/gtm.js?id=••••BED1)
          fix: Under strict, load no tracker script before consent: let the CMP block Google Tag
          Manager and the other tags until Accept.
```

## Warnings

### consent/default-late

A Consent Mode default exists but came after the first Google hit. Not raised when `no-default` is.

```text
  warning consent/default-late    The Consent Mode default came 1501 ms after the first Google hit (No choice)
          first Google hit Google Analytics 4
          (google-analytics.com/g/collect?tid=••••D001&en=page_view) at 0.06s; default at 1.56s
          fix: Move the Consent Mode default above the Google tag or GTM snippet, so it is set
          before the first hit.
```

### consent/v2-signals

Consent Mode v2 added `ad_user_data` and `ad_personalization`. The default doesn't set them, or a hit's `gcd` shows them unset (`l`).

```text
  warning consent/v2-signals      Consent Mode v2 signals ad_user_data and ad_personalization were never set (No choice)
          the default at 0.02s doesn't set ad_user_data and ad_personalization; gcd 13r3r3l3l5 on
          Google Ads at 3.25s shows ad_user_data and ad_personalization unset (l)
          fix: Add ad_user_data and ad_personalization to the Consent Mode default and update
          (Consent Mode v2).
```

### consent/accept-not-granted

The visitor accepted but Google was never told: the site loses measurement it was allowed. Only with Google tags present and the Accept confirmed.

```text
  warning consent/accept-not-granted  After Accept, Google was never told storage is granted (Accept)
          2 Google hits after Accept, the first at 5.78s, carried gcs G100; no Consent Mode update
          granted ad_storage or analytics_storage
          fix: Send a Consent Mode update with the accepted signals granted when the visitor
          accepts.
```

### consent/no-first-layer-reject

Reject isn't on the banner itself, so Echo opened the settings, switched the toggles off and saved. The Reject run still counts as made.

```text
  [3/3] Reject      FAIL Reject isn't on the banner: Echo had to open settings; nothing fired; update denied at 9.3s; kept on 1 more page
...
  warning consent/no-first-layer-reject  Reject isn't on the banner: Echo had to open settings (Reject)
          Reject was on layer 2: Echo clicked "Confirm My Choices" (#onetrust-pc-btn-handler >>
          #onetrust-pc-sdk .save-preference-btn-handler)
          fix: Put a Reject button on the banner itself, as visible as Accept.
```

### consent/no-banner

Trackers that need consent fired or set cookies and no banner showed. Never raised on a page with no tracker activity: a brochure site with no trackers and no banner passes.

```text
  warning consent/no-banner       Google Ads and Google Analytics 4 are on the page but no consent banner showed (No choice)
          Google Ads and Google Analytics 4 are on the page but no consent banner showed within 10s.
          Many banners show only to visitors in some countries: try --geo de, or an EU --proxy. A
          banner Echo doesn't know: --accept-selector and --reject-selector
          fix: Show a consent banner before these trackers run, or remove them; if the banner is
          geo-targeted, check with --geo or a proxy in that region.
```

That example is a banner hidden from browsers that say `HeadlessChrome` (the testbed's c17, checked with that User-Agent). Echo's own default User-Agent drops `Headless`.

### consent/gpc-ignored

In run `gpc` (`--runs +gpc`), the browser sends `Sec-GPC: 1` and sets `navigator.globalPrivacyControl` and Echo makes no click. Advertising trackers that fire are this warning; analytics alone isn't.

```text
  warning consent/gpc-ignored     Meta Pixel fired with Global Privacy Control on (GPC)
          3 hits (PageView on /, /page-2/, /); cookie _fbp written
          fix: Treat Global Privacy Control as an opt-out of sale and sharing: many CMPs have a
          setting to honor it.
```

### consent/tcf-mismatch

An IAB TCF CMP's TC string, read after the visitor's choice, grants a purpose after Reject or none after Accept.

```text
  warning consent/tcf-mismatch    The TCF string grants purposes 1 and 3 after Reject (Reject)
          TC string read at 7.87s (CMP 7, 0 vendors with consent)
          fix: Check the CMP's TCF settings: the TC string must match the choice the visitor made.
```

### consent/embed

An embed loaded or set cookies before the choice or after Reject: YouTube on `youtube.com`, Vimeo without `dnt=1`, Google Maps. `youtube-nocookie.com` and click-to-load placeholders never count.

```text
  warning consent/embed           YouTube embed loaded before the visitor chose (No choice)
          6 hits (on /, /page-2/, /); cookie VISITOR_INFO1_LIVE written; cookie YSC written
          fix: Embed the video from www.youtube-nocookie.com or behind a click-to-load placeholder
          that loads the player only after consent
```

## Notices

### consent/cookieless-ping

Under the default policy `advanced`: Google hits with `gcs` `G100` that wrote no Google cookie, before the choice or after Reject. They don't fail a check. `--consent-mode basic` counts them as hits. See [Consent Mode](Echo-Consent-Mode#cookieless-pings).

```text
  notice  consent/cookieless-ping  Google Analytics 4 sent 2 cookieless pings before the choice (allowed under --consent-mode advanced) (No choice)
          google-analytics.com/g/collect?gcs=G100 at 0.53s and 4.79s; no Google cookie written
          fix: These are Consent Mode advanced pings without cookies; switch to basic mode if your
          policy counts them as tracking.
```

### consent/meta-no-consent-api

Raised with a Meta Pixel error, as the explanation: the pixel never got `fbq('consent', ...)`, which is the only consent signal it reads.

```text
  notice  consent/meta-no-consent-api  Meta Pixel never received fbq('consent', ...) (No choice)
```

### consent/click-fallback

Only with `--click button,api`: Echo couldn't use the button and made the choice through the CMP's own API. The run counts; the button wasn't tested.

```text
  notice  consent/click-fallback  Echo couldn't click the button and used Usercentrics's API, so the button itself wasn't tested (Accept)
```

### consent/ignore-expired

An `ignore:` entry passed its `until` date. The issue it hid counts again and this notice says so:

```text
  error   consent/before-choice   Meta Pixel fired before the visitor chose (No choice)
...
  notice  consent/ignore-expired  The ignore for consent/before-choice on meta-pixel expired on 2026-10-01; the issue counts again
```

## Tuning a check

Levels live in the top-level `checks:` map of `.echo/config.yaml`, as for Echo's other checks: `error`, `warning`, `notice` or `off`.

```yaml
# .echo/config.yaml
checks:
  consent/cookieless-ping: warning
  consent/meta-no-consent-api: "off"
```

```text
Findings - 0 errors, 1 warning, 0 notices - 1 run, 1 tracker, policy advanced
  warning consent/cookieless-ping  Google Analytics 4 sent 2 cookieless pings before the choice (allowed under --consent-mode advanced) (No choice)
```

`--fail-on` decides which levels fail: `error` by default, `warning` or `notice` for stricter. A misspelt ID is a usage error that names the closest one:

```text
rhythm echo: .../tune/.echo/config.yaml: line 2: checks: consent/cookieless-pings isn't a check; did you mean consent/cookieless-ping?
```

## Ignoring a known issue

When a fix is planned but not live yet, `consent: ignore:` hides one issue until a date. Each entry needs `check`, `reason` and `until` (at most a year ahead); `tracker`, `page` and `run` narrow it.

```yaml
consent:
  ignore:
    - check: consent/before-choice
      tracker: hotjar
      page: /
      run: none
      reason: "client signs off until the Nov launch"
      until: 2026-11-30
```

The issue still shows, marked as ignored; it no longer counts toward `--fail-on`:

```text
  error   consent/before-choice   Hotjar fired before the visitor chose (No choice, /) (ignored until 2026-11-30: client signs off until the Nov launch)
```

After `until` it counts again, with [`consent/ignore-expired`](#consentignore-expired). A page given as a path matches it on any host; `host/path` or a URL matches that host only.

## Comparison classes

With `--against`, the result is the change from the baseline, matched by each issue's key (`check|page|run|tracker`, `-v` prints it):

| Class | Means |
|---|---|
| **BROKE** | a new issue at error level: a tracker now fires before the choice or after Reject, Reject is now granted, the choice is no longer kept |
| **NEW** | a new tracker, tracker cookie or CMP, or a new warning or notice |
| **CHANGED** | the consent signals differ for the same page and run: the Consent Mode default, a `gcd` signal, the TCF purposes, the CMP, the Reject layer |
| **FIXED** | an issue in the baseline that is gone |

`--fail-on` then takes `broke` (the default), `new`, `changed` or `none`. `--accept <key>` takes one change as intended. See [Consent in CI](Echo-Consent-CI#baselines).

## Custom trackers

A tracker Echo doesn't know is invisible to every check above. Add it under `consent: trackers:`, in the same shape as the built-in catalogue, with fixtures that prove the pattern:

```yaml
consent:
  trackers:
    - id: shop-pixel
      name: Shop pixel
      category: advertising           # analytics, advertising, session-replay, social, embed, functional
      hits:
        - host: px.shop.test
          path: /collect
      cookies: [_shpx]
      fixtures:
        match: ["https://px.shop.test/collect?e=view"]
        no_match: ["https://px.shop.test/pixel.js"]
```

`rhythm echo checks --test` tries the fixtures; a tracker without both, or whose fixtures fail, makes `consent` refuse the config (exit `2`):

```text
ok   shop-pixel  match    https://px.shop.test/collect?e=view
ok   shop-pixel  no_match https://px.shop.test/pixel.js
```

`rhythm echo consent trackers` then lists it with the others (`48 trackers` with one of your own).

---
<sub>← [Cookie consent](Echo-Consent) · [Home](Home) · [Consent Mode](Echo-Consent-Mode) →</sub>
