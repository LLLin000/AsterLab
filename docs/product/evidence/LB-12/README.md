# LB-12 — 未标注资产的重设坐标接线

## 已实现与验证

为 Run 中已有但坐标为空（未标注）的关联资产接入就地重设坐标能力，调用 Core `RunService.UpdateAssetLinkCoordinates` 就地更新链接坐标，不改变文件身份与 SHA-256，也不重复建立新链接；遵循 `UX-P3`、`UX-P4`、`UX-P5`、`UX-STATE` 与 `UX-DONE`。

- **Build stamp**：`7ff96e1`（与截图采集时 `git rev-parse --short HEAD` 一致，见 `prototypes/mygo-run/dist/BUILD_INFO.txt`）
- **`mygo-run.exe` SHA-256**：`8919CE5EA600CAC76EE6C05AF949927490055F1FD8BC40790E0FC84457B385E3`
- **`Runs.dll` SHA-256**：`24751D853BA2F8B627BB96C88D5CF1F4956F0C95DAEF56B9C3720CB1DAF7D238`
- **采集环境**：Windows 真实窗口 `1360×880` 浅色主题（`light`），经 `mygo-automation` 驱动隔离测试数据库（无原生文件对话框交互）。

## 真实窗口截图

1. [unlabelled-row-1360x880-light.png](unlabelled-row-1360x880-light.png) — 未标注资产行保留说明文案并提供 `重设坐标` 动作。
2. [relabel-form-prefilled-1360x880-light.png](relabel-form-prefilled-1360x880-light.png) — 点击 `重设坐标` 后复用坐标表单 `openAssetLinkForm` 预填默认候选坐标（`A1`、`24 h`、`FOV 1`）。
3. [relabelled-matrix-1360x880-light.png](relabelled-matrix-1360x880-light.png) — 提交 `A1 / 24 h / FOV 2` 后，`a.runAssets` 刷新，孔位 × 时间点矩阵显示 `A1 × 24 h`（`FOV [2]`），资产行切换为可打开状态。

## 单元测试覆盖

- `internal/runworkspace/recording_test.go`：`TestUnlabelledAssetOffersRelabelActionAndLabelledDoesNot`（未标注资产提供 `重设坐标` 回调且不提供打开/重定位；已标注资产不显示 `重设坐标`）。
- `main_test.go`：
  - `TestRelabelUnlabelledAssetUpdatesRunAssetsAndMatrix`
  - `TestRelabelUnlabelledAssetRejectsInvalidCoordinateWithoutMutatingRunAssets`
  - `TestRelabelUnlabelledAssetSurfacesNotFoundWithoutMutatingRunAssets`
