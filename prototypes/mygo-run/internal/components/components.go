package components

import (
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// Button renders a native themed action and reports activation.
func Button(c *ui.Context, label string) bool { return ui.Button(c, label).Clicked() }

// Input renders a native text field and reports edits.
func Input(c *ui.Context, value *string, placeholder string) bool {
	return ui.TextInput(c, value).Placeholder(placeholder).Changed()
}

// Tabs renders a compact keyboard-focusable tab row; selected is the active index.
func Tabs(c *ui.Context, labels []string, selected *int) bool {
	changed := false
	ui.Row(c).Gap(4).Children(func() {
		for i, label := range labels {
			active := i == *selected
			bg, color := ui.Color{}, c.Theme().TextMuted
			if active { bg, color = theme.GetTokens(c.Theme()).Accent, theme.GetTokens(c.Theme()).AccentText }
			b := ui.ButtonBase(c).Radius(float32(theme.GetTokens(c.Theme()).ControlRadius)).Background(bg)
			b.Children(func() { ui.Row(c).Padding(6, 10).Center().Children(func() { ui.Text(c, label).FontSize(13).TextColor(color) }) })
			if b.Clicked() { *selected, changed = i, true }
		}
	})
	return changed
}

// Table renders dense native text rows without imposing a data model.
func Table(c *ui.Context, headers []string, rows [][]string) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Gap(12).Children(func() { for _, h := range headers { ui.Text(c, h).FontSize(12).Bold() } })
		for _, row := range rows {
			row := row
			ui.Row(c).Gap(12).Children(func() { for _, cell := range row { ui.Text(c, cell).FontSize(13) } })
		}
	})
}

// Status communicates loading, empty, error, and ready states consistently.
type StatusKind uint8
const (
	StatusReady StatusKind = iota
	StatusEmpty
	StatusLoading
	StatusError
)
func Status(c *ui.Context, kind StatusKind, message string) {
	t := c.Theme()
	color := t.TextMuted
	if kind == StatusError { color = theme.GetTokens(t).Danger }
	ui.Text(c, message).FontSize(12).TextColor(color)
}
