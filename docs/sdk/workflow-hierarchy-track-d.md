# Track D — hierarchy projection hardening and Run tree navigation

- Baseline: `feature/v2-run-workspace` @ `3f0e358`
- Scope: D1 read-only projection hardening, D2 Run rail tree, D3 acceptance.
- No nested Canvas, no Agent write command, no new execution-event write path.
- Run UI v1 right-side layout is unchanged.

## 1. D1 — projection hardening

- **Unique ownership first.** Every frozen task is matched before any node is
  built. Zero matches -> `Orphans`; more than one match -> `Ambiguous` and the
  task is not assigned. No silent longest-path choice.
- **Recursive containment.** A task path trie builds arbitrary-depth
  composites (`root/rna-extraction/purification/01-wash`), not only
  Step -> Task. `Node.Composite` and `Tree.HasComposite` drive the UI.
- **Block identity validation.** `RunTask.SourceBlockVersionID` is projected
  from `run_workspace.steps[].step.source_block_version_id`. Direct child tasks
  under a composite must match the referenced `block_version_id`. All
  descendants must share one Protocol version. Path-prefix use_id and exact
  path are validated; title is never used.
- **Authoritative order.** `precedes` uses `workflow.business_order` first,
  then explicit `edges`, then authored order. `contains` is the tree itself.
- **Structured task key.** The fallback parses `{ "path": ... }` with
  `encoding/json`, not a substring search.
- **Legacy data.** Missing identity stays orphan/unresolved; no synthesized
  parent.

Tests (`internal/workflowtree`):

- 4 parents / 10 RNA children / 3 leaves, 5/10 current, QC completed,
  RT pending, qPCR blocked;
- nested three-level containment;
- wrong child block version;
- duplicate path, ambiguity, orphan, composite without definition;
- real frozen Run test `TestPathBoundRunTreeProjection` in the app package
  reads run `ad9b452e-f8ea-404b-9a2a-7124f9403b02`, design
  `d5525203-56bb-4b52-82c4-0e05f0ed2ff8`, AsterDoc revision 1, and asserts
  4 roots, 10 RNA children, unique task ownership, and child source block
  `c23df8e1-...`.

Known limit: the frozen task model exposes the expanded child block version,
but not the parent `block_use` parent version/use_id. The projection validates
the path-bound use_id, exact child version, and protocol consistency; the Core
freeze remains authoritative for the parent block_use.

## 2. D2 — Run tree navigation

- `RunWorkspaceViewModel.Tree` is set only when the projection is valid and
  `HasComposite()` is true. Otherwise the existing single-layer rail is kept.
- Tree rail:
  - 4 top-level steps;
  - RNA extraction expands/collapses and shows 10 child operations;
  - parent rows show `Completed/Total` and aggregated status;
  - leaf parents (RNA QC, RT, qPCR) select their single child operation;
  - child selection uses the existing right-side Run UI v1 detail, checks,
    recording, and history;
  - selected child auto-scrolls into view;
  - blocked qPCR shows a danger dot; RNA extraction shows `5/10` current.
- If the projection has diagnostics, the rail shows a warning and falls back to
  the legacy navigation; no incorrect ownership is rendered.
- Session persists `task_path` and the tree `collapsed` map, so restart
  restores the selected child and expansion state.
- Single-layer RNA extraction still uses the legacy 10-card rail; the tree is
  not shown.

## 3. D3 — acceptance evidence

| Screenshot | Content |
|---|---|
| `evidence/rtqpcr-tree.png` | Four top-level steps; RNA extraction expanded with 10 children; `5/10` current; qPCR blocked; selected 01. |
| `evidence/rtqpcr-tree-wash2a.png` | Wash II first selected; right-side frozen instruction, check, recording and history; tree parent progress. |
| `evidence/rtqpcr-tree-restart.png` | Restart restores the same tree, selected Wash II first, expansion and parent status. |
| `evidence/run-09-elute-regression.png` | Single-layer RNA extraction still uses the legacy 10-card rail; no tree regression. |

Commands:

```text
go test ./...                                      PASS
go test ./internal/workflowtree                    PASS
go test -run TestPathBoundRunTreeProjection .      PASS
go run github.com/egoist/mygo/cmd/mygo native check modules/runs   PASS
```

Real run: `ad9b452e-f8ea-404b-9a2a-7124f9403b02`; design
`d5525203-56bb-4b52-82c4-0e05f0ed2ff8`; AsterDoc revision 1; 13 frozen tasks.
Events: 01–05 completed, 06 in progress, QC completed, qPCR blocked, RT
pending.

## 4. Remaining limits

- Parent `block_use` version/use_id are not exposed in the frozen task model;
  nested block definitions have no direct `DefinitionRef` in the projection.
- The tree is a Run rail navigation change only; Canvas expand/drill-down and
  Agent `inspect_tree`/write commands remain future work.
- Sample-level status matrix is still absent.
- No parent task can be completed independently; parent status is derived.
