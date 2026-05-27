#Requires -Version 5.1
<#
.SYNOPSIS
    Cyberbrein DevKit — Windows Installer
.DESCRIPTION
    Downloads the latest devkit binary from GitHub Releases, verifies the
    checksum, installs it to %LOCALAPPDATA%\devkit\ and adds that directory
    to your user PATH so you can run 'devkit' from any terminal.
.EXAMPLE
    irm https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/scripts/install.ps1 | iex
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo    = 'stichting-Cyberbrein-nl/ctfdevkit-cli'
$Binary  = 'devkit.exe'
$InstDir = Join-Path $env:LOCALAPPDATA 'devkit'

# ── Helpers ───────────────────────────────────────────────────────────────────
function Write-Info { param($m) Write-Host "  > $m" -ForegroundColor Cyan    }
function Write-Ok   { param($m) Write-Host "  v $m" -ForegroundColor Green   }
function Write-Warn { param($m) Write-Host "  ! $m" -ForegroundColor Yellow  }
function Write-Fail { param($m) Write-Error "  x $m"                         }

# ── Banner ────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "  ██████╗ ███████╗██╗   ██╗██╗  ██╗██╗████████╗" -ForegroundColor Cyan
Write-Host "  ██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║╚══██╔══╝" -ForegroundColor Cyan
Write-Host "  ██║  ██║█████╗  ██║   ██║█████╔╝ ██║   ██║   " -ForegroundColor Cyan
Write-Host "  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔═██╗ ██║   ██║   " -ForegroundColor Cyan
Write-Host "  ██████╔╝███████╗ ╚████╔╝ ██║  ██╗██║   ██║   " -ForegroundColor Cyan
Write-Host "  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝╚═╝   ╚═╝   " -ForegroundColor Cyan
Write-Host ""
Write-Host "  Cyberbrein DevKit — Windows Installer" -ForegroundColor White
Write-Host ""

