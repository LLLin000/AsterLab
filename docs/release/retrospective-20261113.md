# RETRO-20261113 VPS 阶段回顾

> 范围仅为本任务列出的 19 张 VPS 卡及各自关联 PR；按用户给定时间窗口检视。本文只汇总这些卡，不代表完整路线图或 Windows 验收结果。

## 范围与证据口径

- Kandev 中这 19 张卡均为 `COMPLETED`，关联 PR 均为 `MERGED`；按本次卡片集计，M0 7/7、M1 3/3、M2 1/1、M3 2/2、M4 2/2、M5 4/4。该计数只描述本次 VPS 切片，不等于各里程碑整体完成率。
- Kandev `task_review_runs` 对其中 8 张卡有 13 条结构化记录：10 条 `CHANGES REQUESTED`、2 条被阻塞、1 条 `APPROVED`；session 消息还包含额外 verdict 通知，且单次评审可能重复发消息，因此不把消息数伪装成返工轮数。`human-hold` 标签出现在 REL-02、AG-02；任务库未提供独立的熔断事件记录，故这里只确认标签状态。
- `gh pr checks`：PR #2、#4–#13、#15–#21 的 `linux-tests`、`scope-check`、`windows-build` 最终均通过；PR #3 没有报告 checks。PR #7 的 Kandev 同步快照为 2/3 passing，而本次 `gh pr checks` 返回三项均通过，说明历史计数可能滞后。
- Git 记录与 PR 对应关系：REL-03 #3（1278968）、REL-04 #4（3887d65）、REL-01 #2（f0d437d）、REL-05 #6（65b36cb）、RUN-03a #5（2eaee38）、AG-01 #8（c29471f）、DOC-01 #7（8a457e3）、RUN-04a #9（98a8c16）、AG-02 #10（8363d1c）、REL-02 #11（bb2d325）、DOC-02 #12（e1cd1a2）、REL-06 #13（8a422e1）、REL-07 #15（b5831c3）、AG-03 #17（2423959）、QA-06 #16（19b27d3）、QA-04 #18（747dbbd）、DES-03 #19（3d06f54）、QA-05 #20（5e7f42f）、QA-05a #21（636a49e）。

## 改动总结

- **Core：**RUN-03a/RUN-04a 建立读数事实与事件/重试时间线，AG-01/02/03 形成 Agent-edit 提案、预验证和提交链，DOC-01/02 补齐 Run 资产关联与重定位，DES-03 与 QA-04/05/05a/06 强化复合字段、数据安全及 SQLite 迁移/备份路径（PR #5、#7–#10、#12、#16–#21）。
- **CLI：**DOC-01、AG-03、QA-05a 把资产引用、人工确认后的版本提交及旧库 schema 迁移接入 CLI（PR #7、#17、#21）。
- **CI：**REL-04 建立 Linux 测试、Windows 构建和 scope-check 门禁，REL-05 加入跨平台生成 hash 检查，REL-06 加 Go 测试门禁，REL-07 将 Rust 工具链固定为 1.100.0（PR #4、#6、#13、#15）。
- **Docs：**REL-03 留存 Run Workspace 分支差异审计，REL-01 冻结 Beta 范围/DoD，REL-02 统一文档权威与 MyGo Native 交付方向，并由 QA 卡补充可复现证据（PR #2、#3、#11、#16、#18、#20、#21）。

## 出现的问题

- **REL-01：**首轮评审指出 Beta 非目标覆盖不足，补充非目标说明（4f1f3ec）后，PR #2 的三项检查全绿并合并。
- **REL-02：**PR #11 的 `scope-check` 连续失败，原因是 PR 正文缺少 `kandev-scope` 注释块（run 43344757749，diff 本身未越界）；任务随后带 `human-hold`，补正范围块并修正文档事实、ADR 冲突表述与 beta-scope 引用（a0d8b35）后三项检查通过。
- **REL-05：**Windows CRLF checkout 会造成生成绑定 hash 漂移，PR #6 加入 `.gitattributes` 和双平台 hash 证据后通过。
- **RUN-03a：**原始读数与派生值边界在首轮评审中不清，4a71d73 将派生 readout 排除在 raw facts 之外后获批。
- **DOC-01：**资产路径检查未拒绝非普通文件且路径保真不足，125dfd9 补上拒绝条件与精确路径处理后合并。
- **AG-02：**结构化评审记录三次 `CHANGES REQUESTED` 后才批准，PR #10 的修改收紧了 Core 参数预验证；三项必需检查最终全部通过。
- **DOC-02：**资产 SHA-256 比较把合法的大写十六进制摘要当作不匹配，c55d04e 改为不区分大小写后通过。
- **RUN-04a：**重试目标没有 step 时仍可能被接受，0f04269/9383553 增加 lineage 校验并拒绝无 step 目标。
- **QA-05/QA-05a：**评审发现显式 schema 版本/失败回滚验收不足，且裸 exact filter 会匹配零测试；后续 QA-05a 实现单事务原子迁移并补失败注入与旧库 fixture，QA-05 将证据命令/fixture 改为可重复运行，PR #20/#21 最终检查全绿。
- **QA-06：**跨平台路径断言把 Windows 的 `a:b.csv` 语义处理错，8f77098 修正断言、9e464bb 收紧扫描器匹配后通过。
- **REL-06：**Windows-only Go 测试失败未被修复，而是由 8ecd423 从 CI 门禁排除；当前 CI 因此通过，但该测试覆盖缺口仍在。
- **AG-03、QA-04：**AG-03 评审指出单位别名写入和幂等重放断言问题，df98541/8b69c9a 修正；QA-04 的 SQLite 证据 transcript 经 1d1a9fa/e82d911 校正并重跑后合并。

