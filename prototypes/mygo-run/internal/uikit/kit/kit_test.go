package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

func TestKitListItemKeycapAndContextMenuDismissInside(t *testing.T) {
	clicked := false
	selectedIdx := -1

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		card := ui.Column(c.Key("kit-test-card")).Size(460, 220).Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
		ApplyElevation(card, tok, ElevationCard)
		card.Children(func() {
			SectionHeader(c, tok, lucide.FlaskConical(), "Kit 测试卡片", "2 项", func() {
				Keycap(c, tok, "Ctrl+K")
			})
			ListItem(c, "row-1", tok, ListItemOptions{
				Title:    "实验样本 A",
				Subtitle: "C57BL/6J · SPF",
				Selected: selectedIdx == 0,
				OnClick: func() {
					clicked = true
					selectedIdx = 0
				},
			})
		})
		ContextMenu(c, card, func(m *Menu) {
			_ = m.Item("菜单操作一").Shortcut("Ctrl+D").Chosen()
			_ = m.Item("菜单操作二").Chosen()
		})
	}, 600, 320)

	tt.Frame()
	if !tt.HasText("Ctrl") || !tt.HasText("K") {
		t.Fatalf("expected structured Keycap pills Ctrl + K, got texts=%v", tt.Texts())
	}

	hdr, ok := tt.Find("Kit 测试卡片")
	if !ok {
		t.Fatalf("expected SectionHeader in card")
	}
	ctrlR, ok := tt.Find("K")
	if !ok {
		t.Fatalf("expected trailing Keycap inside card")
	}

	// Open context menu via right-click inside card title area
	tt.RightClickAt(hdr.X+12, hdr.Y+hdr.H/2)
	tt.Advance(180 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("菜单操作一") {
		t.Fatalf("expected kit.ContextMenu to open on right-click")
	}

	// Left-click trailing "K" keycap inside SAME card (strictly outside popup panel) -> must dismiss!
	tt.ClickAt(ctrlR.X+ctrlR.W/2, ctrlR.Y+ctrlR.H/2)
	tt.Advance(180 * time.Millisecond)
	tt.Frame()
	if tt.HasText("菜单操作一") {
		t.Fatalf("kit.ContextMenu failed to dismiss on left-click inside same target card (hdr=%+v ctrl=%+v)", hdr, ctrlR)
	}

	if err := tt.Click("实验样本 A"); err != nil {
		t.Fatalf("click ListItem: %v", err)
	}
	tt.Frame()
	if !clicked || selectedIdx != 0 {
		t.Fatalf("expected ListItem click handler to fire, clicked=%v idx=%d", clicked, selectedIdx)
	}
}

func TestRenderComponentGalleryLightAndDark(t *testing.T) {
	st := &GalleryState{SampleNum: 10}
	for _, dark := range []bool{false, true} {
		tt := ui.NewTester(func(c *ui.Context) {
			if dark {
				c.SetTheme(theme.DarkTheme())
			} else {
				c.SetTheme(theme.LightTheme())
			}
			RenderComponentGallery(c, st)
		}, 980, 720)
		tt.SetDark(dark)
		tt.Frame()
		if !tt.HasText("AsterLab 统一 UI 组件画廊") || !tt.HasText("实验白小鼠") || !tt.HasText("斑马鱼") {
			t.Fatalf("gallery render missing key sections/animals (dark=%v), texts=%v", dark, tt.Texts())
		}
	}
}
