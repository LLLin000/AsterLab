package asterdoc

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func writeTestFluorescencePNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir asset dir: %v", err)
	}
	const w, h = 320, 220
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx1 := float64(x - 105)
			dy1 := float64(y - 110)
			d1 := math.Hypot(dx1, dy1)
			dx2 := float64(x - 210)
			dy2 := float64(y - 105)
			d2 := math.Hypot(dx2, dy2)

			r := uint8(16)
			g := uint8(24)
			b := uint8(34)
			if d1 < 56 {
				intensity := math.Max(0, 1.0-d1/56.0)
				g = uint8(min(255, int(g)+int(195*intensity)))
				b = uint8(min(255, int(b)+int(85*intensity)))
			}
			if d2 < 48 {
				intensity := math.Max(0, 1.0-d2/48.0)
				r = uint8(min(255, int(r)+int(80*intensity)))
				b = uint8(min(255, int(b)+int(215*intensity)))
			}
			if math.Abs(d1-56) < 1.5 || math.Abs(d2-48) < 1.5 {
				r, g, b = 210, 224, 238
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
}

func saveFigureShotIfRequested(t *testing.T, tt *ui.Tester, filename string) {
	t.Helper()
	dir := os.Getenv("ASTERLAB_FIGURE_SHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir shot dir: %v", err)
	}
	img := tt.Image()
	if img == nil {
		t.Fatalf("nil tester image for %s", filename)
	}
	out := filepath.Join(dir, filename)
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("create shot %s: %v", out, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode shot %s: %v", out, err)
	}
}

func settleFigureFrames(tt *ui.Tester, ms int) {
	tt.Frame()
	steps := ms / 16
	if steps < 4 {
		steps = 4
	}
	for i := 0; i < steps; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
}

func meanImageLuma(img image.Image) float64 {
	if img == nil {
		return 255
	}
	b := img.Bounds()
	var sum float64
	var count int
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += (0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8))
			count++
		}
	}
	if count == 0 {
		return 255
	}
	return sum / float64(count)
}

