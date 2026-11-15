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
| [`README.md`](../../README.md) | 权威 | 项目入口导览、交付方向与当前限制；「启动」「当前工作流」节为历史 Web 包记录 | 2026-11-13 |
| [`docs/architecture.md`](../architecture.md) | 权威 | 模块依赖、分层、读写方向与迁移方向；§3/§6 的 Slint 前端内容为历史记录 | 2026-11-13 |
| [`docs/archive/planning-historical/implementation-plan.md`](../archive/planning-historical/implementation-plan.md) | 历史（已归档） | 2026-11-05 起稿的 P0–P6/S0–S4 Slint 路线；已移入 `docs/archive/` | 2026-11-15 |
| [`docs/release/beta-scope.md`](beta-scope.md) | 权威（文档自述为范围冻结候选） | v0.1.0 Beta 交付平台、交付物、必须闭环、非目标与发布 DoD | 2026-11-13 |
| [`CONTEXT.md`](../../CONTEXT.md) | 权威 | 领域术语与对象边界 | 2026-11-05 |
| [`AGENT_API.md`](../../AGENT_API.md) | 权威 | 当前已实现 CLI/HTTP 字段、枚举、错误码 | 2026-11-12 |
| [`docs/README.md`](../README.md) | 权威 | 文档地图与阅读顺序；其「权威文档」表对 implementation-plan 的表述见 §3 | 2026-11-11 |
| [`docs/contracts/core-v1.md`](../contracts/core-v1.md) 等 | 权威 | 各契约的字段、身份、兼容与测试矩阵 | 各文档自身日期（core-v1 为 2026-11-05） |
| [`docs/product/ux-contract.md`](../product/ux-contract.md) | 权威 | UI 信息架构、规则 id 与交互约束 | 2026-11-12 |
| [`docs/product/ownership.md`](../product/ownership.md) | 权威 | 文件所有权与并行写规则 | 2026-11-12 |
| [`docs/research/`](../research)、[`prototypes/`](../prototypes) | 研究 | 调研证据、候选方案与历史原型 | 各文档自身日期 |
| [`docs/archive/`](../archive/README.md) | 历史（已归档） | 早期 Rune/Slint/Web 设计、旧根目录文档与 E2E 哨兵文件 | 2026-11-15 |
| [`docs/sdk/huvec-tube-formation-case/`](../sdk/huvec-tube-formation-case/UI-REDESIGN.md) & [`cross-assay-primitives-audit.md`](../sdk/cross-assay-primitives-audit.md) | 权威候选（V2 演进基线） | HUVEC 纸面设计、三板块重排、五大通用原语与非强制储藏室规范 | 2026-11-15 |

平台结论：**v0.1.0 Beta 的正式交付平台是 Windows x64 + MyGo Native 桌面应用 + 本地 Rust Core。** Web 包是历史交付物，Slint 是研究资产；两者都不是正式交付前端。交付物、验收泳道与非目标见 [`beta-scope.md`](beta-scope.md) §1、§3。

§2 的「生效日期」是该文档当前状态对应的最后修订日期；未随本卡改动的文档取其在 git 中的最后修改日期，不是本登记表的发布日期。

## 3. 冲突与处置

| 位置 | 现表述 | 冲突 | 处置 |
|---|---|---|---|
| [`docs/README.md`](../README.md) 权威文档表 | `implementation-plan.md` 是「唯一当前实施计划」 | **已解决（2026-11-15）**：`docs/README.md` 已同步更新，`implementation-plan.md` 已归档至 `docs/archive/` | — |
| [`CONTEXT.md`](../../CONTEXT.md) 文档权威节 | `implementation-plan.md` 是「唯一当前实施计划」 | 同上，引用前以 `beta-scope.md` 与本文为准 | 不改 `CONTEXT.md` 以免改变术语权威 |
| [`docs/adr/0001-web-first-beta.md`](../adr/0001-web-first-beta.md) | Web UI 是产品壳；Slint 不作第二正式前端 | 未被新 ADR 取代，但 `beta-scope.md` §1 已按 MyGo Native 冻结 | DS-12 要求在发布前新增取代其前端条款的 ADR |
| [`docs/architecture.md`](../architecture.md) §3、§6 | Slint 组件树与时序图 | 早期 Slint 原型记录，非现行前端 | 保留原文，章节头标「历史」，现行前端见 `ux-contract.md` §1 |
| [`beta-scope.md`](beta-scope.md) §1.4 第 2、3 行 | 记 `README.md` 与 `docs/architecture.md` 仍写「Web 是正式交付前端」 | 过期记录 | 随 DS-12 卡一并更新 |

## 4. 历史文档阅读规则

1. 历史文档保留原文事实，不改写当时结论；需要纠正时在头部或章节头加日期化说明。
2. 历史文档中的命令、路径和完成项可用于追溯 commit 与验收记录，但不得作为新实现的依据。
3. 同一事实在权威文档与历史文档不一致时，以权威文档为准；权威文档之间冲突记录到 §3，不在实现中静默选边。
