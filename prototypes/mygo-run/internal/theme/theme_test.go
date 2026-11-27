package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesignTokens(t *testing.T) {
	if PresetPaper != "paper" || normalizePreset("obsidian") != PresetPaper || normalizePreset("aurora") != PresetPaper {
		t.Fatalf("expected PresetPaper=='paper' and legacy migration from obsidian/aurora, got %q", normalizePreset("obsidian"))
	}
	SetActiveLook(PresetPaper, true)
	light, dark := LightTokens(), DarkTokens()
	if light.Canvas != ui.Hex("#ffffff") || light.Elevated != ui.Hex("#f4f4f5") {
		t.Fatalf("paper C1 light surfaces unexpected: canvas=%v elevated=%v", light.Canvas, light.Elevated)
	}
	if light.Accent != ui.Hex("#18181b") || light.AccentText != ui.Hex("#ffffff") || light.Text != ui.Hex("#18181b") || light.Warning != ui.Hex("#c87010") {
		t.Fatalf("paper C1 light ink/accent unexpected: accent=%v text=%v warn=%v", light.Accent, light.Text, light.Warning)
	}
	if dark.Canvas != ui.Hex("#111214") || dark.Accent != ui.Hex("#f4f4f5") || dark.AccentText != ui.Hex("#111214") || dark.Text != ui.Hex("#f4f4f5") || dark.Warning != ui.Hex("#f0a52b") {
		t.Fatalf("paper C1 dark palette unexpected: %+v", dark)
	}
	if light.AccentSoftBg != light.SelectedRow || dark.AccentSoftBg != dark.SelectedRow {
		t.Fatal("accent soft bg must match neutral SelectedRow in C1 monochrome theme")
	}
	if light.Accent == light.Success || light.Accent == light.Warning {
		t.Fatal("accent must stay distinct from semantic success/warning")
	}

	zinc := TokensFor(PresetZinc, false, true)
	if zinc.Canvas != ui.Hex("#f4f5f7") || zinc.Accent != ui.Hex("#1f2430") {
		t.Fatalf("zinc preset mismatch: %+v", zinc)
	}

	warmL := TokensFor(PresetWarm, false, false)
	warmD := TokensFor(PresetWarm, true, false)
	if warmL.Canvas != ui.Hex("#f7f6f3") || warmD.Canvas != ui.Hex("#171614") {
		t.Fatalf("warm preset compatibility broken: light=%v dark=%v", warmL.Canvas, warmD.Canvas)
	}

	lt, dt := LightTheme(), DarkTheme()
	if lt.Dark || !dt.Dark {
		t.Fatal("theme selection mismatch")
	}
	if lt.Background != light.Canvas || lt.Accent != light.Accent || lt.Radius != float32(light.ControlRadius) || lt.Spacing != 4 {
		t.Fatalf("onevra palette not layered with asterlab tokens: %+v", lt)
	}
	if GetTokens(dt).Danger != dark.Danger {
		t.Fatal("painter tokens do not follow the dark palette")
	}

	if !(light.TextPage >= light.TextSection && light.TextSection > light.TextBody && light.TextBody == 15 && light.TextData == 14 && light.TextMeta >= 13) {
		t.Fatalf("typography hierarchy mismatch with Obsidian-grade desktop 15px base: %+v", light)
	}
	if light.IconSm < 14 || light.IconMd < 16 || light.IconLg < 19 {
		t.Fatalf("icon scale tokens too small for 27-inch desktop readability: %+v", light)
	}
	SetAccessibilityPrefs(1.2, false)
	defer SetAccessibilityPrefs(0, false)
	scaled := LightTokens()
	if scaled.TextBody <= light.TextBody || scaled.TextMeta <= light.TextMeta || scaled.IconMd <= light.IconMd {
		t.Fatalf("accessibility font+icon scale did not apply cleanly: base=%+v scaled=%+v", light, scaled)
	}
	// Verify OS TextScale is respected when user scale is 0 (follow OS) without double-scaling.
	SetAccessibilityPrefs(0, false)
	SetSystemTextScale(1.25)
	defer SetSystemTextScale(1.0)
	osScaled := LightTokens()
	osTheme := LightTheme()
	if osScaled.TextBody <= light.TextBody || osTheme.FontSize != float32(osScaled.TextBody) {
		t.Fatalf("OS TextScale must drive both Tokens and Onevra FontSize without double-scaling: tok=%d theme=%.1f", osScaled.TextBody, osTheme.FontSize)
	}

	SetSystemTextScale(1.0)
	SetActiveDensity(DensityCompact)
	defer SetActiveDensity(DensityDefault)
	comp := LightTokens()
	if comp.Density != DensityCompact || comp.RowHeight >= light.RowHeight || comp.Spacing16 >= light.Spacing16 {
		t.Fatalf("compact density should tighten row height & spacing: comp=(%d,%d) def=(%d,%d)", comp.RowHeight, comp.Spacing16, light.RowHeight, light.Spacing16)
	}
	SetActiveDensity(DensityComfortable)
	comf := LightTokens()
	if comf.Density != DensityComfortable || comf.RowHeight <= light.RowHeight || comf.Spacing16 <= light.Spacing16 {
		t.Fatalf("comfortable density should expand row height & spacing: comf=(%d,%d) def=(%d,%d)", comf.RowHeight, comf.Spacing16, light.RowHeight, light.Spacing16)
	}
	if dark.GhostHover.A == 0 || dark.TopHighlight.A == 0 || dark.BorderFocused != dark.Accent {
		t.Fatalf("expected Zed-inspired interaction state & top highlight tokens in dark mode: %+v", dark)
	}
	if light.IconSm != 16 || light.IconMd != 18 || light.IconLg != 20 || light.IconBoxSize != 24 || light.PillRadius != 999 {
		t.Fatalf("optical icon/box/pill tokens mismatch: %+v", light)
	}

	assert4pxGrid := func(label string, tk Tokens) {
		for name, val := range map[string]int{
			"Spacing4":    tk.Spacing4,
			"Spacing8":    tk.Spacing8,
			"Spacing12":   tk.Spacing12,
			"Spacing16":   tk.Spacing16,
			"Spacing24":   tk.Spacing24,
			"Spacing32":   tk.Spacing32,
			"Spacing48":   tk.Spacing48,
			"IconBoxSize": tk.IconBoxSize,
		} {
			if val <= 0 || val%4 != 0 {
				t.Fatalf("%s %s=%d violates strict 4px grid", label, name, val)
			}
		}
	}
	for _, sc := range []float32{0, 1.25} {
		SetAccessibilityPrefs(sc, false)
		for _, dn := range []string{DensityCompact, DensityDefault, DensityComfortable} {
			SetActiveDensity(dn)
			assert4pxGrid("scale="+dn, LightTokens())
		}
	}
}

