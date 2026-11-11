# AsterLab MyGo prototype

Isolated visual prototype; **not production code**.

Run:

```sh
go run .
```

Headless snapshot (no window):

```sh
go test -run TestSnap
```

The snapshot is written to `docs/design/mygo-prototype.png`.

It uses MyGo Native UI, the **Onevra** component library (`github.com/oneo-me/onevra/source`) for the window shell and every
control, and the generated Rust Native Module. The module calls the shared `asterlab-app` library, which includes the
existing CLI command implementation over SQLite.

What Onevra owns:

- `shells/sidebar`: the window frame, floating header, custom caption buttons, and the resizable navigation column
- `components`: sidebar rows, segmented tabs, buttons, text boxes, badges, dialogs, split dividers, typography
- `icons/lucide`: every icon

AsterLab supplies only the palette (`internal/theme`, layered on Onevra's warm palettes with the DESIGN.md colors) and the
workflow canvas: dot grid, edges, ports, and the 96-well plate. Workflow step cards are still real elements, built from the
same Onevra card materials (surface, hairline border, 14 DIP corners, soft shadow, `Rem` type scale) so they match the rest
of the window.

Design is two layers: the flow (steps and edges on the canvas) and the modules each step needs. `internal/blockui` renders
every block type in `internal/blocks` once, in two modes: Design edits the planned values on the step's own instance, Run
renders the recorded values read-only. The inspector's cards are those modules (`稀释计算器`, `96 孔板`, `参数表`, `检查清单`,
`备注`) plus the design's assay and well mappings, so a module cannot drift from the data it represents.

A Run task carries `designStepID`, bound from the design's explicit `stepTasks` (step + assay + timepoint, optionally a
protocol step path when that pair occurs under several paths). One assay can therefore run at D1/D3/D7 under three steps; a
missing, duplicated or path-ambiguous entry stays unresolved, and only designs saved before the table existed use the legacy
unique-assay fallback.
The Run surface renders that step's modules with the same `internal/blockui` (Run: frozen plan read-only; Review: plan plus
recorded results). Unresolved tasks say so instead of guessing from titles. Actual readouts still go through
`SaveReadoutsBatch` execution events. 创建运行 first shows a preflight dialog for unresolved links; invalid well mappings stay
blocking errors. The Run header switches to **Notebook**, a read-only timeline projected from stored events (one entry per
batch, source references, occurred vs recorded time, explicit unknown states). The Protocol page reads the run's frozen
Design Revision when one is loaded and renders it with MyGo's markdown view, so a printed protocol never drifts from the
executed design. The Run's 运行日志 uses Onevra `Tabs`, `List` and `Collapsible` in an activity-feed layout (day headings, icon per
record, right-aligned time, folded details); `internal/notebook` only projects stored data. The standalone Notebook is a
Markdown editor/preview whose saves append `stage=notebook` observation events to the Run's audit trail. The Run's 汇总 tab derives per-sample recorded/missing/excluded counts with mean and sample SD, labelled as
technical replicates — never as an independent biological n.

AsterDoc-backed Runs pin the exact document revision and source hash. Notebook timelines and Protocol use the same read-only
`asterdoc.View` registry as Design; Run/Review can open that frozen document without substituting a newer draft. Export PDF
re-reads and verifies the pinned revision, includes the frozen experiment plan and linked Run, and writes only the chosen file.
The native save picker confirms overwrites; the success toast offers to open the PDF. PDF pagination, Unicode font embedding,
and deterministic output use gofpdf; Chinese export requires an installed CJK TrueType font.

The canvas and plate draw only planned design data; `LoadRun`/`SaveReadout` use the real SQLite Run. See `NATIVE_MODULE.md`.

With `ASTERLAB_RUN_ID` unset, the Run workspace stays empty instead of loading a fixed run. Switching away from an unsaved design requires an explicit save or discard choice. Empty saved designs start blank; workflow nodes and well assignments are added explicitly.
