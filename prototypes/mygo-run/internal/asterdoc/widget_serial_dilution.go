package asterdoc

import (
	"fmt"
	"math"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// renderSerialDilutionTubeSchematic renders a horizontal row of 1.5 mL microcentrifuge tubes
// with overhead pipetting transfer arrows and native scientific-notation concentrations
// (UI-04 & UI-05), replacing bloated multi-row text lists.
func (v *View) renderSerialDilutionTubeSchematic(c *ui.Context, tok theme.Tokens, moduleID string, data map[string]any, res CalcResult) {
	if res.Err != nil {
		ui.Text(c, friendlyCalcError(res.Err)).FontSize(float32(tok.TextMeta)).TextColor(tok.Warning)
		return
	}
	df, ok1 := numericValue(data["dilution_factor"])
	stepsF, ok2 := numericValue(data["steps_count"])
	if !ok1 || df <= 1 || !ok2 || stepsF < 1 {
		return
	}
	steps := int(stepsF)
	if steps > 12 {
		steps = 12
	}

	// Interpret volume_per_unit cleanly:
	// - When >= 0.5 mL (e.g. 1 mL standard final tube volume in 1.5 mL tube), transfer = finalVol / df and diluent = finalVol - transfer.
	// - When small transfer volume (e.g. 100 uL), diluent = transfer * (df - 1).
	vpuVal, vpuUnit, qerr := quantityParts(data["volume_per_unit"])
	if qerr != nil || vpuVal <= 0 {
		vpuVal, vpuUnit = 1, "mL"
	}
	vpuUL := vpuVal
	if vpuUnit == "mL" {
		vpuUL = vpuVal * 1000
	} else if vpuUnit == "L" {
		vpuUL = vpuVal * 1e6
	}
	var transferUL, diluentUL float64
	if vpuUnit == "mL" || vpuUL >= 500 {
		transferUL = vpuUL / df
		diluentUL = vpuUL - transferUL
	} else {
		transferUL = vpuUL
		diluentUL = vpuUL * (df - 1)
	}
	transferLabel := formatBenchVolUL(transferUL)
	diluentLabel := formatBenchVolUL(diluentUL)

	stockVal, stockUnit, stockErr := quantityParts(data["stock"])
	hasStock := stockErr == nil && stockVal > 0

	// 1. Concise bench instruction strip
	ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border.Alpha(0.75)).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Icon(c, lucide.FlaskConical()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
		summary := fmt.Sprintf("1.5 mL 离心管 × %d · 每管预分装 %s 稀释液 · 逐级移取 %s 混匀转移", steps, diluentLabel, transferLabel)
		ui.Text(c, summary).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum")
	})

	// 2. Horizontal tube rack with overhead transfer arrows
	ui.ScrollHorizontal(c.Key("serial-rack-" + moduleID)).FillWidth().Children(func() {
		ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing4)).AlignItems(ui.End).Gap(0).Children(func() {
			if hasStock {
				renderMicroTubeNode(c, tok, "原液", "Stock", stockVal, stockUnit, 0.78, true)
				renderTransferArrowBridge(c, tok, transferLabel)
			}
			factor := 1.0
			for i := 1; i <= steps; i++ {
				factor *= df
				// Dilution factor label: e.g. 10^-1 .. 10^-6 for 10x, or 1:2^i / 1:N
				var dilExpr string
				if math.Abs(df-10) < 1e-6 {
					dilExpr = fmt.Sprintf("10^{-%d}", i)
				} else {
					dilExpr = "1:" + formatSciCompactText(factor)
				}
				concVal := 0.0
				if hasStock {
					concVal = stockVal / factor
				}
				// Liquid opacity smoothly steps down from 0.62 -> 0.14 across the gradient
				alpha := float32(0.62) - float32(i-1)*float32(0.48)/float32(maxIntSD(steps-1, 1))
				if alpha < 0.14 {
					alpha = 0.14
				}
				renderMicroTubeNode(c, tok, fmt.Sprintf("T%d", i), dilExpr, concVal, stockUnit, alpha, false)
				if i < steps {
					renderTransferArrowBridge(c, tok, transferLabel)
				}
			}
		})
	})
}

