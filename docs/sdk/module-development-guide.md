# Module development guide

This guide is for adding a scientific module to the current AsterLab prototype without adding a second document syntax or bypassing Core.

## 1. Choose the SDK category

| Category | Use for | Examples |
| --- | --- | --- |
| `workflow` | Steps, dependencies, experiment-level plan | `experiment`, `group`, `step`, `workflow` |
| `resource` | Materials, reagents, samples, instrument profiles | `instrument_profile` |
| `labware` | Plates, containers, spatial layouts | `plate` |
| `measurement` | Endpoints, measurements, calculations | `assay`, `dilution` |
| `document` | References, viewers, document-level composition | `reference` |
| `device` | Future device commands and acquisition | not implemented |

A module that needs two categories should usually be split. A calculator does not become a device because it is used near an instrument.

## 2. Add the manifest entry

Edit the single manifest:

`prototypes/mygo-run/internal/asterdoc/spec/modules.json`

Minimum shape:

```json
{
  "kind": "lab_example",
  "schema": 1,
  "title": "Example",
  "description": "What it represents and what it does not.",
  "sdk": "1.0.0",
  "category": "measurement",
  "capabilities": ["quantity"],
  "permissions": [],
  "renderers": {"design": "generic.editor", "document": "example.viewer", "run": "generic.run"},
  "lifecycle": {"states": ["draft", "validated", "frozen"], "freeze": "immutable-revision"},
  "provenance": {"origin": "planned", "record_boundary": "run"},
  "modes": ["design", "protocol", "run", "notebook", "review"],
  "inputs": [
    {"name": "kind", "type": "string", "required": true},
    {"name": "id", "type": "string", "required": true},
    {"name": "schema", "type": "int", "required": true},
    {"name": "volume", "type": "quantity", "dimension": "volume", "required": true}
  ],
  "outputs": [],
  "actions": [],
  "rules": ["example-fields"]
}
```

Checklist:

- [ ] `kind` is stable, lowercase, and matches `[a-z][a-z0-9_]*` (it becomes a Go identifier and a directory name).
- [ ] `schema` is a new version if any old field meaning changes.
- [ ] Each input has a Foundation type and a description.
- [ ] Quantities declare `dimension`.
- [ ] References declare `target`.
- [ ] Derived outputs declare `origin: "derived"` and a versioned `calculator`.
- [ ] Actions declare a permission that is present in `permissions`.
- [ ] Every `rules[]` name is registered in Go `knownModuleRules` and Rust `ASTERDOC_RULES`, with an arm in `validate_asterdoc_module`.
- [ ] Renderers are only declared when they exist.
- [ ] No runtime/device claim is made without a Device SDK implementation.

## 3. Implement validation and rendering

Go:

- `internal/asterdoc/spec.go` validates shared types and cross-field rules.
- `internal/asterdoc/modules.go` holds the registration and renderer.
- Register the module in `BuiltinRegistry()`.
- Do not add a second parser. `parseModuleWith` is the entry point.

Rust:

- `src/cli.rs` AsterDoc section validates the same manifest and implements the same rule names.
- The module body must be accepted or rejected consistently with Go; add a sample to `TestAsterDocGoRustParity`.

Rules:

1. **No new fence language.** A module is always ` ```asterlab ` with `kind`, `id`, `schema`.
2. **No direct document model write.** A command patches only the targeted fence body and reparses.
3. **No direct record write.** Actual measurements go through Core commands/events.
4. **No fabricated defaults.** Missing values stay missing; do not fill a scientific value from a title or a common convention.
5. **No privileged bypass.** A button's visibility is not an authorization check.

## 4. Rendering contract

A module may expose independently implemented surfaces:

- `design`: an editor or design-time summary;
- `document`: a read-only Protocol/Notebook/Review view;
- `run`: a frozen-plan view; actual records still go through Core commands;
- `static`: a deterministic static/PDF renderer.

`renderers.<surface>` names a callback key. The host registers shared callbacks such as `generic.editor` and
`generic.run` once; specialized modules can register their own keys. Module kind and schema still select the
data contract, while `View.Density` selects compact, inspector, or full presentation.

The renderer receives parsed module data and `View` context. It cannot write records; Design edits report semantic
commands through the host seam and require `document.edit`.

## 5. Add tests

Minimum:

1. Go parse test: valid document produces the expected module.
2. Go rejection tests: wrong type, wrong unit/dimension, missing typed reference, missing required field.
3. Calculator test: known input produces the expected derived value.
4. Go/Rust parity sample: add both valid and invalid sources to `TestAsterDocGoRustParity`.
5. Manifest test: the descriptor exposes inputs, outputs, permissions and renderers.

For a new Core projection, also add a freeze/round-trip test. For a pure viewer module, a compile/freeze test is enough only if the compiler intentionally ignores it; state that explicitly.

## 6. Reference implementations

### Plate

`plate` owns all supported formats (6/12/24/48/96/384). It validates geometry, group references and duplicate wells;
Design assignment uses `assign_wells` / `clear_wells`, while actual readouts remain Run records. Legacy AsterDoc
`kind: plate96` documents require explicit migration to `kind: plate` plus `format: 96`; there is no alias.

### Dilution

`dilution` is a Measurement module:

```yaml
kind: dilution
id: dilution-01
schema: 1
stock:
  value: 20
  unit: mg/mL
