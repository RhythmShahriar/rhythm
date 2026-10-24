# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Tests scripts/install.ps1 against a fake GitHub API on 127.0.0.1:
#   pwsh scripts/test/run.ps1 [-Shell powershell] [case ...]
#
# Runs every case or only the named ones (e.g. "old_router") and stops at
# the first failure with exit code 1. Each case runs install.ps1 the way
# `irm | iex` does, in a new pwsh (or Windows PowerShell 5.1 with -Shell
# powershell) with the RHYTHM_* and GITHUB_* variables set for that case.
# It needs go to build the rhythm.exe stand-in from stub/ and python3 (or
# python on Windows) to serve the fake API with server.py. It runs in
# Windows PowerShell 5.1 as well as pwsh, on Windows, Linux and macOS.
#
# Environment:
#   RHYTHM_TEST_PORT  port for the fake API (default: any free port)
#   RHYTHM_TEST_KEEP  set to 1 to keep the temp folder for a look afterwards
[CmdletBinding(PositionalBinding = $false)]
param(
    [string]$Shell = '',
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Cases = @()
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Here = $PSScriptRoot
$Root = (Resolve-Path (Join-Path $Here '../..')).Path
$Installer = (Resolve-Path (Join-Path $Here '../install.ps1')).Path
$OnWindows = $PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows
if (-not $Shell) { $Shell = if ($PSVersionTable.PSEdition -eq 'Desktop') { 'powershell' } else { 'pwsh' } }
$Exe = if ($OnWindows) { '.exe' } else { '' }
$Work = Join-Path ([IO.Path]::GetTempPath()) ("rhythm-ps1-" + [Guid]::NewGuid().ToString('N').Substring(0, 8))
$Srv = Join-Path $Work 'srv'
$RequestLog = Join-Path $Work 'requests.log'
$Utf8 = New-Object Text.UTF8Encoding($false) # Windows PowerShell 5.1 writes a BOM otherwise
New-Item -ItemType Directory -Force -Path $Srv | Out-Null

$script:Case = ''
$script:CaseDir = ''
$script:Code = 0
$script:Out = ''

function Fail([string]$Why) {
    Write-Host "FAIL $($script:Case): $Why"
    Write-Host '--- installer output'
    Write-Host $script:Out
    throw "case $($script:Case) failed"
}

# New-Stub builds the rhythm.exe stand-in for one fake release.
function New-Stub([string]$Version, [string]$Tools, [string]$DryRun, [string]$Block, [string]$Out) {
    $Flags = "-X main.version=$Version -X main.tools=$Tools -X main.dryRun=$DryRun -X main.toolsBlock=$Block"
    $env:GOWORK = 'off'
    & go -C $Root build -o $Out -ldflags $Flags ./scripts/test/stub
    if ($LASTEXITCODE -ne 0) { throw "go build of the stub failed" }
}

# New-Zip zips rhythm.exe, marking it executable for pwsh on Linux and
# macOS, whose Expand-Archive restores the mode.
function New-Zip([string]$Binary, [string]$Zip) {
    Add-Type -AssemblyName System.IO.Compression
    $File = [IO.File]::Open($Zip, [IO.FileMode]::Create)
    $Archive = New-Object IO.Compression.ZipArchive($File, [IO.Compression.ZipArchiveMode]::Create)
    try {
        $Entry = $Archive.CreateEntry('rhythm.exe')
        if (-not $OnWindows) { $Entry.ExternalAttributes = -2115174400 } # 0x81ED0000: a regular file, mode 0755
        $Stream = $Entry.Open()
        $Bytes = [IO.File]::ReadAllBytes($Binary)
        $Stream.Write($Bytes, 0, $Bytes.Length)
        $Stream.Dispose()
    }
    finally {
        $Archive.Dispose()
        $File.Dispose()
    }
}

# New-Release writes the zips, checksums.txt and release JSON for a version
# whose downloads start with $Base. $Broken breaks the checksums on purpose:
# 'badsum' writes a wrong sum, 'nosums' leaves checksums.txt out.
function New-Release([string]$Version, [string]$Binary, [string]$Base, [string]$Broken = '') {
    $Out = Join-Path $Srv "dl/rhythm/v$Version"
    $Tags = Join-Path $Srv 'repos/RhythmShahriar/rhythm/releases/tags'
    New-Item -ItemType Directory -Force -Path $Out, $Tags | Out-Null
    $Sums = ''
    $Assets = @()
    foreach ($Arch in 'amd64', 'arm64') {
        $Name = "rhythm_windows_$Arch.zip"
        New-Zip $Binary (Join-Path $Out $Name)
        $Hash = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $Out $Name)).Hash.ToLower()
        if ($Broken -eq 'badsum') { $Hash = -join ($Hash.ToCharArray() | ForEach-Object { if ($_ -eq '0') { '1' } else { '0' } }) }
        $Sums += "$Hash  $Name`n"
        $Assets += "{`"name`":`"$Name`",`"browser_download_url`":`"$Base/$Name`"}"
    }
    if ($Broken -ne 'nosums') {
        $Assets += "{`"name`":`"checksums.txt`",`"browser_download_url`":`"$Base/checksums.txt`"}"
        [IO.File]::WriteAllText((Join-Path $Out 'checksums.txt'), $Sums, $Utf8)
    }
    $Json = "{`"tag_name`":`"v$Version`",`"prerelease`":false,`"assets`":[$($Assets -join ',')]}"
    [IO.File]::WriteAllText((Join-Path $Tags "v$Version"), $Json, $Utf8)
}

# The same releases as mkfake.sh builds for install.sh.
$All = 'tuner:t,soundcheck:sc,echo:e'
$Build = Join-Path $Work 'build'
foreach ($R in @(
        @('0.6.0', $All, 'yes', 'yes'),
        @('0.5.0', $All, 'no', 'yes'),
        @('0.4.0', 'tuner:t,soundcheck:sc', 'no', 'yes'),
        @('0.3.5', $All, 'no', 'no'),
        @('9.0.1', $All, 'yes', 'yes'),
        @('9.0.2', $All, 'yes', 'yes', 'badsum'),
        @('9.0.3', $All, 'yes', 'yes', 'nosums'))) {
    $Binary = Join-Path $Build "$($R[0])/rhythm$Exe"
    New-Stub $R[0] $R[1] $R[2] $R[3] $Binary
    $Base = if ($R[0] -eq '9.0.1') { 'http://example.invalid/dl/rhythm/v9.0.1' } else { "{{BASE}}/dl/rhythm/v$($R[0])" }
    New-Release $R[0] $Binary $Base $(if ($R.Count -gt 4) { $R[4] } else { '' })
}
Copy-Item (Join-Path $Srv 'repos/RhythmShahriar/rhythm/releases/tags/v0.6.0') (Join-Path $Srv 'repos/RhythmShahriar/rhythm/releases/latest')

$Python = if ($OnWindows) { 'python' } else { 'python3' }
$Port = if ($env:RHYTHM_TEST_PORT) { $env:RHYTHM_TEST_PORT } else { '0' }
$Server = Start-Process -FilePath $Python -PassThru -NoNewWindow -ArgumentList @(
    '-I', "`"$(Join-Path $Here 'server.py')`"", "`"$Srv`"", "`"$RequestLog`"", $Port)
$PortFile = Join-Path $Srv '.port'
for ($i = 0; -not (Test-Path $PortFile); $i++) {
    if ($i -gt 100 -or $Server.HasExited) { throw "the fake API didn't start" }
    Start-Sleep -Milliseconds 100
}
$BaseUrl = "http://127.0.0.1:$((Get-Content $PortFile).Trim())"

# The variables every case sets or clears; Invoke-Install restores them.
$Vars = 'RHYTHM_GITHUB_API', 'RHYTHM_INSTALL_DIR', 'RHYTHM_VERSION', 'RHYTHM_TOOLS', 'RHYTHM_ARCH',
'RHYTHM_TEST_LOG', 'GITHUB_TOKEN', 'RHYTHM_GITHUB_TOKEN', 'MOCK_LOG', 'TEMP', 'TMP', 'TMPDIR'

# Set-Var sets an environment variable, or deletes it for $null. PowerShell
# would pass $null to .NET as '', which .NET 9 and later keep as a value.
function Set-Var([string]$Name, $Value) {
    if ($null -eq $Value) { [Environment]::SetEnvironmentVariable($Name, [NullString]::Value) }
    else { [Environment]::SetEnvironmentVariable($Name, [string]$Value) }
}

# Invoke-Install runs install.ps1 for the current case with the defaults
# below, overridden by $Set ($null clears a variable). $Prelude runs first
# in the same session, so a case can stand in its own Invoke-RestMethod.
# $Script runs instead of install.ps1, for the truncation case. It runs in
# the case folder, so a relative RHYTHM_INSTALL_DIR lands there.
function Invoke-Install([hashtable]$Set = @{}, [string]$Prelude = '', [string]$Script = $Installer) {
    $Tmp = Join-Path $script:CaseDir 'tmp'
    New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
    $Values = @{
        RHYTHM_GITHUB_API  = $BaseUrl
        RHYTHM_INSTALL_DIR = (Join-Path $script:CaseDir 'bin')
        RHYTHM_ARCH        = 'amd64'
        RHYTHM_TEST_LOG    = (Join-Path $script:CaseDir 'argv.log')
        MOCK_LOG           = (Join-Path $script:CaseDir 'mock.log')
        TEMP               = $Tmp
        TMP                = $Tmp
        TMPDIR             = $Tmp
    }
    foreach ($K in $Set.Keys) { $Values[$K] = $Set[$K] }
    $Saved = @{}
    foreach ($K in $Vars) {
        $Saved[$K] = [Environment]::GetEnvironmentVariable($K)
        Set-Var $K $Values[$K]
    }
    # A script file rather than -Command, because Windows PowerShell 5.1
    # mangles quotes in arguments to native programs.
    $Runner = Join-Path $script:CaseDir 'run-install.ps1'
    [IO.File]::WriteAllText($Runner, "Set-Location -LiteralPath '$($script:CaseDir)'`n$Prelude`niex ([IO.File]::ReadAllText('$Script'))`n", $Utf8)
    $Prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $Lines = & $Shell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Runner 2>&1 | ForEach-Object { "$_" }
        $script:Code = $LASTEXITCODE
        $script:Out = $Lines -join "`n"
    }
    finally {
        $ErrorActionPreference = $Prev
        foreach ($K in $Vars) { Set-Var $K $Saved[$K] }
    }
}

