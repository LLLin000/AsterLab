# V2 UI 后端就绪审计

- 日期：2026-11-15
- 基线：`origin/main` @ `c55ae66`（LB-00 合并后）
- 范围：`UI-INT-10`、`UI-DES-10`、`UI-RUN-10`、`UI-REC-10`、`UI-PAN-10`、`LB-04`、`LB-05`
- 方法：读取上述卡片范围与 `docs/sdk/huvec-tube-formation-case/UI-REDESIGN.md`，逐项对照 Core、CLI、native bridge（`prototypes/mygo-run/modules/runs/runs.go`）的已实现能力。只读审计，不改代码。

## 结论

V2 相关后端交付（EXP-01…07、LB-00）与设计基线（DOC-06）都已合并，**不需要新开后端阶段**。但 UI 波次开工前有 3 个 native bridge 缺口，会直接卡住 `UI-INT-10`、`UI-DES-10`、`UI-RUN-10` 的关键体验；另有 1 处重复实现需要收口。

## 就绪矩阵

| 能力 | 使用卡 | 已实现位置 | 状态 |
|---|---|---|---|
| 单位子集（`%`、`cells/mL`、`cells/well`、`rpm`） | DES/RUN | `prototypes/mygo-run/internal/asterdoc/spec/units.json`、`crates/asterlab-core/src/operation_spec.rs` | 就绪 |
| 循环/条件/时间预算模型 | DES/RUN | `docs/contracts/operation-spec-v2.md` §optional fields | 就绪（预算投影未接线，见 G1） |
| 板型几何 6/12/24/48/96/384 | DES/RUN | `internal/asterdoc/geometry.go`、`docs/contracts/readout-schema-v1.md` §PlateFormat | 就绪 |
| GelMA/细胞配液计算 | DES | `crates/asterlab-core/src/calculators.rs`、`internal/asterdoc/modules.go::plateMixPlansFor` | 双重实现，见 G4 |
| 持久化 Timer（单 Run） | RUN | `runs.go` `SaveTimer` / `UpdateTimer` / `ListTimers` | 就绪 |
| 资产坐标与缺漏分组 | RUN/LB-04 | `docs/contracts/asset-coordinates-v1.md`、`run.workspace.linked_assets/asset_coverage` | 就绪 |
| RecordProjection（matrix / 动态引用 / 盲法） | REC | `crates/asterlab-core/src/record_projection.rs`、`runs.go RecordProjection` | 就绪 |
| 派生 Design / 签核归档 | REC | `DeriveDesignFromRun`、`SignAndArchiveRun` | 就绪 |
| 储藏室台账与贴标导出 | PAN | `pantry.list/upsert/delete`、`label.export`（EXP-06） | 就绪 |
| 设计树命令、冻结建 Run | DES | `ApplyAsterDocCommand`、`CreateRunFromAsterDoc`、`AssessAsterDoc` | 就绪 |
| 跨页全局 Timer / 多实验并行时钟 | INT | 仅有按 Run 查询的 `ListTimers(runID)` | **缺口 G2** |
| 条件跳过 `skipped_by_rule` / 偏差事件 | RUN | CLI `execution_event.create` 支持，native bridge 无入口 | **缺口 G3** |
| 设计时间预算（含循环/并行空窗） | DES | Core `protocol_preview::project_time_budget` 未暴露；Go 端自行求和 | **缺口 G1** |

## 缺口

### G1 — 预算投影未接线，Go 端求和会算错循环与并行空窗

`crates/asterlab-core/src/protocol_preview.rs:104` 已有 `project_time_budget`，能按 `timing.kind`、`repeat.count`、`schedule` 计算 `total_span_s` / `hands_on_s` / `parallel_windows`；但除测试外没有任何调用点，native bridge 也没有对应方法。

当前 Design 预算条在 `prototypes/mygo-run/internal/designer/step_list.go::designBudget` 里只把卡片上已记录的时长相加，不读取 `OperationSpec` 的 `repeat` / `schedule` / `timing.kind`，因此与 Core 的预算语义不一致；对 `(5m + 10m) × 3`、`wait ≥ 2h`、`t0+6/12/24h` 这类设计，总跨度与占手时间会偏小。

建议：native bridge 增加 `ProjectTimeBudget(input)`（或让 `CompileAsterDoc` 返回预算），Go 预算条只做展示。

### G2 — 只有单 Run 的 Timer 查询，跨页/多实验时钟无法组数据

`runs.go` 的 Timer 接口是 `SaveTimer`、`UpdateTimer`、`ListTimers(ctx, runID)`；`RunSummary` 只有 `runId/title/status/designId/updatedAt`，没有活跃 Timer 或当前步骤字段。`UI-INT-10` 要求“常驻跨页全局 Timer 顶条”，`UI-RUN-10` 的 27 寸布局要求“多实验并行时钟”，需要 N 次 `ListTimers` 才能拼出来，且无法只取运行中的计时器。

建议：增加 `ListActiveTimers()`（跨 Run，返回 `runId/title/status/remaining`）或扩展 `RunSummary.timers`，供 shell 顶条一次读取。

### G3 — native bridge 没有通用事件追加，条件跳过写不进去

Core/CLI 已支持 `execution_event.create`，`event_type` 明确接受 `skipped_by_rule`（`AGENT_API.md` §“执行事件”；`src/cli.rs:4300`），`CompleteTaskInput` 却没有 `step_status`/`skip` 字段，`runs.go` 也没有 `CreateExecutionEvent` 方法。`UI-RUN-10` 的“执行并记参数 / 无气泡按规则跳过”第二半无法通过现有 bridge 落库；偏差随手记同样受限。

建议：bridge 增加 `CreateExecutionEvent(input)`，至少覆盖 `skipped_by_rule`、`deviation`、`observation` 与 `pantry_item_id` 关联。

### G4 — 配液计算在 Core 与 Go 各有一份（风险，非阻塞）

Core `calculators.rs` 有 GelMA 主混液/细胞接种计算，Go `internal/asterdoc/modules.go::plateMixPlansFor` 又从 plate 模块字段重算一遍。目前测试分别通过，但公式、余量与取整规则一旦只改一边就会在 UI 与 Core 之间出现数值分歧。

建议：Beta 前明确唯一权威（倾向 Core），Go 只消费结果；若暂不迁移，至少在两边加一组相同输入的对照测试。

## 建议顺序

1. 三个 bridge 卡（G1/G2/G3）在 VPS 完成，作为 V2 UI 波次的前置；都属于 `prototypes/mygo-run/modules/runs` + Rust 实现，可用 `mygo native check` 与 `--regen` 验证。
2. `G4` 作为技术债卡，不阻塞 UI 开工。
3. Windows Gate 再按 `UI-INT-10` → 四个工作面 → `QA-HUVEC-01` → `RC-*` 推进；`LB-01..03` 已实现待验，`LB-04/05` 作为 RUN/DES 的切片执行。

本审计不改变 `beta-scope.md` 的平台与 DoD，也不提议新的里程碑。
