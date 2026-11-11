param(
    [string]$Output = "target/asterlab-onevra.exe"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

Push-Location (Join-Path $repoRoot "prototypes/mygo-run")
try {
    go build -o $Output .
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
    Pop-Location
}

& (Join-Path $PSScriptRoot "sign-local.ps1") -Path (Join-Path $repoRoot ("prototypes/mygo-run/" + $Output))
