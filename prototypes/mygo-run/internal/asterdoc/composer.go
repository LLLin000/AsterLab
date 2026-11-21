package asterdoc

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

type paletteEntry struct {
	Kind  string
	Label string
	Icon  *ui.SVG
}

type stepBlockDrag struct {
	StepID    string
	BlockID   string
	FromIndex int
}

// StepBlockComposer renders the Design-mode interactive step workbench:
// step metadata header + ordered composable blocks[] + block_refs + 10-module insert palette.
func StepBlockComposer(v *View, c *ui.Context, step *Module) {
	if v == nil || c == nil || step == nil {
		return
	}
	if v.UnifiedLive {
		renderLiveStepComposer(v, c, step)
		return
	}
	tok := theme.GetTokens(c.Theme())
	v.StepID = step.ID
	stepTitle := firstNonEmpty(textValue(step.Data, "title"), step.ID)
	execMode := firstNonEmpty(textValue(step.Data, "execution"), "executable")
	assayRef := textValue(step.Data, "assay")
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
		// Step Header Card (skipped when parent workbench/Run card already renders header)
		if !v.HideStepHeader {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if editable {
						titleBuf := v.ensureWidgetDraft("step_title:"+step.ID, stepTitle)
						ui.Box(c).FillWidth().Children(func() {
							components.TextBox(c, titleBuf, components.TextBoxOptions{}).Placeholder("步骤标题")
						})
						if *titleBuf != stepTitle && strings.TrimSpace(*titleBuf) != "" {
							if components.Button(c, "保存标题", components.ButtonSecondary).Clicked() && v.OnUpdateModule != nil {
								v.OnUpdateModule(step.ID, map[string]any{"title": strings.TrimSpace(*titleBuf)})
							}
						}
					} else {
						ui.Text(c, stepTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
						ui.Spacer(c)
					}
					if execMode == "organizational" {
						components.Badge(c, "组织分组")
					} else {
						components.Badge(c, "可执行步骤", components.BadgeOptions{Tone: components.BadgeAccent})
					}
					if assayRef != "" {
						components.Badge(c, assayRef)
					}
				})
				if editable {
					ui.Row(c).Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("ID: %s · 自由组合模块（无步骤类型限制）", step.ID)).
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						ui.Spacer(c)
						toggleLabel := "转为组织分组"
						nextExec := "organizational"
						if execMode == "organizational" {
							toggleLabel = "转为可执行步骤"
							nextExec = "executable"
						}
						if len(mapSlice(step.Data["blocks"])) == 0 && v.OnUpdateModule != nil {
							if components.Button(c, toggleLabel, components.ButtonOutline).Clicked() {
								props := map[string]any{"execution": nextExec}
								if nextExec == "organizational" {
									props["path"] = ""
									props["assay"] = ""
								}
								v.OnUpdateModule(step.ID, props)
							}
						}
					})
				}
			})
		}

		// Ordered inline blocks[] stream
		blocks := mapSlice(step.Data["blocks"])
		if len(blocks) == 0 && len(stringList(step.Data, "block_refs")) == 0 {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing24)).Radius(float32(tok.PanelRadius)).
				Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "该步骤暂无内容模块").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Text(c, "从下方模块栏选择参数表、示意图/SVG、检查清单、计算器或数据表自由组合。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
		}

		for idx, bm := range blocks {
			idx := idx
			bk := textValue(bm, "kind")
			bid := textValue(bm, "id")
			bs := int(numOrZero(bm["schema"]))
			if bs <= 0 {
				bs = 1
			}
			wrapper := ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Radius(float32(tok.PanelRadius)).Children(func() {
				if editable {
					hdr := ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Icon(c, lucide.GripVertical()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
						ui.Text(c, fmt.Sprintf("#%d · %s", idx+1, blockKindLabel(bk))).
							FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).FontFeatures("tnum")
						ui.Spacer(c)
						if idx > 0 && (v.OnMoveStepBlock != nil || v.OnMoveStepBlockIndex != nil) {
							upBtn := components.IconButton(c, lucide.ArrowUp(), "上移模块", components.IconButtonCompact)
							components.Tooltip(c, upBtn, "上移此模块")
							if upBtn.Clicked() {
								if v.OnMoveStepBlock != nil {
									v.OnMoveStepBlock(step.ID, bid, "up")
								} else if v.OnMoveStepBlockIndex != nil {
									v.OnMoveStepBlockIndex(step.ID, bid, idx-1)
								}
							}
						}
						if idx < len(blocks)-1 && (v.OnMoveStepBlock != nil || v.OnMoveStepBlockIndex != nil) {
							dnBtn := components.IconButton(c, lucide.ArrowDown(), "下移模块", components.IconButtonCompact)
							components.Tooltip(c, dnBtn, "下移此模块")
							if dnBtn.Clicked() {
								if v.OnMoveStepBlock != nil {
									v.OnMoveStepBlock(step.ID, bid, "down")
								} else if v.OnMoveStepBlockIndex != nil {
									v.OnMoveStepBlockIndex(step.ID, bid, idx+1)
								}
							}
						}
						if v.OnRemoveStepBlock != nil {
							rmBtn := components.IconButton(c, lucide.Trash(), "移除模块", components.IconButtonCompact)
							components.Tooltip(c, rmBtn, "从步骤中移除此模块")
							if rmBtn.Clicked() {
								v.OnRemoveStepBlock(step.ID, bid)
							}
						}
					})
					hdr.Cursor(ui.CursorGrab).Drag(stepBlockDrag{
						StepID:    step.ID,
						BlockID:   bid,
						FromIndex: idx,
					})
				}
				v.RenderModule(c, bk, bs, bm)
			})
			if editable {
				if dragged, over := ui.DragOver[stepBlockDrag](wrapper); over && dragged.StepID == step.ID && dragged.FromIndex != idx {
					wrapper.Border(2, tok.Accent)
				}
				if dragged, dropped := ui.Drop[stepBlockDrag](wrapper); dropped && dragged.StepID == step.ID && dragged.FromIndex != idx {
					if v.OnMoveStepBlockIndex != nil {
						v.OnMoveStepBlockIndex(step.ID, dragged.BlockID, idx)
					} else if v.OnMoveStepBlock != nil {
						if dragged.FromIndex < idx {
							for stepMove := dragged.FromIndex; stepMove < idx; stepMove++ {
								v.OnMoveStepBlock(step.ID, dragged.BlockID, "down")
							}
						} else {
							for stepMove := dragged.FromIndex; stepMove > idx; stepMove-- {
								v.OnMoveStepBlock(step.ID, dragged.BlockID, "up")
							}
						}
					}
				}
			}
		}

		// Resolved top-level block_refs previews (e.g. shared plate)
		for _, refID := range stringList(step.Data, "block_refs") {
			refID := refID
			if v.Doc == nil {
				continue
			}
			for i := range v.Doc.Modules {
				if v.Doc.Modules[i].ID == refID {
					m := v.Doc.Modules[i]
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						components.Badge(c, "引用全局模块 · "+firstNonEmpty(textValue(m.Data, "title"), blockKindLabel(m.Kind)))
						ui.Spacer(c)
						if editable && v.OnUpdateModule != nil {
							unBtn := components.IconButton(c, lucide.Trash(), "取消引用", components.IconButtonCompact)
							components.Tooltip(c, unBtn, "移除模块引用")
							if unBtn.Clicked() {
								nextRefs := removeString(append([]string{}, stringList(step.Data, "block_refs")...), refID)
								v.OnUpdateModule(step.ID, map[string]any{"block_refs": nextRefs})
							}
						}
					})
					v.RenderModule(c, m.Kind, m.Schema, m.Data)
					break
				}
			}
		}

		// Bottom 10-Module + Plate Insertion Palette
		if editable && execMode != "organizational" && v.OnInsertStepBlock != nil {
			entries := []paletteEntry{
				{Kind: "prose", Label: "文本/注意", Icon: lucide.FileText()},
				{Kind: "param_table", Label: "参数表", Icon: lucide.SlidersHorizontal()},
				{Kind: "figure", Label: "示意图/SVG", Icon: lucide.Image()},
				{Kind: "data_table", Label: "数据表", Icon: lucide.Table2()},
				{Kind: "group_table", Label: "分组表", Icon: lucide.Table2()},
				{Kind: "checklist", Label: "检查清单", Icon: lucide.SquareCheck()},
				{Kind: "calculator", Label: "计算器", Icon: lucide.Calculator()},
				{Kind: "timer", Label: "计时器", Icon: lucide.Timer()},
				{Kind: "attachment", Label: "附件/拍照", Icon: lucide.Paperclip()},
				{Kind: "subflow", Label: "子流程", Icon: lucide.GitBranch()},
				{Kind: "plate", Label: "标准孔板", Icon: lucide.Grid2x2()},
			}
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.Plus()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
					ui.Text(c, "向本步骤插入通用模块").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					ui.Spacer(c)
					menuBtn := iconButton(c, lucide.Plus(), "模块菜单", components.ButtonSecondary)
					components.MenuButton(c, menuBtn, func(m *components.Menu) {
						for i, ent := range entries {
							if i == 5 || i == 9 {
								m.Separator()
							}
							if m.Item(ent.Label).Chosen() {
								v.OnInsertStepBlock(step.ID, -1, defaultStarterBlock(v.Doc, step.ID, ent.Kind))
							}
						}
					})
				})
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
					for _, ent := range entries {
						ent := ent
						if iconButton(c, ent.Icon, ent.Label, components.ButtonOutline).Clicked() {
							blk := defaultStarterBlock(v.Doc, step.ID, ent.Kind)
							v.OnInsertStepBlock(step.ID, -1, blk)
						}
					}
				})
			})
		}
	})
}

