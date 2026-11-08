# AsterLab — Recording Contract

- 状态：**Native UI Direction Freeze / 2026-11-08**
- 本文定义“执行与记录”的 UI 行为和工程边界；不重复 [CONTEXT.md](CONTEXT.md) 的领域字段。
- 当前可调用 CLI/HTTP 字段以 [AGENT_API.md](AGENT_API.md) 为准。两者冲突时，领域语义以 CONTEXT 为准，实际字段以 AGENT_API 为准。

## Authority

```text
AsterLab Core   领域对象、校验、版本、事务、当前任务状态
AsterLab App    用例命令、ViewModel、行为与状态投影
Rune UI         未来正式前端：只呈现状态、收集输入、发起命令
Web UI          迁移期可写对照与回退
```

UI 不得自行判断：

- 阻断检查是否通过；
- 一个 Task 是否已完成；
- 一个 Run 是否可关闭；
- 一个测量值属于哪个 occurrence；
- 一个样本/孔位/时间点之间的关系。

这些判定必须在 Rust 应用层执行，并返回可渲染的 view model。

## Event model

一条 Run 至少区分两类事件：

| 事件 | 触发 | 内容 | 不包含 |
|---|---|---|---|
| `complete_task` / operation | 用户确认本步完成 | step、scope、timepoint、task_key、检查结果、偏差、操作者（按需） | 不自动生成读数 |
| `measurement` | 用户保存本步读数 | step、scope、timepoint、task_key、`measurements[]`、来源 | 不自动完成任务 |

`complete_task` 与 `measurement` 可以先后发生，也可以只发生一个。结果页必须如实反映。

## Measurement rules

- 字段来自 Assay / Protocol 的结构化 schema；没有定义就不生成字段。
- 默认只显示与本次操作有关的字段；不展示“仪器百科”。
- 数值、单位、样本/孔位分别保存；单位不从名称猜。
- 计划参数、计划布局、计划样本不自动变成实际值。
- 允许多行；每行必须能追溯到明确的样本/孔位或 `Unresolved`。
- 无法确认的关系保持 `Unresolved`；不要用空字符串、0、最近一次同名步骤代替。
- 记录失败保留用户输入；不得静默丢弃或部分提交。

## Non-measurement steps

非测量步骤默认是**轻量确认**：

- 显示本步说明、作用域、关键参数、检查项。
- 可选记录：检查结果、偏差/实际变化、操作人。
- 不强制逐步填写重复的 SOP 参数、备注或操作人。
- 没有测量 schema 时不显示空表格；用一句“此步骤没有定义读数”说明。

## Completion rules

`complete_task` 必须在 Rust 应用层完成：

1. 解析 Run、冻结 DesignVersion、ProtocolVersion、步骤路径、scope、timepoint、task_key。
2. 校验步骤存在且属于该 Run 的冻结 manifest。
3. 校验阻断检查；失败则不提交，返回可读原因。
4. 校验幂等性；同一请求重试不产生重复完成事件。
5. 在事务中写入事件并更新 Run/Task 投影。
6. 返回更新后的 `RunWorkspace` 或最小状态增量。

UI 只发意图，不复制上述逻辑。

## Shared read contract

Run 页和 Results 必须消费同一个结果契约：

```text
RunWorkspace
├─ frozen design / protocol identity
├─ steps[]           定义
├─ tasks[]           PlannedTask + status
├─ occurrences[]     StepOccurrence（仅实际开始）
├─ events[]          operation / measurement / checks / deviation
├─ measurements[]    测量（或从 events 统一投影，二选一，不能两份不一致）
├─ plates[]          实际孔板
└─ unresolved[]      明确未知项
```

禁止：

- Run 页从事件正文猜当前任务，Results 从另一套字段猜状态。
- 在 UI 内重新计算 `completed`、`measurements`、`task status`。
- 迁移时保留两套投影逻辑。

### Known gap to close before native migration

以下问题必须用测试固定，不能复制进 Rune：

- `measurements[]` 写入后是否在 `RunWorkspace` 投影中完整出现；
- Run 当前任务与 Results 已完成任务是否使用同一状态源；
- 旧事件缺少 `occurrence_id` 时是否明确标为“未精确绑定执行实例”；
- 测量事件与完成事件的 timepoint/task_key/scope 是否一致；
- 数据库重启后事件、投影和任务状态是否一致。

在解决前，Rune 只允许对**冻结 fixture**做只读原型，不允许宣称完成闭环。

## Persistence and compatibility

- 迁移不得改变现有科学数据的判定方式。
- 现有 SQLite 数据必须可读；必要时提供兼容读取，不做破坏性迁移。
- Run 冻结引用不得因 UI 迁移被重写。
- Web 与 Rune 写同一数据库时，命令层必须事务化；不并发修改同一 Run 的同一任务。
- 失败回滚；不留下半个测量或半个完成状态。

## Recording UI acceptance

1. 从冻结 Run 进入当前任务，看到步骤、检查、适用范围。
2. 完成本步：只有通过校验才成功；失败原因可见。
3. 录入 2 行 CCK-8 读数：保存、刷新、重启后仍能看到。
4. 只完成、不测量：Results 明确显示“已完成但无读数”。
5. 只测量、不完成：任务仍未完成，读数已存在。
6. 无测量 schema 的步骤：不出现空表格或假字段。
7. Web 与 Rune 对同一冻结 Run 的显示、状态、持久化结果一致。
8. `?page=records` 不产生第二个记录入口。

## Out of scope

- 不在本契约中新增科学对象。
- 不定义仪器通信协议。
- 不把 Protocol 步骤正文复制进 Run 实际事件。
- 不要求每个步骤都有操作人、备注或读数。
