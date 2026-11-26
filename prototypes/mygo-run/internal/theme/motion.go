package theme

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Global 4-tier motion tokens for AsterLab's tactile desktop workbench.
// Every spring preserves velocity on interruption, mixes colors in Oklab space,
// and automatically respects OS ReduceMotion preferences via mygo/ui.
var (
	baseMotionMicro = ui.Animation{
		Duration: 100 * time.Millisecond,
		Ease:     ui.EaseOut,
		Oklab:    true,
	}
	baseMotionMicroFade = ui.Animation{
		Duration:  110 * time.Millisecond,
		Ease:      ui.EaseOut,
		Oklab:     true,
		NonMotion: true,
	}
	baseMotionSnappy = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 160 * time.Millisecond,
			Bounce:   0,
		},
		Oklab: true,
	}
	baseMotionFluid = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 220 * time.Millisecond,
			Bounce:   0.04,
		},
		Oklab: true,
	}
	baseMotionHero = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 260 * time.Millisecond,
			Bounce:   0.03,
		},
		Oklab: true,
	}
	instantMotion = ui.Animation{
		Duration: 1 * time.Millisecond,
		Ease:     ui.EaseOut,
		Oklab:    true,
	}
	baseMotionInstant = ui.Animation{
		Duration: 60 * time.Millisecond,
		Ease:     ui.EaseOut,
		Oklab:    true,
	}

	// Mobile Motion System tokens (Spatial vs Effects dual-track).
	baseMotionTouchPress = ui.Animation{
		Duration: 90 * time.Millisecond,
		Ease:     ui.EaseOut,
		Oklab:    true,
	}
	baseMotionEffect = ui.Animation{
		Duration:  180 * time.Millisecond,
		Ease:      ui.EaseOut,
		Oklab:     true,
		NonMotion: true,
	}
	baseMotionInline = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 260 * time.Millisecond,
			Bounce:   0,
		},
		Oklab: true,
	}
	baseMotionDrawer = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 300 * time.Millisecond,
			Bounce:   0,
		},
		Oklab: true,
	}
	baseMotionPagePush = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 330 * time.Millisecond,
			Bounce:   0,
		},
		Oklab: true,
	}
	baseMotionSheet = ui.Animation{
		Spring: ui.SpringOptions{
			Duration: 340 * time.Millisecond,
			Bounce:   0.04,
		},
		Oklab: true,
	}

	// MotionInstant drives high-frequency hover/tab color snaps per the frequency-tiered motion law.
	MotionInstant = baseMotionInstant

	// MotionMicro drives immediate tactile press states (:active 0.97) and hover reveals.
	MotionMicro = baseMotionMicro

	// MotionMicroFade drives pure opacity/color shifts that remain smooth even under ReduceMotion.
	MotionMicroFade = baseMotionMicroFade

	// MotionSnappy is a critically damped spring (~160ms, zero overshoot) for magnetic
	// shared-layout pills, well Oklab dyeing, and crisp popover/bar entrances.
	MotionSnappy = baseMotionSnappy

	// MotionFluid is a gentle physical spring (~220ms, subtle 0.04 bounce) for accordion
	// height expand/collapse, liquid split drop-zone morphing, and numeric rolling wheels.
	MotionFluid = baseMotionFluid

	// MotionHero drives spatial continuity across larger distances (figure & plate lightbox zoom).
	MotionHero = baseMotionHero

	// MotionTouchPress drives mobile 90ms :active press feedback.
	MotionTouchPress = baseMotionTouchPress

	// MotionEffect drives pure opacity/scrim/color transitions (zero spatial overshoot, NonMotion).
	MotionEffect = baseMotionEffect

	// MotionInline drives mobile compact inline accordions and dock <-> edit bar crossfades.
	MotionInline = baseMotionInline

	// MotionDrawer drives symmetric 1:1 mobile drawer open/close settle springs.
	MotionDrawer = baseMotionDrawer

	// MotionPagePush drives full-screen mobile page push/pop transitions.
	MotionPagePush = baseMotionPagePush

	// MotionSheet drives mobile bottom sheets and tab overview spatial scaling.
	MotionSheet = baseMotionSheet
)

