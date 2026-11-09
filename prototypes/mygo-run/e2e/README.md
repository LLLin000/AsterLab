# MyGo Run E2E Plan (unsigned prototype)

## Automated (Go tests)
- `go test ./...` in `prototypes/mygo-run` runs `TestRustModuleReadsAndWritesSQLite`:
  - loads real Run snapshot
  - saves one readout (Control-1 = 0.82)
  - revision advances
  - rejected write leaves revision unchanged
  - fresh service + CLI cross-read see the value
- Deterministic two-readout harness: `prototypes/mygo-run/e2e` (see below).

## Manual GUI (NOT automated; GUI automation unavailable)
1. Launch `dist/mygo-run.exe` with `LABWORKBENCH_DATA_DIR` pointing to a copied runtime DB.
2. Enter Control-1=0.82 and E1-1=0.41, save; expect "已保存".
3. Complete task; reload window; values persist.
4. CLI `run.workspace` sees both measurements.
5. Stale async response: open two windows? Not supported; rely on revision guard in `app.apply`.

## Failure injection
- Missing sample ID -> SaveReadout error, revision unchanged.
- Same operation ID with different payload -> ErrConflict (core side; not yet implemented).
- Concurrent writer -> not tested (single process Mutex only).

## Performance
- Run `scripts/mygo-measure.ps1`; record MEASURE.txt.

## Packaging
- Unsigned ZIP only. No installer/signing. Label clearly in BUILD_INFO.txt.
