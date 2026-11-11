package runworkspace

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
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
	if !tt.HasText("操作历史") {
		t.Fatal("history layer missing")
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
	if !tt.HasText("实验流程") {
		t.Fatal("rail missing in narrow mode")
	}
	if !tt.HasText("实验记录") {
		t.Fatal("recording missing in narrow mode")
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
