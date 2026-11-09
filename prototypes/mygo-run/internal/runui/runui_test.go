package runui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
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
		Checks: []Check{{Key: "m", Title: "材料", Binding: "GelMA"}},
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
