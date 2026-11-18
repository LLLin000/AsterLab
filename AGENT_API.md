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

### 结构化诊断（`--diagnostics json`）

任何业务命令都可追加 `--diagnostics json`：

```powershell
.\target\release\labworkbench.exe design.revise --input .\revise.json --diagnostics json
```

带该参数时，成功和失败响应都在顶层多一个 `diagnostics` 数组。成功响应通常为 `[]`；只读校验命令（如 `agent_edit.validate`、`semantic.validate`、`asterdoc.validate`）把结论放在 `result.issues`，这些条目也会同时出现在 `diagnostics`，因此 Agent 只读一个数组即可。诊断项是 `asterlab_core::diagnostic::Diagnostic`，字段固定为：

```json
{
  "path": "assignments[0].position_id",
  "code": "assignment_position_invalid",
  "severity": "blocking",
  "message": "assignment position A１ is not valid for container plate-1",
  "affects": [],
  "next_action": "correct assignments[0].position_id and re-run validation",
  "source": "derived"
}
```

- `severity` 只取 `blocking`、`needs_confirmation`、`advisory`；`blocking` 表示该写入/操作被拒绝。
- `source` 只取 `user_confirmed`、`imported_record`、`referenced_protocol`、`derived`、`agent_suggestion`、`unknown`，说明这条结论的来源；Core 校验产生的结论是 `derived`。
- `code` 是稳定机器码，Agent 必须按 `code` 分支，不解析 `message`。
- `path` 定位字段/对象；`affects` 列出受影响的下游对象；`next_action` 给出下一步动作。
- 失败时 `error.code` 与 `error.message` 保持不变，`error.issues` 也继续返回；`diagnostics` 是它们的规范化视图。未带 `--diagnostics json` 时响应完全不变。
- 不带 `--diagnostics` 的旧调用方不受影响；`--diagnostics` 只接受 `json`，其他值返回 `INVALID_INPUT`。

## 命令目录

| 命令 | 用途 | 写入 |
|---|---|---:|
| `project.list` / `project.create` / `project.revise` | 查询、创建或按版本修订项目 | create/revise |
| `design.list` / `design.get` / `design.map` / `design.create` / `design.revise` | 查询、只读实验地图、创建或修订实验设计 | create/revise |
| `run.list` / `run.workspace` / `run.create` / `run.set_status` / `run.revise` | 查询、只读冻结工作区、创建运行、更新状态或只修订名称/计划日期 | create/update |
| `workflow.inspect_tree` | 只读投影 Design（可选 Run）的 Use/Definition/Occurrence 层级树 | 否 |
| `asterdoc.command` | 按 revision 新增步骤、重排/嵌套/提升，或安全删除 Design 步骤 | revise |
| `workflow.progress` | 只读投影一个 Run 每步骤的样本矩阵、attempt 历史与最后事件时间 | 否 |
| `task.create` / `task.set_status` | 创建日历任务或更新状态 | create/update |
| `protocol.list` / `protocol.create` / `protocol.revise` | 查询 Protocol、创建 v1、追加新版本 | create/revise |
| `protocol.block.list` / `protocol.block.get` | 查询项目步骤块库和不可变版本 | 否 |
| `protocol.block.create` / `protocol.block.revise` | 创建步骤块或基于预期版本追加新版本 | create/revise |
| `protocol.block.preview` | 只读递归展开、解析参数来源并列出未填写槽位 | 否 |
| `step.edit` | 步骤块级 insert/reorder/reuse/bind 编辑：无 `confirmation` 时只读校验，带 `confirmation` 时追加新版本 | validate 否 / commit 是 |
| `plate.create` / `well.set` | 创建铺板布局、设置孔位标签或 1D 泳道、检测目标/角色/稀释倍数 | create/update |
| `note.create` | 给实验运行追加自由文本记录 | create |
| `execution_event.create` | 追加结构化操作、材料使用、数值测量、偏差或观察事件 | create |
| `readout.submit` / `readout.batch` | 幂等提交单条读数，或表格/孔板批量读数（`cells` 或 CSV），一次事务写入一条 measurement 事件 | create |
| `record.list` / `record.get` | 按 kind / project 查询，或按 id 读取一条记录 | 否 |
| `asset.link` / `asset.link.update` / `asset.unlink` / `asset.list` / `asset.events` | 关联、就地更新坐标或解除 Run 任务的本地文件引用，或按 Run 查询链接与事件历史 | link/update/unlink 是，list/events 否 |
| `pantry.list` / `label.export` | 只读列出储藏室条目及其效期状态，或导出贴标/扫码插件用的短 ID 清单 | 否 |
| `pantry.upsert` | 创建或整条覆盖一个储藏室条目 | create/update |
| `pantry.delete` | 停用并隐藏一个储藏室条目；短标签 ID 保留不复用 | delete |
| `formula.create` / `formula.revise` | 创建或按版本修订公式 | create/revise |
| `table.create` / `table.revise` | 创建或按版本替换资料表 | create/revise |
| `calculator.*`（10 个） | 稀释（摩尔或质量浓度）、溶液质量、系列稀释、反应体积、GelMA 配液、细胞接种、运行期配平、多样本质量配平、体内给药、生物膜 CFU | 否 |
| `import.study.preview` / `import.study.apply` | 预检并导入项目、设计和 Protocol | apply only |

所有工具的字段细节以 `labworkbench schema <命令>` 为准。Agent 不得自行扩展字段或把别的记录类型塞进现有工具。删除与覆盖不是 CLI 能力。

设计读取与版本写入有固定结果形状：`design.get`、`design.create` 返回 `{ "design": <当前设计记录>, "version": <对应 design_version 记录> }`；`design.revise` 返回 `{ "record": <修订后的设计记录>, "version": <新建的 design_version 记录> }`。`protocol.create` 返回 Protocol 记录和 v1 正文版本，`protocol.revise` 只返回新建的正文版本记录。所有成功响应使用 `ok=true` 和工具名；失败响应使用 `ok=false`、稳定错误码和可读 `message`。每项输出字段以工具自己的 `output_schema` 为准。

