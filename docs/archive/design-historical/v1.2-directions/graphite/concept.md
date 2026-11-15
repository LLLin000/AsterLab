# AsterLab v1.2 Visual Direction — Graphite (Tern × Linear Dark Workstation)

- **Direction Name**: Graphite
- **Inspiration**: Tern Agent IDE × Linear Dark Workstation × Raycast Floating Shell × Cursor IDE
- **Workspace Target**: 1200×760 App Window (with 900×600 compact fallback)
- **Primary Page**: CCK-8 Workflow Designer (L929 细胞毒性评价 / 酶标仪读板)
- **Artifacts**:
  - Interactive Prototype: `docs/design/v1.2-directions/graphite/index.html`
  - High-Resolution Capture: `docs/design/v1.2-directions/graphite/designer-1200x760.png` (1200×760 exact)
  - Standalone Vector Graphic: `docs/design/v1.2-directions/graphite/designer-1200x760.svg` (1200×760 viewBox)

---

## 1. Visual Concept & Design Philosophy

The **Graphite** direction treats AsterLab not as a generic SaaS admin dashboard or casual web form, but as a **high-precision scientific workstation**. It draws upon the design disciplines of Tern Agent IDE and Linear:

1. **Deep Neutral Layered Surfaces**: Eliminates warm/brownish mud and generic charcoal flat blocks. Employs mathematically calibrated cool-graphite elevation levels ($L_0$ to $L_4$) that provide natural visual hierarchy without visual clutter.
2. **Hairline Separators & Subtle Bevels**: Eliminates heavy solid borders. Structural boundaries use 1px hairlines at $5\%\sim 9\%$ white opacity combined with inner top-edge highlights (`inset 0 1px 0 rgba(255,255,255,0.07)`), evoking machined anodized aluminum and dark studio instruments.
3. **Compact Navigation Rail**: Replaces clumsy multi-column sidebars with an ultra-compact 48px icon rail with precision active notch indicators.
4. **Floating Command & Tool Strip**: A translucent acrylic dock (`backdrop-filter: blur(16px)`) floating centrally above the workflow canvas, giving instant access to node creation, connection modes, auto-alignment, and Raycast-style `⌘K` command palette.
5. **Restrained Electric Cool-Blue Accent**: Rejects generic purple SaaS gradients and garish CTA buttons. Accent is restrained to technical electric cobalt (`#4B88FF`), reserved strictly for active workflows, ports, and key scientific focal points.
6. **Subtle Diffuse Halo for Node Selection**: When a scientific node is selected, it does **not** turn into an unstyled colored button or solid colored card. It preserves its deep graphite surface and gains a soft, technical blue aura (`box-shadow: 0 0 0 1px rgba(75, 136, 255, 0.50), 0 0 20px -2px rgba(75, 136, 255, 0.28)`), keeping scientific content perfectly legible.
7. **Tabular Numerals & Monospace Micro-Typography**: Exact scientific measurements ($100\ \mu\text{L/孔}$, $5,000\ \text{cells/孔}$, $\text{OD}_{450}$) are formatted in monospace tabular numerals with strict unit alignments and 9px uppercase category tags.

---

## 2. Design Tokens & Color Palette

### 2.1 Dark Workstation Tokens (Graphite Slate Default)

