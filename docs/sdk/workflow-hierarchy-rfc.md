# Workflow hierarchy RFC — v1 read-only projection

- Status: read-only projection + Run tree navigation implemented; Canvas/Agent proposal only
- Baseline: `feature/v2-run-workspace` after Track A
- Evidence: `internal/workflowtree/tree_test.go`, `docs/sdk/rna-extraction-case/rtqpcr-parent-path-bound.asterml`

## 1. What the RNA/RT-qPCR test proved

- ProtocolBlock `block_use` recursion is already native: the RNA block expands to
  10 operations, and the parent RT-qPCR block expands to 13 steps with paths
  `root/rna-extraction/01-collect` … `root/qpcr`.
- Exact child block versions are pinned by the parent `block_use`; a new RNA
  block version does not rewrite an existing parent or frozen Run.
- The missing piece is not an operation engine. It is a stable containment
  projection and identity contract between the AsterDoc parent step and the
  expanded protocol tasks.
- The original case used `rtqpcr/...` design paths and `root/...` protocol paths,
  so no stable parent link existed. `rtqpcr-parent-path-bound.asterml` is the
  path-bound variant used by the projection test.

## 2. Identity model

| Layer | Identity | Source | Rule |
|---|---|---|---|
| Definition | Exact ProtocolBlock version / protocol version | `block_use.block_version_id`, protocol version | Immutable; never a title |
| Use | AsterDoc step ID + step path + `uses` reference | frozen AsterDoc | One design position; may be reused in another design |
| Occurrence | Frozen Run task key + execution event `task_key`/`protocol_step_path` | frozen Run + Core events | One execution instance; never shared across Runs |

The projection exposes `DefinitionRef`, `Node.ID`/`Path` (Use), and child
`TaskRef.Key` (Occurrence). It never synthesizes identity from a title.

## 3. `contains` and `precedes`

- `contains` is the parent step path plus the expanded child task paths:
  - a parent step path `root/rna-extraction` contains tasks whose path starts
    with `root/rna-extraction/`;
  - a leaf step path `root/rna-qc` contains the exact task path.
- `precedes` is the workflow edge list (`workflow@1.edges`), or the authored
  step order when no edges exist. It is independent of containment.
- A composite step does not get a synthetic task of its own. Its status is
  aggregated from the child tasks.

## 4. Binding contract v1 (frozen-version identity)

1. An executable AsterDoc Step references exactly one `reference` module with
   `view: protocol_block`; its `ref` is the exact ProtocolBlock version ID.
2. The Step's assay ProtocolVersion must contain that exact block version.
   Core checks this in `asterdoc.assess` and again before Run creation.
3. AsterDoc Step paths and ProtocolBlock-internal paths are separate identities.
   Run tasks link through the explicit Step/assay/timepoint mapping; paths and
   titles are never guessed as substitutes.
4. A task whose source block does not match the referenced version is reported,
   not assigned to that composite.
5. `step@1` and `step@2` remain executable; absence of a block never implies
   organizational semantics. `step@3.execution` explicitly declares
   `executable` or `organizational`; organizational steps produce no Run task.

`step@3` versions execution semantics without changing `step@1`/`step@2`. A
future nested-subworkflow reference would require a later schema version.

## 5. Status aggregation

- `completed` only when every child task is `completed`.
- `blocked` if any child is blocked; `current` if any child is in progress;
  `pending` otherwise.
- `unresolved` means an executable step has no matching frozen task. An explicit
  `organizational` step has status `organizational` and total 0.
- The parent counts are `Completed/Total`; they are derived, not stored.

## 6. Freeze and validation

- A Step reference pins the exact ProtocolBlock version; the Run freezes the
  compiled Design revision and expanded ProtocolVersion task paths.
- Core rejects an executable Step unless its exact ProtocolBlock version is
  present in that assay's frozen ProtocolVersion. Organizational nodes create
  no task and are not inferred from a missing block.
- The hierarchy projection reports duplicate step paths, missing/mismatched
  composite definitions, task ambiguity/orphans, and `precedes` cycles.
- The projection is read-only. It does not write, migrate, or rewrite a frozen
  Run.

## 7. Agent read-only surface

Current read-only sources:

- `run.workspace`: `tasks[].step_path`, `steps[].step.path`, event
  `task_key`/`protocol_step_path`;
- `protocol.block.preview`: expanded child steps and `block_version_ids`;
- `asterdoc.inspect`: step `uses` references and module identity.

Proposed (not implemented):

- `run.workspace.tree` or `inspect_tree`: returns the same projection:
  `{definitionRef, use:{stepId,path}, occurrence:{taskKey}, contains, precedes,
  status:{completed,total,state}, issues}`.
- `expand(useId)` and `validate_composition` are read-only. `insert_child` and
  `bind_subflow` are future write operations and are not part of v1.

## 8. Implementation evidence

`internal/workflowtree/tree.go` builds the projection from frozen AsterDoc
steps and `runs.RunTask` paths. `internal/workflowtree/tree_test.go` proves:

- 4 parent steps, RNA extraction with 10 child operations, QC/RT/qPCR leaves;
- `contains` by path prefix/exact path, `precedes` from workflow edges;
- parent aggregation `5/10 current`, blocked qPCR, completed QC;
- ambiguity, orphan, and composite-without-definition rejection.

```text
go test ./internal/workflowtree   PASS
go test -run TestPathBoundRunTreeProjection .   PASS
```

Track D hardening (unique ownership, recursive trie, source-block validation,
business order, structured task key, real frozen Run proof) is documented in
`docs/sdk/workflow-hierarchy-track-d.md`. The Run rail now consumes this
projection through Onevra's existing collapsible `components.Tree`; single-layer
runs keep the legacy rail.

## Track E extension

Design Canvas now consumes the same hierarchy without a Run: AsterDoc steps are
Use nodes, immutable ProtocolBlock versions contribute Definition children,
and frozen Run tasks are Occurrence overlays. The Canvas can preview and enter
one level read-only; see `docs/sdk/design-hierarchy-track-e.md`.

## Read-only Agent query

`workflow.inspect_tree` exposes the same hierarchy contract to the Agent CLI,
with optional Run overlay. The CLI now computes it in Rust Core
(`crates/asterlab-core/src/workflow_tree.rs`); the GUI still uses the Go
hierarchy engine, and the Rust test compares both against the shared RNA
evidence. See `docs/sdk/workflow-tree-contract-v1.md`.
