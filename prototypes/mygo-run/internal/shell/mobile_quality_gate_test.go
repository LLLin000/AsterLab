package shell

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/vault"
)

func imgLumaStats(img *image.RGBA) (median, p99 float64) {
	b := img.Bounds()
	var lumas []float64
	stepX := max(1, b.Dx()/80)
	stepY := max(1, b.Dy()/160)
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			c := img.RGBAAt(x, y)
			l := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
			lumas = append(lumas, l)
		}
	}
	if len(lumas) == 0 {
		return 0, 0
	}
	sort.Float64s(lumas)
	median = lumas[len(lumas)/2]
	p99Idx := int(float64(len(lumas)-1) * 0.99)
	p99 = lumas[p99Idx]
	return median, p99
}

func maybeExportVerifyPNG(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	dir := os.Getenv("ASTERLAB_MOBILE_VERIFY_DIR")
	if dir == "" || img == nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir verify dir: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("create verify png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode verify png: %v", err)
	}
}

func TestMobileSettingsBrightnessParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{
		{"dark", true},
		{"light", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srcMode := "light"
			if tc.dark {
				srcMode = "dark"
			}
			s := &Shell{
				TodayDate:          "2026-11-26",
				SelectedDate:       "2026-11-26",
				ForceMobilePreview: true,
				ThemeSource:        srcMode,
			}
			s.OpenFileInPane(defaultPaneID, "proj/huvec.asterml", "HUVEC 血管新生实验", ModeLab, false)

			tt := ui.NewTesterWith(func(c *ui.Context) {
				s.RenderSplitView(c)
			}, 402, 874, ui.TesterOptions{Dark: tc.dark})
			tt.SetInsets(44, 0, 34, 0)
			tt.Frame()
			advanceMobileFrames(tt, 260*time.Millisecond)
			mainImg := tt.Image()
			mainMed, mainP99 := imgLumaStats(mainImg)
			if tc.dark {
				maybeExportVerifyPNG(t, "01_main_dark.png", mainImg)
			}

			s.VaultSettingsOpen = true
			tt.Frame()
			advanceMobileFrames(tt, 380*time.Millisecond)
			setImg := tt.Image()
			setMed, setP99 := imgLumaStats(setImg)
			if tc.dark {
				maybeExportVerifyPNG(t, "02_settings_dark.png", setImg)
			} else {
				maybeExportVerifyPNG(t, "03_settings_light.png", setImg)
			}

			// If scrim is accidentally rendered ABOVE Settings panel, Light drops ~140 RGB
			// and Dark white text drops from ~230 to ~105. Both fail strictly here.
			diff := mainMed - setMed
			if diff < 0 {
				diff = -diff
			}
			if diff > 18.0 {
				t.Fatalf("%s Settings background brightness drifted from main canvas (mainMed=%.1f, setMed=%.1f, diff=%.1f)",
					tc.name, mainMed, setMed, diff)
			}
			if tc.dark && setP99 < 125.0 {
				t.Fatalf("dark Settings appears darkened by scrim overlay (mainP99=%.1f, setP99=%.1f)", mainP99, setP99)
			}
			if !tc.dark && setP99 < 230.0 {
				t.Fatalf("light Settings peak surface appears darkened by scrim overlay (setP99=%.1f)", setP99)
			}
		})
	}
}

