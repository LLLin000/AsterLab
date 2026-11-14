# Aurora Direction — Raycast × Cursor Hybrid Design Specification

> **Workspace**: AsterLab CCK-8 Designer (v1.2)  
> **Visual Direction**: Aurora (`Raycast × Cursor` Hybrid)  
> **Base Commit**: `50bbe4d`  
> **Target Viewport**: 1200 × 760 (Strict Desktop Baseline)  
> **Target Audience**: Bench scientists, laboratory automation engineers, assay developers  

---

## 1. Visual Direction & Design Concept

### 1.1 Design Philosophy: Scientific Instrument Meets Modern Developer Tooling
Traditional bio-laboratory software oscillates between two extremes: archaic Windows 98-style grey tables (Bio-Rad, SoftMax Pro) or flat, decorative SaaS dashboards with poor information density and excessive blank padding.

**Aurora** introduces a **Raycast × Cursor hybrid visual direction**:
1. **From Cursor**: A compact, multi-panel developer-grade arrangement. The workflow canvas, structural navigation tree, microplate layout, and step parameter inspector coexist within a single 1200×760 viewport with zero dead space. High density, monospace precision, hairline borders, and unambiguous information hierarchy.
2. **From Raycast**: A deep, focused obsidian atmosphere with a fast, keyboard-first **Floating Command Palette (`⌘K`)**. Acrylic glass translucency (`backdrop-filter: blur(24px)`) is employed **strictly for floating overlays and popovers**, keeping core editing canvases completely solid, dark, and high-contrast.
3. **Aurora Signature Accent**: A cold, razor-sharp **Cyan/Teal spectrum** (`#22D3EE` / `#06B6D4` / `#0D9488`). Active pipeline connections emit a controlled luminous edge glow (`box-shadow: 0 0 12px rgba(34, 211, 238, 0.35)`), highlighting the current kinetic trajectory of liquid handling without visual clutter.

---

## 2. Design Tokens & Visual Hierarchy

### 2.1 Color Palette

| Token | Hex / Value | Semantic Role |
|---|---|---|
| `--bg-base` | `#080B10` | Deepest root canvas; matte obsidian |
| `--bg-panel` | `#0D1117` | Structural panels (Sidebar, Inspector, Canvas dock) |
| `--bg-surface` | `#161B22` | Cards, input fields, well inactive backgrounds |
| `--bg-overlay` | `rgba(13, 17, 23, 0.82)` | Floating Command Palette & Tooltips (Blur 24px) |
| `--border-subtle` | `rgba(255, 255, 255, 0.08)` | Default hairline dividers (1px) |
| `--border-bold` | `rgba(255, 255, 255, 0.16)` | Active section boundaries and panel splitters |
| `--border-glow` | `rgba(34, 211, 238, 0.45)` | Selected node edge / luminous glow boundary |
| `--cyan-accent` | `#22D3EE` | Primary cold cyan accent (selection, active ports) |
| `--teal-accent` | `#06B6D4` | Secondary teal (connection curves, progress) |
| `--teal-deep` | `#0D9488` | Deep teal (confirmed runs, valid checks) |
| `--text-primary` | `#F0F6FC` | Primary labels, critical parameters, headings |
| `--text-secondary`| `#8B949E` | Secondary metadata, unit labels, shortcuts |
| `--text-muted` | `#484F58` | Breadcrumb separators, grid coordinates, placeholders |
| `--badge-dirty` | `#F59E0B` | Unsaved draft pill / dirty indicator |
| `--badge-saved` | `#10B981` | Saved revision pill (rev 4) |
| `--badge-running`| `#38BDF8` | In-flight execution / plate sync |

### 2.2 Microplate 96-Well Concentration Gradient Tokens
For the L929 CCK-8 cytotoxicity assay, well assignments use an optical density / concentration progression:

| Plate Group | Fill Token | Border Token | Description |
|---|---|---|---|
| **Blank (B)** | `#161B22` | `#30363D` | Medium without cells (Wells A1–H1, A12–H12) |
| **Control (0 µM)**| `#0E3A42` | `#0891B2` | Vehicle control (Wells A2–H2) |
| **Dox 0.1 µM** | `#083344` | `#06B6D4` | Dose tier 1 (Wells A3–H3, A4–H4) |
| **Dox 0.5 µM** | `#0C4A6E` | `#0EA5E9` | Dose tier 2 (Wells A5–H5, A6–H6) |
| **Dox 1.0 µM** | `#075985` | `#38BDF8` | Dose tier 3 (Wells A7–H7, A8–H8) |
| **Dox 5.0 µM** | `#0369A1` | `#22D3EE` | Dose tier 4 (Wells A9–H9) |
| **Dox 10.0 µM** | `#0284C7` | `#67E8F9` | Dose tier 5 (Wells A10–H10) |
| **Dox 50.0 µM** | `#0369A1` | `#A5F3FC` | High dose (Wells A11–H11) |
| **Selected Well** | `#22D3EE` | `#FFFFFF` | Active edit selection (White halo + Cyan core) |

