package asterdoc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"gopkg.in/yaml.v3"

	"asterlab-mygo-prototype/internal/theme"
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
	plate.Samples = normalizeSamples(plate.Samples)
	plate.Replicates = normalizeReplicates(plate.Replicates)
	plate.Roles = normalizeRoles(plate.Roles)
	plate.Factors = normalizeFactors(plate.Factors)
	plate.Layers = normalizeFactors(plate.Layers)
	plate.ExcludedWells = normalizeWellList(plate.ExcludedWells)
	if err := validatePlate(plate); err != nil {
		return Plate{}, err
	}
	return plate, nil
}

func plateSpec() ModuleSpec {
	renderPlate := func(v *View, c *ui.Context, data map[string]any) {
		plate, err := plateFromData(data)
		if err != nil {
			v.moduleError(c, err)
			return
		}
		v.plate(c, &plate)
	}
	return ModuleSpec{
		Descriptor: builtinDescriptor("plate"),
		Validate:   specValidator("plate"),
		Implementations: map[string]RendererFunc{
			"plate.editor": renderPlate,
			"plate.viewer": renderPlate,
			"plate.run":    renderPlate,
		},
	}
}

func widgetModuleSpec(desc Descriptor, validate func(map[string]any) error, render RendererFunc) ModuleSpec {
	impls := map[string]RendererFunc{}
	if desc.Renderers.Design != "" {
		impls[desc.Renderers.Design] = render
	}
	if desc.Renderers.Document != "" {
		impls[desc.Renderers.Document] = render
	}
	if desc.Renderers.Run != "" {
		impls[desc.Renderers.Run] = render
	}
	return ModuleSpec{
		Descriptor:      desc,
		Validate:        validate,
		Implementations: impls,
	}
}

func proseSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("prose"), specValidator("prose"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderProse(c, data)
	})
}

func groupTableSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("group_table"), specValidator("group_table"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderGroupTable(c, data)
	})
}

func paramTableSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("param_table"), specValidator("param_table"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderParamTable(c, data)
	})
}

func figureSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("figure"), specValidator("figure"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderFigure(c, data)
	})
}

func dataTableSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("data_table"), specValidator("data_table"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderDataTable(c, data)
	})
}

func checklistSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("checklist"), specValidator("checklist"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderChecklist(c, data)
	})
}

func calculatorSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("calculator"), specValidator("calculator"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderCalculator(c, data)
	})
}

func timerSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("timer"), specValidator("timer"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderTimer(c, data)
	})
}

func attachmentSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("attachment"), specValidator("attachment"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderAttachment(c, data)
	})
}

func subflowSpec() ModuleSpec {
	return widgetModuleSpec(builtinDescriptor("subflow"), specValidator("subflow"), func(v *View, c *ui.Context, data map[string]any) {
		v.renderSubflow(c, data)
	})
}

func moduleDocumentSpec(descriptor Descriptor, validate func(map[string]any) error, render RendererFunc) ModuleSpec {
	return ModuleSpec{
		Descriptor: descriptor,
		Validate:   validate,
		Implementations: map[string]RendererFunc{
			descriptor.Renderers.Document: render,
		},
	}
}

func referenceSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("reference"), specValidator("reference"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ref := textValue(data, "ref")
		view := textValue(data, "view")
		ui.Row(c).Padding(t.Space(2), t.Space(3)).Radius(t.Radius).Background(t.Surface).
			Border(1, t.Border).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			tok := theme.GetTokens(t)
			components.Badge(c, "引用")
			ui.Text(c, ref).FontSize(float32(tok.TextData)).TextColor(t.Text)
			if view != "" {
				ui.Text(c, "· "+view).FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
			}
		})
	})
}

func experimentSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("experiment"), specValidator("experiment"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3)).Radius(t.Radius).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			tok := theme.GetTokens(t)
			ui.Text(c, textValue(data, "title")).FontSize(float32(tok.TextSection)).Bold().TextColor(t.Text)
			for _, key := range []string{"objective", "research_question", "experimental_unit"} {
				if value := textValue(data, key); value != "" {
					ui.Text(c, value).FontSize(float32(tok.TextBody)).TextColor(t.TextMuted)
				}
			}
			if points := stringList(data, "timepoints"); len(points) > 0 {
				ui.Text(c, "时间点 · "+strings.Join(points, " / ")).
					FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
			}
		})
	})
}

func groupSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("group"), specValidator("group"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		ui.Row(c).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Background(t.Surface).
			Border(1, t.Border).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			tok := theme.GetTokens(t)
			ui.Box(c).Size(t.Space(2), t.Space(2)).Radius(t.Radius).Background(groupColorForCtx(c, textValue(data, "id")))
			ui.Text(c, textValue(data, "name")).FontSize(float32(tok.TextBody)).Bold().TextColor(t.Text)
			if role := textValue(data, "role"); role != "" {
				components.Badge(c, role)
			}
			if condition := textValue(data, "condition"); condition != "" {
				ui.Text(c, condition).FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
			}
		})
	})
}

func assaySpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("assay"), specValidator("assay"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		col := ui.Column(c).FillWidth().Gap(t.Space(1))
		if v != nil && v.UnifiedLive {
			col.Padding(t.Space(1.5), 0)
		} else {
			col.Padding(t.Space(3)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
		}
		col.Children(func() {
			tok := theme.GetTokens(t)
			ui.Text(c, textValue(data, "title")).FontSize(float32(tok.TextSection)).Bold().TextColor(t.Text)
			line := strings.Join(nonEmpty([]string{
				textValue(data, "endpoint"), textValue(data, "method"), textValue(data, "unit"),
			}), " · ")
			if line != "" {
				ui.Text(c, line).FontSize(float32(tok.TextData)).TextColor(t.TextMuted)
			}
			if points := stringList(data, "timepoints"); len(points) > 0 {
				ui.Text(c, "时间点 · "+strings.Join(points, " / ")).
					FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
			}
		})
	})
}

func stepSpec() ModuleSpec {
	return ModuleSpec{
		Descriptor: descriptorFor("step", 1),
		Validate:   specValidatorFor("step", 1),
	}
}

func renderStepSummary(v *View, c *ui.Context, data map[string]any) {
	tok := theme.GetTokens(c.Theme())
	prevStep := v.StepID
	v.StepID = textValue(data, "id")
	defer func() { v.StepID = prevStep }()
	stepCol := ui.Column(c).FillWidth().Gap(float32(tok.Spacing8))
	if v == nil || !v.UnifiedLive {
		stepCol.Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border)
	} else {
		stepCol.Padding(float32(tok.Spacing8), 0)
	}
	stepCol.Children(func() {
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, textValue(data, "title")).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			if exec := textValue(data, "execution"); exec == "organizational" {
				components.Badge(c, "组织分组")
			} else if v == nil || !v.UnifiedLive {
				components.Badge(c, "步骤", components.BadgeOptions{Tone: components.BadgeAccent})
			}
			ui.Spacer(c)
			for _, tp := range stringList(data, "timepoints") {
				if strings.TrimSpace(tp) != "" {
					components.Badge(c, tp)
				}
			}
		})
		if blocks, ok := data["blocks"].([]any); ok && len(blocks) > 0 {
			for _, raw := range blocks {
				bm, _ := raw.(map[string]any)
				if bm == nil {
					continue
				}
				bk := textValue(bm, "kind")
				bs := int(numOrZero(bm["schema"]))
				if bs <= 0 {
					bs = 1
				}
				v.RenderModule(c, bk, bs, bm)
			}
		}
		if v.Doc != nil {
			for _, refID := range stringList(data, "block_refs") {
				for i := range v.Doc.Modules {
					if v.Doc.Modules[i].ID == refID {
						m := v.Doc.Modules[i]
						v.RenderModule(c, m.Kind, m.Schema, m.Data)
						break
					}
				}
			}
		}
	})
}

func workflowSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("workflow"), specValidator("workflow"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		steps := stringList(data, "steps")
		edges, _ := data["edges"].([]any)
		ui.Column(c).FillWidth().Gap(t.Space(1.5)).Padding(t.Space(3)).Radius(t.Radius).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, "流程").FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("%d 个步骤 · %d 条依赖", len(steps), len(edges))).
				FontSize(t.Rem(.78)).TextColor(t.TextMuted).FontFeatures("tnum")
			ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
				for _, step := range steps {
					components.Badge(c, step)
				}
			})
		})
	})
}

func instrumentProfileSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("instrument_profile"), specValidator("instrument_profile"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		col := ui.Column(c).FillWidth().Gap(t.Space(1.5))
		if v != nil && v.UnifiedLive {
			col.Padding(t.Space(1.5), 0)
		} else {
			col.Padding(t.Space(3)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
		}
		col.Children(func() {
			tok := theme.GetTokens(t)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Text(c, textValue(data, "class")).FontSize(float32(tok.TextSection)).Bold().TextColor(t.Text)
				if model := textValue(data, "model"); model != "" {
					ui.Text(c, "· "+model).FontSize(float32(tok.TextData)).TextColor(t.TextMuted)
				}
				ui.Spacer(c)
				components.Badge(c, "能力档案")
			})
			if capabilities, ok := data["capabilities"].([]any); ok {
				ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
					for _, raw := range capabilities {
						item, _ := raw.(map[string]any)
						if name := textValue(item, "name"); name != "" {
							components.Badge(c, name)
						}
					}
				})
			}
		})
	})
}

func materialSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("material"), specValidator("material"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		tok := theme.GetTokens(t)
		ui.Row(c).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Background(t.Surface).
			Border(1, t.Border).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, textValue(data, "name")).FontSize(float32(tok.TextBody)).Bold().TextColor(t.Text)
			if role := textValue(data, "role"); role != "" {
				components.Badge(c, role)
			}
		})
	})
}

func sampleSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("sample"), specValidator("sample"), func(v *View, c *ui.Context, data map[string]any) {
		t := c.Theme()
		tok := theme.GetTokens(t)
		ui.Row(c).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Background(t.Surface).
			Border(1, t.Border).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, textValue(data, "sample_id")).FontSize(float32(tok.TextBody)).Bold().TextColor(t.Text)
			if grp := textValue(data, "group_ref"); grp != "" {
				components.Badge(c, grp)
			}
		})
	})
}

func readoutSpec() ModuleSpec {
	return moduleDocumentSpec(builtinDescriptor("readout"), func(data map[string]any) error {
		if err := specValidatorFor("readout", 1)(data); err != nil {
			return err
		}
		return validateReadoutFields(data)
	}, func(v *View, c *ui.Context, data map[string]any) {
		tok := theme.GetTokens(c.Theme())
		endpoint := textValue(data, "endpoint")
		schemaID := textValue(data, "schema_id")
		vkind := textValue(data, "value_kind")
		unit := textValue(data, "unit")
		pfmt := textValue(data, "plate_format")
		title := firstNonEmpty(textValue(data, "title"), endpoint, "观测读数定义")
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Padding(float32(tok.Spacing12)).
			Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Spacer(c)
				if vkind != "" {
					components.Badge(c, readoutKindLabel(vkind), components.BadgeOptions{Tone: components.BadgeAccent})
				}
				if unit != "" {
					components.Badge(c, unit)
				}
				if pfmt != "" {
					components.Badge(c, pfmt+" 孔")
				}
			})
			meta := nonEmpty([]string{
				func() string {
					if endpoint != "" && endpoint != title {
						return "观测终点 · " + endpoint
					}
					return ""
				}(),
				func() string {
					if schemaID != "" && !IsRawIdentifier(schemaID) {
						return schemaID
					}
					return ""
				}(),
			})
			if len(meta) > 0 {
				ui.Text(c, strings.Join(meta, " · ")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			}
		})
	})
}

func readoutKindLabel(vkind string) string {
	switch strings.ToLower(strings.TrimSpace(vkind)) {
	case "numeric":
		return "定量数值"
	case "table":
		return "结构化表格"
	case "plate":
		return "孔板矩阵"
	case "categorical":
		return "定性分类"
	case "file":
		return "文件 / 图像"
	default:
		return vkind
	}
}

func textValue(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

func stringList(data map[string]any, key string) []string {
	if data == nil {
		return nil
	}
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

func formatNumber(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
