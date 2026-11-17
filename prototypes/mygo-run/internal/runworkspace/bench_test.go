package runworkspace

import (
	"errors"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

// errTestThrottle stands in for a persistent Core failure.
var errTestThrottle = errors.New("Core 不可用")

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

// TestBenchHasNoBlindSwitch keeps one blind entry: the Run top-bar switch in
// runHeader consumes Core's run.workspace blind projection, so the bench
// offers no second switch. Toggling and masking stay covered by the
// top-bar tests in blind_test.go. UX-IA-1, UX-STATE.
func TestBenchHasNoBlindSwitch(t *testing.T) {
	s, _, _ := benchHarness()
	tt := renderBenchAt(t, s, 1200, 800)
	if tt.HasText("设盲视图") {
		t.Fatalf("bench must not offer a blind switch: %q", tt.Texts())
	}
}

// TestBenchSampleSectionsFollowTaskCapability renders exclusion and typed
// readout entry only for sample-bearing tasks (measurement plan, recorded
// measurements, or capture slots); procedure-only steps keep the generic
// skip/deviation/observation composer. UX-IA-1, UX-P1.
func TestBenchSampleSectionsFollowTaskCapability(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{
		AppendEvent: func(string, runs.ExecutionEventInput) (string, error) { return "", nil },
		ExecuteCommand: func(runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error) {
			return runs.ExecuteCommandResult{}, nil
		},
		CaptureWells: func(taskKey string) ([]string, int) { return nil, 0 },
	})
	renderAt := func(task *RunTaskViewModel) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			renderBench(c, s, task, c.Theme(), theme.GetTokens(c.Theme()))
		}, 1200, 800)
		tt.Frame()
		return tt
	}
	// t-read carries recorded measurements: sample sections render.
	s.Selected = "t-read"
	read := renderAt(vm.TaskByKey["t-read"])
	for _, want := range []string{"样本排除", "分类与表格读数", "偏差与跳过"} {
		if !read.HasText(want) {
			t.Fatalf("sample task bench missing %q: %q", want, read.Texts())
		}
	}
	// A procedure-only step carries no samples: generic composer only.
	pure := &RunTaskViewModel{TaskKey: "t-pure", StepTitle: "离心", Status: TaskStatusCurrent}
	got := renderAt(pure)
	if !got.HasText("偏差与跳过") {
		t.Fatalf("procedure task bench missing composer: %q", got.Texts())
	}
	for _, hidden := range []string{"样本排除", "分类与表格读数"} {
		if got.HasText(hidden) {
			t.Fatalf("procedure task bench must not show %q: %q", hidden, got.Texts())
		}
	}
}

func TestTaskCarriesSamples(t *testing.T) {
	if taskCarriesSamples(nil, nil) {
		t.Fatal("nil task carries no samples")
	}
	plain := &RunTaskViewModel{TaskKey: "t-pure", Status: TaskStatusCurrent}
	if taskCarriesSamples(plain, nil) {
		t.Fatal("procedure task without slots carries no samples")
	}
	if !taskCarriesSamples(plain, []string{"A1"}) {
		t.Fatal("capture slots mean samples")
	}
	measured := &RunTaskViewModel{TaskKey: "t-read", Status: TaskStatusCurrent,
		Actual: &ActualRecord{Measurements: []MeasurementView{{SampleID: "M1"}}}}
	if !taskCarriesSamples(measured, nil) {
		t.Fatal("recorded measurements mean samples")
	}
	planned := &RunTaskViewModel{TaskKey: "t-plan", Status: TaskStatusCurrent,
		Modules: []ModuleRef{{Kind: "measurement_profile"}}}
	if !taskCarriesSamples(planned, nil) {
		t.Fatal("measurement plan means samples")
	}
}

func TestRefreshTimersBacksOffFailingLoads(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	calls := 0
	s := NewState(vm, Callbacks{
		LoadTimers: func(runID string) ([]runs.TimerRecord, error) {
			calls++
			return nil, errTestThrottle
		},
	})
	// A persistent Core failure must not retry ListTimers on every frame:
	// the first attempt records the failure, immediate repeats back off.
	if !s.RefreshTimers(false, time.Minute) {
		t.Fatal("first load must run")
	}
	if s.RefreshTimers(false, time.Minute) {
		t.Fatal("immediate repeat must back off")
	}
	if s.RefreshTimers(false, time.Minute) {
		t.Fatal("immediate repeat must keep backing off")
	}
	if calls != 1 {
		t.Fatalf("failing LoadTimers ran %d times, want 1", calls)
	}
	if msg := s.bench().TimersErr[s.Selected]; msg == "" {
		t.Fatal("failure cause must stay visible")
	}
	// Forcing retries; a zero maxAge disables time-based refresh.
	if !s.RefreshTimers(true, time.Minute) {
		t.Fatal("forced load must run")
	}
	if s.RefreshTimers(false, 0) {
		t.Fatal("zero maxAge must not time-refresh")
	}
	if calls != 2 {
		t.Fatalf("failing LoadTimers ran %d times, want 2", calls)
	}
}

func TestTreeSkippedStatusReadsWarning(t *testing.T) {
	tokens := theme.LightTokens()
	th := theme.LightTheme()
	if got := treeStatusLabel("skipped"); got != "已跳过" {
		t.Fatalf("tree skipped label = %q", got)
	}
	if got := treeStatusLabel("skipped_by_rule"); got != "已跳过" {
		t.Fatalf("tree rule-skip label = %q", got)
	}
	if got := treeStatusColor("skipped", th, tokens); got != th.Warning {
		t.Fatalf("tree skipped color not warning: %v", got)
	}
	if got := StatusLabel(TaskStatusSkipped); got != "已跳过" {
		t.Fatalf("pill skipped label = %q", got)
	}
}
