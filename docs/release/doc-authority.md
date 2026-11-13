# AsterLab 文档权威登记

- 状态：权威（文档状态登记）· 生效日期 2026-11-13
- 范围：只登记每份文档的「权威 / 历史 / 研究」状态、生效日期与冲突处置；不定义产品语义、字段、枚举或验收。
- 关联：Beta 交付范围 [`beta-scope.md`](beta-scope.md)；领域术语 [`CONTEXT.md`](../../CONTEXT.md)；已实现字段 [`AGENT_API.md`](../../AGENT_API.md)；UI 规则 [`ux-contract.md`](../product/ux-contract.md)；文档地图 [`docs/README.md`](../README.md)。

## 1. 状态定义

| 状态 | 含义 | 使用规则 |
|---|---|---|
| 权威 | 当前规范或当前入口 | 可以据此实现、评审和验收；冲突按 §3 处置 |
| 历史 | 已被取代或仅记录当时决策 | 保留原文用于追溯；不得作为当前实现依据 |
| 研究 | 候选方案、证据或调研 | 未被权威文档引用前不进入当前设计 |

## 2. 登记表

| 文档 | 状态 | 权威范围 | 生效日期 |
|---|---|---|---|
| [`README.md`](../../README.md) | 权威 | 项目入口导览、启动方式与当前限制 | 2026-11-13 |
| [`docs/architecture.md`](../architecture.md) | 权威 | 模块依赖、分层、读写方向与迁移方向；§3/§6 的 Slint 前端内容为历史记录 | 2026-11-13 |
| [`docs/implementation-plan.md`](../implementation-plan.md) | 历史 | 2026-11-05 起稿的 P0–P6/S0–S4 Slint 路线、完成项与验收；仅供追溯 | 2026-11-13（标记） |
| [`docs/release/beta-scope.md`](beta-scope.md) | 权威 | v0.1.0 Beta 交付平台、交付物、必须闭环、非目标与发布 DoD | 2026-11-13 |
| [`CONTEXT.md`](../../CONTEXT.md) | 权威 | 领域术语与对象边界 | 2026-11-13 |
| [`AGENT_API.md`](../../AGENT_API.md) | 权威 | 当前已实现 CLI/HTTP 字段、枚举、错误码 | 2026-11-13 |
| [`docs/README.md`](../README.md) | 权威 | 文档地图与阅读顺序；其「权威文档」表对 implementation-plan 的表述见 §3 | 2026-11-13 |
| [`docs/contracts/core-v1.md`](../contracts/core-v1.md) 等 | 权威 | 各契约的字段、身份、兼容与测试矩阵 | 各文档自身日期 |
| [`docs/product/ux-contract.md`](../product/ux-contract.md) | 权威 | UI 信息架构、规则 id 与交互约束 | 2026-11-12 |
| [`docs/product/ownership.md`](../product/ownership.md) | 权威 | 文件所有权与并行写规则 | 2026-11-12 |
| [`docs/research/`](../research)、[`prototypes/`](../prototypes) | 研究 | 调研证据、候选方案与历史原型 | 各文档自身日期 |

平台结论：**v0.1.0 Beta 的正式交付平台是 Windows x64 + MyGo Native 桌面应用 + 本地 Rust Core。** Web 包是历史交付物，Slint 是研究资产；两者都不是正式交付前端。交付物、验收泳道与非目标见 [`beta-scope.md`](beta-scope.md) §1、§3。

## 3. 冲突与处置

| 位置 | 现表述 | 冲突 | 处置 |
|---|---|---|---|
| [`docs/README.md`](../README.md) 权威文档表 | `implementation-plan.md` 是「唯一当前实施计划」 | 与本文将其标记为历史不一致 | REL-02 的 write 范围不含 `docs/README.md`；本表按 Beta 范围标记为历史，权威表修订留给后续卡片，修订前以 `beta-scope.md` 与本文为准 |
| [`CONTEXT.md`](../../CONTEXT.md) 文档权威节 | `implementation-plan.md` 是「唯一当前实施计划」 | 同上 | 同上；不改 CONTEXT 以免改变术语权威 |
| [`docs/adr/0001-web-first-beta.md`](../adr/0001-web-first-beta.md) | Web UI 是产品壳；Slint 不作第二正式前端 | 未被新 ADR 取代，但 `beta-scope.md` §1 已按 MyGo Native 冻结 | 本表不改 ADR；DS-12 要求在发布前新增取代其前端条款的 ADR。在 ADR 落地前，`README.md` 与 `docs/architecture.md` 只声明「按 Beta 范围」而非「平台决策已获批准」 |
| [`docs/architecture.md`](../architecture.md) §3、§6 | Slint 组件树与时序图 | 早期 Slint 原型记录，非现行前端 | 保留原文，章节头标「历史」，现行前端见 `ux-contract.md` §1 |
| [`docs/implementation-plan.md`](../implementation-plan.md) 全文 | P0–P6 以 Slint 为执行/编辑面 | 现行 Beta 前端为 MyGo Native | 保留原文，文首加迁移说明并把「当前结论 / 下一步」标为历史 |

## 4. 历史文档阅读规则

1. 历史文档保留原文事实，不改写当时结论；需要纠正时在头部或章节头加日期化说明。
2. 历史文档中的命令、路径和完成项可用于追溯 commit 与验收记录，但不得作为新实现的依据。
3. 同一事实在权威文档与历史文档不一致时，以权威文档为准；权威文档之间冲突记录到 §3，不在实现中静默选边。
