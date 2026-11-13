# RUN-02 Evidence — Read-Only Procedure & Frozen Parameters

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, base `origin/main`, isolated `ovnfs` checkout `O:\run02`.
- Scope files modified:
  - `prototypes/mygo-run/internal/runui/runui.go`
  - `prototypes/mygo-run/internal/runui/runui_test.go`
  - `prototypes/mygo-run/internal/runworkspace/model.go`
  - `prototypes/mygo-run/internal/runworkspace/view.go`
  - `prototypes/mygo-run/internal/runworkspace/view_test.go`
  - `prototypes/mygo-run/main.go`
  - `docs/product/evidence/RUN-02/`

## UX Contract Rule Citations
- `UX-P1` (Task first): At `900×600`, recording remains primary while frozen procedure/parameters stay accessible inside `操作说明与元数据`.
- `UX-P2` (Progressive disclosure): Frozen `ProtocolVersionID`, `StepPath`, `AssayID`, `Timepoint`, and `StepID` live in the read-only footer/metadata row.
- `UX-P3` (One component language): Built with `theme.GetTokens(t)` chips and `c.Theme()` borders/typography.
- `UX-STATE`: Explicitly labeled `操作说明 · 已冻结 · 只读` and `操作参数 · 已冻结 · 只读` with zero edit controls (`TextInput` or edit buttons).
- `UX-LAYOUT` / `UX-DONE`: Verified at `1360×880` and `900×600` in both light and dark themes.

## Automated Checks
- `cd prototypes/mygo-run && go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native check ./modules/runs` → exit 0 (`Built Runs for windows/amd64`)
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (`21 packages ok, 4 no tests`)

## Real Windows GUI Verification (Design Edited After Run Freeze)
1. Created Protocol v1 (`9e2896eb-2a34-493d-ad89-afe97e83913a`, `"旧版指令：37 °C 静置孵育 60 min。"`, `temp = 37 °C`, `minutes = 60 min`) and froze `RUN02-Old-Run` (`898f22e3-0898-4239-a1e3-fa3dcb44da88`).
2. Revised Protocol to v2 (`6be3de6a-9363-4b03-8777-648c29bdd699`, `"新版指令：改为 42 °C 振荡孵育 120 min。"`, `temp = 42 °C`, `minutes = 120 min`) and saved Design `rev 2`.
3. Reopened `RUN02-Old-Run` across `1360×880` (light/dark) and `900×600` (light/dark):
   - Every view showed frozen v1 (`旧版指令：37 °C 静置孵育 60 min。`, `37 °C`, `60 min`, `9e2896eb`, `AsterDoc rev 1`), with zero leak from v2 (`新版指令`, `42 °C`, `120 min`) and zero edit controls.
