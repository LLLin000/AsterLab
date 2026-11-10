package protocol

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// markdownTheme is MyGo's reading view themed with AsterLab's tokens: slightly
// larger body, relaxed leading, a modest heading ladder and the single accent
// for links and quote bars. It styles the projection; it adds no content.
func markdownTheme(t *ui.Theme) *markdown.Theme {
	th := markdown.DefaultTheme(t)
	th.FontSize = t.Rem(1.02)
	th.LineHeight = 1.75
	th.BlockSpacing = 0.9
	th.ItemSpacing = 0.35
	th.StrongWeight = 600
	th.Headings = [6]markdown.HeadingStyle{
		{Size: 1.55, Weight: 600, Top: 0, Bottom: 0.7},
		{Size: 1.2, Weight: 600, Top: 1.6, Bottom: 0.5},
		{Size: 1.0, Weight: 600, Top: 1.3, Bottom: 0.35},
		{Size: 0.95, Weight: 600, Top: 1.0, Bottom: 0.3},
		{Size: 0.9, Weight: 600, Top: 0.9, Bottom: 0.25},
		{Size: 0.85, Weight: 600, Top: 0.8, Bottom: 0.2},
	}
	th.QuoteBar = t.Accent.Alpha(0.5)
	th.QuoteBarWidth = 2
	th.QuoteText = t.TextMuted
	th.CodeBackground = t.Text.Alpha(0.06)
	th.Rule = t.Border
	return th
}
