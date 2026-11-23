package tools

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/modules/runs"
)

func TestPantryBoxesHierarchySlotPlacementMoveAndSafeDelete(t *testing.T) {
	items := []runs.PantryItem{
		{ID: "p-ab", Name: "兔抗-VEGFA 一抗", Location: "-80°C 冻存盒", Status: "opened", Revision: 1},
		{ID: "p-lig", Name: "T4 DNA 连接酶", Location: "-80°C 冻存盒", Status: "opened", Revision: 1},
		{ID: "p-strip", Name: "GAPDH 引物八联条", Location: "-20°C 冰箱", Status: "unopened", Revision: 1},
		{ID: "p-pbs", Name: "1× 无菌 PBS", Location: "4°C 冷藏架", Status: "unopened", Revision: 1},
	}
	meta := map[string]PantryItemMeta{
		"p-ab": {LotNo: "GR341902"},
	}
	boxes := []PantryBox{
		{ID: "box-cryo", Name: "抗体冻存盒 #1", Kind: BoxCryo9x9, ZoneKey: "-80", Rows: 9, Cols: 9},
		{ID: "box-pcr", Name: "qPCR 引物 96 孔盒", Kind: BoxPCR96, ZoneKey: "-20", Rows: 8, Cols: 12},
	}

	w := New(Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			return runs.PantryLedger{Items: append([]runs.PantryItem(nil), items...)}, nil
		},
		UpsertPantry: func(input runs.PantryUpsert) (runs.PantryItem, error) {
			for i := range items {
				if items[i].ID == input.ID {
					items[i].Location = input.Location
					items[i].Revision++
					return items[i], nil
				}
			}
			return runs.PantryItem{}, nil
		},
		LoadPantryMeta: func() (map[string]PantryItemMeta, error) {
			out := map[string]PantryItemMeta{}
			for k, v := range meta {
				out[k] = clonePantryMeta(v)
			}
			return out, nil
		},
		SavePantryMeta: func(m map[string]PantryItemMeta) error {
			meta = map[string]PantryItemMeta{}
			for k, v := range m {
				meta[k] = clonePantryMeta(v)
			}
			return nil
		},
		LoadPantryBoxes: func() ([]PantryBox, error) {
			return append([]PantryBox(nil), boxes...), nil
		},
		SavePantryBoxes: func(b []PantryBox) error {
			boxes = append([]PantryBox(nil), b...)
			return nil
		},
	})

	// 1. Place p-ab into C2, verify collision guard, then move C2 -> D5.
	if err := w.PlaceReagentInBoxSlot("p-ab", "box-cryo", "c2", false); err != nil {
		t.Fatalf("PlaceReagentInBoxSlot C2: %v", err)
	}
	if err := w.PlaceReagentInBoxSlot("p-lig", "box-cryo", "C2", false); err == nil {
		t.Fatalf("expected collision error placing p-lig into occupied C2")
	}
	if err := w.PlaceReagentInBoxSlot("p-lig", "box-cryo", "A3", false); err != nil {
		t.Fatalf("PlaceReagentInBoxSlot A3: %v", err)
	}
	if err := w.MoveReagentSlot("p-ab", "box-cryo", "C2", "D5"); err != nil {
		t.Fatalf("MoveReagentSlot C2->D5: %v", err)
	}
	disp := w.FormatPantryDisplayLocation(w.Pantry.Items[0])
	if !strings.Contains(disp, "抗体冻存盒 #1") || !strings.Contains(disp, "D5") {
		t.Fatalf("unexpected display location: %q", disp)
	}

	// 2. Place 8-well PCR strip across row B (B1..B8) and verify FormatSlotList range.
	if err := w.PlaceReagentInPCRStrip("p-strip", "box-pcr", 1); err != nil {
		t.Fatalf("PlaceReagentInPCRStrip: %v", err)
	}
	stripMeta := w.MetaForPantryItem(items[2])
	if got := FormatSlotList(ItemBoxSlots(stripMeta)); got != "B1–B8" {
		t.Fatalf("expected B1–B8 for PCR strip, got %q", got)
	}
	slotMapPCR := w.BoxSlotMap(boxes[1])
	if len(slotMapPCR) != 8 || slotMapPCR["B1"].ID != "p-strip" || slotMapPCR["B8"].ID != "p-strip" {
		t.Fatalf("unexpected PCR box slot map: %+v", slotMapPCR)
	}

	// 3. Search matching inside closed box highlights the box and LocatePantryItem drills right in.
	w.pantrySearch = "GR341902"
	tiers, matchSlots := w.buildShelfTiers(w.Pantry.Items)
	if matchSlots["box-cryo"] != "D5" {
		t.Fatalf("expected box-cryo match slot D5, got %q (tiers=%d)", matchSlots["box-cryo"], len(tiers))
	}
	w.LocatePantryItem("p-ab")
	if w.activeBoxID != "box-cryo" || w.selectedBoxSlot != "D5" {
		t.Fatalf("expected LocatePantryItem to open box-cryo@D5, got box=%q slot=%q", w.activeBoxID, w.selectedBoxSlot)
	}

	// 4. Create new 15 mL rack via dialog helper, then safely delete cryobox returning reagents to shelf.
	w.OpenNewBoxDialog("4C")
	w.boxDraft.Kind = BoxFalcon15
	w.selectBoxDraftPreset(BoxFalcon15)
	w.boxDraft.Name = "细胞房 15 mL 离心管架"
	if !w.SavePantryBoxDialog() {
		t.Fatalf("SavePantryBoxDialog failed: %s", w.boxDialogErr)
	}
	if len(w.PantryBoxes) != 3 {
		t.Fatalf("expected 3 boxes after save, got %d", len(w.PantryBoxes))
	}

	// Safely deleting box-cryo must NOT delete p-ab or p-lig; both must reappear loose on shelf.
	if err := w.DeletePantryBox("box-cryo"); err != nil {
		t.Fatalf("DeletePantryBox: %v", err)
	}
	if w.activeBoxID != "" || len(w.Pantry.Items) != 4 {
		t.Fatalf("expected activeBoxID cleared and all 4 reagents preserved, box=%q items=%d", w.activeBoxID, len(w.Pantry.Items))
	}
	if w.PantryMeta["p-ab"].BoxID != "" || w.PantryMeta["p-ab"].BoxSlot != "" {
		t.Fatalf("expected unboxed meta for p-ab, got %+v", w.PantryMeta["p-ab"])
	}
}

