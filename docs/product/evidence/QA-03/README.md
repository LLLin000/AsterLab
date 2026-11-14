# QA-03 Evidence — Third, non-plate assay type on the shared recording kernel

## Baseline & Scope
- Checkout: `O:\qa03`, HEAD `ec5c933e43b559888f53f34f065e410fa229904f` (= `origin/main` at run time).
- Card scope is evidence only: `write: docs/product/evidence/QA-03/`. Nothing outside it changed.
- No fixture, no code, and no frozen Run/Protocol was modified; the harness lives outside the repo (`%TEMP%\qa-harness.py`).

## What ran
1. Third type: **Western Blot** (`western-blot`, `Layout: sample_target`, field `intensity` / `AU`), defined only by the RUN-05 `LayoutConfig` registry — no new recorder code.
2. Seeded a project, Protocol block/version and design through the CLI, then let the real `Runs.dll` `SaveDesign`/`CreateRun` path attach the assay parameters (the same service the GUI uses).
3. Drove the shipped Windows GUI: the type opens on the **table** projection (`位置 / 样本 / 分组 / 重复 / 字段 / 数值 / 状态`), typed `15200` into `g-ctrl · Target_R1`, saved.
4. Force-killed the window, relaunched at 900×600 dark, and confirmed the recorded value and `已保存` state survived.
5. Re-read Core with `labworkbench run.workspace`: one measurement event, cell `(0,0) = 15200`.

## Zero-kernel-change proof
`git diff origin/main -- crates/asterlab-core/src prototypes/mygo-run/internal/resultgrid` is empty
(`commands.txt`). The third type is a layout configuration, so the recorder core and the shared
grid were not touched to add it.

## Screenshots
- `qa03-1-table-1360x880-light.png` — recorded `15200` on the table projection, `已保存`.
- `qa03-2-restart-900x600-dark.png` — the same record after a process kill and relaunch.

## Files
- `commands.txt` — verbatim git output proving the zero-diff claim.
- `run-output.txt` — verbatim harness output including the Core `run.save_batch` commit line.