`design.map` 是只读实验地图投影：输入 `{ "id": "<design-id>" }`，输出 `design_id`、`revision`、`title`、`objective`、`experimental_unit`、`groups[]`、`assays[]`、`sample_links[]`、`planned_outputs[]`、`conflicts[]` 和 `unresolved[]`。它只复制已有设计字段，并由显式缺失/未解决冲突生成 `unresolved`（如独立 n、时间点、Protocol、时间点关系缺失）；不补造组别、对照、n、剂量、时间点或孔位，也不写数据库。HTTP 对应 `GET /api/designs/{id}/map`。

## `asterdoc.command`：Design 步骤树写入

输入使用 `{design_id, expected_revision, command, operation_id?}`。`expected_revision` 必须等于正在编辑的 AsterDoc revision；成功追加一个不可变 revision，`operation_id` 用于重试幂等，外层成功/错误结构遵循通用 CLI 契约。完整闭合 schema 与输出字段以 `labworkbench schema asterdoc.command` 为准。

新增命令：

- `reorder_steps`：`{type, moduleId: <workflow-id>, order: [<step-id>...]}`。`order` 必须是 workflow `steps` 的完整、无重复置换，只更新 `business_order`，不改变 `edges`。
- `add_step`：`{type, module, workflowId?}`。`module.path` 缺失、空串或纯空白时，Core 写入派生路径 `root/<step-id>`；步骤因此从一开始就在 `workflow.inspect_tree` 中可达。已带 `path` 的模块原样保存。
- `nest_step`：`{type, moduleId: <step-id>, parentId: <parent-step-id>}`；`promote_step`：`{type, moduleId: <step-id>}`。两者更新目标步骤及其后代的 `path`；路径冲突和循环嵌套返回 `INVALID_INPUT`，不保存部分 revision。
  - 目标步骤缺 `path` 时先按稳定规则派生 `root/<step-id>`：`nest_step` 得到 `<parent_path>/<step-id>`（父步骤也缺 `path` 时同一次 revision 内补全 `root/<parent-id>`），`promote_step` 只补全 `root/<step-id>`。
  - 派生不能产生唯一无环路径时明确拒绝：`step_path_derivation_failed`（id 为空/含 `/` 或路径无最终段）、`step_path_already_nested`、`step_path_already_top_level`、`step_path_cycle`、`step_path_conflict`（消息指出两个步骤 id、冲突路径与改名示例）。完整规则见 [docs/contracts/asterdoc-step-path-v1.md](docs/contracts/asterdoc-step-path-v1.md)。
  - 已带 `path` 的步骤不被改写；补全只写新的 revision，冻结 Run 与历史 revision 的路径字节不变。
- `remove_step` 遇到被冻结 Run、其他模块/步骤引用、子步骤或 `protect_references: true` 时返回稳定错误 `STEP_REFERENCED`，不写入新 revision。允许删除时，workflow 的 `steps`、`business_order` 和关联 edges 在同一 AsterDoc revision 中清理。

既有 `add_step`、`connect_steps`、`disconnect_steps` 等命令负载保持兼容。`workflow.inspect_tree` 与 `asterdoc.compile` 使用同一 revision 中的稳定 step ID、path、business order 和显式依赖 edges；顺序不隐式生成依赖关系。

## Protocol 步骤块契约

### ProtocolVersion 内容查询

`protocol.version.get` 与 `protocol.version.list` 是只读查询：

- `protocol.version.get` 输入 `{"protocol_version_id":"<immutable-version-id>"}`，返回单个 ProtocolVersion 的 `protocol_version_id`、`protocol_id`、`version`、`root_block_version_id`、`root_bindings`、根参数 `parameter_slots`、递归展开的 `steps`、`resolved_parameters`、`unresolved_parameters`、`block_version_ids` 与 `frozen: true`。
- `protocol.version.list` 输入 `{"protocol_id":"<protocol-id>"}`，返回 `{"versions":[<上述内容>...]}`，按版本号升序。
- 未知或类型不匹配的 ID 返回 `NOT_FOUND`（退出码 3），不返回空对象。自由文本 Protocol 无根块时返回空步骤/参数数组，`root_block_version_id: null`；其不可变版本仍标记为 `frozen: true`。
- 两个查询都不写入数据库。精确字段约束以 `labworkbench schema protocol.version.get` / `labworkbench schema protocol.version.list` 为准。

MyGo 侧是同一份数据的只读入口：`DesignService.GetProtocolVersion(ctx, protocol_version_id)` 与 `DesignService.ListProtocolVersions(ctx, protocol_id)`（后者按版本号升序）。标量字段与 CLI 输出同名同义：`protocol_version_id`、`protocol_id`、`version`、`root_block_version_id`（可空）、`block_version_ids`（字符串数组）、`frozen`。`root_bindings`、`parameter_slots`、`steps`、`resolved_parameters`、`unresolved_parameters` 以 CLI 的 JSON 文本透传，和 `WorkflowTree` 同一约定：解码即用，不在 Go 侧重建。这些展开项含任意 JSON 值（参数 `value`、`provenance`），桥接层没有对应类型，因此不作为 Go struct 跨边界。未知 ID 经 `internal/state.MapError` 归为 `state.ErrNotFound`，返回错误而非空对象。

Protocol 可以继续保存旧式 body 文本，也可以引用一个确切的 root_block_version_id。步骤块是唯一的可复用操作对象。protocol.block.create 的 steps 接受两种项：

