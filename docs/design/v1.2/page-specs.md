# AsterLab v1.2 — Native Desktop Page Specifications

- **Version**: 1.2.0 (Frozen Page Interaction Specification)
- **Scope**: Complete Information Architecture (IA), Layouts, Controls, States, and Core Data Contracts for all 5 native pages.
- **Standards**: Zero generic SaaS dashboard cards; zero equal card grids; zero developer confessions.

---

## Page 1: Project / Workspace (项目工作区)

### 1.1 Purpose & Scientific Context
The Project page is the operational launchpad for a laboratory scientist or research team. It organizes experimental designs, ongoing execution runs, and historical data sets under scientific project umbrellas (e.g. `L929 细胞毒性评价`, `抗肿瘤小分子活性筛选`).

**Anti-Pattern Rejected**:
The existing Demo renders a raw, vertical stack of text and unstyled generic buttons (`继续上次工作`, `新建设计`, `导入`, followed by a flat button list of projects, designs, and runs). There is no metadata, no visual hierarchy, no timestamps, no state badges, and no sense of project context.

### 1.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 项目工作区 / 活跃项目选择器 [L929 生物相容性评估 v]          (Light/Dark)   |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Project Tree Sidebar | Main Project Work Surface (Padding 24px)                                  |
| 56px | (220px)              |                                                                           |
|      |                      | +-- Project Header -----------------------------------------------------+ |
|      | [搜索项目...]        | | Title: L929 生物相容性及细胞毒性评价 (ISO 10993-5)                     | |
|      |                      | | 负责人: 实验组 A · 更新于: 2026-11-09 10:30 · 编号: PRJ-2026-088       | |
|      | ▼ 活跃研究项目 (3)   | +-----------------------------------------------------------------------+ |
|      |   ● L929 生物相容性  |                                                                           |
|      |   ○ MCF-7 药物耐药   | +-- Quick Action Strip -------------------------------------------------+ |
|      |   ○ 3D 肿瘤类器官    | | [▶ 继续当前运行: Day 3 读板]   [+ 新建实验设计]   [↑ 导入外部方案]    | |
|      |                      | +-----------------------------------------------------------------------+ |
|      | ▶ 归档项目 (12)      |                                                                           |
|      |                      | +-- Two-Column Scientific Overview (Gap 20px) --------------------------+ |
|      |                      | | Column A (55%): 实验设计 (Designs)     | Column B (45%): 实验运行 (Runs)| |
|      |                      | | [Card] CCK-8 细胞活力检测设计          | [Card] Run-20261109-01 (D3)    | |
|      |                      | |   · 终点: OD450 · 6 组 · 3 重复        |   · 进度: 3/5 步骤已完成       | |
|      |                      | |   · 修订号: rev 4 · 关联 Protocol v2   |   · 状态: 进行中 (待读板)      | |
|      |                      | |   · [进入设计]  [创建运行]             |   · [进入记录]  [查看原始数据] | |
|      |                      | |                                        |                                |
|      |                      | | [Card] LIVE/DEAD 荧光染色方案          | [Card] Run-20261107-01         | |
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
- Project Tree Sidebar collapses into a header dropdown picker.
- Column A (Designs) and Column B (Runs) transition from a 2-column layout into a single vertical scroll stack.
- Padding decreases from 24px to 16px; card internal padding adjusts to 12px.

### 1.5 Core Data Mapping
- Backend models: `runs.ProjectRecord`, `runs.DesignSnapshot`, `runs.RunSnapshot`.
- Fields consumed: `project.id`, `project.title`, `project.description`, `design.revision`, `design.assays`, `run.status`, `run.tasks`.

---

## Page 2: Experiment Designer (实验设计器)

### 2.1 Purpose & Scientific Context
The Experiment Designer is where biological protocols are turned into structured, executable workflows: connecting biological materials (cell lines, compounds, media), procedural steps (seeding, treatment, incubation), and analytical devices (microplate reader, qPCR system, confocal microscope).

