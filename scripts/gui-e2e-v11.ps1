param(
  [string]$DataDir = "$env:TEMP\asterlab-v11-gui-e2e",
  [string]$Socket = "$env:TEMP\asterlab-v11-gui-e2e.sock",
  [Parameter(Mandatory=$true)][string]$SourceDb,
  [Parameter(Mandatory=$true)][string]$AppExe,
  [Parameter(Mandatory=$true)][string]$CliExe,
  [Parameter(Mandatory=$true)][string]$RunId,
  [string]$QpcrTaskLabel = "qpcr"
)
$ErrorActionPreference = 'Stop'
$null = New-Item -ItemType Directory -Force -Path $DataDir
Copy-Item -Force $SourceDb (Join-Path $DataDir 'labworkbench.sqlite')
Remove-Item -Force $Socket -ErrorAction SilentlyContinue
$env:LABWORKBENCH_DATA_DIR = $DataDir
$env:ASTERLAB_RUN_ID = $RunId
$env:MYGO_AUTOMATION = $Socket
function MGA { & mygo-automation @args }
function Snapshot { MGA snapshot }
function RefFor($Text, $Pattern) {
  $match = [regex]::Match([string]$Text, $Pattern, [System.Text.RegularExpressions.RegexOptions]::Multiline)
  if (-not $match.Success) { throw "Automation target not found: $Pattern" }
  return $match.Groups[1].Value
}
function SaveRef($Text) {
  $label = -join @([char]0x4fdd,[char]0x5b58,[char]0x8bfb,[char]0x6570)
  return RefFor $Text ('^(@e\d+) \[button\] "' + [regex]::Escape($label))
}
function PasteGrid($Text, $Payload, $ExpectedPreview) {
  $cell = RefFor $Text '^(@e\d+) \[textField\].*placeholder='
  Set-Clipboard -Value $Payload
  MGA press $cell --settle | Out-Host
  $wshell = New-Object -ComObject WScript.Shell
  $wshell.SendKeys('^v')
  Start-Sleep -Milliseconds 700
  $preview = Snapshot
  if ($preview -notmatch $ExpectedPreview) { throw "Paste preview not shown: $ExpectedPreview" }
  $apply = RefFor $preview '^(@e\d+) \[button\] "Apply paste"'
  MGA press $apply --settle | Out-Host
}
$proc = Start-Process -FilePath $AppExe -PassThru
try {
  Start-Sleep -Seconds 8
  MGA open --socket $Socket | Out-Host
  MGA wait | Out-Host
  $snap = Snapshot
  if ($snap -notmatch 'cck8\.plate96\.od450\.v1') { throw 'CCK-8 result schema not loaded' }
  $rows = @(); for ($r=0; $r -lt 8; $r++) { $row = @(); for ($c=0; $c -lt 12; $c++) { $row += [string]::Format([Globalization.CultureInfo]::InvariantCulture, '{0:0.00}', (($r * 12 + $c + 1) / 100.0)) }; $rows += ,($row -join "`t") }
  PasteGrid $snap ($rows -join "`n") 'Paste preview: 8.*12'
  $snap = Snapshot
  $cellRefs = [regex]::Matches([string]$snap, '^(@e\d+) \[textField\].*placeholder=', [System.Text.RegularExpressions.RegexOptions]::Multiline)
  if ($cellRefs.Count -lt 2) { throw 'CCK-8 grid cells not exposed to GUI automation' }
  MGA fill $cellRefs[0].Groups[1].Value '0.91' --settle | Out-Host
  MGA fill $cellRefs[1].Groups[1].Value '0.42' --settle | Out-Host
  MGA press (SaveRef (Snapshot)) --settle | Out-Host
  $saved = -join @([char]0x5df2,[char]0x4fdd,[char]0x5b58)
  if ((Snapshot) -notmatch [regex]::Escape($saved)) { throw 'CCK-8 batch save was not acknowledged' }
  MGA close | Out-Host
  Stop-Process -Id $proc.Id -Force
  Start-Sleep -Seconds 2
  Remove-Item -Force $Socket -ErrorAction SilentlyContinue

  $proc = Start-Process -FilePath $AppExe -PassThru
  Start-Sleep -Seconds 8
  MGA open --socket $Socket | Out-Host
  MGA wait | Out-Host
  $snap = Snapshot
  if ($snap -notmatch 'cck8\.plate96\.od450\.v1' -or $snap -notmatch '0\.91' -or $snap -notmatch '0\.42') { throw 'CCK-8 batch values did not restore after restart' }
  $input = Join-Path $DataDir 'workspace-input.json'
  Set-Content -Path $input -Value ('{"id":"' + $RunId + '"}')
  $cli = & $CliExe run.workspace --input $input
  if ($LASTEXITCODE -ne 0 -or $cli -notmatch '0\.91' -or $cli -notmatch '0\.42') { throw "CLI did not cross-read saved CCK-8 cells: $cli" }

  $qpcrButton = RefFor $snap ('^(@e\d+) \[button\] .*' + [regex]::Escape($QpcrTaskLabel))
  MGA press $qpcrButton --settle | Out-Host
  $qpcr = Snapshot
  if ($qpcr -notmatch 'qpcr\.sample_gene\.cq\.v1') { throw 'RT-qPCR schema not loaded for selected task' }
  PasteGrid $qpcr "23.5`t24.1`n22.8`t23.4" 'Paste preview: 2.*2'
  MGA press (SaveRef (Snapshot)) --settle | Out-Host
  if ((Snapshot) -notmatch [regex]::Escape($saved)) { throw 'RT-qPCR batch save was not acknowledged' }
  MGA close | Out-Host
  Write-Host 'GUI E2E PASS: 8x12 CCK-8 paste, two-cell edit, restart, CLI cross-read, RT-qPCR save'
} finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
