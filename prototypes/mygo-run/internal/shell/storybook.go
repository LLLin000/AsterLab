package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
)

// BuiltinShellStorySection builds the Global Shell Chrome Storybook section
// without creating circular imports inside kit.
func (s *Shell) BuiltinShellStorySection() kit.StorySection {
	return kit.StorySection{
		ID:       "shell-chrome",
		Group:    "全局壳层",
		NavTitle: "全局工作台壳层",
		Title:    "4. 全局工作台壳层（Shell Chrome & Navigation）",
		Subtitle: "多窗格激活紫条 · Pinned/Dirty 标签 · 文件树引导线 · 状态栏 · 移动端浮岛",
		Icon:     lucide.AppWindow(),
		Render: func(c *ui.Context, tok theme.Tokens) {
			card := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
			kit.ApplyElevation(card, tok, kit.ElevationCard)
			card.Children(func() {
				kit.SectionHeader(c, tok, lucide.AppWindow(), "4A. 顶栏标签页、分屏与模式头",
					"Obsidian 平权分屏视觉样本（当前实机组件映射）", nil)

				// Top tab strip mockup using real tokens
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
					Border(1, tok.BorderSubtle).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.ArrowLeft()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
					ui.Icon(c, lucide.ArrowRight()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
					tabs := []struct {
						lbl            string
						active, pinned bool
						ico            *ui.SVG
					}{
						{"实验日程", false, true, lucide.CalendarDays()},
						{"HAMA 水凝胶抗菌.asterml", true, false, lucide.FlaskConical()},
						{"储藏室 · 动物房", false, false, lucide.PawPrint()},
					}
					for idx, tb := range tabs {
						box := ui.Column(c.Key(fmt.Sprintf("st-tab-%d", idx))).
							Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius))
						if tb.active {
							box.Background(tok.Surface).Border(1, tok.Border)
						}
						box.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Icon(c, tb.ico).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
								txtCol := tok.TextMuted
								if tb.active {
									txtCol = tok.Text
								}
								ui.Text(c, tb.lbl).FontSize(float32(tok.TextData)).Bold().TextColor(txtCol).SingleLine()
								if tb.pinned {
									ui.Icon(c, lucide.Pin()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
								}
							})
						})
					}
					ui.Spacer(c)
					kit.Keycap(c, tok, "Ctrl+O")
					kit.Keycap(c, tok, "Ctrl+Shift+R")
				})

				// Breadcrumb + Edit/Lab Mode Pill + Active 2px Accent pane indicator
				paneMock := ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
					Background(tok.Surface).Border(1, tok.Border).Clip()
				paneMock.Children(func() {
					ui.Box(c).FillWidth().Height(2).Background(tok.Accent)
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "水凝胶抗菌项目  >  HAMA 双靶向抑菌测定").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
						components.Badge(c, "编辑态", components.BadgeOptions{Tone: components.BadgeAccent})
						components.Badge(c, "实验态", components.BadgeOptions{Tone: components.BadgeNeutral})
					})
					ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

					// Simulated File Tree + Calendar Day Pill + Status Bar
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing16)).Children(func() {
						ui.Column(c).Width(float32(tok.Spacing48)*5.4).Gap(2).Children(func() {
							kit.ListItem(c, "st-tree-0", tok, kit.ListItemOptions{
								Title: "HAMA 课题组", Subtitle: "3 份实验", IndentLevel: 0,
								StartSlot: func() { ui.Icon(c, lucide.FolderOpen()).Size(14, 14).TextColor(tok.Accent) },
							})
							kit.ListItem(c, "st-tree-1", tok, kit.ListItemOptions{
								Title: "2026-11-24_抗菌涂板", IndentLevel: 1, Selected: true,
								StartSlot: func() { ui.Icon(c, lucide.FlaskConical()).Size(14, 14).TextColor(tok.Accent) },
							})
							kit.ListItem(c, "st-tree-2", tok, kit.ListItemOptions{
								Title: "CCK-8 细胞毒性复核", IndentLevel: 1,
								StartSlot: func() { ui.Icon(c, lucide.CircleCheck()).Size(14, 14).TextColor(tok.Success) },
							})
						})

						ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, "底部状态栏与移动端 iOS 26 玻璃浮岛语义").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
							ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
								Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.BorderSubtle).
								AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(tok.Success)
								ui.Text(c, "实时已保存").FontSize(float32(tok.TextMeta)).TextColor(tok.SuccessText)
								ui.Text(c, "· 3 步骤 · 5 模块").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
								ui.Spacer(c)
								ui.Text(c, "快捷键：Ctrl+K / Ctrl+O").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Row(c).Height(float32(tok.Spacing48)).Padding(0, float32(tok.Spacing16)).
									Radius(24).Background(tok.Elevated).Border(1, tok.BorderStrong).
									AlignItems(ui.Center).Gap(float32(tok.Spacing16)).Children(func() {
									ui.Icon(c, lucide.ChevronLeft()).Size(16, 16).TextColor(tok.TextMuted)
									ui.Icon(c, lucide.ChevronRight()).Size(16, 16).TextColor(tok.TextMuted)
									ui.Icon(c, lucide.Search()).Size(16, 16).TextColor(tok.Text)
									ui.Icon(c, lucide.Plus()).Size(18, 18).TextColor(tok.Accent)
									ui.Icon(c, lucide.Menu()).Size(16, 16).TextColor(tok.TextMuted)
								})
								ui.Text(c, "移动端底浮岛 (< 680dp)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
						})
					})
				})
			})
		},
	}
}