# A prelude that stands in for the web cmdlets: it logs the URL and the
# Authorization header, then fails as if offline.
$Mock = @'
function Invoke-RestMethod { param($Uri, $Headers, [switch]$UseBasicParsing) Add-Content -Path $env:MOCK_LOG -Value "$Uri auth=$($Headers.Authorization)"; throw 'offline' }
function Invoke-WebRequest { param($Uri, $OutFile, [switch]$UseBasicParsing) Add-Content -Path $env:MOCK_LOG -Value "$Uri"; throw 'offline' }
'@

function Get-Requests { if (Test-Path $RequestLog) { @(Get-Content $RequestLog) } else { @() } }
function Clear-Requests { [IO.File]::WriteAllText($RequestLog, '') }

function Assert-Code([int]$Want) { if ($script:Code -ne $Want) { Fail "exit code $($script:Code), want $Want" } }
function Assert-Out([string]$Text) { if (-not $script:Out.Contains($Text)) { Fail "output lacks: $Text" } }
function Assert-NoOut([string]$Text) { if ($script:Out.Contains($Text)) { Fail "output has: $Text" } }
function Get-Bin([string]$Name) { Join-Path (Join-Path $script:CaseDir 'bin') $Name }
function Assert-File([string]$Name) { if (-not (Test-Path (Get-Bin $Name))) { Fail "$Name wasn't installed" } }
function Assert-NoFile([string]$Name) { if (Test-Path (Get-Bin $Name)) { Fail "$Name was installed" } }
function Assert-Tools([string[]]$Want = @()) {
    foreach ($T in 'tuner', 'soundcheck', 'echo') {
        if ($Want -contains $T) { Assert-File "rhythm-$T$Exe" } else { Assert-NoFile "rhythm-$T$Exe" }
    }
}
function Get-Version { (& (Get-Bin 'rhythm.exe') --version) }

