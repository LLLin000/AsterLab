package runworkspace

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestBuildTimelineChronologicalAndStable(t *testing.T) {
	vm := &RunWorkspaceViewModel{
		UnassociatedEvents: []EventViewModel{
			{ID: "later", RecordedAt: "2024-01-02T00:00:00Z", Sequence: 4},
			{ID: "tie-first", OccurredAt: "2024-01-01T00:00:00Z", Sequence: 1},
			{ID: "tie-second", RecordedAt: "2024-01-01T00:00:00Z", Sequence: 2},
			{ID: "empty-first", Sequence: 0},
			{ID: "empty-second", Sequence: 3},
		},
	}
	got := BuildTimeline(vm)
	ids := make([]string, len(got))
	for i := range got {
		ids[i] = got[i].ID
	}
	want := []string{"tie-first", "tie-second", "later", "empty-first", "empty-second"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("timeline order = %v, want %v", ids, want)
	}
}

func timelineTestState() *State {
	task := &RunTaskViewModel{TaskKey: "task-1", StepTitle: "培养步骤", Status: TaskStatusPending}
	vm := &RunWorkspaceViewModel{
		TaskByKey: map[string]*RunTaskViewModel{"task-1": task},
		Steps: []*RunTaskViewModel{task},
	}
	task.Events = []EventViewModel{
		{ID: "check-failed", Type: "check", Content: "首次检查失败", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:00:00Z", Sequence: 0, Checks: []CheckAckView{{CheckID: "qc", Status: "failed"}}},
		{ID: "check-retry", Type: "check", Content: "复核通过", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:01:00Z", Sequence: 1, Checks: []CheckAckView{{CheckID: "qc", Status: "passed"}}},
		{ID: "check-blocked", Type: "check", Content: "另一项被阻塞", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:02:00Z", Sequence: 2, Checks: []CheckAckView{{CheckID: "sterility", Status: "blocked"}}},
		{ID: "check-skipped", Type: "check", Content: "可选检查跳过", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:03:00Z", Sequence: 3, Checks: []CheckAckView{{CheckID: "optional", Status: "skipped"}}},
		{ID: "complete", Type: "operation", Content: "步骤结束", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:04:00Z", Sequence: 4, StepStatus: "completed"},
		{ID: "measurement", Type: "measurement", Content: "记录读数", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:05:00Z", Sequence: 5, Measurements: []MeasurementView{{SampleID: "sample", Field: "OD", Value: "0.8", Unit: "AU"}}},
		{ID: "deviation", Type: "deviation", Content: "记录偏差", TaskKey: task.TaskKey,
			RecordedAt: "2024-01-01T00:06:00Z", Sequence: 6, Deviation: "温度偏高"},
	}
	return NewState(vm, Callbacks{})
}

func TestBuildTimelineUsesEventOutcomesAndRetry(t *testing.T) {
	events := BuildTimeline(timelineTestState().VM)
	want := map[string]string{
		"check-failed": "失败", "check-retry": "重试", "check-blocked": "失败",
		"check-skipped": "", "complete": "已完成", "measurement": "测量", "deviation": "偏差",
	}
	for _, event := range events {
		if event.Badge != want[event.ID] {
			t.Errorf("event %q badge = %q, want %q", event.ID, event.Badge, want[event.ID])
		}
		if event.ID == "check-retry" && (!event.Retry || event.Attempt != 2) {
			t.Errorf("retry payload = retry %v, attempt %d; want true, 2", event.Retry, event.Attempt)
		}
	}
}
func TestTimelineTabRendersCoreFactsAtSupportedLayouts(t *testing.T) {
	for _, size := range [][2]int{{1360, 880}, {900, 600}} {
		for _, dark := range []bool{false, true} {
			s := timelineTestState()
			s.Selected = "task-1"
			s.WorkspaceTab = 2
			themeName := "light"
			themeForTest := theme.LightTheme()
			if dark {
				themeName = "dark"
				themeForTest = theme.DarkTheme()
			}
			t.Run(fmt.Sprintf("%s-%dx%d", themeName, size[0], size[1]), func(t *testing.T) {
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(themeForTest)
					View(c, s)
				}, size[0], size[1])
				tt.Frame()
				for _, text := range []string{"时间线", "已确认", "失败", "重试", "已完成", "测量", "偏差",
					"首次检查失败", "复核通过", "另一项被阻塞", "可选检查跳过", "步骤结束", "记录读数", "0.8"} {
					if !tt.HasText(text) {
						t.Errorf("%dx%d %s theme missing %q", size[0], size[1], themeName, text)
					}
				}
			})
		}
	}
}
