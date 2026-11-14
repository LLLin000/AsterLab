# Kandev Review 与合并策略

本文件固定 `VPS Dev` 工作流中 `Review` 步骤的 agent 行为。Review 与实现共用 `GPT-6 Luna 1017` profile（`omp-acp` / `openai-codex/gpt-6-luna`），但进入 `Review` 时按 `profile_session_start_policy: new` 新开独立 session，只审核、不改代码、不合并。DeepSeek 使用 OpenCode Go；额度耗尽时只单向回退到 OpenAI Codex Luna。Luna 的额度错误不会触发回切 DS。`kandev-ready-feeder` 在 30 秒轮询中发现额度错误后执行回退。

## 1. 容量与背压

- `Agent` WIP=4，最多四个实现 agent 并行。
- `Review` WIP=4，最多四个任务停留/审查。
- Review 满 4 个时，`scripts/kandev-ready-feeder.py` 停止把 Inbox 卡提升到 Ready，也停止把 Ready 卡放入 Agent。只有 Review 空出名额后才继续。
- Review agent 与实现 agent 共用 profile 的 4 个 session 名额，因此 Review 可能排队；这是预期行为。

## 2. 分类审查方法

| 分类 | 审查方法 | 通过后的动作 |
|---|---|---|
| `(Docs)` | diff 只落在 `scope.write`；运行卡片验收；检查链接、事实与版本 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(Audit)` | 只读审计；检查证据、commit/file/命令、结论表 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(Build)` | diff 只在 `.github/**` 或 `scripts/ci/**`；检查 YAML、命令、无 secrets | 门槛通过→`APPROVED` + ReviewOutcome；`.github/**` 改动合并后通知人工 |
| `(Contract)` | 契约一致性、类型可编译、错误码可引用 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(Core)` | diff、Rust 测试、冻结语义、错误路径 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(UI)` | diff、Windows 截图/录屏、`ux-contract.md` 规则 id | 门槛通过→`APPROVED` + ReviewOutcome |
| `(Integration)` | fixture 路径、原始输出、证据目录、冻结数据 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(QA)` | 测试矩阵、隔离数据、数据库校验和、复现步骤 | 门槛通过→`APPROVED` + ReviewOutcome |
| `(Release)` | 干净 Windows/CI、安装/升级/回滚证据、资产 hash | 门槛通过→`APPROVED` + ReviewOutcome |

## 3. 共同验收门槛

Review agent 必须逐条确认：

1. 存在与本任务关联的 PR，`gh pr view` 显示 open 且 mergeable。
   - 例外：`gh pr list --head <branch> --state all`（或按卡片编号 `gh pr search '"<卡号>" in:title'`）显示本卡已有 merged PR 时，说明合并已完成、只是事件没送到卡片。直接输出 `VERDICT: APPROVED`，第二行写 `ALREADY-MERGED #<n>`，不要新建 PR、不要重复合并。
2. `git diff --name-status origin/main...HEAD` 只包含 `scope.write` 中的路径；`scope.forbid` 无改动。
3. 任务描述「验收」中的命令已在 PR 或任务评论中贴出原始输出并全部通过。
4. 没有密钥、token、绝对路径泄漏；没有移动 frozen tag；没有直接推 `main`。
5. UI 卡有 1360×880 / 900×600、浅色/深色证据；QA 卡有证据目录。
6. 没有修改任务范围外的文件。

## 4. 结论与动作

结论消息的第一行必须是且仅是下列之一；系统只读第一行做流转。

| 结论（第一行） | 动作 |
|---|---|
| `VERDICT: APPROVED` | 门槛通过且符合设计：附最新 head SHA、必需检查与 scope 证据，然后用 `engctl review outcome record … --verdict APPROVED` 记录 ReviewOutcome；合并由 Home Merge Gate 执行，review agent 不调用 `gh pr merge`、不调用 `step_complete_kandev` |
| `VERDICT: APPROVED`（拿不准设计） | 第二行写疑点，不记录 Outcome，留在 Review 等人工 |
| `VERDICT: CHANGES` | 列出可复现证据与失败文件；**不修改代码、不推送、不合并**，任务留在 Review 由实现 agent 在原分支返工 |
| `VERDICT: BLOCKED-NEEDS-HUMAN` | 卡片自相矛盾或权限不足，agent 无法解决；停下等人工，feeder 推送 Telegram |

Review agent 只读：不写代码、不推送、不合并、不调 Kandev move，职责到结构化结论和（仅 APPROVED 时）ReviewOutcome 记录为止。熔断口径：同一任务累计 3 次 `VERDICT: CHANGES` 后打 `human-hold`，留 Review 等人工。

## 5. 合并门

- `Verify` 与 `Review` 都监听 `on_pr_merged`；`Merged` 进入即完成任务。
- feeder 有兜底扫描：Review/Verify 里的卡，若关联 PR 已 merged，或按卡号能搜到 merged PR（Kandev 未关联时），直接移入 `Merged`。只认**本轮**合并：早于本卡最近一次进入 `Inbox`/`Agent` 的 merged PR 属于上一轮，不算（否则同卡号的旧 PR 会把新卡误判为已完成）。
- 依赖卡只在 `Merged`（即 `COMPLETED`）后解除阻塞，因此不会出现「前序 Done 但 PR 未合并」的基线错位。

## 6. 例外记录与分支清理

- 2026-11-13 `REL-03` / PR #3：仓库已要求 `linux-tests`、`windows-build`、`scope-check`，但 `main` 还没有 `.github/workflows`（只在未合并的 `REL-04` PR #4 中），常规 merge 会被 base-branch policy 拒绝。该次以 `--admin` 合并，diff 已人工核验为单文档、范围合规。这是一次性 CI bootstrap 例外；后续 Review 禁止 `--admin`。
- `REL-04` PR #4 是 CI bootstrap PR。它的 checks 在同一 PR 上运行；三个 context 通过后正常合并，不需要 `--admin`。
- `gh pr merge --delete-branch` 可能因为 `main` 被另一个 worktree 占用而清理失败。此时由人工/维护者执行：
  1. 确认任务 worktree 干净；
  2. 从共享 clone 移除 worktree；
  3. 删除本地分支和远端分支。
- 不要 force-remove 有未提交改动的 worktree；先保留并报告。
