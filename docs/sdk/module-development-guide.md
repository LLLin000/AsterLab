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
  "kind": "lab.example",
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

- [ ] `kind` is stable and lowercase.
- [ ] `schema` is a new version if any old field meaning changes.
- [ ] Each input has a Foundation type and a description.
- [ ] Quantities declare `dimension`.
- [ ] References declare `target`.
- [ ] Derived outputs declare `origin: "derived"` and a versioned `calculator`.
- [ ] Actions declare a permission that is present in `permissions`.
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

## 7. CLI/tooling status

The current development commands are:

```sh
cd prototypes/mygo-run
go test ./internal/asterdoc
go test . -run TestAsterDocGoRustParity
```

The target SDK commands below are **not implemented** and must not be reported as available:

```text
asterlab sdk new
asterlab sdk validate
asterlab sdk test
asterlab sdk manifest
```

The manifest is currently available through the Go registry (`ManifestJSON`) and the shared JSON file. A stable CLI surface is a remaining SDK tooling gap.

## 8. Anti-patterns

- A new `kind` that is parsed by a second code path.
- Instrument parameters embedded in a free-text title.
- A calculator that writes a measured value.
- A template that copies old IDs without rewriting references.
- A module that declares a renderer but has no implementation.
- A document module that stores raw measurement arrays instead of referencing the record store.
- A "success" status inferred from display or animation rather than a Core record.
