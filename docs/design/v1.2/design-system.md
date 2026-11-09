# AsterLab v1.2 — Native Desktop Design System Specification

- **Version**: 1.2.0 (Frozen Design System Specification)
- **Target Platform**: MyGo Native Desktop (`github.com/egoist/mygo`, Windows 10/11, macOS 12+, Linux/GTK)
- **Target Resolutions**: 1200×760 DIP (Standard Workstation), 900×600 DIP (Minimum Compact Laptop)
- **Design Philosophy**: Quiet Precision (克制精密), Native Craft (原生工法), Scientific Clarity (科学明晰), Workflow First (流程驱动)
- **Governing Rule**: Reject generic SaaS dashboard cards, equal card grids, gradient/glass decoration, and developer confessions/debug strings exposed to end users.

---

## 1. Design Tokens & Palette

AsterLab uses a single-source semantic design token architecture mapped natively to `internal/theme`. Pages and widgets must never introduce ad-hoc hex colors.

### 1.1 Color Tokens (Light & Dark)

| Token Name | Light Value | Dark Value | Semantic Purpose & Usage |
| :--- | :--- | :--- | :--- |
| `Canvas` | `#F6F4EF` | `#171614` | Window background, canvas bedrock. Warm, calm, non-glare laboratory paper tone. |
| `Surface` | `#FBFAF7` | `#201E1A` | Primary container surface, sidebars, toolbars, card base. |
| `Elevated` | `#FFFFFF` | `#28251F` | Popovers, dropdown menus, modal dialogs, elevated inspection panels. |
| `Border` | `#DDD7CC` | `#39352D` | Structural 1px hairline dividers, card boundaries, table cell grids. |
| `BorderStrong` | `#C8C0B2` | `#514B40` | Focused boundaries, active card strokes, table headers. |
| `Text` | `#201E1A` | `#F0EDE6` | High-contrast primary text, values, headings, active states. |
| `TextMuted` | `#6F6A60` | `#A8A196` | Secondary labels, units, timestamps, metadata, inactive icons. |
| `Accent` | `#1F6F5F` | `#4FA38D` | Deep Spruce Green. Primary actions, selected items, active rail tabs, focus ring. |
| `AccentHover` | `#195C4F` | `#60B39D` | Pointer hover state over primary accent controls. |
| `AccentPressed` | `#144A3F` | `#3D8270` | Active press / mouse-down state on primary controls. |
| `AccentText` | `#FFFFFF` | `#10201C` | Text drawn on top of Accent background. |
| `Success` | `#2F7D4A` | `#55A76D` | Completed steps, verified parameters, valid ranges, pass checks. |
| `Warning` | `#A9651D` | `#D5964D` | Unsaved drafts, pending verifications, non-blocking deviations. |
| `Danger` | `#A13B36` | `#D26B62` | Blocking validation errors, stale conflict, destructive delete, invalid wells. |
| `RailBackground`| `#101318` | `#0E1014` | Global 56px navigation rail background (dark anchor on both light/dark). |
| `SidebarBg` | `#FBFAF7` | `#1E1C18` | Collapsible navigation/directory sidebar background. |

### 1.2 Scientific Domain Tokens

Laboratory and assay states require strict, unambiguous visual coding:

