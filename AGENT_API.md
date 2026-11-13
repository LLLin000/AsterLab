# Agent CLI 契约（v1）

Agent 通过 `.\target\release\labworkbench.exe` 的命名子命令操作实验工作台。界面和 CLI 共用本机 SQLite；Agent 不使用通用 `kind + data` 写入接口，也没有独立“Agent 草稿”页面。

产品概念和规范用词见 [CONTEXT.md](CONTEXT.md)。本文描述当前可运行 CLI 的真实输入输出契约；术语文件中的目标模型尚不表示这些运行时能力已实现。

## 通用调用规则

```powershell
.\target\release\labworkbench.exe tools --json
.\target\release\labworkbench.exe schema design.create
.\target\release\labworkbench.exe design.create --input .\design.json
```

每个业务命令只接受一个 `--input <json-file|->` 参数：`-` 表示从 stdin 读取。JSON 最大 2 MiB，UTF-8；每个命令使用单独的封闭 schema，未知字段、错误类型和超范围值会被拒绝。嵌套对象也封闭校验，不允许 Agent 顺手塞入未定义字段。schema 查询返回该命令完整的 JSON Schema，包括输入字段、必需项、枚举和成功/失败输出形状。工具清单中也内嵌相同 schema。

`tools --json` 中每项都提供 `description`、`side_effects`、`writes_data`、`input_schema` 和 `output_schema`。`side_effects` 说明会创建、修订、追加版本，或明确说明只读/只计算；Agent 应在调用前按输入 schema 组装参数，并按输出 schema 读取结果，不能从自然语言文案猜结果。

stdout 每次只输出一行 JSON。成功结构为：

```json
{"ok":true,"tool":"design.create","result":{"design":{},"version":{}}}
```

失败结构为：

```json
{"ok":false,"tool":"design.revise","error":{"code":"REVISION_CONFLICT","message":"设计版本已更新；请先重新读取。"}}
```

退出码固定为 `0` 成功、`2` 输入/用法错误、`3` 目标不存在、`4` 版本冲突、`5` 内部错误。不要从 stderr 文案推断业务结果；以 JSON 的 `ok` 与 `error.code` 为准。

## 命令目录

| 命令 | 用途 | 写入 |
|---|---|---:|
| `project.list` / `project.create` / `project.revise` | 查询、创建或按版本修订项目 | create/revise |
| `design.list` / `design.get` / `design.map` / `design.create` / `design.revise` | 查询、只读实验地图、创建或修订实验设计 | create/revise |
| `run.list` / `run.workspace` / `run.create` / `run.set_status` / `run.revise` | 查询、只读冻结工作区、创建运行、更新状态或只修订名称/计划日期 | create/update |
| `workflow.inspect_tree` | 只读投影 Design（可选 Run）的 Use/Definition/Occurrence 层级树 | 否 |
| `task.create` / `task.set_status` | 创建日历任务或更新状态 | create/update |
| `protocol.list` / `protocol.create` / `protocol.revise` | 查询 Protocol、创建 v1、追加新版本 | create/revise |
| `protocol.block.list` / `protocol.block.get` | 查询项目步骤块库和不可变版本 | 否 |
| `protocol.block.create` / `protocol.block.revise` | 创建步骤块或基于预期版本追加新版本 | create/revise |
| `protocol.block.preview` | 只读递归展开、解析参数来源并列出未填写槽位 | 否 |
| `step.edit` | 步骤块级 insert/reorder/reuse/bind 编辑：无 `confirmation` 时只读校验，带 `confirmation` 时追加新版本 | validate 否 / commit 是 |
| `plate.create` / `well.set` | 创建铺板布局、设置孔位标签 | create/update |
| `note.create` | 给实验运行追加自由文本记录 | create |
| `execution_event.create` | 追加结构化操作、材料使用、数值测量、偏差或观察事件 | create |
| `record.list` / `record.get` | 按 kind / project 查询，或按 id 读取一条记录 | 否 |
| `formula.create` / `formula.revise` | 创建或按版本修订公式 | create/revise |
| `table.create` / `table.revise` | 创建或按版本替换资料表 | create/revise |
| `calculator.*`（4 个） | 稀释、溶液质量、系列稀释、反应体积 | 否 |
| `import.study.preview` / `import.study.apply` | 预检并导入项目、设计和 Protocol | apply only |