func TestMobileDrawerControlsAndVaultManagerGate(t *testing.T) {
	s := &Shell{
		TodayDate:          "2026-11-26",
		SelectedDate:       "2026-11-26",
		ForceMobilePreview: true,
		ThemeSource:        "dark",
		VaultPathInput:     "/Users/lab/CellPlannerVault",
		MobileDrawer:       "tree",
		VaultFiles: []vault.FileEntry{
			{RelPath: "proj/cck8.asterml", Name: "cck8.asterml", Title: "CCK-8 细胞增殖"},
			{RelPath: "Templates/cck8-tpl.asterml", Name: "cck8-tpl.asterml", Title: "CCK-8 标准模板", IsTemplate: true},
		},
		VaultTree: &vault.TreeNode{
			IsDir: true,
			Children: []*vault.TreeNode{
				{
					RelPath: "proj/cck8.asterml",
					Name:    "cck8.asterml",
					Entry:   &vault.FileEntry{RelPath: "proj/cck8.asterml", Name: "cck8.asterml", Title: "CCK-8 细胞增殖"},
				},
				{
					RelPath: "Templates",
					Name:    "Templates",
					IsDir:   true,
					IsSystem: true,
					Children: []*vault.TreeNode{
						{
							RelPath: "Templates/cck8-tpl.asterml",
							Name:    "cck8-tpl.asterml",
							Entry:   &vault.FileEntry{RelPath: "Templates/cck8-tpl.asterml", Name: "cck8-tpl.asterml", Title: "CCK-8 标准模板", IsTemplate: true},
						},
					},
				},
			},
		},
	}
	s.OpenCalendarTab()

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 360*time.Millisecond)
	maybeExportVerifyPNG(t, "04_left_drawer_vault_mgr.png", tt.Image())

	// 1. Clicking "实验列表" in Left Drawer toggles template filter and MUST NOT jump to Right Drawer ("outline").
	if !tt.HasText("cck8.asterml") || !tt.HasText("cck8-tpl.asterml") {
		t.Fatalf("expected both experiment and template files initially, got %v", tt.Texts())
	}
	if err := tt.Click("实验列表"); err != nil {
		t.Fatalf("click 实验列表 pill: %v", err)
	}
	advanceMobileFrames(tt, 120*time.Millisecond)
	if s.MobileDrawer != "tree" {
		t.Fatalf("left drawer 实验列表 jumped unexpectedly to %q; must stay in tree", s.MobileDrawer)
	}
	if !tt.HasText("仅看模板") {
		t.Fatalf("expected filter pill to read 仅看模板, got %v", tt.Texts())
	}
	if tt.HasText("cck8.asterml") || !tt.HasText("cck8-tpl.asterml") {
		t.Fatalf("expected 仅看模板 to filter non-template files, texts=%v", tt.Texts())
	}
	if err := tt.Click("仅看模板"); err != nil {
		t.Fatalf("click 仅看模板 pill: %v", err)
	}
	advanceMobileFrames(tt, 120*time.Millisecond)
	if !tt.HasText("实验列表") || !tt.HasText("cck8.asterml") {
		t.Fatalf("expected clicking 仅看模板 again to restore all files, texts=%v", tt.Texts())
	}

	// 2. Clicking Vault Manager trigger ("管理实验库 CellPlannerVault") opens mobile Vault Launcher.
	if err := tt.Click("管理实验库 CellPlannerVault"); err != nil {
		t.Fatalf("click vault manager trigger: %v", err)
	}
	advanceMobileFrames(tt, 380*time.Millisecond)
	if !s.VaultLauncherOpen || s.MobileDrawer != "" {
		t.Fatalf("expected VaultLauncherOpen=true and drawer closed, got VaultLauncherOpen=%v MobileDrawer=%q",
			s.VaultLauncherOpen, s.MobileDrawer)
	}
	if !tt.HasText("选择你的实验库目录") {
		t.Fatalf("expected mobile Vault Launcher page to render, got %v", tt.Texts())
	}
}

func TestMobileModeToggleAndNoDesktopMenuGate(t *testing.T) {
	s := &Shell{
		TodayDate:          "2026-11-26",
		ForceMobilePreview: true,
		ThemeSource:        "dark",
	}
	s.OpenFileInPane(defaultPaneID, "proj/exp.asterml", "CCK-8", ModeLab, false)

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	tt.Frame()
	advanceMobileFrames(tt, 200*time.Millisecond)

	for _, forbidden := range []string{"Select Next Occurrence", "Select All Occurrences", "Expand Selection"} {
		if tt.HasText(forbidden) {
			t.Fatalf("desktop IDE menu text %q leaked into mobile UI", forbidden)
		}
	}

	if err := tt.Click("切换到编辑态"); err != nil {
		t.Fatalf("click mode toggle to edit: %v", err)
	}
	tt.Frame()
	if s.ActiveTab().Mode != ModeEdit {
		t.Fatalf("expected ModeEdit, got %q", s.ActiveTab().Mode)
	}
	if err := tt.Click("切换到实验模式（防误触）"); err != nil {
		t.Fatalf("click mode toggle to lab: %v", err)
	}
	tt.Frame()
	if s.ActiveTab().Mode != ModeLab {
		t.Fatalf("expected ModeLab, got %q", s.ActiveTab().Mode)
	}
}

