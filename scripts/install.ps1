# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Installs rhythm and its tools on Windows from GitHub Releases:
#   irm https://rythm.sh/install.ps1 | iex
#
# Environment:
#   RHYTHM_INSTALL_DIR   where to install (default %LOCALAPPDATA%\Programs\rhythm)
#   RHYTHM_VERSION       a specific version, e.g. 0.5.0 (default: latest)
#   RHYTHM_TOOLS         tools to install as well, separated by spaces or commas; tool@version
#                        pins one (default: the tools rhythm installs on a fresh setup,
#                        "tuner soundcheck echo" for an older rhythm; "none" for none)
#   RHYTHM_LOCKED        1 installs the tools in .\rhythm.lock instead (rhythm install --locked)
#   RHYTHM_SKIP_SIGNATURE  1 skips the signature checks, for a mirror you trust
#   RHYTHM_GITHUB_API    GitHub API base, for mirrors (default https://api.github.com).
#                        It must be https; plain http is accepted only for 127.0.0.1, ::1 and localhost.
#                        It is a plain base URL: no user info, query or fragment.
#   GITHUB_TOKEN         sent to api.github.com only, to lift the anonymous rate limit
#   RHYTHM_GITHUB_TOKEN  sent to the RHYTHM_GITHUB_API host, whatever it is; wins over GITHUB_TOKEN
#   RHYTHM_INSTALL_TIMEOUT  seconds to wait for a server to answer (default 30, at most 3600)
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
    # The shared workflow that signs every release. Its identity alone would
    # accept any repository that calls it, so the repository and tag are
    # pinned as well (see "Verify a release" in README).
    $Workflow = 'RhythmShahriar/.github/.github/workflows/release-go.yml'
    $Issuer = 'https://token.actions.githubusercontent.com'
    # RHYTHM_SKIP_SIGNATURE is for a mirror that re-signs nothing; any value
    # but empty or 0 turns it on.
    $SkipSignature = $env:RHYTHM_SKIP_SIGNATURE -and $env:RHYTHM_SKIP_SIGNATURE -ne '0'
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

    # A mirror base needs none of these. [Uri] reads them as curl does while
    # install.sh refuses them, so both installers take the same values.
    if ($Api -match '[@?#\\]') {
        throw "rhythm install: RHYTHM_GITHUB_API must be a plain base URL, without user info, query or fragment: $Api"
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

    # A stalled API or download would hang `irm | iex` for good. The web
    # cmdlets give up on a server that hasn't answered in this long.
    $Timeout = if ($env:RHYTHM_INSTALL_TIMEOUT) { $env:RHYTHM_INSTALL_TIMEOUT } else { '30' }
    # Digits only: TryParse alone would take ' 5', '+5' or '1e3'. A number
    # too big for Int32 fails TryParse, so it gets the 3600 message too.
    $Secs = 0
    if ($Timeout -notmatch '^[0-9]+$' -or ($Timeout -match '^0+$')) {
        throw "rhythm install: RHYTHM_INSTALL_TIMEOUT must be a whole number of seconds: '$Timeout'"
    }
    if (-not [int]::TryParse($Timeout, [ref]$Secs) -or $Secs -gt 3600) {
        throw "rhythm install: RHYTHM_INSTALL_TIMEOUT must be at most 3600 seconds: '$Timeout'"
    }
    $Timeout = $Secs

    # Test-Timeout reports whether a web cmdlet's error was a timeout. The
    # wording differs between Windows PowerShell 5.1 and pwsh.
    function Test-Timeout($Err) {
        $E = $Err.Exception
        while ($E) {
            if ($E -is [TimeoutException] -or $E -is [Threading.Tasks.TaskCanceledException] -or
                ($E -is [Net.WebException] -and $E.Status -eq [Net.WebExceptionStatus]::Timeout)) { return $true }
            $E = $E.InnerException
        }
        return "$($Err.Exception.Message)" -match 'time(d)? ?out|Timeout'
    }

    $Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { if ($env:RHYTHM_ARCH) { $env:RHYTHM_ARCH } else { throw "rhythm install: unsupported CPU '$env:PROCESSOR_ARCHITECTURE'" } }
    }

    # The tag the signature must name: v, a digit, then only [0-9A-Za-z.+-],
    # so it is safe in a URL path and a cosign flag. A pinned version is what
    # the user asked for, so a mirror can't answer with another one. It is
    # checked before it goes into the release URL.
    $TagPattern = '^v[0-9][0-9A-Za-z.+-]*$'
    if ($env:RHYTHM_VERSION) {
        # One leading v at most, as install.sh's ${RHYTHM_VERSION#v}.
        $Tag = "v$($env:RHYTHM_VERSION -creplace '^v', '')"
        if ($Tag -cnotmatch $TagPattern) { throw "rhythm install: RHYTHM_VERSION isn't a version: '$env:RHYTHM_VERSION'" }
        $ReleaseUrl = "$Api/repos/$Repo/releases/tags/$Tag"
    }
    else {
        $ReleaseUrl = "$Api/repos/$Repo/releases/latest"
    }
    try {
        $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers -UseBasicParsing -TimeoutSec $Timeout
    }
    catch {
        $Status = 0
        if ($_.Exception.Response) { $Status = [int]$_.Exception.Response.StatusCode }
        if ($Status -eq 403 -or $Status -eq 429) { throw 'rhythm install: GitHub rate limit reached; set GITHUB_TOKEN' }
        if ($Status) { throw "rhythm install: couldn't fetch the release from GitHub ($ReleaseUrl): HTTP $Status" }
        if (Test-Timeout $_) { throw "rhythm install: timed out after $($Timeout)s waiting for $($ApiUri.Host) ($ReleaseUrl); set RHYTHM_INSTALL_TIMEOUT to wait longer" }
        throw "rhythm install: couldn't reach $($ApiUri.Host) ($ReleaseUrl): $($_.Exception.Message)"
    }
    if (-not $env:RHYTHM_VERSION) {
        $Tag = "$($Release.tag_name)"
        if ($Tag -cnotmatch $TagPattern) { throw "rhythm install: the release has an odd tag: '$Tag'" }
    }

    $Name = "rhythm_windows_$Arch.zip"
    $Archive = $Release.assets | Where-Object { $_.name -eq $Name }
    $Sums = $Release.assets | Where-Object { $_.name -eq 'checksums.txt' }
    $Bundle = $Release.assets | Where-Object { $_.name -eq 'checksums.txt.sigstore.json' }
    if (-not $Archive) { throw "rhythm install: the release has no build for windows/$Arch" }
    if (-not $Sums) { throw 'rhythm install: the release has no checksums.txt; refusing to install unverified binaries' }
    Assert-Https $Archive.browser_download_url
    Assert-Https $Sums.browser_download_url
    if ($Bundle) { Assert-Https $Bundle.browser_download_url }

    # Save-Url downloads $Url to $OutFile, naming a timeout as one.
    function Save-Url([string]$Url, [string]$OutFile) {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -TimeoutSec $Timeout
        }
        catch {
            if (Test-Timeout $_) { throw "rhythm install: timed out after $($Timeout)s downloading $Url; set RHYTHM_INSTALL_TIMEOUT to wait longer" }
            throw
        }
    }

    # Invoke-Exe runs a program and returns its exit code and its output,
    # stderr included. Windows PowerShell 5.1 turns redirected stderr into
    # errors, which 'Stop' would throw, so this lowers the preference first.
    function Invoke-Exe([string]$Exe, [string[]]$Arguments) {
        $Prev = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $Out = & $Exe @Arguments 2>&1 | ForEach-Object { "$_" }
            @{ Code = $LASTEXITCODE; Out = ($Out -join "`n") }
        }
        finally {
            $ErrorActionPreference = $Prev
        }
    }

    # Test-Signature checks that checksums.txt was signed by rhythm's release
    # workflow for $Tag, with cosign or else gh, whichever is installed and
    # usable. Without either it says why and goes on: once installed, rhythm
    # checks itself and every tool it installs. A release with no bundle
    # while cosign is there makes that line a warning, since whoever can
    # change a release or mirror can delete the bundle. It isn't fatal:
    # releases from before signing have none and gh may still check.
    function Test-Signature([string]$SumsFile, [string]$BundleFile) {
        $Why = ''
        $NoBundle = $false
        if (Get-Command cosign -CommandType Application -ErrorAction SilentlyContinue) {
            if ($BundleFile) {
                $R = Invoke-Exe 'cosign' @('verify-blob', '--bundle', $BundleFile,
                    '--certificate-oidc-issuer', $Issuer,
                    '--certificate-identity', "https://github.com/$Workflow@refs/heads/master",
                    '--certificate-github-workflow-repository', $Repo,
                    '--certificate-github-workflow-ref', "refs/tags/$Tag",
                    $SumsFile)
                if ($R.Code -eq 0) { Write-Host "Signature ok  $Repo @ refs/tags/$Tag (cosign)"; return }
                if ($R.Out) { Write-Host $R.Out }
                throw "rhythm install: cosign says checksums.txt wasn't signed for $Repo $Tag; nothing was installed"
            }
            $Why = 'the release has no checksums.txt.sigstore.json'
            $NoBundle = $true
        }
        # gh needs a login to ask for attestations and --source-ref to pin the tag.
        $GhWhy = ''
        if (-not (Get-Command gh -CommandType Application -ErrorAction SilentlyContinue)) {
            $GhWhy = if ($Why) { 'none' } else { 'no cosign or gh' }
        }
        elseif ((Invoke-Exe 'gh' @('auth', 'status')).Code -ne 0) { $GhWhy = 'gh is not logged in' }
        elseif ((Invoke-Exe 'gh' @('attestation', 'verify', '--help')).Out -notmatch '--source-ref') { $GhWhy = 'gh is too old to pin the tag' }
        if (-not $GhWhy) {
            $R = Invoke-Exe 'gh' @('attestation', 'verify', $SumsFile, '--repo', $Repo, '--signer-workflow', $Workflow, '--source-ref', "refs/tags/$Tag")
            if ($R.Code -eq 0) { Write-Host "Signature ok  $Repo @ refs/tags/$Tag (gh attestation)"; return }
            if ($R.Out) { Write-Host $R.Out }
            throw "rhythm install: gh found no attestation that $Repo $Tag made this checksums.txt; nothing was installed. Releases from before attestations have none: install cosign to check those"
        }
        if ($GhWhy -and $GhWhy -ne 'none') { $Why = if ($Why) { "$Why and $GhWhy" } else { $GhWhy } }
        $Msg = "signature not checked ($Why); install cosign or gh to check it before anything runs"
        if ($NoBundle) { Write-Host "warning: $Msg" } else { Write-Host $Msg }
    }

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
        Save-Url $Archive.browser_download_url $Zip
        Save-Url $Sums.browser_download_url $SumsFile
        $BundleFile = ''
        if ($Bundle) {
            $BundleFile = Join-Path $Tmp 'checksums.txt.sigstore.json'
            Save-Url $Bundle.browser_download_url $BundleFile
        }
        if ($SkipSignature) {
            Write-Host "warning: RHYTHM_SKIP_SIGNATURE is set: nothing checks that this release was signed by $Repo. Use it only with a mirror you trust."
        }
        else {
            Test-Signature $SumsFile $BundleFile
        }

        $Line = Get-Content $SumsFile | Where-Object { $_ -match ('^[0-9a-f]{64}\s+\*?' + [regex]::Escape($Name) + '$') } | Select-Object -First 1
        $Want = if ($Line) { ($Line -split '\s+')[0].ToLower() } else { '' }
        $Got = (Get-FileHash -Algorithm SHA256 -Path $Zip).Hash.ToLower()
        if (-not $Want -or $Want -ne $Got) { throw "rhythm install: checksum mismatch for $Name; nothing was installed" }

        Expand-Archive -Path $Zip -DestinationPath $Tmp -Force
        New-Item -ItemType Directory -Force -Path $Dir | Out-Null
        # A relative folder would be wrong in PATH and in what rhythm is told.
        $Dir = (Resolve-Path -LiteralPath $Dir).ProviderPath
        # The old rhythm.exe is kept until the new one has checked itself.
        $Dest = Join-Path $Dir 'rhythm.exe'
        $Keep = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
        $OldCopy = ''
        if (Test-Path -LiteralPath $Dest -PathType Leaf) {
            New-Item -ItemType Directory -Path $Keep | Out-Null
            $OldCopy = Join-Path $Keep 'rhythm.exe'
            try { Copy-Item -LiteralPath $Dest -Destination $OldCopy } catch { $OldCopy = '' }
        }
        Install-Rhythm (Join-Path $Tmp 'rhythm.exe') $Dest
    }
    finally {
        Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
    }

    # Invoke-Native runs rhythm.exe through Invoke-Exe.
    function Invoke-Native([string[]]$Arguments) { Invoke-Exe $Rhythm $Arguments }

    # Get-KnownTools returns the names in $Tools that the installed rhythm
    # knows, checking every one before any tool is downloaded. Newer routers
    # check the whole list in one `rhythm install --dry-run` and name the
    # unknown ones with a suggestion. Older ones reject the flag, so the names
    # come from the "Tools:" block of `rhythm help`, or from $DefaultTools
    # when even that is missing. A name the user asked for must be known; an
    # unknown default comes from an old pinned rhythm and is skipped.
    function Get-KnownTools([string[]]$Tools) {
        if (-not $Tools.Count) { return @() }
        $R = Invoke-Native (@('install', '--dry-run') + $Tools)
        if ($R.Code -eq 0) { return $Tools }
        $Known = @()
        $Old = $R.Out -match 'flag provided but not defined'
        if ($Old) {
            # Rows look like "  soundcheck (sc)  did my change break anything?".
            $In = $false
            foreach ($Row in ((Invoke-Native @('help')).Out -split "`n")) {
                if ($Row -match '^Tools:') { $In = $true; continue }
                if (-not $In) { continue }
                if ($Row -match '^\s*$') { break }
                $Words = @($Row.Trim() -split '\s+')
                $Known += $Words[0]
                if ($Words.Count -gt 1 -and $Words[1] -match '^\((.+)\)$') { $Known += $Matches[1] }
            }
            if (-not $Known.Count) { $Known = @($DefaultTools -split ' ') }
        }
        elseif ($Explicit) {
            Write-Host $R.Out
            $Bad = @([regex]::Matches($R.Out, 'unknown tool "([^"]*)"') | ForEach-Object { $_.Groups[1].Value })
            if (-not $Bad.Count) { throw "rhythm install: $Installed refused RHYTHM_TOOLS (see above); nothing more was installed" }
            if ($Bad -match '@') { throw "rhythm install: $Installed doesn't know $($Bad -join ', '); it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version" }
            throw "rhythm install: $Installed doesn't know $($Bad -join ', '); run '$Rhythm help' to see the tools"
        }
        $Ok = @()
        foreach ($Tool in $Tools) {
            # A default this rhythm doesn't know: ask about each one, to skip it.
            $Knows = if ($Old) { $Known -contains $Tool } else { (Invoke-Native @('install', '--dry-run', $Tool)).Code -eq 0 }
            if ($Knows) { $Ok += $Tool; continue }
            if ($Tool -match '@') { throw "rhythm install: $Installed doesn't know $Tool; it can't pin tool versions, so set RHYTHM_VERSION to a newer rhythm or leave out @version" }
            if ($Explicit) { throw "rhythm install: $Installed doesn't know $Tool; run '$Rhythm help' to see the tools" }
            Write-Host "warning: $Installed doesn't know $Tool; skipped"
        }
        return $Ok
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
        Remove-Item -Recurse -Force $Keep -ErrorAction SilentlyContinue
        Write-Host "Installed rhythm.exe to $Dir"
        return
    }
    # A rhythm with the verify command checks its own release and bytes.
    try {
        if (-not $SkipSignature -and (Invoke-Native @('help')).Out -match '(?m)^\s*rhythm verify') {
            $R = Invoke-Native @('verify', 'rhythm')
            if ($R.Out) { Write-Host $R.Out }
            if ($R.Code -ne 0) {
                if ($OldCopy) {
                    Install-Rhythm $OldCopy $Rhythm
                    throw 'rhythm install: the new rhythm failed its own check (rhythm verify rhythm), so the previous one was put back; nothing was installed'
                }
                Remove-Item -LiteralPath $Rhythm -Force
                throw 'rhythm install: the new rhythm failed its own check (rhythm verify rhythm), so it was removed; nothing was installed'
            }
        }
    }
    finally {
        Remove-Item -Recurse -Force $Keep -ErrorAction SilentlyContinue
    }
    Write-Host "Installed $Installed to $Dir"

    # An unset RHYTHM_TOOLS means the defaults, which a newer rhythm names
    # itself (`rhythm install --defaults --dry-run`); for an older one they
    # are $DefaultTools. Names are separated by spaces or commas as in
    # rhythm's own lists; "none" stands for no tool and tool@version pins
    # one, passed to rhythm as it is. Windows PowerShell 5.1 can't set a
    # variable to '' (that deletes it), so "none" is the way to ask for none
    # there. An old pinned rhythm may not know every default tool; those are
    # skipped. A name the user asked for must be known, or no tool is
    # installed. RHYTHM_LOCKED=1 installs what .\rhythm.lock says instead.
    $Explicit = $null -ne $env:RHYTHM_TOOLS
    $Locked = $env:RHYTHM_LOCKED -and $env:RHYTHM_LOCKED -ne '0'
    # Get-ToolName turns an alias, a rhythm-<tool> name or a pin
    # (tuner@0.3.1) into the tool's name.
    function Get-ToolName([string]$Tool) {
        $Name = $Tool -replace '@.*$', '' -replace '^rhythm-', ''
        switch ($Name) { 't' { 'tuner' } 'sc' { 'soundcheck' } 'e' { 'echo' } default { $Name } }
    }
    $Known = @()
    if ($Locked) {
        if (-not (Test-Path -LiteralPath 'rhythm.lock' -PathType Leaf)) {
            throw "rhythm install: RHYTHM_LOCKED is set but there is no rhythm.lock in $((Get-Location).ProviderPath)"
        }
        if ((Invoke-Native @('install', '--help')).Out -notmatch '--locked') {
            throw "rhythm install: $Installed can't install from rhythm.lock; set RHYTHM_VERSION to a newer rhythm"
        }
        if ($Explicit) { Write-Host 'warning: RHYTHM_TOOLS is ignored: RHYTHM_LOCKED installs the tools in rhythm.lock' }
    }
    else {
        $R = if ($Explicit) { $null } else { Invoke-Native @('install', '--defaults', '--dry-run') }
        if ($R -and $R.Code -eq 0) {
            $Known = @($R.Out -split "`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        }
        else {
            $ToolList = if ($Explicit) { $env:RHYTHM_TOOLS } else { $DefaultTools }
            $Known = Get-KnownTools @($ToolList -split '[\s,]+' | Where-Object { $_ -and $_ -ne 'none' })
        }
    }
    # An alias and the name are one tool, installed once.
    $Todo = @()
    $Names = @()
    foreach ($Tool in $Known) {
        $Name = Get-ToolName $Tool
        if ($Names -notcontains $Name) { $Names += $Name; $Todo += $Tool }
    }
    # The tools installed and failed, by name rather than alias, for the
    # closing lines. A tool that fails doesn't stop the next; the router has
    # said why.
    $Done = @()
    $Failed = @()
    # The PATH message comes once, at the end. A router that has --quiet
    # leaves out its own PATH notes; an older one prints one per tool,
    # filtered here.
    $Quiet = @()
    if (($Todo.Count -or $Locked) -and (Invoke-Native @('install', '--help')).Out -match '--quiet') { $Quiet = @('--quiet') }
    if ($Locked) {
        # One call; the tools installed are the ones rhythm says it installed.
        $R = Invoke-Native (@('install', '--locked', '--dir', $Dir) + $Quiet)
        $Lines = @($R.Out -split "`n" | Where-Object { $_ -and $_ -notmatch "^Note: .* isn't on your PATH\.$" })
        if ($Lines.Count) { Write-Host ($Lines -join "`n") }
        # A tool already at the locked version counts as installed too.
        $Done = @([regex]::Matches($R.Out, '(?m)^(?:Installed rhythm-(\S+) |rhythm-(\S+) \S+ is already installed at )') |
            ForEach-Object { if ($_.Groups[1].Success) { $_.Groups[1].Value } else { $_.Groups[2].Value } })
        if ($R.Code -ne 0) {
            $Failed = @([regex]::Matches($R.Out, '(?m)^Installed \d+, .*failed \d+: (.*)$') | ForEach-Object { $_.Groups[1].Value -split ',\s*' } | ForEach-Object { $_.Trim() -replace '^rhythm-', '' })
            if (-not $Failed.Count) { $Failed = @('rhythm.lock') }
        }
    }
    foreach ($Tool in $Todo) {
        $R = Invoke-Native (@('install', $Tool, '--dir', $Dir) + $Quiet)
        $Lines = @($R.Out -split "`n" | Where-Object { $_ -and $_ -notmatch "^Note: .* isn't on your PATH\.$" })
        if ($Lines.Count) { Write-Host ($Lines -join "`n") }
        if ($R.Code -eq 0) { $Done += Get-ToolName $Tool } else { $Failed += Get-ToolName $Tool }
    }

    # A terminal keeps the PATH it started with, so a folder that is new to
    # the user PATH, or not yet in this session's, needs a new terminal.
    if ($OnWindows) {
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if (-not (($UserPath -split ';') -contains $Dir)) {
            [Environment]::SetEnvironmentVariable('Path', (($UserPath, $Dir) -join ';').TrimStart(';'), 'User')
            Write-Host "Added $Dir to your user PATH. Open a new terminal to use rhythm."
        }
        elseif (-not (($env:Path -split ';') -contains $Dir)) {
            Write-Host "$Dir is on your user PATH. Open a new terminal to use rhythm."
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
    elseif (-not $Failed.Count) {
        Write-Host 'Install a tool: rhythm install tuner'
    }
    Write-Host 'Rhythm tools are for sites you own or have permission to test: https://github.com/RhythmShahriar/rhythm/wiki/Responsible-Use'
    if ($Failed.Count) {
        $Ok = if ($Done.Count) { $Done -join ', ' } else { 'none' }
        throw "Installed: $Ok. Failed: $($Failed -join ', ') (see above)."
    }
}