func applyMotionPreference(reduce bool) {
	if reduce {
		MotionInstant = instantMotion
		MotionMicro = instantMotion
		MotionSnappy = instantMotion
		MotionFluid = instantMotion
		MotionHero = instantMotion
		MotionTouchPress = instantMotion
		MotionInline = instantMotion
		MotionDrawer = instantMotion
		MotionPagePush = instantMotion
		MotionSheet = instantMotion
		return
	}
	MotionInstant = baseMotionInstant
	MotionMicro = baseMotionMicro
	MotionSnappy = baseMotionSnappy
	MotionFluid = baseMotionFluid
	MotionHero = baseMotionHero
	MotionTouchPress = baseMotionTouchPress
	MotionEffect = baseMotionEffect
	MotionInline = baseMotionInline
	MotionDrawer = baseMotionDrawer
	MotionPagePush = baseMotionPagePush
	MotionSheet = baseMotionSheet
}

// ReducedMotionEnabled reports whether in-app reduced motion is active.
func ReducedMotionEnabled() bool {
	_, rm := AccessibilityPrefs()
	return rm
}

// AccordionTransition smoothly expands and collapses list items, folder children,
// inline deviation inputs, and newly inserted document blocks without jarring layout jumps.
func AccordionTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionFluid,
		ExitDuration: 115 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Collapse: true, Opacity: 0, Y: -4},
		Exit:         &ui.Motion{Collapse: true, Opacity: 0, Y: -4},
	}
}

// FloatingBarTransition lifts a contextual toolbar (such as the plate annotation bar)
// from 0.97 scale + 6px below, and tucks it away swiftly in 110ms when dismissed.
func FloatingBarTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionSnappy,
		ExitDuration: 110 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Collapse: true, Opacity: 0, Y: 6, Scale: 0.97},
		Exit:         &ui.Motion{Collapse: true, Opacity: 0, Y: 4, Scale: 0.98},
	}
}

// DialogSurfaceTransition gives custom overlay surfaces an origin-grounded 0.96 -> 1.0 entrance
// and a fast 110ms exit (never scaling from 0).
func DialogSurfaceTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionSnappy,
		ExitDuration: 110 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Opacity: 0, Scale: 0.96, Y: 6},
		Exit:         &ui.Motion{Opacity: 0, Scale: 0.98, Y: 4},
	}
}

// SharedPillTransition creates a magnetic shared-geometry transition (`layoutId` / `matchedGeometryEffect`)
// for active tab indicators, command palette cursor highlight, and plate layer pills.
func SharedPillTransition(matchKey any) ui.ElementTransition {
	return ui.ElementTransition{
		Match:     matchKey,
		Animation: &MotionSnappy,
	}
}

// LiquidSplitTransition morphs the drag-to-split preview rectangle continuously across
// left / right / top / bottom / center drop zones within a pane.
func LiquidSplitTransition(matchKey any) ui.ElementTransition {
	return ui.ElementTransition{
		Match:        matchKey,
		Animation:    &MotionFluid,
		ExitDuration: 100 * time.Millisecond,
		Enter:        &ui.Motion{Opacity: 0, Scale: 0.96},
		Exit:         &ui.Motion{Opacity: 0, Scale: 0.98},
	}
}

// HeroFlyTransition flies a thumbnail figure or inline plate smoothly into its enlarged
// lightbox modal and back into its exact document slot on close.
func HeroFlyTransition(matchKey any) ui.ElementTransition {
	return ui.ElementTransition{
		Match:        matchKey,
		Fly:          true,
		ScaleContent: true,
		Animation:    &MotionHero,
	}
}

