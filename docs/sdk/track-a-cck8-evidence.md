# Track A: CCK-8 real-window evidence

- Date: 2026-11-11
- Scope: real MyGo window driven through `mygo-automation`; data dir `prototypes/mygo-run/target/e2e-data` (a disposable copy, not the user's live database).
- Status: first CCK-8 vertical loop exercised through batch readout, task completion, Review summary and run log. This is **not** a claim that every product acceptance item passes.

## 1. Frozen design

- Design: `234c5228-15b3-4407-b4c9-2fd75b17ee8d`
- AsterDoc revision frozen into the tested Run: `6`
- Design version: `a3157b3f-bab5-4b4b-b9b8-cb848d67391d`
- Frozen design has 4 planned wells:
  - `A1` Control-1 / `g-control`
  - `A2` Control-2 / `g-control`
  - `B1` Material-1 / `g-material`
  - `B2` Material-2 / `g-material`
- Run: `36d44741-a0b9-4ae8-89cd-85a6b45363db` (`CCK-8 run`)
- Run task list: `D1 root/seed`, `root/dose` (unresolved timepoint), `D1 root/read`.

The duplicate-D1 warning remains: `root/seed` and `root/read` both declare D1. The UI/Core keep those tasks explicitly unlinked rather than guessing; this is recorded as a remaining design-authoring issue, not hidden.

## 2. Real-window operations

1. Opened the frozen Run from the persisted workspace session.
2. Confirmed the blocking check `板与密度已确认`.
3. Selected `D1 root/read`; the batch grid appeared after fixing the AsterDoc assay-ID alias (`assay-cck8` → `cck8` schema family).
4. Entered four OD450 cells in the 96-well grid:
   - A1 Control-1 `0.82`
   - A2 Control-2 `0.41`
   - B1 Material-1 `0.55`
   - B2 Material-2 `0.47`
5. Saved the batch; the UI changed to `Saved` and created a measurement event.
6. Completed `D1 root/read`; progress advanced to `1/3`.
7. Opened `汇总`; the UI showed per-sample arithmetic mean and sample SD, with the explicit note that biological n is not inferred from wells/rows.
8. Opened `运行日志`; it listed check, batch and completion events with batch/event IDs and `打开原始孔板`.
9. Restarted the app; the same Run reopened with `root/read ✓` and the four saved cell values.

## 3. Record evidence

First batch (created before per-readout IDs were added):

- Event: `cc52fb62-1a77-4098-bc48-24282a5fc1a5`
- Batch: `batch-op-1794327103031721700-2`
- `result_cells` carry row/column, sample, group, replicate and value.

Second batch (after adding per-readout IDs):

- Event: `efdb5e1a-ee79-4eee-b1d4-c43f013d5806`
- Batch: `batch-op-1794327493748404300-1`
- Four readout IDs, for example:
  - `readout-op-1794327493748404300-1-0-0-od450` → Control-1 / row 0 / column 0 / 0.83
  - `readout-op-1794327493748404300-1-0-1-od450` → Control-2 / row 0 / column 1 / 0.41
  - `readout-op-1794327493748404300-1-1-0-od450` → Material-1 / row 1 / column 0 / 0.55
  - `readout-op-1794327493748404300-1-1-1-od450` → Material-2 / row 1 / column 1 / 0.47

Check event: `1975ce67-90f7-424c-b03f-c815982b0af8` (custom-check passed).
Completion event: `ff83e1f9-3e9c-4e3e-892a-049936665b1b` (step_status completed).

## 4. Frozen isolation

After the Run was created, the latest AsterDoc was advanced to revision 7 (objective changed) and then revision 10 (SDK modules added). The Run still points to revision 6 and design version `a3157b3f...`; its frozen objective and four wells are unchanged. The latest Design edit does not rewrite the old Run.

The earlier Run `d5163844-1cae-4013-b31e-efe17169d24d` still points to design version `e3c56877-527f-425e-b059-d6e64da8be5e` with the original two wells, even after later edits.

## 5. Remaining gaps

- `root/dose` has no resolved timepoint; the duplicate-D1 warning remains visible.
- The Notebook page now has a read-only `时间线` mode over authoritative Run events (event/batch IDs, task, checks/cells counts, batch drilldown); the editable Markdown modes remain.
- The first batch predates readout IDs; new batches include them, but the old event is intentionally not rewritten.
- High-DPI/scale-200 and dark-mode real-window captures are not part of this evidence.
- The evidence is a disposable fixture copy; it does not claim a user study.
