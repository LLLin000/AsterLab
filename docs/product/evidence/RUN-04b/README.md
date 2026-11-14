# RUN-04b Evidence — Run Timeline (Recorded Chronology, Failures, Retries, Deviations)

## Baseline & Scope
- Repository: `LLLin000/AsterLab`, isolated `ovnfs` checkout `O:\run04b`.
- Scope files modified:
  - `prototypes/mygo-run/internal/runworkspace/model.go`
  - `prototypes/mygo-run/internal/runworkspace/timeline.go`
  - `prototypes/mygo-run/internal/runworkspace/timeline_test.go`
  - `prototypes/mygo-run/internal/runworkspace/view.go`
- Zero Core write-path or schema changes (`crates/` untouched).

## Verification
1. Native Rust `Runs.dll` built + `go test ./... -count=1`: 21 packages passed, 0 failed.
2. Real Windows GUI verification (`dist/mygo-run.exe` + `Runs.dll`) across 1360×880 & 900×600 Light/Dark:
   - Timeline tab (`时间线`) renders all 4 Core events in exact chronological `RecordedAt` order:
     1) Failure (`失败`, `检查 · 失败`, `步骤阻塞`)
     2) Deviation (`偏差`, `偏差：原空白孔污染，重配空白对照`)
     3) Retry pass (`重试`, `第 2 次`, `检查 · 已确认`)
     4) Final readout & completion (`测量`, `重试`, `第 3 次`, `S1 · OD450 0.85 AU`, `步骤已完成`)

## Screenshots
- `run04b-1360x880-light.png`
- `run04b-1360x880-dark.png`
- `run04b-900x600-light.png`
- `run04b-900x600-dark.png`
