# Audit: 项目工作区 / 实验设计 / Protocol

Scope: code read only; no build/run. Claims cite `file:line`.

## 1. Page inventory

| Page | Entry point (file:line) | Primary action | Secondary actions | States | Reads/Writes |
|---|---|---|---|---|---|
| 项目工作区 (Project workspace) | `internal/shell/shell.go:244` `page()` → `projectView()` at `shell.go:260` / `shell.go:271` | Open an existing design card (`shell.go:424-431`) or create via `OnCreate` (`shell.go:302`) | Search designs `shell.go:298`; import `shell.go:305`; continue last `shell.go:308`; switch project `shell.go:367-375`; open run card `shell.go:470-471`; template shortcuts `shell.go:515-521`; recent runs `shell.go:560-561` | loading `shell.go:380`; error `shell.go:383`; empty `homeEmpty` `shell.go:390`; in-progress runs capped to 2 `shell.go:323-330`; designs capped to `6 - len(inProgress)` `shell.go:331-337` | Reads `Shell.Designs/RecentRuns/Projects/Query/Loading/Error`; writes `Selected/Page/Project/Query`; callbacks `OnCreate/OnImport/OnContinue/OnSelect/OnProject/OnRunSelect` |
| 实验设计 — workflow canvas (Design) | `main.go:1405` `designPage()` → `asterDesignPage()` `main.go:1499` when `designDoc != nil`; canvas at `design_canvas.go:36` `renderWorkflowCanvas()` | Edit workflow graph; create Run button `main.go:1550` | Add module `design_canvas.go:62`; add step `design_canvas.go:65`; delete selected `design_canvas.go:68-72`; connect mode `design_canvas.go:74-83`; step properties toggle `design_canvas.go:84-93`; fit canvas `design_canvas.go:94-96`; undo/redo `main.go:1536-1544`; 流程/文档 tabs `main.go:1533`; preflight `main.go:1562` | readiness draft/incomplete/warning/ready `main.go:1517-1531`; doc error banner `main.go:1555-1559`; hierarchy error `design_canvas.go:110-116`; subflow `design_canvas.go:43-46`; inspector open/closed `design_canvas.go:51-54` | Reads `designDoc/designCanvas/designSelected/designHierarchy`; writes via `applyDesignRaw` `main.go:1609` (Core commands); persists canvas layout/session |
| 实验设计 — document view (Design) | Same entry `main.go:1405`/`main.go:1565` (文档 tab) renders `asterdoc.View.Render` `main.go:1588` | Edit module properties via generic editor `asterdoc/view.go:871` `GenericModuleEditor`, save `asterdoc/view.go:942` → `OnUpdateModule` `main.go:1581` → `applyDesignRaw` | Assign/clear wells `main.go:1575-1579`; plate overlay in `asterdoc/view.go:212` | Read-only when `!Can(PermDocumentEdit)` or `ReadOnly` `asterdoc/view.go:893`; artifact/list/table inputs read-only `asterdoc/view.go:914,931` | Reads `designView.Doc`; writes Core command `update_module`; document revision reload |
| Protocol (read-only) | `main.go:1756` `toolsPage()` tab 1 `main.go:1761` → `protocolPage()` `main.go:1766` / `main.go:1432` | Read step details; select step `protocol/view.go:41-42` | Export frozen doc button `main.go:1441` `documentExportButton`; markdown rendering `protocol/document_view.go:27` | Run frozen AsterDoc path `main.go:1434-1461`; Run without AsterDoc `main.go:1466-1481`; live design path `main.go:1489-1497` | Reads `snap.AsterDoc/DesignData/DesignID/DesignRevision` or `designer.Editor.Doc/Instances`; no writes (stated `protocol/view.go:153`) |

## 2. Responsibility overlaps

Where each kind of edit actually happens:

- **Parameter edit**: document view generic editor `asterdoc/view.go:871-1012` (inputs rendered 897-939, save 942-1009) → `OnUpdateModule` `main.go:1581-1584` → `applyDesignRaw` `main.go:1609`. Also step/assay binding in canvas inspector `designer/designer.go:1345-1388` (assay ID, timepoints, component refs). Protocol view only displays `row.Key/Value/Unit` `protocol/view.go:85-91`; not editable.
- **Check edit**: checklist block rendered read-only in protocol `protocol/view.go:106-127` (`item.Checked` display only). In document view, checklist is `list/object/table` class → rendered "read-only" `asterdoc/view.go:931`; so checks are **not editable in either surface** currently. The only editable checks would be via Core/block instance edits not exposed in UI (no file:line found).
- **ProtocolBlock reference**: authored in Design canvas step module: `module.Data["uses"]` with `ref` matched to a `reference` module whose `view == "protocol_block"` `workflowtree/design.go:65-77,98-109`. Drilldown/hierarchy built by `workflowtree.BuildDesign` `workflowtree/design.go:55`. Protocol view does not resolve ProtocolBlocks; it lists module titles only `protocol/document.go:150-152`. So the Protocol page shows block *titles*, while the Design canvas owns the ProtocolBlock reference and expansion.

Overlap summary: Design canvas owns structure/execution (steps, links, assay/timepoints, ProtocolBlock refs); document view owns module property values; Protocol page is a read-only projection of the same frozen design (`protocol/document.go:75` `Build`, `main.go:1482`). There is no single "parameter" surface: assay-level params live in canvas inspector, module params in document view, and protocol only reads both.

