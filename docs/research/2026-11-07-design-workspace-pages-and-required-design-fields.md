# 设计工作区与其余页面：匹配方案与实验设计必需字段

- 日期：2026-11-07
- 状态：研究/讨论稿，不是 UI 规格
- 当前基线：根路径已经是 View 工作区（上流程图、下组件）；`/workspace` 仍是旧功能工作台
- 证据：`web/view.html`、`web/app.js`、`web/index.html`、`CONTEXT.md`、`AGENT_API.md`、`docs/prototypes/tabler-vs-dify-2026-11-06/index.html`

## 1. 先统一“整体 UI”的定义

设计稿真正的结构是：

```text
56px 图标侧栏
└─ 56px 顶栏（项目/设计、保存状态、操作）
   └─ 主工作区
      ├─ 上半：流程图 / 画布（280px 或 140px 收起）
      └─ 下半：当前节点或对象的全组件详情
```

不是“侧栏 + 卡片首页”，也不是“先弹一个四步表单”。

因此其余页面不应继续使用旧 `web/index.html` 的 232px 侧栏和整页卡片列表；它们应复用同一个 Rail + Topbar，只在主工作区换“内容形态”。

## 2. 两种主工作区形态

### A. Canvas + Detail（画布 + 下方组件）

用于有流程/关系的页面：

- 实验设计
- Protocol
- Run 执行

```text
┌ rail ┬ topbar ───────────────────────────────┐
│      ├─ 上半：关系/流程画布（可收起）          │
│      ├─ 下半：当前对象详情 + 全组件网格        │
│      │   Plate 8/12 · Calculator 4/12 · Table 12/12
└──────┴────────────────────────────────────────┘
```

统一要求：

- 只保留一个主滚动容器：下半详情滚动，画布固定。
- 选中节点 → 下半组件切换到该节点。
- 收起流程后，下半自动获得高度。
- 新增/绑定/审阅始终在组件内完成，不再弹整页 modal。

### B. Index + Detail（列表 + 详情）

用于没有流程的画廊/记录页：

- 项目
- 资料与记录
- 工具/资料表

```text
┌ rail ┬ topbar ───────────────────────────────┐
│      ├─ 筛选/搜索条                           │
│      ├─ 主列表或表格（左/中）                  │
│      └─ 选中详情（右侧抽屉或下方展开）          │
└──────┴────────────────────────────────────────┘
```

不做假画布；不要为了“统一”给记录页硬塞一个流程图。

## 3. 页面级匹配方案

| 页面 | 主工作区 | 上半 | 下半/详情 | 主要动作 |
|---|---|---|---|---|
| 项目 | Index + Detail | 无画布 | 项目概览、设计/运行/最近活动 | 新建项目、打开设计、备份 |
| 实验设计 | Canvas + Detail | 设计关系图：组别→检测→时间/样本 | 设计组件：研究问题、比较、组别、检测、时间、样本、材料、检查 | 编辑、审阅、生成 View、创建 Run 前检查 |
| Protocol | Canvas + Detail | 步骤链/嵌套步骤树 | 当前步骤：指令、参数、材料、仪器、检查、版本 | 新建版本、预览、绑定参数 |
| 工作区 View | Canvas + Detail | 组件流程 | 当前 operation + Plate/Calculator/Table/Note | 编辑布局、审阅 Proposal |
| 实验运行 | Canvas + Detail | 冻结步骤链 | 当前步骤、孔板、读数、检查、Lineage | 记录 Actual、追加事件、偏差 |
| 记录 | Index + Detail | 无 | 筛选表 + 选中记录详情 | 搜索、导出、追加更正 |
| 工具 | Index + Detail | 无 | 工具卡 + 参数/结果 | 计算、生成候选板图 |
| 资料表 | Index + Detail | 无 | 表格 + 列定义 | 编辑表、导出 |

### 视觉统一规则

- 侧栏只保留 56px 图标入口；hover 显示名称；当前项用 accent 左线/软背景。
- 顶栏只放：面包屑、保存/审阅状态、主题、备份、必要的 primary action。
- 主区不使用第二层侧栏；需要列表时用左列或抽屉，不新增 232px 导航。
- 卡片统一使用 12 列：Plate 8、Calculator 4、Table/Note 12。
- 所有空状态只给一个主行动，不堆说明。
- 设计稿中的 `Execution brief`、`Checklist / Gate`、`Proposal` 继续作为标准组件。

