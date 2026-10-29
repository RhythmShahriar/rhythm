A static diff tells you what an update adds. It doesn't tell you whether the update breaks the checkout. `--soundcheck <staging-url>` answers the second question in the same run: after the review, Sightread installs the exact archive it verified on a staging copy and has [Soundcheck](Soundcheck) compare the site before and after. [Roadie](Roadie) checks first that the staging copy is safe to break.

```sh
rhythm sightread plugin woocommerce --soundcheck https://staging.shop.test --wp "wp --path=/srv/staging"
```

> [!WARNING]
> This is the one time Sightread's "never runs the code" doesn't hold: `--soundcheck` installs the update on the staging copy and loads it in a browser, so the new code runs there. Point it only at a staging copy. A staging copy made by Roadie is pseudonymised, not anonymised. Echo's seal stops mail, SMS and webhooks, not every outbound connection.

The examples on this page ran against Sightread's testbed: a fixture registry on 127.0.0.1 and a WordPress 6.8.3 staging site in Docker on a network with no route out, published on `http://127.0.0.1:18907`. Its WP-CLI is `testbed/bin/wp`, a wrapper that runs `wp` inside the container. Soundcheck 1.2.2 drove Chrome.

## What it does

1. **Review as usual.** An update on HOLD stops here unless you pass `--soundcheck-anyway`.
2. **Refuse production.** The staging URL must not be listed as production in `soundcheck.production` in the Sightread config, in `production:` of the nearest `.soundcheck/config.yaml` or in Roadie's report. It must not resolve to a production site's address.
3. **Check the copy.** `rhythm-roadie verify --target <url> --wp "<cmd>" --json` must say the copy is Roadie's, signed, sealed by Echo and checked after it was made. `--skip-roadie-verify` skips this and the report says so.
4. **Before.** `rhythm-soundcheck snapshot <url> --name sightread-<name>-<from>`, plus any flags after `--` or in `soundcheck.args`.
5. **Apply the reviewed bytes.** The cached archive is hashed again and must equal the reviewed sha256, then `<wp> plugin install <zip> --force` (or `theme install`). For Composer, npm or a remote staging site, your `--apply` command installs it instead.
6. **After.** `rhythm-soundcheck compare <url> --snapshot <name> --json`.
7. **Roll back** with `--rollback` when Soundcheck finds changes at `--soundcheck-fail-on` (default `broke`): the verified old archive is installed again.

Every program is started with an argument list (never through a shell) and stopped with its children on Ctrl+C.

## A run

`rhythm-roadie` isn't installed on this machine, so the run skips the Roadie check and says so:

```text
$ rhythm sightread plugin e15-soundcheck --soundcheck http://127.0.0.1:18907 --wp testbed/bin/wp --skip-roadie-verify
reviewing 1 of 1: e15-soundcheck 1.0.0 → 1.1.0

  ♪ rhythm sightread 1.0.0  wp-plugin e15-soundcheck  1.0.0 → 1.1.0
  ────────────────────────────────────────────────────────────────────────
  Read-only here. --soundcheck installs the update on the staging copy,
  where its code runs.
  ! --skip-roadie-verify was used: the staging copy wasn't checked by
    rhythm roadie verify, so outbound mail is not known to be sealed
  ! the staging site ran the update; Sightread did not

  ▸ Installed     1.0.0
  ▸ Candidate     1.1.0 · released 2025-10-28 · latest
  ▸ Integrity     both versions: matches WordPress.org's own checksums
                  (both come from WordPress.org, so this proves the copy
                  was not changed on the way, not what the author
                  intended) ✓
  ▸ Publisher     unchanged (scteam)
  ▸ Diff          1 file changed, 1 added · +3 −1 lines · 56 B → 97 B
  ▸ Changelog     none for this version
  ▸ Applied       verified archive 6548eba11c51
  ▸ Roadie        skipped
  ▸ Soundcheck    http://127.0.0.1:18907: 0 broke, 0 new, 0 changed ·
                  .soundcheck/runs/2026-10-28T14-18-56.818Z

 ╭──────────────────────────────────────────────────────────────────────╮
 │  VERDICT                                                    ROUTINE  │
 │  No risk signals found. This is a static diff, not proof the update  │
 │  is safe.                                                            │
 │  Sightread reports what changed. It does not judge intent.           │
 ╰──────────────────────────────────────────────────────────────────────╯
  1 update reviewed · 2 archives verified · 1 from cache · 1.3s

  → update it: wp plugin update e15-soundcheck --version=1.1.0
```

