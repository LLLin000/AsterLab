package tools

import (
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/blocks"
	"github.com/egoist/mygo/ui"
	"errors"
	"asterlab-mygo-prototype/modules/runs"
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

func TestPantryLedgerCRUDAndExpiryGroups(t *testing.T) {
	var items []runs.PantryItem
	nextID := 0
	w := New(Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			return runs.PantryLedger{Items: append([]runs.PantryItem(nil), items...)}, nil
		},
		UpsertPantry: func(input runs.PantryUpsert) (runs.PantryItem, error) {
			for i := range items {
				if items[i].ID == input.ID {
					if input.ExpectedRevision == nil || *input.ExpectedRevision != items[i].Revision {
						t.Fatalf("update missing revision guard: %+v", input)
					}
					items[i].Name, items[i].Location = input.Name, input.Location
					items[i].Revision++
					return items[i], nil
				}
			}
			nextID++
			item := runs.PantryItem{ID: "item", Name: input.Name, Location: input.Location, Revision: int64(nextID), Status: "unopened"}
			items = append(items, item)
			return item, nil
		},
		DeletePantry: func(input runs.PantryDelete) error {
			for i, item := range items {
				if item.ID == input.ID {
					if input.ExpectedRevision == nil || *input.ExpectedRevision != item.Revision {
						t.Fatalf("delete missing revision guard: %+v", input)
					}
					items = append(items[:i], items[i+1:]...)
					return nil
				}
			}
			return errors.New("not found")
		},
	})
	if !w.PantryLoaded || len(w.Pantry.Items) != 0 {
		t.Fatalf("initial read: loaded=%v items=%+v", w.PantryLoaded, w.Pantry.Items)
	}
	w.PantryName, w.PantryLocation = "PBS", "冷藏柜"
	w.SavePantry()
	if len(w.Pantry.Items) != 1 || w.Pantry.Items[0].Name != "PBS" || w.Pantry.Items[0].ID != "item" {
		t.Fatalf("create/read: %+v", w.Pantry.Items)
	}
	w.EditPantry(w.Pantry.Items[0])
	w.PantryName = "PBS 1x"
	w.SavePantry()
	if len(w.Pantry.Items) != 1 || w.Pantry.Items[0].Name != "PBS 1x" || w.Pantry.Items[0].Revision != 2 {
		t.Fatalf("update/read: %+v", w.Pantry.Items)
	}
	w.DeletePantry(w.Pantry.Items[0])
	if len(w.Pantry.Items) != 0 || len(items) != 0 {
		t.Fatalf("delete/read: workspace=%+v service=%+v", w.Pantry.Items, items)
	}

	near, other := groupPantryItems([]runs.PantryItem{
		{ID: "soon", Status: "near_expiry"},
		{ID: "fresh", Status: "opened"},
		{ID: "old", Status: "expired"},
	})
	if len(near) != 1 || near[0].ID != "soon" || len(other) != 2 {
		t.Fatalf("expiry groups: near=%+v other=%+v", near, other)
	}
}

func TestPantryExportOnlyReportsCoreSuccess(t *testing.T) {
	fail := true
	w := New(Callbacks{
		ExportPantry: func() (runs.PantryLabels, error) {
			if fail {
				return runs.PantryLabels{}, errors.New("core unavailable")
			}
			return runs.PantryLabels{Labels: []runs.PantryLabel{{LabelCode: "P-1", Name: "PBS"}}}, nil
		},
	})
	w.ExportPantryLabels()
	if w.Status != "" || w.Error != "core unavailable" || w.PantryRetry == nil {
		t.Fatalf("failed export state: status=%q error=%q retry=%v", w.Status, w.Error, w.PantryRetry != nil)
	}
	fail = false
	w.PantryRetry()
	if w.Error != "" || w.Status != "贴标清单已就绪 · 1 项" || w.PantryExportJSON == "" {
		t.Fatalf("successful retry state: status=%q error=%q json=%q", w.Status, w.Error, w.PantryExportJSON)
	}
}

// UX-P5: import/handoff failures speak §3 vocabulary with cause; success
// states carry no English strings.
func TestImportFailureChineseCause(t *testing.T) {
	w := New(Callbacks{})
	w.ImportJSON = ""
	if err := w.Import(); err == nil || err.Error() != "请粘贴 Typed block 实例或 assay 设计 JSON" {
		t.Fatalf("empty import: %v", err)
	}
	w.ImportJSON = "not-json{{{"
	if err := w.Import(); err == nil || len(err.Error()) < len("JSON 无效") || err.Error()[:len("JSON 无效")] != "JSON 无效" {
		t.Fatalf("invalid JSON: %v", err)
	}
	w.ImportJSON = `{"wells":[]}`
	if err := w.Import(); err == nil || err.Error() != "assay 设计至少需要一个孔位分配" {
		t.Fatalf("empty wells: %v", err)
	}
	w.Active = ToolImport
	w.handoff()
	if w.Error != "请先选择一个草稿再导入" {
		t.Fatalf("handoff default: %q", w.Error)
	}
	w.ImportJSON = `{"id":"x","definitionId":"custom","definitionVersion":1,"type":"note","props":{"title":"T","text":"S"}}`
	if err := w.Import(); err != nil {
		t.Fatal(err)
	}
	if w.Error != "" || w.Status != "导入成功" {
		t.Fatalf("import success state: err=%q status=%q", w.Error, w.Status)
	}
}
