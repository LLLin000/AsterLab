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
function Snapshot { return (MGA snapshot | Out-String -Width 4096) }
function RefFor($Text, $Pattern) {
  $pattern = $Pattern.TrimStart('^')
  $match = [regex]::Match([string]$Text, $pattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
  if (-not $match.Success) { throw "Automation target not found: $Pattern`nSnapshot text:`n$Text" }
  return $match.Groups[1].Value
}
function SaveRef($Text) {
  return RefFor $Text '(@e\d+) \[button\] "[^"]*Ctrl\+S'
}
function PasteGrid($Text, $Payload, $ExpectedPreview) {
  $cell = RefFor $Text '^(@e\d+) \[textField\].*placeholder='
  Set-Clipboard -Value $Payload
  MGA focus $cell --settle | Out-Host
  MGA key Ctrl+V --settle | Out-Host
  Start-Sleep -Milliseconds 700
  $preview = Snapshot
  if ($preview -notmatch $ExpectedPreview) { throw "Paste preview not shown: $ExpectedPreview`nSnapshot text:`n$preview" }
  $apply = RefFor $preview '^(@e\d+) \[button\] "Apply paste"'
  MGA press $apply --settle | Out-Host
}
  $proc = Start-Process -FilePath $AppExe -PassThru -RedirectStandardOutput (Join-Path $DataDir 'gui-first.stdout.log') -RedirectStandardError (Join-Path $DataDir 'gui-first.stderr.log')
try {
  Start-Sleep -Seconds 8
  MGA open --socket $Socket | Out-Host
  MGA wait | Out-Host
  $snap = Snapshot
  if ($snap -notmatch 'cck8\.plate96\.od450\.v1') { throw "CCK-8 result schema not loaded; GUI snapshot:`n$snap" }
  $rows = @(); for ($r=0; $r -lt 8; $r++) { $row = @(); for ($c=0; $c -lt 12; $c++) { $row += [string]::Format([Globalization.CultureInfo]::InvariantCulture, '{0:0.00}', (($r * 12 + $c + 1) / 100.0)) }; $rows += ,($row -join "`t") }
  PasteGrid $snap ($rows -join "`n") 'Paste preview: 8.*12'
  $snap = Snapshot
  $cellRefs = [regex]::Matches([string]$snap, '(@e\d+) \[textField\].*?placeholder=', [System.Text.RegularExpressions.RegexOptions]::Singleline)
  if ($cellRefs.Count -lt 2) { throw 'CCK-8 grid cells not exposed to GUI automation' }
  MGA fill $cellRefs[0].Groups[1].Value '0.91' --settle | Out-Host
  MGA fill $cellRefs[1].Groups[1].Value '0.42' --settle | Out-Host
  MGA press (SaveRef (Snapshot)) --settle | Out-Host
  $snap = Snapshot
  if ($snap -notmatch 'cck8\.plate96\.od450\.v1' -or $snap -notmatch 'Saved') { throw "CCK-8 batch save not acknowledged:`n$snap" }
  MGA close | Out-Host
  Stop-Process -Id $proc.Id -Force
  Start-Sleep -Seconds 2
  Remove-Item -Force $Socket -ErrorAction SilentlyContinue

  $proc = Start-Process -FilePath $AppExe -PassThru -RedirectStandardOutput (Join-Path $DataDir 'gui-restart.stdout.log') -RedirectStandardError (Join-Path $DataDir 'gui-restart.stderr.log')
  Start-Sleep -Seconds 8
  MGA open --socket $Socket | Out-Host
  MGA wait | Out-Host
  $snap = Snapshot
  if ($snap -notmatch 'cck8\.plate96\.od450\.v1' -or $snap -notmatch '0\.91' -or $snap -notmatch '0\.42') { throw 'CCK-8 batch values did not restore after restart' }
  $input = Join-Path $DataDir 'workspace-input.json'
  Set-Content -Path $input -Value ('{"id":"' + $RunId + '"}')
  $cli = & $CliExe run.workspace --input $input
  if ($LASTEXITCODE -ne 0 -or $cli -notmatch '0\.91' -or $cli -notmatch '0\.42') { throw "CLI did not cross-read saved CCK-8 cells: $cli" }

  if ([regex]::IsMatch($snap, ' \[button\] "[^"]*' + [regex]::Escape($QpcrTaskLabel), [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)) {
    $qpcrButton = RefFor $snap ('^(@e\d+) \[button\] .*?' + [regex]::Escape($QpcrTaskLabel))
    MGA press $qpcrButton --settle | Out-Host
    $qpcr = Snapshot
    if ($qpcr -notmatch 'qpcr\.sample_gene\.cq\.v1') { throw 'RT-qPCR schema not loaded for selected task' }
    PasteGrid $qpcr "23.5`t24.1`n22.8`t23.4" 'Paste preview: 2.*2'
    MGA press (SaveRef (Snapshot)) --settle | Out-Host
    if ((Snapshot) -notmatch 'Saved') { throw 'RT-qPCR batch save was not acknowledged' }
    Write-Host 'GUI E2E PASS: 8x12 CCK-8, restart, CLI cross-read, RT-qPCR save'
  } else {
    Write-Host 'GUI E2E PASS: 8x12 CCK-8, restart, CLI cross-read (no qPCR task in selected run)'
  }
  MGA close | Out-Host
} finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
