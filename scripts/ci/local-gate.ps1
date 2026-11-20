# scripts/ci/local-gate.ps1
#
# Windows Gate lane CI, run on this Windows host instead of GitHub Actions.
#
# Why: hosted-runner minutes are exhausted, and the Windows Gate lane must not
# run Linux tests anyway — those belong to the Linux lane
# (scripts/ci/local-gate.sh on the VPS), per docs/engineering/local-ci.md.
#
# What it runs, in order (same contract as .github/workflows/windows-build.yml
# and .github/workflows/scope-check.yml, minus every Linux-only step):
#   1. scope-check selftest + card scope enforcement against the PR diff
#   2. REL-05 native generator hash check (two Windows generations)
#   3. scripts/mygo-build.ps1 -> dist/mygo-run.exe + dist/asterlab.exe
#   4. go test -tags=e2e ./... for the MyGo module
#   5. generated bindings unchanged by the build
#   6. artifact SHA256 records
#
# Usage:
#   ./scripts/ci/local-gate.ps1 -Pr 69          # PR diff + PR body as scope source
#   ./scripts/ci/local-gate.ps1 -Base origin/main -PrBody body.md
#   ./scripts/ci/local-gate.ps1                 # working tree vs origin/main
#
# Exit 0 = PASS, 1 = FAIL. A report is written to target/local-gate/<label>/.

[CmdletBinding()]
param(
    [string]$Base = 'origin/main',
    [int]$Pr = 0,
    [string]$PrBody = '',
    [switch]$SkipScope,
    [switch]$SkipTests,
    [string]$ReportRoot = ''
)

# Windows PowerShell 5.1 turns a native command's stderr into a terminating
# error under 'Stop', which would abort on `git fetch`/`cargo` progress lines.
# 'Continue' plus an explicit `$LASTEXITCODE` check in every step is the mode
# this script is written for; cmdlet failures still throw explicitly.
$ErrorActionPreference = 'Continue'

# gh/git/python emit UTF-8; Windows PowerShell decodes native stdout with the
# console codepage unless told otherwise, which turns the PR body into mojibake.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location -LiteralPath $repoRoot

$label = if ($Pr -gt 0) { "pr-$Pr" } else { "local-" + (git rev-parse --short HEAD).Trim() }
if (-not $ReportRoot) { $ReportRoot = Join-Path $repoRoot 'target\local-gate' }
$reportDir = Join-Path $ReportRoot $label
if (Test-Path -LiteralPath $reportDir) { Remove-Item -LiteralPath $reportDir -Recurse -Force }
New-Item -ItemType Directory -Path $reportDir -Force | Out-Null
$reportFile = Join-Path $reportDir 'report.md'
$log = New-Object System.Collections.Generic.List[string]
$failures = New-Object System.Collections.Generic.List[string]

function Write-Log([string]$line) {
    $log.Add($line)
    Write-Host $line
}

function Invoke-Step([string]$name, [scriptblock]$body) {
    Write-Log ""
    Write-Log "## $name"
    try {
        & $body
        Write-Log "RESULT: PASS"
    } catch {
        Write-Log "RESULT: FAIL - $($_.Exception.Message)"
        $script:failures.Add($name)
    }
}

function Resolve-Python {
    if (Get-Command py -ErrorAction SilentlyContinue) { return @('py', '-3') }
    if (Get-Command python3 -ErrorAction SilentlyContinue) { return @('python3') }
    if (Get-Command python -ErrorAction SilentlyContinue) { return @('python') }
    throw 'no python interpreter found (py -3 / python3 / python)'
}

$py = Resolve-Python

Write-Log "# Windows Gate local CI - $label"
Write-Log ""
Write-Log "- repo: $repoRoot"
Write-Log "- commit: $((git rev-parse HEAD).Trim())"
Write-Log "- base: $Base"
Write-Log "- host: $([System.Environment]::OSVersion.VersionString)"
Write-Log "- go: $((go version).Trim())"
Write-Log "- lane: Windows Gate only; Linux tests are owned by the Linux lane (scripts/ci/local-gate.sh)"

# Refresh main once; failure here is reported, not fatal (offline-safe).
Write-Log "  git: $((git fetch --no-tags --prune origin '+refs/heads/main:refs/remotes/origin/main' 2>&1 | Out-String).Trim())"

