package kit

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
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
	PresetOverride  string // "" | paper | zinc | warm
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

	// Spatial Emergence Motion Sandbox state
	SandboxTransition string // "quick-palette" | "popover" | "modal" | "lightbox"
	SandboxOpen       bool
	SandboxBackdrop   bool

	// Darkroom Lightbox & Contact-Sheet Gallery state
	ContactDarkroom bool
	ContactDarkInit bool
	ContactSelectOn bool
	ContactSelected map[string]bool
	LightboxOpen    bool
	LightboxItemID  string
	LightboxState   DarkroomLightboxState

	// Topinspo Craft Workshop & Break-UI Stress Mode state
	StressMode         bool
	CraftInit          bool
	CraftCounter       int
	CraftMorph         MorphActionState
	CraftSlide         SlideConfirmState
	CraftSelectedCount int

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
	if !st.ContactDarkInit {
		st.ContactDarkInit = true
		st.ContactDarkroom = true
	}
	if st.SandboxTransition == "" {
		st.SandboxTransition = "quick-palette"
	}
	if st.ContactSelected == nil {
		st.ContactSelected = map[string]bool{"spec-huvec-1": true}
	}
	if !st.CraftInit {
		st.CraftInit = true
		if st.CraftCounter == 0 {
			st.CraftCounter = 12
		}
		if st.CraftSelectedCount == 0 {
			st.CraftSelectedCount = 3
		}
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
		{
			ID:       "darkroom-contact-sheet",
			Group:    "暗房图库",
			NavTitle: "暗房灯箱与印样",
			Title:    "3D. 暗房灯箱与印样画廊",
			Subtitle: "DarkroomLightbox · ContactSheetGrid · 2px 紧致方格 · tnum 时间线 · 双击 2x / 下滑关闭",
			Icon:     lucide.Aperture(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				renderStoryDarkroomGallery(c, tok, st)
			},
		},
		{
			ID:       "craft-workshop",
			Group:    "工艺工坊",
			NavTitle: "工艺工坊",
			Title:    "3E. Topinspo 工艺工坊",
			Subtitle: "Top-lit 顶光 · EdgeFadeX · RecessedWell · NumericRoll · SlideConfirm · MorphAction · MiddleTruncate",
			Icon:     lucide.Sparkles(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				renderStoryCraftWorkshop(c, tok, st)
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
		theme.PresetPaper: "纯净白纸",
		theme.PresetZinc:  "冷银极简",
		theme.PresetWarm:  "暖润纸面",
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
				trackInnerRad := tok.InnerControlRadius(tok.Spacing4 / 2)
				// Theme Light/Dark override pills
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "明暗").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
						Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing4)*0.5).Children(func() {
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
							if components.Button(c, tm.lbl, v).Radius(trackInnerRad).Clicked() {
								st.ThemeOverride = tm.id
							}
						}
					})
				})

				// Preset override pills
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "色盘").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
						Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing4)*0.5).Children(func() {
						for _, pr := range []struct{ id, lbl string }{
							{theme.PresetPaper, "纯净白纸"},
							{theme.PresetZinc, "冷银灰"},
							{theme.PresetWarm, "暖润纸面"},
						} {
							pr := pr
							on := (st.PresetOverride == "" && tok.Preset == pr.id) || st.PresetOverride == pr.id
							v := components.ButtonGhost
							if on {
								v = components.ButtonToolbar
							}
							if components.Button(c, pr.lbl, v).Radius(trackInnerRad).Clicked() {
								st.PresetOverride = pr.id
							}
						}
					})
				})

				// Density override pills
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "密度").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					ui.Row(c).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
						Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing4)*0.5).Children(func() {
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
							if components.Button(c, dn.lbl, v).Radius(trackInnerRad).Clicked() {
								st.DensityOverride = dn.id
							}
						}
					})
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

				stressLbl := "正常数据"
				stressVar := components.ButtonSecondary
				if st.StressMode {
					stressLbl = "极限压力"
					stressVar = components.ButtonPrimary
				}
				if components.Button(c.Key("gal-stress-toggle"), stressLbl, stressVar).Clicked() {
					st.StressMode = !st.StressMode
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
									ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing8), float32(tok.Spacing4), float32(tok.Spacing8)).Children(func() {
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

	if st.LightboxOpen {
		spec := findStorySpecimen(st.LightboxItemID)
		lbTitle := spec.Title
		if st.StressMode {
			lbTitle = craftStressFilename
		}
		DarkroomLightbox(c, tok, &st.LightboxOpen, &st.LightboxState, DarkroomLightboxProps{
			Key:          "storybook-darkroom-lightbox",
			Title:        lbTitle,
			FormatBadge:  spec.FormatBadge,
			MetaSummary:  spec.MetaSummary,
			Caption:      spec.Caption,
			HeroMatchKey: spec.HeroKey,
			Image:        spec.Bitmap,
			Actions: []DarkroomLightboxAction{
				{
					ID:    "insert",
					Label: "插入当前实验",
					Icon:  lucide.Plus(),
					Primary: true,
					OnClick: func() {
						st.DemoToasts.Push(Toast{Tone: ToastSuccess, Message: "已插入：" + MiddleTruncate(lbTitle, 24)})
						st.LightboxOpen = false
					},
				},
				{
					ID:    "export",
					Label: "导出原片",
					Icon:  lucide.Download(),
					OnClick: func() {
						st.DemoToasts.Push(Toast{Tone: ToastInfo, Message: "已导出 " + spec.FormatBadge + "：" + spec.ID})
					},
				},
				{
					ID:    "ghost",
					Label: "设为叠影参考",
					Icon:  lucide.Layers(),
					OnClick: func() {
						st.DemoToasts.Push(Toast{Tone: ToastInfo, Message: "已设为叠影参考：" + spec.RoleBadge})
					},
				},
			},
		})
	}
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
					Background(tok.Elevated).Border(1, tok.BorderSubtle).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).FillWidth().Height(float32(tok.Spacing24)).
						Radius(tok.InnerControlRadius(tok.Spacing4)).Background(sw.col).Border(1, tok.Border)
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

		// Interactive Motion Curve Sandbox + Spatial Emergence Grammar
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

		// 1B. Concentric Border Radius + Darkroom & Specular Rim Surface Ladder
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Stretch).Gap(float32(tok.Spacing12)).Children(func() {
			// Concentric Border Radius visual card
			concCard := ui.Column(c.Key("gal-concentric-card")).Grow(1).MinWidth(float32(tok.Spacing48)*6.5).
				Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
				Background(tok.Elevated).Border(1, tok.BorderSubtle).Gap(float32(tok.Spacing8))
			concCard.Children(func() {
				ui.Text(c, "1B. 同心圆角几何 (Concentric Border Radius)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
				ui.Text(c, "R_inner = max(2, R_outer - pad) · 消除嵌套拐角视觉挤压").FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
					// DialogRadius (14) -> pad 4/6/8 -> Inner
					dlgOuter := ui.Box(c).Padding(float32(tok.Spacing4)).
						Radius(float32(tok.DialogRadius)).Background(tok.Surface).Border(1, tok.BorderStrong)
					dlgOuter.Children(func() {
						ui.Column(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Radius(tok.InnerDialogRadius(tok.Spacing4)).
							Background(tok.SelectedRow).Border(1, tok.Border).Gap(float32(tok.Spacing4)*0.5).Children(func() {
							ui.Text(c, fmt.Sprintf("Dialog %d -> Inner %.0f", tok.DialogRadius, tok.InnerDialogRadius(tok.Spacing4))).
								FontSize(float32(tok.TextMeta)).FontFeatures("tnum").Bold().TextColor(tok.Text).SingleLine()
							ui.Text(c, fmt.Sprintf("pad %d -> Inner %.0f", tok.Spacing8, tok.InnerDialogRadius(tok.Spacing8))).
								FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine()
						})
					})

					// PanelRadius (12) -> pad Spacing4 (4) -> Inner (8)
					pnlOuter := ui.Box(c).Padding(float32(tok.Spacing4)).
						Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.BorderStrong)
					pnlOuter.Children(func() {
						ui.Column(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Radius(tok.InnerPanelRadius(tok.Spacing4)).
							Background(tok.SelectedRow).Border(1, tok.Border).Gap(float32(tok.Spacing4)*0.5).Children(func() {
							ui.Text(c, fmt.Sprintf("Panel %d -> Inner %.0f", tok.PanelRadius, tok.InnerPanelRadius(tok.Spacing4))).
								FontSize(float32(tok.TextMeta)).FontFeatures("tnum").Bold().TextColor(tok.Text).SingleLine()
							ui.Text(c, fmt.Sprintf("Control %d -> Inner %.0f", tok.ControlRadius, tok.InnerControlRadius(tok.Spacing4))).
								FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine()
						})
					})
				})
			})

			// Darkroom & Specular Rim Surface Ladder card
			drCard := ui.Column(c.Key("gal-darkroom-ladder")).Grow(1).MinWidth(float32(tok.Spacing48)*6.5).
				Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
				Background(tok.DarkroomCanvas).Border(1, tok.DarkroomBorder).Gap(float32(tok.Spacing8))
			drCard.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if tok.SpecularRim.A > 0 && r.W > 4 && r.H > 4 {
					p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, tok.SpecularRim, theme.ConcentricRadius(float32(tok.PanelRadius), 1), 1)
				}
			})
			drCard.Children(func() {
				ui.Text(c, "1C. 暗房表面阶梯与 1px 高光边 (Darkroom & SpecularRim)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.DarkroomText)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					drSteps := []struct {
						lbl string
						bg  ui.Color
					}{
						{"DarkroomCanvas", tok.DarkroomCanvas},
						{"DarkroomSurface", tok.DarkroomSurface},
						{"DarkroomElevated", tok.DarkroomElevated},
						{"DarkroomBadgeBg", tok.DarkroomBadgeBg},
					}
					for _, ds := range drSteps {
						ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
							Radius(tok.InnerPanelRadius(tok.Spacing4)).
							Background(ds.bg).Border(1, tok.DarkroomBorder).Children(func() {
							ui.Text(c, ds.lbl).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.DarkroomText).SingleLine()
						})
					}
				})
				ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)
				ui.Text(c, "HairlineDivider · SpecularRim 1px 阳极氧化石墨边线（恒定暗房标定）").
					FontSize(float32(tok.TextMeta)).TextColor(tok.DarkroomMuted).SingleLine().Ellipsis("…")
			})
		})

		// 1D. Spatial Emergence Motion Sandbox
		sbCard := ui.Column(c.Key("gal-motion-sandbox")).FillWidth().
			Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
			Background(tok.Elevated).Border(1, tok.BorderSubtle).Gap(float32(tok.Spacing8))
		sbCard.Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "1D. 空间浮现动效沙盒 (Spatial Emergence)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
				triggers := []struct {
					id, lbl string
				}{
					{"quick-palette", "QuickPalette 95ms"},
					{"popover", "Popover 150ms"},
					{"modal", "Modal 190ms"},
					{"lightbox", "Lightbox 220ms"},
				}
				for _, tr := range triggers {
					tr := tr
					v := components.ButtonSecondary
					if st.SandboxOpen && st.SandboxTransition == tr.id {
						v = components.ButtonPrimary
					}
					if components.Button(c.Key("sb-trig-"+tr.id), tr.lbl, v).Clicked() {
						if st.SandboxTransition == tr.id {
							st.SandboxOpen = !st.SandboxOpen
						} else {
							st.SandboxTransition = tr.id
							st.SandboxOpen = true
						}
					}
				}
				bdVar := components.ButtonSecondary
				if st.SandboxBackdrop {
					bdVar = components.ButtonPrimary
				}
				if components.Button(c.Key("sb-trig-backdrop"), "SheetBackdrop 0.94", bdVar).Clicked() {
					st.SandboxBackdrop = !st.SandboxBackdrop
				}
			})

			// Live Emergence Preview Stage
			stage := ui.Row(c.Key("gal-sb-stage")).FillWidth().MinHeight(float32(tok.Spacing48)*1.7).
				Padding(float32(tok.Spacing12)).
				Radius(tok.InnerPanelRadius(tok.Spacing4)).
				Background(tok.Canvas).Border(1, tok.HairlineDivider).
				AlignItems(ui.Center).Gap(float32(tok.Spacing12))
			theme.ApplySheetBackdropDepth(stage, st.SandboxBackdrop)
			stage.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
					ui.Text(c, "工作区画布景深层 (ApplySheetBackdropDepth)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
					statusLbl := "Scale 1.00 · 正常前景"
					if st.SandboxBackdrop {
						statusLbl = fmt.Sprintf("Scale %.2f · 底部 Sheet 升起退让", theme.SheetBackdropScale)
					}
					ui.Text(c, statusLbl).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted)
				})
				if st.SandboxOpen {
					var tr ui.ElementTransition
					var trDesc string
					switch st.SandboxTransition {
					case "popover":
						tr = theme.AnchoredPopoverTransition()
						trDesc = "AnchoredPopoverTransition · 150ms · Scale 0.96 · Flip"
					case "modal":
						tr = theme.ModalDialogTransition()
						trDesc = "ModalDialogTransition · 190ms · Scale 0.96 · Y +6"
					case "lightbox":
						tr = theme.DarkroomLightboxTransition()
						trDesc = "DarkroomLightboxTransition · 220ms · Scale 0.93"
					default:
						tr = theme.QuickPaletteTransition()
						trDesc = "QuickPaletteTransition · 95ms · Scale 0.98 · Y -4"
					}
					surf := ui.Row(c.Key("sb-surf-"+st.SandboxTransition)).
						Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).
						Transition(tr)
					ApplyElevation(surf, tok, ElevationFloating)
					surf.Children(func() {
						ui.Icon(c, lucide.Sparkles()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
						ui.Text(c, trDesc).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").Bold().TextColor(tok.Text).SingleLine()
					})
				}
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
		if st.StressMode {
			sampleRows = []struct {
				title, sub string
				indent     int
			}{
				{MiddleTruncate(craftStressFilename, 34), "999+ 份实验 · 极限文件名压测 · 0 溢出", 0},
				{"HUVEC_VEGF50ng_GroupB_Plate02_WellA01_超长中英混排实验记录标题边界压测", "STEP 999+ 进行中 · 96 孔板 · 999+ 孔", 1},
				{"MRSA_InhibitionZone_LongTitleOverflowCheck_CFU_20261127_Batch999", "STEP 999+ 待记录 · 999+ 块平皿", 1},
			}
		}
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for idx, r := range sampleRows {
				idx, r := idx, r
				badgeTxt := fmt.Sprintf("#%d", idx+1)
				if st.StressMode {
					badgeTxt = "999+"
				}
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
						components.Badge(c, badgeTxt, components.BadgeOptions{Tone: components.BadgeNeutral})
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

type storySpecimen struct {
	ID          string
	BatchID     string
	Title       string
	FormatBadge string
	RoleBadge   string
	CapturedAt  string
	MetaSummary string
	Caption     string
	HeroKey     string
	Kind        string // "fluo" | "bright" | "gel" | "tube"
	Bitmap      *ui.Bitmap
}

var (
	storySpecimensOnce sync.Once
	storySpecimensList []storySpecimen
)

func getStorySpecimens() []storySpecimen {
	storySpecimensOnce.Do(func() {
		specs := []storySpecimen{
			{
				ID: "spec-huvec-1", BatchID: "B1",
				Title: "HUVEC · DAPI+FITC 共定位", FormatBadge: "DNG", RoleBadge: "A1", CapturedAt: "14:22",
				MetaSummary: "ISO 400 · 1/125 s · 5600K · 20x Obj",
				Caption:     "EXP-2026-1127 · 孔 A1 · 10 uM HAMA 处理组",
				HeroKey:     "sb-hero-huvec-1", Kind: "fluo",
			},
			{
				ID: "spec-huvec-2", BatchID: "B1",
				Title: "FITC 血管内皮通道", FormatBadge: "RAW+JPG", RoleBadge: "A2", CapturedAt: "14:24",
				MetaSummary: "ISO 640 · 1/60 s · 488nm · 20x Obj",
				Caption:     "EXP-2026-1127 · 孔 A2 · 荧光定量基线",
				HeroKey:     "sb-hero-huvec-2", Kind: "tube",
			},
			{
				ID: "spec-huvec-3", BatchID: "B1",
				Title: "PBS 对照明场相差", FormatBadge: "DNG", RoleBadge: "B1", CapturedAt: "14:27",
				MetaSummary: "ISO 100 · 1/500 s · 5400K · 10x Obj",
				Caption:     "EXP-2026-1127 · 孔 B1 · PBS 空白对照",
				HeroKey:     "sb-hero-huvec-3", Kind: "bright",
			},
			{
				ID: "spec-huvec-4", BatchID: "B1",
				Title: "TRITC 成管分支网络", FormatBadge: "PNG", RoleBadge: "B2", CapturedAt: "14:31",
				MetaSummary: "ISO 800 · 1/30 s · 561nm · 20x Obj",
				Caption:     "EXP-2026-1127 · 孔 B2 · 血管环完整度 92%",
				HeroKey:     "sb-hero-huvec-4", Kind: "fluo",
			},
			{
				ID: "spec-gel-1", BatchID: "B2",
				Title: "SDS-PAGE 考马斯亮蓝", FormatBadge: "TIFF", RoleBadge: "Lane1", CapturedAt: "18:05",
				MetaSummary: "16-bit · 曝光 0.8 s · 300 dpi",
				Caption:     "蛋白表达验证 · Marker 15–180 kDa",
				HeroKey:     "sb-hero-gel-1", Kind: "gel",
			},
			{
				ID: "spec-gel-2", BatchID: "B2",
				Title: "WB 化学发光 12s", FormatBadge: "DNG", RoleBadge: "Lane2", CapturedAt: "18:09",
				MetaSummary: "ISO 1600 · 12.0 s · f/2.8 · ChemiDoc",
				Caption:     "目的条带 52 kDa · 内参 GAPDH 36 kDa",
				HeroKey:     "sb-hero-gel-2", Kind: "gel",
			},
			{
				ID: "spec-gel-3", BatchID: "B2",
				Title: "抑菌圈平皿暗室复拍", FormatBadge: "RAW+JPG", RoleBadge: "Dish3", CapturedAt: "18:14",
				MetaSummary: "ISO 200 · 1/250 s · 5500K · f/4",
				Caption:     "MRSA 抑菌圈测定 · 环径 18.4 mm",
				HeroKey:     "sb-hero-gel-3", Kind: "tube",
			},
		}
		for i := range specs {
			specs[i].Bitmap = makeSyntheticLabBitmap(specs[i].Kind, i)
		}
		storySpecimensList = specs
	})
	return storySpecimensList
}

func findStorySpecimen(id string) storySpecimen {
	list := getStorySpecimens()
	for _, s := range list {
		if s.ID == id {
			return s
		}
	}
	return list[0]
}

func makeSyntheticLabBitmap(kind string, seed int) *ui.Bitmap {
	const sz = 160
	img := image.NewRGBA(image.Rect(0, 0, sz, sz))
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			fx := float64(x) / sz
			fy := float64(y) / sz
			var r, g, b uint8
			switch kind {
			case "fluo":
				// Darkfield fluorescence microscopy: glowing green/cyan/amber puncta on deep obsidian
				v1 := math.Sin((fx*9+float64(seed)*0.7)*math.Pi) * math.Cos((fy*9-float64(seed)*0.3)*math.Pi)
				v2 := math.Exp(-((fx-0.42)*(fx-0.42) + (fy-0.48)*(fy-0.48)) * 22)
				v3 := math.Exp(-((fx-0.66)*(fx-0.66) + (fy-0.36)*(fy-0.36)) * 30)
				glow := math.Max(0, v1)*0.45 + v2*0.95 + v3*0.75
				if seed%2 == 0 {
					r = uint8(min(255, int(12+glow*40)))
					g = uint8(min(255, int(18+glow*215)))
					b = uint8(min(255, int(24+glow*160)))
				} else {
					r = uint8(min(255, int(14+glow*220)))
					g = uint8(min(255, int(14+glow*145)))
					b = uint8(min(255, int(20+glow*45)))
				}
			case "tube":
				// Vascular ring / Petri colony ring pattern
				dx, dy := fx-0.5, fy-0.5
				dist := math.Sqrt(dx*dx + dy*dy)
				ring := math.Exp(-math.Pow((dist-0.28)*9, 2)) + 0.55*math.Exp(-math.Pow((dist-0.14)*12, 2))
				net := 0.5 + 0.5*math.Sin(fx*22+fy*14)
				val := math.Min(1.0, ring*(0.65+0.35*net))
				r = uint8(18 + val*120)
				g = uint8(24 + val*205)
				b = uint8(30 + val*180)
			case "gel":
				// SDS-PAGE electrophoresis bands across 4 lanes
				r, g, b = 18, 22, 28
				lane := int(fx * 5)
				inLane := (fx*5 - float64(lane))
				if lane >= 1 && lane <= 3 && inLane > 0.22 && inLane < 0.78 {
					band1 := math.Exp(-math.Pow((fy-0.28)*28, 2))
					band2 := math.Exp(-math.Pow((fy-0.54)*24, 2)) * (0.4 + 0.2*float64(lane))
					band3 := math.Exp(-math.Pow((fy-0.76)*30, 2)) * 0.85
					lum := math.Min(1.0, band1+band2+band3)
					r = uint8(18 + lum*195)
					g = uint8(24 + lum*210)
					b = uint8(36 + lum*215)
				}
			default: // brightfield phase contrast
				wave := 0.5 + 0.5*math.Sin(fx*16)*math.Cos(fy*16)
				base := 185 + int(wave*35)
				r, g, b = uint8(base), uint8(base+4), uint8(base+8)
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return ui.NewBitmap(img)
}

func renderStoryDarkroomGallery(c *ui.Context, tok theme.Tokens, st *GalleryState) {
	outer := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
	ApplyElevation(outer, tok, ElevationCard)
	outer.Children(func() {
		SectionHeader(c, tok, lucide.Aperture(), "3D. 暗房灯箱与印样画廊",
			"DarkroomLightbox · ContactSheetGrid · 2px 紧致方格 · tnum 时间线 · 点击入暗房（双击 2x / 下滑关闭）", func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					drVar := components.ButtonSecondary
					if st.ContactDarkroom {
						drVar = components.ButtonPrimary
					}
					if components.Button(c, "暗房底色", drVar).Clicked() {
						st.ContactDarkroom = !st.ContactDarkroom
					}
					selVar := components.ButtonSecondary
					if st.ContactSelectOn {
						selVar = components.ButtonPrimary
					}
					if components.Button(c, "多选模式", selVar).Clicked() {
						st.ContactSelectOn = !st.ContactSelectOn
					}
				})
			})

		specs := getStorySpecimens()
		var b1Items, b2Items []ContactSheetItem
		for i, sp := range specs {
			title := sp.Title
			roleBadge := sp.RoleBadge
			if st.StressMode {
				title = MiddleTruncate(craftStressFilename, 24)
				if i == 0 {
					roleBadge = "999+"
				}
			}
			it := ContactSheetItem{
				ID:           sp.ID,
				Title:        title,
				FormatBadge:  sp.FormatBadge,
				RoleBadge:    roleBadge,
				CapturedAt:   sp.CapturedAt,
				Image:        sp.Bitmap,
				HeroMatchKey: sp.HeroKey,
			}
			if sp.BatchID == "B1" {
				b1Items = append(b1Items, it)
			} else {
				b2Items = append(b2Items, it)
			}
		}

		b1Title := "2026-11-27 · B20261127-01 · HUVEC 免疫荧光"
		if st.StressMode {
			b1Title = MiddleTruncate(craftStressFilename, 32) + " (999+)"
		}
		batches := []ContactSheetBatch{
			{
				ID:          "B20261127-01",
				Title:       b1Title,
				Subtitle:    "20x 物镜 · 共聚焦/明场组",
				ActionLabel: "选择本批",
				OnAction: func() {
					st.ContactSelectOn = true
					for _, it := range b1Items {
						st.ContactSelected[it.ID] = true
					}
				},
				Items: b1Items,
			},
			{
				ID:          "B20261126-03",
				Title:       "2026-11-26 · B20261126-03 · 凝胶与平皿成像",
				Subtitle:    "ChemiDoc + 暗室复拍",
				ActionLabel: "清空选中",
				OnAction: func() {
					clear(st.ContactSelected)
				},
				Items: b2Items,
			},
		}

		var activeHero any
		if st.LightboxOpen {
			activeHero = findStorySpecimen(st.LightboxItemID).HeroKey
		}

		sheetWrap := ui.Column(c).FillWidth().
			Padding(float32(tok.Spacing12)).
			Radius(float32(tok.PanelRadius)).
			Clip()
		if st.ContactDarkroom {
			sheetWrap.Background(tok.DarkroomCanvas).Border(1, tok.DarkroomBorder)
		} else {
			sheetWrap.Background(tok.Canvas).Border(1, tok.HairlineDivider)
		}
		sheetWrap.Children(func() {
			ContactSheetGrid(c, tok, batches, ContactSheetOptions{
				Darkroom:      st.ContactDarkroom,
				SelectedIDs:   st.ContactSelected,
				SelectionOn:   st.ContactSelectOn,
				ActiveHeroKey: activeHero,
				OnItemClick: func(item ContactSheetItem) {
					if st.ContactSelectOn {
						st.ContactSelected[item.ID] = !st.ContactSelected[item.ID]
						return
					}
					st.LightboxItemID = item.ID
					st.LightboxOpen = true
				},
			})
		})
	})
}

