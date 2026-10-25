Answers to common questions and fixes for common problems with Soundcheck. If yours isn't here, check the [CLI reference](Soundcheck-CLI-Reference) or open an issue (see [the end of this page](#reporting-a-bug)).

## The browser

### Chromium download and update

Soundcheck needs Chrome or Chromium. It looks for an installed Chrome or Chromium in the usual places first. When there's none, it downloads **Chrome for Testing** (Google's build of Chrome for automation) once, the first time it needs a browser, into your cache folder (`~/.cache/rhythm/soundcheck/browser` on Linux; see [CLI reference](Soundcheck-CLI-Reference#files-and-locations)). The version and each platform's SHA-256 are pinned in the binary (Chrome for Testing 156.0.8078.47 in Soundcheck 1.2.0). The archive is checked before anything is unpacked. It says so on stderr with `Downloading Chrome for Testing <version> (<n> MB) once, into <folder> …`. Download progress also goes to stderr, never stdout, so `--json` on stdout stays valid. Every run names the browser it uses:

```text
Browser: Chrome 153.0.8010.52 (/usr/bin/google-chrome)
```

The download is reused by every later run. `rhythm soundcheck update` does the same ahead of time (useful in a Docker image build). It also removes downloaded versions this release no longer uses. When an installed Chrome is used, it removes any downloaded Chrome for Testing too:

```text
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
```

| You want to… | Do |
|---|---|
| use a specific browser | `--browser /path/to/chrome` |
| check in Firefox or WebKit | `--browser firefox` or `--browser webkit`; see [Firefox and WebKit](Soundcheck-Browsers) |
| never download (offline, locked-down CI) | `--no-download`; without a browser the run stops with `no Chrome or Chromium found; install Chrome, pass --browser /path/to/chrome, or allow the one-time Chrome for Testing download` |
| free the disk space | `rhythm soundcheck update --clear` |
| see what the browser does | `--show-browser` opens a visible window (needs a display) |

A download that stalls gives up after a minute without data (`downloading Chrome for Testing: no data from <url> for 1m0s; giving up`) and installs nothing. One whose checksum doesn't match installs nothing either. Ctrl+C stops it. `rhythm update` runs `rhythm-soundcheck update` for you after updating the binary.

### Docker, root and the Chrome sandbox

Chrome can't use its sandbox as root. Soundcheck doesn't turn the sandbox off by itself: as root (most containers and some CI runners) it stops with

```text
running as root, where Chrome can't use its sandbox. Run Soundcheck as a normal user, or pass --no-sandbox (or set SOUNDCHECK_NO_SANDBOX=1) to run Chrome without it (only for sites you trust, e.g. in a throwaway container)
```

Run it as a normal user where you can. In a throwaway container checking your own site, pass `--no-sandbox` or set `SOUNDCHECK_NO_SANDBOX=1`; every such run warns that a page exploiting a browser bug could take over the machine. A container whose seccomp or AppArmor profile keeps a normal user's Chrome from its sandbox fails with Chrome's `No usable sandbox!` and the hint `(pass --no-sandbox, or set SOUNDCHECK_NO_SANDBOX=1, if this container can't give Chrome its sandbox; only for sites you trust)`.

In a slim image, the downloaded Chrome for Testing needs the libraries Chrome depends on (`libnss3`, `libatk-bridge2.0-0`, `libgbm1`, `libasound2` and friends). The simplest fix is an image that already has Chrome or Chromium, then `--browser` and `--no-download`:

```sh
rhythm soundcheck run staging.yoursite.com --browser /usr/bin/chromium --no-download
```

Containers may share one `.soundcheck` folder (a mounted volume) as long as each has its own hostname, which is Docker's default.

If Chrome crashes with "no space left on device" in Docker, the container's `/dev/shm` is too small: run it with `--shm-size=1g`.

### CI without Chrome

GitHub's `ubuntu-latest` runners have Chrome. Elsewhere, either use an image with Chrome or Chromium, or let Soundcheck download Chrome for Testing on the first run and cache its folder (`~/.cache/rhythm/soundcheck/browser`) between jobs. See [CI](Soundcheck-CI#other-ci-systems).

## Checking a site

### "can't reach" a site

```text
rhythm soundcheck: can't reach staging.yoursite.com: Get "http://staging.yoursite.com": dial tcp: lookup staging.yoursite.com: no such host
```

Soundcheck couldn't load the start page. Check the address and that the site answers from the machine running Soundcheck (`curl -I https://staging.yoursite.com`). Sites behind a VPN, an IP allowlist or a firewall need that machine allowed. For a bare hostname, Soundcheck tries `https://` and then `http://`; the error shows the last attempt.

A self-signed or otherwise invalid certificate gives `can't reach staging.yoursite.com: invalid TLS certificate (rerun with --insecure to check it anyway)`. Add `-k` on local and staging sites.

A bare `host:port` tries https first. A server that only speaks http and answers the https try in plain http gets `https refused; using http`. PHP's built-in server (`php -S`) closes the connection instead (`can't reach 127.0.0.1:18931: Get "https://127.0.0.1:18931": EOF`), so give it `http://` yourself.

A start page that redirects to another site stops the run with `... redirects to ..., which is another site; check that address instead if it's the one you meant`. Pass `--follow-host` to check the site it leads to.

### Staging behind a password

For HTTP basic auth, send the header (the browser sends it too):

```sh
rhythm soundcheck snapshot staging.yoursite.com \
  -H "Authorization: Basic $(printf 'user:pass' | base64)"
```

For a staging cookie, use `--cookie "staging_auth=…"`. Pass the same header to `compare`. Both go to the checked site only (and its www. or bare host). To keep the secret out of your shell history, put it in the config as `headers: {Authorization: SOUNDCHECK_STAGING_AUTH}` and export `SOUNDCHECK_STAGING_AUTH`.

When the site came from the config's `site:` or a committed snapshot, Soundcheck refuses to send `-H` and `--cookie` to it unless it's this machine: `not sending -H to <host>: the site came from .soundcheck/config.yaml (site); name it on the command line to send them`.

### Logging in

Set a test account in `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD`. Soundcheck uses it for the cache leak test and for flows with `login: true`; pages are always checked as an anonymous visitor. Soundcheck tries the login once, at the start. When it fails, one note says why. The leak test and login flows are then skipped:

```text
note: login failed, so the checks that need it were skipped: WordPress refused the login: Error: The password you entered for the username is incorrect.
```

Common causes:

- **A security plugin or a captcha on the login form.** Allow the test account, or log in with a [login flow](Soundcheck-Forms-And-Flows#logging-in) that handles the extra step.
- **A custom login URL** (WPS Hide Login and similar). Set `login.url: /your-login/`.
- **Two-factor authentication.** Turn it off for the test account on staging.
- **"logged in, yet the site doesn't show the admin bar…"** The login worked. The front end still serves anonymous pages to logged-in users, usually a page cache. That's worth fixing too.

### Slow or huge sites

The time a run takes grows with pages × viewports × what each module does; clicking and the cache double fetch add the most. With every module on, a 37-page local test site took 54 s for the snapshot and 47 s for the compare. A 101-page static site on a 16-core machine took 92 s and 61 s. Clicking is capped at 45 s per page. The pace on the server is set by `--rate` (5 page loads a second), whatever the number of tabs.

| To… | Use |
|---|---|
| check fewer pages | `--limit 100`, or `--include` for the section you changed |
| check one sample per template | `per_group: 1` in the config |
| check more pages at once | `--tabs 12` (default 8); this helps when pages are slow, the rate still applies |
| skip what you don't need | `--skip speed,cache`, `--viewport desktop`, `--level inspect` |
| be gentler on a fragile host | `--rate 1` or `--rate 0.5`, `--tabs 2` |
| allow slow pages | `--timeout 60s` |

Grouping keeps large sites manageable without a limit: a shop with 3,000 products is checked through about 13 product pages, ten on their own and three samples of the rest. See [Discovery](Soundcheck-Discovery#grouping-by-url-pattern).

### "found no pages to check"

The start page loaded. It was filtered out too, so there was nothing to check. The start page is always checked unless it matches an exclude (yours or the [built-in ones](Soundcheck-Discovery#urls-soundcheck-never-visits)) or `robots.txt` disallows it. On your own staging site, a `robots.txt` that disallows everything is common: add `--ignore-robots`.

### Why wasn't a page checked?

The discovery line says why pages were left out (`excluded`, `grouped`, `robots.txt`, `page limit`…). A page that nothing links to and that isn't in a sitemap can't be found: add it with `--page` or `pages.extra`. `compare` never discovers new pages; take a new snapshot to include pages added since.

## Understanding results

### Noisy visual diffs

When visual changes keep appearing that aren't real changes:

1. **Open the HTML report** and look at the Diff view: it shows what actually changed.
2. **A widget that changes on every load** (live chat, ads, a rotating banner, "X people viewing"): add its selector to `visual.ignore`.
3. **Small changes on many pages** (a date, a counter, a font hinting difference): raise `visual.threshold`, for example to `2`.
4. **Content that changes because time passed** (new blog posts on the home page, "posted 3 days ago"): expected; accept it with `rhythm soundcheck accept <id>` after you've reviewed it, or ignore it until a date. See [Review and accept](Soundcheck-Review-And-Accept).
5. **Different machines** render fonts differently: take the snapshot and the compare on the same machine or the same CI image.

Snapshots learn regions that move on their own. They only learn what moves within about a second. See [Visual changes](Soundcheck-Visual#when-visual-results-are-noisy).

### Everything shows as NEW

The comparison probably ran with other settings than the snapshot. `compare` uses the snapshot's modules, levels, viewports and browser unless a flag or the config file sets them. It says so: look for `You changed:` in the header. A level that only ran this time (say `submit`) reports what it finds as NEW. Add `--strict` to stop instead of comparing with other settings.

### A change keeps showing as UNSTABLE

The page gave the problem once and not on the second visit: a flaky server, a timeout under load, a script that failed once. UNSTABLE never fails `--fail-on`. If it shows on every compare, the problem is real but intermittent; look at the server's error log for that URL. See [The confirm pass](Soundcheck-Review-And-Accept#the-confirm-pass-and-unstable).

### A slowdown I can't reproduce

Lab numbers come from one load per page. Run `compare` again; a real slowdown shows again, noise doesn't. Make sure both runs ran on the same machine under similar load. See [Speed](Soundcheck-Speed#getting-stable-numbers).

### Why is a form "skipped"?

Forms that are hidden, have a password field, are search forms, comment forms, the cart form, or post to `/wp-admin` are never submitted; on production URLs none are, unless allowlisted in `interactions.forms`. The reason is in the run's JSON under `interactions`. See [Forms and flows](Soundcheck-Forms-And-Flows#forms-that-are-never-submitted).

### Where are my test submissions?

Every value Soundcheck types contains `soundcheck-test`. Search your inbox, CRM, form plugin entries or database for it to find and delete them. With [Echo](Echo) sealing staging, the emails and webhooks those submits trigger never leave: they're in Echo and tied to each form. `--echo` compares them between runs. See [With Soundcheck](Echo-With-Soundcheck).

## Reporting a bug

Open an issue at [github.com/RhythmShahriar/soundcheck/issues](https://github.com/RhythmShahriar/soundcheck/issues). Include:

- the output of `rhythm soundcheck --version`, your OS and the Chrome or Chromium version;
- the command you ran (with your real domain replaced by `yoursite.com`);
- the change's check ID (from the HTML report or JSON) and why you believe it's wrong;
- for visual issues, the before, after and diff images if you can share them.

> [!WARNING]
> Never paste credentials, cookies, private URLs or client data into a public issue. Screenshots of a site behind a login can contain personal data; crop or blur them first.

### Found a security issue in Soundcheck itself?

Don't open a public issue. Report it privately as described in the [Security policy](Security-Policy).

---
<sub>← [CLI reference](Soundcheck-CLI-Reference) · [Home](Home) · [Benchmark](Soundcheck-Benchmark) →</sub>
