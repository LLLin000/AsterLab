package asterdoc

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

func TestDisplayUnitFormatting(t *testing.T) {
	cases := map[string]string{
		"Cel":   "°C",
		"uL":    "μL",
		"um":    "μm",
		"uM":    "μM",
		"ng/uL": "ng/μL",
		"xg":    "×g",
		"1/min": "min⁻¹",
		"min":   "min",
	}
	for in, want := range cases {
		if got := DisplayUnit(in); got != want {
			t.Fatalf("DisplayUnit(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestStripDuplicateLeadingH1(t *testing.T) {
	body := "\n# L929 药物梯度测定\n\n正文内容\n"
	got := stripDuplicateLeadingH1(body, "L929 药物梯度测定")
	if got != "\n\n正文内容\n" {
		t.Fatalf("expected duplicate leading H1 stripped, got %q", got)
	}
	other := "\n# 完全不同的章节标题\n\n正文内容\n"
	if kept := stripDuplicateLeadingH1(other, "L929 药物梯度测定"); kept != other {
		t.Fatalf("expected non-matching H1 preserved, got %q", kept)
	}
}

func TestStripStandaloneSlashLine(t *testing.T) {
	if rem, ok := stripStandaloneSlashLine("/"); !ok || rem != "" {
		t.Fatalf("expected single '/' to trigger, got ok=%v rem=%q", ok, rem)
	}
	if rem, ok := stripStandaloneSlashLine("第一行说明\n/"); !ok || rem != "第一行说明" {
		t.Fatalf("expected trailing '/'-only line in multiline prose to trigger, got ok=%v rem=%q", ok, rem)
	}
	if _, ok := stripStandaloneSlashLine("100 uL / 孔"); ok {
		t.Fatalf("expected inline slash not to trigger slash menu")
	}
}

func TestPlateNumericFactorGradientLegendAndRadiusAlignment(t *testing.T) {
	if th := theme.LightTheme(); th.Radius != float32(theme.LightTokens().ControlRadius) {
		t.Fatalf("expected theme Radius %.0f to match ControlRadius %d", th.Radius, theme.LightTokens().ControlRadius)
	}
	props := plateui.Props{
		Rows:     8,
		Cols:     12,
		ColorBy:  "factor",
		FactorKey: "浓度梯度",
		Wells: map[string]plateui.WellState{
			"A1": {Factors: map[string]string{"浓度梯度": "12.5 uM"}},
			"A2": {Factors: map[string]string{"浓度梯度": "100 uM"}},
			"A3": {Factors: map[string]string{"浓度梯度": "3.125 uM"}},
			"A4": {Factors: map[string]string{"浓度梯度": "DMSO"}},
		},
	}
	legend := plateui.ComputeLegend(props, false)
	if len(legend) != 4 {
		t.Fatalf("expected 4 legend entries, got %d", len(legend))
	}
	if legend[0].Label != "100 uM" || legend[1].Label != "12.5 uM" || legend[2].Label != "3.125 uM" || legend[3].Label != "DMSO" {
		t.Fatalf("expected numeric legend sorted high-to-low with DMSO last, got %+v", legend)
	}
	// Monotonic darkness: 100 uM should be darker (lower luminance) than 3.125 uM in light mode.
	lum := func(i int) float64 {
		c := legend[i].Color
		return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	}
	if lum(0) >= lum(2) {
		t.Fatalf("expected 100 uM color (%+v) darker than 3.125 uM (%+v) in light mode", legend[0].Color, legend[2].Color)
	}
}
