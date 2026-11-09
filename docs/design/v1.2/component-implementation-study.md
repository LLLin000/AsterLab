# AsterLab 组件实现研究：从高保真到 MyGo Native

- 日期：2026-11-10
- 状态：实现研究，不是已实现代码
- 目标：在现有 Graphite（深色）/ Porcelain（浅色）视觉方向下，把画布、孔板、表格、Inspector、命令面板、变更审阅等做成真正的桌面工作台组件。

## 结论

1. **画布不是一组按钮**：节点、边、端口、选择、拖拽、缩放、框选都由一个 Canvas 状态机管理；节点位置/尺寸/命中区域用数据计算，绘制用 `ui.Painter`。
2. **表格不是 96 个 TextInput**：96 个输入控件会带来焦点、布局和性能问题。可见单元格绘制，只有当前单元格叠加一个文本编辑覆盖层。
3. **面板不是固定三栏**：使用可拖拽分隔条和可折叠上下文区；Inspector 可展开为主工作面，布局尺寸持久化。
4. **Agent 修改必须走 Proposal/Diff/Apply**：先展示结构化差异，用户确认后才写 Core；不能直接改 SQLite。
5. **浅色/深色共享同一布局和交互模型**，只切 Token；不以换色伪装第二套设计。

## 组件清单与实现契约

### 1. Workbench / Shell

结构：

```
Window
├─ TitleBar 40px
├─ Body
│  ├─ PrimaryRail 48px
│  ├─ ContextPanel 200–260px (collapsible)
│  └─ Workspace
│     ├─ Toolbar 36–40px
│     ├─ MainSurface
│     ├─ Splitter 4–6px (draggable)
│     └─ Inspector 220–420px (collapsible/resizable)
└─ StatusBar 24px
```

- `ui.Row/Column`, `Grow(1)`, `Width`, `MinWidth/MaxWidth`
- Splitter: 6px hit area, hover accent, drag adjusts `InspectorWidth`; double-click resets
- 900×600: ContextPanel hidden; Inspector becomes tabs or bottom sheet; never horizontal content overflow
- Persist widths in app state / Core settings, not hardcoded
- Focus order: TitleBar → Rail → Context → Main → Inspector → StatusBar

### 2. Scientific Workflow Canvas

State:

```go
type CanvasState struct {
    Nodes []Node
    Edges []Edge
    Viewport {X, Y, Zoom float32}
    Selected []string
    Hover string
    Drag *DragState
    Connect *ConnectState
    Undo, Redo []Command
}
```

- Node model: `ID, Kind (material/operation/instrument), Title, Params, Position, Size, Ports`
- Render: absolute `ui.Box` per node; `Draw` uses `r.X/r.Y` local coordinates to paint background, hero path, text; `ui.ParsePath` for SVG icon paths, offset into node rect
- Edge: `ui.Painter` cubic path from output port to input port; active/selected edge thicker/accent; dashed ghost while connecting
- Ports: 6–8px circle, 12px invisible hit target; `HitTest`/`OnClick` to start/end connection
- Selection: light fill + 1px accent + halo; multi-select via Shift; marquee via drag on empty canvas
- Pan: drag empty canvas or Space+drag; Zoom: wheel around cursor, 0.5–2.0; show 100% control
- Keyboard: Tab/Shift-Tab nodes, arrows move, Shift+arrows extend, Enter/space select, Delete, Escape cancel connect, Cmd/Ctrl+Z/Y
- Undo: command objects (`MoveNode`, `AddNode`, `Connect`, `Delete`), one drag = one command; no snapshots per frame
- Performance: draw only; no per-node TextInput; cap visible nodes; cull offscreen; animations via `AnimatePose` for selection/pan, not view rebuild
- Persistence: document JSON is source of truth; viewport is session state unless explicitly saved

### 3. Node card

- 3 visual kinds:
  - Material: flask/vial shape, material/sample fields
  - Operation: pipette/syringe shape, volume/time/conditions
  - Instrument: reader/microscope shape, wavelength/mode
- Hero illustration 32–40px; kind label 10–11px muted; title 13px 600; 2–3 params max
- Ports: left input/right output; connected/ready/error states
- Selected: 1px accent border + 6–8px halo; not a filled accent card
- Hover: raise 1px/translate 1px; no layout shift
- Inspector owns full parameters; node shows summary only

### 4. 96-Well Plate

- 8×12 circles 18–20px, gap 4px; A–H/1–12 labels 10px mono
- Draw all circles with `ui.Painter`; one `HitTest` maps pointer to well; not 96 buttons
- States: unassigned/assigned/selected/disabled/invalid/missing; group tint low saturation; selected ring 2px
- Drag rectangle selection; Shift adds; click toggles; keyboard arrows move, Space toggles
- Hover HUD shows well/sample/group/replicate/volume; no critical data only in tooltip
- 96 cells are one data model used by Plate and Table; never duplicate state

### 5. ResultGrid / Editable Table

