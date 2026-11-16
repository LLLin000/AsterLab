package runworkspace

import (
	"testing"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func TestHistoryProjectionMemoizesByRevision(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	snap := makeSnapshot()
	s := NewState(vm, Callbacks{
		HistoryInputs: func() (runs.RunSnapshot, []runs.Asset) {
			return snap, nil
		},
	})
	first := s.historyProjection()
	if first == nil {
		t.Fatal("history projection must build from snapshot inputs")
	}
	if len(first.Summary) != 2 {
		t.Fatalf("summary rows = %d, want 2 frozen tasks", len(first.Summary))
	}
	// The getter re-runs every call (it only hands over stored inputs), but
	// the derivation must not: same revision returns the identical projection.
	if second := s.historyProjection(); second != first {
		t.Fatal("same revision must return the memoized projection")
	}
	snap.Revision = 7
	if third := s.historyProjection(); third == first {
		t.Fatal("new revision must rebuild the projection")
	}
	s.Callbacks.HistoryInputs = nil
	if s.historyProjection() != nil {
		t.Fatal("nil inputs must hide the projection")
	}
}

func TestHistoryTabRendersSharedProjection(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{
		HistoryInputs: func() (runs.RunSnapshot, []runs.Asset) {
			return makeSnapshot(), nil
		},
	})
	s.Selected = "t-read"
	task := vm.TaskByKey["t-read"]
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		historyTab(c, s, c.Theme(), theme.GetTokens(c.Theme()), task)
	}, 1000, 700)
	tt.Frame()
	for _, want := range []string{"计划 vs 实际", "采图对账", "细胞接种", "CCK-8 检测", "计划：OD450 0.82 AU"} {
		if !tt.HasText(want) {
			t.Fatalf("history missing %q: %q", want, tt.Texts())
		}
	}
}

func TestHistoryProjectionRebuildsOnRelink(t *testing.T) {
	snap := makeSnapshot()
	assets := []runs.Asset{
		{ID: "img-1", RootID: "r", RelativePath: "run/a1.tif", MediaType: "image/tiff",
			Link: runs.AssetLink{RunID: snap.RunID, TaskKey: "t-read", WellID: "A1", Timepoint: "D1", FOVIndex: 1}},
		{ID: "img-2", RootID: "r", RelativePath: "run/a2.tif", MediaType: "image/tiff",
			Link: runs.AssetLink{RunID: snap.RunID, TaskKey: "t-read", WellID: "A2", Timepoint: "D1", FOVIndex: 1}},
	}
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{
		HistoryInputs: func() (runs.RunSnapshot, []runs.Asset) {
			return snap, assets
		},
	})
	before := s.historyProjection()
	if filled := matrixFilled(before, "A1", "D1"); filled != 1 {
		t.Fatalf("A1/D1 filled slots = %d, want 1", filled)
	}
	// An LB-12-style coordinate relink: same Run revision, same asset
	// count, one well changed. Asset link writes never bump the Run
	// revision, so the count alone would reuse the stale projection.
	assets[0].Link.WellID = "A3"
	after := s.historyProjection()
	if after == before {
		t.Fatal("relinked coordinates must rebuild the projection")
	}
	// The vacated well leaves no phantom cell; the new well fills one slot.
	if filled := matrixFilled(after, "A1", "D1"); filled != -1 {
		t.Fatalf("A1/D1 filled slots = %d, want cell absent after relink", filled)
	}
	if filled := matrixFilled(after, "A3", "D1"); filled != 1 {
		t.Fatalf("A3/D1 filled slots = %d, want 1 after relink", filled)
	}
}

func matrixFilled(proj *record.RecordProjection, well, timepoint string) int {
	if proj == nil {
		return -1
	}
	for _, cell := range proj.Matrix {
		if cell.Well == well && cell.Timepoint == timepoint {
			return cell.FilledSlots()
		}
	}
	return -1
}