| Domain Token | Light Mode | Dark Mode | Semantic Meaning |
| :--- | :--- | :--- | :--- |
| `WellEmpty` | `#EFECE6` (border `#DDD7CC`) | `#25231F` (border `#39352D`) | Unassigned well on 96-well plate. |
| `WellAssigned` | `#E1EFEA` (border `#8EC1B3`) | `#1B362F` (border `#2D6153`) | Assigned well (has sample/group/replicate). |
| `WellSelected` | `#1F6F5F` (text `#FFFFFF`) | `#4FA38D` (text `#10201C`) | Well currently in operator selection box. |
| `WellControl` | `#D9E8F5` (border `#7AA7D0`) | `#192E42` (border `#305C84`) | Negative / Positive control well. |
| `WellBlank` | `#EBEAE5` (border `#C8C0B2`) | `#2A2824` (border `#4A453C`) | Blank / Medium-only reference well. |
| `StatusRaw` | `#201E1A` (badge `#EAE6DD`) | `#F0EDE6` (badge `#2E2A24`) | Raw instrumental measurement directly read from device. |
| `StatusDerived` | `#1F6F5F` (badge `#E1EFEA`) | `#4FA38D` (badge `#1B362F`) | Calculated value (e.g., mean, normalized OD, ΔΔCt). |
| `StatusMissing` | `#A9651D` (badge `#FDF3E7`) | `#D5964D` (badge `#382A1A`) | Cell expected a value but operator recorded nothing. |
| `StatusInvalid` | `#A13B36` (badge `#FCEBEA`) | `#D26B62` (badge `#3D1B1A`) | Value parsed as NaN, out-of-range, or malformed. |
| `EdgeFlow` | `#B5ADA0` | `#4F493E` | Directed dependency arrow connecting workflow nodes. |
| `EdgeActive` | `#1F6F5F` | `#4FA38D` | Selected or executing step connection edge. |

---

## 2. Typography & Numerical Formatting

Scientific workflows demand maximum legibility for dense tabular data, chemical formulae, biological identifiers, and multi-line Chinese SOP instructions.

### 2.1 Font Stacks

```css
/* Standard User Interface Typography */
--font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;

/* Monospace & Tabular Data Typography (Lab readings, coordinates, hashes) */
--font-mono: "SF Mono", Consolas, "Cascadia Code", "Liberation Mono", Menlo, monospace;
```

**OpenType Features Requirement**:
All numerical displays (tables, well coordinates, plate matrices, dilution concentrations) **MUST** enable `font-variant-numeric: tabular-nums` (OpenType `tnum`). Digits 0–9 must share identical glyph advance widths so that columns never jitter during live updates.

### 2.2 Type Scale & Hierarchy

| Role | Size (DIP) | Weight | Line Height | Tracking | Usage |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `Micro / Tag` | 11px | 600 (Semibold) | 14px | +0.02em | Well coordinates (A1-H12), status pill, badge, timestamp. |
| `Meta / Cell` | 12px | 400 (Regular) | 16px | 0 | Secondary labels, table cell data, stepper chip text, units. |
| `Body` | 13px | 400 (Regular) | 19px | 0 | Standard form inputs, SOP instructions, inspection paragraphs. |
| `Body Strong` | 13px | 600 (Semibold) | 19px | 0 | Field labels, table column headers, active list items. |
| `Subhead` | 15px | 600 (Semibold) | 21px | -0.01em | Panel titles, workflow node titles, card headers. |
| `Title` | 18px | 650 (Bold) | 24px | -0.015em | Page section headers, inspector title, drawer title. |
| `Display` | 20px | 700 (Bold) | 26px | -0.02em | Page title, modal dialog headline. |

---

## 3. Spacing, Elevation & Metrics

### 3.1 Spacing Grid (Multiples of 4)
- `Spacing 4`: 4px (micro gap between well buttons, badge inner padding).
- `Spacing 8`: 8px (standard gap between related controls, input vertical padding).
- `Spacing 12`: 12px (form row gap, compact card padding, toolbar button gap).
- `Spacing 16`: 16px (standard panel padding, section gap, table horizontal cell inset).
- `Spacing 24`: 24px (major work surface padding, column gutter).
- `Spacing 32`: 32px (page layout margin, large modal separation).

### 3.2 Corner Radii
- `Radius 4`: Interactive controls (`Button`, `TextInput`, `Select`, `Badge`, `Chip`).
- `Radius 8`: Containers (`Panel`, `Card`, `Sidebar`, `Workflow Node`, `Drawer`).
- `Radius 12`: Top-level overlays (`Modal Dialog`, `Popover`, `Floating Toolbar`).

### 3.3 Elevation & Shadows
AsterLab avoids heavy, blurry drop shadows that create visual mud in desktop CAD/scientific tools.
- **Flat Surface**: Default for all panels. Defined solely by `1px solid Border`.
- **Raised Card**: `border: 1px solid Border; box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.04)`.
- **Floating Overlay / Dialog**: `border: 1px solid BorderStrong; box-shadow: 0 8px 24px -4px rgba(0, 0, 0, 0.12), 0 2px 6px -1px rgba(0, 0, 0, 0.06)`.

