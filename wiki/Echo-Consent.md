**Does Reject really reject?** Echo clicks your cookie banner three ways and shows every tracker that fired before the choice or after Reject, with Consent Mode decoded from the wire. This page covers what `rhythm echo consent` does, a first run, how to read the result, exit codes and the honest limits. A clean result is evidence about what a browser did on your pages, not a legal compliance guarantee.

```sh
rhythm echo consent https://staging.shop.test          # no choice, Accept and Reject
rhythm echo consent detect https://staging.shop.test   # which buttons Echo would click
rhythm echo consent --runs +gpc,+withdraw https://shop.example
```

`consent` is part of Echo 1.2 (`rhythm-echo`). It needs no `.echo/config.yaml` and no running Echo: one command, one URL. You run it as `rhythm echo consent`, `rhythm e consent` or `rhythm-echo consent`; they're the same command.

> [!IMPORTANT]
> Check only sites you own or are authorised to test. A consent check loads the site like one new visitor: it never logs in, never submits a form and never clicks anything outside the banner. Every tracking hit is recorded and stopped in the browser, so checking production adds nothing to anyone's analytics. See [Responsible use](Responsible-Use).

## Why

Sites install a consent banner and assume it works. Nobody clicks Reject and watches the network. Common causes of a banner that doesn't do its job:

- a second plugin or the theme adds its own GA4 snippet the banner never sees;
- the banner's auto-blocking relies on its last scan and misses a new script;
- a setting such as "allow Google tags to fire before consent" is on;
- **Meta Pixel ignores Google Consent Mode**: it needs `fbq('consent', 'revoke')` before `fbq('init')`;
- Reject is mapped to "granted" in a tag manager's consent tag;
- the choice is stored on the wrong path, so the next page fires everything again.

Regulators fine what happens after Reject, not only before consent. US claims argue about the millisecond a tag fired. Echo gives you a timeline with millisecond times, screenshots and hashes, so you can find the cause and show the fix.

## What it does

`rhythm echo consent <url>` opens the page in a real Chrome (the one you have, or Chrome for Testing, downloaded once) as three new visitors. Each one gets a fresh browser profile, then scrolls, visits one more page on the site and reloads:

| Run | The visitor | Should fire |
|---|---|---|
| **No choice** | ignores the banner | nothing that needs consent |
| **Accept** | clicks Accept; Echo checks the banner's own stored state says so | trackers may fire now |
| **Reject** | clicks Reject, on the banner or in its settings; Echo checks the stored state | nothing that needs consent, on this page or the next |

`--runs +gpc` adds a visitor who sends Global Privacy Control and `--runs +withdraw` one who accepts, reopens the banner and rejects. Hiding a banner is never counted as a Reject: a click counts only when the banner's stored state (its cookie or API) says the choice was made.

Echo judges what left the browser, not what the page says about itself. A hit is a request to a tracker's collection endpoint (GA4's `/g/collect`, Meta's `facebook.com/tr`). Writing a tracker cookie or storage key counts like a hit. The `dataLayer`, `gtag` calls and the banner's own state explain the hits; they never clear one.

What sets it apart:

1. **No choice, Accept and Reject** in fresh profiles, each followed by a scroll, another page and a reload; GPC and withdraw runs on request.
2. **Google Consent Mode v2 read from real hits**: `gcs`, `gcd`, the default and update order and the two v2 signals, judged under a named policy (`advanced`, `basic` or `strict`). See [Consent Mode](Echo-Consent-Mode).
3. **47 trackers** beyond Google: Meta, TikTok, Clarity, LinkedIn, Hotjar, Pinterest and more, from an MIT catalogue with fixtures. Add your own in the config.
4. **Hits held in the browser.** Echo answers each tracker hit itself, so nothing reaches Google, Meta or any other tracker. `--send-hits` sends them and is refused on a `production:` host.
5. **Server side too.** With `--echo`, Meta Conversions API and GA4 Measurement Protocol calls the site's server sends during the Reject run are reported.
6. **An evidence folder** with a millisecond timeline, screenshots, masked IDs, the SHA-256 of every file and an optional ed25519 signature. See [Evidence](Echo-Consent-Evidence).
7. **CI first**: exit codes, `--fail-on`, a baseline with BROKE, NEW, CHANGED and FIXED and the GitHub Action. See [Consent in CI](Echo-Consent-CI).

