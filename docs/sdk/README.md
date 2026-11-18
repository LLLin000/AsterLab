# AsterLab SDK

Status: Foundation v1 and Module SDK v1 are frozen at RC (`sdk-v1-rc3`, 2026-11-12). The contracts in this directory are the developer-facing surface; the GUI exposes the subset recorded in [compatibility and gaps](compatibility-and-gaps.md).

| Document | Purpose |
| --- | --- |
| [Foundation v1](foundation-v1.md) | Frozen AsterML envelope, data types, units, references, time, artifacts and compatibility rules. |
| [Unified module contract v1](module-contract-v1.md) | The twelve contract dimensions every module declares, and where each lives in the single manifest. |
| [Module development guide](module-development-guide.md) | How to add a module without adding a second syntax or bypassing Core. |
| [Module catalog v1](module-catalog-v1.md) | All 15 registered modules, port-by-port, the exact new-module checklist, rule-name parity hazard, and surface wiring. |
| [Compatibility and gaps](compatibility-and-gaps.md) | Acceptance-gate evidence and the limitations that remain. |
| [Track A: CCK-8 real-window evidence](track-a-cck8-evidence.md) | Real Run, batch readouts, task completion, Review and frozen isolation. |
| [Track B: SDK extensibility evidence](track-b-extensibility-evidence.md) | Dilution/instrument_profile generic editing and frozen Run viewing. |
| [Typed Step Module Binding v1](step-binding-v1.md) | `step@2.uses`, `step@3.execution`, exact ProtocolBlock source validation and Run materialization. |
| [AsterML v2 Module Visual Catalog](v2-module-visual-catalog.md) | 全量 21 个 SDK 模块逐一真实渲染图鉴、统一 `plateui.View` 与 `孔板速排` 交互/导出对照。 |
| [AsterML v2 Architecture Retrospective & Evidence](v2-architecture-retrospective-and-evidence.md) | 架构演进复盘、P0+MOD 全量验收账本、多分辨率实图证据与 `孔板速排` 设计器细节。 |
| [Release evidence](evidence/) | Real-window 900×600 light/dark and Run captures. |
| [Worktree classification](worktree-classification.md) | The 72 remaining uncommitted entries and release handling rules. |
| [Foundation JSON Schema](../../prototypes/mygo-run/internal/asterdoc/spec/foundation-v1.schema.json) | Machine-checkable envelope, port, action, manifest and template shapes. |
| [Single module manifest](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json) | The authoritative module registry read by Go and Rust. |
| [Foundation unit registry](../../prototypes/mygo-run/internal/asterdoc/spec/units.json) | The named UCUM subset used for validation and explicit conversion. |

The existing [AsterDoc contract](../asterdoc-contract.md) describes the document store, compiler and Run freeze behaviour. This SDK directory does not replace it; it freezes the extension layer around it.