# Assert-Started checks the closing lines: one first step for each tool
# named, in order, or the "Install a tool" line for none. Never a real site.
function Assert-Started([string[]]$Want = @()) {
    Assert-NoOut 'example.com'
    Assert-Out 'Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use'
    if (-not $Want.Count) {
        Assert-Out 'Install a tool: rhythm install tuner'
        Assert-NoOut 'Get started'
        return
    }
    Assert-NoOut 'Install a tool'
    $Steps = @{ tuner = '  rhythm tuner scan ./your-site'; soundcheck = '  rhythm soundcheck snapshot http://localhost:8080'; echo = '  rhythm echo up' }
    $WantLines = ($Want | ForEach-Object { $Steps[$_] }) -join '|'
    $Lines = $script:Out -split "`n"
    $At = [Array]::IndexOf($Lines, 'Get started:')
    if ($At -lt 0) { Fail 'no Get started line' }
    $Got = ($Lines[($At + 1)..($Lines.Count - 1)] | Where-Object { $_ -like '  rhythm *' }) -join '|'
    if ($Got -ne $WantLines) { Fail "first steps are '$Got', want '$WantLines'" }
}

function Case-default {
    Invoke-Install
    Assert-Code 0
    Assert-File 'rhythm.exe'
    Assert-Tools 'tuner', 'soundcheck', 'echo'
    Assert-Out "Installed rhythm 0.6.0 to $(Join-Path $script:CaseDir 'bin')"
    if (-not (Select-String -Path (Join-Path $script:CaseDir 'argv.log') -SimpleMatch 'rhythm install --dry-run tuner' -Quiet)) {
        Fail "the router wasn't asked about tuner"
    }
    Assert-Started 'tuner', 'soundcheck', 'echo'
}

