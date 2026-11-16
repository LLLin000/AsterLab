package runui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func sampleView() RunView {
	return RunView{
		Title:    "L929",
		Status:   "进行中",
		Revision: 3,
		Tasks: []Task{
			{Key: "a", Title: "Seed", Timepoint: "D1", Status: "completed"},
			{Key: "b", Title: "CCK-8", Timepoint: "D3", Status: "not_started"},
		},
		Measurements: []Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}},
		Events:       []Event{{Kind: "save", Message: "ok"}},
	}
}

func TestSaveCallback(t *testing.T) {
	v := sampleView()
	var got []Measurement
	w := NewWorkspace(v, Callbacks{SaveReadout: func(ms []Measurement) { got = ms }})
	w.Draft.Rows = []DraftRow{{SampleID: "A1", Value: "1.2"}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
	tt.Frame()
	// invoke save directly
	w.save()
	if len(got) != 1 || got[0].SampleID != "A1" {
		t.Fatalf("save callback not invoked: %+v", got)
	}
}

func TestErrorDraftRetention(t *testing.T) {
	v := sampleView()
	w := NewWorkspace(v, Callbacks{})
	w.Draft.Rows = []DraftRow{{SampleID: "", Value: "1.2"}}
	w.save()
	if w.State != StateValidationError {
		t.Fatalf("state = %s", w.State)
	}
	if len(w.Draft.Rows) == 0 || w.Draft.Rows[0].Value != "1.2" {
		t.Fatalf("draft not retained: %+v", w.Draft)
	}
}

func TestTaskCounts(t *testing.T) {
	v := sampleView()
	w := NewWorkspace(v, Callbacks{})
	if d, tot := w.taskCounts(); d != 1 || tot != 2 {
		t.Fatalf("counts = %d/%d", d, tot)
	}
}

func TestNarrowLayout(t *testing.T) {
	v := sampleView()
	w := NewWorkspace(v, Callbacks{})
	w.Width = 900
	w.Narrow = true
	tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 900, 600)
	tt.Frame()
	_ = tt.Image()
}

func TestScreenshot(t *testing.T) {
	v := sampleView()
	w := NewWorkspace(v, Callbacks{})
	w.State = StateSaved
	tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("runui.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestTaskAssayID(t *testing.T) {
	v := RunView{
		Title: "Assay Test",
		Tasks: []Task{
			{Key: "t1", Title: "Task 1", Timepoint: "D1", Status: "not_started", AssayID: "assay-99"},
			{Key: "t2", Title: "Task 2", Timepoint: "D2", Status: "not_started", AssayID: "assay-100"},
		},
	}
	w := NewWorkspace(v, Callbacks{})
	ct := w.currentTask()
	if ct == nil {
		t.Fatal("expected currentTask non-nil")
	}
	if ct.AssayID != "assay-99" {
		t.Fatalf("expected AssayID assay-99, got %q", ct.AssayID)
	}

	w.Selected = "t2"
	ct2 := w.currentTask()
	if ct2 == nil || ct2.AssayID != "assay-100" {
		t.Fatalf("expected AssayID assay-100 after selection, got %+v", ct2)
	}
}
func TestTaskPanelShowsFrozenProtocolReadOnly(t *testing.T) {
	v := sampleView()
	v.DesignRevision, v.AsterDocRevision = 7, 4
	v.Tasks[1].ProtocolVersionID = "protocol-v2"
	v.Tasks[1].Instruction = "保持 4°C 并轻柔混匀"
	v.Tasks[1].Parameters = []FrozenParameter{{Key: "temp", Label: "温度", Value: "4", Unit: "°C"}}
	w := NewWorkspace(v, Callbacks{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		taskPanel(c, w, c.Theme(), theme.GetTokens(c.Theme()))
	}, 700, 700)
	tt.Frame()
	for _, text := range []string{"协议与参数 · 只读 · 已冻结", "协议版本 protocol-v2", "Run rev 3 · Design rev 7 · AsterDoc rev 4", "保持 4°C 并轻柔混匀", "温度 · 4 °C"} {
		if !tt.HasText(text) {
			t.Errorf("frozen task detail missing %q", text)
		}
	}
}

func TestRunShortcuts(t *testing.T) {
	v := sampleView()
	saved := false
	w := NewWorkspace(v, Callbacks{
		SaveReadout: func(ms []Measurement) { saved = true },
	})
	w.Draft.Rows = []DraftRow{{SampleID: "Sample-1", Value: "0.55"}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
	tt.Frame()

	// Cmd+N adds a draft row
	initialRows := len(w.Draft.Rows)
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if len(w.Draft.Rows) != initialRows+1 {
		t.Fatalf("expected %d rows after Cmd+N, got %d", initialRows+1, len(w.Draft.Rows))
	}

	// Cmd+S triggers save
	tt.Key(ui.Cmd, ui.KeyS)
	tt.Frame()
	if !saved {
		t.Fatal("expected save invoked via Cmd+S")
	}
}

func TestScalingAndThemesEvidence(t *testing.T) {
	saveImg := func(name string, img *ui.Tester) {
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img.Image()); err != nil {
			t.Fatal(err)
		}
	}

	v := sampleView()
	// 1. Light and Dark themes at 1200x760
	for _, dark := range []bool{false, true} {
		w := NewWorkspace(v, Callbacks{})
		tt := ui.NewTesterWith(func(c *ui.Context) { View(c, w) }, 1200, 760, ui.TesterOptions{Dark: dark})
		tt.Frame()
		mode := "light"
		if dark {
			mode = "dark"
		}
		saveImg("evidence-runui-1200x760-"+mode+".png", tt)
	}

	// 2. Light and Dark themes at 900x600 (compact stacked)
	for _, dark := range []bool{false, true} {
		w := NewWorkspace(v, Callbacks{})
		w.Width = 900
		tt := ui.NewTesterWith(func(c *ui.Context) { View(c, w) }, 900, 600, ui.TesterOptions{Dark: dark})
		tt.Frame()
		mode := "light"
		if dark {
			mode = "dark"
		}
		saveImg("evidence-runui-900x600-"+mode+".png", tt)
	}
	// 2.5 User dimensions: 1186x753
	{
		w := NewWorkspace(v, Callbacks{})
		tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1186, 753)
		tt.Frame()
		saveImg("evidence-runui-1186x753.png", tt)
	}

	// 3. Scaling at 150% and 200%
	for _, sc := range []float32{1.5, 2.0} {
		w := NewWorkspace(v, Callbacks{})
		tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
		tt.SetScale(sc)
		tt.Frame()
		scaleName := "150"
		if sc == 2.0 {
			scaleName = "200"
		}
		saveImg("evidence-runui-1200x760-scale"+scaleName+".png", tt)
	}

	// 4. Error State with preserved draft
	{
		w := NewWorkspace(v, Callbacks{})
		w.Draft.Rows = []DraftRow{{SampleID: "Invalid-Row", Value: "not-a-number"}}
		w.save()
		tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
		tt.Frame()
		saveImg("evidence-runui-error.png", tt)
	}

	// 5. Saved State
	{
		w := NewWorkspace(v, Callbacks{})
		w.State = StateSaved
		tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
		tt.Frame()
		saveImg("evidence-runui-saved.png", tt)
	}
}