- text：包含 use_id、title、instruction 和 parameter_refs，是普通可读操作。只有列入 parameter_refs 的槽位才作为该步骤的显式输入。
- block_use：包含 use_id、block_version_id 和 bindings，在此位置组合一个确切的子块版本。binding 必须且只能提供 value 或 source_slot；类型和单位必须一致，不做隐式换算。每个必填子块槽位都要明确映射或给出值。
- text 可选 `visualization`：`transfer`、`incubation`、`plate_layout`、`sample_split`、`measurement`。它只决定操作台采用哪种图解布局，不新增实验事实；示意中的样本、孔位和参数都从 Run 冻结清单及实际记录读取。没有合适图解时省略，继续显示文字步骤。
- text 可选 `time_scope`：`once`、`per_timepoint`、`per_assay`、`unresolved`。它表达步骤的执行时间作用域，不从标题/类型推断；仅 `per_timepoint` 才会在 RunWorkspace 中按 Assay 时间点展开任务。缺失或 `unresolved` 时保留无确定时间点的任务，不批量复制到所有检测时间点。

参数槽位包括 key、label、value_type（text、number、integer、boolean）、可选 unit 和 required。步骤块版本不可变；组合块固定引用子块的确切版本。只改本次条件或浓度时，保持步骤块原样，创建新的 Protocol 版本并更新 root_bindings，不复制步骤块。

推荐调用顺序：先用 protocol.block.list / protocol.block.get 读取候选定义；用 protocol.block.preview 检查展开顺序、参数来源与 unresolved_parameters；确认结构确实改变后才用 protocol.block.revise，并携带 expected_version。草案允许有未绑定槽位，preview.ready=false 表示当前方案仍有必填参数待确认，不代表数据库拒绝保存。

run.create 会把 Assay 引用的 Protocol 版本固定到 `protocol_manifest`，并保存根步骤块、全部子块版本依赖、展开步骤、已解析参数与未解决参数。另按每个 `PlannedOutput` 生成一条 `material_manifest`：固定制备 Protocol 与参数，并列出引用该产物的检测/组别。多个消费者仍只列一条计划制备；各检测自己的使用 Protocol 仍分别保留。步骤块引用只是内容复用，不代表实际操作或材料复用；同一块在两个位置出现，仍是两个 BlockUse。Run 清单描述冻结的计划，不证明材料已经制备或实际共用。

为旧数据保留的全局 protocol_version_id 也会进入同一清单，assay_id 为 null，避免全局 Protocol 的根步骤块和参数在 Run 快照中遗漏；新设计优先在各 Assay 上分别指定 Protocol 版本。

`run.create` 另接受可选 `source_run_id`：同项目内提供样本的上游收获 Run。它允许指向**不同 Design** 的 Run（跨 Design 样本血统），校验只有两条——目标必须存在且必须是 Run，所属项目必须与本 Run 相同；跨项目引用返回 `INVALID_INPUT`，未知 ID 返回 `NOT_FOUND`。它和 `repeat_of_run_id` 是两种关系：后者仍是同一 Design 的再次执行，`source_run_id` 只表达材料来源，二者可同时出现，也不改变重复关系语义。

当前 CLI 支持多步骤和递归组合；网页编辑器可创建基础文字步骤块、填写参数并预览/绑定根步骤块。复杂多步骤或嵌套组合请使用封闭的 CLI 工具契约，网页展示仍可读取并预览这些组合。

运行可以先按草案状态创建，以便排期和核对；若其 Protocol manifest 中仍有必填参数未确认，run.set_status 切换到 in_progress 会被拒绝。先追加带完整参数的新 Protocol 版本，再按该版本建立新的 Run。未解决的可选槽位不会自动阻断。

`workflow.inspect_tree` 是只读层级查询：输入 `{design_id, revision?, run_id?}`，返回 `roots[]`、`order`、`precedes`、`issues` 和节点上的 `identity`、`useId`、`blockVersionId`、`instruction`、`parameters`、`unresolvedParameters`、`taskKey`、`status`。`design=true` 表示没有 Run；带 `run_id` 时在相同层级上叠加 Occurrence。由经典 `design.create` 冻结、没有 AsterDoc 文档的 Run 不报 `NOT_FOUND`：返回 `design=false`、空 `roots[]`/`order[]`，并在 `issues[]` 说明该 Run 没有 AsterDoc revision；只有冻结数据声明了 AsterDoc revision 却读不到时才返回 `NOT_FOUND`。字段见 `docs/sdk/workflow-tree-contract-v1.md`。

`workflow.progress` 是只读样本进度查询：输入 `{run_id, step_path?, timepoint?, sample_id?, sample_ids?}`，按实际事件携带的 `(step_path, timepoint)` 分组返回；`timepoint` 显式输出，缺失保持 `null`。不筛选时间点时只返回事件实际出现的分组，不构造 Run 全局样本 × 时间点笛卡尔积。只有同时提供 `step_path` 和具体 `timepoint` 时，`sample_ids[]` 才补入该组的 `not_started` 样本。结果含 `schema_version`、`run_id`、样本 ID 并集、`steps[]` 和 `issues[]`；每组含状态、`complete`、各状态计数、`last_event_at` 与 `samples[]`；状态枚举为 `not_started`、`active`、`in_progress`、`completed`、`failed`、`skipped`、`skipped_by_rule`、`qc_dropped`、`censored_dead`、`rework`，计数含 `active_samples`、`skipped_by_rule_samples`、`qc_dropped_samples`、`censored_dead_samples`、`rework_samples`。样本含当前状态/attempt、`attempt_count`、按 attempt 排列的历史（`event_ids`、`last_event_at`、`deviation`）。样本级身份为 `run_id + step_path + timepoint? + sample_id + attempt`；不同时间点独立去重和 attempt 连续性，重试保留旧 attempt。`complete=true` 只在该 `(step_path,timepoint)` 组所有样本均完成时成立；单个事件的 `step_status = "completed"` 只表示该事件所列样本完成。事件时间取 `recorded_at`，缺失时回退 `occurred_at`。`issues` 报告 `missing_step`、`unknown_step`、`invalid_attempt`、`retry_cycle`，不推断样本状态。

