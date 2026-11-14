# AsterLab 文档地图

本文件只解决“哪份文档现在有权威”。研究笔记不是规范；被权威文档引用后才进入当前设计。

## 权威文档

| 文档 | 权威范围 |
|---|---|
| [`../CONTEXT.md`](../CONTEXT.md) | 领域术语、对象边界、读模型/执行身份、工具/仪器边界 |
| [`../AGENT_API.md`](../AGENT_API.md) | 当前已实现 CLI/HTTP 字段、枚举、错误码 |
| [`architecture.md`](architecture.md) | 模块依赖、前后端职责、迁移方向 |
| [`implementation-plan.md`](implementation-plan.md) | 唯一当前实施阶段、完成/未完成、验收 |
| [`contracts/core-v1.md`](contracts/core-v1.md) | P0 Core 查询/命令/身份/兼容契约 |
| [`sdk/README.md`](sdk/README.md) | AsterML Foundation v1、统一模块契约、SDK 开发指南与已知缺口 |
| [`../DESIGN.md`](../DESIGN.md) | 视觉与交互规范；不反向定义领域对象 |

## 研究分组

| 组 | 文档 | 状态 |
|---|---|---|
| 产品目标 | `../GOAL.md`、`research/2026-11-03-cellplanner-design-baseline-and-glossary.md`、`research/2026-11-02-cellplanner-experiment-design-model.md`、`research/2026-11-02-cellplanner-design-flow-walkthrough.md` | 产品背景/历史论证；目标以 GOAL + CONTEXT 为准 |
| 领域契约 | `../CONTEXT.md`、`../AGENT_API.md`、`research/2026-11-03-cellplanner-contract-modules.md`、`research/2026-11-04-asterlab-assay-coverage-research.md` | 术语/契约权威；研究文档为证据 |
| 实验操作 | `research/2026-11-05-protocol-knowledge-and-related-projects.md`、`prototypes/instrument-step-modules.md`、`prototypes/p2-operator-workflow-spec.md`、`prototypes/2026-11-05-step-editor-implementation-plan.md` | 方案/候选；P4 前不实现 StepMeta |
| 交互与视觉 | `research/2026-11-03-cellplanner-uiux-canvas-review.md`、`research/2026-11-05-asterlab-ui-ux-audit.md`、`research/2026-11-04-asterlab-visual-language-research.md`、`../DESIGN.md`、`prototypes/asterlab-native-ui-acceptance.md` | 视觉权威为 DESIGN；其余为评审证据 |
| 实现与迁移 | `architecture.md`、`implementation-plan.md`、`contracts/core-v1.md`、`prototypes/2026-11-05-experiment-map-and-agent-contract.md`、`../HANDOFF.md` | 当前以 implementation-plan 为唯一计划 |

## 阅读顺序

1. `../GOAL.md`
2. `../CONTEXT.md`
3. `../AGENT_API.md`
4. `architecture.md`
5. `contracts/core-v1.md`
6. `implementation-plan.md`
7. 与当前阶段相关的研究/原型文档

## 状态标记

研究文档应在标题下保留其状态（研究笔记 / 候选 / 已实现 / 已取代）。没有状态的历史文档，默认按“历史研究”读取，不当作当前规范。

<!-- gate probe -->
