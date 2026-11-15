# LB-16 — 设计页校验态与错误出口

- 任务卡：`LB-16`（`9b818af4-174e-4dbd-97ec-30e700c7891e`）
- 规则依据：`docs/product/ux-contract.md` §3（`UX-P5`、`UX-STATE`、`UX-DONE`）及 `docs/product/evidence/LB-13/README.md` 第 5、10 项。

## 修复与变更位置

1. **Core 就绪评估错误显式化与 `校验中` / `未解析` 状态词**
   - `prototypes/mygo-run/main.go:2906-2928`（`refreshReadiness`）：记录 `a.readinessLoading` 与 `a.readinessErr`，`AssessAsterDoc` 或服务不可用失败时保留映射后的 Core 错误原因，不再静默置空吞错。
   - `prototypes/mygo-run/main.go:2930-2967`（`renderPreflight`）：在 `a.readinessLoading` 时显示 `校验中` 并禁用 `创建 Run`；评估缺失时显示 `未解析 · 无法获取 Core 校验结果`、完整 Core 错误文本与 `重试` 按钮。
   - `prototypes/mygo-run/design_page.go:68-94, 155-220`（`designReadinessState`、`asterDesignPage`）：顶栏始终显示 §3 状态词（`校验中`/`未解析`/`不完整`/`待确认`/`就绪`/`草稿`），`校验中` 时禁用 `创建 Run`；`a.readinessErr != ""` 时在横幅展示完整原因与 `重试`。
2. **完整错误展示与可重试出口**
   - `prototypes/mygo-run/design_page.go:31-48, 210-222`（`renderDesignErrorBanner`、`asterDesignPage`）：将 `a.createRunErr` 从顶栏 360px 单行省略移至自动换行的页面横幅并附带 `重试`（`createRunFromDesignDoc`）；`a.designDocErr` 横幅通过 `designDocRetryFunc` / `a.retryDesignDoc` 支持命令/撤销/重做/加载失败后的 `重试`。
3. **消除 `applyDesignRaw` 静默空操作**
   - `prototypes/mygo-run/main.go:2005-2018`：当 `a.designSvc == nil` 或 `a.designDoc == nil` 时写入 `a.designDocErr` 与 `a.err` 并绑定重试回调，不再静默返回 `false`。
4. **步骤属性抽屉草稿保留与切换警告**
   - `prototypes/mygo-run/main.go:44-49, 129-130`、`prototypes/mygo-run/design_page.go:249-299, 412-674`：新增 `stepDrafts`（按步骤 ID 保存未写入 Core 的属性草稿）、`selectAsterStep`、`renderUnsavedDraftWarning` 与 `stepPropertiesDrawer` 状态指示；未保存修改立即在抽屉、步骤行和顶栏显示 `未保存`，切换至其他步骤时保留原步骤草稿并给出警告横幅（支持 `切回草稿` / `放弃草稿`），保存后展示 `已保存`。
5. **后备 `ListView` 完整错误展示与重试**
   - `prototypes/mygo-run/internal/designer/step_list.go:289-368`（`listToolbar`、`listErrorBanner`、`retryListError`、`listStateChip`）：移除工具栏 `MaxWidth(360)` 单行截断，改为全宽自动换行的 `listErrorBanner` + `重试` 按钮，并在 `d.Loading` 时显示 `校验中` 状态词。

## 回归测试（`prototypes/mygo-run/main_test.go`）

- `TestDesignReadinessStatesAndRetryOnFailure`
- `TestDesignCreateRunCommandErrorsAndSilentNoOp`
- `TestDesignStepSwitchKeepsAndWarnsAboutPropertyDraft`
- `TestDesignerListViewFullErrorRetryAndLoadingState`

## 真窗口构建与证据（`1360x880`, `ASTERLAB_THEME=light`）

截图采集自 `prototypes/mygo-run/dist/mygo-run.exe`，对应 `dist/BUILD_INFO.txt` 构建戳（与采集时 `git rev-parse --short HEAD` 一致）：

```text
AsterLab MyGo Prototype (UNSIGNED)
commit f7d0a3d
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 B0FED3B6BE2973931ADE39EF29AF3D51572311A394EAA2E0E84DFB4F33123AE0
mygo-run.exe SHA256 FBA8C047CA47EB50FCAB920E5B81BA100588A9090F0D1CED05CD656E38E4873C
```

驱动方式：使用隔离的 `LABWORKBENCH_DATA_DIR`（`D:\.ci-cache\lb16-data\draft|assess-fail`）与 `mygo-automation` 在真实 Windows 窗口（`1360x880`，浅色主题）采集：

1. [draft-unsaved-1360x880-light.png](draft-unsaved-1360x880-light.png)
   - 编辑步骤 `细胞铺板` 的标题为 `细胞铺板（未保存草稿）` 且未点击 `保存属性`。
   - 顶栏、步骤行与右侧 `步骤属性` 抽屉均显示 `未保存` 状态词与保存提示，并提供 `保存属性` / `放弃修改`。
2. [draft-switch-warning-1360x880-light.png](draft-switch-warning-1360x880-light.png)
   - 在存在未保存属性草稿时点击切换到 `酶标仪读数`。
   - 列表顶部展示警告横幅 `未保存 · 步骤「细胞铺板」有未保存的属性草稿，已保留`（含 `切回草稿` / `放弃草稿`），原步骤保留 `未保存` 标记，切回后草稿完整恢复。
3. [readiness-failure-1360x880-light.png](readiness-failure-1360x880-light.png)
   - 在隔离 SQLite 中使 `asterdoc_version` 的修订号与文档记录错位，触发 `AssessAsterDoc` 返回 `NOT_FOUND: AsterDoc revision v1 does not exist`。
   - 顶栏显示状态词 `未解析`，错误横幅展示完整 Core 错误原因 `Core 校验失败：NOT_FOUND: AsterDoc revision v1 does not exist` 与 `重试` 按钮，步骤列表保持可读。
4. [preflight-readiness-failure-1360x880-light.png](preflight-readiness-failure-1360x880-light.png)
   - 点击 `创建 Run` 打开运行前检查弹窗。
   - 弹窗内显示 `未解析 · 无法获取 Core 校验结果`、完整 Core 错误原因 `NOT_FOUND: AsterDoc revision v1 does not exist` 及 `返回修改` / `重试` 按钮。
