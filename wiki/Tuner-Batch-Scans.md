Tuner can scan many sites in one run: from the command line, a list file or standard input, several at a time. This page covers the list format, parallelism and rate limits, output for many sites, exit codes when some sites fail, and a weekly scan for an agency.

## Three ways to pass targets

```sh
rhythm tuner scan yoursite.com shop.yoursite.com           # on the command line
rhythm tuner scan -l sites.txt                             # from a file
cat sites.txt | rhythm tuner scan --stdin                  # from standard input
```

You can combine them: `rhythm tuner scan yoursite.com -l sites.txt` scans `yoursite.com` and everything in the file. Flags can go before or after the targets.

## List file format

One target per line. Anything after `#` is a comment, blank lines are skipped, surrounding spaces are trimmed, and duplicates are scanned once:

```text
# Client sites, scanned weekly
yoursite.com
https://shop.yoursite.com
https://yoursite.com/blog      # a WordPress install in a subfolder

staging.yoursite.com
yoursite.com                   # duplicate: scanned once
/var/www/yoursite              # a local folder works too
```

Targets are the same as on the command line: a hostname, a full URL (a path scopes the scan to that folder), or a local directory for a [local scan](Tuner-Local-Scans). Duplicates are matched exactly as written, so `yoursite.com` and `https://yoursite.com` count as two targets.

If any target isn't a valid URL, hostname or existing directory, Tuner stops before scanning anything and names it, so a typo doesn't silently skip a site.

## Parallel scans and rate limits

`--parallel` sets how many sites are scanned at once. The default is 4.

```sh
rhythm tuner scan -l sites.txt --parallel 8
```

Each site gets its own rate limit (`--rate`, default 5 requests a second) and its own concurrency limit (`-c`, default 10), so one slow or throttling site never slows the others. The total load from your machine is up to `--rate` × `--parallel` requests a second, spread across different sites. Each site still sees no more than `--rate`.

> [!TIP]
> Many client sites on one shared host or behind one firewall see the combined traffic. For those, lower `--parallel` or `--rate` so the host doesn't throttle or block you.

Request flags such as `-H`, `--cookie` and `-k` apply to **every** site in the run. To scan sites that need different credentials, run them separately.

## Output for many sites

**In the terminal**, each site's report is printed whole when that site finishes, so reports never interleave. They appear in the order sites finish, not the order in the list. There's no live progress line in batch mode; add `-v` to watch requests.

**Machine formats** combine all sites:

| Format | Several sites |
|---|---|
| `--json` | an array of report objects, one per site that was scanned |
| `--jsonl` | findings from all sites, streamed as found; each line has its `target` |
| `--sarif` | one SARIF log with a run per site |
| `--md` | one Markdown document, sites separated by horizontal rules |

```sh
rhythm tuner scan -l sites.txt --parallel 8 --json > results.json
rhythm tuner scan -l sites.txt -o all.md
```

Sites that couldn't be scanned aren't in the report; their errors go to stderr. With `--json`, a batch always gives an array, even when only one site could be scanned (since v0.3.2):

```sh
jq '.[] | {target, score}' results.json
```

A quick summary of every site's score, worst first:

```sh
rhythm tuner scan -l sites.txt --json -q \
  | jq -r '.[] | "\(.score)\t\(.target)"' \
  | sort -n
```

With `--json` and no `-o`, the JSON replaces the live view on stdout. `-q` also keeps warnings off stderr.

## When some sites fail

A site that can't be reached doesn't stop the others. Tuner reports the error and carries on:

```text
rhythm tuner: https://old.yoursite.com: can't reach old.yoursite.com: Get "https://old.yoursite.com": dial tcp: lookup old.yoursite.com: no such host
  check the address and that the site is up
```

Reports for the sites that worked are still written. The exit code tells you what happened across the whole run:

| Code | Meaning |
|---|---|
| `0` | every site was scanned, and nothing reached `--fail-on` |
| `1` | at least one site has a finding at or above `--fail-on` |
| `2` | usage error, e.g. a bad flag or an invalid target; nothing was scanned |
| `3` | at least one site couldn't be scanned, or was cut short by `--max-time` |
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

The plain terminal view and any errors go to a dated log next to the report, and `runs.log` keeps each run's exit code.

**3. The schedule.** Run `crontab -e` and add:

```text
# Mondays at 06:00
0 6 * * 1  /bin/sh "$HOME/tuner/weekly.sh"
```

Putting the date logic in the script avoids cron's special handling of `%`. The `PATH` line matters: cron runs with a minimal `PATH`, and `rhythm` finds Tuner through `PATH`. If you installed somewhere else, or as root (`/usr/local/bin`), adjust it.

> [!NOTE]
> Tuner checks for a newer release at most once a day and only mentions it; it never replaces binaries by itself. Keep scheduled hosts current with `rhythm update` (which also refreshes the data), or set `RHYTHM_NO_UPDATE_CHECK=1` to silence the check. See [Updating](Updating).

Two steps further:

- **Only new findings.** Save a JSON report once (`rhythm tuner scan -l sites.txt -o accepted.json`), then add `--baseline accepted.json` to the weekly command. Findings in it drop out of the report and the score. See [CI and code scanning](Tuner-CI).
- **Alerts.** Add `--fail-on high` and send mail when the exit code is `1`.

---
<sub>← [Output and reports](Tuner-Output) · [Home](Home) · [Local scans](Tuner-Local-Scans) →</sub>
