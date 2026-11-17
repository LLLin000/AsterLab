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

// StepBlockComposer renders the Design-mode interactive step workbench:
// step metadata header + ordered composable blocks[] + block_refs + 10-module insert palette.
func StepBlockComposer(v *View, c *ui.Context, step *Module) {
	if v == nil || c == nil || step == nil {
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
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
				if editable {
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, fmt.Sprintf("#%d · %s", idx+1, blockKindLabel(bk))).
							FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).FontFeatures("tnum")
						ui.Text(c, bid).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						ui.Spacer(c)
						if idx > 0 && v.OnMoveStepBlock != nil {
							upBtn := components.IconButton(c, lucide.ArrowUp(), "上移模块", components.IconButtonCompact)
							components.Tooltip(c, upBtn, "上移此模块")
							if upBtn.Clicked() {
								v.OnMoveStepBlock(step.ID, bid, "up")
							}
						}
						if idx < len(blocks)-1 && v.OnMoveStepBlock != nil {
							dnBtn := components.IconButton(c, lucide.ArrowDown(), "下移模块", components.IconButtonCompact)
							components.Tooltip(c, dnBtn, "下移此模块")
							if dnBtn.Clicked() {
								v.OnMoveStepBlock(step.ID, bid, "down")
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
				}
				v.RenderModule(c, bk, bs, bm)
			})
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
						components.Badge(c, "共享容器/模块引用 · " + refID)
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
	id := nextUniqueBlockID(doc, stepID+"-"+sanitizeIDFrag(kind))
	switch kind {
	case "prose":
		return map[string]any{"kind": "prose", "id": id, "schema": 1, "title": "操作说明", "tone": "tip", "body": "在此填写实验操作要点与注意事项。"}
	case "group_table":
		return map[string]any{"kind": "group_table", "id": id, "schema": 1, "title": "实验分组", "rows": []any{
			map[string]any{"id": id + "-ctrl", "name": "对照组", "role": "control", "independent_n": 3},
			map[string]any{"id": id + "-treat", "name": "处理组", "role": "treatment", "independent_n": 3},
		}}
	case "param_table":
		return map[string]any{"kind": "param_table", "id": id, "schema": 1, "title": "实验参数", "rows": []any{
			map[string]any{"key": "volume", "label": "反应体积", "value": 100, "unit": "uL", "record_actual": true},
		}}
	case "figure":
		return map[string]any{
			"kind": "figure", "id": id, "schema": 1, "title": "分区示意图",
			"caption": "示意图仅作视觉表达，分组真值存于分组数据",
			"svg": `<svg viewBox="0 0 280 200"><circle cx="140" cy="100" r="82" fill="none" stroke="currentColor" stroke-width="2"/><line x1="58" y1="100" x2="222" y2="100" stroke="currentColor" stroke-dasharray="4 4"/><line x1="140" y1="18" x2="140" y2="182" stroke="currentColor" stroke-dasharray="4 4"/><text x="98" y="65" text-anchor="middle" font-size="13">I 对照</text><text x="182" y="65" text-anchor="middle" font-size="13">II 处理A</text><text x="98" y="140" text-anchor="middle" font-size="13">III 处理B</text><text x="182" y="140" text-anchor="middle" font-size="13">IV 阳参</text></svg>`,
		}
	case "data_table":
		return map[string]any{"kind": "data_table", "id": id, "schema": 1, "title": "数据记录表", "endpoint": "measurement", "columns": []any{
			map[string]any{"key": "sample", "label": "样本", "value_kind": "text"},
			map[string]any{"key": "reading", "label": "测定值", "value_kind": "number", "unit": "AU"},
		}}
	case "checklist":
		return map[string]any{"kind": "checklist", "id": id, "schema": 1, "title": "执行检查", "items": []any{
			map[string]any{"id": id + "-c1", "label": "确认试剂与耗材准备就绪", "required": true},
		}}
	case "calculator":
		return map[string]any{
			"kind": "calculator", "id": id, "schema": 1, "title": "稀释计算", "mode": "dilution",
			"stock": map[string]any{"value": 10, "unit": "mg/mL"},
			"target": map[string]any{"value": 1, "unit": "mg/mL"},
			"final_volume": map[string]any{"value": 1, "unit": "mL"},
		}
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
		return map[string]any{"kind": "plate", "id": id, "schema": 1, "title": "12孔板布局", "format": 12, "groups": map[string]any{firstGroup: []any{"A1"}}}
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
