package asterdoc

import (
	"fmt"
	"math"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// serialDilutionVesselSpec chooses the appropriate wet-lab centrifuge tube sticker
// and vessel label based on the final solution volume per tube (UI-05).
func serialDilutionVesselSpec(finalVolUL float64) (labsticker.VesselID, string) {
	switch {
	case finalVolUL > 15000:
		return labsticker.VesselFalcon50, "50 mL 离心管"
	case finalVolUL > 10000:
		return labsticker.VesselFalcon15, "15 mL 离心管"
	case finalVolUL > 5000:
		return labsticker.VesselFalcon15, "10/15 mL 离心管"
	case finalVolUL > 1500:
		return labsticker.VesselFalcon15, "5/15 mL 离心管"
	default:
		return labsticker.VesselEP15, "1.5 mL EP 管"
	}
}

// renderSerialDilutionTubeSchematic renders a horizontal row of wet-lab centrifuge tubes
// reusing Pantry's labsticker artwork with overhead pipetting transfer arrows and native
// scientific-notation concentrations (UI-04 & UI-05).
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

	vpuVal, vpuUnit, qerr := quantityParts(data["volume_per_unit"])
	if qerr != nil || vpuVal <= 0 {
		vpuVal, vpuUnit = 1, "mL"
	}
	vpuUL := vpuVal
	switch vpuUnit {
	case "mL":
		vpuUL = vpuVal * 1000
	case "L":
		vpuUL = vpuVal * 1e6
	}
	var transferUL, diluentUL, finalVolUL float64
	if vpuUnit == "mL" || vpuUnit == "L" || vpuUL >= 500 {
		finalVolUL = vpuUL
		transferUL = vpuUL / df
		diluentUL = vpuUL - transferUL
	} else {
		transferUL = vpuUL
		diluentUL = vpuUL * (df - 1)
		finalVolUL = transferUL + diluentUL
	}
	vesselID, vesselLabel := serialDilutionVesselSpec(finalVolUL)
	transferLabel := formatBenchVolUL(transferUL)
	diluentLabel := formatBenchVolUL(diluentUL)

	stockVal, stockUnit, stockErr := quantityParts(data["stock"])
	hasStock := stockErr == nil && stockVal > 0

	// 1. Concise bench instruction strip
	ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border.Alpha(0.75)).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Icon(c, lucide.FlaskConical()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
		summary := fmt.Sprintf("%s × %d · 每管预分装 %s 稀释液 · 逐级移取 %s 混匀转移", vesselLabel, steps, diluentLabel, transferLabel)
		ui.Text(c, summary).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum")
	})

	// 2. Horizontal tube rack reusing labsticker vessel art + overhead transfer arrows
	ui.ScrollHorizontal(c.Key("serial-rack-" + moduleID)).FillWidth().Children(func() {
		ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing4)).AlignItems(ui.End).Gap(0).Children(func() {
			if hasStock {
				renderMicroTubeNode(c, tok, vesselID, vesselLabel, "原液", "Stock", stockVal, stockUnit, true)
				renderTransferArrowBridge(c, tok, transferLabel)
			}
			factor := 1.0
			for i := 1; i <= steps; i++ {
				factor *= df
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
				renderMicroTubeNode(c, tok, vesselID, vesselLabel, fmt.Sprintf("T%d", i), dilExpr, concVal, stockUnit, false)
				if i < steps {
					renderTransferArrowBridge(c, tok, transferLabel)
				}
			}
		})
	})
}

func renderTransferArrowBridge(c *ui.Context, tok theme.Tokens, transferLabel string) {
	ui.Column(c).Width(float32(tok.Spacing48)).AlignItems(ui.Center).
		Padding(0, 0, float32(tok.Spacing48)+float32(tok.Spacing12), 0).Gap(2).Children(func() {
		ui.Row(c).Padding(1, float32(tok.Spacing4)).Radius(float32(tok.ControlRadius)).
			Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.30)).Children(func() {
			ui.Text(c, transferLabel).FontSize(float32(tok.TextMeta)-1).Bold().
				TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 2).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Height(2).Background(tok.Accent.Alpha(0.75))
			ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
		})
	})
}

func renderMicroTubeNode(c *ui.Context, tok theme.Tokens, vesselID labsticker.VesselID, vesselLabel, code, dilExpr string, concVal float64, concUnit string, isStock bool) {
	ui.Column(c).Width(float32(tok.Spacing48)*1.55).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
		// Tube dilution header with native raised exponent (e.g. 10^-1)
		if isStock {
			ui.Text(c, code).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).SingleLine()
		} else {
			renderNativeScriptRow(c, tok, dilExpr, float32(tok.TextMeta), tok.Text, false, true)
		}

		// Reuse Pantry's authentic labsticker vessel illustration (UI-05)
		labsticker.RenderSticker(c, vesselID, nil, labsticker.StickerOptions{
			TargetHeightDIP: 56,
			BorderDIP:       2.5,
		})

		// Tube index & concentration readout (scientific notation >= 1000 per UI-05)
		if isStock {
			ui.Text(c, vesselLabel).FontSize(float32(tok.TextMeta)-1).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
		} else {
			ui.Text(c, code).FontSize(float32(tok.TextMeta)-1).Bold().TextColor(tok.TextMuted).SingleLine()
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
