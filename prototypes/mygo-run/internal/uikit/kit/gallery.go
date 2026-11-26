package kit

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
	"asterlab-mygo-prototype/internal/uikit/plate"
	"asterlab-mygo-prototype/internal/uikit/workflownode"
)

// StorySection defines one browsable, pluggable chapter in AsterLab's full UI Storybook.
// Feature packages (shell, asterdoc, tools) inject sections from outside kit to avoid import cycles.
type StorySection struct {
	ID       string
	Group    string
	NavTitle string
	Title    string
	Subtitle string
	Icon     *ui.SVG
	Render   func(c *ui.Context, tok theme.Tokens)
}

// GalleryState holds interactive state and live preview overrides for AsterLab's full UI Storybook.
type GalleryState struct {
	ActiveSection   string // "all" or section ID
	Search          string
	ThemeOverride   string // ""/"follow" | "light" | "dark"
	PresetOverride  string // "" | obsidian | zinc | warm
	DensityOverride string // "" | compact | default | comfortable
	NarrowPreview   bool   // Constrain canvas to 520dp narrow-pane check
	MobilePreview   bool   // Constrain canvas to 402dp iPhone viewport check

	SampleNum     float64
	ScrubVal      float64
	HoldConfirmed bool
	SelectedIdx   int
	MotionStep    int
	NudgeSeq      int
	DemoToggle    bool
	DemoCheck     bool
	DemoTab       int
	DemoSelect    string
	DemoInput     string
	CtxLastAction string
	PeekOpen      bool
	FilterState   FilterBarState
	DemoToasts    *ToastQueue

	plateState plate.InteractionState
}

