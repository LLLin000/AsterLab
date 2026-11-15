# AsterLab v1.2 — Native Desktop Page Specifications

- **Version**: 1.2.1 (Revised per Conditional Review)
- **Scope**: Complete Information Architecture (IA), Layouts, Controls, States, and Core Data Contracts for all 5 native pages.
- **Standards**: Zero generic SaaS dashboard cards; zero equal card grids; zero developer confessions; strict 900×600 responsive discipline; honest transactional save semantics.

---

## Page 1: Project / Workspace (项目工作区)

### 1.1 Purpose & Scientific Context
The Project page is the operational launchpad for a laboratory scientist or research team. It organizes experimental designs, ongoing execution runs, and historical data sets under scientific project umbrellas (e.g. `[示例] L929 细胞毒性评价`, `[示例] 抗肿瘤小分子活性筛选`).

**Anti-Pattern Rejected**:
The existing Demo renders a raw, vertical stack of text and unstyled generic buttons (`继续上次工作`, `新建设计`, `导入`, followed by a flat button list of projects, designs, and runs). There is no metadata, no visual hierarchy, no timestamps, no state badges, and no sense of project context.

### 1.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 项目工作区 / 活跃项目选择器 [PRJ-088 · L929 生物相容性 v]     (Light/Dark)   |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Project Tree Sidebar | Main Project Work Surface (Padding 24px)                                  |
| 56px | (220px)              |                                                                           |
|      |                      | +-- Project Header -----------------------------------------------------+ |
|      | [搜索项目...]        | | Title: [示例] L929 生物相容性及细胞毒性评价 (ISO 10993-5)              | |
|      |                      | | 负责人: 实验组 A · 更新于: 2026-11-09 10:30 · 编号: PRJ-2026-088       | |
|      | ▼ 活跃研究项目 (3)   | +-----------------------------------------------------------------------+ |
|      |   ● L929 生物相容性  |                                                                           |
|      |   ○ MCF-7 药物耐药   | +-- Quick Action Strip -------------------------------------------------+ |
|      |   ○ 3D 肿瘤类器官    | | [▶ 继续当前运行: Day 3 读板]   [+ 新建实验设计]   [↑ 导入外部方案]    | |
|      |                      | +-----------------------------------------------------------------------+ |
|      | ▶ 归档项目 (12)      |                                                                           |
|      |                      | +-- Two-Column Scientific Overview (Gap 20px) --------------------------+ |
|      |                      | | Column A (55%): 实验设计 (Designs)     | Column B (45%): 实验运行 (Runs)| |
|      |                      | | [Card] [示例] CCK-8 细胞活力检测设计   | [Card] [示例] Run-20261109-01  | |
|      |                      | |   · 终点: OD450 · 6 组 · 3 重复        |   · 进度: 3/5 步骤已完成       | |
|      |                      | |   · 修订号: rev 4 · 关联 Protocol v2   |   · 状态: 进行中 (待读板)      | |
|      |                      | |   · [进入设计]  [创建运行]             |   · [进入记录]  [查看原始数据] | |
|      |                      | |                                        |                                |
|      |                      | | [Card] [示例] LIVE/DEAD 染色方案       | [Card] [示例] Run-20261107-01  | |
|      |                      | |   · 终点: 活死细胞计数 · 4 组          |   · 进度: 5/5 全部完成         | |
|      |                      | |   · 修订号: rev 1 · 关联 Protocol v1   |   · 状态: 已归档 (5 数据集)    | |
|      |                      | +----------------------------------------+--------------------------------+ |
+------+----------------------+---------------------------------------------------------------------------+
```

### 1.3 Key Controls & Interactive Behavior
1. **Project Switcher & Tree**:
   - Single-click on a project in the sidebar switches global project context.
   - Breadcrumb in the titlebar updates immediately.
2. **Continue Action ("继续当前运行")**:
   - High-prominence action button. If the active project has an uncompleted run, clicking this jumps directly to `Page 4 (Run)` at the current pending step.
   - If no run is in progress, the button gracefully changes to "从最新设计创建运行".
3. **Design Cards**:
   - Hover elevates card border to `BorderStrong` in 120ms.
   - "进入设计" opens the Experiment Designer.
   - "创建运行" creates a new `RunSnapshot` via Core and switches to the Run workspace.
4. **Run Cards**:
   - Clearly displays real-time progress (`3/5 步骤`), status tag (`进行中` / `已完成`), and last modification timestamp.

### 1.4 Compact Desktop Adaptation (900×600)
- Project Tree Sidebar collapses to 0px; project switcher in titlebar handles context switching.
- Column A (Designs) and Column B (Runs) transition into a single vertical scroll stack.
- Padding decreases from 24px to 14px; card internal padding adjusts to 10px. Zero horizontal clipping.

### 1.5 Core Data Mapping
- Backend models: `runs.ProjectRecord`, `runs.DesignSnapshot`, `runs.RunSnapshot`.
- Fields consumed: `project.id`, `project.title`, `project.description`, `design.revision`, `design.assays`, `run.status`, `run.tasks`.

---

## Page 2: Experiment Designer (实验设计器)

### 2.1 Purpose & Scientific Context
The Experiment Designer is where biological protocols are turned into structured, executable workflows: connecting biological materials (cell lines, compounds, media), procedural steps (seeding, treatment, incubation), and analytical devices (microplate reader, qPCR system, confocal microscope).

**Anti-Patterns Rejected**:
1. Comma-separated CSV strings for structured data (`"时间点: 24h, 48h"`, `"对照组: Control, Solvent"`).
2. 96 raw text buttons (`●A1`, `✓B2`) in a chaotic pile with no plate frame.
3. Raw status strings like `"idle"`, `"draft"`, `"saving"`, `"error"`.
4. Flat text boxes without scientific iconography.

### 2.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 实验设计 / [示例] L929 CCK-8 毒性评价 [rev 4]                 (Light/Dark)  |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Upper Zone: Workflow Canvas (58% Height ~ 380px)                                                 |
| 56px | +-- Canvas Toolbar (36px) --------------------------------------------------------------------+ |
|      | | [⤢ 适应画布] [− 100% +]  |  [+ 添加材料] [+ 添加步骤] [+ 添加仪器]  |  [↶ 撤销] [↷ 重做]      | |
|      | +---------------------------------------------------------------------------------------------+ |
|      |                                                                                                 |
|      |   (o)[ <Flask Icon> ](o)      (o)[ <Pipette Icon> ](o)    (o)[ <Reader Icon> ](o)              |
|      |      L929 细胞悬液      ====>     微孔板接种铺板     ====>    酶标仪读板测定 [选中]             |
|      |      [ 1.0x10⁴/孔 ]               [ 100 µL/孔 ]               [ OD450 吸光度 ]                  |
|      |                                                                                                 |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Lower Zone: Structured Inspector (42% Height ~ 276px)                                            |
|      | +-----------------------+------------------------------------------+----------------------------+ |
|      | | Zone A: 步骤参数      | Zone B: 96 孔板分组映射 (Visual Grid)   | Zone C: 检测合约与参数     | |
|      | | (Width: 260px)        | (Width: 540px)                           | (Width: 320px)             | |
|      | | 步骤名称:             |    1  2  3  4  5  6  7  8  9 10 11 12    | 检测类型: [ CCK-8 比色法 v]| |
|      | | [ 酶标仪读板测定    ] | A  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 检测终点: [ OD450 (AU)   ] | |
|      | | 节点类型: [ 仪器测定] | B  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 时间关系:                  | |
|      | | 关联组件:             | C  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | (•) 同一实验单位重复       | |
|      | | [ lab.well_plate_96 ] | D  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | ( ) 每时间点独立样本       | |
|      | | 执行条件:             | E  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 技术重复数: [ 3          ] | |
|      | | 波长: 450 nm          | F  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 对照组:     [ Control    ] | |
|      | | 震板: 5 秒 (中速)     | G  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 预估耗时:   [ 15 分钟    ] | |
|      | |                       | H  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    |                            | |
|      | |                       | [已选 18 孔] 分配至: [ 对照组 Control v] |                            | |
|      | +-----------------------+------------------------------------------+----------------------------+ |
|      | Bottom Action Bar (36px): [● 草稿修改未保存]  [保存设计]  [▶ 从当前设计创建运行]  (rev 4)         |
+------+--------------------------------------------------------------------------------------------------+
```

