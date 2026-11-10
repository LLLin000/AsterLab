# AsterDoc / AsterML module contract (prototype)

Status: working prototype in `prototypes/mygo-run/internal/asterdoc`. Not yet the storage model for every
Design/Protocol/Run/Review surface.

## 1. Source syntax

An AsterDoc is ordinary Markdown. A fenced block whose language is `asterlab` declares one module:

````markdown
# D3 · CCK-8

```asterlab
kind: plate96
id: plate-cck8-d3
schema: 1
assay: cck8
groups:
  control: [A1, A2, A3]
  material: [B1, B2, B3]
```
````

Rules:

- `kind` selects a registered module implementation.
- `schema` selects its version. Unknown kind or version is an error; there is no generic editable fallback.
- `id` is stable inside the document. Duplicate IDs are rejected.
- The YAML fence body is the authored source. Parse never rewrites prose.
- An edit rewrites only that fence body and reparses the whole document; prose outside the fence is preserved.
- 96-well coordinates are `A1`–`H12`; one well cannot belong to two groups.

Plain Markdown (headings, lists, tables, images, formulas, links) stays Markdown.

## 2. Two stores

- **Document store**: AsterML source plus its parsed AST. Design text and module configuration live here.
- **Record store**: Run events, measurements, audit entries, attachments. Rust Core is the only writer.

The AST is a parse cache, never a second fact store. A module can read facts through explicit references, but it
never copies raw measurements into the document. A `reference` module points at an existing object:

```asterlab
kind: reference
id: ref-batch-003
schema: 1
ref: readout_batch:batch-003
view: plate-readout
```

## 3. Module registry

Go contract (`internal/asterdoc/registry.go`):

```go
type ModuleSpec struct {
    Descriptor
    Validate func(data map[string]any) error
    Render   func(v *View, c *ui.Context, data map[string]any)
}
```

`View` carries the render context: `Mode`, `RunID`, `DocumentRevision`, and an explicit `Allowed` permission set.
`Mode` and `ReadOnly` are presentation context; they never grant writes.

A `Descriptor` is the agent-facing contract:

```json
{
  "kind": "plate96",
  "schema": 1,
  "title": "96 孔板",
  "modes": ["design", "protocol", "run", "notebook", "review"],
  "inputs": [
    {"name": "id", "type": "string", "required": true, "description": "..."},
    {"name": "groups", "type": "well-map", "required": true, "description": "..."}
  ],
  "outputs": [
    {"name": "wells", "type": "well-map", "description": "..."},
    {"name": "assigned", "type": "int", "description": "..."}
  ],
  "actions": [
    {"id": "assign_wells", "title": "...", "permission": "document.edit", "writes": ["groups"]}
  ]
}
```

`internal/asterdoc/spec/modules.json` is the single module definition. Go embeds it and derives
`BuiltinRegistry().ManifestJSON()`; Rust includes the same file at compile time and validates saved documents from it.
Renderer implementations stay per language, but the contract, required fields, actions, permissions and validation
rules have one source. `TestAsterDocGoRustParity` pins that both sides accept and reject the same sample sources.
Foundation v1 types, typed references, the named UCUM subset and the twelve-dimension module contract are specified in
[`docs/sdk/`](sdk/README.md); the machine shapes are in
[`foundation-v1.schema.json`](../prototypes/mygo-run/internal/asterdoc/spec/foundation-v1.schema.json).

## 4. Commands and permissions

- `document.edit` — change document source (Design/Notebook editing).
- `run.readout` — append measured values through Core's readout transaction.
- `notebook.append` — append a notebook revision.

A module action names exactly one permission and the fields it writes. The host checks the permission before the
action; a module must also check it in its own `Assign`-style API. `ReadOnly` may hide controls, but is not the
authorization boundary.

## 5. Renderers

The module is a Node in the document, and every output is a renderer over the same AST/source:

- native interactive renderer (MyGo `markdown.Options.CodeBlock` hook);
- notebook/reading renderer (same hook, no edits);
- future PDF/static renderer (same module registry, a `Static` capability instead of `Render`).

A PDF must pin: document revision, run revision, batch IDs, calculation version and export time. Missing references
or unsupported schema versions fail the export visibly; they are never skipped.

## 6. Adding a module

1. Implement `Validate` and `Render` and a `Descriptor` with typed inputs, outputs, modes and actions.
2. Register it in `BuiltinRegistry()` (or a caller-owned `Registry` for tests/plugins).
3. Add round-trip, manifest and permission tests.
4. Do not add a second syntax. Every module is a fenced `asterlab` block with `kind`, `id`, `schema`.

Current shipped modules: `plate96@1`, `plate@1`, `reference@1`, `experiment@1`, `group@1`, `assay@1`, `step@1`,
`workflow@1`, `dilution@1`, `instrument_profile@1`. The reference renderer is a static label; resolution to a
batch/plate view is not implemented yet. Dilution outputs are derived (`dilution.v1`) and are never written as Run
facts. Instrument profiles declare capabilities only; they do not connect to hardware.

## 7. Core document store (Slice A)

Core now owns AsterDoc revisions as first-class records:

- `asterdoc_version` records are children of the design record, with a monotonic `revision`, immutable `source`,
  `source_hash` (SHA-256), `schema_version`, `compiler_version`, `created_at`, optional `actor_id`, and optional
  `operation_id`.
