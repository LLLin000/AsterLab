# scripts/mygo-build-dev.ps1
# Build the developer-only AsterLab desktop debug binary (NEVER shipped to end users).
# Enables:
#   - Ctrl+Shift+D UI Detail Picker & Feedback Bundle exporter (devdebug tag)
#   - MyGo native DevTools Element Inspector (F12 / Ctrl+Shift+I / Ctrl+Shift+C)
#   - MyGo E2E automation plugin (e2e tag)
param(
    [string]$Version = '0.1.9-devdebug'
)
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$protoDir = Join-Path $repoRoot 'prototypes\mygo-run'
$distDir = Join-Path $protoDir 'dist-dev'

if (-not (Test-Path -LiteralPath $protoDir)) { throw "prototype dir missing: $protoDir" }
if (-not (Test-Path -LiteralPath $distDir)) {
    New-Item -ItemType Directory -Path $distDir | Out-Null
}

Push-Location $protoDir
try {
    $env:CGO_ENABLED = '0'
    $commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
    $verFlags = "-X 'asterlab-mygo-prototype/internal/updater.Version=$Version' -X 'asterlab-mygo-prototype/internal/updater.Commit=$commit'"

    $outExe = Join-Path $distDir 'asterlab-dev.exe'
    go build -tags="e2e,devdebug" -ldflags="-H=windowsgui $verFlags" -o $outExe .
    if ($LASTEXITCODE -ne 0) { throw "go build devdebug desktop failed (exit $LASTEXITCODE)" }

    $outCli = Join-Path $distDir 'asterlab.exe'
    go build -ldflags="$verFlags" -o $outCli ./cmd/asterlab
    if ($LASTEXITCODE -ne 0) { throw "go build cli failed (exit $LASTEXITCODE)" }

    $icoSrc = Join-Path $protoDir 'assets\asterlab.ico'
    if (Test-Path -LiteralPath $icoSrc) {
        Copy-Item -LiteralPath $icoSrc -Destination (Join-Path $distDir 'asterlab.ico') -Force
    }

    Write-Output "DEV build ready (DO NOT SHIP TO END USERS):"
    Write-Output "  Desktop: $outExe"
    Write-Output "  CLI:     $outCli"
    Write-Output "  Hotkeys: Ctrl+Shift+D = UI 细节拾取标注 | F12 / Ctrl+Shift+C = MyGo 原生源码检查器"
} finally {
    Pop-Location
}
