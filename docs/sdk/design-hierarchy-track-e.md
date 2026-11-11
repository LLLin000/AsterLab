# Track E — Design Canvas hierarchy browsing

- Baseline: `feature/v2-run-workspace` @ `6608ab1`
- Commits: E0 `d4224f8`, E1 `dfd3ada`, E2/E3 `abb8c75`, F0 `f67c382`,
  F1 `c3382d4`
- Scope: read-only Design hierarchy, one-level Canvas drill-down, F0
  readability, and the read-only Agent query. No nested editing and no Agent
  write commands.
- Existing Run UI v1 right-side layout is unchanged.

## 1. E0 — contract and regression hardening

- `workflowtree.Tree` now has two independent contracts:
  - `Order` — `workflow.business_order`, used for display order;
  - `Precedes` — explicit `workflow.edges` only.
  No edge is synthesized from display order or authored position. Branched or
  parallel workflows keep their real dependency graph.
- Workspace selection persists the complete `TaskKey` first. `task_path` is a
  fallback only when exactly one frozen task carries that path; ambiguous paths
  select nothing.
- The real RT-qPCR Run projection test now reads a committed fixture at
  `testdata/rna-case/labworkbench.sqlite`, copies it to a temporary data
  directory, and fails if the fixture is missing. It no longer depends on the
  untracked local acceptance database or skips on a clean checkout.
- Historical Run/CCK-8 tests and the single-layer RNA rail remain compatible.

## 2. E1 — definition/use/occurrence projection

`workflowtree.BuildDesign(doc, provider)` builds the hierarchy without a Run:

- Design steps are **Use** nodes.
- Referenced immutable ProtocolBlock versions contribute **Definition** child
  steps. The block steps are never copied into the AsterDoc.
- Frozen Run tasks remain the only **Occurrence** layer; the existing
  `workflowtree.Build` overlays them on the same identity model.
- A step's reference resolves to an exact `block_version_id`. Paths locate
  nodes; version IDs and reference IDs validate identity.
- Two uses of the same block version keep separate root `UseID` and child
  paths. A newly created block version cannot change an old reference.
- Missing provider, missing block, duplicate definition paths, mixed
  task/children and cycle cases produce diagnostics instead of guessed
  children.

Core exposes a read-only `PreviewProtocolBlock(blockVersionID)` on
`DesignService`, backed by the existing block expansion. It performs no write
and creates no Run.

Tests:

```text
TestBuildDesignWithoutRun
TestBuildDesignRejectsMissingBlockProvider
TestBuildDesignReusesSameBlockVersionIndependently
TestBuildDesignKeepsOldVersionReference
TestProtocolBlockPreviewFixture
TestDesignHierarchyWithoutRunFixture
TestCanvasStepsCarryCompositeMarker
```

## 3. E2 — Canvas expand and enter

- The root Canvas still shows only the four top-level steps.
- A composite step card is marked `子流程 · N 步`; the existing step editing
  toolbar and inspector remain unchanged.
- Selecting the RNA extraction card shows a read-only actions row:
  - `展开预览` shows the first four real definition operations and the
    remaining count;
  - `进入子流程` focuses the complete child definition set.
- The child view reuses the existing `workflow.Editor` in read-only mode
  (`ExternalWrites=true`, `ReadOnly=true`) and the existing node cards. It has
  no add/delete/connect commands, no layout drag, and no second persistent
  graph.
- A `components.Breadcrumbs` path returns to the root. The root editor object
  is kept intact, so its pan, zoom, selection and saved node positions are
  restored.
- Workspace session stores `design_focus`, `design_preview`, and
  `design_subflow`; restart restores the selected root step, preview state and
  open subflow. These fields never enter AsterDoc.
- No nested editing command, shared block mutation, or Agent operation exists
  in this round.


## 3b. F0 — readability closure

- Preview is now a visible strip with a heading `预览 N / M 个定义操作 · 剩余 K`
  and four real operation cards joined by reading-order chevrons.