所有工具的字段细节以 `labworkbench schema <命令>` 为准。Agent 不得自行扩展字段或把别的记录类型塞进现有工具。删除与覆盖不是 CLI 能力。

设计读取与版本写入有固定结果形状：`design.get`、`design.create` 返回 `{ "design": <当前设计记录>, "version": <对应 design_version 记录> }`；`design.revise` 返回 `{ "record": <修订后的设计记录>, "version": <新建的 design_version 记录> }`。`protocol.create` 返回 Protocol 记录和 v1 正文版本，`protocol.revise` 只返回新建的正文版本记录。所有成功响应使用 `ok=true` 和工具名；失败响应使用 `ok=false`、稳定错误码和可读 `message`。每项输出字段以工具自己的 `output_schema` 为准。

`design.map` 是只读实验地图投影：输入 `{ "id": "<design-id>" }`，输出 `design_id`、`revision`、`title`、`objective`、`experimental_unit`、`groups[]`、`assays[]`、`sample_links[]`、`planned_outputs[]`、`conflicts[]` 和 `unresolved[]`。它只复制已有设计字段，并由显式缺失/未解决冲突生成 `unresolved`（如独立 n、时间点、Protocol、时间点关系缺失）；不补造组别、对照、n、剂量、时间点或孔位，也不写数据库。HTTP 对应 `GET /api/designs/{id}/map`。

## Protocol 步骤块契约

Protocol 可以继续保存旧式 body 文本，也可以引用一个确切的 root_block_version_id。步骤块是唯一的可复用操作对象。protocol.block.create 的 steps 接受两种项：

- text：包含 use_id、title、instruction 和 parameter_refs，是普通可读操作。只有列入 parameter_refs 的槽位才作为该步骤的显式输入。
- block_use：包含 use_id、block_version_id 和 bindings，在此位置组合一个确切的子块版本。binding 必须且只能提供 value 或 source_slot；类型和单位必须一致，不做隐式换算。每个必填子块槽位都要明确映射或给出值。
- text 可选 `visualization`：`transfer`、`incubation`、`plate_layout`、`sample_split`、`measurement`。它只决定操作台采用哪种图解布局，不新增实验事实；示意中的样本、孔位和参数都从 Run 冻结清单及实际记录读取。没有合适图解时省略，继续显示文字步骤。
- text 可选 `time_scope`：`once`、`per_timepoint`、`per_assay`、`unresolved`。它表达步骤的执行时间作用域，不从标题/类型推断；仅 `per_timepoint` 才会在 RunWorkspace 中按 Assay 时间点展开任务。缺失或 `unresolved` 时保留无确定时间点的任务，不批量复制到所有检测时间点。

参数槽位包括 key、label、value_type（text、number、integer、boolean）、可选 unit 和 required。步骤块版本不可变；组合块固定引用子块的确切版本。只改本次条件或浓度时，保持步骤块原样，创建新的 Protocol 版本并更新 root_bindings，不复制步骤块。

推荐调用顺序：先用 protocol.block.list / protocol.block.get 读取候选定义；用 protocol.block.preview 检查展开顺序、参数来源与 unresolved_parameters；确认结构确实改变后才用 protocol.block.revise，并携带 expected_version。草案允许有未绑定槽位，preview.ready=false 表示当前方案仍有必填参数待确认，不代表数据库拒绝保存。

run.create 会把 Assay 引用的 Protocol 版本固定到 `protocol_manifest`，并保存根步骤块、全部子块版本依赖、展开步骤、已解析参数与未解决参数。另按每个 `PlannedOutput` 生成一条 `material_manifest`：固定制备 Protocol 与参数，并列出引用该产物的检测/组别。多个消费者仍只列一条计划制备；各检测自己的使用 Protocol 仍分别保留。步骤块引用只是内容复用，不代表实际操作或材料复用；同一块在两个位置出现，仍是两个 BlockUse。Run 清单描述冻结的计划，不证明材料已经制备或实际共用。

为旧数据保留的全局 protocol_version_id 也会进入同一清单，assay_id 为 null，避免全局 Protocol 的根步骤块和参数在 Run 快照中遗漏；新设计优先在各 Assay 上分别指定 Protocol 版本。

