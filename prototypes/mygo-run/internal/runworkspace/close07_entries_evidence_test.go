package runworkspace

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

// TestClose07EntriesEvidence regenerates the CLOSE-07 entry-matrix
// screenshots. Opt-in so a normal `go test ./...` never writes into docs/:
// CLOSE07_EVIDENCE=1 go test ./internal/runworkspace -run TestClose07EntriesEvidence.
//
// The shots render the same task workspace the Run window draws, one task
// per state:
//   - readout task (measurement plan + recorded values): the bench carries
//     timers, the skip/deviation/observation composer and the sample-scoped
//     exclusion + typed-readout sections; 保存读数 is the single primary,
//     never beside 标记完成. UX-IA-1, UX-P1.
//   - procedure task (no checks, no measurement plan): sample sections stay
//     hidden, 标记完成 is the single primary. UX-IA-1, UX-P1.
//   - the Run top bar carries the only blind switch; the bench keeps none.
//     UX-IA-1, UX-STATE.
//
// The OS native file picker cannot be driven headlessly, so picker clicks
// stay coordinator Gate material; every entry decision around them is
// asserted here.
func TestClose07EntriesEvidence(t *testing.T) {
	if os.Getenv("CLOSE07_EVIDENCE") == "" {
		t.Skip("set CLOSE07_EVIDENCE=1 to regenerate docs/product/evidence/closure-entries")
	}
	outDir := filepath.Join("..", "..", "..", "..", "docs", "product", "evidence", "closure-entries")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	snap := makeSnapshot()
	snap.Tasks = append(snap.Tasks, runs.RunTask{
		Key: "t-spin", DesignStepID: "step-spin", Title: "离心去上清",
		Path: "root/spin", Status: "not_started",
		Instruction: "2000 rpm 离心 5 分钟，弃上清。",
	})
	vm := BuildViewModel(snap, makeFrozenDoc())
	var s *State
	s = NewState(vm, Callbacks{
		SelectTask: func(taskKey string) { s.Selected = taskKey },
		LoadTimers: func(string) ([]runs.TimerRecord, error) { return nil, nil },
		AppendEvent: func(string, runs.ExecutionEventInput) (string, error) {
			return "", nil
		},
		ExecuteCommand: func(runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error) {
			return runs.ExecuteCommandResult{}, nil
		},
		CaptureWells: func(taskKey string) ([]string, int) { return nil, 0 },
		Recording: func(c *ui.Context, task *RunTaskViewModel) {
			// The host only builds grids for measurement tasks; the
			// procedure step has no recording grid, like production.
			if task == nil || task.TaskKey != "t-read" {
				return
			}
			grid := resultgrid.New(resultgrid.Schema{
				ID: "cck8.plate96.od450.v1", Version: 1,
				RowLabels:    []string{"A", "B"},
				ColumnLabels: []string{"1", "2"},
				Fields:       []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
			}, nil)
			rv := &RecordingView{Kind: ViewTable, Grid: grid, Task: task,
				OnSaveBatch: func([]resultgrid.Cell) {}}
			rv.Render(c)
		},
	})

	save := func(name, selected string, w, h int, dark, narrow bool) *ui.Tester {
		s.Selected = selected
		s.Narrow = narrow
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
		return tt
	}

	// Readout task: one primary (保存读数), sample sections present.
	read := save("entries-readout-1360x880-light.png", "t-read", 1360, 880, false, false)
	for _, want := range []string{"执行台面", "计时", "偏差与跳过", "样本排除", "分类与表格读数", "保存读数", "设盲视图"} {
		if !read.HasText(want) {
			t.Fatalf("readout entries missing %q: %q", want, read.Texts())
		}
	}
	for _, hidden := range []string{"标记完成", "确认按规则跳过", "记录偏差", "记录观察", "记录排除", "提交读数", "推进一轮", "have no design step"} {
		if read.HasText(hidden) {
			t.Fatalf("readout task must not offer competing primary %q: %q", hidden, read.Texts())
		}
	}
	// Layer diagnostics collapse behind one disclosure by default (UX-P2);
	// only the count label shows until the operator opens it.
	if !read.HasText("层级诊断（1）") {
		t.Fatalf("diagnostics disclosure missing: %q", read.Texts())
	}
	save("entries-readout-1360x880-dark.png", "t-read", 1360, 880, true, false)
	save("entries-readout-900x600-light.png", "t-read", 900, 600, false, true)
	narrow := save("entries-readout-900x600-dark.png", "t-read", 900, 600, true, true)
	if !narrow.HasText("保存读数") {
		t.Fatalf("narrow readout missing primary: %q", narrow.Texts())
	}

	// Procedure task: sample sections hidden, one primary (标记完成).
	pure := save("entries-procedure-1360x880-light.png", "t-spin", 1360, 880, false, false)
	for _, want := range []string{"偏差与跳过", "标记完成", "设盲视图"} {
		if !pure.HasText(want) {
			t.Fatalf("procedure entries missing %q: %q", want, pure.Texts())
		}
	}
	for _, hidden := range []string{"样本排除", "分类与表格读数", "保存读数", "确认按规则跳过", "记录偏差", "记录观察", "推进一轮"} {
		if pure.HasText(hidden) {
			t.Fatalf("procedure task must not offer %q: %q", hidden, pure.Texts())
		}
	}
	save("entries-procedure-900x600-dark.png", "t-spin", 900, 600, true, true)
}