func TestPantryBoxUIDrillDownNavigationAndSelection(t *testing.T) {
	items := []runs.PantryItem{
		{ID: "p-ab", Name: "兔抗-VEGFA 一抗", Location: "-80°C", Status: "opened", Revision: 1},
		{ID: "p-pbs", Name: "1× PBS", Location: "4°C 冷藏架", Status: "unopened", Revision: 1},
	}
	meta := map[string]PantryItemMeta{
		"p-ab": {BoxID: "box-1", BoxSlot: "C2"},
	}
	boxes := []PantryBox{
		{ID: "box-1", Name: "抗体冻存盒 #1", Kind: BoxCryo9x9, ZoneKey: "-80", Rows: 9, Cols: 9},
	}
	w := New(Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			return runs.PantryLedger{Items: items}, nil
		},
		LoadPantryMeta: func() (map[string]PantryItemMeta, error) { return meta, nil },
		LoadPantryBoxes: func() ([]PantryBox, error) { return boxes, nil },
	})

	tt := ui.NewTester(func(c *ui.Context) {
		View(c, w)
	}, 1360, 920)
	tt.Frame()

	// Level 1 shelf displays the box sticker and allows clicking to drill down into Level 2.
	if err := tt.Click("打开盒子 抗体冻存盒 #1"); err != nil {
		t.Fatalf("click box sticker failed: %v", err)
	}
	tt.Frame()
	if w.activeBoxID != "box-1" || !tt.HasText("盒内试剂清单") || !tt.HasText("← 返回货架") {
		t.Fatalf("expected Level 2 box interior view, activeBox=%q texts=%q", w.activeBoxID, tt.Texts())
	}

	// Clicking an empty slot E4 selects E4 in the inspector.
	if err := tt.Click("孔位 E4"); err != nil {
		t.Fatalf("click slot E4 failed: %v", err)
	}
	tt.Frame()
	if w.selectedBoxSlot != "E4" || !tt.HasText("+ 在 E4 新建试剂") {
		t.Fatalf("expected empty slot inspector for E4, selected=%q", w.selectedBoxSlot)
	}

	// Clicking back returns cleanly to Level 1 shelves.
	if err := tt.Click("← 返回货架"); err != nil {
		t.Fatalf("click back to shelf failed: %v", err)
	}
	tt.Frame()
	if w.activeBoxID != "" || !tt.HasText("+ 收纳盒") {
		t.Fatalf("expected return to Level 1 shelf, activeBox=%q", w.activeBoxID)
	}
}