- Subflow cards show the frozen `Instruction` excerpt and a real parameter
  status (`无参数`, resolved values, or `N 个参数未解析`), never the generic
  `未填写参数`.
- The subflow uses a 3-column snake layout with a separate `Sequence` reading
  guide. Sequence lines are drawn from `Editor.Sequence`, not `Doc.Edges`, so
  they cannot be mistaken for execution dependencies. A compact
  `01 → … → 10` ribbon states the same reading order.
- Selecting a subflow operation opens a read-only dialog with the full frozen
  instruction, resolved parameters or unresolved parameter names, and the exact
  source ProtocolBlock version.
- Root and subflow canvases fit their content on first layout and after a
  window resize. On narrow windows the root inspector starts closed and can be
  reopened from the toolbar; the canvas keeps all four root steps visible.
- Screenshots at logical 1360×880 and 900×600:
  - `design-hierarchy-preview-1360x880.png`
  - `design-hierarchy-preview-900x600.png`
  - `design-hierarchy-subflow-1360x880.png`
  - `design-hierarchy-subflow-900x600.png`
  - `design-hierarchy-subflow-detail-1360x880.png`
- The existing root step editing commands remain available; the derived
  subflow remains `ExternalWrites=true` and `ReadOnly=true`.

## 4. E3 — evidence

| Evidence | Content |
|---|---|
| `evidence/design-hierarchy-root.png` | Design with no Run selected; four root steps; RNA card marked `子流程 · 10 步`; actions row. |
| `evidence/design-hierarchy-preview.png` | Local preview of four real definition operations plus `+6 个操作`. |
| `evidence/design-hierarchy-subflow.png` | Read-only RNA subflow with all 10 definition operations. |
| `evidence/design-hierarchy-subflow-restart.png` | Restart restores the open subflow. |
| `evidence/design-hierarchy-return.png` | Breadcrumb returns to the root with the RNA selection, preview and layout preserved. |
| `evidence/rtqpcr-tree-standard-regression.png` | Existing Run Tree remains intact after Track E. |

The screenshots are from the real 2576×1408 maximized app window with the
RNA/RT-qPCR fixture. The design root screenshot has no selected Run.

## 5. Verification

```text
go test ./...                                                        PASS
go test ./internal/workflowtree                                      PASS
go test -run 'TestDesignHierarchyWithoutRunFixture|TestProtocolBlockPreviewFixture|TestPathBoundRunTreeProjection' .  PASS
go run github.com/egoist/mygo/cmd/mygo native check modules/runs      PASS
```

## 6. Remaining limits

- Drill-down is one level. A nested ProtocolBlock inside the current block is
  shown as a composite card, but there is no second-level `进入` yet.
- The subflow is read-only; no nested step edit command exists. Agent
  `inspect_tree`/`expand` and write operations remain future work.
- Block preview uses empty root bindings. If a block requires root parameters,
  its definition steps are still shown but readiness may be false; that is a
  design-readiness concern, not a hierarchy guess.
- Root Canvas node positions remain the existing per-design layout. The
  subflow uses an auto grid and does not persist child positions.
- Only the subflow's parent and the root focus/preview are persisted; a
  selected node inside the subflow is not part of the session.
- The existing root step card tag/kind is unchanged; composite cards override
  the tag with `子流程 · N 步`.

## F1 — Agent hierarchy query

`workflow.inspect_tree` is available as a read-only Agent command. It returns
the same contract used by the Canvas: `roots[]`, separated `order`/`precedes`,
Definition/Use/Occurrence identity, instructions, parameters, optional
TaskKey/status, and issues. The Rust CLI does not reimplement `BuildDesign`; it
invokes the shared Go hierarchy engine through the app's hidden
`--inspect-tree` mode. The remaining architecture step is moving that producer
into Rust Core while preserving `docs/sdk/workflow-tree-contract-v1.md`.
