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
	if !tester.HasText("0 / 12 孔已分配 · 悬停查看分组与样本明细") {
		t.Fatalf("12-well plate not rendered: %q", tester.Texts())
	}
}

func TestPlateGeometriesAndWellSelectionRoundTrip(t *testing.T) {
	formats := []struct {
		format, rows, cols int
	}{
		{4, 2, 2}, {6, 2, 3}, {12, 3, 4}, {24, 4, 6}, {48, 6, 8}, {96, 8, 12}, {384, 16, 24},
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
			if !tester.HasText(fmt.Sprintf("0 / %d 孔已分配 · 悬停查看分组与样本明细", format.format)) {
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

func TestAdaptiveMetricsAndInteractiveSelection(t *testing.T) {
	w4, _, _ := AdaptiveMetrics(2, 2, "plate", 1)
	w6, _, _ := AdaptiveMetrics(2, 3, "plate", 1)
	w12, _, _ := AdaptiveMetrics(3, 4, "plate", 1)
	w24, _, _ := AdaptiveMetrics(4, 6, "plate", 1)
	w48, _, _ := AdaptiveMetrics(6, 8, "plate", 1)
	w96, _, _ := AdaptiveMetrics(8, 12, "plate", 1)
	w384, _, _ := AdaptiveMetrics(16, 24, "plate", 1)
	if !(w4 > w6 && w6 > w12 && w12 > w24 && w24 > w48 && w48 > w96 && w96 > w384) {
		t.Fatalf("expected strictly decreasing adaptive widths, got %.1f > %.1f > %.1f > %.1f > %.1f > %.1f > %.1f", w4, w6, w12, w24, w48, w96, w384)
	}
	rw, rh, _ := AdaptiveMetrics(1, 8, "rack_1d", 1)
	if rh <= rw {
		t.Fatalf("rack_1d should render vertical capsule (CellH > CellW), got %gx%g", rw, rh)
	}

	var gotSelected []string
	st := &InteractionState{}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, Props{
			Rows: 3, Cols: 4, Interactive: true,
			SelectedIDs: gotSelected,
			OnSelect: func(ids []string) {
				gotSelected = ids
			},
			State: st,
		})
	}, 480, 360)
	tt.Frame()
	if err := tt.Click("选择第 B 行"); err != nil {
		t.Fatalf("row header click failed: %v", err)
	}
	tt.Frame()
	want := []string{"B1", "B2", "B3", "B4"}
	if !reflect.DeepEqual(gotSelected, want) {
		t.Fatalf("row selection = %v, want %v", gotSelected, want)
	}
}
