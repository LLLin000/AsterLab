# 功能出入口验收矩阵（CLOSE-08）

- 基线：`main` @ `0fac770`（2026-11-17，CLOSE-07 合并后）。
- 状态：本阶段只做功能与入口收口，不做配色/字体/留白/视觉重设计。
- 权威关系：术语以 `CONTEXT.md` 为准，已实现字段以 `AGENT_API.md` 为准，范围以 `release/beta-scope.md` 为准，UI 规则以 `product/ux-contract.md` 为准，归档终态见 `contracts/record-closure.md`，多阶段计时见 `contracts/time-plan-closure.md`。
- RC-04（`e2a4603` 前 `d7d25e8`，`evidence/RC-04/README.md`）：Linux 门 PASS；Windows 构建 + Go 全套件 PASS；cargo 有 1 个确定性 Windows-only 失败（`protocol_version_queries_return_expansion_and_stable_not_found`，OS error 32）。
- RC-05（基线 `e2a4603`，`evidence/RC-05/`）：DS-08 `[BLOCKED]`；DS-12/DS-13 `[UNVERIFIED]`；DS-05 GUI E2E 步、DS-07 CLI `sdk.validate` 探针、DS-04 GUI 冻结徽标截图 `[UNVERIFIED]`；DS-09 仅封闭文件语义已证、应用内按钮路径 `[UNVERIFIED]`。未验证项不得冒充完成。

## 1. 六表面出入口

