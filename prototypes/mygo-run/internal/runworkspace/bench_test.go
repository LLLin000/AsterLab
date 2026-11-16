package runworkspace

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

// benchHarness builds a selected task state with fake Core seams.
func benchHarness() (*State, **runs.ExecutionEventInput, *[]runs.TimerRecord) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	var appended *runs.ExecutionEventInput
	var saved []runs.TimerRecord
	s := NewState(vm, Callbacks{
		LoadTimers: func(runID string) ([]runs.TimerRecord, error) { return saved, nil },
		SaveTimer: func(timer runs.TimerRecord) (runs.TimerRecord, error) {
			saved = append(saved, timer)
			return timer, nil
		},
		TimerAction: func(input runs.TimerActionInput) (runs.TimerUpdateResult, error) {
			return runs.TimerUpdateResult{Timer: input.Timer}, nil
		},
		AppendEvent: func(taskKey string, input runs.ExecutionEventInput) (string, error) {
			appended = &input
			return "", nil
		},
		LoadPantry: func() (runs.PantryLedger, error) { return runs.PantryLedger{}, nil },
		TaskAssets: func(taskKey string) []LinkedAsset {
			return []LinkedAsset{
				{AssetID: "a1", WellID: "A1", Timepoint: "D1", FOVIndex: 1},
				{AssetID: "a2", WellID: "A1", Timepoint: "D1", FOVIndex: 2},
			}
		},
		CaptureWells: func(taskKey string) ([]string, int) {
			return []string{"A1", "A2"}, 2
		},
		RequestBatchLink: func(taskKey string) {},
	})
	s.Selected = "t-read"
	return s, &appended, &saved
}

func renderBenchAt(t *testing.T, s *State, width, height int) *ui.Tester {
	t.Helper()
	task := s.VM.TaskByKey["t-read"]
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		renderBench(c, s, task, c.Theme(), theme.GetTokens(c.Theme()))
	}, width, height)
	tt.Frame()
	return tt
}

func TestBenchRendersAtNarrowWidth(t *testing.T) {
	s, _, _ := benchHarness()
	tt := renderBenchAt(t, s, 900, 600)
	for _, want := range []string{"执行台面", "计时", "偏差与跳过", "采图进度", "A1 2/2", "A2 0/2", "按规则跳过", "记偏差", "记观察", "批量关联"} {
		if !tt.HasText(want) {
			t.Fatalf("bench at 900x600 missing %q: %q", want, tt.Texts())
		}
	}
}

func TestBenchSkipFlowAppendsEvent(t *testing.T) {
	s, appended, _ := benchHarness()
	tt := renderBenchAt(t, s, 1200, 800)
	if err := tt.Click("按规则跳过"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("确认按规则跳过"); err != nil {
		t.Fatal(err)
	}
	if *appended == nil {
		t.Fatal("no event appended")
	}
	event := **appended
	if event.EventType != "skipped_by_rule" {
		t.Fatalf("event type = %q", event.EventType)
	}
	if event.StepPath != "root/read" || event.AssayID != "assay-cck8" {
		t.Fatalf("scope = %+v", event)
	}
	if !tt.HasText("已追加到 Run 记录") {
		t.Fatalf("success notice missing: %q", tt.Texts())
	}
}

func TestBenchBlindToggleMasksCapture(t *testing.T) {
	s, _, _ := benchHarness()
	tt := renderBenchAt(t, s, 1200, 800)
	if err := tt.Click("○ 设盲视图"); err != nil {
		t.Fatal(err)
	}
	if s.Bench == nil || !s.Bench.Blind {
		t.Fatal("blind toggle did not flip")
	}
	if !tt.HasText("分组已隐藏") {
		t.Fatalf("blind notice missing: %q", tt.Texts())
	}
}
