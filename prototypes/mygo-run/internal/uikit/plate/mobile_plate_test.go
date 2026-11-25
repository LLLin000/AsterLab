package plate

import (
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMobileInlinePlateCompactSummaryCard(t *testing.T) {
	wells96 := map[string]WellState{
		"A1": {Group: "g-ctrl", SampleID: "Ctrl", Replicate: 1},
		"A2": {Group: "g-ctrl", SampleID: "Ctrl", Replicate: 2},
		"B1": {Group: "g-drug", SampleID: "Drug-10", Replicate: 1},
		"B2": {Group: "g-drug", SampleID: "Drug-10", Replicate: 2},
	}
	labels := map[string]string{
		"g-ctrl": "对照组",
		"g-drug": "给药组",
	}

	// Narrow mobile viewport (< 680dp) renders 96-well plate as compact summary card.
	tt96 := ui.NewTester(func(c *ui.Context) {
		View(c, Props{
			Rows: 8, Cols: 12,
			Interactive: true,
			Wells:       wells96,
			GroupLabels: labels,
		})
	}, 390, 844)
	tt96.Frame()

	for _, want := range []string{"96 孔板 · 8×12", "已排 4 / 96 孔", "对照组 (2)", "给药组 (2)", "全屏排板 / 读数"} {
		if !tt96.HasText(want) {
			t.Fatalf("mobile 96-well summary card missing %q in %v", want, tt96.Texts())
		}
	}
	if _, crushedWellFound := tt96.Find("孔 A1"); crushedWellFound {
		t.Fatalf("96-well inline grid should NOT render crushed individual wells on <680dp viewport")
	}

	// 384-well plate on mobile also collapses to compact summary card.
	tt384 := ui.NewTester(func(c *ui.Context) {
		View(c, Props{
			Rows: 16, Cols: 24,
			Interactive: true,
			Wells:       wells96,
			GroupLabels: labels,
		})
	}, 390, 844)
	tt384.Frame()
	if !tt384.HasText("384 孔板 · 16×24") || !tt384.HasText("已排 4 / 384 孔") || !tt384.HasText("全屏排板 / 读数") {
		t.Fatalf("mobile 384-well summary card unexpected texts: %v", tt384.Texts())
	}

	// MobileCompact=true explicitly forces compact card even on wide viewport.
	ttForced := ui.NewTester(func(c *ui.Context) {
		View(c, Props{
			Rows: 8, Cols: 12,
			Interactive:   true,
			MobileCompact: true,
			Wells:         wells96,
			GroupLabels:   labels,
		})
	}, 1024, 768)
	ttForced.Frame()
	if !ttForced.HasText("全屏排板 / 读数") {
		t.Fatalf("MobileCompact=true should render compact summary card, texts=%v", ttForced.Texts())
	}
}

func settleSheetAnim(tt *ui.Tester) {
	for range 120 {
		tt.Advance(16 * time.Millisecond)
	}
	tt.Frame()
}

func TestMobilePlateFullscreenSheetAndTouchModes(t *testing.T) {
	st := &InteractionState{}
	var selected []string
	var assignedGroup string
	var assignedWells []string

	wells := map[string]WellState{
		"H12": {Group: "g-ctrl", Role: "ntc"},
	}
	labels := map[string]string{
		"g-ctrl": "对照组",
		"g-drug": "给药组",
	}
	idxMap := map[string]int{"g-ctrl": 0, "g-drug": 1}

	tt := ui.NewTester(func(c *ui.Context) {
		View(c, Props{
			Rows:        8,
			Cols:        12,
			Interactive: true,
			Wells:       wells,
			SelectedIDs: selected,
			GroupIndex:  idxMap,
			GroupLabels: labels,
			State:       st,
			OnSelect: func(ids []string) {
				selected = ids
			},
			OnAssignGroup: func(gid string, wellIDs []string) {
				assignedGroup = gid
				assignedWells = append([]string(nil), wellIDs...)
				for _, w := range wellIDs {
					wells[w] = WellState{Group: gid, SampleID: labels[gid]}
				}
				selected = nil
			},
		})
	}, 390, 844)
	tt.Frame()

	// 1. Open fullscreen Zoomable Sheet from compact summary card.
	if st.MobileSheetOpen {
		t.Fatal("expected MobileSheetOpen initially false")
	}
	if err := tt.Click("全屏排板 / 读数"); err != nil {
		t.Fatalf("click 全屏排板 / 读数: %v", err)
	}
	tt.Frame()
	settleSheetAnim(tt)

	if !st.MobileSheetOpen {
		t.Fatal("expected MobileSheetOpen to become true")
	}
	for _, want := range []string{"点选", "涂抹框选", "按行·按列快选", "完成"} {
		if !tt.HasText(want) {
			t.Fatalf("fullscreen sheet missing %q in %v", want, tt.Texts())
		}
	}

	// 2. Tap mode (点选): multi-tap toggle without Shift.
	if st.TouchMode != TouchModeTap {
		t.Fatalf("expected default TouchModeTap, got %v", st.TouchMode)
	}
	if err := tt.Click("孔 A1"); err != nil {
		t.Fatalf("tap A1: %v", err)
	}
	tt.Frame()
	if err := tt.Click("孔 B2"); err != nil {
		t.Fatalf("tap B2: %v", err)
	}
	tt.Frame()
	if !reflect.DeepEqual(selected, []string{"A1", "B2"}) {
		t.Fatalf("tap mode selection = %v, want [A1 B2]", selected)
	}
	// Tap A1 again to deselect it while keeping B2.
	if err := tt.Click("孔 A1"); err != nil {
		t.Fatalf("tap A1 deselect: %v", err)
	}
	tt.Frame()
	if !reflect.DeepEqual(selected, []string{"B2"}) {
		t.Fatalf("tap mode deselect = %v, want [B2]", selected)
	}

	// 3. Brush mode (涂抹框选): finger drag from C1 to D3 sweeps rectangle C1..C3, D1..D3.
	if err := tt.Click("涂抹框选"); err != nil {
		t.Fatalf("switch to brush mode: %v", err)
	}
	tt.Frame()
	if st.TouchMode != TouchModeBrush {
		t.Fatalf("expected TouchModeBrush, got %v", st.TouchMode)
	}
	rC1, ok1 := tt.Find("孔 C1")
	rD3, ok2 := tt.Find("孔 D3")
	if !ok1 || !ok2 {
		t.Fatalf("expected C1 and D3 rects inside sheet: ok1=%v ok2=%v", ok1, ok2)
	}
	tt.FingerDown(rC1.X+rC1.W/2, rC1.Y+rC1.H/2)
	tt.FingerMove(rD3.X+rD3.W/2, rD3.Y+rD3.H/2)
	tt.FingerUp(rD3.X+rD3.W/2, rD3.Y+rD3.H/2)
	tt.Frame()
	for _, wantWell := range []string{"B2", "C1", "C2", "C3", "D1", "D2", "D3"} {
		found := false
		for _, s := range selected {
			if s == wantWell {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("brush drag missing %s in selected=%v", wantWell, selected)
		}
	}

	// 4. Row/Col quick select mode (按行·按列快选).
	if err := tt.Click("按行·按列快选"); err != nil {
		t.Fatalf("switch to row/col mode: %v", err)
	}
	tt.Frame()
	if st.TouchMode != TouchModeRowCol {
		t.Fatalf("expected TouchModeRowCol, got %v", st.TouchMode)
	}
	if err := tt.Click("清空选区"); err != nil {
		t.Fatalf("click 清空选区: %v", err)
	}
	tt.Frame()
	if len(selected) != 0 {
		t.Fatalf("expected cleared selection, got %v", selected)
	}

	// Default RowColAxis selects full row when tapping any well in that row.
	if err := tt.Click("孔 E4"); err != nil {
		t.Fatalf("click E4 in row mode: %v", err)
	}
	tt.Frame()
	if len(selected) != 12 || selected[0] != "E1" || selected[11] != "E9" {
		// sorted lexicographically: E1, E10, E11, E12, E2..E9
		if len(selected) != 12 {
			t.Fatalf("expected 12 wells in row E, got %v", selected)
		}
	}

	// Switch to 整列 axis and tap A6 -> adds all 8 wells of column 6.
	if err := tt.Click("整列"); err != nil {
		t.Fatalf("click 整列: %v", err)
	}
	tt.Frame()
	if err := tt.Click("孔 A6"); err != nil {
		t.Fatalf("click A6 in col mode: %v", err)
	}
	tt.Frame()
	if len(selected) != 19 { // 12 in row E + 8 in col 6 - 1 overlap (E6)
		t.Fatalf("expected 19 wells (row E + col 6), got %d: %v", len(selected), selected)
	}

	// 5. Zoomable controls inside sheet.
	if err := tt.Click("放大孔板"); err != nil {
		t.Fatalf("click 放大孔板: %v", err)
	}
	tt.Frame()
	if st.ZoomState.Scale <= 1 {
		t.Fatalf("expected ZoomState.Scale > 1 after zoom in, got %v", st.ZoomState.Scale)
	}
	if err := tt.Click("重置"); err != nil {
		t.Fatalf("click 重置: %v", err)
	}
	tt.Frame()
	if st.ZoomState.Scale != 1 {
		t.Fatalf("expected ZoomState.Scale == 1 after reset, got %v", st.ZoomState.Scale)
	}

	// 6. Thumb-friendly group assignment bar.
	if err := tt.Click("清空选区"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("点选"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("孔 A1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("归入分组 对照组"); err != nil {
		t.Fatalf("click group assignment chip: %v", err)
	}
	tt.Frame()
	if assignedGroup != "g-ctrl" || !reflect.DeepEqual(assignedWells, []string{"A1"}) {
		t.Fatalf("expected OnAssignGroup(g-ctrl, [A1]), got (%q, %v)", assignedGroup, assignedWells)
	}

	// 7. Close sheet cleanly back to compact summary card.
	if err := tt.Click("完成"); err != nil {
		t.Fatalf("click 完成: %v", err)
	}
	tt.Frame()
	settleSheetAnim(tt)
	if st.MobileSheetOpen {
		t.Fatal("expected MobileSheetOpen false after clicking 完成")
	}
	if !tt.HasText("已排 2 / 96 孔") {
		t.Fatalf("expected compact summary card updated to 2/96 wells, texts=%v", tt.Texts())
	}
}
