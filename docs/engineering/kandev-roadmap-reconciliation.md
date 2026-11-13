# Kandev 路线图对齐记录

日期：2026-11-13。基准：`main` 包含 `320d5a0`（AGENTS.md 与 Kandev 模板）。

本文件记录「G1 → AsterLab v0.1.0 Beta」路线图在填入 Kandev 时解决的矛盾。任务卡本身的分类、范围、验收和依赖以 Kandev 为准。

## 1. 卡片数量

原始设计写「约 32 张主要开发卡」，但逐项列出了 40 张。Kandev 原有多出两张 probe 测试卡，已归档。

为隔离 Core 与 UI 的所有权，`RUN-03` 和 `RUN-04` 各拆成两张：

| 原卡 | 拆分后 | 泳道 |
|---|---|---|
| `RUN-03 统一记录内核` | `RUN-03a (Core) 事实表与查询` + `RUN-03b (UI) Result Grid` | VPS + Windows |
| `RUN-04 事件历史联动` | `RUN-04a (Core) 事件与时间线查询` + `RUN-04b (UI) 事件时间线` | VPS + Windows |

最终 42 张活跃卡。里程碑和依赖仍按 M0–M7 组织，不为每个 Milestone 建独立看板。

## 2. 已解决的矛盾

| 矛盾 | 处理 |
|---|---|
| `RUN-03` 是 VPS 卡，却包含 `resultgrid` GUI，并依赖 Windows 卡 `RUN-02` | 拆成 `RUN-03a`（Core）与 `RUN-03b`（UI）。`RUN-03a` 不再依赖 `RUN-02`，VPS 可独立推进 |
| `RUN-04` 是 Windows 卡，却要求改 Core 事件 | 拆成 `RUN-04a`（Core，VPS）与 `RUN-04b`（UI，Windows） |
| `DES-03` 是 VPS 卡，却包含 MyGo UI 接线 | 改为 `(Core)`，只做校验与序列化；UI 接线不在本卡范围 |
| `DES-02` 是 Windows 卡，却要求改 Core 版本化 | 保持 `(UI)`，明确「Core 版本化 API 必须已存在；缺失则停止并开 Core 卡」 |
| `QA-05` 是 VPS 卡，却要求「UI 入口」 | 改为 `(QA)`，只验 Core/CLI 备份恢复；UI 入口另开 Windows 卡 |
| `REL-05` 是 VPS 卡，却引用宽泛的 MyGo internal | 范围收窄到 `scripts/` 与 `prototypes/mygo-run/modules/runs/rust/**`；跨平台验收由 CI/Windows Gate 提供 |
| `REL-02` 原设计放 Night Auto | 暂留 `VPS Dev`。Night Auto 未启用，且无人值守队列尚未完成非 root 隔离 |
| `REL-04` 原设计写「VPS Dev + Windows Gate」 | 主体在 `VPS Dev`；Windows runner 的实际验收归 `RC-04` / Windows Gate |
| `RUN-04a` 原卡在 Windows 工作流，分类改为 Core 后泳道不一致 | 已移到 `VPS Dev`，标签改为 `lane:vps` |
| 所有卡正文重复 lane/milestone/依赖 | 新模板不再重复；结构化字段和 `blocked_by` 是唯一来源 |

## 3. Review 与合并门

- `Review` 现在是 AI review agent，复用实现 profile，进入时切换角色；不是独立模型。
- 所有分类在门槛通过且 Review agent 判断符合设计时，由 Review agent 自己 squash-merge；拿不准时写 `VERDICT: APPROVED` 留在 Review 等人工。
- 门槛失败时写 `VERDICT: CHANGES`，在同一 session 内就地返工，不移卡；同一张卡累计 3 次 CHANGES 后熔断，留 Review 等人工。
- 门槛无法通过但原因在卡片本身（scope 缺路径、验收不可执行）时写 `VERDICT: BLOCKED-NEEDS-HUMAN`，feeder 推送 Telegram。
- 结论消息的第一行必须是上述三种 `VERDICT:` 之一，机器只读第一行做流转。
- `Review` WIP=4。Review 满 4 个时，feeder 停止把 Inbox 卡提升到 Ready，也停止启动 Ready 卡；这是防止未审查工作堆积的背压。
- `Verify` 与 `Review` 都监听 `on_pr_merged`，PR 合并后直接进入 `Merged`；feeder 另有兜底扫描，PR 已合但卡未动的会被移入 `Merged`。依赖卡只在 `Merged` 后解除阻塞。
- 评审与合并细则见 [`kandev-review-policy.md`](kandev-review-policy.md)。

## 4. 暂不解决的已知风险

