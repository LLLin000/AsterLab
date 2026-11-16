package resultgrid

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

func TestPlateLabelReadsWellCount(t *testing.T) {
	if got := plateLabel(3, 4); got != "12 孔板" {
		t.Fatalf("plateLabel(3,4) = %q", got)
	}
	if got := plateLabel(8, 12); got != "96 孔板" {
		t.Fatalf("plateLabel(8,12) = %q", got)
	}
	if got := plateLabel(2, 3); got != "6 孔板" {
		t.Fatalf("plateLabel(2,3) = %q", got)
	}
	if got := plateLabel(2, 5); got != "2 × 5 孔板" {
		t.Fatalf("plateLabel(2,5) = %q", got)
	}
}

func blindGrid(blind bool) *Grid {
	schema := Schema{
		ID: "tube.plate12.fov.v1", Version: 1,
		RowLabels:    []string{"A", "B", "C"},
		ColumnLabels: []string{"1", "2", "3", "4"},
		Fields:       []Field{{Key: "images", Label: "图像", Unit: "张"}},
	}
	value := 6.0
	grid := New(schema, []Cell{
		{Row: 0, Column: 0, SampleID: "HUVEC-3%-1", Group: "3% GelMA", Field: "images", Value: &value, Status: "raw"},
		{Row: 1, Column: 0, SampleID: "HUVEC-5%-1", Group: "5% GelMA", Field: "images", Value: &value, Status: "raw"},
	})
	grid.Blind = blind
	return grid
}

func TestPlateLegendBlindMask(t *testing.T) {
	render := func(g *Grid) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			PlateView(c, g)
		}, 900, 600)
		tt.Frame()
		return tt
	}
	open := render(blindGrid(false))
	if !open.HasText("3% GelMA") || !open.HasText("12 孔板") {
		t.Fatalf("unmasked legend missing group or label: %q", open.Texts())
	}
	masked := render(blindGrid(true))
	if !masked.HasText("已设盲 · 分组已隐藏") {
		t.Fatalf("blind legend missing mask notice: %q", masked.Texts())
	}
	if masked.HasText("3% GelMA") || masked.HasText("5% GelMA") {
		t.Fatalf("blind legend leaks group identity: %q", masked.Texts())
	}
	if !masked.HasText("12 孔板") {
		t.Fatalf("blind plate must keep its labware label: %q", masked.Texts())
	}
}
