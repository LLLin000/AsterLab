# LB-13 — V2 三板块状态词与错误可见性审计

- 任务卡：`LB-13 (Docs) V2 三板块状态词与错误可见性审计`（`f26c169b-6290-4de9-832a-1aa5da3f8e31`）
- 审计范围：AsterLab V2 三大核心板块 **Design**（`design_page.go`、`design_canvas.go`、`internal/designer/`）、**Run**（`run_page.go`、`internal/runworkspace/`、`internal/resultgrid/`）、**Notebook**（`notebook_page.go`、`internal/notebook/`）及 `prototypes/mygo-run/main.go` 对应状态绑定。
- 审计基线：branch `agent/LB-13-audit`，已合并 `origin/main`（`7ff96e1`，含 `LB-06`、`LB-07`、`LB-10`、`LB-11`）。本任务为只读审计，**未修改任何 Go 代码**。
- 规则来源：仅引用 `docs/product/ux-contract.md` §7 定义的规则 ID（`UX-IA-<n>`、`UX-P<1-5>`、`UX-STATE`、`UX-LAYOUT`、`UX-DONE`）。

---

## 按严重程度排序的缺陷清单（共 10 项）

### 阻断（3 项）

#### 1. [Run V2] `SaveChecks` 写入失败完全静默，无错误原因、无重试，界面状态与 Core 事实脱节
- **位置**：`prototypes/mygo-run/internal/runworkspace/view.go:22`, `1073-1079`；`prototypes/mygo-run/main.go:2253-2257`
- **代码证据**：
  ```go
  // prototypes/mygo-run/internal/runworkspace/view.go:22
  SaveChecks func(taskKey string, checks []CheckAckView)

  // prototypes/mygo-run/internal/runworkspace/view.go:1076-1078
  box := components.Checkbox(c, on, labelOr(check.Label, check.CheckID))
  if box.Changed() && s.Callbacks.SaveChecks != nil {
      s.Callbacks.SaveChecks(task.TaskKey, s.acks(task))
  }

  // prototypes/mygo-run/main.go:2253-2257
  if err != nil {
      log.Printf("save task checks: %v", err)
      a.err = state.MapError(err).Error()
      return
  }
  ```
- **症状**：执行确认勾选框回调 `SaveChecks` 没有返回 `error`；当 Core 拒绝保存确认项时，`main.go:2255` 仅写入从未在任何 UI 组件中读取渲染的 `a.err`（未写入 `a.shell.Error` 或 `runworkspace.State`），且未重置 `s.Checks`。操作者在界面上看到确认项仍保持“已勾选”，没有任何失败原因、未保存提示或重试控件。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：阻断
- **当前状态**：**仍然存在**（`LB-06` / `LB-07` / `LB-11` 均未覆盖 `runworkspace` 的 `SaveChecks` 错误路径）。

#### 2. [Run V2] `LoadRun` / `applyRun` 失败后画面永久停留在“正在加载…”，隐藏加载错误
- **位置**：`prototypes/mygo-run/main.go:2070`, `1835-1843`；`prototypes/mygo-run/internal/runworkspace/view.go:292-297`
- **代码证据**：
  ```go
  // prototypes/mygo-run/main.go:2070
  snap: runs.RunSnapshot{Title: "加载中", Status: "loading"},

  // prototypes/mygo-run/main.go:1835-1843
  func (a *app) applyRun(snap *runs.RunSnapshot, err error) {
      if err != nil {
          if a.ws != nil {
              a.ws.State = runui.StateBlocked
              a.ws.Error = err.Error()
          }
          a.err = err.Error()
          return
      }

  // prototypes/mygo-run/internal/runworkspace/view.go:292-296
  if s.VM.Status == "loading" {
      ui.Row(c).Fill().Center().Children(func() {
          ui.Text(c, "正在加载实验运行数据…").FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted)
      })
      return
  }
  ```
