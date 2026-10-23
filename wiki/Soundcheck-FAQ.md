Answers to common questions and fixes for common problems with Soundcheck. If yours isn't here, check the [CLI reference](Soundcheck-CLI-Reference) or open an issue (see [the end of this page](#reporting-a-bug)).

## The browser

### Chromium download and update

Soundcheck needs Chrome or Chromium. It looks for an installed Chrome or Chromium in the usual places first. When there's none, it downloads a pinned Chromium build once, the first time it needs a browser, into your cache folder (`~/.cache/rhythm/soundcheck/browser` on Linux; see [CLI reference](Soundcheck-CLI-Reference#files-and-locations)), and says so with `Downloading Chromium once, into <folder> …`.

The download is reused by every later run. `rhythm soundcheck update` does the same ahead of time (useful in a Docker image build), and after a Soundcheck update it removes the older Chromium revisions the new version no longer uses. With Chrome installed it only reports what it found:

```text
Browser: installed Chrome at /usr/bin/google-chrome (updated with your system)
```

| You want to… | Do |
|---|---|
| use a specific browser | `--browser /path/to/chrome` |
| never download (offline, locked-down CI) | `--no-download`; without a browser the run stops with `no Chrome or Chromium found; install Chrome, pass --browser /path/to/chrome, or allow the one-time Chromium download` |
| free the disk space | `rhythm soundcheck update --clear` |
| see what the browser does | `--show-browser` opens a visible window (needs a display) |

`rhythm update` runs `rhythm-soundcheck update` for you after updating the binary.

### Docker, root and the Chrome sandbox

Chrome refuses to start its sandbox as root. When Soundcheck runs as root (most containers and some CI runners), it starts Chrome with `--no-sandbox` automatically. As a normal user the sandbox stays on.

In a slim image, the downloaded Chromium needs the libraries Chrome depends on (`libnss3`, `libatk-bridge2.0-0`, `libgbm1`, `libasound2` and friends). The simplest fix is an image that already has Chrome or Chromium, then `--browser` and `--no-download`:

```sh
rhythm soundcheck run staging.yoursite.com --browser /usr/bin/chromium --no-download
```

If Chrome crashes with "no space left on device" in Docker, the container's `/dev/shm` is too small: run it with `--shm-size=1g`.

### CI without Chrome

GitHub's `ubuntu-latest` runners have Chrome. Elsewhere, either use an image with Chrome or Chromium, or let Soundcheck download Chromium on the first run and cache its folder (`~/.cache/rhythm/soundcheck/browser`) between jobs. See [CI](Soundcheck-CI#other-ci-systems).

## Checking a site

### "can't reach" a site

```text
rhythm soundcheck: can't reach staging.yoursite.com: Get "http://staging.yoursite.com": dial tcp: lookup staging.yoursite.com: no such host
```

Soundcheck couldn't load the start page. Check the address, and that the site answers from the machine running Soundcheck (`curl -I https://staging.yoursite.com`). Sites behind a VPN, an IP allowlist or a firewall need that machine allowed. For a bare hostname, Soundcheck tries `https://` and then `http://`; the error shows the last attempt.

A self-signed or otherwise invalid certificate gives `can't reach staging.yoursite.com: invalid TLS certificate (rerun with --insecure to check it anyway)`. Add `-k` on local and staging sites.

### Staging behind a password

For HTTP basic auth, send the header (the browser sends it too):

```sh
rhythm soundcheck snapshot staging.yoursite.com \
  -H "Authorization: Basic $(printf 'user:pass' | base64)"
```

For a staging cookie, use `--cookie "staging_auth=…"`. Pass the same header to `compare`.

### Logging in

Set a test account in `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD`. Soundcheck uses it for the cache leak test and for flows with `login: true`; pages are always checked as an anonymous visitor. When the login fails, a note says why, for example `cache: leak test: couldn't log in: WordPress refused the login: …`. Common causes:

