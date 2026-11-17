# CLOSE-06 证据：冻结蓝图驱动采图入口与批传缺漏闭环

- 基线 HEAD：`431a404`（CLOSE-05 合并后 origin/main；本卡 worktree 为其干净复刻 + 本卡 5 文件修改）
- 数据/二进制：隔离渲染（`ui.NewTester` headless，1360×880 / 900×600，浅/深色）；Real-window 与原生文件选择器走 Gate 人工（见 §5 未验证项）
- 验收命令：`cd prototypes/mygo-run && go test ./internal/runworkspace/... ./internal/resultgrid/... ./internal/uikit/...` → exit 0（`-count=1` 复核通过）；`go vet` 同范围通过；新增/改动单测全部通过（含 main 包 `TestFrozenPlannedWells*`、`TestUnlinkRunAsset*`、`TestPlanBatchDropsSeventhImage*`）

## 1. F2：designPlateWells 读冻结计划，不读 Design 页 live 实例（已修）

复核（`431a404` 仍存在）：`assetCoordinateOptions`（`main.go:747`）在 `snap.DesignData` 为空时回落到 `a.designSnap.Data`（Design 页 live 草稿）；且 `designPlateWells` 只认 mygo `plate_12/plate96` 实例的**全容量**（HUVEC 12 孔返回 12 个槽位），完全不读冻结 AsterDoc 板。`prepareGrid`（`main.go:2076`）有同样的 live 回落。

修复（`main.go:722-1010`）：
- `assetCoordinateOptions` 与 `prepareGrid` 删除 `designSnap` 回落，只读 `snap.DesignData`（冻结）；无冻结数据即显式报错/空槽位（UX-P5，UX-STATE）。
- 新增 `frozenPlannedWells(asterSource, assayID, stepID)`（`main.go:791`）：解析 Run 冻结 AsterDoc 文本（`a.asterDoc.Source`，`loadFrozenAsterDoc` 只装载 Run 钉住的 revision+hash，`main.go:1210+`）。plate 取其 `groups` 实际孔位（计划，非容量）；container 有 `sample_map` 取键（2D 有计划才有槽，无 map 的 2D 声明零槽——容量永不作计划），1D（petri/tube/rack/strip 等）取 `Lane<N>`；文件名永不参与（无坐标文件进“未标注”，`recording.go:426`）。
- 单板保持裸孔位；多板/多容器同 assay 时孔位限定为 `sourceID:well`（`p-one:A1` vs `p-two:A1`），同 A1 不再碰撞。
- 旧 mygo 设计（无 AsterDoc）沿用冻结 `DesignData` 的 `designPlateWells` + `BuildSchema` 回退——同样是冻结源。

证据：`TestFrozenPlannedWellsUsesPlateGroupsNotCapacity`（9 计划孔 vs 12 容量）、`TestAssetCoordinateOptionsReadsFrozenPlanNotLiveDraft`（live 96 孔草稿被忽略）、`TestFrozenPlannedWells1DContainerReadsLaneSlots`（`Lane1..Lane8`）、`TestFrozenPlannedWellsContainer2DUsesSampleMapNotCapacity`、`TestFrozenPlannedWellsMultiPlateQualifiesIdentity`、`TestFrozenPlannedWells384Subset`（`A1/P24`）。规则：UX-STATE、UX-P5、UX-DONE。

## 2. 槽位填充与可恢复状态（已修/已证）

- 显式坐标：单文件 dialog（孔/时间点/FOV 三选 + `validateAssetCoordinates`，`main.go:579`）坐标 milik 本次选择；strip “批量关联”按任务时间点 + 首缺槽自动填（`requestBatchLink`，`run_recording_page.go:252`）。
- 第 7 张溢出：`PlanBatchDrops` 跳过满孔（`capture.go:140`），`TestPlanBatchDropsSeventhImageOverflowsToNextWell`（7 文件 → A1×6 + A2/FOV1）与截图 `capture-huvec-*-light.png`（A1 6/6 + A2 1/6，“1/9 孔已传满”）互相印证。规则：UX-P1（批量关联在 900×600 首屏）、UX-P4。
- 不足槽：只返回装得下的 drops，余数计 skipped 并随原因显示（`run_recording_page.go:342`），已关联文件保留（UX-P5）。
- 取消：文件选择器取消返回空 → 直接返回，草稿/已关联不动；dialog 取消只关窗不写（UX-P5）。
- 重复坐标：`CoverageByWell` 按 FOV 去重计数（`capture.go:36`），同槽多文件并列展示不 double-book，`补齐` 永不复用已占槽；修坐标/解绑见 §4。
- 写失败：`linkAssetPaths` 先全量校验再逐个保存，失败即停并保留 dialog 错误（`main_test.go:TestLinkAssetPathsRejectsOutOfBoundsBatchWithoutSavingFirstFile`）；strip 批量逐文件计 skipped + 首错原因（UX-P5）。
- 多板身份：见 §1 限定式孔位；strip/gallery/历史三处读同一字符串，无第二套身份。

