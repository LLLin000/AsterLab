# MyGo Desktop v1 — Product Spec

## Shell

- Native MyGo title bar (hidden Windows frame) with app name, breadcrumb,
  sidebar toggle and window controls.
- Left rail: 实验设计 / Protocol / 实验运行 / 资料与工具.
- Sidebar: project folder → design files; selecting a design opens it.
- Content max width; 1200×760 baseline; 900×600 usable; Dark and Light.
- Tokens: warm canvas `#f7f6f3`, surface `#fff`, hairline `#e7e3db`,
  accent `#5b45c9`; typography 20/16/14.5/12.5.

## Designer

- Top: SVG workflow canvas with experiment-shaped nodes (flask, plate,
  pipette), connectors, selection, pan/zoom.
- Add, delete, duplicate, move, connect, undo/redo; selected node drives a
  bottom inspector.
- Bottom: current step details, component instances, parameters.
- Design save is explicit; revision conflicts show reload/discard options.
- Layout coordinates are presentation metadata; they do not change Protocol
  sequence or frozen Run facts.

## Blocks

First set: Note, Parameter Table, Measurement Table, 96-well Plate,
Dilution/Calculator, Checklist, nested Workflow.
Each has a stable definition ID/version, typed props, editor, read-only view,
validation and serialization. Shared definitions are versioned; editing a
shared definition never mutates a frozen Run.

## Run

- 2/3 record + 1/3 readiness.
- Snapshot tasks/timepoints/status/checks/measurements all from Core.
- Current task first; save readout and complete task are separate actions.
- Save states: idle/saving/saved/error; failure preserves draft.
- Readiness shows actual material/instrument/plate binding or `未绑定`.
- Static placeholders must be labelled `示意`; no unlabeled demo data in v1.

## Agent-ready design

Structured, versioned design documents with stable IDs; Agent edits are
validated patches with preview, never direct SQLite writes. No chat system in
v1.

## Completion path

Create/open design → edit workflow/components → save revision → create Run →
execute checks → save two readouts → quit → reopen → same data; CLI reads the
same Run; Windows installed EXE + DLL loads.
