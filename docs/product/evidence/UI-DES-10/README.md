# UI-DES-10 — Plate geometry and budget sources

## Design consistency

The plate renderer extends the existing `internal/uikit/plate` widget and uses the shared `theme.GetTokens` palette and spacing/type scale; the Design budget bar remains the existing `internal/designer` bar and keeps Core and annotation values source-labeled and separate.

## Screenshot evidence

Captured from the rebuilt app at build stamp `608b57d` using `mygo-automation` and the isolated `DES-10 Plate Geometry` fixture. Window sizes are logical DIPs; Windows 150% scaling produces 2018×1308 and 1328×888 PNGs.

- [plate-12-light-900x600.png](plate-12-light-900x600.png) — light, 12 wells, C4 selected; budget bar visible.
- [plate-12-dark-900x600.png](plate-12-dark-900x600.png) — dark, 12 wells, C4 selected; budget bar visible.
- [plate-96-light-1360x880.png](plate-96-light-1360x880.png) — light, 96 wells, H12 selected; budget bar visible.
- [plate-384-light-1360x880.png](plate-384-light-1360x880.png) — light, 384 wells, P24 selected and assigned to `g-control`.
- [plate-384-dark-1360x880.png](plate-384-dark-1360x880.png) — dark, 384 wells, P24 selected and assigned to `g-control`.

All captures show the `设计标注` fallback with Core budget fields `—` and no OperationSpec v2 time data; the sources are not combined. `internal/asterdoc` tests cover interactive 6/12/24/48/96/384 geometry. The legacy assay assignment path remains 12-well-specific.

Verification: `go test ./internal/asterdoc -count=1`, `go test ./internal/designer -count=1`, and `scripts/mygo-build.ps1` passed.

## 缺口

The Design workflow cannot yet author OperationSpec v2 (`workflow` currently exposes only `steps`, `edges`, and `business_order`), so Core time budgets are shown only when Core returns a populated projection; otherwise the bar explicitly retains the existing `设计标注` estimates without combining the two sources, and a follow-up Core/Docs card is needed to define the Design-side OperationSpec authoring entry point.
