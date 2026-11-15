# UX-03 evidence — keyboard, focus, scroll, resize and restart

Card `8998d133-0567-47c0-9230-b5fbc60ab6e0`. Branch `agent/UX-03-keyboard-focus`.
Contract rules cited: `UX-P1`, `UX-P3`, `UX-P5`, `UX-STATE`, `UX-LAYOUT`.

## Build provenance

| Role | Commit | mygo pin | Runs.dll SHA256 | mygo-run.exe SHA256 |
|---|---|---|---|---|
| Before (origin/main) | `f624c8a` | `v0.8.4-0.20261108220316-c4197b7fe562` | `14A9A47A292B16F7761D5BECFAB71A0904EC19929505FFCC74FE985F5B6D31CE` | `F76E76FF52EE167F86BAA9CCBBFFFEACBF321AE3F318C5E85A6C43151E7B244A` |
| After (fix) | `c573231` | `v0.8.4-0.20261108220316-c4197b7fe562` | `1E6EBAE6CA142E914DD61BA267DAA252B292BC896A9C6D54E676D1C1A617E9CA` | `97FC30EFBA5A3ABDCEC22D6B6639C8FC610D74D5D5B73A1100FFCB5AC434526A` |

`dist/BUILD_INFO.txt` for the after build reports `commit c573231`; the evidence commit adds only
this directory, so the built source is identical. The final dist is rebuilt at the evidence head and
its `BUILD_INFO.txt` commit equals HEAD (the embedded VCS stamp, and therefore the exe hash, changes
with the commit). Real Windows GUI, light theme,
isolated data directory `%TEMP%\ux03-repro\data`; seeded run `b90c9b1a-355f-44f4-a9ef-fae5ecd48f4c`
(14 executable CCK-8 steps T1..T14, 96-well schema `cck8.plate96.od450.v1`).

## Defect 1 — table kept a stale scroll offset on 孔板 → 表格

Root cause: at ≤1000px the flow rail shares the page scroll with the recording panel. Revealing
the selected rail row (ScrollIntoView, centred) scrolled the whole page, so the recording header
and the table's first rows started above the viewport. The stale offset then persisted into the
table view. Before, accessibility bounds at 900×600 reported the header at `Y=-258` and the first
table row `A1` at `Y=-106` (both offscreen). After the fix the header is at `Y=226` and `A1` at
`Y=378`, inside the 900×600 viewport, with no manual scroll (`UX-P1`, `UX-LAYOUT`).

The rail now reveals its selection only inside its own scroll container (wide layout), and re-arms
that reveal when the layout widens again. Each recording view root is keyed
(`recording-plate/table/gallery/form`) so plate, table, gallery and form keep separate scroll and
focus state.

| Shot | File |
|---|---|
| Before, 900×600, plate | `ux03-scroll-before-900x600-light.png` |
| Before, 900×600, table (A1 offscreen) | `ux03-scroll-before-table-900x600-light.png` |
| Before, 1360×880, plate | `ux03-scroll-before-1360x880-light.png` |
| Before, 1360×880, table | `ux03-scroll-before-table-1360x880-light.png` |
| After, 900×600, plate | `ux03-scroll-after-900x600-light.png` |
| After, 900×600, table (A1 visible) | `ux03-scroll-after-table-900x600-light.png` |
| After, 1360×880, plate | `ux03-scroll-after-1360x880-light.png` |
| After, 1360×880, table | `ux03-scroll-after-table-1360x880-light.png` |

## Defect 2 — selection, rail scroll, focus, zoom across resize and restart

Measured on the fixed build with run `b90c9b1a` and task `读数 T10` selected.