`run.workspace` 只读读取 Run 冻结的 `design_version_id` 和 `protocol_manifest`，返回 `frozen_design`、`scopes`、`steps`、`tasks`、`occurrences`、`materials`、`plates`、`linked_assets`、`asset_coverage`、`events`；不存在冻结快照时返回错误，不回退当前 Design。输入 `{id, view?}`：`view` 省略或为 `unblinded` 时保留冻结分组身份；`view=blind` 时只在冻结设计声明了盲法依据（`blinding` 非空）的情况下清除分组 id/name/role/condition、组样本数与孔位组标签，并替换自由文本中的身份串，其他值返回 `INVALID_INPUT`。`occurrences[]` 只由带 `occurrence_id` 的事件投影，旧事件没有 `occurrence_id` 时保留为历史，不自动生成执行实例。`linked_assets[]` 逐条带出 `asset_id`、`task_key`、`sample_id`、`well_id`、`timepoint`、`fov_index`、`coordinate_id`；`asset_coverage[]` 按 `(well_id, timepoint)` 分组给出 `fov_indices[]` 与 `asset_ids[]`，作为孔位 × 时间点缺漏检查的分组依据（Core 不判断“应有几张”）。字段语义与缺省规则见 [docs/contracts/asset-coordinates-v1.md](docs/contracts/asset-coordinates-v1.md)。

`run.revise` 必须提供 `id` 和当前 `expected_revision`，只接受 `title`、`scheduled_date`；日期可传 `null` 清除。状态仍通过 `run.set_status` 修订。设计版本、Protocol 清单、材料清单、轮次和来源关系属于创建时冻结快照，修订接口拒绝这些字段。HTTP API 与 CLI 共用同一补丁校验；网页不得整体替换 Run JSON。

## 资产坐标（`asset.link` 与 `run.workspace`）

`asset.link` 在 `{run_id, task_key, sample_id?}` 之外接受 `well_id`、`timepoint`、`fov_index`：

- `well_id` 形如 `A1`；`fov_index` 从 `1` 开始；输入 schema 的 `fov_index.minimum = 1`，传 `0` 返回 `INVALID_INPUT`。`timepoint` 原文保存（如 `t6h`）。给出 `timepoint` 或 `fov_index` 时必须同时给出 `well_id`，否则 `INVALID_INPUT`。
- 三者都可省略；省略表示“该链接没有记录坐标”，Core 不按任务名、样本或文件名推断孔位。
- 输出 `record.data.link` 回显 `wellId`、`timepoint`、`fovIndex` 与派生的 `coordinateId`（`A1` / `A1_t6h` / `A1_t6h_fov01`）。坐标不同即不同链接；同一坐标重复关联只刷新原记录并追加 `linked` 事件。
- 旧记录的链接缺这三个字段：回读为 `null`，不补默认值、不迁移。
- `run.workspace` 的 `linked_assets[]` 直接复用同一套字段；`asset_coverage[]` 只聚合带 `well_id` 的链接，按 `well_id`、`timepoint` 升序排列，`fov_indices[]` 与 `asset_ids[]` 去重升序。没有 `well_id` 的链接不进入分组，但仍出现在 `linked_assets[]`。
- MyGo 侧 `runs.AssetLink` 镜像同名字段与 JSON tag；`FOVIndex == 0` 表示未记录。

`asset.link.update` 就地改写一条已存在链接的坐标，不重新读取文件：

- 输入 `{id, well_id, timepoint, fov_index, actor_id?}`。三个坐标键必须出现；`timepoint` / `fov_index` 用显式 `null` 表示清空，省略键返回 `INVALID_INPUT` —— “没传”不等于“清空”。
- 只重写 `link.wellId`、`link.timepoint`、`link.fovIndex` 与重新派生的 `link.coordinateId`；`id`、`rootId`、`relativePath`、`size`、`modTime`、`sha256`、`mediaType`、`availability`、`runId`、`taskKey`、`sampleId`、`linkedAt` 原样保留。
- 记录更新与一条 `asset_link_event` 在同一事务提交：`action = "updated"`，`previous` 为更新前的 `{wellId, timepoint, fovIndex, coordinateId}`。`asset.events` 的 `action` 过滤新增 `updated`。
- 输出 `{record, event}`，与 `asset.link` 同形状。未知或已解绑的记录返回 `NOT_FOUND`（退出码 3）；坐标非法、缺字段或 `id` 不是 `asset_link` 返回 `INVALID_INPUT`（退出码 2）。
- 记录 ID 是创建时的定位句柄，更新坐标不重算。移动坐标用本命令；用 `asset.link` 重新关联同一文件与新坐标会因 ID 由坐标派生而新增一条记录。
- `runs.RunService` 的原生桥接（LB-10）按资产 ID 暴露同一语义，不按记录 ID：
  - `UnlinkAssetLink(ctx, runID, assetID) (runs.Asset, error)`：解除该 Run 上这个资产的链接并追加 `unlinked` 事件，返回被删除链接的原始元数据；文件本身不动。
  - `UpdateAssetLinkCoordinates(ctx, runID, assetID, wellID, timepoint, fovIndex) (runs.Asset, error)`：把链接的坐标三元组就地改成新值，文件身份与记录 ID 不变。`wellID` 必填；`timepoint == ""` 与 `fovIndex == 0` 是 Go 的“未记录”，写入 `null`（Go 没有“键缺失”状态，所以两项始终显式给出）。
  - `assetID` 是 `ListAssetLinks` / `SaveAssetLink` 返回的 `Asset.ID`，在 `runID` 范围内解析。该 Run 没有这个资产的链接（含已解绑）→ `NOT_FOUND`；坐标非法 → `INVALID_INPUT`；同一资产在同一 Run 内有多个坐标链接（只有 CLI 能造出这种状态）→ `INVALID_INPUT`，不猜是哪一条。
  - 错误形态是 Go 字符串：`NOT_FOUND: …` / `INVALID: …` / `INTERNAL: …`，与桥接层其他命令一致。
