# AsterLab VPS 泳道阶段回顾（2026-11-14）

范围：E2E-FULL、E2E-AGENT、E2E-MERGE-GATE、E2E-PRODUCTION-CANARY、QA-07、DES-00；仅统计本次范围内任务及其 PR。六卡均已完成并合并。PR：#51、#52、#53、#54、#63、#71。

## 1. 改动总结

- **Core：** DES-00（PR #71）为 `asterdoc.command` 增加步骤重排、嵌套/提升和引用删除阻断行为，并让 `workflow.inspect_tree` 按层级与业务顺序投影步骤树。
- **CLI：** DES-00 同步 `AGENT_API.md` 命令契约；QA-07 通过真实 CLI fixture 覆盖 Design、Protocol、Run、事件写入与读取。
- **CI：** 六个 PR 的 `scope-check` 均成功；#51–#54 与 #63 的 `linux-tests`、`windows-build` 均成功；#71 当前可见检查只有 `scope-check` 成功，未见 Linux 测试记录。
- **Docs：** #51–#54 分别记录闭环 smoke、Agent 全流程、Merge Gate sentinel 和生产 canary；QA-07 与 DES-00 新增可复核的运行说明和原始验收证据。

## 2. 出现的问题

- **E2E-FULL（#51）：** Kandev 记录 1 个已取消 session，PR 文档只记录受控 smoke 摘要；最终 PR 的三项检查均成功，session 取消没有阻止交付。
- **E2E-AGENT（#52）：** 3 个 session 均记录为取消（含一次 API 停止），显示长生命周期 session 会重启/回收；PR 的三项检查成功，文档明确保留独立 Review 与 Gate 合并路径。
- **E2E-MERGE-GATE（#53）：** 任务有最初“可在检查后合并”的受控目标，PR 文档仅留下 sentinel 记录；三项检查成功并合并，生产路径由 #54 进一步验证。
- **E2E-PRODUCTION-CANARY（#54）：** 3 个 session 记录为取消/回收，但 PR 仅增一份 canary 文档且三项检查成功；因此 session churn 未影响 canary 合并，不能据此断言异常已经消除。
- **QA-07（#63）：** 10 个取消及 2 个失败 session 与最终成功 fixture 并存；通过隔离临时库、三组可重复 fixture、重启后读取、并发只读查询和 SQLite 校验和给出明确结果，三个类别均为 PASS。
- **DES-00（#71）：** 11 个取消及 3 个失败 session 后仍完成实现；PR 添加 2,268 行、删 10 行，记录包含编译警告，且 GitHub 可见检查仅有 `scope-check`。提交的证据文档记载 Rust 三组测试与 schema 命令成功，但 PR CI 未见 `linux-tests`，因此 CI 证据链不完整。

## 3. 做的调整

- 流程验证从文档 smoke（#51–#53）推进到生产 canary（#54）：保持 Review 独立、Merge Gate 负责最终合并，不以测试任务替代或绕过生产 gate。
- QA-07 将三个领域 fixture 放入隔离临时数据目录，并同时产出文本与 JSON 证据；这样可分别复核正常路径、输入边界、重启恢复、并发只读和数据库未被查询改变。
- DES-00 把步骤操作做成带预期 revision 的 Core 命令，并在同一 AsterDoc revision 中更新步骤集合、业务顺序和 edges；引用/冻结 Run/嵌套等保护条件拒绝删除，避免产生局部写入。
- 任务 session 记录显示这六卡合计有 28 个 `CANCELLED` 和 5 个 `FAILED` session 状态；四个 E2E 文档任务、QA-07 和 DES-00 最终均合并。`task_review_runs` 对这六个 task id 没有记录，且 `automation_retry_groups` 为空；因此本报告不把 session 状态称作 Review verdict，也不声称发生过熔断。

## 4. 可能的问题

- E2E、QA 和 Core 卡都出现 session 取消/失败；虽然对已完成卡没有阻断最终合并，但如果回收重启吞掉唯一验收输出，后续证据可能缺失。应把 session 生命周期和产物持久化分别观察，而不是把取消数当作任务失败数。
- DES-00 的高 session churn 与大幅 Core PR 增量叠加，增加实现和审查遗漏风险；可见 PR 检查缺少 `linux-tests` 是当前最直接的验证缺口，任务附带的本地测试输出不能替代 CI 记录。
- PR #51–#54 的 diff 是各一份短文档，因此 CI 通过验证的是仓库和 gate 流程，不是深度功能行为；生产 canary 也不能独自证明所有后续任务都能抗 session churn。
- 六卡的 `task_review_runs` 无 verdict 历史，DB 中也无该范围的 retry-group/fuse 记录；结论依赖任务完成状态、session 状态、PR diff 与检查结果，无法从这些数据还原完整 Review 结论或熔断处置。

## 5. 项目状况

- **Milestone 进度：** 本次范围内六张卡均已完成/合并（#51、#52、#53、#54、#63、#71）；这些卡的任务记录没有 milestone 字段，因此不能据此计算项目整体里程碑百分比。
- **技术债：** HANDOFF-VPS §5 仍列出 static renderer 未实现、通用编辑器的 object-list 字段只读、窄屏 Design inspector 未改为 bottom sheet、RNA 多步骤 Run 验收未在六表面变更后重跑、GUI 仍用 Go workflowtree 等未完成项；本次范围没有验证或关闭这些事项。
- **Windows lane：** AGENTS.md 记载 Windows Gate 是人工泳道、没有自动 agent profile/executor；本次 VPS 回顾不能代替 Windows GUI / `runs.dll` / PowerShell E2E 验收。PR 检查中的 `windows-build: SUCCESS` 不等于完成实机 GUI Gate。

## 6. 下一步方向（候选，待人工拍板）

1. **补跑 DES-00 的 Linux CI 验收并留存检查记录**：原因是 #71 当前只见 scope 检查而无 `linux-tests`；依赖 VPS CI runner 和相同 PR head SHA。
2. **在 Windows Gate 重跑合并后 GUI 验收**：原因是 VPS 无法证明桌面/PowerShell E2E；依赖人工 Windows 主机或 VM 与 Gate 排期。
3. **从审计分类中选下一项 SDK 模块并按 RNA 模块路径交付**：原因是 HANDOFF §6 将其列为后续方向；依赖人工选择具体模块及其契约/验收范围。
4. **关闭一个 SDK 缺口：static renderer 或 object-list 编辑**：原因是 HANDOFF §5 明确两项尚未完成；依赖人工定优先级及 SDK Owner 的范围确认。
5. **检查 session 回收/重启造成的证据连续性**：原因是本范围 33 个 session 终态中有 28 个取消、5 个失败，而最终 PR 仍然完成；依赖 Kandev 生命周期/执行维护者提供可观测性与重试语义的决策。

---

证据来源：Kandev 任务、session、review-run 和 retry-group 表（只读）；GitHub PR #51/#52/#53/#54/#63/#71 的 diff 与状态检查；仓库 HANDOFF-VPS.md §5–6、AGENTS.md。合并时间以本任务提供的范围列表为准。