// RenderComponentGallery draws the interactive full AsterLab UI Storybook covering:
// global tokens/motion, base onevra+kit controls & 6 UI engines, science carriers & all stickers,
// plus injected shell/asterdoc/vivarium/edge sections.
func RenderComponentGallery(c *ui.Context, st *GalleryState, extra ...StorySection) {
	if st == nil {
		st = &GalleryState{SampleNum: 12, ScrubVal: 24.5}
	}
	if st.SampleNum <= 0 {
		st.SampleNum = 12
	}
	if st.ScrubVal == 0 {
		st.ScrubVal = 24.5
	}
	if st.ActiveSection == "" {
		st.ActiveSection = "all"
	}
	if st.ThemeOverride == "" {
		st.ThemeOverride = "follow"
	}
	if st.DemoSelect == "" {
		st.DemoSelect = "SPF"
	}
	if st.DemoInput == "" {
		st.DemoInput = "C57BL/6J · PBS 空白对照"
	}
	if st.DemoToasts == nil {
		st.DemoToasts = NewToastQueue()
	}

	tok := theme.GetTokens(c.Theme())
	if st.ThemeOverride == "light" || st.ThemeOverride == "dark" || st.PresetOverride != "" || st.DensityOverride != "" {
		preset := tok.Preset
		if st.PresetOverride != "" {
			preset = st.PresetOverride
		}
		dark := tok.Dark
		if st.ThemeOverride == "light" {
			dark = false
		} else if st.ThemeOverride == "dark" {
			dark = true
		}
		if st.DensityOverride != "" {
			prev := theme.ActiveDensity()
			theme.SetActiveDensity(st.DensityOverride)
			tok = theme.TokensFor(preset, dark, tok.LiquidGlass)
			theme.SetActiveDensity(prev)
		} else {
			tok = theme.TokensFor(preset, dark, tok.LiquidGlass)
		}
	}

	sections := []StorySection{
		{
			ID:       "tokens-motion",
			Group:    "基础规范",
			NavTitle: "全局 Token 与动效",
			Title:    "1. 全局 Token、色板、海拔与动效",
			Subtitle: "语义色彩 · 字体阶梯 · 4px 间距 · 5 级表面海拔 · Motion",
			Icon:     lucide.Palette(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				renderStoryTokensAndMotion(c, tok, st)
			},
		},
		{
			ID:       "controls-kit",
			Group:    "通用原语",
			NavTitle: "控件与交互引擎",
			Title:    "2. 基础控件与 6 大 UI 交互引擎",
			Subtitle: "onevra 控件 · ScrubNumber · ContextMenu · FilterBar · SidePeek · Toast",
			Icon:     lucide.SlidersHorizontal(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				renderStoryControlsAndEngines(c, tok, st)
			},
		},
		{
			ID:       "science-carriers",
			Group:    "科学载体",
			NavTitle: "空间载体与贴纸",
			Title:    "3. 孔板、平皿、冻存盒与全量科研贴纸",
			Subtitle: "plate.View · plate.Rack · 全量试剂载体 + 8 种实验动物矢量贴纸",
			Icon:     lucide.FlaskConical(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				renderStoryScienceCarriers(c, tok, st)
			},
		},
	}
	sections = append(sections, extra...)

	densityLabel := map[string]string{
		theme.DensityCompact:     "紧凑密度",
		theme.DensityDefault:     "标准密度",
		theme.DensityComfortable: "舒展密度",
	}[tok.Density]
	if densityLabel == "" {
		densityLabel = "标准密度"
	}
	presetLabel := map[string]string{
		theme.PresetObsidian: "纯净白纸",
		theme.PresetZinc:     "冷银极简",
		theme.PresetWarm:     "暖润纸面",
	}[tok.Preset]
	if presetLabel == "" {
		presetLabel = "纯净白纸"
	}
	modeLabel := "浅色"
	if tok.Dark {
		modeLabel = "深色"
	}

	vw, _ := c.Size()
	compactGallery := vw > 0 && vw < 760

	root := ui.Column(c).Fill().Background(tok.Canvas)
	root.Children(func() {
		// Top Story Title & Live Control Toolbar
		topPadX := float32(tok.Spacing16)
		if compactGallery {
			topPadX = float32(tok.Spacing12)
		}
		ui.Column(c).FillWidth().Shrink(0).
			Padding(float32(tok.Spacing8), topPadX).
			Background(tok.Surface).
			Gap(float32(tok.Spacing8)).Children(func() {
			var desktopSearchSlot func()
			if !compactGallery {
				desktopSearchSlot = func() {
					ui.Box(c).Width(float32(tok.Spacing48) * 3.8).Children(func() {
						components.TextBox(c, &st.Search, components.TextBoxOptions{
							LeadingIcon: lucide.Search(),
						}).Placeholder("过滤展区或控件…")
					})
				}
			}
			SectionHeader(c, tok, lucide.Layers(), "AsterLab 统一 UI 组件画廊",
				fmt.Sprintf("%s · %s · %s", densityLabel, presetLabel, modeLabel), desktopSearchSlot)
			if compactGallery {
				components.TextBox(c, &st.Search, components.TextBoxOptions{
					LeadingIcon: lucide.Search(),
				}).Placeholder("过滤展区或控件…").FillWidth()
			}

			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				// Theme Light/Dark override pills
				ui.Text(c, "明暗").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
					for _, tm := range []struct{ id, lbl string }{
						{"follow", "跟随"},
						{"light", "浅色"},
						{"dark", "深色"},
					} {
						tm := tm
						on := st.ThemeOverride == tm.id
						v := components.ButtonGhost
						if on {
							v = components.ButtonToolbar
						}
						if components.Button(c, tm.lbl, v).Clicked() {
							st.ThemeOverride = tm.id
						}
					}
				})

				// Preset override pills
				ui.Text(c, "色盘").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
					for _, pr := range []struct{ id, lbl string }{
						{theme.PresetObsidian, "纯净白纸"},
						{theme.PresetZinc, "冷银灰"},
						{theme.PresetWarm, "暖润纸面"},
					} {
						pr := pr
						on := (st.PresetOverride == "" && tok.Preset == pr.id) || st.PresetOverride == pr.id
						v := components.ButtonGhost
						if on {
							v = components.ButtonToolbar
						}
						if components.Button(c, pr.lbl, v).Clicked() {
							st.PresetOverride = pr.id
						}
					}
				})

				// Density override pills
				ui.Text(c, "密度").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
					for _, dn := range []struct{ id, lbl string }{
						{theme.DensityCompact, "紧凑"},
						{theme.DensityDefault, "标准"},
						{theme.DensityComfortable, "舒展"},
					} {
						dn := dn
						on := (st.DensityOverride == "" && tok.Density == dn.id) || st.DensityOverride == dn.id
						v := components.ButtonGhost
						if on {
							v = components.ButtonToolbar
						}
						if components.Button(c, dn.lbl, v).Clicked() {
							st.DensityOverride = dn.id
						}
					}
				})

				if !compactGallery {
					narrowVar := components.ButtonSecondary
					if st.NarrowPreview {
						narrowVar = components.ButtonPrimary
					}
					if components.Button(c, "520dp 窄屏预览", narrowVar).Clicked() {
						st.NarrowPreview = !st.NarrowPreview
						if st.NarrowPreview {
							st.MobilePreview = false
						}
					}

					mobileVar := components.ButtonSecondary
					if st.MobilePreview {
						mobileVar = components.ButtonPrimary
					}
					if components.Button(c, "402dp iPhone 预览", mobileVar).Clicked() {
						st.MobilePreview = !st.MobilePreview
						if st.MobilePreview {
							st.NarrowPreview = false
						}
					}
				}
			})
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

		if compactGallery {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Background(tok.Surface).Children(func() {
				ui.ScrollHorizontal(c).FillWidth().Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						allBtn := components.Button(c, "全部", components.ButtonGhost)
						if st.ActiveSection == "all" {
							allBtn = components.Button(c, "全部", components.ButtonPrimary)
						}
						if allBtn.Clicked() {
							st.ActiveSection = "all"
						}
						for _, sec := range sections {
							sec := sec
							navLbl := sec.NavTitle
							if navLbl == "" {
								navLbl = sec.Title
							}
							v := components.ButtonSecondary
							if st.ActiveSection == sec.ID {
								v = components.ButtonPrimary
							}
							if components.Button(c.Key("mob-nav-"+sec.ID), navLbl, v).Clicked() {
								st.ActiveSection = sec.ID
							}
						}
					})
				})
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
		}

		// Main body: Left category sidebar (hidden on compact mobile screens) + Right live preview canvas
		ui.Row(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			if !compactGallery {
				// Sidebar nav
				ui.Column(c).Width(float32(tok.Spacing48)*4.5).FillHeight().
					Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing4)).
					Background(tok.Surface).Children(func() {
					ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
						ui.Column(c).FillWidth().Gap(2).Children(func() {
							ListItem(c, "story-nav-all", tok, ListItemOptions{
								Title:    "全部展区总览",
								Subtitle: fmt.Sprintf("%d 个模块组", len(sections)),
								Selected: st.ActiveSection == "all",
								StartSlot: func() {
									ui.Icon(c, lucide.LayoutGrid()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
								},
								OnClick: func() { st.ActiveSection = "all" },
							})
							lastGroup := ""
							for _, sec := range sections {
								sec := sec
								if sec.Group != lastGroup {
									lastGroup = sec.Group
									ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing8), 2, float32(tok.Spacing8)).Children(func() {
										ui.Text(c, sec.Group).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).SingleLine()
									})
								}
								navLbl := sec.NavTitle
								if navLbl == "" {
									navLbl = sec.Title
								}
								ListItem(c, "story-nav-"+sec.ID, tok, ListItemOptions{
									Title:    navLbl,
									Selected: st.ActiveSection == sec.ID,
									StartSlot: func() {
										ico := sec.Icon
										if ico == nil {
											ico = lucide.Box()
										}
										ui.Icon(c, ico).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
									},
									OnClick: func() { st.ActiveSection = sec.ID },
								})
							}
						})
					})
				})
				ui.Box(c).Width(1).FillHeight().Background(tok.BorderSubtle)
			}

			// Right preview canvas
			ui.Column(c).Grow(1).FillHeight().MinWidth(0).MinHeight(0).Clip().Children(func() {
				ui.Scroll(c).Fill().Children(func() {
					canvasPad := float32(tok.Spacing16)
					if compactGallery {
						canvasPad = float32(tok.Spacing12)
					}
					wrap := ui.Column(c).FillWidth().Padding(canvasPad).Gap(float32(tok.Spacing16))
					if !compactGallery && (st.NarrowPreview || st.MobilePreview) {
						wrap.AlignItems(ui.Center)
					}
					wrap.Children(func() {
						inner := ui.Column(c).FillWidth().Gap(float32(tok.Spacing16))
						if !compactGallery && st.MobilePreview {
							inner.MaxWidth(402).Padding(float32(tok.Spacing12)).
								Radius(float32(tok.DialogRadius)).Border(2, tok.BorderStrong)
						} else if !compactGallery && st.NarrowPreview {
							inner.MaxWidth(520).Padding(float32(tok.Spacing12)).
								Radius(float32(tok.PanelRadius)).Border(1, tok.BorderSelected)
						}
						inner.Children(func() {
							q := strings.ToLower(strings.TrimSpace(st.Search))
							shownCount := 0
							for _, sec := range sections {
								if st.ActiveSection != "all" && st.ActiveSection != sec.ID {
									continue
								}
								if q != "" && !strings.Contains(strings.ToLower(sec.Title+" "+sec.Subtitle+" "+sec.Group), q) {
									continue
								}
								shownCount++
								if sec.Render != nil {
									sec.Render(c, tok)
								}
							}
							if shownCount == 0 {
								EmptyState(c, tok, lucide.Search(), "未找到匹配的组件展区",
									"可清空顶部过滤词或切换回「全部展区总览」。", "重置过滤", func() {
										st.Search = ""
										st.ActiveSection = "all"
									})
							}
						})
					})
				})
			})
		})
	})

	// SidePeek + Toast live preview overlays (scoped inside Storybook root)
	SidePeek(c, "storybook-sidepeek", tok, &st.PeekOpen, SidePeekOptions{
		Title:    "SidePeek 侧滑详情引擎预览",
		Subtitle: "Linear / Twenty 同款右侧滑出检查器，点击左侧遮罩或右上关闭返回画廊",
		Icon:     lucide.PanelRight(),
		Footer: func() {
			ui.Spacer(c)
			if components.Button(c, "触发撤销提示 Toast", components.ButtonSecondary).Clicked() {
				prevVal := st.ScrubVal
				st.ScrubVal += 5
				st.DemoToasts.Push(Toast{
					Tone:    ToastSuccess,
					Message: fmt.Sprintf("SidePeek 已更新参数至 %.1f", st.ScrubVal),
					OnUndo:  func() { st.ScrubVal = prevVal },
				})
			}
			if components.Button(c, "关闭抽屉", components.ButtonPrimary).Clicked() {
				st.PeekOpen = false
			}
		},
	}, func() {
		ui.Text(c, "SidePeek 保留左侧笼架/表格全局视野，适合动物笼卡快速修订、试剂出入库核对与日程速览。").
			FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
		ScrubNumber(c, "peek-scrub-demo", tok, &st.ScrubVal, ScrubOptions{
			Label: "实验给药剂量", Unit: "mg/kg", Min: 0, Max: 500, Step: 0.5, Decimals: 1, Width: 260,
		})
	})
	st.DemoToasts.Render(c, tok)
}

