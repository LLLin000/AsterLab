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
	a := &app{page: &page, step: &step, samples: []sample{{"Control-1", ""}, {"E1-1", ""}}, snap: runs.RunSnapshot{Title: "L929 生物相容性", Status: "进行中"}}
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
