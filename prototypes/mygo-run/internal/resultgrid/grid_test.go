package resultgrid

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func schema() Schema {
	rows := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	cols := make([]string, 12)
	for i := range cols {
		cols[i] = string(rune('1' + i))
	}
	return Schema{ID: "test", Version: 1, RowLabels: rows, ColumnLabels: cols, Fields: []Field{{Key: "od450", Unit: "AU", DataType: "number"}}}
}
func TestEightByTwelvePasteAndSelection(t *testing.T) {
	g := New(schema(), nil)
	var input string
	for r := 0; r < 8; r++ {
		if r > 0 {
			input += "\n"
		}
		for c := 0; c < 12; c++ {
			if c > 0 {
				input += "\t"
			}
			input += "1.25"
		}
	}
	p, err := g.PasteTSV(input)
	if err != nil {
		t.Fatal(err)
	}
	if p.Rows != 8 || p.Columns != 12 || len(p.Cells) != 96 {
		t.Fatalf("preview dimensions: %+v cells=%d", p, len(p.Cells))
	}
	if err := g.CommitPaste(); err != nil {
		t.Fatal(err)
	}
	g.SetSelection(Selection{1, 2, 3, 5})
	selected := g.SelectedCells()
	if len(selected) != 12 {
		t.Fatalf("selected %d cells", len(selected))
	}
	if selected[0].Row != 1 || selected[0].Column != 2 || selected[11].Row != 3 || selected[11].Column != 5 {
		t.Fatalf("wrong rectangle: %+v", selected)
	}
}
func TestBlankInvalidOverwriteAndUndo(t *testing.T) {
	g := New(schema(), nil)
	if _, err := g.PasteTSV("not-number"); err == nil {
		t.Fatal("invalid value accepted")
	}
	p, err := g.PasteTSV("\t0")
	if err != nil || len(p.Cells) != 2 {
		t.Fatalf("blank/zero parse: %+v %v", p, err)
	}
	if err := g.CommitPaste(); err != nil {
		t.Fatal(err)
	}
	if a, _ := g.CellAt(0, 0); a.Value != nil || a.Status != "missing" {
		t.Fatalf("blank became value: %+v", a)
	}
	if b, _ := g.CellAt(0, 1); b.Value == nil || *b.Value != 0 {
		t.Fatalf("zero lost: %+v", b)
	}
	g.SetSelection(Selection{0, 1, 0, 1})
	_, _ = g.PasteTSV("2")
	if !g.ConfirmOverwrite {
		t.Fatal("overwrite confirmation not required")
	}
	if err := g.CommitPaste(); err == nil {
		t.Fatal("overwrite committed without confirmation")
	}
	if err := g.ConfirmPasteOverwrite(); err != nil {
		t.Fatal(err)
	}
	if !g.Undo() {
		t.Fatal("undo failed")
	}
	if !g.Redo() {
		t.Fatal("redo failed")
	}
}
func TestGridUIAndScreenshot(t *testing.T) {
	g := New(schema(), nil)
	tt := ui.NewTester(func(c *ui.Context) { View(c, g, Options{FullWidth: true}) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("resultgrid.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
