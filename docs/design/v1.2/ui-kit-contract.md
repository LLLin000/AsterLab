# AsterLab v1.2 — Native Implementation Contract & UI-Kit Specification

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
| `WorkflowCanvas` | `ui.Box().Draw()`, `Painter.Transform`, `Draggable` | Directed graph nodes, bezier edges, pan/zoom coordinate mapping. | Designer |
| `ResultGrid` | `resultgrid.Grid`, `ui.ScrollBoth()`, `ui.Painter` | 96-well tabular entry, cell selection, arrow keys, TSV clipboard. | Run |
| `SectionCard` | `ui.Column().Radius(8).Border(1)` | Structural card with title, action slot, content padding 16px. | Project, Protocol, Tools |
| `FormField` | `ui.Column().Gap(4)`, `ui.TextInputBase` | Field label, text/number input, unit addon, inline error text. | Designer, Tools |
| `ConfirmDialog` | `ui.DialogBase`, `ui.Box().Background(Backdrop)` | Modal overlay, Escape to dismiss, title, body, destructive/action buttons. | Run (Paste overwrite), Project |
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

### 2.2 `StatusBadge` Contract
```go
type BadgeKind string
const (
    BadgeSuccess BadgeKind = "success" // Green: ✓ 已完成, 已就绪
    BadgeWarning BadgeKind = "warning" // Amber: ● 草稿未保存, 待确认
    BadgeDanger  BadgeKind = "danger"  // Red: ⚠ 阻塞错误, 格式错误
    BadgeNeutral BadgeKind = "neutral" // Gray: ○ 未开始, 历史归档
    BadgeRaw     BadgeKind = "raw"     // Dark: 原始读数
    BadgeDerived BadgeKind = "derived" // Teal: 计算派生
)

func StatusBadge(c *ui.Context, text string, kind BadgeKind)
```
- **Metrics**: Height 20px, horizontal padding 8px, font 11px bold, radius 4px.

### 2.3 `StepperChip` Contract
Used on Protocol and Run to display execution chronology without clumsy pagination.

```go
type StepperState string
const (
    StepCompleted StepperState = "completed" // Accent or Success checkmark
    StepActive    StepperState = "active"    // Solid Accent, white text
    StepUpcoming  StepperState = "upcoming"  // Outline, muted text
    StepReview    StepperState = "review"    // Eye icon, subtle blue tint
)

type StepperChipProps struct {
    Index     int
    Title     string
    Timepoint string
    State     StepperState
    OnClick   func()
}

func StepperChip(c *ui.Context, props StepperChipProps)
```

### 2.4 `Plate96Widget` Contract (Custom 96-Well Plate)
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
  - Container aspect ratio 12 : 8.
  - Column headers `1 .. 12`, Row headers `A .. H`.
  - Well circles drawn with crisp 1px strokes.
  - Assigned wells filled with group semantic tints (`WellControl`, `WellAssigned`, `WellBlank`).
  - Selected wells filled with `Accent` (`#1F6F5F`) and white tick mark.
  - Supports rectangular mouse-drag selection (`ui.Draggable` / `HitTest`).

### 2.5 `WorkflowCanvas` Contract
Upper zone of Experiment Designer. Renders a clean DAG of operations.

```go
type WorkflowNodeProps struct {
    ID        string
    Kind      string // "material", "operation", "device"
    Title     string
    Subtitle  string
    Selected  bool
    X, Y      float32
}

func DrawWorkflowGraph(p *ui.Painter, nodes []WorkflowNodeProps, edges []WorkflowEdge, pan ui.Point, zoom float32)
```
- **Nodes**: Rounded rect 160×64px, 1px border, category icon, title in 13px bold, parameter subtitle in 11px muted.
- **Edges**: Directed Bezier curves with smooth cubic control points and solid arrowheads.

### 2.6 `ResultGrid` Contract
High-density wet-lab measurement table.

```go
type ResultGridOptions struct {
    FullWidth bool
    ReadOnly  bool
}

func RenderResultGrid(c *ui.Context, g *resultgrid.Grid, opts ResultGridOptions)
```
- **Row Height**: 28–32px.
- **Columns**: Well, Sample ID, Group Tag, Replicate, Measured Value, Inline Status Tag.
- **Clipboard**: `Cmd/Ctrl + C` copies tab-separated values (TSV); `Cmd/Ctrl + V` opens Paste Preview Modal.
- **Numbers**: Strict `tabular-nums` alignment.

### 2.7 `ReviewBanner` Contract
```go
func ReviewBanner(c *ui.Context, taskTitle string, onUnlock func())
```
- Displayed when viewing completed runs or historical steps.
- Color: Subtle neutral/amber banner (`#FDF3E7` Light / `#2A2216` Dark).
- Copy: `👁 正在回看已完成步骤: [taskTitle] · 当前数据已锁定为只读。`
- Action: "解锁并重新测定" (with explicit confirmation dialog).

---

## 3. Migration Roadmap for Production Modules

When engineering implements v1.2, every module replaces legacy raw code with these UI-Kit primitives:

1. `internal/shell`:
   - Replace raw vertical button stacks in `projectView()` with `SectionCard`, `Button(ButtonPrimary)`, and `StatusBadge`.
   - Embed `PrimaryRail` with proper SVG icons and tooltips.
2. `internal/designer`:
   - Replace comma-separated text inputs with structured `FormField` components.
   - Replace the 96 raw button matrix with `Plate96Widget`.
   - Localize all status messages (`idle` -> `● 就绪`).
3. `internal/protocol`:
   - Replace developer apologies with the professional `ReviewBanner` and `SectionCard`.
   - Implement `StepperChip` sequence.
4. `internal/runui` & `internal/resultgrid`:
   - Maintain 50/50 split; embed `ReviewBanner` for completed tasks.
   - Localize paste dialog and batch save actions.
5. `internal/tools`:
   - Connect dilution calculator outputs directly to `Designer` via structured callback payloads.
