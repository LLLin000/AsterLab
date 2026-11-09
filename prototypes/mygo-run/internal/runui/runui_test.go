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
