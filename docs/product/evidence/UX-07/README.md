# UX-07 Evidence — Windows Gate remainder (UX-01 leftovers)

Checkout `O:/ux07`, branch `agent/ux07-windows-gate` (to be pushed), base
`origin/main` @ `a31e04c`. All writes stay inside the card scope
(`prototypes/mygo-run/internal/runui/`, `internal/tools/`,
`internal/shell/`, `design_page.go`, `design_canvas.go`, this dir).

## Build and seed

- GUI: `go build -tags e2e -o dist/mygo-run.exe .` (the `e2e` tag links the
  `mygo-automation` listener; `dist/` is gitignored). `Runs.dll` reused from
  the same-commit sibling build (Rust sources untouched by this card).
- Seed: copied `D:/.ci-cache/des10-ui-seed/labworkbench.sqlite` (pre-existing
  seed snapshot, not the user tree) into isolated `D:/.ci-cache/ux07-data`;
  `LABWORKBENCH_DATA_DIR` pointed there, stale `mygo-run.exe` killed first.
- Capture: real Windows windows via `mygo-automation`; window outer size set
  with `user32.MoveWindow` to 1360×880 / 900×600. Screenshots are device
  pixels (≈1.48×: e.g. 2018×1308 for the 1360×880 logical window); logical
  breakpoints (≥1360 / <1100) are what the layout code branches on
  (`designNarrow`: `bounds.W < 1100`), so verdicts below are at the right
  breakpoints. Capture scripts live outside the repo (`D:/.ci-cache/`).

## Defect 1 — state word (UX-P5)

- Before: `internal/shell/shell.go:501` returned `执行中` for `in_progress`.
- After: returns `进行中` (`shell.go:517`), matching the contract §3
  vocabulary and the already-correct `runworkspace` label.
- `grep -rn 执行中` over the five in-scope files now hits only the code
  comment noting the change. Verified by `go test ./internal/shell/...`
  (pass) and `TestShellThemes`-family tests (pass).

## Defect 2 — hardcoded FontSize → theme Rem (UX-P3)

Mapping keeps visual sizes ~equal: `t.Rem` multiplies the theme base
(14px on Windows), so `FontSize(N)` → `t.Rem(N/14)` rounded to 2dp:
9→0.64, 10→0.71, 11→0.79, 12→0.86, 13→0.93, 14→1.0, 15→1.07, 16→1.14,
18→1.29, 20→1.43. This matches neighboring converted code (`.78`≈11px,
`.72`≈10px). `FontSize(float32(tok.TextMeta))` forms already use theme
tokens and were kept (8 sites). Every change cites `UX-P3` inline.

| File | Before (`FontSize([0-9]`) | After |
|---|---|---|
| `internal/runui/runui.go` | 80 | 0 |
| `internal/tools/tools.go` | 7 | 0 |
| `design_page.go` | 4 | 0 |
| `design_canvas.go` | 2 | 0 |

Functions without a theme in scope (`renderStateBadge`, `plateGrid`,
delete-step dialog) use `c.Theme().Rem(..)` instead of inventing tokens.

## Structural items — fix-or-close with evidence

| # | Item | Verdict |
|---|---|---|
| 1 | Workspace "show all" affordance (UX-LAYOUT) | FIXED. The in-progress preview was silently capped at 2 with no affordance (`shell.go`). Added `Shell.ShowAllRuns` + ghost buttons `显示全部 N 个进行中` / `收起` ([shot](ux07-workspace-1360x880-light.png) shows 2-item seed; affordance renders only when `len(inProgress) > 2`). Designs were already uncapped + scrolling. |
| 2 | Persistent ≥320px right inspector at 1360 (UX-LAYOUT) | FIXED already in code, verified. `design_canvas.go` renders `stepPropertiesDrawer(..., bottom=false)` with `MinWidth(320)` beside the canvas ([1360 light](ux07-canvas-inspector-1360x880-light.png), [1360 dark](ux07-canvas-inspector-1360x880-dark.png): 检查器 panel ≈370 logical px). |
| 3 | Bottom sheet below 1100 (UX-LAYOUT) | FIXED already in code, verified. Narrow windows start with the inspector closed (toolbar `展开检查器`), and opening it docks a full-width bottom sheet, not a floating card ([900 open](ux07-canvas-inspector-open-900x600-light.png)); the list view shares `stepPropertiesDrawer(..., bottom=true)`. Primary action (toolbar `创建运行`) is never overlaid. |
| 4 | Tools primary button reachable at 900×600 (UX-P1) | FIXED. Root cause: the content `Column` was not scrollable, so the handoff button (and wells 10–12) were clipped and unreachable. Wrapped the content pane in `ui.ScrollBoth` + `AlignItems(ui.Stretch)` on the row (without Stretch the Scroll sizes to content and never scrolls). [Plate scrolled to primary](ux07-plate-900x600-light.png), [cols 10–12 reachable](ux07-plate-hscroll-900x600-light.png). |
| 5 | English status strings + retry (UX-P5) | FIXED. `Choose a draft to import`→`请先选择一个草稿再导入`, `Draft ready`→`草稿已就绪`, `Import accepted`→`导入成功`, `paste a block…`→`请粘贴…`, `invalid JSON`→`JSON 无效`, `assay design needs…`→`assay 设计至少需要一个孔位分配`. Added `Workspace.Retry` so import-validation failures render cause + `重试` beside the Validate button ([shot](ux07-importerr-900x600-light.png)). Pantry keeps `PantryRetry`. New focused test `TestImportFailureChineseCause` passes. |

Follow-ups (found during verification, out of scope — not changed):
- Matrix horizontal overflow was fixed as a drive-by only insofar as
  `ScrollBoth` covers it; no dedicated matrix-layout work was done.
- No Run exists in the seed data, so the shell `进行中` run-card word has
  code+test evidence only, no screenshot.
- `notebook_page.go` was already clean (out of scope, untouched).

## Screenshot list

Workspace / Design(empty) / Tools(pantry): `ux07-{workspace,design,tools}-{1360x880,900x600}-{light,dark}.png`.
Canvas inspector: `ux07-canvas-inspector-{1360x880,900x600}-{light,dark}.png`
plus `ux07-canvas-inspector-open-900x600-light.png` (bottom sheet).
Tools fixes: `ux07-plate-900x600-light.png`,
`ux07-plate-hscroll-900x600-light.png`, `ux07-importerr-900x600-light.png`.

## Checks

- `go build ./...` clean; `go vet` clean on
  `internal/{runui,tools,shell}` + root (root-package FFI test failures are
  pre-existing environment: `Runs` native module not built for
  windows-amd64 in this checkout; scoped packages all pass).
- `go test ./internal/runui/... ./internal/tools/... ./internal/shell/... -count=1`: all ok.
- `gofmt -l` on touched files lists pre-existing unformatted files
  (`tools_test.go` untouched is also listed); no whole-file reformat was
  done to keep the diff minimal — only the touched lines follow gofmt.
- Test-run screenshot artifacts under `prototypes/` were reverted; the
  commit holds source + this evidence only.