### 2.3 Detailed Control Specifications
1. **Interactive Workflow Nodes with Real Scientific SVG Icons**:
   - Each node features a prominent **36×36px Hero SVG Illustration (`<path>`)**:
     - **Material**: Cryovial / flask outline (`path d="M9 3h6v2h-1v5l4 9H6l4-9V5H9z"`).
     - **Operation**: Pipette liquid transfer (`path d="M19 3l2 2-9 9-2-2 9-9z M6 16l-3 5 5-3z"`).
     - **Instrument**: Microplate reader beam (`path d="M4 6h16v12H4z M9 10h6v4H9z"`).
   - Primary visual weight is anchored on the icon; node title and parameter pill (`100 µL`, `OD450`) are concise and secondary.
   - In-ports and out-ports (6px circles) support live Bezier connection drag.
   - States: Default, Selected (`2px solid Accent` + glow), Dragging (`Pose{Scale: 1.02}`), Connecting (port halo).
   - All granular attribute editing happens in the lower Inspector.
2. **Visual 96-Well Plate Layout Widget (Zone B)**:
   - True 8×12 SVG matrix with row labels `A–H` and column labels `1–12`.
   - Supports box-drag selection or multi-click toggle.
   - Assigned wells show distinct group tints (Control: Blue `#D9E8F5`, Treatment: Teal `#E1EFEA`, Blank: Gray `#EBEAE5`, Selected: `#1F6F5F`).
   - Group assignment bar: dropdown selects active group, clicking "应用到已选孔" updates assignments atomically.
