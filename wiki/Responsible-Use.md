Rhythm's tools exist to help you find and fix weaknesses in **your own** websites before someone else does. This page sets out who may use them and on what, what Tuner, Soundcheck, Echo, Roadie and Sightread do and never do, how to handle the personal data Echo captures and how to get permission when you test a site for a client.

> [!IMPORTANT]
> Scan and check only sites you own, or sites you have explicit written permission to test. Even a gentle, read-only scan of someone else's site without permission may be illegal.

## Authorized use only

Use Rhythm tools only on:

- systems you own and control, or
- systems whose owner has given you **explicit written permission** to test them, within the scope and time they agreed.

Scanning systems without authorization may be illegal where you are or where the target is, for example under the U.S. Computer Fraud and Abuse Act, the UK Computer Misuse Act 1990, or similar laws elsewhere. Hosting providers, CDNs and web application firewalls may also have their own terms that limit testing.

Owning a domain doesn't always mean you may scan the server behind it. If your site runs on shared hosting or a managed platform, check the provider's terms first.

## Your responsibility

By using Rhythm tools you agree to comply with all applicable laws and the terms of any service you test. You are responsible for how you use them. The authors and contributors are not responsible for misuse, or for any damage, legal consequences or loss arising from their use.

## No warranty

`rhythm` and Tuner are free software under the [GNU Affero General Public License v3.0](https://github.com/RhythmShahriar/rhythm/blob/master/LICENSE); they are provided "as is", without warranty of any kind. The authors' liability is limited, as set out in **section 15 (Disclaimer of Warranty)** and **section 16 (Limitation of Liability)** of the license. Soundcheck and Echo are free software under the MIT License ([Soundcheck](https://github.com/RhythmShahriar/soundcheck/blob/master/LICENSE), [Echo](https://github.com/RhythmShahriar/echo/blob/master/LICENSE)), which provides them "as is", without warranty of any kind, in the same way.

Findings can include false positives and can miss real issues. Verify a finding before you act on it. Don't treat a clean scan as proof a site is secure or a clean comparison as proof nothing broke. Read [Reading Results](Tuner-Reading-Results) to see how Tuner marks findings it's less sure of. Soundcheck's speed numbers and screenshots are lab measurements that vary between runs. Soundcheck's EAA mode gives evidence, not a compliance guarantee: a clean result reads "no failures found by these tests" and many WCAG criteria need a person. An Echo connector can miss a message sent in a way it doesn't hook, so a clean comparison isn't proof every message went out; use a [hard seal](Echo-Sealing-Staging#hard-seals) where a leak would matter.

## What Tuner does

Tuner detects. It never exploits.

- **Remote scans** send ordinary HTTP **GET** requests for public URLs, the same requests a browser or search engine crawler could send. It looks at the replies. It also opens a TLS connection to see which protocol versions and certificate the server offers.
- **Rate limited by default:** at most 5 requests per second per site, 10 in flight. It slows down when the server answers `429` or `503`, honors `Retry-After` and identifies itself with an honest user agent. See [Requests](Tuner-Requests).
- **Secrets are always masked.** When Tuner finds a credential, such as a database password in an exposed `.env`, it reports that it found one and masks the value in every output format.
- **Version matching happens on your machine**, against public vulnerability data. Tuner doesn't try a vulnerability to see if it works.
- **Local scans** of a directory only read files. They never change them and never run the site's code; the one command Tuner runs is the `php` binary, to ask for its version. See [Local Scans](Tuner-Local-Scans).

## What Tuner never does

- Send exploit payloads, injection strings or proof-of-concept attacks.
- Log in, submit forms, or send `POST`, `PUT` or `DELETE` requests to the site. (You can pass your own headers or cookies, for example to reach a staging site behind basic auth; Tuner only sends them with its GET requests.)
- Try passwords or brute-force anything.
- Change, upload or delete anything on the site.
- Print a secret it finds in full.

Checks are declarative YAML files that can only describe GET requests and how to match the reply, so a check, including one delivered in a [check pack](Tuner-Check-Packs), can't add any of these behaviors.

## What Soundcheck does

Soundcheck browses a site like a visitor, in a real browser. It records what it sees. By default it only reads and clicks; anything that sends data is opt-in.

- **Polite by default:** at most 5 page loads a second per site, whatever the number of pages open at once (8 by default), with an honest user agent (`rhythm-soundcheck/<version>`). It honors `robots.txt` unless you pass `--ignore-robots` for your own site. See [Discovery](Soundcheck-Discovery#politeness).
- **Never visits URLs with side effects.** Admin and login pages, logout links, add-to-cart and other cart actions, previews and comment replies are excluded from discovery. See [Discovery](Soundcheck-Discovery#urls-soundcheck-never-visits).
- **Clicks safely.** The default `click` level only clicks controls that act on the page: menus, tabs, accordions, modals. It skips links that leave the page, form submit buttons, anything under `/wp-admin` and controls that look destructive (delete, remove, log out, cancel, empty cart, add to cart, place order, checkout and more). `confirm()` dialogs are answered **Cancel** and pop-ups are blocked. See [Interactions](Soundcheck-Interactions#keeping-clicks-safe).
- **Submits forms only when you ask.** The `submit` level is off by default. When it's on, forms really send: messages arrive, sign-ups are created. Every value contains `soundcheck-test` so you can find and delete test entries. Login, password, search, comment and cart forms are never submitted. WooCommerce checkout is submitted only with an offline or test-mode payment method. See [Forms and flows](Soundcheck-Forms-And-Flows).
- **Refuses production.** On URLs you list under `production` in `.soundcheck/config.yaml`, Soundcheck never submits a form (unless you allowlist that form) and never runs the cache leak test. List your live site there. [Flows](Soundcheck-Forms-And-Flows#flows-on-production) are held back there too: their click and submit steps are refused unless the flow file says `production: true`.
- **Presses keys without sending.** With `--eaa` (or the keyboard module) it presses Tab, Shift+Tab, Enter, Space, Escape and arrow keys on each page. While it does, every request but GET and HEAD, downloads, file choosers and permission prompts are held or refused. It never presses submit controls or anything that looks destructive. Key flows run dry unless they say `keyboard: send`, which submits only on this machine, a private address or with `--staging`, never on production. See [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough#what-the-walk-never-does).
- **Checks as a visitor.** Pages are always checked anonymously. The test account (`SOUNDCHECK_USER`, `SOUNDCHECK_PASSWORD`) is only used for the cache leak test and for flows that ask for it.
- **Keeps credentials out of reports.** The test account comes only from `SOUNDCHECK_USER` and `SOUNDCHECK_PASSWORD`, flow secrets only from `${env:…}`; both are masked in every message. Leaked personal data in a cache leak finding is masked too.
- **Keeps everything local.** Snapshots, screenshots and reports stay in your project's `.soundcheck/` folder. Nothing is uploaded anywhere.

> [!WARNING]
> Screenshots and reports show the pages as Soundcheck saw them. A report of a site with personal data on its pages contains that data: treat reports like the site's own content when you share them.

## What Echo does

Echo captures what a site sends: emails, SMS, chat alerts, webhooks, analytics and payment calls. It runs on your machine or CI runner, next to a staging or local site.

- **Seals by default.** `rhythm echo up` records every message and answers in each service's place, so nothing reaches real people: no email is delivered, no SMS sent, no webhook posted. Mail that reaches Echo's own SMTP sink or `sendmail` stand-in is kept and recorded as `faked`, never delivered. See [Sealing staging](Echo-Sealing-Staging).
- **Refuses live payments, always.** A live-mode payment call (a Stripe live key, or a key Echo can't read; PayPal's live hosts) gets the provider's own refusal in every mode: sealed or observed, allowlisted or not, so a test can't charge a card. Through the proxy, payment APIs are never tunnelled blind: without `--mitm`, a connection to them is refused, so Echo can always see the key.
- **Observe mode really sends.** With `--observe`, messages a connector handles go out for real, to real recipients. Use it only where that's safe. List your test recipients under `allow_recipients` so anyone else is flagged.
- **Connectors stay off in production.** The WordPress, Laravel and Node connectors and the `sendmail` stand-in do nothing where the environment says production, unless you force them. A sealed connector that can't reach Echo refuses to send rather than letting a message out.
- **Actions send real requests to the site you point them at.** Each action in `.echo/config.yaml` is a request, a command or a replay you wrote; Echo refuses `http` actions on URLs you list under `production:`.
- **Replays are held to strict rules.** `rhythm echo replay` only runs with Echo sealed, only sends to URLs listed under `staging:` or that resolve to loopback or private addresses, refuses when a live Stripe key is anywhere in reach or the payload says `livemode`, never follows redirects and can't forge PayPal webhooks at all. See [Replay](Echo-Replay#safety-rules).
- **Deliverability checks never send.** `check` reads DNS, plus the domain's own MTA-STS policy file when asked; `headers` reads a file. They're safe on production.
- **Keeps everything local.** Captured messages stay in Echo's memory. Snapshots, reports and consent results stay in your project's `.echo/` folder; snapshots and runs are excluded from git (add `/.echo/consent/` yourself). Nothing is uploaded anywhere.

### Personal data in captures

> [!WARNING]
> Captured messages contain personal data: the names, addresses, phone numbers and orders of whoever the site emailed, plus links such as password resets, exactly as sent. Echo masks **credentials** (API keys, tokens, `Authorization` and cookie headers) before storing anything; it doesn't mask **content**.

- Treat `.echo/`, the HTML reports and CI artifacts like a database dump: keep them private, delete them when you're done and don't attach them to public issues.
- Prefer test data on staging. A staging copy of production holds real customers; Echo's sealing keeps them from being messaged. Their data still ends up in the captures when the site sends to them.
- On a machine others use, keep Echo on loopback or set a token: without one, anything on the machine can read the captures through the API. Beyond loopback, `up` refuses to start the API, the SMTP sink or the proxy without a token. The SMTP sink then wants the token as its SMTP password (any username).
- If you're testing a client's site, the captures are their customers' data: agree in writing how long you keep them.

### Cookie consent checks

`rhythm echo consent` loads a site in a real browser like one new visitor, three times (no choice, Accept, Reject). It's the one Echo command meant for production as well as staging:

- **Visitor actions only.** It loads pages, scrolls, visits up to ten same-site links and clicks the cookie banner's own buttons. It never logs in, never submits a form, never types into a field and never clicks outside the banner. It skips links to login, cart, checkout, account and admin pages and downloads.
- **Holds tracking hits.** Requests to trackers' collection endpoints are answered inside the browser and never sent, so a check adds nothing to the site's analytics or ad accounts. `--send-hits` sends them, prints a warning and is refused on hosts under `production:`.
- **Stays out of private networks.** A check of a public site can't reach loopback, private or link-local addresses. It never loads the cloud metadata address. Chrome's own background traffic is refused.
- **Says who it is.** Its User-Agent ends in `rhythm-echo-consent/<version>`.
- **Keeps evidence local and masked.** Results go to `.echo/consent/` with tracking IDs masked; screenshots show the site as the visitor saw it. Nothing is uploaded.
- **Is evidence, not legal advice.** A clean result says what a browser saw on these pages, from this place, on this day. It isn't a legal compliance guarantee.

Check only sites you own or are authorised to test. The first check on a machine prints a one-time note saying so.

## What Echo never does

- Deliver mail, send SMS, post webhooks or make payment calls while sealed.
- Let a live-mode payment call through, in any mode.
- Deliver mail that reached its SMTP sink or `sendmail` stand-in, in any mode.
- Send a message to anyone in its own name: its canaries go to `.invalid` addresses and the deliverability checks send no mail at all.
- Replay webhooks to production, with live keys in reach, or while observing.
- Upload captures, reports or telemetry anywhere.
- Store a credential it can recognize unmasked.
- Send a tracking hit during a consent check, unless you pass `--send-hits` on a host that isn't under `production:`.
- Log in, submit a form or click anything but a cookie banner's buttons during a consent check.

## What Roadie does

Roadie copies a production database to staging with every customer pseudonymised, live keys swapped for test keys or blanked and side effects turned off, then checks the copy and signs a report.

- **Reads production, never writes to it.** A database is read in a read-only transaction; over ssh only a fixed list of read commands can run (see [Sources and targets](Roadie-Sources-And-Targets#remote-commands)). A target that is production, is the source or says it is production is refused before anything is written.
- **Keeps real data off your disk.** A copy streams; a source read twice is spooled encrypted with a key held only in memory. `prepare`'s backup is the staging's own data, kept next to its config.
- **Leaves personal data in the copy.** A pseudonymised copy is still personal data under the GDPR: it keeps order totals, dates, products and anything Roadie can't recognise in free text. Protect copies, backups and reports as you protect production.
- **Talks only to what you name**: the source and target, the staging URL with `verify --site`, Echo at the URL you set plus GitHub and Sigstore for updates. No telemetry. See [Security and limits](Roadie-Security-And-Limits).

## What Roadie never does

- Write to a target that matches `production:`, is the source, or says it is production without your `--target-is-staging`.
- Pipe a dump into a database client, run PHP or WP-CLI on your machine, or unpack an archive.
- Print a personal value, a password or a key: findings are masked and no flag shows a raw value.
- Claim a copy holds no personal data. A PASS is what its checks found; the report says what it doesn't cover.

## What Sightread does

Sightread reads an update before you install it. It downloads the installed version and the candidate from the official sources (WordPress.org, Packagist and the archives it lists, npm, OSV) and compares them as text.

- **Never runs what it downloads.** No PHP, Node, npm, Composer or install script runs on fetched code. The only programs it starts are `git` (to read a lockfile at a base ref), your own `--wp` command (a read-only listing) and, with `--soundcheck`, Soundcheck, Roadie and your install command on the staging copy.
- **Reads your site, never changes it.** A review reads plugin headers, lockfiles and a WP-CLI listing. Only `--soundcheck` installs anything: on a staging copy it checks is not production, with a line in the report that says so.
- **Talks only to the official sources,** over HTTPS, with a fixed host list on every redirect, plus mirrors and private registries you configure. Each lookup tells that registry a package name and version you use. Private names (a plugin with its own update server, a private npm scope or Composer repository) are never sent to a public host unless you pass `--check-confusion`.
- **Keeps reports local.** Reports hold your full inventory, are written readable by you only and use relative paths in CI. Nothing is uploaded; there is no telemetry. See [Security model](Sightread-Security-Model).

## What Sightread never does

- Run, install or unpack under their own names the packages it reviews (apart from `--soundcheck` on staging).
- Call a package malicious or safe. It reports what changed and does not judge intent. A clean result is a static diff, not proof an update is safe.
- Resolve, look up or contact the domains it finds in code.
- Send a token to any host but its own (`GITHUB_TOKEN` only to api.github.com).
- Install an update on a site listed as production, or an update on HOLD without `--soundcheck-anyway`.

## The one-time notice

The first time you run a scan on a machine, Tuner prints this on stderr:

```text
Note: scan only sites you own or are authorised to test.
Tuner is non-intrusive and never sends exploit payloads. This notice is shown once.
```

The interactive picker (`rhythm tuner` with no arguments) shows the same reminder: "Only scan sites you own or are authorised to test." Soundcheck prints its own, once:

```text
Note: check only sites you own or are authorised to test. Soundcheck browses
like a visitor at a polite pace, and only submits forms when you turn on the
submit level, never on sites listed as production. This notice is shown once.
```

Echo prints its own, once:

```text
Note: use Echo on sites you own or are authorised to test. Captured messages can
hold personal data: they stay on this machine, with credentials masked. This notice is shown once.
```

Roadie prints its own, once:

```text
Note: use Roadie on sites you own or are authorised to copy. It reads production
data to pseudonymise it and never writes to production. Pseudonymised data is still
personal data: keep copies as protected as the live site. This notice is shown once.
```

Sightread prints its own on the first command:

```text
Note: Sightread downloads packages from WordPress.org, Packagist and npm and reads
them as text. It never runs their code. A clean result is a static diff, not proof
an update is safe. This notice is shown once.
```

When that first command is `demo`, which downloads nothing, it reads instead:

```text
Note: real reviews download packages from WordPress.org, Packagist and npm and read
them as text; this demo stays on 127.0.0.1. Sightread never runs their code. A clean
result is a static diff, not proof an update is safe. This notice is shown once.
```

Seeing a notice once doesn't change the rules; they apply to every scan and check.

## Testing a site for a client

If you're an agency, freelancer or consultant, get written permission before you scan a client's site, even one you built or host. A short email or signed note from someone who can authorize it is enough for most engagements. It should say:

- **Who** is authorizing it and that they have the authority to (the site owner, or someone acting for them).
- **Who** will run the tests: you or your company.
- **What** is in scope: the exact hostnames, for example `yoursite.com` and `staging.yoursite.com`. Also note anything out of scope.
- **What** you'll do: an automated, non-intrusive vulnerability scan with Tuner, read-only GET requests, rate limited, no exploitation. Mention a local scan if you'll run one on their server. For Soundcheck, say whether you'll only browse and click or also submit forms with test data. Say on which sites (staging only, ideally). For Echo, say that you'll capture what the staging site sends (which includes customer data if it's a copy of production), whether it stays sealed and how long you'll keep the captures.
- **When**: the dates or window.
- **Who to contact** if the scan causes any trouble, on both sides.

Also check with the hosting provider or anyone else who runs part of the stack, such as a managed WordPress host or CDN, if their terms require notice.

Keep the permission with your report. For sites behind a firewall, ask the client to allowlist your IP or user agent rather than working around their protection. Use `--rate` to go gentler on fragile hosts.

## If you find a problem on someone else's site

If you come across a weakness on a site you weren't asked to test, don't scan it to confirm. Report what you noticed to the owner through their published security contact (often a `security.txt` file at `/.well-known/security.txt`) and let them investigate.

Found a vulnerability in Rhythm, Tuner, Soundcheck, Echo, Roadie or Sightread itself? See the [Security Policy](Security-Policy).

---
<sub>← [Benchmark](Sightread-Benchmark) · [Home](Home) · [Security policy](Security-Policy) →</sub>
