# Time Plan Closure Contract v1（CLOSE-03 收口）

- 日期：2026-11-17
- 状态：生产权威；与实现同步落地。
- 基线：CLOSE-03 分支；`OperationStep.operation_spec` / `protocol.time_budget` / Freeze Timer phases 为生产权威。
- 权威关系：术语见 [`CONTEXT.md`](../../CONTEXT.md)，架构见 [`architecture.md`](architecture.md)，时间语义见 [`operation-spec-v2.md`](operation-spec-v2.md)。

## 1. 目标

一个可编辑计划贯通 Core 预算、Freeze 与 Run 阶段计时：

- 设计侧以 `ProtocolBlock` 步骤的 `operation_spec` v2 sidecar 为唯一时间模型；不再以空 `phases` 或 Go 估算冒充闭环。
- Core 预算只读持久化 sidecar：循环次数、持续时间、锚点/并行空窗全部来自存储，不从标题推断。
- Freeze Timer phases 只存游标（`phase_index`/`round_index`），计划由调用方供给；计时事实只经 `execution_event.create` 落库，纯事实依赖持久化检查点。
- 同一冷热交替 5min/10min×3 计划的预算与阶段序列一致；t0+6/12/24h 计划与重启恢复一致。

## 2. 编辑：`operation_spec` 是步骤的一部分

`ProtocolStep::Text.operation_spec`（`Option<OperationSpec>`）是持久化 sidecar：

- CLI `protocol.block.create` / `protocol.block.revise` / `step.edit`（insert/reuse 的 `step`）直接接受 `operation_spec` JSON；`validate_block_content` 按 `operation-spec-v2.md` 三类语义码校验（`unit_mismatch` / `missing_parameter` / `value_out_of_range`），`schema_version` 仅接受 1/2。
- CLI input schema 的 `text_step_input_schema` 显式声明 `operation_spec: object`（sidecar，不再是隐藏字段）。
- `block_preview` / `protocol.version.get` / `protocol_version_content` 展开时原样携带 `operation_spec`；缺 sidecar 的旧步骤不补默认值，预算计为无时长。
- 修改设计不影响旧 Run：新版本追加 `protocol_block_version`，冻结 Run 快照不变。

## 3. 读：`protocol.time_budget`

CLI：`labworkbench protocol.time_budget --input {"block_version_id"}`。
Bridge：`QueryProjection("protocol.time_budget", {"block_version_id"})`（AppCore 通道；Go 不重算）。
Go：复用已有 `TimeBudgetService.ProjectTimeBudget`（纯函数，输入为 OperationSpec 数组 JSON）。

输出即 Core 串行投影：

```json
{
  "block_version_id": "...",
  "step_count": 3,
  "steps_with_timing": 2,
  "steps_without_timing": 1,
  "total_span_s": 129610.0,
  "hands_on_s": 30.0,
  "parallel_windows": [
    {"step_index": 0, "start_s": 21600.0, "end_s": 28800.0, "duration_s": 7200.0, "time_anchor_id": "t0"}
  ]
}
```

规则（与 `project_time_budget` 同一函数，同一权威）：

- 无 `timing` 的步骤不加时长（`steps_without_timing` 计数，不静默补）。
- `repeat.count` 乘每个 occurrence 时长；`schedule.offset` 只延迟首次 occurrence；`interval` 是 occurrence 之间的 gap。
- 每个 `wait` / `timed_endpoint` occurrence 独立成窗，携带 `time_anchor_id`；`hands_on` 计入 `hands_on_s`。
- 均匀 `interval` 不能表达非均匀序列：t0+6h/12h/24h 用三个 `occurrences: 1` 的 anchored rule（各自 offset 6h/12h/24h）表达。
- 串行投影不解析跨步骤锚点：后步骤的 offset 累加在先前 span 之后（`129600 = 86400 + 43200`）。

## 4. Freeze Timer phases：计划在外，游标在内

`TimerSequence`（`TimerPhase{name, planned_s, mode, rounds}`）是调用方供给的计划；`TimerRecord` 只存游标（`phase_index`/`round_index`）与当前轮累积：

- `TimerSequence::new` 拒绝空名、`rounds: 0`、countdown 缺 `planned_s`。
- `advance_countdown(&sequence, now_utc)`：未到计划时长返回 `None`；完成一轮返回 `TimerRound`（`phase_index`/`round_index`/`phase_name`/`planned_s`/`actual_s`/起止锚点/`phase_completed`/`timer_completed`），并把记录推进到下一轮/下一阶段（同步 `mode`/`planned_s`）。
- 旧单阶段 timer 传空 phases 即 `TimerSequence::from_record`，行为与序列前完全一致。
- 持久化：`timer_records(timer_id, run_id, task_key, data_json)`；`AppCore::save_timer_record` 要求 run 存在，`load_timer_records` 按 task_key/timer_id 排序；重启后同一行恢复，`t0` 锚点触发（`trigger_timers_for_anchor_tx` 与事件同一事务）不变。
- 每一轮完成事实由调用方经 `CreateExecutionEvent`（`execution_event.create`）追加；`TimerRound` 本身不是存储表，轮历史从 Run 事件读回。

冷热交替 5min×3 + 10min×3：三轮 cold 后 `phase_completed=true` 进入 hot 阶段（`mode`/`planned_s` 同步为 hot），游标为 `(1, 0)`。

## 5. 闭环证据（同一 HEAD）

- 冷热交替预算一致：`TestTimeBudgetMatchesPersistedSpecsAcrossColdHotAlternation`（Go）与 Core `project_time_budget` 同一算法；`(5m hands_on + 10m wait)×3 → span 2700s / hands-on 900s / 单窗 [900,2700]`。
- 持久化 sidecar 闭环：`time_budget_projects_persisted_specs_without_title_guessing`（CLI）：block 写入 → preview 携带 sidecar → `protocol.time_budget` 投影；t0+6/12/24h 三窗锚点来自存储 offset，不读标题。
- 阶段计时闭环：`freeze_timer_phases_persist_and_advance_through_the_supplied_plan`（App）：save → reopen 恢复游标 → advance 走完 cold 三轮 → 进入 hot 阶段。
- 重启恢复：`timer_records_persist_across_app_reopen_and_t0_triggers_window`（既有）+ 本卡 phases 测试同目录。

## 6. 兼容与禁止

- 旧 v1 sidecar 可读；未知字段与不支持的 `schema_version` 拒绝，不静默迁移冻结 Run。
- Go `internal/designer` 预算条只做展示：`Core` 投影非空时采用它，`设计标注` 估算保留来源标签，不与 Core 值合并求和。
- 计时永不推断：缺 `started_at_utc` 的 running timer 报错；`now_utc` 缺省不上报已耗时间；暂停累积只来自持久化行。
- 修改设计（新 block version）不改已冻结 Run 的 manifest/预算/计时行。
