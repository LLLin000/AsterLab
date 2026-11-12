# Typed Step Module Binding v1

- Date: 2026-11-11
- Status: implemented for `step@2` bindings and `step@3` execution/materialization; `step@1`/`step@2` contracts are unchanged.
- Scope: Step → exact ProtocolBlock version → frozen Run task.

## 1. Syntax

`step@2` is the existing `step@1` shape plus an optional `uses` list:

```yaml
kind: step
schema: 2
id: step-uv
title: GelMA UV 固化
assay: assay-uv
path: root/uv
uses:
  - ref: uv-1
    role: instrument
  - ref: dilution-01
    role: calculator
```

`uses[].ref` is a stable module ID in the same AsterDoc. `uses[].role` is checked against the target module's declared capabilities through the shared registry. Built-in roles are `instrument`, `calculator`, `labware`, `material`, `sample` and `other`; a namespaced role such as `acme:imaging` is accepted only when the target module declares that exact capability. Duplicate refs, missing refs and role/capability mismatches are rejected with the Step and Ref in the message.

`step@1` remains readable and keeps its old meaning. A document must opt into `step@2` to declare bindings; there is no implicit upgrade.

## 2. Compiler projection

The compiler copies each step's `uses` into the frozen Design:

```json
"mygo": {
  "stepTasks": [
    {"stepId":"step-uv","path":"root/uv","assayId":"assay-uv","timepoint":"D1"}
  ],
  "stepUses": {
    "step-uv": [
      {"ref":"uv-1","role":"instrument"},
      {"ref":"dilution-01","role":"calculator"}
    ]
  }
}
```

The `stepUses` map is keyed by stable Step ID, not title or order. `stepTasks` keeps the path/assay/timepoint binding used to resolve a Run task.

## 3. Run task resolution

- A task resolves its design Step through the explicit `stepTasks` entry matched on `step_path` + assay + timepoint, or an unambiguous path-qualified entry.
- The Run viewer reads the frozen `stepUses[designStepID]` and filters the frozen `asterlab.modules` by the referenced IDs.
- If a task has no design step or no uses, it says `该任务未绑定 SDK 模块`; it does not show every module in the document.
- The global document view remains the place to inspect all modules; it is not the task view.

Timepoint is not task identity. Multiple tasks may legitimately share D1; the Step ID / path is what disambiguates them.

## 4. Current limitations

- Built-in role-to-capability mappings live in the shared manifest; adding a new built-in role is a deliberate registry change. Namespaced extension roles are supported by capability declaration.
- The AsterDoc Step Inspector exposes `步骤资源`: existing bindings show their role and a remove action; SDK modules in the document can be added with an inferred role and the role can be changed before saving. Saving uses the existing `update_module` semantic command and creates a new Revision.
- `step@2` does not yet carry a separate Task Template ID; task identity currently uses the frozen step path + scope + timepoint. A future Task Template/Instance layer can extend this without changing `step@1`.
- Actual instrument instances, calibration and device state remain outside `instrument_profile`.

## 5. Explicit execution source (`step@3`)

`step@3` requires `execution: executable|organizational`. No earlier schema
gains an implicit organizational meaning.

- `executable` requires an assay and exactly one `uses` reference whose
  reference module has `view: protocol_block`. Its exact version must occur in
  that assay's frozen ProtocolVersion.
- `organizational` has no assay, path, timepoints, checks, tasks, or module
  bindings. It remains visible in workflow structure and creates no Run task.
- `step@1` and `step@2` remain executable. A legacy step without a ProtocolBlock
  is reported as `STEP_NOT_MATERIALIZED`; it is never converted into an empty
  task. Since `step@1` cannot declare `uses`, binding it requires an explicit
  schema update.
- `workflow.Nodes` retains planned nodes; `stepTasks` records executable Step
  bindings only. Run Task occurrences still expand from the frozen
  ProtocolVersion. Core repeats the exact-version check in `asterdoc.assess`
  and before Run creation. Frozen Runs and their event identities are not
  backfilled.
- The check does not create a second inline-task execution path. A future
  one-step authoring action can create and bind a minimal ProtocolBlock through
  semantic commands.

