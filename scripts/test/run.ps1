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
function New-Stub([string]$Version, [string]$Tools, [string]$Features, [string]$Block, [string]$Out) {
    $Flags = "-X main.version=$Version -X main.tools=$Tools -X main.features=$Features -X main.toolsBlock=$Block"
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

# New-Release writes the zips, checksums.txt, a stand-in
# checksums.txt.sigstore.json and the release JSON for a version whose
# downloads start with $Base. $Broken breaks the checksums on purpose:
# 'badsum' writes a wrong sum, 'nosums' leaves checksums.txt out and
# 'nobundle' the bundle.
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
    if ($Broken -ne 'nosums' -and $Broken -ne 'nobundle') {
        # Only the fake cosign reads the bundle, so any bytes do.
        $Assets += "{`"name`":`"checksums.txt.sigstore.json`",`"browser_download_url`":`"$Base/checksums.txt.sigstore.json`"}"
        [IO.File]::WriteAllText((Join-Path $Out 'checksums.txt.sigstore.json'), '{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}', $Utf8)
    }
    $Json = "{`"tag_name`":`"v$Version`",`"prerelease`":false,`"assets`":[$($Assets -join ',')]}"
    [IO.File]::WriteAllText((Join-Path $Tags "v$Version"), $Json, $Utf8)
}

# The same releases as mkfake.sh builds for install.sh.
$All = 'tuner:t,soundcheck:sc,echo:e'
$New = 'dry-run,verify,defaults,pins,locked'
$Build = Join-Path $Work 'build'
foreach ($R in @(
        @('0.7.0', $All, $New, 'yes'),
        @('0.6.0', $All, 'dry-run', 'yes'),
        @('0.5.0', $All, '', 'yes'),
        @('0.4.0', 'tuner:t,soundcheck:sc', '', 'yes'),
        @('0.3.5', $All, '', 'no'),
        @('9.0.1', $All, $New, 'yes'),
        @('9.0.2', $All, $New, 'yes', 'badsum'),
        @('9.0.3', $All, $New, 'yes', 'nosums'),
        @('9.0.4', $All, $New, 'yes', 'nobundle'))) {
    $Binary = Join-Path $Build "$($R[0])/rhythm$Exe"
    New-Stub $R[0] $R[1] $R[2] $R[3] $Binary
    $Base = if ($R[0] -eq '9.0.1') { 'http://example.invalid/dl/rhythm/v9.0.1' } else { "{{BASE}}/dl/rhythm/v$($R[0])" }
    New-Release $R[0] $Binary $Base $(if ($R.Count -gt 4) { $R[4] } else { '' })
}
Copy-Item (Join-Path $Srv 'repos/RhythmShahriar/rhythm/releases/tags/v0.7.0') (Join-Path $Srv 'repos/RhythmShahriar/rhythm/releases/latest')
# The stand-in fetches each tool's archive before it "installs" the tool, so
# a case can take one away. Only the request matters, not the bytes.
foreach ($T in 'tuner', 'soundcheck', 'echo') {
    $Out = Join-Path $Srv "dl/$T/v0.1.0"
    New-Item -ItemType Directory -Force -Path $Out | Out-Null
    foreach ($Os in 'windows', 'linux', 'darwin') {
        foreach ($Arch in 'amd64', 'arm64') {
            $Ext = if ($Os -eq 'windows') { 'zip' } else { 'tar.gz' }
            [IO.File]::WriteAllText((Join-Path $Out "$($T)_$($Os)_$Arch.$Ext"), 'stand-in', $Utf8)
        }
    }
}

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

