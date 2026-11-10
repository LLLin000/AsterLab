package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesignTokens(t *testing.T) {
	light, dark := LightTokens(), DarkTokens()
	// DESIGN.md: warm neutral canvas, one violet accent reserved for selection
	// and primary action, black body text above gray labels.
	if light.Canvas != ui.Hex("#f7f6f3") || light.Surface != ui.Hex("#ffffff") {
		t.Fatalf("light surfaces diverged from DESIGN.md: canvas=%v surface=%v", light.Canvas, light.Surface)
	}
	if light.Accent != ui.Hex("#5b45c9") || light.Text != ui.Hex("#1c1b18") || light.TextMuted != ui.Hex("#5f5a52") {
		t.Fatalf("light ink/accent diverged from DESIGN.md: accent=%v text=%v muted=%v", light.Accent, light.Text, light.TextMuted)
	}
	if dark.Canvas != ui.Hex("#171614") || dark.Accent != ui.Hex("#a996ff") ||
		dark.Text != ui.Hex("#f2efe9") || dark.TextMuted != ui.Hex("#a9a49a") {
		t.Fatalf("dark palette diverged from DESIGN.md: %+v", dark)
	}
	if light.Accent == light.Success || light.Accent == light.Warning {
		t.Fatal("accent must stay distinct from semantic success/warning")
	}
	lt, dt := LightTheme(), DarkTheme()
	if lt.Dark || !dt.Dark {
		t.Fatal("theme selection mismatch")
	}
	if lt.Background != light.Canvas || lt.Accent != light.Accent || lt.Radius != 8 || lt.Spacing != 4 {
		t.Fatalf("onevra palette not layered with asterlab tokens: %+v", lt)
	}
	if GetTokens(dt).Danger != dark.Danger {
		t.Fatal("painter tokens do not follow the dark palette")
	}
}