func TestMobileLabReadOnlyNoCursorAndFormatCommands(t *testing.T) {
	src := "检测 VEGF 处理组[^1]细胞活力变化。\n\n[^1]: 加入无菌 PBS\n"
	doc, err := asterdoc.Parse(src)
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}
	view := &asterdoc.View{
		Doc:           doc,
		Mode:          asterdoc.ModeDesign,
		UnifiedLive:   true,
		LabMode:       true,
		MobileCompact: true,
		Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
		OnChange: func(next string) {
			if nd, err := asterdoc.Parse(next); err == nil {
				doc = nd
			}
		},
	}

	tt := ui.NewTesterWith(func(c *ui.Context) {
		view.Doc = doc
		c.SetTheme(theme.DarkTheme())
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(16).Children(func() {
				view.Render(c)
			})
		})
	}, 402, 874, ui.TesterOptions{Dark: true})
	tt.Frame()

	// 1. In ModeLab (read-only), single tap + typing MUST NOT activate IME or alter text.
	if err := tt.Click("科学笔记正文"); err != nil {
		t.Fatalf("tap ModeLab prose: %v", err)
	}
	tt.Frame()
	tt.Type("NOPE")
	tt.Frame()
	if active, _ := tt.TextInputActive(); active {
		t.Fatalf("expected TextInputActive=false in ModeLab")
	}
	if strings.Contains(doc.Source, "NOPE") {
		t.Fatalf("ModeLab unexpectedly allowed edit: %q", doc.Source)
	}

	// 2. Switch to Edit mode: double-click word then apply native InputCommand format actions.
	view.LabMode = false
	tt.Frame()
	r, ok := tt.Find("科学笔记正文")
	if !ok {
		t.Fatalf("expected live editor element")
	}
	// Tap to focus prose, then run format commands via InputCommand (same dispatch path as UIEditMenuInteraction).
	tt.ClickAt(r.X+42, r.Y+14)
	tt.Frame()
	tt.Command("selectAll")
	tt.Frame()
	tt.Command("formatBold")
	tt.Frame()
	if !strings.Contains(doc.Source, "**") {
		t.Fatalf("expected formatBold InputCommand to wrap text in **, got %q", doc.Source)
	}
	tt.Command("formatFootnote")
	tt.Frame()
	if !strings.Contains(doc.Source, "[^2]") {
		t.Fatalf("expected formatFootnote InputCommand to insert [^2], got %q", doc.Source)
	}
}

func TestMobileScrollFrameBudgetGate(t *testing.T) {
	s := &Shell{
		TodayDate:          "2026-11-26",
		SelectedDate:       "2026-11-26",
		ForceMobilePreview: true,
		ThemeSource:        "dark",
	}
	s.OpenFileInPane(defaultPaneID, "proj/huvec.asterml", "HUVEC 细胞成管与 CCK-8", ModeLab, false)

	tt := ui.NewTesterWith(func(c *ui.Context) {
		s.RenderSplitView(c)
	}, 402, 874, ui.TesterOptions{Dark: true})
	tt.SetInsets(44, 0, 34, 0)
	// Warmup caches & layout
	for range 8 {
		tt.Frame()
	}

	const frames = 60
	durations := make([]time.Duration, 0, frames)
	for i := 0; i < frames; i++ {
		dy := float32(-24)
		if i%20 >= 10 {
			dy = 24
		}
		start := time.Now()
		tt.Wheel(200, 420, 0, dy)
		tt.Frame()
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	med := durations[len(durations)/2]
	p95 := durations[int(float64(len(durations)-1)*0.95)]
	t.Logf("Mobile scroll frame time over %d frames: median=%v p95=%v max=%v",
		frames, med, p95, durations[len(durations)-1])
	if med > 20*time.Millisecond {
		t.Fatalf("scroll frame median latency too high (%v > 20ms)", med)
	}
}
