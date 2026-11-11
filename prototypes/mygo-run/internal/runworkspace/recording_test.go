package runworkspace

import (
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
	rv := &RecordingView{
		Kind: ViewGallery,
		Task: task,
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
	if !tt.HasText("文件不可用") {
		t.Fatal("unavailable file marker missing")
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
