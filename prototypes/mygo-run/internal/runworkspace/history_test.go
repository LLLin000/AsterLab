package runworkspace

import (
	"testing"

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