| Token Name | CSS Variable | Hex / RGBA | Role / Usage |
| :--- | :--- | :--- | :--- |
| **L0 Void Canvas** | `--bg-app` | `#0B0C0E` | Deepest root canvas, background dot grid |
| **L1 Base Workbench** | `--bg-surface` | `#111317` | Titlebar, compact rail, lower inspector base |
| **L2 Elevated Panel** | `--bg-surface-elevated` | `#16191F` | Inspector header tabs, property cards, command palette |
| **L3 Node / Card** | `--bg-surface-card` | `#1C2027` | Unselected DAG nodes, microplate well surrounds |
| **L4 Hover / Active** | `--bg-surface-hover` | `#232832` | Interactive button hover, table row highlight |
| **Border Subtle** | `--border-subtle` | `rgba(255, 255, 255, 0.05)` | Internal cell dividers, minor field lines |
| **Border Regular** | `--border-regular` | `rgba(255, 255, 255, 0.09)` | Panel frames, node outlines, titlebar baseline |
| **Border Strong** | `--border-strong` | `rgba(255, 255, 255, 0.16)` | Selected card border, command palette container |
| **Top Bevel Highlight** | `--border-bevel` | `inset 0 1px 0 rgba(255,255,255,0.07)` | High-end machined hardware edge light |
| **Text Primary** | `--text-primary` | `#F0F2F5` | Primary node titles, values, column headers |
| **Text Secondary** | `--text-secondary` | `#9DA3AF` | Scientific metadata, parameter values, descriptions |
| **Text Tertiary** | `--text-tertiary` | `#5A606F` | Labels, units, structural keys, inactive ports |
| **Accent Primary** | `--accent` | `#4B88FF` | Electric cobalt, active selection indicator, ports |
| **Accent Hover** | `--accent-hover` | `#6098FF` | Button hover state |
| **Accent Soft** | `--accent-soft` | `rgba(75, 136, 255, 0.12)` | Active tab pill, selected badge background |
| **Accent Halo** | `--accent-halo` | `0 0 0 1px rgba(75,136,255,0.5), 0 0 20px -2px rgba(75,136,255,0.28)` | Selected scientific node diffuse aura |
| **Status Draft** | `--status-draft` | `#F59E0B` | Unsaved draft changes indicator |
| **Status Saving** | `--status-saving` | `#60A5FA` | Active sync / saving transition |
| **Status Saved** | `--status-saved` | `#34D399` | Core synchronized persistent state |
| **Status Error** | `--status-error` | `#F87171` | Validation error, boundary violation |

### 2.2 96-Well Microplate Scientific Tints

| Group / Spec | Dot Fill | Border Ring | Scientific Meaning in CCK-8 |
| :--- | :--- | :--- | :--- |
| **Blank (B)** | `rgba(148, 163, 184, 0.12)` | `#64748B` | Column 1: Cell-free medium + CCK-8 optical background |
| **Negative Control (NC)** | `rgba(94, 234, 212, 0.15)` | `#2DD4BF` | Column 2: Solvent vehicle (0 $\mu\text{g/mL}$) 100% viability baseline |
| **Gradient C1 (6.25 $\mu\text{g/mL}$)** | `rgba(96, 165, 250, 0.22)` | `#3B82F6` | Columns 3–4: Lowest dose, 4–8 biological replicates |
| **Gradient C2 (12.5 $\mu\text{g/mL}$)** | `rgba(99, 102, 241, 0.22)` | `#6366F1` | Columns 5–6: Medium-low dose |
| **Gradient C3 (25.0 $\mu\text{g/mL}$)** | `rgba(139, 92, 246, 0.25)` | `#8B5CF6` | Columns 7–8: Half-maximal inhibitory search window |
| **Gradient C4 (50.0 $\mu\text{g/mL}$)** | `rgba(168, 85, 247, 0.28)` | `#A855F7` | Columns 9–10: High toxicity dose |
| **Gradient C5 (100 $\mu\text{g/mL}$)** | `rgba(192, 132, 252, 0.32)` | `#C084FC` | Column 11: Maximal test dose |
| **Positive Control (PC)** | `rgba(244, 63, 94, 0.20)` | `#F43F5E` | Column 12: 1% DMSO / lethal toxicity reference |

### 2.3 Technical Light Palette (Paired Architectural Silver Slate)

The prototype includes a seamless Dark/Light toggle. In Light Mode:
- Canvas: `#F5F6F8` (Cool Architectural Silver)
- Surface: `#FFFFFF`
- Elevated: `#F8F9FA`
- Borders: `rgba(0, 0, 0, 0.08)`
- Text Primary: `#12151B`
- Accent: `#2563EB` (Precision Sapphire)

---

## 3. Typography & Micro-Hierarchy

- **Interface Sans Font Stack**: `-apple-system, BlinkMacSystemFont, "Inter", "Segoe UI", "PingFang SC", "Hiragino Sans GB", sans-serif`
- **Data & Numeric Mono Stack**: `"JetBrains Mono", "SF Mono", "Cascadia Code", Consolas, monospace`
- **OpenType Features**: `font-feature-settings: "tnum" 1, "cv01" 1, "zero" 1` (strict tabular numerals, slashed zero).

| Size | Weight | Line Height | Usage Scope |
| :--- | :--- | :--- | :--- |
| **9px** | 600 Mono | 1.0 | Uppercase category tags (`[MATERIAL]`, `[STEP]`, `[DEVICE]`), well coordinates |
| **10px** | 500 Mono | 1.2 | Parameter keys, step indices (`01`–`05`), units (`µL/孔`, `nm`, `cells`) |
| **11px** | 500 Sans | 1.3 | Input labels, secondary buttons, breadcrumb crumbs, legend labels |
| **12px** | 600 Sans | 1.3 | Node titles, column section titles, well group names |
| **13px** | 500 Sans | 1.4 | Primary text, toolbar buttons, breadcrumbs |
| **14px** | 600 Sans | 1.4 | Titlebar project title, modal headers |

