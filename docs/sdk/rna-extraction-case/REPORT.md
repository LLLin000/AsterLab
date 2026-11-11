# RNA 提取 / RT-qPCR 多层级 Workflow 适配性测试

> **Post-fix update (Track A):** The Run correctness issues below were fixed after this
> pre-fix adaptivity test. The matrix records the original state; see
> `docs/sdk/run-correctness-track-a.md` for the fixed behavior and evidence.

- 日期：2026-11-11
- 基线：`feature/v2-run-workspace` @ `d649ed1`
- 范围：仅使用现有 SDK、CLI、AsterDoc、ProtocolBlock、Design、Run；未修改任何生产代码，未修改 Run UI v1 布局。
- 数据目录：`target/rna-case-data`（独立测试库，不污染现有项目）。
- CLI：`target/debug/labworkbench.exe`；GUI：`prototypes/mygo-run/target/asterlab-onevra.exe`。
- 本报告中的 RNA 浓度、A260/A280 全部是 **MOCK UI TEST VALUES**，不是真实实验数据。

## 1. 结论摘要

1. 独立的 10 步 RNA 提取实验可以完整创建、冻结、执行、保存测量并在重启后恢复。
2. 4 个虚拟样本可以作为事件作用域记录，但 Run UI 没有样本级状态；任务状态是任务级聚合。
3. 操作条件（体积、离心力、时间、保留/弃置）目前主要通过 **Check 的 label/instruction 文本**可见；右侧“操作参数”显示的是模块原始字段，不是操作参数。
4. `ProtocolBlock` 的递归 `block_use` 能真实展开为 13 个带路径前缀的子步骤，且精确版本冻结有效；但 Design Canvas 和 Run 都只显示平面结果，没有父步骤卡片、父子树或进度聚合。
5. Agent 可以通过 `protocol.block.preview` 读取展开后的树和路径，但没有 `inspect_tree` / `expand` / `insert_child` / `bind_subflow` 等语义命令；修改子步骤需要手工创建新 block version → 新 parent block version → 新 protocol version → 新 design revision。
6. 发现若干现有实现的关联/展示问题，按严重度列在第 6 节；本轮未修复。

## 2. 参考方法与范围

试剂盒：Invitrogen PureLink RNA Mini Kit，MAN0000406，2012 年 6 月修订版。

采用路径：

- ≤5 × 10^6 悬浮细胞（说明书第 15 页 / PDF 提取文本中的 cell protocol）。
- Binding、Washing、Elution（说明书第 19–20 页 / PDF 提取文本中的对应步骤）。

写入的固定条件：

- 收集细胞：2,000 × g，5 min，4°C；弃培养基，保留沉淀。
- 裂解：Lysis Buffer + 1% 2-mercaptoethanol；≤1 × 10^6 用 0.3 mL，1–5 × 10^6 用 0.6 mL。
- 匀浆：说明书允许 Homogenizer（12,000 × g，2 min）、18–21G 针头（5–10 次）、rotor-stator（45 s，然后 2,600 × g，5 min）三选一；测试中未推断，保留“待研究者确认”。
- 加乙醇：等体积 70% 乙醇（1:1）。
- 上柱：每次 ≤700 µL；12,000 × g，15 s；弃流出液，保留吸附柱；重复至全部样品过柱。
- Wash I：700 µL；12,000 × g，15 s；弃流出液和 Collection Tube，换新 Collection Tube。
- Wash II：500 µL；12,000 × g，15 s；弃流出液；重复一次。
- 干柱：12,000 × g，1–2 min；弃 Collection Tube，换 Recovery Tube。
- 洗脱：30–100 µL RNase-free water；室温 1 min；≥12,000 × g，2 min。
- 质检：记录 RNA 浓度与 A260/A280（测试用 mock 值）。

## 3. 测试对象与版本

