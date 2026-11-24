package tools

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
)

func TestVivariumAgeDensityAndSaveRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	if got := VivariumAgeLabel("2026-10-06", now); got != "7.0 周龄" {
		t.Fatalf("expected 7.0 周龄, got %q", got)
	}
	if got := VivariumAgeLabel("2026-11-10", now); got != "14 日龄" {
		t.Fatalf("expected 14 日龄, got %q", got)
	}

	warn := VivariumDensityWarning(VivariumCage{
		Species:     labsticker.AnimalMouseBlack,
		MaleCount:   6,
		FemaleCount: 0,
		Status:      "active",
	})
	if warn == "" {
		t.Fatal("expected GB 14925 density warning for 6 mice in single cage")
	}

	var saved []VivariumCage
	w := New(Callbacks{
		SaveVivarium: func(cages []VivariumCage) error {
			saved = append([]VivariumCage(nil), cages...)
			return nil
		},
	})
	w.OpenNewVivariumDialog()
	w.vivariumDraft.CageCode = "C-08"
	w.vivariumDraft.Strain = "C57BL/6J"
	w.vivariumDraft.IACUCNo = "IACUC-2026-088"
	w.vivariumMaleF = 5
	w.vivariumWeightF = 21.4
	if !w.SaveVivariumDraft() {
		t.Fatalf("expected SaveVivariumDraft to succeed, err=%q", w.vivariumDialogErr)
	}
	if len(saved) != 1 || saved[0].CageCode != "C-08" || saved[0].MaleCount != 5 {
		t.Fatalf("unexpected saved vivarium slice: %+v", saved)
	}

	// Render smoke test inside Vivarium tab
	w.pantryDomainTab = 1
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
			renderVivariumView(c, w, tok)
		})
	}, 980, 680)
	tt.Frame()
	if !tt.HasText("C-08") || !tt.HasText("C57BL/6J") || !tt.HasText("IACUC-2026-088") {
		t.Fatalf("expected vivarium cage card content in UI, texts=%v", tt.Texts())
	}
}
