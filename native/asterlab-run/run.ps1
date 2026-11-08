$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$repo = Resolve-Path (Join-Path $root "../..")
$env:ASTERLAB_CLI = if ($env:ASTERLAB_CLI) { $env:ASTERLAB_CLI } else { Join-Path $repo "target/release/labworkbench.exe" }
$env:LABWORKBENCH_DATA_DIR = if ($env:LABWORKBENCH_DATA_DIR) { $env:LABWORKBENCH_DATA_DIR } else { Join-Path $repo "target/asterlab-ui-runtime3" }
$exe = Join-Path $root "target/release/asterlab-run.exe"
if (!(Test-Path $exe)) { throw "Build first: cargo build --release --manifest-path `"$root/Cargo.toml`"" }
Start-Process -FilePath $exe
