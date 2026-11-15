package plate

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTwelveWellGeometry(t *testing.T) {
	want := []string{"A1", "A2", "A3", "A4", "B1", "B2", "B3", "B4", "C1", "C2", "C3", "C4"}
	got := make([]string, 0, 12)
	for row := range 3 {
		for col := range 4 {
			got = append(got, wellID(row, col))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("12-well geometry = %v, want %v", got, want)
	}

	tester := ui.NewTester(func(c *ui.Context) { View(c, Props{Rows: 3, Cols: 4}) }, 320, 240)
	tester.Frame()
	if !tester.HasText("0 / 12 孔已分配 · 单击选择，Shift+单击多选") {
		t.Fatalf("12-well plate not rendered: %q", tester.Texts())
	}
}

func TestPlateGeometriesAndWellSelectionRoundTrip(t *testing.T) {
	formats := []struct {
		format, rows, cols int
	}{
		{6, 2, 3}, {12, 3, 4}, {24, 4, 6}, {48, 6, 8}, {96, 8, 12}, {384, 16, 24},
	}
	for _, format := range formats {
		t.Run(fmt.Sprint(format.format), func(t *testing.T) {
			ids := make([]string, 0, format.rows*format.cols)
			for row := range format.rows {
				for col := range format.cols {
					id := wellID(row, col)
					gotRow, gotCol, ok := wellPosition(id, format.rows, format.cols)
					if !ok || gotRow != row || gotCol != col {
						t.Fatalf("%s round trip = (%d,%d,%v), want (%d,%d,true)", id, gotRow, gotCol, ok, row, col)
					}
					ids = append(ids, id)
				}
			}
			if len(ids) != format.format {
				t.Fatalf("geometry generated %d wells, want %d", len(ids), format.format)
			}
			tester := ui.NewTester(func(c *ui.Context) {
				View(c, Props{Rows: format.rows, Cols: format.cols})
			}, 900, 600)
			tester.Frame()
			if !tester.HasText(fmt.Sprintf("0 / %d 孔已分配 · 单击选择，Shift+单击多选", format.format)) {
				t.Fatalf("plate render for %d wells = %q", format.format, tester.Texts())
			}
		})
	}
	if _, _, ok := wellPosition("P25", 16, 24); ok {
		t.Fatal("accepted well outside 384-well geometry")
	}

	selected := map[string]bool{}
	selected = nextSelection(selected, "P24", false)
	if !selected["P24"] || len(selected) != 1 {
		t.Fatalf("select = %v", selected)
	}
	selected = nextSelection(selected, "A1", true)
	if len(selected) != 2 || !selected["P24"] || !selected["A1"] {
		t.Fatalf("additive select = %v", selected)
	}
	selected = nextSelection(selected, "P24", true)
	if len(selected) != 1 || !selected["A1"] {
		t.Fatalf("additive deselect = %v", selected)
	}
}