- 完整规则与回滚说明见 [docs/contracts/asset-coordinates-v1.md](docs/contracts/asset-coordinates-v1.md) §5、§6。

## 读数提交（readout.*）

`readout.submit` 与 `readout.batch` 把读数写进现有 `execution_event` 追加记录；不新增记录类型，也不建第二份读数表。两者都只接受本次 Run 冻结清单内的 `step_path` + `protocol_version_id`（检测读数还需 `assay_id`；共享制备读数用 `planned_output_id`），引用校验复用 `execution_event.create` 的同一套逻辑。

- `readout.submit` 有两种互斥形态。标量形态输入 `{run_id, step_path, endpoint, protocol_version_id, value | qualifier, unit?, sample_id?, timepoint?, assay_id?, planned_output_id?, operator?, occurred_at?, content?}`，必须有且只有一个 `value` 或 `qualifier`（`not_detected` / `below_lod` / `above_uloq` / `invalid` / `other`）。数值同时写进 `measurements[]` 与 `readouts[]`；带 `qualifier` 的只写 `readouts[]`，不伪造成 0。输出 `{record, idempotency_key, replayed}`。
- 多值形态输入 `{run_id, step_path, protocol_version_id, assay_id?, timepoint?, batch_id?, operator?, occurred_at?, readouts[], readout_schemas[]?}`，`readouts[]` 是 v1 `Readout` 数组（`schema_id`、`schema_version`、`value` 为 `numeric`/`table`/`plate`/`categorical`/`file` 之一、`unit?`、`missing?`、`source`、`link`），按 `docs/contracts/readout-schema-v1.md` 的同一流水线（schema 校验 → 读数校验 → 派生来源校验）落盘。`readout_schemas[]` 声明本次提交额外依赖的 schema 定义：`ReadoutRegistry` 会校验定义，同一 `schema_id` 不能给出不同定义；内置 schema（`cck8_od450`、`rna_concentration`、`rna_purity`、`cfu_count`）始终可用。`readout_schemas[]` 只作用于本次提交，不落库，也不改变内置注册表。分类、表格、文件读数在 `result_cells[]` 里按 `{"kind": "record", "readout": …}` 保存，不进 `measurements[]`，也不伪造 `readouts[]` 投影。输出 `{record, idempotency_key, replayed, accepted}`，`accepted` 是本次记录的读数条数。
- `readout.batch` 输入 `{run_id, step_path, protocol_version_id, plate_id, schema_id, schema_version=1, assay_id?, timepoint?, batch_id?, ...}`，读数用 `cells[]`（`row`/`column` 从 0 开始，可选 `field`、`value`、`unit`、`sample_id`、`group`、`replicate`、`status`、`qualifier`）或 `csv` 二选一。`qualifier`（`not_detected` / `below_lod` / `above_uloq` / `invalid` / `other`）必须与 `value` 互斥（显式写 `null` 等同省略）；`cells` 与 `csv` 只能提供一种；JSON `cells[]` 带 `qualifier` 时 CLI 按 CSV 同一路径解析，缺失原因的校验、`status=missing` 写入和 `result_cells[]` 保留方式与 CSV 完全一致。CSV 表头 `row,column,field[,value,unit,sample_id,group,replicate,status,qualifier]`：`value` 可写 qPCR 导出的 `Undetermined`，记为 `not_detected` 而不是非法数值；未知或重复列、缺列、行列越界、同批重复孔位、`status` 与 `value` 矛盾都会拒绝整批。带缺失原因的孔位不写进 `measurements[]`。输出 `{record, idempotency_key, replayed, accepted}`。
- 幂等：标量 `readout.submit` 的键是 `(run_id, step_path, endpoint, sample_id, timepoint)`，多值形态的键是 `(run_id, step_path, assay_id, timepoint, readouts[])` 的规范 JSON 摘要；`readout.batch` 的键是同一组身份字段按孔位（`plate_id, row, column, field`）展开后的集合——同一批里多个孔位可以共享 endpoint/sample/timepoint，只用卡片元组会互相覆盖。`run_id` 属于键，因此不同 Run 的同名读数互不影响。重复提交返回首次记录并置 `replayed=true`；值属于内容不属于身份，故重放不会覆盖已记录值。
- 原子性：整批在一个事务内校验并写入一条 `measurement` 事件；任何一行失败都返回 `INVALID_INPUT` 并附 `issues[]`，数据库不产生部分事件。`readout.batch` 必须能唯一确定本次 Run 的一块孔板，否则返回 `unresolved_plate` / `ambiguous_plate` / `unknown_plate`，不猜孔位。
- 与 `execution_event.create` 的关系：后者仍是多用途底层追加接口（含操作、材料、检查等）；`readout.*` 只覆盖读数，`step_status`、`checks`、`controls` 不通过它写入。

## 计算器（`calculator.*`）与 1D 孔位

计算器只做确定性算术，`writes_data=false`：不保存输入或结果，不产生执行事件，也不替研究者判断配方是否科学适用。所有 `calculator.*` 的输入/输出字段以 `labworkbench schema <命令>` 为准。