func blockKindLabel(kind string) string {
	switch kind {
	case "prose":
		return "富文本"
	case "group_table":
		return "分组表"
	case "param_table":
		return "参数表"
	case "figure":
		return "示意图 / 图片"
	case "data_table":
		return "数据记录表"
	case "checklist":
		return "检查清单"
	case "calculator":
		return "计算器"
	case "timer":
		return "计时器"
	case "attachment":
		return "附件 / 采图"
	case "subflow":
		return "子流程"
	case "plate":
		return "容器 / 孔板"
	default:
		return kind
	}
}

func defaultStarterBlock(doc *Document, stepID, kind string) map[string]any {
	return DefaultStarterBlock(doc, stepID, kind)
}

// DefaultStarterBlock builds a ready-to-insert generic AsterML v2 module map.
func DefaultStarterBlock(doc *Document, stepID, kind string) map[string]any {
	return DefaultStarterBlockWithPrefs(doc, stepID, kind, 0, 0)
}

// DefaultStarterBlockWithPrefs builds a ready-to-insert generic AsterML v2 module map
// honoring user preferences for default plate format and mastermix excess percentage.
func DefaultStarterBlockWithPrefs(doc *Document, stepID, kind string, plateFormat, excessPct int) map[string]any {
	id := nextUniqueBlockID(doc, stepID+"-"+sanitizeIDFrag(kind))
	switch kind {
	case "prose":
		return map[string]any{"kind": "prose", "id": id, "schema": 1, "tone": "plain", "body": "在此输入操作说明…"}
	case "group_table":
		return map[string]any{"kind": "group_table", "id": id, "schema": 1, "title": "实验分组", "rows": []any{
			map[string]any{"id": id + "-ctrl", "name": "对照组", "role": "control", "independent_n": 3},
			map[string]any{"id": id + "-treat", "name": "处理组", "role": "treatment", "independent_n": 3},
		}}
	case "param_table":
		return map[string]any{"kind": "param_table", "id": id, "schema": 1, "title": "实验条件", "rows": []any{
			map[string]any{"key": "volume", "label": "反应体积", "value": 100, "unit": "uL", "record_actual": true},
		}}
	case "figure":
		return map[string]any{
			"kind": "figure", "id": id, "schema": 1, "title": "图片 / 示意图",
			"caption": "",
			"svg":     `<svg viewBox="0 0 360 160"></svg>`,
		}
	case "data_table":
		return map[string]any{"kind": "data_table", "id": id, "schema": 1, "title": "数据记录表", "columns": []any{
			map[string]any{"key": "sample", "label": "样本", "value_kind": "text"},
			map[string]any{"key": "reading", "label": "测定值", "value_kind": "number", "unit": "AU"},
		}}
	case "checklist":
		return map[string]any{"kind": "checklist", "id": id, "schema": 1, "items": []any{
			map[string]any{"id": id + "-c1", "label": "核对实验器材与试剂", "blocking": true},
		}}
	case "calculator":
		blk := DefaultCalculatorPreset(doc, stepID, "dilution")
		if excessPct > 0 {
			blk["excess_pct"] = excessPct
		}
		return blk
	case "timer":
		return map[string]any{
			"kind": "timer", "id": id, "schema": 1, "title": "培养计时", "mode": "countdown",
			"duration": map[string]any{"value": 30, "unit": "min"},
		}
	case "attachment":
		return map[string]any{"kind": "attachment", "id": id, "schema": 1, "title": "实验照片/附件", "scope": "step_evidence", "media_hint": "上传当前步骤现场照片或原始文件"}
	case "subflow":
		return map[string]any{"kind": "subflow", "id": id, "schema": 1, "title": "详细子步骤", "items": []any{
			map[string]any{"id": id + "-s1", "title": "准备管架并逐级混匀"},
		}}
	case "plate":
		firstGroup := "g-control"
		if doc != nil {
			if gl := doc.GroupList(); len(gl) > 0 {
				firstGroup = gl[0].ID
			}
		}
		fmtVal := plateFormat
		switch fmtVal {
		case 4, 6, 12, 24, 48, 96, 384:
		default:
			fmtVal = 12
		}
		return map[string]any{"kind": "plate", "id": id, "schema": 1, "title": fmt.Sprintf("%d 孔板布局", fmtVal), "format": fmtVal, "groups": map[string]any{firstGroup: []any{"A1"}}}
	}
	return map[string]any{"kind": "prose", "id": id, "schema": 1, "body": "新模块"}
}