func TestFigureAndEmbedCardDarkroomLightboxDesktopAndMobile(t *testing.T) {
	baseDir := t.TempDir()
	imgRel := "./assets/huvec_a1_6h.png"
	writeTestFluorescencePNG(t, filepath.Join(baseDir, "assets", "huvec_a1_6h.png"))

	src := `---
title: HUVEC 成管显微暗房测试
date: 2026-11-27
---

![[./assets/huvec_a1_6h.png|400]]{@plate-12:A1 6h fov=1 role=obs note="中心视野血管成管"}

` + "```asterlab\nkind: group_table\nid: gt-1\nschema: 1\nrows:\n  - {id: g-ctrl, name: 对照组, role: control}\n```\n\n" +
		"```asterlab\nkind: plate\nid: plate-12\nschema: 1\ntitle: 12 孔成管板\nformat: 12\ngroups:\n  g-ctrl: [A1]\n```\n\n" +
		"```asterlab\nkind: figure\nid: fig-svg-1\nschema: 1\ntitle: 培养皿分区示意图\ncaption: 90 mm 琼脂皿四象限对照\nsvg: |\n  <svg viewBox=\"0 0 360 220\" xmlns=\"http://www.w3.org/2000/svg\">\n    <circle cx=\"180\" cy=\"110\" r=\"82\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"/>\n    <text x=\"180\" y=\"114\" text-anchor=\"middle\" font-size=\"12\">Q1 对照</text>\n  </svg>\n```\n"

	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	for _, vp := range []struct {
		name   string
		w, h   int
		mobile bool
		dark   bool
	}{
		{name: "desktop", w: 1180, h: 760, mobile: false, dark: true},
		{name: "mobile", w: 402, h: 874, mobile: true, dark: false},
	} {
		t.Run(vp.name, func(t *testing.T) {
			var openedExternal string
			var locatedVault string
			v := &View{
				Doc:           doc,
				Registry:      BuiltinRegistry(),
				Mode:          ModeDesign,
				UnifiedLive:   true,
				MobileCompact: vp.mobile,
				BaseDir:       baseDir,
				Allowed:       map[string]bool{PermDocumentEdit: true},
				OnOpenExternalAsset: func(asset AssetThumb) {
					openedExternal = asset.FilePath
				},
				OnOpenVaultLink: func(relPath string) {
					locatedVault = relPath
				},
			}

			tt := ui.NewTester(func(c *ui.Context) {
				if vp.dark {
					c.SetTheme(theme.DarkTheme())
				} else {
					c.SetTheme(theme.LightTheme())
				}
				tok := theme.GetTokens(c.Theme())
				ui.Scroll(c.Key("fig-test-scroll")).Fill().Background(tok.Canvas).Children(func() {
					ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
						v.Render(c)
					})
				})
			}, vp.w, vp.h)

			if vp.mobile {
				tt.SetInsets(59, 0, 34, 0)
			}
			settleFigureFrames(tt, 180)

			if vp.name == "desktop" {
				saveFigureShotIfRequested(t, tt, "figure_card_desktop_dark_1180x760.png")
			} else {
				saveFigureShotIfRequested(t, tt, "figure_card_mobile_402x874.png")
				// Verify no horizontal overflow on 402dp mobile
				for _, info := range tt.AccessAll() {
					if info.Offscreen || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
						continue
					}
					if info.Bounds.X < -1.5 || info.Bounds.X+info.Bounds.W > 403.0 {
						t.Fatalf("mobile figure card overflow: name=%q bounds=%+v", info.Name, info.Bounds)
					}
				}
			}

			// 1. Open embed card DarkroomLightbox via "放大查看"
			if err := tt.Click("放大查看"); err != nil {
				t.Fatalf("click 放大查看 on embed card: %v (texts=%v)", err, tt.Texts())
			}
			settleFigureFrames(tt, 260)

			if !v.figureOpen {
				t.Fatalf("expected v.figureOpen=true after clicking 放大查看")
			}
			if !tt.HasText("PNG") || !tt.HasText("100%") || !tt.HasText("6h") || !tt.HasText("FOV 1") || !tt.HasText("中心视野血管成管") {
				t.Fatalf("expected DarkroomLightbox footer metadata/badge/zoom, texts=%v", tt.Texts())
			}
			if !tt.HasText("定位对象") || !tt.HasText("系统打开") || !tt.HasText("定位文件") {
				t.Fatalf("expected quiet darkroom actions, texts=%v", tt.Texts())
			}
			if luma := meanImageLuma(tt.Image()); luma > 65 {
				t.Fatalf("expected darkroom lightbox canvas mean luma < 65, got %.1f", luma)
			}

			if vp.name == "desktop" {
				saveFigureShotIfRequested(t, tt, "figure_lightbox_desktop_1180x760.png")
			} else {
				saveFigureShotIfRequested(t, tt, "figure_lightbox_mobile_402x874.png")
			}

			// Test quiet darkroom action callbacks
			if err := tt.Click("系统打开"); err != nil {
				t.Fatalf("click 系统打开: %v", err)
			}
			tt.Frame()
			if openedExternal == "" {
				t.Fatalf("expected OnOpenExternalAsset callback to fire")
			}
			if err := tt.Click("定位文件"); err != nil {
				t.Fatalf("click 定位文件: %v", err)
			}
			tt.Frame()
			if locatedVault != imgRel && locatedVault != "./assets/huvec_a1_6h.png" {
				t.Fatalf("expected OnOpenVaultLink callback with %q, got %q", imgRel, locatedVault)
			}

			// 2. Double-click stage center toggles 1x <-> 2x zoom
			cx, cy := float32(vp.w)*0.5, float32(vp.h)*0.45
			tt.ClickAt(cx, cy)
			tt.Advance(40 * time.Millisecond)
			tt.ClickAt(cx, cy)
			settleFigureFrames(tt, 120)
			if !tt.HasText("200%") {
				t.Fatalf("expected double-click to zoom to 200%%, texts=%v", tt.Texts())
			}
			tt.Advance(600 * time.Millisecond)
			tt.ClickAt(cx, cy)
			tt.Advance(40 * time.Millisecond)
			tt.ClickAt(cx, cy)
			settleFigureFrames(tt, 120)
			if !tt.HasText("100%") {
				t.Fatalf("expected second double-click to reset zoom to 100%%, texts=%v", tt.Texts())
			}

			// 3. Single-tap on canvas hides HUD chrome, second tap restores it
			tt.ClickAt(cx, cy)
			settleFigureFrames(tt, 220)
			if tt.HasText("100%") {
				t.Fatalf("expected single tap to hide HUD chrome, texts=%v", tt.Texts())
			}
			tt.ClickAt(cx, cy)
			settleFigureFrames(tt, 220)
			if !tt.HasText("100%") {
				t.Fatalf("expected second tap to restore HUD chrome, texts=%v", tt.Texts())
			}

			// 4. Vertical swipe-down dismisses DarkroomLightbox cleanly
			tt.Press(cx, cy-60)
			tt.Move(cx, cy+20)
			tt.Frame()
			tt.Move(cx, cy+160)
			tt.Frame()
			tt.Release(cx, cy+160)
			settleFigureFrames(tt, 320)
			if v.figureOpen {
				t.Fatalf("expected vertical swipe-down to dismiss lightbox")
			}

			// 5. Select well A1 on plate-12 and verify plate-well relation strip opens DarkroomLightbox
			v.SetPlateSelection("plate-12", []string{"A1"})
			settleFigureFrames(tt, 120)
			for step := 0; step < 12; step++ {
				if r, ok := tt.Find("6h · FOV 1 · huvec_a1_6h.png"); ok && r.Y >= 60 && r.Y+r.H <= float32(vp.h)-40 {
					break
				}
				tt.Scroll(float32(vp.w)*0.5, float32(vp.h)*0.5, 0, 220)
				settleFigureFrames(tt, 80)
			}
			if err := tt.Click("6h · FOV 1 · huvec_a1_6h.png"); err != nil {
				t.Fatalf("click plate well relation chip: %v (texts=%v)", err, tt.Texts())
			}
			settleFigureFrames(tt, 240)
			if !v.figureOpen || !tt.HasText("PNG") || !tt.HasText("100%") {
				t.Fatalf("expected plate well relation strip to open DarkroomLightbox, open=%v texts=%v", v.figureOpen, tt.Texts())
			}
			if err := tt.Click("关闭灯箱"); err != nil {
				t.Fatalf("click 关闭灯箱: %v", err)
			}
			settleFigureFrames(tt, 180)
			if v.figureOpen {
				t.Fatalf("expected 关闭灯箱 to close lightbox")
			}
		})
	}
}

