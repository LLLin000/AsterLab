package runworkspace

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

func buildTestState() *State {
	snap := makeSnapshot()
	doc := makeFrozenDoc()
	vm := BuildViewModel(snap, doc)
	return NewState(vm, Callbacks{})
}

func TestViewRendersRailAndPanel(t *testing.T) {
	s := buildTestState()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("实验流程") {
		t.Fatal("left rail title missing")
	}
	if !tt.HasText("实验记录") {
		t.Fatal("recording layer missing")
	}
	if !tt.HasText("执行确认") {
		t.Fatal("checks layer missing")
	}
	if !tt.HasText("历史") {
		t.Fatal("history tab missing")
	}
}

func TestProcedureShowsFrozenProtocolReadOnly(t *testing.T) {
	snap := makeSnapshot()
	snap.Tasks[1].ProtocolVersionID = "protocol-v2"
	vm := BuildViewModel(snap, makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	if task.ProtocolVersionID != "protocol-v2" || task.Instruction != "读取 OD450 并记录。" ||
		len(task.Parameters) != 1 || task.Parameters[0].Value != "0.82" {
		t.Fatalf("view model did not retain frozen snapshot values: %+v", task)
	}
	s := NewState(vm, Callbacks{})
	s.Selected = "t-read"
	render := func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}
	closed := ui.NewTester(render, 1360, 880)
	closed.Frame()
	for _, text := range []string{"操作说明 · 已冻结 · 只读", "读取 OD450 并记录。", "操作参数 · 已冻结 · 只读", "OD450", "0.82 AU", "技术属性（只读）"} {
		if !closed.HasText(text) {
			t.Errorf("frozen procedure detail missing %q", text)
		}
	}
	for _, id := range []string{"protocol-v2", "step-read", "root/read", "assay-cck8"} {
		if closed.HasText(id) {
			t.Errorf("raw identifier %q leaked into the default view", id)
		}
	}
	// One click on 技术属性 reveals the frozen identity.
	s.ShowTechnical = true
	opened := ui.NewTester(render, 1360, 880)
	opened.Frame()
	for _, want := range []string{"协议版本 protocol-v2", "步骤 ID step-read", "路径 root/read"} {
		if !opened.HasText(want) {
			t.Errorf("expanded technical properties missing %q: %v", want, opened.Texts())
		}
	}
}

func TestViewSelectsStepOnClick(t *testing.T) {
	s := buildTestState()
	var selectedKeys []string
	s.Callbacks.SelectTask = func(key string) { selectedKeys = append(selectedKeys, key) }
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1200, 760)
	tt.Frame()
	if s.Selected == "" {
		t.Fatal("no default selection")
	}
	if len(selectedKeys) != 0 {
		t.Fatalf("selection should not fire without a click: %v", selectedKeys)
	}
}

func TestViewNarrowStacks(t *testing.T) {
	s := buildTestState()
	s.Narrow = true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 900, 600)
	tt.Frame()
	// UX-P1, UX-LAYOUT: recording controls precede procedure and navigation at 900×600.
	text := strings.Join(tt.Texts(), "\n")
	recording := strings.Index(text, "实验记录")
	procedure := strings.Index(text, "操作说明与元数据")
	if recording < 0 || procedure < 0 || recording >= procedure {
		t.Fatalf("recording must precede procedure at 900×600: %v", tt.Texts())
	}
}

func TestFlowRailShowsHierarchyProgressTimepointAndCoreStatus(t *testing.T) {
	s := buildTestState()
	s.Selected = "task-done"
	s.VM.Tree = &workflowtree.Tree{Roots: []*workflowtree.Node{
		{
			ID: "group", Title: "Workflow group", Kind: workflowtree.KindStep,
			Composite: true, Completed: 1, Total: 4, Status: "in_progress",
			Children: []*workflowtree.Node{
				{ID: "task-new", Title: "New task", Kind: workflowtree.KindTask, Status: "not_started", Timepoint: "D0"},
				{ID: "task-active", Title: "Active task", Kind: workflowtree.KindTask, Status: "in_progress", Timepoint: "D1"},
				{ID: "task-done", Title: "Done task", Kind: workflowtree.KindTask, Status: "completed", Timepoint: "D2"},
				{ID: "task-blocked", Title: "Blocked task", Kind: workflowtree.KindTask, Status: "blocked", Timepoint: "D3"},
			},
		},
		{ID: "org", Title: "Organization", Kind: workflowtree.KindStep, Composite: true, Organizational: true, Completed: 99, Total: 99},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1200, 760)
	tt.Frame()
	text := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{
		"Workflow group", "1/4", "D0", "未开始", "进行中", "已完成", "阻塞",
		"Organization", "组织节点", "Done task",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("flow rail missing %q: %v", want, tt.Texts())
		}
	}
	if strings.Contains(text, "99/99") {
		t.Fatalf("organizational node must not show progress counts: %v", tt.Texts())
	}
}