func TestConcentricRadius(t *testing.T) {
	if got := ConcentricRadius(0, 4); got != 0 {
		t.Fatalf("ConcentricRadius(0, 4) = %v, want 0", got)
	}
	if got := ConcentricRadius(-6, 4); got != 0 {
		t.Fatalf("ConcentricRadius(-6, 4) = %v, want 0", got)
	}
	if got := ConcentricRadius(12, 4); got != 8 {
		t.Fatalf("ConcentricRadius(12, 4) = %v, want 8", got)
	}
	if got := ConcentricRadius(6, 4); got != 2 {
		t.Fatalf("ConcentricRadius(6, 4) = %v, want 2", got)
	}
	if got := ConcentricRadius(6, 12); got != 2 {
		t.Fatalf("ConcentricRadius(6, 12) = %v, want 2 (clamped floor)", got)
	}

	tok := LightTokens()
	if got := tok.InnerRadius(tok.PanelRadius, tok.Spacing4); got != 8 {
		t.Fatalf("InnerRadius(12, 4) = %v, want 8", got)
	}
	if got := tok.InnerControlRadius(tok.Spacing4); got != 2 {
		t.Fatalf("InnerControlRadius(4) = %v, want 2", got)
	}
	if got := tok.InnerPanelRadius(tok.Spacing4); got != 8 {
		t.Fatalf("InnerPanelRadius(4) = %v, want 8", got)
	}
	if got := tok.InnerDialogRadius(tok.Spacing8); got != 6 {
		t.Fatalf("InnerDialogRadius(8) = %v, want 6", got)
	}
}

