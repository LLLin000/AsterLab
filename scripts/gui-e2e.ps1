param(
  [string]$DataDir = "O:\rc-gui\e2e-data",
  [string]$Socket = "O:\rc-gui\e2e.sock",
  [string]$SourceDb = "O:\rc-gui\target\asterlab-ui-runtime3\labworkbench.sqlite",
  [string]$AppExe = "O:\rc-gui\prototypes\mygo-run\mygo-run-e2e.exe",
  [string]$CliExe = "O:\rc-gui\target\release\labworkbench.exe",
  [string]$RunId = "1c0c8989-0513-483a-851b-bdc7d6c67483"
)
$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
Remove-Item -Force -Path (Join-Path $DataDir "labworkbench.sqlite") -ErrorAction SilentlyContinue
Copy-Item -Force $SourceDb (Join-Path $DataDir "labworkbench.sqlite")
Remove-Item -Force $Socket -ErrorAction SilentlyContinue

$env:LABWORKBENCH_DATA_DIR = $DataDir
$env:MYGO_AUTOMATION = $Socket
$proc = Start-Process -FilePath $AppExe -PassThru
Start-Sleep -Seconds 10

function MGA { & mygo-automation @args }
MGA open --socket $Socket | Out-Host
MGA wait | Out-Host

# Fill two readouts using first two visible sample/value text fields.
$snap = MGA snapshot
$sampleRefs = ($snap | Select-String '^(@e\d+) \[textField\] value="[^"]*" placeholder="样本编号' | ForEach-Object { $_.Matches[0].Groups[1].Value } | Select-Object -First 2)
$valueRefs  = ($snap | Select-String '^(@e\d+) \[textField\] value="[^"]*" placeholder="吸光度数值' | ForEach-Object { $_.Matches[0].Groups[1].Value } | Select-Object -First 2)
MGA fill $sampleRefs[0] "Control-1" --settle | Out-Host
MGA fill $valueRefs[0] "0.82" --settle | Out-Host
MGA fill $sampleRefs[1] "E1-1" --settle | Out-Host
MGA fill $valueRefs[1] "0.41" --settle | Out-Host

$snap2 = MGA snapshot
$saveRef = ($snap2 | Select-String '^(@e\d+) \[button\] "保存读数' | ForEach-Object { $_.Matches[0].Groups[1].Value } | Select-Object -First 1)
MGA press $saveRef --settle | Out-Host
$snap3 = MGA snapshot
if (-not ($snap3 -match "已保存")) { throw "save badge not found" }
MGA close | Out-Host

# Full exit + relaunch
Stop-Process -Id $proc.Id -Force
Start-Sleep -Seconds 3
Remove-Item -Force $Socket -ErrorAction SilentlyContinue
$proc2 = Start-Process -FilePath $AppExe -PassThru
Start-Sleep -Seconds 10
$env:LABWORKBENCH_DATA_DIR = $DataDir
$env:MYGO_AUTOMATION = $Socket
MGA open --socket $Socket | Out-Host
MGA wait | Out-Host
$snap4 = MGA snapshot
if (-not ($snap4 -match 'value="Control-1"')) { throw "restore missing Control-1" }
if (-not ($snap4 -match 'value="E1-1"')) { throw "restore missing E1-1" }

# CLI cross-read
$input = Join-Path $DataDir "workspace-input.json"
Set-Content -Path $input -Value ('{"id":"' + $RunId + '"}')
$env:LABWORKBENCH_DATA_DIR = $DataDir
$out = & $CliExe run.workspace --input $input
if (-not ($out -match '"ok":true')) { throw "CLI cross-read failed: $out" }
if (-not ($out -match 'E1-1')) { throw "CLI missing E1-1" }

Stop-Process -Id $proc2.Id -Force
Write-Host "GUI E2E PASS"
