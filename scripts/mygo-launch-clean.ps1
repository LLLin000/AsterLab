# Launch the unsigned prototype from a clean data directory copy.
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $repoRoot 'prototypes\mygo-run\dist\mygo-run.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw "missing $exe; run scripts/mygo-build.ps1 first" }
$src = 'D:\L\AI\Cindy\CellPlanner-ProductStudy\cellplanner-replica\target\asterlab-ui-runtime3'
if (-not (Test-Path -LiteralPath $src)) { throw "runtime source missing: $src" }
$clean = Join-Path $env:TEMP 'asterlab-mygo-clean'
if (Test-Path -LiteralPath $clean) { Remove-Item -LiteralPath $clean -Recurse -Force }
New-Item -ItemType Directory -Path $clean | Out-Null
Copy-Item -Path (Join-Path $src '*') -Destination $clean -Recurse
$env:LABWORKBENCH_DATA_DIR = $clean
$env:ASTERLAB_RUN_ID = '1c0c8989-0513-483a-851b-bdc7d6c67483'
Start-Process -FilePath $exe
Write-Output "launched from clean data dir: $clean"