**Anti-Patterns Rejected**:
1. Comma-separated CSV strings for structured data (e.g. `时间点: 24h, 48h`, `对照组: Control, Solvent`).
2. 96 raw text buttons (`●A1`, `✓B2`) in a chaotic pile with no plate frame.
3. Raw status strings like `"idle"`, `"draft"`, `"saving"`, `"error"`.

### 2.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 实验设计 / L929 CCK-8 细胞毒性评价 [rev 4]                    (Light/Dark)  |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Upper Zone: Workflow Canvas (58% Height ~ 380px)                                                 |
| 56px | +-- Canvas Toolbar (36px) --------------------------------------------------------------------+ |
|      | | [⤢ 适应画布] [− 100% +]  |  [+ 添加材料] [+ 添加步骤] [+ 添加仪器]  |  [↶ 撤销] [↷ 重做]      | |
|      | +---------------------------------------------------------------------------------------------+ |
|      |                                                                                                 |
|      |   [ 细胞悬液 ] --------> [ 铺板接种 ] --------> [ 药物处理 ] --------> [ CCK-8 孵育 ] --------> [ 酶标仪读板 ]
|      |   (L929 1x10⁴/孔)        (96 孔板, 100µL)      (6 梯度, 24h)         (10µL/孔, 2h)           (OD450 测定)
|      |                                                                                                 |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Lower Zone: Structured Inspector (42% Height ~ 276px)                                            |
|      | +-----------------------+------------------------------------------+----------------------------+ |
|      | | Zone A: 步骤参数      | Zone B: 96 孔板分组映射 (Visual Grid)   | Zone C: 检测合约与参数     | |
|      | | (Width: 260px)        | (Width: 540px)                           | (Width: 320px)             | |
|      | | 步骤名称:             |    1  2  3  4  5  6  7  8  9 10 11 12    | 检测类型: [ CCK-8 比色法 v]| |
|      | | [ 酶标仪读板        ] | A  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 检测终点: [ OD450 (AU)   ] | |
|      | | 节点类型: [ 操作步骤] | B  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 时间关系:                  | |
|      | | 关联组件:             | C  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | (•) 同一实验单位重复       | |
|      | | [ lab.well_plate_96 ] | D  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | ( ) 每时间点独立样本       | |
|      | | [ tool.reader       ] | E  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 技术重复数: [ 3          ] | |
|      | | 执行条件:             | F  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 对照组:     [ Control    ] | |
|      | | 波长: 450 nm          | G  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 阳性对照:   [ DMSO 10%   ] | |
|      | | 震板: 5 秒 (中速)     | H  ○  ○  ●  ●  ●  ●  ●  ●  ●  ●  ○  ○    | 预估耗时:   [ 15 分钟    ] | |
|      | |                       | [已选 24 孔] 分配至: [ 对照组 Control v] |                            | |
|      | +-----------------------+------------------------------------------+----------------------------+ |
|      | Bottom Action Bar (36px): [● 草稿修改未保存]  [保存设计]  [▶ 从当前设计创建运行]  (rev 4)         |
+------+--------------------------------------------------------------------------------------------------+
```

### 2.3 Detailed Control Specifications
1. **Interactive Canvas**:
   - Nodes render with category icons, title, and concise parameter pill.
   - Selection highlights node with `2px solid Accent` and updates Inspector in real time.
   - Dragging moves nodes smoothly; connecting ports draws cubic Bezier curves (`ui.Painter.StrokePath`).
2. **Visual 96-Well Plate Layout Widget (Zone B)**:
   - True 8×12 SVG matrix with row labels `A–H` and column labels `1–12`.
   - Supports box-drag selection or multi-click toggle.
   - Assigned wells show distinct group tints (Control: Blue `#D9E8F5`, Treatment: Teal `#E1EFEA`, Blank: Gray `#EBEAE5`).
   - Group assignment bar: dropdown selects active group, clicking "应用到已选孔" updates assignments atomically.