当前 CLI 支持多步骤和递归组合；网页编辑器可创建基础文字步骤块、填写参数并预览/绑定根步骤块。复杂多步骤或嵌套组合请使用封闭的 CLI 工具契约，网页展示仍可读取并预览这些组合。

运行可以先按草案状态创建，以便排期和核对；若其 Protocol manifest 中仍有必填参数未确认，run.set_status 切换到 in_progress 会被拒绝。先追加带完整参数的新 Protocol 版本，再按该版本建立新的 Run。未解决的可选槽位不会自动阻断。

`workflow.inspect_tree` 是只读层级查询：输入 `{design_id, revision?, run_id?}`，返回 `roots[]`、`order`、`precedes`、`issues` 和节点上的 `identity`、`useId`、`blockVersionId`、`instruction`、`parameters`、`unresolvedParameters`、`taskKey`、`status`。`design=true` 表示没有 Run；带 `run_id` 时在相同层级上叠加 Occurrence。CLI 通过 `ASTERLAB_APP_BIN` 调用同一 Go 层级引擎，不在 Rust CLI 中复制投影算法。字段见 `docs/sdk/workflow-tree-contract-v1.md`。

`run.workspace` 只读读取 Run 冻结的 `design_version_id` 和 `protocol_manifest`，返回 `frozen_design`、`scopes`、`steps`、`tasks`、`occurrences`、`materials`、`plates`、`events`；不存在冻结快照时返回错误，不回退当前 Design。旧事件没有 `occurrence_id` 时保留为历史，不自动生成执行实例。

`run.revise` 必须提供 `id` 和当前 `expected_revision`，只接受 `title`、`scheduled_date`；日期可传 `null` 清除。状态仍通过 `run.set_status` 修订。设计版本、Protocol 清单、材料清单、轮次和来源关系属于创建时冻结快照，修订接口拒绝这些字段。HTTP API 与 CLI 共用同一补丁校验；网页不得整体替换 Run JSON。

## `step.edit`：步骤块级结构化编辑

`step.edit` 在 agent edit v1 的四段流程（propose → validate → confirm → commit）里承担步骤块结构修改，不引入第二套写入路径。同一个命令有两种阶段：

- 不带 `confirmation`：只读校验，返回 `status`、`edit_digest`、解析后的 `ops` 和计划生成的 `blocks[]`；不写数据库。
- 带 `confirmation`：单事务重新校验并追加新版本；旧版本、Protocol 版本与已冻结 Run 均不改写。

```json
{
  "schema_version": 1,
  "edit_id": "sde-rna-001",
  "block_version_id": "<读取时的确切步骤块版本 id>",
  "expected_version": 3,
  "origin": {"source": "agent", "rationale": "补一个洗涤子步骤"},
  "ops": [
    {"op": "insert", "path": "root/lyse", "position": 1,
     "step": {"kind": "text", "use_id": "wash", "title": "洗涤", "instruction": "加 200 uL 缓冲液后弃上清"}},
    {"op": "reorder", "path": "root", "order": ["prep", "lyse"]},
    {"op": "reuse", "path": "root", "position": 2, "use_id": "read",
     "block_version_id": "<已存在的步骤块版本 id>",
     "bindings": [{"slot": "volume_per_sample", "value": 200, "unit": "uL"}]},
    {"op": "bind", "path": "root/lyse", "bindings": [{"slot": "volume_per_sample", "value": 250, "unit": "uL"}]}
  ],
  "confirmation": {
    "edit_digest": "sha256:<validate 返回的 digest>",
    "confirmed_by": "researcher:zhang",
    "idempotency_key": "sde-rna-001-confirm-1",
    "note": "按试剂盒说明书补洗涤步骤"
  }
}
```

规则：

