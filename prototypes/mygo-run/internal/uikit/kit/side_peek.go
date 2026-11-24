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

// SidePeek renders a right-hand detail drawer keeping the left workspace visible.
// Clicking the dim scrim or pressing Escape closes the drawer cleanly.
func SidePeek(c *ui.Context, key any, tok theme.Tokens, open *bool, opts SidePeekOptions, body func()) {
	if c == nil || open == nil || !*open {
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
			Background(tok.Text.Alpha(0.14))
		if scrim.Clicked() {
			*open = false
		}

		panel := ui.Column(c.Key("sidepeek-panel")).
			Width(panelW).FillHeight().Clip()
		ApplyElevation(panel, tok, ElevationModal)
		panel.Background(tok.Surface).Border(1, tok.BorderStrong)
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
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

			ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
					if body != nil {
						body()
					}
				})
			})

			if opts.Footer != nil {
				ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
				ui.Row(c).FillWidth().
					Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
					AlignItems(ui.Center).
					Gap(float32(tok.Spacing8)).Children(opts.Footer)
			}
		})
	})
}