| 表面（ux-contract §1） | 入口 | Core 命令 / 投影 | 人工确认点 | 输出 | 失败恢复 | 权限 / 冻结限制 | 证据 | 未知 |
|---|---|---|---|---|---|---|---|---|
| 1 项目工作区（一句话：找到实验继续做） | 启动/窗口导航/深链 `asterlab://run/<id>`（单实例跳已运行应用，CLOSE-07） | `project.list`、`design.list`、`run.list`（只读）；`record.archive_state` 供重启检查终态 | 选择继续哪个设计/Run | 继续上次任务或打开设计 | 会话恢复失败→显式错误，不静默清草稿 | 无 | `evidence/RC-05/beta-open-data.png`（6 表面 + Legacy 数据）；`product/evidence/closure-entries/README.md`（深链跳转截图） | 深链跨版本行为未验证 |
| 2 实验设计（起草与理解计划；主动作：创建 Run） | 设计页 → 预检 → 创建 Run | `design.create/revise`（`expected_revision`；过期→`REVISION_CONFLICT`，草稿保留）；`asterdoc.command`/`step.edit`（digest + `confirmed_by` + 幂等键；异载荷 `IDEMPOTENCY_MISMATCH`）；`protocol.block.preview`、`design.map`（只读）；`protocol.time_budget`（CLOSE-03，Core 串行投影，无时长步骤只计数不补） | 预检警告需明示确认；Run 创建是人工动作；`in_progress` 需 protocol-ready | 新 revision；预算（总跨度/净占手/并行空窗）来自 Core 持久化 sidecar，不从标题推断 | 冲突返回 `REVISION_CONFLICT` 并保留草稿（`stale_revision_rollback_test.go`）；digest 不一致拒绝 commit | 草稿可改；Run 创建后冻结快照不跟后续设计修改 | Go revision/rollback 用例 PASS（RC-04）；`product/evidence/closure-time/README.md`（预算条截图） | GUI 冻结徽标截图 `[UNVERIFIED]`（DS-04） |
| 3 实验运行（执行一步并记录事实） | Run 页记录面板（首屏）/步骤轨/计时条/图板 | `run.create`（冻结 manifest；`source_run_id` 仅同项目）、`run.set_status`（CLOSE-01：`in_progress` 复用 `ensure_run_protocol_ready`）、`execution_event.create`、`readout.submit/batch`（单事务、幂等 `replayed`）、`asset.link/update/unlink`（坐标 `wellId/timepoint/fovIndex`；CLOSE-06 冻结板只读计划位）、`run.revise`（仅 title/计划日期） | Timer/图板/读数提交前确认；跳过/排除需显式原因；Tools 草稿到 Run 任务需操作员确认后才写 | 冻结步骤定义 `steps`/计划 `tasks`/实际 `occurrences`（`run.workspace`）；Timer 记录持久化、可重启恢复 | 幂等重放首条（`replayed:true`）；整批任一失败整事务拒绝（`INVALID_INPUT` + `issues[]`，无部分事件） | 冻结 manifest/预算/计时行不随设计修改；`record.sign_archive` 归档后全部写入口拒绝（`REVISION_CONFLICT`），重启不解锁 | `cck8_loop_test.go`、`concurrency_cross_process_test.go` PASS；`product/evidence/closure-entries/`（计时条/读数截图）；`product/evidence/closure-capture/`（冻结板/补缺截图） | GUI E2E 粘贴→保存→重启→回读 `[UNVERIFIED]`（DS-05 GUI 步） |
| 4 Notebook（阐释与叙述，只写散文） | Notebook 纸面 → 引用已有事实 → 保存散文 | 只读 `record.projection`（引用解析）；`note.create`（CLOSE-01：envelope 幂等键；同 key 同载荷重放、异载荷冲突；CLI 保持追加形态） | 写散文前可引事实；引用必须解析到权威记录 | Markdown 研判 + 动态引用（导出 PDF 展平） | 同 key 重试不产生第二条（`replayed`）；旧无 `occurrence_id` 事件标“未精确绑定”，不补 ID | 归档后笔记 upsert 拒绝（`note_locked`）；永不创建测量/完成/执行文件 | `notebook_page_test.go` PASS（RC-04 Go 套件）；`product/evidence/closure-record/`（图板/归档截图） | 无 |
| 5 实验方案 Protocol（只读 frozen/preview 文档） | Protocol 页（冻结版 st 预览版分开） | `protocol.version.get/list`（只读，`frozen:true`；未知 ID→`NOT_FOUND`）；`protocol.block.preview`（只读展开） | 无（无编辑控件；预览永不标 `冻结`） | 冻结正文 + run 修订头；预览带设计修订 + `预览`字样 | 冻结缺失快照报错，不回退当前设计 | 只读；冻结文档不跟随设计编辑；导出只导出对应版本 | `protocol_page_test.go` PASS（RC-04 Go 套件） | 无 |
| 6 资料与工具（准备可复用材料；只送草稿） | Tools 左列表 → 计算/查表 → 送到 Design 或当前 Run 任务 | `calculator.*`（纯计算，`writes_data=false`）；`pantry.list/upsert/delete`、`label.export`（可选轻量台账，不进冻结）；`table/formula`；Tools 送出为未确认草稿 | 送到 Run 任务后操作员在 Run 确认，写入走同一 Core 命令 | 草稿（会话内）/计算结果/贴标清单 | 计算不写库；储藏室为整条覆盖写（省略字段清空），停用为墓碑（不复用短 ID） | Tools 永不直写执行事实；`pantry_item_id` 过期只告警（`advisory`），不阻断 | `prototypes/mygo-run/internal/tools/` 用例 PASS（RC-04 Go 套件） | 无 |

## 2. 横切闭环

