The European Accessibility Act asks services to publish an accessibility statement: how accessible the site is, what isn't yet, how it was assessed and how people report problems. Every `--eaa` run drafts one from what its tests found. It also keeps an evidence pack of the files behind it that anyone can verify. This page covers the `statement` command, its `statement:` config block, what the draft says, the EARL export for the W3C WCAG-EM Report Tool, the JSON and the evidence pack with `evidence verify`, `sign` and `keygen`.

```sh
rhythm soundcheck statement -o accessibility-statement.md   # also .html, .json and .jsonld (EARL)
rhythm soundcheck evidence verify                           # the files and the chain behind it are unchanged
```

> [!IMPORTANT]
> The draft is a **draft**. Soundcheck never chooses the status ("fully", "partially" or "not compliant"): that `[TO DO]` is always yours, after your own review. Of the 50 WCAG 2.1 A and AA criteria, 23 can only be judged by a person; the draft says so for each of them. Evidence, not a compliance guarantee.

## The statement command

```text
rhythm soundcheck statement [run folder|snapshot] [flags]
```

It drafts from the newest `--eaa` run or snapshot, or from the run folder or snapshot you name. Every `--eaa` run already writes `eaa/statement-draft.md` into its folder; that file is written again on every run, so edits belong in a copy made with `-o`.

| Flag | Default | Description |
|---|---|---|
| `--md` | on a terminal and for `.md` | Markdown. |
| `--html` | | Standalone HTML, printable, with no scripts and a strict Content Security Policy. |
| `--json` | | JSON, schema `rhythm-soundcheck/statement/v1`. |
| `--earl` | | EARL JSON-LD for the W3C WCAG-EM Report Tool (`.jsonld`). |
| `-o`, `--output <file>` | stdout | Write to a file, format from the extension (`.md`, `.html`, `.json`, `.jsonld`); repeatable. `-o -` is stdout. |
| `--layout <name>` | `model` | `model`: the EU model statement (Implementing Decision (EU) 2018/1523). `service`: EAA Annex V service information (BFSG Anlage 3 in Germany). |
| `--wcag <version>` | the run's | `2.1` or `2.2`. A 2.2 run can be drawn as 2.1, not the other way round. |
| `--config <file>` | `.soundcheck/config.yaml` | Read `statement:` from this file. |
| `--lang <code>` | `en` | Only `en` in 1.3. |
| `--force` | off | Replace a draft file you have edited. |

```sh
rhythm soundcheck statement -o accessibility-statement.md
```

```text
Statement draft written to accessibility-statement.md
10 [TO DO] markers left
```

Several formats at once:

```sh
rhythm soundcheck statement -o statement.jsonld -o statement.json -o statement.html
```

```text
Statement draft written to statement.jsonld
Statement draft written to statement.json
Statement draft written to statement.html
10 [TO DO] markers left
```

Files are written readable by you only (0600) and their SHA-256 is recorded in the run's `eaa/written.json`. A file you have edited since is never replaced by accident:

```text
rhythm soundcheck: accessibility-statement.md was edited since Soundcheck wrote it; write to another file with -o, or add --force to replace your edits
```

### Exit codes and errors

| Code | Meaning |
|---|---|
| `0` | Written. |
| `2` | Usage error: a bad flag, a missing or wrong `statement:` block, an edited file without `--force`. |
| `3` | No `--eaa` data, an interrupted run, or a file couldn't be read or written. |

```text
rhythm soundcheck: no --eaa results in .soundcheck yet; run rhythm soundcheck snapshot <url> --eaa first
rhythm soundcheck: the run used the WCAG 2.1 rules; --wcag 2.2 needs a run made with --wcag 2.2
rhythm soundcheck: --layout: "services" isn't one of model, service (did you mean "service"?)
```

Without the three required keys: `statement needs statement.owner, statement.website and statement.contact.email in .soundcheck/config.yaml (rhythm soundcheck init --eaa writes them); a statement without a way to report problems isn't one` (exit 2). An interrupted run is refused (`<run> was interrupted after 17 of 42 pages; a statement from part of the site would leave problems out. Run it again`, exit 3). A symbolic link at an `-o` path is refused, even with `--force`.

## The statement: block

Your own text lives in `.soundcheck/config.yaml`, not in the draft, so writing the draft again never loses it. `rhythm soundcheck init --eaa` writes the block commented out; fill it in:

```yaml
statement:
  owner: "Ledger Coffee GmbH"
  website: "https://ledger.example"
  contact:
    email: "accessibility@ledger.example"
  response_days: 10
  country: "DE"
```

