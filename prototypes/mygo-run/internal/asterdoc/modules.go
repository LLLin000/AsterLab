package asterdoc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"gopkg.in/yaml.v3"
)

func plateFromData(data map[string]any) (Plate, error) {
	raw, err := yaml.Marshal(data)
	if err != nil {
		return Plate{}, err
	}
	var plate Plate
	if err := yaml.Unmarshal(raw, &plate); err != nil {
		return Plate{}, err
	}
	plate.Groups = normalizeGroups(plate.Groups)
	if plate.Format == 0 {
		plate.Format = 96
	}
	if err := validatePlate(plate); err != nil {
		return Plate{}, err
	}
	return plate, nil
}

func plate96Spec() ModuleSpec {
	return ModuleSpec{Renders: []string{"plate96.editor", "plate96.viewer"},
		Descriptor: builtinDescriptor("plate96"),
		Validate:   specValidator("plate96"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			plate, err := plateFromData(data)
			if err != nil {
				v.moduleError(c, err)
				return
			}
			v.plate(c, &plate)
		},
	}
}

func plateSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"plate.editor", "plate.viewer"},
		Descriptor: builtinDescriptor("plate"),
		Validate:   specValidator("plate"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			plate, err := plateFromData(data)
			if err != nil {
				v.moduleError(c, err)
				return
			}
			if plate.Format == 96 {
				v.plate(c, &plate)
				return
			}
			v.genericPlate(c, &plate)
		},
	}
}

func referenceSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"reference.editor", "reference.viewer"},
		Descriptor: builtinDescriptor("reference"),
		Validate:   specValidator("reference"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			ref, _ := data["ref"].(string)
			view, _ := data["view"].(string)
			ui.Row(c).Padding(10, 12).Radius(t.Radius).Background(t.Background).
				Border(1, t.Border).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "引用").FontSize(t.Rem(.75)).Bold().TextColor(t.TextMuted)
				ui.Text(c, ref).FontSize(t.Rem(.85)).TextColor(t.Text)
				if view != "" {
					ui.Text(c, "· "+view).FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				}
			})
		},
	}
}

func experimentSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"experiment.editor", "experiment.viewer"},
		Descriptor: builtinDescriptor("experiment"),
		Validate:   specValidator("experiment"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			ui.Column(c).FillWidth().Gap(4).Padding(t.Space(3)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, textValue(data, "title")).FontSize(t.Rem(1.2)).Bold().TextColor(t.Text)
				for _, key := range []string{"objective", "research_question", "experimental_unit"} {
					if value := textValue(data, key); value != "" {
						ui.Text(c, value).FontSize(t.Rem(.85)).TextColor(t.TextMuted)
					}
				}
				if points := stringList(data, "timepoints"); len(points) > 0 {
					ui.Text(c, "时间点 · "+strings.Join(points, " / ")).
						FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				}
			})
		},
	}
}

func groupSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"group.editor", "group.viewer"},
		Descriptor: builtinDescriptor("group"),
		Validate:   specValidator("group"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			ui.Row(c).Padding(6, 10).Radius(t.Space(1)).Background(t.Background).
				Border(1, t.Border).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, textValue(data, "name")).FontSize(t.Rem(.85)).Bold().TextColor(t.Text)
				if role := textValue(data, "role"); role != "" {
					ui.Text(c, role).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
				if condition := textValue(data, "condition"); condition != "" {
					ui.Text(c, "· "+condition).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
			})
		},
	}
}

func assaySpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"assay.editor", "assay.viewer"},
		Descriptor: builtinDescriptor("assay"),
		Validate:   specValidator("assay"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			ui.Column(c).FillWidth().Gap(4).Padding(t.Space(3)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, textValue(data, "title")).FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
				line := strings.Join(nonEmpty([]string{
					textValue(data, "endpoint"), textValue(data, "method"), textValue(data, "unit"),
				}), " · ")
				if line != "" {
					ui.Text(c, line).FontSize(t.Rem(.78)).TextColor(t.TextMuted)
				}
				if points := stringList(data, "timepoints"); len(points) > 0 {
					ui.Text(c, "时间点 · "+strings.Join(points, " / ")).
						FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				}
			})
		},
	}
}

func stepSpec() ModuleSpec {
	return stepSpecAt(1)
}

func step2Spec() ModuleSpec {
	return stepSpecAt(2)
}

