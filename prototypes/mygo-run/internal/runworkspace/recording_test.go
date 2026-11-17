package runworkspace

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
)

func TestRecordingPlateView(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	grid := resultgrid.New(resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Version: 1,
		RowLabels:    []string{"A", "B"},
		ColumnLabels: []string{"1", "2"},
		Fields:       []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}, nil)
	rv := &RecordingView{Kind: ViewPlate, Grid: grid, Task: task}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1000, 700)
	tt.Frame()
	if !tt.HasText("cck8.plate96.od450.v1") {
		t.Fatal("plate schema id missing")
	}
}

func TestRecordingTableViewShowsMeasurements(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	rv := &RecordingView{Kind: ViewTable, Task: task}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1000, 700)
	tt.Frame()
	if !tt.HasText("Control-1") {
		t.Fatal("measurement sample missing")
	}
	if !tt.HasText("OD450") {
		t.Fatal("measurement field missing")
	}
}

func TestRecordingGalleryShowsFiles(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	var relocated LinkedAsset
	rv := &RecordingView{
		Kind: ViewGallery,
		Task: task,
		OnRelocate: func(asset LinkedAsset) { relocated = asset },
		Assets: []LinkedAsset{
			{AssetID: "a1", RootID: "project-data", RelativePath: "huvec/d1/image_01.tif",
				MediaType: "image/tiff", Size: 18742000, Available: true},
			{AssetID: "a2", RootID: "project-data", RelativePath: "huvec/d1/missing.tif",
				MediaType: "image/tiff", Available: false, WellID: "A1", Timepoint: "t6h", FOVIndex: 1},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1000, 700)
	tt.Frame()
	if !tt.HasText("huvec/d1/image_01.tif") {
		t.Fatal("asset path missing")
	}
	// UX-P5 / UX-STATE / UX-DONE: unavailable links remain named and provide a relocation action.
	if !tt.HasText("未解析 · 文件不可用") {
		t.Fatalf("unavailable file state missing: %q", tt.Texts())
	}
	if err := tt.Click("重新定位"); err != nil {
		t.Fatal(err)
	}
	if relocated.AssetID != "a2" {
		t.Fatalf("relocated asset = %+v, want missing link a2", relocated)
	}
}

func TestRecordingCoreWriteErrorOffersRetry(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	grid := resultgrid.New(resultgrid.Schema{
		ID: "readout", Version: 1, RowLabels: []string{"A1"}, ColumnLabels: []string{"OD450"},
	}, nil)
	grid.State, grid.Error = resultgrid.StateError, "permission denied: asset root is read-only"
	saved := false
	rv := &RecordingView{Kind: ViewTable, Grid: grid, Task: task, OnSaveBatch: func([]resultgrid.Cell) { saved = true }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	// UX-P5 / UX-STATE / UX-DONE: show Core cause and permit an explicit retry.
	for _, text := range []string{"未保存", "permission denied: asset root is read-only", "检查目标目录权限", "重试保存"} {
		if !tt.HasText(text) {
			t.Fatalf("failed-write state missing %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click("重试保存"); err != nil {
		t.Fatal(err)
	}
	if !saved {
		t.Fatal("重试保存 did not invoke retry")
	}
}

func TestRecordingFormShowsPlannedParameters(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	if task.Planned == nil {
		t.Fatal("task has no planned step")
	}
	rv := &RecordingView{Kind: ViewForm, Task: task}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 1000, 700)
	tt.Frame()
	if !tt.HasText("计划与实际参数") {
		t.Fatal("form header missing")
	}
}

func TestSelectKindPrefersPlate(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	grid := resultgrid.New(resultgrid.Schema{
		ID: "s", RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
	}, nil)
	rv := &RecordingView{Kind: SelectKind(task, grid, nil), Grid: grid, Task: task}
	if rv.Kind != ViewPlate {
		t.Fatalf("kind = %q", rv.Kind)
	}
	if k := SelectKind(task, nil, nil); k != ViewTable {
		t.Fatalf("no-grid kind = %q, want table", k)
	}
}

func TestHumanSize(t *testing.T) {
	if humanSize(900) != "900 B" {
		t.Fatal(humanSize(900))
	}
	if humanSize(2048) != "2 KB" {
		t.Fatal(humanSize(2048))
	}
	if humanSize(5<<20) != "5.0 MB" {
		t.Fatal(humanSize(5 << 20))
	}
}
func TestRecordingGridViewsShareEditsAndSave(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	grid := resultgrid.New(resultgrid.Schema{
		ID: "shared", RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
		Fields: []resultgrid.Field{{Key: "od", Label: "OD", Unit: "AU"}},
	}, nil)
	var saved []resultgrid.Cell
	rv := &RecordingView{
		Kind: ViewPlate, Grid: grid, Task: vm.TaskByKey["t-read"],
		OnSaveBatch: func(cells []resultgrid.Cell) { saved = cells },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()

	draft, _ := grid.BeginEdit(0, 0)
	*draft = "1.25"
	grid.MarkEditDirty()
	if err := tt.Click("表格"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if grid.Cells[0].Value == nil || *grid.Cells[0].Value != 1.25 {
		t.Fatalf("plate edit not committed to shared cells: %+v", grid.Cells[0])
	}
	if input, ok := grid.InputAt(0, 0); !ok || *input != "1.25" {
		t.Fatalf("table projection input = %v, want 1.25", input)
	}

	if err := tt.Click("A1 读数"); err != nil {
		t.Fatalf("focus table value: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2.5")
	tt.Frame()
	if grid.Cells[0].Value == nil || *grid.Cells[0].Value != 2.5 {
		t.Fatalf("table edit not committed to shared cells: %+v", grid.Cells[0])
	}
	if input, ok := grid.InputAt(0, 0); !ok || *input != "2.5" {
		t.Fatalf("table input = %v, want 2.5", input)
	}

	if err := tt.Click("孔板"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已记录 1") || !tt.HasText("缺失 0") {
		t.Fatalf("plate counts did not project updated cell: %v", tt.Texts())
	}
	if err := tt.Click("保存读数"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(saved) != 1 || saved[0].Value == nil || *saved[0].Value != 2.5 {
		t.Fatalf("save callback did not receive exact shared cells: %+v", saved)
	}
}

func TestRecordingGridLayoutsRenderAtBothBreakpointsAndThemes(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	rows, cols := make([]string, 8), make([]string, 12)
	for i := range rows {
		rows[i] = string(rune('A' + i))
	}
	for i := range cols {
		cols[i] = fmt.Sprint(i + 1)
	}
	for _, size := range [][2]int{{1360, 880}, {900, 600}} {
		for _, dark := range []bool{false, true} {
			for _, view := range []ViewKind{ViewPlate, ViewTable} {
				grid := resultgrid.New(resultgrid.Schema{ID: "96-well", RowLabels: rows, ColumnLabels: cols}, nil)
				rv := &RecordingView{Kind: view, Grid: grid, Task: vm.TaskByKey["t-read"]}
				tt := ui.NewTester(func(c *ui.Context) {
					if dark {
						c.SetTheme(theme.DarkTheme())
					} else {
						c.SetTheme(theme.LightTheme())
					}
					rv.Render(c)
				}, size[0], size[1])
				tt.Frame()
			}
		}
	}
}

func TestSelectKindRoutesByAssayLayoutConfig(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]

	cck8Grid := resultgrid.New(resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Layout: "plate96", RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
	}, nil)
	if got := SelectKind(task, cck8Grid, nil); got != ViewPlate {
		t.Fatalf("CCK-8 kind = %q, want plate", got)
	}

	qpcrGrid := resultgrid.New(resultgrid.Schema{
		ID: "qpcr.sample_gene.cq.v1", Layout: "sample_gene", RowLabels: []string{"S1"}, ColumnLabels: []string{"GAPDH_R1"},
	}, nil)
	if got := SelectKind(task, qpcrGrid, nil); got != ViewTable {
		t.Fatalf("RNA/qPCR kind = %q, want table", got)
	}

	wbGrid := resultgrid.New(resultgrid.Schema{
		ID: "qpcr.sample_gene.cq.v1", Layout: "sample_target", RowLabels: []string{"Control"}, ColumnLabels: []string{"Target_R1"},
	}, nil)
	if got := SelectKind(task, wbGrid, nil); got != ViewTable {
		t.Fatalf("WesternBlot kind = %q, want table", got)
	}
}

func TestRecordingLinkErrorShowsCauseAndRetry(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	var linked int
	rv := &RecordingView{
		Kind: ViewGallery, Task: task, LinkErr: "root is read-only: permission denied",
		OnLinkFile: func(asset LinkedAsset) { linked++ },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	for _, text := range []string{"孔位 × 时间点", "关联失败：root is read-only: permission denied", "重新关联"} {
		if !tt.HasText(text) {
			t.Fatalf("link-error surface missing %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click("重新关联"); err != nil {
		t.Fatal(err)
	}
	if linked != 1 {
		t.Fatalf("重新关联 did not invoke OnLinkFile: linked=%d", linked)
	}
}

func TestAssetMatrixGroupsCoordinatesAndCountsFOVGapsOnce(t *testing.T) {
	cells := assetMatrix([]LinkedAsset{
		{AssetID: "fov3-a", WellID: "A1", Timepoint: "t6h", FOVIndex: 3},
		{AssetID: "fov1", WellID: "A1", Timepoint: "t6h", FOVIndex: 1},
		{AssetID: "fov3-b", WellID: "A1", Timepoint: "t6h", FOVIndex: 3},
		{AssetID: "other", WellID: "B1", Timepoint: "t6h", FOVIndex: 1},
	}, nil, nil, 0)
	if len(cells) != 2 || cells[0].WellID != "A1" || cells[0].Timepoint != "t6h" ||
		cells[1].WellID != "B1" || cells[1].Timepoint != "t6h" {
		t.Fatalf("matrix keys = %+v", cells)
	}
	if len(cells[0].Assets) != 3 || cells[0].Assets[0].FOVIndex != 1 || cells[0].Assets[1].FOVIndex != 3 {
		t.Fatalf("assets not ordered: %+v", cells[0].Assets)
	}
	if len(cells[0].FOVIndices) != 2 || cells[0].FOVIndices[0] != 1 || cells[0].FOVIndices[1] != 3 {
		t.Fatalf("FOV indices not unique and ordered: %+v", cells[0].FOVIndices)
	}
	if len(cells[0].Gaps) != 1 || cells[0].Gaps[0] != 2 {
		t.Fatalf("FOV gap counted incorrectly: %+v", cells[0].Gaps)
	}
}

func TestAssetMatrixKeepsMissingCoordinatesUnlabelledWithoutPathInference(t *testing.T) {
	cells := assetMatrix([]LinkedAsset{{
		AssetID: "unlabelled", RelativePath: "A1_24h_F03.tif",
	}}, nil, nil, 0)
	if len(cells) != 1 || cells[0].WellID != "未标注" || len(cells[0].Assets) != 1 ||
		cells[0].Assets[0].WellID != "" || cells[0].Assets[0].Timepoint != "" ||
		cells[0].Assets[0].FOVIndex != 0 {
		t.Fatalf("filename-derived or dropped asset: %+v", cells)
	}
}
func TestRecordingAssetPreviewShowsCoordinatesAndTimelineLink(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	timeline := false
	rv := &RecordingView{
		Kind: ViewGallery, Task: task,
		Assets: []LinkedAsset{{
			AssetID: "a1", RelativePath: "images/A1_24h_F02.tif",
			WellID: "A1", Timepoint: "t6h", FOVIndex: 2,
		}},
		OnTimeline: func() { timeline = true },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	for _, text := range []string{"孔位：A1 · 时间点：t6h · 视野：2", "返回 Run 时间线"} {
		if !tt.HasText(text) {
			t.Fatalf("asset preview missing %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click("返回 Run 时间线"); err != nil {
		t.Fatal(err)
	}
	if !timeline {
		t.Fatal("timeline link did not invoke callback")
	}
}

func TestRecordingMatrixRendersEmptyPairsAsGaps(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	rv := &RecordingView{
		Kind: ViewGallery, Task: vm.TaskByKey["t-read"],
		Assets: []LinkedAsset{
			{AssetID: "a", WellID: "A1", Timepoint: "t6h", FOVIndex: 1},
			{AssetID: "b", WellID: "B1", Timepoint: "t12h", FOVIndex: 1},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("1 个缺口 · 未关联视野") {
		t.Fatalf("empty matrix pair not explicit: %q", tt.Texts())
	}
}

func TestRecordingMatrixGapPrefillUsesCellCoordinates(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	var picked LinkedAsset
	rv := &RecordingView{
		Kind: ViewGallery, Task: vm.TaskByKey["t-read"],
		Assets: []LinkedAsset{
			{AssetID: "a", WellID: "A1", Timepoint: "t6h", FOVIndex: 1},
			{AssetID: "b", WellID: "A1", Timepoint: "t24h", FOVIndex: 1},
			{AssetID: "c", WellID: "A1", Timepoint: "t24h", FOVIndex: 3},
		},
		OnLinkFile: func(asset LinkedAsset) { picked = asset },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	if err := tt.Click("补齐 FOV 2"); err != nil {
		t.Fatal(err)
	}
	if picked.WellID != "A1" || picked.Timepoint != "t24h" || picked.FOVIndex != 2 {
		t.Fatalf("gap prefill = %+v, want A1/t24h/FOV2", picked)
	}
}

func TestUnlabelledAssetOffersRelabelAndLabelledOffersRelabelUnlink(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	var relabelled, unlinked []LinkedAsset
	rv := &RecordingView{
		Kind: ViewGallery, Task: vm.TaskByKey["t-read"],
		Assets:      []LinkedAsset{{AssetID: "unlabelled", RelativePath: "sample.tif", Available: true}},
		OnOpenAsset: func(LinkedAsset) { t.Fatal("unlabelled asset offered open action") },
		OnRelocate:  func(LinkedAsset) { t.Fatal("unlabelled asset offered relocate action") },
		OnRelabelAsset: func(asset LinkedAsset) {
			relabelled = append(relabelled, asset)
		},
		OnUnlinkAsset: func(asset LinkedAsset) {
			unlinked = append(unlinked, asset)
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("未标注 · 可重设坐标或解除关联") {
		t.Fatalf("unlabelled explanation missing: %q", tt.Texts())
	}
	if tt.HasText("打开") || tt.HasText("重新定位") {
		t.Fatalf("unlabelled asset offered unexpected open/relocate action: %q", tt.Texts())
	}
	if err := tt.Click("重设坐标"); err != nil {
		t.Fatalf("unlabelled asset did not offer relabel action: %v", err)
	}
	if len(relabelled) != 1 || relabelled[0].AssetID != "unlabelled" {
		t.Fatalf("relabel callback = %+v, want unlabelled asset", relabelled)
	}
	if err := tt.Click("解除关联"); err != nil {
		t.Fatalf("unlabelled asset did not offer unlink action: %v", err)
	}
	if len(unlinked) != 1 || unlinked[0].AssetID != "unlabelled" {
		t.Fatalf("unlink callback = %+v, want unlabelled asset", unlinked)
	}

	rv.Assets = []LinkedAsset{{
		AssetID: "labelled", RelativePath: "labelled.tif", Available: true,
		WellID: "A1", Timepoint: "t6h", FOVIndex: 1,
	}}
	rv.OnOpenAsset = func(LinkedAsset) {}
	tt.Frame()
	// A labelled link can be opened, relabelled to a corrected slot, or
	// unlinked; all three go through Core and sync every surface (UX-P5).
	for _, want := range []string{"打开", "重设坐标", "解除关联"} {
		if !tt.HasText(want) {
			t.Fatalf("labelled asset missing action %q: %q", want, tt.Texts())
		}
	}
}

func TestAssetMatrixUnionsFrozenPlanWholeWellsAsGaps(t *testing.T) {
	cells := assetMatrix([]LinkedAsset{
		{AssetID: "a1", WellID: "A1", Timepoint: "6h", FOVIndex: 1},
	}, []string{"A1", "A2", "A3"}, []string{"6h"}, 6)
	if len(cells) != 3 {
		t.Fatalf("plan union cells = %+v, want 3", cells)
	}
	byWell := map[string]assetMatrixCell{}
	for _, cell := range cells {
		byWell[cell.WellID] = cell
	}
	// Plan fill order leads; recorded extras would follow.
	if cells[0].WellID != "A1" || cells[1].WellID != "A2" || cells[2].WellID != "A3" {
		t.Fatalf("plan order lost: %+v", cells)
	}
	a1 := byWell["A1"]
	if len(a1.Gaps) != 5 || a1.Gaps[0] != 2 || a1.Gaps[4] != 6 {
		t.Fatalf("A1 gaps = %v, want [2..6]", a1.Gaps)
	}
	// Whole unphotographed wells surface every planned slot as a gap.
	for _, well := range []string{"A2", "A3"} {
		cell := byWell[well]
		if len(cell.Assets) != 0 || len(cell.Gaps) != 6 || cell.Gaps[0] != 1 || cell.Gaps[5] != 6 {
			t.Fatalf("%s = %+v, want empty with 6 gaps", well, cell)
		}
	}
}

func TestRecordingGalleryRendersWholePlanGapsWithRefill(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	var picked LinkedAsset
	rv := &RecordingView{
		Kind: ViewGallery, Task: vm.TaskByKey["t-read"],
		Assets: []LinkedAsset{
			{AssetID: "a1", WellID: "A1", Timepoint: "6h", FOVIndex: 1},
		},
		PlannedWells:      []string{"A1", "A2"},
		PlannedTimepoints: []string{"6h"},
		PlannedFOV:        2,
		OnLinkFile:        func(asset LinkedAsset) { picked = asset },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		rv.Render(c)
	}, 900, 600)
	tt.Frame()
	// The unphotographed A2 renders from the plan with its own refill.
	if !tt.HasText("2 个缺口 · FOV [1 2]") {
		t.Fatalf("whole-plan gap missing: %q", tt.Texts())
	}
	if err := tt.Click("补齐 FOV 1"); err != nil {
		t.Fatal(err)
	}
	if picked.WellID != "A2" || picked.Timepoint != "6h" || picked.FOVIndex != 1 {
		t.Fatalf("whole-plan refill prefill = %+v, want A2/6h/FOV1", picked)
	}
}
