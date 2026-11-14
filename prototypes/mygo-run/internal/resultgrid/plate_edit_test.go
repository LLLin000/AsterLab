package resultgrid

import (
	"strconv"
	"strings"
	"testing"
)

func TestPlateEditCommitsOnlyOnSubmit(t *testing.T) {
	g := New(schema(), nil)
	draft, ok := g.BeginEdit(0, 0)
	if !ok {
		t.Fatal("BeginEdit rejected a valid cell")
	}
	*draft = "0.8."
	g.MarkEditDirty()
	if !g.EditDirty() {
		t.Fatal("draft not marked dirty")
	}
	if err := g.CommitEdit(); err == nil {
		t.Fatal("incomplete decimal was accepted")
	}
	if cell, _ := g.CellAt(0, 0); cell.Value != nil {
		t.Fatalf("failed commit wrote a value: %+v", cell)
	}
	if draft, _ = g.BeginEdit(0, 0); *draft != "0.8." {
		t.Fatalf("draft lost after failed commit: %q", *draft)
	}

	*draft = "0.82"
	g.MarkEditDirty()
	if err := g.CommitEdit(); err != nil {
		t.Fatal(err)
	}
	cell, _ := g.CellAt(0, 0)
	if cell.Value == nil || *cell.Value != 0.82 || cell.Status != "raw" {
		t.Fatalf("committed cell = %+v", cell)
	}
	if g.EditDirty() {
		t.Fatal("dirty flag survived a successful commit")
	}
	input, ok := g.InputAt(0, 0)
	if !ok || *input != "0.82" {
		t.Fatalf("committed input = %q, want 0.82", *input)
	}
	if g.Cells[0].Value == nil || *g.Cells[0].Value != 0.82 {
		t.Fatalf("shared cell value = %+v", g.Cells[0])
	}
}

func TestPlateSelectionCommitsPreviousCell(t *testing.T) {
	g := New(schema(), nil)
	draft, _ := g.BeginEdit(0, 0)
	*draft = "1.25"
	g.MarkEditDirty()

	if err := g.SelectCell(1, 2); err != nil {
		t.Fatal(err)
	}
	if cell, _ := g.CellAt(0, 0); cell.Value == nil || *cell.Value != 1.25 {
		t.Fatalf("previous cell was not committed: %+v", cell)
	}
	if g.Selection.EndRow != 1 || g.Selection.EndColumn != 2 {
		t.Fatalf("selection = %+v", g.Selection)
	}
	if draft, _ = g.BeginEdit(1, 2); *draft != "" {
		t.Fatalf("new cell draft should be empty, got %q", *draft)
	}
}

func TestPlateEditClearsToMissing(t *testing.T) {
	g := New(schema(), nil)
	draft, _ := g.BeginEdit(2, 3)
	*draft = "4.5"
	g.MarkEditDirty()
	if err := g.CommitEdit(); err != nil {
		t.Fatal(err)
	}
	draft, _ = g.BeginEdit(2, 3)
	*draft = ""
	g.MarkEditDirty()
	if err := g.CommitPendingEdit(); err != nil {
		t.Fatal(err)
	}
	cell, _ := g.CellAt(2, 3)
	if cell.Value != nil || cell.Status != "missing" {
		t.Fatalf("cleared cell = %+v", cell)
	}
}

func TestPlateColumnPasteFillsSelectedColumn(t *testing.T) {
	g := New(schema(), nil)
	g.SetSelection(Selection{StartRow: 0, StartColumn: 3, EndRow: 7, EndColumn: 3})
	if _, err := g.PasteTSV("1\n2\n3\n4\n5\n6\n7\n8"); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitPaste(); err != nil {
		t.Fatal(err)
	}
	for row := 0; row < 8; row++ {
		cell, _ := g.CellAt(row, 3)
		if cell.Value == nil || *cell.Value != float64(row+1) {
			t.Fatalf("row %d column 3 = %+v", row, cell)
		}
	}
	if cell, _ := g.CellAt(0, 4); cell.Value != nil {
		t.Fatalf("neighbouring column was modified: %+v", cell)
	}
}

func TestPaste96CellsUndoAsOneStep(t *testing.T) {
	g := New(schema(), nil)
	lines := make([]string, 8)
	for r := 0; r < 8; r++ {
		cells := make([]string, 12)
		for c := 0; c < 12; c++ {
			if (r+c)%4 == 0 {
				cells[c] = ""
			} else {
				cells[c] = strconv.Itoa(r*12 + c + 1)
			}
		}
		lines[r] = strings.Join(cells, "\t")
	}
	if _, err := g.PasteTSV(strings.Join(lines, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitPaste(); err != nil {
		t.Fatal(err)
	}
	if cell, _ := g.CellAt(0, 0); cell.Value != nil || cell.Status != "missing" {
		t.Fatalf("blank cell = %+v", cell)
	}
	if cell, _ := g.CellAt(7, 11); cell.Value == nil || *cell.Value != 96 {
		t.Fatalf("last cell = %+v", cell)
	}
	if !g.Undo() {
		t.Fatal("undo failed")
	}
	for _, cell := range g.Cells {
		if cell.Value != nil || cell.Status != "missing" {
			t.Fatalf("undo left a value: %+v", cell)
		}
	}
	if !g.Redo() {
		t.Fatal("redo failed")
	}
	if cell, _ := g.CellAt(7, 11); cell.Value == nil || *cell.Value != 96 {
		t.Fatalf("redo lost value: %+v", cell)
	}
}

func TestPasteRejectsInvalidInputWithoutMutation(t *testing.T) {
	g := New(schema(), nil)
	g.SetSelection(Selection{StartRow: 7, StartColumn: 11, EndRow: 7, EndColumn: 11})
	if _, err := g.PasteTSV("1\t2"); err == nil {
		t.Fatal("out-of-bounds paste accepted")
	}
	if cell, _ := g.CellAt(7, 11); cell.Value != nil {
		t.Fatalf("out-of-bounds paste mutated a cell: %+v", cell)
	}
	g.SetSelection(Selection{StartRow: 0, StartColumn: 0, EndRow: 0, EndColumn: 0})
	if _, err := g.PasteTSV("not-a-number"); err == nil {
		t.Fatal("non-numeric paste accepted")
	}
	if cell, _ := g.CellAt(0, 0); cell.Value != nil {
		t.Fatalf("non-numeric paste mutated a cell: %+v", cell)
	}
}

func TestPasteUndoRestoresPreviousValues(t *testing.T) {
	g := New(schema(), nil)
	first := 1.5
	if err := g.SetCell(Cell{Row: 0, Column: 0, Value: &first, Status: "raw"}); err != nil {
		t.Fatal(err)
	}
	g.SetSelection(Selection{StartRow: 0, StartColumn: 0, EndRow: 0, EndColumn: 0})
	if _, err := g.PasteTSV("9"); err != nil {
		t.Fatal(err)
	}
	if err := g.ConfirmPasteOverwrite(); err != nil {
		t.Fatal(err)
	}
	if cell, _ := g.CellAt(0, 0); cell.Value == nil || *cell.Value != 9 {
		t.Fatalf("overwrite failed: %+v", cell)
	}
	if !g.Undo() {
		t.Fatal("undo failed")
	}
	if cell, _ := g.CellAt(0, 0); cell.Value == nil || *cell.Value != 1.5 {
		t.Fatalf("undo did not restore previous value: %+v", cell)
	}
}