# ── Latest version ────────────────────────────────────────────────────────────
function Get-LatestVersion {
    $resp = Invoke-RestMethod `
        -Uri     "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ 'User-Agent' = 'devkit-installer' }
    return $resp.tag_name.TrimStart('v')
}

# ── SHA256 helper ─────────────────────────────────────────────────────────────
function Get-Sha256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLower()
}

# ── Add directory to user PATH (no admin required) ────────────────────────────
# ── Add directory to user PATH (no admin required) ────────────────────────────
function Add-ToUserPath([string]$Dir) {
    $regKey = 'Registry::HKEY_CURRENT_USER\Environment'

    # Read current User PATH directly from registry (not via .NET, dat cachet/quirkt)
    $current = (Get-ItemProperty -Path $regKey -Name 'PATH' -ErrorAction SilentlyContinue).PATH
    if ($null -eq $current) { $current = '' }

    Write-Info "Current User PATH length: $($current.Length) chars"

    $parts = $current -split ';' | Where-Object { $_ -ne '' }

    if ($parts -contains $Dir) {
        Write-Ok "$Dir is already in your User PATH"
        if (($env:PATH -split ';') -notcontains $Dir) {
            $env:PATH = ($env:PATH.TrimEnd(';') + ";$Dir")
        }
        return $false
    }

    $newPath = ($parts + $Dir) -join ';'

    # Behoud REG_EXPAND_SZ als er %VAR% references in zitten, anders REG_SZ
    $valueKind = if ($newPath -match '%[^%]+%') { 'ExpandString' } else { 'String' }
    Write-Info "Writing PATH as $valueKind ($($newPath.Length) chars)"

    try {
        Set-ItemProperty -Path $regKey -Name 'PATH' -Value $newPath -Type $valueKind -Force
    } catch {
        Write-Fail "Failed to write User PATH to registry: $($_.Exception.Message)"
    }

    # Broadcast WM_SETTINGCHANGE zodat nieuwe processen het direct oppikken
    if (-not ('Win32.NativeMethods' -as [type])) {
        Add-Type -Namespace Win32 -Name NativeMethods -MemberDefinition @'
[DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    }
    $HWND_BROADCAST   = [IntPtr]0xffff
    $WM_SETTINGCHANGE = 0x1A
    $result = [UIntPtr]::Zero
    [void][Win32.NativeMethods]::SendMessageTimeout(
        $HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment',
        2, 5000, [ref]$result)

    # Verifieer wat we net geschreven hebben
    $verify = (Get-ItemProperty -Path $regKey -Name 'PATH' -ErrorAction SilentlyContinue).PATH
    $verifyParts = $verify -split ';'
    if ($verifyParts -notcontains $Dir) {
        Write-Fail "PATH write reported success maar $Dir staat er niet in na verify! Geschreven: $($verify.Length) chars"
    }
    Write-Ok "Registry verified: $Dir staat nu in User PATH (totaal $($verifyParts.Count) entries)"

    # En de huidige sessie ook
    $env:PATH = ($env:PATH.TrimEnd(';') + ";$Dir")

    return $true
}

# ── Main ──────────────────────────────────────────────────────────────────────
function Install-Devkit {
    Write-Info "Fetching latest version..."
    $Version = Get-LatestVersion
    $Tag     = "v$Version"

    Write-Info "Version:      $Tag"
    Write-Info "Installing to: $InstDir"
    Write-Host ""

    $BaseUrl     = "https://github.com/$Repo/releases/download/$Tag"
    $ArchiveName = 'devkit-windows-amd64.zip'
    $ArchiveUrl  = "$BaseUrl/$ArchiveName"
    $ChecksumUrl = "$BaseUrl/checksums.txt"

    # Temp directory
    $TmpDir = Join-Path $env:TEMP "devkit-install-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path $TmpDir | Out-Null

    try {
        # ── Download ──────────────────────────────────────────────────────────
        Write-Info "Downloading $ArchiveName..."
        $ArchivePath  = Join-Path $TmpDir $ArchiveName
        $ChecksumPath = Join-Path $TmpDir 'checksums.txt'

        Invoke-WebRequest -Uri $ArchiveUrl  -OutFile $ArchivePath  -UseBasicParsing
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -UseBasicParsing

        # ── Checksum verification ─────────────────────────────────────────────
        Write-Info "Verifying checksum..."
        $Actual       = Get-Sha256 $ArchivePath
        $ExpectedLine = Get-Content $ChecksumPath |
                        Where-Object { $_ -match [regex]::Escape($ArchiveName) }

        if (-not $ExpectedLine) {
            Write-Fail "No checksum found for $ArchiveName in checksums.txt"
        }

        $Expected = ($ExpectedLine -split '\s+')[0].ToLower()

        if ($Actual -ne $Expected) {
            Write-Fail "Checksum mismatch!`n  Expected: $Expected`n  Got:      $Actual"
        }
        Write-Ok "Checksum verified"

        # ── Extract ───────────────────────────────────────────────────────────
        Write-Info "Extracting..."
        Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

        # ── Install ───────────────────────────────────────────────────────────
        New-Item -ItemType Directory -Path $InstDir -Force | Out-Null
        $Dest = Join-Path $InstDir $Binary
        Copy-Item -Path (Join-Path $TmpDir $Binary) -Destination $Dest -Force
        Write-Ok "devkit v$Version installed at $Dest"

        # ── PATH ──────────────────────────────────────────────────────────────
        Write-Info "Updating PATH..."
        $Added = Add-ToUserPath -Dir $InstDir
        if ($Added) {
            Write-Ok "$InstDir added to your user PATH"
            Write-Warn "Open a new terminal window to use 'devkit' from anywhere."
        }

        # ── Smoke test ────────────────────────────────────────────────────────
        try {
            $Ver = & $Dest version 2>$null
            Write-Ok "Smoke test passed: $Ver"
        } catch { }

        # ── Done ──────────────────────────────────────────────────────────────
        Write-Host ""
        Write-Host "  +--------------------------------------------+" -ForegroundColor Green
        Write-Host "  |  Get started:                              |" -ForegroundColor Green
        Write-Host "  |    devkit setup                            |" -ForegroundColor Green
        Write-Host "  |    devkit up                               |" -ForegroundColor Green
        Write-Host "  |                                            |" -ForegroundColor Green
        Write-Host "  |  Update later:                             |" -ForegroundColor Green
        Write-Host "  |    devkit self-update                      |" -ForegroundColor Green
        Write-Host "  +--------------------------------------------+" -ForegroundColor Green
        Write-Host ""
        Write-Warn "Tip: 'devkit setup' needs Administrator rights for TLS certificates."
        Write-Warn "     Right-click PowerShell > 'Run as Administrator' for the first setup."
        Write-Host ""

    } finally {
        Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
    }
}

Install-Devkit