| 闭环 | 入口 | Core 命令 / 投影 | 人工确认点 | 输出 | 失败恢复 | 权限 / 冻结限制 | 证据 | 未知 |
|---|---|---|---|---|---|---|---|---|
| 备份/恢复 | 运行中 UI「备份数据库」/「恢复备份」；CLI 文件复制 | HTTP `POST /api/backup`、`POST /api/restore`（需人工确认头）；恢复验 `PRAGMA integrity_check` 后替换重开 | 恢复前结束写入；一次只做一次恢复 | `<data-dir>/backups/labworkbench-YYYYMMDD-HHMMSS.sqlite`；恢复后记录/版本/事件可读未改写 | 恢复失败删临时文件并重开旧库；迁移失败保持旧库 4 行 + `user_version` 不变（`failed-migration-output.txt`） | 单用户单机；恢复时无其他写入；不跨平台共享数据目录 | QA-05 封闭文件语义已证（4→3→4 行，hash 一致）；`BACKUP_RESTORE.md` | 应用内按钮端到端 `[UNVERIFIED]`（DS-09 部分）；并发恢复 NOT RUN |
| 资产（图/文件引用） | Run 任务采图 → 关联坐标 → 图板/审计 | `asset.link`（`well_id/timepoint/fov_index`；无坐标=未记录，不猜孔位）、`asset.link.update`（显式 `null` 清空）、`asset.unlink`；`run.workspace` 的 `linked_assets`/`asset_coverage` | 同坐标重复关联只刷新原记录；未知板/越界拒绝整批 | `coordinateId`（`A1`/`A1_t6h`/`A1_t6h_fov01`）；图板缺漏按 `(well_id,timepoint)` 分组 | `unresolved_plate`/`ambiguous_plate`/`unknown_plate`/`duplicate_position` 拒绝，不生成匿名孔位事实 | 记录 ID 是定位句柄，更新坐标不重算；文件本身不动 | QA-06（`asset.link` 相对路径 `INVALID_INPUT`）；`closure-capture` 采图/补缺证据 | 无 |
| 派生（计算读数） | 计算器/派生写入 → `readouts[]` | `readout.*`（v1 schema 校验→读数校验→派生来源校验）；`calculator.biofilm_cfu@1`（只读派生，分组平均同组重复） | 母液/目标单位同量纲；目标高于储液拒绝；悬液量冲突拒绝 | 派生读数（`result_kind=derived`，`derived_from` 列来源）；分类/表格/文件存 `result_cells`，不进 `measurements[]` | 同 key 追加新修订（`revision` 递增），旧事件永不改/删/回写；`history` 可查覆盖链 | 派生不写库直到提交；提交走同一执行事件事务 | QA-09 三协议回归夹具（HUVEC/RT-qPCR/in-vivo）；EXP-13 CFU 派生 | 无 |
| 计时（预算/阶段/Timer） | Design 时间 sidecar → Freeze phases → Run Timer → 重启恢复 | `operation_spec` sidecar（v2 三类语义码）；`protocol.time_budget`（Core 串行投影）；`TimerSequence`（计划在外、游标在内；`timer_records` 持久化；`t0` 锚点触发与事件同一事务） | anchored 规则显式 offset（t0+6/12/24h 用三条 `occurrences:1`，均匀 `interval` 不冒充非均匀） | 预算 JSON（`total_span_s`/`hands_on_s`/`parallel_windows`）；阶段游标（`phase_index`/`round_index`） | 无 sidecar 旧步骤预算计无时长，不补默认；旧单阶段 timer 传空 phases 行为一致 | 修改设计（新 block version）不改已冻结 Run 的 manifest/预算/计时行 | `docs/contracts/time-plan-closure.md`；`closure-time` 计时/预算截图 | 无 |
| 归档（签核终态） | Record 页 → 汇总 → 签核 → 归档 | `record.projection`（冻结计划/Timer/引用/资产/读数/笔记锁/归档/issues）；`record.sign_archive`（`signed_by` + RFC3339 `signed_at_utc`，清单验版本/SHA256/路径/摘要）；`record.archive_state`（只读，供重启/回滚检查）；Bridge/Go 同一 Core 状态机 | 签核需 `signed_by` + 时钟（Core 无时钟，调用方提供）；`sign_and_archive` 一步走完需清单全过 | `recording→summarizing→signed→archived`（SQLite `run_archive_state`，重启锁定唯一依据）；清单指纹覆盖资产/笔记/投影 | 清单失败不落态；归档后写入口（note/event/revise/status/plate/well/asset/timer）全部 `REVISION_CONFLICT`，读不受影响；旧无表库视为全 `recording` | 终态锁定一切写入口（CLOSE-02）；`frozen tag sdk-v1-rc3` 不移动；冻结 Run 不改 | `docs/contracts/record-closure.md`；CLOSE-02/04 提交；`closure-record` 签核/归档截图 | 应用内按钮端到端与 GUI E2E 同属 `[UNVERIFIED]`（DS-05/DS-09 范围内） |

