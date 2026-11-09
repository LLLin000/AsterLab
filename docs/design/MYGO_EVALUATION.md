# MyGo Native UI — prototype evaluation

- Date: 2026-11-09
- Version: `github.com/egoist/mygo v0.8.3`
- Scope: one Run screen, flow preview, 96-well plate
- Source: `prototypes/mygo-run/`
- Screenshot: `mygo-prototype.png`
- Status: **visual prototype only; no production migration.**

## What was tested

| Item | Result |
|---|---|
| Go build | pass |
| Native view tester (no window) | pass; PNG rendered |
| Custom titlebar | pass in the view; no Windows title bar |
| Project/design sidebar | pass |
| 2/3 record + 1/3 readiness | pass |
| Native checkbox, TextInput, Button | visible |
| Flow path drawing | pass; 5 nodes and connectors |
| 96-well grid | pass; 12×8 |
| Windows real-window launch | not conclusively measured (automation hung) |
| Rust CLI bridge | not tested; prototype uses synthetic data |
| Windows installer/signing | not tested |

Binary built during the test: ~16 MB (Go native UI, no WebView).

## Code size

`main.go`: ~190 lines for the complete screen, including flow and plate.
This is significantly smaller than the current Rune Run implementation for
the same visual scope.

## Strengths

- The visual result is closer to a designed desktop product than the current
  Rune implementation at the same effort.
- Native controls (checkbox, inputs, buttons, sidebar) have consistent spacing,
  focus, and shape.
- Canvas/path drawing and the 96-well grid are straightforward in one file.
- No WebView; small runtime; Agent can read one Go view function.
- `ui.Tester` renders a real PNG without a window, useful for visual CI.

## Risks

- MyGo is new (v0.8.3); long-term API stability is unproven.
- Rust integration is an experimental bridge, so adopting it would introduce
  a Go/Rust boundary around the existing Core.
- The prototype is synthetic; no real RunWorkspace, save, transaction, or
  restart test.
- Real-window startup/memory measurement was not completed.
- Custom flowchart editing (drag, snapping, undo, layout) is still app work;
  MyGo only supplies drawing and hit testing.

## Recommendation

MyGo is a credible **visual benchmark**, not a production replacement yet.
Use it to compare composition and component feel against Rune. Do not migrate
AsterLab until a second spike proves:

1. real Rust Core/CLI integration;
2. text input/save/restart with the same Run;
3. Windows startup, memory, and packaging;
4. Agent editability on a second screen.

The current Rune test remains the production candidate; MyGo should be judged
as a challenger with a much stronger native visual baseline.
