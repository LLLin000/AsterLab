# Reproducible unsigned Pure-Go MyGo release build: dist + portable zip + NSIS installer + SHA256SUMS.
param(
    [string]$Version = '0.1.0-unsigned'
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
& (Join-Path $PSScriptRoot 'mygo-build.ps1')
if ($LASTEXITCODE -ne 0) { throw "mygo-build.ps1 failed" }

$zip = & (Join-Path $PSScriptRoot 'mygo-package.ps1') -Version $Version
if ($LASTEXITCODE -ne 0) { throw "mygo-package.ps1 failed" }

$nsis = (Get-Command makensis -ErrorAction SilentlyContinue).Source
if (-not $nsis) { throw "makensis not found; install NSIS" }
Push-Location $PSScriptRoot
try {
    & $nsis "/DVERSION=$Version" 'mygo-installer.nsi'
    if ($LASTEXITCODE -ne 0) { throw "makensis failed" }
} finally {
    Pop-Location
}

$releaseDir = Join-Path $repoRoot 'release'
$installer = Join-Path $releaseDir "AsterLab-MyGo-$Version-win-x64-setup.exe"
$distInfo = Join-Path $repoRoot 'prototypes\mygo-run\dist\BUILD_INFO.txt'
$relInfo = Join-Path $releaseDir 'BUILD_INFO.txt'
Copy-Item -LiteralPath $distInfo -Destination $relInfo -Force

$sumsPath = Join-Path $releaseDir 'SHA256SUMS.txt'
Remove-Item -LiteralPath $sumsPath -ErrorAction SilentlyContinue
foreach ($f in @($installer, $zip, (Join-Path $repoRoot 'prototypes\mygo-run\dist\mygo-run.exe'), (Join-Path $repoRoot 'prototypes\mygo-run\dist\asterlab.exe'))) {
    $h = (Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash
    $len = (Get-Item -LiteralPath $f).Length
    $name = [System.IO.Path]::GetFileName($f)
    "$h  $name ($len bytes)" | Out-File -FilePath $sumsPath -Append -Encoding utf8
}
Write-Output "installer: $installer"
Write-Output "zip: $zip"
Write-Output "sums: $sumsPath"