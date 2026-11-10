package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestProtocolNeverFallsBackToLiveDesignForSelectedRun(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{RunID: "run-1", DesignID: "design-1", DesignRevision: 3}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		protocolPage(c, a)
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("已选择 Run，但它的冻结设计缺失。") {
		t.Fatal("missing frozen design did not render its error state")
	}
	if !tt.HasText("不会改用实时设计") {
		t.Fatal("error state does not state that the live design is refused")
	}
}
