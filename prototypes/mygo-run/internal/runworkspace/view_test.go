package runworkspace

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/asterdoc"
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
	s.Callbacks.SaveChecks = func(taskKey string, checks []CheckAckView) error {
		saved = checks
		return nil
	}
	task := s.selectedTask()
	if task == nil || len(task.Checks) == 0 {
		t.Skip("no checks in fixture")
	}
	on := s.checkOn(task.TaskKey, task.Checks[0].CheckID)
	*on = true
	_ = s.Callbacks.SaveChecks(task.TaskKey, s.acks(task))
	if len(saved) == 0 || saved[0].Status != "passed" {
		t.Fatalf("check ack not passed: %+v", saved)
	}
}

func TestSaveChecksFailureShowsErrorRevertsCheckboxAndRetries(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	s.Selected = task.TaskKey
	calls := 0
	var lastAcks []CheckAckView
	s.Callbacks.SaveChecks = func(taskKey string, checks []CheckAckView) error {
		calls++
		lastAcks = append([]CheckAckView(nil), checks...)
		if calls == 1 {
			return fmt.Errorf("CONFLICT: checks write rejected")
		}
		return nil
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()

	if err := tt.Click("板与密度已确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if calls != 1 {
		t.Fatalf("SaveChecks calls = %d, want 1", calls)
	}
	if *s.checkOn(task.TaskKey, "c-plate") {
		t.Fatal("checkbox stayed checked after SaveChecks failure; expected revert to false")
	}
	if !tt.HasText("未保存 · CONFLICT: checks write rejected") || !tt.HasText("重试") {
		t.Fatalf("failed SaveChecks missing error or retry: %v", tt.Texts())
	}

	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if calls != 2 {
		t.Fatalf("SaveChecks calls after retry = %d, want 2", calls)
	}
	if len(lastAcks) == 0 || lastAcks[0].Status != "passed" {
		t.Fatalf("retry did not resend passed check ack: %+v", lastAcks)
	}
	if !*s.checkOn(task.TaskKey, "c-plate") {
		t.Fatal("checkbox did not reflect retried passed state after retry succeeded")
	}
	if tt.HasText("CONFLICT: checks write rejected") {
		t.Fatalf("error stayed visible after retry succeeded: %v", tt.Texts())
	}
}

func TestRunLoadErrorShowsReasonRetryAndNeverShowsLoading(t *testing.T) {
	vm := &RunWorkspaceViewModel{Title: "加载中", Status: "loading"}
	retries := 0
	var s *State
	s = NewState(vm, Callbacks{
		RetryLoad: func() {
			retries++
			s.SyncFrom(BuildViewModel(makeSnapshot(), makeFrozenDoc()))
		},
	})
	s.VM.Status = "error"
	s.LoadError = "NOT_FOUND: run missing"

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}, 1360, 880)
	tt.Frame()

	if tt.HasText("正在加载实验运行数据…") {
		t.Fatalf("failed load rendered loading placeholder: %v", tt.Texts())
	}
	if tt.HasText("此运行没有任务") {
		t.Fatalf("failed load rendered empty-tasks placeholder: %v", tt.Texts())
	}
	if !tt.HasText("NOT_FOUND: run missing") || !tt.HasText("重试") {
		t.Fatalf("failed load missing error reason or retry: %v", tt.Texts())
	}

	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if retries != 1 {
		t.Fatalf("RetryLoad called %d times, want 1", retries)
	}
	if tt.HasText("NOT_FOUND: run missing") || !tt.HasText("实验流程") {
		t.Fatalf("retry did not restore loaded run workspace: %v", tt.Texts())
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

func TestViewRendersFrozenStepBlocksSameSourceModeRun(t *testing.T) {
	src := strings.Join([]string{
		"```asterlab", "kind: experiment", "id: exp-iz", "schema: 1", "title: 抑菌圈定量实验", "```",
		"```asterlab", "kind: group", "id: g-van", "schema: 1", "name: Van", "role: reference", "```",
		"```asterlab", "kind: assay", "id: assay-iz", "schema: 1", "title: 抑菌圈测量", "group_refs: [g-van]", "timepoints: [24h]", "```",
		"```asterlab", "kind: plate", "id: dish-iz", "schema: 1", "format: 6", "topology: dish", "rows: 2", "columns: 2", "assay: assay-iz", "groups:", "  g-van: [A1]", "```",
		"```asterlab", "kind: step", "id: step-treat", "schema: 1", "title: 抑菌圈加药与孵育",
		"execution: executable", "assay: assay-iz", "path: root/treat",
		"blocks:",
		"  - kind: param_table",
		"    id: blk-params",
		"    schema: 1",
		"    title: 抑菌关键参数",
		"    rows:",
		"      - {key: dose, label: 给药体积, value: 10, unit: uL, editable_in_run: true}",
		"  - kind: figure",
		"    id: blk-dish",
		"    schema: 1",
		"    title: 四象限抑菌圈示意图",
		"    caption: 数据归属 group_table，示意图只作可视化说明",
		"    svg: |",
		"      <svg viewBox=\"0 0 120 120\"><circle cx=\"60\" cy=\"60\" r=\"50\" fill=\"none\" stroke=\"currentColor\"/><text x=\"60\" y=\"64\">Van</text></svg>",
		"  - kind: checklist",
		"    id: blk-chk",
		"    schema: 1",
		"    title: 操作确认",
		"    items:",
		"      - {id: chk-1, label: 确认牛津杯贴壁无渗漏, blocking: true}",
		"  - kind: attachment",
		"    id: blk-img",
		"    schema: 1",
		"    title: 培养皿拍照留存",
		"    scope: container_grid",
		"    container_ref: dish-iz",
		"    slots_per_cell: 4",
		"```",
	}, "\n")
	snap := runs.RunSnapshot{
		RunID: "run-iz-1", Title: "抑菌圈定量实验", Revision: 2, Status: "active",
		Tasks: []runs.RunTask{{
			Key: "t-treat", DesignStepID: "step-treat", Path: "root/treat",
			Title: "抑菌圈加药与孵育", AssayID: "assay-iz", Timepoint: "24h", Status: "in_progress",
		}},
	}
	doc := &asterdoc.FrozenAsterDoc{DesignID: "d-iz", Revision: 3, Source: src}
	vm := BuildViewModel(snap, doc)
	if len(vm.TaskByKey["t-treat"].Modules) != 4 {
		t.Fatalf("modules count = %d, want 4", len(vm.TaskByKey["t-treat"].Modules))
	}
	if got := FOVPerWellForDoc(src); got != 4 {
		t.Fatalf("FOVPerWellForDoc = %d, want 4", got)
	}
	st := NewState(vm, Callbacks{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, st)
	}, 1280, 820)
	tt.Frame()
	for _, want := range []string{"步骤模块 · 同源执行", "4 个内容块", "抑菌关键参数", "四象限抑菌圈示意图", "操作确认"} {
		if !tt.HasText(want) {
			t.Fatalf("ModeRun workbench missing %q in %v", want, tt.Texts())
		}
	}
}