## 4. 实验设计页面必须出现的组件

当前设计数据模型已有许多字段，但 UI 的 `design-form` 只暴露了一部分；以下按“设计页下半组件”组织，而不是继续做四步弹窗。

### 4.1 必填/首次保存前应出现

| 组件 | 字段 | 当前状态 |
|---|---|---|
| 研究问题与目标 | `objective`；建议增加 `research_question`、`hypothesis` | `objective` 有；`hypothesis` schema 有但 UI 无独立输入；`research_question` 未在 schema |
| 比较设计 | 比较组/条件；因素与水平；主要比较 | 组别条件有；因素/水平无结构化字段 |
| 实验单位 | `experimental_unit` | 有 |
| 样本量与重复 | `independent_n`、`technical_repeats`、`sample_size_rationale` | 前两项有；`sample_size_rationale` schema 有但 UI 无 |
| 组别与对照 | `group_specs`、`role`、assay `control_group_ids` | 有组别；对照映射有；对照角色分类较粗 |
| 检测与终点 | `assays[]`：title、endpoint、method、unit、protocol_version_id | 有 |
| 时间安排 | `timepoints`、`timepoint_timing`、`time_anchor_event`、`timepoint_relation` | 有 |
| 样本/材料关系 | `sample_allocations`、`planned_outputs` | 有 |
| 设计检查项 | 未解决字段、冲突、影响 | 有 map/gaps；设计页需固定显示 |

### 4.2 方法学与偏倚控制（当前 schema 有、UI 未暴露）

这些字段已经在设计 payload/schema 中，但当前表单没有稳定的可见输入：

- `sample_size_rationale`
- `randomization`
- `blinding`
- `inclusion_criteria`
- `exclusion_criteria`
- `analysis_plan`
- `controls`
- `replicate_plan`
- `independent_variables`
- `hypothesis`
- `planned_run_count`
- `planned_run_relationship`

建议在“设计页下半”增加一个 **Study quality** 组件：

```text
样本量与重复理由
随机化 / 盲法（可为 N/A）
纳入 / 排除标准
分析计划
对照与接受/拒绝标准
```

空值保留为空，不以“无”代替“未填写”。

### 4.3 Readout 与 acceptance

每个 Assay 组件必须能表达：

- raw readout：OD、Cq、CFU count、图像
- derived result：blank-corrected、ΔCq、CFU/mL
- derivation method/version
- controls：blank/negative/positive/vehicle/NTC/NRT
- acceptance window / LOD / count window
- not detected / below LOD / above ULOQ
- gate：blocking / advisory

这不是额外 assay-specific UI；继续用通用组件字段。

### 4.4 当前数据模型缺口

| 缺口 | 影响 | 最小方案 |
|---|---|---|
| `research_question` 不是独立字段 | 目标和问题混在 `objective` | 增加可选字段，旧数据回退到 objective |
| 因素/水平没有结构化字段 | 只能写在组别 condition 文本 | 先增加 `factors[]` 文本/值表；不要先做统计模型 |
| 组别对照角色较粗 | NTC/NRT/vehicle 等需靠文本 | 用 `role + subtype`，语义仍是通用 control |
| 设计页没有版本/审阅状态组件 | 设计草案和 Run 前置状态不清晰 | 加 Design status 组件：draft/needs review/ready/conflict |
| 设计编辑仍是大 modal | 与整体工作区割裂 | 迁移为 Canvas + Detail 的设计模式 |

## 5. 建议的第一轮实现顺序（只讨论，不立即执行）

1. 先做“设计页”的 Canvas + Detail 外壳：上半设计关系图，下半只读卡片；先不替换旧表单。
2. 把现有四步设计表单中的字段拆成 3 个组件：
   - Design question
   - Groups & unit
   - Assays & timepoints
3. 加 `Study quality` 组件，接上 schema 已有但 UI 未暴露的字段。
4. 加 `Readout & controls` 组件，绑定 Assay 的 endpoint/controls/readout 语义。
5. 设计页支持“审阅/生成 View/创建 Run 前检查”，再替换旧 modal。
6. 最后迁移 Protocol、Run、Records、Tools 到同一 Rail/Topbar。

