How `rhythm echo consent` reads Google Consent Mode: the default and update commands, the `gcs` and `gcd` values on each Google hit, the three policies and what a cookieless ping is. It also covers the consent signals of trackers that don't read Consent Mode at all, such as the Meta Pixel.

## What Echo reads

Consent Mode tells Google's tags what the visitor allowed. A site sets a **default** before its Google tags load (usually every signal denied) and sends an **update** when the visitor chooses:

```js
gtag('consent', 'default', {
  ad_storage: 'denied', analytics_storage: 'denied',
  ad_user_data: 'denied', ad_personalization: 'denied'
});
// later, from the banner:
gtag('consent', 'update', { ad_storage: 'granted', analytics_storage: 'granted', ad_user_data: 'granted', ad_personalization: 'granted' });
```

`ad_user_data` and `ad_personalization` are the two Consent Mode v2 signals. Echo reads the commands as the page sends them (`gtag('consent', ...)` and `dataLayer.push(['consent', ...])`) and every Google hit's `gcs` and `gcd`, then puts both on one clock with the click. **The hits decide**: the commands explain what Google was told, they never clear a hit.

The page line says what the default was and when:

```text
  Consent Mode: v2 default, all four signals denied at 0.02s, before the first Google hit
  Consent Mode: v2 default, all four signals denied at 1.56s, after the first Google hit
  Consent Mode: default, ad_storage and analytics_storage denied at 0.02s
  Consent Mode: no default; Google hits carry gcd only, every signal unset
```