## 3. Usability defects (ranked)

1. **Protocol is not a top-level page.** Entry is buried under `toolsPage` tab "Protocol" `main.go:1761-1766`; shell page list is only Project/Design/Run/Notebook/Tools `internal/shell/shell.go:115`. Symptom: user cannot reach Protocol without opening Tools then switching tab; no breadcrumb/state persists `shell.go:43-55` only 5 pages.
2. **Project workspace silently hides designs beyond first 6.** `main.go:331-337` sets `designLimit := 6 - len(inProgress)` and slices `designs` with no "show more". Symptom: with ≥2 in-progress runs, only 4 designs visible; older designs unreachable from home.
3. **Design canvas primary commands are icon-only.** Toolbar uses `IconButton` with labels used as tooltips `design_canvas.go:62-102` ("添加模块", "添加步骤", etc.), no text; at `900×600` with inspector closed the row still shows only glyphs. Symptom: discoverability of add/delete/connect/fit is low; new users cannot tell which icon creates a Run (Run button is separate in header `main.go:1550`, but visually similar accent).
4. **Inspector defaults closed under 1100px.** `designNarrow` `design_canvas.go:28-31` returns true when width < 1100; `renderWorkflowCanvas` sets `designInspectorOpen = !narrow` unless user toggled `design_canvas.go:51-54`. Symptom: at 900×600 the step-property panel is hidden by default, so editing assay/timepoints/component refs requires finding the small "步骤属性" icon `design_canvas.go:84`.
5. **Protocol page splits into two unrelated render paths.** `protocolPage` `main.go:1432-1497`: if `snap.AsterDoc != nil` renders frozen markdown `main.go:1456`; otherwise builds `protocol.Build` `main.go:1482` and calls `protocol.View` `main.go:1487`. Symptom: same "Protocol" tab shows either a paper document or a step inspector depending on run state; user cannot tell which view they are in; no mode label beyond "冻结文档 · rev N" `main.go:1438`.

## 4. Layout options

### 项目工作区 (shell.projectView, `internal/shell/shell.go:271`)
- **A (current)**: 3/2/1 responsive card grid, max-width 1040 `shell.go:274-280,289`. Tradeoff: compact but hard-caps visible designs at 6.
- **B (recommended)**: remove the 6-item cap `shell.go:331-337`, keep grid but add a "显示全部设计" toggle; recent runs remain a separate list `homeRuns` `shell.go:527`. Tradeoff: longer scroll, nothing hidden.
- **C**: two-pane: left scrollable design list (all items), right selected design details/actions. Tradeoff: extra click to open but scales past 20 designs.

Recommendation: **B**.

### 实验设计 (asterDesignPage + renderWorkflowCanvas)
- **A (current)**: full-width canvas with floating 360px inspector toggled `design_canvas.go:118-126`, `inspectorPanel` `designer/designer.go:800-830`. Tradeoff: max canvas area but params hidden on small widths.
- **B (recommended)**: persistent right inspector column (min 320px) at ≥1100px; below 1100px inspector becomes a bottom sheet (320px) opened by the existing toggle. Tradeoff: canvas loses ~320px on desktop but params always visible.
- **C**: inspector always docked bottom at 320px height. Tradeoff: no occlusion but reduces vertical canvas on all sizes.

Recommendation: **B**.

### Protocol (protocolPage + tools tab)
- **A (current)**: under Tools tab `main.go:1761-1766`. Tradeoff: no new nav entry but discoverability poor.
- **B (recommended)**: add a sixth top-level shell page "实验方案/Protocol" to `pages` `shell.go:115` and page enum `shell.go:20-26`, routing to `protocolPage`. Tradeoff: one more sidebar item but direct access.
- **C**: third segmented tab in Design header alongside 流程/文档 `main.go:1533`. Tradeoff: keeps design context but crowds the header and conflates authoring vs reading.

Recommendation: **B**.

## 5. Acceptance criteria

### 项目工作区
- 1360×880: title "项目工作区" `shell.go:293`, search input `shell.go:298`, and "新建设计"/"导入设计"/"继续上次工作" buttons `shell.go:302-308` visible without scrolling.
- 900×600: same primary buttons visible without scrolling; design cards reachable by vertical scroll (no hard cap) — verify no designs are unreachable at 900×600.

### 实验设计
- 1360×880: "创建 Run" button `main.go:1550`, undo/redo `main.go:1536-1544`, and 流程/文档 tabs `main.go:1533` visible in header without scrolling; canvas fills remaining height `design_canvas.go:118`.
- 900×600: "创建 Run" button visible without scrolling; canvas min height ≥300px; inspector toggle "步骤属性" `design_canvas.go:84` visible; opening inspector does not hide the Run button.

### Protocol
- 1360×880: step list width 224 `protocol/view.go:27` plus detail column visible side-by-side without horizontal scroll; paper surface max-width 820 centered `protocol/document_view.go:32`.
- 900×600: selected step's "Parameters" `protocol/view.go:75` and "Checks" `protocol/view.go:106` sections visible without horizontal scroll; step list scrolls independently `protocol/view.go:27`.
