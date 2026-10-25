Tuner can scan many sites in one run: from the command line, a list file or standard input, several at a time. This page covers the list format, parallelism and rate limits, output for many sites, exit codes when some sites fail and a weekly scan for an agency.

## Three ways to pass targets

```sh
rhythm tuner scan yoursite.com shop.yoursite.com           # on the command line
rhythm tuner scan -l sites.txt                             # from a file
cat sites.txt | rhythm tuner scan --stdin                  # from standard input
```

You can combine them: `rhythm tuner scan yoursite.com -l sites.txt` scans `yoursite.com` and everything in the file. Flags can go before or after the targets.

## List file format

One target per line. Anything after `#` is a comment, blank lines are skipped, surrounding spaces are trimmed and duplicates are scanned once:

```text
# Client sites, scanned weekly
yoursite.com
https://shop.yoursite.com
https://yoursite.com/blog      # a WordPress install in a subfolder

staging.yoursite.com
yoursite.com                   # duplicate: scanned once
/var/www/yoursite              # a local folder works too
```

Targets are the same as on the command line: a hostname, a full URL (a path scopes the scan to that folder) or a local directory for a [local scan](Tuner-Local-Scans). A byte order mark and Windows line endings are fine. Duplicates are found after normalizing: scheme and host in lower case, default ports and a trailing `/` ignored; a bare host counts as the same site as one with a scheme. So `yoursite.com`, `https://yoursite.com/` and `HTTPS://YOURSITE.COM:443` are one scan.

If any target isn't a valid URL, hostname or existing directory, Tuner stops before scanning anything and names it, so a typo doesn't silently skip a site. A line that is far too long stops it too, naming the line: `sites.txt line 2: 70000 characters is too long for a target`.

## Parallel scans and rate limits

`--parallel` sets how many sites are scanned at once. The default is 4.

```sh
rhythm tuner scan -l sites.txt --parallel 8
```

Each host gets its own rate limit (`--rate`, default 5 requests a second, 50 for local and private addresses) and each target its own concurrency limit (`-c`, default 10), so one slow or throttling site never slows the others. Targets on the same host share that host's pace: two paths of one site together see no more than `--rate`. The total load from your machine is up to `--rate` × `--parallel` requests a second, spread across different hosts.

> [!TIP]
> Many client sites on one shared host or behind one firewall see the combined traffic. For those, lower `--parallel` or `--rate` so the host doesn't throttle or block you.

Request flags such as `-H`, `--cookie` and `-k` apply to **every** target in the run. Each target's headers go only to that target's own host. To scan sites that need different credentials, run them separately.

## Output for many sites

**In the terminal**, each site's report is printed whole when that site finishes, so reports never interleave. They appear in the order sites finish, not the order in the list. There's no live progress line in batch mode; add `-v` to watch requests.

**Machine formats** combine all sites:

| Format | Several sites |
|---|---|
| `--json` | an array of report objects, then one `{"target": …, "error": …}` entry per site that couldn't be scanned |
| `--jsonl` | findings from all sites, streamed as found; each line has its `target` |
| `--sarif` | one SARIF log with a run per site |
| `--md` | one Markdown document, sites separated by horizontal rules |

```sh
rhythm tuner scan -l sites.txt --parallel 8 --json > results.json
rhythm tuner scan -l sites.txt -o all.md
```

In Markdown, sites that couldn't be scanned are listed in a `# Not scanned` table; in SARIF each gets an empty run with an error. The error also goes to stderr. With `--json`, a batch always gives an array, even when only one site could be scanned; `--json-array` gives one for a single site too. Pick the reports out with `select(.schema)`:

```sh
jq '.[] | select(.schema) | {target, score}' results.json
```

A quick summary of every site's score, worst first:

```sh
rhythm tuner scan -l sites.txt --json -q \
  | jq -r '.[] | select(.schema) | "\(.score)\t\(.target)"' \
  | sort -n
```

With `--json` and no `-o`, the JSON replaces the live view on stdout. `-q` also keeps warnings off stderr.

## When some sites fail

A site that can't be reached doesn't stop the others. Tuner reports the error and carries on:

```text
rhythm tuner scan: 127.0.0.1:20499: can't reach 127.0.0.1:20499: Get "http://127.0.0.1:20499": dial tcp 127.0.0.1:20499: connect: connection refused
  check the address and that the site is up
```

Reports for the sites that worked are still written. The exit code tells you what happened across the whole run:

| Code | Meaning |
|---|---|
| `0` | every site was scanned and nothing reached `--fail-on` or `--fail-on-new` |
| `1` | at least one site has a finding at or above `--fail-on`, or a new one at or above `--fail-on-new` |
| `2` | usage error, e.g. a bad flag or an invalid target; nothing was scanned |
| `3` | at least one site couldn't be scanned or was cut short, or a gate couldn't pass a partial scan or one without vulnerability data |
| `130` | interrupted (Ctrl-C); partial results are written |

When more than one applies, the first in this order wins: `130`, then `1`, then `3`. So with `--fail-on high`, a run where one site fails and another has a high finding exits `1`; check stderr for the failure.

## Example: a weekly agency scan

A list of client sites, scanned every Monday morning, with a dated Markdown report for the team.

**1. The list**, `~/tuner/sites.txt`:

```text
# Acme Agency clients
yoursite.com
shop.yoursite.com
https://yoursite.com/blog
```

**2. A script**, `~/tuner/weekly.sh`:

```sh
#!/bin/sh
# Weekly Tuner scan of every client site.
set -u
export PATH="$HOME/.local/bin:$PATH"   # where the installer put rhythm
cd "$HOME/tuner" || exit 1
mkdir -p reports
day=$(date +%F)

rhythm tuner scan -l sites.txt --parallel 4 --rate 3 --max-time 30m \
  -o "reports/tuner-$day.md" > "reports/tuner-$day.log" 2>&1
echo "$day exit $?" >> reports/runs.log
```

The plain terminal view and any errors go to a dated log next to the report. `runs.log` keeps each run's exit code.

**3. The schedule.** Run `crontab -e` and add:

```text
# Mondays at 06:00
0 6 * * 1  /bin/sh "$HOME/tuner/weekly.sh"
```

Putting the date logic in the script avoids cron's special handling of `%`. The `PATH` line matters because `rhythm` finds Tuner through `PATH` and cron runs with a minimal one. If you installed somewhere else, or as root (`/usr/local/bin`), adjust it.

> [!NOTE]
> Tuner checks for a newer release at most once a day and only mentions it; it never replaces binaries by itself. Keep scheduled hosts current with `rhythm update` (which also refreshes the data), or set `RHYTHM_NO_UPDATE_CHECK=1` to silence the check. See [Updating](Updating).

Two steps further:

- **What changed.** Also write `-o "reports/tuner-$day.json"` and pass last week's file as `--since`: each run then lists what's new and what was fixed, while `--fail-on-new high` exits `1` only for new problems. See [Changes and notifications](Tuner-Changes-And-Notifications).
- **Alerts.** Add `--notify slack://…` or `--notify smtp://…` to send the new findings, or `--html` for a client report.
- **Let Tuner keep the history.** `rhythm tuner watch -l sites.txt --at 06:00 --fail-on-new high --html` does all of the above in one long-running command, or once per cron run with `--once`. See [Watch and serve](Tuner-Watch-And-Serve).
- **Accepted findings.** A [`.tuner.yml`](Tuner-Config) with ignore entries, or `--baseline accepted.json`, keeps reviewed findings out of the report and the score.

---
<sub>← [Output and reports](Tuner-Output) · [Home](Home) · [Local scans](Tuner-Local-Scans) →</sub>
