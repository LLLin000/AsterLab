# AsterLab v1.2 — Native Implementation Contract & UI-Kit Specification

- **Version**: 1.2.1 (Revised per Conditional Review)
- **Target Framework**: MyGo Native Desktop (`github.com/egoist/mygo/ui`)
- **Core Principle**: Component-Driven Native Architecture. **Strictly NO temporary per-page button piles or raw text dumps.**
- **Scope**: Every recurring visual element across all 5 pages is mapped directly to a reusable UI-Kit primitive implemented on MyGo bases.

---

## 1. UI-Kit Component Mapping to MyGo Primitives

| UI-Kit Component | MyGo Underlying Primitive | Visual Attributes & Behaviors | Target Pages |
| :--- | :--- | :--- | :--- |
| `WindowChrome` | `mygo.WindowOptions`, `ui.Row().DragWindow()` | Height 48px, title, breadcrumbs, drag handle, system controls. | All |
| `PrimaryRail` | `ui.Column().Width(56)`, `ui.ButtonBase` | Width 56px, vertical dark anchor, active spruce indicator bar. | All |
| `ContextSidebar` | `ui.Sidebar`, `ui.SidebarSection`, `ui.SidebarItem` | Width 220px, collapsible with `ElementTransition{Width: true}`. | Project, Protocol, Tools |
| `Button` | `ui.ButtonBase` | Variants: Primary, Standard, Ghost/Quiet, Danger. Height 32px, Radius 4. | All |
| `StatusBadge` | `ui.Row().Radius(4).Padding(2, 8)` | Compact chip for `Success`, `Warning`, `Danger`, `Neutral`, `Raw`, `Derived`. | All |
| `StepperChip` | `ui.ButtonBase`, `ui.Row` | Stepper chip displaying index, task title, completion `✓`, active focus. | Protocol, Run |
| `Plate96Widget` | `ui.Box().Draw()`, `ui.HitTest` or Matrix | 8×12 SVG microplate with row A-H, col 1-12, group tints, box-select. | Designer, Tools, Run |
| `WorkflowNode` | `ui.Box().Draw()`, `ui.Draggable`, `Painter` | Scientific SVG hero icon, 3 visual types, ports, 4 interaction states. | Designer |
| `ResultGrid` | `resultgrid.Grid`, `ui.ScrollBoth()`, `ui.Painter` | 96-well tabular entry, cell selection, arrow keys, TSV clipboard. | Run |
| `InspectorTabs` | `ui.SegmentedBase`, `ui.Row` | 3-tab full-width switcher for 900×600 compact desktop in Designer. | Designer (900×600) |
| `SectionCard` | `ui.Column().Radius(8).Border(1)` | Structural card with title, action slot, content padding 16px. | Project, Protocol, Tools |
| `FormField` | `ui.Column().Gap(4)`, `ui.TextInputBase` | Field label, text/number input, unit addon, inline error text. | Designer, Tools |
| `PastePreviewDialog` | `ui.DialogBase`, `ui.Box().Background(Backdrop)` | Non-destructive paste modal showing row/col count and overwrite warning. | Run |
| `EmptyState` | `ui.Column().Center().Padding(32)` | Clean illustration icon, title, description, single primary action. | All |
| `ReviewBanner` | `ui.Row().Padding(8, 12).Radius(6)` | Sticky alert banner distinguishing Read-Only Review Mode from Active Run. | Run |

---

## 2. Component Implementation Contracts

### 2.1 `Button` Contract
Buttons must never be created as raw unstyled `ui.Button(c, "label")` without semantic variant assignment.

```go
type ButtonVariant int
const (
    ButtonPrimary ButtonVariant = iota // Solid Accent background, AccentText
    ButtonOutline                      // Surface background, 1px Border, Text
    ButtonGhost                        // Transparent background, TextMuted, hover tint
    ButtonDanger                       // Danger border / background, white text
)

func Button(c *ui.Context, label string, variant ButtonVariant) *ui.Element
```
- **States**: Resting, Hover (`SurfaceHover` or `AccentHover`), Pressed (scale 0.99), Focused (2px Accent ring), Disabled (opacity 0.42).
- **Height**: 32px standard desktop (30px on 900×600 compact).

### 2.2 `WorkflowNode` Contract (Scientific SVG DAG Nodes)
Nodes in Designer represent biological entities and procedures. They are topological graph tokens; all granular editing happens in the lower Inspector.

```go
type NodeKind int
const (
    NodeMaterial NodeKind = iota   // Material: Cryovial / Flask illustration
    NodeOperation                  // Operation: Pipette / Well Plate dispenser illustration
    NodeInstrument                 // Instrument: Microplate reader / Spectrophotometer illustration
)

type NodeState int
const (
    NodeStateDefault NodeState = iota // 1px Border, resting elevation
    NodeStateSelected                 // 2px Accent border, soft glow, active ports
    NodeStateDragging                 // Lifted (Scale 1.02, elevation shadow)
    NodeStateConnecting               // Port target pulse
)

type WorkflowNodeProps struct {
    ID          string
    Kind        NodeKind
    Title       string
    ParamBadge  string   // Concise pill (e.g. "100 µL", "OD450")
    State       NodeState
    X, Y        float32
    Width       float32  // Standard: 145px (120px compact)
    Height      float32  // Standard: 64px
    HasInput    bool     // False for Material
    HasOutput   bool     // Always true
}

func WorkflowNodeWidget(p *ui.Painter, props WorkflowNodeProps)
```
- **Rendering & Hero Icon**:
  - The node features a **Hero SVG Illustration** drawn via `Painter.StrokePath` / `Painter.FillPath`:
    - Material: Flask/vial path (`M9 3h6v2h-1v5l4 9H6l4-9V5H9z`).
    - Operation: Pipette liquid transfer path (`M19 3l2 2-9 9-2-2 9-9z M6 16l-3 5 5-3z`).
    - Instrument: Microplate optical chamber path (`M4 6h16v12H4z M9 10h6v4H9z`).
  - Icon is the primary visual anchor (upper 36×36 area); title (12px bold) and parameter badge (10px muted) are secondary.
  - Left input port (if `HasInput`) and right output port rendered as 6px circles with 1.5px stroke.

