$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:LABWORKBENCH_PORT = if ($env:LABWORKBENCH_PORT) { $env:LABWORKBENCH_PORT } else { '8832' }
$env:LABWORKBENCH_DATA_DIR = if ($env:LABWORKBENCH_DATA_DIR) { $env:LABWORKBENCH_DATA_DIR } else { Join-Path $PSScriptRoot 'data' }
cargo build --release
if ($LASTEXITCODE -ne 0) { throw 'LabWorkbench build failed.' }
$exe = Join-Path (Join-Path $PSScriptRoot 'target') (Join-Path 'release' 'labworkbench.exe')
if (-not (Test-Path -LiteralPath $exe)) { throw 'Built LabWorkbench executable was not found.' }
& $exe