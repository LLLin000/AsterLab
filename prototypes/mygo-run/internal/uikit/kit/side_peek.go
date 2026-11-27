package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// SidePeekOptions configures a Linear/Twenty-style right slide-over inspector drawer.
type SidePeekOptions struct {
	Title         string
	Subtitle      string
	Icon          *ui.SVG
	Width         float32
	HeaderActions func()
	Footer        func()
}

// SidePeek renders a right-hand detail drawer keeping the left workspace visible on desktop,
// and an Obsidian-style SafeArea-aware full-screen page on mobile viewports (< 680dp).
// Clicking the dim scrim or pressing Escape closes the drawer cleanly.
func SidePeek(c *ui.Context, key any, tok theme.Tokens, open *bool, opts SidePeekOptions, body func()) {
	if c == nil || open == nil || !*open {
		return
	}
	if isMobileContextViewport(c) {
		ui.Overlay(c, func() {
			renderMobileSidePeekPage(c, key, tok, open, opts, body)
		})
		return
	}
	w, _ := c.Size()
	panelW := opts.Width
	if panelW <= 0 {
		panelW = float32(tok.Spacing48) * 10.8
	}
	if w > 0 && panelW > w*0.56 {
		panelW = max(320, w*0.56)
	}

	root := ui.Row(c.Key(key)).Absolute().Fill()
	root.Children(func() {
		scrim := ui.ButtonBase(c.Key("sidepeek-scrim")).Grow(1).FillHeight().
			Label("关闭侧滑详情").
			Background(tok.Scrim).
			Transition(theme.MobileOverlayFadeTransition())
		if scrim.Clicked() {
			*open = false
		}

		panel := ui.Column(c.Key("sidepeek-panel")).
			Width(panelW).FillHeight().Clip().
			Transition(theme.ModalDialogTransition())
		ApplyElevation(panel, tok, ElevationModal)
		panel.Children(func() {
			ui.Row(c).FillWidth().
				Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
				AlignItems(ui.Center).
				Gap(float32(tok.Spacing8)).Children(func() {
				if opts.Icon != nil {
					ui.Icon(c, opts.Icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent).Shrink(0)
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, opts.Title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
					if opts.Subtitle != "" {
						ui.Text(c, opts.Subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
					}
				})
				if opts.HeaderActions != nil {
					ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(opts.HeaderActions)
				}
				if components.IconButton(c, lucide.X(), "关闭侧滑面板", components.IconButtonPlain).Clicked() {
					*open = false
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)

			ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
					if body != nil {
						body()
					}
				})
			})

			if opts.Footer != nil {
				ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)
				ui.Row(c).FillWidth().
					Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
					AlignItems(ui.Center).
					Gap(float32(tok.Spacing8)).Children(opts.Footer)
			}
		})
	})
}

func renderMobileSidePeekPage(c *ui.Context, key any, tok theme.Tokens, open *bool, opts SidePeekOptions, body func()) {
	t := c.Theme()
	topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
	ui.Column(c.Key(key)).Absolute().Fill().Modal().Background(tok.Canvas).
		Transition(theme.MobilePagePushTransition()).Children(func() {
		ui.Row(c).FillWidth().
			Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
			MinHeight(t.Space(12)+topSafe).
			AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
			if components.Button(c.Key("sidepeek-mb-back"), "返回", components.ButtonGhost).Shrink(0).Clicked() {
				*open = false
			}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, opts.Title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
				if opts.Subtitle != "" {
					ui.Text(c, opts.Subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				}
			})
			if opts.HeaderActions != nil {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(t.Space(1)).Children(opts.HeaderActions)
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)

		ui.Scroll(c.Key("sidepeek-mb-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			ui.Column(c).FillWidth().
				Padding(t.Space(4), t.Space(4)+rightSafe, t.Space(6)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(3)).Children(func() {
				if body != nil {
					body()
				}
			})
		})

		if opts.Footer != nil {
			ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)
			ui.Row(c).FillWidth().Wrap().
				Padding(t.Space(3), t.Space(4)+rightSafe, t.Space(3)+bottomSafe, t.Space(4)+leftSafe).
				AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(opts.Footer)
		}
	})
}