The first is correct. The second is [`consent/default-late`](Echo-Consent-Checks#consentdefault-late), the third [`consent/v2-signals`](Echo-Consent-Checks#consentv2-signals) and the fourth [`consent/no-default`](Echo-Consent-Checks#consentno-default). Each run line then gives the update and its time: `update granted at 3.2s`, `update denied at 3.2s`.

## gcs

`gcs` is on most Google hits. It's `G1` and one character for each of `ad_storage` and `analytics_storage`: `0` denied, `1` granted, `-` unset.

| gcs | ad_storage | analytics_storage |
|---|---|---|
| `G100` | denied | denied |
| `G101` | denied | granted |
| `G110` | granted | denied |
| `G111` | granted | granted |
| `G1--` | unset | unset |

Anything else is shown raw as an unknown `gcs` and never read as granted. After Reject, a hit with `G101`, `G110` or `G111` is [`consent/reject-granted`](Echo-Consent-Checks#consentreject-granted).

## gcd

`gcd` says more: the default and the update of all four signals. It isn't documented by Google; Echo reads it with the table the community has worked out (Stape, Didomi, Complianz) and every report says `gcd decoded with an unofficial, community-documented mapping`.

Shape: `11` or `13`, then a letter and a digit for each of `ad_storage`, `analytics_storage`, `ad_user_data` and `ad_personalization`, ending in `5` or `7`. For example `13p3p3p3p5`.

| Letter | Default | Update |
|---|---|---|
| `l` | not set | not set |
| `p` | denied | not set |
| `q` | denied | denied |
| `t` | granted | not set |
| `r` | denied | granted |
| `m` | not set | denied |
| `n` | not set | granted |
| `u` | granted | denied |
| `v` | granted | granted |
| `e` | unknown | granted (one source lists it) |

So `13p3p3p3p5` is a default with all four denied and no update yet; `13r3r3r3r5` is all four denied by default, then granted; `13l3l3l3l5` means Consent Mode was never set. A letter outside the table makes that signal unknown and the wrong shape makes all four unknown; neither ever decides a verdict. `-v` and `report.html` show both, the raw value next to the reading:

```text
     3.20s  hit    Google Analytics 4 page_view gcs=G111 gcd=13r3r3r3r5 (held)
```

## Policies

`--consent-mode` (config `consent_mode:`) says how Google hits sent with storage denied count. Every result names the policy that decided it.

| Policy | Cookieless pings before the choice or after Reject | Tag scripts before the choice |
|---|---|---|
| `advanced` (default) | notice `consent/cookieless-ping` | not judged |
| `basic` | errors, like any hit | not judged |
| `strict` | errors, like any hit | errors: `consent/script-before`, Google Tag Manager included |

Google's own words for advanced mode are that its tags "send measurements without cookies" while consent is denied. Whether that is acceptable before consent is disputed; a regional court has held that even loading Google Tag Manager needs prior consent. The default reports the pings and lets a correct advanced setup pass with no warning. Pick the policy your adviser agrees with and put it in the config so every run and every colleague uses it.

### Cookieless pings

A cookieless ping is a Google hit whose `gcs` says both storage signals are denied (`G100`) and that wrote no Google cookie in that phase. The testbed's `g02` is a correct Consent Mode advanced setup. With the default policy:

```text
  [1/1] No choice   ok Google Analytics 4: 2 cookieless pings (gcs G100); no tracker cookies
     0.00s  load   https://g02.shop.test/
     0.01s  script googletagmanager.com/gtag/js?id=••••D001
     0.01s  gtag   consent default  all four denied
     0.02s  script connect.facebook.net/en_US/fbevents.js
     0.02s  banner Complianz shown
     0.53s  hit    Google Analytics 4 page_view gcs=G100 gcd=13p3p3p3p5 cookieless (held)
     4.23s  reload https://g02.shop.test/
...
All clear - 0 errors, 0 warnings, 1 notice - 1 run, 1 tracker, policy advanced

  notice  consent/cookieless-ping  Google Analytics 4 sent 2 cookieless pings before the choice (allowed under --consent-mode advanced) (No choice)
          (key: consent/cookieless-ping|/|none|ga4)
          google-analytics.com/g/collect?gcs=G100 at 0.53s and 4.79s; no Google cookie written
          fix: These are Consent Mode advanced pings without cookies; switch to basic mode if your
          policy counts them as tracking.
```

The same site under `--consent-mode basic` (exit `1`):

```text
https://g02.shop.test/
  Banner: Complianz, found after 0.1s, Reject on the banner
  Consent Mode: v2 default, all four signals denied at 0.03s, before the first Google hit
  [1/3] No choice   FAIL Google Analytics 4 fired before the choice; Meta Pixel held back
  [2/3] Accept      FAIL Google Analytics 4 fired before the choice; Meta Pixel, Google Ads fired; update granted at 3.7s; kept on 1 more page
  [3/3] Reject      FAIL Google Analytics 4 fired before the choice; Google Analytics 4 fired after Reject on 2 of 3 pages; Meta Pixel held back; update denied at 3.7s; kept on 1 more page

Findings - 4 errors, 0 warnings, 0 notices - 3 runs, 3 trackers, policy basic
```

### Strict

Under `strict`, a tracker script loading before the choice or after Reject is an error too. The testbed's `c25` loads GTM at once in advanced mode:

```text
$ rhythm echo consent https://c25.shop.test/ --consent-mode strict $map
...
  [1/3] No choice   FAIL Google Analytics 4 fired before the choice; Google Tag Manager script loaded before the choice
...
Findings - 7 errors, 0 warnings, 0 notices - 3 runs, 2 trackers, policy strict

  error   consent/before-choice   Google Analytics 4 fired before the visitor chose (No choice)
          3 hits (page_view on /, /page-2/, /)
          fix: Set a Consent Mode default (all four signals denied) before the Google tag, or let
          the banner block it until consent
  error   consent/script-before   Google Tag Manager script loaded before the visitor chose (strict policy) (No choice)
          3 loads (googletagmanager.com/gtm.js?id=••••BED1)
          fix: Under strict, load no tracker script before consent: let the CMP block Google Tag
          Manager and the other tags until Accept.
...
```

## Trackers that don't read Consent Mode

Consent Mode only reaches Google's tags. Others need their own signal or need to be blocked by the banner:

| Tracker | Its consent signal | What Echo reports when it fires anyway |
|---|---|---|
| Meta Pixel | `fbq('consent', 'revoke')` before `fbq('init')`, then `fbq('consent', 'grant')` | the error, plus [`consent/meta-no-consent-api`](Echo-Consent-Checks#consentmeta-no-consent-api) when no `fbq('consent', ...)` was seen |
| Microsoft UET (Bing Ads) | `uetq.push('consent', 'default', ...)` | the error |
| Microsoft Clarity | `window.clarity('consentv2', ...)` | the error, with that fix |
| IAB TCF CMPs | the TC string (`__tcfapi`, `euconsent-v2`) | [`consent/tcf-mismatch`](Echo-Consent-Checks#consenttcf-mismatch) when it disagrees with the choice |
| GPP | the GPP string (`__gpp`) | recorded for the timeline |

The testbed's `c02` shows the common case: Google is set up right and the Meta Pixel fires in every run, because nothing told it about consent. See [Reading the result](Echo-Consent#reading-the-result).

## Where the data is

`consent.json` keeps each page's Consent Mode under `pages[].consent_mode` and every signal of every run, with its time, under `pages[].runs[].signals`. `report.html` has a Consent Mode panel per page: the default and update commands and each Google hit's `gcs` and `gcd`, raw and decoded. The Markdown report ends with a short Consent Mode paragraph:

```text
### Consent Mode

No default; Google hits carry gcd only, every signal unset. Accept: update granted at 3.2s. Reject: update denied at 3.2s. gcd decoded with an unofficial mapping.
```

See [Evidence](Echo-Consent-Evidence).

---
<sub>← [Consent checks](Echo-Consent-Checks) · [Home](Home) · [Banners (CMPs)](Echo-Consent-CMPs) →</sub>
