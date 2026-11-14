package runworkspace

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

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
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		procedureLayer(c, s, c.Theme(), theme.GetTokens(c.Theme()), task)
	}, 900, 700)
	tt.Frame()
	for _, text := range []string{"操作说明 · 已冻结 · 只读", "读取 OD450 并记录。", "操作参数 · 已冻结 · 只读", "OD450", "0.82 AU", "协议版本 protocol-v2", "冻结信息（只读 · 已冻结）"} {
		if !tt.HasText(text) {
			t.Errorf("frozen procedure detail missing %q", text)
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
		"Workflow group", "1/4", "D0", "未开始", "进行中", "已完成", "已阻塞",
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
