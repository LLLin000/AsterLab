# scripts/mygo-build.ps1
# Build Pure-Go AsterLab desktop app + CLI into a clean dist folder.
# Unsigned: this script does not sign anything.
param(
    [string]$Version = '0.1.0-dev'
)
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$protoDir = Join-Path $repoRoot 'prototypes\mygo-run'
$distDir = Join-Path $protoDir 'dist'

if (-not (Test-Path -LiteralPath $protoDir)) { throw "prototype dir missing: $protoDir" }

# Clean dist
if (Test-Path -LiteralPath $distDir) { Remove-Item -LiteralPath $distDir -Recurse -Force }
New-Item -ItemType Directory -Path $distDir | Out-Null

Push-Location $protoDir
try {
    $env:CGO_ENABLED = '0'
    $commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
    $verFlags = "-X 'asterlab-mygo-prototype/internal/updater.Version=$Version' -X 'asterlab-mygo-prototype/internal/updater.Commit=$commit'"

    # 1. Pure-Go Desktop app (windowsgui hides the console window when double-clicked)
    go build -tags=e2e -trimpath -ldflags="-s -w -H=windowsgui $verFlags" -o (Join-Path $distDir 'mygo-run.exe') .
    if ($LASTEXITCODE -ne 0) { throw "go build desktop failed (exit $LASTEXITCODE)" }

    # 2. Pure-Go Vault CLI
    go build -trimpath -ldflags="-s -w $verFlags" -o (Join-Path $distDir 'asterlab.exe') ./cmd/asterlab
    if ($LASTEXITCODE -ne 0) { throw "go build cli failed (exit $LASTEXITCODE)" }

    $icoSrc = Join-Path $protoDir 'assets\asterlab.ico'
    if (Test-Path -LiteralPath $icoSrc) {
        Copy-Item -LiteralPath $icoSrc -Destination (Join-Path $distDir 'asterlab.ico')
    }

    # Evidence
    $shaExe = (Get-FileHash -LiteralPath (Join-Path $distDir 'mygo-run.exe') -Algorithm SHA256).Hash
    $shaCli = (Get-FileHash -LiteralPath (Join-Path $distDir 'asterlab.exe') -Algorithm SHA256).Hash
    $info = "AsterLab Pure-Go Build (UNSIGNED)`nversion $Version`ncommit $commit`nmygo-run.exe SHA256 $shaExe`nasterlab.exe SHA256 $shaCli`n"
    Set-Content -LiteralPath (Join-Path $distDir 'BUILD_INFO.txt') -Value $info -Encoding utf8
    Write-Output "dist: $distDir"
    Write-Output $info
} finally {
    Pop-Location
}