func renderTransferArrowBridge(c *ui.Context, tok theme.Tokens, transferLabel string) {
	ui.Column(c).Width(float32(tok.Spacing48)+float32(tok.Spacing12)).AlignItems(ui.Center).
		Padding(0, 0, float32(tok.Spacing48)+float32(tok.Spacing16), 0).Gap(2).Children(func() {
		ui.Row(c).Padding(1, float32(tok.Spacing4)).Radius(float32(tok.ControlRadius)).
			Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.30)).Children(func() {
			ui.Text(c, transferLabel).FontSize(float32(tok.TextMeta)-1).Bold().
				TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
		})
		// Curved-style pipetting arrow bridge: horizontal bar + arrowhead
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, float32(tok.Spacing4)).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Height(2).Background(tok.Accent.Alpha(0.75))
			ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
		})
	})
}

func renderMicroTubeNode(c *ui.Context, tok theme.Tokens, code, dilExpr string, concVal float64, concUnit string, liquidAlpha float32, isStock bool) {
	borderCol := tok.BorderStrong
	if isStock {
		borderCol = tok.Accent
	}
	ui.Column(c).Width(float32(tok.Spacing48)*1.75).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
		// Tube dilution header with native raised exponent (e.g. 10^-1)
		if isStock {
			ui.Text(c, code).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).SingleLine()
		} else {
			renderNativeScriptRow(c, tok, dilExpr, float32(tok.TextMeta), tok.Text, false, true)
		}

		// Visual 1.5 mL microcentrifuge tube silhouette (cap + rim + body with liquid fill + conical tip)
		ui.Column(c).AlignItems(ui.Center).Gap(0).Children(func() {
			// Flip cap
			ui.Box(c).Width(34).Height(5).Radius(2).Background(borderCol.Alpha(0.65))
			// Tube collar rim
			ui.Box(c).Width(38).Height(4).Radius(1).Background(tok.Surface).Border(1.5, borderCol)
			// Cylindrical upper body with liquid column
			ui.Column(c).Width(30).Height(42).Background(tok.Surface).Border(1.5, borderCol).
				Justify(ui.End).Clip().Children(func() {
				ui.Box(c).FillWidth().Height(26).Background(tok.Accent.Alpha(liquidAlpha))
			})
			// Conical lower tip (tapered 1.5 mL Eppendorf profile)
			ui.Box(c).Width(20).Height(10).Radius(4).Background(tok.Accent.Alpha(liquidAlpha)).
				Border(1.5, borderCol)
			ui.Box(c).Width(10).Height(6).Radius(4).Background(tok.Accent.Alpha(liquidAlpha)).
				Border(1, borderCol)
		})

		// Tube index & concentration readout (scientific notation >= 1000 per UI-05)
		if isStock {
			ui.Text(c, "1.5 mL").FontSize(float32(tok.TextMeta)-1).TextColor(tok.TextMuted).SingleLine()
		} else {
			ui.Text(c, code+" · 1.5 mL").FontSize(float32(tok.TextMeta)-1).TextColor(tok.TextMuted).SingleLine()
		}
		if concVal > 0 {
			renderSciNumber(c, tok, concVal, "", float32(tok.TextMeta)-1, tok.Accent, true)
			if concUnit != "" {
				ui.Text(c, DisplayUnit(concUnit)).FontSize(float32(tok.TextMeta)-2).TextColor(tok.TextMuted).SingleLine()
			}
		}
	})
}

func formatBenchVolUL(ul float64) string {
	if ul >= 1000 {
		return fmt.Sprintf("%s mL", formatNumber(round4(ul/1000)))
	}
	return fmt.Sprintf("%s µL", formatNumber(round4(ul)))
}

func maxIntSD(a, b int) int {
	if a > b {
		return a
	}
	return b
}
