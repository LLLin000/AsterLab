# CLOSE-01 Bridge 命令校验与笔记幂等 — 收口记录

- HEAD: `d7d25e8`（开工基线即当前 main，无其他任务分支改动）
- 复核结论：两个分歧在基线上仍然存在，均已修复（下有失败→通过的测试证据）。

## 1. 入口一致性矩阵（Bridge `execute_command` vs CLI `dispatch`）

| Bridge kind | CLI 同名工具 | 校验/幂等现状 |
|---|---|---|
| `execution_event.create` | `execution_event.create` | 一致：同 `create_execution_event_tx` 校验；envelope key 经 `find_command_event_tx` 重放/冲突 |
| `readout.submit` / `readout.batch` | `readout.submit` / `readout.batch` | 一致：同 planner（`plan_readout_request` 内容键）+ `find_readout_tx` 重放/冲突，单事务提交 |
| `run.create` | `design.create_run` 系（`run.create`） | 一致：同 `create_run_with_source_in_transaction`（跨 Design 允许、跨项目拒绝）；envelope key 经 `find_run_tx` 重放/冲突 |
| `run.set_status` | `run.set_status` | **本卡修复前分歧**：CLI 在 `in_progress` 时调用 `ensure_run_protocol_ready`（`dispatch.rs:626`），Bridge 直写。修复：Bridge 在 `in_progress` 时复用同一 `ensure_run_protocol_ready`，不在 Go 加 guard |
| `run.revise` | `run.revise` | 一致：同 `patch_run`（title/scheduled_date + expected_revision；含 `in_progress` 的 protocol-ready 检查） |
| `note.create` | `note.create` | **本卡修复前分歧**：CLI 为追加写（`dispatch.rs:1565`，无 envelope 字段），Bridge 同样直写导致同 key 重试追加第二条。修复：Bridge 经 `create_note_tx` 消费 envelope `idempotency_key`（存 `data.operation_id` + `operation_payload`；同 key 同载荷重放首条、异载荷 `IDEMPOTENCY_MISMATCH`、失败不落记录、重开连接后重放仍成立）。CLI 保持追加形态不变 |

Go 侧无替代 guard：修复全部在 `crates/asterlab-app/src/lib.rs`（Rust Core），`src/cli/` 零改动。

## 2. 回归测试（消费者可见）

`crates/asterlab-app/src/lib.rs` 新增：

- `bridge_set_status_reuses_protocol_ready_guard`：构造 `protocol_ready=false` + 必填未解决参数的 Run，Bridge `run.set_status → in_progress` 必须拒绝且 revision 不变。
- `bridge_note_create_is_idempotent`：同 key 同载荷两次提交只有一事实（`replayed:true`，同 id）；异载荷冲突；重开 `AppCore` 后重放仍成立。

## 3. 证据

- `cargo test --manifest-path crates/asterlab-app/Cargo.toml`：80 passed
- `cargo test --bin labworkbench`：68 passed
- 原始输出见本 PR 描述。