- Numerical data: tabular figures, right-align, fixed column widths
- 96 rows/cells max in first version; draw rows/cells; only focused cell overlays `TextInputBase`/`HandleTextInput`
- Paste: TSV parser → preview (counts/overwrites/invalid) → Apply to draft → Save batch
- States: Draft (unsaved count), Saving, Saved rev X, Conflict, Error; errors preserve draft
- Undo/Redo at cell/selection/batch level; selection range rectangular
- Keyboard: Tab/Enter/arrows, Shift selection, Cmd/Ctrl+C/V, Esc cancel edit; focus ring always visible
- Batch save one transaction; never per-cell DB call
- Raw vs derived: derived columns visually separated and read-only; provenance shown

### 6. Inspector

- Right panel 280–360px; splitter resizable; 900px tabs
- Groups: identity, parameters, plate assignment, checks, provenance
- Labels 11px muted; values 13px; inputs 32px; rows 28–32px
- Collapsible sections; no card per field; one divider between groups
- Dirty/error/saving state at section and global level
- Selected object drives content; no selected object = clear empty state with action

### 7. Tabs / Segmented Control

- 28–32px height; 1px container border; selected surface elevation, not accent fill
- 2–5 tabs; overflow to select/scroll; keyboard Left/Right/Home/End
- 900px Designer Inspector tabs: 步骤属性 / 孔板映射 / 检测合约
- Save state global, not tab-local

### 8. Command Palette / Agent actions

- `ui.DialogBase`/`PopoverBase`; width 420–520; max height 60vh
- Opens with Cmd/Ctrl+K; search filters; groups and shortcuts; Esc closes
- Focus trapped until close; restore focus to trigger
- Actions are explicit commands, not free text writes
- Agent proposal action opens Diff review, does not save

### 9. Proposal / Diff Review

- `Proposal{ID, BaseRevision, Changes[], Status}`
- Changes are typed: add/remove/update node, edge, parameter, well assignment, group/replicate
- UI: left current vs right proposed; per-change checkboxes; validation summary; Apply/Reject
- Apply uses expected revision + operation ID; conflict shows both versions
- Reject leaves design untouched; Apply result shows new revision
- Never write SQLite from renderer; no “auto apply”

### 10. Modal / Popover / Menu / Tooltip / Toast

- Modal 360–520px, backdrop, Esc/outside close, focus trap
- Popover anchored to trigger, no click-through; 8px offset
- Menu items 28–32px; selected/highlighted uses low-saturation surface; keyboard arrows
- Tooltip delayed 300–500ms; not sole source of key data
- Toast 3–5s success, persistent error; inline status preferred for data saves

### 11. Fields / Select / Stepper / Unit

- Label above, 11px muted; input 32px; error 11px below
- Unit inline/right, muted; numbers tabular
- Select menu uses check + subtle highlight; no colored full-row selection
- Stepper buttons 24–28px, hit area 32px
- Escape restores, Enter commits; blur behavior explicit (no silent autosave for consequential values)

### 12. Module / Block components

- `Module` contract: `Kind`, `Render`, `Inspector`, `Serialize`, `Validate`, `Clone`
- Protocol step block: purpose/parameters/materials/checks/expected; read-only when source is frozen
- Note/Markdown: edit in place; preview toggle; no HTML sanitization bypass
- Parameter table: rows editable; units distinct; import from TSV with preview
- Calculator: pure math + explicit formula; result can create a draft block; no procedural lab advice
- Library: categories/search/preview/insert; insert is a Designer command with undo
- Every module has an explicit empty/loading/error state and provenance

## 交互性能预算

| Interaction | Target |
| --- | --- |
| Hover/focus feedback | < 100ms |
| Node drag frame | 60 fps for ≤ 100 nodes |
| Pan/zoom | no layout rebuild; paint only |
| Plate selection | immediate, 96 cells |
| Grid cell focus | < 100ms |
| Save click feedback | immediately “Saving” |
| Core save | >1s spinner; >10s progress/cancel guidance |

- Use `ui.Draw` and `AnimatePose` for canvas; avoid rebuilding full view on each pan pixel
- Cull offscreen nodes/edges; avoid 96 live text inputs
- Batch database writes; no per-cell transaction
- Keep one state source; view projections don't mutate

## 验收（每组件）

- Structure: layout at 1200×760, 900×600, no clipping
- Visual: tokens, typography, icon optical alignment, state contrast
- Interaction: real mouse/keyboard, focus, hover/pressed/disabled, undo
- Business: real Core/SQLite, revision/conflict, no fake saves
- Screenshot: real window, not only Tester

## 参考

- MyGo `docs/ui/drawing.md` (Draw, Path, HitTest, ClipPath, Trim)
- MyGo `docs/ui/custom-widgets.md` (ButtonBase, SelectBase, DialogBase, TextInputBase, focus/disabled)
- MyGo `docs/ui/motion.md` (AnimateTo, AnimatePose, reduced motion)
- MyGo `docs/ui/layout.md` (Flex/Grid, Splitter, Sticky, Absolute)
- VS Code Custom Layout: https://code.visualstudio.com/docs/configure/custom-layout
- React Flow Adding Interactivity: https://reactflow.dev/learn/concepts/adding-interactivity
- Cursor review/diff pattern: https://cursor.com/learn/reviewing-testing
- WCAG 2.2 Focus Visible / Focus Not Obscured
