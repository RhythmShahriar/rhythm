# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Installs rhythm and its tools on Windows from GitHub Releases:
#   irm https://rythm.sh/install.ps1 | iex
#
# Environment:
#   RHYTHM_INSTALL_DIR   where to install (default %LOCALAPPDATA%\Programs\rhythm)
#   RHYTHM_VERSION       a specific version, e.g. 0.5.0 (default: latest)
#   RHYTHM_TOOLS         tools to install as well (default "tuner soundcheck echo"; set empty for none)
#   RHYTHM_GITHUB_API    GitHub API base, for mirrors (default https://api.github.com).
#                        It must be https; plain http is accepted only for 127.0.0.1, ::1 and localhost.
#   GITHUB_TOKEN         sent to api.github.com only, to lift the anonymous rate limit
#   RHYTHM_GITHUB_TOKEN  sent to the RHYTHM_GITHUB_API host, whatever it is; wins over GITHUB_TOKEN
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
    Copy-Item -Path (Join-Path $Tmp 'rhythm.exe') -Destination (Join-Path $Dir 'rhythm.exe') -Force
}
finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
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

$Tools = if ($null -ne $env:RHYTHM_TOOLS) { $env:RHYTHM_TOOLS } else { $DefaultTools }
foreach ($Tool in ($Tools -split '\s+' | Where-Object { $_ })) {
    & $Rhythm install $Tool --dir $Dir
    if ($LASTEXITCODE -ne 0) { throw "rhythm install: installing $Tool failed" }
}

if ($OnWindows) {
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not (($UserPath -split ';') -contains $Dir)) {
        [Environment]::SetEnvironmentVariable('Path', (($UserPath, $Dir) -join ';').TrimStart(';'), 'User')
        Write-Host "Added $Dir to your PATH. Open a new terminal to use rhythm."
    }
}
Write-Host 'Get started: rhythm tuner scan example.com · rhythm soundcheck snapshot example.com · rhythm echo up'
