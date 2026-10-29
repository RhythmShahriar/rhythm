If you find a security vulnerability in `rhythm`, Tuner, Soundcheck, Echo, Roadie, Sightread or any Rhythm tool, please report it privately so it can be fixed before it's public. This page explains how, what to include and what happens next.

> [!WARNING]
> Please don't report vulnerabilities in public issues, pull requests or discussions. A public report puts every user at risk until a fix ships.

## How to report

Every repository has GitHub's private vulnerability reporting turned on. Open a private report on the repository the issue belongs to:

| Component | Report privately |
|---|---|
| Tuner (`rhythm-tuner`, its checks, check packs, vulnerability matching, reports) | https://github.com/RhythmShahriar/tuner/security/advisories/new |
| Soundcheck (`rhythm-soundcheck`, its browser handling, reports, flows, the GitHub Action) | https://github.com/RhythmShahriar/soundcheck/security/advisories/new |
| Echo (`rhythm-echo`, its catcher, proxy and CA, the WordPress, Laravel and Node connectors, replay, the GitHub Action) | https://github.com/RhythmShahriar/echo/security/advisories/new |
| Roadie (`rhythm-roadie`, its dump parser and allowlist, pseudonymisation, live key detection, kill switch, verify, signed reports, the remote command list, the GitHub Action) | https://github.com/RhythmShahriar/roadie/security/advisories/new |
| Sightread (`rhythm-sightread`, its fetching and verification, archive handling, analysers, reports, `--soundcheck`, the GitHub Action) | https://github.com/RhythmShahriar/sightread/security/advisories/new |
| `rhythm` (the CLI, `install`, `update`, the install scripts, the shared kit) | https://github.com/RhythmShahriar/rhythm/security/advisories/new |

You can also reach the form from the repository's **Security** tab with the **Report a vulnerability** button. You need a GitHub account. The report is visible only to you and the maintainers.

Not sure which repository? Pick the closest one and say so in the report.

## What counts

Report it privately if it's a weakness in Rhythm's own code or release process, for example:

- A way to make `rhythm update`, `rhythm install` or the installers accept a binary that doesn't match the release checksums, or a release whose Sigstore signature is missing, invalid or from another workflow, repository or tag.
- A way to make `rhythm` send `GITHUB_TOKEN` or `RHYTHM_GITHUB_TOKEN` to a host it shouldn't, or install from a bundle or `rhythm.lock` something other than what they pin.
- A way for a check pack, a scanned site or vulnerability data to make Tuner run code, read or write files outside its cache, or send requests it shouldn't.
- A secret printed unmasked in any output format.
- A path in a local scan that changes files or runs site code.
- A way to make Soundcheck submit a form, click a control the destructive blocklist should skip, or run the leak test on a URL listed as production.
- A way to make the keyboard walk or a dry key flow (`keyboard: true`) send data, or a key flow with `keyboard: send` submit on a URL listed as production.
- A way to change an `--eaa` run's evidence pack so that `evidence verify --pub <key>` still passes.
- A test account's credentials, or a flow's `${env:…}` value, written unmasked to a report, a log or a file.
- A way for a checked site to make Soundcheck read or write files outside its `.soundcheck` folder and cache, or reach the browser beyond the pages it checks.
- A way for a message to leave a site Echo has sealed through a hook its connector covers, or for a sealed site to get a success answer to a live-mode payment call.
- A way to make `rhythm echo replay` send to a production or public address it should refuse, or with a live key in reach.
- A credential stored unmasked in a capture, a report or a log, or Echo's CA private key leaving its cache folder.
- A way to use Echo's API or proxy from another machine without the token, or to use the proxy as an open relay.
- A way to make Roadie write to a target that matches `production:`, is its source or says it is production, or run a command on a source host outside its remote command list.
- A dump that gets a statement past Roadie's allowlist into the staging database, or makes Roadie read or write a file outside its work and cache folders.
- A real value from production that reaches a log, a report, a plan or a terminal unmasked, or plain production data written to the local disk during a copy.
- A way to make `report check` accept a report that was changed after signing or signed by another key.
- A way to make Sightread run code from a package it reviews, write outside its cache or report folder from an archive, send a private package name or a token to a host it shouldn't, or fetch from a host outside its allowlist.
- A way to make Sightread report a tampered archive as verified, or an allow entry cover bytes it wasn't recorded for.

Not a vulnerability in Rhythm:

- **A finding Tuner reports on your site**, or a change Soundcheck or Echo reports. That's the tool working; see Tuner's [Reading results](Tuner-Reading-Results) or Soundcheck's [Reading results](Soundcheck-Reading-Results).
- **A false positive or a missed issue.** Open a normal [Tuner](https://github.com/RhythmShahriar/tuner/issues), [Soundcheck](https://github.com/RhythmShahriar/soundcheck/issues), [Echo](https://github.com/RhythmShahriar/echo/issues), [Roadie](https://github.com/RhythmShahriar/roadie/issues) or [Sightread](https://github.com/RhythmShahriar/sightread/issues) issue, unless it reveals something exploitable in the tool itself. A personal value Roadie didn't pseudonymise in a plugin's table it has no profile for is a normal issue (a profile request), as is malicious code Sightread doesn't flag, unless either reveals one of the problems above.
- **A vulnerability in WordPress, a plugin, a theme or PHP.** Report it to that project. If Tuner's data about it is wrong, that comes from [WPVulnerability](https://www.wpvulnerability.com/), wordpress.org or php.net.

## What to include

A good report lets the issue be reproduced quickly:

- The affected tool and version (`rhythm version`, `rhythm tuner --version`, `rhythm soundcheck --version`, `rhythm echo --version`, `rhythm roadie version`, `rhythm sightread version`) and your OS and CPU.
- What the problem is and what an attacker could do with it.
- Steps to reproduce: the exact command plus a minimal input such as a check file, a server response or a test page. Use your own test setup, never a third-party site.
- Any logs or output, with real secrets removed.
- Whether you'd like to be credited in the advisory and under what name.

## What to expect

- The report stays private between you and the maintainers while it's investigated and fixed. You can discuss it in the advisory's thread.
- A fix ships in a normal release, which users get through `rhythm update`. The advisory can then be published on the repository, with credit if you want it.
- Please keep the details private until the advisory is published.

There's no fixed response-time promise. Reports are handled as quickly as their severity calls for.

## Testing safely

When you research Rhythm's security, test against your own machines and your own test sites. Don't scan third-party sites or point Tuner, Soundcheck, Echo or Sightread's `--soundcheck` at them to prove a point. Don't replay webhooks to services you don't run. The [Responsible Use](Responsible-Use) rules apply to security research too.

---
<sub>← [Responsible use](Responsible-Use) · [Home](Home) · [Contributing](Contributing) →</sub>