| 对象 | ID / 版本 |
|---|---|
| 独立 RNA 设计 | `d2d393a8-9af4-452c-beba-c3687503ffee` |
| 独立 RNA AsterDoc | revision 2；source hash `1d349f50e027c8016a7d8c14a488965a0460b4754faacc85feaa0b90cb3ebd53` |
| 独立 RNA 冻结 design version | `f237d71e-ce82-4e5c-858a-ac5a05b87326`；design revision 3 |
| 独立 RNA Run | `357c2f94-d5bf-4002-951e-f84c73e2d0b7`；run revision 19（19 个事件） |
| RNA Protocol | `a5d8318e-6f1c-4dcd-a930-73a6f38d872a` |
| RNA ProtocolBlock version | `c23df8e1-3f2a-4449-8ac9-a4bb7256a113` |
| 父 RT-qPCR 设计 | `b5a04dd5-9ee2-41a6-9ba6-0989d7583a94` |
| 父 RT-qPCR AsterDoc | revision 1；source hash `400722b559b47d84d8ba1f38bbaaa68227e2652e3f4dd3ee249e95b778947322` |
| 父 RT-qPCR 冻结 design version | `78c419e4-4754-4122-84fd-e8292c12a023`；design revision 2 |
| 父 RT-qPCR Run | `07578b9d-3ce6-4bfc-8e20-8897aad1ea83`；0 个事件 |
| 父 Protocol | `2066e8ba-17c9-48be-976a-64e4ddde64a6` |
| 父 ProtocolBlock version | `b41381b7-cd4c-44ca-9efa-97d43f9d66ea` |
| 新 RNA ProtocolBlock version（隔离测试） | `e7d9f1fe-9c0c-496a-bb02-bac406cfa73e` |

案例文件：

- `docs/sdk/rna-extraction-case/purelink-rna.asterml`
- `docs/sdk/rna-extraction-case/rtqpcr-parent.asterml`

## 4. 建立的实验

### 4.1 独立 RNA 提取（10 步）

- 1 个 Experiment、1 个 Group、1 个 Assay、4 个 Sample（RNA-S01–S04）。
- 5 个 Material、2 个 InstrumentProfile、4 个 Spin Cartridge 容器、4 个 Recovery Tube 容器。
- 10 个 `step@2`，每个步骤 2 个 `step.tasks` 和 1 个 `step.checks`。
- 1 个 `workflow@1`，10 个节点、9 条顺序边。
- 1 个 ProtocolBlock（10 个 text step，每个 step 1 个 blocking check）。
- 每个步骤都通过 `execution_event.create` 记录：实际参数、材料使用、样本作用域、检查结果、lineage；步骤 10 记录 8 个 mock measurement；RNA-S03 在 Wash II 第一次有一次 deviation。
- 10 个 Run task 全部 completed。

### 4.2 父 RT-qPCR（4 个一级步骤）

- 1 个 Experiment、1 个 Group、1 个 Assay、4 个 Sample（RNA-S01–S04）。
- 4 个一级 `step@2`：01 RNA 提取、02 RNA 质控、03 逆转录、04 qPCR 扩增及检测。
- 01 RNA 提取通过 `ref-rna-block` 引用 RNA ProtocolBlock version `c23df8e1-...`，不是复制 10 段文字。
- 1 个父 ProtocolBlock，包含：
  - 1 个 `block_use`，引用 RNA ProtocolBlock version `c23df8e1-...`；
  - 3 个 text step：RNA 质控、逆转录、qPCR。
- 父 ProtocolBlock 的 `protocol.block.preview` 展开为 **13 个步骤**：
  - `root/rna-extraction/01-collect` … `root/rna-extraction/10-qc`；
  - `root/rna-qc`；
  - `root/reverse-transcription`；
  - `root/qpcr`。
- 父 Run 的 `run.workspace` 返回 **13 个 flat task**，全部为 `not_started`；没有 4 个父卡片、没有 10 个子步骤聚合、没有 `3/8` 状态。

## 5. 真实 GUI 证据

所有截图来自真实 MyGo 窗口，不是重绘。

