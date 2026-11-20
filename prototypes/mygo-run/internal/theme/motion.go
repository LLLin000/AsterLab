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
)

func applyMotionPreference(reduce bool) {
	if reduce {
		MotionMicro = instantMotion
		MotionSnappy = instantMotion
		MotionFluid = instantMotion
		MotionHero = instantMotion
		return
	}
	MotionMicro = baseMotionMicro
	MotionSnappy = baseMotionSnappy
	MotionFluid = baseMotionFluid
	MotionHero = baseMotionHero
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
