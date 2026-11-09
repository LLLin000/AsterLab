# AsterLab v1.2 — Native Desktop Design System Specification

- **Version**: 1.2.1 (Revised per Conditional Review)
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

## 3. Scientific Workflow Nodes Specification

Designer nodes represent biological materials, wet-lab procedures, and analytical devices. They are **topological tokens**, not form containers. Detailed attribute and well mapping belongs exclusively in the lower Inspector.

```
       [Input Port]                                          [Output Port]
           (o)----------------+---------------------------+-----(o)
                              |  [Hero SVG Icon 32x32]    |
                              |       <flask / vial>      |
                              |                           |
                              |  Node Title (12px bold)   |
                              |  [Compact Param Pill]     |
                              +---------------------------+
```

### 3.1 Three Visual Node Types

1. **Material Node (生物与化学材料节点)**:
   - **Primary Visual**: Hero scientific SVG illustration (`<path>` conical flask / cryogenic vial / reagent bottle / suspension tube).
   - **Visual Anchor**: Icon in upper 36×36px badge. Tinted in subtle neutral slate.
   - **Data**: Entity name (`L929 细胞悬液`, `受试化合物 A`), batch/passage (`P4`), concentration pill (`1.0×10⁴/孔`).
   - **Ports**: Output port on right edge (no input port).

2. **Operation Node (湿实验操作步骤节点)**:
   - **Primary Visual**: Hero scientific SVG illustration (`<path>` pipette / 8-channel microplate dispenser / incubator / plate washer).
   - **Visual Anchor**: Icon in upper 36×36px badge. Tinted in spruce teal.
   - **Data**: Operation verb (`微孔板接种`, `梯度给药`, `CCK-8 孵育`), condition pill (`100 µL/孔`, `37 °C 24h`).
   - **Ports**: Input port on left edge, Output port on right edge.

3. **Instrument / Device Node (分析仪器与读板节点)**:
   - **Primary Visual**: Hero scientific SVG illustration (`<path>` microplate reader / spectrophotometer optical beam / confocal laser / cytometer detector).
   - **Visual Anchor**: Icon in upper 36×36px badge. Tinted in high-contrast analytical spruce.
   - **Data**: Device name (`Multiskan 酶标仪`, `Bio-Rad CFX96`), detection mode pill (`OD450 吸光度`, `FAM/HEX 荧光`).
   - **Ports**: Input port on left edge, terminal status output port on right edge.

### 3.2 Four Interactive Node States

1. **Default (Resting)**: `1px solid Border`, `CardBackground`, ports visible as 6px circles with 1.5px stroke.
2. **Selected**: `2px solid Accent`, soft 4px spruce elevation glow (`box-shadow: 0 0 0 2px rgba(31,111,95,0.2)`), active port rings, drives immediate selection in the Inspector.
3. **Dragging**: `ZIndex(10)`, scaled by 1.02 (`ui.Pose{Scale: 1.02}`), subtle elevation shadow (`0 8px 16px rgba(0,0,0,0.12)`).
4. **Connecting (Port Target)**: Port circle pulses with Accent halo; invalid target ports dim to opacity 0.3.

---

## 4. Run Save Semantics & State Machine

AsterLab enforces honest, transactional save semantics. **The interface MUST NEVER display "已保存" or "已自动同步" before Core confirms persistence.**

```
[ 就绪 Ready (rev X) ]
         |
         | (Cell Edit / TSV Paste)
         v
[ 粘贴预览 Paste Preview ] ---> (Confirm) ---> [ 草稿 Draft (N 项未保存修改) ]
                                                      |
                                                      | (Click "批量保存读数")
                                                      v
                                        [ 提交中 Saving (Inputs Locked) ]
                                                      |
                       +------------------------------+------------------------------+
                       |                                                             |
                       v (Core Confirmed)                                            v (Conflict / Reject)
      [ 已保存 Saved (rev X+1) ]                                        [ 冲突/错误 Conflict (Error Banner) ]
```

### 4.1 State Definitions
1. **就绪 (Ready, rev X)**: Rest state. Result cells match frozen Core state; revision matches active snapshot.
2. **粘贴预览 (Paste Preview Modal)**: Triggered by `Cmd/Ctrl + V` or clicking "粘贴板读仪 TSV". Displays dimensions (`8 × 12 矩阵`), cell count (`96 孔`), and overwrite count (`覆盖已填数据: N 项`). Must be explicitly confirmed or dismissed by operator.
3. **草稿 (Draft, N 项未保存修改)**: Data in UI differs from Core snapshot. Status chip displays `● 草稿 (N 项未保存)` in Amber (`Warning`). Batch Save button is active.
4. **提交中 (Saving Batch...)**: Operator clicked "批量保存读数". Table inputs are temporarily disabled to prevent race conditions; button displays `⏳ 正在向 Core 提交批量保存…`.
5. **已保存 (Saved, rev X+1)**: Core returns atomic confirmation. UI updates snapshot revision, flashes green success badge for 3 seconds (`✓ 已保存 rev X+1 · Core 已确认`), and unlocks cells.
6. **冲突 / 错误 (Conflict / Error)**: Core returns `StaleRevision` or validation failure. UI retains operator draft inputs, surfaces an explicit warning banner, and offers "重新拉取并比对 (Rebase)" without discarding entered data.