| | Echo consent | Website Evidence Collector (EDPS) | Vendor cookie scanners | Free Consent Mode checkers |
|---|---|---|---|---|
| Clicks Accept and Reject | yes; checks the stored choice too | no: collects evidence of one visit; doesn't test what the banner does | no | no |
| Reads what left the browser | yes | yes | lists cookies | reads the `dataLayer` |
| Consent Mode `gcs` and `gcd` | yes | no | no | partly |
| Holds tracker hits | yes | no | no | no |
| Baseline and CI exit codes | yes | no | no | no |
| Price | free, MIT, local | free | part of a subscription | free |

## Install

Echo 1.2 comes with the Rhythm installer. With `rhythm` already set up:

```sh
rhythm install echo        # or rhythm update echo
```

No `rhythm` yet? `curl -fsSL https://rythm.sh/install | sh` on macOS and Linux, `irm https://rythm.sh/install.ps1 | iex` in Windows PowerShell. With Go: `go install github.com/RhythmShahriar/echo/cmd/rhythm-echo@latest`. See [Install](Install).

Consent checks need Chrome or Chromium. Echo uses an installed Chrome when it finds one; otherwise it downloads Chrome for Testing 156.0.8078.47 once into `~/.cache/rhythm/soundcheck/browser` (the folder Soundcheck uses, so one download serves both). `--browser <path>` picks one, `--no-download` forbids the download. `rhythm echo doctor` says which it will use:

```text
ok    Chrome for consent checks: found (Chrome 153 at /usr/bin/google-chrome)
```

## Your first check

```sh
rhythm echo consent https://staging.shop.test
```

The first time, Echo prints a note on stderr, once per machine. Here is a real first run (the output piped, so the runs read `ok`), against the testbed's `g02` shop (a Complianz banner with Consent Mode advanced; see [Try it on the testbed](#try-it-on-the-testbed)):

```text
$ rhythm echo consent https://g02.shop.test/ $map
Note: use Echo on sites you own or are authorised to test. Consent checks load the site like
one new visitor: they never log in or submit forms and every tracking hit is recorded and
stopped in the browser. Results stay on this machine with IDs masked. This notice is shown once.
rhythm echo 1.2.0 - consent check of https://g02.shop.test/

Browser: Chrome 153.0.8010.52
Hits held: tracking hits are recorded and stopped in the browser. Nothing reached Google, Meta or any other tracker.
Hosts mapped with --map-host; https pages were answered over plain http, so TLS wasn't checked.

https://g02.shop.test/
  Banner: Complianz, found after 0.1s, Reject on the banner
  Consent Mode: v2 default, all four signals denied at 0.03s, before the first Google hit
  [1/3] No choice   ok Google Analytics 4: 3 cookieless pings (gcs G100); no tracker cookies; Meta Pixel held back
  [2/3] Accept      ok Meta Pixel, Google Ads, Google Analytics 4 fired; update granted at 3.8s; kept on 1 more page
  [3/3] Reject      ok Google Analytics 4: 2 cookieless pings (gcs G100); no tracker cookies; Meta Pixel held back; update denied at 3.8s; kept on 1 more page

All clear - 0 errors, 0 warnings, 3 notices - 3 runs, 3 trackers, policy advanced

  notice  consent/cookieless-ping  Google Analytics 4 sent 3 cookieless pings before the choice (allowed under --consent-mode advanced) (No choice)
  notice  consent/cookieless-ping  Google Analytics 4 sent 1 cookieless ping before the choice (allowed under --consent-mode advanced) (Accept)
  notice  consent/cookieless-ping  Google Analytics 4 sent 3 cookieless pings before the choice and after Reject (allowed under --consent-mode advanced) (Reject)

Evidence, not legal advice.
full report: .echo/consent/2026-10-28T21-04-45Z-c1460c/report.html
Tip: --save baseline keeps this as the baseline for the next check.
```

