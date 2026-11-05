# Research 文档索引

这些文件记录 AsterLab / Lab Workbench 的产品研究、术语和设计边界。它们含有产品建议、源码观察和桌面走查，**不等同于代码已实现或用户测试已通过**。当前运行能力以 README.md 与 AGENT_API.md 为准；科学对象术语以 CONTEXT.md 为准。

## 建议阅读顺序

1. [完整交接说明](../../HANDOFF.md) — 项目目标、当前基线、待解决问题、OMP 下一步与验收要求。
2. [设计基线与术语表](2026-11-03-cellplanner-design-baseline-and-glossary.md) — 当前产品方向与关键对象定义。
3. [UI/UX 设计画布评审](2026-11-03-cellplanner-uiux-canvas-review.md) — 四步流程、关系图、工具中心、真实案例、消融评估候选。
4. [视觉语言与图标方案研究](2026-11-04-asterlab-visual-language-research.md) — 现有界面核对、图标集、视觉图层、风险和后续验收；内含 [Gemini SVG 图标样张](2026-11-04-asterlab-ui-icon-exploration.svg)。
5. [全界面 UI/UX 走查](2026-11-05-asterlab-ui-ux-audit.md) — 操作者视角的导航收敛、逐屏布局/文案、单位与移动端问题；区分源码事实、浏览器观察和建议。
6. [前端架构:设计工作区与共用词表](2026-11-05-design-workspace-architecture.md) — 四区域壳、浏览/草稿状态、四步流程、受控词表、外部产品参考与已知数据缺口。
7. [K0 知识覆盖审计](2026-11-05-k0-knowledge-coverage-audit.md) — 实验/步骤/仪器覆盖矩阵、代码事实、文档冲突与证据审核计划；当前没有 Source Checked 以上条目。
8. [组别 × 检测覆盖语义研究](2026-11-04-asterlab-assay-coverage-research.md) — 核实现状、未知/明确不做/不适用契约缺口、保守文案与四态候选。
9. [协议知识、步骤模型与相关项目](2026-11-05-protocol-knowledge-and-related-projects.md) — 常见实验的步骤/目的/对照/失败模式、协议软件与标准参照、产品边界。
10. [实验设计契约与步骤块](2026-11-03-cellplanner-contract-modules.md) — Protocol/步骤块复用、参数、Agent 写 Protocol 的契约。
11. [实验设计模型](2026-11-02-cellplanner-experiment-design-model.md) — 科学设计模型、n、Assay、对照、样本及 CLI 契约边界。
12. [实验设计流程走查](2026-11-02-cellplanner-design-flow-walkthrough.md) — L929、SA 梯度、生物膜、动物/影像等已有案例和 gap。

## 证据标签

- **实现现状**：由当前源码、README 或 Agent API 核对。
- **产品候选**：研究者/Agent 提出的设计方向，仍需讨论或比较。
- **待验证**：未由真实用户测试、实际仪器或真实执行数据验证。
- **外部依据**：官方文档、标准或论文链接。官方资料支持的是其具体声明，不自动证明 AsterLab 方案最佳。