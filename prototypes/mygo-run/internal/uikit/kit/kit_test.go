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

func TestIconBoxOpticalBounds(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			IconBox(c, tok, lucide.FlaskConical(), tok.Accent).Label("icon-slot-sample")
		})
	}, 200, 100)
	tt.Frame()
	r, ok := tt.Find("icon-slot-sample")
	if !ok || r.W != 24 || r.H != 24 {
		t.Fatalf("expected 24x24 optical IconBox, got ok=%v rect=%+v", ok, r)
	}
}

func TestSafeTriangleGeometry(t *testing.T) {
	origin := [2]float32{100, 120}
	topEdge := [2]float32{220, 80}
	botEdge := [2]float32{220, 220}
	if !pointInSafeTriangle([2]float32{160, 150}, origin, topEdge, botEdge) {
		t.Fatal("expected midpoint inside right-opening safe triangle")
	}
	if pointInSafeTriangle([2]float32{160, 50}, origin, topEdge, botEdge) {
		t.Fatal("expected upper point outside safe triangle")
	}
	if !pointInSafeTriangle([2]float32{220, 80}, origin, topEdge, botEdge) {
		t.Fatal("expected vertex on boundary to count as inside")
	}
	// Left-opening submenu triangle
	leftTop := [2]float32{20, 80}
	leftBot := [2]float32{20, 220}
	if !pointInSafeTriangle([2]float32{60, 150}, origin, leftTop, leftBot) {
		t.Fatal("expected midpoint inside left-opening safe triangle")
	}
}

func TestContextMenuSubmenuOpenDelaySafeTriangleAndCloseDelay(t *testing.T) {
	chosen := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		box := ui.Column(c.Key("sub-test-target")).Size(320, 240).Padding(float32(tok.Spacing12))
		box.Children(func() {
			ui.Text(c, "右键触发多级菜单").FontSize(float32(tok.TextData)).TextColor(tok.Text)
		})
		ContextMenu(c, box, func(m *Menu) {
			m.Submenu("导出与格式", func(sub *Menu) {
				if sub.Item("导出 PDF").Chosen() {
					chosen = "pdf"
				}
				if sub.Item("导出 CSV").Chosen() {
					chosen = "csv"
				}
			})
			if m.Item("重命名实验").Chosen() {
				chosen = "rename"
			}
		})
	}, 720, 420)
	tt.Frame()

	targetR, ok := tt.Find("右键触发多级菜单")
	if !ok {
		t.Fatalf("missing context menu target")
	}
	tt.RightClickAt(targetR.X+20, targetR.Y+10)
	tt.Advance(60 * time.Millisecond)
	tt.Frame()

	subRow, ok := tt.Find("导出与格式")
	if !ok {
		t.Fatalf("expected parent submenu trigger, texts=%v", tt.Texts())
	}
	renameRow, ok := tt.Find("重命名实验")
	if !ok {
		t.Fatalf("expected sibling menu item, texts=%v", tt.Texts())
	}

	// Hover trigger -> before 120ms open delay expires, submenu should not open yet
	tt.Move(subRow.X+subRow.W*0.5, subRow.Y+subRow.H*0.5)
	tt.Frame()
	if tt.HasText("导出 PDF") {
		t.Fatalf("submenu opened prematurely before open delay")
	}

	// Advance past 120ms open delay -> submenu opens
	tt.Advance(140 * time.Millisecond)
	tt.Frame()
	pdfR, ok := tt.Find("导出 PDF")
	if !ok {
		t.Fatalf("expected submenu items after open delay, texts=%v", tt.Texts())
	}

	// Move diagonally toward submenu through sibling row area inside Safe Triangle:
	// submenu must remain open even past close delay while cursor is in triangle corridor.
	tt.Move(renameRow.X+renameRow.W*0.85, renameRow.Y+renameRow.H*0.5)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("导出 PDF") {
		t.Fatalf("expected submenu to stay open inside Safe Triangle corridor, texts=%v", tt.Texts())
	}

	// Move clearly outside Safe Triangle onto left side of sibling -> closes after close delay
	tt.Move(renameRow.X+12, renameRow.Y+renameRow.H*0.5)
	tt.Frame()
	if !tt.HasText("导出 PDF") {
		t.Fatalf("expected submenu to honor close delay before closing immediately")
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if tt.HasText("导出 PDF") {
		t.Fatalf("expected submenu to close after leaving Safe Triangle + close delay")
	}

	// Re-open submenu via click and trigger nested action
	_ = pdfR
	if err := tt.Click("导出与格式"); err != nil {
		t.Fatalf("click submenu trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("导出 CSV"); err != nil {
		t.Fatalf("click nested submenu action: %v", err)
	}
	tt.Frame()
	if chosen != "csv" || tt.HasText("导出与格式") {
		t.Fatalf("expected nested choice csv and menu dismissal, chosen=%q texts=%v", chosen, tt.Texts())
	}
}