- `block_version_id` 必须是根步骤块的当前 head；`expected_version` 必须等于它的版本号，否则 `VERSION_CONFLICT`（不是 head 则为 `STALE_REFERENCE`）。被改动的子块也必须仍是各自块的 head，否则同样 `VERSION_CONFLICT`。
- `path` 使用 Protocol 展开寻址：`root` 表示根块，`root/<use_id>` 表示某处块引用，`root/<use_id>/<use_id>` 可继续加深。不是以 `root` 开头或含空段/`.`/`..` → `FORBIDDEN_FIELD`；格式合法但解析不到步骤 → `STALE_REFERENCE`；嵌套超过 12 层 → `INVALID_PROPOSAL`。
  `insert` / `reorder` / `reuse` 的 `path` 必须是容器（`root` 或块引用步骤）；指向文字步骤返回 `INVALID_PROPOSAL`，不会退化成修改其父块。`bind` 的 `path` 必须指向块引用步骤，指向文字步骤返回 `INVALID_BINDING`。
- `insert` 只接受 `step.kind = "text"`（`parameter_refs` 必须引用本块已声明的槽位）；复用已有步骤块用 `reuse`。`reorder.order` 必须是该容器当前 `use_id` 的完整排列。`bind` 按槽位 upsert，绑定规则与 `protocol.block.create` 一致：每个必填子槽位必须有值或映射，单位精确匹配、不做换算。
- 版本号由 Core 生成：每个被改动的步骤块得到 `previous_version + 1` 的新版本，子块先于父块写入，父块引用自动指向同一事务里新建的子块版本。未改动的块保持原版本（复用）。每个新版本记录带 `based_on_version_id` 和 `step_edit`（`edit_id`、`edit_digest`、`confirmed_by`、`committed_at`、`idempotency_key`）。
- `edit_digest` 只覆盖 `ops` 与版本前置条件，不覆盖 `confirmation` 本身；同一 `idempotency_key` + 同一 digest 重放返回原结果（`replayed: true`，不写第二个版本），同一 key 配不同 digest 返回 `IDEMPOTENCY_MISMATCH`。
- 错误码沿用 agent edit v1 的封闭集合（`INVALID_PROPOSAL`、`INVALID_BINDING`、`UNIT_MISMATCH`、`STALE_REFERENCE`、`FORBIDDEN_FIELD`、`VERSION_CONFLICT`、`CONFIRMATION_REQUIRED`、`CONFIRMATION_MISMATCH`、`IDEMPOTENCY_MISMATCH`），没有新增含义。退出码仍为 `0/2/3/4/5`。
- 语义拒绝是结果而非传输错误：validate 阶段返回 `ok=true` + `status:"blocked"` + `issues[]`（`code`/`severity`/`path`/`message`/`affects`/`next_action`），blocked 结果不带 `blocks[]`，不得据此 commit。
- 编辑结果只产生步骤块版本。Protocol 版本不可变，若要某个 Protocol 使用新根块，另调 `protocol.revise`（`root_block_version_id` 用返回的 `new_root_block_version_id`）；已冻结 Run 继续引用旧版本。

## 设计数据语义

`design.create` 接收 `project_id` 和 `design` 对象。关键字段含义：

- `experimental_unit`：一个独立观测对应的实验单位定义，例如一只动物、一个培养孔或一个患者；需要由研究者提供。
- `group_specs[]`：每组一个对象：`name`、可选枚举 `role`、具体 `condition`、`independent_n`、`technical_repeats`。`independent_n` 是每组独立实验单位数；技术重复单独填写。系统不把孔、切片或重复测量自动视为独立 n。
- `timepoints[]`：按实验方案填写的采样或测量时间点。空数组表示尚未提供，不表示“无时间维度”。
- `timepoint_timing[]`（可选）：将 `timepoint` 与 `offset_value`、`unit`（`min` / `h` / `day`）及可选提前/延后窗口绑定。设置偏移或窗口时必须同时提供 `time_anchor_event`；时间点顺序与标签必须和 `timepoints[]` 一致。它是计划时间，不等于实际发生时间。
- `primary_endpoint`、`endpoint_method`、`endpoint_unit`：主要测量对象、已确定的方法和适用单位；不适用的单位留空。
- `sample_size_rationale`、`randomization`、`blinding`、纳排标准和 `analysis_plan` 只记录研究者已给出的依据，不由 Agent 臆造。
- `plate.create.format` 只能是 `6`、`12`、`24`、`48`、`96`、`384` 或 `custom`。标准板型的 `rows` / `columns` 必须与板型对应；非标准行列使用 `custom`。
- `plate.create.assay_id` 可将实际板图绑定到 Run 冻结设计中的检测；`timepoint` 需属于该检测冻结的时间点。只要检测或该检测任一组的 `timepoint_relation` 为 `independent_sample_per_timepoint`，且已列出时间点，就必须提供 `timepoint`。同一实验单位或同一份实体样本跨时间重复测量时，板图可不绑定单一时间点；若关系待确认，则不据此生成分时板图。没有实际映射时只显示计划容器位，不生成假坐标。这条 timepoint 绑定规则不证明重复测量一定复用同一实体板；实际板位关系仍需由铺板工具/实际映射明确。

