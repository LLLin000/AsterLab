# MyGo Production Gate — current evidence

Pinned MyGo: `v0.8.4-0.20261108220316-c4197b7fe562` (Native Module CLI is absent in v0.8.3).

## P0 — Windows runtime

| Check | Result |
|---|---|
| Native UI build | pass |
| Rust module DLL | `native check` builds `Runs.dll` |
| Clean dist run | `dist/mygo-run.exe` + `dist/Runs.dll` launches |
| Window title | `AsterLab MyGo Prototype` |
| Working set observed | ~125 MB (real window) |
| Cold-start stopwatch | not recorded reliably; PowerShell window polling hung |

## P1 — Real Run UI

- `LoadRun` loads real SQLite through `asterlab-app`; title/status come from snapshot.
- `SaveReadout` writes a measurement through the shared Core transaction and returns a new snapshot.
- Schedule counts are computed from `snap.Tasks`; current task/record text is still partly static.
- Flow and 96-well plate are explicitly marked `示意`; they are not claimed as real data.
- Two-row input UI is present; the Go module test proves a real write and read-back.
- Full manual UI typing + close/reopen remains to be completed by a human.

## P2 — Revision / concurrency

- `AppCore::load_run` adds a monotonic `revision` = count of execution events.
- Go `RunSnapshot.Revision` is generated and used by the UI `apply` guard:
  a snapshot with a lower revision is rejected.
- Module calls are serialized by `Mutex<AppCore>`.
- Not implemented: `operation_id` idempotency for measurement retries; cross-process
  concurrent writer test.

## P3 — Regression

- `cargo check` root: pass.
- `node --test tests/pages-smoke.cjs`: pass.
- `node tests/execution-acceptance.cjs`: pass.
- `go test ./...`: pass, including:
  - real SQLite copy
  - `LoadRun`
  - `SaveReadout` with `Control-1 / 0.82`
  - revision advance
  - rejected write leaves revision unchanged
  - fresh service reload sees the value
  - CLI `run.workspace` sees the MyGo write

## Remaining before default switch

1. Human UI save with two real readouts, failure message, complete close/reopen.
2. Operation idempotency and a real concurrent writer test.
3. Installer/signing and DLL loading after install (not just a copied dist).
4. Rune remains the fallback; no default UI switch yet.
