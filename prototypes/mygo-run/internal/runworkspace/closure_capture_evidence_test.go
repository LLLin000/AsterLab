package runworkspace

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

// TestClosureCaptureEvidence regenerates the CLOSE-06 capture screenshots. It
// is opt-in so a normal `go test ./...` never writes into docs/; run it with
// CLOSE06_EVIDENCE=1 go test ./internal/runworkspace -run TestClosureCaptureEvidence.
//
// The shots render the same bench strip + gallery the Run window draws: the
// strip fills from the frozen plan (9 planned wells x 6 FOV), the 7th image
// overflows to the next unfilled well, and whole unphotographed wells stay
// visible as gaps with same-task refill. The OS native file picker cannot be
// driven headlessly, so the picker step itself is coordinator Gate material;
// every slot decision around it is asserted here.
func TestClosureCaptureEvidence(t *testing.T) {
	if os.Getenv("CLOSE06_EVIDENCE") == "" {
		t.Skip("set CLOSE06_EVIDENCE=1 to regenerate docs/product/evidence/closure-capture")
	}
	outDir := filepath.Join("..", "..", "..", "..", "docs", "product", "evidence", "closure-capture")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	huvecWells := []string{"A1", "A2", "A3", "B1", "B2", "B3", "C1", "C2", "C3"}
	huvecAssets := func() []LinkedAsset {
		out := make([]LinkedAsset, 0, 7)
		for i := range 6 {
			out = append(out, LinkedAsset{
				AssetID: "a1-fov" + string(rune('1'+i)), RelativePath: "huvec/6h/A1_F0" + string(rune('1'+i)) + ".tif",
				WellID: "A1", Timepoint: "D1", FOVIndex: uint32(i + 1), Available: true,
			})
		}
		return append(out, LinkedAsset{
			AssetID: "a2-fov1", RelativePath: "huvec/6h/A2_F01.tif",
			WellID: "A2", Timepoint: "D1", FOVIndex: 1, Available: true,
		})
	}

	build := func(wells []string, fov int, assets []LinkedAsset) (*State, *bool) {
		batchClicked := false
		s := buildTestState()
		s.Selected = "t-read"
		s.Callbacks.LoadTimers = func(string) ([]runs.TimerRecord, error) { return nil, nil }
		s.Callbacks.AppendEvent = func(string, runs.ExecutionEventInput) (string, error) { return "", nil }
		s.Callbacks.LoadPantry = func() (runs.PantryLedger, error) { return runs.PantryLedger{}, nil }
		s.Callbacks.TaskAssets = func(string) []LinkedAsset { return assets }
		s.Callbacks.CaptureWells = func(string) ([]string, int) { return wells, fov }
		s.Callbacks.RequestBatchLink = func(string) { batchClicked = true }
		s.Callbacks.Recording = func(c *ui.Context, task *RunTaskViewModel) {
			rv := &RecordingView{
				Kind: ViewGallery, Task: task, Assets: assets,
				PlannedWells: wells, PlannedTimepoints: []string{"D1"}, PlannedFOV: fov,
				OnOpenAsset:    func(LinkedAsset) {},
				OnRelocate:     func(LinkedAsset) {},
				OnRelabelAsset: func(LinkedAsset) {},
				OnUnlinkAsset:  func(LinkedAsset) {},
				OnLinkFile:     func(LinkedAsset) {},
			}
			rv.Render(c)
		}
		return s, &batchClicked
	}

	save := func(name string, s *State, w, h int, dark, narrow bool) *ui.Tester {
		if narrow {
			s.Narrow = true
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
		return tt
	}

	// 9 planned wells x 6 FOV with 7 linked files: A1 6/6, A2 1/6, the 7th
	// image overflowed to the next unfilled well.
	s, batchClicked := build(huvecWells, 6, huvecAssets())
	tt := save("capture-huvec-1360x880-light.png", s, 1360, 880, false, false)
	for _, want := range []string{"采图进度", "A1 6/6", "A2 1/6", "1/9 孔已传满", "批量关联", "A2 缺 FOV [2 3 4 5 6]", "孔位 × 时间点", "解除关联", "重设坐标"} {
		if !tt.HasText(want) {
			t.Fatalf("huvec light capture missing %q: %q", want, tt.Texts())
		}
	}
	if err := tt.Click("批量关联"); err != nil {
		t.Fatalf("strip batch entry did not invoke callback: %v", err)
	}
	if !*batchClicked {
		t.Fatal("RequestBatchLink was not invoked from the strip")
	}

	s2, _ := build(huvecWells, 6, huvecAssets())
	save("capture-huvec-1360x880-dark.png", s2, 1360, 880, true, false)
	s3, _ := build(huvecWells, 6, huvecAssets())
	save("capture-huvec-900x600-light.png", s3, 900, 600, false, true)
	s4, _ := build(huvecWells, 6, huvecAssets())
	tt4 := save("capture-huvec-900x600-dark.png", s4, 900, 600, true, true)
	if !tt4.HasText("A1 6/6") || !tt4.HasText("批量关联") {
		t.Fatalf("narrow dark capture missing strip: %q", tt4.Texts())
	}

	// 1D container: tube rack Lane1..Lane8, FOV 1.
	laneWells := []string{"Lane1", "Lane2", "Lane3", "Lane4", "Lane5", "Lane6", "Lane7", "Lane8"}
	laneAssets := []LinkedAsset{{
		AssetID: "lane-1", RelativePath: "cfu/Lane1.tif",
		WellID: "Lane1", Timepoint: "D1", FOVIndex: 1, Available: true,
	}}
	s5, _ := build(laneWells, 1, laneAssets)
	tt5 := save("capture-1d-1360x880-light.png", s5, 1360, 880, false, false)
	for _, want := range []string{"Lane1 1/1", "Lane2 0/1", "批量关联", "补齐"} {
		if !tt5.HasText(want) {
			t.Fatalf("1D capture missing %q: %q", want, tt5.Texts())
		}
	}
	s6, _ := build(laneWells, 1, laneAssets)
	save("capture-1d-900x600-dark.png", s6, 900, 600, true, true)

	// 384 plate subset: only planned wells render, never 384 capacity.
	s7, _ := build([]string{"A1", "P24"}, 6, []LinkedAsset{{
		AssetID: "w1", RelativePath: "plate384/A1_F01.tif",
		WellID: "A1", Timepoint: "D1", FOVIndex: 1, Available: true,
	}})
	tt7 := save("capture-384-1360x880-light.png", s7, 1360, 880, false, false)
	for _, want := range []string{"A1 1/6", "P24 0/6", "0/2 孔已传满"} {
		if !tt7.HasText(want) {
			t.Fatalf("384 capture missing %q: %q", want, tt7.Texts())
		}
	}
	s8, _ := build([]string{"A1", "P24"}, 6, nil)
	save("capture-384-900x600-dark.png", s8, 900, 600, true, true)
}
