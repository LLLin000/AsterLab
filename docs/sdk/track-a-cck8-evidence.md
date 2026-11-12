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

`root/seed` and `root/read` both declare D1. They are distinguished by path / Step ID; the remaining warning is advisory and says so, rather than claiming the tasks are unresolved.

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

- `root/dose` has no resolved timepoint; the shared-D1 advisory remains visible (it no longer claims the tasks are unresolved).
- The Notebook page now has a read-only `时间线` mode over authoritative Run events (event/batch IDs, task, checks/cells counts, batch drilldown); the editable Markdown modes remain.
- The first batch predates readout IDs; new batches include them, but the old event is intentionally not rewritten.
- Real-window captures are stored under `docs/sdk/evidence/`: 900×600 light, 900×600 dark at 150% display scale, and 1080×640 Run. Scale-200 and additional dark pages remain open.
- The evidence is a disposable fixture copy; it does not claim a user study.

## 6. SDK v1 RC re-acceptance (2026-11-12)

A second CCK-8 loop was driven end to end on a fresh fixture, after the workflow
hierarchy join was fixed. This run is the SDK v1 RC acceptance.

- Fixture: project `ef9f32d7-1331-4ac9-a61a-64fff81ab38a`, design
  `9b19d678-7c10-4247-9481-2312479f63d0` (AsterDoc revision 2), Run
  `6116cb32-f5a7-45e8-8f8d-4e971aac09ca`.
- Design: one CCK-8 assay (`assay-cck8`, OD450/AU), two steps
  (`cck8/seed`, `cck8/readout`); the readout step reuses ProtocolBlock
  `5aa33924-33c9-44d7-855d-8965cb36c5cd` through Protocol
  `e8e5fc61-…`/version.
- Drive: `scripts/gui-e2e-v11.ps1` against the real window through
  `mygo-automation`, then `run.workspace` through the typed CLI.

Steps exercised and their evidence:

1. Opened the run workspace; the 96-well grid rendered
   `cck8.plate96.od450.v1 · OD450 (AU)`.
2. Pasted an 8×12 block with `A1=0.91`, `A2=0.42`; the UI reported
   `粘贴预览：8 × 12` and then `已记录 96` (mean `0.485`).
3. Saved the batch; the panel showed `已保存`.
4. Killed the app and relaunched it; the restored grid reported
   `已记录 96`, mean `0.499` (the pasted values, not the paste preview).
5. `run.workspace` cross-read the same cells (`0.91`, `0.42`) from the
   persisted event.
6. Real window resized to 900×600 in both themes; the run workspace,
   hierarchy rail and recording panel render without clipping. Captures:
   `evidence/cck8-hierarchy-900x600-light-v1rc2.png`,
   `evidence/cck8-hierarchy-900x600-dark-v1rc2.png`.

Hierarchy join fixed in this pass. A frozen task path is the protocol
block's own path (`root/seed`), which never equals the author-chosen
design step path (`cck8/readout`). The compiler now records
`mygo.stepBlocks` (design step → reused block version), the native Run
snapshot links each task to its design step through that identity, and
`workflowtree` joins on `DesignStepID` when paths do not contain. The
rail now shows the protocol steps under their design step instead of
reporting every task as orphaned.

Known remaining diagnostic: a design step that reuses no ProtocolBlock
(`cck8/seed` here) produces no frozen task, so the rail still reports
`no frozen task matches this step path` for that step. That is accurate,
not a join failure.