3. **Assay Contract (Zone C)**:
   - Dedicated fields for endpoint, method, replicates, and explicit timepoint relation radio group (`same_experimental_unit_repeated` vs `independent_sample_per_timepoint`).
4. **Bottom Action Bar**:
   - Real-time save status: `● 就绪 (rev 4)` / `● 草稿未保存` / `⏳ 正在保存…` / `✓ 已保存`.
   - "从当前设计创建运行" sends validated `DesignSnapshot` to Core and transitions smoothly into Run.

### 2.4 Compact Desktop Adaptation (900×600)
- Canvas height reduces to 240px; zoom auto-fits all nodes.
- Lower Inspector switches from a 3-column split to a tabbed interface:
  `[ 步骤参数 ] | [ 孔板分组映射 (当前) ] | [ 检测合约 ]`.

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
| Titlebar (48px): AsterLab / 实验方案 / L929 细胞培养与接种标准操作规程 [v1.2]             (Light/Dark)  |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Step Navigation Rail | Main Protocol Document View (Scrollable)                                  |
| 56px | (230px)              |                                                                           |
|      |                      | +-- Protocol Header ----------------------------------------------------+ |
|      | [搜索步骤 / 参数...] | | Title: L929 细胞复苏、传代与 96 孔板接种操作规程 (SOP-CCK8-01)         | |
|      |                      | | 来源设计: L929 CCK-8 细胞毒性评价 · 固化版本: v1.2 · 生效日期: 2026-11 | |
|      | 01 试剂预热与准备    | | [🖨 导出 PDF / 打印]  [📋 复制步骤说明]  [🔍 查看关联设计]             | |
|      |   · 耗时: 15 min     | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      | 02 细胞消化与计数    | +-- Section 1: 实验目的与原理 ------------------------------------------+ |
|      |   · 耗时: 20 min     | | 本方案规定了 L929 成纤维细胞接种于 96 孔微孔板的标准程序，确保每孔细胞  | |
|      |                      | | 密度一致性（CV < 8%），为后续 CCK-8 毒性测定提供可重现的细胞单层。     | |
|      | 03 细胞悬液稀释      | +-----------------------------------------------------------------------+ |
|      |   · 关键: 1x10⁵/mL   |                                                                           |
|      |                      | +-- Section 2: 当前步骤详细指令 (步骤 04: 微孔板接种) ------------------+ |
|      | 04 微孔板接种 [当前] | | 操作指令: 使用多道移液器，沿微孔侧壁每孔加入 100 µL 细胞悬液。接种后    | |
|      |   · 耗时: 15 min     | | 静置 15 分钟，十字交叉晃动混匀，置于 37 °C、5% CO₂ 培养箱孵育 24 小时。 | |
|      |                      | +-----------------------------------------------------------------------+ |
|      | 05 培养箱孵育 (24h)  |                                                                           |
|      |   · 耗时: 24 h       | +-- Section 3: 关键工艺参数与条件 --------------------------------------+ |
|      |                      | | 参数项          | 设定数值与单位     | 容许偏差     | 来源说明          | |
|      |                      | | 接种密度        | 10,000 cells/孔    | ± 5%         | 设计基准          | |
|      |                      | | 每孔体积        | 100 µL             | ± 2 µL       | 移液器校准规范    | |
|      |                      | | 孵育环境        | 37.0 °C / 5.0% CO₂ | ± 0.5 °C     | 细胞培养标准      | |
|      |                      | +-----------------+--------------------+--------------+-------------------+ |
|      |                      |                                                                           |
|      |                      | +-- Section 4: 试剂耗材与仪器清单 --------------------------------------+ |
|      |                      | | 试剂/耗材: 高糖 DMEM 培养基 (含 10% FBS, 1% 双抗) · 批号: 20261012      | |
|      |                      | | 仪器设备: Thermo Heracell 160i 培养箱 / Eppendorf Research Plus 8 道移液器 |
|      |                      | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      |                      | +-- Section 5: 关键质控检查项 (Checks) ---------------------------------+ |
|      |                      | | [✓] 接种前镜检: 细胞活率必须 >= 95% (台盼蓝拒染法)       [ 阻断项 ]     | |
|      |                      | | [✓] 接种后镜检: 确认孔底无气泡，细胞分布均匀              [ 提示项 ]     | |
|      |                      | +-----------------------------------------------------------------------+ |
+------+----------------------+---------------------------------------------------------------------------+
```

### 3.3 Key Controls & Interactive Behavior
1. **Left Step Sequence Rail (230px)**:
   - Chronological list of steps with estimated durations.
   - Clicking a step smoothly scrolls the right pane to the respective step header and updates the URL parameter (`step=3`).
2. **Read-Only Clarity**:
   - The page displays an informative status banner:
     `ℹ 本方案由实验设计 v1.2 固化生成。如需修改实验条件，请在“实验设计”中调整并发布新版本。`
   - Replaces all technical apologies with this professional, scientific notice.
3. **Quality Check Matrix**:
   - Explicit badges distinguishing `阻断检查项` (Blocking pre-condition) from `提示项` (Advisory quality check).

### 3.4 Compact Desktop Adaptation (900×600)
- Step navigation transforms into a top horizontal stepper chip bar (`01 准备` -> `02 消化` -> `03 稀释` -> `04 接种 [当前]`).
- Table fonts tighten to 11px; padding reduces to 12px.

### 3.5 Core Data Mapping
- Backend models: `protocol.ProtocolRecord`, `protocol.ProtocolVersion`, `blocks.Instance`.

---

## Page 4: Run (实验运行与读数记录)

### 4.1 Purpose & Scientific Context
The Run workspace is the mission-critical surface where the wet-lab researcher executes tasks and logs instrumental readouts at the physical bench. Precision, zero latency, and absolute data safety (preventing accidental overwrites of biological records) are paramount.

**Anti-Patterns Rejected**:
1. Stacking task instructions, checklist, and data tables vertically so the operator must scroll back and forth between reading and typing.
2. Mixing review mode with execution mode (editing historical completed steps by accident).
3. English jargon (`Plate`, `Table`, `Paste preview`, `Confirm overwrite`).

### 4.2 Layout Architecture (1200×760) — 50/50 Dual Work Surface

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 实验运行 / Run-20261109-01 (L929 CCK-8 细胞毒性评价)           (Light/Dark) |
+------+--------------------------------------------------------------------------------------------------+
| Rail | Top Execution Stepper Strip (42px)                                                               |
| 56px | [D1 接种 ✓] ---> [D2 给药处理 ✓] ---> [D3 CCK-8 读板 (当前执行)] ---> [D3 显微成像]   进度: 3/5  |
+------+--------------------------------------------------+-----------------------------------------------+
| Rail | Left Work Surface: 操作指引与质控检查 (50%)      | Right Work Surface: 96 孔板读数录入与批量保存 |
|      |                                                  |                                               |
|      | +-- 当前任务信息 ------------------------------+ | +-- 读数工作台 -----------------------------+ |
|      | | 步骤: CCK-8 酶标仪吸光度测定 (时间点: Day 3)  | | 视图切换: [● 表格视图] [○ 孔板视图] [全宽]  |
|      | | 关联 Assay: CCK-8 比色法 · 终点: OD450 (AU)   | | 批操作: [📋 粘贴板读仪 TSV]  [📥 模板下载]  |
|      | +----------------------------------------------+ | +-------------------------------------------+ |
|      |                                                  |                                               |
|      | +-- 执行前确认 (Readiness Checks) -------------+ | +-- 96 孔测定数据表 (resultgrid) ----------+ |
|      | | [x] 酶标仪已预热至 37 °C 并完成自检           | | 孔位 | 样本 ID    | 分组    | OD450  | 状态 | |
|      | | [x] 确认微孔板底无划痕、指纹及冷凝水          | | A01  | Control-01 | 对照组  | 1.245  | 原始 | |
|      | | [ ] 避光孵育满 2 小时整                       | | A02  | Control-02 | 对照组  | 1.258  | 原始 | |
|      | +----------------------------------------------+ | | A03  | Control-03 | 对照组  | 1.239  | 原始 | |
|      |                                                  | | B01  | Dose-1-01  | 10 µM   | 1.012  | 原始 | |
|      | +-- 偏差与实验事件记录 (Deviations) -----------+ | | B02  | Dose-1-02  | 10 µM   | 1.008  | 原始 | |
|      | | [ 记录异常... 如: 读板延迟 5 分钟          ] | | B03  | Dose-1-03  | 10 µM   | 0.995  | 原始 | |
|      | +----------------------------------------------+ | | C01  | Blank-01   | 空白组  | 0.045  | 原始 | |
|      |                                                  | +-------------------------------------------+ |
|      | [✔ 确认本步操作完成并归档]                       |                                               |
|      |                                                  | Action Bar: [✓ 已保存 rev 5] [💾 批量保存读数] |
+------+--------------------------------------------------+-----------------------------------------------+
```

