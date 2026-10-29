Look after many sites without opening each one. `all --sites` reviews every site of a fleet file in one report, each update once however many sites run it. `watch` does that on a schedule and tells Slack, Discord, email or a webhook only what is new: an update worth a look, a new signal on a version a site already runs, an owner change or a WordPress.org closure.

The examples on this page ran against Sightread's testbed (a fixture registry on 127.0.0.1) with two WordPress sites, `shop` and `bakery`, in one folder.

## Fleets

A fleet file lists sites. Each has a `name` and one way to read what it runs:

```yaml
sites:
  - name: shop
    path: /var/www/shop
  - name: brochure
    wp: "ssh deploy@brochure.example -- wp --path=/srv/www"
  - name: api
    path: ~/code/acme-api
  - name: client-remote
    tuner_report: ~/reports/client.json
    allow_file: ~/agency/allowed.yaml
```

| Key | Meaning |
|---|---|
| `name` | the site's name in reports and notifications |
| `path` | a WordPress root or a project folder with lockfiles; relative paths are relative to the fleet file's folder, `~` is your home |
| `wp` | a WP-CLI command that lists the site's plugins and themes, local or over ssh |
| `tuner_report` | a `rhythm-tuner` JSON report of the site, for a site you can't reach |
| `allow_file` | an allow file for this site, on top of the project's |

The file is read with limits on YAML aliases and depth. `watch` reads it again before every run, so you can add a site without restarting.

### all --sites

```text
$ rhythm sightread all --sites sites.yaml
reviewing 2 of 20: b01-widget 2.6.7 → 2.6.8
reviewing 1 of 20: b04-roles 2.0.0 → 2.0.1
...
reviewing 20 of 20: g07-theme 1.0.0 → 1.1.0

  ♪ rhythm sightread 1.0.0  20 updates
  ────────────────────────────────────────────────────────────────────────
  Read-only. Sightread changes nothing on your site and never runs the
  code it downloads.

  UPDATE                       VERDICT             FINDINGS
  b01-widget 2.6.7 → 2.6.8     HOLD on 2 sites     1 critical, 4 high, 2 …
  b03-seo 3.0.0 → 3.0.1        HOLD on 1 site      1 critical, 2 high, 1 …
  b05-cache 1.1.0 → 1.1.1      HOLD on 1 site      1 critical, 1 high, 1 …
  b07-backup 5.0.0 → 5.0.1     HOLD on 1 site      2 critical, 1 high, 1 …
  b08-gallery 2.3.0 → 2.3.1    HOLD on 1 site      1 critical, 2 medium
  b21-essential 2.6.7 → 2.6.8  HOLD on 1 site      2 critical, 4 high
  b02-formtools 1.4.0 → 1.4.1  REVIEW on 2 sites   3 high, 1 medium
  b04-roles 2.0.0 → 2.0.1      REVIEW on 1 site    2 high, 1 medium
  b06-ajax 4.2.0 → 4.2.1       REVIEW on 1 site    2 high, 1 medium
  b09-theme 1.5.0 → 1.6.0      REVIEW on 2 sites   4 high, 1 medium
  g15-takeover 6.0.0 → 6.0.1   REVIEW on 2 sites   1 high
  g01-feature 4.0.0 → 4.1.0    ROUTINE on 2 sites
  g02-bundle 1.0.0 → 1.1.0     ROUTINE on 2 sites
  g03-i18n 2.0.0 → 2.1.0       ROUTINE on 2 sites
  g04-vendor 1.0.0 → 1.0.1     ROUTINE on 2 sites
  g05-cron 1.0.0 → 1.1.0       ROUTINE on 2 sites  1 low
  g06-contrib 1.0.0 → 1.1.0    ROUTINE on 2 sites
  g07-theme 1.0.0 → 1.1.0      ROUTINE on 2 sites
  g08-eol 1.0.0 → 1.0.1        ROUTINE on 2 sites
  g14-site 1.0.0 → 1.1.0       ROUTINE on 2 sites

  HOLD  b01-widget 2.6.7 → 2.6.8 on 2 sites ──────────────────────────────
 ▌  CRITICAL   Fetches remote code and runs it
 ▌             includes/class-analytics.php:3
...
  NOT CHECKED  3 ─────────────────────────────────────────────────────────

    b22-premium 2.8.0 (wp-plugin): not on WordPress.org (site bakery)
      → give the vendor's zip with --to-file
    wordpress 6.8.3: WordPress core is out of scope in v1 (site shop)
      → WordPress core updates come signed from WordPress.org
    wordpress 6.8.3: WordPress core is out of scope in v1 (site bakery)
      → WordPress core updates come signed from WordPress.org

 ╭──────────────────────────────────────────────────────────────────────╮
 │  20 UPDATES                           6 hold · 5 review · 9 routine  │
 │  ■ 8 critical   ■ 24 high   ■ 11 medium   ■ 1 low                    │
 │  Incomplete review: 1 package wasn't checked.                        │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  20 updates reviewed · 40 archives verified · 20 from cache · 0s

  → every finding and the full diff: run again with -o review.html --open
  → explain a finding: rhythm sightread explain remote-fetch-exec
  → update the routine ones:
    wp plugin update g01-feature g02-bundle g03-i18n g04-vendor g05-cron g06-contrib g08-eol g14-site
  → update the routine ones: wp theme update g07-theme
  → record a review:
    rhythm sightread allow wp-plugin:b01-widget@2.6.8 --reason "..."
```

