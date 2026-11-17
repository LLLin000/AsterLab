// Package theme is AsterLab's visual system: warm neutral surfaces, one violet
// accent for selection and primary action, hairline structure instead of card
// stacks. Palettes start from Onevra's warm theme and replace its colors, so
// Onevra components, MyGo widgets and AsterLab's own painters share one look.
package theme

import (
	"github.com/egoist/mygo/ui"
	onevra "github.com/oneo-me/onevra/source/theme"
)

// Tokens defines the shared AsterLab design system.
type Tokens struct {
	Canvas, Surface, Elevated                                                                    ui.Color
	Border, BorderStrong                                                                         ui.Color
	Text, TextMuted                                                                              ui.Color
	Accent, AccentHover, AccentText                                                              ui.Color
	Success, Warning, Danger                                                                     ui.Color
	RailBackground, SidebarBackground                                                            ui.Color
	CardBackground, CardBorder                                                                   ui.Color
	SuccessBg, SuccessText, WarningBg, WarningText, DangerBg, DangerText, NeutralBg, NeutralText ui.Color
	AccentSoftBg, InfoBg, InfoText, CodeBg                                                       ui.Color
	Spacing4, Spacing8, Spacing12, Spacing16, Spacing24, Spacing32, Spacing48                    int
	TextPage, TextSection, TextBody, TextMeta, TextData                                          int
	ButtonHeight, InputHeight, RowHeight                                                         int
	ControlRadius, PanelRadius, DialogRadius                                                     int
}

func LightTokens() Tokens {
	t := Tokens{
		Canvas: ui.Hex("#f7f6f3"), Surface: ui.Hex("#ffffff"), Elevated: ui.Hex("#faf9f7"),
		Border: ui.Hex("#e7e3db"), BorderStrong: ui.Hex("#d6d0c5"),
		Text: ui.Hex("#1c1b18"), TextMuted: ui.Hex("#5f5a52"),
		Accent: ui.Hex("#5b45c9"), AccentHover: ui.Hex("#4b37ae"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#2f7a4f"), Warning: ui.Hex("#7a6233"), Danger: ui.Hex("#a3373a"),
		RailBackground: ui.Hex("#f1efea"), SidebarBackground: ui.Hex("#f1efea"),
		Spacing4: 4, Spacing8: 8, Spacing12: 12, Spacing16: 16, Spacing24: 24, Spacing32: 32, Spacing48: 48,
		TextPage: 22, TextSection: 15, TextBody: 13, TextMeta: 12, TextData: 12,
		ButtonHeight: 30, InputHeight: 30, RowHeight: 32,
		ControlRadius: 6, PanelRadius: 10, DialogRadius: 12,
	}
	return aliases(t)
}

func DarkTokens() Tokens {
	t := LightTokens()
	t.Canvas, t.Surface, t.Elevated = ui.Hex("#171614"), ui.Hex("#1e1d1a"), ui.Hex("#242320")
	t.Border, t.BorderStrong = ui.Hex("#37352f"), ui.Hex("#4a473f")
	t.Text, t.TextMuted = ui.Hex("#f2efe9"), ui.Hex("#a9a49a")
	t.Accent, t.AccentHover, t.AccentText = ui.Hex("#a996ff"), ui.Hex("#bcabff"), ui.Hex("#1b1533")
	t.Success, t.Warning, t.Danger = ui.Hex("#84c9a4"), ui.Hex("#d9b878"), ui.Hex("#e79a9c")
	t.RailBackground, t.SidebarBackground = t.Canvas, t.Canvas
	return aliases(t)
}

func aliases(t Tokens) Tokens {
	t.CardBackground, t.CardBorder = t.Elevated, t.Border
	t.SuccessBg, t.SuccessText = t.Surface, t.Success
	t.WarningBg, t.WarningText = t.Surface, t.Warning
	t.DangerBg, t.DangerText = t.Surface, t.Danger
	t.NeutralBg, t.NeutralText = t.Surface, t.TextMuted
	t.AccentSoftBg = t.Accent.Alpha(.10)
	t.InfoBg, t.InfoText = t.Elevated, t.Accent
	t.CodeBg = t.Elevated
	return t
}

// GetTokens returns the painter tokens matching the frame's appearance.
func GetTokens(t *ui.Theme) Tokens {
	if t != nil && t.Dark {
		return DarkTokens()
	}
	return LightTokens()
}

// LightPalette is the AsterLab light theme.
func LightPalette() *ui.Theme { return apply(false) }

// DarkPalette is the AsterLab dark theme.
func DarkPalette() *ui.Theme { return apply(true) }

// Palette returns the AsterLab palette for the given appearance.
func Palette(dark bool) *ui.Theme {
	if dark {
		return DarkPalette()
	}
	return LightPalette()
}

// Window is the theme function Onevra's shells and Apply take: it follows the
// desktop's appearance, text scale and higher-contrast preference.
func Window(c *ui.Context) *ui.Theme {
	t := Palette(c.Theme().Dark)
	p := c.Preferences()
	if p.HighContrast {
		t.Border = t.Border.Mix(t.Text, 0.45)
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Scrollbar = t.Scrollbar.Alpha(1.6)
	}
	if s := p.TextScale; s > 0 && s != 1 {
		t.FontSize *= s
	}
	return t
}

// Apply installs the AsterLab palette for the current frame. Call it at the
// start of a root content builder that is not inside an Onevra shell.
func Apply(c *ui.Context) { c.SetTheme(Window(c)) }

// LightTheme and DarkTheme install a fixed appearance, for isolated studios.
func LightTheme() *ui.Theme { return LightPalette() }

func DarkTheme() *ui.Theme { return DarkPalette() }

func apply(dark bool) *ui.Theme {
	t := onevra.Palette(dark)
	c := LightTokens()
	if dark {
		c = DarkTokens()
	}
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = c.Canvas, c.Surface, c.Elevated, c.Border
	t.Border, t.Text, t.TextMuted = c.Border, c.Text, c.TextMuted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = c.Accent, c.AccentHover, c.AccentHover, c.AccentText
	t.Danger, t.Warning, t.Success = c.Danger, c.Warning, c.Success
	t.Selection = c.Accent.Alpha(.18)
	t.Focus = c.Accent.Alpha(.55)
	t.Scrollbar = c.Text.Alpha(.28)
	t.FocusRingWidth, t.FocusRingOffset = 2, 1
	t.Radius = 8
	t.Spacing = 4
	return t
}
