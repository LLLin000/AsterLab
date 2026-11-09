# scripts/mygo-measure.ps1
# Measure cold start and working set for dist/mygo-run.exe (unsigned).
# Usage: .\scripts\mygo-measure.ps1 [-Seconds 10]
param([int]$Seconds = 10)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $repoRoot 'prototypes\mygo-run\dist\mygo-run.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw "missing $exe; run scripts/mygo-build.ps1 first" }

if (-not $env:LABWORKBENCH_DATA_DIR) {
    $src = 'D:\L\AI\Cindy\CellPlanner-ProductStudy\cellplanner-replica\target\asterlab-ui-runtime3'
    if (Test-Path -LiteralPath $src) { $env:LABWORKBENCH_DATA_DIR = $src }
}
if (-not $env:ASTERLAB_RUN_ID) { $env:ASTERLAB_RUN_ID = '1c0c8989-0513-483a-851b-bdc7d6c67483' }
$proc = Start-Process -FilePath $exe -PassThru
$start = Get-Date
Start-Sleep -Milliseconds 500
$peakWS = 0
$samples = 0
for ($i = 0; $i -lt ($Seconds * 2); $i++) {
    try {
        $p = Get-Process -Id $proc.Id -ErrorAction Stop
        if ($p.WorkingSet64 -gt $peakWS) { $peakWS = $p.WorkingSet64 }
        $samples++
    } catch { break }
    Start-Sleep -Milliseconds 500
}
$elapsed = ((Get-Date) - $start).TotalSeconds
try { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue } catch {}
$report = "cold_start_observed_s=$([math]::Round($elapsed,2))`nworking_set_peak_mb=$([math]::Round($peakWS/1MB,1))`nsamples=$samples`n"
Write-Output $report
Set-Content -LiteralPath (Join-Path (Split-Path $exe) 'MEASURE.txt') -Value $report -Encoding utf8