func renderStoryTokensAndMotion(c *ui.Context, tok theme.Tokens, st *GalleryState) {
	// 1A. Color Swatches & Semantic Tokens
	tokenCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
	ApplyElevation(tokenCard, tok, ElevationCard)
	tokenCard.Children(func() {
		SectionHeader(c, tok, lucide.Palette(), "1A. 全局色彩色板与状态 Token",
			fmt.Sprintf("Canvas/Surface/Element/Ghost/Border · 字号 %d/%d/%d/%d px", tok.TextPage, tok.TextSection, tok.TextBody, tok.TextMeta), nil)

		cat := theme.CategoricalPalette(tok.Dark)
		swatches := []struct {
			name string
			col  ui.Color
		}{
			{"Canvas", tok.Canvas},
			{"Surface", tok.Surface},
			{"Elevated", tok.Elevated},
			{"Accent", tok.Accent},
			{"AccentSoft", tok.AccentSoftBg},
			{"ElementHover", tok.ElementHover},
			{"GhostHover", tok.GhostHover},
			{"Success", tok.Success},
			{"Warning", tok.Warning},
			{"Danger", tok.Danger},
		}
		for idx := 0; idx < min(4, len(cat)); idx++ {
			swatches = append(swatches, struct {
				name string
				col  ui.Color
			}{fmt.Sprintf("实验分组 #%d", idx+1), cat[idx]})
		}
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			for i, sw := range swatches {
				ui.Column(c.Key(fmt.Sprintf("sw-%d", i))).
					Width(float32(tok.Spacing48)*2.0).
					Padding(float32(tok.Spacing4)).
					Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.BorderSubtle).Gap(4).Children(func() {
					ui.Box(c).FillWidth().Height(float32(tok.Spacing24)).
						Radius(float32(tok.ControlRadius) - 1).Background(sw.col).Border(1, tok.Border)
					ui.Text(c, sw.name).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).SingleLine().Ellipsis("…")
				})
			}
		})

		// Brand + 5-Tier Elevation Ladder
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.BorderSubtle).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if logo := theme.LogoParsedSmall(); logo != nil {
					ui.Icon(c, logo).Size(float32(tok.Spacing24), float32(tok.Spacing24))
				}
				ui.Column(c).Children(func() {
					ui.Text(c, "AsterLab 品牌与排版").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					ui.Text(c, fmt.Sprintf("H1=%d · Section=%d · Body=%d · Data=%d · Meta=%d px (tnum: 0123.45)",
						tok.TextPage, tok.TextSection, tok.TextBody, tok.TextData, tok.TextMeta)).
						FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted)
				})
			})
		})
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			tiers := []struct {
				lbl, sub string
				ev       Elevation
			}{
				{"Canvas", "Level 0 · 画布", ElevationCanvas},
				{"Surface", "Level 1 · 表面", ElevationSurface},
				{"Card", "Level 2 · 卡片", ElevationCard},
				{"Floating", "Level 3 · 顶光", ElevationFloating},
				{"Modal", "Level 4 · 模态", ElevationModal},
			}
			for _, tr := range tiers {
				tile := ui.Column(c).Width(float32(tok.Spacing48)*2.35).
					Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Gap(2)
				ApplyElevation(tile, tok, tr.ev)
				if tr.ev <= ElevationSurface {
					tile.Radius(float32(tok.ControlRadius)).Border(1, tok.BorderStrong)
				}
				tile.Children(func() {
					ui.Text(c, tr.lbl).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
					ui.Text(c, tr.sub).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				})
			}
		})

		// Interactive Motion Curve Sandbox
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "动效沙盒：").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
			nudgeBox := ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8))
			theme.ApplyNudge(nudgeBox, st.NudgeSeq)
			nudgeBox.Children(func() {
				if components.Button(c, fmt.Sprintf("切换磁吸状态 (#%d)", st.MotionStep), components.ButtonSecondary).Clicked() {
					st.MotionStep = (st.MotionStep + 1) % 3
				}
				if components.Button(c, "触发轻晃提醒 ApplyNudge", components.ButtonGhost).Clicked() {
					st.NudgeSeq++
				}
				pill := ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).Border(1, tok.Border)
				bg := tok.Elevated
				lbl := "MotionSnappy · 静止态"
				if st.MotionStep == 1 {
					bg = tok.AccentSoftBg
					lbl = "MotionFluid · 过渡态"
				} else if st.MotionStep == 2 {
					bg = tok.SuccessBg
					lbl = "MotionHero · 达成态"
				}
				pill.Background(pill.AnimateColor("gal-mot-bg", bg, theme.MotionSnappy))
				pill.Children(func() {
					ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).SingleLine()
				})
			})
		})
	})
}

