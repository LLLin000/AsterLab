# RUN-01 Evidence — Workflow Navigation & Task Selection State

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, base `origin/main` (`e2162bb`), isolated `ovnfs` checkout `O:\run01`.
- Scope files modified:
  - `prototypes/mygo-run/internal/runworkspace/recording.go`
  - `prototypes/mygo-run/internal/runworkspace/view.go`
  - `prototypes/mygo-run/internal/runworkspace/view_test.go`
  - `prototypes/mygo-run/main.go`
  - `prototypes/mygo-run/main_test.go`
  - `docs/product/evidence/RUN-01/`

## UX Contract Rule Citations
- `UX-P1` (Task first): At `900×600`, `recordingLayer` renders above `procedureLayer` and `flowRail` so the recording action sits in the initial viewport without scrolling.
- `UX-P5` (Explicit, recoverable state): Per-task `taskGrids` and `taskDrafts` preserve unsaved edits across task switches (`SelectTask` / `selectRunTask`) and only clear on Core save or Run switch; dirty state renders `未保存`.
- `UX-STATE`: Rail status dots and labels (`未开始`, `进行中`, `已完成`, `已阻塞`) project directly from Core `RunTask` / `ExecutionEvent` state.
- `UX-LAYOUT` / `UX-DONE`: Verified at `1360×880` and `900×600` in both light and dark themes.

## Automated Checks
- `cd prototypes/mygo-run && go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native check ./modules/runs` → exit 0 (`Built Runs for windows/amd64`)
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (`20 packages ok, 4 no tests`), including:
  - `TestSelectTaskPreservesUnsavedDraft`
  - `TestFlowRailShowsHierarchyProgressTimepointAndCoreStatus`
  - `TestNarrowLayoutPrioritizesRecordingBeforeNavigation`

## Real Windows GUI Walkthrough (3 Experiment Types)
Using built `dist/mygo-run.exe` against isolated `labworkbench.sqlite` (`CCK-8`, `RNA`, `CFU` non-plate):
1. **CCK-8 (96-well plate + draft preservation)**:
   - Left vertical rail showed `接种细胞 1/2` (`24 h · 已完成`, `48 h · 未开始`) and `CCK-8 酶标检测 0/2` (`读取 OD450 · 24 h · 未开始`, `48 h · 未开始`).
   - Selected `读取 OD450 · 24 h`, entered `0.88` in cell `A1`, switched to `48 h`, switched back to `24 h` → `0.88` (`未保存`) preserved (`run01-cck8-1360x880-light.png`).
   - Narrow `900×600` placed `实验记录` above collapsible `操作说明与元数据` and `实验流程` (`run01-cck8-900x600-light.png`).
2. **RNA (composite sub-steps)**:
   - Rail expanded `RNA 提取 2/4 · 进行中` (`01 收集细胞 · 已完成`, `02 裂解匀浆 · 已完成`, `03 柱上洗涤 · 进行中`, `04 洗脱质检 · 未开始`) and `qPCR 定量 0/1 · 未开始`, highlighting the selected step (`run01-rna-1360x880-dark.png`, `run01-rna-900x600-dark.png`).
3. **CFU (non-plate third type)**:
   - Non-plate `CFU 菌落计数` rendered Core-recorded `CFU-S1 · 1.4e+06 CFU/mL` with `平板涂布计数 0/1 · 进行中` rail status (`run01-cfu-1360x880-light.png`, `run01-cfu-900x600-dark.png`).
