# Module SDK readiness audit

Audit question: is the module SDK a real extension platform, or a built-in module architecture?  
Answer: **built-in module architecture with a strong, language-neutral contract**. There is no dynamic loading, no plugin ABI, and no scaffold tooling; adding a module means editing Go and Rust source and rebuilding both. The contract layer (`spec/modules.json`, `foundation-v1.md`, `module-contract-v1.md`) is the real asset.

Baseline: repository working tree; evidence below is cited as `file:line`.

## 1. Capability matrix

| Dimension | Current implementation | Verdict | What a third party must do today |
| --- | --- | --- | --- |
| Identity + schema | `spec/modules.json` `kind`/`schema`/`title`/`sdk`; Go `Descriptor` `registry.go:78-95`; Rust `asterdoc_module_spec` `src/cli.rs:2827-2831`; duplicate `kind@schema` rejected `src/cli.rs:2584-2587` | **solid** | Add one object to `modules.json`; keep `sdk` == `sdkVersion`. |
| Typed ports + units | `Port` `registry.go:37-49`; Go `validatePortValue` `spec.go:327-436`; Rust `validate_asterdoc_port` `src/cli.rs:2720-2808`; units `spec/units.json` (36 symbols, 9 dims) | **solid (types) / partial (units)** | Declare ports with Foundation types; unit expansion requires a versioned `units.json` change. |
| Lifecycle + provenance | `Lifecycle`/`Provenance` `registry.go:60-68`; `lifecycle.states`/`freeze`, `provenance.origin`/`record_boundary` in manifest | **partial** | Declare metadata only; no runtime state machine or hooks exist. Freeze happens via Core (`asterdoc.create_run`), not the module. |
| Multi-surface renderers | `Renderers` `registry.go:51-58`; `Register` refuses undeclared/missing keys `registry.go:118-158`; `renderRegisteredModule` `view.go:99-123`; renderer gap is visible `view.go:125-138` | **partial** | Implement a Go callback and register its key; missing surface shows a gap, never a fallback. No Rust renderer. |
| Complex-field editing | `GenericModuleEditor` `view.go:867-1012`; compatibility doc states list/object/table/geometry fields are read-only | **partial** | Either restrict to scalar fields, or ship a specialized editor (plate pattern: `plate.editor`). |
| Agent discovery | `sdk.modules` `src/cli.rs:8219-8237`; `sdk.describe` `src/cli.rs:8238-...`; `tools --json` dispatch `src/cli.rs:7859-9188` | **solid** | Manifest is auto-discovered after rebuild; no scaffold command. |
| Dynamic loading | `BuiltinRegistry()` `registry.go:265-299`; no load/unload; module-development-guide §7 lists `sdk new/validate/test/manifest` as not implemented | **absent** | Rebuild Go + Rust binaries; there is no plugin ABI. |
| Go/Rust validator duplication | Go `specValidatorFor` `spec.go:187-325`; Rust `validate_asterdoc_module` `src/cli.rs:2877-...`; parity test `asterdoc_store_test.go` | **partial** | Edit both validators and add a parity sample; drift is caught only on sampled inputs. |
| Tooling (new/validate/test) | `asterdoc.validate` exists; no `sdk new`, `sdk validate`, `sdk test`, `sdk manifest` (module-development-guide §7, compatibility §3) | **partial/absent** | Manually edit files; run `go test ./internal/asterdoc` and `cargo test`. |
| Cross-platform rendering | Renderers are Go (`RendererFunc` `registry.go:110`); static path `StaticPreview`/`StaticDocumentPreview` (compatibility §3) | **partial** | Render through the Go host; Rust has no renderer. |
| Data-write boundary | `record_boundary: run`; `View` never writes (`view.go:19-21`); dilution outputs derived, not Run facts (modules.go:210-211) | **solid** | Keep new modules document/render-only; record writes go through Core commands only. |

## 2. Three extension classes