### 2.3 Typography & Metrics
- **UI Sans**: `-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Inter", sans-serif`
  - Titlebar / Tabs: `12px / 600`, letter-spacing: `0.02em`
  - Node Title: `13px / 600`
  - Section Header: `11px / 700`, uppercase, letter-spacing: `0.06em`
  - Form Label / Body: `12px / 400`
- **Monospace (Numbers, Well Coordinates, CAS, Units)**: `"SF Mono", "Cascadia Code", "JetBrains Mono", Consolas, monospace`
  - Well Index: `10px / 600`
  - Parameter Values (`100 µL`, `OD450`, `1.0×10⁴`): `11px / 500`
  - Timestamps & Hash: `10px / 400`

---

## 3. Structural Component Layout (1200 × 760)

```
+---------------------------------------------------------------------------------------------------------------+
| TITLEBAR (40px) [● ● ●] PRJ-088: L929 CCK-8 细胞毒性评价  [rev 4 ● 未保存]  [ ⌘K 命令面板 ]   Core: 2.1ms (FFI) |
+-----------------+-------------------------------------------------------------+-------------------------------+
| ACTIVITY &      | WORKFLOW CANVAS (Height: 330px)                             | MULTI-TAB INSPECTOR           |
| OUTLINE (190px) | +-- Canvas Floating Toolbar [⤢ 适应] [100%] [＋] [▷] ------+ | (Width: 290px)                |
|                 |                                                             |                               |
| [▼ 流程总览]    |  [细胞悬液]  ==glow==>  [96孔接种]  ==glow==>  [梯度给药*]   | 步骤名称: 梯度给药 (Dox)      |
| 01. 细胞悬液    |  (1.0×10⁴/孔)          (100 µL/孔)             (6个浓度梯度)  | 试剂: 阿霉素盐酸盐            |
| 02. 96孔接种    |                                                       | 溶剂: 0.1% DMSO in DMEM       |
| 03. 梯度给药*   |                                          ║ Luminous   | 分配方式: 8通道排枪等量注入   |
| 04. CCK-8孵育   |  [酶标仪读板] <==glow== [CCK-8 孵育] <══════════╝ Edge       | 单孔加药量: 10 µL/孔          |
| 05. 酶标仪读板  |  (OD450/630)           (10 µL, 2h)                          | 孵育时间: 24.0 h (37℃, 5%CO₂) |
|                 +-------------------------------------------------------------+-------------------------------+
| [▼ 耗材与试剂]  | 96-WELL MICROPLATE MAPPING (Height: 350px)                  | [ 质控约束 ]                  |
| · 96孔平底黑框板 |  1   2   3   4   5   6   7   8   9  10  11  12                 | ✓ 边缘防蒸发孔预注 PBS        |
| · Dojindo CCK-8 | A ○   ●   ●   ●   ●   ●   ●   ●   ●   ●   ●   ○                 | ✓ 每组重复数 ≥ 3 (当前 n=6)   |
| · Doxorubicin   | B ○   ●   ●   ●   ●   ●   ●   ●   ●   ●   ●   ○                 | ✓ DMSO 终浓度 < 0.1% (合格)   |
|                 | C ○   ●   ●   ●   ●   ●   ●   ●   ●   ●   ●   ○                 |                               |
|                 | ... (D-H Rows with 6 replicate columns)                     | [ 保存设计 ⌘S ] [ 提交运行 ▷ ]|
+-----------------+-------------------------------------------------------------+-------------------------------+
| STATUSBAR (24px) [git: main] [板位: 60/96孔 (62.5%)] [Node 03: 梯度给药]                     [UTF-8] [Aurora UI v1.2] |
+---------------------------------------------------------------------------------------------------------------+
```

### 3.1 Distinctive Elements vs Other Directions
1. **Integrated 2-Dimensional Layout**:
   - The 96-well plate is not crammed into a bottom 3-column accordion or hidden behind a modal. It sits right below the workflow graph, establishing a direct visual relationship between the active node (`梯度给药`) and its corresponding well layout.
2. **Raycast Command Palette (`⌘K`)**:
   - Scientists can trigger instantaneous searches for reagents, auto-layout algorithms ("边缘注入蒸发保护", "6梯度棋盘式排布", "添加阴性对照"), or trigger direct execution without digging into nested menus.
