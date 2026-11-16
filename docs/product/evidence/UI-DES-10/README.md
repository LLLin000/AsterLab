# UI-DES-10 — Plate geometry and budget sources

## Design consistency

The plate renderer extends the existing `internal/uikit/plate` widget and uses the shared `theme.GetTokens` palette and spacing/type scale. The expanded editor keeps a fixed 384-well-sized canvas across plate formats and scales well size to fit; the inline read-only preview remains compact. The grid is centered in the fixed viewport and the drag-marquee mapping subtracts that centering offset, so row/column selection and drag selection behave the same on every format. The Design budget bar remains the existing `internal/designer` bar and keeps Core and annotation values source-labeled and separate.
UX rules: `UX-P3`, `UX-P4`, `UX-LAYOUT`, `UX-DONE`, `UX-DESIGN`.

## Screenshot evidence

Full-app captures (build stamp `608b57d`, `mygo-automation`, isolated `DES-10 Plate Geometry` fixture; logical DIPs, Windows 150% scaling produces 2018×1308 and 1328×888 PNGs). They predate the fixed-canvas change and remain the toolbar / dark-mode / budget-bar record:

- [plate-12-light-900x600.png](plate-12-light-900x600.png) — light, 12 wells, C4 selected; budget bar visible.
- [plate-12-dark-900x600.png](plate-12-dark-900x600.png) — dark, 12 wells, C4 selected; budget bar visible.
- [plate-96-light-1360x880.png](plate-96-light-1360x880.png) — light, 96 wells, H12 selected; budget bar visible.
- [plate-384-light-1360x880.png](plate-384-light-1360x880.png) — light, 384 wells, P24 selected and assigned to `g-control`.
- [plate-384-dark-1360x880.png](plate-384-dark-1360x880.png) — dark, 384 wells, P24 selected and assigned to `g-control`.

Post-change captures rendered through the `ui.Tester` harness against the production overlay / Design code (1200×900 and 900×600 logical px). They show the fixed footprint, scaled wells, and the prep-calculator link surface:

- [overlay-12-tester-1200x900.png](overlay-12-tester-1200x900.png) — 12-well overlay, C4 selected; editor card spans the same x-range as the 384-well capture below, wells scaled up.
- [overlay-384-tester-1200x900.png](overlay-384-tester-1200x900.png) — 384-well overlay, P24 selected; same editor footprint, wells scaled down.
- [design-plate-tester-900x600.png](design-plate-tester-900x600.png) — Design pane `12 孔板` section with the `流程 / 12 孔板 / 配液单` switch (the prep-calculator link) and the source-labeled budget bar.
- [design-prep-tester-900x600.png](design-prep-tester-900x600.png) — Design pane `配液单` dilution-calculator surface (`母液 2.5 µL · 稀释液 7.5 µL`).

Post-change native-app captures after the fixed-canvas update (`mygo-automation`; 900×600 logical window, emitted as 1328×888 PNGs on this Windows host). The modal overlays the Design page; these prove well sizing and selection, not prep linkage or budget-bar content:
- [native-12-light-900x600.png](native-12-light-900x600.png) — light, 12 wells, C4 selected and assigned to `g-control`.
- [native-12-dark-900x600.png](native-12-dark-900x600.png) — dark, 12 wells, C4 selected and assigned to `g-control`.
- [native-384-light-900x600.png](native-384-light-900x600.png) — light, 384 wells, P24 selected and assigned to `g-control`.
- [native-384-dark-900x600.png](native-384-dark-900x600.png) — dark, 384 wells, P24 selected and assigned to `g-control`.

The design-page captures show the `设计标注` fallback with Core budget fields `—` and no OperationSpec v2 time data; the sources are not combined. Native-app overlays focus on well geometry and are not budget-bar evidence. `internal/asterdoc` tests cover the shared canvas, scaled wells, and interactive 6/12/24/48/96/384 geometry. The legacy assay assignment path remains 12-well-specific.

Verification (`O:/ui-des10fix/prototypes/mygo-run`, 2026-11-16):

```text
ok  asterlab-mygo-prototype/internal/asterdoc      1.810s
ok  asterlab-mygo-prototype/internal/designer      0.822s
ok  asterlab-mygo-prototype/internal/workflowtree  0.324s
ok  asterlab-mygo-prototype/internal/uikit/plate   2.181s
```

Package-suite coverage includes `TestPlateZoomOverlaySelectsWellsAndReportsAssignment`, `TestPlateFormatsUseEditableOverlay` (6/12/24/48/96/384), `TestPlateGridLayoutKeepsCanvasAndScalesWells`, `TestPlateMixPlanDerivesGelAndSeedingRows`, and `TestPlateMixRejectsInconsistentPlans` — all PASS.

## 缺口

The Design workflow cannot yet author OperationSpec v2 (`workflow` currently exposes only `steps`, `edges`, and `business_order`), so Core time budgets are shown only when Core returns a populated projection; otherwise the bar explicitly retains the existing `设计标注` estimates without combining the two sources, and a follow-up Core/Docs card is needed to define the Design-side OperationSpec authoring entry point.
