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
8. [K2 仪器来源审计](k2-instruments/README.md) — 移液器、CO₂ 培养箱、吸光度读板机、荧光显微镜四份逐项来源记录；全部仍为 research_draft。
9. [K2/K3 操作模块来源审计](k2-operations/README.md) — 液体转移、接种/孵育、洗涤/取样、吸光度/荧光读出四份逐项操作记录；全部仍为 research_draft。逐字段矩阵见 [field-evidence-matrix](k2-operations/field-evidence-matrix.md)；AI 预审见 [k2.1-review](k2-operations/k2.1-review.md)；五操作横向验证见 [k2.2-cross-operation-matrix](k2-operations/k2.2-cross-operation-matrix.md)。
10. [组别 × 检测覆盖语义研究](2026-11-04-asterlab-assay-coverage-research.md) — 核实现状、未知/明确不做/不适用契约缺口、保守文案与四态候选。
11. [协议知识、步骤模型与相关项目](2026-11-05-protocol-knowledge-and-related-projects.md) — 常见实验的步骤/目的/对照/失败模式、协议软件与标准参照、产品边界。
12. [实验设计契约与步骤块](2026-11-03-cellplanner-contract-modules.md) — Protocol/步骤块复用、参数、Agent 写 Protocol 的契约。
13. [实验设计模型](2026-11-02-cellplanner-experiment-design-model.md) — 科学设计模型、n、Assay、对照、样本及 CLI 契约边界。
14. [操作与仪器记录规范](record-standard.md) — OperationRecord / InstrumentRecord 的必填结构、实验来源优先级、证据等级、UNKNOWN 分类、JSON 转换与审核清单;本轮仅限体外/细胞/微生物/分子湿实验。
15. [Protocol 来源清单](protocol-sources.md) — protocols.io、Bio-protocol、Nature Protocols、JoVE、Current Protocols、CSH、ASM 等仓库的用途与访问注意。
16. [覆盖账本](coverage-ledger.md) — 操作/仪器覆盖、来源强度、JSON 覆盖与下一批优先级；文件数量不等于可验收。
17. [粒度冻结](k2-operations/granularity-freeze.md) — 最小可审计科学操作的四条判定、四层结构、Custom Step 与争议裁决。
18. [Bio-protocol 4298 类器官拆解](k2-operations/organoid-bio-protocol-4298.md) — 直接读取完整 Bio-protocol;只覆盖体外 Matrigel dome,不把方法参数写成通用默认。
19. [Morimoto 基础细胞培养](k2-operations/morimoto-basic-cell-culture.md) — 传代、血球计数、台盼蓝、冻存/复苏;大学 lab protocol 直接读取。
20. [Protocol-driven Operation Discovery](protocol-discovery/README.md) — 每篇 Protocol 两张表(步骤拆解/公共操作复用)、来源索引、覆盖矩阵与缺口;B01–B08 已完成,覆盖矩阵见 [coverage-matrix](protocol-discovery/coverage-matrix.md),一致性审查见 [catalog-consistency-audit](protocol-discovery/catalog-consistency-audit.md),候选复审见 [candidate-recheck](protocol-discovery/candidate-recheck.md),剩余实验地图见 [gap-map](protocol-discovery/gap-map.md)。
18. [粒度与生物膜](k2-operations/granularity-and-biofilm.md) — 生物膜不是单一 OperationModule;CV/CFU 是方法模板。
19. [粒度候选](k2-operations/granularity-candidates.md) / [独立复审](k2-operations/granularity-candidate-review.md) / [五场景](k2-operations/granularity-scenarios.md) — 约 18–20 个候选的四维审查与五类实验组合验证。
20. [Catalog 晋升审计](k2-operations/catalog-promotion-audit.md) — 原子性不等于公共模块;复用广度/语义稳定/抽象收益/差异成本四道门槛,以及第一版公共操作清单。
22. [实验设计流程走查](2026-11-02-cellplanner-design-flow-walkthrough.md) — L929、SA 梯度、生物膜、动物/影像等已有案例和 gap。

## 证据标签

- **实现现状**：由当前源码、README 或 Agent API 核对。
- **产品候选**：研究者/Agent 提出的设计方向，仍需讨论或比较。
- **待验证**：未由真实用户测试、实际仪器或真实执行数据验证。
- **外部依据**：官方文档、标准或论文链接。官方资料支持的是其具体声明，不自动证明 AsterLab 方案最佳。