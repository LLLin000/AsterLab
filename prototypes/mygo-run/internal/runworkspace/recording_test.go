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
				MediaType: "image/tiff", Available: false},
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