- **症状**：应用初始化时 `a.snap.Status` 为 `"loading"`；当 Core 读取 Run 失败触发 `applyRun(nil, err)` 时，仅更新旧版 `a.ws` 和不被渲染的 `a.err`，既不更新 `a.runWSVM` / `a.runWS`，也不写入 `a.shell.Error`。Run V2 主工作区会无限停留在 `"正在加载实验运行数据…"`，无错误原因且无重试入口。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：阻断
- **当前状态**：**仍然存在**（`LB-06` / `LB-07` / `LB-11` 未同步 `applyRun` 错误至 `runworkspace`）。

#### 3. [Design V2] `LoadDesign` 失败静默返回；`LoadAsterDoc` 失败退回无重试的错误空白页
- **位置**：`prototypes/mygo-run/main.go:1574-1578`, `1603-1605`；`prototypes/mygo-run/design_page.go:36-43`
- **代码证据**：
  ```go
  // prototypes/mygo-run/main.go:1574-1578
  snap, err := a.designSvc.LoadDesign(context.Background(), id)
  if err != nil {
      a.err = state.MapError(err).Error()
      return
  }

  // prototypes/mygo-run/main.go:1603-1605
  if doc, docErr := a.designSvc.LoadAsterDoc(context.Background(), id); docErr != nil {
      a.designDocErr = state.MapError(docErr).Error()

  // prototypes/mygo-run/design_page.go:36-43
  if a.designDocErr != "" {
      ui.Column(c).Fill().Center().Gap(c.Theme().Space(2)).Children(func() {
          ui.Text(c, "AsterDoc 设计无法读取").FontSize(16).Bold().TextColor(c.Theme().Danger)
          ui.Text(c, a.designDocErr).FontSize(13).TextColor(c.Theme().TextMuted)
          renderAgentProposal(c,a)
      })
      return
  }
  ```
- **症状**：(1) `LoadDesign` 失败只写不可见的 `a.err` 并提前 `return`，不更新 `a.designDocErr` 或 `a.shell.Error`；(2) `LoadAsterDoc` 失败且 `a.designDoc == nil` 时，`designPage` 渲染仅有文案的错误空白页 `"AsterDoc 设计无法读取"`，缺少重试按钮，且不再渲染第 1573 行已加载的 `a.designer` 后备视图。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：阻断
- **当前状态**：**仍然存在**（`LB-07` 仅处理已打开文档后的步骤命令错误）。

---

### 明显（5 项）

#### 4. [Notebook V2] 写入失败仅显示“未保存”且无重试按钮；切换 Run 或重启静默丢弃草稿
- **位置**：`prototypes/mygo-run/notebook_page.go:80-82`, `165-169`, `276-281`, `606-609`, `652-655`
- **代码证据**：
  ```go
  // prototypes/mygo-run/notebook_page.go:165-169
  if a.noteDirty || a.noteErr != "" {
      ui.Text(c, notebook.SaveStateLabel(true)).FontSize(float32(tokens.TextData)).Bold().TextColor(tokens.WarningText)
  } else {
      ui.Text(c, notebook.SaveStateLabel(false)).FontSize(float32(tokens.TextData)).Bold().TextColor(tokens.SuccessText)
  }

  // prototypes/mygo-run/notebook_page.go:276-280
  if a.noteErr != "" {
      ui.Row(c).FillWidth().Padding(6, 10).Radius(6).
          Background(theme.GetTokens(t).DangerBg).Children(func() {
          ui.Text(c, a.noteErr).FontSize(12).TextColor(theme.GetTokens(t).DangerText)
      })
  }

  // prototypes/mygo-run/notebook_page.go:652-655
  if err != nil {
      a.noteErr = state.MapError(err).Error()
      a.noteState = "error"
      return
  }

  // prototypes/mygo-run/notebook_page.go:80-82 & 606-609
  if a.noteRunID != a.snap.RunID {
      a.loadNotebook()
  }
  a.noteDraft = ""
  a.noteDirty = false
  ```
- **症状**：(1) `saveNotebook` 失败时设置的 `a.noteState = "error"` 在全仓从未被读取显示；工具栏仍只用 `notebook.SaveStateLabel(true)` 输出 `"未保存"`，且红色错误横幅（276-280 行）没有重试按钮；(2) 当 `a.noteDirty == true` 时切换 Run，`loadNotebook()` 无条件覆盖 `a.noteDraft` 并重置 `a.noteDirty = false`，且 Notebook 没有磁盘草稿持久化（不像 Run 网格有 `runworkspace.SaveGridDraft`），导致切页/重启静默丢失编辑内容。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：明显
- **当前状态**：**仍然存在**（`UX-06` 补齐了 `已保存`/`未保存` 词表，但未解决失败重试与草稿持久保留）。

