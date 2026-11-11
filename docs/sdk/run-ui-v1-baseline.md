# Run UI v1 baseline

Status: layout baseline accepted for product polishing. Do not restructure the
Run page wholesale; only fix execution-blocking UX issues.

## Layout

- Left: vertical experiment flow. One card per step, connected by vertical
  arrows. The selected step is highlighted; child tasks expand inline.
- Right top: current step detail, ordered by what the operator needs next:
  1. what to do now;
  2. operation parameters;
  3. execution checks;
  4. what to record;
  5. next action;
  6. frozen metadata (path, assay, timepoint, step ID) as a muted last line.
- Right bottom: the recording module. The same Core-backed grid is shown as a
  plate, table, gallery, or form; the view switcher changes presentation only.

## Interaction contract

- The plate is the recording surface, not a preview. Click a well to select it;
  the selected well's value editor and sample/group/replicate identity sit
  directly below the plate.
- Click a row or column header to select the whole line. Shift+click extends a
  rectangular selection. Arrow keys move; Shift+arrows extend.
- `Ctrl+C` copies the selection as TSV; `Ctrl+V` pastes TSV at the selection
  anchor. A paste is one undo step, including overwrites. `Delete` clears the
  selection. Paste preview reports rows, columns, and overwrite count before
  writing.
- Saving commits the pending editor draft first, then sends the batch through
  the Core readout path. A failed save keeps the in-memory grid so the operator
  can retry; it never silently discards uncommitted cells.
- After a successful save and reopen, values and well identity come from the
  Run's frozen record, not from a local UI cache.

## Acceptance focus

- 96-cell Excel paste: coordinate mapping, blank cells, non-numeric rejection,
  out-of-bounds rejection, single-step undo/redo.
- Save failure and retry; restart and reopen; the same 96 values and sample
  identities are visible.
- A later Design revision does not rewrite an existing Run.

## Not frozen here

- Editing non-96 plate formats in the Run surface.
- The generic table editor beyond the shared grid model.
- RT-qPCR and other assay-specific workspaces.
- File gallery workflows beyond local asset links.

Those extend the same Workspace, recording module, and Core write path; they do
not replace this layout.