function Case-none {
    # Before PowerShell 7.5 setting a variable to '' deletes it, so 'none'
    # is the only way to ask for no tools there.
    $Values = @('none', 'NONE', ' None ')
    if ($PSVersionTable.PSVersion -ge [Version]'7.5' -and $Shell -eq 'pwsh') { $Values += '' }
    foreach ($V in $Values) {
        Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin') -ErrorAction SilentlyContinue
        Invoke-Install @{ RHYTHM_TOOLS = $V }
        Assert-Code 0
        Assert-File 'rhythm.exe'
        Assert-Tools
        Assert-Started
    }
}

function Case-some {
    Invoke-Install @{ RHYTHM_TOOLS = 'soundcheck  echo' }
    Assert-Code 0
    Assert-Tools 'soundcheck', 'echo'
    Assert-Started 'soundcheck', 'echo'

    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin')
    Invoke-Install @{ RHYTHM_TOOLS = 'tuner' }
    Assert-Code 0
    Assert-Tools 'tuner'
    Assert-Started 'tuner'

    # Aliases name the same tool, which gets one line.
    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin'), (Join-Path $script:CaseDir 'argv.log')
    Invoke-Install @{ RHYTHM_TOOLS = 'e t echo' }
    Assert-Code 0
    Assert-Tools 'tuner', 'echo'
    Assert-Started 'echo', 'tuner'
    if (@(Get-Content (Join-Path $script:CaseDir 'argv.log') | Where-Object { $_ -match '^rhythm install (e|echo) ' }).Count -ne 1) {
        Fail 'echo was installed more than once'
    }
}

