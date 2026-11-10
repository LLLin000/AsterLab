# AsterLab SDK

Status: Foundation v1 contract work in progress. This directory defines the developer-facing contract; it does not claim that every capability described here is already exposed in the GUI.

| Document | Purpose |
| --- | --- |
| [Foundation v1](foundation-v1.md) | Frozen AsterML envelope, data types, units, references, time, artifacts and compatibility rules. |
| [Unified module contract v1](module-contract-v1.md) | The twelve contract dimensions every module declares, and where each lives in the single manifest. |
| [Module development guide](module-development-guide.md) | How to add a module without adding a second syntax or bypassing Core. |
| [Compatibility and gaps](compatibility-and-gaps.md) | Acceptance-gate evidence and the limitations that remain. |
| [Track A: CCK-8 real-window evidence](track-a-cck8-evidence.md) | Real Run, batch readouts, task completion, Review and frozen isolation. |
| [Track B: SDK extensibility evidence](track-b-extensibility-evidence.md) | Dilution/instrument_profile generic editing and frozen Run viewing. |
| [Typed Step Module Binding v1](step-binding-v1.md) | `step@2.uses`, compiler projection and task-scoped Run filtering. |
| [Foundation JSON Schema](../../prototypes/mygo-run/internal/asterdoc/spec/foundation-v1.schema.json) | Machine-checkable envelope, port, action, manifest and template shapes. |
| [Single module manifest](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json) | The authoritative module registry read by Go and Rust. |
| [Foundation unit registry](../../prototypes/mygo-run/internal/asterdoc/spec/units.json) | The named UCUM subset used for validation and explicit conversion. |

The existing [AsterDoc contract](../asterdoc-contract.md) describes the document store, compiler and Run freeze behaviour. This SDK directory does not replace it; it freezes the extension layer around it.
