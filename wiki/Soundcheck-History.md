Every snapshot, run and comparison adds a line to `.soundcheck/history.jsonl`. `rhythm soundcheck history` shows them as a table with the site's median LCP (or TBT or TTFB) and its trend, so you can see a site getting slower over months, not only since the last snapshot.

```sh
rhythm soundcheck history
```

```text
When               Kind        Name           Pages  Errors  Warnings  Verdict   LCP (median)
2026-10-25 21:52   snapshot    before-update      4       0         1  warnings  146 ms
2026-10-25 21:52   comparison  before-update      4       1         4  broke     24 ms

LCP  █▁  146 ms → 24 ms (-84%)
```

| Column | Means |
|---|---|
| When | when the run started, local time |
| Kind | `snapshot`, `run` or `comparison` |
| Name | the snapshot's name; for a comparison, the snapshot it compared with |
| Errors, Warnings | the run's issues by level |
| Verdict | `broke`, `changed` or `clean` for a comparison; `errors`, `warnings` or `clean` otherwise |
| LCP (median) | the median over the pages that loaded |

The last line is the trend: a sparkline of the runs shown, then the first and last value and the change. Under `--plain` the sparkline is left out and the arrow is text:

```text
LCP  146 ms to 24 ms (-84%)
```

## Flags

| Flag | Description |
|---|---|
| `--metric lcp\|tbt\|ttfb` | The timing to show (default `lcp`). |
| `--page <path>` | That page's timing instead of the site's median. The history keeps the first 50 pages of each run. |
| `--backfill` | Add the snapshots and runs still in `.soundcheck` that the history doesn't have. The first `history` does this by itself and says on stderr `Added 2 past results still in .soundcheck to the history`. |
| `--plain`, `--no-color` | As for every command. |

```text
rhythm soundcheck: metric "cls": want lcp, tbt or ttfb
```

With no entries: `No history yet: it grows with every snapshot, run and comparison.`

## Where the history shows up

- The **HTML report** shows the site's LCP trend under the stats once the history has two or more runs of the site: a sparkline and a line such as `LCP 1.4 s to 2.1 s over the last 3 runs` (the last 12 runs).
- The **fleet page** draws each site's trend in its LCP column. See [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports).

Interrupted runs aren't added. A write that fails prints `rhythm soundcheck: history: <why>` and the command goes on. Each line is JSON (schema `rhythm-soundcheck/history/v1`) with the counts, the verdict, the medians and the first 50 pages' timings, so scripts can read it too. The file is in `.soundcheck/` but, unlike snapshots and runs, isn't excluded from git; add it to `.gitignore` if you don't want to commit it.

---
<sub>← [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports) · [Home](Home) · [Import from BackstopJS and Lost Pixel](Soundcheck-Import) →</sub>