| Key | Required | What it holds |
|---|---|---|
| `owner` | yes | Who runs the website. |
| `website` | yes | The public address. The address the run checked (a staging host, `127.0.0.1` and its port) never appears in the draft. |
| `contact.email` | yes | Where people report problems. Use a role address such as `accessibility@`, not a person's own. |
| `contact.phone`, `contact.address` | no | Other ways to reach you. |
| `response_days` | no | Days to reply, 1 to 365. |
| `country` | no | ISO 3166 alpha-2 code, such as `DE`; the enforcement `[TO DO]` names it. |
| `enforcement` | no | The authority that enforces accessibility law and how to reach it. |
| `scope` | no | Page globs the statement covers, such as `[/, /shop/*, /checkout/*]`. Without it the draft covers the pages checked and asks you to confirm them. |
| `service` | no | A general description of the service, for `--layout service`. |
| `known_issues` | no | Problems you know about: `criterion`, `text` and an optional `until` date. |
| `manual` | no | What a person checked: `criterion`, `result` (`failures found`, `no failures found` or `not checked`), `by` and `date`, all required. A manual result always wins over the tests for its criterion; the draft records who checked and when and keeps the tests' result next to it. |
| `list_private_paths` | no | `true` lists the paths of a logged-in run; by default only their number. |

Everything in the block is published in the statement. Unknown keys and bad values stop the command with exit 2 and name the file and line, such as `unknown setting "statement.ownr" (did you mean "statement.owner"?)` or `statement.country: "de" isn't an ISO 3166 country code such as DE, FR or IE (did you mean "DE"?)`.

```yaml
  known_issues:
    - criterion: 1.2.2
      text: "Product videos have no captions yet."
      until: 2027-03-31
  manual:
    - criterion: 1.4.11
      result: no failures found
      by: Jane Doe
      date: 2026-10-20
```

## What the draft says

Every format starts with the same banner:

```markdown
> **Draft for you to review. Not ready to publish.**
>
> Rhythm Soundcheck 1.3.0 wrote this on 2026-10-29 from a check of 21 pages of https\://ledger.example.
>
> Automated and keyboard tests find some accessibility problems, not all. This is evidence for your own review, not a statement that the site meets WCAG, EN 301 549 or the European Accessibility Act.
>
> Fill in every \[TO DO\], test the criteria marked "needs a person", then delete this box.
```

