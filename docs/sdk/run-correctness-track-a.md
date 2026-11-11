# Track A — Run correctness fixes

- Baseline: `feature/v2-run-workspace` @ `12e68fe`
- Scope: Run execution correctness only; Run UI v1 layout is unchanged.
- No new module, page, nested Canvas, or second event store.

## 1. Fixed behavior

| Requirement | Result |
|---|---|
| Rail scrolling and current task visibility | The left rail is an independent `ui.Scroll`; the selected step/child calls `ScrollIntoView`, so 09/10 are reachable at 1360×880. |
| Human-readable step titles | `RunTask.Title` is the frozen Protocol step title; `RunTask.Path` is the stable path. Titles and paths are no longer interchangeable. |
| Measurement association | `Measurement` carries `TaskKey`/`ProtocolStepPath`; `BuildViewModel` attaches only exact matches. Ambiguous or missing identity goes to `UnassociatedMeasurements`; it is never copied to every task. |
| CheckAck restoration | Native `SaveTaskChecks`/`CompleteTask` events carry the internal `check_ack` marker. Only those formal acks set `TaskCheck.Status`; free-form `execution_event.create` checks stay unknown. |
| Event audit | `RunEvent.DataJSON` carries the projected event; the task history renders operation, material_use, deviation, lineage, measurements, parameters, checks, and check-ack markers. |
| Operation detail | The detail uses the frozen Protocol `instruction` and resolved `parameters`, not raw module fields. Raw module chips are replaced by resource name + ID. |
| Session persistence | `run_id` is saved on every page, not only on Run. Restart restores the selected Run and the last page (including Notebook). |
| Revision labels | Header shows `Run rev N · Design rev N · AsterDoc rev N`; the Run revision is no longer labeled as the frozen design revision. |
| Callback identity | Event writes use `task.Path`, never `task.Title`; `ASTERLAB_TASK_PATH` matches `task.Path`. |

## 2. Acceptance evidence

| Evidence | What it proves |
|---|---|
| `evidence/run-01-fixed.png` | Human title, frozen instruction, 0 measurements on step 01, revision labels, resource IDs. |
| `evidence/run-05-wash1-fixed.png` | Wash I instruction and check; no measurements from other tasks. |
| `evidence/run-06-wash2a-fixed.png` | First Wash II as a distinct human-titled task. |
| `evidence/run-09-elute-fixed.png` | Rail auto-scrolls to 09; elution instruction; no unrelated measurements. |
| `evidence/run-10-qc-fixed.png` | All 8 mock measurements appear only on the QC task. |
| `evidence/run-10-checked.png` | Clicking the check writes a formal CheckAck; status becomes 已确认, count 1/1. |
| `evidence/run-10-restart-checked.png` | The CheckAck survives restart. |
| `evidence/run-06-history-deviation.png` | Task history shows operation, material use, lineage `cartridge → waste`, and the RNA-S03 deviation. |
| `evidence/notebook-run-context.png` | Switching to Notebook keeps the selected Run. |
| `evidence/notebook-restart.png` | Restart restores the Notebook page with the same Run context. |
| `evidence/rtqpcr-run-fixed.png` | Parent Run still exposes 13 flat tasks; containment remains Track B. |

## 3. Tests run

```text
go test ./...                                              PASS
go run github.com/egoist/mygo/cmd/mygo native check modules/runs   PASS
```

New Go test:

- `internal/runworkspace/model_test.go::TestBuildViewModelAssociatesMeasurementsAndAuditEvents`
  covers task/path measurement association, unassociated measurements, unassociated events,
  formal check status, frozen instruction/parameters, materials, deviation, lineage, and check-ack parsing.

GUI checks:

- Clicked the QC check; the native event was stored; status changed to 已确认.
- Killed and relaunched the app; the check, run revision 20, design revision 3, and AsterDoc revision 2 remained.
- Switched to Notebook and restarted; the Run context remained.

## 4. Remaining gaps (Track B/C)

- Parent/child workflow projection and parent status aggregation are not in the Run UI yet; the RT-qPCR parent Run is still 13 flat tasks.
- Sample-level status is still task-level; events carry `sample_ids`, but there is no per-sample matrix.
- Resource chips show module ID/name, not a full container/sample identity model.
- The history is per task; there is no run-level unified timeline (unassociated events have a separate card).
- No `inspect_tree`/`expand` Agent command; Track B defines the read-only contract.