### 4.3 Key Controls & Interactive Behavior
1. **50/50 Dual Work Surface**:
   - Left side stays pinned with protocol instructions, timer, readiness checks, and deviation logger.
   - Right side hosts the high-density interactive `resultgrid` data table.
   - Zero scrolling required to correlate instructions with readings.
2. **Review Mode vs. Active Execution Mode**:
   - Clicking an already-completed step in the top stepper switches to **"任务回看 (Task Review)"** mode.
   - In Review mode:
     - Top banner indicates: `👁 正在回看已完成步骤 · 数据处于只读锁定状态`
     - Table cells are read-only.
     - "批量保存" is hidden, and an "解锁重测 (需要确认)" secondary button is provided.
3. **Plate Reader TSV Fast-Paste**:
   - Scientist copies 8×12 raw TSV grid from BioTek / Molecular Devices reader software.
   - Pressing `Cmd/Ctrl + V` or clicking "粘贴板读仪 TSV" triggers a non-destructive **Paste Preview Dialog**:
     `已识别 8×12 阵列，将写入 96 个单元格。覆盖已填数据: 0 项。`
   - "确认应用粘贴" maps values directly into the result grid.
4. **Authoritative Batch Save**:
   - Single click on "批量保存读数" sends `BatchReadoutInput` to Core with atomic revision guard (`expected_revision: a.snap.Revision`).
   - Success state triggers instant `✓ 已保存 rev X` feedback.

