Ten plugins have updates and one of them breaks the contact page. `rhythm soundcheck wp-update` finds out which: it updates a staging site's plugins with WP-CLI, compares the site after each update (or after all of them, then halves the suspects) and names the culprit with its evidence. It can roll a bad update back and carry on. It writes a maintenance log you can send to the client.

```sh
rhythm soundcheck wp-update staging.yoursite.com --wp "ssh deploy@staging -- wp --path=/srv/www" --bisect --rollback
```

> [!IMPORTANT]
> Run it on staging. It changes the site's plugins. It refuses a site listed under `production:` in the config: `wp-update: <site> is a production site (listed under production: in the config); run updates on staging`.

## How it works

1. It asks WP-CLI which plugins have an update (`plugin list --update=available`).
2. It takes a snapshot named `wp-update-<date-time>` with `--viewport desktop --modules health,interact`.
3. **One by one** (the default): it updates one plugin, compares, records the verdict and moves to the next. **Bisect** (`--bisect`): it updates every plugin at once and compares. When that breaks, it rolls half the suspects back per compare, keeping the plugins proven good updated, until one is left.
4. On a break: with `--rollback` it installs the old version again (`plugin install <slug> --version=<old> --force`), compares once more to check the site works again and goes on. Without it, the loop stops and the update stays installed.
5. It prints a summary and writes the log to `.soundcheck/wp-update/`.

With ten plugins and one bad update, bisect finds it in at most four compares after the break; one by one takes up to ten.

## The --wp command

`--wp` says how to run WP-CLI. It's split into words like a shell would. It runs **without** a shell, so nothing is expanded:

| Site | `--wp` |
|---|---|
| this machine | `"wp --path=/srv/www"` |
| over SSH | `"ssh deploy@staging -- wp --path=/srv/www"` |
| Docker Compose | `"docker compose run --rm --no-deps -T --entrypoint wp cli"` |

Over SSH, every word wp-update adds (slugs, versions) is quoted for the remote shell. A `;`, `|`, `&`, `<`, `>`, backtick or `$(` is refused:

```text
rhythm soundcheck: --wp: "|" needs a shell; the wp command runs without one: give the command itself, e.g. "ssh deploy@staging -- wp --path=/srv/www"
```

## Bisect

The output below is from a local test site with five plugin updates, where `contact-extras` 1.1 removes the About page. Progress goes to stderr, the summary to stdout:

```sh
rhythm soundcheck wp-update http://127.0.0.1:18931 --wp "wp --path=/srv/www" --bisect --rollback
```

```text
Snapshot wp-update-2026-10-25T16-00-43.939Z of http://127.0.0.1:18931
Updating 5 plugins
All 5 updated: broke
Rolling back contact-extras to 1.0
Rolling back reading-time to 2.9
Rolling back seo-lite to 4.1
  akismet, breadcrumbs: clean
  contact-extras: broke
Culprit: contact-extras 1.0 → 1.1 (Page not found (404) on /about.php), found in 2 compares
Rolling back contact-extras to 1.0
Without contact-extras: clean
Log written to .soundcheck/wp-update/2026-10-25T16-00-43.939Z.txt
Log written to .soundcheck/wp-update/2026-10-25T16-00-43.939Z.md
Updated 4 plugins, 1 rolled back: contact-extras 1.0 → 1.1 broke the site (Page not found (404) on /about.php).
The site still works.
```

It exits 1: an update broke the site, even though `--rollback` put it right. With `--rollback`, bisect checks the site once more without the culprit and starts again if it still breaks, so two bad updates are both found. The evidence is the culprit's own: a change whose source is in another updated plugin's folder isn't shown as its evidence.

## One by one

Without `--rollback`, the first bad update stops the loop and stays installed. The summary says how to roll it back with your own `--wp` command:

```sh
rhythm soundcheck wp-update http://127.0.0.1:18931 --wp "wp --path=/srv/www"
```

```text
Snapshot wp-update-2026-10-25T16-01-21.001Z of http://127.0.0.1:18931
[1/5] akismet 5.0 → 5.1: clean
[2/5] breadcrumbs 2.3 → 2.4: clean
[3/5] contact-extras 1.0 → 1.1: broke: Page not found (404) on /about.php
Log written to .soundcheck/wp-update/2026-10-25T16-01-21.001Z.txt
Log written to .soundcheck/wp-update/2026-10-25T16-01-21.001Z.md
Updated 3 plugins.
contact-extras 1.0 → 1.1 broke the site (Page not found (404) on /about.php) and is still installed. Roll it back with: wp --path=/srv/www plugin install contact-extras --version=1.0 --force
```