Each update is downloaded and reviewed once, however many sites run it. `-o fleet.html` writes one page grouped by update, with the sites under each; `--md -o fleet.md` the same in Markdown for a client. Through `--wp`, a site with plugin auto-updates on gets a warning: WordPress may install them before you review (`wp plugin auto-updates disable --all`).

## watch

```sh
rhythm sightread watch -l sites.yaml --at 02:00 --notify slack://hooks.slack.com/services/T0/B0/XXXX
rhythm sightread watch -l sites.yaml --every 24h --notify-min medium --html
rhythm sightread watch -l sites.yaml --once --fail-on-new high
```

`--every <duration>` runs now and then every duration (at least `1m`); `--at 02:00` runs daily at that local time (several: `02:00,14:00`), with `--now` to also run right away; `--once` runs once and exits, for cron and CI. Each run is its own `rhythm sightread all --sites` process, so memory never grows however long the watch runs. It writes `<dir>/<UTC time>.json` (and `.html` with `--html`), keeps the newest 30 (`--keep`) and prints one line:

```text
$ rhythm sightread watch -l sites.yaml --once --dir watch --notify webhook+http://127.0.0.1:18906/hook --fail-on-new high
2026-10-28 20:21 +06: 2 sites, 20 updates available, 11 to review (6 hold); report .../agency/watch/20261028T142125Z.json; incomplete: 1 not checked (see .../agency/watch/20261028T142125Z.log)
```

That first run exits `1`: `--fail-on-new high` fails when a run finds a finding at or above high that no earlier run of this watch had. The same command again finds nothing new and exits `0`:

```text
2026-10-28 20:21 +06: 2 sites, 20 updates available, 11 to review (6 hold); report .../agency/watch/20261028T142125Z-2.json; incomplete: 1 not checked (see .../agency/watch/20261028T142125Z-2.log)
```

The `.log` next to each report is that run's own output. `--dir` defaults to the user cache (`rhythm/sightread/watch/<name>`, the name defaulting to the fleet file's); it also holds `state.json`, which remembers what was notified, plus a lock so two watches can't share it. Review flags for every run go after `--`, e.g. `-- --jobs 2 --no-osv`. Ctrl+C lets a review in progress finish, then stops; a second Ctrl+C stops the review too.

### What it notifies

- An update with findings at or above `--notify-min` (default `high`), once per candidate version per site, grouped by update.
- After a signal data update (`rhythm sightread update`), a new signal on a version a site already runs: the watch reviews the installed versions against the release before them.
- An owner change or a WordPress.org closure of an installed plugin, even without a new version.

A version already notified isn't notified again, so a nightly watch stays quiet until something changes.

### Where it notifies

| `--notify` | Sends |
|---|---|
| `slack://hooks.slack.com/services/T0/B0/XXXX` | a Slack incoming webhook |
| `discord://discord.com/api/webhooks/123/abc` | a Discord webhook |
| `webhook+https://example.com/hook` | JSON to any HTTPS endpoint (`webhook+http://` only on loopback) |
| `smtp://user@mail.example.com:587?to=ops@example.com&from=sightread@example.com` | email (`smtps://` for TLS from the start); the password comes from `SIGHTREAD_SMTP_PASSWORD` |

`--notify` repeats for several. A webhook gets one JSON document per run; this one went to a receiver on 127.0.0.1 (shortened):

```json
{"schema":"rhythm-sightread-notify/v1","tool":"rhythm-sightread","version":"1.0.0","watch":"sites","min_severity":"high","summary":"Sightread sites: 11 updates to look at","report":".../agency/watch3/20261028T142201Z.json","items":[{"kind":"update","ecosystem":"wp-plugin","name":"b01-widget","from":"2.6.7","to":"2.6.8","verdict":"hold","sites":["shop","bakery"],"severity":"critical","findings":7,"top":"Fetches remote code and runs it"},{"kind":"update","ecosystem":"wp-plugin","name":"b02-formtools","from":"1.4.0","to":"1.4.1","verdict":"review","sites":["shop","bakery"],"severity":"high","findings":4,"top":"Contacts a new domain"},...],"incomplete":"Partial: 1 not checked; see .../agency/watch3/20261028T142201Z.log"}
```

A notification that can't be sent is printed under the run's line. With `--notify-strict` it also exits `3`:

```text
$ rhythm sightread watch -l sites.yaml --once --dir watch2 --notify webhook+http://127.0.0.1:38949/hook --notify-strict
2026-10-28 20:21 +06: 2 sites, 20 updates available, 11 to review (6 hold); report .../agency/watch2/20261028T142131Z.json; incomplete: 1 not checked (see .../agency/watch2/20261028T142131Z.log)
  couldn't notify webhook+http://127.0.0.1:38949: connection refused
```

A URL with no path is refused before the run: `--notify: webhook+http://127.0.0.1:38949: the URL has no path; paste the whole webhook URL`. `-v` prints every notification sent, secrets masked.

### Running it for good

`watch --at` runs in the foreground. Keep it running with your service manager, or schedule `--once` with cron and let the state folder carry over between runs:

```cron
0 2 * * * rhythm sightread watch -l /home/ann/agency/sites.yaml --once --notify slack://hooks.slack.com/services/T0/B0/XXXX
```

In GitHub Actions, the [Action](Sightread-CI)'s weekly `all` run does the same for one repository.

---
<sub>← [With Soundcheck and Roadie](Sightread-With-Soundcheck) · [Home](Home) · [CI and the GitHub Action](Sightread-CI) →</sub>