func renderStoryControlsAndEngines(c *ui.Context, tok theme.Tokens, st *GalleryState) {
	ctrlCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
	ApplyElevation(ctrlCard, tok, ElevationCard)
	ctrlCard.Children(func() {
		SectionHeader(c, tok, lucide.SlidersHorizontal(), "2A. 基础交互控件 & 6 大 UI 逻辑引擎",
			"onevra 全变体 · ScrubNumber · ContextMenu · FilterBar · SidePeek · Toast", nil)

		// Buttons & Badges row
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			components.Button(c, "Primary 主按钮", components.ButtonPrimary)
			components.Button(c, "Secondary 次按钮", components.ButtonSecondary)
			components.Button(c, "Toolbar 工具", components.ButtonToolbar)
			components.Button(c, "Ghost 幽灵", components.ButtonGhost)
			components.IconButton(c, lucide.Columns2(), "分屏图标钮", components.IconButtonCompact)
			Badge(c, "Success", components.BadgeOptions{Tone: components.BadgeSuccess})
			Badge(c, "Warning", components.BadgeOptions{Tone: components.BadgeWarning})
			Badge(c, "Danger", components.BadgeOptions{Tone: components.BadgeDanger})
			Badge(c, "Accent", components.BadgeOptions{Tone: components.BadgeAccent})
		})

		// Inputs, ScrubNumber, NumberInput, HoldToConfirm, Keycaps
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Box(c).Width(float32(tok.Spacing48)*4.5).Children(func() {
				components.TextBox(c, &st.DemoInput, components.TextBoxOptions{LeadingIcon: lucide.PenLine()})
			})
			ui.Row(c).Width(float32(tok.Spacing48)*3.3).Children(func() {
				components.NumberInput(c, &st.SampleNum, 1, 100, 1, components.NumberInputOptions{
					LeadingText: "倍比",
				})
			})
			ScrubNumber(c, "gal-scrub", tok, &st.ScrubVal, ScrubOptions{
				Label: "拖拽微调体重", Unit: "g", Min: 1, Max: 500, Step: 0.2, Decimals: 1, Width: float32(tok.Spacing48) * 3.8,
			})
			ui.Box(c.Key("kit-gal-hold")).Children(func() {
				h := components.HoldToConfirm(c, &st.HoldConfirmed, "长按清空", components.HoldToConfirmOptions{
					ConfirmedLabel: "已清空",
					Duration:       900 * time.Millisecond,
					Tone:           components.HoldDanger,
				})
				if h.Changed() {
					st.HoldConfirmed = false
				}
			})
			components.Switch(c, &st.DemoToggle, "开关", components.SwitchOptions{})
			components.Checkbox(c, &st.DemoCheck, "已核验")
			Keycap(c, tok, "Ctrl+K / Ctrl+O")
			Keycap(c, tok, "Ctrl+Shift+F")
		})

		// FilterBar Engine Demo
		FilterBar(c, "gal-filter", tok, &st.FilterState, []FilterField{
			{Key: "grade", Label: "设施等级", Options: []FilterOption{{Value: "SPF", Label: "SPF 屏障"}, {Value: "aquatic", Label: "水生系统"}}},
			{Key: "status", Label: "实验状态", Options: []FilterOption{{Value: "in_study", Label: "实验中"}, {Value: "breeding", Label: "繁育配对"}}},
		}, []SavedFilterView{
			{ID: "all", Label: "全部 (4)"},
			{ID: "spf", Label: "SPF 屏障", Filters: []ActiveFilter{{FieldKey: "grade", Value: "SPF"}}},
			{ID: "study", Label: "实验中", Filters: []ActiveFilter{{FieldKey: "status", Value: "in_study"}}},
		}, "组合筛选条件过滤演示…")

		// ContextMenu + SidePeek + Toast Triggers + ListItem
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			if components.Button(c, "打开 SidePeek 侧滑抽屉", components.ButtonSecondary).Clicked() {
				st.PeekOpen = true
			}
			if components.Button(c, "触发乐观撤销 Toast", components.ButtonSecondary).Clicked() {
				old := st.ScrubVal
				st.ScrubVal += 10
				st.DemoToasts.Push(Toast{
					Tone:    ToastInfo,
					Message: fmt.Sprintf("已调整参数为 %.1f g", st.ScrubVal),
					OnUndo:  func() { st.ScrubVal = old },
				})
			}
			ctxBox := ui.Row(c.Key("gal-ctx-box")).
				Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).
				Background(tok.Elevated).Border(1, tok.Border).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8))
			ctxBox.Children(func() {
				ui.Icon(c, lucide.MousePointerClick()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
				msg := "右键此处试用 kit.ContextMenu"
				if st.CtxLastAction != "" {
					msg = "已触发：" + st.CtxLastAction
				}
				ui.Text(c, msg).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
			})
			ContextMenu(c, ctxBox, func(m *Menu) {
				if m.Item("复制模块结构").Shortcut("Ctrl+C").Chosen() {
					st.CtxLastAction = "复制模块结构"
				}
				m.Submenu("导出与格式", func(sub *Menu) {
					if sub.Item("导出 PDF 报告").Chosen() {
						st.CtxLastAction = "导出 PDF 报告"
					}
					if sub.Item("导出 CSV 数据").Chosen() {
						st.CtxLastAction = "导出 CSV 数据"
					}
				})
				if m.Item("标记为基准模板").Chosen() {
					st.CtxLastAction = "标记为基准模板"
				}
			})
		})

		sampleRows := []struct {
			title, sub string
			indent     int
		}{
			{"抗肿瘤细胞增殖筛选", "文件夹 · 6 份实验", 0},
			{"HUVEC 水凝胶成管实验", "STEP 4 进行中 · 12 孔板", 1},
			{"MRSA 抑菌圈与 CFU 计数", "STEP 2 待记录 · 12 块平皿", 1},
		}
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for idx, r := range sampleRows {
				idx, r := idx, r
				ListItem(c, fmt.Sprintf("gal-li-%d", idx), tok, ListItemOptions{
					Title:       r.title,
					Subtitle:    r.sub,
					IndentLevel: r.indent,
					Selected:    st.SelectedIdx == idx,
					StartSlot: func() {
						ico := lucide.FlaskConical()
						if idx == 0 {
							ico = lucide.Folder()
						}
						ui.Icon(c, ico).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
					},
					EndSlot: func() {
						components.Badge(c, fmt.Sprintf("#%d", idx+1), components.BadgeOptions{Tone: components.BadgeNeutral})
					},
					HoverEndSlot: func() {
						components.IconButton(c, lucide.Columns2(), "分屏打开", components.IconButtonCompact)
					},
					OnClick: func() {
						st.SelectedIdx = idx
					},
				})
			}
		})
		InlineBanner(c, tok, lucide.Info(), BannerInfo, "暗色模式下白底科研照片静息衰减至 86% 亮度，悬停恢复原图质感。", "", nil)
	})
}