# Assert-PathLines checks that the output mentions PATH exactly $Count
# times, and that the one mention is $Text.
function Assert-PathLines([int]$Count, [string]$Text = '') {
    $Lines = @($script:Out -split "`n" | Where-Object { $_.Contains('PATH') })
    if ($Lines.Count -ne $Count) { Fail "$($Lines.Count) lines mention PATH, want $Count" }
    if ($Text) { Assert-Out $Text }
}

function Case-path_note {
    # The router's own PATH notes never show: --quiet for a router that has
    # it, filtered for one that doesn't. On Windows the installer says once
    # that the user PATH changed.
    $Bin = Join-Path $script:CaseDir 'bin'
    $Log = Join-Path $script:CaseDir 'argv.log'
    Invoke-Install
    Assert-Code 0
    Assert-NoOut "isn't on your PATH"
    if (-not (Select-String -Path $Log -SimpleMatch "rhythm install echo --dir $Bin --quiet" -Quiet)) { Fail "the router wasn't told --quiet" }
    if ($OnWindows) { Assert-PathLines 1 "Added $Bin to your user PATH. Open a new terminal to use rhythm." } else { Assert-PathLines 0 }
    if ($OnWindows) {
        Invoke-Install
        Assert-Code 0
        Assert-PathLines 1 "$Bin is on your user PATH. Open a new terminal to use rhythm."
    }

    Remove-Item -LiteralPath $Log
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0' }
    Assert-Code 0
    Assert-Tools 'tuner', 'soundcheck', 'echo'
    Assert-NoOut "isn't on your PATH"
    if (Select-String -Path $Log -SimpleMatch -Pattern '--quiet' -Quiet) { Fail '--quiet went to a router without it' }
    Assert-PathLines $(if ($OnWindows) { 1 } else { 0 })
}

function Case-bad_checksum {
    Invoke-Install @{ RHYTHM_VERSION = '9.0.2' }
    Assert-Code 1
    Assert-Out 'rhythm install: checksum mismatch for rhythm_windows_amd64.zip; nothing was installed'
    if (Test-Path (Join-Path $script:CaseDir 'bin')) { Fail 'a wrong checksum created the install folder' }
}

function Case-no_checksums {
    Clear-Requests
    Invoke-Install @{ RHYTHM_VERSION = '9.0.3' }
    Assert-Code 1
    Assert-Out 'rhythm install: the release has no checksums.txt; refusing to install unverified binaries'
    if (Test-Path (Join-Path $script:CaseDir 'bin')) { Fail 'a release without checksums created the install folder' }
    if (Get-Requests | Where-Object { $_.Contains('/dl/') }) { Fail 'the installer downloaded from a release without checksums' }
}

function Case-missing_dir {
    $Nested = Join-Path $script:CaseDir 'bin/a/b'
    Invoke-Install @{ RHYTHM_INSTALL_DIR = $Nested; RHYTHM_TOOLS = 'tuner' }
    Assert-Code 0
    if (-not (Test-Path (Join-Path $Nested 'rhythm.exe'))) { Fail "rhythm.exe wasn't installed into a new nested folder" }
    if (-not (Test-Path (Join-Path $Nested "rhythm-tuner$Exe"))) { Fail "tuner wasn't installed into a new nested folder" }
}

function Case-relative_dir {
    Invoke-Install @{ RHYTHM_INSTALL_DIR = 'bin'; RHYTHM_TOOLS = 'tuner' }
    Assert-Code 0
    Assert-File 'rhythm.exe'
    Assert-Tools 'tuner'
    $Bin = Join-Path $script:CaseDir 'bin'
    Assert-Out "Installed rhythm 0.6.0 to $Bin"
    if (-not (Select-String -Path (Join-Path $script:CaseDir 'argv.log') -SimpleMatch "rhythm install tuner --dir $Bin --quiet" -Quiet)) {
        Fail "rhythm wasn't given the absolute folder"
    }
}

