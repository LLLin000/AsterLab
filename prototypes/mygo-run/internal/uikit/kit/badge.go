package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// Badge renders a onevra Badge forced onto the horizontal row optical center line
// (fixing components.Badge's default AlignSelf(ui.Start) float when paired with buttons).
func Badge(c *ui.Context, text string, opts ...components.BadgeOptions) ui.Element {
	el := components.Badge(c, text, opts...)
	el.AlignSelf(ui.Center)
	return el
}
