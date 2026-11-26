package kit

import (
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// ListItemOptions configures a standardized Zed-style row with StartSlot, main labels,
// persistent EndSlot, and zero-jitter HoverEndSlot swap.
type ListItemOptions struct {
	Label        string
	Title        string
	Subtitle     string
	IndentLevel  int
	Selected     bool
	Active       bool
	Disabled     bool
	Compact      bool
	StartSlot    func()
	EndSlot      func()
	HoverEndSlot func()
	OnClick      func()
}

// ListItem renders a unified selectable list row across file trees, command palettes,
// outline panels, and completion popups with consistent density, hover, and focus ring.
func ListItem(c *ui.Context, key any, tok theme.Tokens, opts ListItemOptions) ui.Element {
	rowBtn := ui.ButtonBase(c.Key(key)).
		FillWidth().
		MinWidth(0).
		Justify(ui.Start).
		AlignItems(ui.Center).
		Radius(float32(tok.ControlRadius)).
		Disabled(opts.Disabled)
	if opts.Label != "" {
		rowBtn.Label(opts.Label)
	} else if opts.Title != "" {
		rowBtn.Label(opts.Title)
	}
	if opts.Compact {
		rowBtn.Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8))
	} else {
		rowBtn.Padding(float32(tok.Spacing4), float32(tok.Spacing8))
	}

	targetBg := ui.Transparent
	if opts.Selected || opts.Active {
		targetBg = tok.GhostSelected
	} else if !opts.Disabled && rowBtn.Pressed() {
		targetBg = tok.GhostActive
	} else if !opts.Disabled && rowBtn.Hovered() {
		targetBg = tok.GhostHover
	}
	rowBtn.Background(rowBtn.AnimateColor("kit-li-bg", targetBg, theme.MotionInstant))
	ApplyFocusRing(rowBtn, tok)
	theme.ApplyTactilePress(rowBtn)

	indentPx := float32(opts.IndentLevel) * float32(tok.Spacing12)
	rowBtn.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).
			Padding(0, 0, 0, indentPx).
			Gap(float32(tok.Spacing8)).Children(func() {
			if opts.Selected {
				ui.Box(c).Width(2).Height(float32(tok.Spacing16)-2).Radius(float32(tok.PillRadius)).Background(tok.Accent).Shrink(0)
			}
			if opts.StartSlot != nil {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(opts.StartSlot)
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				titleCol := tok.Text
				if opts.Disabled {
					titleCol = tok.TextDisabled
				} else if opts.Selected {
					titleCol = tok.Accent
				}
				titleTxt := ui.Text(c, opts.Title).
					FontSize(float32(tok.TextData)).
					TextColor(titleCol).
					SingleLine().Ellipsis("…")
				if opts.Selected || opts.Active {
					titleTxt.Bold()
				}
				if opts.Subtitle != "" {
					ui.Text(c, opts.Subtitle).
						FontSize(float32(tok.TextMeta)).
						TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…")
				}
			})
			showHover := rowBtn.Hovered() && !opts.Disabled && opts.HoverEndSlot != nil
			if showHover {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(opts.HoverEndSlot)
			} else if opts.EndSlot != nil {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(opts.EndSlot)
			}
		})
	})

	if !opts.Disabled && rowBtn.Clicked() && opts.OnClick != nil {
		opts.OnClick()
	}
	return rowBtn
}