function Case-truncated {
    # A cut-off download must not run at all: iex refuses the whole script.
    $Text = [IO.File]::ReadAllText($Installer)
    foreach ($Pct in 50, 90) {
        $Cut = Join-Path $script:CaseDir "install-$Pct.ps1"
        [IO.File]::WriteAllText($Cut, $Text.Substring(0, [int]($Text.Length * $Pct / 100)), $Utf8)
        Clear-Requests
        Invoke-Install @{} '' $Cut
        if (Test-Path (Join-Path $script:CaseDir 'bin')) { Fail "a script cut at $Pct% created the install folder" }
        if (Get-Requests) { Fail "a script cut at $Pct% made a request" }
    }
}

function Case-http_refused {
    foreach ($Api in 'http://example.invalid', 'HTTP://10.0.0.1:8080', 'ftp://127.0.0.1', 'example.invalid') {
        Remove-Item (Join-Path $script:CaseDir 'mock.log') -ErrorAction SilentlyContinue
        Invoke-Install @{ RHYTHM_GITHUB_API = $Api } $Mock
        Assert-Code 1
        Assert-Out 'rhythm install: RHYTHM_GITHUB_API must be'
        if (Test-Path (Join-Path $script:CaseDir 'mock.log')) { Fail "a request was made for $Api" }
    }
}

function Case-http_download_refused {
    Invoke-Install @{ RHYTHM_VERSION = '9.0.1' }
    Assert-Code 1
    Assert-Out 'refusing to download over http: http://example.invalid/dl/rhythm/v9.0.1/'
    Assert-NoFile 'rhythm.exe'
}

