package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/crashguard"
	"asterlab-mygo-prototype/internal/labio"
	"asterlab-mygo-prototype/internal/recovery"
	"asterlab-mygo-prototype/internal/schedule"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/syncengine"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/vault"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

// TestM3_LargeVault500Exp1000ReagentScale generates a realistic 500-experiment,
// 1,000-reagent Vault and verifies scan, index, search, disk-signature, and
// pantry loading stay well within CI-safe latency budgets with zero errors.
func TestM3_LargeVault500Exp1000ReagentScale(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "Scale500Vault")
	sysDir := filepath.Join(vaultRoot, vault.SystemDirName)
	if err := os.MkdirAll(sysDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const totalExps = 500
	const totalReagents = 1000
	const targetIdx = 387
	const targetAnchor = "M3大规模检索锚点-CRISPR-Cas9-387"

	for i := 0; i < totalExps; i++ {
		folder := filepath.Join(vaultRoot, fmt.Sprintf("Project-%02d", i%20))
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		var doc string
		switch {
		case i == targetIdx:
			doc = strings.Replace(fsmTestDocAlpha, "Van-Lys 液体共培养涂板计数计划", targetAnchor, 1)
		case i%3 == 0:
			doc = strings.Replace(fsmTestDocAlpha, "Van-Lys 液体共培养涂板计数计划", fmt.Sprintf("共培养杀菌实验 #%03d", i), 1)
		case i%3 == 1:
			doc = strings.Replace(fsmTestDocBeta, "Van-Lys 滤纸片扩散抑菌圈计划", fmt.Sprintf("滤纸片抑菌圈实验 #%03d", i), 1)
		default:
			doc = fmt.Sprintf("---\ntitle: 细胞增殖测定 #%03d\nscheduled_date: 2026-11-%02d\nstage: planned\n---\n\n```asterlab\nkind: experiment\nid: exp-%03d\nschema: 1\ntitle: 细胞增殖测定 #%03d\nobjective: 评估药物梯度对 L929 细胞活力的影响。\nexperimental_unit: well\n```\n\n#@ 01 铺板与加药\n\n- 每孔接种 5000 cells，37°C 孵育 24 h。\n",
				i, (i%28)+1, i, i)
		}
		fpath := filepath.Join(folder, fmt.Sprintf("exp-%03d.asterml", i))
		if err := os.WriteFile(fpath, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var pantryBuf bytes.Buffer
	pantryBuf.WriteString(`{"type":"schema","version":1}` + "\n")
	locations := []string{"-80°C", "-20°C 酶盒", "4°C 冷藏架", "常温避光"}
	zoneKeys := []string{"-80", "-20", "4", "rt"}
	for b := 0; b < 10; b++ {
		bx := pantryBoxRecord{
			Type:    "box",
			ID:      fmt.Sprintf("box-%02d", b),
			Name:    fmt.Sprintf("冻存盒 #%02d", b+1),
			Kind:    tools.BoxCryo9x9,
			ZoneKey: zoneKeys[b%len(zoneKeys)],
			Rows:    9,
			Cols:    9,
		}
		line, err := json.Marshal(bx)
		if err != nil {
			t.Fatal(err)
		}
		pantryBuf.Write(line)
		pantryBuf.WriteByte('\n')
	}
	for i := 0; i < totalReagents; i++ {
		shelf := uint32(180 + (i%4)*90)
		rec := pantryReagentRecord{
			Type:          "reagent",
			ID:            fmt.Sprintf("r-%04d", i),
			LabelCode:     fmt.Sprintf("R-%04d", i),
			Name:          fmt.Sprintf("试剂样本-%04d-%s", i, locations[i%len(locations)]),
			Location:      locations[i%len(locations)],
			PurchasedOn:   "2026-10-15",
			ShelfLifeDays: &shelf,
			Revision:      1,
			CreatedAt:     "2026-10-15T08:00:00Z",
			UpdatedAt:     "2026-11-20T08:00:00Z",
			Meta: &tools.PantryItemMeta{
				BoxID:   fmt.Sprintf("box-%02d", i%10),
				BoxSlot: fmt.Sprintf("%c%d", 'A'+(i/9)%9, (i%9)+1),
				LotNo:   fmt.Sprintf("LOT-2026-%04d", i),
			},
		}
		if i == 777 {
			rec.Name = "目标抗体-Anti-VEGFA-777"
		}
		line, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		pantryBuf.Write(line)
		pantryBuf.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(sysDir, pantryJSONLFileName), pantryBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	v, err := vault.Open(vaultRoot)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}

	t0 := time.Now()
	_, files, err := v.ScanTree()
	scanDur := time.Since(t0)
	if err != nil {
		t.Fatalf("ScanTree(500): %v", err)
	}
	if len(files) < totalExps {
		t.Fatalf("expected >= %d files scanned, got %d", totalExps, len(files))
	}

	idx := vault.NewVaultIndex(v)
	t1 := time.Now()
	if err := idx.Refresh(); err != nil {
		t.Fatalf("VaultIndex.Refresh(500): %v", err)
	}
	indexDur := time.Since(t1)

	t2 := time.Now()
	hits := idx.SearchFullText(targetAnchor, 20)
	searchDur := time.Since(t2)
	if len(hits) == 0 || !strings.Contains(hits[0].RelPath, "exp-387.asterml") {
		t.Fatalf("expected search hit for %s, got %+v", targetAnchor, hits)
	}

	t3 := time.Now()
	sig1, pantrySig := computeVaultDiskSignatures(vaultRoot)
	sigDur := time.Since(t3)
	if sig1 == "" || pantrySig == "" {
		t.Fatalf("expected non-empty disk signatures, got vault=%q pantry=%q", sig1, pantrySig)
	}

	t4 := time.Now()
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	ledger, err := a.localListPantry()
	pantryDur := time.Since(t4)
	if err != nil {
		t.Fatalf("localListPantry(1000): %v", err)
	}
	if len(ledger.Items) != totalReagents {
		t.Fatalf("expected %d pantry items loaded, got %d", totalReagents, len(ledger.Items))
	}
	if len(sh.VaultFiles) < totalExps {
		t.Fatalf("expected shell.VaultFiles >= %d, got %d", totalExps, len(sh.VaultFiles))
	}
	if len(sh.PantryItems) != totalReagents {
		t.Fatalf("expected shell.PantryItems == %d, got %d", totalReagents, len(sh.PantryItems))
	}

	runtime.ReadMemStats(&memAfter)
	heapAllocMB := float64(memAfter.HeapAlloc) / (1024 * 1024)
	heapInuseMB := float64(memAfter.HeapInuse) / (1024 * 1024)
	totalAllocDeltaMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / (1024 * 1024)

	t.Logf("M3 scale metrics (500 exps + 1000 reagents): scan=%v index=%v search=%v watchSig=%v appInit+pantry=%v | heapAlloc=%.2fMB heapInuse=%.2fMB totalAllocDelta=%.2fMB gc=%d",
		scanDur, indexDur, searchDur, sigDur, pantryDur, heapAllocMB, heapInuseMB, totalAllocDeltaMB, memAfter.NumGC-memBefore.NumGC)

	if scanDur > 15*time.Second {
		t.Fatalf("500-file ScanTree exceeded 15s budget: %v", scanDur)
	}
	if indexDur > 10*time.Second {
		t.Fatalf("500-file VaultIndex.Refresh exceeded 10s budget: %v", indexDur)
	}
	if searchDur > 1*time.Second {
		t.Fatalf("500-file SearchFullText exceeded 1s budget: %v", searchDur)
	}
	if sigDur > 2*time.Second {
		t.Fatalf("500-file computeVaultDiskSignatures exceeded 2s budget: %v", sigDur)
	}
	if pantryDur > 8*time.Second {
		t.Fatalf("1000-reagent init + pantry load exceeded 8s budget: %v", pantryDur)
	}
}

// TestM3_MobileMemoryWarningCacheRelease populates app caches and multiple live
// views/editors, invokes releaseMemoryCaches(), and verifies inactive caches are
// cleared while the active view remains intact and renderable.
func TestM3_MobileMemoryWarningCacheRelease(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "MemWarnVault")
	if err := os.MkdirAll(filepath.Join(vaultRoot, "Proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name string
		body string
	}{
		{"Active.asterml", fsmTestDocAlpha},
		{"InactiveA.asterml", fsmTestDocBeta},
		{"InactiveB.asterml", huvecAsterDoc},
	} {
		if err := os.WriteFile(filepath.Join(vaultRoot, "Proj", f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.ForceMobilePreview = true

	// Mount and render 3 live files so liveViews / liveEditors are genuinely populated,
	// leaving Proj/Active.asterml as the active tab.
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 390, 844)

	for _, doc := range []struct {
		rel   string
		title string
	}{
		{"Proj/InactiveA.asterml", "InactiveA"},
		{"Proj/InactiveB.asterml", "InactiveB"},
		{"Proj/Active.asterml", "Active"},
	} {
		sh.OpenFileInPane(sh.ActivePaneID, doc.rel, doc.title, shell.ModeEdit, false)
		tt.Frame()
	}

	// Populate secondary/aux caches as well.
	if a.designCache == nil {
		a.designCache = map[string]runs.AsterDocSnapshot{}
	}
	a.designCache["des-1"] = runs.AsterDocSnapshot{DesignID: "des-1", Source: "# Cached Design"}
	a.designCache["des-2"] = runs.AsterDocSnapshot{DesignID: "des-2", Source: "# Cached Design 2"}

	if a.blockPreviewCache == nil {
		a.blockPreviewCache = map[string]workflowtree.BlockDefinition{}
	}
	a.blockPreviewCache["blk-1"] = workflowtree.BlockDefinition{}
	a.blockPreviewCache["blk-2"] = workflowtree.BlockDefinition{}

	if a.cachedAssays == nil {
		a.cachedAssays = map[string]cachedAssayResult{}
	}
	a.cachedAssays["assay-1"] = cachedAssayResult{}

	if a.secondaryViews == nil {
		a.secondaryViews = map[string]*asterdoc.View{}
	}
	a.secondaryViews["sec-1"] = &asterdoc.View{}
	a.secondaryViews["sec-2"] = &asterdoc.View{}

	if a.liveViews == nil {
		a.liveViews = map[string]*asterdoc.View{}
	}
	if _, ok := a.liveViews["Proj/Active.asterml"]; !ok {
		a.liveViews["Proj/Active.asterml"] = &asterdoc.View{}
	}
	a.liveViews["Proj/InactiveA.asterml"] = &asterdoc.View{}
	a.liveViews["Proj/InactiveB.asterml"] = &asterdoc.View{}

	if a.liveEditors == nil {
		a.liveEditors = map[string]*editor.State{}
	}
	if _, ok := a.liveEditors["Proj/Active.asterml"]; !ok {
		a.liveEditors["Proj/Active.asterml"] = &editor.State{}
	}
	a.liveEditors["Proj/InactiveA.asterml"] = &editor.State{}
	a.liveEditors["Proj/InactiveB.asterml"] = &editor.State{}

	activeView := a.liveViews["Proj/Active.asterml"]
	activeEditor := a.liveEditors["Proj/Active.asterml"]

	a.releaseMemoryCaches()

	if len(a.designCache) != 0 {
		t.Fatalf("expected designCache cleared, got %d", len(a.designCache))
	}
	if len(a.blockPreviewCache) != 0 {
		t.Fatalf("expected blockPreviewCache cleared, got %d", len(a.blockPreviewCache))
	}
	if len(a.cachedAssays) != 0 {
		t.Fatalf("expected cachedAssays cleared, got %d", len(a.cachedAssays))
	}
	if len(a.secondaryViews) != 0 {
		t.Fatalf("expected secondaryViews cleared, got %d", len(a.secondaryViews))
	}
	if got := a.liveViews["Proj/Active.asterml"]; got != activeView {
		t.Fatalf("expected active liveView preserved (%p), got %p", activeView, got)
	}
	if _, ok := a.liveViews["Proj/InactiveA.asterml"]; ok {
		t.Fatal("expected Proj/InactiveA.asterml liveView evicted")
	}
	if _, ok := a.liveViews["Proj/InactiveB.asterml"]; ok {
		t.Fatal("expected Proj/InactiveB.asterml liveView evicted")
	}
	if got := a.liveEditors["Proj/Active.asterml"]; got != activeEditor {
		t.Fatalf("expected active liveEditor preserved (%p), got %p", activeEditor, got)
	}
	if _, ok := a.liveEditors["Proj/InactiveA.asterml"]; ok {
		t.Fatal("expected Proj/InactiveA.asterml liveEditor evicted")
	}
	if _, ok := a.liveEditors["Proj/InactiveB.asterml"]; ok {
		t.Fatal("expected Proj/InactiveB.asterml liveEditor evicted")
	}

	// Active view must continue rendering cleanly after cache eviction.
	tt.Frame()
	if !tt.HasText("共培养参数") {
		t.Fatalf("expected active document still rendered after releaseMemoryCaches, texts=%v", tt.Texts())
	}
}

// TestM3_MobileAndHighDPIVisualMatrix verifies Light & Dark rendering at mobile
// 390x844 (Shell Mobile Agenda, Mobile Edit Toolbar, Mobile Pantry, Mobile Plate
// Summary/Sheet, Footnote Sheet) and desktop 1440x900 @ 1.25 and 1.5 DPI scale
// with zero panics, non-blank pixel variance, and zero overlap/truncation regressions.
func TestM3_MobileAndHighDPIVisualMatrix(t *testing.T) {
	outDir := t.TempDir()

	for _, dark := range []bool{false, true} {
		themeName := "light"
		if dark {
			themeName = "dark"
		}

		// 1. Mobile 390x844: Shell Mobile Agenda
		t.Run("mobile_agenda_"+themeName, func(t *testing.T) {
			s := &shell.Shell{
				TodayDate:          "2026-11-20",
				SelectedDate:       "2026-11-20",
				CalendarCursor:     "2026-11-20",
				CalendarMode:       1,
				ForceMobilePreview: true,
				Experiments: []shell.ExperimentItem{
					{ID: "e1", Title: "上午细胞传代", Date: "2026-11-20", StartHour: 9.5, DurationMin: 45, Stage: shell.StageInProgress, Folder: "细胞室"},
					{ID: "e2", Title: "明日流式检测", Date: "2026-11-21", Stage: shell.StageReady, Folder: "免疫组"},
					{ID: "e3", Title: "后日切片染色", Date: "2026-11-22", Stage: shell.StageDraft, Folder: "病理组"},
				},
				CalendarEvents: []schedule.Event{
					{ID: "ev1", Domain: schedule.DomainReagent, Kind: schedule.KindReagentRestock, Title: "补充 DMEM 培养基", DateYMD: "2026-11-20", Severity: schedule.SeverityWarning},
				},
			}
			s.OpenCalendarTab()
			s.SyncThemeLook(nil)

			tt := newM3VisualTester(t, 390, 844, 1.0, dark, func(c *ui.Context) {
				s.RenderSplitView(c)
			})
			settleM3Frames(tt, 240*time.Millisecond)
			if tt.HasText("07:00") {
				t.Fatalf("mobile agenda should not render cramped 7-col hourly grid")
			}
			assertM3VisualFrame(t, tt, outDir, "01_mobile_agenda_"+themeName, 390, 844, 1.0,
				[]string{"11月20日 – 11月22日 · 3日日程", "上午细胞传代", "明日流式检测", "补充 DMEM 培养基"})
		})

		// 2. Mobile 390x844: Mobile Edit Toolbar
		t.Run("mobile_edit_toolbar_"+themeName, func(t *testing.T) {
			vaultRoot := filepath.Join(t.TempDir(), "MobileToolbarVault")
			if err := os.MkdirAll(filepath.Join(vaultRoot, "Proj"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(vaultRoot, "Proj", "Alpha.asterml"), []byte(fsmTestDocAlpha), 0o644); err != nil {
				t.Fatal(err)
			}
			_, sh := newWiredFSMTestApp(t, vaultRoot)
			sh.ForceMobilePreview = true
			sh.MobileEditToolbar = true
			sh.OpenFileInPane(sh.ActivePaneID, "Proj/Alpha.asterml", "Alpha", shell.ModeEdit, false)

			tt := newM3VisualTester(t, 390, 844, 1.0, dark, func(c *ui.Context) {
				sh.RenderSplitView(c)
			})
			settleM3Frames(tt, 240*time.Millisecond)
			if !tt.HasText("+步骤") {
				t.Fatalf("mobile edit toolbar missing +步骤 action in %v", tt.Texts())
			}
			assertM3VisualFrame(t, tt, outDir, "02_mobile_edit_toolbar_"+themeName, 390, 844, 1.0,
				[]string{"插入实体 @", "插入模块 /", "批注 [^]"})
			// Also confirm document blocks rendered cleanly above the floating bottom toolbar without colliding with each other.
			assertM3VisualFrame(t, tt, outDir, "02_mobile_edit_toolbar_"+themeName+"_doc", 390, 844, 1.0,
				[]string{"实验属性", "实验分组"})
		})

		// 3. Mobile 390x844: Mobile Pantry
		t.Run("mobile_pantry_"+themeName, func(t *testing.T) {
			w := newM3MobileToolsWorkspace()
			tt := newM3VisualTester(t, 390, 844, 1.0, dark, func(c *ui.Context) {
				tools.ViewPantry(c, w)
			})
			settleM3Frames(tt, 200*time.Millisecond)
			if !tt.HasText("临期 (1)") {
				t.Fatalf("mobile pantry missing 临期 (1) chip in %v", tt.Texts())
			}
			assertM3VisualFrame(t, tt, outDir, "03_mobile_pantry_"+themeName, 390, 844, 1.0,
				[]string{"储藏室", "全部 (4)", "-80°C (1)", "打开盒子 抗体冻存盒 #1", "1× 无菌 PBS"})
		})

		// 4. Mobile 390x844: Mobile Plate Summary & Fullscreen Sheet
		t.Run("mobile_plate_summary_and_sheet_"+themeName, func(t *testing.T) {
			plateSrc := "---\ntitle: 96 孔板移动端测试\n---\n\n```asterlab\nkind: group_table\nid: gt-m3-plate\nschema: 1\ntitle: 实验分组\nrows:\n  - id: g-ctrl\n    name: 对照组\n    role: control\n    condition: PBS\n    independent_n: 3\n  - id: g-drug\n    name: 给药组\n    role: treatment\n    condition: 10 uM\n    independent_n: 3\n```\n\n```asterlab\nkind: plate\nid: pl-m3-96\nschema: 1\ntitle: 96 孔药敏排板\nformat: 96\ngroups:\n  g-ctrl: [A1, A2, A3]\n  g-drug: [B1, B2, B3]\n```\n"
			doc, err := asterdoc.Parse(plateSrc)
			if err != nil {
				t.Fatalf("asterdoc.Parse plate: %v", err)
			}
			var v *asterdoc.View
			v = &asterdoc.View{
				Doc:           doc,
				Registry:      asterdoc.BuiltinRegistry(),
				Mode:          asterdoc.ModeDesign,
				UnifiedLive:   true,
				MobileCompact: true,
				Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
				OnChange: func(next string) {
					if parsed, pErr := asterdoc.Parse(next); pErr == nil {
						v.Doc = parsed
					}
				},
			}
			tt := newM3VisualTester(t, 390, 844, 1.0, dark, func(c *ui.Context) {
				v.Render(c)
			})
			settleM3Frames(tt, 200*time.Millisecond)
			assertM3VisualFrame(t, tt, outDir, "04a_mobile_plate_summary_"+themeName, 390, 844, 1.0,
				[]string{"96 孔板 · 8×12", "全屏排板 / 读数"})

			if err := tt.Click("全屏排板 / 读数"); err != nil {
				t.Fatalf("open mobile plate sheet: %v", err)
			}
			settleM3Frames(tt, 600*time.Millisecond)
			if !tt.HasText("全屏孔板排板") {
				t.Fatalf("mobile plate sheet missing title 全屏孔板排板 in %v", tt.Texts())
			}
			assertM3VisualFrame(t, tt, outDir, "04b_mobile_plate_sheet_"+themeName, 390, 844, 1.0,
				[]string{"已排 6 / 96 孔 · 已选 0 孔", "孔 A1", "孔 H12", "点选", "涂抹框选", "完成"})
		})

		// 5. Mobile 390x844: Footnote Sheet
		t.Run("mobile_footnote_sheet_"+themeName, func(t *testing.T) {
			src := "---\ntitle: 移动脚注 Sheet 视觉验证\n---\n\n按每孔 100 uL 加入培养基[^1]。\n\n[^1]: 避光分装保存于 4°C，使用前平衡至室温。\n"
			doc, err := asterdoc.Parse(src)
			if err != nil {
				t.Fatalf("asterdoc.Parse: %v", err)
			}
			var v *asterdoc.View
			v = &asterdoc.View{
				Doc:           doc,
				Registry:      asterdoc.BuiltinRegistry(),
				Mode:          asterdoc.ModeDesign,
				UnifiedLive:   true,
				MobileCompact: true,
				Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
				OnChange: func(next string) {
					if parsed, pErr := asterdoc.Parse(next); pErr == nil {
						v.Doc = parsed
					}
				},
			}
			v.OpenMobileFootnoteSheet("1")

			tt := newM3VisualTester(t, 390, 844, 1.0, dark, func(c *ui.Context) {
				v.Render(c)
			})
			settleM3Frames(tt, 600*time.Millisecond)
			assertM3VisualFrame(t, tt, outDir, "05_mobile_footnote_sheet_"+themeName, 390, 844, 1.0,
				[]string{"批注 1", "保存批注", "折叠", "删除"})
		})

		// 6. Desktop 1440x900 @ 1.25 and 1.5 DPI scale
		for _, dpiScale := range []float32{1.25, 1.5} {
			scaleLabel := fmt.Sprintf("%.2f", dpiScale)
			t.Run(fmt.Sprintf("desktop_1440x900_scale_%s_%s", scaleLabel, themeName), func(t *testing.T) {
				vaultRoot := filepath.Join(t.TempDir(), "DesktopDPIVault")
				if err := os.MkdirAll(filepath.Join(vaultRoot, "Proj"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(vaultRoot, "Proj", "HUVEC.asterml"), []byte(huvecAsterDoc), 0o644); err != nil {
					t.Fatal(err)
				}
				_, sh := newWiredFSMTestApp(t, vaultRoot)
				sh.OpenFileInPane(sh.ActivePaneID, "Proj/HUVEC.asterml", "HUVEC", shell.ModeEdit, false)
				sh.SetRightDrawer(true, 0)

				tt := newM3VisualTester(t, 1440, 900, dpiScale, dark, func(c *ui.Context) {
					sh.RenderSplitView(c)
				})
				settleM3Frames(tt, 240*time.Millisecond)
				assertM3VisualFrame(t, tt, outDir, fmt.Sprintf("06_desktop_scale_%s_%s", scaleLabel, themeName), 1440, 900, dpiScale,
					[]string{"HUVEC 水凝胶体外成管测定", "GelMA 浓度实验分组", "实验库"})
			})
		}
	}
}

// TestM3_DataSafetyRoundtripQA runs an integrated end-to-end safety roundtrip
// across syncengine (mock Nutstore WebDAV + >2MB lossless original + mobile
// preview-only sync + on-demand fetch + .asterml conflict copy + pantry.jsonl
// merge), recovery (snapshot + restore + zip backup/restore), labio (XLSX/CSV
// import + Tidy CSV + .docx export), and crashguard (panic crash.log redaction +
// emergency draft restore).
func TestM3_DataSafetyRoundtripQA(t *testing.T) {
	ctx := context.Background()

	// === 1. syncengine: mock Nutstore WebDAV + lossless >2MB image + mobile preview-only + conflict + pantry merge ===
	srv := newM3MockWebDAVServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	desktopVault := filepath.Join(t.TempDir(), "DesktopVault")
	mobileVault := filepath.Join(t.TempDir(), "MobileVault")
	if err := os.MkdirAll(mobileVault, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"2026", "assets", ".asterlab"} {
		if err := os.MkdirAll(filepath.Join(desktopVault, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	expRel := "2026/cck8_assay.asterml"
	expInit := "---\ntitle: CCK8 细胞活力\n---\n# Initial Protocol\n"
	imgRel := "assets/microscopy_4k.png"
	largePNG := buildM3NoisyPNG(t, 1600, 1200)
	if int64(len(largePNG)) <= syncengine.Threshold2MB {
		t.Fatalf("expected largePNG > 2MB, got %d bytes", len(largePNG))
	}
	pantryInit := strings.Join([]string{
		`{"type":"schema","version":1}`,
		`{"type":"reagent","id":"r-1","name":"CCK-8 试剂","revision":1,"updatedAt":"2026-11-25T08:00:00Z"}`,
		`{"type":"tx","tx_id":"tx-base","id":"r-1","delta":100,"timestamp":"2026-11-25T08:00:00Z"}`,
	}, "\n") + "\n"

	if err := os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)), []byte(expInit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(imgRel)), largePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(syncengine.PantryLedgerRelPath)), []byte(pantryInit), 0o644); err != nil {
		t.Fatal(err)
	}

	deskClient, err := syncengine.NewWebDAVClient(syncengine.WebDAVConfig{
		BaseURL:            ts.URL,
		ForceNutstoreMode:  true,
		RateBurst:          100,
		RateLimitPerMinute: 6000,
	})
	if err != nil {
		t.Fatalf("NewWebDAVClient desktop: %v", err)
	}
	if !deskClient.IsNutstoreMode() || deskClient.EffectiveConcurrency() != 1 {
		t.Fatalf("expected Nutstore concurrency=1, got isNutstore=%v conc=%d", deskClient.IsNutstoreMode(), deskClient.EffectiveConcurrency())
	}
	deskEng, err := syncengine.NewEngine(desktopVault, deskClient, syncengine.DefaultDesktopDownloadPolicy(), "lab-pc")
	if err != nil {
		t.Fatalf("NewEngine desktop: %v", err)
	}
	if _, err := deskEng.Sync(ctx); err != nil {
		t.Fatalf("desktop initial sync: %v", err)
	}

	imgSHA := syncengine.SHA256Bytes(largePNG)
	origRemoteKey := syncengine.CanonicalOriginalPath(imgSHA, imgRel)
	prevRemoteKey := syncengine.CanonicalPreviewPath(imgSHA)
	srv.mu.Lock()
	remoteOrig := srv.files[origRemoteKey]
	remotePrev := srv.files[prevRemoteKey]
	srv.mu.Unlock()
	if !bytes.Equal(remoteOrig, largePNG) {
		t.Fatalf("remote original %s not lossless identical (got %d want %d)", origRemoteKey, len(remoteOrig), len(largePNG))
	}
	if len(remotePrev) == 0 || len(remotePrev) >= len(largePNG)/4 {
		t.Fatalf("expected compact preview at %s, got %d bytes", prevRemoteKey, len(remotePrev))
	}

	mobClient, err := syncengine.NewWebDAVClient(syncengine.WebDAVConfig{
		BaseURL:            ts.URL,
		ForceNutstoreMode:  true,
		RateBurst:          100,
		RateLimitPerMinute: 6000,
	})
	if err != nil {
		t.Fatalf("NewWebDAVClient mobile: %v", err)
	}
	mobPolicy := syncengine.DefaultMobileDownloadPolicy()
	mobPolicy.IsMeteredNetwork = false
	mobEng, err := syncengine.NewEngine(mobileVault, mobClient, mobPolicy, "iphone-bench")
	if err != nil {
		t.Fatalf("NewEngine mobile: %v", err)
	}
	mobEng.NowFn = func() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) }

	mobRep1, err := mobEng.Sync(ctx)
	if err != nil {
		t.Fatalf("mobile initial sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(imgRel))); !os.IsNotExist(err) {
		t.Fatalf("expected >2MB original NOT downloaded on mobile initial sync")
	}
	if _, err := os.Stat(filepath.Join(mobileVault, filepath.FromSlash(prevRemoteKey))); err != nil {
		t.Fatalf("expected preview sidecar downloaded on mobile: %v", err)
	}
	if len(mobRep1.NeedsOriginalDownload) != 1 || mobRep1.NeedsOriginalDownload[0] != imgRel {
		t.Fatalf("expected NeedsOriginalDownload=[%s], got %v", imgRel, mobRep1.NeedsOriginalDownload)
	}

	// On-demand original fetch on mobile pulls 100% lossless bytes.
	if err := mobEng.FetchOriginalOnDemand(ctx, imgRel); err != nil {
		t.Fatalf("FetchOriginalOnDemand: %v", err)
	}
	fetchedOrig, err := os.ReadFile(filepath.Join(mobileVault, filepath.FromSlash(imgRel)))
	if err != nil || !bytes.Equal(fetchedOrig, largePNG) {
		t.Fatalf("fetched original mismatch: err=%v", err)
	}

	// Concurrent .asterml edit conflict + pantry.jsonl append-only merge.
	expDesk := "---\ntitle: CCK8 细胞活力\n---\n# Edited on Lab PC\n"
	pantryDesk := pantryInit +
		`{"type":"tx","tx_id":"tx-shared","id":"r-1","delta":-5,"timestamp":"2026-11-26T11:50:00Z"}` + "\n" +
		`{"type":"tx","tx_id":"tx-desk-1","id":"r-1","delta":-10,"timestamp":"2026-11-26T11:55:00Z"}` + "\n"
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)), []byte(expDesk), 0o644)
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(syncengine.PantryLedgerRelPath)), []byte(pantryDesk), 0o644)
	if _, err := deskEng.Sync(ctx); err != nil {
		t.Fatalf("desktop concurrent sync: %v", err)
	}

	expMob := "---\ntitle: CCK8 细胞活力\n---\n# Edited on Mobile\n"
	pantryMob := pantryInit +
		`{"type":"tx","tx_id":"tx-shared","id":"r-1","delta":-5,"timestamp":"2026-11-26T11:50:00Z"}` + "\n" +
		`{"type":"tx","tx_id":"tx-mob-1","id":"r-1","delta":-15,"timestamp":"2026-11-26T11:58:00Z"}` + "\n"
	_ = os.WriteFile(filepath.Join(mobileVault, filepath.FromSlash(expRel)), []byte(expMob), 0o644)
	_ = os.WriteFile(filepath.Join(mobileVault, filepath.FromSlash(syncengine.PantryLedgerRelPath)), []byte(pantryMob), 0o644)

	mobRep2, err := mobEng.Sync(ctx)
	if err != nil {
		t.Fatalf("mobile conflict sync: %v", err)
	}
	if len(mobRep2.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict entry, got %+v", mobRep2.Conflicts)
	}
	gotMobExp, _ := os.ReadFile(filepath.Join(mobileVault, filepath.FromSlash(expRel)))
	if string(gotMobExp) != expMob {
		t.Fatalf("mobile local edit was overwritten: got %q", string(gotMobExp))
	}
	conflictCopy, err := os.ReadFile(filepath.Join(mobileVault, filepath.FromSlash(mobRep2.Conflicts[0].ConflictCopyPath)))
	if err != nil || string(conflictCopy) != expDesk {
		t.Fatalf("conflict copy mismatch: err=%v got=%q", err, string(conflictCopy))
	}
	if !mobRep2.MergedPantry {
		t.Fatalf("expected MergedPantry=true")
	}
	mergedPantry, _ := os.ReadFile(filepath.Join(mobileVault, filepath.FromSlash(syncengine.PantryLedgerRelPath)))
	for _, tx := range []string{"tx-base", "tx-shared", "tx-desk-1", "tx-mob-1"} {
		if !strings.Contains(string(mergedPantry), tx) {
			t.Fatalf("merged pantry missing %q", tx)
		}
	}
	if strings.Count(string(mergedPantry), "tx-shared") != 1 {
		t.Fatalf("expected tx-shared deduplicated to 1 occurrence")
	}

	// === 2. recovery: auto snapshot + restore + full vault zip backup/restore ===
	hm := recovery.NewHistoryManager(desktopVault, recovery.DefaultSnapshotPolicy())
	snap1, created, err := hm.SnapshotFile(expRel, "auto", true)
	if err != nil || !created {
		t.Fatalf("SnapshotFile: created=%v err=%v", created, err)
	}
	// Simulate accidental overwrite, then restore from snapshot.
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)), []byte("ACCIDENTALLY WIPED"), 0o644)
	if err := hm.RestoreSnapshot(expRel, snap1.ID); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}
	restoredExp, _ := os.ReadFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)))
	if string(restoredExp) != expDesk {
		t.Fatalf("RestoreSnapshot mismatch: got %q want %q", string(restoredExp), expDesk)
	}

	zipPath := filepath.Join(t.TempDir(), "vault-backup.zip")
	bkRes, err := recovery.BackupVaultZip(desktopVault, zipPath)
	if err != nil || bkRes.FileCount < 3 {
		t.Fatalf("BackupVaultZip: res=%+v err=%v", bkRes, err)
	}
	if _, err := recovery.VerifyBackupZip(zipPath); err != nil {
		t.Fatalf("VerifyBackupZip: %v", err)
	}
	_ = os.WriteFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)), []byte("DESTROYED BEFORE ZIP RESTORE"), 0o644)
	if _, err := recovery.RestoreVaultZip(zipPath, desktopVault); err != nil {
		t.Fatalf("RestoreVaultZip: %v", err)
	}
	afterZipRestore, _ := os.ReadFile(filepath.Join(desktopVault, filepath.FromSlash(expRel)))
	if string(afterZipRestore) != expDesk {
		t.Fatalf("RestoreVaultZip mismatch: got %q want %q", string(afterZipRestore), expDesk)
	}

	// === 3. labio: XLSX + CSV import -> Tidy CSV -> .docx export ===
	csvMatrix := strings.Join([]string{
		",1,2,3",
		"A,0.120,0.450,0.890",
		"B,0.118,0.442,0.880",
	}, "\n")
	csvRes, err := labio.ImportPlateMatrix([]byte(csvMatrix))
	if err != nil || csvRes.Wells["A2"].Value == nil || *csvRes.Wells["A2"].Value != 0.450 {
		t.Fatalf("ImportPlateMatrix CSV failed: res=%+v err=%v", csvRes, err)
	}
	xlsxRes, err := labio.ImportPlateMatrix(buildM3MinimalXLSX(t))
	if err != nil || xlsxRes.Wells["B2"].Value == nil || *xlsxRes.Wells["B2"].Value != 0.875 {
		t.Fatalf("ImportPlateMatrix XLSX failed: res=%+v err=%v", xlsxRes, err)
	}

	var tidyRows []labio.TidyObservation
	for _, w := range xlsxRes.Ordered {
		if w.Value == nil {
			continue
		}
		tidyRows = append(tidyRows, labio.TidyObservation{
			Experiment: "20261126_CCK8",
			Group:      "Drug-10uM",
			Role:       "treatment",
			Timepoint:  "24h",
			Replicate:  w.Col + 1,
			Well:       w.Coord,
			Metric:     "OD450",
			Value:      *w.Value,
			Unit:       "AU",
		})
	}
	tidyCSV, err := labio.ExportTidyCSV(tidyRows, labio.TidyCSVOptions{IncludeUTF8BOM: true, Precision: 3})
	if err != nil || !bytes.HasPrefix(tidyCSV, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("ExportTidyCSV failed: err=%v len=%d", err, len(tidyCSV))
	}
	parsedCSV, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(tidyCSV, []byte{0xef, 0xbb, 0xbf}))).ReadAll()
	if err != nil || len(parsedCSV) != len(tidyRows)+1 {
		t.Fatalf("re-parse tidy CSV failed: rows=%d err=%v", len(parsedCSV), err)
	}

	pngThumb := buildM3SolidPNG(t, 120, 80, color.RGBA{R: 32, G: 140, B: 220, A: 255})
	docxBytes, err := labio.ExportDocx(labio.DocxDocument{
		Title:    "CCK-8 数据安全全链路报告",
		Subtitle: "Sync + Recovery + LabIO + CrashGuard",
		Metadata: []labio.DocxMetaKV{
			{Key: "实验", Value: "2026/cck8_assay.asterml"},
		},
		Footnotes: []string{"来自 XLSX 与 CSV 导入的验证汇总。"},
		Sections: []labio.DocxSection{
			{
				Heading: "1. 读数汇总",
				Level:   1,
				Paragraphs: []labio.DocxPara{
					{Text: "已通过实验室安全通道归档原始图像与结构化读数。", FootnoteIDs: []int{1}},
				},
				Tables: []labio.DocxTable{
					{
						Caption: "表 1. OD450 抽样",
						Headers: []string{"孔位", "OD450"},
						Rows: [][]string{
							{"A1", "0.250"},
							{"B2", "0.875"},
						},
					},
				},
				Figures: []labio.DocxFigure{
					{Caption: "图 1. 质控缩略图", PNGBytes: pngThumb},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportDocx: %v", err)
	}
	assertM3ValidDocx(t, docxBytes)

	// === 4. crashguard: panic crash.log secret redaction + emergency draft restore ===
	logDir := filepath.Join(t.TempDir(), "logs")
	draftBase := t.TempDir()
	unsavedDraft := "---\ntitle: CCK8 未保存草稿\n---\n# 紧急暂存步骤\n- 移液 100 uL\n"
	if _, err := crashguard.SaveEmergencyDraft(draftBase, expRel, unsavedDraft, 28, "v1.0.0-qa"); err != nil {
		t.Fatalf("SaveEmergencyDraft: %v", err)
	}

	func() {
		defer crashguard.RecoverAndWriteCrashLog(logDir, map[string]string{
			"webdavURL": "https://labuser:ultra-secret-pass@dav.jianguoyun.com/dav",
			"auth":      "Bearer super-secret-jwt-987654",
		})
		panic("simulated crash with KANDEV_API_TOKEN=secret_kandev_tok_abc")
	}()

	crashRaw, err := os.ReadFile(filepath.Join(logDir, crashguard.CrashLogFileName))
	if err != nil {
		t.Fatalf("read crash.log: %v", err)
	}
	crashText := string(crashRaw)
	for _, secret := range []string{"ultra-secret-pass", "super-secret-jwt-987654", "secret_kandev_tok_abc"} {
		if strings.Contains(crashText, secret) {
			t.Fatalf("crash.log leaked secret %q", secret)
		}
	}
	if !strings.Contains(crashText, "<redacted>") || !strings.Contains(crashText, "simulated crash") {
		t.Fatalf("crash.log missing redaction or panic context:\n%s", crashText)
	}

	recoveredDraft, ok, err := crashguard.LoadEmergencyDraft(draftBase, expRel)
	if err != nil || !ok || recoveredDraft.Content != unsavedDraft || recoveredDraft.CursorOffset != 28 {
		t.Fatalf("LoadEmergencyDraft failed: ok=%v err=%v draft=%+v", ok, err, recoveredDraft)
	}
	if err := crashguard.ClearEmergencyDraft(draftBase, expRel); err != nil {
		t.Fatalf("ClearEmergencyDraft: %v", err)
	}
}

// --- Local QA helpers (self-contained inside m3_scale_and_mobile_qa_test.go) ---

func newM3VisualTester(t *testing.T, w, h int, scale float32, dark bool, draw func(c *ui.Context)) *ui.Tester {
	t.Helper()
	tt := ui.NewTesterWith(func(c *ui.Context) {
		if dark {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Background(tok.Canvas).Children(func() {
			draw(c)
		})
	}, w, h, ui.TesterOptions{Dark: dark})
	tt.SetDark(dark)
	if scale > 0 && scale != 1.0 {
		tt.SetScale(scale)
	}
	tt.Frame()
	return tt
}

func settleM3Frames(tt *ui.Tester, d time.Duration) {
	steps := int(d / (16 * time.Millisecond))
	if steps < 1 {
		steps = 1
	}
	for range steps {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
}

func assertM3VisualFrame(t *testing.T, tt *ui.Tester, outDir, name string, wantW, wantH int, scale float32, requiredTexts []string) {
	t.Helper()
	img := tt.Image()
	if img == nil {
		t.Fatalf("%s: nil rendered image", name)
	}
	minW := int(math.Floor(float64(wantW) * float64(scale)))
	minH := int(math.Floor(float64(wantH) * float64(scale)))
	assertNonBlankImage(t, img, name, minW, minH)

	// Persist PNG strictly inside t.TempDir() so tracked repo PNGs are never dirtied.
	outPath := filepath.Join(outDir, name+".png")
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("%s: create temp PNG: %v", name, err)
	}
	if encErr := png.Encode(f, img); encErr != nil {
		_ = f.Close()
		t.Fatalf("%s: encode PNG: %v", name, encErr)
	}
	_ = f.Close()

	// Verify required texts exist, are non-truncated (non-zero reasonable rect inside viewport),
	// and do not overlap one another.
	rects := make([]ui.Rect, 0, len(requiredTexts))
	for _, want := range requiredTexts {
		r, ok := tt.Find(want)
		if !ok {
			t.Fatalf("%s: missing required text %q in %v", name, want, tt.Texts())
		}
		if r.W < 4 || r.H < 4 {
			t.Fatalf("%s: truncated/collapsed rect for %q: %+v", name, want, r)
		}
		if r.X < -2 || r.Y < -2 || r.X+r.W > float32(wantW)+4 || r.Y+r.H > float32(wantH)+4 {
			t.Fatalf("%s: %q clipped outside %dx%d viewport: %+v", name, want, wantW, wantH, r)
		}
		for i, prev := range rects {
			if m3RectsOverlap(prev, r) {
				t.Fatalf("%s: layout overlap between %q (%+v) and %q (%+v)", name, requiredTexts[i], prev, want, r)
			}
		}
		rects = append(rects, r)
	}

	// Verify visible interactive buttons do not collapse to degenerate dimensions.
	for _, acc := range tt.AccessAll() {
		if acc.Offscreen || acc.Role != ui.RoleButton || strings.TrimSpace(acc.Name) == "" {
			continue
		}
		if acc.Bounds.W < 4 || acc.Bounds.H < 4 {
			t.Fatalf("%s: degenerate button bounds for %q: %+v", name, acc.Name, acc.Bounds)
		}
	}
}

func m3RectsOverlap(a, b ui.Rect) bool {
	const eps = 1.0
	if a.X+a.W <= b.X+eps || b.X+b.W <= a.X+eps {
		return false
	}
	if a.Y+a.H <= b.Y+eps || b.Y+b.H <= a.Y+eps {
		return false
	}
	return true
}

func newM3MobileToolsWorkspace() *tools.Workspace {
	items := []runs.PantryItem{
		{ID: "p-ab", Name: "兔抗-VEGFA 一抗", Location: "-80°C", Status: "opened", Revision: 1},
		{ID: "p-taq", Name: "SYBR Green 预混液", Location: "-20°C 酶盒", Status: "near_expiry", ExpiresOn: "2026-12-01", Revision: 1},
		{ID: "p-pbs", Name: "1× 无菌 PBS", Location: "4°C 冷藏架", Status: "unopened", Revision: 1},
		{ID: "p-dmso", Name: "无水 DMSO", Location: "常温避光", Status: "unopened", Revision: 1},
	}
	meta := map[string]tools.PantryItemMeta{
		"p-ab": {BoxID: "box-cryo", BoxSlot: "C2", LotNo: "GR341902"},
	}
	boxes := []tools.PantryBox{
		{ID: "box-cryo", Name: "抗体冻存盒 #1", Kind: tools.BoxCryo9x9, ZoneKey: "-80", Rows: 9, Cols: 9},
		{ID: "box-ep", Name: "酶管 EP 盒", Kind: tools.BoxEP50, ZoneKey: "-20", Rows: 5, Cols: 10},
	}
	return tools.New(tools.Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			return runs.PantryLedger{Items: append([]runs.PantryItem(nil), items...)}, nil
		},
		UpsertPantry: func(input runs.PantryUpsert) (runs.PantryItem, error) {
			ni := runs.PantryItem{ID: "p-new", Name: input.Name, Location: input.Location, Status: "unopened", Revision: 1}
			items = append(items, ni)
			return ni, nil
		},
		LoadPantryMeta: func() (map[string]tools.PantryItemMeta, error) {
			return meta, nil
		},
		SavePantryMeta: func(m map[string]tools.PantryItemMeta) error {
			meta = m
			return nil
		},
		LoadPantryBoxes: func() ([]tools.PantryBox, error) {
			return append([]tools.PantryBox(nil), boxes...), nil
		},
		SavePantryBoxes: func(b []tools.PantryBox) error {
			boxes = append([]tools.PantryBox(nil), b...)
			return nil
		},
	})
}

type m3MockWebDAVServer struct {
	mu          sync.Mutex
	files       map[string][]byte
	dirs        map[string]bool
	inFlight    atomic.Int64
	maxInFlight atomic.Int64
}

func newM3MockWebDAVServer() *m3MockWebDAVServer {
	return &m3MockWebDAVServer{
		files: map[string][]byte{},
		dirs:  map[string]bool{"": true},
	}
}

func (m *m3MockWebDAVServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cur := m.inFlight.Add(1)
	for {
		prev := m.maxInFlight.Load()
		if cur <= prev || m.maxInFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer m.inFlight.Add(-1)

	rel := strings.TrimPrefix(r.URL.Path, "/")
	if decoded, err := url.PathUnescape(r.URL.EscapedPath()); err == nil {
		rel = strings.TrimPrefix(decoded, "/")
	}

	switch r.Method {
	case http.MethodGet:
		m.mu.Lock()
		data, ok := m.files[rel]
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.mu.Lock()
		m.files[rel] = body
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		m.mu.Lock()
		_, ok := m.files[rel]
		delete(m.files, rel)
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "MKCOL":
		dir := strings.Trim(rel, "/")
		m.mu.Lock()
		exists := m.dirs[dir]
		m.dirs[dir] = true
		m.mu.Unlock()
		if exists {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
	case "MOVE":
		dstURL, err := url.Parse(r.Header.Get("Destination"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		dstRel := strings.TrimPrefix(dstURL.Path, "/")
		m.mu.Lock()
		src, ok := m.files[rel]
		if ok {
			m.files[dstRel] = append([]byte(nil), src...)
			delete(m.files, rel)
		}
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	case "PROPFIND":
		dir := strings.Trim(rel, "/")
		m.mu.Lock()
		var children []string
		for k := range m.files {
			if dir == "" || strings.HasPrefix(k, dir+"/") {
				children = append(children, k)
			}
		}
		sort.Strings(children)
		var buf bytes.Buffer
		buf.WriteString(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:">`)
		buf.WriteString(fmt.Sprintf(`<D:response><D:href>/%s</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, dir))
		for _, c := range children {
			buf.WriteString(fmt.Sprintf(`<D:response><D:href>/%s</D:href><D:propstat><D:prop><D:resourcetype/><D:getcontentlength>%d</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, c, len(m.files[c])))
		}
		buf.WriteString(`</D:multistatus>`)
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write(buf.Bytes())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func buildM3NoisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	var state uint32 = 0x9e3779b9
	pix := img.Pix
	for i := 0; i < len(pix); i += 4 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		pix[i] = byte(state)
		pix[i+1] = byte(state >> 8)
		pix[i+2] = byte(state >> 16)
		pix[i+3] = 255
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("encode noisy PNG: %v", err)
	}
	return buf.Bytes()
}

func buildM3SolidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode solid PNG: %v", err)
	}
	return buf.Bytes()
}

func buildM3MinimalXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="OD450" sheetId="1" r:id="rId1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>A</t></si>
  <si><t>B</t></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="B1"><v>1</v></c><c r="C1"><v>2</v></c></row>
    <row r="2"><c r="A2" t="s"><v>0</v></c><c r="B2"><v>0.25</v></c><c r="C2"><v>0.30</v></c></row>
    <row r="3"><c r="A3" t="s"><v>1</v></c><c r="B3"><v>0.80</v></c><c r="C3"><v>0.875</v></c></row>
  </sheetData>
</worksheet>`,
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertM3ValidDocx(t *testing.T, docxBytes []byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		t.Fatalf("ExportDocx invalid zip: %v", err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = raw
	}
	for _, req := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/footnotes.xml", "word/media/image1.png"} {
		if _, ok := parts[req]; !ok {
			t.Fatalf("docx missing required part %q", req)
		}
	}
	for name, raw := range parts {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid XML in %q: %v", name, err)
			}
		}
	}
}
