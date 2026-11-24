package asterdoc

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
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

// renderSerialDilutionTubeSchematic renders a clean serial-dilution schematic supporting
// BOTH centrifuge tube strips AND 96-well multichannel pipetting strips (`vessel: "tube" | "plate96"`).
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
	vesselMode := ""
	if mod := findModuleByID(v.Doc, moduleID); mod != nil {
		vesselMode = strings.ToLower(strings.TrimSpace(moduleDirectiveValue(mod.Body, "vessel")))
	}
	if vesselMode == "" {
		vesselMode = strings.ToLower(strings.TrimSpace(textValue(data, "vessel")))
	}
	if vesselMode == "" && vpuUL > 0 && vpuUL <= 250 {
		vesselMode = "plate96"
	}
	if vesselMode != "plate96" {
		vesselMode = "tube"
	}

	var transferUL, diluentUL, finalVolUL float64
	if vpuUnit == "mL" || vpuUnit == "L" || vpuUL >= 100 {
		finalVolUL = vpuUL
		transferUL = vpuUL / df
		diluentUL = vpuUL - transferUL
	} else {
		transferUL = vpuUL
		diluentUL = vpuUL * (df - 1)
		finalVolUL = transferUL + diluentUL
	}
	vesselID, vesselLabel := serialDilutionVesselSpec(finalVolUL)
	if vesselMode == "plate96" {
		vesselLabel = "96 孔板排枪"
	}
	transferLabel := formatBenchVolUL(transferUL)
	diluentLabel := formatBenchVolUL(diluentUL)

	stockVal, stockUnit, stockErr := quantityParts(data["stock"])
	hasStock := stockErr == nil && stockVal > 0
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && !v.LabMode && v.Can(PermDocumentEdit)

	// 1. Clean single-line bench cue + subtle vessel badge + right-aligned 参数 Popover trigger
	// (vessel switcher lives cleanly inside 参数 Popover — zero redundant toggle bar clutter!).
	cfgOpen := ui.Local(c.Root(), "sd-cfg:"+moduleID, func() bool { return false })
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		vesselModeName := "离心管"
		if vesselMode == "plate96" {
			vesselModeName = "96 孔板排枪"
		}
		if editableDesign {
			modeChip := ui.ButtonBase(c.Key("sd-mode-chip:"+moduleID)).
				Height(float32(tok.Spacing24)-2).Padding(0, float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
				AlignItems(ui.Center).Label("切换稀释容器")
			theme.ApplyTactilePress(modeChip)
			modeChip.Children(func() {
				ui.Text(c, vesselModeName).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).SingleLine()
			})
			if modeChip.Clicked() {
				*cfgOpen = !*cfgOpen
			}
		} else {
			components.Badge(c, vesselModeName)
		}

		benchCue := fmt.Sprintf("预加 %s 稀释液 · 逐级移取 %s 混匀转移", diluentLabel, transferLabel)
		if vesselMode == "plate96" {
			benchCue = fmt.Sprintf("每孔预加 %s 稀释液 · 排枪 %s 混匀逐列转移（末列弃 %s）", diluentLabel, transferLabel, transferLabel)
		}
		ui.Text(c, benchCue).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
		ui.Spacer(c)

		if editableDesign {
			paramBtn := ui.ButtonBase(c.Key("sd-param-btn:"+moduleID)).
				Height(float32(tok.Spacing24)).Padding(0, float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).AlignItems(ui.Center).Label("调整稀释参数")
			theme.ApplyTactilePress(paramBtn)
			hoverBg := ui.Transparent
			if *cfgOpen || paramBtn.Hovered() {
				hoverBg = tok.Text.Alpha(0.05)
			}
			paramBtn.Background(paramBtn.AnimateColor("sd-pbg", hoverBg, theme.MotionMicroFade))
			paramBtn.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Icon(c, lucide.SlidersHorizontal()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
					ui.Text(c, "参数").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				})
			})
			if paramBtn.Clicked() {
				*cfgOpen = !*cfgOpen
			}
			components.Popover(c.Key("sd-pop:"+moduleID), paramBtn, cfgOpen, func(panel ui.Element) {
				panel.Width(float32(tok.Spacing48)*6.0).Padding(float32(tok.Spacing12)).
					Background(tok.Surface).Border(1, tok.Border).Radius(float32(tok.PanelRadius)).
					Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "稀释参数").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
					dfDraft := v.ensureWidgetDraft("sd:"+moduleID+":df", formatNumber(df))
					stDraft := v.ensureWidgetDraft("sd:"+moduleID+":steps", strconv.Itoa(steps))
					vDraft := v.ensureWidgetDraft("sd:"+moduleID+":vpu", formatNumber(finalVolUL))

					ui.Row(c).FillWidth().Padding(2).Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
						for _, vm := range []struct {
							id    string
							label string
						}{
							{"tube", "离心管"},
							{"plate96", "96 孔板排枪"},
						} {
							vm := vm
							active := vesselMode == vm.id
							vBtn := ui.ButtonBase(c.Key("sd-ves:"+moduleID+":"+vm.id)).
								Grow(1).Height(float32(tok.Spacing24)-2).Radius(float32(tok.ControlRadius)-1).
								Center().Label(vm.label)
							theme.ApplyTactilePress(vBtn)
							targetBg, targetFg := ui.Transparent, tok.TextMuted
							if active {
								targetBg, targetFg = tok.Surface, tok.Accent
							}
							vBtn.Background(vBtn.AnimateColor("sdv-bg", targetBg, theme.MotionMicroFade))
							vBtn.Children(func() {
								ui.Text(c, vm.label).FontSize(float32(tok.TextMeta)).Bold().TextColor(targetFg).SingleLine()
							})
							if vBtn.Clicked() && !active {
								patch := map[string]any{}
								if vm.id == "plate96" {
									*vDraft = "200"
									if finalVolUL >= 500 {
										patch["volume_per_unit"] = map[string]any{"value": 200, "unit": "uL"}
									}
								} else {
									*vDraft = "1000"
									if finalVolUL <= 250 {
										patch["volume_per_unit"] = map[string]any{"value": 1, "unit": "mL"}
									}
								}
								v.emitModuleDirectiveEdit(moduleID, "vessel", vm.id, patch)
							}
						}
					})

					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "倍比 D").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Width(float32(tok.Spacing48)*1.5)
						ui.Box(c).Grow(1).Children(func() {
							components.TextBox(c, dfDraft, components.TextBoxOptions{}).Placeholder("10")
						})
					})
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "级数 (列/管)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Width(float32(tok.Spacing48)*1.5)
						ui.Box(c).Grow(1).Children(func() {
							components.TextBox(c, stDraft, components.TextBoxOptions{}).Placeholder("6")
						})
					})
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "终体积 (µL)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Width(float32(tok.Spacing48)*1.5)
						ui.Box(c).Grow(1).Children(func() {
							components.TextBox(c, vDraft, components.TextBoxOptions{}).Placeholder("200")
						})
					})
					ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).Children(func() {
						if components.Button(c, "96孔板 180+20 µL", components.ButtonGhost).Clicked() {
							*dfDraft = "10"
							*vDraft = "200"
							v.emitModuleDirectiveEdit(moduleID, "vessel", "plate96", map[string]any{
								"dilution_factor": 10,
								"volume_per_unit": map[string]any{"value": 200, "unit": "uL"},
							})
							*cfgOpen = false
						}
						if components.Button(c, "EP管 900+100 µL", components.ButtonGhost).Clicked() {
							*dfDraft = "10"
							*vDraft = "1000"
							v.emitModuleDirectiveEdit(moduleID, "vessel", "tube", map[string]any{
								"dilution_factor": 10,
								"volume_per_unit": map[string]any{"value": 1, "unit": "mL"},
							})
							*cfgOpen = false
						}
					})
					ui.Row(c).FillWidth().Justify(ui.End).Gap(float32(tok.Spacing8)).Children(func() {
						if components.Button(c, "应用", components.ButtonPrimary).Clicked() {
							patch := map[string]any{}
							if ndf, err := strconv.ParseFloat(strings.TrimSpace(*dfDraft), 64); err == nil && ndf > 1 {
								patch["dilution_factor"] = ndf
							}
							if nst, err := strconv.Atoi(strings.TrimSpace(*stDraft)); err == nil && nst >= 1 && nst <= 12 {
								patch["steps_count"] = nst
							}
							if nvol, err := strconv.ParseFloat(strings.TrimSpace(*vDraft), 64); err == nil && nvol > 0 {
								patch["volume_per_unit"] = map[string]any{"value": nvol, "unit": "uL"}
							}
							if len(patch) > 0 {
								v.emitAnyModuleEdit(moduleID, patch)
							}
							*cfgOpen = false
						}
					})
				})
			})
		}
	})

	if vesselMode == "plate96" {
		renderSerialDilutionPlate96Strip(c, tok, moduleID, df, steps, diluentLabel, transferLabel, hasStock, stockVal, stockUnit)
		return
	}

	// 2a. Horizontal tube rack reusing labsticker vessel art + overhead transfer arrows
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

