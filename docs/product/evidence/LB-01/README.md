# LB-01 — Design step-card list

Real desktop-window captures of the redesigned Design surface: the default view
is now a vertical step-card list with a top budget bar, a `流程 / 12 孔板 /
配液单` switch, and per-card folded technical properties.

- Build: `prototypes/mygo-run/dist/mygo-run.exe`, `BUILD_INFO.txt` in this
  folder records `commit 38dc5a0` — the code commit this evidence was built
  from (the evidence commit itself adds only these files).
- Window: `mygo-run.exe` resized with `user32 MoveWindow` to 1360×880 and
  900×600, `ASTERLAB_THEME=light|dark`.
- Data: isolated `LABWORKBENCH_DATA_DIR` (copy of an existing fixture DB). The
  design is a plain designer design — its AsterDoc revisions were removed so
  `LoadAsterDoc` returns v0 and the surface opens `designer.ListView`. Its
  `mygo` extension carries three HUVEC steps with real parameters, one assay
  mapping and two block instances. Nothing in the fixture DB was fabricated on
  top of an existing record beyond this test design's `mygo` payload.

## Captures

Each row links 1360×880 and 900×600; every section is captured in both themes.

| Surface | What it shows | Rule ids | Light 1360×880 | Light 900×600 | Dark 1360×880 | Dark 900×600 |
|---|---|---|---|---|---|---|
| 流程 (default) | Step cards with 序号, 标题, parameter summary and cost/type tags; budget bar above; primary 创建 Run in the first viewport | UX-P1, UX-P3, UX-P4 | [flow](lb01-flow-1360x880-light.png) | [flow](lb01-flow-900x600-light.png) | [flow](lb01-flow-1360x880-dark.png) | [flow](lb01-flow-900x600-dark.png) |
| 技术属性 (opened) | Protocol path, reference id and block-version ids reachable in one click; folded by default | UX-P2 | [props](lb01-properties-1360x880-light.png) | [props](lb01-properties-900x600-light.png) | [props](lb01-properties-1360x880-dark.png) | [props](lb01-properties-900x600-dark.png) |
| 12 孔板 | Plate plan of the selected step's assay | UX-P4, UX-P5 | [plate](lb01-plate-1360x880-light.png) | [plate](lb01-plate-900x600-light.png) | [plate](lb01-plate-1360x880-dark.png) | [plate](lb01-plate-900x600-dark.png) |
| 配液单 | Dilution calculator and parameter table of the selected step | UX-P4, UX-P5 | [dilution](lb01-dilution-1360x880-light.png) | [dilution](lb01-dilution-900x600-light.png) | [dilution](lb01-dilution-1360x880-dark.png) | [dilution](lb01-dilution-900x600-dark.png) |

## Behaviour pinned by tests

`prototypes/mygo-run/internal/designer/step_list_test.go`:

- `TestStepCardShowsRecordedParameters` — the card renders the step's recorded
  parameters (`温度 80 °C · 孵育时间 5m · 占手 45m`) and the budget bar totals
  them; no `未填写参数` placeholder.
- `TestStepCardWithoutParametersShowsNoPlaceholder` — a step with nothing
  recorded renders its title and no placeholder.
- `TestSectionSwitchPreservesSelectionAndDraft` — switching 流程 → 12 孔板 →
  配液单 → 流程 keeps `SelectedNode`, the assay/timepoint drafts and the dirty
  flag (UX-P5).
- `TestTechnicalBindingsFoldedByDefault` — the reference UUID, block version
  (`note1@3`) and protocol path are absent from the default card and appear
  after opening `技术属性` (UX-P2).
- `TestStepCardSummaryText` — derivation falls back to assay/timepoints and
  never invents a duration.

## Known limitation

The `12 孔板` section reuses the shared plate widget, which draws the 96-well
plan of the design's well assignments. A dedicated 12-well variant needs
`internal/uikit/plate` and `internal/blocks`, both outside this card's write
scope; the section label follows UI-REDESIGN §2.1 and the switch/selection
behaviour is in place.