func TestModeRunHydratesActualsSubmitsTableAndFeedsCalculator(t *testing.T) {
	src := strings.Join([]string{
		"```asterlab", "kind: experiment", "id: exp-norm", "schema: 1", "title: RNA 定量与归一化", "```",
		"```asterlab", "kind: group", "id: g-ctrl", "schema: 1", "name: Control", "role: control", "```",
		"```asterlab", "kind: assay", "id: assay-rna", "schema: 1", "title: Nanodrop + cDNA", "group_refs: [g-ctrl]", "timepoints: [0h]", "```",
		"```asterlab", "kind: step", "id: step-norm", "schema: 1", "title: 实测归一化加样与记录",
		"execution: executable", "assay: assay-rna", "path: root/norm",
		"blocks:",
		"  - kind: param_table",
		"    id: pt-norm",
		"    schema: 1",
		"    title: 实测参数",
		"    rows:",
		"      - {key: stock_conc, label: 母液浓度, value: 10, unit: mg/mL, record_actual: true}",
		"  - kind: calculator",
		"    id: calc-norm",
		"    schema: 1",
		"    title: 按实测样本稀释归一化",
		"    mode: sample_normalize",
		"    upstream_endpoint: rna_conc",
		"    target_mass: {value: 500, unit: ng}",
		"    volume_per_unit: {value: 10, unit: uL}",
		"  - kind: data_table",
		"    id: dt-norm",
		"    schema: 1",
		"    title: 实测记录表",
		"    schema_id: custom_norm_table",
		"    endpoint: cfu",
		"    columns:",
		"      - {key: sample_id, label: 样本, value_kind: text}",
		"      - {key: rna_conc, label: 浓度, value_kind: number, unit: ng/uL}",
		"```",
	}, "\n")
	evPayload := map[string]any{
		"actual_parameters": []map[string]any{
			{"key": "stock_conc", "value": 25.0, "unit": "mg/mL"},
		},
		"result_cells": []map[string]any{{
			"kind": "record",
			"readout": map[string]any{
				"schema_id":      "custom_norm_table",
				"schema_version": 1,
				"value": map[string]any{
					"kind":    "table",
					"columns": []string{"sample_id", "rna_conc"},
					"rows":    [][]any{{"S-Keep", 250.0}, {"S-Drop", 200.0}},
				},
			},
		}},
	}
	evBytes, _ := json.Marshal(evPayload)
	dropBytes, _ := json.Marshal(map[string]any{"sample_ids": []string{"S-Drop"}})
	snap := runs.RunSnapshot{
		RunID: "run-norm-1", Title: "RNA 定量与归一化", Revision: 2, Status: "active",
		Tasks: []runs.RunTask{{
			Key: "t-norm", DesignStepID: "step-norm", Path: "root/norm", ProtocolVersionID: "pv-1",
			Title: "实测归一化加样与记录", AssayID: "assay-rna", Timepoint: "0h", Status: "in_progress",
		}},
		Events: []runs.RunEvent{
			{ID: "ev-1", EventType: "measurement", TaskKey: "t-norm", ProtocolStepPath: "root/norm", DataJSON: string(evBytes)},
			{ID: "ev-2", EventType: "operation", TaskKey: "t-norm", ProtocolStepPath: "root/norm", StepStatus: "qc_dropped", DataJSON: string(dropBytes)},
		},
	}
	doc := &asterdoc.FrozenAsterDoc{DesignID: "d-norm", Revision: 1, Source: src}
	vm := BuildViewModel(snap, doc)
	if vm.ActualParamNumbers["stock_conc"] != 25.0 {
		t.Fatalf("ActualParamNumbers = %+v", vm.ActualParamNumbers)
	}
	if vm.SampleStatuses["S-Drop"] != "QC剔除" {
		t.Fatalf("SampleStatuses = %+v", vm.SampleStatuses)
	}

	var submitted runs.ExecuteCommandInput
	calls := 0
	st := NewState(vm, Callbacks{
		ExecuteCommand: func(input runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error) {
			calls++
			submitted = input
			return runs.ExecuteCommandResult{ResultJSON: `{"id":"ev-new"}`}, nil
		},
	})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, st)
	}, 1280, 900)
	tt.Frame()
	if !tt.HasText("已代入实测 · S-Keep=250 ng/uL") {
		t.Fatalf("expected live non-excluded upstream sample in calculator badge, got %v", tt.Texts())
	}
	if tt.HasText("S-Drop=200") {
		t.Fatalf("QC-dropped sample S-Drop leaked into calculator actuals: %v", tt.Texts())
	}
	if err := tt.Click("提交表格读数"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if calls != 1 || submitted.Kind != "readout.submit" {
		t.Fatalf("calls=%d submitted=%+v", calls, submitted)
	}
	if !tt.HasText("已保存至 Core") {
		t.Fatalf("expected save confirmation badge, got %v", tt.Texts())
	}
}
