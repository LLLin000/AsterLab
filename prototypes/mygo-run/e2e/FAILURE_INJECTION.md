# Failure injection & stale response plan

## Covered by tests
- Missing sample ID -> SaveReadout returns error, revision unchanged (`TestRustModuleReadsAndWritesSQLite`).
- Two readouts persist and reopen (`TestTwoReadoutsPersistAndCrossRead`).

## Planned / not yet implemented
- Duplicate operation ID with different payload -> ErrConflict (core idempotency not implemented yet).
- Concurrent writer across processes -> not tested (Mutex is in-process).
- Stale async response -> Go `app.apply` ignores lower revision; equal revision mismatch -> conflict/reload (documented, not GUI-automated).
- Failure injection commands:
  - Remove `Runs.dll` from dist -> app fails to load module (manual).
  - Point `LABWORKBENCH_DATA_DIR` to empty dir -> LoadRun error surfaced in UI.

## Evidence
- Test output logs, BUILD_INFO.txt, MEASURE.txt.
