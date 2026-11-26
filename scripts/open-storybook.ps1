param(
    [switch]$Mobile,
    [switch]$Dark,
    [string]$Section = "",
    [switch]$Rebuild
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$ProtoDir = Join-Path $RepoRoot "prototypes\mygo-run"
$ExePath = Join-Path $ProtoDir "AsterLab-Storybook.exe"

$NeedBuild = $Rebuild -or (-not (Test-Path $ExePath))
if (-not $NeedBuild) {
    $ExeTime = (Get-Item $ExePath).LastWriteTimeUtc
    $WatchDirs = @(
        (Join-Path $ProtoDir "cmd\storybook"),
        (Join-Path $ProtoDir "internal\storybook"),
        (Join-Path $ProtoDir "internal\uikit\kit"),
        (Join-Path $ProtoDir "internal\theme"),
        (Join-Path $ProtoDir "internal\shell\storybook.go")
    )
    foreach ($w in $WatchDirs) {
        if (Test-Path $w) {
            $item = Get-Item $w
            if ($item.PSIsContainer) {
                $newer = Get-ChildItem -Path $w -Filter "*.go" -File | Where-Object { $_.LastWriteTimeUtc -gt $ExeTime } | Select-Object -First 1
                if ($newer) { $NeedBuild = $true; break }
            } elseif ($item.LastWriteTimeUtc -gt $ExeTime) {
                $NeedBuild = $true; break
            }
        }
    }
}

if ($NeedBuild) {
    Write-Host "Building standalone AsterLab-Storybook.exe..." -ForegroundColor Cyan
    Push-Location $ProtoDir
    try {
        & go build -ldflags="-H windowsgui" -o $ExePath ./cmd/storybook
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

$argsList = @()
if ($Mobile) { $argsList += "-mobile" }
if ($Dark) { $argsList += "-dark" }
if ($Section -ne "") { $argsList += @("-section", $Section) }

Write-Host "Launching $ExePath $($argsList -join ' ')" -ForegroundColor Green
Start-Process -FilePath $ExePath -ArgumentList $argsList | Out-Null