---

## 4. Workstation Component Architecture & States

### 4.1 Default State (1200×760 Split View)
- **Upper Canvas (52% height, 374px)**:
  - 5 real CCK-8 nodes in logical sequence connected by cubic Bezier curves (`M x1 y1 C cx1 cy1, cx2 cy2, x2 y2`).
  - Connecting edges show directionality with 2.5px pulse dot markers.
  - Floating command strip centered at `top: 14px`.
- **Lower Inspector Workbench (48% height, 346px)**:
  - 36px tabular navigation bar: `步骤属性与仪器参数`, `96 孔板分组映射` (Active, badge `80 孔已分配`), `科学合约与终点校验`.
  - 3-column docked layout:
    - **Col 1 (270px)**: Operational Parameters (`Multiskan SkyHigh`, $\lambda_1 = 450\ \text{nm}$, $\lambda_{\text{ref}} = 650\ \text{nm}$, $5\text{s}$ orbital shaking, $\text{OD}_{\max} = 2.500\ \text{AU}$, blank subtraction policy).
    - **Col 2 (Flex 1)**: Visual 96-well plate mapping matrix with rows A–H and columns 1–12, 20px circular wells, group color rings, and bottom legend.
    - **Col 3 (290px)**: Group binding details for selected wells (reagent volume $80.0\ \mu\text{L}$, total volume $880.0\ \mu\text{L}$, seeded cells $40,000\ \text{cells}$, projected endpoint $\text{OD}_{450}$, replicate binding actions).

