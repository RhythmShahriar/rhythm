A `.tuner.yml` next to your code keeps scan settings out of the command line and gives accepted risk a reviewed home: each ignore entry says why and until when. This page covers every key, how flags and the file combine and the rules that stop a file you didn't write from misusing your secrets.

```yaml
# .tuner.yml
fail_on: high
min_confidence: confirmed
headers:
  X-Scan-Token: ${TUNER_SCAN_TOKEN}     # read from the environment, never printed
ignore:
  - id: hdr-csp-missing
    reason: the theme needs inline scripts; tracked in issue 42
    until: 2026-12-31                   # after this day the finding shows again
  - id: wp-xmlrpc
    reason: Jetpack needs it
    until: 2026-09-30
```

## Where Tuner looks

A scan reads `.tuner.yml` from the **current folder** only; it doesn't walk up to parent folders. `--config <file>` reads another file instead and `--no-config` reads none. The two don't go together. The `checks` command doesn't read the file.

Every scan that reads one says so on stderr, also under `-q`, `--json` and the other machine formats:

```text
$ TUNER_SCAN_TOKEN=abc123 rhythm tuner scan http://127.0.0.1:20400 --plain
rhythm tuner scan: using .tuner.yml
rhythm tuner scan: ignore for wp-xmlrpc expired 2026-09-30: Jetpack needs it
rhythm tuner 0.4.0 - scanning http://127.0.0.1:20400 (config: .tuner.yml; local address: 50 req/s)
…
Score 0/100 after ignores (1 hidden) - 3 critical, 3 high, 1 medium, 6 low, 1 info - 77 checks, 179 requests, 3.6s
note: vulnerability data: WPVulnerability (wpvulnerability.com, CC0)
run rhythm tuner explain <id> for the full fix
rhythm tuner scan: .tuner.yml: 1 ignore entry hid 1 finding
```

The `hdr-csp-missing` entry hid its finding; the `wp-xmlrpc` entry had expired, so XML-RPC is reported again with a note naming the entry.

## Keys

All keys are optional. An unknown key is an error that names its line, so a typo can't silently do nothing.

| Key | Same as | Notes |
|---|---|---|
| `targets` | targets on the command line | a list. Used only when no target comes from the command line, `-l` or `--stdin`. Folder targets are relative to the file's folder |
| `profile` | `--profile` | `quick` or `default`, any case |
| `rate` | `--rate` | `0` means no limit |
| `concurrency` | `-c` | |
| `fail_on` | `--fail-on` | any case |
| `min_confidence` | `--min-confidence` | `confirmed` or `likely` |
| `baseline` | `--baseline` | relative to the file's folder |
| `headers` | `-H` | a map of name to value. `${TUNER_NAME}` reads an environment variable (see below) |
| `tags`, `exclude_tags`, `ids`, `exclude_ids` | `--tags`, `--exclude-tags`, `--id`, `--exclude-id` | lists |
| `ignore` | | entries with `id` and/or `path`, a `reason` and an optional `until` (below) |

Not in the file (use flags): severity, timeout, retries, parallel, user agent, cookie, proxy and output format.

**Flags win.** A flag on the command line always overrides the file. A header from `-H` replaces the file's header of the same name (case-insensitive); the file's other headers are still sent. When the file sets `rate` or `concurrency` and the command line doesn't, the config note says so: `rhythm tuner scan: using .tuner.yml; it sets --rate 2`.

### Ignore entries

| Field | |
|---|---|
| `id` | a check or finding ID; globs work (`hdr-*`) |
| `path` | a finding path glob; the leading `/` is optional and `*` doesn't cross `/` |
| `reason` | required, so the next person knows why the finding is accepted |
| `until` | optional, `YYYY-MM-DD`, inclusive: the entry applies through that day, local time |

