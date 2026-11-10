# SDK Foundation v1: compatibility and gaps

- Date: 2026-11-10
- Baseline: working tree at commit `271eeff`.
- This file reports exercised checks only. It does not claim a user study or real-device validation.

## 1. Acceptance gates

| # | Gate | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Old AsterML documents remain valid | Implemented for the current suite | Existing parser/compiler/freeze tests and Go/Rust parity pass. `plate96`'s optional `assay` remains a legacy string because old documents may name an assay without an assay module. |
| 2 | `plate96` and `plate` remain compatible | Implemented | Plate geometry, group/well validation and assign/clear tests pass. |
| 3 | Schema, unit and reference validation | Implemented for Foundation v1 | `validateSpecModule`, `validatePortValue` and typed-reference tests; invalid units, dimensions and missing reference targets are rejected in Go and Rust. |
| 4 | Go/Rust consistency | Implemented for the tested surface | `TestAsterDocGoRustParity` now includes dilution, instrument-profile and typed-reference samples. |
| 5 | Module actions require Core permission | Existing contract retained | `View.Can(PermDocumentEdit)` and Core command checks; no new write bypass was added. |
| 6 | Document and record are not dual-written | Retained | New modules are document/render-only. Dilution outputs are derived and explicitly not written as Run facts. |
| 7 | Three reference modules use one Registry | Implemented | `plate`/`plate96`, `dilution`, `instrument_profile` all read `spec/modules.json` and register in `BuiltinRegistry`. |
| 8 | Notebook can show modules through the same viewer | Implemented through the shared markdown code-block hook | The same renderer path is used for document surfaces; there is no notebook-only module parser. |
| 9 | New module does not change Core transaction logic | Implemented for these modules | No SQLite/Run transaction changes were made for the new modules. |
| 10 | New fields/features are versioned, not retrofitted | Contract defined | Old `step@1` is unchanged; new module kinds have schema 1. A future breaking change requires a new schema/migration. |

## 2. Implemented in this pass

- `spec/modules.json` now declares `foundationVersion`, `sdkVersion`, SDK/category/capabilities/permissions/renderers/lifecycle/provenance for shipped modules.
- Foundation port types in the shared manifest: quantity, typed reference, enum, decimal/integer/boolean, list, table, geometry, artifact, duration, timepoint, relative time, with legacy aliases.
- Shared `spec/units.json` named UCUM subset and explicit conversion helper.
- Typed reference target validation in Go and Rust.
- `dilution@1`: quantity validation, deterministic `dilution.v1` derived volumes, Go renderer.
- `instrument_profile@1`: capability/constraint/evidence declaration with no hardware control, Go renderer.
- Machine-checkable Foundation schema and manifest contract.
- SDK documentation set.

## 3. Remaining gaps

### Contract and tooling

- The CLI does not yet expose a stable `asterdoc manifest` / `asterlab sdk` command. `ManifestJSON` is a Go API and the JSON file is embedded; there is no user-facing command to be reported as available.
- Structured diagnostics are defined in the schema but the CLI still returns `code` + `message`; `needs_confirmation` / `advisory` arrays are not serialized yet.
- Manifest contract checking exists in the loader/tests, but there is no standalone `sdk validate` CLI.
- There is no module scaffold generator or test harness command.
- Go and Rust validators are maintained in parallel; there is no code generation from the manifest. The parity test catches sampled drift, not all drift.
- Unknown YAML fields are preserved but not reported. A strict mode is not implemented.
- The Foundation JSON Schema is documentation/contract tooling, not the runtime validator. Runtime validation remains the Go/Rust validators.

### Scientific/domain

- The unit registry is a named subset, not all of UCUM. No compound-unit parser, offset temperature conversion, or molar-mass conversion.
- `step@1` still cannot express HUVEC material amounts, UV duration, temperature and concentrations as structured fields; a future version/composable module is required.
- `instrument_profile` has no instance/calibration/reservation model and no Device Adapter. It cannot control hardware.
- SDK modules now bind through `step@2.uses` and are filtered per Run task. `step@1` remains unchanged; role values are not yet a closed enum.
- Notebook has a read-only timeline mode. Real dark-theme 900×600 evidence at 150% scale is captured; 100%/200% and additional dark pages remain open.
- No ISA/AnIML/PROV serialization or import.
- No template instantiation or template command.
- No migration runner for old AsterDoc module versions.
- No static/PDF renderer for the new modules.
- New modules now use the generic Manifest-driven editor in the Design document surface; list/object fields (including `step@2.uses`) remain read-only in that editor.

### Validation coverage

- External cross-document references are not resolved.
- The free `reference.ref` field can point at a record-store object but is not type-checked against record kinds.
- Duplicate YAML key behavior is not yet pinned as a shared diagnostic test.
- Reference target validation checks existence and kind, not semantic role (for example, an assay group ref does not prove the group is a valid control).
- Calculator failure paths are tested in Go; Rust validates inputs but does not expose a calculator result.

### UI/editor follow-up

The prior editor items are now covered by headless and real-window checks:

- local AsterDoc draft undo/redo for inspector fields, tasks, checks and workflow edits, plus Designer draft history;
- real GUI CCK-8: opened the existing document, selected a step, edited a title, undid/redid it, saved rev 3, bound a same-project Protocol through Core, created a Run from rev 5, and observed the Run workspace;
- real window at 900×600: minimum window lowered to 900×600; snapshot showed no top-level element outside the 900×600 client area, and the inspector scrolls internally.

Evidence images (generated from the real window, not headless):

- `prototypes/mygo-run/target/e2e-cck8-900x600.png`
- `prototypes/mygo-run/target/e2e-cck8-run-1080x640.png`

Remaining: high-DPI/scale-200 and dark-mode real-window capture are not yet part of this evidence set.

## 4. Compatibility policy

- Do not edit a released module's schema semantics in place.
- Add a new `schema` for a breaking change; add a migration or keep the old renderer.
- Do not remove a registered legacy kind while old documents can still be opened.
- Treat a renderer capability as a claim only when the renderer exists.
- Do not add a record-writing path to a document module.

## 5. Verification commands

```sh
# Go Foundation and module tests
cd prototypes/mygo-run
go test ./internal/asterdoc

# Go/Rust parity and document store tests
go test . -run 'TestAsterDocGoRustParity|TestAsterDocStoreIsImmutable'

# Full Rust Core tests
cd ../..
cargo test --quiet

# Rebuild the native module after Rust contract changes
cd prototypes/mygo-run
go run github.com/egoist/mygo/cmd/mygo native check modules/runs
```
