package protocol

import (
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

const frozenDesign = `{
  "title": "CCK-8 设计工作区",
  "objective": "比较对照与处理组的细胞活力",
  "research_question": "处理是否降低活力？",
  "experimental_unit": "一个独立培养孔",
  "groups": [{"name":"Control"},{"name":"C1"}],
  "assays": [{"id":"cck8","title":"CCK-8"}],
  "mygo": {
    "workflow": {
      "Nodes": [
        {"id":"n1","kind":"flask","title":"铺板"},
        {"id":"n2","kind":"pipette","title":"读板"}
      ],
      "BusinessOrder": ["n1","n2"]
    },
    "stepAssays": {"n1":"cck8"},
    "assays": {"cck8":{"timepoints":["24 h"],"technicalReplicates":3,"wells":[
      {"wellId":"A1","sampleId":"S1","groupId":"Control","replicate":1,"timepoint":"24 h"},
      {"wellId":"B1","sampleId":"S2","groupId":"C1","replicate":1,"timepoint":"24 h"}
    ]}},
    "instances": {"n1":[
      {"id":"note-1","definitionId":"module-note","definitionVersion":1,"type":"note",
       "props":{"title":"铺板条件","text":"每孔 100 µL"}}
    ]}
  }
}`

// TestBuildProtocolIsDeterministicAndNamesGaps pins the Protocol contract: the
// projection follows business order, lists recorded structure, and names what
// the design does not contain instead of filling it in.
func TestBuildProtocolIsDeterministicAndNamesGaps(t *testing.T) {
	doc, err := Build(frozenDesign, "design-1", 4)
	if err != nil {
		t.Fatal(err)
	}
	if doc.DesignID != "design-1" || doc.Revision != 4 || doc.Title != "CCK-8 设计工作区" {
		t.Fatalf("header = %+v", doc)
	}
	if len(doc.Steps) != 2 || doc.Steps[0].Title != "铺板" || doc.Steps[1].Title != "读板" {
		t.Fatalf("steps = %+v", doc.Steps)
	}
	first := doc.Steps[0]
	if first.AssayID != "cck8" || len(first.Timepoints) != 1 || first.Timepoints[0] != "24 h" {
		t.Fatalf("assay plan = %+v", first)
	}
	if len(first.Modules) != 1 || first.Modules[0] != "备注" {
		t.Fatalf("modules = %+v", first.Modules)
	}
	if len(first.Notes) != 1 || !strings.Contains(first.Notes[0], "每孔 100 µL") {
		t.Fatalf("notes = %+v", first.Notes)
	}
	if doc.PlateWellCount != 2 {
		t.Fatalf("plate wells = %d", doc.PlateWellCount)
	}
	if len(doc.Groups) != 2 || doc.Groups[0] != "C1" || doc.Groups[1] != "Control" {
		t.Fatalf("groups = %+v", doc.Groups)
	}
	if !strings.Contains(strings.Join(doc.Missing, "\n"), "步骤「读板」未关联检测") {
		t.Fatalf("missing gaps = %+v", doc.Missing)
	}

	again, err := Build(frozenDesign, "design-1", 4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc, again) {
		t.Fatal("same design revision produced a different protocol")
	}
}

// TestViewDocumentShowsFrozenVersion guards the reading surface: the version,
// the objective and the named gap are visible.
func TestViewDocumentShowsFrozenVersion(t *testing.T) {
	doc, err := Build(frozenDesign, "design-1", 4)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		ViewDocument(c, doc)
	}, 1000, 800)
	tt.Frame()
	for _, want := range []string{"CCK-8 设计工作区", "冻结设计 · rev 4", "比较对照与处理组的细胞活力", "铺板", "步骤「读板」未关联检测"} {
		if !tt.HasText(want) {
			t.Fatalf("protocol view missing %q: %q", want, tt.Texts())
		}
	}
	f, err := os.Create("evidence-protocol.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
