# LB-11 — 坐标录入缺陷修复

## 缺陷与修复

1. **跨 Assay 时间点串扰与冻结时间点遗漏（`assetCoordinateOptions`）**
   - **缺陷**：此前 `assetCoordinateOptions` 从 `a.snap.Tasks` 的所有任务收集时间点而未按 `AssayID` 过滤，且仅在任务列表没有时间点时才读取 `frozenAssayTimepoints(designData, selectedAssay)`。导致多 Assay Run 中其他 Assay 的时间点可以通过 `validateAssetCoordinates` 校验；单 Assay Run 中若任务仅覆盖部分时间点，其余冻结时间点会被丢弃，导致矩阵缺口补齐预填报 `未知时间点`。
   - **修复**：先从 `taskKey` 定位 `selectedAssay`，仅收集 `task.AssayID == selectedAssay` 的任务时间点，再无条件并入 `frozenAssayTimepoints(designData, selectedAssay)`（兼容 `mygo.assays` 与顶层 `assays`），统一 `TrimSpace` 去重。其他 Assay 的时间点不再进入候选列表。
   - 同步更新 `TestOpenAssetLinkFormRetainsGapCoordinatesAndUsesRunTimepoints` 中 `task-2` 的 `AssayID: "assay"`，使其符合同 Assay 任务前提。

2. **默认时间点未去除首尾空白（`openAssetLinkForm`）**
   - **缺陷**：候选列表已对 `task.Timepoint` 执行 `strings.TrimSpace`，但 `openAssetLinkForm` 读取默认时间点时未去空白，导致带首尾空白的 `task.Timepoint` 无法通过精确匹配校验。
   - **修复**：对 `prefill.WellID`、`prefill.Timepoint` 以及回退读取的 `task.Timepoint` 均执行 `strings.TrimSpace`，确保默认值精确命中候选列表。

3. **批量文件关联非原子校验（`linkRunAsset` / `linkAssetPaths` / `submitAssetLinkForm`）**
   - **缺陷**：此前每选中一个文件即调用一次 `assets.NewLink` 与 `a.runSvc.SaveAssetLink`；若第 1 个文件保存成功而第 2 个文件校验失败（如越界 `rootDir`），第 1 个文件已持久化且 Core 尚无 unlink 接口；同时对话框在调用 `linkRunAsset` 前已关闭，错误未展示在表单错误行中。
   - **修复**：提取 `linkAssetPaths` 先对全部选中路径调用 `assets.NewLink` 完成前置校验，全部通过后再逐个调用 `a.runSvc.SaveAssetLink` 写入；提取 `submitAssetLinkForm`，任一校验或保存失败时保持对话框打开并在 `a.linkDialogErr`（表单错误行）显示原因（符合 `UX-P5`）。

## 回归测试（`prototypes/mygo-run/main_test.go`）

- `TestAssetCoordinateOptionsRejectsOtherAssayTimepoint`
- `TestAssetCoordinateOptionsIncludesAllFrozenTimepointsForGapPrefill`
- `TestOpenAssetLinkFormTrimsDefaultTaskTimepoint`
- `TestLinkAssetPathsRejectsOutOfBoundsBatchWithoutSavingFirstFile`

## 视觉说明

本次修复仅涉及坐标候选过滤、空白修剪与批量前置校验/表单错误保留逻辑，关联弹窗与 Run 界面的视觉布局未作任何改动，因此未重新截取屏幕截图（沿用 [LB-06 证据](../LB-06/README.md)）。
