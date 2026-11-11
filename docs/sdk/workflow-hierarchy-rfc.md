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

## 4. Binding contract v1 (read-only, path-bound)

1. The AsterDoc parent step path MUST equal the protocol step path:
   - composite: `root/<block_use.use_id>`;
   - leaf: the exact protocol step path.
2. A composite step MUST have a `uses` reference whose `ref` equals the exact
   child `block_version_id`. If more than one `block_use` uses the same child
   version, the binding is ambiguous and must be rejected.
3. Child tasks are matched by exact path or `parentPath + "/"`. A task that
   matches more than one step, or no step, is ambiguous/orphan; it is reported,
   never guessed into a parent.
4. Titles are display text only. They are never used for matching or identity.
5. A parent step that mixes an exact task and child tasks is rejected.

This contract requires no new schema. A future `step@3` may add an explicit
`subworkflow_ref { block_version_id, use_id }`; it must not change `step@2` in
place.

## 5. Status aggregation

- `completed` only when every child task is `completed`.
- `blocked` if any child is blocked; `current` if any child is in progress;
  `pending` otherwise.
- `unresolved` when a step has no matching frozen task.
- The parent counts are `Completed/Total`; they are derived, not stored.

## 6. Freeze and validation

- The parent `block_use` pins the exact child version; the Run freezes the
  compiled design revision and all expanded task paths.
- Validation rejects:
  - duplicate step paths;
  - a composite with no explicit child definition reference;
  - a composite with no expanded child tasks;
  - a step with no task;
  - a task matching multiple steps;
  - an orphan task;
  - a `precedes` cycle.
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
