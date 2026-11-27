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
		rimCol := tok.SpecularRim
		if rimCol.A == 0 {
			rimCol = tok.TopHighlight
		}
		hlCol := tok.TopHighlight
		rad := float32(tok.PanelRadius)
		if level == ElevationModal {
			rad = float32(tok.DialogRadius)
		}
		if rimCol.A > 0 || hlCol.A > 0 {
			el.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if rimCol.A > 0 && r.W > 4 && r.H > 4 {
					inner := ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
					p.Stroke(inner, rimCol, theme.ConcentricRadius(rad, 1), 1)
				}
				if hlCol.A > 0 && r.W > rad*2+4 {
					p.Line(r.X+rad*0.75, r.Y+1, r.X+r.W-rad*0.75, r.Y+1, 1, hlCol)
				}
			})
		}
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