1. **Scientific semantics** (new module kinds, ports, units, rules, derived calculators).  
   - What exists: manifest + Go `ModuleSpec`/`Descriptor` + Rust `asterdoc_module_spec` + `validate_asterdoc_module`.  
   - Requires Core changes **by design only when** the module participates in Run compilation (`compile_modules_to_design` `src/cli.rs:3248`, `step_materialization_issues` `src/cli.rs:1316`, `step@2`/`step@3` `uses`).  
   - Must **never** require Core changes for a document/render-only module (compatibility gate 9; module-development-guide §3 rule 3).

2. **Interaction + display** (renderers, editors, density, static/PDF).  
   - What exists: `Registry.RegisterRenderer` `registry.go:160-170`, `View.Density`, `StaticPreview`.  
   - No Core changes needed; but every declared renderer key needs a Go implementation or it is a visible gap (`rendererGap` `view.go:125-138`).

3. **Tools + behaviour** (actions, permissions, business commands).  
   - What exists: `actions[]` with `permission`/`writes`, `View.Can` `view.go:68-70`, Core command seams (`main.go` ~1220-1245 per module-catalog §5).  
   - A new document action needs no Core change; a new behaviour that records a fact must add a Core command by design (data-write boundary).

## 3. What breaks first (new module type → forced edit)

- New `kind`/`schema`: `spec/modules.json` append; Go `modules.go` new `xxxSpec()`; `registry.go` `BuiltinRegistry()` `Register`; Rust `compile_modules_to_design` match arm `src/cli.rs:3256-3264` (unknown kinds silently dropped today); `validate_asterdoc_references` kind-specific branches `src/cli.rs:2425-2523`.
- New cross-field rule name: Rust `validate_asterdoc_module` exhaustive `match rule` → `other => Err(internal(...))` (module-catalog §4.1); Go `specValidatorFor` `spec.go:187-325`.
- New derived calculator: Go `dilutionResultFor`-style function `modules.go:221-261`; Rust has no calculator result (compatibility gap).
- New renderer key: `registry.go` `Register`/`HasRenderer` `registry.go:177-179`; `view.go` `renderRegisteredModule` `view.go:99-123`.
- New step execution semantics: `step_materialization_issues` `src/cli.rs:1316-1360` and `build_run_manifests` `src/cli.rs:1237`.
- Complex field support: `GenericModuleEditor` `view.go:867-1012` (composite fields read-only).

## 4. Compatibility strategy for v2

Freeze: fence envelope and `kind`/`id`/`schema`; Foundation port types + aliases; UCUM subset (expansion versioned); typed references; diagnostic shape; released `step@N` semantics; manifest contract.

Version: `schema` per module for breaking field meaning; `sdkVersion` for manifest; renderer key shared across schemas only when rendering semantics are identical (e.g. `step@1`/`step@2` share `step.viewer`); new renderer key when interaction semantics change.

Leave open: domain fields, editor layouts, device commands, templates, dynamic loading, static renderer, diagnostic arrays, `reference.ref` resolution.

Versioning rule:
- `step@N`: additive only; `step@1`/`step@2` retain executable interpretation; `step@3` adds required `execution: executable|organizational`; never rewrite old fields (foundation-v1.md §4).
- Renderers: a capability is a claim only when implemented; missing surface is a visible gap, never a fallback (module-contract-v1.md §5).

## 5. Five highest-leverage changes (≤1 day each), ranked

1. Add `sdk validate` CLI wrapping `validate_asterdoc_spec` + `validate_asterdoc_source` + compile attempt — files: `src/cli.rs` tool dispatch `src/cli.rs:7859-9188`, `validate_asterdoc_spec` `src/cli.rs:2566-2630`. Closes the biggest tooling gap.
2. Make composite fields editable in `GenericModuleEditor` (lists/tables/objects) — file: `view.go:867-1012`. Unlocks every module's complex fields without per-module editors.
3. Implement `renderers.static` end-to-end via `StaticPreview` — files: `static.go`, `modules.json` renderer field; currently no module declares static (module-contract-v1.md §5).
4. Add a shared fixture consumed by both Go and Rust parity tests — files: `asterdoc_store_test.go`, `src/cli.rs` tests; reduces drift risk (compatibility §3).
5. Remove manual Rust rule-match arm by data-driving rule names from manifest — file: `src/cli.rs:2877` `validate_asterdoc_module`. Prevents the parity failure mode in module-catalog §4.1.