// MobilePagePushTransition pushes full-screen mobile overlay views in from the trailing
// edge with a critically damped MotionPagePush spring and exits swiftly in 200ms.
func MobilePagePushTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionPagePush,
		ExitDuration: 200 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{XPercent: 1.0, Opacity: 1},
		Exit:         &ui.Motion{XPercent: 1.0, Opacity: 1},
	}
}

// MobileOverlayFadeTransition drives pure scrim/backdrop opacity without spatial overshoot.
func MobileOverlayFadeTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionEffect,
		ExitDuration: 150 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Opacity: 0},
		Exit:         &ui.Motion{Opacity: 0},
	}
}

// MobileTabOverviewTransition scales the mobile tab switcher cleanly into view.
func MobileTabOverviewTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionSheet,
		ExitDuration: 180 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Opacity: 0, Scale: 0.94},
		Exit:         &ui.Motion{Opacity: 0, Scale: 0.96},
	}
}

// MobileInlineAccordionTransition collapses/expands inline mobile footnotes cleanly.
func MobileInlineAccordionTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionInline,
		ExitDuration: 170 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Collapse: true, Opacity: 0, Y: -4},
		Exit:         &ui.Motion{Collapse: true, Opacity: 0, Y: -4},
	}
}

// MobileComposerCardTransition lifts keyboard-docked mobile floating composers cleanly.
func MobileComposerCardTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionInline,
		ExitDuration: 160 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Opacity: 0, Y: 14, Scale: 0.97},
		Exit:         &ui.Motion{Opacity: 0, Y: 10, Scale: 0.98},
	}
}

// MobileDockCrossfadeTransition crossfades between Navigation Dock and Keyboard Edit Toolbar.
func MobileDockCrossfadeTransition() ui.ElementTransition {
	return ui.ElementTransition{
		Animation:    &MotionInline,
		ExitDuration: 140 * time.Millisecond,
		ExitEase:     ui.EaseOut,
		Enter:        &ui.Motion{Opacity: 0, Y: 8},
		Exit:         &ui.Motion{Opacity: 0, Y: 6},
	}
}

// NumericRoll returns a mechanical pipette-wheel digit rolling transition.
// Unchanged digits stay rock-solid while changed digits roll vertically.
func NumericRoll(countdown bool) ui.ContentTransition {
	return ui.ContentTransition{
		Numeric:   true,
		Down:      countdown,
		Animation: &MotionFluid,
	}
}

// IconMorph smoothly shrinks an outgoing icon/label while growing the replacement in.
func IconMorph() ui.ContentTransition {
	return ui.ContentTransition{
		Animation: &MotionSnappy,
	}
}

// ApplyTactilePress applies a subtle physical `:active` 0.97 press scale to interactive controls.
func ApplyTactilePress(elem ui.Element) ui.Element {
	if ReducedMotionEnabled() {
		return elem
	}
	target := float32(1.0)
	if elem.Pressed() {
		target = 0.97
	}
	s := elem.AnimateTo("tactile-press", target, MotionMicro)
	if s > 0 && s != 1.0 {
		elem.Scale(s, s)
	}
	return elem
}

// ApplyNudge imparts a gentle horizontal sponge shake (~±5px settling in ~200ms)
// whenever seq increments (e.g., submitting an empty required field or invalid factor),
// using a velocity-injected spring with zero timers or goroutines.
func ApplyNudge(elem ui.Element, seq int) ui.Element {
	if seq <= 0 || ReducedMotionEnabled() {
		return elem
	}
	target := float32(seq%2) * 0.001
	dx := elem.AnimateTo("tactile-nudge", target, ui.Animation{
		Spring: ui.SpringOptions{
			Mass:      1,
			Stiffness: 580,
			Damping:   16,
			Velocity:  240,
		},
	})
	if dx > 0.05 || dx < -0.05 {
		elem.Translate(dx, 0)
	}
	return elem
}