# Stand-ins for cosign and gh, each in its own folder so a case can have
# either. They log their argv to $FAKE_SIG_LOG. cosign exits
# $FAKE_COSIGN_EXIT; gh is logged in unless FAKE_GH_AUTH=1, has
# --source-ref unless FAKE_GH_OLD=1 and its attestation check exits 1 when
# FAKE_GH_EXIT=1.
$CosignBin = Join-Path $Work 'cosignbin'
$GhBin = Join-Path $Work 'ghbin'
New-Item -ItemType Directory -Force -Path $CosignBin, $GhBin | Out-Null
if ($OnWindows) {
    [IO.File]::WriteAllText((Join-Path $CosignBin 'cosign.cmd'), "@echo off`r`necho cosign %*>>`"%FAKE_SIG_LOG%`"`r`nif `"%FAKE_COSIGN_EXIT%`"==`"1`" (echo Error: none of the expected identities matched what was in the certificate 1>&2& exit /b 1)`r`nexit /b 0`r`n", $Utf8)
    [IO.File]::WriteAllText((Join-Path $GhBin 'gh.cmd'), "@echo off`r`nif `"%~1 %~2`"==`"auth status`" (if `"%FAKE_GH_AUTH%`"==`"1`" (exit /b 1) else (exit /b 0))`r`nif `"%~1 %~2 %~3`"==`"attestation verify --help`" (if `"%FAKE_GH_OLD%`"==`"1`" (exit /b 0) else (echo       --source-ref string& exit /b 0))`r`necho gh %*>>`"%FAKE_SIG_LOG%`"`r`nif `"%FAKE_GH_EXIT%`"==`"1`" (echo Error: no attestations found 1>&2& exit /b 1)`r`nexit /b 0`r`n", $Utf8)
}
else {
    [IO.File]::WriteAllText((Join-Path $CosignBin 'cosign'), @'
#!/bin/sh
printf 'cosign %s\n' "$*" >>"$FAKE_SIG_LOG"
[ "${FAKE_COSIGN_EXIT:-0}" = 0 ] || echo 'Error: none of the expected identities matched what was in the certificate' >&2
exit "${FAKE_COSIGN_EXIT:-0}"
'@, $Utf8)
    [IO.File]::WriteAllText((Join-Path $GhBin 'gh'), @'
#!/bin/sh
case "$*" in
"auth status") exit "${FAKE_GH_AUTH:-0}" ;;
"attestation verify --help") [ "${FAKE_GH_OLD:-}" = 1 ] || echo '      --source-ref string'; exit 0 ;;
esac
printf 'gh %s\n' "$*" >>"$FAKE_SIG_LOG"
[ "${FAKE_GH_EXIT:-0}" = 0 ] || echo 'Error: no attestations found' >&2
exit "${FAKE_GH_EXIT:-0}"
'@, $Utf8)
    & chmod 755 (Join-Path $CosignBin 'cosign') (Join-Path $GhBin 'gh')
}
# The PATH every case gets unless it sets one. A real cosign would check the
# stand-in bundle for real, so its folder is left out; the shell running the
# cases is found once, here.
$ShellExe = (Get-Command $Shell -CommandType Application | Select-Object -First 1).Source
$Sep = [IO.Path]::PathSeparator
$SysPath = (@($env:PATH -split $Sep | Where-Object { $_ -and -not (Test-Path (Join-Path $_ 'cosign*')) }) -join $Sep)
$NoGh = Join-Path $Work 'gh-config' # an empty gh config: not logged in
New-Item -ItemType Directory -Force -Path $NoGh | Out-Null

# The variables every case sets or clears; Invoke-Install restores them.
$Vars = 'RHYTHM_GITHUB_API', 'RHYTHM_INSTALL_DIR', 'RHYTHM_VERSION', 'RHYTHM_TOOLS', 'RHYTHM_ARCH',
'RHYTHM_TEST_LOG', 'GITHUB_TOKEN', 'RHYTHM_GITHUB_TOKEN', 'MOCK_LOG', 'TEMP', 'TMP', 'TMPDIR',
'PATH', 'GH_TOKEN', 'GH_CONFIG_DIR', 'RHYTHM_SKIP_SIGNATURE', 'RHYTHM_TEST_VERIFY',
'FAKE_SIG_LOG', 'FAKE_COSIGN_EXIT', 'FAKE_GH_AUTH', 'FAKE_GH_EXIT', 'FAKE_GH_OLD', 'RHYTHM_TEST_DEFAULTS', 'RHYTHM_LOCKED',
'RHYTHM_TEST_ALREADY', 'RHYTHM_INSTALL_TIMEOUT'

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
        FAKE_SIG_LOG       = (Join-Path $script:CaseDir 'sig.log')
        PATH               = $SysPath
        GH_CONFIG_DIR      = $NoGh
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
        $Lines = & $ShellExe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Runner 2>&1 | ForEach-Object { "$_" }
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
    Assert-Out "Installed rhythm 0.7.0 to $(Join-Path $script:CaseDir 'bin')"
    if ((Get-Content (Join-Path $script:CaseDir 'argv.log')) -notcontains 'rhythm install --defaults --dry-run') {
        Fail "the router wasn't asked for its defaults"
    }
    Assert-Started 'echo', 'soundcheck', 'tuner'
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

function Case-commas {
    # Commas separate names, as in rhythm's own lists.
    foreach ($V in 'tuner,echo', 'tuner, echo', ',tuner,,echo,', 'tuner,none echo') {
        Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin') -ErrorAction SilentlyContinue
        Invoke-Install @{ RHYTHM_TOOLS = $V }
        Assert-Code 0
        Assert-Tools 'tuner', 'echo'
        Assert-Started 'tuner', 'echo'
    }
}

function Case-typo {
    # Every name is checked before any tool is downloaded.
    Clear-Requests
    Invoke-Install @{ RHYTHM_TOOLS = 'tuner sondcheck' }
    Assert-Code 1
    Assert-Out 'rhythm: unknown tool "sondcheck" (did you mean soundcheck?)'
    Assert-Out '  known: echo, soundcheck, tuner'
    Assert-Out "rhythm install: rhythm 0.7.0 doesn't know sondcheck; run '$(Get-Bin 'rhythm.exe') help' to see the tools"
    Assert-File 'rhythm.exe'
    Assert-Tools
    if (Get-Requests | Where-Object { $_.Contains('/dl/tuner/') }) { Fail 'tuner was downloaded before the names were checked' }
    if ((Get-Content (Join-Path $script:CaseDir 'argv.log')) -notcontains 'rhythm install --dry-run tuner sondcheck') { Fail "the names weren't checked in one call" }

    Invoke-Install @{ RHYTHM_TOOLS = 'tunr,sondcheck,nope' }
    Assert-Code 1
    Assert-Out 'rhythm: unknown tool "tunr" (did you mean tuner?)'
    Assert-Out 'rhythm: unknown tool "nope"'
    Assert-Out "rhythm install: rhythm 0.7.0 doesn't know tunr, sondcheck, nope;"
}

function Case-tool_fails {
    # A tool whose archive is missing doesn't stop the next one.
    $Dl = Join-Path $Srv 'dl'
    Rename-Item (Join-Path $Dl 'soundcheck') 'soundcheck.off'
    try { Invoke-Install } finally { Rename-Item (Join-Path $Dl 'soundcheck.off') 'soundcheck' }
    Assert-Code 1
    Assert-Tools 'tuner', 'echo'
    Assert-Out 'rhythm-soundcheck  downloading soundcheck_'
    Assert-Out ': HTTP 404'
    Assert-Started 'echo', 'tuner'
    Assert-Out 'Installed: echo, tuner. Failed: soundcheck (see above).'

    Rename-Item (Join-Path $Dl 'tuner') 'tuner.off'
    try { Invoke-Install @{ RHYTHM_TOOLS = 'tuner' } } finally { Rename-Item (Join-Path $Dl 'tuner.off') 'tuner' }
    Assert-Code 1
    Assert-NoOut 'Install a tool'
    Assert-Out 'Installed: none. Failed: tuner (see above).'
}

# Assert-PathLines checks that the output mentions PATH exactly $Count
# times and that the one mention is $Text.
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

function Get-Argv { $F = Join-Path $script:CaseDir 'argv.log'; if (Test-Path $F) { @(Get-Content $F) } else { @() } }

function Case-defaults {
    # Unset RHYTHM_TOOLS takes the router's own list of defaults.
    $Log = Join-Path $script:CaseDir 'argv.log'
    Invoke-Install @{ RHYTHM_TEST_DEFAULTS = 'soundcheck echo' }
    Assert-Code 0
    if ((Get-Argv) -notcontains 'rhythm install --defaults --dry-run') { Fail "the router wasn't asked for its defaults" }
    Assert-Tools 'soundcheck', 'echo'
    Assert-Started 'soundcheck', 'echo'

    # A router without --defaults gets the built-in list, checked at once.
    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin'), $Log
    Invoke-Install @{ RHYTHM_VERSION = '0.6.0' }
    Assert-Code 0
    if ((Get-Argv) -notcontains 'rhythm install --defaults --dry-run') { Fail "the old router wasn't asked for its defaults" }
    if ((Get-Argv) -notcontains 'rhythm install --dry-run tuner soundcheck echo') { Fail "the built-in list wasn't checked in one call" }
    Assert-Tools 'tuner', 'soundcheck', 'echo'
    Assert-Started 'tuner', 'soundcheck', 'echo'

    # An explicit list never asks for the defaults.
    Remove-Item $Log
    Invoke-Install @{ RHYTHM_TOOLS = 'tuner' }
    Assert-Code 0
    if ((Get-Argv) -match '--defaults') { Fail 'the defaults were asked for with RHYTHM_TOOLS set' }
}

function Case-pins {
    # tool@version reaches rhythm as it was written.
    $Bin = Join-Path $script:CaseDir 'bin'
    Invoke-Install @{ RHYTHM_TOOLS = 'tuner@0.3.1,soundcheck' }
    Assert-Code 0
    if ((Get-Argv) -notcontains 'rhythm install --dry-run tuner@0.3.1 soundcheck') { Fail "the pin wasn't checked as written" }
    if ((Get-Argv) -notcontains "rhythm install tuner@0.3.1 --dir $Bin --quiet") { Fail "the pin didn't reach rhythm install" }
    Assert-Out 'Installed rhythm-tuner 0.3.1 '
    Assert-Tools 'tuner', 'soundcheck'
    Assert-Started 'tuner', 'soundcheck'

    # A pin and the plain name are one tool.
    Remove-Item -Recurse -Force $Bin, (Join-Path $script:CaseDir 'argv.log')
    Invoke-Install @{ RHYTHM_TOOLS = 't@0.3.1 tuner' }
    Assert-Code 0
    if (@(Get-Argv | Where-Object { $_ -match '^rhythm install t(uner)?[@ ]' }).Count -ne 1) { Fail 'tuner was installed more than once' }

    # A rhythm that can't pin says so before installing any tool.
    Remove-Item -Recurse -Force $Bin
    Invoke-Install @{ RHYTHM_VERSION = '0.6.0'; RHYTHM_TOOLS = 'tuner@0.3.1 echo' }
    Assert-Code 1
    Assert-Out "rhythm install: rhythm 0.6.0 doesn't know tuner@0.3.1; it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version"
    Assert-Tools
    Invoke-Install @{ RHYTHM_VERSION = '0.5.0'; RHYTHM_TOOLS = 'echo tuner@0.3.1' }
    Assert-Code 1
    Assert-Out "rhythm install: rhythm 0.5.0 doesn't know tuner@0.3.1; it can't pin tool versions"
    Assert-Tools
}

function Case-locked {
    $Bin = Join-Path $script:CaseDir 'bin'
    $Lock = Join-Path $script:CaseDir 'rhythm.lock'
    [IO.File]::WriteAllText($Lock, '{"schema":"rhythm-lock/v1","rhythm":{"name":"rhythm","repo":"RhythmShahriar/rhythm","version":"0.7.0","platforms":{}},"tools":[{"name":"tuner","repo":"RhythmShahriar/tuner","version":"0.3.1","platforms":{}},{"name":"echo","repo":"RhythmShahriar/echo","version":"0.1.0","platforms":{}}]}', $Utf8)
    Invoke-Install @{ RHYTHM_LOCKED = '1' }
    Assert-Code 0
    if ((Get-Argv) -notcontains "rhythm install --locked --dir $Bin --quiet") { Fail "rhythm wasn't asked to install --locked" }
    if ((Get-Argv) -match '--defaults') { Fail 'the defaults were asked for with RHYTHM_LOCKED=1' }
    Assert-Out 'Installed rhythm-tuner 0.3.1 '
    Assert-Tools 'tuner', 'echo'
    Assert-Started 'tuner', 'echo'

    # RHYTHM_TOOLS gives way to the lock.
    Invoke-Install @{ RHYTHM_LOCKED = '1'; RHYTHM_TOOLS = 'soundcheck' }
    Assert-Code 0
    Assert-Out 'warning: RHYTHM_TOOLS is ignored: RHYTHM_LOCKED installs the tools in rhythm.lock'
    Assert-Tools 'tuner', 'echo'

    # A tool the lock names that fails is in the summary.
    $Dl = Join-Path $Srv 'dl'
    Rename-Item (Join-Path $Dl 'echo') 'echo.off'
    try { Invoke-Install @{ RHYTHM_LOCKED = '1' } } finally { Rename-Item (Join-Path $Dl 'echo.off') 'echo' }
    Assert-Code 1
    Assert-Out 'Installed: tuner. Failed: echo (see above).'

    # A rhythm without --locked can't.
    Remove-Item -Recurse -Force $Bin
    Invoke-Install @{ RHYTHM_LOCKED = '1'; RHYTHM_VERSION = '0.6.0' }
    Assert-Code 1
    Assert-Out "rhythm install: rhythm 0.6.0 can't install from rhythm.lock; set RHYTHM_VERSION to a newer rhythm"
    Assert-Tools

    # RHYTHM_LOCKED=0 is off; with no lock RHYTHM_LOCKED=1 stops.
    Remove-Item (Join-Path $script:CaseDir 'argv.log')
    Invoke-Install @{ RHYTHM_LOCKED = '0'; RHYTHM_TOOLS = 'tuner' }
    Assert-Code 0
    if ((Get-Argv) -match '--locked') { Fail 'RHYTHM_LOCKED=0 used the lock' }
    Remove-Item $Lock
    Invoke-Install @{ RHYTHM_LOCKED = '1' }
    Assert-Code 1
    Assert-Out "rhythm install: RHYTHM_LOCKED is set but there is no rhythm.lock in $($script:CaseDir)"
}

function Get-SigLog { $F = Join-Path $script:CaseDir 'sig.log'; if (Test-Path $F) { @(Get-Content $F) } else { @() } }
function Clear-SigLog { Remove-Item (Join-Path $script:CaseDir 'sig.log') -ErrorAction SilentlyContinue }
function Assert-NoInstall([string]$Why) {
    $Bin = Join-Path $script:CaseDir 'bin'
    if ((Test-Path $Bin) -and @(Get-ChildItem $Bin).Count) { Fail "$Why installed $(@(Get-ChildItem $Bin | ForEach-Object { $_.Name }) -join ' ')" }
}

function Case-signature_cosign {
    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath" }
    Assert-Code 0
    Assert-Out 'Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (cosign)'
    Assert-NoOut 'signature not checked'
    Assert-Tools 'tuner', 'soundcheck', 'echo'
    # The same check README tells people to run, flag for flag.
    $Readme = [IO.File]::ReadAllText((Join-Path $Root 'README.md'))
    $Block = [regex]::Match($Readme, '(?s)cosign verify-blob .*?\n  checksums\.txt').Value
    $Flags = @([regex]::Matches($Block, '--[a-z-]+ [^ \\\r\n]+') | ForEach-Object { $_.Value -replace 'v<version>', 'v0.7.0' })
    if ($Flags.Count -ne 5) { Fail "README's cosign command has $($Flags.Count) flags, want 5" }
    $Log = (Get-SigLog) -join "`n"
    foreach ($Flag in $Flags) {
        if ($Flag -like '--bundle *') { $Flag = '--bundle ' }
        if (-not $Log.Contains($Flag)) { Fail "cosign wasn't given $Flag" }
    }
    if ($Log -notmatch '--bundle \S+checksums\.txt\.sigstore\.json ') { Fail "cosign wasn't given the bundle" }
    if ($Log -notmatch ' --certificate-github-workflow-repository RhythmShahriar/rhythm ') { Fail "the repository isn't pinned" }
    if ($Log -notmatch ' --certificate-github-workflow-ref refs/tags/v0\.7\.0 ') { Fail "the tag isn't pinned" }
    if ($Log -notmatch 'checksums\.txt\s*$') { Fail "cosign didn't check checksums.txt" }

    # A pinned version is the tag the signature must name.
    Clear-SigLog
    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath"; RHYTHM_VERSION = '0.6.0' }
    Assert-Code 0
    if (((Get-SigLog) -join "`n") -notmatch ' --certificate-github-workflow-ref refs/tags/v0\.6\.0 ') { Fail "the pinned tag isn't checked" }
}

function Case-signature_cosign_fails {
    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath"; FAKE_COSIGN_EXIT = '1' }
    Assert-Code 1
    Assert-Out 'Error: none of the expected identities matched'
    Assert-Out "rhythm install: cosign says checksums.txt wasn't signed for RhythmShahriar/rhythm v0.7.0; nothing was installed"
    Assert-NoInstall 'a failed signature'
}

function Case-signature_gh {
    Invoke-Install @{ PATH = "$GhBin$Sep$SysPath" }
    Assert-Code 0
    Assert-Out 'Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (gh attestation)'
    if (-not ((Get-SigLog) | Where-Object { $_ -match '^gh attestation verify \S*checksums\.txt --repo RhythmShahriar/rhythm --signer-workflow RhythmShahriar/\.github/\.github/workflows/release-go\.yml --source-ref refs/tags/v0\.7\.0\s*$' })) {
        Fail "gh wasn't asked to verify checksums.txt for the repository and tag: $(Get-SigLog)"
    }

    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin')
    Invoke-Install @{ PATH = "$GhBin$Sep$SysPath"; FAKE_GH_EXIT = '1' }
    Assert-Code 1
    Assert-Out 'rhythm install: gh found no attestation that RhythmShahriar/rhythm v0.7.0 made this checksums.txt; nothing was installed.'
    Assert-NoInstall 'a failed attestation'

    # gh without a login can't ask for attestations; one without
    # --source-ref can't pin the tag. The line says which.
    Clear-SigLog
    Invoke-Install @{ PATH = "$GhBin$Sep$SysPath"; FAKE_GH_AUTH = '1' }
    Assert-Code 0
    Assert-Out 'signature not checked here (gh is not logged in); rhythm will verify itself and every tool it installs'
    Assert-NoOut 'no cosign or gh'
    Assert-NoOut 'warning: signature'
    if ((Get-SigLog).Count) { Fail 'gh ran without a login' }
    Invoke-Install @{ PATH = "$GhBin$Sep$SysPath"; FAKE_GH_OLD = '1' }
    Assert-Code 0
    Assert-Out 'signature not checked here (gh is too old to pin the tag); rhythm will verify itself and every tool it installs'
    if ((Get-SigLog).Count) { Fail 'gh ran without --source-ref' }
}

function Case-signature_none {
    Invoke-Install
    Assert-Code 0
    Assert-Out 'signature not checked here (no cosign or gh); rhythm will verify itself and every tool it installs'
    Assert-Tools 'tuner', 'soundcheck', 'echo'

    # cosign can't check a release without a bundle; gh isn't there.
    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath"; RHYTHM_VERSION = '9.0.4' }
    Assert-Code 0
    Assert-Out 'warning: signature not checked here (the release has no checksums.txt.sigstore.json); rhythm will verify itself and every tool it installs'
    if ((Get-SigLog).Count) { Fail 'cosign ran without a bundle' }

    # The same with a gh that can't be used names both reasons.
    Invoke-Install @{ PATH = "$CosignBin$Sep$GhBin$Sep$SysPath"; RHYTHM_VERSION = '9.0.4'; FAKE_GH_AUTH = '1' }
    Assert-Code 0
    Assert-Out 'warning: signature not checked here (the release has no checksums.txt.sigstore.json and gh is not logged in);'

    # gh still checks a release without a bundle.
    Invoke-Install @{ PATH = "$CosignBin$Sep$GhBin$Sep$SysPath"; RHYTHM_VERSION = '9.0.4' }
    Assert-Code 0
    Assert-Out 'Signature ok  RhythmShahriar/rhythm @ refs/tags/v9.0.4 (gh attestation)'
    Assert-NoOut 'signature not checked'
}

function Case-skip_signature {
    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath"; RHYTHM_SKIP_SIGNATURE = '1'; FAKE_COSIGN_EXIT = '1' }
    Assert-Code 0
    Assert-Out 'warning: RHYTHM_SKIP_SIGNATURE is set: nothing checks that this release was signed by RhythmShahriar/rhythm. Use it only with a mirror you trust.'
    if ((Get-SigLog).Count) { Fail 'cosign ran with RHYTHM_SKIP_SIGNATURE=1' }
    if ((Get-Content (Join-Path $script:CaseDir 'argv.log')) -match '^rhythm verify') { Fail 'rhythm verify ran with RHYTHM_SKIP_SIGNATURE=1' }
    Assert-Tools 'tuner', 'soundcheck', 'echo'

    Invoke-Install @{ PATH = "$CosignBin$Sep$SysPath"; RHYTHM_SKIP_SIGNATURE = '0' }
    Assert-Code 0
    Assert-NoOut 'RHYTHM_SKIP_SIGNATURE'
    if (-not (Get-SigLog).Count) { Fail 'RHYTHM_SKIP_SIGNATURE=0 skipped cosign' }
}

function Case-router_verify {
    $Log = Join-Path $script:CaseDir 'argv.log'
    Invoke-Install
    Assert-Code 0
    if ((Get-Content $Log) -notcontains 'rhythm verify rhythm') { Fail "rhythm didn't verify itself" }
    Assert-Out 'Signature ok  RhythmShahriar/rhythm @ refs/tags/v0.7.0 (release-go.yml)'

    # A rhythm without the command isn't asked.
    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin'), $Log
    Invoke-Install @{ RHYTHM_VERSION = '0.6.0' }
    Assert-Code 0
    if ((Get-Content $Log) -match '^rhythm verify') { Fail 'a rhythm without verify was asked to verify' }

    # A failed self-check puts the previous rhythm back and installs no tool.
    Get-ChildItem (Join-Path $script:CaseDir 'bin') -Filter 'rhythm-*' | Remove-Item -Force
    Invoke-Install @{ RHYTHM_TEST_VERIFY = 'fail' }
    Assert-Code 1
    Assert-Out 'signature not valid: RhythmShahriar/rhythm 0.7.0'
    Assert-Out 'rhythm install: the new rhythm failed its own check (rhythm verify rhythm), so the previous one was put back; nothing was installed'
    if ((Get-Version) -ne 'rhythm 0.6.0') { Fail "rhythm.exe is $(Get-Version), want the previous rhythm 0.6.0" }
    Assert-Tools

    # Without a previous one, the new one is removed.
    Remove-Item -Recurse -Force (Join-Path $script:CaseDir 'bin')
    Invoke-Install @{ RHYTHM_TEST_VERIFY = 'fail' }
    Assert-Code 1
    Assert-Out 'so it was removed; nothing was installed'
    Assert-NoInstall 'a failed self-check'
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
    Assert-Out "Installed rhythm 0.7.0 to $Bin"
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
    Assert-Out "rhythm install: rhythm 0.7.0 doesn't know bogus"
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
        if ((Get-Version) -ne 'rhythm 0.7.0') { Fail "rhythm.exe is $(Get-Version), want rhythm 0.7.0" }
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
    if ((Get-Version) -ne 'rhythm 0.7.0') { Fail "rhythm.exe is $(Get-Version), want rhythm 0.7.0" }
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
    $Cases = 'default', 'none', 'some', 'commas', 'typo', 'tool_fails', 'defaults', 'pins', 'locked', 'path_note',
    'signature_cosign', 'signature_cosign_fails', 'signature_gh', 'signature_none', 'skip_signature', 'router_verify', 'bad_checksum', 'no_checksums', 'missing_dir', 'relative_dir', 'truncated', 'http_refused', 'http_download_refused', 'github_token', 'rhythm_token',
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