1. **VPS agent 仍以 root 运行。** 当前 Kandev service 与 Local executor 都是 root。正式无人值守队列前必须建立受限用户、隔离构建缓存，并验证 agent 不能写工作区外路径。
2. **Windows Gate 没有 executor。** 25 张 Windows 卡仍是人工/待接入 executor；VPS 不能代替 GUI 验收。
3. **Night Auto 未配置。** 工作流存在，但没有 agent profile、自动触发或受限用户；不要把它当成已启用的队列。
4. **scope-check 依赖 PR 正文 marker。** GitHub secrets `KANDEV_API_URL` / `KANDEV_API_TOKEN` 已配置：PR 正文带 `<!-- kandev-task: <uuid> -->` 时由 API 读卡片描述，缺 marker 时回退到 PR 正文的 scope 块。Agent 建 PR 时必须在正文附 marker 与 scope 块。

## 5. 并行策略

- `VPS Dev` 的 `Agent` 列 WIP=4，原生 `pull_from` 已关闭。
- `scripts/kandev-ready-feeder.py` 负责依赖感知的队列：只把无阻塞的 `lane:vps` 卡从 `Inbox` 提升到 `Ready`，并在 `Agent` 有空位时移入 `Agent`。
- 原因：实测 Kandev 原生 `pull_from=Ready` 不会跳过 blocked 卡，曾把 `REL-02` 直接启动。`Ready` 的手动拖动已关闭，避免同样问题。
- 并行只允许在文件范围不重叠的卡之间进行；一张卡一个 Worktree。
- Workspace default executor 已设为 `exec-worktree`。实测 `Local` executor 会让所有任务共用同一个共享 checkout（`/var/lib/kandev/repos/workspaces/.../AsterLab`），不适合并行；Worktree 会为每张卡创建独立路径和分支。
- Core 轨道：`REL-03` → `REL-04` → `AG-01` → `AG-02` → `AG-03` → `RUN-03a` → `RUN-04a` → `QA-04` → `QA-05` → `QA-06`。
- UI 轨道：`RUN-01` → `RUN-02` → `RUN-03b` → `RUN-04b` → `RUN-05` → `RUN-06` → `UX-*`。
- 两条轨道通过 `blocked_by` 在 `RUN-03b`、`RUN-04b`、`QA-02`、`UX-01` 等节点汇合。

## 6. V2 阶段（Scientific Workflow Foundation v2）

日期 2026-11-13。目标：从「通用软件框架」进入「能理解实验、组织实验、管理实验记录的科研工作台」。三个工作包共 13 张卡，标签 `V2`。

| 包 | 卡 | 目的 |
|---|---|---|
| P0 科学语义与契约 | `SCI-01`(Audit) 字段归属盘点、`SCI-02`(Contract) OperationSpec v1、`SCI-03`(Contract) ReadoutSchema v1、`SCI-04`(Core) 样本级执行身份 | 冻结参数、读数、样本的权威来源与类型 |
| P1 SDK 扩展效率 | `SDK-03`(Core) 语义编辑、`SDK-05`(Core) Readout 写入、`SDK-04`(Core) 结构化诊断、`SDK-01`(Build) Scaffold、`SDK-02`(QA) Parity | 让 Agent 能安全编辑与批量记录 |
| P2 真实实验验证 | `OPS-01`(Core) 操作模块、`RDO-01`(Core) 记录模块、`QA-07`(QA) 三组 fixture | 用 CCK-8、RNA/RT-qPCR、非孔板验证通用性 |

依赖顺序：`SCI-01` → `SCI-02`/`SCI-03` → `SCI-04`、`SDK-03`、`SDK-04`、`SDK-05`、`SDK-01`、`OPS-01`、`RDO-01` → `SDK-02`、`QA-07`。`DOC-05`(Docs) 独立，修模块目录与契约状态。

建卡时的两个判断：

- `DOC-05` 独立成卡，不并进 `SCI-01`：`SCI-01` 是只读 Audit，修文档需要写权限。
- 三组 fixture 合并为一张 `QA-07`：共用同一 runner 与契约，分开建会重复脚手架。

已核实的事实（写入 `DOC-05` 验收）：`spec/modules.json` 有 16 个 `kind@schema`（目录文档写 15，`step@1/2/3` 是同一 kind 的三个版本）；`compatibility-and-gaps.md` 位于 `docs/sdk/` 而非 `docs/release/`。

阶段推进门（2026-11-13 起生效）：

- 建卡准入：feeder 把 Inbox 卡推入 Ready 前跑静态校验（scope 块可解析、证据路径在 `scope.write` 内、非 Docs 卡验收含可执行命令、有 `lane:*` 标签、无未替换占位符）。不通过打 `dor-fail` 并推 Telegram，不进 Agent。
- 僵尸回收：`kandev-zombie-reaper.timer` 每小时清理 Kandev 已标记 stopped 但进程仍存活的 omp。
- 阶段回顾：feeder 每轮检查 VPS lane 是否排空，排空且有水位线之后的新合并时自动建 `RETRO-<日期>` 卡，由 Luna 写回顾报告并开 PR。