#### 5. [Design V2] Core 就绪评估错误静默吞并、缺 `校验中`/`未解析` 态、多处错误无重试，切换步骤静默丢弃属性草稿
- **位置**：`prototypes/mygo-run/main.go:1885-1888`, `2719-2730`, `2744-2747`；`prototypes/mygo-run/design_page.go:83-97`, `112-126`, `233-236`, `384-416`
- **代码证据**：
  ```go
  // prototypes/mygo-run/main.go:2724-2728
  assessment, err := a.designSvc.AssessAsterDoc(context.Background(), a.designDoc.DesignID, a.designDoc.Revision)
  if err != nil {
      a.readiness = nil
      return
  }

  // prototypes/mygo-run/main.go:2744-2746
  if assessment == nil {
      ui.Text(c, "无法获取 Core 校验结果").FontSize(12).TextColor(t.Danger)
      return
  }

  // prototypes/mygo-run/design_page.go:83-97
  if a.readiness != nil {
      label, color := "草稿", t.TextMuted
      switch a.readiness.State {
      case "incomplete":
          label = "不完整"
      ...
  }

  // prototypes/mygo-run/design_page.go:112-115
  if a.createRunErr != "" {
      ui.Text(c, a.createRunErr).FontSize(11).TextColor(t.Danger).
          SingleLine().Ellipsis("…").MaxWidth(360)
  }

  // prototypes/mygo-run/design_page.go:233-236
  if components.Button(c, title, components.ButtonSecondary).Clicked() {
      a.designSelected = id
      a.canvasSelLoaded = ""
  }
  ```
- **症状**：(1) `AssessAsterDoc` 失败被 `refreshReadiness()` 静默置 `nil`，顶栏完全隐藏状态词，预检弹窗只显示 `"无法获取 Core 校验结果"`（无 `err.Error()` 原因与重试按钮），且缺失 `校验中` 与 `未解析` 状态词；(2) `a.createRunErr` 在顶栏被 360px 单行省略截断且无重试，`a.designDocErr` 横幅（122-125 行）只有错误信息无重试；(3) `applyDesignRaw` 在服务/文档为空时静默返回 `false`（`main.go:1886-1888`）；(4) 用户在右侧 `stepPropertiesDrawer` 修改属性但未点击 `"保存属性"` 前缺乏 `未保存` 提示（保存后也无 `已保存` 反馈），且点击另一步骤时第 235 行重置 `a.canvasSelLoaded = ""`，直接静默丢弃未保存的属性输入。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：明显
- **当前状态**：**仍然存在**（`LB-07` 新增的 `stepPropertiesDrawer` 与命令错误横幅未包含上述状态词、重试与未保存草稿保护）。

#### 6. [Run V2] 结果网格挂起编辑校验报错绕过顶部错误横幅，错误提示硬编码“目录权限”，且 `StateSaving` 为死代码
- **位置**：`prototypes/mygo-run/internal/runworkspace/recording.go:113-116`, `142-145`, `171-188`；`prototypes/mygo-run/internal/resultgrid/plate.go:480-483`；`prototypes/mygo-run/main.go:2346-2350`, `2470-2486`
- **代码证据**：
  ```go
  // prototypes/mygo-run/internal/runworkspace/recording.go:113-116
  if err := r.Grid.CommitPendingEdit(); err != nil {
      r.Grid.Error = err.Error()
      return
  }

  // prototypes/mygo-run/internal/runworkspace/recording.go:180-187
  if r.Grid != nil && r.Grid.State == resultgrid.StateError && r.Grid.Error != "" {
      ui.Column(c)...Children(func() {
          ui.Text(c, "未保存 · "+r.Grid.Error)...
          ui.Text(c, "检查目标目录权限后，点击重试保存。")...
      })
  }

  // prototypes/mygo-run/main.go:2346-2350
  OnSaveBatch: func(cells []resultgrid.Cell) {
      if grid != nil {
          cb.SaveBatch(grid.Schema, cells)
      }
  }
  ```
