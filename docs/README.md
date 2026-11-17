# AsterLab 文档地图

本文件只解决“哪份文档现在有权威”。研究笔记不是规范；被权威文档引用后才进入当前设计。

## 权威文档

| 文档 | 权威范围 |
|---|---|
| [`../CONTEXT.md`](../CONTEXT.md) | 领域术语、对象边界、读模型/执行身份、工具/仪器边界 |
| [`../AGENT_API.md`](../AGENT_API.md) | 当前已实现 CLI/HTTP 字段、枚举、错误码 |
| [`release/doc-authority.md`](release/doc-authority.md) | 文档「权威 / 历史 / 研究」状态登记与冲突处置 |
| [`release/beta-scope.md`](release/beta-scope.md) | Beta 交付平台、必须闭环、非目标与发布 DoD |
| [`product/ux-contract.md`](product/ux-contract.md) | 当前六表面信息架构、UX 规则与布局契约 |
| [`product/ownership.md`](product/ownership.md) | 文件所有权与并发开发泳道约束 |
| [`architecture.md`](architecture.md) | 模块依赖、前后端职责、迁移方向（Slint 节为历史） |
| [`contracts/core-v1.md`](contracts/core-v1.md) | P0 Core 查询/命令/身份/兼容契约（以及同目录 `operation-spec-v1.md` / `operation-spec-v2.md`（v2 为 CLOSE-03 时间模型 sidecar 权威）/ `readout-schema-v1.md`；归档终态见 `record-closure.md`，多阶段计时见 `time-plan-closure.md`） |
| [`sdk/README.md`](sdk/README.md) | AsterML Foundation v1、统一模块契约、SDK 开发指南 |
| [`../DESIGN.md`](../DESIGN.md) | 视觉与交互规范；不反向定义领域对象 |
| [`sdk/huvec-tube-formation-case/UI-REDESIGN.md`](sdk/huvec-tube-formation-case/UI-REDESIGN.md) | V2 实验台重构基线（含 `DESIGN.md` 与 [`sdk/cross-assay-primitives-audit.md`](sdk/cross-assay-primitives-audit.md)） |

## 归档与研究分组

- 早期 Slint / Rune / Web 时代规划与历史根目录文档已统一归档至 [`archive/README.md`](archive/README.md)。
- 历史研究笔记保留在 [`research/README.md`](research/README.md)，未被权威文档引用前不进入当前设计。

## 阅读顺序

1. [`../README.md`](../README.md) & [`release/doc-authority.md`](release/doc-authority.md)
2. [`../CONTEXT.md`](../CONTEXT.md) & [`../AGENT_API.md`](../AGENT_API.md)
3. [`release/beta-scope.md`](release/beta-scope.md) & [`product/ux-contract.md`](product/ux-contract.md)
4. [`contracts/core-v1.md`](contracts/core-v1.md)、[`contracts/operation-spec-v1.md`](contracts/operation-spec-v1.md)、[`contracts/operation-spec-v2.md`](contracts/operation-spec-v2.md)、[`contracts/readout-schema-v1.md`](contracts/readout-schema-v1.md)（归档：[`contracts/record-closure.md`](contracts/record-closure.md)；计时：[`contracts/time-plan-closure.md`](contracts/time-plan-closure.md)）
5. [`sdk/huvec-tube-formation-case/DESIGN.md`](sdk/huvec-tube-formation-case/DESIGN.md) → [`UI-REDESIGN.md`](sdk/huvec-tube-formation-case/UI-REDESIGN.md) → [`sdk/cross-assay-primitives-audit.md`](sdk/cross-assay-primitives-audit.md)
## 状态标记

研究文档应在标题下保留其状态（研究笔记 / 候选 / 已实现 / 已取代）。没有状态的历史文档，默认按“历史研究”读取，不当作当前规范。
