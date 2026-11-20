package tools

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
)

func TestToolsFourSectionNavSOPLibraryAndBenchCalc(t *testing.T) {
	var createdTitle, createdSource string
	var insertedBlock map[string]any

	w := New(Callbacks{
		CreateDesignFromSOP: func(title, source string) {
			createdTitle, createdSource = title, source
		},
		InsertAsterBlock: func(block map[string]any) {
			insertedBlock = block
		},
	})

	tt := ui.NewTester(func(c *ui.Context) {
		View(c, w)
	}, 1280, 820)
	tt.Frame()

	for _, want := range []string{"储藏室货架", "标准方案库", "孔板速排", "实验台速算"} {
		if !tt.HasText(want) {
			t.Fatalf("missing tool section %q in %v", want, tt.Texts())
		}
	}
	for _, legacy := range []string{"实验参数表", "通用模块模板", "结构化数据导入"} {
		if tt.HasText(legacy) {
			t.Fatalf("legacy retired tool %q still rendered", legacy)
		}
	}
	for _, p := range StandardPlatePresets {
		if p.Key == "dish-4" || p.Topology == "dish" {
			t.Fatalf("dish-4 must not remain in StandardPlatePresets: %+v", p)
		}
	}

	// Switch to 标准方案库 and clone an SOP into Design.
	if err := tt.Click("标准方案库"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("以此新建实验设计") || !tt.HasText("RT-qPCR 多靶点相对定量标准方案") {
		t.Fatalf("SOP library missing document preview or clone action: %v", tt.Texts())
	}
	if err := tt.Click("以此新建实验设计"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if createdTitle == "" || !strings.Contains(createdSource, "kind: plate") {
		t.Fatalf("CreateDesignFromSOP callback not invoked properly: title=%q", createdTitle)
	}

	// Verify 4-in-1 bench calculator precision (pure Go unit logic + block insert).
	massMg, _, warn, err := ComputeMolarStock(10, "mM", 5, "mL", 1449.25)
	if err != nil || math.Abs(massMg-72.4625) > 1e-3 || warn != "" {
		t.Fatalf("ComputeMolarStock = (%v, %q, %v)", massMg, warn, err)
	}
	_, _, lowWarn, err := ComputeMolarStock(1, "uM", 1, "mL", 500)
	if err != nil || !strings.Contains(lowWarn, "不足 1 mg") {
		t.Fatalf("expected low weighing warning <1 mg, got (%q, %v)", lowWarn, err)
	}
	v1, dil, _, err := ComputeUnitDilution(100, "ug/mL", 50, "ng/mL", 4, "mL")
	if err != nil || math.Abs(v1-0.002) > 1e-6 || math.Abs(dil-3.998) > 1e-6 {
		t.Fatalf("ComputeUnitDilution = (%v, %v, %v)", v1, dil, err)
	}
	if _, _, _, err := ComputeUnitDilution(10, "mM", 50, "ng/mL", 4, "mL"); err == nil {
		t.Fatal("expected dimension mismatch error between mM and ng/mL")
	}
	susp, media, total, err := ComputeCellSeeding(120, "×10⁴/mL", 15000, 12, 500, 15)
	if err != nil || math.Abs(total-6.9) > 1e-3 || math.Abs(susp-0.1725) > 1e-3 || math.Abs(media-6.7275) > 1e-3 {
		t.Fatalf("ComputeCellSeeding = (%v, %v, %v, %v)", susp, media, total, err)
	}
	rows, mixUL, singleUL, err := ComputeMasterMix(24, 15, []MasterMixComponent{
		{Name: "SYBR", PerWellUL: "10"},
		{Name: "Primer", PerWellUL: "1.6"},
		{Name: "H2O", PerWellUL: "6.4"},
		{Name: "cDNA", PerWellUL: "2.0", IsTemplate: true},
	})
	if err != nil || len(rows) != 4 || math.Abs(singleUL-20) > 1e-6 || math.Abs(mixUL-496.8) > 1e-3 {
		t.Fatalf("ComputeMasterMix = (%+v, mix=%v, single=%v, err=%v)", rows, mixUL, singleUL, err)
	}

	// Verify BenchCalc block insert button in UI triggers InsertAsterBlock.
	if err := tt.Click("实验台速算"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("插入配液表到设计"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if insertedBlock == nil || insertedBlock["kind"] != "param_table" {
		t.Fatalf("expected param_table inserted from molar bench calc, got %#v", insertedBlock)
	}

	// Keep unused legacy Draft test import happy.
	_ = blocks.DilutionProps{}
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
	f, err := os.Create(filepath.Join(t.TempDir(), "evidence-tools.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tester.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestPlateStudioWorkflowHistoryAndExport(t *testing.T) {
	var persisted []PlateHistoryEntry
	var exportedFiles = map[string][]byte{}
	var handedOffSource string

	w := New(Callbacks{
		SavePlateHistory: func(entries []PlateHistoryEntry) error {
			persisted = append([]PlateHistoryEntry(nil), entries...)
			return nil
		},
		SavePlateFile: func(defaultName, format string, data []byte) (string, error) {
			p := filepath.Join(t.TempDir(), defaultName)
			exportedFiles[format] = append([]byte(nil), data...)
			return p, nil
		},
		UseAsterPlate: func(source, plateID string) {
			handedOffSource = source
		},
	})
	w.Active = ToolPlate

	// Step ① -> Choose 12-well format, select wells & annotate via unified single input.
	w.SelectPlatePreset("plate-12")
	w.SelectPlateWells([]string{"A1", "A2", "A3"})
	if err := w.AnnotateSelectedWells("VEGF-50"); err != nil {
		t.Fatalf("AnnotateSelectedWells group/sample: %v", err)
	}
	w.SelectPlateWells([]string{"A1", "A2", "A3"})
	if err := w.AnnotateSelectedWells("靶点:VEGFA"); err != nil {
		t.Fatalf("AnnotateSelectedWells factor: %v", err)
	}
	w.SelectPlateWells([]string{"C4"})
	if err := w.AnnotateSelectedWells("NTC"); err != nil {
		t.Fatalf("AnnotateSelectedWells qc role: %v", err)
	}

	// Verify AsterML document holds 12-well geometry + replicates 1..3 + factor + QC role.
	doc12, err := asterdoc.Parse(w.plateDoc.Source)
	if err != nil {
		t.Fatalf("parse 12-well source: %v", err)
	}
	p12 := findPrimaryPlate(doc12)
	if p12 == nil || p12.Format != 12 {
		t.Fatalf("expected 12-well plate, got %+v", p12)
	}
	if p12.Replicates["A1"] != 1 || p12.Replicates["A2"] != 2 || p12.Replicates["A3"] != 3 {
		t.Fatalf("expected automatic triplicates 1..3, got %+v", p12.Replicates)
	}
	if p12.Roles["C4"] != "ntc" {
		t.Fatalf("expected C4 role=ntc, got %+v", p12.Roles)
	}

	// Switching preset automatically archives the populated 12-well plate to history.
	w.SelectPlatePreset("rack-8")
	if len(w.PlateHistory) != 1 || len(persisted) != 1 {
		t.Fatalf("expected auto-archived history entry, got history=%d persisted=%d", len(w.PlateHistory), len(persisted))
	}
	w.SelectPlateWells([]string{"A1", "A2"})
	if err := w.AnnotateSelectedWells("Vancomycin"); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.SaveCurrentPlateToHistory(); !ok {
		t.Fatalf("expected explicit save of 8-strip to history")
	}

	// Add a 3rd 96-well entry to verify odd trailing row in 2-column history layout.
	w.SelectPlatePreset("plate-96")
	w.SelectPlateWells([]string{"B1", "B2", "B3"})
	if err := w.AnnotateSelectedWells("Ctrl"); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.SaveCurrentPlateToHistory(); !ok {
		t.Fatalf("expected save of 96-well plate to history")
	}
	if len(w.PlateHistory) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(w.PlateHistory))
	}

	// Select A1..A3 so floating bar renders inside View and mount in strict ui.NewTester.
	w.SelectPlateWells([]string{"A1", "A2"})
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, w)
	}, 1360, 960)
	tt.Frame()

	// Export SVG & PNG directly from the active UI frame (ensures nested NewTester works mid-click).
	if svgBtn, ok := tt.Find("导出 SVG"); !ok {
		t.Fatalf("missing 导出 SVG button")
	} else {
		tt.ClickAt(svgBtn.X+svgBtn.W/2, svgBtn.Y+svgBtn.H/2)
		tt.Frame()
	}
	if err := xml.Unmarshal(exportedFiles["svg"], new(any)); err != nil {
		t.Fatalf("invalid exported SVG XML: %v", err)
	}
	if pngBtn, ok := tt.Find("导出 PNG"); !ok {
		t.Fatalf("missing 导出 PNG button")
	} else {
		tt.ClickAt(pngBtn.X+pngBtn.W/2, pngBtn.Y+pngBtn.H/2)
		tt.Frame()
	}
	if len(exportedFiles["png"]) == 0 {
		t.Fatalf("expected exported PNG bytes")
	}
	if img, err := png.Decode(bytes.NewReader(exportedFiles["png"])); err != nil || img.Bounds().Dx() < 800 {
		t.Fatalf("invalid exported PNG: err=%v bounds=%v", err, img.Bounds())
	}

	// Load the earlier 12-well history card back to top designer & hand off to Design.
	hist12ID := w.PlateHistory[2].ID
	if !w.LoadHistoryEntry(hist12ID) {
		t.Fatalf("LoadHistoryEntry failed for %s", hist12ID)
	}
	loaded := findPrimaryPlate(w.plateDoc)
	if loaded == nil || loaded.Format != 12 || loaded.Samples["A2"] != "VEGF-50" {
		t.Fatalf("expected loaded 12-well plate restored to designer, got %+v", loaded)
	}
	tt.Frame()

	// Verify clicking the top history card's miniature preview in UI clicks through to LoadHistoryEntry.
	ttLarge := ui.NewTester(func(c *ui.Context) {
		View(c, w)
	}, 1360, 1280)
	ttLarge.Frame()
	if loadBtn, ok := ttLarge.Find("载入编辑"); !ok {
		t.Fatalf("expected at least one 载入编辑 button on 2-column history card")
	} else {
		ttLarge.ClickAt(loadBtn.X+loadBtn.W/2, loadBtn.Y+loadBtn.H/2)
		ttLarge.Frame()
	}

	w.LoadHistoryEntry(hist12ID)
	w.handoff()
	if handedOffSource == "" || !strings.Contains(handedOffSource, "VEGFA") {
		t.Fatalf("expected UseAsterPlate handoff containing VEGFA plate, got %q", handedOffSource)
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

func TestPantryStickerShelfDetailManualUploadAndURL(t *testing.T) {
	dir := t.TempDir()
	srcPDF := filepath.Join(dir, "SYBR_Green_Manual.pdf")
	if err := os.WriteFile(srcPDF, []byte("%PDF-1.4\n% mock reagent manual\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := []runs.PantryItem{
		{ID: "p-sybr", Name: "SYBR Green 预混液", Location: "-20°C 酶盒 A2", Status: "near_expiry", PurchasedOn: "2026-06-01", OpenedOn: "2026-10-20", ExpiresOn: "2026-11-28", Revision: 2},
		{ID: "p-dmem", Name: "DMEM 高糖培养基", Location: "4°C 冷藏架", Status: "opened", PurchasedOn: "2026-10-12", OpenedOn: "2026-11-01", Revision: 1},
	}
	storedMeta := map[string]PantryItemMeta{}
	manualsDir := filepath.Join(dir, "manuals")
	var openedLocal string
	var openedURL string

	newWorkspace := func() *Workspace {
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
				return runs.PantryItem{}, errors.New("missing")
			},
			LoadPantryMeta: func() (map[string]PantryItemMeta, error) {
				out := map[string]PantryItemMeta{}
				for k, v := range storedMeta {
					out[k] = clonePantryMeta(v)
				}
				return out, nil
			},
			SavePantryMeta: func(m map[string]PantryItemMeta) error {
				storedMeta = map[string]PantryItemMeta{}
				for k, v := range m {
					storedMeta[k] = clonePantryMeta(v)
				}
				return nil
			},
			PickManualFiles: func() ([]string, error) {
				return []string{srcPDF}, nil
			},
			ImportPantryFile: func(itemKey, srcPath string) (string, string, int64, error) {
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return "", "", 0, err
				}
				rel := filepath.ToSlash(filepath.Join(itemKey, filepath.Base(srcPath)))
				dst := filepath.Join(manualsDir, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
					return "", "", 0, err
				}
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					return "", "", 0, err
				}
				return rel, filepath.Base(srcPath), int64(len(data)), nil
			},
			ResolvePantryFile: func(storedRel string) string {
				return filepath.Join(manualsDir, filepath.FromSlash(storedRel))
			},
			OpenLocalFile: func(absPath string) error {
				openedLocal = absPath
				return nil
			},
			OpenExternalURL: func(rawURL string) error {
				openedURL = rawURL
				return nil
			},
		})
	}

	w := newWorkspace()
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, w)
	}, 1360, 960)
	tt.Frame()

	// Clicking the reagent sticker opens its detail modal directly.
	if btn, ok := tt.Find("SYBR Green 预混液"); !ok {
		t.Fatalf("expected SYBR Green sticker on pantry shelf, texts=%v", tt.Texts())
	} else {
		tt.ClickAt(btn.X+btn.W/2, btn.Y+btn.H/2)
		tt.Frame()
	}
	if !w.PantryDialogOpen || w.PantryID != "p-sybr" {
		t.Fatalf("expected reagent detail dialog to open for p-sybr, open=%v id=%q", w.PantryDialogOpen, w.PantryID)
	}

	// Upload datasheet PDF via button click.
	if upBtn, ok := tt.Find("上传说明书"); !ok {
		t.Fatalf("missing 上传说明书 button in detail dialog")
	} else {
		tt.ClickAt(upBtn.X+upBtn.W/2, upBtn.Y+upBtn.H/2)
		tt.Frame()
	}
	if len(w.pantryDraftMeta.Manuals) != 1 || w.pantryDraftMeta.Manuals[0].FileName != "SYBR_Green_Manual.pdf" {
		t.Fatalf("expected imported PDF manual in draft, got %+v", w.pantryDraftMeta.Manuals)
	}

	// Click 查阅 to open the copied local manual through OpenLocalFile callback.
	if viewBtn, ok := tt.Find("查阅"); !ok {
		t.Fatalf("missing 查阅 button for uploaded manual")
	} else {
		tt.ClickAt(viewBtn.X+viewBtn.W/2, viewBtn.Y+viewBtn.H/2)
		tt.Frame()
	}
	if !strings.HasSuffix(filepath.ToSlash(openedLocal), "p-sybr/SYBR_Green_Manual.pdf") {
		t.Fatalf("expected OpenLocalFile with copied manual path, got %q", openedLocal)
	}

	// Reject dangerous / invalid URL schemes cleanly.
	w.pantryDraftMeta.ProductURL = "javascript:alert(1)"
	if w.SavePantryDetail() {
		t.Fatalf("expected javascript: URL to be rejected")
	}
	if !strings.Contains(w.pantryDialogErr, "http://") {
		t.Fatalf("expected Chinese URL scheme error, got %q", w.pantryDialogErr)
	}

	// Normalize & open valid vendor product URL, then save and verify persistence on reload.
	w.pantryDraftMeta.ProductURL = "www.thermofisher.com/order/catalog/product/A25742"
	tt.Frame()
	if openURLBtn, ok := tt.Find("打开网页"); !ok {
		t.Fatalf("missing 打开网页 button in detail dialog")
	} else {
		tt.ClickAt(openURLBtn.X+openURLBtn.W/2, openURLBtn.Y+openURLBtn.H/2)
		tt.Frame()
	}
	if openedURL != "https://www.thermofisher.com/order/catalog/product/A25742" {
		t.Fatalf("expected normalized https URL opened, got %q (err=%q)", openedURL, w.pantryDialogErr)
	}

	w.pantryDraftMeta.CatalogNo = "Thermo A25742"
	w.pantryDraftMeta.LotNo = "0128945"
	if !w.SavePantryDetail() {
		t.Fatalf("SavePantryDetail failed: %q", w.pantryDialogErr)
	}
	if w.PantryDialogOpen {
		t.Fatalf("expected dialog to close after save")
	}

	reloaded := newWorkspace()
	gotMeta := reloaded.MetaForPantryItem(items[0])
	if gotMeta.ProductURL != "https://www.thermofisher.com/order/catalog/product/A25742" ||
		gotMeta.CatalogNo != "Thermo A25742" ||
		len(gotMeta.Manuals) != 1 ||
		gotMeta.Manuals[0].FileName != "SYBR_Green_Manual.pdf" {
		t.Fatalf("expected persisted manual & URL after reload, got %+v", gotMeta)
	}
}
