package plate

import (
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