多检测设计可在同一 `design` 中使用以下固定字段；未提供数组可省略，表示尚未录入：

- `research_question`（可选）：独立的研究问题；`objective` 仍表示目标/目的。旧记录无此字段时回退为空。
- `factors[]`（可选）：`{name, levels[]}` 的结构化因素与水平；名称非空且唯一，单因素最多 50 个水平。它是设计语义，不从组别文本推断。
- `assays[]`：每项检测有稳定 `id`、`title`、`endpoint`、`method`、`unit`、自己的 `timepoints[]`、可选 `timepoint_timing[]`、`group_ids[]`、`group_names[]`（兼容显示字段）、`control_group_ids[]`、`time_anchor_event`、`timepoint_relation`、`container_type` / `container_format`、`protocol_version_id` 和 `controls[]`。`timepoint_relation` 只能是 `same_experimental_unit_repeated`、`same_physical_sample_repeated`、`independent_sample_per_timepoint`、`unknown` 或 `null`。每项检测自己的时间、适用组、对照和 Protocol 不从其他检测继承。旧字段 `sample_use_policy` 只为兼容旧输入保留，不写入规范化输出；`new_per_timepoint` 可迁移为独立取样，含糊的 `same_sample_repeated` 会迁移为 `unknown`，不会擅自解释成重复测量同一实体样本。旧字段中的 `split_from_assay` / `sequential_after_assay` 也不再作为时间关系使用。当前 `controls[]` 是带来源的补充描述文本，不参与孔位/材料数量计算。
- `sample_allocations[]`：内部“组别 → 检测/计划产物”的分配关系。新记录使用 `timepoint_relation` 表达时间点关系，使用 `source_relation` 表达跨检测来源（`not_applicable`、`unknown`、`split_from_assay`、`sequential_after_assay`）；旧字段 `relation` 只兼容旧输入，不作为新记录的时间点关系依据，规范化输出不包含该字段。分样和顺序使用须提供 `source_assay_id`。`physical_slots_per_group` 记录每组的计划物理位置数，不等同实体样本数或独立 n；physical_slots_scope 说明该数按 per_timepoint（每个时间点）、whole_assay_unique_positions（全检测唯一位置）或 unknown（范围待确认）解释。旧记录有位置数但未给范围时规范化为 unknown，数字保留。时间点关系不推断位置范围；设计页只显示按组录入的数量与范围，不汇总成跨组/跨时间布局总数，实际坐标与总布局由铺板工具映射确认。没有明确样本分配依据时，实体样本数不从 n 或时间点推算。多个分配引用同一个 `planned_output_id` 才表示计划共用一次制备。
- `planned_outputs[]`：每项计划产物有稳定 `id`、`title`、制备 `protocol_version_id`、`root_bindings[]` 参数绑定和可选 `evidence_id`。相同产物 ID 是多项检测计划共用同一份制备的明确表达；相同 Protocol 版本本身不表示共用。
- `source_evidence[]`：`id`、`source_document`、可选 `source_date` / `source_locator`、原始 `claim` 和 `state`。locator 用章节、表格行、行号等定位原文；不要求置信度分数。state 只能是 `source_record`、`proposal`、`user_confirmed`、`calculated`、`agent_suggestion`。原始证据保留，系统推导或 Agent 建议不能改写成实验事实。
- `evidence_conflicts[]`：`id`、`topic`、至少两个 `evidence_ids[]`、`status`（`unresolved` / `resolved`）和可选 `resolution`。`affected_assay_ids[]` 可关联受影响的检测，必须引用本设计中已定义且不重复的检测 ID；当 `affects_container_estimate=true` 时至少关联一项检测。此标记只让界面将对应检测的孔/容器数显示为“暂定”，不自动判定来源冲突。已确认解释必须写明依据；未确认时保持 `unresolved`，不覆盖原始记录。
- `planned_run_count` 是计划执行轮次数；`planned_run_relationship` 只能是 `independent`、`same_source`、`paired`、`unknown`。轮次数不会被合并成独立 n。

