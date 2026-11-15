# LB-17 — Run 网格与资产入口的错误出口

- 任务卡：`LB-17`（`e5a38151-6c7d-4114-9d46-64617554ca61`）
- 规则依据：`docs/product/ux-contract.md` §3（`UX-P5`、`UX-STATE`、`UX-DONE`）及 `docs/product/evidence/LB-13/README.md` 第 6、7、9 项。

## 修复与变更位置

1. **网格提交/保存失败显式进入 `StateError`、`保存中…` 可达、按原因展示错误文案**
   - `prototypes/mygo-run/internal/resultgrid/plate.go`（`setGridError`，第 479 行及孔板点击/提交/粘贴错误分支）：写入 `g.Error` 时同步将 `g.State` 置为 `resultgrid.StateError`，保证顶栏与错误横幅可渲染。
   - `prototypes/mygo-run/internal/runworkspace/recording.go`（`saveNow`、`Render`、`gridErrorPresentation`）：
     - `CommitPendingEdit()` 失败或表格 `SetCell` 失败时同步置为 `resultgrid.StateError` 并展示 `保存失败` 与 `重试保存`。
     - 在 `resultgrid.StateSaving` 时展示 `保存中…` 并禁用 `保存读数` 按钮，同时阻止重复触发保存。
     - 通过 `gridErrorPresentation` 按原因区分提示语（已写入 Core 但刷新失败 / 修订冲突 / Core 拒绝单元格 / 目录权限问题 / 本地单元格输入校验 / 其他 Core 错误原因），不再把所有失败一律归因为目标目录权限。
   - `prototypes/mygo-run/main.go`（`prepareBatchSaveForTask`、`executeBatchSave`、`applyBatchSaveOutcome`、`startSaveBatch`）：
     - 在调用 Core `SaveReadoutsBatch` 前将 `grid.State` 设为 `resultgrid.StateSaving`，并在已创建真实窗口时通过后台 goroutine + `mygo.RunOnMain` 执行以便真实渲染 `保存中…`。
     - 保留 Core 映射后的错误原因，并区分 `SaveReadoutsBatch` 失败、部分单元格被拒（`Accepted != totalCells`）与"已写入 Core，但刷新 Run 失败"。

2. **资产打开/重新定位/列表加载失败页内横幅与 `a.linkErr` 恢复**
   - `prototypes/mygo-run/internal/runworkspace/recording.go`（`AssetErr`、`OnRetryAssetErr`、`renderGallery`、`renderAssetErrBanner`）：在图库与文件区域旁渲染持久化页面横幅并附 `重试` 按钮。
   - `prototypes/mygo-run/main.go`（`loadRunAssets`、`tryOpenRunAsset`、`tryRelocateRunAsset`、`submitAssetLinkForm`）：
     - 将资产列表加载、`打开`（`openRunAsset`）、`重新定位`（`relocateRunAsset`）失败原因与重试回调写入 `a.assetErr` / `a.assetErrRetry`。
     - `submitAssetLinkForm` 失败时同时写入 `a.linkErr` 与 `a.linkErrTask`，使 `recording.go` 中的 `关联失败：... + 重新关联` 横幅重新可达。

3. **消除只写 `a.err`，将快照拒绝/冲突路由到可见横幅**
   - `prototypes/mygo-run/main.go`（`applySnap`、`renderSyncErrorBanner`、`contentPage`、`main`、`createAsterDocDesign`）：删除只写的 `app.err` 字段，改为将 `StaleGuard` 过期快照拒绝、`ApplySnapshot` 失败、同 Run 同修订冲突（`ConflictIfDifferent`）及服务初始化/创建错误写入 `a.syncErr`，并在 `contentPage` 顶部以 `renderSyncErrorBanner` 渲染完整原因及 `重新加载 Run` / `忽略` 操作；同时修复 `applyBatchSaveOutcome` 提前改写 `a.snap.Revision` 与初始占位 Run 误触发同修订冲突的问题。

## 回归测试（`prototypes/mygo-run/main_test.go`）

- `TestGridCommitFailureSetsStateErrorAndShowsBannerWithRetry`
- `TestSaveBatchPassesThroughStateSavingAndDistinguishesCauses`
- `TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable`
- `TestSnapshotRejectionAndConflictRenderedOnContentPage`

## 真窗口构建与证据（`1360x880`, `ASTERLAB_THEME=light`）

截图采集自 `prototypes/mygo-run/dist/mygo-run.exe`，对应 `dist/BUILD_INFO.txt` 构建戳（与采集时 `git rev-parse --short HEAD` 一致）：

```text
AsterLab MyGo Prototype (UNSIGNED)
commit 7c59194
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 50C5793F53C707C3EC1FB4699C194D5F3D914FDDDF15F92F880884FD295DD853
mygo-run.exe SHA256 3B562C3C2D8208BC210D07C461C07ECCF4029FD1F85E3343A4C4AC68E10D0353
```

驱动方式：使用隔离的 `LABWORKBENCH_DATA_DIR`（`D:\.ci-cache\lb17-data\grid-error|grid-saving|asset-error`）与 `mygo-automation` 在真实 Windows 窗口（`1360x880`，浅色主题）采集：

1. [grid-save-error-1360x880-light.png](grid-save-error-1360x880-light.png)
   - 在隔离 SQLite 持锁触发 Core 批量保存失败（超过 `busy_timeout` 后返回 `INTERNAL: database is locked`）。
   - 顶栏显示红色状态 `保存失败` 与 `重试保存` 按钮，错误横幅保留 Core 原始原因 `未保存 · INTERNAL: database is locked` 并给出准确提示语（不再误报权限不足），已录入孔位数值保持保留。
2. [grid-saving-1360x880-light.png](grid-saving-1360x880-light.png)
   - 在 SQLite 写锁暂停 Core 提交事务期间点击 `保存读数`。
   - 顶栏实时显示 `保存中…` 状态且 `保存读数` 按钮处于禁用状态；解锁提交完成后界面自动切换为 `已保存`。
3. [asset-open-error-1360x880-light.png](asset-open-error-1360x880-light.png)
   - 关联本地图像文件后改动文件内容使其 SHA256 与 Core 记录不一致，点击文件行的 `打开`。
   - 资产列表上方出现持久化页内横幅 `无法打开文件：文件校验失败：file content hash changed` 与 `重试` 按钮。