func renderSerialDilutionPlate96Strip(c *ui.Context, tok theme.Tokens, moduleID string, df float64, steps int, diluentLabel, transferLabel string, hasStock bool, stockVal float64, stockUnit string) {
	ui.ScrollHorizontal(c.Key("serial-p96-" + moduleID)).FillWidth().Children(func() {
		ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
			Radius(float32(tok.ControlRadius)).Background(tok.Elevated.Alpha(0.65)).Border(1, tok.Border.Alpha(0.65)).
			AlignItems(ui.End).Gap(0).Children(func() {
			renderWellCol := func(colLabel, dilExpr string, colIndex int, isStock bool, concVal float64) {
				ui.Column(c).Width(float32(tok.Spacing48)*1.32).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					if isStock {
						ui.Text(c, "原液列").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).SingleLine()
					} else {
						renderNativeScriptRow(c, tok, dilExpr, float32(tok.TextMeta), tok.Text, false, true)
					}
					// 3 representative multichannel rows (A/B/C) showing progressive concentration ramp
					rampAlpha := float32(0.88) - float32(colIndex)*0.11
					if rampAlpha < 0.16 {
						rampAlpha = 0.16
					}
					wellBg := tok.Accent.Alpha(rampAlpha)
					wellBd := tok.Accent.Alpha(0.55)
					ui.Column(c).Padding(float32(tok.Spacing4)).Radius(float32(tok.ControlRadius)).
						Background(tok.Surface).Border(1, tok.Border.Alpha(0.85)).Gap(3).Children(func() {
						for _, rowCh := range []string{"A", "B", "C"} {
							ui.Row(c).Size(float32(tok.Spacing24), float32(tok.Spacing16)+2).
								Radius(float32(tok.Spacing12)).Background(wellBg).Border(1, wellBd).
								Center().Children(func() {
								ui.Text(c, rowCh).FontSize(float32(tok.TextMeta)-2).Bold().TextColor(tok.AccentText).SingleLine()
							})
						}
					})
					ui.Text(c, colLabel).FontSize(float32(tok.TextMeta)-1).Bold().TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
					if !isStock {
						ui.Text(c, diluentLabel).FontSize(float32(tok.TextMeta)-2).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
					}
					if concVal > 0 {
						renderSciNumber(c, tok, concVal, "", float32(tok.TextMeta)-1, tok.Accent, true)
					}
				})
			}

			if hasStock {
				renderWellCol("Col 1", "Stock", 0, true, stockVal)
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
				colNum := i
				if hasStock {
					colNum = i + 1
				}
				renderWellCol(fmt.Sprintf("Col %d", colNum), dilExpr, i, false, concVal)
				if i < steps {
					renderTransferArrowBridge(c, tok, transferLabel)
				}
			}
		})
	})
}

func renderTransferArrowBridge(c *ui.Context, tok theme.Tokens, transferLabel string) {
	ui.Column(c).Width(float32(tok.Spacing48)+float32(tok.Spacing4)).AlignItems(ui.Center).
		Padding(0, 0, float32(tok.Spacing48)+float32(tok.Spacing12), 0).Gap(2).Children(func() {
		ui.Row(c).Padding(1, float32(tok.Spacing4)).Radius(float32(tok.ControlRadius)).
			Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.30)).Children(func() {
			ui.Text(c, transferLabel).FontSize(float32(tok.TextMeta)-1).Bold().
				TextColor(tok.Accent).FontFeatures("tnum").SingleLine().Shrink(0)
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 2).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Height(2).Background(tok.Accent.Alpha(0.75))
			ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
		})
	})
}

func renderMicroTubeNode(c *ui.Context, tok theme.Tokens, vesselID labsticker.VesselID, vesselLabel, code, dilExpr string, concVal float64, concUnit string, isStock bool) {
	ui.Column(c).Width(float32(tok.Spacing48)*1.42).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
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
