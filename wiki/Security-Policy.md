If you find a security vulnerability in `rhythm`, Tuner, Soundcheck or any Rhythm tool, please report it privately so it can be fixed before it's public. This page explains how, what to include, and what happens next.

> [!WARNING]
> Please don't report vulnerabilities in public issues, pull requests or discussions. A public report puts every user at risk until a fix ships.

## How to report

Every repository has GitHub's private vulnerability reporting turned on. Open a private report on the repository the issue belongs to:

| Component | Report privately |
|---|---|
| Tuner (`rhythm-tuner`, its checks, check packs, vulnerability matching, reports) | https://github.com/RhythmShahriar/tuner/security/advisories/new |
| Soundcheck (`rhythm-soundcheck`, its browser handling, reports, flows, the GitHub Action) | https://github.com/RhythmShahriar/soundcheck/security/advisories/new |
| `rhythm` (the CLI, `install`, `update`, the install scripts, the shared kit) | https://github.com/RhythmShahriar/rhythm/security/advisories/new |

You can also reach the form from the repository's **Security** tab with the **Report a vulnerability** button. You need a GitHub account. The report is visible only to you and the maintainers.

Not sure which repository? Pick the closest one and say so in the report.

## What counts

Report it privately if it's a weakness in Rhythm's own code or release process, for example:

- A way to make `rhythm update`, `rhythm install` or the installers accept a binary that doesn't match the release checksums.
- A way for a check pack, a scanned site or vulnerability data to make Tuner run code, read or write files outside its cache, or send requests it shouldn't.
- A secret printed unmasked in any output format.
- A path in a local scan that changes files or runs site code.
- A way to make Soundcheck submit a form, click a control the destructive blocklist should skip, or run the leak test on a URL listed as production.
- A test account's credentials, or a flow's `${env:…}` value, written unmasked to a report, a log or a file.
- A way for a checked site to make Soundcheck read or write files outside its `.soundcheck` folder and cache, or reach the browser beyond the pages it checks.

Not a vulnerability in Rhythm:

- **A finding Tuner reports on your site**, or a change Soundcheck reports. That's the tool working; see Tuner's [Reading results](Tuner-Reading-Results) or Soundcheck's [Reading results](Soundcheck-Reading-Results).
- **A false positive or a missed issue.** Open a normal [Tuner issue](https://github.com/RhythmShahriar/tuner/issues) or [Soundcheck issue](https://github.com/RhythmShahriar/soundcheck/issues), unless it reveals something exploitable in the tool itself.
- **A vulnerability in WordPress, a plugin, a theme or PHP.** Report it to that project. If Tuner's data about it is wrong, that comes from [WPVulnerability](https://www.wpvulnerability.com/), wordpress.org or php.net.

## What to include

A good report lets the issue be reproduced quickly:

- The affected tool and version (`rhythm version`, `rhythm tuner --version`, `rhythm soundcheck --version`) and your OS and CPU.
- What the problem is and what an attacker could do with it.
- Steps to reproduce: the exact command, and a minimal input such as a check file, a server response or a test page. Use your own test setup, never a third-party site.
- Any logs or output, with real secrets removed.
- Whether you'd like to be credited in the advisory, and under what name.

## What to expect

- The report stays private between you and the maintainers while it's investigated and fixed. You can discuss it in the advisory's thread.
- A fix ships in a normal release, which users get through `rhythm update`. The advisory can then be published on the repository, with credit if you want it.
- Please keep the details private until the advisory is published.

There's no fixed response-time promise. Reports are handled as quickly as their severity calls for.

## Testing safely

When you research Rhythm's security, test against your own machines and your own test sites. Don't scan, or point Tuner or Soundcheck at, third-party sites to prove a point. The [Responsible Use](Responsible-Use) rules apply to security research too.

---
<sub>← [Responsible use](Responsible-Use) · [Home](Home) · [Contributing](Contributing) →</sub>