| 证据 | 内容 |
|---|---|
| `evidence/design-flow-v2.png` | 独立 RNA Design：10 个步骤卡片，标题为 01–10；画布未显示可见连线。 |
| `evidence/design-step01.png` | 独立 RNA 步骤属性：标题、检测、Protocol、path、步骤资源、任务、检查项。 |
| `evidence/run-01.png` | 默认 1360×880 Run：01–08 可见；右侧 01 收集细胞，check 显示 2,000 × g / 5 min / 4°C；“操作参数”是模块原始字段；8 个测量出现在 01。 |
| `evidence/run-05-wash1.png` | Wash I：check“弃流出液并更换 Collection Tube”，instruction 700 µL / 12,000 × g / 15 s；步骤资源仍为通用 material/container/sample。 |
| `evidence/run-06-wash2a.png` | Wash II 第一次：独立任务 `root/06-wash2a`，check“第一次 Wash II：弃流出液”。 |
| `evidence/run-07-wash2b.png` | Wash II 第二次：独立任务 `root/07-wash2b`，check“第二次 Wash II：弃流出液”。 |
| `evidence/run-09-elute.png` | RNA 洗脱：check“保留洗脱液（RNA 成品）”，instruction 30–100 µL / 1 min / 12,000 × g / 2 min。 |
| `evidence/run-10-qc.png` | RNA 质检：8 个 mock 测量在表格中可见；无 RNA 专用表单。 |
| `evidence/run-restart.png` | 退出重启后：10/10 completed，19 个事件对应的 run revision，测量恢复；check 仍为未勾选。 |
| `evidence/run-history-empty.png` | QC 步骤的“操作历史 >”入口；当前 Run 历史层没有事件列表。 |
| `evidence/notebook-timeline2.png` | Notebook“时间线”：显示冻结 AsterDoc 模块列表，不是执行事件时间线。 |
| `evidence/rtqpcr-design.png` | 父 RT-qPCR Design：4 个一级卡片，01 RNA 提取没有子步骤/复合标记。 |
| `evidence/rtqpcr-design-step01.png` | 父步骤属性：`ref-rna-block` 引用、两个 Protocol chip（父 RT-qPCR / RNA Mini Kit）、4 个 sample、1 个 task、1 个 check；没有 10 个子步骤列表。 |
| `evidence/rtqpcr-run.png` | 父 Run：13 个 flat task，路径为 `root/rna-extraction/...`、`root/rna-qc`、`root/reverse-transcription`、`root/qpcr`；没有父步骤卡片或进度聚合。 |

## 6. 能力矩阵

