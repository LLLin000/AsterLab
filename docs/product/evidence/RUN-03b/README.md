# RUN-03b Evidence — Result Grid Plate & Table Projections

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, `origin/main` at `f087394`, isolated `ovnfs` checkout `O:\run03b`.
- Scope files modified:
  - `prototypes/mygo-run/internal/resultgrid/grid.go`
  - `prototypes/mygo-run/internal/resultgrid/plate_edit_test.go`
  - `prototypes/mygo-run/internal/runworkspace/recording.go`
  - `prototypes/mygo-run/internal/runworkspace/recording_test.go`
  - `docs/product/evidence/RUN-03b/`

## UX Contract Rule Citations
- `UX-P1` (Task first): `保存读数` remains pinned in the recording header across both `孔板` and `表格` projections.
- `UX-P3` / `UX-P4` (One component language, content decides layout): Plate renders the 96-well spatial matrix while Table renders the scrollable 7-column tabular projection (`孔位`, `样本`, `分组`, `重复`, `字段`, `数值`, `状态`), both reading/writing the single `*resultgrid.Grid`.
- `UX-P5`: Invalid numeric edits surface `g.Error` and disable persistence; switching projections commits or preserves pending edits cleanly.
- `UX-LAYOUT` / `UX-DONE`: Verified at `1360×880` and `900×600` in both light and dark themes.

## Automated Checks
- `cd prototypes/mygo-run && go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native check ./modules/runs` → exit 0 (`Built Runs for windows/amd64`)
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (`21 packages ok, 4 no tests`)

## Real Windows GUI Verification
1. Opened `RUN-03b-Run` (`cce59c3c-1315-42bc-82f5-3ea96d6df4bb`) in `dist/mygo-run.exe`.
2. Entered `0.91` for `A1` in `孔板` view → switched to `表格` → `A1 = 0.91` immediately visible (`run03b-plate-1360x880-light.png`, `run03b-table-1360x880-light.png`).
3. Entered `0.73` for `A2` in `表格` view → switched to `孔板` → `已记录 2` immediately reflected in Plate → clicked `保存读数` → `已保存` (`PLATE_TABLE_BIDIRECTIONAL_AND_SAVE_OK`).
4. Cross-checked `labworkbench run.workspace` against SQLite → persisted measurements confirmed.
5. Verified `1360×880` dark and `900×600` light/dark screenshots across both Plate and Table views.