## 6. 需要你确认的 4 个设计决策

1. **设计页上半画什么？**
   - A. 组别 → 检测 → 时间/样本的关系图
   - B. 直接用 Protocol 步骤链
   - C. 两者切换（设计关系 / 执行步骤）

2. **设计页下半是编辑还是只读？**
   - A. 卡片只读，点开组件后编辑
   - B. 卡片内直接编辑
   - C. 只读审阅 + 右侧 Inspector

3. **侧栏文字如何出现？**
   - A. 56px 图标，hover 显示名称（当前设计稿）
   - B. 悬停展开为 180px 带文字
   - C. 始终 80px 图标 + 小字

4. **Study quality 字段的首轮范围？**
   - 只做样本量理由、随机化、盲法、纳排、分析计划
   - 还是同时加 factors/levels 结构化表

这些确定后再进入实现；否则又会把“设计页”做成另一个表单弹窗。

## 7. 对架构审查的收敛

审查提出的核心判断成立：**Scientific Core 比前端成熟，不能把流程画布当成完整产品。**
需要在设计中固定三条边界：

1. **Design / Protocol / Run 三张画布同壳不同源。**
   - Design：组别、因素/水平、检测、时间、样本分配、共享制备。
   - Protocol：方法步骤、参数、配方、仪器程序、来源、检查。
   - Run：冻结步骤、实际参数、检查结果、样本/孔位、读数、lineage。
   不能继续用 `view.project()` 的 ProtocolPreview 步骤链充当 Design Map。

2. **不支持必须显式可见。**
   当前 `renderDetail()` 遇到 unsupported renderer 会 `continue`，这是科学信息静默丢失风险。
   任何未实现/未解析/缺失组件都必须渲染明确的 `Unsupported`/`Unresolved` 卡片，并保留对象引用和原因；
   不得让整块内容消失。

3. **通用表达能力优先于 assay-specific 组件。**
   下一轮优先补通用能力：Parameter Editor、Material/Recipe、Instrument Program、
   Container/Sample Map、Control/Gate、Readout/Analysis、Data Visualization、Lineage/State Transition。
   qPCR/ELISA/Western/IF 等由这些通用能力组合表达，不再新建专用卡片目录。

### 补充数据模型缺口

- `research_question` 目前没有独立字段，`objective` 不能同时承担两个语义。
- 因素/水平（`factors` / `levels`）没有结构化字段，只有组别 `condition` 文本。
- 复杂参数仍是 key/value/unit 与自由表格，缺枚举、有序程序、矩阵和对象引用。
- 附件（原始 qPCR、FCS、显微图像、仪器导出）没有通用引用/校验/来源模型。
- 组件注册不等于渲染；`implemented=false` 必须阻止 Agent/UI 把它当可编辑完成态。

### 建议的下一轮顺序

| 优先级 | 工作 | 验收 |
|---|---|---|
| P0 | 统一 Rail/Topbar 的六类页面入口，不再跳回旧 `/workspace` 才能完成核心操作 | 项目/Design/Protocol/Run/记录/工具均有新壳入口 |
| P0 | 把 Design 从四步 modal 迁移为 Design Map + 组件工作区 | 空白 UI 完成 CCK-8 与 RT-qPCR 设计，不依赖 CLI 预置 |
| P1 | 补 `research_question`、`factors/levels`、Study Quality、Recipe、Instrument Program | 结构化保存与重新打开，不靠自由文本 |
| P1 | unsupported/unresolved 显式渲染；DataBinding/附件 | 不静默丢组件；原始文件可追溯 |
| P2 | 九类 Protocol 设计/保存/重开/记录走查 | 逐项缺口报告与真实截图 |

### 设计页首轮内容分组

不再堆十张同权卡片，固定四个组：

```text
Design Map（上半，可收起）
  Groups → Assays → Time / Sample

Research     问题 · 假设 · 主要比较
Groups       因素/水平 · 对照 · 实验单位 · n
Assays       终点 · 方法 · 时间 · 样本 · Protocol
Quality      随机化 · 盲法 · 纳排 · 分析计划 · 证据
```

首轮必须做结构化 `factors/levels` 和独立 `research_question`；其余 Study Quality 字段可先保留空值，
但要有稳定入口，不能继续藏在 schema 里。
