# scripts/mygo-package-devbeta.ps1
# Builds the explicitly-labeled AsterLab BetaFeedback portable zip (-tags=devdebug)
# for invited beta testers. Public end-user installers (-tags=mygo_noinspector) remain 100% isolated.
param(
    [string]$Version = '0.1.0-unsigned'
)
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$protoDir = Join-Path $repoRoot 'prototypes\mygo-run'
$releaseDir = Join-Path $repoRoot 'release'
New-Item -ItemType Directory -Force -Path $releaseDir | Out-Null

$commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
$endpoint = if ($env:BETA_FEEDBACK_ENDPOINT) { $env:BETA_FEEDBACK_ENDPOINT.Trim() } else { 'https://dev.asterlab.party/api/feedback' }
$uploadKey = if ($env:BETA_FEEDBACK_KEY) { $env:BETA_FEEDBACK_KEY.Trim() } else { '' }

$ldflags = "-s -w -H=windowsgui " +
    "-X 'asterlab-mygo-prototype/internal/updater.Version=$Version-beta' " +
    "-X 'asterlab-mygo-prototype/internal/updater.Commit=$commit' " +
    "-X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadEndpoint=$endpoint' " +
    "-X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadKey=$uploadKey'"

$stageRoot = Join-Path $env:TEMP ("asterlab-betafeedback-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
$stageDir = Join-Path $stageRoot "AsterLab-BetaFeedback-$Version-win-x64"
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null

try {
    Push-Location $protoDir
    try {
        $env:CGO_ENABLED = '0'
        go build -tags="e2e,devdebug" -trimpath -ldflags="$ldflags" -o (Join-Path $stageDir 'AsterLab-BetaFeedback.exe') .
        if ($LASTEXITCODE -ne 0) { throw "go build BetaFeedback desktop failed (exit $LASTEXITCODE)" }
    } finally {
        Pop-Location
    }

    $readme = @"
AsterLab 内测反馈版 (BetaFeedback $Version)
==========================================
1. 双击运行 AsterLab-BetaFeedback.exe。
2. 遇到任何 UI / 交互 / 功能问题：
   - 按 Ctrl+Shift+D（或点击右下角「拾取细节」）框选问题区域，写下建议并保存；
   - 如需截取打开的弹窗/下拉菜单，点击「延迟2s(截弹窗)」或按 Ctrl+Shift+Y；
   - 每条批注自动实时落盘，即使意外崩溃重启也会自动保留全部批注与崩溃堆栈！
3. 全部记完后，点击右下角「队列」-> 填一下你的昵称 -> 点「一键提交给开发者」即可直达开发者。
   （若处于断网/内网环境，可点「导出并复制 .zip」，直接 Ctrl+V 发微信给开发者。）
"@
    Set-Content -LiteralPath (Join-Path $stageDir '内测使用说明.txt') -Value $readme -Encoding utf8

    $zipName = "AsterLab-BetaFeedback-$Version-win-x64.zip"
    $zipPath = Join-Path $releaseDir $zipName
    if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
    Compress-Archive -Path $stageDir -DestinationPath $zipPath -Force
    Write-Output "BetaFeedback Windows zip ready: $zipPath"
} finally {
    Remove-Item -LiteralPath $stageRoot -Recurse -Force -ErrorAction SilentlyContinue
}
