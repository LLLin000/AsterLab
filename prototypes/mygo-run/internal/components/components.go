// Package components is AsterLab's native UI kit: quiet, dense, keyboard-first.
package components

import (
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

const (
	TextPage    = 20
	TextSection = 15
	TextBody    = 13
	TextMeta    = 11
	TextData    = 12
)

func PageTitle(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextPage)).Bold().TextColor(t.Text)
}

func SectionTitle(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextSection)).Bold().TextColor(t.Text)
}

func Body(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextBody)).LineHeight(1.5).TextColor(t.Text)
}

func Meta(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextMeta)).LineHeight(1.35).TextColor(t.TextMuted)
}

func Label(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
}

func Value(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextBody)).TextColor(t.Text).FontFeatures("tnum")
}

func Data(c *ui.Context, text string) {
	t := theme.GetTokens(c.Theme())
	ui.Text(c, text).FontSize(float32(t.TextData)).TextColor(t.Text).FontFeatures("tnum")
}

func Divider(c *ui.Context) { ui.Divider(c) }

func baseButton(c *ui.Context, label string, primary, ghost bool) bool {
	t := theme.GetTokens(c.Theme())
	bg, fg, border := t.Elevated, t.Text, t.Border
	if ghost {
		bg, fg, border = ui.Color{}, t.TextMuted, ui.Color{}
	}
	if primary {
		bg, fg, border = t.Accent, t.AccentText, t.Accent
	}
	b := ui.ButtonBase(c).Height(float32(t.ButtonHeight)).Radius(float32(t.ControlRadius)).
		Background(bg).Border(1, border).Padding(0, 12)
	b.Children(func() {
		ui.Text(c, label).FontSize(float32(t.TextBody)).Bold().TextColor(fg)
	})
	return b.Clicked()
}

func PrimaryButton(c *ui.Context, label string) bool   { return baseButton(c, label, true, false) }
func SecondaryButton(c *ui.Context, label string) bool { return baseButton(c, label, false, false) }
func GhostButton(c *ui.Context, label string) bool     { return baseButton(c, label, false, true) }

// Button preserves the v1.1 API while using the native kit's default styling.
func Button(c *ui.Context, label string) bool { return SecondaryButton(c, label) }

// Input renders a labelled field with help/error text below it.
func Input(c *ui.Context, value *string, placeholder string) bool {
	return ui.TextInput(c, value).Placeholder(placeholder).Height(32).Changed()
}

func Field(c *ui.Context, label string, value *string, placeholder string) bool {
	changed := false
	ui.Column(c).Gap(4).Children(func() {
		Label(c, label)
		if Input(c, value, placeholder) {
			changed = true
		}
	})
	return changed
}

// Tabs is a compact segmented control. Selection is an inset surface, not a
// saturated button; one 1px line separates it from the page.
func Tabs(c *ui.Context, labels []string, selected *int) bool {
	changed := false
	t := theme.GetTokens(c.Theme())
	ui.Row(c).Gap(2).Padding(2).Radius(float32(t.ControlRadius)).Background(t.Surface).Border(1, t.Border).Children(func() {
		for i, label := range labels {
			active := i == *selected
			bg, fg := ui.Color{}, t.TextMuted
			if active {
				bg, fg = t.Elevated, t.Text
			}
			b := ui.ButtonBase(c).Height(26).Radius(float32(t.ControlRadius-1)).Background(bg).Padding(0, 10)
			b.Children(func() {
				ui.Text(c, label).FontSize(float32(t.TextMeta)).TextColor(fg)
			})
			if b.Clicked() {
				*selected, changed = i, true
			}
		}
	})
	return changed
}

// Table renders dense native rows; numeric cells should use Data.
func Table(c *ui.Context, headers []string, rows [][]string) {
	t := theme.GetTokens(c.Theme())
	ui.Column(c).Gap(0).Children(func() {
		ui.Row(c).Height(float32(t.RowHeight)).Gap(12).Border(1, t.Border).Children(func() {
			for _, h := range headers {
				Meta(c, h)
			}
		})
		for _, row := range rows {
			row := row
			ui.Row(c).Height(float32(t.RowHeight)).Gap(12).Children(func() {
				for _, cell := range row {
					Data(c, cell)
				}
			})
		}
	})
}

// Status communicates state with text and a non-color cue.
type StatusKind uint8

const (
	StatusReady StatusKind = iota
	StatusEmpty
	StatusLoading
	StatusError
)

func Status(c *ui.Context, kind StatusKind, message string) {
	t := theme.GetTokens(c.Theme())
	color, mark := t.TextMuted, "•"
	switch kind {
	case StatusError:
		color, mark = t.Danger, "!"
	case StatusLoading:
		mark = "…"
	case StatusReady:
		color, mark = t.Success, "✓"
	}
	ui.Row(c).Gap(6).Children(func() {
		ui.Text(c, mark).FontSize(float32(t.TextMeta)).TextColor(color)
		Meta(c, message)
	})
}
