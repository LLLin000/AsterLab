package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMotionTokensAndHelpersRenderCleanly(t *testing.T) {
	nudgeSeq := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(LightPalette())
		box := ui.Column(c).Key("root").Transition(AccordionTransition())
		box.Children(func() {
			btn := ui.ButtonBase(c).Key("btn").Label("点按")
			ApplyTactilePress(btn)
			ApplyNudge(btn, nudgeSeq)
			btn.Children(func() {
				ui.Text(c, "14:59").ContentTransition(NumericRoll(true))
			})
		})
	}, 400, 300)
	tt.Frame()
	nudgeSeq = 1
	tt.Frame()
}