3. **Bottom Action Bar**:
   - Real-time save status: `● 就绪 (rev 4)` / `● 草稿未保存` / `⏳ 正在保存…` / `✓ 已保存`.

### 2.4 True Responsive Adaptation at 900×600 (Strict Responsive Contract)
- **Canvas Auto-Fit & Pan/Scroll**: Canvas height scales to 240px. Graph nodes auto-fit within view; if graph exceeds width, smooth horizontal panning and subtle scroll indicators appear. **Nodes never overflow off the right window boundary.**
- **Inspector 3-Tab Switcher**: The 3 horizontal columns convert into a top segmented tab bar:
  `[ 步骤属性 ] | [ 96孔板分组映射 (当前) ] | [ 检测合约与参数 ]`
  Selecting a tab renders that pane full-width, providing generous space for the microplate matrix and form inputs without horizontal cramping.

### 2.5 Core Data Mapping
- Backend models: `workflow.WorkflowDocument`, `assays.Design`, `assays.WellAssignment`, `runs.DesignSaveInput`.

---

## Page 3: Protocol (实验方案)

### 3.1 Purpose & Scientific Context
The Protocol page renders the structured Standard Operating Procedure (SOP). It provides bench scientists with an authoritative, read-only sequence of actions, parameters, materials, safety warnings, and acceptance criteria.

**Anti-Patterns Rejected**:
1. Showing developer apologies: `"Core does not currently store structured reagent or instrument details here"`.
2. Showing system limitations to users: `"Protocol editing and saving are not supported by Core; this view is read-only"`.
3. Half-English, half-Chinese layout headers (`STEPS`, `Purpose`, `Parameters`, `Checks`).