| 需求 | 结论 | 当前实现方式 | 证据 | 问题与严重度 |
|---|---|---|---|---|
| 多步骤流程（10 步） | 完全 / 默认窗口导航部分 | 10 个 `step@2` + `workflow@1` + 10 个 ProtocolBlock text step；10 个 Run task 全部可执行 | `design-flow-v2.png`、`run-01.png`、`run-restart.png` | 默认 1360×880 左侧只显示 01–08，09/10 不可见且 rail 不滚动；P1 |
| 子任务与重复操作 | 部分 | Design 有 2 个 `step.tasks`/步；Run 可执行单元是 ProtocolBlock 的 step，不是 `step.tasks`；Wash II 被拆成 06/07 两个 task；上柱重复通过同一 task 的多个 event 记录 | `design-step01.png`、`run-06-wash2a.png`、`run-07-wash2b.png` | 没有通用 repeat 引擎；子任务不在 Run 中逐项执行；P1 |
| 参数展示 | 部分 / 仅文本 | Check 的 label/instruction 显示体积、离心力、时间、保留/弃置；右侧“操作参数”显示 `Amount/Group_ref/Sample_id/Capabilities/...` 原始模块字段 | `run-05-wash1.png`、`run-09-elute.png` | 操作参数不是结构化参数，且被原始模块字段淹没；P1 |
| 关键动作确认 | 部分 | 每个 ProtocolBlock step 有 blocking check；Run 显示 checkbox；event 记录了 `checks: passed` | `run-05-wash1.png`、`run-10-qc.png` | Run task 已 completed，但 checkbox 仍 pending；event check ack 没有回填 UI；P1 |
| 样本级执行状态 | 不支持 | event 有 `sample_ids: RNA-S01..S04`；Run task 状态是聚合的；没有 per-sample 状态矩阵 | `run-01.png`、`run-restart.png` | 无法回答“RNA-S03 现在做到哪一步”；P1 |
| 容器流转 | 仅记录 | `lineage` event 保存 `sample -> cartridge -> recovery`、`cartridge -> waste`；`run.workspace` 投影 lineage；UI 不展示 | `run-05-wash1.png`、`run-09-elute.png` | 没有容器流转视图；容器只显示为 `容器 other` chip；P1 |
| 保留/丢弃对象 | 仅文本 | Check label/instruction 写“保留细胞沉淀；弃培养基”“弃流出液；保留吸附柱”“保留洗脱液”；lineage 有 `to_kind: waste` | `run-05-wash1.png`、`run-09-elute.png` | 无法结构化核对“保留/丢弃”对象；P1 |
| 执行偏差记录 | 仅文本 / 部分 | `event_type: deviation` + `deviation` 字段；`run.workspace` 只保留 content，丢弃 deviation 字段；Run 历史层只渲染 `Actual.Deviations`（未填充） | `run-history-empty.png` | 偏差不在 UI 历史中可见；P1 |
| 测量结果关联 | 部分 | `measurements` 记录 sample_id、endpoint、value、unit；Run 表格显示 8 条 mock 值 | `run-10-qc.png`、`run-restart.png` | 8 条测量出现在**每一个** task（01 也有），没有 task 关联；P1 |
| 冻结与重启恢复 | 部分 | Core 保存 Run、事件、测量；重启后 10/10、测量恢复 | `run-restart.png` | `saveSession` 只在 Run 页保存 `run_id`，切到其他页后重启会丢失 Run 选择；check 状态不恢复；P1 |
| 多层级 containment | 部分 / 组合支持 | ProtocolBlock `block_use` 递归展开为 13 步，路径带 `root/rna-extraction/` 前缀；版本精确冻结 | `rtqpcr-run.png`、`rtqpcr-design-step01.png` | Design/Run 都没有父子树、父卡片、聚合进度；4 个父步骤与 13 个 task 没有 DesignStepID 关联；P1 |
| Agent 树操作 | 仅读 / 组合支持 | `protocol.block.preview` 可读 13 步和路径；`asterdoc.inspect` 可读 reference/step；`protocol.block.revise` 可创建新版本 | `rtqpcr-design-step01.png`、preview 输出 | 没有 `inspect_tree`/`expand`/`insert_child`/`bind_subflow`/`validate_composition`；修改需手工版本链；P1 |
| 版本冻结 | 完全 | 新 RNA block version `e7d9f1fe` 增加 `c05-tube-check`；父 block preview 仍只显示 `c05-wash1`；旧 Run 不变 | preview 对比 | 版本隔离正确；P2（缺可视化） |
| 依赖关系 | 部分 | Workflow 编译出 9/3 条 edges；Run 顺序为 flat task 顺序 | `design-flow-v2.png`、`rtqpcr-run.png` | Canvas 连线不可见；Run task 模型没有统一的 `contains`/`precedes` 关系，containment 只体现为 path 前缀；P2 |

## 7. 缺口明细

### 7.1 数据模型 / Core

