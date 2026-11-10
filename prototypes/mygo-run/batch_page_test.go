package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestBatchPageShowsStoredCellsReadOnly(t *testing.T) {
	value := 0.82
	a := &app{
		batchView: "batch-1",
		snap: runs.RunSnapshot{
			RunID: "run-1",
			Events: []runs.RunEvent{{
				ID: "evt-1", BatchID: "batch-1", AssayID: "cck8", Timepoint: "D1",
				RecordedAt: "2026-11-10T10:43:00Z",
				ResultCells: []runs.ResultCell{{
					Row: 0, Column: 0, SampleID: "Control-1", Group: "Control",
					Replicate: 1, Field: "od450", Value: &value, Status: "raw",
				}},
			}},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		batchPage(c, a)
	}, 1200, 760)
	tt.Frame()
	if !tt.HasText("原始读数") || !tt.HasText("Control-1") || !tt.HasText("0.82") {
		t.Fatal("stored batch cells are not rendered")
	}
	if !tt.HasText("此页不能修改原始读数") {
		t.Fatal("read-only boundary is not stated")
	}
	if f, err := os.Create("internal/runui/evidence-batch.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}
	if err := tt.Click("← 返回 Notebook"); err != nil {
		t.Fatalf("back: %v", err)
	}
	tt.Frame()
	if a.batchView != "" {
		t.Fatal("back did not leave the batch view")
	}
}
