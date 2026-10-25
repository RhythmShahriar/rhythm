The health module knows the common ways PHP and WordPress fail. Your site may fail in its own way: a page builder that prints "Widget not found", a theme that shows a warning banner, a log line your framework writes. This page covers adding your own error rules in YAML, testing them with samples and the `exec` module, which runs your own program on every page.

## Error rules

A rule is one YAML file in `.soundcheck/rules/`, next to a file of samples. Health loads them with its built-in rules on every run.

```yaml
# .soundcheck/rules/php-warning-banner.yaml
id: health/theme-warning-banner
match: text
pattern: 'Warning: Undefined array key "(?P<key>[a-z_]+)" in \S*themes/'
level: warning
title: Theme prints a PHP warning banner
source: key
fix: Set the missing theme option, or update the theme.
```

| Field | Required | Means |
|---|---|---|
| `id` | yes | `health/` and lowercase letters, digits and dashes, such as `health/elementor-missing-widget`. It can't be a built-in check's id: rules add checks, they don't replace them. |
| `match` | yes | what the pattern is matched against: `text` (the page's visible text), `header` (each response header as `name: value`, lowercase names), `console` (each console error) or `status` (the status code as digits) |
| `pattern` | yes | a Go (RE2) regular expression, at most 512 characters |
| `level` | yes | `error`, `warning` or `notice`. A new error is BROKE in a comparison; a new warning or notice is NEW. |
| `title` | yes | the issue's title |
| `fix` | no | the fix line shown under it |
| `source` | no | a capture group's number or name. Its text becomes the issue's source, one issue per distinct source (at most 20 per page). Without it, a rule gives at most one issue per page. |
| `group`, `priority` | no | rules in one group report at most one issue per page, the highest `priority` that matches |

Unknown fields are errors.

### Samples

Every rule needs `<name>_test.yaml` beside it, with at least one sample that must match and one that mustn't:

```yaml
# .soundcheck/rules/php-warning-banner_test.yaml
positive:
  - name: the footer warning
    text: 'Warning: Undefined array key "layout" in /var/www/html/wp-content/themes/acme/footer.php on line 42'
    source: layout
negative:
  - name: a plugin's warning
    text: 'Warning: Undefined array key "layout" in /var/www/html/wp-content/plugins/forms/form.php on line 9'
  - name: a page about warnings
    text: Our roast warnings and tasting notes
```

A sample has a `name` and the input the rule looks at: `text`, `headers`, `console` or `status` (200 when left out). A positive sample can also say which `source` it expects.

Without samples, every run stops before it starts, exit 2:

```text
rhythm soundcheck: .soundcheck/rules: php-warning-banner.yaml: no samples; add php-warning-banner_test.yaml with positive and negative samples
```

The same goes for a rule that doesn't load for any other reason (a bad pattern, a reserved id, an id used twice): every error is listed with its file.

### rules test and rules list

```sh
rhythm soundcheck rules test
```

```text
.soundcheck/rules: 1 rule, 3 samples, 0 failures
```

It runs the samples of `.soundcheck/rules`, or of the folder you name (`rules test testbed/rules`), or of the built-in rules when there's no folder. Each failure gets a line and the command exits 1:

```text
FAIL php-warning-banner_test.yaml: the footer warning: source is "layout", want "lay-out"
.soundcheck/rules: 1 rule, 3 samples, 1 failure
```

Run it in CI next to your other tests. A bad folder or bad arguments exit 2.

`rules list` shows every rule health uses and where it comes from:

```text
ID                           LEVEL    TITLE                              FROM
health/wp-critical           error    WordPress "critical error" screen  built in
health/database              error    Database error on the page         built in
health/error-page            error    Application error page             built in
health/theme-warning-banner  warning  Theme prints a PHP warning banner  .soundcheck/rules/php-warning-banner.yaml
```

### In a run

The rule's issue shows like any other:

```text
[WARNING] PHP Warning: Undefined array key "layout" - /, /contact.php, /shop.php
  source: themes/acme/footer.php:42
  fix: Fix the code at the source line and turn display_errors off in production so visitors never see these.
[WARNING] Theme prints a PHP warning banner - /, /contact.php, /shop.php
  source: layout
  fix: Set the missing theme option, or update the theme.
```

Your rules run after the built-in ones, so a rule can't hide a built-in error screen. Commit `.soundcheck/rules/` so the whole team and CI use them.

## exec: your own program on every page

When a rule isn't enough (you want to read the framework's log for the request, or call an internal API), list a program under `exec:` in [`.soundcheck/config.yaml`](Soundcheck-Configuration):

```yaml
exec:
  - name: footer
    run: ["./bin/check-footer"]
    timeout: 5s
```

| Field | Means |
|---|---|
| `name` | lowercase letters, digits and dashes, up to 40 |
| `run` | the program and its arguments, as a list. It runs without a shell. |
| `timeout` | per page, up to `5m` (default `10s`) |

For every page that loaded, each program gets the page on stdin as JSON (schema `rhythm-soundcheck/page/v1`): `path`, `url`, `final_url`, `status`, `title`, the visible `text` (up to 512 KB), `console`, `requests` and a short list of response `headers` (never cookies or auth). It answers on stdout:

```json
{"issues": [{"check": "exec/footer/copyright", "level": "warning", "title": "Footer has no copyright line", "fix": "Restore the footer widget."}]}
```

Each issue's `check` must start with `exec/<name>/`; `level` is `error`, `warning` or `notice` and `title` is required (`detail`, `source` and `fix` are optional). At most 200 issues and 1 MB per page. Issues are compared between runs like any module's. A small Python checker:

```python
#!/usr/bin/env python3
# Reports a page whose footer lost its copyright line.
import json, sys
page = json.load(sys.stdin)
issues = []
if page["status"] == 200 and "©" not in page["text"]:
    issues.append({"check": "exec/footer/copyright", "level": "warning",
                   "title": "Footer has no copyright line",
                   "fix": "Restore the footer widget."})
print(json.dumps({"issues": issues}))
```

### --allow-exec

A config file can come from a pull request, so its commands only run when you allow it on the command line, with `--allow-exec` (or `SOUNDCHECK_ALLOW_EXEC=1`). Without it, the run notes:

```text
note: exec: the config lists commands to run on every page; they run only with --allow-exec
```

With it, on a local test site whose footer lost its line:

```sh
rhythm soundcheck run http://127.0.0.1:18931 --modules health --allow-exec
```

```text
Modules: health, exec
...
[WARNING] Footer has no copyright line - /, /about.php, /contact.php +1 more
  fix: Restore the footer widget.

a few warnings - 0 errors, 1 warning, 0 notices - 4 pages · 4s
```

The `exec` module turns on by itself when the config lists commands, unless `--skip exec`. Programs run in their own process group, with the `SOUNDCHECK_*` and `RHYTHM_ECHO_*` variables removed from their environment. They stop with the run on Ctrl+C. A program that fails, times out or answers something that isn't the expected JSON adds a run warning such as `exec/footer: took longer than 5s and was stopped (first on /path/)`; it never stops the run.

`modules: [+exec]` with no commands is an error: `.soundcheck/config.yaml line 1: modules: exec runs the commands listed under exec: in the config; there are none`. In a [fleet](Soundcheck-Fleet-And-Client-Reports) a site's `args` can't pass `--allow-exec`; set `<PREFIX>_SOUNDCHECK_ALLOW_EXEC=1` for that site instead.

---
<sub>← [Configuration](Soundcheck-Configuration) · [Home](Home) · [WordPress updates](Soundcheck-WP-Update) →</sub>
