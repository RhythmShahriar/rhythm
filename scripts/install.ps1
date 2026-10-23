# Installs rhythm and its tools on Windows from GitHub Releases:
#   irm https://rythm.sh/install.ps1 | iex
#
# Environment:
#   RHYTHM_INSTALL_DIR  where to install (default %LOCALAPPDATA%\Programs\rhythm)
#   RHYTHM_VERSION      a specific version, e.g. 0.2.0 (default: latest)
#   RHYTHM_TOOLS        tools to install as well (default "tuner"; set empty for none)
#   RHYTHM_GITHUB_API   GitHub API base, for mirrors (default https://api.github.com)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is far slower with the progress bar
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo = 'RhythmShahriar/rhythm'
$Api = if ($env:RHYTHM_GITHUB_API) { $env:RHYTHM_GITHUB_API } else { 'https://api.github.com' }
$Dir = if ($env:RHYTHM_INSTALL_DIR) { $env:RHYTHM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\rhythm' }
$Tools = if ($null -ne $env:RHYTHM_TOOLS) { $env:RHYTHM_TOOLS } else { 'tuner' }
# Windows PowerShell 5.1 has no $IsWindows; it only runs on Windows.
$OnWindows = $PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { if ($env:RHYTHM_ARCH) { $env:RHYTHM_ARCH } else { throw "rhythm install: unsupported CPU '$env:PROCESSOR_ARCHITECTURE'" } }
}

$ReleaseUrl = if ($env:RHYTHM_VERSION) { "$Api/repos/$Repo/releases/tags/v$($env:RHYTHM_VERSION.TrimStart('v'))" } else { "$Api/repos/$Repo/releases/latest" }
$Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers @{ Accept = 'application/vnd.github+json'; 'User-Agent' = 'rhythm-install' }

$Name = "rhythm_windows_$Arch.zip"
$Archive = $Release.assets | Where-Object { $_.name -eq $Name }
$Sums = $Release.assets | Where-Object { $_.name -eq 'checksums.txt' }
if (-not $Archive) { throw "rhythm install: the release has no build for windows/$Arch" }
if (-not $Sums) { throw 'rhythm install: the release has no checksums.txt; refusing to install unverified binaries' }

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
if (-not $OnWindows) {
    Write-Host "Installed rhythm.exe to $Dir"
    return
}
Write-Host "Installed $(& $Rhythm --version) to $Dir"
foreach ($Tool in ($Tools -split '\s+' | Where-Object { $_ })) {
    & $Rhythm install $Tool --dir $Dir
    if ($LASTEXITCODE -ne 0) { throw "rhythm install: installing $Tool failed" }
}

$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not (($UserPath -split ';') -contains $Dir)) {
    [Environment]::SetEnvironmentVariable('Path', (($UserPath, $Dir) -join ';').TrimStart(';'), 'User')
    Write-Host "Added $Dir to your PATH. Open a new terminal to use rhythm."
}
Write-Host 'Get started: rhythm tuner scan example.com'