func TestClassicRenderFigureUsesDarkroomLightbox(t *testing.T) {
	src := `---
title: Classic Figure Test
---

` + "```asterlab\nkind: figure\nid: fig-classic-1\nschema: 1\ntitle: 经典培养皿示意图\ncaption: Q1-Q4 药敏分区\nsvg: |\n  <svg viewBox=\"0 0 240 160\" xmlns=\"http://www.w3.org/2000/svg\">\n    <circle cx=\"120\" cy=\"80\" r=\"50\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"/>\n    <text x=\"120\" y=\"84\" text-anchor=\"middle\" font-size=\"12\">琼脂皿</text>\n  </svg>\n```\n"

	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	v := &View{
		Doc:         doc,
		Registry:    BuiltinRegistry(),
		Mode:        ModeDesign,
		UnifiedLive: false,
		Allowed:     map[string]bool{PermDocumentEdit: true},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.Render(c)
	}, 1180, 760)
	settleFigureFrames(tt, 120)

	if err := tt.Click("放大查看"); err != nil {
		t.Fatalf("click 放大查看 on classic figure: %v", err)
	}
	settleFigureFrames(tt, 240)
	if !v.figureOpen || !tt.HasText("SVG") || !tt.HasText("经典培养皿示意图") || !tt.HasText("Q1-Q4 药敏分区") || !tt.HasText("100%") {
		t.Fatalf("expected classic renderFigure to open DarkroomLightbox with SVG badge and caption, open=%v texts=%v", v.figureOpen, tt.Texts())
	}
	if err := tt.Click("放大"); err != nil {
		t.Fatalf("click 放大 in DarkroomLightbox: %v", err)
	}
	settleFigureFrames(tt, 100)
	if !tt.HasText("150%") {
		t.Fatalf("expected zoom 150%% after clicking 放大, texts=%v", tt.Texts())
	}
	if err := tt.Click("关闭灯箱"); err != nil {
		t.Fatalf("click 关闭灯箱: %v", err)
	}
	settleFigureFrames(tt, 150)
	if v.figureOpen {
		t.Fatalf("expected classic renderFigure lightbox closed")
	}
}
