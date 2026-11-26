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

func TestMobileMotionTokensAndTransitions(t *testing.T) {
	if MotionDrawer.Spring.Duration == 0 || MotionPagePush.Spring.Duration == 0 || !MotionEffect.NonMotion {
		t.Fatalf("expected mobile motion tokens initialized properly")
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(DarkPalette())
		ui.Box(c).Key("page").Transition(MobilePagePushTransition())
		ui.Box(c).Key("scrim").Transition(MobileOverlayFadeTransition())
		ui.Box(c).Key("tabs").Transition(MobileTabOverviewTransition())
		ui.Box(c).Key("acc").Transition(MobileInlineAccordionTransition())
		ui.Box(c).Key("comp").Transition(MobileComposerCardTransition())
		ui.Box(c).Key("dock").Transition(MobileDockCrossfadeTransition())
	}, 402, 874)
	tt.Frame()

	SetAccessibilityPrefs(1.0, true)
	defer SetAccessibilityPrefs(1.0, false)
	if MotionDrawer.Duration != instantMotion.Duration {
		t.Fatalf("expected ReduceMotion to collapse MotionDrawer to instant")
	}
	if !MotionEffect.NonMotion {
		t.Fatalf("expected MotionEffect to keep NonMotion=true under ReduceMotion")
	}
}
