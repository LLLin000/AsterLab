package main

import (
	"image/png"
	"os"
	"strconv"
	"testing"

	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func TestSnap(t *testing.T) {
	page, step := "run", "design-l929"
	a := &app{page: &page, step: &step, samples: []sample{{"Control-1", ""}, {"E1-1", ""}}}
	a.snap = runs.RunSnapshot{Title: "L929 生物相容性", Status: "进行中", Tasks: []runs.RunTask{{Key: "a", Title: "Seed", Timepoint: "D1", Status: "completed"}, {Key: "b", Title: "CCK-8", Timepoint: "D3", Status: "not_started"}, {Key: "c", Title: "Image", Timepoint: "D7", Status: "not_started"}}, Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}}}
	a.shell = &shell.Shell{Project: "L929", Page: shell.Run, SidebarOpen: true, Designs: []shell.DesignItem{{ID: "d1", Title: "L929 生物相容性"}}, Selected: "d1", Content: func(c *ui.Context) { contentPage(c, a) }}
	a.ws = runui.NewWorkspace(toRunView(a.snap), runui.Callbacks{})
	a.ws.State = runui.StateReady
	for _, sz := range []struct{ w, h int }{{1200, 760}, {900, 600}} {
		tt := ui.NewTester(a.view, sz.w, sz.h)
		tt.Frame()
		name := "snapshot-" + strconv.Itoa(sz.w) + "x" + strconv.Itoa(sz.h) + ".png"
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}