(The `--wp` path is shortened here; the run gave the wrapper's full path.) Afterwards the staging site runs 1.1.0 and the host's files are unchanged:

```text
$ testbed/bin/wp plugin list
name	status	update	version	update_version	auto_update
...
e15-soundcheck	active	none	1.1.0		off
```

`▸ Applied` says what was installed: `verified archive <12 hex>` (the reviewed bytes) or `your command`. `▸ Roadie` is `ok` or `skipped`. `▸ Soundcheck` gives Soundcheck's classes and the path of its report; open that with `rhythm soundcheck report` for the screenshots and details, see [Soundcheck: Reading results](Soundcheck-Reading-Results).

With `--plain`:

```text
Applied: verified archive 6548eba11c51
Roadie: skipped
Soundcheck: http://127.0.0.1:18907: 0 broke, 0 new, 0 changed, .soundcheck/runs/2026-10-28T14-35-03.254Z
```

The JSON report has a `soundcheck` block on the update:

```json
{
  "staging": "http://127.0.0.1:18907",
  "roadie_verify": "skipped",
  "snapshot": "sightread-e15-soundcheck-1.0.0",
  "applied": "verified archive 6548eba11c51",
  "summary": {
    "broke": 0,
    "new": 0,
    "changed": 0
  },
  "report": ".soundcheck/runs/2026-10-28T14-35-03.254Z",
  "rolled_back": false,
  "error": ""
}
```

When Soundcheck stops after it touched the staging copy, the report is incomplete, `error` says why, `summary` is null when the comparison never ran and the terminal line reads `<staging>: not compared: <why>`.

## The gate

The run exits `1` when either gate trips: a finding at `--fail-on`, or a Soundcheck change at `--soundcheck-fail-on` (`broke`, `new` or `changed`). A Soundcheck or Roadie run that fails is exit `3`. With `--rollback`, a tripped Soundcheck gate reinstalls the reviewed old version (`rolled_back: true`); Sightread rolls back only a version it installed and reviewed, so after `--apply` put the old version back with your own command.

## Refusals

Each refusal stops before anything is installed. In the testbed's runs the staging site and the host's files were byte-identical afterwards.

An update on HOLD:

```text
$ rhythm sightread plugin b01-widget --soundcheck http://127.0.0.1:18907 --wp testbed/bin/wp --skip-roadie-verify
...
Verdict: HOLD
1 critical, 4 high, 2 medium
...
rhythm sightread plugin: b01-widget 2.6.8 is on HOLD, so --soundcheck won't install it
  review the findings first; --soundcheck-anyway installs it on the staging copy regardless
```

Exit `1`. A production site, here listed in `.soundcheck/config.yaml`:

```text
rhythm sightread plugin: 127.0.0.1 is listed as production in .soundcheck/config.yaml
  --soundcheck applies the update to the site; point it at a staging copy (rhythm roadie copy makes one)
```

Exit `2`. No Roadie, without `--skip-roadie-verify`:

```text
rhythm sightread: --soundcheck needs rhythm-soundcheck and rhythm-roadie
  install them with: rhythm install soundcheck roadie
```

Exit `3`. A copy Roadie doesn't pass (problems found, no signed marker, mail not sealed, a check older than the copy) is exit `3` too, naming the problems, with the hint `fix it with rhythm roadie prepare, or --skip-roadie-verify (the report says it was skipped)`.

A remote `--wp` for a plugin or theme. WP-CLI can install the verified zip only when it sees the local file, so Sightread refuses before it reviews:

```text
$ rhythm sightread plugin g05-cron --soundcheck http://127.0.0.1:18907 --wp "ssh deploy@staging -- wp --path=/srv/www"
rhythm sightread plugin: --soundcheck can install the verified zip only through a local wp command
  for a remote staging site give --apply "<command that installs g05-cron>"; the report then says the applied bytes were not checked
```

Exit `2`. With `--apply`, the report notes `applied by your command; the applied bytes were not checked against the reviewed ones`.

## Composer and npm

There is no WP-CLI to install a verified archive with, so `--apply` names your command and the report says the bytes it applied weren't checked against the reviewed ones:

```sh
rhythm sightread npm axios --to 1.7.9 --soundcheck https://staging.shop.test \
  --apply "ssh deploy@staging -- 'cd /srv/app && npm install axios@1.7.9 && npm run build'"
```

`--soundcheck` installs one update at a time: name one package. `all` and `changes` don't take it.

## In the config

```yaml
soundcheck:
  staging: https://staging.shop.test
  wp: "wp --path=/srv/staging"
  production: [https://shop.example.com]
  fail_on: broke
  args: [--limit, "100"]
```

With these set, `--soundcheck` alone is enough. `doctor` then checks that `rhythm-soundcheck` and `rhythm-roadie` are installed. See [Configuration](Sightread-Configuration#soundcheck).

## With Echo

Run [Echo](Echo) on the staging copy and its connector seals what the site sends, so the update can't email customers while Soundcheck clicks through the site; Roadie's check requires the seal. To compare what forms and orders send before and after the update too, see [Echo with Soundcheck](Echo-With-Soundcheck).

---
<sub>← [Sources and integrity](Sightread-Sources-And-Integrity) · [Home](Home) · [Watch](Sightread-Watch) →</sub>
