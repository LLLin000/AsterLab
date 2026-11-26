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
								ui.Box(c).Size(6, 6).Radius(float32(tok.PillRadius)).Background(tok.Success)
								ui.Text(c, "实时已保存").FontSize(float32(tok.TextMeta)).TextColor(tok.SuccessText)
								ui.Text(c, "· 3 步骤 · 5 模块").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
								ui.Spacer(c)
								ui.Text(c, "快捷键：Ctrl+K / Ctrl+O").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Row(c).Height(float32(tok.Spacing48)).Padding(0, float32(tok.Spacing16)).
									Radius(float32(tok.PillRadius)).Background(tok.Elevated).Border(1, tok.BorderStrong).
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
						ui.Box(c).Size(6, 6).Radius(float32(tok.PillRadius)).Background(p.col)
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

// BuiltinMobileStorySection renders AsterLab's iOS 402×874 mobile-first design
// specification inside the UI Component Gallery so phone layouts, top/bottom SafeArea
// reserves, gesture drawer loops, and mobile.Sheet forms can be inspected alongside desktop tokens.
func (s *Shell) BuiltinMobileStorySection() kit.StorySection {
	return kit.StorySection{
		ID:       "mobile-ios-design",
		Group:    "移动端规范",
		NavTitle: "移动端 402dp 设计",
		Title:    "8. 移动端 iOS 专属设计规范与 402×874 真机参考",
		Subtitle: "顶部 64dp 安全避让 · 双向抽屉闭环 · Obsidian 全屏页面与两级设置 · 移动端工具栏",
		Icon:     lucide.Smartphone(),
		Render: func(c *ui.Context, tok theme.Tokens) {
			t := c.Theme()
			card := ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(4)).Gap(t.Space(4))
			kit.ApplyElevation(card, tok, kit.ElevationCard)
			card.Children(func() {
				kit.SectionHeader(c, tok, lucide.Smartphone(),
					"8A. 402dp 移动工作台壳层、顶/底 SafeArea 避让与手势闭环",
					"原生无桌面 sidebar.Shell 顶栏 · 中置顶部下拉热区 · 右抽屉无底岛遮挡 · Horizontal 滑闭", nil)

				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Start).Gap(t.Space(4)).Children(func() {
					// Frame A: 402dp Top Clearance + Pull Zone + Pantry Header + Bottom Dock
					phoneA := ui.Column(c).Grow(1).Basis(float32(tok.Spacing48)*7).MinWidth(float32(tok.Spacing48)*6).MaxWidth(float32(tok.Spacing48)*8).
						Padding(t.Space(3)).Radius(float32(tok.DialogRadius)).
						Background(tok.Canvas).Border(1, tok.BorderStrong).Gap(t.Space(2)).Clip()
					phoneA.Children(func() {
						ui.Text(c, "iPhone 402dp · 顶栏避让与下拉热区").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
						// Floating Top Bar simulation (Left Orb + Center Pull Strip + Right Twin Pill)
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							orb := ui.Box(c).Size(float32(tok.Spacing48)-t.Space(2), float32(tok.Spacing48)-t.Space(2)).
								Radius(float32(tok.DialogRadius)).Center().Background(tok.Surface).Border(1, tok.Border)
							orb.Children(func() { ui.Icon(c, lucide.PanelLeft()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text) })
							pullTag := ui.Row(c).Grow(1).MinWidth(0).Height(t.Space(8)).Radius(float32(tok.ControlRadius)).
								Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.35)).Center().Gap(t.Space(1))
							pullTag.Children(func() {
								ui.Icon(c, lucide.Zap()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
								ui.Text(c, "中置下拉热区 (46dp)").FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).SingleLine()
							})
							twin := ui.Row(c).Height(float32(tok.Spacing48)-t.Space(2)).Padding(0, t.Space(2)).
								Radius(float32(tok.DialogRadius)).Background(tok.Surface).Border(1, tok.Border).AlignItems(ui.Center).Gap(t.Space(2))
							twin.Children(func() {
								ui.Icon(c, lucide.Calculator()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
								ui.Icon(c, lucide.Search()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
							})
						})
						// 64dp reserved clearance below floating top bar -> Pantry Header
						ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Border(1, tok.BorderSubtle).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							ui.Text(c, "储藏室").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
							components.Badge(c, "试剂耗材 | 动物房", components.BadgeOptions{Tone: components.BadgeNeutral})
						})
						ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
							components.Button(c, "+ 收纳盒", components.ButtonSecondary).Grow(1).Basis(0)
							components.Button(c, "+ 新试剂", components.ButtonPrimary).Grow(1).Basis(0)
						})
						// Floating Glass Bottom Dock + Edit Toolbar Capsule sample
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Padding(t.Space(2), t.Space(3)).
							Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
							ui.Icon(c, lucide.AtSign()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
							ui.Icon(c, lucide.Slash()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
							ui.Icon(c, lucide.MessageSquareQuote()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
							ui.Icon(c, lucide.Sparkles()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
							ui.Icon(c, lucide.ListPlus()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
							ui.Spacer(c)
							ui.Text(c, "底岛避让 SafeArea").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
						})
					})

					// Frame B: Dedicated Right Companion Drawer (no bottom island overlap)
					phoneB := ui.Column(c).Grow(1).Basis(float32(tok.Spacing48)*7).MinWidth(float32(tok.Spacing48)*6).MaxWidth(float32(tok.Spacing48)*8).
						Padding(t.Space(3)).Radius(float32(tok.DialogRadius)).
						Background(tok.Canvas).Border(1, tok.BorderStrong).Gap(t.Space(2)).Clip()
					phoneB.Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							ui.Icon(c, lucide.Calculator()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
							components.Text(c, "步骤与速算（独立右抽屉）").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
							ui.Icon(c, lucide.PanelLeft()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
							ui.Icon(c, lucide.X()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
						})
						ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
						ui.Column(c).FillWidth().Padding(t.Space(2)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Border(1, tok.Border).Gap(t.Space(2)).Children(func() {
							ui.Text(c, "快捷倒计时 · 全量露出无底岛遮挡").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent)
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
								ui.Text(c, "15:00").FontSize(float32(tok.TextSection)).Bold().FontFeatures("tnum").TextColor(tok.Text).Grow(1).MinWidth(0)
								components.Button(c, "开始", components.ButtonPrimary)
								components.Button(c, "重置", components.ButtonSecondary)
							})
						})
						ui.Text(c, "闭环规则：Axis: ui.Horizontal 支持抽屉内反向滑闭，且左抽屉与 SwipeRow 互不抢占。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
				})

				kit.SectionHeader(c, tok, lucide.Layers(),
					"8B. 全屏移动页面规范（Obsidian 两级设置页 & 全屏表单导航栏）",
					"手机端设置与重表单采用 edge-to-edge 全屏页面：一级分组列表 + ‹ 设置 返回二级详情 + 取消 | 标题 | 保存 顶栏", nil)

				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Start).Gap(t.Space(4)).Children(func() {
					// Full-Screen Card 1: Obsidian 2-Level Settings + Toolbar Customize Row
					sheetA := ui.Column(c).Grow(1).Basis(float32(tok.Spacing48)*7).MinWidth(float32(tok.Spacing48)*6).MaxWidth(float32(tok.Spacing48)*8).
						Padding(t.Space(3)).Radius(float32(tok.DialogRadius)).
						Background(tok.Canvas).Border(1, tok.BorderStrong).Gap(t.Space(2)).Clip()
					sheetA.Children(func() {
						// Level 2 Obsidian Top Nav Bar preview
						ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(float32(tok.ControlRadius)).
							Background(tok.Surface).Border(1, tok.BorderSubtle).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
								ui.Icon(c, lucide.ChevronLeft()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
								ui.Text(c, "设置").FontSize(float32(tok.TextData)).TextColor(tok.Accent).SingleLine()
							})
							ui.Text(c, "外观 · 二级设置页").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
							ui.Text(c, "完成").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).SingleLine()
						})
						// Level 1 Grouped Settings Section preview
						ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
							Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
							for i, item := range []struct{ title, hint string }{
								{"文件与库", "AsterLab"},
								{"多端同步", "WebDAV"},
							} {
								if i > 0 {
									ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
								}
								ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
									ui.Text(c, item.title).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
									ui.Text(c, item.hint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
									ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
								})
							}
						})
						// Full-width untruncated Mobile Toolbar Customize row
						for _, rowSpec := range []struct{ name, grp string }{
							{"标题 H1–H3", "排版与结构"},
							{"自然语言转结构", "科研快捷"},
						} {
							ui.Row(c).FillWidth().Padding(t.Space(2)).Radius(float32(tok.ControlRadius)).
								Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
								ui.Icon(c, lucide.Heading()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
								ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
									components.Text(c, rowSpec.name).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine()
									components.Text(c, rowSpec.grp).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
								})
								ui.Icon(c, lucide.ChevronUp()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
								ui.Icon(c, lucide.ChevronDown()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
								components.Badge(c, "显示", components.BadgeOptions{Tone: components.BadgeAccent})
							})
						}
					})

					// Full-Screen Card 2: Full-Screen Form Navigation + 2-Col Pantry Grid
					sheetB := ui.Column(c).Grow(1).Basis(float32(tok.Spacing48)*7).MinWidth(float32(tok.Spacing48)*6).MaxWidth(float32(tok.Spacing48)*8).
						Padding(t.Space(3)).Radius(float32(tok.DialogRadius)).
						Background(tok.Canvas).Border(1, tok.BorderStrong).Gap(t.Space(2)).Clip()
					sheetB.Children(func() {
						ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(float32(tok.ControlRadius)).
							Background(tok.Surface).Border(1, tok.BorderSubtle).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							ui.Text(c, "取消").FontSize(float32(tok.TextData)).TextColor(tok.TextMuted).SingleLine()
							ui.Text(c, "新增试剂 / 新建实验").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
							components.Button(c, "保存", components.ButtonPrimary).Shrink(0)
						})
						ui.Column(c).FillWidth().Padding(t.Space(2)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Border(1, tok.Accent.Alpha(0.45)).Gap(t.Space(1)).Children(func() {
							components.Text(c, "全屏页面替代弹窗 · 无顶部留黑与超宽裁切").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
							components.Text(c, "全文搜索 / 动物房笼卡 & SidePeek / 灯箱 / 反馈均贴齐 SafeArea 全屏铺满").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
						ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
							Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
							for idx, row := range []struct{ code, strain, meta, status string }{
								{"SPF-A01", "C57BL/6J", "♂3 ♀2 · 8w · 22.4g", "实验中"},
								{"SPF-B04", "BALB/c", "♀4 · 6w · 19.1g", "正常在养"},
							} {
								if idx > 0 {
									ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
								}
								ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
									ui.Text(c, row.code).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine().Shrink(0)
									ui.Column(c).Grow(1).MinWidth(0).Children(func() {
										ui.Text(c, row.strain).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
										ui.Text(c, row.meta).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
									})
									components.Badge(c, row.status, components.BadgeOptions{Tone: components.BadgeAccent})
								})
							}
						})
						ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
							for _, box := range []struct{ title, hint string }{
								{"标准冻存管盒", "9×9 · 81 孔"},
								{"EP 离心管盒", "5×10 · 50 孔"},
							} {
								ui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(t.Space(2)).Radius(float32(tok.ControlRadius)).
									Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
									ui.Icon(c, lucide.Package()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
									ui.Text(c, box.title).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
									ui.Text(c, box.hint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
								})
							}
						})
					})
				})
			})
		},
	}
}