- **症状**：(1) 点击 `"保存读数"`（113 行）、切换表格/孔板视图（142 行）或粘贴（`plate.go:480`）时，若 `CommitPendingEdit()` 失败，代码只写 `r.Grid.Error` 而未置 `r.Grid.State = resultgrid.StateError`，导致顶部错误横幅与 `"重试保存"` 按钮不出现；(2) `OnSaveBatch` 同步调用 `cb.SaveBatch` 从未写入 `resultgrid.StateSaving`，`recording.go:171-174` 的 `"保存中…"` 状态无法触发；(3) Core 返回修订冲突、部分单元格拒绝（`main.go:2470-2476`）或保存成功但 `LoadRun` 刷新失败（`main.go:2478-2485`）时，横幅仍固定提示 `"检查目标目录权限后，点击重试保存。"`, 误导排查方向，且在最后一种情形中掩盖了“写入已成功、仅刷新失败”的事实。
- **违反规则**：`UX-P5`, `UX-STATE`
- **严重程度**：明显
- **当前状态**：**仍然存在**（`LB-06` / `LB-07` / `LB-11` 未改动结果网格的保存状态机）。

#### 7. [Run V2] 打开/重定位资产失败只弹 Toast，主界面 `LinkErr` 成死路径，且 `未标注` 资产入口仍被硬编码阻塞
- **位置**：`prototypes/mygo-run/main.go:2320`, `2328-2339`, `2578-2585`；`prototypes/mygo-run/internal/runworkspace/recording.go:448-458`, `599-606`
- **代码证据**：
  ```go
  // prototypes/mygo-run/main.go:2328-2338
  OnOpenAsset: func(asset runworkspace.LinkedAsset) {
      if err := a.openRunAsset(asset); err != nil {
          c.AddToast(ui.Toast{Title: "无法打开文件", Description: err.Error(), Type: components.ToastError})
      }
  },
  OnRelocate: func(asset runworkspace.LinkedAsset) {
      if err := a.relocateRunAsset(asset); err != nil {
          c.AddToast(ui.Toast{Title: "文件重新定位失败", Description: err.Error(), Type: components.ToastError})
      }

  // prototypes/mygo-run/main.go:2577-2580
  a.linkDialogErr = ""
  a.linkErr = ""
  if err := linker(a.linkDialogTask, coordinate); err != nil {
      a.linkDialogErr = err.Error()

  // prototypes/mygo-run/internal/runworkspace/recording.go:599-606
  if a.WellID == "" || a.Timepoint == "" || a.FOVIndex == 0 {
      ui.Text(c, "未标注 · 需要 Core 提供坐标更新/解除关联接口").
          FontSize(t.Rem(.68)).Bold().TextColor(tokens.WarningText)
  }
  if a.WellID == "" || a.Timepoint == "" || a.FOVIndex == 0 {
      return
  }
  ```
- **症状**：(1) 打开或重定位文件失败仅显示瞬态 Toast，页面内不保留原因和重试按钮；(2) `submitAssetLinkForm` 清空 `a.linkErr` 并只写 `a.linkDialogErr`，经全仓检索 `a.linkErr` 已无任何赋值点，导致 `recording.go:448-457` 的 `关联失败：…` + `重新关联` 横幅不可达；(3) `LB-10` 虽已合入 `main`（`7ff96e1`），但 `recording.go:599-606` 仍硬编码 `"未标注 · 需要 Core 提供坐标更新/解除关联接口"` 并在第 605 行直接 `return`，未标注文件仍只读，且文件缺失时也无法看到 `"未解析 · 文件不可用"` 与 `"重新定位"`。
- **违反规则**：`UX-P5`, `UX-STATE`, `UX-DONE`
- **严重程度**：明显
- **当前状态**：**部分已由 LB-11 修复，上述三项残留仍然存在**（`LB-11` 修复了弹窗内 `a.linkDialogErr` 错误保留与原子批量校验；上述 Toast、死 `a.linkErr` 与 `未标注` 只读截断仍存在）。