### 2.3 `Plate96Widget` Contract (8×12 Microplate Matrix)
Replaces the 96 raw button pile with an authentic, scientifically accurate microplate.

```go
type WellState struct {
    WellID    string  // "A01" .. "H12"
    Group     string  // "Control", "10µM", "Blank"
    SampleID  string  // "SMP-001"
    Value     *float64
    Selected  bool
    Assigned  bool
}

type Plate96Props struct {
    Wells        map[string]WellState
    SelectedIDs  []string
    OnSelect     func(wellIDs []string)
    OnToggleWell func(wellID string)
    Interactive  bool // false in read-only run view
}

func Plate96Widget(c *ui.Context, props Plate96Props)
```
- **Rendering**:
  - 8 rows (`A–H`) and 12 columns (`1–12`) with coordinates.
  - Assigned wells tinted by semantic group:
    - Control: `WellControl` (`#D9E8F5` Light / `#192E42` Dark).
    - Treatment: `WellAssigned` (`#E1EFEA` Light / `#1B362F` Dark).
    - Blank: `WellBlank` (`#EBEAE5` Light / `#2A2824` Dark).
    - Selected: `Accent` (`#1F6F5F` Light / `#4FA38D` Dark) with tick mark.
  - Rectangular box-drag selection handled smoothly via `ui.Draggable` and `HitTest`.

### 2.4 `ResultGrid` & Save Lifecycle Contract
High-density wet-lab measurement table with honest transactional save semantics.

```go
type SavePhase int
const (
    SavePhaseReady SavePhase = iota // Rest state, rev X confirmed
    SavePhaseDraft                  // N uncommitted edits in UI, status = Warning
    SavePhaseSubmitting             // Inputs locked, spinner active
    SavePhaseSaved                  // Confirmed rev X+1 by Core
    SavePhaseConflict               // Rebase / conflict banner
)

type ResultGridOptions struct {
    FullWidth bool
    ReadOnly  bool
    Phase     SavePhase
    Revision  uint64
    DirtyCount int
}

func RenderResultGrid(c *ui.Context, g *resultgrid.Grid, opts ResultGridOptions)
```
- **Inviolable Rule**: Grid NEVER reports `✓ 已保存` or `已同步` while `DirtyCount > 0`.
- **Paste TSV Action**: Opens `PastePreviewDialog` showing matrix dimensions and overwrite count before committing to draft cells.
- **Batch Save Commit**: Locks inputs and dispatches atomic `SaveBatch` request with `expected_revision`.

### 2.5 `InspectorTabs` Contract (900×600 Compact Desktop)
```go
type InspectorTab int
const (
    InspectorTabAttributes InspectorTab = iota // 步骤属性
    InspectorTabPlate                          // 96 孔板分组映射 (当前)
    InspectorTabContract                       // 检测合约与参数
)

func InspectorTabs(c *ui.Context, active *InspectorTab, onSelect func(InspectorTab))
```
- At 900×600, converts the 3 horizontal columns into a full-width segmented tab view so that the 96-well plate and form inputs never suffer horizontal squishing.

### 2.6 `ReviewBanner` Contract
```go
func ReviewBanner(c *ui.Context, taskTitle string, onUnlock func())
```
- Displayed when viewing completed runs or historical steps.
- Color: Warning amber banner (`#FDF3E7` Light / `#2A2216` Dark).
- Copy: `👁 正在回看已完成步骤: [taskTitle] · 当前数据已锁定为只读。`
- Action: "解锁并重新测定" (with explicit confirmation dialog).

---

## 3. Migration Roadmap for Production Modules

When engineering implements v1.2, every module replaces legacy raw code with these UI-Kit primitives:

1. `internal/shell`:
   - Replace raw vertical button stacks in `projectView()` with `SectionCard`, `Button(ButtonPrimary)`, and `StatusBadge`.
   - Embed `PrimaryRail` with proper SVG icons and tooltips.
2. `internal/designer`:
   - Replace rounded text-only boxes with `WorkflowNode` featuring hero SVG `<path>` illustrations and ports.
   - Implement `InspectorTabs` for 900×600 compact desktop.
   - Replace the 96 raw button matrix with `Plate96Widget`.
3. `internal/protocol`:
   - Replace developer apologies with the professional `ReviewBanner` and `SectionCard`.
   - Implement `StepperChip` sequence.
4. `internal/runui` & `internal/resultgrid`:
   - Enforce honest save lifecycle (paste preview -> draft -> submitting -> saved rev X).
   - Implement single-column tabbed mode for 900×600 compact desktop.
5. `internal/tools`:
   - Dilution calculator performs pure math derivations without speculative protocol guessing.