- **A security plugin or a captcha on the login form.** Allow the test account, or log in with a [login flow](Soundcheck-Forms-And-Flows#logging-in) that handles the extra step.
- **A custom login URL** (WPS Hide Login and similar). Set `login.url: /your-login/`.
- **Two-factor authentication.** Turn it off for the test account on staging.
- **"logged in, but the site doesn't show the admin bar…"** The login worked, but the front end serves anonymous pages to logged-in users, usually a page cache. That's worth fixing too.

### Slow or huge sites

The time a run takes grows with pages × viewports × what each module does. Clicking and the cache double fetch add the most. On a local test site, 37 pages with every module took about three minutes for the snapshot; the same pages with only health and visual took 45 seconds.

| To… | Use |
|---|---|
| check fewer pages | `--limit 100`, or `--include` for the section you changed |
| check one sample per template | `per_group: 1` in the config |
| check more pages at once | `--tabs 8` (if the server can take it) |
| skip what you don't need | `--skip speed,cache`, `--viewport desktop`, `--level inspect` |
| be gentler on a fragile host | `--rate 1` or `--rate 0.5`, `--tabs 2` |
| allow slow pages | `--timeout 60s` |

Grouping keeps large sites manageable without a limit: a shop with thousands of products is checked through ten product pages plus three samples of the rest. See [Discovery](Soundcheck-Discovery#grouping-by-url-pattern).

### "found no pages to check"

The start page loaded, but every page was filtered out. Usually `--include` doesn't match the start page: start at a page inside the section, such as `rhythm soundcheck run yoursite.com/shop/ --include '/shop/*'`. See [Include and exclude](Soundcheck-Discovery#include-and-exclude). It also happens when `robots.txt` disallows everything; on your own staging site, add `--ignore-robots`.

### Why wasn't a page checked?

The discovery line says why pages were left out (`excluded`, `grouped`, `robots.txt`, `page limit`…). A page nothing links to, and that isn't in a sitemap, can't be found: add it with `--page` or `pages.extra`. `compare` never discovers new pages; take a new snapshot to include pages added since.

## Understanding results

### Noisy visual diffs

When visual changes keep appearing that aren't real changes:

1. **Open the HTML report** and look at the Diff view: it shows what actually changed.
2. **A widget that changes on every load** (live chat, ads, a rotating banner, "X people viewing"): add its selector to `visual.ignore`.
3. **Small changes on many pages** (a date, a counter, a font hinting difference): raise `visual.threshold`, for example to `2`.
4. **Content that changes because time passed** (new blog posts on the home page, "posted 3 days ago"): expected; take a new snapshot after you've reviewed it.
5. **Different machines** render fonts differently: take the snapshot and the compare on the same machine or the same CI image.

Snapshots learn regions that move on their own, but only what moves within about a second. See [Visual changes](Soundcheck-Visual#when-visual-results-are-noisy).

### Everything shows as NEW

The comparison probably ran with different settings than the snapshot. Look for notes such as `modules differ` or `interaction levels differ`: a check that only ran this time reports everything it finds as NEW. Keep `--level`, `--modules` and `--viewport` in `.soundcheck/config.yaml` so both runs match.

### A broken link that isn't broken

Open the link in a browser. If it loads, check whether the server answers the address **without** its trailing slash (`/about` instead of `/about/`): Soundcheck tests unvisited links in that form. WordPress redirects it, which is fine; a server that answers 404 for it shows up as a broken link. Please [report it](#reporting-a-bug) if you hit this.

### A slowdown I can't reproduce

Lab numbers come from one load per page. Run `compare` again; a real slowdown shows again, noise doesn't. Make sure both runs ran on the same machine under similar load. See [Speed](Soundcheck-Speed#getting-stable-numbers).

### Why is a form "skipped"?

Forms that are hidden, have a password field, are search forms, comment forms, the cart form, or post to `/wp-admin` are never submitted; on production URLs none are, unless allowlisted in `interactions.forms`. The reason is in the run's JSON under `interactions`. See [Forms and flows](Soundcheck-Forms-And-Flows#forms-that-are-never-submitted).

### Where are my test submissions?

Every value Soundcheck types contains `soundcheck-test`. Search your inbox, CRM, form plugin entries or database for it to find and delete them.

## Reporting a bug

Open an issue at [github.com/RhythmShahriar/soundcheck/issues](https://github.com/RhythmShahriar/soundcheck/issues). Include:

- the output of `rhythm soundcheck --version`, your OS, and the Chrome or Chromium version;
- the command you ran (with your real domain replaced by `yoursite.com`);
- the change's check ID (from the HTML report or JSON) and why you believe it's wrong;
- for visual issues, the before, after and diff images if you can share them.

> [!WARNING]
> Never paste credentials, cookies, private URLs or client data into a public issue. Screenshots of a site behind a login can contain personal data; crop or blur them first.

### Found a security issue in Soundcheck itself?

Don't open a public issue. Report it privately as described in the [Security policy](Security-Policy).

---
<sub>← [CLI reference](Soundcheck-CLI-Reference) · [Home](Home) · [Benchmark](Soundcheck-Benchmark) →</sub>