- `calculator.dilution`：按 `C1V1 = C2V2` 求储液体积。`stock_unit` / `target_unit` 接受摩尔单位 `M`、`mM`、`µM`、`nM` 或质量浓度 `mg/mL`、`µg/mL`、`ng/µL`；两者必须同一量纲，混用返回 `INVALID_INPUT`。摩尔与质量浓度是不同维度，这里不借分子量互相换算（质量→摩尔是 `calculator.solution` 的职责）。目标浓度高于储液浓度时拒绝。
- `calculator.sample_normalize_batch`：输入 `{target_mass_ng, reaction_volume_ul, samples[]}`，每个样本给出 `name` 与实测质量浓度（`concentration` + `unit`，只接受 `mg/mL`、`µg/mL`、`ng/µL`）。输出每行的 `concentration_ng_per_ul`、`required_sample_volume_ul`、`sample_volume_ul`、`diluent_volume_ul`、`achieved_mass_ng` 与 `insufficient`：样本过稀、所需体积超过反应体积时按反应体积封顶并把该行标为 `insufficient`，不向上取整或静默达标；`insufficient_count` 汇总行数。目标质量不是实测值。
- `calculator.in_vivo_dosing`：输入 `{body_weights_g[], dose_mg_per_kg, dose_volume_ml_per_kg, overage_ratio?, stock_concentration, stock_unit}`。输出每只动物的 `body_weight_g` / `dose_mass_mg` / `injection_volume_ul`，以及 `total_*`、`prepared_dose_mass_mg`、`preparation_volume_ul`、`stock_volume_ul`、`diluent_volume_ul`。`overage_ratio` 只放大配制管，不放大单只注射量；母液过稀导致剂量无法装进制备体积时整体拒绝（`INVALID_INPUT`），不通过放大注射体积掩盖。体重是输入值而非估算。
- `calculator.biofilm_cfu`：只读生物膜 CFU 派生（`calculator.biofilm_cfu@1`）。输入 `{plates[], countable_min, countable_max}`，每板给出 `raw_id`、`sample_id`、`colonies`、逐管稀释分数 `dilution_fractions[]`、涂布体积 + `plated_unit`（L/mL/µL）、悬液体积 + `suspension_unit`（L/mL/µL）。输出 `groups[]`（同样品 + 总稀释度乘积 + 涂布量分组，只平均同组重复）与 `excluded[]`（`below_window` / `above_window`，保留 `raw_id` 与来源总稀释度）。组内 `CFU/mL = mean_colonies ÷ (total_dilution × plated_volume_mL)`，`CFU/sample = CFU/mL × suspension_volume_mL`；每组带 2 条派生读数（`cfu_per_ml` / `CFU/mL`、`cfu_per_sample` / `CFU`，`method = calculator.biofilm_cfu@1`，`derived_from` 列出组内每板 raw ID，`result_kind = derived`）。计数上下限必填且含边界；稀释分数必须在 `(0, 1]` 内，体积必须为正，单位只接受 L/mL/µL，不推断默认值；重复 raw ID、同样品悬液量冲突、非法稀释/体积返回 `INVALID_INPUT`；无可计数板时 `groups` 为空且不估算。计算不写库；保存派生读数时用 `execution_event.create` 附带 `method` 与 `derived_from`。

`well.set` 的位置字段按容器形状解析：2D 板接受 `<行字母><列号>`（`A1`、`AA12`）；`rows = 1` 的单行容器另接受 `Lane<N>`、`L<N>`、`Tube<N>`，并与 `A<N>` 指向同一物理槽位——同一槽位的再次写入被视为覆盖（需要 `expected_revision`），不会产生两个孔位记录。存储地址规范化为 `Lane<N>`（1D）或大写 2D 坐标。可选 `target`（检测目标标签）、`well_role`（`unknown` / `ntc` / `standard` / `positive_control`）和 `dilution_factor`（无单位、必须大于 0）是**计划**注释：它们不写读数，也不参与冻结设计计算。省略这些字段时按 `null` 存储；带 `expected_revision` 的覆盖是整条字段替换，旧注释不会自动保留。

## 储藏室台账（`pantry.*`）与贴标导出（`label.export`）

储藏室是**可选的轻量台账**，不是 Design/Run 的必填依赖。它存在辅助 SQLite 表 `pantry_items`（`id`、`label_code`、`project_id`、`name`、`location`、`purchased_on`、`opened_on`、`shelf_life_days`、`opened_shelf_life_days`、`exhausted`、`revision`、`deleted`、时间戳），不进入 `records`，不参与 Design/Protocol/Run 冻结，也不改变 `PRAGMA user_version`。不建条目时系统行为与之前完全相同。

- `pantry.upsert`：输入 `{id?, project_id?, name, location?, purchased_on?, opened_on?, shelf_life_days?, opened_shelf_life_days?, exhausted?, expected_revision?}`。省略 `id` 创建新条目；提供 `id` 时必须已存在且未停用。这是**整条覆盖**写入：省略的可选字段会被清空，不做字段级合并。`project_id` 提供时必须指向已存在的项目。`expected_revision` 提供时按版本校验，不一致返回 `REVISION_CONFLICT`（退出码 4）；省略则不校验版本。日期必须是真实存在的 `YYYY-MM-DD`；开封日早于购入日、保质期不是 1–3650 天、名称/位置超长都返回 `INVALID_INPUT`（退出码 2）。输出 `{item}`，含 `label_code` 与当前 `status`。
- `pantry.delete`：输入 `{id, expected_revision?}`，把条目标记为已停用（`deleted=1`）并递增 `revision`，输出 `{pantry_item_id, label_code, revision, deleted:true}`。这是**墓碑式删除**，不物理删行：短标签 ID 可能已经贴在实物上，保留它可保证不被重新分配给别的条目；停用后 `pantry.list`、`label.export` 不再返回该条目，`pantry.upsert` 与 `execution_event.create` 把它当作 `NOT_FOUND`（退出码 3），也不可复活。重复停用同样返回 `NOT_FOUND`。
- `pantry.list`：输入 `{project_id?, include_exhausted?, as_of?, near_expiry_days?}`，输出 `{as_of, near_expiry_days, items[]}`。`as_of` 是效期计算所依据的日期（`YYYY-MM-DD`），省略时取当前 UTC 日期；`near_expiry_days` 默认 14，允许 0–3650。`include_exhausted` 默认 `true`，为 `false` 时省略已耗尽条目。只读；不含已停用条目。
- `label.export`：同一输入，输出 `{schema_version:1, generated_at, as_of, near_expiry_days, labels[]}`，每条为 `{label_code, pantry_item_id, name, location, status, expires_on, days_remaining, revision}`，按 `label_code` 排序，便于外部贴标/扫码插件直接消费和比对。只读；不驱动打印机、不生成图形、不写数据库。

效期语义（`status`）：