The `Hosts mapped` line appears because the testbed answers every host through `--map-host`; on your own site over https it's not there. The `Browser` line names the Chrome every check used. A build from source prints `rhythm echo dev` where a release prints its version.

A check takes about 15 seconds per page (25 at the 95th percentile on the testbed). The result is kept in `.echo/consent/<date-time>-<id>/` (the time in UTC, like every time in the reports), next to `.echo/config.yaml` when there is one, else in `./.echo`. Open `report.html` for the timeline and the screenshots, or pass `--open`.

## Reading the result

The header says what was checked and how. `Hits held` means nothing reached a tracker. Then one block per page:

- **Banner:** which banner Echo found (by its own rule, or `Generic banner` for one it found by its buttons' words), when it showed and whether Reject is on the banner or in its settings.
- **Consent Mode:** the default Google tags got, its signals and when, relative to the first Google hit. `no default` when there was none.
- **One line per run**, `ok`, `FAIL` or `INCONCLUSIVE` (in a terminal: nothing, a red `✗` or a `?`), with what fired, the Consent Mode update and whether the choice was kept on the next page.

The verdict line reads `All clear`, `Findings` or `Couldn't check`, with the counts, the number of runs and trackers and the policy. Each issue follows with its check ID, title, run, the evidence (a masked URL and its time, or a count of hits on which pages, plus the cookies written) and a one-line fix.

A broken site, the testbed's `c02` with a Meta Pixel that ignores Consent Mode:

```text
https://c02.shop.test/
  Banner: Complianz, found after 0.1s, Reject on the banner
  Consent Mode: v2 default, all four signals denied at 0.02s
  [1/3] No choice   FAIL Meta Pixel fired before the choice
  [2/3] Accept      FAIL Meta Pixel fired before the choice; Google Ads, Google Analytics 4 fired; update granted at 3.2s; kept on 1 more page
  [3/3] Reject      FAIL Meta Pixel fired before the choice; Meta Pixel fired after Reject on 2 of 3 pages; update denied at 3.2s; kept on 1 more page

Findings - 4 errors, 0 warnings, 1 notice - 3 runs, 3 trackers, policy advanced

  error   consent/before-choice   Meta Pixel fired before the visitor chose (No choice)
          3 hits (PageView on /, /page-2/, /); cookie _fbp written
          fix: Meta Pixel doesn't read Google Consent Mode. Call fbq('consent', 'revoke') before
          fbq('init'), then fbq('consent', 'grant') when the banner reports marketing consent, or
          let the banner block the pixel script.
  error   consent/before-choice   Meta Pixel fired before the visitor chose (Accept)
          facebook.com/tr?id=••••0001&ev=PageView at 0.07s; cookie _fbp written
          fix: as above.
  error   consent/before-choice   Meta Pixel fired before the visitor chose (Reject)
          facebook.com/tr?id=••••0001&ev=PageView at 0.07s; cookie _fbp written
          fix: as above.
  error   consent/after-reject    Meta Pixel fired after Reject (Reject)
          2 hits (PageView on /page-2/, /)
          fix: as above.
  notice  consent/meta-no-consent-api  Meta Pixel never received fbq('consent', ...) (No choice)

Evidence, not legal advice.
full report: .echo/consent/2026-10-29T02-07-50-66afec/report.html
```

Issues are per run: a tracker that fires before the choice in all three runs gives three `consent/before-choice` lines, one per run. IDs are masked to their last four characters (`••••0001`); `--show-ids` keeps them whole for your own debugging. Every check is explained, with an example and the fix, in [Consent checks](Echo-Consent-Checks).

`-v` adds every run's timeline and each issue's key; `-q` prints only the verdict, the issues and the runs that couldn't choose; `--live` prints each tracker hit on stderr as it happens:

```text
$ rhythm echo consent https://c02.shop.test/ --live -q --pages 0 $map
hit  0.07s  Accept, before the choice  Meta Pixel  www.facebook.com/tr (held)
hit  0.08s  No choice  Meta Pixel  www.facebook.com/tr (held)
hit  0.08s  Reject, before the choice  Meta Pixel  www.facebook.com/tr (held)
hit  4.11s  No choice  Meta Pixel  www.facebook.com/tr (held)
hit  3.23s  Accept  Google Ads  googleads.g.doubleclick.net/pagead/viewthroughconversion/••••0001/ (held)
hit  3.23s  Accept  Google Analytics 4  www.google-analytics.com/g/collect (held)
...
```

A part of the `-v` timeline of an Accept run:

```text
  [2/3] Accept      ok Google Ads, Google Analytics 4 fired; update granted at 3.2s; kept on 1 more page
     0.00s  load   https://c01.shop.test/
     0.03s  script googletagmanager.com/gtag/js?id=••••D001
     0.04s  gtag   consent default  all four denied
     0.08s  banner Complianz shown
     3.18s  click  "Accept" (.cmplz-accept)
     3.18s  gtag   consent update   all four granted
     3.20s  hit    Google Ads gcs=G111 gcd=13r3r3r3r5 (held)
     3.20s  hit    Google Analytics 4 page_view gcs=G111 gcd=13r3r3r3r5 (held)
     6.21s  cookie _gcl_au (Google Ads)
     6.21s  cookie _ga (Google Analytics 4)
     6.21s  cookie _ga_••••D001 (Google Analytics 4)
     6.75s  page   /page-2/ (link 1 of 1)
...
```

### Couldn't check

A run whose choice couldn't be made or confirmed is **inconclusive**, never a pass. Echo tries it again once in a fresh profile (`--retries`), then reports it with a reason and what to try:

```text
https://c11.shop.test/
  Banner: Usercentrics, found after 0.1s
  Consent Mode: v2 default, all four signals denied at 0.03s
  [1/3] No choice   ok nothing fired
  [2/3] Accept      INCONCLUSIVE couldn't choose: no button found
  [3/3] Reject      INCONCLUSIVE couldn't choose: no button found

Couldn't check - 0 errors, 0 warnings, 0 notices - 3 runs, 0 trackers, policy advanced

  INCONCLUSIVE /  Accept: Usercentrics's banner showed but Echo found no Accept on it or in its settings. Set --accept-selector (rhythm echo consent detect shows what Echo sees), or --click button,api to use Usercentrics's API
  INCONCLUSIVE /  Reject: Usercentrics's banner showed but Echo found no Reject on it or in its settings. Set --reject-selector (rhythm echo consent detect shows what Echo sees), or --click button,api to use Usercentrics's API
```

That is c11 in its default state; `-q` prints the verdict line and the same two `INCONCLUSIVE` lines, so a CI log says why. That check exits `3`. [Banners (CMPs)](Echo-Consent-CMPs) covers `detect`, the selector flags and `--click button,api`.

A page with no banner and no trackers needs no consent and passes:

```text
https://g12.shop.test/
  No banner and no trackers: nothing to consent to
  [1/3] No choice   ok nothing fired
  [2/3] Accept      ok nothing fired
  [3/3] Reject      ok nothing fired

All clear - 0 errors, 0 warnings, 0 notices - 3 runs, 0 trackers, policy advanced
```

## Exit codes

| Code | Means |
|---|---|
| `0` | Every run made and confirmed its choice (or the page needed none) and no issue is at or above `--fail-on` (default `error`) |
| `1` | An issue at or above `--fail-on`; with `--against`, a change at or above it (default `broke`). Wins over `3` |
| `2` | Usage error, before any browser starts: a wrong flag, a bad URL (a bare word such as `not-a-url` too: give `https://` for a host without a dot), an unknown `consent:` key or check ID, a literal header value in the config, a missing baseline, `--send-hits` on a `production:` host, `--show-ids` with Markdown or JUnit, an invalid selector |
| `3` | Couldn't check: no Chrome, the site didn't answer, a TLS error without `-k`, a bot wall, a redirect loop, a run that couldn't make its choice and nothing at or above `--fail-on`, a report that couldn't be written |
| `130` | Interrupted: finished runs are kept and `consent.json` says `complete: false` |

With `--json`, a check that stops with `2` or `3` before it has a result still writes the `rhythm-echo/v1` envelope to stdout: `kind: consent`, `verdict: error`, `complete: false`, `data: null` and an `error` with the exit code and the message stderr shows. Scripts can read stdout whatever happened.

`consent detect` exits `0` when it found a banner and both buttons, `1` for a banner with a button missing, `3` for no banner. `consent verify` exits `1` when a hash or the signature doesn't match and `3` when a file is missing. All of them are in the [CLI reference](Echo-CLI-Reference#consent).

## More than one page or site

```sh
rhythm echo consent https://shop.example/ https://shop.example/shop/   # pages of one result
rhythm echo consent --sites sites.yaml                                 # every site in a Soundcheck-style file
```

List your usual pages under `consent: urls:` in `.echo/config.yaml` and `rhythm echo consent` alone checks them (see [Configuration](Echo-Configuration#consent)). `--sites` reads the `sites.yaml` Soundcheck fleet uses; [Consent in CI](Echo-Consent-CI#many-sites) shows a real run. `--parallel` (default `3`, at most `8`) sets how many browser runs go at once.

## Compare with a baseline

```sh
rhythm echo consent https://staging.shop.test --save baseline        # once, on a good state
rhythm echo consent https://staging.shop.test --against baseline     # after a change
```

With `--against`, the verdict is the change: **BROKE** (a new error, such as a tracker now firing before the choice), **NEW** (a new tracker, cookie, CMP, warning or notice), **CHANGED** (the consent signals differ) and **FIXED**. A real run after a plugin update added TikTok to the testbed's `c08`:

```text
rhythm echo 1.2.0 - consent check of https://c08.shop.test/ against baseline from 2026-10-28 20:10 UTC
...
Broke - 4 broke, 0 new, 0 changed, 0 fixed - 3 runs, 4 trackers
  BROKE   TikTok Pixel now fires before the choice            / (No choice, Accept, Reject)
  BROKE   TikTok Pixel now fires after Reject                 / (Reject)
```

The counts in the verdict line are per page, run and tracker (one per change key), while the lines below group the keys of one change: here four keys, two lines. `-v` prints each line's keys. See [Consent in CI](Echo-Consent-CI#baselines) for `--against last`, `--accept` and expiring ignores.

## Try it on the testbed

The Echo repository has 40 small shops for the consent checks: 25 broken in one way each, 15 correct. They run on one local Go server, with stand-ins for every tracker and CMP host, so nothing reaches the internet. You need Go and Chrome:

```sh
git clone https://github.com/RhythmShahriar/echo && cd echo
make build                                    # bin/rhythm-echo
make -C testbed/consent serve                 # leave it running; 127.0.0.1:23400 and :23401
```

In another terminal:

```sh
map=$(make -s -C testbed/consent map)         # the --map-host flags for every stand-in host
eval "bin/rhythm-echo consent https://c01.shop.test/ $map"     # all clear
testbed/consent/toggle.sh c01 broken
eval "bin/rhythm-echo consent https://c01.shop.test/ $map"     # GA4 before the choice, no default
testbed/consent/toggle.sh all good
```

**The map line.** `make map` prints `--map-host '*.shop.test=127.0.0.1:23400' --map-host analytics.tiktok.com=127.0.0.1:23400 ... --map-only`. Each `--map-host` answers a host from the testbed instead of the network; `--map-only` refuses every host the map doesn't cover, so nothing a page asks for leaves the machine. `eval` is needed because the line quotes `'*.shop.test=...'`. Without it the quotes reach Echo and it stops with `--map-host '*.shop.test=...' kept its quotes; use eval, as the testbed section shows`.

**Other ports.** When 23400 or 23401 is taken, move both and give `map` the same `LISTEN`, or the map points at the wrong port:

```sh
make -C testbed/consent serve LISTEN=127.0.0.1:39130 ADMIN=127.0.0.1:39131
map=$(make -s -C testbed/consent map LISTEN=127.0.0.1:39130)
```

**A clean checkout.** The testbed writes only what git ignores: the testbed CA in `testbed/consent/ca/` (on the first `make serve`) and the scenario switches in `testbed/consent/state/`. `testbed/consent/toggle.sh all good` puts every scenario back. To leave the checkout untouched, copy `testbed/consent` elsewhere and start the server on the copy: `go run ./testbed/consent/server/cmd/consent-testbed -root /tmp/consent -tls`; the copy's own `toggle.sh` switches its scenarios. Results go to `.echo/consent/` in the folder you run the check from.

**dev, not 1.2.0.** A build from source calls itself `dev`: the header reads `rhythm echo dev` and the User-Agent ends in `rhythm-echo-consent/dev`. It has every consent check of the release it was built from; release builds print their version.

The outputs on these pages come from that testbed, which is why they read `Hosts mapped with --map-host`. The scenario list is in the [testbed README](https://github.com/RhythmShahriar/echo/blob/master/testbed/consent/README.md).

## Honest limits

The same text is at the end of `rhythm echo consent --help` and in the README.

> **What a pass means.** In these runs, from this machine, with this browser and these settings, no tracker on Echo's list sent a request or set a cookie before the visitor chose or after Reject. It does not mean the site complies with the GDPR, ePrivacy, CCPA/CPRA or any other law. Evidence, not legal advice.
>
> **One visitor, one place.** Banners differ by country, language and device. Geo-targeted banners may not show to a CI runner in the US; Echo can set the browser's language and time zone (`--geo`) but not its IP address. Run it from the region that matters, or through a proxy there.
>
> **Blind spots.** Trackers that are not on the list; server-side tagging on your own domain when its requests don't look like GA4 or Meta; fingerprinting without network requests; tags that wait longer than the settle time or need more interaction than a scroll and one more page; sites that hide tags from automated browsers.
>
> **Consent Mode advanced.** Google says its tags in advanced mode "send measurements without cookies" while consent is denied. Whether that is acceptable before consent is disputed. Echo reports these pings as `consent/cookieless-ping` under the default policy `advanced`; `--consent-mode basic` or `strict` counts them as hits.
>
> **gcd is unofficial.** Google doesn't document the `gcd` parameter. Echo shows the raw value next to a reading based on community documentation and treats letters it doesn't know as unknown.
>
> **Pseudonymised, not anonymised.** Masked or hashed identifiers in reports can still be personal data. Treat reports like logs.
>
> **What leaves the machine.** Requests to the site and to the scripts it loads (which shows this machine's IP address to those servers). No tracking hits unless `--send-hits`. Nothing goes to Rhythm. The update check contacts GitHub (`RHYTHM_OFFLINE=1` turns it off).
>
> **Banners Echo can't handle** are reported as "couldn't check" (exit 3), never as a pass. Tested CMPs and the dates their fixtures were captured are in the benchmark.
>
> **A signature** proves who made the evidence file and that it hasn't changed. It doesn't prove the site behaves the same for every visitor, region or day. Keyless signing in the Action writes the repository and workflow name to the public Rekor log.

## FAQ

**Does a clean result mean my site is compliant?** No. It means that in these runs no tracker on Echo's list fired before the choice or after Reject. Whether your banner's wording, categories and records meet the law is a question for your legal adviser. Echo gives them evidence to look at.

**Will checking production pollute our analytics?** No. Tracker hits are answered inside the browser and never sent (`Hits held` in every output). The page and its scripts are loaded for real, so your web server logs see each run load the page, one more page and the page again, with a User-Agent ending in `rhythm-echo-consent/1.2.0`.

**Why does the banner not show in CI?** Many banners show only to visitors in some countries. `--geo de` sets the browser's language, `Accept-Language` and time zone (and a CMP's documented geolocation override when its rule has one) but never the IP address. Run from a runner in that region, or pass `--proxy` with a proxy there.

**Echo says "couldn't check". Is the site broken?** Not necessarily. Echo couldn't make or confirm the choice: the banner didn't show in time (`--banner-wait`), Echo found no button (`--accept-selector`, `--reject-selector`, `--click button,api`) or the click didn't change the stored state. Start with `rhythm echo consent detect <url>`. See [Banners (CMPs)](Echo-Consent-CMPs).

**Why are GA4 cookieless pings only a notice?** Consent Mode advanced sends them on purpose and whether that's acceptable before consent is disputed. The default policy reports them without failing; `--consent-mode basic` makes them errors. See [Consent Mode](Echo-Consent-Mode#policies).

**Can it check a site behind a login or basic auth?** It never logs in. For a staging site behind basic auth, pass the header from the environment: `-H "Authorization: env:RHYTHM_ECHO_STAGING_AUTH"` (a plain value works too, with a warning that it shows in `ps`).

**Does it catch server-side tracking?** Partly. A first-party server-side GTM whose requests look like GA4 or Meta is recognized (`first_party: true`). Events the server sends on its own (Meta Conversions API, GA4 Measurement Protocol) are invisible to a browser; run the site with Echo's connector and pass `--echo` to have Echo report those sent during the Reject run.

**Are the consent results committed to git?** Echo 1.2.0 adds `.echo/snapshots/` and `.echo/runs/` to `.git/info/exclude` but not `.echo/consent/`. Screenshots of the site and masked URLs are in there: add `/.echo/consent/` to `.gitignore` unless you mean to commit the evidence.

More problems and fixes are in [FAQ and troubleshooting](Echo-FAQ#cookie-consent-checks).

## Benchmark

On the consent testbed (Go mode, Chrome 153, 16 CPUs, 2026-10-28):

| Measure | Result | Target |
|---|---|---|
| Broken scenarios caught | 25 of 25 | 25 of 25 |
| False alarms on correct sites | 0 in 400 checks | 0 |
| Flakes over 10 rounds | 0 | 0 |
| Accept and Reject clicks verified | 880 of 880 | 100% |
| Tracker hits that reached the sink | 0 | 0 |
| Time per page | 15.0 s p50, 25.6 s p95 | under 25 s and 45 s |
| `consent detect`, slowest | 4.5 s | under 15 s |
| 50 sites, `--parallel 6` | 5.6 min | under 15 min |
| Echo's memory (Chrome excluded) | 116.5 MiB | under 200 MB |

The commercial CMPs are replicas of public markup captured on 2026-10-26, Google's and Meta's tags are stand-ins written to community-documented behaviour and we wrote both Echo and the testbed: read it as a regression suite with a scoreboard. The full tables are in [Benchmark](Echo-Benchmark#consent-checks) and [`docs/benchmark-consent.md`](https://github.com/RhythmShahriar/echo/blob/master/docs/benchmark-consent.md).

## Where to go next

| You want to… | Read |
|---|---|
| Know what each check means and how to fix it | [Consent checks](Echo-Consent-Checks) |
| Understand `gcs`, `gcd` and the three policies | [Consent Mode](Echo-Consent-Mode) |
| Get Echo to click a banner, or see which banners it knows | [Banners (CMPs)](Echo-Consent-CMPs) |
| Run it in CI, on many sites or against a baseline | [Consent in CI](Echo-Consent-CI) |
| Keep, sign and verify the evidence | [Evidence](Echo-Consent-Evidence) |
| Set it up in `.echo/config.yaml` | [Configuration](Echo-Configuration#consent) |
| Look up a flag | [CLI reference](Echo-CLI-Reference#consent) |

---
<sub>← [Deliverability](Echo-Deliverability) · [Home](Home) · [Consent checks](Echo-Consent-Checks) →</sub>
