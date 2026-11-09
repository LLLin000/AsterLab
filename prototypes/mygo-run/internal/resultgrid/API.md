# ResultGrid API

Package: `asterlab-mygo-prototype/internal/resultgrid`

- `Schema`, `Field`, and `Cell` are JSON-friendly recording types; `Schema.Metadata` carries assay configuration such as reference genes and calibrator group. Schema labels define zero-based row/column coordinates. A nil `Cell.Value` with `Status: "missing"` is explicitly missing, not zero; `Cell.Status` remains free-form for statuses such as `raw`, `missing`, `invalid`, and `undetected`.
- `New(schema Schema, cells []Cell) *Grid` initializes missing cells and overlays supplied cells.
- `Grid.SetCell(Cell) error`, `CellAt(row, column) (Cell, bool)`, `Rows()`, and `Columns()` access/edit the draft.
- `Grid.SetSelection(Selection)` sets an inclusive rectangular selection; `SelectedCells() []Cell` reads it. `MoveSelection("left"|"right"|"up"|"down"|"tab"|"enter", extend bool)` provides bounded keyboard movement and rectangular extension.
- `Grid.PasteTSV(text string) (*PastePreview, error)` parses TSV (including CSV quote escaping) into a non-mutating preview. `CommitPaste()` applies when no populated cell would be overwritten; `ConfirmPasteOverwrite()` explicitly applies overwrite previews.
- `Grid.Undo() bool` and `Grid.Redo() bool` undo and redo edits.
- `Grid.State` is `StateDraft`, `StateSaving`, `StateSaved`, or `StateError`; this component does not save or fake persistence. The host owns state transitions and errors.
- `View(ctx, grid, Options{FullWidth: true})` draws the editable native table; `Options{}` uses constrained column widths.

No Core, database, or assay imports are used by this package.