### 4.2 Scientific Node Interaction & Selected State
- **Unselected Node**: Background is a dark graphite gradient (`#191C23` to `#13151A`), hairline 1px border (`rgba(255,255,255,0.08)`), muted secondary typography.
- **Selected Node (Node #05: 酶标仪吸光度读板)**:
  - Retains dark graphite surface for perfect readability;
  - Border transitions to cool blue (`rgba(75, 136, 255, 0.55)`);
  - Soft diffuse ambient halo emerges: `box-shadow: inset 0 1px 0 rgba(255,255,255,0.12), 0 0 0 1px rgba(75,136,255,0.50), 0 0 20px -2px rgba(75,136,255,0.28)`;
  - Category tag and icon frame illuminate in `#4B88FF`;
  - Input port glows with electric cobalt, and the incoming curve from Node #04 transitions to `edgeActive` gradient with glow filter.

### 4.3 Inspector Expanded State
- Clicking `⤢ 展开检查器` (or pressing `⌘E`):
  - Canvas smoothly compresses from $52\%$ to $28\%$ height;
  - Lower inspector expands to $72\%$ height;
  - Microplate wells scale up to 26px with prominent well coordinate labels;
  - Col 1 and Col 3 expand to display advanced plate statistics, CV% tolerance limits, and pipetting route preview.

### 4.4 Visual Plate Mapping State
- Clicking any well or column (e.g. Column 3):
  - Selected column displays an active electric cobalt selection pill ring with subtle glow;
  - Header updates to `已选 8 孔 (A3 - H3)`;
  - Group card binds to `梯度组 C1 · 6.25 µg/mL (ZnO-NP 浸提液)`;
  - Volumetric metrics recalculate automatically:
    $$\text{CCK-8 Reagent} = 8 \times 10.0\ \mu\text{L} = 80.0\ \mu\text{L}$$
    $$\text{Total Volume} = 8 \times (100.0 + 10.0)\ \mu\text{L} = 880.0\ \mu\text{L}$$
    $$\text{Cell Count} = 8 \times 5,000 = 40,000\ \text{cells}$$

### 4.5 Save State Machine
AsterLab's state is directly tied to the core database revision:
1. **Draft State (`● 草稿`)**:
   - Indicator: Amber dot (`#F59E0B`), text: `● 草稿 (2 项未保存修改)`;
   - Context: Nodes or wells edited, uncommitted to SQLite/Core.
2. **Saving State (`⟳ 正在同步`)**:
   - Indicator: Pulsing blue dot (`#60A5FA`), text: `⟳ 正在同步至核心 v1.2...`;
   - Context: Serialization to JSON and transaction execution in background thread.
3. **Saved State (`✓ 已保存`)**:
   - Indicator: Mint emerald check (`#34D399`), text: `✓ 已保存至核心 v1.2-rev4 (14:32:05)`;
   - Context: Fully synchronized and verified with revision timestamp.

### 4.6 900×600 Compact Desktop Fallback
- Responsive adaptations for small screens (900×600 minimum native window):
  - 48px rail remains fixed on left;
  - Titlebar breadcrumbs compress secondary items;
  - Upper canvas nodes compress from 184px to 148px width, with 20px gap;
  - Lower inspector adjusts column grid to `220px 1fr 230px`;
  - Well cells scale to 18px diameter, omitting internal text while preserving color rings and tooltips;
  - Horizontal scrolling is supported on the DAG flow with zero clipping.

---

## 5. MyGo Native Desktop Implementation Feasibility

The Graphite direction was specifically engineered for direct implementation on AsterLab's native framework: **MyGo Native Desktop** (`github.com/egoist/mygo/ui` + Skia/Direct2D rendering):

| UI Component | MyGo Underlying Primitive | Native Implementation Plan |
| :--- | :--- | :--- |
| **Window Chrome & Titlebar** | `mygo.WindowOptions`, `ui.Row().DragWindow()` | Custom 40px frame, frameless window, system min/max/close controls, draggable titlebar row. |
| **Compact 48px Rail** | `ui.Column().Width(48)`, `ui.ButtonBase` | Fixed column, icon drawing via SVG vector paths, vertical active pill indicator. |
| **Floating Command Dock** | `ui.Box().Draw()`, `ui.HitTest` | Absolute positioned box floating over canvas. Translucent background drawn via Skia blur mask or pre-multiplied solid graphite. |
| **DAG Workflow Canvas** | `ui.Box().Draw()`, `ui.Painter` | Custom painter loop: draws background dot grid via `p.DrawCircle()` on 20px modulo; renders Bezier curves with `p.Path.CubicTo()`. |
| **Scientific Node Cards** | `ui.Box().Draw()`, `ui.Draggable` | Rounded rect `p.DrawRoundRect(r, 8, 8, fillPaint)`. Ports rendered as circular hit-test targets for connection dragging. |
| **Subtle Diffuse Halo** | `ui.Painter`, Skia `MaskFilter` | Drawn with Skia `CreateBlur(kNormal, 8.0)` and progressive opacity, avoiding expensive multi-pass shader recompilation. |
| **96-Well Microplate** | `ui.Box().Draw()`, `ui.HitTest` | 8×12 circular matrix drawn directly with vector circles. Supports box-selection dragging (`DragSelect`) and hover coordinates. |
| **Tabular Inspector Tabs** | `ui.SegmentedBase`, `ui.Row` | Standard MyGo segmented control with active container tint and text styling. |
| **Parameter Form Fields** | `ui.TextInputBase`, `ui.SelectBase` | Native keyboard focus with 1px accent focus ring. Tab key navigates cleanly between parameters. |
| **Command Palette (⌘K)** | `ui.DialogBase`, `ui.TextInputBase` | Top-level modal layer responding to global `KeyCmd+K` accelerator with fuzzy search list. |

---

## 6. What Was Rejected (Anti-Patterns Avoided)

1. **Rejected Generic Dashboard Cards**: Did not use bubbly white cards with heavy drop shadows and random statistical metrics. Every pixel belongs to the CCK-8 scientific workflow.
2. **Rejected Large Card Grids**: Did not turn the Designer into a masonry card wall. Kept the upper 52% as a true DAG workflow canvas and the lower 48% as a structured 3-column laboratory inspector.
3. **Rejected Heavy Borders & Dark-Mode "Lines Everywhere"**: Avoided thick solid `#39352D` borders that make dark themes feel like a spreadsheet jail. Used hairline $5\%\sim 9\%$ alpha lines with top bevel highlights.
4. **Rejected Purple SaaS Buttons & Gradient Fill**: Rejected trendy purple/violet CTA buttons. Used restrained electric cobalt (`#4B88FF`) only where actionable focus is required.
5. **Rejected Form Dumps & CSV Strings**: Replaced raw comma-separated well input strings (`"A1,A2,A3..."`) with a direct, interactive 96-well visual matrix.
6. **Rejected Node as Solid Colored Button**: Did not turn selected nodes into high-saturation solid blue blocks, which destroy readability of multi-line scientific data. Used a refined diffuse halo around a deep graphite surface.
