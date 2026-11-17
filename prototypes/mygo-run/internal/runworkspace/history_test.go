package runworkspace

import (
	"encoding/json"
	"errors"
	"testing"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

func historyProjFixture() record.Projection {
	return record.Projection{
		RunID: "run-cck8",
		Parameters: []record.ParamCompare{
			{
				ScopeID:  "root/read",
				Slot:     "od450",
				Label:    new("OD450"),
				Unit:     new("AU"),
				Planned:  json.RawMessage(`0.82`),
				Actual:   []record.ActualParam{{Value: json.RawMessage(`0.82`), Unit: new("AU"), EventID: "e1"}},
				Status:   "matches",
			},
		},
		Images: record.ImageMatrix{
			FOVPerWell: 1,
			Plates: []record.ImagePlateRow{
				{
					PlateID:      "p1",
					Timepoints:   []*string{new("D1")},
					PlannedSlots: 1,
					FilledSlots:  1,
					Wells: []record.ImageWellRow{
						{
							Coordinate:   "A1",
							GroupLabel:   new("Control"),
							PlannedSlots: 1,
							FilledSlots:  1,
							Slots: []record.ImageSlot{
								{Timepoint: new("D1"), FOVIndex: 1, AssetID: new("img-1")},
							},
						},
					},
				},
			},
			PlannedSlots: 1,
			FilledSlots:  1,
		},
	}
}

func TestHistoryRecordMemoizesByKey(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	proj := historyProjFixture()
	key := RecordKey("run-cck8|6|nofov|unblinded")
	s := NewState(vm, Callbacks{
		// The host memoizes the fetch; it may hand over a fresh copy
		// every frame. Pointer stability is the bench's job.
		FetchRecord: func() (RecordKey, record.Projection, error) {
			cp := proj
			return key, cp, nil
		},
	})
	first, tasks := s.historyRecord()
	if first == nil {
		t.Fatal("history record must project through FetchRecord")
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2 frozen tasks from the view model", len(tasks))
	}
	if second, _ := s.historyRecord(); second != first {
		t.Fatal("same key must return the memoized projection")
	}
	key = RecordKey("run-cck8|7|nofov|unblinded")
	if third, _ := s.historyRecord(); third == first {
		t.Fatal("new key must rebuild the projection")
	}
}

func TestHistoryRecordNilAndError(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{})
	if proj, _ := s.historyRecord(); proj != nil {
		t.Fatal("nil FetchRecord must hide the projection")
	}
	s.Callbacks.FetchRecord = func() (RecordKey, record.Projection, error) {
		return "", record.Projection{}, errors.New("core down")
	}
	if proj, _ := s.historyRecord(); proj != nil {
		t.Fatal("fetch error must hide the projection")
	}
	if s.bench().historyErr == "" {
		t.Fatal("fetch error must be remembered for the surface (UX-P5)")
	}
}

func TestInvalidateRecordRefetches(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	proj := historyProjFixture()
	s := NewState(vm, Callbacks{
		FetchRecord: func() (RecordKey, record.Projection, error) {
			cp := proj
			return RecordKey("k"), cp, nil
		},
	})
	first, _ := s.historyRecord()
	if first == nil {
		t.Fatal("history record must project through FetchRecord")
	}
	if second, _ := s.historyRecord(); second != first {
		t.Fatal("same key must stay memoized")
	}
	// Sign/archive flips persisted state without bumping the Run revision;
	// the host key cannot move, so invalidation must force the rebuild.
	s.InvalidateRecord()
	if third, _ := s.historyRecord(); third == first {
		t.Fatal("invalidate must rebuild the projection on next frame")
	}
}

func TestHistoryTabRendersSharedProjection(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	proj := historyProjFixture()
	s := NewState(vm, Callbacks{
		FetchRecord: func() (RecordKey, record.Projection, error) {
			return RecordKey("k"), proj, nil
		},
	})
	s.Selected = "t-read"
	task := vm.TaskByKey["t-read"]
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		historyTab(c, s, c.Theme(), theme.GetTokens(c.Theme()), task)
	}, 1000, 700)
	tt.Frame()
	for _, want := range []string{"计划 vs 实际", "采图对账", "细胞接种", "CCK-8 检测", "计划 0.82 → 实际 0.82"} {
		if !tt.HasText(want) {
			t.Fatalf("history missing %q: %q", want, tt.Texts())
		}
	}
}
