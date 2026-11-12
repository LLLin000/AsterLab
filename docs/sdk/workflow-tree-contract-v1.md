# Workflow hierarchy contract v1

`workflow.inspect_tree` is the read-only hierarchy query shared by the Design
Canvas and the Agent CLI.

## Input

```json
{"design_id":"<id>","revision":1,"run_id":"<optional-run-id>"}
```

`revision` is optional; omitted means the latest AsterDoc revision. `run_id`
adds the frozen Run occurrence overlay. The command never writes and never
creates a Run.

When `run_id` is supplied the frozen Run is the sole authority:

- `design_id` must equal the Run's owning design, else the query is rejected.
- If `revision` is supplied it must equal the Run's frozen AsterDoc revision,
  else the query is rejected with a version conflict.
- Definition instructions and parameters come from the frozen AsterDoc and the
  ProtocolBlock versions it referenced at freeze time. The latest Design is
  never read to supplement an old Run.

## Output

```json
{
  "designId": "...",
  "revision": 1,
  "runId": "...",
  "design": false,
  "roots": [Node],
  "order": ["step-id"],
  "precedes": {"step-id": ["next-step-id"]},
  "issues": []
}
```

`design=true` means no Run overlay. `order` is the business display order.
`precedes` is the explicit `workflow.edges` dependency graph; neither is
derived from the other.

### Node

| Field | Meaning |
|---|---|
| `id` | stable node ID; occurrence nodes use the TaskKey |
| `title`, `path` | display title and authoritative path |
| `kind` | `step` or `task` |
| `identity` | `use`, `definition`, or `occurrence` |
| `useId` | design/block-use location |
| `blockVersionId`, `definitionRef` | exact ProtocolBlock version backing the operation |
| `protocolVersionId` | Protocol version of the frozen task/design context |
| `instruction` | frozen operation instruction |
| `parameters[]` | resolved `{key,label,value,unit}` |
| `unresolvedParameters[]` | explicit unresolved parameter names |
| `taskKey`, `status`, `assayId`, `timepoint` | present on Run occurrence nodes |
| `assayId`, `timepoint` | execution scope of a Run occurrence |
| `composite`, `total`, `completed` | derived containment/progress |
| `children[]` | recursive child nodes |
| `issues[]` | local diagnostics |

Unresolved references, ambiguous paths, duplicate definitions and missing
providers are returned as `issues`; they are never guessed into children.

Definition/Use/Occurrence association is not path-only. Run enrichment matches
`path + assayId` inside the same frozen revision; two definition nodes at the
same key produce an `identity is ambiguous` issue and copy no metadata.

## Producer boundary

Rust Core (`crates/asterlab-core/src/workflow_tree.rs`) is the only producer.
The CLI adapter and the MyGo native bridge share
`workflow_tree_for_db`, which loads the exact AsterDoc revision, ProtocolBlock
previews, and frozen Run workspace before calling Core. For a Run, the adapter
also rejects a stored `asterdoc.source_hash` that does not match the frozen
hash; it never substitutes the latest revision.

The GUI consumes the same JSON through generated
`DesignService.WorkflowTree(designID, revision)` and
`RunService.WorkflowTree(runID)`. The hidden `--inspect-tree` mode also calls
those methods. Go only decodes the contract for rendering.

`prototypes/mygo-run/internal/workflowtree` remains temporarily as a parity
oracle for tests (`BuildViewModel` without a `Projection` and
`inspectTreeFor`). Those Go paths are test-only and must be retired once parity
coverage is complete. The shared evidence files under
`docs/sdk/rna-extraction-case/evidence/` are checked by the Rust test
`workflow_inspect_tree_matches_shared_go_evidence`.

Identity conflicts are rejected, not resolved by precedence: when a Run task
records a `DesignStepID`, its path match and ID match must name the same
non-organizational step; otherwise the task is reported as ambiguous and is
not assigned.