## 3. 旧卡与本阶段替代关系

| 旧卡/证据 | 结论 | 本阶段处理 | 依据 |
|---|---|---|---|
| DOC-06 旧 marker/PR（`f94ab93`，分支 `feature/doc-06-docs-agents-m-o5i`，AGENTS.md 调度更新） | **未合并**：`git merge-base --is-ancestor f94ab93 HEAD` = NOT-MERGED；远端分支仍存在 | 记录为异常并交协调者：不伪造已合并；本卡不代合该分支，不改其文件 | 本卡 worktree `git show f94ab93 --stat` + ancestor 检查 |
| RC-04（`d7d25e8`）cargo 失败 | 真失败：`protocol_version_queries_return_expansion_and_stable_not_found`（Windows open 句柄 vs `remove_dir_all`）；CORE-02（`2a06ef8`，`src/cli.rs:206` 起 `drop(db)`）修的是另一处单测清理，不冒充修复本失败 | 记录为 `[BLOCKED]`（DS-08），需 owning fix 卡；本卡不碰 `src/**` | `evidence/RC-04/README.md` § Cargo tests；`evidence/RC-05/notes-vs-scope.md` DS-08 行 |
| RC-04 旧 candidate gate | 已有在跑，不重复创建 | 不把旧 candidate gate 当本阶段最终验收；本阶段最终验收只认本矩阵逐项 + 本卡验收命令 | 任务实施 §3 |
| 旧 `implementation-plan.md` 引用 | 已归档：`docs/archive/planning-historical/implementation-plan.md`；`doc-authority.md` §3 记 `docs/README.md` 已同步为已解决 | 本卡仅登记，不改 `CONTEXT.md`（术语权威，避免改变术语权威） | `doc-authority.md` §2–3 |
| DOC-06（`2dd807c`，#96，归档历史文档 + V2 基线） | 已合并（ancestor 检查通过） | 归档终态以 `doc-authority.md` 为准，本卡不重复归档 | `git log --all --grep=DOC` |
| EXP-13/14/15/16、QA-HUVEC-01、QA-05/06/09、RC-01/02/03/05 | 均已合并（见 CHANGELOG + evidence 目录） | 本卡只引用证据，不重复造实现；禁止 mock/单测渲染代替真实写入闭环 | `docs/release/evidence/` 目录 + CHANGELOG |

## 4. 本阶段退出条件

- [ ] 本矩阵每行“证据”栏指向已合并提交/PR 或具名证据文件；“未知”栏无冒充完成的项。
- [ ] `beta-scope.md`、`KNOWN_LIMITATIONS.md`、`CHANGELOG.md`、`README-BETA.md`、`docs/README.md` 与实现一致（本卡 diff）。
- [ ] 卡片验收命令 exit 0（见任务验收）。
- [ ] 阶段退出后才启动独立 UI 美观优化（任务非目标）。

<!-- kandev-task: 6d86f194-f8e0-420b-b87e-8a5e49cf05cf -->

<!-- kandev-scope
write:
  - docs/release/beta-scope.md
  - docs/KNOWN_LIMITATIONS.md
  - docs/release/CHANGELOG.md
  - docs/release/README-BETA.md
  - docs/README.md
  - docs/engineering/closure/entry-exit-matrix.md
read:
  - docs/product/ux-contract.md
  - docs/sdk/huvec-tube-formation-case/UI-REDESIGN.md
  - docs/product/design-system.md
forbid: []
-->
