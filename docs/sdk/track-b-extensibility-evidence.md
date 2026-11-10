# Track B: SDK extensibility evidence

- Date: 2026-11-11
- Scope: the two reference modules, generic Manifest-driven editing, frozen-module viewing.
- Status: both modules register through the single manifest and render through the shared registry; the frozen Run viewer is implemented. Task-scoped binding is still partial (see gaps).

## 1. Modules

| Module | Manifest | Generic editor | Frozen Run viewer |
| --- | --- | --- | --- |
| `dilution@1` | quantity inputs, `dilution.v1` derived outputs | yes | yes |
| `instrument_profile@1` | class/model/capabilities/source | yes | yes |

Both are in `prototypes/mygo-run/internal/asterdoc/spec/modules.json`; no second registry or parallel manifest was introduced.

## 2. Generic editor

`(*Document).UpdateModule` rewrites only the named fence, reparses the whole document, and rejects invalid quantities/units. `View.OnUpdateModule` is the host seam; the host applies an `update_module` semantic command.

Real-window check:

- Design revision 10 contained both modules.
- The `文档` tab rendered `stock`, `target`, `final_volume` as editable fields and `仪器能力档案` with class/model/capabilities.
- Editing `stock` from `20` to `10` mg/mL and saving advanced the AsterDoc from revision 9 to 10.
- The instrument editor has the same generic field path; its save behavior is covered by the headless test `TestGenericEditorSavesChangedInstrumentModel`.

No bespoke dilution or instrument form was added.

## 3. Frozen binding

`compile_asterdoc_to_design` now preserves non-core modules in:

```json
"asterlab": {
  "foundationVersion": 1,
  "sdkVersion": "1.0.0",
  "modules": [ { "kind": "dilution", "id": "...", "schema": 1, ... } ]
}
```

The DesignVersion snapshot and `FrozenDesignView` carry this optional field. The Go Run snapshot merges it into `DesignData`; the Run page renders the modules through the same `internal/asterdoc` registry, read-only, under `SDK 模块 · 冻结视图`.

Real-window check after creating a Run from revision 10:

- Run page showed `SDK 模块 · 冻结视图`.
- Dilution viewer showed `母液 10 mg/mL · 目标 2 mg/mL · 终体积 5 mL`, `母液体积 1 mL`, `稀释液体积 4 mL`, and `dilution.v1`.
- Instrument profile viewer showed `UV-1` / `uv_curing` and the capability.

The module payload is planned/configuration data; it is not written into execution events by the viewer.

After `step@2` was introduced, `step-seed` was bound to `uv-1` and `dilution-01`. In the real Run window, selecting `root/seed` showed `SDK 模块 · 本任务绑定` with both modules; selecting the unbound `root/read` showed `该任务未绑定 SDK 模块`. The binding uses the stable Step ID and `mygo.stepUses`, not the title.

## 4. Tests

- `TestUpdateModulePreservesProseAndOtherFences`
- `TestUpdateModuleRejectsInvalidQuantityUnit`
- `TestGenericEditorRendersDilutionFieldsAndSave`
- `TestGenericEditorReadOnlyNoSave`
- `TestGenericEditorSavesChangedInstrumentModel`
- `TestCompileAsterDocPreservesSDKModules`
- Rust `frozen_design_carries_sdk_modules`
- Go `TestSDKModulesFromFrozenDesign`
- `crates/asterlab-app` batch test asserts per-readout IDs.

## 5. Gaps

- The binding is implemented through `step@2.uses`; `step@1` is unchanged. Role values are currently validated as non-empty strings rather than a closed enum.
- A read-only Notebook timeline now exposes the same authoritative events and batch drilldown.
- The Step Inspector now edits `uses` through the existing `update_module` command; the generic editor still treats arbitrary list/object fields as read-only.
- Task identity currently uses frozen path + scope + timepoint; a separate Task Template/Instance ID remains a future extension.
- The generic editor intentionally leaves list/object fields (capabilities/constraints/evidence) read-only in this pass.
- No `sdk new`/`sdk validate`/`sdk test` CLI; tooling remains the Go registry plus shared manifest and tests.
- Dynamic third-party loading, static/PDF, full UCUM, templates and device adapters remain out of scope.
