# Restart the running unsigned prototype (kill then relaunch clean).
$ErrorActionPreference = 'Stop'
Get-Process -Name 'mygo-run' -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1
& (Join-Path (Split-Path -Parent $PSScriptRoot) 'mygo-launch-clean.ps1')
