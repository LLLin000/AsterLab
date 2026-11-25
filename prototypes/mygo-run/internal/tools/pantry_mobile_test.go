package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func newMobilePantryTestWorkspace() *Workspace {
	items := []runs.PantryItem{
		{ID: "p-ab", Name: "兔抗-VEGFA 一抗", Location: "-80°C", Status: "opened", Revision: 1},
		{ID: "p-taq", Name: "SYBR Green 预混液", Location: "-20°C 酶盒", Status: "near_expiry", ExpiresOn: "2026-12-01", Revision: 1},
		{ID: "p-pbs", Name: "1× 无菌 PBS", Location: "4°C 冷藏架", Status: "unopened", Revision: 1},
		{ID: "p-dmso", Name: "无水 DMSO", Location: "常温避光", Status: "unopened", Revision: 1},
	}
	meta := map[string]PantryItemMeta{
		"p-ab": {BoxID: "box-cryo", BoxSlot: "C2", LotNo: "GR341902"},
	}
	boxes := []PantryBox{
		{ID: "box-cryo", Name: "抗体冻存盒 #1", Kind: BoxCryo9x9, ZoneKey: "-80", Rows: 9, Cols: 9},
		{ID: "box-ep", Name: "酶管 EP 盒", Kind: BoxEP50, ZoneKey: "-20", Rows: 5, Cols: 10},
	}

	return New(Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			return runs.PantryLedger{Items: append([]runs.PantryItem(nil), items...)}, nil
		},
		UpsertPantry: func(input runs.PantryUpsert) (runs.PantryItem, error) {
			for i := range items {
				if items[i].ID == input.ID {
					items[i].Name = input.Name
					items[i].Location = input.Location
					items[i].Revision++
					return items[i], nil
				}
			}
			ni := runs.PantryItem{
				ID:       "p-new",
				Name:     input.Name,
				Location: input.Location,
				Status:   "unopened",
				Revision: 1,
			}
			items = append(items, ni)
			return ni, nil
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
}

func TestMobilePantryZoneChipsAndTouchCardList(t *testing.T) {
	w := newMobilePantryTestWorkspace()
	tt := ui.NewTester(func(c *ui.Context) {
		ViewPantry(c, w)
	}, 390, 844)
	tt.Frame()

	// Horizontal temperature-zone chips should be present with counts.
	if titleRect, ok := tt.Find("储藏室"); !ok || titleRect.W < 36 {
		t.Fatalf("expected untruncated '储藏室' title, ok=%v rect=%+v", ok, titleRect)
	}
	for _, want := range []string{"全部 (4)", "-80°C (1)", "-20°C (1)", "4°C (1)", "常温避光 (1)", "临期 (1)"} {
		if !tt.HasText(want) {
			t.Fatalf("mobile pantry missing zone chip %q in %v", want, tt.Texts())
		}
	}

	// Touch card rows must be >= 48dp tall.
	boxCard, ok := tt.Find("打开盒子 抗体冻存盒 #1")
	if !ok {
		t.Fatalf("expected mobile cryo-box touch card, texts=%v", tt.Texts())
	}
	if boxCard.H < 48 {
		t.Fatalf("expected touch card height >= 48dp, got %.1f", boxCard.H)
	}
	reagentCard, ok := tt.Find("1× 无菌 PBS")
	if !ok {
		t.Fatalf("expected mobile reagent touch card, texts=%v", tt.Texts())
	}
	if reagentCard.H < 48 {
		t.Fatalf("expected reagent touch card height >= 48dp, got %.1f", reagentCard.H)
	}

	// Filter to -80°C zone.
	if err := tt.Click("-80°C (1)"); err != nil {
		t.Fatalf("click -80°C chip: %v", err)
	}
	tt.Frame()
	if w.pantryZoneFilter != "-80" {
		t.Fatalf("expected pantryZoneFilter=-80, got %q", w.pantryZoneFilter)
	}
	if !tt.HasText("抗体冻存盒 #1") || tt.HasText("1× 无菌 PBS") {
		t.Fatalf("zone -80 filter failed: texts=%v", tt.Texts())
	}

	// Scroll horizontal temperature-zone chips to reveal and filter by 临期.
	if chipAll, ok := tt.Find("全部 (4)"); ok {
		tt.Scroll(chipAll.X+chipAll.W/2, chipAll.Y+chipAll.H/2, 240, 0)
		tt.Frame()
	}
	if err := tt.Click("临期 (1)"); err != nil {
		t.Fatalf("click 临期 chip: %v", err)
	}
	tt.Frame()
	if w.pantryZoneFilter != "near_expiry" {
		t.Fatalf("expected pantryZoneFilter=near_expiry, got %q", w.pantryZoneFilter)
	}
	if !tt.HasText("SYBR Green 预混液") || tt.HasText("无水 DMSO") {
		t.Fatalf("zone near_expiry filter failed: texts=%v", tt.Texts())
	}

	// Scroll back left and reset to 全部.
	if chipNear, ok := tt.Find("临期 (1)"); ok {
		tt.Scroll(chipNear.X+chipNear.W/2, chipNear.Y+chipNear.H/2, -240, 0)
		tt.Frame()
	}
	if err := tt.Click("全部 (4)"); err != nil {
		t.Fatalf("click 全部 chip: %v", err)
	}
	tt.Frame()
	if !tt.HasText("1× 无菌 PBS") || !tt.HasText("无水 DMSO") {
		t.Fatalf("expected all items after clicking 全部, texts=%v", tt.Texts())
	}
}

func settleMobileSheet(tt *ui.Tester) {
	for range 120 {
		tt.Advance(16 * time.Millisecond)
	}
	tt.Frame()
}

func TestMobilePantryCryoBoxFullscreenZoomableSheet(t *testing.T) {
	w := newMobilePantryTestWorkspace()
	tt := ui.NewTester(func(c *ui.Context) {
		ViewPantry(c, w)
	}, 390, 844)
	tt.Frame()

	// Tap the 9x9 cryo-box touch card to open the fullscreen Zoomable Sheet.
	if err := tt.Click("打开盒子 抗体冻存盒 #1"); err != nil {
		t.Fatalf("click cryo-box card: %v", err)
	}
	tt.Frame()
	settleMobileSheet(tt)

	if w.activeBoxID != "box-cryo" {
		t.Fatalf("expected activeBoxID=box-cryo, got %q", w.activeBoxID)
	}
	for _, want := range []string{"抗体冻存盒 #1", "关闭", "孔位 C2", "兔抗-VEGFA 一抗"} {
		if !tt.HasText(want) {
			t.Fatalf("cryo-box sheet missing %q in %v", want, tt.Texts())
		}
	}

	// Select an empty slot E5 inside the Zoomable 9x9 grid.
	if err := tt.Click("孔位 E5"); err != nil {
		t.Fatalf("click slot E5 inside Zoomable sheet: %v", err)
	}
	tt.Frame()
	if w.selectedBoxSlot != "E5" {
		t.Fatalf("expected selectedBoxSlot=E5, got %q", w.selectedBoxSlot)
	}
	addBtn, ok := tt.Find("+ 在 E5 新建试剂")
	if !ok {
		t.Fatalf("expected '+ 在 E5 新建试剂' button in sheet inspector, texts=%v", tt.Texts())
	}
	if addBtn.H < 48 {
		t.Fatalf("expected sheet CTA button height >= 48dp, got %.1f", addBtn.H)
	}

	// Zoom controls inside sheet.
	if err := tt.Click("放大冻存盒"); err != nil {
		t.Fatalf("click zoom in: %v", err)
	}
	tt.Frame()
	if !tt.HasText("150%") {
		t.Fatalf("expected 150%% zoom indicator, texts=%v", tt.Texts())
	}
	if err := tt.Click("重置"); err != nil {
		t.Fatalf("click zoom reset: %v", err)
	}
	tt.Frame()
	if !tt.HasText("100%") {
		t.Fatalf("expected 100%% zoom indicator after reset, texts=%v", tt.Texts())
	}

	// Close the sheet and return cleanly to the mobile shelf.
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("click 关闭: %v", err)
	}
	tt.Frame()
	settleMobileSheet(tt)
	if w.activeBoxID != "" {
		t.Fatalf("expected activeBoxID cleared after closing sheet, got %q", w.activeBoxID)
	}
	if !tt.HasText("全部 (4)") {
		t.Fatalf("expected mobile shelf restored after closing sheet, texts=%v", tt.Texts())
	}
}

