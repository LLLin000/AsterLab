# 科学字段归属与写入权威（SCI-01）

- 状态：审计结论，供 DOC-05 引用；不改代码、不改契约。
- 基线：`feature/sci-01-audit-zcf`（`6ca46c5`），审计前后工作区仅 `docs/sdk/science-field-ownership.md` 与 `docs/release/evidence/SCI-01/` 变化。
- 权威边界：对象与术语以 `CONTEXT.md` 为准；已实现 CLI 字段以 `AGENT_API.md` 为准；Core 类型以 `crates/asterlab-core/src/**` 为准；模块清单以 `prototypes/mygo-run/internal/asterdoc/spec/modules.json` 为准。
- 目的：对 Design / Protocol / Run / Notebook 四个写入面逐字段确定**唯一权威写入点**，并给出派生链、冻结边界与重复项处置。

## 0. 判定规则

1. **一个字段只有一个权威写入点。** 权威写入点是能产生该字段持久记录的 Core 命令；同一字段的其他出现只能是引用、只读投影或草案副本。
2. **计划 ≠ 冻结 ≠ 事实。** Design 写计划；`run.create` 把计划冻结成快照；`execution_event.create` 只追加事实。冻结后不再读当前 Design。
3. **派生值不落第二张事实表。** `design.map`、`ProtocolPreview`、`RunWorkspace`、`ReadingFact`、timeline 都是可重算投影，不得回写。
4. **AsterDoc 模块实例是文档草案，不是科学事实。** 模块可以声明 `planned` / `derived` / `view` 端口，但 `record_boundary` 固定为 `run`，事实只能经 Core 命令写入。
5. **Notebook 只写散文。** 它可以引用事实 ID，但不能创建测量、完成步骤或附加执行文件；其内容即使与事件同表存放，也不得被当作事实读取。

四面写入边界：

| 面 | 唯一可写对象 | 可调用命令 | 明确禁止 |
|---|---|---|---|
| Design | `design` + `design_version` | `design.create` / `design.revise` | 写 Run 事实、写冻结版本、直接写读数 |
| Protocol | `protocol` + `protocol_version`、`protocol_block` + `protocol_block_version` | `protocol.create/revise`、`protocol.block.create/revise` | 写组别/n/时间点事实、修改已冻结版本 |
| Run | `run`、`execution_event`、`plate`/`well` | `run.create`、`run.set_status`、`run.revise`、`execution_event.create`、`plate.create`、`well.set` | 修改冻结 Design/Protocol 快照、覆盖旧事件、回写派生事实 |
| Notebook | 仅追加散文记录（`execution_event.stage=notebook`） | MyGo `RunService.SaveNotebook`（`prototypes/mygo-run/modules/runs/rust/src/lib.rs:107-120` → `core.save_readout`） | 创建测量/检查/完成/材料事实、附执行文件、维护第二份时间线 |

## 1. 字段归属表

权威来源列写持久记录类型；读命令列写公开读模型。冻结列说明该字段是否随 `run.create` 进入 Run 快照。

