package tools

import (
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/blocks"
	"github.com/egoist/mygo/ui"
)

func TestToolDraftsAndImport(t *testing.T) {
	var drafts []Draft
	var used []blocks.Instance
	w := New(Callbacks{UseInDesigner: func(i blocks.Instance) { used = append(used, i) }, CreateDraft: func(d Draft) { drafts = append(drafts, d) }})
	if err := w.Plate.ToggleWell(0); err != nil {
		t.Fatal(err)
	}
	if err := w.Plate.SetGroup(0, "Control"); err != nil {
		t.Fatal(err)
	}
	w.Active = ToolPlate
	w.handoff()
	if w.Error != "" || len(used) != 1 || used[0].Type != blocks.TypePlate96 {
		t.Fatalf("plate handoff: err=%q used=%v", w.Error, used)
	}
	var plate blocks.Plate96Props
	if err := used[0].DecodeProps(&plate); err != nil {
		t.Fatal(err)
	}
	if len(plate.Selected) != 1 || plate.Groups[0] != "Control" {
		t.Fatalf("plate=%+v", plate)
	}

	w.Active, w.Stock, w.Final, w.Volume = ToolDilution, "10", "2", "50"
	w.handoff()
	if len(used) != 2 || used[1].Type != blocks.TypeDilutionCalculator {
		t.Fatal("dilution not handed off")
	}
	var dilution blocks.DilutionProps
	if err := used[1].DecodeProps(&dilution); err != nil {
		t.Fatal(err)
	}
	if !dilution.Valid || dilution.Result.StockVol != 10 || dilution.Result.DiluentVol != 40 {
		t.Fatalf("dilution=%+v", dilution)
	}

	w.Active, w.Rows = ToolParameters, []blocks.ParamRow{{Key: "incubation", Value: "24", Unit: "h"}}
	w.handoff()
	if len(used) != 3 || used[2].Type != blocks.TypeParameterTable {
		t.Fatal("parameter table not handed off")
	}

	w.ImportJSON = `{"id":"x","definitionId":"custom","definitionVersion":1,"type":"note","props":{"title":"Protocol","text":"Step"}}`
	if err := w.Import(); err != nil {
		t.Fatal(err)
	}
	if len(used) != 4 || used[3].ID != "x" {
		t.Fatal("valid block import not routed")
	}
	w.ImportJSON = `{"samples":[{"id":"s1"}],"wells":[{"wellId":"A1","sampleId":"s1","groupId":"control","replicate":1,"timepoint":"D1"}]}`
	if err := w.Import(); err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 5 || drafts[4].Assay == nil || drafts[4].Assay.Wells[0].WellID != "A1" {
		t.Fatal("assay draft missing")
	}
	w.ImportJSON = `{"wells":[{"wellId":"A13","sampleId":"s","groupId":"g","replicate":1,"timepoint":"D1"}]}`
	if err := w.Import(); err == nil {
		t.Fatal("invalid well accepted")
	}
}

type screenshotApp struct{ workspace *Workspace }

func (a *screenshotApp) view(c *ui.Context) { View(c, a.workspace) }

func TestToolsWorkspacePNG(t *testing.T) {
	w := New(Callbacks{})
	w.Plate.ToggleWell(0)
	w.Plate.ToggleWell(1)
	w.Plate.SetGroup(0, "Control")
	w.Plate.SetGroup(1, "Control")
	tester := ui.NewTester((&screenshotApp{workspace: w}).view, 1200, 760)
	tester.Frame()
	f, err := os.Create("evidence-tools.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tester.Image()); err != nil {
		t.Fatal(err)
	}
}