## 3. 缺口来自完整冻结计划；补拍/重标/解绑同源同步（已修）

- 整孔未拍也显示：gallery `assetMatrix` 并入计划（`recording.go:426`）——计划孔×计划时间点全交叉，无链接的整孔渲染全部缺口（`TestAssetMatrixUnionsFrozenPlanWholeWellsAsGaps`、`TestRecordingGalleryRendersWholePlanGapsWithRefill`）；strip `CoverageByWell` 本就计划优先；历史 Tab 读 Core `record.Projection` 同源矩阵。三处同一冻结源，无第二份推导矩阵（UX-P4）。
- 补拍/深链同 Run/任务/孔：gallery 每个缺口的“补齐 FOV n”预填同一 taskKey + 孔 + 时间点 + FOV（`recording.go:612`），打开同一 dialog；“返回 Run 时间线”跳回同一任务时间线（UX-P1）。
- 重标/解绑同步：`assetRow` 未标注给“重设坐标/解除关联”，已标注给“打开/重设坐标/解除关联”，缺文件给“重新定位/解除关联”（`recording.go:688`，UX-P5）。`relabelRunAsset`/`unlinkRunAsset` 写后统一走 `syncAssetSurfaces`（`main.go:1100`）：重建 VM + 丢弃 record 投影 memo（link 写不 bump revision，memo 键含资产指纹，见 `fetchRecord`），strip/gallery/历史同帧一致。`TestUnlinkRunAssetSyncsSurfaces` 断言解绑后资产表与 memo 同步失效。
- 盲法：分组名遮罩，孔位坐标永不遮罩（`matrixGroups`，UX-P2）。

## 4. 截图（本目录 PNG，headless tester 实像素渲染）

| 文件 | 内容 | 规则 |
|---|---|---|
| `capture-huvec-1360x880-light.png` / `-dark.png` | 9 计划孔×6 FOV：A1 6/6、A2 1/6、“1/9 孔已传满”、缺 “A2 缺 FOV [2 3 4 5 6]”、批量关联、gallery 整孔缺口 + 补齐、解除关联/重设坐标 | UX-P1、UX-P4、UX-P5、UX-LAYOUT |
| `capture-huvec-900x600-light.png` / `-dark.png` | 同上窄屏：记录面板首屏，批量关联可达 | UX-P1、UX-LAYOUT |
| `capture-1d-1360x880-light.png` / `capture-1d-900x600-dark.png` | 1D 管架 Lane1..Lane8：Lane1 1/1、Lane2 0/1 + 补齐 | UX-P4 |
| `capture-384-1360x880-light.png` / `capture-384-900x600-dark.png` | 384 取计划子集 A1/P24：A1 1/6、P24 0/6、“0/2 孔已传满”（容量 384 永不出现） | UX-P4、UX-STATE |

断言落盘在 `internal/runworkspace/closure_capture_evidence_test.go`（`CLOSE06_EVIDENCE=1` 复现；默认 `go test` 不写盘）。交互：strip “批量关联”点击直达 `RequestBatchLink`（同文件断言）；gallery 缺口预填点击见 `recording_test.go:TestRecordingGalleryRendersWholePlanGapsWithRefill`（A2/6h/FOV1 同任务）。

冻结计划口径：9 计划孔取自仓库既有 HUVEC 板形（`asterdoc_store_test.go:parityPlateMixDoc`：3%/5%/7% × A/B/C 1..3 = 9 孔，12 孔板容量）；3 时间点取自 `internal/asterdoc/testdata/huvec-tube.asterml` assay `timepoints: [6 h, 12 h, 24 h]`；每孔 6 视野取自 UI-REDESIGN（12 孔×6 视野）与 `FOVPerWellForStep`（`per_well_fov_count`）。9×3×6 = 162。凡出现 12 处均为容器容量对照，计划槽位恒为 9。

## 5. 未验证项（诚实缺口，需 Gate 人工，不冒充完成）

1. **原生文件选择器 7 图批传**：`requestBatchLink` 的选文件一步是 OS 原生对话框，自动化驱不动（项目惯例：标未验证）。槽位决策（首缺→A1×6→A2 溢出）与落库循环已有单测 + headless 点击 + `linkAssetPaths` 真文件落盘单测覆盖；Gate 请在实窗手动选 7 图验证跨孔。
2. **162 全矩阵实窗**：headless 渲染断言了 9 孔×6 FOV 结构与整孔缺口；携带 162 资产的实窗长列表滚动与性能未测。
3. **实窗 1360×880/900×600 浅/深色**：本目录 PNG 为 tester 实像素渲染（同绘制代码），非 OS 窗口抓屏； Gate 需实窗复核。
4. **旧 mygo 设计回退链**：`designPlateWells` 容量语义保留给无 AsterDoc 的 legacy run，未新增单测 beyond 既有 `TestDesignPlateWellsUsesConfiguredPlateGeometry`（行为未变）。