---

## 4. Shell & Window Architecture

```
+---------------------------------------------------------------------------------------+
| Window Titlebar (48px) - DragWindow, Breadcrumb, Project Switcher, Global State, Help |
+----+-------------------+--------------------------------------------------------------+
| R  | Context Sidebar   | Main Work Surface                                            |
| a  | (220px, Collapsed |                                                              |
| i  | at 900x600)       | [Page-specific Canvas, 50/50 Split, or Multi-pane Grid]      |
| l  |                   |                                                              |
| 56 |                   |                                                              |
| px |                   |                                                              |
+----+-------------------+--------------------------------------------------------------+
| Bottom Status Bar (28px) - Connection, Revision, Stale Guard, Unsaved Indicator       |
+----+-------------------+--------------------------------------------------------------+
```

### 4.1 Global Titlebar (48px)
- **Window Dragging**: Supports native drag (`ui.Row().DragWindow()`).
- **Brand / Breadcrumbs**: Clean, non-distracting path: `AsterLab` / `[项目选择器]` / `[当前页面名称]`.
- **Global Actions**: Status indicator (`● 就绪` / `⏳ 同步中`), Theme Switcher (Light / Dark), Keyboard Shortcut cheatsheet button.

### 4.2 Primary Navigation Rail (56px)
- Dark background (`#101318` in Light, `#0E1014` in Dark) anchors the left boundary across all themes.
- **Brand Mark**: Top 36×36px square with 8px radius, featuring spruce teal monogram `AL`.
- **Nav Items** (Top to Bottom):
  1. `项目` (Project/Workspace) — Icon: `folder`
  2. `设计` (Experiment Designer) — Icon: `git-branch` / `workflow`
  3. `方案` (Protocol) — Icon: `file-text`
  4. `运行` (Run) — Icon: `play-circle`
  5. `工具` (Tools & Components) — Icon: `grid` / `tool`
- **Active State**: Left 3px spruce indicator bar, background `#222832`, active icon tinted `AccentText`.
- **Keyboard Shortcuts**: `Cmd/Ctrl + 1..5` immediately switches pages.

### 4.3 Context Sidebar (220px)
- Default open at 1200×760; automatically collapses into drawer or icon toggle at 900×600.
- Keyboard toggle: `Cmd/Ctrl + B`.
- Holds context-sensitive navigation: project tree, step list, assay list, tool catalog.

---

## 5. Control States & Interactive Feedback

Every control in AsterLab must implement the complete 8-state model. Unhandled states are prohibited.

```
[Resting] ---> (Hover) ---> [Hovered]
   |                           |
   +--------> (Press) <--------+
                 |
                 v
             [Pressed]
                 |
                 v
             [Focused] (via Keyboard Tab: 2px Accent Ring)
```

1. **Resting (Default)**: Clean, subtle 1px border, high contrast readable text.
2. **Hover**: Surface tint transitions in 120ms (`SurfaceHover` / `AccentHover`), cursor changes to `pointer` or `text`.
3. **Pressed**: Scale down by 0.99 or darken background by 8% (`AccentPressed`).
4. **Focused (Keyboard)**: Native focus ring drawn via `c.Theme().Focus`: 2px wide, 1px offset, color `Accent`. Never shown on mouse click (`:focus-visible` semantics).
5. **Disabled**: Opacity 0.42, cursor `not-allowed`. Screen-readers retain access with explanatory reason (`AccessDisabled`).
6. **Loading / Saving**: Spinner replacement or text change with `⏳` indicator, inputs locked against duplicate submission.
7. **Error / Invalid**: 1.5px `Danger` border, red-tinted background, explicit inline error tooltip.
8. **Empty**: Structured empty state illustration with single actionable button, never a blank gray void.

---

## 6. Motion, Transitions & Reduced Motion

