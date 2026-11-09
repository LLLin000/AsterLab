# Package the unsigned MyGo prototype dist into a zip. No signing/installer.
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $repoRoot 'prototypes\mygo-run\dist'
if (-not (Test-Path -LiteralPath $distDir)) { throw "missing dist; run scripts/mygo-build.ps1 first" }
$releaseDir = Join-Path $repoRoot 'release'
if (-not (Test-Path -LiteralPath $releaseDir)) { New-Item -ItemType Directory -Path $releaseDir | Out-Null }
$version = '0.1.0-beta-unsigned'
$name = "AsterLab-MyGo-$version-win-x64"
$pkg = Join-Path $releaseDir $name
$zip = Join-Path $releaseDir "$name.zip"
if (Test-Path -LiteralPath $pkg) { Remove-Item -LiteralPath $pkg -Recurse -Force }
New-Item -ItemType Directory -Path $pkg | Out-Null
Copy-Item -Path (Join-Path $distDir '*') -Destination $pkg -Recurse
Set-Content -LiteralPath (Join-Path $pkg 'UNSIGNED.txt') -Value "UNSIGNED PROTOTYPE. No installer, no code signing, no certificate.`nKeep mygo-run.exe and Runs.dll in the same folder." -Encoding utf8
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -LiteralPath $pkg -DestinationPath $zip -CompressionLevel Optimal
Write-Output $zip