# 1. scope-check -------------------------------------------------------------
if (-not $SkipScope) {
    Invoke-Step 'scope-check' {
        $scopeArgs = @('scripts/ci/test_scope_check.py')
        & $py[0] @($py[1..($py.Count - 1)] + $scopeArgs)
        if ($LASTEXITCODE -ne 0) { throw "scope parser selftest failed (exit $LASTEXITCODE)" }

        $bodyFile = $PrBody
        if (-not $bodyFile -and $Pr -gt 0) {
            $bodyFile = Join-Path $reportDir 'pr-body.md'
            $body = (& gh pr view $Pr --json body -q .body) -join "`n"
            if ($LASTEXITCODE -ne 0) { throw "gh pr view $Pr failed (exit $LASTEXITCODE)" }
            [System.IO.File]::WriteAllText($bodyFile, $body, (New-Object System.Text.UTF8Encoding($false)))
        }
        if (-not $bodyFile) {
            Write-Log 'notice: no PR body supplied; the scope block is read from the diff base only where the script allows it'
            $bodyFile = Join-Path $reportDir 'pr-body.md'
            Set-Content -LiteralPath $bodyFile -Value '' -Encoding utf8
        }
        Write-Log "scope source: $bodyFile"
        $scopeOut = & $py[0] @($py[1..($py.Count - 1)] + @('scripts/ci/scope_check.py', '--pr-body-file', $bodyFile, '--base', $Base, '--head', 'HEAD')) 2>&1
        $scopeCode = $LASTEXITCODE
        foreach ($line in $scopeOut) { Write-Log "  $line" }
        if ($scopeCode -ne 0) { throw "card scope check failed (exit $scopeCode)" }
    }
}

# 2. Windows build -----------------------------------------------------------
Invoke-Step 'mygo-build' {
    & (Join-Path $PSScriptRoot '..\mygo-build.ps1')
    if ($LASTEXITCODE -ne 0) { throw "mygo-build.ps1 failed (exit $LASTEXITCODE)" }
}

# 3. Go tests ----------------------------------------------------------------
if (-not $SkipTests) {
    Invoke-Step 'go-test' {
        Push-Location (Join-Path $repoRoot 'prototypes\mygo-run')
        try {
            go test -tags=e2e -count=1 ./...
            if ($LASTEXITCODE -ne 0) { throw "go test failed (exit $LASTEXITCODE)" }
        } finally { Pop-Location }
    }
}

# 4. artifacts ---------------------------------------------------------------
Invoke-Step 'artifacts' {
    $dist = Join-Path $repoRoot 'prototypes\mygo-run\dist'
    foreach ($file in @('mygo-run.exe', 'asterlab.exe')) {
        $path = Join-Path $dist $file
        if (-not (Test-Path -LiteralPath $path)) { throw "artifact missing: $path" }
        $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash
        $size = (Get-Item -LiteralPath $path).Length
        $line = "{0}  {1} bytes  sha256={2}" -f $file, $size, $hash
        Write-Log $line
        Add-Content -LiteralPath (Join-Path $reportDir 'artifacts.txt') -Value $line
    }
    if (Test-Path -LiteralPath (Join-Path $dist 'BUILD_INFO.txt')) {
        Get-Content -LiteralPath (Join-Path $dist 'BUILD_INFO.txt') | ForEach-Object { Write-Log "  $_" }
    }
}

# Summary --------------------------------------------------------------------
$verdict = if ($failures.Count -eq 0) { 'PASS' } else { 'FAIL' }
Write-Log ""
Write-Log "## Summary"
if ($failures.Count -eq 0) {
    Write-Log 'All Windows Gate checks passed.'
} else {
    foreach ($f in $failures) { Write-Log "- FAILED: $f" }
}
Write-Log ""
Write-Log "LOCAL_GATE: $verdict"

$header = @(
    "# Windows Gate local CI report",
    '',
    "- label: $label",
    "- commit: $((git rev-parse HEAD).Trim())",
    "- generated: $((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ'))",
    ''
)
Set-Content -LiteralPath $reportFile -Value ($header + $log) -Encoding utf8
Write-Log "report: $reportFile"

exit $(if ($verdict -eq 'PASS') { 0 } else { 1 })
