package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesignTokens(t *testing.T) {
	light, dark := LightTokens(), DarkTokens()
	if light.Canvas != ui.Hex("#F6F4EF") || light.Surface != ui.Hex("#FBFAF7") || light.Accent != ui.Hex("#1F6F5F") {
		t.Fatal("light palette diverged from product spec")
	}
	if dark.Canvas != ui.Hex("#171614") || dark.Surface != ui.Hex("#201E1A") || dark.Accent != ui.Hex("#4FA38D") {
		t.Fatal("dark palette diverged from product spec")
	}
	if LightTheme().Dark || !DarkTheme().Dark || GetTokens(DarkTheme()).Danger != dark.Danger {
		t.Fatal("theme selection mismatch")
	}
}