| 状态 | 判定 |
|---|---|
| `unopened` | 未记录开封，且未进入临期窗口 |
| `opened` | 已记录开封，且未进入临期窗口 |
| `near_expiry` | 距 `expires_on` 0–`near_expiry_days` 天（含当天） |
| `expired` | 已超过 `expires_on` |
| `exhausted` | 已记录耗尽；优先于所有日期规则 |

`expires_on` 与 `expiry_basis` 只从显式字段推导：有 `opened_on + opened_shelf_life_days` 时用开封后窗口（`opened_shelf_life`），否则用 `purchased_on + shelf_life_days`（`shelf_life`）。两者都不全时不编造日期：`expires_on`、`days_remaining`、`expiry_basis` 均为 `null`，状态只反映是否开封/耗尽。

`label_code` 由条目的不可变 `id` 派生（`PN-` + 前 8 位大写字母数字），首次分配后持久保存；同一 `label_code` 已被占用时依次追加 `-2`、`-3` 等后缀。停用（`pantry.delete`）的条目仍占用其短 ID，不会被重新分配。因此重贴标签、重复导出或停用条目都不会改变扫码结果。

`execution_event.create` 可附带可选的 `pantry_item_id`：省略时事件结构不变；提供时必须是已存在的条目，否则返回 `NOT_FOUND`（退出码 3）。成功输出因此固定为 `{record, pantry_notices[]}`；条目处于 `near_expiry` 或 `expired` 时 `pantry_notices` 返回一条 `{code:"pantry_item_expiry_notice", severity:"advisory", …}`，**不阻断写入**；其他情况为空数组。Design 阶段不要求绑定储藏室条目。

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
- 创建运行时固定引用当前 `design_revision`，并生成 `protocol_manifest[]` 固定该版本各检测所引用的 Protocol 精确版本。`run_index` 按设计递增；再次执行可传 `repeat_of_run_id` 与 `repeat_relationship`（`independent`、`same_source`、`paired`、`unknown`），不能仅凭轮次数断言独立性；上游样本来源用 `source_run_id`，可跨同项目的其他 Design。Protocol 修订创建新版本，不覆盖历史版本。
- `asterdoc.validate` 的 `valid` 表示“源码可以产出设计”，不是“YAML 结构没报错”：结构校验通过但设计投影失败（例如缺少唯一的 `experiment` 模块、步骤无法物化）时返回 `valid=false`、`compiled=false` 和结构化 `error`，不会同时给出 `valid: true`。判断能否建 Run 仍以 `asterdoc.compile` / `asterdoc.assess` 的结果为准。
- `note.create` 是追加式自由文本记录。`execution_event.create` 适合需要筛选、计算或追溯的实际事件：输入 `run_id`、`event_type`、`content`，可附 `occurred_at`（带时区 RFC 3339；不确定时留空）、`stage`、`operator`、`sample_ids[]`；`material_use` 事件可写 `materials[]`，`measurement` 事件必须含数值 `measurements[]` 或结构化 `readouts[]`，`deviation` 事件必须说明 `deviation`。声明样本级执行身份时另给 `attempt`（≥1）；它必须与冻结 `protocol_step_path` 和至少一个 `sample_ids` 同时出现，只能是该样本当前最高 attempt 或 `最高 + 1`。同一 `step_path + sample_ids[] + attempt` 的重复完成返回原事件记录，不追加新事实；重试必须使用下一个 attempt。
- `readouts[]` 每项为可选 `readout_id`、`endpoint`、可选 `value` 或 `qualifier`（`not_detected` / `below_lod` / `above_uloq` / `invalid` / `other`），二者必须且只能填一个；`result_kind` 为 `raw` 或 `derived`。派生结果必须提供 `method` 与 `derived_from[]`；`derived_from[]` 引用本次 Run 中已存在的原始 `readout_id`，不能引用未知或派生读数。不得把计算结果写成原始读数。
- `checks[]` 每项为 `check_id`、`status`（`passed` / `failed` / `blocked` / `skipped`）及可选 `checked_by`、`note`；`check_id` 必须存在于同一冻结 Protocol 步骤的 `checks[]`，同一事件不得重复，且检查结果必须关联该步骤。`skipped` 不等于通过；空白或未记录也不等于通过。`controls[]` 记录对照角色 `role`、`status`、样本/实验单位/观察；`lineage[]` 记录 `from_kind`、`from_id`、`to_kind`、`to_id`、`relation`。`actual_parameters[]` 记录实际执行参数 `key`、`value`、可选 `unit` 与 `source`，并与计划参数分开。
- Protocol 文本步骤可定义 `checks[]`：`check_id`、`label`、可选 `instruction` 与 `blocking`。检查项随 ProtocolPreview、Run 冻结清单一同保存；AsterLab 只记录人工结果，不自动判定 gate 通过。
- Protocol 步骤事件可附 `timepoint` 和 `step_status`。`timepoint` 仅用于明确属于某个检测时间点的步骤，并必须匹配 Run 冻结设计；共享制备步骤不应机械绑定某项检测的时间点。`step_status` 只能是 `in_progress`、`completed`、`blocked`、`skipped`、`qc_dropped`、`censored_dead`、`rework`，且必须同时关联冻结 Protocol 步骤；`qc_dropped`、`censored_dead`、`rework` 分别表示样本被 QC 剔除、删失（如中途死亡）和需要重做，`workflow.progress` 按这些状态计数但不把 `qc_dropped`/`censored_dead` 当作步骤完成。普通操作、观察、测量或偏差事件本身不代表步骤完成。未带时间点的旧事件不会被自动分配给 D1/D3。带 `sample_ids` 的事件只改变这些样本的状态；整步完成由 `workflow.progress` 判定，要求全部样本完成。
- `execution_event.create` 的 `event_type` 另接受 `skipped_by_rule`（按规则跳过）；它仍需 `content`，并与其他事件一样经同一套冻结步骤/样本校验写入追加事实。临时补加的步骤必须显式传 `ad_hoc_step: true`（省略即 `false`）：为真时允许 `protocol_step_path` 不属于冻结清单，为假时仍按冻结清单拒绝未知路径；该字段只在输入中出现，回读记录也带同名布尔值，不能用来绕过 `sample_ids`、`attempt` 或检查项校验。
- `execution_event.create` 另接受可选 `parent_sample_id`、`aliquot_from`（文本，≤120 字符）和 `blind_masked`（布尔）：前两者记录这些样本的父样本与分样来源，`blind_masked` 标记该事件在盲法视图下记录。三者省略时事件与旧版本一致；回读记录原样带出同名字段。
- `execution_event.create` 另接受可选 `pantry_item_id`，把事件关联到储藏室条目。省略时事件与旧版本一致；提供时条目必须已存在（否则 `NOT_FOUND`）。该关联不改变任何校验门：条目临期或过期只在输出的 `pantry_notices[]` 里返回 `advisory` 提示，不阻断写入。详见「储藏室台账」一节。
- 写入工具只返回记录 ID、所属关系、版本、数据和创建/更新时间；不声称实际实验已经执行。
- 该 CLI 是本机原型，尚无多用户认证、审计签名、细粒度附件权限或撤销系统。不要把本地访问控制误称为云端安全边界。