- `SaveAsterDoc` validates the source in Rust before writing, checks `expected_revision`, and commits the revision in
  one immediate transaction. The same `operation_id` with the same source is idempotent; a different source conflicts.
- `LoadAsterDoc` returns the latest revision and an empty revision 0 when the design has none.
- Core supports `plate96@1` and `reference@1` today; both sides read `spec/modules.json`. Go keeps an immediate parser
  for rendering, but Core is the save authority.
- The design record remains the document root, matching the existing record/foreign-key model rather than adding a
  parallel table hierarchy. Versions are immutable because Core exposes no update/delete path.

Not yet implemented: Design projection, Run freeze, CLI routes, GUI write-through. The Notebook 文档 mode still saves
through `SaveNotebook`; do not treat it as a Core document revision.

## 8. Compiler and Run freeze (Slice B/C, CCK-8)

`compile_asterdoc_to_design(source)` is implemented in Rust Core:

1. Parse and validate the shared module spec and cross-module references.
2. Require exactly one `experiment` module.
3. Project `experiment`, `group`, `assay`, `step`, `workflow`, and `plate` into the existing Design JSON:
   root scientific fields, `group_specs`, `assays`, and `mygo.workflow/stepAssays/stepTasks/assays`.
4. Validate the projection as a real Design; nothing is written by compile itself.

`CreateRunFromAsterDoc(design_id, document_revision, operation_id, title)` runs in one immediate transaction:
loads the exact immutable revision, compiles it, writes the derived Design revision + design_version, creates the Run,
and stores `asterdoc {design_id, revision, source_hash, schema_version, compiler_version}` on the Run. Existing Run and
Design audit/preflight behavior stays in the same transaction. Same operation + same revision/hash is idempotent;
another revision conflicts.

Once a design has an AsterDoc version, legacy `SaveDesign` is refused so the derived Design cannot become a second
writable source. Design edits must create a new AsterDoc revision and a new Run.

The Run workspace exposes `asterdoc`; Protocol reads the exact frozen revision through `LoadAsterDocAt`, verifies the
source hash, and never falls back to the latest document or live Design.

Tests: `asterdoc_compile_test.go` (projection fields) and `asterdoc_freeze_test.go` (atomic freeze, restart, historical
isolation, legacy-write refusal).

Not yet implemented: Workflow canvas command integration for nested/write-through designs, full Plate Editor undo and
colored/annotation layers, Notebook frozen-document view, preflight confirmation dialog for CreateRun, PDF/static
renderer, evidence/material/container-plan modules, RT-qPCR/CFU acceptance, real-window evidence for the new design
surface. The workflow canvas now supports wheel/pinch pan-zoom, empty-canvas marquee, Shift/Cmd multi-select, batch move and
multi-delete; the enlarged plate supports mouse marquee selection. These were exercised in headless tests. A real-window
CCK-8 walkthrough at 900×600 covered step selection, draft edit/undo/redo, save, same-project Protocol binding, Run
creation and the Run workspace; the shell minimum is now 900×600.

## 9. Semantic edit commands (Phase 3-A)

`ApplyAsterDocCommand(design_id, expected_revision, operation_id, actor_id, command)` applies one semantic edit to the
exact revision, then saves through `save_asterdoc_version`; there is no second write path. Supported commands:

- `add_module`, `remove_module`, `update_module`
- `add_step` / `remove_step` (step module plus workflow steps/business_order/edges atomically)
- `assign_wells`, `clear_wells`
- `connect_steps`, `disconnect_steps`

Only the targeted `asterlab` fence body is rewritten; prose and other modules are preserved. `expected_revision` rejects
stale edits; same operation + command is idempotent, same operation with another command conflicts. Core re-parses,
revalidates and recompiles after the patch.

CLI routes: `asterdoc.load`, `asterdoc.save`, `asterdoc.command`, `asterdoc.compile`, `asterdoc.create_run`.

## 10. Document-backed Design GUI (Phase 3-B partial)

When the selected design has an AsterDoc revision, Design renders the document surface instead of the legacy editor:
- frozen/live document revision and native modules;
- plate group selection and well assignment issue `assign_wells`/`clear_wells` commands through Core;
- "创建 Run" calls `CreateRunFromAsterDoc` and opens the frozen Run.

The Design surface has 流程 / 文档 tabs. The flow tab builds the existing native canvas from AsterDoc step/workflow
modules; add/delete/connect and the step inspector (title, assay, protocol path, timepoints) issue semantic commands and
reload the immutable revision. Node positions remain local layout state and are not written to AsterML. Inline plate
modules are previews only; every format is edited in the enlarged floating plate, which has a toolbar below it (zoom,
groups, batch assign/clear, close) and no inspector panel.

Legacy `SaveDesign` remains refused for document-backed designs.

## 11. Current prototype boundary

Implemented in Notebook as the **文档** mode:

- Markdown + native plate module, group selection, click-to-assign wells, source rewrite.
- Save appends a `stage=notebook` event through `RunService.SaveNotebook`.
- Registry manifest, explicit permissions, duplicate-ID and duplicate-well validation.

Not implemented yet:

- Core document store and document revision; the notebook event is not a Design Revision.
- Freezing an AsterDoc into a Run and rendering Run/Protocol/Review modes from it.
- Reference resolution to batches/plates.
- Static/PDF renderer.
- Milkdown or web-based editing.
