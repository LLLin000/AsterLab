# scripts/mygo-build.ps1
# Build MyGo prototype app + Runs.dll into a clean dist folder.
# Unsigned: this script does not sign anything.
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$protoDir = Join-Path $repoRoot 'prototypes\mygo-run'
$distDir = Join-Path $protoDir 'dist'
$mygoPin = 'v0.8.4-0.20261108220316-c4197b7fe562'
$mygo = 'go'

if (-not (Test-Path -LiteralPath $protoDir)) { throw "prototype dir missing: $protoDir" }

# Clean dist
if (Test-Path -LiteralPath $distDir) { Remove-Item -LiteralPath $distDir -Recurse -Force }
New-Item -ItemType Directory -Path $distDir | Out-Null

Push-Location $protoDir
try {
    # 1. Native module
    & $mygo run github.com/egoist/mygo/cmd/mygo@$mygoPin native check .\modules\runs
    if ($LASTEXITCODE -ne 0) { throw "mygo native check failed (exit $LASTEXITCODE)" }
    $dll = Join-Path $protoDir '.mygo\native\windows-amd64\Runs.dll'
    if (-not (Test-Path -LiteralPath $dll)) { throw "Runs.dll not produced: $dll" }
    Copy-Item -LiteralPath $dll -Destination (Join-Path $distDir 'Runs.dll')

    # 2. Go app
    $env:CGO_ENABLED = '0'
    go build -trimpath -o (Join-Path $distDir 'mygo-run.exe') .
    if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }

    # Evidence
    $commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
    $shaDll = (Get-FileHash -LiteralPath (Join-Path $distDir 'Runs.dll') -Algorithm SHA256).Hash
    $shaExe = (Get-FileHash -LiteralPath (Join-Path $distDir 'mygo-run.exe') -Algorithm SHA256).Hash
    $info = "AsterLab MyGo Prototype (UNSIGNED)`ncommit $commit`nmygo pin v0.8.4-0.20261108220316-c4197b7fe562`nRuns.dll SHA256 $shaDll`nmygo-run.exe SHA256 $shaExe`n"
    Set-Content -LiteralPath (Join-Path $distDir 'BUILD_INFO.txt') -Value $info -Encoding utf8
    Write-Output "dist: $distDir"
    Write-Output $info
} finally {
    Pop-Location
}
