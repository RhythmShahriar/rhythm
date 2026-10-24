<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-dark.svg">
    <img alt="rhythm" src="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-light.svg" width="100%">
  </picture>
</p>

Welcome to the documentation for **rhythm** (one CLI for every Rhythm tool) and for the three tools that run on it: **Tuner**, the WordPress &amp; PHP security scanner; **Soundcheck**, which checks a whole site before and after a change and shows what broke; and **Echo**, which captures everything a site sends, seals staging so nothing reaches real people and shows what each form or order sends before and after a change. Everything lives here; the tools' READMEs link back to these pages.

## The Rhythm set: safe, works, says

| Tool | Asks | Looks at | Start with |
|---|---|---|---|
| 🔒 [Tuner](Tuner) | Is my site **safe**? | what an attacker can find: leaks, known vulnerabilities, weak config | [Your first scan](Tuner-First-Scan) |
| ✅ [Soundcheck](Soundcheck) | Does it still **work**? | what visitors see and do: errors, layout, forms, cache, speed | [Your first check](Soundcheck-First-Check) |
| 📣 [Echo](Echo) | Is it still **saying** what it should? | everything the site sends: email, SMS, webhooks, chat, analytics, payments | [Your first run](Echo-First-Run) |

They work on the same change: Tuner says a plugin needs updating, Soundcheck shows what the update did to the pages and Echo shows what it did to the messages. Run Soundcheck with `--echo` and it submits the real forms and compares what each one sent.

## Start in one minute

| macOS · Linux | Windows (PowerShell) |
|---|---|
| `curl -fsSL https://rythm.sh/install \| sh` | `irm https://rythm.sh/install.ps1 \| iex` |

```sh
rhythm tuner scan yoursite.com                    # what's out of tune: leaks, CVEs, weak config
rhythm soundcheck snapshot staging.yoursite.com   # before a change: record how every page works
rhythm soundcheck compare                         # after it: what broke
rhythm echo up                                    # seal staging: nothing the site sends reaches anyone
rhythm echo compare                               # after a change: what the site no longer sends
```

In CI, `uses: RhythmShahriar/rhythm/setup@v0.7.0` does the same on GitHub Actions ([CI setup](CI-Setup)). Every release is signature checked before it is installed ([Verifying releases](Verifying-Releases)). `rhythm.lock` pins a project's versions for every laptop and pipeline ([Pinning and locking](Pinning-And-Locking)).

New here? For security, follow **[Your first scan](Tuner-First-Scan)**. Before your next plugin update or deploy, follow **[Your first check](Soundcheck-First-Check)**. To seal a staging site and check its emails, SMS and webhooks, follow **[Your first run](Echo-First-Run)**.

## Find your way

<table>
<tr>
<td width="25%" valign="top">

### Get started
- [Install](Install)
- [Updating](Updating)
- [Uninstall](Uninstall)
- [Verifying releases](Verifying-Releases)

### rhythm in depth
- [Command reference](Command-Reference)
- [Pinning and locking](Pinning-And-Locking)
- [Offline installs](Offline-Installs)
- [CI setup](CI-Setup)
- [Extensions](Extensions)

</td>
<td width="25%" valign="top">

### Use Tuner
- [Overview](Tuner)
- [Your first scan](Tuner-First-Scan)
- [Reading results](Tuner-Reading-Results)
- [Choosing checks](Tuner-Choosing-Checks)
- [Requests and politeness](Tuner-Requests)
- [Output and reports](Tuner-Output)
- [Batch scans](Tuner-Batch-Scans)
- [Local scans](Tuner-Local-Scans)

</td>
<td width="25%" valign="top">

### Tuner in depth
- [Known vulnerabilities](Tuner-Known-Vulnerabilities)
- [PHP advisories](Tuner-PHP-Advisories)
- [CI and code scanning](Tuner-CI)
- [Writing checks](Tuner-Writing-Checks)
- [Check packs](Tuner-Check-Packs)
- [Finding reference](Tuner-Finding-Reference)
- [CLI reference](Tuner-CLI-Reference)
- [FAQ and troubleshooting](Tuner-FAQ)

</td>
<td width="25%" valign="top">

### Use Soundcheck
- [Overview](Soundcheck)
- [Your first check](Soundcheck-First-Check)
- [Reading results](Soundcheck-Reading-Results)
- [Discovery](Soundcheck-Discovery)
- [Page health](Soundcheck-Page-Health)
- [Visual changes](Soundcheck-Visual)
- [Interactions](Soundcheck-Interactions)
- [Forms and flows](Soundcheck-Forms-And-Flows)
- [Cache](Soundcheck-Cache)
- [Speed](Soundcheck-Speed)
- [CI](Soundcheck-CI)
- [Configuration](Soundcheck-Configuration)
- [CLI reference](Soundcheck-CLI-Reference)
- [FAQ and troubleshooting](Soundcheck-FAQ)
- [Benchmark](Soundcheck-Benchmark)

</td>
</tr>
<tr>
<td width="25%" valign="top">

### Use Echo
- [Overview](Echo)
- [Your first run](Echo-First-Run)
- [Connectors](Echo-Install-Connectors)
- [Sealing staging](Echo-Sealing-Staging)
- [Messages and checks](Echo-Messages-And-Checks)
- [Actions and compare](Echo-Actions-And-Compare)
- [With Soundcheck](Echo-With-Soundcheck)
- [Replay](Echo-Replay)
- [Deliverability](Echo-Deliverability)

</td>
<td width="25%" valign="top">

### Echo in depth
- [CI](Echo-CI)
- [Configuration](Echo-Configuration)
- [API](Echo-API)
- [CLI reference](Echo-CLI-Reference)
- [FAQ and troubleshooting](Echo-FAQ)
- [Benchmark](Echo-Benchmark)

</td>
<td width="25%" valign="top"></td>
<td width="25%" valign="top"></td>
</tr>
</table>

## Current PHP advisories

> [!IMPORTANT]
> **PHP2Shell** (CVE-2026-109618, CVE-2026-109619) and **CVE-2026-115560** (session upload progress) affect PHP itself, so every WordPress and PHP site is in scope. See **[PHP advisories](Tuner-PHP-Advisories)** for what to check and how Tuner covers each one.

## The project

| | |
|---|---|
| [Responsible use](Responsible-Use) | check only sites you own or may test; what the tools never do; personal data in Echo's captures |
| [Security policy](Security-Policy) | report a vulnerability in Rhythm, Tuner, Soundcheck or Echo privately |
| [Contributing](Contributing) | set up, commit conventions, pull requests |
| [Release process](Release-Process) | how releases, their signatures, attestations, SBOMs and packages, check packs and Echo's npm and Composer packages ship |
| [Architecture](Architecture) | how rhythm, the kit and the tools fit together |
| [Command reference](Command-Reference) | every `rhythm` command, flag, exit code, JSON document and environment variable |

Repositories: [rhythm](https://github.com/RhythmShahriar/rhythm) · [tuner](https://github.com/RhythmShahriar/tuner) · [soundcheck](https://github.com/RhythmShahriar/soundcheck) · [echo](https://github.com/RhythmShahriar/echo) · [community files](https://github.com/RhythmShahriar/.github)