func nextUniqueBlockID(doc *Document, base string) string {
	used := map[string]bool{}
	if doc != nil {
		flat, _ := flattenModules(doc.Modules)
		for _, m := range flat {
			used[m.ID] = true
		}
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !used[cand] {
			return cand
		}
	}
}

// DefaultCalculatorPreset constructs a unified parameter-formula table preset.
func DefaultCalculatorPreset(doc *Document, stepID, preset string) map[string]any {
	id := nextUniqueBlockID(doc, stepID+"-calc")
	switch preset {
	case "mastermix":
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "预混液配制计算", "mode": "formula",
			"rows": []any{
				map[string]any{"key": "n", "label": "反应数 (含余量)", "kind": "input", "value": 24, "unit": "孔"},
				map[string]any{"key": "mix_per", "label": "单孔 2× Mix", "kind": "input", "value": 10, "unit": "uL"},
				map[string]any{"key": "primer_per", "label": "单孔引物", "kind": "input", "value": 1.6, "unit": "uL"},
				map[string]any{"key": "water_per", "label": "单孔无核酸酶水", "kind": "input", "value": 6.4, "unit": "uL"},
				map[string]any{"key": "mix_tot", "label": "总 2× Mix", "kind": "formula", "formula": "n * mix_per", "unit": "uL"},
				map[string]any{"key": "primer_tot", "label": "总引物", "kind": "formula", "formula": "n * primer_per", "unit": "uL"},
				map[string]any{"key": "water_tot", "label": "总水", "kind": "formula", "formula": "n * water_per", "unit": "uL"},
			},
		}
	case "molar":
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "摩尔配液称量", "mode": "formula",
			"rows": []any{
				map[string]any{"key": "conc_mm", "label": "目标浓度", "kind": "input", "value": 10, "unit": "mM"},
				map[string]any{"key": "vol_ml", "label": "配制体积", "kind": "input", "value": 5, "unit": "mL"},
				map[string]any{"key": "mw", "label": "分子量", "kind": "input", "value": 543.5, "unit": "g/mol"},
				map[string]any{"key": "mass_mg", "label": "需称量溶质", "kind": "formula", "formula": "conc_mm * vol_ml * mw / 1000", "unit": "mg"},
			},
		}
	case "seeding":
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "细胞铺板计算", "mode": "formula",
			"rows": []any{
				map[string]any{"key": "density", "label": "实测悬液密度", "kind": "input", "value": 500000, "unit": "cells/mL"},
				map[string]any{"key": "per_well", "label": "每孔目标细胞数", "kind": "input", "value": 5000, "unit": "cells"},
				map[string]any{"key": "well_ul", "label": "每孔接种体积", "kind": "input", "value": 100, "unit": "uL"},
				map[string]any{"key": "wells", "label": "铺板总孔数 (含余量)", "kind": "input", "value": 60, "unit": "孔"},
				map[string]any{"key": "susp_ml", "label": "需取细胞悬液", "kind": "formula", "formula": "per_well * wells / density", "unit": "mL"},
				map[string]any{"key": "med_ml", "label": "补加培养基", "kind": "formula", "formula": "well_ul * wells / 1000 - per_well * wells / density", "unit": "mL"},
			},
		}
	case "blank":
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "参数公式计算表", "mode": "formula",
			"rows": []any{
				map[string]any{"key": "a", "label": "输入参数 A", "kind": "input", "value": 10, "unit": "uL"},
				map[string]any{"key": "b", "label": "倍率 / 样本数", "kind": "input", "value": 3, "unit": ""},
				map[string]any{"key": "res", "label": "计算输出", "kind": "formula", "formula": "a * b", "unit": "uL"},
			},
		}
	default: // dilution
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "母液稀释配制", "mode": "formula",
			"rows": []any{
				map[string]any{"key": "c1", "label": "母液浓度 (C1)", "kind": "input", "value": 10, "unit": "mM"},
				map[string]any{"key": "c2", "label": "目标浓度 (C2)", "kind": "input", "value": 0.5, "unit": "mM"},
				map[string]any{"key": "v2", "label": "配置总体积 (V2)", "kind": "input", "value": 1000, "unit": "uL"},
				map[string]any{"key": "v1", "label": "取母液体积 (V1)", "kind": "formula", "formula": "v2 * c2 / c1", "unit": "uL"},
				map[string]any{"key": "dil", "label": "补加稀释液", "kind": "formula", "formula": "v2 - v2 * c2 / c1", "unit": "uL"},
			},
		}
	}
}