3. **Cursor-Style Information Density**:
   - The left structural tree allows 1-click jumps between experiment steps, while the right multi-tab inspector provides atomic parameter validation with live compliance checkmarks.

---

## 4. Interaction States & Transitions

### 4.1 Node States (Workflow Graph)
- **Idle / Normal**: Surface `#161B22`, border `1px solid rgba(255,255,255,0.08)`, muted icon.
- **Hover**: Border `1px solid rgba(34, 211, 238, 0.4)`, cursor pointer, subtle 1px translate up.
- **Selected (Active Focus: 梯度给药)**:
  - Border: `1.5px solid #22D3EE`
  - Outer Glow: `box-shadow: 0 0 16px rgba(34, 211, 238, 0.25)`
  - Header Tag: Cyan badge with glowing pulse dot.
- **Connection Edges**:
  - Unselected: Hairline stroke `rgba(255,255,255,0.12)`, width 1.5px.
  - Selected / Active Flow: Smooth cubic Bezier stroke `#06B6D4`, width 2.5px, with animated directional pulse dash and luminous ambient bloom (`filter: drop-shadow(0 0 4px rgba(34, 211, 238, 0.6))`).

### 4.2 Microplate 96-Well Matrix States
- **Default Well**: Filled with group token (Blank / Control / Dox 0.1–50 µM).
- **Well Hover**: Scale `1.15`, white border `1.5px solid #FFFFFF`, launches floating translucent tooltip with well coordinates (`D05`), group name (`Dox 0.5 µM`), dispense volume (`10 µL`), replicate index (`Rep 4/6`).
- **Multi-select Drag Box**: Cyan dashed bounding box (`border: 1px dashed #22D3EE`, fill `rgba(34, 211, 238, 0.08)`).
- **Assigned Pulse**: When group assignment is confirmed, target wells flash briefly with a teal ripple animation.

### 4.3 Save State Lifecycle
- **Dirty (Unsaved Draft)**: Status badge displays `● 未保存修改 (3 changes)`, amber pulse dot `#F59E0B`. Shortcut `⌘S` enabled.
- **Saving**: Status badge switches to `⏳ 正在保存 revision 5...`, spinner active, save button disabled with loading indicator.
- **Saved (Synced)**: Status badge displays `✓ 已保存 (rev 5 · 22:31:04)`, emerald badge `#10B981`.

---

## 5. MyGo Desktop (v0.8.3) Native Feasibility Analysis

Can this visual direction be implemented in the MyGo native desktop engine without a Chromium WebView?

### 5.1 Native Feasibility Breakdown

| UI Element | MyGo v0.8.3 Primitives | Feasibility | Implementation Notes |
|---|---|---|---|
| **Multi-Panel Window Shell** | `ui.Flex`, `ui.Splitter` | **100% Native** | Tested in `prototypes/mygo-run`; standard flex row/col tree with custom titlebar. |
| **Workflow Graph & Nodes** | `ui.Canvas`, `DrawPath`, `DrawRoundRect` | **100% Native** | Vector cubic Bezier paths (`canvas.CubicTo`) and rounded cards render natively at 60+ FPS. |
| **Luminous Glowing Edges** | Multi-pass stroke with alpha decay or Skia BlurMaskFilter | **100% Native** | Native canvas draws 3 concentric strokes (`width=6 alpha=0.1`, `width=4 alpha=0.2`, `width=2 alpha=1.0`) to produce crisp hardware-accelerated neon bloom without GPU shader overhead. |
| **96-Well Microplate Grid** | Nested `ui.Flex` or `ui.Canvas` matrix | **100% Native** | In `prototypes/mygo-run/internal/resultgrid`, the 12×8 grid is drawn in ~45 lines of Go code with instant mouse hit-testing. |
| **Floating Command Palette (`⌘K`)** | `ui.Overlay` / `ui.Modal` with DirectComposition Acrylic | **High (Native Alternative)** | DirectComposition on Windows 11 natively supports Acrylic Blur (`DwmSetWindowAttribute`). In pure software canvas fallback, a dark tinted semi-transparent backdrop (`rgba(13, 17, 23, 0.94)`) achieves identical contrast and visual weight. |
| **Monospace Scientific Typography** | DirectWrite / FreeType font binding | **100% Native** | Bundled `Cascadia Code` or system `Consolas` delivers crisp pixel-aligned numerical readouts. |

### 5.2 Verdict
The Aurora direction is **fully viable for MyGo desktop implementation**. Unlike heavy web-based DOM glassmorphism that consumes excessive composite memory, Aurora's strict discipline of **solid dark panels + translucent overlays + vector-drawn luminous paths** maps directly onto standard native 2D drawing pipelines (Direct2D / Skia / CoreGraphics).
