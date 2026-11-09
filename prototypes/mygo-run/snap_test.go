package main

import (
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

func TestSnap(t *testing.T) {
	page, step := "run", "design-l929"
	a := &app{page: &page, step: &step, samples: []sample{{"Control-1", ""}, {"E1-1", ""}}, snap: runs.RunSnapshot{Title: "L929 生物相容性", Status: "进行中", Tasks: []runs.RunTask{{Key: "a", Title: "Seed", Timepoint: "D1", Status: "completed"}, {Key: "b", Title: "CCK-8", Timepoint: "D3", Status: "not_started"}, {Key: "c", Title: "Image", Timepoint: "D7", Status: "not_started"}}, Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}}}}
	tt := ui.NewTester(a.view, 1200, 760)
	tt.Frame()
	img := tt.Image()
	f, err := os.Create("../../docs/design/mygo-prototype.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
