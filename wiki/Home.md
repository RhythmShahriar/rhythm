<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-dark.svg">
    <img alt="rhythm" src="https://raw.githubusercontent.com/RhythmShahriar/rhythm/master/docs/assets/banner-light.svg" width="100%">
  </picture>
</p>

Welcome to the documentation for **rhythm**, one CLI for every Rhythm tool, and for **Tuner**, the WordPress &amp; PHP security scanner that runs on it. Everything lives here; the tools' READMEs link back to these pages.

## Start in one minute

| macOS · Linux | Windows (PowerShell) |
|---|---|
| `curl -fsSL https://rythm.sh/install \| sh` | `irm https://rythm.sh/install.ps1 \| iex` |

```sh
rhythm tuner scan yoursite.com
```

New here? Follow **[Your first scan](Tuner-First-Scan)**, then **[Reading results](Tuner-Reading-Results)**.

## Find your way

<table>
<tr>
<td width="33%" valign="top">

### Get started
- [Install](Install)
- [Updating](Updating)
- [Uninstall](Uninstall)
- [Verifying releases](Verifying-Releases)

</td>
<td width="33%" valign="top">

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
<td width="33%" valign="top">

### Go deeper
- [Known vulnerabilities](Tuner-Known-Vulnerabilities)
- [PHP advisories](Tuner-PHP-Advisories)
- [CI and code scanning](Tuner-CI)
- [Writing checks](Tuner-Writing-Checks)
- [Check packs](Tuner-Check-Packs)
- [Finding reference](Tuner-Finding-Reference)
- [CLI reference](Tuner-CLI-Reference)
- [FAQ and troubleshooting](Tuner-FAQ)

</td>
</tr>
</table>

## Current PHP advisories

> [!IMPORTANT]
> **PHP2Shell** (CVE-2026-109618, CVE-2026-109619) and **CVE-2026-115560** (session upload progress) affect PHP itself, so every WordPress and PHP site is in scope. See **[PHP advisories](Tuner-PHP-Advisories)** for what to check and how Tuner covers each one.

## The project

| | |
|---|---|
| [Responsible use](Responsible-Use) | scan only sites you own or may test; what Tuner never does |
| [Security policy](Security-Policy) | report a vulnerability in Rhythm or Tuner privately |
| [Contributing](Contributing) | set up, commit conventions, pull requests |
| [Release process](Release-Process) | how releases and check packs ship |
| [Architecture](Architecture) | how rhythm, the kit and the tools fit together |

Repositories: [rhythm](https://github.com/RhythmShahriar/rhythm) · [tuner](https://github.com/RhythmShahriar/tuner) · [community files](https://github.com/RhythmShahriar/.github)
