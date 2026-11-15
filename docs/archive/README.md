# 历史与归档文档目录 (`docs/archive`)

本目录存放已被当前 MyGo Native / Rust Core 架构或最新规范取代的历史研究、早期 Spike 截图、早期路线图与自动化哨兵测试文件。**保留原文用于追溯，不得作为新实现或评审依据。**

## 归档清单

| 子目录 | 原位置与内容 | 取代它的现行权威文档 |
|---|---|---|
| `root-historical/` | `COMPONENTS.md`, `GOAL.md`, `HANDOFF.md`, `MIGRATION.md`, `PRODUCT.md`, `RECORDING.md`, `SURFACES.md` | `README.md`, `CONTEXT.md`, `HANDOFF-VPS.md`, `docs/product/ux-contract.md` |
| `planning-historical/` | `docs/BETA_ACCEPTANCE.md`, `docs/DELIVERY_PROGRESS.md`, `docs/implementation-plan.md` | `docs/release/beta-scope.md`, `docs/release/doc-authority.md` |
| `design-historical/` | Slint / Rune / Web 时代 `docs/design/*`、v1.2 方向图、`rune-spike/` 等 | `docs/product/ux-contract.md`, `DESIGN.md`, `docs/sdk/huvec-tube-formation-case/UI-REDESIGN.md` |
| `prototypes-historical/` | `docs/prototypes/*`（`tabler-vs-dify-2026-11-06/index.html` 因 `src/main.rs` 内嵌保留在原位） | `docs/contracts/*`, `docs/sdk/*` |
| `handoff-2026-11-06/` | 2026-11-06 阶段交接记录 | `HANDOFF-VPS.md` 与 Kandev 任务卡 |
| `audits-2026-11-06/` | 2026-11-06 Gate 0 盘点 | `docs/release/doc-authority.md` |
| `e2e-sentinels/` | `docs/e2e-{agent-loop,full-smoke,merge-gate,production-canary}.md` | 仅作流程测试留痕 |
