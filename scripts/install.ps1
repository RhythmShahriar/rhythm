# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Installs rhythm and its tools on Windows from GitHub Releases:
#   irm https://rythm.sh/install.ps1 | iex
#
# Environment:
#   RHYTHM_INSTALL_DIR   where to install (default %LOCALAPPDATA%\Programs\rhythm)
#   RHYTHM_VERSION       a specific version, e.g. 0.5.0 (default: latest)
#   RHYTHM_TOOLS         tools to install as well (default "tuner soundcheck echo"; "none" for none)
#   RHYTHM_GITHUB_API    GitHub API base, for mirrors (default https://api.github.com).
#                        It must be https; plain http is accepted only for 127.0.0.1, ::1 and localhost.
#   GITHUB_TOKEN         sent to api.github.com only, to lift the anonymous rate limit
#   RHYTHM_GITHUB_TOKEN  sent to the RHYTHM_GITHUB_API host, whatever it is; wins over GITHUB_TOKEN
#
# Everything is in one script block, closed on the last line, so a download
# cut off partway is a parse error instead of a partial install. The block
# also keeps its variables and functions out of the session that ran iex.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is far slower with the progress bar
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $Repo = 'RhythmShahriar/rhythm'
    $DefaultTools = 'tuner soundcheck echo'
    $Api = if ($env:RHYTHM_GITHUB_API) { $env:RHYTHM_GITHUB_API.TrimEnd('/') } else { 'https://api.github.com' }
    $Dir = if ($env:RHYTHM_INSTALL_DIR) { $env:RHYTHM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\rhythm' }
    # Windows PowerShell 5.1 has no $IsWindows; it only runs on Windows.
    $OnWindows = $PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows

    # Test-Loopback reports whether a URL points at this machine. Only these
    # hosts may use plain http, so the installer tests can run a local server.
    function Test-Loopback([Uri]$Uri) {
        @('127.0.0.1', '::1', 'localhost') -contains $Uri.Host.Trim('[', ']').ToLowerInvariant()
    }

    # Assert-Https refuses a download URL that isn't https. Plain http passes
    # only when both the API base and the URL are on loopback.
    function Assert-Https([string]$Url) {
        $Uri = $null
        if ([Uri]::TryCreate($Url, [UriKind]::Absolute, [ref]$Uri)) {
            if ($Uri.Scheme -eq 'https') { return }
            if ($Uri.Scheme -eq 'http' -and $ApiIsLoopback -and (Test-Loopback $Uri)) { return }
        }
        throw "rhythm install: refusing to download over http: $Url"
    }

    $ApiUri = $null
    if (-not [Uri]::TryCreate($Api, [UriKind]::Absolute, [ref]$ApiUri) -or ($ApiUri.Scheme -ne 'https' -and $ApiUri.Scheme -ne 'http')) {
        throw "rhythm install: RHYTHM_GITHUB_API must be an https URL: $Api"
    }
    $ApiIsLoopback = Test-Loopback $ApiUri
    if ($ApiUri.Scheme -eq 'http' -and -not $ApiIsLoopback) {
        throw "rhythm install: RHYTHM_GITHUB_API must be https (http only for 127.0.0.1, ::1 and localhost): $Api"
    }

    $Headers = @{ Accept = 'application/vnd.github+json'; 'User-Agent' = 'rhythm-install' }
    # GITHUB_TOKEN belongs to github.com, so it never goes to a mirror.
    if ($env:RHYTHM_GITHUB_TOKEN) {
        $Headers.Authorization = "Bearer $env:RHYTHM_GITHUB_TOKEN"
    }
    elseif ($env:GITHUB_TOKEN -and $ApiUri.Scheme -eq 'https' -and $ApiUri.Host -eq 'api.github.com') {
        $Headers.Authorization = "Bearer $env:GITHUB_TOKEN"
    }

    $Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { if ($env:RHYTHM_ARCH) { $env:RHYTHM_ARCH } else { throw "rhythm install: unsupported CPU '$env:PROCESSOR_ARCHITECTURE'" } }
    }

    $ReleaseUrl = if ($env:RHYTHM_VERSION) { "$Api/repos/$Repo/releases/tags/v$($env:RHYTHM_VERSION.TrimStart('v'))" } else { "$Api/repos/$Repo/releases/latest" }
    try {
        $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers -UseBasicParsing
    }
    catch {
        $Status = 0
        if ($_.Exception.Response) { $Status = [int]$_.Exception.Response.StatusCode }
        if ($Status -eq 403 -or $Status -eq 429) { throw 'rhythm install: GitHub rate limit reached; set GITHUB_TOKEN' }
        if ($Status) { throw "rhythm install: couldn't fetch the release from GitHub ($ReleaseUrl): HTTP $Status" }
        throw "rhythm install: couldn't reach $($ApiUri.Host) ($ReleaseUrl): $($_.Exception.Message)"
    }

    $Name = "rhythm_windows_$Arch.zip"
    $Archive = $Release.assets | Where-Object { $_.name -eq $Name }
    $Sums = $Release.assets | Where-Object { $_.name -eq 'checksums.txt' }
    if (-not $Archive) { throw "rhythm install: the release has no build for windows/$Arch" }
    if (-not $Sums) { throw 'rhythm install: the release has no checksums.txt; refusing to install unverified binaries' }
    Assert-Https $Archive.browser_download_url
    Assert-Https $Sums.browser_download_url

    # Install-Rhythm puts the new rhythm.exe in place without a moment where
    # there is none. It copies to a temp name in the same folder and moves that
    # over the old file. A running rhythm.exe can't be overwritten but can be
    # renamed, so if the move fails the old file goes to rhythm.exe.old first
    # and comes back if the new one still can't move in. kit/update swaps the
    # same way.
    function Install-Rhythm([string]$Source, [string]$Dest) {
        $New = "$Dest.new-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
        try {
            Copy-Item -LiteralPath $Source -Destination $New -Force
        }
        catch {
            Remove-Item -LiteralPath $New -Force -ErrorAction SilentlyContinue
            throw "rhythm install: can't write to $(Split-Path $Dest); set RHYTHM_INSTALL_DIR to a folder you own"
        }
        try {
            Move-Item -LiteralPath $New -Destination $Dest -Force
            return
        }
        catch {
            $Why = $_.Exception.Message
        }
        if (-not (Test-Path -LiteralPath $Dest)) {
            # Nothing to rename away: a fresh install, or -Force removed the old
            # file before the move failed. One more plain move.
            try {
                Move-Item -LiteralPath $New -Destination $Dest
                return
            }
            catch {
                Remove-Item -LiteralPath $New -Force -ErrorAction SilentlyContinue
                throw "rhythm install: can't create $Dest ($Why)"
            }
        }
        # A previous swap may have left a rhythm.exe.old that is still running.
        $Old = "$Dest.old"
        Remove-Item -LiteralPath $Old -Force -ErrorAction SilentlyContinue
        if (Test-Path -LiteralPath $Old) { $Old = "$Dest.old-$([Guid]::NewGuid().ToString('N').Substring(0, 8))" }
        try {
            Move-Item -LiteralPath $Dest -Destination $Old
        }
        catch {
            Remove-Item -LiteralPath $New -Force -ErrorAction SilentlyContinue
            throw "rhythm install: can't replace $Dest; close any running rhythm and try again ($($_.Exception.Message))"
        }
        try {
            Move-Item -LiteralPath $New -Destination $Dest
        }
        catch {
            $Why = $_.Exception.Message
            Move-Item -LiteralPath $Old -Destination $Dest -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath $New -Force -ErrorAction SilentlyContinue
            throw "rhythm install: can't replace $Dest; the old rhythm is still there ($Why)"
        }
        Remove-Item -LiteralPath $Old -Force -ErrorAction SilentlyContinue
    }

    $Tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $Tmp | Out-Null
    try {
        $Zip = Join-Path $Tmp $Name
        $SumsFile = Join-Path $Tmp 'checksums.txt'
        Invoke-WebRequest -Uri $Archive.browser_download_url -OutFile $Zip -UseBasicParsing
        Invoke-WebRequest -Uri $Sums.browser_download_url -OutFile $SumsFile -UseBasicParsing

        $Line = Get-Content $SumsFile | Where-Object { $_ -match ('^[0-9a-f]{64}\s+\*?' + [regex]::Escape($Name) + '$') } | Select-Object -First 1
        $Want = if ($Line) { ($Line -split '\s+')[0].ToLower() } else { '' }
        $Got = (Get-FileHash -Algorithm SHA256 -Path $Zip).Hash.ToLower()
        if (-not $Want -or $Want -ne $Got) { throw "rhythm install: checksum mismatch for $Name; nothing was installed" }

        Expand-Archive -Path $Zip -DestinationPath $Tmp -Force
        New-Item -ItemType Directory -Force -Path $Dir | Out-Null
        # A relative folder would be wrong in PATH and in what rhythm is told.
        $Dir = (Resolve-Path -LiteralPath $Dir).ProviderPath
        Install-Rhythm (Join-Path $Tmp 'rhythm.exe') (Join-Path $Dir 'rhythm.exe')
    }
    finally {
        Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
    }

    # Invoke-Native runs rhythm.exe and returns its exit code and its output,
    # stderr included. Windows PowerShell 5.1 turns redirected stderr into
    # errors, which 'Stop' would throw, so this lowers the preference first.
    function Invoke-Native([string[]]$Arguments) {
        $Prev = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $Out = & $Rhythm @Arguments 2>&1 | ForEach-Object { "$_" }
            @{ Code = $LASTEXITCODE; Out = ($Out -join "`n") }
        }
        finally {
            $ErrorActionPreference = $Prev
        }
    }

    # Test-RouterKnows reports whether the installed rhythm knows a tool. Newer
    # routers answer `rhythm install --dry-run <tool>`. Older ones reject the
    # flag, so the tool list comes from the "Tools:" block of `rhythm help`, or
    # from $DefaultTools when even that is missing.
    $Probe = @{ Mode = ''; Known = @(); Out = '' }
    function Test-RouterKnows([string]$Tool) {
        if ($Probe.Mode -ne 'help') {
            $R = Invoke-Native @('install', '--dry-run', $Tool)
            $Probe.Out = $R.Out
            if ($R.Code -eq 0) { $Probe.Mode = 'dry-run'; return $true }
            if ($Probe.Mode -eq 'dry-run' -or $R.Out -notmatch 'flag provided but not defined') {
                $Probe.Mode = 'dry-run'
                return $false
            }
            $Probe.Mode = 'help'
            $Probe.Out = ''
            # Rows look like "  soundcheck (sc)  did my change break anything?".
            $In = $false
            $List = @()
            foreach ($Row in ((Invoke-Native @('help')).Out -split "`n")) {
                if ($Row -match '^Tools:') { $In = $true; continue }
                if (-not $In) { continue }
                if ($Row -match '^\s*$') { break }
                $Words = @($Row.Trim() -split '\s+')
                $List += $Words[0]
                if ($Words.Count -gt 1 -and $Words[1] -match '^\((.+)\)$') { $List += $Matches[1] }
            }
            $Probe.Known = if ($List.Count) { $List } else { @($DefaultTools -split ' ') }
        }
        return $Probe.Known -contains $Tool
    }

    $Rhythm = Join-Path $Dir 'rhythm.exe'
    # Off Windows (pwsh on Linux or macOS, mostly the installer tests) the
    # Windows rhythm.exe can't run, so the script stops after copying it.
    $Installed = $null
    if ($OnWindows) {
        $Installed = & $Rhythm --version
        if ($LASTEXITCODE -ne 0) { throw "rhythm install: the installed $Rhythm doesn't run" }
    }
    else {
        try { $Installed = & $Rhythm --version 2>$null } catch { $Installed = $null }
        if ($LASTEXITCODE -ne 0) { $Installed = $null }
    }
    if (-not $Installed) {
        Write-Host "Installed rhythm.exe to $Dir"
        return
    }
    Write-Host "Installed $Installed to $Dir"

    # An unset RHYTHM_TOOLS means the defaults. Windows PowerShell 5.1 can't set
    # a variable to '' (that deletes it), so "none" is the way to ask for none
    # there. An old pinned rhythm may not know every default tool; those are
    # skipped. A name the user asked for must be known, or nothing more is
    # installed.
    $Explicit = $null -ne $env:RHYTHM_TOOLS
    $ToolList = if ($Explicit) { $env:RHYTHM_TOOLS } else { $DefaultTools }
    $Tools = @($ToolList -split '\s+' | Where-Object { $_ })
    if ($Tools.Count -eq 1 -and $Tools[0] -eq 'none') { $Tools = @() }
    $Todo = @()
    # Get-ToolName turns an alias or a rhythm-<tool> name into the tool's name.
    function Get-ToolName([string]$Tool) {
        $Name = $Tool -replace '^rhythm-', ''
        switch ($Name) { 't' { 'tuner' } 'sc' { 'soundcheck' } 'e' { 'echo' } default { $Name } }
    }
    $Names = @()
    foreach ($Tool in $Tools) {
        if (Test-RouterKnows $Tool) {
            # An alias and the name are one tool, installed once.
            $Name = Get-ToolName $Tool
            if ($Names -notcontains $Name) { $Names += $Name; $Todo += $Tool }
            continue
        }
        if ($Explicit) {
            if ($Probe.Out) { Write-Host $Probe.Out }
            throw "rhythm install: $Installed doesn't know $Tool; run '$Rhythm help' to see the tools"
        }
        Write-Host "warning: $Installed doesn't know $Tool; skipped"
    }
    # The tools that were installed, by name rather than alias, for the
    # closing lines.
    $Done = @()
    foreach ($Tool in $Todo) {
        & $Rhythm install $Tool --dir $Dir
        if ($LASTEXITCODE -ne 0) { throw "rhythm install: installing $Tool failed" }
        $Done += Get-ToolName $Tool
    }

    if ($OnWindows) {
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if (-not (($UserPath -split ';') -contains $Dir)) {
            [Environment]::SetEnvironmentVariable('Path', (($UserPath, $Dir) -join ';').TrimStart(';'), 'User')
            Write-Host "Added $Dir to your PATH. Open a new terminal to use rhythm."
        }
    }
    # A first command for each tool installed. The targets are local, since
    # the tools are for sites you own.
    if ($Done.Count) {
        Write-Host 'Get started:'
        foreach ($Name in $Done) {
            $Step = switch ($Name) {
                'tuner' { 'rhythm tuner scan ./your-site' }
                'soundcheck' { 'rhythm soundcheck snapshot http://localhost:8080' }
                'echo' { 'rhythm echo up' }
                default { "rhythm $Name --help" }
            }
            Write-Host "  $Step"
        }
    }
    else {
        Write-Host 'Install a tool: rhythm install tuner'
    }
    Write-Host 'Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use'
}
