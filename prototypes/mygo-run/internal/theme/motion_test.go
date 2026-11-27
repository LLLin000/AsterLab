package theme

import (
	"testing"
	"time"

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

func TestSpatialEmergenceMotionGrammarAndReduceMotion(t *testing.T) {
	SetAccessibilityPrefs(1.0, false)
	defer SetAccessibilityPrefs(1.0, false)

	if MotionQuickPalette.Duration != 95*time.Millisecond {
		t.Fatalf("MotionQuickPalette duration = %v, want 95ms", MotionQuickPalette.Duration)
	}
	if MotionPopover.Spring.Duration != 150*time.Millisecond || MotionPopover.Spring.Bounce != 0 {
		t.Fatalf("MotionPopover spring = %+v, want 150ms bounce 0", MotionPopover.Spring)
	}
	if MotionModal.Spring.Duration != 190*time.Millisecond {
		t.Fatalf("MotionModal spring = %+v, want 190ms", MotionModal.Spring)
	}
	if MotionLightbox.Spring.Duration != 220*time.Millisecond {
		t.Fatalf("MotionLightbox spring = %+v, want 220ms", MotionLightbox.Spring)
	}
	if MotionSheetBackdrop.Spring.Duration != 340*time.Millisecond || SheetBackdropScale != float32(0.94) {
		t.Fatalf("MotionSheetBackdrop=%+v SheetBackdropScale=%v", MotionSheetBackdrop.Spring, SheetBackdropScale)
	}

	qp := QuickPaletteTransition()
	ap := AnchoredPopoverTransition()
	md := ModalDialogTransition()
	lb := DarkroomLightboxTransition()
	cs := ContactSheetCardTransition()

	if qp.Enter == nil || qp.Enter.Scale != 0.98 || qp.ExitDuration >= MotionQuickPalette.Duration {
		t.Fatalf("QuickPaletteTransition unexpected: %+v", qp)
	}
	if ap.Enter == nil || ap.Enter.Scale != 0.96 || ap.Enter.Y != -6 || !ap.Enter.Flip {
		t.Fatalf("AnchoredPopoverTransition unexpected: %+v", ap.Enter)
	}
	if md.Enter == nil || md.Enter.Scale != 0.96 {
		t.Fatalf("ModalDialogTransition unexpected: %+v", md.Enter)
	}
	if lb.Enter == nil || lb.Enter.Scale != 0.93 {
		t.Fatalf("DarkroomLightboxTransition unexpected: %+v", lb.Enter)
	}
	if cs.Enter == nil || cs.Enter.Scale != 0.96 {
		t.Fatalf("ContactSheetCardTransition unexpected: %+v", cs.Enter)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(DarkPalette())
		bg := ui.Box(c).Key("bg")
		ApplySheetBackdropDepth(bg, true)
		ui.Box(c).Key("qp").Transition(qp)
		ui.Box(c).Key("ap").Transition(ap)
		ui.Box(c).Key("md").Transition(md)
		ui.Box(c).Key("lb").Transition(lb)
		ui.Box(c).Key("cs").Transition(cs)
	}, 402, 874)
	tt.Frame()

	// Verify ReduceMotion=true degradation across all spatial presets & transition constructors.
	SetAccessibilityPrefs(1.0, true)
	for name, anim := range map[string]ui.Animation{
		"MotionQuickPalette":  MotionQuickPalette,
		"MotionPopover":       MotionPopover,
		"MotionModal":         MotionModal,
		"MotionLightbox":      MotionLightbox,
		"MotionSheetBackdrop": MotionSheetBackdrop,
	} {
		if anim.Duration != instantMotion.Duration {
			t.Fatalf("%s did not degrade to instantMotion under ReduceMotion: %+v", name, anim)
		}
	}

	for name, tr := range map[string]ui.ElementTransition{
		"QuickPaletteTransition":     QuickPaletteTransition(),
		"AnchoredPopoverTransition":  AnchoredPopoverTransition(),
		"ModalDialogTransition":      ModalDialogTransition(),
		"DarkroomLightboxTransition": DarkroomLightboxTransition(),
		"ContactSheetCardTransition": ContactSheetCardTransition(),
	} {
		if tr.Animation == nil || !tr.Animation.NonMotion {
			t.Fatalf("%s should use NonMotion fade animation under ReduceMotion: %+v", name, tr.Animation)
		}
		if tr.Enter == nil || tr.Enter.Scale != 0 || tr.Enter.Y != 0 || tr.Enter.X != 0 || tr.Enter.Opacity != 0 {
			t.Fatalf("%s Enter should degrade to opacity-only fade under ReduceMotion: %+v", name, tr.Enter)
		}
		if tr.Exit == nil || tr.Exit.Scale != 0 || tr.Exit.Y != 0 || tr.Exit.X != 0 || tr.Exit.Opacity != 0 {
			t.Fatalf("%s Exit should degrade to opacity-only fade under ReduceMotion: %+v", name, tr.Exit)
		}
	}
}