## 做的调整

- REL-04 将 `linux-tests`、`windows-build`、`scope-check` 作为 PR 检查；REL-06 在两平台 workflow 加入 Go 测试，REL-07 从实测 CI 工具版本锁定 Rust 1.100.0，原因是避免稳定版漂移造成同一提交结果不一致。
- REL-05 将行尾规范化纳入仓库配置并以 hash 对照验证生成器输出，原因是 Windows checkout 行尾差异已实际改变绑定 hash。
- REL-01/REL-02 把 Beta 范围、非目标和文档权威关系写成可引用的文档基线，减少实现卡依据历史材料产生冲突。
- REL-02 暴露 `scope-check` 直接读取任务描述/PR 正文的 `kandev-scope` 注释块：缺块即红，即使 diff 未越界；后续开 PR 必须携带任务 marker 与范围块。
- 评审方面，Audit/Docs/Build 类按快速门槛审查，Core/Contract/QA 变更继续依赖人工 verdict；Kandev `human-hold` 标签用于标出重复返工后需人工介入的卡，但本次 DB 未保留独立熔断事件原因。

## 可能的问题

- **返工与验证脱节：**AG-02、DOC-02、QA-05 与 REL-02 出现多轮 `CHANGES REQUESTED`，其中有的 PR 检查在修订期间已经全绿，说明自动门禁不能代替契约、边界和证据完整性评审。
- **返工数据不完整：**结构化 `task_review_runs` 只覆盖 8/19 张卡，session 中的 verdict 通知又可能重复；人工复盘无法从单一表准确还原每张卡的返工轮次。
- **Windows 覆盖被低估：**REL-06 通过排除已知失败测试取得绿灯，Windows build 成功不等于完整 Go 测试套件或 GUI/E2E 已验证。
- **CI 状态来源存在时差：**PR #7 的 Kandev 快照记录 2/3 检查通过，而 GitHub 当前检查记录为 3/3 通过；依赖同步快照做发布判断可能误报。
- **评审证据存在边界：**PR #3 没有 CI checks；AG-01 的 Kandev 结构化记录只有一次含 4 个 findings 的 `CHANGES REQUESTED`，而 PR 已合并，当前记录中没有后续结构化批准可证明结论闭环。
- **Windows lane 仍是人工边界：**按 AGENTS.md，Windows Gate 没有 agent profile 或已注册 executor，VPS 不能替代 MyGo/Slint、Runs.dll、安装包及 PowerShell E2E 的 Windows 实机验收。

## 项目状况

- 本次纳入的 19 张 VPS 卡均已完成并合并；各 milestone 在本次切片中的分布为 M0 7、M1 3、M2 1、M3 2、M4 2、M5 4。该数值不代表 M0–M5 全部工作完成。
- 本次可确认的技术债：REL-06 排除的 Windows-only Go 测试待恢复；真实 Windows 构建/GUI/E2E 与安装包验收仍需 Windows Gate；Kandev 的 review verdict、human-hold 原因与 CI 快照需要更完整的结构化留存。
- 本次没有 Windows 实机证据，因此不把 Windows build CI 的通过表述为 Windows 产品验收完成。

## 下一步方向

以下均为候选，不作优先级决定：

1. 在 Windows 主机复现并修复 REL-06 排除的 Go 测试，再恢复双平台完整测试；理由是目前绿灯不覆盖该已知失败，依赖 Windows 环境和可重复用例。
2. 执行 MyGo Native、Runs.dll、安装包与 GUI/PowerShell E2E 的 Windows Gate 验收；理由是 VPS CI 只证明构建/测试门禁，依赖 Windows 主机、人工验证与所需产物。
3. 为 Agent-edit 的 propose→validate→confirm→commit 增加端到端消费方验证；理由是核心契约虽已落地，仍需证明界面确认边界和 Core 写入边界一致，依赖 Windows 端可用的 UI 验收环境。
4. 扩展 SQLite 迁移的旧版 fixture 矩阵并持续验证失败回滚；理由是 QA-05a 已实现原子迁移但兼容范围由旧库样本决定，依赖保留的旧版数据库 fixture。
5. 将 verdict、返工轮次、human-hold 触发原因及 PR CI 快照写入可查询的结构化事件；理由是本次 DB 不能稳定还原完整返工历史，依赖 Kandev 事件/API 能记录并同步这些字段。
