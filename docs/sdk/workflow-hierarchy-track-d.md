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
- Tree rail uses Onevra's existing collapsible tree component
  (`components.Tree` + `TreeItemWithLeading`), not a hand-rolled row tree:
  - standard disclosure arrow, indentation, focus ring, keyboard
    Up/Down/Home/End/Left/Right, and scroll-into-view;
  - parent rows use a folder mark and `Completed/Total` in the label;
  - child rows use a status dot and the frozen human title;
  - 4 top-level steps; RNA extraction expands/collapses and shows 10 child
    operations;
  - leaf parents (RNA QC, RT, qPCR) select their single child operation;
  - child selection uses the existing right-side Run UI v1 detail, checks,
    recording, and history;
  - blocked qPCR shows a danger dot; RNA extraction shows `5/10` current.
- Parent rows only expand/collapse; they are never selected as completable
  tasks. Child rows select the existing detail surface.
- If the projection has diagnostics, the rail shows a warning and falls back to
  the legacy navigation; no incorrect ownership is rendered.
- Session persists `task_path` and the tree `collapsed` map. Arrow, keyboard,
  double-click and row toggles all update the expansion fingerprint, so restart
  restores the selected child and expansion state.
- Single-layer RNA extraction still uses the legacy 10-card rail; the tree is
  not shown.

## 3. D3 — acceptance evidence

| Screenshot | Content |
|---|---|
| `evidence/rtqpcr-tree-standard.png` | Four top-level steps; RNA extraction expanded with 10 children; `5/10` current; qPCR blocked; selected 01. |
| `evidence/rtqpcr-tree-standard-click-wash2a.png` | Wash II first selected by clicking the tree; right-side frozen instruction, check, recording and history. |
| `evidence/rtqpcr-tree-collapsed.png` | Parent row collapsed; only the four top-level steps remain; no parent completion action. |
| `evidence/rtqpcr-tree-collapsed-restart.png` | Restart restores the collapsed branch. |
| `evidence/rtqpcr-tree-standard-restart.png` | Restart restores the selected Wash II first and the expanded tree. |
| `evidence/run-09-elute-regression.png` | Single-layer RNA extraction still uses the legacy 10-card rail; no tree regression. |

Commands:

```text
go test ./...                                      PASS
go test ./internal/workflowtree                    PASS
go test ./internal/runworkspace                    PASS
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
- The existing tree component owns keyboard/focus; row click is intentionally
  expand-only for parents, while double-click and the disclosure arrow also
  toggle in the component.
- Sample-level status matrix is still absent.
- No parent task can be completed independently; parent status is derived.
