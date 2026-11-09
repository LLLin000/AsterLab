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
		Canvas: ui.Hex("#F8F9FA"), Surface: ui.Hex("#FFFFFF"), Elevated: ui.Hex("#F4F5F7"),
		Border: ui.Hex("#E5E7EB"), BorderStrong: ui.Hex("#CBD5E1"),
		Text: ui.Hex("#0F172A"), TextMuted: ui.Hex("#475569"),
		Accent: ui.Hex("#2563EB"), AccentHover: ui.Hex("#1D4ED8"), AccentText: ui.Hex("#FFFFFF"),
		Success: ui.Hex("#10B981"), Warning: ui.Hex("#F59E0B"), Danger: ui.Hex("#EF4444"),
		RailBackground: ui.Hex("#FFFFFF"), SidebarBackground: ui.Hex("#FFFFFF"),
		Spacing4: 4, Spacing8: 8, Spacing12: 12, Spacing16: 16, Spacing24: 24, Spacing32: 32, Spacing48: 48,
		TextPage: 20, TextSection: 15, TextBody: 13, TextMeta: 11, TextData: 12,
		ButtonHeight: 32, InputHeight: 32, RowHeight: 32,
		ControlRadius: 6, PanelRadius: 8, DialogRadius: 12,
	}
	return aliases(t)
}

func DarkTokens() Tokens {
	t := LightTokens()
	t.Canvas, t.Surface, t.Elevated = ui.Hex("#0B0C0E"), ui.Hex("#111317"), ui.Hex("#16191F")
	t.Border, t.BorderStrong = ui.Hex("#232832"), ui.Hex("#393F49")
	t.Text, t.TextMuted = ui.Hex("#F0F2F5"), ui.Hex("#9DA3AF")
	t.Accent, t.AccentHover, t.AccentText = ui.Hex("#4B88FF"), ui.Hex("#6098FF"), ui.Hex("#FFFFFF")
	t.Success, t.Warning, t.Danger = ui.Hex("#34D399"), ui.Hex("#F59E0B"), ui.Hex("#F87171")
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
