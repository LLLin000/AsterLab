# UI-RUN-10 证据：实验运行台面（计时器与 12 孔×6 视野采图）

## 布局决策（UX §5 先行结论）

- 候选 A：台面置顶（计时 → 采图 → 跳过/偏差），记录面板随后。
- 候选 B：新增第 4 个 Tab「计时」。
- 候选 C：只放右侧属性栏（≥1100）。
- **采用 A**：计时开始是计时型步骤的主动作（UX-P1），900×600 首屏必须可见；
  Tab 会把主动作藏到第二屏，右侧栏在窄屏根本不存在。代价是记录面板下移
  一屏，台面保持紧凑（三段共约 3 行标题 + 内容）以控制挤压。

## 与验收项的对应

1. **计时器**：`renderTimerSection` 只渲染 `ListTimers` 检查点
   （`accumulated_s + (now - started_at_utc)`，与 Core `elapsed_s` 同式；
   暂停取存储值；无锚点运行显示「时钟锚点缺失」而不编造）。
   开始/暂停/推进经 `UpdateTimer`（调用方时钟），创建经 `SaveTimer`。
   接力 = 同一计时器的 `round_count` 轮次推进；推进完成返回的 round 事实
   经 `CreateExecutionEvent`（observation）落库，Core 的 round 历史即事件。
   多阶段序列：`TimerPhases` 回调已预留，当前传空（= 计时器自带单阶段，
   Core `TimerSequence.from_record` 语义）。**不支持多阶段 plan 的自动推进**，
   原因见下。
2. **条件跳过/偏差/观察**：`SkipEventInput` 只调 `CreateExecutionEvent`
  （`skipped_by_rule`，Core 自置 `step_status=skipped`），永不调 `CompleteTask`；
   已跳过步骤映射 `TaskStatusSkipped`（显示「已跳过」），`canMarkComplete`
   拒绝。偏差/观察同入口，偏差必填说明，观察可挂 pantry 台账行。
3. **12 孔×6 视野批量关联**：`CoverageByWell` 与 Core `asset_coverage`
   同规则分组（无 well 不成组，FOV 0 不计），与冻结论 `per_well_fov_count`
   求差集得缺口；`PlanBatchDrops` 按缺口顺序填槽，满 6 张跳下一未满孔，
   空槽不足时只返回能装下的部分。每个文件经既有 `asset.link`
  （`well_id/timepoint/fov_index`）持久化。
4. **Pantry/设盲/900×600**：耗材为可选引用（`ListPantry` 失败不阻塞提交；
   Core 的 near-expiry/expired advisory 显示在成功行）。
   设盲为纯展示遮罩（图例、孔点、选中孔分组名），不碰 Record 盲投影。
   台面三段在 900×600 有渲染测试覆盖文本可达（`TestBenchRendersAtNarrowWidth`）。
5. **Run 历史审计（UI-REC-10 同源）**：历史 Tab 改读
   `internal/notebook/record.Project(snap, assets)` 的 `Summary`（计划-vs-实际行）
   与 `Matrix`（采图对账），替代各自推导；条目级时间线仍归 `notebook.Build`
   领域（本 Tab 的事件列表原样保留）。投影按 Run 修订＋资产数记忆化，
   台账输入只是快照与资产表的廉价交接。设盲同样遮罩矩阵分组名。

## Core 契约核实（已读实现，非推测）

- `SaveTimer` 是 upsert（`ON CONFLICT(timer_id) DO UPDATE`，
  `crates/asterlab-app/src/lib.rs::save_timer_record`）：面板拒绝覆盖同名 id，
  避免静默重置累计进度。
- 计时写不进 `revision`（独立 `timer_records` 表）：计时动作后不重载快照，
  只刷新 `ListTimers` 缓存；只有事件追加走 `applySnap`。
- 通用 `execution_event.create` 桥接不触发 t0 锚定（锚定只发生在
  readout/complete 路径，`trigger_timers_for_anchor_tx` 调用点为证）：
  t0 窗口显示锚定状态，不假设跳过事件会起表。
- 多阶段推进需要调用方提供 `phases`（`update_timer_impl` 的 `advance` 分支）；
  冻结论 OperationSpec 计时语义未暴露给 Go（缺口 G1），故本卡只做
  `round_count` 接力，不编造阶段计划。

## 回归测试

`go test ./internal/runworkspace ./internal/runui ./internal/resultgrid -count=1`
全部通过。新增：`timers_test.go`（检查点数学/超限/时钟缺失/窗口 t0/
重复 id 拒绝/完成轮次落库/跳过永不完成）、`capture_test.go`
（Core 同规则分组/跨孔接力/溢出截断/冻结论 FOV 读取/跳过与偏差载荷作用域）、
`bench_test.go`（900×600 台面文本可达/跳过提交流/设盲开关）、
`plate_blind_test.go`（12 孔标签/盲态图例与选中孔遮罩）。

## 待人工验证（Windows Gate，协调人执行）

- [ ] 1360×880 与 900×600 真机截图（浅色/深色）：台面三段无溢出，
      主动作（开始计时/保存读数）在 900×600 首屏。
- [ ] 运行中倒计时约 5 秒刷新一次（随全局时钟条心跳），暂停后静止。
- [ ] 12 孔×6 视野批量拖入：前 6 张进 A1，第 7 张自动进 A2；传满显示 n/n。
- [ ] 按规则跳过后步骤显示「已跳过」且无完成按钮；时间线显示「步骤已跳过」。
- [ ] 设盲视图下图例/选中孔不出现分组名；关闭后恢复。
- [ ] 近效期耗材挂事件后，成功行出现耗材提示。