func TestDarkroomAndSpecularTokens(t *testing.T) {
	SetActiveLook(PresetPaper, true)
	light, dark := LightTokens(), DarkTokens()

	for name, tk := range map[string]Tokens{"light": light, "dark": dark} {
		if tk.DarkroomCanvas != ui.Hex("#0b0c0e") ||
			tk.DarkroomSurface != ui.Hex("#141518") ||
			tk.DarkroomElevated != ui.Hex("#1d1f24") ||
			tk.DarkroomText != ui.Hex("#f4f4f5") ||
			tk.DarkroomMuted != ui.Hex("#9a9ca6") {
			t.Fatalf("%s darkroom solid colors mismatch: %+v", name, tk)
		}
		if tk.DarkroomBorder != ui.RGBA(255, 255, 255, 0.10) {
			t.Fatalf("%s DarkroomBorder mismatch: %+v", name, tk.DarkroomBorder)
		}
		if tk.DarkroomBadgeBg != ui.RGBA(11, 12, 14, 0.72) {
			t.Fatalf("%s DarkroomBadgeBg mismatch: %+v", name, tk.DarkroomBadgeBg)
		}
		if tk.HairlineDivider.A == 0 {
			t.Fatalf("%s HairlineDivider must be non-transparent", name)
		}
	}

	if dark.SpecularRim != ui.RGBA(255, 255, 255, 0.08) {
		t.Fatalf("dark SpecularRim mismatch: %+v", dark.SpecularRim)
	}
	if light.SpecularRim != ui.RGBA(255, 255, 255, 0.65) {
		t.Fatalf("light SpecularRim mismatch: %+v", light.SpecularRim)
	}
}

func TestTopinspoOpticalCraftTokensAcrossPresets(t *testing.T) {
	for _, preset := range []string{PresetPaper, PresetZinc, PresetWarm} {
		lt := TokensFor(preset, false, true)
		dk := TokensFor(preset, true, true)

		if dk.SpecularRimTop.A <= dk.SpecularRimBottom.A {
			t.Fatalf("preset=%s dark: expected SpecularRimTop.A (%d) > SpecularRimBottom.A (%d)",
				preset, dk.SpecularRimTop.A, dk.SpecularRimBottom.A)
		}
		if dk.SpecularRimTop.A <= dk.SpecularRim.A {
			t.Fatalf("preset=%s dark: expected SpecularRimTop.A (%d) > SpecularRim.A (%d)",
				preset, dk.SpecularRimTop.A, dk.SpecularRim.A)
		}
		if dk.SpecularRimBottom.A == 0 {
			t.Fatalf("preset=%s dark: expected SpecularRimBottom.A > 0", preset)
		}
		if lt.InsetShadow.A == 0 || dk.InsetShadow.A == 0 {
			t.Fatalf("preset=%s: expected InsetShadow.A > 0 in light (%d) and dark (%d)",
				preset, lt.InsetShadow.A, dk.InsetShadow.A)
		}
		if dk.InsetShadow.A <= lt.InsetShadow.A {
			t.Fatalf("preset=%s: expected dark InsetShadow.A (%d) > light InsetShadow.A (%d)",
				preset, dk.InsetShadow.A, lt.InsetShadow.A)
		}
		if lt.MaskSolid.A != 255 || dk.MaskSolid.A != 255 {
			t.Fatalf("preset=%s: expected MaskSolid.A == 255 in light (%d) and dark (%d)",
				preset, lt.MaskSolid.A, dk.MaskSolid.A)
		}
	}
}