创建示例：

```json
{
  "project_id": "<project-id>",
  "design": {
    "title": "示例：处理条件比较",
    "objective": "比较预先指定处理条件下的主要终点",
    "experimental_unit": "一个独立培养孔",
    "group_specs": [
      {"name":"对照组","role":"control","condition":"对照条件待研究者确认"},
      {"name":"处理组","role":"treatment","condition":"处理剂量和时长待研究者确认"}
    ],
    "timepoints": [],
    "primary_endpoint": "待研究者指定",
    "endpoint_method": "",
    "endpoint_unit": ""
  }
}
```

示例中的未确认值只是说明如何表达缺失信息；不能当作一个可执行的已完成方案。未知的独立 n、处理条件、时间点、终点或检测方法必须保留为空并在输出中列为待确认。`design.revise` 必须提供 `id`、读取到的 `expected_revision` 和 `changes`。只修改 `changes` 中出现的字段；空字符串或空数组可用于清空字段。版本不匹配时返回 `REVISION_CONFLICT`，必须重新读取后再提修订。

## 导入边界

导入只支持当前 schema 中明确列出的 `project`、`designs[]`、`protocols[]`。Agent 可以先读取外部文件并抽取为这个结构，但不能把原始文件内容、任意 JSON 或未知记录类型直接写入数据库。其他实体通过各自的 `*.create` 工具创建，按工具的输入 schema 校验。

先调用 `import.study.preview`。它**不写入数据库**，返回对象数量与缺失关键字段警告。检查映射、记录归属和待确认项后，只有确定需要落库时才调用 `import.study.apply`；apply 在一个事务中创建项目及其设计版本、Protocol v1，失败时整体回滚。当前不支持导入原始实验结果、附件、任意表结构或通用数据库备份。

## 安全与科学边界

- 只从用户提供的文件、当前项目记录或用户明确确认的来源提取事实。数值、单位、时间、剂量、样本数、处理条件不得猜填。
- 每次生成方案，明确区分“已提供”“从已保存记录读取”“计算所得”“待确认”；不使用“适量”“按常规”“视情况”等无法执行的值。
- `run.create` 是受信任本地写入端操作，CLI 调用必须设置 `LABWORKBENCH_TRUSTED_WRITER=1`；外部 Agent 不应获得该权限，只生成 Protocol/Design/View Draft，由人类确认后通过本地 UI 创建 Run。
- 创建运行时固定引用当前 `design_revision`，并生成 `protocol_manifest[]` 固定该版本各检测所引用的 Protocol 精确版本。`run_index` 按设计递增；再次执行可传 `repeat_of_run_id` 与 `repeat_relationship`（`independent`、`same_source`、`paired`、`unknown`），不能仅凭轮次数断言独立性。Protocol 修订创建新版本，不覆盖历史版本。
- `note.create` 是追加式自由文本记录。`execution_event.create` 适合需要筛选、计算或追溯的实际事件：输入 `run_id`、`event_type`、`content`，可附 `occurred_at`（带时区 RFC 3339；不确定时留空）、`stage`、`operator`、`sample_ids[]`；`material_use` 事件可写 `materials[]`，`measurement` 事件必须含数值 `measurements[]` 或结构化 `readouts[]`，`deviation` 事件必须说明 `deviation`。
- `readouts[]` 每项为可选 `readout_id`、`endpoint`、可选 `value` 或 `qualifier`（`not_detected` / `below_lod` / `above_uloq` / `invalid` / `other`），二者必须且只能填一个；`result_kind` 为 `raw` 或 `derived`。派生结果必须提供 `method` 与 `derived_from[]`；`derived_from[]` 引用本次 Run 中已存在的原始 `readout_id`，不能引用未知或派生读数。不得把计算结果写成原始读数。
- `checks[]` 每项为 `check_id`、`status`（`passed` / `failed` / `blocked` / `skipped`）及可选 `checked_by`、`note`；`check_id` 必须存在于同一冻结 Protocol 步骤的 `checks[]`，同一事件不得重复，且检查结果必须关联该步骤。`skipped` 不等于通过；空白或未记录也不等于通过。`controls[]` 记录对照角色 `role`、`status`、样本/实验单位/观察；`lineage[]` 记录 `from_kind`、`from_id`、`to_kind`、`to_id`、`relation`。`actual_parameters[]` 记录实际执行参数 `key`、`value`、可选 `unit` 与 `source`，并与计划参数分开。
- Protocol 文本步骤可定义 `checks[]`：`check_id`、`label`、可选 `instruction` 与 `blocking`。检查项随 ProtocolPreview、Run 冻结清单一同保存；AsterLab 只记录人工结果，不自动判定 gate 通过。
- Protocol 步骤事件可附 `timepoint` 和 `step_status`。`timepoint` 仅用于明确属于某个检测时间点的步骤，并必须匹配 Run 冻结设计；共享制备步骤不应机械绑定某项检测的时间点。`step_status` 只能是 `in_progress`、`completed`、`blocked`、`skipped`，且必须同时关联冻结 Protocol 步骤；普通操作、观察、测量或偏差事件本身不代表步骤完成。未带时间点的旧事件不会被自动分配给 D1/D3。
- 写入工具只返回记录 ID、所属关系、版本、数据和创建/更新时间；不声称实际实验已经执行。
- 该 CLI 是本机原型，尚无多用户认证、审计签名、细粒度附件权限或撤销系统。不要把本地访问控制误称为云端安全边界。