- **P1 测量没有 task 关联。** `execution_event.create` 的 `measurements` 只有 `sample_id`，没有 task/path；`BuildViewModel` 把所有 `snap.Measurements` 追加到每个 task。结果是 01 收集细胞也显示 RNA 浓度和 A260/A280。
- **P1 偏差字段被读模型丢弃。** `run.workspace` 的 event projection 不含 `deviation`，只有 `content`；UI 无法区分“偏差事件”和普通操作事件。
- **P1 材料使用没有进入 Run 读模型。** `material_use` 事件写入成功，但 `run.workspace` 的 event 不含 `materials`；UI 看不到试剂消耗。
- **P1 样本级状态缺失。** 事件有 `sample_ids`，但任务状态是 task-level；没有 per-sample 状态/进度。
- **P1 父/子步骤没有正式关联。** 父设计的 4 个 `step.path`（`rtqpcr/...`）与 ProtocolBlock 展开路径（`root/rna-extraction/...`）不匹配；Run task 的 DesignStepID 为空，父步骤与 13 个子任务没有稳定 ID 关系。
- **P2 Run revision 与 frozen design revision 混用标签。** Run header 显示“冻结方案 rev 19”，实际是 Run revision（事件数）；AsterDoc revision 是 2，冻结 design revision 是 3。

### 7.2 SDK / CLI

- **P1 没有树查询/修改命令。** `protocol.block.preview` 能读展开树，但 Agent 需要先知道 block version，并解析 `path` 字符串；没有 `inspect_tree`、`expand`、`insert_child`、`bind_subflow`、`validate_composition`。
- **P1 修改子步骤需要版本链。** 要更新 RNA 提取的子步骤，必须新建 RNA block version → 新建 parent block version（替换 `block_version_id`）→ 新建 parent protocol version → 新建 design revision；没有 instance-level 编辑。
- **P2 `sdk.modules` 不列 ProtocolBlock。** Agent 需要知道去 `protocol.block.*` 查找；模块目录与 ProtocolBlock 目录是两个发现面。

### 7.3 Renderer / Workspace / UI

- **P1 默认窗口 rail 不可滚动。** 10 步时 09/10 不可见；13 步时更严重。最大化 2576×1408 才能看到全部。`flowRail` 没有 `ui.Scroll`。
- **P1 右侧“操作参数”是模块原始字段。** 显示 `Amount/Group_ref/Sample_id/Capabilities/Constraints/Class/Format/Wells` 等，不是体积/离心力/时间；真正的条件只在 check instruction 中。
- **P1 任务 completed 与 check pending 不一致。** event 的 checks passed 没有回填 `CheckViewModel.Status`；UI 仍显示 0/1。
- **P1 历史层没有事件列表。** `historyLayer` 只渲染 `Actual.Deviations` 和一句只读说明；20 个执行事件、材料使用、lineage、测量事件都不显示。
- **P1 Notebook“时间线”显示冻结文档模块，不是执行事件。** 无法作为 Run 审计时间线。
- **P2 Design Canvas 连线不可见。** 编译结果有 9 条 edges，但截图中卡片之间没有可见连线；可能是渲染颜色/层级问题。
- **P2 Run rail 标题是 path。** `root/rna-extraction/05-wash1` 等长路径取代了人类步骤名“05 Wash I”。
- **P2 容器/样本 chips 没有可读名称。** 显示“容器 other”“样本 sample”，而不是“Spin Cartridge S01”“RNA-S01”。

### 7.4 持久化 / 会话

- **P1 切换页面会清空 session 的 run_id。** `saveSession` 只在 `Page == Run` 时写 `RunID`；切到 Design/Notebook 后退出，重启不会恢复 Run。测试中切换到 Notebook 后 session 变为 `run_id: ""`。
- **P1 重启后 check 状态不恢复。** 任务 completed、测量恢复，但 checkbox 仍 pending。

## 8. 三类结论

### 真实实现成功

- 10 步独立 RNA 提取的创建、冻结、执行、测量保存、重启恢复。
- ProtocolBlock 递归 `block_use` 和精确版本引用；13 步展开路径。
- 4 个样本作为 event `sample_ids` 的作用域记录。
- 两次 Wash II 作为两个独立 task 执行和确认。
- 材料使用、lineage、deviation、measurement 事件在 Core 中真实写入（只是 UI 不展示全部）。
- 新 RNA block version 不影响旧 parent block / 旧 Run（版本隔离）。

### 只能通过文本或现有功能勉强表达

