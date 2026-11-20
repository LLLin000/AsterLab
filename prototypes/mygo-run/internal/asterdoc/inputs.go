package asterdoc

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
)

// BorderlessTextInput builds a clean Notion/Obsidian-style inline text input on raw
// ui.TextInputBase: transparent and borderless at rest, with only a subtle animated
// surface wash on hover or focus so documents never look like boxed XP forms.
func BorderlessTextInput(c *ui.Context, tok theme.Tokens, value *string, fontSize float32, bold bool, placeholder string) ui.Element {
	ti := ui.TextInputBase(c, value).
		FillWidth().
		FontSize(fontSize).
		TextColor(tok.Text).
		Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing4)).
		Radius(float32(tok.ControlRadius)).
		Placeholder(placeholder)
	if bold {
		ti.Bold()
	}
	targetBg := ui.Transparent
	if ti.Focused() {
		targetBg = tok.Accent.Alpha(0.07)
	} else if ti.Hovered() {
		targetBg = tok.Text.Alpha(0.04)
	}
	ti.Background(ti.AnimateColor("bt-bg", targetBg, theme.MotionMicroFade))
	return ti
}

// StyleSelectQuiet transforms a standard bordered components.Select trigger into an
// unboxed inline property chip whose surface tint appears smoothly on hover/focus.
func StyleSelectQuiet[T comparable](c *ui.Context, tok theme.Tokens, sel *components.SelectParts[T]) {
	if sel == nil {
		return
	}
	b := sel.Trigger
	t := c.Theme()
	targetBg := ui.Transparent
	if b.Pressed() {
		targetBg = tok.Text.Alpha(0.08)
	} else if b.Hovered() || b.Focused() {
		targetBg = tok.Text.Alpha(0.045)
	}
	b.Background(b.AnimateColor("sq-bg", targetBg, theme.MotionMicroFade)).
		Border(0, ui.Transparent).
		Radius(float32(tok.ControlRadius)).
		Padding(t.Space(1), t.Space(5.5), t.Space(1), t.Space(1.5))
}
