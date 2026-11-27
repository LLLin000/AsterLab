// Package kit provides AsterLab's unified application-level UI primitives layered
// cleanly between theme tokens / onevra components and feature packages.
package kit

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// Elevation mirrors Zed GPUI's ElevationIndex for consistent surface hierarchy,
// shadows, borders, and dark-mode specular top highlights.
type Elevation uint8

const (
	ElevationCanvas Elevation = iota
	ElevationSurface
	ElevationCard
	ElevationFloating
	ElevationModal
)

// ApplyElevation styles el according to its semantic elevation tier using theme tokens.
func ApplyElevation(el ui.Element, tok theme.Tokens, level Elevation) ui.Element {
	switch level {
	case ElevationCanvas:
		el.Background(tok.Canvas)
	case ElevationSurface:
		el.Background(tok.Surface)
	case ElevationCard:
		rad := float32(tok.PanelRadius)
		el.Background(tok.Surface).
			Radius(rad).
			Border(1, tok.BorderSubtle).
			Shadow(0, 1, 3, 0, tok.ShadowAmbient)
	case ElevationFloating:
		rad := float32(tok.PanelRadius)
		el.Radius(rad).
			Border(1, tok.Border).
			Shadow(0, float32(tok.Spacing8), float32(tok.Spacing24), 0, tok.ShadowAmbient)
		if tok.LiquidGlass {
			el.Material(glass.Glass{Tint: tok.Surface.Alpha(0.95)})
		} else {
			el.Background(tok.Surface)
		}
	case ElevationModal:
		rad := float32(tok.DialogRadius)
		el.Radius(rad).
			Border(1, tok.BorderStrong).
			Shadow(0, float32(tok.Spacing12), float32(tok.Spacing32), 0, tok.ShadowAmbient)
		if tok.Dark {
			el.Background(tok.Surface)
		} else if tok.LiquidGlass {
			el.Material(glass.Glass{Tint: tok.Surface.Alpha(0.96)})
		} else {
			el.Background(tok.Surface)
		}
	}

	if tok.Dark && level >= ElevationCard {
		theme.ApplyTopLitBorder(el, tok, 1)
	}
	return el
}

// ApplyFocusRing adds a crisp accessible focus ring when keyboard-focused.
func ApplyFocusRing(el ui.Element, tok theme.Tokens) ui.Element {
	if el.FocusVisible() {
		el.Border(2, tok.BorderFocused)
	}
	return el
}