An entry needs an `id` or a `path` (or both). A matched finding is hidden from the live view, every report format and the `--fail-on` and `--fail-on-new` gates. The score line says what hid it: `after ignores (8 hidden)`, `after baseline (1 hidden)` or `after baseline and ignores (9 hidden)`. After `until`, the entry hides nothing and each scan prints `ignore for <id> <path> expired <date>: <reason>`, also under `-q`.

Prefer ignore entries over a baseline for accepted risk you want reviewed: they're short, readable in a pull request and expire on their own. A baseline is better for a long list of findings at once. See [CI and code scanning](Tuner-CI#accepting-known-findings).

### Headers and environment variables

Header values can read the environment with `${NAME}` (only that form; `$NAME` stays literal). Only variables whose names start with `TUNER_` can be read, so a file can't pull an unrelated secret out of your CI environment:

```text
$ rhythm tuner scan 127.0.0.1:20400 -q
rhythm tuner scan: .tuner.yml: headers: X-Leak uses ${SECRET_TOKEN}; only variables that start with TUNER_ can be used, so set TUNER_SECRET_TOKEN and name that
```

An unset or empty variable is an error too, because CI passes a missing secret as an empty string:

```text
rhythm tuner scan: .tuner.yml: headers: X-Scan-Token uses ${TUNER_SCAN_TOKEN}, which is not set
```

Header values never appear in messages. In GitHub Actions, pass the secret under the `TUNER_` name the file uses:

```yaml
- run: rhythm tuner scan https://staging.yoursite.com --sarif -o tuner.sarif
  env:
    TUNER_SCAN_TOKEN: ${{ secrets.SCAN_TOKEN }}
```

## Trust rules

A scan reads `.tuner.yml` from the current folder, so a cloned repository or a pull request can bring its own. These rules keep such a file from turning your secrets or your scan against you. Each refusal is a usage error (exit 2) before any request.

- **A file that chooses the targets can't receive your secrets.** When the targets come from the file, `-H`, `--cookie` and file headers that read a `${TUNER_...}` variable are refused, so a pull request can't edit `targets:` to a host it controls and collect them. Name the targets on the command line, or use `--no-config`:

  ```text
  $ rhythm tuner scan -q -H "Authorization: Bearer x"
  rhythm tuner scan: -H and --cookie need targets on the command line when .tuner.yml chooses them: name the targets or use --no-config
  ```

  Headers written out in the file itself still go to the targets it chose.
- **A file found in the current folder stays in its folder.** Its folder targets and `baseline` can't point outside the folder that holds it (no `../site`, absolute paths or `~` paths). A file named with `--config` may.
- **It always tells you.** The config note names the file, the targets it chose and any `rate` or `concurrency` it set. After the scan another line says how many findings its ignore entries and baseline hid:

  ```text
  rhythm tuner scan: using .tuner.yml, which chose the target: http://127.0.0.1:20400; it sets --rate 2
  ```

Run `rhythm tuner scan --no-config` in folders you don't trust. The [WP-CLI command](Tuner-Integrity-And-Malware#running-it-regularly) runs Tuner in an empty temporary folder with `--no-config`, so a `.tuner.yml` in a web root can't hide findings from it.

## Errors

Every config error names the file and, where it can, the line:

| Message | Fix |
|---|---|
| `.tuner.yml line 2: unknown key "profle"` | fix the key |
| `.tuner.yml: profile: "turbo": want quick or default` | use a listed value |
| `.tuner.yml: ignore 1 (leak-dotenv): needs a reason, so the next person knows why it is accepted` | add `reason:` |
| `.tuner.yml: ignore 1: needs an id or a path` | add one |
| `.tuner.yml: ignore 1 (x): until "31/12/2026": want a date such as 2026-12-31` | use `YYYY-MM-DD` |
| `.tuner.yml: holds more than one YAML document; keep the settings in one` | remove the second `---` section |
| `--config and --no-config don't go together` | pick one |

---
<sub>← [GitHub Action](Tuner-GitHub-Action) · [Home](Home) · [Watch and serve](Tuner-Watch-And-Serve) →</sub>