func TestRecordingDirtyTracksEachTask(t *testing.T) {
	s := NewState(nil, Callbacks{})
	s.SetRecordingDirty("task-a", true)
	s.SetRecordingDirty("task-b", false)
	if !s.RecordingDirty["task-a"] || s.RecordingDirty["task-b"] {
		t.Fatalf("recording dirty states = %+v", s.RecordingDirty)
	}
	s.SetRecordingDirty("task-a", false)
	if s.RecordingDirty["task-a"] {
		t.Fatalf("saved task remains dirty: %+v", s.RecordingDirty)
	}
}


func TestCheckTogglesCallback(t *testing.T) {
	s := buildTestState()
	var saved []CheckAckView
	s.Callbacks.SaveChecks = func(taskKey string, checks []CheckAckView) { saved = checks }
	task := s.selectedTask()
	if task == nil || len(task.Checks) == 0 {
		t.Skip("no checks in fixture")
	}
	on := s.checkOn(task.TaskKey, task.Checks[0].CheckID)
	*on = true
	s.Callbacks.SaveChecks(task.TaskKey, s.acks(task))
	if len(saved) == 0 || saved[0].Status != "passed" {
		t.Fatalf("check ack not passed: %+v", saved)
	}
}

func TestOpenBlockingCountsUnconfirmed(t *testing.T) {
	s := buildTestState()
	task := s.selectedTask()
	if task == nil {
		t.Fatal("no task")
	}
	open := s.openBlocking(task)
	for _, c := range task.Checks {
		if c.Blocking && !*s.checkOn(task.TaskKey, c.CheckID) {
			found := false
			for _, o := range open {
				if o.CheckID == c.CheckID {
					found = true
				}
			}
			if !found {
				t.Fatalf("blocking check %s not reported open", c.CheckID)
			}
		}
	}
}
func TestEmptyCheckTaskOffersCompletion(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	task.Checks = nil
	s.Selected = task.TaskKey
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if !tt.HasText("标记完成") {
		t.Fatalf("empty-check task missing completion action: %v", tt.Texts())
	}
}

func TestTaskWithChecksDoesNotOfferMarkComplete(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	s.Selected = task.TaskKey
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if tt.HasText("标记完成") {
		t.Fatalf("task with checks showed completion action: %v", tt.Texts())
	}
}

