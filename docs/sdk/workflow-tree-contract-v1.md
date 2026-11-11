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
| `blockVersionId`, `definitionRef` | exact ProtocolBlock version |
| `instruction` | frozen operation instruction |
| `parameters[]` | resolved `{key,label,value,unit}` |
| `unresolvedParameters[]` | explicit unresolved parameter names |
| `taskKey`, `status` | present on Run occurrence nodes |
| `composite`, `total`, `completed` | derived containment/progress |
| `children[]` | recursive child nodes |
| `issues[]` | local diagnostics |

Unresolved references, ambiguous paths, duplicate definitions and missing
providers are returned as `issues`; they are never guessed into children.

## Producer boundary

The projection is currently produced by the shared Go hierarchy engine
(`prototypes/mygo-run/internal/workflowtree`) through the app's hidden
`--inspect-tree` contract mode. The Rust CLI `workflow.inspect_tree` is a
transport adapter: it invokes that engine and returns the JSON; it does not
reimplement the algorithm.

The GUI uses the same engine. The Go tests remain the validation oracle. Moving
the producer into Rust Core while preserving this contract is the remaining
architecture step; the CLI/Canvas contract must not fork in the meantime.

`ASTERLAB_APP_BIN` can point the CLI at the app binary. Without it, the adapter
looks for `prototypes/mygo-run/target/asterlab-onevra.exe` relative to the
working directory, then `asterlab-onevra.exe` on `PATH`.
