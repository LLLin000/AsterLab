package labsticker

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

func TestAllVesselsParseAndComposeDieCutStickers(t *testing.T) {
	specs := AllVessels()
	if len(specs) != 12 {
		t.Fatalf("expected 12 curated vessels, got %d", len(specs))
	}
	for _, spec := range specs {
		svg, err := ParseVesselSVG(spec.ID)
		if err != nil {
			t.Fatalf("ParseVesselSVG(%s): %v", spec.ID, err)
		}
		if svg == nil {
			t.Fatalf("nil SVG for %s", spec.ID)
		}
		bmp, metrics, err := Sticker(spec.ID, StickerOptions{TargetHeightDIP: spec.DefaultHeight, BorderDIP: 4.0})
		if err != nil {
			t.Fatalf("Sticker(%s): %v", spec.ID, err)
		}
		if bmp == nil || metrics.WidthDIP <= 0 || metrics.HeightDIP <= 0 {
			t.Fatalf("invalid sticker metrics for %s: %+v", spec.ID, metrics)
		}
		bw, bh := bmp.Size()
		if bw <= 0 || bh <= 0 {
			t.Fatalf("empty bitmap for %s: %dx%d", spec.ID, bw, bh)
		}
	}
}

func TestAllBoxesParseAndComposeDieCutStickers(t *testing.T) {
	boxes := AllBoxes()
	if len(boxes) != 5 {
		t.Fatalf("expected 5 curated boxes/racks, got %d", len(boxes))
	}
	for _, spec := range boxes {
		if !spec.IsBox {
			t.Fatalf("expected IsBox=true for %s", spec.ID)
		}
		svg, err := ParseVesselSVG(spec.ID)
		if err != nil || svg == nil {
			t.Fatalf("ParseVesselSVG(%s): %v", spec.ID, err)
		}
		bmp, metrics, err := Sticker(spec.ID, StickerOptions{TargetHeightDIP: spec.DefaultHeight, BorderDIP: 4.0})
		if err != nil || bmp == nil || metrics.WidthDIP <= 0 || metrics.HeightDIP <= 0 {
			t.Fatalf("Sticker(%s) failed: err=%v metrics=%+v", spec.ID, err, metrics)
		}
	}
}

func TestExteriorFloodFillClosesTransparentCavitiesAndDrawsWhiteBorder(t *testing.T) {
	// Ring shape with a transparent hollow inside (20..40, 20..40) inside 60x60 art
	ring := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := 10; y < 50; y++ {
		for x := 10; x < 50; x++ {
			inInner := x >= 20 && x < 40 && y >= 20 && y < 40
			if !inInner {
				ring.SetRGBA(x, y, color.RGBA{30, 120, 220, 255})
			}
		}
	}
	sil := computeClosedSilhouette(ring, 60, 60)
	// Center (30,30) was transparent in input, so exterior flood-fill MUST close it!
	if !sil[30*60+30] {
		t.Fatalf("expected hollow interior cavity at (30,30) to be marked interior")
	}
	// Outer margin (2,2) MUST remain exterior
	if sil[2*60+2] {
		t.Fatalf("expected outer corner (2,2) to remain exterior")
	}
	bmp, metrics := StickerFromImage(ring, StickerOptions{TargetHeightDIP: 60, BorderDIP: 4})
	if bmp == nil || metrics.WidthDIP <= 60 {
		t.Fatalf("expected custom image sticker with padding, got %+v", metrics)
	}
}