#### 8. [Run V2 & Notebook V2] 默认视图暴露非 §3 状态词、英文标题与原始内部标识
- **位置**：`prototypes/mygo-run/internal/runworkspace/view.go:1080-1089`, `1164`, `1271`；`prototypes/mygo-run/notebook_page.go:90`, `395`, `467`
- **代码证据**：
  ```go
  // prototypes/mygo-run/internal/runworkspace/view.go:1080-1089
  statusText, statusColor := "未知", t.TextMuted
  switch check.Status {
  case "passed": ...
  case "failed": ...
  case "pending": ...
  }

  // view.go:1164 & 1271
  ui.Text(c, fmt.Sprintf("%s · %s", check.CheckID, check.Status))
  ui.Text(c, "Batch "+event.BatchID)

  // notebook_page.go:90, 395, 467
  ui.Text(c, fmt.Sprintf("Document Revision: %d", a.snap.DesignRevision))
  ui.Text(c, "还没有内容。切换到“编辑”开始写。")
  ui.Text(c, entry.SourceType+" "+entry.SourceID)
  ```
- **症状**：(1) Run 检查列表把 `pending`/`passed`/`failed` 之外的状态映射为 `"未知"`（非 §3 状态词），操作历史与任务历史卡片直接显示 `check.CheckID · check.Status` 与 `"Batch "+event.BatchID`；(2) Notebook 顶部栏输出英文 `"Document Revision: %d"`，时间线每一行默认渲染 `entry.SourceType+" "+entry.SourceID`（原始来源类型与 UUID/ID），且只读空态文案 `"还没有内容。切换到“编辑”开始写。"` 缺少 §3 状态词（`未开始`/`草稿`）并与默认 `"实时"` 标签不匹配。
- **违反规则**：`UX-P2`, `UX-STATE`, `UX-DONE`
- **严重程度**：明显
- **当前状态**：**仍然存在**（`LB-06` / `LB-07` / `LB-11` 未涉及以上文本投影）。

---

### 轻微（2 项）

#### 9. [跨板块写入/同步] 过期快照与快照冲突错误写入从未渲染的 `a.err`
- **位置**：`prototypes/mygo-run/main.go:292-304`
- **代码证据**：
  ```go
  // prototypes/mygo-run/main.go:293-303
  if !a.guard.Accept(snap.Revision) {
      a.err = "忽略过期快照"
      return
  }
  newSnap, err := state.ApplySnapshot(a.snap, snap)
  if err != nil {
      a.err = err.Error()
      return
  }
  if err := state.ConflictIfDifferent(a.snap, snap); err != nil {
      a.err = err.Error()
  }
  ```
- **症状**：全仓 `rg "a\.err"` 显示除 `main.go` 自身赋值外无任何 UI 读取逻辑，导致快照被拒绝、合并错误或冲突警告对操作者完全不可见。
- **违反规则**：`UX-P5`, `UX-STATE`
- **严重程度**：轻微
- **当前状态**：**仍然存在**。

#### 10. [Design V2] 后备 `StepList` 列表视图错误文本截断且无重试；`asterDesignPage` 缺少 `预览` 与 `已冻结` 态边界标识
- **位置**：`prototypes/mygo-run/internal/designer/step_list.go:296-299`, `321-336`；`prototypes/mygo-run/design_page.go:83-97`
- **代码证据**：
  ```go
  // prototypes/mygo-run/internal/designer/step_list.go:296-299
  if d.Error != "" {
      components.Text(c, "未保存 · "+d.Error).TextColor(t.Danger).FontSize(t.Rem(.75)).
          SingleLine().Ellipsis("…").MaxWidth(360)
  }

  // prototypes/mygo-run/design_page.go:83-97
  if a.readiness != nil {
      label, color := "草稿", t.TextMuted
      switch a.readiness.State { ... }
  ```
