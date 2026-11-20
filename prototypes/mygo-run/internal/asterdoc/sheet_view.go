package asterdoc

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// RenderDocumentSheet renders the entire AsterML v2 document as a single continuous,
// publication-clean sheet covering experiment metadata, group/material overviews,
// unowned top-level blocks, and all sequential workflow steps in read-only fidelity.
func (v *View) RenderDocumentSheet(c *ui.Context) {
	if v == nil || v.Doc == nil || c == nil {
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)

	// Switch to read-only ModeProtocol temporarily while keeping Interactive overlays
	// (e.g. Maximize plate/figure modal) working seamlessly against the host View.
	prevMode := v.Mode
	prevRO := v.ReadOnly
	prevHideHdr := v.HideStepHeader
	v.Mode = ModeProtocol
	v.ReadOnly = true
	v.HideStepHeader = true
	defer func() {
		v.Mode = prevMode
		v.ReadOnly = prevRO
		v.HideStepHeader = prevHideHdr
	}()

	doc := v.Doc
	stepOwnedRefs := map[string]bool{}
	var steps []*Module
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "step" {
			steps = append(steps, &doc.Modules[i])
			for _, ref := range stringList(doc.Modules[i].Data, "block_refs") {
				stepOwnedRefs[ref] = true
			}
		}
	}

	expTitle := "未命名实验方案"
	var expData map[string]any
	for _, m := range doc.Modules {
		if m.Kind == "experiment" {
			expData = m.Data
			if s := strings.TrimSpace(textValue(m.Data, "title")); s != "" {
				expTitle = s
			}
			break
		}
	}

	ui.Column(c).FillWidth().Padding(t.Space(4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(float32(tok.Spacing48)*18).MaxWidthPercent(100).
			Padding(t.Space(4)).Radius(float32(tok.PanelRadius)).
			Background(tok.Surface).Border(1, tok.Border).
			Shadow(0, 1, 3, 0, tok.Text.Alpha(0.05)).
			Gap(t.Space(4)).Children(func() {
			// Sheet Header + Experiment Objective Card
			ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
					ui.Icon(c, lucide.FileText()).Size(t.Space(5), t.Space(5)).TextColor(tok.Accent)
					ui.Text(c, expTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
						Grow(1).MinWidth(float32(tok.Spacing48)*3.2)
					if expData != nil {
						if sched := NormalizeDateYMD(textValue(expData, "scheduled_date")); sched != "" {
							components.Badge(c, "计划 "+sched, components.BadgeOptions{Tone: components.BadgeNeutral})
						}
					}
					sheetTag := "实验方案预览"
					if strings.TrimSpace(v.RunID) != "" {
						sheetTag = "实验实测记录"
					}
					components.Badge(c, sheetTag, components.BadgeOptions{Tone: components.BadgeAccent})
				})

				if expData != nil {
					summaryPairs := [][2]string{}
					if obj := strings.TrimSpace(textValue(expData, "objective")); obj != "" {
						summaryPairs = append(summaryPairs, [2]string{"实验目的", obj})
					}
					if hyp := strings.TrimSpace(textValue(expData, "hypothesis")); hyp != "" {
						summaryPairs = append(summaryPairs, [2]string{"科学假说", hyp})
					}
					if unit := strings.TrimSpace(textValue(expData, "experimental_unit")); unit != "" {
						summaryPairs = append(summaryPairs, [2]string{"实验单元", unit})
					}
					if ep := strings.TrimSpace(textValue(expData, "primary_endpoint")); ep != "" {
						method := strings.TrimSpace(textValue(expData, "endpoint_method"))
						val := ep
						if method != "" {
							val += " · " + method
						}
						summaryPairs = append(summaryPairs, [2]string{"核心终点", val})
					}
					if len(summaryPairs) > 0 {
						ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Border(1, tok.Border).Gap(t.Space(1)).Children(func() {
							for _, kv := range summaryPairs {
								ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Start).Children(func() {
									ui.Box(c).Width(float32(tok.Spacing48)*1.8).Children(func() {
										ui.Text(c, kv[0]).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
									})
									ui.Text(c, kv[1]).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).
										Grow(1).MinWidth(0)
								})
							}
						})
					}
				}
			})

			ui.Divider(c)

			// Global Overview Modules (groups, materials, top-level unowned figures/plates/tables)
			hasGlobal := false
			for _, m := range doc.Modules {
				switch m.Kind {
				case "experiment", "assay", "sample", "readout", "instrument_profile", "reference", "workflow", "step":
					continue
				}
				if stepOwnedRefs[m.ID] {
					continue
				}
				if !hasGlobal {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						ui.Text(c, "一、实验分组与关键材料总览").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					})
					hasGlobal = true
				}
				v.RenderModule(c, m.Kind, m.Schema, m.Data)
			}

			// Sequential Workflow Steps on the Same Continuous Sheet
			if len(steps) > 0 {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).
					Padding(t.Space(2), t.Space(0), t.Space(0), t.Space(0)).Children(func() {
					ui.Text(c, "二、完整实验操作流程").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					components.Badge(c, fmt.Sprintf("共 %d 步", len(steps)), components.BadgeOptions{Tone: components.BadgeNeutral})
				})
			}

			for idx, step := range steps {
				idx := idx
				step := step
				stepTitle := stripLeadingStepNumber(firstNonEmpty(textValue(step.Data, "title"), step.ID))
				execMode := firstNonEmpty(textValue(step.Data, "execution"), "executable")
				tps := strings.Join(stringList(step.Data, "timepoints"), " · ")

				ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Gap(t.Space(3)).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						components.Badge(c, fmt.Sprintf("步骤 %02d", idx+1), components.BadgeOptions{Tone: components.BadgeAccent})
						ui.Text(c, stepTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
							Grow(1).MinWidth(0)
						if tps != "" {
							components.Badge(c, tps, components.BadgeOptions{Tone: components.BadgeNeutral})
						}
						if execMode == "organizational" {
							components.Badge(c, "组织分组", components.BadgeOptions{Tone: components.BadgeNeutral})
						}
					})
					StepBlockComposer(v, c, step)
				})
			}
		})
	})
}