func renderStoryScienceCarriers(c *ui.Context, tok theme.Tokens, st *GalleryState) {
	// Plates, Petri Dish & Cryo Rack
	plateCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
	ApplyElevation(plateCard, tok, ElevationCard)
	plateCard.Children(func() {
		SectionHeader(c, tok, lucide.Grid2x2(), "3A. 空间实验载体（孔板 · 平皿 · 管架 · 流程节点）",
			"复用 internal/uikit/plate 与 workflownode 渲染", nil)

		demoWells := map[string]plate.WellState{
			"A1": {Group: "PBS", Role: "control"},
			"A2": {Group: "PBS", Role: "control"},
			"B1": {Group: "10uM", Role: "treatment"},
			"B2": {Group: "10uM", Role: "treatment"},
			"C1": {Group: "50uM", Role: "comparator"},
			"C2": {Group: "50uM", Role: "comparator"},
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Start).Gap(float32(tok.Spacing16)).Children(func() {
			ui.Column(c).Width(float32(tok.Spacing48)*8.2).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "24 孔板 / 框选分布视图").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				plate.View(c, plate.Props{
					Rows: 4, Cols: 6, Wells: demoWells, SelectedIDs: []string{"B1", "B2"},
					Interactive: true, HideFooter: true, Zoom: 0.72, State: &st.plateState,
				})
			})
			ui.Column(c).Width(float32(tok.Spacing48)*5.2).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "四象限 90mm 琼脂平皿").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				plate.View(c, plate.Props{
					Rows: 2, Cols: 2, Topology: "dish", Wells: demoWells, HideFooter: true, Zoom: 0.78,
				})
			})
			ui.Column(c).Width(float32(tok.Spacing48)*5.2).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "流程节点卡片 (workflownode)").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				workflownode.View(c, workflownode.NodeProps{
					ID: "gal-wf-1", Kind: workflownode.KindOperation, Order: "01", Tag: "Step",
					Title: "37°C CO2 孵育成管 6 h", State: workflownode.StateSelected,
					Params: []workflownode.ParamItem{{Key: "温度", Val: "37 °C"}, {Key: "湿度", Val: "95%"}},
				}, 1.0, nil)
			})
		})

		plate.Rack(c, plate.RackProps{
			Selected: "Lane2",
			Lanes: []plate.LaneState{
				{Position: "Lane1", SampleID: "PBS-1", Group: "对照", Status: "recorded", HasValue: true},
				{Position: "Lane2", SampleID: "HAMA-10", Group: "处理", Status: "recorded", HasValue: true},
				{Position: "Lane3", SampleID: "HAMA-50", Group: "阳性", Status: "excluded"},
				{Position: "Lane4", SampleID: "Blank", Status: "missing"},
			},
		})
	})

	// All Reagent + Box Stickers
	vessels := append([]labsticker.VesselSpec(nil), labsticker.AllVessels()...)
	vessels = append(vessels, labsticker.AllBoxes()...)
	reagentCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing8))
	ApplyElevation(reagentCard, tok, ElevationCard)
	reagentCard.Children(func() {
		SectionHeader(c, tok, lucide.Package(), "3B. 储藏室全量试剂器皿与冻存盒贴纸",
			fmt.Sprintf("%d 种标准耗材/冻存盒 SVG", len(vessels)), nil)
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			for _, sp := range vessels {
				sp := sp
				tile := ui.Column(c.Key("gal-ves-" + string(sp.ID))).
					Width(float32(tok.Spacing48)*3.9).
					Padding(float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.BorderSubtle).
					AlignItems(ui.Center).Gap(4)
				tile.Children(func() {
					if svg, err := labsticker.ParseVesselSVG(sp.ID); err == nil && svg != nil {
						ui.Image(c, svg).Size(44, 34)
					}
					ui.Text(c, sp.Title).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
					ui.Text(c, sp.VolumeLabel).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				})
			}
		})
	})

	// All 8 Lab Animal Vector Stickers
	animalCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
	ApplyElevation(animalCard, tok, ElevationCard)
	animalCard.Children(func() {
		animals := labsticker.AllAnimals()
		SectionHeader(c, tok, lucide.PawPrint(), "3C. 实验动物矢量贴纸（动物房载体）", fmt.Sprintf("%d 种模式生物", len(animals)), nil)
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			for _, sp := range animals {
				sp := sp
				tile := ui.Column(c.Key("gal-ani-" + string(sp.ID))).
					Width(float32(tok.Spacing48)*4.5).
					Padding(float32(tok.Spacing12), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.BorderSubtle).
					AlignItems(ui.Center).Gap(float32(tok.Spacing4))
				tile.Children(func() {
					labsticker.RenderAnimalSticker(c, sp.ID, 42)
					ui.Text(c, sp.Title).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine()
					ui.Text(c, sp.CommonStrains).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				})
			}
		})
	})
}