Motion in AsterLab serves exactly one purpose: **spatial continuity**. It communicates where an item came from and where it went. Decorative spring bounces, playful wiggle, or slow fades are strictly banned.

### 6.1 Transition Specifications
- **Panel Collapse / Expand**: `160ms cubic-bezier(0.16, 1, 0.3, 1)` (ease-out snappy).
- **Selection / Tab Indicator**: `120ms ease-out`.
- **Modal / Popover Enter**: `140ms ease-out`, scale from `0.97` to `1.0`, opacity `0 -> 1`.
- **Data Table Batch Update**: Cells flash subtle `Accent` tint for 250ms then settle into resting state.

### 6.2 Reduced Motion Mode
- When OS setting `prefers-reduced-motion` is active or `t.ReduceMotion == true`:
  - `Duration` and `ExitDuration` are set to `-1` (instantaneous switch).
  - No translation animations; simple instant visibility toggle.
  - Zero camera pan/zoom animations on the experiment canvas.

---

## 7. Multi-DPI & Compact Desktop Discipline

The interface must scale seamlessly across high-DPI displays (125%, 150%, 200%) and compact screens (900×600 DIP).

### 7.1 Pixel Grid Alignment (Snap to DIP)
- All structural borders use exact 1 DIP metrics.
- Subpixel border anti-aliasing is avoided by aligning container boundaries to integer DIP positions.
- Microplate circles: Drawn with centered SVG vector paths, ensuring crisp rendering at both 100% and 150% Windows scaling.

### 7.2 The 900×600 Compact Desktop Contract
When window size drops to 900×600:
1. Primary Rail remains 56px fixed width.
2. Context Sidebar collapses to 0px, accessible via a floating/header toggle button.
3. 50/50 Dual Work Surfaces (such as Run) convert into top-tabbed single panels: `[ 操作指引 ] | [ 读数记录 (当前) ]`.
4. Table rows drop from 32px to 28px height; padding tightens from 16px to 10px.
5. All interactive buttons maintain a minimum 30×30px touch/click target.

---

## 8. Chinese Scientific Copy & Punctuation Standards

Technical software in life sciences frequently suffers from either machine-translated jargon or developer confessions. AsterLab enforces strict editorial rules:

### 8.1 Terminology Harmonization Table

| Prohibited Jargon / Demo Copy | Approved AsterLab Copy | Rationale |
| :--- | :--- | :--- |
| `Run 模块` / `执行器` | `实验运行` | Standard wet-lab phrasing. |
| `Workflow 流程` / `DAG 画布` | `实验设计` | Focus on research design, not graph tech. |
| `只读步骤` / `Protocol 视图` | `实验方案` (或 `Protocol`) | Clear SOP distinction. |
| `CCK-8 read` | `CCK-8 读板` | Natural lab action. |
| `Core does not store structured details here` | `此步骤尚未绑定结构化试剂或仪器` | User-facing status, not architectural apology. |
| `Rust RunService 不可用` | `后台实验服务未连接，请检查本地环境` | Actionable user message, not internal module name. |
| `idle` / `draft` / `saving` / `saved` | `就绪` / `草稿未保存` / `保存中…` / `已保存` | Localized lifecycle statuses. |
| `Technical ID: f82b-01a` | `条目编号：f82b-01a` | Professional cataloguing. |
| `Same experimental unit repeated` | `同一实验单位重复` | Accurate statistical biology concept. |
| `Independent sample per timepoint` | `每时间点独立样本` | Clear distinction from continuous sampling. |

### 8.2 Punctuation & Spacing
1. **CJK + Western Spacing**: When Latin letters/numbers meet Chinese characters, include 1 half-width space (e.g., `CCK-8 读板记录`, `OD450 读数`, `共 96 孔`).
2. **Units & Punctuation**: Full-width punctuation (`，` `。` `：` `（）`) in descriptive prose. Half-width punctuation in mathematical/unit expressions (`37 °C`, `5% CO₂`, `10 µM`, `A1:H12`).
3. **No Developer Debug Strings**: Never display raw exception traces or internal enum names in the UI. All backend errors pass through `state.MapError()` to yield clear, actionable Chinese messages.