The default layout follows the EU model accessibility statement. From the [walk-through's](Soundcheck-EAA#your-first-eaa-run) comparison, with a keyboard trap and a control that ignores Enter and Space:

```markdown
# Accessibility statement for Ledger Coffee GmbH

Ledger Coffee GmbH wants https\://ledger.example to be usable by as many people as possible.

This statement covers the 21 pages Soundcheck checked, listed under "Pages checked". [TO DO: set statement.scope to the pages this statement covers, or check that these are all of them.]

## How accessible this website is

[TO DO: choose one after your review: "fully compliant", "partially compliant" or "not compliant" with WCAG 2.1 level AA / EN 301 549 V3.2.1. Soundcheck does not choose this for you. Its tests found failures of 2 of 50 criteria and could not test 23 of them.]

## What isn't accessible yet

### Problems found

Found by Soundcheck's tests on 2026-10-29, or by your own review:

- **2.1.1 Keyboard** (EN 301 549 9.2.1.1): 1 element on 1 page: /a11y/wishlist.php.
- **2.1.2 No Keyboard Trap** (EN 301 549 9.2.1.2): 1 element on 1 page: /a11y/consent.php.

[TO DO: for each problem, say in plain words what doesn't work, for whom and what you offer instead.]

### Disproportionate burden

[TO DO: say if fixing any of this would be a disproportionate burden and why, or delete this section.]

### Content outside the scope of the law

[TO DO: list content the law doesn't cover, such as third-party content you don't control, or delete this section.]

## How this statement was prepared

This statement was prepared on [TO DO: date] by [TO DO: who].

Rhythm Soundcheck 1.3.0 checked 21 pages of https\://ledger.example on 2026-10-29 with automated tests (axe-core 4.14.0 with 65 rules plus 2 Soundcheck checks for WCAG 2.1 A and AA) and a keyboard walkthrough of 21 pages (1 widget tested, 1 key flow run).

It was also checked by [TO DO: your manual review, such as a screen reader test of checkout].

It was last reviewed on [TO DO: date].
...
## Feedback and contact

If something on this website doesn't work for you, tell us: email accessibility\@ledger.example.

We reply within 10 days.

## Enforcement

[TO DO: name the authority that enforces accessibility law in DE and how to reach it; or set statement.enforcement.]
```

It goes on with the nine limits sentences under **What these tests can and can't tell** (the same as [Honest limits](Soundcheck-EAA#honest-limits)), then three more sections:

- **Appendix: every WCAG 2.1 A and AA criterion**, a table with the EN 301 549 clause, how Soundcheck tests it and the result:

  ```markdown
  | Criterion | Level | EN 301 549 | How Soundcheck tests it | Result |
  |---|---|---|---|---|
  | 1.1.1 Non-text Content | A | 9.1.1.1 | partly: axe aria-meter-name, aria-progressbar-name, image-alt, input-image-alt, object-alt, role-img-alt, svg-img-alt | no failures found by these tests |
  | 1.2.3 Audio Description or Media Alternative (Prerecorded) | A | 9.1.2.3 | not tested: needs a person | not tested: needs a person |
  ...
  | 2.1.1 Keyboard | A | 9.2.1.1 | partly: axe frame-focusable-content, scrollable-region-focusable, server-side-image-map; keyboard/not-reachable, keyboard/not-operable, keyboard/menu, keyboard/flow | failures found (1 element, 1 page) |
  | 2.1.2 No Keyboard Trap | A | 9.2.1.2 | partly: keyboard/trap | failures found (1 element, 1 page) |
  ...
  | 4.1.1 Parsing | A | 9.4.1.1 | not tested: W3C now treats it as always met for HTML; the owner decides how to state it | not tested: W3C now treats it as always met for HTML; the owner decides how to state it |
  ```

- **Not tested**: pages whose audits didn't finish, frames from other sites, key flows that didn't run, `23 criteria need a person: 1.2.3, 1.2.4, ...`, `Only Chrome was used. The keyboard walk ran at desktop width.`, `No screen reader, voice control or test with disabled users.` and the EN 301 549 clauses outside a website check (5, 6, 7, 10, 11 and 12).
- **Pages checked**: the paths, or for a logged-in run only their number unless `list_private_paths: true`.

Problems your ignore rules or accepts left out of Soundcheck's results are listed under **Accepted or ignored in Soundcheck** with their reason. They still count against their criteria: ignoring a problem doesn't fix it. From `--layout service`, after the wishlist control was ignored:

```markdown
### Accepted or ignored in Soundcheck

These problems were found and left out of Soundcheck's results by your ignore rules or accepts. They still count against their criteria above.

- **2.1.1** (keyboard/not-operable) on /a11y/wishlist.php: wishlist button is rebuilt in the next sprint
```

### Results in the criteria table

| Result | When |
|---|---|
| `failures found (<n> elements, <m> pages)` | A test found a failure. |
| `no failures found by these tests` | The tests ran and found none. Never "passed". |
| `not tested: needs a person` | No test covers the criterion. |
| `not tested: its tests didn't run` | The module that tests it didn't run, or no page's audit finished. |
| `not tested: W3C now treats it as always met for HTML; the owner decides how to state it` | 4.1.1 Parsing (WCAG 2.1 only). |
| `manual: <result> (<by>, <date>); the tests: <tests' result>` | You recorded a manual result. |

One problem counts once: a missing label reported by `a11y/label`, `a11y/placeholder-label` and `interact/unlabeled` on the same element is one finding. A finding on 214 pages is 1 element on 214 pages.

### The service layout

`--layout service` drafts the service information of EAA Annex V: `# Accessibility information for <owner>`, `## About the service` (from `statement.service`, else a `[TO DO]`), `## How the service meets the accessibility requirements` with the status `[TO DO]` and the problems, `## Accessibility features` as a `[TO DO]`, then how it was prepared, contact, enforcement, the appendix, not tested and pages checked as in the model layout.

```sh
rhythm soundcheck statement --layout service -o accessibility-information.md
```

```text
Statement draft written to accessibility-information.md
11 [TO DO] markers left
```

### Privacy in the draft

- The address the run checked and the test account's user name never appear; the draft uses `statement.website`.
- Paths lose their query string and fragment. Tokens after reset, verify, confirm, magic, login and activate are masked (`/reset/•••••/`), as are secrets.
- A logged-in run lists no paths unless `list_private_paths: true`.
- Control and bidi characters are removed from every format; Markdown special characters in your text are escaped (hence `https\://`).

## EARL for WCAG-EM

`--earl`, or `-o <file>.jsonld`, writes the JSON-LD evaluation that the [W3C WCAG-EM Report Tool](https://www.w3.org/WAI/eval/report-tool/) saves and opens. An evaluator imports it and carries on by hand with the criteria that need a person.

- Scope: the website from `statement.website`, the owner as title, conformance target AA, the run's WCAG version.
- Sample: every checked page (none for a logged-in run).
- One assertion per criterion: `earl:failed` for failures found, `earl:cantTell` for no failures found by these tests (Soundcheck never writes `earl:passed`) and `earl:untested` for the rest. A manual entry maps the same way.
- Test mode: `earl:semiAuto` when a keyboard check tests the criterion, `earl:automatic` otherwise, `earl:manual` for your manual entries. Asserted by `Rhythm Soundcheck <version>`.

## JSON

`statement --json` (schema `rhythm-soundcheck/statement/v1`) has the top-level keys `schema`, `tool`, `version`, `date`, `layout`, `wcag`, `en301549`, `standard`, `status` (always `null`), `evidence_note`, `todo`, `website`, `owner`, `pages`, `private_pages`, `checked`, `axe`, `axe_rules`, `own_checks`, `totals`, `criteria`, `known_issues`, `ignored`, `not_audited`, `frames`, `keyboard`, `notes` and `limits`. From the walk-through, after the trap was fixed:

```json
"totals": {"criteria": 50, "failed": 1, "not_tested": 23}
```

```json
{
 "id": "2.1.2",
 "title": "No Keyboard Trap",
 "level": "A",
 "principle": "Operable",
 "en301549": "9.2.1.2",
 "coverage": "partly",
 "tested_by": [
  "keyboard/trap"
 ],
 "how_tested": "partly: keyboard/trap",
 "result": "no_failures_found",
 "tests": "no_failures_found",
 "text": "no failures found by these tests",
 "issues": 0,
 "pages": 0,
 "paths": [],
 "manual": null
}
```

`result` is `failures_found`, `no_failures_found`, `not_tested` or `manual`. Lists are `[]`, never `null`.

## The evidence pack

Every `--eaa` run and snapshot writes `eaa/evidence.json` (schema `rhythm-soundcheck/evidence/v1`) in its folder:

```text
.soundcheck/runs/2026-10-28T19-28-47.087Z/
  run.json  pages.jsonl  comparison.json  report.html
  keyboard/<page id>.json              the keyboard traces
  keyflows/<flow>.txt                  the key flows' key presses
  eaa/
    evidence.json                      the pack
    evidence.sig  evidence.pub         after evidence sign
    statement-draft.md                 written again on every --eaa run
    written.json                       SHA-256 of drafts written with -o
    focus/<page id>-<n>.png            focus ring crops
```

The pack records the Soundcheck, Chrome and axe-core versions (with axe's SHA-256), the run's options, start and end, whether it was interrupted or made with `-k`, each page's audit status per module (paths masked as in the draft), the `eaa` summary and the SHA-256 of `run.json`, the pages file, every keyboard trace and every crop. Each pack names the previous `--eaa` run or snapshot of the same site and its pack's SHA-256, so the packs form a chain. Runs with a pack are never pruned with the old runs, since that would break the chain.

### evidence verify

```text
rhythm soundcheck evidence verify [run folder|snapshot] [--pub <key>]
rhythm soundcheck evidence sign [run folder|snapshot] --key <file>
rhythm soundcheck evidence keygen -o <file>
```

`verify` checks every file of the run and every earlier pack in the chain. Without an argument it checks the pack that records the latest start:

```text
$ rhythm soundcheck evidence verify
Verified .soundcheck/runs/2026-10-28T19-28-47.087Z/eaa/evidence.json: every file matches and the chain is whole
not signed
```

A keyboard trace changed after the run, by one space added at its end:

```text
$ rhythm soundcheck evidence verify
rhythm soundcheck: evidence: keyboard/01cd74fa62.json changed after it was recorded (sha256 differs)
```

Other failures name the file or the gap: `evidence: <file> was deleted after it was recorded`, `evidence: <file> was added after the pack was recorded`, `evidence: the run before <id> is missing from the chain` (a run folder deleted by hand), `evidence: eaa/evidence.sig doesn't match eaa/evidence.json (signature invalid)`.

| Code | Meaning |
|---|---|
| `0` | Verified or signed. |
| `1` | Verification failed; the message names the file or the gap. |
| `2` | Usage error. |
| `3` | Can't read: no `--eaa` results, no pack, or a recorded file that can't be read. |

### Signing

Make a key once. The private key is written readable by you only and never replaces an existing file:

```text
$ rhythm soundcheck evidence keygen -o soundcheck.key
public key: 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=
Private key written to soundcheck.key; keep it out of git
```

Sign a run's pack with it. `sign` writes `eaa/evidence.sig` (an Ed25519 signature of `evidence.json`) and `eaa/evidence.pub`:

```text
$ rhythm soundcheck evidence sign --key soundcheck.key
Signed .soundcheck/runs/2026-10-28T19-28-47.087Z/eaa/evidence.json
```

`verify` then prints the key the pack was signed with:

```text
$ rhythm soundcheck evidence verify
Verified .soundcheck/runs/2026-10-28T19-28-47.087Z/eaa/evidence.json: every file matches and the chain is whole
signed with key 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=; check it against the key the owner published
```

Anyone who changes a pack can sign it again with their own key. So publish your public key. Whoever checks a pack compares the key `verify` prints with it, or passes it with `--pub`:

```text
$ rhythm soundcheck evidence verify --pub 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=
Verified .soundcheck/runs/2026-10-28T19-28-47.087Z/eaa/evidence.json: every file matches and the chain is whole
signed with key 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=, the --pub key

$ rhythm soundcheck evidence verify --pub 8m9Gqva2DKXcrW/7XlBKBaHiNVKQo90VT7KBwfyasm4=
rhythm soundcheck: .soundcheck/runs/2026-10-28T19-28-47.087Z/eaa/evidence.json was signed with key 3NzvAzkbmfHGS8qUv2ibF9YIvW2vBiHJC7UZ+tcLeWE=, not the --pub key
```

The second exits 1, as does an unsigned pack with `--pub` (`... isn't signed, so it can't match --pub`). Other refusals:

```text
rhythm soundcheck: evidence sign needs --key <file>; make one with rhythm soundcheck evidence keygen -o <file>
rhythm soundcheck: can't sign a run made with -k: TLS wasn't verified
rhythm soundcheck: soundcheck.key already exists; keygen never replaces a key, so name a new file
rhythm soundcheck: --pub wants the base64 public key evidence keygen printed
```

To sign with your identity through Sigstore instead of a key file, run `cosign sign-blob eaa/evidence.json` in the run folder.

A signed evidence pack proves who produced it and that it wasn't changed afterwards. It doesn't prove the site was in that state for every visitor.

### Kept packs

Later runs are chained to a snapshot's pack as it was, so a snapshot's pack is kept before anything changes it. `accept` into an `--eaa` snapshot copies the pack, its signature and every file it lists to `.soundcheck/evidence/<sha256>/`, then records the pack again for the accepted state and drops the old signature (sign again):

```text
$ rhythm soundcheck accept da3cdeac
Accepted da3cdeac Keyboard focus is trapped (/a11y/consent.php)
Snapshot "before" now holds 1 accepted change. Run rhythm soundcheck compare to check again.
note: accept changed run.json, the pages file and snapshot.json of snapshot "before", so its evidence pack was recorded again. The pack from before is kept in .soundcheck/evidence/3579a49bdaf3b118bb9af32aa90182e12a094f50ec490c1416b352d39bc7c478 with its files: evidence verify of later --eaa runs, which are chained to it, checks that copy and notes it
```

`verify` checks the kept copy and says so:

```text
$ rhythm soundcheck evidence verify
note: snapshots/before changed by accept after runs/2026-10-28T19-28-47.087Z was chained to it; checked the copy kept in evidence/3579a49bdaf3b118bb9af32aa90182e12a094f50ec490c1416b352d39bc7c478
Verified .soundcheck/runs/2026-10-28T19-29-55.841Z/eaa/evidence.json: every file matches and the chain is whole
not signed

$ rhythm soundcheck evidence verify before
note: snapshots/before was recorded again after accept on 2026-10-28; the pack it replaced is kept in evidence/3579a49bdaf3b118bb9af32aa90182e12a094f50ec490c1416b352d39bc7c478
Verified .soundcheck/snapshots/before/eaa/evidence.json: every file matches and the chain is whole
not signed
```

Taking a snapshot again under the same name, or deleting it, keeps its pack the same way:

```text
$ rhythm soundcheck snapshots delete before
Deleted snapshot before
note: snapshot "before" had an evidence pack; it is kept in .soundcheck/evidence/3579a49bdaf3b118bb9af32aa90182e12a094f50ec490c1416b352d39bc7c478, so evidence verify of later --eaa runs still checks their chain
```

Keep `.soundcheck/evidence/` with the runs you hand over. A run folder deleted by hand still leaves a gap in the chain.

---
<sub>← [Keyboard walkthrough](Soundcheck-Keyboard-Walkthrough) · [Home](Home) · [CI](Soundcheck-CI) →</sub>
