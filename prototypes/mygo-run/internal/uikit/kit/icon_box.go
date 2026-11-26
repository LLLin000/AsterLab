package kit

import (
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// IconBox renders icon inside a fixed 24x24 transparent optical bounding box
// (scaled by tok.IconBoxSize) so varying SVG glyph aspect ratios stay optically aligned.
func IconBox(c *ui.Context, tok theme.Tokens, icon *ui.SVG, color ui.Color) ui.Element {
	return IconBoxSized(c, tok, icon, float32(tok.IconBoxSize), float32(tok.IconMd), color)
}

// IconBoxSized renders icon centered inside a fixed boxSize x boxSize transparent frame.
func IconBoxSized(c *ui.Context, tok theme.Tokens, icon *ui.SVG, boxSize, iconSize float32, color ui.Color) ui.Element {
	if boxSize <= 0 {
		boxSize = float32(tok.IconBoxSize)
	}
	if iconSize <= 0 {
		iconSize = float32(tok.IconMd)
	}
	box := ui.Box(c).Size(boxSize, boxSize).Center().Shrink(0)
	if icon != nil {
		box.Children(func() {
			ui.Icon(c, icon).Size(iconSize, iconSize).TextColor(color)
		})
	}
	return box
}