func stepSpecAt(schema int) ModuleSpec {
	return ModuleSpec{Renders: []string{"step.editor", "step.viewer"},
		Descriptor: descriptorFor("step", schema),
		Validate:   specValidatorFor("step", schema),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(2.5)).Radius(t.Space(1)).
				Background(t.Background).Border(1, t.Border).Gap(t.Space(1.5)).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, textValue(data, "title")).FontSize(t.Rem(.9)).Bold().TextColor(t.Text)
				if assay := textValue(data, "assay"); assay != "" {
					ui.Text(c, assay).FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				}
				if path := textValue(data, "path"); path != "" {
					ui.Text(c, path).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
			})
		},
	}
}

func workflowSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"workflow.editor", "workflow.viewer"},
		Descriptor: builtinDescriptor("workflow"),
		Validate:   specValidator("workflow"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			t := c.Theme()
			steps := stringList(data, "steps")
			edges, _ := data["edges"].([]any)
			ui.Column(c).FillWidth().Gap(4).Padding(t.Space(3)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "流程").FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
				ui.Text(c, fmt.Sprintf("%d 个步骤 · %d 条依赖", len(steps), len(edges))).
					FontSize(t.Rem(.78)).TextColor(t.TextMuted)
				ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
					for _, step := range steps {
						ui.Row(c).Padding(3, 8).Radius(t.Space(1)).Background(t.Background).
							Border(1, t.Border).Children(func() {
							ui.Text(c, step).FontSize(t.Rem(.75)).TextColor(t.Text)
						})
					}
				})
			})
		},
	}
}

func textValue(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

func stringList(data map[string]any, key string) []string {
	items, _ := data[key].([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			out = append(out, value)
		}
	}
	return out
}

func nonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

// dilutionResult is a pure derived view. It never writes a Run fact; the
// authored quantities remain the source of truth.
type dilutionResult struct {
	Stock      float64
	Diluent    float64
	Unit       string
	StockConc  float64
	TargetConc float64
	Final      float64
}

func dilutionResultFor(data map[string]any) (dilutionResult, error) {
	stockValue, stockUnit, err := quantityParts(data["stock"])
	if err != nil {
		return dilutionResult{}, fmt.Errorf("stock: %w", err)
	}
	targetValue, targetUnit, err := quantityParts(data["target"])
	if err != nil {
		return dilutionResult{}, fmt.Errorf("target: %w", err)
	}
	finalValue, finalUnit, err := quantityParts(data["final_volume"])
	if err != nil {
		return dilutionResult{}, fmt.Errorf("final_volume: %w", err)
	}
	stock, stockOK := unitFor(stockUnit)
	target, targetOK := unitFor(targetUnit)
	final, finalOK := unitFor(finalUnit)
	if !stockOK || !targetOK || !finalOK {
		return dilutionResult{}, fmt.Errorf("dilution uses an unregistered unit")
	}
	if stock.Dimension != "mass_concentration" || target.Dimension != "mass_concentration" || final.Dimension != "volume" {
		return dilutionResult{}, fmt.Errorf("dilution needs concentration/concentration/volume")
	}
	if stockValue <= 0 || targetValue <= 0 || finalValue <= 0 {
		return dilutionResult{}, fmt.Errorf("dilution quantities must be positive")
	}
	stockBase := stockValue * stock.Factor
	targetBase := targetValue * target.Factor
	if targetBase > stockBase {
		return dilutionResult{}, fmt.Errorf("target concentration cannot exceed stock concentration")
	}
	// base concentrations are g/L, base volume is L, so the result is L.
	stockVolumeBase := targetBase * finalValue * final.Factor / stockBase
	if stockVolumeBase > finalValue*final.Factor {
		return dilutionResult{}, fmt.Errorf("stock volume exceeds final volume")
	}
	stockVolume := stockVolumeBase / final.Factor
	return dilutionResult{
		Stock: stockVolume, Diluent: finalValue - stockVolume, Unit: finalUnit,
		StockConc: stockValue, TargetConc: targetValue, Final: finalValue,
	}, nil
}

func dilutionSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"dilution.editor", "dilution.viewer"},
		Descriptor: builtinDescriptor("dilution"),
		Validate:   specValidator("dilution"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			if v.Can(PermDocumentEdit) && !v.ReadOnly {
				v.GenericModuleEditor(c, textValue(data, "id"), data)
				return
			}
			t := c.Theme()
			result, err := dilutionResultFor(data)
			ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "稀释计算").FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
					ui.Spacer(c)
					ui.Text(c, "派生值 · 不写 Run").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
				})
				_, stockUnit, _ := quantityParts(data["stock"])
				_, targetUnit, _ := quantityParts(data["target"])
				_, finalUnit, _ := quantityParts(data["final_volume"])
				ui.Text(c, fmt.Sprintf("母液 %s %s · 目标 %s %s · 终体积 %s %s",
					formatNumber(result.StockConc), stockUnit,
					formatNumber(result.TargetConc), targetUnit,
					formatNumber(result.Final), finalUnit)).
					FontSize(t.Rem(.78)).TextColor(t.TextMuted)
				if err != nil {
					v.moduleError(c, err)
					return
				}
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Text(c, fmt.Sprintf("母液体积 %s %s", formatNumber(result.Stock), result.Unit)).
						FontSize(t.Rem(.85)).Bold().TextColor(t.Text)
					ui.Text(c, fmt.Sprintf("稀释液体积 %s %s", formatNumber(result.Diluent), result.Unit)).
						FontSize(t.Rem(.85)).TextColor(t.TextMuted)
				})
				ui.Text(c, "dilution.v1 · 输入未修改；实际配制以 Run 记录为准").
					FontSize(t.Rem(.68)).TextColor(t.TextMuted)
			})
		},
	}
}

func instrumentProfileSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"instrument-profile.editor", "instrument-profile.viewer"},
		Descriptor: builtinDescriptor("instrument_profile"),
		Validate:   specValidator("instrument_profile"),
		Render: func(v *View, c *ui.Context, data map[string]any) {
			if v.Can(PermDocumentEdit) && !v.ReadOnly {
				v.GenericModuleEditor(c, textValue(data, "id"), data)
				return
			}
			t := c.Theme()
			ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
					ui.Text(c, textValue(data, "class")).FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
					if model := textValue(data, "model"); model != "" {
						ui.Text(c, "· "+model).FontSize(t.Rem(.8)).TextColor(t.TextMuted)
					}
					ui.Spacer(c)
					ui.Text(c, "能力档案").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
				})
				if capabilities, ok := data["capabilities"].([]any); ok {
					ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
						for _, raw := range capabilities {
							item, _ := raw.(map[string]any)
							name, _ := item["name"].(string)
							if name == "" {
								continue
							}
							ui.Row(c).Padding(3, 8).Radius(t.Space(1)).Background(t.Background).
								Border(1, t.Border).Children(func() {
								ui.Text(c, name).FontSize(t.Rem(.72)).TextColor(t.Text)
							})
						}
					})
				}
				ui.Text(c, "声明能力，不代表某台设备、校准状态或执行结果。").
					FontSize(t.Rem(.68)).TextColor(t.TextMuted)
			})
		},
	}
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func materialSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"material.editor", "material.viewer"}, Descriptor: builtinDescriptor("material"), Validate: specValidator("material"), Render: func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Row(c).Padding(6, 10).Radius(t.Space(1)).Background(t.Background).Border(1, t.Border).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, textValue(data, "name")).FontSize(t.Rem(.85)).Bold().TextColor(t.Text)
		})
	}}
}

func sampleSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"sample.editor", "sample.viewer"}, Descriptor: builtinDescriptor("sample"), Validate: specValidator("sample"), Render: func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Row(c).Padding(6, 10).Radius(t.Space(1)).Background(t.Background).Border(1, t.Border).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, textValue(data, "sample_id")).FontSize(t.Rem(.85)).Bold().TextColor(t.Text)
		})
	}}
}

func containerSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"container.editor", "container.viewer"}, Descriptor: builtinDescriptor("container"), Validate: specValidator("container"), Render: func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Column(c).FillWidth().Gap(4).Padding(t.Space(3)).Radius(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, textValue(data, "format")).FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
		})
	}}
}

func measurementProfileSpec() ModuleSpec {
	return ModuleSpec{Renders: []string{"measurement_profile.editor", "measurement_profile.viewer"}, Descriptor: builtinDescriptor("measurement_profile"), Validate: specValidator("measurement_profile"), Render: func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Column(c).FillWidth().Gap(4).Padding(t.Space(3)).Radius(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, textValue(data, "endpoint")).FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
		})
	}}
}
