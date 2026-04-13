#Requires -Version 5.1
<#
.SYNOPSIS
    Installs the devkit CLI for Windows.
.DESCRIPTION
    Downloads the latest devkit binary from GitHub Releases, verifies the checksum,
    and installs it to $env:LOCALAPPDATA\devkit\ (added to the user's PATH).
.EXAMPLE
    irm https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/scripts/install.ps1 | iex
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo    = "stichting-Cyberbrein-nl/ctfdevkit-cli"
$Binary  = "devkit.exe"
$InstDir = Join-Path $env:LOCALAPPDATA "devkit"

function Write-Info    { param($m) Write-Host "  ● $m" -ForegroundColor Cyan }
function Write-Success { param($m) Write-Host "  ✓ $m" -ForegroundColor Green }
function Write-Warn    { param($m) Write-Host "  ⚠ $m" -ForegroundColor Yellow }
function Write-Fail    { param($m) Write-Error "  ✗ $m" }

function Get-LatestVersion {
    $url  = "https://api.github.com/repos/$Repo/releases/latest"
    $resp = Invoke-RestMethod -Uri $url -Headers @{ 'User-Agent' = 'devkit-installer' }
    return $resp.tag_name.TrimStart('v')
}

function Get-FileHash256 {
    param([string]$Path)
    return (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLower()
}

function Install-Devkit {
    Write-Host ""
    Write-Host "  ██████╗ ███████╗██╗   ██╗██╗  ██╗██╗████████╗" -ForegroundColor Cyan
    Write-Host "  ██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║╚══██╔══╝" -ForegroundColor Cyan
    Write-Host "  ██║  ██║█████╗  ██║   ██║█████╔╝ ██║   ██║   " -ForegroundColor Cyan
    Write-Host "  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔═██╗ ██║   ██║   " -ForegroundColor Cyan
    Write-Host "  ██████╔╝███████╗ ╚████╔╝ ██║  ██╗██║   ██║   " -ForegroundColor Cyan
    Write-Host "  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝╚═╝   ╚═╝   " -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Cyberbrein DevKit Installer for Windows"
    Write-Host ""

    $version = Get-LatestVersion
    Write-Info "Version:      v$version"
    Write-Info "Installing to: $InstDir"
    Write-Host ""

    $baseUrl     = "https://github.com/$Repo/releases/download/v$version"
    $archiveName = "devkit-windows-amd64.zip"
    $archiveUrl  = "$baseUrl/$archiveName"
    $checksumUrl = "$baseUrl/checksums.txt"

    $tmpDir = Join-Path $env:TEMP "devkit-install-$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    try {
        # Download archive.
        Write-Info "Downloading $archiveName..."
        $archivePath = Join-Path $tmpDir $archiveName
        Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath -UseBasicParsing

        # Download + verify checksum.
        Write-Info "Verifying checksum..."
        $checksumPath = Join-Path $tmpDir "checksums.txt"
        Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath -UseBasicParsing

        $actualHash   = Get-FileHash256 $archivePath
        $expectedLine = Get-Content $checksumPath | Where-Object { $_ -match $archiveName }
        if (-not $expectedLine) {
            Write-Fail "Could not find checksum for $archiveName in checksums.txt"
            return
        }
        $expectedHash = ($expectedLine -split '\s+')[0].ToLower()

        if ($actualHash -ne $expectedHash) {
            Write-Fail "Checksum mismatch: expected $expectedHash, got $actualHash"
            return
        }
        Write-Success "Checksum verified"

        # Extract.
        Write-Info "Extracting..."
        Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

        # Install.
        New-Item -ItemType Directory -Path $InstDir -Force | Out-Null
        Copy-Item -Path (Join-Path $tmpDir $Binary) -Destination (Join-Path $InstDir $Binary) -Force

        # Add to user PATH if not already present.
        $userPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
        if ($userPath -notlike "*$InstDir*") {
            [System.Environment]::SetEnvironmentVariable('PATH', "$userPath;$InstDir", 'User')
            Write-Info "Added $InstDir to your PATH"
            Write-Warn "Restart your terminal for PATH changes to take effect."
        }

        Write-Host ""
        Write-Success "devkit v$version installed to $InstDir\$Binary"
        Write-Host ""
        Write-Host "  Get started:"
        Write-Host "    devkit setup"
        Write-Host ""

    } finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

Install-Devkit
