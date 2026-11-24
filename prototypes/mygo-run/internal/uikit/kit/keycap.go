package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// Keycap renders a single shortcut label or chord (e.g., "Ctrl+K" or "Ctrl+K / Ctrl+O")
// as clean keyboard keycap pills matching desktop IDE/notebook polish.
func Keycap(c *ui.Context, tok theme.Tokens, chord string) {
	chord = strings.TrimSpace(chord)
	if chord == "" {
		return
	}
	alts := strings.Split(chord, "/")
	ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Shrink(0).Children(func() {
		for ai, alt := range alts {
			alt = strings.TrimSpace(alt)
			if alt == "" {
				continue
			}
			if ai > 0 {
				ui.Text(c, "/").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
			}
			keys := strings.Split(alt, "+")
			ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
				for _, k := range keys {
					k = strings.TrimSpace(k)
					if k == "" {
						continue
					}
					ui.Row(c).
						Padding(1, float32(tok.Spacing4)+1).
						Radius(float32(tok.ControlRadius)-2).
						Background(tok.ElementBg).
						Border(1, tok.Border).
						Center().Children(func() {
						ui.Text(c, k).
							FontSize(float32(tok.TextMeta)).
							TextColor(tok.TextMuted).
							FontFeatures("tnum").
							SingleLine()
					})
				}
			})
		}
	})
}