// TestReviewViewSummarizesRecordedCells drives 汇总: the same grid becomes a
// read-only per-sample derivation labelled as technical replicates.
func TestReviewViewSummarizesRecordedCells(t *testing.T) {
	w := NewWorkspace(sampleView(), Callbacks{})
	v1, v2 := 0.8, 1.0
	schema := resultgrid.Schema{
		ID: "cck8", Version: 2, RowLabels: []string{"S1"}, ColumnLabels: []string{"OD450"},
		Fields: []resultgrid.Field{{Key: "OD450", Label: "OD450", Unit: "AU"}},
	}
	w.Grid = resultgrid.New(schema, nil)
	// Two technical replicates for one sample share the grid's single row slot.
	w.Grid.Cells = []resultgrid.Cell{
		{Row: 0, Column: 0, SampleID: "S1", Group: "Control", Field: "OD450", Value: &v1, Status: "raw"},
		{Row: 0, Column: 0, SampleID: "S1", Group: "Control", Field: "OD450", Value: &v2, Status: "raw"},
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, w) }, 1200, 760)
	tt.Frame()
	if err := tt.Click("汇总"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, text := range []string{"结果审阅", "技术重复", "生物学重复 n 不由孔位或行数推导"} {
		if !tt.HasText(text) {
			t.Fatalf("review view missing %q: %q", text, tt.Texts())
		}
	}
	if !w.ReviewView {
		t.Fatal("summary tab did not switch the view")
	}
	f, err := os.Create("evidence-review-summary.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestNotebookViewRendersStoredTimeline drives the Run/Notebook switch: the
// read-only timeline shows stored times, sources and the unresolved note.
func TestNotebookViewRendersStoredTimeline(t *testing.T) {
	w := NewWorkspace(sampleView(), Callbacks{})
	w.Notebook = notebook.Build(runs.RunSnapshot{
		RunID: "run-1", DesignID: "design-1", DesignRevision: 3, Status: "in_progress",
		CreationAudit: &runs.CreationAudit{ActorID: "tester", RecordedAt: "2026-11-10T10:00:00Z",
			Decision: "created_with_warnings", Warnings: []string{"任务未关联步骤"}},
		Tasks: []runs.RunTask{{Key: "task-1", DesignStepID: "step-s1"}},
		Events: []runs.RunEvent{
			{ID: "evt-batch", EventType: "measurement", TaskKey: "task-1", BatchID: "batch-1",
				SchemaID: "cck8.plate96.od450.v1", SchemaVersion: 1,
				OccurredAt: "2026-11-10T10:42:00+08:00", RecordedAt: "2026-11-10T10:43:00+08:00",
				ResultCells: []runs.ResultCell{{Row: 0, Column: 0, SampleID: "S1", Field: "OD450"}}},
			{ID: "evt-legacy", EventType: "observation", Content: "边缘气泡"},
		},
	})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, w)
	}, 1200, 760)
	tt.Frame()
	if err := tt.Click("运行日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"只读", "保存读数批次", "来源：读数批次 batch-1", "10:43", "边缘气泡", "时间未记录", "操作者 tester"} {
		if !tt.HasText(want) {
			t.Fatalf("notebook missing %q: %q", want, tt.Texts())
		}
	}
	if !w.NotebookView {
		t.Fatal("运行日志 tab did not switch the surface")
	}
	if err := tt.Click("查看任务"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.NotebookView || w.Selected != "task-1" {
		t.Fatalf("查看任务 did not return to the task: view=%v selected=%q", w.NotebookView, w.Selected)
	}
	w.NotebookView = true
	var opened string
	w.Callbacks.OpenBatch = func(batchID string) { opened = batchID }
	tt.Frame()
	if err := tt.Click("打开原始孔板"); err != nil {
		t.Fatalf("click batch link: %v", err)
	}
	if opened != "batch-1" {
		t.Fatalf("batch link opened %q", opened)
	}
	tt.Frame()
	tt.Frame()
	f, err := os.Create("evidence-notebook.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskChecksPersistAndBlockCompletion(t *testing.T) {
	v := RunView{
		Title: "检查项",
		Audit: &AuditInfo{ActorID: "tester", RecordedAt: "2026-11-10T10:00:00Z", Decision: "created_with_warnings",
			Warnings: []string{"任务未关联步骤"}},
		Tasks: []Task{{
			Key: "t1", Title: "读板", Status: "not_started",
			Checks: []TaskCheck{
				{CheckID: "blank", Label: "空白孔已确认", Blocking: true},
				{CheckID: "plate", Label: "板位已核对"},
			},
		}},
	}
	var saved, completed []CheckAck
	w := NewWorkspace(v, Callbacks{
		SaveChecks:   func(taskKey string, checks []CheckAck) { saved = checks },
		CompleteTask: func(taskKey string, checks []CheckAck) { completed = checks },
	})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, w)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("空白孔已确认") {
		t.Fatal("frozen check is not rendered")
	}
	if !tt.HasText("创建记录") || !tt.HasText("任务未关联步骤") {
		t.Fatal("creation audit is not visible in the run surface")
	}
	tt.Click("完成本步")
	tt.Frame()
	if completed != nil {
		t.Fatal("completion ran while a blocking check was unconfirmed")
	}
	if !tt.HasText("完成前需确认：空白孔已确认") {
		t.Fatal("blocking reason is not explained in the UI")
	}
	if f, err := os.Create("evidence-run-checks.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}

	tt.Click("空白孔已确认")
	tt.Frame()
	tt.Frame()
	if len(saved) == 0 || saved[0].CheckID != "blank" || saved[0].Status != "passed" {
		t.Fatalf("checkbox change was not persisted: %+v", saved)
	}

	w.View.Events = []Event{{TaskKey: "t1", Checks: saved}}
	w.SyncChecks()
	tt.Frame()
	tt.Click("完成本步")
	tt.Frame()
	if len(completed) == 0 || completed[0].Status != "passed" {
		t.Fatalf("completion did not carry the confirmed checks: %+v", completed)
	}
}

func TestSkippedTaskNeverCompletesLegacySurface(t *testing.T) {
	v := RunView{
		Title: "Skip Test",
		Tasks: []Task{
			{Key: "t1", Title: "离心", Timepoint: "D1", Status: "skipped"},
		},
	}
	completed := false
	w := NewWorkspace(v, Callbacks{
		CompleteTask: func(taskKey string, checks []CheckAck) { completed = true },
	})
	w.complete()
	if completed {
		t.Fatal("a skipped step must not complete on the legacy surface")
	}
	if w.State == StateSaving {
		t.Fatal("a skipped step must not enter the saving state")
	}
	if taskSkipped(nil) || !taskSkipped(&Task{Status: "skipped_by_rule"}) {
		t.Fatal("taskSkipped must match Core's skip statuses only")
	}
}