target:
  value: 2
  unit: mg/mL
final_volume:
  value: 5
  unit: mL
```

Derived outputs are `stock_volume` and `diluent_volume`, both `dilution.v1`. The renderer shows the values and labels them as derived; it does not write a preparation event.

### Instrument profile

`instrument_profile` is a Resource module. It declares class, model, capabilities, constraints, source and evidence. It is not a specific machine, a calibration record or a device connection. A future Device SDK may bind a profile to an adapter, but that is deliberately out of scope here.

## 7. Scaffold and check

Two repository scripts implement the "new module" checklist as commands. Both use only the Python 3
standard library, read `spec/modules.json` plus the Go/Rust sources, and are not the `asterlab sdk` CLI.

```sh
python3 scripts/sdk/scaffold.py --kind lab_example --schema 1 --dry-run   # file list only
python3 scripts/sdk/scaffold.py --kind lab_example --schema 1
python3 scripts/sdk/check.py                                              # exit 0 = contract holds
python3 scripts/sdk/check.py --json                                       # modules[] count + per-check results
```

`scaffold.py` writes six files under `prototypes/mygo-run/internal/asterdoc/_scaffold/<kind>@<schema>/`:

| file | purpose |
| --- | --- |
| `manifest.json` | object to append to `spec/modules.json` |
| `module.go` | `ModuleSpec` constructor and renderer |
| `registration.go` | the `BuiltinRegistry()` entry |
| `module.rs` | Rust mirror declaration and rule-registration note |
| `module_test.go` | minimal valid/rejected contract test |
| `README.md` | apply checklist and guardrails |

The `_scaffold/` prefix starts with `_`, so the Go tool ignores the directory: a skeleton cannot change a
build until a maintainer applies it. The generator refuses a duplicate `kind@schema`, a schema gap for an
existing kind, an unknown category or permission, and a rule name missing from either validator. It does
not edit `spec/modules.json`, `registry.go` or `src/cli.rs`; applying the skeleton stays a reviewed diff.

`check.py` verifies, and fails with the specific keys or names that drifted:

| check | fails when |
| --- | --- |
| `manifest` | identity, title, sdk, category, ports, references, derived calculators, lifecycle or rules violate the contract |
| `registration` | a `kind@schema` is declared but not registered in `BuiltinRegistry()`, registered twice, or registered without a manifest entry |
| `schema` | a kind's schemas are not the gap-free sequence 1..N |
| `permissions` | an action names an undeclared permission, or a declaration has no Go `Perm*` constant |
| `catalog` | `module-catalog-v1.md` lists another key set or another declared count |
| `parity` | Go `knownModuleRules` / `foundationPortTypes` / `foundationCategories`, Rust `ASTERDOC_RULES` / `types` / `categories`, or the rule dispatch arms disagree |
| `scaffold` | a `_scaffold/<kind>@<schema>/` directory is incomplete, disagrees with its fragments, or is already merged |

Tests for the tools build a minimal manifest + Go + Rust tree and assert both the passing and the failing
paths:

```sh
python3 scripts/sdk/test_sdk_tools.py
```

Module tests and the Go/Rust parity sample are still required:

```sh
cd prototypes/mygo-run
go test ./internal/asterdoc
go test . -run TestAsterDocGoRustParity
```

The `asterlab sdk new`, `sdk validate`, `sdk test` and `sdk manifest` commands are **not implemented** and
must not be reported as available. The manifest remains available through the Go registry (`ManifestJSON`)
and the shared JSON file; dynamic/third-party module loading does not exist.

## 8. Anti-patterns

- A new `kind` that is parsed by a second code path.
- Instrument parameters embedded in a free-text title.
- A calculator that writes a measured value.
- A template that copies old IDs without rewriting references.
- A module that declares a renderer but has no implementation.
- A document module that stores raw measurement arrays instead of referencing the record store.
- A "success" status inferred from display or animation rather than a Core record.
