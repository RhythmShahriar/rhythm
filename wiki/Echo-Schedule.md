Plugin updates on client sites go out in a maintenance window, often at night. `rhythm echo schedule` runs that window for every site on an [agency hub](Echo-Hub-And-Reports): for each site in turn it takes a snapshot of what the site's actions send, runs the update, then compares. The results stay in the hub's store, where `rhythm echo report --site` makes the client's report from them. This page covers the window's syntax, running one now, what it prints and what the update command may see.

```sh
rhythm echo schedule "Tue 02:00" --store /srv/echo --after "wp plugin update --all"   # every Tuesday at 02:00
rhythm echo schedule --now --store /srv/echo --after "wp plugin update --all"         # one window, now
```

`schedule` talks to a running hub, with the hub's read token in its environment (the `read_token_env` of `sites.yaml`).

## One window

```sh
rhythm echo schedule --now --store /home/you/hub/store --after "touch /var/www/staging/wp-content/testbed-state/s01"
```

```text
client-a: snapshot before the update
client-a: touch /var/www/staging/wp-content/testbed-state/s01
client-a: compare with "pre-update"
client-a: Something broke: 2 broke, 0 new, 0 changed, 0 fixed.
```

(The "update" here breaks the test shop's contact form, as a plugin update that switched off the owner's notification would.) For each site in `sites.yaml`, in turn:

1. **Snapshot**: the site's actions from its `config:` run against the hub and are saved as the snapshot `pre-update`, replaced each window.
2. **Update**: the `--after` command runs with `sh -c` in the site's project folder (the one holding its `.echo/`), with `RHYTHM_ECHO_SITE=<id>` and `RHYTHM_ECHO_URL=<the hub>` set. Its output is printed as `<id>:   <line>`.
3. **Compare**: the actions run again and are compared with `pre-update`, with the site config's `checks:` levels and each action's `ignore:`, as `rhythm echo compare` does.

The last line per site is `Something broke: …`, `Something changed: …` or `Nothing broke: …`, `<id>: couldn't compare: <error>` when the compare couldn't run, with ` (the update failed: <error>)` after it when the command failed. Each window keeps `run.json`, `comparison.json` and `schedule.json` in `<store>/sites/<id>/runs/` (104 windows per site), which is what the [agency report](Echo-Hub-And-Reports#the-agency-report) reads.

## When

```sh
rhythm echo schedule "Tue 02:00" --store /home/you/hub/store --after true
```

```text
Next window: Tue 27 Oct 02:00 UTC (in 1d 11h30m)
```

Without `--now` or `--once`, `schedule` waits for each window, runs it and waits for the next, until you stop it (exit `0`). It prints the next window on stderr each time. Times are local.

| Schedule | Runs |
|---|---|
| `"Tue 02:00"` | every Tuesday at 02:00 |
| `"Mon,Thu 03:30"` | Mondays and Thursdays |
| `"daily 02:00"`, `"everyday 02:00"` | every day |
| `"weekdays 22:00"`, `"weekends 06:00"` | Monday to Friday, or Saturday and Sunday |
| `"02:00"` | a bare time: every day |

Day names work in any case, short or long. A time a daylight-saving change skips moves to the next matching day. Anything else is refused (exit `2`):

```text
rhythm echo schedule: schedule "Tue 2am": want days and a time, like "Tue 02:00", "Mon,Thu 03:30", "daily 02:00" or "weekdays 22:00"
```

## Flags

| Flag | Default | Does |
|---|---|---|
| `<when>` | | The schedule, in quotes. Not needed with `--now`. |
| `--store <dir>` | (required) | The hub's store. |
| `--sites <file>` | `sites.yaml` | The `sites.yaml`, as `rhythm echo hub` reads it. |
| `--only <ids>` | all | A comma list of site IDs to run. |
| `--after <command>` | (required) | The update. |
| `--after-timeout <d>` | `30m` | Stop the command after this long. |
| `--snapshot <name>` | | Compare with this snapshot instead of taking one first. |
| `--url <url>` | `http://127.0.0.1:8025` | The hub's API. |
| `--ca <file>` | the hub's self-signed certificate in the store | The certificate to trust for an HTTPS hub. |
| `--now` | | Run one window now and exit. |
| `--once` | | Wait for the next window, run it and exit. |

`--sites` names the file and `--only` picks the sites:

```text
rhythm echo schedule: --sites client-a: no such file; --sites names the sites.yaml (as for rhythm echo hub) and --only picks the sites, like --only client-a
rhythm echo schedule: --only: no site "client-z"; the sites are: client-a
rhythm echo schedule: say when, like "Tue 02:00", or run one window now with --now
```

## Exit codes

With `--now` and `--once`: `0` nothing broke, `1` a site broke or its update failed, `2` a usage error or a bad sites file, `3` couldn't run (no site could be compared at all, such as with the hub down). Ctrl+C during a window exits `130` with `rhythm echo schedule: canceled; the window is partial`.

## What the update may see

The `--after` command runs as your user, with the hub's tokens removed from its environment: every variable `sites.yaml` names (the read token's and each site's) plus any other variable holding one of those values. So it can't ask the hub for other sites' mail. It can still read the store, which holds every site's mail, unless it runs as a user that can't:

```sh
rhythm echo schedule "Tue 02:00" --store /srv/echo --after "sudo -u www-data wp plugin update --all"
```

Every request `schedule` makes for a site carries `X-Echo-Site: <id>`, so the hub ties untagged mail (a queued job, plain SMTP) during that site's actions to that site only.

---
<sub>← [Agency hub and reports](Echo-Hub-And-Reports) · [Home](Home) · [MCP server](Echo-MCP) →</sub>
