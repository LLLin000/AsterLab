package plate

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestPlateRecessedWellsAndNumericRollCounters(t *testing.T) {
	wells := map[string]WellState{
		"A1": {Group: "g-ctrl", SampleID: "Ctrl-1", Replicate: 1},
		"A2": {Group: "g-ctrl", SampleID: "Ctrl-2", Replicate: 2},
		"B1": {Group: "g-drug", SampleID: "Drug-1", Replicate: 1},
	}
	labels := map[string]string{
		"g-ctrl": "对照组",
		"g-drug": "给药组",
	}
	idxMap := map[string]int{"g-ctrl": 0, "g-drug": 1}
	selected := []string{"A1"}
	st := &InteractionState{}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		View(c, Props{
			Rows:        3,
			Cols:        4,
			Interactive: true,
			Wells:       wells,
			SelectedIDs: selected,
			GroupIndex:  idxMap,
			GroupLabels: labels,
			State:       st,
			OnSelect: func(ids []string) {
				selected = ids
			},
		})
	}, 640, 480)
	tt.Frame()
	if !tt.HasText("3 / 12 孔已分配") {
		t.Fatalf("expected initial 3 / 12 count, texts=%v", tt.Texts())
	}

	// Add an assigned well and trigger NumericRoll transition frames.
	wells["B2"] = WellState{Group: "g-drug", SampleID: "Drug-2", Replicate: 2}
	for range 8 {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("4 / 12 孔已分配") {
		t.Fatalf("expected updated 4 / 12 count after NumericRoll, texts=%v", tt.Texts())
	}

	// Verify mobile compact summary card & recessed touch mode selector + 1D rack at 402x874.
	var touchSt InteractionState
	mob := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing12)).Children(func() {
			View(c, Props{
				Rows:          8,
				Cols:          12,
				Interactive:   true,
				MobileCompact: true,
				Wells:         wells,
				SelectedIDs:   []string{"A1", "A2"},
				GroupIndex:    idxMap,
				GroupLabels:   labels,
			})
			RenderTouchModeSelector(c, &touchSt, func() {})
			Rack(c, RackProps{
				Interactive: true,
				Lanes: []LaneState{
					{Position: LanePosition(0), SampleID: "Ctrl-1", Group: "g-ctrl", HasValue: true},
					{Position: LanePosition(1)},
				},
			})
		})
	}, 402, 874)
	mob.Frame()
	for _, want := range []string{"已排 4 / 96 孔", "已选 2 孔", "点选", "涂抹框选", "2 管架", "已记录 1 · 缺失 1"} {
		if !mob.HasText(want) {
			t.Fatalf("missing %q in mobile plate/rack craft view, texts=%v", want, mob.Texts())
		}
	}
	for _, info := range mob.AccessAll() {
		if info.Offscreen || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
			continue
		}
		if info.Bounds.X < -1.5 || info.Bounds.X+info.Bounds.W > 403.0 {
			t.Fatalf("mobile plate/rack craft overflow: name=%q bounds=%+v", info.Name, info.Bounds)
		}
	}
}
