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
| Verdict | `broke`, `changed` or `clean` for a comparison; `errors`, `warnings` or `clean` otherwise. A clean [EAA](Soundcheck-EAA) result shows `unchanged` (comparison) or `no failures found` (snapshot, run), never `clean`. |
| LCP (median) | the median over the pages that loaded |

The last line is the trend: a sparkline of the runs shown, then the first and last value and the change. Under `--plain` the sparkline is left out and the arrow is text:

```text
LCP  146 ms to 24 ms (-84%)
```

## Flags

| Flag | Description |
|---|---|
| `--metric lcp\|tbt\|ttfb\|a11y\|keyboard` | The timing to show (default `lcp`), or the open accessibility (`a11y`) or keyboard (`keyboard`) problems. |
| `--page <path>` | That page's figures instead of the site's. The history keeps the first 50 pages of each run. Give the path as visitors see it; tokens in paths are stored masked. |
| `--json` | The series and the history's entries as JSON. |
| `--backfill` | Add the snapshots and runs still in `.soundcheck` that the history doesn't have. The first `history` does this by itself and says on stderr `Added 2 past results still in .soundcheck to the history`. |
| `--plain`, `--no-color` | As for every command. |

```text
rhythm soundcheck: --metric: "a11" isn't one of lcp, tbt, ttfb, a11y, keyboard (did you mean "a11y"?)
```

With no entries: `No history yet: it grows with every snapshot, run and comparison.`

## Accessibility and keyboard

Every run with the a11y or keyboard module on (every `--eaa` run) records how many accessibility and keyboard problems were open and which WCAG criteria they fail. `--metric a11y` shows them. From the [EAA walk-through](Soundcheck-EAA#your-first-eaa-run), whose problems were all keyboard problems:

```text
$ rhythm soundcheck history --metric a11y
When               Kind        Name    Pages  Errors  Warnings  Verdict            A11y open
2026-10-29 01:38   snapshot    before     21       0         0  no failures found  0
2026-10-29 01:39   comparison  before     21       1         1  broke              0
2026-10-29 01:39   comparison  before     21       0         1  unchanged          0

A11y open  0 to 0  ▅▅▅
Worst criteria now (a11y and keyboard problems): 2.1.1 Keyboard (1)
```

```text
$ rhythm soundcheck history --metric keyboard
When               Kind        Name    Pages  Errors  Warnings  Verdict            Keyboard open
2026-10-29 01:38   snapshot    before     21       0         0  no failures found  0
2026-10-29 01:39   comparison  before     21       1         1  broke              2
2026-10-29 01:39   comparison  before     21       0         1  unchanged          1

Keyboard open  0 to 1  ▁█▅
```

The worst criteria are the top 3 of the latest entry with counts, a11y and keyboard problems together. A run without the module shows `-`: a missing count means "not measured", never 0. `--page` shows one page's open problems:

```text
$ rhythm soundcheck history --metric keyboard --page /a11y/wishlist.php
When               Kind        Name    Pages  Errors  Warnings  Verdict            Keyboard open /a11y/wishlist.php
2026-10-29 01:28   snapshot    before     21       0         0  no failures found  0
2026-10-29 01:28   comparison  before     21       1         1  broke              1
...
```

A page the history doesn't have: `no a11y counts for /nope/ in the history; it keeps the first 50 pages of each run with the a11y module on` (exit 2).

### JSON

`--json` prints `metric`, `points` (one per entry: `at`, `kind`, `name`, `target`, `verdict` and `value`, `null` where the entry has none) and `entries` (the history's lines), plus `page` with `--page`:

```json
{
  "at": "2026-10-28T19:39:09.735181583Z",
  "kind": "comparison",
  "name": "before",
  "target": "http://127.0.0.1:39200/",
  "verdict": "broke",
  "value": 2
}
```

Each line of `history.jsonl` from a run with the a11y or keyboard module gains `a11y_open`, `keyboard_open` and `a11y_by_criterion`; an `--eaa` entry also has `"eaa": true`. Its `verdict` stays `clean` when nothing was found.

## Where the history shows up

- The **HTML report** shows the site's LCP trend under the stats once the history has two or more runs of the site: a sparkline and a line such as `LCP 1.4 s to 2.1 s over the last 3 runs` (the last 12 runs).
- The **fleet page** draws each site's trend in its LCP column. See [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports).

Interrupted runs aren't added. A write that fails prints `rhythm soundcheck: history: <why>` and the command goes on. Each line is JSON (schema `rhythm-soundcheck/history/v1`) with the counts, the verdict, the medians and the first 50 pages' timings, so scripts can read it too. The file is in `.soundcheck/` but, unlike snapshots and runs, isn't excluded from git; add it to `.gitignore` if you don't want to commit it.

---
<sub>← [Fleet and client reports](Soundcheck-Fleet-And-Client-Reports) · [Home](Home) · [Import from BackstopJS and Lost Pixel](Soundcheck-Import) →</sub>