### 3.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 实验方案 / [示例] L929 细胞微孔板接种操作规程 [v1.2]          (Light/Dark)  |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Step Navigation Rail | Main Protocol Document View (Scrollable)                                  |
| 56px | (230px)              |                                                                           |
|      |                      | +-- Protocol Header ----------------------------------------------------+ |
|      | [搜索步骤 / 参数...] | | Title: [示例] L929 细胞微孔板接种与培养操作规程 (SOP-CCK8-01)          | |
|      |                      | | 来源设计: L929 CCK-8 细胞毒性评价 · 固化版本: v1.2 · 生效日期: 2026-11 | |
|      | 01 试剂耗材准备      | | [🖨 导出 PDF / 打印]  [📋 复制步骤说明]  [🔍 查看关联设计]             | |
|      |   · 耗时: 15 min     | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      | 02 L929 细胞消化     | +-- Professional Guidance Banner ---------------------------------------+ |
|      |   · 耗时: 20 min     | | ℹ 本方案由实验设计 v1.2 固化生成。如需调整接种密度或参数，请在设计中调整 |
|      |                      | +-----------------------------------------------------------------------+ |
|      | 03 细胞悬液稀释      |                                                                           |
|      |   · 关键: 1x10⁵/mL   | +-- Section 1: 当前步骤详细指令 (步骤 04: 96 孔微孔板接种铺板) ---------+ |
|      |                      | | 操作指令: 使用多道移液器，沿微孔侧壁每孔加入 100 µL 细胞悬液。接种后    | |
|      | 04 微孔板接种 [当前] | | 十字交叉晃动混匀，置于 37 °C、5% CO₂ 培养箱孵育 24 小时。               | |
|      |   · 耗时: 15 min     | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      | 05 培养箱孵育 (24h)  | +-- Section 2: 关键工艺参数表 ------------------------------------------+ |
|      |   · 耗时: 24 h       | | 参数项          | 设定数值与单位     | 容许偏差     | 来源与监控方式    | |
|      |                      | | 接种体积        | 100 µL / 孔        | ± 2 µL       | 移液器标准档位    | |
|      |                      | | 细胞活率        | ≥ 95%              | ≥ 90% 下限   | 台盼蓝计数仪      | |
|      |                      | | 孵育环境        | 37.0 °C / 5.0% CO₂ | ± 0.5 °C     | 培养箱在线监控    | |
|      |                      | +-----------------+--------------------+--------------+-------------------+ |
|      |                      |                                                                           |
|      |                      | +-- Section 3: 质控与接受标准 (Quality Checks) -------------------------+ |
|      |                      | | [✓] 设计前置校验: 96 孔孔位均已完成组别绑定与对照设置    [ 前置校验 ]   | |
|      |                      | | [✓] 现场执行质控: 接种前镜检确认无细胞团块 (台盼蓝拒染)  [ 阻断项 ]     | |
|      |                      | | [✓] 现场执行质控: 接种后确认微孔底部无可见气泡           [ 提示项 ]     | |
|      |                      | +-----------------------------------------------------------------------+ |
+------+----------------------+---------------------------------------------------------------------------+
```

### 3.3 Key Controls & Separation of Checks
1. **Check Taxonomy Distinction**:
   - **设计前置校验 (Design Pre-conditions)**: Validated at compile/design time (e.g. well mapping complete).
   - **现场执行质控 (Execution Bench Checks)**: Validated by the wet-lab scientist at the bench (Blocking vs Advisory).
2. **Read-Only Professional Notice**:
   `ℹ 本方案由实验设计 v1.2 固化生成。如需调整接种密度或孵育参数，请在“实验设计”中调整并发布新版本。`

### 3.4 Compact Desktop Adaptation (900×600)
- Step navigation sidebar collapses into a top horizontal stepper chip bar (`01 准备` -> `02 消化` -> `03 稀释` -> `04 接种 [当前]`).
- Document padding adjusts to 14px.

---

## Page 4: Run (实验运行与读数记录)

### 4.1 Purpose & Scientific Context
The Run workspace is the mission-critical surface where the wet-lab researcher executes tasks and logs instrumental readouts at the physical bench. Precision, zero latency, and absolute data safety (preventing accidental overwrites of biological records) are paramount.

### 4.2 Layout Architecture (1200×760) — 50/50 Dual Work Surface

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 实验运行 / [示例] Run-20261109-01 (CCK-8 测定)                 (Light/Dark) |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Top Execution Stepper Strip (42px)                                                               |
| 56px | [D1 接种 ✓] ---> [D2 给药处理 ✓] ---> [D3 CCK-8 读板 (当前执行)] ---> [D3 显微镜检]   进度: 3/5  |
+------+--------------------------------------------------+-----------------------------------------------+
| Rail | Left Work Surface: 操作指引与质控检查 (50%)      | Right Work Surface: 96 孔板读数录入与批量保存 |
|      |                                                  |                                               |
|      | +-- 当前任务信息 ------------------------------+ | +-- 读数工作台 -----------------------------+ |
|      | | 步骤: CCK-8 酶标仪吸光度测定 (时间点: Day 3)  | | 状态: [● 草稿 (96 项修改未保存)]           |
|      | | 关联 Assay: CCK-8 比色法 · 终点: OD450 (AU)   | | 批操作: [📋 粘贴板读仪 TSV]  [📥 模板下载]  |
|      | +----------------------------------------------+ | +-------------------------------------------+ |
|      |                                                  |                                               |
|      | +-- 现场执行确认清单 (Readiness Checks) -------+ | +-- 96 孔测定数据表 (resultgrid) ----------+ |
|      | | [x] 酶标仪已预热至 37 °C 并完成自检           | | 孔位 | 样本 ID    | 分组    | OD450  | 状态 | |
|      | | [x] 培养板避光孵育满 2 小时整                 | | A01  | Control-01 | 对照组  | 1.245  | 原始 | |
|      | | [ ] 板底擦拭清洁，无冷凝水与划痕              | | A02  | Control-02 | 对照组  | 1.258  | 原始 | |
|      | +----------------------------------------------+ | | A03  | Control-03 | 对照组  | 1.239  | 原始 | |
|      |                                                  | | B01  | Dose-10-01 | 10 µM   | 1.012  | 原始 | |
|      | +-- 偏差与实验事件记录 (Deviations) -----------+ | | B02  | Dose-10-02 | 10 µM   | 1.008  | 原始 | |
|      | | [ 记录异常... 如: 读板延迟 5 分钟          ] | | B03  | Dose-10-03 | 10 µM   | 0.995  | 原始 | |
|      | +----------------------------------------------+ | | H12  | Blank-03   | 空白组  | 0.045  | 原始 | |
|      |                                                  | +-------------------------------------------+ |
|      | [✔ 确认本步操作完成并归档]                       |                                               |
|      |                                                  | Action Bar: [● 未保存修改] [💾 批量保存读数]  |
+------+--------------------------------------------------+-----------------------------------------------+
```