function Case-github_token {
    # api.github.com gets GITHUB_TOKEN.
    Invoke-Install @{ RHYTHM_GITHUB_API = $null; GITHUB_TOKEN = 'gh-test-secret' } $Mock
    Assert-Code 1
    $Log = Get-Content (Join-Path $script:CaseDir 'mock.log')
    if ($Log -notcontains 'https://api.github.com/repos/RhythmShahriar/rhythm/releases/latest auth=Bearer gh-test-secret') {
        Fail "GITHUB_TOKEN wasn't sent to api.github.com: $Log"
    }
    Assert-NoOut 'gh-test-secret'

    # A mirror never sees GITHUB_TOKEN.
    Clear-Requests
    Invoke-Install @{ GITHUB_TOKEN = 'gh-test-secret'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    if (-not (Get-Requests | Where-Object { $_ -match 'releases/latest auth=-$' })) { Fail "the mirror wasn't asked for the release" }
    if (Get-Requests | Where-Object { $_.Contains('gh-test-secret') }) { Fail 'GITHUB_TOKEN reached the mirror' }
}

function Case-rhythm_token {
    Clear-Requests
    Invoke-Install @{ GITHUB_TOKEN = 'gh-test-secret'; RHYTHM_GITHUB_TOKEN = 'mirror-test-secret'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    if (-not (Get-Requests | Where-Object { $_ -match 'releases/latest auth=Bearer mirror-test-secret$' })) {
        Fail "RHYTHM_GITHUB_TOKEN wasn't sent to the mirror"
    }
    if (Get-Requests | Where-Object { $_.Contains('gh-test-secret') }) { Fail 'GITHUB_TOKEN reached the mirror' }
    if (Get-Requests | Where-Object { $_.Contains('/dl/') -and $_.Contains('mirror-test-secret') }) { Fail 'the token went to a download' }
    Assert-NoOut 'mirror-test-secret'
}

function Case-rate_limit {
    foreach ($Status in 403, 429) {
        Invoke-Install @{ RHYTHM_GITHUB_API = "$BaseUrl/status/$Status" }
        Assert-Code 1
        Assert-Out 'rhythm install: GitHub rate limit reached; set GITHUB_TOKEN'
        Assert-NoOut 'API rate limit exceeded'
    }
    Invoke-Install @{ RHYTHM_GITHUB_API = "$BaseUrl/status/500" }
    Assert-Code 1
    Assert-Out "couldn't fetch the release from GitHub ($BaseUrl/status/500/repos/RhythmShahriar/rhythm/releases/latest): HTTP 500"
}

function Case-old_router {
    Invoke-Install @{ RHYTHM_VERSION = '0.4.0' }
    Assert-Code 0
    Assert-Out "warning: rhythm 0.4.0 doesn't know echo; skipped"
    Assert-Tools 'tuner', 'soundcheck'
    Assert-Started 'tuner', 'soundcheck'
    if (Select-String -Path (Join-Path $script:CaseDir 'argv.log') -SimpleMatch 'install echo' -Quiet) {
        Fail 'the router was asked to install echo'
    }
}

function Case-old_router_named {
    Invoke-Install @{ RHYTHM_VERSION = 'v0.4.0'; RHYTHM_TOOLS = 'tuner echo' }
    Assert-Code 1
    Assert-Out "rhythm install: rhythm 0.4.0 doesn't know echo; run '$(Get-Bin 'rhythm.exe') help' to see the tools"
    Assert-File 'rhythm.exe'
    Assert-Tools
}

function Case-router_without_dry_run {
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0' }
    Assert-Code 0
    Assert-Tools 'tuner', 'soundcheck', 'echo'
    Assert-NoOut 'warning'
}

function Case-router_without_tools_block {
    Invoke-Install @{ RHYTHM_VERSION = '0.3.5' }
    Assert-Code 0
    Assert-Tools 'tuner', 'soundcheck', 'echo'
}

function Case-unknown_tool {
    Invoke-Install @{ RHYTHM_TOOLS = 'tuner bogus' }
    Assert-Code 1
    Assert-Out 'rhythm: unknown tool "bogus"'
    Assert-Out "rhythm install: rhythm 0.6.0 doesn't know bogus"
    Assert-File 'rhythm.exe'
    Assert-Tools
}

function Case-running_exe {
    # Windows can't overwrite a running rhythm.exe; the install must end with
    # the new one in place all the same.
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    $Held = Start-Process -FilePath (Get-Bin 'rhythm.exe') -ArgumentList 'hold' -PassThru
    try {
        Start-Sleep -Milliseconds 300
        if ($Held.HasExited) { Fail "the held rhythm.exe didn't keep running" }
        Invoke-Install @{ RHYTHM_TOOLS = 'none' }
        Assert-Code 0
        if ((Get-Version) -ne 'rhythm 0.6.0') { Fail "rhythm.exe is $(Get-Version), want rhythm 0.6.0" }
        if (Get-ChildItem (Join-Path $script:CaseDir 'bin') -Filter 'rhythm.exe.new-*') { Fail 'a rhythm.exe.new-* file was left behind' }
    }
    finally {
        Stop-Process -Id $Held.Id -Force -ErrorAction SilentlyContinue
    }
}

# Preludes that make Move-Item fail the way Windows does with a running
# rhythm.exe, so every platform runs install.ps1's fallback swap.
$Locked = @'
function Move-Item {
    [CmdletBinding()] param([string]$LiteralPath, [string]$Destination, [switch]$Force)
    if ($Force) { throw 'the file is in use' }
    Microsoft.PowerShell.Management\Move-Item -LiteralPath $LiteralPath -Destination $Destination
}
'@
$Stuck = @'
function Move-Item {
    [CmdletBinding()] param([string]$LiteralPath, [string]$Destination, [switch]$Force)
    if ($Force -or $LiteralPath -like '*.new-*') { throw 'the file is in use' }
    Microsoft.PowerShell.Management\Move-Item -LiteralPath $LiteralPath -Destination $Destination
}
'@

function Case-swap_fallback {
    # When rhythm.exe can't be overwritten, it is renamed away and the new
    # one moves in.
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    Invoke-Install @{ RHYTHM_TOOLS = 'none' } $Locked
    Assert-Code 0
    if ((Get-Version) -ne 'rhythm 0.6.0') { Fail "rhythm.exe is $(Get-Version), want rhythm 0.6.0" }
    $Left = @(Get-ChildItem (Join-Path $script:CaseDir 'bin') | Where-Object { $_.Name -like 'rhythm.exe.*' } | ForEach-Object { $_.Name })
    if ($Left.Count) { Fail "the swap left $($Left -join ' ') behind" }
}

function Case-swap_restore {
    # When the new rhythm.exe can't move in either, the old one comes back.
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    Invoke-Install @{ RHYTHM_TOOLS = 'none' } $Stuck
    Assert-Code 1
    Assert-Out "rhythm install: can't replace $(Get-Bin 'rhythm.exe'); the old rhythm is still there"
    if ((Get-Version) -ne 'rhythm 0.5.0') { Fail "rhythm.exe is $(Get-Version), want the old rhythm 0.5.0" }
    $Left = @(Get-ChildItem (Join-Path $script:CaseDir 'bin') | Where-Object { $_.Name -like 'rhythm.exe.*' } | ForEach-Object { $_.Name })
    if ($Left.Count) { Fail "the swap left $($Left -join ' ') behind" }
}

function Case-readonly_dir {
    # The install fails with a clear error and keeps the old rhythm.exe.
    # Windows has no read-only folders; root ignores the mode.
    if ($OnWindows -or ((& id -u) -eq '0')) { Write-Host "skip $($script:Case)"; return }
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0'; RHYTHM_TOOLS = 'none' }
    Assert-Code 0
    $Bin = Join-Path $script:CaseDir 'bin'
    & chmod 555 $Bin
    try {
        Invoke-Install @{ RHYTHM_TOOLS = 'none' }
        Assert-Code 1
        Assert-Out "rhythm install: can't write to $Bin"
        if ((Get-Version) -ne 'rhythm 0.5.0') { Fail "rhythm.exe is $(Get-Version), want the old rhythm 0.5.0" }
    }
    finally {
        & chmod 755 $Bin
    }
}

if (-not $Cases.Count) {
    $Cases = 'default', 'none', 'some', 'path_note', 'bad_checksum', 'no_checksums', 'missing_dir', 'relative_dir', 'truncated', 'http_refused', 'http_download_refused', 'github_token', 'rhythm_token',
    'rate_limit', 'old_router', 'old_router_named', 'router_without_dry_run', 'router_without_tools_block',
    'unknown_tool', 'running_exe', 'swap_fallback', 'swap_restore', 'readonly_dir'
}
# Installing on Windows adds the folder to the user PATH; put it back after.
$UserPath = if ($OnWindows) { [Environment]::GetEnvironmentVariable('Path', 'User') } else { $null }
$Failed = $false
try {
    foreach ($C in $Cases) {
        $script:Case = $C
        $script:CaseDir = Join-Path $Work "cases/$C"
        $script:Out = ''
        New-Item -ItemType Directory -Force -Path $script:CaseDir | Out-Null
        & "Case-$C"
        Write-Host "ok   $C"
    }
    Write-Host "all installer cases passed with $Shell"
}
catch {
    Write-Host $_
    $Failed = $true
}
finally {
    if ($OnWindows) { [Environment]::SetEnvironmentVariable('Path', $UserPath, 'User') }
    Stop-Process -Id $Server.Id -Force -ErrorAction SilentlyContinue
    if ($env:RHYTHM_TEST_KEEP -eq '1') { Write-Host "kept $Work" } else { Remove-Item -Recurse -Force $Work -ErrorAction SilentlyContinue }
}
if ($Failed) { exit 1 }
