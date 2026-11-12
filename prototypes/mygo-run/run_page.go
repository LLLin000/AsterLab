package main

// Run surface: the frozen execution workspace. Owned by the Run Owner after
// Wave 1 extraction; routing stays in main.go.

import (
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/runworkspace"
	"github.com/egoist/mygo/ui"
)

// runPage renders the selected Run: an optional frozen document strip, then the
// workspace, a stored readout batch, or the legacy UI in that order.
func runPage(c *ui.Context, a *app) {
	ui.Column(c).Fill().Children(func() {
		a.documentToolbar(c)
		if a.showFrozenDoc && a.snap.AsterDoc != nil {
			ui.Scroll(c).Height(260).Children(func() {
				ui.Box(c).FillWidth().Background(c.Theme().Surface).
					Padding(c.Theme().Space(3), c.Theme().Space(4)).Children(func() {
					a.renderFrozenAsterDoc(c, asterdoc.ModeRun)
				})
			})
		}
		if a.batchView != "" {
			batchPage(c, a)
		} else if a.runWS != nil {
			runworkspace.View(c, a.runWS)
		} else if a.ws != nil {
			a.ws.Width = 1200
			runui.View(c, a.ws)
		} else {
			ui.Text(c, "加载中…")
		}
	})
}