func TestMobilePlateStudioCompactAndSheetAnnotation(t *testing.T) {
	w := newMobilePantryTestWorkspace()
	w.Active = ToolPlate

	tt := ui.NewTester(func(c *ui.Context) {
		tok := theme.GetTokens(c.Theme())
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8)).Children(func() {
				plateView(c, w, tok)
			})
		})
	}, 390, 844)
	tt.Frame()

	// Mobile Plate Studio renders inline compact summary card for 96-well plate.
	if !tt.HasText("96 孔板 · 8×12") || !tt.HasText("全屏排板 / 读数") {
		t.Fatalf("expected compact summary card in mobile Plate Studio, texts=%v", tt.Texts())
	}

	// Open fullscreen sheet, tap A1, and mark 空白(PBS).
	if err := tt.Click("全屏排板 / 读数"); err != nil {
		t.Fatalf("open mobile plate studio sheet: %v", err)
	}
	tt.Frame()
	settleMobileSheet(tt)

	if err := tt.Click("孔 A1"); err != nil {
		t.Fatalf("tap A1 in plate studio sheet: %v", err)
	}
	tt.Frame()
	if err := tt.Click("空白(PBS)"); err != nil {
		t.Fatalf("tap 空白(PBS) in plate studio sheet: %v", err)
	}
	tt.Frame()

	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil || len(plate.WellGroups()) != 1 || !strings.EqualFold(plate.Roles["A1"], "blank") {
		t.Fatalf("expected A1 annotated as blank in plateDoc, got plate=%+v", plate)
	}

	if err := tt.Click("完成"); err != nil {
		t.Fatalf("close plate studio sheet: %v", err)
	}
	tt.Frame()
	settleMobileSheet(tt)
	if !tt.HasText("已排 1 / 96 孔") {
		t.Fatalf("expected updated summary count 1/96 after closing sheet, texts=%v", tt.Texts())
	}
}
