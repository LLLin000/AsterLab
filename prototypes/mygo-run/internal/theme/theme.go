package theme

import (
	"github.com/egoist/mygo/ui"
)

// Tokens defines AsterLab unified theme configuration.
type Tokens struct {
	RailBackground    ui.Color
	SidebarBackground ui.Color
	CardBackground    ui.Color
	CardBorder        ui.Color
	SuccessBg         ui.Color
	SuccessText       ui.Color
	WarningBg         ui.Color
	WarningText       ui.Color
	DangerBg          ui.Color
	DangerText        ui.Color
	NeutralBg         ui.Color
	NeutralText       ui.Color
}

// LightTokens returns semantic colors for light theme.
func LightTokens() Tokens {
	return Tokens{
		RailBackground:    ui.Hex("#f1f5f9"),
		SidebarBackground: ui.Hex("#f8fafc"),
		CardBackground:    ui.Hex("#ffffff"),
		CardBorder:        ui.Hex("#e2e8f0"),
		SuccessBg:         ui.Hex("#ecfdf5"),
		SuccessText:       ui.Hex("#059669"),
		WarningBg:         ui.Hex("#fffbeb"),
		WarningText:       ui.Hex("#d97706"),
		DangerBg:          ui.Hex("#fff1f2"),
		DangerText:        ui.Hex("#e11d48"),
		NeutralBg:         ui.Hex("#f1f5f9"),
		NeutralText:       ui.Hex("#475569"),
	}
}

// DarkTokens returns semantic colors for dark theme.
func DarkTokens() Tokens {
	return Tokens{
		RailBackground:    ui.Hex("#090d16"),
		SidebarBackground: ui.Hex("#0e1526"),
		CardBackground:    ui.Hex("#131b2e"),
		CardBorder:        ui.Hex("#1e293b"),
		SuccessBg:         ui.Hex("#064e3b"),
		SuccessText:       ui.Hex("#34d399"),
		WarningBg:         ui.Hex("#78350f"),
		WarningText:       ui.Hex("#fbbf24"),
		DangerBg:          ui.Hex("#881337"),
		DangerText:        ui.Hex("#fda4af"),
		NeutralBg:         ui.Hex("#1e293b"),
		NeutralText:       ui.Hex("#94a3b8"),
	}
}

// GetTokens returns the semantic tokens matching the given theme.
func GetTokens(t *ui.Theme) Tokens {
	if t != nil && t.Dark {
		return DarkTokens()
	}
	return LightTokens()
}

// LightTheme returns a calibrated AsterLab light theme.
func LightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = ui.Hex("#f8fafc")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f1f5f9")
	t.SurfacePressed = ui.Hex("#e2e8f0")
	t.Border = ui.Hex("#e2e8f0")
	t.Text = ui.Hex("#0f172a")
	t.TextMuted = ui.Hex("#64748b")
	t.Accent = ui.Hex("#4f46e5")
	t.AccentHover = ui.Hex("#4338ca")
	t.AccentPressed = ui.Hex("#3730a3")
	t.AccentText = ui.Hex("#ffffff")
	t.InactiveAccent = ui.Hex("#94a3b8")
	t.Danger = ui.Hex("#e11d48")
	t.Warning = ui.Hex("#d97706")
	t.Success = ui.Hex("#059669")
	t.Selection = ui.Hex("#c7d2fe")
	t.Focus = ui.Hex("#6366f1")
	t.FocusRingWidth = 2
	t.FocusRingOffset = 1
	t.Radius = 8
	t.Spacing = 4
	return t
}

// DarkTheme returns a calibrated AsterLab dark theme.
func DarkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Dark = true
	t.Background = ui.Hex("#0b0f19")
	t.Surface = ui.Hex("#131b2e")
	t.SurfaceHover = ui.Hex("#1e293b")
	t.SurfacePressed = ui.Hex("#334155")
	t.Border = ui.Hex("#1e293b")
	t.Text = ui.Hex("#f8fafc")
	t.TextMuted = ui.Hex("#94a3b8")
	t.Accent = ui.Hex("#6366f1")
	t.AccentHover = ui.Hex("#818cf8")
	t.AccentPressed = ui.Hex("#4f46e5")
	t.AccentText = ui.Hex("#ffffff")
	t.InactiveAccent = ui.Hex("#475569")
	t.Danger = ui.Hex("#f43f5e")
	t.Warning = ui.Hex("#f59e0b")
	t.Success = ui.Hex("#10b981")
	t.Selection = ui.Hex("#312e81")
	t.Focus = ui.Hex("#818cf8")
	t.FocusRingWidth = 2
	t.FocusRingOffset = 1
	t.Radius = 8
	t.Spacing = 4
	return t
}
