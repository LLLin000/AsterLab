# Record Closure Contract v1（CLOSE-02 收口）

- 日期：2026-11-17
- 状态：生产权威；与实现同步落地。
- 基线：CLOSE-02 分支；`RecordProjection` / 归档状态为生产权威。
- 权威关系：术语见 [`CONTEXT.md`](../../CONTEXT.md)，架构见 [`architecture.md`](../architecture.md)。

## 1. 目标

`RecordProjection` 与归档终态是 Record 页面的唯一生产权威：

- 读取 `RecordProjection` 必须含冻结计划、Timer 实测、动态引用、资产与分类/表格读数；计划槽从冻结板/时间点/FOV 生成，整孔整时点未采集也计缺。
- 保持 plate/assay/task/timepoint 复合身份，禁止跨板串槽。
- `summarizing` / `signed` / `archived` 状态及 manifest 持久化；`SignAndArchiveRun` 及 CLI 进入共享事务校验。
- 所有“会影响已归档内容的写入口”处理一致；重启不解锁；禁止把正文 HTML 注释作为权威。
- 合法跳过/排除/缺失必须显式解释与确认，不硬要求全部任务 completed 或缺图为 0。
- 归档清单绑定确切投影/笔记/资产 checksum，校验失败不部分归档；保留旧记录兼容但不得把旧注释静默提升成 Core 归档。
- 覆盖重启、缺整孔、跨板、未知图、归档后写入拒绝与回滚。

## 2. 读：`record.projection`

CLI：`labworkbench record.projection --input {"run_id","view?","fov_per_well?","now_utc?"}`。
Bridge：`query_projection("record.projection", {run_id, view?, fov_per_well?, now_utc?})`。
Go：`RunService.RecordProjection`（已有 native 绑定）。

投影输入全部显式：冻结 workspace、资产链接、Timer 记录、笔记、归档状态。
`view=blind` 只遮蔽已声明盲法依据的冻结分组身份；默认 `unblinded`。
`fov_per_well=0` 或省略按每孔 1 个计划槽位；`now_utc` 省略时运行中计时器不上报已耗时间。
计划槽从冻结板/时间点/FOV 生成；整孔整时点未采集计 `missing_slots`，不从已上传资产反推计划。

输出即 `RecordProjection` JSON（含 `schema_version=1`、`parameters`、`timers`、
`images`、`assets`、`readings`/`readouts`、`timeline`、`notes` 引用解析、
`notes_locked`、`archive`、`issues`）。只读；不写数据库。

## 3. 状态机与持久化

状态：`recording -> summarizing -> signed -> archived`，终态锁定一切写入口。

- `summarize_record`：只有 `recording` 可进 `summarizing`。
- `sign_record`：只有 `summarizing` 可签核，需 `signed_by` 与 RFC 3339 `signed_at_utc`（调用方提供时钟，Core 无时钟）。
- `archive_record`：只有 `signed` 可归档；先 `verify_archive_manifest`（版本、SHA256 形状、逻辑路径、摘要一致），失败不落态。
- `sign_and_archive`：`summarizing -> signed -> archived` 一步完成；清单指纹覆盖每个已链接资产、每条笔记与投影自身。
- `apply_note_edit`：`archived` 拒绝（`note_locked`），其余状态允许 upsert。

持久化：共享 SQLite 表 `run_archive_state(run_id, data_json, updated_at)`。
由 Core `open_db` 迁移创建（与 `records` / `projection_cache` / `pantry_items`
同一事务），`AppCore::open` 与 native `ensure_module_tables` 幂等复建。
无该表的旧库（原始 `records`-only 单测夹具）视为全 `recording`，不报错。
状态行是重启后锁定的唯一依据；内存状态永不作为权威。

CLI 写入：`labworkbench record.sign_archive --input {"run_id","signed_by","signed_at_utc","statement?","view?","fov_per_well?","now_utc?"}`，
`labworkbench record.archive_state --input {"run_id"}`（只读，供重启/回滚检查）。
Bridge 写入：`execute_command("record.sign_archive", payload, idempotency_key)`。
Go 写入：已有 `RunService.SignAndArchiveRun`（native 直调同一 Core 状态机）。

## 4. 写锁：归档后拒绝

`ensure_run_not_archived`（`Connection` 版与事务 `Transaction` 版）是 CLI、
AppCore 与 native 桥共享的唯一规则：`notes_locked()` 即 `REVISION_CONFLICT`
（Bridge 信封为 `CONFLICT:` / `REVISION_CONFLICT:`），读不受影响。

覆盖的写入口（全部同规则）：

- `note.create`（CLI 与 Bridge 信封两条路径）
- `execution_event.create`（含 `create_execution_event_tx` 内全部上层：readout、batch、notebook、task）
- `run.revise` / `run.set_status`（含 `patch_run` 与 `set_status`）
- `plate.create` / `well.set`
- `asset.link` / `asset.unlink` / `asset.link.update`（经 `asset_link_scope`）
- `save_timer` / `update_timer`（native 桥；`timer_records` 行写入）
- AppCore `save_readout` / `save_readouts_batch` / `complete_task` / `execute_command`

回滚：任一校验失败不部分归档，不部分写入；失败保持草稿/旧记录不变。

## 5. 清单与引用语义

`ArchiveManifest`（`schema_version=1`）：条目为 `asset`（`rootId/relativePath`
逻辑路径 + SHA256 + size）、`note`（`notes/<id>.md` + markdown SHA256）、
`projection.json`（投影字节 SHA256）；`manifest_sha256` 为
`"<sha256>  <logical_path>\n"` 规范行的 SHA256。绝对路径拒绝
（`absolute_path`）；摘要不一致拒绝（`manifest_digest_mismatch`）。

动态引用（`@matrix` / `@params` / `@timeline`）由投影解析为结构化数据；
未知或畸形引用保持可见的未解析引用 + issue，不猜测。
HTML 注释永不作为归档权威：只有持久化状态行是权威。

## 6. 兼容与禁止

- 旧 `notebook` 观察事件与 `note` 记录都进入投影笔记；不静默提升旧注释为归档。
- 缺资产校验信息（无 `AssetRef` / 无 SHA256 / 无逻辑路径）拒绝归档
  （`asset_checksum_missing`），不跳过该资产。
- 重复归档拒绝（`already_archived`）；`recording` 直接签核拒绝
  （`invalid_transition`；CLI 自动先 `summarizing`，纯 Core 调用仍需显式状态）。
- 跨板同坐标资产报 `ambiguous_well` issue；未知孔资产报 `unknown_well`；
  无孔位资产报 `asset_without_well`；超计划 FOV 报 `fov_out_of_range`。
