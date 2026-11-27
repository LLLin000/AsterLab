package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

func TestMiddleTruncateUnicodeAndExtensions(t *testing.T) {
	if got := MiddleTruncate("abc.dng", 0); got != "" {
		t.Fatalf("expected empty for maxRunes<=0, got %q", got)
	}
	if got := MiddleTruncate("short.dng", 20); got != "short.dng" {
		t.Fatalf("expected short string unchanged, got %q", got)
	}

	gotStress := MiddleTruncate(craftStressFilename, 28)
	if utf8.RuneCountInString(gotStress) != 28 {
		t.Fatalf("expected 28 runes, got %d (%q)", utf8.RuneCountInString(gotStress), gotStress)
	}
	if !strings.HasPrefix(gotStress, "20261127_") || !strings.HasSuffix(gotStress, ".dng") || !strings.Contains(gotStress, "…") {
		t.Fatalf("expected prefix, ellipsis, and .dng suffix preserved, got %q", gotStress)
	}

	// Mixed Chinese + ASCII filename must stay valid UTF-8 and respect rune budget.
	mixed := "20261127_血管内皮细胞_VEGF50ng_GroupB_Plate02_WellA01_6h_FOV02.tiff"
	gotMixed := MiddleTruncate(mixed, 24)
	if !utf8.ValidString(gotMixed) {
		t.Fatalf("invalid utf-8 from MiddleTruncate: %q", gotMixed)
	}
	if utf8.RuneCountInString(gotMixed) > 24 || !strings.HasSuffix(gotMixed, ".tiff") {
		t.Fatalf("expected <=24 runes preserving .tiff, got %q", gotMixed)
	}
}

func TestShortcutKeycapsSkeletonAndEmptyStateCard(t *testing.T) {
	acted := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing12)).Children(func() {
			ShortcutKeycaps(c, tok, "Ctrl", "Shift", "K")
			SkeletonCard(c, tok)
			EmptyStateCard(c, tok, EmptyStateProps{
				Icon:        lucide.FlaskConical(),
				Title:       "暂无共聚焦样本",
				Hint:        "拖入 DNG 文件或使用快捷键创建",
				ActionLabel: "新建记录",
				Shortcuts:   []string{"Ctrl", "N"},
				OnAction:    func() { acted = true },
			})
		})
	}, 520, 480)

	tt.Frame()
	for _, want := range []string{"Ctrl", "Shift", "K", "暂无共聚焦样本", "新建记录", "N"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in rendered primitives, texts=%v", want, tt.Texts())
		}
	}
	if err := tt.Click("新建记录"); err != nil {
		t.Fatalf("click EmptyStateCard action: %v", err)
	}
	tt.Frame()
	if !acted {
		t.Fatal("expected EmptyStateCard OnAction callback to fire")
	}
}

func TestMorphActionButtonWidthStabilityAndPhaseCycle(t *testing.T) {
	st := MorphActionState{Phase: MorphIdle}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Row(c).Padding(float32(tok.Spacing12)).Children(func() {
			if MorphActionButton(c, tok, &st, MorphActionOptions{
				Key:       "test-morph",
				IdleLabel: "归档数据",
				BusyLabel: "归档校验中…",
				DoneLabel: "已归档",
				IdleIcon:  lucide.Archive(),
				Primary:   true,
			}) {
				st.Phase = MorphBusy
			}
		})
	}, 400, 120)

	tt.Frame()
	idleRect, ok := tt.Find("归档数据")
	if !ok || idleRect.W <= 0 {
		t.Fatalf("expected MorphIdle button rect, ok=%v rect=%+v", ok, idleRect)
	}

	if err := tt.Click("归档数据"); err != nil {
		t.Fatalf("click MorphActionButton: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if st.Phase != MorphBusy {
		t.Fatalf("expected phase MorphBusy after click, got %v", st.Phase)
	}
	busyRect, ok := tt.Find("归档校验中…")
	if !ok {
		t.Fatalf("expected MorphBusy label, texts=%v", tt.Texts())
	}
	if busyRect.W != idleRect.W {
		t.Fatalf("expected width-stable MorphActionButton across Idle->Busy: idle=%.1f busy=%.1f", idleRect.W, busyRect.W)
	}

	st.Phase = MorphDone
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	doneRect, ok := tt.Find("已归档")
	if !ok {
		t.Fatalf("expected MorphDone label, texts=%v", tt.Texts())
	}
	if doneRect.W != idleRect.W {
		t.Fatalf("expected width-stable MorphActionButton across Idle->Done: idle=%.1f done=%.1f", idleRect.W, doneRect.W)
	}
}

