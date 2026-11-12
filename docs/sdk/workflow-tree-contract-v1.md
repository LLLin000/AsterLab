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

The CLI `workflow.inspect_tree` is produced directly by Rust Core
(`crates/asterlab-core/src/workflow_tree.rs`). The adapter loads the AsterDoc
revision, ProtocolBlock previews, and frozen Run workspace, then calls the
Core projection; it does not launch the desktop app and does not need
`ASTERLAB_APP_BIN`.

The Go hierarchy engine (`prototypes/mygo-run/internal/workflowtree`) remains
the GUI Canvas/Run-rail producer. Both producers must preserve this contract
until the GUI migrates. The shared evidence files under
`docs/sdk/rna-extraction-case/evidence/` are checked by the Rust test
`workflow_inspect_tree_matches_shared_go_evidence`, so a CLI/Core change cannot
silently fork the shape while the Go oracle is still authoritative.
