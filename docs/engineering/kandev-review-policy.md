# Kandev Review 与合并策略

本文件固定 `VPS Dev` 工作流中 `Review` 步骤的 agent 行为。Review agent 目前复用实现 agent 的 profile（`DeepSeek V4.1 Flash 1017` / `omp-acp` / `opencode-go/deepseek-v4.1-flash`），进入 Review 时切换角色，不是独立模型。

## 1. 容量与背压

- `Agent` WIP=2，最多两个实现 agent 并行。
- `Review` WIP=2，最多两个任务停留/审查。
- Review 满 2 个时，`scripts/kandev-ready-feeder.py` 停止把 Inbox 卡提升到 Ready，也停止把 Ready 卡放入 Agent。只有 Review 空出名额后才继续。
- Review agent 与实现 agent 共用 profile 的 2 个 session 名额，因此 Review 可能排队；这是预期行为。

## 2. 分类审查方法

| 分类 | 审查方法 | 合并权限 |
|---|---|---|
| `(Docs)` | diff 只落在 `scope.write`；运行卡片验收；检查链接、事实与版本 | Review agent 可 squash-merge |
| `(Audit)` | 只读审计；检查证据、commit/file/命令、结论表 | Review agent 可 squash-merge |
| `(Build)` | diff 只在 `.github/**` 或 `scripts/ci/**`；检查 YAML、命令、无 secrets | Review agent 可 squash-merge |
| `(Contract)` | 契约一致性、类型可编译、错误码可引用 | 只给结论，人工合并 |
| `(Core)` | diff、Rust 测试、冻结语义、错误路径 | 只给结论，人工合并 |
| `(UI)` | diff、Windows 截图/录屏、`ux-contract.md` 规则 id | 只给结论，人工合并 |
| `(Integration)` | fixture 路径、原始输出、证据目录、冻结数据 | 只给结论，人工合并 |
| `(QA)` | 测试矩阵、隔离数据、数据库校验和、复现步骤 | 只给结论，人工合并 |
| `(Release)` | 干净 Windows/CI、安装/升级/回滚证据、资产 hash | 只给结论，人工合并 |

## 3. 共同验收门槛

Review agent 必须逐条确认：

1. 存在与本任务关联的 PR，`gh pr view` 显示 open 且 mergeable。
2. `git diff --name-status origin/main...HEAD` 只包含 `scope.write` 中的路径；`scope.forbid` 无改动。
3. 任务描述「验收」中的命令已在 PR 或任务评论中贴出原始输出并全部通过。
4. 没有密钥、token、绝对路径泄漏；没有移动 frozen tag；没有直接推 `main`。
5. UI 卡有 1360×880 / 900×600、浅色/深色证据；QA 卡有证据目录。
6. 没有修改任务范围外的文件。

## 4. 结论与动作

| 结论 | 动作 |
|---|---|
| 快速合并分类且全部门槛通过 | `gh pr merge <n> --squash --delete-branch`，然后调用 `step_complete_kandev`；任务进入 `Merged` |
| 非快速分类且门槛通过 | 评论 `REVIEW: APPROVED`，列出门槛证据；不合并，任务留在 Review 等人工 |
| 任一门槛失败 | 评论 `REVIEW: CHANGES REQUESTED`，列出文件/行/命令；不合并；需要返工时把任务移回 `Agent` |
| 没有关联 PR | 评论并停止；不要合并 |

Review agent 不在 Review 里改代码、提交、推送或 rebase。

## 5. 合并门

- `Verify` 步骤监听 `on_pr_merged`，PR 合并后任务进入 `Merged`。
- `Merged` 进入即标记任务完成。
- 依赖卡只在 `Merged`（即 `COMPLETED`）后解除阻塞，因此不会出现「前序 Done 但 PR 未合并」的基线错位。
- 对于非快速分类，人工在 Review 里批准后，仍需合并 PR；合并事件会把任务推进到 `Merged`。