### 4.4 Compact Desktop Adaptation (900×600)
- Dual 50/50 split collapses into top toggle buttons: `[ 操作指引 (3/5) ] | [ 读数表格 (已录入 96/96) ]`.
- Floating bottom bar keeps "批量保存" accessible at all times.

### 4.5 Core Data Mapping
- Backend models: `runs.RunSnapshot`, `runs.RunTask`, `runs.ResultCell`, `runs.BatchReadoutInput`, `runs.CompleteTaskInput`.

---

## Page 5: Tools & Components (资料与工具工作台)

### 5.1 Purpose & Scientific Context
Tools & Components is an actionable laboratory workspace providing scientific calculation, microplate design, protocol block drafting, and structured importing.

**Anti-Patterns Rejected**:
1. Isolated toys: Calculators that output floating-point text into nowhere without any way to push values to Designer or Run.
2. English labels (`WORKSPACE`, `Plate layout`, `Use plate in Designer`).
3. Dead buttons for deprecated legacy views.

### 5.2 Layout Architecture (1200×760)

```
+---------------------------------------------------------------------------------------------------------+
| Titlebar (48px): AsterLab / 资料与工具 / 实验准备与计算工作台                              (Light/Dark)  |
+------+----------------------+---------------------------------------------------------------------------+
| Rail | Utility Catalog Rail | Active Utility Workspace (Main Surface)                                   |
| 56px | (210px)              |                                                                           |
|      |                      | [ Active Tool: 溶液稀释与配比计算器 (Dilution Calculator) ]               |
|      | ▼ 实验工具箱         |                                                                           |
|      |   ● 溶液稀释计算器   | 模式选择: (•) 单次稀释 (C1·V1=C2·V2)   ( ) 梯度倍比稀释   ( ) 试剂反应体系 |
|      |   ○ 96 孔板布局器    |                                                                           |
|      |   ○ 实验参数表格器   | +-- 输入条件 -----------------------------------------------------------+ |
|      |                      | | 储液浓度 (C1):    [ 100       ] [ mM  v]                              | |
|      | ▼ 模块与导入         | | 目标浓度 (C2):    [ 10        ] [ µM  v]                              | |
|      |   ○ 标准实验模块库   | | 目标体积 (V2):    [ 50        ] [ mL  v]                              | |
|      |   ○ 结构化 JSON 导入 | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      |                      | +-- 计算结果与移液配方 (Live Output) -----------------------------------+ |
|      |                      | | 所需母液体积 (V1): 5.0000 µL                                          | |
|      |                      | | 所需溶剂体积 (V_diluent): 49.9950 mL                                  | |
|      |                      | | 稀释倍数: 10,000 ×                                                    | |
|      |                      | | 操作说明: 使用 P10 移液器取 5.0 µL 母液，加入 49.995 mL 溶剂中充分混匀 | |
|      |                      | +-----------------------------------------------------------------------+ |
|      |                      |                                                                           |
|      |                      | Action Bar: [📋 复制配方文本]  [💾 插入至当前设计方案]  [导出计算记录]    |
+------+----------------------+---------------------------------------------------------------------------+
```