| 对象 | 字段 | 权威来源（表/文件） | 唯一写入命令 | 读取面/命令 | 派生与校验 | 冻结 |
|---|---|---|---|---|---|---|
| Experiment | `title` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.get`、`design.map` | `DesignMapContent.title` 直接复制 | 是（DesignVersion） |
| Experiment | `objective` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 不推断科学目标 | 是 |
| Experiment | `research_question` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 缺字段回退空，不继承 `objective` | 是 |
| Experiment | `factors[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 名称唯一、单因素≤50 水平；不从组名推断 | 是 |
| Experiment | `experimental_unit` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 缺失时进入 `unresolved`，不默认“孔” | 是 |
| Experiment | `primary_endpoint` / `endpoint_method` / `endpoint_unit` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 是概要字段；不自动关联到某个 Assay | 是 |
| Experiment | `sample_size_rationale` / `randomization` / `blinding` / `analysis_plan` / 纳排标准 | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 只存研究者原文，不生成依据 | 是 |
| Experiment | `planned_run_count` / `planned_run_relationship` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map` | 轮次数不得换算成独立 n | 是 |
| Group | `group_specs[].id` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].id` | Assay 对照通过该 ID 引用，不按名称猜 | 是 |
| Group | `group_specs[].name` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].name` | 显示名；名称不承担引用身份 | 是 |
| Group | `group_specs[].role` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].role` | 枚举校验；未知不猜 | 是 |
| Group | `group_specs[].condition` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].condition` | 自由文本；不是参数绑定的权威值 | 是 |
| Group | `group_specs[].independent_n` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].independent_n` | 缺失即未知；不由孔数/读数推算 | 是 |
| Group | `group_specs[].technical_repeats` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.groups[].technical_repeats` | 单独字段，不计入 n | 是 |
| Assay | `assays[].id` / `assays[].title` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | ID 稳定；Run scope 引用该 ID | 是 |
| Assay | `assays[].endpoint` / `method` / `unit` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | 测量指标不是原始读数 | 是 |
| Assay | `assays[].group_ids[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[].group_ids` | 必须引用本设计组 ID | 是 |
| Assay | `assays[].control_group_ids[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | 对照关系用 ID；`controls[]` 文本只是说明 | 是 |
| Assay | `assays[].protocol_version_id` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `protocol.block.preview`、`run.workspace.scopes` | `run.create` 解析为精确 `protocol_manifest` 条目 | 是 |
| Assay | `assays[].container_type` / `container_format` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | 计划容器类型；不生成实际坐标 | 是 |
| Timepoint | `assays[].timepoints[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[].timepoints` | 每项 Assay 自有；不从其他 Assay 继承 | 是 |
| Timepoint | `assays[].timepoint_timing[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[].timepoint_timing` | 锚点+偏移+窗口；标签顺序须与 `timepoints[]` 一致 | 是 |
| Timepoint | `assays[].time_anchor_event` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | 有偏移/窗口时必须提供 | 是 |
| Timepoint | `assays[].timepoint_relation` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.assays[]` | 五个明确枚举；旧含糊值迁为 `unknown` | 是 |
| Timepoint | `step.time_scope` | `protocol_block_version` 记录（`ProtocolStep::Text.time_scope`） | `protocol.block.create` / `protocol.block.revise` | `protocol.block.preview`、`run.workspace.steps` | 仅 `per_timepoint` 展开 Assay 时间点；缺失不复制到全部时间点 | 是（随 ProtocolVersion） |
| Timepoint | `mygo.stepTasks[].timepoint` | `design_version` 快照内的 `mygo` 附注 | `design.create` / `design.revise`（随设计快照） | `run_workspace::build` 的显式时间绑定 | 仅缺 `time_scope` 时按 path 解析；不得覆盖 Design `timepoints[]` | 是 |
| Timepoint | `plate.timepoint` | `records.data_json(kind=plate)` | `plate.create` | `run.workspace.plates[].timepoint` | 必须属于冻结 Assay 时间点；独立取样关系下必填 | 否（创建 Run 后作为实际板图读取） |
| Step | `use_id` / `title` / `instruction` | `protocol_block_version.data_json.steps[]` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.get`、`protocol.block.preview` | 版本不可变；旧版本不改 | 是 |
| Step | `parameter_refs[]` | `protocol_block_version.data_json.steps[]` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.preview` | 只有列入的槽位成为该步骤显式输入 | 是 |
| Step | `visualization` | `protocol_block_version.data_json.steps[]` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.preview` | 只决定图解布局，不新增事实 | 是 |
| Step | `checks[]`（定义） | `protocol_block_version.data_json.steps[]` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.preview`、`run.workspace.steps` | `check_id` 定义来源；执行结果不写回定义 | 是 |
| Step | `OperationStep.path` / `order` | 无独立存储；由 `protocol.block.preview` 展开 | 只读 | `protocol.block.preview`、`run.workspace.steps` | 由展开顺序派生；不由标题生成 | 是（展开结果随 manifest） |
| Block | `block_id` / `version` | `records`（`kind=protocol_block` / `protocol_block_version`） | `protocol.block.create` / `protocol.block.revise` | `protocol.block.list` / `protocol.block.get` | 修订追加新版本；`expected_version` 防并发覆盖 | 是（精确版本引用） |
| Block | `parameter_slots[]` | `protocol_block_version.data_json` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.get`、`protocol.block.preview` | key/label/value_type/unit/required；单位不做隐式换算 | 是 |
| Block | `steps[]`（`text` / `block_use`） | `protocol_block_version.data_json` | `protocol.block.create` / `protocol.block.revise` | `protocol.block.get`、`protocol.block.preview` | 组合块固定子块精确版本，禁环 | 是 |
| Block | `StepBinding.value` / `source_slot` / `unit` | `protocol_block_version.data_json`（BlockUse）与 Protocol 根绑定 | `protocol.block.create/revise`、`protocol.create/revise` | `protocol.block.preview` | `value` 与 `source_slot` 二选一；父子映射必须显式 | 是 |
| Block | `module_contract` / `module_origin` / `catalog_review` | `protocol_block_version.data_json` 可选 sidecar | `protocol.block.create/revise`（编译期写入） | `protocol.block.get` | 同一科学知识不得在 `StepMeta` 再存一份权威值 | 是 |
| Protocol | `root_block_version_id` / `root_bindings[]` | `records.data_json(kind=protocol_version)` | `protocol.create` / `protocol.revise` | `protocol.block.preview` | 只改绑定不复制块；新版本追加 | 是 |
| Protocol | `resolved_parameters[]` / `unresolved_parameters[]` | 无独立存储 | 只读 | `protocol.block.preview`、`run.workspace.scopes` | 展开绑定后派生；`ready` 只表示必填绑定完成 | 是（manifest 结果） |
| Run | `run.id` / `run_index` | `records`（`kind=run`） | `run.create` | `run.list`、`run.workspace` | 索引按设计递增 | 否（Run 身份本身） |
| Run | `status` | `records.data_json(kind=run)` | `run.create` / `run.set_status` | `run.list`、`run.workspace.status` | 参数未齐时拒绝 `in_progress` | 否 |
| Run | `title` / `scheduled_date` | `records.data_json(kind=run)` | `run.create` / `run.revise` | `run.list` | `run.revise` 只接受这两个字段 | 否 |
| Run | `design_version_id` / `design_revision` | `records.data_json(kind=run)` | `run.create` | `run.workspace.frozen_design` | 必须指向不可变 DesignVersion；缺失即错误，不回退 | 是（冻结指针） |
| Run | `protocol_manifest[]` | `records.data_json(kind=run)` | `run.create` | `run.workspace.scopes` / `.steps` | 冻结每个 Assay/PlannedOutput 的 Protocol 精确版本、根块、参数 | 是 |
| Run | `material_manifest[]` | `records.data_json(kind=run)` | `run.create` | `run.workspace.materials` | 每个 `PlannedOutput` 一条；消费者只列引用 | 是 |
| Run | `container_plan` / `labware_profiles` | `records.data_json(kind=run)` | `run.create` | `run.workspace`（计划容器上下文） | 冻结计划和 Profile 全文，避免 Registry 漂移 | 是 |
| Run | `repeat_of_run_id` / `repeat_relationship` | `records.data_json(kind=run)` | `run.create` | `run.list`、`run.workspace` | 只表达再次执行；不证明样本独立 | 是 |
| Run | `occurrence_id` + 上下文 | `execution_event` 记录内首次写定的字段 | `execution_event.create` | `run.workspace.events[].occurrence_id` | 首次事件分配；旧事件不自动补；与 `idempotency_key` 分离 | 否（事实追加） |
| Sample | `sample_allocations[].group_id` / `assay` / `planned_output_id` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.sample_links[]` | 计划分配关系，不是实际样本记录 | 是 |
| Sample | `sample_allocations[].physical_slots_per_group` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.sample_links[]` | 计划位置数；≠实体样本数、≠n | 是 |
| Sample | `sample_allocations[].physical_slots_scope` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.sample_links[]` | `per_timepoint` / `whole_assay_unique_positions` / `unknown` | 是 |
| Sample | `sample_allocations[].timepoint_relation` / `source_relation` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.sample_links[]` | 时间关系与跨检测来源分开；禁写旧 `relation` | 是 |
| Sample | `planned_outputs[].id` / `title` / `protocol_version_id` / `root_bindings[]` | `records.data_json(kind=design)` | `design.create` / `design.revise` | `design.map.planned_outputs[]`、`run.workspace.materials` | 同 ID 才表示计划共用一份制备 | 是 |
| Sample | `well.group_label` / `well.sample_id` / `well.note` | `records.data_json(kind=plate)`（`well` 映射） | `plate.create`、`well.set` | `run.workspace.plates[].wells[]` | 实际孔位映射；未映射不生成坐标 | 否（Run 读取实际板图） |
| Sample | `event.sample_ids[]` / `materials[]` / `lineage[]` | `execution_event` 记录 | `execution_event.create` | `run.workspace.events[]` | 实际使用/分样/顺序关系；计划共用不证明材料共用 | 否（追加事实） |
| Readout | `measurements[]` | `execution_event` 记录 | `execution_event.create` | `run.workspace.events[]`、`reading_facts` | 数值测量；`null` 不被写成 0 | 否 |
| Readout | `result_cells[]` | `execution_event` 记录 | `execution_event.create`（批次写入） | `reading_facts` | 结构化网格含 `missing`；存在时优先于旧 `readouts` | 否 |
| Readout | `readouts[]`（`raw`） | `execution_event` 记录（旧形状） | `execution_event.create` | `reading_facts` 兼容读取 | 仅当 `result_cells` 与 `measurements` 都缺失时读取 | 否 |
| Readout | `readouts[]`（`derived` + `method` + `derived_from[]`） | `execution_event` 记录 | `execution_event.create` | `run.workspace.events[]` | 派生结果保留在事件；**不进入**数值事实表 | 否 |
| Readout | `checks[]`（执行结果） | `execution_event` 记录 | `execution_event.create` | `run.workspace.events[]`、timeline | `check_id` 必须存在于冻结步骤；`skipped`≠通过 | 否 |
| Readout | `controls[]` / `actual_parameters[]` | `execution_event` 记录 | `execution_event.create` | `run.workspace.events[]` | 实际参数与计划参数分开；不覆盖 Protocol 绑定 | 否 |
| Readout | `ReadingFact.value` / `unit` / `status` | 无独立存储；`reading_facts` 从事件派生 | 只读（禁止回写） | `query_readings`、`project_table`、`project_plate` | key = step+assay+timepoint+endpoint+target；`value=null` 表缺失 | 否（可重算） |
| Readout | `ReadingFact.revision` / `supersedes_event_id` | 无独立存储；按事件时间序派生 | 只读 | `query_readings(include_history)` | 同 key 递增；旧事件保留为 history | 否（可重算） |
| Notebook | `note.content` / `stage` | `execution_event` 记录（`event_type=observation`、`stage=notebook`） | `RunService.SaveNotebook`（`lib.rs:107-120`） | Notebook 页读同一 Run 事件并按 `stage` 过滤（`notebook_page.go:316`） | 散文可引用事实 ID，但不是事实副本 | 否（追加） |

## 2. 数据流表

| # | 流 | 权威起点 | 派生/冻结步骤 | 消费读模型 | 禁止的回流 |
|---|---|---|---|---|---|
| 1 | 计划 → 冻结设计 | `design` 当前记录 | `design.revise` 生成新 `design_version` → `run.create` 固定 `design_version_id` | `CurrentDesignMap`、`FrozenDesignView` | 冻结后读当前 Design 覆盖 |
| 2 | 组别/n | `design.group_specs` | 原样进入 DesignVersion 快照 | `design.map.groups`、Run 冻结设计 | 用孔位/读数反推 `independent_n` |
| 3 | Assay/时间点 | `design.assays[]` | 每 Assay 独立快照；`timepoints` 供 Task 展开 | `design.map.assays`、`PlannedTask.timepoint` | 跨 Assay 继承时间点或对照组 |
| 4 | 步骤定义 | `protocol_block_version` | `ProtocolVersion` 根块/绑定 → `run.create` 展开并写 `protocol_manifest.steps` | `ProtocolPreview`、`RunWorkspace.steps` | 从步骤列表推断已执行 |
| 5 | 参数 | `ParameterSlot` + `StepBinding`/根绑定 | 展开解析为 `ResolvedParameter`；未解析保留 `UnresolvedParameter` | `ProtocolPreview`、`RunScope.resolved_parameters` | 隐式继承父值或用默认值填坑 |
| 6 | 计划任务 | 冻结 `steps` + `time_scope` + Assay `timepoints` | 仅 `per_timepoint` 或显式 `stepTasks` 展开；否则单任务 | `RunWorkspace.tasks` | 时间点 × 样本 × 孔位笛卡尔积 |
| 7 | 材料计划 | `design.planned_outputs[]` + `sample_allocations[]` | `run.create` 生成一条 `material_manifest` | `RunWorkspace.materials` | 相同 Protocol 版本被当成材料共用 |
| 8 | 实际执行 | `execution_event.create` | 追加事件；首次写 `occurrence_id` 上下文 | `run.workspace.events`、timeline | 覆盖/删除旧事件，或按“最近同名步骤”归属 |
| 9 | 任务状态 | 事件 `step_status` | 由事件时间序投影 | `PlannedTask.status`、Run 任务面板 | 把普通测量/观察当作步骤完成 |
| 10 | 读数事实 | `measurement` 事件 | `reading_facts` 按 key 归一化、递增 revision | `query_readings` → `ReadingTable` / `ReadingPlate` | 表格与孔板各自再查事件或各存副本 |
| 11 | 结构读数 | 批次 `result_cells` | 生成一个 measurement 事件；`measurements` 为派生镜像 | `reading_facts`、结果网格 | 把派生读数写成 `raw`，或把 `missing` 写成 0 |
| 12 | 派生结果 | `readouts[result_kind=derived]` + `method` + `derived_from[]` | 保留在事件正文 | Run 事件、Notebook 引用 | 进入数值事实表或被显示层重算后落库 |
| 13 | 实际材料/样本 | `material_use`/`lineage` 事件、`plate/well` 映射 | 由事件与板图分别投影 | `RunWorkspace.materials`、`plates` | 用计划分配冒充实际样本 ID |
| 14 | Notebook 散文 | `RunService.SaveNotebook` → `execution_event(event_type=observation, stage=notebook)` | 追加记录，不改旧修订 | Notebook 页按 stage 过滤（`notebook_page.go:316`） | 经 Notebook 创建测量/完成步骤；把 `note.create` 的 `kind=note` 记录当 Notebook 正文 |
| 15 | 模块文档值 | AsterDoc 文档（`modules.json` 声明的端口） | 编辑草案；`planned` 端口不落事实，`derived` 端口需 calculator | 设计/文档渲染器 | 把模块输出当 Run 事实或第二权威计划 |

## 3. 冲突与重复清单

| 对象 | 现状 | 处置 | 理由 | 目标卡 |
|---|---|---|---|---|
| Group 字段 | `design.group_specs[]` 与 AsterDoc `group@1` 同时声明 `name/role/condition/independent_n/technical_repeats` | 保留 `design` 记录为权威；`group@1` 实例只作草案/渲染，不得被 Run 冻结读取 | 两个写入点会产生冲突的 n/条件；冻结链只读 DesignVersion | DOC-05（文档规则）；后续模块对齐另开卡 |
| Assay 字段 | `design.assays[]` 与 `assay@1` 同时声明 `endpoint/method/unit/timepoints/group refs/timepoint_relation/protocol_version_id` | 保留 `design.assays[]` 为权威；`assay@1` 实例引用 Design ID，不复制计划值 | Run `protocol_manifest` 只从 DesignVersion 解析；模块副本无法证明是最新计划 | DOC-05；后续模块对齐另开卡 |
| 孔位/样本映射 | `plate@1` 文档 well-map、`design.sample_allocations.physical_slots_per_group`、`plate`/`well` 记录三处出现位置信息 | 计划数量归 Design；实际坐标归 `plate`/`well`；`plate@1` 文档仅为编辑草案 | 三者语义不同（计划数/计划布局/实际映射），混用会生成假坐标 | DOC-05；UI 卡引用本表 |
| 读数数组 | 同一事件可带 `result_cells[]`、`measurements[]`、`readouts[]` 三种表示 | 事件仍是唯一权威；存在 `result_cells` 时以其为规范网格，`measurements` 视作查询镜像，`readouts` 仅旧数据兼容 | 避免结果表格与孔板视图各取一份而分叉；契约已规定优先级 | DOC-05（写入规则）；Core 后续可收紧写入 |
| 派生读数 | `derived` readout 与 `ReadingFact` 都可能被界面当成“结果” | 保留事件中的 `derived` 为权威派生记录；`ReadingFact` 只收 `raw`/`missing` | 派生值缺少输入与公式就不应成为事实；契约明确排除 | DOC-05 |
| Notebook 存储 | 散文与 Run 事实同存 `execution_event`，仅靠 `stage` 区分 | 保留同表追加；Notebook 投影必须按 `stage=notebook` 过滤，Run 事实查询必须排除该 stage | 分离表会造成第二写入路径；同表则必须用显式判别字段 | DOC-05 + Notebook/Run 边界卡 |
| Notebook 写入点 | Notebook 页用 `RunService.SaveNotebook` 写 `execution_event(stage=notebook)`；CLI `note.create` 另写 `kind=note` 记录（`src/cli.rs:9954-9979`），双方互不读取 | 保留 `SaveNotebook` 为 Notebook 散文权威；`note.create` 标注为独立旧记录类型，不得当 Notebook 正文或 Run 事实 | 同名“note”存在两个写点，读者会拿错记录 | DOC-05（AGENT_API 说明）+ CLI 收缩卡 |
| 全局 Protocol 版本 | `design.protocol_version_id`（旧）与每 Assay `protocol_version_id` 并存 | 新设计以每 Assay 为权威；全局字段仅旧数据兼容，manifest 中 `assay_id=null` | 多检测设计不能共享一个全局版本；契约已声明优先级 | DOC-05 |
| 参数层级 | `group.condition` 文本、Protocol 绑定、`actual_parameters[]`、模块 `quantity` 四层都可能写浓度/体积 | 按层分工：Design 条件文本、Protocol 计划绑定、事件实际参数、模块草案输入；派生值只进 `derived` 输出 | 同名数值跨层混用会把目标值当实测值 | DOC-05（来源标注规则） |
| `StepMeta` 与 `module_contract` | 契约规定模块契约为准、`StepMeta` 只作补充；`StepMeta` 尚未实现 | 保留该优先级；在 Schema 冻结前禁止任何字段双写 | 同一字段两个权威值无法校验一致性 | DOC-05；StepMeta 卡另行评审 |

## 4. 模块目录与契约陈旧事实（供 DOC-05）

审计方法：以 `modules.json`、`registry.go`、`crates/asterlab-core/src/**` 为事实源，逐条对照 `docs/sdk/module-catalog-v1.md`、`docs/sdk/module-contract-v1.md`、`docs/contracts/core-v1.md`。

| 对象 | 现状（文件/行） | 处置 | 理由 | 目标卡 |
|---|---|---|---|---|
| 模块总数 | `docs/sdk/module-catalog-v1.md:3` 与 `:145` 写“15 个已注册模块”；`modules.json` 实有 16 个，`registry.go:285-290` 注册 16 个 spec | 改为 16，并同步 §1.1 矩阵 | 计数与 manifest、注册代码不一致；权威来源就是 manifest | DOC-05 |
| 模块总表 | `docs/sdk/module-catalog-v1.md:24` 已列 `rna_reagent_usage@1`，但 §1.1 模块就绪矩阵（`:37-48`）没有该行 | 在矩阵补 `rna_reagent_usage@1` 行 | 总表与就绪矩阵覆盖范围不一致，读者会误判 RNA 模块未注册 | DOC-05 |
| 权限枚举 | `module-contract-v1.md:109` 写当前权限含 `run.readout`、`notebook.append`；manifest 16 个模块的 `permissions` 实际只有 `document.edit` | 改为“常量已定义（`registry.go:33-34`），当前无模块声明后两者” | 文档把已定义常量误写成已声明权限 | DOC-05 |
| Renderers 维度 | `module-contract-v1.md:21` 的 12 维表只列 `renderers.design/.document/.static`；manifest 每个模块都有 `renderers.run`，同文 `:116` 示例也含 `run` | 维度表补 `renderers.run` | 契约正文与自身示例、manifest 不一致 | DOC-05 |
| 模板能力 | `module-contract-v1.md:195` 写模板实例化“not implemented”；`module-catalog-v1.md:181/189` 写 `internal/asterdoc/template.go` 已实现 | 以代码为准改为“Go API 已实现，无 CLI/市场” | 两份权威文档直接矛盾；`template.go` 存在 | DOC-05 |
| `ProtocolPreview` 形状 | `core-v1.md:93-107` 声明 `protocol_version_id`、`root_block_version_id`、`ordered_steps`、`parameters_ready`；`protocol_preview.rs:10-22` 实际为 `block_version_id`、`block_id`、`title`、`version`、`ready`、`steps` | 按实际类型重写契约字段，或删除未实现别名 | 契约字段与可编译类型不一致，Agent 会按错字段调用 | DOC-05 |
| `ready` 语义 | `core-v1.md:103/107` 说 `parameters_ready` 为新字段、`ready` 为兼容别名；实现只有 `ready`（`protocol_preview.rs:15`） | 二选一：实现 `parameters_ready`，或删掉该别名声明 | 文档承诺的字段不存在，`ready` 语义仍易被误读为科学就绪 | DOC-05 |
| Core 实现状态 | `core-v1.md:3-4` 仍写“P0 契约冻结；不是已实现代码”；`crates/asterlab-core/src` 已有 `design_map.rs`、`run_workspace.rs`、`reading_facts.rs` 等实现与测试 | 更新状态行，区分“已实现类型”与“冻结边界” | 状态描述会让 DOC-05/Agent 误判能力缺口 | DOC-05 |
| `occurrences` 投影 | 契约要求 `RunWorkspace.occurrences` 来自实际事件；`run_workspace.rs:529` 直接置空 `Vec::new()`，测试也断言为空 | 明确标注为已知实现缺口；读方不得依赖 `occurrences` | 事件里已有 `occurrence_id`，但工作区投影未暴露实例，界面会误判“无执行” | 待开 Run Core 卡 |
| `module-catalog` 就绪矩阵声明 | 矩阵标题称“四个独立维度”，但 `schema/校验` 列与模块总数/权限描述已漂移 | 与上列计数、权限、renderers 一并修订 | 同一文档内的权威声明需要一次修订闭合 | DOC-05 |

## 5. 未决问题

1. **模块实例与 Design 记录的二选一尚未落到 schema。** 本报告只能规定“Design 记录权威、模块实例草案”，但现有 AsterDoc 文档仍可携带 `independent_n`、`timepoints` 等计划字段；在 DOC-05 定稿前，任何读方都不得把模块字段当冻结计划。
2. **`result_cells` 与 `measurements` 的规范角色需要写入契约。** 当前 `reading_facts` 已有兼容优先级，但 `execution_event.create` 仍允许只写其一；建议在下一版 Core 契约中明确“写入者必须同时提供网格与测量镜像，读方以网格为准”。
3. **`StepOccurrence` 的持久化位置未定。** 事件携带 `occurrence_id` 与上下文，但 `RunWorkspace.occurrences` 未投影；需要 Run Core 卡决定是继续从事件派生，还是增加独立 occurrence 记录。
4. **Notebook 与 Run 共用事件表是否需要 stage 级约束。** 目前靠 `stage=notebook` 约定；建议 Core 在读取事实/时间线时显式排除该 stage，避免散文进入读数或完成状态。
5. **全局 `protocol_version_id` 的弃用窗口未定。** 旧设计仍可读，但新写入是否拒绝该字段、何时迁移，需要 DOC-05 与迁移卡共同决定。
6. **`parameters_ready` 的兼容策略未定。** 是补实现还是删文档，会影响 CLI `protocol.block.preview` 的消费者。

## 6. 复核命令与证据

```bash
# 字段表行数与“权威”声明数
test -f docs/sdk/science-field-ownership.md && grep -c '^| ' docs/sdk/science-field-ownership.md
grep -c '权威' docs/sdk/science-field-ownership.md
# 只允许 write 范围变化
git status --short
# 陈旧事实复核
python3 - <<'PY'
import json
d=json.load(open('prototypes/mygo-run/internal/asterdoc/spec/modules.json'))
print('manifest modules:', len(d['modules']))
print('permissions:', sorted({p for m in d['modules'] for p in m.get('permissions',[])}))
print('renderer keys:', sorted({k for m in d['modules'] for k in m.get('renderers',{})}))
PY
grep -n "15 个已注册模块\|注册 15 个" docs/sdk/module-catalog-v1.md
grep -n "run.readout\|notebook.append" docs/sdk/module-contract-v1.md
grep -n "ordered_steps\|parameters_ready\|不是已实现代码" docs/contracts/core-v1.md
grep -n "let occurrences = Vec::new()" crates/asterlab-core/src/run_workspace.rs
```

原始输出见 `docs/release/evidence/SCI-01/field-check.out` 与 `docs/release/evidence/SCI-01/acceptance.out`。