func TestMarkCompleteCallsCallbackOnce(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	task.Checks = nil
	s.Selected = task.TaskKey
	calls := 0
	s.Callbacks.Complete = func(taskKey string, checks []CheckAckView) error {
		calls++
		if taskKey != "t-read" || len(checks) != 0 {
			t.Fatalf("Complete(%q, %v)", taskKey, checks)
		}
		return nil
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if err := tt.Click("标记完成"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if calls != 1 {
		t.Fatalf("Complete called %d times, want once", calls)
	}
}

func TestFailedCompletionShowsCauseAndRetry(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	task.Checks = nil
	s.Selected = task.TaskKey
	calls := 0
	s.Callbacks.Complete = func(string, []CheckAckView) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("Core rejected completion")
		}
		return nil
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if err := tt.Click("标记完成"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Core rejected completion") || !tt.HasText("重试完成") {
		t.Fatalf("failed completion missing cause/retry: %v", tt.Texts())
	}
	if err := tt.Click("重试完成"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if calls != 2 || tt.HasText("Core rejected completion") {
		t.Fatalf("retry did not re-invoke completion: calls=%d texts=%v", calls, tt.Texts())
	}
}

func TestTaskWithMeasurementPlanDoesNotOfferMarkComplete(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	task.Checks = nil
	task.Modules = append(task.Modules, ModuleRef{Kind: "measurement_profile"})
	s.Selected = task.TaskKey
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if tt.HasText("标记完成") {
		t.Fatalf("task with measurement plan showed completion action: %v", tt.Texts())
	}
}

func TestSyncFromKeepsSelection(t *testing.T) {
	s := buildTestState()
	first := s.Selected
	s.SyncFrom(BuildViewModel(makeSnapshot(), makeFrozenDoc()))
	if s.Selected != first {
		t.Fatalf("selection changed after sync: %q -> %q", first, s.Selected)
	}
}

func TestTreeExpansionRoundTrip(t *testing.T) {
	s := NewState(nil, Callbacks{})
	s.SetTreeCollapsed(map[string]bool{"step-rna-extract": true})
	if got := s.TreeCollapsed(); !got["step-rna-extract"] {
		t.Fatalf("collapsed state not returned: %+v", got)
	}
	open := s.treeOpen("step-rna-extract")
	if open == nil || *open {
		t.Fatalf("collapsed branch pointer = %v", open)
	}
	s.SetTreeCollapsed(map[string]bool{"step-rna-extract": false})
	if !*open {
		t.Fatal("expanded state not restored")
	}
}

func TestHistoryTabListsStoredEventAndOpensBatch(t *testing.T) {
	snap := makeSnapshot()
	snap.Events = []runs.RunEvent{{
		EventType: "measurement", Content: "OD recorded", OccurredAt: "2024-01-01",
		TaskKey: "t-read", ProtocolStepPath: "root/read", BatchID: "batch-1",
	}}
	vm := BuildViewModel(snap, makeFrozenDoc())
	opened := ""
	s := NewState(vm, Callbacks{OpenBatch: func(id string) { opened = id }})
	s.WorkspaceTab = 1
	s.Selected = "t-read"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("OD recorded") {
		t.Fatalf("stored event missing: %v", tt.Texts())
	}
	if !tt.HasText("打开原始孔板") {
		t.Fatalf("open batch action missing: %v", tt.Texts())
	}
	if err := tt.Click("打开原始孔板"); err != nil {
		t.Fatal(err)
	}
	if opened != "batch-1" {
		t.Fatalf("opened batch = %q, want batch-1", opened)
	}
}

// TestStackedRunViewStartsAtRecordingAndKeepsTableFirstRow pins UX-03: at
// 900x600 the inline flow rail must not reveal its selection by scrolling the
// shared page, which used to push the recording header and the table's first
// rows off screen and leak the offset into the table view.
func TestStackedRunViewStartsAtRecordingAndKeepsTableFirstRow(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	rows, cols := make([]string, 8), make([]string, 12)
	for i := range rows {
		rows[i] = string(rune('A' + i))
	}
	for i := range cols {
		cols[i] = strconv.Itoa(i + 1)
	}
	grid := resultgrid.New(resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Layout: "plate96",
		RowLabels: rows, ColumnLabels: cols,
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}, nil)
	kind := ViewPlate
	s := NewState(vm, Callbacks{Recording: func(c *ui.Context, task *RunTaskViewModel) {
		rv := &RecordingView{Kind: kind, Grid: grid, Task: task}
		rv.Render(c)
	}})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 900, 600)
	tt.Frame()
	header, ok := tt.Find("实验记录")
	if !ok || header.W == 0 || header.Y < 0 || header.Y >= 600 {
		t.Fatalf("recording header not in the initial viewport at 900x600: %+v ok=%v", header, ok)
	}
	kind = ViewTable
	tt.Frame()
	first, ok := tt.Find("A1")
	if !ok || first.W == 0 || first.Y < 0 || first.Y >= 600 {
		t.Fatalf("table first row A1 not in the viewport after switching views: %+v ok=%v", first, ok)
	}
	second, ok := tt.Find("A2")
	if !ok || second.Y <= first.Y {
		t.Fatalf("table rows lost their order after switching views: A1=%+v A2=%+v", first, second)
	}
}