func renderLiveStepComposer(v *View, c *ui.Context, step *Module) {
	tok := theme.GetTokens(c.Theme())
	prevStep := v.StepID
	v.StepID = step.ID
	defer func() { v.StepID = prevStep }()
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	stepTitle := firstNonEmpty(textValue(step.Data, "title"), step.ID)
	blocks := mapSlice(step.Data["blocks"])

	buildInsertMenu := func(m *components.Menu, insertIdx int) {
		if m.Item("文本段落").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "prose"))
		}
		if m.Item("检查清单").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "checklist"))
		}
		if m.Item("计时器").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "timer"))
		}
		m.Separator()
		if m.Item("实验分组表").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "group_table"))
		}
		if m.Item("实验条件参数表").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "param_table"))
		}
		if m.Item("孔板 / 管架").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "plate"))
		}
		if m.Item("通用数据记录表").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "data_table"))
		}
		m.Group("定量计算表", func(sub *components.Menu) {
			if sub.Item("母液稀释 (C1·V1=C2·V2)").Chosen() {
				v.OnInsertStepBlock(step.ID, insertIdx, DefaultCalculatorPreset(v.Doc, step.ID, "dilution"))
			}
			if sub.Item("预混液体系配制").Chosen() {
				v.OnInsertStepBlock(step.ID, insertIdx, DefaultCalculatorPreset(v.Doc, step.ID, "mastermix"))
			}
			if sub.Item("摩尔配液称量").Chosen() {
				v.OnInsertStepBlock(step.ID, insertIdx, DefaultCalculatorPreset(v.Doc, step.ID, "molar"))
			}
			if sub.Item("细胞计数铺板").Chosen() {
				v.OnInsertStepBlock(step.ID, insertIdx, DefaultCalculatorPreset(v.Doc, step.ID, "seeding"))
			}
			if sub.Item("空白参数公式表").Chosen() {
				v.OnInsertStepBlock(step.ID, insertIdx, DefaultCalculatorPreset(v.Doc, step.ID, "blank"))
			}
		})
		m.Separator()
		if m.Item("图片 / 示意图").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "figure"))
		}
		if m.Item("附件与采图").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "attachment"))
		}
		if m.Item("子流程").Chosen() {
			v.OnInsertStepBlock(step.ID, insertIdx, defaultStarterBlock(v.Doc, step.ID, "subflow"))
		}
		if sharedBlockClipboard != nil {
			m.Separator()
			if m.Item("粘贴已复制组件").Shortcut("Ctrl+V").Chosen() {
				clone := cloneAnyMap(sharedBlockClipboard)
				delete(clone, "id")
				v.OnInsertStepBlock(step.ID, insertIdx, clone)
			}
		}
	}

	renderHoverInsertGap := func(insertIdx int, alwaysShow bool) {
		if !editable || v.OnInsertStepBlock == nil {
			return
		}
		gapBox := ui.Row(c.Key(fmt.Sprintf("gap-%s-%d", step.ID, insertIdx))).FillWidth().
			Height(float32(tok.Spacing16)).AlignItems(ui.Center).Gap(float32(tok.Spacing4))
		hovered := gapBox.Hovered()
		targetOp := float32(0)
		if hovered || alwaysShow {
			targetOp = 1
		}
		op := gapBox.AnimateTo("gap-op", targetOp, theme.MotionMicroFade)
		gapBox.Opacity(op).Children(func() {
			if op > 0.02 || hovered || alwaysShow {
				ui.Box(c).Grow(1).Height(1).Background(tok.Accent.Alpha(.35))
				plusBtn := components.IconButton(c, lucide.Plus(), "在此处插入组件", components.IconButtonCompact)
				components.MenuButton(c, plusBtn, func(m *components.Menu) {
					buildInsertMenu(m, insertIdx)
				})
				ui.Box(c).Grow(1).Height(1).Background(tok.Accent.Alpha(.35))
			}
		})
	}

	// Compute subtle step-checklist completion state for instrument-style indicator settle
	totalStepChecks, doneStepChecks := 0, 0
	for _, bm := range blocks {
		if textValue(bm, "kind") == "checklist" {
			for _, it := range mapSlice(bm["items"]) {
				totalStepChecks++
				cid := textValue(it, "id")
				doneInline, _ := it["done"].(bool)
				if doneInline || v.CheckedItems[cid] || v.CheckedItems[step.ID+":"+cid] {
					doneStepChecks++
				}
			}
		}
	}

	if v.stepCollapsed == nil {
		v.stepCollapsed = map[string]bool{}
	}
	collapsed := v.stepCollapsed[step.ID]
	stepMatchesSearch := false
	if sq := strings.ToLower(strings.TrimSpace(v.SearchQuery)); sq != "" {
		if strings.Contains(strings.ToLower(stepTitle), sq) {
			stepMatchesSearch = true
		} else {
			for _, bm := range blocks {
				if strings.Contains(strings.ToLower(fmt.Sprintf("%v", bm)), sq) {
					stepMatchesSearch = true
					collapsed = false
					break
				}
			}
		}
	}

	stepBox := ui.Column(c).FillWidth().Gap(float32(tok.Spacing8))
	if stepMatchesSearch {
		stepBox.Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
			Background(tok.AccentSoftBg.Alpha(0.35)).Border(1, tok.Accent.Alpha(0.45))
	}
	stepBox.Children(func() {
		// Clean Obsidian H2 Step Heading with collapse chevron
		hdrRow := ui.Row(c.Key("live-step-hdr:"+step.ID)).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4))
		if v.ScrollToStepID != "" && v.ScrollToStepID == step.ID {
			hdrRow.ScrollIntoView()
			v.ScrollToStepID = ""
		}
		hdrRow.Children(func() {
			chev := lucide.ChevronDown()
			chevTip := "折叠本步骤"
			if collapsed {
				chev = lucide.ChevronRight()
				chevTip = "展开本步骤"
			}
			foldBtn := components.IconButton(c, chev, chevTip, components.IconButtonCompact)
			if foldBtn.Clicked() {
				v.stepCollapsed[step.ID] = !collapsed
			}
			if editable {
				stBuf := v.ensureWidgetDraft("live_step_t:"+step.ID, stepTitle)
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					ti := BorderlessTextInput(c, tok, stBuf, float32(tok.TextSection), true, "步骤标题…")
					if ti.Submitted() || (!ti.Focused() && strings.TrimSpace(*stBuf) != "" && strings.TrimSpace(*stBuf) != stepTitle) {
						if v.OnUpdateModule != nil {
							v.OnUpdateModule(step.ID, map[string]any{"title": strings.TrimSpace(*stBuf)})
						} else if v.Doc != nil && v.OnChange != nil {
							if nextSrc, uErr := v.Doc.UpdateModule(step.ID, map[string]any{"title": strings.TrimSpace(*stBuf)}); uErr == nil {
								v.OnChange(nextSrc)
							}
						}
					}
				})
			} else {
				ui.Text(c, stepTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			}
			if collapsed && len(blocks) > 0 {
				components.Badge(c, fmt.Sprintf("%d 块", len(blocks)))
			}
			if totalStepChecks > 0 {
				allDone := doneStepChecks == totalStepChecks
				pillBox := ui.Row(c.Key("step-chk-prog:"+step.ID)).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).
					Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Shrink(0)
				targetBg := ui.Transparent
				targetFg := tok.TextMuted
				if allDone {
					targetBg = tok.Success.Alpha(.10)
					targetFg = tok.Success
				}
				pillBox.Background(pillBox.AnimateColor("sbg", targetBg, theme.MotionSnappy))
				fgCol := pillBox.AnimateColor("sfg", targetFg, theme.MotionSnappy)
				pillBox.Children(func() {
					ico := lucide.CircleDot()
					if allDone {
						ico = lucide.Check()
					}
					ui.Icon(c, ico).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(fgCol).ContentTransition(theme.IconMorph())
					lbl := fmt.Sprintf("本步核对 %d / %d", doneStepChecks, totalStepChecks)
					if allDone {
						lbl = fmt.Sprintf("本步核对已就绪 (%d/%d)", doneStepChecks, totalStepChecks)
					}
					ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fgCol).SingleLine().
						FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
				})
			}
			if editable && v.OnInsertStepBlock != nil {
				addBtn := components.Button(c, "+ 添加组件", components.ButtonGhost)
				components.MenuButton(c, addBtn, func(m *components.Menu) {
					buildInsertMenu(m, len(blocks))
				})
			}
		})
		if collapsed {
			return
		}
		// Visual step rail: clearly indicates all blocks below belong to this step.
		ui.Row(c.Key("live-step-rail:"+step.ID)).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
			ui.Box(c).Width(2).FillHeight().Radius(float32(tok.ControlRadius)).Background(tok.Accent.Alpha(0.28))
			ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing8)).Children(func() {
				if len(blocks) == 0 && len(stringList(step.Data, "block_refs")) == 0 {
					if editable && v.OnInsertStepBlock != nil {
						ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, "此步骤暂无内容组件 — 选择下方常用组件直接添加到本步骤：").
								FontSize(float32(tok.TextData)).TextColor(tok.TextMuted)
							ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
								for _, q := range []struct {
									kind, label string
								}{
									{"prose", "+ 文本说明"},
									{"checklist", "+ 检查清单"},
									{"timer", "+ 计时器"},
									{"param_table", "+ 参数表"},
									{"plate", "+ 孔板"},
									{"data_table", "+ 数据表"},
									{"calculator", "+ 计算器"},
								} {
									q := q
									if components.Button(c, q.label, components.ButtonSecondary).Clicked() {
										v.OnInsertStepBlock(step.ID, 0, defaultStarterBlock(v.Doc, step.ID, q.kind))
									}
								}
								moreBtn := components.Button(c, "更多组件 ▾", components.ButtonGhost)
								components.MenuButton(c, moreBtn, func(m *components.Menu) {
									buildInsertMenu(m, 0)
								})
							})
						})
					}
					return
				}

				for idx, bm := range blocks {
					idx, bm := idx, bm
					bk := textValue(bm, "kind")
					bid := textValue(bm, "id")
					bs := int(numOrZero(bm["schema"]))
					if bs <= 0 {
						bs = 1
					}

					blockRow := ui.Row(c.Key("blk-row-"+bid)).FillWidth().AlignItems(ui.Start).
						Gap(float32(tok.Spacing4)).Transition(theme.AccordionTransition())
					rowHovered := blockRow.Hovered()
					blockRow.Children(func() {
						if editable {
							grip := ui.Box(c.Key("grip-"+bid)).Width(float32(tok.Spacing16)).Height(float32(tok.Spacing24)).
								Center().Cursor(ui.CursorGrab).Label("操作块 " + bid)
							gripTarget := float32(0)
							if rowHovered {
								gripTarget = 1
							}
							gripOp := grip.AnimateTo("grip-op", gripTarget, theme.MotionMicroFade)
							grip.Opacity(gripOp).Children(func() {
								ui.Icon(c, lucide.GripVertical()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
							})
							grip.Drag(stepBlockDrag{StepID: step.ID, BlockID: bid, FromIndex: idx})
							if grip.Dragging() {
								blockRow.Opacity(0.45)
							}
							components.ContextMenu(c, grip, func(m *components.Menu) {
								v.buildBlockContextMenu(c, m, step.ID, bid, idx, len(blocks), bk, bs, bm, editable)
							})
						}
						ui.Box(c).Grow(1).MinWidth(0).Children(func() {
							v.RenderModule(c, bk, bs, bm)
						})
					})

					components.ContextMenu(c, blockRow, func(m *components.Menu) {
						v.buildBlockContextMenu(c, m, step.ID, bid, idx, len(blocks), bk, bs, bm, editable)
					})
					if editable {
						if dragged, over := ui.DragOver[stepBlockDrag](blockRow); over && dragged.StepID == step.ID && dragged.FromIndex != idx {
							blockRow.Border(2, tok.Accent)
						}
						if dragged, dropped := ui.Drop[stepBlockDrag](blockRow); dropped && dragged.StepID == step.ID && dragged.FromIndex != idx {
							if v.OnMoveStepBlockIndex != nil {
								v.OnMoveStepBlockIndex(step.ID, dragged.BlockID, idx)
							} else if v.OnMoveStepBlock != nil {
								if idx > dragged.FromIndex {
									v.OnMoveStepBlock(step.ID, dragged.BlockID, "down")
								} else {
									v.OnMoveStepBlock(step.ID, dragged.BlockID, "up")
								}
							}
						}
						// Check slash menu open trigger from prose block
						slashOpen := ui.Local(c.Root(), "slash-menu-step:"+step.ID+":"+bid, func() bool { return false })
						if *slashOpen && v.OnInsertStepBlock != nil {
							components.Popover(c, blockRow, slashOpen, func(panel ui.Element) {
								panel.Width(float32(tok.Spacing48)*5).Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing4)).Children(func() {
									ui.Text(c, "插入组件").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
									for _, ent := range []paletteEntry{
										{Kind: "prose", Label: "文本段落", Icon: lucide.FileText()},
										{Kind: "checklist", Label: "检查清单", Icon: lucide.SquareCheck()},
										{Kind: "timer", Label: "计时器", Icon: lucide.Timer()},
										{Kind: "param_table", Label: "参数表", Icon: lucide.SlidersHorizontal()},
										{Kind: "group_table", Label: "分组表", Icon: lucide.Table2()},
										{Kind: "plate", Label: "孔板 / 管架", Icon: lucide.Grid2x2()},
										{Kind: "data_table", Label: "数据记录表", Icon: lucide.Table2()},
										{Kind: "calculator", Label: "稀释计算表", Icon: lucide.Calculator()},
										{Kind: "figure", Label: "图片 / 示意图", Icon: lucide.Image()},
										{Kind: "attachment", Label: "附件与采图", Icon: lucide.Paperclip()},
										{Kind: "subflow", Label: "子流程", Icon: lucide.GitBranch()},
									} {
										ent := ent
										if components.Button(c, ent.Label, components.ButtonGhost).Clicked() {
											*slashOpen = false
											v.OnInsertStepBlock(step.ID, idx+1, DefaultStarterBlockWithPrefs(v.Doc, step.ID, ent.Kind, v.DefaultPlateFormat, v.DefaultExcessPct))
										}
									}
								})
							})
						}
					}
					renderHoverInsertGap(idx+1, false)
				}

				// Resolved top-level block_refs
				for _, refID := range stringList(step.Data, "block_refs") {
					if v.Doc == nil {
						continue
					}
					for i := range v.Doc.Modules {
						if v.Doc.Modules[i].ID == refID {
							m := v.Doc.Modules[i]
							v.RenderModule(c, m.Kind, m.Schema, m.Data)
							break
						}
					}
				}
			})
		})
	})
}