func TestSlideConfirmDragSnapBackConfirmAndKeyboard(t *testing.T) {
	var st SlideConfirmState
	confirmedCount := 0

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Children(func() {
			if SlideConfirm(c, tok, &st, SlideConfirmOptions{
				Key:            "test-slide",
				Label:          "滑动确认销毁",
				ConfirmedLabel: "已确认销毁",
				Danger:         true,
				Width:          300,
			}) {
				confirmedCount++
			}
		})
	}, 360, 140)

	tt.Frame()
	handleInfo, ok := tt.Access("滑动确认销毁")
	if !ok {
		t.Fatalf("expected accessible handle for SlideConfirm, all=%+v", tt.AccessAll())
	}
	startX := handleInfo.Bounds.X + handleInfo.Bounds.W/2
	startY := handleInfo.Bounds.Y + handleInfo.Bounds.H/2

	// 1. Drag only ~40% (< 82% threshold) -> must snap back and stay unconfirmed.
	tt.Press(startX, startY)
	tt.Move(startX+105, startY)
	tt.Frame()
	tt.Release(startX+105, startY)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if st.Confirmed || confirmedCount != 0 {
		t.Fatalf("expected drag <82%% to snap back unconfirmed, confirmed=%v count=%d progress=%.2f", st.Confirmed, confirmedCount, st.Progress)
	}

	// 2. Drag >= 82% -> must lock confirmed and increment confirmedCount once.
	tt.Press(startX, startY)
	tt.Move(startX+250, startY)
	tt.Frame()
	tt.Release(startX+250, startY)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if !st.Confirmed || confirmedCount != 1 || !tt.HasText("已确认销毁") {
		t.Fatalf("expected drag >=82%% to confirm, confirmed=%v count=%d texts=%v", st.Confirmed, confirmedCount, tt.Texts())
	}

	// 3. Reset and verify keyboard Enter confirms when handle is focused.
	st.Reset()
	tt.Frame()
	if st.Confirmed {
		t.Fatal("expected Reset() to clear Confirmed")
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !st.Confirmed || confirmedCount != 2 {
		t.Fatalf("expected keyboard Enter on focused handle to confirm, confirmed=%v count=%d", st.Confirmed, confirmedCount)
	}
}

func TestFloatingSelectionBarAndClear(t *testing.T) {
	count := 3
	exported := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Children(func() {
			FloatingSelectionBar(c, tok, FloatingSelectionBarProps{
				Key:        "test-fsb",
				Count:      count,
				UnitLabel:  "项已选",
				ClearLabel: "清空",
				OnClear:    func() { count = 0 },
				Actions: []FloatingSelectionAction{
					{ID: "exp", Label: "批量导出", Primary: true, OnClick: func() { exported = true }},
				},
			})
		})
	}, 480, 140)

	tt.Frame()
	if !tt.HasText("3") || !tt.HasText("项已选") || !tt.HasText("批量导出") {
		t.Fatalf("expected FloatingSelectionBar rendered, texts=%v", tt.Texts())
	}
	if err := tt.Click("批量导出"); err != nil {
		t.Fatalf("click FloatingSelectionBar action: %v", err)
	}
	tt.Frame()
	if !exported {
		t.Fatal("expected FloatingSelectionBar action callback to fire")
	}
	if err := tt.Click("清空"); err != nil {
		t.Fatalf("click FloatingSelectionBar clear: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if count != 0 || tt.HasText("项已选") {
		t.Fatalf("expected FloatingSelectionBar dismissed after clear, count=%d texts=%v", count, tt.Texts())
	}
}

func TestToastQueueCountdownProgressRatio(t *testing.T) {
	now := time.Now()
	auto := Toast{Duration: 5 * time.Second, createdAt: now.Add(-2 * time.Second)}
	if r := toastRemainingRatio(auto, now); r < 0.59 || r > 0.61 {
		t.Fatalf("expected ~0.60 remaining ratio, got %.3f", r)
	}
	det := Toast{Duration: 5 * time.Second, ProgressOverride: 0.68, createdAt: now}
	if r := toastRemainingRatio(det, now.Add(4*time.Second)); r != 0.68 {
		t.Fatalf("expected ProgressOverride=0.68, got %.3f", r)
	}
	clamped := Toast{Duration: 5 * time.Second, ProgressOverride: 1.5, createdAt: now}
	if r := toastRemainingRatio(clamped, now); r != 1.0 {
		t.Fatalf("expected clamped ProgressOverride=1.0, got %.3f", r)
	}

	q := NewToastQueue()
	q.Push(Toast{
		Tone:             ToastWarning,
		Message:          "已删除 3 张暗房印样",
		UndoLabel:        "撤销",
		ProgressOverride: 0.65,
		Duration:         30 * time.Second,
		OnUndo:           func() {},
	})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Children(func() {
			q.Render(c, tok)
		})
	}, 460, 220)
	tt.Frame()
	if !tt.HasText("已删除 3 张暗房印样") || !tt.HasText("撤销") {
		t.Fatalf("expected undo toast rendered, texts=%v", tt.Texts())
	}
}