func TestRenderLabStickerVisuals(t *testing.T) {
	outDir := os.Getenv("ASTER_CAPTURE_DIR")
	if outDir == "" {
		return
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	capturePNG := func(name string, w, h int, dark bool, draw func(c *ui.Context)) {
		t.Helper()
		tt := ui.NewTester(func(c *ui.Context) {
			if dark {
				c.SetTheme(theme.DarkTheme())
			} else {
				c.SetTheme(theme.LightTheme())
			}
			tok := theme.GetTokens(c.Theme())
			ui.Box(c).Fill().Background(tok.Canvas).Children(func() {
				draw(c)
			})
		}, w, h)
		tt.SetDark(dark)
		tt.Frame()
		img := tt.Image()
		if img == nil {
			t.Fatalf("%s: nil screenshot image", name)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Complete 12-vessel BioRender-style die-cut sticker specimen sheet
	capturePNG("comp_16_labsticker_sheet_v3.png", 1280, 740, false, func(c *ui.Context) {
		RenderStickerCatalogSheet(c)
	})

	// 2 & 3. Crafted Pantry shelf preview (Light + Dark mode): paired compact cubbies (-80°C & -20°C)
	// + full-width 4°C bench rack, left-packed natural spacing, 3D shelf ledges, and manual/URL indicators.
	tiers := []ShelfTier{
		{
			Title:    "-80°C 超低温细胞与抗体冻存架",
			Subtitle: "第一层 · 长期保藏冻存管",
			ZoneKey:  "-80",
			Items: []ShelfItem{
				{
					ID: "t4lig", Name: "T4 DNA 连接酶", Vessel: VesselCryovial2,
					Status: StatusOpened, BadgeText: "已开封", MetaLine: "2 mL · Box A3", HasManual: true,
				},
				{
					ID: "vegfa-ab", Name: "兔抗-VEGFA 一抗", Vessel: VesselAmberMicrovial,
					Status: StatusOK, MetaLine: "避光 · 冻存盒 C2", HasManual: true, HasURL: true,
				},
			},
		},
		{
			Title:    "-20°C 酶与核酸反应预混架",
			Subtitle: "第二层 · qPCR 与分子试剂",
			ZoneKey:  "-20",
			Items: []ShelfItem{
				{
					ID: "sybr", Name: "SYBR Green 预混液", Vessel: VesselScrewTube2,
					Status: StatusNearExpiry, BadgeText: "临期 9天", MetaLine: "2 mL · 余 40%", HasManual: true, HasURL: true,
				},
				{
					ID: "vanco", Name: "万古霉素标准品", Vessel: VesselSerumVial,
					Status: StatusOK, MetaLine: "10 mL · 冻干粉", HasURL: true,
				},
				{
					ID: "qpcr-strip", Name: "GAPDH 引物八联条", Vessel: VesselPCRStrip,
					Status: StatusExhausted, BadgeText: "已用尽", MetaLine: "0.2 mL×8 · 待补购",
				},
			},
		},
		{
			Title:    "4°C 冷藏培养基与常用缓冲液货架",
			Subtitle: "第三层 · 细胞培养与日常前处理",
			ZoneKey:  "4C",
			Items: []ShelfItem{
				{
					ID: "dmem", Name: "DMEM 高糖培养基", Vessel: VesselMediaBottle500,
					Status: StatusOpened, BadgeText: "已开封", MetaLine: "500 mL · 4°C 门架", HasManual: true, HasURL: true,
				},
				{
					ID: "pbs", Name: "1× 无菌 PBS 缓冲液", Vessel: VesselReagentGL45,
					Status: StatusOK, MetaLine: "500 mL · 二层左", HasURL: true,
				},
				{
					ID: "fbs", Name: "胎牛血清 (FBS)", Vessel: VesselFalcon50,
					Status: StatusOpened, MetaLine: "50 mL · 暖琥珀分装", HasManual: true,
				},
				{
					ID: "trypsin", Name: "0.25% 胰酶 EDTA", Vessel: VesselFalcon15,
					Status: StatusNearExpiry, BadgeText: "余 14天", MetaLine: "15 mL · 二层右", HasURL: true,
				},
				{
					ID: "pfa", Name: "4% 多聚甲醛固定液", Vessel: VesselAmberBottle500,
					Status: StatusOK, MetaLine: "500 mL · 避光柜", HasManual: true,
				},
			},
		},
	}

	renderShelfShowcase := func(c *ui.Context) {
		t := c.Theme()
		tok := theme.GetTokens(t)
		query := ""
		zones := []string{"全部 (10)", "-80°C (2)", "-20°C (3)", "4°C (5)", "临期提醒 (2)"}
		ui.Column(c).Fill().Background(tok.Canvas).Padding(t.Space(6)).Gap(t.Space(3)).Children(func() {
			// Header + Action
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Column(c).Gap(t.Space(0)).Children(func() {
					ui.Text(c, "储藏室货架 · 试剂贴纸视图").FontSize(float32(tok.TextPage)*0.84).Bold().TextColor(tok.Text)
					ui.Text(c, "按温区自然成组陈列，点击任意容器贴纸即可查阅批号、产品网址与已上传说明书 (PDF)。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
				ui.Spacer(c)
				components.Button(c, "导出贴标清单", components.ButtonSecondary)
				components.Button(c, "+ 新试剂上架", components.ButtonPrimary)
			})

			// Top Search & Zone Filter Toolbar
			ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(3)).
				Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).
				Shadow(0, 1, 2, 0, tok.Text.Alpha(0.04)).
				AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Box(c).Width(240).Children(func() {
					components.TextBox(c, &query, components.TextBoxOptions{LeadingIcon: lucide.Search()}).
						Placeholder("搜索试剂名称、货号或柜层…")
				})
				ui.Box(c).Width(1).Height(t.Space(4)).Background(tok.BorderStrong)
				for idx, z := range zones {
					variant := components.ButtonGhost
					if idx == 0 {
						variant = components.ButtonToolbar
					}
					components.Button(c, z, variant).FontFeatures("tnum")
				}
			})

			RenderShelfGrid(c, tiers, ShelfRenderOptions{
				SelectedID:  "sybr",
				OnAddInTier: func(ShelfTier) {},
			})
		})
	}

	capturePNG("comp_16b_labsticker_shelf_light_v3.png", 1280, 760, false, renderShelfShowcase)
	capturePNG("comp_16c_labsticker_shelf_dark_v3.png", 1280, 760, true, renderShelfShowcase)
}