## AsterDoc v2 模块与步骤内编排接口

AsterML v2 将实验结构收口为 Markdown + ` ```asterlab ` 通用科学模块与 `step@1.blocks[]` 步骤内自由编排，Core CLI 通过以下工具暴露校验、步骤块编辑、自包含 Run 冻结与结构化表格读数通道：

| 命令 | 用途 | 写入 |
|---|---|---:|
| `sdk.modules` / `sdk.describe` | 列出或按 `kind`/`schema` 查询 AsterML v2 通用模块规格、输入端口与能力 | 否 |
| `sdk.validate` / `asterdoc.validate` | 校验 AsterML 源码并尝试投影为 Design（返回 `valid`、`compiled`、`errors`/`warnings`） | 否 |
| `asterdoc.load` / `asterdoc.save` | 读取或乐观锁追加保存 Design 的 AsterML 源码修订 | save 创建修订 |
| `asterdoc.inspect` | 解析并返回文档模块、清单与全局资源预算 `budget`；传入 `module_id` 时同步搜索顶层模块及嵌套 `step.blocks[]` 子块 | 否 |
| `asterdoc.assess` / `asterdoc.compile` | 评估是否可执行（`ready`/`warning`/`incomplete`）并编译为 Design JSON | 否 |
| `asterdoc.command` | 对 AsterDoc 执行结构化增量命令，生成新修订 | 创建修订 |
| `asterdoc.create_run` | 从指定 AsterDoc revision 创建并冻结 Run；未绑定外部 `ProtocolBlockVersion` 的自包含步骤自动物化冻结 | create |

### `asterdoc.command` 支持的操作类型

- 顶层模块与流程：`add_module`、`add_step`、`update_module`、`remove_module`、`remove_step`、`reorder_steps`、`nest_step`、`promote_step`、`connect_steps`、`disconnect_steps`。
- 容器孔位分派与多维标注：`assign_wells`、`clear_wells`（同步清理 `groups`、`samples`、`replicates`、`roles` 与 `factors`）；`plate@1` 支持通用正交实验因子映射 `factors: {<维度名>: {<孔位>: <因子水平>}}`，技术复孔唯一性键按 `(sample_id, canonical_factor_signature, replicate)` 校验，允许同一样本在多靶点基因/多处理水平下拥有独立 `#1..#k` 复孔序列。
- **步骤内模块自由编排**（作用目标 `stepId` 或 `moduleId` 指向目标 `step@1`）：
  - `insert_step_block`：传入 `block`（`{kind, id, schema?, ...}`）及可选插入位置 `index`（省略追加末尾）。
  - `update_step_block`：传入 `blockId` 与 `properties` 增量更新步骤子块字段（禁止覆盖 `id`/`kind`/`schema`）。
  - `move_step_block`：传入 `blockId`，支持按 `toIndex`（`>= 0` 拖拽精准落位）或 `direction`（`"up"` / `"down"`）重排步骤内模块顺序。
  - `remove_step_block`：传入 `blockId` 从步骤 `blocks[]` 中移除对应模块。

### `asterdoc.inspect` 预算结构（`budget`）

返回 `{steps, executable_steps, groups, wells, hands_on_sec, total_span_sec, has_open_phase}`，与桌面端 `ComputeBudgetSummary` 对齐统计：`steps`/`executable_steps`（步骤总数与可执行步骤数）、`groups`（`group`/`group_table`/`plate.groups` 去重实验组数）、`wells`（`plate` 已分配孔/皿位数）、`hands_on_sec` 与 `total_span_sec`（`timer` 四种模式秒级累计，含 `window` 偏置与周期轮次）及 `has_open_phase`（是否含开放式计时阶段）。

### 多值型表格读数（`readout.submit` `ReadoutValue::Table`）

- 结构化表格模块（`data_table@1`）通过 `TypedReadoutSubmit` 的 `readouts[]` + `readout_schemas[]` 提交 `value.kind = "table"`：
  - `readout_schemas[]` 与 `readouts[]` 均采用 `deny_unknown_fields` 严格校验。
  - `table.columns` 必须按 `schema.table.columns` 声明顺序对齐；`numeric` 列仅接受数值或 `null`，`categorical` 列必须为非空字符串或 `null`（即便是 `"0.5"`、`"10"` 等纯数字形态分类标签也必须保持 JSON 字符串类型）。
  - Core 内置标量 schema ID（`cck8_od450`、`rna_concentration`、`rna_purity`、`cfu_count`）禁止以不同 `value_kind` 或列定义覆盖；若 `data_table` 复用上述保留 ID，调用方应映射独立表格 schema ID（例如附加 `.table` 后缀）。

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
