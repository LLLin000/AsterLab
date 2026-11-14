# RUN-06 Evidence — Keyboard Entry, Crash Draft Recovery & Idempotent Retry

## Baseline & Scope
- Repository: `LLLin000/AsterLab`, isolated `ovnfs` checkout `O:\run06`.
- Scope files modified:
  - `prototypes/mygo-run/internal/resultgrid/grid.go`
  - `prototypes/mygo-run/internal/resultgrid/plate.go`
  - `prototypes/mygo-run/internal/runworkspace/draft.go`
  - `prototypes/mygo-run/internal/runworkspace/draft_test.go`
  - `prototypes/mygo-run/internal/runworkspace/recording.go`
  - `prototypes/mygo-run/main.go`
  - `prototypes/mygo-run/main_test.go`
- Zero Core changes (`crates/`, `src/` untouched).

## Implementation
- **Keyboard workflow (`UX-P1`, `UX-P3`, `UX-LAYOUT`)**: `plate.go` `OnSubmit` (`Enter` inside well editor) commits the cell and advances selection to the next well; `recording.go` adds `Ctrl+S` (`ui.Cmd + ui.KeyS`) to trigger `OnSaveBatch`.
- **Periodic atomic draft persistence & crash recovery (`UX-STATE`, `UX-P5`)**: `draft.go` flushes dirty grid cells + `PendingOpID` atomically via temp file + rename to `<LABWORKBENCH_DATA_DIR>/drafts/<hash>.json` on cell changes / 1s periodic interval; restores automatically on restart with `"已从上次未保存草稿恢复（Ctrl+S 保存到 Core）"` and `"丢弃草稿"` action; clears draft on Core commit.
- **Idempotent retry (`UX-P5`)**: `Grid.PendingOpID` + `Grid.CellsDigest()` keep the exact same `operation_id` across retries when cells are unchanged (allocating a new `operation_id` only if cells change), and `RecordingView` enables `"重试保存"` when `StateError` instead of disabling Save.

## Verification
1. `go test ./... -count=1`: 21 packages passed (`TestGridDraftCrashRecoveryAndClear`, `TestSaveBatchReusesOperationIDAcrossRetries`).
2. Real Windows GUI verification (`dist/mygo-run.exe` + `Runs.dll` + `labworkbench.exe`):
   - Typed `0.81` into `A1`, pressed `Return` -> committed cell and advanced to `B1`; draft flushed to `data/drafts/e1890f03fbcd47531b04f66b20190529.json` (`run06-draft-1360x880-light.png`).
   - Force-killed `mygo-run.exe` (`SIGKILL` / `proc.kill()`) without closing.
   - Restarted GUI (`1360×880` dark & `900×600` light) -> recovered `A1=0.81` with `"已从上次未保存草稿恢复"` (`run06-recovered-1360x880-dark.png`, `run06-recovered-900x600-light.png`).
   - Saved via `Ctrl+S` -> `"已保存"`, draft file removed, persisted across restart (`run06-saved-900x600-dark.png`), Core `run.workspace` confirmed `events=1` measurement event.

## Screenshots
- `run06-draft-1360x880-light.png`
- `run06-recovered-1360x880-dark.png`
- `run06-recovered-900x600-light.png`
- `run06-saved-900x600-dark.png`