- **症状**：后备 `ListView` 工具栏把写入/加载错误截断在 `MaxWidth(360)` 单行并省略，没有重试操作；`asterDesignPage` 顶栏状态仅在 `a.readiness != nil` 时展示 `草稿`/`不完整`/`待确认`/`就绪`，未提供与 `预览` 或快照 `已冻结` 关联的可辨识状态词。
- **违反规则**：`UX-P5`, `UX-STATE`
- **严重程度**：轻微
- **当前状态**：**仍然存在**。

---

## 已由 LB-06 / LB-07 / LB-11 修复的缺陷（代码已验证）

1. **Run 资产关联坐标表单与缺口补齐预填（LB-06）**：`prototypes/mygo-run/main.go:2530-2566` 已实现坐标录入表单与文件不可用时的 `"未解析 · 文件不可用"` / `"重新定位"` 状态展示（`recording.go:607-615`）。
2. **Design 步骤列表写命令与 `designDocErr` 横幅（LB-07）**：`prototypes/mygo-run/design_page.go:122-125`, `233-286`, `345-418` 已接入 `stepPropertiesDrawer` 与步骤排序/缩进/提升/删除命令，并将 Core 拒绝消息渲染在 `a.designDocErr` 横幅中。
3. **Run 坐标候选串扰、空白修剪、批量前置校验与弹窗错误保留（LB-11）**：`prototypes/mygo-run/main.go:2548-2585` 已在 `submitAssetLinkForm` 失败时保持关联弹窗打开并在 `a.linkDialogErr` 展示错误（`main_test.go:468-480` 有回归断言）。

---

## 已消失（V2 重构移除的 UX-01 / UX-06 旧缺陷）

依据仓库既有证据 `docs/product/evidence/UX-01/README.md` 与 `docs/product/evidence/UX-06/README.md`，以下旧版缺陷已在 V2 重构中移除：

1. **Design 非法警告状态词 `可创建（有警告）`（`UX-01` 第 42 行）**：V2 `asterDesignPage`（`design_page.go:89-91`）已改用 §3 词汇 `待确认`。
2. **Run 暴露英文状态词 `in_progress` 与原始标识符 `ref-assay` / `assay-cck8`（`UX-01` 第 45, 48 行）**：V2 `runworkspace` 已改为中文化状态投影与人类可读步骤名称（注：全仓公共壳层 `internal/shell/shell.go:448-462` 仍保留项目卡片的 `执行中` 映射，不属于本卡 Run 板块）。
3. **Notebook 只有状态圆点缺少 `已保存` 状态词（`UX-01` 第 54 行）**：已于 `UX-06`（`762d35a` / `7f75359`）改用 `notebook.SaveStateLabel`（`notebook_page.go:165-169`）显式输出 `已保存` / `未保存`（但其失败态与重试仍缺失，见第 4 项发现）。
4. **Run 文件关联在外部弹窗闭环、无行内失败原因与重试（`UX-06` 第 12-13 行）**：`UX-06` 曾新增 `recording.go:448-457` 的 `关联失败：…` + `重新关联`，随后 `LB-11` 改为在打开状态的表单弹窗内由 `a.linkDialogErr` 展示（见第 7 项发现关于遗留死代码的说明）。

> 关于 `UX-07`：经全仓检索，本仓库中不存在 `UX-07` 的文档或提交记录，详见下方“未确认”。

---

## 未确认

1. **`UX-07` 旧缺陷清单**：经检查 `docs/` 目录（`rg -n "UX-0[167]" docs`）及 Git 提交历史（`git log --all --oneline --grep="UX-07"`），本仓库无任何 `UX-07` 文档、证据或提交记录，无法确认 `UX-07` 对应的旧缺陷及是否已消失。
2. **Windows 实机界面行为与画布/方案次路径**：本次任务为静态只读代码审计，未在 Windows 环境构建并运行 `mygo-run.exe` 验证渲染截图。`design_canvas.go` 画布模式的局部异常展示、以及 `protocol_page.go` 中 `loadProtocolPreview` 的实机加载/错误跳转链路未做运行时复现。
3. **遗留后备路径 `runui.Workspace`**：`run_page.go` 在正常路径使用 `runworkspace.View`，仅当 `a.runWS == nil` 时回退至 `runui.View`；后备旧视图 `internal/runui/runui.go` 未逐行做完整回归审计。
