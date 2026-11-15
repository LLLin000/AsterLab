# LB-14 — 加载与写入失败必须可见（真窗口证据）

## LB-13 阻断缺陷

1. **`SaveChecks` 写入失败静默吞错**：`runworkspace.Callbacks.SaveChecks` 无返回值；`main.go` 仅记录日志并写入未渲染的 `a.err`，既未在检查清单旁展示 Core 错误原因，也未将复选框回退至最近已持久化状态或提供重试操作。
2. **`applyRun(nil, err)` 使 Run 面板卡在加载占位**：加载失败时仅更新旧 `a.ws` / `a.err`，未将 `a.runWSVM` / `a.runWS` 切换为错误态，导致 `runworkspace.View` 持续停留在“正在加载实验运行数据…”。
3. **Design 读取失败不可见且遮蔽已加载数据**：`LoadDesign` 失败仅写入 `a.err` 并直接返回；`LoadAsterDoc` 失败会进入不可重试的空白页并隐藏已加载的 `designer` / `designDoc`。

## 变更位置（`file:line`）

- `prototypes/mygo-run/internal/runworkspace/view.go`
  - `Callbacks.SaveChecks` 返回 `error`，新增 `Callbacks.RetryLoad`（`:22-25`）；`State` 新增 `LoadError`、`CheckError`、`CheckErrorTask`、`pendingCheckAcks`（`:81-87`）。
  - `(*State).SyncFrom` 在非 error 态清空 `LoadError` 并同步检查项状态（`:162-177`）。
  - `View` 在 `(s.LoadError != "" || s.VM.Status == "error") && len(s.VM.Steps) == 0` 时渲染 `runLoadErrorState`，有步骤时渲染 `runLoadErrorBanner`，禁止卡在加载占位（`:308-320`, `:1193-1228`）。
  - `checkList` 在复选框变化时调用 `s.saveChecks`，失败时通过 `persistChecks` / `revertChecks` 回退复选框并在清单旁显示 `"未保存 · " + s.CheckError` 与“重试”按钮（`retryChecks`）（`:1105-1191`）。
- `prototypes/mygo-run/main.go`
  - `(*app).retryLoadRun` 支持对失败的 Run 加载发起重试（`:1185-1199`）。
  - `(*app).loadDesign` 与 `(*app).retryDesignID`、`(*app).selectDesign` 保留已加载的 `designer` / `designDoc`，记录 `a.designLoadID`、`a.designLoadErr`、`a.designDocLoadFailed`（`:1578-1675`, `:1804-1824`）。
  - `(*app).applyRun` 在失败时同步 `a.snap`、`a.runWSVM` 为 `Status: "error"` 并设置 `a.runWS.LoadError`（`:1889-1911`）。
  - `(*app).saveRunChecks` 返回映射后的 Core 错误并接通 `rwcb.SaveChecks` / `rwcb.RetryLoad`（`:2308-2313`, `:2482-2515`）。
- `prototypes/mygo-run/design_page.go`
  - 新增 `renderDesignErrorBanner` 在页面顶部展示错误原因与“重试”按钮（`:31-48`）。
  - `designPage` 与 `asterDesignPage` 在 `a.designer != nil` 或 `a.designDoc != nil` 时保留已有设计界面可见并叠加错误横幅；无已加载数据时展示带“重试”的错误页（`:50-97`, `:165-170`）。

## 回归测试名称

- `prototypes/mygo-run/internal/runworkspace/view_test.go`
  - `TestSaveChecksFailureShowsErrorRevertsCheckboxAndRetries`
  - `TestRunLoadErrorShowsReasonRetryAndNeverShowsLoading`
- `prototypes/mygo-run/main_test.go`
  - `TestSaveRunChecksFailureRevertsAndRetriesThroughApp`
  - `TestApplyRunLoadErrorUpdatesRunWorkspaceAndRetries`
  - `TestLoadDesignFailuresVisibleKeepLoadedAndRetry`

## 真窗口构建与证据（`1360x880`, `ASTERLAB_THEME=light`）

截图采集自 `prototypes/mygo-run/dist/mygo-run.exe`，对应 `dist/BUILD_INFO.txt` 构建戳（与采集时 `git rev-parse --short HEAD` 一致）：

```text
AsterLab MyGo Prototype (UNSIGNED)
commit c489778
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 FAC92FE5FBFB8C07421115B66B71CD9BB73F4DFBC6AA77D6E22541BB60FD367C
mygo-run.exe SHA256 6B9D35C3C89572F972B9EA51CC66037931217E845BBBA8C5CD1A371723640AC5
```

驱动方式：使用隔离的 `LABWORKBENCH_DATA_DIR`（`D:\.ci-cache\lb14-data\state1|state2|state3`）与 `mygo-automation` 在真实 Windows 窗口（`1360x880`，浅色主题）采集：

1. [run-load-error-1360x880-light.png](run-load-error-1360x880-light.png)
   - `ASTERLAB_RUN_ID=missing-run-lb14` 指向不存在的 Run ID。
   - 实验运行页显示“实验运行加载失败”、Core 原因 `INTERNAL: 记录不存在：missing-run-lb14` 与“重试”按钮，未停留于“正在加载实验运行数据…”。
2. [checks-save-error-1360x880-light.png](checks-save-error-1360x880-light.png)
   - 先正常加载含“台面已消毒”检查项的 Run，再从隔离 SQLite 中移除该 Run 记录后点击复选框。
   - 复选框回退为未勾选，检查清单旁显示 `未保存 · INTERNAL: 实验运行不存在` 与“重试”按钮。
3. [design-read-error-1360x880-light.png](design-read-error-1360x880-light.png)
   - 先通过 `ASTERLAB_DESIGN_ID` 加载“LB-14 已加载设计 A”，再从隔离 SQLite 中移除“LB-14 读取失败设计 B”后在侧边栏点击切换。
   - 顶部横幅显示 `设计加载失败：NOT_FOUND: design not found` 与“重试”按钮，此前已加载的设计 A（`AsterDoc 设计` `rev 1` 及步骤“酶标仪读数”）保持可见。