func TestDarkroomLightboxMiddleTruncatesLongTitle(t *testing.T) {
	open := true
	st := DarkroomLightboxState{Zoom: 1.0}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Children(func() {
			DarkroomLightbox(c, tok, &open, &st, DarkroomLightboxProps{
				Key:         "trunc-lb",
				Title:       craftStressFilename,
				FormatBadge: "DNG",
			})
		})
	}, 402, 874)

	tt.Frame()
	wantTitle := MiddleTruncate(craftStressFilename, 28)
	if !tt.HasText(wantTitle) {
		t.Fatalf("expected MiddleTruncate(28) title %q in mobile lightbox, texts=%v", wantTitle, tt.Texts())
	}
}

func TestGalleryCraftWorkshopAndStressModeZeroOverflow402x874(t *testing.T) {
	st := &GalleryState{
		ActiveSection: "craft-workshop",
		StressMode:    true,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		RenderComponentGallery(c, st)
	}, 402, 874)

	tt.Frame()
	if !tt.HasText("3E. Topinspo 工艺工坊") || !tt.HasText("极限压力") || !tt.HasText("999+ uL") {
		t.Fatalf("expected Craft Workshop & StressMode active, texts=%v", tt.Texts())
	}

	// Verify no rendered accessible control/text outside horizontal scroll strips overflows 402px width.
	stripHdr, hasStrip := tt.Find("ApplyEdgeFadeX + ScrollSnap(SnapMandatory) 印样横滑条")
	for _, info := range tt.AccessAll() {
		if info.Bounds.W <= 0 || info.Offscreen {
			continue
		}
		// Skip buttons inside the compact mobile horizontal nav bar (Y ~ 245..295)
		// and specimen cards inside the horizontal ScrollSnap strip (which intentionally scroll on X).
		if info.Bounds.Y >= 245 && info.Bounds.Y <= 295 {
			continue
		}
		if hasStrip && info.Bounds.Y >= stripHdr.Y && info.Bounds.Y <= stripHdr.Y+130 {
			continue
		}
		if info.Bounds.X+info.Bounds.W > 403 {
			t.Fatalf("horizontal overflow at 402x874 on %q (%v): rect=%+v", info.Name, info.Role, info.Bounds)
		}
	}

	// Toggle back to normal mode via toolbar button.
	if err := tt.Click("极限压力"); err != nil {
		t.Fatalf("click stress toggle: %v", err)
	}
	tt.Frame()
	if st.StressMode || !tt.HasText("正常数据") {
		t.Fatalf("expected toggle back to normal mode, StressMode=%v texts=%v", st.StressMode, tt.Texts())
	}
}

func TestExportTopinspoCraftScreenshotsWhenRequested(t *testing.T) {
	dir := os.Getenv("ASTERLAB_TOPINSPO_SHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	saveShot := func(name string, tt *ui.Tester) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name))
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

	// 1. Dark 1360x880 (Topinspo 工艺工坊)
	{
		st := &GalleryState{
			ActiveSection:      "craft-workshop",
			ThemeOverride:      "dark",
			CraftCounter:       18,
			CraftSelectedCount: 4,
			CraftInit:          true,
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			RenderComponentGallery(c, st)
		}, 1360, 880, ui.TesterOptions{Dark: true})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("topinspo_gallery_dark_1360x880.png", tt)
	}

	// 2. Light 1360x880 (Topinspo 工艺工坊)
	{
		st := &GalleryState{
			ActiveSection:      "craft-workshop",
			ThemeOverride:      "light",
			CraftCounter:       18,
			CraftSelectedCount: 4,
			CraftInit:          true,
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			RenderComponentGallery(c, st)
		}, 1360, 880, ui.TesterOptions{Dark: false})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("topinspo_gallery_light_1360x880.png", tt)
	}

	// 3. Stress Mode 402x874 (break-ui worst-case mobile viewport)
	{
		st := &GalleryState{
			ActiveSection:      "craft-workshop",
			ThemeOverride:      "dark",
			StressMode:         true,
			CraftCounter:       999,
			CraftSelectedCount: 999,
			CraftInit:          true,
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			RenderComponentGallery(c, st)
		}, 402, 874, ui.TesterOptions{Dark: true})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("topinspo_gallery_stress_402x874.png", tt)
	}

	// 4. Undo Toast + FloatingSelectionBar 1360x880
	{
		q := NewToastQueue()
		q.Push(Toast{
			Tone:             ToastWarning,
			Message:          "已移除 4 张共聚焦显微印样",
			UndoLabel:        "撤销",
			Duration:         30 * time.Second,
			ProgressOverride: 0.68,
			OnUndo:           func() {},
		})
		st := &GalleryState{
			ActiveSection:      "craft-workshop",
			ThemeOverride:      "dark",
			CraftCounter:       24,
			CraftSelectedCount: 4,
			CraftInit:          true,
			DemoToasts:         q,
		}
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			RenderComponentGallery(c, st)
		}, 1360, 880, ui.TesterOptions{Dark: true})
		tt.Frame()
		// Scroll preview canvas down so FloatingSelectionBar, SkeletonCard, EmptyStateCard,
		// and the bottom-right Undo Toast countdown hairline are all clearly in view.
		tt.Scroll(760, 500, 0, 360)
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("topinspo_gallery_undo_floating_1360x880.png", tt)
	}
}
