package resultgrid

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

func TestPlateLabelReads1DAsRack(t *testing.T) {
	if got := plateLabel(1, 8); got != "8 管架" {
		t.Fatalf("plateLabel(1,8) = %q, want 8 管架", got)
	}
	if got := plateLabel(8, 1); got != "8 管架" {
		t.Fatalf("plateLabel(8,1) = %q, want 8 管架", got)
	}
	// 2D labware keeps its well-count label even at matching counts.
	if got := plateLabel(3, 4); got != "12 孔板" {
		t.Fatalf("plateLabel(3,4) = %q, want 12 孔板", got)
	}
}

// TestPlateViewRendersLaneStrip checks the 1D contract end to end: Core
// canonical Lane<N> positions, sample coordinates, the shared palette, and
// blind masking without leaking groups.
func TestPlateViewRendersLaneStrip(t *testing.T) {
	schema := Schema{
		ID: "tube.rack8.v1", Version: 1,
		RowLabels:    []string{"A"},
		ColumnLabels: []string{"1", "2", "3", "4", "5", "6", "7", "8"},
		Fields:       []Field{{Key: "cfu", Label: "CFU", Unit: "CFU"}},
	}
	value := 42.0
	grid := New(schema, []Cell{
		{Row: 0, Column: 0, SampleID: "M3-1", Group: "给药组", Field: "cfu", Value: &value, Status: "raw"},
		{Row: 0, Column: 2, SampleID: "M3-3", Group: "对照组", Field: "cfu", Status: "excluded"},
	})
	render := func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		PlateView(c, grid)
	}
	open := ui.NewTester(render, 900, 600)
	open.Frame()
	for _, want := range []string{"8 管架", "Lane1", "Lane3", "M3-1", "给药组", "排除 1"} {
		if !open.HasText(want) {
			t.Fatalf("lane strip missing %q: %q", want, open.Texts())
		}
	}
	grid.Blind = true
	masked := ui.NewTester(render, 900, 600)
	masked.Frame()
	if masked.HasText("给药组") || masked.HasText("对照组") {
		t.Fatalf("blind lane strip leaks groups: %q", masked.Texts())
	}
	for _, want := range []string{"8 管架", "Lane1", "M3-1", "已设盲 · 分组已隐藏"} {
		if !masked.HasText(want) {
			t.Fatalf("blind lane strip missing %q: %q", want, masked.Texts())
		}
	}
}
