package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesignTokens(t *testing.T) {
	light, dark := LightTokens(), DarkTokens()
	if light.Canvas != ui.Hex("#F8F9FA") || light.Surface != ui.Hex("#FFFFFF") || light.Accent != ui.Hex("#2563EB") {
		t.Fatal("Porcelain palette diverged from product spec")
	}
	if dark.Canvas != ui.Hex("#0B0C0E") || dark.Surface != ui.Hex("#111317") || dark.Accent != ui.Hex("#4B88FF") {
		t.Fatal("Graphite palette diverged from product spec")
	}
	if LightTheme().Dark || !DarkTheme().Dark || GetTokens(DarkTheme()).Danger != dark.Danger {
		t.Fatal("theme selection mismatch")
	}
}
