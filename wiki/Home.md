<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-dark.svg">
    <img alt="rhythm" src="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-light.svg" width="100%">
  </picture>
</p>

Welcome to the documentation for **rhythm**, one CLI for every Rhythm tool, and for the two tools that run on it: **Tuner**, the WordPress &amp; PHP security scanner, and **Soundcheck**, which checks a whole site before and after a change and shows what broke. Everything lives here; the tools' READMEs link back to these pages.

## Start in one minute

| macOS · Linux | Windows (PowerShell) |
|---|---|
| `curl -fsSL https://rythm.sh/install \| sh` | `irm https://rythm.sh/install.ps1 \| iex` |

```sh
rhythm tuner scan yoursite.com                    # what's out of tune: leaks, CVEs, weak config
rhythm soundcheck snapshot staging.yoursite.com   # before a change: record how every page works
rhythm soundcheck compare                         # after it: what broke
```

New here? For security, follow **[Your first scan](Tuner-First-Scan)**. Before your next plugin update or deploy, follow **[Your first check](Soundcheck-First-Check)**.

## Find your way

<table>
<tr>
<td width="25%" valign="top">

### Get started
- [Install](Install)
- [Updating](Updating)
- [Uninstall](Uninstall)
- [Verifying releases](Verifying-Releases)

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
</table>

## Current PHP advisories

> [!IMPORTANT]
> **PHP2Shell** (CVE-2026-109618, CVE-2026-109619) and **CVE-2026-115560** (session upload progress) affect PHP itself, so every WordPress and PHP site is in scope. See **[PHP advisories](Tuner-PHP-Advisories)** for what to check and how Tuner covers each one.

## The project

| | |
|---|---|
| [Responsible use](Responsible-Use) | check only sites you own or may test; what Tuner and Soundcheck never do |
| [Security policy](Security-Policy) | report a vulnerability in Rhythm, Tuner or Soundcheck privately |
| [Contributing](Contributing) | set up, commit conventions, pull requests |
| [Release process](Release-Process) | how releases and check packs ship |
| [Architecture](Architecture) | how rhythm, the kit and the tools fit together |

Repositories: [rhythm](https://github.com/RhythmShahriar/rhythm) · [tuner](https://github.com/RhythmShahriar/tuner) · [soundcheck](https://github.com/RhythmShahriar/soundcheck) · [community files](https://github.com/RhythmShahriar/.github)