## 组件/计划容器只读接口（C3）

| 命令 | 用途 | 写入 |
|---|---|---:|
| `component.list` | 列出 Definition 与 `implemented` 状态；未实现项只能作为 catalog-only | 否 |
| `component.define` | 注册不可变 ComponentDefinition（受信写入端） | create |
| `labware.define` | 注册不可变 LabwareProfile（受信写入端） | create |
| `view.project` | 从 ProtocolPreview 生成初始 ViewDocument；必须显式 `scope` | 否 |
| `view.create` / `view.get` / `view.revise` | 保存/读取/修订 ViewDocument；修订走 ViewChangeSet | create/revise |
| `view.data_preview` | 按显式 WellPlate `instance_id` 读取 PlannedContainer，执行已登记 Pure ToolAction 并返回 Table | 否 |
| `semantic.inspect` / `semantic.search` / `semantic.validate` / `semantic.propose_patch` / `semantic.assess_patch` | 只读语义查询与 Patch 评估 | 否 |
| `semantic.apply_binding_patch` | 受信写入端创建新的不可变 BlockVersion；不自动改 Protocol/Run | create |

`view.project` 的 `scope` 必须是 `{"kind":"assay","assay_id":"..."}`、`{"kind":"planned_output","planned_output_id":"..."}` 或 `{"kind":"unscoped"}`。无 Assay 关联的只读预览应显式 unscoped；不能自动把未知作用域当作某个 Assay。

HTTP: `GET /api/views/{id}/data?instance_id=...&calculator_instance_id=...&extra_ratio=...` 对应同一只读投影。

`view.data_preview` 仍是最小只读投影：输入必须提供 `view_id` 和 WellPlate `instance_id`。可提供 `calculator_instance_id`，此时必须指向 `tool.calculator` 且 `props.tool_ref` 为 `tool.reagent_total@1`。体积输入必须显式提供 `volume_per_well` + `volume_unit`，`extra_ratio` 也必须显式提供；`volume_unit` 只接受 `uL`/`µL`/`mL`/`L`（`ul`、`μL` 归一化为 `uL`），非法单位返回输入错误。必须用 `position_ids`（逗号分隔，HTTP）或 `position_ids` 数组（CLI）明确本次操作的目标孔位；缺失时保持 `unresolved`，不自动统计该计划容器的全部已分配孔。缺少体积或额外比例返回 `unresolved`，不补默认值。当前尚未实现从任意 Table 行/列映射 Calculator 输入的通用契约；不会把表格第一格当作体积。它不写 Design/Protocol/Run。

`component.list` 的 `implemented=true` 表示当前有可用数据适配/测试；`catalog-only` 不代表可被 Agent 创建。12 个注册项中当前只有 workflow/operation/layout.section/content.table/tool.calculator/lab.well_plate 标为 implemented。
