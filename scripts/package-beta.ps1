$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$version = '0.1.0-beta'
$name = "AsterLab-$version-win-x64"
$out = Join-Path $root 'release'
$pkg = Join-Path $out $name
$zip = Join-Path $out "$name.zip"
$exe = Join-Path $root 'target\release\labworkbench.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw 'target/release/labworkbench.exe missing; run cargo build --release' }
if (Test-Path -LiteralPath $pkg) { Remove-Item -LiteralPath $pkg -Recurse -Force }
New-Item -ItemType Directory -Path $pkg | Out-Null
Copy-Item -LiteralPath $exe -Destination (Join-Path $pkg 'AsterLab.exe')
Copy-Item -LiteralPath (Join-Path $root 'docs\release\README-BETA.md') -Destination (Join-Path $pkg 'README.md')
Copy-Item -LiteralPath (Join-Path $root 'docs\release\START_HERE.md') -Destination (Join-Path $pkg 'START_HERE.md')
Copy-Item -LiteralPath (Join-Path $root 'docs\release\BACKUP_RESTORE.md') -Destination (Join-Path $pkg 'BACKUP_RESTORE.md')
Copy-Item -LiteralPath (Join-Path $root 'docs\KNOWN_LIMITATIONS.md') -Destination (Join-Path $pkg 'KNOWN_LIMITATIONS.md')
Copy-Item -LiteralPath (Join-Path $root 'docs\DELIVERY_ACCEPTANCE.md') -Destination (Join-Path $pkg 'ACCEPTANCE.md')
Copy-Item -LiteralPath (Join-Path $root 'docs\release\LICENSES.txt') -Destination (Join-Path $pkg 'LICENSES.txt')
Copy-Item -LiteralPath (Join-Path $root 'docs\release\START_HERE.bat') -Destination (Join-Path $pkg 'START_HERE.bat')
$examples = Join-Path $pkg 'examples'
$tools = Join-Path $pkg 'tools\backup-restore'
New-Item -ItemType Directory -Path $examples,$tools -Force | Out-Null
Copy-Item -Path (Join-Path $root 'docs\release\examples\*') -Destination $examples
Copy-Item -Path (Join-Path $root 'docs\release\backup-restore\*') -Destination $tools
Set-Content -LiteralPath (Join-Path $pkg 'VERSION.txt') -Value "AsterLab $version`ncommit $(git -C $root rev-parse --short HEAD)" -Encoding utf8
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -LiteralPath $pkg -DestinationPath $zip -CompressionLevel Optimal
Write-Output $zip
