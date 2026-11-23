package shell

import (
	"math"
	"testing"
)

func TestQuickCalcCompleteUnitsAndCompatibility(t *testing.T) {
	// UI-07: Complete wet-lab units including X, %, g/L, pM, U/mL, nL, L.
	cases := []struct {
		name      string
		st        RightCalcState
		wantV1UL  float64
		wantDilUL float64
	}{
		{
			name:      "10X PBS to 1X in 500 mL",
			st:        RightCalcState{Stock: "10", StockUnit: "X", Target: "1", TargetUnit: "X", FinalVol: "500", VolUnit: "mL"},
			wantV1UL:  50000,
			wantDilUL: 450000,
		},
		{
			name:      "10% to 0.5% in 10 mL",
			st:        RightCalcState{Stock: "10", StockUnit: "%", Target: "0.5", TargetUnit: "%", FinalVol: "10", VolUnit: "mL"},
			wantV1UL:  500,
			wantDilUL: 9500,
		},
		{
			name:      "100 nM to 10 pM in 10 mL",
			st:        RightCalcState{Stock: "100", StockUnit: "nM", Target: "10", TargetUnit: "pM", FinalVol: "10", VolUnit: "mL"},
			wantV1UL:  1,
			wantDilUL: 9999,
		},
		{
			name:      "1000 U/mL to 50 mU/mL in 20 mL",
			st:        RightCalcState{Stock: "1000", StockUnit: "U/mL", Target: "50", TargetUnit: "mU/mL", FinalVol: "20", VolUnit: "mL"},
			wantV1UL:  1,
			wantDilUL: 19999,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v1, dil, msg := computeQuickDilution(tc.st)
			if msg != "" {
				t.Fatalf("unexpected error: %s", msg)
			}
			if math.Abs(v1-tc.wantV1UL) > 0.01 || math.Abs(dil-tc.wantDilUL) > 0.01 {
				t.Fatalf("got v1=%.3f dil=%.3f, want v1=%.3f dil=%.3f", v1, dil, tc.wantV1UL, tc.wantDilUL)
			}
		})
	}

	// UI-08: Compatible target units for mass concentration never include molar units.
	for _, u := range compatibleTargetUnitsFor("mg/mL") {
		if fam, _ := quickConcUnitBase(u); fam != "mass" {
			t.Fatalf("incompatible unit %q (%s) listed for mg/mL", u, fam)
		}
	}
	for _, u := range compatibleTargetUnitsFor("mM") {
		if fam, _ := quickConcUnitBase(u); fam != "molar" {
			t.Fatalf("incompatible unit %q (%s) listed for mM", u, fam)
		}
	}
}

func TestRightDrawerContextPerActiveTab(t *testing.T) {
	s := &Shell{}
	s.EnsureTabs()
	s.OpenCalendarTab()
	if got := s.activeDrawerContext(); got != drawerCtxCalendar {
		t.Fatalf("calendar tab should yield drawerCtxCalendar, got %d", got)
	}
	s.OpenPantryTab()
	if got := s.activeDrawerContext(); got != drawerCtxPantry {
		t.Fatalf("pantry tab should yield drawerCtxPantry, got %d", got)
	}
}