// TestReadingFieldKeepsFocusAcrossBreakpointResize pins UX-03: the reading
// field the operator is typing in keeps keyboard focus when the window crosses
// the 1360/900 layout breakpoint, which rebuilds the whole recording subtree.
func TestReadingFieldKeepsFocusAcrossBreakpointResize(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	rows, cols := make([]string, 8), make([]string, 12)
	for i := range rows {
		rows[i] = string(rune('A' + i))
	}
	for i := range cols {
		cols[i] = strconv.Itoa(i + 1)
	}
	grid := resultgrid.New(resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Layout: "plate96",
		RowLabels: rows, ColumnLabels: cols,
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}, nil)
	s := NewState(vm, Callbacks{Recording: func(c *ui.Context, task *RunTaskViewModel) {
		rv := &RecordingView{Kind: ViewTable, Grid: grid, Task: task}
		rv.Render(c)
	}})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	if err := tt.Click("A1 读数"); err != nil {
		t.Fatalf("focus A1 reading: %v", err)
	}
	tt.Frame()
	if !tt.Focused("A1 读数") {
		t.Fatal("A1 reading did not take focus at 1360x880")
	}
	tt.SetSize(900, 600)
	tt.Frame()
	if !tt.Focused("A1 读数") {
		t.Fatal("A1 reading lost focus crossing to 900x600")
	}
	tt.SetSize(1360, 880)
	tt.Frame()
	if !tt.Focused("A1 读数") {
		t.Fatal("A1 reading lost focus crossing back to 1360x880")
	}
}

// TestSelectingStepGroupReportsTaskKey pins UX-03: a step group's synthetic
// step id is not a Core task key, so reporting it as the selection made the
// chosen task impossible to restore from the session. Selecting the group must
// report the real task key of its task.
func TestSelectingStepGroupReportsTaskKey(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	var selected string
	s := NewState(vm, Callbacks{SelectTask: func(key string) { selected = key }})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("CCK-8 检测"); err != nil {
		t.Fatal(err)
	}
	if selected != "t-read" {
		t.Fatalf("step group reported selection %q, want the Core task key t-read", selected)
	}
	if s.Selected != "t-read" {
		t.Fatalf("state selection = %q, want t-read", s.Selected)
	}
}

// TestWideLayoutPlacesRecordingBetweenFlowAndProperties enforces UX-LAYOUT at
// ≥1100: left flow chain, middle record projection, right compact properties.
func TestWideLayoutPlacesRecordingBetweenFlowAndProperties(t *testing.T) {
	s := buildTestState()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()
	flow, okFlow := tt.Find("实验流程")
	recording, okRecording := tt.Find("实验记录")
	properties, okProperties := tt.Find("操作说明 · 已冻结 · 只读")
	if !okFlow || !okRecording || !okProperties {
		t.Fatalf("column anchors missing: flow=%v recording=%v properties=%v (%v)",
			okFlow, okRecording, okProperties, tt.Texts())
	}
	if !(flow.X < recording.X && recording.X < properties.X) {
		t.Fatalf("expected flow < recording < properties, got x=%0.1f/%0.1f/%0.1f",
			flow.X, recording.X, properties.X)
	}
}

// TestDefaultViewHidesRawIdentifiers enforces UX-P2: the default run view
// never shows a raw step/block/protocol identifier at either breakpoint.
func TestDefaultViewHidesRawIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		width  int
		height int
		narrow bool
	}{
		{"three-column", 1360, 880, false},
		{"single-column", 900, 600, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := buildTestState()
			s.Narrow = tc.narrow
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.LightTheme())
				View(c, s)
			}, tc.width, tc.height)
			tt.Frame()
			text := strings.Join(tt.Texts(), "\n")
			for _, id := range []string{
				"step-seed", "step-read", "root/seed", "root/read",
				"assay-cck8", "t-seed", "t-read", "uv-reader", "dil-stock",
			} {
				if strings.Contains(text, id) {
					t.Errorf("%dx%d default view leaked raw identifier %q: %v",
						tc.width, tc.height, id, tt.Texts())
				}
			}
			for _, raw := range []string{"in_progress", "planned", "running"} {
				if strings.Contains(text, raw) {
					t.Errorf("%dx%d default view leaked raw status %q: %v",
						tc.width, tc.height, raw, tt.Texts())
				}
			}
		})
	}
}

// TestStatusLabelContractVocabulary enforces UX-STATE: every frozen status maps
// to the contract word, including the raw planned/in_progress spellings.
func TestStatusLabelContractVocabulary(t *testing.T) {
	cases := map[string]string{
		"planned":     "未开始",
		"in_progress": "进行中",
		"completed":   "已完成",
		"blocked":     "阻塞",
	}
	for raw, want := range cases {
		if got := treeStatusLabel(raw); got != want {
			t.Errorf("treeStatusLabel(%q) = %q, want %q", raw, got, want)
		}
		if got := StatusLabel(buildTaskStatus(runs.RunTask{Status: raw})); got != want {
			t.Errorf("StatusLabel(buildTaskStatus(%q)) = %q, want %q", raw, got, want)
		}
	}
}