// RenderStewardPreviewPanel renders the live Ambient Lab Steward status pills and compact
// 「关联体检」 drawer body for the UI Gallery Storybook.
func RenderStewardPreviewPanel(c *ui.Context, tok theme.Tokens) {
	demo := &Shell{
		CurrentDocRelPath: "HAMA/L929-CCK8.asterml",
		CurrentDiagnostics: []DiagnosticItem{
			{
				Code:         "PANTRY_EXPIRED",
				Severity:     "blocking",
				Path:         "pantry.p-old",
				Message:      "文中提及试剂「旧批号胰酶 (Trypsin)」在储藏室中已过期（2026-10-15）",
				PantryItemID: "p-old",
			},
			{
				Code:         "PANTRY_NEAR_EXPIRY",
				Severity:     "needs_confirmation",
				Path:         "pantry.p-cck8",
				Message:      "文中提及试剂「CCK-8 试剂盒」即将过期（2026-11-28）",
				PantryItemID: "p-cck8",
			},
		},
		CurrentMatchedReagents: []MatchedReagentItem{
			{ID: "p-old", Name: "旧批号胰酶", Location: "-20°C B2", Status: "expired"},
			{ID: "p-cck8", Name: "CCK-8 试剂盒", Location: "-20°C A1", Status: "near_expiry"},
			{ID: "p-dmem", Name: "DMEM 培养基", Location: "4°C 上层", Status: "normal"},
		},
		CurrentDocSuggestions: []DocSuggestionItem{
			{KindLabel: "参数表 (param_table)", Summary: "37°C · 5% CO₂ · 24h", ByteOffset: 120},
			{KindLabel: "计时器 (timer)", Summary: "避光显色 90 min", ByteOffset: 240},
		},
		CurrentRelatedDocs: []RelatedDocItem{
			{RelPath: "Fig4/Van-Lys-抗菌测定.asterml", Title: "Van-Lys 液体共培养抗菌测定", Date: "2026-11-20", Folder: "Fig4"},
			{RelPath: "Cell/L929-毒性初筛-R1.asterml", Title: "L929 细胞毒性初筛（第 1 轮）", Date: "2026-11-12", Folder: "Cell"},
		},
		OnLocatePantry:       func(string) {},
		OnApplyDocSuggestion: func(DocSuggestionItem) {},
		OnOpenSearchHit:      func(string, string) {},
	}
	ui.Column(c.Key("sb-steward-preview")).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "底栏状态胶囊三态：").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			for _, p := range []struct {
				txt string
				col ui.Color
				bg  ui.Color
			}{
				{"2 项提醒", tok.DangerText, tok.DangerBg},
				{"✦ 2 处可结构化", tok.Accent, tok.AccentSoftBg},
				{"方案就绪", tok.SuccessText, tok.Elevated},
			} {
				ui.Row(c).Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Background(p.bg).
					Border(1, tok.BorderSubtle).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					if !strings.HasPrefix(p.txt, "✦") {
						ui.Box(c).Size(6, 6).Radius(3).Background(p.col)
					}
					ui.Text(c, p.txt).FontSize(float32(tok.TextMeta)).Bold().TextColor(p.col).SingleLine()
				})
			}
		})
		ui.Box(c).FillWidth().Padding(float32(tok.Spacing12)).
			Radius(float32(tok.ControlRadius)).Background(tok.Surface).
			Border(1, tok.Border).Children(func() {
			demo.renderDrawerLinksAndDiagnostics(c, tok)
		})
	})
}