| Behaviour | Before | After | Verdict |
|---|---|---|---|
| Selected task across 1360→900→1360 resize | selection kept in memory | selection kept; rail scrolls the selected row into view again when it regains its own scroll | PASS |
| Sidebar (flow rail) scroll after resize | rail scroll node is rebuilt; stale reveal key left the selection unrevealed | reveal re-armed on widening (`railInline` reset); `读数 T10` at `Y=514`, `读数 T1` scrolled out of view | PASS |
| Selected task after app restart | selecting a step group stored its synthetic step id (`step-read10`), which is not a Core task key; `workspace-session.json` had no `task_key`, so restart fell back to T1 | the group reports its first real task key; session now stores `task_key` (`root/assay10`, T10) and restart reopens run + T10, rail scrolled to it | PASS (fixed) |
| Keyboard focus across 1360↔900 resize | `A1 读数` lost focus on the first resize | `A1 读数` keeps focus across both resizes (focus bound to the result grid, auto-focused when the breakpoint rebuilds the field) | PASS (fixed) |
| Design canvas zoom across resize | `Editor.Zoom` is view state on the long-lived Designer, not a layout input | unchanged by resize | PASS by implementation; GUI gesture NOT VERIFIED (see below) |
| Design canvas zoom across restart | not persisted (no session field) | not persisted | NOT VERIFIED / broken: persistence would need the host session (`main.go`), outside this card's write scope |

Resize pair: `ux03-resize-1360x880-before-light.png` → `ux03-resize-900x600-light.png` →
`ux03-resize-1360x880-after-light.png`. Restart: `ux03-restart-1360x880-light.png` (T10 selected,
rail scrolled, page `run` restored from `workspace-session.json`).

## Keyboard checklist (real GUI, automation `snapshot` focus/checked state)

| Key | Surface | Result | Observed |
|---|---|---|---|
| Tab | Run workspace / shell | PASS | Focus starts on the navigation: 项目工作区 → 实验设计 → 实验方案 (tree order, no trap) |
| Shift+Tab | Run workspace / shell | PASS | Reverse: 实验方案 → 实验设计 → 项目工作区 → sidebar splitter |
| Arrow Up/Down | Left navigation and flat flow rail | FAIL | Focus stays on `实验运行`; sidebar items are buttons and flat rail rows expose no focus target, so Up/Down do not move between them |
| Arrow Left/Right | Run workspace tabs | PASS | Focused `历史` + Right → `时间线` (checked, focused) |
| Enter | Shell navigation | PASS | Focused `实验设计` + Enter navigated to the Design surface |
| Enter | Recording view switcher | PASS | Focused `表格` + Enter switched the projection (孔位 / 样本 rows appeared) |
| Esc | Dialog dismissal | NOT VERIFIED | Ctrl+K command palette did not appear in automation snapshots on the Design surface, so a dialog Esc path could not be exercised |

## Automated checks

`cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (all packages). New tests, all FAIL on
`origin/main` / PASS with the fix:

- `TestStackedRunViewStartsAtRecordingAndKeepsTableFirstRow` (`internal/runworkspace`) — fails on
  origin/main at the recording-header assertion (`{Y:52 W:0 H:0}`); passes with the fix.
- `TestReadingFieldKeepsFocusAcrossBreakpointResize` (`internal/runworkspace`) — fails on
  origin/main with "A1 reading lost focus crossing to 900x600"; passes with the fix.
- `TestSelectingStepGroupReportsTaskKey` (`internal/runworkspace`) — fails on origin/main with
  `step group reported selection "step-read", want t-read`; passes with the fix.

## Gaps

- Design canvas zoom could not be driven from the real GUI: `mygo-automation` exposes wheel/pinch
  only as scroll, and the canvas zoom readout changes solely on the pinch gesture
  (`internal/workflow/view.go`, `gesture.Zoom`). Resize invariance follows from the state living on
  the Designer, not the layout; the restart gap needs `main.go` session persistence (forbidden here).
- Esc dialog dismissal is unverified for the reason above; no dialog could be opened through the
  automation surface.
- The left navigation and flat flow rail are not arrow-key navigable; recorded as FAIL, not fixed
  in this card (would add focusable row semantics across the shell and run rail).
