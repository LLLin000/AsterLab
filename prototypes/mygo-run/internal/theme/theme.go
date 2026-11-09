package theme

import (
	"github.com/egoist/mygo/ui"
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
	Spacing4, Spacing8, Spacing12, Spacing16, Spacing24, Spacing32, Spacing48                    int
	TextPage, TextSection, TextBody, TextMeta, TextData                                          int
	ButtonHeight, InputHeight, RowHeight                                                         int
	ControlRadius, PanelRadius, DialogRadius                                                     int
}

func LightTokens() Tokens {
	t := Tokens{
		Canvas: ui.Hex("#F6F4EF"), Surface: ui.Hex("#FBFAF7"), Elevated: ui.Hex("#FFFFFF"),
		Border: ui.Hex("#DDD7CC"), BorderStrong: ui.Hex("#C8C0B2"),
		Text: ui.Hex("#201E1A"), TextMuted: ui.Hex("#6F6A60"),
		Accent: ui.Hex("#1F6F5F"), AccentHover: ui.Hex("#195C4F"), AccentText: ui.Hex("#FFFFFF"),
		Success: ui.Hex("#2F7D4A"), Warning: ui.Hex("#A9651D"), Danger: ui.Hex("#A13B36"),
		RailBackground: ui.Hex("#FBFAF7"), SidebarBackground: ui.Hex("#FBFAF7"),
		Spacing4: 4, Spacing8: 8, Spacing12: 12, Spacing16: 16, Spacing24: 24, Spacing32: 32, Spacing48: 48,
		TextPage: 20, TextSection: 15, TextBody: 13, TextMeta: 11, TextData: 12,
		ButtonHeight: 32, InputHeight: 32, RowHeight: 32,
		ControlRadius: 6, PanelRadius: 8, DialogRadius: 12,
	}
	return aliases(t)
}

func DarkTokens() Tokens {
	t := LightTokens()
	t.Canvas, t.Surface, t.Elevated = ui.Hex("#171614"), ui.Hex("#201E1A"), ui.Hex("#28251F")
	t.Border, t.BorderStrong = ui.Hex("#39352D"), ui.Hex("#514B40")
	t.Text, t.TextMuted = ui.Hex("#F0EDE6"), ui.Hex("#A8A196")
	t.Accent, t.AccentHover, t.AccentText = ui.Hex("#4FA38D"), ui.Hex("#60B39D"), ui.Hex("#10201C")
	t.Success, t.Warning, t.Danger = ui.Hex("#55A76D"), ui.Hex("#D5964D"), ui.Hex("#D26B62")
	t.RailBackground, t.SidebarBackground = t.Surface, t.Surface
	return aliases(t)
}

func aliases(t Tokens) Tokens {
	t.CardBackground, t.CardBorder = t.Elevated, t.Border
	t.SuccessBg, t.SuccessText = t.Surface, t.Success
	t.WarningBg, t.WarningText = t.Surface, t.Warning
	t.DangerBg, t.DangerText = t.Surface, t.Danger
	t.NeutralBg, t.NeutralText = t.Surface, t.TextMuted
	return t
}

func GetTokens(t *ui.Theme) Tokens {
	if t != nil && t.Dark {
		return DarkTokens()
	}
	return LightTokens()
}

func LightTheme() *ui.Theme {
	return apply(ui.LightTheme(), LightTokens(), false)
}

func DarkTheme() *ui.Theme {
	return apply(ui.DarkTheme(), DarkTokens(), true)
}

func apply(t *ui.Theme, c Tokens, dark bool) *ui.Theme {
	t.Dark = dark
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = c.Canvas, c.Surface, c.Elevated, c.Border
	t.Border, t.Text, t.TextMuted = c.Border, c.Text, c.TextMuted
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = c.Accent, c.AccentHover, c.Accent, c.AccentText
	t.Danger, t.Warning, t.Success, t.Focus = c.Danger, c.Warning, c.Success, c.Accent
	t.FocusRingWidth, t.FocusRingOffset = 2, 1
	t.Radius, t.Spacing = float32(c.PanelRadius), 4
	return t
}