const (
	craftNormalFilename = "20261127_HUVEC_Plate02_WellA01_6h_FOV02.dng"
	craftStressFilename = "20261127_HUVECTUBE_VEGF50ng_GroupB_Plate02_WellA01_6h_FOV02_RAW_16bit.dng"
)

func renderStoryCraftWorkshop(c *ui.Context, tok theme.Tokens, st *GalleryState) {
	outer := ui.Column(c.Key("craft-workshop-card")).FillWidth().
		Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
		Gap(float32(tok.Spacing12))
	ApplyElevation(outer, tok, ElevationCard)
	outer.Children(func() {
		headerMeta := "Top-lit 顶光 · EdgeFadeX · RecessedWell · NumericRoll · SlideConfirm"
		if st.StressMode {
			headerMeta = "极限压力态 · 73 字符文件名 · 999+ 计数 · 402dp 零溢出"
		}
		SectionHeader(c, tok, lucide.Sparkles(), "3E. Topinspo 工艺工坊", headerMeta, func() {
			badgeTxt := "正常样本"
			badgeTone := components.BadgeNeutral
			if st.StressMode {
				badgeTxt = "999+ 极限压力"
				badgeTone = components.BadgeWarning
			}
			Badge(c, badgeTxt, components.BadgeOptions{Tone: badgeTone})
		})

		// 1. Top-lit Specular Rim cards & Recessed Well + ShortcutKeycaps
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Stretch).Gap(float32(tok.Spacing12)).Children(func() {
			rimCard := ui.Column(c.Key("craft-toplit-card")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).
				Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
			ApplyElevation(rimCard, tok, ElevationCard)
			rimCard.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "Top-lit Specular Rim (180° 顶光边线)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
					Badge(c, "ApplyTopLitBorder", components.BadgeOptions{Tone: components.BadgeAccent})
				})
				ui.Text(c, "暗色模式由 SpecularRimTop → Border → SpecularRimBottom 单层渐变勾勒立体边缘。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				floatPill := ui.Row(c.Key("craft-toplit-floating")).FillWidth().
					Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8))
				ApplyElevation(floatPill, tok, ElevationFloating)
				floatPill.Children(func() {
					ui.Icon(c, lucide.Layers()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent).Shrink(0)
					lbl := "ElevationFloating · 顶光渐变描边"
					if st.StressMode {
						lbl = MiddleTruncate(craftStressFilename, 28)
					}
					ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.Text).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				})
			})

			wellCard := ui.Column(c.Key("craft-recessed-card")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).
				Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
			ApplyElevation(wellCard, tok, ElevationCard)
			wellCard.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "Recessed Well & 凹槽键帽 (ShortcutKeycaps)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
					if st.StressMode {
						Badge(c, "999+", components.BadgeOptions{Tone: components.BadgeDanger})
					}
				})
				wellTrack := ui.Row(c.Key("craft-recessed-well")).FillWidth().Wrap().
					Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).
					Background(tok.Canvas).
					Border(1, tok.BorderSubtle).
					AlignItems(ui.Center).
					Gap(float32(tok.Spacing8))
				theme.ApplyRecessedWell(wellTrack, tok)
				wellTrack.Children(func() {
					ui.Text(c, "快捷指令").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					ShortcutKeycaps(c, tok, "Ctrl", "K")
					ShortcutKeycaps(c, tok, "Shift", "Alt", "E")
				})
			})
		})

		// 2. ApplyEdgeFadeX Horizontal ScrollSnap Strip
		ui.Column(c.Key("craft-edgefade-box")).FillWidth().
			Padding(float32(tok.Spacing12)).
			Radius(float32(tok.PanelRadius)).
			Background(tok.Elevated).
			Border(1, tok.BorderSubtle).
			Gap(float32(tok.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "ApplyEdgeFadeX + ScrollSnap(SnapMandatory) 印样横滑条").
					FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
				stripMeta := "5 组样本"
				if st.StressMode {
					stripMeta = "999+ 样本"
				}
				ui.Text(c, stripMeta).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine()
			})
			strip := ui.ScrollHorizontal(c.Key("craft-snap-strip")).FillWidth().ScrollSnap(ui.SnapMandatory)
			theme.ApplyEdgeFadeX(strip, tok, true, true)
			strip.Children(func() {
				ui.Row(c).AlignItems(ui.Stretch).Padding(float32(tok.Spacing4), float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
					snapItems := []struct {
						id, title, meta string
					}{
						{"s1", "A1 · HUVEC_DAPI.dng", "ISO 400 · 20x"},
						{"s2", "A2 · VEGF_50ng.dng", "ISO 640 · 20x"},
						{"s3", "B1 · PBS_Ctrl.dng", "ISO 100 · 10x"},
						{"s4", "B2 · TRITC_Tube.dng", "ISO 800 · 20x"},
						{"s5", "C1 · ChemiDoc_WB.tiff", "16-bit · 12s"},
					}
					for _, item := range snapItems {
						item := item
						ttl := item.title
						meta := item.meta
						if st.StressMode {
							ttl = MiddleTruncate(craftStressFilename, 24)
							meta = "999+ · 16-bit RAW"
						}
						sc := ui.Column(c.Key("craft-snap-"+item.id)).
							Width(float32(tok.Spacing48)*3.8).
							Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Gap(float32(tok.Spacing4)).
							SnapAlign(ui.Start)
						ApplyElevation(sc, tok, ElevationCard)
						sc.Children(func() {
							ui.Text(c, ttl).FontSize(float32(tok.TextData)).Bold().FontFeatures("tnum").
								TextColor(tok.Text).SingleLine().Ellipsis("…").MinWidth(0)
							ui.Text(c, meta).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
								TextColor(tok.TextMuted).SingleLine().Ellipsis("…").MinWidth(0)
						})
					}
				})
			})
		})

		// 3. NumericRoll Mechanical Pipette Counter + MorphActionButton + SlideConfirm
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Stretch).Gap(float32(tok.Spacing12)).Children(func() {
			// Mechanical Pipette NumericRoll
			pipetteCard := ui.Column(c.Key("craft-pipette-card")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).
				Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
			ApplyElevation(pipetteCard, tok, ElevationCard)
			pipetteCard.Children(func() {
				ui.Text(c, "机械移液器 NumericRoll 滚轮计数").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if components.Button(c.Key("craft-cnt-dec"), "-", components.ButtonSecondary).Clicked() {
						if st.CraftCounter > 0 {
							st.CraftCounter--
						}
					}
					cntStr := fmt.Sprintf("%d uL", st.CraftCounter)
					if st.StressMode {
						cntStr = "999+ uL"
					}
					well := ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).Center()
					theme.ApplyRecessedWell(well, tok)
					well.Children(func() {
						ui.Text(c, cntStr).
							FontSize(float32(tok.TextSection)).Bold().FontFeatures("tnum").
							TextColor(tok.Text).SingleLine().
							ContentTransition(theme.NumericRoll(false))
					})
					if components.Button(c.Key("craft-cnt-inc"), "+", components.ButtonSecondary).Clicked() {
						st.CraftCounter++
					}
				})
			})

			// Width-stable MorphActionButton
			morphCard := ui.Column(c.Key("craft-morph-card")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).
				Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
			ApplyElevation(morphCard, tok, ElevationCard)
			morphCard.Children(func() {
				ui.Text(c, "等宽 MorphActionButton (Idle → Busy → Done)").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if MorphActionButton(c, tok, &st.CraftMorph, MorphActionOptions{
						Key:       "craft-morph-btn",
						IdleLabel: "归档实验数据",
						BusyLabel: "归档校验中…",
						DoneLabel: "已归档",
						IdleIcon:  lucide.Archive(),
						Primary:   true,
					}) {
						st.CraftMorph.Phase = MorphBusy
						st.CraftMorph.PhaseSince = time.Now()
					}
					if components.Button(c.Key("craft-morph-step"), "切换阶段", components.ButtonSecondary).Clicked() {
						st.CraftMorph.Phase = (st.CraftMorph.Phase + 1) % 3
					}
				})
			})
		})

		// SlideConfirm bar
		slideCard := ui.Column(c.Key("craft-slide-card")).FillWidth().
			Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
		ApplyElevation(slideCard, tok, ElevationCard)
		slideCard.Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "SlideConfirm 不可逆滑块确认 (≥82% 锁定 / <82% 弹簧回弹)").
					FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
				if st.CraftSlide.Confirmed {
					if components.Button(c.Key("craft-slide-reset"), "重置滑块", components.ButtonGhost).Clicked() {
						st.CraftSlide.Reset()
					}
				}
			})
			slideLbl := "向右滑动销毁废液批号 · B20261127"
			if st.StressMode {
				slideLbl = "滑动确认销毁 " + MiddleTruncate(craftStressFilename, 24)
			}
			if SlideConfirm(c, tok, &st.CraftSlide, SlideConfirmOptions{
				Key:            "craft-slide-confirm",
				Label:          slideLbl,
				ConfirmedLabel: "已确认锁定销毁",
				Danger:         true,
			}) {
				st.DemoToasts.Push(Toast{
					Tone:             ToastDanger,
					Message:          "已锁定废液批号销毁操作",
					UndoLabel:        "撤销",
					ProgressOverride: 0.72,
					OnUndo: func() {
						st.CraftSlide.Reset()
					},
				})
			}
		})

		// 4. MiddleTruncate Before/After + FloatingSelectionBar + Undo Toast Trigger
		truncCard := ui.Column(c.Key("craft-truncate-card")).FillWidth().
			Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
		ApplyElevation(truncCard, tok, ElevationCard)
		truncCard.Children(func() {
			ui.Text(c, "MiddleTruncate 科研文件名智能中段截断对照").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
			srcName := craftNormalFilename
			if st.StressMode {
				srcName = craftStressFilename
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				Badge(c, "原文", components.BadgeOptions{Tone: components.BadgeNeutral})
				ui.Text(c, srcName).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
					TextColor(tok.TextMuted).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				Badge(c, "28 字符", components.BadgeOptions{Tone: components.BadgeAccent})
				ui.Text(c, MiddleTruncate(srcName, 28)).FontSize(float32(tok.TextData)).Bold().FontFeatures("tnum").
					TextColor(tok.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				Badge(c, "42 字符", components.BadgeOptions{Tone: components.BadgeSuccess})
				ui.Text(c, MiddleTruncate(srcName, 42)).FontSize(float32(tok.TextData)).FontFeatures("tnum").
					TextColor(tok.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
		})

		// FloatingSelectionBar + Undo Toast countdown hairline demo
		fsbWrap := ui.Column(c.Key("craft-fsb-card")).FillWidth().
			Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8))
		ApplyElevation(fsbWrap, tok, ElevationCard)
		fsbWrap.Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "FloatingSelectionBar 悬浮上下文操作条 & Undo Toast 倒计时进度条").
					FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
				if components.Button(c.Key("craft-sel-add"), "选中 +1", components.ButtonSecondary).Clicked() {
					st.CraftSelectedCount++
				}
				if components.Button(c.Key("craft-sel-sub"), "选中 -1", components.ButtonSecondary).Clicked() {
					if st.CraftSelectedCount > 0 {
						st.CraftSelectedCount--
					}
				}
				if components.Button(c.Key("craft-undo-toast-btn"), "触发进度条 Undo Toast", components.ButtonSecondary).Clicked() {
					prev := st.CraftSelectedCount
					st.CraftSelectedCount = 0
					st.DemoToasts.Push(Toast{
						Tone:             ToastWarning,
						Message:          "已移除所选显微印样",
						UndoLabel:        "撤销",
						Duration:         6 * time.Second,
						ProgressOverride: 0.68,
						OnUndo: func() {
							st.CraftSelectedCount = prev
						},
					})
				}
			})
			selCount := st.CraftSelectedCount
			if st.StressMode {
				selCount = 999
			}
			FloatingSelectionBar(c, tok, FloatingSelectionBarProps{
				Key:        "craft-floating-bar",
				Count:      selCount,
				UnitLabel:  "项已选",
				ClearLabel: "清空",
				OnClear: func() {
					st.CraftSelectedCount = 0
				},
				Actions: []FloatingSelectionAction{
					{
						ID:      "export",
						Label:   "批量导出",
						Icon:    lucide.Download(),
						Primary: true,
						OnClick: func() {
							st.DemoToasts.Push(Toast{Tone: ToastSuccess, Message: fmt.Sprintf("已导出 %d 项样本", selCount)})
						},
					},
					{
						ID:     "delete",
						Label:  "移除",
						Icon:   lucide.Trash(),
						Danger: true,
						OnClick: func() {
							prev := st.CraftSelectedCount
							st.CraftSelectedCount = 0
							st.DemoToasts.Push(Toast{
								Tone:             ToastDanger,
								Message:          "已移除选中样本",
								UndoLabel:        "撤销",
								ProgressOverride: 0.65,
								OnUndo:           func() { st.CraftSelectedCount = prev },
							})
						},
					},
				},
			})
		})

		// 5. SkeletonCard & EmptyStateCard
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Stretch).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Column(c.Key("craft-skel-wrap")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "SkeletonCard 骨架占位卡").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				SkeletonCard(c, tok)
			})
			ui.Column(c.Key("craft-empty-wrap")).Grow(1).MinWidth(float32(tok.Spacing48)*5.4).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "EmptyStateCard 快捷键引导空态").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				emptyTitle := "暂无待归档的共聚焦图像"
				emptyHint := "拖拽 DNG/TIFF 至画廊或按下快捷键新建记录"
				if st.StressMode {
					emptyTitle = MiddleTruncate(craftStressFilename, 28)
					emptyHint = "999+ 条记录筛选为空 · 请重置条件或快捷键新建"
				}
				EmptyStateCard(c, tok, EmptyStateProps{
					Icon:        lucide.FlaskConical(),
					Title:       emptyTitle,
					Hint:        emptyHint,
					ActionLabel: "新建样本记录",
					Shortcuts:   []string{"Ctrl", "N"},
					OnAction: func() {
						st.CraftSelectedCount++
					},
				})
			})
		})
	})
}
