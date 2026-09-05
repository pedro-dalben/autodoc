# AutoDoc installer for Windows: downloads a versioned binary from GitHub
# Releases, verifies its SHA-256 checksum, and installs it. No Go needed.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.ps1 | iex
#   .\install.ps1 -Version v0.1.0
#   $env:AUTODOC_VERSION = "v0.1.0"; .\install.ps1
param(
  [string]$Version = $env:AUTODOC_VERSION,
  [string]$InstallDir = $env:AUTODOC_INSTALL_DIR,
  [string]$Repo = "pedro-dalben/autodoc"
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrEmpty($Version)) { $Version = "latest" }
if ([string]::IsNullOrEmpty($InstallDir)) {
  $InstallDir = Join-Path $env:LOCALAPPDATA "autodoc\bin"
}
if ($Version -notlike "v*") { $Version = "v$Version" }

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default { Write-Error "unsupported architecture: $env:PROCESSOR_ARCHITECTURE (supported: AMD64, ARM64)"; exit 1 }
}

if ($Version -eq "vlatest") {
  # GitHub's /releases/latest endpoint only serves stable releases, never
  # pre-releases. While AutoDoc has no stable release yet, install the
  # release candidate explicitly instead of relying on "latest".
  try {
    $latest = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $latest.tag_name
  } catch {
    Write-Error "no stable AutoDoc release is available yet (only pre-releases). Install the release candidate explicitly: .\install.ps1 -Version <tag> (see https://github.com/$Repo/releases and docs/install.md)"
    exit 1
  }
}
$AssetVer = $Version
if ($AssetVer.StartsWith("v")) { $AssetVer = $AssetVer.Substring(1) }
$Asset = "autodoc_${AssetVer}_windows_${Arch}.zip"
$Base = "https://github.com/$Repo/releases/download/$Version"
New-Item -ItemType Directory -Path $Tmp | Out-Null
try {
  Write-Host "==> autodoc $Version (windows/$Arch)"
  try {
    Invoke-WebRequest "$Base/$Asset" -OutFile "$Tmp\autodoc.zip"
    Invoke-WebRequest "$Base/checksums.txt" -OutFile "$Tmp\checksums.txt"
  } catch {
    Write-Error "download failed: $Base/$Asset not found (bad version or unsupported platform?)"
    exit 1
  }

  $line = Select-String -Path "$Tmp\checksums.txt" -Pattern " $Asset`$" | Select-Object -First 1
  if (-not $line) {
    Write-Error "checksum for $Asset missing in checksums.txt (aborting)"
    exit 1
  }
  $want = ($line.Line -split " ")[0]
  $got = (Get-FileHash "$Tmp\autodoc.zip" -Algorithm SHA256).Hash.ToLower()
  if ($got -ne $want.ToLower()) {
    Write-Error "checksum mismatch for $Asset (expected $want, got $got; aborting)"
    exit 1
  }

  Expand-Archive "$Tmp\autodoc.zip" -DestinationPath "$Tmp\extract" -Force
  $bin = Get-ChildItem "$Tmp\extract" -Filter "autodoc.exe" -Recurse | Select-Object -First 1
  if (-not $bin) {
    Write-Error "autodoc.exe not found inside $Asset"
    exit 1
  }
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item $bin.FullName (Join-Path $InstallDir "autodoc.exe") -Force
  & (Join-Path $InstallDir "autodoc.exe") version | Out-Null
  if ($LASTEXITCODE -ne 0) {
    Write-Error "installed binary failed to run"
    exit 1
  }

  Write-Host "installed $InstallDir\autodoc.exe"
  $path = [Environment]::GetEnvironmentVariable("Path", "User")
  if (($path -split ";") -notcontains $InstallDir) {
    Write-Host "note: $InstallDir is not on PATH. Add it with:"
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$InstallDir', 'User')"
  }
  Write-Host "next: autodoc init; autodoc doctor"
} finally {
  Remove-Item $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
