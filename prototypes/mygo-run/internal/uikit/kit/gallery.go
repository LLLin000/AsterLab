package kit

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
)

// GalleryState holds interactive sample state for AsterLab's built-in Zed-style UI Storybook.
type GalleryState struct {
	SampleNum     float64
	HoldConfirmed bool
	SelectedIdx   int
}

// RenderComponentGallery draws the interactive UI Kit showcase covering Elevation Ladder,
// Interaction State Tokens, Keycap, ListItem, EmptyState/InlineBanner, NumberInput/HoldToConfirm,
// and the 8 Lab Animal Stickers.
func RenderComponentGallery(c *ui.Context, st *GalleryState) {
	if st == nil {
		st = &GalleryState{SampleNum: 10}
	}
	if st.SampleNum <= 0 {
		st.SampleNum = 10
	}
	tok := theme.GetTokens(c.Theme())

	ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing16)).Children(func() {
			SectionHeader(c, tok, lucide.Layers(), "AsterLab 统一 UI 组件画廊",
				fmt.Sprintf("密度：%s · 主题：%s", tok.Density, tok.Preset), nil)

			// 1. Elevation Ladder
			elevCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing8))
			ApplyElevation(elevCard, tok, ElevationCard)
			elevCard.Children(func() {
				SectionHeader(c, tok, lucide.Box(), "1. 表面海拔阶梯与暗色顶边高光", "Canvas / Surface / Card / Floating / Modal", nil)
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
					tiers := []struct {
						lbl string
						ev  Elevation
					}{
						{"Canvas", ElevationCanvas},
						{"Surface", ElevationSurface},
						{"Card", ElevationCard},
						{"Floating", ElevationFloating},
						{"Modal", ElevationModal},
					}
					for _, tr := range tiers {
						tr := tr
						tile := ui.Column(c).Width(float32(tok.Spacing48)*2.5).
							Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Gap(2)
						ApplyElevation(tile, tok, tr.ev)
						if tr.ev <= ElevationSurface {
							tile.Radius(float32(tok.ControlRadius)).Border(1, tok.BorderSubtle)
						}
						tile.Children(func() {
							ui.Text(c, tr.lbl).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
							ui.Text(c, "Token").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
					}
				})
			})

			// 2. State Tokens & Native Controls (Keycap, NumberInput, HoldToConfirm, Badges)
			ctrlCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
			ApplyElevation(ctrlCard, tok, ElevationCard)
			ctrlCard.Children(func() {
				SectionHeader(c, tok, lucide.SlidersHorizontal(), "2. 交互控件、键帽与防误触确认", "", nil)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Row(c).Width(float32(tok.Spacing48)*3.4).Children(func() {
						components.NumberInput(c, &st.SampleNum, 1, 100, 1, components.NumberInputOptions{
							LeadingText: "倍比",
						})
					})
					ui.Box(c.Key("kit-gal-hold")).Children(func() {
						h := components.HoldToConfirm(c, &st.HoldConfirmed, "长按清空实验组", components.HoldToConfirmOptions{
							ConfirmedLabel: "已清空",
							Duration:       900 * time.Millisecond,
							Tone:           components.HoldDanger,
						})
						if h.Changed() {
							st.HoldConfirmed = false
						}
					})
					Keycap(c, tok, "Ctrl+K / Ctrl+O")
					Keycap(c, tok, "Ctrl+Shift+F")
					Keycap(c, tok, "Esc")
				})
			})

			// 3. ListItem + EmptyState + InlineBanner
			listCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing8))
			ApplyElevation(listCard, tok, ElevationCard)
			listCard.Children(func() {
				SectionHeader(c, tok, lucide.List(), "3. 统一列表行（ListItem）与状态提示", "悬停尾部平滑切换动作 · 零抖动", nil)
				sampleRows := []struct {
					title, sub string
					indent     int
				}{
					{"抗肿瘤细胞增殖筛选", "文件夹 · 6 份实验", 0},
					{"HUVEC 水凝胶成管实验", "STEP 4 进行中 · 12 孔板", 1},
					{"MRSA 抑菌圈与 CFU 计数", "STEP 2 待记录 · 12 块平皿", 1},
				}
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
				InlineBanner(c, tok, lucide.Info(), BannerInfo, "暗色模式下白底科研照片静息衰减至 86% 亮度，悬停恢复原图质感。", "", nil)
			})

			// 4. Lab Animal Vector Stickers Showcase
			animalCard := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
			ApplyElevation(animalCard, tok, ElevationCard)
			animalCard.Children(func() {
				animals := labsticker.AllAnimals()
				SectionHeader(c, tok, lucide.PawPrint(), "4. 实验动物矢量贴纸（动物房载体）", fmt.Sprintf("%d 种模式生物", len(animals)), nil)
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
					for _, sp := range animals {
						sp := sp
						tile := ui.Column(c.Key("gal-ani-" + string(sp.ID))).
							Width(float32(tok.Spacing48)*4.95).
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
		})
	})
}
