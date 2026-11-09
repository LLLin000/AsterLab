# Reproducible unsigned MyGo release build: build dist, then NSIS installer.
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
& (Join-Path $PSScriptRoot 'mygo-build.ps1')
$nsis = (Get-Command makensis -ErrorAction SilentlyContinue).Source
if (-not $nsis) { throw "makensis not found; install NSIS" }
Push-Location $PSScriptRoot
try { & $nsis 'mygo-installer.nsi' } finally { Pop-Location }
Write-Output "installer: $(Join-Path $repoRoot 'release\AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe')"