### 4.3 Honest Transactional Save Lifecycle
1. **Resting State**: `● 就绪 (rev 4)`.
2. **Paste TSV Modal**: Clicking "粘贴板读仪 TSV" displays modal:
   `已识别 8×12 阵列，将写入 96 个单元格。覆盖已填数据: 0 项。`
3. **Draft Unsaved State**: On confirmation, status updates to `● 草稿 (96 项修改未保存)`. **Never claims "已保存".**
4. **Submitting State**: On clicking "批量保存读数", inputs lock, button displays `⏳ 正在向 Core 提交批量保存…`.
5. **Saved State**: Core returns confirmation with new revision, button updates to `✓ 已保存 (rev 5 · Core 已确认)`.

### 4.4 Compact Desktop Adaptation (900×600)
- Dual 50/50 split converts to single-pane tabbed view:
  `[ 操作指引与质控 (3/5) ] | [ 测定读数录入 (96/96) (当前) ]`
- Sticky bottom action bar retains "批量保存读数" accessible across both tabs.

---

## Page 5: Tools & Components (资料与工具工作台)

### 5.1 Purpose & Scientific Context
Tools & Components is an actionable laboratory workspace providing scientific calculation, microplate design, protocol block drafting, and structured importing.

### 5.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 资料与工具 / 实验准备与计算工作台                              (Light/Dark)  |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Utility Catalog Rail | Active Utility Workspace (Main Surface)                                   |
| 56px | (210px)              |                                                                           |
|      |                      | [ Active Tool: 溶液稀释与移液配比计算器 (Dilution Calculator) ]           |
|      | ▼ 实验工具箱         |                                                                           |
|      |   ● 溶液稀释计算器   | 模式选择: (•) 单次稀释 (C1·V1=C2·V2)   ( ) 梯度倍比稀释   ( ) 试剂反应体系 |
|      |   ○ 96 孔板布局器    |                                                                           |
|      |   ○ 实验参数表格器   | +-- 输入条件 [示例用例: 100 mM 母液稀释至 10 µM] -----------------------+ |
|      |                      | | 储液浓度 (C1):    [ 100       ] [ mM  v]                              | |
|      | ▼ 模块与导入         | | 目标浓度 (C2):    [ 10        ] [ µM  v]                              | |
|      |   ○ 标准实验模块库   | | 目标体积 (V2):    [ 50        ] [ mL  v]                              | |
|      |   ○ 结构化 JSON 导入 | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      |                      | +-- 纯数学推导与液体配比结果 (Math Derivation Output) ------------------+ |
|      |                      | | 稀释倍数 (D): 10,000 ×                                                | |
|      |                      | | 所需母液体积 (V1 = C2·V2/C1): 5.0000 µL                               | |
|      |                      | | 所需稀释液体积 (V_diluent = V2 - V1): 49.9950 mL                      | |
|      |                      | | 声明: 计算仅提供质量/浓度守恒推导；混匀与保存请遵循实验 SOP。           | |
|      |                      | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      |                      | Action Bar: [📋 复制配比数据]  [💾 插入至当前实验设计]                    |
+------+----------------------+---------------------------------------------------------------------------+
```

### 5.3 Scientific Calculator Purity (Math Only)
- Computes $V_1 = \frac{C_2 \cdot V_2}{C_1}$ and $V_{\text{diluent}} = V_2 - V_1$.
- Strictly no automatic procedural assumptions ("涡旋 10 秒", "使用 P10 移液器").
- Explicit boundary disclaimer: `"计算结果仅提供质量/浓度守恒推导；稀释液选择、移液器具校准及保存条件请依具体实验 SOP 执行。"`

### 5.4 Compact Desktop Adaptation (900×600)
- Utility rail collapses to a top selector dropdown.
- Input cards and output formula stack cleanly with 12px padding.