---

## 5. Separation of Scientific Examples from Product Defaults

AsterLab is an extensible scientific workbench. Product defaults, calculation math, and quality checks must remain cleanly decoupled from specific biological testcases.

### 5.1 Sample Data Annotation Rule
Whenever biological examples are used in documentation, mockups, or seed fixtures (e.g., `L929`, `CCK-8`, `DMEM + 10% FBS`, `100 mM 储液`), they **MUST** be explicitly labeled with `[示例数据 / 演示用例]`. The application must never behave as if L929 or CCK-8 is hardcoded into the data model.

### 5.2 Scientific Calculator Purity (Math Only)
Calculators in `Tools` perform physical-chemical mathematical derivations only:
$$V_1 = \frac{C_2 \cdot V_2}{C_1}, \quad V_{\text{diluent}} = V_2 - V_1, \quad D = \frac{V_2}{V_1}$$
- **Prohibited**: Generating speculative procedural recipes (e.g., "涡旋振荡 10 秒", "室温避光保存", "使用 P10 移液器量取").
- **Required**: Generating exact physical quantities with unit conversions and explicit scientific boundary disclaimers:
  `"计算结果仅提供质量/浓度守恒推导；稀释液选择、移液器具校准及保存条件请依具体实验 SOP 执行。"`

### 5.3 Quality Check Taxonomy
1. **设计前置校验 (Design Pre-conditions)**: Evaluated at design-time in Designer (e.g., all 96 wells mapped, controls present, non-zero replicates).
2. **现场执行质控 (Execution Bench Checks)**: Evaluated by operator at wet bench in Run (e.g., pre-run temperature verified, blank well absorbance < 0.08 AU, cell suspension verified clump-free).

---

## 6. Two-Tier Desktop Responsive Discipline (1200×760 vs 900×600)

```
+------------------------------------------------------------------------------------------------+
| 1200×760 Standard: Full Dual-Surfaces & Multi-Zone Panels                                      |
| [Rail 56px] [Sidebar 220px] [Main Dual Work Surface / 3-Zone Inspector]                       |
+------------------------------------------------------------------------------------------------+
| 900×600 Compact: Strict Responsive Flow (Zero Horizontal Clipping)                            |
| [Rail 56px] [Header Dropdown (Sidebar 0px)]                                                    |
| Designer:  Canvas (auto-fit pan/scroll) + Inspector (3-Tab Switcher: 属性 | 孔板 | 合约)        |
| Run:       Single-Column Tabbed Surface ([ 操作指引 (3/5) ] | [ 读数表格 (96/96) ])             |
+------------------------------------------------------------------------------------------------+
```

### 6.1 Designer Responsive Contract at 900×600
- Canvas height adjusts to 240px; graph nodes automatically pan and fit into viewport with visible horizontal scroll indicators if required. No node is clipped off-screen.
- Lower Inspector converts from 3 horizontal columns into a top 3-tab segmented switcher:
  `[ 步骤属性 ] | [ 96孔板分组映射 (当前) ] | [ 检测合约与参数 ]`
- Each tab occupies the full available width, providing generous room for the 8×12 microplate grid without horizontal squishing.

### 6.2 Run Responsive Contract at 900×600
- The 50/50 dual pane converts into a single-pane tabbed view:
  `[ 操作指引与质控 (3/5) ] | [ 测定读数录入 (96/96) ]`
- A persistent, sticky bottom action bar keeps the "批量保存读数 (SaveBatch)" trigger immediately accessible regardless of active tab.

---

## 7. Chinese Scientific Copy & Punctuation Standards

Technical software in life sciences frequently suffers from either machine-translated jargon or developer confessions. AsterLab enforces strict editorial rules:

### 7.1 Terminology Harmonization Table

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

### 7.2 Punctuation & Spacing
1. **CJK + Western Spacing**: When Latin letters/numbers meet Chinese characters, include 1 half-width space (e.g., `CCK-8 读板记录`, `OD450 读数`, `共 96 孔`).
2. **Units & Punctuation**: Full-width punctuation (`，` `。` `：` `（）`) in descriptive prose. Half-width punctuation in mathematical/unit expressions (`37 °C`, `5% CO₂`, `10 µM`, `A1:H12`).
3. **No Developer Debug Strings**: Never display raw exception traces or internal enum names in the UI. All backend errors pass through `state.MapError()` to yield clear, actionable Chinese messages.