- 操作参数（体积、离心力、时间）→ check instruction 文本。
- 保留/丢弃对象 → check label 文本 + lineage `waste`。
- 样本级状态 → event `sample_ids` + 测量表；没有状态矩阵。
- 父/子流程 → ProtocolBlock 路径前缀 + reference 模块；没有 UI 树。
- 偏差 → deviation event 的 content 文本；`deviation` 字段不进 UI。
- 子任务 → Design `step.tasks` 文本；Run 不逐项执行。

### 完全无法实现（现有功能内）

- 在 Design Canvas 上展开/进入复合步骤的子画布。
- 在 Run 中显示父步骤卡片、子步骤树、`3/8` 聚合进度。
- 通过单一 SDK 命令定位/插入/移动子步骤，并保持 instance identity。
- 在 Run UI 中查看执行事件时间线、材料消耗、容器流转、样本级状态。
- 在 Run UI 中把测量精确关联到 task/sample 组合。

## 9. 后续改进建议（按影响排序，最多 5 项）

1. **先定 Workflow 层级契约，不先做嵌套 Canvas。** 增加 `contains` 与 `precedes` 的正式区分；复合步骤引用精确 block version；定义 Definition / Use(Instance) / Occurrence 三层身份；冻结时递归固定子块版本；提供 `inspect_tree` / `expand` / `validate_composition`。这是父/子 Run 和 Agent 编辑的前提。
2. **修 Run 的执行可读性。** 右侧按“做什么 → 操作参数 → 确认 → 记录 → 下一步”渲染结构化参数，隐藏原始模块字段；Run rail 改为可滚动，并用人类步骤名而不是 path；把 task completed 与 check 状态统一。
3. **把事件历史变成真正的审计面。** 在 Run 历史/Notebook 时间线中显示 operation、material_use、deviation、measurement、lineage；保留 `deviation` 字段；显示材料消耗和容器流转；测量按 task/sample 关联，不再出现在所有 task。
4. **补样本级状态与容器身份。** 在 Run 中提供 per-sample 状态/进度；容器使用可读 ID/名称，而不是 `容器 other`；保留/丢弃对象用结构化 `to_kind/to_id` 呈现。
5. **修会话与版本标签。** `run_id` 在任意页面切换后仍保存；重启恢复 Run；Run header 明确区分 Run revision 与 frozen design revision；Design Canvas 修复 edge 可见性。

## 10. 复现与限制

复现命令（均在仓库根目录，使用独立数据目录）：

```bash
export LABWORKBENCH_DATA_DIR="$PWD/target/rna-case-data"
CLI=./target/debug/labworkbench.exe
$CLI tools --json
$CLI sdk.modules --input <(printf '{}')
$CLI asterdoc.validate --input <validate.json>
$CLI asterdoc.assess --input <assess.json>
$CLI asterdoc.compile --input <assess.json>
$CLI asterdoc.create_run --input <run.json>
$CLI run.workspace --input <workspace.json>
$CLI protocol.block.preview --input <block-version.json>
$CLI execution_event.create --input <event.json>
```

关键命令与输入文件保存在 `target/rna-case-data/`（测试库，不是交付源码）：

- `save2.json` / `assess2.json` / `run2.json`：独立 RNA revision 2 / Run 2。
- `rtqpcr-save.json` / `rtqpcr-assess.json` / `rtqpcr-run.json`：父 RT-qPCR。
- `preview-new.json` / `preview-parent.json`：版本隔离对比。
- `events2/`：19 个执行事件。

限制：

- 未执行真实实验；RNA 浓度、A260/A280 是 mock UI 测试值。
- 未新增任何 SDK 模块、Schema、Renderer、组件或页面；未修改冻结 Run。
- GUI 截图来自真实窗口；默认窗口 1360×880，最大化窗口 2576×1408。默认窗口下 09/10 不可见是本次发现的真实导航缺口，不是截图裁剪。
- 设计画布的节点坐标来自测试 session（单列布局），用于让 10 个步骤可读；坐标属于 Workspace session，不进入实验语义。