func (v *View) buildBlockContextMenu(c *ui.Context, m *components.Menu, stepID, blockID string, idx, total int, kind string, schema int, bm map[string]any, editable bool) {
	if editable {
		if m.Item("剪切").Shortcut("Ctrl+X").Chosen() {
			sharedBlockClipboard = cloneAnyMap(bm)
			if v.OnRemoveStepBlock != nil {
				v.OnRemoveStepBlock(stepID, blockID)
			}
		}
		if m.Item("复制块").Shortcut("Ctrl+C").Chosen() {
			sharedBlockClipboard = cloneAnyMap(bm)
		}
		if m.Item("粘贴到下方").Shortcut("Ctrl+V").Disabled(sharedBlockClipboard == nil || v.OnInsertStepBlock == nil).Chosen() {
			clone := cloneAnyMap(sharedBlockClipboard)
			delete(clone, "id")
			v.OnInsertStepBlock(stepID, idx+1, clone)
		}
		if m.Item("创建副本").Shortcut("Ctrl+D").Chosen() {
			if v.OnDuplicateBlock != nil {
				v.OnDuplicateBlock(stepID, blockID)
			} else if v.OnInsertStepBlock != nil {
				clone := cloneAnyMap(bm)
				delete(clone, "id")
				v.OnInsertStepBlock(stepID, idx+1, clone)
			}
		}
		m.Separator()
		if m.Item("上移").Disabled(idx == 0).Chosen() {
			if v.OnMoveStepBlockIndex != nil {
				v.OnMoveStepBlockIndex(stepID, blockID, idx-1)
			} else if v.OnMoveStepBlock != nil {
				v.OnMoveStepBlock(stepID, blockID, "up")
			}
		}
		if m.Item("下移").Disabled(idx >= total-1).Chosen() {
			if v.OnMoveStepBlockIndex != nil {
				v.OnMoveStepBlockIndex(stepID, blockID, idx+1)
			} else if v.OnMoveStepBlock != nil {
				v.OnMoveStepBlock(stepID, blockID, "down")
			}
		}
		m.Separator()
		if m.Item("删除").Shortcut("Del").Chosen() && v.OnRemoveStepBlock != nil {
			v.OnRemoveStepBlock(stepID, blockID)
		}
		m.Separator()
	}
	v.buildComponentShareMenu(c, m, kind, schema, bm)
}