With `--rollback`, one by one installs the old version, compares once more (`after the rollback: clean`) and moves on. If the site still breaks after the rollback, it stops there: `Stopped: the site still breaks after rolling <slug> back to <old>; check it by hand.`

## Flags

| Flag | Default | Description |
|---|---|---|
| `--wp <command>` | (required) | How to run WP-CLI. |
| `--plugins <list>` | all with an update | Only these plugins, e.g. `--plugins akismet,woocommerce`. |
| `--one-by-one` | on | Update and compare one plugin at a time. |
| `--bisect` | off | Update all, then halve the suspects on a break. |
| `--stop-on <class>` | `broke` | What counts as a break: `broke`, `new` (BROKE or NEW) or `changed` (any BROKE, NEW, CHANGED or SLOWER change). |
| `--rollback` | off | Put a bad update back to its old version and go on. |
| `--from-tuner <file>` | none | A `rhythm-tuner --json` report: update the plugins it lists as vulnerable first, worst first. See [Tuner](Tuner). |
| `--config <file>` | `.soundcheck/config.yaml` | Settings file for the snapshot and compares. |
| `-- <flags>` | none | Flags after `--` go to every snapshot and compare, e.g. `-- --limit 50 --page /checkout/`. |

| Code | Meaning |
|---|---|
| `0` | Every update worked. |
| `1` | An update broke the site (also when `--rollback` put it back and the site works now). |
| `2` | Usage error, refused before anything ran: no `--wp`, a production site, a `--wp` that needs a shell. |
| `3` | An update or a check failed. |
| `130` | Interrupted. |

```text
rhythm soundcheck: wp-update: --wp is missing: the WP-CLI command, e.g. "wp --path=/srv/www"
```

## The maintenance log

Each run writes a text and a Markdown log to `.soundcheck/wp-update/<date-time>.txt` and `.md`:

```text
WordPress updates for http://127.0.0.1:18931
25 Oct 2026 22:00 · bisect · stop on broke · snapshot wp-update-2026-10-25T16-00-43.939Z

Updated 4 plugins, 1 rolled back: contact-extras 1.0 → 1.1 broke the site (Page not found (404) on /about.php).
The site still works.

Plugin           Version     Result
akismet          5.0 → 5.1   updated
breadcrumbs      2.3 → 2.4   updated
contact-extras   1.0 → 1.1   rolled back: broke the site, Page not found (404) on /about.php
reading-time     2.9 → 3.0   updated
seo-lite         4.1 → 4.2   updated

4 compares in 26s.
```

## With Tuner

[Tuner](Tuner) finds the plugins with known vulnerabilities. Give wp-update its JSON report and those plugins are updated first, worst first:

```sh
rhythm tuner scan staging.yoursite.com --json -o tuner.json
rhythm soundcheck wp-update staging.yoursite.com --wp "wp --path=/srv/www" --from-tuner tuner.json --rollback
```

On the test site, with a report that lists `seo-lite` and `reading-time` as vulnerable:

```text
Snapshot wp-update-2026-10-25T16-06-55.174Z of http://127.0.0.1:18931
[1/5] seo-lite 4.1 → 4.2: clean
[2/5] reading-time 2.9 → 3.0: clean
[3/5] akismet 5.0 → 5.1: clean
[4/5] breadcrumbs 2.3 → 2.4: clean
[5/5] contact-extras 1.0 → 1.1: broke: Page not found (404) on /about.php
Rolling back contact-extras to 1.0
      after the rollback: clean
Log written to .soundcheck/wp-update/2026-10-25T16-06-55.174Z.txt
Log written to .soundcheck/wp-update/2026-10-25T16-06-55.174Z.md
Updated 4 plugins, 1 rolled back: contact-extras 1.0 → 1.1 broke the site (Page not found (404) on /about.php).
2 vulnerable plugins fixed, site still works.
```

The log marks them `updated (security fix)`. A vulnerable plugin whose update was rolled back is listed as `Still vulnerable: <slugs>.` A file that isn't a Tuner JSON report stops the command: `--from-tuner: <path> isn't a rhythm-tuner JSON report (run rhythm-tuner with --json)`.

---
<sub>← [Error rules and exec](Soundcheck-Error-Rules) · [Home](Home) · [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports) →</sub>