### 5.3 Detailed Utility Specifications
1. **Utility 1: 溶液稀释计算器 (Dilution Calculator)**:
   - Modes: `单次稀释 (C1·V1=C2·V2)`, `梯度系列稀释 (Serial Dilution)`, `多孔试剂反应体系 (Reaction Mix)`.
   - Automatic unit normalization (M, mM, µM, nM, pM; L, mL, µL).
   - Generates step-by-step pipetting instruction.
   - Action: "插入至当前设计方案" creates a structured `tool.dilution` block instance in the active design.
2. **Utility 2: 96 孔板布局器 (96-Well Plate Designer)**:
   - Full 8×12 interactive microplate sandbox.
   - Pre-configured templates: CCK-8 活力测定、qPCR 384/96 靶标排布、ELISA 双抗夹心布局。
   - Assign sample IDs, group tags, replicate numbers.
   - Action: "应用至实验设计" exports `assays.Design` JSON directly to Core.
3. **Utility 3: 实验参数表格器 (Parameter Table Builder)**:
   - Key-value-unit editor with row reordering and type constraints (numerical, categorical, range).
   - Action: "输出为 Protocol 参数块".
4. **Utility 4: 标准实验模块库 (Component Library)**:
   - Visual catalog of validated biological operation blocks:
     - 离心操作 (Centrifugation: RPM, RCF/g, Temp, Time)
     - 恒温孵育 (Incubation: Temp, CO₂, Shaking RPM)
     - 荧光读板 (Fluorescence Detection: Ex, Em, Gain)
     - 流式分析 (Flow Cytometry: Channels, Gating, Event count)
5. **Utility 5: 结构化 JSON 导入器 (Structured Importer)**:
   - Textarea with live syntax highlighting and JSON schema validation.
   - Supports importing external protocol definitions or assay designs into the active workspace with pre-flight linting.

### 5.4 Compact Desktop Adaptation (900×600)
- Utility catalog sidebar collapses to a top compact selector.
- Dilution calculator inputs convert to a 2-column compact grid; output formula strip stays anchored at the bottom.

### 5.5 Core Data Mapping
- Backend models: `blocks.Instance`, `blocks.Definition`, `assays.Design`, `tools.Workspace`.
