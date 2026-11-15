package runworkspace

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
)

// TestLB02Evidence regenerates the LB-02 layout screenshots. It is opt-in so a
// normal `go test ./...` never writes into docs/; run it with
// LB02_EVIDENCE=1 go test ./internal/runworkspace -run TestLB02Evidence.
func TestLB02Evidence(t *testing.T) {
	if os.Getenv("LB02_EVIDENCE") == "" {
		t.Skip("set LB02_EVIDENCE=1 to regenerate docs/product/evidence/LB-02")
	}
	outDir := filepath.Join("..", "..", "..", "..", "docs", "product", "evidence", "LB-02")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	grid := resultgrid.New(resultgrid.Schema{
		ID: "huvec.plate12.curing.v1", Version: 1,
		RowLabels:    []string{"3%", "5%", "7%"},
		ColumnLabels: []string{"1", "2", "3", "4"},
		Fields:       []resultgrid.Field{{Key: "seconds", Label: "固化秒数", Unit: "s"}},
	}, nil)

	save := func(name string, w, h int, dark bool, mutate func(*State), table bool) {
		s := buildTestState()
		s.Callbacks.Complete = func(string, []CheckAckView) error { return nil }
		s.Callbacks.Recording = func(c *ui.Context, task *RunTaskViewModel) {
			rv := &RecordingView{Kind: ViewPlate, Grid: grid, Task: task}
			if table {
				// The narrow shot uses the table projection so the recording
				// shows a populated record instead of an empty plate.
				rv = &RecordingView{Kind: ViewTable, Task: task}
			}
			rv.Render(c)
		}
		if mutate != nil {
			mutate(s)
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			View(c, s)
		}, w, h, ui.TesterOptions{Dark: dark})
		tt.Frame()
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}

	// ≥1100: left flow chain, middle record projection, right properties.
	save("lb02-1360x880-light.png", 1360, 880, false, nil, false)
	save("lb02-1360x880-dark.png", 1360, 880, true, nil, false)

	// <1100: one column, recording first, procedure collapsed.
	save("lb02-900x600-light.png", 900, 600, false, func(s *State) { s.Narrow = true }, true)
	save("lb02-900x600-dark.png", 900, 600, true, func(s *State) { s.Narrow = true }, true)

	// Properties collapse before/after: raw identifiers hidden, then revealed.
	save("lb02-properties-collapsed-1360x880-light.png", 1360, 880, false, func(s *State) { s.ShowTechnical = false }, false)
	save("lb02-properties-expanded-1360x880-light.png", 1360, 880, false, func(s *State) { s.ShowTechnical = true }, false)
}
